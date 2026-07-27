---
slug: capability-absence-needs-a-failed-attempt
hook: "An agent's own report that a capability is unavailable is untrusted input — only a failed attempt or a policy denial establishes absence; a missing tool NAME and an unobserved result establish nothing."
topics: [process, subagents, verification]
changes: [137]
created: 2026-07-27
updated: 2026-07-27
promotion_state: candidate
promoted_to:
---

## Apply
When a run reports that some runtime capability is missing — a dispatch mechanism, a skill, a tool —
treat that report as a hypothesis, not a finding. Resolve before concluding:

1. **Search the deferred surface.** Tool sets can be partially deferred behind a search/lazy-load
   layer, so a capability is trivially *unobserved* without ever having been *resolved*.
2. **Escalate to one trivial attempt** when resolution is inconclusive. That attempt is the evidence.
3. **Only a failed attempt or an explicit policy denial establishes unavailability.** The absence of
   a tool with a particular *name* never does, and neither does not having seen a result.

A tool name is a **diagnostic, never a decision input** — never write prose that makes some specific
name load-bearing, because it primes the next agent to probe for that literal and conclude absence
when the mechanism ships under a different one.

Related: [[skill-fallback-degrades-discipline]] is the downstream symptom (the fallback fires
correctly on a false premise and the discipline silently drops); [[verify-the-claim]] is the same
posture applied to documents; [[harness-behavior-is-mode-and-version-scoped]] bounds how long any
probe result stays true.

## War story
- 2026-07-27 (#137, PR #126) — Two live runs (#127, #136) reported "no subagent-dispatch (`Task`)
  tool", so `superpowers:subagent-driven-development` and `superpowers:requesting-code-review` both
  fell back to their inline `auto` paths. Every artifact still looked complete. A probe found the
  premise was false on every count: there is no tool named `Task` in current Claude Code at all —
  the mechanism is named `Agent` — and dispatch, nesting, and `Skill` all resolve on **both** the
  agent-dispatched and `context: fork` paths. Docket's own prose had named `Task`, priming the
  probe. `AskUserQuestion` really is absent, so ADR-0024's fork-exclusion stands.
- The spike **reproduced the defect while being the instrument that tested it**: a dispatched child
  issued a nested dispatch, backgrounded it, yielded, and — having no result in context — reported
  `NESTING: FAILED`. The transcript showed the nested child had run and replied `NESTED_OK`.
  Dispatch succeeded; only *retrieval* failed, and the agent converted that into a reported
  capability gap. An independent re-derivation of the convention's never-yield rule.
