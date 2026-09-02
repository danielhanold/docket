<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0359 — Run gate gives up too soon](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0359-run-gate-gives-up-too-soon.md)**
<!-- docket:backlink:end -->
# Run gate gives up too soon — results

Change: #359 · Branch: fix/run-gate-gives-up-too-soon · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-09-02-run-gate-gives-up-too-soon.md · ADRs: 107 (supersedes 98)

## Verify (human)

Genuinely manual checks for the merge gate — the automated suite is green, but the run-gate
continuation behavior is a cross-harness dispatch contract that no in-repo test can exercise
end-to-end against a live agent harness. Each item is PENDING until a human confirms it.

- [ ] **Four-harness real-world re-probe.** With the rebuilt binary installed, drive an actual
      `docket-implement-next` run whose gated build detaches and stops at a WAITING handoff, then
      confirm the parent takeover + `gate-continue` path resumes it (no premature `gate-stop`) on
      each configured agent harness: claude, cursor, plus the two remaining generated surfaces. The
      change's thesis is "the run gate gives up too soon"; this probe is the only place the fix is
      observed against a real detached child rather than the integration test's simulated one.
- [ ] **Second-crash fail-closed check.** Confirm that a *second* detached crash within one run
      (after the first takeover has closed the outer scope) fails closed to `gate-stop
      gate-unavailable` and that `gate-before --resume` re-arms a fresh scope — the once-per-arming
      bound now documented in the spec (§5) and in code comments (see Findings).

## Findings

- **ADR-0107** — *Event-authorized parent takeover extends fingerprinted gate-drive ownership*
  (supersedes ADR-0098). Records the recovery-scope / event-authorized-takeover / `gate-continue`
  decision. Landed on `origin/docket`; 359's `adrs:` is `[24, 75, 95, 98, 107]`.
- **Deep review (3 findings, 0 blocker), all fixed in-branch before the PR opened:**
  - *important* — the outer recovery scope is single-use per arming; a second detached crash in one
    run fails closed rather than auto-continuing. Made explicit in the spec continuation section and
    in comments at `Takeover`/`gateOuterContinuation` (commit 4dd926d2 + spec commit on `docket`).
  - *minor* — `bindScopeChange` was production-dead; wired into the fresh-run attribution path so the
    outer scope binds once to the attributed change id (commit a3bb311c, RED-first tested).
  - *minor* — the repoguard direct-suite excuse was block-level; narrowed to line-level to mirror the
    sanctioned same-line `gate drive` recipe (commit e751572e).

## Follow-ups

- The `internal/workspace` `TestPrepareConcurrentDistinctTargets` parallel `-race` flake observed
  during this build is **already tracked** as in-flight change **#0373**
  (harden-integration-race-test-isolation-under-parallel-load) — no new capture needed. It was
  serial-confirmed green in isolation during this run; not a defect in 0359's diff.
