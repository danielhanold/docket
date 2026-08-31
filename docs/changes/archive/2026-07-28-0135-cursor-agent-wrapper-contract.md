---
id: 135
slug: cursor-agent-wrapper-contract
title: "Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort"
status: done
priority: high
type: fix
created: 2026-07-23
updated: 2026-07-28
depends_on: []
related: [16, 44, 45, 46, 48, 49, 66, 113, 137]
discovered_from: []
adrs: [8, 15, 24, 59, 60]
spec: docs/superpowers/specs/2026-07-26-cursor-agent-wrapper-contract-design.md
plan: docs/superpowers/plans/2026-07-27-cursor-agent-wrapper-contract.md
results: docs/results/2026-07-27-cursor-agent-wrapper-contract-results.md
trivial: false
auto_groomable:
branch: feat/cursor-agent-wrapper-contract
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/127
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-26-cursor-agent-wrapper-contract-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-26-cursor-agent-wrapper-contract-design.md) |
| Plan | [2026-07-27-cursor-agent-wrapper-contract.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-27-cursor-agent-wrapper-contract.md) |
| Results | [2026-07-27-cursor-agent-wrapper-contract-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-27-cursor-agent-wrapper-contract-results.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md) |
<!-- docket:artifacts:end -->

## Why

A live `docket-implement-next` run under Cursor exposed that docket's generated Cursor wrappers do
not match Cursor's documented subagent contract. The wrapper advertised
`superpowers:subagent-driven-development` as the resolved build skill, but the child had no
documented Skill tool with which to invoke it. Plan, build, review, and finish consequently degraded
to their inline `auto` fallbacks. The run still produced a plausible PR, but it did not execute the
SDD workflow's fresh per-task implementers, TDD discipline, or per-task review gates.

This is the Cursor-specific instance of the defect already recorded by the
`skill-fallback-degrades-discipline` learning from change 0066: successful artifacts can conceal
that the configured workflow discipline was unreachable in the harness that actually ran the
build.

