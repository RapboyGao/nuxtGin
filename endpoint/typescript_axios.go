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

type tsInterfaceDef struct {
	Name string
	Body string
	Sig  string
}

type tsInterfaceRegistry struct {
	defs       []tsInterfaceDef
	sigToName  map[string]string
	nameCount  map[string]int
	typeToName map[reflect.Type]string
}

func newTSInterfaceRegistry() *tsInterfaceRegistry {
	return &tsInterfaceRegistry{
		defs:       make([]tsInterfaceDef, 0),
		sigToName:  map[string]string{},
		nameCount:  map[string]int{},
		typeToName: map[reflect.Type]string{},
	}
}

func (r *tsInterfaceRegistry) ensureNamedStructType(t reflect.Type) (string, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t.Name() == "" {
		return "", fmt.Errorf("type %s is not a named struct", t.String())
	}
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return "string", nil
	}
	if existing, ok := r.typeToName[t]; ok {
		return existing, nil
	}

	base := sanitizeTypeName(t.Name())
	if base == "" {
		base = "AnonymousType"
	}
	name := base
	if count := r.nameCount[base]; count > 0 {
		name = fmt.Sprintf("%s%d", base, count+1)
	}
	r.nameCount[base]++
	r.typeToName[t] = name

	body, sig, err := renderStructBodyByType(t, r)
	if err != nil {
		return "", err
	}
	namedSig := "named:" + t.PkgPath() + "." + t.Name() + ":" + sig
	if existing, ok := r.sigToName[namedSig]; ok {
		r.typeToName[t] = existing
		return existing, nil
	}

	r.defs = append(r.defs, tsInterfaceDef{
		Name: name,
		Body: body,
		Sig:  namedSig,
	})
	r.sigToName[namedSig] = name
	return name, nil
}

type axiosFuncMeta struct {
	FuncName       string
	Method         string
	Path           string
	ParamsType     string
	RequestType    string
	ResponseType   string
	APIDescription string
	RequestDesc    string
	ResponseDesc   string
	ResponseStatus int
	PathParamMap   map[string]string
	QueryParamMap  map[string]string
	HeaderParamMap map[string]string
	CookieParamMap map[string]string
	HasParams      bool
	HasPath        bool
	HasQuery       bool
	HasHeader      bool
	HasCookie      bool
	HasReqBody     bool
}

