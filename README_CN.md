# errkind

[English](README.md) | 简体中文

**errkind** 是一个兼容 Go 标准错误机制的业务错误库，提供稳定的错误分类、结构化诊断，以及按需接入的日志和协议适配。

通过 `Kind` 判断“发生了哪类错误”，通过消息、属性和原因记录“这次为什么失败”，再由接口出口决定“客户端能看到什么”。业务代码不需要依赖 HTTP 状态码，也不需要为每种错误编写结构体。

## 核心特性

- **稳定的业务分类** —— `Define(10001, "user_not_found")` 定义一次，使用 `errors.Is(err, UserNotFound)` 判断，不依赖错误文本。
- **完整保留诊断** —— `New(msg)` / `Wrap(cause, msg)` 创建实例，`With(key, value)` 补充参数，兼容 `%w`、`errors.As` 和 `errors.Join`。
- **一行接入日志** —— 支持 slog、zap、zerolog、logrus，统一输出包含原因树的结构化字段。
- **出口选择公开内容** —— HTTP/gRPC 默认隐藏内部诊断，在返回位置指定状态、消息和字段，不要求集中维护映射表。
- **按需引入** —— 核心零第三方依赖；默认注册中心即可上手，多个注册中心和调用栈按需启用。

## 安装

要求 **Go 1.25.0+**。

```bash
go get github.com/im-wmkong/errkind
```

`http`、`slog` 随核心提供，无需额外安装。第三方集成按需安装，例如 gRPC：

```bash
go get github.com/im-wmkong/errkind/grpc
```

其他集成将路径末尾的 `grpc` 换成 `zap`、`zerolog`、`logrus` 或 `otel` 即可。

> 本文对应尚未发布的 v0.2.0 API，当前 `go get` 尚不能安装本文所述版本及新集成路径；提前体验请使用[参与贡献](#参与贡献)中的本地联调步骤。

## 能力总览

所有包的导入路径均以 `github.com/im-wmkong/errkind` 开头。

| 包 | 核心职责 | 关键 API |
| :--- | :--- | :--- |
| **`errkind`** | 错误身份与诊断 | `Define`、`New`、`Wrap`、`With`、`KindOf / CodeOf / NameOf / MessageOf / AttrsOf`、`NewRegistry`、`CaptureStack / StackOf` |
| **`/slog`、`/zap`、`/zerolog`、`/logrus`** | 结构化日志 | `Err` / `Fields` 一行接入；保留 `code / name / message / attrs / stack / causes` |
| **`/http`** | HTTP 公开响应 | `Write`、`ResponseOf`、`Status / Message / Identity / Field`、可选 `Responder` |
| **`/grpc`** | gRPC 薄转换 | `ToStatus`、`FromStatus`、`Code / Message / Identity / Field` |
| **`/otel`** | Trace 错误诊断 | `RecordError`、`Attributes`、调用级 `Prefix` |
| **`cmd/errkind`、`cmd/errkindlint`** | 错误码工具 | 生成 Markdown/JSON 目录、检查定义冲突 |

## 快速开始

以下三段代码依次放入同一个 `main.go`，然后执行 `go run .`。

### 1. 定义错误种类

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

`Kind` 只代表稳定身份，通常在包级定义一次。同一注册中心内 code 或 name 重复会 panic；它不携带默认消息或协议状态。

### 2. 创建本次错误，保留底层原因

```go
func findUser(id int64) error {
    return UserNotFound.Wrap(sql.ErrNoRows, "lookup user", errkind.With("uid", id))
}
```

有底层原因用 `Wrap(cause, msg, opts...)`，没有原因用 `New(msg, opts...)`。消息描述本次操作，属性记录本次参数；`Wrap(nil, msg)` 返回 nil。

### 3. 判断错误并记录日志

```go
func main() {
    err := findUser(42)
    fmt.Println(errors.Is(err, UserNotFound))
    fmt.Println(errors.Is(err, sql.ErrNoRows))

    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    logger.Error("request failed", slogerr.Err(err))
}
```

两次判断都输出 `true`，业务身份和底层原因均被保留。日志中的 `err` 字段：

```json
{
  "code": 10001,
  "name": "user_not_found",
  "message": "lookup user",
  "attrs": {"uid": 42},
  "causes": [{"message": "sql: no rows in result set"}]
}
```

调用方引用同一个 `UserNotFound` 即可判断，无需知道数据库细节。日志包含内部诊断，不应直接作为客户端响应。

## 常用模式

下列片段按需放入业务函数或对应包中，复用上面的 `UserNotFound`；每个片段独立使用，不重复执行定义。

### 创建和读取结构化错误

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

依次输出 `10001 true`、`active user is missing`、`[{uid 42} {operation profile}]`。`msg` 是固定参数；不需要额外消息时传 `""`，格式化使用 `fmt.Sprintf`。同名属性后值覆盖前值，保留首次插入的位置。

### 接入已有日志框架

应用负责初始化 logger 和选择日志级别；在处理错误的边界记录一次即可，不必每层重复打印。

| 框架 | 导入适配器 | 调用 |
| :--- | :--- | :--- |
| slog | `slogerr "github.com/im-wmkong/errkind/slog"` | `logger.Error("failed", slogerr.Err(err))` |
| zap | `zaperr "github.com/im-wmkong/errkind/zap"` | `logger.Error("failed", zaperr.Err(err))` |
| zerolog | `zeroerr "github.com/im-wmkong/errkind/zerolog"` | `logger.Error().Func(zeroerr.Err(err)).Msg("failed")` |
| logrus | `logruserr "github.com/im-wmkong/errkind/logrus"` | `logger.WithFields(logruserr.Fields(err)).Error("failed")` |

四种集成使用相同的嵌套诊断对象。要按字段检索日志，请使用 JSON 输出；slog 选 `NewJSONHandler`，logrus 选 `JSONFormatter`。普通 `zap.Error(err)` 等框架原有写法不会自动使用 errkind 的结构化树。

### 在 HTTP 出口选择公开内容

下面的辅助函数放在 handler 所在包，复用 `UserNotFound`；handler 将返回的写入错误记入日志，不再尝试发送第二份响应。

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
            httperr.Message("用户不存在"),
            httperr.Identity(),
            httperr.Field("request_id", requestID),
        )
    }
    return httperr.Write(w, err)
}
```

对用户不存在的错误传入 `requestID="req-42"`，返回 HTTP 404：

```json
{"code":10001,"name":"user_not_found","message":"用户不存在","fields":{"request_id":"req-42"}}
```

内部的消息、uid 和数据库原因不会被复制。其他非 nil 错误默认返回 HTTP 500、`{"message":"Internal Server Error"}`。`Status` 不会自动设置公开消息，二者分别指定。

已有框架或响应体格式时，用 `ResponseOf(err, opts...)` 获取 `Status` 和 `Body` 后交给框架。多个接口确有共同文案规则时，可用 `Responder(func(context.Context, error) []Option)`，不需要建立强制映射中心。

### gRPC 状态转换与客户端判断

使用适配器 `grpcerr "github.com/im-wmkong/errkind/grpc"` 和标准包 `google.golang.org/grpc/codes`。下面演示本地转换往返；真实客户端先用 `status.FromError(rpcErr)` 取出收到的状态。

```go
st := grpcerr.ToStatus(UserNotFound.New("lookup user"),
    grpcerr.Code(codes.NotFound),
    grpcerr.Message("用户不存在"),
    grpcerr.Identity(),
)

