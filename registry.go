package errkind

import (
	"fmt"
	"sync"
)

// Registry 持有一组 Kind, 保证 (code, name) 在自身范围内唯一。
//
// 通常使用包级 Define / Kinds / LookupCode / LookupName 即可;
// 测试或多租户场景可以 NewRegistry() 创建独立注册中心。
type Registry struct {
	mu           sync.RWMutex
	byCode       map[Code]*Kind
	byName       map[string]*Kind
	all          []*Kind
	captureStack bool
}

type registryConfig struct{ captureStack bool }

type RegistryOption func(*registryConfig)

func CaptureStack() RegistryOption {
	return func(c *registryConfig) { c.captureStack = true }
}

func NewRegistry(opts ...RegistryOption) *Registry {
	var config registryConfig
	for _, option := range opts {
		option(&config)
	}
	return &Registry{
		byCode:       map[Code]*Kind{},
		byName:       map[string]*Kind{},
		captureStack: config.captureStack,
	}
}

// Define 注册并返回一个新的 Kind; 重复 code/name 立即 panic。
func (r *Registry) Define(code Code, name string) *Kind {
	if name == "" {
		panic("errkind: Define name must not be empty")
	}
	k := &Kind{code: code, name: name, registry: r}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existed, ok := r.byCode[code]; ok {
		panic(fmt.Sprintf("errkind: duplicate code %d (registered as %q, new %q)",
			code, existed.name, name))
	}
	if existed, ok := r.byName[name]; ok {
		panic(fmt.Sprintf("errkind: duplicate name %q (registered with code %d, new %d)",
			name, existed.code, code))
	}
	r.byCode[code] = k
	r.byName[name] = k
	r.all = append(r.all, k)
	return k
}

// Kinds 返回所有已注册 Kind, 按注册顺序; 用于生成错误码文档。
func (r *Registry) Kinds() []*Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Kind, len(r.all))
	copy(out, r.all)
	return out
}

// LookupCode 按 code 查找; 不存在返回 nil。
func (r *Registry) LookupCode(c Code) *Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byCode[c]
}

// LookupName 按 name 查找; 不存在返回 nil。
func (r *Registry) LookupName(n string) *Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byName[n]
}

var defaultRegistry = NewRegistry()

func DefaultRegistry() *Registry { return defaultRegistry }

func Define(code Code, name string) *Kind {
	return defaultRegistry.Define(code, name)
}

// Kinds 返回默认注册中心的所有 Kind。
func Kinds() []*Kind { return defaultRegistry.Kinds() }

// LookupCode 默认注册中心查询。
func LookupCode(c Code) *Kind { return defaultRegistry.LookupCode(c) }

// LookupName 默认注册中心查询。
func LookupName(n string) *Kind { return defaultRegistry.LookupName(n) }
