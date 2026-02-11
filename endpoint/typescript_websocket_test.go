package endpoint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type wsClientMessage struct {
	Type    string                  `json:"type" tsdoc:"消息类型 / Message type"`
	Payload wsClientPayloadEnvelope `json:"payload" tsdoc:"消息载荷 / Message payload"`
}

type wsClientPayloadEnvelope struct {
	JoinRoom *wsClientJoinRoomPayload `json:"joinRoom,omitempty" tsdoc:"加入房间消息 / Join room payload"`
	ChatText *wsClientChatTextPayload `json:"chatText,omitempty" tsdoc:"聊天文本消息 / Chat text payload"`
}

type wsClientJoinRoomPayload struct {
	RoomID   string `json:"roomID" tsdoc:"房间ID / Room identifier"`
	ClientID string `json:"clientID" tsdoc:"客户端ID / Client identifier"`
}

type wsClientChatTextPayload struct {
	RoomID string `json:"roomID" tsdoc:"房间ID / Room identifier"`
	Text   string `json:"text" tsdoc:"文本内容 / Message text"`
}

type wsServerMessageEnvelope struct {
	Type    string                  `json:"type" tsdoc:"服务端消息类型 / Server message type"`
	Payload wsServerPayloadEnvelope `json:"payload" tsdoc:"服务端消息载荷 / Server message payload"`
}

type wsServerPayloadEnvelope struct {
	Ack       *wsServerAckPayload       `json:"ack,omitempty" tsdoc:"确认消息 / Acknowledgement payload"`
	Broadcast *wsServerBroadcastPayload `json:"broadcast,omitempty" tsdoc:"广播消息 / Broadcast payload"`
}

type wsServerAckPayload struct {
	Accepted bool   `json:"accepted" tsdoc:"是否接受 / Whether accepted"`
	Reason   string `json:"reason,omitempty" tsdoc:"原因说明 / Reason detail"`
}

type wsServerBroadcastPayload struct {
	FromClientID string `json:"fromClientID" tsdoc:"发送者客户端ID / Sender client identifier"`
	RoomID       string `json:"roomID" tsdoc:"房间ID / Room identifier"`
	Level        string `json:"level" tsunion:"warning,success,error" tsdoc:"消息等级 / Message level"`
	Priority     int    `json:"priority" tsunion:"1,2,3" tsdoc:"优先级 / Priority"`
	Text         string `json:"text" tsdoc:"广播文本 / Broadcast text"`
}

