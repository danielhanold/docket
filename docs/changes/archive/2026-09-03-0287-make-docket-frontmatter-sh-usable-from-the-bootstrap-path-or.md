---
id: 287
slug: make-docket-frontmatter-sh-usable-from-the-bootstrap-path-or
title: 'Make docket-frontmatter.sh usable from the bootstrap path, or split a Bash 3.2-safe core out of it'
status: 'killed'
priority: medium
type: refactor
created: 2026-08-11
updated: '2026-09-03'
depends_on: []
related: []
discovered_from: [208]
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

**Trigger** — surfaced while building change 0208, extracting a shared `worktree-scope:` frontmatter reader into `scripts/lib/docket-agent-scope.sh`. The obvious implementation was to delegate to `docket-frontmatter.sh`'s `fm_field`, the repo's canonical anchored frontmatter reader. It was built that way first and broke a real guard: `tests/test_sync_agents_defaults.sh`'s "0051 rider: empty scan_dirs run succeeds under /bin/bash" runs `sync-agents.sh` under macOS system Bash 3.2, where `declare -gA` in `docket-frontmatter.sh` errors at source time and `set -e` aborts the generator.

**Opportunity** — docket has one canonical anchored frontmatter reader that the bootstrap-path scripts cannot use. Every script that must run under system Bash 3.2 — `sync-agents.sh` today, and anything `install.sh` reaches before `DOCKET_BASH_PATH` is resolved — has to hand-roll its own parser instead, which is exactly the duplicated-extraction hazard the canonical reader exists to remove. Either make `docket-frontmatter.sh` bootstrap-safe (guard or defer the Bash 4 constructs so sourcing it under 3.2 is inert rather than fatal), or split a 3.2-safe core out of it that both tiers share.

**Independent value** — stands with change 0208 fully reverted. 0208's lib is one instance; the constraint binds every future bootstrap-path script that needs to read frontmatter, and today nothing states it except one comment in one file.

**Boundary** — the work is `docket-frontmatter.sh` plus whatever sourcing sites change, and a test that sources the lib under a real 3.2 interpreter so the property is guarded rather than remembered. It deliberately leaves alone: which scripts must be 3.2-safe (an existing, separate decision), and the runtime-routing design that gives most scripts `DOCKET_BASH_PATH`.

**Reason for deferral** — 0208's branch is a delegation-gate hardening. Reworking a shared frontmatter library used across the repo, plus its interpreter-version test surface, is a different blast radius and would expand that branch well past its spec.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): superseded by the Go migration — scripts/lib/docket-frontmatter.sh is deleted; frontmatter is read by internal/document, with no Bash-version constraint.
