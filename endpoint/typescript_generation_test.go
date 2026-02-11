package endpoint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type PathByID struct {
	ID string `json:"id" tsdoc:"路径ID / Path identifier"`
}

type PathByUpperID struct {
	ID string `json:"ID" tsdoc:"路径ID(大写) / Uppercase path identifier"`
}

type PathByURIID struct {
	ID string `uri:"id" tsdoc:"路径ID(uri) / URI path identifier"`
}

type GetPersonReq struct {
	PersonID    string  `json:"personID" tsdoc:"人员ID / Person identifier"`
	Level       string  `json:"level" tsunion:"warning,success,error" tsdoc:"消息等级 / Message level"`
	RetryAfter  int     `json:"retryAfter" tsunion:"0,5,30" tsdoc:"重试时间(秒) / Retry delay in seconds"`
	CanFallback bool    `json:"canFallback" tsunion:"true,false" tsdoc:"是否允许降级 / Whether fallback is allowed"`
	TraceID     *string `json:"traceID,omitempty"`
}

type ResumeItem struct {
	Company   string    `json:"company" tsdoc:"公司名称 / Company name"`
	Title     string    `json:"title" tsdoc:"职位名称 / Job title"`
	StartDate time.Time `json:"startDate" tsdoc:"开始时间 / Start date"`
	EndDate   time.Time `json:"endDate" tsdoc:"结束时间 / End date"`
}

type PersonDetailResp struct {
	PersonID string       `json:"personID" tsdoc:"人员ID / Person identifier"`
	Salary   int64        `json:"salary" tsdoc:"薪资(分) / Salary in cents"`
	Resumes  []ResumeItem `json:"resumes" tsdoc:"履历列表 / Resume items"`
}

type QueryParams struct {
	Page     int `form:"page" tsdoc:"页码 / Page index"`
	PageSize int `form:"pageSize" tsdoc:"每页条数 / Page size"`
}

type HeaderParams struct {
	ClientID string `json:"ClientID" tsdoc:"客户端ID / Client identifier"`
}

type CookieParams struct {
	SessionID string `json:"sessionID" tsdoc:"会话ID / Session identifier"`
}

