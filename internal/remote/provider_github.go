package remote

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

type GitHubProvider struct{}

var githubAPIBaseURL = "https://api.github.com"
var githubRepositoryBaseURL = "https://github.com"

type githubRepoInfo struct {
	DefaultBranch string `json:"default_branch"`
	PushedAt      string `json:"pushed_at"`
}

type githubCommitInfo struct {
	SHA    string `json:"sha"`
	Commit struct {
		Committer struct {
			Date string `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

func (p GitHubProvider) ID() string {
	return "repo"
}

func (p GitHubProvider) Match(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	return strings.HasPrefix(trimmed, githubRepositoryBaseURL+"/")
}

func (p GitHubProvider) Resolve(ref string) ([]SkillSummary, error) {
	owner, repo, err := parseGitHubRef(ref)
	if err != nil {
		return nil, err
	}

	branch, version, versionDate, err := githubRepoVersion(owner, repo)
	if err != nil {
		return nil, err
	}

	files, err := githubZipFiles(owner, repo, branch)
	if err != nil {
		return nil, err
	}

	summaries := make([]SkillSummary, 0)
	for skillPath := range detectSkillDirectories(files) {
		base := filepath.Base(skillPath)
			summaries = append(summaries, SkillSummary{
				Provider:      "repo",
				ID:            fmt.Sprintf("%s/%s:%s", owner, repo, filepath.ToSlash(skillPath)),
				Name:          base,
				Source:        fmt.Sprintf("%s/%s", owner, repo),
				SourceURL:     fmt.Sprintf("%s/%s/%s", githubRepositoryBaseURL, owner, repo),
				Version:       version,
				VersionDate:   versionDate,
				RepoSkillPath: filepath.ToSlash(skillPath),
		})
	}

	if len(summaries) == 0 {
		return nil, fmt.Errorf("no SKILL.md directories found in %s", ref)
	}

	return summaries, nil
}

func (p GitHubProvider) FetchSkill(skill SkillSummary) (SkillBundle, error) {
	owner, repo, err := parseGitHubSource(skill.Source)
	if err != nil {
		return SkillBundle{}, err
	}

	branch, version, versionDate, err := githubRepoVersion(owner, repo)
	if err != nil {
		return SkillBundle{}, err
	}

	allFiles, err := githubZipFiles(owner, repo, branch)
	if err != nil {
		return SkillBundle{}, err
	}

	prefix := strings.Trim(strings.TrimSpace(skill.RepoSkillPath), "/") + "/"
	bundleFiles := make([]File, 0)
	for _, file := range allFiles {
		if file.Path == strings.TrimSuffix(prefix, "/")+"/SKILL.md" || strings.HasPrefix(file.Path, prefix) {
			rel := strings.TrimPrefix(file.Path, prefix)
			if rel == "" {
				continue
			}
			bundleFiles = append(bundleFiles, File{Path: rel, Contents: file.Contents})
		}
	}

	updated := skill
	updated.Version = version
	updated.VersionDate = versionDate
	updated.ID = fmt.Sprintf("%s/%s:%s", owner, repo, filepath.ToSlash(skill.RepoSkillPath))

	return SkillBundle{Skill: updated, Files: bundleFiles}, nil
}

func (p GitHubProvider) FetchAudit(skill SkillSummary) (AuditReport, error) {
	return AuditReport{Skill: skill}, nil
}

func (p GitHubProvider) CheckUpdate(installed SkillSummary) (SkillSummary, bool, error) {
	resolved, err := p.Resolve(installed.SourceURL)
	if err != nil {
		return SkillSummary{}, false, err
	}

	for _, skill := range resolved {
		if skill.RepoSkillPath != installed.RepoSkillPath {
			continue
		}
		return skill, skill.Version != installed.Version, nil
	}

	return SkillSummary{}, false, fmt.Errorf("installed skill path %q no longer exists in %s", installed.RepoSkillPath, installed.SourceURL)
}

func parseGitHubRef(ref string) (string, string, error) {
	trimmed := strings.TrimSpace(ref)
	trimmed = strings.TrimPrefix(trimmed, githubRepositoryBaseURL+"/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository reference %q", ref)
	}
	return parts[0], parts[1], nil
}

func parseGitHubSource(source string) (string, string, error) {
	parts := strings.Split(strings.Trim(source, "/"), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid GitHub source %q", source)
	}
	return parts[0], parts[1], nil
}

func githubRepoVersion(owner string, repo string) (string, string, time.Time, error) {
	var repoInfo githubRepoInfo
	if err := getJSON(fmt.Sprintf("%s/repos/%s/%s", githubAPIBaseURL, owner, repo), &repoInfo); err != nil {
		return "", "", time.Time{}, err
	}

	var commit githubCommitInfo
	if err := getJSON(fmt.Sprintf("%s/repos/%s/%s/commits/%s", githubAPIBaseURL, owner, repo, repoInfo.DefaultBranch), &commit); err != nil {
		return "", "", time.Time{}, err
	}

	return repoInfo.DefaultBranch, shortVersion(commit.SHA), parseRemoteTime(commit.Commit.Committer.Date), nil
}

func githubZipFiles(owner string, repo string, branch string) ([]File, error) {
	url := fmt.Sprintf("%s/%s/%s/archive/refs/heads/%s.zip", githubRepositoryBaseURL, owner, repo, branch)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("download repository archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("download repository archive failed with %s: %s", resp.Status, string(body))
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read repository archive: %w", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open repository archive: %w", err)
	}

	files := make([]File, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		parts := strings.SplitN(file.Name, "/", 2)
		if len(parts) != 2 {
			continue
		}
		relPath := filepath.ToSlash(parts[1])

		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open archived file %s: %w", file.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read archived file %s: %w", file.Name, err)
		}

		files = append(files, File{Path: relPath, Contents: string(content)})
	}

	return files, nil
}

func detectSkillDirectories(files []File) map[string]struct{} {
	result := make(map[string]struct{})
	for _, file := range files {
		if filepath.Base(file.Path) != "SKILL.md" {
			continue
		}
		result[filepath.ToSlash(filepath.Dir(file.Path))] = struct{}{}
	}
	return result
}
