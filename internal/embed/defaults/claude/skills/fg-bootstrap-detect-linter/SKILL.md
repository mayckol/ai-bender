---
name: fg-bootstrap-detect-linter
context: fg
description: "Detect linters, formatters, and pre-commit hooks."
provides: [detect, lint]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-linter

Look for .golangci.yml, .eslintrc.*, .prettierrc.*, .pre-commit-config.yaml. Update Lint section.
