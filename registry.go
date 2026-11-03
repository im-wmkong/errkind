package errkind

import (
	"fmt"
	"sync"
)

// Registry isolates identities and enforces unique codes and names.
// Its zero value is ready to use. A Registry must not be copied after use.
type Registry struct {
	mu     sync.RWMutex
	byCode map[Code]*kind
	byName map[string]*kind
	kinds  []*kind
}

// NewRegistry creates an empty Registry with an independent error catalog.
func NewRegistry() *Registry { return &Registry{} }

// Define registers a Kind. An empty name or duplicate code/name panics.
func (r *Registry) Define(code Code, name string) Kind {
	if name == "" {
		panic("errkind: Define name must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if registered := r.byCode[code]; registered != nil {
		panic(fmt.Sprintf("errkind: duplicate code %d (registered as %q, new %q)", code, registered.name, name))
	}
	if registered := r.byName[name]; registered != nil {
		panic(fmt.Sprintf("errkind: duplicate name %q (registered with code %d, new %d)", name, registered.code, code))
	}
	if r.byCode == nil {
		r.byCode = make(map[Code]*kind)
	}
	if r.byName == nil {
		r.byName = make(map[string]*kind)
	}
	k := &kind{code: code, name: name}
	r.byCode[code], r.byName[name] = k, k
	r.kinds = append(r.kinds, k)
	return k
}

// Kinds returns a copy in registration order, suitable for application-owned catalogs.
func (r *Registry) Kinds() []Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.kinds) == 0 {
		return nil
	}
	result := make([]Kind, len(r.kinds))
	for i, k := range r.kinds {
		result[i] = k
	}
	return result
}

var defaultRegistry = NewRegistry()

// Define registers an identity in the default Registry.
func Define(code Code, name string) Kind { return defaultRegistry.Define(code, name) }

// Kinds returns the default Registry's catalog.
func Kinds() []Kind { return defaultRegistry.Kinds() }
