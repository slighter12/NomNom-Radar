package postgres

import (
	stderrors "errors"
	"net/http"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"
)

func withSourceStack(err error) error {
	// Non-AppError callers wrap explicitly when a source stack is needed.
	if appErr, ok := stderrors.AsType[domainerrors.AppError](err); ok && appErr.HTTPCode() >= http.StatusInternalServerError {
		return observability.WithSourceStackSkip(err, 1)
	}

	return err
}
