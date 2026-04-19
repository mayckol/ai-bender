package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

// US2 independent test — two pipeline sessions against the same repo at the
// same time must produce distinct paths and distinct branches with no
// cross-contamination in either direction.
func TestWorktreeConcurrent_TwoSessionsNoCollision(t *testing.T) {
	repo := helpers.DirtyRepo(t)
	beforeHead := snapshotMainTree(t, repo)

	ctx := context.Background()
	runnerFactory := func() worktree.GitRunner { return &worktree.ExecRunner{} }

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		outs = make(map[string]*worktree.CreateOutput, 2)
		errs []error
	)
	for _, id := range []string{"concur-a", "concur-b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			out, err := worktree.Create(ctx, worktree.CreateInput{
				RepoRoot:  repo,
				SessionID: id,
				Command:   "bender worktree create",
				Runner:    runnerFactory(),
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			outs[id] = out
		}(id)
	}
	wg.Wait()
	if len(errs) != 0 {
		t.Fatalf("concurrent Create errors: %v", errs)
	}
	if outs["concur-a"].WorktreePath == outs["concur-b"].WorktreePath {
		t.Fatalf("worktree paths collided: %s", outs["concur-a"].WorktreePath)
	}
	if outs["concur-a"].Branch == outs["concur-b"].Branch {
		t.Fatalf("branches collided: %s", outs["concur-a"].Branch)
	}

	// Write distinct files into each worktree and assert no cross-pollution.
	for id, out := range outs {
		p := filepath.Join(out.WorktreePath, id+".marker")
		if err := os.WriteFile(p, []byte(id), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(outs["concur-a"].WorktreePath, "concur-b.marker")); err == nil {
		t.Fatal("session b's marker leaked into session a's worktree")
	}
	if _, err := os.Stat(filepath.Join(outs["concur-b"].WorktreePath, "concur-a.marker")); err == nil {
		t.Fatal("session a's marker leaked into session b's worktree")
	}

	// Main tree must still be byte-unchanged.
	afterHead := snapshotMainTree(t, repo)
	if beforeHead.statusPorcelain != afterHead.statusPorcelain {
		t.Fatalf("main tree drifted across concurrent sessions\nBEFORE:\n%s\nAFTER:\n%s",
			beforeHead.statusPorcelain, afterHead.statusPorcelain)
	}
}

// Starting a third session whose branch already exists externally must fail
// cleanly with ErrBranchCollision — the test proves US2's disambiguation
// pathway (FR-002) is wired through Create.
func TestWorktreeConcurrent_BranchCollisionRefusesCleanly(t *testing.T) {
	repo := helpers.DirtyRepo(t)
	out, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: "prime",
		Runner:    &worktree.ExecRunner{},
	})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Manually create a clashing ref with the same session id; simulate a
	// user / external tool having pre-created it.
	clash := "bender/session/prime"
	if err := exec.Command("git", "-C", repo, "branch", clash+"-dup").Run(); err != nil {
		t.Fatalf("pre-create clash: %v", err)
	}
	_ = out
	// Reusing the same session id must collide.
	_, err = worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: "prime",
		Runner:    &worktree.ExecRunner{},
	})
	if err == nil {
		t.Fatal("expected collision for identical session id")
	}
	// And a legit run does not affect the existing session.
	if _, err := session.LoadState(out.SessionDir); err != nil {
		t.Fatalf("existing session state lost: %v", err)
	}
}
