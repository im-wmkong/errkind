package diagnostic

import (
	"encoding/json"
	"fmt"

	"github.com/im-wmkong/errkind"
)

const MaxNodes = 256
const MaxDepth = 64

type Node struct {
	Code      *errkind.Code              `json:"code,omitempty"`
	Name      string                     `json:"name,omitempty"`
	Message   string                     `json:"message,omitempty"`
	Attrs     map[string]json.RawMessage `json:"attrs,omitempty"`
	Stack     []errkind.Frame            `json:"stack,omitempty"`
	Causes    []*Node                    `json:"causes,omitempty"`
	Truncated bool                       `json:"truncated,omitempty"`
}

func (n *Node) String() string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

func Build(err error) *Node {
	remaining := MaxNodes
	return build(err, 0, &remaining)
}

func build(err error, depth int, remaining *int) *Node {
	if err == nil {
		return nil
	}
	n := &Node{}
	*remaining--
	if d, ok := err.(errkind.Details); ok {
		if code, exists := d.BusinessCode(); exists {
			n.Code = &code
		}
		n.Name, n.Message = d.Name(), d.Message()
		for _, a := range d.Attrs() {
			if n.Attrs == nil {
				n.Attrs = make(map[string]json.RawMessage)
			}
			n.Attrs[a.Key] = Value(a.Val)
		}
	} else {
		n.Message = err.Error()
	}
	if tracer, ok := err.(errkind.Tracer); ok {
		n.Stack = tracer.StackTrace()
	}
	var children []error
	switch e := err.(type) {
	case interface{ Unwrap() error }:
		if child := e.Unwrap(); child != nil {
			children = []error{child}
		}
	case interface{ Unwrap() []error }:
		children = e.Unwrap()
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		if *remaining <= 0 || depth+1 >= MaxDepth {
			n.Truncated = true
			break
		}
		n.Causes = append(n.Causes, build(child, depth+1, remaining))
	}
	return n
}

// Encode isolates broken attribute encoders without discarding sibling diagnostics.
func Encode(err error) json.RawMessage {
	raw, _ := json.Marshal(Build(err))
	return raw
}

func Value(value any) (result json.RawMessage) {
	defer func() {
		if recover() != nil {
			result, _ = json.Marshal(fmt.Sprintf("<unencodable %T>", value))
		}
	}()
	raw, err := json.Marshal(value)
	if err != nil {
		raw, _ = json.Marshal(fmt.Sprintf("<unencodable %T>", value))
	}
	return raw
}
