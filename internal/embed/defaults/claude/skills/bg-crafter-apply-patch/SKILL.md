---
name: bg-crafter-apply-patch
context: bg
description: "Apply a minimum-diff patch implementing the task."
provides: [code-generation]
stages: [ghu, implement]
applies_to: [any]
---

# bg-crafter-apply-patch

Read the affected files, write the smallest change that satisfies the acceptance criteria. No refactors outside scope.
