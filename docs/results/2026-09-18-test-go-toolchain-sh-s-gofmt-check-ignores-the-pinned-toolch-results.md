<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0436 — test_go_toolchain.sh's gofmt check ignores the pinned toolchain, flip-flopping CI red](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-19-0436-test-go-toolchain-sh-s-gofmt-check-ignores-the-pinned-toolch.md)**
<!-- docket:backlink:end -->
# test_go_toolchain.sh's gofmt check ignores the pinned toolchain, flip-flopping CI red — Results

## Outcome

Check 1 of `tests/test_go_toolchain.sh` now resolves the formatter from the toolchain declared in
`go.mod` (its single explicit `toolchain` directive, currently `go1.26.5`) instead of PATH's
`gofmt`. It reads the directive with `awk`, requires exactly one, resolves that toolchain's GOROOT
with a command-scoped `GOTOOLCHAIN="$toolchain_names" go env GOROOT` (exact name, no `+auto`),
captures stdout as the path with stderr separated to a scratch file, requires an executable
`bin/gofmt` there, and runs `"$gofmt_goroot/bin/gofmt" -l` over the existing go-list-derived package
directories. Every resolution failure — absent/ambiguous directive, nonzero/empty GOROOT,
non-executable gofmt, or a silent nonzero formatter exit — fails the one formatting assert with an
actionable diagnostic and never falls back to PATH's gofmt. The go-list capture/empty checks, the
four-result-marker contract (`markers_emitted -eq 4`), concurrency limits, GOFLAGS handling, and the
git-common-dir cache selection are byte-for-byte unchanged (ADR-0050, ADR-0108, changes 0373/0434).

This removes the local/CI divergence: an ambient newer Go (observed here: go1.27.1) reported the
tree clean while the declared go1.26.5 formatter flagged
`internal/githubcli/comment_integration_test.go` (trailing-comment alignment in
`TestIntegrationEnsureCommentIdempotent`). That one file was reformatted with the declared
formatter (formatting-only, two lines). A `tests/README.md` remedy documents the reformat command,
deriving the version from `go.mod` at run time rather than hard-coding a version.

No departures from the spec. Out-of-scope boundaries were honored: no `go.mod`/CI version edits, no
new configuration, downloader, suite lane, retry, timeout, or budget changes.

## Verification performed

- Behavioral regression coverage added in `internal/repoguard/gofmt_toolchain_test.go`: a fixture
  runs a live copy of the real wrapper against fake `go`/`gofmt` tools on a prepended PATH, pinning
  the mechanism (which binary ran, with which observed `GOTOOLCHAIN`) rather than only the marker
  outcome. It covers ambient-formatter-ignored, go.mod-derived toolchain name (fixture declares
  `go1.99.7`, not the repo's real version), stderr chatter not contaminating the resolved path,
  silent nonzero formatter failure, unusable-toolchain fail-closed (three sub-cases), and the
  absent/ambiguous directive cases.
- Mutation resistance is committed and proven three ways: committed shape tests
  (`TestGofmtMutationBareGofmtIsDetected`, `TestGofmtMutationDroppedGotoolchainIsDetected`,
  `TestGofmtMutationCountGuardIsDetected`) that neutralize the pinned invocation, the command-scoped
  `GOTOOLCHAIN`, and the exactly-one-toolchain count guard respectively and assert each mutant
  observably reddens; plus uncached (`-count=1`) mutations of the real wrapper (bare `gofmt`, dropped
  `GOTOOLCHAIN`) that each reddened a Task-1 assert and were restored to a clean tree.
- Real formatter discrepancy reproduced during reconcile: ambient `gofmt` reported the target file
  clean; the declared go1.26.5 `gofmt -l` flagged it; post-repair the declared `gofmt -l` over
  `go list ./...` is silent while a deliberately-unformatted scratch file is still detected.
- Full configured build suite (`go run ./cmd/docket development test`) driven through the native
  gate driver: green at head `e022f1dd07ba7a85574f1804bacf5ba53cb45d48` (52/52 files, 423 asserts).
  Build evidence recorded and verified against that head.

## Findings and limitations

### First-run toolchain download on machines with a newer ambient Go

On a machine whose ambient Go is newer than the declared `go1.26.5` (the exact divergence this
change targets), the first `GOTOOLCHAIN=go1.26.5 go env GOROOT` invocation downloads the declared
toolchain as a `golang.org/toolchain` module into the shared `docket-go-cache` GOMODCACHE.
Subsequent runs are warm and offline runs fail closed as designed — the same first-run-network /
warm-thereafter caveat the wrapper header already documents for the module cache, now noted there
for the pinned-toolchain resolution as well. Not a regression to the stated invariant, but worth a
human's awareness for fresh CI images.

## Follow-ups

None. Adjacent work seen but out of scope: the separate integration-shard registration issue tracked
on change 0368 (referenced in the original stub) is unrelated and untouched here.
