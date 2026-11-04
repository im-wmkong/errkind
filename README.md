# ErrKind 🚀

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkind.svg)](https://pkg.go.dev/github.com/im-wmkong/errkind)
[![Go Report Card](https://goreportcard.com/badge/github.com/im-wmkong/errkind)](https://goreportcard.com/report/github.com/im-wmkong/errkind)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**ErrKind** 是一个零第三方依赖的 Go 业务错误库。

用 **Kind 定义错误身份，Error 记录本次失败**，通过消息、属性和原因链说明发生了什么。业务代码通过 `Kind.New / Wrap` 返回实例，用 `errors.Is` 匹配身份，在应用边界通过一次 `errors.As` 从同一实例读取身份、消息和属性。`Kind` 和 `Error` 都是公开接口，注册身份与构造实例采用私有实现，兼容标准包装和 `errors.Join`。

适合需要统一业务错误码、保留底层原因，并为日志或协议出口读取错误信息的场景。

## ✨ 核心特性

- 🪪 **稳定的业务身份** —— `Kind` 用于定义、构造和 `errors.Is` 匹配，同一身份下的实例可以携带不同消息和属性。
- 🔗 **标准原因链** —— `Kind.New / Wrap` 返回可直接读取详情的 `Error`，`Wrap` 保留底层原因，兼容标准包装和多分支错误。
- 🔎 **统一读取错误信息** —— `errors.As` 以 `Error` 接口为目标，从同一实例读取 `Kind / Code / Name / Message / Attr / Attrs`。
- 🏷️ **有序实例属性** —— `With` 创建属性，`Error.Attr` 按键读取，`Error.Attrs` 返回全部属性的浅拷贝；同名键保留最后值和首次位置。
- 📦 **独立或默认注册表** —— 包级 `Define / Kinds` 使用默认 Registry；`NewRegistry` 隔离定义，支持并发注册和目录读取。

## 📦 安装

需要 **Go 1.21 或更新版本**。

```bash
go get github.com/im-wmkong/errkind
```

## 🗺️ 能力总览

| 组件 | 核心职责 | 关键能力 |
| :--- | :--- | :--- |
| **`Kind`** | 描述业务错误身份的公开接口 | `Define` 返回不可变身份；`Code / Name` 读取错误码与名称；`New / Wrap` 创建实例；作为 `errors.Is` 的匹配目标。 |
| **`Error`** | 描述本次失败的公开接口 | `Kind / Code / Name / Message / Attr / Attrs` 读取实例信息；通过标准库 `errors.Is / As / Unwrap` 匹配、提取和访问原因。 |
| **`Registry`** | 管理错误定义 | `NewRegistry` 创建独立注册表；`Define` 检查 code/name 唯一性；`Kinds` 返回注册目录。 |
| **`Attr / Attrs`** | 携带实例参数 | `With(key, value)` 创建属性；`Get` 按键读取最后值；`Clone` 保留全部元素；`Unique` 生成按键去重的副本。 |

→ API 参考：[pkg.go.dev](https://pkg.go.dev/github.com/im-wmkong/errkind)。源码入口见 [kind.go](kind.go)、[error.go](error.go)、[registry.go](registry.go) 和 [attr.go](attr.go)。

## 🚀 快速开始

以查询用户失败为例：

1. 用 `Define` 声明业务错误的 code 和 name。
2. 用 `Wrap` 保留底层原因，添加本次失败的消息与属性。
3. 用 `errors.Is` 匹配身份，用一次 `errors.As` 读取实例的身份和详情。

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

	var detail errkind.Error
	if errors.As(err, &detail) {
		fmt.Println(detail.Code(), detail.Name())
		fmt.Println(detail.Message(), detail.Attrs())
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
10001 user_not_found
lookup user [{uid 42}]
user_not_found(10001): lookup user: sql: no rows in result set
```

> **💡 提示**：`New / Wrap` 的消息按字面文本保存，需要格式化时使用 `fmt.Sprintf`。`Wrap(nil, ...)` 返回 nil，便于在已有错误处理流程中添加业务信息。

## 💡 常用模式

以下片段沿用快速开始中的 `ErrUserNotFound` 和 `findUser`，按需补充导入。

### 返回错误实例

业务失败统一返回 `New / Wrap` 创建的实例。只需要报告业务类别时，使用空消息：

```go
func checkUser() error {
	return ErrUserNotFound.New("")
}
```

需要说明本次失败时，传入消息与属性：

```go
err := ErrUserNotFound.New("missing active user", errkind.With("uid", 42))
fmt.Println(err.Code(), err.Name())         // 10001 user_not_found
fmt.Println(err.Message())                  // missing active user
fmt.Println(err.Attrs())                    // [{uid 42}]
fmt.Println(errors.Is(err, ErrUserNotFound)) // true
fmt.Println(errors.Unwrap(err) == nil)       // true
```

`New / Wrap` 返回 `Error` 接口，可以直接调用 `Kind / Code / Name / Message / Attr / Attrs`。`Code()` 和 `Name()` 分别返回所属 Kind 的错误码和名称，与 `Kind().Code()`、`Kind().Name()` 一致。`New` 与原因非 nil 的 `Wrap` 每次创建独立实例；实例的消息、属性和底层原因由本次调用决定，共享的 Kind 用于匹配业务类别。

有效 Kind 的 `Wrap(nil, ...)` 返回 nil 接口，赋给内置 `error` 或从返回 `error` 的函数传出后仍然是 nil：

```go
var err error = ErrUserNotFound.Wrap(nil, "lookup user")
fmt.Println(err == nil) // true
```

### 从同一实例读取身份和详情

在应用边界，以 `Error` 接口为 `errors.As` 的目标，一次读取所需字段：

```go
for _, err := range []error{checkUser(), findUser(42)} {
	var detail errkind.Error
	if errors.As(err, &detail) {
		fmt.Println(detail.Code(), detail.Name()) // 10001 user_not_found
		fmt.Println(detail.Message(), detail.Attrs())
	}
}
```

`errors.As(err, &detail)` 按标准库的前序、深度优先顺序，返回首先遇到的 `Error` 实例。身份、消息和属性都从该实例读取；需要身份对象时使用 `detail.Kind()`。通过 `errors.Unwrap(err)` 可以访问直接原因。

注册表返回的 Kind 可以安全复制，副本保留相同身份。重新赋值接口变量，或替换 `Kinds()` 返回切片中的元素，不会修改注册定义和已经创建的错误。

### 复制、去重和读取属性

```go
attrs := errkind.Attrs{
	errkind.With("uid", 1),
	errkind.With("operation", "profile"),
	errkind.With("uid", 42),
}

fmt.Println(attrs.Clone())    // [{uid 1} {operation profile} {uid 42}]
fmt.Println(attrs.Unique())   // [{uid 42} {operation profile}]
fmt.Println(attrs.Get("uid")) // 42 true

err := ErrUserNotFound.New("missing active user", attrs...)
fmt.Println(err.Attr("uid"))     // 42 true
fmt.Println(err.Attr("missing")) // <nil> false

returned := err.Attrs()
returned[0].Value = 99
fmt.Println(err.Attrs()) // [{uid 42} {operation profile}]
```

`Get(key)` 返回指定键最后一次设置的值；键不存在时返回 `(nil, false)`，键存在但最后值为 nil 时返回 `(nil, true)`。空字符串也可以作为属性键。

`Clone()` 浅拷贝全部元素，保留顺序与重复键。`Unique()` 返回去重后的浅拷贝，同名键最后的值胜出，位置沿用第一次出现的位置；空集合返回 nil。`Get(key)` 在去重前后的结果一致。

`New / Wrap` 自动通过 `Unique()` 复制和去重属性。`Error.Attrs()` 每次通过 `Clone()` 返回独立的切片，修改切片元素不会改变实例。

`Error.Attr(key)` 委托给实例内部的 `Attrs.Get(key)`，只查询当前实例，不搜索原因链，也不复制整个属性切片。

通过 `Attrs.Get()`、`Error.Attr()` 或 `Error.Attrs()` 读取的 map、slice 和指针仍引用原对象。需要独立的值时，在传入前自行复制。

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

var first errkind.Error
if errors.As(joined, &first) {
	fmt.Println(first.Name()) // user_not_found
}

ErrLoadProfileFailed := errkind.Define(30001, "load_profile_failed")
err := ErrLoadProfileFailed.Wrap(joined, "load profile")
var outer errkind.Error
if errors.As(err, &outer) {
	fmt.Println(outer.Name()) // load_profile_failed
}
fmt.Println(errors.Is(err, ErrUserNotFound)) // true
```

`errors.Is` 搜索原因树中的匹配身份；`errors.As` 以 `Error` 为目标时读取首个实例。外层的 `Wrap` 为整体失败提供明确身份和详情，同时保留下游错误。

`errors.Is(err, target)` 为真，并不意味着随后读取的首个 `Error` 属于 target。需要判断所提取实例的身份时，使用 `detail.Kind()`。一次 `errors.As` 只提取一个实例；记录所有分支的结构化详情需要分别遍历和读取。

标准 `fmt.Errorf("load profile: %w", err)` 也可以补充函数层上下文。通过 `errors.As` 仍可读取链中的 `Error`，以及底层的 `*os.PathError` 等具体类型。

### 在应用边界记录日志

```go
err := findUser(42)
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

var detail errkind.Error
if errors.As(err, &detail) {
	logger = logger.With(
		slog.Uint64("code", detail.Code()),
		slog.String("kind", detail.Name()),
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

该示例从同一个实例记录 code/name、消息和属性。错误树中没有 `Error` 实例时，包括只有裸 Kind 的情况，不附加这些结构化字段，仍通过 `err.Error()` 记录完整错误文本。HTTP/gRPC 状态、公开消息、重试与观测策略由应用按业务场景决定。

### 导出实际注册的错误目录

```go
for _, kind := range errkind.Kinds() {
	fmt.Printf("%d\t%s\n", kind.Code(), kind.Name())
}
```

`Kinds()` 按注册顺序返回切片副本，修改该切片不会改变注册表。应用选择需要导入的业务错误包，并将实际目录导出为自己的 JSON 或 Markdown 格式。

## 🚧 使用约定

- **错误码**：使用 `uint64`，支持完整的 64 位无符号范围，0 是有效值。
- **定义唯一性**：同一个 Registry 内 code 与 name 各自唯一，重复定义或空 name 会 panic。通过 `Define / Registry.Define` 登记 Kind。
- **接口边界**：`Kind` 和 `Error` 都是公开接口。本库注册的身份由私有实现提供，不可变且可比较；构造的实例通过只读方法提供信息。接口允许外部实现，其行为由该实现决定。
- **构造与返回值**：Kind 用于定义、构造和 `errors.Is` 匹配；业务失败统一返回 `New / Wrap` 创建的 `Error`，没有消息时使用 `New("")`。有效 Kind 的 `Wrap(nil, ...)` 返回 nil，转换为内置 `error` 后仍为 nil。
- **边界读取**：以 `Error` 为目标调用一次 `errors.As`，从同一实例读取身份、消息和属性。需要身份对象时使用 `detail.Kind()`。
- **nil 接口**：`Kind` 和 `Error` 的零值都是 nil 接口，直接调用方法会 panic。nil Kind 调用 `Wrap(nil, ...)` 也会 panic。
- **消息与格式化**：消息按字面文本保存。错误文本由 `name(code)`、非空消息与非 nil 原因组成，`%v / %s / %+v` 使用相同文本。标准包装、`errors.Join` 和自定义原因保留各自的 `Error()` 文本；文本不缓存，会反映底层原因的当前内容。
- **属性所有权**：构造实例和调用 `Attrs()` 会复制属性切片，`Attr(key)` 直接返回对应值。引用类型的属性值仍共享原对象；共享值的并发访问由调用方同步。
- **并发使用**：Registry 支持并发定义与目录读取，零值可直接使用，使用后不可复制。实例信息通过只读方法访问；底层原因的并发行为由其实现决定。

### 裸 Kind 与实例提取

Kind 实现了 `error` 接口，可以直接作为错误值返回，也可以通过 `errors.As(err, &kind)` 读取首个业务身份。推荐业务返回和边界处理采用上面的实例路径。本库注册的裸 Kind 不满足 `Error` 接口；错误树中没有实例时，`errors.As(err, &detail)` 返回 false。

身份查询与实例查询各自遵循遍历规则。例如 `errors.Join(ErrUserNotFound, ErrCacheUnavailable.New("load cache"))` 的首个身份是 `ErrUserNotFound`，首个实例却属于 `ErrCacheUnavailable`，不能直接组合两次查询的字段。以 `Error` 为唯一提取入口时，读取的是 `ErrCacheUnavailable` 实例自身的身份和详情。

## 📚 示例

完整可运行示例见 [`examples/main.go`](examples/main.go)，包含业务身份匹配、底层原因匹配、单次实例提取和 JSON 日志。

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

CI 在 Go 1.21 和最新稳定版下覆盖 Ubuntu 与 macOS，执行 vet、构建、竞态测试和示例，要求核心覆盖率不低于 90%；静态检查与漏洞扫描使用稳定版工具链。

## 📄 开源协议

[MIT License](LICENSE)。
