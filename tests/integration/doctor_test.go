package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctor_HealthyOnFreshInit: T086.
func TestDoctor_HealthyOnFreshInit(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	out, err := runBender(t, bin, root, "doctor")
	if err != nil {
		t.Fatalf("doctor exited non-zero on healthy fixture: %v\n%s", err, out)
	}
	if !strings.Contains(out, "status: healthy") {
		t.Fatalf("expected healthy status; got:\n%s", out)
	}
}

// TestDoctor_BlocksOnBrokenSkill: T088.
// Drop a skill file with no frontmatter; doctor should report a parse error and exit 41.
func TestDoctor_BlocksOnBrokenSkill(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	bad := filepath.Join(root, ".claude", "skills", "broken-skill")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "SKILL.md"), []byte("not yaml frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runBender(t, bin, root, "doctor")
	if err == nil {
		t.Fatalf("expected non-zero exit on broken skill; got:\n%s", out)
	}
	if !strings.Contains(out, "broken-skill") || !strings.Contains(out, "frontmatter") {
		t.Fatalf("expected parse error mentioning broken-skill and frontmatter:\n%s", out)
	}
}
