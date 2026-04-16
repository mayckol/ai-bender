---
name: bg-linter-report-unfixable
context: bg
description: "Emit findings for warnings/errors the linter cannot autofix."
provides: [lint, report]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-report-unfixable

Group by severity; cite file path + line range; do not modify code.
