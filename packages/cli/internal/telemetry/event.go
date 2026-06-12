// Package telemetry records anonymous, opt-in command-invocation events.
// See OBSERVABILITY.md at the repo root for the schema, opt-in flow,
// and data retention policy.
package telemetry

import (
	"fmt"
	"regexp"
	"time"

	"github.com/oklog/ulid/v2"
)

// Event is the JSON-serializable record of a single command invocation.
// It carries no arguments, no paths, and no PII. The seven fields are
// the entire payload; the server schema matches these names exactly.
//
// The JSON field order is fixed by struct declaration order, which
// encoding/json honours. Reordering the fields will break the
// byte-for-byte schema test.
type Event struct {
	Command    string `json:"command"`
	ExitStatus int    `json:"exit_status"`
	InstallID  string `json:"install_id"`
	HostID     string `json:"host_id"`
	Timestamp  string `json:"timestamp"`
	Version    string `json:"version"`
	EventID    string `json:"event_id"`
}

var (
	idHexRe     = regexp.MustCompile(`^[0-9a-f]{32}$`)
	ulidRe      = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	timestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
)

// Validate returns an error if any field does not match the schema
// documented in OBSERVABILITY.md. Callers should Validate before
// sending or buffering an event so malformed data is rejected at the
// source rather than downstream by the server or the JSONL drain.
func (e *Event) Validate() error {
	if e.Command == "" {
		return fmt.Errorf("event: command is required")
	}
	if e.ExitStatus != 0 && e.ExitStatus != 1 {
		return fmt.Errorf("event: exit_status must be 0 or 1, got %d", e.ExitStatus)
	}
	if !idHexRe.MatchString(e.InstallID) {
		return fmt.Errorf("event: install_id must be 32 hex chars, got %q", e.InstallID)
	}
	if !idHexRe.MatchString(e.HostID) {
		return fmt.Errorf("event: host_id must be 32 hex chars, got %q", e.HostID)
	}
	if !timestampRe.MatchString(e.Timestamp) {
		return fmt.Errorf("event: timestamp must be RFC3339 UTC, got %q", e.Timestamp)
	}
	if e.Version == "" {
		return fmt.Errorf("event: version is required")
	}
	if !ulidRe.MatchString(e.EventID) {
		return fmt.Errorf("event: event_id must be a 26-char ULID, got %q", e.EventID)
	}
	return nil
}

// NewEventID returns a fresh 26-character Crockford base32 ULID.
// Uses ulid.Make, which combines the current Unix-millisecond
// timestamp with 80 bits of entropy from the default monotonic
// reader (crypto/rand on every supported platform).
func NewEventID() string {
	return ulid.Make().String()
}

// NewTimestamp returns the current time formatted as RFC3339 UTC
// (e.g. "2026-06-11T12:34:56Z"). The format omits sub-second
// precision, matching the regex the schema validation requires.
func NewTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
