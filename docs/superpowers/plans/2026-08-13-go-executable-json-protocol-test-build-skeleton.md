<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0304 — Go executable, JSON protocol, and test/build skeleton](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0304-go-executable-json-protocol-test-build-skeleton.md)**
<!-- docket:backlink:end -->

# Go executable, JSON protocol, and test/build skeleton — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the smallest independently buildable Go Docket — one module, a Cobra `docket` executable with `version` and `diagnostic runtime`, the protocol-v1 result envelope and taxonomy, a text/JSON presenter with exit mapping, four-tuple buildability, the fixture convention, and a real `tests/test_*.sh` producer wiring the Go checks into the existing whole-suite gate.

**Architecture:** `cmd/docket` is the only `os.Exit` site; `internal/app` owns typed results (envelope + taxonomy + the two read-only operations); `internal/buildinfo` owns injected build identity and runtime facts; `internal/cli` is an inward-facing Cobra adapter with one application-owned presenter and a deliberately narrow pre-Cobra `--json` transport scan. Business behavior and protocol structs do not depend on Cobra.

**Tech Stack:** Go 1.26 (`toolchain go1.26.5`), Cobra v1.10.2 (sole direct dependency), the repo's existing Bash test suite (`scripts/run-tests.sh` + `tests/runtime-budgets.tsv`).

**Spec:** `.docket/docs/superpowers/specs/2026-08-13-go-executable-json-protocol-test-build-skeleton-design.md` (metadata worktree; docket-mode — the spec lives on the `docket` branch, not this feature branch).

## Global Constraints

- Module path `github.com/danielhanold/docket`, at the repository root. Language line `go 1.26.0`; `toolchain go1.26.5`.
- Cobra pinned at **v1.10.2**; commit `go.sum`; do NOT vendor. No Viper, no DI framework, no logging framework, no `cobra-cli` generated scaffolding.
- Protocol v1: every result begins `protocol_version` (numeric `1`), `operation` (string), `result` (string from the 11-value taxonomy), encoded in that order. Operation-specific fields are typed and top-level (no generic `data` wrapper).
- JSON mode emits **exactly one** compact UTF-8 JSON document on stdout, one trailing newline, for successful AND handled-unsuccessful outcomes. Never banners, progress, help, usage, or a second value. A handled JSON result is not duplicated on stderr.
- Exit mapping: `applied`/`no-op` → 0, `invalid-input` → 2, every other non-success result → 1.
- Human parse failures: one `docket: ...` line on stderr, stdout empty, exit 2. Human help: Cobra-rendered on stdout, exit 0.
- JSON and help are mutually exclusive: `--json` with `--help`/`-h`/`help` → one `invalid-input` document, reason `json-help-conflict`, exit 2, no help text.
- Approved target tuples: `darwin/amd64`, `darwin/arm64`, `linux/amd64`, `linux/arm64`. Cross-compiling them is a buildability gate only (`CGO_ENABLED=0`).
- Development build identity literals: `development` / `unknown` / `unknown`.
- Checks that must pass: `gofmt` clean, `go vet ./...`, `go test ./...`, the four cross-builds, and the whole suite via `scripts/run-tests.sh` (the resolved `finalize.test_command`).
- Repo shell rules (AGENTS.md) bind every shell step: no producer-pipe-into-early-exit under pipefail; `mv -f` on install paths; templated `mktemp`; mutation tests restore from a `cp` backup and prove the mutation landed with `/usr/bin/grep -cF` counts (never PATH `grep`, which is ugrep).

## File Structure

```text
go.mod, go.sum                      module + pins (Task 1)
cmd/docket/main.go                  entry point, sole os.Exit (Task 5)
cmd/docket/main_test.go             built-binary subprocess + cross-build tests (Task 5)
internal/buildinfo/buildinfo.go     injected identity + runtime facts (Task 1)
internal/buildinfo/buildinfo_test.go
internal/app/result.go              envelope, taxonomy, exit mapping (Task 2)
internal/app/version.go             version operation (Task 2)
internal/app/runtime.go             diagnostic.runtime operation (Task 2)
internal/app/clierror.go            CLI-failure result shape (Task 2)
internal/app/*_test.go
internal/cli/jsonmode.go            pre-Cobra --json transport scan (Task 3)
internal/cli/jsonmode_test.go
internal/cli/presenter.go           the one protocol write site (Task 4)
internal/cli/root.go                Cobra tree + Run() (Task 4)
internal/cli/*_test.go
testdata/README.md                  fixture-convention record (Task 7)
tests/test_go_toolchain.sh          suite producer for the Go checks (Task 8)
tests/runtime-budgets.tsv           + one row (Task 8)
tests/test_runtime_budgets.sh       EXPECTED_TOTAL re-seed (Task 8)
```

All commits happen in this feature worktree on `feat/go-executable-json-protocol-test-build-skeleton`. Never touch docket metadata (change files, BOARD.md, ADRs) from here.

---

### Task 1: Go module + `internal/buildinfo`

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `internal/buildinfo/buildinfo.go`
- Test: `internal/buildinfo/buildinfo_test.go`

**Interfaces:**
- Produces: `buildinfo.Info{Version, Commit, BuildDate string}`, `buildinfo.Current() Info`, `buildinfo.RuntimeFacts{GoVersion, GOOS, GOARCH string}`, `buildinfo.CurrentRuntime() RuntimeFacts`, package vars `Version`, `Commit`, `BuildDate` (the `-X` seam).

- [ ] **Step 1: Initialize the module and pin Cobra**

Run from the repo root (this worktree):

```bash
go mod init github.com/danielhanold/docket
go get github.com/spf13/cobra@v1.10.2
```

Then edit `go.mod` so the version lines read exactly:

```text
go 1.26.0

toolchain go1.26.5
```

Cobra is not yet imported by any file, so `go mod tidy` would drop it — do NOT run `go mod tidy` until Task 4 imports Cobra. Expected `go.mod` requires: `github.com/spf13/cobra v1.10.2` plus indirect `github.com/spf13/pflag v1.0.9` and `github.com/inconshreveable/mousetrap v1.1.0`.

- [ ] **Step 2: Write the failing test**

`internal/buildinfo/buildinfo_test.go`:

