package apiSchema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type GetPersonRequest struct {
	PersonID string  `json:"personID"`
	TraceID  *string `json:"traceID,omitempty"`
}

type ResumeItem struct {
	Company    string    `json:"company"`
	Title      string    `json:"title"`
	StartDate  time.Time `json:"startDate"`
	EndDate    time.Time `json:"endDate"`
	Attachment []byte    `json:"attachment,omitempty"`
}

type PersonDetailResponse struct {
	PersonID string       `json:"personID"`
	Name     string       `json:"name"`
	Age      int          `json:"age"`
	Salary   int64        `json:"salary"`
	Resumes  []ResumeItem `json:"resumes"`
}

type GetPersonResumesByRangeRequest struct {
	PersonID  string `json:"personID"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

func TestGenerateAndExportAxiosTS(t *testing.T) {
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

	schemas := []ApiSchema{
		{
			Name:               "get_person_detail",
			Method:             HTTPMethodPost,
			Path:               "/person/detail",
			Description:        "Get detail of one person.",
			RequestDescription: "Request by personID.",
			RequestBody:        GetPersonRequest{},
			Responses: []APIResponse{
				{StatusCode: 200, Description: "Person detail payload.", Body: PersonDetailResponse{}},
			},
		},
		{
			Name:        "get_person_resumes_by_range",
			Method:      HTTPMethodPost,
			Path:        "/person/resumes/range",
			RequestBody: GetPersonResumesByRangeRequest{},
			Responses: []APIResponse{
				{StatusCode: 200, Body: []ResumeItem{}},
			},
		},
		{
			Name:        "batch_upsert_resumes",
			Method:      HTTPMethodPost,
			Path:        "/person/resumes/batch-upsert",
			RequestBody: []ResumeItem{{}},
			Responses: []APIResponse{
				{StatusCode: 200, Body: map[string]ResumeItem{}},
			},
		},
		{
			Name:   "get_person_by_id",
			Method: HTTPMethodGet,
			Path:   "/person/:id",
			PathParams: map[string]any{
				"id": "p-1",
			},
			Responses: []APIResponse{
				{StatusCode: 200, Body: PersonDetailResponse{}},
			},
		},
	}

	outPath := filepath.Join(".generated", "schema", "person_api.ts")
	if err := ExportAxiosFromSchemasToTSFile("/api/v1", schemas, outPath); err != nil {
		t.Fatalf("ExportAxiosFromSchemasToTSFile returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(moduleRoot, outPath))
	if err != nil {
		t.Fatalf("read generated ts file failed: %v", err)
	}
	code := string(data)

	if !strings.Contains(code, "export interface GetPersonRequest") {
		t.Fatalf("expected GetPersonRequest interface")
	}
	if !strings.Contains(code, "export interface ResumeItem") {
		t.Fatalf("expected ResumeItem interface")
	}
	if strings.Count(code, "export interface ResumeItem") != 1 {
		t.Fatalf("expected ResumeItem interface to be generated once, got %d", strings.Count(code, "export interface ResumeItem"))
	}
	if !strings.Contains(code, "export interface PersonDetailResponse") {
		t.Fatalf("expected PersonDetailResponse interface")
	}
	if !strings.Contains(code, "resumes: ResumeItem[];") {
		t.Fatalf("expected resumes field to be ResumeItem[]")
	}
	if !strings.Contains(code, "startDate: string;") || !strings.Contains(code, "endDate: string;") {
		t.Fatalf("expected time.Time fields startDate/endDate to map as string")
	}
	if !strings.Contains(code, "traceID?: string;") {
		t.Fatalf("expected omitempty field to become optional property")
	}
	if !strings.Contains(code, "attachment?: string;") {
		t.Fatalf("expected []byte field to be mapped as optional string")
	}
	if !strings.Contains(code, "salary: string;") {
		t.Fatalf("expected int64 field to be mapped to string")
	}
	if !strings.Contains(code, "export interface AxiosConvertOptions") {
		t.Fatalf("expected conversion options interface in generated code")
	}
	if !strings.Contains(code, "export async function getPersonDetail") {
		t.Fatalf("expected generated axios function")
	}
	if !strings.Contains(code, "/**") ||
		!strings.Contains(code, "Get detail of one person.") ||
		!strings.Contains(code, "@request Request by personID.") ||
		!strings.Contains(code, "@response 200 Person detail payload.") {
		t.Fatalf("expected generated function docs from schema descriptions")
	}
	if strings.Contains(code, "export interface GetPersonDetailParams") ||
		strings.Contains(code, "export interface GetPersonResumesByRangeParams") ||
		strings.Contains(code, "export interface BatchUpsertResumesParams") ||
		strings.Contains(code, "export interface GetPersonByIdParams") {
		t.Fatalf("expected no params interfaces when params are not defined")
	}
	if !strings.Contains(code, "export async function getPersonDetail(requestBody: GetPersonRequest, options?: AxiosConvertOptions<GetPersonRequest, PersonDetailResponse>): Promise<PersonDetailResponse> {") {
		t.Fatalf("expected getPersonDetail function to have only requestBody argument")
	}
	if !strings.Contains(code, "export async function getPersonResumesByRange(") ||
		!strings.Contains(code, "requestBody: GetPersonResumesByRangeRequest") ||
		!strings.Contains(code, "Promise<ResumeItem[]>") {
		t.Fatalf("expected response array type to be Promise<ResumeItem[]> without wrapper interface")
	}
	if !strings.Contains(code, "export async function batchUpsertResumes(") ||
		!strings.Contains(code, "requestBody: ResumeItem[]") ||
		!strings.Contains(code, "Promise<Record<string, ResumeItem>>") {
		t.Fatalf("expected request array and response dictionary to use direct TS types")
	}
	if strings.Contains(code, "Promise<GetPersonResumesByRangeResponse200Body>") {
		t.Fatalf("should not generate wrapper response interface for array response")
	}
	if !strings.Contains(code, "${encodeURIComponent(String(params.path?.id ?? ''))}") {
		t.Fatalf("expected :id path param replacement in generated url")
	}
	if !strings.Contains(code, "export async function getPersonById(params: {") {
		t.Fatalf("expected map-based params to be inlined in function signature")
	}
	if !strings.Contains(code, "options?: AxiosConvertOptions<never, PersonDetailResponse>") {
		t.Fatalf("expected options generic for requestless function")
	}
	if strings.Contains(code, "} = {}): Promise<PersonDetailResponse>") {
		t.Fatalf("expected getPersonById params to be required when path params exist")
	}
}

func TestGenerateAxiosFromSchemas_ValidationError(t *testing.T) {
	schemas := []ApiSchema{
		{
			Name:   "invalid_params",
			Method: HTTPMethodGet,
			Path:   "/person/:id",
			PathParams: map[string]any{
				"personID": "p-1",
			},
			QueryParams: map[string]any{
				"id": "p-1",
			},
			Responses: []APIResponse{
				{StatusCode: 200, Body: PersonDetailResponse{}},
			},
		},
	}

	_, err := GenerateAxiosFromSchemas("/api/v1", schemas)
	if err == nil {
		t.Fatalf("expected validation error when path/query params mismatch with path")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("expected validation error message, got: %v", err)
	}
}