func TestGenerateWebSocketClientFromEndpoints_ClassAndTypedHandlers(t *testing.T) {
	ws := &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientMessage{}),
		ServerMessageType: reflect.TypeOf(wsServerMessageEnvelope{}),
		MessageTypes:      []string{"room:join", "chat:text", "system:ack"},
	}

	code, err := GenerateWebSocketClientFromEndpoints("/ws/v1", []WebSocketEndpointLike{ws})
	if err != nil {
		t.Fatalf("GenerateWebSocketClientFromEndpoints returned error: %v", err)
	}

	if !strings.Contains(code, "export class TypedWebSocketClient<") {
		t.Fatalf("expected class based websocket client generation")
	}
	if !strings.Contains(code, "public status:") || !strings.Contains(code, "connecting") || !strings.Contains(code, "closing") || !strings.Contains(code, "closed") {
		t.Fatalf("expected websocket client status member generation")
	}
	if !strings.Contains(code, "public readonly url: string;") {
		t.Fatalf("expected websocket client url member generation")
	}
	if !strings.Contains(code, "public messagesSent = 0;") || !strings.Contains(code, "public messagesReceived = 0;") {
		t.Fatalf("expected websocket client message counters generation")
	}
	if !strings.Contains(code, "window.location.hostname}:${resolveGinPort()}") {
		t.Fatalf("expected websocket dev-mode host with ginPort generation")
	}
	if !strings.Contains(code, "useRuntimeConfig().public.ginPort") {
		t.Fatalf("expected websocket ginPort to read from nuxt runtimeConfig public")
	}
	if !strings.Contains(code, "(import.meta as any).env?.DEV") {
		t.Fatalf("expected websocket env mode to read from import.meta env")
	}
	if !strings.Contains(code, "`${protocol}://${window.location.host}${url}`") {
		t.Fatalf("expected websocket prod-mode URL to follow browser host")
	}
	if !strings.Contains(code, "get readyState(): number {") || !strings.Contains(code, "get isOpen(): boolean {") {
		t.Fatalf("expected websocket client ready-state getters generation")
	}
	if !strings.Contains(code, "this.messagesSent += 1;") || !strings.Contains(code, "this.messagesReceived += 1;") {
		t.Fatalf("expected websocket client counters update logic")
	}
	if !strings.Contains(code, "export function validateWsClientMessage(") || !strings.Contains(code, "value is WsClientMessage") {
		t.Fatalf("expected websocket interface validator generation")
	}
	if !strings.Contains(code, "/** 消息类型 / Message type */") {
		t.Fatalf("expected websocket tsdoc comment generation")
	}
	if !strings.Contains(code, "export function validateWsClientJoinRoomPayload(") || !strings.Contains(code, "value is WsClientJoinRoomPayload") {
		t.Fatalf("expected websocket validator generation for nested client payload")
	}
	if !strings.Contains(code, "export function validateWsServerAckPayload(") || !strings.Contains(code, "value is WsServerAckPayload") {
		t.Fatalf("expected websocket validator generation for nested server payload")
	}
	if !strings.Contains(code, "level:") || !strings.Contains(code, "warning") || !strings.Contains(code, "success") || !strings.Contains(code, "error") {
		t.Fatalf("expected websocket tsunion field to generate string literal union")
	}
	if !strings.Contains(code, "typeof obj[\"level\"] ===") || !strings.Contains(code, "obj[\"level\"] ===") {
		t.Fatalf("expected websocket tsunion validator generation")
	}
	if !strings.Contains(code, "priority: 1 | 2 | 3;") {
		t.Fatalf("expected websocket numeric tsunion field generation")
	}
	if !strings.Contains(code, "typeof obj[\"priority\"] ===") || !strings.Contains(code, "obj[\"priority\"] === 3") {
		t.Fatalf("expected websocket numeric tsunion validator generation")
	}
	if !strings.Contains(code, "export function ensureWsClientMessage(") || !strings.Contains(code, "): WsClientMessage") {
		t.Fatalf("expected websocket interface ensure function generation")
	}
	if !strings.Contains(code, "if (!validateWsClientMessage(value)) {") {
		t.Fatalf("expected websocket interface ensure function to validate first")
	}
	if !strings.Contains(code, "onType(") || !strings.Contains(code, "type: TType") || !strings.Contains(code, "TypeHandlerOptions<TReceive>") {
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
	if !strings.Contains(code, "options?: TypeHandlerOptions<TReceive>") || !strings.Contains(code, "options?: TypedHandlerOptions<TReceive, TPayload>") {
		t.Fatalf("expected base on handlers to keep optional options signature")
	}
	if !strings.Contains(code, "options?: TypeHandlerOptions<WsServerMessageEnvelope>") {
		t.Fatalf("expected endpoint type handlers to keep optional options signature")
	}
	if !strings.Contains(code, "if (options === undefined)") || !strings.Contains(code, "options = { validate: validateWsServerMessageEnvelope };") {
		t.Fatalf("expected endpoint type handlers to provide default message validator")
	}
	if !strings.Contains(code, "options?: TypedHandlerOptions<WsServerMessageEnvelope, TPayload>") {
		t.Fatalf("expected endpoint payload handlers to keep optional options signature")
	}
	if !strings.Contains(code, "function defaultValidatePayload(") || !strings.Contains(code, "return validateWsServerMessageEnvelope(message);") || !strings.Contains(code, "options = { validate: defaultValidatePayload };") {
		t.Fatalf("expected endpoint payload handlers to provide default message validator")
	}
	if !strings.Contains(code, "// ignore single listener errors and continue dispatch") {
		t.Fatalf("expected listener dispatch fallback behavior")
	}
	if !strings.Contains(code, "export class ChatEvents<") || !strings.Contains(code, "extends TypedWebSocketClient<") || !strings.Contains(code, "ChatEventsMessageType") {
		t.Fatalf("expected endpoint-specific class extending TypedWebSocketClient")
	}
	if !strings.Contains(code, "export type ChatEventsMessageType =") || !strings.Contains(code, "chat:text") || !strings.Contains(code, "room:join") || !strings.Contains(code, "system:ack") {
		t.Fatalf("expected message type union alias generation")
	}
	if strings.Contains(code, "export function chatEvents<TSend =") {
		t.Fatalf("expected endpoint factory function to be removed")
	}
	if !strings.Contains(code, "onRoomJoinType(") || !strings.Contains(code, "onChatTextType(") || !strings.Contains(code, "onSystemAckType(") {
		t.Fatalf("expected endpoint-specific on<Type>Type helpers")
	}
	if !strings.Contains(code, "onRoomJoinPayload<TPayload>(") || !strings.Contains(code, "onChatTextPayload<TPayload>(") || !strings.Contains(code, "onSystemAckPayload<TPayload>(") {
		t.Fatalf("expected endpoint-specific on<Type>Payload helpers")
	}
	if !strings.Contains(code, "static readonly MESSAGE_TYPES = [") {
		t.Fatalf("expected endpoint-specific message type metadata")
	}
	if !strings.Contains(code, "options: WebSocketConvertOptions<TSend, WsServerMessageEnvelope>") {
		t.Fatalf("expected required options in endpoint class constructor")
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
		MessageTypes:      []string{"room:join", "chat:text", "system:ack"},
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
