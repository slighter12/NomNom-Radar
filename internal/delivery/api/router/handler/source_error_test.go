package handler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	domainerrors "radar/internal/domain/errors"

	"github.com/slighter12/go-lib/errors/stack"
)

func TestWithSourceStackWrapsClientAppError(t *testing.T) {
	err := withSourceStack(fmt.Errorf("invalid request: %w", domainerrors.ErrInvalidInput))

	if _, ok := errors.AsType[stack.Provider](err); !ok {
		t.Fatal("4xx app error should capture source stack")
	}
	if !errors.Is(err, domainerrors.ErrInvalidInput) {
		t.Fatal("4xx app error should preserve errors.Is")
	}
}

func handlerSourceStackTestError() error {
	return withSourceStack(domainerrors.ErrPersistenceFailed)
}

func TestWithSourceStackWrapsInternalAppError(t *testing.T) {
	err := handlerSourceStackTestError()

	provider, ok := errors.AsType[stack.Provider](err)
	if !ok {
		t.Fatal("5xx app error should capture source stack")
	}
	if !errors.Is(err, domainerrors.ErrPersistenceFailed) {
		t.Fatal("5xx app error should preserve errors.Is")
	}
	frames := provider.Stack()
	if !strings.Contains(frames, "handlerSourceStackTestError") {
		t.Fatalf("stack should include source caller, got:\n%s", frames)
	}
	firstFrame, _, _ := strings.Cut(frames, "; ")
	if strings.Contains(firstFrame, "withSourceStack") {
		t.Fatalf("first stack frame should be the handler source caller, got %q", firstFrame)
	}
}

func TestWithSourceStackWrapsNonAppError(t *testing.T) {
	baseErr := errors.New("boom")
	err := withSourceStack(baseErr)

	if _, ok := errors.AsType[stack.Provider](err); !ok {
		t.Fatal("non-app error should capture source stack")
	}
	if !errors.Is(err, baseErr) {
		t.Fatal("non-app error should preserve errors.Is")
	}
}
