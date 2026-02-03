package endpoint

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	defaultWSReadBufferSize  = 1024
	defaultWSWriteBufferSize = 1024
	defaultWSWriteTimeout    = 10 * time.Second
)

// NoMessage is a marker type meaning "no websocket message payload".
// NoMessage 是一个标记类型，表示“不发送/不接收 websocket 消息体”。
type NoMessage struct{}

// WebSocketEndpointMeta is the metadata view used to generate TypeScript.
// WebSocketEndpointMeta 是用于 TS 生成的元数据视图。
type WebSocketEndpointMeta struct {
	Name              string
	Path              string
	Description       string
	ClientMessageType reflect.Type
	ServerMessageType reflect.Type
}

// WebSocketEndpointLike is implemented by WebSocketEndpoint to expose metadata and gin handler.
// WebSocketEndpointLike 由 WebSocketEndpoint 实现，用于暴露元数据与 gin handler。
type WebSocketEndpointLike interface {
	WebSocketMeta() WebSocketEndpointMeta
	GinHandler() gin.HandlerFunc
}

type wsClient[ServerMsg any] struct {
	id   string
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient[ServerMsg]) send(message ServerMsg) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(defaultWSWriteTimeout)); err != nil {
		return err
	}
	return c.conn.WriteJSON(message)
}

type wsHub[ServerMsg any] struct {
	mu      sync.RWMutex
	clients map[string]*wsClient[ServerMsg]
}

func newWebSocketHub[ServerMsg any]() *wsHub[ServerMsg] {
	return &wsHub[ServerMsg]{
		clients: map[string]*wsClient[ServerMsg]{},
	}
}

func (h *wsHub[ServerMsg]) add(conn *websocket.Conn) *wsClient[ServerMsg] {
	client := &wsClient[ServerMsg]{id: uuid.NewString(), conn: conn}
	h.mu.Lock()
	h.clients[client.id] = client
	h.mu.Unlock()
	return client
}

func (h *wsHub[ServerMsg]) remove(id string) {
	h.mu.Lock()
	delete(h.clients, id)
	h.mu.Unlock()
}

func (h *wsHub[ServerMsg]) sendTo(id string, message ServerMsg) error {
	h.mu.RLock()
	client := h.clients[id]
	h.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("websocket client not found: %s", id)
	}
	return client.send(message)
}

func (h *wsHub[ServerMsg]) broadcast(message ServerMsg) error {
	h.mu.RLock()
	clients := make([]*wsClient[ServerMsg], 0, len(h.clients))
	for _, c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	var firstErr error
	for _, c := range clients {
		if err := c.send(message); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *wsHub[ServerMsg]) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WebSocketContext provides access to the current connection and publish helpers.
// WebSocketContext 提供当前连接与发布消息的方法。
type WebSocketContext[ClientMsg, ServerMsg any] struct {
	ID       string
	Conn     *websocket.Conn
	Request  *http.Request
	endpoint *WebSocketEndpoint[ClientMsg, ServerMsg]
}

// Send replies to the current client.
// Send 向当前客户端发送消息。
func (c *WebSocketContext[ClientMsg, ServerMsg]) Send(message ServerMsg) error {
	if c.endpoint == nil {
		return errors.New("websocket endpoint is nil")
	}
	return c.endpoint.hub.sendTo(c.ID, message)
}

// Publish broadcasts to all connected clients.
// Publish 向所有已连接客户端广播消息。
func (c *WebSocketContext[ClientMsg, ServerMsg]) Publish(message ServerMsg) error {
	if c.endpoint == nil {
		return errors.New("websocket endpoint is nil")
	}
	return c.endpoint.hub.broadcast(message)
}

// WebSocketEndpoint is a strongly-typed websocket endpoint definition.
// WebSocketEndpoint 是强类型 WebSocket 端点定义。
type WebSocketEndpoint[ClientMsg, ServerMsg any] struct {
	Name        string
	Path        string
	Description string

	// Optional upgrader configuration. If zero-value, a default upgrader is used.
	// Upgrader 可选配置；若为空则使用默认 Upgrader。
	Upgrader websocket.Upgrader

	// Optional hooks.
	// 可选回调。
	OnConnect    func(ctx *WebSocketContext[ClientMsg, ServerMsg]) error
	HandlerFunc  func(message ClientMsg, ctx *WebSocketContext[ClientMsg, ServerMsg]) (*ServerMsg, error)
	OnDisconnect func(ctx *WebSocketContext[ClientMsg, ServerMsg], err error)

	hub *wsHub[ServerMsg]
}

// NewWebSocketEndpoint constructs a WebSocketEndpoint with initialized hub.
// NewWebSocketEndpoint 构建并初始化 WebSocketEndpoint。
func NewWebSocketEndpoint[ClientMsg, ServerMsg any]() *WebSocketEndpoint[ClientMsg, ServerMsg] {
	return &WebSocketEndpoint[ClientMsg, ServerMsg]{
		hub: newWebSocketHub[ServerMsg](),
	}
}

// WebSocketMeta exposes metadata for TS generation.
// WebSocketMeta 暴露 TS 生成所需的元数据。
func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) WebSocketMeta() WebSocketEndpointMeta {
	s.ensureHub()
	return WebSocketEndpointMeta{
		Name:              s.Name,
		Path:              s.Path,
		Description:       s.Description,
		ClientMessageType: typeOf[ClientMsg](),
		ServerMessageType: typeOf[ServerMsg](),
	}
}

