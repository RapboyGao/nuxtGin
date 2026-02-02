package schema

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

func (r *tsInterfaceRegistry) ensureInterface(baseName string, value any) (string, error) {
	body, sig, err := renderTopLevelInterface(value, r)
	if err != nil {
		return "", err
	}
	if existing, ok := r.sigToName[sig]; ok {
		return existing, nil
	}

	name := sanitizeTypeName(baseName)
	if name == "" {
		name = "AnonymousType"
	}
	if count := r.nameCount[name]; count > 0 {
		name = fmt.Sprintf("%s%d", name, count+1)
	}
	r.nameCount[sanitizeTypeName(baseName)]++

	r.defs = append(r.defs, tsInterfaceDef{
		Name: name,
		Body: body,
		Sig:  sig,
	})
	r.sigToName[sig] = name
	return name, nil
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
	FuncName        string
	Method          string
	Path            string
	ParamsType      string
	RequestType     string
	ResponseType    string
	HasParams       bool
	HasPath         bool
	HasQuery        bool
	HasHeader       bool
	HasCookie       bool
	HasReqBody      bool
	RequestRequired bool
}

// generateAxiosFromSchemas converts schemas into TypeScript axios client code.
// It also generates export interfaces for Params / RequestBody / ResponseBody,
// and deduplicates identical interface shapes globally.
func generateAxiosFromSchemas(baseURL string, schemas []Schema) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]axiosFuncMeta, 0, len(schemas))

	for i, s := range schemas {
		var err error
		if err := validateSchemaForAxios(s); err != nil {
			return "", fmt.Errorf("schema[%d] validation failed: %w", i, err)
		}
		base := schemaBaseName(s, i)

		paramsShape, hasPath, hasQuery, hasHeader, hasCookie := buildParamsShape(s)
		hasParams := hasPath || hasQuery || hasHeader || hasCookie
		paramsType := ""
		if hasParams {
			paramsType, err = resolveModelType(registry, base+"Params", paramsShape)
			if err != nil {
				return "", fmt.Errorf("build params interface for schema[%d]: %w", i, err)
			}
		}

		hasReqBody := s.RequestBody != nil
		requestType := ""
		if hasReqBody {
			requestType, err = resolveModelType(registry, base+"RequestBody", s.RequestBody)
			if err != nil {
				return "", fmt.Errorf("build request interface for schema[%d]: %w", i, err)
			}
		}
		for j := range s.Responses {
			if s.Responses[j].Body == nil {
				continue
			}
			if _, err := resolveModelType(registry, fmt.Sprintf("%sResponse%dBody", base, s.Responses[j].StatusCode), s.Responses[j].Body); err != nil {
				return "", fmt.Errorf("build response[%d] interface for schema[%d]: %w", j, i, err)
			}
		}

		responseShape := inferPrimaryResponseBody(s)
		responseType := "void"
		if responseShape != nil {
			responseType, err = resolveModelType(registry, base+"ResponseBody", responseShape)
			if err != nil {
				return "", fmt.Errorf("build response interface for schema[%d]: %w", i, err)
			}
		}

		metas = append(metas, axiosFuncMeta{
			FuncName:        toLowerCamel(base),
			Method:          strings.ToUpper(string(s.Method)),
			Path:            s.Path,
			ParamsType:      paramsType,
			RequestType:     requestType,
			ResponseType:    responseType,
			HasParams:       hasParams,
			HasPath:         hasPath,
			HasQuery:        hasQuery,
			HasHeader:       hasHeader,
			HasCookie:       hasCookie,
			HasReqBody:      hasReqBody,
			RequestRequired: s.RequestRequired,
		})
	}

	var b strings.Builder
	b.WriteString("import axios from 'axios';\n\n")
	b.WriteString("export interface AxiosConvertOptions<TRequest = unknown, TResponse = unknown> {\n")
	b.WriteString("  serializeRequest?: (value: TRequest) => unknown;\n")
	b.WriteString("  deserializeResponse?: (value: unknown) => TResponse;\n")
	b.WriteString("}\n\n")

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
		b.WriteString(buildTSURLExprWithBase(baseURL, m.Path))
		b.WriteString(";\n")
		if m.HasReqBody {
			b.WriteString("  const requestData = options?.serializeRequest ? options.serializeRequest(requestBody) : requestBody;\n")
		}
		b.WriteString("  const response = await axios.request<")
		b.WriteString(m.ResponseType)
		b.WriteString(">({\n")
		b.WriteString("    method: '")
		b.WriteString(m.Method)
		b.WriteString("',\n")
		b.WriteString("    url,\n")
		if m.HasQuery {
			b.WriteString("    params: params.query,\n")
		}
		if m.HasHeader && !m.HasCookie {
			b.WriteString("    headers: params.header,\n")
		}
		if m.HasCookie {
			b.WriteString("    headers: {\n")
			if m.HasHeader {
				b.WriteString("      ...(params.header ?? {}),\n")
			}
			b.WriteString("      Cookie: buildCookieHeader((params.cookie ?? {}) as Record<string, unknown>),\n")
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

func validateSchemaForAxios(s Schema) error {
	pathParamNames := extractPathParams(s.Path)
	pathParamSet := make(map[string]struct{}, len(pathParamNames))
	for _, n := range pathParamNames {
		pathParamSet[n] = struct{}{}
	}

	for key := range s.PathParams {
		if _, ok := pathParamSet[key]; !ok {
			return fmt.Errorf("path param %q not found in path %q", key, s.Path)
		}
	}
	for key := range s.QueryParams {
		if _, ok := pathParamSet[key]; ok {
			return fmt.Errorf("query param %q conflicts with path param in path %q", key, s.Path)
		}
	}
	return nil
}

// exportAxiosFromSchemasToTSFile generates axios TypeScript code and writes it to
// a .ts file path that must be relative to the current working directory.
func exportAxiosFromSchemasToTSFile(baseURL string, schemas []Schema, relativeTSPath string) error {
	if strings.TrimSpace(relativeTSPath) == "" {
		return fmt.Errorf("relative ts path is required")
	}
	if filepath.IsAbs(relativeTSPath) {
		return fmt.Errorf("ts file path must be relative to cwd")
	}

	code, err := generateAxiosFromSchemas(baseURL, schemas)
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

func buildParamsShape(s Schema) (map[string]any, bool, bool, bool, bool) {
	params := map[string]any{}

	pathParams := cloneAnyMap(s.PathParams)
	for _, p := range extractPathParams(s.Path) {
		if _, ok := pathParams[p]; !ok {
			pathParams[p] = ""
		}
	}
	if len(pathParams) > 0 {
		params["path"] = pathParams
	}
	if len(s.QueryParams) > 0 {
		params["query"] = s.QueryParams
	}
	if len(s.HeaderParams) > 0 {
		params["header"] = s.HeaderParams
	}
	if len(s.CookieParams) > 0 {
		params["cookie"] = s.CookieParams
	}

	return params, len(pathParams) > 0, len(s.QueryParams) > 0, len(s.HeaderParams) > 0, len(s.CookieParams) > 0
}

func inferPrimaryResponseBody(s Schema) any {
	if len(s.Responses) == 0 {
		return nil
	}

	// Prefer the first 2xx response as the primary axios return type.
	for i := range s.Responses {
		code := s.Responses[i].StatusCode
		if code >= 200 && code < 300 {
			return s.Responses[i].Body
		}
	}
	return s.Responses[0].Body
}

func cloneAnyMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

func resolveModelType(registry *tsInterfaceRegistry, fallbackName string, value any) (string, error) {
	v := reflect.ValueOf(value)
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			break
		}
		v = v.Elem()
	}
	if v.IsValid() && v.Kind() == reflect.Struct && v.Type().Name() != "" && !(v.Type().PkgPath() == "time" && v.Type().Name() == "Time") {
		return registry.ensureNamedStructType(v.Type())
	}
	if !v.IsValid() {
		return registry.ensureInterface(fallbackName, map[string]any{})
	}

	// For top-level request/response array or dictionary, use direct TS types
	// (e.g. ResumeItem[] / Record<string, ResumeItem>) instead of wrapper interfaces.
	t, _, err := tsTypeFromValue(v, registry)
	if err != nil {
		return "", err
	}
	return t, nil
}

func schemaBaseName(s Schema, index int) string {
	if n := strings.TrimSpace(s.Name); n != "" {
		return toUpperCamel(n)
	}
	raw := strings.ToLower(string(s.Method)) + "_" + s.Path
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
