# docket-status sweep — delegate archiving to archive-change.sh — results

Change: #0036 · Branch: feat/status-sweep-double-archive · PR: <set at PR open> · Plan: docs/superpowers/plans/2026-06-21-status-sweep-double-archive.md · ADRs: none

## Verify (human)

No interactive/manual checks required at the merge gate — the change is documentation-only (skill-body prose) and is fully covered by the sentinel suite. The merge gate's automated `tests/` run is the verification.

## Findings

- **Plan defect, caught and fixed during the build (commit `68060f8`).** The plan's verbatim Task-1 sentinel `! grep -qiE "non-zero . abort-and-report"` was self-defeating against the plan's own verbatim replacement prose: the new sweep prose intentionally contains the phrase "non-zero ⇒ abort-and-report" inside a *contrastive* sentence ("deliberately divergent from `docket-finalize-change`'s step 3, whose `non-zero ⇒ abort-and-report` fits a single-change close-out…"). A blunt absence-grep cannot tell a contrastive mention from an adopted posture. Replaced with a positive anchor — `grep -qiE "deliberately divergent from .?docket-finalize-change"` — which is satisfied by the correct prose and mutation-flips to NOT OK if the divergence framing is removed (i.e. if the sweep ever actually adopted abort-and-report). The contrastive sentence is correct and valuable and was kept.
  - Apply (generalizable): a `! grep` "must-not-say-X" sentinel is fragile when X legitimately appears in a *contrast/negation* clause of the same doc. Prefer a positive anchor on the divergence framing over a bare absence check.
- No new architectural decision — the change applies ADR-0012 (script-vs-model boundary), the #0026 archive-primitive extraction, and the #0035 renderer-ordering learning. `adrs: []` unchanged (spec assumption A8).

## Follow-ups

- **Minor (optional, from the whole-branch review):** the renderer-before-publish ordering sentinel (`tests/test_closeout.sh`) uses an existential `awk` order check (any render precedes any later publish), not a strict total order. Correct for the current single-render prose; if ever made airtight, anchor to the unique step-d phrasing ("before the publish below") instead of bare script-name proximity. Not blocking.
- The spec (assumption A2) noted a possible future refactor: extract a shared "archive + render + publish" wrapper that both `docket-status`'s sweep and `docket-finalize-change` step 3 call, instead of byte-aligned prose in two places. Deliberately out of scope here (larger blast radius); a candidate change if the two ever drift again.
