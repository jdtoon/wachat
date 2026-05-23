# Getting Started

## Build

Requires **Go 1.25+** and (optionally) GNU `make`.

```bash
git clone https://github.com/jdtoon/wachat
cd wachat
make hooks   # installs the local pre-commit hook (one-time)
make check   # gofmt + go vet + go test ./...
make build   # produces a ~38 MB stripped binary
```

## Run

```bash
./wachat               # connects to WhatsApp, prints QR on first run
./wachat -no-connect   # offline UI against the local DB (dev mode)
./wachat -db custom.db # alternate DB path
./wachat -version
```

## First-run pairing

On first launch with no existing session, `wachat` prints a half-block QR code to your **terminal**. Open WhatsApp on your phone → **Settings → Linked Devices → Link a Device** and scan it. The session is saved to `wachat-session.db` (alongside `wachat.db`), so subsequent launches skip pairing.

If pairing fails or times out, just restart `wachat`.

## Demo data (no real account needed)

```bash
go run ./cmd/seed -db wachat.db -n 60
./wachat -no-connect -db wachat.db
```

This populates 5 demo chats (Alice, Bob, Family, Work, Ada) with 60 mixed messages each so you can see the UI without pairing.

## Perf harness

```bash
make bench
```

Seeds a temp DB with 100k synthetic messages, prints `store.Open`, seed throughput, Go heap, RSS proxy, and first-page / deep-page keyset query latencies.
