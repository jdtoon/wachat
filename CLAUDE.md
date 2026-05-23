# CLAUDE.md — Project Context

> This file is loaded as always-on context by Claude Code. It is deliberately
> focused: it states **what we're building, why, and the rules that must not be
> broken**. When a request conflicts with the constraints below, surface the
> conflict rather than silently working around it.

---

## 1. What we're building

A **desktop WhatsApp client** for personal use, possibly open-sourced later.

The official desktop client is Electron-based and heavy on RAM. This project
exists to be the opposite: a fast, lean, native-feeling client that does not tax
the machine.

**North star: performance first.** Low memory footprint, fast cold start, and
smooth interaction over very large message histories are the primary design
constraints. We optimize for efficiency *before* convenience. If a harder
approach is meaningfully faster or lighter, we take the harder approach. "Just
make it work" is not the goal; "make it work efficiently from the start" is.

---

## 2. Non-negotiable constraints

These are the reason the project exists. Do not relax them without explicit
discussion.

1. **No Electron, no webview, no bundled browser.** This includes Electron,
   Tauri, Wails, CEF, or embedding `web.whatsapp.com` in any browser surface.
   Embedding the web app means embedding their renderer, which defeats the
   entire performance goal. The UI is rendered by us, natively.
2. **Memory usage must be independent of history size.** A chat with 200k
   messages must cost roughly the same to display as one with 200. This is
   achieved by virtualization + pagination (see §6). Never load a full chat
   into memory.
3. **Single language, single process.** Everything is Go, in one process. No
   cross-language IPC bridges (they reintroduce serialization cost and
   complexity we explicitly rejected).
4. **Minimize cgo and heavy dependencies.** Some platform cgo is unavoidable
   (the GUI toolkit links OS graphics/windowing APIs). The rule is: do not
   *add* cgo or large dependencies beyond what's strictly required, and justify
   any new dependency on performance/footprint grounds.

---

## 3. Tech stack

| Layer | Choice | Why |
|------|--------|-----|
| Language | **Go** | One lean runtime; protocol + UI in one process. |
| WhatsApp protocol | **whatsmeow** (`go.mau.fi/whatsmeow`) | Mature, actively maintained multidevice library; compiles into the binary with no separate runtime. Powers the well-tested Matrix WhatsApp bridge. |
| GUI | **Gio** (`gioui.org`) | Immediate-mode, pure-Go API, GPU-rendered, no webview. Its list widget lays out only visible rows — the core of our memory strategy. |
| Storage | **SQLite via `modernc.org/sqlite`** | Pure-Go driver (no cgo) keeps builds and cross-compilation simple. Marginally slower than the cgo `mattn` driver, but the difference is irrelevant at personal-client scale and the cgo-free build is worth more to us. |
| Media | **Files on disk** | Never stored as blobs in the DB (see §7). |

Notes:
- **whatsmeow's API drifts.** Recent versions thread `context.Context` through
  calls like `sqlstore.New` and `GetFirstDevice`. **Always verify current
  signatures against the godoc** (`pkg.go.dev/go.mau.fi/whatsmeow`) rather than
  trusting any example — including the ones in this file, which are
  illustrative.
- Gio is immediate-mode: the UI is a function of state, redrawn each frame.
  There is no retained widget tree to mutate. Internalize this before writing UI
  code.

---

## 4. Architecture & data flow

One process, clear boundaries:

```
WhatsApp servers
      │  (multidevice protocol, encrypted)
      ▼
whatsmeow client ──fires events on its OWN goroutines──┐
                                                       │
   (event handler: do minimal work)                   │
      1. write message to SQLite                       │
      2. non-blocking send on a channel                │
      3. w.Invalidate()  ── wakes the frame loop ──────┤
                                                       ▼
                                            Gio UI goroutine
                                            (owns the window)
      drain channel → update view model → virtualized render
```

The golden rule: **whatsmeow events arrive on background goroutines; the Gio UI
goroutine owns all UI state.** Never mutate UI state from an event handler.
Persist, hand off via channel, and `Invalidate()` to wake the frame loop. The
frame loop reads the data.

Illustrative handoff (verify signatures):

```go
// Called from whatsmeow's goroutine — keep it cheap.
onMsg := func(m ChatMessage) {
    store.Insert(m)                        // persist first
    select { case msgCh <- m: default: }   // non-blocking; never block the handler
    w.Invalidate()                         // wake Gio's frame loop
}
```

---

## 5. Project structure

```
wachat/
  main.go                    // Gio window + frame loop; owns UI goroutine
  go.mod
  CLAUDE.md                  // this file
  internal/
    wa/
      client.go              // whatsmeow connect, pairing, event handler
    store/
      store.go               // SQLite: schema, prepared statements, queries
      schema.sql             // canonical schema
    ui/
      app.go                 // top-level layout, view-model state
      chatlist.go            // virtualized list of chats
      messages.go            // virtualized message view + bubble rendering
  media/                     // downloaded media on disk (gitignored)
  wachat.db                  // SQLite file (gitignored)
```

Keep protocol, storage, and UI in separate packages. The UI depends on the
store; the store does not depend on the UI.

---

## 6. Performance levers (ranked by impact)

Implement these from the first commit — they are scaffolding, not optimizations
to retrofit later.

