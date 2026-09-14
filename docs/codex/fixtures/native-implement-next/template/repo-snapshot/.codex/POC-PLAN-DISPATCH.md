# Complete native planner handoff — Codex POC

`docket-plan-writer` is an AGENT, whose charter is already loaded by native named-agent dispatch. There is intentionally no `.agents/skills/docket-plan-writer/SKILL.md`. Do not search for or require one. The planner invokes the selected planning SKILL, `superpowers:writing-plans`, using its real snapshot at `<Feature worktree>/.agents/skills/superpowers-writing-plans/SKILL.md`. The coordinator invokes `docket-build` later. No generic planner or fallback plan authoring counts for this POC.

For this fixture the selected plan/build bindings are explicitly `superpowers:writing-plans` and `docket-build`. The installed binary defers the YAML skills configuration, so these POC instructions and the checked-in `.codex/poc-skill-bindings.json` mapping select the snapshots. Do not guess a directory from the agent's name. If a runtime configuration contradicts these bindings, halt with the conflict instead of silently substituting.

## Coordinator: assemble before spawning

After claim/reconcile/workspace creation and the normal claim refresh, read the synchronized metadata record and spec through the normal Docket contract. Construct `evidence/planner-payload.json` using the keys below. These are this POC's handoff fields, not a Docket operation's request schema. Populate every field from this run's resolved values; no placeholders. Retain the exact prepared workspace receipt and use the current pre-dispatch HEAD. This payload contains no parent or worker capability secrets.

| Key | Value |
| --- | --- |
| change_id | Integer 1 |
| title | Actual change title |
| primary_checkout | Canonical absolute primary root |
| feature_worktree | Canonical absolute newly Docket-created feature root |
| feature_ref | Exact refs/heads/... ref |
| pre_dispatch_head | Full current clean feature HEAD |
| workspace_receipt | Absolute path to this run's workspace-created.json |
| boundary_checker | Absolute external check-boundary.py path |
| metadata_worktree | Absolute synchronized metadata root from repository.prepare |
| change_file | Absolute synchronized change-file path |
| spec_file | Absolute synchronized spec-file path |
| backlink_change_path | Change path relative to the metadata root for artifact.backlink |
| plan_skill | superpowers:writing-plans |
| plan_skill_file | Absolute FEATURE snapshot path .agents/skills/superpowers-writing-plans/SKILL.md |
| build_skill | docket-build |
| build_skill_file | Absolute FEATURE snapshot path .agents/skills/docket-build/SKILL.md |
| learnings_enabled | Boolean false |
| learnings_index | null because learnings are disabled |
| test_command | Resolved build.test_command: go test -count=1 ./... |
| build_profile | standard |
| task_count | Integer 1 |
| preserve_baseline_tests | Boolean true |
| assignment | Write and commit the one-task plan only; return PLAN_PATH; do not execute it or ask the human for execution choice. |

Run the external `validate-plan-payload.py` with `--payload <absolute payload file>` from the feature worktree (explicit workdir). It verifies completeness and readable actual snapshots/metadata; it does not resolve Docket operations, dispatch agents, or write a plan. On failure repair missing INPUTS before any dispatch, or halt if the real inputs cannot be obtained. Do not launch a child to discover missing inputs. Save its JSON output as evidence/planner-payload-validation.json.

On success, construct the native spawn `message` with these exact labeled lines populated from the JSON: `Feature worktree:`, `Primary checkout:`, `Feature ref:`, `Pre-dispatch HEAD:`, `Workspace receipt:`, `Boundary checker:`. Then include the COMPLETE validated JSON text, this run's relevant POC instructions, and an explicit directive to follow the selected plan skill in plan-only mode. Do not send only a file path or a summary of the JSON. Save the exact unencrypted message in evidence/planner-dispatch.txt and pass that same text to native `spawn_agent` for `docket-plan-writer`, fork_turns none. The saved message is evidence, not an alternative runner.

## Planner: consume, do not rediscover

Perform the boundary checker first as required by your agent's entry contract. Then read the exact `plan_skill_file` named in the supplied payload and follow it as the selected skill. The agent charter is already loaded; no same-name skill is required. Read the supplied change/spec and current feature code. Honor the original agent charter: plan only, one artifact, backlink/trailer/commit, PLAN_PATH. Resolve artifact.backlink from your own live validated capability catalog. Missing inputs return a concrete diagnostic; never report a nonexistent agent-named skill as mandatory.

The coordinator verifies and attaches this newly committed plan before dispatching the real standard worker. The ordinary worker payload must still include task text, the plan path, branch, updated clean HEAD, boundary bundle, routing reason and the complete prepared child-capability/driver bundle. The plan payload checker does not replace the worker's scope checks.
