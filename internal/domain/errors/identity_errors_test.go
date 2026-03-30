package errors

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrAuthNotFound_HTTPCodeIsNotFound(t *testing.T) {
	assert.Equal(t, http.StatusNotFound, ErrAuthNotFound.HTTPCode())
}
