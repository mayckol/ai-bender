// Package workflow groups consecutive sessions belonging to the same feature
// (for example /tdd followed by /ghu) into one logical workflow. Each session
// persists its workflow id (and parent session id) in state.json; the UI reads
// those fields to stitch multiple session timelines into a single live view.
//
// The package is intentionally thin: no daemon, no on-disk workflow index.
// Identity is derived on demand by scanning .bender/sessions/*/state.json.
package workflow

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mayckol/ai-bender/internal/session"
)

// Linkage is the result of Resolve: the workflow id the caller should persist
// on the new session, and (when non-empty) the id of the prior session whose
// workflow this one joins.
type Linkage struct {
	WorkflowID     string
	ParentSession  string
	InheritedFrom  string // human-readable reason, e.g. "tdd session session-x on key /tmp/repo#main"
	WorkflowIsNew  bool
}

// ResolveParams configures Resolve. ProjectRoot resolves to
// <ProjectRoot>/.bender/sessions. Key is the grouping token (normally the
// current git branch or the current feature-directory path); two sessions
// sharing the same Key inherit the same workflow id. MaxAge is the lookback
// window; sessions older than MaxAge are ignored (a long-abandoned workflow
// must not silently grab a new session).
type ResolveParams struct {
	ProjectRoot string
	Key         string
	MaxAge      time.Duration
	Now         func() time.Time
}

// Resolve returns the linkage to apply to a new session.
//
// Rules:
//  1. If any prior session in the sessions directory carries a WorkflowKey
//     annotation equal to p.Key (stored as a state.json payload in future
//     iterations — today we fall back to "most recent session with the same
//     session-branch") and its StartedAt is within MaxAge, inherit its
//     WorkflowID and set ParentSession to its SessionID.
//  2. Otherwise, mint a new WorkflowID derived from p.Key + a random suffix.
func Resolve(p ResolveParams) (Linkage, error) {
	if p.ProjectRoot == "" {
		return Linkage{}, fmt.Errorf("workflow.Resolve: project_root required")
	}
	if p.Key == "" {
		return Linkage{}, fmt.Errorf("workflow.Resolve: key required")
	}
	now := time.Now().UTC
	if p.Now != nil {
		now = p.Now
	}
	maxAge := p.MaxAge
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}

	listings, err := session.List(p.ProjectRoot)
	if err != nil {
		return Linkage{}, fmt.Errorf("workflow.Resolve: list sessions: %w", err)
	}
	// newest first
	sort.Slice(listings, func(i, j int) bool {
		return listings[i].State.StartedAt.After(listings[j].State.StartedAt)
	})

	cutoff := now().Add(-maxAge)
	for _, l := range listings {
		if l.State.StartedAt.Before(cutoff) {
			continue
		}
		if l.State.WorkflowID == "" {
			continue
		}
		if matchesKey(l.State, p.Key) {
			return Linkage{
				WorkflowID:    l.State.WorkflowID,
				ParentSession: l.State.SessionID,
				InheritedFrom: fmt.Sprintf("session %s (key %s)", l.State.SessionID, p.Key),
			}, nil
		}
	}

	id, err := mintWorkflowID()
	if err != nil {
		return Linkage{}, err
	}
	return Linkage{
		WorkflowID:    id,
		WorkflowIsNew: true,
	}, nil
}

// matchesKey reports whether a prior session's state considers p.Key a match.
// Today the heuristic is: same SessionBranch (worktree branch) or, if that's
// empty, same base branch + base SHA prefix. Future iterations may add an
// explicit "workflow_key" payload to state.json.
func matchesKey(s session.State, key string) bool {
	if key == "" {
		return false
	}
	if normalizeBranch(s.SessionBranch) == normalizeBranch(key) {
		return true
	}
	if normalizeBranch(s.BaseBranch) == normalizeBranch(key) {
		return true
	}
	return false
}

func normalizeBranch(b string) string {
	return strings.TrimPrefix(b, "refs/heads/")
}

func mintWorkflowID() (string, error) {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("workflow: mint id: %w", err)
	}
	return "wf-" + time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(buf[:]), nil
}
