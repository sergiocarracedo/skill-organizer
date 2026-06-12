package telemetry

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

// validEvent returns an Event that passes Validate. Tests mutate one
// field at a time to assert the validator rejects malformed inputs.
func validEvent() Event {
	return Event{
		Command:    "check-security",
		ExitStatus: 0,
		InstallID:  "0123456789abcdef0123456789abcdef",
		HostID:     "0123456789abcdef0123456789abcdef",
		Timestamp:  "2026-06-11T00:00:00Z",
		Version:    "0.4.0",
		EventID:    "01HXYZABCDEFGHJKMNPQRSTVWX",
	}
}

func TestEventValidateAcceptsValid(t *testing.T) {
	e := validEvent()
	if err := e.Validate(); err != nil {
		t.Fatalf("validEvent().Validate() = %v, want nil", err)
	}
}

func TestEventValidateFields(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Event)
		wantSub string
	}{
		{
			name:    "empty command",
			mutate:  func(e *Event) { e.Command = "" },
			wantSub: "command is required",
		},
		{
			name:    "invalid exit_status",
			mutate:  func(e *Event) { e.ExitStatus = 2 },
			wantSub: "exit_status must be 0 or 1",
		},
		{
			name:    "malformed install_id",
			mutate:  func(e *Event) { e.InstallID = "not-hex" },
			wantSub: "install_id must be 32 hex chars",
		},
		{
			name:    "malformed timestamp",
			mutate:  func(e *Event) { e.Timestamp = "yesterday" },
			wantSub: "timestamp must be RFC3339 UTC",
		},
		{
			name:    "malformed event_id",
			mutate:  func(e *Event) { e.EventID = "abc" },
			wantSub: "event_id must be a 26-char ULID",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			e := validEvent()
			tc.mutate(&e)
			err := e.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("Validate() = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

func TestEventValidateRejectsBadHostID(t *testing.T) {
	e := validEvent()
	e.HostID = "garbage"
	err := e.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "host_id must be 32 hex chars") {
		t.Fatalf("Validate() = %q, want it to mention host_id", err.Error())
	}
}

func TestEventValidateRejectsEmptyVersion(t *testing.T) {
	e := validEvent()
	e.Version = ""
	err := e.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("Validate() = %q, want it to mention version", err.Error())
	}
}

func TestEventJSONShape(t *testing.T) {
	e := validEvent()
	body, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}

	// Unmarshal into a generic map to assert the key set and types.
	var asMap map[string]any
	if err := json.Unmarshal(body, &asMap); err != nil {
		t.Fatalf("json.Unmarshal() = %v", err)
	}

	wantKeys := []string{
		"command", "exit_status", "install_id", "host_id",
		"timestamp", "version", "event_id",
	}
	if len(asMap) != len(wantKeys) {
		t.Fatalf("json keys count = %d, want %d (got %v)", len(asMap), len(wantKeys), asMap)
	}
	for _, k := range wantKeys {
		if _, ok := asMap[k]; !ok {
			t.Fatalf("json missing key %q (body = %s)", k, body)
		}
	}

	// Type checks: most are strings, exit_status is a number.
	stringKeys := []string{"command", "install_id", "host_id", "timestamp", "version", "event_id"}
	for _, k := range stringKeys {
		v, ok := asMap[k].(string)
		if !ok {
			t.Fatalf("json key %q is not a string (got %T)", k, asMap[k])
		}
		if v == "" {
			t.Fatalf("json key %q is an empty string", k)
		}
	}
	if _, ok := asMap["exit_status"].(float64); !ok {
		t.Fatalf("json key %q is not a number (got %T)", "exit_status", asMap["exit_status"])
	}

	// Assert field declaration order: encoding/json marshals struct
	// fields in declaration order, so the byte offsets of the keys
	// must be monotonically increasing in the expected order.
	expected := []string{`"command":`, `"exit_status":`, `"install_id":`, `"host_id":`, `"timestamp":`, `"version":`, `"event_id":`}
	cursor := 0
	for _, marker := range expected {
		idx := strings.Index(string(body)[cursor:], marker)
		if idx < 0 {
			t.Fatalf("json body missing marker %q after offset %d (body = %s)", marker, cursor, body)
		}
		cursor += idx + len(marker)
	}
}

func TestNewEventIDProducesValidFormat(t *testing.T) {
	ulidRegex := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := NewEventID()
		if len(id) != 26 {
			t.Fatalf("NewEventID() len = %d, want 26 (id = %q)", len(id), id)
		}
		if !ulidRegex.MatchString(id) {
			t.Fatalf("NewEventID() = %q, does not match ULID Crockford base32 regex", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewEventID() returned duplicate %q on iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}

func TestNewTimestampFormat(t *testing.T) {
	ts := NewTimestamp()
	pattern := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	if !pattern.MatchString(ts) {
		t.Fatalf("NewTimestamp() = %q, does not match RFC3339 UTC shape", ts)
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("time.Parse(time.RFC3339, %q) = %v", ts, err)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("parsed timestamp location = %v, want UTC", parsed.Location())
	}
}
