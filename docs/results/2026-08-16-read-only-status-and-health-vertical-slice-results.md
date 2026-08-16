<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0310 — Read-only status and health vertical slice](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0310-read-only-status-and-health-vertical-slice.md)**
<!-- docket:backlink:end -->

# Read-only status and health vertical slice — results
Change: #0310 · Branch: feat/read-only-status-and-health-vertical-slice · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-08-16-read-only-status-and-health-vertical-slice.md · ADRs: none

## Verify (human)

<!-- No genuinely-manual merge-gate checks: the slice is fully covered by automated Go tests
     (application fakes, real-git integration, golden presenters, the frozen v0.9.3 semantic corpus,
     and three mutation probes) plus the whole-suite gate. Nothing here requires human eyes at merge. -->
- [ ] (none)

## Findings

- **Two review findings, both fixed in-branch** (see the PR body's disposition table for the
  authoritative accounting):
  - *important* — the `testdata/repositories/v0.9.3/` tree now legitimately carries two provenance
    conventions (the change-0324 agent-defaults sidecar under the tree-root `PROVENANCE.md`, plus the
    new `status-corpus/PROVENANCE.md`), which contradicted `testdata/README.md`'s "one convention per
    tree, never a mix" rule. Fixed by amending `testdata/README.md` to carve out a documented
    multiple-independent-owners exception (commit `9e88a5c2`). The exception remains prose-only and
    unenforced by any test.
  - *minor* — `internal/app/status.go:matchesFilter` hand-reimplements domain's unexported selection
    predicate; the two copies agree today but could drift with no test to catch it. Fixed by adding a
    mutation-tested regression guard `TestStatusReadySubsetOfDisplayed` asserting the `ready ⊆ displayed`
    invariant (commit `2189c93f`). Production logic unchanged.

- **v0.9.3 corpus scope — a plan-vs-reality reconciliation.** The `v0.9.3` tag (peeled commit
  `dd742abd`) is docket's own metadata-branch content, which carries only terminal records — 9 archived
  changes + 5 Accepted ADRs, and **no active changes, no learnings ledger, no stacked changes**. The
  frozen semantic corpus therefore exercises the complete-corpus inventory + health/validation path
  with an empty active projection; the active-change readiness / ready-queue / effective-base semantics
  remain covered by the fake-reader application tests in `status_test.go`. Documented in
  `testdata/repositories/v0.9.3/status-corpus/PROVENANCE.md`. Additionally, the frozen production
  `.docket.yml` legitimately resolves with 4 config diagnostics (3 deferred-capability errors + 1
  deferred-setting notice), which the corpus test asserts as part of its 6-finding oracle.

## Follow-ups

- **Race-suite budget is tight.** `tests/test_go_race.sh` measured 57s standalone-serial against its
  60s **hard ceiling** (3s margin); `tests/test_go_toolchain.sh` was 41s vs its 45s row (4s margin).
  Neither triggered a re-budget or shard under the plan's rule, so nothing moved — but the next growth
  of the Go suite will likely force `test_go_race.sh` to shard (the shard scaffold and partition-guard
  extension are described in the plan's Task 7). Worth watching.
- The `testdata/README.md` multi-owner-provenance exception is documentation-only and unenforced;
  a future versioned tree could mix conventions without the disjoint-owner justification and no guard
  would flag it. Candidate for a future deterministic guard if the pattern recurs.
