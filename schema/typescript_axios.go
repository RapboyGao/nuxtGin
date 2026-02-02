package schema

import (
	"fmt"
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
	defs      []tsInterfaceDef
	sigToName map[string]string
	nameCount map[string]int
}

func newTSInterfaceRegistry() *tsInterfaceRegistry {
	return &tsInterfaceRegistry{
		defs:      make([]tsInterfaceDef, 0),
		sigToName: map[string]string{},
		nameCount: map[string]int{},
	}
}

func (r *tsInterfaceRegistry) ensureInterface(baseName string, value any) (string, error) {
	body, sig, err := renderTopLevelInterface(value)
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

type axiosFuncMeta struct {
	FuncName    string
	Method      string
	Path        string
	ParamsType  string
	RequestType string
	ResponseType string
	HasPath     bool
	HasQuery    bool
	HasHeader   bool
	HasCookie   bool
	HasReqBody  bool
}

// GenerateAxiosFromSchemas converts schemas into TypeScript axios client code.
// It also generates export interfaces for Params / RequestBody / ResponseBody,
// and deduplicates identical interface shapes globally.
func GenerateAxiosFromSchemas(schemas []Schema) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]axiosFuncMeta, 0, len(schemas))

	for i, s := range schemas {
		base := schemaBaseName(s, i)

		paramsShape, hasPath, hasQuery, hasHeader, hasCookie := buildParamsShape(s)
		paramsType, err := registry.ensureInterface(base+"Params", paramsShape)
		if err != nil {
			return "", fmt.Errorf("build params interface for schema[%d]: %w", i, err)
		}

		requestShape := s.RequestBody
		if requestShape == nil {
			requestShape = map[string]any{}
		}
		requestType, err := registry.ensureInterface(base+"RequestBody", requestShape)
		if err != nil {
			return "", fmt.Errorf("build request interface for schema[%d]: %w", i, err)
		}

		responseShape := s.ResponseBody
		if responseShape == nil {
			responseShape = map[string]any{}
		}
		responseType, err := registry.ensureInterface(base+"ResponseBody", responseShape)
		if err != nil {
			return "", fmt.Errorf("build response interface for schema[%d]: %w", i, err)
		}

		metas = append(metas, axiosFuncMeta{
			FuncName:     toLowerCamel(base),
			Method:       strings.ToUpper(string(s.Method)),
			Path:         s.Path,
			ParamsType:   paramsType,
			RequestType:  requestType,
			ResponseType: responseType,
			HasPath:      hasPath,
			HasQuery:     hasQuery,
			HasHeader:    hasHeader,
			HasCookie:    hasCookie,
			HasReqBody:   s.RequestBody != nil,
		})
	}

	var b strings.Builder
	b.WriteString("import axios from 'axios';\n\n")

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
		paramsArg := "params: " + m.ParamsType + " = {}"
		reqArg := "requestBody?: " + m.RequestType
		if !m.HasPath && !m.HasQuery && !m.HasHeader && !m.HasCookie {
			paramsArg = "_params: " + m.ParamsType + " = {}"
		}
		if !m.HasReqBody {
			reqArg = "_requestBody?: " + m.RequestType
		}

		b.WriteString("export const ")
		b.WriteString(m.FuncName)
		b.WriteString(" = async (")
		b.WriteString(paramsArg)
		b.WriteString(", ")
		b.WriteString(reqArg)
		b.WriteString("): Promise<")
		b.WriteString(m.ResponseType)
		b.WriteString("> => {\n")

		b.WriteString("  const url = ")
		b.WriteString(buildTSURLExpr(m.Path))
		b.WriteString(";\n")
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
			b.WriteString("    data: requestBody,\n")
		}
		b.WriteString("  });\n")
		b.WriteString("  return response.data;\n")
		b.WriteString("};\n\n")
	}

	return strings.TrimSpace(b.String()) + "\n", nil
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
		b.WriteString(strings.ToLower(p[1:]))
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

