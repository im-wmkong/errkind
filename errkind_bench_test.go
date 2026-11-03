package errkind_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/im-wmkong/errkind"
)

var benchmarkError error
var benchmarkDetail errkind.Error
var benchmarkKind errkind.Kind
var benchmarkMatched bool

func BenchmarkNew(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "new")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkError = k.New("lookup user")
	}
}

func BenchmarkNewWithAttrs(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "attrs")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkError = k.New("lookup user", errkind.With("uid", 42), errkind.With("operation", "profile"), errkind.With("request", "abc"))
	}
}

func BenchmarkWrap(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "wrap")
	cause := errors.New("database")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkError = k.Wrap(cause, "lookup user")
	}
}

func BenchmarkMatch(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "match")
	leaf := k.New("")
	var deep error = leaf
	for i := 0; i < 32; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for name, err := range map[string]error{"single": leaf, "deep32": deep, "join": errors.Join(errors.New("left"), leaf)} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkMatched = errors.Is(err, k)
			}
		})
	}
}

func BenchmarkAs(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "as")
	leaf := k.New("lookup user")
	var deep error = leaf
	for i := 0; i < 32; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for name, err := range map[string]error{"single": leaf, "deep32": deep, "join": errors.Join(errors.New("left"), leaf)} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkMatched = errors.As(err, &benchmarkDetail)
			}
		})
	}
}

func BenchmarkAsKind(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "as_kind")
	leaf := k.New("lookup user")
	var deep error = leaf
	for i := 0; i < 32; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for name, err := range map[string]error{"bare": k, "single": leaf, "deep32": deep, "join": errors.Join(errors.New("left"), leaf)} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				benchmarkMatched = errors.As(err, &benchmarkKind)
			}
		})
	}
}
