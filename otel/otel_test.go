package otel_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/im-wmkong/errkind"
	otelint "github.com/im-wmkong/errkind/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

func attrMap(attrs []attribute.KeyValue) map[string]attribute.Value {
	m := map[string]attribute.Value{}
	for _, a := range attrs {
		m[string(a.Key)] = a.Value
	}
	return m
}
func TestRecord(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer tp.Shutdown(context.Background())
	_, span := tp.Tracer("test").Start(context.Background(), "op")
	k := errkind.NewRegistry().Define(0, "failed")
	err := k.Wrap(errors.New("db"), "internal", errkind.With("uid", 42))
	otelint.RecordError(span, err)
	span.End()
	s := rec.Ended()[0]
	a := attrMap(s.Attributes())
	if s.Status().Code != codes.Error || s.Status().Description != "internal" || len(s.Events()) != 1 || a["err.code"].AsInt64() != 0 || a["err.name"].AsString() != "failed" || a["err.attrs.uid"].AsInt64() != 42 || !json.Valid([]byte(a["err.diagnostic"].AsString())) {
		t.Fatalf("span: %v", s)
	}
	joined := errors.Join(k.New("", errkind.With("uid", 1)), k.New("", errkind.With("uid", 2)))
	a = attrMap(otelint.Attributes(joined, otelint.Prefix("biz.")))
	if _, ok := a["biz.code"]; ok {
		t.Fatal("arbitrary primary")
	}
	if _, ok := a["biz.attrs.uid"]; ok {
		t.Fatal("siblings merged")
	}
	var tree map[string]any
	if e := json.Unmarshal([]byte(a["biz.diagnostic"].AsString()), &tree); e != nil || len(tree["causes"].([]any)) != 2 {
		t.Fatal("causes lost")
	}
	otelint.RecordError(nil, err)
	otelint.RecordError(span, nil)
	_, empty := noop.NewTracerProvider().Tracer("noop").Start(context.Background(), "noop")
	otelint.RecordError(empty, err)
	if otelint.Attributes(nil) != nil {
		t.Fatal("nil")
	}
}
func TestValues(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "values")
	for _, value := range []any{"s", true, int(1), int32(2), int64(3), uint(4), uint32(5), uint64(6), uint64(math.MaxUint64), uint(math.MaxUint64), float32(1.5), float64(2.5), map[string]int{"x": 1}, make(chan int)} {
		a := attrMap(otelint.Attributes(k.New("", errkind.With("value", value)), otelint.Prefix("custom.")))
		if _, ok := a["custom.attrs.value"]; !ok {
			t.Fatalf("value: %T", value)
		}
	}
	if attrMap(otelint.Attributes(errors.New("plain")))["err.message"].AsString() != "plain" {
		t.Fatal("plain")
	}
}
