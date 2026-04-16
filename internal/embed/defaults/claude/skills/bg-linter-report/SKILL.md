---
name: bg-linter-report
context: bg
description: "Emit findings for warnings/errors the linter cannot autofix. Group by severity. Cite file and line range. Never modifies code."
provides: [lint, report]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-report

Translate the linter's residual output (everything `bg-linter-run-and-fix` did not autofix) into structured findings the reviewer and PR summary can consume.

## Steps

1. Collect the linter's remaining warnings and errors after autofix.
2. **Group by severity**:
   - Linter errors → `low`-severity findings.
   - Linter warnings → `info`-severity findings.
3. **Cite precisely**: `file_path:line_start-line_end` plus the rule name (e.g., `errcheck`, `no-unused-vars`).
4. Emit one `finding_reported` event per finding. Severity calibration matters: do not flood the run report with `info` items.
5. **Never modify code**. Your write scope already prevents it; this is a reminder for the prompt.
