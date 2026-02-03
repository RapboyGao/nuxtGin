package apiSchema

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// HTTPMethod defines the allowed HTTP method values used by Schema.Method.
// HTTPMethod 定义 Schema.Method 可使用的 HTTP 方法值，避免直接写字符串导致拼写错误。
type HTTPMethod string

const (
	// HTTPMethodGet represents HTTP GET.
	// HTTPMethodGet 表示 HTTP GET 方法。
	HTTPMethodGet HTTPMethod = HTTPMethod(http.MethodGet)
	// HTTPMethodPost represents HTTP POST.
	// HTTPMethodPost 表示 HTTP POST 方法。
	HTTPMethodPost HTTPMethod = HTTPMethod(http.MethodPost)
	// HTTPMethodPut represents HTTP PUT.
	// HTTPMethodPut 表示 HTTP PUT 方法。
	HTTPMethodPut HTTPMethod = HTTPMethod(http.MethodPut)
	// HTTPMethodPatch represents HTTP PATCH.
	// HTTPMethodPatch 表示 HTTP PATCH 方法。
	HTTPMethodPatch HTTPMethod = HTTPMethod(http.MethodPatch)
	// HTTPMethodDelete represents HTTP DELETE.
	// HTTPMethodDelete 表示 HTTP DELETE 方法。
	HTTPMethodDelete HTTPMethod = HTTPMethod(http.MethodDelete)
	// HTTPMethodHead represents HTTP HEAD.
	// HTTPMethodHead 表示 HTTP HEAD 方法。
	HTTPMethodHead HTTPMethod = HTTPMethod(http.MethodHead)
	// HTTPMethodOptions represents HTTP OPTIONS.
	// HTTPMethodOptions 表示 HTTP OPTIONS 方法。
	HTTPMethodOptions HTTPMethod = HTTPMethod(http.MethodOptions)
)

// IsValid returns whether m is one of the supported HTTP method constants.
// IsValid 用于判断 m 是否是当前库支持的 HTTPMethod 常量之一。
func (m HTTPMethod) IsValid() bool {
	switch m {
	case HTTPMethodGet, HTTPMethodPost, HTTPMethodPut, HTTPMethodPatch, HTTPMethodDelete, HTTPMethodHead, HTTPMethodOptions:
		return true
	default:
		return false
	}
}

// ApiSchema describes a single API endpoint definition used for:
// 1) runtime route registration in Gin; and
// 2) TypeScript axios client generation.
// ApiSchema 描述一个接口端点定义，可用于：
// 1) 在 Gin 中注册运行时路由；
// 2) 生成 TypeScript axios 客户端代码。
type ApiSchema struct {
	// Name is a logical API name, used by TS generator to build function/type names.
	// Name 是接口逻辑名称，TS 生成器会优先用它构造函数名和类型名。
	Name string `json:"name,omitempty"`
	// Method is the HTTP verb of this endpoint.
	// Method 表示该接口的 HTTP 方法。
	Method HTTPMethod `json:"method"`
	// Path is the route template (supports :id and {id} placeholders).
	// Path 是路由模板（支持 :id 和 {id} 占位符）。
	Path string `json:"path"`
	// Description is the API-level description used for docs/comments generation.
	// Description 是接口级说明，可用于文档与生成代码注释。
	Description string `json:"description,omitempty"`
	// Parameters grouped by location.
	// 按位置分组的参数定义。
	PathParams   map[string]any `json:"pathParams,omitempty"`
	QueryParams  map[string]any `json:"queryParams,omitempty"`
	HeaderParams map[string]any `json:"headerParams,omitempty"`
	CookieParams map[string]any `json:"cookieParams,omitempty"`

	// Request metadata and payload.
	// 请求元信息与请求体定义。
	RequestRequired bool `json:"requestRequired,omitempty"`
	RequestBody     any  `json:"requestBody,omitempty"`
	// RequestDescription is the request-level description.
	// RequestDescription 是请求级说明。
	RequestDescription string `json:"requestDescription,omitempty"`

	// Unified response definitions.
	// 统一响应定义（建议至少提供一个 2xx 响应作为成功响应）。
	Responses []APIResponse `json:"responses,omitempty"`

	// Security schemes and scopes, e.g. [{"bearerAuth":[]}] .
	// 鉴权方案与 scope，例如 [{"bearerAuth":[]}]。
	Security []map[string][]string `json:"security,omitempty"`

	// GinHandler is the runtime gin handler for this API endpoint.
	// GinHandler 是该接口对应的 Gin 运行时处理函数，不参与 JSON 序列化。
	GinHandler gin.HandlerFunc `json:"-"`
}

// APIResponse defines one response variant for an endpoint (status code + payload).
// APIResponse 定义接口的一种响应分支（状态码 + 响应体等信息）。
type APIResponse struct {
	// StatusCode is the HTTP status code, e.g. 200/400/500.
	// StatusCode 为 HTTP 状态码，如 200/400/500。
	StatusCode int `json:"statusCode"`
	// Body is the response payload schema/example object.
	// Body 为响应体结构/示例对象。
	Body any `json:"body,omitempty"`
	// Description is the response-level description used for docs/comments generation.
	// Description 是响应级说明，可用于文档与生成代码注释。
	Description string `json:"description,omitempty"`
}

