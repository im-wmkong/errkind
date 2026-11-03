package errkind_test

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/im-wmkong/errkind"
)

var packageLevelKind = errkind.Define(900001, "pkg_level_define")

func requireError(t *testing.T, err error) errkind.Error {
	t.Helper()
	var detail errkind.Error
	if !errors.As(err, &detail) {
		t.Fatalf("expected an errkind.Error, got %T", err)
	}
	return detail
}

func TestCodeRange(t *testing.T) {
	registry := errkind.NewRegistry()
	for _, test := range []struct {
		code errkind.Code
		text string
	}{
		{0, "0"},
		{1, "1"},
		{1<<32 - 1, "4294967295"},
		{1 << 32, "4294967296"},
		{1<<32 + 1, "4294967297"},
		{1<<64 - 1, "18446744073709551615"},
	} {
		t.Run(test.text, func(t *testing.T) {
			name := "code_" + test.text
			kind := registry.Define(test.code, name)
			err := kind.New("failed")
			detail := requireError(t, err)
			if detail.Code() != test.code || detail.Name() != name {
				t.Fatal("instance code or name changed")
			}
			if detail.Code() != detail.Kind().Code() || detail.Name() != detail.Kind().Name() || !errors.Is(err, kind) {
				t.Fatal("instance metadata must match its Kind")
			}
			if kind.Error() != name+"("+test.text+")" || err.Error() != kind.Error()+": failed" {
				t.Fatal("error text lost code precision")
			}
		})
	}
}

func TestIdentityAndCauses(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(0, "a"), r.Define(2, "b")
	other := errkind.NewRegistry().Define(0, "a")
	cause := &os.PathError{Op: "read", Path: "user", Err: os.ErrNotExist}
	first, second := a.New("first"), a.New("second")
	joined := errors.Join(a.Wrap(cause, "inner"), second)
	err := b.Wrap(fmt.Errorf("context: %w", joined), "outer")
	if first == second || !errors.Is(err, a) || !errors.Is(err, b) || !errors.Is(err, cause) || !errors.Is(err, os.ErrNotExist) || errors.Is(err, other) {
		t.Fatal("identity or cause matching failed")
	}
	if errors.Is(a, first) || errors.Is(first, second) || errors.Is(nil, a) || a.Error() != "a(0)" {
		t.Fatal("matching must be directional and pointer-based")
	}
	var pathError *os.PathError
	if !errors.As(err, &pathError) || pathError != cause {
		t.Fatal("standard errors.As lost the underlying cause")
	}
	detail := requireError(t, err)
	if detail.Kind() != b || detail.Message() != "outer" {
		t.Fatal("outer instance must win")
	}
	if a.Wrap(nil, "ignored") != nil || errors.Unwrap(a.Wrap(cause, "")) != cause {
		t.Fatal("Wrap must preserve nil and non-nil causes")
	}
}

func TestErrorResults(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	detail := k.New("query", errkind.With("uid", 42))
	if detail.Kind() != k || detail.Message() != "query" || detail.Attrs()[0].Value != 42 {
		t.Fatal("New must return directly readable instance details")
	}
	cause := errors.New("database")
	wrapped := k.Wrap(cause, "lookup")
	if wrapped.Kind() != k || wrapped.Code() != k.Code() || wrapped.Name() != k.Name() || wrapped.Message() != "lookup" || errors.Unwrap(wrapped) != cause {
		t.Fatal("Wrap must return directly readable details and preserve its cause")
	}
	var err error = wrapped
	if requireError(t, err) != wrapped || !errors.Is(err, cause) || !errors.Is(err, k) {
		t.Fatal("conversion to error must preserve the instance and its error chain")
	}
}

func TestWrapNilResult(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	var detail errkind.Error = k.Wrap(nil, "ignored", errkind.With("uid", 42))
	var err error = detail
	if detail != nil || err != nil || errors.Join(err) != nil {
		t.Fatal("Wrap(nil) must remain nil after conversion to error")
	}
	returned := func() error { return k.Wrap(nil, "ignored") }()
	if returned != nil {
		t.Fatal("returning Wrap(nil) from an error-returning function must remain nil")
	}
}

func TestErrorOwnership(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	cause := errors.New("database")
	original := k.Wrap(cause, "first", errkind.With("uid", 42))
	copied := original
	if copied != original {
		t.Fatal("copying an Error must preserve the instance")
	}
	chain := fmt.Errorf("context: %w", copied)
	copied = k.New("second")
	if copied == original || requireError(t, chain) != original {
		t.Fatal("reassignment must not replace an instance held by a wrapper")
	}
	if original.Message() != "first" || original.Attrs()[0].Value != 42 || errors.Unwrap(original) != cause {
		t.Fatal("reassignment changed the original instance")
	}
}

