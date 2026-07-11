# Slim docket-implement-next + small skills — results
Change: #55 · Branch: feat/slim-remaining-skills · PR: <set on open> · Plan: docs/superpowers/plans/2026-07-11-slim-remaining-skills.md · ADRs: none

## Verify (human)

- [ ] **The one named behavior delta:** both kill paths (docket-new-change proposed-kill, docket-implement-next reconcile-kill) now delegate to `references/terminal-close-out.md` and thereby gain its step-2 `## Artifacts` re-render. This is a **no-op for kills** (a killed change has no `plan:`/`results:` to re-point), and the reference now states that a no-diff re-render is success (never trips the skip-publish guard). Confirm you're comfortable with both kill callers running the identical shared sequence. Everything else is behavior-neutral.
- [ ] **Live smoke is at merge:** finalizing this PR exercises the slimmed `docket-finalize-change`… no — this change does not touch finalize. The read-only Step-0 smoke (`eval "$(…/docket-config.sh --export)"` → BOOTSTRAP → metadata-tree sync) of a slimmed skill was run during the build (spec verification step 4).

## Findings

- **Behavior-neutral, verified two ways:** a whole-branch review read old-vs-new for all five skills and found no invariant without a home (no Critical/Important; the only behavior change is the named kill-path re-render delta above); and the **full test suite is green (0 failures)**, confirmed by an independent re-run.
- **Sizes:** docket-implement-next 137→**107** (target ≤~100), docket-new-change 70→**59** (~55), docket-groom-next 77→**75** (~65), docket-adr 88→**86** (~78), docket-auto-groom 64→**62** (~58). implement-next/groom-next/adr land modestly over target: the review confirmed the residual is load-bearing/test-anchored content (selection bands, recap contract, the four ADR publish contracts, the SHA-compare + cross-tree narration), not un-cut narration — the spec's size estimates were optimistic. **Decision: accept the sizes; no further trim** (behavior-neutral > size target).
- **DRY win:** implement-next's three near-duplicate `render-change-links.sh` field-write paragraphs (steps 4/6/7) collapse to one named **field-write rule** in *Branch & metadata discipline* — the single surviving full invocation, which also makes the mechanical invariant more salient.
- **Sentinel re-points** were confined to genuinely-moved content: `test_closeout` (5 kill script-path asserts → the reference, relabeled `wiring(close-out ref)`) and `test_docket_metadata_branch` K3/K4 (main-mode "archive commit is itself the terminal record" phrasing + additive caller-pointer asserts). None weakened; the kill-wiring `terminal-publish` asserts still hold against the skills.

## Follow-ups

- None. This completes the #0053-family skill slimming (#0053 convention+status, #0054 finalize, #0055 implement-next + small skills).
