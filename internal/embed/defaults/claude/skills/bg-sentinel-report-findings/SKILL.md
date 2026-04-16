---
name: bg-sentinel-report-findings
context: bg
description: "Aggregate sentinel findings into artifacts/ghu/security/<ts>/."
provides: [security, report]
stages: [ghu, implement]
applies_to: [any]
---

# bg-sentinel-report-findings

One file per finding (id-based). Final summary in the run report.
