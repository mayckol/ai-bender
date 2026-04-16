---
name: bg-crafter-check-api-contract
context: bg
description: "Verify that the implementation honors the planned API contract."
provides: [check, api]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-check-api-contract

Compare exposed endpoints / CLI surface / types to artifacts/plan/api-contract-<ts>.md. Emit findings on divergence.
