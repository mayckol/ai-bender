package constitution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/discovery"
)

func TestRender_PendingForEmptyResult(t *testing.T) {
	out, err := Render(discovery.Result{Pending: []string{"purpose: pending"}}, time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)
	for _, want := range []string{
		"# Project Constitution",
		"## Purpose",
		"## Stack",
		"## Structure",
		"## Tests",
		"## Lint",
		"## Build / CI",
		"## Conventions",
		"## Glossary",
		"## Dependencies",
		"_pending: true",
		"created_at: 2026-04-16T14:03:22Z",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("constitution missing %q\n--- body ---\n%s", want, body)
		}
	}
}

func TestRender_PopulatesDetectedSections(t *testing.T) {
	r := discovery.Result{
		Stack:     discovery.StackInfo{Language: "Go", PackageManager: "go modules"},
		Structure: discovery.StructureInfo{Folders: []string{"cmd", "internal"}, EntryPoints: []string{"main.go"}},
		Tests:     discovery.TestsInfo{Framework: "go test (stdlib)"},
		Build:     discovery.BuildInfo{Tool: "go build", HasMakefile: true},
	}
	out, err := Render(r, time.Now())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	body := string(out)
	for _, want := range []string{
		"Language: Go",
		"Package manager: go modules",
		"Folders: cmd, internal",
		"Entry points: main.go",
		"Framework: go test (stdlib)",
		"Build tool: go build",
		"Makefile: true",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("constitution missing %q", want)
		}
	}
}

func TestWrite_ArchivesPriorRevision(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC)

	// First write: no prior, archive should remain empty.
	if _, err := Write(root, discovery.Result{}, now); err != nil {
		t.Fatalf("first write: %v", err)
	}
	revs, _ := os.ReadDir(filepath.Join(root, ".bender/artifacts", "constitution"))
	if len(revs) != 0 {
		t.Fatalf("expected no revisions after first write, got %d", len(revs))
	}

	// Second write with DIFFERENT content: prior is moved into .bender/artifacts/constitution/<ts>.md.
	// (v0.2.1 no-ops identical rewrites; we need a real content change to trigger archiving.)
	now2 := now.Add(time.Hour)
	changed := discovery.Result{
		Stack: discovery.StackInfo{Language: "Go"},
	}
	if _, err := Write(root, changed, now2); err != nil {
		t.Fatalf("second write: %v", err)
	}
	revs, _ = os.ReadDir(filepath.Join(root, ".bender/artifacts", "constitution"))
	if len(revs) != 1 {
		t.Fatalf("expected one revision after second write, got %d", len(revs))
	}
	// Revision filename uses the archive-time timestamp (now2), not the prior file's creation time.
	if !strings.HasPrefix(revs[0].Name(), "2026-04-16T15-03-22") {
		t.Fatalf("unexpected revision filename %q", revs[0].Name())
	}
}

func TestWrite_CollisionSuffix(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 4, 16, 14, 3, 22, 0, time.UTC)
	// Manually plant an existing revision so the next archive must use a collision suffix.
	if err := os.MkdirAll(filepath.Join(root, ".bender/artifacts", "constitution"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".bender/artifacts", "constitution", "2026-04-16T14-03-22.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	// First write installs current (with the Go stack).
	if _, err := Write(root, discovery.Result{Stack: discovery.StackInfo{Language: "Go"}}, now); err != nil {
		t.Fatal(err)
	}
	// Second write at the same instant with DIFFERENT content — forces an archive.
	// (Writing the same content twice is a no-op after v0.2.1; we need a real content change
	// to trigger the collision path.)
	changed := discovery.Result{
		Stack:        discovery.StackInfo{Language: "Go"},
		Dependencies: []discovery.Dependency{{Name: "example.com/foo", Version: "v1.0.0"}},
	}
	if _, err := Write(root, changed, now); err != nil {
		t.Fatal(err)
	}
	revs, _ := os.ReadDir(filepath.Join(root, ".bender/artifacts", "constitution"))
	if len(revs) != 2 {
		t.Fatalf("expected two revisions, got %d", len(revs))
	}
	hasSuffix := false
	for _, r := range revs {
		if strings.Contains(r.Name(), "-1.md") {
			hasSuffix = true
		}
	}
	if !hasSuffix {
		var names []string
		for _, r := range revs {
			names = append(names, r.Name())
		}
		t.Fatalf("expected a -1 suffixed file, got %v", names)
	}
}
