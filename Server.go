package nuxtGin

import (
	"fmt"
	"log"

	"github.com/RapboyGao/nuxtGin/endpoint"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

/**
 * 创建适合本程序的 Gin 引擎实例
 * 配置了：
 * - 默认中间件（日志、恢复）
 * - CORS 跨域支持（默认配置）
 * - Vue 前端服务
 * - API 路由注册（并导出 TS）
 */
func CreateServer(endpoints []endpoint.EndpointLike) (*gin.Engine, error) {
	if GetGinMode() == gin.DebugMode {
		setupGinDebugPrinter()
	}

	// 创建默认 Gin 引擎，包含日志和恢复中间件
	engine := gin.Default()

	// 启用 CORS 支持，使用默认配置
	if GetGinMode() == gin.DebugMode {
		engine.Use(cors.Default())
	}

	// 配置 Vue 前端服务（开发环境代理 / 生产环境静态文件）
	ServeVue(engine)

	// 注册 API 路由，并导出 TS
	if _, err := endpoint.ApplyEndpoints(engine, endpoints); err != nil {
		return nil, err
	}

	return engine, nil
}

/**
 * 创建带 WebSocket 端点的 Gin 引擎实例
 * 配置了：
 * - 默认中间件（日志、恢复）
 * - CORS 跨域支持（默认配置）
 * - Vue 前端服务
 * - API 路由注册（并导出 TS）
 * - WebSocket 路由注册（并导出 TS）
 */
func CreateServerWithWebSockets(endpoints []endpoint.EndpointLike, wsEndpoints []endpoint.WebSocketEndpointLike) (*gin.Engine, error) {
	if GetGinMode() == gin.DebugMode {
		setupGinDebugPrinter()
	}

	// 创建默认 Gin 引擎，包含日志和恢复中间件
	engine := gin.Default()

	// 启用 CORS 支持，使用默认配置
	if GetGinMode() == gin.DebugMode {
		engine.Use(cors.Default())
	}

	// 配置 Vue 前端服务（开发环境代理 / 生产环境静态文件）
	ServeVue(engine)

	// 注册 API 路由，并导出 TS
	if _, err := endpoint.ApplyEndpoints(engine, endpoints); err != nil {
		return nil, err
	}

	// 注册 WebSocket 路由，并导出 TS
	if _, err := endpoint.ApplyWebSocketEndpoints(engine, wsEndpoints); err != nil {
		return nil, err
	}

	return engine, nil
}

/**
 * 运行 Gin 服务器
 * 配置了：
 * - Gin 运行模式
 * - 服务器端口
 * - 日志记录
 * - 路由注册
 */
func RunServer(endpoints []endpoint.EndpointLike) error {
	// 根据项目目录配置 Gin 运行模式
	ConfigureGinMode()

	// 记录服务器启动日志
	LogServer()

	// 创建 Gin 引擎
	router, err := CreateServer(endpoints)
	if err != nil {
		return err
	}

	// 启动 Gin 服务器
	port := ":" + fmt.Sprint(GetConfig.GinPort)
	return router.Run(port)
}

/**
 * 运行带 WebSocket 端点的 Gin 服务器
 * 配置了：
 * - Gin 运行模式
 * - 服务器端口
 * - 日志记录
 * - 路由注册
 */
func RunServerWithWebSockets(endpoints []endpoint.EndpointLike, wsEndpoints []endpoint.WebSocketEndpointLike) error {
	// 根据项目目录配置 Gin 运行模式
	ConfigureGinMode()

	// 记录服务器启动日志
	LogServer()

	// 创建 Gin 引擎
	router, err := CreateServerWithWebSockets(endpoints, wsEndpoints)
	if err != nil {
		return err
	}

	// 启动 Gin 服务器
	port := ":" + fmt.Sprint(GetConfig.GinPort)
	return router.Run(port)
}

/**
 * 运行 Gin 服务器（失败则直接退出）
 */
func MustRunServer(endpoints []endpoint.EndpointLike) {
	if err := RunServer(endpoints); err != nil {
		log.Fatal(err)
	}
}

/**
 * 运行带 WebSocket 端点的 Gin 服务器（失败则直接退出）
 */
func MustRunServerWithWebSockets(endpoints []endpoint.EndpointLike, wsEndpoints []endpoint.WebSocketEndpointLike) {
	if err := RunServerWithWebSockets(endpoints, wsEndpoints); err != nil {
		log.Fatal(err)
	}
}
