<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0308 — Git adapter and authoritative object source](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-15-0308-git-adapter-and-authoritative-object-source.md)**
<!-- docket:backlink:end -->

# Git adapter and authoritative object source — results
Change: #308 · Branch: feat/git-adapter-and-authoritative-object-source · PR: recorded in the change manifest's `pr:` field · Plan: docs/superpowers/plans/2026-08-15-git-adapter-and-authoritative-object-source.md · ADRs: 1, 34

## Verify (human)

- [ ] Accept the pre-existing suite advisory `OVER BUDGET: test_sync_agents_runners` — 197s against
  a 60s ceiling. Wall-clock budgets are machine-dependent and the overage predates this branch, but
  it is over 3× and is the suite's real long pole; confirm it needs no action before merge.
- [ ] Accept `tests/test_go_race.sh`'s budget row landing **at** the 60s ceiling with no headroom.
  The sizing is mechanical (worst reading 53 → next multiple of 5 → +5s margin), but the next thing
  that slows the race detector must shard the file rather than raise the row, because raising it is
  what the ceiling forbids. That instruction is written into both the tsv header and the test file.
- [ ] Accept that the Go tests now run **twice** per suite — once plain (`test_go_toolchain.sh`,
  which owns the four-tuple cross-build) and once instrumented (`test_go_race.sh`). They run
  concurrently in the parallel phase, and an instrumented build is a separate build-cache entry, so
  neither run makes the other free.

## Findings

- **A guard added for a review finding turned out to be decoration, and mutation testing is what
  caught it.** The fix for the double network timeout first added an `inheritDeadline` sentinel to
  `runRequest` so a multi-process operation could suppress `run`'s per-process clock. Mutating the
  sentinel away left the test **green**: the shared `context.WithTimeout` that `FetchBranch` opens
  already caps both processes, because an inner `WithTimeout` can only ever shorten the deadline
  already in force, never extend past it. The sentinel was removed and the guard re-verified against
  the shared context, which does redden. The seam would have read as load-bearing to every later
  reader.

- **The review's stated premise for the pathspec finding was wrong, and the wrong fix would have
  passed review.** Finding #6 asserted the `GIT_*_PATHSPECS` family is subsumed by appending
  `GIT_LITERAL_PATHSPECS=1`, so no separate scrub was needed. Reproduced against real git, appending
  the literal control alongside an inherited `GIT_ICASE_PATHSPECS` fails with
  `fatal: global 'literal' pathspec setting is incompatible with all other global pathspec settings`
  — it swaps one fatal for another and leaves Finding #1's actual requirement unmet. The fix scrubs
  the whole `_PATHSPECS`-suffix family so the appended control stands unopposed.

- **Secret redaction belongs at the boundary, not at the call sites.** Remote URLs reach a
  diagnostic only through `stderrExcerpt`, so redaction lives there and covers every stderr-derived
  `Detail` in the package by construction. A per-site scrub is only as complete as the list of sites
  someone remembered to change. It keys on URL *shape* — any scheme, plus git's scp-like
  `user@host:path` — rather than a scheme list.

- **Redaction must precede truncation.** Bounding the excerpt first can sever a URL mid-token and
  leave the scheme and its userinfo credential inside the surviving window. `TestStderrExcerptRedactsBeforeTruncating`
  pins the ordering with a URL straddling the 1024-byte boundary.

- **A declared field that is never written is a silent contract break.** `Failure.ExitCode` was
  declared with a documented meaning and never assigned; every call site read `res.exitCode` locally
  instead. Every failure classified from a non-zero child exit now carries the status it was
  classified from.

- **The timeout guarantee held for the direct child only.** Without `cmd.WaitDelay`, a grandchild
  that inherited stdout/stderr — a credential helper, an ssh multiplexer — keeps the capture pipe
  open and `cmd.Run` blocks past the fired deadline. The test spawns exactly that shape.

- **The runtime-budget table's ceiling is a relief counter, and it worked.** Folding
  `go test -race ./...` into `tests/test_go_toolchain.sh` measured that file at 57/53/53s, above the
  table's 60s hard ceiling. The ceiling exists to catch slow work laundered into one row's budget,
  so raising it would have been the evasion it detects. Sharding into `tests/test_go_race.sh` is the
  sanctioned response: two rows, each under the ceiling, running concurrently.

- **The race gate is repo-wide by choice.** `./...` rather than an enumerated package list: the
  adapter surfaces in `internal/gitcli` are the ones held concurrently today, but an enumeration
  gates only the packages someone remembered, and the package that grows a race is by definition the
  one nobody thought of.

## Mutation evidence

Every guard introduced or repaired in the review fix loop was mutation-tested — the guarded thing
removed, the assert observed to redden, the mutation reverted:

| Mutation | Reddens |
|---|---|
| `stderrExcerpt` stops calling `redactRemoteLocations` | `TestNoRemoteURLInDiagnostics`, `TestStderrExcerptRedactsBeforeTruncating` |
| `withExitCode` dropped from `ResolveRef`'s non-zero-exit failure | `TestFailureCarriesChildExitCode` |
| `cmd.WaitDelay` not set | `TestRunWaitDelayBoundsPipeHoldingGrandchild` |
| `--full-tree` removed from `ReadBlobs`' `ls-tree` | `TestReadBlobsResolvesFromRepositoryRoot` |
| `FetchBranch`'s shared network context removed | `TestFetchFailureClassificationSharesOneNetworkBudget` |
| `GIT_LITERAL_PATHSPECS=1` append stripped | `TestPathspecMagicPathsResolveLiterally`, `TestReadBlobsNeutralizesInheritedPathspecMagic` |
| `_PATHSPECS` suffix scrub disabled | `TestReadBlobsNeutralizesInheritedPathspecMagic` (exact git fatal) |
| `GIT_EXEC_PATH` removed from the scrub set | `TestSanitizeRemovesRedirectionClassesKeepsAuthSentinel` |
| control-dedup case deleted | `TestSanitizeDropsInboundControlCopies` |

`TestConcurrentOperationsShareClientAndSourceSafely` is the one addition with no mutation pair: it is
a regression net over a property that currently holds by construction (no mutable per-call state),
not a guard over a specific line. Its value is that a future field which starts being written
mid-call reddens it under `-race`.

## Runtime budget

The Go suite was re-measured rather than assumed. Serial, warm, one machine:

- `tests/test_go_toolchain.sh` — 15s Go-test portion; row stays **20s**, unchanged.
- `tests/test_go_race.sh` — 53/52/52s → worst 53 → 55 + 5s margin → row **60s** (new file).
- `EXPECTED_TOTAL` 1825 → **1885**, recomputed from the table rather than hand-adjusted.

## Verification

- `go test -count=1 ./internal/gitcli/` — ok
- `go vet ./...` — clean; `gofmt -l` — clean
- `scripts/run-tests.sh` — `SUITE files=115 passed=115 failed=0 asserts=9300 wall=197s`, `SUITE_EXIT=0`

## Follow-ups

- None minted. All ten review findings were fixed in-branch (dispositions in the PR body). The
  `test_sync_agents_runners` budget overage is pre-existing and unrelated to this change's subject
  matter; it is surfaced above as a human verify item rather than captured as work this change
  discovered.
