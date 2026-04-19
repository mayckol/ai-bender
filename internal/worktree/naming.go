package worktree

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// BranchPrefix is the reserved namespace for session branches. The prefix
// exists so humans (and `gh pr list` wildcards) can filter bender branches.
// Starting a new bender session on a ref already under this prefix is
// refused — nested sessions are not supported.
const BranchPrefix = "bender/session/"

// sessionIDSafe rejects anything that could produce an unsafe git ref.
// Git refs cannot contain: space, tilde, caret, colon, question mark,
// asterisk, open bracket, backslash, range separator, consecutive slashes,
// or a leading/trailing dash. We also require at least one character.
var sessionIDSafe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._\-]*$`)

// BranchName returns the deterministic session branch name for a session id.
// The caller must pre-validate the session id via ValidateSessionID.
func BranchName(sessionID string) string {
	return BranchPrefix + sessionID
}

// ValidateSessionID rejects session ids that would produce an unsafe ref.
func ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session id: empty")
	}
	if !sessionIDSafe.MatchString(sessionID) {
		return fmt.Errorf("session id: unsafe for git ref: %q", sessionID)
	}
	if strings.HasPrefix(sessionID, "bender/session/") {
		return fmt.Errorf("session id: must not start with %q", BranchPrefix)
	}
	return nil
}

// BranchExists reports whether a local branch matching the given ref name
// already exists in the repo at `cwd`. Returns (false, nil) when missing,
// (true, nil) when present, and non-nil err only on non-existence-related
// git failures (e.g. not a git repo).
func BranchExists(ctx context.Context, runner GitRunner, cwd, branch string) (bool, error) {
	ref := "refs/heads/" + strings.TrimPrefix(branch, "refs/heads/")
	_, _, err := runner.Run(ctx, cwd, "show-ref", "--verify", "--quiet", ref)
	if err == nil {
		return true, nil
	}
	// git show-ref --verify --quiet exits non-zero when the ref doesn't exist.
	// We treat that as "not a git failure" — the branch simply doesn't exist.
	return false, nil
}