remote := grpcerr.FromStatus(st, errkind.DefaultRegistry())
fmt.Println(errors.Is(remote, UserNotFound))
fmt.Println(errkind.MessageOf(remote))
```

输出 `true` 和 `用户不存在`。服务端返回 `st.Err()`；客户端在指定注册中心中同时匹配 code/name 后，才能绑定本地 Kind。传 nil 注册中心则只读取远端信息，不绑定身份。

gRPC 集成不安装拦截器、不接管流。完整 RPC 调用见 [examples/grpc](examples/grpc/main.go)。

### 独立注册中心与调用栈

```go
registry := errkind.NewRegistry(errkind.CaptureStack())
DatabaseFailed := registry.Define(20001, "database_failed")
err := DatabaseFailed.New("connect database")
fmt.Printf("%+v\n", err)
```

实际应用中将 registry 和 Kind 在包级创建一次。默认注册中心不抓栈；需要隔离错误定义或启用抓栈时才创建独立注册中心。不同注册中心即使 code/name 相同，也不是同一个 Kind。

### 多个错误一起返回

```go
BatchFailed := errkind.Define(30001, "batch_failed")
joined := errors.Join(UserNotFound.New("lookup user"), errors.New("cache unavailable"))

fmt.Println(errors.Is(joined, UserNotFound))
fmt.Println(errkind.KindOf(joined) == nil)

