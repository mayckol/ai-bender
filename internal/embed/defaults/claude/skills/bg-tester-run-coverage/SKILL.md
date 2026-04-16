---
name: bg-tester-run-coverage
context: bg
description: "Run the project's coverage tool and report numbers."
provides: [test, coverage]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-run-coverage

Use the coverage tool from the constitution. Report total + per-package coverage.
