<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0528 — Make the solo budget re-check detect a concurrent suite in another worktree](../../changes/archive/2026-10-17-0528-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a.md)**
<!-- docket:backlink:end -->

# Make the solo budget re-check detect a concurrent suite in another worktree — Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A solo budget re-check that reads over the line while another `docket development test` run of this repository is running is reported as `SERIAL CONFIRMATION NOT COUNTED:` and stays due, never as `SERIAL CONFIRMED OVER BUDGET:`.

**Architecture:** Every `suiterunner.Run` with a non-empty `cfg.StatePath` puts up a crash-safe "run sign": a file in `<dir of StatePath>/development-test-runs/`, held under an exclusive flock (`process.TryExclusiveLock`) for the run's lifetime. `soloConfirm` scans that folder (`process.ProbeLock`) just before launching the target and just after it returns; any other held sign, or any can't-tell answer, makes an over-line reading "not counted". A new file `internal/suiterunner/runsign.go` owns the sign and the scan. `budgetstate.go` owns the classification. `run.go` owns the sign's lifetime.

**Tech Stack:** Go (`internal/suiterunner`, `internal/process`), the `syscall.Flock` helpers already in `internal/process/lock.go`.

**Spec:** `docs/superpowers/specs/2026-10-16-make-the-solo-budget-re-check-detect-a-concurrent-suite-in-a-design.md` (on the `docket` metadata branch). Read it before Task 1.

## Global Constraints

- The new line's fixed prefix is exactly `SERIAL CONFIRMATION NOT COUNTED: ` and it must never match `^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):` (the release-candidate workflow fails a release on those prefixes).
- Stored result for a not-counted re-check: `last_confirmation_result` (`bsRecord.confRes`) = `not-counted`. Nothing else on the record changes (state, streak, since, lastSolo, dueSeq).
- Under-line readings clear exactly as today whether or not another run was seen. A non-zero target exit is `SERIAL CONFIRMATION FAILED:` exactly as today.
- Strict mode (`StrictConfirmCandidates`): a not-counted reading arms exit 4, like a failed confirmation. No other path can fail a run because of this change.
- Nothing waits, retries, sleeps, or adds time to a run.
- Every sign problem is advisory: it may print one warning line on stderr and must never change the exit code.
- Empty `cfg.StatePath`: no sign is put up and no check runs — exactly today's behavior.
- Tests use `testsupport.TempDir(t)`, never `t.TempDir()` or `os.MkdirTemp` (repoguard `TestRealProcessPackagesUseFixtureTempDir`).
- Comment cross-references anchor on symbol names, never line numbers (ADR-0054). Docs describe current behavior only — no change numbers in `tests/README.md` or the glossary.
- Every focused test run and every mutation probe uses `go test -count=1` (a cached `ok` is not evidence).

## Review Focus

1. **A crashed peer (SIGKILL, power loss) left its sign behind.** The flock dies with the process, so the sign probes free: it must be ignored and deleted, never counted. Pinned by Task 1 `dead visible sign` and Task 2 `TestDeadPeerSignDoesNotBlockConfirmation`.
2. **A peer that starts mid-measurement.** The before-check alone would miss it. Pinned by Task 2 `TestPeerAppearingDuringMeasurementIsCaught`.
3. **The sign folder cannot be created (read-only state dir, full disk).** One stderr warning, exit unchanged. Pinned by Task 3 `TestRunSignFailureWarnsAndPasses`.
4. **A crash between creating the temporary file and renaming it.** The temp file must never count as a sign and must not pile up. Pinned by Task 1 `stale temp swept` / `young temp kept`.
5. **A peer sign with empty or garbled content.** It is still a live run and still counts; the report names the sign file instead of a worktree. Pinned by Task 1 `held sign without worktree line`.

Accepted race (no test): two re-checks probing the same dead sign at once, or a probe briefly holding a free sign while another probes it. The worst outcome is a spurious "seen" (not counted, stays due), which is the safe direction. Note it in the `scanPeers` doc comment.

---

### Task 1: Run sign and peer scan (`runsign.go`)

**Files:**
- Create: `internal/suiterunner/runsign.go`
- Create: `internal/suiterunner/runsign_test.go`

