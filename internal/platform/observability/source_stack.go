package observability

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

const maxSourceStackFrames = 32

// SourceStackProvider exposes the call stack captured when an error was first
// marked for source-location logging.
type SourceStackProvider interface {
	error
	SourceStack() string
}

type sourceStackError struct {
	err   error
	pcs   []uintptr
	once  sync.Once
	stack string
}

func (e *sourceStackError) Error() string { return e.err.Error() }

func (e *sourceStackError) Unwrap() error { return e.err }

func (e *sourceStackError) SourceStack() string {
	e.once.Do(func() { e.stack = formatSourceStack(e.pcs) })

	return e.stack
}

// WithSourceStack captures the current call stack without changing the error chain.
func WithSourceStack(err error) error { return withSourceStack(err, 2) }

// WithSourceStackSkip captures the current call stack after skipping extra helper frames.
func WithSourceStackSkip(err error, skip int) error { return withSourceStack(err, skip+2) }

func withSourceStack(err error, skip int) error {
	if err == nil {
		return nil
	}

	if _, ok := stderrors.AsType[SourceStackProvider](err); ok {
		return err
	}

	return &sourceStackError{err: err, pcs: captureSourcePCs(skip)}
}

// UnwrapSourceStack removes only top-level source stack wrappers for cleaner logs.
func UnwrapSourceStack(err error) error {
	sourceErr, ok := err.(*sourceStackError) //nolint:errorlint // intentionally inspect only the top-level wrapper
	if !ok {
		return err
	}

	return sourceErr.Unwrap()
}

// CaptureSourceStack captures the current stack for fallback logging.
func CaptureSourceStack(skip int) string { return formatSourceStack(captureSourcePCs(skip + 1)) }

func captureSourcePCs(skip int) []uintptr {
	pcs := make([]uintptr, maxSourceStackFrames)
	callersCount := runtime.Callers(skip+2, pcs)
	if callersCount == 0 {
		return nil
	}

	return pcs[:callersCount]
}

func formatSourceStack(pcs []uintptr) string {
	if len(pcs) == 0 {
		return ""
	}
	var builder strings.Builder
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		builder.WriteString(frame.Function)
		builder.WriteByte('\n')
		fmt.Fprintf(&builder, "\t%s:%d", frame.File, frame.Line)
		if !more {
			break
		}
		builder.WriteByte('\n')
	}

	return builder.String()
}
