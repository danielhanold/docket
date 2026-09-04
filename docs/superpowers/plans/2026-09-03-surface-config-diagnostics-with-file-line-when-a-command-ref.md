<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0403 — Surface config diagnostics with file:line when a command refuses on invalid configuration](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0403-surface-config-diagnostics-with-file-line-when-a-command-ref.md)**
<!-- docket:backlink:end -->
# Surface Config Diagnostics With file:line Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `repository check`, `repository prepare`, `repository init`, `repository migrate`, or `status` refuses on an invalid configuration, the refusal names every defect's source file, key path, and line in both human and JSON output; `docket diagnostic config`'s human lines gain the file:line too.

**Architecture:** The resolver (`internal/config.Resolve`) already returns `[]config.Diagnostic` alongside `ErrInvalidConfig`; both refusing resolve sites currently discard that slice. The change carries the slice on the two error types (`RepoResolutionError` gains a `Diagnostics` field; `loadOperationalContext` swaps its `fmt.Errorf` wrap for a small typed error), exposes one accessor (`ConfigDiagnostics`), lifts diagnostics into each refusing operation's existing `findings` array via one shared parts mapping, and introduces one shared human line renderer (`ConfigDiagnosticLine`) that `diagnostic config` switches to. No result vocabulary, reason, or exit code changes.

**Tech Stack:** Go (`internal/app`, `internal/config` untouched). Tests: Go unit tests plus git-fixture integration tests in `internal/app` (tagged `//go:build integration`, sharded by test-name prefix).

**Spec:** `docs/superpowers/specs/2026-09-03-surface-config-diagnostics-with-file-line-when-a-command-ref-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/...` in the primary checkout). Change: `docs/changes/active/0403-surface-config-diagnostics-with-file-line-when-a-command-ref.md`.

## Global Constraints

- **No change to result vocabularies, reasons, or exit codes.** Every refusal keeps today's `result`, `reason`, `message`, and exit byte-for-byte; only `findings` (and human text below the existing header line) are added.
- `internal/config` is untouched. Change 0392 (install-path unknown-key tolerance) is a disjoint path: do not modify `ResolveRepoPhase`'s own `RepoResolutionError` constructors in `repophase.go` beyond the struct gaining a field (their `Diagnostics` stays nil).
- Every test run whose purpose is to observe an outcome uses `-count=1` (learning cached-runner-serves-a-mutated-tree). Integration-tagged tests need `-tags integration`.
- Mutation probes restore via `cp` backup (`cp f f.bak; mutate; run; mv f.bak f`), never `git checkout --`, and prove the mutation landed via `git diff` before believing any reading (learning mutation-restore-needs-a-backup-copy).
- New integration test names must match an existing shard prefix exactly (`tests/test_go_integration_contract.sh` enforces every tagged test matches exactly one runner): `TestIntegrationRepoCheck*`, `TestIntegrationRepoPrepare*`, `TestIntegrationRepoSetup*` (init), `TestIntegrationRepoMigration*`. The status test is **untagged** and must NOT carry a `TestIntegration` prefix (contract clause: no integration-prefixed test visible to the default-tag corpus).
- Cross-references in comments anchor on symbol names or verbatim clauses, never line numbers.
- The build gate runs the whole suite via the configured `build.test_command` (today `go run ./cmd/docket development test`), entered from the feature worktree.
- Commit messages end with the trailer `Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV`.

## Spec discrepancies found at planning (record in the results file)

1. The spec claims "these fields already exist" for every findings array; **`RepositoryMigrateResult` has no `Findings` field** (`internal/app/repository_migrate.go`, struct `RepositoryMigrateResult`). Task 6 adds it (`json:"findings,omitempty"`). The 0399 schema surface derives from live Go types (`internal/app/schema_tags.go`), so no schema artefact is hand-edited, but `schema_tags_test.go` / `internal/cli/schema_command_test.go` must be run and any derived expectation updated.
2. The spec's §3 example renders check's refusal header as `repository check: unsupported-config (unknown)` while §2 (normative) says "the existing header line followed by the shared finding block". This plan follows §2: today's header lines (e.g. `repository check: unsupported-config: invalid configuration`) are preserved byte-for-byte and the finding block is appended, so no existing header-pinning test or consumer breaks.
3. Status's healthy path already lifts config diagnostics via `configFinding` in `status.go`, which **deliberately drops provenance** ("a global config path is host-absolute and the protocol context forbids one" — its own comment). The spec does not mention it. It is left untouched: the refusal path uses the new projection per the spec; unifying the healthy path is out of scope. Note in results.

## File Structure

- Create: `internal/app/config_diagnostics.go` — `ConfigDiagnostics`, `errInvalidConfiguration`, `configDiagnosticParts`, `configDiagnosticFindings`, `configDiagnosticStatusFindings`, `appendConfigFindingBlock`, `ConfigDiagnosticLine`.
- Create: `internal/app/config_diagnostics_test.go` — unit tests for all of the above, plus the fixture-semantics probe test.
- Create: `internal/app/configrefusal_integration_test.go` — the four repository-family integration tests (tagged).
- Modify: `internal/app/repophase.go` — `Diagnostics` field on `RepoResolutionError`.
- Modify: `internal/app/repository_facts.go` — `resolveSetupConfig` passes `diags` through.
- Modify: `internal/app/operational_context.go` — typed error at the `config.Resolve` failure.
- Modify: `internal/app/repository_check.go` (`checkGatherFailure`), `repository_prepare.go` (`prepareGatherFailure`), `repository_init.go` (`repositoryGatherFailure`), `repository_migrate.go` (`RepositoryMigrateResult` + `migrateGatherFailure`), `status.go` (`statusFailure`).
- Modify: `internal/app/config.go` — `ConfigInspectionResult.HumanText` diagnostics loop uses `ConfigDiagnosticLine`.
- Modify: `internal/app/operational_context_test.go` or new sibling — untagged status refusal test.

---

### Task 1: Carry the diagnostics on both error types + `ConfigDiagnostics` accessor

**Files:**
- Create: `internal/app/config_diagnostics.go`
- Create: `internal/app/config_diagnostics_test.go`
- Modify: `internal/app/repophase.go` (struct `RepoResolutionError`)
- Modify: `internal/app/repository_facts.go` (func `resolveSetupConfig`)
- Modify: `internal/app/operational_context.go` (func `loadOperationalContext`, the `config.Resolve` failure branch)

**Interfaces:**
- Consumes: `config.Diagnostic`, `config.Resolve` (existing), `ErrStatusInvalidInput`, `ReasonInvalidConfig`.
- Produces: `func ConfigDiagnostics(err error) []config.Diagnostic`; `RepoResolutionError.Diagnostics []config.Diagnostic`; unexported `errInvalidConfiguration` (constructed only inside `loadOperationalContext`). Tasks 4–7 call `ConfigDiagnostics` / read `rre.Diagnostics`.

- [ ] **Step 1: Write the failing unit tests** in `internal/app/config_diagnostics_test.go`:

