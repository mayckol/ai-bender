package pr

import (
	"context"
	"fmt"
	"strings"
)

// GitLabAdapter shells out to the `glab` CLI. Authentication is delegated
// to `glab auth login`; bender neither stores nor requests credentials.
//
// Minimum glab version: 1.30 (first release with `mr view --output json`).
// Older versions surface as a non-zero exit on the view / create step,
// which maps to exit code 32 via the caller — same treatment as a missing
// CLI.
type GitLabAdapter struct {
	exec ExecRunner
}

// NewGitLabAdapter wires a GitLabAdapter with the given ExecRunner.
func NewGitLabAdapter(runner ExecRunner) *GitLabAdapter { return &GitLabAdapter{exec: runner} }

// Name implements Adapter.
func (*GitLabAdapter) Name() string { return "glab" }

// Detect implements Adapter.
func (*GitLabAdapter) Detect(remoteURL string) bool { return IsGitLabURL(remoteURL) }

// AuthCheck implements Adapter by calling `glab auth status`.
func (a *GitLabAdapter) AuthCheck(ctx context.Context) error {
	_, stderr, err := a.exec.Run(ctx, "", "glab", "auth", "status")
	if err != nil {
		return fmt.Errorf("glab auth status: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// Push implements Adapter by calling `git push -u <remote> <branch>:<branch>`.
// Identical to the GitHub adapter — git does the push, glab is not involved.
func (a *GitLabAdapter) Push(ctx context.Context, in PushInput) error {
	refspec := strings.TrimPrefix(in.LocalBranch, "refs/heads/") + ":" + in.RemoteBranch
	_, stderr, err := a.exec.Run(ctx, in.RepoRoot, "git", "push", "-u", in.Remote, refspec)
	if err != nil {
		return fmt.Errorf("git push: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return nil
}

// OpenOrUpdate implements Adapter via `glab mr view` / `glab mr create` /
// `glab mr update`. Flow:
//  1. `glab mr view <head> --output json` — if it succeeds and the JSON
//     contains a web_url:
//     - when RefuseUpdate is true, return ErrExistingPRPresent with the URL;
//     - otherwise refresh title/body via `glab mr update`.
//  2. Otherwise run `glab mr create` with source-branch / target-branch /
//     title / description (and --draft when requested).
func (a *GitLabAdapter) OpenOrUpdate(ctx context.Context, in OpenArgs) (*PRRef, error) {
	viewOut, _, viewErr := a.exec.Run(ctx, in.RepoRoot, "glab", "mr", "view", in.Head, "--output", "json")
	mrURL := extractGitLabWebURL(viewOut)
	mrExists := viewErr == nil && mrURL != ""

	if mrExists && in.RefuseUpdate {
		return &PRRef{URL: mrURL, Updated: false},
			fmt.Errorf("%w: %s", ErrExistingPRPresent, mrURL)
	}
	if mrExists {
		_, stderr, err := a.exec.Run(ctx, in.RepoRoot, "glab", "mr", "update", in.Head,
			"--title", in.Title, "--description", in.Body)
		if err != nil {
			return nil, fmt.Errorf("glab mr update: %w: %s", err, strings.TrimSpace(string(stderr)))
		}
		return &PRRef{URL: mrURL, Updated: true}, nil
	}

	args := []string{"mr", "create",
		"--source-branch", in.Head,
		"--target-branch", in.Base,
		"--title", in.Title,
		"--description", in.Body,
	}
	if in.Draft {
		args = append(args, "--draft")
	}
	stdout, stderr, err := a.exec.Run(ctx, in.RepoRoot, "glab", args...)
	if err != nil {
		return nil, fmt.Errorf("glab mr create: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	// glab mr create prints the MR URL on its last non-empty line; grab it.
	url := lastNonEmptyLine(string(stdout))
	if url == "" {
		return nil, fmt.Errorf("glab mr create: no URL on stdout")
	}
	return &PRRef{URL: url, Updated: false}, nil
}

// extractGitLabWebURL pulls the `web_url` field out of a `glab mr view --output
// json` payload without pulling in a JSON parser. The field is a plain string;
// the JSON object is tiny; a regex-free substring scan is sufficient and
// matches the parsing style of the GitHub adapter's extractURL.
func extractGitLabWebURL(b []byte) string {
	s := string(b)
	const key = `"web_url":"`
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

func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}
