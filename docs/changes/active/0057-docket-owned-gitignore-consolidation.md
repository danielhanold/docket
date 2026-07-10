---
id: 57
slug: docket-owned-gitignore-consolidation
title: Fold the migration-time .gitignore entries into the managed docket:generated block
status: proposed
priority: low
created: 2026-07-10
updated: 2026-07-10
depends_on: [51]
related: [51]
adrs: [20]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

docket now writes `.gitignore` entries from two unrelated places with two different
lifecycles. `migrate-to-docket.sh` step 5 appends three bare, unmanaged lines once at
migration time (`.docket/`, `.worktrees/`, `.claude/settings.local.json`); change 0051's
managed `# docket:generated:start/end` block (owned by `sync-agents.sh`) covers the
machine-local generated agent artifacts and is self-healing — regenerated whenever missing
or stale. The bare entries have no such repair: if someone deletes the `.docket/` line,
nothing restores it, and an accidentally committed `.docket/` worktree is an ugly failure
mode. Daniel raised this while live-testing 0051 (2026-07-10): all docket-owned ignores
should plausibly live in the one marker-bounded, self-healing place.

## What changes

Fold the three migration-time entries into the managed `docket:generated` block (or
explicitly decide not to, recording why) so a single docket-owned, self-healing section of
`.gitignore` carries every ignore docket needs.

## Out of scope

- Any change to which agent artifacts are generated or where (ADR-0020 semantics stay).
- Ignoring anything beyond the three existing migration entries + the 0051 block patterns.

## Open questions

- The 0051 block is written only for agent-opted-in repos or repos carrying a
  `.docket.local.yml`; the three migration entries are needed by EVERY docket-mode repo.
  Either `migrate-to-docket.sh` seeds the block too (a second writer of a block
  `sync-agents.sh` currently owns exclusively — needs a shared emitter to avoid a second
  roster) or the block's write trigger widens (which would touch `.gitignore` in
  tracking-only repos, against the learned zero-surprise-writes posture from change 0048).
- Dedup/migration story for existing repos that already carry the bare entries outside the
  block: leave harmless duplicates, or remove the old lines when the block adopts them?
- `.claude/settings.local.json` is written by `ensure-claude-settings.sh`, not generation —
  does it belong in a "generated" block, or does the block need a broader name/section?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