```go
package app

import (
	"errors"
	"fmt"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

func sampleDiags() []config.Diagnostic {
	return []config.Diagnostic{
		{Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key",
			Message: "is not a docket configuration setting",
			Provenance: &config.Provenance{Layer: config.LayerRepository, Source: ".docket.yml", Line: 2, Column: 1}},
		{Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "ignored", Remedy: "remove runtime.bash from this file",
			Provenance: &config.Provenance{Layer: config.LayerGlobal, Source: "/home/x/.config/docket/config.yml", Line: 3}},
	}
}

// ConfigDiagnostics unwraps both refusal shapes and returns nil for anything else.
func TestConfigDiagnosticsUnwrapsBothErrorTypes(t *testing.T) {
	diags := sampleDiags()

	rre := &RepoResolutionError{Reason: ReasonInvalidConfig, Err: config.ErrInvalidConfig, Diagnostics: diags}
	if got := ConfigDiagnostics(fmt.Errorf("wrapped: %w", rre)); len(got) != 2 {
		t.Fatalf("ConfigDiagnostics(RepoResolutionError) = %d diags, want 2", len(got))
	}

	eic := &errInvalidConfiguration{diagnostics: diags, err: config.ErrInvalidConfig}
	if got := ConfigDiagnostics(fmt.Errorf("wrapped: %w", eic)); len(got) != 2 {
		t.Fatalf("ConfigDiagnostics(errInvalidConfiguration) = %d diags, want 2", len(got))
	}

	if got := ConfigDiagnostics(errors.New("unrelated")); got != nil {
		t.Fatalf("ConfigDiagnostics(unrelated) = %v, want nil", got)
	}
	// A RepoResolutionError with no diagnostics (e.g. LoadFilesystemSources failed
	// before resolution) yields nil, not an empty non-nil slice.
	bare := &RepoResolutionError{Reason: ReasonInvalidConfig, Err: config.ErrInvalidConfig}
	if got := ConfigDiagnostics(bare); got != nil {
		t.Fatalf("ConfigDiagnostics(bare RepoResolutionError) = %v, want nil", got)
	}
}

// The typed operational error classifies as ErrStatusInvalidInput and keeps
// today's message text byte-for-byte, so every existing caller is unchanged.
func TestErrInvalidConfigurationCompatibility(t *testing.T) {
	eic := &errInvalidConfiguration{diagnostics: sampleDiags(), err: config.ErrInvalidConfig}
	if !errors.Is(eic, ErrStatusInvalidInput) {
		t.Fatal("errors.Is(errInvalidConfiguration, ErrStatusInvalidInput) = false, want true")
	}
	if !errors.Is(eic, config.ErrInvalidConfig) {
		t.Fatal("errors.Is(errInvalidConfiguration, config.ErrInvalidConfig) = false, want true")
	}
	legacy := fmt.Errorf("%w: %v", ErrStatusInvalidInput, config.ErrInvalidConfig)
	if eic.Error() != legacy.Error() {
		t.Fatalf("Error() = %q, want the legacy wrap text %q", eic.Error(), legacy.Error())
	}
}
```

Note: check the exact spelling of the layer constants before compiling (`grep -n "Layer" internal/config/config.go` — the type is `LayerKind`; use whatever repository/global constants exist, e.g. by reading the `LayerKind` const block). If constructing provenance layers is awkward, the tests may leave `Layer` at its zero value — nothing in this change reads it.

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/app/ -run 'TestConfigDiagnostics|TestErrInvalidConfiguration' -v`
Expected: FAIL to compile (`ConfigDiagnostics`, `errInvalidConfiguration`, `Diagnostics` field undefined).

- [ ] **Step 3: Implement.** In `internal/app/repophase.go`, extend the struct (the file already imports `config`):

```go
// RepoResolutionError carries the stable machine reason a repository resolution
// ... (keep the existing comment, extend it with:)
// Diagnostics carries the resolver's per-defect diagnostics when Reason is
// ReasonInvalidConfig and config.Resolve itself produced them; nil otherwise
// (a source-loading failure has no resolver diagnostics).
type RepoResolutionError struct {
	Reason      string
	Err         error
	Diagnostics []config.Diagnostic
}
```

In `internal/app/repository_facts.go`, `resolveSetupConfig`, change the `config.Resolve` call to keep the middle return and pass it through (the `LoadFilesystemSources` failure branch above it is unchanged):

```go
	snap, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		return config.Effective{}, &RepoResolutionError{Reason: ReasonInvalidConfig, Err: err, Diagnostics: diags}
	}
```

In `internal/app/operational_context.go`, `loadOperationalContext`, replace only the `config.Resolve` failure return (keep the explanatory comment above it; the `operationalConfigSources` failure keeps its existing wrap):

```go
	snap, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})
	if err != nil {
		// (existing comment retained)
		return oc, &errInvalidConfiguration{diagnostics: diags, err: err}
	}
```

Create `internal/app/config_diagnostics.go`:

```go
// Package app: config_diagnostics carries resolver diagnostics across an
// invalid-configuration refusal and renders them for humans and findings
// arrays (change 0403). The resolver stays the single author of diagnostic
// content; this file only transports and formats it.
package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/reposetup"
)

// errInvalidConfiguration is the operational loader's invalid-config refusal.
// It carries the resolver's diagnostics so the refusing operation can lift
// them into its findings, and it classifies as ErrStatusInvalidInput so every
// existing errors.Is caller is unchanged. Error() preserves the legacy
// fmt.Errorf("%w: %v", ErrStatusInvalidInput, err) text byte-for-byte.
type errInvalidConfiguration struct {
	diagnostics []config.Diagnostic
	err         error
}

func (e *errInvalidConfiguration) Error() string {
	return ErrStatusInvalidInput.Error() + ": " + e.err.Error()
}
func (e *errInvalidConfiguration) Unwrap() error { return e.err }
func (e *errInvalidConfiguration) Is(target error) bool { return target == ErrStatusInvalidInput }

// ConfigDiagnostics returns the resolver diagnostics an invalid-configuration
// refusal carries, in resolver order, or nil when err is not such a refusal
// (or the refusal predates resolution and carries none).
func ConfigDiagnostics(err error) []config.Diagnostic {
	var rre *RepoResolutionError
	if errors.As(err, &rre) && len(rre.Diagnostics) > 0 {
		return rre.Diagnostics
	}
	var eic *errInvalidConfiguration
	if errors.As(err, &eic) && len(eic.diagnostics) > 0 {
		return eic.diagnostics
	}
	return nil
}
```

(`fmt`, `strings`, `reposetup` become used in Task 2; if the compiler complains at this step, add them in Task 2 instead.)

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/app/ -run 'TestConfigDiagnostics|TestErrInvalidConfiguration' -v` then `go build ./...` and `go test -count=1 ./internal/app/`
Expected: PASS; whole package still green (the `statusFailure` path still classifies the new error identically via `errors.Is`).

- [ ] **Step 5: Commit**

