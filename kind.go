package errkind

import "strconv"

// Code is a 64-bit unsigned business error code. Zero is valid.
type Code uint64

// Kind describes a business error identity and creates instances of it.
// Define and Registry.Define return immutable, comparable implementations that
// match by pointer, even across Registries with identical codes and names.
// Copying a returned Kind preserves its identity. Other implementations may
// define their own behavior. The zero value is a nil interface and cannot be used.
// Use errors.As with a *Kind target to read an identity from an error tree.
type Kind interface {
	error

	// Code returns the business error code.
	Code() Code

	// Name returns the business error name.
	Name() string

	// New creates an Error with a literal message and optional attributes.
	// It copies the attribute slice, retaining the last value for each key at its
	// first insertion position.
	New(message string, attrs ...Attr) Error

	// Wrap creates an Error while preserving cause for errors.Is and errors.As.
	// Messages and attributes follow New. A nil cause returns a nil Error,
	// which remains nil when assigned to error.
	Wrap(cause error, message string, attrs ...Attr) Error
}

type kind struct {
	code Code
	name string
}

func (k *kind) Code() Code { return k.code }

func (k *kind) Name() string { return k.name }

func (k *kind) Error() string {
	return k.name + "(" + strconv.FormatUint(uint64(k.code), 10) + ")"
}

func (k *kind) New(message string, attrs ...Attr) Error {
	return k.newError(nil, message, attrs)
}

func (k *kind) Wrap(cause error, message string, attrs ...Attr) Error {
	if cause == nil {
		return nil
	}
	return k.newError(cause, message, attrs)
}

func (k *kind) newError(cause error, message string, attrs Attrs) *instance {
	return &instance{
		kind:    k,
		message: message,
		attrs:   attrs.Unique(),
		cause:   cause,
	}
}
