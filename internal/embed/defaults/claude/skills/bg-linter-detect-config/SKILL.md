---
name: bg-linter-detect-config
context: bg
description: "Detect the project's lint configuration."
provides: [lint, config]
stages: [ghu, implement]
applies_to: [any]
---

# bg-linter-detect-config

Resolve linter from the constitution; verify config file exists; report config path.
