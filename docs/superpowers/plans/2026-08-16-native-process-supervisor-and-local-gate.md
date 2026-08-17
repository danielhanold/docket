<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0314 — Native process supervisor and local gate](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0314-native-process-supervisor-and-local-gate.md)**
<!-- docket:backlink:end -->

# Native Process Supervisor and Local Gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A new repository-independent `internal/process` package plus a public `docket gate` CLI group (launch / observe / stop / recover) that re-executes the `docket` binary as a per-run session-leader supervisor, records exact exit/signal status durably, and signals only ownership-proven process groups.

**Architecture:** `internal/cli` (flags, `--` boundary, presentation) → `internal/app` (operation names, `GateResult` DTO, result mapping) → `internal/process` (run identity, durable atomic state, launch/observe/stop/recover state machines, platform syscall seam) → stdlib only. The supervisor is the same binary re-exec'd with `Setsid`, activated by a private environment variable checked before Cobra ever runs; it holds the run's `live.lock` for its lifetime and is the only writer of `terminal.json`.

**Tech Stack:** Go 1.26 stdlib (`os/exec`, `syscall`, `crypto/rand`, `encoding/json`), Cobra (CLI layer only), bash test-suite wrappers (`tests/test_*.sh`) over `go test`.

**Spec:** `docs/superpowers/specs/2026-08-16-native-process-supervisor-and-local-gate-design.md` (synchronized copy readable at `.docket/docs/superpowers/specs/…` from the primary checkout). The plan argues from the spec; executors read both.

## Global Constraints

- Package dependency direction is fixed: `internal/cli` → `internal/app` → `internal/process` → stdlib/syscalls. `internal/process` imports **no** `github.com/danielhanold/docket/...` package (not even `internal/buildinfo`); `internal/cli` never imports `internal/process`. Guarded by tests in Tasks 1 and 12.
- `cmd/docket/main.go` stays the **only** `os.Exit` site (`cmd/docket/exit_sites_test.go` enforces this). The supervisor path returns an int up through `cli.Run` like every other path.
- Protocol operation names are exactly `gate.launch`, `gate.observe`, `gate.stop`, `gate.recover`. JSON is one compact document + one newline on stdout via the existing `cli.Presenter`; exit codes come only from `app.ExitCode` (0 applied/no-op, 2 invalid-input, 1 everything else).
- Never `fork` and continue the Go runtime in a child: new processes start only via `os/exec` exec boundaries. No shell strings — argv arrays only. No `runtime.GOOS` branches in shared state machines; platform syscalls live in `platform_darwin.go` / `platform_linux.go`.
- Promised modes are enforced with explicit `os.Chmod` after creation (`0700` dirs, `0600` files) — creation-mode arguments are umask-masked. Every structured write is same-directory temp file → write → `Sync` → `Chmod` → `Rename` → dir `Sync`.
- Production timing constants: establishment bound **10s**, stop TERM wait **10s**, KILL verification **5s**. Tests never tune sleeps against these; they shrink the bounds through package-private `Service` fields and synchronize on pipes/files/kernel state with generous outer deadlines.
- Every mutation probe: `cp` the file to a `.bak` first, confirm the mutation landed with `git diff` (or `grep -c` via `/usr/bin/grep -F`), run `go test -count=1` (never a bare `go test` — the cache serves stale greens), then `mv` the `.bak` back. `git checkout --` is forbidden as a restore step.
- All Go files gofmt-clean (the suite's Go gate reddens otherwise).
- Free-form argv, environment values, and child output bytes never appear in protocol error text or `HumanText`. Logs stay in the run directory.
- The ADR superseding ADR-0081 is **not** a plan task: docket-implement-next Step 6 mints it through `docket-adr` at review time (ADR-0081 stays listed on the change's `adrs:`). Nothing in this plan touches `docs/adrs/`.
- Behavior of changes 0305–0313 and 0315–0318 is out of scope: do not touch `scripts/gate-run.sh`, its Bash tests, shipped skills, config resolution, or any workspace/metadata code path.

---

### Task 1: `internal/process` scaffold — identity, failure taxonomy, import boundary

**Files:**
- Create: `internal/process/process.go`, `internal/process/ids.go`, `internal/process/failure.go`
- Test: `internal/process/ids_test.go`, `internal/process/failure_test.go`, `internal/process/boundary_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: `process.Service` (fields consumed by every later task), `NewService(executable string) (*Service, error)`, `NewRunIdentity() (runID, token string, err error)`, `State` + its six constants, `Terminal{Kind string; ExitCode int; Signal int}`, `Failure{Class FailureClass; Stage, Reason string}` implementing `error`, `FailureClass` constants `FailInvalidInput`, `FailInvalidState`, `FailBlocked`, `FailExternal`, helper `failf(class FailureClass, stage, format string, a ...any) *Failure`, `AsFailure(err error) (*Failure, bool)`.

- [ ] **Step 1: Write the failing tests**

`internal/process/ids_test.go`:

```go
package process

import (
	"regexp"
	"testing"
)

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestNewRunIdentityShape(t *testing.T) {
	id, tok, err := NewRunIdentity()
	if err != nil {
		t.Fatalf("NewRunIdentity: %v", err)
	}
	if !hex32.MatchString(id) || !hex32.MatchString(tok) {
		t.Fatalf("want 32 lowercase hex chars each, got id=%q token=%q", id, tok)
	}
	if id == tok {
		t.Fatalf("run id and token must be independent randomness")
	}
}

func TestNewRunIdentityUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		id, tok, err := NewRunIdentity()
		if err != nil {
			t.Fatalf("NewRunIdentity: %v", err)
		}
		if seen[id] || seen[tok] {
			t.Fatalf("collision at iteration %d", i)
		}
		seen[id], seen[tok] = true, true
	}
}
```

`internal/process/failure_test.go`:

```go
package process

import (
	"errors"
	"fmt"
	"testing"
)

func TestFailureError(t *testing.T) {
	f := failf(FailInvalidInput, "validate-root", "root %q is not absolute", "x")
	if f.Class != FailInvalidInput || f.Stage != "validate-root" {
		t.Fatalf("class/stage: %+v", f)
	}
	want := `validate-root: root "x" is not absolute`
	if f.Error() != want {
		t.Fatalf("Error() = %q, want %q", f.Error(), want)
	}
}

func TestAsFailure(t *testing.T) {
	f := failf(FailBlocked, "stop-reprove", "ownership unprovable")
	wrapped := fmt.Errorf("outer: %w", f)
	got, ok := AsFailure(wrapped)
	if !ok || got.Class != FailBlocked {
		t.Fatalf("AsFailure(wrapped) = %v, %v", got, ok)
	}
	if _, ok := AsFailure(errors.New("plain")); ok {
		t.Fatalf("plain error must not classify")
	}
}

func TestNewServiceRejectsRelativeExecutable(t *testing.T) {
	if _, err := NewService("docket"); err == nil {
		t.Fatalf("relative executable path must be rejected")
	}
	svc, err := NewService("/bin/true")
	if err != nil || svc == nil {
		t.Fatalf("absolute path rejected: %v", err)
	}
	if svc.establishTimeout.Seconds() != 10 || svc.stopTermWait.Seconds() != 10 || svc.stopKillWait.Seconds() != 5 {
		t.Fatalf("production bounds wrong: %+v", svc)
	}
}
```

`internal/process/boundary_test.go` — the spec's import-containment guard. It parses every non-test `.go` file in the package directory and refuses any module-internal import; `internal/process` is stdlib-only, which subsumes the 0305–0313 sibling ban:

```go
package process

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestImportBoundaryStdlibOnly proves internal/process imports no
// github.com/danielhanold/docket package — the spec's dependency rule
// (cli -> app -> process -> stdlib). Test files are exempt: they may use
// helpers, but production code may not.
func TestImportBoundaryStdlibOnly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checked++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.Contains(path, ".") && strings.Contains(path, "/") {
				// Stdlib paths have no dot in the first element; any dotted
				// domain import (module-internal or third-party) is a breach.
				t.Errorf("%s imports %q — internal/process is stdlib-only", name, path)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("population floor: no production files checked — the guard is scanning the wrong directory")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/homer/dev/docket/.worktrees/native-process-supervisor-and-local-gate && go test -count=1 ./internal/process/`
Expected: FAIL to build — `NewRunIdentity`, `failf`, `NewService` undefined.

- [ ] **Step 3: Write the implementation**

`internal/process/process.go`:

```go
// Package process owns Docket's native per-run gate supervision: run
// identities, private durable run state, the re-exec'd supervisor, exact
// wait-status terminal records, ownership-gated signalling, and
// abandoned-run recovery. It is repository-independent and imports only
// the Go standard library (guarded by TestImportBoundaryStdlibOnly).
package process

import (
	"path/filepath"
	"time"
)

// Service performs the gate operations. The executable path is an explicit
// dependency: production passes the current docket binary; tests pass the
// test binary, whose TestMain routes supervisor re-execution back into
// this package.
type Service struct {
	executable string

	// Bounds are production constants (spec: 10s establishment, 10s TERM
	// wait, 5s KILL verification). Package-private so tests can shrink
	// them; nothing outside the package can.
	establishTimeout time.Duration
	stopTermWait     time.Duration
	stopKillWait     time.Duration
	pollInterval     time.Duration
}

// NewService builds a Service around the absolute path of the binary to
// re-execute as the supervisor.
func NewService(executable string) (*Service, error) {
	if !filepath.IsAbs(executable) {
		return nil, failf(FailInvalidInput, "new-service", "executable path %q is not absolute", executable)
	}
	return &Service{
		executable:       executable,
		establishTimeout: 10 * time.Second,
		stopTermWait:     10 * time.Second,
		stopKillWait:     5 * time.Second,
		pollInterval:     25 * time.Millisecond,
	}, nil
}

// State is the run-state vocabulary fixed by the spec.
type State string

const (
	StateRunning  State = "running"
	StatePassed   State = "passed"
	StateFailed   State = "failed"
	StateSignaled State = "signaled"
	StateStopped  State = "stopped"
	StateVanished State = "vanished"
)

// Terminal is the exact decoded child wait status. Kind is "exit" or
// "signal"; exactly one of ExitCode/Signal is meaningful per Kind.
type Terminal struct {
	Kind     string
	ExitCode int
	Signal   int
}
```

`internal/process/ids.go`:

```go
package process

import (
	"crypto/rand"
	"encoding/hex"
)

// NewRunIdentity allocates 128 bits of cryptographic randomness for the
// run ID and, independently, 128 for the ownership token, each encoded as
// 32 lowercase hex characters.
func NewRunIdentity() (runID, token string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", failf(FailExternal, "allocate-identity", "reading randomness: %v", err)
	}
	return hex.EncodeToString(buf[:16]), hex.EncodeToString(buf[16:]), nil
}
```

`internal/process/failure.go`:

```go
package process

import (
	"errors"
	"fmt"
)

// FailureClass mirrors the app-result classes an operation failure maps to.
type FailureClass string

const (
	FailInvalidInput FailureClass = "invalid-input"
	FailInvalidState FailureClass = "invalid-state"
	FailBlocked      FailureClass = "blocked"
	FailExternal     FailureClass = "external-failed"
)

// Failure is a typed operation failure with a stable stage and bounded safe
// reason. It never carries argv, environment values, or child output.
type Failure struct {
	Class  FailureClass
	Stage  string
	Reason string
}

func (f *Failure) Error() string { return f.Stage + ": " + f.Reason }

func failf(class FailureClass, stage, format string, a ...any) *Failure {
	return &Failure{Class: class, Stage: stage, Reason: fmt.Sprintf(format, a...)}
}

// AsFailure unwraps err to a *Failure when one is in the chain.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/`
Expected: PASS (boundary test included).

- [ ] **Step 5: Mutation-test the boundary guard**

