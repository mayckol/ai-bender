---
name: fg-tester-scaffold
context: fg
description: "Mirror the source tree under .bender/artifacts/plan/tests/ with prose-only test descriptions per source file. Drives /tdd."
provides: [tdd, scaffold, propose, coverage]
stages: [tdd]
applies_to: [any]
---

# fg-tester-scaffold

Drives the optional `/tdd` stage. Mirrors the source files the plan's tasks will touch and writes prose-only test case descriptions per file. **No executable test code is written here.**

## Steps

1. **Resolve the latest approved plan**. Read its `affected_files` glob per task.
2. **Mirror the source tree** under `.bender/artifacts/plan/tests/<source-path>-<ts>.md` for each file that needs coverage.
3. **Propose test cases** for each scaffold:
   - Per acceptance criterion in the source task: at least one positive case and one negative case.
   - Boundary conditions: nil/empty/zero/max/overflow inputs.
   - Concurrency hazards if the code is concurrent.
   - Error paths: every documented error return has a test.
4. **Apply the appropriate pattern** per case:
   - **Unit**: isolated, fast, no I/O. Document Preconditions / Steps / Expected outcome.
   - **Integration**: real adapters at boundaries (DB, HTTP, filesystem). Document Setup / Inputs / Expected behaviour.
   - **Contract**: producer/consumer compatibility. Document Producer / Consumer / Schema / Compatibility expectation.
   - **E2E**: full pipeline against a fixture environment. Document Workload / Environment / User-observable assertions.
5. **Compute coverage targets** from the constitution's coverage tool. Default: ≥80% line coverage for new code; flag gaps explicitly.
6. **Use any user-provided seed scenarios** (positional args to `/tdd`) verbatim, then add scenarios proposed from the plan.

## Output frontmatter (per scaffold)

```yaml
---
from_plan: <plan-timestamp>
status: draft
mirrors: <source-path>
created_at: <iso>
tool_version: <bender version>
---
```

## What you DO NOT do

- Write executable test code. That's `bg-tester-write-and-run` during `/ghu`.
- Implement the code under test. That's `bg-crafter-implement`.
