<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0138 — Board generator wraps each change title in literal double quotes](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0138-unquote-board-change-titles.md)**
<!-- docket:backlink:end -->

# Unquote board change titles — results
Change: #138 · Branch: feat/unquote-board-change-titles · Plan: docs/superpowers/plans/2026-07-24-unquote-board-change-titles.md · ADRs: 58

## Verify (human)

No interactive/manual checks required — the fix is covered end-to-end by automated tests, and the rendered board will pick up bare titles on the next board refresh. Nothing to run by hand at the merge gate.

## Findings

- **The spec's "every `field()` consumer benefits" premise was incomplete — caught by the full suite at the finish gate, not by reconcile or the whole-branch review.** After the reader fix (`field()` strips a matched surrounding quote pair), the full test suite went red on `tests/test_render_learnings_index.sh`. Root cause: `scripts/render-learnings-index.sh` reads the finding `hook` via `field()` and then runs its OWN full YAML unescaper `dequote()` (handles `\"`→`"`, `\\`→`\`, `''`→`'`, plus an escaped-closer guard `_dq_dquote_closer_is_real`). `dequote` requires the RAW quoted scalar — outer quotes intact — to detect the quote style and run its escaped-closer check; stripping the quotes in `field()` broke it (and could even mis-strip an escaped-closer like `"foo\"`). This is the ONE consumer in the repo that does its own decode on a `field()` result (audited every `field()`/`fm_field()` call site).
- **Resolution → ADR-0058 (two-tier readers).** Added `field_raw()` — the raw token, surrounding quotes intact (the pre-0138 `field()` behavior) — and redefined `field()` as `field_raw()` piped through the `_docket_unwrap_quotes` helper, so the raw read has a single definition. Repointed `render-learnings-index.sh`'s `hook` read at `field_raw`. Rule (recorded in ADR-0058): a consumer that does its own quote/escape decoding must read via `field_raw()`, never `field()`. Full suite back to 62/62 green.
- **Process note:** reconcile confirmed the six enumerated *title* consumers but trusted the spec's shared-path enumeration rather than independently auditing *all* `field()` consumers for post-decode behavior; the learnings `hook` consumer (a `field()` reader that dequotes) was outside the title enumeration. The full-suite finish gate is what caught it. (Related learnings: `verify-the-claim`, `foundational-test-discipline`.)

## Follow-ups

None warranting a new change. The near-identical `field()`/`field_raw()` pair is a deliberate two-tier contract (ADR-0058) guarded by `tests/test_docket_frontmatter.sh` (both tiers) and `tests/test_render_learnings_index.sh` (the hook behavior); a future "merge these two readers" simplification would re-introduce this regression and is explicitly warned against in the ADR. The spec's deferred full-YAML-unescaping in `field()` (Assumption 3) remains correctly out of scope — `field_raw()` + the consumer's own `dequote` cover the one place richer decoding is actually needed.
