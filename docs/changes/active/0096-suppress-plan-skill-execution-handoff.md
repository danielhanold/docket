---
id: 96
slug: suppress-plan-skill-execution-handoff
title: An autonomous run can be halted by a sub-skill's interactive hand-off — pre-specify the outcome at every autonomous call site
status: in-progress
priority: high
created: 2026-07-18
updated: 2026-07-18
depends_on: []
related: [16, 44, 49, 61, 95]
adrs: [24]
spec: docs/superpowers/specs/2026-07-18-autonomous-skill-handoff-precedence-design.md
plan: docs/superpowers/plans/2026-07-18-autonomous-skill-handoff-precedence.md
results:
trivial: false
auto_groomable:
branch: feat/suppress-plan-skill-execution-handoff
claimed_at: 2026-07-18T21:25:10Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-18-autonomous-skill-handoff-precedence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-18-autonomous-skill-handoff-precedence-design.md) |
| Plan | [2026-07-18-autonomous-skill-handoff-precedence.md](https://github.com/danielhanold/docket/blob/feat/suppress-plan-skill-execution-handoff/docs/superpowers/plans/2026-07-18-autonomous-skill-handoff-precedence.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md) |
<!-- docket:artifacts:end -->

## Why

`docket-implement-next` guarantees it "runs with **no human interaction**". On 2026-07-18, during
the build of change 0095, it broke that guarantee: the forked run stopped after planning and asked
the human which execution mode to use, ending the fork. Everything after — the whole six-task
build — ran in the parent session instead of the isolated subagent.

**The prompt is not docket's.** It is `superpowers:writing-plans` §"Execution Handoff"
(`SKILL.md:156-174`), which instructs: *"After saving the plan, offer execution choice… 1.
Subagent-Driven (recommended) / 2. Inline Execution… Which approach?"* — the text that surfaced,
verbatim. `docket-implement-next` §4 invokes that skill as the resolved `$SKILL_PLAN`, so its
closing instruction lands inside an autonomous run.

**Three live instructions collide.** The plan skill says *ask the human*. The agent wrapper says
*"never an interactive prompt"* (the abort-and-report rule every autonomous wrapper carries). The
skill body's §5 says the build method is already decided — *"the resolved build skill —
`$SKILL_BUILD` from the Step-0 config export"*. Two of the three say don't ask; the third says ask;
nothing states which wins. Resolution is left to model judgment, and it is not stable.

**Measured, not theorized.** Across every `docket-implement-next` fork on this machine — 39 runs
from 06-21 to 07-17 — none surfaced the question; each picked SDD and dispatched 8–31 subagents
from inside the fork. Run 40 complied with the plan skill instead. Machinery was verified intact
and is *not* the cause: `context: fork` frontmatter present, wrapper present, and the model pin
held exactly (`claude-opus-4-8`, effort `xhigh`, read from the fork's own log). It is a latent
ambiguity that had simply always landed the same way.

The cost is not correctness — 0095 completed and was fully reviewed — it is the autonomy contract
and context isolation, failing unpredictably at roughly 1-in-40 with no signal that it will.

**Grooming sized it and found the fix already half-built.** Two of the four default role skills
carry a hand-off, not one: `writing-plans` (plan) and `finishing-a-development-branch` (finish);
`subagent-driven-development` and `requesting-code-review` do not. And the finish skill — the more
prescriptive of the two, with two option menus and a destructive option behind a typed confirmation
— was invoked in all 40 runs and never derailed one, because §7 already pre-specifies its outcome
at the point of invocation. §4 does not. That difference is the whole defect, and the remedy is a
technique this repo has already proven against a harder case.

## What changes

- **Pre-specify the outcome at each autonomous call site** — the load-bearing fix. §4 invokes
  `$SKILL_PLAN` directed to write the plan and stop, then proceeds via `$SKILL_BUILD` with no
  prompt, answering any choice internally from resolved config. §7 stands and cites the rule.
  `docket-finalize-change:124` keeps its interactive close-out, with the human-present condition
  made explicit as the one exception. Directions are phrased by shape, never by citing a vendored
  heading, so a superpowers upgrade cannot silently stale them.
- **State the precedence once** in `docket-convention`'s *Skill layer*: an invoked skill's
  interactive step never outranks an autonomous caller's no-prompt rule, and pre-specification is
  the mechanism. This is durability for future bindings, **not** the fix — two general
  "don't prompt" instructions already existed and both lost at run 40. The spec records why, so a
  re-slim cannot keep the tidy paragraph and drop the call-site directions.
- **Trace conditionally** — one run-output line naming the role and skill, only when a hand-off was
  actually met and suppressed. Not the PR body; not every run (#0066's unconditional warning decayed
  into boilerplate).
- **Guard by coverage, not presence** — assert every autonomous role invocation carries a
  pre-specified outcome, with finalize's human-present condition permitted. Fails against today's
  §4 on arrival.

## Out of scope

- **Editing the superpowers plugin.** `writing-plans` is vendored under
  `~/.claude/plugins/cache/.../superpowers/6.1.1/`; a local edit is overwritten on upgrade. The fix
  must live in docket's own prose.
- Changing which build skill is used, or the `skills.build` binding itself — `$SKILL_BUILD`
  already resolves correctly (#0049). This change makes the run honor it without asking.
- SDD's per-dispatch model selection (#0044, blocked) and the interactive skills
  (`docket-new-change`, `docket-groom-next`), which are *supposed* to prompt.
- **Behavioral testing** — asserting no prompt surfaces in a real run. The failure is model
  judgment at ~1-in-40 inside a fork; not deterministically reproducible. A bounded decision, not
  an oversight.

## Open questions

None — all three were resolved at grooming (see the spec).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-18 — reconciled, scope unchanged

Verified every premise against `origin/main` at reconcile time; all three call sites read exactly as
the spec describes, so the design stands with no scope adjustment.

- **§4 is still the open-ended site.** `skills/docket-implement-next/SKILL.md:64` invokes
  `$SKILL_PLAN` with no outcome pre-specified — the defect is live, unchanged since grooming.
- **§7 still pre-specifies** (`:80`, "DIRECTED to: push the feature branch and open a PR — do NOT
  merge — then stop"), and `skills/docket-finalize-change/SKILL.md:124` still opens with "When a
  human is present". Both remain valid as the reference shape and the permitted exception.
- **Related changes clear.** #0095 reached `done` (archived 2026-07-18) and its ADR-0043 retires the
  bot-approve subsystem — no overlap with this change's surface. #0044 is still `blocked`, so SDD
  per-dispatch model selection stays out of scope as written.

**One constraint folded in that the spec predates naming:** `tests/test_skill_size_budgets.sh`
(change 0085) budgets every `skills/**/*.md` by lines and words, and this change adds prose to three
of them. Current headroom is adequate but not generous — convention 294/317 lines and 4769/5104
words, implement-next 127/140 and 2641/2845, finalize-change 132/160 and 2266/2699. The build keeps
the additions inside those budgets rather than raising the table rows; the precedence paragraph in
particular must earn its ~13 lines. If a budget must rise, the row is edited in the same diff, per
that test's own instruction. The new `tests/test_skill_handoff_precedence.sh` needs no registration
— the suite is discovered by glob.
