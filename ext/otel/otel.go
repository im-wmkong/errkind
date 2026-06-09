// Package otel 提供错误的 telemetry 命名约定 (用于 metrics / tracing 维度切分)。
//
// 默认 NameOf(err) 回退到 errkind Kind 的 Name(); 业务可显式覆盖:
//
//	err = otelext.Name("biz.user.miss")(err)
package otel

import (
	"errors"

	"github.com/im-wmkong/errkind"
)

type withName struct {
	error
	name string
}

func (w *withName) TelemetryName() string { return w.name }
func (w *withName) Unwrap() error          { return w.error }

// Name 返回装饰器, 显式覆盖 telemetry 名称。
func Name(name string) func(error) error {
	return func(err error) error {
		if err == nil {
			return nil
		}
		return &withName{error: err, name: name}
	}
}

// NameOf 解析顺序:
//  1. 显式 Name() 装饰
//  2. errkind Kind.Name()
//  3. ""
func NameOf(err error) string {
	var t interface{ TelemetryName() string }
	if errors.As(err, &t) {
		return t.TelemetryName()
	}
	if name, ok := errkind.NameOf(err); ok {
		return name
	}
	return ""
}
