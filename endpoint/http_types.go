package endpoint

import (
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

// HTTPMethod defines the allowed HTTP method values used by Endpoint.Method.
// HTTPMethod 定义 Endpoint.Method 可使用的 HTTP 方法值，避免直接写字符串导致拼写错误。
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

// NoParams is a marker type meaning "no params".
// NoParams 是一个标记类型，表示“没有参数”。
type NoParams struct{}

// NoBody is a marker type meaning "no request body".
// NoBody 是一个标记类型，表示“没有请求体”。
type NoBody struct{}

// Response is a typed response wrapper for Endpoint handlers.
// Response 是 Endpoint 处理函数的强类型响应封装。
type Response[T any] struct {
	StatusCode  int    `json:"statusCode"`
	Body        T      `json:"body,omitempty"`
	Description string `json:"description,omitempty"`
}

// EndpointMeta is the metadata view used to generate TypeScript from Endpoint.
// EndpointMeta 是用于 TS 生成的元数据视图。
type EndpointMeta struct {
	Name               string
	Method             HTTPMethod
	Path               string
	Description        string
	RequestDescription string
	PathParamsType     reflect.Type
	QueryParamsType    reflect.Type
	HeaderParamsType   reflect.Type
	CookieParamsType   reflect.Type
	RequestBodyType    reflect.Type
	Responses          []ResponseMeta
}

// ResponseMeta is the response metadata used to generate TypeScript.
// ResponseMeta 是用于 TS 生成的响应元数据。
type ResponseMeta struct {
	StatusCode  int
	BodyType    reflect.Type
	Description string
}

// EndpointLike is implemented by Endpoint to expose metadata and gin handler.
// EndpointLike 由 Endpoint 实现，用于暴露元数据与 gin handler。
type EndpointLike interface {
	EndpointMeta() EndpointMeta
	GinHandler() gin.HandlerFunc
}

// Endpoint is a strongly-typed server API definition.
// HandlerFunc receives typed params/body and returns a typed Response.
// Endpoint 是强类型服务器端 API 定义，HandlerFunc 接收强类型参数并返回强类型 Response。
type Endpoint[PP, QP, HP, CP, Req, Resp any] struct {
	Name               string
	Method             HTTPMethod
	Path               string
	Description        string
	RequestDescription string
	PathParams         PP
	QueryParams        QP
	HeaderParams       HP
	CookieParams       CP
	RequestBody        Req
	Responses          []Response[Resp]
	HandlerFunc        func(pathParams PP, queryParams QP, headerParams HP, cookieParams CP, requestBody Req, ctx *gin.Context) (Response[Resp], error)
}

// EndpointMeta exposes metadata for TS generation.
// EndpointMeta 暴露 TS 生成所需的元数据。
func (s Endpoint[PP, QP, HP, CP, Req, Resp]) EndpointMeta() EndpointMeta {
	meta := EndpointMeta{
		Name:               s.Name,
		Method:             s.Method,
		Path:               s.Path,
		Description:        s.Description,
		RequestDescription: s.RequestDescription,
		PathParamsType:     typeOf[PP](),
		QueryParamsType:    typeOf[QP](),
		HeaderParamsType:   typeOf[HP](),
		CookieParamsType:   typeOf[CP](),
		RequestBodyType:    typeOf[Req](),
	}
	if len(s.Responses) == 0 {
		meta.Responses = []ResponseMeta{{
			StatusCode: 200,
			BodyType:   typeOf[Resp](),
		}}
		return meta
	}
	meta.Responses = make([]ResponseMeta, 0, len(s.Responses))
	for _, r := range s.Responses {
		meta.Responses = append(meta.Responses, ResponseMeta{
			StatusCode:  r.StatusCode,
			BodyType:    typeOf[Resp](),
			Description: r.Description,
		})
	}
	return meta
}

// GinHandler builds a gin.HandlerFunc that auto-binds params/body and calls HandlerFunc.
// GinHandler 会自动绑定参数/请求体并调用 HandlerFunc。
func (s Endpoint[PP, QP, HP, CP, Req, Resp]) GinHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		pathParams, err := bindStructT[PP](ctx.ShouldBindUri)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		queryParams, err := bindStructT[QP](ctx.ShouldBindQuery)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		headerParams, err := bindStructT[HP](ctx.ShouldBindHeader)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cookieParams, err := bindCookieStructT[CP](ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		requestBody, err := bindJSONStructT[Req](ctx)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		resp, callErr := s.HandlerFunc(pathParams, queryParams, headerParams, cookieParams, requestBody, ctx)
		status := http.StatusOK
		if resp.StatusCode > 0 {
			status = resp.StatusCode
		}
		if callErr != nil {
			ctx.JSON(status, gin.H{"error": callErr.Error()})
			return
		}
		ctx.JSON(status, resp.Body)
	}
}

func typeOf[T any]() reflect.Type {
	var p *T
	return reflect.TypeOf(p).Elem()
}

// NewEndpoint builds an Endpoint with a simplified handler that only returns a 200 response body.
// NewEndpoint 使用简化 handler 构建 Endpoint，只支持 200 响应返回值。
func NewEndpoint[PP, QP, HP, CP, Req, Resp any](
	name string,
	method HTTPMethod,
	path string,
	handler func(pathParams PP, queryParams QP, headerParams HP, cookieParams CP, requestBody Req, ctx *gin.Context) (Resp, error),
) Endpoint[PP, QP, HP, CP, Req, Resp] {
	return Endpoint[PP, QP, HP, CP, Req, Resp]{
		Name:   name,
		Method: method,
		Path:   path,
		HandlerFunc: func(pp PP, qp QP, hp HP, cp CP, req Req, ctx *gin.Context) (Response[Resp], error) {
			body, err := handler(pp, qp, hp, cp, req, ctx)
			return Response[Resp]{StatusCode: http.StatusOK, Body: body}, err
		},
	}
}

// NewEndpointNoBody builds an Endpoint with NoBody request and 200 response.
// NewEndpointNoBody 构建无请求体的 Endpoint，只返回 200。
func NewEndpointNoBody[PP, QP, HP, CP, Resp any](
	name string,
	method HTTPMethod,
	path string,
	handler func(pathParams PP, queryParams QP, headerParams HP, cookieParams CP, ctx *gin.Context) (Resp, error),
) Endpoint[PP, QP, HP, CP, NoBody, Resp] {
	return NewEndpoint(name, method, path, func(pp PP, qp QP, hp HP, cp CP, _ NoBody, ctx *gin.Context) (Resp, error) {
		return handler(pp, qp, hp, cp, ctx)
	})
}

// NewEndpointNoParams builds an Endpoint without params and with 200 response.
// NewEndpointNoParams 构建无参数的 Endpoint，只返回 200。
func NewEndpointNoParams[Req, Resp any](
	name string,
	method HTTPMethod,
	path string,
	handler func(requestBody Req, ctx *gin.Context) (Resp, error),
) Endpoint[NoParams, NoParams, NoParams, NoParams, Req, Resp] {
	return NewEndpoint(name, method, path, func(_ NoParams, _ NoParams, _ NoParams, _ NoParams, req Req, ctx *gin.Context) (Resp, error) {
		return handler(req, ctx)
	})
}
