---
name: bg-surgeon-verify-behavior
context: bg
description: "Verify a refactor preserves observable behaviour by running the test suite before and after. Refuses if tests are red on entry."
provides: [refactor, verify]
stages: [ghu, implement]
applies_to: [any]
---

# bg-surgeon-verify-behavior

The verification half of `bg-surgeon-refactor`. Always invoked around any refactor.

## Steps

1. **Before**: run the test suite using the constitution's test command. Record the pass/fail set.
   - If any test is red, **refuse the refactor**. Emit `high`-severity `finding_reported` directing the orchestrator to fix tests first. Stop.
2. **After**: run the test suite again with the refactor applied.
3. **Compare pass/fail sets**:
   - Identical → success. Emit `skill_completed`.
   - Different → emit `high`-severity finding describing which tests changed status, then revert the refactor.

## Output

Write the before/after test summaries to `artifacts/ghu/perf/<run-timestamp>/refactor-verify.md` (re-using the perf folder since it's already where structural-quality artifacts live; if a separate `artifacts/ghu/refactor/` is preferred in your project, override via the agent's write_scope).

## Why this is its own skill

Decoupling the verify pass from the refactor means the orchestrator can call it independently — for example, after a `crafter` patch in code that happened to refactor incidentally, to confirm the test set is still identical to baseline.
