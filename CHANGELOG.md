# Changelog

All notable changes to `wachat` are documented in this file.

The format is based on [Keep a Changelog][kac], and this project adheres
to [Semantic Versioning][semver].

## [Unreleased]

(nothing yet)

## [0.0.1] - 2026-05-23

The initial bootstrap snapshot. The CLAUDE.md §12 roadmap is fully
checked off; the binary boots, persists messages, paginates by keyset,
auto-loads older history on scroll, virtualizes both lists, and the
media-cache framework is ready to wire into the message bubble.

### Added

- **Project scaffolding**: MIT license, README, CONTRIBUTING, CoC, SECURITY,
  `.github/` issue + PR templates, Makefile, pre-commit hook shim
  (`scripts/pre-commit` + `scripts/install-hooks.sh`)
- **`internal/store`** (SQLite, pure-Go via `modernc.org/sqlite`):
  - WAL + `synchronous=NORMAL`; embedded `schema.sql` applied on every open
  - `Insert` with dedup on `wa_id`; `UpsertChat`; `MarkRead`
  - Keyset `PageOlder` with `(TS, ID)` cursor so tied timestamps are
    neither skipped nor duplicated
- **`internal/wa`** (whatsmeow boundary):
  - `Client` wrapper around `sqlstore` + `whatsmeow.Client` (verify
    signatures against godoc on bumps — see CLAUDE.md §3)
  - `Handler` with the persist → non-blocking channel send → notify
    pipeline; benchmarked at ~190 ns/op under back-pressure
  - `QRItem` re-exported so callers don't need to import whatsmeow
- **`internal/ui`**:
  - View-model `State` reducer with `LoadChats` / `SelectChat` /
    `LoadOlder` / `OnIncoming` / `MarkSelectedRead`
  - Gio two-pane layout (chat list ↔ messages), virtualized via
    `material.List`
  - Auto-page on scroll-near-end (leading-edge trigger)
- **`internal/media`**:
  - LRU `Cache` keyed by file path with refcounted Decode/Release
  - `Tracker.SetVisible` computes set deltas so cache calls match
    visibility 1:1
- **`cmd/bench`**: seeds N messages, prints store-open / heap / Sys /
  first-page / deep-page timings
- **`cmd/seed`**: 5-chat demo data for manual UI testing
- **Local-only quality gates**: `make check` (fmt + vet + tests),
  enforced by a `scripts/pre-commit` shim so hook edits don't need
  re-installation
- **Performance budget validated**: 100k messages → ~0.5 MB Go heap,
  ~16 MB `Sys` (RSS proxy), ~520 µs first keyset page, ~12 ms deep
  keyset page (cold pages). Memory is flat with history size.

### Constraints upheld (CLAUDE.md §2 / §11)

- No Electron / Tauri / webview / embedded browser
- No full-chat in-memory loads — keyset paging throughout
- No DB blobs for media — only paths
- `CGO_ENABLED=0` confirmed via `go version -m wachat`
- UI goroutine never receives DB writes from background goroutines

[unreleased]: https://github.com/jdtoon/wachat/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/jdtoon/wachat/releases/tag/v0.0.1
[kac]: https://keepachangelog.com/en/1.1.0/
[semver]: https://semver.org/spec/v2.0.0.html