```bash
cd /Users/homer/dev/docket/.worktrees/native-process-supervisor-and-local-gate
cp internal/process/ids.go internal/process/ids.go.bak
# Plant a module-internal import (used via _ to compile):
perl -0pi -e 's{import \(}{import (\n\t_ "github.com/danielhanold/docket/internal/buildinfo"}' internal/process/ids.go
git diff --stat internal/process/ids.go   # MUST show the file changed
go test -count=1 -run TestImportBoundaryStdlibOnly ./internal/process/
# Expected: FAIL naming ids.go and the import. Then restore:
mv internal/process/ids.go.bak internal/process/ids.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: gofmt and commit**

```bash
gofmt -l internal/process/   # must print nothing
git add internal/process/
git commit -m "feat(0314): internal/process scaffold — identity, failure taxonomy, import boundary"
```

---

### Task 2: Atomic durable records and private file modes

**Files:**
- Create: `internal/process/atomic.go`, `internal/process/records.go`
- Test: `internal/process/atomic_test.go`, `internal/process/records_test.go`

**Interfaces:**
- Consumes: `Failure`/`failf` (Task 1).
- Produces:
  - `writeAtomicJSON(path string, v any) error` — temp-in-same-dir, sync, chmod 0600, rename, dir sync.
  - `ensurePrivateDir(path string) error` — MkdirAll + explicit `Chmod(0700)`.
  - Record schemas (all with `Schema int` fixed at `recordSchema = 1`, JSON tags as shown):
    - `manifestRecord{Schema int "schema"; RunID string "run_id"; Token string "token"; Root string "root"; RunDir string "run_dir"; SupervisorPID int "supervisor_pid"; PGID int "pgid"; SID int "sid"; Phase string "phase"; Cwd string "cwd"; Argv0 string "argv0"; Argc int "argc"; CreatedAt string "created_at"; UpdatedAt string "updated_at"}` — phases: `"allocated"`, `"established"`, `"running"`, `"terminal"`.
    - `terminalRecord{Schema int "schema"; RunID string "run_id"; Kind string "kind"; ExitCode int "exit_code"; Signal int "signal"; RecordedAt string "recorded_at"}`
    - `stopIntentRecord{Schema int "schema"; RunID string "run_id"; Reason string "reason"; RecordedAt string "recorded_at"}`
    - `stoppedRecord{Schema int "schema"; RunID string "run_id"; VerifiedAt string "verified_at"}`
    - `abandonedRecord{Schema int "schema"; RunID string "run_id"; Cause string "cause"; RecordedAt string "recorded_at"}`
    - `failureRecord{Schema int "schema"; RunID string "run_id"; Stage string "stage"; Reason string "reason"; RecordedAt string "recorded_at"}` (supervisor start-failure — distinct from a terminal child record).
  - Readers: `readManifest(runDir string) (*manifestRecord, error)`, `readTerminal(runDir string) (*terminalRecord, error)`, `readStopIntent`, `readStopped`, `readAbandoned`, `readFailureRecord` — each returns `(nil, nil)` on a **cleanly absent** file (`errors.Is(err, fs.ErrNotExist)`), `FailInvalidState` on malformed JSON or `Schema != 1`, and `FailExternal` on any other read error. Absence and unreadability never share a value.
  - File-name constants: `manifestFile = "manifest.json"`, `terminalFile = "terminal.json"`, `stopIntentFile = "stop-intent.json"`, `stoppedFile = "stopped.json"`, `abandonedFile = "abandoned.json"`, `failureFile = "failure.json"`, `liveLockFile = "live.lock"`, `registryLockFile = "registry.lock"`, `stdoutLogFile = "stdout.log"`, `stderrLogFile = "stderr.log"`, `supervisorLogFile = "supervisor.log"`.

- [ ] **Step 1: Write the failing tests**

`internal/process/atomic_test.go` — three tests:

```go
package process

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
)

// TestAtomicWriteModesUnderHostileUmask proves the documented 0700/0600
// modes survive umask 077-style masking because they are chmod'ed, not
// merely requested at creation.
func TestAtomicWriteModesUnderHostileUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := filepath.Join(t.TempDir(), "run")
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, "m.json"), map[string]int{"schema": 1}); err != nil {
		t.Fatal(err)
	}
	di, _ := os.Stat(dir)
	fi, _ := os.Stat(filepath.Join(dir, "m.json"))
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("dir mode %o, want 0700", di.Mode().Perm())
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %o, want 0600", fi.Mode().Perm())
	}
}

