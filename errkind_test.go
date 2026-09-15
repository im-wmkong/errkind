package errkind_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/im-wmkong/errkind"
)

var packageLevelKind = errkind.Define(900001, "pkg_level_define")

func TestIdentity(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(0, "a"), r.Define(2, "b")
	other := errkind.NewRegistry().Define(0, "a")
	cause := errors.New("root")
	first, second := a.New("first"), a.New("second")
	err := b.Wrap(fmt.Errorf("context: %w", errors.Join(a.Wrap(cause, "inner"), second)), "outer")
	if first == second || !errors.Is(err, a) || !errors.Is(err, b) || !errors.Is(err, cause) || errors.Is(err, other) {
		t.Fatal("identity or cause matching failed")
	}
	if errors.Is(a, first) || errors.Is(first, second) || errors.Is(nil, a) || a.Error() != "a(0)" {
		t.Fatal("matching must be directional and pointer-based")
	}
	var d errkind.Details
	if !errors.As(err, &d) || d.Name() != "b" || errkind.KindOf(err) != b {
		t.Fatal("outer instance must win")
	}
	if a.Wrap(nil, "ignored") != nil {
		t.Fatal("Wrap(nil) must return nil")
	}
}

func TestMessages(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	cause := errors.New("root")
	opts := []errkind.Option{errkind.With("uid", 42)}
	for _, msg := range []string{"", "查询用户失败", "literal %s", fmt.Sprintf("uid=%d", 42)} {
		t.Run(msg, func(t *testing.T) {
			for _, err := range []error{k.New(msg, opts...), k.Wrap(cause, msg, opts...)} {
				if errkind.MessageOf(err) != msg || err.(errkind.Details).Message() != msg {
					t.Fatalf("message changed: %v", err)
				}
				want := "failed(1)"
				if msg != "" {
					want += ": " + msg
				}
				if errors.Unwrap(err) != nil {
					if !errors.Is(err, cause) {
						t.Fatal("cause lost")
					}
					want += ": root"
				}
				if err.Error() != want || errkind.AttrsOf(err)[0].Val != 42 {
					t.Fatalf("details: %v", err)
				}
			}
		})
	}
}

func TestPrimary(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(0, "a"), r.Define(2, "b")
	leaf := a.New("internal", errkind.With("uid", 1))
	for _, err := range []error{leaf, fmt.Errorf("lookup: %w", leaf), errors.Join(leaf)} {
		if c, ok := errkind.CodeOf(err); !ok || c != 0 {
			t.Fatalf("zero code lost: %v %v", c, ok)
		}
		if n, ok := errkind.NameOf(err); !ok || n != "a" || errkind.KindOf(err) != a || errkind.MessageOf(err) != "internal" || len(errkind.AttrsOf(err)) != 1 {
			t.Fatal("primary details lost")
		}
	}
	for _, err := range []error{nil, errors.New("plain"), a, errors.Join(leaf, b.New("")), fmt.Errorf("both: %w %w", leaf, b.New(""))} {
		if _, ok := errkind.CodeOf(err); ok {
			t.Fatalf("unexpected primary: %v", err)
		}
		if _, ok := errkind.NameOf(err); ok || errkind.KindOf(err) != nil || errkind.AttrsOf(err) != nil {
			t.Fatalf("ambiguous or absent primary: %v", err)
		}
	}
	if errkind.MessageOf(nil) != "" || errkind.MessageOf(errors.New("plain")) != "plain" || errkind.MessageOf(a.New("")) != "" {
		t.Fatal("message fallback")
	}
	joined := errors.Join(leaf, b.New(""))
	if errkind.MessageOf(joined) != joined.Error() || errkind.KindOf(b.Wrap(joined, "outer")) != b {
		t.Fatal("join must need explicit classification")
	}
}

