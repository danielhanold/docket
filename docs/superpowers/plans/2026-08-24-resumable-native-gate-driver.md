<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0342 — Harden autonomous build/implement agents against the suite-yield deadlock (ADR-0024)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-25-0342-harden-autonomous-build-implement-agents-against-the-suite-y.md)**
<!-- docket:backlink:end -->
# Resumable Native Gate Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native Go gate *driver* above the existing raw process supervisor so every workflow suite run advances in short slice-bounded synchronous calls with one persisted deadline and execution identity, and migrate every workflow gate caller onto it — eliminating the full-budget foreground polling loops that let a forked agent background-and-yield (ADR-0024).

**Architecture:** A new `internal/gatedrive` package holds a versioned drive state machine that composes the existing `internal/process` supervisor. It persists a drive record under the Git common dir (`<git-common-dir>/docket/gate-drives/v1/<id>/`), advances in ≤30s slices, and returns one of four typed outcomes (`WAITING`/`PASSED`/`FAILED`/`HALTED`). An `internal/app` service seam and `internal/cli` adapters (`docket gate drive start|advance|handoff|claim`) expose it; finalize composes the service in-process. Build task workers, the build controller final gate, implement-next's evidence re-mint/re-gates, and finalize's local gate all consume the driver instead of raw verbs. A new `run-waiting` verify-run verdict is derived purely from agreeing local receipts. A whole-repository, mutation-tested structural guard forbids raw-gate use in workflow-shaped source.

**Tech Stack:** Go 1.x (`github.com/danielhanold/docket`), cobra CLI, the existing `internal/process` native supervisor, protocol-v1 JSON envelopes, Bash test shards under `tests/`, `scripts/run-tests.sh` as the whole suite.

**Spec:** `docs/superpowers/specs/2026-08-24-resumable-native-gate-driver-design.md` — the plan argues from the spec; executors MUST read both. Every task's detailed behavioral requirements (state/outcome matrix, fingerprint dimensions, relaunch conditions, handoff CAS, `run-waiting` agreement conditions, guard categories, acceptance criteria) live in the spec sections referenced per task and are binding even where this plan sketches rather than fully transcribes them.

## Global Constraints

- The existing `internal/process` supervisor remains the SOLE authority for process identity, terminal status, session isolation, durable logs, stop, recovery, cleanup. The driver composes it; it never re-implements process liveness, never uses `pgrep`/process-name matching, never parses logs to infer state. (Spec: Constraints 1, 4, 9.)
- A workflow call never blocks for the full observation budget: it reaches a terminal or returns a typed nonterminal after one short slice. Production slice target = 30s; it is plumbing, not a user knob. Tests inject clock + slice. (Spec: Constraint 2, Deadline semantics.)
- One absolute deadline is computed once at `start` from resolved `gate_observation_budget` and never extended by slices, restarts, handoffs, or the one relaunch. Backward clock jumps that could lengthen the budget HALT. (Spec: Constraint 3, Deadline semantics.)
- At most one owned raw process tree is live per drive; uncertain ownership fails closed and can never authorize another launch. (Spec: Constraint 4.)
- `PASSED` certifies exact repo bytes at drive start: HEAD + staged + unstaged + untracked + modes + rename/deletion + symlink values, recomputed at every ownership boundary and before accepting a terminal pass. Any drift ⇒ stop-if-owned + `HALTED`, never red. (Spec: Constraint 5, Persisted execution identity.)
- `FAILED` means the suite itself completed red. Process death, malformed state, deadline expiry, identity uncertainty, handoff mismatch are NEVER converted into `FAILED`. (Spec: Constraint 6.)
- Waiting is private runtime state — NOT change frontmatter, NOT a new change status, NOT a `claimed_at` refresh, NOT a board move. (Spec: Constraint 7, Local run-waiting verdict.)
- Go owns deterministic mechanics (launch, time, identity, persistence, transition, ownership transfer); AI agents keep judgment (editing, repair, task completion, review, publication). (Spec: Constraint 8.)
- No new daemon/notification channel/long-lived coordination process; the per-run supervisor stays the only long-lived Go process. (Spec: Constraint 9.)
- Config values (`gate_observation_budget`, `finalize.test_command`) come from authoritative config, never agent input. Only a trusted `PASSED` may feed `EvidenceRecord`.
- Protocol is versioned and SHARED by CLI, app seam, and tests — one typed document, no divergent copies. Diagnostic prose never emits the launch command, env values, worktree diff, file contents, or ownership credential.
- Follow repo conventions in `CLAUDE.md`: guards keyed on syntactic SHAPE not enumerated spellings; guards are mutation-tested; whole suite runs at the build gate (`scripts/run-tests.sh`); `git add` by explicit path; frontmatter/YAML rules; comment cross-refs anchor on symbol/verbatim-clause never line numbers.
- Merge boundary is atomic even though the branch builds in stages: no merged workflow may choose between the raw loop and the driver. (Spec: Migration and compatibility.)

