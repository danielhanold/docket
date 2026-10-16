---
id: 539
slug: 'list-the-artifact-backlink-finding-codes-in-the-docket-statu'
title: 'Describe repository-check drift findings by family in the docket-status skill'
status: 'in-progress'
priority: 'low'
type: 'docs'
created: '2026-10-07'
updated: '2026-10-16'
depends_on: []
stacked_on:
related: [533]
discovered_from: [533]
adrs: []
spec:
plan: 'docs/superpowers/plans/2026-10-16-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md'
results:
trivial: true
auto_groomable:
branch_prefix:
branch: 'docs/list-the-artifact-backlink-finding-codes-in-the-docket-statu'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-16T12:09:39Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-10-16-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md](../../superpowers/plans/2026-10-16-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md) |
<!-- docket:artifacts:end -->

## Why

The docket-status skill's *Findings* paragraph (`skills/docket-status/SKILL.md`) has one sentence on derived-view drift: "Derived-view drift (`board-stale`, `adr-index-stale`, `artifact-links-stale`, and their `-malformed` forms) is reported by the `repository.check` operation and repaired through `repository.repair`." Tracing the code (groomed 2026-10-16) found that sentence is incomplete and partly wrong:

- It omits the three artifact-backlink codes change 533 added (`artifact-backlink-stale`, `-malformed`, `-shared`) and `artifact-links-missing`.
- It says all of them are repaired through `repository.repair`. They are not: `internal/reposetup/derived.go` marks only the `-stale`/`-missing` codes repairable. Every `-malformed` code and `artifact-backlink-shared` is manual review — a human edits a file first, then repair catches the views up.
- A hand-maintained code list drifts each time a code is added (this stub exists because it did). Since change 553, every finding carries its own plain-language remedy, so the skill no longer needs to restate per-code fixes.

The glossary's *Derived view / generated block / backlink* entry has the sibling gap: its example comment says `repository check` "reports records whose links are stale or absolute", which covers only the change record's links block, not the backlink at the top of a spec, plan, or results file.

## What changes

Trivial — no design question. Human chose the family-plus-remedy wording over a full code list (2026-10-16).

1. **`skills/docket-status/SKILL.md`, *Findings*:** replace the derived-view drift sentence with one that names the family, not the codes, and defers to each finding's remedy. Target wording (adjust to fit the paragraph):

   > Derived-view drift — the board, the ADR index, a change's `## Artifacts` block, or the back-link at the top of a spec, plan, or results file being out of date or broken — is reported by the `repository.check` operation. Each finding carries its own remedy: most clear with `repository.repair`; a broken marker, or a file linked from more than one change, needs a hand edit first. Relay the finding's remedy rather than telling the human to just run repair.

   Also check the skill's *Sweep posture* line ("a stale `## Artifacts` block surfaces later as an `artifact-links-stale` finding that the authorized `repository.repair` operation re-renders") stays accurate — it is (that code is repairable); leave it unless the new sentence makes it redundant.
2. **`docs/reference/glossary.md`, *Derived view / generated block / backlink*:** make the `docket repository check` example comment cover both directions, e.g. `# reports links blocks and back-links that are stale or broken`.
3. Regenerate embedded assets (`go generate ./internal/assets/`) and run the skill/doc drift guards and the suite.

## Out of scope

Changing any finding, its detection, its remedy text, or repair behavior; listing finding codes anywhere in the skills (the `schema` operation owns codes and shapes, each finding owns its remedy); the bash-upgrade guide's findings table, which is a deliberate subset for Bash repositories.

## Reconcile log

### 2026-10-16

2026-10-16: Re-traced against origin/main e99f310. The docket-status Findings sentence (line 124), the Sweep posture line, the glossary example comment, and the repairability split in internal/reposetup/derived.go are all as the groom described. Scope unchanged.
