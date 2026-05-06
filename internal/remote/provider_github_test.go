package remote

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubProviderResolveAndFetchSkill(t *testing.T) {
	zipBytes := buildTestZip(t, map[string]string{
		"repo-main/skills/demo/SKILL.md":       "---\nname: demo\ndescription: test\n---\n",
		"repo-main/skills/demo/helper.txt":     "helper",
		"repo-main/skills/ignored/README.md":   "ignored",
		"repo-main/skills/second/SKILL.md":     "---\nname: second\ndescription: second\n---\n",
	})

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/owner/repo/commits/main":
			_, _ = w.Write([]byte(`{"sha":"abcdef1234567890","commit":{"committer":{"date":"2026-05-06T20:00:00Z"}}}`))
		default:
			t.Fatalf("unexpected api path %s", r.URL.Path)
		}
	}))
	defer apiServer.Close()

	repoServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/owner/repo/archive/refs/heads/main.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipBytes)
		default:
			t.Fatalf("unexpected repo path %s", r.URL.Path)
		}
	}))
	defer repoServer.Close()

	originalAPI := githubAPIBaseURL
	originalRepo := githubRepositoryBaseURL
	githubAPIBaseURL = apiServer.URL
	githubRepositoryBaseURL = repoServer.URL
	defer func() {
		githubAPIBaseURL = originalAPI
		githubRepositoryBaseURL = originalRepo
	}()

	provider := GitHubProvider{}
	resolved, err := provider.Resolve(repoServer.URL + "/owner/repo")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("Resolve() len = %d, want 2", len(resolved))
	}

	bundle, err := provider.FetchSkill(resolved[0])
	if err != nil {
		t.Fatalf("FetchSkill() error = %v", err)
	}
	if bundle.Skill.Version != "abcdef1" {
		t.Fatalf("FetchSkill().Skill.Version = %q", bundle.Skill.Version)
	}
	if len(bundle.Files) == 0 {
		t.Fatalf("FetchSkill() returned no files")
	}
}

func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := zip.NewWriter(buffer)
	for path, contents := range files {
		file, err := writer.Create(path)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", path, err)
		}
		if _, err := file.Write([]byte(contents)); err != nil {
			t.Fatalf("Write(%q) error = %v", path, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buffer.Bytes()
}
