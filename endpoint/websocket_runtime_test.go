package endpoint

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

func dialWS(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		t.Fatalf("dial websocket failed: %v", err)
	}
	return conn
}

func TestBroadcastAndSendWebSocketJSONByPath(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	chatEndpoint := NewWebSocketEndpoint()
	chatEndpoint.Name = "Chat"
	chatEndpoint.Path = "/chat"
	chatEndpoint.PingPeriod = 2 * time.Second
	chatEndpoint.PongWait = 4 * time.Second

	otherEndpoint := NewWebSocketEndpoint()
	otherEndpoint.Name = "Other"
	otherEndpoint.Path = "/other"
	otherEndpoint.PingPeriod = 2 * time.Second
	otherEndpoint.PongWait = 4 * time.Second

	engine := gin.New()
	api := WebSocketAPI{
		BasePath:  "/ws-go",
		GroupPath: "/v1",
		Endpoints: []WebSocketEndpointLike{chatEndpoint, otherEndpoint},
	}
	if _, err := api.BuildGinGroup(engine); err != nil {
		t.Fatalf("BuildGinGroup failed: %v", err)
	}

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	wsBase := "ws" + server.URL[len("http"):]

	chatConn1 := dialWS(t, wsBase+"/ws-go/v1/chat")
	defer chatConn1.Close()
	chatConn2 := dialWS(t, wsBase+"/ws-go/v1/chat")
	defer chatConn2.Close()
	otherConn := dialWS(t, wsBase+"/ws-go/v1/other")
	defer otherConn.Close()

	waitForCondition(t, time.Second, func() bool {
		return chatEndpoint.ConnectedCount() == 2 && otherEndpoint.ConnectedCount() == 1
	}, "expected websocket clients to register")

	broadcastMessage := map[string]any{"type": "broadcast", "payload": map[string]string{"scope": "chat"}}
	if err := BroadcastWebSocketJSON(chatEndpoint.fullPath, broadcastMessage); err != nil {
		t.Fatalf("BroadcastWebSocketJSON failed: %v", err)
	}

	for _, conn := range []*websocket.Conn{chatConn1, chatConn2} {
		var received map[string]any
		if err := conn.ReadJSON(&received); err != nil {
			t.Fatalf("ReadJSON failed: %v", err)
		}
		if received["type"] != "broadcast" {
			t.Fatalf("unexpected broadcast message: %#v", received)
		}
	}

	if err := otherConn.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	var unexpected map[string]any
	if err := otherConn.ReadJSON(&unexpected); err == nil {
		t.Fatalf("unexpected cross-path message: %#v", unexpected)
	}
	_ = otherConn.SetReadDeadline(time.Time{})

	var targetClientID string
	waitForCondition(t, time.Second, func() bool {
		clients := SnapshotWebSocketClients(chatEndpoint.fullPath)
		for id := range clients {
			targetClientID = id
			break
		}
		return targetClientID != ""
	}, "expected chat client ID")

	directMessage := map[string]any{"type": "direct", "payload": map[string]string{"scope": "single"}}
	if err := SendWebSocketJSON(chatEndpoint.fullPath, targetClientID, directMessage); err != nil {
		t.Fatalf("SendWebSocketJSON failed: %v", err)
	}

	matched := 0
	for _, conn := range []*websocket.Conn{chatConn1, chatConn2} {
		if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("SetReadDeadline failed: %v", err)
		}
		var received map[string]any
		if err := conn.ReadJSON(&received); err == nil && received["type"] == "direct" {
			matched++
		}
		_ = conn.SetReadDeadline(time.Time{})
	}
	if matched != 1 {
		t.Fatalf("expected direct message to reach exactly one client, reached %d", matched)
	}
}

func TestWebSocketHeartbeatAndDisconnectCleanup(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	wsEndpoint := NewWebSocketEndpoint()
	wsEndpoint.Name = "Heartbeat"
	wsEndpoint.Path = "/heartbeat"
	wsEndpoint.PingPeriod = 20 * time.Millisecond
	wsEndpoint.PongWait = 80 * time.Millisecond
	wsEndpoint.MessageHandlers = map[string]func(payload json.RawMessage, ctx *WebSocketContext) (any, error){}

	engine := gin.New()
	api := WebSocketAPI{
		BasePath:  "/ws-go",
		GroupPath: "/v1",
		Endpoints: []WebSocketEndpointLike{wsEndpoint},
	}
	if _, err := api.BuildGinGroup(engine); err != nil {
		t.Fatalf("BuildGinGroup failed: %v", err)
	}

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	wsURL := fmt.Sprintf("ws%s/ws-go/v1/heartbeat", server.URL[len("http"):])

	conn := dialWS(t, wsURL)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	waitForCondition(t, time.Second, func() bool {
		return wsEndpoint.ConnectedCount() == 1
	}, "expected websocket client to connect")

	time.Sleep(120 * time.Millisecond)
	if wsEndpoint.ConnectedCount() != 1 {
		t.Fatalf("expected active websocket client to survive heartbeat, count=%d", wsEndpoint.ConnectedCount())
	}

	_ = conn.Close()
	<-done
	waitForCondition(t, time.Second, func() bool {
		return wsEndpoint.ConnectedCount() == 0
	}, "expected websocket client to be removed after disconnect")
}
