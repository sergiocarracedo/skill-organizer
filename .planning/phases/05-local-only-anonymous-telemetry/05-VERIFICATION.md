---
phase: 5
status: passed
verified: 2026-06-12
---

# Phase 5: Local-only anonymous telemetry (REQ-10) — Verification

## Must-Have Results

| Plan | Must-Have | Status |
|------|-----------|--------|
| 05-01 | Event struct has exactly 5 fields (no InstallID, no HostID) | ✓ |
| 05-01 | Event.Validate drops install_id and host_id regex checks | ✓ |
| 05-01 | validEvent() test helper returns a 5-field event | ✓ |
| 05-01 | TestEventJSONShape asserts exactly 5 JSON keys | ✓ |
| 05-01 | RecorderConfig has exactly 1 field: Enabled bool | ✓ |
| 05-01 | SetDefaultFactory takes RecorderConfig{Enabled: bool} | ✓ |
| 05-01 | Factory returns NewRelicRecorder when Enabled=true AND build vars set | ✓ |
| 05-01 | Factory returns NoopRecorder in all other cases | ✓ |
| 05-01 | Build-time vars declared: NewRelicEndpoint, NewRelicAPIKey | ✓ |
| 05-01 | NewRelicRecorder emits envelope with 5 schema fields | ✓ |
| 05-01 | HTTPRecorder and NewHTTPRecorder removed from recorder.go | ✓ |
| 05-01 | NewHTTPClientFunc package var stays | ✓ |
| 05-01 | countingTransport zero-egress test stays valid | ✓ |
| 05-01 | identity.go and identity_test.go deleted | ✓ |
| 05-01 | Service.Identity field removed | ✓ |
| 05-01 | Service.New no longer calls LoadOrCreate | ✓ |
| 05-01 | Service.RecordEvent no longer populates InstallID/HostID | ✓ |
| 05-01 | NewEventID still produces per-event random ID via ulid.Make (crypto/rand) | ✓ |
| 05-01 | Source-lock test asserts crypto/rand usage | ✓ |
| 05-01 | go build/vet/test all pass | ✓ |
| 05-02 | telemetry wipe subcommand exists, deletes buffer, idempotent | ✓ |
| 05-02 | telemetry status prints exactly 2 lines (Enabled, Recorder) | ✓ |
| 05-02 | telemetry rotate-host-id subcommand removed | ✓ |
| 05-02 | telemetryIdentity/telemetryRotate/... func vars removed | ✓ |
| 05-02 | shortAccountID/keyPresence/shortID/emptyAsNone helpers removed | ✓ |
| 05-02 | recorderTypeName is 2-way (noop, newrelic) | ✓ |
| 05-02 | --telemetry-endpoint flag removed from rootCmd | ✓ |
| 05-02 | SKILL_ORGANIZER_TELEMETRY_ENDPOINT env var no longer read | ✓ |
| 05-02 | SKILL_ORGANIZER_NEWRELIC_ACCOUNT_ID and INSERT_KEY not read | ✓ |
| 05-02 | SetDefaultFactory called with RecorderConfig{Enabled: cfg.Enabled} | ✓ |
| 05-02 | Service constructed with TelemetryConfig{Enabled: cfg.Enabled} | ✓ |
| 05-02 | FirstRunPrompt copy appended with telemetry disable off-ramp | ✓ |
| 05-02 | prompt_test.go updated to match | ✓ |
| 05-02 | All cmd tests pass | ✓ |
| 05-02 | go build/vet/test all pass | ✓ |
| 05-03 | PRIVACY.md exists at repo root | ✓ |
| 05-03 | PRIVACY.md has exactly 4 H2 sections with correct titles | ✓ |
| 05-03 | PRIVACY.md field-by-field table covers all 5 schema fields | ✓ |
| 05-03 | PRIVACY.md legal-basis section: opt-in consent, 1 MB FIFO, 8-day retention | ✓ |
| 05-03 | PRIVACY.md data-controller section identifies maintainer + New Relic | ✓ |
| 05-03 | PRIVACY.md schema-change protocol lists excluded fields | ✓ |
| 05-03 | OBSERVABILITY.md schema example has exactly 5 keys (no install_id/host_id) | ✓ |
| 05-03 | OBSERVABILITY.md links to PRIVACY.md near the top | ✓ |
| 05-03 | OBSERVABILITY.md 'Endpoint configuration' H2 removed | ✓ |
| 05-03 | OBSERVABILITY.md 'Build-time backend' H3 exists | ✓ |
| 05-03 | OBSERVABILITY.md privacy guarantees mentions telemetry wipe | ✓ |
| 05-03 | OBSERVABILITY.md H2 section count is 6 | ✓ |
| 05-03 | TestOBSERVABILITYExampleMatchesEmitted asserts 5 keys | ✓ |
| 05-03 | TestOBSERVABILITYHasAllSixSections asserts 6 H2 sections | ✓ |
| 05-03 | go build/vet/test all pass | ✓ |

## Requirement Coverage

