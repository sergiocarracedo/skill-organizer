// Package telemetry records anonymous, opt-in command-invocation events.
// The cmd package is the only intended caller for the public API below.
// See OBSERVABILITY.md at the repo root for the schema, opt-in flow,
// and data retention policy.
package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

// TelemetryConfig is the runtime configuration for the telemetry
// layer. It is a type alias of configpkg.TelemetryConfig so the cmd
// package can construct it from the same struct that holds the YAML
// persistence tags.
//
// Phase 5 (REQ-10): only the `Enabled` field is user-facing. The
// endpoint is no longer user-configurable; it is a build-time
// variable (NewRelicEndpoint) set via -ldflags. The
// configpkg.TelemetryConfig struct may still carry a deprecated
// `endpoint` field for YAML backwards compatibility, but the
// recorder factory ignores it — the build-time var is the only
// source of truth.
type TelemetryConfig = configpkg.TelemetryConfig

// promptSentinelFile marks that the first-run prompt has been answered.
// It is separate from TelemetryConfig.Enabled so the non-TTY fallback
// (Pitfall P10) can re-prompt on the next TTY run: the sentinel is
// only written when the user actually answered.
const promptSentinelFile = "telemetry-prompted"

// Service is the runtime telemetry object. One Service per CLI
// invocation. The cmd package constructs it in PersistentPreRun
// and uses it in PersistentPostRun.
type Service struct {
	AppDir   string
	Version  string
	Cfg      TelemetryConfig // for inspection; the recorder factory reads from a snapshot
	Recorder Recorder
	Buffer   *Buffer
}

// New builds a Service from a resolved TelemetryConfig and a resolved
// appDir. The factory must be set via SetDefaultFactory BEFORE New is
// called, so RecorderFactoryFunc is the current production closure.
//
// Phase 5 (REQ-10): no identity module. The Service no longer writes
// install_id or host_id files. Existing files left on disk from a
// prior version are ignored — the loader that read them is gone.
func New(appDir string, version string, cfg TelemetryConfig) (*Service, error) {
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, fmt.Errorf("create app dir: %w", err)
	}
	return &Service{
		AppDir:   appDir,
		Version:  version,
		Cfg:      cfg,
		Recorder: NewRecorder(),
		Buffer:   NewBuffer(filepath.Join(appDir, BufferFileName)),
	}, nil
}

// RecordEvent is the single write path called from PersistentPostRun.
// On Recorder error, the event is appended to the on-disk buffer for
// a later opportunistic drain. Returns any error from the recorder
// OR the buffer (whichever fails last).
//
// Phase 5 (REQ-10): the 5-field event is built inline; there is no
// Identity to read from. The EventID is fresh per call (ULID from
// crypto/rand-backed ulid.Make).
func (s *Service) RecordEvent(ctx context.Context, command string, exitStatus int) error {
	event := Event{
		Command:    command,
		ExitStatus: exitStatus,
		Timestamp:  NewTimestamp(),
		Version:    s.Version,
		EventID:    NewEventID(),
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	recType := "NoopRecorder"
	if _, ok := s.Recorder.(*NewRelicRecorder); ok {
		recType = "NewRelicRecorder"
	}
	eventJSON, _ := json.Marshal(event)
	pterm.Debug.Printfln("telemetry: recording event via %s: %s", recType, string(eventJSON))

	recErr := s.Recorder.Record(ctx, event)
	if recErr == nil {
		return nil
	}
	pterm.Debug.Printfln("telemetry: recorder failed (%v), buffering event", recErr)

	// Recorder failed: append to buffer for later drain.
	if bufErr := s.Buffer.Append(event); bufErr != nil {
		return fmt.Errorf("record failed (%v) and buffer write failed (%w)", recErr, bufErr)
	}
	return recErr
}

// DrainBuffer reads every buffered event and tries to send it via the
// Recorder. On success, the buffer is truncated. On any send failure,
// the drain stops and the unsent events are preserved.
func (s *Service) DrainBuffer(ctx context.Context) error {
	pterm.Debug.Printfln("telemetry: starting buffer drain from %s", s.Buffer.Path)
	err := s.Buffer.Drain(func(e Event) error {
		eventJSON, _ := json.Marshal(e)
		pterm.Debug.Printfln("telemetry: draining buffered event: %s", string(eventJSON))
		return s.Recorder.Record(ctx, e)
	})
	if err != nil {
		pterm.Debug.Printfln("telemetry: buffer drain error: %v", err)
	} else {
		pterm.Debug.Printfln("telemetry: buffer drain complete")
	}
	return err
}

// MaybeRunFirstRunPrompt shows the first-run prompt if and only if:
//  1. stdin is a TTY
//  2. <appDir>/telemetry-prompted does not exist
//
// On non-TTY (CI, pipes), the function returns silently WITHOUT
// writing the sentinel — the next TTY run will re-prompt (Pitfall
// P10). On answer, the sentinel is created AND the cfg is updated
// (via the onAnswer callback the cmd package provides).
func (s *Service) MaybeRunFirstRunPrompt(ctx context.Context, stdout io.Writer, stdin io.Reader, onAnswer func(yes bool) error) {
	sentinelPath := filepath.Join(s.AppDir, promptSentinelFile)
	if _, err := os.Stat(sentinelPath); err == nil {
		return // already answered
	}
	MaybeRunFirstRunPrompt(ctx, stdout, stdin, func(yes bool) error {
		if err := onAnswer(yes); err != nil {
			return err
		}
		// Persist the sentinel so the next run skips the prompt.
		if err := os.WriteFile(sentinelPath, []byte(yesStr(yes)), 0o644); err != nil {
			return fmt.Errorf("write prompt sentinel: %w", err)
		}
		return nil
	})
}

func yesStr(yes bool) string {
	if yes {
		return "yes"
	}
	return "no"
}

// commandNameAliases maps short alias names to their canonical form
// for the schema's `command` field. The aliases are the top-level
// shortcuts registered in root.go (add, delete, enable, disable,
// check-updates) plus a few common ones (on, install, rm) for
// forward compatibility. The function returns the canonical name;
// unknown names pass through unchanged.
var commandNameAliases = map[string]string{
	"on":      "enable",
	"off":     "disable",
	"install": "add",
	"rm":      "delete",
}

// NormalizeCommandName returns the canonical event-emit name for a
// cobra command. Unknown names pass through. Always returns a
// non-empty string.
func NormalizeCommandName(name string) string {
	if canonical, ok := commandNameAliases[name]; ok {
		return canonical
	}
	return name
}
