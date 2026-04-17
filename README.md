<p align="center">
  <img src="docs/bender-logo.png" alt="ai-bender" width="240" />
</p>

# ai-bender

A spec-driven scaffold for Claude Code. `bender` is a Go CLI that installs a `.claude/` workspace into your projects and gives you slash commands you run inside Claude Code: `/cry`, `/plan`, `/tdd`, `/ghu`, `/implement`. Think Spec Kit's `/specify`, but with a multi-project workspace, a heuristic-discovered project constitution, and `bender doctor` for catalog validation.

The binary itself never calls an LLM. It scaffolds, validates, and inspects. Claude Code (or any client that reads `.claude/`) is the executor.

## Install

```bash
go install github.com/mayckol/ai-bender/cmd/bender@latest
```

(Homebrew tap and `curl | sh` installer are planned for v1; until then, `go install` or build from source.)

```bash
git clone https://github.com/mayckol/ai-bender
cd ai-bender
make build install
```

## Prerequisites for `/bender-doctor`

The `/bender-doctor` slash command (and any future slash command that delegates to the binary) needs **either** the `bender` binary on your `PATH` **or** the matching shell fallback script in the project at `.specify/scripts/bash/bender-doctor.sh` (or the PowerShell counterpart on Windows).

If you see this message inside Claude Code:

> The bender binary is not on PATH, and the fallback script path `.specify/scripts/bash/bender-doctor.sh` does not exist in this repo. Cannot validate the catalog — neither bender nor the fallback is available.

…it means neither was found. Resolve it one of two ways:

1. **Install the binary globally** (recommended):

   ```bash
   go install github.com/mayckol/ai-bender/cmd/bender@latest
   # then verify
   which bender   # → /Users/you/go/bin/bender (or similar)
   bender doctor  # → status: healthy
   ```

   Make sure `$(go env GOPATH)/bin` is on your `PATH`.

