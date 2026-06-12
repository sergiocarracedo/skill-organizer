package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// exampleRe matches the first ```json ... ``` fenced block in
// OBSERVABILITY.md. The example block is the contract for the wire
// format; tests below assert the recorder's output matches.
var exampleRe = regexp.MustCompile("(?s)```json\n(.*?)\n```")

// sectionHeaders are the 6 required sections of OBSERVABILITY.md.
// TestOBSERVABILITYHasAllSixSections asserts each is present.
var sectionHeaders = []string{
	"## What is collected",
	"## Schema",
	"## How to enable / disable",
	"## Data retention",
	"## Privacy guarantees",
	"## FAQ",
}

// findObservabilityMD walks up from the current working directory
// until OBSERVABILITY.md is found. The file lives at the repo root;
// the test may run from packages/cli/ (under go test ./...) or
// packages/cli/internal/telemetry/ (under go test ./internal/...),
// so we walk up to handle both.
func findObservabilityMD(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v", err)
	}
	for {
		candidate := filepath.Join(dir, "OBSERVABILITY.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("OBSERVABILITY.md not found by walking up from %q", dir)
		}
		dir = parent
	}
}

// readExampleBlock returns the JSON object from the first ```json ```
// fence in OBSERVABILITY.md. The block is the source of truth for
// the wire format.
func readExampleBlock(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v", path, err)
	}
	match := exampleRe.FindStringSubmatch(string(content))
	if len(match) < 2 {
		t.Fatalf("no ```json``` block found in %s", path)
	}
	return match[1]
}

// TestOBSERVABILITYExampleMatchesEmitted parses the example block in
// OBSERVABILITY.md, asserts the 5 schema fields are present and
// match the emitted Event shape, then emits a real Event through
// the recorder and asserts the volatile fields also match their
// regexes (the values are fresh, the shape is fixed).
//
// Phase 5 (REQ-10) deviation: OBSERVABILITY.md is updated in
// plan 05-03 to the 5-field schema. Until then, the example block
// in the doc may carry install_id/host_id from the Phase 3
// schema. This test asserts the 5 common fields (the ones present
// in both the example and the emitted event); the dropped identity
// fields are tested separately in TestEventHasNoIdentityFields and
// TestService_RecordEvent_NoIdentityFields.
func TestOBSERVABILITYExampleMatchesEmitted(t *testing.T) {
	path := findObservabilityMD(t)
	example := readExampleBlock(t, path)

	var exampleMap map[string]any
	if err := json.Unmarshal([]byte(example), &exampleMap); err != nil {
		t.Fatalf("example block is not valid JSON: %v\nblock:\n%s", err, example)
	}

	// The 5 schema fields must be present in the example.
	// (OBSERVABILITY.md may carry legacy 7-field keys until
	// plan 05-03; the 5 common keys are the contract.)
	wantCommonKeys := []string{"command", "exit_status", "timestamp", "version", "event_id"}
	for _, k := range wantCommonKeys {
		if _, ok := exampleMap[k]; !ok {
			t.Fatalf("example missing key %q (block:\n%s)", k, example)
		}
	}

	// Deterministic keys: must be present with the expected types.
	if _, ok := exampleMap["command"].(string); !ok {
		t.Fatalf("example command is not a string (got %T)", exampleMap["command"])
	}
	if _, ok := exampleMap["exit_status"].(float64); !ok {
		t.Fatalf("example exit_status is not a number (got %T)", exampleMap["exit_status"])
	}
	if _, ok := exampleMap["version"].(string); !ok {
		t.Fatalf("example version is not a string (got %T)", exampleMap["version"])
	}

	// Volatile keys: must be present and match the regex shapes
	// (not the example values, which are placeholders).
	ulidRe := regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)
	tsRe := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

	if v, ok := exampleMap["event_id"].(string); !ok || !ulidRe.MatchString(v) {
		t.Fatalf("example event_id = %v, want 26-char ULID", exampleMap["event_id"])
	}
	if v, ok := exampleMap["timestamp"].(string); !ok || !tsRe.MatchString(v) {
		t.Fatalf("example timestamp = %v, want RFC3339 UTC", exampleMap["timestamp"])
	}

	// Build a real Event and marshal it; assert the volatile fields
	// match the regexes (shape) and the deterministic fields match
	// the example's values byte-for-byte.
	emitted := validEvent()
	emittedBody, err := json.Marshal(emitted)
	if err != nil {
		t.Fatalf("json.Marshal(emitted) = %v", err)
	}
	var emittedMap map[string]any
	if err := json.Unmarshal(emittedBody, &emittedMap); err != nil {
		t.Fatalf("emitted body is not valid JSON: %v", err)
	}
	// The emitted event must have exactly 5 keys (the 5 schema
	// fields). The dropped identity fields must NOT be present.
	if len(emittedMap) != 5 {
		t.Fatalf("emitted keys count = %d, want 5 (got %v)", len(emittedMap), emittedMap)
	}
	for _, banned := range []string{"install_id", "host_id"} {
		if _, present := emittedMap[banned]; present {
			t.Fatalf("emitted body must NOT contain %q (Phase 5 REQ-10 drops both identity fields)", banned)
		}
	}
	if emittedMap["command"] != exampleMap["command"] {
		t.Fatalf("emitted command = %v, example command = %v (must match byte-for-byte)", emittedMap["command"], exampleMap["command"])
	}
	if emittedMap["exit_status"] != exampleMap["exit_status"] {
		t.Fatalf("emitted exit_status = %v, example exit_status = %v (must match byte-for-byte)", emittedMap["exit_status"], exampleMap["exit_status"])
	}
	if emittedMap["version"] != exampleMap["version"] {
		t.Fatalf("emitted version = %v, example version = %v (must match byte-for-byte)", emittedMap["version"], exampleMap["version"])
	}
	if v, ok := emittedMap["event_id"].(string); !ok || !ulidRe.MatchString(v) {
		t.Fatalf("emitted event_id = %v, want 26-char ULID", emittedMap["event_id"])
	}
	if v, ok := emittedMap["timestamp"].(string); !ok || !tsRe.MatchString(v) {
		t.Fatalf("emitted timestamp = %v, want RFC3339 UTC", emittedMap["timestamp"])
	}
}

