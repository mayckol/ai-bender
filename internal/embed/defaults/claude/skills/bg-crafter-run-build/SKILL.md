---
name: bg-crafter-run-build
context: bg
description: "Run the project's build to verify the change compiles."
provides: [code-generation]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-run-build

Use the build tool from the constitution (e.g., go build, npm run build, cargo build). Emit a finding on failure.
