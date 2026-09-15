package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/im-wmkong/errkind"
	slogerr "github.com/im-wmkong/errkind/slog"
)

var UserNotFound = errkind.Define(10001, "user_not_found")
var errNoRows = errors.New("sql: no rows in result set")

func getUser(id int64) error {
	return UserNotFound.Wrap(errNoRows, "lookup user", errkind.With("uid", id))
}

func main() {
	err := getUser(42)
	fmt.Println("Is UserNotFound:", errors.Is(err, UserNotFound))
	fmt.Println("Is errNoRows:", errors.Is(err, errNoRows))
	slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("request failed", slogerr.Err(err))
}
