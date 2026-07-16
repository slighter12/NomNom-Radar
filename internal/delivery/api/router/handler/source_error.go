package handler

import (
	"errors"
	"net/http"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"
)

func withSourceStack(err error) error {
	if appErr, ok := errors.AsType[domainerrors.AppError](err); ok && appErr.HTTPCode() < http.StatusInternalServerError {
		return err
	}

	return observability.WithSourceStackSkip(err, 1)
}
