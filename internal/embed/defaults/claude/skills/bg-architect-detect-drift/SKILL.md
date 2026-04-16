---
name: bg-architect-detect-drift
context: bg
description: "Detect drift between the implementation and the planned data model / API contract."
provides: [architect, drift]
stages: [ghu, implement]
applies_to: [architectural]
---

# bg-architect-detect-drift

Compare resolved types/endpoints to artifacts/plan/. Emit findings on undocumented additions.
