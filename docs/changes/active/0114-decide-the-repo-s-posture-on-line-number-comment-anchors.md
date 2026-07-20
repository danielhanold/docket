---
id: 114
slug: decide-the-repo-s-posture-on-line-number-comment-anchors
title: Decide the repo's posture on line-number comment anchors
status: proposed
priority: medium
created: 2026-07-20
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [106]
adrs: []
spec: docs/superpowers/specs/2026-07-20-line-number-comment-anchors-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-20-line-number-comment-anchors-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-20-line-number-comment-anchors-design.md) |
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
