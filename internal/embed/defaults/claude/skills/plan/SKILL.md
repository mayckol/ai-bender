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

## Event emission discipline — STREAM, never batch

**Every** event listed in the "Observability shape" section below MUST be
appended to `.bender/sessions/<id>/events.jsonl` **the moment its trigger
happens** — one tool call per event, not one big `Write` at the end. The
bender-ui viewer tails the file via fsnotify; batching every event into a
single end-of-run write collapses the whole timeline into one notification
and the user sees `Waiting for events…` for the full duration of the run.

How to append a single event (use Bash, one call per event):

```bash
printf '%s\n' '<single-line JSON>' >> .bender/sessions/<id>/events.jsonl
```

Ordering rule:

- Emit BEFORE the action for intent events (`skill_invoked`,
  `orchestrator_decision`, `agent_started`).
- Emit AFTER the action for result events (`file_changed`,
  `artifact_written`, `skill_completed`, `stage_completed`,
  `session_completed`).
- Do NOT buffer events in memory and dump them via one `Write` — that
  is the bug this rule exists to prevent.

## Plan Set Production

1. **Generate one shared timestamp** for the entire plan set.

2. **Create the session directory** `.bender/sessions/<id>/` and write
   `state.json` (status: running).
   Append `session_started` to events.jsonl. Append `stage_started` to
   events.jsonl.

3. **Spec draft**:
   - Append `skill_invoked` for `spec_draft` BEFORE writing the spec.
   - Write `.bender/artifacts/specs/<slug>-<timestamp>.md` (frontmatter:
     `from_capture, status: draft, created_at, tool_version`; body: User
     Scenarios & Testing, Requirements, Success Criteria, Assumptions).
   - Append `artifact_written` for the spec immediately after.
   - Append `skill_completed` for `spec_draft` immediately after.

4. **Data model**:
   - Append `skill_invoked` for `data_model`.
   - Write `.bender/artifacts/plan/data-model-<timestamp>.md` (entities,
     fields, validation, relationships, state transitions).
   - Append `artifact_written`, then `skill_completed`.

5. **API contract** (only if the plan has an externally consumable interface):
   - Append `skill_invoked` for `api_contract`.
   - Write `.bender/artifacts/plan/api-contract-<timestamp>.md`.
   - Append `artifact_written`, then `skill_completed`.

6. **Risk assessment**:
   - Append `skill_invoked` for `risk_assessment`.
   - Write `.bender/artifacts/plan/risk-assessment-<timestamp>.md` (risks
     with severity × likelihood, mitigations, open risks).
   - Append `artifact_written`, then `skill_completed`.

7. **Task list**:
   - Append `skill_invoked` for `tasks_decompose`.
   - Write `.bender/artifacts/plan/tasks-<timestamp>.md` (T001+, title,
     description, agent_hints, depends_on, affected_files, acceptance).
   - Reject cyclic dependencies before writing.
   - Append `artifact_written`, then `skill_completed`.

8. **Finalize**: rewrite `state.json` with `status: awaiting_confirm` and
   `completed_at`. Append `stage_completed`. Append `session_completed` with
   `payload.status: "awaiting_confirm"`. The draft plan set is not truly
   complete until the user runs `/plan confirm`; only the confirm run flips
   the status to `completed`.

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
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"awaiting_confirm","duration_ms":<int>,"agents_summary":[]}}
```

A draft `/plan` run emits `status: "awaiting_confirm"` — the stage only reaches
`completed` once the user runs `/plan confirm` (the confirm run emits `status:
"completed"`).

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/plan",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|awaiting_confirm|completed|failed",
  "source_artifacts": [".bender/artifacts/cry/<slug>-<ts>.md"],
  "skills_invoked": ["spec_draft","data_model","api_contract","risk_assessment","tasks_decompose"],
  "files_changed": <int>,
  "findings_count": 0
}
```

A draft `/plan` run finalises with `status: awaiting_confirm`. The subsequent
`/plan confirm` session finalises with `status: completed`.

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
5. `session_completed` with `payload.status: "completed"`

state.json for the `/plan confirm` session finalises with `status:
"completed"`. The original `/plan` draft session stays as `awaiting_confirm`.

## Post-Execution

Run any `hooks.after_plan`.

## Notes

- All artifacts in the set MUST share the same timestamp.
- `/plan confirm` MUST flip the entire set atomically — all-or-nothing.
- Never write code or executable tests. Those belong in `/ghu`.