---

## File Structure

New package `internal/gatedrive/` (driver state machine, persistence, clock/ownership/fingerprint seams):
- `drive.go` — drive record types, protocol-v1 outcome document, versioned schema.
- `store.go` — durable record persistence under the Git common dir (atomic temp+rename, lock, generation CAS).
- `clock.go` — injectable clock seam; monotonic-in-slice + persisted UTC deadline binding.
- `fingerprint.go` — repository execution-identity fingerprint (index/unstaged/untracked/modes/rename/deletion/symlink), hashes only.
- `driver.go` — the state machine: `Start`, `Advance`, `Handoff`, `Claim`; composes `internal/process` via an injected process seam.
- `ownership.go` — opaque drive-id + owner-generation + single-use handoff-receipt CAS.
- `*_test.go` — table-driven state-machine, clock, fingerprint, ownership, and process-integration tests.

`internal/app/`:
- `gate_drive.go` — app service seam wrapping `internal/gatedrive` for in-process callers (finalize) + CLI; typed result mapping.
- `gate_drive_test.go`.
- Modify `finalize_rebase.go` / `finalize_context.go` — replace `observeToTerminal` 30-min loop with the driver service admitting a nonterminal `WAITING`.
- Modify `run_verify.go` (or wherever `run` verdicts live) — add `run-waiting` derived from agreeing receipts.

`internal/cli/`:
- `gate.go` — add `drive` subcommand group: `start|advance|handoff|claim`.
- `gate_test.go`, `cmd/docket/gate_cli_test.go` — CLI adapter tests.

`internal/assets/embedded/tree/skills/` (authored contracts; regenerated to `~/.claude` + other harnesses via install):
- `docket-build/references/gate-caller-loop.md` — replace Bash fence + `jq` with the typed driver contract.
- `docket-build/SKILL.md`, `docket-build-task` contract — add `WAITING` return + handoff requirement.
- `docket-finalize-change/references/gate-failure.md` / gate docs — driver contract.
- `docket-implement-next` step 6 gate text — driver contract for evidence re-mint / re-gate.
- primitive/operator docs — keep raw verbs as primitive/operator APIs only.

Guard:
- `tests/test_gate_driver_boundary.sh` (+ mutation proof) — whole-repo structural guard.

ADR:
- `docs/adrs/<NNNN>-structured-gate-waiting-and-ownership-handoff.md` — recorded in the review phase via docket-adr (task placeholder at end).

---

## Task 1: Drive record model + protocol-v1 outcome document

**Files:**
- Create: `internal/gatedrive/drive.go`
- Test: `internal/gatedrive/drive_test.go`

**Interfaces:**
- Produces: `type Outcome string` with `WAITING`, `PASSED`, `FAILED`, `HALTED`; `type DriveDoc struct` carrying `ProtocolVersion int`, `DriveID string`, `Generation string`, `Attempt int`, `Deadline time.Time`, `Outcome Outcome`, `Cause string` (typed, optional), `RawRunDir string` (populated on PASSED only). `func (DriveDoc) MarshalJSON`/canonical form matching the repo's protocol-v1 envelope convention.
- Produces: `type driveRecord struct` (persisted schema) with `SchemaVersion int` and every field from spec "Persisted execution identity" (repo identity, worktree path, change/task/phase identity, branch/ref + full HEAD OID, fingerprint, resolved command + cwd, config provenance + budget, env hash, timestamps + fixed deadline + last-accepted clock + protocol version, current raw run dir + raw ownership identity + attempt + relaunch count + terminal receipt, current owner generation or single-use handoff generation).

- [ ] **Step 1: Write failing test** for outcome-doc JSON shape and schema-version presence.

```go
func TestDriveDocCarriesProtocolAndOutcome(t *testing.T) {
    d := DriveDoc{ProtocolVersion: 1, DriveID: "d1", Generation: "g1", Attempt: 1,
        Outcome: PASSED, RawRunDir: "/runs/abc"}
    b, err := json.Marshal(d)
    if err != nil { t.Fatal(err) }
    var m map[string]any
    _ = json.Unmarshal(b, &m)
    if m["protocol_version"].(float64) != 1 { t.Fatalf("missing protocol_version") }
    if m["outcome"] != "PASSED" { t.Fatalf("outcome not surfaced") }
    if _, ok := m["raw_run_dir"]; !ok { t.Fatalf("passed doc must expose raw run dir") }
}

func TestDriveDocRedactsSecrets(t *testing.T) {
    // launch argv, env values, worktree diff, credential must never appear in the doc.
    d := DriveDoc{ProtocolVersion: 1, Outcome: HALTED, Cause: "identity-mismatch"}
    b, _ := json.Marshal(d)
    for _, banned := range []string{"argv", "env", "diff", "credential", "token"} {
        if strings.Contains(strings.ToLower(string(b)), banned) {
            t.Fatalf("doc leaked %q", banned)
        }
    }
}
```

