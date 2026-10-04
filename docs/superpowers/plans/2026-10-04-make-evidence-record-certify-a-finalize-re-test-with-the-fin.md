<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0517 — Make evidence.record certify a finalize re-test with the finalize gate settings](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0517-make-evidence-record-certify-a-finalize-re-test-with-the-fin.md)**
<!-- docket:backlink:end -->
# evidence.record certifies a finalize run with the finalize settings — Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build`: each task is routed to the
> named tier agent under the `docket-build-task` contract, and `docket-build` runs the single
> full-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `evidence.record` takes an optional `--owner build|finalize`; with `finalize` it records
`finalize.test_command` and never mints skipped evidence, and both finalize call sites (the
`finalize.rebase` built-in gate and the skill's repaired-head re-test) pass that owner.

**Architecture:** `EvidenceRecordRequest` gains an `Owner` field validated before any config read;
`EvidenceRecord` splits on it the same way `gate drive start --owner` already splits (each owner
reads only its own `<owner>.test_command`). `processFinalizeGate.mapTerminalDrive` hands the
seam's own owner (zero value = finalize) into the request, so `evidence.recertify`'s build-owned
drive stays build. The CLI adds the flag; the finalize skill text adds `--owner finalize` to its
`evidence.record` call, guarded by a new `TestAlignmentContracts` sentinel.

**Tech Stack:** Go (`internal/app`, `internal/cli`, `internal/repoguard`, `internal/assets`
generator), Markdown skill prose.

**Spec:** `docs/superpowers/specs/2026-10-04-make-evidence-record-certify-a-finalize-re-test-with-the-fin-design.md`
(on the `docket` branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary
checkout).

## Global Constraints

- `--owner` is **optional**; empty means `build`, and every existing caller keeps its exact behavior.
- Any owner other than `build`, `finalize`, or empty is refused as invalid input with stable reason
  `invalid-owner`, **before** config is read.
- Owner `finalize` reads **only** `finalize.test_command`: no skipped path, `finalize.gate` is not
  consulted, an empty `finalize.test_command` refuses `unconfigured-gate-command` with a message
  naming `finalize.test_command`.
- The evidence record format (`internal/evidence`) is unchanged — the record gains no owner field.
- `evidence.recertify` stays build-owned (its drive seam owner is `build`; its gate-off branch calls
  `EvidenceRecord` with no owner).
- Every test fixture configures build and finalize with **different** commands (`build-cmd` vs
  `finalize-cmd`) — identical commands cannot tell the old code from the new.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line
  numbers (ADR-0054).
- Every mutation probe runs with `go test -count=1` (the Go test cache serves a stale pass), and
  every mutation restores from a `cp` backup with `mv -f`, never `git checkout --`.
- The `internal/app` default (untagged) corpus must never start real git: tests that build
  `evidenceDepsWithConfig` (it calls `newWorkingRepo`) go in the `//go:build integration` file
  `internal/app/evidence_ops_integration_test.go` with the `TestIntegrationEvidence` name prefix.

## Review Focus

1. **Owner spelling drift** (`Finalize`, ` build`, `review`): refused `invalid-owner`, never
   silently treated as build — pinned in Task 1 (`TestEvidenceRecordUnknownOwnerRefusedBeforeConfigRead`).
2. **Finalize owner with no `--run`**: refused `missing-run-dir` with a message that does not
   claim `build.gate` is local — pinned in Task 1 (`…FinalizeOwnerMissingRunDirNamesNoBuildGate`).
3. **Finalize owner at a moved head**: still refused `head-mismatch` (the shared
   `verifyFeatureHead` precondition is not bypassed by the new branch) — pinned in Task 1.
4. **`finalize.gate: off` with owner finalize**: the gate setting is not consulted, a passed run
   still records green `finalize-cmd` — pinned in Task 1's finalize table.
5. **Zero-value `processFinalizeGate{}` seam** (owner unset): certifies as finalize, never as
   build — pinned in Task 2's table.

---

### Task 1: `EvidenceRecord` owner split

**Tier:** standard

**Files:**
- Modify: `internal/app/evidence_ops.go` (file header comment, `ReasonEvidence*` consts,
  `EvidenceRecordRequest`, `EvidenceRecord`)
- Test: `internal/app/evidence_ops_integration_test.go` (integration tag)
- Test: `internal/app/evidence_ops_test.go` (default corpus — the invalid-owner test only)

**Interfaces:**
- Consumes: existing `EvidenceRecord(ctx, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecordRequest) EvidenceOpResult`;
  test helpers `evidenceDepsWithConfig`, `evidenceRecordWithConfig`, `readyWorkspace`,
  `passedRunDir`, `currentFeatureHead`, consts `evidenceHead`, `evidenceOtherHead`.
- Produces (used by Tasks 2 and 3):
  - `const EvidenceOwnerBuild = "build"`, `const EvidenceOwnerFinalize = "finalize"`
  - `const ReasonEvidenceInvalidOwner = "invalid-owner"`
  - `EvidenceRecordRequest.Owner string` with tag `json:"owner"`
  - test consts in the integration file: `ownerBuildCmd`, `ownerFinalizeCmd`, `ownerMixedLocal`,
    `ownerBuildOff`, `ownerBuildEmpty`, `ownerFinalizeEmpty`, `ownerFinalizeGateOff`, and helper
    `evidenceRecordPassedRunAs(t, yaml, owner string) EvidenceOpResult`.

- [ ] **Step 1: Write the failing integration tests**

Add to `internal/app/evidence_ops_integration_test.go`. Extend its import block to:

```go
import (
	"context"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/testsupport"
)
```

Append:

```go
// --- record: per-request owner (change 0517) ---------------------------------

// Mixed-owner fixtures: build and finalize ALWAYS carry different commands.
// With identical commands the build-owned and finalize-owned paths produce the
// same record, so only a mixed fixture can tell them apart (learning
// shared-resource-keeps-first-owner-assumptions).
const (
	ownerBuildCmd        = "build-cmd"
	ownerFinalizeCmd     = "finalize-cmd"
	ownerMixedLocal      = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerBuildOff        = "build:\n  gate: \"off\"\n  test_command: build-cmd\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerBuildEmpty      = "build:\n  gate: local\nfinalize:\n  gate: local\n  test_command: finalize-cmd\n"
	ownerFinalizeEmpty   = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: local\n"
	ownerFinalizeGateOff = "build:\n  gate: local\n  test_command: build-cmd\nfinalize:\n  gate: \"off\"\n  test_command: finalize-cmd\n"
)

// evidenceRecordPassedRunAs runs EvidenceRecord over a real passed gate run dir
// at the current feature head, under the given YAML overlay and owner.
func evidenceRecordPassedRunAs(t *testing.T, yaml, owner string) EvidenceOpResult {
	t.Helper()
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), yaml)
	return EvidenceRecord(context.Background(), deps, wdeps, repoDir,
		EvidenceRecordRequest{ID: 7, RunDir: passedRunDir(t), Head: evidenceHead, Owner: owner})
}

// assertGreenCommand fails unless res is an applied green record naming want,
// both in the result field and inside the rendered canonical block.
func assertGreenCommand(t *testing.T, res EvidenceOpResult, want string) {
	t.Helper()
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s: %s), want applied", res.Result, res.Reason, res.Message)
	}
	if res.Outcome != string(evidence.ResultGreen) || res.Command != want {
		t.Fatalf("outcome/command = %q/%q, want green/%q", res.Outcome, res.Command, want)
	}
	rec, err := evidence.Extract([]byte(res.Block))
	if err != nil {
		t.Fatalf("block does not parse: %v\n%s", err, res.Block)
	}
	if rec.Result != evidence.ResultGreen || rec.Command != want {
		t.Fatalf("block record = %s/%q, want green/%q", rec.Result, rec.Command, want)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerCertifiesFinalizeCommand: owner
// finalize records finalize.test_command whatever the build settings say —
// (a) build.gate off is NOT skipped, (b) a divergent build command is not
// recorded, (c) an empty build.test_command is not refused — and finalize.gate
// is not consulted.
func TestIntegrationEvidenceRecordFinalizeOwnerCertifiesFinalizeCommand(t *testing.T) {
	for _, tc := range []struct{ name, yaml string }{
		{"build-gate-off", ownerBuildOff},
		{"both-local", ownerMixedLocal},
		{"build-command-empty", ownerBuildEmpty},
		{"finalize-gate-off-not-consulted", ownerFinalizeGateOff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertGreenCommand(t, evidenceRecordPassedRunAs(t, tc.yaml, EvidenceOwnerFinalize), ownerFinalizeCmd)
		})
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerUnconfiguredCommandRefused: an
// empty finalize.test_command is a typed setup refusal naming the FINALIZE key.
func TestIntegrationEvidenceRecordFinalizeOwnerUnconfiguredCommandRefused(t *testing.T) {
	res := evidenceRecordWithConfig(t, ownerFinalizeEmpty, EvidenceRecordRequest{
		ID: 7, Head: currentFeatureHead(t), RunDir: testsupport.TempDir(t), Owner: EvidenceOwnerFinalize})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result/reason = %v/%s, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if !strings.Contains(res.Message, "finalize.test_command") || strings.Contains(res.Message, "build.") {
		t.Errorf("message %q must name finalize.test_command and no build.* key", res.Message)
	}
	if !strings.Contains(res.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the setup remedy", res.Message)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerMissingRunDirNamesNoBuildGate:
// owner finalize with no --run is missing-run-dir, and the message does not
// claim a build gate setting (Review Focus 2).
func TestIntegrationEvidenceRecordFinalizeOwnerMissingRunDirNamesNoBuildGate(t *testing.T) {
	res := evidenceRecordWithConfig(t, ownerBuildOff, EvidenceRecordRequest{
		ID: 7, Head: currentFeatureHead(t), Owner: EvidenceOwnerFinalize})
	if res.Result != ResultInvalidInput || res.Reason != ReasonEvidenceMissingRunDir {
		t.Fatalf("result/reason = %v/%s, want invalid-input/%s", res.Result, res.Reason, ReasonEvidenceMissingRunDir)
	}
	if strings.Contains(res.Message, "build.") {
		t.Errorf("finalize-owned message %q must not name a build.* key", res.Message)
	}
}

// TestIntegrationEvidenceRecordFinalizeOwnerHeadMismatchRefused: the new
// finalize branch still goes through verifyFeatureHead (Review Focus 3).
func TestIntegrationEvidenceRecordFinalizeOwnerHeadMismatchRefused(t *testing.T) {
	deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), ownerMixedLocal)
	res := EvidenceRecord(context.Background(), deps, wdeps, repoDir, EvidenceRecordRequest{
		ID: 7, RunDir: passedRunDir(t), Head: evidenceOtherHead, Owner: EvidenceOwnerFinalize})
	if res.Reason != ReasonEvidenceHeadMismatch {
		t.Fatalf("reason = %s (%v), want %s", res.Reason, res.Result, ReasonEvidenceHeadMismatch)
	}
}

// TestIntegrationEvidenceRecordBuildOwnerAndOmittedStayBuild: owner build and
// an omitted owner behave exactly as before 0517 on the mixed fixture — green
// naming build-cmd under a local build gate, skipped under build.gate: off.
func TestIntegrationEvidenceRecordBuildOwnerAndOmittedStayBuild(t *testing.T) {
	for _, tc := range []struct{ name, owner string }{
		{"omitted", ""},
		{"build", EvidenceOwnerBuild},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertGreenCommand(t, evidenceRecordPassedRunAs(t, ownerMixedLocal, tc.owner), ownerBuildCmd)

			off := evidenceRecordWithConfig(t, ownerBuildOff,
				EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t), Owner: tc.owner})
			if off.Result != ResultApplied || off.Outcome != string(evidence.ResultSkipped) {
				t.Fatalf("gate-off result/outcome = %v/%q (%s), want applied/skipped", off.Result, off.Outcome, off.Reason)
			}
			if off.Command != "" || !strings.Contains(off.Block, evidence.ReasonBuildGateOff) {
				t.Errorf("gate-off command/block = %q/%q, want empty/%s", off.Command, off.Block, evidence.ReasonBuildGateOff)
			}
		})
	}
}
```

And add to `internal/app/evidence_ops_test.go` (default corpus — it touches no git and no config):

```go
// TestEvidenceRecordUnknownOwnerRefusedBeforeConfigRead (change 0517): an owner
// other than build/finalize/empty is invalid input with its own reason. The
// zero PlanningDeps has a nil Reader, so any config read would panic — a typed
// refusal proves the owner check runs before config is read. Near-miss
// spellings are refused, never folded into build (Review Focus 1).
func TestEvidenceRecordUnknownOwnerRefusedBeforeConfigRead(t *testing.T) {
	for _, owner := range []string{"review", "Finalize", " build", "build "} {
		res := EvidenceRecord(context.Background(), PlanningDeps{}, WorkspaceDeps{}, testsupport.TempDir(t),
			EvidenceRecordRequest{ID: 7, Head: evidenceHead, Owner: owner})
		if res.Result != ResultInvalidInput || res.Reason != ReasonEvidenceInvalidOwner {
			t.Errorf("owner %q: result/reason = %v/%s, want invalid-input/%s", owner, res.Result, res.Reason, ReasonEvidenceInvalidOwner)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestEvidenceRecordUnknownOwner' ./internal/app`
and: `go test -count=1 -tags integration -run '^TestIntegrationEvidenceRecord(FinalizeOwner|BuildOwnerAndOmitted)' ./internal/app`
Expected: compile failure (`EvidenceOwnerFinalize`, `ReasonEvidenceInvalidOwner`, field `Owner`
undefined).

- [ ] **Step 3: Implement the owner split in `internal/app/evidence_ops.go`**

Add `"strconv"` to the import block.

Replace the header bullet that begins "The recorded command is the OBSERVED gate command" with:

```go
//   - The recorded command is the OBSERVED gate command: the owner's resolved
//     test_command (build.test_command or finalize.test_command), read from
//     authoritative config. There is no agent-supplied command and no
//     agent-supplied `passed` boolean in the request shapes — the terminal
//     record and the config are the only inputs.
```

Replace the header paragraph that begins "EvidenceRecord is BUILD-owned (change 0374)" with:

```go
// EvidenceRecord is owned per request (change 0517), with the same split
// `gate drive start --owner` uses: each owner reads ONLY its own test_command.
// Owner build — the default when the request names none — validates against
// build configuration (change 0374): an explicit build.gate: off mints truthful
// skipped evidence with no run observed, and a local build gate with no
// build.test_command is a typed setup refusal. Owner finalize reads ONLY
// finalize.test_command: it never mints skipped evidence (its only reason,
// build-gate-off, would be false) and never consults finalize.gate — the gate
// setting decides whether finalize runs its suite, and once that run passed,
// the record states what ran.
```

Change the `ReasonEvidenceUnconfiguredGate` comment and add the new reason beside it:

```go
	// ReasonEvidenceUnconfiguredGate: the owner's test_command is unconfigured
	// (build.test_command under a local build gate, or finalize.test_command),
	// so there is no gate command to record.
	ReasonEvidenceUnconfiguredGate = "unconfigured-gate-command"
	// ReasonEvidenceInvalidOwner: the request named an owner other than build or
	// finalize. It is refused before config is read.
	ReasonEvidenceInvalidOwner = "invalid-owner"
```

Also reword the `ReasonEvidenceMissingRunDir` comment to "the request named no run dir to
observe on a path that records a run (a local build gate, or owner finalize); only build-owned
gate-off skips it."

Add after the reason const block:

```go
// Evidence record owners: whose gate settings certify the run. An empty
// request owner means EvidenceOwnerBuild, so every pre-0517 caller is unchanged.
const (
	EvidenceOwnerBuild    = "build"
	EvidenceOwnerFinalize = "finalize"
)
```

Replace `EvidenceRecordRequest` with:

```go
// EvidenceRecordRequest is the closed request for `evidence record`. There is
// deliberately no command field (the gate command is observed from config) and
// no `passed` boolean (the terminal record decides). RunDir is absolute. Owner
// selects whose settings certify the run: EvidenceOwnerBuild (or empty) or
// EvidenceOwnerFinalize.
type EvidenceRecordRequest struct {
	ID     int    `json:"id"`
	RunDir string `json:"run_dir"`
	Head   string `json:"head"`
	Owner  string `json:"owner"`
}
```

Replace the `EvidenceRecord` doc comment and the body from step (1) through step (4) (everything
before the `// (5) Observe the run.` comment) with:

```go
// EvidenceRecord validates the request owner, then resolves that owner's gate
// policy. Owner build (or empty) either mints truthful skipped evidence
// (build.gate: off) or, for a local gate, records build.test_command; owner
// finalize always records finalize.test_command. Either recording requires a
// `passed` terminal at the current feature head. It returns the immutable
// typed record plus its canonical rendered block. It writes no second evidence
// store: the block travels as bytes and becomes the durable record only after
// `pr publish`.
func EvidenceRecord(ctx context.Context, deps PlanningDeps, wdeps WorkspaceDeps, repoDir string, req EvidenceRecordRequest) EvidenceOpResult {
	// (0) The owner is validated BEFORE config is read: an unknown owner names
	// no settings to read, and near-miss spellings are never folded into build.
	owner := req.Owner
	if owner == "" {
		owner = EvidenceOwnerBuild
	}
	if owner != EvidenceOwnerBuild && owner != EvidenceOwnerFinalize {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceInvalidOwner,
			"--owner must be build or finalize, got "+strconv.Quote(req.Owner), req.ID)
	}

	// (1) Pin authoritative config — the owner's gate policy decides everything
	// downstream, including whether a run is observed at all.
	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, r := classifyStatusError(ctx, err)
		return newEvidenceRefusal(OperationEvidenceRecord, result, r, err.Error(), req.ID)
	}
	eff := pin.Config.Effective

	// (2)+(3) Resolve the owner's command. Each owner reads ONLY its own key
	// (the gate-drive split, ADR-0102).
	var command, unconfigured, missingRun string
	if owner == EvidenceOwnerFinalize {
		// Finalize: no skipped path and no finalize.gate read — a finalize run
		// that passed has already run, and the record states what ran.
		command = eff.Finalize.TestCommand.Value
		unconfigured = "finalize.test_command is unconfigured, so a finalize run has no command to record; run `docket repository configure-tests` and review the pending edit"
		missingRun = "--owner finalize records a finalize run; --run must name the gate run directory to observe"
	} else {
		build := eff.Build
		// build.gate: off — an explicit no-gate policy. Mint truthful skipped
		// evidence at the verified current feature head; observe no run.
		if build.Gate.Value == "off" {
			if refusal, ok := verifyFeatureHead(ctx, deps, wdeps, repoDir, req); !ok {
				return refusal
			}
			rec, err := evidence.NewSkippedRecord(req.Head, deps.Clock.Now())
			if err != nil {
				return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceInvalidRecord, err.Error(), req.ID)
			}
			return EvidenceOpResult{
				Envelope: NewEnvelope(OperationEvidenceRecord, ResultApplied),
				ID:       req.ID,
				Head:     rec.Head,
				RanAt:    rec.RanAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
				Outcome:  string(rec.Result),
				Reason:   rec.Reason,
				Block:    evidence.Render(rec),
			}
		}
		// A local build gate records build.test_command — never finalize's.
		command = build.TestCommand.Value
		unconfigured = "build.gate is local but build.test_command is unconfigured; run `docket repository configure-tests` and review the pending edit"
		missingRun = "build.gate is local; --run must name the gate run directory to observe"
	}
	if command == "" {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultUnsupportedConfig, ReasonEvidenceUnconfiguredGate, unconfigured, req.ID)
	}

	// (4) A recorded run must be a real run dir.
	if req.RunDir == "" {
		return newEvidenceRefusal(OperationEvidenceRecord, ResultInvalidInput, ReasonEvidenceMissingRunDir, missingRun, req.ID)
	}
```

Leave steps (5)–(7) (observe, `verifyFeatureHead`, `evidence.NewRecord(command, …)`) unchanged —
they already use the local `command`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestEvidence' ./internal/app`
and: `go test -count=1 -tags integration -run '^TestIntegrationEvidence' ./internal/app`
Expected: PASS, including the pre-existing `TestIntegrationEvidenceEvidenceRecord*` tests
(build-owned behavior unchanged).

- [ ] **Step 5: Mutation-test the owner switch**

```bash
f=internal/app/evidence_ops.go
cp "$f" "$f.bak"
grep -c 'if owner == EvidenceOwnerFinalize {' "$f"   # expect 1
sed -i '' 's/if owner == EvidenceOwnerFinalize {/if false \&\& owner == EvidenceOwnerFinalize {/' "$f"
grep -c 'if false && owner == EvidenceOwnerFinalize {' "$f"   # expect 1 — the mutation landed
go test -count=1 -tags integration -run '^TestIntegrationEvidenceRecordFinalizeOwnerCertifiesFinalizeCommand' ./internal/app
mv -f "$f.bak" "$f"
```

Expected: the `build-gate-off`, `both-local`, and `build-command-empty` subtests FAIL (skipped,
`build-cmd`, and `unconfigured-gate-command` respectively). Then mutate the owner validation the
same backup way:

```bash
cp "$f" "$f.bak"
sed -i '' 's/if owner != EvidenceOwnerBuild \&\& owner != EvidenceOwnerFinalize {/if false {/' "$f"
grep -c 'if false {' "$f"   # expect at least 1 — the mutation landed
go test -count=1 -run 'TestEvidenceRecordUnknownOwnerRefusedBeforeConfigRead' ./internal/app
mv -f "$f.bak" "$f"
```

Expected: FAIL (a panic on the nil Reader counts as red). Re-run Step 4 to confirm green after
restore.

- [ ] **Step 6: Commit**

```bash
git add internal/app/evidence_ops.go internal/app/evidence_ops_test.go internal/app/evidence_ops_integration_test.go
git commit -m "fix(evidence): evidence.record certifies a finalize run with finalize.test_command"
```

---

### Task 2: The `finalize.rebase` built-in gate hands its owner to `EvidenceRecord`

**Tier:** standard

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`processFinalizeGate.mapTerminalDrive`)
- Test: `internal/app/evidence_ops_integration_test.go` (integration tag)

