<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0428 — Recover legacy gate history without blocking unrelated worktree admission](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-14-0428-recover-legacy-gate-history-without-blocking-unrelated-workt.md)**
<!-- docket:backlink:end -->
# Recover Legacy Gate History Without Blocking Unrelated Worktree Admission — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** An ordinary `gate drive start` (scoped, scopeless, or raw `gate launch`) admits normally in a repository whose Git common directory carries completed pre-0375 schema-2 gate-drive history, recovers safely-recoverable HALTED history automatically inside the original admission, refuses with an exact `inventory-legacy-drive-<id>` locator when it must, and shares the same classifier with a new cataloged `gate.history.cleanup` operation.

**Architecture:** A new history reader/classifier in `internal/gatedrive` (history.go) understands schema 2 for *historical assessment only* (never execution), recognizes terminal PASSED/FAILED history *before* resolving possibly-removed worktree paths, and reuses the *existing* process recovery predicate (`internal/process` classifyRun) for HALTED assessment — extracted into an assess/apply split, never a second liveness implementation. `inventoryLegacyDrives` delegates to the classifier and returns a compact `LegacyHistorySummary` that rides `OwnershipError` on refusal and `DriveDoc` on success; `internal/app/gate_drive.go` propagates the typed inventory stage + locator instead of the misleading generic "this worktree" message. A thin `Driver.CleanupHistory` plus an app operation and CLI leaf expose the same classifier as `docket gate history cleanup`.

**Tech Stack:** Go (stdlib only), cobra CLI registration, the repo's own suite runner.

**Spec:** `docs/superpowers/specs/2026-09-14-recover-legacy-gate-history-without-blocking-unrelated-workt-design.md` (synchronized on the `docket` metadata branch). The change file is `docs/changes/active/0428-recover-legacy-gate-history-without-blocking-unrelated-workt.md`.

## Global Constraints

