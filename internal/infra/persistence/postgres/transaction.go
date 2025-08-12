// Package postgres contains the concrete implementation of the persistence layer using GORM and PostgreSQL.
package postgres

import (
	"context"
	"log/slog"

	"radar/internal/domain/repository"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// gormTransactionManager implements the domain's TransactionManager interface using GORM.
type gormTransactionManager struct {
	db     *gorm.DB
	logger *slog.Logger
}

// gormRepositoryFactory implements the domain's RepositoryFactory interface.
// It holds a specific GORM transaction object (*gorm.Tx) and uses it to create
// repository instances that are bound to that single transaction.
type gormRepositoryFactory struct {
	tx *gorm.DB // In GORM, a transaction object *gorm.Tx is also a *gorm.DB
}

// NewUserRepository creates a new user repository instance bound to the transaction.
func (f *gormRepositoryFactory) NewUserRepository() repository.UserRepository {
	return NewUserRepository(f.tx)
}

// NewAuthRepository creates a new auth repository instance bound to the transaction.
func (f *gormRepositoryFactory) NewAuthRepository() repository.AuthRepository {
	return NewAuthRepository(f.tx)
}

// NewTransactionManager is the constructor for gormTransactionManager.
// This function will be used as an Fx provider.
func NewTransactionManager(db *gorm.DB, logger *slog.Logger) repository.TransactionManager {
	return &gormTransactionManager{db: db, logger: logger}
}

// Execute runs the given function within a single database transaction.
func (tm *gormTransactionManager) Execute(ctx context.Context, fn func(repoFactory repository.RepositoryFactory) error) error {
	// Begin a new transaction
	tx := tm.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return errors.Wrap(tx.Error, "failed to begin transaction")
	}

	// This defer block ensures that if a panic occurs within the callback function,
	// the transaction is always rolled back. This is a critical safety measure.
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			// Re-panic to allow Fx or other middleware to handle the panic.
			panic(r)
		}
	}()

	// Create a repository factory that is bound to this specific transaction.
	factory := &gormRepositoryFactory{tx: tx}

	// Execute the application logic (the use case's core work)
	err := fn(factory)
	if err != nil {
		// If the business logic returns an error, roll back the transaction.
		if rbErr := tx.Rollback().Error; rbErr != nil {
			// Log the rollback error, but return the original, more meaningful business error.
			tm.logger.Error("transaction rollback failed", "error", rbErr)
		}

		return errors.Wrap(err, "transaction failed") // Return the original business error.
	}

	// If the business logic completes without error, commit the transaction.
	if err := tx.Commit().Error; err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}