```go
package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentDevelopmentDefaults(t *testing.T) {
	info := Current()
	if info.Version != "development" || info.Commit != "unknown" || info.BuildDate != "unknown" {
		t.Fatalf("development defaults wrong: %+v", info)
	}
}

func TestCurrentReflectsInjectedVars(t *testing.T) {
	origV, origC, origD := Version, Commit, BuildDate
	defer func() { Version, Commit, BuildDate = origV, origC, origD }()
	Version, Commit, BuildDate = "1.2.3", "abc1234", "2026-08-13"
	info := Current()
	if info.Version != "1.2.3" || info.Commit != "abc1234" || info.BuildDate != "2026-08-13" {
		t.Fatalf("injected identity not reflected: %+v", info)
	}
}

func TestCurrentRuntimeMatchesHost(t *testing.T) {
	f := CurrentRuntime()
	if f.GoVersion != runtime.Version() || f.GOOS != runtime.GOOS || f.GOARCH != runtime.GOARCH {
		t.Fatalf("runtime facts diverge from host: %+v", f)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/buildinfo/`
Expected: FAIL (compile error — package has no non-test source).

- [ ] **Step 4: Implement**

`internal/buildinfo/buildinfo.go`:

```go
// Package buildinfo owns injected build identity and running-toolchain facts.
package buildinfo

import "runtime"

// Development-build defaults. A release build may override each via the Go
// linker, e.g.:
//
//	go build -ldflags "-X github.com/danielhanold/docket/internal/buildinfo.Version=v1.0.0 \
//	  -X github.com/danielhanold/docket/internal/buildinfo.Commit=<sha> \
//	  -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=<date>" ./cmd/docket
//
// This change documents and tests that seam; release packaging is change 0317's.
var (
	Version   = "development"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// Info is the build identity the version operation reports.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
}

// Current returns the identity of this binary.
func Current() Info { return Info{Version: Version, Commit: Commit, BuildDate: BuildDate} }

// RuntimeFacts are the running toolchain and target tuple. They are a value,
// not live reads, so operations stay deterministic under test injection.
type RuntimeFacts struct {
	GoVersion string
	GOOS      string
	GOARCH    string
}

// CurrentRuntime reads the facts of the running binary.
func CurrentRuntime() RuntimeFacts {
	return RuntimeFacts{GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/buildinfo/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/buildinfo/
git commit -m "feat(0304): Go module, Cobra pin, and buildinfo identity seam"
```

---

### Task 2: `internal/app` — envelope, taxonomy, exit mapping, two operations

**Files:**
- Create: `internal/app/result.go`, `internal/app/version.go`, `internal/app/runtime.go`, `internal/app/clierror.go`
- Test: `internal/app/result_test.go`, `internal/app/version_test.go`, `internal/app/runtime_test.go`

**Interfaces:**
- Consumes: `buildinfo.Info`, `buildinfo.RuntimeFacts` (Task 1).
- Produces: `app.Result` (string type) + 11 constants + `app.AllResults []Result`; `app.ProtocolVersion = 1`; `app.Envelope{ProtocolVersion int; Operation string; Result Result}` with `Env() Envelope`; `app.OperationResult interface { Env() Envelope; HumanText() string }`; `app.ExitCode(Result) int`; `app.Version(buildinfo.Info) VersionResult`; `app.DiagnosticRuntime(buildinfo.RuntimeFacts) RuntimeResult`; `app.CLIError(reason, message string) CLIErrorResult`; reason constants `app.ReasonInvalidArguments = "invalid-arguments"`, `app.ReasonJSONHelpConflict = "json-help-conflict"`.

- [ ] **Step 1: Write the failing tests**

`internal/app/result_test.go`:

```go
package app

import (
	"encoding/json"
	"testing"
)

func TestResultTaxonomySpellings(t *testing.T) {
	want := []string{
		"applied", "no-op", "contended", "invalid-input", "invalid-state",
		"blocked", "unsupported-config", "gate-failed", "external-failed",
		"interrupted", "internal-error",
	}
	if len(AllResults) != len(want) {
		t.Fatalf("taxonomy has %d results, want %d", len(AllResults), len(want))
	}
	for i, w := range want {
		if string(AllResults[i]) != w {
			t.Fatalf("AllResults[%d] = %q, want %q", i, AllResults[i], w)
		}
	}
}

func TestExitCodeMapping(t *testing.T) {
	for _, r := range AllResults {
		got := ExitCode(r)
		var want int
		switch r {
		case ResultApplied, ResultNoOp:
			want = 0
		case ResultInvalidInput:
			want = 2
		default:
			want = 1
		}
		if got != want {
			t.Fatalf("ExitCode(%q) = %d, want %d", r, got, want)
		}
	}
}

func TestEnvelopeFieldNamesAndOrder(t *testing.T) {
	b, err := json.Marshal(NewEnvelope("op.name", ResultApplied))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"op.name","result":"applied"}`
	if string(b) != want {
		t.Fatalf("envelope encoding = %s, want %s", b, want)
	}
}
```

`internal/app/version_test.go`:

```go
package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestVersionDevelopmentTextAndJSON(t *testing.T) {
	r := Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"})
	if got, want := r.HumanText(), "docket development (commit unknown, built unknown)"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}

func TestVersionInjectedIdentity(t *testing.T) {
	r := Version(buildinfo.Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-08-13"})
	if got, want := r.HumanText(), "docket 1.2.3 (commit abc1234, built 2026-08-13)"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
```

`internal/app/runtime_test.go`:

```go
package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestDiagnosticRuntimeSupportedTuple(t *testing.T) {
	r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "darwin", GOARCH: "arm64"})
	if !r.SupportedTarget {
		t.Fatal("darwin/arm64 must be a supported target")
	}
	if got, want := r.HumanText(), "go_version: go1.26.5\ngo_os: darwin\ngo_arch: arm64\nsupported_target: true"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}

func TestDiagnosticRuntimeUnsupportedTupleStillApplied(t *testing.T) {
	r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "windows", GOARCH: "amd64"})
	if r.SupportedTarget {
		t.Fatal("windows/amd64 must not be a supported target")
	}
	if r.Result != ResultApplied {
		t.Fatalf("inspection on an unsupported tuple is still applied, got %q", r.Result)
	}
}

func TestAllFourApprovedTuples(t *testing.T) {
	for _, tuple := range [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
		r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: tuple[0], GOARCH: tuple[1]})
		if !r.SupportedTarget {
			t.Fatalf("%s/%s must be supported", tuple[0], tuple[1])
		}
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/app/`
Expected: FAIL (compile error — package does not exist).

- [ ] **Step 3: Implement**

`internal/app/result.go`:

```go
// Package app owns application results: the protocol-v1 envelope, the result
// taxonomy, exit mapping, and the read-only operations of change 0304. It has
// no dependency on Cobra or any presentation concern beyond HumanText.
package app

