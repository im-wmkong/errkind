// Package errkind separates error identity from instance diagnostics and public responses.
package errkind

type Code uint32

type Attr struct {
	Key string
	Val any
}

// Details exposes instance diagnostics; Attrs returns a shallow copy owned by the caller.
type Details interface {
	error
	BusinessCode() (Code, bool)
	Name() string
	Message() string
	Attrs() []Attr
}
