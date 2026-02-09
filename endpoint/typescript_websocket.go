package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

type wsFuncMeta struct {
	FuncName     string
	Path         string
	Description  string
	ClientType   string
	ServerType   string
	MessageTypes []string
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
			FuncName:     toLowerCamel(base),
			Path:         meta.Path,
			Description:  strings.TrimSpace(meta.Description),
			ClientType:   clientType,
			ServerType:   serverType,
			MessageTypes: normalizeMessageTypes(meta.MessageTypes),
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
	b.WriteString("const normalizeWsRequestJSON = (value: unknown): unknown => {\n")
	b.WriteString("  if (value instanceof Date) return value.toISOString();\n")
	b.WriteString("  if (Array.isArray(value)) return value.map(normalizeWsRequestJSON);\n")
	b.WriteString("  if (isPlainObject(value)) {\n")
	b.WriteString("    const out: Record<string, unknown> = {};\n")
	b.WriteString("    for (const [k, v] of Object.entries(value)) out[k] = normalizeWsRequestJSON(v);\n")
	b.WriteString("    return out;\n")
	b.WriteString("  }\n")
	b.WriteString("  return value;\n")
	b.WriteString("};\n\n")
	b.WriteString("const normalizeWsResponseJSON = (value: unknown): unknown => {\n")
	b.WriteString("  if (Array.isArray(value)) return value.map(normalizeWsResponseJSON);\n")
	b.WriteString("  if (typeof value === 'string' && isoDateLike.test(value)) {\n")
	b.WriteString("    const date = new Date(value);\n")
	b.WriteString("    if (!Number.isNaN(date.getTime())) return date;\n")
	b.WriteString("  }\n")
	b.WriteString("  if (isPlainObject(value)) {\n")
	b.WriteString("    const out: Record<string, unknown> = {};\n")
	b.WriteString("    for (const [k, v] of Object.entries(value)) out[k] = normalizeWsResponseJSON(v);\n")
	b.WriteString("    return out;\n")
	b.WriteString("  }\n")
	b.WriteString("  return value;\n")
	b.WriteString("};\n\n")

	b.WriteString("export interface WebSocketConvertOptions<TSend = unknown, TReceive = unknown> {\n")
	b.WriteString("  serialize?: (value: TSend) => unknown;\n")
	b.WriteString("  deserialize?: (value: unknown) => TReceive;\n")
	b.WriteString("}\n\n")

	b.WriteString("export interface TypedHandlerOptions<TReceive, TPayload> {\n")
	b.WriteString("  selectPayload?: (message: TReceive) => unknown;\n")
	b.WriteString("  decode?: (payload: unknown) => TPayload;\n")
	b.WriteString("  validate?: (payload: unknown, message: TReceive) => boolean;\n")
	b.WriteString("}\n\n")

	b.WriteString("export interface TypeHandlerOptions<TReceive> {\n")
	b.WriteString("  validate?: (message: TReceive) => boolean;\n")
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

	b.WriteString("export class TypedWebSocketClient<TReceive = unknown, TSend = unknown, TType extends string = string> {\n")
	b.WriteString("  public readonly socket: WebSocket;\n")
	b.WriteString("  private readonly serialize: (value: TSend) => unknown;\n")
	b.WriteString("  private readonly deserialize: (value: unknown) => TReceive;\n")
	b.WriteString("  private readonly messageListeners = new Set<(message: TReceive) => void>();\n")
	b.WriteString("  private readonly openListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  private readonly closeListeners = new Set<(event: CloseEvent) => void>();\n")
	b.WriteString("  private readonly errorListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  private readonly typedListeners = new Map<TType, Set<(message: TReceive) => void>>();\n\n")
	b.WriteString("  constructor(\n")
	b.WriteString("  url: string,\n")
	b.WriteString("  options: WebSocketConvertOptions<TSend, TReceive>\n")
	b.WriteString("  ) {\n")
	b.WriteString("    this.socket = new WebSocket(resolveWebSocketURL(url));\n")
	b.WriteString("    this.serialize = options?.serialize ?? ((value: TSend) => normalizeWsRequestJSON(value));\n")
	b.WriteString("    this.deserialize = options?.deserialize ?? ((value: unknown) => normalizeWsResponseJSON(value) as TReceive);\n")
	b.WriteString("\n")
	b.WriteString("    this.socket.addEventListener('message', (event) => {\n")
	b.WriteString("      let payload: unknown = event.data;\n")
	b.WriteString("      if (typeof payload === 'string') {\n")
	b.WriteString("        try {\n")
	b.WriteString("          payload = JSON.parse(payload);\n")
	b.WriteString("        } catch {\n")
	b.WriteString("          // keep raw payload\n")
	b.WriteString("        }\n")
	b.WriteString("      }\n")
	b.WriteString("      const message = this.deserialize(payload);\n")
	b.WriteString("      this.emitMessage(message);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('open', (event) => {\n")
	b.WriteString("      for (const listener of this.openListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('close', (event) => {\n")
	b.WriteString("      for (const listener of this.closeListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('error', (event) => {\n")
	b.WriteString("      for (const listener of this.errorListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("  }\n\n")
	b.WriteString("  send(message: TSend): void {\n")
	b.WriteString("    const data = this.serialize(message);\n")
	b.WriteString("    this.socket.send(JSON.stringify(data));\n")
	b.WriteString("  }\n\n")
	b.WriteString("  close(): void {\n")
	b.WriteString("    this.socket.close();\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onMessage(handler: (message: TReceive) => void): () => void {\n")
	b.WriteString("    this.messageListeners.add(handler);\n")
	b.WriteString("    return () => this.messageListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onOpen(handler: (event: Event) => void): () => void {\n")
	b.WriteString("    this.openListeners.add(handler);\n")
	b.WriteString("    return () => this.openListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onClose(handler: (event: CloseEvent) => void): () => void {\n")
	b.WriteString("    this.closeListeners.add(handler);\n")
	b.WriteString("    return () => this.closeListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onError(handler: (event: Event) => void): () => void {\n")
	b.WriteString("    this.errorListeners.add(handler);\n")
	b.WriteString("    return () => this.errorListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onType(type: TType, handler: (message: TReceive) => void, options?: TypeHandlerOptions<TReceive>): () => void {\n")
	b.WriteString("    const listeners = this.typedListeners.get(type) ?? new Set<(message: TReceive) => void>();\n")
	b.WriteString("    const wrapped = (message: TReceive) => {\n")
	b.WriteString("      if (options?.validate && !options.validate(message)) return;\n")
	b.WriteString("      handler(message);\n")
	b.WriteString("    };\n")
	b.WriteString("    listeners.add(wrapped);\n")
	b.WriteString("    this.typedListeners.set(type, listeners);\n")
	b.WriteString("    return () => {\n")
	b.WriteString("      const current = this.typedListeners.get(type);\n")
	b.WriteString("      if (!current) return;\n")
	b.WriteString("      current.delete(wrapped);\n")
	b.WriteString("      if (current.size === 0) this.typedListeners.delete(type);\n")
	b.WriteString("    };\n")
	b.WriteString("  }\n\n")
	b.WriteString("  onTyped<TPayload>(\n")
	b.WriteString("    type: TType,\n")
	b.WriteString("    handler: (payload: TPayload, message: TReceive) => void,\n")
	b.WriteString("    options?: TypedHandlerOptions<TReceive, TPayload>\n")
	b.WriteString("  ): () => void {\n")
	b.WriteString("    return this.onType(type, (message) => {\n")
	b.WriteString("      const rawPayload = options?.selectPayload ? options.selectPayload(message) : this.defaultPayload(message);\n")
	b.WriteString("      if (options?.validate && !options.validate(rawPayload, message)) return;\n")
	b.WriteString("      const payload = options?.decode ? options.decode(rawPayload) : (rawPayload as TPayload);\n")
	b.WriteString("      handler(payload, message);\n")
	b.WriteString("    });\n")
	b.WriteString("  }\n\n")
	b.WriteString("  private emitMessage(message: TReceive): void {\n")
	b.WriteString("    for (const listener of this.messageListeners) {\n")
	b.WriteString("      try {\n")
	b.WriteString("        listener(message);\n")
	b.WriteString("      } catch {\n")
	b.WriteString("        // ignore single listener errors and continue dispatch\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n")
	b.WriteString("    const type = this.defaultMessageType(message);\n")
	b.WriteString("    if (!type) return;\n")
	b.WriteString("    const listeners = this.typedListeners.get(type);\n")
	b.WriteString("    if (!listeners) return;\n")
	b.WriteString("    for (const listener of listeners) {\n")
	b.WriteString("      try {\n")
	b.WriteString("        listener(message);\n")
	b.WriteString("      } catch {\n")
	b.WriteString("        // ignore single listener errors and continue dispatch\n")
	b.WriteString("      }\n")
	b.WriteString("    }\n")
	b.WriteString("  }\n\n")
	b.WriteString("  private defaultMessageType(message: TReceive): TType | undefined {\n")
	b.WriteString("    if (!isPlainObject(message)) return undefined;\n")
	b.WriteString("    const value = (message as Record<string, unknown>)['type'];\n")
	b.WriteString("    return typeof value === 'string' ? (value as TType) : undefined;\n")
	b.WriteString("  }\n\n")
	b.WriteString("  private defaultPayload(message: TReceive): unknown {\n")
	b.WriteString("    if (!isPlainObject(message)) return message;\n")
	b.WriteString("    return (message as Record<string, unknown>)['payload'];\n")
	b.WriteString("  }\n")
	b.WriteString("}\n\n")

	basePath := strings.TrimSpace(baseURL)
	for _, m := range metas {
		if m.Description != "" {
			b.WriteString("/**\n")
			b.WriteString(" * ")
			b.WriteString(escapeTSComment(m.Description))
			b.WriteString("\n")
			b.WriteString(" */\n")
		}
		b.WriteString("// Literal union is emitted as type because interface cannot model union values.\n")
		b.WriteString("// 字面量联合类型使用 type，因为 interface 不能表达联合值。\n")
		b.WriteString("export type ")
		b.WriteString(toUpperCamel(m.FuncName))
		b.WriteString("MessageType = ")
		b.WriteString(renderMessageTypeUnion(m.MessageTypes))
		b.WriteString(";\n")
		b.WriteString("export function ")
		b.WriteString(m.FuncName)
		b.WriteString("<TSend = ")
		b.WriteString(m.ClientType)
		b.WriteString(">(options: WebSocketConvertOptions<TSend, ")
		b.WriteString(m.ServerType)
		b.WriteString(">): TypedWebSocketClient<")
		b.WriteString(m.ServerType)
		b.WriteString(", TSend, ")
		b.WriteString(toUpperCamel(m.FuncName))
		b.WriteString("MessageType> {\n")
		b.WriteString("  const url = joinURLPath('")
		b.WriteString(strings.ReplaceAll(basePath, "'", "\\'"))
		b.WriteString("', '")
		b.WriteString(strings.ReplaceAll(m.Path, "'", "\\'"))
		b.WriteString("');\n")
		b.WriteString("  return new TypedWebSocketClient<")
		b.WriteString(m.ServerType)
		b.WriteString(", ")
		b.WriteString("TSend, ")
		b.WriteString(toUpperCamel(m.FuncName))
		b.WriteString("MessageType")
		b.WriteString(">(url, options);\n")
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
			b.WriteString("/**\n")
				b.WriteString(" * Ensure a typed ")
			b.WriteString(def.Name)
			b.WriteString(" after validation.\n")
				b.WriteString(" * 先校验，再确保得到类型化的 ")
			b.WriteString(def.Name)
			b.WriteString("。\n")
			b.WriteString(" */\n")
				b.WriteString("export function ensure")
			b.WriteString(def.Name)
			b.WriteString("(value: unknown): ")
			b.WriteString(def.Name)
			b.WriteString(" {\n")
			b.WriteString("  if (!validate")
			b.WriteString(def.Name)
			b.WriteString("(value)) {\n")
			b.WriteString("    throw new Error('Invalid ")
			b.WriteString(def.Name)
			b.WriteString("');\n")
			b.WriteString("  }\n")
			b.WriteString("  return value;\n")
			b.WriteString("}\n\n")
		}
	}

	return strings.TrimSpace(b.String()) + "\n", nil
}

func normalizeMessageTypes(types []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(types))
	for _, t := range types {
		v := strings.TrimSpace(t)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func renderMessageTypeUnion(types []string) string {
	if len(types) == 0 {
		return "string"
	}
	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, "'"+strings.ReplaceAll(t, "'", "\\'")+"'")
	}
	return strings.Join(parts, " | ")
}