// ProtocolVersion is fixed at 1 for this protocol generation. JSON field
// names and types are protocol: removing, renaming, or retyping a field
// requires a later protocol version; adding operation-specific fields is
// compatible within v1.
const ProtocolVersion = 1

// Result is one spelling from the protocol-v1 result taxonomy.
type Result string

const (
	ResultApplied           Result = "applied"
	ResultNoOp              Result = "no-op"
	ResultContended         Result = "contended"
	ResultInvalidInput      Result = "invalid-input"
	ResultInvalidState      Result = "invalid-state"
	ResultBlocked           Result = "blocked"
	ResultUnsupportedConfig Result = "unsupported-config"
	ResultGateFailed        Result = "gate-failed"
	ResultExternalFailed    Result = "external-failed"
	ResultInterrupted       Result = "interrupted"
	ResultInternalError     Result = "internal-error"
)

// AllResults enumerates the complete v1 taxonomy, in documentation order.
var AllResults = []Result{
	ResultApplied, ResultNoOp, ResultContended, ResultInvalidInput,
	ResultInvalidState, ResultBlocked, ResultUnsupportedConfig,
	ResultGateFailed, ResultExternalFailed, ResultInterrupted,
	ResultInternalError,
}

// Envelope carries the three fields every protocol-v1 result begins with.
// Operation-specific result structs embed it; reserved envelope names cannot
// be shadowed by an operation's own fields.
type Envelope struct {
	ProtocolVersion int    `json:"protocol_version"`
	Operation       string `json:"operation"`
	Result          Result `json:"result"`
}

// NewEnvelope builds the envelope for one operation outcome.
func NewEnvelope(operation string, result Result) Envelope {
	return Envelope{ProtocolVersion: ProtocolVersion, Operation: operation, Result: result}
}

// Env returns the envelope; embedding gives every result struct this method.
func (e Envelope) Env() Envelope { return e }

// OperationResult is a fully-computed operation outcome the presenter can
// render as protocol JSON or as human text.
type OperationResult interface {
	Env() Envelope
	HumanText() string
}

// ExitCode maps a result to the deliberately coarse process exit status.
// JSON consumers use result, not the exit code.
func ExitCode(r Result) int {
	switch r {
	case ResultApplied, ResultNoOp:
		return 0
	case ResultInvalidInput:
		return 2
	default:
		return 1
	}
}
```

`internal/app/version.go`:

```go
package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// VersionResult reports injected build identity.
type VersionResult struct {
	Envelope
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Version is the `docket version` operation.
func Version(info buildinfo.Info) VersionResult {
	return VersionResult{
		Envelope:  NewEnvelope("version", ResultApplied),
		Version:   info.Version,
		Commit:    info.Commit,
		BuildDate: info.BuildDate,
	}
}

// HumanText renders the one-line default text form.
func (r VersionResult) HumanText() string {
	return fmt.Sprintf("docket %s (commit %s, built %s)", r.Version, r.Commit, r.BuildDate)
}
```

`internal/app/runtime.go`:

```go
package app

import (
	"fmt"

	"github.com/danielhanold/docket/internal/buildinfo"
)

// approvedTargets is the released-product tuple set. supported_target: false
// is data under an applied result, not an inspection failure.
var approvedTargets = map[string]struct{}{
	"darwin/amd64": {},
	"darwin/arm64": {},
	"linux/amd64":  {},
	"linux/arm64":  {},
}

// RuntimeResult reports the running toolchain and target tuple.
type RuntimeResult struct {
	Envelope
	GoVersion       string `json:"go_version"`
	GoOS            string `json:"go_os"`
	GoArch          string `json:"go_arch"`
	SupportedTarget bool   `json:"supported_target"`
}

// DiagnosticRuntime is the `docket diagnostic runtime` operation. It reads
// only the injected facts — never the repository, configuration, Git, or gh.
func DiagnosticRuntime(facts buildinfo.RuntimeFacts) RuntimeResult {
	_, supported := approvedTargets[facts.GOOS+"/"+facts.GOARCH]
	return RuntimeResult{
		Envelope:        NewEnvelope("diagnostic.runtime", ResultApplied),
		GoVersion:       facts.GoVersion,
		GoOS:            facts.GOOS,
		GoArch:          facts.GOARCH,
		SupportedTarget: supported,
	}
}

// HumanText renders the four labeled lines in stable order.
func (r RuntimeResult) HumanText() string {
	return fmt.Sprintf("go_version: %s\ngo_os: %s\ngo_arch: %s\nsupported_target: %t",
		r.GoVersion, r.GoOS, r.GoArch, r.SupportedTarget)
}
```

`internal/app/clierror.go`:

```go
package app

// Stable machine reasons for CLI-level failures. Message is explanatory prose
// and must not be parsed; the framework's error text may improve freely.
const (
	ReasonInvalidArguments = "invalid-arguments"
	ReasonJSONHelpConflict = "json-help-conflict"
)

// CLIErrorResult is the stable shape for CLI parsing and usage failures.
type CLIErrorResult struct {
	Envelope
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// CLIError builds an invalid-input result under the "cli" operation name.
func CLIError(reason, message string) CLIErrorResult {
	return CLIErrorResult{
		Envelope: NewEnvelope("cli", ResultInvalidInput),
		Reason:   reason,
		Message:  message,
	}
}

// HumanText renders the human-mode diagnostic line (routed to stderr by the
// presenter; stdout stays empty on human parse failures).
func (r CLIErrorResult) HumanText() string { return "docket: " + r.Message }
```

- [ ] **Step 4: Run to verify they pass**

Run: `go test ./internal/app/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/
git commit -m "feat(0304): protocol-v1 envelope, result taxonomy, exit mapping, and the two operations"
```

---

### Task 3: `internal/cli/jsonmode.go` — the bounded transport scan

**Files:**
- Create: `internal/cli/jsonmode.go`
- Test: `internal/cli/jsonmode_test.go`

**Interfaces:**
- Produces: `cli.DetectJSONMode(args []string) bool`.

- [ ] **Step 1: Write the failing table test**

`internal/cli/jsonmode_test.go`:

```go
package cli

import "testing"

func TestDetectJSONMode(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"empty", nil, false},
		{"no flag", []string{"version"}, false},
		{"before command", []string{"--json", "version"}, true},
		{"after command", []string{"version", "--json"}, true},
		{"explicit true", []string{"version", "--json=true"}, true},
		{"explicit false", []string{"version", "--json=false"}, false},
		{"last recognized wins", []string{"--json", "version", "--json=false"}, false},
		{"false then true", []string{"--json=false", "--json"}, true},
		{"stops at standalone double dash", []string{"version", "--", "--json"}, false},
		{"recognized before the boundary", []string{"--json", "--", "--json=false"}, true},
		{"after a malformed token", []string{"version", "--bogus", "--json"}, true},
		{"unrecognized json spellings ignored", []string{"--json=1", "-json", "--jsonx"}, false},
	}
	for _, c := range cases {
		if got := DetectJSONMode(c.args); got != c.want {
			t.Fatalf("%s: DetectJSONMode(%v) = %v, want %v", c.name, c.args, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL (compile error — package does not exist).

- [ ] **Step 3: Implement**

`internal/cli/jsonmode.go`:

```go
// Package cli is Docket's inward-facing Cobra adapter: command tree,
// output-mode bootstrap, presenters, and exit mapping. It is not a supported
// Go API.
package cli

// DetectJSONMode is the output-transport selection step, not a second
// argument parser. Output mode must be known even when ordinary parsing stops
// before Cobra reaches --json (e.g. `docket version --bogus --json`), so this
// deliberately narrow scan runs over raw arguments before Cobra executes.
// Its bounded grammar:
//   - it recognizes only --json, --json=true, and --json=false;
//   - the last recognized value before a standalone -- selects the mode;
//   - it stops at the first standalone --;
//   - it neither validates, removes, reorders, nor interprets any other
//     argument. Cobra still performs all command and flag validation.
func DetectJSONMode(args []string) bool {
	mode := false
	for _, a := range args {
		switch a {
		case "--":
			return mode
		case "--json", "--json=true":
			mode = true
		case "--json=false":
			mode = false
		}
	}
	return mode
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/cli/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat(0304): bounded --json transport scan ahead of Cobra"
```

---

### Task 4: `internal/cli` — presenter and Cobra tree

**Files:**
- Create: `internal/cli/presenter.go`, `internal/cli/root.go`
- Test: `internal/cli/presenter_test.go`, `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `app.OperationResult`, `app.Version`, `app.DiagnosticRuntime`, `app.CLIError`, `app.ExitCode`, reason constants (Task 2); `DetectJSONMode` (Task 3); `buildinfo.Info`, `buildinfo.RuntimeFacts` (Task 1).
- Produces: `cli.Run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts) int` — Task 5's `main` calls exactly this; `cli.Presenter{Stdout, Stderr io.Writer; JSON bool}` with `Present(app.OperationResult) int` and `PresentHumanError(app.CLIErrorResult) int`.

- [ ] **Step 1: Write the failing presenter test**

`internal/cli/presenter_test.go`:

```go
package cli

import (
	"bytes"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestPresentJSONWritesOneCompactDocument(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: true}
	code := p.Present(app.Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr must be empty, got %q", errBuf.String())
	}
}

func TestPresentHumanTextLine(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: false}
	code := p.Present(app.Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}))
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out.String() != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("stdout = %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("stderr must be empty, got %q", errBuf.String())
	}
}

