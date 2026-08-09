---
id: 272
slug: de-duplicate-the-gitignore-block-writer-s-second-copy-of-the
title: 'De-duplicate the gitignore-block writer''s second copy of the write orchestration'
status: proposed
priority: low
type: fix
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [242]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while fixing a review finding on change 0242. That finding was that
`ensure_managed_block` in `scripts/lib/docket-gitignore-block.sh` returns the status word `wrote`
even when its output redirect fails, so a caller logs "wrote/updated … — COMMIT THIS" for a file it
never wrote. Fixing it exposed that `ensure_docket_gitignore_block`, in the same library, carries
its **own un-refactored copy** of that write orchestration and still has the identical latent
false-success.

**Opportunity** — one write path in the library, rather than two copies that must be remembered
together. Either fold `ensure_docket_gitignore_block`'s orchestration onto the now-corrected
`ensure_managed_block`, or give it the same redirect-status keying and a `failed` word its caller
handles.

**Independent value** — stands with 0242 fully reverted: the duplicate predates that change, and
the defect is reachable on its own path (the managed `.gitignore` block) whenever the target is
unwritable — a read-only checkout, a permissions problem, a full disk. The library is shared by
`migrate-to-docket.sh`, `docket-config.sh --bootstrap`, and `sync-agents.sh`, so a false "COMMIT
THIS" there misleads at setup time, when the user has the least context to catch it.

**Boundary** — `scripts/lib/docket-gitignore-block.sh` and its own test file only. It stops at the
`.gitignore` writer: `ensure_managed_block` was already corrected by 0242 and is not reopened, and
no caller's own diagnostics or `case` vocabulary are redesigned beyond accepting the new word.

**Reason for deferral** — 0242's review finding was scoped to the generic helper, whose only
executable caller is the dispatch-surface writer it introduced. The `.gitignore` path has different
callers (including the two setup scripts), a different test file, and a different blast radius at
first-run bootstrap; pulling it into a branch about the Claude run-completion gate would expand that
branch's scope into setup-path behavior it otherwise never touches.
