# Skills and agents inventory

A skill is a named, reusable instruction set an agent loads for one job; an agent is a separately
launched worker with its own context, pinned to a model and effort. The lists below are derived from
the `skills/` and `agents/` directories at writing time — those two directories
([`../../skills/`](../../skills/) and [`../../agents/`](../../agents/)) are the authoritative
inventory, and each skill's `SKILL.md` and each agent's markdown file own the current detail. Which
agents are actually dispatchable depends on your harness (the tool that runs the agent: Claude Code,
Cursor, Codex, or opencode): the harness's own agent registry, not this page, decides availability.

## Skills

Each line names one skill directory under `skills/` and its job.

- **docket-adr** — record, supersede, reverse, and index architecture decisions.
- **docket-auto-groom** — drain the auto-groomable needs-brainstorm queue with no human, gated by an adversarial critic.
- **docket-brainstorm** — the consultant-author brainstorm flow that produces or critiques a spec.
- **docket-build** — the build role: route each plan task to a profile worker and run one full-suite gate at the end.
- **docket-build-task** — the per-task worker contract: one plan task from focused test through one commit.
- **docket-convention** — the shared contract: config, layout, manifest and ADR format, lifecycle, branch model (pure reference).
- **docket-finalize-change** — close a change out: merge if approved, verify, archive, clean up, refresh the board.
- **docket-groom-next** — groom the next needs-brainstorm change to build-ready through an interactive brainstorm.
- **docket-implement-next** — implement the next build-ready change end to end to an open PR with no human interaction.
- **docket-new-change** — capture a new unit of planned work into the backlog through up-front design.
- **docket-review** — the bounded read-only whole-branch reviewer role.
- **docket-status** — refresh the backlog, sweep merged changes to done, and run health checks.

## Agents

Each line names one agent file under `agents/` and its job.

- **docket-adr** — dispatch wrapper for the ADR-recording workflow.
- **docket-auto-groom-critic** — adversarially review an auto-groom draft and return one verdict.
- **docket-auto-groom** — dispatch wrapper for autonomous grooming.
- **docket-brainstorm-consultant** — pinned design consultant that authors a spec or returns critique concerns.
- **docket-build-economy** — cheapest build profile: fully-specified, pattern-following plan tasks.
- **docket-build-max** — strongest build profile: tasks whose mistakes cannot be walked back.
- **docket-build-premium** — build profile for consequential but correctable named risk.
- **docket-build-standard** — default build profile and uncertainty sink for normal tasks.
- **docket-finalize-change** — dispatch wrapper for the finalize sequence.
- **docket-implement-next** — dispatch wrapper for the autonomous backlog-drainer.
- **docket-integration-repair** — re-green the suite after finalize's rebase in at most two attempts.
- **docket-plan-writer** — write and commit the implementation plan on the feature branch.
- **docket-rebase-resolver** — resolve rebase conflicts during finalize's rebase gate.
- **docket-review-deep** — the deep rung of the whole-branch reviewer.
- **docket-review-lean** — the lean rung of the whole-branch reviewer.
- **docket-review-standard** — the standard rung of the whole-branch reviewer.
- **docket-status** — dispatch wrapper for the status refresh and health scan.

(The `agents/` directory also holds `harness-defaults.yml`, the shipped default model/effort pins —
data the wrappers read, not an agent.)
