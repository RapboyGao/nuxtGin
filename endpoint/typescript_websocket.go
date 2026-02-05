package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

type wsFuncMeta struct {
	FuncName    string
	Path        string
	Description string
	ClientType  string
	ServerType  string
}

// GenerateWebSocketClientFromEndpoints generates TypeScript websocket client source code from endpoints.
// GenerateWebSocketClientFromEndpoints 根据 WebSocketEndpoint 列表生成 TypeScript 客户端代码。
func GenerateWebSocketClientFromEndpoints(baseURL string, endpoints []WebSocketEndpointLike) (string, error) {
	return generateWebSocketClientFromEndpoints(baseURL, endpoints)
}

// ExportWebSocketClientFromEndpointsToTSFile writes generated TS code from endpoints to a file.
// ExportWebSocketClientFromEndpointsToTSFile 将 WebSocketEndpoint 生成的 TS 代码写入文件。
func ExportWebSocketClientFromEndpointsToTSFile(baseURL string, endpoints []WebSocketEndpointLike, relativeTSPath string) error {
	return exportWebSocketClientFromEndpointsToTSFile(baseURL, endpoints, relativeTSPath)
}

func generateWebSocketClientFromEndpoints(baseURL string, endpoints []WebSocketEndpointLike) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]wsFuncMeta, 0, len(endpoints))

	for i, e := range endpoints {
		meta := e.WebSocketMeta()
		if err := validateWebSocketMeta(meta); err != nil {
			return "", fmt.Errorf("websocket endpoint[%d] validation failed: %w", i, err)
		}

		base := wsBaseName(meta, i)

		clientType, _, err := tsTypeFromType(meta.ClientMessageType, registry)
		if err != nil {
			return "", fmt.Errorf("build client message type for websocket endpoint[%d]: %w", i, err)
		}
		serverType, _, err := tsTypeFromType(meta.ServerMessageType, registry)
		if err != nil {
			return "", fmt.Errorf("build server message type for websocket endpoint[%d]: %w", i, err)
		}

		metas = append(metas, wsFuncMeta{
			FuncName:    toLowerCamel(base),
			Path:        meta.Path,
			Description: strings.TrimSpace(meta.Description),
			ClientType:  clientType,
			ServerType:  serverType,
		})
	}

	return renderWebSocketTS(baseURL, registry, metas)
}

