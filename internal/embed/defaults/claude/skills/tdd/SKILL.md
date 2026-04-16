---
name: tdd
context: fg
description: "Optional — mirror the source tree under artifacts/plan/tests/ with prose-only test descriptions per source file. No executable code."
provides: [stage, tdd, scaffold]
stages: [tdd]
applies_to: [any]
inputs:
  - artifacts/plan/tasks-*.md
outputs:
  - artifacts/plan/tests/<source-path>-<timestamp>.md
---

# `/tdd` — Test Scaffolds (optional)

Mirror the source tree under `artifacts/plan/tests/`. For each source file that needs coverage, write a prose-only description of the test cases (names, preconditions, expected outcomes). **Do not write executable test code.**

## User Input

```text
$ARGUMENTS
```

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

3. **Identify source files** that the plan's tasks will touch (from the `affected_files` field of each task). Mirror those paths under `artifacts/plan/tests/<source-path>-<timestamp>.md`.

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

6. **Emit events** (`artifact_written` per scaffold, `stage_completed`).

7. **Print** the count of scaffolds produced and "next: `/tdd confirm`".

## Notes

- Prose only. No executable code.
- Mirror the source tree faithfully so reviewers can read scaffolds alongside the code they describe.
