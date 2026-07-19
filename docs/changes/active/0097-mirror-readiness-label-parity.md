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
plan:
results:
trivial: false
auto_groomable:
branch: feat/mirror-readiness-label-parity
claimed_at: 2026-07-19T11:45:13Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-19-mirror-readiness-label-parity-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-19-mirror-readiness-label-parity-design.md) |
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
