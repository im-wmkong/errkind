package errkind

import "strings"

// Error describes a business error instance. Registered Kinds create private
// implementations whose fields can only be read through this interface.
// Attribute values and the underlying cause remain shared with their owners.
// Other implementations may define their own behavior. The zero value is a nil
// interface and cannot be used. Use errors.As with a pointer to Error to find an
// instance through standard wrappers and joined errors.
type Error interface {
	error

	// Kind returns the instance's stable identity.
	Kind() Kind

	// Code returns the business error code of the instance's Kind.
	Code() Code

	// Name returns the business error name of the instance's Kind.
	Name() string

	// Message returns the instance's literal message.
	Message() string

	// Attr returns this instance's value for key without searching the cause.
	// The boolean distinguishes a present nil value from a missing key.
	// Referenced values remain shared.
	Attr(key string) (any, bool)

	// Attrs returns a shallow copy in insertion order. Referenced values remain shared.
	Attrs() Attrs
}

type instance struct {
	kind    *kind
	message string
	attrs   Attrs
	cause   error
}

func (e *instance) Kind() Kind { return e.kind }

func (e *instance) Code() Code { return e.kind.Code() }

func (e *instance) Name() string { return e.kind.Name() }

func (e *instance) Message() string { return e.message }

func (e *instance) Attr(key string) (any, bool) { return e.attrs.Get(key) }

func (e *instance) Attrs() Attrs { return e.attrs.Clone() }

// Error returns the identity, message and cause as text.
func (e *instance) Error() string {
	var text strings.Builder
	for {
		text.WriteString(e.kind.Error())
		if e.message != "" {
			text.WriteString(": ")
			text.WriteString(e.message)
		}
		if e.cause == nil {
			break
		}
		text.WriteString(": ")
		// Only traverse our own instances; other errors control their text.
		next, ok := e.cause.(*instance)
		if !ok {
			text.WriteString(e.cause.Error())
			break
		}
		e = next
	}
	return text.String()
}

// Unwrap returns the underlying cause for errors.Unwrap, errors.Is and errors.As.
func (e *instance) Unwrap() error { return e.cause }

// Is reports whether target is the instance's Kind.
func (e *instance) Is(target error) bool { return target == e.kind }

// As exposes the instance's identity to a *Kind target. Used through errors.As,
// it selects this identity before searching the cause. Other targets follow
// the standard errors.As traversal rules.
func (e *instance) As(target any) bool {
	identity, ok := target.(*Kind)
	if !ok {
		return false
	}
	*identity = e.kind
	return true
}
