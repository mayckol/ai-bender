package integration_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInit_OptOut_Benchmarker: US1 Independent Test.
func TestInit_OptOut_Benchmarker(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)

	stdout, stderr, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker")
	if code != 0 {
		t.Fatalf("init --without benchmarker exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	// Benchmarker agent file must be absent.
	if _, err := os.Stat(filepath.Join(work, ".claude", "agents", "benchmarker.md")); !os.IsNotExist(err) {
		t.Errorf("benchmarker.md should be absent: %v", err)
	}
	// Benchmarker skill directories must be absent.
	for _, sk := range []string{"bg-benchmarker-analyze", "bg-benchmarker-measure"} {
		if _, err := os.Stat(filepath.Join(work, ".claude", "skills", sk)); !os.IsNotExist(err) {
			t.Errorf("skill %s should be absent: %v", sk, err)
		}
	}
	// Mandatory agents remain.
	for _, mand := range []string{"scout", "architect", "crafter", "tester", "linter", "reviewer", "scribe"} {
		if _, err := os.Stat(filepath.Join(work, ".claude", "agents", mand+".md")); err != nil {
			t.Errorf("mandatory agent %s absent: %v", mand, err)
		}
	}
	// Pipeline must not reference benchmarker.
	data, err := os.ReadFile(filepath.Join(work, ".bender", "pipeline.yaml"))
	if err != nil {
		t.Fatalf("read pipeline.yaml: %v", err)
	}
	if bytes.Contains(data, []byte("benchmarker")) {
		t.Errorf("pipeline.yaml still references benchmarker:\n%s", data)
	}
	// Selection manifest must record the exclusion.
	sel, err := os.ReadFile(filepath.Join(work, ".bender", "selection.yaml"))
	if err != nil {
		t.Fatalf("read selection.yaml: %v", err)
	}
	if !bytes.Contains(sel, []byte("benchmarker")) {
		t.Errorf("selection.yaml missing benchmarker entry:\n%s", sel)
	}
	if !bytes.Contains(sel, []byte("schema_version: 1")) {
		t.Errorf("selection.yaml missing schema_version:\n%s", sel)
	}
	// Summary mentions the exclusion.
	if !strings.Contains(stdout, "excluded") {
		t.Errorf("summary missing excluded block:\n%s", stdout)
	}
}

// TestInit_Full_Default_Regression: US1 — with no flags, every mandatory and
// every optional component's files must exist.
func TestInit_Full_Default_Regression(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)

	_, stderr, code := runBenderExit(t, bin, work, "init")
	if code != 0 {
		t.Fatalf("init exited %d\nstderr:\n%s", code, stderr)
	}

	for _, agent := range []string{"scout", "architect", "crafter", "tester", "linter", "reviewer", "scribe", "sentinel", "benchmarker", "surgeon", "mistakeinator"} {
		if _, err := os.Stat(filepath.Join(work, ".claude", "agents", agent+".md")); err != nil {
			t.Errorf("agent %s: %v", agent, err)
		}
	}
}

// TestInit_RefuseMandatoryDeselection: US1.
func TestInit_RefuseMandatoryDeselection(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)

	_, stderr, code := runBenderExit(t, bin, work, "init", "--without", "scout")
	if code != 2 {
		t.Fatalf("want exit 2, got %d\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "scout") {
		t.Errorf("stderr should name the rejected component: %s", stderr)
	}
	if !strings.Contains(stderr, "mandatory") {
		t.Errorf("stderr should mention 'mandatory': %s", stderr)
	}
	// Nothing should have been scaffolded.
	if _, err := os.Stat(filepath.Join(work, ".claude")); !os.IsNotExist(err) {
		t.Errorf(".claude/ should not exist after refusal: %v", err)
	}
}

// TestInit_RejectUnknownComponent: US1.
func TestInit_RejectUnknownComponent(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	_, stderr, code := runBenderExit(t, bin, work, "init", "--without", "nonexistent")
	if code != 2 {
		t.Fatalf("want exit 2, got %d\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "unknown") {
		t.Errorf("stderr should mention 'unknown': %s", stderr)
	}
}

// TestInit_ContradictoryFlags: US1.
func TestInit_ContradictoryFlags(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	_, stderr, code := runBenderExit(t, bin, work, "init", "--with", "benchmarker", "--without", "benchmarker")
	if code != 2 {
		t.Fatalf("want exit 2, got %d\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "benchmarker") {
		t.Errorf("stderr should name the contradicted component: %s", stderr)
	}
}

// TestInit_Idempotent: SC-003.
func TestInit_Idempotent(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker"); code != 0 {
		t.Fatal("first init failed")
	}
	snap := snapshotDir(t, work)
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker"); code != 0 {
		t.Fatal("second init failed")
	}
	after := snapshotDir(t, work)
	for path, hashA := range snap {
		hashB, ok := after[path]
		if !ok {
			t.Errorf("file disappeared on second run: %s", path)
			continue
		}
		if hashA != hashB {
			// constitution.md embeds a "Last Amended" timestamp which legitimately
			// changes between runs. That's the only expected drift.
			if strings.HasSuffix(path, "constitution.md") {
				continue
			}
			t.Errorf("file changed on idempotent re-run: %s", path)
		}
	}
	for path := range after {
		if _, ok := snap[path]; !ok {
			t.Errorf("file appeared only on second run: %s", path)
		}
	}
}

