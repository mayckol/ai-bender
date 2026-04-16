---
name: bg-benchmarker-find-hotpaths
context: bg
description: "Identify likely hot paths in the changed code."
provides: [perf, hotpaths]
stages: [ghu, implement]
applies_to: [performance]
---

# bg-benchmarker-find-hotpaths

Inspect loops, allocations, sync primitives. Mark each as info-severity "needs measurement" if not directly measurable.
