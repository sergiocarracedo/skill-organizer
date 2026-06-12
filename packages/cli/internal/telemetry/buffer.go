package telemetry

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// BufferFileName is the on-disk JSONL file inside the app dir.
const BufferFileName = "telemetry-buffer.jsonl"

// BufferMaxBytes is the post-condition cap (per CONTEXT). Exceeding
// it triggers a FIFO eviction pass that drops the oldest events.
const BufferMaxBytes = 1 << 20 // 1 MB

// Buffer is the JSONL spool for offline-retry. The Path is
// <appDir>/telemetry-buffer.jsonl (the cmd package passes it in
// from configpkg.AppDir()). Concurrent CLI processes are safe: writes
// use O_APPEND (POSIX-atomic up to PIPE_BUF = 4096, a single event
// is ~200 bytes). Drains read the whole file, call the callback per
// event, and on full success truncate; the rare mid-truncate crash
// loses at most one event, accepted per CONTEXT ("opportunistic drain").
type Buffer struct {
	Path string

	mu sync.Mutex // serializes Append + Drain within one process
}

// NewBuffer returns a Buffer pointing at the given path. The file is
// not opened or created until the first Append or Drain call.
func NewBuffer(path string) *Buffer {
	return &Buffer{Path: path}
}

// Append marshals event to JSON, writes a single line to the buffer
// file (O_APPEND, atomic up to PIPE_BUF), and then checks the file
// size. If the size exceeds BufferMaxBytes, evict() drops the oldest
// events until the file is under the cap.
func (b *Buffer) Append(event Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(b.Path), 0o755); err != nil {
		return fmt.Errorf("create buffer dir: %w", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	f, err := os.OpenFile(b.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open buffer: %w", err)
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write buffer: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		_ = f.Close()
		return fmt.Errorf("write buffer newline: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close buffer: %w", err)
	}

	info, err := os.Stat(b.Path)
	if err != nil {
		return fmt.Errorf("stat buffer: %w", err)
	}
	if info.Size() > BufferMaxBytes {
		if err := b.evictLocked(); err != nil {
			return fmt.Errorf("evict: %w", err)
		}
	}
	return nil
}

// evictLocked drops the oldest events until the file is at or below
// BufferMaxBytes. Must be called with b.mu held.
func (b *Buffer) evictLocked() error {
	content, err := os.ReadFile(b.Path)
	if err != nil {
		return fmt.Errorf("read buffer: %w", err)
	}

	// Walk the lines from the front (oldest) and keep dropping them
	// until the remaining bytes are at or below the cap. Each line
	// ends with '\n'; we keep the trailing newline on the kept events.
	lines := splitLinesKeepNewline(content)
	for len(lines) > 0 {
		remaining := joinBytes(lines)
		if int64(len(remaining)) <= BufferMaxBytes {
			break
		}
		lines = lines[1:]
	}

	if err := os.WriteFile(b.Path, joinBytes(lines), 0o644); err != nil {
		return fmt.Errorf("rewrite buffer: %w", err)
	}
	return nil
}

// Drain reads the buffer file line by line, calls send for each
// successfully-parsed event, and on full success truncates the file.
// If send returns a non-nil error, the scan stops and the error is
// returned WITHOUT truncating — the un-sent events are preserved for
// the next run. If the file does not exist, Drain returns nil (an
// empty buffer is the steady state).
func (b *Buffer) Drain(send func(Event) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	f, err := os.Open(b.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open buffer: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Allow lines up to 1 MB so a single malformed large line is
	// rejected by json.Unmarshal rather than silently truncated.
	scanner.Buffer(make([]byte, 0, 64*1024), BufferMaxBytes)

	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip empty lines (e.g. trailing newline at EOF).
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("unmarshal buffered event: %w", err)
		}
		if err := send(event); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan buffer: %w", err)
	}

	if err := os.WriteFile(b.Path, nil, 0o644); err != nil {
		return fmt.Errorf("truncate buffer: %w", err)
	}
	return nil
}

// splitLinesKeepNewline splits content on '\n' keeping a trailing
// '\n' on each non-empty piece (so the rewrite preserves the JSONL
// shape). An empty input returns an empty slice.
func splitLinesKeepNewline(content []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range content {
		if b != '\n' {
			continue
		}
		// Include the newline in the segment.
		lines = append(lines, content[start:i+1])
		start = i + 1
	}
	if start < len(content) {
		// Trailing fragment without a newline — still keep it as a
		// line (preserves content on the rare partial-write case).
		lines = append(lines, content[start:])
	}
	return lines
}

// joinBytes concatenates the byte slices.
func joinBytes(parts [][]byte) []byte {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	out := make([]byte, 0, total)
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
