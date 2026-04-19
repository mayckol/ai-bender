package main

import (
	"bytes"
	"strings"
	"testing"
)

// Smoke: the top-level `worktree` command is registered and displays help.
func TestWorktreeCmd_Help(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"worktree", "--help"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	help := out.String()
	for _, want := range []string{"create", "list", "remove", "prune"} {
		if !strings.Contains(help, want) {
			t.Errorf("worktree --help missing %q:\n%s", want, help)
		}
	}
}

// Smoke: `worktree create` with no session id fails with a usage message.
func TestWorktreeCreate_MissingSessionID(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"worktree", "create"})
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for missing session-id")
	}
}
