---
name: implement
user-invocable: true
argument-hint: "<task-id-or-title> [--skip=<name>[,<name>...]] — e.g. T012 --skip=lint,docs"
context: fg
description: "Execute a single named task from the latest approved plan. Same as /ghu but scoped to one task."
provides: [stage, execute, single-task]
stages: [implement]
applies_to: [any]
inputs:
  - .bender/artifacts/specs/*.md
  - .bender/artifacts/plan/tasks-*.md
outputs:
  - .bender/artifacts/ghu/run-<timestamp>-report.md
---

# `/implement <task-id-or-title>` — Single-Task Execution

Same execution machinery as `/ghu`, but only the named task is dispatched.

## User Input

```text
$ARGUMENTS
```

## Event emission discipline — STREAM, never batch

**Every** event in "Observability shape" MUST be appended to `.bender/sessions/<id>/events.jsonl` the moment its trigger happens — **one Bash tool call per event**, not a single `Write` at the end. The bender-ui viewer tails the file via fsnotify; batching collapses the timeline into one notification and the user sees `Waiting for events…` for the full run.

```bash
printf '%s\n' '<single-line JSON>' >> .bender/sessions/<id>/events.jsonl
```

Intent events (`skill_invoked`, `orchestrator_decision`, `agent_started`) append BEFORE the action; result events (`file_changed`, `artifact_written`, `skill_completed`, `agent_completed`, `stage_completed`, `session_completed`) append AFTER. Progress events (`orchestrator_progress`, `agent_progress`) append as their percent changes. Never buffer events and flush them with one `Write`.

## Workflow

### Worktree provisioning (MANDATORY — first action, no silent fall-back)

Before creating `.bender/sessions/<id>/`, writing `state.json`, or dispatching any
stage, this skill MUST provision a git worktree via the bender binary. Every file
the pipeline writes during this session lands inside that worktree; the main
working tree is never touched by the pipeline.

```bash
# 1. Generate <session-id> — same format as before (UTC timestamp + rand3).
SESSION_ID="$(date -u +%Y-%m-%dT%H-%M-%S)-$(head -c 6 /dev/urandom | od -An -tx1 | tr -d ' \n' | head -c 3)"

# 2. Probe the bender binary.
if command -v bender >/dev/null 2>&1 && bender worktree --help >/dev/null 2>&1; then
    WT_OUT="$(bender worktree create "$SESSION_ID")"
elif [[ -x .specify/extensions/worktree/scripts/bash/worktree.sh ]]; then
    WT_OUT="$(bash .specify/extensions/worktree/scripts/bash/worktree.sh create "$SESSION_ID")"
else
    printf 'error: bender v0.18.0+ is required for worktree-isolated sessions.\n' >&2
    printf 'Install: go install github.com/mayckol/ai-bender/cmd/bender@latest\n' >&2
    printf '    OR   drop .specify/extensions/worktree/scripts/ into the project.\n' >&2
    exit 1
fi

# 3. Parse the binary's two output lines:
#    worktree: <absolute path>
#    branch:   bender/session/<SESSION_ID>
WORKTREE_PATH="$(printf '%s\n' "$WT_OUT" | awk '/^worktree:/ {print $2}')"
SESSION_BRANCH="$(printf '%s\n' "$WT_OUT" | awk '/^branch:/ {print $2}')"

# 4. Adopt the worktree as the working directory for every subsequent tool call.
cd "$WORKTREE_PATH"

# 5. state.json and events.jsonl have ALREADY been populated by
#    `bender worktree create` — do not rewrite them here. The skill continues
#    with its single-task dispatch next, per the steps below.
```

On refusal (exit code 10/11/12/13 from `bender worktree create`), abort the
session immediately — never fall back to writing in the main tree.

<!-- END WORKTREE PROVISIONING BLOCK — do not modify this comment;
     tests/integration/skills_initialisation_test.go grep for it. -->

1. **Required argument**: a task id (e.g., `T012`) or a unique substring of a task title.
2. **Resolve** the latest approved plan; refuse if missing (`error: no approved plan; run /plan confirm first`).
3. **Locate the task** in `.bender/artifacts/plan/tasks-<ts>.md`. If the argument matches multiple tasks, list them and refuse.
4. **Load `.bender/pipeline.yaml`** and evaluate its variables exactly as `/ghu` does (§"Evaluate the pipeline's declared variables" in `.claude/skills/ghu/SKILL.md`). Emit the `pipeline_loaded` `orchestrator_decision`. `tdd_mode` and `plan_refactor` drive the same branches.
5. **Narrow the candidate node set to the task.** Filter pipeline nodes by the resolved task's `agent_hints` — any node whose `agent` is not named in the hints (or whose `agent` is explicitly skipped via `--skip`) is treated as `when: false` for this run. Always retain the read-only orientation nodes (`scout`, `architect`) and the post-implementation review nodes (reviewer, sentinel, benchmarker, scribe) unless `--skip` removes them.
6. **Honor `--skip=<name>[,<name>...]`** identically to `/ghu` (same name set, same aliases, same rules: `crafter` and `scout` are not skippable; `tester` is rejected in TDD mode; unknown names error out; every resolved skip emits `orchestrator_decision` with `decision_type: "skip"`).
7. **Walk the filtered DAG** using the exact algorithm in `.claude/skills/ghu/SKILL.md` §"Walk the pipeline DAG". Parallelism is emergent from the DAG; any batch of ≥2 ready nodes dispatches via a single `orchestrator_decision(parallel_dispatch)` followed by one Agent tool call per node in the SAME assistant message. Never one-per-turn.
8. **Agent attribution.** Every event emitted during an agent's work MUST carry `payload.agent: "<agent-name>"` so the viewer and `bender sessions validate` can thread events by responsible agent — applies to `skill_invoked`, `skill_completed`, `skill_failed`, `file_changed`, `finding_reported`, `agent_progress`. Orchestrator decisions about a specific agent use `payload.dispatched_agent`.
9. **Emit events** identically to `/ghu` — see the "Observability shape" section below (same envelope, same event types, same payload contracts) **except** set `payload.command` to `/implement` in `session_started` and set `payload.stage` to `implement` in every stage/skill/artifact event. `session_completed.payload.skipped` carries the canonical names of any agents/groups the user bypassed.
10. **Write the final report** at `.bender/artifacts/ghu/run-<timestamp>-report.md` covering only the executed task. Include a "Skipped" line in the summary header when `--skip` was in effect. If TDD mode was active, prefix the report summary with `**TDD mode:** Red → Green → Refactor` and include the RED and GREEN finding ids.

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as `/ghu`. Stage is **`implement`** for every stage/skill/artifact event. Skip `orchestrator_decision` of kind `task_decomposition` — there is only one task.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/implement","invoker":"<$USER>","working_dir":"<abs path>","registered_projects":[],"parallelism":1}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"implement"},"type":"stage_started","payload":{"stage":"implement","inputs":[".bender/artifacts/specs/<slug>-<ts>.md",".bender/artifacts/plan/tasks-<ts>.md"]}}
```

### Everything else
Identical to the `/ghu` shapes — `agent_started` / `agent_progress` / `agent_completed` / `agent_failed` / `agent_blocked`, `skill_invoked` / `skill_completed` / `skill_failed`, `file_changed`, `finding_reported`, `artifact_written` — but with `payload.stage: "implement"` on every stage/skill/artifact event. See `.claude/skills/ghu/SKILL.md` for the full set. The `agents_summary` in `session_completed` typically has 1-3 entries (crafter + tester + optional reviewer).

### state.json (authored by `bender worktree create`)

`state.json` is written by the binary (or the fallback script) during the
Worktree provisioning step, at schema v2, with `worktree`, `session_branch`,
`base_branch`, and `base_sha` populated. The orchestrator does NOT rewrite
this file during the session — it only updates the top-level `status`,
`completed_at`, `skills_invoked`, `files_changed`, and `findings_count`
fields at terminal transitions, preserving every v2 worktree field verbatim.

Full schema: `internal/session/state.go`'s `State` struct plus
`specs/004-worktree-flow/contracts/state-v2.schema.yaml`.

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names other than `implement` — WRONG.
- `kind` on `artifact_written` — WRONG.

## Notes

- All write-scope, failure-policy, and serialization rules from `/ghu` apply identically here.
- `--abort-on-failure` is implicit — there are no siblings to continue.
