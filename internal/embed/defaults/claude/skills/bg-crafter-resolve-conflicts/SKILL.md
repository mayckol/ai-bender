---
name: bg-crafter-resolve-conflicts
context: bg
description: "Resolve merge or write-scope conflicts when multiple tasks touch overlapping files."
provides: [code-generation, conflicts]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-resolve-conflicts

Sequence writes to overlapping paths; emit findings if a true logical conflict (not just textual) is detected.
