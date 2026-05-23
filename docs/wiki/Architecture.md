# Architecture

> The canonical source is [`CLAUDE.md`](https://github.com/jdtoon/wachat/blob/main/CLAUDE.md). This page paraphrases for wiki browsers; if the two ever drift, `CLAUDE.md` wins.

## One process, three layers

```
internal/wa     ── whatsmeow boundary. Connect, QR pairing, event handler.
internal/store  ── SQLite. Schema, dedup insert, keyset PageOlder.
internal/ui     ── view-model State + Gio layout + virtualized lists.
internal/media  ── lazy thumbnail cache + visibility tracker.
```

Plus `main.go` as the wiring layer that creates the Gio window and connects the goroutines.

## The handoff rule

whatsmeow events fire on background goroutines. The Gio UI goroutine owns all UI state. The boundary is:

1. **Persist** the message to SQLite (so a missed UI delivery doesn't lose it).
2. **Non-blocking send** on a buffered channel (`select { case ch <- ev: default: }`).
3. **Notify** via `w.Invalidate()` to wake the frame loop.

The frame loop drains the channel each frame and calls `state.OnIncoming` — the only mutation that runs in response to network events.

## Keyset pagination

The cursor is `(TS, ID)`, not just `TS`. Two messages sharing a millisecond would otherwise be skipped or duplicated across page boundaries; the compound tuple makes the comparison strictly monotonic. See `internal/store/paging.go`.

We never use `OFFSET` — it's O(N) at depth.

## Virtualization

Both lists go through `material.List` → `gioui.org/layout.List`, which invokes the per-row builder only for items in the visible window. Tests assert this: `TestView_ChatListVirtualizes_OnlyVisibleRowsBuilt` confirms 13 / 10_000 rows are built per frame.
