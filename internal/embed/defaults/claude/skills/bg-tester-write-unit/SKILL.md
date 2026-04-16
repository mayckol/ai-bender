---
name: bg-tester-write-unit
context: bg
description: "Author unit tests for new code."
provides: [test, unit]
stages: [ghu, implement]
applies_to: [any]
---

# bg-tester-write-unit

One test file per production file under test. Cover acceptance criteria + edge cases enumerated by tester agent.