- [ ] **Step 2: Run to verify it fails** — `go test ./internal/gatedrive/ -run TestDriveDoc -v` ⇒ FAIL (package/types undefined).
- [ ] **Step 3: Implement** `drive.go` with the four outcome constants, `DriveDoc` with explicit json tags (no argv/env/diff/credential fields), and the private `driveRecord` schema struct with `SchemaVersion`. Derive canonical JSON per the repo's existing `Envelope` convention in `internal/app`.
- [ ] **Step 4: Run to verify pass** — same command ⇒ PASS.
- [ ] **Step 5: Commit** — `git add internal/gatedrive/drive.go internal/gatedrive/drive_test.go && git commit -m "feat(gatedrive): drive record model and protocol-v1 outcome document"`

---

## Task 2: Injectable clock + one-shot deadline binding

**Files:**
- Create: `internal/gatedrive/clock.go`
- Test: `internal/gatedrive/clock_test.go`

**Interfaces:**
- Consumes: `driveRecord` (Task 1).
- Produces: `type Clock interface { Now() time.Time; Since(time.Time) time.Duration }`; `func computeDeadline(start time.Time, budget time.Duration) time.Time`; `func (r *driveRecord) deadlineState(now time.Time) (expired bool, backwardJump bool)`. Slice bound uses monotonic clock while the process is alive; across invocations the persisted UTC deadline + last-accepted clock value bind elapsed time.

- [ ] **Step 1: Write failing tests** — (a) deadline computed once from budget and unchanged by later `Now()`; (b) forward jump past deadline ⇒ expired; (c) backward jump that could lengthen budget ⇒ `backwardJump=true` (caller HALTs); (d) budget zero ⇒ deadline == start (one observation then stop-and-halt contract).

