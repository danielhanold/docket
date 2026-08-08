---
id: 269
slug: decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child
title: Decouple the shim wrapper's own pin from the delegated child's
status: in-progress
priority: high
type: fix
created: 2026-08-08
updated: 2026-08-08
depends_on: []
related: [195, 256]
discovered_from: [258]
adrs: [15, 38, 67]
spec: docs/superpowers/specs/2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child-design.md
plan: docs/superpowers/plans/2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md
results:
trivial: false
auto_groomable:
branch: feat/decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child
claimed_at: 2026-08-08T19:36:17Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child-design.md) |
| Plan | [2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md](https://github.com/danielhanold/docket/blob/feat/decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child/docs/superpowers/plans/2026-08-08-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md), [ADR-0067](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0067-runner-bearing-agent-requires-a-user-configured-model.md) |
<!-- docket:artifacts:end -->

## Why

Runner delegation is broken end to end on the claude harness, and has been since it shipped.

`sync-agents.sh`'s `emit_shim` builds a delegation shim's frontmatter from the resolved **child**
model and effort. But the shim agent runs in the parent harness — Claude Code — and does exactly one
thing: a foreground `docket.sh runner-dispatch` call plus a stdout relay. Pinning it to the child's
model tells Claude Code to run that relay on a model it cannot resolve, so the run dies with a bare
harness error before `runner-dispatch.sh` is reached. The failure does not even name the runner,
which makes it read as an unrelated harness problem.

ADR-0067 makes this unavoidable rather than incidental: a runner-bearing agent **must** carry a
user-configured model, so the child's ID is always present to be copied into the wrong slot. No
configuration avoids it — every runner-delegated claude wrapper is born broken.

Observed 2026-08-08 during a `docket-implement-next 258` run, where every `docket-build-*` dispatch
failed with the wrappers pinned to `openrouter/deepseek/deepseek-v4-flash-0731`.

The behavior was decided deliberately. ADR-0038 keeps the frontmatter `model:` line "for bookkeeping
— the effective pin is the baked argument," and lists byte-stability with the native wrapper shape
among its accepted consequences. That premise is false: Claude Code reads the line as the live pin
for the shim agent. This change corrects the premise, not just the code.

## What changes

- Two new per-runner config keys, `runners.<name>.shim_model` (default `inherit`) and
  `runners.<name>.shim_effort` (default `low`), governing the shim's **own** frontmatter pin.
- `emit_shim`'s model/effort parameters are repurposed to carry those values instead of the child's;
  the child's continue to ride the baked `--model` / `--effort` arguments unchanged.
- The smallest reader in `sync-agents.sh` that resolves the two keys across the config layers.
- A new ADR superseding ADR-0038: a shim's frontmatter pin governs the parent-side agent and must be
  resolvable by the **parent** harness.
- Test coverage in `tests/test_sync_agents.sh`, including the regression assert that a shim's
  frontmatter model is never the value baked into `--model`.
- Documentation: the runner contracts, both config examples, the README, and the agent-layer
  reference.

Defaulting to `inherit` repairs every existing broken wrapper by regeneration alone, with no config
edit required. The knob is a cost optimization on top, not a prerequisite.

## Out of scope

- **The `runners.opencode.permissions` locality defect** — `.docket.local.yml` is gitignored, so a
  fresh feature worktree has no copy, and a build worker anchored there resolves a
  `permissions: auto-approve` grant back to the default `ask` and is refused. Real and separately
  reproducible; needs its own change.
- **Config-reader consolidation** (#0256). This change knowingly adds a second consumer of the
  `runners:` block rather than unifying the two parsers; 0256 should absorb both.
- ADR-0067's requirement that a runner-bearing agent carry a user-configured model.
- Codex and cursor wrapper generation — `emit_shim` is reachable only for `harness = claude`.

## Open questions

None. The 0256 boundary was considered and deliberately deferred; see the spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-08 — reconciled at claim

Design confirmed against current `origin/main`. The defect is live and unchanged: `emit_shim`
(`sync-agents.sh`, repo ROOT — not `scripts/`) still does `emit "$1" "$2" "$3"` with the resolved
child model, `REGISTERED_RUNNERS` is still the static `"codex cursor opencode"` with no `runners:`
reader, and both cited ADRs (0038, 0067) are still `Accepted`. Scope and the two deferral boundaries
(0256 config-reader consolidation; the opencode permissions locality defect) hold as written.

Four corrections folded in, none scope-changing:

- **Test home.** The spec's *Testing* section names `tests/test_sync_agents.sh`. The runner-shim
  tests actually live in `tests/test_sync_agents_runners.sh`, and that file already carries the
  assert that encodes the false premise: `0079: shim keeps frontmatter model (bookkeeping)`, which
  asserts the shim's frontmatter model equals the configured child model. That assert must be
  inverted, not merely joined by a new one — the regression assert and the existing assert are the
  same claim with opposite signs, so leaving both would make the suite unsatisfiable.
- **`emit_wrapper`'s calling contract (change 0220)** is a second documented statement of the false
  premise: its header says `$2` "is used TWICE and for two different things — as emit_shim's
  frontmatter pin, and … the baked --model flag." After this change `$2` reaches only the baked
  flag. The header and the reasoning it carries (why the provenance filter is a second spelling
  rather than a call) must be restated, and the `$2 != RES_MODEL` assertion itself stays — it
  constrains `emit_wrapper`'s own input, not what it forwards to `emit_shim`.
- **Reader placement.** `section_body` in `sync-agents.sh` is the existing dedenting awk twin of
  `runner-dispatch.sh`'s `yaml_section`; the new reader composes it (`section_body runners |
  section_body <name>`) rather than adding a third parse shape. The 0256 deferral is unchanged, but
  the duplication it will absorb should be the twin pattern, not a novel one.
- **ADR number.** The highest ADR on `docket` is 0077, so the superseding ADR lands at 0078.

Auto-capture: the out-of-scope opencode `permissions` locality defect was unminted and clears all
six admission gates — minted as stub **0270** (`type: fix`, `discovered_from: [269]`).
