package runtime

import (
	"fmt"
	"strings"

	"github.com/RapboyGao/nuxtGin/endpoint"
	"github.com/RapboyGao/nuxtGin/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// APIServerConfig configures server runtime, API registration, and TS generation outputs.
// APIServerConfig 用于统一配置服务端口、API 注册和 TS 输出路径。
type APIServerConfig struct {
	// Server contains base URL and ports used by the runtime.
	// Server 包含运行时使用的基础 URL 与端口配置。
	Server ServerRuntimeConfig

	// EnableCORS controls whether cors.Default() is attached in debug mode.
	// EnableCORS 控制是否在调试模式挂载 cors.Default()。
	EnableCORS bool

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
}

func (c APIServerConfig) normalized() APIServerConfig {
	out := c
	if out.Server.GinPort <= 0 {
		out.Server.GinPort = GetConfig.GinPort
	}
	if strings.TrimSpace(out.Server.BaseUrl) == "" {
		out.Server.BaseUrl = GetConfig.BaseUrl
	}
	if out.Server.NuxtPort <= 0 {
		out.Server.NuxtPort = GetConfig.NuxtPort
	}
	if strings.TrimSpace(out.ServerTSPath) == "" {
		out.ServerTSPath = "vue/composables/auto-generated-api.ts"
	}
	if strings.TrimSpace(out.WebSocketTSPath) == "" {
		out.WebSocketTSPath = "vue/composables/auto-generated-ws.ts"
	}
	if strings.TrimSpace(out.SchemaTSPath) == "" {
		out.SchemaTSPath = "vue/composables/auto-generated-shared.ts"
	}

	if strings.TrimSpace(out.ServerAPI.BasePath) == "" {
		out.ServerAPI.BasePath = "/api-go/v1"
	}
	if strings.TrimSpace(out.ServerAPI.GroupPath) == "" {
		out.ServerAPI.GroupPath = out.ServerAPI.BasePath
	}
	if strings.TrimSpace(out.WebSocketAPI.BasePath) == "" {
		out.WebSocketAPI.BasePath = "/ws-go/v1"
	}
	if strings.TrimSpace(out.WebSocketAPI.GroupPath) == "" {
		out.WebSocketAPI.GroupPath = out.WebSocketAPI.BasePath
	}
	return out
}

// BuildServerFromConfig builds a gin engine from APIServerConfig and exports TS if configured.
// BuildServerFromConfig 根据 APIServerConfig 构建 gin engine，并按配置导出 TS。
func BuildServerFromConfig(cfg APIServerConfig) (*gin.Engine, error) {
	cfg = cfg.normalized()

	if GetGinMode() == gin.DebugMode {
		setupGinDebugPrinter()
	}

	engine := newGinEngine()
	if GetGinMode() == gin.DebugMode && cfg.EnableCORS {
		engine.Use(cors.Default())
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

	if cfg.ExportUnifiedTS {
		err := endpoint.ExportUnifiedAPIsToTSFiles(
			cfg.ServerAPI,
			cfg.WebSocketAPI,
			endpoint.UnifiedTSExportOptions{
				ServerTSPath:    cfg.ServerTSPath,
				WebSocketTSPath: cfg.WebSocketTSPath,
				SchemaTSPath:    cfg.SchemaTSPath,
			},
		)
		if err != nil {
			return nil, err
		}
	} else {
		if err := cfg.ServerAPI.ExportTS(cfg.ServerTSPath); err != nil {
			return nil, err
		}
		if len(cfg.WebSocketAPI.Endpoints) > 0 {
			if err := cfg.WebSocketAPI.ExportTS(cfg.WebSocketTSPath); err != nil {
				return nil, err
			}
		}
	}

	return engine, nil
}

// RunServerFromConfig configures gin mode, logs server info, builds router, and runs it.
// RunServerFromConfig 会配置 gin mode、打印日志、构建路由并启动服务。
func RunServerFromConfig(cfg APIServerConfig) error {
	ConfigureGinMode()
	cfg = cfg.normalized()
	utils.LogServerWithBasePath(false, cfg.Server.GinPort, cfg.Server.BaseUrl)

	router, err := BuildServerFromConfig(cfg)
	if err != nil {
		return err
	}
	return router.Run(":" + fmt.Sprint(cfg.Server.GinPort))
}
