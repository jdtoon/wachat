# Changelog

All notable changes to `wachat` are documented in this file.

The format is based on [Keep a Changelog][kac], and this project adheres
to [Semantic Versioning][semver].

## [Unreleased]

(nothing yet)

## [0.0.5] - 2026-05-23

### Added

- **Full-text search** via SQLite FTS5. `messages_fts` is a content-
  rowid'd virtual table over `messages`; AI/AD/AU triggers keep the
  index in sync. unicode61 tokenizer with `remove_diacritics 2` so
  `cafe` matches `café`.
- **`store.Search(ctx, query, limit)`** returns ranked `SearchHit`s
  with `snippet()` highlighting (FTS5 emits `[[…]]` tokens around
  matches).
- **`store.PageAround(ctx, chatJID, anchorID, before, after)`** loads
  a window centered on a message — used by jump-to-message. Pure
  keyset, no `OFFSET`.
- **Search bar** widget in the sidebar header. Enter submits;
  inline-styled overlay swaps in for the chat list when a search is
  active.
- **`State.Search`** and **`State.JumpToMessage`** plus
  `ViewCallbacks.OnSearch` / `OnJumpToMessage` for the wiring.
- Backfill on `store.Open` re-indexes any v0.0.4-era DBs whose
  messages predate the FTS5 table.

### Notes

- FTS5 is verified present in our `modernc.org/sqlite` build.
- Snippet rendering currently strips the `[[…]]` markers and renders
  as plain TextSecondary; richer inline accent coloring waits on
  Gio's text shaper supporting inline color runs.

[0.0.5]: https://github.com/jdtoon/wachat/releases/tag/v0.0.5

## [0.0.4] - 2026-05-23

### Added

- **`wa.SendText(ctx, chatJID, body)`**: thin wrapper around
  whatsmeow's `SendMessage` that returns the server-assigned message
  ID. Signatures verified against `pkg.go.dev/go.mau.fi/whatsmeow`.
- **`wa.OwnJID()`**: nil-safe accessor for the paired device's JID.
  Populates `State.OwnJID` so bubble alignment switches off the real
  comparison once paired.
- **`ui.Composer`** widget: multiline `widget.Editor` + send button.
  Enter sends; Shift+Enter inserts a newline. Trim + clear on submit.
- **Conversation pane layout**: header (chat name strip) ↓ messages
  (flexed) ↓ composer. Surface-colored composer with a top divider.
- **`State.AddOptimistic(ctx, waID, chatJID, body, ts)`**: persists +
  folds into the view-model so an outgoing bubble appears
  immediately. Reuses the existing WAID dedup path.
- **`ViewCallbacks.OnSend(chatJID, body)`**: the new request surface
  the caller wires to send + optimistic insert.

### Known follow-up

The optimistic `waID` is currently a placeholder (`"optimistic-<ts>"`),
so a redelivered receipt with the server-assigned ID will briefly
double-bubble before dedup converges. A v0.0.4.x patch will mint
`whatsmeow.GenerateMessageID` and pass it as `SendRequestExtra.ID`
so the IDs match on first arrival.

[0.0.4]: https://github.com/jdtoon/wachat/releases/tag/v0.0.4

## [0.0.3] - 2026-05-23

### Added

- **Message bubbles** (`internal/ui/bubble.go`): rounded sent-right /
  recv-left bubbles using `Theme.BubbleSent` / `Theme.BubbleRecv`,
  width capped at 70% of pane, HH:MM meta row inside the bubble.
- **Bubble grouping** (`internal/ui/grouping.go`): pure `GroupMessages`
  classifies each message as Solo / Head / Middle / Tail based on
  sender + 5-minute window. Used to tighten vertical spacing within
  a sender run.
- **Chat-list row redesign** (`internal/ui/chatrow.go`): circular
  avatar with deterministic-hue tint + initial, two-line name +
  subtitle, trailing time + unread badge.
- **Newest-at-bottom message ordering**: messages render with
  `Messages[count-1-i]` mapping so the newest sits at the bottom of
  the pane (WhatsApp Web convention). `widget.List.ScrollToEnd = true`
  anchors the view.
- `State.OwnJID` so bubble alignment can switch on the real device JID
  once v0.0.6 pairs.

### Changed

- `isNearEnd` → `isNearOldestLoaded`: the paging trigger flips to
  "user scrolled near the top of the loaded buffer", matching the new
  newest-at-bottom orientation.
- `checkPagingTrigger` extracted as a method so the leading-edge
  trigger is unit-testable against synthetic positions (real
  `View.Layout` overwrites `msgList.Position` from the layout pass).

[0.0.3]: https://github.com/jdtoon/wachat/releases/tag/v0.0.3

## [0.0.2] - 2026-05-23

### Added

- **Design system** (`internal/ui/theme.go` + light/dark palettes). All
  widgets now consult typed `Theme.Palette`, `Theme.Spacing`,
  `Theme.Type`, `Theme.Radius`, `Theme.Motion`, `Theme.Density` —
  raw color/spacing literals removed from `view.go`.
- **Public Sans** (OFL) embedded in three weights via `//go:embed`. Go
  fonts kept as emoji/script fallback. Adds ~250 KB to the binary.
- **WCAG AA contrast tested**: body text on every relevant background
  ≥ 4.5:1; button labels on accent ≥ 3:1.
- `docs/wiki/Theming.md` documenting the token model.

### Internal

- `Theme.Duration(d)` is the single hook for reduced-motion respect.
- `Theme.RowPad()` derives row padding from `Density`.

[0.0.2]: https://github.com/jdtoon/wachat/releases/tag/v0.0.2

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

[unreleased]: https://github.com/jdtoon/wachat/compare/v0.0.5...HEAD
[0.0.1]: https://github.com/jdtoon/wachat/releases/tag/v0.0.1
[kac]: https://keepachangelog.com/en/1.1.0/
[semver]: https://semver.org/spec/v2.0.0.html
