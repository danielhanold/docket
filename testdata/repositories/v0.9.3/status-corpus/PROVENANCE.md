# Provenance — `testdata/repositories/v0.9.3/status-corpus/`

- **Source repo:** `danielhanold/docket`
- **Tag:** `v0.9.3`
- **Peeled commit:** `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`
- **Date:** 2026-08-16
- **Redaction:** none — every file is byte-exact.

This directory is the frozen semantic corpus for the read-only `docket status`
operation (change 0310). It is a **representative slice** of the `v0.9.3` tag's
tree, not the whole tree. It coexists with — and never relabels or absorbs —
the agent-defaults sidecar that shares this versioned tree; that sidecar keeps
its own provenance in `../PROVENANCE.md`.

## Capture procedure (reproducible)

1. In a scratch clone of `danielhanold/docket`, assert the tag peels to the
   pinned commit and abort on any other value:

   ```
   test "$(git rev-parse 'v0.9.3^{commit}')" = dd742abd5e9fcdf8ffe78eb6f36a293410873bbf
   ```

2. Export each selected path from that commit with `git show`, so the bytes are
   exact and no working tree is involved:

   ```
   git show dd742abd:<path> > status-corpus/<path>
   ```

   Byte-exactness was verified by comparing `git hash-object` of each captured
   file against `git rev-parse dd742abd:<path>`.

## Selected paths

The `v0.9.3` tag names docket's `main`-branch content, which carries only
**terminal** planning records — archived changes and Accepted ADRs. It has **no
active changes and no learnings ledger** (those live on the `docket` metadata
branch, which this tag does not point at), and it carries **no stacked
changes** anywhere in the archive. The slice was chosen to be fully
self-consistent so every validation finding is enumerable by hand.

- `.docket.yml` — the repo's committed configuration at the commit. Docket
  runs in docket-mode (`metadata_branch: docket`, `integration_branch: main`).
- `docs/changes/archive/` — 9 archived changes (all `status: done`):
  ids `1, 2, 3, 4, 5, 6, 12, 13, 36`. Changes `1` and `2` each carry all three
  artifact links (`spec:`, `plan:`, `results:`).
- `docs/adrs/` — 5 Accepted ADRs: ids `1, 2, 3, 4, 5` (a **contiguous** 1..5
  range, so no ADR id-gap warnings can fire).

Every change/ADR cross-reference resolves **within** the slice EXCEPT change
`36`, whose `depends_on: [35]` and `related: [35]` both name change `35`, which
is deliberately excluded. This is the one intentional health defect the slice
carries, and it exercises both the readiness-gating (error) and associative
(warning) dangling-reference paths.

## Expected semantic outcomes (the oracle)

Derived **by hand from the frozen records**, asserted in
`internal/app/status_corpus_test.go`:

- **Summary counts:** total changes `9`, active `0`, displayed `0`, ready `0`,
  ADRs `5`, learnings `0`.
- **Ready queue / displayed changes:** empty — readiness and the ready queue
  are evaluated over active changes, of which this tree has none.
- **Record inventory (14):** the 9 changes by ascending id (location
  `archive`), then the 5 ADRs by ascending id (location `ledger`).
- **Health findings (6):**
  - three `deferred-capability-requested` **errors** — the frozen config
    requests `build.checkpoint`, `finalize.skip_results_only_delta`, and
    `terminal_publish`, capabilities the Go v1 config resolver defers;
  - one `deferred-setting` **notice** — `learnings.enabled`;
  - one `change-reference-dangling` **error** — change `36` `depends_on: [35]`;
  - one `change-reference-dangling` **warning** — change `36` `related: [35]`.
  - Error tally `4`, warning tally `1` (the notice is counted in neither).

The four config diagnostics reflect Go v1's implemented-capability set at
capture time. If the port later implements one of those capabilities, this
corpus's expected outcomes must be **re-derived by hand** (the frozen bytes do
not change; the resolver's verdict over them does) — that reconciliation is the
frozen-corpus discipline working as intended, not a regression.