- Baseline main: `06ebb52c058894b564ac2a8432922ddf4b2d56b3` (spec-stated, verified at reconcile).
- The build gate runs the WHOLE suite via the configured `build.test_command` (`go run ./cmd/docket development test`) from the source checkout — never only the tests named here. Read the budget report even on green (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines).
- Preserve original drive records, logs, and lock files. No retirement store, reference census, age filter, log pruning, schema rewrite, or `--older-than`/`--prune-logs` options.
- No new executable compatibility for schema 2: the execution reader (`readStored`) keeps accepting exactly v4 + v3. Historical assessment is a separate reader.
- Reuse or extract the existing process recovery predicate (`internal/process` `classifyRun`); do NOT add a second liveness implementation. A probe error and a clean absence are different answers; "unknown" never shares a branch with "absent" when the other branch authorizes recovery (learnings: `probe-error-is-not-clean-absence`, `duplicated-gate-copies-the-whole-predicate`).
- A bare Observe `vanished` is NOT teardown proof (it can mean only that the supervisor lock is free).
- Lock order stays WORKTREE ADMISSION → SCOPE → DRIVE; no outer gate lock acquired from an inner one; no lock files removed; empty lock files prove nothing.
- Diagnostics: validate every drive id (`validateID`, 32 lowercase hex) before rendering it; an arbitrary directory name never enters a diagnostic — use the safe inventory-level locator `inventory-legacy-drives` when there is no valid id. Never expose raw record contents, argv, env values, gate context, or ownership credentials.
- Keep the compatible refusal reason token `unresolved-execution`.
- Every guard added here is mutation-tested (strip it, watch the assert redden, restore from a backup copy — never `git checkout --` over uncommitted work; defeat Go's test cache with `-count=1`).
- New JSON fixtures under `testdata/` are frozen DATA: if a repo-wide guard trips on them, exclude the fixture directory boundedly and mutation-test the exclusion (learning: `frozen-fixture-corpus-trips-repo-wide-scans`).
- TDD per task; one commit per task; commit messages `feat(0428): …` / `fix(0428): …` / `test(0428): …` / `docs(0428): …`.

## File Structure

- `internal/process/recover.go` — Modify: split `classifyRun` into assess/apply; export `Service.ClassifyRun`.
- `internal/gatedrive/history.go` — Create: schema-2 historical reader, `historicalDrive` view, classifier, `LegacyHistorySummary`, `recoverySeam`, `Driver.CleanupHistory`.
- `internal/gatedrive/history_test.go` — Create.
- `internal/gatedrive/testdata/legacy-v2/*.json` — Create: frozen schema-2 fixtures.
- `internal/gatedrive/admission.go` — Modify: `inventoryLegacyDrives` delegates to the classifier; `reserveWorktreeExecution` seam + summary; delete `admissionObservationProvesTeardown`.
- `internal/gatedrive/ownership.go` — Modify: `OwnershipError` gains `Legacy *LegacyHistorySummary`.
- `internal/gatedrive/driver.go` — Modify: `ProcessSeam` gains `ClassifyRun`; thread summary into `AdmissionTicket`/`DriveDoc`.
- `internal/gatedrive/drive.go` — Modify: `DriveDoc.LegacyHistory`; upgrade-boundary doc comment.
- `internal/app/gate.go` — Modify: raw-launch reserve passes the service as the recovery seam.
- `internal/app/gate_drive.go` — Modify: propagate stage/locator/summary; inventory-specific next-action message.
- `internal/app/gate_history.go` + `internal/app/gate_history_test.go` — Create: `gate.history.cleanup` operation + result document.
- `internal/app/schema_registry.go` — Modify: register the new operation.
- `internal/cli/gate.go` — Modify: `gate history` group + `cleanup` leaf with capability metadata.
- `internal/cli/install.go` — Modify: allowlist `gate history` / `gate history cleanup`.
- `docs/adrs/` — Create one ADR (via the docket-adr skill) on the historical-assessment vs executable-compatibility boundary.

---

### Task 1: Assess/apply split of the process recovery predicate

**Files:**
- Modify: `internal/process/recover.go`
- Test: `internal/process/recover_classify_test.go` (create; existing recover tests stay untouched and green)

**Interfaces:**
- Consumes: existing unexported `classifyRun`, `readManifest`, `probeFlock`, `readTerminal`, `readStopped`, `readAbandoned`, `recoverGroupProbe`, `writeAtomicJSON`, `runIDPattern`.
- Produces: `func (s *Service) ClassifyRun(runDir string, mark bool) (RecoveryEntry, error)` — one run slot's disposition. With `mark=false` it NEVER writes; a slot whose recorded group is provably absent (the arm that would earn a fresh abandoned marker) reports disposition `"abandonable"`. With `mark=true` it behaves exactly as `Recover`'s per-slot pass (writes the marker, reports `"abandoned-marked"`). All other dispositions (`live`, `terminal`, `stopped`, `already-abandoned`, `needs-inspection`, `invalid`, `unresolved-establishment`) are identical in both modes.

- [ ] **Step 1: Write the failing tests**

```go
package process

import (
	"os"
	"path/filepath"
	"testing"
)

// A provably-absent recorded group is "abandonable" under assess mode and no
// marker is written; the same slot under mark mode writes exactly the marker
// Recover would.
func TestClassifyRunAssessDoesNotMark(t *testing.T) {
	svc := newTestService(t) // reuse the package's existing test constructor helper
	runDir := makeAbandonedCandidateSlot(t) // helper: manifest + free lock + no verdicts, PGID of a dead group (reuse the fixture pattern from the existing Recover abandoned-marker test)
	old := recoverGroupProbe
	recoverGroupProbe = func(int) probeAnswer { return probeAbsent }
	defer func() { recoverGroupProbe = old }()

	entry, err := svc.ClassifyRun(runDir, false)
	if err != nil {
		t.Fatalf("assess: %v", err)
	}
	if entry.Disposition != "abandonable" {
		t.Fatalf("want abandonable, got %q (%s)", entry.Disposition, entry.Reason)
	}
	if _, err := os.Stat(filepath.Join(runDir, abandonedFile)); !os.IsNotExist(err) {
		t.Fatal("assess mode must write no abandoned marker")
	}

	entry, err = svc.ClassifyRun(runDir, true)
	if err != nil {
		t.Fatalf("mark: %v", err)
	}
	if entry.Disposition != "abandoned-marked" {
		t.Fatalf("want abandoned-marked, got %q", entry.Disposition)
	}
	if _, err := os.Stat(filepath.Join(runDir, abandonedFile)); err != nil {
		t.Fatal("mark mode must write the abandoned marker")
	}
}

// An unprovable group probe is needs-inspection in BOTH modes — never
// collapsed into abandonable/marked (probe error != clean absence).
func TestClassifyRunUnprovableProbeIsNeedsInspectionBothModes(t *testing.T) {
	svc := newTestService(t)
	runDir := makeAbandonedCandidateSlot(t)
	old := recoverGroupProbe
	recoverGroupProbe = func(int) probeAnswer { return probeUnknown }
	defer func() { recoverGroupProbe = old }()
	for _, mark := range []bool{false, true} {
		entry, err := svc.ClassifyRun(runDir, mark)
		if err != nil || entry.Disposition != "needs-inspection" {
			t.Fatalf("mark=%v: want needs-inspection, got %q err=%v", mark, entry.Disposition, err)
		}
	}
}

// ClassifyRun validates its input like Recover validates its root.
func TestClassifyRunRejectsRelativeAndNonSlotPaths(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.ClassifyRun("relative/dir", false); err == nil {
		t.Fatal("relative path must be refused")
	}
	dir := t.TempDir() // basename is not a 32-hex run id
	if entry, err := svc.ClassifyRun(dir, false); err != nil || entry.Disposition != "foreign" {
		t.Fatalf("non-run-id slot must be foreign, got %q err=%v", entry.Disposition, err)
	}
}
```

Adapt the two helpers (`newTestService`, `makeAbandonedCandidateSlot`) from the existing recover tests in `internal/process` — copy the fixture construction that today's abandoned-marker Recover test uses; do not invent a new manifest shape.

- [ ] **Step 2: Run tests, verify they fail** — `go test ./internal/process/ -run TestClassifyRun -count=1` → FAIL (undefined `ClassifyRun`).

- [ ] **Step 3: Implement**

Refactor `classifyRun` to `func (s *Service) classifyRun(runDir, name string, mark bool) (RecoveryEntry, error)`. The only behavioral fork is the final `probeAbsent` arm:

```go
	switch recoverGroupProbe(m.PGID) {
	case probeAbsent:
		if !mark {
			entry.Disposition = "abandonable"
			entry.Reason = "recorded group provably absent; an applied recovery would write the abandoned marker"
			return entry, nil
		}
		// existing writeAtomicJSON marker branch unchanged …
```

`Recover` calls `s.classifyRun(dir, name, true)`. Add the exported wrapper:

```go
// ClassifyRun classifies ONE run slot with the same predicate Recover applies
// per slot. mark=false is a pure assessment: it never writes; a slot Recover
// would mark reports "abandonable". mark=true applies the identical marker
// write. It is the single liveness/teardown predicate — callers must never
// reimplement it.
func (s *Service) ClassifyRun(runDir string, mark bool) (RecoveryEntry, error) {
	if !filepath.IsAbs(runDir) {
		return RecoveryEntry{}, failf(FailInvalidInput, "classify-run", "run dir must be an absolute path")
	}
	name := filepath.Base(runDir)
	if li, err := os.Lstat(runDir); err != nil || li.Mode()&os.ModeSymlink != 0 || !li.IsDir() || !runIDPattern.MatchString(name) {
		return RecoveryEntry{RunID: name, RunDir: runDir, Disposition: "foreign", Reason: "not a run slot; left untouched"}, nil
	}
	return s.classifyRun(runDir, name, mark)
}
```

- [ ] **Step 4: Run** `go test ./internal/process/ -count=1` → PASS (all existing recover tests included).
- [ ] **Step 5: Mutation-check** — temporarily make the `!mark` branch fall through to the marker write; `TestClassifyRunAssessDoesNotMark` must redden. Restore.
- [ ] **Step 6: Commit** — `git add internal/process && git commit -m "feat(0428): split process run classification into assess and apply modes"`

---

### Task 2: Schema-2 historical reader + frozen fixtures

**Files:**
- Create: `internal/gatedrive/history.go`, `internal/gatedrive/history_test.go`, `internal/gatedrive/testdata/legacy-v2/{passed,failed,halted,waiting}.json`, `internal/gatedrive/testdata/legacy-v2/{schema1,missing-worktree,corrupt}.json`

**Interfaces:**
- Consumes: `storedRecord`, `driveRecord`, `Store.readStored`, `Store.driveDir`, `storeErr`, kinds `ErrUnknownSchema`/`ErrCorruptRecord`/`ErrNotFound`.
- Produces:

```go
// historicalDrive is the bounded, assessment-only view of one persisted drive,
// readable across the historical schema range {2} plus the executable range
// {3,4}. It exposes exactly what classification needs — never command, env,
// credentials, or generations.
type historicalDrive struct {
	ID             string
	SchemaVersion  int
	RepoIdentity   string
	WorktreePath   string
	LastOutcome    Outcome
	RawRunDir      string
	PriorRawRunDir string
	RunRoot        string
}

// loadHistoricalDrive reads drive id for HISTORICAL ASSESSMENT ONLY.
func (s *Store) loadHistoricalDrive(id string) (historicalDrive, error)
```

- [ ] **Step 1: Derive the authentic schema-2 field set from history, then freeze fixtures**

```bash
git log --format='%H %ad %s' --date=short -S 'driveSchemaVersion = 2' -- internal/gatedrive/drive.go | tail -3
# pick the commit where driveSchemaVersion = 2 was CURRENT (the change-0359 commit), then:
git show <that-commit>:internal/gatedrive/drive.go | sed -n '/type driveRecord/,/^}/p'
```

Freeze `testdata/legacy-v2/passed.json` as a full `storedRecord` envelope with every v2 field populated the way that era's writer populated it (`schema_version: 2`, no `admission_token`, no `relaunch_reserved`/`relaunch_token`), e.g.:

```json
{
  "generation": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "record": {
    "schema_version": 2,
    "repo_identity": "/repo/main",
    "worktree_path": "/repo/.worktrees/old-feature",
    "change_id": "16", "task_id": "3", "phase": "build",
    "branch": "fix/old-feature", "ref": "refs/heads/fix/old-feature",
    "head_oid": "1111111111111111111111111111111111111111",
    "fingerprint": {},
    "command": ["/bin/sh", "-c", "true"], "cwd": "/repo/.worktrees/old-feature",
    "run_root": "/tmp/docket-gate-old/run-root",
    "idempotent_suite_gate": true,
    "config_provenance": "gate_observation_budget=default;build.test_command=repo",
    "budget": 1800000000000, "env_hash": "deadbeef",
    "started_at": "2026-08-01T10:00:00Z", "updated_at": "2026-08-01T10:30:00Z",
    "deadline": "2026-08-01T11:00:00Z", "last_clock": "2026-08-01T10:30:00Z",
    "protocol_version": 1,
    "raw_run_dir": "/tmp/docket-gate-old/run-root/00000000000000000000000000000001",
    "raw_ownership": "owner", "attempt": 1, "relaunch_count": 0,
    "terminal_receipt": "receipt",
    "last_outcome": "PASSED",
    "owner_generation": "",
    "scope_id": "s1", "gate_context_hash": "cafe"
  }
}
```

Adjust field names/values to match the historical struct exactly (the `git show` above is the oracle — the fixture must round-trip the historical writer's shape, not today's; inventory what the historical source actually had before finalizing fixtures, per learning `frozen-corpus-covers-what-it-contains`). Derivatives: `failed.json` (`last_outcome: "FAILED"`), `halted.json` (`"HALTED"`, `last_cause: "deadline-expired"`), `waiting.json` (`"WAITING"`), `schema1.json` (`"schema_version": 1`), `missing-worktree.json` (v2 PASSED whose `worktree_path` does not exist on any machine, e.g. `/nonexistent/removed-worktree`), `corrupt.json` (truncated JSON).

- [ ] **Step 2: Write the failing tests**

```go
package gatedrive

import (
	"os"
	"path/filepath"
	"testing"
)

// copyLegacyFixture installs testdata/legacy-v2/<name>.json as drive <id>'s
// record in store s (creating the 0700 dir), returning the id.
func copyLegacyFixture(t *testing.T, s *Store, name string) string {
	t.Helper()
	id := "0428aaaaaaaaaaaaaaaaaaaaaaaaaa" + map[string]string{
		"passed": "01", "failed": "02", "halted": "03", "waiting": "04",
		"schema1": "05", "missing-worktree": "06", "corrupt": "07"}[name]
	dir := filepath.Join(s.root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join("testdata", "legacy-v2", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, recordFileName), buf, 0o600); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestLoadHistoricalDriveReadsSchema2(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	h, err := s.loadHistoricalDrive(id)
	if err != nil {
		t.Fatalf("schema-2 must load for assessment: %v", err)
	}
	if h.SchemaVersion != 2 || h.LastOutcome != PASSED || h.WorktreePath == "" || h.RepoIdentity == "" {
		t.Fatalf("bad view: %+v", h)
	}
	// The EXECUTABLE reader still refuses it — no new compatibility granted.
	if _, err := s.Load(id); !isStoreKind(err, ErrUnknownSchema) {
		t.Fatalf("execution reader must still refuse v2, got %v", err)
	}
}

func TestLoadHistoricalDriveFailsClosed(t *testing.T) {
	s := OpenStore(t.TempDir())
	for name, kind := range map[string]StoreErrorKind{
		"schema1": ErrUnknownSchema,
		"corrupt": ErrCorruptRecord,
	} {
		id := copyLegacyFixture(t, s, name)
		if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, kind) {
			t.Fatalf("%s: want %s, got %v", name, kind, err)
		}
	}
}

// A v2 document with a required identity/outcome field missing must be refused
// as corrupt — never zero-value-decoded into a trustworthy record.
func TestLoadHistoricalDriveValidatesRequiredFields(t *testing.T) {
	s := OpenStore(t.TempDir())
	id := copyLegacyFixture(t, s, "passed")
	rewriteRecordField(t, s, id, `"worktree_path": "/repo/.worktrees/old-feature"`, `"worktree_path": ""`)
	if _, err := s.loadHistoricalDrive(id); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("empty worktree_path must fail closed, got %v", err)
	}
	id2 := copyLegacyFixture(t, s, "halted")
	rewriteRecordField(t, s, id2, `"last_outcome": "HALTED"`, `"last_outcome": "EXPLODED"`)
	if _, err := s.loadHistoricalDrive(id2); !isStoreKind(err, ErrCorruptRecord) {
		t.Fatalf("unknown outcome vocabulary must fail closed, got %v", err)
	}
}
```

`rewriteRecordField` is a 6-line helper: read `record.json`, `strings.Replace` (assert exactly one occurrence), write back.

- [ ] **Step 3: Run, verify FAIL** — `go test ./internal/gatedrive/ -run TestLoadHistoricalDrive -count=1` → undefined `loadHistoricalDrive`.

- [ ] **Step 4: Implement in `history.go`**

```go
// historicalSchemaV2 is the pre-0375 drive schema this repository's history
// can contain. It is readable for HISTORICAL ASSESSMENT ONLY — never loaded
// into the executable state machine, never migrated, never re-written.
const historicalSchemaV2 = 2

func (s *Store) loadHistoricalDrive(id string) (historicalDrive, error) {
	const op = "load-historical-drive"
	dir, err := s.driveDir(id)
	if err != nil {
		return historicalDrive{}, err
	}
	// Executable range first: v4/v3 load through the existing reader unchanged.
	stored, rerr := s.readStored(dir)
	if rerr == nil {
		return historicalView(id, stored.Record), nil
	}
	if !storeErrIs(rerr, ErrUnknownSchema) {
		return historicalDrive{}, rerr // corrupt / IO / not-found: unchanged
	}
	// Historical range: decode the raw envelope and validate schema 2
	// explicitly. Field presence is validated — a document is never trusted
	// because Go zero-filled it.
	buf, ferr := os.ReadFile(filepath.Join(dir, recordFileName))
	if ferr != nil {
		return historicalDrive{}, storeErr(ErrIO, op, ferr)
	}
	var raw struct {
		Record struct {
			SchemaVersion  int     `json:"schema_version"`
			RepoIdentity   string  `json:"repo_identity"`
			WorktreePath   string  `json:"worktree_path"`
			LastOutcome    Outcome `json:"last_outcome"`
			RawRunDir      string  `json:"raw_run_dir"`
			PriorRawRunDir string  `json:"prior_raw_run_dir"`
			RunRoot        string  `json:"run_root"`
			StartedAt      time.Time `json:"started_at"`
		} `json:"record"`
	}
	if err := json.Unmarshal(buf, &raw); err != nil {
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, err)
	}
	r := raw.Record
	if r.SchemaVersion != historicalSchemaV2 {
		return historicalDrive{}, storeErr(ErrUnknownSchema, op,
			fmt.Errorf("schema version %d is neither executable nor a supported historical schema", r.SchemaVersion))
	}
	switch r.LastOutcome {
	case "", WAITING, PASSED, FAILED, HALTED:
	default:
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, fmt.Errorf("unknown outcome vocabulary"))
	}
	if r.RepoIdentity == "" || r.WorktreePath == "" || r.StartedAt.IsZero() {
		return historicalDrive{}, storeErr(ErrCorruptRecord, op, fmt.Errorf("required identity fields missing"))
	}
	return historicalDrive{ID: id, SchemaVersion: r.SchemaVersion, RepoIdentity: r.RepoIdentity,
		WorktreePath: r.WorktreePath, LastOutcome: r.LastOutcome, RawRunDir: r.RawRunDir,
		PriorRawRunDir: r.PriorRawRunDir, RunRoot: r.RunRoot}, nil
}

func historicalView(id string, r driveRecord) historicalDrive {
	return historicalDrive{ID: id, SchemaVersion: r.SchemaVersion, RepoIdentity: r.RepoIdentity,
		WorktreePath: r.WorktreePath, LastOutcome: r.LastOutcome, RawRunDir: r.RawRunDir,
		PriorRawRunDir: r.PriorRawRunDir, RunRoot: r.RunRoot}
}
```

- [ ] **Step 5: Run** `go test ./internal/gatedrive/ -run TestLoadHistoricalDrive -count=1` → PASS.
- [ ] **Step 6: Mutation-check** — delete the `r.RepoIdentity == "" …` validation; `TestLoadHistoricalDriveValidatesRequiredFields` must redden. Restore.
- [ ] **Step 7: Commit** — `git add internal/gatedrive/history.go internal/gatedrive/history_test.go internal/gatedrive/testdata && git commit -m "feat(0428): historical schema-2 drive reader with frozen fixtures"`

---

### Task 3: The shared legacy classifier

**Files:**
- Modify: `internal/gatedrive/history.go`
- Test: `internal/gatedrive/history_test.go`

**Interfaces:**
- Consumes: `historicalDrive`, `Store.admissionKeyFor`, `isTerminalOutcome`, Task 1's `process.RecoveryEntry` dispositions.
- Produces (all in `history.go`):

```go
// recoverySeam is the single process-recovery predicate the classifier
// consults for HALTED history. *process.Service satisfies it (Task 1).
type recoverySeam interface {
	ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)
}

// Legacy classification classes.
const (
	LegacyNonblocking = "nonblocking"
	LegacyRecovered   = "recovered"
	LegacyRecoverable = "recoverable" // dry-run only: an applied run would recover it
	LegacyRetained    = "retained"
)

// LegacyFinding is one drive's assessment. Reason is a bounded token/phrase;
// DriveID is always a validated id (never an arbitrary directory name).
type LegacyFinding struct {
	DriveID string `json:"drive_id"`
	Class   string `json:"class"`
	Reason  string `json:"reason"`
}

// LegacyHistorySummary is the compact recovery summary carried on successful
// AND refused starts when legacy history was relevant.
type LegacyHistorySummary struct {
	Checked   int             `json:"checked"`
	Recovered []string        `json:"recovered,omitempty"`
	Retained  []LegacyFinding `json:"retained,omitempty"`
}

// classifyLegacyDrive assesses one historical record. requestedWorktree is the
// canonical root being admitted, or "" for a repository-wide (cleanup) pass.
// apply=true may write the existing process abandoned marker via the seam;
// apply=false previews (LegacyRecoverable instead of LegacyRecovered).
func (s *Store) classifyLegacyDrive(h historicalDrive, requestedWorktree string, proc recoverySeam, apply bool) LegacyFinding
```

- [ ] **Step 1: Write the failing tests** (table-driven; fixtures from Task 2; a fake `recoverySeam` recording calls and returning a scripted `process.RecoveryEntry`):

```go
type fakeRecovery struct {
	entries map[string]process.RecoveryEntry // key: runDir
	err     error
	marks   []string
}

func (f *fakeRecovery) ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error) {
	if mark {
		f.marks = append(f.marks, runDir)
	}
	if f.err != nil {
		return process.RecoveryEntry{}, f.err
	}
	e, ok := f.entries[runDir]
	if !ok {
		return process.RecoveryEntry{Disposition: "invalid", Reason: "no slot"}, nil
	}
	if mark && e.Disposition == "abandonable" {
		e.Disposition = "abandoned-marked"
	}
	return e, nil
}
```

Cases (assert `Class` and, where noted, that `f.marks` is empty / has one entry):

1. v2 PASSED, worktree path **nonexistent** (`missing-worktree` fixture), requestedWorktree = some other real dir → `nonblocking`, seam **never called** — terminal history is recognized BEFORE path resolution.
2. v2 FAILED, same worktree as requested → `nonblocking`.
3. v2 WAITING, worktree resolves to a *different* real dir, admission mode → `nonblocking` (irrelevant to this admission), reason mentions different worktree.
4. v2 WAITING, worktree path unresolvable (gone) → `retained` — an unresolvable path is NOT proof of unrelatedness.
5. v2 WAITING, same worktree → `retained` (live/nonterminal is never guessed dead).
6. v2 HALTED, seam says `terminal` for RawRunDir → `nonblocking` (durable teardown evidence), no mark.
7. v2 HALTED, seam says `already-abandoned` → `nonblocking`.
8. v2 HALTED, seam says `abandonable`, apply=true → `recovered`, exactly one mark.
9. v2 HALTED, seam says `abandonable`, apply=false → `recoverable`, zero marks.
10. v2 HALTED, seam says `needs-inspection` (unprovable probe / live group) → `retained`.
11. v2 HALTED, seam returns an **error** → `retained` (injected probe error retains — probe error is not clean absence).
12. v2 HALTED with empty `RawRunDir` → `retained` (missing evidence certifies nothing).
13. v2 HALTED with `PriorRawRunDir` set: BOTH run dirs must prove teardown; prior `needs-inspection` → `retained` even when current is `terminal`.
14. v2 HALTED whose worktree path is gone but seam proves `terminal` → `nonblocking` (assessed via run identities without requiring the worktree to exist).
15. nil seam with a HALTED record → `retained` (preserves the current nil-observe fail-closed stance).

- [ ] **Step 2: Run, verify FAIL** — undefined `classifyLegacyDrive`.

- [ ] **Step 3: Implement** — the order is the spec's numbered order and is load-bearing:

```go
func (s *Store) classifyLegacyDrive(h historicalDrive, requestedWorktree string, proc recoverySeam, apply bool) LegacyFinding {
	f := LegacyFinding{DriveID: h.ID}
	// 2. Trustworthy completed history is nonblocking BEFORE any path
	//    resolution: a supervisor-committed PASSED/FAILED outcome is durable
	//    evidence a removed worktree or temp dir cannot undo.
	if h.LastOutcome == PASSED || h.LastOutcome == FAILED {
		f.Class, f.Reason = LegacyNonblocking, "completed terminal outcome ("+string(h.LastOutcome)+")"
		return f
	}
	// 3. Not conclusively completed: establish the worktree binding. A valid
	//    binding to a DIFFERENT worktree is irrelevant to this admission; an
	//    unresolvable path is NOT proof of unrelatedness or teardown.
	if requestedWorktree != "" {
		if legacyRoot, _, err := s.admissionKeyFor(h.WorktreePath, "inventory-legacy-drive"); err == nil && legacyRoot != requestedWorktree {
			f.Class, f.Reason = LegacyNonblocking, "bound to a different worktree"
			return f
		}
	}
	// 4. Terminal HALTED: one exact-run recovery assessment through the
	//    existing process predicate — both recorded attempts must prove
	//    teardown. A probe error or missing evidence retains.
	if h.LastOutcome == HALTED {
		if proc == nil || h.RawRunDir == "" {
			f.Class, f.Reason = LegacyRetained, "halted with no assessable run evidence"
			return f
		}
		runs := []string{h.RawRunDir}
		if h.PriorRawRunDir != "" {
			runs = append(runs, h.PriorRawRunDir)
		}
		recovered := false
		for _, runDir := range runs {
			entry, err := proc.ClassifyRun(runDir, apply)
			if err != nil {
				f.Class, f.Reason = LegacyRetained, "process assessment failed; evidence unprovable"
				return f
			}
			switch entry.Disposition {
			case "terminal", "stopped", "already-abandoned":
				// durable teardown evidence already present
			case "abandoned-marked":
				recovered = true
			case "abandonable": // apply=false preview
				recovered = true
			default: // live, needs-inspection, invalid, foreign, unresolved-establishment
				f.Class, f.Reason = LegacyRetained, "halted run not provably torn down ("+entry.Disposition+")"
				return f
			}
		}
		if recovered {
			if apply {
				f.Class, f.Reason = LegacyRecovered, "abandoned marker recorded from provable group absence"
			} else {
				f.Class, f.Reason = LegacyRecoverable, "provable group absence; an applied run would record the marker"
			}
		} else {
			f.Class, f.Reason = LegacyNonblocking, "halted with durable teardown evidence"
		}
		return f
	}
	// 5. Live/nonterminal (WAITING or empty outcome): never guessed dead.
	f.Class, f.Reason = LegacyRetained, "nonterminal execution state"
	return f
}
```

- [ ] **Step 4: Run** the table → PASS. Then mutation-check the two guards named by the acceptance criteria: (a) swap steps 2 and 3 (resolve path first) — case 1 must redden; (b) make the seam-error branch fall through to `recovered` — case 11 must redden. Restore.
- [ ] **Step 5: Commit** — `git commit -am "feat(0428): shared legacy-drive classifier (terminal-before-path, halted via process predicate)"`

---

### Task 4: Rewire the admission inventory through the classifier

**Files:**
- Modify: `internal/gatedrive/admission.go`, `internal/gatedrive/ownership.go`, `internal/gatedrive/driver.go`, `internal/app/gate.go`
- Test: `internal/gatedrive/admission_test.go` (extend), `internal/gatedrive/history_test.go`

**Interfaces:**
- Consumes: Tasks 1–3.
- Produces:
  - `OwnershipError` gains `Legacy *LegacyHistorySummary` (nil except on inventory refusals). `Kind`/`Op` unchanged so every existing consumer compiles and behaves identically.
  - `func (s *Store) inventoryLegacyDrives(worktreeRoot string, proc recoverySeam) (*LegacyHistorySummary, error)` — deterministic id order; assesses ALL records, then refuses ONCE (first blocker's locator in `Op`, all blockers in the summary).
  - `func (s *Store) reserveWorktreeExecution(rec admissionRecord, proc recoverySeam) (token string, legacy *LegacyHistorySummary, err error)`; same for `ReserveWorktreeExecution` (nil seam) and `ReserveRawWorktreeExecution(repoIdentity, worktreeRoot string, proc recoverySeam)`.
  - `ProcessSeam` gains `ClassifyRun(runDir string, mark bool) (*process.RecoveryEntry, error)`? **No** — keep the value-return shape from Task 1: `ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)`. Update every fake seam in the gatedrive/app tests to add a stub.
  - `admissionObservationProvesTeardown` is DELETED (its only caller is replaced; a bare Observe `vanished` no longer certifies anything).

- [ ] **Step 1: Write the failing tests first** (in `history_test.go`, driving through `store.reserveWorktreeExecution` exactly as `admission_test.go` does today — reuse its record/worktree helpers):

1. **The reported refusal, then the fix (RED against current code):** store containing the v2 `passed.json` fixture bound to a *removed* worktree; reserve a *different, unrelated* worktree → today `ErrUnresolvedExecution`; after the fix: token minted, `legacy.Checked == 1`, `len(legacy.Retained) == 0`.
2. Multiple completed legacy drives (v2 passed + v2 failed + v2 halted-with-terminal-evidence) → admission succeeds, `Checked == 3`.
3. v1 `schema1.json` record present → refusal, `err` is `*OwnershipError` with `Kind == ErrUnresolvedExecution`, `Op == "inventory-legacy-drive-<that id>"`, and `err.Legacy.Retained` names it.
4. `corrupt.json` → same refusal shape.
5. A non-directory entry / an entry whose name fails `validateID` under `s.root` → refusal with `Op == "inventory-legacy-drives"` (safe inventory-level locator; the raw name appears NOWHERE in the error).
6. Record-less drive directory → still skipped (existing behavior preserved; existing test keeps passing).
7. v2 HALTED same-worktree with fake seam scripting `abandonable` → admission succeeds, `legacy.Recovered == []string{id}`, seam saw `mark=true` exactly once per run dir.
8. v2 HALTED with seam scripting `needs-inspection` → refusal, locator names the id; and with a scripted seam **error** → refusal (probe error retains).
9. nil seam (exported `ReserveWorktreeExecution`) + HALTED record → refusal (unchanged fail-closed stance).
10. Second reservation of the same worktree after a release does NOT re-run the inventory (the census is first-admission-only, keyed on `ErrNotFound` of the slot record — assert the seam is not called again).

- [ ] **Step 2: Run — verify case 1 and 3 shapes fail** against current code for the *right* reason (case 1 currently refuses; the summary fields don't exist yet → compile failure first, then behavioral red).

- [ ] **Step 3: Implement**

`ownership.go`:

```go
type OwnershipError struct {
	Kind OwnershipErrorKind
	Op   string
	// Legacy is the first-admission legacy-history summary, populated ONLY on
	// an inventory refusal so the caller can surface which historical drives
	// were checked, recovered, and retained. Bounded ids and reasons only.
	Legacy *LegacyHistorySummary
}
```

`admission.go` — replace the body of `inventoryLegacyDrives`:

```go
func (s *Store) inventoryLegacyDrives(worktreeRoot string, proc recoverySeam) (*LegacyHistorySummary, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, ownershipErr(ErrUnresolvedExecution, "inventory-legacy-drives")
	}
	sum := &LegacyHistorySummary{}
	firstLocator := ""
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		id := entry.Name()
		if !entry.IsDir() || validateID(id) != nil {
			// Arbitrary names never enter a diagnostic: safe inventory-level locator.
			sum.Retained = append(sum.Retained, LegacyFinding{DriveID: "", Class: LegacyRetained, Reason: "unrecognized entry in the drive registry"})
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drives"
			}
			continue
		}
		h, lerr := s.loadHistoricalDrive(id)
		if lerr != nil {
			if storeErrIs(lerr, ErrNotFound) {
				continue // record-less creation window — unchanged
			}
			sum.Checked++
			reason := "unreadable record"
			if storeErrIs(lerr, ErrUnknownSchema) {
				reason = "unknown schema"
			}
			sum.Retained = append(sum.Retained, LegacyFinding{DriveID: id, Class: LegacyRetained, Reason: reason})
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drive-" + id
			}
			continue
		}
		sum.Checked++
		f := s.classifyLegacyDrive(h, worktreeRoot, proc, true)
		switch f.Class {
		case LegacyRecovered:
			sum.Recovered = append(sum.Recovered, id)
		case LegacyRetained:
			sum.Retained = append(sum.Retained, f)
			if firstLocator == "" {
				firstLocator = "inventory-legacy-drive-" + id
			}
		}
	}
	if firstLocator != "" {
		oe := ownershipErr(ErrUnresolvedExecution, firstLocator)
		oe.Legacy = sum
		return sum, oe
	}
	if sum.Checked == 0 {
		return nil, nil // no relevant legacy history: no summary narration
	}
	return sum, nil
}
```

Thread the summary through `reserveWorktreeExecution` (store it nowhere durable; return it), update `ReserveWorktreeExecution` (nil seam), `ReserveRawWorktreeExecution` (seam param), `Driver.reserveWorktreeExecution` (pass `d.proc`, return the summary), and `internal/app/gate.go`'s raw reserve call site (`store.ReserveRawWorktreeExecution(repoIdentity, worktreeRoot, svc)` — `*process.Service` satisfies `recoverySeam` after Task 1). Add `ClassifyRun` to `ProcessSeam` (mirroring `process.Service`) and to every fake seam in `internal/gatedrive` tests (a two-line stub returning `{Disposition: "invalid"}` unless the test scripts it). Delete `admissionObservationProvesTeardown` and its test.

- [ ] **Step 4: Run** `go test ./internal/gatedrive/ ./internal/app/ -count=1` → PASS (including all pre-existing admission, driver, takeover, integration tests — `git grep -n "inventoryLegacyDrives\|reserveWorktreeExecution" internal/` first and fix every call site the compiler names).
- [ ] **Step 5: Mutation-check** — restore the old "resolve path before terminal check" ordering inside the classifier call path; case 1 must redden. Restore.
- [ ] **Step 6: Commit** — `git commit -am "fix(0428): admission inventory assesses legacy history through the shared classifier"`

---### Task 5: Carry the recovery summary on successful starts

**Files:**
- Modify: `internal/gatedrive/drive.go` (DriveDoc), `internal/gatedrive/driver.go` (Admit/Start/StartAdmitted paths + `AdmissionTicket`)
- Test: `internal/gatedrive/driver_test.go` or `internal/gatedrive/history_test.go`

**Interfaces:**
- Produces: `DriveDoc.LegacyHistory *LegacyHistorySummary \`json:"legacy_history,omitempty"\`` — populated on the START document only, only when `Checked > 0`. Adding an omitempty field does not bump `ProtocolVersion` (that bumps only on remove/rename/retype — see the `ProtocolVersion` doc comment). `AdmissionTicket` gains an unexported `legacy *LegacyHistorySummary` field the scoped/scopeless start paths thread from the reserve to the returned doc.

- [ ] **Step 1: Failing test** — scoped-or-scopeless driver Start over a store carrying the v2 `passed.json` fixture (different worktree): the returned `DriveDoc.LegacyHistory` has `Checked == 1`; a start with NO legacy records returns `LegacyHistory == nil` (no narration on normal starts). Reuse the existing driver-start test harness/fake seam.
- [ ] **Step 2: Run — FAIL** (field undefined).
- [ ] **Step 3: Implement** — thread the summary returned by `d.reserveWorktreeExecution` into the ticket, and set `doc.LegacyHistory = ticket.legacy` (respectively the local summary on the non-ticket path) just before returning the start document. Exactly one process launches either way — no change to launch logic.
- [ ] **Step 4: Run** `go test ./internal/gatedrive/ -count=1` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(0428): start documents carry the legacy history recovery summary"`

---

### Task 6: Propagate stage, locator, and summary through the application refusal

**Files:**
- Modify: `internal/app/gate_drive.go`
- Test: `internal/app/gate_drive_test.go` (extend)

**Interfaces:**
- Produces on `GateDriveResult`:

```go
	// Stage + Locator carry the typed refusal site for an inventory refusal:
	// Stage "legacy-inventory", Locator "inventory-legacy-drive-<id>" (validated
	// id) or the safe "inventory-legacy-drives". Empty for every other refusal.
	Stage   string `json:"stage,omitempty"`
	Locator string `json:"locator,omitempty"`
	// LegacyHistory mirrors the drive document's summary onto refusals, where
	// no drive document exists.
	LegacyHistory *gatedrive.LegacyHistorySummary `json:"legacy_history,omitempty"`
```

- [ ] **Step 1: Failing tests** (fake engine returning a crafted `*OwnershipError`):

1. `ErrUnresolvedExecution` with `Op == "inventory-legacy-drive-" + validID` and a summary → result has `Reason == "unresolved-execution"` (token unchanged), `Stage == "legacy-inventory"`, `Locator == "inventory-legacy-drive-"+validID`, `LegacyHistory` non-nil, and `Message` names `docket gate history cleanup` and does NOT contain the words "this worktree".
2. Same kind with `Op == "inventory-legacy-drives"` → `Locator == "inventory-legacy-drives"`.
3. Same kind with a NON-inventory op (e.g. `"reserve-worktree-execution"`) → `Stage`/`Locator`/`LegacyHistory` all empty, `Message` is the existing slot-recovery text (regression pin for the non-inventory path).
4. `Op == "inventory-legacy-drive-../evil"` (id fails validation) → `Locator == "inventory-legacy-drives"` — an arbitrary name never renders.
5. `HumanText` on case 1 contains `stage: legacy-inventory`, the locator line, and a compact `legacy_history: checked 1 recovered 0 retained 1` line; on a SUCCESS result whose `Drive.LegacyHistory` is set, the same compact line renders.

- [ ] **Step 2: Run — FAIL.**

- [ ] **Step 3: Implement** in `mapDriveResult`'s ownership-error branch:

```go
		if oe, ok := gatedrive.AsOwnershipError(err); ok {
			if stage, locator, ok2 := legacyInventoryLocator(oe.Op); ok2 {
				result.Stage, result.Locator = stage, locator
				result.LegacyHistory = oe.Legacy
				result.Message = "historical gate drives block this admission; inspect or recover them with docket gate history cleanup (--dry-run first); run.cancel applies only to a live run with an owning epoch"
			} else {
				result.Message = ownershipNextAction(oe.Kind)
			}
		}
```

with:

```go
// legacyInventoryLocator recognizes the inventory refusal ops and returns a
// SAFE locator: a drive-id-bearing op is rendered only when the id validates;
// anything else collapses to the inventory-level locator.
func legacyInventoryLocator(op string) (stage, locator string, ok bool) {
	const prefix = "inventory-legacy-drive-"
	switch {
	case op == "inventory-legacy-drives":
		return "legacy-inventory", op, true
	case strings.HasPrefix(op, prefix):
		if id := strings.TrimPrefix(op, prefix); gatedrive.ValidDriveID(id) {
			return "legacy-inventory", op, true
		}
		return "legacy-inventory", "inventory-legacy-drives", true
	}
	return "", "", false
}
```

Export a tiny `func ValidDriveID(id string) bool { return validateID(id) == nil }` from `internal/gatedrive` (one line, beside `validateID`). Extend `HumanText` with the three lines from test 5 (`stage:`, `locator:`, `legacy_history: checked N recovered N retained N` — counts only, never per-record content in prose).

- [ ] **Step 4: Run** `go test ./internal/app/ -run GateDrive -count=1` → PASS; then mutation-check test 4 by relaxing the id validation to a prefix/no-op — it must redden (learning: `identity-match-relaxed-to-prefix-is-vacuous`). Restore.
- [ ] **Step 5: Commit** — `git commit -am "fix(0428): gate-drive refusals carry the typed inventory stage and safe locator"`

---

### Task 7: `Driver.CleanupHistory` — the shared manual assessment

**Files:**
- Modify: `internal/gatedrive/history.go`, `internal/gatedrive/driver.go`
- Test: `internal/gatedrive/history_test.go`

**Interfaces:**
- Produces:

```go
// HistoryCleanupRequest selects the manual assessment's scope. DriveID "" scans
// the whole registry in ascending id order; a non-empty DriveID must validate.
type HistoryCleanupRequest struct {
	DriveID string
	DryRun  bool
}

// HistoryCleanupOutcome reports every candidate with its class and reason. A
// mixed outcome never claims complete recovery: Retained > 0 means blockers
// remain visible.
type HistoryCleanupOutcome struct {
	Findings []LegacyFinding
	Checked, Recovered, Recoverable, Retained, Nonblocking int
}

func (d *Driver) CleanupHistory(req HistoryCleanupRequest) (HistoryCleanupOutcome, error)
```

- [ ] **Step 1: Failing tests** (store seeded from the Task 2 fixtures; fake seam):

1. Repo-wide scan over {passed, failed, halted(abandonable), waiting, schema1} → findings sorted ascending by id; classes: nonblocking, nonblocking, recovered, retained("nonterminal execution state"), retained("unknown schema"); counts match; **worktree resolution never required** (fixtures' worktrees don't exist; classifier runs with `requestedWorktree == ""`).
2. `DryRun: true` over the same store → halted drive reports `recoverable`, seam saw `mark=false` only, zero marker writes.
3. Applied run twice: second run's halted finding is `nonblocking` ("halted with durable teardown evidence" — the seam now reports `already-abandoned`), zero additional marks — idempotent.
4. `DriveID` set → exactly that one finding; `DriveID: "../evil"` → typed `ErrInvalidID` error, nothing scanned.
5. Empty/absent registry root → zero findings, no error.
6. Record-less directory → skipped, not counted.

- [ ] **Step 2: Run — FAIL.**

- [ ] **Step 3: Implement** — enumeration mirrors `inventoryLegacyDrives` (sorted `os.ReadDir(s.root)`, `ErrNotExist` → empty, record-less skip, invalid names reported as a `retained` finding with empty DriveID and the bounded reason "unrecognized entry in the drive registry"), but with `requestedWorktree=""` and `apply=!req.DryRun`, and it NEVER refuses: every candidate lands in `Findings`. `Driver.CleanupHistory` delegates to a `Store` method with `d.proc` as the seam. It takes no admission/scope/drive lock (it mutates no gate state; the only write is the process layer's own lock-guarded marker).

- [ ] **Step 4: Run** `go test ./internal/gatedrive/ -count=1` → PASS.
- [ ] **Step 5: Commit** — `git commit -am "feat(0428): Driver.CleanupHistory shares the legacy classifier for manual recovery"`

---

### Task 8: The `gate.history.cleanup` application operation

**Files:**
- Create: `internal/app/gate_history.go`, `internal/app/gate_history_test.go`
- Modify: `internal/app/schema_registry.go`

**Interfaces:**
- Produces:

```go
const OperationGateHistoryCleanup = "gate.history.cleanup"

type GateHistoryCleanupRequest struct {
	RepoDir string `json:"repo_dir"`
	DriveID string `json:"drive_id,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty"`
}

type HistoryCleanupFinding struct {
	DriveID string `json:"drive_id,omitempty"`
	Class   string `json:"class"`  // nonblocking | recovered | recoverable | retained
	Reason  string `json:"reason"` // bounded phrase, never record content
}

type GateHistoryCleanupResult struct {
	Envelope
	DryRun      bool                    `json:"dry_run"`
	Checked     int                     `json:"checked"`
	Recovered   int                     `json:"recovered"`
	Recoverable int                     `json:"recoverable"`
	Retained    int                     `json:"retained"`
	Findings    []HistoryCleanupFinding `json:"findings"`
	Reason      string                  `json:"reason,omitempty"`
}

// GateHistoryCleanup runs the shared legacy assessment without starting any
// execution. gitCommonDir/exePath resolve exactly as the commandless drive
// service resolves them; no metadata preparation, remote fetch, or worktree
// existence is required.
func GateHistoryCleanup(gitCommonDir, exePath string, req GateHistoryCleanupRequest) GateHistoryCleanupResult
```

- [ ] **Step 1: Failing tests** — compose over a real temp gitCommonDir seeded with Task 2 fixtures (the process seam here is the real `process.NewService(exePath)`; halted fixtures' run dirs don't exist so they classify retained — script the richer dispositions at the gatedrive layer, already covered by Task 7; here assert the MAPPING):
1. Applied run: envelope op `gate.history.cleanup`, `ResultApplied`, counts and findings mirror the outcome, `HumanText` header like `history cleanup — checked 5: 2 nonblocking, 0 recovered, 0 recoverable, 3 retained` plus one `  <id-or-(registry)>  <class>  <reason>` line per finding, and when `Retained > 0` a trailing `retained records remain; recovery is not complete` line (never a repository-readiness claim).
2. Invalid `--drive-id` → `ResultInvalidInput`, `Reason: "invalid-id"`.
3. `DryRun` echoes into the document.
4. Redaction bound: marshal the result of a run over every fixture and assert the JSON contains no `"command"`, no `"env"`, no fixture `head_oid`, no generation token — findings carry ONLY id/class/reason.
- [ ] **Step 2: Run — FAIL.**
- [ ] **Step 3: Implement** — build `gatedrive.OpenStore(gitCommonDir)` + `process.NewService(exePath)` + `gatedrive.NewSystemDriver`, call `CleanupHistory`, map `LegacyFinding` → `HistoryCleanupFinding` verbatim, map the `ErrInvalidID` store error to `ResultInvalidInput`/`invalid-id` via the existing `mapDriveFailure`. Register in `schema_registry.go`:

```go
	{ID: "gate.history.cleanup", Request: GateHistoryCleanupRequest{}, Result: GateHistoryCleanupResult{}}, // GateHistoryCleanup
```

- [ ] **Step 4: Run** `go test ./internal/app/ -count=1` → PASS (the registry/fidelity/vocab guard tests will name anything the new documents violate — fix what they name, never relax them).
- [ ] **Step 5: Commit** — `git commit -am "feat(0428): gate.history.cleanup operation with bounded result document"`

---

### Task 9: CLI leaf `docket gate history cleanup` + capability registration

**Files:**
- Modify: `internal/cli/gate.go`, `internal/cli/install.go`
- Test: `internal/cli/gate_test.go`, plus whatever `internal/cli/capability_production_test.go` requires

**Interfaces:**
- Produces the cataloged command `docket gate history cleanup --repo-dir <r> [--drive-id <id>] [--dry-run] [--json]`, capability id `gate.history.cleanup`.

- [ ] **Step 1: Failing tests** — mirror the existing gate-command CLI tests: (a) the capability walker output includes id `gate.history.cleanup` with the `gate history cleanup` argv; (b) running `--json gate history cleanup --repo-dir <fixture>` over a temp repo emits the operation envelope; (c) `--drive-id` shape refusal surfaces `invalid-id`; (d) missing `--repo-dir` errors.
- [ ] **Step 2: Run — FAIL.**
- [ ] **Step 3: Implement** in `newGateCommand`: a `history` group command (`Use: "history"`, short: "historical gate-drive assessment and recovery") plus the leaf:

```go
	cleanupHist := &cobra.Command{
		Use:         "cleanup --repo-dir <dir> [--drive-id <id>] [--dry-run]",
		Short:       "assess pre-admission gate-drive history; recover safely-recoverable records, retaining all evidence",
		Annotations: capability("gate.history.cleanup", EffectProcessControl, EffectLocalWrite),
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { … resolve repo-dir → common dir + exe exactly as gateDriveRepoContext does; call app.GateHistoryCleanup; setResult … },
	}
```

(Reuse `gateDriveRepoContext` for common-dir/exe resolution. If the capability-vocabulary test rejects `EffectProcessControl` for a command that only probes and marks, follow the test's own vocabulary — the effects it accepts are the oracle, not this plan.) Register `histCmd.AddCommand(cleanupHist)` and `gateCmd.AddCommand(…, histCmd, …)`. In `install.go` add both entries beside the existing gate rows:

```go
	"gate history":         true, // the group itself; it reports a missing command
	"gate history cleanup": true,
```

- [ ] **Step 4: Run** `go test ./internal/cli/ -count=1` → PASS (the production capability test enumerates every public leaf — it is the guard that forces complete registration).
- [ ] **Step 5: Commit** — `git commit -am "feat(0428): docket gate history cleanup CLI leaf and capability registration"`

---

### Task 10: End-to-end acceptance, guidance, ADR, and the suite gate

**Files:**
- Test: `internal/gatedrive/integration_test.go` or a new `internal/gatedrive/history_integration_test.go`; `internal/app/gate_drive_test.go`
- Modify: `internal/gatedrive/drive.go` (doc comment), any maintained guidance the greps below name
- Create: one ADR via the docket-adr skill

- [ ] **Step 1: End-to-end acceptance tests** (the spec's criteria 1, 5, 7 — write red where possible first):
1. **Criterion 1:** real driver + real process seam + temp repo: seed the v2 PASSED fixture (removed worktree), then one ordinary scoped `Start` through `GateDriveService` (build owner, fake-fast suite command `/bin/echo`): it reaches launch with NO prior cleanup call and NO second start; assert exactly one run dir exists under the run root and `SuiteBudgetUsage` reports exactly 1 attempt used.
2. **Criterion 5 (concurrency):** two concurrent `Start`s for the same worktree over a legacy-seeded store: exactly one wins (`ErrWorktreeBusy`/`ErrUnresolvedExecution` for the loser, never two launches); a concurrent `CleanupHistory` during a start neither deadlocks nor deletes anything (assert every fixture record file still byte-identical afterward).
3. **Criterion 7 negative space:** a FAILED suite verdict and a post-launch HALTED doc trigger NO cleanup and NO second start (assert the seam's mark count and the drive count); an admission refusal from a busy slot (non-inventory) carries no stage/locator.
- [ ] **Step 2: Upgrade boundary + guidance.** Extend the `driveSchemaVersion` doc comment in `drive.go`: schema 2 is readable by the HISTORICAL reader (`loadHistoricalDrive`) for admission assessment and `gate history cleanup` only — never loaded for execution, never migrated, never re-written; no claim is made that an old binary launched concurrently honors new admission rules. Then derive the guidance edits from greps, never a hand list:

```bash
git grep -n "a prior execution in this worktree is unresolved" -- ':!docs/changes' ':!docs/results' ':!docs/superpowers'
git grep -ln "unresolved-execution" -- 'docs' 'skills' ':!docs/changes' ':!docs/results' ':!docs/adrs' ':!docs/superpowers'
```

Update every MAINTAINED hit (source messages were already changed in Task 6; skills/docs guidance should now name `docket gate history cleanup` as the inspection/recovery route for retained historical records, and prescribe `run.cancel` only for a live run with an owning epoch). Point-in-time records (archived changes, results, specs, accepted ADRs) stay untouched.
- [ ] **Step 3: ADR.** Invoke the docket-adr skill to record: "Historical gate-drive schemas are assessed, never executed" — context (0375's admission inventory meeting pre-0375 schema-2 history), decision (a separate historical reader + classifier with terminal-before-path ordering and the single process recovery predicate; the executable reader's schema policy unchanged), consequences (each future schema bump must decide whether the retired version joins the historical range; no retirement lifecycle). Add the new ADR id to the change's `adrs:` list via the metadata flow the skill owns.
- [ ] **Step 4: Full suite gate.** From the source checkout run the configured build gate (`go run ./cmd/docket development test`). Read the budget report; a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding and a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on. If any repo-wide guard trips on `testdata/legacy-v2/` fixtures, exclude that exact directory boundedly and mutation-test the exclusion (plant a violation just outside it; the scan must still redden).
- [ ] **Step 5: Mutation sweep of the new guards** (each: mutate with a backup copy, `go test -count=1`, confirm red, restore): terminal-before-path ordering (Task 3), probe-error-retains (Task 3), locator id validation (Task 6), v2 required-field validation (Task 2), assess-mode-never-marks (Task 1).
- [ ] **Step 6: Commit** — `git add -A && git commit -m "test(0428): end-to-end legacy-history acceptance, guidance, and upgrade-boundary docs"`

---

## Self-Review (performed)

- **Spec coverage:** Goal/§Automatic recovery → Tasks 4–6, 10.1; §One shared history check steps 1–5 → Tasks 2–4 (step order pinned by mutation checks); §Small manual recovery command → Tasks 7–9 (draft options `--older-than`/`--prune-logs` deliberately absent); §Locking and diagnostics → Tasks 4, 6, 7 (no new locks, safe locators, no "this worktree" without ownership); §Acceptance 1→10.1, 2→3.1/4.1–4.2, 3→4.3–4.5/2, 4→1/3.6–3.15/4.7–4.8, 5→10.1–10.2 (all three callers funnel through the one `store.reserveWorktreeExecution` chokepoint — verified in Task 4 step 3), 6→7.1–7.3, 7→5/6/10.3, 8→10.2 byte-identical assert + no deletion anywhere, 9→10.4–10.5.
- **Placeholder scan:** none of the banned patterns; every code step carries concrete code or an exact derivation command.
- **Type consistency:** `recoverySeam.ClassifyRun(runDir string, mark bool) (process.RecoveryEntry, error)` matches Task 1's export; `LegacyHistorySummary{Checked, Recovered []string, Retained []LegacyFinding}` used identically in Tasks 3–8; `historicalDrive` fields used in Tasks 2–3 match; `OwnershipError.Legacy` set in Task 4, read in Task 6.
- **Known judgment calls recorded for the builder:** raw `gate launch` shares the admission behavior (Task 4) but its result document does not grow a summary field — the spec requires the summary on `gate.drive.start` success/refusal only; the classifier treats `abandonable` under `apply=true` as `recovered` only after the seam's mark actually reported `abandoned-marked`.
