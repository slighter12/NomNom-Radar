package postgres

import (
	"context"
	"testing"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAddressRepository_MutationsRejectUnsupportedOwnerType(t *testing.T) {
	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{
		DSN:                  "host=localhost user=test password=test dbname=test sslmode=disable",
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	require.NoError(t, err)

	repo := NewAddressRepository(db)
	ctx := context.Background()
	unsupportedOwnerType := entity.OwnerType("unsupported")
	label := "Home"

	err = repo.UpdateAddress(ctx, uuid.New(), unsupportedOwnerType, uuid.New(), &entity.AddressUpdate{Label: &label})
	require.Error(t, err)

	err = repo.DeleteAddress(ctx, uuid.New(), unsupportedOwnerType, uuid.New())
	require.Error(t, err)
}
