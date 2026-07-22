---
id: 114
slug: decide-the-repo-s-posture-on-line-number-comment-anchors
title: Decide the repo's posture on line-number comment anchors
status: done
priority: medium
created: 2026-07-20
updated: 2026-07-22
depends_on: []
related: []
discovered_from: [106]
adrs: [54]
spec: docs/superpowers/specs/2026-07-20-line-number-comment-anchors-design.md
plan: docs/superpowers/plans/2026-07-22-line-number-comment-anchors.md
results: docs/results/2026-07-22-decide-the-repo-s-posture-on-line-number-comment-anchors-results.md
trivial: false
auto_groomable: true
branch: feat/decide-the-repo-s-posture-on-line-number-comment-anchors
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/119
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-line-number-comment-anchors-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-line-number-comment-anchors-design.md) |
| Plan | [2026-07-22-line-number-comment-anchors.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-22-line-number-comment-anchors.md) |
| Results | [2026-07-22-decide-the-repo-s-posture-on-line-number-comment-anchors-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-22-decide-the-repo-s-posture-on-line-number-comment-anchors-results.md) |
| PR | [#119](https://github.com/danielhanold/docket/pull/119) |
| ADRs | [ADR-0054](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0054-cross-reference-anchor-style.md) |
<!-- docket:artifacts:end -->

## Why

Change 0106 exists because a code comment was the sole assertion of a behavioral property, and it
had already shipped that property backwards once (`a9da1e2`, caught only at the 0101 review). The
fix was a test fixture. But the replacement comment block 0106 landed in `scripts/docket-config.sh`
anchors its cross-references on hard line numbers (`:194`, `:201`) — reintroducing the same rot
vector one level up: any edit above those lines silently staled the pointer, with nothing to notice.

The 0106 whole-branch review raised this as a Minor finding and it was **considered and declined
in scope** for three stated reasons: it matches house style throughout the repo, it is comment-only,
and leaving `tests/test_docket_config.sh` byte-identical to its mutation-verified state was worth
more than the marginal improvement. The decline was explicitly scoped to that one block, with the
repo-wide version left open: "worth revisiting repo-wide rather than in this one block."

## What changes

**The survey ran, and it decided: convert, do not close.** 27 anchor references across 10 files;
in the surfaces this change actually touches, 26 refs across 9 files with 3 already stale (11.5%).
Every stale anchor but one points into a top-four-churn file (`docket-finalize-change/SKILL.md` at
54 commits/90d, `docket-status.sh` 27, `docket-config.sh` 23, `render-board.sh` 13).

**Re-measured at reconcile (2026-07-22): the guarded form's population has doubled, 13 → 26 lines
across 8 files**, as changes 0102/0104/0111/0112 landed. The rot argument is unchanged and the
verdict holds a fortiori — the idiom accretes faster than it rots, which is the case for the guard
rather than against it. Current counts and the re-verified stale list are in the spec's
*Reconcile — re-measured 2026-07-22* section.

Three things follow:

1. **Adopt the anchor idiom** — a cross-reference anchors on a symbol name or a verbatim-quoted
   clause, never on a line number. A quoted clause is greppable, so drift is mechanically visible;
   a line number is not checkable by anything.
2. **Convert the 26 in-scope refs** in `scripts/`, `skills/`, `tests/`, `agents/`, `cursor-rules/`,
   and the root `*.md` / `*.yml`. The stale ones are repointed to what the prose *means*, not
   re-anchored to whatever line they now hit.
3. **Add `tests/test_comment_anchor_style.sh`** — a partial guard, deliberately. It enforces the
   explicit-file form (`<file>.<ext>:<N>`) only, the one predicate measured false-positive-free
   (13/13). The bare-`:N` and prose-`line N` forms carry 32–85% and 60% false-positive rates with
   no exception path, so they are converted but left to the convention plus review, with the rule
   documented in `AGENTS.md`. A new ADR records the posture and why the guard stops where it does.

Frozen and metadata-branch surfaces are excluded — `docs/results/`, `docs/changes/archive/`,
`docs/superpowers/specs/` (213 refs, point-in-time records), `docs/adrs/` (Accepted ADRs are
immutable, so a guard cannot demand a repair the convention forbids), and `docs/changes/active/`
(absent from `origin/main` — structurally invisible to the suite).

## Out of scope

- Rewriting `tests/test_docket_config.sh` or the 0106 fixtures beyond their anchor comments.
- Any behavioral change to `scripts/docket-config.sh`.
- The excluded doc surfaces listed above.
- Guarding the bare-`:N` and prose-`line N` forms.

## Open questions

Both resolved by the survey; see the spec for the evidence.

- ~~How many sites are there?~~ **27 across 10 files** (26 in scope), 11.5% in-scope rot rate,
  concentrated in the highest-churn targets. Dozens, not one or two — so: a sweep.
- ~~Is there a stable-anchor idiom already in use to standardize on?~~ **Yes** — the symbol-name
  anchor (`main()`, `field()`, `renders_row()`) is used pervasively in the same comment blocks, and
  `scripts/board-checks.sh` already demonstrates the best form by quoting the target code inline.

One premise from the stub's `## Why` was **checked and found wrong**: the 0106 review declined
partly because line-number anchoring "matches house style throughout the repo." It does — 213 such
anchors sit in `docs/superpowers/specs/` alone. That claim is correct and is not the reason to act.
The reason to act is narrower and better evidenced: in *maintained source*, these anchors measurably
rot, fastest where the code moves fastest.

## Reconcile log

### 2026-07-22 — reconciled against current `origin/main`

Design **intact**; scope **grew**. Re-measured every quantitative claim the spec rests on.

1. **Guarded (explicit-file) population doubled: 13 → 26 lines, 8 files.** By file now:
   `tests/test_docket_example_yml.sh` 7, `tests/test_board_checks.sh` 5, `scripts/board-checks.sh` 5,
   `tests/test_docket_config.sh` 3, `scripts/github-mirror.md` 2, `.docket.example.yml` 2,
   `scripts/docket-config.sh` 1, `scripts/docket-config.md` 1. `test_docket_example_yml.sh` did not
   appear in the original survey at all — it arrived with 0102 and grew since. Still measured
   **false-positive-free**, so A3's clean-predicate claim survives on the larger population.
2. **Unguarded forms unchanged.** The prose form still measures 5 matches / 2 true (60% FP) exactly
   as specced, including the three `line 2` fixture references in `tests/test_render_board.sh` and
   the en-dash `lines 2–19` in `scripts/docket-status.md:30`. No re-derivation needed.
3. **All three in-scope stale anchors re-verified STILL stale, and the spec's repoint targets still
   hold**: #2 `github-mirror.md` → the real sites remain the arg-parse and the
   `${PROJECT_FLAG:+--project ...}` invocation; #3 `test_finalize_disposition.sh` → the delegation
   assert; #4 the bare `:297+` → the archive section, which `render-board.sh:297` still is not.
   Note #4's *referring* line drifted 133 → 135 under 0111 — the anchors moved while the change sat.
4. **A7's premise is corrected; its conclusion survives.** 0111 is not an ungroomed stub — it
   **merged** (archived 2026-07-21) and edited `scripts/board-checks.sh`, repointing its header at
   `BOARD_CHECK_IDS`. 0115/0116 are now groomed (specs set) but remain `proposed` with empty
   `branch:` — nothing in flight in any file this change touches. 0114 still lands first cleanly.
5. **New build trap.** `git grep -E` does **not** support `\b`; a `\b`-anchored pattern returns
   silently empty and reads as "zero violations." Encountered live during this reconcile. The guard
   must not use `\b`, and the spec's population assert is exactly what catches this class.
6. ADR ledger is at **0053**, so the new ADR mints as 0054. `relates_to` targets ADR-0031 and
   ADR-0050 both confirmed present and `Accepted`.
7. `tests/test_comment_anchor_style.sh` confirmed absent — unbuilt, as expected.

No scope dropped, nothing folded in from elsewhere. The conversion workload roughly doubles; the
guard, the ADR, and the `AGENTS.md` rule are unaffected in shape.
