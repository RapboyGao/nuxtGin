package runtime

import (
	"fmt"
	"strings"

	"github.com/RapboyGao/nuxtGin/endpoint"
	"github.com/RapboyGao/nuxtGin/internal/runtimeutil"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// APIServerConfig configures server runtime, API registration, and TS generation outputs.
// APIServerConfig 用于统一配置服务端口、API 注册和 TS 输出路径。
type APIServerConfig struct {
	// Server contains base URL and ports used by the runtime.
	// Server 包含运行时使用的基础 URL 与端口配置。
	Server ServerRuntimeConfig

	// GinMode controls debug/release mode. Empty means auto-resolve.
	// GinMode 控制 debug/release 模式；为空表示自动解析。
	GinMode string

	// CORS is the actual gin-contrib/cors config.
	// CORS 为 gin-contrib/cors 的实际配置；为 nil 表示不启用 CORS。
	CORS *cors.Config

	// API definitions (already include GroupPath/BasePath inside each API struct).
	// API 定义（各自结构体内已包含 GroupPath/BasePath）。
	ServerAPI    endpoint.ServerAPI
	WebSocketAPI endpoint.WebSocketAPI

	// Three TS output paths.
	// 三个 TS 输出路径。
	ServerTSPath    string
	WebSocketTSPath string
	SchemaTSPath    string

	// ExportUnifiedTS controls whether to export into three files via shared schema mode.
	// ExportUnifiedTS 控制是否使用共享 schema 的三文件统一导出。
	ExportUnifiedTS bool

	// ExportTSOnRun controls whether RunServerFromConfig exports TS before starting.
	// ExportTSOnRun 控制 RunServerFromConfig 是否在启动前导出 TS。
	ExportTSOnRun bool
}

// DefaultAPIServerConfig returns a fully initialized default config with endpoints.
// DefaultAPIServerConfig 返回一份可直接使用的默认配置，并注入 Endpoints。
func DefaultAPIServerConfig(
	endpoints []endpoint.EndpointLike,
	wsEndpoints []endpoint.WebSocketEndpointLike,
) APIServerConfig {
	config := APIServerConfig{
		ServerAPI: endpoint.ServerAPI{
			BasePath:  "/api-go",
			GroupPath: "/v1",
			Endpoints: endpoints,
		},
		WebSocketAPI: endpoint.WebSocketAPI{
			BasePath:  "/ws-go",
			GroupPath: "/v1",
			Endpoints: wsEndpoints,
		},
		ServerTSPath:    "vue/composables/auto-generated-api.ts",
		WebSocketTSPath: "vue/composables/auto-generated-ws.ts",
		SchemaTSPath:    "vue/composables/auto-generated-types.ts",
		ExportUnifiedTS: true,
		ExportTSOnRun:   true,
	}
	return config
}

func resolveServerRuntimeConfig(config ServerRuntimeConfig) (ServerRuntimeConfig, error) {
	if err := config.Validate(); err == nil {
		SetActiveConfig(config)
		return config, nil
	}

	fileConfig, err := LoadConfig()
	if err != nil {
		return ServerRuntimeConfig{}, fmt.Errorf("server runtime config is incomplete and server.config.json could not be loaded: %w", err)
	}

	resolved := config
	if resolved.GinPort <= 0 {
		resolved.GinPort = fileConfig.GinPort
	}
	if resolved.NuxtPort <= 0 {
		resolved.NuxtPort = fileConfig.NuxtPort
	}
	if strings.TrimSpace(resolved.BaseUrl) == "" {
		resolved.BaseUrl = fileConfig.BaseUrl
	}
	if err := resolved.Validate(); err != nil {
		return ServerRuntimeConfig{}, err
	}
	SetActiveConfig(resolved)
	return resolved, nil
}

func (c APIServerConfig) normalized() (APIServerConfig, error) {
	out := c
	resolvedServer, err := resolveServerRuntimeConfig(out.Server)
	if err != nil {
		return APIServerConfig{}, err
	}
	out.Server = resolvedServer
	if strings.TrimSpace(out.ServerTSPath) == "" {
		out.ServerTSPath = "vue/composables/auto-generated-api.ts"
	}
	if strings.TrimSpace(out.WebSocketTSPath) == "" {
		out.WebSocketTSPath = "vue/composables/auto-generated-ws.ts"
	}
	if strings.TrimSpace(out.SchemaTSPath) == "" {
		out.SchemaTSPath = "vue/composables/auto-generated-shared.ts"
	}

	if strings.TrimSpace(out.ServerAPI.BasePath) == "" && strings.TrimSpace(out.ServerAPI.GroupPath) == "" {
		out.ServerAPI.BasePath = "/api-go"
		out.ServerAPI.GroupPath = "/v1"
	}
	if strings.TrimSpace(out.WebSocketAPI.BasePath) == "" && strings.TrimSpace(out.WebSocketAPI.GroupPath) == "" {
		out.WebSocketAPI.BasePath = "/ws-go"
		out.WebSocketAPI.GroupPath = "/v1"
	}
	if out.CORS == nil && ResolveGinMode(out.GinMode) == gin.DebugMode {
		defaultCorsConfig := cors.DefaultConfig()
		defaultCorsConfig.AllowAllOrigins = true
		out.CORS = &defaultCorsConfig
	}
	if !out.ExportUnifiedTS && strings.TrimSpace(out.SchemaTSPath) == "" {
		out.SchemaTSPath = "vue/composables/auto-generated-types.ts"
	}
	return out, nil
}

// ExportTypesFromConfig exports TS files from APIServerConfig without building the server.
// ExportTypesFromConfig 根据 APIServerConfig 导出 TS 文件，但不构建服务。
func ExportTypesFromConfig(cfg APIServerConfig) error {
	cfg, err := cfg.normalized()
	if err != nil {
		return err
	}
	if cfg.ExportUnifiedTS {
		return endpoint.ExportUnifiedAPIsToTSFiles(
			cfg.ServerAPI,
			cfg.WebSocketAPI,
			endpoint.UnifiedTSExportOptions{
				ServerTSPath:    cfg.ServerTSPath,
				WebSocketTSPath: cfg.WebSocketTSPath,
				SchemaTSPath:    cfg.SchemaTSPath,
			},
		)
	}
	if err := cfg.ServerAPI.ExportTS(cfg.ServerTSPath); err != nil {
		return err
	}
	if len(cfg.WebSocketAPI.Endpoints) > 0 {
		if err := cfg.WebSocketAPI.ExportTS(cfg.WebSocketTSPath); err != nil {
			return err
		}
	}
	return nil
}

// BuildServerFromConfig builds a gin engine from APIServerConfig.
// BuildServerFromConfig 根据 APIServerConfig 构建 gin engine。
func BuildServerFromConfig(cfg APIServerConfig) (*gin.Engine, error) {
	cfg, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	if ResolveGinMode(cfg.GinMode) == gin.DebugMode {
		setupGinDebugPrinter()
	}

	engine := newGinEngine()
	if cfg.CORS != nil {
		corsCfg := *cfg.CORS
		if err := corsCfg.Validate(); err != nil {
			return nil, fmt.Errorf("invalid CORS config: %w", err)
		}
		engine.Use(cors.New(corsCfg))
	}
	ServeVue(engine)

	if _, err := cfg.ServerAPI.BuildGinGroup(engine); err != nil {
		return nil, err
	}
	if len(cfg.WebSocketAPI.Endpoints) > 0 {
		if _, err := cfg.WebSocketAPI.BuildGinGroup(engine); err != nil {
			return nil, err
		}
	}
	return engine, nil
}

// RunServerFromConfig configures gin mode, logs server info, builds router, and runs it.
// RunServerFromConfig 会配置 gin mode、打印日志、构建路由并启动服务。
func RunServerFromConfig(cfg APIServerConfig) error {
	cfg, err := cfg.normalized()
	if err != nil {
		return err
	}
	ConfigureGinMode(cfg.GinMode)
	if cfg.ExportTSOnRun {
		if err := ExportTypesFromConfig(cfg); err != nil {
			return err
		}
	}
	runtimeutil.LogServerWithBasePath(false, cfg.Server.GinPort, cfg.Server.BaseUrl)

	router, err := BuildServerFromConfig(cfg)
	if err != nil {
		return err
	}
	return router.Run(":" + fmt.Sprint(cfg.Server.GinPort))
}
