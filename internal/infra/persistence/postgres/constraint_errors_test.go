package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestIsUniqueConstraintViolation(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "gorm duplicated key",
			err:  gorm.ErrDuplicatedKey,
			want: true,
		},
		{
			name: "postgres duplicate key code",
			err:  &pgconn.PgError{Code: "23505", ConstraintName: "idx_users_email_active"},
			want: true,
		},
		{
			name: "postgres duplicate key message",
			err: errors.New(
				`ERROR: duplicate key value violates unique constraint "idx_merchant_profiles_business_license_active" (SQLSTATE 23505)`,
			),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("connection failed"),
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isUniqueConstraintViolation(tc.err))
		})
	}
}

func TestIsBusinessLicenseUniqueConstraint(t *testing.T) {
	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "active business license index from pg error",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "idx_merchant_profiles_business_license_active",
			},
			want: true,
		},
		{
			name: "legacy business license constraint from message",
			err: errors.New(
				`ERROR: duplicate key value violates unique constraint "merchant_profiles_business_license_key" (SQLSTATE 23505)`,
			),
			want: true,
		},
		{
			name: "user email unique index",
			err: &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "idx_users_email_active",
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isBusinessLicenseUniqueConstraint(tc.err))
		})
	}
}
