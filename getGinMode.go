package nuxtGin

import (
	"github.com/arduino/go-paths-helper" // 文件路径操作工具
	"github.com/fatih/color"
	"github.com/gin-gonic/gin" // Gin Web框架
)

/**
 * 获取Gin框架运行模式
 * 根据项目目录下是否存在node_modules目录和api_default.go文件判断运行模式：
 * - 存在node_modules目录或api_default.go文件或vue/pages/index.vue文件：开发模式（默认模式，输出详细日志）
 * - 不存在node_modules目录且不存在api_default.go文件且不存在vue/pages/index.vue文件：生产模式（禁用详细日志，提高性能）
 */
func GetGinMode() string {
	// 创建指向node_modules目录的路径对象
	path1 := paths.New("node_modules")

	path2 := paths.New("server/api/api_default.go")
	path3 := paths.New("vue/pages/index.vue")

	// 将路径转换为绝对路径（基于当前工作目录）
	path1.ToAbs()
	// 转换api_default.go的绝对路径
	path2.ToAbs()
	// 转换vue/pages/index.vue的绝对路径
	path3.ToAbs()

	// 判断node_modules目录是否存在
	if path1.IsDir() || !path2.NotExist() || !path3.NotExist() {
		return gin.DebugMode
	} else {
		return gin.ReleaseMode
	}
}

/**
 * 配置Gin框架运行模式
 * 根据项目目录下是否存在node_modules目录决定运行模式：
 * - 存在：开发模式（默认模式，输出详细日志）
 * - 不存在：生产模式（禁用详细日志，提高性能）
 */
func ConfigureGinMode() {
	mode := GetGinMode()
	gin.SetMode(mode)

	// 开发环境：存在node_modules目录，使用调试模式
	if mode == gin.DebugMode {
		color.New(color.FgGreen).Println("/node_modules  found: using gin.DebugMode")
	} else {
		// 生产环境：不存在node_modules目录，使用生产模式
		color.New(color.FgBlue).Println("/node_modules not found: using gin.ReleaseMode")
	}
}