// TestAtomicWriteNeverExposesPartialJSON hammers reads during repeated
// replacement: every successful read must parse as complete JSON.
func TestAtomicWriteNeverExposesPartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.json")
	if err := writeAtomicJSON(path, map[string]string{"v": "seed"}); err != nil {
		t.Fatal(err)
	}
	var stop atomic.Bool
	done := make(chan error, 1)
	go func() {
		defer close(done)
		big := make([]byte, 1<<16)
		for i := range big {
			big[i] = 'a'
		}
		for i := 0; i < 200; i++ {
			if err := writeAtomicJSON(path, map[string]string{"v": string(big)}); err != nil {
				done <- err
				return
			}
		}
		stop.Store(true)
	}()
	for !stop.Load() {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read during replacement: %v", err)
		}
		var v map[string]string
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatalf("partial JSON observed (%d bytes): %v", len(raw), err)
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// TestAtomicWriteLeavesNoTempFile — a completed write leaves exactly the
// target in the directory.
func TestAtomicWriteLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := writeAtomicJSON(filepath.Join(dir, "only.json"), map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "only.json" {
		t.Fatalf("directory not clean after write: %v", entries)
	}
}
```

`internal/process/records_test.go` — table-driven round trips plus the three-way read contract:

```go
package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &manifestRecord{Schema: recordSchema, RunID: "aa", Token: "bb", Root: "/r",
		RunDir: dir, SupervisorPID: 42, PGID: 42, SID: 42, Phase: "allocated",
		Cwd: "/w", Argv0: "true", Argc: 1, CreatedAt: "2026-08-16T00:00:00Z", UpdatedAt: "2026-08-16T00:00:00Z"}
	if err := writeAtomicJSON(filepath.Join(dir, manifestFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *out != *in {
		t.Fatalf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}

func TestTerminalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &terminalRecord{Schema: recordSchema, RunID: "aa", Kind: "signal", Signal: 15, RecordedAt: "x"}
	if err := writeAtomicJSON(filepath.Join(dir, terminalFile), in); err != nil {
		t.Fatal(err)
	}
	out, err := readTerminal(dir)
	if err != nil || out.Kind != "signal" || out.Signal != 15 {
		t.Fatalf("got %+v, %v", out, err)
	}
}

// TestReadersThreeWayContract — absent is (nil, nil); malformed JSON and a
// wrong schema are FailInvalidState; the three answers never collapse.
func TestReadersThreeWayContract(t *testing.T) {
	dir := t.TempDir()
	if rec, err := readTerminal(dir); rec != nil || err != nil {
		t.Fatalf("clean absence must be (nil, nil), got %v, %v", rec, err)
	}
	if err := os.WriteFile(filepath.Join(dir, terminalFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerminal(dir); err == nil {
		t.Fatal("malformed record must error")
	} else if f, ok := AsFailure(err); !ok || f.Class != FailInvalidState {
		t.Fatalf("malformed record class = %v", err)
	}
	if err := writeAtomicJSON(filepath.Join(dir, terminalFile),
		map[string]any{"schema": 99, "run_id": "aa", "kind": "exit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerminal(dir); err == nil {
		t.Fatal("unknown schema must be refused, not guessed")
	} else if f, ok := AsFailure(err); !ok || f.Class != FailInvalidState {
		t.Fatalf("unknown schema class = %v", err)
	}
}
```

Add matching round-trip tests (same shape as `TestTerminalRoundTrip`) for `stopIntentRecord`, `stoppedRecord`, `abandonedRecord`, and `failureRecord`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/process/` — Expected: build FAIL (symbols undefined).

- [ ] **Step 3: Write the implementation**

`internal/process/atomic.go`:

```go
package process

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ensurePrivateDir creates path (and parents) and enforces 0700 with an
// explicit chmod — a create-time mode is a request the umask can mask.
func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return failf(FailExternal, "ensure-dir", "creating %s: %v", filepath.Base(path), err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return failf(FailExternal, "ensure-dir", "chmod 0700 %s: %v", filepath.Base(path), err)
	}
	return nil
}

// writeAtomicJSON writes v as JSON at path via a same-directory temp file,
// fsync, chmod 0600, atomic rename, and directory fsync. A reader sees a
// complete old or new document, never a partial one.
func writeAtomicJSON(path string, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return failf(FailExternal, "atomic-write", "encoding %s: %v", filepath.Base(path), err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return failf(FailExternal, "atomic-write", "temp for %s: %v", filepath.Base(path), err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after successful rename
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "writing %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "syncing %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return failf(FailExternal, "atomic-write", "chmod %s: %v", filepath.Base(path), err)
	}
	if err := tmp.Close(); err != nil {
		return failf(FailExternal, "atomic-write", "closing %s: %v", filepath.Base(path), err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return failf(FailExternal, "atomic-write", "renaming into %s: %v", filepath.Base(path), err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return failf(FailExternal, "atomic-write", "opening dir for sync: %v", err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return failf(FailExternal, "atomic-write", "dir sync: %v", err)
	}
	return nil
}
```

`internal/process/records.go`: the six struct definitions and constants exactly as the Interfaces block specifies, plus one generic reader used by all six named readers:

```go
package process

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const recordSchema = 1

const (
	manifestFile      = "manifest.json"
	terminalFile      = "terminal.json"
	stopIntentFile    = "stop-intent.json"
	stoppedFile       = "stopped.json"
	abandonedFile     = "abandoned.json"
	failureFile       = "failure.json"
	liveLockFile      = "live.lock"
	registryLockFile  = "registry.lock"
	stdoutLogFile     = "stdout.log"
	stderrLogFile     = "stderr.log"
	supervisorLogFile = "supervisor.log"
)

// readRecord reads one schema-1 JSON record. Three outcomes, never
// collapsed: cleanly absent -> (false, nil); malformed or wrong schema ->
// FailInvalidState; any other filesystem error -> FailExternal.
func readRecord(runDir, name string, v any, schema func() int) (bool, error) {
	raw, err := os.ReadFile(filepath.Join(runDir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, failf(FailExternal, "read-record", "reading %s: %v", name, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, failf(FailInvalidState, "read-record", "%s is not a valid record: %v", name, err)
	}
	if schema() != recordSchema {
		return false, failf(FailInvalidState, "read-record", "%s has unrecognized schema %d", name, schema())
	}
	return true, nil
}

func readManifest(runDir string) (*manifestRecord, error) {
	var r manifestRecord
	ok, err := readRecord(runDir, manifestFile, &r, func() int { return r.Schema })
	if !ok {
		return nil, err
	}
	return &r, nil
}
```

…and the same four-line pattern for `readTerminal`, `readStopIntent`, `readStopped`, `readAbandoned`, `readFailureRecord`. (Struct fields per the Interfaces block, each field with the exact JSON tag listed there.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the chmod guard**

```bash
cp internal/process/atomic.go internal/process/atomic.go.bak
perl -0pi -e 's/if err := os\.Chmod\(path, 0o700\); err != nil \{/if false {\n\terr := error(nil); _ = err/' internal/process/atomic.go
git diff --stat internal/process/atomic.go   # MUST show a change
go test -count=1 -run TestAtomicWriteModesUnderHostileUmask ./internal/process/
# Expected: FAIL ("dir mode 700" no longer guaranteed under umask 077).
mv internal/process/atomic.go.bak internal/process/atomic.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): atomic private run-state records with three-way read contract"
```

---

### Task 3: Request and path validation

**Files:**
- Create: `internal/process/paths.go`
- Test: `internal/process/paths_test.go`

**Interfaces:**
- Consumes: `Failure` taxonomy (Task 1), file constants (Task 2).
- Produces:
  - `validateLaunchRequest(req LaunchRequest) error` — `FailInvalidInput` unless: `Root` absolute; `Cwd` absolute and an existing directory (`os.Stat`); `len(Argv) >= 1` with non-empty `Argv[0]`.
  - `resolveRunDir(root, runDir string) (canonicalRunDir string, runID string, err error)` — the ownership conjunction's clause 1: `runDir` absolute; its parent, after `filepath.EvalSymlinks` on **both** root and parent, equals the canonical root (every hop canonicalised — an absolute symlink target is still a spelling); `os.Lstat(runDir)` is a real directory, not a symlink; base name is 32 lowercase hex (the run-ID shape). Returns `FailInvalidInput` for shape violations, `FailBlocked` for a symlink at the run slot or containment breach.
  - `boundReason(s string) string` — flattens whitespace runs to single spaces, truncates to 200 **runes** (never a byte cut — a byte-offset cut through text splits runes), strips control characters.
  - `LaunchRequest{Root, Cwd string; Argv []string}` moves here (referenced by Task 6).

- [ ] **Step 1: Write the failing tests**

`internal/process/paths_test.go`:

```go
package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLaunchRequest(t *testing.T) {
	cwd := t.TempDir()
	ok := LaunchRequest{Root: t.TempDir(), Cwd: cwd, Argv: []string{"/bin/true"}}
	if err := validateLaunchRequest(ok); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	cases := map[string]LaunchRequest{
		"relative root":  {Root: "rel", Cwd: cwd, Argv: []string{"x"}},
		"relative cwd":   {Root: t.TempDir(), Cwd: "rel", Argv: []string{"x"}},
		"missing cwd":    {Root: t.TempDir(), Cwd: filepath.Join(cwd, "absent"), Argv: []string{"x"}},
		"empty argv":     {Root: t.TempDir(), Cwd: cwd, Argv: nil},
		"empty argv0":    {Root: t.TempDir(), Cwd: cwd, Argv: []string{""}},
	}
	for name, req := range cases {
		err := validateLaunchRequest(req)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if f, ok := AsFailure(err); !ok || f.Class != FailInvalidInput {
			t.Errorf("%s: class = %v, want invalid-input", name, err)
		}
	}
}

func TestResolveRunDirContainment(t *testing.T) {
	root := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	real := filepath.Join(root, id)
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	got, gotID, err := resolveRunDir(root, real)
	if err != nil || gotID != id || !strings.HasSuffix(got, id) {
		t.Fatalf("valid run dir refused: %v %v %v", got, gotID, err)
	}
	// A symlink at the run slot is refused as blocked.
	link := filepath.Join(root, "fedcba9876543210fedcba9876543210")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveRunDir(root, link); err == nil {
		t.Fatal("symlink run slot accepted")
	} else if f, _ := AsFailure(err); f == nil || f.Class != FailBlocked {
		t.Fatalf("symlink class = %v", err)
	}
	// A directory outside the root is refused even when its NAME looks right.
	outside := filepath.Join(t.TempDir(), id)
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveRunDir(root, outside); err == nil {
		t.Fatal("escaped run dir accepted")
	}
	// A non-run-id name inside the root is invalid input.
	odd := filepath.Join(root, "not-a-run-id")
	os.Mkdir(odd, 0o700)
	if _, _, err := resolveRunDir(root, odd); err == nil {
		t.Fatal("non-hex run dir accepted")
	}
}

// TestResolveRunDirCanonicalisesRoot — on macOS /tmp is a symlink to
// /private/tmp, so a root spelled through the symlink and a run dir spelled
// physically must still match. TempDir already exercises this on darwin;
// build the two spellings explicitly so the test bites on linux too.
func TestResolveRunDirCanonicalisesRoot(t *testing.T) {
	base := t.TempDir()
	physical := filepath.Join(base, "phys")
	os.Mkdir(physical, 0o700)
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Skip("symlinks unavailable")
	}
	id := "0123456789abcdef0123456789abcdef"
	os.Mkdir(filepath.Join(physical, id), 0o700)
	if _, _, err := resolveRunDir(alias, filepath.Join(physical, id)); err != nil {
		t.Fatalf("aliased root vs physical run dir must canonicalise equal: %v", err)
	}
}

func TestBoundReason(t *testing.T) {
	if got := boundReason("a\t b\nc\x00d"); got != "a b cd" {
		t.Fatalf("flatten: %q", got)
	}
	long := strings.Repeat("é", 300)
	got := boundReason(long)
	if len([]rune(got)) != 200 {
		t.Fatalf("rune bound: %d runes", len([]rune(got)))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/process/` — Expected: build FAIL.

- [ ] **Step 3: Write the implementation** in `internal/process/paths.go` exactly per the Interfaces block. Containment core:

```go
func resolveRunDir(root, runDir string) (string, string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(runDir) {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "root and run dir must be absolute")
	}
	canonRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "canonicalising root: %v", err)
	}
	li, err := os.Lstat(runDir)
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "inspecting run dir: %v", err)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return "", "", failf(FailBlocked, "resolve-run-dir", "run slot is a symlink")
	}
	if !li.IsDir() {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "run path is not a directory")
	}
	canonParent, err := filepath.EvalSymlinks(filepath.Dir(runDir))
	if err != nil {
		return "", "", failf(FailExternal, "resolve-run-dir", "canonicalising parent: %v", err)
	}
	if canonParent != canonRoot {
		return "", "", failf(FailBlocked, "resolve-run-dir", "run dir is not an immediate child of the root")
	}
	id := filepath.Base(runDir)
	if !runIDPattern.MatchString(id) {
		return "", "", failf(FailInvalidInput, "resolve-run-dir", "directory name is not a run id")
	}
	return filepath.Join(canonParent, id), id, nil
}
```

with `var runIDPattern = regexp.MustCompile("^[0-9a-f]{32}$")` and `boundReason` using `strings.Fields` + rune-slice truncation.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the symlink refusal**

```bash
cp internal/process/paths.go internal/process/paths.go.bak
perl -0pi -e 's/if li\.Mode\(\)&os\.ModeSymlink != 0 \{/if false {/' internal/process/paths.go
git diff --stat internal/process/paths.go
go test -count=1 -run TestResolveRunDirContainment ./internal/process/
# Expected: FAIL ("symlink run slot accepted" — EvalSymlinks on the parent
# alone must NOT be what saves us; the Lstat refusal is load-bearing).
mv internal/process/paths.go.bak internal/process/paths.go
go test -count=1 ./internal/process/
```

If this mutation does **not** redden, that is a finding about the code (the two guards collapse), not a residual — investigate before proceeding.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): launch validation, run-dir containment, and reason bounding"
```

---

### Task 4: Platform seam, locks, and three-way liveness

**Files:**
- Create: `internal/process/platform.go`, `internal/process/platform_darwin.go`, `internal/process/platform_linux.go`, `internal/process/platform_unsupported.go`, `internal/process/lock.go`
- Test: `internal/process/platform_test.go`, `internal/process/lock_test.go`

**Interfaces:**
- Consumes: `Failure` taxonomy.
- Produces:
  - `type probeAnswer int` with `probeLive`, `probeAbsent`, `probeUnknown` — the three-way vocabulary every liveness question returns. **`probeUnknown` never shares a branch with `probeAbsent` when the other branch signals or deletes.**
  - Shared file `platform.go`: `func processAlive(pid int) probeAnswer` (`syscall.Kill(pid, 0)`: nil→live, `ESRCH`→absent, anything else→unknown), `func groupAlive(pgid int) probeAnswer` (`syscall.Kill(-pgid, 0)`, same mapping), `func getPGID(pid int) (int, probeAnswer)` (`syscall.Getpgid`), `func signalGroup(pgid int, sig syscall.Signal) error`, and `func sessionAttrs() *syscall.SysProcAttr` returning `&syscall.SysProcAttr{Setsid: true}`.
  - Per-platform files (build-tagged `//go:build darwin` / `//go:build linux`): `func getSID(pid int) (int, probeAnswer)` via `syscall.Syscall(syscall.SYS_GETSID, uintptr(pid), 0, 0)` — nonzero errno `ESRCH`→absent, other errno→unknown.
  - `platform_unsupported.go` (`//go:build !darwin && !linux`): `getSID` returns `(0, probeUnknown)` and a package-level `const platformSupported = false` (darwin/linux files declare `= true`); `Launch` (Task 6) refuses with `FailExternal` "unsupported operating system" when false — explicit failure, never weaker detachment.
  - `lock.go`: `acquireFlock(path string) (*os.File, error)` (open O_CREATE|O_RDWR 0600 + explicit `Chmod(0600)` + `syscall.Flock(fd, LOCK_EX|LOCK_NB)`; on `EWOULDBLOCK` returns `FailBlocked` "lock held"); `probeFlock(path string) (held bool, answer probeAnswer)` — try `LOCK_EX|LOCK_NB` on a fresh descriptor: acquired→(false, probeAbsent) after immediate unlock+close (supervisor gone, cleanly); `EWOULDBLOCK`→(true, probeLive); missing file→(false, probeAbsent); any other error→(false, probeUnknown).
  - `identityConjunction(m *manifestRecord, selfPGID int) error` — clauses 3–5 of the spec's ownership conjunction: lock held (`probeFlock`), `m.SupervisorPID > 1`, `getPGID == m.PGID == m.SupervisorPID`, `getSID == m.SID == m.SupervisorPID`, and `m.PGID != selfPGID`; returns nil when all hold, `FailBlocked` naming the first unprovable clause otherwise. (Clauses 1–2 — containment and manifest/token agreement — live in `resolveRunDir` + callers.)

- [ ] **Step 1: Write the failing tests**

`internal/process/platform_test.go` — real processes, no mocks:

```go
package process

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// startSleeper starts a real child that blocks until killed; returns pid.
func startSleeper(t *testing.T) (*exec.Cmd, int) {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "300")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	return cmd, cmd.Process.Pid
}

func TestProcessAliveThreeWay(t *testing.T) {
	cmd, pid := startSleeper(t)
	if got := processAlive(pid); got != probeLive {
		t.Fatalf("live pid probed %v", got)
	}
	cmd.Process.Kill()
	cmd.Wait() // reap: an unreaped zombie still probes live
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) != probeAbsent {
		if time.Now().After(deadline) {
			t.Fatalf("reaped pid never probed absent")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// pid 1 belongs to root: kill(1, 0) is EPERM for a non-root test run —
	// the canonical "unknown, not absent" case.
	if got := processAlive(1); got == probeAbsent {
		t.Fatalf("EPERM collapsed into clean absence")
	}
}

func TestGetSIDAndPGIDReadLiveFacts(t *testing.T) {
	_, pid := startSleeper(t)
	pgid, ans := getPGID(pid)
	if ans != probeLive || pgid <= 0 {
		t.Fatalf("getPGID: %d %v", pgid, ans)
	}
	sid, ans := getSID(pid)
	if ans != probeLive || sid <= 0 {
		t.Fatalf("getSID: %d %v", sid, ans)
	}
}

func TestSessionAttrsCreateNewSession(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "300")
	cmd.SysProcAttr = sessionAttrs()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	pid := cmd.Process.Pid
	pgid, _ := getPGID(pid)
	sid, _ := getSID(pid)
	if pgid != pid || sid != pid {
		t.Fatalf("want pid==pgid==sid, got pid=%d pgid=%d sid=%d", pid, pgid, sid)
	}
	self, _ := syscall.Getpgid(0)
	if pgid == self {
		t.Fatalf("child shares the test's own group")
	}
}
```

`internal/process/lock_test.go`:

```go
package process

import (
	"path/filepath"
	"testing"
)

func TestFlockLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), liveLockFile)
	f, err := acquireFlock(path)
	if err != nil {
		t.Fatal(err)
	}
	if held, ans := probeFlock(path); !held || ans != probeLive {
		t.Fatalf("held lock probed %v %v", held, ans)
	}
	if _, err := acquireFlock(path); err == nil {
		t.Fatal("second acquisition of a held lock succeeded")
	} else if fl, _ := AsFailure(err); fl == nil || fl.Class != FailBlocked {
		t.Fatalf("contended class = %v", err)
	}
	f.Close() // kernel releases on close
	if held, ans := probeFlock(path); held || ans != probeAbsent {
		t.Fatalf("released lock probed %v %v", held, ans)
	}
	// A missing lock file is clean absence, not an error.
	if held, ans := probeFlock(filepath.Join(t.TempDir(), "never")); held || ans != probeAbsent {
		t.Fatalf("missing lock file probed %v %v", held, ans)
	}
}

func TestIdentityConjunctionRejectsOwnGroup(t *testing.T) {
	// A manifest describing the OBSERVER's own group must never pass —
	// clause 5 exists so stop cannot signal itself.
	self := syscall_Getpid()
	pgid, _ := getPGID(self)
	sid, _ := getSID(self)
	dir := t.TempDir()
	f, err := acquireFlock(filepath.Join(dir, liveLockFile))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := &manifestRecord{Schema: recordSchema, RunID: "aa", Token: "bb", RunDir: dir,
		SupervisorPID: self, PGID: pgid, SID: sid}
	if err := identityConjunction(m, pgid); err == nil {
		t.Fatal("observer's own group passed the conjunction")
	}
}
```

(`syscall_Getpid` is `func syscall_Getpid() int { return syscall.Getpid() }` in the test file, or call `os.Getpid()` — implementer's choice; keep the assertion.)

Note `identityConjunction` takes the run dir via `m.RunDir` for the lock probe — set that field.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/process/` — Expected: build FAIL.

- [ ] **Step 3: Implement** the five files per the Interfaces block. `identityConjunction`:

```go
// identityConjunction proves clauses 3-5 of the spec's ownership
// conjunction for a live run: lock held by a live supervisor whose pid is
// >1 and still equals its live pgid and sid, and whose group is not the
// observer's own. Any unprovable read is FailBlocked — never treated as
// absence, never permission to signal.
func identityConjunction(m *manifestRecord, selfPGID int) error {
	held, ans := probeFlock(filepath.Join(m.RunDir, liveLockFile))
	if ans == probeUnknown {
		return failf(FailBlocked, "identity", "live lock unprobeable")
	}
	if !held {
		return failf(FailBlocked, "identity", "live lock not held")
	}
	if m.SupervisorPID <= 1 {
		return failf(FailBlocked, "identity", "recorded pid %d is not a valid supervisor", m.SupervisorPID)
	}
	pgid, pans := getPGID(m.SupervisorPID)
	sid, sans := getSID(m.SupervisorPID)
	if pans != probeLive || sans != probeLive {
		return failf(FailBlocked, "identity", "supervisor process facts unprovable")
	}
	if pgid != m.PGID || sid != m.SID || m.PGID != m.SupervisorPID || m.SID != m.SupervisorPID {
		return failf(FailBlocked, "identity", "recorded identity no longer matches live process facts")
	}
	if m.PGID == selfPGID {
		return failf(FailBlocked, "identity", "recorded group is the observer's own")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the EPERM branch**

```bash
cp internal/process/platform.go internal/process/platform.go.bak
# Collapse unknown into absent in processAlive:
perl -0pi -e 's/return probeUnknown/return probeAbsent/ if $. == 0' internal/process/platform.go
git diff internal/process/platform.go   # MUST show exactly the processAlive arm changed; if perl touched more than one site, restore and mutate by hand-editing only processAlive
go test -count=1 -run TestProcessAliveThreeWay ./internal/process/
# Expected: FAIL ("EPERM collapsed into clean absence").
mv internal/process/platform.go.bak internal/process/platform.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: Verify the linux tuple still cross-compiles**

Run: `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./...`
Expected: clean build (proves `SYS_GETSID` spelling exists on linux; the suite's `TestCrossCompileApprovedTargets` will keep this pinned).

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): platform seam, flock lifecycle, three-way liveness, identity conjunction"
```

---

### Task 5: Supervisor mode — re-exec entry, session establishment, env/fd hygiene

**Files:**
- Create: `internal/process/supervisor.go`, `internal/app/gate_supervisor.go`
- Modify: `internal/cli/root.go` (top of `run`)
- Test: `internal/process/supervisor_test.go`, `internal/process/main_test.go` (TestMain hook)

**Interfaces:**
- Consumes: records/atomic (Task 2), platform/lock (Task 4).
- Produces:
  - Env/fd contract constants in `supervisor.go`: `supervisorRunDirEnv = "DOCKET_GATE_SUPERVISOR_RUN_DIR"`, `supervisorLockFD = 3` (first `ExtraFiles` slot), `supervisorHandshakeFD = 4` (second).
  - `func SupervisorRequested() bool` — true iff `os.Getenv(supervisorRunDirEnv) != ""`.
  - `func RunSupervisorFromEnv() int` — the whole supervisor lifetime; returns process exit code (0 after a durable terminal record, 1 on supervisor failure). Never calls `os.Exit`.
  - Handshake protocol on fd 4 (write side in supervisor): single lines `established\n`, `running\n`, `terminal\n`, `failed\n`, then close. The launcher (Task 6) treats the pipe as a wake-up and re-reads the atomic records as truth.
  - `internal/app/gate_supervisor.go`: `func MaybeRunGateSupervisor() (code int, ok bool)` — `ok` false unless `process.SupervisorRequested()`.
  - `internal/cli/root.go`: first statement of `run(...)` becomes:

    ```go
    // Package-private supervisor re-execution: when the launcher re-execs
    // this binary as a gate supervisor it must never parse public flags,
    // print protocol documents, or read stdin — it IS the durable waiter.
    if code, ok := app.MaybeRunGateSupervisor(); ok {
        return code
    }
    ```

Supervisor sequence inside `RunSupervisorFromEnv` (all in `internal/process`):

1. Read run dir from env; adopt the inherited lock file (`os.NewFile(uintptr(supervisorLockFD), liveLockFile)`) and handshake pipe (`os.NewFile(uintptr(supervisorHandshakeFD), "handshake")`); immediately `syscall.CloseOnExec` **both** fds — `ExtraFiles` arrive without CLOEXEC and must never leak into the user command.
2. Open `supervisor.log` (append, 0600 + explicit chmod) and route all supervisor diagnostics there; nothing to stdout/stderr.
3. Prove `pid == pgid == sid` via `getPGID`/`getSID` on `os.Getpid()`; on failure write a `failureRecord`, send `failed\n`, return 1.
4. Re-read the manifest, update `SupervisorPID/PGID/SID`, `Phase: "established"`, `UpdatedAt`; `writeAtomicJSON`; send `established\n`.
5. Install `signal.Notify` for `SIGTERM` and `SIGINT` into a channel the supervisor merely records to `supervisor.log` — the supervisor survives the group-directed TERM long enough to wait and record; the child keeps default disposition.
6. Open `stdout.log`/`stderr.log` (create 0600 + chmod). Build `exec.Command(argv[0], argv[1:]...)` from the launch argv (passed via a JSON `argv` array in the manifest? **No** — the manifest deliberately stores only a bounded descriptor). The full argv arrives via a second private env var `DOCKET_GATE_SUPERVISOR_ARGV`, JSON-encoded; the supervisor decodes it, then **removes both private env vars** from the child env: `cmd.Env = envWithout(os.Environ(), supervisorRunDirEnv, supervisorArgvEnv)`. Set `cmd.Dir` from the manifest `Cwd`, `cmd.Stdin, _ = os.Open(os.DevNull)`, `cmd.Stdout/Stderr` to the two log files. **No `SysProcAttr`** — the child joins the supervisor's group.
7. `cmd.Start()`; on start error write `failureRecord{Stage: "start-command"}`, send `failed\n`, return 1 — never a fabricated terminal record. On success update manifest `Phase: "running"`, send `running\n`.
8. `cmd.Wait()`; decode `cmd.ProcessState.Sys().(syscall.WaitStatus)`: `Exited()` → `terminalRecord{Kind: "exit", ExitCode: ws.ExitStatus()}`; `Signaled()` → `{Kind: "signal", Signal: int(ws.Signal())}`. `writeAtomicJSON(terminal.json)`, update manifest `Phase: "terminal"`, send `terminal\n`, close pipe, **then** close the lock file (record durable before the lock releases), return 0.
- Also produce `envWithout(env []string, keys ...string) []string` and `supervisorArgvEnv = "DOCKET_GATE_SUPERVISOR_ARGV"`.

- [ ] **Step 1: Write the TestMain hook and failing test**

`internal/process/main_test.go`:

```go
package process

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestMain routes the two re-exec roles of the test binary:
//   - supervisor mode (env var set by Launch) -> RunSupervisorFromEnv
//   - child helper mode (argv marker) -> runTestHelper
// go test itself never sets either, so ordinary runs fall through to m.Run.
func TestMain(m *testing.M) {
	if SupervisorRequested() {
		os.Exit(RunSupervisorFromEnv())
	}
	if len(os.Args) > 1 && os.Args[1] == "gate-test-helper" {
		os.Exit(runTestHelper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// runTestHelper is the purpose-built child command. Modes:
//   exit <n>            exit with code n
//   emit <out> <err>    write out to stdout, err to stderr, exit 0
//   sleep               block forever (killed by the test or by stop)
//   ignore-term <path>  ignore SIGTERM, write "ready" to path, block
//   read-stdin          exit 0 iff stdin is at EOF immediately, else 3
func runTestHelper(args []string) int {
	if len(args) == 0 {
		return 90
	}
	switch args[0] {
	case "exit":
		n, _ := strconv.Atoi(args[1])
		return n
	case "emit":
		fmt.Fprint(os.Stdout, args[1])
		fmt.Fprint(os.Stderr, args[2])
		return 0
	case "sleep":
		select {}
	case "ignore-term":
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(args[1], []byte("ready"), 0o600); err != nil {
			return 91
		}
		select {}
	case "read-stdin":
		buf := make([]byte, 1)
		if n, _ := os.Stdin.Read(buf); n != 0 {
			return 3
		}
		return 0
	}
	return 92
}

// helperArgv builds the child argv for a helper mode.
func helperArgv(t *testing.T, mode string, extra ...string) []string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return append([]string{exe, "gate-test-helper", mode}, extra...)
}

// waitFor polls fn under a generous outer deadline; correctness rests on
// the state transition, never on the interval.
func waitFor(t *testing.T, what string, deadline time.Duration, fn func() bool) {
	t.Helper()
	end := time.Now().Add(deadline)
	for !fn() {
		if time.Now().After(end) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
```

`internal/process/supervisor_test.go` (unit-level pieces that don't need Launch):

```go
package process

import (
	"os"
	"testing"
)

func TestEnvWithout(t *testing.T) {
	in := []string{"A=1", supervisorRunDirEnv + "=/x", "B=2", supervisorArgvEnv + `=["y"]`}
	out := envWithout(in, supervisorRunDirEnv, supervisorArgvEnv)
	if len(out) != 2 || out[0] != "A=1" || out[1] != "B=2" {
		t.Fatalf("envWithout = %v", out)
	}
}

func TestSupervisorRequested(t *testing.T) {
	if SupervisorRequested() {
		t.Fatal("requested with env unset")
	}
	t.Setenv(supervisorRunDirEnv, "/somewhere")
	if !SupervisorRequested() {
		t.Fatal("not requested with env set")
	}
}
```

(The full supervisor behavior is exercised end-to-end through `Launch` in Tasks 6–7; the session/env/fd proofs live there because the supervisor only runs re-exec'd.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/process/` — Expected: build FAIL (`RunSupervisorFromEnv`, `envWithout` undefined).

- [ ] **Step 3: Implement** `internal/process/supervisor.go` per the sequence above, `internal/app/gate_supervisor.go`:

```go
package app

import "github.com/danielhanold/docket/internal/process"

// MaybeRunGateSupervisor runs the package-private gate supervisor when this
// process was re-executed as one. cli.Run calls it before Cobra parses
// anything: the supervisor is not a public command and must never touch the
// protocol streams.
func MaybeRunGateSupervisor() (int, bool) {
	if !process.SupervisorRequested() {
		return 0, false
	}
	return process.RunSupervisorFromEnv(), true
}
```

and the two-line hook at the top of `cli/run` shown in the Interfaces block.

- [ ] **Step 4: Run tests + whole-module build**

Run: `go test -count=1 ./internal/process/ ./internal/app/ ./internal/cli/ && go vet ./...`
Expected: PASS. (`cmd/docket` exit-sites guard still green: `go test -count=1 ./cmd/docket/ -run TestExit` or the package's actual exit-site test name — run the whole package.)

- [ ] **Step 5: Commit**

```bash
gofmt -l internal/ && git add internal/ && git commit -m "feat(0314): package-private supervisor mode with env/fd contract and pre-Cobra hook"
```

---### Task 6: Launch — ordered state machine, handshake, survival, logs

**Files:**
- Create: `internal/process/launch.go`
- Test: `internal/process/launch_test.go`

**Interfaces:**
- Consumes: everything above.
- Produces:
  - `type LaunchOutcome struct { RunID, RunDir, StdoutLog, StderrLog string; State State; Terminal *Terminal }`
  - `func (s *Service) Launch(req LaunchRequest) (*LaunchOutcome, error)`
  - `func newTestService(t *testing.T) *Service` in `launch_test.go`: builds a Service on `os.Executable()` with `establishTimeout` kept at 10s (generous outer bound; the handshake pipe makes success fast) — later tests shrink bounds per-case.

Launch algorithm (spec order is load-bearing):

1. `validateLaunchRequest`; `platformSupported` false → `FailExternal`. All validation precedes any filesystem create.
2. `ensurePrivateDir(root)`; under `acquireFlock(root/registry.lock)` (released at the end of allocation, never held for the gate): `NewRunIdentity()`, `ensurePrivateDir(runDir)`, `acquireFlock(runDir/live.lock)` (this descriptor is handed to the supervisor), write `allocated` manifest (`SupervisorPID/PGID/SID` zero, `Argv0: filepath.Base(req.Argv[0])`, `Argc: len(req.Argv)`, timestamps `time.Now().UTC().Format(time.RFC3339)`). **Lock before manifest**: no manifest is visible before a live lock is held, so recovery cannot misread a half-published launch.
3. Build the supervisor cmd: `exec.Command(s.executable)` with `Env = append(os.Environ(), supervisorRunDirEnv+"="+runDir, supervisorArgvEnv+"="+string(argvJSON))`, `SysProcAttr = sessionAttrs()`, `Stdin` from `os.DevNull`, `Stdout`/`Stderr` to the opened `supervisor.log` file, `ExtraFiles = []*os.File{lockFile, pipeW}` (`os.Pipe()` for the handshake). Start; close the launcher's `pipeW` and `lockFile` copies (the flock survives on the supervisor's inherited descriptor).
4. Read handshake lines from `pipeR` with a deadline of `s.establishTimeout` (wrap in a goroutine feeding a channel; `bufio.Scanner`). On `running`: check `readTerminal` once — a fast command may already be terminal; return `StateRunning` or the decoded terminal state (`terminalState` helper below). On `failed`: read `failure.json`, return `FailExternal` with its stage/reason. On EOF before `running`: re-read `terminal.json` (present → return its state — the command ran and finished); else `failure.json`; else `FailExternal` "supervisor exited before establishment". On deadline: **bounded ownership-checked teardown** — re-read manifest; if `identityConjunction` passes, `signalGroup(m.PGID, SIGKILL)` and wait up to `s.stopKillWait` for `groupAlive == probeAbsent`; return `FailExternal` "establishment timed out". If the conjunction is unprovable, signal nothing and return `FailBlocked` — never a usable handle while an unaddressable command might still start.
5. `terminalState(term *terminalRecord, stopIntentPresent bool) State`: `exit`+code 0→`StatePassed`; `exit`+nonzero→`StateFailed`; `signal`+stop intent→`StateStopped`; `signal` without→`StateSignaled`. (Shared helper — observe and stop reuse it; define it in `launch.go` or `observe.go`, once.)

- [ ] **Step 1: Write the failing tests**

`internal/process/launch_test.go`:

```go
package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(exe)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func launchHelper(t *testing.T, svc *Service, root string, mode string, extra ...string) *LaunchOutcome {
	t.Helper()
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: helperArgv(t, mode, extra...)})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return out
}

func observeUntilTerminal(t *testing.T, svc *Service, runDir string) *Observation {
	t.Helper()
	var obs *Observation
	waitFor(t, "terminal state", 30*time.Second, func() bool {
		o, err := svc.Observe(runDir)
		if err != nil {
			t.Fatalf("Observe: %v", err)
		}
		obs = o
		return o.State != StateRunning
	})
	return obs
}

// TestLaunchEstablishesAddressableSession — the handshake returns only
// after a live pid==pgid==sid supervisor exists, distinct from ours.
func TestLaunchEstablishesAddressableSession(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	if out.State != StateRunning {
		t.Fatalf("state = %v", out.State)
	}
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatal(err)
	}
	if m.Phase != "running" {
		t.Fatalf("phase = %q", m.Phase)
	}
	if m.SupervisorPID != m.PGID || m.PGID != m.SID {
		t.Fatalf("not a session leader: %+v", m)
	}
	self, _ := syscall.Getpgid(0)
	if m.PGID == self {
		t.Fatalf("supervisor in the launcher's own group")
	}
	// Live facts agree with the record.
	if err := identityConjunction(m, self); err != nil {
		t.Fatalf("conjunction on a live run: %v", err)
	}
	// Modes: run dir 0700, records 0600.
	di, _ := os.Stat(out.RunDir)
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("run dir mode %o", di.Mode().Perm())
	}
	// Teardown for hygiene.
	if _, err := svc.Stop(out.RunDir, "test teardown"); err != nil {
		t.Fatalf("teardown stop: %v", err)
	}
}

// TestGateSurvivesLauncherExit — the launcher is a REAL separate process
// (this test binary in a re-exec'd "go test -run" of the launcher helper is
// overkill; instead run the launch through a subprocess `go run`-style
// re-exec of the test binary's helper is not available, so we approximate
// the spec's requirement precisely: launch from a child process that exits).
func TestGateSurvivesLauncherExit(t *testing.T) {
	root := t.TempDir()
	exe, _ := os.Executable()
	// The test binary re-runs ONLY this launcher body via helper mode
	// "launch": performs svc.Launch(sleep) against root, prints run dir.
	cmd := exec.Command(exe, "gate-test-helper", "launch", root)
	outBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("launcher subprocess: %v", err)
	}
	runDir := strings.TrimSpace(string(outBytes))
	// The launcher subprocess has fully exited (cmd.Output returned and
	// reaped it); its process group is gone. The gate must still be live.
	svc := newTestService(t)
	obs, err := svc.Observe(runDir)
	if err != nil {
		t.Fatalf("Observe after launcher death: %v", err)
	}
	if obs.State != StateRunning {
		t.Fatalf("gate did not survive the launcher: %v", obs.State)
	}
	svc.Stop(runDir, "test teardown")
}

// TestLaunchStreamsAndStdin — stdout/stderr byte-exact separate durable
// files; child stdin is at EOF.
func TestLaunchStreamsAndStdin(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "emit", "OUT-BYTES", "ERR-BYTES")
	obs := observeUntilTerminal(t, svc, out.RunDir)
	if obs.State != StatePassed {
		t.Fatalf("emit helper: %v", obs.State)
	}
	so, _ := os.ReadFile(filepath.Join(out.RunDir, stdoutLogFile))
	se, _ := os.ReadFile(filepath.Join(out.RunDir, stderrLogFile))
	if string(so) != "OUT-BYTES" || string(se) != "ERR-BYTES" {
		t.Fatalf("streams not byte-exact/separate: out=%q err=%q", so, se)
	}
	fi, _ := os.Stat(filepath.Join(out.RunDir, stdoutLogFile))
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("stdout.log mode %o", fi.Mode().Perm())
	}
	out2 := launchHelper(t, svc, t.TempDir(), "read-stdin")
	if obs := observeUntilTerminal(t, svc, out2.RunDir); obs.State != StatePassed {
		t.Fatalf("stdin not closed: %v", obs.State)
	}
}

// TestLaunchStripsSupervisorEnvAndFDs — the child sees neither private
// env var; the lock fd is not inherited (CLOEXEC).
func TestLaunchStripsSupervisorEnv(t *testing.T) {
	svc := newTestService(t)
	// helper mode "env-check": exits 0 iff both private vars are unset and
	// fd 3 is closed; add it to runTestHelper in main_test.go:
	//   case "env-check":
	//       if os.Getenv("DOCKET_GATE_SUPERVISOR_RUN_DIR") != "" ||
	//          os.Getenv("DOCKET_GATE_SUPERVISOR_ARGV") != "" { return 4 }
	//       if _, err := syscall.Getpgid(0); err != nil { return 93 } // sanity
	//       if _, serr := os.NewFile(3, "probe").Stat(); serr == nil { return 5 }
	//       return 0
	out := launchHelper(t, svc, t.TempDir(), "env-check")
	if obs := observeUntilTerminal(t, svc, out.RunDir); obs.State != StatePassed {
		t.Fatalf("supervisor leaked env or fds into the child: %v", obs.State)
	}
}

// TestLaunchFastExit — a command that finishes inside the establishment
// window returns its exact terminal state from Launch itself.
func TestLaunchFastExit(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	out, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: helperArgv(t, "exit", "0")})
	if err != nil {
		t.Fatal(err)
	}
	if out.State == StateRunning {
		// Racing running->terminal is legal; observe must converge.
		if obs := observeUntilTerminal(t, svc, out.RunDir); obs.State != StatePassed {
			t.Fatalf("fast exit converged to %v", obs.State)
		}
	} else if out.State != StatePassed {
		t.Fatalf("fast exit state %v", out.State)
	}
}

func TestLaunchRejectsBeforeCreating(t *testing.T) {
	svc := newTestService(t)
	root := filepath.Join(t.TempDir(), "root-not-yet")
	_, err := svc.Launch(LaunchRequest{Root: root, Cwd: "relative", Argv: helperArgv(t, "sleep")})
	if err == nil {
		t.Fatal("invalid cwd accepted")
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("validation failure created the root anyway")
	}
}
```

Add the `"launch"` and `"env-check"` helper modes to `runTestHelper` in `main_test.go` exactly as the comments specify (the `launch` mode: `svc, _ := NewService(os.Executable() value); out, err := svc.Launch(LaunchRequest{Root: args[1], Cwd: os.TempDir(), Argv: helperArgv…("sleep")})` — inside helper mode there is no `*testing.T`, so build argv directly from `os.Executable()`; print `out.RunDir` to stdout; return 0).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run 'TestLaunch|TestGateSurvives' ./internal/process/` — Expected: build FAIL (`Launch`, `Observe`, `Stop` undefined). Note: `Observe`/`Stop` are Tasks 8–9; to keep this task green in isolation, add **minimal** versions now in `launch.go`'s own commit? No — instead this task also creates stub-free real `Observe` is not available. **Resolution (intermediate state must be buildable):** write the tests above but reference only `Launch` + `readManifest`/`readTerminal` in THIS task; move `observeUntilTerminal`, `TestLaunchStreamsAndStdin`, `TestLaunchStripsSupervisorEnv`, `TestLaunchFastExit`'s observe fallback, and both `svc.Stop` teardowns into Task 8/9's test additions. In this task, terminal convergence is asserted by polling `readTerminal(out.RunDir)` directly with `waitFor`, and teardown uses `signalGroup(m.PGID, syscall.SIGKILL)` after re-reading the manifest. Keep the assertions identical otherwise.

- [ ] **Step 3: Implement** `internal/process/launch.go` per the algorithm block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 -run 'TestLaunch|TestGateSurvives' ./internal/process/` — Expected: PASS. Then the whole package: `go test -count=1 ./internal/process/`.

- [ ] **Step 5: Mutation-test establishment ordering**

The command must never start before the group is addressable. Mutate `supervisor.go` to skip the `established` manifest write (publish `running` only):

```bash
cp internal/process/supervisor.go internal/process/supervisor.go.bak
# Hand-edit: comment out the phase-"established" writeAtomicJSON call and its handshake line.
git diff --stat internal/process/supervisor.go
go test -count=1 -run TestLaunchEstablishesAddressableSession ./internal/process/
# Expected: still PASS is possible (running implies established) — this mutation
# probes ORDER, so the discriminating assert is TestLaunchFastExit/handshake EOF
# handling. If no test reddens, add to supervisor_test.go an assert that the
# manifest phase sequence recorded in supervisor.log includes "established"
# before "running", then re-run. A guard that never saw red is decoration.
mv internal/process/supervisor.go.bak internal/process/supervisor.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): launch state machine — allocation, re-exec, handshake, survival"
```

---

### Task 7: Exact terminal status — exit vs signal, start failure

**Files:**
- Test: extend `internal/process/launch_test.go` (or new `internal/process/terminal_test.go`)

**Interfaces:**
- Consumes: `Launch` (Task 6), `readTerminal` (Task 2), helper modes (Task 5).
- Produces: proof for acceptance criterion 3; no new production symbols (any decoding bug found here is fixed in `supervisor.go`).

- [ ] **Step 1: Write the failing/probing tests** in `internal/process/terminal_test.go`:

```go
package process

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func launchAndWaitTerminal(t *testing.T, mode string, extra ...string) (*Service, *LaunchOutcome, *terminalRecord) {
	t.Helper()
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), mode, extra...)
	var term *terminalRecord
	waitFor(t, "terminal record", 30*time.Second, func() bool {
		rec, err := readTerminal(out.RunDir)
		if err != nil {
			t.Fatalf("readTerminal: %v", err)
		}
		term = rec
		return rec != nil
	})
	return svc, out, term
}

func TestExactExitCodes(t *testing.T) {
	for _, code := range []int{0, 7, 143} {
		_, _, term := launchAndWaitTerminal(t, "exit", itoa(code))
		if term.Kind != "exit" || term.ExitCode != code {
			t.Fatalf("exit %d recorded as %+v", code, term)
		}
	}
}

// TestExit143IsNotSignal15 — the spec's headline: a genuine `exit 143`
// stays kind=exit code=143; SIGTERM death stays kind=signal signal=15.
// No 128+signal heuristic anywhere.
func TestSignalTermIsExactlySignal15(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	m, err := readManifest(out.RunDir)
	if err != nil {
		t.Fatal(err)
	}
	// TERM the GROUP the way an external killer would; the supervisor
	// catches it and stays alive to record, the child dies by it.
	if err := signalGroup(m.PGID, syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	var term *terminalRecord
	waitFor(t, "signal terminal record", 30*time.Second, func() bool {
		rec, rerr := readTerminal(out.RunDir)
		if rerr != nil {
			t.Fatalf("readTerminal: %v", rerr)
		}
		term = rec
		return rec != nil
	})
	if term.Kind != "signal" || term.Signal != 15 {
		t.Fatalf("SIGTERM recorded as %+v (128+signal fabrication?)", term)
	}
}

// TestStartFailureIsDistinct — an unstartable command produces a
// failure.json, never a fabricated terminal record.
func TestStartFailureIsDistinct(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no-such-binary")
	_, err := svc.Launch(LaunchRequest{Root: root, Cwd: t.TempDir(), Argv: []string{missing}})
	if err == nil {
		t.Fatal("unstartable command reported a usable handle")
	}
	if f, ok := AsFailure(err); !ok || f.Class != FailExternal {
		t.Fatalf("start failure class = %v", err)
	}
	// The one run dir under root must hold failure.json and NO terminal.json.
	entries, _ := os.ReadDir(root)
	var runDir string
	for _, e := range entries {
		if e.IsDir() {
			runDir = filepath.Join(root, e.Name())
		}
	}
	if runDir == "" {
		t.Fatal("no run dir retained for diagnosis")
	}
	if rec, _ := readTerminal(runDir); rec != nil {
		t.Fatalf("fabricated terminal record: %+v", rec)
	}
	fr, err := readFailureRecord(runDir)
	if err != nil || fr == nil || fr.Stage != "start-command" {
		t.Fatalf("failure record: %+v, %v", fr, err)
	}
}
```

Add `func itoa(n int) string { return strconv.Itoa(n) }` or use `strconv.Itoa` inline with the import.

- [ ] **Step 2: Run to see which fail**

Run: `go test -count=1 -run 'TestExact|TestSignal|TestStartFailure' ./internal/process/`
Expected: `TestSignalTermIsExactlySignal15` FAILS if the supervisor dies with the group instead of catching TERM (fix in `supervisor.go` Step 5 of Task 5 sequence); `TestStartFailureIsDistinct` FAILS until Launch's EOF arm reads `failure.json`. Fix in place.

- [ ] **Step 3: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 4: Mutation-test the decode**

```bash
cp internal/process/supervisor.go internal/process/supervisor.go.bak
# Hand-edit the Signaled() branch to write Kind "exit", ExitCode 128+sig (the
# Bash heuristic this change exists to remove).
git diff --stat internal/process/supervisor.go
go test -count=1 -run TestSignalTermIsExactlySignal15 ./internal/process/
# Expected: FAIL with the fabrication named in the message.
mv internal/process/supervisor.go.bak internal/process/supervisor.go
go test -count=1 ./internal/process/
```

- [ ] **Step 5: Commit**

```bash
git add internal/process/ && git commit -m "test(0314): exact terminal status — exit 0/7/143 vs signal 15, start-failure record"
```

---

### Task 8: Observe — ordered read-only decision

**Files:**
- Create: `internal/process/observe.go`
- Test: `internal/process/observe_test.go`; move the deferred observe-dependent asserts from Task 6 Step 2's resolution back in (streams test's `observeUntilTerminal`, fast-exit convergence).

**Interfaces:**
- Consumes: all prior tasks.
- Produces:
  - `type Observation struct { RunID, RunDir string; State State; Terminal *Terminal; Cause string; StdoutLog, StderrLog string }`
  - `func (s *Service) Observe(runDir string) (*Observation, error)` — errors are `Failure`s (`invalid-input` bad path, `invalid-state` malformed records, `blocked` unprovable, `external-failed` syscalls); a valid run always yields a state, never a guessed one.

Observe order (load-bearing, spec §Observation): resolve+manifest+run-ID agreement (`m.RunID != dirname` → `FailInvalidState`) → `readTerminal` first (present → `terminalState(term, stopIntent != nil)`) → `probeFlock`: held → full `identityConjunction` (pass → `StateRunning`; unprovable → `FailBlocked`) → cleanly free → **re-read `terminal.json`** (a terminal write racing the first read wins) → completed `stopped.json` → `StateStopped` → else `StateVanished` with `Cause` from `abandoned.json`/`failure.json` when present. `probeUnknown` from the lock → `FailBlocked`. Logs: only paths returned, never content parsed — no success-text search.

- [ ] **Step 1: Write the failing tests**

`internal/process/observe_test.go`:

```go
package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestObserveRunningThenTerminal(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	obs, err := svc.Observe(out.RunDir)
	if err != nil || obs.State != StateRunning {
		t.Fatalf("running observe: %+v, %v", obs, err)
	}
	if obs.StdoutLog == "" || obs.StderrLog == "" {
		t.Fatalf("log paths missing from observation")
	}
	m, _ := readManifest(out.RunDir)
	signalGroup(m.PGID, syscall_SIGKILL())
	// Supervisor dies with the child under KILL: no terminal record can
	// exist, no stop intent was recorded -> vanished.
	waitFor(t, "vanished", 30*time.Second, func() bool {
		o, oerr := svc.Observe(out.RunDir)
		if oerr != nil {
			return false // lock release can race; keep polling
		}
		return o.State == StateVanished
	})
}

func TestObserveExitStates(t *testing.T) {
	svc := newTestService(t)
	for code, want := range map[int]State{0: StatePassed, 7: StateFailed, 143: StateFailed} {
		out := launchHelper(t, svc, t.TempDir(), "exit", strconv.Itoa(code))
		obs := observeUntilTerminal(t, svc, out.RunDir)
		if obs.State != want {
			t.Fatalf("exit %d observed %v, want %v", code, obs.State, want)
		}
		if want == StateFailed && (obs.Terminal == nil || obs.Terminal.ExitCode != code) {
			t.Fatalf("exact code lost: %+v", obs.Terminal)
		}
	}
}

// TestObserveNeverFabricatesFromMalformedState
func TestObserveNeverFabricatesFromMalformedState(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "0")
	observeUntilTerminal(t, svc, out.RunDir)
	if err := os.WriteFile(filepath.Join(out.RunDir, terminalFile), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Observe(out.RunDir)
	if err == nil {
		t.Fatal("malformed terminal record produced a verdict")
	}
	if f, _ := AsFailure(err); f == nil || f.Class != FailInvalidState {
		t.Fatalf("malformed class = %v", err)
	}
}

// TestObserveTokenAgreement — a manifest whose run_id disagrees with its
// directory is invalid state, not a report about some other run.
func TestObserveTokenAgreement(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "0")
	observeUntilTerminal(t, svc, out.RunDir)
	m, _ := readManifest(out.RunDir)
	m.RunID = "00000000000000000000000000000000"
	writeAtomicJSON(filepath.Join(out.RunDir, manifestFile), m)
	if _, err := svc.Observe(out.RunDir); err == nil {
		t.Fatal("identity-mismatched manifest accepted")
	}
}
```

(`syscall_SIGKILL()` = `syscall.SIGKILL`; import `syscall`, `strconv`, `time` as needed. Also move `observeUntilTerminal` here from the Task 6 sketch, and restore the Task 6 assertions that were deferred: re-add observe-based convergence to `TestLaunchStreamsAndStdin` and `TestLaunchFastExit`.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestObserve ./internal/process/` — Expected: build FAIL.

- [ ] **Step 3: Implement** `internal/process/observe.go` per the ordered algorithm.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the re-read race arm**

```bash
cp internal/process/observe.go internal/process/observe.go.bak
# Hand-edit: delete the second readTerminal after the free-lock probe.
git diff --stat internal/process/observe.go
go test -count=1 ./internal/process/
# The race window is not deterministically forceable from outside, so if no
# test reddens, add a package-private seam: `var observePostProbeHook func()`
# called between probe and re-read; a test sets it to write a terminal
# record, asserting the observation reports the terminal state, not
# vanished. That test MUST redden under this mutation. Do not record this
# as a residual — the seam makes it detectable.
mv internal/process/observe.go.bak internal/process/observe.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): observe — terminal-first ordered decision with post-probe re-read"
```

---

### Task 9: Stop — ownership-gated bounded group termination

**Files:**
- Create: `internal/process/stop.go`
- Test: `internal/process/stop_test.go`

**Interfaces:**
- Consumes: all prior tasks.
- Produces:
  - `type StopOutcome struct { RunID, RunDir string; State State; Terminal *Terminal; Performed bool }` — `Performed` false for the already-terminal no-op.
  - `func (s *Service) Stop(runDir, reason string) (*StopOutcome, error)`

Stop algorithm (spec §Stop, order load-bearing):

1. Resolve + manifest + agreement checks (as observe). `readTerminal` first: present → `StopOutcome{State: terminalState(...), Terminal: …, Performed: false}` (no-op preserving the child's verdict).
2. No terminal: require full `identityConjunction` immediately before signalling. Free lock or unprovable → re-read terminal (present → no-op outcome) → else `FailBlocked`; **never signal**.
3. `writeAtomicJSON(stop-intent.json, stopIntentRecord{…, Reason: boundReason(reason)})`, then `signalGroup(m.PGID, syscall.SIGTERM)`.
4. Poll (interval `s.pollInterval`) up to `s.stopTermWait` for `readTerminal != nil` **and** `groupAlive(m.PGID) == probeAbsent`.
5. Not done: re-prove `identityConjunction`. Unprovable → `FailBlocked`, retain diagnostics, no escalation. Provable → `signalGroup(m.PGID, syscall.SIGKILL)`, poll up to `s.stopKillWait` for group absence.
6. After verified teardown re-read terminal: `exit` → passed/failed as recorded; `signal` after our intent → `StateStopped`. No terminal possible (KILL took the supervisor) **and** group absence verified → `writeAtomicJSON(stopped.json)`, `State: StateStopped`. `Performed: true`. Stop **never** writes `terminal.json`.
7. Once the intent is written and the first signal sent, run the bounded sequence to completion regardless of caller context (`Stop` takes no context — completing is the contract).

- [ ] **Step 1: Write the failing tests**

`internal/process/stop_test.go`:

```go
package process

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestStopGracefulTermRecorded(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "sleep")
	res, err := svc.Stop(out.RunDir, "operator asked")
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !res.Performed || res.State != StateStopped {
		t.Fatalf("graceful stop: %+v", res)
	}
	// The supervisor recorded the exact signal, and the intent classifies
	// it as stopped, not signaled.
	term, _ := readTerminal(out.RunDir)
	if term == nil || term.Kind != "signal" || term.Signal != int(syscall.SIGTERM) {
		t.Fatalf("terminal after graceful stop: %+v", term)
	}
	if obs, _ := svc.Observe(out.RunDir); obs.State != StateStopped {
		t.Fatalf("observe after stop: %v", obs.State)
	}
}

func TestStopNoOpOnTerminalRun(t *testing.T) {
	svc := newTestService(t)
	out := launchHelper(t, svc, t.TempDir(), "exit", "7")
	observeUntilTerminal(t, svc, out.RunDir)
	res, err := svc.Stop(out.RunDir, "late stop")
	if err != nil {
		t.Fatal(err)
	}
	if res.Performed || res.State != StateFailed || res.Terminal.ExitCode != 7 {
		t.Fatalf("no-op must preserve the verdict: %+v", res)
	}
	if _, statErr := os.Stat(filepath.Join(out.RunDir, stopIntentFile)); !os.IsNotExist(statErr) {
		t.Fatalf("no-op stop wrote an intent")
	}
}

// TestStopEscalatesTermIgnorer — bounded KILL for a child that ignores
// TERM; stopped marker only after verified group absence.
func TestStopEscalatesTermIgnorer(t *testing.T) {
	svc := newTestService(t)
	svc.stopTermWait = 500 * time.Millisecond // test seam: shrink, don't sleep-tune
	ready := filepath.Join(t.TempDir(), "ready")
	out := launchHelper(t, svc, t.TempDir(), "ignore-term", ready)
	waitFor(t, "helper ready", 30*time.Second, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	})
	res, err := svc.Stop(out.RunDir, "escalate")
	if err != nil {
		t.Fatalf("escalating stop: %v", err)
	}
	if !res.Performed || res.State != StateStopped {
		t.Fatalf("escalation outcome: %+v", res)
	}
	m, _ := readManifest(out.RunDir)
	if groupAlive(m.PGID) != probeAbsent {
		t.Fatalf("stopped reported while the group still exists")
	}
}

// TestStopRefusesUnprovableOwnership — a free lock plus live-looking
// records must never authorize a signal (PID-reuse defense).
func TestStopRefusesUnprovableOwnership(t *testing.T) {
	svc := newTestService(t)
	// Fabricate an owned-looking run whose "supervisor" is a live process
	// we started OURSELVES (so the pid exists and leads its own session)
	// but which holds no live.lock.
	root := t.TempDir()
	id := "0123456789abcdef0123456789abcdef"
	runDir := filepath.Join(root, id)
	ensurePrivateDir(runDir)
	decoyCmd := exec.Command("/bin/sleep", "300")
	decoyCmd.SysProcAttr = sessionAttrs()
	if err := decoyCmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { decoyCmd.Process.Kill(); decoyCmd.Wait() })
	pid := decoyCmd.Process.Pid
	writeAtomicJSON(filepath.Join(runDir, manifestFile), &manifestRecord{
		Schema: recordSchema, RunID: id, Token: "aa", Root: root, RunDir: runDir,
		SupervisorPID: pid, PGID: pid, SID: pid, Phase: "running", Cwd: "/",
		Argv0: "sleep", Argc: 2, CreatedAt: "x", UpdatedAt: "x"})
	_, err := svc.Stop(runDir, "must refuse")
	if err == nil {
		t.Fatal("stop signalled a run whose lock is not held")
	}
	if f, _ := AsFailure(err); f == nil || f.Class != FailBlocked {
		t.Fatalf("refusal class = %v", err)
	}
	// THE decisive assert: the decoy is still alive — nothing was signalled.
	if processAlive(pid) != probeLive {
		t.Fatalf("stop killed a process it could not prove it owned")
	}
}

func TestStopNeverWritesTerminal(t *testing.T) {
	// Source-shape assert would be decoration; behavioral pin: after the
	// KILL-takes-supervisor path (SIGKILL group via stop on a term-ignorer
	// whose SUPERVISOR also ignores nothing — KILL is unblockable), the
	// run dir contains stopped.json and terminal.json only if the
	// supervisor got it out first; both classify as stopped either way,
	// and stop itself produced no terminal when stopped.json exists alone.
	svc := newTestService(t)
	svc.stopTermWait = 500 * time.Millisecond
	ready := filepath.Join(t.TempDir(), "ready")
	out := launchHelper(t, svc, t.TempDir(), "ignore-term", ready)
	waitFor(t, "helper ready", 30*time.Second, func() bool { _, err := os.Stat(ready); return err == nil })
	res, err := svc.Stop(out.RunDir, "kill path")
	if err != nil || res.State != StateStopped {
		t.Fatalf("%+v, %v", res, err)
	}
	term, terr := readTerminal(out.RunDir)
	stopped, serr := readStopped(out.RunDir)
	if terr != nil || serr != nil {
		t.Fatalf("read-back: %v %v", terr, serr)
	}
	if term == nil && stopped == nil {
		t.Fatalf("neither terminal nor stopped marker after verified teardown")
	}
	if term != nil && term.Kind == "exit" {
		t.Fatalf("a KILLed group cannot have exited normally: %+v", term)
	}
}
```

(Import `os/exec` for the decoy; `strconv` unused — trim imports to what compiles.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestStop ./internal/process/` — Expected: build FAIL.

- [ ] **Step 3: Implement** `internal/process/stop.go` per the algorithm.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the ownership gate**

```bash
cp internal/process/stop.go internal/process/stop.go.bak
# Hand-edit: replace the pre-signal `if err := identityConjunction(...)` guard with `if false {`.
git diff --stat internal/process/stop.go
go test -count=1 -run TestStopRefusesUnprovableOwnership ./internal/process/
# Expected: FAIL — and specifically on the "decoy still alive" assert, which
# pins the MECHANISM (nothing signalled), not just the outcome (an error).
mv internal/process/stop.go.bak internal/process/stop.go
go test -count=1 ./internal/process/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): stop — intent record, TERM/KILL bounds, ownership re-prove, no false terminal"
```

---

### Task 10: Recover — owned abandoned-run marking, everything else retained

**Files:**
- Create: `internal/process/recover.go`
- Test: `internal/process/recover_test.go`

**Interfaces:**
- Consumes: all prior tasks.
- Produces:
  - `type RecoveryEntry struct { RunID, RunDir, Disposition, Reason string }` — dispositions: `"live"`, `"terminal"`, `"stopped"`, `"abandoned-marked"`, `"already-abandoned"`, `"needs-inspection"`, `"foreign"`, `"invalid"`.
  - `type RecoverOutcome struct { Entries []RecoveryEntry; Marked int }` — `Entries` sorted by `RunID` ascending; `Marked` counts **newly written** `abandoned.json` markers only.
  - `func (s *Service) Recover(root string) (*RecoverOutcome, error)` — root must be absolute + an existing directory (`FailInvalidInput`).

Algorithm: under `acquireFlock(root/registry.lock)` take the snapshot (`os.ReadDir`, keep `Lstat`-real dirs whose names match `runIDPattern`; symlinks and non-matching names → `foreign`, byte-untouched). Release the registry lock, then per candidate: `readManifest` — read error/malformed/ID-disagreement → `invalid`, untouched; `probeFlock` held → `live`, untouched; `probeUnknown` → `needs-inspection`; free → re-read `terminal`/`stopped` (present → `terminal`/`stopped`); `abandoned.json` already present → `already-abandoned`; else probe `groupAlive(m.PGID)`: `probeAbsent` → write `abandonedRecord{Cause: "supervisor lock released with no terminal record and the recorded group cleanly absent"}` → `abandoned-marked`, `Marked++`; `probeLive`/`probeUnknown` → `needs-inspection`, signal nothing, delete nothing. Recovery never deletes any file or directory.

- [ ] **Step 1: Write the failing tests**

`internal/process/recover_test.go`:

```go
package process

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"testing"
	"time"
)

func TestRecoverMarksCleanlyAbandonedOwnedRun(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	out := launchHelper(t, svc, root, "sleep")
	m, _ := readManifest(out.RunDir)
	// KILL the whole group: supervisor dies without a terminal record —
	// the abandoned shape.
	signalGroup(m.PGID, syscall.SIGKILL)
	waitFor(t, "lock release", 30*time.Second, func() bool {
		held, _ := probeFlock(filepath.Join(out.RunDir, liveLockFile))
		return !held
	})
	waitFor(t, "group gone", 30*time.Second, func() bool {
		return groupAlive(m.PGID) == probeAbsent
	})
	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 1 || len(res.Entries) != 1 || res.Entries[0].Disposition != "abandoned-marked" {
		t.Fatalf("recover: %+v", res)
	}
	if rec, _ := readAbandoned(out.RunDir); rec == nil {
		t.Fatalf("abandoned.json not written")
	}
	// Observation now carries the stable cause.
	obs, _ := svc.Observe(out.RunDir)
	if obs.State != StateVanished || obs.Cause == "" {
		t.Fatalf("post-recovery observe: %+v", obs)
	}
	// Idempotent: second pass marks nothing.
	res2, _ := svc.Recover(root)
	if res2.Marked != 0 || res2.Entries[0].Disposition != "already-abandoned" {
		t.Fatalf("second recover: %+v", res2)
	}
}

func TestRecoverRetainsLiveForeignAndInvalid(t *testing.T) {
	svc := newTestService(t)
	root := t.TempDir()
	live := launchHelper(t, svc, root, "sleep")
	// Foreign: a directory that is not run-id-shaped, with content.
	foreign := filepath.Join(root, "not-docket")
	os.Mkdir(foreign, 0o755)
	os.WriteFile(filepath.Join(foreign, "keep.txt"), []byte("bytes"), 0o644)
	// Invalid: run-id-shaped but malformed manifest.
	badID := "ffffffffffffffffffffffffffffffff"
	bad := filepath.Join(root, badID)
	os.Mkdir(bad, 0o700)
	os.WriteFile(filepath.Join(bad, manifestFile), []byte("{broken"), 0o600)
	// Symlink at a run slot.
	linkID := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	os.Symlink(bad, filepath.Join(root, linkID))

	res, err := svc.Recover(root)
	if err != nil {
		t.Fatal(err)
	}
	if res.Marked != 0 {
		t.Fatalf("marked %d, want 0", res.Marked)
	}
	byID := map[string]string{}
	for _, e := range res.Entries {
		byID[e.RunID] = e.Disposition
	}
	if byID[live.RunID] != "live" || byID[badID] != "invalid" {
		t.Fatalf("dispositions: %v", byID)
	}
	// Foreign and invalid state byte-untouched.
	if b, _ := os.ReadFile(filepath.Join(foreign, "keep.txt")); string(b) != "bytes" {
		t.Fatalf("foreign content touched")
	}
	if b, _ := os.ReadFile(filepath.Join(bad, manifestFile)); string(b) != "{broken" {
		t.Fatalf("invalid manifest rewritten")
	}
	// Deterministic order.
	ids := make([]string, len(res.Entries))
	for i, e := range res.Entries {
		ids[i] = e.RunID
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("entries not sorted by run id: %v", ids)
	}
	svc.Stop(live.RunDir, "teardown")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 -run TestRecover ./internal/process/` — Expected: build FAIL.

- [ ] **Step 3: Implement** `internal/process/recover.go` per the algorithm.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/process/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the destructive-branch guard**

```bash
cp internal/process/recover.go internal/process/recover.go.bak
# Hand-edit: in the group probe, route probeUnknown into the probeAbsent arm
# (the classic probe-error-is-not-clean-absence collapse).
git diff --stat internal/process/recover.go
# Need a test that injects an unprovable probe: add to recover_test.go a case
# using a manifest whose SupervisorPID/PGID is 1 (kill(-1,0) is EPERM for
# non-root — permanently unprovable), assert disposition "needs-inspection"
# and NO abandoned.json. Under the mutation it must redden.
go test -count=1 -run TestRecover ./internal/process/
mv internal/process/recover.go.bak internal/process/recover.go
go test -count=1 ./internal/process/
```

Write that unprovable-probe case as part of this step (before mutating), commit it with the task.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/process/ && git add internal/process/ && git commit -m "feat(0314): recover — mark proved-abandoned owned runs, retain everything else"
```

---

### Task 11: `internal/app` gate operations — DTO, result mapping, human text

**Files:**
- Create: `internal/app/gate.go`
- Test: `internal/app/gate_test.go`

**Interfaces:**
- Consumes: `process.Service` API (Tasks 6–10), `app.Envelope`/`Result`/`NewEnvelope` (existing).
- Produces (exact spec DTO):

```go
type GateState string

type RecoveryEntry struct {
	RunID       string `json:"run_id"`
	RunDir      string `json:"run_dir"`
	Disposition string `json:"disposition"`
	Reason      string `json:"reason,omitempty"`
}

type GateResult struct {
	Envelope
	RunID     string          `json:"run_id,omitempty"`
	RunDir    string          `json:"run_dir,omitempty"`
	State     GateState       `json:"state,omitempty"`
	ExitCode  *int            `json:"exit_code,omitempty"`
	Signal    *int            `json:"signal,omitempty"`
	Cause     string          `json:"cause,omitempty"`
	StdoutLog string          `json:"stdout_log,omitempty"`
	StderrLog string          `json:"stderr_log,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Recovery  []RecoveryEntry `json:"recovery,omitempty"`
}

func GateLaunch(root, cwd string, argv []string) GateResult
func GateObserve(runDir string) GateResult
func GateStop(runDir, reason string) GateResult
func GateRecover(root string) GateResult
```

  Each constructor resolves `os.Executable()` (failure → `external-failed`), builds `process.NewService`, calls the operation, and maps:
  - failure classes: `process.FailInvalidInput`→`invalid-input`, `FailInvalidState`→`invalid-state`, `FailBlocked`→`blocked`, `FailExternal`→`external-failed`, unclassified error→`internal-error`; `Reason` = the failure's `Stage + ": " + Reason` (bounded safe text only).
  - observation/launch states: `running`/`passed` (and successful launch) → `applied`; `failed` → `gate-failed` with `ExitCode` set; `signaled`/`stopped`/`vanished` → `interrupted` (with `Signal` set for signaled/stopped-with-signal-record).
  - stop: `Performed` true → `applied`; already-terminal → `no-op` **carrying that terminal state** (`gate.stop` result `no-op` even when the preserved state is `failed` — the stop performed nothing; consumers read `state`).
  - recover: `Marked >= 1` → `applied`; clean scan → `no-op`; per-run blocked findings stay entries. `Recovery` normalized: `if entries == nil { entries = []RecoveryEntry{} }` on **every** gate.recover path (the landed status-result nil-collection convention).
  - `HumanText()`: stable labeled lines in fixed order, only non-empty fields — `state: …`, `run_id: …`, `run_dir: …`, `exit_code: …`, `signal: …`, `cause: …`, `stdout_log: …`, `stderr_log: …`, `reason: …`; recover renders `marked: N` then one `run: <id> <disposition>` line per entry.

- [ ] **Step 1: Write the failing tests** — `internal/app/gate_test.go` uses real launches against `t.TempDir()` roots with cheap commands (`/bin/echo`, `/usr/bin/false` — prefer `[]string{"/bin/sh", "-c", "exit 7"}`? **No shell**: use `/bin/echo` for applied and a nonexistent absolute path for external-failed; for gate-failed launch the test binary is not re-exec-safe from app's package, so gate-failed mapping is tested through the mapping function directly). Split the mapping into a pure function so it is unit-testable without processes:

```go
// mapObservation is the operation-sensitive result table for gate.observe
// and post-launch states; pure, so the table is testable without a process.
func mapObservation(st process.State) Result
```

Tests:

```go
package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

func TestMapObservationTable(t *testing.T) {
	cases := map[process.State]Result{
		process.StateRunning:  ResultApplied,
		process.StatePassed:   ResultApplied,
		process.StateFailed:   ResultGateFailed,
		process.StateSignaled: ResultInterrupted,
		process.StateStopped:  ResultInterrupted,
		process.StateVanished: ResultInterrupted,
	}
	for st, want := range cases {
		if got := mapObservation(st); got != want {
			t.Errorf("%s -> %s, want %s", st, got, want)
		}
	}
}

func TestGateLaunchObserveEndToEnd(t *testing.T) {
	root := t.TempDir()
	res := GateLaunch(root, t.TempDir(), []string{"/bin/echo", "hello"})
	if res.Operation != "gate.launch" {
		t.Fatalf("operation %q", res.Operation)
	}
	if res.Result != ResultApplied {
		t.Fatalf("launch result %s (%s)", res.Result, res.Reason)
	}
	if res.RunDir == "" || res.RunID == "" {
		t.Fatalf("no handle: %+v", res)
	}
	// Poll observe to terminal; /bin/echo exits 0 fast.
	deadline := 300 // x100ms
	for i := 0; ; i++ {
		obs := GateObserve(res.RunDir)
		if obs.Result == ResultApplied && obs.State == "passed" {
			if obs.ExitCode == nil || *obs.ExitCode != 0 {
				t.Fatalf("exact code: %+v", obs.ExitCode)
			}
			break
		}
		if obs.State != "running" {
			t.Fatalf("unexpected: %+v", obs)
		}
		if i > deadline {
			t.Fatal("echo never became terminal")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestGateLaunchInvalidInput(t *testing.T) {
	res := GateLaunch("relative-root", "/", []string{"/bin/echo"})
	if res.Result != ResultInvalidInput {
		t.Fatalf("result %s", res.Result)
	}
	if ExitCode(res.Result) != 2 {
		t.Fatalf("exit mapping")
	}
}

func TestGateRecoverNormalizesEmptyEntries(t *testing.T) {
	res := GateRecover(t.TempDir())
	if res.Result != ResultNoOp {
		t.Fatalf("clean scan result %s", res.Result)
	}
	buf, _ := json.Marshal(res)
	if !strings.Contains(string(buf), `"recovery":[]`) {
		t.Fatalf("nil collection leaked as absent: %s", buf)
	}
}

func TestGateResultHumanTextStable(t *testing.T) {
	code := 7
	r := GateResult{Envelope: NewEnvelope("gate.observe", ResultGateFailed),
		RunID: "aa", RunDir: "/r/aa", State: "failed", ExitCode: &code,
		StdoutLog: "/r/aa/stdout.log", StderrLog: "/r/aa/stderr.log"}
	want := "state: failed\nrun_id: aa\nrun_dir: /r/aa\nexit_code: 7\nstdout_log: /r/aa/stdout.log\nstderr_log: /r/aa/stderr.log"
	if r.HumanText() != want {
		t.Fatalf("HumanText:\n got %q\nwant %q", r.HumanText(), want)
	}
}
```

**Important — `Recovery`'s tag cannot be `omitempty`.** `encoding/json` omits an empty slice under `omitempty`, but the spec requires `"recovery": []` on every gate.recover path (the landed nil-collection convention), while the other three operations must omit the field entirely. One struct cannot express both, so the recover operation gets its own result type: drop `Recovery` from `GateResult` and add

```go
type GateRecoverResult struct {
	Envelope
	Marked   int             `json:"marked"`
	Recovery []RecoveryEntry `json:"recovery"`
}
```

with `GateRecover(root string) GateRecoverResult` normalizing a nil entry slice to `[]RecoveryEntry{}` before return. Write `TestGateRecoverNormalizesEmptyEntries` against `GateRecoverResult`; its `HumanText()` renders `marked: N` then one `run: <id> <disposition>` line per entry. The spec's single-DTO sketch explicitly yields to package conventions ("the exact Go split may follow package conventions, but protocol meanings are fixed") — protocol meanings are unchanged.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/app/` — Expected: build FAIL.

- [ ] **Step 3: Implement** `internal/app/gate.go` (both structs, four constructors, `mapObservation`, `mapFailure(err error) (Result, string)`, `HumanText` for both result types).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/app/` — Expected: PASS.

- [ ] **Step 5: Mutation-test the mapping table**

```bash
cp internal/app/gate.go internal/app/gate.go.bak
# Hand-edit mapObservation: return ResultApplied for StateFailed.
git diff --stat internal/app/gate.go
go test -count=1 -run TestMapObservationTable ./internal/app/
# Expected: FAIL naming failed->applied.
mv internal/app/gate.go.bak internal/app/gate.go
go test -count=1 ./internal/app/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/app/ && git add internal/app/ && git commit -m "feat(0314): app gate operations — protocol DTOs, result mapping, human text"
```

---

### Task 12: `internal/cli` gate group + `cmd/docket` end-to-end

**Files:**
- Create: `internal/cli/gate.go`
- Modify: `internal/cli/root.go` (register `newGateCommand()` on root, next to the existing group registrations)
- Test: `internal/cli/gate_test.go`, `cmd/docket/gate_cli_test.go`

**Interfaces:**
- Consumes: `app.GateLaunch/GateObserve/GateStop/GateRecover` (Task 11); existing `run` result plumbing (`result = …; return nil` pattern), `Presenter`.
- Produces: `func newGateCommand(setResult func(app.OperationResult)) *cobra.Command` — follow the existing registration idiom in `root.go` (the diagnostic group closes over `result`; mirror it):
  - `gate` group: `Args: cobra.NoArgs`, `RunE` returns `errors.New("missing command")` — byte-parity with `diagnostic`/`development` group behavior.
  - `launch`: flags `--root` (string, required), `--cwd` (string, required); argv = `args[cmd.ArgsLenAtDash():]` — require `ArgsLenAtDash() >= 0` (a `--` was present) and at least one word after it and **zero** positional words before it; violations return an error (→ invalid-input path) with a message naming the `--` contract. Then `result = app.GateLaunch(root, cwd, argv)`.
  - `observe <run-dir>`: `cobra.ExactArgs(1)` → `app.GateObserve(args[0])`.
  - `stop <run-dir> [--reason <text>]`: `cobra.ExactArgs(1)` + `--reason` flag → `app.GateStop(args[0], reason)`.
  - `recover --root <dir>`: required `--root`, `cobra.NoArgs` → `app.GateRecover(root)`.
- Also produces `internal/cli/boundary_test.go`-style guard **inside `gate_test.go`**: parse every `internal/cli/*.go` production file's imports and assert none is `github.com/danielhanold/docket/internal/process` (same `go/parser` shape as Task 1, forbidden set = the one path, plus a population floor).

- [ ] **Step 1: Write the failing tests**

`internal/cli/gate_test.go` — drive through `cli.Run` with real streams (existing tests show the pattern; follow `root_test.go`'s harness):

```go
package cli

// Tests follow root_test.go's runCLI/capture pattern — reuse its helpers if
// exported within the package; otherwise construct bytes.Buffers and call
// Run(args, strings.NewReader(""), &out, &errB, testInfo, testFacts).

func TestGateGroupMissingCommand(t *testing.T) {
	// `docket gate` behaves like `docket diagnostic`: invalid input, error
	// on stderr in human mode, one JSON document in JSON mode.
}

func TestGateLaunchRequiresDashBoundary(t *testing.T) {
	// `docket gate launch --root /abs --cwd /abs` (no --) -> exit 2,
	// message names the `--` requirement.
	// `docket gate launch --root /abs --cwd /abs stray -- /bin/echo` -> exit 2.
}

func TestGateLaunchJSONOneDocument(t *testing.T) {
	// `docket gate launch --json --root <tmp> --cwd <tmp> -- /bin/echo hi`
	// -> exit 0; stdout parses as exactly one JSON document with
	// operation "gate.launch", result "applied", non-empty run_dir;
	// stderr empty. Then `docket gate observe --json <run_dir>` polled
	// until state "passed" with exit_code 0.
}

func TestGateStopAndRecoverWiring(t *testing.T) {
	// stop on the passed run above -> result "no-op", state preserved.
	// recover --root <tmp> -> "no-op", "recovery":[] present in the JSON.
}

func TestCLIDoesNotImportProcess(t *testing.T) {
	// go/parser sweep over ./*.go production files: no import of
	// internal/process; population floor >= 5 files checked.
}
```

Write these as full tests (the sketch comments become code; each JSON assertion decodes with `json.Unmarshal` into `map[string]any` and checks fields — never string-contains on human text).

`cmd/docket/gate_cli_test.go` — through the **built production binary** (the package's `TestMain` already builds it; reuse its harness pattern from `main_test.go`): launch `/bin/echo` under a `t.TempDir()` root via the real `docket` binary, poll `gate observe --json` to `passed`, assert exact `exit_code: 0`, one-document stdout, empty stderr, process exit 0. This is the only test exercising the true production re-exec (binary re-executing the binary) rather than the test-binary seam.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/cli/ ./cmd/docket/` — Expected: FAIL (`unknown command "gate"`).

- [ ] **Step 3: Implement** `internal/cli/gate.go` + the one-line registrations in `root.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/cli/ ./cmd/docket/` — Expected: PASS. Then `go test -count=1 ./...` and `go vet ./...`.

- [ ] **Step 5: Mutation-test the `--` boundary guard**

```bash
cp internal/cli/gate.go internal/cli/gate.go.bak
# Hand-edit: drop the ArgsLenAtDash() >= 0 requirement (treat all args as argv).
git diff --stat internal/cli/gate.go
go test -count=1 -run TestGateLaunchRequiresDashBoundary ./internal/cli/
# Expected: FAIL.
mv internal/cli/gate.go.bak internal/cli/gate.go
go test -count=1 ./internal/cli/
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/cli/ cmd/ && git add internal/cli/ cmd/ internal/app/ && git commit -m "feat(0314): docket gate CLI group with -- argv boundary and one-document JSON"
```

---

### Task 13: Race-shard the heavy process suite + budget rows

**Files:**
- Create: `tests/test_go_race_process.sh`
- Modify: `tests/test_go_race.sh` (add `internal/process` to the derived exclusion + the partition/completeness guard), `tests/runtime-budgets.tsv` (new rows + re-seeded `EXPECTED_TOTAL` per the table's own header rules)

**Interfaces:**
- Consumes: the sharding pattern of `tests/test_go_race_transaction.sh` / `tests/test_go_race_workspace.sh` (changes 0309/0313) — copy their file shape byte-for-byte where it is contract: the canonical `assert` helper (byte-exact allowlist in `scripts/check-test-source-hygiene.sh`), the CACHES block pinning `GOMODCACHE/GOCACHE` to `<git common dir>/docket-go-cache/{mod,build}`, `-modcacherw`, and `set -uo pipefail`.
- Produces: a fourth shard running `go test -race -count=1 ./internal/process/`; `test_go_race.sh`'s `go list`-derived complement now also excludes `internal/process`; the partition guard at that file's foot (union of the four shards == `go list ./...`) updated so a package cannot fall through the gap.

- [ ] **Step 1: Read the reference shard**

Read `tests/test_go_race_transaction.sh` end to end and `tests/test_go_race.sh`'s exclusion + completeness-guard section. The new file is that shape with the package path swapped and a header explaining: real-process supervisor tests re-exec instrumented binaries and wait on real children, so folded into the main race row they would breach the 60s hard ceiling — the sanctioned answer is a shard (see 0309/0324 precedent in the budgets header).

- [ ] **Step 2: Write `tests/test_go_race_process.sh`** (executable bit set: `chmod +x`). Core check: `go test -race -count=1 ./internal/process/` with the assert-marker accounting the other shards use.

- [ ] **Step 3: Update `tests/test_go_race.sh`** — add `internal/process` to the excluded set the same way `internal/repository/transaction` and `internal/workspace` are excluded (extend the existing derived-list mechanism; do **not** introduce a hand-enumerated second list), and extend the partition guard so the union assertion covers all four shards.

- [ ] **Step 4: Mutation-test the partition guard**

```bash
cp tests/test_go_race.sh tests/test_go_race.sh.bak
# Hand-edit: additionally exclude internal/render from the main shard WITHOUT
# adding it to any sibling (a package falling through the gap).
bash tests/test_go_race.sh; echo "exit=$?"
# Expected: the completeness guard reddens (NOT OK naming the orphaned package).
mv tests/test_go_race.sh.bak tests/test_go_race.sh
```

- [ ] **Step 5: Measure and add budget rows**

```bash
scripts/run-tests.sh -j 1 --timings /tmp/timings-process.txt tests/test_go_race_process.sh
scripts/run-tests.sh -j 1 --timings /tmp/timings-race.txt tests/test_go_race.sh
scripts/run-tests.sh -j 1 --timings /tmp/timings-toolchain.txt tests/test_go_toolchain.sh
```

Read `tests/runtime-budgets.tsv`'s header for its own new-file and re-budget rules (next multiple of 5 plus a 5s margin over the **worst standalone serial** reading; `EXPECTED_TOTAL` re-seed). Add the `tests/test_go_race_process.sh` row; re-measure `tests/test_go_race.sh` and `tests/test_go_toolchain.sh` (both gained the process package — toolchain's plain `go test ./...` now runs the real-process tests uninstrumented) and raise their rows only if the measured worst serial demands it, recording the measurement and `-j` level in the row comment per the table's convention. Then `bash tests/test_runtime_budgets.sh` must pass.

- [ ] **Step 6: Commit**

```bash
git add tests/test_go_race_process.sh tests/test_go_race.sh tests/runtime-budgets.tsv
git commit -m "test(0314): shard internal/process race run; re-measure Go gate budgets"
```

---

### Task 14: Whole-suite gate and close-out evidence

**Files:**
- Modify: none expected (fixes only if the gate finds them)

- [ ] **Step 1: gofmt + vet + full Go sweep**

```bash
cd /Users/homer/dev/docket/.worktrees/native-process-supervisor-and-local-gate
gofmt -l .            # nothing
go vet ./...
go test -count=1 ./...
```

- [ ] **Step 2: Run the whole resolved suite** — the build gate is `finalize.test_command` = `scripts/run-tests.sh` (read it from `.docket.yml`; never a second copy), the entire suite, not only the tests this plan enumerates:

```bash
scripts/run-tests.sh
```

Expected: all files pass. Read any trailing `OVER BUDGET:` lines as findings to act on (they do not fail the run): a whole-suite cliff of many rows over by a similar factor is a statement about machine saturation — re-run at `-j 3` before touching any budget; a single new-file row over is a real re-measure item back in Task 13. If shell files redden uniformly on a missing builtin, check which interpreter ran (PATH) before reading the diff.

- [ ] **Step 3: Darwin + Linux evidence** — the real-process suite must run on both targets before acceptance (spec §Testing). Darwin is this machine's run above. For Linux: run `go test -count=1 ./internal/process/` on a Linux environment if one is available to the build loop; if none is reachable from this worktree, record in the build evidence that Linux execution is **outstanding for the review/merge gate** (cross-compilation via `TestCrossCompileApprovedTargets` is a build check, not execution evidence — say so explicitly rather than letting the green cross-build stand in). This is a named human-visible checkpoint, not an assert that can only pass.

- [ ] **Step 4: Close-out notes for the results file** (docket-build records these with the build evidence):
  - remaining margin, as numbers, for every runtime-budget row this change moved (`test_go_toolchain.sh`, `test_go_race.sh`, new `test_go_race_process.sh`) with the `-j` level of the measurement;
  - the Linux execution status from Step 3;
  - the reminder that Step 6 of the implement-next run mints the ADR superseding ADR-0081 (spec §ADR action) — not done in this plan.

- [ ] **Step 5: Final commit** (only if Steps 1–2 forced fixes)

```bash
git add -u && git commit -m "chore(0314): full-suite gate fixes"
```

---

## Self-Review (performed while writing)

- **Spec coverage:** launch/establishment (T5–6), exact terminal (T7), observe (T8), stop (T9), recover (T10), DTO/result mapping + nil-collection rule (T11), CLI surface + `--` contract + one-document JSON (T12), privacy/mode/atomicity rules (T2–3), platform boundary + explicit unsupported-OS failure (T4), import boundary both directions (T1, T12), both-OS evidence (T14), ADR action deferred to Step 6 (Global Constraints, T14). Escaped-descendant residual: documented in `stop.go`'s header comment as the spec's retained residual (add one sentence there in T9) — it is genuinely undetectable-by-signalling-scope, not unprobed.
- **Placeholder scan:** the two deliberately open decisions (Task 6 Step 2's test split, Task 11's `Recovery` tag) are resolved in-line with a final decision each; no TBDs remain.
- **Type consistency:** `Service` bounds fields (T1) are the ones T9 shrinks; `terminalState` defined once (T6) and reused (T8, T9); `LaunchRequest` lives in `paths.go` (T3) and is what T6/T11 construct; `GateRecover` returns `GateRecoverResult` (T11) and T12's recover test asserts `"recovery":[]` against it.
