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

func requireError(t *testing.T, err error) *errkind.Error {
	t.Helper()
	var detail *errkind.Error
	if !errors.As(err, &detail) {
		t.Fatalf("expected an *errkind.Error, got %T", err)
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
			if detail.Kind().Code() != test.code || !errors.Is(err, kind) {
				t.Fatal("code or identity changed")
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
	if detail.Kind().Code() != 0 || detail.Kind().Name() != "a" || detail.Message() != "internal" || len(detail.Attrs()) != 1 {
		t.Fatal("instance metadata changed")
	}
	right := b.New("right")
	outer := b.Wrap(errors.Join(leaf, right), "batch")
	deep := leaf
	for i := 0; i < 300; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for _, test := range []struct {
		name string
		err  error
		want *errkind.Error
	}{
		{"direct", leaf, leaf.(*errkind.Error)},
		{"wrapped", fmt.Errorf("lookup: %w", leaf), leaf.(*errkind.Error)},
		{"deep", deep, leaf.(*errkind.Error)},
		{"single_join", errors.Join(leaf), leaf.(*errkind.Error)},
		{"join_left", errors.Join(leaf, right), leaf.(*errkind.Error)},
		{"join_right", errors.Join(right, leaf), right.(*errkind.Error)},
		{"multiple_wrap", fmt.Errorf("both: %w %w", leaf, right), leaf.(*errkind.Error)},
		{"outer", outer, outer.(*errkind.Error)},
		{"bare_kind", a, nil},
		{"plain", errors.New("plain"), nil},
		{"nil", nil, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			var detail *errkind.Error
			if matched := errors.As(test.err, &detail); matched != (test.want != nil) || detail != test.want {
				t.Fatalf("got %p; want %p", detail, test.want)
			}
		})
	}
	var identity *errkind.Kind
	if !errors.As(fmt.Errorf("lookup: %w", a), &identity) || identity != a || !errors.Is(a, a) {
		t.Fatal("bare Kinds must retain standard error identity")
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
			if detail.Kind() != kind || detail.Message() != "query" || attrs[0].Value != 42 || detail.Unwrap() != cause || detail.Error() != "failed(1): query: database" {
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

func TestNilKindConstruction(t *testing.T) {
	var kind *errkind.Kind
	if kind.Wrap(nil, "ignored") != nil {
		t.Fatal("nil cause must remain a nil error")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("nil Kind cannot create a valid instance")
		}
	}()
	_ = kind.New("invalid receiver")
}
