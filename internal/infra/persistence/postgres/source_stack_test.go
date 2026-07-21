package postgres

import (
	stderrors "errors"
	"strings"
	"testing"

	domainerrors "radar/internal/domain/errors"

	"github.com/slighter12/go-lib/errors/stack"
)

func postgresSourceStackTestError() error {
	return withSourceStack(domainerrors.ErrPersistenceFailed)
}

func TestWithSourceStackWrapsInternalAppError(t *testing.T) {
	err := postgresSourceStackTestError()

	if !stderrors.Is(err, domainerrors.ErrPersistenceFailed) {
		t.Fatal("wrapped error should preserve errors.Is")
	}

	provider, ok := stderrors.AsType[stack.Provider](err)
	if !ok {
		t.Fatal("internal app error should expose source stack")
	}
	frames := provider.Stack()
	if !strings.Contains(frames, "postgresSourceStackTestError") {
		t.Fatalf("stack should include source caller, got:\n%s", frames)
	}
	firstFrame, _, _ := strings.Cut(frames, "; ")
	if strings.Contains(firstFrame, "withSourceStack") {
		t.Fatalf("first stack frame should be the repository source caller, got %q", firstFrame)
	}
}

func TestWithSourceStackWrapsClientAppError(t *testing.T) {
	err := withSourceStack(domainerrors.ErrAddressNotFound)

	if _, ok := stderrors.AsType[stack.Provider](err); !ok {
		t.Fatal("client app error should expose source stack")
	}
	if !stderrors.Is(err, domainerrors.ErrAddressNotFound) {
		t.Fatal("client app error should preserve errors.Is")
	}
}

func TestReplaceWithSourceStackPreservesClassificationAndCause(t *testing.T) {
	cause := stderrors.New("database failed")
	err := replaceWithSourceStack(cause, domainerrors.ErrPersistenceFailed)

	if !stderrors.Is(err, domainerrors.ErrPersistenceFailed) {
		t.Fatal("replacement error should preserve the outer app error")
	}
	if !stderrors.Is(err, cause) {
		t.Fatal("replacement error should preserve the original cause")
	}
	appErr, ok := stderrors.AsType[domainerrors.AppError](err)
	if !ok || appErr.ErrorCode() != domainerrors.ErrPersistenceFailed.ErrorCode() {
		t.Fatal("replacement error should expose the outer app error")
	}
	provider, ok := stderrors.AsType[stack.Provider](err)
	if !ok {
		t.Fatal("replacement error should expose a source stack")
	}
	firstFrame, _, _ := strings.Cut(provider.Stack(), "; ")
	if strings.Contains(firstFrame, "replaceWithSourceStack") {
		t.Fatalf("first stack frame should be the repository source caller, got %q", firstFrame)
	}
}
