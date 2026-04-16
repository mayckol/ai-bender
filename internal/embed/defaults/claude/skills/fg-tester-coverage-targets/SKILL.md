---
name: fg-tester-coverage-targets
context: fg
description: "Compute the coverage target for the change set."
provides: [tdd, coverage]
stages: [tdd]
applies_to: [any]
---

# fg-tester-coverage-targets

Read the project's coverage tool (from constitution). Target ≥80% line coverage for new code; flag gaps.
