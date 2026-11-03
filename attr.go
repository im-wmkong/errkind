package errkind

import "slices"

// Attr records an instance parameter. Values are retained without a deep copy.
type Attr struct {
	Key   string
	Value any
}

// Attrs is an ordered collection of instance attributes.
type Attrs []Attr

// With creates an attribute. Constructors retain the last value for a key
// at its first insertion position.
func With(key string, value any) Attr {
	return Attr{Key: key, Value: value}
}

// Get returns the last value for key and whether it exists.
// A present nil value returns (nil, true); a missing key returns (nil, false).
// Referenced values remain shared.
func (attrs Attrs) Get(key string) (any, bool) {
	for i := len(attrs) - 1; i >= 0; i-- {
		if attrs[i].Key == key {
			return attrs[i].Value, true
		}
	}
	return nil, false
}

// Clone returns a shallow copy, preserving all attributes and their order.
func (attrs Attrs) Clone() Attrs { return slices.Clone(attrs) }

// Unique returns a shallow copy with one attribute per key. The last value wins
// at the key's first insertion position. Empty collections return nil.
func (attrs Attrs) Unique() Attrs {
	if len(attrs) == 0 {
		return nil
	}
	result := make(Attrs, 0, len(attrs))
	for _, attr := range attrs {
		index := slices.IndexFunc(result, func(existing Attr) bool {
			return existing.Key == attr.Key
		})
		if index >= 0 {
			result[index] = attr
		} else {
			result = append(result, attr)
		}
	}
	return result
}
