<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0455 — Document finalize's record-invalid reason in the docket-finalize-change skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-27-0455-document-finalize-s-record-invalid-reason-in-the-docket-fina.md)**
<!-- docket:backlink:end -->
# Document finalize's record-invalid reason in the docket-finalize-change skill — Results

**Human action:** None required beyond reading the two changed skill paragraphs in the PR diff. This is a documentation-only change.

## Outcome

Change 0449 made two operations that touch GitHub, `finalize.merge` and `pr.publish`, refuse with the reason `record-invalid` when the change, or a record it structurally depends on, fails validation. Neither skill mentioned this, so an agent that hit the refusal had no documented fix.

Both skills now describe the refusal:

- `docket-finalize-change` step 8 says the merge refuses with `invalid-state`/`blocked` and reason `record-invalid` before any merge call. The check covers the change, its `depends_on` targets, its stack ancestors, and any other record carrying its id. It never follows `related`, `discovered_from`, or ADR links. The fix is to repair exactly the records the result's `findings` name and then re-run. The step also explains that a PR merged outside docket, with a broken record in scope, now reports `record-invalid` rather than `already-merged`, and that this is expected.
- `docket-implement-next` ("Publish the PR") says `pr.publish` refuses `record-invalid` (`invalid-state`) under the same scope before any GitHub call. The fix is the same: repair the named records and re-publish.

The skill word budgets in `internal/repoguard/budgets_test.go` were raised to the exact new word counts, and the embedded asset bundle was regenerated.

One planned item was dropped. The spec also asked for a `gofmt` fix to `internal/githubcli/comment_integration_test.go`. That file is already formatted for the toolchain pinned in `go.mod` (commit 21f851142, change 0436). Only the older `gofmt` on PATH flags it, and reformatting with that one would undo 21f851142. No change was made.

`skills/docket-finalize-change/references/gate-failure.md` was intentionally left unchanged. It covers merge refusals in general and names no individual refusal token.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed through the build gate. The final certification head is recorded in the PR's build-evidence block.
- Focused `repoguard` and `assets` tests passed after each skill edit, and `go run ./cmd/genassets -check` matched. The word-budget guard was mutation-tested: setting the ceiling one below the real count made it fail.
- A lean whole-branch review compared the prose against `finalize_merge.go`, `pr_publish.go`, and `subject_scope.go` and confirmed it is accurate. It raised two minor wording findings. Both are fixed in commit 2f77444d7: finalize now states the `invalid-state` result, and the scope now mentions records that carry the same id.
- Only automated guards check that the phrases are present. Whether the wording is accurate was checked by review.