func TestMessages(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	cause := errors.New("root")
	attrs := errkind.Attrs{errkind.With("uid", 42)}
	for _, message := range []string{"", "查询用户失败", "literal %s", fmt.Sprintf("uid=%d", 42)} {
		t.Run(message, func(t *testing.T) {
			for _, err := range []error{k.New(message, attrs...), k.Wrap(cause, message, attrs...)} {
				detail := requireError(t, err)
				if detail.Message() != message || detail.Attrs()[0].Value != 42 {
					t.Fatalf("message or parameters changed: %q %+v", detail.Message(), detail.Attrs())
				}
				want := "failed(1)"
				if message != "" {
					want += ": " + message
				}
				if errors.Unwrap(err) != nil {
					want += ": root"
				}
				if err.Error() != want {
					t.Fatalf("got %q; want %q", err, want)
				}
			}
		})
	}
}

func TestErrorAs(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(0, "a"), r.Define(2, "b")
	leaf := a.New("internal", errkind.With("uid", 1))
	detail := requireError(t, leaf)
	if detail.Code() != 0 || detail.Name() != "a" || detail.Message() != "internal" || len(detail.Attrs()) != 1 {
		t.Fatal("instance metadata changed")
	}
	right := b.New("right")
	outer := b.Wrap(errors.Join(leaf, right), "batch")
	var deep error = leaf
	for i := 0; i < 300; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for _, test := range []struct {
		name string
		err  error
		want errkind.Error
	}{
		{"direct", leaf, leaf},
		{"wrapped", fmt.Errorf("lookup: %w", leaf), leaf},
		{"deep", deep, leaf},
		{"single_join", errors.Join(leaf), leaf},
		{"join_left", errors.Join(leaf, right), leaf},
		{"join_right", errors.Join(right, leaf), right},
		{"multiple_wrap", fmt.Errorf("both: %w %w", leaf, right), leaf},
		{"outer", outer, outer},
		{"bare_kind", a, nil},
		{"plain", errors.New("plain"), nil},
		{"nil", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			var detail errkind.Error
			if matched := errors.As(test.err, &detail); matched != (test.want != nil) || detail != test.want {
				t.Fatalf("got %p; want %p", detail, test.want)
			}
		})
	}
	var identity errkind.Kind
	if !errors.As(fmt.Errorf("lookup: %w", a), &identity) || identity != a || !errors.Is(a, a) {
		t.Fatal("bare Kinds must retain standard error identity")
	}
}

func TestKindAs(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(1, "a"), r.Define(2, "b")
	leaf := a.New("leaf")
	var deep error = leaf
	for i := 0; i < 32; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for _, test := range []struct {
		name string
		err  error
		want errkind.Kind
	}{
		{"bare", a, a},
		{"wrapped_bare", fmt.Errorf("lookup: %w", a), a},
		{"instance", leaf, a},
		{"wrapped_instance", fmt.Errorf("lookup: %w", leaf), a},
		{"deep", deep, a},
		{"single_join", errors.Join(leaf), a},
		{"outer_bare", b.Wrap(a, "outer"), b},
		{"outer_instance", b.Wrap(leaf, "outer"), b},
		{"join_bare_instance", errors.Join(a, b.New("right")), a},
		{"join_instance_bare", errors.Join(b.New("left"), a), b},
		{"join_plain_bare", errors.Join(errors.New("plain"), a), a},
		{"join_plain_instance", errors.Join(errors.New("plain"), leaf), a},
		{"outer_join", b.Wrap(errors.Join(a, leaf), "outer"), b},
		{"depth_first", errors.Join(fmt.Errorf("context: %w", leaf), b), a},
		{"multiple_wrap", fmt.Errorf("both: %w %w", a, b.New("right")), a},
		{"plain", errors.New("plain"), nil},
		{"nil", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			var identity errkind.Kind
			if matched := errors.As(test.err, &identity); matched != (test.want != nil) || identity != test.want {
				t.Fatalf("got %v; want %v", identity, test.want)
			}
		})
	}
}

func TestKindAndInstanceQueries(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(1, "a"), r.Define(2, "b")
	instance := b.New("right", errkind.With("source", "b"))
	joined := errors.Join(a, instance)
	var identity errkind.Kind
	if !errors.As(joined, &identity) || identity != a {
		t.Fatal("identity query must include bare Kinds")
	}
	if detail := requireError(t, joined); detail.Kind() != b || detail != instance || detail.Attrs()[0].Value != "b" {
		t.Fatal("instance query must find the first instance")
	}
	outer := a.Wrap(instance, "outer")
	if !errors.Is(outer, b) || requireError(t, outer).Kind() != a {
		t.Fatal("matching a cause must not change instance selection")
	}
}

