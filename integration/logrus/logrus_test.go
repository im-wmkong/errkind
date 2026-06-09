package logrus_test

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind"
	grpcext "github.com/im-wmkong/errkind/ext/grpc"
	httpext "github.com/im-wmkong/errkind/ext/http"
	logrusext "github.com/im-wmkong/errkind/integration/logrus"
	"github.com/sirupsen/logrus"
)

func TestFieldsFull(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(1, "x", errkind.DefaultMessage("默认"))
	err := K.Wrap(stderrors.New("root"), errkind.With("uid", 9))
	err = httpext.Status(404)(err)
	err = grpcext.Code(5)(err)

	var buf bytes.Buffer
	logger := logrus.New()
	logger.Out = &buf
	logger.Formatter = &logrus.JSONFormatter{}
	logger.WithFields(logrusext.Fields(err)).Error("fail")

	out := buf.String()
	for _, w := range []string{
		`"err.name":"x"`,
		`"err.code":1`,
		`"err.message":"默认"`,
		`"err.attrs.uid":9`,
		`"err.http_status":404`,
		`"err.grpc_code":5`,
		`"err.cause":"root"`,
	} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in:\n%s", w, out)
		}
	}
}

func TestFieldsNil(t *testing.T) {
	f := logrusext.Fields(nil)
	if len(f) != 0 {
		t.Fatalf("expected empty fields, got %v", f)
	}
}

func TestFieldsCustomPrefix(t *testing.T) {
	r := errkind.NewRegistry()
	K := r.Define(2, "y")
	err := K.New(errkind.Message("oops"))

	f := logrusext.FieldsWithPrefix("biz", err)
	if f["biz.name"] != "y" || f["biz.message"] != "oops" {
		t.Fatalf("unexpected fields: %v", f)
	}
}
