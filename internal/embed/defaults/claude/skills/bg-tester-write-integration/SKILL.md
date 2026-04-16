---
name: bg-tester-write-integration
context: bg
description: "Author integration tests for boundary-crossing code."
provides: [test, integration]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-write-integration

Use real adapters where the boundary lives (DB, HTTP, filesystem). Avoid mocks at the boundary itself.
