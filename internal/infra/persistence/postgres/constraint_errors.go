package postgres

import (
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// Helper functions for PostgreSQL error checking
func isUniqueConstraintViolation(err error) bool {
	// Check for GORM's duplicate key error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	// Check error message for PostgreSQL-specific unique constraint violation patterns (may not be needed)
	// errMsg := strings.ToLower(err.Error())
	// return strings.Contains(errMsg, "duplicate key") ||
	// 	strings.Contains(errMsg, "unique constraint") ||
	// 	strings.Contains(errMsg, "already exists")
	return false

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

func isCheckConstraintViolation(err error) bool {
	// Check for GORM's check constraint violation error
	if errors.Is(err, gorm.ErrCheckConstraintViolated) {
		return true
	}

	// Check error message for PostgreSQL-specific check constraint violation patterns (may not be needed)
	// errMsg := strings.ToLower(err.Error())
	// return strings.Contains(errMsg, "check constraint") ||
	// 	strings.Contains(errMsg, "validation failed") // PostgreSQL check_violation error code
	return false
}
