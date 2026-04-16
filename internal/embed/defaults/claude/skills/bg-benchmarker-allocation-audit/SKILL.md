---
name: bg-benchmarker-allocation-audit
context: bg
description: "Audit allocation patterns for unnecessary heap pressure."
provides: [perf, alloc]
stages: [ghu, implement]
applies_to: [performance]
---

# bg-benchmarker-allocation-audit

Flag allocations inside hot loops; suggest pooling or reuse where applicable.
