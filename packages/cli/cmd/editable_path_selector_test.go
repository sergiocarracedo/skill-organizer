package cmd

import "testing"

func TestEditablePathSelectorSelectedKeysWithoutCheckboxesIncludesAllItems(t *testing.T) {
	selector := newEditablePathSelector([]editablePathSelectorItem{
		{Key: "alpha", Label: "alpha", Parent: "alpha", AlwaysSelected: true},
		{Key: "beta", Label: "beta", Parent: "nested/beta", AlwaysSelected: true},
	}, []string{"", "nested"}, editablePathSelectorOptions{ShowCheckboxes: false, ShowToggleAll: false})

	got := selector.SelectedKeys()
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("SelectedKeys() = %#v, want [alpha beta]", got)
	}
}

func TestEditablePathSelectorLinesWithoutCheckboxesHideToggleAndMarkers(t *testing.T) {
	selector := newEditablePathSelector([]editablePathSelectorItem{{Key: "alpha", Label: "alpha", Parent: "alpha", AlwaysSelected: true}}, []string{""}, editablePathSelectorOptions{
		BasePathLabel:  "skills-organized/",
		ShowCheckboxes: false,
		ShowToggleAll:  false,
	})

	lines := selector.lines()
	if len(lines) != 2 {
		t.Fatalf("lines() len = %d, want 2", len(lines))
	}
	if got := stripANSI(lines[1]); got != "🡲 alpha -> skills-organized/alpha/" {
		t.Fatalf("stripANSI(lines()[1]) = %q", got)
	}
}
