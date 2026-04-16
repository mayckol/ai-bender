---
name: fg-bootstrap-detect-stack
context: fg
description: "Detect the project's primary language, package manager, and frameworks."
provides: [detect, stack]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-stack

Use file probes (manifests, lock files) and extension counts to identify the stack. Update `artifacts/constitution.md` Stack section.