```go
func TestDeadlineFixedOnce(t *testing.T) {
    start := time.Unix(1000, 0).UTC()
    dl := computeDeadline(start, 30*time.Minute)
    if !dl.Equal(start.Add(30*time.Minute)) { t.Fatalf("deadline not fixed from budget") }
}
func TestBackwardClockJumpHalts(t *testing.T) {
    r := &driveRecord{Deadline: time.Unix(2000,0).UTC(), LastClock: time.Unix(1500,0).UTC()}
    _, backward := r.deadlineState(time.Unix(1400,0).UTC())
    if !backward { t.Fatalf("backward jump must be flagged") }
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./internal/gatedrive/ -run 'TestDeadline|TestBackward' -v`.
- [ ] **Step 3: Implement** `clock.go` with a real monotonic clock and the deadline/backward-jump logic; add `LastClock`/`Deadline` fields to `driveRecord` if not already present.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "feat(gatedrive): fixed-once deadline with injectable clock and backward-jump halt"`

---

## Task 3: Repository execution-identity fingerprint

**Files:**
- Create: `internal/gatedrive/fingerprint.go`
- Test: `internal/gatedrive/fingerprint_test.go`

**Interfaces:**
- Produces: `type Fingerprint struct` (structural metadata + per-dimension hashes, NOT content); `func ComputeFingerprint(repoDir string, git GitSeam) (Fingerprint, error)` covering index bytes, unstaged tracked bytes, untracked file bytes, file modes, deletion state, rename state, symlink values (symlinks hashed by link value, never followed); `func (a Fingerprint) Equal(b Fingerprint) bool`. `type GitSeam interface` abstracting the git reads for injection.

- [ ] **Step 1: Write failing tests**, one dimension mutated at a time, each proving inequality: staged byte change, unstaged byte change, untracked file added, mode 0644→0755, file deleted, file renamed, symlink target changed. Plus: identical dirty state ⇒ `Equal` true; symlink hashed by value not target (a dangling symlink still fingerprints).

```go
func TestFingerprintDetectsModeChange(t *testing.T) {
    repo := newDirtyRepo(t) // helper: seeds a temp git repo
    a, _ := ComputeFingerprint(repo, realGit{})
    chmod(t, repo, "x.sh", 0o755)
    b, _ := ComputeFingerprint(repo, realGit{})
    if a.Equal(b) { t.Fatalf("mode change must alter fingerprint") }
}
```

- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** `fingerprint.go`; store hashes + structural metadata only; never persist file/diff content.
- [ ] **Step 4: Run to verify pass** for every dimension.
- [ ] **Step 5: Commit** — `git commit -m "feat(gatedrive): per-dimension repository execution-identity fingerprint"`

---

## Task 4: Durable drive store — atomic writes, lock, generation CAS

**Files:**
- Create: `internal/gatedrive/store.go`
- Test: `internal/gatedrive/store_test.go`

**Interfaces:**
- Consumes: `driveRecord` (Task 1).
- Produces: `type Store struct`; `func OpenStore(gitCommonDir string) *Store`; `func (s *Store) NewDrive(rec driveRecord) (id string, gen string, err error)` (opaque high-entropy id + initial generation); `func (s *Store) Load(id string) (driveRecord, error)`; `func (s *Store) CAS(id string, expectGen string, mutate func(*driveRecord) error) (newGen string, err error)`. Records live at `<gitCommonDir>/docket/gate-drives/v1/<id>/`, owner-only perms, sibling-temp + atomic rename, flock. Unknown schema/impossible transition ⇒ typed error (caller HALTs), never best-effort migration. User-supplied ids validated before path construction (no traversal/symlink escape).

- [ ] **Step 1: Write failing tests** — round-trip persist/load; CAS succeeds only with matching generation and advances the generation; concurrent CAS: exactly one of two racing writers wins; unknown `SchemaVersion` load ⇒ error; a `../` or symlink id is rejected before any path is touched; files created with 0600 / dir 0700.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** `store.go` reusing `internal/process/atomic.go` patterns (`writeAtomicJSON`, `ensurePrivateDir`) and an flock like `internal/process/lock.go`. Generate ids with crypto entropy.
- [ ] **Step 4: Run to verify pass**, including the race test (`go test -race` where supported).
- [ ] **Step 5: Commit** — `git commit -m "feat(gatedrive): durable owner-private drive store with generation CAS"`

---

## Task 5: Ownership generations + single-use handoff receipt CAS

**Files:**
- Create: `internal/gatedrive/ownership.go`
- Test: `internal/gatedrive/ownership_test.go`

**Interfaces:**
- Consumes: `Store` (Task 4), `Fingerprint` (Task 3).
- Produces: `func (d *Driver) Handoff(id, ownerGen string) (DriveDoc, error)` and `func (d *Driver) Claim(id, handoffID string) (DriveDoc, error)` at the ownership layer (state-machine wiring lands in Task 6, but the CAS primitives and receipt type live here): `type handoffReceipt struct` (single-use, carries change/task/phase + drive + generation chain); `verifyOwner`, `invalidateOwner`, `writeHandoffReceipt`, `consumeHandoffCAS`. A claim that lost the race or no longer fingerprint-matches acquires NO partial authority.

- [ ] **Step 1: Write failing tests** — current owner can create a handoff (old token invalidated afterward, `advance` by old owner rejected); a fresh claimant with an exact fingerprint match consumes the receipt and gets a NEW generation; two claimants race one receipt ⇒ exactly one wins; a plain `WAITING` drive (no handoff) cannot be claimed; a fingerprint mismatch at claim rejects.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** `ownership.go` on top of the Task-4 CAS; receipt is single-use and forms one unambiguous change/task/phase/drive/generation chain.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "feat(gatedrive): ownership generations and single-use handoff CAS"`

---

## Task 6: Driver state machine — Start/Advance over the process seam

**Files:**
- Create: `internal/gatedrive/driver.go`
- Test: `internal/gatedrive/driver_test.go`

**Interfaces:**
- Consumes: `Store`, `Clock`, `Fingerprint`, ownership primitives, and a `ProcessSeam` abstracting `internal/process` (`Launch(LaunchRequest)`, `Observe(runDir)`, `Stop(runDir,reason)`).
- Produces: `type Driver struct`; `func NewDriver(store *Store, clock Clock, proc ProcessSeam, git GitSeam) *Driver`; `func (d *Driver) Start(req StartRequest) (DriveDoc, error)`; `func (d *Driver) Advance(id, ownerGen string) (DriveDoc, error)`. `StartRequest` carries repoDir, worktree, change/task/phase identity, resolved command + cwd, budget, and an `idempotentSuiteGate bool` (only such gates may relaunch). Mapping (Spec: State transitions and recovery, Typed outcomes):
  - live `running` + slice expired ⇒ record observation, return `WAITING` (no shell monitor/sleep/notification created).
  - raw `passed` ⇒ revalidate fingerprint; match ⇒ `PASSED` (+ raw run dir); mismatch ⇒ stop-if-owned + `HALTED`.
  - raw `failed` ⇒ `FAILED`.
  - `stopped` not proven-initiated-by-this-drive ⇒ `HALTED`, never red.
  - `signaled`/`vanished` ⇒ prove no owned tree survives; relaunch ONCE only if all 5 conditions hold (idempotent gate; former tree proven gone; no prior relaunch; worktree/command/config/env identity match; deadline remains); else `HALTED`, preserving both attempts.
  - deadline expired with live run ⇒ stop, re-observe, `HALTED` (no relaunch earned).
  - unreadable/unknown observation, corrupt transition, schema mismatch, identity disagreement ⇒ `HALTED`; only exact `running` is retryable.
  - budget zero ⇒ one observation; if still live, stop + `HALTED`.