2. **Drop the fallback into the project** (if installing globally isn't an option):

   ```bash
   # from the ai-bender repo
   mkdir -p <target-project>/.specify/scripts/bash
   cp .specify/scripts/bash/bender-doctor.sh <target-project>/.specify/scripts/bash/
   ```

   The fallback is a thin Bash script that checks `.claude/` for parse errors. It does not replace the full catalog walk the binary performs — install the binary if you can.

Note: `bender init` only materializes the `.claude/` workspace. The `.specify/scripts/` fallbacks live in *this* repo and are not embedded in the binary; we don't push them into target projects automatically because not every project wants the extra surface.

## Quickstart

```bash
# 1. Scaffold a project
cd ~/projects/my-service
bender init
# → .claude/{skills,agents} (Claude Code native), .bender/{pipeline.yaml,groups.yaml,artifacts/constitution.md} (bender-owned config + runtime)

# 2. (optional) register it for cross-project tooling
bender register-project ~/projects/my-service --name my-service
bender list-projects

# 3. Open the project in Claude Code and run the slash commands
#    /cry "users need roles"
#    /cry confirm
#    /plan
#    /plan confirm
#    /tdd "users can be assigned a role"
#    /ghu

# 4. Inspect what happened
bender sessions list
bender sessions show <session-id>
bender sessions export <session-id> > /tmp/session.json
bender sessions validate <session-id>   # check state.json + events.jsonl against the v1 schema

# 5. Validate the catalog any time
bender doctor
```

## Command reference

There are two surfaces:

1. **Binary commands** (`bender <verb>`) — run directly from a shell. Every binary command has a Bash counterpart at `.specify/scripts/bash/bender-<verb>.sh` and a PowerShell counterpart at `.specify/scripts/powershell/bender-<verb>.ps1` so projects degrade gracefully on machines without the binary installed.
2. **Slash commands** (`/<verb>`) — markdown files materialized under `.claude/skills/<verb>/SKILL.md`. Claude Code reads them when you type the slash command. The binary never executes them.

### Binary commands

#### Global flags (apply to every binary command)

| Flag | Purpose |
|---|---|
| `--project <name>` | Operate on a registered project by name (defaults to the project containing the cwd). |
| `--config <path>` | Override the path to the project config file. |
| `--no-color` | Disable color output. |
| `--quiet` | Suppress informational logs; errors still print. |
| `--verbose` | Print loader and resolver decisions to stderr. |
| `--help`, `-h` | Per-command help. |
| `--version` | Print the binary version and exit. |

#### `bender init [--here] [--force]`

Scaffold `.claude/` from the embedded defaults, run heuristic discovery, and write `artifacts/constitution.md`. Idempotent — re-runs preserve user-modified files unless `--force`.

```bash
cd ~/projects/my-service
bender init
# bender: scaffolded 102 files (0 preserved)
# bender: constitution written → artifacts/constitution.md
# bender: pending sections:
#   - purpose: derived from README/manifests by /cry or /bender-bootstrap
#   - conventions: naming/error/DI/architecture pattern (run /bender-bootstrap)
#   …
# next: open this project in Claude Code and try `/cry "<your request>"`
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 2 | Partial (some discovery sections marked pending) |
| 10 | Filesystem error |

#### `bender sync-defaults [--force]`

Re-materialize embedded defaults for files added since the last `init`. Use after `bender update` (or after pulling a new release) to pick up new defaults without losing your customizations.

```bash
bender sync-defaults
# sync-defaults: 3 added, 99 preserved

bender sync-defaults --force
# sync-defaults: 0 added, 0 preserved, 102 overwritten via --force
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 10 | Filesystem error |

#### `bender register-project <path> [--name <name>]`

Add a project to the global workspace registry at `~/.bender/workspace.yaml` (or `$XDG_CONFIG_HOME/bender/workspace.yaml` if set). When `--name` is omitted the directory's basename is kebab-case-normalized.

```bash
bender register-project ~/projects/api --name api
# registered: api → /Users/me/projects/api

bender register-project ~/projects/MyService_v2
# registered: myservice-v2 → /Users/me/projects/MyService_v2
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 30 | Validation error (bad name, missing path) |
| 31 | Duplicate name |

#### `bender list-projects`

List registered projects. The project containing the cwd is marked `current`; projects whose paths no longer exist are marked `missing`.

```bash
bender list-projects
# NAME          STATUS     PATH
# api           current    /Users/me/projects/api
# web           available  /Users/me/projects/web
# old-service   missing    /Users/me/projects/old-service
```

#### `bender doctor [--project=<name>]`

Validate the catalog: load embedded defaults plus `.claude/` overrides, resolve every (agent × stage × issue type) combination, and report empty skill sets, broken selectors, missing required external tools, and parse errors.

```bash
bender doctor
# bender doctor: 90 skills, 10 agents, 3 groups loaded
# SEVERITY  CATEGORY         SUBJECT                                  MESSAGE
# info      broken-selector  agent=crafter skills.patterns="check-*"  glob pattern matches no current skills (will pick up matching user-added skills)
# status: healthy
```

| Exit | Meaning |
|---|---|
| 0 | Healthy (no warnings or errors) |
| 40 | Warnings present |
| 41 | Errors that would block a slash-command run |

#### `bender sessions list [--project=<name>]`

List on-disk sessions Claude wrote during slash-command runs.

```bash
bender sessions list
# ID                            COMMAND  STARTED_AT            DURATION  STATUS     FILES  FINDINGS
# 2026-04-16T14-03-22-a3f       /ghu     2026-04-16T14:03:22Z  2m8s      completed  7      2
# 2026-04-16T15-12-04-9be       /plan    2026-04-16T15:12:04Z  1m45s     completed  0      0
```

#### `bender sessions show <session-id> [--project=<name>]`

Print every event from `events.jsonl` in original order (NDJSON, byte-identical to the on-disk file). Suitable for piping into `jq`.

```bash
bender sessions show 2026-04-16T14-03-22-a3f | jq -c 'select(.type=="finding_reported")'
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 50 | Session not found |

#### `bender sessions export <session-id> [--project=<name>]`

Produce a single JSON document with the final `state.json` plus the full ordered event list. The shape is the v1 ingest contract for the **bender-ui** server (see [Real-time viewer](#real-time-viewer)). Round-trips losslessly through `bender sessions show` re-emission (SC-009).

```bash
bender sessions export 2026-04-16T14-03-22-a3f > /tmp/session.json
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 50 | Session not found |

#### `bender sessions validate <session-id> [--project=<name>]`

Check a session's `state.json` and `events.jsonl` against the v1 schema contract. Reports every drift in one pass: missing top-level state fields (`schema_version`, `command`, `status`, `completed_at` when terminal), per-event envelope problems (unknown types, missing `actor`), per-event payload-field requirements (e.g., `session_started` must carry `invoker` and `working_dir`; `agent_completed` must carry `agent`, `task_ids`, `duration_ms`), session-id consistency across every line, and cross-file consistency (if `state.findings_count > 0`, the log must contain at least one `finding_reported` event).

```bash
bender sessions validate 2026-04-16T14-03-22-a3f
# ok: 2026-04-16T14-03-22-a3f is schema-compliant
```

| Exit | Meaning |
|---|---|
| 0 | Compliant |
| 50 | Session not found |
| 51 | One or more schema violations |

#### `bender sessions clear [<session-id>] [--all]`

Remove session data from disk. Artifacts under `.bender/artifacts/` are **preserved** — they are the authored outputs of a run, not the session log.

```bash
bender sessions clear 2026-04-16T22-04-07-f6g   # remove one
bender sessions clear --all                     # remove every session + the scout cache
```

Behavior:

- `bender sessions clear <id>` removes `<.bender/sessions/<id>/>` and the matching `.bender/cache/scout/<id>/` directory.
- `bender sessions clear --all` removes every subdirectory of `.bender/sessions/` plus the entire `.bender/cache/` tree (so the next run starts with a cold scout cache).
- Running `bender sessions clear` with neither `<id>` nor `--all` is rejected — no accidental mass-delete.

| Exit | Meaning |
|---|---|
| 0 | Success |
| 50 | `<id>` not found (single-id form) |
| 73 | Neither `<id>` nor `--all` supplied |

#### `bender update [--check]`

Replace the current `bender` binary with the latest release. With `--check`, only print the available version without modifying anything.

> **v1 note**: a release server isn't published yet. `--check` prints the current version as up-to-date; running without `--check` exits non-zero with a message pointing at `go install` / Homebrew / `curl | sh`. The command exists for surface completeness so a future release can plug in without changing the CLI shape.

| Exit | Meaning |
|---|---|
| 0 | Success |
| 20 | Network error / no release channel |
| 21 | Integrity check failed |
| 22 | Permission denied |

#### `bender version`

Print the binary version and exit.

```bash
bender version
# 0.1.0
```

### Base stages at a glance

A **stage** is a user-invocable slash command whose skill file orchestrates a single end-to-end step. Every stage reads its inputs from `.bender/artifacts/` (or the constitution), writes its outputs there, and — when long-running — tracks progress via `orchestrator_progress` events the [real-time viewer](#real-time-viewer) renders as a bar.

| Stage | Command | Inputs | Outputs | Notes |
|---|---|---|---|---|
| bootstrap | `/bender-bootstrap` | discovered project files | `.bender/artifacts/constitution/<ts>.md` + merged `.bender/artifacts/constitution.md` | One-shot; fills the project constitution from heuristics. |
| cry | `/cry <request>` | user natural-language request | `.bender/artifacts/cry/<slug>-<ts>.md` (status: draft → approved after `/cry confirm`) | Classifies intent and drafts the feature idea. |
| plan | `/plan` | latest approved cry | `.bender/artifacts/specs/<slug>-<ts>.md` + `plan/data-model-<ts>.md` + `plan/risk-assessment-<ts>.md` + `plan/tasks-<ts>.md` (all draft → approved after `/plan confirm`) | Architect-led design pass. |
| tdd *(optional)* | `/tdd [<scenario>...]` | latest approved plan | `.bender/artifacts/plan/tests/**/*.md` — prose-only scaffolds (draft → approved after `/tdd confirm`) | Enables TDD mode in `/ghu` if approved. |
| ghu | `/ghu [--bg \| --inline] [--skip=…] [--only=…]` | approved spec + tasks (+ optional tdd scaffolds) | `.bender/sessions/<id>/` (state + events) + `.bender/artifacts/ghu/run-<ts>-report.md` + per-agent outputs | The only stage that writes source code. |
| implement | `/implement <task-id-or-title> [--skip=…]` | same as `/ghu`, scoped to one task | same report path as `/ghu` | Pruned `/ghu` for a single task. |
| doctor | `/bender-doctor` | `.claude/` catalog + `.bender/config` (if any) | stdout health report | Wrapper around `bender doctor` with narrative. |

Progress is reported at two levels during `/ghu` / `/implement`:

- **Session-level** — the orchestrator emits `orchestrator_progress` events with `{percent, current_step, completed_nodes, remaining_nodes}` after each graph-node transition. Baseline anchors: scout 10%, architect 20%, mid-cycle 50%, linter 70%, review-sweep 90%, final report 100%.
- **Agent-level** — each worker agent emits `agent_progress` events with `{agent, percent, current_step}` so the viewer can thread per-agent progress alongside the session bar.

### Extending stages

A stage is just a skill file under `.claude/skills/<name>/SKILL.md` with `user-invocable: true` and `stages: [<name>]`. You can add, update, or remove stages — the bender binary doesn't hard-code them. What you DO need to keep in sync is the **orchestrator skill**: every downstream stage that consumes your stage's outputs reads from paths declared in its own SKILL.md.

- **Add** a new stage: drop `.claude/skills/<your-stage>/SKILL.md` with frontmatter (`name`, `user-invocable: true`, `stages: [<your-stage>]`, `inputs`, `outputs`). If a later stage should consume your outputs, edit that stage's SKILL.md so it reads the new artifact paths.
- **Update** a stage: edit the SKILL.md. If the change alters the artifact paths or the event shape, update every downstream consumer (the orchestrator skill of the next stage). Running `bender doctor` after the edit catches broken inputs/outputs references.
- **Remove** a stage: delete `.claude/skills/<stage>/` and edit every downstream SKILL.md that referenced it (e.g., `/ghu` mentions approved plan artifacts; if `/plan` goes away, `/ghu` needs a new source).

`bender doctor` validates the catalog; `bender sessions validate` validates each run's events. Together they catch both catalog-level and runtime-level drift after a change.

### Slash commands (run in Claude Code)

These are markdown files materialized into `.claude/skills/<name>/SKILL.md` by `bender init`. Claude Code reads them and follows the instructions when you type the slash command. The binary never executes them.

#### `/cry [--type=<bug|feature|performance|architectural>] <request>`

Capture intent at a high level. Classifies the issue type if `--type` is omitted, then writes `artifacts/cry/<slug>-<timestamp>.md` with the verbatim request, interpreted requirements, proposed direction, open questions, and affected areas.

- `/cry "<more context>"` after the first run iterates by writing a new artifact that links to its predecessor via a `previous:` field.
- `/cry confirm` flips the most recent draft to `status: approved` so `/plan` will consume it.

#### `/plan [--from=<cry-artifact>]`

Low-level design. From the latest approved capture, produces a coherent plan set under one shared timestamp:
- `artifacts/specs/<slug>-<ts>.md`
- `artifacts/plan/data-model-<ts>.md`
- `artifacts/plan/api-contract-<ts>.md` (only when an external interface is involved)
- `artifacts/plan/risk-assessment-<ts>.md`
- `artifacts/plan/tasks-<ts>.md`

`/plan confirm` flips the entire set to `status: approved` atomically.

#### `/tdd [<scenario>...]`

Optional. Mirrors the source tree under `artifacts/plan/tests/` with prose-only test descriptions per source file. No executable test code is written. `/tdd confirm` approves the scaffold set.

#### `/ghu [--bg | --inline] [--from=<spec>] [--only=<task>] [--skip=<name>[,<name>...]] [--abort-on-failure]`

Execute the approved plan. The only stage that writes code. If `/tdd` produced approved scaffolds under `.bender/artifacts/plan/tests/`, `/ghu` switches into **TDD mode** (Red → Green → Refactor): tester materialises executable tests from the scaffolds and runs them until they fail, crafter then implements until they pass, a surgeon cleanup pass keeps tests green. Otherwise it walks the plain graph (scout → architect → optional surgeon → crafter ∥ tester → linter → reviewer ∥ sentinel ∥ benchmarker ∥ scribe → final report). Both paths produce:
- Source-tree mutations within each agent's declared write scope.
- `artifacts/ghu/run-<ts>-report.md` final report.
- `artifacts/ghu/{reviews,security,perf}/<ts>/` per-agent outputs.
- `artifacts/.bender/sessions/<id>/{state.json,events.jsonl}` for `bender sessions` to inspect.

**Execution mode**

- `/ghu` (default) — runs in an **isolated subagent** via Claude Code's `Agent` tool with `run_in_background: true`. The main conversation stays clean: you see the session ID and the target report path immediately, and the heavy orchestration (file reads, agent invocations, tool output) happens in a forked context window. You'll be notified when the run completes; the full report is on disk at `.bender/artifacts/ghu/run-<ts>-report.md`.
- `/ghu --bg` — same as default; explicit form.
- `/ghu --inline` — opt out of the fork and run the workflow directly in the current conversation. Use this for debugging, short scoped runs (`--only=<task>`), or when you want to observe each step as it happens.

**Skipping stages.** `--skip` takes a comma-separated list of agent names, group names, or their aliases:

| Name | Aliases | What it skips |
|---|---|---|
| `linter` | `lint` | the linter pass + `bg-linter-*` |
| `tester` | `test`, `tests` | the tester agent (rejected in TDD mode) |
| `reviewer` | `review` | reviewer critique + PR summary |
| `sentinel` | `security`, `sec` | security sweep |
| `benchmarker` | `perf`, `bench` | perf pass |
| `scribe` | `docs` | doc + inline-comment updates |
| `surgeon` | `refactor` | refactor pass |
| `architect` | — | architect validation during /ghu |
| `review-sweep` | `reviews` | the whole parallel reviewer ∥ sentinel ∥ benchmarker ∥ scribe fan-out |

`crafter` and `scout` are not skippable (the former because the run would produce nothing, the latter because its cache is what keeps every other agent token-efficient). Each resolved skip is recorded as an `orchestrator_decision` event with `decision_type: "skip"`, and the run report's summary header lists the bypassed names.

Refuses to start if any required upstream artifact is missing. With `--abort-on-failure`, halts pending agents on first failure; default policy marks the failed agent as blocked and continues siblings.

#### `/implement <task-id-or-title> [--skip=<name>[,<name>...]]`

`/ghu` scoped to a single task from the latest approved plan. All write-scope, failure-policy, `--skip`, and serialization rules from `/ghu` apply identically.

#### `/bender-doctor`

Wrapper around `bender doctor`. Runs the binary's catalog walker via the shell and summarises the output for the user with a recommendation (healthy / warnings present / blocking errors present).

#### `/bender-bootstrap`

Fills the three constitution sections (`Purpose`, `Conventions`, `Glossary`) that `bender init` marks `_pending: true`. Reads README + manifests + a sample of source files, synthesises the missing sections, archives the prior constitution under `artifacts/constitution/<timestamp>.md`, and writes the new one. Run this once after `bender init` (and re-run any time the codebase changes shape enough that the conventions or glossary should be revisited).

### Default agents (subagents Claude invokes during `/ghu`)

| Agent | Purpose | Write scope |
|---|---|---|
| `crafter` | Implement production code with minimum-diff edits | source files; never tests, docs, CI |
| `tester` | Author and run tests; enumerate edge cases | tests, fixtures, testdata only |
| `reviewer` | Critique against spec / constitution / idioms | read-only; writes findings to `artifacts/ghu/reviews/` |
| `linter` | Run linters/formatters; safe autofixes | autofix-only; no semantic changes |
| `architect` | Design + boundary validation | writes during `/plan`; read-only during `/ghu` |
| `scribe` | Keep docs in sync with code | docs/comments only; never logic |
| `scout` | Read-only codebase exploration with caching | zero write scope |
| `sentinel` | Security analysis (secrets, deps, inputs, auth, crypto) | read-only; writes findings to `artifacts/ghu/security/` |
| `benchmarker` | Performance analysis with measurement | read-only; writes findings to `artifacts/ghu/perf/` |
| `surgeon` | Behavior-preserving refactors (tests pass before AND after) | source files |

### Default groups (named selectors in `.bender/groups.yaml`)

| Group | Purpose | Selector |
|---|---|---|
| `bootstrap` | Refine constitution sections that the binary's heuristic discovery cannot fill | `patterns: ["fg-bootstrap-detect-*"]` |
| `pre-implementation-checks` | Validations run before crafter begins writing code | `tags.any_of: [check, validation]`, `context: [bg]` |
| `security-sweep` | Post-implementation security sweep | `patterns: ["bg-sentinel-*"]` |

### Shell fallbacks

| Bash | PowerShell |
|---|---|
| `.specify/scripts/bash/bender-init.sh` | `.specify/scripts/powershell/bender-init.ps1` |
| `.specify/scripts/bash/bender-sync-defaults.sh` | `.specify/scripts/powershell/bender-sync-defaults.ps1` |
| `.specify/scripts/bash/bender-register-project.sh` | `.specify/scripts/powershell/bender-register-project.ps1` |
| `.specify/scripts/bash/bender-list-projects.sh` | `.specify/scripts/powershell/bender-list-projects.ps1` |
| `.specify/scripts/bash/bender-doctor.sh` | `.specify/scripts/powershell/bender-doctor.ps1` |
| `.specify/scripts/bash/bender-sessions-list.sh` | `.specify/scripts/powershell/bender-sessions-list.ps1` |
| `.specify/scripts/bash/bender-sessions-show.sh` | `.specify/scripts/powershell/bender-sessions-show.ps1` |
| `.specify/scripts/bash/bender-sessions-export.sh` | `.specify/scripts/powershell/bender-sessions-export.ps1` |
| `.specify/scripts/bash/bender-update.sh` | `.specify/scripts/powershell/bender-update.ps1` |

The fallbacks produce the same artifact layout the binary does. AI-driven slash commands (`/cry`, `/plan`, …) live in `.claude/skills/` and are executed by Claude Code itself — there's no shell fallback for those because the AI is the executor.

## Project layout after `bender init`

`bender init` touches exactly two top-level locations: `.claude/` (configuration for Claude Code) and `.bender/` (everything bender produces, including per-project config).

```
your-project/
├── .claude/                        # Claude Code–native artefacts
│   ├── agents/                     #   10 default subagents
│   └── skills/                     #   7 slash-command skills + 20 worker skills
├── .bender/                        # bender-owned config + runtime — the only bender root
│   ├── pipeline.yaml               #   declarative execution DAG for /ghu + /implement
│   ├── groups.yaml                 #   named skill-selector bundles
│   ├── artifacts/                  #   human-readable pipeline output (commit this)
│   │   ├── constitution.md         #     current constitution
│   │   ├── constitution/<ts>.md    #     prior revisions
│   │   ├── cry/<slug>-<ts>.md      #     /cry output
│   │   ├── specs/<slug>-<ts>.md    #     /plan output
│   │   ├── plan/{data-model,api-contract,risk-assessment,tasks}-<ts>.md
│   │   ├── plan/tests/…            #     /tdd scaffolds
│   │   └── ghu/run-<ts>-report.md + {reviews,security,perf}/
│   ├── sessions/<id>/              #   state.json + events.jsonl (gitignored)
│   └── cache/                      #   scout caches (gitignored)
│   └── config.yaml             # per-project agent/skill overrides

artifacts/
└── constitution.md         # heuristic project profile; AI-required sections marked pending
```

## Customising agents and skills

Claude Code reads `.claude/agents/*.md` and `.claude/skills/*/SKILL.md` directly. There is no indirection layer — to change which skills an agent uses, edit the agent file; to add a new agent, drop a new agent file. The registry loader in `internal/agent` walks the embedded defaults first and then `.claude/`, so a same-name user file fully replaces the embedded default and a new-name file is added.

### Change an existing agent's skill binding

```bash
# .claude/agents/crafter.md (already on disk after bender init)
#   skills:
#     patterns: ["bg-crafter-*"]
#     tags: { none_of: [destructive, read-only] }
```

Edit the `skills` or `write_scope` blocks directly. `bender doctor` validates the result. That's it — no regeneration step.

### Add a brand-new agent

1. Drop a file at `.claude/agents/<your-name>.md` with the standard agent frontmatter.
2. Optionally drop its skills at `.claude/skills/<skill>/SKILL.md`.
3. `bender doctor` validates the new catalog.

Example agent file:

```yaml
---
name: security-auditor
purpose: "Deep review for authN/authZ regressions"
persona_hint: "paranoid senior security engineer"
write_scope:
  allow: []                          # read-only: writes findings, not code
  deny:  ["**"]
skills:
  explicit: [bg-sentinel-deep-auth]  # bind to skills you also added
  patterns: ["bg-sentinel-*"]
context: [bg]
invoked_by: [ghu, implement]
---

# security-auditor

Agent body / persona prose goes here.
```

## Extending without modifying core

Drop a new skill into `.claude/skills/<name>/SKILL.md`. Every agent whose selector matches it will pick it up on the next `bender doctor` and the next slash-command run. No registration, no rebuild.

Drop `.claude/agents/<name>.md` to override an embedded default agent (same-name fully replaces). Edit `.bender/groups.yaml` to redefine group selectors. Edit `.bender/pipeline.yaml` to reshape the `/ghu` + `/implement` execution DAG — add nodes, change dependencies, tune `max_concurrent`, gate branches with `when` expressions. No code changes required; `bender doctor` validates your edits.

Per-project tweaks without forking go in `.bender/config.yaml`:

```yaml
agents:
  crafter:
    skills:
      add: [project-specific-migration-check]
      remove: [check-data-model]
    write_scope:
      deny_add: ["internal/legacy/**"]
```

## Multi-project workspace

`bender register-project <path>` adds a project to `~/.bender/workspace.yaml` (or `$XDG_CONFIG_HOME/bender/workspace.yaml`). `bender list-projects` shows them all and marks the current one (the project containing your cwd). Binary commands accept `--project=<name>` to operate against any registered project.

## Real-time viewer

The `bender` binary embeds a small local web viewer (`bender-ui`) that live-streams session events over Server-Sent Events. It reads `.bender/sessions/<id>/` directly — the same files the CLI reads — so there is no separate database and no migration. Designed for watching `/ghu --bg` runs as they unfold in a forked subagent context.

```bash
# From any project that has .bender/ in it:
bender server                 # detaches, pid+log under .bender/
bender server --port 4000     # custom port
bender server --foreground    # stay attached (useful under systemd/launchd)
bender server status
bender server stop
```

Open `http://localhost:4317/`:

- `/` — session list with live updates when new sessions appear.
- `/sessions/<id>` — live timeline: event stream with a coloured agent badge on every row, clickable agent filter chips, findings panel, file-changed summary, report link. Freezes on `session_completed`.

When `/ghu --bg` dispatches, the SKILL.md instructs Claude Code to print the viewer URL (`http://localhost:4317/sessions/<id>`) alongside the session id and report path. If the server is running, the dispatcher also issues a platform `open` / `xdg-open` once; otherwise the URL is just printed so you can start the viewer later with `bender server`.

| Exit | Meaning |
|---|---|
| 0 | Success |
| 60 | `server` (start): already running (stale pid file cleaned up; retry) |
| 61 | `server stop` / `server status`: not running |
| 62 | `server` (start): failed to spawn the detached child |

The dev-time Bun server under `ui/` (`cd ui && bun run dev`) is kept for fast client iteration (hot rebundle on source edits). `bender server` serves the same UI but from an embedded bundle baked into the binary, so `go install github.com/mayckol/ai-bender/cmd/bender@latest` is all anyone needs to run the viewer.

See [`ui/README.md`](ui/README.md) for the HTTP API and test commands.

## Upgrade notes — v0.17.0

Bender-owned configuration moved out of `.claude/` into `.bender/`. `.claude/` is now reserved for Claude Code–native artefacts (agents + skills); `.bender/` owns every bender-invented file (`pipeline.yaml` + `groups.yaml` + runtime state).

Migration for projects scaffolded on v0.16 or earlier:

```bash
cd path/to/your-project
bender sync-defaults --force
bender doctor         # must report `status: healthy`
```

`sync-defaults --force` creates `.bender/pipeline.yaml` and `.bender/groups.yaml`. The legacy `.claude/groups.yaml` is **left on disk** (non-destructive migration); no runtime code reads it anymore. Delete it by hand whenever convenient.

What changed in the run-time model: `/ghu` and `/implement` no longer follow the prose ASCII graph in their SKILL documents. They load `.bender/pipeline.yaml` and walk it as a declarative DAG — parallelism is emergent from the dependency structure, and `priority` only breaks ties when more nodes are ready than `max_concurrent` allows. Add/remove/reorder pipeline nodes by editing that one YAML file; no SKILL or agent edits required.

## Documentation

- Spec: `specs/001-ai-bender-pipeline/spec.md`
- Plan: `specs/001-ai-bender-pipeline/plan.md`
- Contracts: `specs/001-ai-bender-pipeline/contracts/`
- Quickstart with success criteria: `specs/001-ai-bender-pipeline/quickstart.md`
- Constitution (engineering principles): `.specify/memory/constitution.md`

## License

TBD.
