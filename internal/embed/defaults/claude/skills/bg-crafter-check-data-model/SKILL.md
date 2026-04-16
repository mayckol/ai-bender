---
name: bg-crafter-check-data-model
context: bg
description: "Verify that the implementation honors the planned data model."
provides: [check, data-model]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-check-data-model

Compare the implementation's types/structs/schemas to artifacts/plan/data-model-<ts>.md. Emit findings on divergence.
