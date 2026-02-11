package endpoint

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
)

// ServerAPI is the single struct you use to describe server-side APIs.
// It can build a gin.RouterGroup and export axios TypeScript.
// ServerAPI 是你用于描述服务器端 API 的唯一结构体；
// 可构建 gin.RouterGroup，并生成 axios TypeScript。
type ServerAPI struct {
	// BasePath is the URL prefix used for generated TS clients.
	// BasePath 用于生成 TS 客户端时的 URL 前缀。
	BasePath string

	// GroupPath is the router-group path used when registering handlers in gin.
	// GroupPath 是在 gin 中注册路由时使用的分组路径。
	GroupPath string

	// Endpoints contains all HTTP endpoints under this API group.
	// Endpoints 包含该 API 分组下的全部 HTTP 端点。
	Endpoints []EndpointLike
}

// BuildGinGroup registers all endpoints and returns the RouterGroup.
// BuildGinGroup 会注册所有端点并返回 RouterGroup。
func (s ServerAPI) BuildGinGroup(engine *gin.Engine) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	groupPath := resolveAPIPath(s.BasePath, s.GroupPath)
	if strings.TrimSpace(groupPath) == "" {
		return nil, errors.New("base path or group path is required")
	}
	group := engine.Group(groupPath)
	if err := registerEndpointHandlers(group, s.Endpoints); err != nil {
		return nil, err
	}
	return group, nil
}

// ExportTS generates axios TypeScript to a relative path.
// If relativeTSPath is empty, it defaults to vue/composables/my-schemas.ts.
// ExportTS 会生成 axios TypeScript 到相对路径；
// 若 relativeTSPath 为空，则默认 vue/composables/my-schemas.ts。
func (s ServerAPI) ExportTS(relativeTSPath string) error {
	if !shouldExportTSInCurrentEnv() {
		return nil
	}
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/my-schemas.ts"
	}
	return exportAxiosFromEndpointsToTSFile(s.BasePath, s.GroupPath, s.Endpoints, relativeTSPath)
}

// Build builds gin.RouterGroup and exports TS in one call.
// Build 一次性完成 RouterGroup 构建与 TS 导出。
func (s ServerAPI) Build(engine *gin.Engine, relativeTSPath string) (*gin.RouterGroup, error) {
	group, err := s.BuildGinGroup(engine)
	if err != nil {
		return nil, err
	}
	if err := s.ExportTS(relativeTSPath); err != nil {
		return nil, err
	}
	return group, nil
}

// GenerateAxiosFromEndpoints generates TypeScript axios client source code from endpoints.
// GenerateAxiosFromEndpoints 根据 Endpoint 列表生成 TypeScript axios 客户端代码。
func GenerateAxiosFromEndpoints(basePath string, endpoints []EndpointLike) (string, error) {
	return generateAxiosFromEndpoints(basePath, "", endpoints)
}

// ExportAxiosFromEndpointsToTSFile writes generated TS code from endpoints to a file.
// ExportAxiosFromEndpointsToTSFile 将 Endpoint 生成的 TS 代码写入文件。
func ExportAxiosFromEndpointsToTSFile(basePath string, endpoints []EndpointLike, relativeTSPath string) error {
	return exportAxiosFromEndpointsToTSFile(basePath, "", endpoints, relativeTSPath)
}

// ApplyEndpoints registers endpoints to gin.Engine and exports TS in one call.
// Defaults: basePath="/api-go/v1", tsPath="vue/composables/auto-generated-api.ts".
// ApplyEndpoints 一次性完成 gin 注册与 TS 导出。
// 默认 basePath 为 /api-go/v1，TS 输出路径为 vue/composables/auto-generated-api.ts。
func ApplyEndpoints(engine *gin.Engine, endpoints []EndpointLike) (*gin.RouterGroup, error) {
	basePath := "/api-go/v1"
	relativeTSPath := "vue/composables/auto-generated-api.ts"
	api := ServerAPI{
		BasePath:  basePath,
		GroupPath: basePath,
		Endpoints: endpoints,
	}
	return api.Build(engine, relativeTSPath)
}

