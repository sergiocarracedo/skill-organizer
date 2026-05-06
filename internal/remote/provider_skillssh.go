package remote

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

var skillsShBaseURL = "https://skills.sh/api/v1"

type SkillsShProvider struct{}

type skillsShListResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Source     string `json:"source"`
		InstallURL string `json:"installUrl"`
		URL        string `json:"url"`
	} `json:"data"`
}

type skillsShDetailResponse struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Slug    string `json:"slug"`
	Hash    string `json:"hash"`
	Version string `json:"version"`
	Date    string `json:"date"`
	Files   []struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
	} `json:"files"`
}

type skillsShAuditResponse struct {
	Audits []struct {
		Provider   string   `json:"provider"`
		Status     string   `json:"status"`
		Summary    string   `json:"summary"`
		AuditedAt  string   `json:"auditedAt"`
		RiskLevel  string   `json:"riskLevel"`
		Categories []string `json:"categories"`
	} `json:"audits"`
}

func (p SkillsShProvider) ID() string {
	return "skills.sh"
}

func (p SkillsShProvider) Match(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if strings.HasPrefix(trimmed, "https://github.com/") {
		return false
	}
	return true
}

func (p SkillsShProvider) Resolve(ref string) ([]SkillSummary, error) {
	id := normalizeSkillsShID(ref)
	if id != "" && strings.Count(id, "/") >= 2 {
		var detail skillsShDetailResponse
		if err := getJSON(skillsShBaseURL+"/skills/"+id, &detail); err == nil {
			return []SkillSummary{skillsShSummaryFromDetail(detail)}, nil
		}
	}

	query := strings.TrimSpace(ref)
	if query == "" {
		return nil, fmt.Errorf("invalid skills.sh reference %q", ref)
	}

	var response skillsShListResponse
	searchURL := skillsShBaseURL + "/skills/search?q=" + url.QueryEscape(query) + "&limit=20"
	if err := getJSON(searchURL, &response); err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("no skills found for %q", ref)
	}

	result := make([]SkillSummary, 0, len(response.Data))
	for _, item := range response.Data {
		result = append(result, SkillSummary{
			Provider:  "skills.sh",
			ID:        item.ID,
			Name:      item.Name,
			Source:    item.Source,
			SourceURL: item.InstallURL,
			AuditURL:  item.URL,
		})
	}
	return result, nil
}

func (p SkillsShProvider) FetchSkill(skill SkillSummary) (SkillBundle, error) {
	var detail skillsShDetailResponse
	if err := getJSON(skillsShBaseURL+"/skills/"+skill.ID, &detail); err != nil {
		return SkillBundle{}, err
	}

	files := make([]File, 0, len(detail.Files))
	for _, file := range detail.Files {
		files = append(files, File{Path: file.Path, Contents: file.Contents})
	}

	return SkillBundle{Skill: skillsShSummaryFromDetail(detail), Files: files}, nil
}

func (p SkillsShProvider) FetchAudit(skill SkillSummary) (AuditReport, error) {
	var response skillsShAuditResponse
	if err := getJSON(skillsShBaseURL+"/skills/audit/"+skill.ID, &response); err != nil {
		return AuditReport{}, err
	}

	entries := make([]AuditEntry, 0, len(response.Audits))
	for _, audit := range response.Audits {
		entries = append(entries, AuditEntry{
			Provider:   audit.Provider,
			Status:     audit.Status,
			Summary:    audit.Summary,
			AuditedAt:  parseRemoteTime(audit.AuditedAt),
			RiskLevel:  audit.RiskLevel,
			Categories: append([]string{}, audit.Categories...),
		})
	}

	return AuditReport{Skill: skill, Entries: entries}, nil
}

func (p SkillsShProvider) CheckUpdate(installed SkillSummary) (SkillSummary, bool, error) {
	resolved, err := p.Resolve(installed.ID)
	if err != nil {
		return SkillSummary{}, false, err
	}
	if len(resolved) == 0 {
		return SkillSummary{}, false, fmt.Errorf("skill %q not found", installed.ID)
	}

	available := resolved[0]
	changed := available.Version != installed.Version || (available.Hash != "" && available.Hash != installed.Hash)
	return available, changed, nil
}

func normalizeSkillsShID(ref string) string {
	trimmed := strings.TrimSpace(ref)
	trimmed = strings.TrimPrefix(trimmed, "https://skills.sh/")
	trimmed = strings.TrimPrefix(trimmed, "http://skills.sh/")
	trimmed = strings.TrimPrefix(trimmed, "skills.sh/")
	trimmed = strings.Trim(trimmed, "/")
	return trimmed
}

func skillsShSummaryFromDetail(detail skillsShDetailResponse) SkillSummary {
	sourceURL := "https://github.com/" + detail.Source
	if strings.Contains(detail.Source, ".") {
		sourceURL = (&url.URL{Scheme: "https", Host: detail.Source}).String()
	}

	version := strings.TrimSpace(detail.Version)
	if version == "" {
		version = shortVersion(detail.Hash)
	}

	return SkillSummary{
		Provider:    "skills.sh",
		ID:          detail.ID,
		Name:        detail.Slug,
		Source:      detail.Source,
		SourceURL:   sourceURL,
		Version:     version,
		VersionDate: parseRemoteTime(detail.Date),
		Hash:        detail.Hash,
		AuditURL:    "https://skills.sh/" + detail.ID,
	}
}

func parseRemoteTime(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func shortVersion(hash string) string {
	trimmed := strings.TrimSpace(hash)
	if len(trimmed) <= 7 {
		return trimmed
	}
	return trimmed[:7]
}
