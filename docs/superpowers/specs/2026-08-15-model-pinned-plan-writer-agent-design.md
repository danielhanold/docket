<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0324 — Extract plan writing into a model-pinned internal agent](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0324-model-pinned-plan-writer-agent.md)**
<!-- docket:backlink:end -->

# Model-pinned plan-writer agent — design

**Change:** #0324
**Date:** 2026-08-15
**Status:** Settled

## Problem

`docket-implement-next` is both the autonomous lifecycle orchestrator and the author of the plan
that drives the build. Step 4 invokes the resolved `skills.plan` binding in the implementer's own
context, so the implementer's model and effort also determine plan quality. A user cannot lower the
cost of selection, claims, metadata coordination, and other orchestration without lowering the
capability applied to the detailed implementation plan.

The existing agent layer already gives build workers, reviewers, ADR writing, status, conflict
resolution, and other distinct workloads independent model/effort pins. Plan writing is the blind
spot: it remains a judgment-heavy operation embedded inside the cheaper-to-run controller.

## Goals

- Give plan writing its own harness-first `model` and `effort` configuration.
- Preserve `skills.plan` as the public, pluggable plan-method binding.
- Make the plan writer own a committed, backlink-stamped plan artifact on the feature branch.
- Keep `docket-implement-next` responsible for orchestration, verification, and metadata writes.
- Preserve custom plan-skill output locations rather than hard-coding the Superpowers directory.
- Land without moving judgment or agent dispatch into the parallel Go migration.

## Assumptions and settled constraints

- The planner is an internal composition agent, not a public skill or a sixth workflow role.
- The change lands before change 0315 is groomed and implemented. The two changes are related, but
  this change does not alter 0315's dependency graph.
- Step 4 remains one top-level step, **Worktree + plan**. The new dispatch is a composition seam
  inside that step, not a lifecycle transition.
- The parent consumes only the planner's repo-relative plan path. Git state, not the child's prose,
  proves that planning completed.
- A custom `skills.plan` binding owns its plan location. `docs/superpowers/plans/` is specific to
  `superpowers:writing-plans` and to Docket's `auto`/missing-skill fallback, not a universal path.

## Architecture

### Internal leaf agent

Add `agents/docket-plan-writer.md` as an internal, feature-worktree-scoped agent source. It carries
its complete worker contract in the agent body, like `docket-brainstorm-consultant`; it adds no
discoverable Docket skill and parses no configuration. It does not statically preload the default
plan skill, because `skills.plan` is a runtime passthrough and may name any installed skill.

The agent owns exactly one durable artifact: the plan file committed on the feature branch. It may
read the synchronized metadata worktree, but it performs no Docket metadata mutation, board update,
or status transition.

### Parent/child boundary

`docket-implement-next` retains these responsibilities:

1. Complete Step 3's reconcile and SHA-confirm its metadata push.
2. Create the feature worktree and confirm a clean pre-dispatch state.
3. Resolve `SKILL_PLAN`, `SKILL_BUILD`, learnings enablement, and all repository paths through the
   existing Step-0 export.
4. Record the feature branch's pre-dispatch HEAD and dispatch `docket-plan-writer` in the foreground.
5. Validate the returned path and the feature branch's resulting git state.
6. Land the verified path in the change's `plan:` field under the existing metadata field-write
   rule.

The dispatch payload supplies, without making the child rediscover configuration:

- change id, title, and synchronized change-file path;
- synchronized spec path;
- feature-worktree path and pre-dispatch HEAD;
- the resolved plan and build skill names;
- whether learnings are enabled and, when enabled, the learnings index path; and
- the inputs needed to invoke the artifact-backlink renderer against the change file.

The child reads the change, spec, current feature-tree code, and relevant learnings. Selecting the
relevant finding files from the index is part of planning judgment and therefore belongs inside the
plan-writer context rather than in the lower-cost parent.

## Plan-writer contract

The plan writer runs autonomously and never prompts the human. It performs the following bounded
sequence:

1. Confirm the feature worktree is clean and still at the handed-off HEAD.
2. Read the supplied design and repository context.
3. Invoke the resolved plan skill **DIRECTED to:** write the plan file and stop there. Any execution
   choice is answered internally from the supplied build binding and never surfaced.
4. When `SKILL_PLAN=auto`, author the plan directly. When the resolved skill cannot be invoked,
   apply the existing missing-skill rule inside the child: warn prominently and author the same
   fallback artifact.
5. Determine the path produced by that binding. Superpowers and fallback plans use
   `docs/superpowers/plans/`; a custom skill's own contract determines its location.
6. Run the deterministic backlink renderer on that file, stage only the returned plan path, and
   commit the complete plan artifact on the feature branch.
7. Finish with the single authoritative success line `PLAN_PATH=<repo-relative-path>`.

Informational warning lines may precede the terminal success line so the parent can carry a
missing-skill degradation into the run report and PR body. On failure, the child returns a concrete
blocking diagnostic instead of a `PLAN_PATH` line. It never leaves success-shaped output for an
uncommitted or partially written plan.

## Parent verification

The returned path is a claim, not proof. Before writing `plan:`, the parent verifies from git that:

- the path is a safe repo-relative path contained by the feature worktree;
- the file exists, is tracked, and changed after the recorded pre-dispatch HEAD;
- the worktree is clean;
- the full branch delta since the recorded HEAD contains only the returned plan file;
- the artifact's managed backlink markers are ordered, balanced, and point to change 0324; and
- the existing Step 4 plan-artifact structural requirements hold.

