package postgres

import (
	stderrors "errors"
	"strings"
	"testing"

	domainerrors "radar/internal/domain/errors"
	"radar/internal/platform/observability"
)

func postgresSourceStackTestError() error {
	return withSourceStack(domainerrors.ErrPersistenceFailed)
}

func TestWithSourceStackWrapsInternalAppError(t *testing.T) {
	err := postgresSourceStackTestError()

	if !stderrors.Is(err, domainerrors.ErrPersistenceFailed) {
		t.Fatal("wrapped error should preserve errors.Is")
	}

	provider, ok := stderrors.AsType[observability.SourceStackProvider](err)
	if !ok {
		t.Fatal("internal app error should expose source stack")
	}
	stack := provider.SourceStack()
	if !strings.Contains(stack, "postgresSourceStackTestError") {
		t.Fatalf("stack should include source caller, got:\n%s", stack)
	}
	firstFrame, _, _ := strings.Cut(stack, "\n")
	if strings.Contains(firstFrame, "withSourceStack") {
		t.Fatalf("first stack frame should be the repository source caller, got %q", firstFrame)
	}
}

func TestWithSourceStackLeavesClientAppErrorUnwrapped(t *testing.T) {
	err := domainerrors.ErrAddressNotFound

	if _, ok := stderrors.AsType[observability.SourceStackProvider](err); ok {
		t.Fatal("client app error should not expose source stack")
	}
	if !stderrors.Is(err, domainerrors.ErrAddressNotFound) {
		t.Fatal("client app error should preserve errors.Is")
	}
}
