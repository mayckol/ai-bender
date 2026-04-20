package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventCmd_Help(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"event", "--help"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out.String(), "emit") {
		t.Fatalf("event --help missing 'emit':\n%s", out.String())
	}
}

func TestEventEmit_MissingFlags(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"event", "emit"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing flags")
	}
}

func TestEventEmit_AppendsLine(t *testing.T) {
	project := t.TempDir()
	sid := "2026-04-20T13-15-22-abc"
	sessionDir := filepath.Join(project, ".bender", "sessions", sid)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	root := newRootCmd()
	root.SetArgs([]string{
		"event", "emit",
		"--sessions-root", filepath.Join(project, ".bender", "sessions"),
		"--session", sid,
		"--type", "orchestrator_progress",
		"--actor-kind", "orchestrator",
		"--actor-name", "ghu",
		"--payload", `{"percent":25,"current_step":"scout","completed_nodes":2,"total_nodes":8}`,
	})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr=%s", err, errBuf.String())
	}

	data, err := os.ReadFile(filepath.Join(sessionDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events.jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"percent":25`) {
		t.Fatalf("events.jsonl missing expected payload: %s", data)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("events.jsonl line must end with newline")
	}
}

func TestEventEmit_RejectsBadPayloadJSON(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{
		"event", "emit",
		"--session", "x",
		"--type", "orchestrator_progress",
		"--actor-kind", "orchestrator",
		"--actor-name", "ghu",
		"--payload", `not-json`,
	})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}
