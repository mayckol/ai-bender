---
name: fg-bootstrap-detect-build
context: fg
description: "Detect build tooling and containerization."
provides: [detect, build]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-build

Look for Makefile, Dockerfile, build scripts, language-specific build tools. Update Build / CI section.
