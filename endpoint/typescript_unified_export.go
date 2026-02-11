package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UnifiedTSExportOptions controls output paths for unified TS export.
// UnifiedTSExportOptions 用于配置统一 TS 导出的输出路径。
type UnifiedTSExportOptions struct {
	ServerTSPath    string
	WebSocketTSPath string
	SchemaTSPath    string
}

// ExportUnifiedAPIsToTSFiles exports ServerAPI and WebSocketAPI into two TS files,
// and deduplicates interfaces/types/validators/ensure functions into one shared schema file.
// ExportUnifiedAPIsToTSFiles 会导出 ServerAPI 与 WebSocketAPI 到两个 TS 文件，
// 并将接口/类型/validator/ensure 去重后输出到一个共享 schema 文件。
func ExportUnifiedAPIsToTSFiles(serverAPI ServerAPI, wsAPI WebSocketAPI, options UnifiedTSExportOptions) error {
	if strings.TrimSpace(options.ServerTSPath) == "" {
		return fmt.Errorf("server ts path is required")
	}
	if strings.TrimSpace(options.WebSocketTSPath) == "" {
		return fmt.Errorf("websocket ts path is required")
	}
	if strings.TrimSpace(options.SchemaTSPath) == "" {
		return fmt.Errorf("schema ts path is required")
	}
	if filepath.IsAbs(options.ServerTSPath) || filepath.IsAbs(options.WebSocketTSPath) || filepath.IsAbs(options.SchemaTSPath) {
		return fmt.Errorf("all ts paths must be relative")
	}

	serverBase := resolveAPIPath(serverAPI.BasePath, serverAPI.GroupPath)
	wsBase := resolveAPIPath(wsAPI.BasePath, wsAPI.GroupPath)

	serverCode, err := GenerateAxiosFromEndpoints(serverBase, serverAPI.Endpoints)
	if err != nil {
		return err
	}
	wsCode, err := GenerateWebSocketClientFromEndpoints(wsBase, wsAPI.Endpoints)
	if err != nil {
		return err
	}

	serverCodeBody, serverSchemaRegion, err := splitInterfacesRegion(serverCode)
	if err != nil {
		return fmt.Errorf("extract server schema region failed: %w", err)
	}
	wsCodeBody, wsSchemaRegion, err := splitInterfacesRegion(wsCode)
	if err != nil {
		return fmt.Errorf("extract websocket schema region failed: %w", err)
	}

	blocks := dedupeExportBlocks(append(parseExportBlocks(serverSchemaRegion), parseExportBlocks(wsSchemaRegion)...))
	sharedCode := renderSharedSchemaTS(blocks)

	typeNames, funcNames := collectSharedExportNames(blocks)
	schemaImportForServer := buildTSImportPath(options.ServerTSPath, options.SchemaTSPath)
	schemaImportForWS := buildTSImportPath(options.WebSocketTSPath, options.SchemaTSPath)

	serverTypeImports := usedSymbolsInCode(typeNames, serverCodeBody)
	serverFuncImports := usedSymbolsInCode(funcNames, serverCodeBody)
	serverCodeBody = injectTSImports(serverCodeBody, buildImportStatements(schemaImportForServer, serverTypeImports, serverFuncImports))

	wsTypeImports := usedSymbolsInCode(typeNames, wsCodeBody)
	wsFuncImports := usedSymbolsInCode(funcNames, wsCodeBody)
	wsCodeBody = injectTSImports(wsCodeBody, buildImportStatements(schemaImportForWS, wsTypeImports, wsFuncImports))

	if err := writeRelativeTSFile(options.SchemaTSPath, sharedCode); err != nil {
		return err
	}
	if err := writeRelativeTSFile(options.ServerTSPath, serverCodeBody); err != nil {
		return err
	}
	if err := writeRelativeTSFile(options.WebSocketTSPath, wsCodeBody); err != nil {
		return err
	}
	return nil
}

type tsExportBlock struct {
	Kind string
	Name string
	Body string
}

func splitInterfacesRegion(code string) (string, string, error) {
	const startTag = "// #region Interfaces & Validators"
	const endTag = "// #endregion Interfaces & Validators"
	start := strings.Index(code, startTag)
	if start < 0 {
		return "", "", fmt.Errorf("interfaces region start marker not found")
	}
	end := strings.Index(code[start:], endTag)
	if end < 0 {
		return "", "", fmt.Errorf("interfaces region end marker not found")
	}
	end += start
	end += len(endTag)

	region := code[start:end]
	body := strings.TrimSpace(code[:start]) + "\n\n" + strings.TrimSpace(code[end:]) + "\n"
	return body, region, nil
}

