# errkind

English | [简体中文](README_CN.md)

**errkind** is a business error library that works with Go's standard error handling, providing stable error identities, structured diagnostics, and optional logging and protocol integrations.

Use a `Kind` to identify **what failed**, messages, attributes, and causes to explain **this occurrence**, and boundary adapters to decide **what clients see**. Business code does not need HTTP status codes or a custom struct for every error type.

## Core Features

- **Stable business identity** — define `Define(10001, "user_not_found")` once and match with `errors.Is(err, UserNotFound)`, without relying on error text.
- **Preserved diagnostics** — create instances with `New(msg)` / `Wrap(cause, msg)`, attach data with `With(key, value)`, and keep using `%w`, `errors.As`, and `errors.Join`.
- **One-line logging** — slog, zap, zerolog, and logrus emit the same structured fields, including cause trees.
- **Explicit public responses** — HTTP/gRPC hide internal diagnostics by default. Choose status, message, and fields at the return boundary, without a mandatory central mapping table.
- **Use only what you need** — no third-party dependencies in the core. Start with the default registry; add separate registries and stack capture when needed.

## Installation

Requires **Go 1.25.0+**.

```bash
go get github.com/im-wmkong/errkind
```

`http` and `slog` are included with the core. Install third-party integrations as needed, for example gRPC:

```bash
go get github.com/im-wmkong/errkind/grpc
```

For other integrations, replace the final `grpc` path segment with `zap`, `zerolog`, `logrus`, or `otel`.

