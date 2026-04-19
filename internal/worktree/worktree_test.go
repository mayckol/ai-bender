package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/session"
)

func fakeCreateRunner(t *testing.T) *FakeRunner {
	t.Helper()
	return &FakeRunner{
		Responses: map[string]FakeResponse{
			"--version":  {Stdout: []byte("git version 2.42.0\n")},
			"rev-parse":  {Stdout: []byte("9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345\n")},
			"show-ref":   {Err: errors.New("exit 1")}, // absent => no collision
			"worktree":   {Stdout: []byte("")},        // add / remove / prune all succeed
		},
	}
}

func TestCreate_HappyPath(t *testing.T) {
	repo := t.TempDir()
	// rev-parse needs to serve three different responses in order:
	//   1. --is-bare-repository => "false\n"
	//   2. --abbrev-ref HEAD    => "main\n"
	//   3. --verify main        => SHA
	// FakeRunner only keys by subcommand, so we chain with a custom runner:
	runner := &sequencedRunner{
		responses: map[string][]string{
			"--version":  {"git version 2.42.0\n"},
			"rev-parse":  {"false\n", "main\n", "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345\n"},
			"show-ref":   {"__err__"},
			"worktree":   {""},
		},
	}
	now := time.Date(2026, 4, 18, 14, 30, 22, 0, time.UTC)
	out, err := Create(context.Background(), CreateInput{
		RepoRoot:  repo,
		SessionID: "sess-1",
		Command:   "bender worktree create",
		Runner:    runner,
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasSuffix(out.WorktreePath, filepath.Join(".bender", "worktrees", "sess-1")) {
		t.Fatalf("WorktreePath: got %q", out.WorktreePath)
	}
	if out.Branch != "bender/session/sess-1" {
		t.Fatalf("Branch: got %q", out.Branch)
	}
	if out.BaseBranch != "main" {
		t.Fatalf("BaseBranch: got %q", out.BaseBranch)
	}
	if out.CreatedAt != now {
		t.Fatalf("CreatedAt: got %v want %v", out.CreatedAt, now)
	}

	st, err := session.LoadState(out.SessionDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if st.Worktree.Status != session.WorktreeActive {
		t.Fatalf("worktree status: got %q", st.Worktree.Status)
	}
	if st.BaseSHA == "" || st.SessionBranch == "" {
		t.Fatalf("v2 fields empty: %+v", st)
	}

	ev, err := os.ReadFile(filepath.Join(out.SessionDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(ev), `"type":"worktree_created"`) {
		t.Fatalf("expected worktree_created event, got:\n%s", ev)
	}
}

func TestCreate_RefusesOnInvalidSessionID(t *testing.T) {
	_, err := Create(context.Background(), CreateInput{
		RepoRoot:  t.TempDir(),
		SessionID: "bender/session/nested",
		Runner:    fakeCreateRunner(t),
	})
	if err == nil {
		t.Fatal("expected error for invalid session id")
	}
}

func TestCreate_RefusesOnBareRepo(t *testing.T) {
	runner := &sequencedRunner{
		responses: map[string][]string{
			"--version": {"git version 2.42.0\n"},
			"rev-parse": {"true\n"}, // --is-bare-repository => true
		},
	}
	_, err := Create(context.Background(), CreateInput{
		RepoRoot:  t.TempDir(),
		SessionID: "sess-bare",
		Runner:    runner,
	})
	if !errors.Is(err, ErrRepoIncompatible) {
		t.Fatalf("want ErrRepoIncompatible, got %v", err)
	}
}

func TestCreate_RefusesOnBranchCollision(t *testing.T) {
	repo := t.TempDir()
	runner := &sequencedRunner{
		responses: map[string][]string{
			"--version": {"git version 2.42.0\n"},
			"rev-parse": {"false\n", "main\n", "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345\n"},
			"show-ref":  {""}, // empty stdout + nil err => present
		},
	}
	_, err := Create(context.Background(), CreateInput{
		RepoRoot:  repo,
		SessionID: "sess-c",
		Runner:    runner,
	})
	if !errors.Is(err, ErrBranchCollision) {
		t.Fatalf("want ErrBranchCollision, got %v", err)
	}
}

func TestCreate_RefusesOnMidRebase(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".git", "MERGE_HEAD"), []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &sequencedRunner{
		responses: map[string][]string{
			"--version": {"git version 2.42.0\n"},
			"rev-parse": {"false\n"},
		},
	}
	_, err := Create(context.Background(), CreateInput{
		RepoRoot:  repo,
		SessionID: "sess-m",
		Runner:    runner,
	})
	if !errors.Is(err, ErrRepoIncompatible) {
		t.Fatalf("want ErrRepoIncompatible, got %v", err)
	}
}

func TestList_SortsAndStats(t *testing.T) {
	repo := t.TempDir()
	// Plant two sessions with worktree paths, one present, one absent.
	s1 := filepath.Join(repo, ".bender", "sessions", "aaa")
	s2 := filepath.Join(repo, ".bender", "sessions", "bbb")
	wt1 := filepath.Join(repo, ".bender", "worktrees", "aaa")
	if err := os.MkdirAll(wt1, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []struct {
		dir, path string
	}{
		{s1, wt1},
		{s2, filepath.Join(repo, ".bender", "worktrees", "bbb-missing")},
	} {
		st := &session.State{
			SchemaVersion: session.SchemaVersion,
			SessionID:     filepath.Base(spec.dir),
			Command:       "bender worktree create",
			StartedAt:     time.Now().UTC(),
			Status:        "completed",
			CompletedAt:   time.Now().UTC(),
			SessionBranch: "refs/heads/bender/session/" + filepath.Base(spec.dir),
			BaseBranch:    "main",
			BaseSHA:       "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
			Worktree: session.Worktree{
				Path:      spec.path,
				Status:    session.WorktreeCompleted,
				CreatedAt: time.Now().UTC(),
			},
		}
		if err := session.SaveState(spec.dir, st); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := List(repo)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d want 2", len(rows))
	}
	if rows[0].SessionID != "aaa" || rows[1].SessionID != "bbb" {
		t.Fatalf("sort order: got %v, %v", rows[0].SessionID, rows[1].SessionID)
	}
	if !rows[0].PresentOnDisk || rows[1].PresentOnDisk {
		t.Fatalf("presence: got aaa=%v bbb=%v", rows[0].PresentOnDisk, rows[1].PresentOnDisk)
	}
}

func TestRemove_RefusesOnActive(t *testing.T) {
	repo := t.TempDir()
	id := "sess-active"
	sd := filepath.Join(repo, ".bender", "sessions", id)
	st := &session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     id,
		Command:       "x",
		StartedAt:     time.Now().UTC(),
		Status:        "running",
		SessionBranch: "refs/heads/bender/session/" + id,
		BaseBranch:    "main",
		BaseSHA:       "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
		Worktree: session.Worktree{
			Path:      filepath.Join(repo, ".bender", "worktrees", id),
			Status:    session.WorktreeActive,
			CreatedAt: time.Now().UTC(),
		},
	}
	if err := session.SaveState(sd, st); err != nil {
		t.Fatal(err)
	}
	_, err := Remove(context.Background(), RemoveInput{
		RepoRoot:  repo,
		SessionID: id,
		Runner:    fakeCreateRunner(t),
	})
	if !errors.Is(err, ErrActiveSession) {
		t.Fatalf("want ErrActiveSession, got %v", err)
	}
}

func TestRemove_ReconcilesMissing(t *testing.T) {
	repo := t.TempDir()
	id := "sess-missing"
	sd := filepath.Join(repo, ".bender", "sessions", id)
	wtPath := filepath.Join(repo, ".bender", "worktrees", id)
	st := &session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     id,
		Command:       "x",
		StartedAt:     time.Now().UTC().Add(-time.Hour),
		CompletedAt:   time.Now().UTC().Add(-30 * time.Minute),
		Status:        "completed",
		SessionBranch: "refs/heads/bender/session/" + id,
		BaseBranch:    "main",
		BaseSHA:       "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
		Worktree: session.Worktree{
			Path:      wtPath,
			Status:    session.WorktreeCompleted,
			CreatedAt: time.Now().UTC().Add(-time.Hour),
		},
	}
	if err := session.SaveState(sd, st); err != nil {
		t.Fatal(err)
	}
	// Note: wtPath does not exist on disk; Remove must reconcile.
	out, err := Remove(context.Background(), RemoveInput{
		RepoRoot:  repo,
		SessionID: id,
		Runner:    fakeCreateRunner(t),
	})
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if out.Reason != "reconciled-missing" {
		t.Fatalf("reason: got %q want reconciled-missing", out.Reason)
	}
}

func TestRemove_SessionNotFound(t *testing.T) {
	repo := t.TempDir()
	_, err := Remove(context.Background(), RemoveInput{
		RepoRoot:  repo,
		SessionID: "does-not-exist",
		Runner:    fakeCreateRunner(t),
	})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("want ErrSessionNotFound, got %v", err)
	}
}

func TestPrune_SkipsYoungerThanOlderThan(t *testing.T) {
	repo := t.TempDir()
	id := "sess-young"
	sd := filepath.Join(repo, ".bender", "sessions", id)
	wtPath := filepath.Join(repo, ".bender", "worktrees", id)
	now := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	st := &session.State{
		SchemaVersion: session.SchemaVersion,
		SessionID:     id,
		Command:       "x",
		StartedAt:     now.Add(-time.Hour),
		CompletedAt:   now.Add(-10 * time.Minute),
		Status:        "completed",
		SessionBranch: "refs/heads/bender/session/" + id,
		BaseBranch:    "main",
		BaseSHA:       "9f0a1b2c3d4e5f60718293a4b5c6d7e8f9012345",
		Worktree: session.Worktree{
			Path:      wtPath,
			Status:    session.WorktreeCompleted,
			CreatedAt: now.Add(-time.Hour),
		},
	}
	if err := session.SaveState(sd, st); err != nil {
		t.Fatal(err)
	}
	summary, err := Prune(context.Background(), PruneInput{
		RepoRoot:  repo,
		OlderThan: time.Hour,
		Runner:    fakeCreateRunner(t),
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if summary.Removed != 0 || summary.Reconciled != 0 {
		t.Fatalf("younger-than should be skipped; got %+v", summary)
	}
	if summary.Skipped != 1 {
		t.Fatalf("skipped: got %d want 1", summary.Skipped)
	}
}

// sequencedRunner returns canned responses in call order per subcommand, so
// tests can simulate several rev-parse invocations with different outputs.
type sequencedRunner struct {
	responses map[string][]string
	calls     int
	cursor    map[string]int
}

func (s *sequencedRunner) Run(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
	s.calls++
	if s.cursor == nil {
		s.cursor = map[string]int{}
	}
	if len(args) == 0 {
		return nil, nil, nil
	}
	key := args[0]
	resps, ok := s.responses[key]
	if !ok {
		return nil, nil, nil
	}
	idx := s.cursor[key]
	if idx >= len(resps) {
		idx = len(resps) - 1
	}
	s.cursor[key] = idx + 1
	raw := resps[idx]
	if raw == "__err__" {
		return nil, nil, errors.New("exit status 1")
	}
	return []byte(raw), nil, nil
}
