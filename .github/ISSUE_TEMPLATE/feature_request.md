---
name: Feature request
about: Suggest an improvement
title: "[feat] "
labels: enhancement
---

## The problem

What are you trying to do that's hard or impossible today?

## Proposed change

What you'd like to see. Keep it focused on the smallest useful version.

## Constraints check

Please confirm this proposal does not conflict with any guardrail in
[`CLAUDE.md §11`](../CLAUDE.md). In particular:

- [ ] No Electron/Tauri/webview involvement
- [ ] Does not require loading a full chat into memory
- [ ] Does not require storing media bytes in SQLite
- [ ] Does not move DB / network / image decoding onto the UI goroutine
- [ ] Does not introduce `OFFSET` pagination
- [ ] No heavy new dependency without a footprint/perf justification
- [ ] Not a bulk-messaging / automation feature

If any box is unchecked, explain why the trade-off is worth it.

## Alternatives considered

Optional — anything you ruled out and why.