**Interfaces:**
- Consumes: `process.TryExclusiveLock(path) (*os.File, bool, error)`, `process.ProbeLock(path) process.LockProbe` and its `LockProbeHeld/Free/Missing/Unknown` constants (`internal/process/lock.go`).
- Produces (all unexported, package `suiterunner`):
  - `const runSignDirName = "development-test-runs"`
  - `func runSignDir(statePath string) string` — `""` for `""`, else `filepath.Join(filepath.Dir(statePath), runSignDirName)`.
  - Visible sign name: `run-<pid>-<16 lowercase hex>.sign`, matched by `var runSignName = regexp.MustCompile(`^run-[0-9]+-[0-9a-f]{16}\.sign$`)`. Temporary name: the same with a leading `.tmp-` (`.tmp-run-<pid>-<hex>.sign`), matched by `runSignTempName`.
  - Sign content: `key=value` lines, at least `worktree=<repoRoot>`, `pid=<pid>`, `started=<RFC3339 UTC>`.
  - `type runSign struct { f *os.File; path string }`
  - `func putUpRunSign(dir, repoRoot string) (*runSign, error)`
  - `func (s *runSign) Path() string` — nil-safe, `""` on nil.
  - `func (s *runSign) takeDown()` — nil-safe; `os.Remove(path)` FIRST, then `f.Close()`, so a live run's visible sign is never unlocked.
  - `type peerScan struct { worktrees []string; unknown bool }`
  - `func (p peerScan) seen() bool` — `len(worktrees) > 0 || unknown`.
  - `func (p peerScan) merge(q peerScan) peerScan` — union of worktrees (deduped, sorted), OR of unknown.
  - `func (p peerScan) describe() string` — the reason text used in the NOT COUNTED line (exact values below).
  - `func scanPeers(dir, self string) peerScan`
  - `var runSignTempMaxAge = 10 * time.Minute` (package var so tests can stamp mtimes against it).

- [ ] **Step 1: Write the failing tests** in `runsign_test.go`. Helper used by this file and by Task 2's tests:

```go
// holdPeerSign plays "another run": it creates a visible sign in dir, takes the
// flock through a separate open file description (so ProbeLock reports held even
// in this process), and writes content (pass "" for none). release removes the
// sign and closes the file (idempotent); it is also registered with t.Cleanup.
func holdPeerSign(t *testing.T, dir, name, content string) (path string, release func())
```

It `MkdirAll`s dir, calls `process.TryExclusiveLock(filepath.Join(dir, name))` (fatal on err or busy), and writes content. Use names such as `run-424242-00000000000000ab.sign`.

Tests (table subtests where natural):

- `TestRunSignDir`: `runSignDir("")==""`; `runSignDir("/a/b/state.tsv")=="/a/b/development-test-runs"`.
- `TestPutUpRunSignIsHeldAndNamed`: dir = a not-yet-existing subdir of `testsupport.TempDir(t)`; `s, err := putUpRunSign(dir, "/repo/x")` → no error; `filepath.Dir(s.Path())==dir`; `runSignName` matches its base; `process.ProbeLock(s.Path())==process.LockProbeHeld`; file content contains `worktree=/repo/x\n`; no entry in dir matches `runSignTempName`. Then `s.takeDown()` → `ProbeLock(path)==LockProbeMissing`. `(*runSign)(nil).takeDown()` and `.Path()` do not panic, Path returns `""`.
- `TestPutUpRunSignNamesAreUnique`: two signs in one dir have different paths.
- `TestScanPeers` subtests (each asserts the full `peerScan` with `reflect.DeepEqual` or field checks):
  - `missing dir` → `peerScan{}`.
  - `empty dir` → `peerScan{}`.
  - `held peer` with content `worktree=/w/y\n` → `worktrees == ["/w/y"]`, `unknown == false`.
  - `two held peers` → both worktrees, sorted.
  - `self excluded`: a held sign passed as `self` → `peerScan{}`.
  - `dead visible sign`: a sign-named regular file nobody holds → `peerScan{}` AND the file no longer exists afterwards.
  - `held temp ignored`: held `.tmp-run-…sign` → `peerScan{}`.
  - `non-matching name ignored`: held file `notes.txt` → `peerScan{}`.
  - `directory with a sign name ignored`: `os.Mkdir(dir/run-1-0000000000000001.sign)` → `peerScan{}`.
  - `held sign without worktree line`: content `""` → `worktrees == [<sign base name>]`.
  - `unreadable dir` (skip when `os.Geteuid()==0`): dir chmod `0o000`, restored in `t.Cleanup` → `unknown == true`.
  - `unreadable sign` (skip as root): a sign-named file chmod `0o000` → `unknown == true`, file still exists.
