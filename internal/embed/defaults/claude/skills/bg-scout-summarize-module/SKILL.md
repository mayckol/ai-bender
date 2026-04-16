---
name: bg-scout-summarize-module
context: bg
description: "Summarise a module: file count, public API surface, inbound dependencies."
provides: [scout, summarize]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scout-summarize-module

Cache the summary; subsequent lookups in the same session are served from cache.
