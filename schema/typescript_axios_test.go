package schema

import (
	"strings"
	"testing"
)

func TestGenerateAxiosFromSchemas_DeduplicateInterfaces(t *testing.T) {
	schemas := []Schema{
		{
			Name:        "get_user",
			Method:      HTTPMethodGet,
			Path:        "/users/{id}",
			QueryParams: map[string]any{"verbose": true},
			RequestBody: map[string]any{"traceId": ""},
			Responses: []APIResponse{
				{StatusCode: 200, Body: map[string]any{"id": 1, "name": ""}},
			},
		},
		{
			Name:        "get_user_copy",
			Method:      HTTPMethodGet,
			Path:        "/members/{id}",
			QueryParams: map[string]any{"verbose": true},
			RequestBody: map[string]any{"traceId": ""},
			Responses: []APIResponse{
				{StatusCode: 200, Body: map[string]any{"id": 2, "name": ""}},
			},
		},
	}

	code, err := GenerateAxiosFromSchemas(schemas)
	if err != nil {
		t.Fatalf("GenerateAxiosFromSchemas returned error: %v", err)
	}

	if strings.Count(code, "export interface ") != 3 {
		t.Fatalf("expected exactly 3 interfaces (params/request/response) after dedup, got %d", strings.Count(code, "export interface "))
	}

	if !strings.Contains(code, "export const getusercopy = async (params: Getuserparams = {}, requestBody?: Getuserrequestbody): Promise<Getuserresponsebody> => {") {
		t.Fatalf("expected getUserCopy to reuse deduplicated interfaces")
	}
}

func TestGenerateAxiosFromSchemas_Uses2xxResponseAndPathParams(t *testing.T) {
	schemas := []Schema{
		{
			Name:   "get_order",
			Method: HTTPMethodGet,
			Path:   "/users/:id/orders/{orderId}",
			Responses: []APIResponse{
				{StatusCode: 400, Body: map[string]any{"error": ""}},
				{StatusCode: 200, Body: map[string]any{"ok": true}},
			},
		},
	}

	code, err := GenerateAxiosFromSchemas(schemas)
	if err != nil {
		t.Fatalf("GenerateAxiosFromSchemas returned error: %v", err)
	}

	if !strings.Contains(code, "${encodeURIComponent(String(params.path?.id ?? ''))}") {
		t.Fatalf("expected generated url to include :id path param replacement")
	}
	if !strings.Contains(code, "${encodeURIComponent(String(params.path?.orderId ?? ''))}") {
		t.Fatalf("expected generated url to include {orderId} path param replacement")
	}
	if !strings.Contains(code, "ok: boolean;") {
		t.Fatalf("expected response interface to use first 2xx response body")
	}
	if strings.Contains(code, "error: string;") {
		t.Fatalf("expected non-2xx response body not to be selected as primary response")
	}
}
