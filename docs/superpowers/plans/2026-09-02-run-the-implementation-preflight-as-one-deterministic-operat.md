<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0397 — Run the implementation preflight as one deterministic operation instead of a docket-status dispatch, and drop status --json's corpus records by default](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0397-run-the-implementation-preflight-as-one-deterministic-operat.md)**
<!-- docket:backlink:end -->
# Implementation Preflight as One Deterministic Operation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In a docket run, `docket-build` executes this plan task-by-task under the docket-build-task contract.

**Goal:** Replace `docket-implement-next`'s ~2-minute / ~85k-token Step-0 `docket-status` dispatch with one deterministic `maintenance.preflight` operation, and make `docket status --json` omit the 130 KB `records` array unless `--records` asks for it.

**Architecture:** A thin composition in `internal/app` sequences the existing `MaintenanceSweep(SweepScopeImplementation)` mutation with a fresh compact `Status` read and returns one protocol-v1 envelope carrying both halves, a Go-computed `preflight: clean|problem` verdict, the `problem_entries` subset, and the post-sweep metadata revision. No new sweep or status logic — the composition only sequences and projects. The CLI grows `docket maintenance preflight` (catalog id `maintenance.preflight`, effects `metadata-write`) and a `--records` opt-in on `status`; `docket-implement-next` Step 0 becomes a single inline Bash call; skill/convention prose and the prose-contract guards follow.

**Tech Stack:** Go (cobra CLI in `internal/cli`, operations in `internal/app`, guards in `internal/repoguard`); markdown skill prose in `skills/`; suite via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-02-run-the-implementation-preflight-as-one-deterministic-operat-design.md` (metadata branch `docket`). Change file: `docs/changes/active/0397-run-the-implementation-preflight-as-one-deterministic-operat.md`.

## Global Constraints

- Protocol v1: removing/renaming/retyping a JSON field needs a new protocol version; **adding** operation-specific fields is compatible (`internal/app/result.go`, `ProtocolVersion`). Dropping `records` by default is a behavior flip on an existing field — gated by the consumer audit in Task 1.
- The spec's `result: "applied | no-op | refused | error"` is shorthand: the envelope always carries a real v1 `Result` spelling (`applied`, `no-op`, `invalid-state`, `external-failed`, …). The preflight envelope mirrors the failing half's actual result — never a new spelling.
- Every guard added or edited is mutation-tested: strip the guarded thing, watch the assert redden, restore (AGENTS.md "Guards and tests"). Defeat Go's test cache on every mutation probe: `go test -count=1` (learning: cached-runner-serves-a-mutated-tree).
- Never hand-list sites of a literal being retired — derive from a whole-repo grep, sort prose vs executable (AGENTS.md).
- The build gate runs the whole suite via the command `build.test_command` resolves to (today `go run ./cmd/docket development test`), once, at the end — docket-build owns that gate; tasks run only their focused tests.
- Prose-contract phrases are matched with `strings.Contains` on raw file bytes (`internal/repoguard/prose_contracts_test.go`): keep every new guard phrase short enough that a re-flow cannot wrap it (learning: phrase-grep-over-wrapped-prose).
- Point-in-time records (archived changes, frozen specs/plans, Accepted ADRs) keep their old wording; only maintained source is edited (AGENTS.md "Comments and cross-references").
- This is a perf change: correctness asserts cannot be its oracle — the measured numbers in Task 7 are the acceptance evidence (learning: optimization-needs-a-measured-oracle).

## Reconcile-time facts the tasks rely on

Verified in this worktree at `78d42319` (branch `perf/run-the-implementation-preflight-as-one-deterministic-operat`):

- `app.Status` (`internal/app/status.go`) computes `corpusRecords` unconditionally at step 7 and has exactly **one** production caller: `internal/cli/root.go` (`statusCmd.RunE`). No other Go production code calls it.
- Repo-wide grep for `records` consumers: the only executable readers of the JSON field are tests — `internal/app/status_result_test.go` (asserts `"records":[]` on failure paths) and `internal/cli/root_test.go` (asserts `"records":` present). Frozen docs (`docs/superpowers/plans/2026-08-16-read-only-status-and-health-vertical-slice.md`, the 2026-08-15 status spec) are point-in-time records — untouched. No skill, agent, or script reads the field.
- Sweep disposition tokens live in `internal/app/maintenance.go`: `SweepDispApplied|NoOp|Contended|Blocked|Unknown|Failed|Skipped` ("applied", "noop", "contended", "blocked", "unknown", "failed", "skipped").
- `MaintenanceResult` embeds `Envelope` and carries `Entries`, `Reason`, `Message`, `Findings`, `Scope`, `DeferredHistoricalCleanups`. `StatusResult` embeds `Envelope` and carries `Context` (with `MetadataRevision`), `Summary`, `Changes`, `Ready`, `Records`, `Findings`.
- The capability catalog is derived by walking cobra annotations (`capability(id, effects...)` in `internal/cli/capability.go`); `internal/cli/capability_production_test.go` enforces forward/reverse population match, so a new annotated leaf is counted automatically.
- `scripts/docket-status.md` **does not exist** in the integration tree (the bash-suite retirement removed it). The spec's instruction to update it resolves to: nothing to update; the stale pointers to it inside `skills/docket-status/SKILL.md` predate this change and stay out of scope except where a sentence being rewritten anyway contains one.
- The prose-contract guard table (`internal/repoguard/prose_contracts_test.go`) has three `change_0389_sweep_scope` rows whose required phrases live in exactly the prose Tasks 4–5 delete — those rows must move in the same task as the prose edit or the suite reds (learning: restatement-accumulates-its-own-guards).

---

### Task 1: `status --json` drops `records` behind `--records` (`IncludeRecords` option)

**Files:**
- Modify: `internal/app/status.go` (StatusOptions, `Status` step 7)
- Modify: `internal/app/status_result.go` (`Records` field tag + `NewStatusResult`)
- Modify: `internal/app/status_result_test.go` (failure-path array asserts)
- Modify: `internal/cli/root.go` (`statusCmd` flag + wiring)
- Modify: `internal/cli/root_test.go` (the `"records":` presence assert)
- Test: `internal/app/status_test.go` (or the existing status test file colocated with `status.go`'s tests), `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `app.Status(ctx, reader, StatusOptions)`, `StatusOptions{RepoDir, Types, Priorities}`, `StatusResult.Records []StatusRecord`.
- Produces: `StatusOptions.IncludeRecords bool`; `StatusResult.Records *[]StatusRecord` with `json:"records,omitempty"` — `nil` (field absent) unless requested, non-nil (marshals `[]` or the full array) when requested. Task 2's composition calls `Status` with `IncludeRecords: false`.

