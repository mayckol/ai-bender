---
name: bg-scout-read-file
context: bg
description: "Read a file (or a line range) and cache the result."
provides: [scout, read]
stages: [ghu, implement]
applies_to: [any]
---

# bg-scout-read-file

Use Read; honor offset/limit if requested. Cache by file path + mtime.