**Interfaces:**
- Consumes: from Task 1 — `EvidenceOwnerBuild`, `EvidenceOwnerFinalize`,
  `EvidenceRecordRequest.Owner`, test consts `ownerMixedLocal`, `ownerBuildOff`, `ownerBuildCmd`,
  `ownerFinalizeCmd`; existing `gateOwnerFinalize`, `gateOwnerBuild`, `processFinalizeGate{planning, wdeps, owner}`,
  `(*processFinalizeGate).mapTerminalDrive(ctx, req LocalGateRequest, doc *gatedrive.DriveDoc, removeRoot *bool) LocalGateResult`,
  `LocalGateRequest{RepoDir, ID, WorkspaceDir, Head}`, `gatedrive.DriveDoc{Outcome, RawRunDir}`.
- Produces: nothing new for later tasks.

- [ ] **Step 1: Write the failing test**

Add `"github.com/danielhanold/docket/internal/gatedrive"` to the integration file's imports and
append:

```go
// TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner (change 0517):
// a PASSED drive in the finalize.rebase built-in gate certifies with the
// seam's own owner. The finalize seam — and the zero-value seam, which the
// seam treats as finalize — records finalize-cmd, green even under
// build.gate: off; the build-owned recertify seam still records build-cmd.
func TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner(t *testing.T) {
	for _, tc := range []struct{ name, owner, yaml, want string }{
		{"finalize-seam", gateOwnerFinalize, ownerMixedLocal, ownerFinalizeCmd},
		{"zero-value-seam-is-finalize", "", ownerMixedLocal, ownerFinalizeCmd},
		{"finalize-seam-build-gate-off", gateOwnerFinalize, ownerBuildOff, ownerFinalizeCmd},
		{"recertify-build-seam", gateOwnerBuild, ownerMixedLocal, ownerBuildCmd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deps, wdeps, repoDir := evidenceDepsWithConfig(t, readyWorkspace(), tc.yaml)
			g := &processFinalizeGate{planning: deps, wdeps: wdeps, owner: tc.owner}
			removeRoot := true
			doc := &gatedrive.DriveDoc{Outcome: gatedrive.PASSED, RawRunDir: passedRunDir(t)}
			res := g.mapTerminalDrive(context.Background(),
				LocalGateRequest{RepoDir: repoDir, ID: 7, Head: evidenceHead}, doc, &removeRoot)
			if res.Outcome != FinalizeGatePassed {
				t.Fatalf("outcome = %s (halt %s), want passed", res.Outcome, res.HaltCause)
			}
			rec, err := evidence.Extract([]byte(res.Evidence))
			if err != nil {
				t.Fatalf("evidence block does not parse: %v\n%s", err, res.Evidence)
			}
			if rec.Result != evidence.ResultGreen || rec.Command != tc.want {
				t.Fatalf("evidence = %s/%q, want green/%q", rec.Result, rec.Command, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 -tags integration -run '^TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner' ./internal/app`
