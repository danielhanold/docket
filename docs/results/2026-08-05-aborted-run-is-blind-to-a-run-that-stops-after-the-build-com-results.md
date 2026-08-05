<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0211 — aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md)**
<!-- docket:backlink:end -->

# aborted-run leg C — built but not delivered — results
Change: #0211 · Branch: feat/aborted-run-is-blind-to-a-run-that-stops-after-the-build-com · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-05-aborted-run-built-but-not-delivered-leg-plan.md · ADRs: none

## Verify (human)

- [ ] **Decide the two `important` review findings** (both recorded in full under *Findings*). Neither is a defect in shipped behavior; both are judgment calls the reviewer deliberately left to you rather than auto-fixing:
  - the untested empty-`ar_bases` short-circuit (a coverage gap on the one predicate with no mutation),
  - leg C's assertive message register versus leg B's hedged one, on a predicate documented to fire on healthy runs.
- [ ] **Sanity-check leg C against this repo's own live state.** This run is itself the signature: run `docket.sh docket-status` while change 0211 sits `in-progress` with its PR open, and confirm leg C does NOT fire on it (`pr:` is set, so the leg short-circuits at the free frontmatter read). That is the cheapest end-to-end confirmation the `pr:` gate works against real data rather than fixtures.

## Findings

**Two plan defects caught during the build.** Both were in plan-supplied test code, both would have produced a green suite that proved less than it claimed — the `plan-supplied-test-code-is-unverified` learning, hit twice in one change.

1. **`RESULTS_DIR_REL` collision (Task 2, fixture 237).** The plan advanced the fixture's origin with a commit at `docs/results/2026-06-02-advance-results.md`. That path is inside `RESULTS_DIR_REL`, and leg A's results probe resolves "branch-only" against the same deliberately-stale local `main` — so the advancing file rode onto `feat/ar21` and fired **leg A** on the fixture. The silence assert is id-scoped, not leg-scoped, so it went `NOT OK`. Leg C itself was correctly silent the whole time. Fixed by advancing through a neutral path outside both `docs/results` and `docs/superpowers/plans`.

2. **The same collision in `$ARM`, plus a missing second advance (Task 3, fixtures 244/245).** Worse here than in (1): it would have made **mutation K vacuously true** — `has_finding "$armKout" aborted-run 245` would have passed on a leg-A finding even with the both-bases predicate dropped, i.e. the mutation guarding the change's central design decision would have proven nothing. Separately, fixture 244's single-commit advance left its tip carrying a real wall-clock date (2026-08) against `NOW_EPOCH` (2025-06), so it failed the idle floor and **mutation H could never have fired it**. Fixed with a two-commit advance (B1 fast-forwarded onto both bases for 244, B2 pushed to origin only for 245), three new precondition asserts pinning the base relationships, and a message-shape assert on mutation K so a future reintroduction of the collision cannot satisfy it via a leg-A finding.

**Non-vacuity was verified directly, not assumed.** Each mutation's `sed` was neutered to `cat` in a throwaway copy and the suite re-run: all six outcome asserts reddened while every "still fires" control stayed green. The idle floor was separately patched out of `board-checks.sh` itself to measure mutation G's blast radius — the new 201/214 exclusions, the 220/221/223/225 pins, and mutations A/B/D/D2's own asserts all reddened, confirming the shared `$ARM` repo's pre-existing mutations are genuinely affected by leg C and now pinned rather than left to the sign of a date delta.

**Review outcome — `docket-review-deep`, 0 blockers / 2 important / 7 minor.** Full text in the PR body. No ADR was warranted: every non-obvious decision (both-bases exclusion, the 2h floor, one-leg-two-messages, no new check-id) was settled in the spec's `## Assumptions` before the build, and the two deviations above are test mechanics, not architecture.

## Follow-ups

- **Change 0219** (minted at reconcile, `discovered_from: [211]`) — aborted-run's *sixth* signature: the PR opened and `pr:` written, then the run dies before `status: implemented`. Leg C's `pr:`-empty gate makes it invisible by design; leg B catches it at 12h. Its evidence is a manifest/GitHub comparison, and `board-checks.sh` is git-only by contract, so it needs a different oracle — a design question, not a fourth leg.
- The seven `minor` review findings are recorded in the PR body rather than filed as changes: each is a one-line edit to code this PR introduces (message wording, a stale cost count, a comment whose stated failure mode cannot occur, an absence assert narrower than its twin, "1 commits"). They belong to whoever merges this, not to a future change.
