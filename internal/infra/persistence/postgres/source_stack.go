package postgres

import (
	"radar/internal/platform/observability"
)

func withSourceStack(err error) error {
	return observability.WithSourceStackSkip(err, 1)
}