func exportWebSocketClientFromEndpointsToTSFile(baseURL string, endpoints []WebSocketEndpointLike, relativeTSPath string) error {
	if strings.TrimSpace(relativeTSPath) == "" {
		return fmt.Errorf("relative ts path is required")
	}
	if filepath.IsAbs(relativeTSPath) {
		return fmt.Errorf("ts file path must be relative to cwd")
	}

	code, err := generateWebSocketClientFromEndpoints(baseURL, endpoints)
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

func validateWebSocketMeta(meta WebSocketEndpointMeta) error {
	if strings.TrimSpace(meta.Path) == "" {
		return fmt.Errorf("path is required")
	}
	if meta.ClientMessageType == nil || meta.ClientMessageType.Kind() == reflect.Invalid || isNoType(meta.ClientMessageType) {
		return fmt.Errorf("client message type is required")
	}
	if meta.ServerMessageType == nil || meta.ServerMessageType.Kind() == reflect.Invalid || isNoType(meta.ServerMessageType) {
		return fmt.Errorf("server message type is required")
	}
	return nil
}

func wsBaseName(meta WebSocketEndpointMeta, index int) string {
	if n := strings.TrimSpace(meta.Name); n != "" {
		return toUpperCamel(n)
	}
	raw := meta.Path
	raw = strings.ReplaceAll(raw, "{", " ")
	raw = strings.ReplaceAll(raw, "}", " ")
	raw = strings.ReplaceAll(raw, ":", " by ")
	raw = strings.ReplaceAll(raw, "/", " ")
	base := toUpperCamel(raw)
	if base == "" {
		return fmt.Sprintf("Ws%d", index+1)
	}
	return base
}

func renderWebSocketTS(baseURL string, registry *tsInterfaceRegistry, metas []wsFuncMeta) (string, error) {
	var b strings.Builder

	writeTSBanner(&b, "Nuxt Gin WebSocket Client")
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

	b.WriteString("export interface WebSocketConvertOptions<TSend = unknown, TReceive = unknown> {\n")
	b.WriteString("  serialize?: (value: TSend) => unknown;\n")
	b.WriteString("  deserialize?: (value: unknown) => TReceive;\n")
	b.WriteString("}\n\n")

	b.WriteString("export interface WebSocketClient<TReceive = unknown, TSend = unknown> {\n")
	b.WriteString("  socket: WebSocket;\n")
	b.WriteString("  send: (message: TSend) => void;\n")
	b.WriteString("  close: () => void;\n")
	b.WriteString("  onMessage: (handler: (message: TReceive) => void) => () => void;\n")
	b.WriteString("  onOpen: (handler: (event: Event) => void) => () => void;\n")
	b.WriteString("  onClose: (handler: (event: CloseEvent) => void) => () => void;\n")
	b.WriteString("  onError: (handler: (event: Event) => void) => () => void;\n")
	b.WriteString("}\n\n")

	b.WriteString("const resolveWebSocketURL = (url: string): string => {\n")
	b.WriteString("  if (url.startsWith('ws://') || url.startsWith('wss://')) return url;\n")
	b.WriteString("  if (url.startsWith('http://')) return `ws://${url.slice(7)}`;\n")
	b.WriteString("  if (url.startsWith('https://')) return `wss://${url.slice(8)}`;\n")
	b.WriteString("  if (url.startsWith('/')) {\n")
	b.WriteString("    const isHttps = typeof window !== 'undefined' && window.location?.protocol === 'https:';\n")
	b.WriteString("    const host = typeof window !== 'undefined' ? window.location.host : '';\n")
	b.WriteString("    return `${isHttps ? 'wss' : 'ws'}://${host}${url}`;\n")
	b.WriteString("  }\n")
	b.WriteString("  return url;\n")
	b.WriteString("};\n\n")

	b.WriteString("const joinURLPath = (baseURL: string, path: string): string => {\n")
	b.WriteString("  const base = baseURL.trim();\n")
	b.WriteString("  const p = path.trim();\n")
	b.WriteString("  if (!base) return p.startsWith('/') ? p : `/${p}`;\n")
	b.WriteString("  if (!p) return base.startsWith('/') ? base.replace(/\\/+$/, '') : `/${base.replace(/\\/+$/, '')}`;\n")
	b.WriteString("  const trimmedBase = base.replace(/\\/+$/, '');\n")
	b.WriteString("  const trimmedPath = p.replace(/^\\/+/, '');\n")
	b.WriteString("  return trimmedBase.startsWith('/') ? `${trimmedBase}/${trimmedPath}` : `/${trimmedBase}/${trimmedPath}`;\n")
	b.WriteString("};\n\n")

	b.WriteString("const createWebSocketClient = <TReceive, TSend>(\n")
	b.WriteString("  url: string,\n")
	b.WriteString("  options?: WebSocketConvertOptions<TSend, TReceive>\n")
	b.WriteString("): WebSocketClient<TReceive, TSend> => {\n")
	b.WriteString("  const socket = new WebSocket(resolveWebSocketURL(url));\n")
	b.WriteString("  const messageListeners = new Set<(message: TReceive) => void>();\n")
	b.WriteString("  const openListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  const closeListeners = new Set<(event: CloseEvent) => void>();\n")
	b.WriteString("  const errorListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  const serialize = options?.serialize ?? ((value: TSend) => normalizeRequestJSON(value));\n")
	b.WriteString("  const deserialize = options?.deserialize ?? ((value: unknown) => normalizeResponseJSON(value) as TReceive);\n")
	b.WriteString("\n")
	b.WriteString("  socket.addEventListener('message', (event) => {\n")
	b.WriteString("    let payload: unknown = event.data;\n")
	b.WriteString("    if (typeof payload === 'string') {\n")
	b.WriteString("      try {\n")
	b.WriteString("        payload = JSON.parse(payload);\n")
	b.WriteString("      } catch {\n")
	b.WriteString("        // keep raw payload\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n")
	b.WriteString("    const message = deserialize(payload);\n")
	b.WriteString("    for (const listener of messageListeners) listener(message);\n")
	b.WriteString("  });\n")
	b.WriteString("  socket.addEventListener('open', (event) => {\n")
	b.WriteString("    for (const listener of openListeners) listener(event);\n")
	b.WriteString("  });\n")
	b.WriteString("  socket.addEventListener('close', (event) => {\n")
	b.WriteString("    for (const listener of closeListeners) listener(event);\n")
	b.WriteString("  });\n")
	b.WriteString("  socket.addEventListener('error', (event) => {\n")
	b.WriteString("    for (const listener of errorListeners) listener(event);\n")
	b.WriteString("  });\n")
	b.WriteString("\n")
	b.WriteString("  return {\n")
	b.WriteString("    socket,\n")
	b.WriteString("    send: (message: TSend) => {\n")
	b.WriteString("      const data = serialize(message);\n")
	b.WriteString("      socket.send(JSON.stringify(data));\n")
	b.WriteString("    },\n")
	b.WriteString("    close: () => socket.close(),\n")
	b.WriteString("    onMessage: (handler) => {\n")
	b.WriteString("      messageListeners.add(handler);\n")
	b.WriteString("      return () => messageListeners.delete(handler);\n")
	b.WriteString("    },\n")
	b.WriteString("    onOpen: (handler) => {\n")
	b.WriteString("      openListeners.add(handler);\n")
	b.WriteString("      return () => openListeners.delete(handler);\n")
	b.WriteString("    },\n")
	b.WriteString("    onClose: (handler) => {\n")
	b.WriteString("      closeListeners.add(handler);\n")
	b.WriteString("      return () => closeListeners.delete(handler);\n")
	b.WriteString("    },\n")
	b.WriteString("    onError: (handler) => {\n")
	b.WriteString("      errorListeners.add(handler);\n")
	b.WriteString("      return () => errorListeners.delete(handler);\n")
	b.WriteString("    },\n")
	b.WriteString("  };\n")
	b.WriteString("};\n\n")

	for _, def := range registry.defs {
		b.WriteString("export interface ")
		b.WriteString(def.Name)
		b.WriteString(" {\n")
		if def.Body != "" {
			b.WriteString(def.Body)
		}
		b.WriteString("}\n\n")
	}

	basePath := strings.TrimSpace(baseURL)
	for _, m := range metas {
		if m.Description != "" {
			b.WriteString("/**\n")
			b.WriteString(" * ")
			b.WriteString(escapeTSComment(m.Description))
			b.WriteString("\n")
			b.WriteString(" */\n")
		}
		b.WriteString("export function ")
		b.WriteString(m.FuncName)
		b.WriteString("(options?: WebSocketConvertOptions<")
		b.WriteString(m.ClientType)
		b.WriteString(", ")
		b.WriteString(m.ServerType)
		b.WriteString(">): WebSocketClient<")
		b.WriteString(m.ServerType)
		b.WriteString(", ")
		b.WriteString(m.ClientType)
		b.WriteString("> {\n")
		b.WriteString("  const url = joinURLPath('")
		b.WriteString(strings.ReplaceAll(basePath, "'", "\\'"))
		b.WriteString("', '")
		b.WriteString(strings.ReplaceAll(m.Path, "'", "\\'"))
		b.WriteString("');\n")
		b.WriteString("  return createWebSocketClient<")
		b.WriteString(m.ServerType)
		b.WriteString(", ")
		b.WriteString(m.ClientType)
		b.WriteString(">(url, options);\n")
		b.WriteString("}\n\n")
	}

	return strings.TrimSpace(b.String()) + "\n", nil
}
