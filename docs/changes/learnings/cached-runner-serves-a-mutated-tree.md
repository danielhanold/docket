---
slug: cached-runner-serves-a-mutated-tree
hook: "A test runner with a result cache can report a green PASS against a tree you just mutated — every mutation probe and manual re-verification must defeat the cache explicitly (Go: -count=1)."
topics: [testing, mutation, caching]
changes: [304, 332]
created: 2026-08-13
updated: 2026-08-20
promotion_state: candidate
promoted_to:
---

## Apply
Mutation testing rests on one observation: *I broke the thing, and the assert went red.* A runner
that caches results can hand back the **pre-mutation** verdict, so the observation is fabricated —
and it fabricates the reassuring direction. You conclude the guard is decoration when it is fine,
or (worse) you conclude a guard is fine when your mutation never ran at all.

Go's test cache is the sharp instance because its key does **not** cover everything the test
actually depends on: it keys on the package's own inputs, not on a binary that `TestMain` builds
from a *different* package's sources. Mutate `internal/app`, re-run `go test ./cmd/docket/`, and
the answer can be `ok  …  (cached)` — a stale pass reported in the same words as a real one, with
one parenthetical as the only tell.

**The rule:** any run whose purpose is to observe a change in outcome must defeat the cache.

- Go: `go test -count=1 ./...` for every mutation probe and every manual re-verification. Treat a
  bare `go test` as a *build-loop convenience*, never as evidence.
- Generally: before believing a probe, ask whether the runner has a cache and whether its key
  covers the file you edited. Anything the runner builds out-of-band — a binary, a fixture, a
  generated artifact — is exactly what the key will miss.
- Read `(cached)`, `up to date`, `nothing to do`, and their kin as **absence of evidence** in a
  mutation context, the same way an unexecuted sweep is ([[agent-shell-noop-reads-as-success]]).

This is the cache-shaped member of the family that already contains
[[mutation-target-needs-a-forced-exit]] and
[[assert-detects-removal-not-replacement]]: three different ways a mutation test reports a result
that was never actually produced by the mutated tree.

## War story
- 2026-08-13 (#304, PR #204) — The Go skeleton's `cmd/docket` tests build the binary in `TestMain`
  and exercise it end-to-end, so their real dependency is `internal/app` and `internal/cli`. During
  the review-fix loop a bare `go test ./cmd/docket/` returned `ok (cached)` against a tree that had
  just been mutated, briefly reading as "the guard does not fire". `-count=1` reproduced the
  expected red immediately. Recorded as a standing rule for changes 0305–0318, which all grow this
  same package set and will all be mutation-probed against it.
- 2026-08-20 (#332, PR #222) — Collapsing the four `-race` shards back into one whole-module
  `go test -race ./...` serial gate, review caught that the gate omitted `-count=1`: the race
  detector could serve a *cached* pass and silently skip re-executing against the current tree,
  so a real data race introduced later would read as green. The rule applies to the gate itself,
  not only to mutation probes — any run whose job is to observe the current tree's behavior must
  defeat the cache. Fixed to `go test -race -count=1 ./...` before merge.
