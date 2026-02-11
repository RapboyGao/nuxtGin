package endpoint

import (
	"os"
	"path/filepath"
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

	outPath := filepath.Join(".generated", "schema", "server_api.ts")
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

	outPath := filepath.Join(".generated", "schema", "custom_endpoint_api.ts")
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
