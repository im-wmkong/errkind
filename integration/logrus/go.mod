module github.com/im-wmkong/errkind/integration/logrus

go 1.24.0

require (
	github.com/im-wmkong/errkind v0.0.0
	github.com/sirupsen/logrus v1.9.3
)

require golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect

replace github.com/im-wmkong/errkind => ../..
