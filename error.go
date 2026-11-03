package errkind

// Error is an instance of a Kind with a message, attributes and an optional cause.
// Kind.New and Kind.Wrap create valid instances; the zero value is invalid.
// Use errors.As to find an *Error through standard wrappers and joined errors.
type Error struct {
	kind    *Kind
	message string
	attrs   Attrs
	cause   error
}

// Kind returns the instance's stable identity.
func (e *Error) Kind() *Kind { return e.kind }

// Message returns the instance's literal message.
func (e *Error) Message() string { return e.message }

// Attrs returns a shallow copy in insertion order. Referenced values remain shared.
func (e *Error) Attrs() Attrs { return e.attrs.Clone() }

// Error returns the identity, message and cause as text.
func (e *Error) Error() string {
	text := e.kind.Error()
	if e.message != "" {
		text += ": " + e.message
	}
	if e.cause != nil {
		text += ": " + e.cause.Error()
	}
	return text
}

// Unwrap returns the underlying cause for errors.Unwrap, errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }

// Is reports whether target is the instance's Kind.
func (e *Error) Is(target error) bool { return target == e.kind }