func TestPresentJSONCLIErrorGoesToStdoutOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: true}
	code := p.Present(app.CLIError(app.ReasonInvalidArguments, "unknown flag: --bogus"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	want := `{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments","message":"unknown flag: --bogus"}` + "\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("a handled JSON result must not duplicate on stderr, got %q", errBuf.String())
	}
}

func TestPresentHumanErrorGoesToStderrOnly(t *testing.T) {
	var out, errBuf bytes.Buffer
	p := Presenter{Stdout: &out, Stderr: &errBuf, JSON: false}
	code := p.PresentHumanError(app.CLIError(app.ReasonInvalidArguments, "unknown flag: --bogus"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout must be empty on human parse failure, got %q", out.String())
	}
	if errBuf.String() != "docket: unknown flag: --bogus\n" {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/cli/`
Expected: FAIL with "undefined: Presenter".

- [ ] **Step 3: Implement the presenter**

`internal/cli/presenter.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/danielhanold/docket/internal/app"
)

// Presenter performs the sole protocol write. An operation computes its
// complete result before presentation, so a failure cannot leave partial
// stdout behind.
type Presenter struct {
	Stdout io.Writer
	Stderr io.Writer
	JSON   bool
}

// Present renders one fully-computed result and returns its exit code.
// JSON mode: one compact document plus one newline on stdout, nothing on
// stderr. Human mode: the result's text on stdout.
func (p Presenter) Present(r app.OperationResult) int {
	if p.JSON {
		buf, err := json.Marshal(r)
		if err != nil {
			// Marshal of our own typed structs cannot fail on real inputs;
			// this is a genuinely unexpected diagnostic, so stderr-only.
			fmt.Fprintf(p.Stderr, "docket: internal error: %v\n", err)
			return app.ExitCode(app.ResultInternalError)
		}
		p.Stdout.Write(append(buf, '\n'))
		return app.ExitCode(r.Env().Result)
	}
	fmt.Fprintln(p.Stdout, r.HumanText())
	return app.ExitCode(r.Env().Result)
}

// PresentHumanError routes a handled human-mode CLI failure to stderr and
// leaves stdout empty.
func (p Presenter) PresentHumanError(r app.CLIErrorResult) int {
	fmt.Fprintln(p.Stderr, r.HumanText())
	return app.ExitCode(r.Env().Result)
}
```

Run: `go test ./internal/cli/` — presenter tests PASS.

- [ ] **Step 4: Write the failing command-tree tests**

`internal/cli/root_test.go` (in-process layer; the built-binary subprocess layer is Task 5):

```go
package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func devInfo() buildinfo.Info {
	return buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"}
}

func hostFacts() buildinfo.RuntimeFacts {
	return buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "darwin", GOARCH: "arm64"}
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, strings.NewReader(""), &out, &errBuf, devInfo(), hostFacts())
	return out.String(), errBuf.String(), code
}

func TestVersionHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestVersionJSONFlagPositions(t *testing.T) {
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	for _, args := range [][]string{{"--json", "version"}, {"version", "--json"}} {
		out, errS, code := runCLI(t, args...)
		if code != 0 || errS != "" || out != want {
			t.Fatalf("args=%v out=%q err=%q code=%d", args, out, errS, code)
		}
	}
}

