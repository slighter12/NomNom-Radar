package postgres

import (
	"github.com/slighter12/go-lib/errors/stack"
)

func withSourceStack(err error) error {
	return stack.WithSkip(err, 1)
}

func replaceWithSourceStack(err, replacement error) error {
	return stack.Replace(stack.WithSkip(err, 1), replacement)
}
