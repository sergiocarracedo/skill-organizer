package cmd

import (
	"strings"
	"testing"

	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

func TestFormatSkillLabelShowsInstalledAndAvailableVersions(t *testing.T) {
	label := formatSkillLabel(statuspkg.SkillStatus{
		State: statuspkg.StateSynced,
		InstalledVersion: "006f8413941b59eff54a7ce64851b8a2fb79e7a3a5f1a895e97a48f01482553d",
		InstalledDate: "2026-05-01T12:00:00Z",
		AvailableVersion: "0.22.5",
		AvailableCheckedDate: "2026-05-10T13:49:48Z",
	}, "asciinema-recorder")

	for _, want := range []string{"[synced]", "installed 006f841 2026-05-01", "update 0.22.5 2026-05-10"} {
		if !strings.Contains(stripANSI(label), want) {
			t.Fatalf("formatSkillLabel() missing %q in %q", want, stripANSI(label))
		}
	}
}