Expected: FAIL — `finalize-seam` and `zero-value-seam-is-finalize` record `build-cmd`;
`finalize-seam-build-gate-off` records `skipped`. `recertify-build-seam` passes.

- [ ] **Step 3: Implement the hand-off**

In `processFinalizeGate.mapTerminalDrive`, replace the `case gatedrive.PASSED:` call:

```go
	case gatedrive.PASSED:
		// Certify with the seam's own owner (change 0517), applying the seam's
		// zero-value rule: anything but the build owner is finalize, so an empty
		// owner never reaches EvidenceRecord (where empty would mean build).
		owner := EvidenceOwnerFinalize
		if g.owner == gateOwnerBuild {
			owner = EvidenceOwnerBuild
		}
		evd := EvidenceRecord(ctx, g.planning, g.wdeps, req.RepoDir,
			EvidenceRecordRequest{ID: req.ID, RunDir: doc.RawRunDir, Head: req.Head, Owner: owner})
```

Leave the rest of the case unchanged. Do not touch `EvidenceRecertify`'s gate-off call in
`internal/app/evidence_recertify.go` — it stays build (no owner).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -tags integration -run '^TestIntegrationEvidence' ./internal/app`
and: `go test -count=1 -run 'TestMapDriveOutcome|TestFinalize' ./internal/app`
Expected: PASS.

- [ ] **Step 5: Mutation-test the hand-off**

```bash
f=internal/app/finalize_rebase.go
cp "$f" "$f.bak"
grep -c 'Head: req.Head, Owner: owner})' "$f"   # expect 1
sed -i '' 's/Head: req.Head, Owner: owner})/Head: req.Head})/' "$f"
grep -c 'Head: req.Head, Owner: owner})' "$f"   # expect 0 — the mutation landed
go test -count=1 -tags integration -run '^TestIntegrationEvidenceFinalizeGatePassCertifiesWithSeamOwner' ./internal/app
mv -f "$f.bak" "$f"
```

(If the compiler rejects the now-unused `owner` variable, also prefix its declaration with `_ = owner;`
in the mutated copy — the point is the request carries no owner.) Expected: the three finalize
rows FAIL. Re-run Step 4 to confirm green after restore.

- [ ] **Step 6: Commit**

```bash
git add internal/app/finalize_rebase.go internal/app/evidence_ops_integration_test.go
git commit -m "fix(finalize): the built-in gate certifies a pass with the seam's owner"
```

---

### Task 3: `docket evidence record --owner`

**Tier:** economy

**Files:**
- Modify: `internal/cli/evidence.go` (the `record` command)
- Test: `internal/cli/evidence_test.go`

**Interfaces:**
- Consumes: from Task 1 — `app.EvidenceRecordRequest.Owner`, reason `invalid-owner`.
- Produces: CLI flag `--owner build|finalize` on `docket evidence record` (Task 4's skill text
  invokes it).

- [ ] **Step 1: Write the failing tests**

In `TestEvidenceCommandsRegistered`, change the record row to
`{[]string{"evidence", "record"}, []string{"id", "run", "head", "owner", "repo-dir"}},`.

Append:

```go
// TestEvidenceRecordRoutesOwner (change 0517): --owner reaches the request. An
// unknown owner is refused invalid-owner before config is read (so even a
// non-repository directory yields that reason); a valid owner is not.
func TestEvidenceRecordRoutesOwner(t *testing.T) {
	root := testsupport.TempDir(t)
	run := filepath.Join(root, "run")
	out, errS, _ := runCLI(t, "evidence", "record",
		"--id", "7", "--run", run, "--head", "abc", "--owner", "review", "--repo-dir", root, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q", errS)
	}
	if !strings.Contains(out, `"reason":"invalid-owner"`) {
		t.Fatalf("unknown --owner must be refused invalid-owner: %q", out)
	}
	out, _, _ = runCLI(t, "evidence", "record",
		"--id", "7", "--run", run, "--head", "abc", "--owner", "finalize", "--repo-dir", root, "--json")
	if strings.Contains(out, `"reason":"invalid-owner"`) {
		t.Fatalf("--owner finalize must be accepted: %q", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestEvidence' ./internal/cli`
Expected: FAIL — `missing --owner flag`, and `unknown flag: --owner` for the routing test.

- [ ] **Step 3: Implement the flag**

In the `record` command's `RunE`, read the flag and pass it:

```go
			id, _ := c.Flags().GetInt("id")
			run, _ := c.Flags().GetString("run")
			head, _ := c.Flags().GetString("head")
			owner, _ := c.Flags().GetString("owner")
```

```go
			setResult(app.EvidenceRecord(c.Context(), deps, wdeps, repoDir,
				app.EvidenceRecordRequest{ID: id, RunDir: run, Head: head, Owner: owner}))
```

Replace the `--run` flag help and add `--owner`:

```go
	record.Flags().String("run", "", "absolute gate run `dir` to observe (required, except for owner build under build.gate: off, where no run is observed)")
	record.Flags().String("owner", "", "which `role`'s settings certify the run: build (default) or finalize; finalize records finalize.test_command and never mints skipped evidence")
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 -run 'TestEvidence' ./internal/cli`
Expected: PASS.

- [ ] **Step 5: Mutation-test the routing**

```bash
f=internal/cli/evidence.go
cp "$f" "$f.bak"
sed -i '' 's/Head: head, Owner: owner}))/Head: head}))/' "$f"
grep -c 'Head: head}))' "$f"   # expect 1 — the mutation landed (add `_ = owner` if the compiler objects)
go test -count=1 -run 'TestEvidenceRecordRoutesOwner' ./internal/cli
mv -f "$f.bak" "$f"
```

Expected: FAIL (no `invalid-owner` reason). Re-run Step 4 after restore.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/evidence.go internal/cli/evidence_test.go
git commit -m "feat(cli): evidence record --owner build|finalize"
```

---

### Task 4: Skill text, glossary, alignment guard, and embedded assets

**Tier:** standard

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (the paragraph beginning "Then re-gate the
  repaired head through the gate driver", and the sentence "A passing gate records evidence
  through the landed `evidence.record` seam")
- Modify: `skills/docket-finalize-change/references/gate-failure.md` (item 2, "The repair agent")
- Modify: `skills/docket-implement-next/SKILL.md` (under "Create the durable evidence")
- Modify: `docs/reference/glossary.md` (`### Build evidence`)
- Modify: `internal/repoguard/prose_contracts_test.go` (`alignmentContracts`)
- Regenerate: `internal/assets/embedded/tree/**` via `go generate ./internal/assets`

**Interfaces:**
- Consumes: Task 3's `--owner finalize` CLI flag (named in prose).
- Produces: sentinel `align_0517_finalize_evidence_owner`.

- [ ] **Step 1: Write the failing guard**

In `internal/repoguard/prose_contracts_test.go`, add after the last `align_0502_finalize_regate`
row:

```go
	// 0517: the repaired-head re-test certifies with the finalize settings. The
	// evidence.record CALL itself must carry --owner finalize — the 0502 rows'
	// bare "`--owner finalize`" is already satisfied by the gate.drive.start
	// clause, so they cannot catch a dropped evidence flag.
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{"the `evidence.record` operation with `--owner finalize --id <id>"},
		absent:  []string{"the `evidence.record` operation with `--id <id> --run"}},
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-finalize-change/references/gate-failure.md",
		present: []string{"the `evidence.record` operation with `--owner finalize --id <id>"},
		absent:  []string{"the `evidence.record` operation with `--id <id> --run"}},
	// implement-next: the command comes from build configuration, not the run dir.
	{sentinel: "align_0517_finalize_evidence_owner", file: "skills/docket-implement-next/SKILL.md",
		present: []string{"the gate command from the build configuration"},
		absent:  []string{"reads the observed gate command and outcome from the run directory"}},
```

- [ ] **Step 2: Run the guard to verify it fails**

Run: `go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard`
Expected: FAIL — all three files report `[align_0517_finalize_evidence_owner]` (missing present,
found absent).

- [ ] **Step 3: Edit the prose**

`skills/docket-finalize-change/SKILL.md`, in the "Then re-gate the repaired head" paragraph,
replace

```
feeds the `evidence.record` operation with `--id <id> --run <raw run dir from the PASSED document> --head <repaired head>`, which returns the immutable block
```

with

```
feeds the `evidence.record` operation with `--owner finalize --id <id> --run <raw run dir from the PASSED document> --head <repaired head>`, which records `finalize.test_command` (the command that ran) and returns the immutable block
```

In the same file, replace "A passing gate records evidence through the landed `evidence.record`
seam and returns the block in the rebase document" with "A passing gate records evidence through
the landed `evidence.record` seam with the finalize owner, so the block names
`finalize.test_command`, and returns the block in the rebase document".

`skills/docket-finalize-change/references/gate-failure.md`, replace

```
   runs `finalize.test_command` and charges no build attempt. A `PASSED` drive whose head equals
   the repaired head feeds the `evidence.record` operation with `--id <id> --run <raw run dir from
   the PASSED document> --head <repaired head>`; `FAILED` returns to repair within that budget;
```

with (keep `--owner finalize --id <id>` on one line so the Step 5 mutation can match it)

```
   runs `finalize.test_command` and charges no build attempt. A `PASSED` drive whose head equals
   the repaired head feeds the `evidence.record` operation with `--owner finalize --id <id> --run
   <raw run dir from the PASSED document> --head <repaired head>`, which records
   `finalize.test_command`; `FAILED` returns to repair within that budget;
```

`skills/docket-implement-next/SKILL.md`, under "Create the durable evidence", replace "reads the
observed gate command and outcome from the run directory that disposition exposed" with "reads the
outcome from the run directory that disposition exposed and the gate command from the build
configuration (`build.test_command`)".

`docs/reference/glossary.md`, in `### Build evidence`, after the sentence ending "the record says
`skipped` (`build-gate-off`)." add: "When finalize re-tests a head, `evidence record --owner
finalize` records `finalize.test_command` instead, and is never `skipped`." (Current behavior only —
no change numbers.)

