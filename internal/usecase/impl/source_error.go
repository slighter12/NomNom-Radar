package impl

import "github.com/slighter12/go-lib/errors/stack"

func replaceWithSourceStack(err, replacement error) error {
	return stack.Replace(stack.WithSkip(err, 1), replacement)
}
