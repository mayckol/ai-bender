---
name: bg-surgeon-verify-behavior-preserved
context: bg
description: "Verify a refactor preserves observable behaviour by running tests before and after."
provides: [refactor, verify]
stages: [ghu, implement]
applies_to: [any]
---

# bg-surgeon-verify-behavior-preserved

Run the test suite before the refactor (must be green); run again after; require identical pass/fail set.
