---
name: fg-cry-classify-issue
context: fg
description: "Classify a free-form request into bug | feature | performance | architectural."
provides: [classify]
stages: [cry]
applies_to: [any]
---

# fg-cry-classify-issue

Pick the single best label based on the request. Default to `feature` when ambiguous.
