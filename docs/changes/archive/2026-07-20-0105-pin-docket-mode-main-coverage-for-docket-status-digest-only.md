---
id: 105
slug: pin-docket-mode-main-coverage-for-docket-status-digest-only
title: Pin DOCKET_MODE=main coverage for docket-status --digest-only
status: killed
priority: medium
created: 2026-07-19
updated: 2026-07-20
depends_on: []
related: []
discovered_from: [94]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0094 shipped `docket-status --digest-only` as a write-free selection read, with its own
fail-closed gates because it deliberately skips `docket_preflight` (ADR-0047).

**The filed premise ("its `DOCKET_MODE=main` path is unfixtured") was falsified during grooming**
(2026-07-19, `docket-auto-groom` + critic). Two findings, both verified against the running code:

1. **A main-mode fixture already exists** — `tests/test_docket_status.sh:2645-2705`, the
   `--digest-only` real-git fixture, exports `DOCKET_MODE=main` / `METADATA_WORKTREE=.`, runs with
   real `git`, and already pins two of the three properties this change was filed to add: primary-tree
   resolution (it finds and emits `change 60` from the primary tree) and the write-free contract
   (BOARD.md byte-unchanged, working tree clean, HEAD unmoved). The subdirectory-anchoring test at
   ~2715-2730 reuses it.
2. **`DOCKET_MODE` is inert on this path, so mode-specific coverage is not buildable.**
   `docket_metadata_worktree` (`scripts/lib/docket-root.sh:52-58`) reads `DOCKET_MODE` *only* to pick
   a default when `METADATA_WORKTREE` is unset — and `docket-config.sh:180-181` sets that key
   unconditionally in both modes, so the default branch is unreachable in production. The only other
   `DOCKET_MODE` reader in `docket-status.sh` is `health_checks` (line 622), which `--digest-only`
   never reaches (`main()` short-circuits at lines 791-793). Mutation-proved: flipping the fixture to
   `DOCKET_MODE=docket` yields byte-identical stdout and rc=0.

3. **The `ready` line's grammar is already asserted** — `tests/test_docket_status.sh:2617`
   (`grep -qE "^ready( [0-9]+)*$"`), with ordering at 2620 and the empty-backlog case at 2804. That
   assert sits on the spy-git fixture (`DOCKET_MODE=docket`), but by finding 2 the mode framing is
   not a real partition, so it is not a *different* test from a hypothetical main-mode one.

What is left is small and its value is contested — see `## Auto-groom blocked`.

## What changes

**Needs a human decision on scope before this is buildable.** The originally filed work (add
main-mode coverage for three properties) is two-thirds already done and, per finding 2, not
buildable as *mode* coverage at all. Two candidate residuals are set out in `## Auto-groom blocked`.

## Out of scope

Any behavior change to `--digest-only` itself — this was filed as coverage for what already ships.

## Auto-groom blocked

**2026-07-19** — `docket-auto-groom` groomed this stub to a `trivial` verdict, failed the critic gate
after the one permitted revision round, and abstained. No spec emitted, `trivial` reverted to false.
The findings above are verified and stand; what follows is what a human needs to settle.

### Why this abstained

The change shrank under investigation, twice, and the second shrink put it below the size where
"should this exist at all?" is answerable without you.

- Filed as: add main-mode coverage for three properties.
- After finding 1: two of the three are already covered.
- After finding 2: the third cannot be covered *as main-mode coverage* — `DOCKET_MODE` is inert on
  this path, mutation-proved.
- After finding 3: the `ready` grammar is already pinned at 2617, so a new assert would be largely
  redundant.

### The decision you need to make

**Option A — reduce to label hygiene.** Correct the two mislabelled assert descriptions at
`tests/test_docket_status.sh:2696` and `2726`; drop everything else. Both say "emits the ready line"
and both actually `grep -qF "change 60 …"` — the change line. A label that is a receipt for a check
that is not happening is worth removing, but this is a two-line edit.

**Option B — add one non-redundant assert.** There is a genuine claim 2617 does not make: that the
`ready` line survives the **real-git anchoring path** (real `git`, `METADATA_WORKTREE=.`, invoked
from a subdirectory), where `docket_anchor_path` does real work — as against 2617's spy-git fixture,
where it takes its soft not-a-repo fallback. The expected line is exactly `ready 60`. This would
have to be labelled as real-git-anchoring passthrough, **never** as main-mode coverage.

**Option C — kill it.** Defensible if label hygiene plus one passthrough assert is not worth a PR.
Kill is never autonomous, so it stays here for you.

### Two things to fix whichever option you take

1. **The title and slug still assert the falsified premise.** "Pin DOCKET_MODE=main coverage for
   docket-status --digest-only" becomes the branch name, PR title, and commit subject — permanently
   asserting mode coverage that finding 2 shows is not buildable. Retitle before building.
2. **Do not make the fixture mode-discriminating by omitting `METADATA_WORKTREE`.** That tests a
   config `docket-config.sh` cannot emit, barred by the fixture-faithfulness rule at
   `tests/test_docket_status.sh:250-255`. Note also that `tests/test_docket_root.sh:77-78` is *not*
   a main-mode-discriminating assert (it sets `METADATA_WORKTREE` explicitly, so `DOCKET_MODE` is
   inert there too); the only genuinely mode-discriminating assert in that file is 79-80, the
   docket-mode default, and **no main-mode counterpart exists**. Adding one is a real option, but it
   is a different change from this one.

## Why killed

Premise falsified: the DOCKET_MODE=main path for --digest-only is already fixtured at tests/test_docket_status.sh:2645-2705 against a real git repo, and DOCKET_MODE is mutation-proved inert on this path, so mode-specific coverage is not buildable. Residual was two mislabelled assert descriptions.
