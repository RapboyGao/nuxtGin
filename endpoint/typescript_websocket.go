package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type wsFuncMeta struct {
	FuncName            string
	Path                string
	Description         string
	ClientType          string
	ServerType          string
	MessageTypes        []string
	ClientPayloadByType map[string]string
	ServerPayloadByType map[string]string
}

// GenerateWebSocketClientFromEndpoints generates TypeScript websocket client source code from endpoints.
// GenerateWebSocketClientFromEndpoints 根据 WebSocketEndpoint 列表生成 TypeScript 客户端代码。
func GenerateWebSocketClientFromEndpoints(baseURL string, endpoints []WebSocketEndpointLike) (string, error) {
	return generateWebSocketClientFromEndpoints(baseURL, "", endpoints)
}

// ExportWebSocketClientFromEndpointsToTSFile writes generated TS code from endpoints to a file.
// ExportWebSocketClientFromEndpointsToTSFile 将 WebSocketEndpoint 生成的 TS 代码写入文件。
func ExportWebSocketClientFromEndpointsToTSFile(baseURL string, endpoints []WebSocketEndpointLike, relativeTSPath string) error {
	return exportWebSocketClientFromEndpointsToTSFile(baseURL, "", endpoints, relativeTSPath)
}

func generateWebSocketClientFromEndpoints(basePath string, groupPath string, endpoints []WebSocketEndpointLike) (string, error) {
	registry := newTSInterfaceRegistry()
	metas := make([]wsFuncMeta, 0, len(endpoints))

	for i, e := range endpoints {
		meta := e.WebSocketMeta()
		if err := validateWebSocketMeta(meta); err != nil {
			return "", fmt.Errorf("websocket endpoint[%d] validation failed: %w", i, err)
		}
		if err := validateWebSocketPayloadTypeMappings(meta); err != nil {
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
		clientPayloadByType := map[string]string{}
		for msgType, payloadType := range meta.ClientPayloadTypes {
			if payloadType == nil || payloadType.Kind() == reflect.Invalid || isNoType(payloadType) {
				continue
			}
			payloadTSType, _, typeErr := tsTypeFromType(payloadType, registry)
			if typeErr != nil {
				return "", fmt.Errorf("build client payload type for websocket endpoint[%d] message type %q: %w", i, msgType, typeErr)
			}
			clientPayloadByType[msgType] = payloadTSType
		}
		serverPayloadByType := map[string]string{}
		for msgType, payloadType := range meta.ServerPayloadTypes {
			if payloadType == nil || payloadType.Kind() == reflect.Invalid || isNoType(payloadType) {
				continue
			}
			payloadTSType, _, typeErr := tsTypeFromType(payloadType, registry)
			if typeErr != nil {
				return "", fmt.Errorf("build server payload type for websocket endpoint[%d] message type %q: %w", i, msgType, typeErr)
			}
			serverPayloadByType[msgType] = payloadTSType
		}

		metas = append(metas, wsFuncMeta{
			FuncName:            toLowerCamel(base),
			Path:                meta.Path,
			Description:         strings.TrimSpace(meta.Description),
			ClientType:          clientType,
			ServerType:          serverType,
			MessageTypes:        normalizeMessageTypes(meta.MessageTypes),
			ClientPayloadByType: clientPayloadByType,
			ServerPayloadByType: serverPayloadByType,
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		ci := toUpperCamel(metas[i].FuncName)
		cj := toUpperCamel(metas[j].FuncName)
		if ci != cj {
			return ci < cj
		}
		return metas[i].Path < metas[j].Path
	})

	return renderWebSocketTS(basePath, groupPath, registry, metas)
}

func exportWebSocketClientFromEndpointsToTSFile(basePath string, groupPath string, endpoints []WebSocketEndpointLike, relativeTSPath string) error {
	if strings.TrimSpace(relativeTSPath) == "" {
		return fmt.Errorf("relative ts path is required")
	}
	if filepath.IsAbs(relativeTSPath) {
		return fmt.Errorf("ts file path must be relative to cwd")
	}

	code, err := generateWebSocketClientFromEndpoints(basePath, groupPath, endpoints)
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

func renderWebSocketTS(basePath string, groupPath string, registry *tsInterfaceRegistry, metas []wsFuncMeta) (string, error) {
	var b strings.Builder

	writeTSBanner(&b, "Nuxt Gin WebSocket Client")
	writeTSMarker(&b, "Runtime Helpers")
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

	b.WriteString("const isDevelopmentEnv = (): boolean => {\n")
	b.WriteString("  if (typeof import.meta !== 'undefined' && (import.meta as any)?.env) {\n")
	b.WriteString("    const dev = (import.meta as any).env?.DEV;\n")
	b.WriteString("    if (typeof dev === 'boolean') return dev;\n")
	b.WriteString("  }\n")
	b.WriteString("  return false;\n")
	b.WriteString("};\n\n")
	b.WriteString("const resolveGinPort = (): string => {\n")
	b.WriteString("  if (typeof window !== 'undefined') {\n")
	b.WriteString("    const ginPort = useRuntimeConfig().public.ginPort;\n")
	b.WriteString("    if (ginPort !== undefined && ginPort !== null && String(ginPort).trim() !== '') {\n")
	b.WriteString("      return String(ginPort);\n")
	b.WriteString("    }\n")
	b.WriteString("    if (window.location?.port && window.location.port.trim() !== '') {\n")
	b.WriteString("      return window.location.port;\n")
	b.WriteString("    }\n")
	b.WriteString("    return window.location?.protocol === 'https:' ? '443' : '80';\n")
	b.WriteString("  }\n")
	b.WriteString("  if (typeof import.meta !== 'undefined' && (import.meta as any)?.env?.NUXT_GIN_PORT) {\n")
	b.WriteString("    return String((import.meta as any).env.NUXT_GIN_PORT);\n")
	b.WriteString("  }\n")
	b.WriteString("  return '80';\n")
	b.WriteString("};\n\n")
	b.WriteString("const resolveWebSocketURL = (url: string): string => {\n")
	b.WriteString("  if (url.startsWith('ws://') || url.startsWith('wss://')) return url;\n")
	b.WriteString("  if (url.startsWith('http://')) return `ws://${url.slice(7)}`;\n")
	b.WriteString("  if (url.startsWith('https://')) return `wss://${url.slice(8)}`;\n")
	b.WriteString("  if (url.startsWith('/')) {\n")
	b.WriteString("    const isHttps = typeof window !== 'undefined' && window.location?.protocol === 'https:';\n")
	b.WriteString("    const protocol = isHttps ? 'wss' : 'ws';\n")
	b.WriteString("    if (typeof window !== 'undefined') {\n")
	b.WriteString("      if (isDevelopmentEnv()) {\n")
	b.WriteString("        return `${protocol}://${window.location.hostname}:${resolveGinPort()}${url}`;\n")
	b.WriteString("      }\n")
	b.WriteString("      return `${protocol}://${window.location.host}${url}`;\n")
	b.WriteString("    }\n")
	b.WriteString("    return url;\n")
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
	writeTSMarkerEnd(&b, "Runtime Helpers")

	writeTSMarker(&b, "Typed WebSocket Client")
	b.WriteString("/**\n")
	b.WriteString(" * Generic typed WebSocket client with message and type-based subscriptions.\n")
	b.WriteString(" * 通用的类型化 WebSocket 客户端，支持全量消息订阅与按 type 订阅。\n")
	b.WriteString(" */\n")
	b.WriteString("export class TypedWebSocketClient<TReceive = unknown, TSend = unknown, TType extends string = string> {\n")
	b.WriteString("  public readonly socket: WebSocket;\n")
	b.WriteString("  public readonly url: string;\n")
	b.WriteString("  public status: 'connecting' | 'open' | 'closing' | 'closed' = 'connecting';\n")
	b.WriteString("  public lastError?: Event;\n")
	b.WriteString("  public lastClose?: CloseEvent;\n")
	b.WriteString("  public connectedAt?: Date;\n")
	b.WriteString("  public closedAt?: Date;\n")
	b.WriteString("  public messagesSent = 0;\n")
	b.WriteString("  public messagesReceived = 0;\n")
	b.WriteString("  public reconnectCount = 0;\n")
	b.WriteString("  private readonly serialize: (value: TSend) => unknown;\n")
	b.WriteString("  private readonly deserialize: (value: unknown) => TReceive;\n")
	b.WriteString("  private readonly messageListeners = new Set<(message: TReceive) => void>();\n")
	b.WriteString("  private readonly openListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  private readonly closeListeners = new Set<(event: CloseEvent) => void>();\n")
	b.WriteString("  private readonly errorListeners = new Set<(event: Event) => void>();\n")
	b.WriteString("  private readonly typedListeners = new Map<TType, Set<(message: TReceive) => void>>();\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Create a websocket client and connect immediately.\n")
	b.WriteString("   * 创建 websocket 客户端并立即发起连接。\n")
	b.WriteString("   */\n")
	b.WriteString("  constructor(\n")
	b.WriteString("  url: string,\n")
	b.WriteString("  options: WebSocketConvertOptions<TSend, TReceive>\n")
	b.WriteString("  ) {\n")
	b.WriteString("    const resolvedURL = resolveWebSocketURL(url);\n")
	b.WriteString("    this.url = resolvedURL;\n")
	b.WriteString("    this.socket = new WebSocket(resolvedURL);\n")
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
	b.WriteString("      this.messagesReceived += 1;\n")
	b.WriteString("      this.emitMessage(message);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('open', (event) => {\n")
	b.WriteString("      this.status = 'open';\n")
	b.WriteString("      this.connectedAt = new Date();\n")
	b.WriteString("      this.closedAt = undefined;\n")
	b.WriteString("      for (const listener of this.openListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('close', (event) => {\n")
	b.WriteString("      this.status = 'closed';\n")
	b.WriteString("      this.lastClose = event;\n")
	b.WriteString("      this.closedAt = new Date();\n")
	b.WriteString("      for (const listener of this.closeListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("    this.socket.addEventListener('error', (event) => {\n")
	b.WriteString("      this.lastError = event;\n")
	b.WriteString("      for (const listener of this.errorListeners) listener(event);\n")
	b.WriteString("    });\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Current WebSocket readyState.\n")
	b.WriteString("   * 当前 WebSocket 连接状态。\n")
	b.WriteString("   */\n")
	b.WriteString("  get readyState(): number {\n")
	b.WriteString("    return this.socket.readyState;\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Whether the socket is currently open.\n")
	b.WriteString("   * 当前连接是否处于打开状态。\n")
	b.WriteString("   */\n")
	b.WriteString("  get isOpen(): boolean {\n")
	b.WriteString("    return this.readyState === WebSocket.OPEN;\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Send one typed message.\n")
	b.WriteString("   * 发送一条类型化消息。\n")
	b.WriteString("   */\n")
	b.WriteString("  send(message: TSend): void {\n")
	b.WriteString("    const data = this.serialize(message);\n")
	b.WriteString("    this.socket.send(JSON.stringify(data));\n")
	b.WriteString("    this.messagesSent += 1;\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Close the websocket connection.\n")
	b.WriteString("   * 主动关闭 websocket 连接。\n")
	b.WriteString("   */\n")
	b.WriteString("  close(): void {\n")
	b.WriteString("    this.status = 'closing';\n")
	b.WriteString("    this.socket.close();\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to all incoming messages.\n")
	b.WriteString("   * 订阅所有接收到的消息。\n")
	b.WriteString("   */\n")
	b.WriteString("  onMessage(handler: (message: TReceive) => void): () => void {\n")
	b.WriteString("    this.messageListeners.add(handler);\n")
	b.WriteString("    return () => this.messageListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to websocket open event.\n")
	b.WriteString("   * 订阅 websocket 打开事件。\n")
	b.WriteString("   */\n")
	b.WriteString("  onOpen(handler: (event: Event) => void): () => void {\n")
	b.WriteString("    this.openListeners.add(handler);\n")
	b.WriteString("    return () => this.openListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to websocket close event.\n")
	b.WriteString("   * 订阅 websocket 关闭事件。\n")
	b.WriteString("   */\n")
	b.WriteString("  onClose(handler: (event: CloseEvent) => void): () => void {\n")
	b.WriteString("    this.closeListeners.add(handler);\n")
	b.WriteString("    return () => this.closeListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to websocket error event.\n")
	b.WriteString("   * 订阅 websocket 错误事件。\n")
	b.WriteString("   */\n")
	b.WriteString("  onError(handler: (event: Event) => void): () => void {\n")
	b.WriteString("    this.errorListeners.add(handler);\n")
	b.WriteString("    return () => this.errorListeners.delete(handler);\n")
	b.WriteString("  }\n\n")
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to messages by the `type` field.\n")
	b.WriteString("   * 按消息的 `type` 字段进行订阅。\n")
	b.WriteString("   */\n")
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
	b.WriteString("  /**\n")
	b.WriteString("   * Subscribe to typed payload messages with optional select/validate/decode steps.\n")
	b.WriteString("   * 订阅类型化 payload 消息，并可通过 select/validate/decode 进行处理。\n")
	b.WriteString("   */\n")
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
	writeTSMarkerEnd(&b, "Typed WebSocket Client")

	if len(registry.defs) > 0 {
		writeTSMarker(&b, "Interfaces & Validators")
		b.WriteString("// =====================================================\n")
		b.WriteString("// INTERFACES & VALIDATORS\n")
		b.WriteString("// Default: object schemas use interface.\n")
		b.WriteString("// Fallback: use type only when interface cannot model the shape.\n")
		b.WriteString("// 默认：对象结构使用 interface。\n")
		b.WriteString("// 兜底：只有 interface 无法表达时才使用 type。\n")
		b.WriteString("// =====================================================\n\n")
	}
	sortedDefs := append([]tsInterfaceDef(nil), registry.defs...)
	sort.Slice(sortedDefs, func(i, j int) bool {
		return sortedDefs[i].Name < sortedDefs[j].Name
	})
	for _, def := range sortedDefs {
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
	if len(registry.defs) > 0 {
		writeTSMarkerEnd(&b, "Interfaces & Validators")
	}

	writeTSMarker(&b, "Endpoint Classes")
	normalizedBasePath := normalizePathSegment(basePath)
	normalizedGroupPath := normalizePathSegment(groupPath)
	fullPathPrefix := resolveAPIPath(normalizedBasePath, normalizedGroupPath)
	for _, m := range metas {
		className := toUpperCamel(m.FuncName)
		messageTypeAlias := className + "MessageType"
		serverPayloadMapAlias := className + "ServerPayloadByType"
		clientPayloadMapAlias := className + "ClientPayloadByType"
		receiveUnionAlias := className + "ReceiveUnion"
		sendUnionAlias := className + "SendUnion"
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
		b.WriteString(className)
		b.WriteString("MessageType = ")
		b.WriteString(renderMessageTypeUnion(m.MessageTypes))
		b.WriteString(";\n")
		if len(m.ServerPayloadByType) > 0 {
			b.WriteString("export interface ")
			b.WriteString(serverPayloadMapAlias)
			b.WriteString(" {\n")
			for _, mt := range sortMessageTypesByDeclaredOrder(m.MessageTypes, m.ServerPayloadByType) {
				b.WriteString("  ")
				b.WriteString(strconv.Quote(mt))
				b.WriteString(": ")
				b.WriteString(m.ServerPayloadByType[mt])
				b.WriteString(";\n")
			}
			b.WriteString("}\n")
		}
		if len(m.ClientPayloadByType) > 0 {
			b.WriteString("export interface ")
			b.WriteString(clientPayloadMapAlias)
			b.WriteString(" {\n")
			for _, mt := range sortMessageTypesByDeclaredOrder(m.MessageTypes, m.ClientPayloadByType) {
				b.WriteString("  ")
				b.WriteString(strconv.Quote(mt))
				b.WriteString(": ")
				b.WriteString(m.ClientPayloadByType[mt])
				b.WriteString(";\n")
			}
			b.WriteString("}\n")
		}
		if len(m.ServerPayloadByType) > 0 {
			b.WriteString("export type ")
			b.WriteString(receiveUnionAlias)
			b.WriteString(" = ")
			b.WriteString(renderTypePayloadUnion(m.MessageTypes, m.ServerPayloadByType))
			b.WriteString(";\n")
		}
		if len(m.ClientPayloadByType) > 0 {
			b.WriteString("export type ")
			b.WriteString(sendUnionAlias)
			b.WriteString(" = ")
			b.WriteString(renderTypePayloadUnion(m.MessageTypes, m.ClientPayloadByType))
			b.WriteString(";\n")
		}
		b.WriteString("export class ")
		b.WriteString(className)
		b.WriteString("<TSend = ")
		b.WriteString(m.ClientType)
		b.WriteString("> extends TypedWebSocketClient<")
		b.WriteString(m.ServerType)
		b.WriteString(", TSend, ")
		b.WriteString(messageTypeAlias)
		b.WriteString("> {\n")
		b.WriteString("  static readonly NAME = '")
		b.WriteString(strings.ReplaceAll(m.FuncName, "'", "\\'"))
		b.WriteString("' as const;\n")
		b.WriteString("  static readonly PATHS = {\n")
		b.WriteString("    base: '")
		b.WriteString(strings.ReplaceAll(normalizedBasePath, "'", "\\'"))
		b.WriteString("',\n")
		b.WriteString("    group: '")
		b.WriteString(strings.ReplaceAll(normalizedGroupPath, "'", "\\'"))
		b.WriteString("',\n")
		b.WriteString("    api: '")
		b.WriteString(strings.ReplaceAll(m.Path, "'", "\\'"))
		b.WriteString("',\n")
		b.WriteString("  } as const;\n")
		b.WriteString("  static readonly FULL_PATH = '")
		b.WriteString(strings.ReplaceAll(joinURLPath(fullPathPrefix, m.Path), "'", "\\'"))
		b.WriteString("' as const;\n")
		b.WriteString("  static readonly MESSAGE_TYPES = [")
		for i, t := range m.MessageTypes {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString("'")
			b.WriteString(strings.ReplaceAll(t, "'", "\\'"))
			b.WriteString("'")
		}
		b.WriteString("] as const;\n")
		b.WriteString("  public readonly endpointName = ")
		b.WriteString(className)
		b.WriteString(".NAME;\n")
		b.WriteString("  public readonly endpointPath = ")
		b.WriteString(className)
		b.WriteString(".FULL_PATH;\n\n")
		b.WriteString("  constructor(options: WebSocketConvertOptions<TSend, ")
		b.WriteString(m.ServerType)
		b.WriteString(">) {\n")
		b.WriteString("    const url = ")
		b.WriteString(className)
		b.WriteString(".FULL_PATH;\n")
		b.WriteString("    super(url, options);\n")
		b.WriteString("  }\n\n")
		if len(m.ServerPayloadByType) > 0 {
			b.WriteString("  onTypedMessage<TType extends ")
			b.WriteString(messageTypeAlias)
			b.WriteString(">(\n")
			b.WriteString("    type: TType,\n")
			b.WriteString("    handler: (message: Extract<")
			b.WriteString(receiveUnionAlias)
			b.WriteString(", { type: TType }>) => void,\n")
			b.WriteString("    options?: TypeHandlerOptions<")
			b.WriteString(m.ServerType)
			b.WriteString(">\n")
			b.WriteString("  ): () => void {\n")
			b.WriteString("    return this.onType(type, (message) => handler(message as unknown as Extract<")
			b.WriteString(receiveUnionAlias)
			b.WriteString(", { type: TType }>), options);\n")
			b.WriteString("  }\n\n")
		}
		if len(m.ClientPayloadByType) > 0 {
			b.WriteString("  sendTypedMessage(message: ")
			b.WriteString(sendUnionAlias)
			b.WriteString("): void {\n")
			b.WriteString("    this.send(message as TSend);\n")
			b.WriteString("  }\n\n")
		}
		for _, mt := range m.MessageTypes {
			methodSuffix := wsMessageTypeMethodSuffix(mt)
			serverPayloadType := "unknown"
			if v, ok := m.ServerPayloadByType[mt]; ok && strings.TrimSpace(v) != "" {
				serverPayloadType = v
			}
			clientPayloadType := "unknown"
			if v, ok := m.ClientPayloadByType[mt]; ok && strings.TrimSpace(v) != "" {
				clientPayloadType = v
			}
			b.WriteString("  /**\n")
			b.WriteString("   * Subscribe to messages with type ")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" for ")
			b.WriteString(className)
			b.WriteString(".\n")
			b.WriteString("   * 订阅 ")
			b.WriteString(className)
			b.WriteString(" 中 type=")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" 的完整消息。\n")
			b.WriteString("   */\n")
			b.WriteString("  on")
			b.WriteString(methodSuffix)
			b.WriteString("Type(\n")
			b.WriteString("    handler: (message: ")
			if serverPayloadType == "unknown" {
				b.WriteString(m.ServerType)
			} else {
				b.WriteString("{ type: ")
				b.WriteString(strconv.Quote(mt))
				b.WriteString("; payload: ")
				b.WriteString(serverPayloadType)
				b.WriteString(" }")
			}
			b.WriteString(") => void,\n")
			b.WriteString("    options?: TypeHandlerOptions<")
			b.WriteString(m.ServerType)
			b.WriteString(">\n")
			b.WriteString("  ): () => void {\n")
			b.WriteString("    if (options === undefined) {\n")
			b.WriteString("      options = { validate: validate")
			b.WriteString(m.ServerType)
			b.WriteString(" };\n")
			b.WriteString("    }\n")
			b.WriteString("    return this.onType(")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" as ")
			b.WriteString(messageTypeAlias)
			b.WriteString(", (message) => handler(message as unknown as ")
			if serverPayloadType == "unknown" {
				b.WriteString(m.ServerType)
			} else {
				b.WriteString("{ type: ")
				b.WriteString(strconv.Quote(mt))
				b.WriteString("; payload: ")
				b.WriteString(serverPayloadType)
				b.WriteString(" }")
			}
			b.WriteString("), options);\n")
			b.WriteString("  }\n\n")
			b.WriteString("  /**\n")
			b.WriteString("   * Subscribe to payload of messages with type ")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" for ")
			b.WriteString(className)
			b.WriteString(".\n")
			b.WriteString("   * 订阅 ")
			b.WriteString(className)
			b.WriteString(" 中 type=")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" 的 payload，并可通过 options 做选择、校验与解码。\n")
			b.WriteString("   */\n")
			b.WriteString("  on")
			b.WriteString(methodSuffix)
			b.WriteString("Payload(\n")
			b.WriteString("    handler: (payload: ")
			b.WriteString(serverPayloadType)
			b.WriteString(", message: ")
			b.WriteString(m.ServerType)
			b.WriteString(") => void,\n")
			b.WriteString("    options?: TypedHandlerOptions<")
			b.WriteString(m.ServerType)
			b.WriteString(", ")
			b.WriteString(serverPayloadType)
			b.WriteString(">\n")
			b.WriteString("\n")
			b.WriteString("  ): () => void {\n")
			b.WriteString("    if (options === undefined) {\n")
			b.WriteString("      function defaultValidatePayload(_payload: unknown, message: ")
			b.WriteString(m.ServerType)
			b.WriteString("): boolean {\n")
			b.WriteString("        return validate")
			b.WriteString(m.ServerType)
			b.WriteString("(message);\n")
			b.WriteString("      }\n")
			b.WriteString("      options = { validate: defaultValidatePayload };\n")
			b.WriteString("    }\n")
			b.WriteString("    return this.onTyped<")
			b.WriteString(serverPayloadType)
			b.WriteString(">(")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" as ")
			b.WriteString(messageTypeAlias)
			b.WriteString(", handler, options);\n")
			b.WriteString("  }\n\n")
			b.WriteString("  /**\n")
			b.WriteString("   * Send payload with fixed message type ")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(".\n")
			b.WriteString("   * 发送固定 type=")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(" 的 payload。\n")
			b.WriteString("   */\n")
			b.WriteString("  send")
			b.WriteString(methodSuffix)
			b.WriteString("Payload(payload: ")
			b.WriteString(clientPayloadType)
			b.WriteString("): void {\n")
			b.WriteString("    this.send({ type: ")
			b.WriteString(strconv.Quote(mt))
			b.WriteString(", payload } as TSend);\n")
			b.WriteString("  }\n\n")
		}
		b.WriteString("}\n")
		b.WriteString("export function create")
		b.WriteString(className)
		b.WriteString("<TSend = ")
		b.WriteString(m.ClientType)
		b.WriteString(">(options: WebSocketConvertOptions<TSend, ")
		b.WriteString(m.ServerType)
		b.WriteString(">): ")
		b.WriteString(className)
		b.WriteString("<TSend> {\n")
		b.WriteString("  return new ")
		b.WriteString(className)
		b.WriteString("<TSend>(options);\n")
		b.WriteString("}\n")
		b.WriteString("\n")
	}
	writeTSMarkerEnd(&b, "Endpoint Classes")

	return finalizeTypeScriptCode(b.String()), nil
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

func renderTypePayloadUnion(types []string, payloadByType map[string]string) string {
	orderedTypes := sortMessageTypesByDeclaredOrder(types, payloadByType)
	if len(orderedTypes) == 0 {
		return "never"
	}
	parts := make([]string, 0, len(orderedTypes))
	for _, mt := range orderedTypes {
		payloadType := strings.TrimSpace(payloadByType[mt])
		if payloadType == "" {
			payloadType = "unknown"
		}
		parts = append(parts, "{ type: "+strconv.Quote(mt)+"; payload: "+payloadType+" }")
	}
	return strings.Join(parts, " | ")
}

func wsMessageTypeMethodSuffix(messageType string) string {
	parts := strings.FieldsFunc(messageType, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		return true
	})
	base := toUpperCamel(strings.Join(parts, " "))
	if base == "" {
		return "Message"
	}
	first := base[0]
	if first >= '0' && first <= '9' {
		return "Type" + base
	}
	return base
}

func sortMessageTypesByDeclaredOrder(declared []string, payloadMap map[string]string) []string {
	if len(payloadMap) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(payloadMap))
	for _, mt := range declared {
		if _, ok := payloadMap[mt]; !ok {
			continue
		}
		if _, ok := seen[mt]; ok {
			continue
		}
		seen[mt] = struct{}{}
		out = append(out, mt)
	}
	rest := make([]string, 0, len(payloadMap)-len(out))
	for mt := range payloadMap {
		if _, ok := seen[mt]; ok {
			continue
		}
		rest = append(rest, mt)
	}
	sort.Strings(rest)
	out = append(out, rest...)
	return out
}