func generateAxiosFromEndpoints(baseURL string, endpoints []EndpointLike) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]axiosFuncMeta, 0, len(endpoints))

	for i, e := range endpoints {
		meta := e.EndpointMeta()
		if err := validateEndpointMeta(meta); err != nil {
			return "", fmt.Errorf("endpoint[%d] validation failed: %w", i, err)
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
		primaryResp := inferPrimaryResponseMeta(meta)
		if primaryResp != nil && primaryResp.BodyType != nil && primaryResp.BodyType.Kind() != reflect.Invalid {
			responseType, _, err = tsTypeFromType(primaryResp.BodyType, registry)
			if err != nil {
				return "", fmt.Errorf("build response type for endpoint[%d]: %w", i, err)
			}
		}

		fnMeta := axiosFuncMeta{
			FuncName:       toLowerCamel(base),
			Method:         strings.ToUpper(string(meta.Method)),
			Path:           meta.Path,
			ParamsType:     paramsType,
			RequestType:    requestType,
			ResponseType:   responseType,
			APIDescription: strings.TrimSpace(meta.Description),
			RequestDesc:    strings.TrimSpace(meta.RequestDescription),
			PathParamMap:   pathParamFieldMap(meta.PathParamsType),
			QueryParamMap:  paramFieldMap(meta.QueryParamsType),
			HeaderParamMap: paramFieldMap(meta.HeaderParamsType),
			CookieParamMap: paramFieldMap(meta.CookieParamsType),
			HasParams:      hasParams,
			HasPath:        hasPath,
			HasQuery:       hasQuery,
			HasHeader:      hasHeader,
			HasCookie:      hasCookie,
			HasReqBody:     hasReqBody,
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
	b.WriteString("import axios from 'axios';\n\n")
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
	b.WriteString("axiosClient.interceptors.request.use((config) => {\n")
	b.WriteString("  if (config.data !== undefined) config.data = normalizeRequestJSON(config.data);\n")
	b.WriteString("  if (config.params !== undefined) config.params = normalizeRequestJSON(config.params);\n")
	b.WriteString("  return config;\n")
	b.WriteString("});\n\n")
	b.WriteString("axiosClient.interceptors.response.use((response) => {\n")
	b.WriteString("  response.data = normalizeResponseJSON(response.data);\n")
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

	for _, def := range registry.defs {
		b.WriteString("export interface ")
		b.WriteString(def.Name)
		b.WriteString(" {\n")
		if def.Body != "" {
			b.WriteString(def.Body)
		}
		b.WriteString("}\n\n")
	}

	for _, m := range metas {
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
		args := make([]string, 0, 3)
		if m.HasParams {
			args = append(args, "params: "+m.ParamsType)
		}
		if m.HasReqBody {
			args = append(args, "requestBody: "+m.RequestType)
		}
		b.WriteString("export async function ")
		b.WriteString(m.FuncName)
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

		b.WriteString("  const url = ")
		b.WriteString(buildTSURLExprWithBaseAndMap(baseURL, m.Path, m.PathParamMap))
		b.WriteString(";\n")
		if m.HasReqBody {
			b.WriteString("  const requestData = options?.serializeRequest ? options.serializeRequest(requestBody) : requestBody;\n")
		}
		if m.HasQuery || m.HasHeader || m.HasCookie {
			b.WriteString("  const normalizedParams = normalizeParamKeys(params, {\n")
			if m.HasQuery {
				b.WriteString("    query: ")
				b.WriteString(renderParamMapObject(m.QueryParamMap))
				b.WriteString(",\n")
			}
			if m.HasHeader {
				b.WriteString("    header: ")
				b.WriteString(renderParamMapObject(m.HeaderParamMap))
				b.WriteString(",\n")
			}
			if m.HasCookie {
				b.WriteString("    cookie: ")
				b.WriteString(renderParamMapObject(m.CookieParamMap))
				b.WriteString(",\n")
			}
			b.WriteString("  });\n")
		}
		b.WriteString("  const response = await axiosClient.request<")
		b.WriteString(m.ResponseType)
		b.WriteString(">({\n")
		b.WriteString("    method: '")
		b.WriteString(m.Method)
		b.WriteString("',\n")
		b.WriteString("    url,\n")
		if m.HasQuery {
			b.WriteString("    params: normalizedParams.query,\n")
		}
		if m.HasHeader && !m.HasCookie {
			b.WriteString("    headers: normalizedParams.header,\n")
		}
		if m.HasCookie {
			b.WriteString("    headers: {\n")
			if m.HasHeader {
				b.WriteString("      ...(normalizedParams.header ?? {}),\n")
			}
			b.WriteString("      Cookie: buildCookieHeader((normalizedParams.cookie ?? {}) as Record<string, unknown>),\n")
			b.WriteString("    },\n")
		}
		if m.HasReqBody {
			b.WriteString("    data: requestData,\n")
		}
		b.WriteString("  });\n")
		if m.ResponseType == "void" {
			b.WriteString("  return;\n")
		} else {
			b.WriteString("  const responseData = response.data as unknown;\n")
			b.WriteString("  if (options?.deserializeResponse) {\n")
			b.WriteString("    return options.deserializeResponse(responseData);\n")
			b.WriteString("  }\n")
			b.WriteString("  return responseData as ")
			b.WriteString(m.ResponseType)
			b.WriteString(";\n")
		}
		b.WriteString("}\n\n")
	}

	return strings.TrimSpace(b.String()) + "\n", nil
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

func sanitizeTypeName(s string) string {
	s = toUpperCamel(s)
	if s == "" {
		return ""
	}
	return s
}

func toUpperCamel(s string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9]+`)
	parts := re.Split(s, -1)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if len(p) == 1 {
			b.WriteString(strings.ToUpper(p))
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "T" + out
	}
	return out
}

func toLowerCamel(s string) string {
	u := toUpperCamel(s)
	if u == "" {
		return "api"
	}
	return strings.ToLower(u[:1]) + u[1:]
}

func renderTopLevelInterface(value any, registry *tsInterfaceRegistry) (string, string, error) {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return "", "empty", nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return "", "empty", nil
	}

	switch v.Kind() {
	case reflect.Struct:
		body, sig, err := renderStructBody(v, registry)
		return body, "struct:" + sig, err
	case reflect.Map:
		body, sig, err := renderMapBody(v, registry)
		return body, "map:" + sig, err
	default:
		t, sig, err := tsTypeFromValue(v, registry)
		if err != nil {
			return "", "", err
		}
		return "  value: " + t + ";\n", "scalar:" + sig, nil
	}
}

func renderStructBody(v reflect.Value, registry *tsInterfaceRegistry) (string, string, error) {
	return renderStructBodyByType(v.Type(), registry)
}

func renderStructBodyByType(t reflect.Type, registry *tsInterfaceRegistry) (string, string, error) {
	lines := make([]string, 0, t.NumField())
	sigs := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, optional, ok := jsonFieldMeta(f)
		if !ok {
			continue
		}

		fieldType, fieldSig, err := tsTypeFromType(f.Type, registry)
		if err != nil {
			return "", "", err
		}
		separator := ";"
		if isMultilineObjectType(fieldType) {
			separator = ","
		}
		propName := tsPropName(name)
		if optional {
			propName += "?"
		}
		lines = append(lines, fmt.Sprintf("  %s: %s%s\n", propName, fieldType, separator))
		sigs = append(sigs, name+fmt.Sprintf("(%t):", optional)+fieldSig)
	}
	sort.Strings(sigs)
	return strings.Join(lines, ""), "{" + strings.Join(sigs, ";") + "}", nil
}

func renderMapBody(v reflect.Value, registry *tsInterfaceRegistry) (string, string, error) {
	if v.Type().Key().Kind() != reflect.String {
		return "  [key: string]: unknown;\n", "{[key:string]:unknown}", nil
	}
	if v.Len() == 0 {
		return "", "{}", nil
	}

	keys := v.MapKeys()
	names := make([]string, 0, len(keys))
	keyToVal := make(map[string]reflect.Value, len(keys))
	for _, k := range keys {
		name := k.String()
		names = append(names, name)
		keyToVal[name] = v.MapIndex(k)
	}
	sort.Strings(names)

	var lines strings.Builder
	sigs := make([]string, 0, len(names))
	for _, name := range names {
		fieldType, fieldSig, err := tsTypeFromValue(keyToVal[name], registry)
		if err != nil {
			return "", "", err
		}
		lines.WriteString("  ")
		lines.WriteString(tsPropName(name))
		lines.WriteString(": ")
		lines.WriteString(fieldType)
		if isMultilineObjectType(fieldType) {
			lines.WriteString(",\n")
		} else {
			lines.WriteString(";\n")
		}
		sigs = append(sigs, name+":"+fieldSig)
	}
	return lines.String(), "{" + strings.Join(sigs, ";") + "}", nil
}

func isMultilineObjectType(tsType string) bool {
	return strings.HasPrefix(tsType, "{\n") && strings.HasSuffix(tsType, "}")
}

func tsTypeFromValue(v reflect.Value, registry *tsInterfaceRegistry) (string, string, error) {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return "null", "null", nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return "unknown", "unknown", nil
	}
	if v.Kind() == reflect.Map &&
		v.Type().Key().Kind() == reflect.String &&
		v.Type().Elem().Kind() == reflect.Interface &&
		v.Len() > 0 {
		body, sig, err := renderMapBody(v, registry)
		if err != nil {
			return "", "", err
		}
		return "{\n" + body + "}", "map" + sig, nil
	}

	return tsTypeFromType(v.Type(), registry)
}

func tsTypeFromType(t reflect.Type, registry *tsInterfaceRegistry) (string, string, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.PkgPath() == "time" && t.Name() == "Time" {
		return "string", "string", nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return "boolean", "boolean", nil
	case reflect.String:
		return "string", "string", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Float32, reflect.Float64:
		return "number", "number", nil
	case reflect.Int64, reflect.Uint64:
		return "string", "int64_as_string", nil
	case reflect.Struct:
		if t.Name() != "" {
			name, err := registry.ensureNamedStructType(t)
			if err != nil {
				return "", "", err
			}
			return name, "named:" + t.PkgPath() + "." + t.Name(), nil
		}
		body, sig, err := renderStructBodyByType(t, registry)
		if err != nil {
			return "", "", err
		}
		return "{\n" + body + "}", "obj" + sig, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return "Record<string, unknown>", "record_unknown", nil
		}
		elemType, elemSig, err := tsTypeFromType(t.Elem(), registry)
		if err != nil {
			return "", "", err
		}
		return "Record<string, " + elemType + ">", "record[" + elemSig + "]", nil
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return "string", "bytes_as_base64", nil
		}
		elemType, elemSig, err := tsTypeFromType(t.Elem(), registry)
		if err != nil {
			return "", "", err
		}
		return elemType + "[]", "arr[" + elemSig + "]", nil
	case reflect.Interface:
		return "unknown", "unknown", nil
	default:
		return "unknown", "unknown", nil
	}
}

func jsonFieldMeta(f reflect.StructField) (string, bool, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false, false
	}
	optional := strings.Contains(tag, ",omitempty")
	if tag == "" {
		return f.Name, optional, true
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return f.Name, optional, true
	}
	return name, optional, true
}

var tsIdentifierRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func tsPropName(name string) string {
	if tsIdentifierRegexp.MatchString(name) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
}

func escapeTSComment(s string) string {
	return strings.ReplaceAll(s, "*/", "* /")
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

func buildTSURLExpr(path string) string {
	template := pathParamRegexp.ReplaceAllStringFunc(path, func(seg string) string {
		name := strings.Trim(seg, ":{}")
		return "${encodeURIComponent(String(params.path?." + name + " ?? ''))}"
	})
	return "`" + template + "`"
}

func buildTSURLExprWithBase(baseURL string, path string) string {
	fullPath := joinURLPath(baseURL, path)
	return buildTSURLExpr(fullPath)
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

func joinURLPath(baseURL string, path string) string {
	base := strings.TrimSpace(baseURL)
	p := strings.TrimSpace(path)

	if base == "" {
		if strings.HasPrefix(p, "/") {
			return p
		}
		return "/" + p
	}
	if p == "" {
		if strings.HasPrefix(base, "/") {
			return strings.TrimRight(base, "/")
		}
		return "/" + strings.TrimRight(base, "/")
	}

	base = strings.TrimRight(base, "/")
	p = strings.TrimLeft(p, "/")
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return base + "/" + p
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
		name, _, ok := jsonFieldMeta(f)
		if !ok {
			continue
		}
		if name == "" {
			name = f.Name
		}
		out[strings.ToLower(name)] = name
		// also map the raw field name for safety
		out[strings.ToLower(f.Name)] = f.Name
	}
	return out
}

func paramFieldMap(t reflect.Type) map[string]string {
	return pathParamFieldMap(t)
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
