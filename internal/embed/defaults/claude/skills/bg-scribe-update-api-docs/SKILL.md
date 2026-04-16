---
name: bg-scribe-update-api-docs
context: bg
description: "Update API docs (godoc, JSDoc, OpenAPI, etc.) for changed public surface."
provides: [scribe, api-docs]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scribe-update-api-docs

For each changed public symbol, update its doc comment / spec entry.
