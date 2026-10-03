<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0492 — Suite teardown can outlive its supervisor](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0492-suite-teardown-can-outlive-its-supervisor.md)**
<!-- docket:backlink:end -->
# Suite teardown can outlive its supervisor — Implementation Plan

> **For agentic workers:** this plan is executed by **docket-build**: one tier-routed worker per
> task, strictly sequential in the feature worktree, one commit per task, no per-task review, and a
> single full-suite gate at the end. Each task names its **Risk** (the routing tier). Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a gate's supervisor dies alone (SIGKILL or a crash) and its suite keeps running,
`run.cancel`, the death guardian, and `run.verdict`'s success closeout report an informational
`tree-survives:<drive>:<pgid>` finding instead of `run-terminal:<drive>`, without changing any
disposition or verdict and without ever signalling the group.

**Architecture:** (1) `internal/process` gains a read-only `Service.ProbeLeftover(runDir)` that,
for a run whose supervisor has exited, answers **none** (the recorded group is empty),
**leftover** (the group still has members and its leader pid is gone), or **unclear** (anything
else). It shares `Observe`'s manifest validation through one extracted helper. (2)
`internal/gatedrive`'s `ProcessSeam` gains the method, and the launch census's `supervisorGone`
calls it only in its "observed exit" branch: a leftover is still torn down, but its finding becomes
`tree-survives:<drive>:<pgid>`, which outranks `replacement-stopped` and `run-terminal` in
`proveRunDirsGone`. (3) The app layer needs no code change: accounted census findings already
surface in `run.cancel`'s `findings` and the closeout's `completion_findings`. Tests pin that.
(4) Glossary, concept doc, and comments follow.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `go test` (with `-tags integration`
for the real-git/real-process app tests), the Go suite runner (`go run ./cmd/docket development
test`, run by docket-build at the gate, not by any task).

**Spec:** `docs/superpowers/specs/2026-10-02-suite-teardown-can-outlive-its-supervisor-design.md`
(on the `docket` metadata branch; the synchronized copy is under `.docket/` in the primary
checkout). Executors read the spec beside this plan. Change file:
`docs/changes/active/0492-suite-teardown-can-outlive-its-supervisor.md`.

## Global Constraints

- **Docket never signals on this evidence.** `ProbeLeftover` never calls `signalGroup`,
  `syscall.Kill` with a non-zero signal, or any writer. The census never stops a leftover.
- **No new halt and no new block.** A **leftover** never changes a census's settled result
  (`supervisorGone` still returns `true`), a cancel disposition, or a closeout verdict. **None**
  and **unclear** (including a probe error) keep exactly today's `run-terminal:<drive>`.
- **Callers act only on `LeftoverPresent`.** An unclear answer or a non-nil error is never read
  as leftover and never as none-with-a-different-finding. It is today's behavior.
- **`internal/process` stays standard-library-only** (`TestImportBoundaryStdlibOnly`).
- **The finding is credential-free:** exactly `tree-survives:<drive id>:<decimal pgid>`. No path,
  argv, env value, or token.
