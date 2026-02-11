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

func buildCommonHTTPTestAPIs() []EndpointLike {
	return []EndpointLike{
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
}

// TestGenerateAxiosFromEndpoints
// 这个测试验证 HTTP TS 生成主流程是否完整可用，重点覆盖：
// 1) 可从一组 Go Endpoint 生成 class 风格的 TS API（每个 API 一个 class + request 便捷函数）。
// 2) 生成的 class 元数据是否正确（METHOD/NAME/SUMMARY/PATHS/FULL_PATH）。
// 3) base/group/api 路径拆分是否保留配置输入，并能拼出正确 FULL_PATH。
// 4) path/query/header/cookie 参数映射与路径参数大小写映射是否正确。
// 5) requestConfig/request/buildURL 等静态方法是否按预期生成并被 request 复用。
// 6) 结构体字段注释(tsdoc)、联合字面量(tsunion)、omitempty 可选字段是否正确落到 TS。
// 7) validator + ensure 函数是否为生成的 interface 正确输出。
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

	apis := buildCommonHTTPTestAPIs()

	outPath := filepath.Join(".generated", "schema", "http", "server_api.ts")
	httpAPI := ServerAPI{
		BasePath:  "/api",
		GroupPath: "/v1",
		Endpoints: apis,
	}
	if err := httpAPI.ExportTS(outPath); err != nil {
		t.Fatalf("ServerAPI.ExportTS returned error: %v", err)
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
	if !strings.Contains(code, "static readonly PATHS =") || !strings.Contains(code, "static readonly FULL_PATH =") || !strings.Contains(code, "/api/v1/Person/:ID") {
		t.Fatalf("expected endpoint class static PATHS/FULL_PATH generation")
	}
	if !strings.Contains(code, `base: "/api"`) || !strings.Contains(code, `group: "/v1"`) {
		t.Fatalf("expected endpoint class PATHS to preserve base/group from ServerAPI")
	}
	if !strings.Contains(code, "static async request(") {
		t.Fatalf("expected endpoint class static request method generation")
	}
	if !strings.Contains(code, "export async function requestGetPersonByIDGet(") || !strings.Contains(code, "return GetPersonByIDGet.request(") {
		t.Fatalf("expected generated convenience request function for endpoint class")
	}
	if !strings.Contains(code, "return ListPeopleGet.FULL_PATH;") {
		t.Fatalf("expected static FULL_PATH usage via class name for endpoints without path placeholders in buildURL")
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
	if !strings.Contains(code, "export function ensureGetPersonReq(") || !strings.Contains(code, "): GetPersonReq") {
		t.Fatalf("expected interface ensure function generation")
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

// TestGenerateAxiosFromEndpoints_Int64AsStringMode
// 这个测试只验证一个开关行为：
// 当 TSInt64MappingMode 设置为 TSInt64ModeString 时，Go 的 int64 字段是否生成为 TS string。
// 目的是保证“大整数按字符串传输”的模式不会回退。
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

	code, err := generateAxiosFromEndpoints("/api", "/v1", apis)
	if err != nil {
		t.Fatalf("GenerateAxiosFromEndpoints returned error: %v", err)
	}
	if !strings.Contains(code, "salary: string;") {
		t.Fatalf("expected int64 to map to string when TSInt64ModeString is enabled")
	}
}

// TestGenerateAxiosFromEndpoints_ValidationError
// 这个测试验证生成前的元数据校验逻辑：
// 当路由 path 中声明了路径参数（如 :id），但 Endpoint 的 PathParams 使用了 NoParams 时，
// 生成器必须返回明确错误，而不是继续生成无效 TS。
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

	_, err := generateAxiosFromEndpoints("/api", "/v1", apis)
	if err == nil {
		t.Fatalf("expected validation error for missing path params type")
	}
	if !strings.Contains(err.Error(), "path params required") {
		t.Fatalf("expected validation error message, got: %v", err)
	}
}

// TestGenerateAxiosFromEndpoints_CustomEndpoint_ExportTSFile
// 这个测试验证 CustomEndpoint 路径：
// 1) CustomEndpoint 能与普通 Endpoint 一样参与 TS 生成并写入文件。
// 2) RequestKind/ResponseKind 的自定义行为能体现在 TS（如 form-urlencoded 与 text 响应）。
// 3) 自定义路径参数（含 uri/json tag）插值时字段名映射正确。
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
	customAPI := ServerAPI{
		BasePath:  "/api",
		GroupPath: "/v2",
		Endpoints: []EndpointLike{custom},
	}
	if err := customAPI.ExportTS(outPath); err != nil {
		t.Fatalf("ServerAPI.ExportTS returned error: %v", err)
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

type wsClientEnvelope struct {
	Type    string          `json:"type" tsdoc:"消息类型 / Message type"`
	Payload wsClientPayload `json:"payload" tsdoc:"消息载荷 / Message payload"`
}

type wsClientPayload struct {
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

type wsServerEnvelope struct {
	Type    string          `json:"type" tsdoc:"服务端消息类型 / Server message type"`
	Payload wsServerPayload `json:"payload" tsdoc:"服务端消息载荷 / Server message payload"`
}

type wsServerPayload struct {
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

func buildCommonWSTestEndpoint() *WebSocketEndpoint {
	return &WebSocketEndpoint{
		Name:              "chat_events",
		Path:              "/chat/events",
		ClientMessageType: reflect.TypeOf(wsClientEnvelope{}),
		ServerMessageType: reflect.TypeOf(wsServerEnvelope{}),
		MessageTypes:      []string{"room:join", "chat:text", "system:ack"},
	}
}

func buildNotifyWSTestEndpoint() *WebSocketEndpoint {
	return &WebSocketEndpoint{
		Name:              "notify_events",
		Path:              "/notify/events",
		ClientMessageType: reflect.TypeOf(wsClientEnvelope{}),
		ServerMessageType: reflect.TypeOf(wsServerEnvelope{}),
		MessageTypes:      []string{"notify:new", "notify:read"},
	}
}

// TestGenerateWebSocketClientFromEndpoints_ClassAndTypedHandlers
// 这个测试验证 WebSocket TS 生成的核心能力，覆盖面最大：
// 1) 基础 TypedWebSocketClient 是否生成（状态字段、计数器、生命周期订阅等）。
// 2) URL 解析策略是否同时覆盖开发环境/生产环境分支。
// 3) validator + ensure 是否为 WS 相关 interface 正确生成。
// 4) onType/onTyped 及其 options 默认校验逻辑是否按设计输出。
// 5) 每个 WS Endpoint 专属 class（如 ChatEvents）及 message type union、onXxx helper 是否完整。
func TestGenerateWebSocketClientFromEndpoints_ClassAndTypedHandlers(t *testing.T) {
	ws := buildCommonWSTestEndpoint()

	code, err := generateWebSocketClientFromEndpoints("/ws", "/v1", []WebSocketEndpointLike{ws})
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
	if !strings.Contains(code, "export function validateWsClientEnvelope(") || !strings.Contains(code, "value is WsClientEnvelope") {
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
	if !strings.Contains(code, "export function ensureWsClientEnvelope(") || !strings.Contains(code, "): WsClientEnvelope") {
		t.Fatalf("expected websocket interface ensure function generation")
	}
	if !strings.Contains(code, "if (!validateWsClientEnvelope(value)) {") {
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
	if !strings.Contains(code, "options?: TypeHandlerOptions<WsServerEnvelope>") {
		t.Fatalf("expected endpoint type handlers to keep optional options signature")
	}
	if !strings.Contains(code, "if (options === undefined)") || !strings.Contains(code, "options = { validate: validateWsServerEnvelope };") {
		t.Fatalf("expected endpoint type handlers to provide default message validator")
	}
	if !strings.Contains(code, "options?: TypedHandlerOptions<WsServerEnvelope, TPayload>") {
		t.Fatalf("expected endpoint payload handlers to keep optional options signature")
	}
	if !strings.Contains(code, "function defaultValidatePayload(") || !strings.Contains(code, "return validateWsServerEnvelope(message);") || !strings.Contains(code, "options = { validate: defaultValidatePayload };") {
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
	if !strings.Contains(code, "options: WebSocketConvertOptions<TSend, WsServerEnvelope>") {
		t.Fatalf("expected required options in endpoint class constructor")
	}
	if !strings.Contains(code, "options: WebSocketConvertOptions<TSend, TReceive>") {
		t.Fatalf("expected required options in websocket client constructor")
	}
}

// TestGenerateWebSocketClientFromEndpoints_ExportFile
// 这个测试验证“文件导出”链路：
// 不仅要能生成字符串，还要能通过 WebSocketAPI.ExportTS 成功落盘，
// 并且输出文件中存在核心类定义，保证导出流程可直接给前端使用。
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

	ws := buildCommonWSTestEndpoint()

	outPath := filepath.Join(".generated", "schema", "sockets", "ws_client.ts")
	wsAPI := WebSocketAPI{
		BasePath:  "/ws",
		GroupPath: "/v1",
		Endpoints: []WebSocketEndpointLike{ws},
	}
	if err := wsAPI.ExportTS(outPath); err != nil {
		t.Fatalf("WebSocketAPI.ExportTS returned error: %v", err)
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

// TestGenerateWebSocketClientFromEndpoints_MultipleEndpoints_PathMetadata
// 这个测试验证多 WS Endpoint 同时生成时的稳定性与路径元数据：
// 1) 多个 endpoint class 是否都会生成（避免后者覆盖前者）。
// 2) PATHS.base/group/api 与 FULL_PATH 是否分别正确。
// 3) 每个 endpoint 的 createXxx 便捷构造函数是否都存在。
func TestGenerateWebSocketClientFromEndpoints_MultipleEndpoints_PathMetadata(t *testing.T) {
	ws1 := buildCommonWSTestEndpoint()
	ws2 := buildNotifyWSTestEndpoint()

	code, err := generateWebSocketClientFromEndpoints("/ws", "/v2", []WebSocketEndpointLike{ws1, ws2})
	if err != nil {
		t.Fatalf("generateWebSocketClientFromEndpoints returned error: %v", err)
	}

	if !strings.Contains(code, "export class ChatEvents<") || !strings.Contains(code, "export class NotifyEvents<") {
		t.Fatalf("expected multiple websocket endpoint classes generation")
	}
	if !strings.Contains(code, "static readonly PATHS = {") {
		t.Fatalf("expected websocket PATHS metadata generation")
	}
	if !strings.Contains(code, `base: "/ws"`) || !strings.Contains(code, `group: "/v2"`) {
		t.Fatalf("expected websocket PATHS base/group values")
	}
	if !strings.Contains(code, `api: "/chat/events"`) || !strings.Contains(code, `api: "/notify/events"`) {
		t.Fatalf("expected websocket PATHS api values")
	}
	if !strings.Contains(code, `static readonly FULL_PATH = "/ws/v2/chat/events"`) ||
		!strings.Contains(code, `static readonly FULL_PATH = "/ws/v2/notify/events"`) {
		t.Fatalf("expected websocket FULL_PATH generation")
	}
	if !strings.Contains(code, "export function createNotifyEvents<TSend =") {
		t.Fatalf("expected convenience create function for second websocket endpoint class")
	}
}

// TestGenerateWebSocketClientFromEndpoints_ValidationErrors
// 这个测试验证 WS 端点元数据的错误兜底（表驱动）：
// 1) path 为空时必须报错。
// 2) server message type 缺失时必须报错。
// 目标是确保不合法输入在生成阶段就被拦截，并返回可读错误信息。
func TestGenerateWebSocketClientFromEndpoints_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []WebSocketEndpointLike
		wantErr   string
	}{
		{
			name: "missing path",
			endpoints: []WebSocketEndpointLike{
				&WebSocketEndpoint{
					Name:              "bad_ws",
					Path:              "",
					ClientMessageType: reflect.TypeOf(wsClientEnvelope{}),
					ServerMessageType: reflect.TypeOf(wsServerEnvelope{}),
				},
			},
			wantErr: "path is required",
		},
		{
			name: "missing server message type",
			endpoints: []WebSocketEndpointLike{
				&WebSocketEndpoint{
					Name:              "bad_ws",
					Path:              "/bad",
					ClientMessageType: reflect.TypeOf(wsClientEnvelope{}),
				},
			},
			wantErr: "server message type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := generateWebSocketClientFromEndpoints("/ws", "/v1", tt.endpoints)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestExportUnifiedAPIsToTSFiles
// 这个测试验证“统一导出三文件”能力：
// 1) ServerAPI 与 WebSocketAPI 是否分别输出到各自文件。
// 2) 两侧共享的 interface/validator/ensure 是否去重后写入 shared schema 文件。
// 3) server/ws 文件是否正确 import shared 文件，且移除各自内联 schema 区块。
// 4) 共享文件中关键类型是否同时包含 HTTP 与 WS 所需定义。
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
		BasePath:  "/api",
		GroupPath: "/v1",
		Endpoints: buildCommonHTTPTestAPIs(),
	}
	ws := WebSocketAPI{
		BasePath:  "/ws",
		GroupPath: "/v1",
		Endpoints: []WebSocketEndpointLike{buildCommonWSTestEndpoint()},
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
	if !strings.Contains(sharedCode, "export interface WsServerEnvelope") {
		t.Fatalf("expected shared schema to include websocket interfaces")
	}
	if strings.Count(sharedCode, "export interface PathByURIID") != 1 {
		t.Fatalf("expected shared schema interface dedupe")
	}
}
