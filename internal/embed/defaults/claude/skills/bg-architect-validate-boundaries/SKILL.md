---
name: bg-architect-validate-boundaries
context: bg
description: "Validate that the implementation respects the planned boundaries."
provides: [architect, validate]
stages: [ghu, implement]
applies_to: [architectural]
---

# bg-architect-validate-boundaries

Re-walk imports during /ghu; flag any new edges that cross boundaries the plan said to keep separate.