- `TestScanPeersSweepsStaleTemps`: an unlocked `.tmp-run-…sign` whose mtime is `runSignTempMaxAge + time.Minute` in the past (`os.Chtimes`) is removed by `scanPeers`; an unlocked one with a fresh mtime is kept; a held one with an old mtime is kept. None affect the result.
- `TestPeerScanDescribe`:
  - `peerScan{worktrees: []string{"/a", "/b"}}.describe() == "another suite was running (/a, /b)"`
  - `peerScan{unknown: true}.describe() == "the runner could not check for other suites"`
  - both set → `"another suite was running (/a); could not check for other suites"`
  - `merge` dedupes `/a` from both sides and ORs unknown.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 -run 'TestRunSign|TestPutUpRunSign|TestScanPeers|TestPeerScan' ./internal/suiterunner/`
Expected: build failure (undefined `runSignDir`, `putUpRunSign`, …).

- [ ] **Step 3: Implement `runsign.go`.** File header comment: what a sign is, why flock (dies with the process, immune to pid reuse), why single-use names make checker deletion safe (contrast with `gatedrive`'s worktree lock, which never deletes because its name is fixed), and that every failure is advisory.

`putUpRunSign`: `MkdirAll(dir, 0o755)`; build a random 16-hex suffix (`crypto/rand`, 8 bytes); `process.TryExclusiveLock(dir/.tmp-run-<pid>-<hex>.sign)` (busy → error); write content; `os.Rename` to the visible name (the lock stays with the inode); on any error after the lock, remove the temp file and close. The lock is taken BEFORE the rename, so a visible sign is never unlocked while its owner lives.

`scanPeers` algorithm (decides cases the tests pin; write the doc comment to match):

```
entries, err := os.ReadDir(dir)
  not-exist      -> peerScan{}
  other err      -> peerScan{unknown: true}
for each entry:
  name matches runSignTempName:
      if regular, ProbeLock == Free, and mtime older than runSignTempMaxAge -> os.Remove (ignore error)
      continue                                    // temps never count
  name does not match runSignName, or entry not a regular file -> continue
  path == self                                    -> continue
  switch ProbeLock(path):
      Held    -> worktrees += worktree line of the file, or the base name if absent/unreadable
      Free    -> os.Remove(path) (ignore error)   // dead run
      Missing -> continue                         // vanished between ReadDir and probe
      Unknown -> unknown = true