- **Unchanged:** `Observe`'s states and decision order, `Stop`, `identityConditions` (ADR-0095),
  `classifyRun` and its abandoned-marker rule, `gate.observe`/`gate.recover`/`gate.cleanup`, the
  drive slice including the relaunch branch and `proveNoTreeSurvives` (change 0493's), the worktree
  lock, the cancel disposition vocabulary, the closeout verdict, and resume admission.
- **ADR work is out of scope for every build task.** The new ADR and the dated Update note on
  ADR-0132 are recorded by the coordinator through docket-adr after the build. No task creates or
  edits anything under `docs/adrs/`.
- **Frozen records are never edited:** `docs/superpowers/plans/**` (except this plan),
  `docs/superpowers/specs/**`, `docs/results/**`, `docs/changes/**`, and Accepted ADR bodies.
- **Comment anchors** name a symbol or quote a clause, never a line number (ADR-0054).
- **Do not write the hyphenated run-id spelling** in new prose or comments; write "run id" (the
  0491 retired-vocabulary seal owns the hyphenated tokens).
- **Every test run that observes a change in outcome defeats the cache:** `go test -count=1 …`.
- **Mutation restore uses a backup copy**, never `git checkout --`:
  `cp <file> <file>.bak` → apply the mutation → run the test → `mv -f <file>.bak <file>`.
  Confirm the mutated run actually executed (no `(cached)`), and confirm the restored run is green.
- **Seam-swapping tests never call `t.Parallel()`**: `leftoverGroupProbe` and
  `leftoverLeaderProbe` are package globals, exactly like `recoverGroupProbe`.
- **App integration tests keep their shard prefixes:** a run.cancel test name starts with
  `TestIntegrationRunCancel`, a closeout test name with `TestIntegrationRunCompletion`
  (`tests/test_go_integration_app_runcancel.sh`, `tests/test_go_integration_app_runcompletion.sh`).
- **Tasks run focused tests only.** The whole suite runs once at docket-build's gate through
  `build.test_command`; read every `BUDGET WATCH:` and `SERIAL CONFIRMED OVER BUDGET:` line there.

## Review Focus

1. **A HALTED drive is the realistic path.** After a supervisor is killed, the drive's slice
   HALTs it, so the census reaches `supervisorGone` through `reconcileHaltedDrive`, not the
   nonterminal branch. Expect the same `tree-survives` finding there. Pinned in Task 2
   (`halted` subtests) and end to end in Task 3.
2. **A probe error carrying a "leftover" answer.** The census must trust the answer only when
   the error is nil. Pinned in Task 2 (`error` row returns `LeftoverPresent` with an error and
   expects `run-terminal`).
3. **A live.lock that is missing, held by someone else, or not a file.** Missing reads as free
   (as `Observe` treats it) and proceeds to the group probe; held or unprovable is unclear.
   Pinned in Task 1 (`TestProbeLeftoverNeedsAFreeLiveLock`).
4. **Both recorded run dirs leftover.** The drive reports one finding: the first dir's
   (`RawRunDir`) group. Pinned in Task 2 (`both-leftover` subtest).
5. **A census stop followed by a leftover probe.** The `replacement-stopped` branch must not
   probe at all: `Stop` already waited for the group to empty. Pinned in Task 2 (the probed-dirs
   assertion in `prior-leftover`).

---

### Task 1: `process.Service.ProbeLeftover`, a read-only leftover check

**Risk:** premium — the three-way answer is the decision. Collapsing an unknown probe into
"leftover" or "none" is exactly the defect class (probe-error-is-not-clean-absence), and the
real-process tests depend on zombie and reaping behavior.

**Files:**
- Modify: `internal/process/observe.go` (extract `validatedManifest`; `Observe` calls it)
- Create: `internal/process/leftover.go`
- Create: `internal/process/leftover_test.go`

**Interfaces:**
- Consumes: `readManifest`, `resolveRunDir`, `probeFlock`, `groupAlive`, `processAlive`,
  `failf`, `liveLockFile`, `manifestFile`, `writeAtomicJSON` (tests), and the test helpers
  `newTestService`, `launchHelper`, `helperArgv`, `waitFor`, `waitTerminalState`, `quiesceRun`,
  `signalGroup`, `TryExclusiveLock`.
- Produces (Task 2 relies on these exact names):
  ```go
  type LeftoverAnswer string
  const (
      LeftoverNone    LeftoverAnswer = "none"
      LeftoverPresent LeftoverAnswer = "leftover"
      LeftoverUnclear LeftoverAnswer = "unclear"
  )
  type Leftover struct {
      Answer LeftoverAnswer
      PGID   int
  }
  func (s *Service) ProbeLeftover(runDir string) (Leftover, error)
  ```
  Package-private seams: `var leftoverGroupProbe = groupAlive`, `var leftoverLeaderProbe = processAlive`.
  Package-private helper: `func validatedManifest(op, runDir string) (*manifestRecord, error)`.

- [ ] **Step 1: Write the failing tests**

Create `internal/process/leftover_test.go`:

```go
package process

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// The leftover check (change 0492). ProbeLeftover answers, read-only, whether a
// run whose supervisor has exited still has members in its recorded process
// group. Real-process tests pin the leftover, none, and zombie answers; seam
// tests drive every unclear branch deterministically on every platform. The
// seam-swapping tests never run in parallel: the seams are package globals.

// withLeftoverProbes swaps the group and leader probes for one test and
// restores them at cleanup.
func withLeftoverProbes(t *testing.T, group, leader probeAnswer) {
	t.Helper()
	prevGroup, prevLeader := leftoverGroupProbe, leftoverLeaderProbe
	leftoverGroupProbe = func(int) probeAnswer { return group }
	leftoverLeaderProbe = func(int) probeAnswer { return leader }
	t.Cleanup(func() { leftoverGroupProbe, leftoverLeaderProbe = prevGroup, prevLeader })
}

// quiescedRun launches a fast-exit run and waits until its supervisor is gone,
// returning a valid run dir with a free live.lock whose manifest records
// pgid == supervisor_pid — the shape ProbeLeftover's callers hand it.
func quiescedRun(t *testing.T) (*Service, string, manifestRecord) {
	t.Helper()
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "exit", "0")
	waitTerminalState(t, out.RunDir, false)
	quiesceRun(t, out.RunDir)
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.PGID <= 1 || m.PGID != m.SupervisorPID {
		t.Fatalf("precondition: an established manifest with pgid == supervisor_pid, got %+v (err %v)", m, err)
	}
	return svc, out.RunDir, *m
}

// writeManifestCopy writes m (a copy of the run's manifest, mutated by the
// caller) back over the run's manifest.json.
func writeManifestCopy(t *testing.T, runDir string, m manifestRecord) {
	t.Helper()
	if err := writeAtomicJSON(filepath.Join(runDir, manifestFile), &m); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
}

func mustProbeLeftover(t *testing.T, svc *Service, runDir string) Leftover {
	t.Helper()
	got, err := svc.ProbeLeftover(runDir)
	if err != nil {
		t.Fatalf("ProbeLeftover: %v", err)
	}
	return got
}

// TestProbeLeftoverReportsSuiteOfKilledSupervisor: a supervisor SIGKILLed alone,
// and reaped (launchHelper's reapSupervisor plays init), leaves its command
// running in the supervisor's group: leftover, naming that group. Once the
// command is gone too, the group is empty: none.
func TestProbeLeftoverReportsSuiteOfKilledSupervisor(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, testsupport.TempDir(t), "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 || m.PGID != m.SupervisorPID {
		t.Fatalf("manifest: %+v (err %v)", m, err)
	}
	t.Cleanup(func() { _ = signalGroup(m.PGID, syscall.SIGKILL) })
	if err := syscall.Kill(m.SupervisorPID, syscall.SIGKILL); err != nil {
		t.Fatalf("kill the supervisor: %v", err)
	}
	waitFor(t, "the killed supervisor to be reaped", 30*time.Second, func() bool {
		return processAlive(m.SupervisorPID) == probeAbsent
	})

	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverPresent || got.PGID != m.PGID {
		t.Fatalf("a reaped supervisor's still-running command: got %+v, want leftover with pgid %d", got, m.PGID)
	}

	if err := signalGroup(m.PGID, syscall.SIGKILL); err != nil {
		t.Fatalf("end the orphaned command: %v", err)
	}
	waitFor(t, "the orphaned command to be reaped", 30*time.Second, func() bool {
		return groupAlive(m.PGID) == probeAbsent
	})
	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverNone {
		t.Fatalf("an empty group: got %+v, want none", got)
	}
}

// TestProbeLeftoverZombieSupervisorIsUnclear pins spec fact 8: a dead supervisor
// whose launching process (this test) has not reaped it is a zombie whose pid
// still answers kill(pid, 0), so the check cannot prove the leader gone and
// answers unclear — never leftover. The group probe may read live (the command)
// or unknown (EPERM); both must land on unclear.
func TestProbeLeftoverZombieSupervisorIsUnclear(t *testing.T) {
	svc := newTestService(t)
	// svc.Launch directly, NOT launchHelper: no reapSupervisor, so the killed
	// supervisor stays a zombie child of this process.
	out, err := svc.Launch(LaunchRequest{Root: testsupport.TempDir(t), Cwd: testsupport.TempDir(t), Argv: helperArgv(t, "sleep")})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	m, err := readManifest(out.RunDir)
	if err != nil || m == nil || m.SupervisorPID <= 1 {
		t.Fatalf("manifest: %+v (err %v)", m, err)
	}
	pid := m.SupervisorPID
	t.Cleanup(func() {
		_ = signalGroup(m.PGID, syscall.SIGKILL)
		// Reap exactly this zombie (never -1, so no other test's child is stolen).
		var ws syscall.WaitStatus
		for {
			wpid, werr := syscall.Wait4(pid, &ws, 0, nil)
			if werr != syscall.EINTR || wpid == pid {
				return
			}
		}
	})
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatalf("kill the supervisor: %v", err)
	}
	// The kernel closes a dying process's descriptors before it becomes a zombie,
	// so a free live.lock proves the supervisor has exited.
	waitFor(t, "live lock release", 30*time.Second, func() bool {
		held, ans := probeFlock(filepath.Join(out.RunDir, liveLockFile))
		return !held && ans == probeAbsent
	})
	if got := processAlive(pid); got == probeAbsent {
		t.Fatalf("precondition (spec fact 8): an unreaped supervisor must still answer kill(pid, 0), got %v", got)
	}

	if got := mustProbeLeftover(t, svc, out.RunDir); got.Answer != LeftoverUnclear {
		t.Fatalf("a zombie supervisor: got %+v, want unclear", got)
	}
}

// TestProbeLeftoverGroupAndLeaderAnswers drives the group and leader probes
// through every combination. Only a live group whose leader is provably absent
// is leftover; only an absent group is none; everything else is unclear.
func TestProbeLeftoverGroupAndLeaderAnswers(t *testing.T) {
	svc, runDir, m := quiescedRun(t)
	for _, tc := range []struct {
		name          string
		group, leader probeAnswer
		want          LeftoverAnswer
	}{
		{"group-absent", probeAbsent, probeLive, LeftoverNone},
		{"group-unknown", probeUnknown, probeAbsent, LeftoverUnclear},
		{"leader-absent", probeLive, probeAbsent, LeftoverPresent},
		{"leader-live", probeLive, probeLive, LeftoverUnclear},
		{"leader-unknown", probeLive, probeUnknown, LeftoverUnclear},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withLeftoverProbes(t, tc.group, tc.leader)
			got := mustProbeLeftover(t, svc, runDir)
			if got.Answer != tc.want || got.PGID != m.PGID {
				t.Fatalf("group %v, leader %v: got %+v, want %s with pgid %d", tc.group, tc.leader, got, tc.want, m.PGID)
			}
		})
	}
}

// TestProbeLeftoverRefusesUnaddressableGroup: a recorded pgid <= 1, or one that
// is not the supervisor's own pid, names no group the run owned — unclear, even
// when the probes would otherwise read leftover.
func TestProbeLeftoverRefusesUnaddressableGroup(t *testing.T) {
	svc, runDir, orig := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover
	for _, tc := range []struct {
		name string
		pgid int
	}{
		{"pgid-zero", 0},
		{"pgid-one", 1},
		{"pgid-not-supervisor", orig.SupervisorPID + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := orig
			m.PGID = tc.pgid
			writeManifestCopy(t, runDir, m)
			if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
				t.Fatalf("pgid %d (supervisor_pid %d): got %+v, want unclear", tc.pgid, m.SupervisorPID, got)
			}
		})
	}
}

// TestProbeLeftoverNeedsAFreeLiveLock: the check means something only once the
// supervisor is gone. A held live.lock and an unprovable one are unclear; a
// missing live.lock reads as free, as Observe reads it, so the group probe
// decides.
func TestProbeLeftoverNeedsAFreeLiveLock(t *testing.T) {
	svc, runDir, _ := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover
	lockPath := filepath.Join(runDir, liveLockFile)

	t.Run("held", func(t *testing.T) {
		f, busy, err := TryExclusiveLock(lockPath)
		if err != nil || busy {
			t.Fatalf("take the live lock: busy=%v err=%v", busy, err)
		}
		defer f.Close()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
			t.Fatalf("a held live lock: got %+v, want unclear", got)
		}
	})
	t.Run("unprovable", func(t *testing.T) {
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(lockPath, 0o700); err != nil { // opening a directory O_RDWR fails: probeUnknown
			t.Fatal(err)
		}
		defer func() {
			_ = os.Remove(lockPath)
			_ = os.WriteFile(lockPath, nil, 0o600)
		}()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverUnclear {
			t.Fatalf("an unprovable live lock: got %+v, want unclear", got)
		}
	})
	t.Run("missing", func(t *testing.T) {
		if err := os.Remove(lockPath); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.WriteFile(lockPath, nil, 0o600) }()
		if got := mustProbeLeftover(t, svc, runDir); got.Answer != LeftoverPresent {
			t.Fatalf("a missing live lock reads as free: got %+v, want leftover", got)
		}
	})
}

// TestProbeLeftoverValidationFailureIsAnError: Observe's validation applies —
// a manifest whose run id disagrees with its directory, or no manifest at all,
// is an error (callers treat it as unclear), never an answer.
func TestProbeLeftoverValidationFailureIsAnError(t *testing.T) {
	svc, runDir, orig := quiescedRun(t)
	withLeftoverProbes(t, probeLive, probeAbsent) // would be leftover

	m := orig
	m.RunID = "ffffffffffffffffffffffffffffffff"
	writeManifestCopy(t, runDir, m)
	if got, err := svc.ProbeLeftover(runDir); err == nil || got.Answer != LeftoverUnclear {
		t.Fatalf("a disagreeing manifest: got %+v err %v, want an error with an unclear answer", got, err)
	}

	if err := os.Remove(filepath.Join(runDir, manifestFile)); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.ProbeLeftover(runDir); err == nil || got.Answer != LeftoverUnclear {
		t.Fatalf("no manifest: got %+v err %v, want an error with an unclear answer", got, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestProbeLeftover' ./internal/process/`
Expected: FAIL to compile — `undefined: leftoverGroupProbe`, `undefined: LeftoverPresent`,
`svc.ProbeLeftover undefined`.

- [ ] **Step 3: Extract `validatedManifest` from `Observe`**

In `internal/process/observe.go`, add above `Observe`:

```go
// validatedManifest is Observe's step (1), shared with ProbeLeftover so both
// read a run through the same whole predicate: the manifest exists and decodes,
// the run dir resolves inside the manifest's recorded root, and the manifest's
// run id equals the run directory's name. op names the caller in a failure.
func validatedManifest(op, runDir string) (*manifestRecord, error) {
	m, err := readManifest(runDir)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, failf(FailInvalidState, op, "run directory has no manifest")
	}
	_, dirID, err := resolveRunDir(m.Root, runDir)
	if err != nil {
		return nil, err
	}
	if m.RunID != dirID {
		return nil, failf(FailInvalidState, op, "manifest run id disagrees with the run directory")
	}
	return m, nil
}
```

Replace `Observe`'s step (1) body (from `m, err := readManifest(runDir)` through the
`m.RunID != dirID` check) with:

```go
	// (1) Validate run path + manifest + run-ID agreement. The manifest supplies
	// the recorded root, so containment is proven against what the run claims
	// rather than against the run dir's own parent.
	m, err := validatedManifest("observe", runDir)
	if err != nil {
		return nil, err
	}
```

The failure strings and stages are byte-identical to before, so every `Observe` test stays green.

- [ ] **Step 4: Write `leftover.go`**

Create `internal/process/leftover.go`:

```go
package process

import "path/filepath"

// LeftoverAnswer is ProbeLeftover's three-way verdict on a run whose supervisor
// has exited (change 0492).
type LeftoverAnswer string

const (
	// LeftoverNone: the run's recorded process group is provably empty, so
	// nothing that stayed in the supervisor's group is still running.
	LeftoverNone LeftoverAnswer = "none"
	// LeftoverPresent: the recorded group still has members while its leader
	// (the supervisor's own pid) is provably gone — part of the suite outlived
	// its supervisor.
	LeftoverPresent LeftoverAnswer = "leftover"
	// LeftoverUnclear: anything the check cannot prove — a supervisor still
	// holding live.lock, an unprovable probe, an unaddressable group, or a
	// leader pid that still answers (a zombie supervisor, or a reused number).
	LeftoverUnclear LeftoverAnswer = "unclear"
)

// Leftover is ProbeLeftover's result: the answer and the recorded group id it
// is about (zero only when validation failed).
type Leftover struct {
	Answer LeftoverAnswer
	PGID   int
}

// leftoverGroupProbe and leftoverLeaderProbe are ProbeLeftover's liveness
// probes. Production is groupAlive and processAlive; a test overrides them to
// drive every unclear branch deterministically on every platform, in the style
// of recoverGroupProbe.
var (
	leftoverGroupProbe  = groupAlive
	leftoverLeaderProbe = processAlive
)

// ProbeLeftover answers, for a run whose supervisor has exited, whether any of
// its suite is still running in the supervisor's process group. It is
// read-only: it never signals and never writes.
//
//   - none: the recorded group is provably absent.
//   - leftover: the group still has members and its leader pid is provably
//     gone. Process-group ids are not reused while the group exists (Linux and
//     the BSD-derived kernels, macOS included, never hand out a pid still in
//     use as a process-group id), so a populated group whose leader is gone is
//     the run's own group.
//   - unclear: the supervisor still holds live.lock or the lock is
//     unprovable; the recorded pgid is <= 1 or differs from supervisor_pid; the
//     group probe is unknown; or the leader pid still answers (a zombie
//     supervisor whose launcher has not reaped it, or the number reused by an
//     unrelated process) or is unknown.
//
// Validation is Observe's (validatedManifest); a validation failure is an
// error, which callers treat as unclear. Callers act only on leftover — none
// and unclear both keep today's behavior.
//
// It sees only what stayed in the supervisor's group (go run, the suite
// runner). Test targets lead their own groups; while the runner lives the
// supervisor's group is non-empty, so that is enough. Targets whose runner was
// itself killed are invisible here (an accepted loss).
func (s *Service) ProbeLeftover(runDir string) (Leftover, error) {
	m, err := validatedManifest("probe-leftover", runDir)
	if err != nil {
		return Leftover{Answer: LeftoverUnclear}, err
	}
	out := Leftover{Answer: LeftoverUnclear, PGID: m.PGID}

	// The check means something only once the supervisor is gone: a held lock
	// or an unprovable probe is unclear, never absence.
	held, ans := probeFlock(filepath.Join(runDir, liveLockFile))
	if ans == probeUnknown || held {
		return out, nil
	}

	// Only the group the supervisor led is the run's: pgid <= 1 is never a real
	// group, and a pgid that is not the supervisor's own pid was never its.
	if m.PGID <= 1 || m.PGID != m.SupervisorPID {
		return out, nil
	}

	switch leftoverGroupProbe(m.PGID) {
	case probeAbsent:
		out.Answer = LeftoverNone
	case probeLive:
		// A live group alone is not enough: a zombie supervisor (or an unrelated
		// process reusing the number) still answers as its leader.
		if leftoverLeaderProbe(m.PGID) == probeAbsent {
			out.Answer = LeftoverPresent
		}
	}
	return out, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestProbeLeftover|TestObserve|TestImportBoundaryStdlibOnly' ./internal/process/`
