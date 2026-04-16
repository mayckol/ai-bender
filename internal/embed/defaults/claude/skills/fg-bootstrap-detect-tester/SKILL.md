---
name: fg-bootstrap-detect-tester
context: fg
description: "Detect the test framework, conventions, and coverage tooling."
provides: [detect, tests]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-tester

Inspect dev dependencies (package.json devDependencies, pyproject testing, go.mod tests). Update Tests section.
