# Bring test_go_race, rebaserecovery, and closeout back under budget — Results

**Human action:** No action is required. The suite is green and no file is over budget. One optional check is listed below if you want to see the speed-up for yourself.

## Outcome

Three test files had run past their time budgets, and the race-detector gate had timed out once when two test runs shared the machine. This change fixes all four.

- **The race gate.** Most of the race gate's time went to one test in `internal/repoguard`, the retired-vocabulary seal. It ran about 110 regular expressions against every line it scanned. Each expression is now skipped unless the line contains the literal word that expression looks for. Every expression contains that word, so the check finds exactly what it found before. The seal dropped from 62.4s to 1.2s under `-race`, and the whole package from 85.9s to 21.8s. A new subtest, `prefilter_equivalence`, checks that the filtered scan and the unfiltered scan agree.
- **Two integration shards** are each split into two runner files. The tests themselves are unchanged; some were renamed so each half gets its own name prefix.
  - `closeout` is now `closeout` plus a new `archive`.
  - `rebaserecovery` is now `rebaserecovery` plus a new `rebaserefresh`.
- **gofmt:** this needed no change. The project's declared toolchain (go1.26.5) already reports `internal/githubcli/comment_integration_test.go` as formatted. Only the newer go1.27.1 `gofmt` on this machine's PATH flags it (see Known issues).

## Human actions and testing

### Optional — see the seal speed-up

Prerequisite: a checkout of this branch with Go installed.

1. Run `go test -race -count=1 -run TestRetiredVocabularySeal -v ./internal/repoguard`.
   Expected: every subtest passes, and `TestRetiredVocabularySeal` takes a few seconds rather than about a minute.

## Verification performed

Each new or re-measured file was timed on its own, with the machine otherwise quiet. A file breaches its budget at 1.5 times its row.

| File | Budget row | Breach threshold (row × 1.5) | Measured alone |
|---|---|---|---|
| `tests/test_go_race.sh` | 60 (kept) | 90s | 32.1s |
| `tests/test_go_integration_app_closeout.sh` | 40 | 60s | 33.1s |
| `tests/test_go_integration_app_archive.sh` (new) | 40 | 60s | 34.7s |
| `tests/test_go_integration_app_rebaserecovery.sh` | 40 | 60s | 34.6s |
| `tests/test_go_integration_app_rebaserefresh.sh` (new) | 35 | 52.5s | 26.8s |

- **Seal timing.** Run with `go test -race -count=1 -json ./internal/repoguard`.
  - Before: seal 62.42s, `maintained_surfaces` 53.13s, package 85.86s.
  - After: seal 1.18s, `maintained_surfaces` 1.03s, package 21.80s.
  - The before reading came from the earlier halted attempt, at the same head on a clean worktree (load about 6–14). The after reading ran at load about 2.
- **Mutation probes.** Each probe broke one filter on purpose. All three turned `non_vacuity` and `prefilter_equivalence` red, and the file was restored byte for byte afterwards.
  - M1, the text filter checks `r.New` instead of `r.Old`: `row 7: "gate-before" detected in 1 of 3 planted sites`, with 265 equivalence errors.
  - M2, the same change in the Go-literal filter: `row 7: "gate-before" detected in 2 of 3 planted sites`, with 524 equivalence errors.
  - M3, the bound-flag block skip inverted: `row 40: "--version" detected in 1 of 3 planted sites`, and a bound `--version` was missed.
- **Two concurrent gates.** This reproduces the shape behind the earlier timeout: one full suite and two `go test -race ./...` runs started together.
  - Before, on the untouched base 97cc46c3e: everything passed. `internal/repoguard` took 241.8s and 243.3s, far ahead of the next package (`internal/app`, about 82s). Peak 1-minute load was 31.7.
  - After, on the branch head: everything passed. `internal/repoguard` took 81.0s and 80.2s, in line with `internal/app` (71–74s) and `internal/gatedrive` (about 67s), and nowhere near the 480s backstop. Peak load was 29.8. The suite's `test_go_race` dropped from 367s to 202s.
- **Idle per-package reading** (`-p 2`, `GOMAXPROCS=2`, load 2–3): the slowest package is now `internal/cli` at 24.7s, with `internal/repoguard` second at 22.8s. The narrated number in the `tests/test_go_race.sh` header has been updated to match.
- **Shard contract.** `tests/test_go_integration_contract.sh` went red with the renamed tests orphaned, then green once each new runner existed. `TestRuntimeBudgetsCorrespondence` passes.
- **gofmt.** `go run cmd/gofmt -l internal/ cmd/` under `GOTOOLCHAIN=go1.26.5` prints nothing.

## Known issues and follow-ups

### A newer local gofmt flags one file

When `gofmt` from go1.27.1 is first on PATH, `gofmt -l internal/ cmd/` lists `internal/githubcli/comment_integration_test.go`. The project's declared toolchain, go1.26.5, does not flag it, and the suite's gofmt check uses the declared toolchain. This is confirmed. It does no harm today, but it will show up again whenever the repo moves to Go 1.27. Suggested next action: reformat the file as part of that toolchain bump.
