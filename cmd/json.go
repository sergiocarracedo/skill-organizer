package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

type statusJSON struct {
	ConfigPath string           `json:"configPath"`
	Source     string           `json:"source"`
	Target     string           `json:"target"`
	Report     statuspkg.Report `json:"report"`
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
