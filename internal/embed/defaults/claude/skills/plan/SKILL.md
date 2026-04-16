---
name: plan
context: fg
description: "Low-level design — produce a spec, data model, optional API contract, risk assessment, and decomposed task list under one shared timestamp."
provides: [stage, plan, design]
stages: [plan]
applies_to: [any]
inputs:
  - artifacts/cry/*.md
outputs:
  - artifacts/specs/<slug>-<timestamp>.md
  - artifacts/plan/data-model-<timestamp>.md
  - artifacts/plan/api-contract-<timestamp>.md
  - artifacts/plan/risk-assessment-<timestamp>.md
  - artifacts/plan/tasks-<timestamp>.md
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

1. Find the most recent **approved** capture artifact under `artifacts/cry/`. If none exists, print:
   - `error: no approved capture artifact found. Run \`/cry "<your request>"\` and \`/cry confirm\` first.`
   - Exit. Do **not** create empty artifacts.

## Plan Set Production

1. **Generate one shared timestamp** for the entire plan set.

2. **Create the session directory** and emit `session_started` + `stage_started`.

3. **Author the spec** at `artifacts/specs/<slug>-<timestamp>.md`:
   - Frontmatter: `from_capture, status: draft, created_at, tool_version`.
   - Body: User Scenarios & Testing, Requirements (functional + key entities), Success Criteria, Assumptions.

4. **Author the data model** at `artifacts/plan/data-model-<timestamp>.md`:
   - Entities, fields, validation rules, relationships, state transitions.

5. **Author the API contract** at `artifacts/plan/api-contract-<timestamp>.md` *(only if the plan involves an externally consumable interface)*:
   - Endpoints / CLI grammar / GraphQL types / etc., as appropriate.

6. **Author the risk assessment** at `artifacts/plan/risk-assessment-<timestamp>.md`:
   - Risks (severity × likelihood), mitigations, open risks.

7. **Author the task list** at `artifacts/plan/tasks-<timestamp>.md`:
   - One section per task: id (T001+), title, description, agent_hints, depends_on, affected_files, acceptance.
   - Reject cyclic dependencies before writing.

8. **Emit events** for each artifact written (`artifact_written` with sha256 + byte count).

9. **Print** every artifact path produced and "next: `/plan confirm`".

## Post-Execution

Run any `hooks.after_plan`.

## Notes

- All artifacts in the set MUST share the same timestamp.
- `/plan confirm` MUST flip the entire set atomically — all-or-nothing.
- Never write code or executable tests. Those belong in `/ghu`.
