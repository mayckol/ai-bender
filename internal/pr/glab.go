package pr

import (
	"context"
	"fmt"
)

// GitLabAdapter is a stub that proves the wiring for future GitLab support.
// Detect matches gitlab.com URLs so the dispatcher never silently falls
// through to "no adapter" for a recognisable remote; every operation
// returns ErrAdapterUnimplemented so users see a specific error instead of
// an obscure failure mode.
type GitLabAdapter struct{}

// NewGitLabAdapter returns the stub. The ExecRunner is reserved for when
// the real implementation lands.
func NewGitLabAdapter(_ ExecRunner) *GitLabAdapter { return &GitLabAdapter{} }

// Name implements Adapter.
func (*GitLabAdapter) Name() string { return "glab" }

// Detect implements Adapter.
func (*GitLabAdapter) Detect(remoteURL string) bool { return IsGitLabURL(remoteURL) }

// AuthCheck implements Adapter.
func (*GitLabAdapter) AuthCheck(context.Context) error {
	return fmt.Errorf("%w: glab adapter is not yet implemented in this release", ErrAdapterUnimplemented)
}

// Push implements Adapter.
func (*GitLabAdapter) Push(context.Context, PushInput) error {
	return fmt.Errorf("%w: glab adapter is not yet implemented in this release", ErrAdapterUnimplemented)
}

// OpenOrUpdate implements Adapter.
func (*GitLabAdapter) OpenOrUpdate(context.Context, OpenArgs) (*PRRef, error) {
	return nil, fmt.Errorf("%w: glab adapter is not yet implemented in this release", ErrAdapterUnimplemented)
}
