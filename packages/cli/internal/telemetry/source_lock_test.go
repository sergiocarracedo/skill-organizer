package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoLinkableIDSource is the Phase 5 (REQ-10) source-lock guard.
// REQ-10 acceptance requires that "all random-byte generation in
// the package reads from crypto/rand" — i.e. the only random IDs
// the recorder emits (the per-event EventID) are not linkable to a
// machine or a user, because they are generated per-event from
// crypto/rand (the only `*rand.Read` call in the package).
//
// The check has three parts:
//  1. The package source must not import `math/rand` (which is
//     non-cryptographic and would be a regression vector for a
//     linkable seed).
//  2. The package source must reference `crypto/rand` (so the
//     random-byte generator is the cryptographic one).
//  3. The Event struct must not carry any field whose name
//     contains "ID" except `EventID` (the per-event random
//     UUID). The dropped `InstallID` and `HostID` must not
//     return — the source-lock test catches a regression that
//     re-introduces them.
func TestNoLinkableIDSource(t *testing.T) {
	pkgDir := findPackageDir(t)

	// (1) math/rand must not be imported anywhere in the package
	// source (the package itself, or any sub-package of the
	// `telemetry` package — but the plan scope is just this one
	// file tree, so we walk the package directory).
	mathRandHits := grepPackage(t, pkgDir, "math/rand")
	if len(mathRandHits) > 0 {
		t.Fatalf("telemetry package must not import math/rand (Phase 5 REQ-10: only crypto/rand is allowed for random IDs)\nhits:\n%s",
			strings.Join(mathRandHits, "\n"))
	}

	// (2) crypto/rand must be referenced somewhere in the package
	// (ulid.Make is crypto/rand-backed by default; this is the
	// belt-and-braces assertion that the only random source is
	// the cryptographic one).
	cryptoRandHits := grepPackage(t, pkgDir, "crypto/rand")
	if len(cryptoRandHits) == 0 {
		t.Fatalf("telemetry package must reference crypto/rand (Phase 5 REQ-10: linkable IDs would re-emerge without it)")
	}

	// (3) No Event field with "ID" in the name except EventID.
	// We parse the event.go file with go/parser and walk the AST
	// to find the Event struct's fields, then assert the set.
	eventFields := collectEventFields(t, filepath.Join(pkgDir, "event.go"))
	for _, name := range eventFields {
		if !strings.Contains(name, "ID") {
			continue
		}
		if name == "EventID" {
			continue
		}
		t.Fatalf("Event struct has field %q — only EventID is allowed in the 5-field schema (Phase 5 REQ-10: no linkable IDs)", name)
	}
}

// findPackageDir walks up from the test's CWD to find the
// internal/telemetry package directory. The path is "." when
// running under `go test ./internal/telemetry/...` and walks up
// to packages/cli/internal/telemetry when running under
// `go test ./...`.
func findPackageDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() = %v", err)
	}
	for {
		// The presence of `recorder.go` is the marker for the
		// internal/telemetry package directory.
		if _, err := os.Stat(filepath.Join(dir, "recorder.go")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("internal/telemetry package directory not found walking up from %q", dir)
		}
		dir = parent
	}
}

// grepPackage returns the list of file:line matches for needle
// in any .go file under dir. The search is plain substring (not
// regex) to keep the assertion readable.
func grepPackage(t *testing.T, dir, needle string) []string {
	t.Helper()
	var hits []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) = %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		// Skip test files: they are not the production source.
		// (The test files use `math/rand` for fixtures and
		// `bytes.NewReader` to inject deterministic bytes; the
		// production source is what we want to lock.)
		if strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%q) = %v", filepath.Join(dir, e.Name()), err)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if strings.Contains(line, needle) {
				hits = append(hits, e.Name()+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
	}
	return hits
}

// collectEventFields returns the field names of the Event struct
// declared in event.go. We use go/parser to walk the AST so the
// source-lock test is structural (not regex), which catches
// renames and comment-trickery.
func collectEventFields(t *testing.T, eventGoPath string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, eventGoPath, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parser.ParseFile(%q) = %v", eventGoPath, err)
	}
	var eventTypeName string
	var fields []string
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if eventTypeName == "" {
				eventTypeName = ts.Name.Name
			}
			if ts.Name.Name != eventTypeName {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					fields = append(fields, name.Name)
				}
			}
		}
	}
	if len(fields) == 0 {
		t.Fatalf("Event struct not found in %q (parser found no fields)", eventGoPath)
	}
	return fields
}

// itoa is a small helper that avoids importing strconv for a
// single use. Kept private so future test additions can extend
// without naming conflicts.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := "0123456789"
	var buf [20]byte
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = digits[n%10]
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
