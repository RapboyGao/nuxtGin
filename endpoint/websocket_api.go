package endpoint

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// WebSocketAPI describes websocket endpoints, supports gin registration and TS export.
// WebSocketAPI 描述 websocket 端点，可构建 gin.RouterGroup，并生成 TS。
type WebSocketAPI struct {
	BasePath  string
	GroupPath string
	Endpoints []WebSocketEndpointLike
}

// BuildGinGroup registers all websocket endpoints and returns the RouterGroup.
// BuildGinGroup 注册所有 websocket 端点并返回 RouterGroup。
func (s WebSocketAPI) BuildGinGroup(engine *gin.Engine) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	if strings.TrimSpace(s.GroupPath) == "" {
		return nil, errors.New("group path is required")
	}
	group := engine.Group(s.GroupPath)
	if err := registerWebSocketHandlers(group, s.GroupPath, s.Endpoints); err != nil {
		return nil, err
	}
	return group, nil
}

// ExportTS generates websocket TypeScript to a relative path.
// ExportTS 会生成 websocket TypeScript 到相对路径。
func (s WebSocketAPI) ExportTS(relativeTSPath string) error {
	if !shouldExportTSInCurrentEnv() {
		return nil
	}
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/auto-generated-ws.ts"
	}
	base := strings.TrimSpace(s.BasePath)
	if base == "" {
		base = s.GroupPath
	}
	return ExportWebSocketClientFromEndpointsToTSFile(base, s.Endpoints, relativeTSPath)
}

// Build builds gin.RouterGroup and exports TS in one call.
// Build 一次性完成 RouterGroup 构建与 TS 导出。
func (s WebSocketAPI) Build(engine *gin.Engine, relativeTSPath string) (*gin.RouterGroup, error) {
	group, err := s.BuildGinGroup(engine)
	if err != nil {
		return nil, err
	}
	if err := s.ExportTS(relativeTSPath); err != nil {
		return nil, err
	}
	return group, nil
}

// ApplyWebSocketEndpoints registers endpoints to gin.Engine and exports TS in one call.
// Defaults: basePath="/ws-go/v1", tsPath="vue/composables/auto-generated-ws.ts".
// ApplyWebSocketEndpoints 一次性完成 gin 注册与 TS 导出。
// 默认 basePath 为 /ws-go/v1，TS 输出路径为 vue/composables/auto-generated-ws.ts。
func ApplyWebSocketEndpoints(engine *gin.Engine, endpoints []WebSocketEndpointLike) (*gin.RouterGroup, error) {
	basePath := "/ws-go/v1"
	relativeTSPath := "vue/composables/auto-generated-ws.ts"
	api := WebSocketAPI{
		BasePath:  basePath,
		GroupPath: basePath,
		Endpoints: endpoints,
	}
	return api.Build(engine, relativeTSPath)
}

// ApplyWebSocketEndpointsDevOnly registers endpoints in all modes, but only exports TS in gin.DebugMode.
// Defaults: basePath="/ws-go/v1", tsPath="vue/composables/auto-generated-ws.ts".
// ApplyWebSocketEndpointsDevOnly 会在所有模式下注册路由，但仅在 gin.DebugMode 下生成 TS。
// 默认 basePath 为 /ws-go/v1，TS 输出路径为 vue/composables/auto-generated-ws.ts。
func ApplyWebSocketEndpointsDevOnly(engine *gin.Engine, endpoints []WebSocketEndpointLike) (*gin.RouterGroup, error) {
	basePath := "/ws-go/v1"
	relativeTSPath := "vue/composables/auto-generated-ws.ts"
	api := WebSocketAPI{
		BasePath:  basePath,
		GroupPath: basePath,
		Endpoints: endpoints,
	}
	group, err := api.BuildGinGroup(engine)
	if err != nil {
		return nil, err
	}
	if gin.Mode() == gin.DebugMode {
		if err := api.ExportTS(relativeTSPath); err != nil {
			return nil, err
		}
	}
	return group, nil
}

func registerWebSocketHandlers(router gin.IRouter, groupPath string, endpoints []WebSocketEndpointLike) error {
	for i := range endpoints {
		meta := endpoints[i].WebSocketMeta()
		if strings.TrimSpace(meta.Path) == "" {
			return fmt.Errorf("register websocket endpoint[%d] failed: path is required", i)
		}
		fullPath := joinWSPath(groupPath, meta.Path)
		endpoints[i].SetFullPath(fullPath)
		router.GET(meta.Path, endpoints[i].GinHandler())
	}
	return nil
}

func joinWSPath(baseURL string, path string) string {
	base := strings.TrimSpace(baseURL)
	p := strings.TrimSpace(path)

	if base == "" {
		if strings.HasPrefix(p, "/") {
			return p
		}
		return "/" + p
	}
	if p == "" {
		if strings.HasPrefix(base, "/") {
			return strings.TrimRight(base, "/")
		}
		return "/" + strings.TrimRight(base, "/")
	}

	base = strings.TrimRight(base, "/")
	p = strings.TrimLeft(p, "/")
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return base + "/" + p
}
