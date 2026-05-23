# ROADMAP.md — Feature Plan

> Companion to `CLAUDE.md`. That file holds the architecture and the
> non-negotiable rules; this file holds **what to build, in what order, and what
> each feature is allowed to cost**. Work top to bottom. Do not pull a feature
> forward if it skips its phase's foundations.
>
> The north star still wins every tie: **performance first.** "Fancy" here means
> craft (native-smooth, instant, lightweight), never weight. Every feature has a
> cost tag and, where relevant, a mandatory implementation approach. If a feature
> can't be built within its budget, raise it — don't ship the heavy version.

---

## Legend

**Cost tags**
- `[FREE]` — UI work only; memory stays flat. Build freely.
- `[WATCH]` — fine if built correctly (virtualize / load-on-visible / frame-cap).
  The note says how. Building it naively violates the north star.
- `[HEAVY]` — real risk to the memory model. Mandatory mitigation listed; do not
  ship without it.

**Status:** `[ ]` todo · `[~]` in progress · `[x]` done

---

## Decisions log (locked unless revisited explicitly)

- **Calls: notifications & history only.** whatsmeow cannot place or answer
  voice/video calls. We surface incoming-call *events* and a call log. **Do not
  attempt to implement actual calling** — it's a protocol-layer gap, not a UI
  problem.
- **Status / Stories:** experimental in whatsmeow (may fail for large contact
  lists). Stretch only, best-effort, never core.
- **Broadcast list sending:** out of scope (unsupported by the protocol).
- **No automation / bulk send:** ToS/ban risk and out of scope (see CLAUDE.md §11).

---

## whatsmeow capability map (quick reference)

Native (UI work only): text + media send/receive (private & group), group
management + change events, invite links, typing notifications, delivery/read
receipts, app state (contacts, pin/mute), presence/last-seen, privacy settings,
media up/download, newsletters/Channels.

Message-level (ride the message protobuf — **verify exact helper names against
the godoc** before implementing): reactions, replies/quotes, @mentions, edits,
delete-for-everyone (revoke), polls + votes, disappearing-message timers,
view-once.

Gaps: calls (events only), status (experimental), broadcast send (none).

---

## Phase 1 — Spine

**Goal:** it runs, it's fast, the performance scaffolding is real from commit one.

- [ ] `go.mod` + pinned deps (`whatsmeow`, `gioui.org`, `modernc.org/sqlite`)
- [ ] whatsmeow connect + **QR pairing** + session persistence to SQLite
- [ ] Auto-reconnect + retry-receipt handling
- [ ] SQLite store: schema, WAL, indexes, insert-with-dedup on `wa_id`
- [ ] Event handler → non-blocking channel → `Invalidate()` wiring (CLAUDE.md §4/§8)
- [ ] Gio frame loop + two-pane layout (chat list | message view)
- [ ] `[WATCH]` Virtualized chat list — `list.Layout`, visible rows only
- [ ] `[WATCH]` Virtualized message view — keyset pagination, ~50/page, no `OFFSET`
- [ ] Send/receive **text** end to end
- [ ] `[FREE]` **Full-text search (FTS5)** across all history
  - Note: confirm the SQLite driver build has **FTS5 compiled in**
    (`modernc.org/sqlite` usually does — verify). Build the index incrementally
    on insert; keep it on disk; never load it into memory. This is a headline
    feature and nearly free here — prioritize it.

**Done when:** cold start < 1s; idle RSS in tens of MB; scrolling a synthetic
100k-message chat has no frame hitches; search returns instantly on that chat.

---

## Phase 2 — Parity (replaces the official client)

**Goal:** everything a daily driver needs, all within budget.

Media
- [ ] `[HEAVY]` Images/video — store files on disk, paths in DB. **Thumbnails
      decoded + downscaled on-visible, released on scroll-away.** Full-res only
      on explicit open. Naive (decode-all / blob-in-DB) is forbidden.
- [ ] `[WATCH]` Voice notes — stream from disk; waveform generated once + cached.
- [ ] `[FREE]` Documents — metadata + download-on-demand, never auto-loaded.
- [ ] `[WATCH]` Stickers (static first) — decode-on-visible, small cache.

Messaging
- [ ] `[FREE]` Reactions (send + render)
- [ ] `[FREE]` Replies / quoted messages
- [ ] `[FREE]` @mentions (compose + highlight)
- [ ] `[FREE]` Edit message + delete-for-everyone (revoke)
- [ ] `[FREE]` Delivery/read receipts UI + typing indicators

