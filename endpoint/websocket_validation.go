package endpoint

import (
	"fmt"
	"strings"
)

func validateWebSocketPayloadTypeMappings(meta WebSocketEndpointMeta) error {
	if len(meta.MessageTypes) == 0 {
		return nil
	}

	clientMap := meta.ClientPayloadTypes
	serverMap := meta.ServerPayloadTypes

	for _, rawType := range meta.MessageTypes {
		msgType := strings.TrimSpace(rawType)
		if msgType == "" {
			continue
		}
		if clientMap == nil {
			return fmt.Errorf("client payload map is required when MessageTypes is set (missing %q)", msgType)
		}
		if serverMap == nil {
			return fmt.Errorf("server payload map is required when MessageTypes is set (missing %q)", msgType)
		}
		if _, ok := clientMap[msgType]; !ok {
			return fmt.Errorf("client payload type is required for message type %q", msgType)
		}
		if _, ok := serverMap[msgType]; !ok {
			return fmt.Errorf("server payload type is required for message type %q", msgType)
		}
	}
	return nil
}