The same investigation found a second contract mismatch in model configuration. Cursor's current
[Subagents documentation](https://cursor.com/docs/subagents) documents these custom-agent
frontmatter fields: `name`, `description`, `model`, `readonly`, and `is_background`. Reasoning
effort is a model parameter encoded in the model value, for example
`model: claude-opus-4-8[effort=high]`. Docket instead generates a Claude-shaped wrapper for Cursor:

```yaml
model: claude-opus-4-8
effort: xhigh
skills: [docket-implement-next, docket-convention]
```

Neither the standalone `effort:` field nor `skills:` preload is part of Cursor's documented
frontmatter. In `sync-agents.sh`, `emit_for_harness()` routes every non-Codex harness through the
same generic Markdown emitter, so Cursor inherits Claude Code semantics. Existing Cursor generation
tests preserve that shape rather than checking Cursor's actual contract. Model pinning, reasoning
effort, and skill availability can therefore all differ from what docket reports.

**Grooming (2026-07-26) resolved the nesting question, and the stub's premise was wrong.** Cursor
documents a nesting limit of **two** ("the main agent and its direct subagents can launch subagents,
but a subagent launched by another subagent can't launch further ones"), but docket's tree does not
run three deep. `superpowers:subagent-driven-development`'s topology is flat: the orchestrator
dispatches implementers, task reviewers, fix subagents, and the final reviewer as siblings — the
implementer never dispatches. Docket needs exactly depth 2, which Cursor permits, so **SDD is
genuinely reachable on Cursor** and no halt posture is needed for nesting. Grooming also established
that docket's skills are already discoverable in Cursor (`link-skills.sh` symlinks them into
`~/.cursor/skills/`, one of Cursor's documented discovery directories) — `skills:` fails because it
is an ignored field with no body instruction behind it, not because the skills are missing. The fix
is an instruction, not an installation.

**Note added 2026-07-25 (from change [[137]]'s grooming).** These two changes look like one defect
but are failures at different layers, and neither fix reaches the other harness. **This change's
failure is skill-delivery, not dispatch**: Cursor's Task tool exists and is documented; what the
Cursor child lacked was a Skill tool and the docket instructions, because `skills:` preload and
standalone `effort:` are not Cursor frontmatter fields. Change 137's failure is the reverse — its
dispatch tool and skills both work, and it mis-detected them (Claude Code's dispatch tool is named
`Agent`, not `Task`, and subagent tool sets are partially deferred). Consequently **change 137 does
not fix this change**: it delivers its fix as docket-convention prose, through the very channel this
change is broken on. Repairing wrapper delivery here is the prerequisite for any convention-level
rule reaching a Cursor child. The two stay `related:`, never `depends_on:` — neither gates the other.

Change 137 records the shared cross-harness decision (capability-based dispatch detection rather
than tool-name matching, plus a tiered posture for genuinely-unavailable dispatch) in a new
**harness-neutral** ADR that this change should **cite and implement for Cursor** rather than
re-decide.

## What changes

Make generated Cursor agents conform to Cursor's documented subagent contract, so the workflow
discipline docket advertises is genuinely reachable — and so ADR-0059's rules can reach a Cursor
child at all. Design in the spec; at scope altitude:

- Add **`emit_cursor_md()`**, a named Cursor emitter, and give `emit_for_harness()` explicit
  `cursor)`/`claude)` branches — documenting the `*)` catch-all as an unverified gap rather than a
  supported mapping.
- Emit only Cursor's documented frontmatter fields, with model and effort encoded verbatim as
  `<model>[effort=<e>]` — **no allowlist of Cursor model IDs or effort tokens** (ADR-0015 passthrough;
  ADR-0059's rejection of vendor-internal tables). The one edge case, `inherit` + effort, drops the
  pin with a loud generation-time WARN.
- Replace the inert `skills:` preload with a **body preamble** naming the skills to load, mirroring
  the Codex emitter's `developer_instructions` preamble.
- Reword the Cursor dispatch rule (head + nine fragments) from `Task`-named instructions to
  capability language, keeping the concrete call as a labelled illustration.
- Add a **`cursor` runner adapter** (`scripts/runners/cursor.sh`, mirroring `codex.sh`) whose failure
  posture is loud abort-and-report — never a silent inline fall-back.
- Split the per-harness wrapper shapes in `agent-layer.md` and correct any prose implying one uniform
  shape.
- Record a **new ADR**: a generated wrapper conforms to its target harness's own documented contract;
  the generic emitter is Claude's shape, not a default other harnesses may inherit. Refines ADR-0008
  and ADR-0015 without superseding either; cites ADR-0059.
- Verify in **three tiers**: hermetic contract tests replacing today's byte-identical-to-Claude
  assertions (gating); a `cursor-agent -p` spike that is explicitly **best-effort and non-gating**,
  under a stated evidence rule that a negative CLI result is never evidence the contract is wrong;
  and a **human-executed Cursor IDE checklist** (the #0078 house pattern) that is the certifying
  tier. Because that tier runs after the PR opens, the **PR body must state that IDE validation is
  pending** so the merge gate is not cleared on a green suite alone.

## Out of scope

- Changing the Superpowers SDD or TDD skills themselves.
- Retrofitting or reopening the consuming `cet-devops` change 6 implementation or its PR.
- Completing change 0044's broader configurable per-role build-model design, except for keeping its
  eventual configuration compatible with the corrected Cursor emitter.
- General redesign of Claude Code or Codex wrapper generation beyond changes needed to preserve
  their existing behavior while splitting out Cursor semantics.
- Re-deciding ADR-0059's capability-resolution rule or its tiered unavailability posture. This change
  **implements and delivers** that decision for Cursor; it does not reopen it. The stub's "define an
  honest failure posture" item is therefore already owned: `skills.build`/`skills.review` are
  authorized-or-halt under Tier C, and what was missing on Cursor was delivery, not a decision.
- Other unvalidated harness tokens (`kiro`, `windsurf`), which remain on the documented `*)` gap.

## Open questions

None — all seven were resolved at grooming on 2026-07-26 and the answers are recorded in the spec.
Two are worth flagging as premise corrections rather than mere answers: Cursor's nesting limit is
**two** and docket needs exactly two (the stub's three-deep tree was wrong, so SDD is reachable), and
the honest-failure-posture question was already decided by ADR-0059 rather than open.

The remaining uncertainty is empirical, not architectural, and is discharged by the spec's Tier 3
Cursor IDE checklist rather than by design: whether the Cursor IDE honors the corrected wrappers as
documented. `cursor-agent` is known to be unreliable and to lag the IDE, so the CLI spike is
best-effort and cannot settle it.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-27 — reconcile at claim

Scope, spec, and design all hold as written; the spec was groomed 2026-07-26 (one day old) and every
premise it rests on was re-verified against `origin/main` at `0da1c0aa`:

- **ADR-0059 has landed.** `docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md`
  is present and `Accepted`, and change 0137 is merged and published. The spec's "cite, do not
  re-decide" posture is therefore live rather than anticipatory. `adrs:` already carries `59`, so the
  spec's note about #0137 not having folded it in is now discharged — no edit needed.
- **The defect is still present, unchanged.** `sync-agents.sh`'s `emit_for_harness()` (~line 361)
  still routes `codex)` to `emit_codex_toml` and everything else — Cursor included — through the
  generic Claude-shaped `emit()`. `harness_ext()` still maps every non-Codex token to `md`. Nothing
  in changes 0130–0137 touched Cursor emission.
- **`scripts/runners/` still contains only `codex.{sh,md}`**, so the runner adapter is net-new as the
  spec assumes, and remains the pre-agreed carve-out point.
- **The dispatch rule surface is as described**: `cursor-rules/dispatch.head.md` plus exactly nine
  `cursor-rules/dispatch/docket-*.md` fragments.
- **The defect-encoding tests exist as the spec claims.** `tests/test_sync_agents.sh` asserts a Cursor
  wrapper's standalone `effort:` field is inherited from `default` and that a default-only config
  makes the Claude and Cursor files **byte-identical**. Both assertions encode the defect and are
  rewritten by Tier 1 rather than kept.

No scope was dropped, added, or resized. `reconciled: true`.