Expected: PASS, with no `(cached)`.

- [ ] **Step 6: Mutation-test the guards (backup-copy restore each time)**

For each mutation: `cp internal/process/leftover.go internal/process/leftover.go.bak`, edit,
run `go test -count=1 -run 'TestProbeLeftover' ./internal/process/`, record the red tests, then
`mv -f internal/process/leftover.go.bak internal/process/leftover.go`.

1. Leader probe dropped: replace the `probeLive` case body with `out.Answer = LeftoverPresent`.
   Expected RED: `TestProbeLeftoverGroupAndLeaderAnswers/leader-live` and `/leader-unknown`
   (the spec's required mutation), and `TestProbeLeftoverZombieSupervisorIsUnclear` wherever the
   group probe reads live.
2. Unknown group collapsed into none: change `case probeAbsent:` to `case probeAbsent, probeUnknown:`.
   Expected RED: `TestProbeLeftoverGroupAndLeaderAnswers/group-unknown`.
3. Supervisor-pid guard dropped: change `m.PGID <= 1 || m.PGID != m.SupervisorPID` to `m.PGID <= 1`.
   Expected RED: `TestProbeLeftoverRefusesUnaddressableGroup/pgid-not-supervisor`.
4. Lock check dropped: delete the `if ans == probeUnknown || held { return out, nil }` block.
   Expected RED: `TestProbeLeftoverNeedsAFreeLiveLock/held` and `/unprovable`.

After the last restore, re-run Step 5's command: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/process/observe.go internal/process/leftover.go internal/process/leftover_test.go
git commit -m "feat(0492): process.ProbeLeftover reports a dead supervisor's still-populated group"
```

---

### Task 2: The launch census reports `tree-survives` and never changes its outcome

**Risk:** standard — an interface extension with mechanical test-double updates and one small
branch in the census. The posture (settled stays settled) is pinned by a mutation.

**Files:**
- Modify: `internal/gatedrive/driver.go` (`ProcessSeam` gains `ProbeLeftover`)
- Modify: `internal/gatedrive/reconcile.go` (file header, `supervisorGone`, `proveRunDirsGone`,
  new `censusFindingRank`)
- Modify: `internal/gatedrive/driver_test.go` (`fakeProc`)
- Modify: `internal/gatedrive/driver_concurrency_test.go` (`racingProc`, `terminalSettleProc`,
  `gatedLoserSeam`, `claimWindowProc`, `countingProc`)
- Modify: `internal/gatedrive/reconcile_test.go` (`supervisors` helper; new tests)

**Interfaces:**
- Consumes: `process.Leftover`, `process.LeftoverPresent`, `process.LeftoverNone`,
  `process.LeftoverUnclear`, `(*process.Service).ProbeLeftover` from Task 1.
- Produces: `ProcessSeam.ProbeLeftover(runDir string) (process.Leftover, error)`; the census
  finding `tree-survives:<drive id>:<pgid>`; `func censusFindingRank(f string) int`.

The six `ProcessSeam` implementations are every type with a
`Launch(process.LaunchRequest)` method under `internal/gatedrive` (re-derive with
`git grep -n ') Launch(.*process.LaunchRequest' -- '*.go'`). Production wires only
`*process.Service` (`NewSystemDriver` callers in `internal/app`), which gains the method in
Task 1. The interface change and every double change land in this one task so the package
compiles at every commit.

- [ ] **Step 1: Extend the seam and the test doubles (compile-only change)**

In `internal/gatedrive/driver.go`, add to `ProcessSeam` after `ResolveReservation`:

```go
	// ProbeLeftover answers, read-only, whether a run whose supervisor has exited
	// still has members in its recorded process group (change 0492): none,
	// leftover, or unclear. The launch census reports a leftover as tree-survives
	// and never signals on it; an unclear answer or an error is today's behavior.
	ProbeLeftover(runDir string) (process.Leftover, error)
```

In `internal/gatedrive/driver_test.go`, give `fakeProc` a closure field and a method. Add the
field beside `resolve`:

```go
	leftover func(runDir string) (process.Leftover, error)
```

add `leftoverN` to the counter line (`launchN, observeN, stopN, resolveN, leftoverN int`), and
add after `ResolveReservation`:

```go
// ProbeLeftover defaults to none — an exited supervisor whose group is empty —
// so every existing census test keeps its run-terminal finding. Tests that
// model a leftover suite inject a closure.
func (f *fakeProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	f.leftoverN++
	if f.leftover == nil {
		return process.Leftover{Answer: process.LeftoverNone}, nil
	}
	return f.leftover(runDir)
}
```

In `internal/gatedrive/driver_concurrency_test.go`, add after each type's `ResolveReservation`:

```go
func (p *racingProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}
```

```go
func (p *terminalSettleProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}
```

```go
func (s *gatedLoserSeam) ProbeLeftover(runDir string) (process.Leftover, error) {
	return s.core.ProbeLeftover(runDir)
}
```

```go
func (p *claimWindowProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}
```

```go
func (p *countingProc) ProbeLeftover(runDir string) (process.Leftover, error) {
	return process.Leftover{Answer: process.LeftoverNone}, nil
}
```

Run: `go vet ./internal/gatedrive/ && go vet -tags integration ./internal/gatedrive/ ./internal/app/`
Expected: no output (everything compiles; `var _ ProcessSeam = (*process.Service)(nil)` in
`driver_test.go` holds because Task 1 added the method).

- [ ] **Step 2: Write the failing census tests**

In `internal/gatedrive/reconcile_test.go`, extend the `supervisors` helper. Add fields:

```go
	// leftover[dir] is what ProbeLeftover answers for dir (an unlisted dir is
	// none); leftoverErr[dir] makes it fail. probed records every probed dir.
	leftover    map[string]process.Leftover
	leftoverErr map[string]error
	probed      []string
```

initialise the two maps in `newSupervisors`:

```go
func newSupervisors() *supervisors {
	return &supervisors{
		state: map[string]process.State{}, stuck: map[string]bool{},
		leftover: map[string]process.Leftover{}, leftoverErr: map[string]error{},
	}
}
```

and wire a `leftover` closure into the `fakeProc` that `proc()` returns (beside `resolve`):

```go
		leftover: func(runDir string) (process.Leftover, error) {
			s.probed = append(s.probed, runDir)
			lo, ok := s.leftover[runDir]
			if !ok {
				lo = process.Leftover{Answer: process.LeftoverNone}
			}
			return lo, s.leftoverErr[runDir]
		},
```

Add these tests at the end of the file:

```go
// containsString reports whether xs holds s.
func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// TestCensusReportsLeftoverOfDeadSupervisor (change 0492): a dead supervisor
// whose recorded group still has members is STILL torn down — the drive stays
// settled in both modes and nothing is stopped — but the finding names the
// surviving group, tree-survives:<drive>:<pgid>, instead of run-terminal. This
// holds for a nonterminal drive and for the HALTED drive a killed supervisor's
// slice actually leaves behind.
//
// Mutation check: making supervisorGone return false on a leftover must turn
// this red (the failure posture: a leftover never changes the outcome).
func TestCensusReportsLeftoverOfDeadSupervisor(t *testing.T) {
	for _, st := range []process.State{process.StateVanished, process.StateSignaled, process.StateStopped} {
		for _, halted := range []bool{false, true} {
			name := string(st)
			if halted {
				name += "/halted"
			}
			t.Run(name, func(t *testing.T) {
				sup := newSupervisors()
				proc := sup.proc()
				d, store := newTestDriver(t, &fakeClock{now: startRun()}, proc, stableGit())
				dir := liveRunDir(t, "run1")
				sup.state[dir] = st
				sup.leftover[dir] = process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}
				id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
					r.RawRunDir = dir
					if halted {
						r.LastOutcome = HALTED
						r.LastCause = "relaunch-exhausted"
					}
				})

				for _, mode := range censusModes(d) {
					report, err := mode.run(capHash(censusCtxA))
					if err != nil {
						t.Fatalf("%s: %v", mode.name, err)
					}
					if !report.Accounted || !findingFor(report.Findings, "tree-survives", id+":4242") {
						t.Fatalf("%s: a leftover is settled and reported tree-survives:%s:4242, got %+v", mode.name, id, report)
					}
					if findingFor(report.Findings, "run-terminal", id) {
						t.Fatalf("%s: tree-survives replaces run-terminal for the drive, got %+v", mode.name, report)
					}
				}
				if proc.stopN != 0 {
					t.Fatalf("a leftover is never stopped, Stop called %d times", proc.stopN)
				}
			})
		}
	}
}

