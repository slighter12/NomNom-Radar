package observability

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

func sourceStackTestError() error { return WithSourceStack(stderrors.New("boom")) }

func TestWithSourceStackCapturesCallerAndPreservesChain(t *testing.T) {
	inner := stderrors.New("boom")
	err := WithSourceStack(inner)
	if !stderrors.Is(err, inner) {
		t.Fatal("wrapped error should preserve errors.Is")
	}
	if _, ok := stderrors.AsType[SourceStackProvider](err); !ok {
		t.Fatal("wrapped error should expose source stack")
	}
	err = sourceStackTestError()
	provider, _ := stderrors.AsType[SourceStackProvider](err)
	stack := provider.SourceStack()
	if !strings.Contains(stack, "sourceStackTestError") {
		t.Fatalf("stack should include source caller, got:\n%s", stack)
	}
	firstFrame, _, _ := strings.Cut(stack, "\n")
	if strings.Contains(firstFrame, "WithSourceStack") || strings.Contains(firstFrame, "withSourceStack") {
		t.Fatalf("first stack frame should be the source caller, got %q", firstFrame)
	}
}

func TestWithSourceStackDoesNotDoubleWrap(t *testing.T) {
	err := sourceStackTestError()
	provider, _ := stderrors.AsType[SourceStackProvider](err)
	wrappedProvider, _ := stderrors.AsType[SourceStackProvider](WithSourceStack(err))
	if wrappedProvider.SourceStack() != provider.SourceStack() {
		t.Fatal("WithSourceStack should keep the first captured stack")
	}
}

func TestUnwrapSourceStackOnlyRemovesTopLevelStackWrapper(t *testing.T) {
	inner := stderrors.New("boom")
	wrapped := WithSourceStack(inner)
	if got := UnwrapSourceStack(wrapped); !stderrors.Is(got, inner) {
		t.Fatalf("top-level source stack should unwrap to inner error, got %T", got)
	}
	if _, ok := stderrors.AsType[SourceStackProvider](UnwrapSourceStack(wrapped)); ok {
		t.Fatal("top-level source stack should be removed")
	}

	nested := fmt.Errorf("outer: %w", wrapped)
	got := UnwrapSourceStack(nested)
	if !stderrors.Is(got, nested) {
		t.Fatalf("nested source stack should preserve outer context, got %T", got)
	}
	if _, ok := stderrors.AsType[SourceStackProvider](got); !ok {
		t.Fatal("nested source stack should be preserved")
	}
}
