# docket-auto-groom — autonomous grooming drain (design)

**Change:** 0014 · **Date:** 2026-06-12 · **Status:** approved by Daniel (brainstorm 2026-06-12)

## Problem

docket's build half is already autonomous: `docket-implement-next` takes any build-ready
change to an open PR with no human. The grooming half is not: every needs-brainstorm stub
waits for an interactive session (`docket-groom-next`, change 0012), even when the human
would only click through the agent's recommended defaults. For repos where Daniel wants
the agent to "just build," grooming is the human bottleneck.

Change 0012 scoped this out explicitly ("Autonomous (no-human) spec writing — the
brainstorm stays interactive"). This change is that deliberately-excluded sibling.

**Key insight from the brainstorm:** no auto-*build* flag is needed. Build-readiness
already is build permission — `docket-implement-next` builds whatever is build-ready and
stops at the human merge gate. The only missing capability is autonomous grooming.
End-to-end chaining (groom → build in one run) is a separate future change, alongside
0008 (parallel backlog drain).

## Trust model — who may be auto-groomed

- **`.docket.yml` knob:** `auto_groom: false` — the repo-wide default. Off unless the
  repo opts in.
- **Change frontmatter field:** `auto_groomable:` — tri-state. Unset ⇒ inherit the repo
  knob; explicit `true`/`false` ⇒ wins. **Effective auto-groomable** = the override if
  set, else the repo default. Tri-state (not materialized at create time) so flipping the
  repo knob applies retroactively to changes that never overrode.
- The field is human input with one exception: the abstain flip (below) is the single
  agent write.
- `docket-new-change` may set `auto_groomable: true` at create time when the human
  provides rich initial context and says so. Scan-mode stubs leave it unset (inherit).

## The skill — `docket-auto-groom`, drain semantics

A seventh operating skill. Fully autonomous; writes markdown only (never branches,
worktrees, or code) — all writes on the metadata branch via the metadata working tree,
pushed immediately.

It is a **drain**, not a "next": nobody is waiting between stubs, so it loops until the
eligible queue is empty. This is deliberately a different control shape from
`docket-groom-next` (one stub per invocation, because human attention is the scarce
resource) — which is why it is a separate skill, not a mode.

Loop, per iteration:

1. Sync the metadata tree. Select the next **eligible** stub: needs-brainstorm
   (`proposed`, no `spec:`, not `trivial: true`) AND effective-auto-groomable — in the
   shared deterministic order (`priority` → `created` → lowest `id`). Unsatisfied
   `depends_on` does NOT exclude a stub — same design-ahead rule as `docket-groom-next`;
   the designer pass factors dependency state into its assumptions, and the implementer's
   build-time reconcile re-validates the spec anyway.
2. Groom it via the designer + critic passes (below); exit spec / trivial / abstain.
3. Commit and push that stub's outcome individually, CAS-style: a rejected push ⇒
   re-pull, re-validate the stub is still eligible, re-apply or skip. Board pass rides
   each outcome commit.
4. Repeat. When no eligible stub remains, emit a final report: groomed N (specs),
   trivial M, abstained K with one-line reasons.

**Termination is guaranteed:** every exit removes the stub from the eligible queue
(spec ⇒ no longer needs-brainstorm; trivial ⇒ same; abstain ⇒ no longer
effective-auto-groomable). No stub is visited twice per drain.

**Concurrency:** no claim field. The CAS push discipline is the only guard; a lost race
skips the stub. This adopts ADR-0004 ("Grooming takes no claim — final-push CAS
suffices"): half of that ADR's rationale (human-attended sessions) does not apply to an
autonomous drain, but the load-bearing half does — each stub's writes land in a single
final commit, so a late collision wastes minutes, not hours, and the mandatory re-read
after a rebased push is the arbiter.

## The groom step — self-brainstorm + critic

The mechanism keeps superpowers' brainstorming *reasoning* and replaces its *interaction
protocol* (ask-one-question-and-wait) with an audit trail. It does NOT invoke
`superpowers:brainstorming` with a simulated human answerer — a subagent picking "the
recommended option" is the model agreeing with itself while faking an approval gate.
Rejected explicitly.

**Designer pass.** Read the stub body, related changes (`related`/`depends_on`
neighbours), the ADR index, `<changes_dir>/LEARNINGS.md` (the learnings ledger —
`docket-groom-next` reads it pre-brainstorm; the autonomous designer gets the same
memory), and relevant code. Enumerate the decision points an
interactive brainstorm would raise. For each, weigh 2–3 approaches and commit to the
conservative / recommended default. Draft the spec to the normal spec path
(`docs/superpowers/specs/…` on the metadata branch). The spec carries an
**`## Assumptions`** block: every decision, the chosen default, the rejected
alternatives, and why — the human's deferred audit trail, readable at the merge gate.

**Critic pass.** A separate adversarial reviewer — a fresh subagent, not the designer —
attacks the draft. Per assumption, verdict:

- **sound** — stands;
- **wrong but fixable from available context** — designer revises; one bounded revision
  round, then the critic re-checks only the revised items;
- **needs human context** — unresolvable autonomously ⇒ the whole groom **abstains**
  (a spec must only be emitted when every decision in it is safe to auto-commit,
  because emission = build-ready = the autonomous builder will build it).

The critic gates ALL build-ready exits: spec emission and trivial verdicts alike.

## Exits

| Exit | When | Effect |
|---|---|---|
| **Spec** | every assumption survives the critic | `spec:` set, `updated:` bumped — build-ready |
| **Trivial** | genuinely mechanical; critic confirms no hidden design decisions | `trivial: true`, no spec — build-ready; reasoning logged in the change body |
| **Abstain** | any needs-human-context decision, or unfixable critic refute | no spec; flip `auto_groomable: false` (explicit override) + dated `## Auto-groom blocked` body section; stays needs-brainstorm |

**Kill and defer are never autonomous.** When the designer concludes a stub should die or
shelve, that is an abstain whose blocked note carries the recommendation ("I think this
should be killed because …"). Verdict authority over the backlog's composition stays
human.

**The abstain record** (`## Auto-groom blocked`, dated like a reconcile-log entry):
the decision(s) that could not be defaulted, what the critic refuted or what context is
missing, what a human would need to supply, and any recommendation. The flag flip is the
dedup guard — the drain never re-selects an abstained stub — and the body section is the
provenance that distinguishes "agent bailed" from "human opted out" (no section ⇒ a
human set the flag). **Re-arm** = human supplies the missing context in the stub and
flips `auto_groomable` back to `true`; next drain re-attempts with the new context.
Forward-compatible with 0009: once the human-escalation-loop exists, the blocked note
upgrades naturally into structured questions-for-you with notification delivery — same
content, plus a channel.

## `docket-groom-next` selection amendment (auto-groom-aware, human decides)

Interactive grooming's queue still includes **every** needs-brainstorm stub — the human
can always groom anything, including auto-groomable stubs (explicit id override
already exists). But default selection prefers the stubs that actually need a human:

1. abstained stubs (`## Auto-groom blocked` — literally waiting on the human), then
2. effective-`auto_groomable: false` stubs, then
3. effective-auto-groomable stubs — sorted last, with a note: "#NNNN is auto-groomable —
   docket-auto-groom will handle it unless you want it now."

Within each band, the shared deterministic order applies. The board surfaces abstained
stubs as **"auto-groom blocked — needs you,"** distinct from plain needs-brainstorm.

## Convention touches (single source, no drift)

`docket-convention` gains the shared vocabulary, referenced (never restated) by both
grooming skills: the `auto_groom` knob, the tri-state `auto_groomable` field and
effective resolution, the autonomous-eligible queue definition, the abstain rule
(flag flip + blocked section), and the groom-next selection bands.

## Out of scope

- **Chaining groom → build** in one autonomous run — future change, with/after 0008.
- **Notifications / escalation delivery** — 0009 + the cloud-routine layer; abstain is
  designed to upgrade into it, not to depend on it.
- **A repo-level "auto mode" umbrella** (autonomy levels in `.docket.yml` beyond the one
  knob) — future, if ever.
- **Any change to `docket-implement-next`** — build-readiness already feeds it.
- **Autonomous kill/defer** — permanently out, by design, not just deferred.

## Testing / verification shape

Skill-suite changes are verified the way this repo already does: a fresh-session load
check of the new skill + the convention edits, a dry run against a sandbox repo with
seeded stubs covering each exit (spec / trivial / abstain / empty queue / lost CAS
race), and a review pass confirming `docket-groom-next` band ordering and board
treatment.
