# ErrKind 🚀

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkind.svg)](https://pkg.go.dev/github.com/im-wmkong/errkind)
[![Go Report Card](https://goreportcard.com/badge/github.com/im-wmkong/errkind)](https://goreportcard.com/report/github.com/im-wmkong/errkind)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**ErrKind** 是一个零第三方依赖的 Go 业务错误库。

用 **Kind 定义错误身份，Error 记录本次失败**，通过消息、属性和原因链说明发生了什么。错误匹配与实例读取直接使用标准库的 `errors.Is / As / Unwrap`，业务代码可以继续组合 `fmt.Errorf` 和 `errors.Join`。

适合需要统一业务错误码、保留底层原因，并为日志或协议出口读取错误信息的场景。

## ✨ 核心特性

- 🪪 **稳定的业务身份** —— `Kind` 同时是错误值与匹配目标；通过 `errors.Is` 按身份判断，实例消息和属性可以各不相同。
- 🔗 **标准原因链** —— `Kind.New / Wrap` 创建错误实例，`Wrap` 保留底层原因，兼容标准包装和多分支错误。
- 🔎 **直接读取实例** —— `errors.As` 取得 `*Error`，通过 `Kind / Message / Attrs` 读取身份、消息和参数。
- 🏷️ **有序实例属性** —— `With` 创建属性，`Attrs.Clone / Unique` 提供浅拷贝与按键去重；同名键保留最后值和首次位置。
- 📦 **独立或默认注册表** —— 包级 `Define / Kinds` 使用默认 Registry；`NewRegistry` 隔离定义，支持并发注册和目录读取。

## 📦 安装

需要 **Go 1.21 或更新版本**。

```bash
go get github.com/im-wmkong/errkind
```

## 🗺️ 能力总览

| 组件 | 核心职责 | 关键能力 |
| :--- | :--- | :--- |
| **`Kind`** | 定义稳定的错误身份 | `Define` 注册身份；`Code / Name` 读取错误码与名称；`New / Wrap` 创建实例。 |
| **`Error`** | 记录本次失败 | `Kind / Message / Attrs` 读取实例信息；`Error / Is / Unwrap` 接入标准错误机制。 |
| **`Registry`** | 管理错误定义 | `NewRegistry` 创建独立注册表；`Define` 检查 code/name 唯一性；`Kinds` 返回注册目录。 |
| **`Attr / Attrs`** | 携带实例参数 | `With(key, value)` 创建属性；`Clone` 保留全部元素；`Unique` 生成按键去重的副本。 |
| **`Code`** | 表达业务错误码 | 基于 `uint64` 的命名类型，支持完整的 64 位无符号范围，0 是有效值。 |

→ API 参考：[pkg.go.dev](https://pkg.go.dev/github.com/im-wmkong/errkind)。源码入口见 [kind.go](kind.go)、[error.go](error.go)、[registry.go](registry.go) 和 [attr.go](attr.go)。

## 🚀 快速开始

以查询用户失败为例：

1. 用 `Define` 声明业务错误的 code 和 name。
2. 用 `Wrap` 保留底层原因，添加本次失败的消息与属性。
3. 用 `errors.Is` 匹配身份，用 `errors.As` 读取实例。

将下面的代码保存为 `main.go`：

```go
package main

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/im-wmkong/errkind"
)

var ErrUserNotFound = errkind.Define(10001, "user_not_found")

func findUser(id int64) error {
	return ErrUserNotFound.Wrap(sql.ErrNoRows, "lookup user", errkind.With("uid", id))
}

func main() {
	err := findUser(42)
	fmt.Println(errors.Is(err, ErrUserNotFound))
	fmt.Println(errors.Is(err, sql.ErrNoRows))

	var detail *errkind.Error
	if errors.As(err, &detail) {
		fmt.Println(detail.Kind().Code(), detail.Kind().Name(), detail.Message(), detail.Attrs())
	}
	fmt.Println(err)
}
```

```bash
go run main.go
```

输出：

```text
true
true
10001 user_not_found lookup user [{uid 42}]
user_not_found(10001): lookup user: sql: no rows in result set
```

> **💡 提示**：`New / Wrap` 的消息按字面文本保存，需要格式化时使用 `fmt.Sprintf`。`Wrap(nil, ...)` 返回 nil，便于在已有错误处理流程中添加业务信息。

## 💡 常用模式

以下片段沿用快速开始中的 `ErrUserNotFound` 和 `findUser`，按需补充导入。

### 直接返回 Kind，或创建错误实例

只需要报告业务类别时，可以直接从函数返回 Kind：

```go
func checkUser() error {
	return ErrUserNotFound
}
```

裸 Kind 可通过 `errors.Is` 匹配，也可通过 `errors.As` 读取 `*Kind`。它只包含身份信息；`errors.As` 查找 `*Error` 时，需要链中存在错误实例。

需要说明本次失败时，使用 `New` 创建实例：

```go
err := ErrUserNotFound.New("missing active user", errkind.With("uid", 42))
fmt.Println(errors.Is(err, ErrUserNotFound)) // true
fmt.Println(errors.Unwrap(err) == nil)      // true
```

`New` 与原因非 nil 的 `Wrap` 每次创建独立实例；实例的消息、属性和底层原因由本次调用决定，共享的 Kind 用于匹配业务类别。

### 复制、去重和读取属性

```go
attrs := errkind.Attrs{
	errkind.With("uid", 1),
	errkind.With("operation", "profile"),
	errkind.With("uid", 42),
}

fmt.Println(attrs.Clone())  // [{uid 1} {operation profile} {uid 42}]
fmt.Println(attrs.Unique()) // [{uid 42} {operation profile}]

err := ErrUserNotFound.New("missing active user", attrs...)
var detail *errkind.Error
if errors.As(err, &detail) {
	returned := detail.Attrs()
	returned[0].Value = 99
	fmt.Println(detail.Attrs()) // [{uid 42} {operation profile}]
}
```

`Clone()` 浅拷贝全部元素，保留顺序与重复键。`Unique()` 返回去重后的浅拷贝，同名键最后的值胜出，位置沿用第一次出现的位置；空集合返回 nil。

`New / Wrap` 自动通过 `Unique()` 复制和去重属性。`Error.Attrs()` 每次通过 `Clone()` 返回独立的切片，修改切片元素不会改变实例。

属性值中的 map、slice 和指针仍引用原对象。需要独立的值时，在传入前自行复制。

### 用独立 Registry 隔离定义

```go
registry := errkind.NewRegistry()
localNotFound := registry.Define(10001, "user_not_found")
err := localNotFound.New("lookup user")

fmt.Println(errors.Is(err, localNotFound))   // true
fmt.Println(errors.Is(err, ErrUserNotFound)) // false
```

不同 Registry 中即使 code/name 相同，也属于不同身份。包级 `Define / Kinds` 使用同一个默认 Registry；独立注册表适合应用内不同定义域或测试隔离。

### 组合多个错误，并为整体失败指定身份

```go
ErrCacheUnavailable := errkind.Define(10002, "cache_unavailable")
joined := errors.Join(
	ErrUserNotFound.New("lookup user"),
	ErrCacheUnavailable.New("load cache"),
)

fmt.Println(errors.Is(joined, ErrUserNotFound))     // true
fmt.Println(errors.Is(joined, ErrCacheUnavailable)) // true

var first *errkind.Error
if errors.As(joined, &first) {
	fmt.Println(first.Kind().Name()) // user_not_found
}

ErrLoadProfileFailed := errkind.Define(30001, "load_profile_failed")
err := ErrLoadProfileFailed.Wrap(joined, "load profile")
var outer *errkind.Error
if errors.As(err, &outer) {
	fmt.Println(outer.Kind().Name()) // load_profile_failed
}
fmt.Println(errors.Is(err, ErrUserNotFound)) // true
```

`errors.Is` 搜索原因树中的匹配身份；`errors.As` 按 Go 标准规则深度优先查找，返回第一个匹配实例。外层的 `Wrap` 为整体失败提供明确身份，同时保留下游错误。

标准 `fmt.Errorf("load profile: %w", err)` 也可以补充函数层上下文。通过 `errors.As` 仍可读取链中的 `*Error` 或底层的 `*os.PathError` 等具体类型。

### 在应用边界记录日志

```go
err := findUser(42)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

var detail *errkind.Error
if errors.As(err, &detail) {
	logger = logger.With(
		slog.Uint64("code", uint64(detail.Kind().Code())),
		slog.String("kind", detail.Kind().Name()),
		slog.String("message", detail.Message()),
		slog.Any("attrs", detail.Attrs()),
	)
}
logger.Error("request failed",
	slog.String("request_id", "req-1"),
	slog.String("err", err.Error()),
)
```

该片段需要导入 `log/slog` 和 `os`。

应用直接选择需要记录的字段，完整错误文本使用 `err.Error()`。HTTP/gRPC 状态、公开消息、重试与观测策略也由应用按业务场景决定。

### 导出实际注册的错误目录

```go
for _, kind := range errkind.Kinds() {
	fmt.Printf("%d\t%s\n", kind.Code(), kind.Name())
}
```

`Kinds()` 按注册顺序返回切片副本，修改该切片不会改变注册表。应用选择需要导入的业务错误包，并将实际目录导出为自己的 JSON 或 Markdown 格式。

## 🚧 使用约定

- **定义唯一性**：同一个 Registry 内 code 与 name 各自唯一，重复定义或空 name 会 panic。通过 `Define / Registry.Define` 登记 Kind。
- **构造与返回值**：`Error` 应通过 `Kind.New / Wrap` 创建，其零值无效。两个方法返回 `error`；`Wrap(nil, ...)` 返回 nil。
- **nil Kind**：`New` 或携带非 nil 原因的 `Wrap` 会 panic；`Wrap(nil, ...)` 仍返回 nil。
- **消息与格式化**：消息按字面文本保存。错误文本由 `name(code)`、非空消息与非 nil 原因组成，`%v / %s / %+v` 使用相同文本。
- **属性所有权**：构造和读取都会复制切片，引用类型的属性值仍共享原对象；共享值的并发访问由调用方同步。
- **并发使用**：Registry 支持并发定义与目录读取，零值可直接使用，使用后不可复制。实例信息通过只读方法访问；底层原因的并发行为由其实现决定。

## 📚 示例

完整可运行示例见 [`examples/main.go`](examples/main.go)，包含业务身份匹配、底层原因匹配、实例读取和 JSON 日志。

在仓库根目录运行：

```bash
go run ./examples
```

## 🤝 参与贡献

欢迎提交 Issue 与 Pull Request。提交前请运行：

```bash
go vet ./...
go test -race ./...
go run ./examples
```

性能基准：

```bash
go test -run='^$' -bench=. -benchmem .
```

CI 在 Go 1.21 下覆盖 Ubuntu 与 macOS，执行 vet、构建、竞态测试和示例，要求核心覆盖率不低于 90%；静态检查与漏洞扫描使用稳定版工具链。

## 📄 开源协议

[MIT License](LICENSE)。