err := BatchFailed.Wrap(joined, "load profile")
fmt.Println(errkind.KindOf(err) == BatchFailed)
```

三行都是 `true`。`errors.Is` 问“树里有没有这种错误”，`KindOf` 问“整个失败的主错误是什么”。裸多分支 Join 没有唯一主错误；外层 `BatchFailed` 明确分类后，日志仍保留所有原因分支。

### 记录到已有 Span

使用适配器 `otelerr "github.com/im-wmkong/errkind/otel"`，`span` 由应用创建：

```go
otelerr.RecordError(span, err, otelerr.Prefix("biz.err."))
```

添加标准错误事件、Error 状态和主错误属性；完整原因树放在 `biz.err.diagnostic`。只需要属性时用 `otelerr.Attributes(err)`。默认前缀为 `err.`，自定义前缀的句点需自行包含。

## 示例与详细说明

以下命令在仓库根目录执行：

| 主题 | 可运行示例 | 命令 |
| :--- | :--- | :--- |
| 创建、匹配与日志 | [examples/basic](examples/basic/main.go) | `go run ./examples/basic` |
| HTTP handler 与安全响应 | [examples/http](examples/http/main.go) | `go run ./examples/http` |
| gRPC 服务端与客户端 | [examples/grpc](examples/grpc/main.go) | `go -C examples/grpc run .` |

HTTP 示例监听 `127.0.0.1:8080`；`/user?id=42`、`/user?id=0`、`/user?id=999`、`/user?id=500` 分别返回 200、400、404、500。gRPC 示例使用内存连接，不需要外部服务。

<details>
<summary>错误读取、属性与日志的语义</summary>

- `KindOf / CodeOf / NameOf / MessageOf / AttrsOf` 沿单链取最外层业务实例；在找到实例前遇到多个非空分支时，不任取其中一支。
- `CodeOf` / `NameOf` 用 bool 表示是否存在，业务码 0 合法。未绑定的远端错误可以有 code/name，但 `KindOf` 为 nil。裸 Kind 是匹配目标，不是诊断实例，应通过 `New` / `Wrap` 创建实例。
- `MessageOf` 只读当前主错误的消息；无业务实例时回退到 `Error()`，nil 返回空字符串。完整文本用 `Error()`，完整诊断用日志集成。
- `AttrsOf` 和 `Details.Attrs` 返回切片浅拷贝，不深拷贝 map、slice 或指针等属性值；需要快照时在传入前复制，不要并发修改共享值。
- `AllAttrs` 是显式有损的扁平视图，深度优先、从左到右，同名属性首先遇到的值胜出。默认日志不会合并兄弟属性。
- 日志对象包含 `code / name / message / attrs / stack / causes / truncated`；空字段省略，零业务码保留，nil 记录为 null。普通 Go 错误的 message 保留其完整文本。
- 自定义日志 key 使用 `slogerr.Value`、`zaperr.Object`、`zeroerr.Field`、`logruserr.FieldsWithPrefix`；后者的参数是对象 key，不是展开后的点号前缀。
- 属性编码失败或编码器 panic 时，仅该值降级为 `<unencodable 类型>`。日志不会自动脱敏，密码和令牌等不应写入属性。
- 抓栈配置在 Registry 创建时固定；原因中已有非空栈则不重复采集。`Tracer.StackTrace()` 只读本节点，`StackOf` 搜索第一个非空栈，日志在实际持有者上记录栈。

</details>

<details>
<summary>协议默认值、覆盖顺序与失败处理</summary>

- HTTP 的 `Identity` 才公开主错误 code/name，`Field` 放入独立 `fields` 对象，不自动复制内部 attrs。nil 错误不写任何内容。
- HTTP 状态限 400–599。非法状态或公开字段编码失败时，`Write` 发送安全 500 并返回构造错误；写入失败也返回错误。`ResponseOf` 构造失败返回安全响应和非 nil 错误，nil 输入返回 `(nil, nil)`。
- `Responder.Write(w, r, err, opts...)` 按“安全默认值 → 回调 → 本次选项”覆盖，同名字段最后值胜出；nil 错误不执行回调。
- gRPC 普通/业务错误默认 Internal 和通用消息；nil 输入返回 nil，`FromStatus` 对 nil/OK 也返回 nil。
- 无外层业务重新分类且无出口选项时，gRPC 原生/远端 status 保留原始 code/message/details，创建者负责公开内容安全；普通单链中的 context 取消/超时保留标准状态。
- gRPC **任何出口选项都会从安全默认值重建**，不继承旧 details。只传 `Message` 不会保留旧 NotFound，需要时同时传 `Code`。外层业务实例包装 status/context 时也不自动继承内层状态。
- gRPC 的 `Field` 仅接受字符串；`Identity` 才发送业务身份。非法状态、非法 UTF-8 或 `_errkind.*` 保留键写入回退为通用 Internal。身份使用 ErrorInfo 的固定 `errkind/v1` 标记，陌生、重复或畸形身份不绑定，原始 status 保留。
- 远端绑定表示调用方选择了某个服务契约，不是认证或授权。客户端应预先定义或导入对应 Kind；不同服务的契约可用不同 Registry。
- OTel 只对非 nil、可记录的 span 工作；nil 错误不做任何操作。HTTP/gRPC 最终状态由应用单独记录，OTel 属性超出 int64 的整数以 JSON 文本保存。

</details>

<details>
<summary>错误码检查、文档生成与版本迁移</summary>

在 errkind 源码根目录安装工具，再进入业务项目执行后续命令；确保 Go 二进制安装目录位于 PATH：

```bash
go install ./cmd/errkind ./cmd/errkindlint
```

```bash
errkindlint ./...
errkind doc -format=md .
errkind doc -format=json -o=errors.json .
```

lint 检查重复 code/name、空 name 和扫描错误，失败非零退出。doc 生成 Code、Name、Source，不替代 lint；`-o` 会覆盖目标文件，扫描或参数校验失败时不写入。

工具仅识别 `errkind.Define` 的两个字面量参数，支持 import alias，不解析常量引用、Registry 方法、点导入或函数别名，不执行 init 或按 build tags 筛选。独立应用分开扫描，自定义 Registry 仍依赖运行时注册检查。

从 v0.1.x 迁移时：

- `DefaultMessage`、核心 `Message / Messagef` 改为 `New(msg, opts...)` / `Wrap(cause, msg, opts...)`；HTTP/gRPC 的公开 `Message` 选项保留。
- `Kind.Is(err)` 改为 `errors.Is(err, Kind)`，全局抓栈开关改为 `NewRegistry(CaptureStack())`。
- `ext/http`、`ext/slog`、`integration/<name>` 改为顶层包；旧装饰器和 gRPC 拦截器改为出口显式转换。
- 日志从扁平 cause / dot-key 改为嵌套 `causes` / `err`；更新日志查询，同时检查主错误选择、远端绑定和隐式 JSON 的行为变化。

完整变更见 [CHANGELOG](CHANGELOG.md)。

</details>

## 当前限制

- 仅需文本上下文时直接用 `fmt.Errorf("operation: %w", err)`，不必为每一层定义 Kind；库不自动重试，也不接管请求生命周期。
- 属性只做浅拷贝；日志含内部信息，不负责自动脱敏；核心错误不提供隐式 JSON，请使用日志或协议接口。
- 多分支错误不自动选主错误。核心提取与栈查询最多检查 256 个节点；日志树最多 256 节点、64 层，省略子节点时标记 `truncated:true`。
- 当前 API 和包路径相对 v0.1.x 有破坏性变更，候选版尚未发布，不提供旧路径转发。

## 参与贡献

欢迎提交 Issue 和 Pull Request。提交前在仓库根目录执行：

```bash
bash scripts/test.sh -race -cover -count=2
go run ./cmd/errkindlint -exclude=examples/ .
python3 scripts/verify-release.py
```

测试脚本覆盖全部 7 个 module；根目录 `go test ./...` 不会跨 module。消费验证脚本验证隔离缓存下的外部接入与 CLI 安装，会下载依赖但不会发布。性能基准可运行 `go test -run='^$' -bench=. -benchmem . ./internal/diagnostic ./slog`。

修改 `README_CN.md` 时请同步更新 `README.md`，反之亦然。

<details>
<summary>本地联调：在自己的项目中使用未发布源码</summary>

仅运行仓库示例时，在本仓库根目录执行 `go run ./examples/basic` 即可。

在其他项目中联调时，将 `ERRKIND_DIR` 替换为本仓库的绝对路径。以下命令从新建空目录开始；已有 Go 项目跳过 `go mod init`。

```bash
ERRKIND_DIR=/absolute/path/to/errkind
go mod init example.com/errkind-demo
go mod edit -require=github.com/im-wmkong/errkind@v0.2.0
go mod edit "-replace=github.com/im-wmkong/errkind=$ERRKIND_DIR"
```

`http`、`slog` 随核心提供。第三方集成各自独立，使用时额外指向对应源码，例如 gRPC：

```bash
INTEGRATION=grpc
go mod edit "-require=github.com/im-wmkong/errkind/$INTEGRATION@v0.2.0"
go mod edit "-replace=github.com/im-wmkong/errkind/$INTEGRATION=$ERRKIND_DIR/$INTEGRATION"
```

可将 `grpc` 替换为 `zap`、`zerolog`、`logrus` 或 `otel`。添加代码和 import 后执行 `go mod tidy`，并保留核心的本地 `replace`；候选版本号仅配合本地源码使用，不表示远端已发布。

</details>

## 开源协议

[MIT License](LICENSE)。
