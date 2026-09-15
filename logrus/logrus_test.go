package logrus_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind"
	"github.com/im-wmkong/errkind/internal/diagnostic"
	logrusext "github.com/im-wmkong/errkind/logrus"
	"github.com/sirupsen/logrus"
)

func TestTextLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := logrus.New()
	logger.Out = &buf
	logger.Formatter = &logrus.TextFormatter{DisableColors: true, DisableTimestamp: true}
	k := errkind.NewRegistry().Define(1, "failed")
	logger.WithFields(logrusext.Fields(k.Wrap(errors.New("database"), "lookup user", errkind.With("uid", 42)))).Error("failed")
	_, value, ok := strings.Cut(strings.TrimSpace(buf.String()), "err=")
	if !ok {
		t.Fatal(buf.String())
	}
	raw, err := strconv.Unquote(value)
	if err != nil || !json.Valid([]byte(raw)) || !strings.Contains(raw, `"uid":42`) || !strings.Contains(raw, "database") {
		t.Fatalf("text log: %s", buf.String())
	}
}

func TestLogging(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	tree := errors.Join(k.New("", errkind.With("uid", 1), errkind.With("bad", make(chan int))), k.New("", errkind.With("uid", 2)))
	for _, err := range []error{nil, errors.New("plain"), tree} {
		var buf bytes.Buffer
		logger := logrus.New()
		logger.Out = &buf
		logger.Formatter = &logrus.JSONFormatter{}
		logger.WithFields(logrusext.Fields(err)).WithFields(logrusext.FieldsWithPrefix("custom", err)).Error("failed")
		var got map[string]any
		var want any
		if e := json.Unmarshal(buf.Bytes(), &got); e != nil {
			t.Fatal(e)
		}
		if e := json.Unmarshal(diagnostic.Encode(err), &want); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(got["err"], want) || !reflect.DeepEqual(got["custom"], want) {
			t.Fatalf("log: %s", buf.String())
		}
	}
}
