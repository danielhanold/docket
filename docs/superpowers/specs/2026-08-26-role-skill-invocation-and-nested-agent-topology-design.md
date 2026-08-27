<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0355 — Build/review roles are skill-invoked that fan out to profile agents — Step 5 'dispatch' vocabulary invites an agent-not-found misfire](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-27-0355-build-review-roles-are-skill-invoked-that-fan-out-to-profile.md)**
<!-- docket:backlink:end -->

# Role-skill invocation and nested-agent topology — design (change 0355)

## Summary

Change 0355 removes an ambiguity at `docket-implement-next`'s build and review seams. A configured
`skills.*` value is a **skill binding**: the driver invokes that skill, and the invoked skill owns
whatever named-agent topology its contract requires. The binding is not a same-name agent
identifier.

The distinction matters for Docket's shipped build and review roles:

- `docket-build` is invoked as a skill and directs the controller to dispatch one named build
  profile worker per plan task.
- `docket-review` is invoked as a skill and, for that built-in binding only, Step 6 dispatches the
  deterministically selected reviewer-rung wrapper.
- A custom build or review skill owns its own internal topology. The driver never adds Docket's
  profile or rung dispatches on top of a rebound role.

The parent-facing managed dispatch block remains unchanged. It already says to dispatch a workflow
only when a registered same-name agent exists, and to avoid inventing one otherwise. The defect is
inside the role invocation seam, where `docket-implement-next` mixes skill-invocation language with
phrases such as "long build dispatch" and leaves the built-in review fan-out unconditional.

This design clarifies existing decisions in ADR-0059, ADR-0063, ADR-0066, and ADR-0069. It creates
no new role, wrapper, lifecycle state, or architecture decision.

## Problem

### A role binding was mistaken for a named agent

During change 0351, the running implementer reached Step 5 and attempted to dispatch
`docket-build` as a named agent. The active harness rejected the attempt because no such agent is
registered. The run then self-corrected, invoked `docket-build` as a skill, and continued through
the profile workers.

The missing same-name wrapper is deliberate. `docket-build` is a controller skill, while the four
registered workers are `docket-build-economy`, `docket-build-standard`,
`docket-build-premium`, and `docket-build-max`. Likewise, `docket-review` is the shared skill body
behind `docket-review-lean`, `docket-review-standard`, and `docket-review-deep`; there is no
same-name `docket-review` wrapper.

The current Step 5 paragraph invites the mistake. It calls the operation a "long build dispatch,"
then says the resolved build skill is invoked, then discusses an invocable role that "cannot
dispatch." Those three uses refer to different layers but appear as one action. A model can read
the first rejected same-name attempt as the Tier-C failure described by the last clause and falsely
halt a healthy run.

### The review rebound path is semantically ambiguous

The Skill layer promises verbatim passthrough: any custom skill may be bound to `skills.review`.
Step 6 currently says to invoke the resolved review skill and then unconditionally dispatch the
selected Docket reviewer rung. With a rebound such as `superpowers:requesting-code-review`, that
wording can run the custom review and then add a second Docket review that the binding did not ask
for.

The build paragraph is less exposed because it names `docket-build` when describing profile
routing, but it still lacks the general boundary: Docket's worker topology belongs to the built-in
skill, not to every possible `skills.build` value.

### Three failure classes are being collapsed

The workflow already defines three distinct cases:

1. **Missing skill:** the configured binding cannot be invoked, so the Skill layer degrades to the
   role's `auto` fallback with a prominent warning.
2. **Wrong same-name agent attempt:** the caller tried an operation the role contract never
   required. Its rejection says nothing about whether the invoked role can perform its required
   nested dispatches.
3. **Required nested dispatch unavailable:** an invoked discipline role reaches a dispatch its
   contract requires and that capability genuinely fails under ADR-0059. Tier C applies and the run
   halts; an explicitly `auto` binding would have taken the fallback before invoking any skill and
   therefore would never reach this failure.

The design must keep those classes separate. In particular, the absence of `docket-build` or
`docket-review` from an agent registry is neither case 1 nor case 3.

## Decision

### 1. State the generic boundary in the Skill layer

`skills/docket-convention/SKILL.md`'s Skill layer will state one generic rule:

> A resolved `skills.<role>` value names a skill to invoke, not a same-name agent to dispatch. Any
> nested named-agent dispatch belongs to the invoked skill's own contract. The driver must not
> infer or add a topology from the role noun or the configured skill name.

