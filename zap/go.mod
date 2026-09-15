module github.com/im-wmkong/errkind/zap

go 1.25.0

require (
	github.com/im-wmkong/errkind v0.2.0
	go.uber.org/zap v1.27.0
)

require go.uber.org/multierr v1.10.0 // indirect

replace github.com/im-wmkong/errkind => ..
