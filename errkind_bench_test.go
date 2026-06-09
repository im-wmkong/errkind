package errkind_test

import (
	stderrors "errors"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/im-wmkong/errkind"
)

// 这些 benchmark 使用独立 Registry, 避免与功能测试冲突。
// 每个 Benchmark 内部 Define 一次, 然后在 b.N 循环里只跑创建/解析操作。

func BenchmarkNew_NoOpts(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.new_noopt")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.New()
	}
}

func BenchmarkNew_WithMessage(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.new_msg")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.New(errkind.Message("用户不存在"))
	}
}

func BenchmarkNew_With3Attrs(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.new_attrs")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.New(
			errkind.With("uid", 42),
			errkind.With("name", "alice"),
			errkind.With("trace", "abc-123"),
		)
	}
}

func BenchmarkWrap(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.wrap")
	cause := stderrors.New("boom")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.Wrap(cause, errkind.With("uid", 42))
	}
}

func BenchmarkNew_WithStackOn(b *testing.B) {
	errkind.SetCaptureStack(true)
	defer errkind.SetCaptureStack(false)

	r := errkind.NewRegistry()
	K := r.Define(1, "bench.new_stack_on")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.New()
	}
}

func BenchmarkKindIs(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.is")
	e := K.New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = K.Is(e)
	}
}

func BenchmarkCodeOf(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.codeof")
	e := K.New()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = errkind.CodeOf(e)
	}
}

func BenchmarkAllAttrs_Depth3(b *testing.B) {
	r := errkind.NewRegistry()
	A := r.Define(1, "bench.attrs_a")
	B := r.Define(2, "bench.attrs_b")
	C := r.Define(3, "bench.attrs_c")
	e := C.Wrap(
		B.Wrap(
			A.New(errkind.With("a", 1)),
			errkind.With("b", 2),
		),
		errkind.With("c", 3),
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = errkind.AllAttrs(e)
	}
}

func BenchmarkFormatV(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.fmt_v")
	e := K.New(errkind.Message("用户不存在"), errkind.With("uid", 42))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%v", e)
	}
}

func BenchmarkFormatPlusV_NoStack(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.fmt_plus")
	e := K.New(errkind.Message("用户不存在"), errkind.With("uid", 42))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%+v", e)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	r := errkind.NewRegistry()
	K := r.Define(1, "bench.json", errkind.DefaultMessage("默认"))
	e := K.Wrap(stderrors.New("root"),
		errkind.With("uid", 42),
		errkind.With("name", "alice"),
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(e)
	}
}
