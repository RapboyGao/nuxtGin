package endpoint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type wsClientMessage struct {
	Type    string                  `json:"type"`
	Payload wsClientPayloadEnvelope `json:"payload"`
}

type wsClientPayloadEnvelope struct {
	JoinRoom *wsClientJoinRoomPayload `json:"joinRoom,omitempty"`
	ChatText *wsClientChatTextPayload `json:"chatText,omitempty"`
}

type wsClientJoinRoomPayload struct {
	RoomID   string `json:"roomID"`
	ClientID string `json:"clientID"`
}

type wsClientChatTextPayload struct {
	RoomID string `json:"roomID"`
	Text   string `json:"text"`
}

type wsServerMessageEnvelope struct {
	Type    string                  `json:"type"`
	Payload wsServerPayloadEnvelope `json:"payload"`
}

type wsServerPayloadEnvelope struct {
	Ack       *wsServerAckPayload       `json:"ack,omitempty"`
	Broadcast *wsServerBroadcastPayload `json:"broadcast,omitempty"`
}

type wsServerAckPayload struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

type wsServerBroadcastPayload struct {
	FromClientID string `json:"fromClientID"`
	RoomID       string `json:"roomID"`
	Text         string `json:"text"`
}

func TestGenerateWebSocketClientFromEndpoints_ClassAndTypedHandlers(t *testing.T) {
	ws := &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientMessage{}),
		ServerMessageType: reflect.TypeOf(wsServerMessageEnvelope{}),
	}

	code, err := GenerateWebSocketClientFromEndpoints("/ws/v1", []WebSocketEndpointLike{ws})
	if err != nil {
		t.Fatalf("GenerateWebSocketClientFromEndpoints returned error: %v", err)
	}

	if !strings.Contains(code, "export class TypedWebSocketClient<") {
		t.Fatalf("expected class based websocket client generation")
	}
	if !strings.Contains(code, "export function validateWsClientMessage(value: unknown): value is WsClientMessage {") {
		t.Fatalf("expected websocket interface validator generation")
	}
	if !strings.Contains(code, "export function validateWsClientJoinRoomPayload(value: unknown): value is WsClientJoinRoomPayload {") {
		t.Fatalf("expected websocket validator generation for nested client payload")
	}
	if !strings.Contains(code, "export function validateWsServerAckPayload(value: unknown): value is WsServerAckPayload {") {
		t.Fatalf("expected websocket validator generation for nested server payload")
	}
	if !strings.Contains(code, "export function createWsClientMessage(value: unknown): WsClientMessage {") {
		t.Fatalf("expected websocket interface factory generation")
	}
	if !strings.Contains(code, "export function onReceiveWsClientMessage<TReceive, TSend>(") {
		t.Fatalf("expected websocket onReceive helper generation")
	}
	if !strings.Contains(code, "if (!validateWsClientMessage(raw)) return;") {
		t.Fatalf("expected websocket onReceive helper validation")
	}
	if !strings.Contains(code, "if (!validateWsClientMessage(value)) {") {
		t.Fatalf("expected websocket interface factory to validate before create")
	}
	if !strings.Contains(code, "addTypeHandler(type: string, handler: (message: TReceive) => void, options?: TypeHandlerOptions<TReceive>): number") {
		t.Fatalf("expected addTypeHandler registration API")
	}
	if !strings.Contains(code, "removeTypeHandler(handlerID: number): boolean") {
		t.Fatalf("expected removeTypeHandler API")
	}
	if !strings.Contains(code, "addTypedHandler<TPayload>(") {
		t.Fatalf("expected addTypedHandler API")
	}
	if !strings.Contains(code, "clearTypeHandlers(type?: string): void") {
		t.Fatalf("expected clearTypeHandlers API")
	}
	if !strings.Contains(code, "onType(type: string, handler: (message: TReceive) => void, options?: TypeHandlerOptions<TReceive>): () => void") {
		t.Fatalf("expected onType typed handler registration")
	}
	if !strings.Contains(code, "onTyped<TPayload>(") {
		t.Fatalf("expected onTyped generic payload handler registration")
	}
	if !strings.Contains(code, "validate?: (message: TReceive) => boolean;") {
		t.Fatalf("expected onType validator options generation")
	}
	if !strings.Contains(code, "validate?: (payload: unknown, message: TReceive) => boolean;") {
		t.Fatalf("expected onTyped validator options generation")
	}
	if !strings.Contains(code, "if (options?.validate && !options.validate(rawPayload, message)) return;") {
		t.Fatalf("expected onTyped validator usage")
	}
	if !strings.Contains(code, "// ignore single listener errors and continue dispatch") {
		t.Fatalf("expected listener dispatch fallback behavior")
	}
	if !strings.Contains(code, "new TypedWebSocketClient<") {
		t.Fatalf("expected endpoint factory to return class instance")
	}
	if !strings.Contains(code, "export function chatEvents<TSend =") {
		t.Fatalf("expected endpoint factory to allow custom send union types")
	}
	if !strings.Contains(code, "TypedWebSocketClient<WsServerMessageEnvelope, TSend>") {
		t.Fatalf("expected endpoint factory return type to use renamed server message type")
	}
	if !strings.Contains(code, "export function chatEvents<TSend =") {
		t.Fatalf("expected endpoint factory function generation")
	}
	if !strings.Contains(code, ">(options: WebSocketConvertOptions<TSend, WsServerMessageEnvelope>)") {
		t.Fatalf("expected required options in endpoint factory")
	}
	if !strings.Contains(code, "options: WebSocketConvertOptions<TSend, TReceive>") {
		t.Fatalf("expected required options in websocket client constructor")
	}
}

func TestGenerateWebSocketClientFromEndpoints_ExportFile(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}

	moduleRoot := cwd
	for {
		if _, statErr := os.Stat(filepath.Join(moduleRoot, "go.mod")); statErr == nil {
			break
		}
		next := filepath.Dir(moduleRoot)
		if next == moduleRoot {
			t.Fatalf("go.mod not found from cwd: %s", cwd)
		}
		moduleRoot = next
	}

	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(moduleRoot); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	ws := &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientMessage{}),
		ServerMessageType: reflect.TypeOf(wsServerMessageEnvelope{}),
	}

	outPath := filepath.Join(".generated", "schema", "ws_client.ts")
	if err := ExportWebSocketClientFromEndpointsToTSFile("/ws/v1", []WebSocketEndpointLike{ws}, outPath); err != nil {
		t.Fatalf("ExportWebSocketClientFromEndpointsToTSFile returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(moduleRoot, outPath))
	if err != nil {
		t.Fatalf("read generated ts file failed: %v", err)
	}
	code := string(data)
	if !strings.Contains(code, "export class TypedWebSocketClient<") {
		t.Fatalf("expected generated websocket class in output file")
	}
}
