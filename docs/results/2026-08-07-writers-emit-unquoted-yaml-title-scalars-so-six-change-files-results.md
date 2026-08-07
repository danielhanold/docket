<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0235 — Writers emit unquoted YAML title scalars, so six change files fail to parse](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-07-0235-writers-emit-unquoted-yaml-title-scalars-so-six-change-files.md)**
<!-- docket:backlink:end -->

# Writers emit unquoted YAML title scalars, so six change files fail to parse — results
Change: #0235 · Branch: feat/writers-emit-unquoted-yaml-title-scalars-so-six-change-files · PR: <url> · Plan: docs/superpowers/plans/2026-08-07-writers-emit-unquoted-yaml-title-scalars.md · ADRs: 71, 73

## Verify (human)

Every automated check is green; these are the ones no test in this repo can reach, because the
metadata branch is invisible to the hermetic suite (`metadata-branch-invisible-to-suite`).

- [ ] **The five repaired titles read correctly on the board today, pre-merge.** The repair landed on
      `docket` (commit `b8564803`), not on this branch. Open
      `docs/changes/BOARD.md` on `origin/docket` and confirm change 0121's row reads
      `The manifest's elsewhere: check proves…` with **one** apostrophe. A `''` anywhere in the board
      means a title was single-quoted where the currently-running reader cannot undouble it.
- [ ] **The three archived records match on both branches.** `0173`, `0211` and `0217` were
      republished onto `main` (three `terminal-publish` commits, `035e8eba..f88d34c0`). Confirm
      `git show origin/main:<path>` and `git show origin/docket:<path>` agree for each.
- [ ] **Merge order is not load-bearing, but the quoting form is.** The two apostrophe-bearing titles
      (0121, 0217) were deliberately **double**-quoted rather than single-quoted; see *Findings* for
      why. After this PR merges, the reader can undouble `''`, and both forms stay valid — nothing
      needs re-doing.

## Findings

**A new ADR came out of the review: ADR-0073** — *the scalar-quote predicate has no flow-collection
exemption*. The design spec gave the predicate an exemption so `discovered_from: [234]` would not be
flagged, and ADR-0071 recorded that it would "stay in the checker's predicate". The whole-branch
review showed the exemption was evaluated **first** and therefore suppressed all five legs, not just
the indicator leg: `[a title: with colon]` and `{a: b} tail}` both returned no finding, and both
*were* reported by the pre-change checker. No ordering rescues it — protecting a flow sequence
requires sitting above the indicator leg, while colon-space and trailing-colon sit above that, and a
flow map's `key: value` is a colon-space by construction. It was removed, and the domain contract
now carries the argument: the predicate asks whether a value is well-formed as a bare *scalar*, and a
flow collection is not a scalar. ADR-0071's Decision is untouched and still Accepted; only that one
consequence fell, recorded as a dated `## Update` note on it per the immutability rule.

**Review outcome: 13 findings, 0 blockers — 4 important, 9 minor, all 13 fixed in-branch** across 6
fix tasks. The disposition table is in the PR body. Two are worth reading here because they are
defects the *branch itself* introduced rather than pre-existing ones:

- The new `comment-introducer` leg was structurally unreachable for `blocked_by`, because
  `fm_field_raw` strips ` #…` before returning. A real file proves the miss: change 0044's
  `blocked_by: PR #69 is stale (…)` reached the predicate as the bare token `PR`. Fixed by giving
  that read a comment-strip-free `fm_field_verbatim` accessor.
- Unconditional title quoting turned a rare truncation into a routine one: `fm_field` on
  `title: 'clear finding #3 from review'` returned `'clear finding` — truncated **and** carrying a
  stray opening quote — which `render-artifact-backlink.sh` would have stamped into every artifact.
  Fixed by making the comment strip skip a quoted value.

**Three deliberate plan deviations, each a correction rather than a shortcut:**

1. **Task 6 passed `--outcome killed`, not the plan's hardcoded `done`,** when republishing change
   0217 — 0217 is `killed`. `--outcome` only feeds validation and the commit message; it never
   rewrites status, and the published bytes are identical either way. Passing `done` would have
   written a false statement into `main`'s history.
2. **Task 4 puts each `emit` arm's `;;` on its own line.** Load-bearing, and the reviewer confirmed
   it: the suite's in-file mutation deletes the whole colon-space `emit` line, and with an inline
   `;;` that leaves `colon-space)` immediately followed by `trailing-colon)` — a `case` syntax error
   that would kill the script, so the mutation would fail to run rather than go green.
3. **Task 1 set the repeated-apostrophe fixture at eight quotes, not the plan's nine.** The plan text
   was internally inconsistent; nine is not valid YAML (an odd interior run), so the figure could not
   have satisfied fixture, comment and assert at once.

**The repair's quoting form was chosen for the pre-merge reader, deliberately.** Titles with no
apostrophe (0173, 0211, 0234) were single-quoted; the two apostrophe-bearing ones (0121, 0217) were
**double**-quoted. The board is rendered by the *primary tree's* scripts, which track `origin/main`
and do not gain the `''`-undoubling leg until this PR merges — single-quoting those two would have
rendered `manifest''s` on the planning surface for as long as the PR sits open. Both forms are valid
YAML, the checker's skip leg accepts either, and both readers return the identical logical value.
This is a one-time choice about existing bytes and does not weaken the writer's unconditional
single-quote rule.

**Residual risk, stated plainly:** change 0173's defect is a *trailing colon*, which the live
two-leg checker running on `main` cannot detect at all. Its silence there comes from the skip leg
(the value is now quoted), not from detection — the trailing-colon leg only starts working when this
PR merges. Confidence in that one repair rests on byte-identical round-trip verification under both
the old and new readers, not on the live check.

**Live-tree backstop** (the hermetic suite structurally cannot provide it): `board-checks` over the
real metadata branch reports `no scalar-form findings`. Proven to be a real signal rather than a
mis-pointed path by restoring 0121's bare title in an isolated copy, watching the finding fire, and
re-applying the repair. Separately, the reviewer ran the new predicate over all 236 change files:
zero findings, with the single caveat noted above about change 0044's `blocked_by`, which this branch
now makes detectable.

## Follow-ups

- **Change 0240** — *Audit which frontmatter accessor each call site should use, now that three
  anchored read shapes exist.* Auto-captured from the review. `scripts/lib/docket-frontmatter.sh` now
  offers `fm_field` (quote- and comment-stripped), `fm_field_raw` (quotes kept, comment-stripped) and
  `fm_field_verbatim` (neither), and nothing states which shape a given call site should use — the
  difference stays invisible until a value happens to carry ` #` or a quote. This change fixed the two
  symptoms it hit and deliberately did not audit the rest.
- **Ten test files are flagged over budget** by the parallel runner (advisory, exit 0). Pre-existing
  timing, unrelated to this change, and unchanged by it.
