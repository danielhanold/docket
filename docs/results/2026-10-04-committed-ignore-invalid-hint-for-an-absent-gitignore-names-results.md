<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0500 — committed-ignore-invalid remedies print the paste-ready managed block](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0500-committed-ignore-invalid-hint-for-an-absent-gitignore-names.md)**
<!-- docket:backlink:end -->
# committed-ignore-invalid remedies print the paste-ready managed block — Results

**Human action:** None required to merge. One optional walkthrough below lets you see the new remedy in a real `repository check` output.

## Outcome

When `docket repository check` finds that the committed `.gitignore` on the integration branch lacks docket's managed ignore block, it reports `committed-ignore-invalid`. Before this change, the remedy for a missing `.gitignore` told you to re-run `docket repository migrate`, which has been a no-op on migrated repositories since change 0496. None of the remedies showed what the managed block looks like, and its marker lines have an exact spelling you cannot guess.

Now every variant of the finding keeps its own instruction (add the file, append, replace the legacy block, fix the markers first, restore missing entries, rewrite) and ends with the exact managed block on its own lines. The block comes from the same source that `repository init` writes and `repository check` validates. `repository check` prints it flush left, so you can paste it into `.gitignore` as is. No remedy names `repository migrate` any more.

The design was followed as written. New guards cover every defect variant, including any added later, and the column-0 rendering.

## Human actions and testing

### Optional — see the new remedy in a scratch repository

Shows the paste-ready block in real output. Automated tests already cover this behavior.

Prerequisites: a throwaway clone of a migrated docket repository, and the `docket` binary built from this branch (`go build -o /tmp/docket-0500 ./cmd/docket` from the feature worktree).

1. In the scratch clone, delete the managed block from `.gitignore` on the integration branch, then commit and push the change to a scratch remote.
2. Run `/tmp/docket-0500 repository check`.
   Expected: a `committed-ignore-invalid` finding whose remedy says "Append exactly these lines to the .gitignore…", followed by the `# docket:start (managed by docket — do not hand-edit)` … `# docket:end` lines, each starting at column 0.
3. Paste those lines back into `.gitignore`, commit, push, and re-run the check.
   Expected: the finding is gone.

Cleanup: delete the scratch clone and remote.

## Verification performed

- Each build task ran its focused tests (`go test ./internal/reposetup`, and the `repository check` tests in `./internal/app`) green.
- Mutation probes made the new guards fail: the old missing-file text put back, one case without the helper, block lines indented, a space join instead of a newline, and the check renderer indenting continuation lines. Every probe turned a test red, and each file was restored byte for byte afterwards.
- The full suite runs at the build gate through the configured `build.test_command`. The PR's build-evidence block records the result.
- Whole-branch review (standard tier) returned one minor finding: an existing integration test (`TestIntegrationRepoCheckMissingIgnoreEntryNamed`) that checks the remedy names the missing entry would always pass, because the appended canonical block already contains that entry. It was fixed in-branch: the test now checks only the instruction text before the block, and a mutation probe confirmed it goes red when the entry list is dropped.
