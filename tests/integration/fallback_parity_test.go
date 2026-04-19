package integration

import (
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

// TestFallbackParity_BashCreateMirrorsBinary asserts that the Bash fallback's
// `create` verb materialises a worktree with the same on-disk shape the Go
// binary's worktree.Create produces: identical session-state fields, a live
// worktree directory, a session branch in the refs, and a worktree_created
// event line. Byte-level equality is not expected because the scripts derive
// their own timestamps and stringify state.json slightly differently than
// encoding/json; the contract is semantic parity.
func TestFallbackParity_BashCreateMirrorsBinary(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not on PATH")
	}
	bashPath := repoRelPath(t, ".specify/extensions/worktree/scripts/bash/worktree.sh")
	if _, err := os.Stat(bashPath); err != nil {
		t.Skipf("fallback script missing: %v", err)
	}

	// Leg 1: binary path.
	repoA := helpers.NewSeededRepo(t)
	outA, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repoA,
		SessionID: "parity-a",
		Runner:    &worktree.ExecRunner{},
	})
	if err != nil {
		t.Fatalf("binary Create: %v", err)
	}

	// Leg 2: Bash path against a fresh fixture.
	repoB := helpers.NewSeededRepo(t)
	cmd := exec.Command("bash", bashPath, "create", "parity-b")
	cmd.Dir = repoB
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash worktree.sh create: %v\n%s", err, out)
	}

	// Compare structural invariants.
	stA, err := session.LoadState(outA.SessionDir)
	if err != nil {
		t.Fatalf("load state A: %v", err)
	}
	stB, err := session.LoadState(filepath.Join(repoB, ".bender", "sessions", "parity-b"))
	if err != nil {
		t.Fatalf("load state B: %v", err)
	}

	if stA.SchemaVersion != stB.SchemaVersion {
		t.Errorf("schema_version: binary=%d bash=%d", stA.SchemaVersion, stB.SchemaVersion)
	}
	if stA.Worktree.Status != stB.Worktree.Status {
		t.Errorf("worktree.status: binary=%q bash=%q", stA.Worktree.Status, stB.Worktree.Status)
	}
	if !strings.HasPrefix(stA.SessionBranch, "refs/heads/bender/session/") ||
		!strings.HasPrefix(stB.SessionBranch, "refs/heads/bender/session/") {
		t.Errorf("session_branch drift: A=%q B=%q", stA.SessionBranch, stB.SessionBranch)
	}
	if stA.BaseBranch == "" || stB.BaseBranch == "" {
		t.Errorf("base_branch unset: A=%q B=%q", stA.BaseBranch, stB.BaseBranch)
	}
	if _, err := os.Stat(stA.Worktree.Path); err != nil {
		t.Errorf("binary worktree dir missing: %v", err)
	}
	if _, err := os.Stat(stB.Worktree.Path); err != nil {
		t.Errorf("bash worktree dir missing: %v", err)
	}

	// events.jsonl on both sides carries worktree_created.
	for label, sd := range map[string]string{"binary": outA.SessionDir,
		"bash": filepath.Join(repoB, ".bender", "sessions", "parity-b")} {
		raw, err := os.ReadFile(filepath.Join(sd, "events.jsonl"))
		if err != nil {
			t.Errorf("%s: read events: %v", label, err)
			continue
		}
		if !strings.Contains(string(raw), `"type":"worktree_created"`) {
			t.Errorf("%s: worktree_created event missing:\n%s", label, raw)
		}
	}
}

// repoRelPath resolves a path relative to the repo root for use inside tests.
// t.TempDir() gives us a scratch space but our tests live inside the main
// repo so we can just use the working directory + rel, verifying presence.
func repoRelPath(t *testing.T, rel string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// tests/integration is two levels below repo root.
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	return filepath.Join(root, rel)
}
