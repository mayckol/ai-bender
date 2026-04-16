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
# → .claude/{skills,agents}, .claude/groups.yaml, bender.yaml, artifacts/constitution.md

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
| `--config <path>` | Override the path to the project `bender.yaml`. |
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

Produce a single JSON document with the final `state.json` plus the full ordered event list. The shape is the v1 ingest contract for a future UI server. Round-trips losslessly through `bender sessions show` re-emission (SC-009).

```bash
bender sessions export 2026-04-16T14-03-22-a3f > /tmp/session.json
```

| Exit | Meaning |
|---|---|
| 0 | Success |
| 50 | Session not found |

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

#### `/ghu [--from=<spec>] [--only=<task>] [--abort-on-failure]`

Execute the approved plan. The only stage that writes code. Walks the default execution graph (scout → architect → optional surgeon → crafter ∥ tester → linter → reviewer ∥ sentinel ∥ benchmarker ∥ scribe → final report) and produces:
- Source-tree mutations within each agent's declared write scope.
- `artifacts/ghu/run-<ts>-report.md` final report.
- `artifacts/ghu/{reviews,security,perf}/<ts>/` per-agent outputs.
- `artifacts/.bender/sessions/<id>/{state.json,events.jsonl}` for `bender sessions` to inspect.

Refuses to start if any required upstream artifact is missing. With `--abort-on-failure`, halts pending agents on first failure; default policy marks the failed agent as blocked and continues siblings.

#### `/implement <task-id-or-title>`

`/ghu` scoped to a single task from the latest approved plan. All write-scope, failure-policy, and serialization rules from `/ghu` apply identically.

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

### Default groups (named selectors in `.claude/groups.yaml`)

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

## What's in `.claude/` after `bender init`

```
.claude/
├── agents/                 # 10 default subagents (crafter, tester, reviewer, linter,
│                           #   architect, scribe, scout, sentinel, benchmarker, surgeon)
├── skills/
│   ├── cry/SKILL.md        # the /cry slash-command instructions
│   ├── plan/SKILL.md
│   ├── tdd/SKILL.md
│   ├── ghu/SKILL.md
│   ├── implement/SKILL.md
│   ├── bender-doctor/SKILL.md
│   ├── fg-bootstrap-detect-*/SKILL.md   (10)
│   ├── fg-cry-*/SKILL.md                (5)
│   ├── fg-plan-*/SKILL.md               (7)
│   ├── fg-tester-*/SKILL.md             (7, TDD scaffolds)
│   ├── bg-crafter-*/SKILL.md            (7)
│   ├── bg-tester-*/SKILL.md             (6)
│   ├── bg-linter-*/SKILL.md             (5)
│   ├── bg-reviewer-*/SKILL.md           (5)
│   ├── bg/fg-architect-*/SKILL.md       (5)
│   ├── bg-scribe-*/SKILL.md             (5)
│   ├── bg-scout-*/SKILL.md              (6)
│   ├── bg-sentinel-*/SKILL.md           (6)
│   ├── bg-benchmarker-*/SKILL.md        (5)
│   └── bg-surgeon-*/SKILL.md            (5)
└── groups.yaml             # bootstrap, pre-implementation-checks, security-sweep
bender.yaml                 # per-project agent/skill overrides (lives at project root)
artifacts/
└── constitution.md         # heuristic project profile; AI-required sections marked pending
```

## Extending without modifying core

Drop a new skill into `.claude/skills/<name>/SKILL.md`. Every agent whose selector matches it will pick it up on the next `bender doctor` and the next slash-command run. No registration, no rebuild.

Drop `.claude/agents/<name>.md` to override an embedded default agent (same-name fully replaces). Edit `.claude/groups.yaml` to redefine groups.

Per-project tweaks without forking go in `bender.yaml`:

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

## Documentation

- Spec: `specs/001-ai-bender-pipeline/spec.md`
- Plan: `specs/001-ai-bender-pipeline/plan.md`
- Contracts: `specs/001-ai-bender-pipeline/contracts/`
- Quickstart with success criteria: `specs/001-ai-bender-pipeline/quickstart.md`
- Constitution (engineering principles): `.specify/memory/constitution.md`

## License

TBD.