Presence & chats
- [ ] `[FREE]` Presence / last-seen dots
  - Note: receiving others' presence may require marking yourself available,
    which affects your own visibility. Make this a user-controllable setting,
    default off, and document the tradeoff in-app.
- [ ] `[FREE]` Group management (create, add/remove, subject/desc, invite links)
- [ ] `[FREE]` Pin / mute / archive (app state — syncs across devices)
- [ ] `[FREE]` Unread counts, mark-read, chat sorting

**Done when:** you can use it as your only WhatsApp client for a week without
opening the official app (calls excepted).

---

## Phase 3 — Fancy (beats the official client)

**Goal:** the differentiators. Speed-as-feature + power-user tooling + tasteful polish.

Power-user
- [ ] `[FREE]` **Command palette (Ctrl+K)** — jump to chat, run actions, search
- [ ] `[FREE]` Quick chat switcher + vim-style keyboard navigation
- [ ] `[HEAVY]` **Multi-account in one window** — each account is its own
      whatsmeow `Client` + device store. Keep per-account DBs separate; lazy-init
      inactive accounts so idle accounts cost near-zero. Do not hold all accounts
      fully resident.
- [ ] `[FREE]` Message bookmarks / starred messages
- [ ] `[FREE]` Custom folders / filters / per-chat notification rules
- [ ] `[FREE]` Scheduled send / draft queue (local timer → send on fire)
- [ ] `[FREE]` Local export (Markdown / JSON) per chat or range

Composer & content
- [ ] `[FREE]` Polls (create + vote) — verify msgsecret/poll helpers in godoc
- [ ] `[FREE]` Disappearing-message timers + view-once handling
- [ ] `[WATCH]` Rich link previews — fetch off the UI goroutine, cache result,
      downscale preview image, render on-visible
- [ ] `[WATCH]` Emoji / sticker / GIF pickers — **virtualize the grids.** A
      non-virtualized picker is a surprising memory spike. Frame-cap any preview.

Polish
- [ ] `[FREE]` Themes (incl. true OLED dark), density toggle — Gio styling is
      just color/style structs, so this stays free (no webview CSS cost)
- [ ] `[WATCH]` Animated stickers / GIF autoplay — decode on-visible, **cap
      framerate**, free frames on scroll-away. Never retain off-screen animation.
- [ ] `[FREE]` Smooth native transitions, jump-to-date, scrollback-to-anywhere

**Done when:** the things that are slow/missing in the official client (search,
startup, scrollback, keyboard control, multi-account) are demonstrably better here.

---

## Phase 4 — Stretch (nice-to-have)

- [ ] `[FREE]` Incoming-call **notifications + call log** (events only — no calling)
- [ ] `[WATCH]` Channels / newsletters — virtualized like any other feed
- [ ] `[WATCH]` Status / Stories viewing — experimental; best-effort, isolate so
      failures don't affect core
- [ ] `[FREE]` Plugin / scripting hooks (e.g. message filters, auto-tag)
- [ ] `[FREE]` Backup / restore of local DB + media

---

## Cross-cutting performance budget

The recurring discipline for every `[WATCH]`/`[HEAVY]` item:

| Risk | Rule |
|---|---|
| Decoded media in RAM | Load-on-visible, release-on-hide. Full-res only on open. |
| Animation frames | Decode-on-visible, frame-capped, freed off-screen. |
| Large grids/lists/pickers | Virtualize — visible children only, always. |
| Media bytes | On disk, never DB blobs. DB stores paths. |
| Search index | On disk, incremental, never resident. |
| Inactive accounts | Lazy-init, near-zero idle cost. |
| UI goroutine | Never blocked: no DB/network/decode on it (CLAUDE.md §8). |

**Per-phase gate:** before closing a phase, re-measure idle RSS, cold start, and
scroll smoothness on a large synthetic history. A phase that regresses these is
not done, regardless of feature completeness.

---

## Reminders for the implementer

- whatsmeow's API drifts (notably `context.Context` threading). Verify every
  signature against `pkg.go.dev/go.mau.fi/whatsmeow` — do not trust examples,
  including in this repo's docs.
- When a feature tempts you toward a webview, Electron, in-memory full history,
  DB media blobs, or `OFFSET` pagination: stop and flag (CLAUDE.md §11).
- Prefer the harder, lighter approach. That is the whole point of this project.