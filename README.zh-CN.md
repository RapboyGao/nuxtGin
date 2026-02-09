# nuxtGin

🧩 一个实用的 Go 工具库：组合 **Gin + Nuxt**，并提供 **类型化 API 定义** 与 **TypeScript 客户端生成**（HTTP + WebSocket）。

## 🚀 核心能力

- 🛣️ Nuxt 服务：生产环境静态托管，开发环境反向代理。
- 🧠 Go 端强类型 HTTP Endpoint 定义。
- 🔌 WebSocket Endpoint 抽象与类型化消息处理。
- 🧾 TS 生成支持字段注释（`tsdoc`）与字面量联合（`tsunion`）。
- 🧱 HTTP 生成结果为**每个接口一个 class**，带静态元信息。
- 🎨 生成后自动排版（可用时走 Prettier）。

## 📦 安装

```bash
go get github.com/RapboyGao/nuxtGin
```

## ⚙️ 配置

在项目根目录创建 `server.config.json`：

```json
{
  "ginPort": 8080,
  "nuxtPort": 3000,
  "baseUrl": "/"
}
```

## 🧭 快速开始

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

## 🧱 HTTP Endpoint + TS 客户端

### 1）在 Go 里定义类型化接口

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/RapboyGao/nuxtGin/endpoint"
)

type GetUserReq struct {
    ID     string `json:"id" tsdoc:"唯一用户ID / Unique user id"`
    Level  string `json:"level" tsunion:"warning,success,error" tsdoc:"消息等级 / Message level"`
    Retry  int    `json:"retry" tsunion:"0,1,3" tsdoc:"重试次数 / Retry count"`
    Strict bool   `json:"strict" tsunion:"true,false" tsdoc:"严格模式 / Strict mode"`
}

type GetUserResp struct {
    Name string `json:"name" tsdoc:"显示名称 / Display name"`
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

### 2）一行完成路由注册 + TS 导出

```go
engine := gin.Default()
_, err := endpoint.ApplyEndpoints(engine, buildEndpoints())
if err != nil {
    panic(err)
}
```

默认值：

- Base path：`/api-go/v1`
- TS 输出：`vue/composables/auto-generated-api.ts`

## 🧰 HTTP 生成结果（Class 风格）

每个 endpoint 会生成一个 class（类名带 Method），例如：

- `GetUserPost`

类内含以下静态成员/方法：

- `NAME`
- `SUMMARY`
- `METHOD`
- `PATH`
- `pathParamsShape()`
- `buildURL(...)`
- `requestConfig(...)`
- `request(...)`

示例结构：

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

## 🔌 WebSocket Endpoint + TS 客户端

使用 `WebSocketEndpoint` / `WebSocketAPI` 注册 WS 路由并导出 TS 客户端。

默认值：

- Base path：`/ws-go/v1`
- TS 输出：`vue/composables/auto-generated-ws.ts`

WS 生成结果包含：

- `TypedWebSocketClient<TReceive, TSend, TType>`
- `onType(...)` 与 `onTyped(...)`
- 自动生成 validator 与 `ensureXxx(...)`
- 若声明 `MessageTypes`，会生成消息类型联合别名

### `TypedWebSocketClient` 运行时成员

可直接用于前端状态展示与排障：

- `url`
- `status`：`'connecting' | 'open' | 'closing' | 'closed'`
- `readyState`（getter）
- `isOpen`（getter）
- `lastError`
- `lastClose`
- `connectedAt`
- `closedAt`
- `messagesSent`
- `messagesReceived`
- `reconnectCount`

这些值会由内置生命周期处理自动更新（`open`、`close`、`error`、`message`、`send`、`close()`）。

## 🏷️ `tsdoc` 与 `tsunion`

### `tsdoc`

用于给生成的 TS 字段写注释：

```go
Name string `json:"name" tsdoc:"显示名称 / Display name"`
```

### `tsunion`

用于生成 TS 字面量联合 + 运行时校验。

支持的 Go 字段类型：

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

## 🎨 TS 排版策略

生成完成后会尽力格式化：

1. 先尝试 `prettier --parser typescript`
2. 若失败，回退到 `npx prettier --parser typescript`
3. 都不可用则保留原始生成文本

格式化失败不会阻塞生成。

## 🗂️ 项目结构

```text
serve_vue.go             # Nuxt 服务（静态/代理）
config.go                # server.config.json 读取
gin_mode.go              # 开发/生产模式判断
server.go                # 服务启动封装
endpoint/                # HTTP/WS Endpoint 与 TS 生成器
utils/                   # 工具函数
README.md
README.zh-CN.md
```

## 🔎 说明

- 根目录存在 `node_modules` 时会进入开发模式。
- 若需要完全自定义 Gin 行为，可使用 `CustomEndpoint`。
- 模板项目：[Nuxt Gin Starter](https://github.com/RapboyGao/nuxt-gin-starter)

## 📄 许可证

MIT
