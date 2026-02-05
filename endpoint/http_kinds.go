package endpoint

// TSKind describes how request/response bodies should be handled in TS.
// TSKind 描述在 TS 中如何处理请求/响应体。
type TSKind string

const (
	TSKindJSON           TSKind = "json"
	TSKindMultipart      TSKind = "multipart"
	TSKindFormURLEncoded TSKind = "form_urlencoded"
	TSKindText           TSKind = "text"
	TSKindBytes          TSKind = "bytes"
	TSKindStream         TSKind = "stream"
)

// EndpointTSHints provides extra metadata for TS generation.
// EndpointTSHints 提供 TS 生成的额外元数据。
type EndpointTSHints struct {
	RequestKind  TSKind
	ResponseKind TSKind
}

// EndpointTSHintsProvider allows endpoints to customize TS generation behavior.
// EndpointTSHintsProvider 允许 endpoint 自定义 TS 生成行为。
type EndpointTSHintsProvider interface {
	EndpointTSHints() EndpointTSHints
}

// FormData is a marker type used for TS generation of multipart/form-data.
// FormData 是用于 multipart/form-data 的 TS 生成标记类型。
type FormData struct{}

// RawBytes is a marker type used for raw byte request bodies.
// RawBytes 是用于原始二进制请求体的标记类型。
type RawBytes []byte

// StreamResponse is a marker type used for streaming responses.
// StreamResponse 是用于流式响应的标记类型。
type StreamResponse struct{}
