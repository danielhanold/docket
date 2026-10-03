---
id: 496
slug: 'drop-final-claimed-at-is-reported-repairable-but-nothing-can'
title: 'Add `docket repository repair` and stop flagging empty claimed_at'
status: 'proposed'
priority: 'low'
type: 'fix'
created: '2026-10-03'
updated: '2026-10-03'
depends_on: []
stacked_on:
related: [352, 377, 464]
discovered_from: [491]
adrs: []
spec: 'docs/superpowers/specs/2026-10-03-drop-final-claimed-at-is-reported-repairable-but-nothing-can-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-03-drop-final-claimed-at-is-reported-repairable-but-nothing-can-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-03-drop-final-claimed-at-is-reported-repairable-but-nothing-can-design.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` reports 269 `drop-final-claimed-at` warnings, each marked repairable with the remedy "Apply the previewed mechanical repair". Every one is a false alarm: all 269 archived records carry an *empty* `claimed_at:` key, which is docket's deliberate cleared form (the same bare-null form `branch:` and `pr:` take), and the record validator already reads it as "no claim". Only the check's repair planner (`planClaimedAt`) counts the bare key, so every closeout adds one more warning, and because a healthy repository with any finding exits 1, `repository check` can never come back clean.

The remedy is also wrong on two counts. On an already-migrated repository nothing applies frontmatter repairs: `migrate --repair-frontmatter --yes` prints `repository already migrated` and changes nothing. And the derived-view repair that *does* run there (board, `## Artifacts` blocks, ADR index) lives on `migrate` behind a frontmatter-named flag, so its remedies — "Run `docket repository migrate`" in `check`, `status`, and three skills — do nothing as typed. Repairing an already-migrated repository is not a migration and should not be spelled as one.

## What changes

- Stop the false alarm: `planClaimedAt` flags a final archived record only when `claimed_at` holds a value; an empty key is not a finding. The 269 warnings clear with no record rewrite, and closeout keeps its uniform cleared form.
- Add `docket repository repair` (catalog operation `repository.repair`): on a migrated repository it previews every mechanically repairable finding `check` reports — frontmatter fixes and board / `## Artifacts` / ADR-index re-renders — and with `--yes` applies them in one commit on `docket` under the existing pinned-revision lease. A repository that is not yet migrated is refused with a pointer to `docket repository migrate`.
- `migrate` only migrates: its already-migrated path stops repairing and names `docket repository repair` instead (even when `--repair-frontmatter` is passed). `--repair-frontmatter` keeps its legacy-migration meaning.
- Every remedy and skill line that sends a reader to `migrate` for these repairs names `docket repository repair` (skills by catalog id).
- Record the rule — repair on a migrated repository is `repository repair`; `migrate` only migrates — as an ADR.

## Out of scope

- Changing the frontmatter repair roster's membership or rules.
- Any new blocking gate: these findings stay visibility-only warnings.
- Changing closeout's cleared-field form or rewriting the 269 archived records.
- Changing the agent-autonomy posture for applying repairs; it carries over from today's `migrate --repair-frontmatter`.
- Broader README / guide alignment beyond the remedy sites (change 0464).
