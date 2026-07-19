---
id: 44
slug: autonomy-precedence-call-site-pre-specification
title: Autonomy precedence is enforced by pre-specification at the call site
status: Accepted
date: 2026-07-18
supersedes: []
reverses: []
relates_to: [18, 8, 24]
change: 96
---

## Context

docket's workflow steps are pluggable *roles* (convention *Skill layer*, change 0049):
`docket-implement-next` invokes resolved role skills — by default `superpowers:writing-plans` (§4),
`superpowers:subagent-driven-development` (§5), `superpowers:requesting-code-review` (§6), and
`superpowers:finishing-a-development-branch` (§7). These are vendored third-party skills and are
never edited locally — they live under the plugin cache and are replaced wholesale on upgrade.

Two of the four default role skills contain an interactive hand-off. `writing-plans` has an
"Execution Handoff" step that asks the operator to choose Subagent-Driven vs. Inline execution.
`finishing-a-development-branch` presents a close-out option menu (one option gated behind a typed
confirmation). When `docket-implement-next` runs autonomously — as a forked subagent or a wrappered
agent, with no human on the other end — such a prompt has no answer available.

The motivating incident (build of change 0095, the 40th autonomous run on this machine): the plan
skill's hand-off fired *inside* the fork and stopped it after planning, even though two general,
already-in-context instructions said not to ask — the wrapper's standing abort-and-report rule, and
§5's own statement that the build method ($SKILL_BUILD) is already resolved from config. 39 of 40
prior runs suppressed the same prompt; run 40 did not. General precedence prose sitting elsewhere in
context was not reliably decisive at the moment the sub-skill's own instruction was read.

The survey (see `docs/superpowers/specs/2026-07-18-autonomous-skill-handoff-precedence-design.md`)
found this affects two of the four default role skills, not one, and also found the mechanism that
already works: `$SKILL_FINISH` at `docket-implement-next` §7 is invoked "DIRECTED to: push the
feature branch and open a PR — do NOT merge — then stop," and its skill
(`finishing-a-development-branch`) is markedly more prescriptive than the plan skill's hand-off — yet
it never surfaced a menu in any of the 40 runs, including run 40. The only difference between the
two sites is that §7 pre-specifies the outcome at the point of invocation and (pre-fix) §4 did not.

## Decision

Two parts; the second is the load-bearing one.

1. **State the rule once**, in the convention's *Skill layer*: an invoked skill's interactive step
   never outranks the caller's autonomy contract. An autonomous caller (one carrying a wrapper's
   abort-and-report rule) answers any choice a sub-skill poses internally, from already-resolved
   config, and emits one run-output line naming the role and skill only when a hand-off was actually
   met and suppressed.
2. **Enforce it at each call site** by pre-specifying the outcome in the direction given to the role
   skill, using a single house marker — `DIRECTED to:` — e.g. `docket-implement-next` §4: "DIRECTED
   to: write the plan file and stop there." What beats a specific instruction read at the moment of
   invocation is a specific counter-instruction delivered at that same moment; general precedence
   prose sitting elsewhere in context is exactly the shape that already lost at run 40.

Corollaries:

- Directions are phrased by **shape** ("any execution-mode or option choice it poses"), never by
  citing a vendored section heading — a plugin upgrade would silently stale such a citation while
  every doc sentinel stayed green.
- The rule binds only skills with a generated wrapper (the autonomous set). Skills with no wrapper —
  `docket-new-change`, `docket-groom-next` — are interactive and unaffected: their prompts are the
  product.
- `docket-finalize-change`'s human-present close-out (`skills/docket-finalize-change/SKILL.md:124`)
  is the single exception, and it is conditional on exactly that — on the autonomous path finalize
  does not invoke the finish skill at all; docket's own steps drive the close-out and the
  rebase-retest gate governs the merge.
- The convention paragraph is **durability for future bindings, not the enforcement**: a future
  slimming round must not keep the general paragraph and drop the call-site directions, which would
  restore the exact failure mode (a third general instruction that also loses).

## Consequences

**Enables.** Any third-party role skill can be bound via `skills:` without auditing it for
interactive steps, since the caller pre-specifies the outcome regardless of what the bound skill
contains.

**Costs.** Every new autonomous role-invocation site must carry the `DIRECTED to:` marker. This is
enforced by the coverage guard `tests/test_skill_handoff_precedence.sh`, which derives sites from a
whole-repo grep for `$SKILL_*` / `${SKILL_*}` under `skills/`, requires the marker on each site owned
by a wrappered skill (autonomy is determined by the presence of a generated `agents/<skill>.md`
wrapper, not by name), and pins the finalize exception as the only one — with a `checked >= 5`
floor and an `exceptions == 1` floor guarding against the classifier going silently vacuous.

**Given up.** docket cannot fix this by patching the vendored skills (`writing-plans` and the others
live under the plugin cache and are overwritten on upgrade), so the contract lives entirely in
docket's own prose and must be re-asserted at each call site rather than centralized in one place.

**Out of scope (see spec).** Behavioral testing that no prompt ever surfaces in a real run — the
original failure is a model-judgment event at roughly 1-in-40 inside a fork, not deterministically
reproducible; editing the vendored superpowers plugin; and changing which build skill is bound
(`$SKILL_BUILD` already resolved correctly before this change — change 0049 — this change makes the
run honor it without asking).

**Cross-links.** Change 96. Relates to ADR-0018 (pluggable workflow skills — unvalidated
passthrough + degrade-to-auto on a missing skill — the same unvalidated-binding property that means
a bound skill's interactive steps can't be known in advance), ADR-0008 (the generated-subagent agent
layer, which is what makes a skill "autonomous" in this ADR's sense — the presence of a wrapper), and
ADR-0024 (Claude Code `context: fork` skill dispatch — the fork mechanism is what puts the
implementer in a session with no human to answer a prompt).