func TestKindIdentityOwnership(t *testing.T) {
	r := errkind.NewRegistry()
	original := r.Define(1, "a")
	other := r.Define(2, "b")
	copied := original
	err := copied.New("created before reassignment")
	if copied != original || !errors.Is(err, original) || requireError(t, err).Kind() != copied {
		t.Fatal("copying a Kind must preserve its identity")
	}
	copied = other
	catalog := r.Kinds()
	catalog[0] = other
	if copied != other || requireError(t, err).Kind() != original || r.Kinds()[0] != original {
		t.Fatal("reassignment must not change an existing definition")
	}
	if err.Error() != "a(1): created before reassignment" || errors.Is(err, other) {
		t.Fatal("existing error metadata or matching changed")
	}
	if original.Code() != 1 || original.Name() != "a" {
		t.Fatal("definition metadata changed")
	}
}

func TestAttrsOwnershipAndOverrides(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "a")
	value := map[string]int{"n": 1}
	attrs := errkind.Attrs{errkind.With("uid", 1), errkind.With("map", value), errkind.With("uid", 2)}
	cloned := attrs.Clone()
	if len(cloned) != 3 || cloned[0].Value != 1 || cloned[2].Value != 2 {
		t.Fatal("Clone must preserve duplicate keys and their values")
	}
	cloned[0].Value = 99
	if attrs[0].Value != 1 {
		t.Fatal("Clone must own its slice")
	}
	err := k.New("query", attrs...)
	attrs[0].Value = 99
	detail := requireError(t, err)
	parameters := detail.Attrs()
	if len(parameters) != 2 || parameters[0].Key != "uid" || parameters[0].Value != 2 || parameters[1].Key != "map" {
		t.Fatalf("attribute order or override: %+v", parameters)
	}
	parameters[0].Value = 100
	again := detail.Attrs()
	if again[0].Value != 2 {
		t.Fatal("Attrs must return its own slice")
	}
	value["n"] = 3
	if again[1].Value.(map[string]int)["n"] != 3 {
		t.Fatal("values are shallow copies")
	}
	if empty := requireError(t, k.New("")); empty.Attrs() != nil {
		t.Fatal("empty attributes")
	}
}

func TestErrorConcurrentReaders(t *testing.T) {
	kind := errkind.NewRegistry().Define(1, "failed")
	cause := errors.New("database")
	detail := requireError(t, kind.Wrap(cause, "query", errkind.With("uid", 42)))
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attrs := detail.Attrs()
			var identity errkind.Kind
			if !errors.As(detail, &identity) || identity != kind {
				t.Error("concurrent identity extraction changed the Kind")
			}
			if detail.Kind() != kind || detail.Message() != "query" || attrs[0].Value != 42 || errors.Unwrap(detail) != cause || detail.Error() != "failed(1): query: database" {
				t.Error("concurrent read changed error details")
			}
			attrs[0].Value = 99
		}()
	}
	wg.Wait()
}

func TestRegistry(t *testing.T) {
	for _, duplicate := range []struct {
		code errkind.Code
		name string
	}{{1, "b"}, {2, "a"}, {3, ""}} {
		t.Run(fmt.Sprint(duplicate), func(t *testing.T) {
			r := errkind.NewRegistry()
			r.Define(1, "a")
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			r.Define(duplicate.code, duplicate.name)
		})
	}
	var r errkind.Registry
	if r.Kinds() != nil {
		t.Fatal("empty registry catalog must be nil")
	}
	a, b := r.Define(7, "a"), r.Define(8, "b")
	list := r.Kinds()
	if len(list) != 2 || list[0] != a || list[1] != b {
		t.Fatal("catalog order")
	}
	list[0] = nil
	if r.Kinds()[0] != a {
		t.Fatal("catalog slice alias")
	}
	found := false
	for _, kind := range errkind.Kinds() {
		found = found || kind == packageLevelKind
	}
	if !found {
		t.Fatal("default registry catalog")
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Define(errkind.Code(i+100), fmt.Sprint(i))
			_ = r.Kinds()
		}(i)
	}
	wg.Wait()
	if len(r.Kinds()) != 34 {
		t.Fatal("concurrent registrations lost")
	}
}

func TestStandardFormatting(t *testing.T) {
	k := errkind.NewRegistry().Define(7, "boom")
	err := k.Wrap(errors.New("root"), "context")
	for _, format := range []string{"%v", "%s", "%+v"} {
		if fmt.Sprintf(format, err) != "boom(7): context: root" {
			t.Fatal("standard formatting")
		}
	}
	if fmt.Sprintf("%q", err) != fmt.Sprintf("%q", err.Error()) {
		t.Fatal("quoted format")
	}
}

func TestNilKind(t *testing.T) {
	var kind errkind.Kind
	for _, test := range []struct {
		name string
		call func()
	}{
		{"new", func() { _ = kind.New("invalid receiver") }},
		{"wrap_nil", func() { _ = kind.Wrap(nil, "invalid receiver") }},
		{"wrap_cause", func() { _ = kind.Wrap(errors.New("cause"), "invalid receiver") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("calling a nil Kind must panic")
				}
			}()
			test.call()
		})
	}
}
