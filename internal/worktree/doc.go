// Package worktree manages the per-session git worktrees that isolate every
// bender pipeline run from the user's main working tree.
//
// Each pipeline session owns:
//   - one git worktree directory under the configured root
//     (default `<repo>/.bender/worktrees/<session-id>/`),
//   - one session branch `bender/session/<session-id>` derived from the base
//     branch at session start.
//
// The package wraps `git worktree add`, `git worktree remove`, and
// `git worktree prune` through the [GitRunner] interface so the package can be
// unit-tested with a fake runner and so the bundled Bash/PowerShell fallback
// scripts under `.specify/extensions/worktree/scripts/` stay in lock-step with
// the binary (Constitution VII).
//
// The Go binary is not a runtime executor of pipeline stages; Claude Code (or
// any client reading `.claude/`) drives stage dispatch. This package is the
// scaffolding/cleanup side of that contract — it creates, lists, removes, and
// reconciles worktrees, and records their metadata into the session state file.
package worktree
