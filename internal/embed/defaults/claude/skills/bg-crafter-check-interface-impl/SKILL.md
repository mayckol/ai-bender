---
name: bg-crafter-check-interface-impl
context: bg
description: "Verify that interfaces are implemented by the right concrete types."
provides: [check, interface]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-check-interface-impl

For each declared interface, find implementing types and check method signatures match. Emit findings on partial implementations.
