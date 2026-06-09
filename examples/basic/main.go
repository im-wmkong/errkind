// 演示 errkind 的最小可运行用法。
package main

import (
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/im-wmkong/errkind"
	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	httpext "github.com/im-wmkong/errkind/ext/http"
	otelext "github.com/im-wmkong/errkind/ext/otel"
	slogext "github.com/im-wmkong/errkind/ext/slog"
)

// 1. Identity: 一次性 Define, 全局单例。
var UserNotFound = errkind.Define(
	10001,
	"user_not_found",
	errkind.DefaultMessage("用户不存在"),
)

var errNoRows = stderrors.New("sql: no rows in result set")

func getUser(id int64) error {
	// 2. Instance: 每次调用产生新错误。
	err := UserNotFound.Wrap(errNoRows, errkind.With("uid", id))

	// 3. 装饰: 协议字段不污染 core。
	err = httpext.Status(404)(err)
	err = grpcext.Code(5)(err) // codes.NotFound
	err = otelext.Name("biz.user.miss")(err)
	return err
}

func main() {
	errkind.SetCaptureStack(true) // dev 环境抓栈

	err := getUser(42)

	fmt.Println("Is UserNotFound:", UserNotFound.Is(err))
	fmt.Println("Is errNoRows  :", stderrors.Is(err, errNoRows))
	fmt.Println("Kind:", errkind.KindOf(err).Name())

	if c, ok := httpext.StatusOf(err); ok {
		fmt.Println("HTTP:", c)
	}
	if c, ok := grpcext.CodeOf(err); ok {
		fmt.Println("gRPC:", c)
	}
	fmt.Println("Telemetry:", otelext.NameOf(err))

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Error("request failed", slogext.Err(err))
}
