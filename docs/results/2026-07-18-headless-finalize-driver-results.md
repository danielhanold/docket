# Headless finalize — the finalize-side disposition contract — results
Change: #87 · Branch: feat/headless-finalize-driver · PR: (see change file `pr:`) · Plan: docs/superpowers/plans/2026-07-18-headless-finalize-driver-plan.md · ADRs: cites ADR-0043, produces none

## Verify (human)

The deliverable here is **skill prose an agent executes**, not code a suite can drive. The board
cell and the `has_section` fix are covered by tests; the contract itself is covered only by
sentinel greps, which sample rather than parse. These checks are what the suite cannot do:

- [ ] **Read the new contract end-to-end for executability** — `skills/docket-finalize-change/SKILL.md`,
      the *Selection* matrix + *Terminal disposition* + *`## Finalize blocked`* sections. The question
      is not "is it true" but "could an agent execute it without a second interpretation."
- [ ] **Confirm the branch-protection prerequisite still holds on this repo** — the drain assumes
      require-a-PR-with-zero-approvals (README §*Hands-off finalize*). If you re-tighten to require a
      review, every drain stops at `halted` on the first merge, by design.
- [ ] **Exercise one real drain before relying on it unattended** — `/loop docket-finalize-change <id>`
      against a single known-good change. `/loop` composition is confirmed on the *implement* side at
      CC 2.1.214; the finalize side has not been driven live, and `contended`/`halted` remain
      unexercised on both halves (they need, respectively, two racing agents and a real failure).

## Findings

**The whole-branch review found the feature would have shipped inert.** The `## Finalize blocked`
marker was fully specified as a *definition* — semantics, clearing rule, board cell, convention
entry — but no procedural path ever wrote it. The gate Flow didn't, the abort-and-report set
didn't, and *Where the reason surfaces* enumerated exactly what happens on an abort (relay
in-context, comment on the PR) and stopped. Every marker sentinel passed on the definition alone.
Fixed in `aba5867` by wiring the write into the surfacing step, and guarded by a new sentinel
anchored on that paragraph specifically — the pre-existing "selection SKIPS a marked change"
assert passes whether or not anything ever writes the marker, which is exactly how the gap
survived to review.

This is a **sentinel-coverage lesson, not a one-off**: grep sentinels over prose assert that a
*claim is present*, never that it is *reachable*. Where a contract has a producer and a consumer,
the sentinel set needs one assertion anchored on the producer.

**Three further presence-encoded-state gaps closed in the same commit.** The repo's
`presence-encoded-state` finding says every transition *out* of the state must remove the
artifact; the review found three transitions that didn't:
- A re-mark appended a **second** `## Finalize blocked` heading (retries are explicitly supported
  via the named-id override, so this was reachable). Now a re-mark replaces.
- A change carrying a stale marker that the human then merged **by hand** was skipped by finalize
  forever — never archived, board showing `finalize blocked — needs you` for an already-merged
  change. The skip is now scoped to *unmerged* candidates.
- The `drained`/`halted` boundary was decidable two ways for the same backlog (does a
  marker-skipped candidate count toward the non-empty set?). Now stated explicitly.

**A latent `has_section` bug was found and fixed during the build, unasked.** `has_section` was
`grep -qF` — an unanchored substring match — while its own docblock promised the literal *line*.
docket change files routinely *mention* these markers in prose (change 0087's own file mentions
both), so a prose mention read as a section. Verified live: the old predicate matches
`0083-terminal-publish-gap-detection.md` purely on a prose mention. It did not mislabel the live
board only because that change has a `spec:` and `readiness()` short-circuits before the call —
a landmine, not an active fire. Fixed to `grep -qxF` in `7c4c631`, tested at three levels for both
markers in both directions. This also forced the **bare-heading rule**: under a whole-line match a
dated heading (`## Finalize blocked — 2026-07-18`) is undetectable, so the heading is bare and the
date lives in the body, matching the live `## Auto-groom blocked` instances.

**Metadata-branch verification, done at build time** (per the `metadata-branch-invisible-to-suite`
finding — the hermetic suite runs against the integration-branch checkout and cannot see real
docket state). `render-board.sh --changes-dir .docket/docs/changes` was run read-only against the
live backlog: the `implemented` table emits the new `Readiness` column, the cell is empty for
#0078, `--format digest` stays in parity, and **no change on the live backlog is spuriously
marked**. The detection path was proven non-vacuous by mutation rather than assumed.

**Guards mutation-proven.** Seven new sentinels, each verified to redden when the clause it guards
is stripped (marker-write reachability · re-mark-replaces · already-merged-archived-regardless ·
skip-scoped-to-unmerged · both halves of the drained boundary · classifier-denial abort point ·
README prerequisite cross-link), then restored to `PASS`. The reviewer independently mutated the
pre-existing set, including reverting `has_section` to `grep -qF` (10 assertions fired across two
test files).

**No ADR produced.** The grooming anticipated none and that holds: every decision here is a
refinement of a contract that lives in the skill prose itself, which is the durable artifact. An
ADR would duplicate the contract rather than explain a force behind it. ADR-0043 is cited, not
amended.

## Deviations from the spec, recorded deliberately

The plan's own *Deviations* section covers only §7's test shape. Four further reversals were made
during the build and are recorded here rather than left in commit messages — a later reconcile
against this spec would otherwise "discover" them as contradictions:

1. **`CONFLICTING` deprioritizes rather than excludes** (reverses spec §3.3.3). Excluding it would
   strand a fixable PR as human-blocked, when the gate's `docket-rebase-resolver` usually resolves
   it. It now sorts *last* among eligible candidates — which is what mergeability ordering is for.
2. **`CONFLICTING` is not marked at selection time** (reverses spec §3.4 bullet 1). Same reason:
   marking happens where every other abort reason does, at the gate. **Consequence to note:** §3.4's
   stated rationale — "mark all CONFLICTING so the board surfaces *every* change needing attention"
   — is deliberately dropped. The board now surfaces at most the one change this run gated.
3. **`advanced` widened** to include archiving an already-merged PR. Under the spec's wording
   ("merged one change → closed out") a run that archives a human-merged PR had *no* defined
   disposition. Real close-out work ran; the driver should continue.
4. **The multi-candidate prompt is superseded on the driver path, and a named id overrides the
   marker skip.** The latter closes a genuine deadlock: a skipped change can never be finalized, so
   the clearing rule could never fire without an override.

## Follow-ups

- **GitHub mirror parity gap.** `scripts/github-mirror.sh` `readiness_label` early-returns for
  non-`proposed` changes, so `finalize-blocked` never becomes a `docket:readiness/` label. The board
  and the digest surface it; the mirror does not. Not a regression (the mirror never showed
  readiness for `implemented`), but the three projections now disagree — worth a stub.
- **No health check for a stale marker.** `scripts/board-checks.sh` gained nothing here. A marker
  whose cause the human fixed without merging and without naming the id sits on the board
  indefinitely with no advisory. `merge-gate-stall` is the obvious precedent.
- **Consider re-phrasing the clearing rule as "the marker never survives into `done`."** A
  successful finalize archives the change, which hides the cell anyway, so the rule as written has
  near-zero observable effect; the transitions that genuinely need clearing are the ones fixed
  above. Wording only — no behavior change intended.