// TestCensusNoLeftoverKeepsRunTerminal (change 0492): none, unclear, and a probe
// error all keep today's run-terminal:<drive>. The error row answers leftover
// WITH an error — the census trusts an answer only when the probe succeeded.
func TestCensusNoLeftoverKeepsRunTerminal(t *testing.T) {
	for _, tc := range []struct {
		name string
		lo   process.Leftover
		err  error
	}{
		{"none", process.Leftover{Answer: process.LeftoverNone, PGID: 4242}, nil},
		{"unclear", process.Leftover{Answer: process.LeftoverUnclear, PGID: 4242}, nil},
		{"error", process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}, errors.New("probe failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sup := newSupervisors()
			d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
			dir := liveRunDir(t, "run1")
			sup.state[dir] = process.StateVanished
			sup.leftover[dir] = tc.lo
			if tc.err != nil {
				sup.leftoverErr[dir] = tc.err
			}
			id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) { r.RawRunDir = dir })

			for _, mode := range censusModes(d) {
				report, err := mode.run(capHash(censusCtxA))
				if err != nil {
					t.Fatalf("%s: %v", mode.name, err)
				}
				if !report.Accounted || !findingFor(report.Findings, "run-terminal", id) {
					t.Fatalf("%s: %s keeps run-terminal:%s, got %+v", mode.name, tc.name, id, report)
				}
				if reconcileFindingPresent(report.Findings, "tree-survives:") {
					t.Fatalf("%s: %s must never report tree-survives, got %+v", mode.name, tc.name, report)
				}
			}
		})
	}
}

