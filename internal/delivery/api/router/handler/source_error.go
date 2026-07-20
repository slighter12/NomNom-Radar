package handler

import (
	"github.com/slighter12/go-lib/errors/stack"
)

func withSourceStack(err error) error {
	return stack.WithSkip(err, 1)
}
