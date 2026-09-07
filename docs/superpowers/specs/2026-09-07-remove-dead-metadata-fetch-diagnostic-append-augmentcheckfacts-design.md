<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0383 — Remove or plumb the dead metadata-fetch diagnostic append in augmentCheckFacts](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0383-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts.md)**
<!-- docket:backlink:end -->

# Remove the dead metadata-fetch diagnostic append

## Decision

Remove the unused diagnostic append from `augmentCheckFacts` in `internal/app/repository_check.go`. This is a behavior-preserving cleanup: retain the existing fetch-failure facts and all public check output. The user requested a written spec rather than a trivial verdict.

## Problem and evidence

When the metadata fetch fails, `augmentCheckFacts` sets `f.MetadataRoot` to `reposetup.RootUnknown`, then appends a `metadata-fetch` entry to `sc.diagnostics`. The function receives `setupContext` by value, returns no context or diagnostics, and never reads that appended entry. Its caller, `RunRepositoryCheck`, does not consume `sc.diagnostics` after augmentation either.

The separate `prepareNotices` consumer reads diagnostics produced for repository preparation. It does not consume the context copy inside `augmentCheckFacts`. Changing the augmentation parameter to a pointer alone would therefore not deliver this error to check users.

Change 0378 required failed metadata fetches to leave ownership unknown and forbid stale-object ownership proof. That requirement is implemented by the facts assignment, independently of the dead append. Change 0377 discovered this cleanup during review; both related changes are done. Change 0403 subsequently added configuration-error findings on a separate error path and does not make this append live.

## Scope and implementation

Delete only the `sc.diagnostics = append(...)` statement for the `metadata-fetch` probe in `augmentCheckFacts`.

Preserve:

- The `FetchBranch` call and its existing control flow.
- The explicit `f.MetadataRoot = reposetup.RootUnknown` assignment when the fetch fails.
- The prohibition on proving ownership from an older local object or the earlier `ls-remote` tip after a failed fetch.
- The successful-fetch path, including its fetched remote revision and shared ownership verifier.
- All subsequent local-branch, worktree, synchronization, corpus, and health checks.
- Existing function signatures, shared diagnostic types, and repository-preparation diagnostic handling.

No caller plumbing or API/schema changes are needed. No new log, finding, notice, result value, or exit-code behavior is introduced. Existing check classification and human/JSON output remain unchanged.

## Alternatives considered

Returning a diagnostic and adding it to check findings would make the fetch cause visible, but would introduce public reporting behavior and require decisions about finding vocabulary and rendering. That is a separate enhancement. Removing the unused append resolves this change's single-site cleanup without creating that interface.

Passing `setupContext` by pointer is insufficient by itself because the check caller still has no diagnostic consumer.

## Verification and acceptance

At implementation-time reconciliation, re-check the producer and all consumers of `setupContext.diagnostics`. If this append has become live, revise the design before deleting it.

The implementation is accepted when the dead append is absent and the failure and success branches otherwise retain their behavior. Review the narrow diff directly; do not add a source-text guard merely to assert that one statement was deleted.

Retain the existing `TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown` coverage in `internal/app/repoownership_integration_test.go` and the ownership/classifier tests. The existing failure fixture checks the unknown fact and rejection of foreign ownership; it starts with a zero-valued root and should not be represented as a mutation-tested proof of the explicit assignment. This cleanup does not change that assignment or require a new test mirroring the deletion.

Run the full build suite through the configured `build.test_command`, resolved from config at build time, and handle budget findings per `tests/README.md`. Tests are an implementation gate; grooming itself changes metadata only.

## Boundaries and readiness

No broader facts-pipeline refactor, ownership-verifier redesign, diagnostic-pipeline cleanup, new ADR, dependency, or stack parent is required. Keep `related: [377, 378]` and `discovered_from: [377]`; `depends_on` and `adrs` remain empty.

All design questions are resolved. Attach this spec to change 0383 and leave its status proposed, making it build-ready. Implementation belongs to a subsequent implement workflow.
