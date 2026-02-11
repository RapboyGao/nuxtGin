# nuxtGin

[![Go Version](https://img.shields.io/github/go-mod/go-version/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/RapboyGao/nuxtGin)](https://goreportcard.com/report/github.com/RapboyGao/nuxtGin)
[![Latest Release](https://img.shields.io/github/v/release/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/releases)
[![License](https://img.shields.io/github/license/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/blob/main/LICENSE)
[![Nuxt Gin Starter](https://img.shields.io/badge/starter-nuxt--gin--starter-2ea44f)](https://github.com/RapboyGao/nuxt-gin-starter)

🧩 A pragmatic Go toolkit that combines **Gin + Nuxt** and provides a **typed API layer** with **TypeScript client generation** for both HTTP and WebSocket.

Quick Jump:

- [中文说明（跳转）](#中文说明)

This package is primarily designed for and validated in:

- [nuxt-gin-starter](https://github.com/RapboyGao/nuxt-gin-starter)

## 🚀 Highlights

- 🛣️ Serve Nuxt in production (static files) and proxy Nuxt in development.
- 🧠 Strongly-typed HTTP endpoint definition in Go.
- 🔌 WebSocket endpoint abstraction with typed message handling.
- 🧾 TypeScript generation with field comments (`tsdoc`) and literal unions (`tsunion`).
- 🧱 Generated HTTP client now uses **per-endpoint classes** with static metadata.
- 🎨 Generated TypeScript is auto-formatted (Prettier if available).

## 📦 Install

```bash
go get github.com/RapboyGao/nuxtGin
```

## ⚙️ Config

Create `server.config.json` in your project root:

```json
{
  "ginPort": 8080,
  "nuxtPort": 3000,
  "baseUrl": "/"
}
```

## 🧭 Quick Start

```go
package main

import (
    "github.com/RapboyGao/nuxtGin"
    "github.com/RapboyGao/nuxtGin/endpoint"
)

func main() {
    endpoints := []endpoint.EndpointLike{}
    nuxtGin.MustRunServer(endpoints)
}
```

## 🧱 HTTP Endpoints + TS Client

### 1) Define typed endpoints in Go

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/RapboyGao/nuxtGin/endpoint"
)

type GetUserReq struct {
    ID     string `json:"id" tsdoc:"Unique user id / 用户唯一标识"`
    Level  string `json:"level" tsunion:"warning,success,error" tsdoc:"Message level / 消息等级"`
    Retry  int    `json:"retry" tsunion:"0,1,3" tsdoc:"Retry count / 重试次数"`
    Strict bool   `json:"strict" tsunion:"true,false" tsdoc:"Strict mode / 严格模式"`
}

type GetUserResp struct {
    Name string `json:"name" tsdoc:"Display name / 显示名称"`
}

func buildEndpoints() []endpoint.EndpointLike {
    return []endpoint.EndpointLike{
        endpoint.Endpoint[endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, GetUserReq, GetUserResp]{
            Name:   "GetUser",
            Method: endpoint.HTTPMethodPost,
            Path:   "/user/get",
            HandlerFunc: func(_ endpoint.NoParams, _ endpoint.NoParams, _ endpoint.NoParams, _ endpoint.NoParams, req GetUserReq, _ *gin.Context) (endpoint.Response[GetUserResp], error) {
                return endpoint.Response[GetUserResp]{StatusCode: 200, Body: GetUserResp{Name: "Alice"}}, nil
            },
        },
    }
}
```

### 2) Register + export TS in one call

```go
engine := gin.Default()
_, err := endpoint.ApplyEndpoints(engine, buildEndpoints())
if err != nil {
    panic(err)
}
```

Default output:

- Base path: `/api-go/v1`
- TS file: `vue/composables/auto-generated-api.ts`

## 🧰 Generated HTTP TS Style

Each endpoint generates one class (class name includes method), for example:

- `GetUserPost`

And includes static members/methods:

- `NAME`
- `SUMMARY`
- `METHOD`
- `PATH`
- `pathParamsShape()`
- `buildURL(...)`
- `requestConfig(...)`
- `request(...)`

Example shape:

```ts
export class GetUserPost {
  static readonly NAME = "getUser" as const;
  static readonly SUMMARY = "..." as const;
  static readonly METHOD = "POST" as const;
  static readonly PATH = "/api-go/v1/user/get" as const;

  static pathParamsShape() { ... }
  static buildURL(...) { ... }
  static requestConfig(...) { ... }
  static async request(...) { ... }
}
```

## 🔌 WebSocket Endpoints + TS Client

Use `WebSocketEndpoint` / `WebSocketAPI` to register WS routes and export TS client.

Default WS output:

- Base path: `/ws-go/v1`
- TS file: `vue/composables/auto-generated-ws.ts`

Generated WS TS includes:

- `TypedWebSocketClient<TReceive, TSend, TType>`
- `onType(...)` and `onTyped(...)`
- generated validators + `ensureXxx(...)`
- optional message-type union aliases when endpoint declares `MessageTypes`
- per-endpoint discriminated unions: `XxxReceiveUnion` / `XxxSendUnion`
- typed helpers: `onTypedMessage(...)` and `sendTypedMessage(...)`

### Recommended Envelope Shape

Keep one stable websocket envelope for all message kinds:

```go
type ChatEnvelope struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
```

Then:

- declare `MessageTypes` on endpoint
- register client payload mapping via `RegisterWebSocketTypedHandler(...)`
- register server payload mapping via `RegisterWebSocketServerPayloadType(...)`

Validation rule:

- if `MessageTypes` is set, every message type must exist in both client/server payload maps
- invalid mapping fails fast during build/export

### `TypedWebSocketClient` runtime members

Useful runtime members for UI state and diagnostics:

- `url`
- `status`: `'connecting' | 'open' | 'closing' | 'closed'`
- `readyState` (getter)
- `isOpen` (getter)
- `lastError`
- `lastClose`
- `connectedAt`
- `closedAt`
- `messagesSent`
- `messagesReceived`
- `reconnectCount`

These values are updated by built-in websocket lifecycle handlers (`open`, `close`, `error`, `message`, `send`, `close()`).

## 🏷️ `tsdoc` and `tsunion`

### `tsdoc`

Use on struct fields to generate TSDoc comments.

```go
Name string `json:"name" tsdoc:"Display name / 显示名称"`
```

### `tsunion`

Use on fields to generate TS literal unions + runtime validator checks.

Supported Go field kinds:

- `string`
- `bool`
- `int/int8/int16/int32`
- `uint/uint8/uint16/uint32`
- `float32/float64`

Examples:

```go
Level  string `json:"level" tsunion:"warning,success,error"`
Retry  int    `json:"retry" tsunion:"0,1,3"`
Strict bool   `json:"strict" tsunion:"true,false"`
```

## 🎨 TS Formatting Behavior

Generated TS is finalized with best-effort formatting:

1. try `prettier --parser typescript`
2. fallback to `npx prettier --parser typescript`
3. if both unavailable, keep raw generated output

This never blocks generation.

## 🗂️ Project Layout

```text
runtime/                 # server runtime (config, mode, vue serving, bootstrap)
runtime_compat.go        # compatibility exports
endpoint/                # HTTP/WS endpoint layer + TS generators
utils/                   # utility helpers
README.md
README.zh-CN.md
```

## 🔎 Notes

- Dev mode is inferred when `node_modules` exists in the project root.
- If you need fully custom Gin handler behavior, use `CustomEndpoint`.
- Recommended starter project: [Nuxt Gin Starter](https://github.com/RapboyGao/nuxt-gin-starter)

## 中文说明

🧩 `nuxtGin` 是一个务实的 Go 工具包，结合 **Gin + Nuxt**，并提供 **强类型 API 层** 与 **TypeScript 客户端自动生成**（HTTP + WebSocket）。

本包主要面向并在以下项目中验证：

- [nuxt-gin-starter](https://github.com/RapboyGao/nuxt-gin-starter)

### 🚀 亮点

- 🛣️ 生产环境可直接托管 Nuxt 静态文件，开发环境可反向代理 Nuxt 服务。
- 🧠 在 Go 侧定义强类型 HTTP Endpoint。
- 🔌 提供 WebSocket Endpoint 抽象，支持按消息类型处理。
- 🧾 支持通过 `tsdoc`/`tsunion` 生成更可读、更强约束的 TS 类型。
- 🧱 HTTP 客户端按“每个 API 一个 class”生成，带静态元数据。
- 🎨 生成的 TS 支持自动格式化（可用时走 Prettier）。

### 📦 安装

```bash
go get github.com/RapboyGao/nuxtGin
```

### ⚙️ 配置

在项目根目录创建 `server.config.json`：

```json
{
  "ginPort": 8080,
  "nuxtPort": 3000,
  "baseUrl": "/"
}
```

### 🧭 快速开始

```go
package main

import (
    "github.com/RapboyGao/nuxtGin"
    "github.com/RapboyGao/nuxtGin/endpoint"
)

func main() {
    endpoints := []endpoint.EndpointLike{}
    nuxtGin.MustRunServer(endpoints)
}
```

### 🧱 HTTP Endpoints + TS 客户端

#### 1) 在 Go 中定义强类型 Endpoint

```go
type GetUserReq struct {
    ID     string `json:"id" tsdoc:"Unique user id / 用户唯一标识"`
    Level  string `json:"level" tsunion:"warning,success,error" tsdoc:"Message level / 消息等级"`
    Retry  int    `json:"retry" tsunion:"0,1,3" tsdoc:"Retry count / 重试次数"`
    Strict bool   `json:"strict" tsunion:"true,false" tsdoc:"Strict mode / 严格模式"`
}

type GetUserResp struct {
    Name string `json:"name" tsdoc:"Display name / 显示名称"`
}
```

#### 2) 一次完成注册与导出

```go
engine := gin.Default()
_, err := endpoint.ApplyEndpoints(engine, buildEndpoints())
if err != nil {
    panic(err)
}
```

默认输出：

- Base path: `/api-go/v1`
- TS 文件：`vue/composables/auto-generated-api.ts`

#### HTTP 生成风格

每个 API 会生成一个 class（类名包含 Method），并提供：

- `NAME`
- `SUMMARY`
- `METHOD`
- `PATHS`（`base/group/api`）
- `FULL_PATH`
- `pathParamsShape()`
- `buildURL(...)`
- `requestConfig(...)`
- `request(...)`

### 🔌 WebSocket Endpoints + TS 客户端

使用 `WebSocketEndpoint` / `WebSocketAPI` 注册 WS 路由并导出 TS。

默认输出：

- Base path: `/ws-go/v1`
- TS 文件：`vue/composables/auto-generated-ws.ts`

生成内容包括：

- `TypedWebSocketClient<TReceive, TSend, TType>`
- `onType(...)` 与 `onTyped(...)`
- 自动生成的 `validator + ensure`
- `MessageTypes` 对应的字面量联合类型
- 每个 endpoint 的 `XxxReceiveUnion` / `XxxSendUnion`
- 每个 endpoint 的 `onTypedMessage(...)` / `sendTypedMessage(...)`

#### 推荐 Envelope 结构

```go
type ChatEnvelope struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
```

推荐搭配：

- 在 endpoint 声明 `MessageTypes`
- 用 `RegisterWebSocketTypedHandler(...)` 注册客户端 payload 类型
- 用 `RegisterWebSocketServerPayloadType(...)` 注册服务端 payload 类型

校验规则：

- 只要设置了 `MessageTypes`，每个 message type 必须同时存在 client/server payload 映射
- 映射不完整会在 build/export 阶段直接报错（fail fast）

### 🏷️ `tsdoc` 与 `tsunion`

#### `tsdoc`

为 struct 字段生成 TSDoc：

```go
Name string `json:"name" tsdoc:"Display name / 显示名称"`
```

#### `tsunion`

生成 TS 字面量联合类型，并在 validator 中加入运行时检查。支持：

- `string`
- `bool`
- `int/int8/int16/int32`
- `uint/uint8/uint16/uint32`
- `float32/float64`

示例：

```go
Level  string `json:"level" tsunion:"warning,success,error"`
Retry  int    `json:"retry" tsunion:"0,1,3"`
Strict bool   `json:"strict" tsunion:"true,false"`
```

### 🎨 TS 格式化

生成 TS 时按以下顺序尝试：

1. `prettier --parser typescript`
2. `npx prettier --parser typescript`
3. 均不可用时保留原始生成内容

该流程不会阻塞生成。

### 🗂️ 项目结构

```text
runtime/                 # server runtime (config, mode, vue serving, bootstrap)
runtime_compat.go        # compatibility exports
endpoint/                # HTTP/WS endpoint layer + TS generators
utils/                   # utility helpers
README.md
README.zh-CN.md
```

### 🔎 说明

- 项目根目录存在 `node_modules` 时会判定为开发模式。
- 如需完全自定义 Gin handler，可使用 `CustomEndpoint`。
- 推荐 Starter 项目：[Nuxt Gin Starter](https://github.com/RapboyGao/nuxt-gin-starter)

## 📄 License

MIT