- [ ] **Step 1: Write failing table-driven tests** with injected clock/slice/process/fs/git seams covering EVERY state→outcome pair and every forbidden transition from Spec "Verification strategy → State-machine tests": several WAITING slices retaining one drive/run/attempt/deadline; terminal arriving between slices; zero-budget; forward+backward clock; malformed observation fails closed; stop applied / stop no-op + re-observe / uncertain stop; one admitted death-relaunch + every refusal reason; interruption between atomic writes; schema/version mismatch. No test sleeps for production durations.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** `driver.go`; persist an atomic record at each transition; recompute fingerprint at each ownership boundary and before accepting a pass; keep exactly one live owned tree.
- [ ] **Step 4: Run to verify pass** for the full table (`-race` where supported).
- [ ] **Step 5: Commit** — `git commit -m "feat(gatedrive): Start/Advance state machine with single relaunch and fail-closed halts"`

---

## Task 7: Process-integration tests against the real supervisor

**Files:**
- Create: `internal/gatedrive/integration_test.go`

**Interfaces:** Consumes the real `internal/process.Service` as the `ProcessSeam` with test-only short slices and a child spanning multiple slices.

- [ ] **Step 1: Write failing integration tests** (Spec: Process integration tests): each driver invocation returns within its slice bound; supervisor+child identity stable across invocations; terminating a driver CLI invocation does not kill/duplicate the child; a fresh process resumes from disk and consumes the exact terminal status; a process-tree death permits at most one non-overlapping relaunch; deadline expiry stops the whole owned tree; durable logs + passed-run evidence remain usable. Oracle = recorded PIDs/session identity from the native receipt, NEVER process-name matching.
- [ ] **Step 2: Run to verify fail** (driver not yet wired to real service, or child helper missing).
- [ ] **Step 3: Implement** a test-only multi-slice child (small Go helper or shell sleeper invoked through the supervisor) and wire the real service seam.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "test(gatedrive): real-supervisor process integration across slices and relaunch"`

---

## Task 8: Handoff/repository race tests

**Files:**
- Modify: `internal/gatedrive/ownership_test.go` (extend) or Create `internal/gatedrive/handoff_test.go`

- [ ] **Step 1: Write failing tests** (Spec: Handoff and repository tests): clean and dirty handoffs; mutate staged/unstaged/untracked bytes, names, deletions, exec modes, symlink values one dimension at a time ⇒ each rejects claim OR terminal consumption; identical dirty state succeeds with NO WIP commit; race two claimants over one receipt ⇒ one wins; old owner cannot advance after handoff; plain WAITING cannot be claimed; a fresh owner consumes a terminal written while no agent was active.
- [ ] **Step 2–4:** run-fail, implement any missing claim/terminal fingerprint checks in `ownership.go`/`driver.go`, run-pass.
- [ ] **Step 5: Commit** — `git commit -m "test(gatedrive): dirty-handoff identity and single-winner claim races"`

---

## Task 9: App service seam (`internal/app/gate_drive.go`)

**Files:**
- Create: `internal/app/gate_drive.go`
- Test: `internal/app/gate_drive_test.go`

**Interfaces:**
- Consumes: `internal/gatedrive` (all above).
- Produces: `type GateDriveService struct`; `func NewGateDriveService(...) *GateDriveService`; methods `Start`, `Advance`, `Handoff`, `Claim` returning the shared `DriveDoc` mapped into the app's protocol-v1 `Envelope` (like `GateResult`). Resolves the Git common dir, config-provenanced budget/command. This is the in-process seam finalize composes; it MUST NOT shell out to docket's own CLI.

- [ ] **Step 1: Write failing test** that the service maps a driver `WAITING`/`PASSED`/`FAILED`/`HALTED` into the protocol-v1 doc and that only `PASSED` exposes the raw run dir; a command failure (unparseable args / unrecognized drive) is distinct from a recognized `FAILED`/`HALTED` workflow result.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** `gate_drive.go`.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "feat(app): in-process gate-drive service seam over the driver"`

