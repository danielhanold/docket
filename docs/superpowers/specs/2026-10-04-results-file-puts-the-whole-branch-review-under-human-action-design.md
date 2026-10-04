<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0498 — Results file puts the whole-branch review under Human actions and testing](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0498-results-file-puts-the-whole-branch-review-under-human-action.md)**
<!-- docket:backlink:end -->

# Results file: name the home for whole-branch review outcomes — design

## Problem

Implement-next writes whole-branch review findings into the results file, but no guidance names
the section they belong in. Two instructions put them there without a destination:

- `docket-implement-next` Step 6.5's checkpoint lifecycle — "(ii) after review returns, and again
  after fixes with their **actual** dispositions".
- `references/fix-loop.md`'s *Results-checkpoint linkage (change 0410)* paragraph — "the
  coordinator persists the returned review findings to the results artifact … The results file
  preserves the findings, their evidence, and their impact for the human."

The only placement rule is negative: the template's Known issues guidance says "A fixed finding
belongs here only when it explains a remaining risk or a consequential design decision" (change
0440). A fixed finding with no remaining risk therefore has no named home, and runs improvise:

- Change 0494's results filed `### Whole-branch review (deep tier) and fixes` under
  `## Human actions and testing`, making a one-optional-walkthrough section read as six items.
- Three results files since 2026-09-22 filed "Fixed after review" entries under
  `## Known issues and follow-ups`, against the template's own rule.

Meanwhile the PR body already carries the complete review disposition table — `fix-loop.md`'s
*Recording — the PR-body disposition table* names the PR body "the disposition table's durable
home". The full per-finding list in the final results file is a second copy of code-level detail
that change 0440 says belongs in the PR.

The results-content validator (`ValidateResultsContent`, `internal/app/results_content.go`) does
not inspect which section holds what, so this is a guidance defect only.

## Decision

**The PR body holds the full review table; the final results file holds a one-line review
summary.**

1. **Final results file.**
   - `## Verification performed` carries **one line** for the whole-branch review: which review
     ran (the tier, or the custom review skill) and how its findings ended — e.g. "Whole-branch
     review (deep tier): 5 findings, all fixed in-branch; full table in the PR body." Do not link
     a PR number: final consolidation precedes the PR opening.
   - `## Known issues and follow-ups` carries an entry for every finding that is **not fixed** —
     disposition `deferred`, `reverted`, or `recorded` — and for every `reported` beyond-the-branch
     finding, plus any **fixed** finding that still explains a remaining risk or a consequential
     design decision (the existing 0440 rule, unchanged).
   - Fixed findings with no remaining risk appear in the final results file **only** through that
     summary line.
2. **During the build (halt insurance, 0410's reason — kept).** The review/fix checkpoints may
   still persist the full returned findings to the results file before and after the fix loop, so
   they survive a halt before the PR exists. **Final consolidation** (Step 6.5 checkpoint (iv))
   condenses them into the summary line and the Known issues entries above. State this as the
   explicit exception to the per-checkpoint rule "a checkpoint updates it, never truncates it".
3. **Human actions and testing is for human work only.** Add a class-level rule to its Step 6.5
   paragraph: the section holds only what a human should do or check — never a record of what the
   run already checked, which belongs under Verification performed. This covers review outcomes
   and any other performed-check record, without enumerating them.

## Edits

All prose; no Go behavior changes.

- `skills/docket-implement-next/results-template.md` — extend the `## Verification performed`
  angle-bracket guidance with the review summary line (decision 1); point fixed findings without
  remaining risk at that line.
- `skills/docket-implement-next/SKILL.md`, Step 6.5 — name the review homes (decision 1), add the
  final-consolidation condensation as the explicit exception to "never truncates" (decision 2), and
  add the Human-actions class rule (decision 3).
- `skills/docket-implement-next/references/fix-loop.md`, *Results-checkpoint linkage* — replace
  "The results file preserves the findings, their evidence, and their impact for the human" with
  the build-time-insurance / final-condensation split of decision 2. Keep the sentence that the
  PR body remains the disposition table's durable home.
- Regenerate the embedded asset tree (`internal/assets/embedded/tree/…`) through the existing
  `go generate` path (`cmd/genassets`), never by hand-copying.

## Testing

- Add a sentinel row to `internal/repoguard/prose_contracts_test.go` (house pattern: the
  `change_0440_*` rows) pinning, as `present` phrases, the new review-summary wording in the
  template, the condensation exception and the Human-actions class rule in Step 6.5, and the
  build-time/final split in `fix-loop.md`; pin the retired sentence "The results file preserves the
  findings, their evidence, and their impact for the human" as `absent` (assert-detects-removal).
- Mutation-test the row per AGENTS.md: strip each pinned phrase in turn and confirm the row
  reddens; restore and confirm green.
- Run the whole suite at the build gate.

## Out of scope

- Any validator, health check, or merge-boundary refusal on section membership — visibility via
  guidance only; no new blocking gate.
- Editing 0494's or any other merged results file — merged results are frozen build records.
- Adding, removing, or reordering results sections; changing the PR-body disposition table.
- An ADR — this is a prose placement rule inside the existing 0410/0440 results design.