func TestAttrs(t *testing.T) {
	r := errkind.NewRegistry()
	a, b := r.Define(1, "a"), r.Define(2, "b")
	value := map[string]int{"n": 1}
	err := a.New(fmt.Sprintf("uid=%d", 2), errkind.With("uid", 1), errkind.With("map", value), errkind.With("uid", 2))
	attrs := errkind.AttrsOf(err)
	if len(attrs) != 2 || attrs[0].Key != "uid" || attrs[0].Val != 2 || errkind.MessageOf(err) != "uid=2" {
		t.Fatal("attribute order/override or message")
	}
	attrs[0].Val = 99
	d := err.(errkind.Details)
	copy := d.Attrs()
	copy[0].Val = 100
	if d.Attrs()[0].Val != 2 {
		t.Fatal("slice alias")
	}
	value["n"] = 3
	if d.Attrs()[1].Val.(map[string]int)["n"] != 3 {
		t.Fatal("values should not be deep-copied")
	}
	tree := b.Wrap(errors.Join(err, a.New("", errkind.With("right", true), errkind.With("uid", 4))), "", errkind.With("uid", 5))
	all := errkind.AllAttrs(fmt.Errorf("context: %w", tree))
	if len(all) != 3 || all[0].Val != 5 || all[2].Key != "right" {
		t.Fatalf("flat view: %v", all)
	}
	if errkind.AllAttrs(nil) != nil || errkind.AllAttrs(errors.New("plain")) != nil || errkind.AttrsOf(a.New("")) != nil {
		t.Fatal("empty attrs")
	}
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
	r := errkind.NewRegistry()
	k := r.Define(7, "lookup")
	if k.Code() != 7 || k.Name() != "lookup" || r.LookupCode(7) != k || r.LookupName("lookup") != k || r.LookupCode(8) != nil || r.LookupName("missing") != nil {
		t.Fatal("lookup")
	}
	list := r.Kinds()
	list[0] = nil
	if r.Kinds()[0] != k {
		t.Fatal("Kinds alias")
	}
	if errkind.DefaultRegistry().LookupCode(900001) != packageLevelKind || errkind.LookupCode(900001) != packageLevelKind || errkind.LookupName("pkg_level_define") != packageLevelKind || len(errkind.Kinds()) == 0 {
		t.Fatal("default registry")
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r.Define(errkind.Code(i+100), fmt.Sprint(i))
			_ = r.Kinds()
			_ = r.LookupCode(7)
		}(i)
	}
	wg.Wait()
}

type tracer struct{ frames []errkind.Frame }

func (t tracer) Error() string               { return "traced" }
func (t tracer) StackTrace() []errkind.Frame { return t.frames }

func TestStacks(t *testing.T) {
	plain := errkind.NewRegistry().Define(1, "plain")
	traced := errkind.NewRegistry(errkind.CaptureStack()).Define(1, "traced")
	if len(errkind.StackOf(plain.New(""))) != 0 || len(errkind.StackOf(nil)) != 0 {
		t.Fatal("default stacks")
	}
	inner := traced.New("")
	frames := errkind.StackOf(inner)
	if len(frames) == 0 || !strings.Contains(frames[0].Function, "TestStacks") {
		t.Fatalf("callsite: %v", frames)
	}
	for _, cause := range []error{inner, errors.Join(plain.New(""), inner), tracer{frames: []errkind.Frame{{Function: "fake", File: "f.go", Line: 1}}}} {
		wrapped := traced.Wrap(cause, "")
		if len(wrapped.(errkind.Tracer).StackTrace()) != 0 || len(errkind.StackOf(wrapped)) == 0 {
			t.Fatal("stack reuse")
		}
	}
	for _, cause := range []error{plain.New(""), tracer{}} {
		if len(traced.Wrap(cause, "").(errkind.Tracer).StackTrace()) == 0 {
			t.Fatal("empty tracer must not suppress capture")
		}
	}
	frames[0].File = "changed"
	if errkind.StackOf(inner)[0].File == "changed" {
		t.Fatal("stack slice alias")
	}
	if (errkind.Frame{Function: "f", File: "f.go", Line: 3}).String() != "f\n\tf.go:3" {
		t.Fatal("frame format")
	}
}

func TestFormattingAndNoImplicitJSON(t *testing.T) {
	for _, r := range []*errkind.Registry{errkind.NewRegistry(), errkind.NewRegistry(errkind.CaptureStack())} {
		k := r.Define(7, "boom")
		err := k.Wrap(errors.New("root"), "context")
		if err.Error() != "boom(7): context: root" || fmt.Sprintf("%v", err) != err.Error() || fmt.Sprintf("%s", err) != err.Error() || fmt.Sprintf("%q", err) != fmt.Sprintf("%q", err.Error()) {
			t.Fatal("format")
		}
		if len(errkind.StackOf(err)) > 0 && !strings.Contains(fmt.Sprintf("%+v", err), ".go:") {
			t.Fatal("formatted stack")
		}
		if _, ok := err.(json.Marshaler); ok {
			t.Fatal("implicit diagnostic JSON must not be exported")
		}
		raw, e := json.Marshal(err)
		if e != nil || string(raw) != "{}" {
			t.Fatalf("JSON: %s %v", raw, e)
		}
	}
}

type cyclic struct{}

func (*cyclic) Error() string   { return "cycle" }
func (c *cyclic) Unwrap() error { return c }
func TestTraversalBound(t *testing.T) {
	c := &cyclic{}
	if errkind.KindOf(c) != nil || errkind.AllAttrs(c) != nil || errkind.StackOf(c) != nil {
		t.Fatal("cycle")
	}
}