func TestVersionJSONFalseIsHuman(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--json=false")
	if code != 0 || errS != "" || out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestDiagnosticRuntimeJSON(t *testing.T) {
	out, errS, code := runCLI(t, "diagnostic", "runtime", "--json")
	want := `{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}` + "\n"
	if code != 0 || errS != "" || out != want {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestHostileParseOrderStillJSON(t *testing.T) {
	// --json AFTER the failing token: the transport scan, not Cobra, must
	// select JSON mode, or this emits a human error.
	out, errS, code := runCLI(t, "version", "--bogus", "--json")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if errS != "" {
		t.Fatalf("stderr must be empty in JSON mode, got %q", errS)
	}
	if !strings.HasPrefix(out, `{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments",`) {
		t.Fatalf("stdout = %q", out)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

func TestHumanParseErrorStdoutEmpty(t *testing.T) {
	out, errS, code := runCLI(t, "version", "--bogus")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("stderr = %q", errS)
	}
}

func TestJSONHelpConflictThreeForms(t *testing.T) {
	for _, args := range [][]string{
		{"--json", "--help"},
		{"--json", "-h"},
		{"--json", "help"},
	} {
		out, errS, code := runCLI(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		if !strings.Contains(out, `"reason":"json-help-conflict"`) {
			t.Fatalf("args=%v stdout=%q", args, out)
		}
		if strings.Contains(out, "Usage") {
			t.Fatalf("args=%v help text leaked into protocol stream: %q", args, out)
		}
	}
}

func TestHumanHelpIsolation(t *testing.T) {
	out, errS, code := runCLI(t, "--help")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, "Usage") {
		t.Fatalf("human help must render Cobra text, got %q", out)
	}
}

func TestMissingCommand(t *testing.T) {
	out, errS, code := runCLI(t, "--json")
	if code != 2 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("stdout = %q", out)
	}
}

func TestExtraArgumentRejected(t *testing.T) {
	_, _, code := runCLI(t, "version", "extra")
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	out, errS, code := runCLI(t, "version", "extra", "--json")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestUnknownCommandJSON(t *testing.T) {
	out, errS, code := runCLI(t, "--json", "bogus")
	if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
}

func TestNoCompletionCommand(t *testing.T) {
	_, _, code := runCLI(t, "completion")
	if code == 0 {
		t.Fatal("default completion command must be disabled")
	}
	out, _, _ := runCLI(t, "--help")
	if strings.Contains(out, "completion") {
		t.Fatalf("help must not advertise a completion command: %q", out)
	}
}
```

- [ ] **Step 5: Run to verify they fail**

Run: `go test ./internal/cli/`
Expected: FAIL with "undefined: Run".

- [ ] **Step 6: Implement the command tree**

`internal/cli/root.go`:

```go
package cli

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/buildinfo"
)

// Run wires arguments and explicit streams through Cobra to the application
// and presents exactly one outcome. It returns the process exit code; only
// cmd/docket/main.go converts it into os.Exit.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, info buildinfo.Info, facts buildinfo.RuntimeFacts) int {
	jsonMode := DetectJSONMode(args)
	p := Presenter{Stdout: stdout, Stderr: stderr, JSON: jsonMode}

	var result app.OperationResult
	helpConflict := false

	root := &cobra.Command{
		Use:   "docket",
		Short: "docket tracks planned work as changes and records decisions as ADRs",
		// Docket owns error presentation: Cobra must not print errors or
		// usage itself, or the one-document stdout contract breaks.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().Bool("json", false, "emit protocol-v1 JSON on stdout")
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(args)

	// JSON mode and help are mutually exclusive: any help path in JSON mode
	// records a conflict instead of writing help into the protocol stream.
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(c *cobra.Command, a []string) {
		if jsonMode {
			helpConflict = true
			return
		}
		defaultHelp(c, a)
	})

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Report this binary's build identity",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result = app.Version(info)
			return nil
		},
	}

	diagnosticCmd := &cobra.Command{
		Use:   "diagnostic",
		Short: "Read-only diagnostics",
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("missing command")
		},
	}
	runtimeCmd := &cobra.Command{
		Use:   "runtime",
		Short: "Report the running Go toolchain and target tuple",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result = app.DiagnosticRuntime(facts)
			return nil
		},
	}
	diagnosticCmd.AddCommand(runtimeCmd)
	root.AddCommand(versionCmd, diagnosticCmd)

	err := root.Execute()
	switch {
	case helpConflict:
		return p.Present(app.CLIError(app.ReasonJSONHelpConflict,
			"--json cannot be combined with --help, -h, or the help command"))
	case err != nil:
		res := app.CLIError(app.ReasonInvalidArguments, err.Error())
		if jsonMode {
			return p.Present(res)
		}
		return p.PresentHumanError(res)
	case result != nil:
		return p.Present(result)
	default:
		// Human help was rendered by Cobra on stdout; exit 0.
		return 0
	}
}
```

Notes for the implementer:
- Cobra treats `docket bogus` and `docket version extra` as errors when the root has subcommands; both flow through `err != nil`. If `cobra.NoArgs` on `versionCmd` does not reject `extra` (Cobra reports "unknown command" at the root instead), that is fine — either path returns an error, which is what the tests assert.
- `-h`/`--help` set Cobra's internal help flow which calls the help func and returns `nil` from `Execute()` — that is why `helpConflict` is checked before `err`.
- If `runCLI(t, "--json", "help")` does not trip the help func in this Cobra version (the `help` command resolves differently from the flag), add an explicit guard: check `args` for a leading `help` token after flag filtering — but ONLY if the test fails; try the help-func route first and keep whichever the test proves.

- [ ] **Step 7: Run the package tests, then tidy**

Run: `go test ./internal/cli/`
Expected: PASS. Then run `go mod tidy` (Cobra is now imported; the require set must not change beyond promoting Cobra to used) and `go vet ./...`.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/ go.mod go.sum
git commit -m "feat(0304): presenter and Cobra command tree with one-document output contract"
```

---

### Task 5: `cmd/docket` — entry point, subprocess layer, cross-build test

**Files:**
- Create: `cmd/docket/main.go`
- Test: `cmd/docket/main_test.go`

