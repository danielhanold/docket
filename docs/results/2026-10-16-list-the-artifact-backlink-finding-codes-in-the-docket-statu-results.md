<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0539 — Describe repository-check drift findings by family in the docket-status skill](../changes/archive/2026-10-16-0539-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md)**
<!-- docket:backlink:end -->

# Describe repository-check drift findings by family in the docket-status skill — Results

**Human action:** None required to merge. One optional wording follow-up and one flaky test are noted below.

## Outcome

The docket-status skill's *Findings* paragraph no longer lists drift codes. It also no longer claims that `repository.repair` fixes every drift finding. It now names the family (board, ADR index, a change's `## Artifacts` block, or the backlink at the top of a spec, plan, or results file) and says each finding carries its own remedy. Most clear with repair. A broken marker, or a file linked from more than one change, needs a hand edit first. The skill tells the agent to relay that remedy. The *Sweep posture* line is unchanged and still accurate.

The glossary's *Derived view / generated block / backlink* example now reads `# reports links blocks and backlinks that are stale or broken` for `repository check` and `# re-renders the stale ones in one metadata commit` for `repository repair`.

Two `alignmentContracts` rows in `internal/repoguard/prose_contracts_test.go` keep the old code list and the "repaired through `repository.repair`" claim from coming back. The docket-status word budget was re-baselined from 2923 to 2983 words, and the embedded assets were regenerated.

## Verification performed

- Each guard row was checked red before and green after its edit, and the skill guard was mutation-checked by restoring the old sentence.
- The full suite (`go run ./cmd/docket development test`) passed on the final head.
- Review: the standard-tier review returned one minor finding. Its fix was reverted when the post-fix suite run failed on an unrelated flaky test, so the finding stands (see below).
- Backlog check: 24 proposed or deferred changes checked.

## Known issues and follow-ups

- **The glossary repair comment slightly understates repair (confirmed, minor).** The comment "re-renders the stale ones" leaves out that repair also restores a *missing* `## Artifacts` block (`artifact-links-missing` is repairable in `internal/reposetup/derived.go`). A reader might think a missing block needs a hand edit. Practical impact is small. A fix ("the stale or missing ones", plus the matching guard string) was made but reverted. The post-fix suite run went red on the unrelated flake below, and the fix-pass rule reverts non-blocker fixes on a red run. Suggested next action: re-apply the one-line fix on this PR before merging, or in a follow-up. This is this branch's own diff, so it gets no backlog verdict.
- **`TestObserveRunningThenTerminal` in `internal/process` is flaky (confirmed once).** One full-suite run failed with `timed out waiting for vanished: last observe state=signaled` after 30s. The next run on a docs-only revert of the same tree passed. When it fires, it fails a build gate for reasons unrelated to the change. Workaround: re-run the suite. Suggested next action: **Fits #527** (its "tests that race a timer" item covers the same timing-dependent process-observation pattern; this one is in `internal/process`, not the gate driver). #527 is groomed and proposed, so edit it through `docket-groom-next 527` (revise) to add this test.
