---
name: implement
user-invocable: true
argument-hint: "<task-id-or-title> — e.g. T012 or a unique title substring"
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

## Workflow

1. **Required argument**: a task id (e.g., `T012`) or a unique substring of a task title.
2. **Resolve** the latest approved plan; refuse if missing (`error: no approved plan; run /plan confirm first`).
3. **Locate the task** in `.bender/artifacts/plan/tasks-<ts>.md`. If the argument matches multiple tasks, list them and refuse.
4. **Dispatch** exactly the agents implied by that task's `agent_hints` (or the orchestrator default if not declared).
5. **Run the same execution graph as `/ghu`**, but pruned to the single task and its direct review/lint follow-ups.
6. **Emit events** identically to `/ghu` — see the "Observability shape" section below (same envelope, same event types, same payload contracts) **except** set `payload.command` to `/implement` in `session_started` and set `payload.stage` to `implement` in every stage/skill/artifact event.
7. **Write the final report** at `.bender/artifacts/ghu/run-<timestamp>-report.md` covering only the executed task.

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

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/implement",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": ["<spec path>","<tasks path>"],
  "skills_invoked": ["<skill names actually invoked>"],
  "files_changed": <int>,
  "findings_count": <int>
}
```

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names other than `implement` — WRONG.
- `kind` on `artifact_written` — WRONG.

## Notes

- All write-scope, failure-policy, and serialization rules from `/ghu` apply identically here.
- `--abort-on-failure` is implicit — there are no siblings to continue.
