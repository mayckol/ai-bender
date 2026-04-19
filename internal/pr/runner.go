package pr

import (
	"bytes"
	"context"
	"os/exec"
)

// ExecRunner is the platform-CLI equivalent of worktree.GitRunner: it
// abstracts process invocation so the adapters can be unit-tested with a
// fake.
type ExecRunner interface {
	Run(ctx context.Context, cwd, bin string, args ...string) (stdout, stderr []byte, err error)
}

// SystemExecRunner is the production ExecRunner backed by os/exec.
type SystemExecRunner struct{}

// Run implements ExecRunner.
func (SystemExecRunner) Run(ctx context.Context, cwd, bin string, args ...string) ([]byte, []byte, error) {
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

// FakeExec records calls and replays canned responses. Responses maps the
// binary name (e.g. "gh", "git") to its default reply; Sequence (when set)
// consumes one entry per call in order and takes precedence over Responses.
type FakeExec struct {
	Calls     []FakeExecCall
	Responses map[string]FakeExecResponse
	Sequence  []FakeExecResponse
	seqIdx    int
}

// FakeExecCall is one invocation against FakeExec.
type FakeExecCall struct {
	CWD  string
	Bin  string
	Args []string
}

// FakeExecResponse is the canned response for a given binary.
type FakeExecResponse struct {
	Stdout []byte
	Stderr []byte
	Err    error
}

// Run implements ExecRunner.
func (f *FakeExec) Run(_ context.Context, cwd, bin string, args ...string) ([]byte, []byte, error) {
	f.Calls = append(f.Calls, FakeExecCall{CWD: cwd, Bin: bin, Args: append([]string(nil), args...)})
	if f.Sequence != nil {
		idx := f.seqIdx
		if idx >= len(f.Sequence) {
			idx = len(f.Sequence) - 1
		}
		f.seqIdx++
		resp := f.Sequence[idx]
		return resp.Stdout, resp.Stderr, resp.Err
	}
	if resp, ok := f.Responses[bin]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}
	return nil, nil, nil
}
