package nuxtGin

import (
	"github.com/arduino/go-paths-helper" // 文件路径操作工具
	"github.com/fatih/color"
	"github.com/gin-gonic/gin" // Gin Web框架
)

/**
 * 获取Gin框架运行模式
 * 根据项目目录下特征文件判断运行模式：
 * - 存在 node_modules 或 api_default.go 或 vue/pages/index.vue：开发模式（默认模式，输出详细日志）
 * - 不存在上述文件，但存在 vue/.output/public：生产模式（禁用详细日志，提高性能）
 * - 其它情况：生产模式
 */
func GetGinMode() string {
	// 创建指向node_modules目录的路径对象
	path1 := paths.New("node_modules")

	path2 := paths.New("server/api/api_default.go")
	path3 := paths.New("vue/pages/index.vue")
	path4 := paths.New("vue/.output/public")

	// 将路径转换为绝对路径（基于当前工作目录）
	path1.ToAbs()
	// 转换api_default.go的绝对路径
	path2.ToAbs()
	// 转换vue/pages/index.vue的绝对路径
	path3.ToAbs()
	// 转换vue/.output/public的绝对路径
	path4.ToAbs()

	// 判断node_modules目录是否存在
	if path1.IsDir() || !path2.NotExist() || !path3.NotExist() {
		return gin.DebugMode
	}
	if path4.IsDir() {
		return gin.ReleaseMode
	}
	return gin.ReleaseMode
}

/**
 * 配置Gin框架运行模式
 * 根据项目目录下特征文件决定运行模式。
 */
func ConfigureGinMode() {
	mode := GetGinMode()
	gin.SetMode(mode)

	// 开发环境：存在node_modules目录，使用调试模式
	if mode == gin.DebugMode {
		color.New(color.FgGreen).Println("gin mode: Debug")
	} else {
		// 生产环境：不存在node_modules目录，使用生产模式
		color.New(color.FgGreen).Println("gin mode: Release")
	}
}