```

Use `entry.Type().IsRegular()` (or `Info()`; an `Info()` error on a sign-named entry is `unknown`). Doc comment notes the accepted race from Review Focus.

- [ ] **Step 4: Run to verify they pass** — same command. Expected: PASS.

- [ ] **Step 5: Mutation-probe** (restore from a backup copy, never `git checkout --`): (a) make the `Free` arm count as held → `dead visible sign` reddens; (b) make `Unknown` fall through as clean → `unreadable sign` reddens; (c) drop the `self` exclusion → `self excluded` reddens; (d) drop the temp-name `continue` → `held temp ignored` reddens. Record each red in the commit body.

- [ ] **Step 6: Commit**

```bash
git add internal/suiterunner/runsign.go internal/suiterunner/runsign_test.go
git commit -m "feat(suiterunner): add the run sign and peer scan for budget re-checks"
```

---

### Task 2: Classify a contended over-line re-check as not counted

**Files:**
- Modify: `internal/suiterunner/budgetstate.go` (`budgetState`, `soloConfirm`, `ScheduleConfirmation`, `StrictConfirmCandidates`, file header and their doc comments)
- Modify: `internal/suiterunner/budgetstate_test.go`

**Interfaces:**
- Consumes: Task 1's `runSignDir`, `scanPeers`, `peerScan` (`seen`, `merge`, `describe`), `holdPeerSign` test helper.
- Produces:
  - New field `budgetState.selfSign string` — this run's own sign path, set by `Run` (Task 3); `""` excludes nothing.
  - `func (s *budgetState) soloConfirm(ctx context.Context, cfg Config, path string, ceiling int) (rc, secs int, peers peerScan)` — scans `runSignDir(cfg.StatePath)` (excluding `s.selfSign`) immediately before `ExecuteTarget` and immediately after it returns, and returns `before.merge(after)`. Empty `cfg.StatePath` → no scan, zero `peerScan`. An `ExecuteTarget` error still returns `1, 0` (peers irrelevant).
  - Test seam: `var soloMeasuredHook func()` — called, when non-nil, after `ExecuteTarget` returns and before the after-scan. Production leaves it nil.
  - `func notCountedLine(path string, secs int, threshold string, peers peerScan) string` returning exactly
    `fmt.Sprintf("SERIAL CONFIRMATION NOT COUNTED: %s — %ds solo while %s; solo threshold %ss; recheck stays due", path, secs, peers.describe(), threshold)`.
    Example: `SERIAL CONFIRMATION NOT COUNTED: tests/x.sh — 49s solo while another suite was running (/path/to/worktree-y); solo threshold 45s; recheck stays due`.

Decision table to implement in both callers (rc first, then over-line with peers seen, then today's branches):

| rc | SoloOver | peers.seen() | Scheduled outcome | Strict outcome |
|---|---|---|---|---|
| ≠0 | any | any | FAILED (today) | FAILED, armed (today) |
| 0 | true | true | print `notCountedLine`; `confRes = "not-counted"`; touch nothing else | same line; `confRes = "not-counted"`; `armed = true`; `lastSolo`/`state` untouched |
| 0 | true | false | CONFIRMED (today) | CONFIRMED, armed (today) |
| 0 | false | any | cleared (today) | cleared, not armed (today) |

- [ ] **Step 1: Write the failing tests** in `budgetstate_test.go`. Shared setup helper:

```go
// dueFixture makes one due record (5 overruns) for a passing script and returns
// the state, the target path, its key, and a Config whose StatePath lives in a
// fresh dir (so runSignDir(cfg.StatePath) is that dir's development-test-runs).
func dueFixture(t *testing.T, name string, soloSecs string) (*budgetState, string, string, Config)
```

Ceiling 10 (solo threshold `15`), parallel secs 30, solo injected through `writeDurations`. Over-line uses solo `20`, under-line uses solo `1`. Peer worktree literal: `/path/to/worktree-y`.

- `TestNotCountedWhenPeerSeenOverLine` (spec tests 1 and 11): hold a peer sign in `runSignDir(cfg.StatePath)`; snapshot the record; `ScheduleConfirmation`. Assert: output contains `SERIAL CONFIRMATION DUE: <path>`; contains the exact line `SERIAL CONFIRMATION NOT COUNTED: <path> — 20s solo while another suite was running (/path/to/worktree-y); solo threshold 15s; recheck stays due`; contains no `SERIAL CONFIRMED OVER BUDGET`; no output line matches `regexp.MustCompile(`^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):`)`; record `confRes == "not-counted"` and `state`, `streak`, `since`, `lastSolo`, `dueSeq` equal the snapshot. After the solo run, `dueCandidates()` still returns the key first.
- `TestPeerSeenUnderLineStillClears` (2): peer held, solo 1 → `state == "parallel-sensitive"`, `confRes == "cleared"`, `dueSeq == "-"`, `lastSolo == "1"`, no NOT COUNTED line.
- `TestNoPeerStillConfirms` (3): subtests `missing sign folder` and `empty sign folder` → CONFIRMED line, `confRes == "breached"`.
- `TestDeadPeerSignDoesNotBlockConfirmation` (4): a sign-named unlocked file in the folder → CONFIRMED, and the file is gone afterwards.
- `TestOwnSignIsNotContention` (5): `putUpRunSign` into the folder, `s.selfSign = sign.Path()`, `t.Cleanup(sign.takeDown)` → CONFIRMED.
- `TestPeerAppearingDuringMeasurementIsCaught` (6): folder empty at launch; set `soloMeasuredHook = func() { holdPeerSign(t, dir, …, "worktree=/path/to/worktree-y\n") }`, restore to nil in `t.Cleanup` → NOT COUNTED line naming `/path/to/worktree-y`.
- `TestPeerLeavingDuringMeasurementIsCaught` (guards the before-scan): peer held at launch; `soloMeasuredHook` calls the peer's `release` → NOT COUNTED line naming `/path/to/worktree-y`.
- `TestCannotTellIsNotCounted` (7, skip as root): subtests `unreadable folder` (chmod `0o000`) and `unreadable sign` (sign-named file chmod `0o000`) → line contains `while the runner could not check for other suites`, `confRes == "not-counted"`.
- `TestStrictNotCountedArms` (8): peer held; candidates `a` (solo 20) and `b` (solo 1) → NOT COUNTED for a, no CONFIRMED line, `armed == true`, `records[ka].confRes == "not-counted"`, `records[kb].confRes == "cleared"`. Subtest `under-line only`: only `b` with peer held → `armed == false`.

Existing tests (`TestConfirmationPrecedesAuthoritativeBreach`, `TestFailedConfirmationClearsNothing`, `TestStrictConfirmsAllAndArms`, …) build `Config` without `StatePath`; they must pass unchanged (spec test 10).

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 -run 'NotCounted|PeerSeen|NoPeer|DeadPeer|OwnSign|PeerAppearing|PeerLeaving|CannotTell' ./internal/suiterunner/`
Expected: build failure (`soloMeasuredHook`, `selfSign` undefined), then FAILs on missing NOT COUNTED lines.

