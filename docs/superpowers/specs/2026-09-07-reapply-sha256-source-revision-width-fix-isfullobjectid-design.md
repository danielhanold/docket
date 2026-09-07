<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0379 — Re-apply the SHA-256 (64-hex) source-revision width fix to isFullObjectID](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0379-reapply-sha256-source-revision-width-fix-isfullobjectid.md)**
<!-- docket:backlink:end -->

# Accept SHA-256 source revisions in metadata ownership verification

## Purpose

Change #379 corrects a false foreign-metadata verdict for a valid migration receipt whose source revision is a full 64-character SHA-256 Git object ID. The ownership verifier must accept the same full lowercase-hex widths as its downstream Git reader while preserving every other ownership check.

The user requested a full spec and build-ready grooming on 2026-09-07. This design records the settled scope; implementation and build-time planning remain separate.

## Current behavior and evidence

Design baseline: integration revision `effc9a6dc549f1c7b1fa888707d3303681c80cad`.

In `internal/app/metadata_ownership.go`, `verifyMetadataOwnership` handles `reposetup.SeedMigrate` by first checking `isFullObjectID(rec.SourceRevision)` and matching the receipt's copy digest to the root tree. Only after those checks does it call `git.IsAncestor` to verify source reachability from the integration tip.

`isFullObjectID` currently requires exactly 40 bytes and restricts every byte to ASCII `0-9` or `a-f`. A valid 64-character revision therefore returns false and produces `RootForeign` before reachability can be evaluated. This is a false rejection, not evidence that an invalid ownership proof can pass.

The downstream `validateObjectID` in `internal/gitcli/types.go` already accepts exactly 40 or 64 lowercase-hex characters. The app-side width restriction is the mismatch.

Change #378 is done and introduced this shared verifier. Its historical commit `41245f279426574e21c48f5c9a5d7cd05233084d` contains a reverted correction and focused unit test. The patch is useful reference material, but the current source and this spec define the implementation contract.

## Required behavior

`isFullObjectID(s)` returns true if and only if:

1. The byte length of `s` is exactly 40 or exactly 64.
2. Every byte is an ASCII decimal digit or a lowercase hexadecimal letter.

| Input shape | Result |
| --- | --- |
| 40 lowercase-hex characters | Accept |
| 64 lowercase-hex characters | Accept |
| Empty, abbreviated, or any other length | Reject |
| Uppercase or mixed-case hexadecimal at either valid width | Reject |
| Non-hexadecimal characters at either valid width | Reject |
| Whitespace, control bytes, Unicode characters, or ref-expression syntax | Reject |

Validation does not trim whitespace, lowercase input, expand abbreviations, resolve refs, or infer the repository's hash algorithm. It remains a pure syntax check. Syntactic acceptance does not prove that an object exists, is a commit, belongs to the repository, or is reachable.

For a valid SHA-256 migration receipt, passing this check permits the existing digest and ancestry verification to continue. A malformed source revision still follows the existing foreign verdict path; digest mismatch, unreachable source, and Git probe failures retain their current behavior.

## Design and code boundaries

Change the length predicate in `isFullObjectID` to reject values whose length is neither 40 nor 64. Keep the byte-validation loop and boolean return contract unchanged.

Update the helper's comment to name both supported widths and the untrusted-receipt boundary. Describe syntactic compatibility with the Git reader without claiming that accepted strings cannot encounter a later Git error.

Put focused, table-driven unit coverage in `internal/app/metadata_ownership_test.go`, in package `app`, without an integration build tag. If that file exists by implementation time, extend it. The test should exercise `isFullObjectID` directly and require no repository fixture or external Git process.

The expected production change is confined to `internal/app/metadata_ownership.go`. No exported API, receipt format, configuration field, new package, or common validator abstraction is needed. Keep `verifyMetadataOwnership` control flow unchanged.

The historical patch may be reapplied selectively, or the small change may be recreated against current main. Do not blindly cherry-pick unrelated follow-up work or reproduce stale comments. Run `gofmt` on edited Go files before verification.

## Verification design

The unit matrix must cover:

- Valid 40- and 64-character strings containing both digits and lowercase letters.
- Empty input and a representative abbreviated ID.
- Lowercase-hex lengths 39, 41, 63, and 65, plus an intermediate length such as 50.
- Uppercase letters and non-hexadecimal bytes at both accepted widths.
- Invalid bytes in the first and final positions, including the final byte of a 64-character value, so coverage detects validation accidentally stopping after 40 bytes.
- Whitespace and a non-ASCII input constructed at a nominally accepted byte length, so rejection depends on character validation rather than only on length.
- A ref name or revision expression that must never be accepted as a full object ID.

Construct fixtures so their intended byte lengths are evident and correct. Tests assert returned behavior, not the spelling of the production predicate.

Prove the regression test before accepting the fix: the 64-character positive case fails against the original implementation and passes after the correction. Then mutation-test the completed guard: temporarily restore the 40-only length restriction and require that case to fail; temporarily bypass character validation and require malformed-input cases to fail. Use uncached Go test execution (`-count=1`), verify each mutation landed, and restore the exact corrected source after each probe.

Run the focused unit test and relevant existing metadata-ownership integration coverage. Existing integration fixtures remain the regression coverage for digest, ancestry, and refusal behavior; this change does not claim comprehensive SHA-256 repository support from a syntax test alone.

At the build gate, resolve and run the full `build.test_command` from current configuration. At finalize, independently resolve the configured finalize gate. Both currently resolve to `go run ./cmd/docket development test`; do not substitute a remembered command for current configuration. Follow `tests/README.md` for placement and budget findings. No new shell wrapper or budget increase is expected for this small unit test.

## Compatibility, risks, and alternatives

SHA-1 inputs preserve their existing results. The only newly accepted inputs are syntactically valid 64-character lowercase-hex strings, which remain subject to the same later ownership checks. There is no data migration, persistent-state rewrite, network behavior change, or new error vocabulary.

The main risks are accidentally accepting a range of lengths, weakening character validation, checking only the first 40 bytes, or mistaking syntax acceptance for ownership. The explicit matrix and mutation probes cover the validator risks; leaving the downstream ownership flow intact preserves its boundary.

A repository-format-aware validator would require extra plumbing without improving this local contract, since the existing Git reader accepts both full widths and Git verifies the referenced object. Exporting or consolidating validators across packages would broaden a one-function correction into an abstraction change. Neither is required here.

## Related work and non-goals

- Preserve `related: [378]` and `discovered_from: [378]`.
- There are no dependencies or stack parent; #378 has already reached done.
- #380 owns the separate descendant-receipt negative fixture.
- Broader object-ID abstraction, hash-algorithm plumbing, unrelated process flakes, and cleanup of historical plans or results are outside this change.
- No new architecture decision is required; the existing ownership design remains intact.

## Acceptance criteria

1. Exactly 40- and 64-byte lowercase-hex inputs pass `isFullObjectID`; all specified malformed classes fail.
2. The helper comment accurately describes both widths and the syntax-only guarantee.
3. Uncached regression and mutation evidence demonstrates detection of the original width defect and loss of character validation.
4. Relevant existing ownership coverage and the configured full build suite pass, edited Go files are formatted, and authoritative budget breaches are addressed.
5. The implementation diff remains within the validator, its explanatory comment, and focused tests; later ownership checks retain their existing semantics.

## Grooming outcome

Attach this spec to #379, keep `status: proposed` and `trivial: false`, remove the resolved Open questions section, and retain the existing relationships. With no unmet dependencies, the linked spec makes the change build-ready. There are no unresolved design questions.