// ApplyEndpointsDevOnly registers endpoints in all modes, but only exports TS in gin.DebugMode.
// Defaults: basePath="/api-go/v1", tsPath="vue/composables/auto-generated-api.ts".
// ApplyEndpointsDevOnly 会在所有模式下注册路由，但仅在 gin.DebugMode 下生成 TS。
// 默认 basePath 为 /api-go/v1，TS 输出路径为 vue/composables/auto-generated-api.ts。
func ApplyEndpointsDevOnly(engine *gin.Engine, endpoints []EndpointLike) (*gin.RouterGroup, error) {
	basePath := "/api-go/v1"
	relativeTSPath := "vue/composables/auto-generated-api.ts"
	api := ServerAPI{
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

// WebSocketAPI describes websocket endpoints, supports gin registration and TS export.
// WebSocketAPI 描述 websocket 端点，可构建 gin.RouterGroup，并生成 TS。
type WebSocketAPI struct {
	// BasePath is the URL prefix used for generated TS websocket clients.
	// BasePath 用于生成 TS websocket 客户端时的 URL 前缀。
	BasePath string

	// GroupPath is the router-group path used when registering websocket handlers in gin.
	// GroupPath 是在 gin 中注册 websocket 路由时使用的分组路径。
	GroupPath string

	// Endpoints contains all websocket endpoints under this API group.
	// Endpoints 包含该 API 分组下的全部 websocket 端点。
	Endpoints []WebSocketEndpointLike

	// DefaultClientMessageType is the default envelope type for endpoint.ClientMessageType.
	// DefaultClientMessageType 作为 endpoint.ClientMessageType 的默认封装类型。
	DefaultClientMessageType reflect.Type

	// DefaultServerMessageType is the default envelope type for endpoint.ServerMessageType.
	// DefaultServerMessageType 作为 endpoint.ServerMessageType 的默认封装类型。
	DefaultServerMessageType reflect.Type
}

// BuildGinGroup registers all websocket endpoints and returns the RouterGroup.
// BuildGinGroup 注册所有 websocket 端点并返回 RouterGroup。
func (s WebSocketAPI) BuildGinGroup(engine *gin.Engine) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	s.applyDefaults()
	groupPath := resolveAPIPath(s.BasePath, s.GroupPath)
	if strings.TrimSpace(groupPath) == "" {
		return nil, errors.New("base path or group path is required")
	}
	group := engine.Group(groupPath)
	if err := registerWebSocketHandlers(group, groupPath, s.Endpoints); err != nil {
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
	s.applyDefaults()
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/auto-generated-ws.ts"
	}
	return exportWebSocketClientFromEndpointsToTSFile(s.BasePath, s.GroupPath, s.Endpoints, relativeTSPath)
}

// Build builds gin.RouterGroup and exports TS in one call.
// Build 一次性完成 RouterGroup 构建与 TS 导出。
func (s WebSocketAPI) Build(engine *gin.Engine, relativeTSPath string) (*gin.RouterGroup, error) {
	s.applyDefaults()
	group, err := s.BuildGinGroup(engine)
	if err != nil {
		return nil, err
	}
	if err := s.ExportTS(relativeTSPath); err != nil {
		return nil, err
	}
	return group, nil
}

func (s WebSocketAPI) applyDefaults() {
	for i := range s.Endpoints {
		ws, ok := s.Endpoints[i].(*WebSocketEndpoint)
		if !ok || ws == nil {
			continue
		}
		if ws.ClientMessageType == nil || ws.ClientMessageType.Kind() == reflect.Invalid {
			ws.ClientMessageType = s.DefaultClientMessageType
		}
		if ws.ServerMessageType == nil || ws.ServerMessageType.Kind() == reflect.Invalid {
			ws.ServerMessageType = s.DefaultServerMessageType
		}
	}
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
		if err := validateWebSocketPayloadTypeMappings(meta); err != nil {
			return fmt.Errorf("register websocket endpoint[%d] failed: %w", i, err)
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

func resolveAPIPath(basePath string, groupPath string) string {
	base := normalizePathSegment(basePath)
	group := normalizePathSegment(groupPath)

	if base == "" {
		return group
	}
	if group == "" {
		return base
	}
	if group == base || strings.HasPrefix(group, base+"/") {
		return group
	}
	if base == group || strings.HasPrefix(base, group+"/") {
		return base
	}
	return base + "/" + strings.TrimLeft(group, "/")
}

func normalizePathSegment(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return ""
	}
	p = "/" + strings.Trim(p, "/")
	if p == "/" {
		return ""
	}
	return p
}
