# wachat — wiki

A lean, native desktop WhatsApp client written in Go. **No Electron, no webview.** This wiki collects design notes, runbooks, and FAQs that don't belong in the [README](https://github.com/jdtoon/wachat) but are still worth writing down.

> **v0.1.0** shipped — see the [release notes](https://github.com/jdtoon/wachat/releases/tag/v0.1.0). Phase 1 of `docs/roadmap.md` is complete: search, send, in-window pairing, dark mode, narrow-window collapse, and a flat-heap perf profile.

## Quick links

- [Project README](https://github.com/jdtoon/wachat#readme) — what it is, how to build & run
- [`CLAUDE.md`](https://github.com/jdtoon/wachat/blob/main/CLAUDE.md) — architecture, non-negotiable constraints, performance levers
- [`CHANGELOG.md`](https://github.com/jdtoon/wachat/blob/main/CHANGELOG.md) — what landed in each version
- [Latest release](https://github.com/jdtoon/wachat/releases/latest)
- [Issue tracker](https://github.com/jdtoon/wachat/issues)

## Pages

- [Getting Started](./Getting-Started) — first run, pairing, what to expect
- [Architecture](./Architecture) — one-process design + the wa / store / ui boundaries
- [Performance Notes](./Performance-Notes) — the budget, how it's measured, what flat-heap means
- [FAQ](./FAQ) — common questions, ToS / ban risk caveats

## Contributing

The README's [Contributing section](https://github.com/jdtoon/wachat/blob/main/CONTRIBUTING.md) is the canonical doc. The TL;DR: `make hooks && make check`. Don't add CI, don't add Electron, don't load full chats into memory.