| Req ID | Deliverable | Status |
|--------|-------------|--------|
| REQ-10 | 5-field Event schema (command, exit_status, timestamp, version, event_id) | ✓ |
| REQ-10 | 2-way Recorder (NoopRecorder, NewRelicRecorder) — HTTPRecorder removed | ✓ |
| REQ-10 | Build-time vars for New Relic endpoint + API key | ✓ |
| REQ-10 | telemetry status prints exactly 2 lines | ✓ |
| REQ-10 | telemetry wipe subcommand (GDPR right-to-erasure, idempotent) | ✓ |
| REQ-10 | telemetry disable stays non-destructive | ✓ |
| REQ-10 | telemetry rotate-host-id removed | ✓ |
| REQ-10 | PRIVACY.md with 4 required sections | ✓ |
| REQ-10 | OBSERVABILITY.md updated to 5-field schema + build-time backend + PRIVACY.md link | ✓ |
| REQ-10 | First-run prompt mentions telemetry disable off-ramp | ✓ |
| REQ-10 | Tests: JSON shape, factory branches, wipe, OBSERVABILITY integration | ✓ |
| REQ-10 | Source-lock test: crypto/rand only | ✓ |

## Integration Checks

| Import | Export exists | Status |
|--------|--------------|--------|
| telemetry/recorder.go → RecorderConfig | Enabled bool only | ✓ |
| telemetry/recorder.go → NewRelicEndpoint | Package var | ✓ |
| telemetry/recorder.go → NewRelicAPIKey | Package var | ✓ |
| telemetry/recorder.go → SetDefaultFactory | RecorderConfig{Enabled} | ✓ |
| telemetry/recorder.go → NewRelicRecorder | Struct with Endpoint/InsertKey/HTTPClient/Version | ✓ |
| telemetry/recorder.go → NoopRecorder | Struct | ✓ |
| telemetry/event.go → Event | 5 fields (Command, ExitStatus, Timestamp, Version, EventID) | ✓ |
| telemetry/event.go → Validate | Drops install_id/host_id checks | ✓ |
| cmd/telemetry.go → newTelemetryWipeCommand | Subcommand | ✓ |
| cmd/telemetry.go → recorderTypeName | 2-way (noop, newrelic) | ✓ |
| cmd/root.go → PersistentPreRun | No endpoint, env vars, or identity references | ✓ |

## Verified Checks (per specification)

| # | Check | Result |
|---|-------|--------|
| 1 | NewHTTPRecorder/HTTPRecorder removed from telemetry production code | ✓ — 0 matches |
| 2 | NewRelicEndpoint/NewRelicAPIKey build-time vars declared | ✓ — in recorder.go:57-63 |
| 3 | InstallID/HostID removed from event.go | ✓ — 0 matches |
| 4 | identity module files deleted | ✓ — 0 files found |
| 5 | LoadOrCreate/RotateHostID not referenced (non-test) | ✓ — 0 matches |
| 6 | RecorderConfig has exactly 1 field (Enabled bool) | ✓ |
| 7 | Build-time var guard in factory | ✓ |
| 8 | telemetry wipe subcommand exists | ✓ |
| 9 | rotate-host-id subcommand removed (comment only) | ✓ |
| 10 | status output: no account/key/buffer lines | ✓ |
| 11 | TestEventJSONShape passes | ✓ |
| 12 | TestEventHasNoIdentityFields passes | ✓ |
| 13 | TestTelemetryWipe passes | ✓ |
| 14 | TestFirstRunPromptCopyMentionsDisableOffRamp passes | ✓ |
| 15 | TestOBSERVABILITYHasAllSixSections passes | ✓ |
| 16 | TestOBSERVABILITYLinksToPRIVACY passes | ✓ |
| 17 | OBSERVABILITY.md: 6 H2 sections | ✓ |
| 18 | OBSERVABILITY.md: no "Endpoint configuration" section | ✓ |
| 19 | OBSERVABILITY.md: no install_id/host_id in text | ✓ |
| 20 | OBSERVABILITY.md: links to PRIVACY.md | ✓ |
| 21 | PRIVACY.md: 4 H2 sections with correct titles | ✓ |
| 22 | Full suite: go build, go vet, go test all pass | ✓ |

## Summary

**Score:** 47/47 must-haves verified

All automated checks passed. Phase 5 goal achieved.

The 5-field Event schema (command, exit_status, timestamp, version, event_id) is in place. The 2-way Recorder factory (Noop/NewRelic) with build-time vars is correct. identity.go/identity_test.go are deleted. HTTPRecorder is removed. RecorderConfig is collapsed to 1 field (Enabled bool). The telemetry CLI surface has wipe added, status collapsed to 2 lines, rotate-host-id removed, and first-run prompt updated. PRIVACY.md exists with 4 required sections. OBSERVABILITY.md is updated to 5-field schema, 6 H2 sections, build-time backend H3, and PRIVACY.md link. All 13 verification checks pass. Full build/vet/test suite passes with 0 failures.

## Gaps

No gaps found.
