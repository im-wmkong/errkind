package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/im-wmkong/errkind"
	httperr "github.com/im-wmkong/errkind/http"
)

type panicJSON struct{}

func (panicJSON) MarshalJSON() ([]byte, error) { panic("secret") }

type onceJSON struct{ calls int }

func (v *onceJSON) MarshalJSON() ([]byte, error) {
	v.calls++
	if v.calls > 1 {
		return nil, errors.New("encoded twice")
	}
	return []byte(`{"x":1}`), nil
}

func TestSafeDefault(t *testing.T) {
	k := errkind.NewRegistry(errkind.CaptureStack()).Define(0, "private_identity")
	for _, err := range []error{errors.New("secret"), k.Wrap(errors.New("cause_secret"), "message_secret", errkind.With("token", "token_secret")), errors.Join(k.New(""), errors.New("secret"))} {
		rec := httptest.NewRecorder()
		if e := httperr.Write(rec, err); e != nil {
			t.Fatal(e)
		}
		if rec.Code != 500 || rec.Body.String() != `{"message":"Internal Server Error"}`+"\n" {
			t.Fatalf("leaked: %d %s", rec.Code, rec.Body)
		}
	}
	rec := httptest.NewRecorder()
	if e := httperr.Write(rec, nil, httperr.Status(404)); e != nil || rec.Body.Len() != 0 || len(rec.Header()) != 0 {
		t.Fatal("nil must not write")
	}
	if r, e := httperr.ResponseOf(nil); r != nil || e != nil {
		t.Fatal("nil response")
	}
}

func TestExplicitResponse(t *testing.T) {
	k := errkind.NewRegistry().Define(0, "missing")
	v := &onceJSON{}
	rec := httptest.NewRecorder()
	err := k.New("private", errkind.With("token", "private"))
	if e := httperr.Write(rec, err, httperr.Status(404), httperr.Message("not found"), httperr.Identity(), httperr.Field("request_id", "old"), httperr.Field("request_id", "public"), httperr.Field("code", 99), httperr.Field("object", v)); e != nil {
		t.Fatal(e)
	}
	var body map[string]any
	if e := json.Unmarshal(rec.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	if rec.Code != 404 || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || body["code"] != float64(0) || body["name"] != "missing" || body["message"] != "not found" || body["fields"].(map[string]any)["request_id"] != "public" || v.calls != 1 || strings.Contains(rec.Body.String(), "private") {
		t.Fatalf("response: %s", rec.Body)
	}
	joined := errors.Join(err, errors.New("other"))
	r, e := httperr.ResponseOf(joined, httperr.Identity())
	if e != nil || r.Body.Code != nil || r.Body.Name != "" {
		t.Fatal("ambiguous identity")
	}
	r, e = httperr.ResponseOf(k.Wrap(joined, "lookup user"), httperr.Identity())
	if e != nil || r.Body.Code == nil || *r.Body.Code != 0 {
		t.Fatal("outer classification")
	}
}

func TestInvalidResponseFallsBack(t *testing.T) {
	for _, opt := range []httperr.Option{httperr.Status(0), httperr.Status(200), httperr.Status(399), httperr.Status(600), httperr.Field("bad", make(chan int)), httperr.Field("bad", panicJSON{})} {
		rec := httptest.NewRecorder()
		err := httperr.Write(rec, errors.New("secret"), httperr.Status(404), httperr.Message("custom"), opt)
		if err == nil || rec.Code != 500 || rec.Body.String() != `{"message":"Internal Server Error"}`+"\n" {
			t.Fatalf("fallback: %v %d %s", err, rec.Code, rec.Body)
		}
	}
}

type failedWriter struct{ header http.Header }

func (w failedWriter) Header() http.Header     { return w.header }
func (failedWriter) WriteHeader(int)           {}
func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestWriteFailure(t *testing.T) {
	err := httperr.Write(failedWriter{http.Header{}}, errors.New("private"), httperr.Status(0))
	if !errors.Is(err, io.ErrClosedPipe) || !strings.Contains(err.Error(), "invalid error status") {
		t.Fatalf("lost errors: %v", err)
	}
}

func TestResponder(t *testing.T) {
	type contextKey struct{}
	calls := 0
	shared := []httperr.Option{httperr.Status(403), httperr.Message("callback"), httperr.Field("locale", "en")}
	responder := httperr.Responder(func(ctx context.Context, err error) []httperr.Option {
		calls++
		if ctx.Value(contextKey{}) != "zh" || err == nil {
			t.Fatal("context or error lost")
		}
		return shared
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.WithValue(context.Background(), contextKey{}, "zh"))
	rec := httptest.NewRecorder()
	if e := responder.Write(rec, req, errors.New("secret"), httperr.Status(404), httperr.Message("调用参数"), httperr.Field("locale", "zh")); e != nil {
		t.Fatal(e)
	}
	if rec.Code != 404 || !strings.Contains(rec.Body.String(), "调用参数") || !strings.Contains(rec.Body.String(), `"locale":"zh"`) {
		t.Fatal(rec.Body)
	}
	if e := responder.Write(rec, req, nil); e != nil || calls != 1 {
		t.Fatal("nil should skip callback")
	}
	var defaultResponder httperr.Responder
	if e := defaultResponder.Write(httptest.NewRecorder(), req, errors.New("plain")); e != nil {
		t.Fatal(e)
	}
}
