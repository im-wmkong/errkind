# ErrKind 🚀

[![Go Reference](https://pkg.go.dev/badge/github.com/im-wmkong/errkind.svg)](https://pkg.go.dev/github.com/im-wmkong/errkind)
[![Go Report Card](https://goreportcard.com/badge/github.com/im-wmkong/errkind)](https://goreportcard.com/report/github.com/im-wmkong/errkind)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

**ErrKind** 是一个零第三方依赖的 Go 业务错误库：用 **Kind 定义错误身份，Error 记录本次失败**。

通过 `New / Wrap` 创建实例，`errors.Is` 匹配身份，`errors.As` 读取消息和属性。支持不可变的属性补充、独立注册表、标准包装和 `errors.Join`。

## 📦 安装

需要 **Go 1.21 或更新版本**。

```bash
go get github.com/im-wmkong/errkind
```

## 🚀 快速开始

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
	fmt.Println(errors.Is(err, ErrUserNotFound)) // true
	fmt.Println(errors.Is(err, sql.ErrNoRows))   // true

	var detail errkind.Error
	if errors.As(err, &detail) {
		fmt.Println(detail.Code(), detail.Name())     // 10001 user_not_found
		fmt.Println(detail.Message(), detail.Attrs()) // lookup user [{uid 42}]
	}
	fmt.Println(err) // user_not_found(10001): lookup user: sql: no rows in result set
}
```

```bash
go run main.go
```

## 🏷️ 属性

以下片段沿用快速开始中的 `ErrUserNotFound`。

### 属性集合

```go
attrs := errkind.Attrs{errkind.With("uid", 1), errkind.With("uid", 42)}
merged := attrs.Merge(errkind.With("uid", 99), errkind.With("attempt", 2))

fmt.Println(attrs)  // [{uid 1} {uid 42}]
fmt.Println(merged) // [{uid 99} {attempt 2}]
```

| 方法 | 行为 |
| :--- | :--- |
| `Get(key)` | 返回最后一个值；不存在时返回 `(nil, false)`，键存在且值为 nil 时返回 `(nil, true)`。 |
| `Clone()` | 浅拷贝全部元素，保留顺序和重复键。 |
| `Unique()` | 返回去重副本，同名键保留最后值和首次位置。 |
| `Merge(attrs...)` | 合并后返回去重副本，不修改输入；无参数时等价于 `Unique()`。 |

`New / Wrap` 和 `WithAttrs` 使用相同的去重规则。`Unique / Merge` 的空结果为 nil。

### 补充实例属性

```go
original := ErrUserNotFound.New("lookup user", errkind.With("uid", 42))
updated := original.WithAttrs(errkind.With("uid", 99), errkind.With("attempt", 2))

fmt.Println(original.Attrs())    // [{uid 42}]
fmt.Println(updated.Attrs())     // [{uid 99} {attempt 2}]
fmt.Println(updated.Attr("uid")) // 99 true
```

- `WithAttrs` 有参数时返回新实例，需接收返回值；无参数或传入空切片时返回原实例。
- 新实例保留 Kind、消息和原 cause，不增加错误层级，也不保留原实例的指针身份。需要增加业务身份或文字上下文时使用 `Wrap`。
- `Attr(key)` 按 `Get` 的规则读取，`WithAttrs` 合并属性，二者都只作用于当前实例，不搜索原因链。
- 属性切片会复制，`Attrs()` 也返回副本；map、slice、指针等属性值仍共享，并发访问由调用方同步。

> 从标准包装或 `errors.Join` 中提取实例后，不要用其 `WithAttrs` 返回值替换整棵错误树，否则会丢失外围包装或其他分支。

## 📦 注册表

```go
registry := errkind.NewRegistry()
localNotFound := registry.Define(10001, "user_not_found")
err := localNotFound.New("lookup user")

fmt.Println(errors.Is(err, localNotFound))   // true
fmt.Println(errors.Is(err, ErrUserNotFound)) // false
```

不同 Registry 中即使 code/name 相同，也属于不同身份。包级 `Define / Kinds` 使用同一个默认 Registry；独立注册表适合应用内不同定义域或测试隔离。

`Kinds()` 按注册顺序返回目录的切片副本，可用于导出错误码：

```go
for _, kind := range errkind.Kinds() {
	fmt.Printf("%d\t%s\n", kind.Code(), kind.Name())
}
```

## 🚧 使用约定

- **定义**：code 为 `uint64`，0 有效。同一 Registry 内 code/name 各自唯一，重复定义或空 name 会 panic。
- **返回值**：业务代码返回 `New / Wrap` 创建的 `Error`，无需消息时用 `New("")`。本库的裸 Kind 虽然实现 `error`，但不满足 `Error` 接口。
- **nil**：有效 Kind 的 `Wrap(nil, ...)` 返回 nil；nil `Kind / Error` 不能调用方法，包括 `WithAttrs`。
- **错误链**：标准包装和 `errors.Join` 遵循 `errors.Is / As` 遍历规则。以 `Error` 为目标的 `errors.As` 只提取首个实例；它不一定是 `errors.Is` 匹配的身份，应通过 `detail.Kind()` 确认。
- **消息**：按字面文本保存，格式化用 `fmt.Sprintf`。错误文本包含身份、消息和原因，不包含属性；协议映射与日志策略由应用决定。
- **并发**：Registry 支持并发定义和读取，使用后不可复制。属性引用值和底层原因的并发安全由其所有者负责。
- **接口**：`Kind / Error` 允许外部实现；已有自定义 `Error` 实现和 mock 需补充 `WithAttrs` 方法。

## 📚 示例与开发

- [API 参考](https://pkg.go.dev/github.com/im-wmkong/errkind)
- [完整示例](examples/main.go)：身份匹配、原因链、详情提取和 JSON 日志。

在仓库根目录运行：

```bash
go vet ./...
go test -race ./...
go run ./examples
```

## 📄 开源协议

[MIT License](LICENSE)。
