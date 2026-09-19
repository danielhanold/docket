<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0436 — test_go_toolchain.sh's gofmt check ignores the pinned toolchain, flip-flopping CI red](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0436-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch.md)**
<!-- docket:backlink:end -->

# Change 0436: use the declared Go toolchain for the formatting gate

## Intent

Make the existing formatting check use the formatter shipped with the toolchain declared in this repository's go.mod, regardless of the Go or gofmt selected by the caller's PATH. Keep the change within the existing Check 1 of tests/test_go_toolchain.sh and its regression coverage.

## Evidence and prior context

Investigated main at ab9216d2df9a2db8488f7a0445357315af137467 on 2026-09-18.

- tests/test_go_toolchain.sh discovers package directories through go list, then invokes bare gofmt. It already supplies persistent caches, separates go list stderr from its directory output, and reports four checks with a result-marker count.
- go.mod declares go 1.26.0 and toolchain go1.26.5. On the current host, go version reports go1.27.1 and go env GOROOT resolves the Homebrew 1.27.1 installation. GOTOOLCHAIN=go1.26.5 go env GOROOT resolves the downloaded 1.26.5 distribution.
- Read-only formatter probes over the same go-list-derived directory set reproduce the discrepancy: ambient gofmt reports no files; 1.26.5's gofmt reports only internal/githubcli/comment_integration_test.go. Its diff changes trailing-comment alignment in TestIntegrationEnsureCommentIdempotent. No files were reformatted during grooming.
- .github/workflows/release-candidate.yml selects the 1.26 family. This is not an exact 1.26.5 patch pin. The shared check, rather than setup-go's patch selection, will establish the formatting version on both CI and developer machines.
- Change 0304 introduced the four-check wrapper and the declared development toolchain. Its later stderr correction and the captured-stderr-becomes-arguments learning apply directly to the new GOROOT capture.
- Change 0317 established the candidate workflow. Change 0370 retained Go wrappers and moved substantive regression coverage into Go tests. Changes 0373 and 0434 concern concurrency and test isolation; preserve their behavior and ADR-0108.
- ADR-0050 favors checks derived from the actual consumer. Preserve module-derived package discovery rather than hand-listing directories. No new ADR is needed for this local correction.
- Go's toolchain selection documentation confirms that auto may retain a newer toolchain and that an explicit GOTOOLCHAIN value selects a specific version: https://go.dev/doc/toolchain.

## Design

Within the existing formatting check, read the explicit toolchain directive from the repository's go.mod. Require one usable explicit toolchain name; if it is absent, ambiguous, or unusable, fail the formatting check with an actionable diagnostic. Do not invent a fallback version or a configurable formatter policy. Let Go validate the selected toolchain name.

After the wrapper's existing cache setup, resolve its GOROOT using a command-scoped GOTOOLCHAIN value equal to the declared name, without +auto. Capture stdout as the path and stderr separately using the existing scratch directory. Check the command's exit status and nonempty result, then require an executable bin/gofmt at that path. Preserve error diagnostics and never fall back to PATH's gofmt.

Run that quoted formatter path with -l over the existing go-list-derived package directories. A successful check requires both a zero formatter exit status and empty output. A nonzero exit with empty output must still fail. Keep the existing go list failure and empty-population checks, the four-result marker contract, and the read-only nature of the check.

The exact toolchain selection applies only to formatter resolution. Existing go list, vet, test, cross-build ownership, concurrency limits, caller GOFLAGS, and cache selection keep their current behavior. Use Go's existing toolchain download/cache behavior; no downloader or bootstrap layer is introduced. If the requested toolchain cannot be obtained, the check fails rather than silently certifying a different formatter.

During implementation, apply the selected formatter only to files reported by the corrected check. Currently this is internal/githubcli/comment_integration_test.go. Re-discover the set against the implementation checkout and keep edits formatting-only. Add a short explanation and a usable formatting-remedy command to tests/README.md, deriving the version from go.mod rather than copying go1.26.5 into maintained instructions.

## Alternatives

- Reformat the one affected file alone: insufficient because the next run of a newer formatter recreates the mismatch.
- Use plain go env GOROOT: insufficient because the observed host retains Go 1.27.1 under auto.
- Pin the entire suite or change CI/toolchain policy: broader than the demonstrated formatting defect. The existing shared wrapper can make formatting deterministic without those changes.

## Verification

Add focused behavioral Go regression coverage in internal/repoguard, using a temporary fixture containing the current wrapper and fake Go tools. Exercise the actual wrapper, stubbing its expensive vet/test work so the regression does not recursively run the suite or download toolchains. Reuse the existing test fixture conventions.

Prove that an ambient newer formatter is ignored, the requested version comes from fixture go.mod rather than a hard-coded version, and stderr chatter during successful GOROOT resolution does not contaminate the executable path. Exercise selected-formatter clean and dirty results, silent nonzero formatter failure, and failed/unusable toolchain resolution; assert the formatting result and tool invocations, not only an overall nonzero exit.

Mutation-test the regression: replacing the selected formatter with bare gofmt and removing the explicit GOTOOLCHAIN selection must each cause a targeted failure. Perform mutations in fixtures or safely restored scratch copies, with uncached test execution.

Verify the actual formatter result under the declared toolchain and a newer ambient Go. The corrected check must pass after the formatting-only repair, and deliberately unformatted scratch input must be detected. Run the whole configured build suite from source through the existing Go runner and read its budget report. Grooming itself does not run a build gate or modify code.

## Scope and relations

No new configuration, shared toolchain service, helper framework, suite lane, retries, timeout or budget changes, CI version changes, or go.mod version edits. The integration-shard registration issue mentioned in the original stub is unrelated.

Dependencies: none. Related changes: 0304, 0317, 0370, 0373. Discovered from: 0434. Relevant ADRs: 0050 and 0108. Deliverable: one bounded fix with a linked spec; implementation and planning remain separate workflows.