---

## Task 10: CLI adapters — `docket gate drive start|advance|handoff|claim`

**Files:**
- Modify: `internal/cli/gate.go`
- Test: `internal/cli/gate_test.go`, `cmd/docket/gate_cli_test.go`

**Interfaces:**
- Consumes: `GateDriveService` (Task 9).
- Produces: a `drive` subcommand group on `gateCmd` with `start`, `advance`, `handoff`, `claim`. Every op takes opaque drive/claim identifiers (NOT PIDs/PGIDs/raw state/deadlines). `--json` emits the shared protocol-v1 doc; human text names identity + outcome only (redaction). The CLI is a thin adapter over the same state machine as the app seam.

- [ ] **Step 1: Write failing CLI tests** — `gate drive start -- <argv>` returns a doc with a drive id + outcome; `advance` resumes; `handoff` then `claim` transfers; argument-parse failure is a command failure not a workflow outcome; no secret/argv/env leaks in output.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** the subcommands in `gate.go`.
- [ ] **Step 4: Run to verify pass** (`go test ./internal/cli/ ./cmd/docket/ -run Gate -v`).
- [ ] **Step 5: Commit** — `git commit -m "feat(cli): docket gate drive start/advance/handoff/claim adapters"`

---

## Task 11: Migrate finalize's local gate onto the driver

**Files:**
- Modify: `internal/app/finalize_rebase.go` (remove `observeToTerminal` 30-min synchronous loop), `internal/app/finalize_context.go` as needed
- Test: `internal/app/finalize_*_test.go`

**Interfaces:** `FinalizeGate` composes `GateDriveService`; admits a nonterminal `WAITING`; caller carries the opaque continuation and re-enters the same local-gate phase after a slice without rerunning completed rebase/repair phases. Only trusted `PASSED` feeds `EvidenceRecord`; red feeds existing integration-repair; waiting does not. `finalize.test_command` still from authoritative config.

