package main

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/im-wmkong/errkind"
	grpcerr "github.com/im-wmkong/errkind/grpc"
	slogerr "github.com/im-wmkong/errkind/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	UserNotFound    = errkind.Define(10001, "user_not_found")
	InvalidArgument = errkind.Define(10002, "invalid_argument")
)

const methodName = "/errkind.example.UserSvc/Get"

func getUser(id int64) error {
	switch {
	case id <= 0:
		return InvalidArgument.New("id must be positive", errkind.With("id", id))
	case id == 999:
		return UserNotFound.Wrap(errors.New("sql: no rows in result set"), "lookup user", errkind.With("uid", id))
	default:
		return nil
	}
}

func registerService(srv *grpc.Server) {
	desc := &grpc.ServiceDesc{
		ServiceName: "errkind.example.UserSvc",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{{
			MethodName: "Get",
			Handler: func(_ any, ctx context.Context, decode func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
				var req wrapperspb.Int64Value
				if err := decode(&req); err != nil {
					return nil, err
				}
				handler := func(_ context.Context, raw any) (any, error) {
					if err := getUser(raw.(*wrapperspb.Int64Value).GetValue()); err != nil {
						var opts []grpcerr.Option
						switch errkind.KindOf(err) {
						case UserNotFound:
							opts = []grpcerr.Option{grpcerr.Code(codes.NotFound), grpcerr.Message("用户不存在"), grpcerr.Identity()}
						case InvalidArgument:
							opts = []grpcerr.Option{grpcerr.Code(codes.InvalidArgument), grpcerr.Message("id 必须是正整数"), grpcerr.Identity()}
						}
						st := grpcerr.ToStatus(err, opts...)
						slog.Error("request failed", slogerr.Err(err), slog.String("grpc_code", st.Code().String()))
						return nil, st.Err()
					}
					return wrapperspb.String("ok"), nil
				}
				if interceptor == nil {
					return handler(ctx, &req)
				}
				return interceptor(ctx, &req, &grpc.UnaryServerInfo{Server: srv, FullMethod: methodName}, handler)
			},
		}},
	}
	srv.RegisterService(desc, struct{}{})
}

func startServer(listener net.Listener) func() {
	srv := grpc.NewServer()
	registerService(srv)
	go func() { _ = srv.Serve(listener) }()
	return srv.Stop
}
