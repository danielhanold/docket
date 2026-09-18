<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0431 — Native Codex acceptance for active worker validation](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-18-0431-native-codex-acceptance-for-active-worker-validation.md)**
<!-- docket:backlink:end -->

# Native Codex acceptance for active worker validation

Use only explicit change 431 in the fresh acceptance-active-validation checkout of danielhanold/docket. Stack on 425; both effective base and PR base are codex/restore-native-codex-dispatch-for-multi-agent-v2-docket-coor. The neighboring manifest pins the candidate executable and resources. Preserve the halted change 430 and its fixture.

## Implementation

Create internal/nativeacceptance/value.go and value_test.go in the new feature worktree. Value() int returns 1; Double() int returns 2 * Value(). First add TestValue and TestDouble, observe the intended failing focused gate, implement the functions, then pass go test ./internal/nativeacceptance -count=1. Limit implementation edits to these files; plan and results artifacts are also required.

## Native execution

Use a registered native planner, one scoped build-profile worker, and a native reviewer. Preserve immutable assignments and private payloads. Bind every scoped child to the outer run epoch and unchanged dispatch context; retain native child identity until terminal return. Parent capabilities and the outer gate key remain private.

After workspace preparation, copy the launch primary's pinned .agents/skills tree into the feature's .agents/skills and verify every copied hash against resources.json. Verify the feature worktree is clean before planner assignment. Project-native roles are registered from the launch primary's .codex/agents. Do not copy fixture implementation code.

The worker finishes edits and its task commit, checks agent.check-inputs at active with unchanged assignment AND private payload locators/digests, then acknowledges its final drive and returns without further writes. Retain the immutable payload's original predecessor; active validation does not admit another launch. Verify acknowledgement through agent.check-receipt. A check after acknowledgement must remain scope-closed. Never bypass validation, omit payload authority, reopen a closed scope, or send parent capability to the worker. Before WAITING handoff check active without committing.

## Review and completion

Run both full configured gate commands: go run ./cmd/docket development test. Read budget reports and address confirmed serial breaches. Before native review, use evidence.record with --output naming a new file under /Users/homer/dev/docket-0425-launch-kit/candidate/acceptance-active-validation/control. Verify the returned record_path at the reviewed HEAD, declare record_path and record_sha256 as a hashed review resource, set build_evidence to its logical id, and include the canonical control directory in read_roots. Do not use the JSON response envelope as the evidence file.

Commit and attach results, complete final certification, publish the feature, and open a real PR in danielhanold/docket against the published 425 branch. Mark only 431 implemented after its postconditions hold. Do not merge. The root parent runs its keyed run.gate-verdict only after the native coordinator terminates, then obeys the report.

Report native planner/worker/reviewer lineage and model pins, binary/resource provenance, active-check and acknowledgement order, durable evidence path/hash, reviewed HEAD, plan/results commits, both full gate reports, branch and PR URL/base.
