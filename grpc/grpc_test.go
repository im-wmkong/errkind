package grpc_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/im-wmkong/errkind"
	grpcerr "github.com/im-wmkong/errkind/grpc"
	slogerr "github.com/im-wmkong/errkind/slog"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

var defaultKind = errkind.Define(900002, "default_remote")

func TestSafeDefaults(t *testing.T) {
	k := errkind.NewRegistry(errkind.CaptureStack()).Define(0, "private_kind")
	for _, err := range []error{errors.New("password=secret"), k.Wrap(errors.New("secret cause"), "secret message", errkind.With("token", "secret")), errors.Join(k.New(""), status.Error(codes.NotFound, "secret"))} {
		st := grpcerr.ToStatus(err)
		if st.Code() != codes.Internal || st.Message() != "Internal Server Error" || len(st.Details()) != 0 {
			t.Fatalf("leaked: %v", st)
		}
	}
	if grpcerr.ToStatus(nil) != nil || grpcerr.FromStatus(nil, nil) != nil || grpcerr.FromStatus(status.New(codes.OK, ""), nil) != nil {
		t.Fatal("nil/OK")
	}
}

func TestRoundTripBinding(t *testing.T) {
	r := errkind.NewRegistry()
	k := r.Define(0, "missing")
	err := k.New("private", errkind.With("token", "secret"))
	st := grpcerr.ToStatus(err, grpcerr.Code(codes.NotFound), grpcerr.Message("not found"), grpcerr.Identity(), grpcerr.Field("request_id", "old"), grpcerr.Field("request_id", "abc"), grpcerr.Field("code", "public"))
	if st.Code() != codes.NotFound || st.Message() != "not found" || len(st.Details()) != 1 {
		t.Fatal(st)
	}
	info := st.Details()[0].(*errdetails.ErrorInfo)
	if info.Domain != "errkind/v1" || info.Reason != "missing" || info.Metadata["_errkind.code"] != "0" || info.Metadata["token"] != "" || info.Metadata["request_id"] != "abc" || len(info.Metadata) != 3 {
		t.Fatal(info)
	}
	other := errkind.NewRegistry()
	otherKind := other.Define(0, "missing")
	mismatch := errkind.NewRegistry()
	mismatch.Define(0, "other")
	for _, tc := range []struct {
		registry *errkind.Registry
		want     *errkind.Kind
	}{{r, k}, {other, otherKind}, {mismatch, nil}, {nil, nil}} {
		remote := grpcerr.FromStatus(st, tc.registry)
		if errkind.KindOf(remote) != tc.want || errors.Is(remote, k) != (tc.want == k) || errkind.MessageOf(remote) != "not found" {
			t.Fatalf("binding: %v", remote)
		}
		if c, ok := errkind.CodeOf(remote); !ok || c != 0 {
			t.Fatal("zero code")
		}
		if n, ok := errkind.NameOf(remote); !ok || n != "missing" {
			t.Fatal("name")
		}
		attrs := errkind.AttrsOf(remote)
		if len(attrs) != 2 {
			t.Fatal(attrs)
		}
		attrs[0].Val = "mutated"
		if errkind.AttrsOf(remote)[0].Val == "mutated" {
			t.Fatal("alias")
		}
		if grpcerr.ToStatus(remote) != st || grpcerr.ToStatus(fmt.Errorf("private wrapper: %w", remote)) != st {
			t.Fatal("original status lost")
		}
		if got, ok := status.FromError(remote); !ok || !proto.Equal(got.Proto(), st.Proto()) {
			t.Fatal("GRPCStatus contract")
		}
		var buf bytes.Buffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Error("failed", slogerr.Err(remote))
		if !strings.Contains(buf.String(), `"request_id":"abc"`) || !strings.Contains(buf.String(), `"code":0`) {
			t.Fatal(buf.String())
		}
		if _, ok := errkind.CodeOf(errors.Join(remote, errors.New("other"))); ok {
			t.Fatal("join primary")
		}
	}
	defaultStatus := grpcerr.ToStatus(defaultKind.New(""), grpcerr.Identity())
	if errors.Is(grpcerr.FromStatus(defaultStatus, nil), defaultKind) || !errors.Is(grpcerr.FromStatus(defaultStatus, errkind.DefaultRegistry()), defaultKind) {
		t.Fatal("default registry must be explicit")
	}
	fieldOnly := grpcerr.FromStatus(grpcerr.ToStatus(err, grpcerr.Field("request_id", "abc")), r)
	if _, ok := errkind.CodeOf(fieldOnly); ok || len(errkind.AttrsOf(fieldOnly)) != 1 {
		t.Fatal("fields must not imply identity")
	}
	joined := errors.Join(err, errors.New("other"))
	if len(grpcerr.ToStatus(joined, grpcerr.Identity()).Details()) != 0 {
		t.Fatal("ambiguous public identity")
	}
}

