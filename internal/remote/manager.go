package remote

import (
	"fmt"
	"strings"
)

type Manager struct {
	providers []Provider
}

func NewManager(providers ...Provider) *Manager {
	return &Manager{providers: append([]Provider{}, providers...)}
}

func (m *Manager) Provider(id string) (Provider, error) {
	needle := strings.TrimSpace(id)
	for _, provider := range m.providers {
		if provider.ID() == needle {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("unknown provider %q", id)
}

func (m *Manager) Resolve(ref string) (Provider, []SkillSummary, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return nil, nil, fmt.Errorf("skill reference is required")
	}

	for _, provider := range m.providers {
		if !provider.Match(trimmed) {
			continue
		}

		skills, err := provider.Resolve(trimmed)
		if err != nil {
			return nil, nil, err
		}
		return provider, skills, nil
	}

	return nil, nil, fmt.Errorf("could not determine provider for %q", ref)
}
