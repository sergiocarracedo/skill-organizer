package remote

import "time"

type SkillRef struct {
	Provider string
	Ref      string
}

type SkillSummary struct {
	Provider      string
	ID            string
	Name          string
	Source        string
	SourceURL     string
	Version       string
	VersionDate   time.Time
	Hash          string
	AuditURL      string
	RepoSkillPath string
}

type File struct {
	Path     string
	Contents string
}

type SkillBundle struct {
	Skill SkillSummary
	Files []File
}

type AuditEntry struct {
	Provider   string
	Status     string
	Summary    string
	AuditedAt  time.Time
	RiskLevel  string
	Categories []string
}

type AuditReport struct {
	Skill   SkillSummary
	Entries []AuditEntry
}

type UpdateInfo struct {
	Current   SkillSummary
	Available SkillSummary
	HasUpdate bool
	CheckedAt time.Time
	Cached    bool
}

type Provider interface {
	ID() string
	Match(ref string) bool
	Resolve(ref string) ([]SkillSummary, error)
	FetchSkill(skill SkillSummary) (SkillBundle, error)
	FetchAudit(skill SkillSummary) (AuditReport, error)
	CheckUpdate(installed SkillSummary) (SkillSummary, bool, error)
}
