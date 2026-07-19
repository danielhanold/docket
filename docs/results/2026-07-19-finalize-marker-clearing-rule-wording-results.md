# Re-phrase the `## Finalize blocked` clearing rule — results

Change: #99 · Branch: feat/finalize-marker-clearing-rule-wording · PR: (see `pr:`) · Plan: docs/superpowers/plans/2026-07-19-finalize-marker-clearing-rule-wording.md · ADRs: none

## Verify (human)

Nothing interactive. The change is wording-only; `git diff origin/main..` touches two `skills/**/SKILL.md` files plus the plan and this file, and no `.sh` or `tests/` file. The three guard suites were run green at baseline, after the edit, and after the review fix.

## Findings

**A fourth reader of the marker, found by the whole-branch review.** The spec (and the first draft of
the shipped text) enumerated three readers of `## Finalize blocked`. There are four: the GitHub
mirror's readiness label, `scripts/github-mirror.sh:141`, added by change 0097. It matters more than
the other three, because it is the only reader that scans `archive/` at all
(`github-mirror.sh:109` finds across `active` **and** `archive`) — and it is safe for exactly the
reason the change argues, since `readiness_label`'s `case "$status"` has no `done` arm. It was the
strongest available evidence for the change's own thesis and it was the one initially missing.
Now named in the shipped bullet.

**Reader-roster staleness, twice in one change.** The stub's premise counted two readers; the
reconcile pass found change 0098 had added a third *the same day the spec was authored*; the final
review found a fourth that had been live since 0097. That is three different counts across one
change's lifetime, which is why the shipped text states the scoping **property** and marks the
parenthetical roster `today:` rather than resting the claim on an enumeration.

**No ADR.** The load-bearing decision (reject strip-on-archive; treat the `presence-encoded-state`
rule as discharged where every automated reader has stopped consulting the artifact) was
maintainer-settled at grooming, is recorded in the spec's *Explicitly decided against* and
*Reconciling with the `presence-encoded-state` learning* sections, and now lives in the skill prose
that owns the rule. An ADR would be a third copy.

**The `presence-encoded-state` finding stays as written.** It is a promotion-candidate whose hook
("every transition out of that state must remove the artifact") a naive reading would have made this
change violate. The shipped text resolves the tension via the finding's own design-time enumeration
— removal is the rule's usual *means*; the *end* is that no reader is left misinformed — rather than
by weakening the finding. Harvest note: if this change produces a learning, it belongs as a war
story **on that existing finding**, not as a new one.

## Follow-ups

Neither was minted as a stub — this repo runs `auto_capture: false`, so both are recorded here for a
human to file if wanted.

1. **The GitHub mirror never removes a readiness label (latent, pre-existing).** `github-mirror.sh:232`
   reconciles labels with `--add-label` and has no removal pass, so an issue that once carried
   `docket:readiness/finalize-blocked` keeps it after the change is archived to `done`. This is *not*
   a counterexample to this change's claim — the label is applied while the change is `implemented`
   and would survive strip-on-archive too — but it is a real one-way-mirror drift worth its own
   change. Noticed by the final whole-branch review.
2. **`README.md:184` still restates the clearing rule independently.** It is accurate and asserts
   nothing about archived files, so it was correctly left out of scope. But the "only the source
   should carry the phrasing" rationale that justified re-pointing the convention applies to it
   equally; a future docs-consolidation pass could re-point it the same way.

## Plan deviations

One, after the plan's single task was complete and reviewed clean: the final whole-branch review
returned one Important finding (the fourth reader) plus two same-line polish items, all fixed in a
single follow-on commit (`dc1cd18`) rather than by amending the task commit. The plan's Self-Review
also overclaimed slightly — it said the spec's *Explicitly decided against* section was carried into
the shipped text, but only the first of its two rejected options (strip-on-archive) is; the second
(a `## Finalize resolved` note at archive time) remains recorded in the spec only. Left as-is: the
spec is committed on the metadata branch and linked from the change's `spec:` field, and this repo's
convention keeps rejected-alternative rationale in specs rather than skill prose.
