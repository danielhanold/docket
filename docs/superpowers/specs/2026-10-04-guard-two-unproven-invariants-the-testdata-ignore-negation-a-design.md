<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0504 — Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0504-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md)**
<!-- docket:backlink:end -->

# Guard two unproven invariants — design

## Problem

Two behaviours are correct today, but no committed test proves them, so a regression in either
would pass the suite silently. Each was left as an unprobed residual by an earlier change.

1. **Tracked fixtures hidden by an ignore rule (from #305, formerly #320).** The managed docket
   block in the root `.gitignore` ignores `.docket.local.yml` at any depth. The repository-local
   config fixtures under `testdata/repositories/` imitate that filename on purpose. They stay
   committable only because of the nested `testdata/repositories/.gitignore` negation
   (`!**/.docket.local.yml`), which lives outside the managed block. Two manual
   `git check-ignore` probes are the only proof. If the nested file is deleted, the three tracked
   fixtures (`testdata/repositories/v0.9.2/{fenced-machine-keys,four-layer-collision,invalid/model-typo}/repo/.docket.local.yml`)
   become tracked-but-ignored, any later sibling fixture is silently never added, and no test
   reddens. The general form of the failure is the anti-pattern in the
   `gitignore-guarantee-must-be-committed` learning: a file that is tracked only because of a
   one-time `git add -f` while an ignore rule still matches it.
2. **Receipt trailers read anywhere but the seed root (from #378, formerly #380).**
   `verifyMetadataOwnership` (`internal/app/metadata_ownership.go`) reads its `OpInitRoot` /
   `OpMigrateSeed` receipt trailers from the sole parentless root commit only: it scans from
   `own.Root` and keeps only the scan entry whose `Commit == own.Root`. No test places a receipt
   on a *descendant*, so a refactor that re-anchored the scan at the tip would let a descendant's
   receipt authorize an unrecognized root. Build 0378 wrote the closing fixtures in commit
   `9c5ced015` ("test(0378): pin root-anchored trailer read; …"). They were reverted (`7b924c0f7`)
   together with 0378's other non-blocker fixes when its fix-loop gate reddened. Nothing suggests
   the fixtures themselves caused the red.

## Decision

### Guard 1 — no tracked file is hidden by the repository's own ignore rules

A repo-wide invariant, keyed on what git reports rather than on a list of paths: **every tracked
file must be un-ignored by the committed `.gitignore` rules.** It is true today (empty result) and
covers the testdata negation as one case of the general rule.

- **Home.** A new test file in `internal/repoguard` (default build tag). That package is not under
  the change-0465 no-real-git guard and already starts subprocesses; the probe is one fast
  `git ls-files` call.
- **Probe.** From `repoguard.Root()`, run
  `git ls-files -z --cached --ignored --exclude-per-directory=.gitignore`. The test passes only
  when the output is empty. Use `--exclude-per-directory=.gitignore` and **never**
  `--exclude-standard`: only the repository's own `.gitignore` files then decide, so a
  developer's `.git/info/exclude` or user-global `core.excludesFile` can neither redden nor mask
  the guard. Parse the `-z` output on NUL boundaries.
- **Failure message.** List every offending path. Give the remedy: add a committed negation in a
  nested `.gitignore` beside the files, outside the managed docket block, or stop tracking the
  file. Cite the `gitignore-guarantee-must-be-committed` learning by name.
- **Fail-closed.** If the root cannot be resolved, git fails to start, or git exits non-zero, the
  test fails (`t.Fatal`) with the diagnostic. A failed probe is never read as an empty list
  (probe-error-is-not-clean-absence).
- **Committed non-vacuity control.** Factor the probe into a helper that takes a directory, so the
  main assert and the control share one code path. The control builds a throwaway repository in a
  temp dir taken from `internal/testsupport` (the change-0373 tempdir guard requires that in this
  package). It commits a file, then commits a `.gitignore` rule matching it, and asserts the helper
  reports exactly that path. If the flags ever stop loading the rules, the control reddens instead
  of letting the main assert pass vacuously. Isolate the temp repository's git from user config
  (`GIT_CONFIG_GLOBAL=/dev/null`, `GIT_CONFIG_NOSYSTEM=1`, and an explicit commit identity), so
  the control does not depend on the developer's machine.
- **Mutation evidence (build time, recorded in the results file).** In the feature worktree,
  delete `testdata/repositories/.gitignore` and run the new test with `-count=1`. It must redden
  and list the three v0.9.2 fixtures; then restore the file. Groom-time dry run (2026-10-04, on a
  scratch clone of `main` at `2587e6dc7`): baseline empty; with the negation deleted, the probe
  listed exactly those three paths.
- **Residual (undetectable, not unprobed).** A brand-new fixture that is ignored and never added
  is absent from every clone's committed state, so no test over the repository can see it. The
  guard instead catches every *tracked* instance and the deletion of any negation that protects
  one. Record this residual in the results file.

### Guard 2 — restore the root-anchored receipt fixtures

- Re-apply **only the test hunk** of `9c5ced015` to
  `internal/app/repoownership_integration_test.go` (`//go:build integration`). It adds
  `TestIntegrationRepoOwnershipDescendantInitReceiptCannotAuthorizeLegacyRoot` and
  `TestIntegrationRepoOwnershipDescendantMigrateReceiptCannotAuthorizeForeignRoot`, with their
  section comment. The commit's other half (F4, the `HistoryEntry.Tree` fast skip in
  `verifyLegacyEquivalence`) is a production refactor and stays out.
- Do not rewrite the fixtures' assertions. The init case pins that a receiptless empty root keeps
  its own `proofLegacyEmpty` proof and never adopts `proofInitReceipt` from a descendant. The
  migrate case pins that a receiptless nonempty root stays `RootForeign` even when a descendant
  carries a fully valid `OpMigrateSeed` receipt whose copy digest equals the root tree.
- **Groom-time evidence (2026-10-04, `main` at `2587e6dc7`).** The hunk applies cleanly and both
  tests pass. Under the mutation that anchors the scan at the tip
  (`ScanCommitTrailers(ctx, repo, tip, …)` with the match `s.Commit == tip`), both tests redden.
  The build re-proves both facts on its own base with `-count=1` and records them in the results
  file.
- If reconcile finds that the hunk no longer applies (the fixture helpers moved), re-author the
  same two fixtures against the current helpers with the same assertions.

## Out of scope

- Changing `verifyMetadataOwnership`, the ignore layout, the managed `.gitignore` block, or the
  fixtures under `testdata/repositories/`. Both behaviours are correct today; this change is test
  coverage only.
- The F4 `verifyLegacyEquivalence` refactor and the other 0378 follow-ups (the SHA-256 width fix,
  the `internal/process` flake).
- Probing untracked files or synthetic paths (the original #320 single-folder probe). The
  repo-wide invariant replaces it; the one case it would add is the residual above.

## Acceptance

- Both guards are present and green in the full suite (`build.test_command`).
- The results file records the mutation evidence for both guards: the deleted negation reddens
  Guard 1 and lists the three fixtures; the tip-anchored scan reddens both Guard 2 tests.
- The non-vacuity control is committed and green.
