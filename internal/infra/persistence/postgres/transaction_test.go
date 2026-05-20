package postgres

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGormRepositoryFactory_ReusesReposWithinUnitOfWork(t *testing.T) {
	factory := newGormRepositoryFactory(newTransactionFactoryTestDB(t))

	assert.Same(t, factory.UserRepo(), factory.UserRepo())
	assert.Same(t, factory.AuthRepo(), factory.AuthRepo())
	assert.Same(t, factory.AddressRepo(), factory.AddressRepo())
	assert.Same(t, factory.RefreshTokenRepo(), factory.RefreshTokenRepo())
	assert.Same(t, factory.LoginAttemptRepo(), factory.LoginAttemptRepo())
	assert.Same(t, factory.DiscoveryRepo(), factory.DiscoveryRepo())
}

func TestGormRepositoryFactory_DoesNotShareReposAcrossUnitsOfWork(t *testing.T) {
	db := newTransactionFactoryTestDB(t)
	first := newGormRepositoryFactory(db)
	second := newGormRepositoryFactory(db)

	assert.NotSame(t, first.UserRepo(), second.UserRepo())
	assert.NotSame(t, first.AuthRepo(), second.AuthRepo())
	assert.NotSame(t, first.AddressRepo(), second.AddressRepo())
	assert.NotSame(t, first.RefreshTokenRepo(), second.RefreshTokenRepo())
	assert.NotSame(t, first.LoginAttemptRepo(), second.LoginAttemptRepo())
	assert.NotSame(t, first.DiscoveryRepo(), second.DiscoveryRepo())
}

func TestGormRepositoryFactory_AccessorsAreReadOnlyUnderConcurrentUse(t *testing.T) {
	factory := newGormRepositoryFactory(newTransactionFactoryTestDB(t))

	var wg sync.WaitGroup
	nilRepo := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			if factory.UserRepo() == nil ||
				factory.AuthRepo() == nil ||
				factory.AddressRepo() == nil ||
				factory.RefreshTokenRepo() == nil ||
				factory.LoginAttemptRepo() == nil ||
				factory.DiscoveryRepo() == nil {
				nilRepo <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(nilRepo)

	assert.Empty(t, nilRepo)
}

func newTransactionFactoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{
		DSN: "host=localhost user=user password=password dbname=auth_db port=5432 sslmode=disable",
	}), &gorm.Config{
		DisableAutomaticPing: true,
		DryRun:               true,
	})
	require.NoError(t, err)

	return db
}
