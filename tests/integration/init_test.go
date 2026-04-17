// Package integration_test holds end-to-end tests that drive the bender CLI as a subprocess against
// fixture projects under tests/integration/fixtures/. They validate behaviour as a user would observe it.
package integration_test

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBenderOnce builds bin/bender once per test binary invocation. Subsequent tests reuse it.
func buildBenderOnce(tb testing.TB) string {
	tb.Helper()
	repoRoot := repoRootDir(tb)
	binPath := filepath.Join(repoRoot, "bin", "bender")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/bender")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		tb.Fatalf("build bender: %v\n%s", err, out)
	}
	return binPath
}

func repoRootDir(tb testing.TB) string {
	tb.Helper()
	wd, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	// Test runs from tests/integration; repo root is two parents up.
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func mkProject(tb testing.TB) string {
	tb.Helper()
	root := tb.TempDir()
	return root
}

func copyFixtureInto(tb testing.TB, fixtureName, dest string) {
	tb.Helper()
	repoRoot := repoRootDir(tb)
	src := filepath.Join(repoRoot, "tests", "integration", "fixtures", fixtureName)
	if err := copyDir(src, dest); err != nil {
		tb.Fatalf("copy fixture %s: %v", fixtureName, err)
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func runBender(tb testing.TB, bin, dir string, args ...string) (string, error) {
	tb.Helper()
	// Isolate the workspace registry so a `default_project` in the developer's
	// real `~/.bender/workspace.yaml` can't leak into cwd-scoped tests.
	return runBenderEnv(tb, bin, dir, []string{"XDG_CONFIG_HOME=" + tb.TempDir()}, args...)
}

func runBenderEnv(tb testing.TB, bin, dir string, extraEnv []string, args ...string) (string, error) {
	tb.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func mkNamedProject(tb testing.TB, name string) string {
	tb.Helper()
	parent := tb.TempDir()
	root := filepath.Join(parent, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		tb.Fatal(err)
	}
	return root
}

// TestInit_OnEmptyProject: T033.
func TestInit_OnEmptyProject(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	out, err := runBender(t, bin, root, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	mustExist(t, root, ".bender/groups.yaml")
	mustExist(t, root, ".bender/pipeline.yaml")
	mustExist(t, root, ".bender/artifacts/constitution.md")
	mustNotExist(t, root, ".claude/groups.yaml")

	body := mustRead(t, filepath.Join(root, ".bender/artifacts", "constitution.md"))
	if !strings.Contains(body, "_pending: true") {
		t.Fatalf("expected pending sections on empty project; body:\n%s", body)
	}
	if !strings.Contains(out, "next: open this project in Claude Code") {
		t.Fatalf("expected next-step hint in stdout; got:\n%s", out)
	}
}

// TestInit_OnGoProject: T034 (≥90% of detectable items populated).
func TestInit_OnGoProject(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	copyFixtureInto(t, "go-project", root)
	out, err := runBender(t, bin, root, "init")
	if err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	body := mustRead(t, filepath.Join(root, ".bender/artifacts", "constitution.md"))
	for _, want := range []string{
		"Language: Go",
		"Package manager: go modules",
		"Folders: cmd, internal",
		"Framework: go test (stdlib)",
		"Build tool: go build",
		"Makefile: true",
		"CI: GitHub Actions",
		"Linters: golangci-lint",
		"Formatters: gofmt",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("constitution missing %q\n---\n%s", want, body)
		}
	}
	// Conventions/glossary remain pending.
	if !strings.Contains(body, "_pending: true") {
		t.Fatalf("expected at least one pending section (conventions/glossary)")
	}
}

// TestInit_PreservesUserFiles: T035.
func TestInit_PreservesUserFiles(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)

	// First init.
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	// User edits one of the materialised files.
	custom := filepath.Join(root, ".bender", "groups.yaml")
	if err := os.WriteFile(custom, []byte("groups: { custom: { description: mine, select: { explicit: [foo] } } }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Second init without --force.
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("re-init: %v\n%s", err, out)
	}
	got := mustRead(t, custom)
	if !strings.Contains(got, "custom: { description: mine") {
		t.Fatalf("user-edited file was overwritten without --force; body:\n%s", got)
	}

	// Third init with --force: defaults restored.
	if out, err := runBender(t, bin, root, "init", "--force"); err != nil {
		t.Fatalf("forced re-init: %v\n%s", err, out)
	}
	got = mustRead(t, custom)
	if strings.Contains(got, "custom: { description: mine") {
		t.Fatalf("--force did not restore the embedded default; body:\n%s", got)
	}
}

func mustExist(tb testing.TB, root, rel string) {
	tb.Helper()
	if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
		tb.Fatalf("expected %s to exist: %v", rel, err)
	}
}

func mustNotExist(tb testing.TB, root, rel string) {
	tb.Helper()
	if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
		tb.Fatalf("expected %s to NOT exist", rel)
	}
}

func mustRead(tb testing.TB, path string) string {
	tb.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
