package workflow

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/session"
)

func seedSession(t *testing.T, projectRoot, id, branch, workflowID string, startedAt time.Time) {
	t.Helper()
	dir := filepath.Join(projectRoot, ".bender", "sessions", id)
	s := session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     id,
		Command:       "/ghu",
		StartedAt:     startedAt,
		Status:        "completed",
		SessionBranch: branch,
		WorkflowID:    workflowID,
	}
	if err := session.SaveState(dir, &s); err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestResolve_NewWorkflowWhenNoPriorMatch(t *testing.T) {
	root := t.TempDir()
	lk, err := Resolve(ResolveParams{ProjectRoot: root, Key: "feature-branch"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !lk.WorkflowIsNew {
		t.Fatalf("expected new workflow, got %+v", lk)
	}
	if lk.WorkflowID == "" {
		t.Fatal("minted workflow id must not be empty")
	}
	if lk.ParentSession != "" {
		t.Fatalf("new workflow must not have parent, got %q", lk.ParentSession)
	}
}

func TestResolve_InheritsFromMatchingRecent(t *testing.T) {
	root := t.TempDir()
	seedSession(t, root, "tdd-001", "feature-branch", "wf-abc", time.Now().UTC().Add(-10*time.Minute))

	lk, err := Resolve(ResolveParams{ProjectRoot: root, Key: "feature-branch"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if lk.WorkflowIsNew {
		t.Fatalf("expected to inherit, got new: %+v", lk)
	}
	if lk.WorkflowID != "wf-abc" {
		t.Errorf("want workflow_id wf-abc, got %q", lk.WorkflowID)
	}
	if lk.ParentSession != "tdd-001" {
		t.Errorf("want parent tdd-001, got %q", lk.ParentSession)
	}
}

func TestResolve_IgnoresStaleSessions(t *testing.T) {
	root := t.TempDir()
	seedSession(t, root, "old-001", "feature-branch", "wf-stale", time.Now().UTC().Add(-48*time.Hour))

	lk, err := Resolve(ResolveParams{
		ProjectRoot: root,
		Key:         "feature-branch",
		MaxAge:      24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !lk.WorkflowIsNew {
		t.Fatalf("expected new workflow (prior was stale), got %+v", lk)
	}
}

func TestResolve_IgnoresDifferentKey(t *testing.T) {
	root := t.TempDir()
	seedSession(t, root, "other-001", "other-branch", "wf-other", time.Now().UTC().Add(-5*time.Minute))

	lk, err := Resolve(ResolveParams{ProjectRoot: root, Key: "feature-branch"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !lk.WorkflowIsNew {
		t.Fatalf("expected new workflow (different key), got %+v", lk)
	}
}

func TestResolve_RequiresProjectRootAndKey(t *testing.T) {
	if _, err := Resolve(ResolveParams{}); err == nil {
		t.Fatal("expected error for empty ProjectRoot")
	}
	if _, err := Resolve(ResolveParams{ProjectRoot: t.TempDir()}); err == nil {
		t.Fatal("expected error for empty Key")
	}
}
