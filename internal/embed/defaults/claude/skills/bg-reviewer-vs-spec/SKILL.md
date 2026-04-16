---
name: bg-reviewer-vs-spec
context: bg
description: "Verify changes match the approved spec."
provides: [review, spec]
stages: [ghu, implement]
applies_to: [any]
---

# bg-reviewer-vs-spec

Read each spec section; for each, check the implementation honors it. Emit findings citing spec section + code location.