// TestCensusLeftoverOutranksOtherFindings (change 0492): across a drive's two
// recorded run dirs, tree-survives outranks replacement-stopped and
// run-terminal; with two leftovers the first dir's (RawRunDir) group is named.
// A dir the census itself stopped is never probed: Stop already waited for its
// group to empty.
func TestCensusLeftoverOutranksOtherFindings(t *testing.T) {
	t.Run("prior-leftover", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		replacement, prior := liveRunDir(t, "run2"), liveRunDir(t, "run1")
		sup.state[replacement] = process.StateRunning // stopped by the census
		sup.state[prior] = process.StateVanished
		sup.leftover[prior] = process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.PriorRawRunDir = replacement, prior
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "tree-survives", id+":4242") {
			t.Fatalf("tree-survives outranks replacement-stopped, got %+v", report)
		}
		if containsString(sup.probed, replacement) {
			t.Fatalf("the census-stopped dir %s was probed for a leftover; probed %v", replacement, sup.probed)
		}
	})
	t.Run("replacement-leftover", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		replacement, prior := liveRunDir(t, "run2"), liveRunDir(t, "run1")
		sup.state[replacement] = process.StateVanished
		sup.leftover[replacement] = process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}
		sup.state[prior] = process.StateRunning // stopped by the census, after the leftover
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.PriorRawRunDir = replacement, prior
			r.RelaunchCount = 1
		})

		report, err := d.ReconcileRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("ReconcileRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "tree-survives", id+":4242") {
			t.Fatalf("a later replacement-stopped must not displace tree-survives, got %+v", report)
		}
	})
	t.Run("both-leftover", func(t *testing.T) {
		sup := newSupervisors()
		d, store := newTestDriver(t, &fakeClock{now: startRun()}, sup.proc(), stableGit())
		replacement, prior := liveRunDir(t, "run2"), liveRunDir(t, "run1")
		sup.state[replacement] = process.StateVanished
		sup.state[prior] = process.StateVanished
		sup.leftover[replacement] = process.Leftover{Answer: process.LeftoverPresent, PGID: 4242}
		sup.leftover[prior] = process.Leftover{Answer: process.LeftoverPresent, PGID: 5353}
		id, _ := seedRunDrive(t, store, censusCtxA, func(r *driveRecord) {
			r.RawRunDir, r.PriorRawRunDir = replacement, prior
			r.RelaunchCount = 1
		})

		report, err := d.VerdictRunLaunches(capHash(censusCtxA))
		if err != nil {
			t.Fatalf("VerdictRunLaunches: %v", err)
		}
		if !report.Accounted || !findingFor(report.Findings, "tree-survives", id+":4242") ||
			findingFor(report.Findings, "tree-survives", id+":5353") {
			t.Fatalf("two leftovers report the first dir's group only, got %+v", report)
		}
	})
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestCensusReportsLeftoverOfDeadSupervisor|TestCensusNoLeftoverKeepsRunTerminal|TestCensusLeftoverOutranksOtherFindings' ./internal/gatedrive/`
Expected: FAIL — `TestCensusReportsLeftoverOfDeadSupervisor` and
`TestCensusLeftoverOutranksOtherFindings` report `run-terminal` / `replacement-stopped` instead of
`tree-survives`. `TestCensusNoLeftoverKeepsRunTerminal` already passes (it pins today's behavior).

- [ ] **Step 4: Implement the census change**

In `internal/gatedrive/reconcile.go`, add `"strconv"` and `"strings"` to the imports.

In `supervisorGone`, replace the first switch's exited case

```go
	case o.State.SupervisorExited():
		return true, "run-terminal:" + id
```

with

```go
	case o.State.SupervisorExited():
		// The supervisor is gone, so the drive is torn down whatever is left. A
		// populated group whose leader is gone is reported, never signalled
		// (change 0492): with its supervisor dead, no lock proves the group is the
		// run's own. Only a successful leftover answer changes the finding; none,
		// unclear, and a probe error keep run-terminal.
		if lo, lerr := d.proc.ProbeLeftover(runDir); lerr == nil && lo.Answer == process.LeftoverPresent {
			return true, "tree-survives:" + id + ":" + strconv.Itoa(lo.PGID)
		}
		return true, "run-terminal:" + id
```

Leave the post-`Stop` switch (`replacement-stopped`) untouched: no probe there.

In `proveRunDirsGone`, replace

```go
		if f != "" && finding != "replacement-stopped:"+id {
			finding = f
		}
```

with

```go
		if censusFindingRank(f) > censusFindingRank(finding) {
			finding = f
		}
```

and add after `proveRunDirsGone`:

```go
// censusFindingRank orders the findings proveRunDirsGone may report for one
// drive: tree-survives (a dead supervisor's group still has members) over
// replacement-stopped (a stop this census performed) over run-terminal. A tie
// keeps the first dir's finding, so RawRunDir's leftover is the one named.
func censusFindingRank(f string) int {
	switch {
	case strings.HasPrefix(f, "tree-survives:"):
		return 3
	case strings.HasPrefix(f, "replacement-stopped:"):
		return 2
	case f != "":
		return 1
	default:
		return 0
	}
}
```

Update the doc comments:

- `proveRunDirsGone`: replace "(replacement-stopped over run-terminal), so a stop this census
  performed is always reported." with "(tree-survives over replacement-stopped over run-terminal,
  censusFindingRank), so a surviving group, and otherwise a stop this census performed, is always
  reported."
- `supervisorGone`: after "An observed exit (supervisorExited: passed, failed, signaled, stopped,
  vanished) counts as torn down." insert "Its finding is run-terminal, or tree-survives:<drive>:<pgid>
  when process.Service.ProbeLeftover proves the dead supervisor's group still has members — still
  torn down, reported, never signalled (change 0492)."
- File header: replace the paragraph starting "Teardown proof is the lock model's proof: the
  supervisor is gone." up to and including "A probe or stop error is never clean absence: it keeps
  the run pending." with:

```go
// Teardown proof is the lock model's proof: the supervisor is gone. For each run
// dir a drive records (RawRunDir, PriorRawRunDir) a dir that no longer exists is
// clean absence; any observed state other than running counts as torn down; a
// running supervisor is stopped (cancel mode) and must then observe as not
// running, or is reported run-live (verdict mode). A probe or stop error is never
// clean absence: it keeps the run pending.
// A dead supervisor's suite can outlive it (change 0492): when the supervisor's
// recorded process group still has members, the drive still counts as torn down,
// and its finding is tree-survives:<drive>:<pgid> instead of run-terminal. The
// census reports that and never signals the group — with the supervisor dead, no
// lock proves the group is the run's own. An unclear or failed leftover probe
// keeps run-terminal, exactly as before.
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestCensus|TestDriver|TestReconcile' ./internal/gatedrive/`
Expected: PASS, with no `(cached)`.

Then run the whole package once, still focused on this task's blast radius:
`go test -count=1 ./internal/gatedrive/`
Expected: PASS.

- [ ] **Step 6: Mutation-test the posture and the ranking (backup-copy restore each time)**

For each: `cp internal/gatedrive/reconcile.go internal/gatedrive/reconcile.go.bak`, edit, run
`go test -count=1 -run 'TestCensusReportsLeftoverOfDeadSupervisor|TestCensusNoLeftoverKeepsRunTerminal|TestCensusLeftoverOutranksOtherFindings' ./internal/gatedrive/`,
record the red tests, then `mv -f internal/gatedrive/reconcile.go.bak internal/gatedrive/reconcile.go`.

1. Posture: change `return true, "tree-survives:" + …` to `return false, "tree-survives:" + …`.
   Expected RED: `TestCensusReportsLeftoverOfDeadSupervisor` (not accounted). This is the spec's
   required posture mutation; Task 3 repeats it against the app-level cancel test.
2. Ranking: restore the old line `if f != "" && finding != "replacement-stopped:"+id { finding = f }`.
   Expected RED: `TestCensusLeftoverOutranksOtherFindings/prior-leftover` and `/replacement-leftover`.
3. Unclear collapsed into leftover: change `lo.Answer == process.LeftoverPresent` to
   `lo.Answer != process.LeftoverNone`.
   Expected RED: `TestCensusNoLeftoverKeepsRunTerminal/unclear`.
4. Error ignored: delete `lerr == nil && `.
   Expected RED: `TestCensusNoLeftoverKeepsRunTerminal/error`.