This rule applies to all role bindings without enumerating today's five roles or today's agent
roster. It covers the existing consultant-brainstorm flow without requiring a local edit there:
`docket-brainstorm` is invoked as a skill and its own contract dispatches the specifically named
`docket-brainstorm-consultant`.

The rule sits in the Skill layer because it defines the meaning of a `skills.*` binding. The
Dispatch-capability section continues to define how a required nested dispatch is resolved and how
genuine unavailability is classified.

### 2. Make Step 5 a role invocation with built-in-specific fan-out

`skills/docket-implement-next/SKILL.md` Step 5 will:

1. refresh the claim before the long **build-role invocation** and after it returns;
2. invoke the resolved `$SKILL_BUILD` value with the existing `DIRECTED to:` outcome;
3. state that when the value is `docket-build`, that invoked skill directs the controller to
   dispatch the named build-profile workers and run the full-suite gate;
4. state that a custom build skill owns its own execution and dispatch topology; and
5. retain the existing `auto`, missing-skill, and Tier-C postures with their distinct triggers.

The paragraph will not call the role invocation a build dispatch. It may still use "dispatch" for
the actual nested worker operation because that is the correct action and the distinction needs to
remain visible.

An erroneous attempt to dispatch a same-name `docket-build` agent is a wrong operation, not a
capability probe. The correct recovery is to return to the required skill-invocation path. No
metadata or worktree state has changed merely because that rejected probe occurred.

### 3. Make Step 6 conditional on the resolved review binding

Step 6 will preserve build-evidence validation and Docket's deterministic rung-selection rule, but
make the fan-out conditional:

- When `$SKILL_REVIEW` is `docket-review`, invoke the skill, select the rung from the build record
  and diff-size modifier, and dispatch exactly that rung wrapper foreground with the existing
  payload and worktree requirements.
- When `$SKILL_REVIEW` names another invocable skill, invoke it with the existing `DIRECTED to:`
  outcome and consume its whole-branch findings. Do not dispatch a Docket reviewer rung in
  addition.
- When `$SKILL_REVIEW` is `auto`, run the existing inline whole-branch fallback and dispatch no
  reviewer.
- When the configured review skill is missing, use that same fallback with the existing prominent
  warning.

The deterministic Docket rung selection is therefore part of the `docket-review` binding's
topology, not a universal post-step applied to every review binding. The findings still feed the
same Step 6 triage and fix loop; this change does not alter their downstream handling.

### 4. Apply Tier C only at a required nested-dispatch boundary

The convention and Steps 5–6 will use the following behavior matrix:

| Resolved binding or event | Required behavior |
|---|---|
| `auto` | Run the role's inline fallback; invoke no skill and dispatch no agent. |
| Configured skill cannot be invoked | Run the fallback with the existing prominent missing-skill warning. |
| `docket-build` | Invoke the skill; its contract dispatches build-profile workers. |
| Custom build skill | Invoke it; do not add Docket profile dispatches. |
| `docket-review` | Invoke the skill; select and dispatch one Docket reviewer rung. |
| Custom review skill | Invoke it; do not add a Docket reviewer dispatch. |
| Rejected same-name `docket-build`/`docket-review` attempt | Treat it as the wrong operation, not evidence about required dispatch capability; resume at skill invocation. |
| Required nested profile, rung, or custom-skill dispatch genuinely unavailable | Apply Tier C and halt; do not silently substitute inline work. |

A concrete rejection of an actual required profile or rung agent remains meaningful. For example,
a rejected `docket-build-standard` or `docket-review-deep` dispatch is evidence that the required
named agent is unavailable in this session. The existing stale-install remedy and halt posture
remain intact. What is excluded is treating rejection of a nonexistent same-name **role** agent as
evidence about that required nested edge.

## Source changes

The implementation is deliberately limited to five maintained source files:

1. `skills/docket-convention/SKILL.md` — the generic role-binding boundary in the Skill layer.
2. `skills/docket-implement-next/SKILL.md` — Step 5 invocation language and built-in build
   specialization; Step 6 built-in/custom review split.
3. `tests/test_skill_handoff_precedence.sh` — the derived generic role-invocation guard.
4. `tests/test_docket_build.sh` — the Step 5 built-in/custom topology guard.
5. `tests/test_docket_review.sh` — the Step 6 conditional rung-dispatch guard.

No edit is made to:

