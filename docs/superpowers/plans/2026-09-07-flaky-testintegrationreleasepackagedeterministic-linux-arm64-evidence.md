# Change 0406 — reproduction evidence

Evidence record for the flaky `TestIntegrationReleasePackageDeterministic` (linux_arm64 bundle
nondeterminism) investigation. Task 1 of the implementation plan: bounded reproduction on the
unmodified base, plus a direct build-input probe. This file is the record later tasks and the
results file cite.

## Step 1 — Experiment header

- `git rev-parse HEAD`: `1d29fa77a8e48e308c17f0785dd5a6ec2cbc0120`
- `git status --porcelain`: clean (empty output) at the time of the runs below. The only untracked
  entry created during Task 1 is this evidence file itself, staged and committed in Step 6.
- `go version`: `go version go1.26.5 darwin/arm64`
- Date (UTC): 2026-09-07 (runs executed 2026-09-07 ~02:0x UTC; `date -u` → `Mon Sep  7 02:01:44 UTC 2026`)
- Machine note: darwin/arm64 host (`uname -sm` → `Darwin arm64`).
- Build-cache state: **warm** — `go clean -cache` was NOT run.

## Step 2 — Direct input probe

Command:

```bash
GOFLAGS= CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /tmp/docket-0406-probe ./cmd/docket
go version -m /tmp/docket-0406-probe   # vcs lines captured into a variable, then grepped (not piped to grep -q)
rm /tmp/docket-0406-probe
```

Captured `vcs` lines from `go version -m`:

```
	build	vcs=git
	build	vcs.revision=effc9a6dc549f1c7b1fa888707d3303681c80cad
	build	vcs.time=2026-09-06T13:00:04Z
	build	vcs.modified=false
```

All four expected lines are present: `vcs=git`, `vcs.revision`, `vcs.time`, `vcs.modified`. This
confirms that in the exact release build configuration (minus ldflags, which do not affect vcs
stamping) the produced linux_arm64 binary embeds ambient repository git state as a build input.
`vcs.revision`/`vcs.time` are the last-commit identity and `vcs.modified` reflects working-tree
cleanliness — any transient dirtying/cleaning of the checkout, or a mid-run commit, changes the
embedded buildinfo blob and therefore the binary bytes. This establishes ambient git state as an
embedded build input; it does not by itself prove the wild event's cause (that gate is Task 4).

## Step 3 — Isolated reproduction (exactly 5 runs)

```bash
for i in 1 2 3 4 5; do
  go test -tags integration -count=1 -run '^TestIntegrationReleasePackageDeterministic$' ./internal/release/
done
```

Verbatim per-run outcomes:

| Run | Outcome | Wall |
|-----|---------|------|
| 1   | `ok  github.com/danielhanold/docket/internal/release` (exit 0) | 77.661s |
| 2   | `ok  github.com/danielhanold/docket/internal/release` (exit 0) | 23.192s |
| 3   | `ok  github.com/danielhanold/docket/internal/release` (exit 0) | 13.138s |
| 4   | `ok  github.com/danielhanold/docket/internal/release` (exit 0) | 11.697s |
| 5   | `ok  github.com/danielhanold/docket/internal/release` (exit 0) | 11.850s |

All 5 PASS. No FAIL output to preserve.

## Step 4 — Suite-load reproduction (exactly 2 whole-suite runs)

```bash
go run ./cmd/docket development test   # run twice
```

**Suite run 1** — `SUITE files=40 passed=40 failed=0 asserts=368 wall=418s` (exit 0).
- `test_go_integration_release` row: `120s  rc=0  ok=4  notok=0`.
- Budget-clause lines: only `BUDGET WATCH:` screening findings (streak 1/5), including
  `test_go_integration_release.sh — 120s under -j11`. No `SERIAL CONFIRMED OVER BUDGET:` line.
  Recorded here, not acted on (routes through the standing gate rules per the plan).

**Suite run 2** — `SUITE files=40 passed=40 failed=0 asserts=368 wall=220s` (exit 0).
- `test_go_integration_release` row: `91s  rc=0  ok=4  notok=0`.
- Budget-clause lines: only `BUDGET WATCH:` screening findings (finalize_e2e, go_race, go_toolchain,
  streak 2/5). `test_go_integration_release.sh` was NOT flagged this run. No
  `SERIAL CONFIRMED OVER BUDGET:` line.

No determinism failure occurred in either suite-load run; no failing output to preserve.