After the last restore, re-run Step 5's commands: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/gatedrive/driver.go internal/gatedrive/reconcile.go internal/gatedrive/driver_test.go internal/gatedrive/driver_concurrency_test.go internal/gatedrive/reconcile_test.go
git commit -m "feat(0492): the launch census reports tree-survives for a dead supervisor's live group"
```

---

### Task 3: Pin the app-layer posture — cancel stays `cancelled`, closeout stays `run-complete`

**Risk:** standard — real-process integration test over the production seams, plus a seam-level
closeout test. No production code changes beyond comments.

**TDD note:** the behavior these tests pin ships in Task 2, so they pass on first run. RED evidence
comes from the mutations in Step 4, which are the spec's required posture mutation applied at the
app layer.

**Files:**
- Modify: `internal/app/runtracker_cancel_integration_test.go`
  (`TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled`)
- Modify: `internal/app/runtracker_complete_integration_test.go` (new test)
- Modify: `internal/app/runtracker_cancel.go` (comments only)
- Modify: `internal/app/runtracker_complete.go` (comments only)

**Interfaces:**
- Consumes: the census finding `tree-survives:<drive>:<pgid>` (Task 2); app test helpers
  `newCancelFixture`, `startRunDrive`, `awaitRunPhase`, `awaitStart`, `onlyDriveID`, `runCancel`,
  `productionCancelSeams`, `hasFinding`, `loadRunState`, `newCompletionFixture`,
  `fakeLaunchObserver`, `LoadRunTrackerRecord`, `runTrackerCompleteRun`.
- Produces: nothing new.

- [ ] **Step 1: Extend the vanished/signaled cancel test**

In `internal/app/runtracker_cancel_integration_test.go`, rewrite
`TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled` so the vanished case proves the
leftover finding end to end. The production fixture's `reapRunSupervisors` (started by
`startRunDrive`) reaps the killed supervisor; the test waits for that reap so the leader pid is
provably gone, then expects `tree-survives:<drive>:<pgid>`. The signaled case's suite died with
its shell, so its finding stays `run-terminal:<drive>` (an empty group reads none; an unreaped one
reads unclear; both keep `run-terminal`).

Replace the doc comment and body with:

```go
// TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled: when the run's
// drive's supervisor is already gone — killed outright (vanished), or exited after
// recording its suite's signal death (signaled) — cancel reaches cancelled, never
// cancellation-pending: teardown proof is "the supervisor is gone", and every
// state but running proves it, for the census and for a registered execution
// participant's stop alike. A supervisor killed alone leaves its suite running in
// its group (change 0492): cancel still reports cancelled, adds the informational
// tree-survives:<drive>:<pgid> finding, and never signals that group.
func TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		command string
		kill    bool // SIGKILL the supervisor alone once the suite runs
	}{
		{"vanished", "sleep 60", true},
		{"signaled", "kill -KILL $$", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			runRoot, done := startRunDrive(t, fx, tc.command)
			var runDir string
			var m runManifest
			if tc.kill {
				runDir, m = awaitRunPhase(t, runRoot, "running")
				// The orphaned suite outlives its killed supervisor; cancel reports it
				// and never stops it (change 0492). End its group at cleanup.
				t.Cleanup(func() { _ = syscall.Kill(-m.PGID, syscall.SIGKILL) })
				if err := syscall.Kill(m.SupervisorPID, syscall.SIGKILL); err != nil {
					t.Fatalf("kill the supervisor: %v", err)
				}
			}
			awaitStart(t, done) // the drive's slice sees the death and HALTs
			if runDir == "" {
				runDir, _ = awaitRunPhase(t, runRoot, "terminal")
			}
			if obs := GateObserve(runDir); obs.State == "running" {
				t.Fatalf("precondition: the supervisor must be gone, observed %q", obs.State)
			}
			if tc.kill {
				// The leftover check needs the leader pid provably gone: wait for the
				// fixture's reaper to collect the killed supervisor.
				deadline := time.Now().Add(30 * time.Second)
				for syscall.Kill(m.SupervisorPID, 0) != syscall.ESRCH {
					if time.Now().After(deadline) {
						t.Fatalf("the killed supervisor %d was never reaped", m.SupervisorPID)
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
			// The run also registered the run as an execution participant, so the
			// stopper's proof rule (process.State.SupervisorExited) is exercised beside the census's.
			must(t, RegisterRunParticipant(fx.repo, fx.key,
				RunParticipant{Kind: participantKindGateScope, NativeHandle: runDir}))
			id := onlyDriveID(t, fx)

			res := runCancel(productionCancelSeams(fx.repo), fx.repo, fx.key, "human stop")
			if res.Disposition != CancelDispositionCancelled {
				t.Fatalf("disposition = %q, want cancelled (findings=%v)", res.Disposition, res.Findings)
			}
			if st := loadRunState(t, fx.repo, fx.key); st != RunCancelled {
				t.Fatalf("run state = %q, want cancelled", st)
			}
			want := "run-terminal:" + id
			if tc.kill {
				want = fmt.Sprintf("tree-survives:%s:%d", id, m.PGID)
			}
			if !hasFinding(res.Findings, want) {
				t.Fatalf("findings = %v, want %s", res.Findings, want)
			}
			if tc.kill && syscall.Kill(-m.PGID, 0) != nil {
				t.Fatalf("cancel must never signal the leftover group %d, but it is gone", m.PGID)
			}
		})
	}
}
```

(`fmt`, `syscall`, and `time` are already imported in this file.)

- [ ] **Step 2: Add the closeout test**

In `internal/app/runtracker_complete_integration_test.go`, add:

```go
// TestIntegrationRunCompletionTreeSurvivesNeverBlocksCloseout (change 0492): a
// census that accounts the run's drives but reports a dead supervisor's
// surviving group (tree-survives) never blocks the closeout. The keyed verdict is
// unchanged — run-done run-complete — the run is durably completed, and the
// finding is surfaced in completion_findings.
func TestIntegrationRunCompletionTreeSurvivesNeverBlocksCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	const leftover = "tree-survives:d1:4242"
	fx.launchObserver.report = gatedrive.RunLaunchReport{Accounted: true, Findings: []string{leftover}}
	rec, err := LoadRunTrackerRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatalf("LoadRunTrackerRecord: %v", err)
	}

	res := runTrackerCompleteRun(fx.repo, fx.key, rec, 42, fx.seams())
	if res.Decision != RunDecisionDone || res.Outcome != VerdictRunComplete {
		t.Fatalf("verdict = %s %s (reason %q, findings %v), want run-done run-complete",
			res.Decision, res.Outcome, res.Reason, res.CompletionFindings)
	}
	if !hasFinding(res.CompletionFindings, leftover) {
		t.Fatalf("completion findings = %v, want %s surfaced", res.CompletionFindings, leftover)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("run state = %q, want completed", st)
	}
}
```

- [ ] **Step 3: Run the tests**

Run: `go test -count=1 -tags integration -run 'TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled|TestIntegrationRunCompletionTreeSurvivesNeverBlocksCloseout' ./internal/app/`
Expected: PASS, with no `(cached)`.

- [ ] **Step 4: Mutation-test the posture at the app layer (backup-copy restore each time)**

1. Census posture: `cp internal/gatedrive/reconcile.go internal/gatedrive/reconcile.go.bak`;
   in `supervisorGone` change `return true, "tree-survives:" + …` to `return false, "tree-survives:" + …`;
   run `go test -count=1 -tags integration -run 'TestIntegrationRunCancelSignaledOrVanishedSupervisorIsCancelled' ./internal/app/`.
   Expected RED: `…/vanished` (disposition `cancellation-pending`).
   Restore: `mv -f internal/gatedrive/reconcile.go.bak internal/gatedrive/reconcile.go`.
2. Closeout posture: `cp internal/app/runtracker_complete.go internal/app/runtracker_complete.go.bak`;
   in `accountCompletionLaunches` change the final `return false, report.Findings` to
   `return len(report.Findings) > 0, report.Findings`;
   run `go test -count=1 -tags integration -run 'TestIntegrationRunCompletionTreeSurvivesNeverBlocksCloseout' ./internal/app/`.
   Expected RED (verdict `run-stop`, reason `completion-unaccounted`).
   Restore: `mv -f internal/app/runtracker_complete.go.bak internal/app/runtracker_complete.go`.

After both restores, re-run Step 3's command: PASS.

- [ ] **Step 5: Update the app-layer comments**

In `internal/app/runtracker_cancel.go`, step (5c)'s comment in `reconcileRunTeardown`: after
"stop a running supervisor (teardown proof is \"the supervisor is gone\")," insert "report a dead
supervisor's still-populated group as the informational tree-survives finding without stopping it
(change 0492),".

In `internal/app/runtracker_complete.go`, step (3)'s comment in `completeSuccessfulRun`: change
"informational findings (a settled drive's run-terminal) are accounted." to "informational
findings (a settled drive's run-terminal or tree-survives) are accounted and never block."

Run: `go vet -tags integration ./internal/app/`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/app/runtracker_cancel_integration_test.go internal/app/runtracker_complete_integration_test.go internal/app/runtracker_cancel.go internal/app/runtracker_complete.go
git commit -m "test(0492): cancel stays cancelled and closeout stays run-complete beside tree-survives"
```