- [ ] **Step 1: Write failing tests** (Spec: Finalize tests) — one application call returns after a slice (`WAITING`); resuming the local-gate phase does NOT repeat completed rebase/repair; a `FAILED` routes to integration-repair, `WAITING` does not; only `PASSED` mints evidence.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** — delete the polling loop, compose the driver, thread the continuation.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "refactor(finalize): drive local gate in slices, retire 30-minute polling loop"`

---

## Task 12: `run-waiting` verdict + verify-run receipts

**Files:**
- Modify: the run-verify implementation (`internal/app/run_verify.go` or equivalent — locate via `grep -rn "run-incomplete\|run-complete\|run-halted" internal/`), `scripts/*verify-run*` mapping if present
- Test: corresponding `_test.go`

**Interfaces:** verify-run gains `run-waiting <change-id> <opaque-handoff-id> <phase>` — one line, consumed by spelling not exit code, no owner credential/command exposed. Emitted ONLY when all Spec "Local run-waiting verdict" conditions independently agree (claimed in-progress change; recorded branch+worktree exist and match handoff; HEAD + full dirty fingerprint match drive+handoff receipts; driver record recognized with an explicit unclaimed handoff and a live deadline unless a durable terminal is waiting; referenced raw run + native ownership receipt match the active attempt; the change/task/phase/drive/generation chain is unambiguous). Completed run postconditions precede a stale handoff; a valid `run-waiting` precedes ordinary `run-incomplete`; a persisted run-halt stays terminal; missing local state on another machine never invents waiting.

- [ ] **Step 1: Write failing tests** — a fully-agreeing receipt chain yields `run-waiting`; EVERY single receipt mutation makes it disappear (mutation-style: HEAD drift, fingerprint drift, claimed handoff, expired deadline w/o terminal, mismatched raw run, broken chain); completed postconditions win over a stale handoff; run-halt stays terminal.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** the derivation from agreeing local receipts.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "feat(run): local receipt-derived run-waiting verify-run verdict"`

---

## Task 13: Teach every verdict consumer about `run-waiting`

**Files:**
- Modify: top-level dispatch consumers + delegated runner dispatch (authored dispatch text under `internal/assets/embedded/tree/` managed `docket:dispatch` blocks; `runner-dispatch` in `scripts/` + its contract) and any Go consumer of run verdicts
- Test: relevant contract/Go tests

**Interfaces:** consumers never treat `run-waiting` as completed/failed or as permission to start another change; when an exact continuation dispatch is available they resume that handoff, else report the waiting continuation and stop safely. Delegated runner dispatch relays the verdict faithfully without redesigning its detachment protocol/budget.

- [ ] **Step 1: Write failing tests** that each consumer maps `run-waiting` to resume-or-report-and-stop, never to complete/fail/next-change.
- [ ] **Step 2–4:** run-fail, update consumers + regenerate managed dispatch text, run-pass.
- [ ] **Step 5: Commit** — `git commit -m "feat(dispatch): every run-verdict consumer understands run-waiting"`

---

## Task 14: Migrate build task workers + controller onto the driver contract

**Files:**
- Modify: `internal/assets/embedded/tree/skills/docket-build/references/gate-caller-loop.md`, `docket-build/SKILL.md`, the `docket-build-task` contract file, `docket-build/references/gate-execution*.md`
- Test: authored-to-generated drift tests + any build-contract shard under `tests/`

**Interfaces:** `docket-build-task` return vocabulary gains `WAITING` (alongside `COMPLETE`/`NEEDS_ESCALATION`/`BLOCKED`); a valid task `WAITING` MUST name an explicit driver handoff (stale "still waiting" prose is invalid). Contract forbids direct raw-gate verbs, background suite processes, agent-authored polling, notification waits. `docket-build` controller replaces the executable Bash fence + `jq` parsing with the typed driver contract; understands task `WAITING`, owns the continuation while the worker is absent, prevents another task starting in the shared worktree; its final gate uses the driver; a passed task gate does not substitute for the final gate; the final raw run dir stays available to evidence.

- [ ] **Step 1: Write/adjust failing tests** — the authored contract contains the typed driver operations and the handoff-required `WAITING`, and NO Bash gate loop / `jq` workflow parsing; drift test proves generated copies match authored source.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Rewrite** the authored contracts; run `docket install` / the asset-regeneration path to refresh generated copies.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "refactor(docket-build): typed driver contract, WAITING-with-handoff, retire Bash gate loop"`

---

## Task 15: Migrate implement-next evidence re-mint + re-gate; retire caller loop + jq; regenerate assets

**Files:**
- Modify: `docket-implement-next` step-6 gate text (embedded authored source), any remaining `gate-caller-loop.md` references, primitive/operator docs
- Test: drift tests + repo `jq`-dependency check

**Interfaces:** implement-next evidence re-mint and every post-review re-gate use the driver, not a copied launch/observe loop. The Bash fence in `gate-caller-loop.md` retires along with `jq` as a workflow gate-sequencing dependency; its replacement documents the typed driver operations/dispositions; primitive/operator docs keep the raw verbs as primitive/operator APIs (not workflow recipes). All generated skill copies/manifests/agent defs/managed dispatch/golden assets regenerated mechanically. Historical specs/results/accepted ADRs are NOT rewritten.

- [ ] **Step 1: Write failing tests** — no workflow-shaped authored contract calls raw verbs or uses `jq` for gate sequencing; implement-next gate text names the driver; drift tests green.
- [ ] **Step 2: Run to verify fail.**
- [ ] **Step 3: Implement** the rewrites + regenerate.
- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -m "refactor(implement-next): driver-based evidence re-mint/re-gate; retire workflow jq"`

---

## Task 16: Mutation-tested architectural boundary guard

**Files:**
- Create: `tests/test_gate_driver_boundary.sh`
- Test: the guard's own mutation proofs (inside the script or a sibling `tests/test_gate_driver_boundary_mutation.sh`)

**Interfaces:** a derived whole-repository guard that finds raw gate use by syntactic SHAPE (not a hand-listed call-site allowlist, not Markdown-only) and classifies each match as executable-workflow / primitive-impl-or-test / operator / point-in-time-prose. At minimum covers: raw `docket gate launch|observe|stop` shapes in skills/agent-defs/executable scripts; direct app-orchestration `GateLaunch|GateObserve|GateStop` calls outside the driver layer; workflow copies that parse raw observation state or recreate a sleep/poll loop; task-level `WAITING` contracts that omit an explicit handoff identity. Permits the raw CLI impl, native process/app primitive packages, primitive-level tests/fixtures, operator diagnostics/recovery/cleanup docs, immutable historical records — categories derived from source shape, no per-file allowlist.

- [ ] **Step 1: Write the guard** keyed on shape, following `CLAUDE.md` (declare leading `--` in grep; capture-then-grep under pipefail; awk `[^[:space:]]`; derive sites from a whole-repo grep sorted into prose vs executable).
- [ ] **Step 2: Write the mutation proofs** — inject a structurally-valid direct raw call into a workflow-shaped fixture ⇒ guard rejects; remove the driver call OR the handoff requirement from an authored workflow contract ⇒ that contract's test goes red. Prove each mutation reddens; a mutation that stays green fails.
- [ ] **Step 3: Run** the guard against the current tree ⇒ green (all workflow callers already migrated by Tasks 11–15); run the mutation proofs ⇒ each reddens then reverts.
- [ ] **Step 4: Wire** the guard into the suite (`scripts/run-tests.sh` discovery / `tests/README.md` placement).
- [ ] **Step 5: Commit** — `git commit -m "test(guard): mutation-proven gate-driver architectural boundary"`

---

## Task 17: Record the ADR (structured waiting + ownership handoff + nearest-owner continuation)

**Files:**
- Create: `docs/adrs/<NNNN>-structured-gate-waiting-and-ownership-handoff.md` (number assigned by docket-adr)

> **Note for the executor/orchestrator:** the ADR is recorded in the review phase via the `docket-adr` agent (it assigns the number, updates the index, commits on the docket branch, and returns the number, which is then added to the change's `adrs:` relation). This task is a placeholder marking the requirement; do NOT hand-author the ADR file or number here.

- [ ] **Step 1:** During review, dispatch docket-adr with Context/Decision/Consequences: first-class structured waiting, explicit fingerprinted ownership handoff, nearest-owner continuation; ADR-0024 remains correct (a forked agent still cannot yield+notify; this design makes every short call synchronous and stores the continuation outside the transcript, requiring a deliberate handoff); ADR-0095 remains authoritative for raw native supervision (this composes it).
- [ ] **Step 2:** Record the returned number in the change's `adrs:` relation via `docket change reconcile` relations update.

---

## Task 18: Whole-suite gate + budget investigation

**Files:** none (verification task)

- [ ] **Step 1:** Run the architectural boundary guard + mutations, authored-to-generated asset drift tests, Go tests under `-race` where supported, shell contract shards, and the configured whole suite (`scripts/run-tests.sh`).
- [ ] **Step 2:** Investigate every trailing `SERIAL CONFIRMED OVER BUDGET:` report (a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is screening only; confirm serially per `scripts/run-tests.md`).
- [ ] **Step 3:** Confirm the Spec "Acceptance criteria" bullets hold: no full-budget synchronous polling loop remains; no executable workflow composes raw gate primitives outside the driver; every foreground driver call is slice-bounded; one deadline+fingerprint survives CLI restart+handoff; only exact-match handoff authorizes a fresh owner; waiting terminates at the nearest controller and consumes neither repair nor escalation; failure/death/deadline/ambiguous-ownership stay distinct; ≤1 non-overlapping relaunch under the original deadline; `run-waiting` is local/read-only/receipt-derived/understood by every consumer; finalize+evidence-re-mint+re-gate+build gates all use the driver; a gate pass cannot bypass evidence/review/publication/mark-implemented; the guard is mutation-proven and the whole suite passes.
- [ ] **Step 4: Commit** any final fixups — `git commit -m "test: whole-suite gate green for resumable gate driver"`

---

## Self-Review

- **Spec coverage:** Problem/Terminology → Tasks 1–8 (model, clock, fingerprint, store, ownership, state machine, integration, handoff). Driver-above-raw-verbs + typed outcomes + CLI surface → Tasks 6, 9, 10. Durable drive record (location/identity/deadline) → Tasks 1, 2, 4. State transitions + single relaunch + fail-closed → Task 6. Explicit handoff + nearest-owner continuation → Tasks 5, 8, 14. Workflow migration (build tasks, controller, implement-next, finalize) → Tasks 11, 14, 15. Local `run-waiting` + consumers → Tasks 12, 13. Evidence/cleanup independence → covered by Task 6 (raw run dir on PASSED) + Task 9 + existing EvidenceRecord (unchanged verifier). Guard → Task 16. Verification strategy → Tasks 6, 7, 8, 12, 16, 18. ADR → Task 17.
- **Placeholder scan:** the only deliberate deferral is Task 17 (ADR number assigned by docket-adr at review) — flagged explicitly, not a code placeholder. Behavioral detail for tables/dimensions is delegated to named spec sections, which travel with the plan per the header.
- **Type consistency:** `DriveDoc`/`driveRecord` (Task 1), `Clock` (Task 2), `Fingerprint`/`GitSeam` (Task 3), `Store`/`CAS` (Task 4), `handoffReceipt` (Task 5), `Driver`/`ProcessSeam`/`StartRequest` (Task 6), `GateDriveService` (Task 9) are used with consistent names/signatures downstream.

**Note on execution:** this is an autonomous docket run — the orchestrator dispatches docket-build to execute these tasks task-by-task and gates on a single whole-suite run; no interactive execution-mode choice is posed.