- [ ] **Step 3: Implement** the interfaces and decision table above. Update comments: the file header's "Only a solo (uncontended) re-measurement over ceiling*3/2 is ever authoritative" becomes "…a solo re-measurement over ceiling*3/2, taken with no other suite seen, is ever authoritative"; say the same in the `soloConfirm`, `ScheduleConfirmation`, and `StrictConfirmCandidates` doc comments, naming `scanPeers` and `SERIAL CONFIRMATION NOT COUNTED`. Add `not-counted` to the `confRes` field comment on `bsRecord`.

- [ ] **Step 4: Run** `go test -count=1 ./internal/suiterunner/`. Expected: PASS (new and existing).

- [ ] **Step 5: Mutation-probe** (backup-copy restore): (a) delete the after-scan → `TestPeerAppearingDuringMeasurementIsCaught` reddens; (b) delete the before-scan → `TestPeerLeavingDuringMeasurementIsCaught` reddens; (c) Clear `lastSolo`/`dueSeq` in the not-counted branch → test 1 reddens. (d) Drop `armed = true` from the strict not-counted branch → test 8 reddens. Record the reds in the commit body.

- [ ] **Step 6: Commit**

```bash
git add internal/suiterunner/budgetstate.go internal/suiterunner/budgetstate_test.go
git commit -m "fix(suiterunner): never confirm a budget breach while another suite runs"
```

---

### Task 3: Put up the sign for the whole run

**Files:**
- Modify: `internal/suiterunner/run.go` (`Run`)
- Modify: `internal/suiterunner/run_test.go`

**Interfaces:**
- Consumes: Task 1's `runSignDir`, `putUpRunSign`, `(*runSign).Path`, `(*runSign).takeDown`; Task 2's `budgetState.selfSign`.
- Produces: no new symbols. The warning text is fixed: `run-tests: run sign not put up in <dir>: <err> — other suites will not see this run`.

- [ ] **Step 1: Write the failing tests** in `run_test.go`:

- `TestRunSignLivesForTheRun` (spec test 9). StatePath = `<TempDir>/state/state.tsv`; RepoRoot = a TempDir; one fixture target `hold` whose body touches an absolute marker path, then waits up to ~30s (bounded loop of `sleep 0.1`, so a broken test fails instead of hanging) for an absolute release path, then echoes `ok - hold`. Run `Run` in a goroutine. The test waits (deadline 30s) for the marker, then reads `runSignDir(StatePath)`: exactly one entry matches `runSignName`, `process.ProbeLock` on it is `LockProbeHeld`, its content contains `worktree=<RepoRoot>`, and no entry matches `runSignTempName`. Write the release file, wait for `Run` (deadline 30s), assert exit `0`, and assert no entry matching `runSignName` remains.
- `TestRunSignFailureWarnsAndPasses` (skip as root). StatePath inside a `0o555` dir (restore mode in `t.Cleanup`); `defer setLockTiming(2, time.Millisecond)()` so the store's own lock warning does not cost 3s. Green fixture → exit `0`, stderr contains `run sign not put up`.
- `TestRunWithoutStatePathPutsUpNoSign` (spec test 10). Green fixture with `StatePath == ""` → exit `0`, stderr does not contain `run sign`.

