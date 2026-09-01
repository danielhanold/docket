<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0374 — Separate build and finalize test configuration](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0374-separate-build-and-finalize-test-configuration.md)**
<!-- docket:backlink:end -->
# Separate Build and Finalize Test Configuration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is executed by docket-build profile-routed workers; each task carries a **Build profile** annotation.

**Goal:** Give build and finalize independent `gate`/`test_command` configuration, remove runtime `auto`, add truthful `skipped` build evidence for `build.gate: off`, and move test-command inference to a shared setup-time discovery planner used by init, migrate, and a new `docket repository configure-tests` command.

**Architecture:** The config schema gains a `build.gate`/`build.test_command` pair resolving independently of the finalize pair (neither falls back to the other; legacy `auto` resolves to the empty unconfigured state for both). Gate-drive construction splits into owner-specific constructors so build reads only build policy and finalize only finalize policy, with provenance naming the owning key. The evidence package gains a `skipped` result; publication/implemented gates accept green-or-skipped at exact head, while finalize's suite-skip waiver requires green plus byte-equal command. A pure detector-registry planner in `internal/reposetup` feeds init, migrate, and the new configure-tests operation, all of which leave pending human-reviewed config edits.

**Tech Stack:** Go (spf13/cobra CLI, gopkg.in/yaml.v3 node editing already vendored in `internal/reposetup/configedit.go`), docket's marker-block document engine (`internal/document`), the existing hermetic Go test suite.

**Spec:** `docs/superpowers/specs/2026-08-30-separate-build-and-finalize-test-configuration-design.md` (on the `docket` metadata branch; also readable at `.docket/docs/superpowers/specs/…` from the primary tree). The change file is `docs/changes/active/0374-separate-build-and-finalize-test-configuration.md` on the same branch.

## Global Constraints

