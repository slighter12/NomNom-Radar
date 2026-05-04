package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	constraintMerchantBusinessLicenseActive = "idx_merchant_profiles_business_license_active"
	constraintUsersEmailActive              = "idx_users_email_active"
	rowLockStrengthUpdate                   = "UPDATE"
)

// Helper functions for PostgreSQL error checking
func isUniqueConstraintViolation(err error) bool {
	// Check for GORM's duplicate key error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}

	errMsg := strings.ToLower(err.Error())

	return strings.Contains(errMsg, "duplicate key") ||
		strings.Contains(errMsg, "unique constraint") ||
		strings.Contains(errMsg, "23505")
}

func violatedConstraintName(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.ConstraintName
	}

	errMsg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(errMsg, constraintMerchantBusinessLicenseActive):
		return constraintMerchantBusinessLicenseActive
	case strings.Contains(errMsg, "merchant_profiles_business_license_key"):
		return "merchant_profiles_business_license_key"
	case strings.Contains(errMsg, constraintUsersEmailActive):
		return constraintUsersEmailActive
	case strings.Contains(errMsg, "users_email_key"):
		return "users_email_key"
	case strings.Contains(errMsg, "idx_auth_provider_provider_user_id_active"):
		return "idx_auth_provider_provider_user_id_active"
	case strings.Contains(errMsg, "idx_auth_provider_provider_user_id"):
		return "idx_auth_provider_provider_user_id"
	default:
		return ""
	}
}

func isBusinessLicenseUniqueConstraint(err error) bool {
	switch violatedConstraintName(err) {
	case constraintMerchantBusinessLicenseActive, "merchant_profiles_business_license_key":
		return true
	default:
		return false
	}
}

func isForeignKeyConstraintViolation(err error) bool {
	// Check for GORM's foreign key violation error
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return true
	}

	// Check error message for PostgreSQL-specific foreign key constraint violation patterns (may not be needed)
	// errMsg := strings.ToLower(err.Error())
	// return strings.Contains(errMsg, "foreign key") ||
	// 	strings.Contains(errMsg, "references") ||
	// 	strings.Contains(errMsg, "constraint") // PostgreSQL foreign_key_violation error code
	return false
}

func isNotNullConstraintViolation(err error) bool {
	// Check error message for PostgreSQL-specific not null constraint violation patterns
	errMsg := strings.ToLower(err.Error())

	return strings.Contains(errMsg, "null value") ||
		strings.Contains(errMsg, "not null") ||
		strings.Contains(errMsg, "required") ||
		strings.Contains(errMsg, "23502") // PostgreSQL not_null_violation error code
}

// func isCheckConstraintViolation(err error) bool {
// 	// Check for GORM's check constraint violation error
// 	if errors.Is(err, gorm.ErrCheckConstraintViolated) {
// 		return true
// 	}

// 	// Check error message for PostgreSQL-specific check constraint violation patterns (may not be needed)
// 	// errMsg := strings.ToLower(err.Error())
// 	// return strings.Contains(errMsg, "check constraint") ||
// 	// 	strings.Contains(errMsg, "validation failed") // PostgreSQL check_violation error code
// 	return false
// }
