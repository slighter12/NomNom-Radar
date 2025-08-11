package repository

import "context"

// TransactionManager defines the interface for managing database transactions.
// This allows the use case layer to handle transactions without depending on a specific DB driver like GORM.
type TransactionManager interface {
	// Execute runs a function within a database transaction.
	// If the function returns an error, the transaction is rolled back. Otherwise, it's committed.
	Execute(ctx context.Context, fn func(txRepoFactory RepositoryFactory) error) error
}

// RepositoryFactory provides a way to get repository instances that are bound to a specific transaction.
type RepositoryFactory interface {
	NewUserRepository() UserRepository
	NewAuthRepository() AuthRepository
}
