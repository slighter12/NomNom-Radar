package handler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"
)

func TestWithSourceStackSkipsClientAppError(t *testing.T) {
	err := withSourceStack(fmt.Errorf("invalid request: %w", domainerrors.ErrInvalidInput))

	if _, ok := errors.AsType[observability.SourceStackProvider](err); ok {
		t.Fatal("4xx app error should not capture source stack")
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

	provider, ok := errors.AsType[observability.SourceStackProvider](err)
	if !ok {
		t.Fatal("5xx app error should capture source stack")
	}
	if !errors.Is(err, domainerrors.ErrPersistenceFailed) {
		t.Fatal("5xx app error should preserve errors.Is")
	}
	stack := provider.SourceStack()
	if !strings.Contains(stack, "handlerSourceStackTestError") {
		t.Fatalf("stack should include source caller, got:\n%s", stack)
	}
	firstFrame, _, _ := strings.Cut(stack, "\n")
	if strings.Contains(firstFrame, "withSourceStack") {
		t.Fatalf("first stack frame should be the handler source caller, got %q", firstFrame)
	}
}

func TestWithSourceStackWrapsNonAppError(t *testing.T) {
	baseErr := errors.New("boom")
	err := withSourceStack(baseErr)

	if _, ok := errors.AsType[observability.SourceStackProvider](err); !ok {
		t.Fatal("non-app error should capture source stack")
	}
	if !errors.Is(err, baseErr) {
		t.Fatal("non-app error should preserve errors.Is")
	}
}