```bash
git add internal/app/config_diagnostics.go internal/app/config_diagnostics_test.go internal/app/repophase.go internal/app/repository_facts.go internal/app/operational_context.go
git commit -m "fix(0403): carry resolver diagnostics on both invalid-config refusal errors

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 2: One shared mapping — lift helpers and the human finding block

**Files:**
- Modify: `internal/app/config_diagnostics.go`
- Modify: `internal/app/config_diagnostics_test.go`

**Interfaces:**
- Produces (consumed by Tasks 4–7):
  - `func configDiagnosticParts(d config.Diagnostic) configFindingParts` — internal single source; `configFindingParts{code, severity, ref, message, remedy string}`.
  - `func configDiagnosticFindings(diags []config.Diagnostic) []reposetup.Finding`
  - `func configDiagnosticStatusFindings(diags []config.Diagnostic) []StatusFinding`
  - `func appendConfigFindingBlock(header string, findings []reposetup.Finding) string`

- [ ] **Step 1: Write the failing unit tests** (append to `config_diagnostics_test.go`):

```go
// The field mapping (spec §2): code and severity verbatim; ref is
// source:line, source alone at line 0, empty with no provenance; message is
// "path: message" when a key path exists; remedy verbatim; warnings carried.
func TestConfigDiagnosticFindingsMapping(t *testing.T) {
	diags := []config.Diagnostic{
		{Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message: `expects a string, got int "42"`,
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6}},
		{Code: "invalid-yaml", Severity: config.SeverityError,
			Message: "broken document",
			Provenance: &config.Provenance{Source: ".docket.yml"}}, // line 0 → source alone
		{Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "ignored", Remedy: "remove runtime.bash"}, // no provenance → empty ref
	}
	got := configDiagnosticFindings(diags)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (warnings are carried)", len(got))
	}
	want := []reposetup.Finding{
		{Code: "invalid-type", Severity: reposetup.Severity("error"), Ref: ".docket.yml:6",
			Message: `agents.claude.adr.model: expects a string, got int "42"`},
		{Code: "invalid-yaml", Severity: reposetup.Severity("error"), Ref: ".docket.yml",
			Message: "broken document"},
		{Code: "obsolete-setting", Severity: reposetup.Severity("warning"), Ref: "",
			Message: "runtime.bash: ignored", Remedy: "remove runtime.bash"},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if configDiagnosticFindings(nil) != nil {
		t.Error("configDiagnosticFindings(nil) must be nil")
	}
}

// The status projection is the same parts mapping in the StatusFinding shape
// (Path carries the ref; severity goes through normalizeSeverity).
func TestConfigDiagnosticStatusFindingsMapping(t *testing.T) {
	diags := []config.Diagnostic{
		{Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message: "expects a string",
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6}},
		{Code: "some-note", Severity: config.SeverityInfo, Message: "fyi"},
	}
	got := configDiagnosticStatusFindings(diags)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Code != "invalid-type" || got[0].Severity != "error" ||
		got[0].Path != ".docket.yml:6" || got[0].Message != "agents.claude.adr.model: expects a string" {
		t.Errorf("status finding[0] = %+v", got[0])
	}
	if got[1].Severity != "notice" { // normalizeSeverity: info → notice, per the DTO contract
		t.Errorf("info severity projected as %q, want notice", got[1].Severity)
	}
}

