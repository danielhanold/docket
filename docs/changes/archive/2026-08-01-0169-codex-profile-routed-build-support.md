---
id: 169
slug: codex-profile-routed-build-support
title: Codex support for profile-routed Docket builds
status: done
priority: medium
type: feat
created: 2026-07-30
updated: 2026-08-01
depends_on: [167, 168]
related: [77, 78, 79]
discovered_from: [167]
adrs: [36, 37, 38, 63, 64]
spec: docs/superpowers/specs/2026-07-31-codex-profile-routed-build-support-design.md
plan: docs/superpowers/plans/2026-07-31-codex-profile-routed-build-support.md
results: docs/results/2026-07-31-codex-profile-routed-build-support-results.md
trivial: false
auto_groomable:
branch: feat/codex-profile-routed-build-support
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/143
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-31-codex-profile-routed-build-support-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-31-codex-profile-routed-build-support-design.md) |
| Plan | [2026-07-31-codex-profile-routed-build-support.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-31-codex-profile-routed-build-support.md) |
| Results | [2026-07-31-codex-profile-routed-build-support-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-31-codex-profile-routed-build-support-results.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0037](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0037-runner-delegation-explicit-runner-field.md), [ADR-0038](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 shipped Docket's profile-routed build under Claude first. Change 0168 then moved all
shipped agent defaults into the harness-indexed `agents/harness-defaults.yml` sidecar and made each
shipped harness complete across all twelve wrappers. Codex already has native TOML generation and
dispatch, but it is the remaining known harness with no shipped block, so every Codex wrapper is
honestly unpinned today.

The missing work is now narrow: supply one complete, validated Codex mapping, certify the three
build profiles under the native harness, and flip the guards and documentation that deliberately
encode the current unpinned state.

## What changes

- Add a complete twelve-agent `codex:` sidecar block. Promote the existing nine illustrative
  mappings unchanged and ship the selected build profiles: Luna/xhigh for economy, Terra/high for
  standard, and Sol/medium for premium.
- Add Codex to the shipped-harness completeness gate and promote `.docket.example.yml`'s Codex
  block from a doubly commented illustration to a singly commented exact mirror.
- Reuse the existing Codex TOML emitter and native named-agent dispatch. Keep whole-run runner
  delegation separate and prove shipped native defaults never leak into runner flags.
- Replace the pre-0169 TOML absence assertions with shipped-value assertions, extend the derived
  mirror/round-trip guards, and update maintained Codex and build-profile documentation.
- Certify all three named profile dispatches in a real Codex session. Automatic classification and
  single escalation remain hermetically covered, with the live-observation waiver recorded in the
  results artifact.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Adding a Codex-specific controller branch or invoking `codex exec` once per task.
- Cursor support (change 0168) or replacement of the whole-branch review skill.
- Revisiting ADR-0064's sidecar design. This change is a consumer of it: it adds a harness block
  and satisfies the existing completeness rule. If Codex needs a shape the sidecar cannot express,
  that is a new ADR, not an edit to that one.

## Reconcile log

### 2026-07-31 — build-time reconcile

Re-read the change and its spec against `origin/main` at `9d41fa6b`, the cited ADRs
(0036/0037/0038/0063/0064), the just-archived dependencies 0167 and 0168, and the current code.
**The design holds unchanged; no scope adjustment was needed.** Every premise the spec rests on was
re-verified rather than assumed:

- **Both dependencies are `done`.** 0167 shipped the profile-routed controller; 0168 shipped the
  harness-indexed sidecar and a *complete* twelve-agent `cursor:` block. The sidecar's
  completeness-by-shipped-harness rule is therefore already live, which is exactly the shape this
  change plugs into.
- **The pre-0169 state is still literally the pre-0169 state.** `agents/harness-defaults.yml` is 55
  lines with `claude:` and `cursor:` blocks and **no `codex:` block**;
  `scripts/lib/harness-defaults.sh:26` still reads `HD_SHIPPED_HARNESSES="claude cursor"`. The
  deliberate absence markers the spec promises to flip are all present and enumerable —
  `tests/test_harness_defaults.sh:56`, `tests/test_sync_agents_codex.sh:42,135`,
  `tests/test_sync_agents.sh:1482`, `tests/test_docket_example_yml.sh:842`, `sync-agents.sh:622`,
  `README.md:723`, `docs/codex/setup.md:7,75`, and `.docket.example.yml:304,335`. None of this work
  was done elsewhere in the interim.
- **The `.docket.example.yml` Codex block is exactly as the spec describes it** — doubly commented,
  labelled unvalidated, with nine non-build rows that already match the spec's shipped table and
  three illustrative build rows (all Sol at low/medium/high) that this change replaces with the
  settled Luna/Terra/Sol pairs.

**Catalog re-probe (the spec's build-time failure-behavior gate).** Design-time evidence was checked
during grooming against Codex CLI 0.146.0. Re-probed the *installed* toolchain at reconcile:
`codex-cli 0.146.0`, and `codex debug models` reports all three slugs with every selected effort
token available —

| Model | Efforts reported | Selected pairs used by the sidecar |
|---|---|---|
| `gpt-5.6-luna` | low, medium, high, xhigh, max | `xhigh` |
| `gpt-5.6-terra` | low, medium, high, xhigh, max, ultra | `high`, `xhigh` |
| `gpt-5.6-sol` | low, medium, high, xhigh, max, ultra | `low`, `medium`, `high` |

All twelve model/effort pairs in the spec's table resolve against the live catalog. **No design
drift; the change proceeds without a human escalation.** The catalog must be re-probed once more
immediately before live certification, per the spec.

**Carried forward as a build-time decision, not a scope change.** The spec's Tier 2 requires the
three named profile dispatches observed in a real Codex session. The 0168 precedent shipped Tier 1
autonomously and left Tier 2 as a recorded checklist in the results artifact for the maintainer,
explicitly refusing a sibling CLI as an IDE substitute. Codex differs in a way that matters here:
`codex exec` is the same Codex CLI and the same native agent registry, not a lagging sibling, so
certification may be genuinely executable inside this build. The plan decides; either way the
outcome and any waiver are recorded explicitly in the results artifact, and an unobserved dispatch
is reported as uncertified rather than assumed.

No adjacent follow-up work met the auto-capture materiality bar during this pass.
