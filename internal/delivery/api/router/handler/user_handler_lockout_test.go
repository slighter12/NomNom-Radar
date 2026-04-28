package handler

import (
	"errors"
	"net/http"
	"testing"

	"radar/internal/usecase"

	"github.com/stretchr/testify/assert"
)

func TestSetRetryAfterHeaderOnLockout(t *testing.T) {
	c, _ := newJSONContext(http.MethodPost, "/link-provider", `{}`)
	err := &usecase.LockoutError{
		RetryAfterSeconds: 900,
		Err:               errors.New("invalid credentials"),
	}

	setRetryAfterHeaderOnLockout(c, err)

	assert.Equal(t, "900", c.Response().Header().Get("Retry-After"))
}

func TestSetRetryAfterHeaderOnLockoutIgnoresOtherErrors(t *testing.T) {
	c, _ := newJSONContext(http.MethodPost, "/link-provider", `{}`)

	setRetryAfterHeaderOnLockout(c, errors.New("other error"))

	assert.Empty(t, c.Response().Header().Get("Retry-After"))
}
