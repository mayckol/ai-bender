---
name: bg-linter-format
context: bg
description: "Run the project's formatter (gofmt, prettier, ruff format, rustfmt, etc.)."
provides: [lint, format]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-format

Run with no special flags; commit only formatting changes.