// TestInit_AddBackPreviouslySkipped: US2.
func TestInit_AddBackPreviouslySkipped(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker"); code != 0 {
		t.Fatal("lean init failed")
	}
	// Capture an unrelated file pre-re-run.
	unrelated := filepath.Join(work, ".claude", "agents", "scout.md")
	before, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatalf("read scout.md: %v", err)
	}
	// Re-init, adding benchmarker back.
	if _, _, code := runBenderExit(t, bin, work, "init", "--with", "benchmarker"); code != 0 {
		t.Fatal("add-back init failed")
	}
	if _, err := os.Stat(filepath.Join(work, ".claude", "agents", "benchmarker.md")); err != nil {
		t.Errorf("benchmarker.md should exist: %v", err)
	}
	after, err := os.ReadFile(unrelated)
	if err != nil {
		t.Fatalf("read scout.md (after): %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("scout.md was modified on re-run")
	}
}

// TestInit_RemovePreviouslyInstalled: US3.
// Start from full scaffold, re-init deselecting sentinel, assert sentinel
// files gone and unrelated files untouched.
func TestInit_RemovePreviouslyInstalled(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work, "init"); code != 0 {
		t.Fatal("full init failed")
	}
	// Pre-flight: sentinel files present.
	if _, err := os.Stat(filepath.Join(work, ".claude", "agents", "sentinel.md")); err != nil {
		t.Fatalf("pre-flight: sentinel.md: %v", err)
	}
	// Capture an unrelated file.
	unrelated := filepath.Join(work, ".claude", "agents", "scout.md")
	before, _ := os.ReadFile(unrelated)

	stdout, _, code := runBenderExit(t, bin, work, "init", "--without", "sentinel")
	if code != 0 {
		t.Fatalf("remove init failed (exit %d)", code)
	}

	// Sentinel files removed.
	if _, err := os.Stat(filepath.Join(work, ".claude", "agents", "sentinel.md")); !os.IsNotExist(err) {
		t.Errorf("sentinel.md should be removed: %v", err)
	}
	for _, sk := range []string{"bg-sentinel-static-scan", "bg-sentinel-runtime-paths"} {
		if _, err := os.Stat(filepath.Join(work, ".claude", "skills", sk)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed: %v", sk, err)
		}
	}
	// Unrelated file unchanged.
	after, _ := os.ReadFile(unrelated)
	if !bytes.Equal(before, after) {
		t.Errorf("scout.md should be untouched")
	}
	// Summary shows excluded.
	if !strings.Contains(stdout, "sentinel") {
		t.Errorf("summary should mention sentinel as excluded: %s", stdout)
	}
}

// TestInit_RemoveWithLocalEdits_PreservedWithoutForce: US3 safety.
// A user-edited file belonging to a deselected component is NOT removed
// without --force.
func TestInit_RemoveWithLocalEdits_PreservedWithoutForce(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work, "init"); code != 0 {
		t.Fatal("full init failed")
	}
	sentinelAgent := filepath.Join(work, ".claude", "agents", "sentinel.md")
	// Edit it.
	edited := []byte("# user-edited sentinel override\n")
	if err := os.WriteFile(sentinelAgent, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	// Deselect without --force.
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "sentinel"); code != 0 {
		t.Fatalf("re-init exited non-zero; user edit should just be preserved")
	}
	// File should remain on disk.
	after, err := os.ReadFile(sentinelAgent)
	if err != nil {
		t.Fatalf("sentinel.md should be preserved: %v", err)
	}
	if !bytes.Equal(after, edited) {
		t.Errorf("user-edited sentinel.md was modified: %s", after)
	}
	// --force removes it.
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "sentinel", "--force"); code != 0 {
		t.Fatalf("forced re-init failed (exit %d)", code)
	}
	if _, err := os.Stat(sentinelAgent); !os.IsNotExist(err) {
		t.Errorf("sentinel.md should be removed under --force")
	}
}

// TestInit_MistakeinatorDirective: Phase 5 — rendered plan SKILL contains or
// omits the mistakes-loading block based on selection.
func TestInit_MistakeinatorDirective(t *testing.T) {
	bin := buildBenderOnce(t)

	// Without mistakeinator → directive absent.
	work := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work, "init", "--without", "mistakeinator"); code != 0 {
		t.Fatal("lean init failed")
	}
	data, err := os.ReadFile(filepath.Join(work, ".claude", "skills", "plan", "SKILL.md"))
	if err != nil {
		t.Fatalf("read plan SKILL: %v", err)
	}
	if bytes.Contains(data, []byte("mistakes.md")) {
		t.Errorf("rendered plan SKILL should not reference mistakes.md when mistakeinator deselected")
	}

	// With mistakeinator → directive present.
	work2 := mkProject(t)
	if _, _, code := runBenderExit(t, bin, work2, "init"); code != 0 {
		t.Fatal("full init failed")
	}
	data2, err := os.ReadFile(filepath.Join(work2, ".claude", "skills", "plan", "SKILL.md"))
	if err != nil {
		t.Fatalf("read plan SKILL (with): %v", err)
	}
	if !bytes.Contains(data2, []byte("mistakes.md")) {
		t.Errorf("rendered plan SKILL should reference mistakes.md when mistakeinator selected:\n%s", data2)
	}
}

// snapshotDir returns rel-path → sha256-hex for every regular file under
// root. Used to assert idempotency (same selection + content ⇒ no writes).
func snapshotDir(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[rel] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return out
}
