# Autonomous skill hand-off precedence — design

**Change:** #0096
**Date:** 2026-07-18
**Status:** Settled

## Problem

`docket-implement-next` advertises that it runs with **no human interaction**. On 2026-07-18, during
the build of change 0095, it broke that guarantee: the forked run stopped after planning and asked
the human which execution mode to use, ending the fork. The remaining six-task build ran in the
parent session instead of the isolated subagent.

The prompt is not docket's. It is `superpowers:writing-plans` §"Execution Handoff", which docket
invokes as the resolved `$SKILL_PLAN` in §4. Three live instructions collide at that moment: the
plan skill says *ask the human*; the autonomous wrapper's abort-and-report rule says *never an
interactive prompt*; §5 says the build method is already resolved from config. Two say don't ask,
one says ask, and nothing states which wins. Resolution is left to model judgment, and it is not
stable: across 39 prior forked runs on this machine the question never surfaced; run 40 complied
with the plan skill instead. The cost is not correctness — 0095 completed and was fully reviewed —
it is the autonomy contract and context isolation, failing at roughly 1-in-40 with no advance signal.

## What the survey found

Reading the four resolved default skills at `superpowers/6.1.1`:

| Role | Default skill | Interactive hand-off? |
|---|---|---|
| plan | `writing-plans` | **Yes** — §"Execution Handoff", "Which approach?" |
| build | `subagent-driven-development` | No — explicitly anti-prompt |
| review | `requesting-code-review` | No |
| finish | `finishing-a-development-branch` | **Yes** — two option menus, one destructive option behind a typed confirmation |

Two of four, not one. This answers the stub's third open question and rules out treating the defect
as a `writing-plans` quirk.

Three call sites invoke a hand-off-bearing role skill:

1. `docket-implement-next` §4 — `$SKILL_PLAN`. **Open-ended.** This is the defect.
2. `docket-implement-next` §7 — `$SKILL_FINISH`, invoked *"DIRECTED to: push the feature branch and
   open a PR — do NOT merge — then stop."*
3. `docket-finalize-change:124` — `$SKILL_FINISH`, scoped *"When a human is present"*, deliberately
   allowing its chooser to drive a non-standard close-out.

## The key evidence

**The mechanism already exists and is proven against a harder case.** `finishing-a-development-branch`
is markedly more prescriptive than the section that failed — *"present exactly these 3 options"*,
*"Don't add explanation"*, a four-option menu, a destructive option gated on typed confirmation.
It was invoked in **every one of the 40 runs, including run 40**, and has never surfaced a menu into
an autonomous run. `writing-plans` §"Execution Handoff" is by comparison ordinary prose — *"offer
execution choice"*, with no hard gate and no stop instruction — and it derailed a run.

The only difference between the two sites is that §7 pre-specifies the outcome at the point of
invocation and §4 does not.

**There is also no genuine conflict to win at §4.** Both branches of the plan skill's hand-off
terminate in a REQUIRED SUB-SKILL, and the branch it recommends is `subagent-driven-development` —
precisely docket's `$SKILL_BUILD` default. Pre-specifying does not override the skill; it answers a
question whose answer `skills.build` already holds, with the value the skill itself recommends.

## Design

### 1. Call-site pre-specification — the load-bearing element

Each autonomous invocation of a resolved role skill states the outcome up front, so any choice the
sub-skill would pose is already answered when it is reached.

- **§4** — `$SKILL_PLAN` is invoked directed to write the plan and stop; the build then proceeds via
  `$SKILL_BUILD` with no prompt. Any execution-mode or option choice the plan skill poses is
  answered internally from the already-resolved config, never surfaced.
- **§7** — wording stands; it cites the general rule rather than restating its rationale.
- **`docket-finalize-change:124`** — the human-present condition is made explicit as *the* exception:
  the chooser may drive a non-standard close-out only on the human-attended path; the autonomous
  path pre-specifies its outcome like §7.

**Direction is phrased by shape, never by citing a vendored heading.** "Any execution-mode or option
choice it poses" survives a superpowers upgrade that renames or restructures the section; a
reference to §"Execution Handoff" would silently go stale at 6.2 while continuing to look correct.

