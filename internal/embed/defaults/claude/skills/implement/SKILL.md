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
6. **Emit events** identically to `/ghu`.
7. **Write the final report** at `.bender/artifacts/ghu/run-<timestamp>-report.md` covering only the executed task.

## Notes

- All write-scope, failure-policy, and serialization rules from `/ghu` apply identically here.
- `--abort-on-failure` is implicit — there are no siblings to continue.
