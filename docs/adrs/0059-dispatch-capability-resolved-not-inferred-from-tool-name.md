---
id: 59
slug: dispatch-capability-resolved-not-inferred-from-tool-name
title: Dispatch capability is resolved, never inferred from a tool name; unavailability is tiered
status: Accepted
date: 2026-07-26
supersedes: []
reverses: []
relates_to: [8, 17, 24, 26]
change: 137
---

## Context

docket composes autonomous work out of subagent dispatches: `docket-status` and `docket-adr`
from the implementer, an adversarial critic from `docket-auto-groom`, and the `build`/`review`
role skills, which dispatch internally. Twice — changes #0127 (2026-07-22) and #0136
(2026-07-24) — a live run reported that its runtime had no subagent-dispatch tool, so the
`build`/`review` roles fell back to their inline `auto` paths. Both runs produced sound PRs and
disclosed the degradation honestly, but the isolation and independent review those roles exist
to provide did not happen, while every artifact still looked complete.

The premise was false. A grooming-time probe and a build-time spike (2026-07-25/26, Claude Code
`2.1.218`) found that a dispatch mechanism resolves on **both** invocation paths ADR-0026 names
first-class (skill-invoke and agent-dispatch), that a nested dispatch succeeds, and that the
`build`/`review` role skills are present in the tool list. The tool name docket's own prose had
been searching for simply was not the name the runtime used, and part of the tool surface is
deferred behind a search step — so absence was observable without anything having actually been
resolved. Docket's prose had primed an agent to look for a specific name; the Skill layer's
missing-skill rule then fired correctly on a false premise.

The build's own spike reproduced the failure mode live, independent of the historical incidents:
a probe agent reported a nested dispatch as failed while its own transcript showed the nested
child had returned the expected sentinel. It had backgrounded the child, yielded, and read its
own missing result as a missing capability — the convention's existing never-yield rule,
re-derived from a fresh angle.

## Decision

1. **Resolve, don't name-match.** A dispatch-dependent step may be declared unavailable only
   after attempting to resolve a dispatch mechanism — including searching deferred or
   lazily-loaded tool surfaces — and, if that is inconclusive, attempting one trivial dispatch.
   Only a failed attempt or an explicit policy denial establishes unavailability. The absence of
   a specifically-named tool never does.

2. **Stated by capability, not by tool name.** The rule docket depends on is a capability, never
   an interface. A tool name may appear in a failure diagnostic ("no dispatch mechanism
   resolved; searched `Agent`, `Task`") as an observed internal — it is never a decision input,
   and docket depends on it for nothing. This is what keeps the rule harness-neutral: it is
   stated the same way for Claude Code, Cursor, or any future harness, and a harness-specific
   tool name is a dated diagnostic observation, never part of the rule itself. Change #0135
   (Cursor) can cite this ADR without inheriting a Claude-Code-flavored assumption it would then
   have to fight — though #0135 must still repair its own wrapper-delivery problem before any
   convention-level rule reaches Cursor at all; that repair is #0135's, not this ADR's.

3. **Tiered unavailability posture**, because the dispatch kinds this decision governs are not
   equivalent:
   - **Deterministic composition** (`docket-status`, `docket-adr`) — runs **inline** as a
     first-class equivalent path when dispatch is genuinely unavailable. Its contract is git
     state, not an in-context return, so inline execution of the same deterministic orchestrator
     satisfies it fully; this is a reclassification, not a degradation.
   - **Adversarial gate** (`docket-auto-groom-critic`) — **abstains**. An author cannot be their
     own adversary, so genuine unavailability routes to `docket-auto-groom`'s existing abstain
     path (flip `auto_groomable: false`, append a dated `## Auto-groom blocked` section) rather
     than either running self-critique inline or halting.
   - **Discipline roles** (`skills.build`, `skills.review`) — **authorized-or-halt**. An
     explicitly configured `auto` is the human's authorization to run inline; any other
     configured value that cannot dispatch is abort-and-report.

4. **Deliberately rejected: a per-harness table of dispatch tool names.** It looks more
   deterministic and is worse:
   - A stale name produces a **silent** false negative — the check says "degrade to inline," the
     run completes, and the disclosure reads as boilerplate rather than a caught defect.
   - The name went stale between 2026-07-17 and 2026-07-22 with no signal at all; a committed
     name table makes a vendor internal load-bearing.
   - A name-presence check also misses the **reverse** errors it needs to catch: present but
     policy-denied, present but deferred-and-unsearched, present but capped by nesting depth. An
     agent type denied dispatch outright would read as capable under a name table and then fail.
   - It cannot be tested honestly: a sentinel asserting the prose names a given tool is
     specified-but-unreachable, and a fixture omitting the tool routes every test through the
     degrade path and still goes green.

## Consequences

- Capability-based detection costs at most one trivial dispatch on the rare inconclusive path,
  and buys a probe that fails only when dispatch is genuinely absent.
- Tier A reclassifies inline execution of deterministic composition as an equivalent path rather
  than a degradation, so those runs stop emitting misleading warnings.
- Tier C makes a lost discipline gate loud instead of silent: an unauthorized inline build now
  halts, a behavior change a repo opts out of by configuring `skills.build`/`skills.review` to
  `auto` deliberately. A Tier C halt uses only existing state — the change stays `in-progress`
  with `claimed_at` refreshed and a dated halt note — so the existing reclaim lease self-heals an
  abandoned claim. No new status, no new field.
- The rule is harness-neutral by construction, so #0135 (Cursor) can cite it directly.
- **This decision rests on a build-time spike, not a standing test.** docket's suite is
  hermetic bash and cannot dispatch a subagent, so there is no regression test behind the tiered
  posture. The evidence is scoped to Claude Code `2.1.218` (2026-07-25/26) and to the two
  invocation paths probed; it must be re-probed when the harness version moves, and is recorded
  verbatim (with version) in change #0137's results file rather than restated as fact here.
- **Known incomplete.** `docket-finalize-change`'s two in-context-gating dispatches
  (`docket-rebase-resolver`, `docket-integration-repair`) match no tier row above — their reports
  flow back to finalize in-context to gate a merge, unlike Tier A's git-state contract. Finalize's
  existing abort-and-report set already covers a resolution failure in both cases, so nothing is
  presently broken; extending this taxonomy to name them explicitly is tracked as a follow-up
  rather than resolved here.

This ADR is **parallel and additive** — it supersedes and reverses no ADR. It relates to
ADR-0008 (the agent layer whose dispatches this governs), ADR-0017 (the Cursor dispatch rule),
and ADR-0024 (whose fork-exclusion reasoning this ADR's evidence extends — see its dated
`## Update`).
