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
		_, hasClient := clientMap[msgType]
		_, hasServer := serverMap[msgType]
		if !hasClient && !hasServer {
			return fmt.Errorf("payload type is required for message type %q in either client or server map", msgType)
		}
	}
	return nil
}