- The whole-repo suite command is `go run ./cmd/docket development test` — the build gate runs it once after all tasks (docket-build owns that gate; no plan task runs the whole suite).
- TDD per task: failing test first, then minimal implementation. Run focused package tests with `go test ./internal/<pkg>/ -count=1` — `-count=1` always, so a mutation probe can never be served from the test cache.
- Every guard added or edited must be mutation-tested: strip the guarded thing, watch the assert redden, restore. Record the mutation in the task commit message body if nontrivial.
- Never edit a frozen fixture tree in place. `testdata/repositories/v0.9.*` are immutable inputs; a new upstream state gets a new versioned tree plus `PROVENANCE.md` (Task 12 does exactly this, following the drift guard's own remedy).
- YAML written by generators: the gate value `off` MUST be written as the quoted scalar `"off"` (bare `off` is a YAML boolean keyword). Commands containing `: ` or other indicator characters are quoted. Flow collections are never quoted.
- Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers.
- Historical records (archived changes, accepted ADR bodies, frozen plans/results, `docs/changes/archive/`, `docs/results/`, versioned fixture trees) are never rewritten. `grep` hits inside them are prose, not violations.
- Every remedy message that names the new setup command spells it exactly: `docket repository configure-tests`.
- Spec vocabulary is fixed: evidence results are `green` and `skipped`; the skipped reason is `build-gate-off`; discovery outcomes are `configured`, `detected`, `none`, `ambiguous`.

---

### Task 1: Config schema, Effective struct, and independent resolution

**Build profile:** standard

**Files:**
- Modify: `internal/config/schema.go` (the `// 15: build.` registry section)
- Modify: `internal/config/config.go` (the `Effective` and `Finalize` structs)
- Modify: `internal/config/resolve.go` (the `assign` block and the `autoSentinel` masking)
- Modify: `internal/app/config.go` (the `leafLine` presentation rows)
- Test: `internal/config/resolve_test.go`, `internal/config/schema_test.go`, `internal/app/config_test.go`

**Interfaces:**
- Produces: `config.Build` struct — `type Build struct { Gate Value[string]; TestCommand Value[string] }` reachable as `eff.Build.Gate.Value` (`"local"`|`"off"`, default `"local"`) and `eff.Build.TestCommand.Value` (`""` == unconfigured). Every later task consumes these two exact paths.
- Produces: schema paths `build.gate` (enum `local`,`off`) and `build.test_command`; `finalize.test_command`'s built-in default becomes `""` (empty), with the literal `auto` still legal declared input that resolves to `""` for BOTH commands (legacy migration input, provenance preserved).

- [ ] **Step 1: Write the failing resolution tests**

In `internal/config/resolve_test.go`, add (mirroring the file's existing table style — read a neighboring finalize.test_command test first and reuse its harness helpers):

```go
func TestBuildGateAndCommandResolveIndependently(t *testing.T) {
	// build.test_command set at the repo layer must NOT touch finalize, and
	// vice versa — neither key falls back to the other.
	snap := resolveLayers(t, layers{
		repository: "build:\n  gate: local\n  test_command: go test ./...\nfinalize:\n  test_command: make check\n",
	})
	if got := snap.Effective.Build.TestCommand.Value; got != "go test ./..." {
		t.Errorf("build.test_command = %q, want %q", got, "go test ./...")
	}
	if got := snap.Effective.Finalize.TestCommand.Value; got != "make check" {
		t.Errorf("finalize.test_command = %q, want %q", got, "make check")
	}
	if got := snap.Effective.Build.Gate.Value; got != "local" {
		t.Errorf("build.gate = %q, want local", got)
	}
}

func TestBuildCommandLegacyAutoResolvesUnset(t *testing.T) {
	// `auto` is legacy input for BOTH commands: it resolves to "" (unconfigured),
	// keeps its declaring layer's provenance, and masks a lower layer's explicit
	// command exactly as finalize.test_command's auto already does.
	snap := resolveLayers(t, layers{
		global:     "build:\n  test_command: make global-suite\n",
		repository: "build:\n  test_command: auto\n",
	})
	got := snap.Effective.Build.TestCommand
	if got.Value != "" {
		t.Errorf("build.test_command = %q, want \"\" (auto is the unset sentinel)", got.Value)
	}
	if got.Provenance.Layer != LayerRepository {
		t.Errorf("provenance layer = %v, want the repository layer that declared auto", got.Provenance.Layer)
	}
}

func TestCommandsDefaultUnconfigured(t *testing.T) {
	// Both commands default to the empty unconfigured state; both gates to local.
	snap := resolveLayers(t, layers{})
	if v := snap.Effective.Build.TestCommand.Value; v != "" {
		t.Errorf("default build.test_command = %q, want \"\"", v)
	}
	if v := snap.Effective.Finalize.TestCommand.Value; v != "" {
		t.Errorf("default finalize.test_command = %q, want \"\"", v)
	}
	if v := snap.Effective.Build.Gate.Value; v != "local" {
		t.Errorf("default build.gate = %q, want local", v)
	}
}

func TestBuildGateRejectsUnknownValue(t *testing.T) {
	// build.gate is local|off only — no ci/both (finalize keeps its wider enum).
	assertResolveError(t, layers{repository: "build:\n  gate: ci\n"}, ErrInvalidConfig)
}
```

Adapt helper names (`resolveLayers`, `layers`, `assertResolveError`) to whatever the file actually provides — the assertions above are the contract; the harness is the file's own. Also UPDATE (do not delete) the existing `auto`-sentinel masking matrix tests around the assertion `want \"\" (auto is the unset sentinel)`: they keep their masking semantics and gain build-side twins (learning `shared-resource-keeps-first-owner-assumptions`: the discriminating fixture is the one that sets BOTH pairs divergently — keep `TestBuildGateAndCommandResolveIndependently` as that fixture).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -count=1 -run 'TestBuild|TestCommandsDefault'`
Expected: FAIL — `snap.Effective.Build` undefined.

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add beside `Finalize`:

```go
// Build is the build role's OWN gate policy (change 0374). It resolves
// independently of Finalize: neither command falls back to the other.
type Build struct {
	Gate        Value[string] `json:"gate"`         // local|off
	TestCommand Value[string] `json:"test_command"` // "" == unconfigured (legacy `auto` resolves away)
}
```

and the field `Build Build \`json:"build"\`` in `Effective` (place after `Finalize`). Update the `Finalize.TestCommand` comment to say `// "" == unconfigured (legacy `auto` resolves away)`.

In `internal/config/schema.go`, in the `// 15: build.` section beside `build.checkpoint`:

```go
{path: "build.gate", kind: kindString, enum: []string{"local", "off"}, def: "local",
	merge: mergeScalar, scope: scopeAny, disp: dispSupported,
	validate: enumLeaf("local", "off")},
{path: "build.test_command", kind: kindString, def: "",
	merge: mergeScalar, scope: scopeAny, disp: dispSupported,
	validate: stringLeaf(false, false, false)},
```

Change `finalize.test_command`'s `def: "auto"` to `def: ""`.

In `internal/config/resolve.go`: add `set(assign(&eff.Build.Gate, r.declared, "build.gate"))` and `set(assign(&eff.Build.TestCommand, r.declared, "build.test_command"))` beside the finalize assigns; extend the existing autoSentinel masking block to both commands:

```go
// `auto` is legacy input for BOTH test commands: it spells "unconfigured"
// and never survives resolution (spec: never a valid resolved gate command).
if eff.Finalize.TestCommand.Value == autoSentinel {
	eff.Finalize.TestCommand.Value = ""
}
if eff.Build.TestCommand.Value == autoSentinel {
	eff.Build.TestCommand.Value = ""
}
```

In `internal/app/config.go`, beside the `finalize.test_command` `leafLine` row, add rows for `build.gate` and `build.test_command` following the exact same call shape.

- [ ] **Step 4: Run package tests, fix fallout**

Run: `go test ./internal/config/ ./internal/app/ -count=1`
Expected fallout to fix in the same task: `schema_test.go` row-count/enumeration asserts, `internal/app/config_test.go` presentation goldens, and any decode/fixture test enumerating supported paths. `example_correspondence_test.go` will now fail Direction B (schema key with no example entry) — that is Task 2's deliverable; if it blocks this task's package run, add the example entries in Task 2's format now and fold Task 2's Step 1 verification there (note it in the commit message). Mutation-test one guard: revert the resolve.go build-auto masking line, confirm `TestBuildCommandLegacyAutoResolvesUnset` reddens, restore.

- [ ] **Step 5: Commit**

```bash
git add internal/config internal/app/config.go internal/app/config_test.go
git commit -m "feat(0374): independent build.gate/build.test_command config pair; auto is legacy unset input for both commands"
```

---

### Task 2: Canonical example, embedded twin, and config documentation

**Build profile:** economy

**Files:**
- Modify: `.docket.example.yml` (the `finalize` and `build` blocks)
- Modify: `internal/assets/embedded/tree/.docket.example.yml` (regenerated, never hand-edited)
- Test: `internal/config/example_correspondence_test.go` (existing; must pass), embedded-asset drift tests in `internal/assets`

**Interfaces:**
- Consumes: Task 1's schema keys `build.gate`, `build.test_command`.
- Produces: the canonical example documents both pairs, spells empty commands as quoted empty strings (`""`), and no longer presents `auto` as the supported spelling.

- [ ] **Step 1: Edit `.docket.example.yml`**

In the `finalize:` block, rewrite the `test_command` entry (currently `test_command: auto` with prose saying finalize auto-detects): the new prose leads with what the key does for the reader — the suite finalize's merge gate runs — states that an empty `""` (the default) means unconfigured, that a local gate with no command halts with a typed remedy naming `docket repository configure-tests` (no runtime discovery), and that the legacy spelling `auto` is accepted only as migration input meaning the same as `""`. Set the example value:

```yaml
  test_command: ""
```

In the `build:` block (beside `checkpoint`), add both new keys with the same reader-first register (learning `config-knob-ship-end-to-end`):

```yaml
  # gate — whether docket-build certifies the finished branch with one whole-suite run.
  # local (default): run build.test_command once after all plan tasks; a green run mints
  # exact-head build evidence. "off": this repository declares it has no build test gate —
  # truthful `skipped` evidence is recorded instead of running anything. Quote "off".
  gate: local
  # test_command — the suite the build gate runs. "" (default) is unconfigured: a local
  # build gate refuses to run until you set it (remedy: `docket repository configure-tests`).
  # Independent of finalize.test_command — the two may diverge.
  test_command: ""
```

- [ ] **Step 2: Verify correspondence**

Run: `go test ./internal/config/ -count=1 -run Example`
Expected: PASS both directions (documented→known and known→documented).

- [ ] **Step 3: Regenerate embedded assets**

Run: `go generate ./internal/assets && go test ./internal/assets/ -count=1`
Expected: PASS; the embedded twin now matches. (The generator is `cmd/genassets`; never hand-edit files under `internal/assets/embedded/tree/`.)

- [ ] **Step 4: Commit**

```bash
git add .docket.example.yml internal/assets
git commit -m "docs(0374): document build.gate/build.test_command; empty string replaces auto as the canonical unconfigured spelling"
```

---

### Task 3: Evidence `skipped` result — record, codec, verify

**Build profile:** standard

**Files:**
- Modify: `internal/evidence/record.go`, `internal/evidence/codec.go`, `internal/evidence/verify.go`
- Test: `internal/evidence/record_test.go`, `internal/evidence/codec_test.go`, `internal/evidence/verify_test.go`, `internal/evidence/boundary_test.go`

**Interfaces:**
- Consumes: nothing new (pure package).
- Produces: `evidence.ResultSkipped Result = "skipped"`, `evidence.ReasonBuildGateOff = "build-gate-off"`, `evidence.NewSkippedRecord(head string, ranAt time.Time) (Record, error)`, `Record.Reason string` field, `evidence.VerdictSkipped Verdict = "skipped"` from `Verify`. Green record bytes are byte-identical to today's (legacy blocks parse unchanged).

- [ ] **Step 1: Write the failing tests**

```go
func TestNewSkippedRecord(t *testing.T) {
	rec, err := NewSkippedRecord("AB12"+strings.Repeat("cd", 18), time.Date(2026, 8, 31, 12, 0, 0, 500e6, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Result != ResultSkipped || rec.Reason != ReasonBuildGateOff {
		t.Errorf("result/reason = %q/%q, want skipped/build-gate-off", rec.Result, rec.Reason)
	}
	if rec.Command != "" {
		t.Errorf("a skipped record carries no command, got %q", rec.Command)
	}
	if rec.Head != "ab12"+strings.Repeat("cd", 18) {
		t.Errorf("head not normalized: %q", rec.Head)
	}
}

func TestSkippedRoundTrip(t *testing.T) {
	rec, _ := NewSkippedRecord(strings.Repeat("ab", 20), time.Now())
	block := Render(rec)
	if strings.Contains(block, "command") {
		t.Errorf("skipped block must carry no command line:\n%s", block)
	}
	if !strings.Contains(block, "reason:") || !strings.Contains(block, "build-gate-off") {
		t.Errorf("skipped block must carry the reason line:\n%s", block)
	}
	got, err := Extract([]byte(block))
	if err != nil || got != rec {
		t.Errorf("round trip: got %+v err %v, want %+v", got, err, rec)
	}
}

func TestVerifySkipped(t *testing.T) {
	head := strings.Repeat("ab", 20)
	rec, _ := NewSkippedRecord(head, time.Now())
	if v := Verify([]byte(Render(rec)), head); v != VerdictSkipped {
		t.Errorf("exact-head skipped = %v, want VerdictSkipped", v)
	}
	if v := Verify([]byte(Render(rec)), strings.Repeat("cd", 20)); v != VerdictStale {
		t.Errorf("wrong-head skipped = %v, want VerdictStale", v)
	}
}

func TestGreenRenderUnchanged(t *testing.T) {
	// Legacy compatibility pin: a green record's rendered bytes carry exactly
	// command/result/head_sha/ran_at — no reason line.
	rec, _ := NewRecord("go test ./...", strings.Repeat("ab", 20), time.Now())
	if strings.Contains(Render(rec), "reason") {
		t.Errorf("green block grew a reason line:\n%s", Render(rec))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/evidence/ -count=1 -run 'Skipped|GreenRender'`
Expected: FAIL — `NewSkippedRecord` undefined.

- [ ] **Step 3: Implement**

`record.go`: add `ResultSkipped Result = "skipped"`, `const ReasonBuildGateOff = "build-gate-off"`, field `Reason string` on `Record` (empty for green), and:

```go
// NewSkippedRecord returns the truthful record for an explicitly disabled
// build gate (build.gate: off). It carries no command — the gate ran nothing —
// and the fixed reason build-gate-off. Only explicit gate-off policy reaches
// this constructor: an unconfigured command, launch failure, or ambiguous
// verdict produces no record of any kind.
func NewSkippedRecord(head string, ranAt time.Time) (Record, error) {
	h := strings.ToLower(head)
	if !validHead(h) {
		return Record{}, fmt.Errorf("evidence: head is not a full lowercase 40- or 64-hex object ID: %q", head)
	}
	return Record{Result: ResultSkipped, Reason: ReasonBuildGateOff, Head: h, RanAt: ranAt.UTC().Truncate(time.Second)}, nil
}
```

Update the package comment and the `Result` doc (green is no longer "the only value"; skipped means the repository explicitly set `build.gate: off`).

`codec.go`: add `keyReason = "reason"`. `interior` branches: green renders `command/result/head_sha/ran_at` exactly as today (byte-identical); skipped renders `result/reason/head_sha/ran_at`. `Extract`'s interior parser: accept both field sets keyed on the `result` value — a `result: skipped` block requires `reason: build-gate-off` and forbids `command`; a `result: green` block keeps today's exact requirements. Any other result value or field mix stays a parse error (malformed).

`verify.go`: add `VerdictSkipped Verdict = "skipped"`; in `Verify`, after extraction, branch on `record.Result`: green + head match → `VerdictVerified`; skipped + head match → `VerdictSkipped`; head mismatch → `VerdictStale` either way. Update the doc comment ("green" claims).

- [ ] **Step 4: Run the package, mutation-test**

Run: `go test ./internal/evidence/ -count=1`
Expected: PASS (fix any boundary/upsert test enumerating field names). Mutations: (a) make `interior` emit a `command` line for skipped → `TestSkippedRoundTrip` reddens; (b) make `Verify` return `VerdictVerified` for skipped → `TestVerifySkipped` reddens. Restore.

- [ ] **Step 5: Commit**

```bash
git add internal/evidence
git commit -m "feat(0374): evidence gains the skipped result (build-gate-off) with green bytes unchanged"
```

---

### Task 4: Owner-split gate-drive construction and CLI routing

**Build profile:** premium (named risk: replaces the `gate drive start -- <argv>` public CLI surface and rewires the finalize gate path; a wrong provenance or command source silently certifies the wrong policy)

**Files:**
- Modify: `internal/app/gate_drive.go` (constructor split + provenance)
- Modify: `internal/app/finalize_rebase.go` (`buildDriveService`)
- Modify: `internal/cli/gate.go` (`start` command, `buildGateDriveService`, `gateDriveEffective`)
- Test: `internal/app/gate_drive_test.go`, `internal/cli/gate_test.go`, `cmd/docket/gate_cli_test.go`

**Interfaces:**
- Consumes: Task 1's `eff.Build.Gate/TestCommand`, `eff.Finalize.TestCommand`.
- Produces: `app.NewBuildGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string)` and `app.NewFinalizeGateDriveService(...)` (same signature) — each reads ONLY its own `test_command`; persisted provenance strings become `gate_observation_budget=<layer>;build.test_command=<layer>` / `…;finalize.test_command=<layer>`. CLI: `docket gate drive start` gains a required `--owner build|finalize` flag and DROPS the `-- <argv>` suite-command surface entirely; the command now resolves authoritative pinned config. Task 13 updates the skill texts that invoke it.

- [ ] **Step 1: Write the failing app-layer tests**

In `internal/app/gate_drive_test.go` (reuse the file's fake-engine harness):

```go
func TestOwnerConstructorsReadOnlyTheirOwnCommand(t *testing.T) {
	eff := config.Effective{}
	eff.GateObservation = config.Value[int]{Value: 5}
	eff.Build.TestCommand = config.Value[string]{Value: "go test ./build-only",
		Provenance: config.Provenance{Layer: config.LayerRepository}}
	eff.Finalize.TestCommand = config.Value[string]{Value: "make finalize-only",
		Provenance: config.Provenance{Layer: config.LayerGlobal}}

	b, _, _ := NewBuildGateDriveService(t.TempDir(), "/bin/true", eff)
	if b.command != "go test ./build-only" {
		t.Errorf("build service command = %q; it must read only build.test_command", b.command)
	}
	if want := "gate_observation_budget=repository;build.test_command=repository"; !strings.HasSuffix(b.provenance, "build.test_command=repository") {
		t.Errorf("build provenance = %q, want it to name build.test_command (e.g. %q)", b.provenance, want)
	}

	f, _, _ := NewFinalizeGateDriveService(t.TempDir(), "/bin/true", eff)
	if f.command != "make finalize-only" {
		t.Errorf("finalize service command = %q; it must read only finalize.test_command", f.command)
	}
	if !strings.Contains(f.provenance, "finalize.test_command=") {
		t.Errorf("finalize provenance = %q, want it to name finalize.test_command", f.provenance)
	}
}
```

Fix the `GateObservation` provenance in the fixture so the exact expected strings assert cleanly (set its `Provenance.Layer` explicitly). This is the divergent-command fixture the spec's Testing section requires: the two commands DIFFER, so a service reading the wrong key cannot pass.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -count=1 -run TestOwnerConstructors`
Expected: FAIL — constructors undefined.

- [ ] **Step 3: Implement the app layer**

In `gate_drive.go`: replace `NewGateDriveService` with the two owner constructors sharing one private core:

```go
func NewBuildGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string) {
	return newOwnedGateDriveService(gitCommonDir, exePath, eff,
		eff.Build.TestCommand, "build.test_command")
}

func NewFinalizeGateDriveService(gitCommonDir, exePath string, eff config.Effective) (*GateDriveService, Result, string) {
	return newOwnedGateDriveService(gitCommonDir, exePath, eff,
		eff.Finalize.TestCommand, "finalize.test_command")
}

func newOwnedGateDriveService(gitCommonDir, exePath string, eff config.Effective,
	command config.Value[string], owningPath string) (*GateDriveService, Result, string) {
	proc, err := process.NewService(exePath)
	if err != nil {
		r, reason := mapGateFailure(err)
		return nil, r, reason
	}
	store := gatedrive.OpenStore(gitCommonDir)
	engine := gatedrive.NewSystemDriver(store, proc)
	budget := time.Duration(eff.GateObservation.Value) * time.Minute
	prov := fmt.Sprintf("gate_observation_budget=%s;%s=%s",
		eff.GateObservation.Provenance.Layer, owningPath, command.Provenance.Layer)
	return newGateDriveService(engine, budget, command.Value, prov), "", ""
}
```

Delete `gateDriveProvenance` (folded in above). Update the package/constructor comments: command selection is an explicit domain boundary; no caller may substitute a command around authoritative configuration (quote the spec clause "no agent or CLI caller may substitute an arbitrary command around authoritative configuration"). Grep the repo for `NewGateDriveService` and update every executable caller: `finalize_rebase.go`'s `buildDriveService` calls `NewFinalizeGateDriveService` (and its guard `pin.Config.Effective.Finalize.TestCommand.Value == ""` stays finalize-owned).

- [ ] **Step 4: Rework the CLI `start` command**

In `internal/cli/gate.go`:
- Delete the `-- <argv>` handling (the `ArgsLenAtDash` block), `gateDriveEffective`, and the fixed `gateDriveBudgetMinutes` wiring from `start`'s path.
- Add `start.Flags().String("owner", "", "which policy owns this drive: build or finalize (required)")` + `MarkFlagRequired("owner")`; reject any value other than `build`/`finalize` as a command error.
- `start` resolves authoritative pinned config the way `internal/cli/evidence.go` builds its app deps (mirror that file's deps construction and `PinContext`-backed resolution — same reader, same error surface), then calls `app.NewBuildGateDriveService` or `app.NewFinalizeGateDriveService` on `repo.CommonDir`. `advance`/`handoff`/`claim` never consult the command: keep composing their service without a command (a small `newCommandlessGateDriveService`-style path or either owner constructor over an `Effective` whose command is empty — pick the shape that keeps `advance` config-resolution-free, matching its current behavior, and document why).
- An empty resolved command for the requested owner still surfaces through `Start`'s existing `unresolved-command` refusal; extend that refusal's human message to name the remedy: `"no resolved <owner> test command; run docket repository configure-tests"` (keep the `unresolved-command` reason token stable).
- Update `use`/`short` strings: `start --repo-dir <dir> --run-root <dir> --owner build|finalize`.

- [ ] **Step 5: Run and fix CLI tests**

Run: `go test ./internal/cli/ ./cmd/docket/ ./internal/app/ -count=1`
Expected fallout: `gate_test.go` / `gate_cli_test.go` fixtures that pass `-- argv` — rewrite them to `--owner` + a config fixture declaring the command. Add one CLI test asserting `--owner build` with a repo whose `.docket.yml` sets divergent commands launches the build one (assert via the fake/recorded start request, mirroring existing test seams). Mutation: swap the owner constructor called for `--owner build` to the finalize one → the divergent-command CLI test reddens; restore.

- [ ] **Step 6: Commit**

```bash
git add internal/app internal/cli cmd/docket
git commit -m "feat(0374): owner-split gate-drive construction; gate drive start takes --owner and resolves authoritative config"
```

---

### Task 5: Build-owned evidence operation with skipped minting

**Build profile:** standard

**Files:**
- Modify: `internal/app/evidence_ops.go` (`EvidenceRecord`, refusal reasons, `HumanText`)
- Modify: `internal/cli/evidence.go` (flag requiredness if `--run-dir` is currently mandatory)
- Test: `internal/app/evidence_ops_test.go`, `internal/cli/evidence_test.go`

**Interfaces:**
- Consumes: Task 1's `eff.Build.Gate/TestCommand`; Task 3's `evidence.NewSkippedRecord`.
- Produces: `EvidenceRecord` resolves BUILD config only. `build.gate: off` → applied result carrying the rendered skipped block (no run observation, no RunDir needed). `build.gate: local` + empty `build.test_command` → `ResultUnsupportedConfig`/`ReasonEvidenceUnconfiguredGate` with a message naming `docket repository configure-tests`. `build.gate: local` + command → today's passed-run flow recording `build.test_command`.

- [ ] **Step 1: Write the failing tests**

In `internal/app/evidence_ops_test.go` (reuse its deps/fixture harness):

```go
func TestEvidenceRecordBuildGateOffMintsSkipped(t *testing.T) {
	// build.gate: off — no run is observed (RunDir empty), the head must still be
	// the current feature head, and the block is the truthful skipped record.
	res := evidenceRecordWithConfig(t, "build:\n  gate: \"off\"\n", EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %v (%s), want applied", res.Result, res.Reason)
	}
	if res.Outcome != "skipped" || !strings.Contains(res.Block, "build-gate-off") {
		t.Errorf("outcome/block = %q/%q, want skipped/build-gate-off", res.Outcome, res.Block)
	}
}

func TestEvidenceRecordUnconfiguredBuildCommandIsTypedSetupRefusal(t *testing.T) {
	res := evidenceRecordWithConfig(t, "build:\n  gate: local\n", EvidenceRecordRequest{ID: 7, Head: currentFeatureHead(t), RunDir: t.TempDir()})
	if res.Result != ResultUnsupportedConfig || res.Reason != ReasonEvidenceUnconfiguredGate {
		t.Fatalf("result/reason = %v/%s, want unsupported-config/%s", res.Result, res.Reason, ReasonEvidenceUnconfiguredGate)
	}
	if !strings.Contains(res.Message, "docket repository configure-tests") {
		t.Errorf("message %q must name the setup remedy", res.Message)
	}
}

func TestEvidenceRecordRecordsBuildCommandNotFinalize(t *testing.T) {
	// Divergent-command fixture: the recorded command is build.test_command.
	res := evidenceRecordPassedRun(t, "build:\n  gate: local\n  test_command: go test ./build-only\nfinalize:\n  test_command: make finalize-only\n")
	if res.Command != "go test ./build-only" {
		t.Errorf("recorded command = %q; evidence must record build.test_command", res.Command)
	}
}
```

Build the two helpers (`evidenceRecordWithConfig`, `evidenceRecordPassedRun`) on the file's existing fixture machinery — the passed-run helper reuses however the current tests fake `svc.Observe` returning `process.StatePassed`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -count=1 -run TestEvidenceRecord`
Expected: FAIL — gate-off path missing; recorded command is finalize's.

- [ ] **Step 3: Implement**

Reorder `EvidenceRecord`: pin config FIRST (step 2's `PinContext` moves ahead of observation). Then:

```go
build := pin.Config.Effective.Build
if build.Gate.Value == "off" {
	// Explicit policy: no build test gate. Mint truthful skipped evidence at the
	// verified current feature head; observe no run.
	// (head-equals-current-feature-head check identical to the green path)
	rec, err := evidence.NewSkippedRecord(req.Head, deps.Clock.Now())
	...
}
command := build.TestCommand.Value
if command == "" {
	return newEvidenceRefusal(OperationEvidenceRecord, ResultUnsupportedConfig, ReasonEvidenceUnconfiguredGate,
		"build.gate is local but build.test_command is unconfigured; run `docket repository configure-tests` and review the pending edit", req.ID)
}
// then today's observe-the-run flow, recording `command`
```

The gate-off branch performs the same `WorkspaceInspect` head equality check as the green path (share it — extract a small helper rather than duplicating the predicate; learning `duplicated-gate-copies-the-whole-predicate`). Require `RunDir` non-empty only on the local-gate path (typed invalid-input if missing there). Update the operation's doc comment: it validates against BUILD configuration and never re-resolves `finalize.test_command` (quote that spec clause). Extend `HumanText` so a skipped applied result renders `recorded skipped at <head> (build-gate-off)` — branch on empty `r.Command`. Check `internal/cli/evidence.go`: if `--run-dir` is `MarkFlagRequired`, drop the mark and let the app layer enforce it conditionally; update the flag help text.

- [ ] **Step 4: Run and mutation-test**

Run: `go test ./internal/app/ ./internal/cli/ -count=1`
Mutation: change the recorded command source back to `pin.Config.Effective.Finalize.TestCommand.Value` → `TestEvidenceRecordRecordsBuildCommandNotFinalize` reddens (this is the guard the divergent fixture exists for). Restore.

- [ ] **Step 5: Commit**

```bash
git add internal/app internal/cli
git commit -m "feat(0374): evidence record is build-owned — skipped minting for gate off, typed setup refusal for unconfigured command"
```

---

### Task 6: Verify-caller split — green-or-skipped publication vs green-plus-command finalize waiver

**Build profile:** premium (named risk: this predicate decides when a merge skips its test run; a wrong acceptance silently waives finalize)

**Files:**
- Modify: `internal/app/finalize_rebase.go` (`prBodyEvidence`, `gateDecision`, its caller near `evidenceHead, evidenceGreen := prBodyEvidence(pr)`)
- Modify: `internal/app/pr_publish.go`, `internal/app/change_implemented.go`, `internal/app/finalize_publish.go`, `internal/app/run_verify.go` (the four `evidence.Verify` sites)
- Test: `internal/app/finalize_rebase_test.go`, `internal/app/pr_publish_test.go`, `internal/app/change_implemented_test.go`, `internal/app/finalize_publish_test.go`, `internal/app/run_verify_test.go`

**Interfaces:**
- Consumes: Task 3's `VerdictSkipped`, `Record.Result`, `Record.Command`; Task 1's `eff.Finalize.TestCommand`.
- Produces: `gateDecision(noop bool, evidenceHead, currentHead string, evidenceGreen bool, evidenceCommand, resolvedCommand string) (skip bool, permit string)` — skip only when noop AND green AND exact head AND `evidenceCommand == resolvedCommand` (byte equality) AND both non-empty. Publication/implemented/run-verify sites accept `VerdictVerified` OR `VerdictSkipped`.

- [ ] **Step 1: Write the failing waiver tests**

```go
func TestGateDecisionRequiresCommandByteEquality(t *testing.T) {
	head := strings.Repeat("ab", 20)
	cases := []struct {
		name                     string
		green                    bool
		evCmd, resolvedCmd       string
		wantSkip                 bool
	}{
		{"green same command skips", true, "go test ./...", "go test ./...", true},
		{"green differing command runs the suite", true, "go test ./...", "make check", false},
		{"green empty resolved command runs (never a vacuous match)", true, "", "", false},
		{"skipped evidence never waives finalize", false, "", "go test ./...", false},
	}
	for _, c := range cases {
		skip, permit := gateDecision(true, head, head, c.green, c.evCmd, c.resolvedCmd)
		if skip != c.wantSkip {
			t.Errorf("%s: skip = %v, want %v", c.name, skip, c.wantSkip)
		}
		if skip && permit != head {
			t.Errorf("%s: permit = %q, want the head", c.name, permit)
		}
	}
}

func TestPRPublishAcceptsSkippedEvidenceAtExactHead(t *testing.T) {
	// build.gate: off repositories publish PRs on truthful skipped evidence.
	// (fixture: render evidence.NewSkippedRecord at the feature head, run PRPublish,
	// assert the operation proceeds past the evidence conjunct.)
}
```

Write the second test concretely against the file's existing publish fixture (mirror the neighboring green-evidence fixture, substituting a rendered skipped block); add the same-shape twins for `ChangeImplemented`, `FinalizePublish`, and `RunVerify` (the facade's PR-body check accepts a skipped block at the exact head). Add a negative: `FinalizeRebase`'s suite-skip with skipped PR-body evidence still runs the suite.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -count=1 -run 'GateDecision|AcceptsSkipped'`
Expected: FAIL — `gateDecision` has the old arity; skipped evidence is refused everywhere.

- [ ] **Step 3: Implement**

`finalize_rebase.go`:

```go
func prBodyEvidence(pr githubcli.PullRequest) (evidenceHead, evidenceCommand string, green bool) {
	if pr.Body == "" {
		return "", "", false
	}
	rec, err := evidence.Extract([]byte(pr.Body))
	if err != nil {
		return "", "", false
	}
	return rec.Head, rec.Command, rec.Result == evidence.ResultGreen
}

// gateDecision skips ONLY when the rebase was a no-op AND the PR body carries
// GREEN evidence for the EXACT current head AND the recorded command is
// byte-equal to the currently resolved finalize.test_command. Differing
// commands are different assertions, even at the same SHA; skipped build
// evidence never waives finalize's local gate.
func gateDecision(noop bool, evidenceHead, currentHead string, evidenceGreen bool,
	evidenceCommand, resolvedCommand string) (skip bool, permit string) {
	if noop && evidenceGreen && evidenceHead != "" && evidenceHead == currentHead &&
		evidenceCommand != "" && evidenceCommand == resolvedCommand {
		return true, evidenceHead
	}
	return false, ""
}
```

The caller threads `pin.Config.Effective.Finalize.TestCommand.Value` (already pinned on that path — follow how `buildDriveService` pins it) into `gateDecision`. Update the file-top comment (near the `carries green evidence for the EXACT current head` clause) and the `gateComposeSkipped` doc string to name the command condition.

The four `Verify` sites change their acceptance from `verdict != evidence.VerdictVerified` to rejecting only when the verdict is neither `VerdictVerified` nor `VerdictSkipped`; update each site's refusal message ("does not verify green" → "does not verify (green or skipped) against the requested head"). Keep each site's reason token unchanged.

- [ ] **Step 4: Run and mutation-test**

Run: `go test ./internal/app/ -count=1`
Mutations (each must redden the named test, then restore): (a) drop the `evidenceCommand == resolvedCommand` conjunct → `TestGateDecisionRequiresCommandByteEquality`; (b) drop `evidenceCommand != ""` → the vacuous-match case (learning `identity-match-relaxed-to-prefix-is-vacuous`: empty-vs-empty must not skip); (c) let `prBodyEvidence` report `green` for a skipped record → the skipped-never-waives case.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(0374): finalize suite-skip requires green + byte-equal command; publication gates accept green-or-skipped at exact head"
```

---

### Task 7: Workflow contexts expose owned gate policy

**Build profile:** standard

**Files:**
- Modify: `internal/app/implementation_context.go` (`ContextWorkflow`)
- Modify: `internal/app/finalize_context.go` (the context struct field near `TestCommand string \`json:"test_command,omitempty"\`` and its fill site `TestCommand: eff.Finalize.TestCommand.Value`)
- Modify: `internal/app/repository_prepare.go` (`PrepareFinalize` sibling + fill site)
- Test: `internal/app/implementation_context_test.go`, `internal/app/finalize_context_test.go`, `internal/app/repository_prepare` tests (in `internal/app`)

**Interfaces:**
- Consumes: Task 1's `eff.Build.*`.
- Produces: `ContextWorkflow` (implementation context) RETIRES `TestCommand` and gains `BuildGate string \`json:"build_gate"\`` + `BuildTestCommand string \`json:"build_test_command,omitempty"\``. Finalize context renames its field to `FinalizeGate`/`FinalizeTestCommand` with JSON keys `finalize_gate`/`finalize_test_command` (generic `test_command` names are retired per spec Touch point 3). `PrepareContext` gains `Build PrepareBuild` with `type PrepareBuild struct { Gate string \`json:"gate"\`; TestCommand string \`json:"test_command"\` }` (inside the ownership-named `build` block, so the leaf name stays). Task 13 updates every skill that reads these JSON keys.

- [ ] **Step 1: Write the failing tests**

Extend the existing context tests: assert the implementation context JSON carries `"build_gate":"local"` and `"build_test_command"` from a fixture whose config sets divergent build/finalize commands, and does NOT carry a bare `"test_command"` key in `workflow`; assert the finalize context carries `finalize_gate`/`finalize_test_command` (finalize's values, not build's). Use one divergent-command fixture for both (the discriminating fixture).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -count=1 -run 'Context'`
Expected: FAIL.

- [ ] **Step 3: Implement**

`implementation_context.go` `ContextWorkflow`: replace `TestCommand string \`json:"test_command,omitempty"\`` with:

```go
BuildGate        string `json:"build_gate"`                   // local|off — the build role's own gate policy
BuildTestCommand string `json:"build_test_command,omitempty"` // "" == unconfigured (typed setup halt at the gate)
```

Fill from `eff.Build.Gate.Value` / `eff.Build.TestCommand.Value`. `finalize_context.go`: rename the field and JSON key to `FinalizeGate`(add, from `eff.Finalize.Gate.Value`)/`FinalizeTestCommand`; keep `omitempty` on the command. `repository_prepare.go`: add `PrepareBuild` mirroring `PrepareFinalize`'s exactly-the-supported-fields comment discipline, fill both `Gate` and `TestCommand` from `cfg.Build.*`.

- [ ] **Step 4: Run, chase consumers**

Run: `go test ./internal/app/ ./internal/cli/ -count=1`
Then grep the whole repo for `"test_command"` and `TestCommand` consumers of these context JSONs outside historical records (skills read them — inventory the hits for Task 13; fix any executable Go/test consumer now). Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app
git commit -m "feat(0374): contexts expose owned gate policy — build_gate/build_test_command and finalize_gate/finalize_test_command"
```

---

### Task 8: The pure test-discovery planner

**Build profile:** standard

**Files:**
- Create: `internal/reposetup/testdiscovery.go`
- Test: `internal/reposetup/testdiscovery_test.go`

**Interfaces:**
- Consumes: nothing outside the standard library (pure, side-effect-free).
- Produces:

```go
type DiscoveryKind string
const (
	DiscoveryConfigured DiscoveryKind = "configured"
	DiscoveryDetected   DiscoveryKind = "detected"
	DiscoveryNone       DiscoveryKind = "none"
	DiscoveryAmbiguous  DiscoveryKind = "ambiguous"
)

// TestTree is the pinned-tree read seam. Init reads the primary worktree;
// migrate reads the pinned git tree; tests use a map. Every method returns an
// error ONLY for a probe failure — absence is (false/nil, nil), never an error
// (learning probe-error-is-not-clean-absence: a probe error aborts discovery
// as unknown; it is never folded into "none" and never into "off").
type TestTree interface {
	Exists(path string) (bool, error)
	ReadFile(path string) ([]byte, error) // fs.ErrNotExist for absent
	Glob(pattern string) ([]string, error) // path.Match semantics, repo-root-relative
}

type DetectedSuite struct {
	Family   string // stable token: makefile|go|node|pytest|rust|shell
	Command  string // the exact command this family certifies
	Evidence string // one human sentence naming the files that matched
}

type DiscoveryOutcome struct {
	Kind       DiscoveryKind
	Command    string          // set for detected; the single suite command
	Candidates []DetectedSuite // detected: len 1; ambiguous: all matches, family order
}

// DiscoverTests: declared build/finalize commands that are explicit and
// non-legacy ("" and "auto" are unconfigured) yield configured without
// probing. Otherwise every registered detector runs; one match is detected,
// zero is none, two or more is ambiguous (no priority list guesses).
func DiscoverTests(tree TestTree, declaredBuildCommand, declaredFinalizeCommand string) (DiscoveryOutcome, error)
```

- The detector registry (a package-level ordered slice — the single owning place for supported shapes) covers exactly, in this order: **makefile** — `Makefile` or `makefile` containing a column-0 `test:` target line → `make test`; **go** — root `go.mod` AND `Glob("*_test.go")` or `Glob("*/*_test.go")` (probe both levels; a Go module with only deeper tests still matches via `Glob("**")` being unsupported — implement the two-level glob and state that bound in the detector's doc comment) → `go test ./...`; **node** — `package.json` whose parsed `scripts.test` is non-empty and not the npm placeholder (contains `"no test specified"`) AND exactly one recognized lockfile among `package-lock.json`→`npm test`, `yarn.lock`→`yarn test`, `pnpm-lock.yaml`→`pnpm test` (zero or two-plus lockfiles → this detector reports NO match — an unrecognizable launcher is not a detected suite; note this in `Evidence` only when the scripts.test existed); **pytest** — (`pytest.ini` exists, or `pyproject.toml` contains `[tool.pytest.ini_options]`, or `setup.cfg` contains `[tool:pytest]`) AND (`Glob("test_*.py")` or `Glob("tests/test_*.py")` or `Glob("*/test_*.py")` non-empty) → `pytest`; **rust** — root `Cargo.toml` → `cargo test`; **shell** — `Glob("tests/test_*.sh")` non-empty → `bash -c 'set -e; for t in tests/test_*.sh; do bash "$t"; done'`.

- [ ] **Step 1: Write the failing tests**

Build a `mapTree` test double implementing `TestTree` over a `map[string]string`, plus an `errTree` whose probes fail. Cover: configured (explicit commands, no probe runs — prove it with a tree double that panics on any call), legacy `auto` declared → discovery runs, one family (each of the six, with its exact command asserted verbatim), zero families → none, go+rust together → ambiguous with both candidates, node with two lockfiles → that detector silent (rust-less tree → none), pytest config without test files → silent, makefile without a `test:` target → silent, probe error → `(DiscoveryOutcome{}, err)` with a non-nil error and NO outcome kind.

```go
func TestDiscoverAmbiguousListsAllCandidates(t *testing.T) {
	tree := mapTree{"go.mod": "module x", "x_test.go": "", "Cargo.toml": "[package]"}
	out, err := DiscoverTests(tree, "", "")
	if err != nil || out.Kind != DiscoveryAmbiguous {
		t.Fatalf("kind = %v err %v, want ambiguous", out.Kind, err)
	}
	if len(out.Candidates) != 2 || out.Candidates[0].Family != "go" || out.Candidates[1].Family != "rust" {
		t.Errorf("candidates = %+v, want [go rust]", out.Candidates)
	}
	if out.Command != "" {
		t.Errorf("ambiguous must carry no command, got %q", out.Command)
	}
}

func TestProbeErrorIsUnknownNeverNone(t *testing.T) {
	_, err := DiscoverTests(errTree{}, "", "")
	if err == nil {
		t.Fatal("a probe error must surface as an error, never a clean none")
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./internal/reposetup/ -count=1 -run Discover` → FAIL (undefined).

- [ ] **Step 3: Implement** the registry and `DiscoverTests` per the interface block above. Deterministic: family order is the registry slice order; no map iteration reaches output. Each detector is a `func(TestTree) (*DetectedSuite, error)` returning nil for no-match, error for probe failure (abort the whole discovery).

- [ ] **Step 4: Run** — `go test ./internal/reposetup/ -count=1` → PASS. Mutation: make the node detector treat two lockfiles as npm → its silence test reddens; restore.

- [ ] **Step 5: Commit**

```bash
git add internal/reposetup/testdiscovery.go internal/reposetup/testdiscovery_test.go
git commit -m "feat(0374): pure deterministic test-discovery planner with a closed detector registry"
```

---

### Task 9: Config-edit renderer for generated test policy

**Build profile:** standard

**Files:**
- Create: `internal/reposetup/testconfigedit.go`
- Test: `internal/reposetup/testconfigedit_test.go`

**Interfaces:**
- Consumes: Task 8's `DiscoveryOutcome`.
- Produces:

```go
// RenderTestConfigEdit produces the pending `.docket.yml` bytes for a
// discovery outcome. existing == nil means no file exists (fresh init renders
// a minimal file). It returns changed == false when the outcome requires no
// edit (configured, ambiguous, or the file already carries these exact
// settings — the idempotency case). It never writes: callers own the pending,
// unstaged write. Gate "off" is always the QUOTED scalar "off".
func RenderTestConfigEdit(existing []byte, out DiscoveryOutcome) (edited []byte, changed bool, err error)
```

Semantics: `detected` → ensure `build:` and `finalize:` mappings each carry `gate: local` and `test_command: <command>` (commands identical initially but written under both keys — separate settings). `none` → `gate: "off"` under both, and NO `test_command` key written (no fake command). `configured`/`ambiguous` → `(existing, false, nil)`. Existing unrelated keys, comments, and ordering are preserved: extend the yaml.Node splice machinery in `internal/reposetup/configedit.go` (`topLevelMapping`, `spliceOutLines` — follow `RemoveMetadataBranchKey`'s source-preserving approach; append missing blocks at EOF, splice-replace an existing `test_command`/`gate` line in place). Malformed existing YAML → error, file untouched (never a destructive rewrite).

- [ ] **Step 1: Write the failing tests** — table-driven over: nil existing + detected (golden minimal file, both blocks, exact bytes asserted), nil + none (both `gate: "off"` quoted — assert the literal `"off"` with quotes in the bytes), existing file with unrelated keys + detected (unrelated keys and their comments byte-preserved; assert via prefix/contains on the surviving comment lines), idempotency (`RenderTestConfigEdit(edited, sameOutcome)` → `changed == false` and byte-identical), existing explicit new-style settings + detected (`changed == false` — preserved), ambiguous (`changed == false`), malformed YAML → error.

- [ ] **Step 2: Run to verify failure** — `go test ./internal/reposetup/ -count=1 -run TestConfigEdit` → FAIL.

- [ ] **Step 3: Implement** per the contract above.

- [ ] **Step 4: Run and mutation-test** — mutation: write bare `off` instead of `"off"` → the quoted-scalar assert reddens (this guard enforces the AGENTS.md YAML-boolean rule); second mutation: make the idempotent re-run return `changed == true` → idempotency test reddens. Restore both.

- [ ] **Step 5: Commit**

```bash
git add internal/reposetup/testconfigedit.go internal/reposetup/testconfigedit_test.go
git commit -m "feat(0374): source-preserving pending-config renderer for discovered test policy"
```

---

### Task 10: Init and migrate integration

**Build profile:** premium (named risk: migrate mutates a remote through a previewed two-pass transaction; a preview/execution byte divergence violates decide-and-act-on-the-same-copy)

**Files:**
- Modify: `internal/reposetup/initplan.go` (+`internal/app`'s `RunRepositoryInit` execution — locate via `repositoryInitRunner` in `internal/cli/repository.go`)
- Modify: `internal/reposetup/migrateplan.go` + the migrate preview/execution path (`RunRepositoryMigrate`, the two-pass confirm flow in `internal/cli/repository.go`)
- Test: `internal/reposetup/initplan_test.go`, `internal/reposetup/migrateplan_test.go`, the `internal/app` repository-setup tests

**Interfaces:**
- Consumes: Tasks 8–9 (`DiscoverTests`, `RenderTestConfigEdit`); the existing pending-path reporting that already covers `.gitignore` (init leaves generated files uncommitted/unstaged; repository stays `needs-review`).
- Produces: `InitPlan` gains `DocketYMLPath string` and `DocketYMLBytes []byte` (nil when no test-policy write applies) plus `TestDiscovery DiscoveryOutcome`; `MigrationPlan` gains `ConfigBytes []byte` (the exact authorized bytes, nil when unchanged) and `TestDiscovery DiscoveryOutcome`.

- [ ] **Step 1: Write the failing plan tests** — extend `initplan_test.go`: PlanInit over a facts fixture whose tree has one Go suite yields `DocketYMLBytes` containing both `test_command: go test ./...` lines and `TestDiscovery.Kind == DiscoveryDetected`; a no-suite tree yields both `gate: "off"`; an ambiguous tree yields nil bytes + the candidates. Extend `migrateplan_test.go`: a legacy tree with explicit `finalize.test_command: make check` yields `ConfigBytes` where `build.test_command` is `make check` too (preserve + copy rule) and finalize's unchanged; a legacy `test_command: auto` tree runs discovery; ambiguity yields an error/refusal BEFORE any receipt is composed (assert PlanMigration returns the typed refusal and no plan).

- [ ] **Step 2: Run to verify failure** — `go test ./internal/reposetup/ -count=1` → FAIL (plan structs lack the fields).

- [ ] **Step 3: Implement the plans** — thread a `TestTree` argument into `PlanInit`/`PlanMigration` (init's tree reads the primary worktree at the same pinned clean state the rest of init uses; migrate's reads the pinned source revision's git tree — the app layer supplies each, implementing `TestTree` over its existing file access; keep the planners pure by taking the interface). Migrate's preserve/copy rule runs BEFORE discovery: an explicit non-`auto` legacy `finalize.test_command` short-circuits to a `RenderTestConfigEdit`-shaped edit that copies it into `build.test_command` (build the bytes through the Task 9 splice helpers so comments survive). Ambiguous during migrate → return a typed error naming the candidates and the exact remedy (`docket repository configure-tests` after migration, or an explicit edit) — the app layer surfaces it before ANY remote mutation.

- [ ] **Step 4: Wire the app execution** — init writes `DocketYMLBytes` to the primary-tree `.docket.yml` as another pending unstaged path (exactly the `.gitignore` pattern — same write helper, same pending-path report; grep `GitignorePath` consumers to find it) and reports ambiguity's candidates in the init result without failing init. Migrate includes `ConfigBytes` in the existing preview (the human sees the exact bytes; the authorization key already pins the revision) and commits exactly those bytes with the migration transaction. Extend the app-layer tests covering init pending paths and the migrate preview to assert: init performed no `git add` (learning: generated config is human-gated), preview text contains the config bytes verbatim, ambiguous migrate performs zero remote writes (assert on the recorded git/remote fake).

- [ ] **Step 5: Run** — `go test ./internal/reposetup/ ./internal/app/ ./internal/cli/ -count=1` → PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup internal/app internal/cli
git commit -m "feat(0374): init and migrate generate reviewed test policy through the shared planner"
```

---

### Task 11: `docket repository configure-tests` and the health finding

**Build profile:** standard

**Files:**
- Create: `internal/app/repository_configure_tests.go` (+ test)
- Modify: `internal/cli/repository.go` (new subcommand), `internal/cli/install.go` (the asset-independent map holding `"repository migrate": true`)
- Modify: `internal/reposetup/health.go` (`EvaluateHealth` + `findingFor`), `internal/reposetup/health_test.go`
- Test: `internal/cli/repository_test.go` (`TestRepositoryCommandsRegistered`), `internal/app/repository_configure_tests_test.go`

**Interfaces:**
- Consumes: Tasks 8–9; the repository classification/facts machinery (`Classify`, `Facts`) and the pending-path posture from Task 10.
- Produces: `app.RunRepositoryConfigureTests(ctx, deps app.SetupDeps) app.OperationResult` and the CLI subcommand `docket repository configure-tests`. A new health reason (token `test-config-missing`) with remedy text naming `` `docket repository configure-tests` `` fires when the resolved config has a local gate (build or finalize) with an empty command OR a declared literal `auto` in the committed repository layer.

- [ ] **Step 1: Write the failing tests** — CLI: extend `TestRepositoryCommandsRegistered` with `configure-tests`; add an `install.go` asset-independence entry test if one enumerates the map. App: over the same repo-fixture harness `RunRepositoryInit` tests use — detected suite → `.docket.yml` gains a pending unstaged edit, result reports the path, `git status` porcelain shows it unstaged; valid explicit settings → applied no-op (no write, idempotent); rerun over its own pending bytes → no further change; ambiguous → typed result listing candidates, file untouched (byte-compare before/after). Health: a facts/classification fixture whose resolved config has `build.gate: local` + empty command yields the `test-config-missing` finding with the remedy string; a fixture with both commands explicit yields no such finding; declared `auto` in the committed bytes yields the finding.

- [ ] **Step 2: Run to verify failure** — `go test ./internal/app/ ./internal/cli/ ./internal/reposetup/ -count=1 -run 'ConfigureTests|RepositoryCommands|Health'` → FAIL.

- [ ] **Step 3: Implement** — the operation classifies the repository (must be the healthy Docket topology — it is the upgrade path for already-initialized repositories; a fresh or legacy state gets a typed refusal naming init/migrate instead), reads the committed `.docket.yml` bytes plus the primary tree, runs `DiscoverTests` + `RenderTestConfigEdit`, and writes the pending unstaged edit (same write helper as init). Never commits, never stages. CLI subcommand mirrors `init`'s runner-variable pattern (`repositoryConfigureTestsRunner`); add `"repository configure-tests": true` beside `"repository migrate": true` in `install.go`'s asset-independent set. Health: `EvaluateHealth` needs the resolved test-policy facts — extend `Facts` (or the health entry point's parameters, matching how config-derived findings already reach it; if none do, add a `TestConfigFinding(cfg config.Effective, committedYML []byte) *Finding` helper called from the check pipeline) so the finding is CLOSED: it fires on exactly (local gate with empty command) or (literal `auto` declared in committed bytes).

- [ ] **Step 4: Run and mutation-test** — mutation: suppress the health finding when the finalize command is set but build's is empty → the build-side health assert reddens (the finding must key on EACH local gate independently, not on "any command exists"). Restore.

- [ ] **Step 5: Commit**

```bash
git add internal/app internal/cli internal/reposetup
git commit -m "feat(0374): docket repository configure-tests upgrade path with a closed health finding"
```

---

### Task 12: Docket's own `.docket.yml` and the frozen-fixture re-baseline

**Build profile:** economy

**Files:**
- Modify: `.docket.yml`
- Create: `testdata/repositories/v0.9.7/` (copy of `v0.9.6` with only the docket-self repo config overwritten) + its `PROVENANCE.md`
- Modify: `internal/config/fixtures_test.go` (`docketSelfRoot` constant and the re-derived expectations)

**Interfaces:**
- Consumes: Task 1's schema (the file must resolve).
- Produces: the committed repo config carries both explicit commands.

- [ ] **Step 1: Edit `.docket.yml`** — under `build:`, add above `checkpoint`:

```yaml
  # The build role's own whole-suite gate (change 0374) — same suite as finalize today,
  # but an independent setting; the two may diverge.
  gate: local
  test_command: go run ./cmd/docket development test
```

In the `finalize:` block, delete the stale sentence `scripts/run-tests.sh stays the frozen oracle (0369/0370).` from the comment (that file is deleted; keep the rest of the comment).

- [ ] **Step 2: Re-baseline the frozen docket-self fixture** — the drift guard `TestFixtureDocketSelf` byte-compares the live `.docket.yml`; obey its own remedy (learning `config-edit-trips-its-own-frozen-drift-guard`): `cp -R testdata/repositories/v0.9.6 testdata/repositories/v0.9.7`, overwrite only `v0.9.7/docket-self/repo/.docket.yml` with the edited file, write `v0.9.7/PROVENANCE.md` (copy v0.9.6's format: source = v0.9.6, changed file = docket-self/repo/.docket.yml, reason = change 0374 adds build.gate/build.test_command and drops the frozen-oracle comment). Re-point `const docketSelfRoot = "../../testdata/repositories/v0.9.7"` in `fixtures_test.go`. Never edit v0.9.6.

- [ ] **Step 3: Re-derive expectations by running the resolver** — `go test ./internal/config/ -count=1 -run FixtureDocketSelf`; read the ACTUAL failure diffs (blocker paths, resolved values) and update the test's expected sets to what the resolver truly returns — never guess. Expected end state: PASS with the fixture's other properties (e.g. `MutationAllowed`) unchanged unless the resolver says otherwise.

- [ ] **Step 4: Run the config and app packages** — `go test ./internal/config/ ./internal/app/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add .docket.yml testdata/repositories/v0.9.7 internal/config/fixtures_test.go
git commit -m "config(0374): explicit build.test_command in docket's own config; v0.9.7 fixture re-baseline per the drift guard's remedy"
```

---

### Task 13: Skill bodies, bundled assets, and their guards

**Build profile:** standard

**Files:**
- Modify: `skills/docket-build/SKILL.md` (the halt-vocabulary bullets near `No suite is detectable` and the whole `## The build gate` section)
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-convention/SKILL.md` (grep-derived sites)
- Modify: `internal/assets/embedded/tree/skills/…` (regenerated only)
- Test: whatever suite files currently pin the docket-build gate section's wording (grep-derived; see Step 1)

**Interfaces:**
- Consumes: Task 4's `--owner build` CLI shape; Task 5's skipped-evidence operation; Task 7's `build_gate`/`build_test_command` context keys.
- Produces: no maintained skill text claims runtime auto-detection or a shared test command.

- [ ] **Step 1: Derive the site inventory (never hand-list)** — run and record in the task notes:

```bash
grep -rn -e "FINALIZE_TEST_COMMAND" -e "BUILD_TEST_COMMAND" -e "finalize\.test_command" -e "auto-detect" -e "auto_detect" -e "test_command" skills/ internal/assets/embedded/tree/skills/ README.md tests/ docs/superpowers/README.md 2>/dev/null | grep -v -e "docs/changes/archive" -e "docs/results"
```

Sort hits into executable (test asserts, runnable command blocks in skill bodies — agent-executed markdown is code) vs descriptive prose. Every executable hit must be resolved in this task or Task 15; prose in historical records stays.

- [ ] **Step 2: Rewrite `skills/docket-build/SKILL.md`** — the `## The build gate` section's command-resolution ladder (the numbered list beginning `Use the already-resolved FINALIZE_TEST_COMMAND`) is replaced with:
  - The gate policy arrives in the implementation context as `build_gate` and `build_test_command` (authoritative config; never invent a command).
  - `build_gate: off` → run NOTHING; mint truthful skipped evidence via `docket evidence record` (no run dir) and proceed — the record carries `result: skipped` / `reason: build-gate-off`.
  - `build_gate: local` with empty `build_test_command` → a configuration gap, not a red suite: halt with the remedy `docket repository configure-tests`. Never convert to a repair task, never infer success.
  - `build_gate: local` with a command → drive it via `docket gate drive start --owner build …` then `docket gate drive advance` slices exactly as the section already describes (update the quoted invocations to the `--owner` shape; the `-- <argv>` form is gone).
  - The evidence block description gains the skipped variant beside the green one (fields `result`/`reason`/`head_sha`/`ran_at`, no `command`).
  Also fix the halt-vocabulary bullet `**No suite is detectable** — no FINALIZE_TEST_COMMAND and nothing finalize's auto-detection recognizes` to the configuration-gap wording above. Remember the skill body ships into other repos: no docket-repo-specific sentences (learning `distributed-body-has-no-local-repo`).

- [ ] **Step 3: Sweep the other skills** — from Step 1's inventory, update `docket-implement-next` (evidence acceptance: green OR skipped at exact head; PR body stays truthful about which), `docket-finalize-change` (the suite-skip waiver now also requires the recorded command to equal the resolved `finalize.test_command`; skipped build evidence never waives the finalize gate), and `docket-convention` (config reference: the two independent pairs, `auto` demoted to legacy migration input, `docket repository configure-tests` exists). Keep each edit minimal — these are reference texts, not essays.

- [ ] **Step 4: Regenerate embedded assets and run the doc guards** — `go generate ./internal/assets && go test ./internal/assets/ -count=1`. Then run the test files Step 1 found pinning skill wording; repoint asserts at the NEW wording (relocation, not restoration — learning `restatement-accumulates-its-own-guards`), and for each rewritten guard mutation-test it: restore the old `FINALIZE_TEST_COMMAND` sentence in the skill body, watch the guard redden, re-remove.

- [ ] **Step 5: Commit**

```bash
git add skills internal/assets tests
git commit -m "docs(0374): build skill owns its gate policy; skills drop shared-command and auto-detection claims"
```

---

### Task 14: Maintained documentation — README, AGENTS.md, tests/README

**Build profile:** economy

**Files:**
- Modify: `AGENTS.md` (the `## Guards and tests` bullet quoting `whatever finalize.test_command resolves to`), `README.md`, `tests/README.md` (grep-derived), `docs/superpowers/README.md` if hit

**Interfaces:**
- Consumes: Task 13's inventory (the non-skill hits).

- [ ] **Step 1: Edit AGENTS.md** — the build-gate bullet currently reads `The suite command is whatever finalize.test_command resolves to — read it there, never from a second copy`. Rewrite: the BUILD gate's suite command is whatever `build.test_command` resolves to and finalize's is `finalize.test_command` — read each from config, never from a second copy; in this repo both are `go run ./cmd/docket development test` today. Keep the rest of the bullet (budget clauses, runner) intact.

- [ ] **Step 2: Sweep README/tests-README hits** — from the Task 13 inventory: update any sentence claiming the key is universally shared. Where the release-candidate workflow (release-smoke docs, if hit) deliberately stays bound to finalize policy, STATE that choice explicitly rather than deleting the mention (spec Touch point 7).

- [ ] **Step 3: Run doc-adjacent guards** — rerun any test files that grep these documents (from the inventory). Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md README.md tests docs
git commit -m "docs(0374): maintained docs name the owning test-command key per gate"
```

---

### Task 15: Regression-guard sweep — independent ownership is mutation-sensitive

**Build profile:** standard

**Files:**
- Modify: grep-derived test files that (a) require build to read `FINALIZE_TEST_COMMAND`, (b) forbid `BUILD_TEST_COMMAND`, or (c) assert the `auto` sentinel/masking matrix in its old single-pair shape
- Test: the modified files themselves

**Interfaces:**
- Consumes: everything prior; this task proves the change's own thesis holds against its own additions (learning `fix-reintroduces-its-own-defect-class`).

- [ ] **Step 1: Inventory** — `grep -rn -e "BUILD_TEST_COMMAND" -e "FINALIZE_TEST_COMMAND" tests/ internal/ cmd/ --include="*_test.go" --include="*.sh"` plus the Task 13 leftovers. Any guard whose premise was "the command is shared" is replaced by asking what it GUARDS, not what it asserts (learning `test-premise-deleted-not-regated`): the successor guard asserts independent ownership — typically the divergent-command fixture pattern from Tasks 4–7.

- [ ] **Step 2: Verify the divergent-fixture coverage is complete** — confirm (running each, `-count=1`) that these mutation-backed asserts from earlier tasks exist and redden under their mutations: gate-drive owner swap (Task 4), evidence command source swap (Task 5), gateDecision command conjunct removal (Task 6), context field source swap (Task 7). Any missing one is written here, in that task's file, in its style.

- [ ] **Step 3: Audit the change's own additions against its thesis** — the new code paths most likely to re-couple the pair: `RenderTestConfigEdit` (writes both commands from ONE discovery — correct at setup time, but assert the two written keys are independently editable: a fixture where existing explicit `build.test_command` differs from `finalize.test_command` must survive a re-run untouched), and the CLI `--owner` routing. Add the surviving-divergence renderer test if Task 9 lacks it.

- [ ] **Step 4: Run the touched packages** — `go test ./... -count=1 -run '.'` is the build gate's job; here run every package this task touched. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A tests internal cmd
git commit -m "test(0374): shared-command guards regated as independent-ownership guards"
```

---

### Task 16: Supersede ADR-0063's shared-command decision

**Build profile:** economy (a dispatch, not code)

**Files:**
- None directly — the docket-adr workflow owns the ADR ledger on the metadata branch.

**Interfaces:**
- Consumes: the finished design as built (Tasks 1–15).

- [ ] **Step 1: Dispatch the `docket-adr` agent** (per AGENTS.md "Docket agents — dispatch, don't run inline") with this request: record a new ADR for change 0374 that SUPERSEDES ADR-0063, carrying forward its unaffected build-role decisions (profile routing, one bounded escalation, no per-task review, single whole-suite gate) and replacing Decision 5's shared-command rule ("derived from finalize.test_command … rather than a second, driftable test-command key") with: build and finalize own independent `gate`/`test_command` pairs; neither falls back to the other; `build.gate: off` produces truthful skipped evidence; runtime `auto` is removed in favor of setup-time discovery (`docket repository configure-tests`); finalize reuses build evidence only when green AND head AND command all match. Note ADR-0074's tri-state verdict rule remains in force (configuration gaps and launch failures are halts, never red suites). The new ADR id must be added to the change file's `adrs:` list in the same workflow (learning `adr-update-delivery`).

- [ ] **Step 2: Verify** — after the dispatch returns, confirm on the metadata branch: the new ADR file exists, ADR-0063's status is Superseded with a pointer, the index regenerated, and the change file's `adrs:` names the new id. A dispatch report without these artifacts is not completion — check the files, not the prose.

- [ ] **Step 3: No commit in the feature worktree** — the ADR lives on the metadata branch; this task leaves the feature branch untouched.

---

## Self-Review Notes

- **Spec coverage:** independent config (T1), example/presentation (T2), skipped evidence + semantics (T3, T5), remove runtime auto (T1, T2, T11, T13), gate ownership + provenance + no-substitution (T4), evidence op build-owned + validates against build config (T5), reuse predicate green+head+command / skipped-never-waives (T6), contexts (T7), shared pure planner + closed registry + probe-error-is-unknown (T8), pending-edit generation + quoting (T9), init/migrate integration + preview fidelity + ambiguity-stops (T10), configure-tests + health remedy + idempotency (T11), self-hosting `.docket.yml` + fixture re-baseline (T12), skills/assets (T13), docs incl. release-candidate statement (T14), regression/drift guards (T15), ADR supersession (T16). The complete-suite requirement is docket-build's own final gate, not a task.
- **Deliberate scope notes:** `gate drive start`'s `-- <argv>` surface is removed rather than deprecated — the spec's no-substitution clause forbids it, and Task 13 updates the only executable consumers (skill bodies). `finalize.gate`'s vocabulary and `ci`/`both` classification are untouched.
- Historical records, archived changes, accepted ADR bodies (except 0063's status via the ADR workflow), frozen plans/results, and versioned legacy fixture trees stay byte-unchanged; every inventory grep excludes them explicitly.
