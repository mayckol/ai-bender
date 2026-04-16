---
name: bg-crafter-run-generate
context: bg
description: "Run code generation (go generate, codegen scripts) when the change requires it."
provides: [code-generation]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-run-generate

Detect generators from the constitution / Makefile. Run them; verify generated outputs are committed.
