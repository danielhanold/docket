<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0530 — Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0530-keep-plan-results-and-build-evidence-on-the-metadata-branch.md)**
<!-- docket:backlink:end -->
# Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR — Results

**Human action:** Pending — the build is complete and the whole-branch review has not run yet. Install ordering (below) will need a human regardless.

## Outcome

Build artifacts move off the PR. After this change a change's plan and results files are written to the `docket` metadata branch at their existing paths by `change.attach-plan` / `change.attach-results` (which now carry the Markdown with `--markdown`), and build evidence lives in a `## Build evidence` section of the change record instead of the PR description. The feature branch's first commit is a copy of the spec (new `workspace.commit-spec` operation), so a PR carries the spec copy plus code and nothing else. Links between artifacts on the same branch are relative, and the post-merge backlink commit to `main`, the `Docket-Plan-Path` trailer, `artifact.backlink`, and the `finalize.skip_results_only_delta` key are retired.

Departures from the spec:

- `pr.publish` writes no evidence anywhere. The spec listed it as a writer; `change.mark-implemented` already records the same verified evidence in a metadata transaction, and a second write at publish would move the record revision between the two steps. `pr.publish` still verifies `--evidence` against the head.
- `finalize.publish` with skipped (`build.gate: off`) evidence now records it in the change record. Previously it refused with `body-assembly-failed`.
- `workspace.commit-spec` builds the commit in a private index and fast-forwards the branch by compare-and-swap rather than staging in the worktree, so a failed commit leaves the workspace untouched.
- The PR description gains a generated block of absolute links to the plan and results on the metadata branch, right after the backlink.

## Known issues and follow-ups

### This run used the previous flow

This change was built with the previously installed binary, so its own plan and results ride this feature branch and its evidence sits in the PR description, as the spec's cutover rule requires. Changes in progress or implemented when this merges must finish on the old binary before the new one is installed.
