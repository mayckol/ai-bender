package integration_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitPRPreference_SurvivesReRun asserts that when a project already
// carries `preferences.open_pr_on_success: true` in its selection.yaml,
// running `bender init --no-interactive` preserves that preference rather
// than silently dropping it (spec 007 FR-012).
func TestInitPRPreference_SurvivesReRun(t *testing.T) {
	binPath := buildBenderOnce(t)
	project := mkProject(t)

	seed := `schema_version: 1
components:
  benchmarker:
    selected: true
  sentinel:
    selected: true
  surgeon:
    selected: true
  mistakeinator:
    selected: true
preferences:
  open_pr_on_success: true
`
	seedPath := filepath.Join(project, ".bender", "selection.yaml")
	mustWriteFile(t, seedPath, seed)

	cmd := exec.Command(binPath, "init", "--here", "--no-interactive")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bender init: %v\n%s", err, out)
	}

	data := mustReadFile(t, seedPath)
	if !strings.Contains(data, "open_pr_on_success: true") {
		t.Fatalf("PR preference dropped on re-run:\n%s", data)
	}
}

// TestInitPRPreference_NoPreferenceBlockStaysClean asserts that a fresh
// project run in --no-interactive mode does NOT synthesise a preference
// block the user never asked for — defaults live in code, not on disk.
func TestInitPRPreference_NoPreferenceBlockStaysClean(t *testing.T) {
	binPath := buildBenderOnce(t)
	project := mkProject(t)

	cmd := exec.Command(binPath, "init", "--here", "--no-interactive")
	cmd.Dir = project
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bender init: %v\n%s", err, out)
	}
	data := mustReadFile(t, filepath.Join(project, ".bender", "selection.yaml"))
	if strings.Contains(data, "open_pr_on_success: true") {
		t.Fatalf("fresh project got an enabled preference it never set:\n%s", data)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := mkdirAllForFile(path); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := writeFileBytes(path, []byte(content)); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := readFileBytesHelper(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
