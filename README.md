<p align="center">
  <img src="docs/bender-logo.png" alt="ai-bender" width="240" />
</p>

# ai-bender

Spec-driven scaffold for Claude Code. `bender` is a Go CLI that installs a `.claude/` workspace into your projects and gives you slash commands you run inside Claude Code: `/cry`, `/plan`, `/tdd`, `/ghu`, `/implement`.

The binary never calls an LLM. It scaffolds, validates, and inspects. Claude Code is the executor.

## Install

**Homebrew** (macOS / Linux):

```bash
brew install mayckol/tap/bender
```

**curl | sh** (macOS / Linux, no Go required):

```bash
curl -fsSL https://raw.githubusercontent.com/mayckol/ai-bender/main/scripts/install.sh | sh
```

Env overrides: `BENDER_VERSION=v0.22.0`, `BENDER_PREFIX=$HOME/.local` (avoids `sudo`).

**go install**:

```bash
go install github.com/mayckol/ai-bender/cmd/bender@latest
```

**From source**:

```bash
git clone https://github.com/mayckol/ai-bender
cd ai-bender
make build install
```

Ensure the install dir (`/usr/local/bin`, `$(go env GOPATH)/bin`, or `$BENDER_PREFIX/bin`) is on your `PATH`. Verify with `bender version`.

## Quickstart

```bash
cd ~/projects/my-service
bender init                 # scaffolds .claude/ and .bender/
bender server               # live viewer on http://localhost:4317
# open the project in Claude Code:
#   /cry "users need roles"   → /cry confirm
#   /plan                     → /plan confirm
#   /tdd (optional)           → /tdd confirm
#   /ghu
bender sessions list        # inspect on-disk sessions
```

## Pipeline stages

Each stage reads its inputs from `.bender/artifacts/`, writes its outputs there, and emits events to `.bender/sessions/<id>/events.jsonl`.

| Stage | Command | Inputs | Outputs |
|---|---|---|---|
| bootstrap | `/bender-bootstrap` | discovered project files | `.bender/artifacts/constitution.md` |
| cry | `/cry <request>` | user request | `.bender/artifacts/cry/<slug>-<ts>.md` |
| plan | `/plan` | approved cry | spec + data model + risks + tasks under `.bender/artifacts/{specs,plan}/` |
| tdd *(optional)* | `/tdd` | approved plan | prose test scaffolds under `.bender/artifacts/plan/tests/` |
| ghu | `/ghu` | approved spec + tasks | code + `.bender/artifacts/ghu/run-<ts>-report.md` |
| implement | `/implement <task>` | same as `/ghu`, scoped | same report path |
| doctor | `/bender-doctor` | `.claude/` catalog | stdout health report |

Every draft artifact carries `status: draft` until you run `/<stage> confirm`. `/plan` is design-only — it refuses implementation requests while the plan is in `awaiting_confirm`. `/ghu` refuses to start if the plan set is still draft.

## Worktree isolation

Every `/ghu` and `/implement` session runs inside its own git worktree on `bender/session/<id>`. Your main working tree is never touched. Two sessions can run in parallel against the same repo without colliding.

Layout:

```
<repo>/.bender/worktrees/<session-id>/   # session worktree (gitignored)
<repo>/.bender/sessions/<session-id>/    # state.json + events.jsonl
```

Override the worktree root in `.bender/config.yaml`:

```yaml
worktree:
  root: ../bender-worktrees
```

`bender worktree {create,list,remove,prune}` manages them. When git is missing, the repo is bare, or the repo is mid-rebase/merge, bender refuses with a specific exit code — it never falls back to writing in the main tree.

## Workflow linkage

Consecutive sessions on the same feature branch (e.g. `/tdd` → `/ghu`) share a `workflow_id`, so the live viewer stitches them into one timeline. `bender workflow resolve --key <branch>` returns the id; `bender worktree create --workflow-id ... --workflow-parent ...` persists it.

## Binary commands

Global flags: `--project`, `--config`, `--no-color`, `--quiet`, `--verbose`.

