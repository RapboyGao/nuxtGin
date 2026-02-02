package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type GetPersonRequest struct {
	PersonID string `json:"personID"`
}

type ResumeItem struct {
	Company   string `json:"company"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type PersonDetailResponse struct {
	PersonID string       `json:"personID"`
	Name     string       `json:"name"`
	Age      int          `json:"age"`
	Resumes  []ResumeItem `json:"resumes"`
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

	schemas := []Schema{
		{
			Name:        "get_person_detail",
			Method:      HTTPMethodPost,
			Path:        "/person/detail",
			RequestBody: GetPersonRequest{PersonID: "p-1001"},
			Responses: []APIResponse{
				{StatusCode: 200, Body: PersonDetailResponse{
					PersonID: "p-1001",
					Name:     "Alice",
					Age:      28,
					Resumes: []ResumeItem{
						{Company: "ACME", Title: "Engineer", StartDate: "2021-01-01", EndDate: "2023-12-31"},
					},
				}},
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
	if !strings.Contains(code, "export interface PersonDetailResponse") {
		t.Fatalf("expected PersonDetailResponse interface")
	}
	if !strings.Contains(code, "resumes: ResumeItem[];") {
		t.Fatalf("expected resumes field to be ResumeItem[]")
	}
	if !strings.Contains(code, "export const getPersonDetail") {
		t.Fatalf("expected generated axios function")
	}
}
