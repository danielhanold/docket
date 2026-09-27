<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0455 — Document finalize's record-invalid reason in the docket-finalize-change skill](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0455-document-finalize-s-record-invalid-reason-in-the-docket-fina.md)**
<!-- docket:backlink:end -->

# Document the `record-invalid` refusal in the finalize and implement-next skills — design

## Problem

Change 0449 (ADR-0127) added a named pre-effect validation to two operations that touch GitHub. Each refuses with the closed reason `record-invalid` before making any GitHub call:

| Operation | Driving skill | Reason constant | Result / disposition |
|---|---|---|---|
| `finalize.merge` | `docket-finalize-change`, step 8 | `ReasonMergeRecordInvalid` (`internal/app/finalize_merge.go`) | result `invalid-state`, disposition `blocked` |
| `pr.publish` | `docket-implement-next`, "Publish the PR" | `ReasonPRRecordInvalid` (`internal/app/pr_publish.go`) | result `invalid-state` |

Neither skill mentions the reason. An agent or operator who hits it gets no documented remedy and may guess wrong.

## Facts the prose must carry (traced from code, not the stub)

1. **Validation scope is structural, not only the change itself.** `namedPreEffectErrors` (`internal/app/subject_scope.go`) checks the named change and the records it structurally requires: its `depends_on` targets and stack ancestors (ADR-0127). Associative links (`related`, `discovered_from`, `adrs`) are **not** followed, so an unrelated or merely related broken record can never produce `record-invalid`.
2. **The refusal names the culprits.** `namedPreEffectMessage` renders "change NNNN or a record it structurally requires carries a validation error (<code> at <path>; …); repair it before any GitHub effect". The result's `findings` list each bad record's code and path. The remedy is to repair exactly the records `findings` name, then re-run. Never guess which record is at fault, and never edit an unnamed record.
3. **No GitHub effect has happened.** On a merge refusal no merge call was made. On a publish refusal no PR was created or edited. A re-run after the repair is safe.
4. **Merged-outside-docket precedence.** In `finalize.merge` the named validation runs before the already-merged recovery. So a PR merged outside docket, with a defective record in scope, reports `record-invalid` where it used to report `already-merged`. This is intentional (see the code comment above the check in `finalize_merge.go`): closeout would refuse the same defect, so reporting a verified no-op that could only strand the change is never useful. The prose should call this expected behavior, not a regression.

## Design

### `skills/docket-finalize-change/SKILL.md` — step 8 ("Merge exactly once")

Add `record-invalid` to the paragraph that lists the merge's refusal tokens. Suggested substance (wording is the builder's call; keep the paragraph's existing density):

- `finalize.merge` refuses `blocked` with reason `record-invalid` before any merge call when the change, a `depends_on` target, or a stack ancestor carries a validation error. Related, discovered-from, and ADR links are never checked. The run is `halted`.
- Remedy: repair the records named in the result's `findings`, then re-run finalize. A named id is fine; no override exists or is needed.
- One sentence: an already-merged-outside-docket PR whose scope has a defective record now reports `record-invalid` rather than `already-merged`. This is expected, because closeout would refuse the same defect. Fix the record, and the re-run takes the merged-recovery path.

Decide during build whether `references/gate-failure.md` needs a matching line. It is `docket-finalize-change`'s failure reference, and it only needs one if it enumerates the merge refusals that write a `## Finalize blocked` marker. Do not add one just for symmetry.

### `skills/docket-implement-next/SKILL.md` — "Publish the PR"

Add one clause to the existing `pr.publish` paragraph: it refuses `record-invalid` (`invalid-state`) before any GitHub call under the same structural scope. Remedy: repair the records named in `findings`, then re-publish. The run follows its existing halt posture for a typed refusal. Do not invent a new disposition or retry rule.

### `internal/githubcli/comment_integration_test.go`

Run `gofmt -w` on the file (`gofmt -l` flags it on `main`). The drift is unrelated and predates 0449; it is bundled only so a trivial fix has a tracked home. Formatting only, no semantic edits.

## Non-goals

- No change to either operation's behavior, reason token, or the `record-invalid`-before-`already-merged` precedence.
- No real-gate end-to-end finalize test with a broken record (0449 accepted this as a known limit).
- No new repoguard pinning skill reason tokens. None exists for the sibling tokens (`merge-method-unavailable` etc.), and adding one is out of proportion here (YAGNI).

## Verification

- `grep` confirms `record-invalid` now appears in both skill files, with the structural scope and the `findings` remedy.
- `gofmt -l internal/githubcli/` prints nothing.
- The full suite (`build.test_command`) stays green. The prose change is caught by any existing skill-prose/anchor guards (for example `TestCommentAnchorStyle` and the repoguard contract tests). Cross-references anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
