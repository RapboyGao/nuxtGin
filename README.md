# nuxtGin

[![Go Version](https://img.shields.io/github/go-mod/go-version/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/blob/main/go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/RapboyGao/nuxtGin)](https://goreportcard.com/report/github.com/RapboyGao/nuxtGin)
[![Latest Release](https://img.shields.io/github/v/release/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/releases)
[![License](https://img.shields.io/github/license/RapboyGao/nuxtGin)](https://github.com/RapboyGao/nuxtGin/blob/main/LICENSE)
[![Nuxt Gin Starter](https://img.shields.io/badge/starter-nuxt--gin--starter-2ea44f)](https://github.com/RapboyGao/nuxt-gin-starter)

🧩 A pragmatic Go toolkit that combines **Gin + Nuxt** and provides a **typed API layer** with **TypeScript client generation** for both HTTP and WebSocket.

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

### Recommended Envelope Shape

Keep one stable websocket envelope for all message kinds:

```go
type ChatEnvelope struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload"`
}
```

Then dispatch by `Type` and decode `Payload` per message type via typed handlers.

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

## 📄 License

MIT
