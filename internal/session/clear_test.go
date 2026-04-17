package session

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func populateSession(t *testing.T, root, id string) {
	t.Helper()
	dir := filepath.Join(SessionsRoot(root), id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"schema_version":1,"session_id":"`+id+`","command":"/ghu","started_at":"2026-04-17T00:00:00Z","completed_at":"2026-04-17T00:00:01Z","status":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// Scout cache directory for this session.
	cache := filepath.Join(CacheRoot(root), "scout", id)
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "index.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClear_RemovesSessionAndCache(t *testing.T) {
	root := t.TempDir()
	id := "2026-04-17T00-00-00-aaa"
	populateSession(t, root, id)

	if err := Clear(root, id); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(SessionsRoot(root), id)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("session dir still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(CacheRoot(root), "scout", id)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cache dir still exists: %v", err)
	}
}

func TestClear_NotFoundReturnsErrNoExist(t *testing.T) {
	root := t.TempDir()
	err := Clear(root, "does-not-exist")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("want fs.ErrNotExist, got %v", err)
	}
}

func TestClearAll_RemovesEverySessionAndCacheRoot(t *testing.T) {
	root := t.TempDir()
	populateSession(t, root, "2026-04-17T00-00-00-aaa")
	populateSession(t, root, "2026-04-17T00-00-01-bbb")
	populateSession(t, root, "2026-04-17T00-00-02-ccc")

	removed, err := ClearAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 3 {
		t.Fatalf("want 3 removed, got %d", removed)
	}
	if _, err := os.Stat(CacheRoot(root)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cache root still exists: %v", err)
	}
}

func TestClearAll_NoSessionsIsNoOp(t *testing.T) {
	root := t.TempDir()
	removed, err := ClearAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("want 0, got %d", removed)
	}
}

func TestClearAll_PreservesArtifacts(t *testing.T) {
	root := t.TempDir()
	populateSession(t, root, "2026-04-17T00-00-00-aaa")
	artDir := filepath.Join(root, ".bender", "artifacts", "ghu")
	if err := os.MkdirAll(artDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artFile := filepath.Join(artDir, "run-2026-04-17T00-00-00-report.md")
	if err := os.WriteFile(artFile, []byte("# report"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ClearAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(artFile); err != nil {
		t.Fatalf("artifact was deleted, expected preserved: %v", err)
	}
}