- [ ] **Step 4: Regenerate the embedded assets and run the guards**

```bash
go generate ./internal/assets
git status --short internal/assets
go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard
go test -count=1 ./internal/assets
```

Expected: `git status` lists the regenerated copies of the three skill files (and the glossary
only if the generator embeds it); both test commands PASS.

- [ ] **Step 5: Mutation-test the guard, one file at a time**

```bash
for f in skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md; do
  cp "$f" "$f.bak"
  grep -c 'operation with `--owner finalize --id' "$f"   # expect 1
  sed -i '' 's/operation with `--owner finalize --id/operation with `--id/' "$f"
  grep -c 'operation with `--owner finalize --id' "$f"   # expect 0 — the mutation landed
  go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard   # expect FAIL naming $f
  mv -f "$f.bak" "$f"
done
f=skills/docket-implement-next/SKILL.md
cp "$f" "$f.bak"
sed -i '' 's/reads the outcome from the run directory that disposition exposed and the gate command from the build configuration (`build.test_command`)/reads the observed gate command and outcome from the run directory that disposition exposed/' "$f"
go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard   # expect FAIL naming $f
mv -f "$f.bak" "$f"
go test -count=1 -run 'TestAlignmentContracts' ./internal/repoguard   # expect PASS after restore
```

Each mutation must redden `[align_0517_finalize_evidence_owner]` for the mutated file; a green run
means the mutation did not land or the guard is vacuous — fix before committing.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-finalize-change/references/gate-failure.md \
  skills/docket-implement-next/SKILL.md docs/reference/glossary.md \
  internal/repoguard/prose_contracts_test.go internal/assets/embedded
git commit -m "docs(skills): finalize's re-test records evidence with --owner finalize"
```

---

## Final gate (owned by `docket-build`)

After Task 4, `docket-build` runs the whole suite once through the resolved `build.test_command`
(`go run ./cmd/docket development test` in this repo), entered from the feature worktree source.
Read the budget report even when green.
