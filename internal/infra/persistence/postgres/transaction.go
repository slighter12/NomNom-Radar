package postgres

import (
	"context"
	"log/slog"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"

	"gorm.io/gorm"
)

// gormTransactionManager implements the domain's TransactionManager interface using GORM.
type gormTransactionManager struct {
	db     *gorm.DB
	logger *slog.Logger
}

// gormRepositoryFactory implements the domain's transaction-scoped Unit of Work.
type gormRepositoryFactory struct {
	tx *gorm.DB // In GORM, a transaction object *gorm.Tx is also a *gorm.DB

	userRepo         repository.UserRepository
	authRepo         repository.AuthRepository
	addressRepo      repository.AddressRepository
	refreshTokenRepo repository.RefreshTokenRepository
	loginAttemptRepo repository.LoginAttemptRepository
	discoveryRepo    repository.DiscoveryRepository
}

// UserRepo returns a user repository instance bound to the transaction.
func (f *gormRepositoryFactory) UserRepo() repository.UserRepository {
	if f.userRepo == nil {
		f.userRepo = NewUserRepository(f.tx)
	}

	return f.userRepo
}

// AuthRepo returns an auth repository instance bound to the transaction.
func (f *gormRepositoryFactory) AuthRepo() repository.AuthRepository {
	if f.authRepo == nil {
		f.authRepo = NewAuthRepository(f.tx)
	}

	return f.authRepo
}

// AddressRepo returns an address repository instance bound to the transaction.
func (f *gormRepositoryFactory) AddressRepo() repository.AddressRepository {
	if f.addressRepo == nil {
		f.addressRepo = NewAddressRepository(f.tx)
	}

	return f.addressRepo
}

// RefreshTokenRepo returns a refresh token repository instance bound to the transaction.
func (f *gormRepositoryFactory) RefreshTokenRepo() repository.RefreshTokenRepository {
	if f.refreshTokenRepo == nil {
		f.refreshTokenRepo = NewRefreshTokenRepository(f.tx)
	}

	return f.refreshTokenRepo
}

// LoginAttemptRepo returns a login attempt repository instance bound to the transaction.
func (f *gormRepositoryFactory) LoginAttemptRepo() repository.LoginAttemptRepository {
	if f.loginAttemptRepo == nil {
		f.loginAttemptRepo = NewLoginAttemptRepository(f.tx)
	}

	return f.loginAttemptRepo
}

// DiscoveryRepo returns a discovery repository instance bound to the transaction.
func (f *gormRepositoryFactory) DiscoveryRepo() repository.DiscoveryRepository {
	if f.discoveryRepo == nil {
		f.discoveryRepo = NewDiscoveryRepository(f.tx)
	}

	return f.discoveryRepo
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
		return tx.Error
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
			tm.logger.Error("transaction rollback failed", slog.String("error", rbErr.Error()))
		}

		return err //nolint:wrapcheck // preserve the original business error without adding a redundant wrapper
	}

	// If the business logic completes without error, commit the transaction.
	if err := tx.Commit().Error; err != nil {
		return domainerrors.ErrPersistenceFailed
	}

	return nil
}