type native struct{ st *status.Status }

func (native) Error() string                { return "native" }
func (n native) GRPCStatus() *status.Status { return n.st }

type cycle struct{}

func (*cycle) Error() string   { return "cycle" }
func (c *cycle) Unwrap() error { return c }

func TestStatusPrecedence(t *testing.T) {
	st, e := status.New(codes.PermissionDenied, "denied").WithDetails(&errdetails.ErrorInfo{Domain: "foreign", Reason: "FOREIGN"})
	if e != nil {
		t.Fatal(e)
	}
	k := errkind.NewRegistry().Define(1, "classified")
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{st.Err(), codes.PermissionDenied}, {fmt.Errorf("private: %w", st.Err()), codes.PermissionDenied}, {context.Canceled, codes.Canceled}, {fmt.Errorf("private: %w", context.DeadlineExceeded), codes.DeadlineExceeded},
		{errors.Join(context.Canceled), codes.Canceled}, {errors.Join(context.Canceled, st.Err()), codes.Internal}, {k.Wrap(st.Err(), "private"), codes.Internal}, {k.Wrap(context.Canceled, "private"), codes.Internal}, {k.Wrap(grpcerr.FromStatus(st, nil), "private"), codes.Internal}, {native{}, codes.Internal}, {native{status.New(codes.OK, "")}, codes.Internal}, {native{status.New(codes.Code(99), "")}, codes.Internal}, {&cycle{}, codes.Internal},
	} {
		got := grpcerr.ToStatus(tc.err)
		if got.Code() != tc.code || strings.Contains(got.Message(), "private") {
			t.Fatalf("%v => %v", tc.err, got)
		}
	}
	for _, err := range []error{st.Err(), grpcerr.FromStatus(st, nil), context.Canceled} {
		got := grpcerr.ToStatus(err, grpcerr.Message("new"))
		if got.Code() != codes.Internal || got.Message() != "new" || len(got.Details()) != 0 {
			t.Fatal("options must rebuild from safe defaults")
		}
	}
	for _, opts := range [][]grpcerr.Option{{grpcerr.Code(codes.OK)}, {grpcerr.Code(codes.Code(99))}, {grpcerr.Field("_errkind.code", "999")}, {grpcerr.Field("_errkind.other", "x")}, {grpcerr.Message(string([]byte{255}))}, {grpcerr.Field("bad", string([]byte{255}))}, {grpcerr.Field(string([]byte{255}), "bad")}} {
		got := grpcerr.ToStatus(k.New(""), opts...)
		if got.Code() != codes.Internal || got.Message() != "Internal Server Error" || len(got.Details()) != 0 {
			t.Fatalf("invalid option: %v", got)
		}
	}
}

