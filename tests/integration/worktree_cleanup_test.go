package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

// US3 independent test — happy-path cleanup.
func TestWorktreeCleanup_RemovesDirAndMetadataKeepsBranch(t *testing.T) {
	repo := helpers.NewSeededRepo(t)
	runner := &worktree.ExecRunner{}

	out, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: "cleanup-e2e",
		Runner:    runner,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Advance session to a terminal state so Remove accepts it.
	markCompleted(t, out.SessionDir)

	res, err := worktree.Remove(context.Background(), worktree.RemoveInput{
		RepoRoot:  repo,
		SessionID: out.SessionID,
		Runner:    runner,
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if res.Reason != "cleanup" {
		t.Fatalf("reason: got %q want cleanup", res.Reason)
	}
	if _, err := os.Stat(out.WorktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree dir still present: %v", err)
	}
	// git worktree list must no longer mention this path.
	list, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git worktree list: %v", err)
	}
	if bytes.Contains(list, []byte(out.WorktreePath)) {
		t.Fatalf("stale worktree metadata:\n%s", list)
	}
	// Session branch MUST still exist.
	if err := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet",
		"refs/heads/"+out.Branch).Run(); err != nil {
		t.Fatalf("session branch was deleted: %v", err)
	}
	// State flagged as removed with a removed_at timestamp.
	st, err := session.LoadState(out.SessionDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Worktree.Status != session.WorktreeRemoved || st.Worktree.RemovedAt == nil {
		t.Fatalf("state not transitioned: %+v", st.Worktree)
	}
	ev, err := os.ReadFile(filepath.Join(out.SessionDir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(ev, []byte(`"type":"worktree_removed"`)) {
		t.Fatalf("worktree_removed event missing:\n%s", ev)
	}
}

// US3 reconcile-missing path — worktree dir deleted out-of-band, Remove
// must succeed with reason=reconciled-missing and prune git metadata.
func TestWorktreeCleanup_ReconcilesMissing(t *testing.T) {
	repo := helpers.NewSeededRepo(t)
	runner := &worktree.ExecRunner{}
	out, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: "reconcile-e2e",
		Runner:    runner,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	markCompleted(t, out.SessionDir)
	// Delete the worktree dir out-of-band.
	if err := os.RemoveAll(out.WorktreePath); err != nil {
		t.Fatal(err)
	}
	res, err := worktree.Remove(context.Background(), worktree.RemoveInput{
		RepoRoot:  repo,
		SessionID: out.SessionID,
		Runner:    runner,
	})
	if err != nil {
		t.Fatalf("Remove (reconcile): %v", err)
	}
	if res.Reason != "reconciled-missing" {
		t.Fatalf("reason: got %q want reconciled-missing", res.Reason)
	}
	// git worktree list must not contain this path.
	list, err := exec.Command("git", "-C", repo, "worktree", "list", "--porcelain").Output()
	if err != nil {
		t.Fatalf("git worktree list: %v", err)
	}
	if strings.Contains(string(list), out.WorktreePath) {
		t.Fatalf("stale metadata remained:\n%s", list)
	}
}

func markCompleted(t *testing.T, sessionDir string) {
	t.Helper()
	st, err := session.LoadState(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	st.Status = "completed"
	st.CompletedAt = time.Now().UTC()
	if err := session.SaveState(sessionDir, st); err != nil {
		t.Fatal(err)
	}
}
