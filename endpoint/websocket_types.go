package endpoint

import (
	"encoding/json"
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
	SetFullPath(path string)
}

type wsClient struct {
	id   string
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsClient) send(message any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(defaultWSWriteTimeout)); err != nil {
		return err
	}
	return c.conn.WriteJSON(message)
}

type wsHub struct {
	mu      sync.RWMutex
	clients map[string]*wsClient
}

func newWebSocketHub() *wsHub {
	return &wsHub{
		clients: map[string]*wsClient{},
	}
}

func (h *wsHub) add(conn *websocket.Conn) *wsClient {
	client := &wsClient{id: uuid.NewString(), conn: conn}
	h.mu.Lock()
	h.clients[client.id] = client
	h.mu.Unlock()
	return client
}

func (h *wsHub) remove(id string) {
	h.mu.Lock()
	delete(h.clients, id)
	h.mu.Unlock()
}

func (h *wsHub) sendTo(id string, message any) error {
	h.mu.RLock()
	client := h.clients[id]
	h.mu.RUnlock()
	if client == nil {
		return fmt.Errorf("websocket client not found: %s", id)
	}
	return client.send(message)
}

func (h *wsHub) broadcast(message any) error {
	h.mu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
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

func (h *wsHub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WebSocketClientsByPath stores all connected clients by websocket full path.
// WebSocketClientsByPath 按 websocket 完整路径保存所有连接的客户端。
// 注意：访问请使用 WebSocketClientsByPathMu 加锁。
var WebSocketClientsByPath = map[string]map[string]*websocket.Conn{}

// WebSocketClientsByPathMu guards WebSocketClientsByPath.
// WebSocketClientsByPathMu 用于保护 WebSocketClientsByPath。
var WebSocketClientsByPathMu sync.RWMutex

// SnapshotWebSocketClients returns a copy of current clients for the path.
// SnapshotWebSocketClients 返回指定路径当前客户端的副本。
func SnapshotWebSocketClients(path string) map[string]*websocket.Conn {
	WebSocketClientsByPathMu.RLock()
	defer WebSocketClientsByPathMu.RUnlock()
	src := WebSocketClientsByPath[path]
	out := make(map[string]*websocket.Conn, len(src))
	for id, conn := range src {
		out[id] = conn
	}
	return out
}

// BroadcastWebSocketJSON sends a JSON message to all clients of the path.
// BroadcastWebSocketJSON 向指定路径的所有客户端发送 JSON。
func BroadcastWebSocketJSON(path string, message any) error {
	clients := SnapshotWebSocketClients(path)
	var firstErr error
	for _, conn := range clients {
		if err := conn.WriteJSON(message); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SendWebSocketJSON sends a JSON message to a specific client of the path.
// SendWebSocketJSON 向指定路径的某个客户端发送 JSON。
func SendWebSocketJSON(path string, clientID string, message any) error {
	WebSocketClientsByPathMu.RLock()
	conn := WebSocketClientsByPath[path][clientID]
	WebSocketClientsByPathMu.RUnlock()
	if conn == nil {
		return fmt.Errorf("websocket client not found: %s", clientID)
	}
	return conn.WriteJSON(message)
}

// WebSocketContext provides access to the current connection and publish helpers.
// WebSocketContext 提供当前连接与发布消息的方法。
type WebSocketContext struct {
	ID       string
	Conn     *websocket.Conn
	Request  *http.Request
	endpoint *WebSocketEndpoint
}

// Send replies to the current client.
// Send 向当前客户端发送消息。
func (c *WebSocketContext) Send(message any) error {
	if c.endpoint == nil {
		return errors.New("websocket endpoint is nil")
	}
	return c.endpoint.hub.sendTo(c.ID, message)
}

// Publish broadcasts to all connected clients.
// Publish 向所有已连接客户端广播消息。
func (c *WebSocketContext) Publish(message any) error {
	if c.endpoint == nil {
		return errors.New("websocket endpoint is nil")
	}
	return c.endpoint.hub.broadcast(message)
}

// WebSocketEndpoint is a websocket endpoint definition.
// WebSocketEndpoint 是 WebSocket 端点定义。
type WebSocketEndpoint struct {
	Name        string
	Path        string
	Description string

	// Message types for TS generation. If ClientMessageType is nil, defaults to WebSocketMessage.
	// 用于 TS 生成的消息类型；若 ClientMessageType 为空则默认 WebSocketMessage。
	ClientMessageType reflect.Type
	ServerMessageType reflect.Type

	// Optional upgrader configuration. If zero-value, a default upgrader is used.
	// Upgrader 可选配置；若为空则使用默认 Upgrader。
	Upgrader websocket.Upgrader

	// Optional hooks.
	// 可选回调。
	OnConnect    func(ctx *WebSocketContext) error
	HandlerFunc  func(message any, ctx *WebSocketContext) (any, error)
	OnDisconnect func(ctx *WebSocketContext, err error)

	// Optional typed handlers based on message type.
	// When MessageHandlers is set, HandlerFunc is ignored.
	// 可选按消息类型分发的处理器；若设置则忽略 HandlerFunc。
	MessageHandlers   map[string]func(payload json.RawMessage, ctx *WebSocketContext) (any, error)
	MessageTypeGetter func(message any) (msgType string, payload json.RawMessage, err error)

	hub      *wsHub
	fullPath string
}

// NewWebSocketEndpoint constructs a WebSocketEndpoint with initialized hub.
// NewWebSocketEndpoint 构建并初始化 WebSocketEndpoint。
func NewWebSocketEndpoint() *WebSocketEndpoint {
	return &WebSocketEndpoint{
		hub:               newWebSocketHub(),
		ClientMessageType: reflect.TypeOf(WebSocketMessage{}),
	}
}

// WebSocketMeta exposes metadata for TS generation.
// WebSocketMeta 暴露 TS 生成所需的元数据。
func (s *WebSocketEndpoint) WebSocketMeta() WebSocketEndpointMeta {
	s.ensureHub()
	clientType := s.ClientMessageType
	if clientType == nil {
		clientType = reflect.TypeOf(WebSocketMessage{})
	}
	return WebSocketEndpointMeta{
		Name:              s.Name,
		Path:              s.Path,
		Description:       s.Description,
		ClientMessageType: clientType,
		ServerMessageType: s.ServerMessageType,
	}
}

// GinHandler upgrades and handles websocket connections.
// GinHandler 负责升级并处理 websocket 连接。
func (s *WebSocketEndpoint) GinHandler() gin.HandlerFunc {
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
		s.registerClient(client.id, conn)
		wsCtx := &WebSocketContext{
			ID:       client.id,
			Conn:     conn,
			Request:  ctx.Request,
			endpoint: s,
		}

		if s.OnConnect != nil {
			if err := s.OnConnect(wsCtx); err != nil {
				s.hub.remove(client.id)
				s.unregisterClient(client.id)
				_ = conn.Close()
				return
			}
		}

		var readErr error
		for {
			message, err := s.readClientMessage(conn)
			if err != nil {
				readErr = err
				break
			}
			resp, err := s.handleMessage(message, wsCtx)
			if err != nil {
				readErr = err
				break
			}
			if resp != nil {
				if err := wsCtx.Send(resp); err != nil {
					readErr = err
					break
				}
			}
		}

		s.hub.remove(client.id)
		s.unregisterClient(client.id)
		_ = conn.Close()
		if s.OnDisconnect != nil {
			s.OnDisconnect(wsCtx, readErr)
		}
	}
}

// Publish broadcasts a server message to all connected clients.
// Publish 向所有已连接客户端广播消息。
func (s *WebSocketEndpoint) Publish(message any) error {
	s.ensureHub()
	return s.hub.broadcast(message)
}

// SendTo sends a server message to a specific client.
// SendTo 向指定客户端发送消息。
func (s *WebSocketEndpoint) SendTo(clientID string, message any) error {
	s.ensureHub()
	return s.hub.sendTo(clientID, message)
}

// ConnectedCount returns the current connected client count.
// ConnectedCount 返回当前已连接客户端数量。
func (s *WebSocketEndpoint) ConnectedCount() int {
	s.ensureHub()
	return s.hub.count()
}

func (s *WebSocketEndpoint) ensureHub() {
	if s.hub == nil {
		s.hub = newWebSocketHub()
	}
}

// SetFullPath stores the full websocket path (including group path).
// SetFullPath 保存 websocket 完整路径（包含 group path）。
func (s *WebSocketEndpoint) SetFullPath(path string) {
	s.fullPath = path
}

func (s *WebSocketEndpoint) registerClient(id string, conn *websocket.Conn) {
	path := strings.TrimSpace(s.fullPath)
	if path == "" {
		return
	}
	WebSocketClientsByPathMu.Lock()
	clients, ok := WebSocketClientsByPath[path]
	if !ok {
		clients = map[string]*websocket.Conn{}
		WebSocketClientsByPath[path] = clients
	}
	clients[id] = conn
	WebSocketClientsByPathMu.Unlock()
}

func (s *WebSocketEndpoint) unregisterClient(id string) {
	path := strings.TrimSpace(s.fullPath)
	if path == "" {
		return
	}
	WebSocketClientsByPathMu.Lock()
	clients := WebSocketClientsByPath[path]
	delete(clients, id)
	if len(clients) == 0 {
		delete(WebSocketClientsByPath, path)
	}
	WebSocketClientsByPathMu.Unlock()
}

func (s *WebSocketEndpoint) readClientMessage(conn *websocket.Conn) (any, error) {
	t := s.ClientMessageType
	if t == nil {
		t = reflect.TypeOf(WebSocketMessage{})
	}
	valPtr := reflect.New(t)
	if err := conn.ReadJSON(valPtr.Interface()); err != nil {
		return nil, err
	}
	if t.Kind() == reflect.Ptr {
		return valPtr.Interface(), nil
	}
	return valPtr.Elem().Interface(), nil
}

func (s *WebSocketEndpoint) handleMessage(message any, ctx *WebSocketContext) (any, error) {
	if len(s.MessageHandlers) > 0 {
		msgType, payload, err := s.extractMessageType(message)
		if err != nil {
			return nil, err
		}
		handler := s.MessageHandlers[msgType]
		if handler == nil {
			return nil, fmt.Errorf("websocket handler not found for message type: %s", msgType)
		}
		return handler(payload, ctx)
	}
	if s.HandlerFunc == nil {
		return nil, nil
	}
	return s.HandlerFunc(message, ctx)
}

func (s *WebSocketEndpoint) extractMessageType(message any) (string, json.RawMessage, error) {
	if s.MessageTypeGetter != nil {
		return s.MessageTypeGetter(message)
	}

	switch v := message.(type) {
	case WebSocketMessage:
		return v.Type, v.Payload, nil
	case *WebSocketMessage:
		if v == nil {
			return "", nil, errors.New("websocket message is nil")
		}
		return v.Type, v.Payload, nil
	default:
		data, err := json.Marshal(message)
		if err != nil {
			return "", nil, err
		}
		var env WebSocketMessage
		if err := json.Unmarshal(data, &env); err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(env.Type) == "" {
			return "", nil, errors.New("websocket message type is empty")
		}
		return env.Type, env.Payload, nil
	}
}

// RegisterWebSocketTypedHandler registers a typed handler for a message type.
// RegisterWebSocketTypedHandler 注册指定消息类型的强类型处理器。
func RegisterWebSocketTypedHandler[Payload any](
	endpoint *WebSocketEndpoint,
	messageType string,
	handler func(payload Payload, ctx *WebSocketContext) (any, error),
) {
	if endpoint == nil {
		return
	}
	if endpoint.MessageHandlers == nil {
		endpoint.MessageHandlers = map[string]func(payload json.RawMessage, ctx *WebSocketContext) (any, error){}
	}
	endpoint.MessageHandlers[messageType] = func(payload json.RawMessage, ctx *WebSocketContext) (any, error) {
		var typed Payload
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &typed); err != nil {
				return nil, err
			}
		}
		return handler(typed, ctx)
	}
}

// WebSocketMessage is a default envelope for multi-handler messages.
// WebSocketMessage 是多 handler 消息的默认封装。
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}
