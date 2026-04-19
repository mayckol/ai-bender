package pr

import (
	"context"
	"errors"
	"testing"
)

func TestGitLabAdapter_Detect(t *testing.T) {
	a := NewGitLabAdapter(&FakeExec{})
	for _, u := range []string{
		"https://gitlab.com/acme/repo",
		"https://gitlab.com/acme/repo.git",
		"git@gitlab.com:acme/repo.git",
		"ssh://git@gitlab.com/acme/repo",
	} {
		if !a.Detect(u) {
			t.Errorf("Detect(%q) = false", u)
		}
	}
	for _, u := range []string{
		"https://github.com/acme/repo",
		"https://codeberg.org/acme/repo",
	} {
		if a.Detect(u) {
			t.Errorf("Detect(%q) = true; must refuse non-gitlab hostnames", u)
		}
	}
}

func TestGitLabAdapter_AuthCheck_ForwardsStderr(t *testing.T) {
	e := &FakeExec{
		Responses: map[string]FakeExecResponse{
			"glab": {Stderr: []byte("not authenticated"), Err: errors.New("exit 1")},
		},
	}
	err := NewGitLabAdapter(e).AuthCheck(context.Background())
	if err == nil || !containsAny(err.Error(), "not authenticated", "glab auth status") {
		t.Fatalf("expected forwarded stderr, got %v", err)
	}
}

func TestGitLabAdapter_Push_ProducesExpectedGitInvocation(t *testing.T) {
	e := &FakeExec{}
	err := NewGitLabAdapter(e).Push(context.Background(), PushInput{
		RepoRoot:     "/repo",
		Remote:       "origin",
		LocalBranch:  "refs/heads/bender/session/xyz",
		RemoteBranch: "bender/session/xyz",
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(e.Calls) != 1 {
		t.Fatalf("calls: got %d want 1", len(e.Calls))
	}
	c := e.Calls[0]
	if c.Bin != "git" {
		t.Fatalf("bin: got %q want git", c.Bin)
	}
	wantArgs := []string{"push", "-u", "origin", "bender/session/xyz:bender/session/xyz"}
	if !equalStrs(c.Args, wantArgs) {
		t.Fatalf("args: got %v want %v", c.Args, wantArgs)
	}
}

func TestGitLabAdapter_OpenOrUpdate_CreatesWhenAbsent(t *testing.T) {
	// Call 1 (glab mr view) returns err → MR absent.
	// Call 2 (glab mr create) succeeds with the new MR URL on stdout.
	e := &FakeExec{
		Sequence: []FakeExecResponse{
			{Err: errors.New("exit 1")},
			{Stdout: []byte("https://gitlab.com/acme/repo/-/merge_requests/7\n")},
		},
	}
	a := NewGitLabAdapter(e)
	ref, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot: "/repo",
		Base:     "main",
		Head:     "bender/session/xyz",
		Title:    "new feature",
		Body:     "pre-populated body",
	})
	if err != nil {
		t.Fatalf("OpenOrUpdate: %v", err)
	}
	if ref.Updated {
		t.Error("expected Updated=false for create path")
	}
	if ref.URL != "https://gitlab.com/acme/repo/-/merge_requests/7" {
		t.Fatalf("URL: got %q", ref.URL)
	}
	if len(e.Calls) != 2 {
		t.Fatalf("calls: got %d want 2", len(e.Calls))
	}
	if e.Calls[0].Args[0] != "mr" || e.Calls[0].Args[1] != "view" {
		t.Fatalf("first call wasn't mr view: %v", e.Calls[0].Args)
	}
	if e.Calls[1].Args[0] != "mr" || e.Calls[1].Args[1] != "create" {
		t.Fatalf("second call wasn't mr create: %v", e.Calls[1].Args)
	}
	// --source-branch / --target-branch / --title / --description must all be present.
	createArgs := e.Calls[1].Args
	for _, needle := range []string{"--source-branch", "bender/session/xyz", "--target-branch", "main", "--title", "new feature"} {
		found := false
		for _, a := range createArgs {
			if a == needle {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mr create missing arg %q; got %v", needle, createArgs)
		}
	}
}

func TestGitLabAdapter_OpenOrUpdate_UpdatesWhenPresent(t *testing.T) {
	// Call 1: view returns JSON with web_url. Call 2: mr update succeeds.
	e := &FakeExec{
		Sequence: []FakeExecResponse{
			{Stdout: []byte(`{"web_url":"https://gitlab.com/acme/repo/-/merge_requests/7"}`)},
			{Stdout: []byte("updated\n")},
		},
	}
	a := NewGitLabAdapter(e)
	ref, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot: "/repo",
		Base:     "main",
		Head:     "bender/session/xyz",
		Title:    "refreshed",
		Body:     "newer body",
	})
	if err != nil {
		t.Fatalf("OpenOrUpdate: %v", err)
	}
	if !ref.Updated {
		t.Error("expected Updated=true for update path")
	}
	if ref.URL != "https://gitlab.com/acme/repo/-/merge_requests/7" {
		t.Fatalf("URL: got %q", ref.URL)
	}
	if e.Calls[1].Args[0] != "mr" || e.Calls[1].Args[1] != "update" {
		t.Fatalf("second call wasn't mr update: %v", e.Calls[1].Args)
	}
}

func TestGitLabAdapter_OpenOrUpdate_RefusesUpdateWhenFlagSet(t *testing.T) {
	e := &FakeExec{
		Responses: map[string]FakeExecResponse{
			"glab": {Stdout: []byte(`{"web_url":"https://gitlab.com/acme/repo/-/merge_requests/7"}`)},
		},
	}
	a := NewGitLabAdapter(e)
	_, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot:     "/repo",
		Base:         "main",
		Head:         "bender/session/xyz",
		Title:        "x",
		Body:         "y",
		RefuseUpdate: true,
	})
	if !errors.Is(err, ErrExistingPRPresent) {
		t.Fatalf("want ErrExistingPRPresent, got %v", err)
	}
}

func TestGitLabAdapter_OpenOrUpdate_CreatePassesDraftWhenRequested(t *testing.T) {
	e := &FakeExec{
		Sequence: []FakeExecResponse{
			{Err: errors.New("exit 1")},
			{Stdout: []byte("https://gitlab.com/acme/repo/-/merge_requests/8\n")},
		},
	}
	a := NewGitLabAdapter(e)
	if _, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot: "/repo",
		Base:     "main",
		Head:     "bender/session/xyz",
		Title:    "t",
		Body:     "b",
		Draft:    true,
	}); err != nil {
		t.Fatalf("OpenOrUpdate: %v", err)
	}
	sawDraft := false
	for _, a := range e.Calls[1].Args {
		if a == "--draft" {
			sawDraft = true
		}
	}
	if !sawDraft {
		t.Fatalf("expected --draft in mr create args: %v", e.Calls[1].Args)
	}
}
