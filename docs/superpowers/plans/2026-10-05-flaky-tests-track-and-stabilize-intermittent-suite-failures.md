<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0507 — Fix the observe-test hang and bring two test files back under their time limits](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0507-flaky-tests-track-and-stabilize-intermittent-suite-failures.md)**
<!-- docket:backlink:end -->
# Observe-Test Hang and Two Over-Limit Test Files — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`.

**Goal:** Make `TestObserveRunningThenTerminal`'s 30s hang either fixed at its root cause or self-explaining, and bring `tests/test_go_integration_app_merge.sh` and `tests/test_go_finalize_e2e.sh` to a quiet solo median of 25s or less (or an argued row re-set), with every timing recorded before and after.

**Architecture:** Part A tightens the test (no discarded errors) and adds a test-only `describeStuckRun` report printed when the 30s wait expires. A bounded reproduction under two load profiles then decides whether a root-cause fix (A3) follows. Part B lets the 12 `TestIntegrationFinalizeMerge*` tests call `t.Parallel()` by giving the shared fixture a planning-node builder that never calls `t.Setenv` (the `e2eNode` precedent). Part C measures the e2e file phase by phase and cuts only measured, redundant work. Part D measures the race file only. Measurements are taken by small scratch scripts that live outside the repo, under `${TMPDIR:-/tmp}/docket-0507/`. The numbers are carried in commit messages.

**Tech Stack:** Go tests (`internal/process`, `internal/app` under the `integration` and `e2e` build tags), POSIX shell test wrappers under `tests/`, macOS `sysctl`/`pgrep`/`perl` for measurement.

**Spec:** `docs/superpowers/specs/2026-10-05-flaky-tests-track-and-stabilize-intermittent-suite-failures-design.md` (on the `docket` branch; read it from `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-05-flaky-tests-track-and-stabilize-intermittent-suite-failures-design.md`).

## Global Constraints

- **Never** raise the 30s deadline in `TestObserveRunningThenTerminal`, add retries, loosen or remove an assertion, or skip the test under load (spec A5).
- Retry-on-failure machinery and any change to gate or budget policy (the 5/2 and 3/2 factors, exit codes, the screen-then-confirm regime) are out of scope. So is making the runner's solo re-check notice another suite, and so is #273.
- Every timing number is taken with the shared measurement protocol: from the feature worktree, `bash tests/<file>.sh` with `DOCKET_GO_TEST_CONCURRENCY` unset, a private HOME, TMPDIR and XDG_CONFIG_HOME, and a quiet machine (no competing `docket development test` / `go test` / test binary, 1-minute load average below the CPU count). Take one discarded warm run, then three runs, and report the median. Record each run's seconds and starting load average. The `measure.sh` script embedded in Tasks 4, 5 and 6 implements this exactly.
- Target for Parts B and C: a quiet solo median of **25s or less**. Fallback: re-set the row by the table's own sizing rule (post-cut median rounded up to the next multiple of 5, plus a 5s margin) and argue it in the commit message: what was cut, what remains, and why the rest cannot be cut. Never raise a row without the cut attempt.
- Never cut assertions or coverage. For the e2e file, keep the `go vet -tags e2e ./internal/app/` coverage, the build-tag assert, and both completeness asserts.
- Every Go verification run uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`). Every mutation is made on a backup copy and restored from it with `cp`, never with `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- Run integration tests only with a `-run` prefix: `go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/`.
- Measurement and reproduction scripts and notes live under `${TMPDIR:-/tmp}/docket-0507/`, never in the repo. Measurement numbers go into the task's commit message. A task with no code change records its numbers in an empty commit (`git commit --allow-empty`).
- Shell rules from AGENTS.md apply to the scratch scripts: no producer piped into an early-exiting consumer under pipefail, `grep -e` for patterns, templated `mktemp`.
- Cross-references in maintained source anchor on symbol names, never line numbers (`TestCommentAnchorStyle`).
- Every `os.MkdirTemp` in test code carries a `// tempdir-exempt: <reason>` comment on the line above (`internal/repoguard/tempdir_fixture_test.go`).
- Stage explicit paths only; never `git add -A`. Do not edit anything on the `docket` metadata branch.
- Run tasks strictly in order. Task 2's load generators must be fully stopped before any measurement task (4, 5, 6) runs.

## Review Focus

1. **A parallelized merge test that shares mutable package state with another** (a package-level hook, seam, or shared fake). A person expects the shard to stay correct and race-free, not flaky in a new way. Pinned in Task 4 Step 6: the shard runs once under `-race`, and any test that swaps package state stays sequential with a comment.
2. **The parallel fixture silently dropping global-config isolation.** A person expects merge tests never to read their own `~/.config/docket/config.yml`. Pinned in Task 4 by `TestIntegrationFinalizeMergeParallelNodeIsolatesGlobalConfig`, mutation-tested.
3. **A stuck-run report that itself crashes or misleads when `Observe` never succeeded** (nil observation, error every poll) **or the manifest names no real group.** A person expects the report to say "<none>" and "unknown", never to panic and never to call an unprovable probe "absent". Pinned in Task 1 by `TestDescribeStuckRunNamesEveryProbe`.
4. **A measurement taken while another worktree's suite is running.** A person expects every recorded number to say whether it was quiet. Pinned by `measure.sh`'s quiet check, which labels the run `CONTENDED` rather than passing it off as quiet (Tasks 4, 5, 6).
5. **An e2e cut that leaves the completeness or vet asserts vacuous.** A person expects a renamed `TestE2E*` function to still turn the file red. Pinned in Task 5 Step 7 by a rename mutation.

## File Structure

- Modify `internal/process/observe_test.go`: `TestObserveRunningThenTerminal` stops discarding the `readManifest`/`signalGroup` results and reports what was stuck on timeout. Adds the test-only helper `describeStuckRun` and its two tests.
- Possibly modify `internal/process/*` (Task 3 only, only if the hang reproduces): the root-cause fix.
- Modify `internal/app/finalize_rebase_test.go`: extract `setupRebaseFixtureWithNode` (planning-node builder injected); `setupRebaseFixtureStatus` delegates to it.
- Modify `internal/app/finalize_merge_test.go`: extract `setupMergeFixtureWithNode`; `setupMergeFixture` delegates to it.
- Modify `internal/app/finalize_merge_integration_test.go`: add `parallelPlanningNode` plus its isolation test, add `t.Parallel()` to the tests, and route fixtures through the no-Setenv builder.
- Modify `tests/test_go_finalize_e2e.sh` and/or `internal/app/finalize_e2e_test.go` (Task 5): only the measured cuts.
- Possibly modify `tests/runtime-budgets.tsv` (fallback only).

---

### Task 1: The observe test stops discarding errors and reports what was stuck

**Files:**
- Modify: `internal/process/observe_test.go` (`TestObserveRunningThenTerminal`; new `describeStuckRun`, `TestDescribeStuckRunNamesEveryProbe`, `TestDescribeStuckRunReportsLiveRun`)

**Interfaces:**
- Consumes (existing, package `process`): `readManifest(runDir string) (*manifestRecord, error)`, `signalGroup(pgid int, sig syscall.Signal) error`, `groupAlive(pgid int) probeAnswer`, `processAlive(pid int) probeAnswer`, `probeFlock(path string) (held bool, answer probeAnswer)`, `liveLockFile`, `probeAnswer.String()` returning `live`/`absent`/`unknown`, `(*Service).Observe(runDir) (*Observation, error)`, `launchHelper`, `newTestService`, `waitFor`, `syscall_SIGKILL()`.
- Produces: `describeStuckRun(runDir string, m *manifestRecord, last *Observation, lastErr error) string` (test-only). Its output contains these exact fragments: `state=<State or "<none>">`, `err=<error text or "<none>">`, `group <pgid>: <live|absent|unknown>`, `supervisor pid <pid>: <live|absent|unknown>`, `live.lock: <held|free|unknown>`. On timeout the hang test fails with a message starting `timed out waiting for vanished: `. Task 2's reproduction script counts that exact string.