**Interfaces:**
- Consumes: `cli.Run`, `buildinfo.Current`, `buildinfo.CurrentRuntime`.
- Produces: the `docket` executable; nothing later tasks import.

- [ ] **Step 1: Write main.go**

`cmd/docket/main.go`:

```go
// Command docket is the Docket executable. This is the only os.Exit site.
package main

import (
	"os"

	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		buildinfo.Current(), buildinfo.CurrentRuntime()))
}
```

Run: `go build ./cmd/docket && ./docket version && rm -f docket`
Expected: prints `docket development (commit unknown, built unknown)`.

- [ ] **Step 2: Write the failing subprocess tests**

The built-binary layer exists so accidental writes to process-global stdout/stderr are visible — in-process tests (Task 4) cannot see those.

`cmd/docket/main_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "docket-bin-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "docket")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		panic("building test binary: " + err.Error() + "\n" + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	cmd := exec.Command(binPath, args...)
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	code = 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		code = ee.ExitCode()
	}
	return out.String(), errBuf.String(), code
}

// assertOneJSONDocument decodes stdout and proves it is exactly one complete
// JSON value with one trailing newline, returning the decoded object.
func assertOneJSONDocument(t *testing.T, stdout string) map[string]any {
	t.Helper()
	if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("want exactly one newline-terminated document, got %q", stdout)
	}
	dec := json.NewDecoder(strings.NewReader(stdout))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("decoding %q: %v", stdout, err)
	}
	var second any
	if err := dec.Decode(&second); err != io.EOF {
		t.Fatalf("stdout carries a second JSON value: %q", stdout)
	}
	return doc
}

func TestVersionTextGolden(t *testing.T) {
	out, errS, code := run(t, "version")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	if out != "docket development (commit unknown, built unknown)\n" {
		t.Fatalf("stdout = %q", out)
	}
}

func TestVersionJSONGoldenBytes(t *testing.T) {
	out, errS, code := run(t, "version", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}` + "\n"
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
	assertOneJSONDocument(t, out)
}

func TestDiagnosticRuntimeReflectsHost(t *testing.T) {
	out, errS, code := run(t, "diagnostic", "runtime", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	doc := assertOneJSONDocument(t, out)
	// The subprocess runs on this same host, so its facts equal ours.
	if doc["go_version"] != runtime.Version() || doc["go_os"] != runtime.GOOS || doc["go_arch"] != runtime.GOARCH {
		t.Fatalf("runtime facts diverge from host: %v", doc)
	}
	if doc["protocol_version"] != float64(1) || doc["operation"] != "diagnostic.runtime" || doc["result"] != "applied" {
		t.Fatalf("envelope wrong: %v", doc)
	}
}

func TestInjectedBuildIdentity(t *testing.T) {
	injected := filepath.Join(t.TempDir(), "docket-injected")
	ldflags := "-X github.com/danielhanold/docket/internal/buildinfo.Version=1.2.3" +
		" -X github.com/danielhanold/docket/internal/buildinfo.Commit=abc1234" +
		" -X github.com/danielhanold/docket/internal/buildinfo.BuildDate=2026-08-13"
	if out, err := exec.Command("go", "build", "-ldflags", ldflags, "-o", injected, ".").CombinedOutput(); err != nil {
		t.Fatalf("injected build failed: %v\n%s", err, out)
	}
	var stdout bytes.Buffer
	cmd := exec.Command(injected, "version")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "docket 1.2.3 (commit abc1234, built 2026-08-13)\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestJSONErrorCasesOneDocumentEmptyStderr(t *testing.T) {
	cases := [][]string{
		{"--json", "bogus"},              // unknown command
		{"--json", "version", "--bogus"}, // unknown flag before --json? no — flag after mode flag
		{"version", "--bogus", "--json"}, // unknown flag with --json AFTER the failing token
		{"--json", "version", "extra"},   // extra argument
		{"--json"},                       // missing command
	}
	for _, args := range cases {
		out, errS, code := run(t, args...)
		if code != 2 {
			t.Fatalf("args=%v code=%d, want 2", args, code)
		}
		if errS != "" {
			t.Fatalf("args=%v stderr=%q, want empty", args, errS)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["result"] != "invalid-input" || doc["operation"] != "cli" {
			t.Fatalf("args=%v doc=%v", args, doc)
		}
	}
}

func TestHumanParseErrorStderrOnly(t *testing.T) {
	out, errS, code := run(t, "version", "--bogus")
	if code != 2 || out != "" {
		t.Fatalf("out=%q code=%d", out, code)
	}
	if !strings.HasPrefix(errS, "docket: ") {
		t.Fatalf("stderr = %q", errS)
	}
}

func TestHelpConflictAndHumanHelp(t *testing.T) {
	for _, args := range [][]string{{"--json", "--help"}, {"--json", "-h"}, {"--json", "help"}} {
		out, errS, code := run(t, args...)
		if code != 2 || errS != "" {
			t.Fatalf("args=%v err=%q code=%d", args, errS, code)
		}
		doc := assertOneJSONDocument(t, out)
		if doc["reason"] != "json-help-conflict" {
			t.Fatalf("args=%v doc=%v", args, doc)
		}
	}
	out, errS, code := run(t, "--help")
	if code != 0 || errS != "" || !strings.Contains(out, "Usage") {
		t.Fatalf("human help: err=%q code=%d out=%q", errS, code, out)
	}
	if strings.Contains(out, "completion") {
		t.Fatalf("help advertises completion: %q", out)
	}
}

func TestCrossCompileApprovedTargets(t *testing.T) {
	// Buildability gate only: the four tuples must compile with CGO off.
	// Foreign binaries are never executed (change 0317 owns on-target runs).
	tuples := [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}}
	dir := t.TempDir()
	for _, tp := range tuples {
		out := filepath.Join(dir, "docket-"+tp[0]+"-"+tp[1])
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+tp[0], "GOARCH="+tp[1])
		if msg, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cross-build %s/%s failed: %v\n%s", tp[0], tp[1], err, msg)
		}
		if fi, err := os.Stat(out); err != nil || fi.Size() == 0 {
			t.Fatalf("cross-build %s/%s produced no binary", tp[0], tp[1])
		}
	}
}
```

- [ ] **Step 3: Run to verify, fix, and pass**

Run: `go test ./cmd/docket/`
Expected: PASS. If a Cobra behavior differs from an assumption (e.g. which error text `version extra` produces), fix the implementation or the message expectations — never weaken the one-document/empty-stderr/exit-code assertions: those are the spec's contract.

- [ ] **Step 4: Run the full Go gate locally**

```bash
gofmt -l cmd internal
go vet ./...
go test ./...
```

Expected: `gofmt -l` prints nothing; vet and tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/
git commit -m "feat(0304): docket entry point, built-binary subprocess layer, and four-tuple cross-build test"
```