// GinHandler upgrades and handles websocket connections.
// GinHandler 负责升级并处理 websocket 连接。
func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) GinHandler() gin.HandlerFunc {
	s.ensureHub()
	return func(ctx *gin.Context) {
		upgrader := s.Upgrader
		if upgrader.CheckOrigin == nil {
			upgrader.CheckOrigin = func(_ *http.Request) bool { return true }
		}
		if upgrader.ReadBufferSize == 0 {
			upgrader.ReadBufferSize = defaultWSReadBufferSize
		}
		if upgrader.WriteBufferSize == 0 {
			upgrader.WriteBufferSize = defaultWSWriteBufferSize
		}

		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			return
		}
		client := s.hub.add(conn)
		wsCtx := &WebSocketContext[ClientMsg, ServerMsg]{
			ID:       client.id,
			Conn:     conn,
			Request:  ctx.Request,
			endpoint: s,
		}

		if s.OnConnect != nil {
			if err := s.OnConnect(wsCtx); err != nil {
				s.hub.remove(client.id)
				_ = conn.Close()
				return
			}
		}

		var readErr error
		for {
			var message ClientMsg
			if err := conn.ReadJSON(&message); err != nil {
				readErr = err
				break
			}
			if s.HandlerFunc == nil {
				continue
			}
			resp, err := s.HandlerFunc(message, wsCtx)
			if err != nil {
				readErr = err
				break
			}
			if resp != nil {
				if err := wsCtx.Send(*resp); err != nil {
					readErr = err
					break
				}
			}
		}

		s.hub.remove(client.id)
		_ = conn.Close()
		if s.OnDisconnect != nil {
			s.OnDisconnect(wsCtx, readErr)
		}
	}
}

// Publish broadcasts a server message to all connected clients.
// Publish 向所有已连接客户端广播消息。
func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) Publish(message ServerMsg) error {
	s.ensureHub()
	return s.hub.broadcast(message)
}

// SendTo sends a server message to a specific client.
// SendTo 向指定客户端发送消息。
func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) SendTo(clientID string, message ServerMsg) error {
	s.ensureHub()
	return s.hub.sendTo(clientID, message)
}

// ConnectedCount returns the current connected client count.
// ConnectedCount 返回当前已连接客户端数量。
func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) ConnectedCount() int {
	s.ensureHub()
	return s.hub.count()
}

func (s *WebSocketEndpoint[ClientMsg, ServerMsg]) ensureHub() {
	if s.hub == nil {
		s.hub = newWebSocketHub[ServerMsg]()
	}
}

// WebSocketAPI describes websocket endpoints, supports gin registration and TS export.
// WebSocketAPI 描述 websocket 端点，可构建 gin.RouterGroup，并生成 TS。
type WebSocketAPI struct {
	BasePath  string
	GroupPath string
	Endpoints []WebSocketEndpointLike
}

// BuildGinGroup registers all websocket endpoints and returns the RouterGroup.
// BuildGinGroup 注册所有 websocket 端点并返回 RouterGroup。
func (s WebSocketAPI) BuildGinGroup(engine *gin.Engine) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	if strings.TrimSpace(s.GroupPath) == "" {
		return nil, errors.New("group path is required")
	}
	group := engine.Group(s.GroupPath)
	if err := registerWebSocketHandlers(group, s.Endpoints); err != nil {
		return nil, err
	}
	return group, nil
}

// ExportTS generates websocket TypeScript to a relative path.
// ExportTS 会生成 websocket TypeScript 到相对路径。
func (s WebSocketAPI) ExportTS(relativeTSPath string) error {
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/auto-generated-ws.ts"
	}
	base := strings.TrimSpace(s.BasePath)
	if base == "" {
		base = s.GroupPath
	}
	return ExportWebSocketClientFromEndpointsToTSFile(base, s.Endpoints, relativeTSPath)
}

// Build builds gin.RouterGroup and exports TS in one call.
// Build 一次性完成 RouterGroup 构建与 TS 导出。
func (s WebSocketAPI) Build(engine *gin.Engine, relativeTSPath string) (*gin.RouterGroup, error) {
	group, err := s.BuildGinGroup(engine)
	if err != nil {
		return nil, err
	}
	if err := s.ExportTS(relativeTSPath); err != nil {
		return nil, err
	}
	return group, nil
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

func registerWebSocketHandlers(router gin.IRouter, endpoints []WebSocketEndpointLike) error {
	for i := range endpoints {
		meta := endpoints[i].WebSocketMeta()
		if strings.TrimSpace(meta.Path) == "" {
			return fmt.Errorf("register websocket endpoint[%d] failed: path is required", i)
		}
		router.GET(meta.Path, endpoints[i].GinHandler())
	}
	return nil
}
