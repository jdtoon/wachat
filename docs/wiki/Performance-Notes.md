# Performance Notes

## Budget (from `CLAUDE.md §12`)

- Idle RSS in the **tens** of MB (vs. the Electron client's hundreds).
- Sub-second cold start on a warm OS file cache.
- No frame hitches scrolling a 100k-message chat.

## Measurements

Reproduce via `make bench`. Numbers below are from an Intel Core Ultra 7 258V on Windows 11, pure-Go build (`CGO_ENABLED=0`).

| Metric                  | Result                        |
|-------------------------|-------------------------------|
| `store.Open`            | ~5 ms                         |
| Bulk insert             | ~10k msgs/s                   |
| Go heap (`HeapAlloc`)   | ~0.5 MB after 100k msgs       |
| Go runtime `Sys` (RSS≈) | ~16 MB                        |
| First keyset page (50)  | ~520 µs                       |
| Deep keyset page (90%)  | ~12 ms (cold index pages)     |

## Why flat heap matters

The Go heap stays at ~0.5 MB whether the DB has 5k or 100k messages. That's the contract from `CLAUDE.md §2`: **memory must be independent of history size.** It's achieved by:

1. Storing media bytes on disk, not in the DB (`§7`).
2. Keyset paging so only ~50 messages are in memory at a time.
3. Virtualized lists so only ~13–23 rows are laid out per frame.

If you ever see heap growing linearly with chat size, something has regressed. The `cmd/bench` output is the canary.

## Why deep page is slower

The deep page (~90% into history) hits SQLite index pages that aren't in the OS page cache yet. ~12 ms is well under one frame (~16 ms), so it doesn't matter for interactive scrolling. The `first-page` query is fast because those rows were just inserted and are hot in cache.