Why a pointer: `encoding/json` `omitempty` omits *any* zero-length slice, so a plain slice cannot distinguish "not requested" from "requested, empty corpus". A `*[]StatusRecord` that is nil is omitted; a pointer to an empty slice marshals `[]`.

- [ ] **Step 1: Write the failing app-layer tests**

In the app package's status tests (same package/fixtures as the existing `Status` tests — reuse their fake `StatusReader`):

```go
func TestStatusRecordsOptIn(t *testing.T) {
	// reader: the existing fake with at least one change record.
	// Default: no records field.
	res := Status(ctx, reader, StatusOptions{})
	if res.Records != nil {
		t.Fatalf("records computed without IncludeRecords: %v", *res.Records)
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), `"records"`) {
		t.Fatalf("records key present by default: %s", b)
	}

	// Opt-in: identical array, same order, key present even when empty.
	res = Status(ctx, reader, StatusOptions{IncludeRecords: true})
	if res.Records == nil {
		t.Fatal("IncludeRecords did not populate records")
	}
	b, _ = json.Marshal(res)
	if !strings.Contains(string(b), `"records"`) {
		t.Fatalf("records key absent with IncludeRecords: %s", b)
	}
}
```

Also assert content equality with today's array: capture the record rows the fixture implies (kind/identity/location/path/version per `corpusRecords`' ordering: changes by id, ADRs by id, learnings by slug) and compare `*res.Records` against them — this pins "identical array, same order", not just presence.

Update `internal/app/status_result_test.go`: the failure-path loop asserting `{"changes":[]", "ready":[]", "records":[]", "findings":[]"}` drops `"records":[]` and gains a negative assert that `"records"` is absent from failure documents (failures never carry the inventory now; the other three arrays still marshal `[]`).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestStatusRecordsOptIn|TestNewStatusResult' -count=1`
Expected: FAIL — `StatusOptions` has no `IncludeRecords` field (compile error) or records always present.

- [ ] **Step 3: Implement**

`internal/app/status_result.go`:

```go
	// Records is the artifact-integrity inventory over the complete corpus.
	// It is computed and marshaled only when StatusOptions.IncludeRecords asks
	// for it (the --records flag): nil means "not requested" and the key is
	// absent; a non-nil empty slice still marshals as []. Absence is the
	// signal — never an empty array (change 0397).
	Records *[]StatusRecord `json:"records,omitempty"`
```

In `NewStatusResult`, delete the `if r.Records == nil { r.Records = []StatusRecord{} }` normalization (records are no longer one of the always-`[]` arrays) and update the function comment from "the four arrays" to "the three arrays".

`internal/app/status.go`:

```go
type StatusOptions struct {
	RepoDir    string
	Types      []string
	Priorities []string
	// IncludeRecords opts in to the corpus artifact-integrity inventory
	// (the records array). Off, the inventory is neither computed nor
	// marshaled (change 0397: the 130 KB majority of the payload that no
	// preflight, selection, or human read uses).
	IncludeRecords bool
}
```

At step 7 of `Status`:

```go
	var records *[]StatusRecord
	if opts.IncludeRecords {
		r := corpusRecords(snap, blobByPath)
		records = &r
	}
	findings := assembleFindings(pin.ConfigDiags, parseFindings, build.Report, artifactFindings)
```

and pass `Records: records` in the `NewStatusResult(ResultApplied, …)` literal. `corpusRecords` itself is unchanged.

`internal/cli/root.go` — wire the flag (learning: defaulted-param-hides-caller-wiring — the CLI test in Step 1/5 asserts the resolved non-default, i.e. records actually appear with the flag):

```go
	includeRecords, _ := c.Flags().GetBool("records")
	result = app.Status(c.Context(), app.NewGitStatusReader(client),
		app.StatusOptions{RepoDir: repoDir, Types: types, Priorities: priorities, IncludeRecords: includeRecords})
```

```go
	statusCmd.Flags().Bool("records", false, "include the corpus artifact-integrity inventory (the records array) in --json output")
```

