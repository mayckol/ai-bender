---
name: fg-bootstrap-detect-purpose
context: fg
description: "Extract the project's purpose from README, manifests, and top-level docs."
provides: [detect, purpose]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-purpose

Read README.md, package.json description, pyproject.toml description, etc. Summarise the purpose in one paragraph. Update Purpose section.
