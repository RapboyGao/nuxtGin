package schema

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HTTPMethod constrains allowed HTTP methods for API definitions.
type HTTPMethod string

const (
	HTTPMethodGet     HTTPMethod = HTTPMethod(http.MethodGet)
	HTTPMethodPost    HTTPMethod = HTTPMethod(http.MethodPost)
	HTTPMethodPut     HTTPMethod = HTTPMethod(http.MethodPut)
	HTTPMethodPatch   HTTPMethod = HTTPMethod(http.MethodPatch)
	HTTPMethodDelete  HTTPMethod = HTTPMethod(http.MethodDelete)
	HTTPMethodHead    HTTPMethod = HTTPMethod(http.MethodHead)
	HTTPMethodOptions HTTPMethod = HTTPMethod(http.MethodOptions)
)

func (m HTTPMethod) IsValid() bool {
	switch m {
	case HTTPMethodGet, HTTPMethodPost, HTTPMethodPut, HTTPMethodPatch, HTTPMethodDelete, HTTPMethodHead, HTTPMethodOptions:
		return true
	default:
		return false
	}
}

// Schema represents one OpenAPI-style endpoint without generics.
type Schema struct {
	Name        string     `json:"name,omitempty"`
	Method      HTTPMethod `json:"method"`
	Path        string     `json:"path"`
	Tags        []string   `json:"tags,omitempty"`
	Summary     string     `json:"summary,omitempty"`
	Description string     `json:"description,omitempty"`
	Deprecated  bool       `json:"deprecated,omitempty"`

	// Parameters grouped by location.
	PathParams   map[string]any `json:"pathParams,omitempty"`
	QueryParams  map[string]any `json:"queryParams,omitempty"`
	HeaderParams map[string]any `json:"headerParams,omitempty"`
	CookieParams map[string]any `json:"cookieParams,omitempty"`

	// Request metadata and payload.
	RequestContentType string `json:"requestContentType,omitempty"`
	RequestRequired    bool   `json:"requestRequired,omitempty"`
	RequestBody        any    `json:"requestBody,omitempty"`
	RequestExample     any    `json:"requestExample,omitempty"`

	// Primary response.
	ResponseCode        int            `json:"responseCode"`
	ResponseContentType string         `json:"responseContentType,omitempty"`
	ResponseHeaders     map[string]any `json:"responseHeaders,omitempty"`
	ResponseBody        any            `json:"responseBody,omitempty"`
	ResponseExample     any            `json:"responseExample,omitempty"`

	// Extra responses keyed by status code, such as "400", "401", "500".
	Responses map[string]any `json:"responses,omitempty"`

	// Security schemes and scopes, e.g. [{"bearerAuth":[]}] .
	Security []map[string][]string `json:"security,omitempty"`

	// GinHandler is the runtime gin handler for this API endpoint.
	GinHandler gin.HandlerFunc `json:"-"`
}

// RegisterToGin registers this API endpoint to a gin router.
func (a Schema) RegisterToGin(router gin.IRouter) error {
	if router == nil {
		return errors.New("router is nil")
	}
	if strings.TrimSpace(string(a.Method)) == "" {
		return errors.New("method is required")
	}
	if !a.Method.IsValid() {
		return errors.New("invalid http method")
	}
	if strings.TrimSpace(a.Path) == "" {
		return errors.New("path is required")
	}
	if a.GinHandler == nil {
		return errors.New("gin handler is required")
	}

	router.Handle(string(a.Method), a.Path, a.GinHandler)
	return nil
}

// RegisterAPIsToGin registers multiple APIs to a gin router in order.
func RegisterAPIsToGin(router gin.IRouter, apis []Schema) error {
	for i := range apis {
		if err := apis[i].RegisterToGin(router); err != nil {
			return fmt.Errorf("register api[%d] failed: %w", i, err)
		}
	}
	return nil
}
