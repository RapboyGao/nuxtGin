package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type axiosFuncMeta struct {
	FuncName         string
	Method           string
	Path             string
	ParamsType       string
	RequestType      string
	ResponseType     string
	ResponseWireType string
	APIDescription   string
	RequestDesc      string
	ResponseDesc     string
	ResponseStatus   int
	PathParamMap     map[string]string
	QueryParamMap    map[string]string
	HeaderParamMap   map[string]string
	CookieParamMap   map[string]string
	HasParams        bool
	HasPath          bool
	HasQuery         bool
	HasHeader        bool
	HasCookie        bool
	HasReqBody       bool
	RequestKind      TSKind
	ResponseKind     TSKind
}

func generateAxiosFromEndpoints(baseURL string, endpoints []EndpointLike) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]axiosFuncMeta, 0, len(endpoints))

	for i, e := range endpoints {
		meta := e.EndpointMeta()
		if err := validateEndpointMeta(meta); err != nil {
			return "", fmt.Errorf("endpoint[%d] validation failed: %w", i, err)
		}

		requestKind := TSKindJSON
		responseKind := TSKindJSON
		if hintProvider, ok := e.(EndpointTSHintsProvider); ok {
			hints := hintProvider.EndpointTSHints()
			if hints.RequestKind != "" {
				requestKind = hints.RequestKind
			}
			if hints.ResponseKind != "" {
				responseKind = hints.ResponseKind
			}
		}

		base := schemaBaseName(meta, i)

		paramsType, hasPath, hasQuery, hasHeader, hasCookie, err := buildParamsTypeFromTypes(registry, meta.PathParamsType, meta.QueryParamsType, meta.HeaderParamsType, meta.CookieParamsType)
		if err != nil {
			return "", fmt.Errorf("build params type for endpoint[%d]: %w", i, err)
		}
		hasParams := hasPath || hasQuery || hasHeader || hasCookie

		requestType := ""
		hasReqBody := meta.RequestBodyType != nil && meta.RequestBodyType.Kind() != reflect.Invalid && !isNoType(meta.RequestBodyType)
		if hasReqBody {
			requestType, _, err = tsTypeFromType(meta.RequestBodyType, registry)
			if err != nil {
				return "", fmt.Errorf("build request type for endpoint[%d]: %w", i, err)
			}
		}

		for j := range meta.Responses {
			if meta.Responses[j].BodyType == nil || meta.Responses[j].BodyType.Kind() == reflect.Invalid {
				continue
			}
			if _, _, err := tsTypeFromType(meta.Responses[j].BodyType, registry); err != nil {
				return "", fmt.Errorf("build response[%d] type for endpoint[%d]: %w", j, i, err)
			}
		}

		responseType := "void"
		responseWireType := "void"
		primaryResp := inferPrimaryResponseMeta(meta)
		if primaryResp != nil && primaryResp.BodyType != nil && primaryResp.BodyType.Kind() != reflect.Invalid {
			responseType, _, err = tsTypeFromType(primaryResp.BodyType, registry)
			if err != nil {
				return "", fmt.Errorf("build response type for endpoint[%d]: %w", i, err)
			}
			responseWireType = responseType
		}
		switch responseKind {
		case TSKindStream:
			responseType = "Blob"
			responseWireType = "Blob"
		case TSKindText:
			responseType = "string"
			responseWireType = "string"
		case TSKindBytes:
			responseType = "Uint8Array"
			responseWireType = "ArrayBuffer"
		}

		fnMeta := axiosFuncMeta{
			FuncName:         toLowerCamel(base),
			Method:           strings.ToUpper(string(meta.Method)),
			Path:             meta.Path,
			ParamsType:       paramsType,
			RequestType:      requestType,
			ResponseType:     responseType,
			ResponseWireType: responseWireType,
			APIDescription:   strings.TrimSpace(meta.Description),
			RequestDesc:      strings.TrimSpace(meta.RequestDescription),
			PathParamMap:     pathParamFieldMap(meta.PathParamsType),
			QueryParamMap:    queryParamFieldMap(meta.QueryParamsType),
			HeaderParamMap:   headerParamFieldMap(meta.HeaderParamsType),
			CookieParamMap:   cookieParamFieldMap(meta.CookieParamsType),
			HasParams:        hasParams,
			HasPath:          hasPath,
			HasQuery:         hasQuery,
			HasHeader:        hasHeader,
			HasCookie:        hasCookie,
			HasReqBody:       hasReqBody,
			RequestKind:      requestKind,
			ResponseKind:     responseKind,
		}
		if primaryResp != nil {
			fnMeta.ResponseDesc = strings.TrimSpace(primaryResp.Description)
			fnMeta.ResponseStatus = primaryResp.StatusCode
		}
		metas = append(metas, fnMeta)
	}

	return renderAxiosTS(baseURL, registry, metas)
}

