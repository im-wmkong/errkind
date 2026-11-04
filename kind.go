package errkind

import "strconv"

// Kind describes a business error identity and creates instances of it.
// Define and Registry.Define return immutable, comparable implementations that
// match by pointer, even across Registries with identical codes and names.
// Copying a returned Kind preserves its identity. Other implementations may
// define their own behavior. The zero value is a nil interface and cannot be used.
// Use Kind for definitions, construction and errors.Is matching. Return New or
// Wrap instances from business operations, using New("") without a message.
// A Kind also implements error and can be extracted with a *Kind errors.As target.
// At application boundaries, extract an Error and read its Kind and details.
type Kind interface {
	error

	// Code returns the business error code. Zero is valid.
	Code() uint64

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
	code uint64
	name string
}

func (k *kind) Code() uint64 { return k.code }

func (k *kind) Name() string { return k.name }

func (k *kind) Error() string {
	return k.name + "(" + strconv.FormatUint(k.code, 10) + ")"
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
