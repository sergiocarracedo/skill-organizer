package telemetry

import (
	"context"
	"testing"
)

// fakeRecorder is a test double for the Recorder interface used by
// the factory-swap test. It captures every recorded event in the
// order they were received so tests can assert on the captured slice.
type fakeRecorder struct {
	events []Event
}

// Record appends e to the captured events and returns nil.
func (f *fakeRecorder) Record(_ context.Context, e Event) error {
	f.events = append(f.events, e)
	return nil
}

func TestNoopRecorderDropsEvents(t *testing.T) {
	rec := NoopRecorder{}
	event := Event{
		Command:    "x",
		InstallID:  "0123456789abcdef0123456789abcdef",
		HostID:     "0123456789abcdef0123456789abcdef",
		Timestamp:  "2026-06-11T00:00:00Z",
		Version:    "1.0.0",
		EventID:    "01HXYZABCDEFGHJKMNPQRSTVWX",
		ExitStatus: 0,
	}
	for i := 0; i < 1000; i++ {
		if err := rec.Record(context.Background(), event); err != nil {
			t.Fatalf("NoopRecorder.Record() iter %d = %v, want nil", i, err)
		}
	}
}

func TestRecorderFactoryReturnsNoopOnEmptyConfig(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() {
		RecorderFactoryFunc = original
	})

	// Force the production default: the literal that the package
	// itself uses. This makes the test resilient to a future swap
	// in package init code.
	RecorderFactoryFunc = func() Recorder { return NoopRecorder{} }

	rec := NewRecorder()
	if _, ok := rec.(NoopRecorder); !ok {
		t.Fatalf("NewRecorder() returned %T, want NoopRecorder", rec)
	}
}

func TestRecorderFactorySwapRoundtrip(t *testing.T) {
	original := RecorderFactoryFunc
	t.Cleanup(func() {
		RecorderFactoryFunc = original
	})

	RecorderFactoryFunc = func() Recorder { return &fakeRecorder{} }

	rec := NewRecorder()
	fr, ok := rec.(*fakeRecorder)
	if !ok {
		t.Fatalf("NewRecorder() returned %T, want *fakeRecorder", rec)
	}

	ev := validEvent()
	if err := fr.Record(context.Background(), ev); err != nil {
		t.Fatalf("fake.Record() = %v, want nil", err)
	}
	if len(fr.events) != 1 {
		t.Fatalf("fake captured %d events, want 1", len(fr.events))
	}
	if fr.events[0].Command != ev.Command {
		t.Fatalf("fake captured Command = %q, want %q", fr.events[0].Command, ev.Command)
	}
}