func exportAxiosFromEndpointsToTSFile(baseURL string, endpoints []EndpointLike, relativeTSPath string) error {
	if strings.TrimSpace(relativeTSPath) == "" {
		return fmt.Errorf("relative ts path is required")
	}
	if filepath.IsAbs(relativeTSPath) {
		return fmt.Errorf("ts file path must be relative to cwd")
	}

	code, err := generateAxiosFromEndpoints(baseURL, endpoints)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	fullPath := filepath.Clean(filepath.Join(cwd, relativeTSPath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(code), 0o644)
}

func renderAxiosTS(baseURL string, registry *tsInterfaceRegistry, metas []axiosFuncMeta) (string, error) {
	var b strings.Builder
	writeTSBanner(&b, "Nuxt Gin HTTP API Client (Axios)")
	b.WriteString("import axios, { type AxiosRequestConfig } from 'axios';\n\n")
	b.WriteString("const axiosClient = axios.create();\n\n")
	b.WriteString("const isPlainObject = (value: unknown): value is Record<string, unknown> =>\n")
	b.WriteString("  Object.prototype.toString.call(value) === '[object Object]';\n\n")
	b.WriteString("const isoDateLike = /^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}(?:\\.\\d{1,9})?(?:Z|[+\\-]\\d{2}:\\d{2})$/;\n\n")
	b.WriteString("const normalizeRequestJSON = (value: unknown): unknown => {\n")
	b.WriteString("  if (value instanceof Date) return value.toISOString();\n")
	b.WriteString("  if (Array.isArray(value)) return value.map(normalizeRequestJSON);\n")
	b.WriteString("  if (isPlainObject(value)) {\n")
	b.WriteString("    const out: Record<string, unknown> = {};\n")
	b.WriteString("    for (const [k, v] of Object.entries(value)) out[k] = normalizeRequestJSON(v);\n")
	b.WriteString("    return out;\n")
	b.WriteString("  }\n")
	b.WriteString("  return value;\n")
	b.WriteString("};\n\n")
	b.WriteString("const normalizeResponseJSON = (value: unknown): unknown => {\n")
	b.WriteString("  if (Array.isArray(value)) return value.map(normalizeResponseJSON);\n")
	b.WriteString("  if (typeof value === 'string' && isoDateLike.test(value)) {\n")
	b.WriteString("    const date = new Date(value);\n")
	b.WriteString("    if (!Number.isNaN(date.getTime())) return date;\n")
	b.WriteString("  }\n")
	b.WriteString("  if (isPlainObject(value)) {\n")
	b.WriteString("    const out: Record<string, unknown> = {};\n")
	b.WriteString("    for (const [k, v] of Object.entries(value)) out[k] = normalizeResponseJSON(v);\n")
	b.WriteString("    return out;\n")
	b.WriteString("  }\n")
	b.WriteString("  return value;\n")
	b.WriteString("};\n\n")
	b.WriteString("const toFormUrlEncoded = (value: unknown): URLSearchParams => {\n")
	b.WriteString("  if (value instanceof URLSearchParams) return value;\n")
	b.WriteString("  const params = new URLSearchParams();\n")
	b.WriteString("  if (!isPlainObject(value)) return params;\n")
	b.WriteString("  for (const [k, v] of Object.entries(value)) {\n")
	b.WriteString("    if (v === undefined || v === null) continue;\n")
	b.WriteString("    if (Array.isArray(v)) {\n")
	b.WriteString("      for (const item of v) params.append(k, String(item));\n")
	b.WriteString("      continue;\n")
	b.WriteString("    }\n")
	b.WriteString("    params.append(k, String(v));\n")
	b.WriteString("  }\n")
	b.WriteString("  return params;\n")
	b.WriteString("};\n\n")
	b.WriteString("axiosClient.interceptors.request.use((config) => {\n")
	b.WriteString("  if (config.data !== undefined) config.data = normalizeRequestJSON(config.data);\n")
	b.WriteString("  if (config.params !== undefined) config.params = normalizeRequestJSON(config.params);\n")
	b.WriteString("  return config;\n")
	b.WriteString("});\n\n")
	b.WriteString("axiosClient.interceptors.response.use((response) => {\n")
	b.WriteString("  const rt = response.config?.responseType;\n")
	b.WriteString("  if (rt !== 'arraybuffer' && rt !== 'blob' && rt !== 'text') {\n")
	b.WriteString("    response.data = normalizeResponseJSON(response.data);\n")
	b.WriteString("  }\n")
	b.WriteString("  return response;\n")
	b.WriteString("});\n\n")
	b.WriteString("export interface AxiosConvertOptions<TRequest = unknown, TResponse = unknown> {\n")
	b.WriteString("  serializeRequest?: (value: TRequest) => unknown;\n")
	b.WriteString("  deserializeResponse?: (value: unknown) => TResponse;\n")
	b.WriteString("}\n\n")
	b.WriteString("const normalizeParamKeys = (\n")
	b.WriteString("  params: Record<string, any>,\n")
	b.WriteString("  maps: { query?: Record<string, string>; header?: Record<string, string>; cookie?: Record<string, string> }\n")
	b.WriteString(") => {\n")
	b.WriteString("  const out: Record<string, any> = {};\n")
	b.WriteString("  for (const key of ['query', 'header', 'cookie']) {\n")
	b.WriteString("    const group = (params as any)?.[key] ?? {};\n")
	b.WriteString("    const map = (maps as any)?.[key] ?? {};\n")
	b.WriteString("    const normalized: Record<string, any> = {};\n")
	b.WriteString("    for (const [k, v] of Object.entries(group)) {\n")
	b.WriteString("      const mapped = map[k.toLowerCase()] ?? k;\n")
	b.WriteString("      normalized[mapped] = v;\n")
	b.WriteString("    }\n")
	b.WriteString("    out[key] = normalized;\n")
	b.WriteString("  }\n")
	b.WriteString("  return out;\n")
	b.WriteString("};\n\n")

	needsCookieHelper := false
	for _, m := range metas {
		if m.HasCookie {
			needsCookieHelper = true
			break
		}
	}
	if needsCookieHelper {
		b.WriteString("const buildCookieHeader = (cookie: Record<string, unknown>): string =>\n")
		b.WriteString("  Object.entries(cookie)\n")
		b.WriteString("    .map(([k, v]) => `${k}=${encodeURIComponent(String(v))}`)\n")
		b.WriteString("    .join('; ');\n\n")
	}

	for _, m := range metas {
		className := toUpperCamel(m.FuncName) + toUpperCamel(strings.ToLower(m.Method))
		fullPath := joinURLPath(baseURL, m.Path)
		hasPathPlaceholders := len(extractPathParams(m.Path)) > 0
		pathParamNames := extractPathParams(m.Path)
		mappedPathParamNames := make([]string, 0, len(pathParamNames))
		for _, raw := range pathParamNames {
			key := strings.ToLower(raw)
			if mapped, ok := m.PathParamMap[key]; ok && strings.TrimSpace(mapped) != "" {
				mappedPathParamNames = append(mappedPathParamNames, mapped)
				continue
			}
			mappedPathParamNames = append(mappedPathParamNames, raw)
		}
		if m.APIDescription != "" || m.RequestDesc != "" || m.ResponseDesc != "" {
			b.WriteString("/**\n")
			if m.APIDescription != "" {
				b.WriteString(" * ")
				b.WriteString(escapeTSComment(m.APIDescription))
				b.WriteString("\n")
			}
			if m.RequestDesc != "" {
				b.WriteString(" * @request ")
				b.WriteString(escapeTSComment(m.RequestDesc))
				b.WriteString("\n")
			}
			if m.ResponseDesc != "" {
				b.WriteString(" * @response")
				if m.ResponseStatus > 0 {
					b.WriteString(" ")
					b.WriteString(fmt.Sprintf("%d", m.ResponseStatus))
				}
				b.WriteString(" ")
				b.WriteString(escapeTSComment(m.ResponseDesc))
				b.WriteString("\n")
			}
			b.WriteString(" */\n")
		}
		b.WriteString("export class ")
		b.WriteString(className)
		b.WriteString(" {\n")
		b.WriteString("  static readonly NAME = '")
		b.WriteString(strings.ReplaceAll(m.FuncName, "'", "\\'"))
		b.WriteString("' as const;\n")
		b.WriteString("  static readonly SUMMARY = '")
		b.WriteString(strings.ReplaceAll(escapeTSComment(m.APIDescription), "'", "\\'"))
		b.WriteString("' as const;\n")
		b.WriteString("  static readonly METHOD = '")
		b.WriteString(m.Method)
		b.WriteString("' as const;\n")
		b.WriteString("  static readonly PATH = '")
		b.WriteString(strings.ReplaceAll(fullPath, "'", "\\'"))
		b.WriteString("' as const;\n\n")
		args := make([]string, 0, 3)
		if m.HasParams {
			args = append(args, "params: "+m.ParamsType)
		}
		if m.HasReqBody {
			args = append(args, "requestBody: "+m.RequestType)
		}
		b.WriteString("  static pathParamsShape(): readonly string[] {\n")
		if len(mappedPathParamNames) == 0 {
			b.WriteString("    return [] as const;\n")
		} else {
			b.WriteString("    return [")
			for i, name := range mappedPathParamNames {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString("'")
				b.WriteString(strings.ReplaceAll(name, "'", "\\'"))
				b.WriteString("'")
			}
			b.WriteString("] as const;\n")
		}
		b.WriteString("  }\n\n")
		b.WriteString("  static buildURL")
		if hasPathPlaceholders {
			b.WriteString("(params: ")
			b.WriteString(m.ParamsType)
			b.WriteString("): string {\n")
			b.WriteString("    return ")
			b.WriteString(buildTSURLExprWithBaseAndMap(baseURL, m.Path, m.PathParamMap))
			b.WriteString(";\n")
		} else {
			b.WriteString("(): string {\n")
			b.WriteString("    return ")
			b.WriteString(className)
			b.WriteString(".PATH;\n")
		}
		b.WriteString("  }\n\n")
		requestConfigArgs := make([]string, 0, 3)
		requestConfigArgs = append(requestConfigArgs, args...)
		if m.HasReqBody {
			requestConfigArgs = append(requestConfigArgs, "options?: AxiosConvertOptions<"+m.RequestType+", "+m.ResponseType+">")
		}
		b.WriteString("  static requestConfig")
		b.WriteString("(")
		b.WriteString(strings.Join(requestConfigArgs, ", "))
		b.WriteString("): AxiosRequestConfig {\n")
		if hasPathPlaceholders {
			b.WriteString("    const url = ")
			b.WriteString(className)
			b.WriteString(".buildURL(params);\n")
		} else {
			b.WriteString("    const url = ")
			b.WriteString(className)
			b.WriteString(".buildURL();\n")
		}
		if m.HasReqBody {
			if m.RequestKind == TSKindFormURLEncoded {
				b.WriteString("    const serializedRequest = options?.serializeRequest ? options.serializeRequest(requestBody) : requestBody;\n")
				b.WriteString("    const requestData = toFormUrlEncoded(serializedRequest);\n")
			} else {
				b.WriteString("    const requestData = options?.serializeRequest ? options.serializeRequest(requestBody) : requestBody;\n")
			}
		}
		needsNormalizedParams := m.HasQuery || m.HasHeader || m.HasCookie
		if needsNormalizedParams {
			b.WriteString("    const normalizedParams = normalizeParamKeys(params, {\n")
			if m.HasQuery {
				b.WriteString("      query: ")
				b.WriteString(renderParamMapObject(m.QueryParamMap))
				b.WriteString(",\n")
			}
			if m.HasHeader {
				b.WriteString("      header: ")
				b.WriteString(renderParamMapObject(m.HeaderParamMap))
				b.WriteString(",\n")
			}
			if m.HasCookie {
				b.WriteString("      cookie: ")
				b.WriteString(renderParamMapObject(m.CookieParamMap))
				b.WriteString(",\n")
			}
			b.WriteString("    });\n")
		}
		requestHeaderValue := ""
		switch m.RequestKind {
		case TSKindFormURLEncoded:
			requestHeaderValue = "application/x-www-form-urlencoded"
		case TSKindText:
			requestHeaderValue = "text/plain; charset=utf-8"
		case TSKindBytes:
			requestHeaderValue = "application/octet-stream"
		}
		needsHeaders := m.HasHeader || m.HasCookie || requestHeaderValue != ""
		if requestHeaderValue != "" {
			b.WriteString("    const requestHeaders = { 'Content-Type': '")
			b.WriteString(requestHeaderValue)
			b.WriteString("' };\n")
		}
		if needsHeaders {
			b.WriteString("    const headers = {\n")
			if m.HasHeader {
				b.WriteString("      ...(normalizedParams?.header ?? {}),\n")
			}
			if requestHeaderValue != "" {
				b.WriteString("      ...requestHeaders,\n")
			}
			if m.HasCookie {
				b.WriteString("      Cookie: buildCookieHeader((normalizedParams?.cookie ?? {}) as Record<string, unknown>),\n")
			}
			b.WriteString("    };\n")
		}
		b.WriteString("    return {\n")
		b.WriteString("      method: ")
		b.WriteString(className)
		b.WriteString(".METHOD,\n")
		b.WriteString("      url,\n")
		if m.HasQuery {
			b.WriteString("      params: normalizedParams.query,\n")
		}
		if needsHeaders {
			b.WriteString("      headers,\n")
		}
		switch m.ResponseKind {
		case TSKindStream:
			b.WriteString("      responseType: 'blob',\n")
		case TSKindBytes:
			b.WriteString("      responseType: 'arraybuffer',\n")
		case TSKindText:
			b.WriteString("      responseType: 'text',\n")
		}
		if m.HasReqBody {
			b.WriteString("      data: requestData,\n")
		}
		b.WriteString("    };\n")
		b.WriteString("  }\n\n")
		b.WriteString("  static async request")
		b.WriteString("(")
		b.WriteString(strings.Join(args, ", "))
		if len(args) > 0 {
			b.WriteString(", ")
		}
		b.WriteString("options?: AxiosConvertOptions<")
		if m.HasReqBody {
			b.WriteString(m.RequestType)
		} else {
			b.WriteString("never")
		}
		b.WriteString(", ")
		b.WriteString(m.ResponseType)
		b.WriteString(">")
		b.WriteString("): Promise<")
		b.WriteString(m.ResponseType)
		b.WriteString("> {\n")
		callArgs := make([]string, 0, 3)
		if m.HasParams {
			callArgs = append(callArgs, "params")
		}
		if m.HasReqBody {
			callArgs = append(callArgs, "requestBody")
			callArgs = append(callArgs, "options")
		}
		b.WriteString("    const response = await axiosClient.request<")
		b.WriteString(m.ResponseWireType)
		b.WriteString(">(")
		b.WriteString(className)
		b.WriteString(".requestConfig(")
		b.WriteString(strings.Join(callArgs, ", "))
		b.WriteString("));\n")
		if m.ResponseType == "void" {
			b.WriteString("    return;\n")
		} else {
			if m.ResponseKind == TSKindBytes {
				b.WriteString("    const responseData = new Uint8Array(response.data as ArrayBuffer);\n")
				b.WriteString("    if (options?.deserializeResponse) {\n")
				b.WriteString("      return options.deserializeResponse(responseData);\n")
				b.WriteString("    }\n")
				b.WriteString("    return responseData;\n")
			} else {
				b.WriteString("    const responseData = response.data as unknown;\n")
				b.WriteString("    if (options?.deserializeResponse) {\n")
				b.WriteString("      return options.deserializeResponse(responseData);\n")
				b.WriteString("    }\n")
				b.WriteString("    return responseData as ")
				b.WriteString(m.ResponseType)
				b.WriteString(";\n")
			}
		}
		b.WriteString("  }\n")
		b.WriteString("}\n\n")
	}

	if len(registry.defs) > 0 {
		b.WriteString("// =====================================================\n")
		b.WriteString("// INTERFACES & VALIDATORS\n")
		b.WriteString("// Default: object schemas use interface.\n")
		b.WriteString("// Fallback: use type only when interface cannot model the shape.\n")
		b.WriteString("// 默认：对象结构使用 interface。\n")
		b.WriteString("// 兜底：只有 interface 无法表达时才使用 type。\n")
		b.WriteString("// =====================================================\n\n")
	}
	for _, def := range registry.defs {
		b.WriteString("// -----------------------------------------------------\n")
		b.WriteString("// TYPE: ")
		b.WriteString(def.Name)
		b.WriteString("\n")
		b.WriteString("// -----------------------------------------------------\n")
		b.WriteString("export interface ")
		b.WriteString(def.Name)
		b.WriteString(" {\n")
		if def.Body != "" {
			b.WriteString(def.Body)
		}
		b.WriteString("}\n\n")
		if strings.TrimSpace(def.Validator) != "" {
			b.WriteString(def.Validator)
			b.WriteString("\n")
		}
	}

	return finalizeTypeScriptCode(b.String()), nil
}

func validateEndpointMeta(meta EndpointMeta) error {
	if strings.TrimSpace(string(meta.Method)) == "" {
		return fmt.Errorf("method is required")
	}
	if !meta.Method.IsValid() {
		return fmt.Errorf("invalid http method")
	}
	if strings.TrimSpace(meta.Path) == "" {
		return fmt.Errorf("path is required")
	}
	pathParams := extractPathParams(meta.Path)
	if len(pathParams) > 0 && isNoType(meta.PathParamsType) {
		return fmt.Errorf("path params required but PathParams type is NoParams")
	}
	return nil
}

func buildParamsTypeFromTypes(registry *tsInterfaceRegistry, pathType, queryType, headerType, cookieType reflect.Type) (string, bool, bool, bool, bool, error) {
	hasPath := isValidType(pathType)
	hasQuery := isValidType(queryType)
	hasHeader := isValidType(headerType)
	hasCookie := isValidType(cookieType)

	fields := make(map[string]string, 4)
	if hasPath {
		t, _, err := tsTypeFromType(pathType, registry)
		if err != nil {
			return "", false, false, false, false, err
		}
		fields["path"] = t
	}
	if hasQuery {
		t, _, err := tsTypeFromType(queryType, registry)
		if err != nil {
			return "", false, false, false, false, err
		}
		fields["query"] = t
	}
	if hasHeader {
		t, _, err := tsTypeFromType(headerType, registry)
		if err != nil {
			return "", false, false, false, false, err
		}
		fields["header"] = t
	}
	if hasCookie {
		t, _, err := tsTypeFromType(cookieType, registry)
		if err != nil {
			return "", false, false, false, false, err
		}
		fields["cookie"] = t
	}

	if len(fields) == 0 {
		return "", false, false, false, false, nil
	}
	return buildInlineObjectType(fields), hasPath, hasQuery, hasHeader, hasCookie, nil
}

func buildInlineObjectType(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{\n")
	for _, k := range keys {
		b.WriteString("  ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(fields[k])
		if isMultilineObjectType(fields[k]) {
			b.WriteString(",\n")
		} else {
			b.WriteString(";\n")
		}
	}
	b.WriteString("}")
	return b.String()
}

func isValidType(t reflect.Type) bool {
	return t != nil && t.Kind() != reflect.Invalid && !isNoType(t)
}

func inferPrimaryResponseMeta(meta EndpointMeta) *ResponseMeta {
	if len(meta.Responses) == 0 {
		return nil
	}
	for i := range meta.Responses {
		code := meta.Responses[i].StatusCode
		if code >= 200 && code < 300 {
			return &meta.Responses[i]
		}
	}
	return &meta.Responses[0]
}

func schemaBaseName(meta EndpointMeta, index int) string {
	if n := strings.TrimSpace(meta.Name); n != "" {
		return toUpperCamel(n)
	}
	raw := string(meta.Method) + "_" + meta.Path
	raw = strings.ReplaceAll(raw, "{", " ")
	raw = strings.ReplaceAll(raw, "}", " ")
	raw = strings.ReplaceAll(raw, ":", " by ")
	raw = strings.ReplaceAll(raw, "/", " ")
	base := toUpperCamel(raw)
	if base == "" {
		return fmt.Sprintf("Api%d", index+1)
	}
	return base
}

var pathParamRegexp = regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)|\{([A-Za-z_][A-Za-z0-9_]*)\}`)

func extractPathParams(path string) []string {
	matches := pathParamRegexp.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func buildTSURLExprWithBaseAndMap(baseURL string, path string, fieldMap map[string]string) string {
	fullPath := joinURLPath(baseURL, path)
	template := pathParamRegexp.ReplaceAllStringFunc(fullPath, func(seg string) string {
		raw := strings.Trim(seg, ":{}")
		key := strings.ToLower(raw)
		if mapped, ok := fieldMap[key]; ok && mapped != "" {
			return "${encodeURIComponent(String(params.path?." + mapped + " ?? ''))}"
		}
		return "${encodeURIComponent(String(params.path?." + raw + " ?? ''))}"
	})
	return "`" + template + "`"
}

func pathParamFieldMap(t reflect.Type) map[string]string {
	out := map[string]string{}
	if t == nil || t.Kind() == reflect.Invalid {
		return out
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		externalName, ok := resolveParamFieldName(f, "uri")
		if !ok {
			continue
		}
		if externalName == "" {
			externalName = f.Name
		}
		tsFieldName, _, tsOK := jsonFieldMeta(f)
		if !tsOK {
			continue
		}
		if tsFieldName == "" {
			tsFieldName = f.Name
		}
		out[strings.ToLower(externalName)] = tsFieldName
		rawKey := strings.ToLower(f.Name)
		if _, exists := out[rawKey]; !exists {
			out[rawKey] = tsFieldName
		}
	}
	return out
}

func queryParamFieldMap(t reflect.Type) map[string]string {
	return paramFieldMapWithPrimaryTag(t, "form")
}

func headerParamFieldMap(t reflect.Type) map[string]string {
	return paramFieldMapWithPrimaryTag(t, "header")
}

func cookieParamFieldMap(t reflect.Type) map[string]string {
	return paramFieldMapWithPrimaryTag(t, "cookie")
}

func paramFieldMapWithPrimaryTag(t reflect.Type, primaryTag string) map[string]string {
	out := map[string]string{}
	if t == nil || t.Kind() == reflect.Invalid {
		return out
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return out
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, ok := resolveParamFieldName(f, primaryTag)
		if !ok {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[strings.ToLower(name)] = name
		// keep json tag name as priority; only fallback to raw field name when missing
		rawKey := strings.ToLower(f.Name)
		if _, exists := out[rawKey]; !exists {
			out[rawKey] = f.Name
		}
	}
	return out
}

func paramFieldMap(t reflect.Type) map[string]string {
	return paramFieldMapWithPrimaryTag(t, "")
}

func resolveParamFieldName(f reflect.StructField, primaryTag string) (string, bool) {
	if primaryTag != "" {
		if name, ok, ignored := nameFromStructTag(f, primaryTag); ignored {
			return "", false
		} else if ok {
			return name, true
		}
	}

	if name, _, ok := jsonFieldMeta(f); ok {
		return name, true
	}

	return f.Name, true
}

func nameFromStructTag(f reflect.StructField, tagKey string) (name string, found bool, ignored bool) {
	if tagKey == "" {
		return "", false, false
	}
	raw := f.Tag.Get(tagKey)
	if raw == "" {
		return "", false, false
	}
	if raw == "-" {
		return "", true, true
	}
	name = strings.Split(raw, ",")[0]
	if name == "" {
		name = f.Name
	}
	return name, true, false
}

func renderParamMapObject(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("'")
		b.WriteString(strings.ReplaceAll(k, "'", "\\'"))
		b.WriteString("': '")
		b.WriteString(strings.ReplaceAll(m[k], "'", "\\'"))
		b.WriteString("'")
	}
	b.WriteString("}")
	return b.String()
}