---

### Task 6: Spec-mandated mutation checks

No files change permanently. Both probes follow the backup-copy discipline: `cp` the file aside, mutate, prove the mutation LANDED with `/usr/bin/grep -cF` counts before and after, run the test, `mv -f` the backup back, and re-run the test green. Never restore with `git checkout --` (it restores to HEAD, destroying uncommitted work), and never trust a mutating command's exit code as proof it changed anything.

- [ ] **Step 1: Mutation A — remove the Cobra silence settings**

```bash
cd "$(git rev-parse --show-toplevel)"
cp internal/cli/root.go internal/cli/root.go.bak
/usr/bin/grep -cF 'SilenceErrors: true' internal/cli/root.go   # expect 1
perl -pi -e 's/SilenceErrors: true/SilenceErrors: false/; s/SilenceUsage:  true/SilenceUsage:  false/' internal/cli/root.go
/usr/bin/grep -cF 'SilenceErrors: false' internal/cli/root.go  # expect 1 — the mutation landed
go test ./cmd/docket/ ./internal/cli/ 2>&1 | tail -5
```

Expected: at least one FAIL (Cobra now prints the error/usage itself, breaking the one-document stdout or empty-stderr assertions). If everything stays green, the guards are decoration — STOP and strengthen `TestJSONErrorCasesOneDocumentEmptyStderr` / `TestHumanParseErrorStderrOnly` until this mutation reddens.

```bash
mv -f internal/cli/root.go.bak internal/cli/root.go
go test ./internal/cli/ ./cmd/docket/   # green again
```

- [ ] **Step 2: Mutation B — bypass the bootstrap on the hostile argument order**

```bash
cp internal/cli/root.go internal/cli/root.go.bak
/usr/bin/grep -cF 'jsonMode := DetectJSONMode(args)' internal/cli/root.go   # expect 1
perl -pi -e 's/jsonMode := DetectJSONMode\(args\)/jsonMode := false \/\/ MUTATION: bootstrap bypassed/' internal/cli/root.go
/usr/bin/grep -cF 'MUTATION: bootstrap bypassed' internal/cli/root.go       # expect 1 — landed
go test -run 'TestHostileParseOrderStillJSON|TestJSONErrorCasesOneDocumentEmptyStderr' ./internal/cli/ ./cmd/docket/ 2>&1 | tail -5
```

Expected: FAIL on the `version --bogus --json` case (human error on stderr instead of a JSON document). If green, the hostile-order guard is vacuous — STOP and fix the test.

```bash
mv -f internal/cli/root.go.bak internal/cli/root.go
go test ./...   # whole module green again
```

- [ ] **Step 3: Record**

No commit (tree is unchanged). Note both mutation outcomes (which tests reddened) in the task report — they feed the build-evidence record and the results file.

---

### Task 7: Fixture-convention record — `testdata/README.md`

**Files:**
- Create: `testdata/README.md`

- [ ] **Step 1: Write the README**

`testdata/README.md`:

```markdown
# testdata — fixture conventions

Two fixture tiers, one rule each:

## Package-local `testdata/`

Narrow unit fixtures and output goldens live in the owning package's own
`testdata/` directory (e.g. `internal/cli/testdata/`). They belong to that
package's tests alone; no other package reads them.

## Root `testdata/repositories/v0.9.2/<fixture-name>/`

Frozen cross-package repository fixtures — snapshots of real docket-managed
repository states, versioned by the docket release that produced them.

- **Provenance:** each `<fixture-name>/` directory records where its content
  came from in a `PROVENANCE.md` at its root (source repo, commit, date, and
  any redaction applied).
- **Immutability:** frozen fixtures are immutable source inputs. Never edit a
  file under `v0.9.2/` — a changed input silently re-bases every test that
  reads it. A new upstream state gets a new versioned tree, never an edit.
- **Copy before mutation:** a test that needs to mutate a repository fixture
  copies it into its own temp directory first (`cp -R`) and mutates the copy.
  Tests never write inside `testdata/`.
- **Expected outputs live with the test:** expected transformed output
  belongs beside the owning test (its package `testdata/`), never inside the
  frozen input tree.

Change 0304 establishes the convention only; the first frozen fixtures arrive
with the changes that need them (0305 configuration, 0306 documents).
```

- [ ] **Step 2: Commit**

```bash
git add testdata/README.md
git commit -m "docs(0304): record the two-tier fixture convention"
```

---

### Task 8: Suite producer `tests/test_go_toolchain.sh` + budget registration

**Files:**
- Create: `tests/test_go_toolchain.sh`
- Modify: `tests/runtime-budgets.tsv` (one row)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` re-seed)

**Interfaces:**
- Consumes: the Go module (Tasks 1–5); `scripts/run-tests.sh` discovers `tests/test_*.sh` automatically (`find "$REPO/tests" -maxdepth 1 -name 'test_*.sh'`), so the file self-registers with the runner — but NOT with the budget table.
- Produces: the whole-suite Go gate. Removing this file later reddens `tests/test_runtime_budgets.sh` (orphaned row), which is the "fails if the Go gate is removed from the whole-suite path" guarantee; gutting it reddens its own self-count assert.

- [ ] **Step 1: Write the producer**

`tests/test_go_toolchain.sh`:

```bash
#!/usr/bin/env bash
# tests/test_go_toolchain.sh — the whole-suite Go gate (change 0304).
#
# Runs the four canonical Go checks from the spec's build contract: gofmt
# cleanliness, go vet, go test, and the four-tuple CGO-off cross-build. This
# file is the REAL producer wiring those checks into scripts/run-tests.sh via
# the tests/test_*.sh discovery glob — not a documentation-only command.
#
# Guard shape: CHECKS_RUN counts each executed check and a final assert pins
# the count, so deleting a check from this file reddens the file itself
# rather than silently narrowing the gate. Deleting the FILE orphans its
# tests/runtime-budgets.tsv row, which reddens tests/test_runtime_budgets.sh.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing.
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"

PASS=0
FAIL=0

