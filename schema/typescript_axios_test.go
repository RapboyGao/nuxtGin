package schema

import (
	"strings"
	"testing"
)

type QueryRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type QueryModel struct {
	Keyword string     `json:"keyword"`
	Tags    []string   `json:"tags"`
	Range   QueryRange `json:"range"`
}

type RequestProfile struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type RequestModel struct {
	Profile RequestProfile    `json:"profile"`
	Scores  []int             `json:"scores"`
	Meta    map[string]string `json:"meta"`
}

type ErrorResponseModel struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

type ResponseItem struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type SuccessResponseModel struct {
	OK     bool                    `json:"ok"`
	Items  []ResponseItem          `json:"items"`
	Matrix [][]int                 `json:"matrix"`
	Ratios [2]float64              `json:"ratios"`
	Extra  map[string]ResponseItem `json:"extra"`
}

func TestGenerateAxiosFromSchemas_DeduplicateInterfaces(t *testing.T) {
	schemas := []Schema{
		{
			Name:   "get_user",
			Method: HTTPMethodGet,
			Path:   "/users/{id}",
			QueryParams: map[string]any{
				"query": QueryModel{
					Keyword: "alice",
					Tags:    []string{"a", "b"},
					Range:   QueryRange{Start: 1, End: 10},
				},
			},
			RequestBody: RequestModel{
				Profile: RequestProfile{Name: "Alice", Age: 20},
				Scores:  []int{90, 95},
				Meta:    map[string]string{"traceId": "t-1"},
			},
			Responses: []APIResponse{
				{StatusCode: 200, Body: SuccessResponseModel{
					OK:     true,
					Items:  []ResponseItem{{Code: "A", Count: 1}},
					Matrix: [][]int{{1, 2}},
					Ratios: [2]float64{0.1, 0.2},
					Extra:  map[string]ResponseItem{"primary": {Code: "X", Count: 2}},
				}},
			},
		},
		{
			Name:   "get_user_copy",
			Method: HTTPMethodGet,
			Path:   "/members/{id}",
			QueryParams: map[string]any{
				"query": QueryModel{
					Keyword: "bob",
					Tags:    []string{"c"},
					Range:   QueryRange{Start: 5, End: 15},
				},
			},
			RequestBody: RequestModel{
				Profile: RequestProfile{Name: "Bob", Age: 21},
				Scores:  []int{88},
				Meta:    map[string]string{"traceId": "t-2"},
			},
			Responses: []APIResponse{
				{StatusCode: 200, Body: SuccessResponseModel{
					OK:     false,
					Items:  []ResponseItem{{Code: "B", Count: 3}},
					Matrix: [][]int{{3, 4}},
					Ratios: [2]float64{0.3, 0.4},
					Extra:  map[string]ResponseItem{"primary": {Code: "Y", Count: 5}},
				}},
			},
		},
	}

	code, err := GenerateAxiosFromSchemas("/api/v1", schemas)
	if err != nil {
		t.Fatalf("GenerateAxiosFromSchemas returned error: %v", err)
	}
	if !strings.Contains(code, "const basePath = '/api/v1';") {
		t.Fatalf("expected generated code to include basePath")
	}

	if strings.Count(code, "export interface ") != 3 {
		t.Fatalf("expected exactly 3 interfaces (params/request/response) after dedup, got %d", strings.Count(code, "export interface "))
	}

	if !strings.Contains(code, "export const getusercopy = async (params: Getuserparams = {}, requestBody?: Getuserrequestbody): Promise<Getuserresponsebody> => {") {
		t.Fatalf("expected getUserCopy to reuse deduplicated interfaces")
	}
}

func TestGenerateAxiosFromSchemas_UsesStructAndCompositeTypes(t *testing.T) {
	schemas := []Schema{
		{
			Name:   "get_order",
			Method: HTTPMethodGet,
			Path:   "/users/:id/orders/{orderId}",
			QueryParams: map[string]any{
				"query": QueryModel{
					Keyword: "book",
					Tags:    []string{"new"},
					Range:   QueryRange{Start: 2, End: 8},
				},
			},
			RequestBody: RequestModel{
				Profile: RequestProfile{Name: "C", Age: 22},
				Scores:  []int{80, 81},
				Meta:    map[string]string{"traceId": "t-3"},
			},
			Responses: []APIResponse{
				{StatusCode: 400, Body: ErrorResponseModel{ErrorCode: "E400", Message: "bad request"}},
				{StatusCode: 200, Body: SuccessResponseModel{
					OK:     true,
					Items:  []ResponseItem{{Code: "P", Count: 1}},
					Matrix: [][]int{{1, 2, 3}},
					Ratios: [2]float64{0.9, 0.8},
					Extra:  map[string]ResponseItem{"primary": {Code: "Q", Count: 2}},
				}},
			},
		},
	}

	code, err := GenerateAxiosFromSchemas("/api/v1", schemas)
	if err != nil {
		t.Fatalf("GenerateAxiosFromSchemas returned error: %v", err)
	}

	if !strings.Contains(code, "${encodeURIComponent(String(params.path?.id ?? ''))}") {
		t.Fatalf("expected generated url to include :id path param replacement")
	}
	if !strings.Contains(code, "${encodeURIComponent(String(params.path?.orderId ?? ''))}") {
		t.Fatalf("expected generated url to include {orderId} path param replacement")
	}
	if !strings.Contains(code, "const url = joinBasePath(basePath, `/users/${encodeURIComponent(String(params.path?.id ?? ''))}/orders/${encodeURIComponent(String(params.path?.orderId ?? ''))}`);") {
		t.Fatalf("expected generated code to join basePath and endpoint path")
	}
	if !strings.Contains(code, "ok: boolean;") {
		t.Fatalf("expected response interface to use first 2xx response body")
	}
	if strings.Contains(code, "errorCode: string;") {
		t.Fatalf("expected non-2xx response body not to be selected as primary response")
	}
	if !strings.Contains(code, "tags: string[];") {
		t.Fatalf("expected query struct to generate slice type")
	}
	if !strings.Contains(code, "scores: number[];") {
		t.Fatalf("expected request body struct to generate number[]")
	}
	if !strings.Contains(code, "matrix: number[][];") {
		t.Fatalf("expected response body struct to generate nested array type")
	}
	if !strings.Contains(code, "ratios: number[];") {
		t.Fatalf("expected response body struct to generate array type")
	}
}
