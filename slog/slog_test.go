package slog_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind"
	"github.com/im-wmkong/errkind/internal/diagnostic"
	slogext "github.com/im-wmkong/errkind/slog"
)

func BenchmarkJSONLogging(b *testing.B) {
	k := errkind.NewRegistry().Define(1, "failed")
	leaf := k.New("", errkind.With("uid", 42))
	deep := leaf
	for i := 0; i < 32; i++ {
		deep = fmt.Errorf("context: %w", deep)
	}
	for name, err := range map[string]error{"single": leaf, "deep32": deep, "join": errors.Join(leaf, k.New("", errkind.With("uid", 43)))} {
		b.Run(name, func(b *testing.B) {
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				logger.Error("failed", slogext.Err(err))
			}
		})
	}
}

func TestTextLogging(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	err := k.Wrap(errors.New("database"), "lookup user", errkind.With("uid", 42))
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Error("failed", slogext.Err(err))
	_, value, ok := strings.Cut(strings.TrimSpace(buf.String()), "err=")
	if !ok {
		t.Fatal(buf.String())
	}
	raw, parseErr := strconv.Unquote(value)
	if parseErr != nil || !json.Valid([]byte(raw)) || !strings.Contains(raw, `"uid":42`) || !strings.Contains(raw, "database") {
		t.Fatalf("text log: %s", buf.String())
	}
}

func TestLogging(t *testing.T) {
	k := errkind.NewRegistry().Define(1, "failed")
	tree := errors.Join(k.New("", errkind.With("uid", 1), errkind.With("bad", make(chan int))), k.New("", errkind.With("uid", 2)))
	for _, err := range []error{nil, errors.New("plain"), tree} {
		var buf bytes.Buffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Error("failed", slogext.Err(err), slog.Any("custom", slogext.Value(err)))
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
