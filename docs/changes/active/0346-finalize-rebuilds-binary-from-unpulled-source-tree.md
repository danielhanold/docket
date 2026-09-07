---
id: 346
slug: finalize-rebuilds-binary-from-unpulled-source-tree
title: "Finalize's post-merge binary rebuild runs against an unpulled source tree"
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-25
updated: '2026-09-07'
depends_on: [388]
related: [283, 340, 392]
discovered_from: [342]
adrs: [99, 104]
spec: 'docs/superpowers/specs/2026-09-07-finalize-rebuilds-binary-from-unpulled-source-tree-design.md'
plan:
results:
trivial: false
auto_groomable:
branch: 'fix/finalize-rebuilds-binary-from-unpulled-source-tree'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-07T15:00:26Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-finalize-rebuilds-binary-from-unpulled-source-tree-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-finalize-rebuilds-binary-from-unpulled-source-tree-design.md) |
| ADRs | [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md) |
<!-- docket:artifacts:end -->

## Why

The repository requires a binary rebuild after a PR merges into main. During change 0342's closeout, installation succeeded from a primary checkout that still predated the merge: the binary's timestamp changed while its contents stayed stale.

Change 0388 now provides safe integration-checkout syncing at the end of finalize, but sync can deliberately skip an unsafe checkout or report a failure. The rebuild policy still needs to depend on successful sync and prove that the exact source used by install contains the merge. A successful install alone is insufficient; the installed binary's identity must also match that verified source.

## What changes

- Run repository-required rebuilds after finalize's existing end-of-run integration sync. Keep the shipped workflow generic and the concrete Docket rebuild requirement in AGENTS.md.
- Rebuild only after successful sync, a clean primary source checkout at the synced commit, and proof that the source contains every verified main merge handled by the run.
- Use the existing installation and version operations, then verify that source state stayed unchanged and the installed binary reports the same full, clean commit identity.
- Keep verified merged changes done when sync, rebuild, or verification cannot finish; report the outstanding binary rebuild and a valid recovery sequence separately.
- Add mutation-tested regression guards for the instruction contract and regenerate affected embedded skill assets.

## Out of scope

Removing the rebuild-after-merge requirement; reimplementing integration sync; new production CLI operations, flags, configuration keys, or installer modes; changing ordinary developer installs; temporary source checkouts or destructive source recovery; new lifecycle states, rollback of merges or installations, or edits to frozen records; imposing Docket binary installation on consuming repositories; restoring main-mode or Bash workflows.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-09-07

2026-09-07 — Reconciled against current main (0d1a7e1b) and docket metadata. Change 0388 is done: finalize's SKILL.md already runs the once-per-run `repository.sync-integration` suffix after batch closeout/cleanup (SKILL.md "After the batch's closeout and cleanup attempts ... run the `repository.sync-integration` operation once"). Related 0340 (build-identity stamping) and 0392 (installer-tolerant config read) are done, so their outputs are available as building blocks. The design's chosen approach (workflow + AGENTS.md policy correction plus Go regression guards in internal/repoguard, no new production CLI surface) remains valid and unchanged. Scope, relations (depends_on [388], related [283, 340, 392], discovered_from [342], adrs [99, 104]), and out-of-scope stand as authored; no obsolete work found, no fold-ins required.
