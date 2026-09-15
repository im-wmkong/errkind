package zerolog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/im-wmkong/errkind"
	"github.com/im-wmkong/errkind/internal/diagnostic"
	zeroext "github.com/im-wmkong/errkind/zerolog"
	"github.com/rs/zerolog"
)

func TestLogging(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	tree := errors.Join(k.New("", errkind.With("uid", 1), errkind.With("bad", make(chan int))), k.New("", errkind.With("uid", 2)))
	for _, err := range []error{nil, errors.New("plain"), tree} {
		var buf bytes.Buffer
		logger := zerolog.New(&buf)
		logger.Error().Func(zeroext.Err(err)).Func(zeroext.Field("custom", err)).Msg("failed")
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
