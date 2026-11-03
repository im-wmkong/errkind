package errkind

import "strconv"

// Code is a 64-bit unsigned business error code. Zero is valid.
type Code uint64

// Kind is a stable identity within a Registry. It is also a valid context-free error.
// Definitions are immutable and match by pointer, even across Registries with
// identical codes and names. Create them with Define or Registry.Define.
type Kind struct {
	code Code
	name string
}

// Code returns the business error code.
func (k *Kind) Code() Code { return k.code }

// Name returns the business error name.
func (k *Kind) Name() string { return k.name }

// Error returns the identity as name(code).
func (k *Kind) Error() string {
	return k.name + "(" + strconv.FormatUint(uint64(k.code), 10) + ")"
}

// New creates an *Error with a literal message and optional attributes.
// It copies the attribute slice, retaining the last value for each key at its
// first insertion position. A nil Kind panics.
func (k *Kind) New(message string, attrs ...Attr) error {
	return k.newError(nil, message, attrs)
}

// Wrap creates an *Error while preserving cause for errors.Is and errors.As.
// Messages and attributes follow New. A nil cause returns nil, including when
// the Kind is nil. A nil Kind with a non-nil cause panics.
func (k *Kind) Wrap(cause error, message string, attrs ...Attr) error {
	if cause == nil {
		return nil
	}
	return k.newError(cause, message, attrs)
}

func (k *Kind) newError(cause error, message string, attrs Attrs) *Error {
	if k == nil {
		panic("errkind: cannot create an instance from a nil Kind")
	}
	return &Error{
		kind:    k,
		message: message,
		attrs:   attrs.Unique(),
		cause:   cause,
	}
}
