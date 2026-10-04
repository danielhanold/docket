<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0506 — Drop the retired-harness globs from the managed .gitignore block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0506-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi.md)**
<!-- docket:backlink:end -->
# Drop the retired-harness globs from the managed .gitignore block — Results

**Human action:** Yes, but only in other repositories that use docket. In each one, re-run `docket repository init` and commit the rewritten `.gitignore`, otherwise `docket repository check` reports the repository as not healthy. This repository's own `.gitignore` is already updated in this branch.

## Outcome

`docket repository init` and `docket repository migrate` write a block of docket-owned ignore rules into each repository's `.gitignore`. That block used to ignore agent files for three harnesses docket no longer supports (`.agents/`, `.kiro/`, `.windsurf/`). Those three lines are gone. The block now lists only the supported harnesses (`claude`, `codex`, `cursor`, `opencode`), and their lines stay so that leftover agent files from older docket versions remain hidden.

This repository's own `.gitignore` carries the new block. A new test fails if the block ever again names a harness outside the supported list. Two more tests show that re-running `docket repository init` cleanly replaces the old block (anything you wrote outside the block is kept) and that `docket repository check` flags the old block and prints the new one to paste. The out-of-date code comments that described a deleted shell script are rewritten.

## Human actions and testing

### Important — update the ignore block in other docket repositories

Every other repository using docket still has the old 13-line block committed. Until it is replaced, `docket repository check` reports that repository as not healthy and `docket repository configure-tests` refuses to run. Day-to-day commands (status, grooming, implementing, finalizing) do not read the block and keep working.

Prerequisites: install the docket binary built from `main` after this change merges. Each repository needs a clean checkout on its default branch.

1. In the repository, run `docket repository check`.
   Expected: the managed `.gitignore` block is reported as non-canonical, and the output includes the new block.
2. Run `docket repository init`.
   Expected: the command finishes. `git diff .gitignore` shows exactly three removed lines (`.agents/agents/docket-*.md`, `.kiro/agents/docket-*.md`, `.windsurf/agents/docket-*.md`), and nothing outside the block changed.
3. Commit and push `.gitignore`, then run `docket repository check` again.
   Expected: the repository is reported as healthy.

## Verification performed

- `go test ./internal/reposetup/...` passes. The new roster test was checked against its own failure modes: putting back a `.kiro` line, or removing the `.cursor` line, makes it fail.
- The full suite (`go run ./cmd/docket development test`) passed through the build gate. Its budget report had only screening lines for unrelated suites, and nothing was confirmed over budget when re-run alone.
- Whole-branch review (standard tier) found one minor issue, a redundant phrase in a code comment. It was fixed in the branch, and the full table is in the PR body.
