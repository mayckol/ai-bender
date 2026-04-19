package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRoot_DefaultWhenNoConfig(t *testing.T) {
	repo := t.TempDir()
	got, err := ResolveRoot(repo)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	want := filepath.Join(repo, DefaultRootRelPath)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveRoot_ConfigOverride_Relative(t *testing.T) {
	repo := t.TempDir()
	cfgDir := filepath.Join(repo, ".bender")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "worktree:\n  root: custom/wts\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRoot(repo)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	want := filepath.Join(repo, "custom/wts")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveRoot_ConfigOverride_Absolute(t *testing.T) {
	repo := t.TempDir()
	cfgDir := filepath.Join(repo, ".bender")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	abs := "/tmp/bender-wt-test"
	body := "worktree:\n  root: " + abs + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRoot(repo)
	if err != nil {
		t.Fatalf("ResolveRoot: %v", err)
	}
	if got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
}

func TestValidatePlacement_RefusesUnderGit(t *testing.T) {
	repo := t.TempDir()
	cand := filepath.Join(repo, ".git", "worktrees", "x")
	err := ValidatePlacement(repo, cand)
	if !errors.Is(err, ErrPlacementRefused) {
		t.Fatalf("want ErrPlacementRefused, got %v", err)
	}
}

func TestValidatePlacement_RefusesMainTree(t *testing.T) {
	repo := t.TempDir()
	err := ValidatePlacement(repo, repo)
	if !errors.Is(err, ErrPlacementRefused) {
		t.Fatalf("want ErrPlacementRefused, got %v", err)
	}
}

func TestValidatePlacement_RefusesExistingPath(t *testing.T) {
	repo := t.TempDir()
	cand := filepath.Join(repo, ".bender", "worktrees", "abc")
	if err := os.MkdirAll(cand, 0o755); err != nil {
		t.Fatal(err)
	}
	err := ValidatePlacement(repo, cand)
	if !errors.Is(err, ErrPlacementRefused) {
		t.Fatalf("want ErrPlacementRefused, got %v", err)
	}
}

func TestValidatePlacement_AcceptsFreshPath(t *testing.T) {
	repo := t.TempDir()
	cand := filepath.Join(repo, ".bender", "worktrees", "brand-new")
	err := ValidatePlacement(repo, cand)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
