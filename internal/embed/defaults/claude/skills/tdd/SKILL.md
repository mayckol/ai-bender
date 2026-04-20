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

**Every** event MUST be appended to `.bender/sessions/<id>/events.jsonl` the moment its trigger happens — **one `bender event emit` call per event**, not a single `Write` at the end. The bender-ui viewer tails the file via fsnotify; batching collapses the timeline into a single notification and the user sees `Waiting for events…` for the full run.

```bash
bender event emit \
  --sessions-root "$SESSIONS_ROOT" \
  --session "$SESSION_ID" \
  --type skill_invoked \
  --actor-kind stage \
  --actor-name tdd \
  --payload '{"skill":"scaffold_enumerate","agent":"tester"}'
```

`$SESSIONS_ROOT` is the absolute path to the main repo's `.bender/sessions`. Fallback when the binary is unavailable: `.specify/scripts/bash/event-emit.sh` with the same flags. Do **not** use raw `printf >> events.jsonl`.

When `/tdd` runs as a prerequisite to `/ghu`, resolve a shared workflow id so both sessions render as one timeline:

```bash
WORKFLOW_KEY="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [[ -n "$WORKFLOW_KEY" ]] && command -v bender >/dev/null 2>&1; then
    WF_JSON="$(bender workflow resolve --key "$WORKFLOW_KEY" 2>/dev/null || true)"
fi
```

Pass the parsed `workflow_id` (and `parent_session_id`, when inheriting) through to whichever session-creation step this skill uses.

Intent events (`skill_invoked`) emit BEFORE the action; result events (`file_changed`, `artifact_written`, `skill_completed`, `stage_completed`, `session_completed`) emit AFTER. Never buffer events and flush them with one `Write`.

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
{"schema_version":1,"session_id":"<id>","timestamp":"<iso>","actor":{"kind":"orchestrator","name":"core"},"type":"session_completed","payload":{"status":"awaiting_confirm","duration_ms":<int>,"agents_summary":[]}}
```

A draft `/tdd` run emits `status: "awaiting_confirm"`; only `/tdd confirm`
emits `status: "completed"`.

### state.json (overwrite in place)
```json
{
  "schema_version": 1,
  "session_id": "<session_id>",
  "command": "/tdd",
  "started_at": "<iso>",
  "completed_at": "<iso, once terminal>",
  "status": "running|awaiting_confirm|completed|failed",
  "source_artifacts": [".bender/artifacts/plan/tasks-<ts>.md"],
  "skills_invoked": ["scaffold_enumerate","scaffold_<slug>", "..."],
  "files_changed": <int>,
  "findings_count": 0
}
```

A draft `/tdd` run finalises with `status: awaiting_confirm`. The subsequent
`/tdd confirm` session finalises with `status: completed`.

### Forbidden shortcuts
- `ts` / `event` / inlined payload fields / missing `schema_version|session_id|actor|payload` — all WRONG.
- Stage names other than `tdd` — WRONG.
- `kind` on `artifact_written` — WRONG.

## /tdd confirm: fresh session, emit:
1. `session_started` (command = `/tdd confirm`)
2. `stage_started` (stage = `tdd`)
3. one `artifact_written` per flipped scaffold (new sha256)
4. `stage_completed`
5. `session_completed` with `payload.status: "completed"`

The `/tdd confirm` session finalises state.json with `status: "completed"`.
The original `/tdd` draft session stays as `awaiting_confirm`.

## Notes

- Prose only. No executable code.
- Mirror the source tree faithfully so reviewers can read scaffolds alongside the code they describe.
