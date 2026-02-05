package endpoint

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CustomEndpoint defines an API endpoint with a customizable body binder.
// CustomEndpoint 定义一个可自定义请求体绑定方式的 API。
type CustomEndpoint[PP, QP, HP, CP, Req, Resp any] struct {
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
	RequestKind        TSKind
	ResponseKind       TSKind
	HandlerFunc        gin.HandlerFunc
}

// EndpointMeta exposes metadata for TS generation.
// EndpointMeta 暴露 TS 生成所需的元数据。
func (s CustomEndpoint[PP, QP, HP, CP, Req, Resp]) EndpointMeta() EndpointMeta {
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

// EndpointTSHints customizes TS generation.
// EndpointTSHints 自定义 TS 生成。
func (s CustomEndpoint[PP, QP, HP, CP, Req, Resp]) EndpointTSHints() EndpointTSHints {
	return EndpointTSHints{
		RequestKind:  s.RequestKind,
		ResponseKind: s.ResponseKind,
	}
}

// GinHandler builds a gin.HandlerFunc that binds params/body and calls HandlerFunc.
// GinHandler 会绑定参数/请求体并调用 HandlerFunc。
func (s CustomEndpoint[PP, QP, HP, CP, Req, Resp]) GinHandler() gin.HandlerFunc {
	if s.HandlerFunc == nil {
		return func(ctx *gin.Context) {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "custom endpoint handler is nil"})
		}
	}
	return s.HandlerFunc
}

// NewCustomEndpoint builds a CustomEndpoint with common fields.
// NewCustomEndpoint 构建 CustomEndpoint，便于快速初始化。
func NewCustomEndpoint[PP, QP, HP, CP, Req, Resp any](
	name string,
	method HTTPMethod,
	path string,
	handler gin.HandlerFunc,
) CustomEndpoint[PP, QP, HP, CP, Req, Resp] {
	return CustomEndpoint[PP, QP, HP, CP, Req, Resp]{
		Name:        name,
		Method:      method,
		Path:        path,
		HandlerFunc: handler,
	}
}
