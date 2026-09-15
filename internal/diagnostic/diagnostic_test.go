package diagnostic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind"
)

type panicValue struct{}

func (panicValue) MarshalJSON() ([]byte, error) { panic("bad encoder") }

type changingValue struct{ calls int }

func (v *changingValue) MarshalJSON() ([]byte, error) {
	v.calls++
	if v.calls > 1 {
		return nil, errors.New("second encoding")
	}
	return []byte(`42`), nil
}

type cycle struct{}

func (*cycle) Error() string   { return "cycle" }
func (c *cycle) Unwrap() error { return c }

func TestTree(t *testing.T) {
	r := errkind.NewRegistry(errkind.CaptureStack())
	a, b := r.Define(0, "a"), r.Define(2, "b")
	value := &changingValue{}
	left := a.Wrap(errors.New("database"), "left", errkind.With("uid", 1), errkind.With("broken", make(chan int)), errkind.With("panic", panicValue{}), errkind.With("once", value))
	right := b.New("right", errkind.With("uid", 2))
	n := Build(fmt.Errorf("batch: %w", errors.Join(left, right)))
	if n.Code != nil || !strings.Contains(n.Message, "batch:") || len(n.Causes) != 1 || len(n.Causes[0].Causes) != 2 {
		t.Fatalf("tree: %+v", n)
	}
	l, rr := n.Causes[0].Causes[0], n.Causes[0].Causes[1]
	if l.Code == nil || *l.Code != 0 || l.Name != "a" || l.Message != "left" || string(l.Attrs["uid"]) != "1" || string(rr.Attrs["uid"]) != "2" || l.Causes[0].Message != "database" {
		t.Fatal("node association")
	}
	if len(l.Stack) == 0 || len(rr.Stack) == 0 || len(n.Stack) != 0 {
		t.Fatal("stack ownership")
	}
	for _, key := range []string{"broken", "panic"} {
		if !strings.Contains(string(l.Attrs[key]), "unencodable") {
			t.Fatal("attribute fallback")
		}
	}
	raw, err := json.Marshal(n)
	if err != nil || !json.Valid(raw) || value.calls != 1 || !strings.Contains(string(raw), `"once":42`) {
		t.Fatalf("encoding: %s %v %d", raw, err, value.calls)
	}
	if string(Encode(nil)) != "null" || (*Node)(nil).String() != "null" {
		t.Fatal("nil")
	}
	if n.String() != string(raw) {
		t.Fatal("text diagnostic differs from JSON")
	}
}

func TestBounds(t *testing.T) {
	n := Build(&cycle{})
	count := 1
	for len(n.Causes) > 0 {
		n = n.Causes[0]
		count++
	}
	if count != MaxDepth || !n.Truncated {
		t.Fatalf("depth: %d %+v", count, n)
	}
	children := make([]error, MaxNodes+20)
	for i := range children {
		children[i] = errors.New("leaf")
	}
	n = Build(errors.Join(children...))
	if len(n.Causes) != MaxNodes-1 || !n.Truncated {
		t.Fatalf("width: %d", len(n.Causes))
	}
}

func BenchmarkEncodeTree(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "bench")
	err := k.Wrap(errors.Join(k.New("", errkind.With("uid", 1)), errors.New("database")), "failed")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Encode(err)
	}
}
