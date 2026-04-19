// Package pr opens pull requests from a bender session's worktree branch via
// the user's locally installed platform CLI.
//
// The package is strictly opt-in: nothing in this package is invoked during a
// pipeline session; only `bender session pr <session-id>` reaches it. The
// package defines an [Adapter] interface so the command remains platform-
// agnostic and future platform CLIs (GitLab's `glab`, Codeberg, etc.) attach
// without changes to `cmd/bender`.
//
// v1 ships a working GitHub adapter (shells to `gh`) plus a GitLab stub that
// proves the wiring but returns [ErrAdapterUnimplemented] for operations.
package pr
