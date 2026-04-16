---
name: cry
user-invocable: true
argument-hint: "<your request> — free-form description of a bug, feature, performance, or architectural change"
context: fg
description: "Capture intent — write a high-level capture artifact for a user request. Never designs, never implements."
provides: [stage, capture, intent]
stages: [cry]
applies_to: [any]
inputs: []
outputs:
  - .bender/artifacts/cry/<slug>-<timestamp>.md
---

# `/cry` — Capture Intent

Capture a user's request as a structured artifact. Classify the issue type, record verbatim, interpret functional requirements, propose a high-level direction, list open questions, and identify affected areas. Do **not** design or implement.

## User Input

```text
$ARGUMENTS
```

## Pre-Execution Checks

1. Check if `.specify/extensions.yml` exists with a `hooks.before_cry` entry. Honor `enabled` and `optional`. For mandatory hooks, run them first.

## Workflow

### If the user typed `/cry confirm`

1. Find the most recent draft artifact under `.bender/artifacts/cry/`.
2. Update its frontmatter `status: draft` → `status: approved`.
3. Print the path and suggest `/plan` as the next command.
4. Append `stage_completed` to `events.jsonl`.
5. Stop.

### Otherwise (new or refining capture)

1. **Parse the input**:
   - If the user passed `--type=<bug|feature|performance|architectural>`, use it.
   - Otherwise, classify the request into one of those four types based on keywords and intent.
   - Derive a kebab-case slug from the request title (≤80 chars, ASCII only).
   - Generate a timestamp: `YYYY-MM-DDTHH-MM-SS` (UTC, no colons).

2. **Find a predecessor**:
   - Look for the most recent existing capture artifact with the same slug.
   - If found, set `previous: .bender/artifacts/cry/<that-file>` in the new artifact's frontmatter.

3. **Create the session directory**:
   - `.bender/sessions/<timestamp>-<rand3>/`
   - Write `state.json` with `command: /cry, status: running, started_at: <iso>`.
   - Append a `session_started` event and a `stage_started` event to `events.jsonl`.

4. **Write the artifact** at `.bender/artifacts/cry/<slug>-<timestamp>.md`:

   ```markdown
   ---
   issue_type: <bug|feature|performance|architectural>
   status: draft
   created_at: <iso>
   slug: <slug>
   previous: <relative-path-or-omit>
   tool_version: <bender version>
   ---

   # Capture: <Title from request>

   ## Verbatim User Request
   > <verbatim>

   ## Interpreted Functional Requirements
   - …

   ## Proposed High-Level Direction
   …

   ## Open Questions
   - …

   ## Affected Areas
   - …
   ```

5. **Emit events** as you proceed — use the exact shapes in "Observability shape" below.
   Emit in this order: `session_started` → `stage_started` → one `skill_invoked` + matching
   `skill_completed` per sub-step (`classification`, `predecessor_lookup`, `drafting`) →
   `artifact_written` → `stage_completed` → `session_completed`.

6. **Update `state.json`** per the "Observability shape" section below.

7. **Print** the artifact path, the chosen issue type, and "next: `/cry confirm` to approve, or re-run `/cry` with more context".

## Observability shape — emit verbatim, do NOT invent fields

Every event is one JSON line in `.bender/sessions/<session_id>/events.jsonl` with this
exact top-level envelope. `session_id` is the directory name (e.g. `2026-04-16T19-16-21-758`).

```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "timestamp": "<iso8601 UTC, e.g. 2026-04-16T19:22:14Z>",
  "actor": {"kind": "<actor-kind>", "name": "<actor-name>"},
  "type": "<event-type>",
  "payload": { }
}
```

Valid event types for `/cry` (emit in order). Fill `timestamp` at emit time.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/cry","invoker":"<$USER>","working_dir":"<abs path>"}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"cry"},"type":"stage_started","payload":{"stage":"cry","inputs":[]}}
```

### skill_invoked (one per sub-step)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"cry"},"type":"skill_invoked","payload":{"skill":"classification","agent":"cry"}}
```

### skill_completed (one per sub-step, after the matching skill_invoked)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"cry"},"type":"skill_completed","payload":{"skill":"classification","agent":"cry","duration_ms":<int>,"outputs":[]}}
```

### artifact_written
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"cry"},"type":"artifact_written","payload":{"path":".bender/artifacts/cry/<slug>-<ts>.md","stage":"cry","checksum":"<sha256 hex lowercase>","bytes":<int>}}
```

### stage_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"cry"},"type":"stage_completed","payload":{"stage":"cry","inputs":[],"outputs":[".bender/artifacts/cry/<slug>-<ts>.md"]}}
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
  "command": "/cry",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": [],
  "skills_invoked": ["classification","predecessor_lookup","drafting"],
  "files_changed": 1,
  "findings_count": 0
}
```

### Forbidden shortcuts
- `ts` instead of `timestamp` — WRONG.
- `event` instead of `type` — WRONG.
- Payload fields at the top level instead of nested under `payload` — WRONG.
- Omitting `schema_version`, `session_id`, `actor`, or `payload` — WRONG.
- `session_resumed` — not a valid type. `/cry confirm` opens a **new** session with its own `session_id` and its own `session_started` + `session_completed`.
- A `kind` field on `artifact_written` — not in the contract; the artifact's own frontmatter already records `status: draft|approved`.
- Stage names other than `cry` — the stage is always `cry` for both initial and confirm runs.

## Post-Execution

Run any `hooks.after_cry` from `.specify/extensions.yml`.

### /cry confirm: emit this event sequence

Confirm is a fresh session with its own `session_id` (directory), NOT a resumption.

1. `session_started` (payload.command = `/cry confirm`)
2. `stage_started` (payload.stage = `cry`)
3. `artifact_written` (payload.path is the approved artifact, new sha256)
4. `stage_completed`
5. `session_completed`

Plus a state.json with `command: /cry confirm`, terminal `completed_at`, `source_artifacts: [<the draft path>]`.

## Notes

- Never write code, design data models, or decompose tasks here. Those are `/plan` and `/ghu`.
- Status MUST flip to `approved` only via explicit `/cry confirm`.
