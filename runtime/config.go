package runtime

import (
	"github.com/RapboyGao/nuxtGin/utils"
	"github.com/arduino/go-paths-helper"   // 文件路径操作工具
	jsoniter "github.com/json-iterator/go" // 高性能JSON处理库
)

/**
 * 运行时服务配置
 * 包含与前后端服务相关的配置参数
 */
type ServerRuntimeConfig struct {
	GinPort  int    `json:"ginPort"`  // Gin服务器端口
	NuxtPort int    `json:"nuxtPort"` // Nuxt应用端口
	BaseUrl  string `json:"baseUrl"`  // 应用基础URL
}

// Backward-compatible alias.
// 兼容旧命名 Config。
type Config = ServerRuntimeConfig

// 使用高性能JSON解析器实例
var json = jsoniter.ConfigFastest

/**
 * 从配置文件加载配置
 * 读取server.config.json文件并解析到Config结构体
 */
func (config *ServerRuntimeConfig) Acquire() {
	// 创建配置文件路径
	jsonPath := paths.New("server.config.json")

	// 读取文件内容
	bytes, _ := jsonPath.ReadFile()

	// 解析JSON数据到结构体
	json.Unmarshal(bytes, config)
}

// 全局配置实例，程序启动时初始化
var GetConfig *ServerRuntimeConfig = func() *ServerRuntimeConfig {
	config := &ServerRuntimeConfig{} // 创建配置实例
	config.Acquire()                 // 从文件加载配置
	return config                    // 返回初始化后的配置
}()

func LogServer() {
	utils.LogServerWithBasePath(false, GetConfig.GinPort, GetConfig.BaseUrl)
}