// TestOBSERVABILITYHasAllSixSections guards against a refactor
// that accidentally drops a section from OBSERVABILITY.md. The 6
// sections are the contract documented in
// .planning/phases/05-local-only-anonymous-telemetry/05-CONTEXT.md.
func TestOBSERVABILITYHasAllSixSections(t *testing.T) {
	path := findObservabilityMD(t)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v", path, err)
	}
	body := string(content)
	for _, header := range sectionHeaders {
		if !strings.Contains(body, header) {
			t.Fatalf("OBSERVABILITY.md missing section %q\nfile: %s", header, path)
		}
	}
}

// TestOBSERVABILITYLinksToPRIVACY asserts the OBSERVABILITY.md
// file links to PRIVACY.md. The link should appear near the top
// of the file as a one-liner cross-reference.
func TestOBSERVABILITYLinksToPRIVACY(t *testing.T) {
	path := findObservabilityMD(t)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) = %v", path, err)
	}
	if !strings.Contains(string(content), "PRIVACY.md") {
		t.Fatalf("OBSERVABILITY.md does not reference PRIVACY.md\nfile: %s", path)
	}
}

// TestOBSERVABILITYExampleIsValidJSON asserts the JSON example
// block parses as JSON. An invalid example is a documentation
// bug; the schema test in TestOBSERVABILITYExampleMatchesEmitted
// builds on this assumption.
func TestOBSERVABILITYExampleIsValidJSON(t *testing.T) {
	path := findObservabilityMD(t)
	example := readExampleBlock(t, path)
	if !json.Valid([]byte(example)) {
		t.Fatalf("example block is not valid JSON:\n%s", example)
	}
}
