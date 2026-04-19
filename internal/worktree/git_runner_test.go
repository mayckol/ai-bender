package worktree

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestFakeRunner_RecordsCalls(t *testing.T) {
	f := &FakeRunner{
		Responses: map[string]FakeResponse{
			"rev-parse": {Stdout: []byte("abc123\n")},
		},
	}
	stdout, _, err := f.Run(context.Background(), "/tmp", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(stdout) != "abc123\n" {
		t.Fatalf("stdout: got %q want %q", stdout, "abc123\n")
	}
	if len(f.Calls) != 1 {
		t.Fatalf("Calls: got %d want 1", len(f.Calls))
	}
	if f.Calls[0].CWD != "/tmp" {
		t.Fatalf("CWD: got %q want /tmp", f.Calls[0].CWD)
	}
	if f.Calls[0].Args[0] != "rev-parse" || f.Calls[0].Args[1] != "HEAD" {
		t.Fatalf("Args: got %v", f.Calls[0].Args)
	}
}

func TestFakeRunner_UnregisteredSubcommandReturnsNothing(t *testing.T) {
	f := &FakeRunner{}
	stdout, stderr, err := f.Run(context.Background(), "", "diff")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(stdout) != 0 || len(stderr) != 0 {
		t.Fatalf("empty response expected, got stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestFakeRunner_PropagatesErr(t *testing.T) {
	sentinel := errors.New("bang")
	f := &FakeRunner{
		Responses: map[string]FakeResponse{
			"worktree": {Err: sentinel},
		},
	}
	_, _, err := f.Run(context.Background(), "", "worktree", "add")
	if !errors.Is(err, sentinel) {
		t.Fatalf("want sentinel err, got %v", err)
	}
}

// TestExecRunner_RealGit is a smoke test that the production runner can talk
// to the real git binary. Skipped when git is absent so it doesn't fail on
// slim containers.
func TestExecRunner_RealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	r := &ExecRunner{}
	if err := ProbeGit(context.Background(), r); err != nil {
		t.Fatalf("ProbeGit: %v", err)
	}
}
