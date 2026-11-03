package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/im-wmkong/errkind"
)

var ErrUserNotFound = errkind.Define(10001, "user_not_found")

func findUser(id int64) error {
	return ErrUserNotFound.Wrap(sql.ErrNoRows, "lookup user", errkind.With("uid", id))
}

func main() {
	err := findUser(42)
	fmt.Println("Is ErrUserNotFound:", errors.Is(err, ErrUserNotFound))
	fmt.Println("Is sql.ErrNoRows:", errors.Is(err, sql.ErrNoRows))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var detail *errkind.Error
	if errors.As(err, &detail) {
		logger = logger.With(
			slog.Uint64("code", uint64(detail.Kind().Code())),
			slog.String("kind", detail.Kind().Name()),
			slog.String("message", detail.Message()),
			slog.Any("attrs", detail.Attrs()),
		)
	}
	logger.Error("request failed", slog.String("request_id", "req-1"), slog.String("err", err.Error()))
}
