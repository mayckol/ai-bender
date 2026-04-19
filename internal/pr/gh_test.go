package pr

import (
	"context"
	"errors"
	"testing"
)

func TestGitHubAdapter_Detect(t *testing.T) {
	a := NewGitHubAdapter(&FakeExec{})
	if !a.Detect("https://github.com/acme/repo") {
		t.Error("github.com should match")
	}
	if a.Detect("https://gitlab.com/acme/repo") {
		t.Error("gitlab.com should not match")
	}
}

func TestGitHubAdapter_AuthCheck_ForwardsStderr(t *testing.T) {
	e := &FakeExec{
		Responses: map[string]FakeExecResponse{
			"gh": {Stderr: []byte("you are not logged in"), Err: errors.New("exit 1")},
		},
	}
	err := NewGitHubAdapter(e).AuthCheck(context.Background())
	if err == nil || !containsAny(err.Error(), "not logged in", "gh auth status") {
		t.Fatalf("expected forwarded stderr, got %v", err)
	}
}

func TestGitHubAdapter_Push_ProducesExpectedGitInvocation(t *testing.T) {
	e := &FakeExec{}
	err := NewGitHubAdapter(e).Push(context.Background(), PushInput{
		RepoRoot:     "/repo",
		Remote:       "origin",
		LocalBranch:  "refs/heads/bender/session/abc",
		RemoteBranch: "bender/session/abc",
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
	wantArgs := []string{"push", "-u", "origin", "bender/session/abc:bender/session/abc"}
	if !equalStrs(c.Args, wantArgs) {
		t.Fatalf("args: got %v want %v", c.Args, wantArgs)
	}
}

func TestGitHubAdapter_OpenOrUpdate_CreatesWhenAbsent(t *testing.T) {
	// Call 1 (gh pr view) returns err => PR absent.
	// Call 2 (gh pr create) succeeds with the new PR URL on stdout.
	e := &FakeExec{
		Sequence: []FakeExecResponse{
			{Err: errors.New("exit 1")},
			{Stdout: []byte("https://github.com/acme/repo/pull/42\n")},
		},
	}
	a := NewGitHubAdapter(e)
	ref, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot: "/repo",
		Base:     "main",
		Head:     "bender/session/abc",
		Title:    "new feature",
		Body:     "pre-populated body",
	})
	if err != nil {
		t.Fatalf("OpenOrUpdate: %v", err)
	}
	if ref.Updated {
		t.Error("expected Updated=false for create path")
	}
	// Should have issued two gh invocations: view, then create.
	if len(e.Calls) != 2 {
		t.Fatalf("calls: got %d want 2", len(e.Calls))
	}
	if e.Calls[0].Args[1] != "view" {
		t.Fatalf("first call wasn't view: %v", e.Calls[0].Args)
	}
	if e.Calls[1].Args[1] != "create" {
		t.Fatalf("second call wasn't create: %v", e.Calls[1].Args)
	}
}

func TestGitHubAdapter_OpenOrUpdate_RefusesUpdateWhenFlagSet(t *testing.T) {
	e := &FakeExec{
		Responses: map[string]FakeExecResponse{
			"gh": {Stdout: []byte(`{"url":"https://github.com/acme/repo/pull/42"}`)},
		},
	}
	a := NewGitHubAdapter(e)
	_, err := a.OpenOrUpdate(context.Background(), OpenArgs{
		RepoRoot:     "/repo",
		Base:         "main",
		Head:         "bender/session/abc",
		Title:        "x",
		Body:         "y",
		RefuseUpdate: true,
	})
	if !errors.Is(err, ErrExistingPRPresent) {
		t.Fatalf("want ErrExistingPRPresent, got %v", err)
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && len(s) >= len(n) {
			for i := 0; i+len(n) <= len(s); i++ {
				if s[i:i+len(n)] == n {
					return true
				}
			}
		}
	}
	return false
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