func renderTopLevelInterface(value any) (string, string, error) {
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
		body, sig, err := renderStructBody(v)
		return body, "struct:" + sig, err
	case reflect.Map:
		body, sig, err := renderMapBody(v)
		return body, "map:" + sig, err
	default:
		t, sig, err := tsTypeFromValue(v)
		if err != nil {
			return "", "", err
		}
		return "  value: " + t + ";\n", "scalar:" + sig, nil
	}
}

func renderStructBody(v reflect.Value) (string, string, error) {
	t := v.Type()
	lines := make([]string, 0, t.NumField())
	sigs := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, ok := jsonFieldName(f)
		if !ok {
			continue
		}

		fieldType, fieldSig, err := tsTypeFromValue(v.Field(i))
		if err != nil {
			return "", "", err
		}
		lines = append(lines, fmt.Sprintf("  %s: %s;\n", tsPropName(name), fieldType))
		sigs = append(sigs, name+":"+fieldSig)
	}
	sort.Strings(sigs)
	return strings.Join(lines, ""), "{" + strings.Join(sigs, ";") + "}", nil
}

func renderMapBody(v reflect.Value) (string, string, error) {
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
		fieldType, fieldSig, err := tsTypeFromValue(keyToVal[name])
		if err != nil {
			return "", "", err
		}
		lines.WriteString("  ")
		lines.WriteString(tsPropName(name))
		lines.WriteString(": ")
		lines.WriteString(fieldType)
		lines.WriteString(";\n")
		sigs = append(sigs, name+":"+fieldSig)
	}
	return lines.String(), "{" + strings.Join(sigs, ";") + "}", nil
}

func tsTypeFromValue(v reflect.Value) (string, string, error) {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return "null", "null", nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return "unknown", "unknown", nil
	}

	t := v.Type()
	if t.PkgPath() == "time" && t.Name() == "Time" {
		return "string", "string", nil
	}

	switch v.Kind() {
	case reflect.Bool:
		return "boolean", "boolean", nil
	case reflect.String:
		return "string", "string", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number", "number", nil
	case reflect.Struct:
		body, sig, err := renderStructBody(v)
		if err != nil {
			return "", "", err
		}
		return "{\n" + body + "}", "obj" + sig, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return "Record<string, unknown>", "record_unknown", nil
		}
		if v.Len() == 0 {
			return "Record<string, unknown>", "record_empty", nil
		}
		body, sig, err := renderMapBody(v)
		if err != nil {
			return "", "", err
		}
		return "{\n" + body + "}", "map" + sig, nil
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return "unknown[]", "arr_unknown", nil
		}
		typeSet := map[string]string{}
		for i := 0; i < v.Len(); i++ {
			itemType, itemSig, err := tsTypeFromValue(v.Index(i))
			if err != nil {
				return "", "", err
			}
			typeSet[itemSig] = itemType
		}
		sigs := make([]string, 0, len(typeSet))
		for sig := range typeSet {
			sigs = append(sigs, sig)
		}
		sort.Strings(sigs)

		types := make([]string, 0, len(sigs))
		for _, sig := range sigs {
			types = append(types, typeSet[sig])
		}
		elem := strings.Join(types, " | ")
		if len(types) > 1 {
			elem = "(" + elem + ")"
		}
		return elem + "[]", "arr[" + strings.Join(sigs, "|") + "]", nil
	default:
		return "unknown", "unknown", nil
	}
}

func jsonFieldName(f reflect.StructField) (string, bool) {
	tag := f.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if tag == "" {
		return f.Name, true
	}
	name := strings.Split(tag, ",")[0]
	if name == "" {
		return f.Name, true
	}
	return name, true
}

var tsIdentifierRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func tsPropName(name string) string {
	if tsIdentifierRegexp.MatchString(name) {
		return name
	}
	return `"` + strings.ReplaceAll(name, `"`, `\"`) + `"`
}

