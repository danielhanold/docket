---
id: 270
slug: machine-local-runner-config-is-unreachable-from-a-feature-wo
title: 'Machine-local runner config is unreachable from a feature worktree (opencode permissions locality)'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: []
discovered_from: [269]
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

**Trigger** — surfaced while reconciling change 0269 (decoupling the shim wrapper's own frontmatter
pin from the delegated child's). 0269's spec names this defect in its own `## Out of scope` as real,
independently reproducible, and needing its own change; nothing has filed it.

**Opportunity** — `runners.<name>.permissions` is resolved from config anchored at
`DOCKET_REPO_ROOT`, which `runner-dispatch.sh` sets to the `--worktree` anchor. A build worker is
dispatched with `--worktree <feature worktree>`, and `.docket.local.yml` is gitignored — so a freshly
created feature worktree carries no copy of it. A `permissions: auto-approve` grant written in the
main worktree therefore resolves back to the default `ask` inside the feature worktree, and the
opencode adapter refuses the dispatch. There is no mechanism today that makes a machine-local runner
grant reachable from the worktree a delegated worker actually runs in.

**Independent value** — stands entirely with 0269 reverted. 0269 repairs the shim's parent-side
frontmatter pin so the dispatch reaches `runner-dispatch.sh` at all; this defect fires strictly
downstream of that, in the adapter, and would still block every opencode build delegation on a
machine using `permissions: auto-approve`.

**Boundary** — the work is: decide and implement where a machine-local runner config is read from
when the anchor is a feature worktree (main-worktree fallback, an explicit inherit, or an exported
resolution), and cover it in the runner tests. It deliberately leaves alone the `--worktree` gates
themselves (ADR-0034 anchoring, the build-* requirement), the permission semantics of any adapter,
and the `runners:` config-reader duplication that change 0256 owns.

**Reason for deferral** — 0269 is scoped to `sync-agents.sh`'s shim emission plus two new
`runners.*` generation-time keys, and records an ADR about which harness must resolve a shim's
frontmatter pin. Fixing config locality means changing how `runner-dispatch.sh` anchors its config
layers at dispatch time — a different file, a different layer of the stack, and a change whose
correctness argument is about gitignored-file reachability rather than about wrapper generation.
Folding it in would put two unrelated failure modes behind one review and one PR.
