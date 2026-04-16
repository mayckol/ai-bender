---
name: plan
user-invocable: true
argument-hint: "[--from=<cry-artifact>] — produces a plan set from the latest approved capture (or the one you pass)"
context: fg
description: "Low-level design — produce a spec, data model, optional API contract, risk assessment, and decomposed task list under one shared timestamp."
provides: [stage, plan, design]
stages: [plan]
applies_to: [any]
inputs:
  - .bender/artifacts/cry/*.md
outputs:
  - .bender/artifacts/specs/<slug>-<timestamp>.md
  - .bender/artifacts/plan/data-model-<timestamp>.md
  - .bender/artifacts/plan/api-contract-<timestamp>.md
  - .bender/artifacts/plan/risk-assessment-<timestamp>.md
  - .bender/artifacts/plan/tasks-<timestamp>.md
---

# `/plan` — Produce a Plan Set

From the latest approved capture artifact, produce a coherent plan set under a single shared timestamp. **No code, no tests.**

## User Input

```text
$ARGUMENTS
```

## Pre-Execution Checks

Run any `hooks.before_plan` from `.specify/extensions.yml`.

## Workflow

### If the user typed `/plan confirm`

1. Find the most recent draft plan set (all artifacts share one timestamp).
2. Atomically flip every artifact in the set from `status: draft` → `status: approved`.
3. Print "next: `/tdd` (optional) or `/ghu`".
4. Stop.

### If the user passed `--from=<cry-artifact>`

Use that artifact as the source.

### Otherwise

1. Find the most recent **approved** capture artifact under `.bender/artifacts/cry/`. If none exists, print:
   - `error: no approved capture artifact found. Run \`/cry "<your request>"\` and \`/cry confirm\` first.`
   - Exit. Do **not** create empty artifacts.

## Plan Set Production

1. **Generate one shared timestamp** for the entire plan set.

2. **Create the session directory** and emit `session_started` + `stage_started`.

3. **Author the spec** at `.bender/artifacts/specs/<slug>-<timestamp>.md`:
   - Frontmatter: `from_capture, status: draft, created_at, tool_version`.
   - Body: User Scenarios & Testing, Requirements (functional + key entities), Success Criteria, Assumptions.

4. **Author the data model** at `.bender/artifacts/plan/data-model-<timestamp>.md`:
   - Entities, fields, validation rules, relationships, state transitions.

5. **Author the API contract** at `.bender/artifacts/plan/api-contract-<timestamp>.md` *(only if the plan involves an externally consumable interface)*:
   - Endpoints / CLI grammar / GraphQL types / etc., as appropriate.

6. **Author the risk assessment** at `.bender/artifacts/plan/risk-assessment-<timestamp>.md`:
   - Risks (severity × likelihood), mitigations, open risks.

7. **Author the task list** at `.bender/artifacts/plan/tasks-<timestamp>.md`:
   - One section per task: id (T001+), title, description, agent_hints, depends_on, affected_files, acceptance.
   - Reject cyclic dependencies before writing.

8. **Emit events** for each artifact written — use the exact shapes in "Observability shape" below.
   Order: `session_started` → `stage_started` → one `skill_invoked` + matching `skill_completed` per
   sub-step (`spec_draft`, `data_model`, `api_contract` if applicable, `risk_assessment`,
   `tasks_decompose`) → one `artifact_written` per artifact produced (5 total, or 4 if no API
   contract) → `stage_completed` → `session_completed`.

9. **Print** every artifact path produced and "next: `/plan confirm`".

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as `/cry`. Stage is **`plan`** for every stage/skill/artifact event. The session_id is the directory name (e.g. `2026-04-16T19-22-14-770`).

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/plan","invoker":"<$USER>","working_dir":"<abs path>"}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"plan"},"type":"stage_started","payload":{"stage":"plan","inputs":[".bender/artifacts/cry/<slug>-<ts>.md"]}}
```

### skill_invoked / skill_completed (one pair per sub-step: spec_draft, data_model, api_contract, risk_assessment, tasks_decompose)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"plan"},"type":"skill_invoked","payload":{"skill":"<step>","agent":"architect"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"plan"},"type":"skill_completed","payload":{"skill":"<step>","agent":"architect","duration_ms":<int>,"outputs":["<artifact path>"]}}
```

### artifact_written (one per plan artifact)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"plan"},"type":"artifact_written","payload":{"path":"<repo-relative>","stage":"plan","checksum":"<sha256>","bytes":<int>}}
```

### stage_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"plan"},"type":"stage_completed","payload":{"stage":"plan","inputs":[".bender/artifacts/cry/<slug>-<ts>.md"],"outputs":["<spec path>","<data-model path>","<api-contract path or omit>","<risk-assessment path>","<tasks path>"]}}
```

### session_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed","duration_ms":<int>,"agents_summary":[]}}
```

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/plan",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": [".bender/artifacts/cry/<slug>-<ts>.md"],
  "skills_invoked": ["spec_draft","data_model","api_contract","risk_assessment","tasks_decompose"],
  "files_changed": <int>,
  "findings_count": 0
}
```

### Forbidden shortcuts
- `ts` instead of `timestamp`; `event` instead of `type`; payload fields inlined at the top level — all WRONG.
- Stage names like `plan_set` — WRONG. Use `plan`.
- `kind` field on `artifact_written` — WRONG. The contract is `{path, stage, checksum, bytes}`.
- Missing `schema_version`, `session_id`, `actor`, or `payload` — WRONG.

## /plan confirm: emit this event sequence

A fresh session with its own `session_id`:
1. `session_started` (payload.command = `/plan confirm`)
2. `stage_started` (payload.stage = `plan`, payload.inputs = the 5 draft paths)
3. 5 × `artifact_written` (one per plan-set file, new sha256 since `status: draft` → `approved`)
4. `stage_completed`
5. `session_completed`

## Post-Execution

Run any `hooks.after_plan`.

## Notes

- All artifacts in the set MUST share the same timestamp.
- `/plan confirm` MUST flip the entire set atomically — all-or-nothing.
- Never write code or executable tests. Those belong in `/ghu`.
