<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0318 — Go-only source cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-29-0318-config-contraction-self-hosting-and-hard-cutover.md)**
<!-- docket:backlink:end -->
# Go-Native Whole-Suite Test Runner and Gate Cutover — Implementation Plan

> **For agentic workers:** This plan is executed by **docket-build** — one build-profile
> worker per task, in order, TDD per task, one full-suite gate at the end. Steps use
> checkbox (`- [ ]`) syntax for tracking. Each task is a coherent, independently
> testable unit; a worker sees only its own task plus this header, so the
> **Interfaces** blocks are load-bearing.

**Goal:** Add `docket development test` as the Go-native whole-suite runner and cut
`finalize.test_command`, contributor docs, and RC source validation over to the
branch-faithful entry `go run ./cmd/docket development test`, while the entire Bash
runner (`scripts/run-tests.sh`), its helpers, and the whole test corpus remain present
and green as the frozen parity oracle.

**Architecture:** A new orchestration package `internal/suiterunner` owns discovery,
budget policy, isolation, scheduling, the durable result protocol, deterministic
aggregation, the screen-then-confirm budget state machine, and signal handling. A thin
cobra wiring in `internal/cli` exposes it as `development test`. The runner executes
the SAME suite the Bash runner does — every `tests/test_*.sh` file, which already
includes the Go targets wrapped as Bash shards — via the same deterministic discovery
rule, so there is no second target list. The cutover then repoints `.docket.yml`,
the RC workflow's suite step, and contributor docs at the one source entry.

**Tech Stack:** Go 1.26 (stdlib only: `os/exec`, `os/signal`, `syscall.Setpgid`,
`encoding/json`), cobra (existing CLI tree), Bash test corpus as child targets.
Nearest in-repo analogues: `internal/gatedrive` (driver/store split),
`internal/process` (process supervision).

**Spec:** `docs/superpowers/specs/2026-08-28-config-contraction-self-hosting-and-hard-cutover-design.md`
(synchronized copy under `.docket/`; the spec's 24 acceptance criteria are the
contract). Change file: `docs/changes/active/0318-config-contraction-self-hosting-and-hard-cutover.md`.

## Global Constraints

Every task's requirements implicitly include all of these.

- **Frozen prior workflow:** `scripts/run-tests.sh`, `scripts/run-tests.md`,
  `scripts/check-test-source-hygiene.sh`, every existing `tests/test_*.sh`, and every
  existing caller stay byte-identical except where a task names an edit. NEVER weaken,
  delete, or "pre-migrate" an existing test because 0369/0370 will remove its
  mechanism later. No forwarding shim, no caller migration, no facade deletion.
- **Frozen fixtures:** `testdata/repositories/v0.9.2` … `v0.9.5` are immutable inputs.
  The `.docket.yml` edit is delivered by cutting a NEW tree `testdata/repositories/v0.9.6`
  (Task 9) — that is the drift guard `assertFrozenCopyMatchesLive`'s own remedy.
- **Exact clause spellings** (machine-detected by the RC workflow and AGENTS.md
  discipline; copy verbatim, line-anchored at column 0):
  - `BUDGET WATCH: <path> — <N>s under -j<J>; consecutive parallel-overrun streak <S>/5`
  - `PARALLEL-SENSITIVE: <path> — <N>s under -j<J>; last solo measurement <M>s; recheck progress <K>/10`
  - `SERIAL CONFIRMATION DUE: <path>`
  - `SERIAL CONFIRMATION FAILED: <path>`
  - `SERIAL CONFIRMATION DEFERRED: <path> — Recheck is due; another test consumed this run's confirmation slot`
  - `SERIAL CONFIRMED OVER BUDGET: <path> — <P>s under -j<J>; <S>s solo; solo threshold <T>s`
  - per-file suffix `  OVER BUDGET (ceiling <C>s)` and summary line `OVER BUDGET:<names>`
  - `SUITE files=<n> passed=<n> failed=<n> asserts=<n> wall=<n>s`
  - `FAILED:<names>` / `NO RESULT:<names>`
- **Threshold arithmetic, integer, exactly as the Bash oracle:**
  screening (contended parallel): `secs*2 > ceil*5`; authoritative (solo / serial-mode /
  -j1): `secs*2 > ceil*3`. Threshold display: `half := ceil*3`; even → `half/2`,
  odd → `half/2` followed by `.5`. Default ceiling **60s**, default mode **parallel**.
- **Exit contract** (mirrors `scripts/run-tests.sh`, precedence `1 > 3 > 4 > 0`):
  0 all passed (advisory breaches included); 1 a test failed; 2 usage error /
  runner-internal fail-closed (unusable bash, unusable hygiene checker, duplicate
  basenames, missing file, bad env value); 3 a scheduled target produced no valid
  result; 4 strict-mode confirmed/failed budget breach; 5 source-hygiene violation
  (zero targets executed); 130/143 interrupted by SIGINT/SIGTERM.
- **Budget state machine constants:** streak ≥ 5 arms the first confirmation;
  since-counter ≥ 10 arms a recheck; at most ONE scheduled confirmation per
  qualifying run; a qualifying run = default corpus AND jobs>1 AND budget check on
  AND zero failed AND zero no-result; a clean measurement resets the streak in
  `unobserved/watching` but never the since-counter in
  `parallel-sensitive/confirmed-breach`; a FAILED confirmation clears nothing and
  the candidate stays due. State store is ADVISORY and fail-open everywhere.
- **Verification runs:** every `go test` whose purpose is evidence uses `-count=1`
  (learning `cached-runner-serves-a-mutated-tree`).
- **Go package test runtime:** `internal/suiterunner`'s default-tag tests must stay
  lean — they run inside `tests/test_go_toolchain.sh` and `tests/test_go_race.sh`,
  both already near their 60s hard ceilings. Target: whole package under ~8s under
  `-race`. No sleeps for timing verdicts — use the injected-duration seam and
  deadline-polled pidfiles. Slow end-to-end coverage lives in the new Bash suite
  files, which carry their own budget rows.
- **New Bash test files:** each gets a `tests/runtime-budgets.tsv` row in the SAME
  task that adds it (`tests/test_runtime_budgets.sh` fails a rowless file, and pins
  `EXPECTED_TOTAL` — re-seed it and say why in the row comment). Use the tree's
  canonical byte-exact assert helper (copy it from `tests/test_go_integration_contract.sh`),
  `set -uo pipefail`, no backticks anywhere in test source
  (`scripts/check-test-source-hygiene.sh` aborts the whole suite otherwise), no
  `producer | early-exiting-consumer` pipelines, `grep -E -e` for patterns leading
  with `--`. Read `tests/README.md` before writing one.
- **Comment anchors:** symbol names or verbatim-quoted clauses, never `file:line`
  (`tests/test_comment_anchor_style.sh` enforces).
- **Env seams (shared with the Bash runner so the differential harness drives both
  with one setup):** `DOCKET_RUNTESTS_TESTS_DIR` (suite dir override),
  `DOCKET_RUNTESTS_TEST_DURATIONS` (TSV: `<base>.sh<TAB><parallel-secs><TAB><solo-secs>`;
  replaces measured durations), `DOCKET_RUNTESTS_SOLO=1` (exported to solo
  confirmations), `DOCKET_BASH_PATH` (bash used for child targets). Go-runner-only
  seams (the Bash runner uses flags for these): `DOCKET_RUNTESTS_JOBS`,
  `DOCKET_RUNTESTS_BUDGETS`, `DOCKET_RUNTESTS_STATE`, `DOCKET_RUNTESTS_STRICT=1`.
- **Commit per task**, message style `feat(0318): …` / `test(0318): …`, staging only
  the task's own paths (never `git add -A` — the worktree is shared surface).

## File Structure