| Command | Purpose |
|---|---|
| `bender init` | Scaffold `.claude/` + `.bender/` from embedded defaults. Idempotent. |
| `bender sync-defaults [--force]` | Materialise embedded defaults added since `init`. |
| `bender register-project <path> [--name]` | Add a project to `~/.bender/workspace.yaml`. |
| `bender list-projects` | List registered projects; marks `current`, `available`, `missing`. |
| `bender doctor` | Validate the catalog. |
| `bender sessions list` | List on-disk sessions. |
| `bender sessions show <id>` | NDJSON dump of `events.jsonl`. Pipe to `jq`. |
| `bender sessions export <id>` | Single JSON doc (state + events) matching the viewer ingest contract. |
| `bender sessions validate <id>` | Check `state.json` + `events.jsonl` against the v1 schema. |
| `bender sessions clear [<id>] [--all]` | Remove session data. Artifacts under `.bender/artifacts/` are preserved. |
| `bender sessions pr <id>` | Opt-in: push the session branch and open a PR via `gh` / `glab`. |
| `bender worktree create <id>` | Create a session worktree. Skills call this automatically. |
| `bender worktree list` | Show every session's worktree status. |
| `bender worktree remove <id>` | Tear down one worktree; keep the session branch. |
| `bender worktree prune` | Bulk-clean completed/aborted worktrees. |
| `bender workflow resolve --key <branch>` | Return the workflow id a new session should inherit. |
| `bender event emit --session --type --actor-kind --actor-name --payload` | Atomic append of one event line to `events.jsonl`. Skills call this; humans rarely. |
| `bender server [--port] [--foreground]` | Start the live viewer. |
| `bender update [--check]` | Replace the binary with the latest release. |
| `bender version` | Print binary version. |

Every binary verb has Bash + PowerShell fallbacks under `.specify/scripts/{bash,powershell}/` and extension scripts under `.specify/extensions/*/scripts/`. Install the binary globally for the full surface; the fallbacks cover machines without Go installed.

## Slash commands (run in Claude Code)

Materialised by `bender init` under `.claude/skills/<name>/SKILL.md`.

- `/cry [--type=<bug|feature|performance|architectural>] <request>` — capture intent. `/cry confirm` approves.
- `/plan [--from=<cry-artifact>]` — produce spec + data model + risks + tasks under one timestamp. `/plan confirm` flips the set to approved. **Design-only: never writes code.** Follow-ups during `awaiting_confirm` re-plan the artifacts; implementation-shaped messages are refused.
- `/tdd` — mirror the source tree under `.bender/artifacts/plan/tests/` with prose scaffolds. `/tdd confirm` approves.
- `/ghu [--bg | --inline] [--only=<task>] [--skip=<name>[,...]] [--abort-on-failure]` — execute the approved plan. First action provisions a worktree. Only stage that writes code. When `/tdd` scaffolds exist, runs in TDD mode (Red → Green → Refactor). Otherwise walks scout → architect → crafter ∥ tester → linter → reviewer ∥ sentinel ∥ benchmarker ∥ scribe → report.
- `/implement <task>` — `/ghu` scoped to one task.
- `/bender-doctor` — narrative wrapper around `bender doctor`.
- `/bender-bootstrap` — fill pending constitution sections from README + manifests.

`--skip` accepts `linter`/`lint`, `tester`/`test`, `reviewer`/`review`, `sentinel`/`security`, `benchmarker`/`perf`, `scribe`/`docs`, `surgeon`/`refactor`, `architect`, `review-sweep`. `crafter` and `scout` are not skippable.

`/ghu` (default `--bg`) runs in an isolated subagent via Claude Code's `Agent` tool. `--inline` runs inline for debugging.

## Agents

| Agent | Purpose | Write scope |
|---|---|---|
| `crafter` | Implement production code with minimum-diff edits | source files |
| `tester` | Author + run tests; enumerate edge cases | tests / fixtures only |
| `reviewer` | Critique against spec / constitution | read-only; findings only |
| `linter` | Run linters/formatters; safe autofixes | autofix-only |
| `architect` | Design + boundary validation | writes during `/plan`; read-only during `/ghu` |
| `scribe` | Keep docs in sync with code | docs/comments only |
| `scout` | Read-only codebase exploration with caching | zero |
| `sentinel` | Security analysis (secrets, deps, inputs, auth, crypto) | read-only; findings only |
| `benchmarker` | Performance analysis with measurement | read-only; findings only |
| `surgeon` | Behavior-preserving refactors (tests green before and after) | source files |

