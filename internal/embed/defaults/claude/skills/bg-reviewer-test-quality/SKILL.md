---
name: bg-reviewer-test-quality
context: bg
description: "Critique the new tests authored this run."
provides: [review, tests]
stages: [ghu, implement]
applies_to: [any]
---

# bg-reviewer-test-quality

Check: coverage of acceptance criteria, edge cases, isolation, naming. Emit findings on gaps.
