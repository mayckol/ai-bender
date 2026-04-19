package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

// US1 independent test — worktree isolation against a dirty main working tree.
//
// Starts a real git worktree against a seeded fixture, then asserts:
//   - the main working tree's tracked+untracked state is byte-unchanged
//     before vs. after,
//   - the worktree directory exists on disk,
//   - the session branch exists in the repo's ref store,
//   - state.json and events.jsonl were persisted with v2 worktree fields.
func TestWorktreeIsolation_EndToEnd(t *testing.T) {
	repo := helpers.DirtyRepo(t)

	before := snapshotMainTree(t, repo)
	ctx := context.Background()
	out, err := worktree.Create(ctx, worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: "isolation-e2e",
		Command:   "bender worktree create",
		Runner:    &worktree.ExecRunner{},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	after := snapshotMainTree(t, repo)

	if before.trackedHash != after.trackedHash {
		t.Errorf("main tree tracked hash changed\nBEFORE:\n%s\nAFTER:\n%s",
			before.trackedRaw, after.trackedRaw)
	}
	if before.untracked != after.untracked {
		t.Errorf("main tree untracked changed\nBEFORE:\n%s\nAFTER:\n%s",
			before.untracked, after.untracked)
	}
	if before.statusPorcelain != after.statusPorcelain {
		t.Errorf("git status changed\nBEFORE:\n%s\nAFTER:\n%s",
			before.statusPorcelain, after.statusPorcelain)
	}
	if before.headRef != after.headRef {
		t.Errorf("HEAD branch changed: %s -> %s", before.headRef, after.headRef)
	}

	if _, err := os.Stat(out.WorktreePath); err != nil {
		t.Fatalf("worktree path missing after create: %v", err)
	}
	// Branch must exist in the main repo's refs.
	cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet",
		"refs/heads/"+out.Branch)
	if err := cmd.Run(); err != nil {
		t.Fatalf("session branch missing: %v", err)
	}

	st, err := session.LoadState(out.SessionDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.SchemaVersion != session.SchemaVersion {
		t.Fatalf("schema_version: got %d want %d", st.SchemaVersion, session.SchemaVersion)
	}
	if st.Worktree.Status != session.WorktreeActive {
		t.Fatalf("worktree status: got %q", st.Worktree.Status)
	}

	evPath := filepath.Join(out.SessionDir, "events.jsonl")
	raw, err := os.ReadFile(evPath)
	if err != nil {
		t.Fatalf("read events.jsonl: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"type":"worktree_created"`)) {
		t.Fatalf("worktree_created event missing:\n%s", raw)
	}

	// Writes inside the worktree must not bleed into the main tree.
	sideFile := filepath.Join(out.WorktreePath, "agent-written.txt")
	if err := os.WriteFile(sideFile, []byte("inside worktree only"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainSide := filepath.Join(repo, "agent-written.txt")
	if _, err := os.Stat(mainSide); err == nil {
		t.Fatalf("write leaked into main tree: %s exists", mainSide)
	}
}

func TestWorktreeIsolation_RefusesBareRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	bare := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", "-q")
	cmd.Dir = bare
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	_, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  bare,
		SessionID: "bare-refuse",
		Runner:    &worktree.ExecRunner{},
	})
	if err == nil {
		t.Fatal("expected refusal for bare repo")
	}
	if !strings.Contains(err.Error(), "bare") {
		t.Fatalf("expected refusal to mention 'bare', got: %v", err)
	}
}

type mainTreeSnapshot struct {
	statusPorcelain string
	headRef         string
	trackedHash     string
	trackedRaw      string
	untracked       string
}

func snapshotMainTree(t *testing.T, repo string) mainTreeSnapshot {
	t.Helper()
	porc, err := exec.Command("git", "-C", repo, "status", "--porcelain=v2", "--untracked-files=all").Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	head, err := exec.Command("git", "-C", repo, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	tracked, err := exec.Command("git", "-C", repo, "ls-files", "--stage").Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var untracked strings.Builder
	cmd := exec.Command("git", "-C", repo, "ls-files", "--others", "--exclude-standard")
	cmd.Stdout = &untracked
	if err := cmd.Run(); err != nil {
		t.Fatalf("git ls-files --others: %v", err)
	}
	return mainTreeSnapshot{
		statusPorcelain: strings.TrimSpace(string(porc)),
		headRef:         strings.TrimSpace(string(head)),
		trackedHash:     hashBytes(tracked),
		trackedRaw:      string(tracked),
		untracked:       untracked.String(),
	}
}

func hashBytes(b []byte) string {
	// cheap deterministic hash so test errors don't print the whole tree.
	var h uint64 = 14695981039346656037
	for _, x := range b {
		h ^= uint64(x)
		h *= 1099511628211
	}
	return strings.ToLower(fmtHex(h))
}

func fmtHex(x uint64) string {
	const digits = "0123456789abcdef"
	var buf [16]byte
	for i := 15; i >= 0; i-- {
		buf[i] = digits[x&0xf]
		x >>= 4
	}
	return string(buf[:])
}
