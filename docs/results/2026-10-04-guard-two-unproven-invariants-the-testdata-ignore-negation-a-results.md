<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0504 — Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0504-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md)**
<!-- docket:backlink:end -->
# Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read — Results

**Human action:** None needed beyond the normal PR review. This change only adds tests, and every guard was shown to turn red when the behavior it protects was removed.

## Outcome

Two behaviors were correct, but no test checked them. A regression in either one would have passed the suite. Each now has a regression guard:

- **No tracked file is hidden by the repository's own `.gitignore` rules.** The new `internal/repoguard/tracked_ignored_test.go` runs `git ls-files --cached --ignored --exclude-per-directory=.gitignore` over the repository and fails if the list is not empty. It names every offending path and says how to fix it. Only committed `.gitignore` files count, so a developer's `.git/info/exclude` or global excludes file can't make the test fail or hide a failure. If git itself fails, the test fails too; that is never treated as "nothing found". A control test builds a throwaway repository and checks that the probe catches an ignored tracked file, accepts a nested negation, and does not read local excludes. That control is what would catch the probe silently finding nothing. The nested `testdata/repositories/.gitignore` negation is one case this rule covers.
- **A receipt on a later commit never authorizes the metadata seed root.** I re-applied the two integration fixtures from build 0378 (commit `9c5ced015`, test hunk only, byte-identical) to `internal/app/repoownership_integration_test.go`. In each, a valid `OpInitRoot` or `OpMigrateSeed` receipt sits on a later commit, and the test asserts that the root does not take it as proof of ownership. The production refactor from that same commit was left out.

No production code changed, and the build matches the spec.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the build gate. Its `PARALLEL-SENSITIVE` lines were only screening notes for files this change did not touch.
- Mutation, Guard 1: with `testdata/repositories/.gitignore` deleted, `TestNoTrackedFileIsIgnoredByCommittedRules` (`-count=1`) failed and listed exactly the three `testdata/repositories/v0.9.2/{fenced-machine-keys,four-layer-collision,invalid/model-typo}/repo/.docket.local.yml` fixtures. After the file was restored, the test passed again.
- Mutation, Guard 1 control: switching the probe to `--exclude-standard` made `TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile` fail at its local-excludes step. After the revert, the test passed again.
- Mutation, Guard 2: with the trailer scan in `verifyMetadataOwnership` changed to read from the tip (`ScanCommitTrailers(ctx, repo, tip, …)`, match `s.Commit == tip`), both `TestIntegrationRepoOwnershipDescendant*` tests failed ("Proof = 1, want proofLegacyEmpty"; "Shape = 1, want RootForeign"). They passed again once the original file was restored. The ownership shard script stayed within its time budget.
- Whole-branch review (standard tier): no findings.

## Known issues and follow-ups

### A brand-new ignored fixture that is never committed can't be detected

This happens when someone adds a new fixture whose name an ignore rule matches. Git silently leaves it out, so it never reaches any clone, and no test that reads the repository can see it. The guard catches every file that is actually committed while an ignore rule matches it. It also catches the deletion of any negation that protects such a file. This is a limit of what any test can see, not a missing check. No action is suggested.
