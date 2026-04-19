package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mayckol/ai-bender/internal/pr"
	"github.com/mayckol/ai-bender/internal/session"
	"github.com/mayckol/ai-bender/internal/worktree"
	"github.com/mayckol/ai-bender/tests/integration/helpers"
)

type fakeGitLabAdapter struct {
	pushes      int32
	opens       int32
	updates     int32
	existingURL string
	openURL     string
}

func (f *fakeGitLabAdapter) Name() string                      { return "glab" }
func (f *fakeGitLabAdapter) Detect(url string) bool            { return strings.Contains(url, "gitlab.com") }
func (f *fakeGitLabAdapter) AuthCheck(context.Context) error   { return nil }
func (f *fakeGitLabAdapter) Push(context.Context, pr.PushInput) error {
	atomic.AddInt32(&f.pushes, 1)
	return nil
}
func (f *fakeGitLabAdapter) OpenOrUpdate(_ context.Context, in pr.OpenArgs) (*pr.PRRef, error) {
	if f.existingURL != "" && in.RefuseUpdate {
		return &pr.PRRef{URL: f.existingURL}, pr.ErrExistingPRPresent
	}
	if f.existingURL != "" {
		atomic.AddInt32(&f.updates, 1)
		return &pr.PRRef{URL: f.existingURL, Updated: true}, nil
	}
	atomic.AddInt32(&f.opens, 1)
	return &pr.PRRef{URL: f.openURL, Updated: false}, nil
}

// seedGitLabSession materialises a completed v2 session in a repo whose origin
// remote is a gitlab.com URL. Used by every US1 integration case.
func seedGitLabSession(t *testing.T, id string) (repo string, st *session.State) {
	t.Helper()
	repo = helpers.NewSeededRepoWithRemote(t, "https://gitlab.com/acme/repo.git")
	out, err := worktree.Create(context.Background(), worktree.CreateInput{
		RepoRoot:  repo,
		SessionID: id,
		Runner:    &worktree.ExecRunner{},
	})
	if err != nil {
		t.Fatalf("worktree.Create: %v", err)
	}
	// Commit inside the worktree so rev-list finds a commit beyond base.
	marker := filepath.Join(out.WorktreePath, "change.md")
	if err := os.WriteFile(marker, []byte("glab session change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range [][]string{
		{"git", "-C", out.WorktreePath, "add", "."},
		{"git", "-C", out.WorktreePath, "commit", "-q", "-m", "session commit"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bender-test", "GIT_AUTHOR_EMAIL=bender@test",
			"GIT_COMMITTER_NAME=bender-test", "GIT_COMMITTER_EMAIL=bender@test",
		)
		if cb, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", cmd, err, cb)
		}
	}
	st, err = session.LoadState(out.SessionDir)
	if err != nil {
		t.Fatal(err)
	}
	st.Status = "completed"
	st.CompletedAt = time.Now().UTC()
	if err := session.SaveState(out.SessionDir, st); err != nil {
		t.Fatal(err)
	}
	return repo, st
}

// US1 end-to-end: detect a gitlab.com remote, select the glab adapter,
// push, open the MR, re-invoke to exercise the update path.
func TestGitLabPR_OpenThenUpdate(t *testing.T) {
	_, st := seedGitLabSession(t, "glab-happy")
	fa := &fakeGitLabAdapter{openURL: "https://gitlab.com/acme/repo/-/merge_requests/1"}

	adapter, err := pr.SelectAdapter([]pr.Adapter{fa}, "https://gitlab.com/acme/repo.git")
	if err != nil {
		t.Fatalf("SelectAdapter: %v", err)
	}
	if adapter.Name() != "glab" {
		t.Fatalf("expected glab adapter, got %q", adapter.Name())
	}
	ctx := context.Background()
	if err := adapter.AuthCheck(ctx); err != nil {
		t.Fatal(err)
	}
	shortBranch := strings.TrimPrefix(st.SessionBranch, "refs/heads/")
	if err := adapter.Push(ctx, pr.PushInput{
		Remote:       "origin",
		LocalBranch:  st.SessionBranch,
		RemoteBranch: shortBranch,
	}); err != nil {
		t.Fatalf("Push: %v", err)
	}
	ref, err := adapter.OpenOrUpdate(ctx, pr.OpenArgs{
		Base: st.BaseBranch, Head: shortBranch, Title: "from test", Body: "body",
	})
	if err != nil {
		t.Fatalf("OpenOrUpdate: %v", err)
	}
	if ref.Updated {
		t.Errorf("expected Updated=false on first open")
	}
	if fa.pushes != 1 || fa.opens != 1 {
		t.Fatalf("calls: pushes=%d opens=%d (want 1,1)", fa.pushes, fa.opens)
	}

	fa.existingURL = ref.URL
	ref2, err := adapter.OpenOrUpdate(ctx, pr.OpenArgs{
		Base: st.BaseBranch, Head: shortBranch, Title: "refreshed", Body: "newer",
	})
	if err != nil {
		t.Fatalf("second OpenOrUpdate: %v", err)
	}
	if !ref2.Updated {
		t.Error("expected Updated=true on re-invocation")
	}
}

// Codeberg remote (not github, not gitlab) must refuse with ErrNoAdapter;
// this locks the "do not silently expand adapter coverage" guarantee (US1
// acceptance scenario 5).
func TestGitLabPR_NonGitLabRemote_RefusesWithNoAdapter(t *testing.T) {
	adapters := []pr.Adapter{
		pr.NewGitHubAdapter(&pr.FakeExec{}),
		pr.NewGitLabAdapter(&pr.FakeExec{}),
	}
	_, err := pr.SelectAdapter(adapters, "https://codeberg.org/acme/repo.git")
	if err == nil {
		t.Fatal("expected ErrNoAdapter")
	}
	if !strings.Contains(err.Error(), "no adapter matches remote") {
		t.Fatalf("unexpected error text: %v", err)
	}
}
