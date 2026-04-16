---
name: ghu
user-invocable: true
argument-hint: "[--from=<spec>] [--only=<task-id>] [--abort-on-failure]"
context: fg
description: "Execute the approved plan — implement, test, lint, review, and report. The only stage that writes code."
provides: [stage, execute]
stages: [ghu]
applies_to: [any]
inputs:
  - artifacts/specs/*.md
  - artifacts/plan/tasks-*.md
outputs:
  - artifacts/ghu/run-<timestamp>-report.md
---

# `/ghu` — Execute the Plan

Decompose the approved task list, dispatch work to specialised agents, and produce a final report. This is the **only** stage that writes source code.

## User Input

```text
$ARGUMENTS
```

## Pre-Execution Checks

Run any `hooks.before_ghu`.

## Workflow

1. **Resolve the source artifacts**:
   - The latest **approved** spec at `artifacts/specs/<slug>-<ts>.md`.
   - The matching task list at `artifacts/plan/tasks-<ts>.md`.
   - Optionally, test scaffolds at `artifacts/plan/tests/`.
   - If anything required is missing, print:
     - `error: missing required upstream artifact: <name>. Run /plan and /plan confirm first.`
     - Exit. Do **not** create a partial session.

2. **Honor `--only=<task>`** to scope to a single task (same as `/implement`).

3. **Honor `--abort-on-failure`** to halt on the first task failure (default: continue and mark blocked).

4. **Create a session directory** under `artifacts/.bender/sessions/<id>/`. Write `state.json` and append `session_started`, `stage_started`, `orchestrator_decision` (with the task decomposition).

5. **Walk the default execution graph** (overridable per command file):

   ```
   scout (map codebase) → architect (validate approach)
   ↓
   surgeon (if refactor needed)
   ↓
   crafter (implement) ∥ tester (write tests)
   ↓
   linter (autofix + report)
   ↓
   reviewer ∥ sentinel ∥ benchmarker ∥ scribe
   ↓
   final report
   ```

   For each agent invocation:
   - Use the **Agent tool** with `subagent_type=<agent-name>` to invoke it (the agent definitions are at `.claude/agents/<name>.md`).
   - Pass the relevant task IDs, affected files, and acceptance criteria in the agent's prompt.
   - Emit `agent_started`, `agent_progress` (as the agent reports back), `agent_completed` / `agent_failed` / `agent_blocked`.

6. **Enforce write scopes**:
   - Each agent's write scope is declared in its frontmatter (`write_scope.allow` / `write_scope.deny`).
   - Before any file write, verify the path matches `allow` and does not match `deny`. If it doesn't, refuse and emit `agent_failed`.

7. **Serialize concurrent same-path writes**:
   - If two agent assignments target the same file path, dispatch them sequentially.

8. **Apply the failure policy**:
   - Default: a failed agent is marked blocked; siblings continue; the final report enumerates the blocker.
   - Strict (`--abort-on-failure`): halt pending agents on first failure.

9. **Write the final report** at `artifacts/ghu/run-<timestamp>-report.md` with frontmatter and the sections from `contracts/artifacts.md` §5.

10. **Emit `session_completed`** with `status: completed | failed`, `duration_ms`, `agents_summary`.

11. **Print** the run summary: tasks attempted/completed, files changed, tests added, findings, blockers, report path.

## Post-Execution

Run any `hooks.after_ghu`.

## Notes

- This is the only stage that mutates the source tree.
- Always emit events; the session log is how `bender sessions show/export` reconstructs the run.
- If a required tool is missing on PATH for a skill (per its `requires_tools`), emit `skill_failed` and continue with the next skill.
