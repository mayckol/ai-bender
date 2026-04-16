---
name: bg-surgeon-rename-symbol
context: bg
description: "Rename a symbol across the codebase."
provides: [refactor, rename]
stages: [ghu, implement]
applies_to: [any]
---

# bg-surgeon-rename-symbol

Update every reference; run tests to verify behaviour preserved.