### 2. Convention precedence rule — durability, not enforcement

One paragraph in `docket-convention`'s *Skill layer*, alongside the missing-skill and `auto` rules
it already owns:

> An invoked skill's interactive step never outranks the caller's autonomy contract. When an
> autonomous skill (one carrying a wrapper's abort-and-report rule) invokes a resolved role skill
> that ends in a human hand-off, the caller **pre-specifies the outcome** in its direction to that
> skill and answers any choice internally from already-resolved config. Interactive skills
> (`docket-new-change`, `docket-groom-next`) are unaffected — their prompts are the product.

**This paragraph is necessary but not sufficient, and the spec says so on purpose.** The wrapper's
abort-and-report rule and §5's "the build method is already resolved" are both general instructions
that already said *don't prompt* — and both lost at run 40. A third general instruction, read at
Step 0, is not what beats a specific instruction read at the moment of invocation; a specific
counter-instruction at that same moment is. The paragraph's job is to tell a future author why the
call sites read as they do, and what to do when binding a new role skill. It is not the fix.

A future re-slim must therefore not "simplify" by keeping the tidy general paragraph and dropping
the call-site directions — that would delete the working part and retain the decorative one.

### 3. Suppression trace — conditional, run output only

When a hand-off is encountered and suppressed, emit one line naming the role and the resolved skill.

- **Conditional, not unconditional.** A line on every run decays into boilerplate — exactly what
  happened to the missing-skill warning in #0066, where plan, build, and review all silently
  degraded to `auto` across an entire build, were correctly warned, and nobody acted
  (`skill-fallback-degrades-discipline`). A line that appears only when a hand-off was actually met
  carries information: if `requesting-code-review` grows one at 6.2, a *new* line appears.
- **Run output only, not the PR body.** The missing-skill warning reaches the PR body because a
  degraded binding changes what shipped. A suppressed prompt does not.
- **Best-effort.** This depends on the model noticing it suppressed something — the same judgment
  that failed at run 40. It is a breadcrumb, not a guarantee. Enforcement is §1.

### 4. Guard — coverage, not presence

`tests/test_skill_handoff_precedence.sh` asserts:

1. The precedence paragraph is present in `docket-convention`'s *Skill layer*.
2. **Every autonomous invocation of a resolved role skill carries a pre-specified outcome** — with
   `docket-finalize-change:124` permitted only via an explicit human-present condition.

Assertion 2 is the point. A presence check alone would test the part that does not work (§2) while
leaving the part that does (§1) unguarded. The test **fails against today's §4 on arrival**.

The guard keys on autonomous wrappers only. `docket-new-change` and `docket-groom-next` are supposed
to prompt — this change was itself groomed through such a prompt sequence.

Per `foundational-test-discipline` (#0093), a sentinel is sampling rather than parsing; this one is
paired with the whole-branch review that reads the call sites for meaning.

## Out of scope

- **Behavioral testing** — asserting that no prompt surfaces in a real run. The failure is a
  model-judgment event at roughly 1-in-40 inside a fork; it is not deterministically reproducible.
  Named here so it reads as a bounded decision rather than an oversight.
- **Editing the superpowers plugin.** `writing-plans` is vendored under
  `~/.claude/plugins/cache/.../superpowers/6.1.1/`; a local edit is overwritten on upgrade. The fix
  lives in docket's own prose.
- **Changing which build skill is used**, or the `skills.build` binding. `$SKILL_BUILD` already
  resolves correctly (#0049); this change makes the run honor it without asking.
- SDD's per-dispatch model selection (#0044, blocked).

## Expected ADR

The build is expected to produce an ADR recording the precedence decision — which contract wins when
an invoked skill's interactive step meets an autonomous caller's no-prompt rule, and that
pre-specification at the call site (not general precedence prose) is the mechanism. It relates to
ADR-0018 (pluggable skills — passthrough and degrade-to-auto), ADR-0008 (the agent layer), and
ADR-0024 (`context: fork` dispatch).

## Open questions

None. All three of the stub's open questions were resolved during grooming: the survey settled the
scope question (two of four roles), the rule lands in both the convention and the call sites, and the
trace is conditional.
