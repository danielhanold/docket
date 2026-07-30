# Lean profile-routed build

Design for a first-class Docket build skill that preserves fresh per-task implementers and
test-driven development while removing the review loops and implicit effort selection that make
`superpowers:subagent-driven-development` expensive in long autonomous runs.

## Context

`docket-implement-next` currently resolves its `skills.build` role to
`superpowers:subagent-driven-development` (SDD) by default. For each plan task, SDD dispatches a
fresh implementer, then a fresh task reviewer, and may dispatch repeated fix/re-review pairs. SDD
also performs its own final whole-branch review. Docket subsequently invokes its separately
configurable `skills.review` role for another whole-branch review.

For `T` tasks and `R` failed task-review rounds, the normal composition is approximately
`2T + 2R + 2` nested agent runs: one implementer and one reviewer per task, two more agents per
fix/re-review round, SDD's final reviewer, and Docket's reviewer. Even a clean five-task change is
therefore about twelve nested runs; three review-fix rounds raise it to eighteen.

Fresh per-task implementers are not themselves the defect. They bound task context, isolate
implementation responsibility, and make it possible to assign reasoning effort according to the
task. The multiplicative task-review loop, duplicate branch review, verbose task artifacts,
repeated suite runs, and inherited/implicit effort are the expensive parts.