Update `internal/cli/root_test.go`: the existing success-path assert list containing `` `"records":` `` moves to the negative (absent by default); add a run with `--records` asserting `"records":` present. The human renderer (`status_human.go`) never printed records — assert byte-identical human output with and without `--records` on the same fixture (spec §2's "human renderer is byte-identical either way").

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ ./internal/cli/ -run 'Status|Root|Records' -count=1`
Expected: PASS.

- [ ] **Step 5: Re-run the consumer audit at build time (spec §2 requires it at build, not only at design)**

Run: `git grep -nE '\brecords\b' -- skills/ agents/ scripts/ tests/ internal/ docs/superpowers cursor-rules/ README.md | grep -vE '_test\.go|testdata|docs/superpowers/(plans|specs)/2026-0[1-8]'`

Sort every hit into prose vs executable. Expected executable hits after this task: only the files this task edited (plus `internal/app/status.go`/`status_result.go` themselves). Any *other* executable consumer found is switched to `--records` (or its narrow need exposed) **in this task** before it completes. Record the audit command and outcome in the task's commit message body.

- [ ] **Step 6: Commit**

```bash
git add internal/app/status.go internal/app/status_result.go internal/app/status_result_test.go internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: status --json drops records by default behind --records opt-in"
```

---

### Task 2: `maintenance.preflight` composition in `internal/app` (verdict, envelope, seams)

**Files:**
- Create: `internal/app/maintenance_preflight.go`
- Test: `internal/app/maintenance_preflight_test.go`

**Interfaces:**
- Consumes: `MaintenanceSweep(ctx, deps FinalizeDeps, repoDir string, scope SweepScope) MaintenanceResult`; `Status(ctx, reader StatusReader, opts StatusOptions) StatusResult` with Task 1's `IncludeRecords`; `Envelope`/`NewEnvelope`/`Result` from `result.go`; sweep disposition constants.
- Produces (Task 3 wires the CLI to these):

```go
const OperationMaintenancePreflight = "maintenance.preflight"

const (
	PreflightClean   = "clean"
	PreflightProblem = "problem"
)

// PreflightSweepHalf is the sweep half of the preflight envelope: the sweep's
// own envelope result and report fields, plus the problem_entries projection.
type PreflightSweepHalf struct {
	Result                     Result             `json:"result"`
	Scope                      string             `json:"scope"`
	Entries                    []MaintenanceEntry `json:"entries"`
	ProblemEntries             []MaintenanceEntry `json:"problem_entries"`
	DeferredHistoricalCleanups int                `json:"deferred_historical_cleanups"`
	Reason                     string             `json:"reason,omitempty"`
	Message                    string             `json:"message,omitempty"`
	Findings                   []StatusFinding    `json:"findings"`
}

// PreflightStatusHalf is the compact post-sweep read: no records, no changes.
type PreflightStatusHalf struct {
	Result   Result          `json:"result"`
	Summary  StatusSummary   `json:"summary"`
	Ready    []int           `json:"ready"`
	Findings []StatusFinding `json:"findings"`
}

type MaintenancePreflightResult struct {
	Envelope
	Preflight        string               `json:"preflight"` // clean | problem
	Sweep            PreflightSweepHalf   `json:"sweep"`
	Status           *PreflightStatusHalf `json:"status,omitempty"` // absent when the sweep refused/errored
	MetadataRevision string               `json:"metadata_revision,omitempty"`
	Reason           string               `json:"reason,omitempty"`
	Message          string               `json:"message,omitempty"`
}

func (r MaintenancePreflightResult) HumanText() string
func MaintenancePreflight(ctx context.Context, deps FinalizeDeps, reader StatusReader, repoDir string) MaintenancePreflightResult
```

- internal test seam (mirrors `sweepOps`): `preflightOps{ sweep func(ctx context.Context) MaintenanceResult; status func(ctx context.Context) StatusResult }` and `maintenancePreflight(ctx context.Context, ops preflightOps) MaintenancePreflightResult`; `MaintenancePreflight` binds production closures (`MaintenanceSweep(..., SweepScopeImplementation)`, `Status(..., StatusOptions{RepoDir: repoDir, IncludeRecords: false})`).

**Verdict and envelope rules (spec §1, restated as the closed table the tests pin):**

1. `preflight = "problem"` iff the sweep failed as a whole (`Result` not `applied`/`no-op`) OR any entry disposition is `blocked`, `failed`, `unknown`, or `contended`. Otherwise `clean` — `applied`, `noop`, and every `skipped` (including the `reclaim-auto-disabled` policy skip) are clean.
2. `problem_entries` = the entries whose disposition is in that same four-token set, in sweep order. Marshals `[]`, never null.
3. Sweep refused/errored → `Status` nil, `Preflight = "problem"`, envelope `Result`/`Reason`/`Message` mirror the sweep's, `MetadataRevision` empty.
4. Sweep ok, status read failed → `Sweep` half intact, `Status` nil, envelope `Result` mirrors the status failure's actual v1 spelling (never `applied`), `Reason`/`Message` from the status failure. `Preflight` stays whatever rule 1 computed from the entries — a failed read is signaled by `Result`, not by faking a sweep problem; the parent halts on either signal.
5. Both ok → envelope `Result` = sweep's result (`applied` or `no-op`); `MetadataRevision` = the status half's pinned `Context.MetadataRevision` (the post-sweep pin — the same value the old child reported); `Status` populated from the compact projection (`Result`, `Summary`, `Ready`, `Findings` — no changes, no records).
6. The verdict is a pure function `preflightVerdict(sweep MaintenanceResult) (string, []MaintenanceEntry)` so it is trivially mutation-testable.

- [ ] **Step 1: Write the failing tests**

`internal/app/maintenance_preflight_test.go` — table-driven over the seam; no real repository (composition-only, per learning metadata-branch-invisible-to-suite the real-repo behavior is verified in Task 7):

```go
func TestPreflightCleanComposesBothHalves(t *testing.T) {
	sweep := newMaintenanceResult(ResultApplied, MaintenanceResult{
		Scope: string(SweepScopeImplementation),
		Entries: []MaintenanceEntry{
			{ID: 12, Kind: "closeout", Disposition: SweepDispApplied},
			{ID: 13, Kind: "reclaim", Disposition: SweepDispSkipped, Reason: "reclaim-auto-disabled"},
			{ID: 14, Kind: "closeout", Disposition: SweepDispNoOp},
		},
		DeferredHistoricalCleanups: 241,
	})
	status := NewStatusResult(ResultApplied, StatusResult{
		Context: StatusContext{MetadataRevision: "abc123"},
		Summary: StatusSummary{TotalChanges: 3},
		Ready:   []int{15},
	})
	res := maintenancePreflight(ctx, preflightOps{
		sweep:  func(context.Context) MaintenanceResult { return sweep },
		status: func(context.Context) StatusResult { return status },
	})
	// envelope
	if res.Operation != OperationMaintenancePreflight || res.ProtocolVersion != 1 || res.Result != ResultApplied {
		t.Fatalf("envelope: %+v", res.Envelope)
	}
	if res.Preflight != PreflightClean {
		t.Fatalf("verdict: %q", res.Preflight)
	}
	if len(res.Sweep.Entries) != 3 || len(res.Sweep.ProblemEntries) != 0 || res.Sweep.DeferredHistoricalCleanups != 241 {
		t.Fatalf("sweep half: %+v", res.Sweep)
	}
	if res.Status == nil || res.Status.Summary.TotalChanges != 3 || len(res.Status.Ready) != 1 {
		t.Fatalf("status half: %+v", res.Status)
	}
	if res.MetadataRevision != "abc123" {
		t.Fatalf("metadata revision: %q", res.MetadataRevision)
	}
	// the compact projection carries no records/changes keys at all
	b, _ := json.Marshal(res)
	for _, forbidden := range []string{`"records"`, `"changes"`} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("preflight envelope leaks %s: %s", forbidden, b)
		}
	}
}
```

One test per verdict rule (each named so the mutation probe in Step 5 maps 1:1):

```go
func TestPreflightProblemPerDisposition(t *testing.T) {
	for _, disp := range []string{SweepDispBlocked, SweepDispFailed, SweepDispUnknown, SweepDispContended} {
		// sweep: one applied entry + one entry with disp; status: applied.
		// assert Preflight == PreflightProblem and ProblemEntries == exactly the disp entry.
	}
}

func TestPreflightCleanDispositionsStayClean(t *testing.T) {
	// all of applied/noop/skipped (incl. reason reclaim-auto-disabled) => clean, empty problem_entries.
}

func TestPreflightSweepRefusalOmitsStatus(t *testing.T) {
	// sweep: maintenanceRefusal(ResultInvalidState, "some-reason", "msg"); status seam must NOT be called
	// (wire a status func that t.Fatal's).
	// assert: Result == ResultInvalidState, Reason == "some-reason", Preflight == PreflightProblem,
	// Status == nil, MetadataRevision == "", and marshal contains no "status" key.
}

func TestPreflightStatusFailureKeepsSweepHalf(t *testing.T) {
	// sweep: applied w/ clean entries; status: NewStatusResult(ResultExternalFailed, ...Reason...).
	// assert: Result == ResultExternalFailed (mirrored), Sweep half intact (entries preserved),
	// Status == nil, Preflight == PreflightClean (verdict is the sweep's), Reason from status failure.
}

