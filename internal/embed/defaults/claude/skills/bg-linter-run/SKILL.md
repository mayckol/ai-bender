---
name: bg-linter-run
context: bg
description: "Run the configured linter against changed files (or the whole tree on demand)."
provides: [lint, run]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-run

Default scope: changed files. On --all: whole tree. Capture stdout/stderr.
