package mover

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
	statuspkg "github.com/sergiocarracedo/skill-organizer/cli/internal/status"
)

type Move struct {
	Name   string
	Source string
	Target string
}

func Plan(location configpkg.Location) ([]Move, error) {
	report, err := statuspkg.Build(location)
	if err != nil {
		return nil, err
	}

	planned := make([]Move, 0, len(report.Unmanaged))
	for _, name := range report.Unmanaged {
		planned = append(planned, Move{
			Name:   name,
			Source: filepath.Join(location.Target, name),
			Target: filepath.Join(location.Source, name),
		})
	}

	sort.Slice(planned, func(i, j int) bool {
		return planned[i].Name < planned[j].Name
	})

	return planned, nil
}

func Apply(moves []Move) error {
	for _, move := range moves {
		if err := os.MkdirAll(filepath.Dir(move.Target), 0o755); err != nil {
			return fmt.Errorf("create move destination parent for %q: %w", move.Name, err)
		}

		if _, err := os.Stat(move.Target); err == nil {
			return fmt.Errorf("move target already exists: %s", move.Target)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat move target %q: %w", move.Target, err)
		}

		if err := os.Rename(move.Source, move.Target); err != nil {
			return fmt.Errorf("move unmanaged target %q: %w", move.Name, err)
		}
	}

	return nil
}
