// Package grpc converts public gRPC statuses without owning transport lifecycle.
package grpc

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/im-wmkong/errkind"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const wireFormat = "errkind/v1"
const codeKey = "_errkind.code"
const publicMessage = "Internal Server Error"

type options struct {
	code     codes.Code
	message  string
	fields   map[string]string
	identity bool
}
type Option func(*options)

func Code(code codes.Code) Option   { return func(o *options) { o.code = code } }
func Message(message string) Option { return func(o *options) { o.message = message } }
func Identity() Option              { return func(o *options) { o.identity = true } }
func Field(key, value string) Option {
	return func(o *options) {
		if o.fields == nil {
			o.fields = make(map[string]string)
		}
		o.fields[key] = value
	}
}

func ToStatus(err error, opts ...Option) *status.Status {
	if err == nil {
		return nil
	}
	if len(opts) == 0 {
		if st := existingStatus(err); st != nil {
			return st
		}
	}
	o := options{code: codes.Internal, message: publicMessage}
	for _, opt := range opts {
		opt(&o)
	}
	if o.code == codes.OK || o.code > codes.Unauthenticated || !utf8.ValidString(o.message) {
		return status.New(codes.Internal, publicMessage)
	}
	info := &errdetails.ErrorInfo{Domain: wireFormat, Metadata: make(map[string]string)}
	for key, value := range o.fields {
		if strings.HasPrefix(key, "_errkind.") || !utf8.ValidString(key) || !utf8.ValidString(value) {
			return status.New(codes.Internal, publicMessage)
		}
		info.Metadata[key] = value
	}
	if o.identity {
		name, named := errkind.NameOf(err)
		if code, ok := errkind.CodeOf(err); ok && named {
			info.Reason = name
			info.Metadata[codeKey] = strconv.FormatUint(uint64(code), 10)
		}
	}
	st := status.New(o.code, o.message)
	if len(info.Metadata) == 0 {
		return st
	}
	withDetails, encodeErr := st.WithDetails(info)
	if encodeErr != nil {
		return status.New(codes.Internal, publicMessage)
	}
	return withDetails
}

// A local classification is a boundary: do not recover an inner transport status through it.
func existingStatus(err error) *status.Status {
	for n := 0; err != nil && n < 256; n++ {
		if remote, ok := err.(*remoteErr); ok {
			return remote.status
		}
		if _, modeled := err.(errkind.Details); modeled {
			return nil
		}
		if native, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
			st := native.GRPCStatus()
			if st != nil && st.Code() > codes.OK && st.Code() <= codes.Unauthenticated {
				return st
			}
			return nil
		}
		if err == context.Canceled {
			return status.New(codes.Canceled, context.Canceled.Error())
		}
		if err == context.DeadlineExceeded {
			return status.New(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
		}
		switch e := err.(type) {
		case interface{ Unwrap() error }:
			err = e.Unwrap()
		case interface{ Unwrap() []error }:
			var next error
			for _, child := range e.Unwrap() {
				if child == nil {
					continue
				}
				if next != nil {
					return nil
				}
				next = child
			}
			err = next
		default:
			return nil
		}
	}
	return nil
}

// FromStatus only binds identity when the caller supplies a matching registry.
func FromStatus(st *status.Status, registry *errkind.Registry) error {
	if st == nil || st.Code() == codes.OK {
		return nil
	}
	e := &remoteErr{status: st}
	var info *errdetails.ErrorInfo
	for _, detail := range st.Details() {
		if candidate, ok := detail.(*errdetails.ErrorInfo); ok && candidate.Domain == wireFormat {
			if info != nil {
				return e
			}
			info = candidate
		}
	}
	if info == nil {
		return e
	}
	for key := range info.Metadata {
		if strings.HasPrefix(key, "_errkind.") && key != codeKey {
			return e
		}
	}
	if raw, present := info.Metadata[codeKey]; present {
		code, parseErr := strconv.ParseUint(raw, 10, 32)
		if parseErr != nil || strconv.FormatUint(code, 10) != raw || info.Reason == "" {
			return e
		}
		e.code, e.hasCode, e.name = errkind.Code(code), true, info.Reason
	} else if info.Reason != "" {
		return e
	}
	keys := make([]string, 0, len(info.Metadata))
	for key := range info.Metadata {
		if key != codeKey {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		e.attrs = append(e.attrs, errkind.Attr{Key: key, Val: info.Metadata[key]})
	}
	if registry != nil && e.hasCode {
		if kind := registry.LookupCode(e.code); kind != nil && kind.Name() == e.name {
			e.kind = kind
		}
	}
	return e
}

type remoteErr struct {
	code    errkind.Code
	hasCode bool
	name    string
	attrs   []errkind.Attr
	kind    *errkind.Kind
	status  *status.Status
}

func (e *remoteErr) Error() string                      { return e.status.Err().Error() }
func (e *remoteErr) Message() string                    { return e.status.Message() }
func (e *remoteErr) Name() string                       { return e.name }
func (e *remoteErr) BusinessCode() (errkind.Code, bool) { return e.code, e.hasCode }
func (e *remoteErr) Attrs() []errkind.Attr              { return append([]errkind.Attr(nil), e.attrs...) }
func (e *remoteErr) Kind() *errkind.Kind                { return e.kind }
func (e *remoteErr) Is(target error) bool               { return e.kind != nil && target == e.kind }
func (e *remoteErr) GRPCStatus() *status.Status         { return e.status }
