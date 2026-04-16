---
name: bg-tester-run-suite
context: bg
description: "Run the test suite and report failures."
provides: [test, run]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-run-suite

Use the project's test command (from constitution). Emit findings on failure with the failing test name + one-line diagnosis.
