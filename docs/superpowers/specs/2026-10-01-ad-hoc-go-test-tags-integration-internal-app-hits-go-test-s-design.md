<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0479 — Refuse an unfiltered integration-tagged run of internal/app before go test's 10-minute timeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0479-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s.md)**
<!-- docket:backlink:end -->

# Refuse an unfiltered integration-tagged run of internal/app before go test's 10-minute timeout — design

## Goal

`go test -tags integration ./internal/app/` with no `-run` filter runs the whole integration corpus of `internal/app` in one process. That takes longer than go test's default 10-minute per-package timeout, so the run dies at 10 minutes with `panic: test timed out after 10m0s` and a goroutine dump. Change 0472's build lost a verification step this way. This change makes the integration-tagged `internal/app` test binary refuse that shape of run right after compile, with a message naming the supported forms, and documents those forms in `tests/README.md`.

## Trace at grooming (2026-10-01)

- **Suite runner: not affected.** The integration corpus of `internal/app` runs only through the 39 `tests/test_go_integration_app_*.sh` shard runners. Each sources `tests/lib/go-integration-shard.sh`, whose `run_integration_shard` always passes `-run "^${SHARD_PREFIX}"`. Their ceilings in `tests/runtime-budgets.tsv` are 10–55s each and sum to 1125s.
- **Gate drive: not affected.** It runs `build.test_command` (`go run ./cmd/docket development test`), which is the same suite runner.
- **Other callers.** `tests/test_go_integration_contract.sh` and `tests/test_release_partition_fidelity.sh` call `go test -tags integration -list`. `tests/test_go_finalize_e2e.sh` uses `-tags e2e -run TestE2E`. CI (`.github/workflows/release-candidate.yml`) runs no integration-tagged tests. No maintained source runs the package unfiltered; the four in-source comments that show a hand-run command all carry `-run`.
- **Where the unfiltered command comes from: plans.** Five plans under `docs/superpowers/plans/` prescribe an unfiltered integration-tagged run that includes `./internal/app/`, three of them dated 2026-09-28 to 2026-09-30 (0468, 0477, 0472). 0472's results file records the 10-minute panic and the workaround (`-run` plus `-timeout 25m`).
- **Flag values a TestMain sees** (verified with go1.26 at grooming): with no `-timeout`, `go test` passes `-test.timeout=10m0s`, and an explicit `-timeout 10m` is indistinguishable from it. `-timeout 0` arrives as `0s`. `-list X` sets `test.list`. `-skip .` with no `-run` leaves `test.run` empty. A compiled test binary run directly with no flags has `test.timeout` 0.
- **An older incident is resolved.** On 2026-09-18 shards hit 10 minutes under the suite's parallel load. Change 0373 (ADR-0108) bounded Go test load at the runner, and 15 more shards have been split off since. No shard timeout has been recorded after that.

## Decisions settled at grooming

1. **Refuse at the point of failure; no always-in-context rule.** AGENTS.md stays unchanged and no learnings finding is recorded. The refusal message is the documentation, shown to whoever makes the mistake at the moment they make it.
2. **Refuse only at the default timeout.** The guard refuses a run with an empty `test.run`, an empty `test.list`, and `test.timeout` exactly 10m. Any other timeout, including `0`, is the caller's deliberate choice and is allowed. Only `-run` counts as a filter; `-skip` does not.
3. **`internal/app` only.** The other packages whose `TestMain` calls `testsupport.InstallNoGitGuard` (`internal/gatedrive`, `internal/repository/transaction`, `internal/workspace`) have integration corpora that fit the default timeout, so refusing there would block valid runs.
4. **No suite timeout changes.** This keeps to ADR-0108, which rejected raising the per-package timeout for the suite. The shard runners get no `-timeout` backstop; the budget rows and the runner's screening findings are the growth detectors.

## Design

### The guard (internal/testsupport)

Follow the build-split pattern of `InstallNoGitGuard` (`nogit_install.go` with `//go:build !integration && !e2e`, and its no-op twin `nogit_install_off.go`): exactly one implementation of the entry point compiles for any tag set.

