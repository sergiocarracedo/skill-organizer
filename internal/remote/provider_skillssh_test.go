package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSkillsShProviderResolveSearchAndAudit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/skills/search":
			_, _ = w.Write([]byte(`{"data":[{"id":"owner/repo/demo-skill","name":"Demo Skill","source":"owner/repo","installUrl":"https://github.com/owner/repo","url":"https://skills.sh/owner/repo/demo-skill"}]}`))
		case "/api/v1/skills/owner/repo/demo-skill":
			_, _ = w.Write([]byte(`{"id":"owner/repo/demo-skill","source":"owner/repo","slug":"demo-skill","hash":"abcdef123456","date":"2026-05-06T20:00:00Z","files":[{"path":"SKILL.md","contents":"---\nname: demo-skill\ndescription: test\n---\n"}]}`))
		case "/api/v1/skills/audit/owner/repo/demo-skill":
			_, _ = w.Write([]byte(`{"audits":[{"provider":"Socket","status":"pass","summary":"No alerts","auditedAt":"2026-05-06T20:10:00Z","riskLevel":"LOW"}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	original := skillsShBaseURL
	skillsShBaseURL = server.URL + "/api/v1"
	defer func() { skillsShBaseURL = original }()

	provider := SkillsShProvider{}
	resolved, err := provider.Resolve("demo skill")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID != "owner/repo/demo-skill" {
		t.Fatalf("Resolve() = %#v", resolved)
	}

	bundle, err := provider.FetchSkill(resolved[0])
	if err != nil {
		t.Fatalf("FetchSkill() error = %v", err)
	}
	if len(bundle.Files) != 1 || bundle.Skill.Version != "abcdef1" {
		t.Fatalf("FetchSkill() = %#v", bundle)
	}

	audit, err := provider.FetchAudit(resolved[0])
	if err != nil {
		t.Fatalf("FetchAudit() error = %v", err)
	}
	if len(audit.Entries) != 1 || audit.Entries[0].Provider != "Socket" {
		t.Fatalf("FetchAudit() = %#v", audit)
	}
}