Upstream SDD's
[Model Selection](https://github.com/obra/superpowers/blob/main/skills/subagent-driven-development/SKILL.md#model-selection)
chooses models by controller judgment and requires the chosen model on each dispatch. Its
implementer and reviewer templates do not provide a corresponding per-dispatch effort choice.
Claude Code can bind both `model` and `effort` in a named subagent's frontmatter, so Docket can make
the cost decision explicit by routing tasks to named agents rather than relying on ambient effort.

Active change 0044 and ADR-0023 attempted to solve only the model-selection part while retaining
SDD's implementer/reviewer topology. This design replaces that premise: remove the repeated review
work first, then make both model and effort explicit for the remaining task workers.

## Goals

- Preserve one fresh implementation subagent per plan task.
- Route each task to an explicit model-and-effort profile.
- Keep focused TDD wherever a meaningful behavioral test is possible.
- Remove per-task reviewer agents, task fix/re-review rounds, and SDD's final review.
- Keep Docket's one independent whole-branch review as the sole review gate for now.
- Run focused tests during tasks and the full suite once after all tasks.
- Continue automatically through one bounded profile escalation when a worker needs more reasoning
  capacity.
- Keep routine output and persisted orchestration artifacts small.
- Ship the capability as a Docket-owned build role without changing other harnesses' default build
  behavior in the Claude-first release.

## Non-goals

- Eliminating fresh per-task subagents.
- Weakening the requirement for tests or replacing TDD with review.
- Promising that a higher-cost profile provides a stronger correctness guarantee.
- Adding hard subagent turn caps.
- Replacing `skills.review` in this change.
- Shipping supported Cursor or Codex task-profile routing in this change.
- Making `docket-build` the global default before the other supported harnesses have an intentional
  profile mapping.

## Architecture

### Build controller and worker contract

The change adds two Docket skills:

- `docket-build` is the build controller invoked inline at `docket-implement-next` Step 5 through
  the existing `skills.build` role. It reads the plan, selects a profile, dispatches tasks
  sequentially, applies the escalation protocol, and runs the build gate.
- `docket-build-task` is the compact worker contract preloaded into each profile agent. It owns one
  task from focused test through implementation, verification, self-review, and commit.

The controller is not a router subagent. Routing is a decision made in the already-running
`docket-implement-next` context. Each selected task gets exactly one fresh worker dispatch unless
that worker requests the single allowed escalation.

The shared worker skill contains the lean TDD and return protocol directly. It does not invoke or
preload SDD, a task-review skill, or another adversarial agent. Workers remain sequential because
later tasks build on earlier task commits and share the feature worktree.

### Claude Code profile agents

The Claude-first release ships three named profile agents:

| Agent | Default model | Effort | Turn cap |
|---|---|---|---|
| `docket-build-economy` | `claude-opus-5` | `low` | none |
| `docket-build-standard` | `claude-opus-5` | `medium` | none |
| `docket-build-premium` | `claude-opus-5` | `high` | none |

All three preload the same `docket-build-task` skill. Their model and effort are their only
behavioral difference. `premium` means greater reasoning investment, not greater assurance; every
profile has the same testing and completion obligations.

The profile entries live under the Claude harness, never the harness-neutral fallback:

```yaml
agents:
  claude:
    build-economy:  { model: claude-opus-5, effort: low }
    build-standard: { model: claude-opus-5, effort: medium }
    build-premium:  { model: claude-opus-5, effort: high }
```

Putting these identifiers under `agents.default` would falsely present Claude model IDs as
harness-portable. Cursor and Codex receive their own mappings only in their follow-up changes.
Model and effort otherwise use Docket's existing generated-agent resolution and may be overridden
at the global, repo-committed, or repo-local layer. No profile emits `maxTurns`.

### Selecting the build

The feature is selected through the existing pluggable skill role:

```yaml
skills:
  build: docket-build
```

Docket's own repository opts in so the feature is dogfooded. The shipped global default remains
`superpowers:subagent-driven-development` in this Claude-first change. Existing users and
unsupported harnesses therefore do not silently acquire a build topology they cannot dispatch.

Selecting `docket-build` without a resolvable profile-dispatch mechanism follows the existing
Tier-C discipline posture: halt and report. An explicit `skills.build: auto` continues to
authorize inline plan execution, but selecting `docket-build` is not implicit authorization to
discard its isolation or model/effort contract.

## Routing

### Explicit override

A plan task may carry:

```markdown
**Build profile:** economy
```

The accepted values are `economy`, `standard`, and `premium`. A valid explicit value is
authoritative and its use is recorded in the task's concise return. An invalid value is a plan
contract error and halts rather than silently falling back.

### Automatic classification

Without an explicit override, the controller applies this rubric:

- `premium` when the task involves authentication or security boundaries, migrations or
  irreversible data changes, concurrency or locking, release infrastructure, or unresolved
  architecture.
- `economy` only when the task is fully specified, localized to roughly one or two files, follows
  an established pattern, and has no consequential risk.
- `standard` for the remaining normal feature, integration, refactor, and debugging work.

The asymmetry is deliberate: `economy` must be positively established, named risk selects
`premium`, and uncertainty defaults to `standard`. The concise routing line names both the profile
and its reason.

### Escalation

Each task may escalate automatically at most once:

```text
economy -> standard -> premium -> halt
```

A worker returns `NEEDS_ESCALATION` only when the task proves materially more complex or risky
than the assigned profile, with a concrete reason. An expected RED test, ordinary debugging, or a
single failed test run is not an escalation condition.

The stronger worker continues in the same worktree. It must inspect and account for any existing
uncommitted changes; it may revise them but must not discard them blindly. A task produces a
commit only on successful completion.

If a premium worker requests escalation, an escalated worker still cannot finish, requirements
contradict, authority or dependencies are missing, or continuation is unsafe, the build returns
`halted`. The change stays `in-progress` and the worktree is preserved for inspection or resume.
Successful escalation continues the same `docket-implement-next` run automatically.

## Worker protocol

Every dispatched worker receives the plan task, current branch and worktree, applicable repository
instructions, selected profile and routing reason, and the compact completion schema. It owns only
that task and must not rewrite earlier task commits or unrelated user work.

Valid outcomes are:

- `COMPLETE`: focused verification is green and exactly one successful task commit exists.
- `NEEDS_ESCALATION`: the profile is insufficient for a concrete complexity or risk reason.
- `BLOCKED`: a stronger model cannot resolve the missing authority, contradiction, dependency, or
  unsafe condition.

A missing or malformed outcome halts. The controller never infers success from a child merely
reporting that it finished.

With checkpointing off, a normal successful return stays short: outcome, selected profile,
routing reason, focused verification command/result, TDD evidence or exception, and commit SHA.
There are no SDD-style brief files, task reports, or per-task review records.

## TDD and task verification

### Default

Where a meaningful behavioral test is possible, each task:

1. Runs the narrowest relevant tests to establish the baseline.
2. Adds or identifies a test that fails for the intended reason.
3. Implements the smallest change that makes it pass.
4. Re-runs the focused test set.
5. Self-reviews the diff and commits the completed task.

Bug fixes require a failing regression test. Guards require mutation evidence: remove or defeat
the thing guarded and verify that the guard turns red. Repository instructions such as
`AGENTS.md` override the generic worker contract.

### Evidence-bound discretion

The worker may skip literal RED/GREEN only when a meaningful pre-implementation failure is
unavailable or misleading. It must state:

- why RED/GREEN was unsuitable;
- what verification replaced it;
- what residual risk remains.

“Small change,” “hard to test,” and “no existing tests” are not sufficient reasons.

The following are examples, not an exhaustive allowlist:

- Documentation-only changes with no executable behavior change. Substitute applicable lint,
  link, rendering, or precise inspection checks.
- Generated artifacts when the generator is unchanged and the task only refreshes its output.
  Verify reproducible regeneration and the expected diff. A generator change still defaults to
  TDD.
- Behavior-preserving refactors already covered by focused characterization tests. Establish green
  coverage before editing and prove it remains green; manufacturing a failing test would
  misrepresent the intended behavior.
- Plan-required manual-only behavior for which no meaningful automated assertion exists. Perform
  the specified manual or static verification and record the residual risk.

## Full-suite build gate

Task workers run focused tests, not the whole suite after every task. After all plan tasks commit,
the controller runs the whole suite once:

1. Use the already-resolved `finalize.test_command` when configured.
2. Otherwise reuse finalize's existing suite auto-detection.

The awkward `finalize` namespace remains the single source rather than introducing a second test
command that can drift.

A green suite proceeds to Docket Step 6 and its single whole-branch review. A red suite does not
invoke review. It becomes one synthetic integration-repair task using the same worker contract:

```text
standard repair worker -> premium escalation -> halt
```

The repair worker diagnoses the cross-task failure, adds regression coverage when appropriate,
fixes it, reruns the full suite, and commits the repair. There is no repeated repair/review loop.
Failure after the premium repair path returns `halted` with the change still `in-progress`.

## Optional checkpointing

Persisted build receipts are configurable and off by default:

```yaml
build:
  checkpoint: false
```

`build.checkpoint` is global-able and resolves with Docket's standard precedence:
repo-local, repo-committed, global, then the built-in `false`. Values other than `true` or `false`
are configuration errors rather than silent fallback.

When false:

- completed work is still durable through per-task code commits;
- the controller retains only the compact in-context worker returns;
- no `.superpowers/docket-build/` files are written;
- a resumed run reconstructs progress conservatively from the plan, commits, code, and tests
  rather than trusting a formal receipt.

When true, the controller writes a compact ignored ledger under
`.superpowers/docket-build/<change-id>/progress.md`. It records the branch, plan path and blob hash,
task identity and status, profile and reason, escalation, TDD evidence or exception, verification,
and commit SHA. This is a state ledger, not a prose task report.

A resumed task is skipped only when its ledger entry is `COMPLETE`, the plan hash still matches,
and its commit is an ancestor of the current branch. Missing, stale, malformed, or contradictory
state never marks a task complete.

The existing committed `.superpowers/` ignore rule covers the ledger; no local-only ignore is
allowed to stand in for that repository guarantee.

## Review boundary

This build deliberately performs no per-task independent review and no final review of its own.
The worker's self-review is part of implementation, not a second agent or adversarial gate.

After the build gate is green, `docket-implement-next` Step 6 invokes the resolved `skills.review`
role once over the whole branch. That existing boundary remains independently configurable and
keeps implementation and review as separate roles.

The current review skill may itself be heavier than needed. Replacing it is a separate change so
this build redesign can be measured without simultaneously changing the sole remaining independent
review gate.

## Observability and cost bound

The controller emits concise, stable lines for:

- task-to-profile selection and reason;
- escalation and reason;
- worker outcome and commit;
- focused verification;
- full-suite command and result;
- terminal build disposition.

No verbose task artifact is written unless `build.checkpoint: true`. Material TDD exceptions and
residual risks flow into the eventual PR description or the already-conditional results artifact.

For a successful `T`-task build with no escalation or integration failure, this phase dispatches
exactly `T` workers. Docket then dispatches its one whole-branch reviewer, for `T + 1` nested runs
across build and review instead of SDD's approximate `2T + 2` clean-path composition. Escalations
and integration repair add work only when their named condition fires.

## Configuration and documentation changes

The implementation extends the existing layered resolver with a nested `build:` block and exports
the checkpoint boolean. It registers the three profile agent keys in the generated-agent layer and
documents their Claude-specific defaults under `agents.claude`; it must not place those IDs under
`agents.default`.

The Docket repository opts in with:

```yaml
skills:
  build: docket-build

build:
  checkpoint: false

agents:
  claude:
    build-economy:  { model: claude-opus-5, effort: low }
    build-standard: { model: claude-opus-5, effort: medium }
    build-premium:  { model: claude-opus-5, effort: high }
```

The canonical example and user documentation explain the routing rubric, plan override, cost
trade-off, checkpoint default, Claude-only support boundary, and how to opt back into SDD.

## Supersession and follow-ups

This change supersedes active change 0044. Its implementation closes 0044 as killed/superseded
rather than rebasing or merging its stale branch and PR.

A new ADR records the profile-routed model-and-effort decision and supersedes ADR-0023. The new
record explains why direct model configuration inside SDD no longer matches the selected topology:
the task reviewer role is removed, task complexity is routed through named agents, and effort is
first-class.

Three separate follow-up changes are created:

1. Cursor support for build-profile dispatch, native model identifiers, and Cursor effort
   semantics.
2. Codex support for build-profile dispatch, native model identifiers, and reasoning-effort
   semantics.
3. A lean Docket-owned `skills.review` replacement: one bounded, read-only whole-branch review with
   explicit model and effort and no recursive review agents.

These are separate backlog changes, not tasks hidden inside this implementation.

## Verification

The implementation is verified at four levels:

1. **Configuration tests**
   - all four layers resolve `build.checkpoint` with the documented precedence;
   - the default is false;
   - malformed booleans fail;
   - Claude profile overrides resolve under `agents.claude`, not `agents.default`.
2. **Generated-agent tests**
   - all three Claude wrappers are generated with the intended model/effort and shared worker
     skill;
   - no `maxTurns` field is emitted;
   - generation and `--check` remain idempotent;
   - unsupported harnesses are not represented as sharing the Claude model identifiers.
3. **Skill contract and mutation tests**
   - remove a profile or escalation edge and watch the guard fail;
   - remove the task-commit requirement and watch the guard fail;
   - reintroduce a task-review/fix-review dispatch and watch the no-review guard fail;
   - remove the focused-test or full-suite gate and watch the relevant guard fail;
   - prove `finalize.test_command` and auto-detection are the build gate's derived sources rather
     than hand-maintaining suite spellings.
4. **Claude Code smoke test**
   - run a small multi-task fixture through all three explicit profile overrides;
   - verify the selected named agent and effort for each dispatch;
   - exercise one automatic escalation;
   - verify the clean path produces one worker per task, no task reviewer, one full-suite run, and
     then one Docket whole-branch review;
   - repeat with checkpointing off and on to prove the default creates no ledger and the opt-in
     path resumes only validated commits.

The repository's entire suite runs at the build gate. Focused structural tests alone are not
sufficient completion evidence.

## Expected outcome

The lean build retains the two mechanisms that buy the most confidence per token—fresh task
implementers and focused tests—while removing the repeated independent reviews that dominate the
current dispatch count. Model and effort become explicit, configurable profile properties; higher
effort is paid only for tasks whose risk or discovered complexity warrants it. Docket still stops
at one independent whole-branch review and the human merge gate.