assert() {
  local desc="$1" cond="$2"
  if eval "$cond"; then
    PASS=$((PASS + 1))
    echo "ok: $desc"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL: $desc" >&2
  fi
}

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  echo "FAIL: cannot run the Go gate without go" >&2
  exit 1
fi

CHECKS_RUN=0
scratch="$(mktemp -d "${TMPDIR:-/tmp}/docket-go-gate.XXXXXX")"
trap 'rm -rf "$scratch"' EXIT

# Check 1: gofmt reports no unformatted Go source.
CHECKS_RUN=$((CHECKS_RUN + 1))
unformatted="$(gofmt -l cmd internal 2>&1)"
assert "gofmt reports no unformatted files" '[ -z "$unformatted" ] || { echo "  unformatted: $unformatted" >&2; false; }'

# Check 2: go vet passes.
CHECKS_RUN=$((CHECKS_RUN + 1))
vet_out="$(go vet ./... 2>&1)"
vet_rc=$?
assert "go vet ./... passes" '[ "$vet_rc" -eq 0 ] || { printf "%s\n" "$vet_out" >&2; false; }'

# Check 3: go test passes on the host.
CHECKS_RUN=$((CHECKS_RUN + 1))
test_out="$(go test ./... 2>&1)"
test_rc=$?
assert "go test ./... passes" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'

# Check 4: CGO-off cross-build succeeds for each approved tuple.
CHECKS_RUN=$((CHECKS_RUN + 1))
build_failures=""
for tuple in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do
  goos="${tuple%/*}"
  goarch="${tuple#*/}"
  build_out="$(CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -o "$scratch/docket-$goos-$goarch" ./cmd/docket 2>&1)" \
    || build_failures="$build_failures $tuple:$build_out"
done
assert "CGO_ENABLED=0 go build succeeds for all four approved tuples" \
  '[ -z "$build_failures" ] || { echo "  failed:$build_failures" >&2; false; }'

# Self-count: the gate ran every check it claims to run. Deleting one from
# this file must redden the file, not silently narrow the whole-suite gate.
assert "all 4 Go checks executed" '[ "$CHECKS_RUN" -eq 4 ]'

echo
echo "pass: $PASS  fail: $FAIL"
[ "$FAIL" -eq 0 ]
```

Note the AGENTS.md rules honored: no producer-pipe (each command's output captured into a variable first), templated `mktemp` (bare `mktemp -d` ignores `TMPDIR` on macOS).

- [ ] **Step 2: Run it standalone**

Run: `bash tests/test_go_toolchain.sh`
Expected: exit 0, `pass: 6  fail: 0` (toolchain assert, four checks, self-count).

- [ ] **Step 3: Mutation-test the producer's self-count**

```bash
cp tests/test_go_toolchain.sh tests/test_go_toolchain.sh.bak
/usr/bin/grep -cF '# Check 2: go vet passes.' tests/test_go_toolchain.sh   # expect 1
perl -0pi -e 's/# Check 2: go vet passes\.\nCHECKS_RUN=\$\(\(CHECKS_RUN \+ 1\)\)\nvet_out="\$\(go vet \.\/\.\.\. 2>&1\)"\nvet_rc=\$\?\nassert "go vet \.\/\.\.\. passes" .*?\n//s' tests/test_go_toolchain.sh
/usr/bin/grep -cF 'go vet' tests/test_go_toolchain.sh   # expect 0 — the mutation landed
bash tests/test_go_toolchain.sh; echo "rc=$?"
```

Expected: `rc=1` (the self-count assert reddens at `CHECKS_RUN=3`). If the perl slice fails to land (count still ≥ 1), delete the Check 2 block by hand in the editor instead — then verify the count is 0 before running. Restore:

```bash
mv -f tests/test_go_toolchain.sh.bak tests/test_go_toolchain.sh
bash tests/test_go_toolchain.sh   # green again
```

- [ ] **Step 4: Register the budget row from a measurement**

Measure the file serially, twice (the Go build cache makes the first run the expensive one; both readings matter — seed from the WORST standalone serial reading per the table's own rule):

```bash
scripts/run-tests.sh -j 1 tests/test_go_toolchain.sh
scripts/run-tests.sh -j 1 tests/test_go_toolchain.sh
```

Read the reported wall-clock seconds. Seed the ceiling with the table's rule — next multiple of 5 plus a 5s margin, minimum 10s, and NO row may exceed 60s. Add the row to `tests/runtime-budgets.tsv` in its sorted position (TAB-separated):

```text
tests/test_go_toolchain.sh	<ceiling>	parallel
```

If the worst serial reading pushes the ceiling above 60s, do not bump past the cap: mark the row `serial` only if parallel contention (not the work itself) is the driver, and otherwise split the producer (e.g. cross-builds into `tests/test_go_toolchain_crossbuild.sh` with its own measured row) — the 60s cap is a rule, not a starting bid.

- [ ] **Step 5: Re-seed `EXPECTED_TOTAL`**

In `tests/test_runtime_budgets.sh`, the line `EXPECTED_TOTAL=1795` pins the table's total. Add the new row's ceiling to it (e.g. a 30s row makes it 1825) and extend the trailing comment with the legitimate-move case: a new test file brings its own row (change 0304).

- [ ] **Step 6: Run the registry check and the producer through the runner**

```bash
bash tests/test_runtime_budgets.sh
scripts/run-tests.sh tests/test_go_toolchain.sh tests/test_runtime_budgets.sh
```

Expected: both green, no OVER BUDGET line for the new row.

- [ ] **Step 7: Commit**

```bash
git add tests/test_go_toolchain.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0304): whole-suite Go gate producer with measured budget row"
```

---

### Task 9: Whole-suite verification

- [ ] **Step 1: Run the resolved whole-suite command**

Run: `scripts/run-tests.sh`
Expected: exit 0. Read the tail: an `OVER BUDGET:` block is a finding to act on (re-measure and re-seed the affected row per Task 8's rule), not noise — nothing else will catch it.

- [ ] **Step 2: Record the new row's margin**

Note the new file's measured-vs-ceiling margin as a NUMBER (e.g. "22s measured against a 30s ceiling — 8s margin") for the results file: changes 0305–0318 all build on this module and several will extend this producer's cost, so the margin is the finding the next change into this file needs.

- [ ] **Step 3: Commit any stragglers and stop**

The branch should now hold Tasks 1–8's commits, `gofmt`/`go vet`/`go test ./...` green, and the whole suite green. Do not open a PR from within the build — the caller owns review and PR.
