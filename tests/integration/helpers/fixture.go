// Package helpers provides test fixtures for feature 004-worktree-flow
// integration tests. Instead of shipping a binary tarball, each fixture is
// built on demand with `git init` + seeded commits so the test surface is
// transparent and reviewable.
package helpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NewSeededRepo creates a temporary git repository with one commit on `main`,
// a `.bender/.gitignore` entry, and an initial README. Returns the absolute
// path to the repo root. The t.TempDir() cleanup removes it automatically.
//
// Skips the test if the git binary is unavailable.
func NewSeededRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	dir := t.TempDir()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bender-test",
			"GIT_AUTHOR_EMAIL=bender@test.local",
			"GIT_COMMITTER_NAME=bender-test",
			"GIT_COMMITTER_EMAIL=bender@test.local",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run("git", "init", "-q", "-b", "main")
	run("git", "config", "user.email", "bender@test.local")
	run("git", "config", "user.name", "bender-test")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".bender"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".bender", ".gitignore"),
		[]byte("sessions/\ncache/\nworktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"),
		[]byte(".bender/sessions/\n.bender/cache/\n.bender/worktrees/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "seed")
	return dir
}

// DirtyRepo returns a seeded repo plus an uncommitted-change setup:
//   - a tracked file with a local modification,
//   - an untracked file in the root.
//
// Used by tests that verify the main working tree is untouched across a
// pipeline session.
func DirtyRepo(t *testing.T) string {
	t.Helper()
	repo := NewSeededRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# fixture\nLOCAL EDIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "untracked.txt"), []byte("not staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}
