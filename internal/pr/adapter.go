package pr

import (
	"context"
	"errors"
)

// Adapter is the seam between bender and a platform CLI (gh, glab, …).
// Implementations shell out to the CLI the user already has installed —
// bender never ships its own credentials or HTTP client.
type Adapter interface {
	// Name returns a short identifier used in state.json#pull_request.adapter.
	Name() string
	// Detect returns true iff this adapter recognises the given remote URL.
	Detect(remoteURL string) bool
	// AuthCheck probes the CLI to confirm the user is authenticated. Returning
	// an error short-circuits the rest of the flow with a specific "auth failed"
	// message; the caller surfaces it verbatim.
	AuthCheck(ctx context.Context) error
	// Push pushes the session branch to the remote. Must be idempotent.
	Push(ctx context.Context, in PushInput) error
	// OpenOrUpdate opens a PR — or updates it if one already exists for the
	// head/base pair. The Updated field in the returned PRRef is true iff a
	// pre-existing PR was updated rather than freshly created.
	OpenOrUpdate(ctx context.Context, in OpenArgs) (*PRRef, error)
}

// PushInput carries the data the adapter needs to `git push` (or equivalent).
type PushInput struct {
	RepoRoot       string
	Remote         string // e.g. "origin"
	LocalBranch    string // full ref, e.g. "refs/heads/bender/session/<id>"
	RemoteBranch   string // short name on the remote
}

// OpenArgs carries the data the adapter needs to open or update a PR.
type OpenArgs struct {
	RepoRoot     string
	Remote       string
	Base         string // branch name on the remote, e.g. "main"
	Head         string // branch name on the remote, e.g. "bender/session/<id>"
	Title        string
	Body         string
	Draft        bool
	RefuseUpdate bool
}

// PRRef is the snapshot captured after a successful OpenOrUpdate call.
type PRRef struct {
	URL     string
	Updated bool
}

// Shared error sentinels, exposed so cmd/bender can map them to exit codes
// (see contracts/cli.md):
var (
	ErrAdapterUnimplemented = errors.New("adapter unimplemented")
	ErrNoAdapter            = errors.New("no adapter matches remote")
	ErrPRNotEligible        = errors.New("session not eligible for PR")
	ErrExistingPRPresent    = errors.New("existing PR present; update refused")
)
