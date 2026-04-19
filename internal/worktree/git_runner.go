package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// GitRunner abstracts invocations of the git binary. The real implementation
// shells to `git`; tests inject a fake that records invocations and returns
// canned output.
//
// Run executes `git <args...>` with the given working directory (empty string
// uses the current process CWD) and returns combined stdout, stderr, and the
// ExitError on non-zero exit. Callers MUST treat exit status separately from
// the Go error — git reports semantic failures (bad branch, not-a-repo, etc.)
// via non-zero exit with stderr text the caller wants to surface verbatim.
type GitRunner interface {
	Run(ctx context.Context, cwd string, args ...string) (stdout, stderr []byte, err error)
}

// ExecRunner is the production GitRunner. It uses os/exec.
type ExecRunner struct {
	// Path is the git binary to invoke. Empty string means resolve "git" on PATH.
	Path string
}

// Run implements GitRunner.
func (r *ExecRunner) Run(ctx context.Context, cwd string, args ...string) ([]byte, []byte, error) {
	bin := r.Path
	if bin == "" {
		bin = "git"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// ErrGitUnavailable is returned (wrapped) when the git binary cannot be found
// or executed. Callers map this to exit code 10.
var ErrGitUnavailable = errors.New("git binary unavailable")

// ProbeGit verifies that the git binary exists and is executable. Returns
// ErrGitUnavailable (wrapped) on any failure. Cheap — runs `git --version`.
func ProbeGit(ctx context.Context, r GitRunner) error {
	_, stderr, err := r.Run(ctx, "", "--version")
	if err != nil {
		return fmt.Errorf("%w: %v: %s", ErrGitUnavailable, err, bytes.TrimSpace(stderr))
	}
	return nil
}

// FakeCall records a single invocation made against a FakeRunner.
type FakeCall struct {
	CWD  string
	Args []string
}

// FakeRunner is a GitRunner for unit tests. It replays canned responses keyed
// by the first positional arg (subcommand). If a subcommand has no canned
// response, FakeRunner returns empty output and a nil error — tests that want
// stricter behaviour can inspect Calls after the fact.
type FakeRunner struct {
	Calls []FakeCall
	// Responses maps the first positional arg (subcommand) to a canned response.
	// Later entries for the same subcommand override earlier ones.
	Responses map[string]FakeResponse
}

// FakeResponse is a canned response for FakeRunner.Run.
type FakeResponse struct {
	Stdout []byte
	Stderr []byte
	Err    error
}

// Run implements GitRunner.
func (f *FakeRunner) Run(_ context.Context, cwd string, args ...string) ([]byte, []byte, error) {
	f.Calls = append(f.Calls, FakeCall{CWD: cwd, Args: append([]string(nil), args...)})
	if len(args) == 0 {
		return nil, nil, nil
	}
	if resp, ok := f.Responses[args[0]]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}
	return nil, nil, nil
}