- [ ] **Step 1: Write the failing tests**

Append to `internal/process/observe_test.go` (add `"errors"`, `"fmt"`, `"strings"` to the imports):

```go
// TestDescribeStuckRunNamesEveryProbe pins the report's shape when nothing is
// provable: no observation ever succeeded, the manifest names no real group or
// supervisor, and no lock file exists. Unprovable probes must read "unknown",
// never "absent", and a nil observation must not panic.
func TestDescribeStuckRunNamesEveryProbe(t *testing.T) {
	dir := testsupport.TempDir(t)
	m := &manifestRecord{PGID: 0, SupervisorPID: 0}
	got := describeStuckRun(dir, m, nil, errors.New("boom"))
	for _, want := range []string{
		"state=<none>",
		"err=boom",
		"group 0: unknown",
		"supervisor pid 0: unknown",
		"live.lock: free",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report %q missing %q", got, want)
		}
	}
}

// TestDescribeStuckRunReportsLiveRun pins the report against a real, still
// running run: the group and supervisor are live and live.lock is held.
func TestDescribeStuckRunReportsLiveRun(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	obs, oerr := svc.Observe(out.RunDir)
	got := describeStuckRun(out.RunDir, m, obs, oerr)
	for _, want := range []string{
		"state=running",
		"err=<none>",
		fmt.Sprintf("group %d: live", m.PGID),
		fmt.Sprintf("supervisor pid %d: live", m.SupervisorPID),
		"live.lock: held",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report %q missing %q", got, want)
		}
	}
	// Teardown: launchHelperReq's drain (quiesceRun) kills the still-running group.
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures && go test -count=1 -run '^TestDescribeStuckRun' ./internal/process`
Expected: build failure `undefined: describeStuckRun`.

- [ ] **Step 3: Implement `describeStuckRun`**

Add to `internal/process/observe_test.go`, after `observeUntilTerminal`:

```go
// describeStuckRun reports what was still stuck when a kill-then-wait-for-gone
// test's wait expired: the last Observe state and error, whether the killed
// group and its supervisor still have live members (zero-signal probes), and
// whether live.lock is still held. Every probe keeps its three-way answer;
// "unknown" is never folded into "absent". A pid or pgid <= 1 is not probed
// (kill(0, 0) would address the caller's own group) and reads "unknown".
func describeStuckRun(runDir string, m *manifestRecord, last *Observation, lastErr error) string {
	state := "<none>"
	if last != nil {
		state = string(last.State)
	}
	errText := "<none>"
	if lastErr != nil {
		errText = lastErr.Error()
	}
	supervisor := probeUnknown
	if m.SupervisorPID > 1 {
		supervisor = processAlive(m.SupervisorPID)
	}
	held, ans := probeFlock(filepath.Join(runDir, liveLockFile))
	lock := "free"
	switch {
	case ans == probeUnknown:
		lock = "unknown"
	case held:
		lock = "held"
	}
	return fmt.Sprintf("last observe state=%s err=%s; group %d: %v; supervisor pid %d: %v; live.lock: %s",
		state, errText, m.PGID, groupAlive(m.PGID), m.SupervisorPID, supervisor, lock)
}
```

(`groupAlive` already returns `probeUnknown` for a pgid of 1 or less. That is why the group probe needs no guard of its own.)

- [ ] **Step 4: Run the new tests to verify they pass**

Run: `go test -count=1 -run '^TestDescribeStuckRun' ./internal/process`
Expected: `ok`.

- [ ] **Step 5: Rewrite `TestObserveRunningThenTerminal` to stop discarding results and to report on timeout**

Replace the body after the `obs.StdoutLog` check (the `m, _ := readManifest(...)` line through the end of the `waitFor` call) with:

```go
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if err := signalGroup(m.PGID, syscall_SIGKILL()); err != nil {
		t.Fatalf("SIGKILL group %d: %v", m.PGID, err)
	}
	// Supervisor dies with the child under KILL: no terminal record can
	// exist, no stop intent was recorded -> vanished. An Observe error while
	// polling is kept (lock release can race) and reported if the wait expires.
	var last *Observation
	var lastErr error
	end := time.Now().Add(30 * time.Second)
	for {
		last, lastErr = svc.Observe(out.RunDir)
		if lastErr == nil && last.State == StateVanished {
			return
		}
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for vanished: %s", describeStuckRun(out.RunDir, m, last, lastErr))
		}
		time.Sleep(20 * time.Millisecond)
	}
```

The 30s deadline and the 20ms interval are unchanged. The function no longer calls `waitFor`. Leave `waitFor` itself untouched: other tests use it.

- [ ] **Step 6: Run the package tests, including under the race detector**

Run: `go test -count=1 ./internal/process && go test -race -count=20 -run '^TestObserveRunningThenTerminal$|^TestDescribeStuckRun' ./internal/process`
Expected: both `ok`. Each `TestObserveRunningThenTerminal` iteration takes well under 1s.

- [ ] **Step 7: Mutation-test both changes (restore from a backup copy)**

```bash
cd /Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures
B="$(mktemp "${TMPDIR:-/tmp}/observe_test.go.XXXXXX")"; cp internal/process/observe_test.go "$B"
# Mutation A: the report claims the group is always absent -> TestDescribeStuckRunReportsLiveRun must redden.
perl -0pi -e 's/m\.PGID, groupAlive\(m\.PGID\), m\.SupervisorPID/m.PGID, probeAbsent, m.SupervisorPID/' internal/process/observe_test.go
go test -count=1 -run '^TestDescribeStuckRunReportsLiveRun$' ./internal/process; echo "rc=$? (want non-zero)"
cp "$B" internal/process/observe_test.go
# Mutation B: kill a non-real group -> the test must fail IMMEDIATELY naming the refusal, not after 30s.
perl -0pi -e 's/signalGroup\(m\.PGID, syscall_SIGKILL\(\)\)/signalGroup(0, syscall_SIGKILL())/' internal/process/observe_test.go
go test -count=1 -v -run '^TestObserveRunningThenTerminal$' ./internal/process 2>&1 | tail -n 5; echo "(want FAIL in well under 30s naming 'refusing to signal non-real group 0')"
cp "$B" internal/process/observe_test.go
git diff --stat   # only the intended Task 1 edits remain
```

If either mutation stays green, the assert is defective. Fix it before going on.

- [ ] **Step 8: Commit**

```bash
git add internal/process/observe_test.go
git commit -m "test(process): observe hang test fails fast on discarded errors and reports what was stuck

TestObserveRunningThenTerminal no longer discards readManifest's error or
signalGroup's result, and when its unchanged 30s wait expires it reports the
last Observe state/error, group and supervisor liveness, and live.lock state
(describeStuckRun). Mutation-tested: both mutations reddened."
```

---

### Task 2: Bounded reproduction of the hang under two load profiles (spec A1)

No repository file changes. This task runs a bounded reproduction attempt against the Task 1 build, so any hang prints its stuck-run report. It records the attempts in an empty commit.

**Files:**
- Create (scratch, outside the repo): `${TMPDIR:-/tmp}/docket-0507/repro.sh`
- Writes (scratch): `${TMPDIR:-/tmp}/docket-0507/repro-state.tsv`, `failure-*.log`, `measurements.md`

