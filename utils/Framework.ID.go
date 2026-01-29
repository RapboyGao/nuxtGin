package utils

import (
	"github.com/google/uuid"
	"github.com/rs/xid"
)

// NewUUID returns a RFC 4122 UUID string.
func NewUUID() string {
	return uuid.NewString()
}

// ParseUUID validates and parses a UUID string.
func ParseUUID(value string) (uuid.UUID, error) {
	return uuid.Parse(value)
}

// NewXID returns a lexicographically sortable XID string.
func NewXID() string {
	return xid.New().String()
}
