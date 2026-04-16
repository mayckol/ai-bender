---
name: bg-tester-diff-coverage
context: bg
description: "Compute coverage on the diff (changed lines only)."
provides: [test, coverage]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-diff-coverage

Compare changed lines against the coverage report; flag any uncovered new code.
