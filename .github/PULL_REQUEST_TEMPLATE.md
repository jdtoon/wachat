## Summary

<!-- One or two sentences on what this PR changes and why. -->

## Checklist

- [ ] `make check` passes locally (gofmt, vet, race-enabled tests)
- [ ] New / changed behavior has tests in the same commit
- [ ] No new dependency, or new dep justified on footprint/perf grounds
- [ ] No `OFFSET` paging; no full-chat loads; no DB blobs for media
- [ ] UI goroutine stays pure (no DB / network / decode on the frame loop)
- [ ] `CLAUDE.md §11` guardrails respected
- [ ] README status checklist updated if a `§12` line ticks

## Test plan

<!-- How a reviewer can verify this works. Commands to run, scenarios to
exercise manually, expected output. -->

## Notes

<!-- Anything reviewers should know: trade-offs, follow-ups, screenshots
(no message contents please). -->
