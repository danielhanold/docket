<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0430 — Native Codex acceptance for durable review evidence](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0430-native-codex-acceptance-for-durable-review-evidence.md)**
<!-- docket:backlink:end -->

# Native Codex acceptance for durable review evidence

Run only this explicitly assigned acceptance change in the prepared checkout of
`danielhanold/docket`. Stack this change on 425, whose published branch is
`codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor`.
The candidate binary and immutable resource hashes are pinned in the launch manifest.

## Implementation

In the new feature worktree, create `internal/nativeacceptance/value.go` with
`Value() int` returning 1 and `Double() int` returning `2 * Value()`. First add
`TestValue` and `TestDouble` in `value_test.go`, observe failure, implement the functions, and pass
`go test ./internal/nativeacceptance -count=1`. Keep code changes within these two
files; plan and results artifacts are also required.

## Workflow acceptance

Use the registered native planner, one scoped build-profile worker, and the selected
native reviewer. Bind all child dispatches to the parent's run epoch and unchanged
dispatch context. Preserve and observe each native child identity through terminal return.

After workspace preparation and before planner assignment, copy the candidate-pinned
`.agents/skills` tree from the prepared primary into the feature's `.agents/skills`.
The clone's shared Git exclude already ignores these generated resources in feature
worktrees. Verify their hashes against resources.json and verify a clean feature
worktree before dispatch. Do not copy primary fixture code: the worker creates its
assigned files from scratch on the 425 base. Native role registration comes from
the prepared primary's project `.codex/agents`.

Run the complete configured build suite through the candidate's gate driver.
Before native review, use `evidence.record --output` to create a new evidence file in
the launch kit's control directory, verify it against the review head, and declare
the returned path and SHA-256 as the review assignment's build-evidence resource.
Include that canonical directory in read_roots. Never use the JSON envelope as
the evidence file, relax the reviewer validator, or overwrite pinned evidence.

Commit and attach results, perform the configured final certification, publish the
feature branch, and open a real PR in `danielhanold/docket` targeting
`codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor`. Mark only this change implemented after all
postconditions hold. Do not merge. The root parent reports the authoritative keyed
gate verdict after observing its native coordinator through terminal return.

Both configured suite commands remain `go run ./cmd/docket development test`.
Report planner/worker/reviewer native lineage and model pins, evidence path/hash,
reviewed HEAD, plan/results commit identities, PR URL/base, and both gate reports.
