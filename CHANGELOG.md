# Changelog

All notable changes to `wachat` are documented in this file.

The format is based on [Keep a Changelog][kac], and this project adheres
to [Semantic Versioning][semver].

## [Unreleased]

(nothing yet)

## [0.1.8] - 2026-05-23

### Added

- **Image messages** are extracted, persisted as thumbnails to
  `media/<waID>.jpg`, and rendered inside bubbles via the existing
  `media.Cache` + `media.Tracker` decode-on-visible pipeline. LRU
  budget of 30 MB caps the worst-case decoded-RAM footprint.
- Media-type pill (📷 Photo / 🎬 Video / 🎙 Voice / 📄 Document /
  🎴 Sticker / 📎 Attachment) for messages without a decodable
  thumbnail yet.
- `wa.DownloadImage` for the future "click thumbnail" full-res
  viewer (not yet wired into UI).
- `wa.SetMediaDir` / `wa.MediaDir` to control the on-disk root.

### Follow-ups

- Aspect-preserving thumbnail scaling (currently native size).
- Full-resolution viewer on tap.
- History-sync image rehydration (older messages don't carry the
  embedded thumbnail; we'd need to re-download).
- Voice notes / documents / link previews (v0.1.9).

[0.1.8]: https://github.com/jdtoon/wachat/releases/tag/v0.1.8

## [0.1.7] - 2026-05-23

### Added

- **Pin chats.** Pinned conversations sort to the top of the chat
  list with a 📌 prefix. Driven by `events.Pin`.
- **Mute chats.** 🔇 prefix on the chat name; `ChatSummary.IsMuted`
  helper for the future notifications path.
- **Archive chats.** Archived conversations are filtered from the
  main chat list (dedicated archive view is a follow-up).
- **Typing indicator.** "Alice is typing…" / "X and Y are typing…"
  appears above the composer for the active chat. Per-chat TTL
  (`TypingTTL = 10s`) so a missed "paused" event still clears the
  line.

### Internal

- New `chats.pinned`, `chats.archived`, `chats.mute_until` columns
  with transparent migration.
- `store.SetPinned` / `SetArchived` / `SetMuteUntil`.
- `wa.Handler.OnTyping` callback + `applyPin` / `applyMute` /
  `applyArchive` event handlers.
- `ChatStateStore` interface for the wa-side store contract.

Mentions rendering and presence/last-seen stay on the v0.1.7
checklist and land in a follow-up so this patch ships focused.

[0.1.7]: https://github.com/jdtoon/wachat/releases/tag/v0.1.7

## [0.1.6] - 2026-05-23

### Added

- **Emoji reactions** (receive + render). A reaction chip cluster
  appears below the bubble meta row, e.g. `👍 3 · ❤ 2`. Inbound
  `ReactionMessage` events update the `reactions` table; the bubble
  renders from `State.Reactions[waID]` populated by the
  `ReactionsForChat` batch fetch.
- **`wa.SendReaction(ctx, chatJID, targetSender, targetWAID, emoji)`**
  using `whatsmeow.BuildReaction`. Pass `""` to remove a reaction.
  Programmatic for now — the UI button lands with the broader
  context menu.
- New `store.SetReaction` / `ListReactions` / `ReactionsForChat`
  plus the `reactions` table with composite PK + index.

[0.1.6]: https://github.com/jdtoon/wachat/releases/tag/v0.1.6

## [0.1.5] - 2026-05-23

### Added

- **Reply quotes** render above the bubble body: a thin
  accent-bordered sub-box showing the quoted sender + snippet.
  Inbound messages with `ContextInfo.QuotedMessage` are persisted
  with quote info.
- **Edit-message** support (receive). `ProtocolMessage MESSAGE_EDIT`
  updates the message body and shows a `(edited)` suffix.
- **Delete-for-everyone** (receive). `ProtocolMessage REVOKE` flips
  the `revoked` flag and the bubble renders "🚫 message deleted" in
  italic instead of the body.
- New `store.ApplyEdit` / `ApplyRevoke` and 5 message columns
  (`quoted_waid`, `quoted_body`, `quoted_sender`, `edited`,
  `revoked`) with transparent migration.

### Internal

- `scanMessageRows` / `scanMessageRow` helpers de-duplicate the long
  SELECT column list across PageOlder / PageAround.
- New `EditRevoker` interface narrows the wa-side store contract.

Send side (right-click → reply / edit / delete) is a follow-up
alongside the broader message context menu.

[0.1.5]: https://github.com/jdtoon/wachat/releases/tag/v0.1.5

## [0.1.4] - 2026-05-23

### Added

- **Group sender labels.** In group chats the head bubble of each
  sender run shows the participant's display name in a
  deterministic-hue accent so the eye can scan the conversation.
- **Chat-row unread emphasis.** Unread chats render with
  SemiBold names and TextPrimary subtitle alongside the existing
  accent badge.
- **Composer polish.** The widget.Editor sits inside a rounded
  Canvas-tinted box, with proper padding and Theme-driven text size.
- New `State.NameFor(jid)` helper and `IsGroup(jid)` predicate.

[0.1.4]: https://github.com/jdtoon/wachat/releases/tag/v0.1.4

## [0.1.3] - 2026-05-23

### Added

- **Receipt indicators** in the message bubble meta row (outgoing
  only): ⏱ pending, ✓ sent, ✓✓ delivered, ✓✓ in accent
  for read (blue-ticks convention), ✓✓ in accent for played.
- **`messages.status` column** with values `pending|sent|delivered|
  read|played`. Migration via `PRAGMA table_info` + `ALTER TABLE` so
  v0.1.2 DBs upgrade transparently.
- **`store.UpdateStatus(ctx, waID, status)`** — single mutator for
  receipt-driven status transitions.
- **WAID dedup on first arrival.** `wa.GenerateID()` pre-mints a
  whatsmeow message ID; `wa.SendText(ctx, chatJID, body, msgID)`
  passes it via `SendRequestExtra.ID`. The optimistic bubble and the
  server-confirmed receipt now share a row from the start — no more
  brief double-bubble.
- **Receipt event handler.** `Handler.OnReceipt` maps
  `events.Receipt` (with all `types.ReceiptType` constants) into
  status updates. `StatusUpdater` interface lets fake stores stay
  narrow in tests.

### Tested

- 6 store status tests (round-trip, unknown WAID, defaults, page-read).
- 7 receipt tests (full kind→status truth table, multi-ID, nil
  safety, missing-StatusUpdater).
- 7-row bubble glyph truth table.

[0.1.3]: https://github.com/jdtoon/wachat/releases/tag/v0.1.3

## [0.1.2] - 2026-05-23

### Added

- **History sync persistence.** `events.HistorySync` is now actually
  stored — `fromWMHistorySync` converts the proto into wachat's
  flat-message + chat-summary form, `store.InsertBatch` writes
  thousands of rows in one transaction (FTS5 triggers index along
  the way), and chats get their display name from the
  Conversation.Name field with a push-name fallback for unnamed 1:1
  chats.
- **Push-name resolution.** `events.PushName` upserts the chats
  table so the chat list flips from raw JIDs to display names as
  soon as whatsmeow learns them.
- New `HistoryPersister` interface (Persister + `InsertBatch` +
  `UpsertChat`) for the wa-side store contract.
- `Handler.Adapter(ctx, ownJIDFn)` — the converter for from-me
  messages in history sync needs the local device JID.

### Tested

- 7 store tests: `InsertBatch` round-trip, dedup, partial skip, no-
  unread, FTS5 index feed.
- 12 wa tests: sender truth table, push-name fallback, nil safety,
  persister-interface enforcement.

## [0.1.1] - 2026-05-23

### Fixed

- **Pairing was silently broken.** `main.go` called
  `waCli.QRChannel` twice (terminal + window); whatsmeow's QR channel
  is single-consumer, so the in-window pairing view never received
  codes. Replaced with a fan-out goroutine.
- **`state.OwnJID` was stuck empty after fresh pair.** Read once at
  startup before the device ID was assigned. Now refreshed on
  `ConnectionConnected`.
- **Chat list didn't refresh post-pair.** Now reloads on
  `ConnectionConnected`.

[0.1.2]: https://github.com/jdtoon/wachat/releases/tag/v0.1.2
[0.1.1]: https://github.com/jdtoon/wachat/releases/tag/v0.1.1

## [0.1.0] - 2026-05-23

**First minor release.** No new code beyond v0.0.7 — this tag is the
milestone marker: the `CLAUDE.md §12` checklist is fully ticked and
the `docs/roadmap.md` Phase 1 "Spine" is shipped.

### What landed across v0.0.2 → v0.0.7

- **Design system** (v0.0.2): typed `Theme` with palette / type /
  spacing / radius / motion / density tokens; light + dark palettes;
  Public Sans embedded; WCAG AA contrast tested.
- **Message UI** (v0.0.3): rounded bubbles with sent-right /
  recv-left alignment, sender-grouping by 5-min window, chat-row
  redesign with deterministic-hue avatars, newest-at-bottom
  ordering, paging-trigger flipped to the top of loaded buffer.
- **Send text** (v0.0.4): `wa.SendText` + `ui.Composer` + optimistic
  bubble via `State.AddOptimistic`. Enter sends, Shift+Enter
  newlines.
- **Full-text search** (v0.0.5): SQLite FTS5 over message bodies,
  search bar in the sidebar, jump-to-message via the new
  `store.PageAround` keyset query.
- **Pairing UX** (v0.0.6): in-window QR drawn from `rsc.io/qr` matrix
  data (no PNG round-trip), pairing state machine, connection
  banner, auto-reconnect, `wa.PairPhone` wrapper for the
  pair-by-code fallback.
- **Themes + layout polish** (v0.0.7): runtime dark/light toggle,
  comfortable/compact density, narrow-window collapse below 760dp,
  preferences persisted to a `settings` table in SQLite.

### Measured at v0.1.0

100k messages, Intel Core Ultra 7 258V, Windows 11, pure-Go build:

| Metric                  | Result                        |
|-------------------------|-------------------------------|
| `store.Open`            | ~6 ms                         |
| Go heap                 | ~0.5 MB (**flat with N**)     |
| Go runtime `Sys`        | ~16 MB                        |
| First keyset page (50)  | ~1 ms                         |
| Deep keyset page (90%)  | ~20 ms                        |
| Bulk insert             | ~2.5k msgs/s (incl. FTS5)     |

The Go heap is **identical** to v0.0.1 despite five new features
(themes, fonts, composer, search, pairing UI). That's the design
system + virtualized lists + keyset paging earning their keep.

### Constraints upheld (CLAUDE.md §2 / §11)

- No Electron / Tauri / webview / embedded browser.
- No full-chat in-memory loads — every read is keyset.
- No DB blobs for media — only paths.
- `CGO_ENABLED=0` confirmed via `go version -m wachat`.
- UI goroutine never blocks on DB writes from background goroutines.
- No `OFFSET` pagination anywhere.

[0.1.0]: https://github.com/jdtoon/wachat/releases/tag/v0.1.0

## [0.0.7] - 2026-05-23

### Added

- **Dark mode + light mode toggle**: ☾/☀ glyph in the conversation
  header. Palette swaps instantly — no widget restyle, no asset
  reload (the Theme is data).
- **Density toggle**: ☰/≡ glyph for comfortable / compact.
- **Narrow-window collapse**: under 760dp the layout flips to a
  single pane (sidebar OR conversation), with a back arrow in the
  conversation header to return to the chat list. Comes straight
  from docs/design.md §2.
- **Settings persistence**: tiny `settings` table in SQLite stores
  `ui.theme` and `ui.density` so launches remember the user's choice.
  New `store.GetSetting` / `SetSetting` helpers; 4 unit tests.

### Changed

- `ViewCallbacks` gains `OnToggleTheme`, `OnToggleDensity`, `OnBack`.
- `View` now owns `themeBtn`, `densityBtn`, `backBtn` Clickables.

[0.0.7]: https://github.com/jdtoon/wachat/releases/tag/v0.0.7

## [0.0.6] - 2026-05-23

### Added

- **In-window QR pairing**: replaces the terminal-only QR with a
  centered 320dp code drawn directly from `rsc.io/qr.Code.Black(x,y)`
  (no PNG round-trip). Phase state machine: WaitingQR → Scanned →
  Syncing → Ready, with Failed for timeouts and unknown events.
- **Connection banner**: thin top strip for Connecting / Offline /
  Signed-out states. Zero-height for ConnConnected so the chrome
  doesn't shift in steady state.
- **`wa.Client.PairPhone(ctx, phone, clientDisplay)`** wraps
  whatsmeow's `PairPhone` for the 8-character pair-by-code flow.
  Callable; UI consumer (phone-input form) lands in a follow-up.
- **`wa.ConnectionState`** + `Handler.OnConnState` callback derived
  from `events.Connected / Disconnected / LoggedOut / PairSuccess`.
- **Auto-reconnect** enabled (`whatsmeow.Client.EnableAutoReconnect`)
  so transient disconnects recover without manual intervention.

### Internal

- New `ui.LayoutConnectionBanner` helper + `bannerCopy` pure function
  for testing.
- `renderRoot` in main.go now picks between PairingView and the main
  two-pane layout.

### Tests

- `internal/ui/pairing_test.go`: state-machine transitions for every
  QR event + direct setter coverage.
- Banner copy: empty on Connected, non-empty on Connecting /
  Disconnected / LoggedOut.

[0.0.6]: https://github.com/jdtoon/wachat/releases/tag/v0.0.6

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

[unreleased]: https://github.com/jdtoon/wachat/compare/v0.1.8...HEAD
[0.0.1]: https://github.com/jdtoon/wachat/releases/tag/v0.0.1
[kac]: https://keepachangelog.com/en/1.1.0/
[semver]: https://semver.org/spec/v2.0.0.html
