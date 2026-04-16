package constitution

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/discovery"
)

// TestWrite_NoOpOnUnchangedContent: re-running Write with the same discovery result
// MUST NOT create a new archive and MUST leave the existing file in place. This is the
// fix for v0.2.1 — v0.2.0 created useless identical archives on every re-run.
func TestWrite_NoOpOnUnchangedContent(t *testing.T) {
	root := t.TempDir()
	r := discovery.Result{
		Stack: discovery.StackInfo{Language: "Go", PackageManager: "go modules"},
	}

	// First write — no archive, creates current.
	if _, err := Write(root, r, time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("first write: %v", err)
	}
	archives, _ := os.ReadDir(filepath.Join(root, ".bender/artifacts/constitution"))
	if len(archives) != 0 {
		t.Fatalf("first write created %d archives; want 0", len(archives))
	}

	// Second write at a different time, same content — MUST be a no-op.
	if _, err := Write(root, r, time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("second write: %v", err)
	}
	archives, _ = os.ReadDir(filepath.Join(root, ".bender/artifacts/constitution"))
	if len(archives) != 0 {
		t.Fatalf("second write created %d archives on unchanged content; want 0", len(archives))
	}
}

// TestWrite_ArchivesOnRealChange: when the discovery result changes substantively,
// the prior constitution MUST be archived exactly once.
func TestWrite_ArchivesOnRealChange(t *testing.T) {
	root := t.TempDir()
	t0 := time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC)

	// First: Go project.
	if _, err := Write(root, discovery.Result{Stack: discovery.StackInfo{Language: "Go"}}, t0); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// Second: dependency added — real content change.
	r2 := discovery.Result{
		Stack:        discovery.StackInfo{Language: "Go"},
		Dependencies: []discovery.Dependency{{Name: "example.com/foo", Version: "v1.0.0"}},
	}
	if _, err := Write(root, r2, t1); err != nil {
		t.Fatalf("second write: %v", err)
	}
	archives, _ := os.ReadDir(filepath.Join(root, ".bender/artifacts/constitution"))
	if len(archives) != 1 {
		t.Fatalf("real change must produce exactly one archive; got %d", len(archives))
	}
}
