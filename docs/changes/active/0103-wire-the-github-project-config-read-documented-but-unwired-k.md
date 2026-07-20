---
id: 103
slug: wire-the-github-project-config-read-documented-but-unwired-k
title: Wire the github_project config read (documented-but-unwired key)
status: proposed
priority: low
created: 2026-07-19
updated: 2026-07-19
depends_on: []
related: []
discovered_from: [101]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`github_project` is documented in `.docket.yml.example`, coordination-fenced in
`scripts/docket-config.sh:169`, and described in `docket-convention`'s config block as
"minted-and-written-back on first sync if unset" — but **no script reads it from config**.

Traced during change 0101's whole-branch review:

- `scripts/github-mirror.sh` resolves its board solely from `--project` /
  `--auto-create-project` (`github-mirror.sh:67`).
- `scripts/docket-status.sh` populates those flags only from its own CLI flags
  (`docket-status.sh:272`, `PROJECT_FLAG` / `AUTO_CREATE_PROJECT`), which **no skill passes**.
- The resolver never emits `GITHUB_PROJECT`; the fence loop is the key's only live effect.

So a user who follows the canonical reference — sets `board_surfaces: [inline, github]`, leaves
`github_project: auto` (or writes an explicit `{owner, number}`) — gets **issues-only mirroring**,
no Projects v2 board, and no diagnostic explaining why. The `project-minted` write-back path the
convention describes only fires under the opt-in `--auto-create-project` flag that nothing passes.

Change 0101 annotated the key as NOT-WIRED in `.docket.yml.example`, `scripts/github-mirror.md`,
and `scripts/docket-config.md` rather than silently shipping a false claim — accurate documentation
of a real gap, but the gap itself is untouched.

## What changes

Wire the config read end to end, or decide the key should not exist:

- Resolve `github_project` in `docket-config.sh` (emitting an export key) and have the Board pass
  forward it to `github-mirror.sh` as `--project` / `--auto-create-project`.
- `auto` must resolve to the same "no board configured" state as an absent key, and the
  `project-minted` write-back must **overwrite** a literal `auto` rather than mistake it for a
  minted board reference (ADR-0048 / change 0101 established the sentinel's meaning).
- Decide whether enabling `github` in `board_surfaces` should imply auto-create, or whether board
  creation stays behind an explicit opt-in — today the flag exists but is unreachable from config.
- Retire the NOT-WIRED annotations in the three files above once the read lands, and pin the
  behavior with a test.

`finalize.require_pr_approval` has the same shape (documented key, no working layer resolution) and
is tracked separately as change 0102; the two could reasonably be groomed together as one
"documented-but-unwired config keys" sweep.

## Out of scope

- Any change to the one-way mirror direction (change files stay the source of truth).
- Projects v2 field/column semantics beyond what `github-mirror.sh` already implements.
