---
name: bg-surgeon-consolidate-duplication
context: bg
description: "Consolidate duplicated code into a shared helper."
provides: [refactor, consolidate]
stages: [ghu, implement]
applies_to: [any]
---

# bg-surgeon-consolidate-duplication

Identify the duplication; verify the helper preserves behaviour for every caller.
