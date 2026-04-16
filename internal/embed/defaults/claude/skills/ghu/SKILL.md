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
  - .bender/artifacts/specs/*.md
  - .bender/artifacts/plan/tasks-*.md
outputs:
  - .bender/artifacts/ghu/run-<timestamp>-report.md
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
   - The latest **approved** spec at `.bender/artifacts/specs/<slug>-<ts>.md`.
   - The matching task list at `.bender/artifacts/plan/tasks-<ts>.md`.
   - Optionally, test scaffolds at `.bender/artifacts/plan/tests/`.
   - If anything required is missing, print:
     - `error: missing required upstream artifact: <name>. Run /plan and /plan confirm first.`
     - Exit. Do **not** create a partial session.

2. **Honor `--only=<task>`** to scope to a single task (same as `/implement`).

3. **Honor `--abort-on-failure`** to halt on the first task failure (default: continue and mark blocked).

4. **Create a session directory** under `.bender/sessions/<id>/`. Write `state.json` and append `session_started`, `stage_started`, `orchestrator_decision` (with the task decomposition).

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

9. **Write the final report** at `.bender/artifacts/ghu/run-<timestamp>-report.md` with frontmatter and the sections from `contracts/artifacts.md` §5.

10. **Emit `session_completed`** with `status: completed | failed`, `duration_ms`, `agents_summary`.

11. **Print** the run summary: tasks attempted/completed, files changed, tests added, findings, blockers, report path.

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as `/cry`, `/plan`, `/tdd`. Stage is **`ghu`** for every event.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/ghu","invoker":"<$USER>","working_dir":"<abs path>","registered_projects":[],"parallelism":1}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"stage_started","payload":{"stage":"ghu","inputs":[".bender/artifacts/specs/<slug>-<ts>.md",".bender/artifacts/plan/tasks-<ts>.md"]}}
```

### orchestrator_decision (emit per task_decomposition, agent_assignment, graph_node_transition)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"task_decomposition","tasks":["T001","T002","..."]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"agent_assignment","agent":"crafter"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"orchestrator_decision","payload":{"decision_type":"graph_node_transition","from_node":"scout","to_node":"architect"}}
```

### agent_started / agent_progress / agent_completed (or agent_failed / agent_blocked)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_started","payload":{"agent":"crafter","task_ids":["T004"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_progress","payload":{"percent":42,"current_step":"applying patch","completed":["read","plan"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_completed","payload":{"agent":"crafter","task_ids":["T004"],"duration_ms":<int>}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_failed","payload":{"agent":"crafter","task_ids":["T004"],"duration_ms":<int>,"error":"<human-readable reason>"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"agent_blocked","payload":{"agent":"crafter","task_ids":["T004"],"error":"blocked by finding from sentinel"}}
```

### skill_invoked / skill_completed / skill_failed (inside each agent)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_invoked","payload":{"skill":"bg-crafter-implement","agent":"crafter"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_completed","payload":{"skill":"bg-crafter-implement","agent":"crafter","duration_ms":<int>,"outputs":["pkg/foo/bar.go"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"skill_failed","payload":{"skill":"bg-crafter-verify-build","agent":"crafter","duration_ms":<int>,"error":"<stderr summary>"}}
```

### file_changed (one per modified file)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"crafter"},"type":"file_changed","payload":{"path":"pkg/foo/bar.go","lines_added":42,"lines_removed":7,"agent":"crafter"}}
```

### finding_reported (from reviewer/sentinel/benchmarker)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"agent","name":"reviewer"},"type":"finding_reported","payload":{"finding_id":"R-001","severity":"medium","category":"review","title":"<one line>","description":"<full>","location":{"path":"pkg/foo/bar.go","line_start":12,"line_end":18}}}
```

### artifact_written (final report + per-agent outputs)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"artifact_written","payload":{"path":".bender/artifacts/ghu/run-<ts>-report.md","stage":"ghu","checksum":"<sha256>","bytes":<int>}}
```

### stage_completed / session_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"ghu"},"type":"stage_completed","payload":{"stage":"ghu","inputs":["<spec path>","<tasks path>"],"outputs":["<run report path>"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed","duration_ms":<int>,"agents_summary":[{"agent":"crafter","status":"completed"},{"agent":"tester","status":"completed"}]}}
```

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/ghu",
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
- Stage names other than `ghu` — WRONG.
- `kind` on `artifact_written` — WRONG.
- `session_resumed` — WRONG (always fresh sessions).

## Post-Execution

Run any `hooks.after_ghu`.

## Notes

- This is the only stage that mutates the source tree.
- Always emit events; the session log is how `bender sessions show/export` reconstructs the run.
- If a required tool is missing on PATH for a skill (per its `requires_tools`), emit `skill_failed` and continue with the next skill.
