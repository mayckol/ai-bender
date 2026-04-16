---
name: bg-benchmarker-detect-n-plus-one
context: bg
description: "Detect N+1 query patterns in DB code."
provides: [perf, n+1]
stages: [ghu, implement]
applies_to: [performance]
---

# bg-benchmarker-detect-n-plus-one

Trace queries inside loops; emit findings citing the loop + query.
