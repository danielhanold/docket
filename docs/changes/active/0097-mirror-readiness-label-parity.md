---
id: 97
slug: mirror-readiness-label-parity
title: GitHub mirror readiness parity — readiness labels stop at `proposed`
status: in-progress
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: [87]
discovered_from: [87]
adrs: []
spec: docs/superpowers/specs/2026-07-19-mirror-readiness-label-parity-design.md
plan: docs/superpowers/plans/2026-07-19-mirror-readiness-label-parity.md
results:
trivial: false
auto_groomable:
branch: feat/mirror-readiness-label-parity
claimed_at: 2026-07-19T11:47:03Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-mirror-readiness-label-parity-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-mirror-readiness-label-parity-design.md) |
| Plan | [2026-07-19-mirror-readiness-label-parity.md](https://github.com/danielhanold/docket/blob/feat/mirror-readiness-label-parity/docs/superpowers/plans/2026-07-19-mirror-readiness-label-parity.md) |
<!-- docket:artifacts:end -->

## Why

`scripts/github-mirror.sh`'s `readiness_label` early-returns for any change that is not
`proposed`, so the `finalize-blocked` readiness state introduced by change 0087 never becomes a
`docket:readiness/` label. The inline board renders the cell and the digest surfaces it; the
mirror does not.

This is **not a regression** — the mirror never showed readiness for `implemented` changes. But
docket now has three projections of the same state (board, digest, mirror) and they disagree,
which is the kind of drift that gets noticed at the worst time: a maintainer watching the GitHub
Projects board sees nothing while the inline board says `finalize blocked — needs you`.

## What changes

Extend `readiness_label` in `scripts/github-mirror.sh` past its `proposed` early-return so an
`implemented` change carrying the `## Finalize blocked` marker maps to a
`docket:readiness/finalize-blocked` label, restoring parity with the inline board
(`finalize blocked — needs you`) and the digest (`finalize-blocked`).

**The stated rule (the scope question, settled):** the mirror does not invent its own readiness
policy — it mirrors the board/digest ownership `render-board.sh` already declares as the single
source. Readiness is owed for `proposed` (its four tokens) and for `implemented` (finalize-blocked);
no other status has a readiness notion. Any future readiness value is added to the owner
(`render-board.sh`) first, and the mirror follows the same status→owner shape — so the three
projections cannot drift again at the next value. `finalize_blocked` is already in scope
(`github-mirror.sh` sources the lib that defines it) and the new label self-provisions via the
existing idempotent `gh label create --force` path.

Design settled in the linked spec; see its `## Assumptions` for why a label (not a bespoke surface)
is the right form and why label pruning under the additive `--add-label` reconcile is a pre-existing
mirror-wide concern (affecting status labels too) left out of scope here.

## Out of scope

- Making the mirror two-way, or reading anything back from GitHub.
- Changing the inline board or digest readiness rendering (both already correct).
- Label pruning: the mirror's additive `--add-label` never removes a stale readiness label when a
  change leaves the state — identical to every existing derived label (status, priority, the four
  proposed-readiness tokens); a mirror-wide concern, not introduced here.

## Reconcile log

### 2026-07-19 — reconcile (implement-next claim)

Verified against current `origin/main` (tip `92131be`):

- **Spec accurate, code unchanged.** `scripts/github-mirror.sh:124` still carries the
  `[ "$status" = "proposed" ] || return 0` early-return the spec cites; the `readiness_label`
  `case "$tok"` block (lines 126–133) is exactly as the spec quotes. `labels_for` (line 177) already
  calls `readiness_label "$f" "$status"` and emits non-empty output — no caller change needed.
- **Owner + lib confirmed.** `render-board.sh`'s `digest_readiness` (lines 91–98) declares the
  status→owner rule the spec adopts (`proposed` via `readiness()`, `implemented` via
  `finalize_blocked()`, every other status `-`); both `readiness()` and `finalize_blocked()` live in
  `lib/docket-frontmatter.sh` (lines 94, 107), which `github-mirror.sh` already sources (line 80). The
  `docket:readiness/finalize-blocked` label self-provisions via the existing idempotent
  `gh label create --force` path.
- **Test seam present.** `tests/test_github_mirror.sh` uses a `--dry-run` + mock-`gh` fixture and
  already builds an `implemented` change (`0013-target.md`) with no `## Finalize blocked` marker —
  ready to serve as the "no readiness label" regression case; a new marker-bearing `implemented`
  fixture covers the positive case.
- **Dependencies / relations.** `depends_on: []`; `related`/`discovered_from: [87]` — #0087
  (`headless-finalize-driver`) is `done`, archived 2026-07-19. No design-ahead gating.

No scope change, no obsolescence, no fundamental invalidation. Build proceeds on the spec as written.
