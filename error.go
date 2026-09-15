package errkind

import (
	"fmt"
	"io"
	"strings"
)

type kerr struct {
	kind    *Kind
	message string
	attrs   []Attr
	cause   error
	pcs     []uintptr
}

func (e *kerr) Error() string {
	var b strings.Builder
	b.WriteString(e.kind.Error())
	if e.message != "" {
		b.WriteString(": ")
		b.WriteString(e.message)
	}
	if e.cause != nil {
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}
	return b.String()
}

func (e *kerr) Unwrap() error              { return e.cause }
func (e *kerr) Kind() *Kind                { return e.kind }
func (e *kerr) BusinessCode() (Code, bool) { return e.kind.code, true }
func (e *kerr) Name() string               { return e.kind.name }
func (e *kerr) Message() string            { return e.message }
func (e *kerr) Is(target error) bool       { return target == e.kind }
func (e *kerr) Attrs() []Attr              { return append([]Attr(nil), e.attrs...) }

// StackTrace returns this node's stack; StackOf also searches causes.
func (e *kerr) StackTrace() []Frame { return resolveFrames(e.pcs) }

func (e *kerr) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v', 's':
		_, _ = io.WriteString(s, e.Error())
		if verb == 'v' && s.Flag('+') {
			for _, f := range StackOf(e) {
				_, _ = io.WriteString(s, "\n"+f.String())
			}
		}
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
