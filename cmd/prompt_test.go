package cmd

import "testing"

func TestEditableInputStateInsertDeleteAndCursorMovement(t *testing.T) {
	state := editableInputState{}
	state.insertRunes([]rune("abc"))
	if got := state.String(); got != "abc" {
		t.Fatalf("state.String() = %q, want %q", got, "abc")
	}

	state.moveLeft()
	state.insertRunes([]rune("X"))
	if got := state.String(); got != "abXc" {
		t.Fatalf("state.String() = %q, want %q", got, "abXc")
	}
	if state.cursor != 3 {
		t.Fatalf("state.cursor = %d, want %d", state.cursor, 3)
	}

	state.deleteBeforeCursor()
	if got := state.String(); got != "abc" {
		t.Fatalf("state.String() = %q, want %q", got, "abc")
	}

	state.moveHome()
	state.deleteAtCursor()
	if got := state.String(); got != "bc" {
		t.Fatalf("state.String() = %q, want %q", got, "bc")
	}

	state.moveEnd()
	if state.cursor != 2 {
		t.Fatalf("state.cursor = %d, want %d", state.cursor, 2)
	}
}

func TestEditableInputStateAutocomplete(t *testing.T) {
	state := editableInputState{}
	state.setValue("thi")
	state.applyAutocomplete([]string{"personal", "thirdparty/asciinema", "thirdparty/go"})

	if got := state.String(); got != "thirdparty/asciinema" {
		t.Fatalf("state.String() = %q, want %q", got, "thirdparty/asciinema")
	}
	if state.cursor != len([]rune("thirdparty/asciinema")) {
		t.Fatalf("state.cursor = %d", state.cursor)
	}
}

func TestAutocompleteSuggestionAtCursorUsesTokenBeforeCursor(t *testing.T) {
	state := editableInputState{}
	state.setValue("thirdparty/asc notes")
	state.cursor = len([]rune("thirdparty/asc"))

	updated := autocompleteSuggestionAtCursor(state, []string{"personal", "thirdparty/asciinema", "thirdparty/go"})

	if got := updated.String(); got != "thirdparty/asciinema notes" {
		t.Fatalf("updated.String() = %q, want %q", got, "thirdparty/asciinema notes")
	}
	if updated.cursor != len([]rune("thirdparty/asciinema")) {
		t.Fatalf("updated.cursor = %d", updated.cursor)
	}
}

func TestPathTokenBounds(t *testing.T) {
	start, end := pathTokenBounds("foo bar baz", len([]rune("foo bar")))
	if start != 4 || end != 7 {
		t.Fatalf("pathTokenBounds() = (%d, %d), want (4, 7)", start, end)
	}
}
