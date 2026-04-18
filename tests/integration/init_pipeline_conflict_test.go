package integration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPipelineConflict_WritesProposedAndExits4: FR-015.
// After a successful init, modify pipeline.yaml on disk; a subsequent init
// with a different selection must write pipeline.yaml.proposed, leave the
// original untouched, and exit with code 4.
func TestPipelineConflict_WritesProposedAndExits4(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)

	// First init — baseline full selection.
	if _, _, code := runBenderExit(t, bin, work, "init"); code != 0 {
		t.Fatal("baseline init failed")
	}

	// User edits pipeline.yaml (non-trivial change so rendered differs).
	pipelinePath := filepath.Join(work, ".bender", "pipeline.yaml")
	data, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("read pipeline.yaml: %v", err)
	}
	edited := append(data, []byte("\n# user note\n")...)
	if err := os.WriteFile(pipelinePath, edited, 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-init with different selection → drift + different content = conflict.
	stdout, stderr, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker")
	if code != 4 {
		t.Fatalf("want exit 4, got %d\nstdout: %s\nstderr: %s", code, stdout, stderr)
	}
	// Original file must be untouched.
	after, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("read pipeline.yaml (after): %v", err)
	}
	if !bytes.Equal(after, edited) {
		t.Errorf("pipeline.yaml was modified on conflict; expected byte-identical")
	}
	// .proposed sidecar must exist with the generated content (no benchmarker).
	proposedBytes, err := os.ReadFile(pipelinePath + ".proposed")
	if err != nil {
		t.Fatalf("read .proposed: %v", err)
	}
	if bytes.Contains(proposedBytes, []byte("benchmarker")) {
		t.Errorf(".proposed should not reference benchmarker: %s", proposedBytes)
	}
	// Stderr should mention the conflict.
	if !strings.Contains(stderr, "pipeline.yaml") {
		t.Errorf("stderr should mention pipeline.yaml: %s", stderr)
	}
}

// TestPipelineConflict_ForceOverwrites: --force resolves the conflict by
// overwriting the user's file and removing .proposed.
func TestPipelineConflict_ForceOverwrites(t *testing.T) {
	bin := buildBenderOnce(t)
	work := mkProject(t)

	if _, _, code := runBenderExit(t, bin, work, "init"); code != 0 {
		t.Fatal("baseline init failed")
	}
	pipelinePath := filepath.Join(work, ".bender", "pipeline.yaml")
	data, _ := os.ReadFile(pipelinePath)
	if err := os.WriteFile(pipelinePath, append(data, []byte("\n# user note\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runBenderExit(t, bin, work, "init", "--without", "benchmarker", "--force")
	if code != 0 {
		t.Fatalf("want exit 0 with --force, got %d", code)
	}
	after, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("read pipeline.yaml: %v", err)
	}
	if bytes.Contains(after, []byte("benchmarker")) {
		t.Errorf("--force should have pruned benchmarker: %s", after)
	}
	if _, err := os.Stat(pipelinePath + ".proposed"); !os.IsNotExist(err) {
		t.Errorf(".proposed should be cleaned up after --force")
	}
}
