package pr

import (
	"context"
	"fmt"
	"strings"
)

// GitHubAdapter shells out to the `gh` CLI. It never stores or requests
// credentials directly — authentication is the user's responsibility via
// `gh auth login`.
type GitHubAdapter struct {
	exec ExecRunner
}

// NewGitHubAdapter wires a GitHubAdapter with the given ExecRunner.
func NewGitHubAdapter(runner ExecRunner) *GitHubAdapter { return &GitHubAdapter{exec: runner} }

// Name implements Adapter.
func (*GitHubAdapter) Name() string { return "gh" }

// Detect implements Adapter.
func (*GitHubAdapter) Detect(remoteURL string) bool { return IsGitHubURL(remoteURL) }

// AuthCheck implements Adapter by calling `gh auth status`.
func (a *GitHubAdapter) AuthCheck(ctx context.Context) error {
	_, stderr, err := a.exec.Run(ctx, "", "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("gh auth status: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// Push implements Adapter by calling `git push <remote> <local>:<remote-branch>`.
// Uses -u to set upstream on first push; idempotent on re-runs.
func (a *GitHubAdapter) Push(ctx context.Context, in PushInput) error {
	refspec := strings.TrimPrefix(in.LocalBranch, "refs/heads/") + ":" + in.RemoteBranch
	_, stderr, err := a.exec.Run(ctx, in.RepoRoot, "git", "push", "-u", in.Remote, refspec)
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// OpenOrUpdate implements Adapter using `gh pr create` / `gh pr edit`.
//
// Flow:
//  1. `gh pr view <head>` — if the PR exists:
//     * when RefuseUpdate is true, return ErrExistingPRPresent with the URL;
//     * otherwise refresh title/body via `gh pr edit`.
//  2. Otherwise run `gh pr create`.
func (a *GitHubAdapter) OpenOrUpdate(ctx context.Context, in OpenArgs) (*PRRef, error) {
	viewOut, _, err := a.exec.Run(ctx, in.RepoRoot, "gh", "pr", "view", in.Head,
		"--json", "url")
	prExists := err == nil && len(strings.TrimSpace(string(viewOut))) > 0
	if prExists && in.RefuseUpdate {
		return &PRRef{URL: extractURL(viewOut), Updated: false},
			fmt.Errorf("%w: %s", ErrExistingPRPresent, extractURL(viewOut))
	}
	if prExists {
		_, stderr, err := a.exec.Run(ctx, in.RepoRoot, "gh", "pr", "edit", in.Head,
			"--title", in.Title, "--body", in.Body)
		if err != nil {
			return nil, fmt.Errorf("gh pr edit: %w: %s", err, strings.TrimSpace(string(stderr)))
		}
		return &PRRef{URL: extractURL(viewOut), Updated: true}, nil
	}
	args := []string{"pr", "create",
		"--base", in.Base,
		"--head", in.Head,
		"--title", in.Title,
		"--body", in.Body,
	}
	if in.Draft {
		args = append(args, "--draft")
	}
	stdout, stderr, err := a.exec.Run(ctx, in.RepoRoot, "gh", args...)
	if err != nil {
		return nil, fmt.Errorf("gh pr create: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return &PRRef{URL: strings.TrimSpace(string(stdout)), Updated: false}, nil
}

// extractURL pulls the pr_url out of `gh pr view --json url` output, which
// is a tiny JSON object: {"url":"https://github.com/.../pull/N"}. We avoid
// pulling in a full JSON parser for a one-liner.
func extractURL(b []byte) string {
	s := string(b)
	const key = `"url":"`
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	s = s[i+len(key):]
	j := strings.Index(s, `"`)
	if j < 0 {
		return ""
	}
	return s[:j]
}
