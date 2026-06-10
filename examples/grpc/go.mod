module github.com/im-wmkong/errkind/examples/grpc

go 1.24.0

require (
	github.com/im-wmkong/errkind v0.0.0
	github.com/im-wmkong/errkind/integration/grpc v0.0.0-20260609120453-e15168aec72f
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.10
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace github.com/im-wmkong/errkind => ../..
