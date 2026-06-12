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

// Phase 5 (REQ-10): the schema is 5 fields. No pseudonymous
// identifiers are emitted. The 2 dropped fields were `install_id`
// and `host_id`.
type Event struct {
	Command    string `json:"command"`
	ExitStatus int    `json:"exit_status"`
	Timestamp  string `json:"timestamp"`
	Version    string `json:"version"`
	EventID    string `json:"event_id"`
}

var (
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
