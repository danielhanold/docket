<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0447 — repository check flags docket's own single-quoted frontmatter as needing manual review](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0447-repository-check-flags-docket-s-own-single-quoted-frontmatte.md)**
<!-- docket:backlink:end -->
# repository check flags docket's own single-quoted frontmatter as needing manual review — Results

**Human action:** None required. Optionally rebuild the installed `docket` binary after merge (the usual post-merge rebuild) and confirm `docket repository check` no longer lists `frontmatter-manual-review` findings.

## Outcome

`docket repository check` used to report every correctly quoted string field in a change record (`slug: 'x'`, `title: '…'`, `type: 'fix'`) as a `frontmatter-manual-review` error that no command could clear. The checker compared the raw quoted bytes with the decoded value, so any quoting looked "ambiguous". On the live corpus that was 921 false findings, growing by about three per new change.

The checker now parses each value on its own and treats a token that is exactly one single- or double-quoted YAML string as well-formed, so it reports nothing. Bare values that change meaning when parsed (for example an unquoted `yes` or `true`) are still reported exactly as before, and malformed quoting is still rejected. The `repository migrate` preview loses its matching `[manual]` noise lines.

One departure from the design: a plain single parse of the token accepts trailing content (`'a' b` decodes as `a`), so the helper also requires the parser to reach end of input and checks the value is a plain string with no tag or anchor. This is stricter than the spec, not looser.

## Verification performed

- Live corpus, installed binary (before) versus a binary built from this branch (after): `frontmatter-manual-review` 921 → 0. The other finding families are unchanged: `artifact-links-stale` 275, `drop-terminal-claimed-at` 241, `local-metadata-diverged` 1, `metadata-worktree-dirty` 1.
- New tests: a direct helper table, a no-finding table for quoted tokens, still-flagged cases for bare scalars and malformed quoting, a writer/checker parity table over 39 adversarial strings, and an end-to-end `change.create` test asserting zero repair findings. Each was mutation-checked by its worker: reverting the fix, loosening the helper to a first-byte check, dropping the end-of-input check, and dropping the style check each turned the relevant tests red.
- Full suite: run at the build gate on the head that contains this file (see the PR's build-evidence block).

## Known issues and follow-ups

### Parity table's second leg is not independently mutation-proven

In each parity-table row the `document.New` assertion runs first and stops the row on failure, so reverting the fix shows the table going red but does not separately prove the `applyValue` (patch) leg would fail by itself. Impact is low: both legs go through the same writer quoting. Suggested action: none unless the two writer paths diverge.
