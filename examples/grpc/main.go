package main

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc/test/bufconn"
)

func main() {
	listener := bufconn.Listen(1 << 16)
	defer listener.Close()
	stop := startServer(listener)
	defer stop()
	conn, err := dialClient(func(context.Context, string) (net.Conn, error) { return listener.Dial() })
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, id := range []int64{42, 0, 999} {
		callGet(ctx, conn, id)
	}
}
