# Contributing to wachat

Thanks for your interest. `wachat` is small and opinionated; this document
captures what is most useful to know before sending changes.

## Ground rules

The project has hard constraints that exist for a reason. Read
[`CLAUDE.md`](./CLAUDE.md), especially:

- **§2** — non-negotiable constraints (no Electron, memory must be independent
  of history size, single language/process).
- **§6** — the five performance levers (virtualization, keyset paging, SQLite
  pragmas, lazy media, pure UI goroutine).
- **§11** — the explicit guardrails. If a change touches any of these, surface
  the conflict rather than working around it silently.

If you're unsure whether a change fits, open an issue first.

## Local development loop

There is no CI. Every gate runs on your machine.

```bash
git clone https://github.com/jdtoon/wachat
cd wachat
make hooks   # one-time: installs scripts/pre-commit into .git/hooks
make check   # gofmt -l + go vet + go test ./... -race -count=1
```

`make check` is the same set of checks the pre-commit hook runs, so commits
that pass locally won't surprise you later.

### Useful targets

| Target | Purpose |
|--------|---------|
| `make fmt` | format the tree (`gofmt -w .`) |
| `make vet` | `go vet ./...` |
| `make test` | `go test ./... -race -count=1` |
| `make cover` | coverage report |
| `make run` | `go run .` (launches the app) |
| `make build` | stripped release binary |
| `make clean` | remove built artifacts and the local DB/media |

## Commit style

- One logical change per commit.
- Imperative subject, lowercase, optional Conventional Commits prefix
  (`feat(store):`, `fix(ui):`, `chore:`, `docs:`, `test:` …).
- Tests land in the same commit as the code that introduces them. The
  pre-commit hook nudges this with a heuristic check.
- Update the status checklist in `README.md` when a `§12` line ticks.

## Tests

Testing is treated as a first-class concern, not an afterthought:

- The store layer (`internal/store/`) must have unit tests for every public
  function: schema idempotency, dedup, keyset paging correctness, and a
  benchmark proving page latency is O(page) not O(N).
- The wa layer (`internal/wa/`) tests cover the pure parts — event
  normalization, dedup-key derivation, and the **non-blocking** handoff
  contract. We do not mock the whatsmeow client.
- The UI layer (`internal/ui/`) tests the view-model reducer without spinning
  up a Gio frame loop, and instruments virtualization to assert that row
  callbacks are O(viewport).

## Dependencies

No new dependency should be added without a footprint/perf justification in
the PR description (see `CLAUDE.md §3`). cgo is a hard no for anything not
strictly required by the OS GUI integration.

## Reporting bugs

Use the bug-report issue template. Please include:

- OS + Go version
- Steps to reproduce
- `make check` output if the bug surfaces during the test gate

## Security

See [`SECURITY.md`](./SECURITY.md). Do not file security issues publicly.