- [ ] **Step 2: Run to verify they fail**

Run: `go test -count=1 -run 'TestRunSign|TestRunWithoutStatePath' ./internal/suiterunner/`
Expected: `TestRunSignLivesForTheRun` fails (no sign), `TestRunSignFailureWarnsAndPasses` fails (no warning); `TestRunWithoutStatePathPutsUpNoSign` may already pass — that is fine, it guards the empty-path branch against Step 3.

- [ ] **Step 3: Implement.** In `Run`, after the work dirs are created and before `InstallSignalHandling` / `runLanes`: when `runSignDir(cfg.StatePath) != ""`, call `putUpRunSign(dir, cfg.RepoRoot)`; on error print the fixed warning to stderr and continue with a nil sign; otherwise `defer sign.takeDown()` (the deferred closure keeps the `*os.File` reachable for the whole run — say so in a comment, since a garbage-collected file would silently drop the lock). After each of the two `Load(...)` calls set `st.selfSign = sign.Path()`.

- [ ] **Step 4: Run** `go test -count=1 ./internal/suiterunner/`. Expected: PASS.

- [ ] **Step 5: Mutation-probe:** remove the `defer sign.takeDown()` → `TestRunSignLivesForTheRun` reddens on the after-run assert; remove `putUpRunSign` → it reddens on the during-run assert; turn the warning into `return 2` → `TestRunSignFailureWarnsAndPasses` reddens.

- [ ] **Step 6: Commit**

```bash
git add internal/suiterunner/run.go internal/suiterunner/run_test.go
git commit -m "feat(suiterunner): hold a run sign for the life of every suite run"
```

---

### Task 4: Docs, vocabulary sweep, and the build gate

**Files:**
- Modify: `tests/README.md` (budget paragraph under "Running it")
- Modify: `docs/reference/glossary.md` (entry "Budget watch / serially confirmed breach")
- Check, do not change unless the sweep says so: `AGENTS.md`, `.github/workflows/release-candidate.yml`, `tests/runtime-budgets.tsv` header, `internal/suiterunner/sandbox.go`.

**Interfaces:**
- Consumes: the line prefix `SERIAL CONFIRMATION NOT COUNTED:` from Task 2.
- Produces: nothing code-facing.

- [ ] **Step 1: Edit `tests/README.md`.** After the sentence ending "…is an authoritative breach (`SERIAL CONFIRMED OVER BUDGET:`).", add: a scheduled or strict re-check that reads over the line while another `docket development test` run, in any worktree of this repository, is running is not counted (`SERIAL CONFIRMATION NOT COUNTED:`) and the test stays due for the next quiet run; a re-check under the line counts either way. No change numbers.

- [ ] **Step 2: Edit the glossary entry.** Add `SERIAL CONFIRMATION NOT COUNTED:` to the lines it explains: a re-check outcome that is neither a screening finding nor a breach — another suite of the same repository was running, so the reading is set aside and the test is re-checked on a later quiet run.

- [ ] **Step 3: Vocabulary sweep.** Derive the consumers instead of trusting the spec's list:

Run: `git grep -n -E 'SERIAL CONFIRM|OVER BUDGET' -- ':!docs/results' ':!docs/superpowers' ':!docs/release'`
Expected: hits only in `internal/suiterunner/*`, `tests/README.md`, `tests/runtime-budgets.tsv` (header prose), `AGENTS.md` (prose), `docs/reference/glossary.md`, `.github/workflows/release-candidate.yml` (the `^(OVER BUDGET|SERIAL CONFIRMED OVER BUDGET):` grep, which the new prefix does not match), and the `sandbox.go` comment. Any executable consumer outside that set (a skill, a script, a repoguard pin) must be handled in this task and named in the commit body.

- [ ] **Step 4: Commit**

```bash
git add tests/README.md docs/reference/glossary.md
git commit -m "docs: describe the not-counted budget re-check line"
```

- [ ] **Step 5: Build gate.** Run the whole suite through the configured `build.test_command` (the Go runner, from source). Expected: exit `0`. Read the budget report: the `internal/suiterunner` tests ride `tests/test_go_race.sh` (row `60`); record its measured seconds and remaining margin as numbers in the results notes. A `SERIAL CONFIRMATION NOT COUNTED:` line in the gate's own output is expected behavior when another worktree's suite is running, not a failure.