---

### Task 4: Glossary, the run-tracker concept doc, and the remaining 0492 comments

**Risk:** economy — documentation and comments only, fully specified below.

**Files:**
- Modify: `docs/reference/glossary.md`
- Modify: `docs/concepts/run-tracker.md`
- Modify: `internal/process/worktree_lock_test.go` (comment on `TestWorktreeLockFreedWhenSupervisorKilled`)
- Modify: `internal/gatedrive/driver_lock_integration_test.go` (comment on
  `TestIntegrationGatedriveKilledSupervisorFreesWorktreeForNextStart`)

**Interfaces:**
- Consumes: the finding token and behavior from Tasks 1–3.
- Produces: nothing executable.

- [ ] **Step 1: Re-derive the prose sites**

Run:

```bash
git grep -n -e "run-terminal" -e "supervisor is gone" -e "torn down" -e "0492" -- . \
  ':!docs/superpowers' ':!docs/results' ':!docs/changes' ':!docs/adrs'
```

Sort every hit into executable or prose and into "about the census's teardown proof" or not. At
plan time the maintained prose that describes the census's teardown or names 0492 as a pending gap
was: `docs/concepts/run-tracker.md` (the paragraph "`run.cancel` finds a run's drives …
are tracked by change 0492."), and the two test comments listed above. The `internal/process`
and `internal/gatedrive` comments that say "the supervisor is gone" describe `Observe`'s free-lock
step, `Recover`, launch EOF, or the worktree lock, and stay as they are. `internal/gatedrive` and
`internal/app` code comments were already updated in Tasks 2–3. If the grep finds another
maintained site that states a gone supervisor means a gone suite, update it in the same register as
below and list it in the commit body.

- [ ] **Step 2: Add the glossary entry**

In `docs/reference/glossary.md`, insert directly after the `### Cancel` entry's closing
```` ``` ```` fence (before `### Continuation`):

````markdown
### Cancel finding `tree-survives`

`tree-survives:<drive>:<pgid>` is an informational finding from `run.cancel`, the death guardian,
and `run.verdict`'s success closeout. The drive's supervisor has exited (killed alone or crashed),
but its process group `<pgid>` still has members, so part of the suite (usually `go run` and the
test runner) is still running. Cancel still reports `cancelled`, the closeout verdict is unchanged,
and the leftover suite finishes on its own. It is information, never a blocker: no skill or
reviewer escalates it into one.

Docket never signals the group: with the supervisor dead, nothing proves the group is still the
run's own. To stop it yourself, confirm its members first, then signal the group:

```sh
pgrep -lg <pgid>
kill -TERM -<pgid>
```

The check sees only the supervisor's own group, not test targets that lead their own groups. When
it cannot tell (for example, the dead supervisor is still an unreaped zombie), the finding is the
ordinary `run-terminal:<drive>`.
````

- [ ] **Step 3: Update the run-tracker concept doc**

In `docs/concepts/run-tracker.md`, replace the paragraph

```markdown
`run.cancel` finds a run's drives by the run context those drives record,
and counts a drive torn down once its supervisor is gone. The known
process-tree gaps — a supervisor that dies alone while its children keep
running, a KILL escalation, and TERM ending `go run` while the suite runner
is still stopping its targets — are tracked by change 0492.
```

with

```markdown
`run.cancel` finds a run's drives by the run context those drives record,
and counts a drive torn down once its supervisor is gone. When a supervisor
died alone (SIGKILL or a crash) and its process group still has members,
cancel still reports `cancelled` and adds a `tree-survives:<drive>:<pgid>`
finding: the suite is still running and finishes on its own. Docket never
stops it; the glossary's `tree-survives` entry shows how to. Two smaller
gaps stay accepted: a KILL escalation can leave test targets running in
their own process groups, and after a graceful stop the worktree frees a
moment before the suite runner finishes stopping its targets.
```

- [ ] **Step 4: Update the two test comments**

In `internal/process/worktree_lock_test.go`, the comment above
`TestWorktreeLockFreedWhenSupervisorKilled` reads
"suite may survive (accepted gap, change 0492) — the test ends the group itself." Change it to
"suite may survive; the census reports that as tree-survives and never stops it (change 0492) —
the test ends the group itself."

In `internal/gatedrive/driver_lock_integration_test.go`, the comment above
`TestIntegrationGatedriveKilledSupervisorFreesWorktreeForNextStart` reads
"The killed supervisor's suite survives it (the accepted teardown gap, change 0492), so cleanup
ends its process group." Change it to "The killed supervisor's suite survives it (the census
reports it as tree-survives and never stops it, change 0492), so cleanup ends its process group."

- [ ] **Step 5: Verify the generated mirror is untouched and the edits compile**

Run:

```bash
go generate ./internal/assets && git status --porcelain -- internal/assets
go vet ./internal/process/ && go vet -tags integration ./internal/gatedrive/
go test -count=1 -run 'TestCommentAnchorStyle' ./internal/repoguard/
```

Expected: `git status` prints nothing (no mirrored file changed: the glossary and
`docs/concepts/` are not mirrored); `go vet` prints nothing; `TestCommentAnchorStyle` PASS. If
`git status` does list a regenerated file, stage it with this task's commit.

- [ ] **Step 6: Commit**

```bash
git add docs/reference/glossary.md docs/concepts/run-tracker.md internal/process/worktree_lock_test.go internal/gatedrive/driver_lock_integration_test.go
git commit -m "docs(0492): glossary and run-tracker concept describe the tree-survives finding"
```

---

## Spec coverage

| Spec item | Task |
|---|---|
| Decision 1: read-only leftover check, Observe's validation, three answers, seams, stdlib-only, `classifyRun` unchanged | 1 |
| Decision 2: `supervisorGone` reports `tree-survives:<drive>:<pgid>`, still settled; no check after a census stop; ranking in `proveRunDirsGone` | 2 |
| `run.cancel` / guardian: disposition `cancelled` with the finding | 3 (guardian shares `reconcileRunTeardown`) |
| Success closeout: verdict unchanged, finding surfaced | 3 |
| `run.start --resume`: unchanged | no task (no code path changes) |
| Failure posture: pinned, mutation-checked | 2 Step 6, 3 Step 4 |
| Tests: real kill + reap → leftover → none; zombie → unclear; seam unclear branches; leader-probe mutation | 1 |
| Prose: glossary, `reconcile.go` header and doc comments, process doc comment, mirror check, whole-repo grep | 1, 2, 4 |
| Decision record (new ADR, ADR-0132 Update note) | coordinator via docket-adr, not a build task |
| Build gate: whole suite through `build.test_command`, budget lines | docket-build's gate |
