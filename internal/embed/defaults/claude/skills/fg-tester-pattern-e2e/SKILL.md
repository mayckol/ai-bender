---
name: fg-tester-pattern-e2e
context: fg
description: "Apply the end-to-end pattern: full pipeline against a fixture environment."
provides: [tdd, e2e]
stages: [tdd]
applies_to: [any]
---

# fg-tester-pattern-e2e

Document each e2e case as: Workload / Environment / Assertions on user-observable outcomes.
