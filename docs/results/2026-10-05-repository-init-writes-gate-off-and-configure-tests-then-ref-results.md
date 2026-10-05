<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0526 — repository configure-tests takes the test command as input](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0526-repository-init-writes-gate-off-and-configure-tests-then-ref.md)**
<!-- docket:backlink:end -->
# repository configure-tests takes the test command as input — Results

**Human action:** None required before merge. One optional walkthrough below shows `--command`
working on a repository whose only test is a root `test.sh`.

## Outcome

Before this change, a repository whose tests Docket could not detect, such as one whose only
test is a root `test.sh`, ended up with both test gates `off`. No command could turn them on
again: `docket repository configure-tests` took no input, re-ran the same detection, and
printed a false "the test policy is already configured; nothing to write".

Now:

- `docket repository configure-tests --command "<cmd>"` sets `build.gate` and `finalize.gate` to
  `local` with that command. It replaces any existing gate value or command and leaves the usual
  pending, unstaged `.docket.yml` edit for review. Running it again with the same command does
  nothing. An empty or whitespace-only value, `auto`, or a value with control characters or line
  breaks is refused, and nothing is written.
- Without `--command`, configure-tests says what detection actually found: no suite (with the
  resolved gates and the `--command` remedy), two candidate suites (each with its command), or
  already configured (naming the commands). The half-configured note and init's "no suite" and
  "two suites" notes now point at `--command` instead of a hand edit.
- When `docket repository migrate` finds two candidate suites, it now says to commit
  `finalize.test_command` and re-run migrate. It no longer points at configure-tests, which
  refuses a legacy repository.
- The glossary and the guide pages describe `--command`.

How this differs from the design:

- **Quoting.** A command that YAML would read as a number or date (`123`, `2026-10-05`) is now
  quoted when written. Before, such a value would have produced a `.docket.yml` that fails to
  load.
- **The no-suite message.** It reports the gates as they actually resolve rather than saying
  they are `off`, because a gate the file already sets explicitly is left alone.

## Human actions and testing

### Optional — turn the gates on for a root `test.sh` repository

This shows the original problem fixed from start to finish. The integration test
`TestIntegrationRepoSetupConfigureTestsCommandTurnsOnRootTestSh` already covers it.

Prerequisites: a `docket` binary built from this branch on `PATH`, and an empty GitHub-less
scratch directory with a bare remote.

1. `mkdir -p /tmp/ct526 && cd /tmp/ct526 && git init --bare remote.git && git clone remote.git repo && cd repo && printf '#!/bin/sh\nexit 0\n' > test.sh && chmod +x test.sh && git add test.sh && git commit -m init && git push origin HEAD`
2. `docket repository init`, then commit and push the paths it reports as pending.
   Expected: the note says no supported test suite was found and names
   `docket repository configure-tests --command "<cmd>"`. `.docket.yml` has both gates `off`.
3. `docket repository configure-tests --command "sh ./test.sh"`
   Expected: an applied result. `git diff .docket.yml` shows `gate: local` and
   `test_command: sh ./test.sh` under both `build:` and `finalize:`.
4. Run the same command again.
   Expected: a no-op naming the command.

Cleanup: `rm -rf /tmp/ct526`.

## Verification performed

- Each task followed TDD with focused package tests. The overwrite rule, the string-quoting
  check, the command-flag handling, and the no-suite message were each
  mutation-tested: breaking the guard made its test fail, and restoring it made the test pass.
- The full suite (`go run ./cmd/docket development test`) passed after the build. The PR body
  carries the evidence from the final run. The run printed
  `BUDGET WATCH` and `PARALLEL-SENSITIVE` lines for unrelated app-integration shards, which this
  change does not touch, and no `SERIAL CONFIRMED OVER BUDGET` line.
- Whole-branch review (deep tier): 2 findings (1 important, 1 minor), both fixed in-branch.
  The first fix-pass suite run failed because the important fix's new test used a bare
  `t.TempDir()`, which a repository guard rejects; both fixes were reverted, the run halted, and
  on the authorized resume both were re-applied with the test switched to
  `testsupport.TempDir(t)`. The splice now re-parses the edited `.docket.yml` and refuses, file
  untouched, when a folded/literal block `test_command` or an inline `{...}` `build:`/`finalize:`
  would not round-trip to exactly the requested settings. The init no-suite note now says only
  gates `.docket.yml` did not already set were written `off`. The full table is in the PR body.
- A second fix-pass run failed in `internal/process` TestObserveRunningThenTerminal ("timed out
  waiting for vanished"), a package this branch does not touch; the final gate run is recorded in
  the PR body's evidence block.