There is deliberately no directory allowlist. Containment, single-artifact scope, committed state,
and backlink identity are stable properties across custom plan bindings; a hard-coded
`docs/superpowers/plans/` check would make `skills.plan` only nominally pluggable.

Once those checks pass, the parent writes the path verbatim to `plan:` and completes Step 4's
existing two-tree postcondition: the plan and backlink are committed on the feature branch, and the
link-bearing metadata field plus rendered Artifacts block have landed on the metadata branch.

## Failure posture

Plan-agent dispatch joins the convention's Tier C discipline class. A silent inline fallback would
recreate the quality coupling this change exists to remove.

- **Dispatch unavailable:** halt by default. An explicitly resolved `SKILL_PLAN=auto` authorizes
  the parent to use Step 4's inline fallback, matching the established Tier C authorization shape.
- **Plan skill unavailable after a successful agent dispatch:** the child degrades to its auto
  authoring path and warns. The independently pinned planner still authors the plan, so the agent
  boundary remains intact.
- **Child ambiguity or authored failure:** halt; do not retry with the parent or a weaker model.
- **Malformed return, unsafe path, unexpected branch delta, dirty worktree, missing commit, or
  invalid backlink:** halt; never adopt, repair, or commit the child's uncommitted output.

Every hard failure follows `docket-implement-next`'s existing disposition contract: append and land
the bare `## Run halted` section with the human remedy, then return `halted`.

## Model and effort defaults

The new short agent key is `plan-writer`, resolved through the existing harness-first `agents:`
layers and emitted by the existing full-agent-set generator. Shipped defaults are exact opaque
passthrough pairs:

| Harness | Model | Effort |
|---|---|---|
| Claude | `claude-opus-5` | `high` |
| Codex | `gpt-5.6-terra` | `high` |
| Cursor | `cursor-grok-4.5-xhigh` | `auto` |
| OpenCode | `openrouter/deepseek/deepseek-v4-pro-0813` | `medium` |

Users override them through `agents.<harness>.plan-writer`, with the same field-level precedence,
runner behavior, bareness rules, and harness-specific emission shapes as every existing agent. No
new config section or plan-specific model key is introduced.

## Documentation and guards

Update the agent-layer and workflow documentation as one coherent surface:

- `docket-convention` names the Step 4 foreground composition, adds plan dispatch to Tier C, and
  updates the generated-wrapper and no-preloaded-skill cardinalities.
- `docket-implement-next` keeps one Step 4 but splits its prose into preparation and dispatched
  plan-authoring/verification phases. Its postcondition remains the two-tree conjunction.
- README agent tables, tuning examples, and `.docket.example.yml` include `plan-writer` and explain
  that `skills.plan` selects the method while `agents.*.plan-writer` selects its execution model.

Tests derive the agent roster from the source glob where possible and directly guard the new
semantic seams where derivation is insufficient:

- shipped-default completeness and the four exact model/effort pairs;
- all four generated harness wrapper shapes and `worktree-scope: feature`;
- Step 4's foreground dispatch and path-only success protocol;
- the parent's no-directory-whitelist verification shape and single-artifact git proof;
- Tier C's default halt and explicit-`auto` authorization;
- mutation checks that removing the dispatch, verification, or a shipped row makes the relevant
  guard fail; and
- updated skill/agent size budgets with rationale.

The build gate runs the repository's resolved whole suite, `scripts/run-tests.sh`, and treats any
trailing `OVER BUDGET:` report as a finding rather than noise.

## Go-migration isolation

The Go migration's approved boundary says agents own authored plans and harness-native dispatch;
Go owns deterministic repository and metadata mechanics. This change stays on the agent side of
that boundary. It changes no Go domain, transaction, repository, workflow, or installer behavior.

Change 0311 already made skills, agent sources, harness defaults, and dispatch instructions embedded
release assets. The implementation therefore runs the existing asset generator and commits its
mechanical outputs: the mirrored authored files, manifest digests, and generated embed source.
Those generated updates are required release synchronization, not a new Go runtime feature.

Change 0324 is `critical` and is implemented before 0315. It relates to 0315 so that change's
reconcile pass treats the settled Step 4 agent seam as current input; it does not modify the Go
program map or introduce a dependency edge into the migration sprint.

## Expected ADR

The build should record the non-obvious boundary: plan writing is a pinned internal composition
agent that owns a git-verifiable plan artifact, while the implementer owns orchestration and
metadata attachment; unavailable dispatch is Tier C rather than a silent inline fallback. The ADR
should relate to ADR-0008 (generated agent layer), ADR-0018 (pluggable skills and missing-skill
fallback), ADR-0044 (autonomy precedence), ADR-0059 (dispatch-capability posture), ADR-0064
(harness-indexed shipped defaults), and ADR-0083 (declared worktree scope).

## Out of scope

- A public `docket-plan-writer` skill or direct human invocation surface.
- A new `skills:` role, a plan-directory config key, or model-ID validation.
- Changing the `superpowers:writing-plans` default or vendored skill behavior.
- Renumbering `docket-implement-next` steps or changing the build, review, finish, or terminal
  disposition contracts.
- Moving plan authorship or agent dispatch into the Go executable.
- Implementing, rescoping, or adding dependencies to change 0315.

## Success criteria

The change is complete when a `docket-implement-next` run can use one model/effort pair for
orchestration and a separately configured pair for plan writing; the internal planner commits and
returns a plan from either the default or a custom location; the parent verifies and attaches that
artifact without trusting child prose; unavailable dispatch cannot silently collapse the boundary;
all generated harness and embedded-asset outputs are current; and the whole suite passes without an
unaddressed budget finding.

## Open questions

None.
