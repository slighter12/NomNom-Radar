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
	// NewUserRepository returns a UserRepository instance bound to the current transaction.
	NewUserRepository() UserRepository

	// NewAuthRepository returns an AuthRepository instance bound to the current transaction.
	NewAuthRepository() AuthRepository

	// NewAddressRepository returns an AddressRepository instance bound to the current transaction.
	NewAddressRepository() AddressRepository

	// NewRefreshTokenRepository returns a RefreshTokenRepository instance bound to the current transaction.
	NewRefreshTokenRepository() RefreshTokenRepository
}
