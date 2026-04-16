---
name: bg-linter-autofix
context: bg
description: "Apply safe autofixes only (formatting, import sorting)."
provides: [lint, autofix]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-autofix

Use the linter's --fix or formatter command. Refuse to apply semantic changes.