func TestMalformedIdentity(t *testing.T) {
	r := errkind.NewRegistry()
	k := r.Define(1, "one")
	for _, info := range []*errdetails.ErrorInfo{
		{Domain: "foreign", Reason: "one", Metadata: map[string]string{"_errkind.code": "1"}},
		{Domain: "errkind/v1", Reason: "one"},
		{Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "-1"}},
		{Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "4294967296"}},
		{Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "01"}},
		{Domain: "errkind/v1", Metadata: map[string]string{"_errkind.code": "1"}},
		{Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "1", "_errkind.order": "[]"}},
	} {
		st, e := status.New(codes.Internal, "remote").WithDetails(info)
		if e != nil {
			t.Fatal(e)
		}
		err := grpcerr.FromStatus(st, r)
		if errors.Is(err, k) || errkind.KindOf(err) != nil {
			t.Fatal("invalid binding")
		}
		if _, ok := errkind.CodeOf(err); ok || len(errkind.AttrsOf(err)) != 0 {
			t.Fatal("malformed diagnostics accepted")
		}
		if grpcerr.ToStatus(err) != st {
			t.Fatal("status lost")
		}
	}
	info := &errdetails.ErrorInfo{Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "1"}}
	st, e := status.New(codes.Internal, "duplicate").WithDetails(info, info)
	if e != nil {
		t.Fatal(e)
	}
	if errors.Is(grpcerr.FromStatus(st, r), k) {
		t.Fatal("duplicate identities")
	}
	for _, info := range []*errdetails.ErrorInfo{{Domain: "errkind/v1", Reason: "two", Metadata: map[string]string{"_errkind.code": "1"}}, {Domain: "errkind/v1", Reason: "one", Metadata: map[string]string{"_errkind.code": "2"}}} {
		st, e := status.New(codes.Internal, "mismatch").WithDetails(info)
		if e != nil {
			t.Fatal(e)
		}
		remote := grpcerr.FromStatus(st, r)
		if errors.Is(remote, k) || errkind.KindOf(remote) != nil {
			t.Fatal("partial match bound")
		}
		if _, ok := errkind.CodeOf(remote); !ok {
			t.Fatal("valid but unbound details lost")
		}
	}
}

func TestBufconn(t *testing.T) {
	r := errkind.NewRegistry()
	k := r.Define(42, "missing")
	respond := func() error {
		return grpcerr.ToStatus(k.New("private"), grpcerr.Code(codes.NotFound), grpcerr.Message("not found"), grpcerr.Identity(), grpcerr.Field("request_id", "abc")).Err()
	}
	desc := grpc.ServiceDesc{ServiceName: "errkind.test.Service", HandlerType: (*any)(nil), Methods: []grpc.MethodDesc{{MethodName: "Get", Handler: func(_ any, _ context.Context, decode func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
		var req emptypb.Empty
		if err := decode(&req); err != nil {
			return nil, err
		}
		return nil, respond()
	}}}, Streams: []grpc.StreamDesc{{StreamName: "Watch", ServerStreams: true, Handler: func(_ any, stream grpc.ServerStream) error {
		if err := stream.SendMsg(&emptypb.Empty{}); err != nil {
			return err
		}
		return respond()
	}}}}
	listener := bufconn.Listen(1 << 16)
	defer listener.Close()
	server := grpc.NewServer()
	server.RegisterService(&desc, struct{}{})
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	conn, e := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	check := func(err error) {
		t.Helper()
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Fatalf("wire error: %v", err)
		}
		remote := grpcerr.FromStatus(st, r)
		if !errors.Is(remote, k) || errkind.MessageOf(remote) != "not found" || len(errkind.AttrsOf(remote)) != 1 {
			t.Fatalf("remote: %v", remote)
		}
	}
	check(conn.Invoke(ctx, "/errkind.test.Service/Get", &emptypb.Empty{}, &emptypb.Empty{}))
	stream, e := conn.NewStream(ctx, &desc.Streams[0], "/errkind.test.Service/Watch")
	if e != nil {
		t.Fatal(e)
	}
	if e = stream.CloseSend(); e != nil {
		t.Fatal(e)
	}
	if e = stream.RecvMsg(&emptypb.Empty{}); e != nil {
		t.Fatal(e)
	}
	check(stream.RecvMsg(&emptypb.Empty{}))
}
