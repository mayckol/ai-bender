---
name: fg-bootstrap-detect-structure
context: fg
description: "Detect the project's folder layout, entry points, and module boundaries."
provides: [detect, structure]
stages: [bootstrap]
applies_to: [any]
---

# fg-bootstrap-detect-structure

Walk top-level directories. Identify entry points (main.go, index.js, app.py, etc.). Update Structure section.