// appendConfigFindingBlock renders exactly the block shape
// RepositoryCheckResult.HumanText already emits per finding.
func TestAppendConfigFindingBlock(t *testing.T) {
	findings := []reposetup.Finding{
		{Code: "unknown-key", Severity: reposetup.Severity("error"), Ref: ".docket.yml:2",
			Message: "bogus_key: is not a docket configuration setting"},
		{Code: "obsolete-setting", Severity: reposetup.Severity("warning"),
			Message: "runtime.bash: ignored", Remedy: "remove it"},
	}
	got := appendConfigFindingBlock("header line", findings)
	want := "header line" +
		"\n- [error] unknown-key (.docket.yml:2)\n  bogus_key: is not a docket configuration setting" +
		"\n- [warning] obsolete-setting\n  runtime.bash: ignored\n  remedy: remove it"
	if got != want {
		t.Errorf("block:\n%q\nwant:\n%q", got, want)
	}
	if got := appendConfigFindingBlock("h", nil); got != "h" {
		t.Errorf("empty findings must return the bare header, got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/app/ -run 'TestConfigDiagnostic|TestAppendConfigFindingBlock' -v`
Expected: FAIL to compile (helpers undefined).

- [ ] **Step 3: Implement** (append to `config_diagnostics.go`):

```go
// configFindingParts is the single internal projection of one resolver
// diagnostic into finding vocabulary. Both finding shapes (reposetup.Finding
// and StatusFinding) are thin views of it, so the two cannot drift (spec §2).
type configFindingParts struct {
	code, severity, ref, message, remedy string
}

func configDiagnosticParts(d config.Diagnostic) configFindingParts {
	ref := ""
	if d.Provenance != nil {
		ref = d.Provenance.Source
		if d.Provenance.Line > 0 {
			ref = fmt.Sprintf("%s:%d", d.Provenance.Source, d.Provenance.Line)
		}
	}
	message := d.Message
	if d.Path != "" {
		message = d.Path + ": " + d.Message
	}
	return configFindingParts{
		code:     d.Code,
		severity: string(d.Severity),
		ref:      ref,
		message:  message,
		remedy:   d.Remedy,
	}
}

// configDiagnosticFindings lifts resolver diagnostics into the repository
// family's finding shape, one finding per diagnostic, in resolver order.
// Warnings ride along so the reader sees the whole resolver verdict; the
// operation's result and exit are decided by the error, never by these.
// Repairable stays nil: config findings are never auto-repairable.
func configDiagnosticFindings(diags []config.Diagnostic) []reposetup.Finding {
	if len(diags) == 0 {
		return nil
	}
	out := make([]reposetup.Finding, 0, len(diags))
	for _, d := range diags {
		p := configDiagnosticParts(d)
		out = append(out, reposetup.Finding{
			Code:     p.code,
			Severity: reposetup.Severity(p.severity),
			Ref:      p.ref,
			Message:  p.message,
			Remedy:   p.remedy,
		})
	}
	return out
}

// configDiagnosticStatusFindings is the same lift in the status DTO's finding
// shape. Severity goes through normalizeSeverity (info → notice) to honor the
// StatusFinding severity vocabulary; the ref rides in Path, the locator slot
// the human renderer already prints.
func configDiagnosticStatusFindings(diags []config.Diagnostic) []StatusFinding {
	if len(diags) == 0 {
		return nil
	}
	out := make([]StatusFinding, 0, len(diags))
	for _, d := range diags {
		p := configDiagnosticParts(d)
		out = append(out, StatusFinding{
			Code:     p.code,
			Severity: normalizeSeverity(p.severity),
			Path:     p.ref,
			Message:  p.message,
			Remedy:   p.remedy,
		})
	}
	return out
}

// appendConfigFindingBlock appends the shared per-finding human block to a
// refusal's existing header line — the same "- [severity] code (ref)" block
// RepositoryCheckResult.HumanText emits on the healthy classification path.
func appendConfigFindingBlock(header string, findings []reposetup.Finding) string {
	var b strings.Builder
	b.WriteString(header)
	for _, f := range findings {
		fmt.Fprintf(&b, "\n- [%s] %s", f.Severity, f.Code)
		if f.Ref != "" {
			fmt.Fprintf(&b, " (%s)", f.Ref)
		}
		if f.Message != "" {
			fmt.Fprintf(&b, "\n  %s", f.Message)
		}
		if f.Remedy != "" {
			fmt.Fprintf(&b, "\n  remedy: %s", f.Remedy)
		}
	}
	return b.String()
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/app/ -run 'TestConfigDiagnostic|TestAppendConfigFindingBlock' -v`
Expected: PASS. (If `reposetup.Severity` string-conversion or `StatusFinding` field names differ from the code above, fix against the real definitions — `internal/reposetup/health.go` `type Finding struct` and `internal/app/status_result.go` `type StatusFinding struct` — not by weakening asserts.)

- [ ] **Step 5: Add the fixture-semantics probe test.** The spec's fixture claims specific codes and lines; pin them against the real resolver once, so the integration tests in Tasks 4–7 rest on measured behavior (learning verify-the-claim). Append to `config_diagnostics_test.go`:

```go
// invalidConfigYML is the shared invalid fixture (spec §Testing): three
// defects at known lines — unknown-key at 2, invalid-type at 6,
// invalid-value at 7.
const invalidConfigYML = `changes_dir: docs/changes
bogus_key: 1
agents:
  claude:
    adr:
      model: 42
auto_capture: true
`

// TestInvalidConfigFixtureSemantics pins what the resolver actually says
// about the shared fixture: the three error diagnostics, their codes, their
// .docket.yml:<line> provenance, and their order. The integration tests
// assert the lift of exactly this verdict.
func TestInvalidConfigFixtureSemantics(t *testing.T) {
	sources := []config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: []byte(invalidConfigYML)}}
	_, diags, err := config.Resolve(sources, config.ResolveContext{DefaultBranch: "main"})
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("Resolve err = %v, want ErrInvalidConfig", err)
	}
	var errs []config.Diagnostic
	for _, d := range diags {
		if d.Severity == config.SeverityError {
			errs = append(errs, d)
		}
	}
	if len(errs) != 3 {
		t.Fatalf("error diagnostics = %d (%+v), want 3", len(errs), diags)
	}
	wantRefs := map[string]string{
		"unknown-key":   ".docket.yml:2",
		"invalid-type":  ".docket.yml:6",
		"invalid-value": ".docket.yml:7",
	}
	for _, d := range errs {
		want, ok := wantRefs[d.Code]
		if !ok {
			t.Errorf("unexpected error code %q (%+v)", d.Code, d)
			continue
		}
		if got := configDiagnosticParts(d).ref; got != want {
			t.Errorf("code %q ref = %q, want %q", d.Code, got, want)
		}
		delete(wantRefs, d.Code)
	}
	for code := range wantRefs {
		t.Errorf("expected error code %q missing", code)
	}
}
```

Check the exact `config.Source`/`LayerRepository` field spellings against `internal/config/config.go` (`type Source struct{ Layer LayerKind; Name string; Data []byte }` — confirm the repository layer constant's name, e.g. via `grep -n "LayerKind =" internal/config/*.go`). Run it: `go test -count=1 ./internal/app/ -run TestInvalidConfigFixtureSemantics -v`. **If a code or line differs from the spec's claim** (e.g. `auto_capture: true` classifies under a different code), pin the observed value in this test AND use the observed values in Tasks 4–7's asserts, and record the discrepancy for the results file — never adapt the assert silently.

- [ ] **Step 6: Commit**

```bash
git add internal/app/config_diagnostics.go internal/app/config_diagnostics_test.go
git commit -m "fix(0403): shared diagnostic→finding lift, human block, fixture semantics probe

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 3: `ConfigDiagnosticLine` + `diagnostic config` human renderer

**Files:**
- Modify: `internal/app/config_diagnostics.go`, `internal/app/config_diagnostics_test.go`
- Modify: `internal/app/config.go` (func `(r ConfigInspectionResult) HumanText`, the `diagnostics:` loop)
- Possibly modify: `internal/app/config_test.go` (any assert pinning the old diagnostics row format)

**Interfaces:**
- Produces: `func ConfigDiagnosticLine(d config.Diagnostic) string` (exported; the one shared human line, spec §3).

- [ ] **Step 1: Write the failing unit tests** (append to `config_diagnostics_test.go`):

```go
// ConfigDiagnosticLine (spec §3): severity padded to width of "warning",
// then code, key path, <file>:<line>, message, each omitted when empty and
// separated by two spaces; remedy appended as " | remedy: ...".
func TestConfigDiagnosticLine(t *testing.T) {
	cases := []struct {
		name string
		d    config.Diagnostic
		want string
	}{
		{"full with line", config.Diagnostic{
			Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message: `expects a string, got int "42"`,
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6},
		}, `error    invalid-type  agents.claude.adr.model  .docket.yml:6  expects a string, got int "42"`},
		{"warning with remedy", config.Diagnostic{
			Code: "obsolete-setting", Severity: config.SeverityWarning, Path: "runtime.bash",
			Message: "it is ignored", Remedy: "remove runtime.bash from this file",
			Provenance: &config.Provenance{Source: "/Users/x/.config/docket/config.yml", Line: 3},
		}, "warning  obsolete-setting  runtime.bash  /Users/x/.config/docket/config.yml:3  it is ignored | remedy: remove runtime.bash from this file"},
		{"line zero keeps file alone", config.Diagnostic{
			Code: "invalid-yaml", Severity: config.SeverityError, Message: "broken",
			Provenance: &config.Provenance{Source: ".docket.yml"},
		}, "error    invalid-yaml  .docket.yml  broken"},
		{"no provenance", config.Diagnostic{
			Code: "unknown-key", Severity: config.SeverityError, Path: "bogus_key", Message: "not a setting",
		}, "error    unknown-key  bogus_key  not a setting"},
	}
	for _, c := range cases {
		if got := ConfigDiagnosticLine(c.d); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
```

(Note the padding arithmetic: `%-7s` pads `error` to 7 chars, then the two-space separator gives `error` + 2 spaces of pad + 2 separator = `error    ` (4 spaces) and `warning  ` (2 spaces) — matching the spec's §3 example exactly.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/app/ -run TestConfigDiagnosticLine -v`
Expected: FAIL to compile.

- [ ] **Step 3: Implement** (append to `config_diagnostics.go`):

```go
// ConfigDiagnosticLine renders one config diagnostic the way every human
// surface prints it: severity (padded to the width of "warning"), code, key
// path, <file>:<line>, message — fields two-space separated, omitted when
// empty — with the remedy appended as " | remedy: ..." (spec: one shared
// human rendering, change 0403). The <file>:<line> follows the same ref rule
// as configDiagnosticParts: file alone at line 0, nothing without provenance.
func ConfigDiagnosticLine(d config.Diagnostic) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-7s  %s", string(d.Severity), d.Code)
	if d.Path != "" {
		fmt.Fprintf(&b, "  %s", d.Path)
	}
	if ref := configDiagnosticParts(d).ref; ref != "" {
		fmt.Fprintf(&b, "  %s", ref)
	}
	if d.Message != "" {
		fmt.Fprintf(&b, "  %s", d.Message)
	}
	if d.Remedy != "" {
		fmt.Fprintf(&b, " | remedy: %s", d.Remedy)
	}
	return b.String()
}
```

Then in `internal/app/config.go`, `ConfigInspectionResult.HumanText`, replace the body of the diagnostics loop (currently `fmt.Fprintf(&b, "  %-7s %s", d.Severity, d.Code)` + path + ` — %s` message):

```go
	if len(r.Diagnostics) > 0 {
		b.WriteString("\ndiagnostics:\n")
		for _, d := range r.Diagnostics {
			fmt.Fprintf(&b, "  %s\n", ConfigDiagnosticLine(d))
		}
	}
```

- [ ] **Step 4: Add one renderer assertion for the file:line gain.** Find the existing `diagnostic config` human-text tests: `grep -n "diagnostics:" internal/app/config_test.go`. Update any assert pinning the old `sev code path — message` row to the new `ConfigDiagnosticLine` form, and add (in `config_test.go` or `config_diagnostics_test.go`, following the existing test construction pattern for `ConfigInspectionResult`):

```go
// The diagnostic-config human surface now names the offending file:line.
func TestConfigInspectionHumanTextNamesFileLine(t *testing.T) {
	r := ConfigInspectionResult{
		Envelope: NewEnvelope(OperationDiagnosticConfig, ResultUnsupportedConfig),
		Diagnostics: []config.Diagnostic{{
			Code: "invalid-type", Severity: config.SeverityError, Path: "agents.claude.adr.model",
			Message: "expects a string",
			Provenance: &config.Provenance{Source: ".docket.yml", Line: 6},
		}},
	}
	if h := r.HumanText(); !strings.Contains(h, ".docket.yml:6") {
		t.Errorf("HumanText lacks the .docket.yml:6 ref:\n%s", h)
	}
}
```

(Check the real `ConfigInspectionResult` construction requirements — `Operation`/`Result` envelope constants — against `internal/app/config.go` `DiagnosticConfig`; mirror how existing tests in `config_test.go` build one rather than inventing a shape.)

- [ ] **Step 5: Run the package**

Run: `go test -count=1 ./internal/app/`
Expected: PASS, including any updated `diagnostic config` golden asserts. Also run `go test -count=1 ./internal/cli/ ./cmd/...` — CLI-level tests may pin the old human diagnostics rows.

- [ ] **Step 6: Commit**

```bash
git add internal/app/config_diagnostics.go internal/app/config_diagnostics_test.go internal/app/config.go
git add internal/app/config_test.go  # only if modified
git commit -m "fix(0403): shared ConfigDiagnosticLine; diagnostic config human rows gain file:line

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 4: `repository check` refusal carries the findings

**Files:**
- Create: `internal/app/configrefusal_integration_test.go` (tagged `//go:build integration`)
- Modify: `internal/app/repository_check.go` (func `checkGatherFailure`)

**Interfaces:**
- Consumes: `configDiagnosticFindings`, `appendConfigFindingBlock` (Task 2); `newInitRepo`, `runCheck` fixtures (`internal/app/reposetup_integration_test.go`, `repocheck_integration_test.go` — tagged, so available to this file); `invalidConfigYML` (Task 2 — defined in an untagged file, visible here).
- Produces: the shared integration assertion helper `assertConfigRefusalFindings` used by Tasks 5–6.

- [ ] **Step 1: Write the failing integration test.** Create `internal/app/configrefusal_integration_test.go`:

```go
//go:build integration

package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- invalid-configuration refusal diagnostics (change 0403) -----------------
//
// Every command that refuses on an invalid .docket.yml must lift the
// resolver's diagnostics into its findings array — code, severity, and a
// .docket.yml:<line> ref — and name each ref in its human text, while the
// result / reason / exit stay exactly what they were before the lift.
// The fixture is invalidConfigYML (three error defects at lines 2, 6, 7;
// semantics pinned by TestInvalidConfigFixtureSemantics).

// wantConfigRefusalRefs maps the expected finding codes to their refs.
// Keep in lockstep with TestInvalidConfigFixtureSemantics.
var wantConfigRefusalRefs = map[string]string{
	"unknown-key":   ".docket.yml:2",
	"invalid-type":  ".docket.yml:6",
	"invalid-value": ".docket.yml:7",
}

// assertConfigRefusalFindings asserts the lifted error findings and the human
// refs for one refusing command's result.
func assertConfigRefusalFindings(t *testing.T, findings []reposetup.Finding, human string) {
	t.Helper()
	var errs []reposetup.Finding
	for _, f := range findings {
		if f.Severity == reposetup.Severity("error") {
			errs = append(errs, f)
		}
	}
	if len(errs) != 3 {
		t.Fatalf("error findings = %d (%+v), want 3", len(errs), findings)
	}
	seen := map[string]bool{}
	for _, f := range errs {
		want, ok := wantConfigRefusalRefs[f.Code]
		if !ok || seen[f.Code] {
			t.Errorf("unexpected or duplicate finding code %q (%+v)", f.Code, f)
			continue
		}
		seen[f.Code] = true
		if f.Ref != want {
			t.Errorf("finding %q ref = %q, want %q", f.Code, f.Ref, want)
		}
		if f.Message == "" {
			t.Errorf("finding %q has an empty message", f.Code)
		}
	}
	for _, ref := range wantConfigRefusalRefs {
		if !strings.Contains(human, ref) {
			t.Errorf("human text lacks ref %q:\n%s", ref, human)
		}
	}
}

// TestIntegrationRepoCheckInvalidConfigDiagnostics: check still refuses
// unsupported-config with exit 2 (state unknown), and now carries the three
// findings in JSON and their refs in the human text.
func TestIntegrationRepoCheckInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := r.runCheck(t)

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	if res.RepositoryState != string(reposetup.StateUnknown) {
		t.Errorf("repository_state = %q, want unknown", res.RepositoryState)
	}
	if code := res.CheckExitCode(); code != 2 {
		t.Errorf("exit = %d, want 2 (state unknown, unchanged by the lift)", code)
	}
	human := res.HumanText()
	if !strings.HasPrefix(human, "repository check: "+string(ResultUnsupportedConfig)+": ") {
		t.Errorf("human header changed: %q", human)
	}
	assertConfigRefusalFindings(t, res.Findings, human)
}
```

(Check `ResultUnsupportedConfig`'s type — if `res.Result` compares against a string, adjust the comparisons the way `checkGatherFailure`'s own switch spells them.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration -count=1 ./internal/app/ -run TestIntegrationRepoCheckInvalidConfigDiagnostics -v`
Expected: FAIL — `res.Findings` is empty and the human text carries no refs. (Result/state/exit asserts should already pass; if any of those fail, stop — that is a false premise about today's behavior, not something to adjust the code toward.)

- [ ] **Step 3: Implement.** In `internal/app/repository_check.go`, `checkGatherFailure`, populate findings and append the block in the invalid-config case only:

```go
func checkGatherFailure(err error) RepositoryCheckResult {
	var rre *RepoResolutionError
	result := ResultExternalFailed
	switch {
	case errors.As(err, &rre):
		result = ResultUnsupportedConfig
	case errors.Is(err, ErrStatusInvalidInput):
		result = ResultInvalidInput
	}
	out := RepositoryCheckResult{
		Envelope:        NewEnvelope(OperationRepositoryCheck, result),
		RepositoryState: string(reposetup.StateUnknown),
	}
	out.human = fmt.Sprintf("repository check: %s: %s", result, err.Error())
	// An invalid configuration carries the resolver's own diagnostics: lift
	// them into findings and append the shared block, so the refusal names
	// each defect's .docket.yml:<line> (change 0403). The result and exit are
	// unchanged — decided above by the error, never by the findings.
	if rre != nil && len(rre.Diagnostics) > 0 {
		out.Findings = configDiagnosticFindings(rre.Diagnostics)
		out.human = appendConfigFindingBlock(out.human, out.Findings)
	}
	return out
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test -tags integration -count=1 ./internal/app/ -run 'TestIntegrationRepoCheck' -v` (the whole check shard prefix, so existing check scenarios prove unbroken), then `go test -count=1 ./internal/app/`
Expected: PASS. Note: `CheckExitCode` derives exit from state + findings via `reposetup.CheckExit`; the test's exit-2 assert is the guard that populating findings did not shift it.

- [ ] **Step 5: Commit**

```bash
git add internal/app/configrefusal_integration_test.go internal/app/repository_check.go
git commit -m "fix(0403): repository check invalid-config refusal lifts resolver diagnostics into findings

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 5: `repository prepare` refusal carries the findings

**Files:**
- Modify: `internal/app/configrefusal_integration_test.go`
- Modify: `internal/app/repository_prepare.go` (func `prepareGatherFailure`)

**Interfaces:**
- Consumes: `assertConfigRefusalFindings` (Task 4), `configDiagnosticFindings`, `appendConfigFindingBlock`; prepare fixtures — find how `repoprepare_integration_test.go` invokes `RunRepositoryPrepare` against an `initRepo`-style fixture (`grep -n "RunRepositoryPrepare" internal/app/repoprepare_integration_test.go`) and reuse its runner helper.

- [ ] **Step 1: Write the failing integration test** (append to `configrefusal_integration_test.go`; mirror the prepare shard's own invocation helper — the snippet below assumes a `runPrepare`-style helper or direct `RunRepositoryPrepare(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, PrepareOptions{})` call; copy the exact deps/options construction from an existing `TestIntegrationRepoPrepare*` test):

```go
// TestIntegrationRepoPrepareInvalidConfigDiagnostics: prepare still refuses
// (unsupported-config / refused), and now carries the structured findings the
// Step-0 contract promises, plus the refs in its human text.
func TestIntegrationRepoPrepareInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := RunRepositoryPrepare(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, PrepareOptions{})

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	if res.Disposition != PrepareDispositionRefused {
		t.Errorf("disposition = %q, want refused", res.Disposition)
	}
	human := res.HumanText()
	if !strings.HasPrefix(human, "repository prepare: "+PrepareDispositionRefused+": ") {
		t.Errorf("human header changed: %q", human)
	}
	assertConfigRefusalFindings(t, res.Findings, human)
}
```

(Add `"context"` to the file's imports.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration -count=1 ./internal/app/ -run TestIntegrationRepoPrepareInvalidConfigDiagnostics -v`
Expected: FAIL on empty findings / missing refs; result and disposition asserts pass.

- [ ] **Step 3: Implement.** In `internal/app/repository_prepare.go`, `prepareGatherFailure`, the `errors.As(err, &rre)` case becomes:

```go
	case errors.As(err, &rre):
		// Lift the resolver's diagnostics into the structured findings the
		// Step-0 contract promises, and name each defect's file:line in the
		// human text (change 0403). Disposition and result are unchanged.
		findings := configDiagnosticFindings(rre.Diagnostics)
		out := RepositoryPrepareResult{
			Envelope:    NewEnvelope(OperationRepositoryPrepare, ResultUnsupportedConfig),
			Disposition: PrepareDispositionRefused,
			Findings:    findings,
		}
		out.human = appendConfigFindingBlock(
			fmt.Sprintf("repository prepare: %s: %s", PrepareDispositionRefused, rre.Error()), findings)
		return out
```

(`prepareGatherFailure` also catches `RepoResolutionError`s with nil `Diagnostics` — e.g. a `LoadFilesystemSources` failure; `configDiagnosticFindings(nil)` returns nil and `appendConfigFindingBlock` then returns the bare header, so that path's behavior is byte-identical to today. No `RepositoryPrepareResult.HumanText` change: the explicit `human` string is returned as-is.)

- [ ] **Step 4: Run to verify pass**

Run: `go test -tags integration -count=1 ./internal/app/ -run 'TestIntegrationRepoPrepare' -v` and `go test -count=1 ./internal/app/`
Expected: PASS, existing prepare scenarios included.

- [ ] **Step 5: Commit**

```bash
git add internal/app/configrefusal_integration_test.go internal/app/repository_prepare.go
git commit -m "fix(0403): repository prepare invalid-config refusal carries structured findings

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 6: `repository init` and `repository migrate` refusals carry the findings

**Files:**
- Modify: `internal/app/configrefusal_integration_test.go`
- Modify: `internal/app/repository_init.go` (func `repositoryGatherFailure`)
- Modify: `internal/app/repository_migrate.go` (struct `RepositoryMigrateResult` + func `migrateGatherFailure`)

**Interfaces:**
- Consumes: `assertConfigRefusalFindings`, `configDiagnosticFindings`, `appendConfigFindingBlock`; `r.runInit(t)` (defined in `reposetup_integration_test.go`); `RunRepositoryMigrate(ctx, SetupDeps, MigrateOptions)` — copy the exact options from an existing `TestIntegrationRepoMigration*` test.
- Produces: `RepositoryMigrateResult.Findings []reposetup.Finding` (`json:"findings,omitempty"`) — the one new protocol field of this change.

- [ ] **Step 1: Write the failing integration tests** (append; shard prefixes: init lives in the reposetup shard, so the init test must be named `TestIntegrationRepoSetup*`; migrate in `TestIntegrationRepoMigration*`):

```go
// TestIntegrationRepoSetupInitInvalidConfigDiagnostics: init still refuses
// unsupported-config, now with the lifted findings and human refs.
func TestIntegrationRepoSetupInitInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := r.runInit(t)

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	human := res.HumanText()
	if !strings.HasPrefix(human, string(OperationRepositoryInit)+": "+string(ResultUnsupportedConfig)+": ") {
		t.Errorf("human header changed: %q", human)
	}
	assertConfigRefusalFindings(t, res.Findings, human)
}

// TestIntegrationRepoMigrationInvalidConfigDiagnostics: migrate still refuses
// unsupported-config, now with the lifted findings (a new field on its result
// document) and human refs.
func TestIntegrationRepoMigrationInvalidConfigDiagnostics(t *testing.T) {
	r := newInitRepo(t, invalidConfigYML, nil)
	res := RunRepositoryMigrate(context.Background(), SetupDeps{Git: newGitClient(t), RepoDir: r.invocation}, MigrateOptions{})

	if res.Result != ResultUnsupportedConfig {
		t.Fatalf("result = %q (%s), want %q", res.Result, res.HumanText(), ResultUnsupportedConfig)
	}
	human := res.HumanText()
	assertConfigRefusalFindings(t, res.Findings, human)
}
```

(Verify the `OperationRepositoryInit` header spelling against `repositoryGatherFailure`'s `fmt.Sprintf` — it prints the `operation` parameter; use whatever string the existing init human text starts with. Same for `MigrateOptions` — copy a minimal construction from `repomigration_integration_test.go`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration -count=1 ./internal/app/ -run 'TestIntegrationRepoSetupInitInvalidConfig|TestIntegrationRepoMigrationInvalidConfig' -v`
Expected: init FAILs on empty findings; migrate FAILs to compile (`res.Findings` undefined — the field does not exist yet).

- [ ] **Step 3: Implement.** In `internal/app/repository_migrate.go`, add to `RepositoryMigrateResult` (after `PendingLocal`):

```go
	// Findings carries the diagnosis a refusal lifts from the resolver — one
	// finding per config diagnostic (change 0403); empty on success.
	Findings []reposetup.Finding `json:"findings,omitempty"`
```

In `repositoryGatherFailure` (`repository_init.go`), the `errors.As(err, &rre)` branch becomes:

```go
	if errors.As(err, &rre) {
		findings := configDiagnosticFindings(rre.Diagnostics)
		out := newRepositoryOpResult(operation, ResultUnsupportedConfig, RepositoryOpResult{Findings: findings})
		out.human = appendConfigFindingBlock(
			fmt.Sprintf("%s: %s: %s", operation, ResultUnsupportedConfig, rre.Error()), findings)
		return out
	}
```

In `migrateGatherFailure` (`repository_migrate.go`), the `errors.As(err, &rre)` branch becomes:

```go
	if errors.As(err, &rre) {
		findings := configDiagnosticFindings(rre.Diagnostics)
		out := newMigrateResult(ResultUnsupportedConfig, RepositoryMigrateResult{Findings: findings})
		out.human = appendConfigFindingBlock(
			fmt.Sprintf("%s: %s: %s", OperationRepositoryMigrate, ResultUnsupportedConfig, rre.Error()), findings)
		return out
	}
```

(Both keep the nil-diagnostics path byte-identical, same as Task 5.)

- [ ] **Step 4: Run to verify pass, including the schema surface**

Run: `go test -tags integration -count=1 ./internal/app/ -run 'TestIntegrationRepoSetup|TestIntegrationRepoMigration' -v`, then `go test -count=1 ./internal/app/ ./internal/cli/`
Expected: PASS. Watch `schema_tags_test.go` and `internal/cli/schema_command_test.go` specifically — the new `Findings` field on the migrate document is derived from the live Go type; if a derived-surface test reds, follow its own remedy (regenerate/extend the derived expectation), never hand-edit a schema artefact against the type.

- [ ] **Step 5: Commit**

```bash
git add internal/app/configrefusal_integration_test.go internal/app/repository_init.go internal/app/repository_migrate.go
git commit -m "fix(0403): init and migrate invalid-config refusals carry findings (migrate result gains findings field)

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 7: `status` refusal carries the findings

**Files:**
- Modify: `internal/app/operational_context_test.go` (or a new untagged sibling `internal/app/config_refusal_status_test.go` if the file is crowded)
- Modify: `internal/app/status.go` (func `statusFailure`)

**Interfaces:**
- Consumes: `ConfigDiagnostics` (Task 1), `configDiagnosticStatusFindings` (Task 2); untagged git fixture helpers from `status_git_test.go` (`requireRealGit`, `runGit`, `gitIdentity`, `writeRepoFile`, `newGitClient`, `NewGitStatusReader`, and the `gitRepo` struct — mirror `newLegacyRepo`'s body).

- [ ] **Step 1: Write the failing test.** This test is untagged (plain `go test`), so it must NOT carry a `TestIntegration` prefix, and it cannot use the tagged `newInitRepo`; build the fixture the way `newLegacyRepo` does (same file's helpers), with the invalid `.docket.yml` committed and pushed on main — status resolves config from the pinned default-branch blob:

```go
// TestStatusInvalidConfigDiagnostics: an invalid committed .docket.yml still
// refuses with reason invalid-input and today's message, and now carries the
// resolver's findings — code, .docket.yml:<line> in the path slot — with the
// refs in the human text (change 0403).
func TestStatusInvalidConfigDiagnostics(t *testing.T) {
	requireRealGit(t)
	root := testsupport.TempDir(t)
	origin := filepath.Join(root, "origin.git")
	writer := filepath.Join(root, "writer")
	invocation := filepath.Join(root, "invocation")
	runGit(t, root, "init", "--bare", "-b", "main", origin)
	runGit(t, root, "init", "-b", "main", writer)
	gitIdentity(t, writer)
	writeRepoFile(t, writer, ".docket.yml", invalidConfigYML)
	writeRepoFile(t, writer, "README.md", "readme\n")
	runGit(t, writer, "add", "-A")
	runGit(t, writer, "commit", "-q", "-m", "invalid config")
	runGit(t, writer, "remote", "add", "origin", origin)
	runGit(t, writer, "push", "-q", "-u", "origin", "main")
	runGit(t, root, "clone", "-q", origin, invocation)

	res := Status(context.Background(), NewGitStatusReader(newGitClient(t)), StatusOptions{RepoDir: invocation})

	if res.Reason != "invalid-input" {
		t.Fatalf("reason = %q (message %q), want invalid-input", res.Reason, res.Message)
	}
	if !strings.Contains(res.Message, "invalid configuration") {
		t.Errorf("message = %q, want it to keep today's invalid-configuration text", res.Message)
	}
	var errs []StatusFinding
	for _, f := range res.Findings {
		if f.Severity == "error" {
			errs = append(errs, f)
		}
	}
	if len(errs) != 3 {
		t.Fatalf("error findings = %d (%+v), want 3", len(errs), res.Findings)
	}
	wantRefs := map[string]string{
		"unknown-key":   ".docket.yml:2",
		"invalid-type":  ".docket.yml:6",
		"invalid-value": ".docket.yml:7",
	}
	for _, f := range errs {
		if want, ok := wantRefs[f.Code]; !ok || f.Path != want {
			t.Errorf("finding %q path = %q, want %q", f.Code, f.Path, wantRefs[f.Code])
		}
	}
	human := res.HumanText()
	for _, ref := range wantRefs {
		if !strings.Contains(human, ref) {
			t.Errorf("human text lacks ref %q:\n%s", ref, human)
		}
	}
}
```

(Imports as the host file already has them: `context`, `path/filepath`, `strings`, `testing`, plus the `testsupport` package `status_git_test.go` uses — copy its import path. The `res.Result` for this path is whatever `classifyStatusError` returns today — the test deliberately pins only reason/message, the spec's stability contract.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/app/ -run TestStatusInvalidConfigDiagnostics -v`
Expected: FAIL on empty findings (reason/message asserts pass — Task 1 preserved the classification and text; the human refs assert also fails).

- [ ] **Step 3: Implement.** In `internal/app/status.go`, `statusFailure`, extend the fallthrough branch (the `errRepositoryNotOperational` branch above it is untouched):

```go
	result, reason := classifyStatusError(ctx, err)
	out := StatusResult{
		Context: contextFromPin(pin),
		Reason:  reason,
		Message: err.Error(),
	}
	// An invalid-configuration refusal carries the resolver's diagnostics:
	// lift them into the findings array so the refusal names each defect's
	// file:line (change 0403). Reason and Message are decided above, exactly
	// as before, never by the lifted findings.
	if diags := ConfigDiagnostics(err); len(diags) > 0 {
		out.Findings = configDiagnosticStatusFindings(diags)
	}
	return NewStatusResult(result, out)
```

- [ ] **Step 4: Run to verify pass**

Run: `go test -count=1 ./internal/app/ -run 'TestStatus|TestOperationalGate' -v`, then the whole package `go test -count=1 ./internal/app/`
Expected: PASS. The human refs come free: `StatusResult.HumanText`'s health section prints every finding via `writeFinding`, whose `findingLocator` falls through to `f.Path` — which now carries `.docket.yml:<line>`.

- [ ] **Step 5: Commit**

```bash
git add internal/app/status.go internal/app/operational_context_test.go
git commit -m "fix(0403): status invalid-config refusal lifts resolver diagnostics into findings

Claude-Session: https://claude.ai/code/session_01Rs21XXhyRSmUBsD7AJbBBV"
```

---

### Task 8: Mutation evidence and the suite gate

**Files:**
- No new source. Produces build-evidence records only (the mutation probes are run once and recorded, not kept as tests — spec §Testing).

- [ ] **Step 1: Mutation A — a mapper's lift removed reddens that command's test.** For ONE mapper (use `statusFailure`):

```bash
cd <feature-worktree>
cp internal/app/status.go internal/app/status.go.bak
# Mutate: neutralize the lift — change the guard to `if false {` on the
# `if diags := ConfigDiagnostics(err); len(diags) > 0 {` line, e.g.:
#   perl -pi -e 's/if diags := ConfigDiagnostics\(err\); len\(diags\) > 0 \{/if diags := ConfigDiagnostics(err); false \&\& len(diags) > 0 {/' internal/app/status.go
git diff --stat internal/app/status.go   # MUST show the file changed before reading any result
go test -count=1 ./internal/app/ -run TestStatusInvalidConfigDiagnostics
# Expected: FAIL ("error findings = 0 ..., want 3")
mv internal/app/status.go.bak internal/app/status.go
go test -count=1 ./internal/app/ -run TestStatusInvalidConfigDiagnostics   # green again
```

Record the red output in the build evidence. (If the perl pattern fails to match it exits 0 having changed nothing — the `git diff` gate above is mandatory before believing any reading.)

- [ ] **Step 2: Mutation B — `resolveSetupConfig` leaving `Diagnostics` unpopulated reddens all four repository-family tests.**

```bash
cp internal/app/repository_facts.go internal/app/repository_facts.go.bak
# Mutate: drop the Diagnostics pass-through at the config.Resolve wrap site:
#   perl -pi -e 's/\{Reason: ReasonInvalidConfig, Err: err, Diagnostics: diags\}/\{Reason: ReasonInvalidConfig, Err: err\}/' internal/app/repository_facts.go
git diff --stat internal/app/repository_facts.go   # MUST show a change
go test -tags integration -count=1 ./internal/app/ -run 'InvalidConfigDiagnostics' -v
# Expected: all four family tests FAIL (check, prepare, setup-init, migration) on empty findings;
# TestStatusInvalidConfigDiagnostics is untagged and unaffected by this probe.
mv internal/app/repository_facts.go.bak internal/app/repository_facts.go
go test -tags integration -count=1 ./internal/app/ -run 'InvalidConfigDiagnostics'   # green again
```

Record the four reds in the build evidence.

- [ ] **Step 3: Full builds and the whole-package runs**

Run: `go build ./... && go test -count=1 ./... && go test -tags integration -count=1 ./internal/app/ -run 'TestIntegrationRepoCheck|TestIntegrationRepoPrepare|TestIntegrationRepoSetup|TestIntegrationRepoMigration'`
Expected: green throughout.

- [ ] **Step 4: Budget screening.** The four new integration tests each build a bare-origin + clone fixture inside already-budgeted shards (`test_go_integration_app_repocheck.sh`, `..._repoprepare.sh`, `..._reposetup.sh`, `..._repomigration.sh` — see `tests/runtime-budgets.tsv`). At the build gate, read the suite's budget clause lines for those rows: a `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` line is a screening finding to note in results; a `SERIAL CONFIRMED OVER BUDGET:` line must be acted on (confirm serially before treating it as a breach).

- [ ] **Step 5: Suite gate.** Run the whole suite via the configured `build.test_command` (today `go run ./cmd/docket development test`, from the feature worktree). This is docket-build's final gate; nothing else runs the budget rows or the shard-completeness contract (`test_go_integration_contract.sh`, which verifies the four new tagged tests each match exactly one shard prefix and the untagged status test none).

- [ ] **Step 6: Commit any evidence artifacts per docket-build's contract** (no source changes expected in this task).

---

## Self-Review (performed at plan time)

- **Spec coverage:** §1 error carriage → Task 1; §2 lift + five mappers → Tasks 2, 4, 5, 6, 7; §3 shared human line + `diagnostic config` → Task 3; §Testing fixture/integration/unit/mutation → Tasks 2 (fixture semantics probe + unit), 4–7 (integration), 8 (mutation + suite). Non-goals respected: `internal/config` untouched; no severity relaxation; no new `diagnostics` array; no remedy pointer at `diagnostic config`.
- **Deviations from spec letter, argued:** (a) migrate's missing `Findings` field is added (spec assumed it existed — discrepancy #1); (b) refusal human text keeps today's full header line (spec §2 normative wording) rather than §3's example header (discrepancy #2); (c) status projection routes severity through `normalizeSeverity` so the `info → notice` DTO vocabulary holds — for the spec's own error/warning cases this is verbatim, as required; (d) the spec's "operation's existing finding block" for prepare/init/migrate does not exist on their refusal paths (they print single-line humans), so the shared `appendConfigFindingBlock` — byte-identical to check's block — is used for all four, which is the spec's stated intent ("one human rendering... the same block the healthy-path HumanText already emits").
- **Type consistency:** `ConfigDiagnostics(err) []config.Diagnostic` (Tasks 1→4–7); `configDiagnosticFindings(diags) []reposetup.Finding` and `configDiagnosticStatusFindings(diags) []StatusFinding` (Tasks 2→4–7); `appendConfigFindingBlock(header string, findings []reposetup.Finding) string` (Tasks 2→4–6); `ConfigDiagnosticLine(d config.Diagnostic) string` (Task 3); `invalidConfigYML` const (Task 2→4–7); `assertConfigRefusalFindings(t, []reposetup.Finding, string)` (Task 4→5–6).
- **Known verification points for the executor** (exact spellings to check against the tree, flagged inline in their tasks): `LayerRepository`-style constant names in `internal/config`; `Result` comparison types; `MigrateOptions`/`PrepareOptions` minimal construction; the `testsupport` import path; existing `diagnostic config` human-format asserts in `config_test.go`.