1. **Virtualize every list** (chat list AND message list). Use Gio's
   `list.Layout(gtx, count, fn)` so `fn` is called only for visible rows.
   This is the single biggest win and the reason memory stays flat.
2. **Keyset (cursor) pagination — never `OFFSET`.** Load a window of ~50 rows;
   fetch older rows as the user scrolls up, using the oldest loaded timestamp as
   the cursor. `OFFSET` degrades linearly with depth; keyset stays O(page).
3. **SQLite tuning:** `PRAGMA journal_mode=WAL;` (concurrent read while writing),
   `PRAGMA synchronous=NORMAL;`, an index on `(chat_jid, ts DESC)`, and prepared
   statements for hot queries.
4. **Media on disk, decoded lazily.** Store file paths in the DB. Decode and
   downscale thumbnails only when a row becomes visible; release them when it
   scrolls away. Full-resolution images held in memory are how chat clients
   balloon — don't.
5. **Keep the UI goroutine pure.** DB writes, network, and image decoding happen
   off it. The frame loop should only read prepared data and lay it out.

---

## 7. Data model & store rules

Canonical schema (see `internal/store/schema.sql`):

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;

CREATE TABLE IF NOT EXISTS messages (
  id         INTEGER PRIMARY KEY,
  wa_id      TEXT UNIQUE,          -- WhatsApp message id; dedup on this
  chat_jid   TEXT NOT NULL,
  sender_jid TEXT,
  ts         INTEGER NOT NULL,     -- unix millis
  body       TEXT,
  media_path TEXT,                 -- path on disk; NULL for text-only
  media_type TEXT                  -- image/video/audio/doc, NULL for text
);
CREATE INDEX IF NOT EXISTS idx_chat_ts ON messages(chat_jid, ts DESC);

CREATE TABLE IF NOT EXISTS chats (
  jid         TEXT PRIMARY KEY,
  name        TEXT,
  last_ts     INTEGER,
  unread      INTEGER DEFAULT 0
);
```

Rules:
- **Never store media bytes in the DB.** Write the file to `media/`, store the
  path. Blobs bloat the DB, slow every query, and break the memory model.
- **Dedup on `wa_id`** with `INSERT ... ON CONFLICT(wa_id) DO ...` — whatsmeow
  can redeliver on reconnect.
- Page query: `SELECT ... FROM messages WHERE chat_jid=? AND ts<? ORDER BY ts
  DESC LIMIT 50`. The newest page uses a sentinel large `ts`.

---

## 8. Concurrency rules

- The Gio UI goroutine owns the window and all UI state.
- whatsmeow handlers run on background goroutines: persist + hand off via channel
  + `Invalidate()`. Nothing else.
- Channel sends from handlers are **non-blocking** (`select { case ch <- x:
  default: }`) so a slow UI frame can never stall the protocol layer.
- Long work (media download/decode) runs in its own goroutine and signals
  completion the same way (persist path → `Invalidate()`).

---

## 9. Build & run

```bash
go mod tidy
go run .          # dev
go build -o wachat -ldflags="-s -w" .   # stripped release binary
```

First run pairs via QR code (link as a companion device, like WhatsApp Web).
Session must persist to SQLite so subsequent launches don't re-pair — broken
session persistence is the most common cause of repeated re-authentication.

Useful checks:
```bash
gofmt -l .        # must be clean
go vet ./...
```

---

## 10. Risks & gotchas

- **Unofficial client / Terms of Service.** This uses a reverse-engineered
  library and violates WhatsApp's ToS; there is real ban risk. Mitigations:
  personal use only, human-like behavior (no bulk/automated sending), and
  rock-solid session persistence. Treat this as a known, accepted risk for a
  personal project — flag, don't silently expand it (e.g. don't add bulk-send
  features).
- **Protocol drift.** whatsmeow can break when WhatsApp changes the protocol;
  expect occasional dependency bumps and signature changes. Verify APIs against
  current godoc.
- **Gio learning curve.** Immediate-mode is unfamiliar if you're used to
  retained-mode/React. State lives outside the render functions.

---

## 11. Guardrails — do NOT do these

Even if asked indirectly, stop and confirm before:
- Introducing Electron, Tauri, Wails, or any webview/embedded browser.
- Loading an entire chat's messages into memory.
- Storing media as blobs in SQLite.
- Doing DB/network/decode work on the UI goroutine, or mutating UI state from a
  whatsmeow handler.
- Using `OFFSET` for message pagination.
- Adding a heavy dependency without a footprint/performance justification.
- Adding bulk-messaging or automation features (ToS/ban risk, and out of scope).

---

## 12. Status & next steps

- [ ] `go.mod` + dependencies pinned
- [ ] whatsmeow connect + QR pairing + session persistence
- [ ] SQLite store: schema, insert-with-dedup, keyset page query
- [ ] Event handler → channel → `Invalidate()` wiring
- [ ] Gio frame loop + two-pane layout (chat list | messages)
- [ ] Virtualized chat list
- [ ] Virtualized message view + bubble rendering
- [ ] Lazy media download + thumbnail decode/release
- [ ] Measure: idle RAM, cold-start time, scroll smoothness over a large history

**Performance budget (targets, validate as we go):** idle RSS in the tens of MB
(vs. the official client's hundreds), sub-second cold start, no frame hitches
scrolling a 100k-message chat.

When in doubt, choose the option that keeps memory flat and the UI goroutine
free — that is almost always the right call for this project.
