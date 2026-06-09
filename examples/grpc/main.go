// 演示 errkit 错误如何映射成 *status.Status (含 ErrorInfo details)。
//
//	go run ./examples/grpc
//
// 关键点:
//   - 业务层只产 errkit 错误 + ext/grpc 装饰, 不直接构造 *status.Status。
//   - toStatus 是唯一的"协议出口", 决定 grpc code 与 ErrorInfo 形状。
//   - ErrorInfo.Reason = errkit name, Metadata = errkit attrs (字符串化)。
//
// 在真实服务里, 把 toStatus 包进 grpc.UnaryServerInterceptor 即可:
//
//	func interceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
//	    resp, err := h(ctx, req)
//	    if err != nil {
//	        return nil, toStatus(err).Err()
//	    }
//	    return resp, nil
//	}
package main

import (
	stderrors "errors"
	"fmt"

	"github.com/im-wmkong/errkit"
	grpcext "github.com/im-wmkong/errkit/ext/grpc"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	UserNotFound = errkit.Define(10001, "user_not_found",
		errkit.DefaultMessage("用户不存在"),
	)
	InvalidArgument = errkit.Define(10002, "invalid_argument",
		errkit.DefaultMessage("参数非法"),
	)
)

var errNoRows = stderrors.New("sql: no rows in result set")

func getUser(id int64) error {
	if id <= 0 {
		return grpcext.Code(uint32(codes.InvalidArgument))(
			InvalidArgument.New(errkit.With("id", id)),
		)
	}
	if id == 999 {
		return grpcext.Code(uint32(codes.NotFound))(
			UserNotFound.Wrap(errNoRows, errkit.With("uid", id)),
		)
	}
	return nil
}

// toStatus 把 errkit 错误映射成 gRPC *status.Status。
//
//	code     <- ext/grpc 装饰; 没有则 codes.Unknown
//	message  <- errkit.MessageOf
//	details  <- ErrorInfo{ Reason: name, Domain: "errkit", Metadata: attrs }
func toStatus(err error) *status.Status {
	if err == nil {
		return nil
	}
	c := codes.Unknown
	if g, ok := grpcext.CodeOf(err); ok {
		c = codes.Code(g)
	}
	st := status.New(c, errkit.MessageOf(err))

	info := &errdetails.ErrorInfo{Domain: "errkit", Metadata: map[string]string{}}
	if n, ok := errkit.NameOf(err); ok {
		info.Reason = n
	}
	for _, kv := range errkit.AllAttrs(err) {
		info.Metadata[kv.Key] = fmt.Sprint(kv.Val)
	}
	if d, derr := st.WithDetails(info); derr == nil {
		return d
	}
	return st
}

func main() {
	for _, id := range []int64{42, 0, 999} {
		err := getUser(id)
		fmt.Printf("\n[id=%d]\n", id)
		if err == nil {
			fmt.Println("  ok")
			continue
		}
		st := toStatus(err)
		fmt.Printf("  code    = %s\n", st.Code())
		fmt.Printf("  message = %q\n", st.Message())
		for _, d := range st.Details() {
			if info, ok := d.(*errdetails.ErrorInfo); ok {
				fmt.Printf("  reason   = %s\n", info.Reason)
				fmt.Printf("  domain   = %s\n", info.Domain)
				fmt.Printf("  metadata = %v\n", info.Metadata)
			}
		}
	}
}
