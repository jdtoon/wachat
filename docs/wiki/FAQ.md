# FAQ

## Will I get banned for using this?

**Maybe.** `wachat` uses [`whatsmeow`](https://github.com/tulir/whatsmeow), a reverse-engineered library. Running any unofficial client violates WhatsApp's Terms of Service. The risk is real but not theoretical — we mitigate by behaving like a human client (no bulk sending, no automation) and persisting the session properly so we don't re-pair constantly.

Use a personal account, accept the risk, don't connect a business-critical number.

## Why no Electron / webview / browser?

Because Electron-based WhatsApp Desktop eats hundreds of MB of RAM at idle. The whole point of `wachat` is to be the opposite — see [`CLAUDE.md §1`](https://github.com/jdtoon/wachat/blob/main/CLAUDE.md). Gio renders natively, the binary is one process, idle RSS sits in the tens of MB.

## Why Go and not Rust / C++ / Swift?

One lean runtime, protocol (`whatsmeow`) + UI (`Gio`) in the same language, fast compiles, easy cross-compilation. The cost is a slightly larger binary (~38 MB stripped) vs. a Rust equivalent, but it's a wash with the alternative's complexity.

## Why pure-Go SQLite?

`modernc.org/sqlite` removes the cgo dependency. The slight perf hit vs. the C driver is irrelevant at personal-client scale, and we get clean cross-compilation and a simpler build (`CGO_ENABLED=0` confirmed in releases).

## Where's the iOS / Android / macOS version?

`wachat` targets desktop. Mobile clients exist already and they're not memory-bound the way desktop is.

## Can I run a bot with this?

No. `wachat` is for personal use; the README and `SECURITY.md` are explicit. We won't add bulk-messaging or automation features — that
