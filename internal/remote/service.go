package remote

import (
	"fmt"
	"time"

	configpkg "github.com/sergiocarracedo/skill-organizer/cli/internal/config"
)

type Service struct {
	manager *Manager
	cache   *Cache
}

func NewService() (*Service, error) {
	registryPath, err := configpkg.RegistryPath()
	if err != nil {
		return nil, err
	}
	appConfig, err := configpkg.LoadAppConfigOrDefault(registryPath)
	if err != nil {
		return nil, err
	}
	cache, err := NewCache(appConfig.Updates.CacheTTLHours)
	if err != nil {
		return nil, err
	}

	return &Service{
		manager: NewManager(SkillsShProvider{}, GitHubProvider{}),
		cache:   cache,
	}, nil
}

func (s *Service) Manager() *Manager {
	return s.manager
}

func (s *Service) Resolve(ref string) (Provider, []SkillSummary, error) {
	return s.manager.Resolve(ref)
}

func (s *Service) Audit(provider Provider, skill SkillSummary) (AuditReport, error) {
	cacheKey := provider.ID() + ":" + skill.ID + ":audit"
	var cached AuditReport
	if hit, err := s.cache.Load("audits", cacheKey, &cached); err == nil && hit {
		return cached, nil
	}

	report, err := provider.FetchAudit(skill)
	if err != nil {
		return AuditReport{}, err
	}
	if err := s.cache.Save("audits", cacheKey, report); err != nil {
		return report, nil
	}
	return report, nil
}

func (s *Service) Update(provider Provider, current SkillSummary) (UpdateInfo, error) {
	cacheKey := provider.ID() + ":" + current.ID + ":update"
	var cached UpdateInfo
	if hit, err := s.cache.Load("updates", cacheKey, &cached); err == nil && hit {
		cached.Cached = true
		return cached, nil
	}

	available, hasUpdate, err := provider.CheckUpdate(current)
	if err != nil {
		return UpdateInfo{}, err
	}
	update := UpdateInfo{
		Current:   current,
		Available: available,
		HasUpdate: hasUpdate,
		CheckedAt: time.Now().UTC(),
	}
	if err := s.cache.Save("updates", cacheKey, update); err != nil {
		return update, nil
	}
	return update, nil
}

func (s *Service) FetchSkill(provider Provider, skill SkillSummary) (SkillBundle, error) {
	bundle, err := provider.FetchSkill(skill)
	if err != nil {
		return SkillBundle{}, fmt.Errorf("fetch skill %s: %w", skill.ID, err)
	}
	return bundle, nil
}
