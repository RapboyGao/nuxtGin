package endpoint

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
)

// ServerAPI is the single struct you use to describe server-side APIs.
// It can build a gin.RouterGroup and export axios TypeScript.
// ServerAPI 是你用于描述服务器端 API 的唯一结构体；
// 可构建 gin.RouterGroup，并生成 axios TypeScript。
type ServerAPI struct {
	BasePath  string
	GroupPath string
	Endpoints []EndpointLike
}

// BuildGinGroup registers all endpoints and returns the RouterGroup.
// BuildGinGroup 会注册所有端点并返回 RouterGroup。
func (s ServerAPI) BuildGinGroup(engine *gin.Engine) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	if strings.TrimSpace(s.GroupPath) == "" {
		return nil, errors.New("group path is required")
	}
	group := engine.Group(s.GroupPath)
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
	base := strings.TrimSpace(s.BasePath)
	if base == "" {
		base = s.GroupPath
	}
	return ExportAxiosFromEndpointsToTSFile(base, s.Endpoints, relativeTSPath)
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
	return generateAxiosFromEndpoints(basePath, endpoints)
}

// ExportAxiosFromEndpointsToTSFile writes generated TS code from endpoints to a file.
// ExportAxiosFromEndpointsToTSFile 将 Endpoint 生成的 TS 代码写入文件。
func ExportAxiosFromEndpointsToTSFile(basePath string, endpoints []EndpointLike, relativeTSPath string) error {
	return exportAxiosFromEndpointsToTSFile(basePath, endpoints, relativeTSPath)
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
