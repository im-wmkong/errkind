// Package slog provides explicit structured error logging for log/slog.
package slog

import (
	"log/slog"

	"github.com/im-wmkong/errkind/internal/diagnostic"
)

func Err(err error) slog.Attr { return slog.Attr{Key: "err", Value: Value(err)} }

func Value(err error) slog.Value {
	return slog.AnyValue(diagnostic.Build(err))
}
