---
id: 80
slug: link-skills-create-harness-dir
title: link-skills.sh creates a missing skills subdir when the harness is present
status: in-progress
priority: medium
created: 2026-07-15
updated: 2026-07-15
depends_on: []
related: [45]
adrs: []
spec:
plan:
results:
trivial: true
auto_groomable:
branch: feat/link-skills-create-harness-dir
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`link-skills.sh` symlinks docket's skills into each agent harness's global skills dir. Its guard
skips any harness whose skills dir does not already exist (`[ -d "$dir" ] || continue`, line 37),
with the stated intent "we never create a harness you don't use." But that guard checks the
**`skills` subdir** (e.g. `~/.cursor/skills`), not the **parent harness dir** (`~/.cursor/`). So a
user who *does* use a harness — the parent dir exists — but has never created its `skills`
subdirectory yet gets silently skipped: no docket skills are linked, and the script reports
`Created: 0` with no indication anything was missed.

This was reported for Cursor (`~/.cursor` present, `~/.cursor/skills` absent), but the guard is
uniform across all six listed harnesses (claude, codex, cursor, agents, kiro, windsurf) — claude
and codex hit the identical bug the moment their `skills` subdir happens to be absent.

## What changes

Change the guard's signal from "the skills subdir exists" to "the harness itself is present":

- For each harness skills dir, gate on the **parent** dir (`dirname "$dir"`, e.g. `~/.cursor/`).
  If the parent is absent, still `continue` — we never materialize a harness the user doesn't use.
- If the parent is present but the `skills` subdir is missing, `mkdir -p` it, then link as today.

The fix applies uniformly to all six harnesses (the reported Cursor case plus claude/codex and the
rest) because they all flow through the same loop.

Update `tests/test_link_skills.sh` to cover the new contract:

- A harness present with **no** `skills` subdir → the subdir is created and skills are linked.
- A **fully absent** harness (no parent dir) → still not created (the existing line-19 invariant
  holds, re-expressed against the parent dir).

## Out of scope

- Changing the list of supported harnesses, or how the symlink target is resolved.
- Any change to per-repo agent wrapper generation (`sync-agents.sh`) — this is the global
  skills-linking installer only.
- Auto-installing or detecting harnesses the user has never set up.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-07-15 — Reconciled against current `origin/main`. Verified the described bug is live and
  unchanged: `link-skills.sh` guard `[ -d "$dir" ] || continue` sits at **line 37**, gating on the
  `skills` subdir rather than the parent harness dir; `HARNESS_SKILL_DIRS` lists exactly the six
  harnesses named (`.claude .codex .cursor .agents .kiro .windsurf`). `tests/test_link_skills.sh`
  currently leaves `.cursor/.codex/.kiro/.windsurf` **fully absent** (no parent) and asserts "does
  NOT create an absent harness dir" — that invariant survives the new contract (parent-absent still
  `continue`s), and the new "parent present, skills subdir missing → create + link" case must be
  added. Related change 45 (`sync-agents.sh` per-repo wrapper generation) is out of scope and does
  not touch this installer — no overlap. No recent commit has pre-empted the fix. Scope unchanged;
  trivial (no spec). Build-ready as written.
