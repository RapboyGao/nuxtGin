package endpoint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type wsClientMessage struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type wsServerMessage struct {
	Type string `json:"type"`
	Body string `json:"body"`
}

func TestGenerateWebSocketClientFromEndpoints_ClassAndTypedHandlers(t *testing.T) {
	ws := &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientMessage{}),
		ServerMessageType: reflect.TypeOf(wsServerMessage{}),
	}

	code, err := GenerateWebSocketClientFromEndpoints("/ws/v1", []WebSocketEndpointLike{ws})
	if err != nil {
		t.Fatalf("GenerateWebSocketClientFromEndpoints returned error: %v", err)
	}

	if !strings.Contains(code, "export class TypedWebSocketClient<") {
		t.Fatalf("expected class based websocket client generation")
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
	if !strings.Contains(code, "TypedWebSocketClient<WsServerMessage, TSend>") {
		t.Fatalf("expected endpoint factory return type to use generic send type")
	}
	if !strings.Contains(code, "export function chatEvents<TSend =") {
		t.Fatalf("expected endpoint factory function generation")
	}
}

func TestGenerateWebSocketClientFromEndpoints_ExportFile(t *testing.T) {
	ws := &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientMessage{}),
		ServerMessageType: reflect.TypeOf(wsServerMessage{}),
	}

	outPath := filepath.Join(".generated", "schema", "ws_client.ts")
	if err := ExportWebSocketClientFromEndpointsToTSFile("/ws/v1", []WebSocketEndpointLike{ws}, outPath); err != nil {
		t.Fatalf("ExportWebSocketClientFromEndpointsToTSFile returned error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated ts file failed: %v", err)
	}
	code := string(data)
	if !strings.Contains(code, "export class TypedWebSocketClient<") {
		t.Fatalf("expected generated websocket class in output file")
	}
}
