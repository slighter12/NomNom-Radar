package repository

import "context"

// TransactionManager defines the interface for managing database transactions.
// This allows the use case layer to handle transactions without depending on a specific DB driver like GORM.
type TransactionManager interface {
	// Execute runs a function within a database transaction.
	// If the function returns an error, the transaction is rolled back. Otherwise, it's committed.
	// All repository operations within the function will use the same database transaction.
	Execute(ctx context.Context, fn func(txRepoFactory RepositoryFactory) error) error
}

// RepositoryFactory provides a way to get repository instances that are bound to a specific transaction.
// This ensures all repository operations within a transaction use the same database connection.
type RepositoryFactory interface {
	// UserRepo returns a UserRepo instance bound to the current transaction.
	UserRepo() UserRepository
	// AuthRepo returns an AuthRepo instance bound to the current transaction.
	AuthRepo() AuthRepository
	// AddressRepo returns an AddressRepo instance bound to the current transaction.
	AddressRepo() AddressRepository
	// RefreshTokenRepo returns a RefreshTokenRepo instance bound to the current transaction.
	RefreshTokenRepo() RefreshTokenRepository
}
