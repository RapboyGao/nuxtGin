package runtime

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/RapboyGao/nuxtGin/internal/runtimeutil"
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
func (config *ServerRuntimeConfig) Acquire() error {
	// 创建配置文件路径
	jsonPath := paths.New("server.config.json")

	// 读取文件内容
	bytes, err := jsonPath.ReadFile()
	if err != nil {
		return fmt.Errorf("read server.config.json failed: %w", err)
	}

	// 解析JSON数据到结构体
	if err := json.Unmarshal(bytes, config); err != nil {
		return fmt.Errorf("parse server.config.json failed: %w", err)
	}
	if err := config.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate checks whether the runtime config is safe to use.
// Validate 校验运行时配置是否合法。
func (config ServerRuntimeConfig) Validate() error {
	if config.GinPort <= 0 {
		return errors.New("server.config.json: ginPort must be greater than 0")
	}
	if config.NuxtPort <= 0 {
		return errors.New("server.config.json: nuxtPort must be greater than 0")
	}
	baseURL := strings.TrimSpace(config.BaseUrl)
	if baseURL == "" {
		return errors.New("server.config.json: baseUrl is required")
	}
	if !strings.HasPrefix(baseURL, "/") {
		return errors.New("server.config.json: baseUrl must start with /")
	}
	return nil
}

// 全局配置实例，按需加载。
var GetConfig = &ServerRuntimeConfig{}

var (
	loadConfigOnce sync.Once
	loadConfigErr  error
)

// LoadConfig loads and validates runtime config once.
// LoadConfig 按需加载并校验运行时配置。
func LoadConfig() (*ServerRuntimeConfig, error) {
	loadConfigOnce.Do(func() {
		cfg := &ServerRuntimeConfig{}
		loadConfigErr = cfg.Acquire()
		if loadConfigErr == nil {
			*GetConfig = *cfg
		}
	})
	if loadConfigErr != nil {
		return nil, loadConfigErr
	}
	return GetConfig, nil
}

func SetActiveConfig(config ServerRuntimeConfig) {
	*GetConfig = config
}

func LogServer() {
	if _, err := LoadConfig(); err != nil {
		panic(err)
	}
	runtimeutil.LogServerWithBasePath(false, GetConfig.GinPort, GetConfig.BaseUrl)
}
