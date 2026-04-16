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
  - artifacts/cry/<slug>-<timestamp>.md
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

1. Find the most recent draft artifact under `artifacts/cry/`.
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
   - If found, set `previous: artifacts/cry/<that-file>` in the new artifact's frontmatter.

3. **Create the session directory**:
   - `artifacts/.bender/sessions/<timestamp>-<rand3>/`
   - Write `state.json` with `command: /cry, status: running, started_at: <iso>`.
   - Append a `session_started` event and a `stage_started` event to `events.jsonl`.

4. **Write the artifact** at `artifacts/cry/<slug>-<timestamp>.md`:

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

5. **Emit events** as you proceed:
   - `skill_invoked` for each sub-step (classification, drafting).
   - `artifact_written` when the artifact lands on disk (with sha256 + byte count).
   - `stage_completed` and `session_completed` at the end.

6. **Update `state.json`** with `status: completed`.

7. **Print** the artifact path, the chosen issue type, and "next: `/cry confirm` to approve, or re-run `/cry` with more context".

## Post-Execution

Run any `hooks.after_cry` from `.specify/extensions.yml`.

## Notes

- Never write code, design data models, or decompose tasks here. Those are `/plan` and `/ghu`.
- Status MUST flip to `approved` only via explicit `/cry confirm`.
