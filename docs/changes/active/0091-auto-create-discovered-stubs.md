---
id: 91
slug: auto-create-discovered-stubs
title: Auto-create discovered stubs — a config flag that turns mid-run findings into proposed changes
status: implemented
priority: medium
created: 2026-07-17
updated: 2026-07-18
depends_on: [90]
related: [90]
adrs: [19, 45, 46]
spec: docs/superpowers/specs/2026-07-17-auto-create-discovered-stubs-design.md
plan: docs/superpowers/plans/2026-07-18-auto-create-discovered-stubs.md
results: docs/results/2026-07-18-auto-create-discovered-stubs-results.md
trivial: false
auto_groomable: true
branch: feat/auto-create-discovered-stubs
claimed_at: 2026-07-18T23:20:33Z
pr: https://github.com/danielhanold/docket/pull/104
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-auto-create-discovered-stubs-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-auto-create-discovered-stubs-design.md) |
| Plan | [2026-07-18-auto-create-discovered-stubs.md](https://github.com/danielhanold/docket/blob/feat/auto-create-discovered-stubs/docs/superpowers/plans/2026-07-18-auto-create-discovered-stubs.md) |
| Results | [2026-07-18-auto-create-discovered-stubs-results.md](https://github.com/danielhanold/docket/blob/feat/auto-create-discovered-stubs/docs/results/2026-07-18-auto-create-discovered-stubs-results.md) |
| PR | [#104](https://github.com/danielhanold/docket/pull/104) |
| ADRs | [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0045](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0045-auto-capture-is-best-effort.md), [ADR-0046](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0046-cas-reset-hard-shared-worktree-tracked-clean-tree-precondition.md) |
<!-- docket:artifacts:end -->

## Why

Captured alongside #0090 (2026-07-17). Today, when an autonomous run surfaces follow-up work —
implement-next's reconcile/review notices an adjacent gap, a build discovers a latent bug, a
close-out finding implies a next step — the model *asks* the human whether to file it, or worse,
mentions it in prose that scrolls away. In an unattended run there is no human to ask, so
discovered work is routinely dropped on the floor. Beads' agents are simply told to capture
(`bd create`/`bd q` mid-task, with `discovered-from` provenance); the near-zero-friction capture
path is what makes agent-discovered work durable.

docket should have the same posture behind a flag: when enabled, a skill that identifies genuine
follow-up work mints a needs-brainstorm stub directly (with `discovered_from:` set, per #0090)
instead of asking. Stubs are cheap, reviewable markdown on the metadata branch — the human still
gates everything at groom time, so auto-creation adds no autonomy risk, only capture fidelity.

## What changes

- A new **boolean** config knob `auto_capture: true | false`, default `false` — **global-able**
  across all layers (repo `.docket.yml`, user-level global config, `.docket.local.yml`), classified
  by direct analogy to `auto_groom` under ADR-0019 (gates a local-run behavior producing ordinary
  backlog commits, never coordination state). Resolved with the same layered read as `auto_groom`
  and recorded in the authoritative fence table in `scripts/docket-config.md`.
- When enabled, the **autonomous single-change** skills — `docket-implement-next` (reconcile/review
  discoveries) and the `docket-finalize-change` / `docket-status` harvest (close-out findings) —
  mint `proposed` needs-brainstorm stubs for discovered work with `discovered_from:` populated (per
  #0090), instead of asking or mentioning. When disabled, today's ask-or-mention behavior is
  unchanged. `docket-auto-groom` is deliberately **not** a mint site (it would break its own
  provable-termination invariant and create an `auto_groom` × `auto_capture` growth loop);
  interactive skills already mint with a human present.
- The mint reuses `docket-new-change`'s id-allocation + CAS routine via a deterministic helper: the
  model decides *what* is material (a stub = distinct follow-up work that would be its own PR; not a
  learnings lesson, not current-change drift), the helper does the mechanical mint (ADR-0012).
- Guardrails against noise: the materiality bar above, a cheap active-slug dedup check, and a small
  hardcoded per-invocation cap (overflow surfaced in the run report, not dropped).
- Minting is a metadata-worktree write only — it never disturbs the running change's own
  claim/branch/PR state.
- Shipped end-to-end: the knob in `config.yml.example` + the `.docket.yml` schema block, README, and
  the relaxed convention prose.

## Out of scope

- Auto-grooming or auto-implementing the created stubs (existing `auto_groom` machinery already
  governs what happens next).
- The provenance field itself (#0090; this change consumes it via `depends_on: [90]`, kept a
  separate change — not merged).
- Deduplication beyond a cheap check against existing active titles/slugs.
- Making the per-invocation cap configurable (deferred follow-up).

## Open questions

Resolved at grooming (2026-07-17; rationale + rejected alternatives in the spec's `## Assumptions`):

- **Combine with #0090 or keep separate?** → Keep separate; #0091 consumes #0090's field via
  `depends_on: [90]`. (A cross-change *merge* was out of scope for this groom.)
- **Per-skill granularity or one switch?** → One global boolean `auto_capture`; granularity is a
  reversible follow-up if a need appears.
- **New board flag for auto-created stubs?** → No new board state; they surface as ordinary
  needs-brainstorm and provenance rendering is #0090's territory.

## Reconcile log

### 2026-07-18 — reconcile at claim (docket-implement-next)

Design **holds unchanged**; no scope drop, no fundamental invalidation. Verified against current
`origin/main` (`df4c6ec`), the archived #0090, and the ADR ledger.

**Dependency verified satisfied.** #0090 is `done` (PR #97, archived 2026-07-18) and landed all
three surfaces this change consumes: `discovered_from:` in the convention manifest block
(`skills/docket-convention/SKILL.md`), seeded empty in `skills/docket-new-change/change-template.md`,
and populated by `docket-new-change`'s step-3 scan. The mint contract can populate the field with no
folding-in of #0090 work.

**Design assumptions re-validated against current code.**

- The `auto_groom` resolution line in `scripts/docket-config.sh` is the exact four-layer shape
  `auto_capture` copies (repo-local > repo-committed > global > built-in `false`); the spec's stale
  `docket-config.sh:206` line-number citation was corrected to a symbolic reference.
- The ADR-0019 fence table in `scripts/docket-config.md` is still the authoritative per-key
  classification surface the spec's §8 requires the new row to land in.
- `docket-new-change`'s **step 1 (Allocate)** still carries the scan-max-id + CAS-on-rejection
  routine verbatim as the spec describes — the routine `mint-stub.sh` factors out is intact and
  unmoved.
- No `mint-stub.sh` (or equivalent) exists yet; `scripts/` has no auto-capture surface. Greenfield.

**Constraints folded in that post-date grooming (2026-07-17).** These are build-time obligations the
spec could not have known; none change the design, all shape the plan:

- **Skill size budgets** (`tests/test_skill_size_budgets.sh`, change 0085) now gate every
  `skills/**/*.md`. Current headroom for the files this change must edit: implement-next 127/140
  lines, 2641/2845 words; finalize-change 132/160, 2266/2699; status 107/118, 2175/2393; convention
  294/317, 4769/5104. The prose additions must fit, or **raise the budget row in the same diff** (the
  test explicitly sanctions that, and forbids silent regrowth).
- **Script-contract coverage** (`tests/test_script_contracts_coverage.sh`) requires a co-located
  `scripts/mint-stub.md` contract for any new `scripts/mint-stub.sh`.
- **Facade wiring** — a new op must be added to `WRAPPED_OPS` in `scripts/docket.sh` *and* its
  header op list (`tests/test_docket_facade.sh`, `tests/test_skill_facade_wiring.sh`).
- **Export-interface count** — adding `AUTO_CAPTURE` moves the emit interface from 23→24 lines
  (shell) / 24→25 (plain). Two assertions in `tests/test_docket_config.sh` (the direct-pipe count
  and the 0050 E' global-layer count) plus the counts in `scripts/docket-config.md` must move
  together.
- **Change 0089's `reclaim:` block** is the freshest end-to-end precedent for shipping a config knob
  (example-file prose, fence-table row, export, validation, tests) — `auto_capture` follows it, but
  as a **top-level boolean** like `auto_groom`, not a nested block.
- **Change 0088's terminal-disposition contract** is now where `docket-implement-next`'s "run
  report" lives. The spec's cap-overflow and dedup-skip reporting (§5/§6) should ride in that
  existing enumerated final report rather than invent a parallel reporting surface.

**Open questions:** none reopened. The three resolved at grooming remain resolved, and the
per-invocation cap stays a hardcoded constant per §6.
