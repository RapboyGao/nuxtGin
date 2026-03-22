package runtime

import (
	"os"
	"strings"

	"github.com/RapboyGao/nuxtGin/internal/runtimeutil"
	"github.com/arduino/go-paths-helper" // 文件路径操作工具
	"github.com/gin-gonic/gin"           // Gin Web框架
)

func normalizeGinModeValue(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "debug":
		return gin.DebugMode
	case "release":
		return gin.ReleaseMode
	default:
		return ""
	}
}

func detectGinModeByFilesystem() string {
	path1 := paths.New("node_modules")
	path2 := paths.New("server/api/api_default.go")
	path3 := paths.New("vue/pages/index.vue")
	path4 := paths.New("vue/.output/public")

	path1.ToAbs()
	path2.ToAbs()
	path3.ToAbs()
	path4.ToAbs()

	if path1.IsDir() || !path2.NotExist() || !path3.NotExist() {
		return gin.DebugMode
	}
	if path4.IsDir() {
		return gin.ReleaseMode
	}
	return gin.ReleaseMode
}

/**
 * 获取Gin框架运行模式
 * 根据项目目录下特征文件判断运行模式：
 * - 存在 node_modules 或 api_default.go 或 vue/pages/index.vue：开发模式（默认模式，输出详细日志）
 * - 不存在上述文件，但存在 vue/.output/public：生产模式（禁用详细日志，提高性能）
 * - 其它情况：生产模式
 */
func GetGinMode() string {
	return ResolveGinMode("")
}

func ResolveGinMode(explicitMode string) string {
	if mode := normalizeGinModeValue(explicitMode); mode != "" {
		return mode
	}
	if mode := normalizeGinModeValue(os.Getenv("NUXT_GIN_MODE")); mode != "" {
		return mode
	}
	if mode := normalizeGinModeValue(os.Getenv("GIN_MODE")); mode != "" {
		return mode
	}
	return detectGinModeByFilesystem()
}

/**
 * 配置Gin框架运行模式
 * 根据项目目录下特征文件决定运行模式。
 */
func ConfigureGinMode(explicitMode string) string {
	mode := ResolveGinMode(explicitMode)
	gin.SetMode(mode)

	if mode == gin.DebugMode {
		runtimeutil.Print("Gin mode: Debug (development) / Gin模式：调试（开发环境）")
	} else {
		runtimeutil.Print("Gin mode: Release (production) / Gin模式：发布（生产环境）")
	}
	return mode
}
