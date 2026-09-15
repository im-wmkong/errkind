// Package http builds public responses independently of error diagnostics.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/im-wmkong/errkind"
)

type Body struct {
	Code    *errkind.Code              `json:"code,omitempty"`
	Name    string                     `json:"name,omitempty"`
	Message string                     `json:"message"`
	Fields  map[string]json.RawMessage `json:"fields,omitempty"`
}

type Response struct {
	Status int
	Body   Body
}

type options struct {
	status   int
	message  string
	fields   map[string]any
	identity bool
}

type Option func(*options)

func Status(code int) Option        { return func(o *options) { o.status = code } }
func Message(message string) Option { return func(o *options) { o.message = message } }
func Identity() Option              { return func(o *options) { o.identity = true } }
func Field(key string, value any) Option {
	return func(o *options) {
		if o.fields == nil {
			o.fields = make(map[string]any)
		}
		o.fields[key] = value
	}
}

func ResponseOf(err error, opts ...Option) (*Response, error) {
	if err == nil {
		return nil, nil
	}
	o := options{status: http.StatusInternalServerError, message: http.StatusText(http.StatusInternalServerError)}
	for _, opt := range opts {
		opt(&o)
	}
	if o.status < 400 || o.status > 599 {
		return fallback(), fmt.Errorf("errkind/http: invalid error status %d", o.status)
	}
	r := &Response{Status: o.status, Body: Body{Message: o.message}}
	if o.identity {
		if c, ok := errkind.CodeOf(err); ok {
			r.Body.Code = &c
		}
		r.Body.Name, _ = errkind.NameOf(err)
	}
	for key, value := range o.fields {
		raw, encodeErr := encode(value)
		if encodeErr != nil {
			return fallback(), fmt.Errorf("errkind/http: field %q: %w", key, encodeErr)
		}
		if r.Body.Fields == nil {
			r.Body.Fields = make(map[string]json.RawMessage)
		}
		r.Body.Fields[key] = raw
	}
	return r, nil
}

func Write(w http.ResponseWriter, err error, opts ...Option) error {
	r, buildErr := ResponseOf(err, opts...)
	if r == nil {
		return nil
	}
	raw, encodeErr := json.Marshal(r.Body)
	if encodeErr != nil {
		return encodeErr
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(r.Status)
	_, writeErr := w.Write(append(raw, '\n'))
	return errors.Join(buildErr, writeErr)
}

type Responder func(context.Context, error) []Option

func (respond Responder) Write(w http.ResponseWriter, r *http.Request, err error, opts ...Option) error {
	if err == nil {
		return nil
	}
	var resolved []Option
	if respond != nil {
		resolved = append(resolved, respond(r.Context(), err)...)
	}
	resolved = append(resolved, opts...)
	return Write(w, err, resolved...)
}

func fallback() *Response {
	return &Response{Status: http.StatusInternalServerError, Body: Body{Message: http.StatusText(http.StatusInternalServerError)}}
}

func encode(value any) (raw []byte, err error) {
	defer func() {
		if recover() != nil {
			raw = nil
			err = errors.New("JSON encoder panicked")
		}
	}()
	return json.Marshal(value)
}