| Path | Role |
|---|---|
| `internal/suiterunner/discover.go` | deterministic suite discovery + target validation |
| `internal/suiterunner/budgets.go` | budget table load, ceilings, modes, thresholds |
| `internal/suiterunner/sandbox.go` | per-target isolation env + workdir layout |
| `internal/suiterunner/execute.go` | single-target child execution + durable result publication |
| `internal/suiterunner/schedule.go` | parallel/serial lanes, bounded concurrency, launch order |
| `internal/suiterunner/results.go` | durable result records: schema, atomic write, validation |
| `internal/suiterunner/aggregate.go` | deterministic report, failure taxonomy, exit mapping |
| `internal/suiterunner/budgetstate.go` | screening state machine, store, solo confirmation, strict |
| `internal/suiterunner/signal.go` | signal forwarding, pgid escalation, orphan prevention |
| `internal/suiterunner/run.go` | `Run(ctx, Config) int` — the orchestration entrypoint |
| `internal/suiterunner/*_test.go` | package unit tests (fast, hermetic, synthetic fixtures) |
| `internal/cli/development_test_cmd.go` | `development test` cobra wiring |
| `tests/test_devtest_runner.sh` | end-to-end Bash coverage of `go run ./cmd/docket development test` |
| `tests/test_devtest_differential.sh` | Bash-vs-Go differential harness over synthetic suites |
| `tests/test_devtest_cutover.sh` | source-fidelity + docs + RC-workflow cutover guards |
| `.docket.yml`, `testdata/repositories/v0.9.6/…`, `internal/config/fixtures_test.go`, `.github/workflows/release-candidate.yml`, `AGENTS.md`, `tests/README.md`, `scripts/run-tests.md`, `tests/runtime-budgets.tsv` | cutover edits (Tasks 7–9) |

Naming note: the Go file is `development_test_cmd.go`, NOT `development_test.go` —
a `*_test.go` suffix would make the command's production source a test file.

---

### Task 1: `internal/suiterunner` — discovery, budgets, target validation

**Files:**
- Create: `internal/suiterunner/discover.go`, `internal/suiterunner/budgets.go`
- Test: `internal/suiterunner/discover_test.go`, `internal/suiterunner/budgets_test.go`

**Interfaces:**
- Consumes: nothing (foundation task).
- Produces (later tasks rely on these exact names):

```go
package suiterunner

type Mode string

const (
	ModeParallel Mode = "parallel"
	ModeSerial   Mode = "serial"
)

const DefaultCeiling = 60

// Target is one scheduled suite member. Base is the stable identity join key
// (basename), mirroring the Bash oracle's "Basename is the join key" rule.
type Target struct {
	Path    string // as given (absolute or repo-relative)
	Base    string // "test_x.sh"
	Ceiling int    // seconds
	Mode    Mode
}

// Discover returns tests/test_*.sh under dir (maxdepth 1), sorted by byte
// value (C collation). Fail-closed: an unreadable dir or an empty result is
// an error, never an empty pass.
func Discover(dir string) ([]string, error)

// LoadBudgets parses the runtime-budgets TSV: `<path>\t<seconds>\t<parallel|serial>`,
// comment/blank lines skipped, keyed by basename. A malformed seconds field
// falls back to DefaultCeiling (the oracle keeps running the tests; the table's
// own guard test makes malformed rows loud). Missing file => empty map, nil error.
func LoadBudgets(path string) (map[string]budgetRow, error)

type budgetRow struct {
	Ceiling int
	Mode    Mode
}

// ResolveTargets joins discovered/explicit paths with the budget table and
// validates the whole input set before anything runs (learning
// validate-the-whole-input-set-first): every path must exist, and no two
// targets may share a basename. All violations are reported together.
func ResolveTargets(paths []string, budgets map[string]budgetRow) ([]Target, error)

// ScreenOver / SoloOver are the two threshold predicates, integer-exact
// against the Bash oracle: contended screening secs*2 > ceil*5, authoritative
// solo secs*2 > ceil*3.
func ScreenOver(secs, ceil int) bool
func SoloOver(secs, ceil int) bool

// SoloThreshold renders ceil*3/2 the way the oracle prints it: "45" or "22.5".
func SoloThreshold(ceil int) string
```

- [ ] **Step 1: Write failing tests.** In `discover_test.go` / `budgets_test.go`
  (fixtures via `t.TempDir()`):
  - `TestDiscoverSortsByByteValue` — create `test_b.sh`, `test_a.sh`, `test_Z.sh`,
    a non-matching `helper.sh`, and a subdir `sub/test_c.sh`; assert the result is
    exactly `[test_Z.sh test_a.sh test_b.sh]` paths (uppercase sorts before
    lowercase in C collation; subdir excluded).
  - `TestDiscoverFailsClosed` — nonexistent dir errors; a dir with zero matches
    errors (message contains `no test files`).
  - `TestLoadBudgetsParsesOracleFormat` — a table containing a comment line, a
    valid row `tests/test_a.sh\t20\tserial`, a malformed-seconds row
    `tests/test_b.sh\tabc\tparallel` (→ Ceiling 60), an unknown-mode row
    (→ ModeParallel), and a final row without trailing newline (must still parse).
  - `TestResolveTargetsRejectsDuplicateBasenamesAndMissingFilesTogether` — two
    paths `a/test_x.sh`, `b/test_x.sh` plus one missing path: error names ALL
    three problems, not just the first.
  - `TestThresholds` — table-driven: `ScreenOver(150,60)=false` (150*2=300 == 60*5),
    `ScreenOver(151,60)=true`, `SoloOver(90,60)=false`, `SoloOver(91,60)=true`,
    `SoloThreshold(60)="90"`, `SoloThreshold(15)="22.5"`, `SoloThreshold(10)="15"`.
- [ ] **Step 2:** `go test -count=1 ./internal/suiterunner/` — expect compile
  failures / reds.
- [ ] **Step 3:** Implement the four files' functions exactly per the signatures
  above. Discovery: `os.ReadDir` + `strings.HasPrefix(name,"test_")` +
  `strings.HasSuffix(name,".sh")` + `sort.Strings` (byte sort == C collation).
- [ ] **Step 4:** `go test -count=1 ./internal/suiterunner/` — PASS. Also
  `gofmt -l internal/suiterunner` (empty) and `go vet ./internal/suiterunner/`.
- [ ] **Step 5: Commit** `feat(0318): suiterunner discovery, budget table, thresholds`.

---

### Task 2: sandbox, single-target execution, durable result publication

**Files:**
- Create: `internal/suiterunner/sandbox.go`, `internal/suiterunner/execute.go`,
  `internal/suiterunner/results.go`
- Test: `internal/suiterunner/execute_test.go`, `internal/suiterunner/results_test.go`

**Interfaces:**
- Consumes: `Target` from Task 1.
- Produces:

```go
// Result is the durable per-target record, one JSON file per target at
// <work>/stat/<Base>.json, written temp-beside-destination + atomic rename.
type Result struct {
	Schema  int    `json:"schema"`  // 1
	Target  string `json:"target"`  // Base — identity, validated against filename
	RC      int    `json:"rc"`
	Seconds int    `json:"seconds"`
	OK      int    `json:"ok"`      // count of log lines matching ^ok[[:space:]]*-
	NotOK   int    `json:"notok"`   // count of log lines matching ^NOT OK
}

// Sandbox builds the isolated child environment for one target under
// jobdir: HOME, TMPDIR, XDG_CONFIG_HOME, GIT_CONFIG_GLOBAL/SYSTEM (synthetic
// identity "docket test <test@docket.invalid>", defaultBranch main),
// GIT_TERMINAL_PROMPT=0, GIT_ASKPASS=true, GIT_EDITOR/EDITOR/VISUAL=true,
// GIT_PAGER/PAGER=cat, GIT_MERGE_AUTOEDIT=no — the exact env set the Bash
// oracle's launch() exports. Returns the env slice (base os.Environ() with
// these overriding) and creates the dirs/files.
func Sandbox(jobdir string) ([]string, error)

// ExecuteTarget runs one target under bash in its sandbox, captures combined
// output to <work>/logs/<Base>.log, counts ok/NOT OK markers, and atomically
// publishes the Result. It sets the child's process group (Setpgid) and
// registers it with reg so the signal layer can reach the whole tree.
// extraEnv entries (e.g. DOCKET_RUNTESTS_SOLO=1) are appended last.
// The returned Result is the runner-observed truth the aggregator later
// cross-checks against the durable file.
func ExecuteTarget(ctx context.Context, bash string, t Target, work string, reg *procRegistry, extraEnv []string) (Result, error)

// WriteResult / ReadResult: atomic publish and strict read of a Result file.
// ReadResult fails on: unreadable, invalid JSON, wrong schema, empty Target.
func WriteResult(dir string, r Result) error
func ReadResult(path string) (Result, error)

// procRegistry tracks live child pgids; Task 6 owns its signal methods, but
// the type and Register/Unregister live here so ExecuteTarget can compile.
type procRegistry struct{ mu sync.Mutex; pgids map[int]bool }
func newProcRegistry() *procRegistry
func (r *procRegistry) Register(pgid int)
func (r *procRegistry) Unregister(pgid int)
func (r *procRegistry) Snapshot() []int
```

- [ ] **Step 1: Write failing tests:**
  - `TestSandboxIsolation` — run a script via `ExecuteTarget` that prints `$HOME`,
    `$TMPDIR`, and `git config user.email`; assert all three resolve inside the
    jobdir / to `test@docket.invalid`, and that the real `os.Getenv("HOME")` never
    appears in the log.
  - `TestExecuteWritesDurableResultAtomically` — a passing script printing
    `ok - one` and `ok - two`; assert exactly one `stat/test_p.json` exists, no
    temp leftovers (`stat/` has exactly 1 entry), and the record round-trips with
    RC=0, OK=2, NotOK=0.
  - `TestExecuteRecordsFailure` — script printing `NOT OK - broke` then `exit 1`;
    Result has RC=1, NotOK=1; log file contains the marker line.
  - `TestReadResultFailureModes` — table: truncated JSON, wrong schema (2),
    empty target, unreadable file → each errors with a distinct message substring
    (`malformed`, `unsupported schema`, `missing target identity`).
  - `TestExecuteChildGetsOwnProcessGroup` — script prints its own pgid
    (`ps -o pgid= -p $$`); assert it differs from the test process's pgid and was
    Registered/Unregistered (Snapshot empty after completion).
- [ ] **Step 2:** run, expect red.
- [ ] **Step 3:** Implement. Key points: `exec.CommandContext` is NOT used for the
  child (its default kill reaches only the direct child) — build `exec.Cmd` with
  `SysProcAttr: &syscall.SysProcAttr{Setpgid: true}`; marker counts by scanning
  the log with `regexp.MustCompile(`^ok[[:space:]]*-`)` / `^NOT OK` per line;
  atomic publish: `os.CreateTemp(dir, ".stat-*")` → write → `chmod 0644` →
  `os.Rename`.
- [ ] **Step 4:** green under `go test -race -count=1 ./internal/suiterunner/`.
- [ ] **Step 5: Commit** `feat(0318): suiterunner sandboxed execution and durable results`.

---

### Task 3: scheduler — bounded parallel lane, serial lane, launch order

**Files:**
- Create: `internal/suiterunner/schedule.go`
- Test: `internal/suiterunner/schedule_test.go`

**Interfaces:**
- Consumes: `Target`, `ExecuteTarget`, `procRegistry` (Tasks 1–2).
- Produces:

```go
// Schedule partitions targets into the parallel lane (ceiling-descending,
// then path-ascending — the oracle's longest-budget-first order) and the
// serial lane (discovery order). Pure; trivially testable.
func Schedule(targets []Target) (par, ser []Target)

// runLanes executes the parallel lane with at most jobs in flight, waits for
// it to drain, then runs the serial lane one at a time. Each completion is
// reported to onDone (used for the stderr ticker) as it happens; result
// files are the durable record. Stops launching when ctx is cancelled and
// returns the set of targets that were never launched.
func runLanes(ctx context.Context, cfg Config, par, ser []Target, reg *procRegistry, onDone func(Target, Result)) (unlaunched []Target)
```

(`Config` is defined in Task 7's `run.go`; for this task declare it in
`schedule.go` with the fields the lanes need — `Bash string`, `Jobs int`,
`Work string`, `ExtraEnv []string` — Task 7 extends it in place.)

- [ ] **Step 1: Write failing tests:**
  - `TestSchedulePartitionAndOrder` — targets with ceilings 10/60/30 parallel and
    one serial; assert par order is 60,30,10 (ties broken by path) and ser keeps
    input order.
  - `TestSerialTargetsNeverOverlap` — the concurrency-safety mutation guard
    (spec mutation "schedule an unsafe target concurrently"). Build 3 serial-mode
    fixture scripts that each do: `mkdir "$OVERLAP_DIR/lock" || { echo "NOT OK - overlap"; exit 1; }`,
    sleep 0.2, `rmdir "$OVERLAP_DIR/lock"`; run with Jobs=4; assert all RC=0.
    Mutating the scheduler to run serial targets concurrently makes `mkdir` fail
    on the held lock → red. (`OVERLAP_DIR` passed via ExtraEnv, pointing at a
    `t.TempDir()`; mkdir is atomic, so the probe has no race of its own.)
  - `TestParallelBoundIsRespected` — 6 parallel fixtures that increment/decrement
    a live-counter via flock-free atomic dir creation (`mkdir "$D/slot.$i"`, count
    entries, fail if > Jobs) with Jobs=2; assert green. Keep sleeps at 0.1s.
  - `TestCancelStopsScheduling` — 4 parallel targets with Jobs=1 where the first
    cancels the context via onDone; assert `unlaunched` contains the targets that
    never started and no result files exist for them.
- [ ] **Step 2:** red. **Step 3:** implement with a buffered-semaphore goroutine
  pool + `sync.WaitGroup`; serial lane strictly after `wg.Wait()`.
- [ ] **Step 4:** `go test -race -count=1 ./internal/suiterunner/` green.
- [ ] **Step 5: Commit** `feat(0318): suiterunner bounded parallel and serial scheduling`.

---

### Task 4: result validation, deterministic aggregation, report, exit mapping

**Files:**
- Create: `internal/suiterunner/aggregate.go`
- Test: `internal/suiterunner/aggregate_test.go`

**Interfaces:**
- Consumes: `Target`, `Result`, `ReadResult`, threshold predicates (Tasks 1–2).
- Produces:

```go
// TargetOutcome classifies one scheduled target after the run.
type OutcomeKind int

const (
	OutcomePassed OutcomeKind = iota
	OutcomeFailed              // rc != 0
	OutcomeNoResult            // no durable result file
	OutcomeInvalidResult       // malformed / wrong-target / duplicate / observation conflict
	OutcomeInterrupted         // scheduled or launched, run was interrupted
)

type TargetOutcome struct {
	Target  Target
	Kind    OutcomeKind
	Result  Result // valid only for Passed/Failed
	Detail  string // human diagnostic for the invalid kinds
	OverDirect bool // authoritative direct crossing (solo/serial-mode lane)
	Screened   bool // contended parallel screening crossing
}

// ValidateResults joins the COMPLETE scheduled set against the stat dir.
// Failures (each spec bullet under "Durable result protocol"):
//   - a scheduled target with no file            -> OutcomeNoResult
//   - a file whose Target field != its filename  -> OutcomeInvalidResult (wrong-target)
//   - an unparseable/unsupported file            -> OutcomeInvalidResult (malformed)
//   - a stat-dir file matching NO scheduled base -> returned in unknown (unknown/unscheduled)
//   - leftover temp files (".stat-*")            -> the owning target OutcomeInvalidResult
//     (publication cannot be shown durable)
//   - file disagrees with runner-observed rc     -> OutcomeInvalidResult (conflict)
// It validates ALL targets, never stopping at the first failure.
func ValidateResults(scheduled []Target, observed map[string]Result, statDir string, interrupted map[string]bool) (outcomes []TargetOutcome, unknown []string)

// RenderReport writes the deterministic report to w: per-target rows sorted
// by basename then path (byte order), the SUITE line, FAILED:/NO RESULT:/
// OVER BUDGET: blocks with the oracle's exact wording (including the shard
// remedy line and the advisory/strict notes), and returns the aggregate
// tallies the exit mapping needs. Row format is the oracle's:
//   fmt.Sprintf("%-52s %4ss  rc=%s  ok=%-5s notok=%-4s", ...)
// with "  OVER BUDGET (ceiling %ds)" appended on OverDirect.
type Tally struct{ Files, Passed, Failed, Asserts, NoResult, Invalid, OverDirect int }
func RenderReport(w io.Writer, outcomes []TargetOutcome, unknown []string, wall int, verbose bool, logsDir string) Tally

// ExitCode applies the precedence 1 > 3 > 4 > 0. Invalid results and unknown
// files share exit 3 (the run certified nothing about the scheduled set).
// strictArmed is Task 5's confirmed/failed-breach flag.
func ExitCode(t Tally, unknownCount int, strictArmed bool) int
```

