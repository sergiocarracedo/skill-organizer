package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// IsStdInTTYFunc is a swappable function variable for IsStdInTTY.
// Production calls IsStdInTTY(os.Stdin) via the implementation; tests
// reassign to control the TTY behaviour.
var IsStdInTTYFunc = func() bool { return isStdInTTY(os.Stdin) }

// IsStdInTTY returns true when the current process's stdin is
// attached to a terminal. Piped input, redirected files, and CI
// environments return false. The check delegates to
// golang.org/x/term.IsTerminal, which on Windows uses console mode
// APIs and on POSIX uses isatty(2).
func IsStdInTTY() bool {
	return IsStdInTTYFunc()
}

func isStdInTTY(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// ConfirmFunc is a swappable function variable for the yes/no prompt.
// Plan 02 wires it in the cmd package to point at cmd.confirm
// (pterm.DefaultInteractiveConfirm). The reason this is a func var
// and not a direct call is that cmd.confirm lives in the cmd package
// and would create a circular import; the func var breaks the cycle.
var ConfirmFunc = defaultConfirm

// defaultConfirm is a safe no-op that respects the default. The cmd
// package's init() (added in task 03-02-07) overrides ConfirmFunc to
// point at cmd.confirm.
func defaultConfirm(prompt string, defaultValue bool) (bool, error) {
	fmt.Fprintf(io.Discard, "telemetry: confirm not wired (prompt=%q default=%v)\n", prompt, defaultValue)
	return defaultValue, nil
}

// FirstRunPrompt asks the user whether to enable anonymous telemetry.
// Returns (true, nil) on yes, (false, nil) on no, and (false, err)
// on error (e.g. ctrl-c). The default answer is `false` (CONTEXT:
// "Default = off"). The caller (MaybeRunFirstRunPrompt) is responsible
// for persisting the answer.
func FirstRunPrompt(stdout io.Writer, stdin io.Reader) (bool, error) {
	return ConfirmFunc("Enable anonymous telemetry? (only command names, no args/paths/PII; use `telemetry disable` to turn off at any time)", false)
}

// MaybeRunFirstRunPrompt is the fire-and-forget wrapper that mirrors
// the maintenance.MaybeNotify* signature. It checks the TTY via
// IsStdInTTYFunc, calls FirstRunPrompt if stdin is a TTY, and on
// answer invokes onAnswer (which persists the choice).
//
// On non-TTY (CI, pipes), the function returns silently without
// writing the answer to YAML: the next TTY run will re-prompt.
// (Pitfall P10.) Errors are intentionally swallowed to match the
// MaybeNotify* precedent; the first-run prompt's failure mode is
// "user pressed Ctrl-C" or "stdin closed", both user-initiated, no
// error message needed.
func MaybeRunFirstRunPrompt(ctx context.Context, stdout io.Writer, stdin io.Reader, onAnswer func(yes bool) error) {
	if !IsStdInTTYFunc() {
		return
	}
	answer, err := FirstRunPrompt(stdout, stdin)
	if err != nil {
		return
	}
	_ = onAnswer(answer)
}

// drainReader placeholder removed; bufio is no longer used here.
// (The drain-buffer utility lives in buffer.go; the prompt is
// purely the user-facing yes/no via ConfirmFunc.)
