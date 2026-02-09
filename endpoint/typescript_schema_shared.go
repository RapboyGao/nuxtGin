package endpoint

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type tsInterfaceDef struct {
	Name      string
	Body      string
	Validator string
	Sig       string
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
	validator, err := renderStructValidatorByType(t, r, name)
	if err != nil {
		return "", err
	}
	namedSig := "named:" + t.PkgPath() + "." + t.Name() + ":" + sig
	if existing, ok := r.sigToName[namedSig]; ok {
		r.typeToName[t] = existing
		return existing, nil
	}

	r.defs = append(r.defs, tsInterfaceDef{
		Name:      name,
		Body:      body,
		Validator: validator,
		Sig:       namedSig,
	})
	r.sigToName[namedSig] = name
	return name, nil
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
		if unionValues, ok, err := tsUnionValuesFromField(f); err != nil {
			return "", "", err
		} else if ok {
			fieldType = tsUnionType(unionValues)
			fieldSig = "union[" + tsUnionSig(unionValues) + "]"
		}
		separator := ";"
		if isMultilineObjectType(fieldType) {
			separator = ","
		}
		propName := tsPropName(name)
		if optional {
			propName += "?"
		}
		if tsdoc := strings.TrimSpace(f.Tag.Get("tsdoc")); tsdoc != "" {
			lines = append(lines, renderTSFieldComment(tsdoc))
		}
		lines = append(lines, fmt.Sprintf("  %s: %s%s\n", propName, fieldType, separator))
		sigs = append(sigs, name+fmt.Sprintf("(%t):", optional)+fieldSig)
	}
	sort.Strings(sigs)
	return strings.Join(lines, ""), "{"+strings.Join(sigs, ";")+"}", nil
}

func renderStructValidatorByType(t reflect.Type, registry *tsInterfaceRegistry, interfaceName string) (string, error) {
	var b strings.Builder
	b.WriteString("/**\n")
	b.WriteString(" * Validate whether a value matches ")
	b.WriteString(interfaceName)
	b.WriteString(".\n")
	b.WriteString(" * 校验一个值是否符合 ")
	b.WriteString(interfaceName)
	b.WriteString(" 结构。\n")
	b.WriteString(" */\n")
	b.WriteString("export function validate")
	b.WriteString(interfaceName)
	b.WriteString("(value: unknown): value is ")
	b.WriteString(interfaceName)
	b.WriteString(" {\n")
	b.WriteString("  if (!isPlainObject(value)) return false;\n")
	b.WriteString("  const obj = value as Record<string, unknown>;\n")

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, optional, ok := jsonFieldMeta(f)
		if !ok {
			continue
		}
		valueExpr := "obj[" + strconv.Quote(name) + "]"
		expr, err := tsValidatorExprFromType(f.Type, valueExpr, registry, 0)
		if err != nil {
			return "", err
		}
		if unionValues, ok, err := tsUnionValuesFromField(f); err != nil {
			return "", err
		} else if ok {
			expr = tsUnionValidatorExpr(valueExpr, unionValues)
		}
		if optional {
			b.WriteString("  if (obj[")
			b.WriteString(strconv.Quote(name))
			b.WriteString("] !== undefined && !(")
			b.WriteString(expr)
			b.WriteString(")) return false;\n")
			continue
		}
		b.WriteString("  if (!( ")
		b.WriteString(strconv.Quote(name))
		b.WriteString(" in obj)) return false;\n")
		b.WriteString("  if (!(")
		b.WriteString(expr)
		b.WriteString(")) return false;\n")
	}
	b.WriteString("  return true;\n")
	b.WriteString("}\n")
	return b.String(), nil
}

func tsValidatorExprFromType(t reflect.Type, valueExpr string, registry *tsInterfaceRegistry, depth int) (string, error) {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.PkgPath() == "time" && t.Name() == "Time" {
		return "typeof " + valueExpr + " === 'string'", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "FormData" {
		return valueExpr + " instanceof FormData", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "RawBytes" {
		return valueExpr + " instanceof Uint8Array", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "StreamResponse" {
		return valueExpr + " instanceof Blob", nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return "typeof " + valueExpr + " === 'boolean'", nil
	case reflect.String:
		return "typeof " + valueExpr + " === 'string'", nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Float32, reflect.Float64:
		return "typeof " + valueExpr + " === 'number'", nil
	case reflect.Int64, reflect.Uint64:
		return "typeof " + valueExpr + " === 'string'", nil
	case reflect.Struct:
		if t.Name() != "" {
			name, err := registry.ensureNamedStructType(t)
			if err != nil {
				return "", err
			}
			return "validate" + name + "(" + valueExpr + ")", nil
		}
		return "isPlainObject(" + valueExpr + ")", nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return "isPlainObject(" + valueExpr + ")", nil
		}
		itemName := fmt.Sprintf("v%d", depth+1)
		elemExpr, err := tsValidatorExprFromType(t.Elem(), itemName, registry, depth+1)
		if err != nil {
			return "", err
		}
		return "isPlainObject(" + valueExpr + ") && Object.values(" + valueExpr + ").every((" + itemName + ") => " + elemExpr + ")", nil
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return "typeof " + valueExpr + " === 'string'", nil
		}
		itemName := fmt.Sprintf("v%d", depth+1)
		elemExpr, err := tsValidatorExprFromType(t.Elem(), itemName, registry, depth+1)
		if err != nil {
			return "", err
		}
		return "Array.isArray(" + valueExpr + ") && " + valueExpr + ".every((" + itemName + ") => " + elemExpr + ")", nil
	case reflect.Interface:
		return "true", nil
	default:
		return "true", nil
	}
}

func renderTSFieldComment(comment string) string {
	lines := strings.Split(escapeTSComment(comment), "\n")
	if len(lines) == 1 {
		return fmt.Sprintf("  /** %s */\n", strings.TrimSpace(lines[0]))
	}
	var b strings.Builder
	b.WriteString("  /**\n")
	for _, line := range lines {
		b.WriteString("   * ")
		b.WriteString(strings.TrimSpace(line))
		b.WriteString("\n")
	}
	b.WriteString("   */\n")
	return b.String()
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
		val := keyToVal[name]
		fieldType, fieldSig, err := tsTypeFromValue(val, registry)
		if err != nil {
			return "", "", err
		}
		prop := tsPropName(name)
		separator := ";"
		if isMultilineObjectType(fieldType) {
			separator = ","
		}
		lines.WriteString("  ")
		lines.WriteString(prop)
		lines.WriteString(": ")
		lines.WriteString(fieldType)
		lines.WriteString(separator)
		lines.WriteString("\n")
		sigs = append(sigs, name+":"+fieldSig)
	}
	sort.Strings(sigs)
	return lines.String(), "{" + strings.Join(sigs, ";") + "}", nil
}

