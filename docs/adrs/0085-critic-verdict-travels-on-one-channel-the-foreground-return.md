---
id: 85
slug: critic-verdict-travels-on-one-channel-the-foreground-return
title: "Critic verdict travels on exactly one channel: the foreground dispatch return"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [9, 24, 59, 84]
change: 281
---

## Context

During the 2026-08-09 groom campaign, every `docket-auto-groom-critic` dispatch — 7 for 7 — failed to
deliver its verdict back to its dispatcher by name-addressed agent messaging, each time with
`No agent named 'docket-auto-groom' is reachable`. A dispatched groom agent is **not registered under
its skill name**, so no such address resolves. The verdicts survived only as the critic's transcript
output, and the campaign completed only because a coordinating human session read each one and
relayed it by hand — an unmodeled coordinator dependency sitting inside a flow that is autonomous by
contract.

Three co-operating prose defects, no scripts:

1. `agents/docket-auto-groom-critic.md` named **no** delivery channel at all, so a critic that cannot
   see the dispatching skill's body invented one.
2. The convention's *Composition* paragraph misclassified the critic dispatch into the git-state-contract
   family ("their contract is **git state** … never an in-context return"), leaving a groom that read it
   literally with no sanctioned way to receive a verdict.
3. No groom-side posture existed for "no verdict arrived" — no timeout, no fallback collect, no
   diagnostic — so the run wedged silently.

## Decision

The adversarial critic's verdict travels on **exactly one channel: its final report, read as the
dispatch's return value while the dispatcher actively blocks.**

1. **Foreground-only.** The verdict is the critic's final report. Name-addressed message-back is banned
   as a verdict channel **in both directions**. Both critic rounds — the first pass and the bounded
   re-check — use the same channel.
2. **The critic's own contract carries the delivery clause** at the point it stops, and states the
   reason (a dispatched groom is not registered under its skill name), so a critic cannot reason its
   way into an invented channel. A believed-unavailable return channel changes nothing about what it
   does: write the verdict as the final report and stop.
3. **The convention reclassifies the dispatch.** The *Composition* paragraph moves the critic dispatch
   out of the git-state clause into the **in-context-return** family alongside `docket-rebase-resolver`
   and `docket-integration-repair`. Foreground, unconditional, and never-yield all stand unchanged.
4. **Bounded no-verdict posture:** one collect attempt from the child's completed report, then one fresh
   foreground re-dispatch — issued through whatever mechanism makes the parent block on the return; if
   no such mechanism exists, that leg would only repeat the first and is skipped — then **Tier B
   abstain**. Never a third dispatch; never an indefinite wait.

### Rejected alternatives

- **Mint a resolvable return address.** Would require creating and passing a harness-specific address.
  The observed failure is precisely that no such address exists for a dispatched skill-agent, and any
  spelling of one puts a tool/harness name into normative prose — which the convention bans as a
  decision input (ADR-0059).
- **Collect-on-timeout as the primary channel.** A subagent has no timer surface (ADR-0024's
  no-notification-channel finding), so "timeout" degenerates into polling prose that invites the exact
  yield being fixed. Collect survives only as the bounded fallback in step 4.
- **Unbounded retry.** Violates provable termination.

### The no-verdict route takes the full Abstain exit

Settled at review: the no-verdict route takes the **full** Abstain exit, `auto_groomable: false` flip
included, even though the trigger is a per-run transient rather than a design problem — so a healthy
draft can be disarmed by a plumbing fault.

It was chosen anyway because the alternative breaks provable termination. `docket-auto-groom`'s drain
re-ranks every autonomous-eligible stub each iteration, so a stub left armed is re-selected **at
unchanged rank**, hits the same broken channel, and spins — on an unattended run with nobody to stop
it. Rescuing the diagnostic-only variant would require treating `## Auto-groom blocked` presence as an
eligibility exclusion, which is a change to the convention's shared eligibility definition, contradicts
its "the abstain is the single agent write" framing, and breaks its re-arm protocol (which tells a human
to flip a flag back to `true` that was never `false`).

## Consequences

- The verdict channel is now stated at the point the critic stops, so the failure mode that produced
  7-for-7 breakage is closed at its source rather than only in the dispatcher's body.
- The autonomous flow no longer depends on a human relay. The unmodeled coordinator dependency is gone.
- The convention's dispatch families now match reality: the critic sits with the in-context-return
  dispatches, not the git-state ones.
- **The recorded cost:** a broken return channel is rarely stub-specific, so a drain that hits one will
  disarm each stub it reaches rather than parking a single one — burning the whole drain instead of one
  entry. That is accepted as the price of provable termination; a human re-arms per the standard re-arm
  protocol.
- Guarded by the mutation-tested `tests/test_critic_return_channel.sh`.

Surfaces changed by change 0281: `agents/docket-auto-groom-critic.md`, `skills/docket-auto-groom/SKILL.md`
Step 3, the `**Composition (change 0017).**` paragraph of `skills/docket-convention/SKILL.md`, and the new
guard `tests/test_critic_return_channel.sh`.
