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

type GetPersonReq struct {
	PersonID string  `json:"personID" tsdoc:"人员ID / Person identifier"`
	TraceID  *string `json:"traceID,omitempty"`
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
	Page     int `json:"Page" tsdoc:"页码 / Page index"`
	PageSize int `json:"pageSize" tsdoc:"每页条数 / Page size"`
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

	if !strings.Contains(code, "export async function getPersonByID(") {
		t.Fatalf("expected generated function to preserve name casing")
	}
	if !strings.Contains(code, "params: {") || !strings.Contains(code, "ID: string;") {
		t.Fatalf("expected inline path params type to preserve casing")
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
	if !strings.Contains(code, "export function validateGetPersonReq(value: unknown): value is GetPersonReq {") {
		t.Fatalf("expected interface validator generation")
	}
	if !strings.Contains(code, "if (!( \"personID\" in obj)) return false;") {
		t.Fatalf("expected required-field validation generation")
	}
	if !strings.Contains(code, "/** 人员ID / Person identifier */") {
		t.Fatalf("expected tsdoc comment generation")
	}
	if !strings.Contains(code, "traceID?: string;") {
		t.Fatalf("expected omitempty field to become optional")
	}
	if !strings.Contains(code, "salary: string;") {
		t.Fatalf("expected int64 to map to string")
	}
	if !strings.Contains(code, "startDate: string;") {
		t.Fatalf("expected time.Time to map to string")
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
