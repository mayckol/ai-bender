package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestAfterImplementPR_DisabledByPreferenceSkips asserts the hook short-
// circuits when preferences.open_pr_on_success is absent or false.
func TestAfterImplementPR_DisabledByPreferenceSkips(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	script := repoRelPath(t, ".specify/extensions/pr/scripts/bash/open-pr.sh")
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".bender", "selection.yaml"), `schema_version: 1
components:
  scout:
    selected: true
preferences:
  open_pr_on_success: false
`)
	writeFile(t, filepath.Join(project, ".bender", "sessions", "s1", "state.json"),
		`{"schema_version":2,"session_id":"s1","command":"/ghu","status":"completed"}`)

	out, err := runHook(t, script, project, "s1")
	if err != nil {
		t.Fatalf("hook: %v\n%s", err, out)
	}
	if !strings.Contains(out, "disabled by preference") {
		t.Fatalf("missing skip log: %s", out)
	}
}

func TestAfterImplementPR_IncompleteSessionSkips(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	script := repoRelPath(t, ".specify/extensions/pr/scripts/bash/open-pr.sh")
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".bender", "selection.yaml"), `schema_version: 1
components:
  scout:
    selected: true
preferences:
  open_pr_on_success: true
`)
	writeFile(t, filepath.Join(project, ".bender", "sessions", "s1", "state.json"),
		`{"schema_version":2,"session_id":"s1","command":"/ghu","status":"running"}`)

	out, err := runHook(t, script, project, "s1")
	if err != nil {
		t.Fatalf("hook: %v\n%s", err, out)
	}
	if !strings.Contains(out, "session not completed") {
		t.Fatalf("missing skip log: %s", out)
	}
}

func TestAfterImplementPR_PreferenceOnCompletedSessionInvokesAdapter(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	script := repoRelPath(t, ".specify/extensions/pr/scripts/bash/open-pr.sh")
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".bender", "selection.yaml"), `schema_version: 1
components:
  scout:
    selected: true
preferences:
  open_pr_on_success: true
`)
	writeFile(t, filepath.Join(project, ".bender", "sessions", "s1", "state.json"),
		`{"schema_version":2,"session_id":"s1","command":"/ghu","status":"completed"}`)

	// Stub "bender" in a tiny shadow PATH: when the script shells to
	// `bender session pr s1` we want to record the call without hitting a
	// real adapter. Script exits 0 on any outcome so this also covers the
	// "adapter failure is non-fatal" branch.
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "bender"), `#!/usr/bin/env bash
echo "stub-bender called $@" >> "$STUB_LOG"
exit 42
`)
	if err := os.Chmod(filepath.Join(bin, "bender"), 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	stubLog := filepath.Join(bin, "calls.log")

	out, err := runHookWithEnv(t, script, project, "s1", map[string]string{
		"PATH":     bin + ":" + os.Getenv("PATH"),
		"STUB_LOG": stubLog,
	})
	if err != nil {
		t.Fatalf("hook: %v\n%s", err, out)
	}

	// Regardless of stub exit 42, hook exit is 0.
	if !strings.Contains(out, "bender session pr failed") && !strings.Contains(out, "pr opened/refreshed for s1") {
		t.Fatalf("unexpected hook output: %s", out)
	}
	data, _ := os.ReadFile(stubLog)
	if !strings.Contains(string(data), "session pr s1") {
		t.Fatalf("stub did not receive session pr call: %s", data)
	}
}

func runHook(t *testing.T, script, project, sessionID string) (string, error) {
	t.Helper()
	return runHookWithEnv(t, script, project, sessionID, map[string]string{})
}

func runHookWithEnv(t *testing.T, script, project, sessionID string, extra map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", script, sessionID)
	cmd.Env = append(os.Environ(), "SESSION_ID="+sessionID, "PROJECT_ROOT="+project)
	for k, v := range extra {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
