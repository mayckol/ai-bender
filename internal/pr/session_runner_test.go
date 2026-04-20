package pr

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mayckol/ai-bender/internal/session"
)

func writeState(t *testing.T, dir string, st session.State) {
	t.Helper()
	if err := session.SaveState(dir, &st); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func TestRunForSession_SkipsMissingSession(t *testing.T) {
	root := t.TempDir()
	res, err := RunForSession(context.Background(), SessionRunOptions{
		ProjectRoot: root,
		SessionID:   "no-such-id",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Skipped || res.Reason != ReasonSessionMissing {
		t.Fatalf("want skip(session_missing), got %+v", res)
	}
}

func TestRunForSession_SkipsLegacy(t *testing.T) {
	root := t.TempDir()
	sid := "legacy-001"
	writeState(t, filepath.Join(root, ".bender", "sessions", sid), session.State{
		SchemaVersion: 1,
		SessionID:     sid,
		Command:       "/ghu",
		Status:        "completed",
	})
	res, err := RunForSession(context.Background(), SessionRunOptions{
		ProjectRoot: root,
		SessionID:   sid,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Skipped || res.Reason != ReasonLegacySession {
		t.Fatalf("want skip(legacy), got %+v", res)
	}
}

func TestRunForSession_SkipsActive(t *testing.T) {
	root := t.TempDir()
	sid := "active-001"
	writeState(t, filepath.Join(root, ".bender", "sessions", sid), session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     sid,
		Command:       "/ghu",
		Status:        "running",
		SessionBranch: "refs/heads/bender/session/active-001",
		BaseBranch:    "main",
		BaseSHA:       "deadbeef",
		Worktree: session.Worktree{
			Path:   "/tmp/wt",
			Status: session.WorktreeActive,
		},
	})
	res, err := RunForSession(context.Background(), SessionRunOptions{
		ProjectRoot: root,
		SessionID:   sid,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.Skipped || res.Reason != ReasonActive {
		t.Fatalf("want skip(session_active), got %+v", res)
	}
}

func TestRunForSession_RequiresProjectAndSession(t *testing.T) {
	if _, err := RunForSession(context.Background(), SessionRunOptions{}); err == nil {
		t.Fatal("expected error for empty ProjectRoot")
	}
	if _, err := RunForSession(context.Background(), SessionRunOptions{ProjectRoot: t.TempDir()}); err == nil {
		t.Fatal("expected error for empty SessionID")
	}
}