- `AGENTS.md`, `CLAUDE.md`, `internal/harness/dispatch.go`, `sync-agents.sh`, or their generated
  fixtures;
- `skills/docket-build/SKILL.md` or `skills/docket-review/SKILL.md`;
- agent wrapper sources or harness defaults; or
- runtime config resolution.

The built-in role skills already describe their own internal topology correctly. The driver and
shared binding contract are the ambiguous surfaces.

## Verification

### Generic role-invocation guard

Extend `tests/test_skill_handoff_precedence.sh`. It already derives every autonomous `$SKILL_*`
invocation site from a whole-repository search rather than hand-listing role names. Its convention
section will additionally assert that the Skill layer states both halves of the generic rule:

- a resolved binding is invoked as a skill rather than dispatched as a same-name agent; and
- nested agent dispatch belongs to the invoked skill's contract.

The derived-site pass will reject dispatch-framed wording at a resolved-role invocation site while
continuing to permit genuine nested-dispatch prose elsewhere. This avoids a whole-file ban on the
word "dispatch," which would reject the behavior the built-in roles are supposed to perform.

### Build seam guard

Extend `tests/test_docket_build.sh` to extract Step 5 and assert:

- the old "long build dispatch" state is absent;
- `$SKILL_BUILD` is still invoked with the `DIRECTED to:` marker;
- `docket-build` is the condition that owns Docket profile routing; and
- the custom-binding branch says no Docket profile topology is added.

The negative assert detects restoration of the observed failure state rather than merely pinning
new prose.

### Review seam guard

Extend `tests/test_docket_review.sh` to extract Step 6 and assert:

- `$SKILL_REVIEW` remains a directed skill invocation;
- Docket rung dispatch is explicitly conditional on the `docket-review` binding; and
- a custom review binding does not receive an additional Docket rung.

The guard must fail if the rung-dispatch sentence is made unconditional again. Anchor it on the
Step 6 syntactic section and the binding branch, not on the bare presence of `docket-review` or a
rung name elsewhere in the file.

### Mutation proof and suite

Before completion, mutation-test each new guard by independently:

1. restoring "long build dispatch" at the Step 5 invocation site;
2. removing the custom build topology clause;
3. making the Docket reviewer-rung dispatch unconditional; and
4. removing either half of the generic Skill-layer rule.

Confirm each mutation actually landed before running its test, confirm the intended assert reddens,
restore from a backup copy rather than from `HEAD`, and re-run the focused tests. Then run the full
suite resolved from `finalize.test_command`, including the repository's budget screening.

## Relationships and compatibility

- Change 0212 and ADR-0069 are the direct precedent: the build and review skill bodies are loaded
  into the driver's context, so their local instructions must remain scoped while the driver
  continues its own sequence.
- Change 0257 is related because it also touches the convention plus build/review contract tests;
  whichever change lands second must reconcile the shared prose and test anchors.
- Change 0283 is related because its settled design keeps the parent-facing dispatch table
  verbatim, matching this design's decision not to expand that surface.
- Change 0351 remains the discovery source and live incident record.
- ADR-0059 governs dispatch-capability evidence and Tier C; ADR-0063 and ADR-0066 own the built-in
  build/review topologies; ADR-0069 governs inline-loaded role-skill provenance.

There are no dependencies. Custom bindings become more faithful to the existing passthrough
contract: no configuration format or supported value changes.

## Out of scope

- Adding same-name `docket-build` or `docket-review` agent wrappers.
- Editing or expanding the managed parent-facing dispatch block.
- Changing profile routing, reviewer-rung selection, build evidence, the review fix loop, or any
  Tier-C halt consequence.
- Changing the built-in build or review skill bodies.
- Introducing a generic runtime role dispatcher or validating custom skill names in config.
- Reworking `docket-brainstorm`; the generic convention rule already describes its correct flow.

## Acceptance criteria

The change is complete when:

1. every resolved non-`auto` role binding is unambiguously described as a skill invocation;
2. the built-in build and review topologies are conditional on their respective bindings;
3. custom build/review bindings receive no extra Docket profile or rung fan-out;
4. a rejected same-name role-agent attempt cannot trigger Tier C;
5. a genuinely unavailable required nested dispatch still triggers Tier C unchanged;
6. the managed parent-facing dispatch block is byte-untouched;
7. the new guards have been mutation-proven; and
8. the full suite is green with no authoritative budget breach.