func TestGenerateAxiosFromEndpoints(t *testing.T) {
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

	apis := []EndpointLike{
		Endpoint[PathByID, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
			Name:        "GetPersonByID",
			Method:      HTTPMethodGet,
			Path:        "/Person/:ID",
			Description: "Get person by id.",
			HandlerFunc: func(path PathByID, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
		Endpoint[PathByUpperID, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
			Name:        "GetPersonByLowerPath",
			Method:      HTTPMethodGet,
			Path:        "/PersonByLower/:id",
			Description: "Get person by lowercase path param but uppercase field.",
			HandlerFunc: func(path PathByUpperID, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
		Endpoint[PathByURIID, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
			Name:        "GetPersonByURIPath",
			Method:      HTTPMethodGet,
			Path:        "/PersonByURI/:id",
			Description: "Get person by uri-tag path param.",
			HandlerFunc: func(path PathByURIID, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
		Endpoint[NoParams, NoParams, NoParams, NoParams, GetPersonReq, PersonDetailResp]{
			Name:               "get_person_detail",
			Method:             HTTPMethodPost,
			Path:               "/person/detail",
			RequestDescription: "Request by personID.",
			HandlerFunc: func(_ NoParams, _ NoParams, _ NoParams, _ NoParams, _ GetPersonReq, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
		Endpoint[NoParams, QueryParams, HeaderParams, CookieParams, NoBody, PersonDetailResp]{
			Name:         "list_people",
			Method:       HTTPMethodGet,
			Path:         "/people",
			QueryParams:  QueryParams{},
			HeaderParams: HeaderParams{},
			CookieParams: CookieParams{},
			HandlerFunc: func(_ NoParams, _ QueryParams, _ HeaderParams, _ CookieParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
	}

	outPath := filepath.Join(".generated", "schema", "http", "server_api.ts")
	if err := ExportAxiosFromEndpointsToTSFile("/api/v1", apis, outPath); err != nil {
		t.Fatalf("ExportAxiosFromEndpointsToTSFile returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(moduleRoot, outPath))
	if err != nil {
		t.Fatalf("read generated ts file failed: %v", err)
	}
	code := string(data)

	if !strings.Contains(code, "export class GetPersonByIDGet {") {
		t.Fatalf("expected per-endpoint class generation")
	}
	if !strings.Contains(code, "static readonly METHOD =") || !strings.Contains(code, "GET") {
		t.Fatalf("expected endpoint class static METHOD generation")
	}
	if !strings.Contains(code, "static readonly NAME =") || !strings.Contains(code, "getPersonByID") {
		t.Fatalf("expected endpoint class static NAME generation")
	}
	if !strings.Contains(code, "static readonly SUMMARY =") || !strings.Contains(code, "Get person by id.") {
		t.Fatalf("expected endpoint class static SUMMARY generation")
	}
	if !strings.Contains(code, "static pathParamsShape(): readonly string[] {") {
		t.Fatalf("expected endpoint class pathParamsShape generation")
	}
	if !strings.Contains(code, "static buildURL(") {
		t.Fatalf("expected endpoint class buildURL generation")
	}
	if !strings.Contains(code, "static requestConfig(") {
		t.Fatalf("expected endpoint class requestConfig generation")
	}
	if !strings.Contains(code, "axiosClient.request<PersonDetailResp>(") || !strings.Contains(code, "GetPersonByIDGet.requestConfig(") {
		t.Fatalf("expected request to reuse requestConfig via class name")
	}
	if !strings.Contains(code, "static readonly PATH =") || !strings.Contains(code, "/api/v1/Person/:ID") {
		t.Fatalf("expected endpoint class static PATH generation")
	}
	if !strings.Contains(code, "static async request(") {
		t.Fatalf("expected endpoint class static request method generation")
	}
	if !strings.Contains(code, "export async function requestGetPersonByIDGet(") || !strings.Contains(code, "return GetPersonByIDGet.request(") {
		t.Fatalf("expected generated convenience request function for endpoint class")
	}
	if !strings.Contains(code, "return ListPeopleGet.PATH;") {
		t.Fatalf("expected static PATH usage via class name for endpoints without path placeholders in buildURL")
	}
	if !strings.Contains(code, "params: {") || !strings.Contains(code, "ID: string;") {
		t.Fatalf("expected inline path params type to preserve casing")
	}
	if !strings.Contains(code, "export class GetPersonByLowerPathGet {") {
		t.Fatalf("expected class generation for lowercase path placeholder endpoint")
	}
	if !strings.Contains(code, `return ["ID"] as const;`) {
		t.Fatalf("expected pathParamsShape to map lowercase route param to struct field ID")
	}
	if !strings.Contains(code, `params.path?.ID ?? ""`) {
		t.Fatalf("expected buildURL to use mapped struct field ID")
	}
	if !strings.Contains(code, "export class GetPersonByURIPathGet {") {
		t.Fatalf("expected class generation for uri-tag path placeholder endpoint")
	}
	if !strings.Contains(code, "PersonByURI") || !strings.Contains(code, "params.path?.ID ?? \"\"") {
		t.Fatalf("expected uri-tag endpoint to interpolate path param with original casing (ID)")
	}
	if !strings.Contains(code, "normalizeParamKeys") {
		t.Fatalf("expected param key normalization helper")
	}
	hasQuery := strings.Contains(code, "normalizedParams.query")
	hasHeader := strings.Contains(code, "normalizedParams.header") || strings.Contains(code, "normalizedParams?.header")
	hasCookie := strings.Contains(code, "normalizedParams.cookie") || strings.Contains(code, "normalizedParams?.cookie")
	if !hasQuery || !hasHeader || !hasCookie {
		t.Fatalf("expected normalized params usage for query/header/cookie")
	}
	if !strings.Contains(code, "export interface GetPersonReq") {
		t.Fatalf("expected request interface generation")
	}
	if !strings.Contains(code, "export function validateGetPersonReq(") || !strings.Contains(code, "value is GetPersonReq") {
		t.Fatalf("expected interface validator generation")
	}
	if !strings.Contains(code, `if (!("personID" in obj)) return false;`) {
		t.Fatalf("expected required-field validation generation")
	}
	if !strings.Contains(code, "/** 人员ID / Person identifier */") {
		t.Fatalf("expected tsdoc comment generation")
	}
	if !strings.Contains(code, "traceID?: string;") {
		t.Fatalf("expected omitempty field to become optional")
	}
	if !strings.Contains(code, "level:") || !strings.Contains(code, "warning") || !strings.Contains(code, "success") || !strings.Contains(code, "error") {
		t.Fatalf("expected tsunion field to generate string literal union")
	}
	if !strings.Contains(code, "typeof obj[\"level\"] ===") || !strings.Contains(code, "obj[\"level\"] ===") {
		t.Fatalf("expected tsunion validator generation")
	}
	if !strings.Contains(code, "retryAfter: 0 | 5 | 30;") {
		t.Fatalf("expected numeric tsunion field generation")
	}
	if !strings.Contains(code, "typeof obj[\"retryAfter\"] ===") || !strings.Contains(code, "obj[\"retryAfter\"] === 30") {
		t.Fatalf("expected numeric tsunion validator generation")
	}
	if !strings.Contains(code, "canFallback: true | false;") {
		t.Fatalf("expected boolean tsunion field generation")
	}
	if !strings.Contains(code, "typeof obj[\"canFallback\"] ===") || !strings.Contains(code, "obj[\"canFallback\"] === true") {
		t.Fatalf("expected boolean tsunion validator generation")
	}
	if !strings.Contains(code, "salary: number;") {
		t.Fatalf("expected int64 to map to number")
	}
	if !strings.Contains(code, "startDate: string;") {
		t.Fatalf("expected time.Time to map to string")
	}
	if !strings.Contains(code, `query: { page: "page", pagesize: "pageSize" }`) {
		t.Fatalf("expected query key map to use form tags")
	}
}

func TestGenerateAxiosFromEndpoints_Int64AsStringMode(t *testing.T) {
	oldMode := TSInt64MappingMode
	SetTSInt64MappingMode(TSInt64ModeString)
	t.Cleanup(func() {
		SetTSInt64MappingMode(oldMode)
	})

	apis := []EndpointLike{
		Endpoint[NoParams, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
			Name:   "int64_mode_check",
			Method: HTTPMethodGet,
			Path:   "/int64-mode",
			HandlerFunc: func(_ NoParams, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
	}

	code, err := GenerateAxiosFromEndpoints("/api", apis)
	if err != nil {
		t.Fatalf("GenerateAxiosFromEndpoints returned error: %v", err)
	}
	if !strings.Contains(code, "salary: string;") {
		t.Fatalf("expected int64 to map to string when TSInt64ModeString is enabled")
	}
}

func TestGenerateAxiosFromEndpoints_ValidationError(t *testing.T) {
	apis := []EndpointLike{
		Endpoint[NoParams, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
			Name:   "invalid_path_params",
			Method: HTTPMethodGet,
			Path:   "/person/:id",
			HandlerFunc: func(_ NoParams, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
				return Response[PersonDetailResp]{StatusCode: 200}, nil
			},
		},
	}

	_, err := GenerateAxiosFromEndpoints("/api", apis)
	if err == nil {
		t.Fatalf("expected validation error for missing path params type")
	}
	if !strings.Contains(err.Error(), "path params required") {
		t.Fatalf("expected validation error message, got: %v", err)
	}
}

func TestGenerateAxiosFromEndpoints_CustomEndpoint_ExportTSFile(t *testing.T) {
	type CustomPathParams struct {
		OrderID string `uri:"orderID" json:"orderID" tsdoc:"订单ID / Order identifier"`
	}
	type CustomReq struct {
		Format string `json:"format" tsunion:"json,text" tsdoc:"返回格式 / Response format"`
	}
	type CustomResp struct {
		Result string `json:"result" tsdoc:"结果文本 / Result text"`
	}

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

	custom := CustomEndpoint[CustomPathParams, NoParams, NoParams, NoParams, CustomReq, CustomResp]{
		Name:         "submit_order_custom",
		Method:       HTTPMethodPost,
		Path:         "/custom/order/:orderID",
		Description:  "Submit order with custom endpoint.",
		RequestKind:  TSKindFormURLEncoded,
		ResponseKind: TSKindText,
		Responses: []Response[CustomResp]{
			{StatusCode: 200, Description: "ok"},
		},
		HandlerFunc: func(ctx *gin.Context) {
			ctx.String(200, "ok")
		},
	}

	outPath := filepath.Join(".generated", "schema", "custom", "custom_endpoint_api.ts")
	if err := ExportAxiosFromEndpointsToTSFile("/api/v2", []EndpointLike{custom}, outPath); err != nil {
		t.Fatalf("ExportAxiosFromEndpointsToTSFile returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(moduleRoot, outPath))
	if err != nil {
		t.Fatalf("read generated ts file failed: %v", err)
	}
	code := string(data)

	if !strings.Contains(code, "export class SubmitOrderCustomPost {") {
		t.Fatalf("expected class generation for custom endpoint")
	}
	if !strings.Contains(code, "export async function requestSubmitOrderCustomPost(") || !strings.Contains(code, "return SubmitOrderCustomPost.request(") {
		t.Fatalf("expected generated convenience request function for custom endpoint class")
	}
	if !strings.Contains(code, "toFormUrlEncoded") {
		t.Fatalf("expected form-urlencoded helper usage for custom endpoint")
	}
	if !strings.Contains(code, `responseType: "text"`) {
		t.Fatalf("expected text response type for custom endpoint")
	}
	if !strings.Contains(code, "params.path?.orderID") {
		t.Fatalf("expected path param interpolation to use orderID casing")
	}
}

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
		t.Fatalf("expected legacy endpoint factory function to be removed")
	}
	if !strings.Contains(code, "export function createChatEvents<TSend =") || !strings.Contains(code, "return new ChatEvents<TSend>(options);") {
		t.Fatalf("expected generated convenience create function for endpoint class")
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

	outPath := filepath.Join(".generated", "schema", "sockets", "ws_client.ts")
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

func TestExportUnifiedAPIsToTSFiles(t *testing.T) {
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
	if err := os.Chdir(moduleRoot); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	server := ServerAPI{
		BasePath:  "/api/v1",
		GroupPath: "/api/v1",
		Endpoints: []EndpointLike{
			Endpoint[PathByURIID, NoParams, NoParams, NoParams, NoBody, PersonDetailResp]{
				Name:   "GetPersonByURIPath",
				Method: HTTPMethodGet,
				Path:   "/PersonByURI/:id",
				HandlerFunc: func(path PathByURIID, _ NoParams, _ NoParams, _ NoParams, _ NoBody, _ *gin.Context) (Response[PersonDetailResp], error) {
					return Response[PersonDetailResp]{StatusCode: 200}, nil
				},
			},
		},
	}
	ws := WebSocketAPI{
		BasePath:  "/ws/v1",
		GroupPath: "/ws/v1",
		Endpoints: []WebSocketEndpointLike{
			&WebSocketEndpoint{
				Name:              "chat_events",
				Path:              "/chat/events",
				ClientMessageType: reflect.TypeOf(wsClientMessage{}),
				ServerMessageType: reflect.TypeOf(wsServerMessageEnvelope{}),
				MessageTypes:      []string{"room:join", "chat:text", "system:ack"},
			},
		},
	}

	opts := UnifiedTSExportOptions{
		ServerTSPath:    filepath.Join(".generated", "schema", "unified_server_api.ts"),
		WebSocketTSPath: filepath.Join(".generated", "schema", "unified_ws_client.ts"),
		SchemaTSPath:    filepath.Join(".generated", "schema", "unified_shared.ts"),
	}
	if err := ExportUnifiedAPIsToTSFiles(server, ws, opts); err != nil {
		t.Fatalf("ExportUnifiedAPIsToTSFiles returned error: %v", err)
	}

	serverCodeBytes, err := os.ReadFile(filepath.Join(moduleRoot, opts.ServerTSPath))
	if err != nil {
		t.Fatalf("read server ts file failed: %v", err)
	}
	serverCode := string(serverCodeBytes)
	wsCodeBytes, err := os.ReadFile(filepath.Join(moduleRoot, opts.WebSocketTSPath))
	if err != nil {
		t.Fatalf("read ws ts file failed: %v", err)
	}
	wsCode := string(wsCodeBytes)
	sharedCodeBytes, err := os.ReadFile(filepath.Join(moduleRoot, opts.SchemaTSPath))
	if err != nil {
		t.Fatalf("read shared ts file failed: %v", err)
	}
	sharedCode := string(sharedCodeBytes)

	if !strings.Contains(serverCode, "from './unified_shared'") {
		t.Fatalf("expected server ts to import unified shared schema")
	}
	if !strings.Contains(wsCode, "from './unified_shared'") {
		t.Fatalf("expected ws ts to import unified shared schema")
	}
	if strings.Contains(serverCode, "// #region Interfaces & Validators") {
		t.Fatalf("expected server ts to remove inlined interfaces section")
	}
	if strings.Contains(wsCode, "// #region Interfaces & Validators") {
		t.Fatalf("expected ws ts to remove inlined interfaces section")
	}
	if !strings.Contains(sharedCode, "export interface PathByURIID") {
		t.Fatalf("expected shared schema to include server interfaces")
	}
	if !strings.Contains(sharedCode, "export interface WsServerMessageEnvelope") {
		t.Fatalf("expected shared schema to include websocket interfaces")
	}
	if strings.Count(sharedCode, "export interface PathByURIID") != 1 {
		t.Fatalf("expected shared schema interface dedupe")
	}
}
