package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration_OldLayoutToNew (T036): a project with the old pre-v0.17
// layout (`.claude/groups.yaml`, no `.bender/pipeline.yaml`) must migrate
// cleanly via `sync-defaults --force`. Legacy file is left in place;
// `bender doctor` reports healthy.
func TestMigration_OldLayoutToNew(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)

	// Plant the old layout: a user-edited `.claude/groups.yaml`, no .bender/.
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyGroups := []byte("groups:\n  legacy-pre-v017:\n    description: \"user edit from before the migration\"\n    select: { explicit: [bg-scout-explore] }\n    ordered: false\n")
	legacyPath := filepath.Join(claudeDir, "groups.yaml")
	if err := os.WriteFile(legacyPath, legacyGroups, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runBender(t, bin, root, "sync-defaults", "--force")
	if err != nil {
		t.Fatalf("sync-defaults: %v\n%s", err, out)
	}

	// New files landed.
	mustExist(t, root, ".bender/pipeline.yaml")
	mustExist(t, root, ".bender/groups.yaml")

	// Legacy file untouched (bytes match what we planted).
	got, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("read legacy: %v", err)
	}
	if string(got) != string(legacyGroups) {
		t.Fatalf("legacy .claude/groups.yaml was mutated:\n%s", string(got))
	}

	// Doctor agrees on healthy.
	doctorOut, err := runBender(t, bin, root, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, doctorOut)
	}
	if !strings.Contains(doctorOut, "status: healthy") {
		t.Fatalf("expected 'status: healthy', got:\n%s", doctorOut)
	}
}

// TestMigration_IsNonDestructive (T037): hand-edited `.bender/pipeline.yaml`
// survives a non-force `sync-defaults`.
func TestMigration_IsNonDestructive(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// User hand-edits the pipeline: bump max_concurrent to 2 so we can detect it.
	pipelinePath := filepath.Join(root, ".bender", "pipeline.yaml")
	edited := []byte("schema_version: 1\npipeline:\n  id: user-edited\n  description: \"custom pipeline\"\n  max_concurrent: 2\nnodes:\n  - id: only\n    agent: scout\n    skill: bg-scout-explore\n")
	if err := os.WriteFile(pipelinePath, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runBender(t, bin, root, "sync-defaults"); err != nil {
		t.Fatalf("sync-defaults: %v\n%s", err, out)
	}

	got, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("read pipeline: %v", err)
	}
	if !strings.Contains(string(got), "id: user-edited") {
		t.Fatalf("hand-edited pipeline was overwritten:\n%s", string(got))
	}
}
