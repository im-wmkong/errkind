// Package zap provides explicit structured error logging for zap.
package zap

import (
	"github.com/im-wmkong/errkind/internal/diagnostic"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Err(err error) zap.Field { return Object("err", err) }

func Object(key string, err error) zap.Field {
	return zap.Inline(field{key: key, err: err})
}

type field struct {
	key string
	err error
}

func (f field) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	return enc.AddReflected(f.key, diagnostic.Build(f.err))
}
