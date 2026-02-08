# nuxtGin

一个 Go 模块，用于将 Gin 与 Nuxt 结合：生产环境提供静态文件，开发环境反向代理 Nuxt，同时提供类型化 HTTP Endpoint 与 TS axios 客户端生成。

## 你能得到什么

- Gin 运行模式自动判断（开发/生产）
- Nuxt 服务（静态/代理）
- 类型化 HTTP Endpoint + TS 客户端生成
- 常用工具函数

## 安装

```bash
go get github.com/RapboyGao/nuxtGin
```

## 快速开始

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

## 配置

在项目根目录创建 `server.config.json`：

```json
{
  "ginPort": 8080,
  "nuxtPort": 3000,
  "baseUrl": "/"
}
```

## HTTP Endpoint + TS 客户端

定义 Endpoint，并在 `ApplyEndpoints` 时生成 TS：

```go
api := []endpoint.EndpointLike{
    endpoint.Endpoint[endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoBody, struct{ Ok bool }]{
        Name:   "Ping",
        Method: endpoint.HTTPMethodGet,
        Path:   "/ping",
        HandlerFunc: func(_ endpoint.NoParams, _ endpoint.NoParams, _ endpoint.NoParams, _ endpoint.NoParams, _ endpoint.NoBody, _ *gin.Context) (endpoint.Response[struct{ Ok bool }], error) {
            return endpoint.Response[struct{ Ok bool }]{Body: struct{ Ok bool }{Ok: true}}, nil
        },
    },
}

engine := gin.Default()
endpoint.ApplyEndpoints(engine, api)
```

### 在生成 TS 字段上加注释（`tsdoc`）

`Go` 源码里的普通注释（`// ...`）运行时反射拿不到，请使用 struct tag：

```go
type User struct {
    ID   string `json:"id" tsdoc:"唯一用户ID / Unique user id"`
    Name string `json:"name" tsdoc:"显示名称 / Display name"`
}
```

生成的 TypeScript interface 会带字段注释，例如：

```ts
/** 唯一用户ID / Unique user id */
id: string;
```

需要完全掌控 Gin 行为时，用 `CustomEndpoint`：

```go
endpoint.CustomEndpoint[endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoBody, endpoint.NoBody]{
    Name:        "Raw",
    Method:      endpoint.HTTPMethodGet,
    Path:        "/raw",
    HandlerFunc: func(ctx *gin.Context) { ctx.String(200, "ok") },
}
```

## 项目结构

```text
ServeVue.go            # Nuxt 服务（静态/代理）
getConfig.go           # server.config.json 读取
getGinMode.go          # 开发/生产判断
Server.go              # CreateServer/RunServer 封装
endpoint/              # Endpoint + TS 生成
utils/                 # 工具函数
```

## 说明

- 根目录存在 `node_modules` 时使用开发模式。
- TS 输出默认路径：`vue/composables/auto-generated-api.ts`。
- 模板项目：[Nuxt Gin Starter](https://github.com/RapboyGao/nuxt-gin-starter)

## 许可证

MIT
