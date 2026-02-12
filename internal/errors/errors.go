// Package errors provides a unified interface for error handling,
// combining stdlib errors with pkg/errors for stack trace support.
package errors

import (
	stderrors "errors"

	pkgerrors "github.com/pkg/errors"
)

// New returns an error that formats as the given text.
func New(text string) error {
	return stderrors.New(text)
}

// Is reports whether any error in err's tree matches target.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// As finds the first error in err's tree that matches target.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// Unwrap returns the result of calling the Unwrap method on err.
func Unwrap(err error) error {
	return stderrors.Unwrap(err)
}

// Join returns an error that wraps the given errors.
func Join(errs ...error) error {
	return stderrors.Join(errs...)
}

// AsType is a generic version of As that returns the typed error and a boolean.
// This is available in Go 1.26+.
func AsType[T error](err error) (T, bool) {
	return stderrors.AsType[T](err)
}

// Wrap returns an error annotating err with a stack trace and the supplied message.
func Wrap(err error, message string) error {
	return pkgerrors.Wrap(err, message)
}

// Wrapf returns an error annotating err with a stack trace and the format specifier.
func Wrapf(err error, format string, args ...any) error {
	return pkgerrors.Wrapf(err, format, args...)
}

// WithStack annotates err with a stack trace at the point WithStack was called.
func WithStack(err error) error {
	return pkgerrors.WithStack(err)
}

// WithMessage annotates err with a new message.
func WithMessage(err error, message string) error {
	return pkgerrors.WithMessage(err, message)
}

// Errorf formats according to a format specifier and returns the string as a
// value that satisfies error with stack trace.
func Errorf(format string, args ...any) error {
	return pkgerrors.Errorf(format, args...)
}

// Cause returns the underlying cause of the error, if possible.
//
//nolint:wrapcheck // Compatibility passthrough to preserve pkg/errors semantics.
func Cause(err error) error {
	return pkgerrors.Cause(err)
}
