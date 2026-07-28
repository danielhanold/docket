---
slug: capability-absence-needs-a-failed-attempt
hook: "An agent's own report that a capability is unavailable is untrusted input — only a failed attempt or a policy denial establishes absence; a missing tool NAME and an unobserved result establish nothing."
topics: [process, subagents, verification]
changes: [137, 135]
created: 2026-07-27
updated: 2026-07-28
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
- 2026-07-28 (#135, PR #127) — **The rule's first live exercise, and it held.** A Tier 2 spike ran
  `cursor-agent -p --output-format text` to probe whether Cursor honors docket's wrapper contract;
  it returned `Error: Authentication required` and never reached a model. The run recorded the
  result as **absent, therefore uninformative** rather than as evidence the contract was wrong —
  the standing evidence rule being that a negative or absent result from an unreliable probe is
  never a verdict, and only a *positive* result carries weight (and only for the surface it was
  observed on). Promoting that silence would have been the exact false-negative shape this finding
  exists to prevent: an absence observed in the wrong surface, converted into a capability verdict.
  The results file states the rule again so a future implementer cannot re-promote the spike to a
  gate. Scoping matters here too — the observation is pinned to `cursor-agent`
  `2026.01.23-916f423`, unauthenticated, headless
  ([[harness-behavior-is-mode-and-version-scoped]]).
