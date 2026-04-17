package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyncDefaults_PreservesUserFiles: T101.
func TestSyncDefaults_PreservesUserFiles(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	custom := filepath.Join(root, ".bender", "groups.yaml")
	if err := os.WriteFile(custom, []byte("groups: { custom: { description: mine, select: { explicit: [foo] } } }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runBender(t, bin, root, "sync-defaults"); err != nil {
		t.Fatalf("sync-defaults: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(custom)
	if !strings.Contains(string(got), "custom: { description: mine") {
		t.Fatalf("user-edited file was overwritten:\n%s", string(got))
	}
}

// TestSyncDefaults_ForceOverwrites: T102.
func TestSyncDefaults_ForceOverwrites(t *testing.T) {
	bin := buildBenderOnce(t)
	root := mkProject(t)
	if out, err := runBender(t, bin, root, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	custom := filepath.Join(root, ".bender", "groups.yaml")
	if err := os.WriteFile(custom, []byte("groups: { custom: { description: mine, select: { explicit: [foo] } } }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runBender(t, bin, root, "sync-defaults", "--force"); err != nil {
		t.Fatalf("sync-defaults --force: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(custom)
	if strings.Contains(string(got), "custom: { description: mine") {
		t.Fatalf("--force did not restore the embedded default:\n%s", string(got))
	}
}
