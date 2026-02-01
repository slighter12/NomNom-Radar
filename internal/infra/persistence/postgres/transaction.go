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

// UserRepo creates a new user repository instance bound to the transaction.
func (f *gormRepositoryFactory) UserRepo() repository.UserRepository {
	return NewUserRepository(f.tx)
}

// AuthRepo creates a new auth repository instance bound to the transaction.
func (f *gormRepositoryFactory) AuthRepo() repository.AuthRepository {
	return NewAuthRepository(f.tx)
}

// AddressRepo creates a new address repository instance bound to the transaction.
func (f *gormRepositoryFactory) AddressRepo() repository.AddressRepository {
	return NewAddressRepository(f.tx)
}

// RefreshTokenRepo creates a new refresh token repository instance bound to the transaction.
func (f *gormRepositoryFactory) RefreshTokenRepo() repository.RefreshTokenRepository {
	return NewRefreshTokenRepository(f.tx)
}

// NewTransactionManager is the constructor for gormTransactionManager.
// This function will be used as an Fx provider.
func NewTransactionManager(db *gorm.DB, logger *slog.Logger) repository.TransactionManager {
	return &gormTransactionManager{db: db, logger: logger}
}

// GetRepositoryFactory returns a repository factory for non-transactional operations.
func (tm *gormTransactionManager) GetRepositoryFactory() repository.RepositoryFactory {
	return &gormRepositoryFactory{tx: tm.db}
}

// Execute runs the given function within a single database transaction.
func (tm *gormTransactionManager) Execute(ctx context.Context, fn func(repoFactory repository.RepositoryFactory) error) error {
	// Begin a new transaction
	tx := tm.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return errors.WithStack(tx.Error)
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
			tm.logger.Error("transaction rollback failed", slog.Any("error", rbErr))
		}

		return errors.WithStack(err) // Return the original business error.
	}

	// If the business logic completes without error, commit the transaction.
	if err := tx.Commit().Error; err != nil {
		return errors.WithStack(err)
	}

	return nil
}