func isMultilineObjectType(tsType string) bool {
	return strings.Contains(tsType, "\n")
}

func tsTypeFromValue(v reflect.Value, registry *tsInterfaceRegistry) (string, string, error) {
	if !v.IsValid() {
		return "unknown", "unknown", nil
	}

	if v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "unknown", "unknown", nil
		}
		return tsTypeFromValue(v.Elem(), registry)
	}

	if v.Kind() == reflect.Map {
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
		return "string", "time", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "FormData" {
		return "FormData", "formdata", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "RawBytes" {
		return "Uint8Array", "rawbytes", nil
	}
	if t.PkgPath() == "github.com/RapboyGao/nuxtGin/endpoint" && t.Name() == "StreamResponse" {
		return "Blob", "blob", nil
	}

	switch t.Kind() {
	case reflect.Bool:
		return "boolean", "bool", nil
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

type tsUnionLiteral struct {
	Type  string
	Value string
}

func tsUnionValuesFromField(f reflect.StructField) ([]tsUnionLiteral, bool, error) {
	raw := strings.TrimSpace(f.Tag.Get("tsunion"))
	if raw == "" {
		return nil, false, nil
	}
	base := f.Type
	for base.Kind() == reflect.Ptr {
		base = base.Elem()
	}
	parts := strings.Split(raw, ",")
	if len(parts) == 1 {
		parts = strings.Split(raw, "|")
	}
	values := make([]tsUnionLiteral, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		literal, err := parseTSUnionLiteral(base, v)
		if err != nil {
			return nil, false, fmt.Errorf("field %s: %w", f.Name, err)
		}
		key := literal.Type + ":" + literal.Value
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, literal)
	}
	if len(values) == 0 {
		return nil, false, fmt.Errorf("field %s: tsunion is empty", f.Name)
	}
	return values, true, nil
}

func parseTSUnionLiteral(base reflect.Type, raw string) (tsUnionLiteral, error) {
	switch base.Kind() {
	case reflect.String:
		v := strings.Trim(raw, `"'`)
		if v == "" {
			return tsUnionLiteral{}, fmt.Errorf("tsunion string literal is empty")
		}
		return tsUnionLiteral{Type: "string", Value: v}, nil
	case reflect.Bool:
		v := strings.ToLower(strings.TrimSpace(raw))
		if v != "true" && v != "false" {
			return tsUnionLiteral{}, fmt.Errorf("tsunion bool literal must be true or false")
		}
		return tsUnionLiteral{Type: "boolean", Value: v}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return tsUnionLiteral{}, fmt.Errorf("invalid integer literal %q", raw)
		}
		return tsUnionLiteral{Type: "number", Value: strconv.FormatInt(n, 10)}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return tsUnionLiteral{}, fmt.Errorf("invalid unsigned integer literal %q", raw)
		}
		return tsUnionLiteral{Type: "number", Value: strconv.FormatUint(n, 10)}, nil
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return tsUnionLiteral{}, fmt.Errorf("invalid float literal %q", raw)
		}
		return tsUnionLiteral{Type: "number", Value: strconv.FormatFloat(n, 'f', -1, 64)}, nil
	default:
		return tsUnionLiteral{}, fmt.Errorf("tsunion supports string, bool, int/uint, float fields only")
	}
}

func tsUnionType(values []tsUnionLiteral) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		switch v.Type {
		case "string":
			parts = append(parts, "'"+strings.ReplaceAll(v.Value, "'", "\\'")+"'")
		default:
			parts = append(parts, v.Value)
		}
	}
	return strings.Join(parts, " | ")
}

func tsUnionSig(values []tsUnionLiteral) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, v.Type+":"+v.Value)
	}
	return strings.Join(parts, "|")
}

func tsUnionValidatorExpr(valueExpr string, values []tsUnionLiteral) string {
	if len(values) == 0 {
		return "false"
	}
	typeofName := values[0].Type
	for _, v := range values {
		if v.Type != typeofName {
			return "false"
		}
	}
	tsTypeOf := "string"
	switch typeofName {
	case "number":
		tsTypeOf = "number"
	case "boolean":
		tsTypeOf = "boolean"
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		if v.Type == "string" {
			parts = append(parts, valueExpr+" === '"+strings.ReplaceAll(v.Value, "'", "\\'")+"'")
			continue
		}
		parts = append(parts, valueExpr+" === "+v.Value)
	}
	return "typeof " + valueExpr + " === '" + tsTypeOf + "' && (" + strings.Join(parts, " || ") + ")"
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
