# nuxtGin

A Go module that pairs Gin with Nuxt (serve static in production, reverse proxy in development) and provides a typed HTTP endpoint layer with TypeScript client generation.

## What You Get

- Gin mode auto-detection (dev/prod)
- Vue/Nuxt serving helper (static or proxy)
- Typed HTTP endpoints with TS axios client generation
- Utilities for common server tasks

## Install

```bash
go get github.com/RapboyGao/nuxtGin
```

## Quick Start

```go
package main

import (
    "github.com/RapboyGao/nuxtGin"
    "github.com/RapboyGao/nuxtGin/endpoint"
)

func main() {
    // Create and run server with your endpoints
    endpoints := []endpoint.EndpointLike{}
    nuxtGin.MustRunServer(endpoints)
}
```

## Configuration

Create `server.config.json` in your project root:

```json
{
  "ginPort": 8080,
  "nuxtPort": 3000,
  "baseUrl": "/"
}
```

## HTTP Endpoints + TS Client

Define endpoints and generate TS automatically when you call `ApplyEndpoints`:

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

### Add Field Comments To Generated TS (`tsdoc`)

`Go` source comments (`// ...`) are not available via reflection at runtime, so use struct tags:

```go
type User struct {
    ID   string `json:"id" tsdoc:"Unique user id / 用户唯一标识"`
    Name string `json:"name" tsdoc:"Display name / 显示名称"`
}
```

The generated TypeScript interface will include field comments, for example:

```ts
/** Unique user id / 用户唯一标识 */
id: string;
```

If you need full control of Gin behavior, use `CustomEndpoint`:

```go
endpoint.CustomEndpoint[endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoParams, endpoint.NoBody, endpoint.NoBody]{
    Name:        "Raw",
    Method:      endpoint.HTTPMethodGet,
    Path:        "/raw",
    HandlerFunc: func(ctx *gin.Context) { ctx.String(200, "ok") },
}
```

## Project Layout

```text
serve_vue.go           # Nuxt serving (static/proxy)
config.go              # server.config.json loader
gin_mode.go            # dev/prod detection
server.go              # CreateServer/RunServer helpers
endpoint/              # Endpoint + TS generator
utils/                 # Utility helpers
```

## Notes

- Dev mode is used when `node_modules` exists in the project root.
- TS output defaults to `vue/composables/auto-generated-api.ts`.
- Starter template: [Nuxt Gin Starter](https://github.com/RapboyGao/nuxt-gin-starter)

## License

MIT