**Interfaces:**
- Consumes: Task 1's failure text `timed out waiting for vanished: ` and the `describeStuckRun` report it carries.
- Produces for Task 3: `repro-state.tsv` rows `profile<TAB>iterations<TAB>hangs<TAB>other_failures<TAB>seconds<TAB>first_failure_at<TAB>load1`. It also produces a recorded verdict line, either `REPRO-VERDICT reproduced profile=<p> first_failure_at=<n>` or `REPRO-VERDICT not-reproduced`.

**Bounds (the spec's A1 cap, kept smaller):** at most 2,000 iterations or 1,500 seconds per profile, two profiles. That is 4,000 iterations and 50 minutes overall, inside the spec's 5,000-iteration / one-hour bound. Stop at the first failure of any kind.

- [ ] **Step 1: Write the reproduction script**

```bash
mkdir -p "${TMPDIR:-/tmp}/docket-0507"
cat > "${TMPDIR:-/tmp}/docket-0507/repro.sh" <<'EOF'
#!/usr/bin/env bash
# repro.sh <worktree> <cpu|gosuite> [slice-seconds] — one bounded slice of change
# 0507's A1 reproduction loop for TestObserveRunningThenTerminal. Re-run until the
# profile reports its cap or a failure. One row per batch goes to repro-state.tsv:
#   profile iterations hangs other_failures seconds first_failure_at load1
set -uo pipefail
set -m   # background load jobs get their own process groups so stop_load kills whole trees
wt="$1"; profile="$2"; budget="${3:-480}"
case "$profile" in cpu|gosuite) ;; *) echo "profile must be cpu or gosuite" >&2; exit 2 ;; esac
S="${TMPDIR:-/tmp}/docket-0507"; mkdir -p "$S"
state="$S/repro-state.tsv"; touch "$state"
ncpu="$(sysctl -n hw.ncpu)"
BATCH=50; CAP_ITERS=2000; CAP_SECS=1500
cum(){ awk -F'\t' -v p="$profile" -v c="$1" '$1==p{s+=$c} END{print s+0}' "$state"; }
anyfail(){ awk -F'\t' '($3+$4)>0{f=1} END{print f+0}' "$state"; }
if [ "$(anyfail)" = 1 ]; then echo "REPRO: a failure is already recorded; stop reproducing"; exit 0; fi
bin="$S/process.test"
( cd "$wt" && go test -race -c -o "$bin" ./internal/process ) || exit 1
loadpids=()
stop_load(){
  local p
  for p in ${loadpids[@]+"${loadpids[@]}"}; do kill -- "-$p" 2>/dev/null || kill "$p" 2>/dev/null; done
  wait 2>/dev/null
  loadpids=()
}
trap stop_load EXIT
if [ "$profile" = cpu ]; then
  for ((i = 0; i < 2 * ncpu; i++)); do yes >/dev/null & loadpids+=("$!"); done
else
  ( cd "$wt" && while :; do go test -race -count=1 ./internal/... >/dev/null 2>&1; done ) &
  loadpids+=("$!")
fi
sleep 5   # let the load ramp up
start=$SECONDS
while :; do
  it="$(cum 2)"; secs="$(cum 5)"
  if [ "$it" -ge "$CAP_ITERS" ] || [ "$secs" -ge "$CAP_SECS" ]; then
    echo "REPRO: profile $profile reached its cap ($it iterations, ${secs}s)"; break
  fi
  if [ $((SECONDS - start)) -ge "$budget" ]; then echo "REPRO: slice budget spent; re-run to continue"; break; fi
  load="$(sysctl -n vm.loadavg | awk '{print $2}')"
  b0=$SECONDS
  out="$(cd "$wt/internal/process" && "$bin" -test.run '^TestObserveRunningThenTerminal$' \
        -test.count="$BATCH" -test.timeout=20m -test.v 2>&1)"
  dur=$((SECONDS - b0))
  passes="$(grep -c -e '^--- PASS: TestObserveRunningThenTerminal' <<<"$out")"
  fails="$(grep -c -e '^--- FAIL: TestObserveRunningThenTerminal' <<<"$out")"
  hangs="$(grep -c -e 'timed out waiting for vanished' <<<"$out")"
  other=$((fails - hangs)); [ "$other" -lt 0 ] && other=0
  ran=$((passes + fails))
  if [ "$ran" -lt "$BATCH" ] && [ "$fails" -eq 0 ]; then other=1; fi   # crash / panic / timeout of the binary
  first="-"
  if [ $((hangs + other)) -gt 0 ]; then
    pos="$(awk '/^--- PASS: TestObserveRunningThenTerminal/{n++} /^--- FAIL: TestObserveRunningThenTerminal/{print n+1; exit}' <<<"$out")"
    first=$((it + ${pos:-$ran}))
    log="$S/failure-$profile-$first.log"; printf '%s\n' "$out" >"$log"
    echo "REPRO: failure in $profile at cumulative iteration $first; full output in $log"
  fi
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$profile" "$ran" "$hangs" "$other" "$dur" "$first" "$load" >>"$state"
  if [ $((hangs + other)) -gt 0 ]; then echo "REPRO: FAILURE REPRODUCED"; break; fi
done
printf 'REPRO-TOTAL profile=%s iterations=%s hangs=%s other_failures=%s seconds=%s ncpu=%s\n' \
  "$profile" "$(cum 2)" "$(cum 3)" "$(cum 4)" "$(cum 5)" "$ncpu"
EOF
chmod +x "${TMPDIR:-/tmp}/docket-0507/repro.sh"
```

- [ ] **Step 2: Start from a clean state and confirm no stray load exists**

```bash
rm -f "${TMPDIR:-/tmp}/docket-0507/repro-state.tsv"
pgrep -x yes || echo "no yes processes"
```

- [ ] **Step 3: Run the CPU-stress profile, slice by slice**

Run in the foreground with a Bash timeout of 600000 ms. Do not background the command and do not yield:

`"${TMPDIR:-/tmp}/docket-0507/repro.sh" /Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures cpu 480`

Repeat the same command until it prints `reached its cap` or `FAILURE REPRODUCED`. After each slice, confirm the load is gone: `pgrep -x yes || echo clean`. Kill any listed pid; they are this script's leftovers.

- [ ] **Step 4: Run the Go-suite-load profile (only if Step 3 recorded no failure)**

This profile stands in for "a full suite run alongside". It runs `go test -race ./internal/...` in a loop instead of `docket development test`, so no contended reading lands in the runner's budget history.

`"${TMPDIR:-/tmp}/docket-0507/repro.sh" /Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures gosuite 480`

Repeat until `reached its cap` or `FAILURE REPRODUCED`. After each slice, confirm nothing is left running: capture `out="$(pgrep -fl 'go test -race -count=1 ./internal/' || true)"` and kill any pid it lists.

- [ ] **Step 5: Record the verdict in the notes file**

```bash
S="${TMPDIR:-/tmp}/docket-0507"
{ echo "## A1 reproduction ($(date -u +%FT%TZ))"
  echo '```'; cat "$S/repro-state.tsv"; echo '```'
  awk -F'\t' '{i[$1]+=$2; h[$1]+=$3; o[$1]+=$4; s[$1]+=$5} END{for (p in i) printf "REPRO-TOTAL profile=%s iterations=%d hangs=%d other_failures=%d seconds=%d\n", p, i[p], h[p], o[p], s[p]}' "$S/repro-state.tsv"
  f="$(awk -F'\t' '$6!="-"{print $1" "$6; exit}' "$S/repro-state.tsv")"
  if [ -n "$f" ]; then set -- $f; echo "REPRO-VERDICT reproduced profile=$1 first_failure_at=$2"; else echo "REPRO-VERDICT not-reproduced"; fi
} >>"$S/measurements.md"
tail -n 8 "$S/measurements.md"
```

If a failure was recorded, also copy the `t.Fatalf` line, which holds the stuck-run report, from the `failure-*.log` file into the notes.

- [ ] **Step 6: Commit the record (empty commit)**

```bash
git commit --allow-empty -F - <<EOF
test(process): record the bounded A1 reproduction of the observe hang

$(sed -n '/## A1 reproduction/,$p' "${TMPDIR:-/tmp}/docket-0507/measurements.md")
EOF
```

---

### Task 3: Root-cause fix if the hang reproduced (spec A3), otherwise a recorded no-op

**Files:**
- Read: `${TMPDIR:-/tmp}/docket-0507/measurements.md` (the `REPRO-VERDICT` line from Task 2), `failure-*.log`
- Modify (only on `reproduced`): `internal/process/observe_test.go` and/or the `internal/process` source the evidence implicates, plus every sibling test with the same kill-then-wait-for-gone shape

**Interfaces:**
- Consumes: Task 2's `REPRO-VERDICT` and `repro-state.tsv`. Also `repro.sh` (same arguments) for the proof streak.
- Produces: either a fix commit carrying the streak evidence, or an empty commit recording that A3 was not triggered and the root cause stays open (spec A4 third bullet).

- [ ] **Step 1: Read the verdict**

`grep -e 'REPRO-VERDICT' "${TMPDIR:-/tmp}/docket-0507/measurements.md"`

If it is `not-reproduced`, skip to Step 7.

- [ ] **Step 2: Diagnose from the evidence (only on `reproduced`)**

Use superpowers:systematic-debugging. Start from the stuck-run report in the failure log and from the spec's A2 suspects:
- Did `signalGroup` succeed? If not, the failure is the `SIGKILL group` fatal, and the reason is named in it.
- Was `m.PGID` the real group?
- Is `live.lock` still held after the kill, and by whom? While a hang is live, `lsof <runDir>/live.lock` lists the holders. Is every holder in the killed group?
- What did `Observe` return the whole time: a persistent error (which one), or a persistent `running`?

Treat these as hypotheses (learning `groomed-root-cause-is-a-hypothesis`). Enumerate every code path in `(*Service).Observe` that can keep returning a non-vanished answer before fixing.

- [ ] **Step 3: Write a test that fails for the identified cause**

Where the cause can be forced deterministically, for example through an existing seam like `observePostProbeHook`, write a focused failing test for it. Where it cannot, the proof is the Step 5 streak.

- [ ] **Step 4: Fix the real cause, then apply it to every same-shape site**

Derive the sites from a grep, never from a hand list:

`grep -n -e 'signalGroup(' internal/process/*_test.go`

Today this includes `observe_test.go`, `recover_test.go`, `worktree_lock_test.go`, `leftover_test.go` and `launch_test.go`. Apply the fix to each site whose shape is "kill, then wait for gone". A5 still holds: never raise the 30s, add retries, or loosen an assert.

- [ ] **Step 5: Prove the fix under the reproducing profile**

Required streak: `max(1000, 3 × first_failure_at)` consecutive iterations with zero failures, under the profile named in `REPRO-VERDICT`. Reset the state, then re-run slices until the streak is reached. If the streak needs more than 2,000 iterations, raise `CAP_ITERS` and `CAP_SECS` in a copy of the script to fit:

```bash
S="${TMPDIR:-/tmp}/docket-0507"; mv -f "$S/repro-state.tsv" "$S/repro-state-before-fix.tsv"
"$S/repro.sh" /Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures <profile> 480
```

Any failure resets the claim. Go back to Step 2.

- [ ] **Step 6: Commit the fix (on `reproduced`)**

`git add` the exact files changed, then commit with a message that states the cause, the fix, the sites it was applied to (with the grep that found them), and the before/after evidence. Before: the profile and `first_failure_at`. After: the streak length and profile.

- [ ] **Step 7: On `not-reproduced`, record the no-op (empty commit)**

```bash
git commit --allow-empty -m "test(process): A3 not triggered; observe hang root cause remains open

The bounded A1 reproduction (see the previous commit) produced no failure, so
no root-cause fix is claimed. Task 1's diagnostics (no discarded errors,
stuck-run report on timeout) are the A4 outcome; the next sighting will name
what was stuck. The 30s deadline is unchanged."
```

---

### Task 4: The merge shard runs its tests in parallel (spec Part B)

**Files:**
- Modify: `internal/app/finalize_rebase_test.go` (`setupRebaseFixtureStatus` → delegates to new `setupRebaseFixtureWithNode`)
- Modify: `internal/app/finalize_merge_test.go` (`setupMergeFixture` → delegates to new `setupMergeFixtureWithNode`)
- Modify: `internal/app/finalize_merge_integration_test.go` (`parallelPlanningNode`, its isolation test, `t.Parallel()` on every test that can take it, fixtures routed through the no-Setenv builder)
- Possibly modify (fallback only): `tests/runtime-budgets.tsv` row `tests/test_go_integration_app_merge.sh`
- Scratch: `${TMPDIR:-/tmp}/docket-0507/measure.sh`

**Interfaces:**
- Consumes (existing): `planningDepsFor(t *testing.T, dir string) realNode` (its `newGitClient` calls `t.Setenv`, which panics under `t.Parallel()`), `realNode{dir string; deps PlanningDeps}`, `PlanningDeps{Client, Engine, Reader, Clock}`, `gitcli.NewClient()`, `transaction.NewEngine(client, clock)`, `NewGitStatusReader(client)`, `testClock()`, `mergePRRef()`, `(*mergeFixture).patchParent`.
- Produces:
  - `setupRebaseFixtureWithNode(t *testing.T, m planRepoMode, status string, nodeFor func(*testing.T, string) realNode) *rebaseFixture`
  - `setupMergeFixtureWithNode(t *testing.T, m planRepoMode, nodeFor func(*testing.T, string) realNode) *mergeFixture`
  - `parallelPlanningNode(t *testing.T, dir string) realNode` (integration tag only)

The spec says none of these tests calls `t.Setenv`. That is true only of the test bodies. Their fixture `setupMergeFixture` → `setupRebaseFixtureStatus` → `planningDepsFor` → `newGitClient` calls `t.Setenv("XDG_CONFIG_HOME", ...)`, so adding `t.Parallel()` alone panics. This task routes the fixture through a builder that isolates the global-config layer once per process, the same trade `e2eNode` makes in `internal/app/finalize_e2e_test.go`.

- [ ] **Step 1: Create the measurement script (if missing) and take the baseline**

```bash
mkdir -p "${TMPDIR:-/tmp}/docket-0507"
[ -x "${TMPDIR:-/tmp}/docket-0507/measure.sh" ] || { cat > "${TMPDIR:-/tmp}/docket-0507/measure.sh" <<'EOF'
#!/usr/bin/env bash
# measure.sh <worktree> <tests/test_x.sh> <label> <warm|1|2|3|summary> — change 0507's
# shared measurement protocol, one run per call (each call stays under 10 minutes).
# Private HOME/TMPDIR/XDG_CONFIG_HOME, DOCKET_GO_TEST_CONCURRENCY and GOMAXPROCS unset,
# quiet-machine check before each run (waits up to 5 minutes, then labels CONTENDED).
set -uo pipefail
wt="$1"; file="$2"; label="$3"; run="$4"
S="${TMPDIR:-/tmp}/docket-0507"; mkdir -p "$S"
state="$S/measure-state.tsv"; notes="$S/measurements.md"; touch "$state"
ncpu="$(sysctl -n hw.ncpu)"
base="$(basename "$file" .sh)"
now(){ perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
load1(){ sysctl -n vm.loadavg | awk '{print $2}'; }
competitors(){ pgrep -fl 'docket development test|go test|\.test( |$)' || true; }
if [ "$run" = summary ]; then
  rows="$(awk -F'\t' -v l="$label" -v f="$file" '$1==l && $2==f && $3!="warm"' "$state")"
  secs="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | paste -sd, -)"
  med="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | awk '{a[NR]=$1} END{print a[int((NR+1)/2)]}')"
  loads="$(awk -F'\t' '{print $6}' <<<"$rows" | paste -sd, -)"
  quiet="$(awk -F'\t' '{print $7}' <<<"$rows" | sort -u | paste -sd, -)"
  rcs="$(awk -F'\t' '{print $5}' <<<"$rows" | paste -sd, -)"
  line="MEASURE label=$label file=$file runs=$secs median=$med loads=$loads quiet=$quiet rc=$rcs ncpu=$ncpu"
  echo "$line"; echo "- $line" >>"$notes"; exit 0
fi
quiet=CONTENDED
for ((i = 0; i < 10; i++)); do
  c="$(competitors)"; l="$(load1)"
  if [ -z "$c" ] && awk -v l="$l" -v n="$ncpu" 'BEGIN{exit !(l < n)}'; then quiet=yes; break; fi
  sleep 30
done
[ "$quiet" = yes ] || { echo "CONTENDED: competitors:"; echo "$c"; }
l="$(load1)"
d="$(mktemp -d "${TMPDIR:-/tmp}/m0507.XXXXXX")"; mkdir -p "$d/home" "$d/tmp" "$d/xdg"
log="$S/run-$label-$base-$run.log"
s="$(now)"
( cd "$wt" && env -u DOCKET_GO_TEST_CONCURRENCY -u GOMAXPROCS \
    HOME="$d/home" TMPDIR="$d/tmp" XDG_CONFIG_HOME="$d/xdg" bash "$file" ) >"$log" 2>&1
rc=$?
e="$(now)"; rm -rf "$d"
secs="$(awk -v s="$s" -v e="$e" 'BEGIN{printf "%.1f", e - s}')"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$file" "$run" "$secs" "$rc" "$l" "$quiet" >>"$state"
echo "RUN label=$label file=$file run=$run secs=$secs rc=$rc load1=$l quiet=$quiet log=$log"
EOF
chmod +x "${TMPDIR:-/tmp}/docket-0507/measure.sh"; }
M="${TMPDIR:-/tmp}/docket-0507/measure.sh"; WT=/Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures
pgrep -x yes || echo "no stray load"
```

Then make four separate foreground calls, each with a Bash timeout of 600000 ms:

```bash
"$M" "$WT" tests/test_go_integration_app_merge.sh B-before warm
"$M" "$WT" tests/test_go_integration_app_merge.sh B-before 1
"$M" "$WT" tests/test_go_integration_app_merge.sh B-before 2
"$M" "$WT" tests/test_go_integration_app_merge.sh B-before 3
"$M" "$WT" tests/test_go_integration_app_merge.sh B-before summary
```

Every run must report `rc=0`. A non-zero rc is a failure to investigate, not a number to record. Expect a median of roughly 41s, as at grooming.

- [ ] **Step 2: Audit for package-level state the tests swap**

```bash
cd "$WT"
grep -n -E -e '^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*(Hook|Seam|Fn|Func|Override)[[:space:]]*=' internal/app/finalize_merge_integration_test.go
grep -n -E -e 'os\.Setenv|t\.Setenv|os\.Chdir|t\.Chdir' internal/app/finalize_merge_integration_test.go
grep -n -E -e '^var ' internal/app/finalize_merge.go internal/app/finalize_merge_integration_test.go
```

For every package-level variable a test in this file assigns, that test stays sequential: no `t.Parallel()`, plus a one-line comment `// Not parallel: swaps <name>.` Record the result of the audit; it goes in the commit message.

- [ ] **Step 3: Extract the injectable fixture builders**

In `internal/app/finalize_rebase_test.go`, rename the body of `setupRebaseFixtureStatus` into the new function and make the old one delegate. The only body change is the node line:

```go
// setupRebaseFixtureStatus builds the fixture with the record at a given
// lifecycle status (the workspace is prepared regardless, so a not-implemented
// precondition can be exercised over a real feature branch).
func setupRebaseFixtureStatus(t *testing.T, m planRepoMode, status string) *rebaseFixture {
	t.Helper()
	return setupRebaseFixtureWithNode(t, m, status, planningDepsFor)
}

// setupRebaseFixtureWithNode is setupRebaseFixtureStatus with the planning-node
// builder chosen by the caller, so a parallel test can pass one that never calls
// t.Setenv (parallelPlanningNode in finalize_merge_integration_test.go).
func setupRebaseFixtureWithNode(t *testing.T, m planRepoMode, status string, nodeFor func(*testing.T, string) realNode) *rebaseFixture {
	t.Helper()
	// ... the previous body of setupRebaseFixtureStatus, unchanged except:
	node := nodeFor(t, repo.invocation)
	// ...
}
```

In `internal/app/finalize_merge_test.go`:

```go
func setupMergeFixture(t *testing.T, m planRepoMode) *mergeFixture {
	t.Helper()
	return setupMergeFixtureWithNode(t, m, planningDepsFor)
}

// setupMergeFixtureWithNode is setupMergeFixture with the planning-node builder
// chosen by the caller (see setupRebaseFixtureWithNode).
func setupMergeFixtureWithNode(t *testing.T, m planRepoMode, nodeFor func(*testing.T, string) realNode) *mergeFixture {
	t.Helper()
	f := setupRebaseFixtureWithNode(t, m, "implemented", nodeFor)
	mf := &mergeFixture{rebaseFixture: f}
	mf.patchParent(t, "implemented", mergePRRef(), "")
	return mf
}
```

Verify that existing callers are unaffected:
`go vet ./internal/app/ && go test -count=1 -run 'Rebase|Merge' ./internal/app/`
Expected: `ok`.

- [ ] **Step 4: Write the failing isolation test for the parallel node**

Append to `internal/app/finalize_merge_integration_test.go`:

```go
// TestIntegrationFinalizeMergeParallelNodeIsolatesGlobalConfig pins that the
// no-Setenv planning node still isolates the in-process global-config layer:
// after it runs, XDG_CONFIG_HOME names the process-wide empty dir it created,
// never the developer's own config home, and that dir carries no docket config.
func TestIntegrationFinalizeMergeParallelNodeIsolatesGlobalConfig(t *testing.T) {
	t.Parallel()
	parallelPlanningNode(t, testsupport.TempDir(t))
	got := os.Getenv("XDG_CONFIG_HOME")
	if got == "" || got != mergeXDGDir {
		t.Fatalf("XDG_CONFIG_HOME = %q, want the isolated dir %q", got, mergeXDGDir)
	}
	if _, err := os.Stat(filepath.Join(got, "docket", "config.yml")); !os.IsNotExist(err) {
		t.Fatalf("isolated global-config dir carries a docket config (stat err %v)", err)
	}
}
```

Run: `go test -tags integration -count=1 -run '^TestIntegrationFinalizeMergeParallelNodeIsolatesGlobalConfig$' ./internal/app/`
Expected: build failure `undefined: parallelPlanningNode` / `mergeXDGDir`.

- [ ] **Step 5: Implement `parallelPlanningNode`**

Add to `internal/app/finalize_merge_integration_test.go` (imports: `os`, `path/filepath`, `sync`, `gitcli`, `transaction`, `testsupport` as needed):

```go
var (
	mergeXDGOnce sync.Once
	mergeXDGDir  string
	mergeXDGErr  error
)

// parallelPlanningNode is planningDepsFor without t.Setenv, so the
// TestIntegrationFinalizeMerge* tests can call t.Parallel(). It isolates the
// global-config layer ONCE per test process with os.Setenv, the trade e2eNode
// makes in finalize_e2e_test.go. That is safe because this file is behind the
// `integration` build tag and the merge shard (tests/test_go_integration_app_merge.sh)
// runs only ^TestIntegrationFinalizeMerge in its process; Go runs every
// sequential top-level test before releasing parallel ones, so no t.Setenv
// elsewhere overlaps it.
func parallelPlanningNode(t *testing.T, dir string) realNode {
	t.Helper()
	mergeXDGOnce.Do(func() {
		// tempdir-exempt: one empty global-config dir per test process, shared by every parallel merge test (the e2eNode precedent).
		mergeXDGDir, mergeXDGErr = os.MkdirTemp("", "docket-merge-xdg-*")
		if mergeXDGErr == nil {
			mergeXDGErr = os.Setenv("XDG_CONFIG_HOME", mergeXDGDir)
		}
	})
	if mergeXDGErr != nil {
		t.Fatalf("isolate global config: %v", mergeXDGErr)
	}
	client, err := gitcli.NewClient()
	if err != nil {
		t.Fatalf("gitcli.NewClient: %v", err)
	}
	engine, err := transaction.NewEngine(client, testClock())
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	return realNode{
		dir: dir,
		deps: PlanningDeps{
			Client: client,
			Engine: engine,
			Reader: NewGitStatusReader(client),
			Clock:  testClock(),
		},
	}
}
```

Run the Step 4 test again. Expected: `ok`.

Mutation (restore from a backup copy): delete the `os.Setenv` line, run the test, and expect it to fail with `XDG_CONFIG_HOME = ...`. Then `cp` the backup back.

- [ ] **Step 6: Parallelize the shard's tests**

In `internal/app/finalize_merge_integration_test.go`:
- Make `t.Parallel()` the first statement of every `TestIntegrationFinalizeMerge*` function the Step 2 audit cleared.
- Make `t.Parallel()` the first statement of every `t.Run` closure in those tests that builds its own fixture.
- Replace every `setupMergeFixture(t, m)` call with `setupMergeFixtureWithNode(t, m, parallelPlanningNode)`. Replace every `setupRebaseFixture(t, m)` / `setupRebaseFixtureStatus(t, m, s)` call with `setupRebaseFixtureWithNode(t, m, "implemented" /* or s */, parallelPlanningNode)`. Replace any direct `planningDepsFor(t, d)` with `parallelPlanningNode(t, d)`.

Go 1.26 loop variables are per-iteration, so no capture copies are needed. Then run:

```bash
go test -tags integration -count=1 -run '^TestIntegrationFinalizeMerge' -v ./internal/app/ 2>&1 | tail -n 30
go test -tags integration -race -count=1 -run '^TestIntegrationFinalizeMerge' ./internal/app/
```

Expected: both pass, and the race run reports no `DATA RACE`.

If a run panics with `test using t.Setenv ... can not use t.Parallel`, trace the panic's stack to the helper that reached `t.Setenv`. Route that helper through `nodeFor` (or an equivalent parameter) the same way. Never remove the isolation and never keep the call by dropping `t.Parallel()` silently. If a data race names shared state, make that test sequential with the `// Not parallel:` comment.

Also run the other shards that share the rebase fixture, to prove the refactor:

`go test -tags integration -count=1 -run '^TestIntegrationFinalizeRebase|^TestIntegrationEvidence|^TestIntegrationFinalizeState|^TestIntegrationFinalizeOps' ./internal/app/`

Expected: `ok`.

- [ ] **Step 7: Run the shard file itself and its contract**

`bash tests/test_go_integration_app_merge.sh && bash tests/test_go_integration_contract.sh`
Expected: every line `ok - `, exit 0. The declared count is now 13 (the new isolation test), and every selected test ran.

- [ ] **Step 8: Measure after, with the same protocol**

Make four separate foreground calls (600000 ms timeout each), then the summary:

```bash
"$M" "$WT" tests/test_go_integration_app_merge.sh B-after warm
"$M" "$WT" tests/test_go_integration_app_merge.sh B-after 1
"$M" "$WT" tests/test_go_integration_app_merge.sh B-after 2
"$M" "$WT" tests/test_go_integration_app_merge.sh B-after 3
"$M" "$WT" tests/test_go_integration_app_merge.sh B-after summary
```

If the median is above 25s: look for redundant work that is identical across tests in the shard (for example, fixture setup that is rebuilt the same way in each test). Cut it only where behavior is unchanged, then re-measure. If it still cannot reach 25s, apply the fallback. Set the `tests/test_go_integration_app_merge.sh` row in `tests/runtime-budgets.tsv` to `ceil5(post-cut median) + 5`, where `ceil5` rounds up to the next multiple of 5, and keep the `parallel` column.

- [ ] **Step 9: Commit with the numbers**

```bash
git add internal/app/finalize_rebase_test.go internal/app/finalize_merge_test.go internal/app/finalize_merge_integration_test.go
# plus tests/runtime-budgets.tsv only under the fallback
git commit -F - <<EOF
test(app): run the finalize merge integration shard in parallel

TestIntegrationFinalizeMerge* now call t.Parallel(); the shared fixture takes
an injected planning-node builder (setupRebaseFixtureWithNode /
setupMergeFixtureWithNode) and the shard uses parallelPlanningNode, which
isolates the global-config layer once per process instead of t.Setenv.
Package-state audit: <result of Step 2>. -race run: clean.

Quiet solo measurements (shared protocol):
$(grep -e 'label=B-before' -e 'label=B-after' "${TMPDIR:-/tmp}/docket-0507/measurements.md")
<if fallback: what was cut, what remains, why it cannot be cut, new row value>
EOF
```

---

### Task 5: Cut redundant work from the finalize e2e file (spec Part C)

**Files:**
- Modify (only for measured wins): `tests/test_go_finalize_e2e.sh`, `internal/app/finalize_e2e_test.go`
- Possibly modify (fallback only): `tests/runtime-budgets.tsv` row `tests/test_go_finalize_e2e.sh`
- Scratch: `${TMPDIR:-/tmp}/docket-0507/measure.sh`, `${TMPDIR:-/tmp}/docket-0507/e2e-phases.sh`

**Interfaces:**
- Consumes: `measure.sh` (same contract as in Task 4; create it from the block below if missing).
- Produces: the measured cut, plus `C-before` and `C-after` lines in `measurements.md`.

- [ ] **Step 1: Ensure the measurement script exists and take the baseline**

If `${TMPDIR:-/tmp}/docket-0507/measure.sh` is missing, create it with exactly this content:

```bash
mkdir -p "${TMPDIR:-/tmp}/docket-0507"
[ -x "${TMPDIR:-/tmp}/docket-0507/measure.sh" ] || { cat > "${TMPDIR:-/tmp}/docket-0507/measure.sh" <<'EOF'
#!/usr/bin/env bash
# measure.sh <worktree> <tests/test_x.sh> <label> <warm|1|2|3|summary> — change 0507's
# shared measurement protocol, one run per call (each call stays under 10 minutes).
# Private HOME/TMPDIR/XDG_CONFIG_HOME, DOCKET_GO_TEST_CONCURRENCY and GOMAXPROCS unset,
# quiet-machine check before each run (waits up to 5 minutes, then labels CONTENDED).
set -uo pipefail
wt="$1"; file="$2"; label="$3"; run="$4"
S="${TMPDIR:-/tmp}/docket-0507"; mkdir -p "$S"
state="$S/measure-state.tsv"; notes="$S/measurements.md"; touch "$state"
ncpu="$(sysctl -n hw.ncpu)"
base="$(basename "$file" .sh)"
now(){ perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
load1(){ sysctl -n vm.loadavg | awk '{print $2}'; }
competitors(){ pgrep -fl 'docket development test|go test|\.test( |$)' || true; }
if [ "$run" = summary ]; then
  rows="$(awk -F'\t' -v l="$label" -v f="$file" '$1==l && $2==f && $3!="warm"' "$state")"
  secs="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | paste -sd, -)"
  med="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | awk '{a[NR]=$1} END{print a[int((NR+1)/2)]}')"
  loads="$(awk -F'\t' '{print $6}' <<<"$rows" | paste -sd, -)"
  quiet="$(awk -F'\t' '{print $7}' <<<"$rows" | sort -u | paste -sd, -)"
  rcs="$(awk -F'\t' '{print $5}' <<<"$rows" | paste -sd, -)"
  line="MEASURE label=$label file=$file runs=$secs median=$med loads=$loads quiet=$quiet rc=$rcs ncpu=$ncpu"
  echo "$line"; echo "- $line" >>"$notes"; exit 0
fi
quiet=CONTENDED
for ((i = 0; i < 10; i++)); do
  c="$(competitors)"; l="$(load1)"
  if [ -z "$c" ] && awk -v l="$l" -v n="$ncpu" 'BEGIN{exit !(l < n)}'; then quiet=yes; break; fi
  sleep 30
done
[ "$quiet" = yes ] || { echo "CONTENDED: competitors:"; echo "$c"; }
l="$(load1)"
d="$(mktemp -d "${TMPDIR:-/tmp}/m0507.XXXXXX")"; mkdir -p "$d/home" "$d/tmp" "$d/xdg"
log="$S/run-$label-$base-$run.log"
s="$(now)"
( cd "$wt" && env -u DOCKET_GO_TEST_CONCURRENCY -u GOMAXPROCS \
    HOME="$d/home" TMPDIR="$d/tmp" XDG_CONFIG_HOME="$d/xdg" bash "$file" ) >"$log" 2>&1
rc=$?
e="$(now)"; rm -rf "$d"
secs="$(awk -v s="$s" -v e="$e" 'BEGIN{printf "%.1f", e - s}')"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$file" "$run" "$secs" "$rc" "$l" "$quiet" >>"$state"
echo "RUN label=$label file=$file run=$run secs=$secs rc=$rc load1=$l quiet=$quiet log=$log"
EOF
chmod +x "${TMPDIR:-/tmp}/docket-0507/measure.sh"; }
M="${TMPDIR:-/tmp}/docket-0507/measure.sh"; WT=/Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures
```

Then make separate foreground calls (600000 ms each):

```bash
"$M" "$WT" tests/test_go_finalize_e2e.sh C-before warm
"$M" "$WT" tests/test_go_finalize_e2e.sh C-before 1
"$M" "$WT" tests/test_go_finalize_e2e.sh C-before 2
"$M" "$WT" tests/test_go_finalize_e2e.sh C-before 3
"$M" "$WT" tests/test_go_finalize_e2e.sh C-before summary
```

Expect a median of roughly 36s.

- [ ] **Step 2: Time each phase separately**

```bash
cat > "${TMPDIR:-/tmp}/docket-0507/e2e-phases.sh" <<'EOF'
#!/usr/bin/env bash
# e2e-phases.sh <worktree> — time each phase of tests/test_go_finalize_e2e.sh on its own.
set -uo pipefail
wt="$1"; S="${TMPDIR:-/tmp}/docket-0507"; mkdir -p "$S"
cd "$wt" || exit 1
common="$(git rev-parse --git-common-dir)"; case "$common" in /*) ;; *) common="$wt/$common" ;; esac
export GOFLAGS="-modcacherw" GOMODCACHE="$common/docket-go-cache/mod" GOCACHE="$common/docket-go-cache/build"
unset DOCKET_GO_TEST_CONCURRENCY GOMAXPROCS
now(){ perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
phase(){ local name="$1"; shift; local s e rc; s="$(now)"; "$@" >"$S/phase-$name.log" 2>&1; rc=$?; e="$(now)"
  awk -v s="$s" -v e="$e" -v n="$name" -v rc="$rc" 'BEGIN{printf "PHASE %-14s %6.1fs rc=%d\n", n, e - s, rc}'; }
phase vet        go vet -tags e2e ./internal/app/
phase testcompile go test -tags e2e -c -o "$S/app-e2e.test" ./internal/app/
phase docketbuild go build -o "$S/docket-phase" ./cmd/docket
( cd internal/app && phase testrun "$S/app-e2e.test" -test.run TestE2E -test.count=1 -test.v )
slow="$(grep -E -e '^--- (PASS|FAIL): TestE2E' "$S/phase-testrun.log" | sort -t'(' -k2 -rn)"
printf 'Per-test (slowest first):\n%s\n' "$slow"
EOF
chmod +x "${TMPDIR:-/tmp}/docket-0507/e2e-phases.sh"
"${TMPDIR:-/tmp}/docket-0507/e2e-phases.sh" "$WT"   # run twice; record the second (warm) reading
"${TMPDIR:-/tmp}/docket-0507/e2e-phases.sh" "$WT" | tee -a "${TMPDIR:-/tmp}/docket-0507/measurements.md"
```

- [ ] **Step 3: Choose the cuts from the phase numbers**

Evaluate these levers in this order. Keep each one only if a re-measure shows a win:

1. **`-vet=off` on the matrix run.** The wrapper's full `go vet -tags e2e ./internal/app/` has just vetted this exact package. The `go test` that follows repeats a subset of the same checks on the same code. In `tests/test_go_finalize_e2e.sh`, change the test line to:
   ```bash
   test_out="$(go test -tags e2e -vet=off $go_conc_args -run "TestE2E" -count=1 -v ./internal/app/ 2>&1)"
   ```
   Add a comment above it: `# -vet=off: the full go vet -tags e2e above already vetted this package; go test's built-in vet subset would repeat it.` Leave the vet line and its assert unchanged.
2. **The fake gh build.** `buildE2EBinaries` writes the fake gh module to a fresh temp dir in every process. If the phase log or a `go build -x` shows that build recompiling every run and costing 2s or more, pursue a behavior-neutral fix and measure it. Otherwise leave it alone.
3. **Per-test redundant setup.** Using the slowest-first list, read the three slowest `TestE2E*` tests. Look for fixture work they repeat identically that could be built once, like `sharedBinOnce`. Cut it only when every assert is unchanged.

Do not run phases concurrently, for example vet alongside the test. That would push this file past its ADR-0108 share of the machine under the suite runner.

- [ ] **Step 4: Apply the chosen cuts and verify the file still passes**

`bash tests/test_go_finalize_e2e.sh`
Expected: every line `ok - `, exit 0, including `every declared top-level TestE2E test actually ran and passed`.

- [ ] **Step 5: Measure after**

```bash
"$M" "$WT" tests/test_go_finalize_e2e.sh C-after warm
"$M" "$WT" tests/test_go_finalize_e2e.sh C-after 1
"$M" "$WT" tests/test_go_finalize_e2e.sh C-after 2
"$M" "$WT" tests/test_go_finalize_e2e.sh C-after 3
"$M" "$WT" tests/test_go_finalize_e2e.sh C-after summary
```

Make each a separate foreground call. If the median is above 25s after every measured lever, apply the fallback. Set the `tests/test_go_finalize_e2e.sh` row in `tests/runtime-budgets.tsv` to `ceil5(post-cut median) + 5`, keep `parallel`, and argue it in the commit message.

- [ ] **Step 6: Mutation-check that the completeness assert still bites (restore from a backup copy)**

```bash
B="$(mktemp "${TMPDIR:-/tmp}/finalize_e2e_test.go.XXXXXX")"; cp internal/app/finalize_e2e_test.go "$B"
perl -0pi -e 's/^func TestE2EStack\(/func testE2EStackRenamed(/m' internal/app/finalize_e2e_test.go
bash tests/test_go_finalize_e2e.sh; echo "rc=$? (want non-zero: a NOT OK completeness line, or a vet unused-function failure)"
cp "$B" internal/app/finalize_e2e_test.go
git diff --stat
```

- [ ] **Step 7: Commit with the numbers**

```bash
git add tests/test_go_finalize_e2e.sh   # plus internal/app/finalize_e2e_test.go / tests/runtime-budgets.tsv only if changed
git commit -F - <<EOF
test(e2e): cut redundant work from the finalize e2e file

<each kept lever, one line, with its measured saving; each rejected lever with its measurement>

Phase timings before: <PHASE lines from Step 2>
Quiet solo measurements (shared protocol):
$(grep -e 'label=C-before' -e 'label=C-after' "${TMPDIR:-/tmp}/docket-0507/measurements.md")
<if fallback: what was cut, what remains, why it cannot be cut, new row value>
EOF
```

---

### Task 6: Record the race file's quiet measurement and the full measurement record (spec Part D)

No repository file changes. The record goes in an empty commit.

**Files:**
- Scratch: `${TMPDIR:-/tmp}/docket-0507/measure.sh`, `measurements.md`

**Interfaces:**
- Consumes: `measure.sh` (create it from the block below if missing). Also `measurements.md` as filled by Tasks 2 to 5.
- Produces: one empty commit whose message is the complete measurement record, so the results file can cite it.

- [ ] **Step 1: Ensure the measurement script exists**

If `${TMPDIR:-/tmp}/docket-0507/measure.sh` is missing, create it with exactly this content:

```bash
mkdir -p "${TMPDIR:-/tmp}/docket-0507"
[ -x "${TMPDIR:-/tmp}/docket-0507/measure.sh" ] || { cat > "${TMPDIR:-/tmp}/docket-0507/measure.sh" <<'EOF'
#!/usr/bin/env bash
# measure.sh <worktree> <tests/test_x.sh> <label> <warm|1|2|3|summary> — change 0507's
# shared measurement protocol, one run per call (each call stays under 10 minutes).
# Private HOME/TMPDIR/XDG_CONFIG_HOME, DOCKET_GO_TEST_CONCURRENCY and GOMAXPROCS unset,
# quiet-machine check before each run (waits up to 5 minutes, then labels CONTENDED).
set -uo pipefail
wt="$1"; file="$2"; label="$3"; run="$4"
S="${TMPDIR:-/tmp}/docket-0507"; mkdir -p "$S"
state="$S/measure-state.tsv"; notes="$S/measurements.md"; touch "$state"
ncpu="$(sysctl -n hw.ncpu)"
base="$(basename "$file" .sh)"
now(){ perl -MTime::HiRes=time -e 'printf "%.2f\n", time'; }
load1(){ sysctl -n vm.loadavg | awk '{print $2}'; }
competitors(){ pgrep -fl 'docket development test|go test|\.test( |$)' || true; }
if [ "$run" = summary ]; then
  rows="$(awk -F'\t' -v l="$label" -v f="$file" '$1==l && $2==f && $3!="warm"' "$state")"
  secs="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | paste -sd, -)"
  med="$(awk -F'\t' '{print $4}' <<<"$rows" | sort -n | awk '{a[NR]=$1} END{print a[int((NR+1)/2)]}')"
  loads="$(awk -F'\t' '{print $6}' <<<"$rows" | paste -sd, -)"
  quiet="$(awk -F'\t' '{print $7}' <<<"$rows" | sort -u | paste -sd, -)"
  rcs="$(awk -F'\t' '{print $5}' <<<"$rows" | paste -sd, -)"
  line="MEASURE label=$label file=$file runs=$secs median=$med loads=$loads quiet=$quiet rc=$rcs ncpu=$ncpu"
  echo "$line"; echo "- $line" >>"$notes"; exit 0
fi
quiet=CONTENDED
for ((i = 0; i < 10; i++)); do
  c="$(competitors)"; l="$(load1)"
  if [ -z "$c" ] && awk -v l="$l" -v n="$ncpu" 'BEGIN{exit !(l < n)}'; then quiet=yes; break; fi
  sleep 30
done
[ "$quiet" = yes ] || { echo "CONTENDED: competitors:"; echo "$c"; }
l="$(load1)"
d="$(mktemp -d "${TMPDIR:-/tmp}/m0507.XXXXXX")"; mkdir -p "$d/home" "$d/tmp" "$d/xdg"
log="$S/run-$label-$base-$run.log"
s="$(now)"
( cd "$wt" && env -u DOCKET_GO_TEST_CONCURRENCY -u GOMAXPROCS \
    HOME="$d/home" TMPDIR="$d/tmp" XDG_CONFIG_HOME="$d/xdg" bash "$file" ) >"$log" 2>&1
rc=$?
e="$(now)"; rm -rf "$d"
secs="$(awk -v s="$s" -v e="$e" 'BEGIN{printf "%.1f", e - s}')"
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$label" "$file" "$run" "$secs" "$rc" "$l" "$quiet" >>"$state"
echo "RUN label=$label file=$file run=$run secs=$secs rc=$rc load1=$l quiet=$quiet log=$log"
EOF
chmod +x "${TMPDIR:-/tmp}/docket-0507/measure.sh"; }
M="${TMPDIR:-/tmp}/docket-0507/measure.sh"; WT=/Users/homer/dev/docket/.worktrees/flaky-tests-track-and-stabilize-intermittent-suite-failures
```

- [ ] **Step 2: Measure `tests/test_go_race.sh`**

Make each a separate foreground call (600000 ms):

```bash
"$M" "$WT" tests/test_go_race.sh D warm
"$M" "$WT" tests/test_go_race.sh D 1
"$M" "$WT" tests/test_go_race.sh D 2
"$M" "$WT" tests/test_go_race.sh D 3
"$M" "$WT" tests/test_go_race.sh D summary
```

Expect a median of roughly 54s against the 60s row. No code change follows from this number, whatever it is. If it is over 60s, say so in the commit message as a finding for a separate change.

- [ ] **Step 3: Commit the full measurement record (empty commit)**

```bash
git commit --allow-empty -F - <<EOF
test: record change 0507's quiet solo measurements

tests/test_go_race.sh needs no code work; its quiet median puts the earlier
117s (change 0517) and 94s (PR 361) readings on record as contended
measurements, not growth.

$(cat "${TMPDIR:-/tmp}/docket-0507/measurements.md")
EOF
```

After the build gate runs the full suite, the results file must address every `BUDGET WATCH:`, `PARALLEL-SENSITIVE:` and `SERIAL CONFIRMED OVER BUDGET:` line in its budget report, citing these numbers (spec acceptance criteria).
