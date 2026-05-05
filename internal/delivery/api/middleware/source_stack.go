package middleware

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

const maxSourceStackFrames = 32

type sourceStackProvider interface {
	SourceStack() string
}

type sourceStackError struct {
	err   error
	stack string
}

func (e *sourceStackError) Error() string {
	return e.err.Error()
}

func (e *sourceStackError) Unwrap() error {
	return e.err
}

func (e *sourceStackError) SourceStack() string {
	return e.stack
}

// WithSourceStack captures the current call stack for centralized 5xx request logs.
func WithSourceStack(err error) error {
	if err == nil {
		return nil
	}

	var existing sourceStackProvider
	if errors.As(err, &existing) {
		return err
	}

	return &sourceStackError{
		err:   err,
		stack: captureSourceStack(1),
	}
}

func captureSourceStack(skip int) string {
	pcs := make([]uintptr, maxSourceStackFrames)
	callersCount := runtime.Callers(skip+2, pcs)
	if callersCount == 0 {
		return ""
	}

	var builder strings.Builder
	frames := runtime.CallersFrames(pcs[:callersCount])
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