// RegisterToGin validates the Schema and registers it into a gin.IRouter.
// It checks router/method/path/handler before registration.
// RegisterToGin 会先校验 Schema（router、method、path、handler），
// 校验通过后将该接口注册到 gin.IRouter。
func (a ApiSchema) RegisterToGin(router gin.IRouter) error {
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

// RegisterAPIsToGin registers a batch of schemas in order.
// It stops on first error and returns the failed index for quick diagnosis.
// RegisterAPIsToGin 按顺序批量注册接口，遇到第一个错误即停止，
// 并在错误中携带失败索引，方便快速定位。
func RegisterAPIsToGin(router gin.IRouter, apis []ApiSchema) error {
	for i := range apis {
		if err := apis[i].RegisterToGin(router); err != nil {
			return fmt.Errorf("register api[%d] failed: %w", i, err)
		}
	}
	return nil
}

// GenerateAxiosFromSchemas generates TypeScript axios client source code from schemas.
// basePath is prefixed to each endpoint path in generated code.
// GenerateAxiosFromSchemas 根据 Schema 列表生成 TypeScript axios 客户端代码。
// basePath 会作为统一前缀拼接到每个接口路径。
func GenerateAxiosFromSchemas(basePath string, schemas []ApiSchema) (string, error) {
	return generateAxiosFromSchemas(basePath, schemas)
}

// ExportAxiosFromSchemasToTSFile generates TS axios code and writes it to a file path
// relative to current working directory (cwd). Absolute paths are rejected.
// ExportAxiosFromSchemasToTSFile 生成 TS axios 代码并写入相对 cwd 的文件路径，
// 不允许传入绝对路径。
func ExportAxiosFromSchemasToTSFile(basePath string, schemas []ApiSchema, relativeTSPath string) error {
	return exportAxiosFromSchemasToTSFile(basePath, schemas, relativeTSPath)
}

// RegisterSchemasAndExportTSInDevMode registers schemas to Gin and exports TS axios
// client code only when Gin is running in development mode (gin.DebugMode).
// It uses the default output path: vue/composables/my-schemas.ts.
// In non-development mode, this function does nothing and returns nil.
// RegisterSchemasAndExportTSInDevMode 会在 Gin 处于开发模式（gin.DebugMode）时，
// 同时完成路由批量注册与 TS 客户端导出；默认输出路径为
// vue/composables/my-schemas.ts。非开发模式下不执行任何操作并返回 nil。
func RegisterSchemasAndExportTSInDevMode(router gin.IRouter, schemas []ApiSchema, basePath string) error {
	return RegisterSchemasAndExportTSInDevModeWithPath(router, schemas, basePath, "vue/composables/my-schemas.ts")
}

// RegisterSchemasAndExportTSInDevModeWithPath behaves like
// RegisterSchemasAndExportTSInDevMode, but allows a custom TS output path.
// RegisterSchemasAndExportTSInDevModeWithPath 与默认函数行为一致，
// 但允许自定义 TS 输出路径。
func RegisterSchemasAndExportTSInDevModeWithPath(router gin.IRouter, schemas []ApiSchema, basePath string, relativeTSPath string) error {
	if gin.Mode() != gin.DebugMode {
		return nil
	}
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/my-schemas.ts"
	}
	if err := RegisterAPIsToGin(router, schemas); err != nil {
		return err
	}
	return ExportAxiosFromSchemasToTSFile(basePath, schemas, relativeTSPath)
}

// BuildRouterGroupFromSchemas creates a gin.RouterGroup, registers all schemas,
// optionally exports TS axios code, and returns the group for composition.
// BuildRouterGroupFromSchemas 会创建一个 gin.RouterGroup，
// 批量注册所有 ApiSchema，并可选导出 TS 客户端代码，然后返回该 group。
// If relativeTSPath is empty, it defaults to vue/composables/my-schemas.ts.
// 若 relativeTSPath 为空，则默认使用 vue/composables/my-schemas.ts。
func BuildRouterGroupFromSchemas(engine *gin.Engine, groupPath string, schemas []ApiSchema, basePath string, relativeTSPath string) (*gin.RouterGroup, error) {
	if engine == nil {
		return nil, errors.New("engine is nil")
	}
	if strings.TrimSpace(relativeTSPath) == "" {
		relativeTSPath = "vue/composables/my-schemas.ts"
	}
	group := engine.Group(groupPath)
	if err := RegisterAPIsToGin(group, schemas); err != nil {
		return nil, err
	}
	if err := ExportAxiosFromSchemasToTSFile(basePath, schemas, relativeTSPath); err != nil {
		return nil, err
	}
	return group, nil
}