- [ ] **Step 1: Write failing tests** (all against a hand-built statDir in
  `t.TempDir()`; no child processes — this is the pure core, and the package's
  mutation-evidence anchor):
  - `TestValidateCompletenessOverWholeSet` — 3 scheduled, file present for 1,
    malformed for 1, absent for 1: assert all three outcomes present at once
    (mutation "omit a scheduled target from validation" / "stopping after the
    first failure" reddens this).
  - `TestValidateWrongTargetIdentity` — `stat/test_a.json` carrying
    `"target":"test_b.sh"` → OutcomeInvalidResult with `wrong-target` detail.
  - `TestValidateUnknownAndUnscheduled` — a `stat/test_ghost.json` for a target
    never scheduled → returned in `unknown`, and `ExitCode` with unknown>0 is 3.
  - `TestValidateDuplicatePublication` — a leftover `.stat-xyz` temp beside a
    valid record → OutcomeInvalidResult (`publication not shown durable`).
  - `TestValidateObservationConflict` — observed rc=1, durable file rc=0 →
    OutcomeInvalidResult (`conflicts with runner-observed execution`); a nominal
    result cannot conceal an execution failure.
  - `TestReportOrderIsCompletionIndependent` — same outcomes fed in two shuffled
    orders render byte-identical reports (mutation "aggregate in completion
    order" reddens).
  - `TestReportPreservesAllFailures` — 2 failed + 1 no-result + 1 invalid all
    appear; SUITE line tallies match; `FAILED:`/`NO RESULT:` name the right bases.
  - `TestExitPrecedence` — table: failed>0 → 1 even with no-result; no-result or
    invalid or unknown → 3; strictArmed → 4 only when failed==0 and noresult==0;
    else 0.
  - `TestAdvisoryBreachExitsZeroWithLoudReport` — OverDirect target, strict off:
    exit 0 AND report contains `OVER BUDGET:` and the line beginning
    `Advisory: the tests all passed, so this run does not fail on the breach (exit 0).`
    (ADR-0074 completed-with-observation; mutation "collapse ADR-0074 states"
    reddens either half.) The remedy line must lead with sharding and never
    suggest raising the ceiling (learning guard-remedy-must-not-teach-the-evasion):
    `Remedy: shard this file or extend an existing shard so each part stays under its ceiling.`
- [ ] **Step 2:** red. **Step 3:** implement. **Step 4:** green
  (`go test -race -count=1 ./internal/suiterunner/`).
- [ ] **Step 5: Commit** `feat(0318): suiterunner exact-result validation and deterministic aggregation`.

---

### Task 5: budget state machine, store, solo confirmation, strict mode

**Files:**
- Create: `internal/suiterunner/budgetstate.go`
- Test: `internal/suiterunner/budgetstate_test.go`

**Interfaces:**
- Consumes: thresholds (Task 1), `ExecuteTarget`+`Sandbox` (Task 2 — solo
  confirmation re-runs one target in a fresh sandbox under `<work>/solo/`),
  `TargetOutcome.Screened` (Task 4).
- Produces:

```go
// The store speaks the oracle's exact v1 format so a human can read both with
// one set of eyes: header "# docket-run-tests-budget-state v1", then
// "# next_due_sequence N", then TAB rows
// context_key state initial_overrun_streak overruns_since_confirmation
// last_parallel_seconds last_solo_seconds budget_seconds
// last_confirmation_result due_sequence test_path.
// The GO RUNNER'S store lives at its OWN default path
// <git-common-dir>/docket/development-test-budget-state.tsv — never the Bash
// runner's run-tests-budget-state.tsv. Two independent writers on one
// advisory file would corrupt both histories; separate files, same schema.
// This is a documented intentional deviation (spec "Parity and mutation
// strategy" allows it with a focused contract test — see
// TestStorePathIsNotTheBashRunners below).

type budgetState struct { /* per-key records mirroring the oracle's columns */ }

// ContextKey embeds every dimension that makes two measurements incomparable,
// in the oracle's exact rendering: "%s|j%d|c%d|%s|%s|b%d|m%s|s1"
// (path, jobs, cpus, GOOS-uname spelling via `uname -s`-equivalent
// runtime mapping, arch, ceiling, mode, schema).
func ContextKey(path string, jobs, cpus int, osName, arch string, ceiling int, mode Mode) string

// ApplyScreenObservations folds this run's contended parallel measurements in:
// overrun: unobserved/watching -> streak+1 (due at 5); parallel-sensitive/
// confirmed-breach -> since+1 (due at 10). Clean: watching resets streak AND
// drops the due stamp; sensitive/breach counters untouched.
func (s *budgetState) ApplyScreenObservations(obs []ScreenObs)

// ScheduleConfirmation picks AT MOST ONE due record (largest overdue, then
// lowest due_sequence, then path), announces `SERIAL CONFIRMATION DUE:`,
// re-runs the target solo (fresh sandbox under work/solo, DOCKET_RUNTESTS_SOLO=1,
// injected solo duration from the durations seam column 3), and classifies:
// rc!=0 -> FAILED (clears nothing, stays due); SoloOver -> confirmed-breach +
// `SERIAL CONFIRMED OVER BUDGET:` line; else parallel-sensitive/cleared.
// Every OTHER due record prints `SERIAL CONFIRMATION DEFERRED:`.
func (s *budgetState) ScheduleConfirmation(ctx context.Context, cfg Config, w io.Writer) 

// StrictConfirmCandidates confirms EVERY current candidate immediately,
// ignoring streak history and the one-per-run bound; a breach or failed
// confirmation arms strict (exit 4). Only confirmation outcomes are
// persisted, never screening counters.
func (s *budgetState) StrictConfirmCandidates(ctx context.Context, cfg Config, w io.Writer) (armed bool)

// EmitScreenReport prints one BUDGET WATCH: / PARALLEL-SENSITIVE: line per
// current candidate in path order, from the just-updated state.
func (s *budgetState) EmitScreenReport(w io.Writer)

// Load/Save: fail-open — missing, corrupt-header, unlockable (mkdir-based
// <path>.lock, ~3s of 100ms attempts), or unwritable state NEVER fails the
// run; malformed rows are skipped with one warning. Save is atomic
// temp-beside + rename, chmod 0600.
```

- [ ] **Step 1: Write failing tests** (pure state-machine tests use injected
  observations, no processes; the two confirmation tests use trivial scripts with
  injected durations so nothing sleeps):
  - `TestContextKeyRendersOracleFormat` — exact string equality against
    `"tests/test_a.sh|j8|c8|Darwin|arm64|b60|mparallel|s1"` given those inputs.
  - `TestStreakArmsAtFiveAndCleanResets` — 4 overruns + 1 clean + 5 overruns →
    due exactly once, with a due_sequence stamped at the fifth consecutive.
  - `TestSensitiveSinceCounterNeverResetsOnClean` — parallel-sensitive record,
    interleave 9 overruns with cleans → since==9, not due; 10th overrun → due
    (mutation "skip required serial confirmation" reddens: with the state machine
    bypassed, either the DUE line never prints or a breach prints without one).
  - `TestConfirmationPrecedesAuthoritativeBreach` — drive a due record whose solo
    duration (injected, column 3) exceeds `ceil*3/2`: output MUST contain
    `SERIAL CONFIRMATION DUE:` BEFORE `SERIAL CONFIRMED OVER BUDGET:`, and the
    breach line carries parallel secs, solo secs, and the threshold rendering.
  - `TestFailedConfirmationClearsNothing` — confirmation script exits 2: line
    `SERIAL CONFIRMATION FAILED:` appears, record stays due, since/streak
    untouched, last_confirmation_result=failed.
  - `TestOnePerRunAndDeferredLines` — two due records: exactly one DUE, the
    other exactly one DEFERRED with the verbatim deferred wording.
  - `TestScreeningNeverAuthoritative` — a screening candidate with strict OFF:
    report has `BUDGET WATCH:`; report does NOT contain any line beginning
    `OVER BUDGET` for it; exit stays 0 (mutation "treat parallel screening as
    authoritative" reddens).
  - `TestStrictConfirmsAllAndArms` — strict ON with two candidates: two DUE
    lines, arm on the breached one, screening counters NOT advanced in the
    saved store.
  - `TestStoreFailOpen` — corrupt header → run proceeds, state discarded;
    unwritable dir → warning path taken, no error; lock dir held by a stranger →
    bounded wait then no-history warning.
  - `TestStorePathIsNotTheBashRunners` — the resolved default path ends
    `/docket/development-test-budget-state.tsv` and never
    `run-tests-budget-state.tsv` (the documented intentional deviation's focused
    contract test).
- [ ] **Step 2:** red. **Step 3:** implement. **Step 4:** green under `-race -count=1`.
- [ ] **Step 5: Commit** `feat(0318): suiterunner screen-then-confirm budget state machine`.

---

### Task 6: interruption and process lifecycle

**Files:**
- Create: `internal/suiterunner/signal.go`
- Modify: `internal/suiterunner/execute.go` (registry signal methods)
- Test: `internal/suiterunner/signal_test.go`

**Interfaces:**
- Consumes: `procRegistry` (Task 2), `runLanes` ctx-cancel (Task 3).
- Produces:

```go
// InstallSignalHandling subscribes to SIGINT/SIGTERM. On the first signal it:
// (1) cancels ctx so no further target launches; (2) forwards the SAME signal
// to every registered process GROUP (kill(-pgid, sig)); (3) starts a bounded
// escalation timer (default 5s, override via killAfter for tests) after which
// surviving groups get SIGKILL; (4) records which signal fired so the exit
// code is 130 (INT) or 143 (TERM). A second signal is absorbed (the handler
// never re-enters). Returns a func() (signal Name, fired bool) the run
// entrypoint reads after the lanes drain.
func InstallSignalHandling(cancel context.CancelFunc, reg *procRegistry, killAfter time.Duration) (fired func() (os.Signal, bool), stop func())

// Interrupt path in Run (Task 7): after lanes return, already-durable results
// are still collected and validated; never-launched and launched-but-
// unfinished targets become OutcomeInterrupted; the report renders with an
// `INTERRUPTED (<signal>)` note per such row; exit is 130/143 regardless of
// how many targets had passed — an interrupted run can NEVER exit 0.
```

- [ ] **Step 1: Write failing tests** (self-signaling, deadline-polled — no bare
  sleeps as verdicts; note learning exec-optimization-erases-the-process-marker:
  identify children by PIDFILES they write, never by pgrep pattern):
  - `TestSignalReachesProcessGroupIncludingGrandchildren` — target script spawns
    a background grandchild that writes `$PIDS_DIR/grand.pid` then sleeps 30;
    after both pids exist, send SIGTERM to the registry's groups via the
    handler's forwarding path; poll with a 5s deadline that BOTH pids are gone
    (`syscall.Kill(pid, 0)` returns ESRCH). Mutation "skip signal propagation or
    orphan cleanup" (forward to pid instead of -pgid) leaves the grandchild
    alive → red.
  - `TestEscalationKillsIgnorers` — target script traps and ignores TERM
    (`trap '' TERM`), writes its pidfile, sleeps 30; killAfter=300ms; assert the
    process is gone within the deadline anyway (KILL escalation) and the test
    completes in well under 5s.
  - `TestInterruptedRunCannotPass` — a 3-target suite where target 2 sends
    SIGTERM to the runner's handler mid-run: `Run` returns 143, the report marks
    unfinished targets interrupted, and target 1's already-durable result is
    still present in the report (diagnostics preserved — an intentional
    improvement over the Bash oracle, which discards the report on interrupt;
    the differential harness (Task 8) therefore compares interruption on exit
    code + no-clean-pass, never report bytes).
  - `TestInterruptBetweenSchedulingAndLaunch` — cancel fired while targets are
    still queued (Jobs=1, first target long enough to hold the lane): queued
    targets appear as interrupted/never-launched, no result files for them,
    exit non-zero.
- [ ] **Step 2:** red. **Step 3:** implement (guard every kill with pgid>0;
  `signal.Stop` in `stop()`; absorb the second signal by draining the channel).
- [ ] **Step 4:** green under `-race -count=1`; confirm the package's total test
  wall time stays under ~8s (`go test -race -count=1 ./internal/suiterunner/`
  prints it) — trim fixture sleeps if not.
- [ ] **Step 5: Commit** `feat(0318): suiterunner signal forwarding, escalation, orphan prevention`.

---

### Task 7: `Run` entrypoint, CLI wiring, hygiene preflight, e2e Bash coverage

**Files:**
- Create: `internal/suiterunner/run.go`, `internal/cli/development_test_cmd.go`,
  `tests/test_devtest_runner.sh`
- Modify: `internal/cli/root.go` (register subcommand + asset-independent key),
  `tests/runtime-budgets.tsv` (row + EXPECTED_TOTAL re-seed)
- Test: `internal/suiterunner/run_test.go`, `tests/test_devtest_runner.sh`

**Interfaces:**
- Consumes: everything from Tasks 1–6.
- Produces:

```go
// Config is the fully-resolved run configuration; the CLI layer resolves it
// and suiterunner.Run never reads global state it doesn't list here.
type Config struct {
	RepoRoot      string   // git toplevel of the checkout under test
	TestsDir      string   // default RepoRoot/tests; DOCKET_RUNTESTS_TESTS_DIR overrides
	BudgetsPath   string   // default RepoRoot/tests/runtime-budgets.tsv; DOCKET_RUNTESTS_BUDGETS overrides
	Jobs          int      // default runtime.NumCPU(); DOCKET_RUNTESTS_JOBS overrides (>=1 or usage error)
	Bash          string   // DOCKET_BASH_PATH else exec.LookPath("bash"); missing => exit 2
	HygienePath   string   // RepoRoot/scripts/check-test-source-hygiene.sh
	StatePath     string   // default <git-common-dir>/docket/development-test-budget-state.tsv; DOCKET_RUNTESTS_STATE overrides
	Strict        bool     // DOCKET_RUNTESTS_STRICT=1
	DurationsPath string   // DOCKET_RUNTESTS_TEST_DURATIONS
	Verbose       bool     // reserved false in 0318 (no flags)
	Stdout, Stderr io.Writer
	Work          string   // runner-owned scratch; "" => os.MkdirTemp
	ExtraEnv      []string
	KillAfter     time.Duration // 0 => 5s
}

// Run executes the whole flow: discover -> resolve targets -> hygiene
// preflight (bash HygienePath -- targets...; violation => print the oracle's
// zero-files-executed wording, return 5; unusable checker => 2) -> schedule
// -> execute -> validate -> budget machinery -> render -> exit code.
// Interruption per Task 6. Every internal uncertainty fails closed to a
// non-zero, attributable exit — never a fabricated pass.
func Run(ctx context.Context, cfg Config) int
```

CLI: in `root.go`, beside `developmentInstallCmd`, add

```go
developmentTestCmd := &cobra.Command{
	Use:   "test",
	Short: "Run the complete configured test suite from this checkout",
	Args:  cobra.NoArgs,
	// Non-interactive whole-suite runner (change 0318). The report streams to
	// this process's stdout/stderr and the exit code carries the verdict
	// (documented beside "Exit contract" in internal/suiterunner/run.go), so
	// it bypasses the JSON result presenter deliberately.
	RunE: ...
}
developmentCmd.AddCommand(developmentInstallCmd, developmentTestCmd)
```

and register the command's key in the `assetIndependent` set — the
`PersistentPreRunE` asset-dependence guard refuses unregistered commands, and
a suite runner must never read installed version-tree assets. Follow how
`development install` threads its exit status through the CLI's existing
exit-code path (inspect the `result`/presenter flow in `root.go` and
`cmd/docket`); the acceptance is behavioral, pinned by the e2e test below:
raw report on stdout, ticker on stderr, faithful exit codes.

- [ ] **Step 1: Go-side failing tests** (`run_test.go`, all on synthetic suites
  in `t.TempDir()` via `Config{TestsDir: fixture}`):
  - `TestRunGreenSuite` — 3 passing scripts → exit 0, SUITE line
    `files=3 passed=3 failed=0`.
  - `TestRunRedSuite` — one failing → exit 1, `FAILED:` names it.
  - `TestRunHygieneViolationExecutesNothing` — a fixture whose test source
    contains a backtick in a double-quoted string; assert exit 5 AND no result
    files / no log files were created (zero targets executed).
  - `TestRunHygieneCheckerMissingFailsClosed` — HygienePath absent → exit 2.
  - `TestRunNoResultExitsThree` — point `Config.Work` at a pre-created dir and
    `chmod 0555` its `stat/` subdirectory so publication fails for every
    target: assert exit 3, a `NO RESULT:` line naming the targets, and no
    `FAILED:` line (a missing result is a harness failure, not a test
    failure). Restore the mode before test cleanup.
  - `TestRunUsageErrors` — `DOCKET_RUNTESTS_JOBS=0` → exit 2; unresolvable bash
    (Bash: "/nonexistent") → exit 2.
- [ ] **Step 2:** red. **Step 3:** implement `run.go`, then the CLI wiring.
  `go build ./...` clean.
- [ ] **Step 4: Write `tests/test_devtest_runner.sh`** (e2e through the real
  command — this is the file that proves the SOURCE entry, not the package):
  - Header comment: what it proves, CACHES note copied in spirit from
    `tests/test_go_toolchain.sh` (pin GOMODCACHE/GOCACHE to
    `<git common dir>/docket-go-cache/{mod,build}`, `GOFLAGS=-modcacherw`) so
    the sandboxed HOME does not force cold module downloads.
  - Canonical assert helper, `set -uo pipefail`, `REPO` resolution like sibling
    tests.
  - Build once: `go build -o "$scratch/docket" ./cmd/docket` is NOT the tested
    entry — the tested entry is `go run ./cmd/docket development test` exactly
    as the gate will run it; use it directly (Go caches the build between
    invocations, so repeated `go run` here is cheap after the first).
  - Case 1 (green): synthetic fixture dir with two passing tests + a budgets
    table; `DOCKET_RUNTESTS_TESTS_DIR=$fix DOCKET_RUNTESTS_BUDGETS=$fix/budgets.tsv
    DOCKET_RUNTESTS_JOBS=2 DOCKET_RUNTESTS_STATE=$scratch/state.tsv` →
    rc 0, stdout has `SUITE files=2 passed=2`.
  - Case 2 (red): add a failing test → rc 1, `FAILED:` present.
  - Case 3 (screening advisory): injected durations
    (`DOCKET_RUNTESTS_TEST_DURATIONS`) putting one parallel file over
    `ceil*5/2` → rc 0 and a `BUDGET WATCH:` line; assert NO line begins
    `OVER BUDGET` (use `grep -E -e '^OVER BUDGET'` shape; remember negated
    greps need the `-e` form).
  - Case 4 (serial-mode authoritative): a `serial`-mode fixture with injected
    parallel-lane duration over `ceil*3/2` → rc 0 (advisory), row suffix
    `OVER BUDGET (ceiling` present, summary `OVER BUDGET:` present, and with
    `DOCKET_RUNTESTS_STRICT=1` → rc 4.
  - Case 5 (usage error): `go run ./cmd/docket development test extra-arg` →
    non-zero rc and a usage-style diagnostic (cobra `Args: cobra.NoArgs`).
  - Case 6 (interruption): launch a run in the background over a fixture whose
    one test writes a `started` sentinel then sleeps 20; poll for the
    sentinel, then `kill -TERM $bgpid` where `$bgpid` is the backgrounded
    `go run` pid (`go run` forwards signals to the built binary); `wait
    $bgpid` and assert the recorded rc is 143 and the captured output never
    contains a clean `SUITE … failed=0` pass line. Bound every poll at 10s
    with 0.2s intervals.
- [ ] **Step 5:** Add the budget row: `tests/test_devtest_runner.sh<TAB>40<TAB>parallel`
  in `tests/runtime-budgets.tsv` (then measure standalone serial with
  `/usr/bin/time -p bash tests/test_devtest_runner.sh` and re-cut the row to the
  table's rule — next multiple of 5 above the worst serial reading plus 5s
  margin, min 10, max 60); re-seed `EXPECTED_TOTAL` in
  `tests/test_runtime_budgets.sh` and note the re-seed reason in the row comment.
- [ ] **Step 6:** `bash scripts/run-tests.sh tests/test_devtest_runner.sh` → PASS;
  `go test -race -count=1 ./internal/suiterunner/ ./internal/cli/` → PASS.
- [ ] **Step 7: Commit** `feat(0318): docket development test command with e2e coverage`.

---

### Task 8: differential harness — Bash oracle vs Go runner

**Files:**
- Create: `tests/test_devtest_differential.sh`
- Modify: `tests/runtime-budgets.tsv` (row + EXPECTED_TOTAL re-seed)

**Interfaces:**
- Consumes: `go run ./cmd/docket development test` (Task 7) and the frozen
  `scripts/run-tests.sh`, both driven over the SAME synthetic fixture suite via
  the shared env seams (`DOCKET_RUNTESTS_TESTS_DIR`,
  `DOCKET_RUNTESTS_TEST_DURATIONS`, budgets table; Bash side gets `-j N`
  `--budgets` `--budget-state` flags, Go side the equivalent env vars).
- Produces: the parity evidence AC 18 requires; no reusable code.

- [ ] **Step 1: Write the harness.** One fixture-builder function per scenario;
  each scenario runs BOTH runners and compares NORMALIZED observations.
  Normalization (an awk/sed pass over each captured output): strip absolute
  temp paths, PIDs, and every `<N>s` wall-clock number and the per-run seconds
  column; KEEP target identity, row order, rc values, ok/notok counts, failure
  category lines (`FAILED:`, `NO RESULT:`), budget clause KINDS, and exit codes.
  Scenarios (assert exit-code equality AND normalized-line equality per
  scenario, with the per-scenario carve-outs noted):
  1. discovery + stable order: 4 files named to exercise C collation → identical
     row order and SUITE tallies.
  2. success and ordinary failure: one failing file → both exit 1, same
     `FAILED:` names.
  3. launch/infrastructure failure: a target that is a directory or 0-length
     unreadable file → both non-zero with the failure attributed (rc values may
     differ in TEXT; compare category, not wording — document this carve-out in
     a comment).
  4. concurrency classification: budgets table marking one file `serial` →
     both report it and neither overlaps it (order identical).
  5. duplicate basename rejection: `a/test_x.sh b/test_x.sh` as Bash-runner
     arguments → exit 2 with the duplicate diagnostic. The Go leg is
     unreachable here by construction (the command takes no target arguments
     and maxdepth-1 discovery cannot produce a basename collision), so this
     scenario is Bash-behavior-only, with a comment naming the Go runner's
     equivalent coverage:
     `TestResolveTargetsRejectsDuplicateBasenamesAndMissingFilesTogether`.
  6. budget screening equivalence: identical injected durations over `5/2` →
     both emit a `BUDGET WATCH:` line for the same target, both exit 0.
  7. serial confirmation + authoritative breach: seed BOTH state stores to
     due-state by running the qualifying overrun scenario 5 times per runner
     (injected durations make each run fast), then one more run with an
     injected solo duration over `3/2` → both emit
     `SERIAL CONFIRMATION DUE:` then `SERIAL CONFIRMED OVER BUDGET:` for the
     same target and still exit 0 (advisory).
  8. tri-state interpretation: green-with-advisory-breach (exit 0 + finding),
     red (exit 1), and no-result (exit 3 — Bash leg: fixture test that
     `kill -9`s its own runner subshell is fragile; instead run Bash leg with a
     stat-dir sabotage? The Bash runner offers no seam — assert the Go leg's
     exit-3 against `scripts/run-tests.md`'s documented exit-3 meaning by
     grepping the CONTRACT doc for `produced no result`, an anchored
     producer-consumer pairing, and note the asymmetry in a comment).
  Interruption parity is deliberately OUT of this file (machine-timing flake
  risk under suite contention); it is covered by `TestInterruptedRunCannotPass`
  (Go) and the documented-deviation note (report preserved vs discarded), per
  the spec's "documents the difference and proves the replacement contract".
- [ ] **Step 2:** run the file solo: `bash scripts/run-tests.sh tests/test_devtest_differential.sh`
  → PASS. Sanity-mutate once (flip the Go aggregate sort to completion order via
  a scratch edit — keep a `cp` backup of the edited Go file first, NEVER restore
  with `git checkout` (learning mutation-restore-needs-a-backup-copy)) → scenario
  1 reddens → restore from the backup copy → green again.
- [ ] **Step 3:** budget row (start `50<TAB>parallel`, then measure standalone
  serial and re-cut per the table's rounding rule) + `EXPECTED_TOTAL` re-seed
  with the reason noted.
- [ ] **Step 4: Commit** `test(0318): bash-vs-go differential parity harness`.

---

### Task 9: the cutover — config, fixture re-cut, RC workflow, docs, fidelity guards

**Files:**
- Modify: `.docket.yml`, `internal/config/fixtures_test.go`,
  `.github/workflows/release-candidate.yml`, `AGENTS.md`, `tests/README.md`,
  `scripts/run-tests.md`, `tests/runtime-budgets.tsv`
- Create: `testdata/repositories/v0.9.6/` (tree + `PROVENANCE.md`),
  `tests/test_devtest_cutover.sh`

**Interfaces:**
- Consumes: the working command from Task 7.
- Produces: `finalize.test_command: go run ./cmd/docket development test` as the
  sole configured gate command; every later consumer (docket-build's final
  full-suite gate included) resolves it from `.docket.yml` — never a second copy.

- [ ] **Step 1: Write the failing guard test** `tests/test_devtest_cutover.sh`
  (this is the "invoke an installed binary at a source-validation gate" mutation
  guard — write it FIRST, watch it fail against the un-cut-over tree):
  - Assert `.docket.yml`'s first `finalize:` block carries
    `test_command: go run ./cmd/docket development test` — anchored inside the
    `finalize:` mapping (awk from `/^finalize:/` to the next top-level key),
    never a bare whole-file grep.
  - Assert the value begins `go run ./cmd/docket` and NOT a bare `docket ` —
    the branch-faithful shape that cannot select an installed binary.
  - Assert `.github/workflows/release-candidate.yml`'s suite step still DERIVES
    the command from `.docket.yml` (grep for the derivation comment clause
    `finalize.test_command` AND for the absence of any literal
    `run-tests.sh` invocation line in the workflow's run blocks), and that no
    `bash $test_cmd`-shaped wrap remains (a multi-word `go run …` command must
    be executed as a shell command line, not handed to bash as a script path).
  - Assert `AGENTS.md` and `tests/README.md` each name
    `go run ./cmd/docket development test` within a bounded gap of their
    whole-suite instruction (learning prose-guard-binds-phrase-to-claim: bind
    the command to the "whole suite" claim by collapsing whitespace first —
    learning phrase-grep-over-wrapped-prose — never a bare presence grep), and
    that neither presents `scripts/run-tests.sh` as the way to run the whole
    suite anymore.
  - Assert `scripts/run-tests.md` carries the frozen-oracle notice (Step 5).
- [ ] **Step 2: Flip `.docket.yml`.** Replace the `test_command:` line and its
  0227 comment block with:

```yaml
  # The Go-native whole-suite runner (change 0318), entered from SOURCE so the
  # gate tests the exact checkout under review and can never select a stale
  # installed binary. The Bash runner scripts/run-tests.sh remains present as
  # the frozen parity oracle until changes 0369/0370 retire it.
  test_command: go run ./cmd/docket development test
```

- [ ] **Step 3: Re-cut the frozen fixture** (the drift guard's OWN remedy —
  learning config-edit-trips-its-own-frozen-drift-guard; never edit v0.9.5):
  - `cp -R testdata/repositories/v0.9.5 testdata/repositories/v0.9.6`, then copy
    the NEW live `.docket.yml` over `testdata/repositories/v0.9.6/docket-self/repo/.docket.yml`.
  - Write `testdata/repositories/v0.9.6/PROVENANCE.md` following the v0.9.5
    file's shape (source: this repository's live `.docket.yml` at the 0318
    cutover commit; date; what changed: `finalize.test_command`).
  - In `internal/config/fixtures_test.go`, `TestFixtureDocketSelf`: point
    `docketSelfRoot` at `"../../testdata/repositories/v0.9.6"` and re-derive the
    expectation to `snap.Effective.Finalize.TestCommand.Value != "go run ./cmd/docket development test"`.
    The blocker-set expectations are unchanged (the edit touches only
    test_command); verify rather than assume:
    `go test -count=1 ./internal/config/ -run TestFixtureDocketSelf`.
- [ ] **Step 4: RC workflow.** In `.github/workflows/release-candidate.yml`,
  "Run the resolved test suite": keep the awk derivation from `.docket.yml`
  verbatim (one source), and replace the execution line
  `bash $test_cmd 2>&1 | tee suite.log` with
  `sh -c "$test_cmd" 2>&1 | tee suite.log` — the same `/bin/sh -c` shape
  `commandArgv` uses in `internal/app/gate_drive.go`, so both gates launch the
  identical process tree. Keep the budget-vocabulary grep block unchanged (the
  Go runner emits the same clause spellings by construction — Tasks 4–5).
  Keep the "Provision suite Bash" step: the Bash SHARDS still need GNU Bash and
  `DOCKET_BASH_PATH` is now consumed by the Go runner's child-bash resolution.
- [ ] **Step 5: Docs, derived from a grep, not a hand list** (AGENTS.md rule:
  never hand-list the sites of a literal you are gating). Run
  `grep -rn "run-tests.sh" . --include="*.md" --include="*.yml" --include="*.sh"`,
  exclude `testdata/` (frozen), `docs/changes/`, `docs/results/`, `docs/adrs/`,
  `docs/superpowers/` (point-in-time records — rewriting them falsifies
  history), and `tests/`+`scripts/` implementation (frozen oracle + its own
  tests). What remains is the live contributor-doc set; expected members and
  their edits:
  - `AGENTS.md` "Guards and tests" whole-suite bullet: the suite command is
    still "whatever `finalize.test_command` resolves to" (keep that sentence —
    it is the no-second-copy rule); update the descriptive tail to say the
    resolved command is the Go-native `go run ./cmd/docket development test`,
    that budget clause lines keep their meanings, and repoint the "see" pointer
    from `scripts/run-tests.md` to `tests/README.md`. Check whether `CLAUDE.md`
    is a symlink to `AGENTS.md` (`ls -l CLAUDE.md`); if it is a real copy, apply
    the same edit there.
  - `tests/README.md` "how to run" head: canonical whole-suite command becomes
    `go run ./cmd/docket development test`; keep the focused-file examples
    (`scripts/run-tests.sh --verbose tests/test_x.sh` stays a documented
    focused tool — the spec permits focused commands; it may no longer be
    presented as THE whole-suite gate).
  - `scripts/run-tests.md` header: add a short frozen-oracle notice — this
    runner remains present and green as the parity oracle and migration corpus
    for changes 0369/0370; the canonical whole-suite gate is
    `go run ./cmd/docket development test` resolved via `finalize.test_command`.
    Do not rewrite the rest of the contract (the Bash runner still honors it).
- [ ] **Step 6:** Budget row for `tests/test_devtest_cutover.sh`
  (`10<TAB>parallel`, re-measure and re-cut if needed) + `EXPECTED_TOTAL`
  re-seed with reason.
- [ ] **Step 7: Verify.**
  - `bash scripts/run-tests.sh tests/test_devtest_cutover.sh` → PASS (was
    failing at Step 1).
  - `go test -count=1 ./internal/config/` → PASS (fixture re-cut proven).
  - `go run ./cmd/docket development test` from the worktree root → full real
    corpus, expect green end-to-end (this is AC 22's first full rehearsal;
    on an authoritative `SERIAL CONFIRMED OVER BUDGET:` finding, disposition it
    explicitly — shard or re-cut the named row per the table's own rules —
    never ignore it).
- [ ] **Step 8: Commit** `feat(0318): cut finalize.test_command, RC gate, and docs over to go run ./cmd/docket development test`.

---

### Task 10: final parity sweep and evidence

**Files:**
- Modify: none expected (fix-forward only if the sweep finds red).

- [ ] **Step 1:** `gofmt -l . | grep -v '^testdata/'` → empty;
  `go vet ./...` → clean; `go test -race -count=1 ./...` → PASS.
- [ ] **Step 2:** Frozen-oracle proof: `bash scripts/run-tests.sh` (the OLD
  entry, full corpus) → exit 0. This is AC 5 — the prior workflow is present
  and green, untouched.
- [ ] **Step 3:** Canonical-entry proof: `go run ./cmd/docket development test`
  (full corpus) → exit 0, SUITE tallies match Step 2's file/assert counts
  (wall clock will differ; tallies must not). Any `SERIAL CONFIRMED OVER
  BUDGET:` line from either runner is an authoritative finding to resolve or
  explicitly disposition before this task completes (AC 22).
- [ ] **Step 4:** Confirm the diff surface honors the non-goals: `git diff
  --no-renames --name-only <pre-branch-base>` contains NO deletions under
  `scripts/`, `tests/` (existing files), no edits under
  `testdata/repositories/v0.9.2`–`v0.9.5`, and no changes to
  `internal/app/gate_drive.go` or the finalize/evidence consumers (they read
  the resolved command; nothing about them changes).
- [ ] **Step 5:** Record in the task report (for the results file the build
  loop writes): the measured serial timings of the three new Bash test files
  against their rows and the remaining margin AS NUMBERS (learning
  budget-headroom-is-spent-before-it-is-breached), and the two documented
  intentional deviations (separate budget-state store path; interruption
  preserves the report instead of discarding it) with their focused contract
  tests named.
- [ ] **Step 6: Commit** only if fixes were needed
  (`fix(0318): final parity sweep findings`).

---

## Spec acceptance-criteria map (self-review)

| AC | Where |
|---|---|
| 1 command exists, non-interactive | Task 7 (cobra `Args: cobra.NoArgs`, no flags, env seams only) |
| 2 complete authoritative suite | Task 1 Discover (same glob rule) + Task 10 Step 3 tally match |
| 3 one branch-faithful entry everywhere | Task 9 (.docket.yml + RC workflow derive-from-config + docs) |
| 4 all Go targets + Bash shards run through it | shards ARE `tests/test_*.sh` members; Task 10 Step 3 |
| 5 frozen prior workflow green | Global Constraints + Task 10 Steps 2/4 |
| 6 no shim / migration / deletion | Global Constraints + Task 10 Step 4 |
| 7 isolated per-target state | Task 2 Sandbox + `TestSandboxIsolation` |
| 8 only safe targets overlap, bounded | Task 3 (`TestSerialTargetsNeverOverlap`, `TestParallelBoundIsRespected`) |
| 9–11 exact durable results, atomic | Task 2 (atomic publish) + Task 4 ValidateResults tests |
| 12 deterministic aggregation, all failures | Task 4 (`TestReportOrderIsCompletionIndependent`, `TestReportPreservesAllFailures`) |
| 13 interruption contract | Task 6 (all four tests) |
| 14 BUDGET WATCH screening | Task 5 (`TestScreeningNeverAuthoritative`) + Task 7 Case 3 |
| 15 confirmation precedes authoritative breach | Task 5 (`TestConfirmationPrecedesAuthoritativeBreach`) |
| 16 confirmation outcomes distinguishable | Task 5 (FAILED / cleared / breached / deferred tests) |
| 17 ADR-0074 tri-state coverage | Task 4 (`TestAdvisoryBreachExitsZeroWithLoudReport`, `TestExitPrecedence`) + Task 8 scenario 8 |
| 18 differential coverage | Task 8 |
| 19 synthetic fixtures | Tasks 2–7 (all fixtures deterministic, injected durations) |
| 20 mutation evidence | named mutation guards in Tasks 3,4,5,6,8,9 (each spec mutation bullet has a designated reddening test; TDD steps prove each fails first) |
| 21 internal failures fail closed, distinct | Task 7 (`TestRunHygieneCheckerMissingFailsClosed`, `TestRunUsageErrors`, exit-2 family) |
| 22 full suite green via canonical runner | Task 9 Step 7 + Task 10 Step 3 |
| 23 merged state usable alone | cutover complete in-branch; oracle intact |
| 24 nothing out of scope | Task 10 Step 4 diff audit |

Spec mutation-bullet → reddening test: omit-from-validation →
`TestValidateCompletenessOverWholeSet`; accept zero/dup/malformed/wrong-target →
`TestValidate*` family; completion-order aggregation →
`TestReportOrderIsCompletionIndependent` + differential scenario 1; unsafe
concurrent scheduling → `TestSerialTargetsNeverOverlap`; skip signal/orphan →
`TestSignalReachesProcessGroupIncludingGrandchildren`; screening-as-authoritative
→ `TestScreeningNeverAuthoritative` + runner Case 3; skip serial confirmation →
`TestSensitiveSinceCounterNeverResetsOnClean` +
`TestConfirmationPrecedesAuthoritativeBreach`; collapse ADR-0074 →
`TestAdvisoryBreachExitsZeroWithLoudReport` + `TestExitPrecedence`; installed
binary at the gate → `tests/test_devtest_cutover.sh`.
