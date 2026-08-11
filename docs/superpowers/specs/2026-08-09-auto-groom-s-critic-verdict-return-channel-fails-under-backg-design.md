<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0281 — Auto-groom's critic verdict return channel fails under background dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0281-auto-groom-s-critic-verdict-return-channel-fails-under-backg.md)**
<!-- docket:backlink:end -->

# Auto-groom's critic verdict return channel — design

Change: 0281. Fixes the critic→dispatcher return-channel contract after the 2026-08-09 campaign's
7-for-7 message-back failures ("No agent named 'docket-auto-groom' is reachable"), stranded
verdicts, and grooms yielding indefinitely on "waiting for the critic's re-check."

## Problem

Three co-operating defects, all in prose contracts (no scripts):

1. **The critic names no delivery channel.** `agents/docket-auto-groom-critic.md` says only
   "Return exactly one verdict per the dispatching skill's protocol." A dispatched critic that
   cannot see the dispatching skill's body (it deliberately never loads it) invents a channel —
   in the observed campaign, name-addressed agent messaging to `docket-auto-groom`, which cannot
   resolve because a dispatched groom agent is not registered under its skill name. Per the
   learning `prohibition-needs-a-return-value`: the contract is incomplete until it names the
   channel the verdict travels on.
2. **The convention misclassifies the critic dispatch's contract.** The *Composition* paragraph
   lumps the critic with the `docket-status`/`docket-adr` dispatches whose "contract is git state
   … never an in-context return." The critic writes no git state — its verdict IS an in-context
   return, the same family as the rebase-resolver/integration-repair reports that "flow back …
   in-context." A groom reading the paragraph literally has no sanctioned way to receive a
   verdict at all.
3. **No groom-side posture when no verdict arrives.** A groom that backgrounds the critic (or
   whose harness dispatch surfaces the child's completion as a notification) and then waits for a
   message waits forever — no timeout, no fallback collect, no diagnostic. The never-yield rule
   already forbids the yield, but forbidding is not a recovery path.

## Decision — the settled contract

**Leg chosen: foreground-only.** The critic's verdict travels on exactly one channel: **the
critic's final report, read by the dispatcher as the dispatch's return value while it actively
blocks on the child.** Name-addressed message-back is banned as a verdict channel in both
directions. Both critic rounds (first pass and the bounded re-check) use the same channel.

Contract edits, by surface:

### 1. `agents/docket-auto-groom-critic.md` (critic agent source)

Add a delivery clause, binding at the point the critic finishes (per
`prohibition-needs-a-return-value`, the mapping lives in the clause, not a distant section):

- Your verdict is your **final report** — the text you end your run with. That return is the
  only channel; the dispatcher is blocking on it.
- Never attempt to message, address, or resolve your dispatcher by name or by any agent-listing
  surface: a dispatched groom is not registered under its skill name, so no such address
  resolves, and a verdict sent there is stranded. If you believe the return channel itself is
  unavailable, that belief changes nothing about what you do: write the verdict as your final
  report and stop.

### 2. `skills/docket-auto-groom/SKILL.md` (Step 3)

Step 3 already mandates foreground dispatch. Add the receiving half and the no-verdict posture:

- The verdict is read **from the critic's return** (its final report). The groom never waits for
  a message, notification, or any out-of-band delivery — there is nothing registered to deliver
  to (harness-neutral phrasing; no tool names in normative prose).
- **No-verdict posture (bounded, two steps, then out):** if the dispatch returns without a
  legible verdict — malformed return, pre-yield prose, or a backgrounded child's bare
  completion — the groom makes **one collect attempt** (read the child's completed final report
  if the harness surfaces it) and, failing that, **one fresh foreground re-dispatch** of the
  critic over the same draft. Still no verdict ⇒ treat as a failed dispatch attempt under the
  convention's *Dispatch-capability resolution* — **Tier B, abstain** for the stub, recording
  the return-channel diagnostic in the `## Auto-groom blocked` section. Never a third dispatch;
  never an indefinite wait. (Re-dispatching a critic is safe where re-dispatching a build worker
  is not — the critic is read-only over prose, holds no worktree, and writes no git state, so
  the closed-doors analysis in `yielded-worker-return-closes-every-door` does not bind here.)

### 3. `skills/docket-convention/SKILL.md` (*Composition* paragraph)

Move the critic dispatch out of the git-state-contract clause and into the in-context-return
family alongside `docket-rebase-resolver` / `docket-integration-repair`: foreground and
unconditional as before, but "its verdict flows back to the groom in-context as the dispatch's
return — never via git state and never via agent messaging." One-sentence surgical edit; the
never-yield rule and everything else in the paragraph stands.

### 4. Guard

One sentinel in the existing prose-guard style — **settled at reconcile (2026-08-11): the new file
`tests/test_critic_return_channel.sh`**, which obliges a `tests/runtime-budgets.tsv` row plus the
matching `EXPECTED_TOTAL` bump in `tests/test_runtime_budgets.sh`. It asserts (a) the critic
source binds the verdict to its final report and contains the never-address-your-dispatcher
clause, (b) Step 3 maps the no-verdict case to the abstain exit (bind phrase to claim with a
bounded gap, whitespace-collapsed match), (c) the convention no longer lists
`docket-auto-groom-critic` inside the git-state-contract clause. Mutation-tested per house rule.

## Assumptions

1. **Return-channel leg: foreground-only (chosen) vs resolvable return address vs
   collect-on-timeout as primary.** Chosen because it is the path already proven working in the
   same campaign, requires zero new machinery, and matches the convention's existing never-yield
   rule. *Rejected — resolvable return address:* would require the groom to mint and pass a
   harness-specific address; the observed failure is that no such address exists for a dispatched
   skill-agent, and any spelling of one is a tool/harness name in normative prose, which the
   convention bans as a decision input. *Rejected — collect-on-timeout as primary:* a subagent
   has no timer surface (ADR-0024's no-notification-channel finding), so "timeout" degenerates
   into polling prose that invites the exact yield being fixed. Collect survives only as the
   bounded fallback in the no-verdict posture.
2. **No-verdict posture: one collect + one fresh re-dispatch, then Tier B abstain.** Chosen over
   *abstain immediately* (would junk sound drafts on transient plumbing; verify-run's
   re-dispatch-once precedent shows one bounded retry is house style) and over *unbounded
   retry* (violates provable termination). The abstain mapping satisfies
   `prohibition-needs-a-return-value`: the closed exit vocabulary (spec/trivial/abstain) gains
   no new value.
3. **Critic re-dispatch is race-safe.** Asserted from the critic's contract (read-only, no
   worktree, no git writes), not from harness behavior. If a future critic gains write behavior
   this assumption must be revisited — noted here for the audit trail.
4. **Fix surfaces are prose + one test; no scripts.** Matches the stub's boundary. The guard is
   a test file, not a `scripts/` helper.
5. **Harness-neutral phrasing throughout.** Failure diagnostics MAY name what was attempted
   (e.g. the observed "No agent named…" string); normative clauses phrase by capability/shape
   only — per the convention's *Harness-native recovery* and *Dispatch-capability resolution*
   sections.
6. **Out of scope confirmed:** harness agent-naming internals; the shared-worktree contention
   family (0247 — the stub's `discovered_from: [247]`, plus `related: [247]` set at this spec
   exit as the forward coupling link). No dependency: 0281's prose edits do not touch files
   0247 plans to edit except `docket-convention/SKILL.md`, and there in a different paragraph —
   a rebase-level composition, not a readiness gate (`concurrent-edits-compose-at-rebase`).