func parseExportBlocks(region string) []tsExportBlock {
	re := regexp.MustCompile(`(?m)^export\s+(interface|type|function)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	matches := re.FindAllStringSubmatchIndex(region, -1)
	if len(matches) == 0 {
		return nil
	}

	blocks := make([]tsExportBlock, 0, len(matches))
	for i, m := range matches {
		declStart := m[0]
		declEnd := len(region)
		if i+1 < len(matches) {
			declEnd = matches[i+1][0]
		}
		blockStart := findLeadingCommentBlockStart(region, declStart)
		kind := region[m[2]:m[3]]
		name := region[m[4]:m[5]]
		body := strings.TrimSpace(region[blockStart:declEnd])
		if body == "" {
			continue
		}
		blocks = append(blocks, tsExportBlock{Kind: kind, Name: name, Body: body})
	}
	return blocks
}

func findLeadingCommentBlockStart(content string, start int) int {
	i := start
	for i > 0 {
		prevNL := strings.LastIndex(content[:i-1], "\n")
		lineStart := prevNL + 1
		line := strings.TrimSpace(content[lineStart:i])
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "*/") {
			i = lineStart
			continue
		}
		break
	}
	return i
}

func dedupeExportBlocks(blocks []tsExportBlock) []tsExportBlock {
	seen := map[string]struct{}{}
	out := make([]tsExportBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Name == "" {
			continue
		}
		if _, ok := seen[b.Name]; ok {
			continue
		}
		seen[b.Name] = struct{}{}
		out = append(out, b)
	}
	return out
}

func collectSharedExportNames(blocks []tsExportBlock) ([]string, []string) {
	typeNames := make([]string, 0)
	funcNames := make([]string, 0)
	for _, b := range blocks {
		switch b.Kind {
		case "function":
			funcNames = append(funcNames, b.Name)
		default:
			typeNames = append(typeNames, b.Name)
		}
	}
	sort.Strings(typeNames)
	sort.Strings(funcNames)
	return uniqueStrings(typeNames), uniqueStrings(funcNames)
}

func usedSymbolsInCode(symbols []string, code string) []string {
	out := make([]string, 0, len(symbols))
	for _, s := range symbols {
		if s == "" {
			continue
		}
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(s) + `\b`)
		if re.FindStringIndex(code) != nil {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

func buildImportStatements(schemaImportPath string, typeNames, funcNames []string) []string {
	stmts := make([]string, 0, 2)
	if len(typeNames) > 0 {
		stmts = append(stmts, "import type { "+strings.Join(typeNames, ", ")+" } from '"+schemaImportPath+"';")
	}
	if len(funcNames) > 0 {
		stmts = append(stmts, "import { "+strings.Join(funcNames, ", ")+" } from '"+schemaImportPath+"';")
	}
	return stmts
}

func injectTSImports(code string, importStmts []string) string {
	if len(importStmts) == 0 {
		return code
	}
	importsText := strings.Join(importStmts, "\n") + "\n"

	importRe := regexp.MustCompile(`(?m)^import .*;$`)
	locs := importRe.FindAllStringIndex(code, -1)
	if len(locs) > 0 {
		insertAt := locs[len(locs)-1][1]
		return code[:insertAt] + "\n" + importsText + code[insertAt:]
	}

	bannerEnd := strings.Index(code, "*/")
	if bannerEnd >= 0 {
		insertAt := bannerEnd + 2
		return code[:insertAt] + "\n\n" + importsText + code[insertAt:]
	}
	return importsText + "\n" + code
}

func buildTSImportPath(fromFileRelPath string, toFileRelPath string) string {
	fromDir := filepath.Dir(fromFileRelPath)
	rel, err := filepath.Rel(fromDir, toFileRelPath)
	if err != nil {
		rel = toFileRelPath
	}
	rel = filepath.ToSlash(rel)
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	if !strings.HasPrefix(rel, ".") {
		rel = "./" + rel
	}
	return rel
}

func renderSharedSchemaTS(blocks []tsExportBlock) string {
	var b strings.Builder
	writeTSBanner(&b, "Nuxt Gin Shared Schemas")
	writeTSMarker(&b, "Shared Helpers")
	b.WriteString("const isPlainObject = (value: unknown): value is Record<string, unknown> =>\n")
	b.WriteString("  Object.prototype.toString.call(value) === '[object Object]';\n\n")
	writeTSMarkerEnd(&b, "Shared Helpers")

	writeTSMarker(&b, "Interfaces & Validators")
	for _, block := range blocks {
		b.WriteString(block.Body)
		b.WriteString("\n\n")
	}
	writeTSMarkerEnd(&b, "Interfaces & Validators")
	return finalizeTypeScriptCode(b.String())
}

func writeRelativeTSFile(relativeTSPath string, code string) error {
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

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