- **Pure decision, untagged file.** A function of the three flag values (run, list, timeout) that returns whether to refuse. It lives in an untagged file so the default-build unit test reaches it.
- **Real entry point, `//go:build integration`.** Takes the package name and shard glob (the two values `internal/app` already passes to `InstallNoGitGuard`: `nogitPkg`, `nogitShardGlob`). Calls `flag.Parse()` if flags are not parsed yet, reads `test.run`, `test.list`, and `test.timeout` through `flag.Lookup`, applies the decision, and returns a non-nil error carrying the remedy text on refusal, nil otherwise. A missing flag (`flag.Lookup` returns nil) is a setup error, never a silent allow.
- **No-op twin, `//go:build !integration`.** Always returns nil, so the default and e2e builds never refuse.

Names are the plan's choice.

### Wiring (internal/app)

`TestMain` in `internal/app/gate_test.go` calls the entry point after the supervisor and guardian re-exec routing (re-exec'd children never parse test flags) and before `m.Run`. On a non-nil error it prints the error to stderr and exits 1, the same way it handles an `InstallNoGitGuard` setup error. Only `internal/app` gets the call.

### Remedy text

The message names the package, says the whole integration-tagged corpus outlasts go test's default 10-minute timeout, and lists the three supported forms:

```
internal/app: the integration-tagged corpus outlasts go test's default 10m timeout when run whole.
Run one shard:   bash tests/test_go_integration_app_<name>.sh   (tests/test_go_integration_app_*.sh)
or filter:       go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/
or run it whole: add -timeout 30m
```

Exact wording is the plan's choice. Required: the package name, the shard glob (from the passed-in value, never a second literal), the `-run` form, and the `-timeout` value. **30m** is about 1.6× the sum of the `internal/app` shard ceilings (1125s, about 19m, at grooming). A whole-package run compiles once and runs the same tests, so the ceiling sum is an upper bound on its wall. The build recomputes the sum from `tests/runtime-budgets.tsv` and records the inputs in a code comment next to the value.

### Documentation (tests/README.md)

Add `### Running integration-tagged Go tests by hand` under `## Running it`, next to the existing gofmt subsection. It says:

- the real-git, subprocess, and process-lifecycle Go tests sit behind the `integration` build tag (change 0333);
- the suite runs them only through the `tests/test_go_integration_*.sh` shard runners, each filtered to one test-name prefix;
- to run some by hand, run a shard runner or use `go test -tags integration -count=1 -run '^<Prefix>' <pkg>`;
- `internal/app`'s test binary refuses an unfiltered run at the default timeout, and `-timeout 30m` is the form for a deliberate whole run.

## Testing

1. **Unit test of the decision** (internal/testsupport, default build): refuses for run empty, list empty, 10m; allows with run set; allows with list set; allows with an explicit 30m and with 0. Flipping any clause of the condition must redden a case.
2. **End-to-end wiring proof.** Run the real command shape, `go test -tags integration -count=1 -skip . ./internal/app/`, and assert a non-zero exit and output carrying the remedy (the shard glob and `-timeout`). `-skip .` keeps a broken guard cheap: with the guard or its call removed, every test is skipped and the run passes, so the assert reddens in seconds instead of running the 19-minute corpus. Place it in an existing file with budget room. `tests/test_go_integration_contract.sh` (15s ceiling) already compiles this binary with `-list`, so it is the first candidate. Measure its wall with the addition; if it no longer fits its row, use a sibling file rather than raising the ceiling (`tests/README.md`, "Never grow a file past its budget").
3. **Mutation-test the wiring.** Remove the `TestMain` call, watch check 2 redden, restore.
4. **Full suite at the build gate.** Every shard passes `-run`, the contract test's `-list` probes are allowed, and the e2e lane builds without the `integration` tag, so no existing caller changes behavior.

## Out of scope

- A `-timeout` backstop on the shard runners, and any change to suite or race-gate timeouts (0465, ADR-0108).
- Other packages' integration corpora.
- AGENTS.md and the learnings ledger.
- Editing merged plans that prescribe the unfiltered command (they are frozen build records).
- The closeout shard's budget breach (0475).
