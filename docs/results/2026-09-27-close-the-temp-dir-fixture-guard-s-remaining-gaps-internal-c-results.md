<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0462 — Close the temp-dir fixture guard's remaining gaps (internal/cli gateTempDir, scan-root removal)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0462-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c.md)**
<!-- docket:backlink:end -->
# Close the temp-dir fixture guard's remaining gaps (internal/cli gateTempDir, scan-root removal) — Results

**Human action:** No human action is required. This change only touches tests and a repository guard, and the full suite passed. Reading the PR diff at merge time is enough.

## Outcome

The repository guard now makes sure that tests which start real processes create their temporary directories through the shared `testsupport.TempDir` fixture. That fixture waits for leftover writers before it deletes the directory. Change 0398 left two gaps in the guard, and this change closes both.

- **The private helper is gone.** `internal/cli/gate_test.go` used to have its own temp-dir helper built on `os.MkdirTemp`. It is deleted, and all of its call sites now use the shared fixture.
- **Unmarked `MkdirTemp` calls now fail the guard.** An `<x>.MkdirTemp(` call in a real-process test package is only allowed when it has a `// tempdir-exempt: <reason>` comment with a non-empty reason. The comment can sit on the same line, or alone on the line directly above. Eight existing sites have a site-specific reason:
  - `TestMain` binary builds.
  - Directories created once per process.
  - Failure evidence that has to survive the test.
  - The macOS `/tmp` alias test.
- **The guard scans the whole repository.** It used to walk a hand-written list of folders (`internal`, `cmd`). It now uses the same repo-wide file walk the other guards use. Package floors (`internal/process`, `cmd/docket`) still catch a scan that loses coverage.

This differs from the design in one way. The spec listed `internal/app/gate_drive_test.go` runroot as a site to mark. The builder converted it to the fixture instead, because it is an ordinary per-test directory. So 8 sites are marked, not 9.

## Verification performed

- **Full suite:** passed at head `6a8d356dc` (54/54 files). The review fix came after that run, so the suite was run again for the final certification. The PR body's build-evidence block records the certifying run.
- **Guard mutation checks:** each check below was run through the gate driver and then reverted, and each made the guard fail as intended:
  - Restoring the old helper.
  - Deleting a marker.
  - Giving a marker an empty reason.
  - Adding an unmarked helper in another package.
  - Putting the marker two lines above the call.
  - Excluding `cmd` from the walk, and separately excluding `internal`.
- **Masking check:** a `MkdirTemp` spelling inside a comment or a string does not trigger the guard.
- **Review:** one minor finding came back. A trailing marker on one line also exempted a `MkdirTemp` call on the next line. It was fixed in `96c2e96a9`, with a new test case that failed before the fix.

## Known issues and follow-ups

### Shared walker included `.docket/` when run from the main checkout — resolved in this change

Running a guard from the primary checkout instead of a feature worktree made the whole-repo walk include the `.docket/` metadata worktree and the local harness installs (`.claude/`, `.codex/`, `.cursor/`, `.agents/`, `.superpowers/`). At the human's request after review, this change now resolves that inside its own scope. `repoguard.MaintainedFiles` prunes every hidden directory at any depth except `.github/`, which is the only tracked hidden directory and stays in the scan. That rule also covers the old `.git`/`.worktrees` exclusions. `TestMaintainedFilesIncludesAndExcludes` pins both directions with `.docket/…`, `.claude/…`, a nested `internal/app/.cache/…`, and the existing `.github/workflows/ci.yml` inclusion. It was mutation-tested: without the `.github` carve-out the test turns red, and before the rule was added the new exclusion cases were red.

### Suite budget watch lines

The green run printed `BUDGET WATCH` lines (streak 1/5) for long parallel integration suites such as `test_go_race` and `test_go_toolchain`. These are screening signals that depend on the machine. None of them is a serially confirmed budget breach, and this change did not touch those suites.
