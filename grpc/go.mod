module github.com/im-wmkong/errkind/grpc

go 1.25.0

require (
	github.com/im-wmkong/errkind v0.2.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478
	google.golang.org/grpc v1.82.2
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
)

replace github.com/im-wmkong/errkind => ..
