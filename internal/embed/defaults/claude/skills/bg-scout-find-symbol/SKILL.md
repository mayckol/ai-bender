---
name: bg-scout-find-symbol
context: bg
description: "Find the definition of a symbol."
provides: [scout, find-symbol]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scout-find-symbol

Search by exact name in the index. Cache the result under artifacts/.bender/cache/symbol/<name>.