## Observability

Every stage emits structured events via `bender event emit` (atomic `O_APPEND | O_SYNC` writes). Two progress levels during `/ghu` and `/implement`:

- **Session-level** — `orchestrator_progress` events carry `{percent, current_step, completed_nodes, total_nodes}`. `percent = round(100 × completed / total)` over the DAG walk.
- **Agent-level** — `agent_progress` events carry `{agent, percent, current_step}`, one tick per internal sub-step so the bar advances smoothly.

## Real-time viewer

```bash
bender server                 # detaches, pid + log under .bender/
bender server --port 4000
bender server --foreground
bender server status
bender server stop
```

Open `http://localhost:4317/`:

- `/` — session list with live updates.
- `/sessions/<id>` — live timeline: per-agent rows, findings panel, file-changed summary, report link. Freezes on `session_completed`.

When `/ghu --bg` dispatches, the skill prints the viewer URL and best-effort-opens your browser. Sessions that share a `workflow_id` merge onto one timeline. Every event's raw JSON has a copy-to-clipboard button.

## Init preferences

`bender init` asks one confirm: *Open a pull request on successful `/ghu` runs?* The answer persists to `.bender/selection.yaml` under `preferences.open_pr_on_success`. When enabled, the `after_implement` `pr` extension runs `bender session pr <id>` after a successful run. Adapter failures never fail the implement run.

## Project layout after `bender init`

```
your-project/
├── .claude/                        # Claude Code–native artefacts
│   ├── agents/                     #   default subagents
│   └── skills/                     #   slash-command + worker skills
└── .bender/                        # bender-owned config + runtime
    ├── pipeline.yaml               #   declarative execution DAG
    ├── groups.yaml                 #   named skill-selector bundles
    ├── config.yaml                 #   per-project overrides
    ├── selection.yaml              #   init-time selection + preferences
    ├── artifacts/                  #   pipeline outputs (commit this)
    │   ├── constitution.md
    │   ├── cry/<slug>-<ts>.md
    │   ├── specs/<slug>-<ts>.md
    │   ├── plan/{data-model,api-contract,risk-assessment,tasks}-<ts>.md
    │   └── ghu/run-<ts>-report.md + {reviews,security,perf}/
    ├── worktrees/<session-id>/     #   per-session git worktree (gitignored)
    ├── sessions/<id>/              #   state.json + events.jsonl (gitignored)
    └── cache/                      #   scout caches (gitignored)
```

## Customising

Claude Code reads `.claude/agents/*.md` and `.claude/skills/*/SKILL.md` directly — no registration step.

**Change an agent's skill binding** — edit `.claude/agents/<name>.md`. `bender doctor` validates the result.

**Add an agent** — drop `.claude/agents/<your-name>.md` with standard frontmatter.

**Add a skill** — drop `.claude/skills/<name>/SKILL.md`; every matching agent selector picks it up.

**Per-project overrides** — `.bender/config.yaml`:

```yaml
agents:
  crafter:
    skills:
      add: [project-specific-migration-check]
      remove: [check-data-model]
    write_scope:
      deny_add: ["internal/legacy/**"]
```

**Reshape the `/ghu` DAG** — edit `.bender/pipeline.yaml` (nodes, dependencies, `max_concurrent`, `when` expressions). `bender doctor` validates edits.

## Multi-project workspace

`bender register-project <path>` adds a project to `~/.bender/workspace.yaml` (honors `$XDG_CONFIG_HOME`). `bender list-projects` shows them. Every binary command accepts `--project=<name>`.

## Documentation

- Constitution: `.specify/memory/constitution.md`
- Feature specs: `specs/<NNN>-<slug>/`
- Viewer HTTP API: [`ui/README.md`](ui/README.md)

## License

MIT. See [`LICENSE`](LICENSE).
