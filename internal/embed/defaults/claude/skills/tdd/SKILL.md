---
name: tdd
user-invocable: true
argument-hint: "[scenario...] — optional seed scenarios to include in the test scaffolds"
context: fg
description: "Optional — mirror the source tree under .bender/artifacts/plan/tests/ with prose-only test descriptions per source file. No executable code."
provides: [stage, tdd, scaffold]
stages: [tdd]
applies_to: [any]
inputs:
  - .bender/artifacts/plan/tasks-*.md
outputs:
  - .bender/artifacts/plan/tests/<source-path>-<timestamp>.md
---

# `/tdd` — Test Scaffolds (optional)

Mirror the source tree under `.bender/artifacts/plan/tests/`. For each source file that needs coverage, write a prose-only description of the test cases (names, preconditions, expected outcomes). **Do not write executable test code.**

## User Input

```text
$ARGUMENTS
```

## Event emission discipline — STREAM, never batch

**Every** event MUST be appended to `.bender/sessions/<id>/events.jsonl` the moment its trigger happens — **one Bash tool call per event**, not a single `Write` at the end. The bender-ui viewer tails the file via fsnotify; batching collapses the timeline into a single notification and the user sees `Waiting for events…` for the full run.

```bash
printf '%s\n' '<single-line JSON>' >> .bender/sessions/<id>/events.jsonl
```

Intent events (`skill_invoked`) append BEFORE the action; result events (`file_changed`, `artifact_written`, `skill_completed`, `stage_completed`, `session_completed`) append AFTER. Never buffer events and flush them with one `Write`.

## Pre-Execution Checks

Run any `hooks.before_tdd`.

## Workflow

### If `/tdd confirm`

1. Find the most recent draft scaffold set.
2. Flip every scaffold's frontmatter `status: draft` → `status: approved`.
3. Print "next: `/ghu`".
4. Stop.

### Otherwise

1. **Resolve** the latest **approved** plan set. If missing, print an error and stop.

2. **Generate one shared timestamp** for the scaffold set.

3. **Identify source files** that the plan's tasks will touch (from the `affected_files` field of each task). Mirror those paths under `.bender/artifacts/plan/tests/<source-path>-<timestamp>.md`.

4. **For each scaffold**, write:

   ```markdown
   ---
   from_plan: <plan-timestamp>
   status: draft
   mirrors: <source-path>
   created_at: <iso>
   tool_version: <bender version>
   ---

   # Tests for `<source-path>`

   ## TC1 — <test name>
   - Preconditions: …
   - Steps: …
   - Expected outcome: …

   ## TC2 — …
   ```

5. **Use any user-provided seed scenarios** (positional args) verbatim plus add scenarios drawn from the plan's data model, risk assessment, and acceptance criteria.

6. **Emit events** — use the exact shapes in "Observability shape" below.
   Order: `session_started` → `stage_started` → `skill_invoked`/`skill_completed` pairs
   for `scaffold_enumerate` + one pair per source file (`scaffold_<slug>`) → one
   `artifact_written` per scaffold → `stage_completed` → `session_completed`.

7. **Print** the count of scaffolds produced and "next: `/tdd confirm`".

## Observability shape — emit verbatim, do NOT invent fields

Same envelope as `/cry` and `/plan`. Stage is **`tdd`** for every event.

### session_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"user","name":"claude-code"},"type":"session_started","payload":{"command":"/tdd","invoker":"<$USER>","working_dir":"<abs path>"}}
```

### stage_started
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"tdd"},"type":"stage_started","payload":{"stage":"tdd","inputs":[".bender/artifacts/plan/tasks-<ts>.md"]}}
```

### skill_invoked / skill_completed (per sub-step)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"tdd"},"type":"skill_invoked","payload":{"skill":"<step>","agent":"tester"}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"tdd"},"type":"skill_completed","payload":{"skill":"<step>","agent":"tester","duration_ms":<int>,"outputs":["<scaffold path>"]}}
```

### artifact_written (one per scaffold file)
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"tdd"},"type":"artifact_written","payload":{"path":".bender/artifacts/plan/tests/<source-path>-<ts>.md","stage":"tdd","checksum":"<sha256>","bytes":<int>}}
```

### stage_completed / session_completed
```json
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"stage","name":"tdd"},"type":"stage_completed","payload":{"stage":"tdd","inputs":[".bender/artifacts/plan/tasks-<ts>.md"],"outputs":["<scaffold paths...>"]}}
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"completed","duration_ms":<int>,"agents_summary":[]}}
```

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/tdd",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|completed|failed",
  "source_artifacts": [".bender/artifacts/plan/tasks-<ts>.md"],
  "skills_invoked": ["scaffold_enumerate","scaffold_<slug>", "..."],
  "files_changed": <int>,
  "findings_count": 0
}
```

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names other than `tdd` — WRONG.
- `kind` on `artifact_written` — WRONG.

## /tdd confirm: fresh session, emit:
1. `session_started` (command = `/tdd confirm`)
2. `stage_started` (stage = `tdd`)
3. one `artifact_written` per flipped scaffold (new sha256)
4. `stage_completed`
5. `session_completed`

## Notes

- Prose only. No executable code.
- Mirror the source tree faithfully so reviewers can read scaffolds alongside the code they describe.
