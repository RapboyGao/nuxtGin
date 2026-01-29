package utils

import (
	"time"

	"github.com/araddon/dateparse"
)

// ParseTimeAny parses a date/time string with dateparse.
func ParseTimeAny(value string) (time.Time, error) {
	return dateparse.ParseAny(value)
}
