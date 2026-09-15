package main

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/im-wmkong/errkind"
	grpcerr "github.com/im-wmkong/errkind/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func dialClient(dialer func(context.Context, string) (net.Conn, error)) (*grpc.ClientConn, error) {
	return grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func callGet(ctx context.Context, conn *grpc.ClientConn, id int64) {
	var response wrapperspb.StringValue
	err := conn.Invoke(ctx, methodName, wrapperspb.Int64(id), &response)
	if err == nil {
		fmt.Printf("id=%d: %s\n", id, response.GetValue())
		return
	}
	st, ok := status.FromError(err)
	if !ok {
		fmt.Printf("id=%d: local error: %v\n", id, err)
		return
	}
	remote := grpcerr.FromStatus(st, errkind.DefaultRegistry())
	fmt.Printf("id=%d: %s, %s\n", id, st.Code(), errkind.MessageOf(remote))
	switch {
	case errors.Is(remote, UserNotFound):
		fmt.Println("提示用户去注册")
	case errors.Is(remote, InvalidArgument):
		fmt.Println("提示用户检查参数")
	}
}