func TestPreflightHumanText(t *testing.T) {
	// clean path: HumanText contains the verdict token "clean" and both halves' one-liners.
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestPreflight -count=1`
Expected: FAIL to compile — `maintenancePreflight`, `preflightOps`, DTOs undefined.

- [ ] **Step 3: Implement `internal/app/maintenance_preflight.go`**

```go
package app

import (
	"context"
	"fmt"
)

// OperationMaintenancePreflight is the operation key `maintenance preflight`
// records in its envelope: the implementation-scope sweep and the compact
// post-sweep read, sequenced in one process (change 0397). A thin composition
// over MaintenanceSweep and Status — no new sweep or status logic lives here.
const OperationMaintenancePreflight = "maintenance.preflight"

// The Go-computed preflight verdict vocabulary. `problem` when any sweep entry
// is blocked, failed, unknown, or contended — the rule docket-implement-next's
// Step 0 previously stated in prose; `clean` otherwise (an intentional policy
// skip and a genuine noop are clean).
const (
	PreflightClean   = "clean"
	PreflightProblem = "problem"
)

// problemDispositions is the closed set of entry dispositions that make the
// preflight verdict `problem` and populate problem_entries.
var problemDispositions = map[string]bool{
	SweepDispBlocked:   true,
	SweepDispFailed:    true,
	SweepDispUnknown:   true,
	SweepDispContended: true,
}
```

(DTOs exactly as the Interfaces block above.)

```go
// preflightVerdict computes the verdict and the problem_entries projection
// from the sweep alone. A whole-sweep failure is a problem with no entries;
// otherwise the verdict is entry-driven, in sweep order.
func preflightVerdict(sweep MaintenanceResult) (string, []MaintenanceEntry) {
	problems := []MaintenanceEntry{}
	for _, e := range sweep.Entries {
		if problemDispositions[e.Disposition] {
			problems = append(problems, e)
		}
	}
	if sweep.Result != ResultApplied && sweep.Result != ResultNoOp {
		return PreflightProblem, problems
	}
	if len(problems) > 0 {
		return PreflightProblem, problems
	}
	return PreflightClean, problems
}

// preflightOps is the injection seam: the two sequenced operations, already
// bound to their scope and options. Production binds MaintenanceSweep at
// SweepScopeImplementation and Status with IncludeRecords off; tests inject
// canned results so the composition rules are proved without a repository.
type preflightOps struct {
	sweep  func(ctx context.Context) MaintenanceResult
	status func(ctx context.Context) StatusResult
}

func maintenancePreflight(ctx context.Context, ops preflightOps) MaintenancePreflightResult {
	sweep := ops.sweep(ctx)
	verdict, problems := preflightVerdict(sweep)
	out := MaintenancePreflightResult{
		Preflight: verdict,
		Sweep: PreflightSweepHalf{
			Result:                     sweep.Result,
			Scope:                      sweep.Scope,
			Entries:                    sweep.Entries,
			ProblemEntries:             problems,
			DeferredHistoricalCleanups: sweep.DeferredHistoricalCleanups,
			Reason:                     sweep.Reason,
			Message:                    sweep.Message,
			Findings:                   sweep.Findings,
		},
	}
	if sweep.Result != ResultApplied && sweep.Result != ResultNoOp {
		// Whole-sweep refusal/error: no read is attempted — a parent must
		// never mistake a failed sweep for a failed read.
		out.Reason, out.Message = sweep.Reason, sweep.Message
		out.Envelope = NewEnvelope(OperationMaintenancePreflight, sweep.Result)
		return out
	}
	status := ops.status(ctx)
	if status.Result != ResultApplied {
		// Read failed after a successful sweep: sweep half stays intact and
		// the envelope mirrors the read's failure spelling, so the parent
		// never advances on either half.
		out.Reason, out.Message = status.Reason, status.Message
		out.Envelope = NewEnvelope(OperationMaintenancePreflight, status.Result)
		return out
	}
	out.Status = &PreflightStatusHalf{
		Result:   status.Result,
		Summary:  status.Summary,
		Ready:    status.Ready,
		Findings: status.Findings,
	}
	out.MetadataRevision = status.Context.MetadataRevision
	out.Envelope = NewEnvelope(OperationMaintenancePreflight, sweep.Result)
	return out
}

// MaintenancePreflight is the production entry point the CLI wires: the
// implementation-scope sweep, then the compact post-sweep read over a fresh
// pin (no records, no changes), in one process.
func MaintenancePreflight(ctx context.Context, deps FinalizeDeps, reader StatusReader, repoDir string) MaintenancePreflightResult {
	return maintenancePreflight(ctx, preflightOps{
		sweep: func(ctx context.Context) MaintenanceResult {
			return MaintenanceSweep(ctx, deps, repoDir, SweepScopeImplementation)
		},
		status: func(ctx context.Context) StatusResult {
			return Status(ctx, reader, StatusOptions{RepoDir: repoDir, IncludeRecords: false})
		},
	})
}
```

Normalize nil slices (`Entries`, `ProblemEntries`, `Findings`, and the status half's `Ready`/`Findings`) to `[]` before returning so every array marshals `[]` on every path — follow `newMaintenanceResult`'s pattern.

`HumanText` (non-`--json` output reuses the two existing renderers, spec §1):

```go
// HumanText reuses the two composed renderers — sweep line, then status
// report when present — prefixed by the verdict, never an authored body.
func (r MaintenancePreflightResult) HumanText() string {
	sweep := MaintenanceResult{ /* rebuild from r.Sweep + envelope for rendering */ }
	// Simplest faithful rebuild: stamp Envelope{Operation: OperationMaintenanceSweep, Result: r.Sweep.Result},
	// Entries, Scope, Reason, Message, DeferredHistoricalCleanups from r.Sweep.
	out := fmt.Sprintf("%s: %s\n%s", r.Operation, r.Preflight, sweep.HumanText())
	if r.Status != nil {
		// Render the compact half with the same one-line register:
		out += fmt.Sprintf("\nstatus: %d change(s), %d ready, %d finding(s)",
			r.Status.Summary.TotalChanges, len(r.Status.Ready), len(r.Status.Findings))
	}
	return out
}
```

(If rebuilding `MaintenanceResult` for rendering proves awkward, keep an unexported `renderSweepLine(PreflightSweepHalf) string` that produces the same one-line register — the requirement is reuse of the *register*, and no authored document body.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run TestPreflight -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-test every verdict rule (spec §5: "Each verdict rule is mutation-tested")**

For each mutation below: apply, run `go test ./internal/app/ -run TestPreflight -count=1`, confirm the named test REDDENS, restore the file from your editor buffer — never `git checkout` over uncommitted work (learning: mutation-restore-needs-a-backup-copy; keep a `cp` of the file first).

| Mutation | Must redden |
|---|---|
| Remove `SweepDispContended` from `problemDispositions` | `TestPreflightProblemPerDisposition` (contended case) |
| Remove `SweepDispUnknown` from the set | same (unknown case) |
| Make `preflightVerdict` return `PreflightClean` unconditionally | `TestPreflightProblemPerDisposition` |
| Drop the whole-sweep-failure early return (always call status) | `TestPreflightSweepRefusalOmitsStatus` (its status seam `t.Fatal`s) |
| On status failure, still populate `out.Status` | `TestPreflightStatusFailureKeepsSweepHalf` |
| Stamp envelope `ResultApplied` on the status-failure path | `TestPreflightStatusFailureKeepsSweepHalf` |
| Take `MetadataRevision` from `""` (skip assignment) | `TestPreflightCleanComposesBothHalves` |

Record the table outcome (each probe reddened) in the commit message body.

- [ ] **Step 6: Commit**

```bash
git add internal/app/maintenance_preflight.go internal/app/maintenance_preflight_test.go
git commit -m "feat: maintenance.preflight composition over sweep + compact status read"
```

---

### Task 3: CLI `docket maintenance preflight` + catalog entry

**Files:**
- Modify: `internal/cli/maintenance.go` (new subcommand)
- Modify: `internal/cli/capability_production_test.go` (representative-signature/effects pin)
- Test: `internal/cli/maintenance_test.go`

**Interfaces:**
- Consumes: `app.MaintenancePreflight(ctx, deps, reader, repoDir)` (Task 2); `newSweepFinalizeDeps()`; `app.NewGitStatusReader(gitcli client)`; `capability("maintenance.preflight", EffectMetadataWrite)`.
- Produces: catalog id `maintenance.preflight`, signature `[--repo-dir <dir>]`, effects `[metadata-write]` — the spelling Step-0 prose (Task 4) resolves from the capability catalog.

Note the deliberate effect declaration: the spec pins `[metadata-write]` (§1 "cataloged as `maintenance.preflight` with effects `[metadata-write]` (it runs the sweep)") even though `maintenance.sweep` itself carries the wider union — the preflight's implementation scope is the startup recovery pass and the spec owns this call. Do not widen it; cite the spec line in the annotation comment so a reviewer sees the choice was made upstream.

- [ ] **Step 1: Write the failing tests**

`internal/cli/maintenance_test.go` (follow the register of the existing sweep CLI tests — the package has `runCLI` helpers; if the existing maintenance tests run against fixture repos, reuse that harness):

```go
// The command exists, is annotated, and refuses nothing at parse time with
// just --repo-dir. A full end-to-end run needs a real repo; the app-layer
// composition is already proven, so this test pins WIRING: the command
// resolves flags, builds the sweep deps and status reader, and returns
// app.MaintenancePreflight's result to the presenter.
func TestMaintenancePreflightCommandWiring(t *testing.T) {
	// invalid repo-dir => protocol failure envelope with operation
	// maintenance.preflight (proves the result reaches the presenter).
	out, _, _ := runCLI(t, "maintenance", "preflight", "--repo-dir", t.TempDir(), "--json")
	if !strings.Contains(out, `"operation":"maintenance.preflight"`) {
		t.Fatalf("preflight envelope missing: %s", out)
	}
	if !strings.Contains(out, `"preflight":`) {
		t.Fatalf("verdict field missing: %s", out)
	}
}
```

`internal/cli/capability_production_test.go` — extend the `TestRepresentativeSignatures` `want` map:

```go
		// composition leaf: optional repo dir only (change 0397).
		"maintenance.preflight": "[--repo-dir <dir>]",
```

and add an effects pin next to the existing `entryByID` uses (the production catalog test covers the entry per spec §5):

```go
	if e, ok := entryByID(entries, "maintenance.preflight"); !ok {
		t.Error("maintenance.preflight absent from the catalog")
	} else if len(e.Effects) != 1 || e.Effects[0] != "metadata-write" {
		t.Errorf("maintenance.preflight effects = %v, want [metadata-write]", e.Effects)
	}
```

(Match the field/type names the catalog entry struct actually uses — read `collectCapabilities`' entry type in `internal/cli/capability.go` first.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'MaintenancePreflight|RepresentativeSignatures|Catalog' -count=1`
Expected: FAIL — unknown command "preflight", catalog entry absent.

- [ ] **Step 3: Implement in `internal/cli/maintenance.go`**

```go
// newMaintenancePreflightSubcommand builds `maintenance preflight`: the
// implementation-scope sweep followed by the compact post-sweep status read
// (no records, no changes), returned as one protocol-v1 envelope with the
// Go-computed clean|problem verdict (change 0397). One process, one envelope —
// the operation docket-implement-next's Step 0 runs inline instead of
// dispatching the docket-status composition.
func newMaintenancePreflightSubcommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run the implementation-scope sweep plus a compact post-sweep read as one operation",
		Args:  cobra.NoArgs,
		// metadata-write: the spec pins the preflight's declared effect to the
		// metadata mutation it exists to perform (spec 2026-09-02 §1:
		// "cataloged as `maintenance.preflight` with effects `[metadata-write]`").
		Annotations: capability("maintenance.preflight", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			deps, err := newSweepFinalizeDeps()
			if err != nil {
				return err
			}
			statusClient, err := gitcli.NewClient()
			if err != nil {
				return err
			}
			setResult(app.MaintenancePreflight(c.Context(), deps, app.NewGitStatusReader(statusClient), repoDir))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	return cmd
}
```

Register it in `newMaintenanceCommand` beside the sweep:

```go
	maintenanceCmd.AddCommand(newMaintenanceSweepSubcommand(setResult))
	maintenanceCmd.AddCommand(newMaintenancePreflightSubcommand(setResult))
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/cli/ -run 'MaintenancePreflight|RepresentativeSignatures|Catalog|Capability' -count=1`
Expected: PASS (the forward/reverse catalog population test counts the new annotated leaf automatically; if a hand-pinned leaf-count constant exists and reddens, bump it as the guard's own remedy directs).

- [ ] **Step 5: Mutation-test the effects pin**

Change the annotation to `capability("maintenance.preflight", EffectRead)`; run `go test ./internal/cli/ -run 'RepresentativeSignatures|Catalog' -count=1`; the effects assert must redden. Restore.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/maintenance.go internal/cli/maintenance_test.go internal/cli/capability_production_test.go
git commit -m "feat: docket maintenance preflight command, cataloged with metadata-write effects"
```

---

### Task 4: Rewrite `docket-implement-next` Step 0 to run the operation inline (+ its prose guards)

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (the `### Step 0 — Sync & sweep` section body; one sentence in Step 1's Acquisition paragraph)
- Modify: `internal/repoguard/prose_contracts_test.go` (the `change_0389_sweep_scope` implement-next row; new change-0397 rows)

**Interfaces:**
- Consumes: the catalog id `maintenance.preflight` and verdict vocabulary `clean|problem` (Tasks 2–3); the existing pre-claim run-reporting path and `repository.prepare` prose already in the SKILL (both unchanged).
- Produces: the retired-phrase set Task 5's convention/status edits must also not reintroduce.

This task and its guard-table edit land in **one commit**: the guard rows grep the prose copy, so editing one without the other reds the suite (learning: restatement-accumulates-its-own-guards; test-premise-deleted-not-regated — ask what each 0389 row *guards*: the agent-side completion barrier, whose premise a child-free Step 0 deletes).

- [ ] **Step 1: Replace the Step-0 dispatch paragraph**

In `skills/docket-implement-next/SKILL.md`, the `### Step 0 — Sync & sweep` section keeps its first paragraph (the `repository.prepare` re-sync) and replaces the entire dispatch paragraph (the one beginning "Then, before selection, **dispatch the `docket-status` subagent**…") with:

```markdown
Then, before selection, run the **implementation preflight** inline: resolve the `maintenance.preflight` operation from the capability catalog (never from a tool name) and run it **as its own Bash call** with `--json`. One process runs the implementation-scope sweep and the compact post-sweep read and returns one envelope; there is no child, so there is no completion barrier to arbitrate — a shell call that returns is terminal by construction (change 0397, retiring the step-0 `docket-status` dispatch of change 0017/0389).

Validate the envelope before trusting it: `protocol_version` MUST be `1`, `operation` MUST be `maintenance.preflight`, and `sweep.scope` MUST be `implementation`. Then key on exactly two fields — the envelope `result` and the Go-computed `preflight` verdict — never on prose and never on the process exit code. On `preflight: problem`, or any `result` other than `applied`/`no-op`, **halt before claiming** through the pre-claim run-reporting path, surfacing the envelope's `problem_entries` (and `reason`/`message` when the sweep or its post-sweep read failed as a whole) — before a claim there is no owned change record to write a halt on, and you never mutate an unrelated proposed record to carry it. A validation failure (wrong protocol, operation, or scope) or missing output halts the same way; no new run-gate state exists for any of this.

On `preflight: clean` with `result` `applied` or `no-op`, re-run the `repository.prepare` operation and derive readiness and claim state from fresh origin — the envelope is control-flow proof, never a substitute for metadata authority — then continue to Step 1. The envelope's `status` half (`summary`, `ready`, `findings`) is the preflight digest only; Step 1 takes its own fresh read.
```

Delete with the old paragraph every clause that only existed because a child could return early: the terminal-sweep-evidence barrier, "incomplete child return", late-notification correlation, child retirement verification, and the Tier-A inline fallback sentence (the operation is already inline). Do not touch the section's opening `repository.prepare` paragraph.

In Step 1's Acquisition paragraph, change "only AFTER Step 0's `docket-status` dispatch and metadata re-sync" to "only AFTER Step 0's `maintenance.preflight` run and metadata re-sync".

Then run the derivation grep the AGENTS.md rule requires before declaring the retirement complete inside this file:

`grep -n "docket-status" skills/docket-implement-next/SKILL.md`

Expected: zero step-0-dispatch hits remain (hits naming `docket-status` in unrelated contexts, if any, are read and left only when their claim survives this change).

- [ ] **Step 2: Update the guard table**

In `internal/repoguard/prose_contracts_test.go`, replace the implement-next `change_0389_sweep_scope` row (its required phrases — "terminal sweep evidence for implementation scope", "a contract violation, not a dismissable duplicate" — guard the agent-side barrier this change deletes) with change-0397 rows. Write the asserts that DETECT the removed state, not ones that merely confirm the new wording (learning: assert-detects-removal-not-replacement), and bind each present-phrase to its claim (learning: prose-guard-binds-phrase-to-claim):

```go
	// change 0397 — Step 0 is one inline deterministic operation. The absent
	// phrases are the retired step-0 dispatch instruction and the completion
	// barrier that only a child return needed; the present phrases bind the
	// inline call to its two authoritative fields.
	{sentinel: "change_0397_preflight_op", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"maintenance.preflight", "the envelope `result` and the Go-computed `preflight` verdict",
			"as its own Bash call"},
		absent: []string{"dispatch the `docket-status` subagent",
			"terminal sweep evidence for implementation scope"}},
```

- [ ] **Step 3: Run to verify pass**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: PASS.

- [ ] **Step 4: Mutation-test the new row both ways**

1. Re-insert the sentence fragment `dispatch the ` + `` `docket-status` `` + ` subagent` anywhere in the SKILL body → test must redden (absent-phrase leg). Restore.
2. Delete the `maintenance.preflight` spelling from the Step-0 paragraph → redden (present-phrase leg). Restore.
Run each probe with `-count=1`; record both reddened in the commit body.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md internal/repoguard/prose_contracts_test.go
git commit -m "feat: implement-next Step 0 runs maintenance.preflight inline, retiring the docket-status dispatch"
```

---

### Task 5: `docket-status` / `docket-convention` prose follow-through (+ their guard rows)

**Files:**
- Modify: `skills/docket-status/SKILL.md`
- Modify: `skills/docket-convention/SKILL.md`
- Modify: `README.md` (only if the Step-6 grep below finds a step-0-dispatch sentence — none is known at plan time)
- Modify: `internal/repoguard/prose_contracts_test.go` (the docket-status and convention `change_0389_sweep_scope` rows)

**Interfaces:**
- Consumes: Task 4's retired-phrase set; the surviving full-scope sweep barrier prose in `skills/docket-status/SKILL.md` (kept — the command barrier guards *any* sweep this skill still runs).
- Produces: the final maintained-prose state Task 7's measurements describe.

One commit, same reasoning as Task 4.

- [ ] **Step 1: Edit `skills/docket-status/SKILL.md`**

Spec §4: the two remaining modes are see-only and explicit refresh/cleanup; the `--scope implementation` vocabulary stays documented as the preflight operation's scope with a pointer to `maintenance.preflight`.

- *When to use* bullet list: delete the bullet "`docket-implement-next` calls this at step 0 as its implementation-preflight recovery before selecting the next change."
- *Mode choice* section: delete the "**`docket-implement-next`'s step-0 implementation preflight**" mode bullet entirely. Where the section (or the sweep-scope prose around it) documents `--scope implementation`, keep the scope documented but repoint its ownership, e.g. replace the deleted bullet with:

```markdown
- Implementation scope (`--scope implementation`) is the startup-preflight scope: current merged-work recovery plus reclaim gating, with independent historical cleanup retries deferred and counted in `deferred_historical_cleanups`. It is owned by the `maintenance.preflight` operation, which `docket-implement-next` runs inline at its Step 0 — not a mode of this skill. This skill's two modes are the see-only read and the explicit `--scope full` refresh/cleanup.
```

- The sentence "step-0 implementation preflight — run the `maintenance.sweep` operation … **before** the read" (the mode-dispatch line near the modes table) is deleted with its mode.
- The merge-sweep section's sentence "Runs at `docket-implement-next` step 0 in implementation scope, and in full scope on any explicit refresh/cleanup invocation." becomes "Runs inside the `maintenance.preflight` operation at implementation scope (`docket-implement-next` Step 0 runs that operation inline), and in full scope on any explicit refresh/cleanup invocation."
- **Keep** the command-barrier prose ("a liveness transition, not completion", "never start a second shell watcher", the applied-envelope caveat): it guards the sweeps this skill still runs in full scope; its premise survives.
- Leave the agent wrapper (`agents/docket-status.md`), harness pins, `docket:dispatch` blocks, and `cursor-rules/` untouched (spec §4 last bullet).

- [ ] **Step 2: Edit `skills/docket-convention/SKILL.md`**

- *Composition (change 0017)* paragraph: remove the `docket-status` step-0 dispatch as a composition edge. Rewrite the opening sentences to:

```markdown
**Composition (change 0017; step 0 amended by change 0397).** `docket-implement-next` dispatches `docket-adr` (step 6) — **foreground** (the parent suspends until the child returns), **unconditional**, and resting on **git state** on `origin/docket`, re-read after a re-sync: that is the whole contract, never an in-context return. Its step-0 implementation preflight is **not a dispatch**: it runs the `maintenance.preflight` operation inline — the implementation-scope sweep (the `maintenance.sweep` behavior at `--scope implementation`, so startup still no longer implies a full historical sweep) plus a compact post-sweep read, one process, one protocol-v1 envelope keyed on its `preflight` verdict (change 0397, amending change 0017 for step 0 only; decision recorded in the change-0397 ADR).
```

  Delete the now-orphaned hybrid-dispatch sentences about the status child ("The `docket-status` step-0 dispatch is **hybrid** (change 0389)…", "…its return must carry the scoped sweep's terminal protocol-v1 evidence…", "a first terminal result arriving after the parent advanced is a contract violation…"). Keep everything from "`docket-implement-next` also dispatches `docket-plan-writer` (step 4)…" onward unchanged, including the foreground/never-yield rule (other dispatches still need it).
- *Dispatch-capability resolution* Tier-A row: keep `docket-status` and `docket-adr` as the Tier-A composition dispatches (humans still dispatch docket-status; spec §4), but the row must no longer imply step 0 reaches it. The current row text names no step-0 sentence, so verify by grep (`grep -n "step" skills/docket-convention/SKILL.md` around the table) and edit only if a step-0 clause appears elsewhere in the tier table's surroundings.
- Preserve the phrase "no longer implies a full historical sweep" somewhere in the rewritten Composition text (as drafted above) — an existing guard row requires it and its claim is still true.

- [ ] **Step 3: Whole-repo retirement grep (derive the site list, never hand-list it)**

Run: `git grep -nE "step[- ]0|dispatch(es)? the .docket-status" -- skills/ agents/ README.md cursor-rules/ docs/cursor docs/codex internal/assets/embedded/tree/`

Sort hits into prose vs executable vs point-in-time. Expected live sites are the two skill files above (now clean) plus convention lines already edited. `internal/assets/embedded/tree/` mirrors of edited files must be regenerated/updated the way the repo's embedding is maintained (check `internal/assets` for a sync mechanism or test that pins tree contents — if a drift test reds at the gate, update the embedded copies in this task). Archived changes and frozen specs/plans stay as written. If a README or cursor/codex doc names the step-0 dispatch, update it to name the operation; at plan time none is known.

- [ ] **Step 4: Update the two remaining `change_0389_sweep_scope` guard rows**

- docket-status row: it requires `"--scope implementation"` plus the three command-barrier phrases. The barrier phrases survive Step 1; `--scope implementation` also survives (the scope stays documented). Verify the row still passes; if the retained phrases moved, repoint the row at the surviving sentences rather than deleting the guard (the premise — the command barrier — still stands).
- convention row: `"no longer implies a full historical sweep"` survives per Step 2. Verify.
- Add the reciprocal absence rows so the retired dispatch cannot come back through these two files either:

```go
	{sentinel: "change_0397_preflight_op", file: "skills/docket-status/SKILL.md",
		present: []string{"maintenance.preflight"},
		absent:  []string{"step-0 implementation preflight** ⇒"}},
	{sentinel: "change_0397_preflight_op", file: "skills/docket-convention/SKILL.md",
		present: []string{"runs the `maintenance.preflight` operation inline"},
		absent:  []string{"dispatches the `docket-status` subagent (step 0)"}},
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/repoguard/ -run TestProseContracts -count=1`
Expected: PASS.

- [ ] **Step 6: Mutation-test the new rows**

For each of the two new rows: (a) re-insert its absent phrase into the file → redden → restore; (b) delete its present phrase → redden → restore. Four probes, `-count=1` each, outcomes in the commit body.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-status/SKILL.md skills/docket-convention/SKILL.md internal/repoguard/prose_contracts_test.go
git commit -m "docs: retire the step-0 docket-status dispatch from status/convention prose"
```

(Add `README.md` / embedded-tree paths to the `git add` only if Step 3 actually edited them.)

---

### Task 6: ADR — "implementation preflight is a deterministic operation, not a composition dispatch"

**Files:**
- None on the feature branch. The ADR is a **metadata-branch artifact** (`docs/adrs/` on `docket`), recorded through the `docket-adr` flow at `docket-implement-next` Step 6 — never committed to `perf/run-the-implementation-preflight-as-one-deterministic-operat` (learning: metadata-branch-invisible-to-suite; the suite cannot see it and must not try).

**Interfaces:**
- Produces: the decision content Step 6's `docket-adr` dispatch records verbatim; the new ADR id is appended to change 0397's `adrs:` frontmatter by that flow.

- [ ] **Step 1: Hand the recording step this decision content**

Title: *Implementation preflight is a deterministic operation, not a composition dispatch.*

Decision: `docket-implement-next`'s Step-0 preflight runs the cataloged `maintenance.preflight` operation inline — one process sequencing the implementation-scope `MaintenanceSweep` with a compact `Status` read (no records, no changes) and returning one protocol-v1 envelope with a Go-computed `clean|problem` verdict over the sweep entries, the `problem_entries` subset, and the post-sweep metadata revision. The `docket-status` composition dispatch is retired for step 0 only.

Why: the dispatch topology cost ~2 minutes and ~85k tokens per run (fresh subagent bootstrap, 78 KB skill preload, a duplicated 164 KB status read, and a prose report the parent re-validated) around a sub-4-second, 159-byte sweep; and a child that returns before its sweep finishes is a completion-signalling failure class (0389's six-minute early return) an inline shell call cannot have. `docket status --json` simultaneously drops the 130 KB `records` inventory behind `--records`, because no preflight, selection, or human read consumed it.

Alternatives rejected (from the spec's resolved questions): a flag on `context.implementation` (would fold a metadata-write into a `read`-effect catalog entry, breaking the effect vocabulary); keeping and slimming the subagent (the bootstrap + prose report is the floor of the cost and keeps the failure class); skipping Step 0 only for targeted runs (moot at seconds of cost).

Relations: relates to ADR-0012 (script-vs-model boundary — the sweep is mechanical; the judgment follow-ups the status skill keeps are not preflight concerns), ADR-0024 (fork/dispatch completion — the child that cannot signal completion is the failure class removed), ADR-0101 (the sweep scope this operation runs at); **amends** the change-0017 composition decision for step 0 only. `docket-status` itself — skill, agent, wrapper, pins — is unretired and still serves see-only reads and explicit full maintenance.

- [ ] **Step 2: Verification for the executor**

This task is complete when the content above is present in the plan for Step 6 to consume — no build action, no commit. Do not create `docs/adrs/` files on this branch; the build gate's tree must not contain one.

---

### Task 7: Measurements (before/after) for the results file

**Files:**
- None committed by this task directly; the numbers land in the change's results file (metadata branch) at run close-out. Capture them into the build evidence now, while both states are reproducible.

**Interfaces:**
- Consumes: the finished binary states — `main` at `78d42319` (before) and this branch's HEAD after Tasks 1–5 (after).
- Produces: the measurement table the results file records (spec §5, last bullet; learnings: optimization-needs-a-measured-oracle, tolerance-constant-calibrated-on-one-machine — record the measurement context, not just the numbers).

- [ ] **Step 1: Measure `status --json` payload size, before and after**

From the feature worktree (after: current tree; before: `git -C /Users/homer/dev/docket` main checkout or `git stash`-free `go run` against the base commit):

```bash
# after (this tree)
go run ./cmd/docket status --json --repo-dir /Users/homer/dev/docket | wc -c
go run ./cmd/docket status --json --records --repo-dir /Users/homer/dev/docket | wc -c
# before (base): run from the primary tree, which is still at main
git -C /Users/homer/dev/docket log -1 --format=%H   # confirm 78d42319 lineage first
(cd /Users/homer/dev/docket && go run ./cmd/docket status --json --repo-dir /Users/homer/dev/docket | wc -c)
```

Expected shape: before ≈ 164 KB; after (no flag) ≈ 30–35 KB; after `--records` ≈ before. Capture exact bytes.

- [ ] **Step 2: Measure `maintenance preflight` wall clock**

```bash
time go run ./cmd/docket maintenance preflight --json --repo-dir /Users/homer/dev/docket > /private/tmp/claude-501/-Users-homer-dev-docket/a008b089-69d0-4170-a5d4-c46943992aed/scratchpad/preflight.json
wc -c /private/tmp/claude-501/-Users-homer-dev-docket/a008b089-69d0-4170-a5d4-c46943992aed/scratchpad/preflight.json
```

Note: this **runs the real implementation-scope sweep against the live repo** (a metadata-write effect). That is acceptable exactly once here — it is the operation's real startup semantics and the sweep is idempotent recovery — but record in the evidence which entries it produced. If the run environment must stay write-free, measure against a disposable clone instead and say so in the record.

- [ ] **Step 3: Record the deferred half of the measurement**

The "one real `docket-implement-next` run's Step-0 token and wall-clock cost before and after, from the harness transcript" cannot be produced inside this build (it requires a full before/after implement-next run in the harness). Record in the build evidence, for the results file: the before figures already measured in the spec (≈2 min, ≈85k tokens, spec §Problem table, measured 2026-09-02 at `78d42319`), and that the after figure is to be read from **this very run's own transcript** — this change's implementing run is the last dispatch-shaped Step 0, and the first inline Step 0 is the next change's run; name that follow-up explicitly so close-out copies the number in.

- [ ] **Step 4: Store the table in the build evidence**

Write the numbers (bytes before/after/with-flag, preflight wall clock, machine context: this Mac, date, repo state) into the run's build-evidence record so the results file and PR body can cite them. No feature-branch commit.

---

## Execution notes for docket-build

- Task order is 1 → 2 → 3 → (4, 5 in either order but each atomic) → 6 → 7. Tasks 2–3 depend on Task 1's `IncludeRecords`; Tasks 4–5 depend on Task 3's catalog id existing (their prose names it).
- Task 6 and Task 7 produce no feature-branch commits; they are evidence/handoff tasks — do not invent files for them.
- The single full-suite gate at the end is docket-build's own: `go run ./cmd/docket development test` from source, budget clauses per `tests/README.md`.
- Learnings consulted for this plan: optimization-needs-a-measured-oracle, restatement-accumulates-its-own-guards, assert-detects-removal-not-replacement, prose-guard-binds-phrase-to-claim, phrase-grep-over-wrapped-prose, test-premise-deleted-not-regated, cached-runner-serves-a-mutated-tree, mutation-restore-needs-a-backup-copy, defaulted-param-hides-caller-wiring, metadata-branch-invisible-to-suite, tolerance-constant-calibrated-on-one-machine.

## Self-review

- Spec §1 (operation, envelope, verdict, human output) → Tasks 2–3. Spec §2 (records opt-in, shared option, no computation when unrequested, audit) → Task 1. Spec §3 (Step-0 inline rewrite, halt posture, barrier prose removal) → Task 4. Spec §4 (status/convention/README prose, ADR, wrapper untouched) → Tasks 5–6; `scripts/docket-status.md` resolved as absent-at-build (Reconcile-time facts). Spec §5 (mutation-tested verdict rules, catalog test, records opt-in test, prose guards, measurements) → Tasks 1–5 test steps + Task 7.
- Envelope naming consistent across tasks: `MaintenancePreflightResult`, `PreflightSweepHalf`, `PreflightStatusHalf`, `preflightVerdict`, `preflightOps`, `maintenancePreflight`, `MaintenancePreflight`, `OperationMaintenancePreflight`, `PreflightClean`/`PreflightProblem`; `StatusOptions.IncludeRecords`, `Records *[]StatusRecord`.
- Known deliberate deviations, argued from the repo's real types: (a) spec's `refused|error` result spellings map to the actual v1 taxonomy (Global Constraints); (b) `Records` becomes a pointer because `omitempty` cannot express "requested but empty" on a slice; (c) `preflight` on the status-read-failure path reflects the sweep's entry-driven verdict while the envelope `result` carries the failure — the parent halts on `result` there, satisfying the spec's "never advances on either".
