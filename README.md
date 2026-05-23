# wachat

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/jdtoon/wachat)](./go.mod)
[![Latest release](https://img.shields.io/github/v/release/jdtoon/wachat?display_name=tag&sort=semver)](https://github.com/jdtoon/wachat/releases)
[![Repo size](https://img.shields.io/github/repo-size/jdtoon/wachat)](https://github.com/jdtoon/wachat)
[![Last commit](https://img.shields.io/github/last-commit/jdtoon/wachat/main)](https://github.com/jdtoon/wachat/commits/main)

A lean, native desktop WhatsApp client written in Go. No Electron, no webview,
no bundled browser. One process, low RAM, fast cold start.

> The official WhatsApp desktop client is Electron and eats hundreds of MB
> of RAM at rest. `wachat` aims to be the opposite: tens of MB at idle, sub-
> second cold start, and smooth scrolling over 100k-message histories.

**Status:** early bootstrap. See [Status](#status).

## North star

Performance first. Memory usage must be independent of history size, achieved
by **virtualized rendering** (only visible rows laid out) and **keyset
pagination** (never `OFFSET`). If a harder approach is meaningfully faster or
lighter, we take the harder approach.

## Stack

| Layer | Choice | Why |
|------|--------|-----|
| Language | Go | One lean runtime, protocol + UI in one process |
| Protocol | [`whatsmeow`](https://pkg.go.dev/go.mau.fi/whatsmeow) | Mature multidevice library, compiled into the binary |
| GUI | [`Gio`](https://gioui.org) | Immediate-mode, pure-Go, GPU-rendered, no webview |
| Storage | [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) | Pure-Go SQLite driver — no cgo, clean cross-compilation |
| Media | Files on disk | Never blobs in the DB |

See [`CLAUDE.md`](./CLAUDE.md) for the full architecture and the non-negotiable
constraints (no Electron/webview, memory must be independent of history size,
no DB blobs for media, no `OFFSET` pagination, etc.).

## Build & run

Requires **Go 1.25+** (pinned by `modernc.org/sqlite`). No CI pipeline — every
gate runs locally.

```bash
make hooks   # one-time: installs the local pre-commit hook
make check   # gofmt + go vet + go test ./...
make run     # go run .
make build   # produces stripped `wachat` binary
```

First run pairs with your phone via QR code displayed in the terminal
(companion-device flow, same as WhatsApp Web). The session is persisted to
SQLite — subsequent launches skip pairing.

## Project layout

```
wachat/
  main.go                   # Gio window + frame loop
  internal/
    wa/        client.go    # whatsmeow connect, pairing, events
    store/     store.go     # SQLite: schema, dedup insert, keyset paging
    ui/        app.go       # view-model state, two-pane layout
               chatlist.go  # virtualized chat list
               messages.go  # virtualized message view
  scripts/                  # pre-commit hook + installer
  Makefile
```

## Status

Tracks [`CLAUDE.md §12`](./CLAUDE.md#12-status--next-steps).

- [x] `go.mod` + initial dependencies pinned (`modernc.org/sqlite`, `whatsmeow`, `gioui.org`)
- [x] whatsmeow client wrapper (connect, QR pairing, session container)
- [x] SQLite store: schema, insert-with-dedup, keyset page query
- [x] Event handler → channel → notify pipeline (persist-first, non-blocking send)
- [x] Gio frame loop + two-pane layout (chat list | messages)
- [x] Virtualized chat list — bounded by viewport regardless of total
- [x] Virtualized message view + bubble rendering
- [x] whatsmeow connect + QR pairing wired into main.go
- [x] Auto-page older messages on scroll-near-end
- [x] Lazy media cache + visibility tracker (see `internal/media`)
- [x] Perf measurement harness — `make bench` (see Performance budget below)

## Performance budget (validated as we go)

Targets:

- Idle RSS in the tens of MB (vs. hundreds for the official client)
- Sub-second cold start on a warm OS file cache
- No frame hitches scrolling a 100k-message chat

Measured via `make bench` (100k seeded messages, Intel Core Ultra 7 258V,
Windows 11, pure-Go build):

| Metric                  | Result                        |
|-------------------------|-------------------------------|
| `store.Open`            | ~5 ms                         |
| Go heap after 100k msgs | ~0.5 MB (independent of N)    |
| Go runtime `Sys` (RSS≈) | ~16 MB                        |
| First keyset page (50)  | ~520 µs                       |
| Deep keyset page (90%)  | ~12 ms (cold index pages)     |

The flat heap line is the load-bearing measurement: it proves the
"memory must be independent of history size" constraint from
[`CLAUDE.md §2`](./CLAUDE.md#2-non-negotiable-constraints).

## Contributing

See [`CONTRIBUTING.md`](./CONTRIBUTING.md). The TL;DR: run `make hooks` once,
keep `make check` green, never violate the guardrails in `CLAUDE.md §11`.

## Risks & scope

`wachat` uses a reverse-engineered library and violates WhatsApp's Terms of
Service; there is real account-ban risk. It is built for **personal use**.
Bulk-messaging or automation features are explicitly out of scope.

## License

[MIT](./LICENSE).
