package integration

import (
	"context"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/internal/workflow"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

// TestWorkflowLinkage_TwoSessionsShareID asserts that when two sessions are
// created on the same feature branch within the lookback window, the second
// inherits the first's workflow_id and records the first as its parent.
func TestWorkflowLinkage_TwoSessionsShareID(t *testing.T) {
	repo := helpers.NewSeededRepo(t)

	firstOut, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:   repo,
		SessionID:  "tdd-001",
		Runner:     &worktree.ExecRunner{},
		WorkflowID: "wf-seed-123",
	})
	if err != nil {
		t.Fatalf("tdd session create: %v", err)
	}

	lk, err := workflow.Resolve(workflow.ResolveParams{
		ProjectRoot: repo,
		Key:         firstOut.Branch,
		MaxAge:      1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if lk.WorkflowIsNew {
		t.Fatalf("expected to inherit, got new: %+v", lk)
	}

	if _, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:                repo,
		SessionID:               "ghu-002",
		Runner:                  &worktree.ExecRunner{},
		BaseBranch:              firstOut.Branch,
		WorkflowID:              lk.WorkflowID,
		WorkflowParentSessionID: lk.ParentSession,
	}); err != nil {
		t.Fatalf("ghu session create: %v", err)
	}

	st1, err := session.LoadState(firstOut.SessionDir)
	if err != nil {
		t.Fatalf("load tdd: %v", err)
	}
	st2, err := session.LoadState(repo + "/.bender/sessions/ghu-002")
	if err != nil {
		t.Fatalf("load ghu: %v", err)
	}

	if st1.WorkflowID == "" {
		t.Fatal("tdd session did not persist WorkflowID")
	}
	if st2.WorkflowID != st1.WorkflowID {
		t.Errorf("workflow_id mismatch: tdd=%s ghu=%s", st1.WorkflowID, st2.WorkflowID)
	}
	if st2.WorkflowParentSessionID != st1.SessionID {
		t.Errorf("ghu workflow_parent_session_id: got %q want %q",
			st2.WorkflowParentSessionID, st1.SessionID)
	}
}
