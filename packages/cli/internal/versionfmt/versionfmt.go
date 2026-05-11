package versionfmt

import (
	"strings"
	"time"
)

func DisplayVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if isHexHash(trimmed) && len(trimmed) > 7 {
		return trimmed[:7]
	}
	return trimmed
}

func DisplayDate(value string) string {
	parsed := parseLooseDate(value)
	if parsed.IsZero() {
		return ""
	}
	return parsed.UTC().Format("2006-01-02")
}

func DisplayTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02")
}

func isHexHash(value string) bool {
	if len(value) < 8 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func parseLooseDate(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "January 2006", "Jan 2006", "2006-01-02"} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}
