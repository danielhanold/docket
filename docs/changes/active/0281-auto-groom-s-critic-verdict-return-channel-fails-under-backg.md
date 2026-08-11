---
id: 281
slug: auto-groom-s-critic-verdict-return-channel-fails-under-backg
title: 'Auto-groom''s critic verdict return channel fails under background dispatch'
status: in-progress
priority: medium
type: fix
created: 2026-08-09
updated: 2026-08-11
depends_on: []
related: [247]
discovered_from: [247]
adrs: [85]
spec: docs/superpowers/specs/2026-08-09-auto-groom-s-critic-verdict-return-channel-fails-under-backg-design.md
plan: docs/superpowers/plans/2026-08-11-auto-groom-s-critic-verdict-return-channel.md
results:
trivial: false
auto_groomable: true
branch: feat/auto-groom-s-critic-verdict-return-channel-fails-under-backg
claimed_at: 2026-08-11T07:28:56Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-auto-groom-s-critic-verdict-return-channel-fails-under-backg-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-auto-groom-s-critic-verdict-return-channel-fails-under-backg-design.md) |
| Plan | [2026-08-11-auto-groom-s-critic-verdict-return-channel.md](https://github.com/danielhanold/docket/blob/feat/auto-groom-s-critic-verdict-return-channel-fails-under-backg/docs/superpowers/plans/2026-08-11-auto-groom-s-critic-verdict-return-channel.md) |
| ADRs | [ADR-0085](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0085-critic-verdict-travels-on-one-channel-the-foreground-return.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — observed 7-for-7 during the 2026-08-09 groom campaign (first on change 0247's
critic round, then on 0275, 0261, 0263, 0266, 0277, 0279, 0272, 0265, and 0195): every
`docket-auto-groom-critic` that tried to deliver its verdict back to its dispatcher by
name-addressed agent messaging failed — "No agent named 'docket-auto-groom' is reachable" —
and several critics additionally reported having no agent-listing surface with which to resolve
a ref. Each verdict survived only as the critic's final transcript output, and the campaign
completed only because the coordinating session manually relayed every verdict to the paused
groom — an unmodeled coordinator dependency inside a flow that is autonomous by contract.

**Opportunity** — the critic gate already has one working return path: a groom that dispatches
its critic foreground reads the child's return value directly (several rounds in the same
campaign used it successfully). The failing path is the message-back protocol under background
dispatch: a dispatched groom agent is not registered under its skill name, so name-addressed
delivery cannot resolve, and a groom that yields "waiting for the critic's re-check" waits on a
channel that never delivers. Without a relay the run wedges silently — there is no timeout, no
fallback collect, and no diagnostic.

**Independent value** — unattended throughput and correctness of the auto-groom drain: with the
return channel severed, every backgrounded groom stalls at its first critic round regardless of
how sound its draft is. The defect is in docket's own skill/agent contracts, so it stands
independent of any single harness session.

**Boundary** — settle the critic→dispatcher return-channel contract for both dispatch shapes:
either mandate foreground critic dispatch and ban the message-back protocol, or specify a
return address/instruction that resolves from a background child, and in either case define the
groom-side posture when no verdict arrives (bounded wait, then collect the verdict from the
critic's transcript/report output — never an indefinite yield). The fix lands in the
`docket-auto-groom` skill body, the `docket-auto-groom-critic` agent source, and their
dispatch-contract prose; no scripts are expected to change. Out of scope: the hosting harness's
agent-naming implementation, and the shared-worktree contention family (change 0247).

## What changes

Design settled (2026-08-09 auto-groom, critic-gated 8/8 sound; full decision trail in the linked
spec's `## Assumptions`). **Leg chosen: foreground-only.** The critic's verdict travels on exactly
one channel — its final report, read as the dispatch's return value while the groom actively
blocks; name-addressed message-back is banned as a verdict channel in both directions. Edits, all
prose plus one guard:

- **Critic agent source** (`agents/docket-auto-groom-critic.md`): a delivery clause binding at the
  point the critic finishes — the verdict IS the final report; never message, address, or resolve
  the dispatcher by name or agent-listing surface (no such address resolves for a dispatched
  skill-agent).
- **Groom skill Step 3** (`skills/docket-auto-groom/SKILL.md`): the receiving half (read the
  verdict from the critic's return; never await out-of-band delivery) plus a bounded no-verdict
  posture — one collect attempt from the child's completed report, one fresh foreground
  re-dispatch, then Tier B **abstain** with the return-channel diagnostic. Never a third dispatch,
  never an indefinite wait.
- **Convention *Composition* paragraph**: reclassify the critic dispatch out of the
  git-state-contract clause into the in-context-return family (alongside rebase-resolver /
  integration-repair); foreground, unconditional, and never-yield all stand.
- **Guard**: a mutation-tested prose sentinel binding the critic's final-report clause, Step 3's
  no-verdict→abstain mapping, and the convention reclassification.

## Out of scope

The hosting harness's agent-naming implementation, and the shared-worktree contention family
(change 0247; overlap confined to a different paragraph of `docket-convention/SKILL.md` —
composes at rebase, no dependency).

Also out of scope, and confirmed so at reconcile: the four *other* populations of critic-dispatch
prose (`cursor-rules/dispatch/docket-auto-groom-critic.md`, `AGENTS.md`'s generated
`docket:dispatch` block, `agents/harness-defaults.yml`, `README.md` / `docs/codex/`). They address
the **parent**, and each already states foreground dispatch; the delivery clause this change adds
binds the **critic**, which loads only its own agent source plus `docket-convention`.

## Reconcile log

### 2026-08-11 — reconciled against origin/main (five changes merged since the 2026-08-09 spec)

**Verdict: the spec holds unchanged.** All three defects it names are present verbatim on
`origin/main` today — the critic source still says only "Return exactly one verdict per the
dispatching skill's protocol" and names no channel; the *Composition* paragraph still lumps the
critic into the "contract is **git state** … never an in-context return" clause; Step 3 still has
no no-verdict posture. No scope adjustment needed.

**Population survey (whole-repo grep, not a hand-list — per the 0208 rule).** Five populations
restate the critic dispatch contract, not one: skill bodies, `agents/`, `agents/harness-defaults.yml`,
`cursor-rules/dispatch/`, and `AGENTS.md`'s generated block (plus `README.md` and
`docs/codex/validation-runbook.md`). Only the first two are edited here, for the reason recorded
under *Out of scope*. Recording the survey so the exclusion reads as a decision, not an oversight.

**Bearing of the five post-spec merges:**

- **#0286** (`gate-run --observe` caller loops) — **does not apply.** Its capture-then-match poll
  loop is the correct shape for observing a backgrounded child; the leg chosen here bans the
  backgrounded critic outright, so there is no child to poll and importing a poll loop would
  reintroduce exactly the yield being fixed. Noted so the absence reads as deliberate.
- **#0277** (`--brief-file`, ADR-0082) — **does not apply.** `skills/docket-auto-groom/SKILL.md`
  contains no runner delegation at all; the critic dispatch is a harness-native in-session
  subagent dispatch, so the brief-file argument surface is never crossed.
- **#0275** (ADR-0084) — **reinforces the design.** Its rule (re-dispatch permission is gated on
  mechanical attribution capability, never on launch shape or the child's own report) is the same
  principle one level down: foreground dispatch makes the return itself the attribution. Its
  closing observation — "an agent left with no procedure improvises" — is precisely defect 3, and
  the bounded no-verdict posture is the procedure that removes the improvisation.
- **#0208** (ADR-0083, `worktree-scope:`) — the critic source already carries
  `worktree-scope: metadata`; untouched by this change.
- **#0270** (runner-config locality) — no bearing.

**Build-time choice settled:** the guard lands as the new file `tests/test_critic_return_channel.sh`
(the spec permitted either that or a fold-in), which obliges a `tests/runtime-budgets.tsv` row and
a matching `EXPECTED_TOTAL` bump in `tests/test_runtime_budgets.sh`.

**Coordination:** change 0260 also edits `skills/docket-convention/SKILL.md`. This diff touches only
the *Composition* paragraph, so the two compose at rebase (`concurrent-edits-compose-at-rebase`).

**Auto-capture:** enabled; nothing surfaced at this pass clears the six admission gates — every
finding above is in-scope drift and routes to this log. Minted 0.