> This guide describes the unreleased v0.2.0 API. These commands cannot yet install the documented version and new integration paths; to try them now, follow the local development instructions under [Contributing](#contributing).

## Capability Map

All import paths start with `github.com/im-wmkong/errkind`.

| Package | Responsibility | Key APIs |
| :--- | :--- | :--- |
| **`errkind`** | Error identity and diagnostics | `Define`, `New`, `Wrap`, `With`, `KindOf / CodeOf / NameOf / MessageOf / AttrsOf`, `NewRegistry`, `CaptureStack / StackOf` |
| **`/slog`, `/zap`, `/zerolog`, `/logrus`** | Structured logging | One-line `Err` / `Fields`; preserve `code / name / message / attrs / stack / causes` |
| **`/http`** | Public HTTP responses | `Write`, `ResponseOf`, `Status / Message / Identity / Field`, optional `Responder` |
| **`/grpc`** | Thin gRPC conversion | `ToStatus`, `FromStatus`, `Code / Message / Identity / Field` |
| **`/otel`** | Trace diagnostics | `RecordError`, `Attributes`, per-call `Prefix` |
| **`cmd/errkind`, `cmd/errkindlint`** | Error-code tooling | Markdown/JSON catalogues and definition checks |

## Quick Start

Put the next three snippets into the same `main.go`, in order, then run `go run .`.

### 1. Define an error kind

```go
package main

import (
    "database/sql"
    "errors"
    "fmt"
    "log/slog"
    "os"

    "github.com/im-wmkong/errkind"
    slogerr "github.com/im-wmkong/errkind/slog"
)

var UserNotFound = errkind.Define(10001, "user_not_found")
```

A `Kind` is a stable identity, usually defined once at package scope. Duplicate codes or names within a registry panic. It carries no default message or protocol status.

### 2. Create an instance and keep its cause

```go
func findUser(id int64) error {
    return UserNotFound.Wrap(sql.ErrNoRows, "lookup user", errkind.With("uid", id))
}
```

Use `Wrap(cause, msg, opts...)` when there is an underlying error, or `New(msg, opts...)` when there is not. The message describes this operation and attributes hold its data. `Wrap(nil, msg)` returns nil.

### 3. Match and log the error

```go
func main() {
    err := findUser(42)
    fmt.Println(errors.Is(err, UserNotFound))
    fmt.Println(errors.Is(err, sql.ErrNoRows))

    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    logger.Error("request failed", slogerr.Err(err))
}
```

Both checks print `true`: the business identity and underlying cause are preserved. The log's `err` field is:

```json
{
  "code": 10001,
  "name": "user_not_found",
  "message": "lookup user",
  "attrs": {"uid": 42},
  "causes": [{"message": "sql: no rows in result set"}]
}
```

Callers only need the shared `UserNotFound` definition to handle this failure, not knowledge of the database. Logs contain internal diagnostics and should not be used directly as client responses.

## Patterns at a Glance

Use the following snippets independently in the appropriate functions or packages, reusing `UserNotFound` above. Do not repeatedly execute Kind definitions.

### Create and inspect structured errors

```go
err := UserNotFound.New("active user is missing",
    errkind.With("uid", 42),
    errkind.With("operation", "profile"),
)

code, ok := errkind.CodeOf(err)
fmt.Println(code, ok)
fmt.Println(errkind.MessageOf(err))
fmt.Println(errkind.AttrsOf(err))
```

This prints `10001 true`, `active user is missing`, and `[{uid 42} {operation profile}]`. `msg` is a required argument; pass `""` when no additional message is needed, or use `fmt.Sprintf` to format it. Repeated attribute keys keep the last value and the first insertion position.

### Connect an existing logger

Your application initializes the logger and chooses the level. Log once at the boundary that handles the error, rather than repeatedly in every layer.

| Framework | Adapter import | Call |
| :--- | :--- | :--- |
| slog | `slogerr "github.com/im-wmkong/errkind/slog"` | `logger.Error("failed", slogerr.Err(err))` |
| zap | `zaperr "github.com/im-wmkong/errkind/zap"` | `logger.Error("failed", zaperr.Err(err))` |
| zerolog | `zeroerr "github.com/im-wmkong/errkind/zerolog"` | `logger.Error().Func(zeroerr.Err(err)).Msg("failed")` |
| logrus | `logruserr "github.com/im-wmkong/errkind/logrus"` | `logger.WithFields(logruserr.Fields(err)).Error("failed")` |

All four integrations use the same nested diagnostic object. Choose JSON output for field-based queries: `NewJSONHandler` for slog or `JSONFormatter` for logrus. Ordinary framework calls such as `zap.Error(err)` do not automatically use errkind's structured tree.

### Choose public content at the HTTP boundary

Place this helper in the handler's package, reusing `UserNotFound`. The handler should log a returned write error rather than attempt a second response.

```go
import (
    "net/http"
    "github.com/im-wmkong/errkind"
    httperr "github.com/im-wmkong/errkind/http"
)

func writeUserError(w http.ResponseWriter, err error, requestID string) error {
    if errkind.KindOf(err) == UserNotFound {
        return httperr.Write(w, err,
            httperr.Status(http.StatusNotFound),
            httperr.Message("User not found"),
            httperr.Identity(),
            httperr.Field("request_id", requestID),
        )
    }
    return httperr.Write(w, err)
}
```

For a missing-user error with `requestID="req-42"`, it returns HTTP 404:

```json
{"code":10001,"name":"user_not_found","message":"User not found","fields":{"request_id":"req-42"}}
```

The internal message, uid, and database cause are not copied. Other non-nil errors default to HTTP 500 and `{"message":"Internal Server Error"}`. `Status` does not set the public message automatically; specify each separately.

For an existing web framework or response envelope, use `ResponseOf(err, opts...)` and pass its `Status` and `Body` to the framework. If endpoints genuinely share presentation rules, use `Responder(func(context.Context, error) []Option)` without a mandatory mapping system.

### Convert gRPC statuses and match on the client

Import `grpcerr "github.com/im-wmkong/errkind/grpc"` and standard gRPC package `google.golang.org/grpc/codes`. This example performs a local conversion round trip; a real client first reads the incoming status with `status.FromError(rpcErr)`.

```go
st := grpcerr.ToStatus(UserNotFound.New("lookup user"),
    grpcerr.Code(codes.NotFound),
    grpcerr.Message("User not found"),
    grpcerr.Identity(),
)

remote := grpcerr.FromStatus(st, errkind.DefaultRegistry())
fmt.Println(errors.Is(remote, UserNotFound))
fmt.Println(errkind.MessageOf(remote))
```

It prints `true` and `User not found`. The server returns `st.Err()`. On receipt, both code and name must match in the supplied registry to bind a local Kind. Pass a nil registry to read remote information without binding identity.

The integration does not install interceptors or manage streams. See [examples/grpc](examples/grpc/main.go) for a real RPC call.

### Separate registries and stack capture

```go
registry := errkind.NewRegistry(errkind.CaptureStack())
DatabaseFailed := registry.Define(20001, "database_failed")
err := DatabaseFailed.New("connect database")
fmt.Printf("%+v\n", err)
```

In application code, create the registry and Kind once at package scope. The default registry does not capture stacks. Create separate registries only when you need definition isolation or stack capture. Identical code/name pairs in different registries are still different Kinds.

### Return multiple failures together

```go
BatchFailed := errkind.Define(30001, "batch_failed")
joined := errors.Join(UserNotFound.New("lookup user"), errors.New("cache unavailable"))

fmt.Println(errors.Is(joined, UserNotFound))
fmt.Println(errkind.KindOf(joined) == nil)

err := BatchFailed.Wrap(joined, "load profile")
fmt.Println(errkind.KindOf(err) == BatchFailed)
```

All three lines print `true`. `errors.Is` asks whether the tree contains a kind; `KindOf` asks for the primary failure. A bare multi-cause Join has no unique primary error. An outer `BatchFailed` explicitly classifies the operation while logs retain every cause branch.

### Record on an existing span

Import `otelerr "github.com/im-wmkong/errkind/otel"`; the application creates `span`:

```go
otelerr.RecordError(span, err, otelerr.Prefix("biz.err."))
```

This adds a standard error event, Error status, and primary-error attributes. The full cause tree is stored in `biz.err.diagnostic`. Use `otelerr.Attributes(err)` if you only need attributes. The default prefix is `err.`; include the trailing dot in custom prefixes.

## Examples and Details

Run these commands from the repository root:

| Topic | Runnable example | Command |
| :--- | :--- | :--- |
| Creation, matching, and logging | [examples/basic](examples/basic/main.go) | `go run ./examples/basic` |
| HTTP handlers and safe responses | [examples/http](examples/http/main.go) | `go run ./examples/http` |
| gRPC server and client | [examples/grpc](examples/grpc/main.go) | `go -C examples/grpc run .` |

The HTTP example listens on `127.0.0.1:8080`: `/user?id=42`, `/user?id=0`, `/user?id=999`, and `/user?id=500` return 200, 400, 404, and 500. It uses Chinese public messages. The gRPC example uses an in-memory connection and needs no external service.

<details>
<summary>Error inspection, attributes, and logging semantics</summary>

- `KindOf / CodeOf / NameOf / MessageOf / AttrsOf` select the outermost business instance along a single chain. If multiple non-nil branches appear first, they do not arbitrarily select one.
- `CodeOf` / `NameOf` use a bool for presence; business code zero is valid. An unbound remote error can have code/name while `KindOf` is nil. A bare Kind is a matching target, not a diagnostic instance; create instances with `New` / `Wrap`.
- `MessageOf` reads only the primary error's message, falling back to `Error()` without a business instance and returning an empty string for nil. Use `Error()` for full text or a logging integration for complete diagnostics.
- `AttrsOf` and `Details.Attrs` return shallow slice copies, not deep copies of maps, slices, or pointers stored as values. Copy before passing values when you need a snapshot, and do not mutate shared values concurrently.
- `AllAttrs` is an explicitly lossy flat view: depth-first, left-to-right, keeping the first value for each key. Default logging does not merge sibling attributes.
- Log objects contain `code / name / message / attrs / stack / causes / truncated`. Empty fields are omitted, zero business codes are kept, and nil is null. Ordinary Go errors retain their full text in message.
- Custom log keys use `slogerr.Value`, `zaperr.Object`, `zeroerr.Field`, or `logruserr.FieldsWithPrefix`. The latter accepts an object key, not an expanded dot-key prefix.
- An unencodable attribute or panicking encoder degrades only that value to `<unencodable type>`. Logs do not redact data automatically; do not store passwords or tokens in attributes.
- Stack settings are fixed at Registry construction. Non-empty cause stacks suppress recapture. `Tracer.StackTrace()` reads this node only; `StackOf` finds the first non-empty stack. Logs attach stacks to their actual owners.

</details>

<details>
<summary>Protocol defaults, precedence, and failure handling</summary>

- HTTP `Identity` publishes the primary code/name; `Field` adds to a separate `fields` object without automatically copying internal attrs. Nil errors write nothing.
- HTTP statuses are limited to 400–599. Invalid status or public-field encoding failure makes `Write` send a safe 500 and return a construction error. Write failures are returned too. `ResponseOf` returns a safe response plus a non-nil error on construction failure, or `(nil, nil)` for nil input.
- `Responder.Write(w, r, err, opts...)` applies safe defaults, the callback, then per-call options. The last assignment to a field wins. Nil errors skip the callback.
- gRPC ordinary/business errors default to Internal and a generic message. Nil input returns nil; `FromStatus` also returns nil for nil/OK.
- Without outer business reclassification or output options, native/remote gRPC statuses preserve code/message/details; their creator owns public-content safety. Context cancellation/deadline on an ordinary single chain preserves standard status codes.
- **Any gRPC output option rebuilds from safe defaults**, without inheriting old details. `Message` alone does not preserve NotFound; also pass `Code` if needed. An outer business instance wrapping status/context does not automatically inherit the inner status either.
- gRPC `Field` accepts strings only; only `Identity` sends business identity. Invalid codes, invalid UTF-8, or reserved `_errkind.*` key writes fall back to generic Internal. Identity uses the fixed `errkind/v1` ErrorInfo marker; unrecognized, duplicate, or malformed identity is not bound, while the original status is preserved.
- Remote binding represents the caller's choice of service contract, not authentication or authorization. Clients should define or import matching Kinds first; different services can use separate registries.
- OTel records only on non-nil recording spans; nil errors are no-ops. Applications log final HTTP/gRPC status separately. OTel preserves integers outside int64 range as JSON text.

</details>

<details>
<summary>Error-code checks, documentation generation, and migration</summary>

Install the tools from the errkind source root, then run subsequent commands in your business project. Ensure Go's binary installation directory is on PATH:

```bash
go install ./cmd/errkind ./cmd/errkindlint
```

```bash
errkindlint ./...
errkind doc -format=md .
errkind doc -format=json -o=errors.json .
```

Lint checks duplicate codes/names, empty names, and scan errors, exiting non-zero on failure. Doc produces Code, Name, and Source, not a replacement for lint. `-o` overwrites its destination, but scan or argument-validation failures do not write it.

The tools recognize two literal arguments to `errkind.Define`, including import aliases. They do not resolve constants, Registry methods, dot imports, or function aliases, execute init, or filter by build tags. Scan independent applications separately; custom registries still rely on runtime registration checks.

When migrating from v0.1.x:

- Replace `DefaultMessage` and core `Message / Messagef` options with `New(msg, opts...)` / `Wrap(cause, msg, opts...)`. HTTP/gRPC public `Message` options remain.
- Replace `Kind.Is(err)` with `errors.Is(err, Kind)` and global stack switches with `NewRegistry(CaptureStack())`.
- Replace `ext/http`, `ext/slog`, and `integration/<name>` with top-level packages. Replace old decorators and gRPC interceptors with explicit boundary conversion.
- Move log queries from flat cause / dot-keys to nested `causes` / `err`. Also check primary-error selection, remote binding, and implicit JSON behavior.

See [CHANGELOG](CHANGELOG.md) for the full history.

</details>

## Known Limitations

- For plain text context, use `fmt.Errorf("operation: %w", err)`; not every layer needs a Kind. The library does not automatically retry or manage request lifecycles.
- Attributes are shallow copies. Logs contain internal information and do not redact automatically. Core errors have no implicit JSON; use logging or protocol APIs.
- Multi-cause errors do not automatically get a primary identity. Core extraction and stack searches inspect at most 256 nodes; log trees are limited to 256 nodes and 64 levels, with `truncated:true` when children are omitted.
- API and package paths have breaking changes from v0.1.x. This candidate is unpublished and provides no old-path forwarding packages.

## Contributing

Issues and Pull Requests are welcome. Before submitting, run these from the repository root:

```bash
bash scripts/test.sh -race -cover -count=2
go run ./cmd/errkindlint -exclude=examples/ .
python3 scripts/verify-release.py
```

The test script covers all seven modules; root `go test ./...` does not cross module boundaries. Consumer verification checks external usage and CLI installation with isolated caches, downloading dependencies but publishing nothing. Run benchmarks with `go test -run='^$' -bench=. -benchmem . ./internal/diagnostic ./slog`.

Keep `README.md` and `README_CN.md` in sync when changing either one.

<details>
<summary>Local development: use unpublished source in another project</summary>

To run the repository example, simply execute `go run ./examples/basic` from this checkout's root.

For testing in another project, replace `ERRKIND_DIR` with this checkout's absolute path. Start in a new empty directory; skip `go mod init` in an existing Go project.

```bash
ERRKIND_DIR=/absolute/path/to/errkind
go mod init example.com/errkind-demo
go mod edit -require=github.com/im-wmkong/errkind@v0.2.0
go mod edit "-replace=github.com/im-wmkong/errkind=$ERRKIND_DIR"
```

`http` and `slog` come with the core. Third-party integrations are separate modules; point each one you use at its source, for example gRPC:

```bash
INTEGRATION=grpc
go mod edit "-require=github.com/im-wmkong/errkind/$INTEGRATION@v0.2.0"
go mod edit "-replace=github.com/im-wmkong/errkind/$INTEGRATION=$ERRKIND_DIR/$INTEGRATION"
```

Replace `grpc` with `zap`, `zerolog`, `logrus`, or `otel` as needed. After adding code and imports, run `go mod tidy` and keep the core module's local replacement. The candidate version is paired with local source, not a claim that it has been published.

</details>

## License

[MIT License](LICENSE).
