// Package otel records error diagnostics without mixing sibling attributes.
package otel

import (
	"math"

	"github.com/im-wmkong/errkind"
	"github.com/im-wmkong/errkind/internal/diagnostic"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type options struct{ prefix string }
type Option func(*options)

func Prefix(prefix string) Option { return func(o *options) { o.prefix = prefix } }

func RecordError(span trace.Span, err error, opts ...Option) {
	if span == nil || err == nil || !span.IsRecording() {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, errkind.MessageOf(err))
	span.SetAttributes(Attributes(err, opts...)...)
}

func Attributes(err error, opts ...Option) []attribute.KeyValue {
	if err == nil {
		return nil
	}
	o := options{prefix: "err."}
	for _, opt := range opts {
		opt(&o)
	}
	out := []attribute.KeyValue{attribute.String(o.prefix+"diagnostic", string(diagnostic.Encode(err)))}
	if c, ok := errkind.CodeOf(err); ok {
		out = append(out, attribute.Int64(o.prefix+"code", int64(c)))
	}
	if n, ok := errkind.NameOf(err); ok {
		out = append(out, attribute.String(o.prefix+"name", n))
	}
	if msg := errkind.MessageOf(err); msg != "" {
		out = append(out, attribute.String(o.prefix+"message", msg))
	}
	for _, a := range errkind.AttrsOf(err) {
		out = append(out, kvAttr(o.prefix+"attrs."+a.Key, a.Val))
	}
	return out
}

func kvAttr(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int(key, v)
	case int32:
		return attribute.Int64(key, int64(v))
	case int64:
		return attribute.Int64(key, v)
	case uint:
		if uint64(v) <= math.MaxInt64 {
			return attribute.Int64(key, int64(v))
		}
	case uint32:
		return attribute.Int64(key, int64(v))
	case uint64:
		if v <= math.MaxInt64 {
			return attribute.Int64(key, int64(v))
		}
	case float32:
		return attribute.Float64(key, float64(v))
	case float64:
		return attribute.Float64(key, v)
	}
	return attribute.String(key, string(diagnostic.Value(value)))
}
