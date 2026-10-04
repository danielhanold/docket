<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0510 — Match reported follow-ups against proposed and deferred changes](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0510-match-reported-follow-ups-against-proposed-and-deferred-chan.md)**
<!-- docket:backlink:end -->
# Match reported follow-ups against proposed and deferred changes — Results

**Human action:** None required. The change is prose-only skill and guide wording, guarded by a new prose-contract test; an optional walkthrough is described below for anyone who wants to see the new verdicts on a real run.

## Outcome

Before this change, `docket-implement-next` listed out-of-scope follow-ups in a results file's `## Known issues and follow-ups` section and in its final report, but only linked an existing change when that change happened to be in its context. Most follow-ups reached the human unlinked, and a missed match turned into a duplicate change.

Now, at final results consolidation (and only there), the run reads every `proposed` and `deferred` change's `## Why` and `## What changes` sections and gives each out-of-scope follow-up one verdict: **Fits #N**, **Related to #N**, or **No existing change fits (checked K)**. Each verdict carries a next action matched to #N's state: edit a needs-grooming or groomed change through `docket-groom-next <N>` (revise), revive a deferred change first, or capture a new change for related or unmatched work. The check only recommends; it never edits, creates, revives, or kills another change, and a failed backlog read degrades to the old wording with a note under Verification performed, never a halt.

The wording landed in the implement-next skill (Step 6.5 *Backlog match*, with verdict pointers in Step 3, Step 6, and the final-report rule), `fix-loop.md`, the results template, the docket convention's results-shape paragraph, and the user guides `docs/guide/capturing-work.md` and `docs/guide/reviewing-before-the-human.md`.

One correction to the spec: it said the root `skills/` tree is a derived copy of the embedded tree. It is the other way round — root `skills/` is authored and `internal/assets/embedded/` is regenerated from it with `go generate ./internal/assets/` (checked by `TestEmbeddedMatchesAuthored`). The build edited root `skills/` and regenerated the bundle.

## Human actions and testing

### Optional — see the verdicts on the next real run

Useful if you want to confirm the model follows the new paragraph; no automated test can drive model judgment.

1. Let the next `docket-implement-next` run that reports follow-up work finish to its PR.
   Expected: each out-of-scope entry under `## Known issues and follow-ups` in its results file ends with one of the three verdicts and a matching next action, and the run's final report lists the same verdicts.
2. Pick one "Fits #N" verdict and open #N.
   Expected: #N is `proposed` or `deferred`, and its stated scope plausibly covers the follow-up.

## Verification performed

- New guard `TestFollowUpBacklogMatchDocContracts` (`internal/repoguard/prose_contracts_test.go`) pins the new clauses in all four skill files and rejects the retired "link an existing change when one is known" wording; each row was mutation-checked (restoring old wording or deleting a new clause turned it red).
- Size budgets for implement-next `SKILL.md`, `fix-loop.md`, and `results-template.md` were re-baselined to their measured counts in `internal/repoguard/budgets_test.go`.
- The first full-suite run failed only `TestCapabilitySurface`: the new paragraph added a sixth human-typed `docket change create` against a pin of five. The clause was reworded to point at Step 3's existing capture sentence instead of raising the pin; the next full-suite run passed.
- Whole-branch review (deep tier): 3 findings (1 important, 2 minor), all fixed in-branch — the user guides now describe folding a fitting follow-up into its change, the final-report rule asks for a verdict only when final consolidation produced one, and the groomed and deferred next actions name their typed paths.
- This change's own run reported no out-of-scope follow-up work, so no backlog verdicts appear here.
