<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0440 — Make results artifacts readable and actionable](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-21-0440-make-results-artifacts-readable-and-actionable.md)**
<!-- docket:backlink:end -->
# Make results artifacts readable and actionable — Results

**Human action:** No required action. The change is covered by automated tests and a
regenerated-asset drift gate; an optional walkthrough that demonstrates the new
structural check is included below for anyone who wants to see it directly.

## Outcome

Docket's per-change "results" file is the human-facing close-out artifact a reviewer reads
to understand a delivered change. Before this change its template pushed the reader toward
Docket implementation trivia (a "Human testing" list, separate "Findings and limitations"
and "Follow-ups" sections) and gave a mid-level engineer little help deciding what, if
anything, they had to do.

This change adopts the approved reader-first specification for all *future* results
artifacts. Historical results are untouched. The new shape is:

- A **Human action statement** immediately after the title — one or two sentences that say
  whether a human needs to do anything before relying on the change. (This file's own
  statement above is an example.)
- **Outcome** — the problem, the delivered behavior, and any departure from the design,
  led by observable effects.
- **Human actions and testing** — items each labeled **Important** or **Optional**, with a
  reason, setup, concrete steps, expected results, and cleanup. Optional walkthroughs may
  exercise behavior that automated tests already cover.
- **Verification performed** — a concise account of checks actually run.
- **Known issues and follow-ups** — one plain-language section merging what used to be split
  across "Findings and limitations" and "Follow-ups".

Three surfaces were changed together so they cannot drift apart:

1. The canonical template `skills/docket-implement-next/results-template.md` was rewritten to
   the new reading order, and the generated copy under `internal/assets/embedded/tree/` was
   regenerated to match (a build-time drift gate fails if they diverge).
2. The authoring guidance — `docket-implement-next` Step 6.5 and the `docket-convention`
   "Results artifact shape and lifecycle" paragraph — was reconciled to describe the new
   sections and the action-statement requirement, and the old blanket rule that human
   testing may never cover automated behavior was relaxed to allow optional walkthroughs.
3. The shared Go validator `internal/app/results_content.go` gained a new final-boundary
   structural check: a results file is refused at the "implemented" transition unless it
   carries a substantive `**Human action:**` statement between the title and the first
   section. "Substantive" excludes an empty statement, a `None`/`N/A`-style filler body, and
   — after an in-review fix — an unfilled `<...>` template placeholder. Two new stable
   machine reasons (`results-action-statement-missing`, `results-action-statement-empty`)
   report the two ways it can fail. The check binds only at the final phase; a mid-build
   checkpoint attach is unaffected. All prior checks (required substantive `## Outcome`, no
   leftover placeholders, no empty or filler sections) are preserved.

The detection is keyed on the label-plus-colon *shape* (`**Human action:** …`,
`**Human action**: …`, and a plain `Human action: …` all match), never an enumerated list of
sentence spellings.

## Human actions and testing

### Optional — see the new structural check reject a bad action statement

Anyone curious can watch the validator enforce the new requirement directly. This exercises
behavior the automated tests already cover, so it is optional.

Prerequisites: a checkout of this branch and a working Go toolchain.

1. Run the focused validator tests:
   `go test ./internal/app/ -run TestValidateResultsContentActionStatement -count=1 -v`
   Expected: PASS, with named subtests covering an accepted statement, a missing statement
   (`results-action-statement-missing`), an empty statement, a filler statement, and an
   unfilled `<...>` placeholder (all `results-action-statement-empty`).
2. Inspect the new template to confirm the reading order:
   `sed -n '1,40p' skills/docket-implement-next/results-template.md`
   Expected: an `**Human action:**` line right after the H1, then `## Outcome`,
   `## Human actions and testing`, `## Verification performed`, `## Known issues and follow-ups`.

No cleanup needed (read-only).

## Verification performed

- Full suite `go run ./cmd/docket development test` driven through the native gate driver:
  green at the final head (`e1ba11f4`). It was also green after the four build tasks
  (`ea775fea`) and re-run green after the two review fixes.
- `internal/repoguard` prose-contract sentinels (`change_0440_*`) and skill-size budgets
  green: they assert the new section headings and action-statement phrase are present and the
  removed headings absent (guarding against silent regression of the rewrite).
- Embedded-asset drift gate `go run ./cmd/genassets -repo . -check`: clean — the generated
  tree is byte-identical to the edited skill sources.
- The new validator check was mutation-tested: removing the check reddens its tests; removing
  only the scaffold/empty term reddens exactly the empty/filler/placeholder cases.
- Whole-branch review (standard rung) returned two findings, both fixed in-branch (see
  below); no re-review round is run by design.

## Known issues and follow-ups

### Review findings, both fixed in-branch

- The final-phase check initially accepted the *unfilled* template placeholder
  (`**Human action:** <Whether human action is needed, …>`) as a valid statement, so the
  headline requirement could be met by leftover scaffolding. Fixed (commit `3c1b842c`) by
  factoring the angle-bracket-scaffold shape rule into a shared helper and treating a
  scaffold body as non-substantive; a regression test and mutation evidence were added.
  Confirmed resolved.
- The `isResultsPlaceholderLine` doc comment cited example placeholder phrases the rewritten
  template no longer emits. Refreshed to current phrases (commit `e1ba11f4`). Cosmetic;
  confirmed resolved.

No open issues. No follow-up work was surfaced that falls outside this change.
