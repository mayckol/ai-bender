package pr

import (
	"context"
	"errors"
	"testing"
)

func TestIsGitHubURL(t *testing.T) {
	for _, u := range []string{
		"https://github.com/acme/repo",
		"https://github.com/acme/repo.git",
		"git@github.com:acme/repo.git",
		"ssh://git@github.com/acme/repo",
	} {
		if !IsGitHubURL(u) {
			t.Errorf("IsGitHubURL(%q) = false", u)
		}
	}
	for _, u := range []string{
		"https://gitlab.com/acme/repo",
		"https://bitbucket.org/acme/repo",
		"",
	} {
		if IsGitHubURL(u) {
			t.Errorf("IsGitHubURL(%q) = true", u)
		}
	}
}

func TestIsGitLabURL(t *testing.T) {
	for _, u := range []string{
		"https://gitlab.com/acme/repo",
		"git@gitlab.com:acme/repo.git",
	} {
		if !IsGitLabURL(u) {
			t.Errorf("IsGitLabURL(%q) = false", u)
		}
	}
	if IsGitLabURL("https://github.com/acme/repo") {
		t.Error("IsGitLabURL should return false for GitHub URL")
	}
}

type fakeAdapter struct {
	name      string
	detectURL string
}

func (f *fakeAdapter) Name() string                        { return f.name }
func (f *fakeAdapter) Detect(u string) bool                { return u == f.detectURL }
func (f *fakeAdapter) AuthCheck(context.Context) error     { return nil }
func (f *fakeAdapter) Push(context.Context, PushInput) error { return nil }
func (f *fakeAdapter) OpenOrUpdate(context.Context, OpenArgs) (*PRRef, error) {
	return &PRRef{URL: "https://" + f.name + "/pr/1"}, nil
}

func TestSelectAdapter_Matches(t *testing.T) {
	adapters := []Adapter{
		&fakeAdapter{name: "gh", detectURL: "https://github.com/acme/repo"},
		&fakeAdapter{name: "glab", detectURL: "https://gitlab.com/acme/repo"},
	}
	a, err := SelectAdapter(adapters, "https://gitlab.com/acme/repo")
	if err != nil {
		t.Fatalf("SelectAdapter: %v", err)
	}
	if a.Name() != "glab" {
		t.Fatalf("got %q want glab", a.Name())
	}
}

func TestSelectAdapter_NoMatch(t *testing.T) {
	adapters := []Adapter{
		&fakeAdapter{name: "gh", detectURL: "https://github.com/acme/repo"},
	}
	_, err := SelectAdapter(adapters, "https://codeberg.org/acme/repo")
	if !errors.Is(err, ErrNoAdapter) {
		t.Fatalf("want ErrNoAdapter, got %v", err)
	}
}
