---
id: 22
slug: consultant-authored-brainstorm
title: Consultant-authored brainstorm — opt-in pinned design agent for the brainstorm role
status: Accepted
date: 2026-07-11
supersedes: []
reverses: []
relates_to: [8, 9, 18]
change: 56
---

## Context

The brainstorm is the only load-bearing design work in docket that runs at
whatever model the session happens to be on. [[0008]] deliberately left the two
interactive skills (`docket-new-change`, `docket-groom-next`) inline with an
advisory-only model recommendation — a skill cannot force the session model — on
the reasoning that a brainstorm is live human dialogue and a subagent was
fire-and-forget. The consequence: on a cheap session the spec that feeds the
autonomous builder (`docket-implement-next`) is cheap-model prose, and there is
no way to run day-to-day sessions on a fast/cheap model while fanning the
*design* thinking out to a high tier.

Two premises behind [[0008]]'s inline-only posture have since shifted:

- **The fire-and-forget premise fell** when harness agent continuation arrived —
  a dispatched agent can now hand work back. (The final design here needs no
  continuation at all; it is a single in-context dispatch. But the premise that
  made a subagent unusable for a brainstorm no longer holds.)
- **Convention-reload token cost was never the load-bearing blocker.** [[0009]]'s
  critic already reloads the convention routinely on every auto-groom gate, so
  "a subagent would have to re-load context" was never the reason to keep the
  brainstorm inline.

Crucially, the interactive skills staying inline is still correct — a brainstorm
*is* live human dialogue, and [[0006]]'s boundary forbids a simulated human. What
changes is not where the dialogue runs, but where the *authorship of the settled
design* runs.

## Decision

docket adds an **opt-in, off-by-default consultant-author pattern** for the
brainstorm role. A new skill `skills/docket-brainstorm` runs the human dialogue
**inline at the session model**, then dispatches a single **pinned consultant
subagent** (`agents/docket-brainstorm-consultant`) which either **authors the
spec** or **returns critique concerns**. The dispatch is a single in-context
return — **no SendMessage, no continuation** — so it is fully harness-portable.

The built-in brainstorm default stays `superpowers:brainstorming`; the consultant
pattern is activated either **per-invocation** (verbal) or **durably**
(`skills: brainstorm: docket-brainstorm`). Default pin **opus/xhigh**, matching
[[0009]]'s critic-tier rationale: design judgment sits at or above the tier of
what it feeds.

This decision stands in a deliberate relationship to three existing ADRs and
respects a fourth boundary:

- **Refines [[0008]], does not reverse it.** The two interactive skills DO stay
  inline; only the brainstorm *role* they invoke may now fan its *authorship* out
  to a pinned consultant. The advisory-only model recommendation for the inline
  skills is untouched.
- **Deliberately deviates from [[0009]].** The auto-groom critic wrapper injects
  `docket-convention` because it judges build-readiness and docket semantics; the
  consultant wrapper injects **NO skill and NO convention**. It authors prose and
  performs **zero docket operations** — no git, no status writes, no board — so it
  needs no docket vocabulary. A compact brief rides the dispatch prompt instead.
- **Consistent with [[0018]].** Binding uses the existing change-0049 `skills:`
  passthrough — no new machinery. If the consultant cannot be dispatched,
  `docket-brainstorm` **degrades to running inline at the session model with a
  prominent warning** — never a hard abort (the [[0018]] degrade-and-warn
  posture, not abort-and-report).
- **Respects the [[0006]] boundary.** No simulated human: the dialogue stays with
  the real human, inline; only the *settled* design is handed to the consultant.

## Consequences

- **Enables a cheap-parent / expensive-design-authorship split**, portable across
  harnesses — day-to-day sessions can run on a fast model while the load-bearing
  design authorship runs pinned at a high tier.
- **The author-or-critique gate keeps a pinned tier load-bearing even though
  option generation runs at the session model** — the consultant either writes
  the spec or hands back concerns, so a pinned tier always touches the artifact
  that feeds the autonomous builder.
- **Cost: one extra pinned dispatch per brainstorm when opted in.** Off by
  default, so no cost is imposed on repos that do not activate it.
- **A new wrapper shape.** `docket-brainstorm-consultant` is the **fourth no-skill
  wrapper** (after `docket-auto-groom-critic`, `docket-rebase-resolver`,
  `docket-integration-repair`) and the **first no-convention one** — every prior
  no-skill wrapper still injects `docket-convention`; this one injects neither a
  skill nor the convention, relying entirely on its dispatch-prompt brief.
