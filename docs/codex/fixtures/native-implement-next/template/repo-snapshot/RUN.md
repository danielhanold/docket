# Run boundary

Run one native coordinator chain from the primary checkout:

claim → reconcile → create feature worktree → native plan writer with explicit worktree targeting → attach new plan → native standard build worker with explicit worktree targeting → real RED/GREEN and commit → final full-suite gate → safe results checkpoint → deliberate halt → parent gate verdict.

This is option 2. A child starting in primary is expected and permitted; writing feature artifacts there is not. Entry validation and explicit operation paths replace startup-cwd equality. The external checker is a read-only test assertion, not an agent launcher or a tool wrapper. Read .codex/POC-WORKTREE.md and AGENTS.md for the exact boundary.

Fixture source behavior: trim surrounding Unicode whitespace in Greet, preserve internal whitespace and the existing greeting format. Exactly one standard-profile task. Parent Terra/low; coordinator Terra/low; plan writer Sol/medium; standard worker Terra/medium. No Luna, agent.enter, review, PR or merge. Start once in a fresh Local app task or fresh CLI session at the primary repo. Do not select the app's Worktree mode.

The helpers, source references and launch instructions live in the fixture's parent directory. No agents or gates were launched during preparation; the candidate must still be unclaimed and no feature worktree or plan may exist when the run begins. After this POC, production progressive disclosure belongs in Codex-only skill/agent references as a separate change.


## Required planner handoff correction

Before native planner dispatch, read `.codex/POC-PLAN-DISPATCH.md`. `docket-plan-writer` is an agent and has no same-name skill snapshot. Its actual selected skill is `superpowers:writing-plans`, included at `.agents/skills/superpowers-writing-plans/SKILL.md`; the selected builder remains `docket-build`. The coordinator must write the full planner payload, run `@@FIXTURE_ROOT@@/validate-plan-payload.py --payload <absolute JSON path>` with explicit feature workdir, capture PLANNER_PAYLOAD_OK, and pass the entire validated JSON plus entry labels in the native message. Preserve the exact dispatch message in external evidence. Missing real inputs must be resolved before spawning; do not invent a docket-plan-writer skill or treat directory binding as the complete assignment.
