---
id: 270
slug: machine-local-runner-config-is-unreachable-from-a-feature-wo
title: 'Machine-local runner config is unreachable from a feature worktree (opencode permissions locality)'
status: proposed
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-09
depends_on: []
related: []
discovered_from: [269]
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

## Auto-groom blocked

**2026-08-09** — autonomous grooming abstains: the stub's factual premise fails reproduction, and
the honest dispositions (kill, or repurpose) are human-only verdicts.

**Undecidable decision** — whether this defect exists at all. The stub (inheriting 0269's spec
`## Out of scope`) claims `runners.<name>.permissions` is "resolved from config anchored at
`DOCKET_REPO_ROOT`", the `--worktree` anchor, so a main-worktree `.docket.local.yml` grant is
unreachable from a feature worktree. The code says otherwise: `runner-dispatch.sh`'s config-layer
loop iterates `"$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"` where
`REPO_ROOT` is `docket_main_worktree` — the MAIN worktree, never the anchor — and has been anchored
there since the loop's creation (change 0079; change 0206 only moved the `DOCKET_REPO_ROOT` export
to the anchor). The adapters resolve no config files: `runners/opencode.sh` reads only the
already-exported `DOCKET_RUNNER_CFG_PERMISSIONS` and uses `DOCKET_REPO_ROOT` solely as `--dir`. An
empirical probe (fixture repo, grant in the main worktree's gitignored `.docket.local.yml` only,
sibling feature worktree, env-dumping fake adapter, dispatch from inside the worktree with
`--worktree` set) exported `PERMISSIONS=auto-approve` — the grant IS reachable. An adversarial
critic pass independently attacked the non-reproduction claim (nested dispatch, XDG /
`DOCKET_HARNESS_ROOT`, hand invocation of the adapter, `--launch` detachment, worktree-list
ordering) and found no reproduction path; verdict sound on all counts.

**What context is missing** — 0269's spec calls the defect "real, independently reproducible" but
records no reproduction. Only the human who ran 0269 can say whether a real field refusal was
observed and, if so, what actually caused it (candidates: a quoted `"auto-approve"` value hitting
the adapter's unknown-value leg, a hand invocation of the adapter bypassing the facade so no config
was resolved at all, or a mistaken code reading when the spec was written).

**What a human should supply** — the original failure transcript or a confirmation that none
exists.

**Recommendation** — this change should probably be **killed** (premise disproven at
`runner-dispatch.sh`'s config loop and `runners/opencode.sh`'s env-only permissions read), or
**repurposed** into a small chore: a regression test pinning "a main-worktree `.docket.local.yml`
`runners.*` grant reaches a feature-worktree dispatch", plus correcting the stale mechanism claim
in `scripts/runners/opencode.md`'s env table. If repurposed, note the file collision with groomed
change 0208 (also edits `scripts/runner-dispatch.sh`; disjoint concern — input gates) as a
`related:` coupling at that point.