## Step 5 — Bounded-extent statement

**5 isolated + 2 suite-load executions; 0 mismatches reproduced.** The intermittent
linux_arm64 bundle checksum mismatch did not recur within this bounded experiment. No runs were
added beyond the prescribed bound. The Step 2 probe independently confirms the leading enumerated
mechanism's precondition (ambient git state embedded in the release binaries) is live in this
environment; whether that mechanism caused the wild 0403-gate event remains for Task 4's controlled
demonstration to address (per the plan's Global Constraints, the fix ships only if that demonstration
reddens).

## Task 4 Step 3 — Mechanism demonstration (red-first, with positive control)

Mechanism demonstrated on a hermetic fixture: an ambient tree-state change alters the built bytes
under the current release flag set. The regression test `TestIntegrationReleaseBuildIgnoresAmbientGitState`
builds a committed single-file Go module in its own git repo, then dirties the tree via a tracked
NON-Go file (`README.md`) so `vcs.modified` flips while every compile input stays identical. The
**positive control** (`buildDefault`, plain `go build -trimpath` at default `-buildvcs`) confirms the
mechanism is live before the immunity assert runs.

Command:

```bash
go test -tags integration -count=1 -run '^TestIntegrationReleaseBuildIgnoresAmbientGitState$' ./internal/release/
```

Verbatim failing output (before the `-buildvcs=false` fix):

```
=== RUN   TestIntegrationReleaseBuildIgnoresAmbientGitState
    build_determinism_integration_test.go:110: buildTuple output depends on ambient git state: clean and dirty-tree builds differ (1749266 vs 1749266 bytes)
--- FAIL: TestIntegrationReleaseBuildIgnoresAmbientGitState (0.58s)
FAIL
FAIL	github.com/danielhanold/docket/internal/release	0.815s
FAIL
```

The failure is at the **immunity assert** (`buildTuple output depends on ambient git state`), NOT the
positive control. The control passed — i.e. the clean vs dirty binaries under default `-buildvcs`
genuinely differ — so the assert is not vacuous: `buildTuple`'s output (today's release flag set,
`-trimpath` but no `-buildvcs=false`) is byte-sensitive to ambient repository tree state. The two
binaries are the same size (1749266 bytes) but differ in content, consistent with a differing embedded
`vcs.modified` buildinfo stamp rather than a code-size change. This is the red-first demonstration the
Global Constraints gate requires before shipping the fix.

## Task 4 Step 5 — Fix applied

`-buildvcs=false` was added to `buildTuple`'s `go build` argv (after `-trimpath`); the doc comment was
finalized to state that the produced bytes depend only on the declared inputs and never on ambient
repository VCS state. With the fix in place:

```bash
go test -tags integration -count=1 -run '^TestIntegrationReleaseBuildIgnoresAmbientGitState$' ./internal/release/
# ok  github.com/danielhanold/docket/internal/release  1.008s   (regression now PASSES)

go test -tags integration -count=1 -run '^TestIntegrationRelease' ./internal/release/
# ok  github.com/danielhanold/docket/internal/release  17.831s  (all four tuples: e2e + determinism + collision + checksums)
```

## Task 4 Step 7 — Repair-linkage mutation test (remove → red, restore → green)

The fix is causally linked to the regression: removing `-buildvcs=false` reddens the immunity assert
for the intended reason, and restoring it greens.

**Remove `-buildvcs=false`, re-run:**

```
=== RUN   TestIntegrationReleaseBuildIgnoresAmbientGitState
    build_determinism_integration_test.go:110: buildTuple output depends on ambient git state: clean and dirty-tree builds differ (1749266 vs 1749266 bytes)
--- FAIL: TestIntegrationReleaseBuildIgnoresAmbientGitState (0.57s)
FAIL
FAIL	github.com/danielhanold/docket/internal/release	0.783s
FAIL
```

**Restore `-buildvcs=false`, re-run:**

```
=== RUN   TestIntegrationReleaseBuildIgnoresAmbientGitState
--- PASS: TestIntegrationReleaseBuildIgnoresAmbientGitState (0.49s)
PASS
ok  	github.com/danielhanold/docket/internal/release	0.710s
```

The mutation reddens at the same immunity assert (`build_determinism_integration_test.go:110`) and no
other; the restore greens. This is the spec's revert/restore proof — the repair (`-buildvcs=false`) is
the exact thing the regression pins.
