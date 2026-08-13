<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0305 — Configuration and capability envelope](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0305-configuration-and-capability-envelope.md)**
<!-- docket:backlink:end -->

# Configuration and Capability Envelope Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A typed `internal/config` package that resolves the final Bash-release (`v0.9.2`) configuration surface through four layers with exact provenance, classifies every setting's Go v1 capability, and exposes a read-only `docket diagnostic config` operation plus a mutation preflight that refuses active deferred/dropped behavior with `unsupported-config`.

**Architecture:** A pure resolver over already-supplied `Source` layers, driven by one package-owned schema/policy registry (paths, validators, defaults, merge rules, fences, capability rules — no second key list anywhere). YAML is parsed to `yaml.Node` first for duplicate-key rejection, strict boolean spellings, and line/column provenance, then decoded per-leaf against the registry. Resolution sets provenance at assignment time; classification and the mutation preflight consume the resolved snapshot. A thin filesystem adapter and a thin Cobra subcommand wrap the core; both reuse change 0304's envelope, presenter, and exit mapping unchanged.

**Tech Stack:** Go 1.26, Cobra (already present), `go.yaml.in/yaml/v3` pinned at `v3.0.4` (new dependency — the only one this change adds).

**Spec:** `.docket/docs/superpowers/specs/2026-08-13-configuration-and-capability-envelope-design.md` (read it before any task; this plan argues from it). The spec's *Exhaustive `v0.9.2` setting matrix* is the authority for every classification; this plan restates it as executable data in *Reference B*.

## Global Constraints

- YAML library is exactly `go.yaml.in/yaml/v3 v3.0.4` — never `gopkg.in/yaml.v3`, never a v4 RC.
- No code in this change invokes `git`, `gh`, the network, or writes any file outside test temp dirs. The filesystem adapter only **reads**.
- Booleans accept only the spellings `true` and `false` (plain or `!!bool`-tagged scalar). `yes`/`no`/`on`/`off`/`"true"` (quoted string) are `invalid-type`/`invalid-value` errors.
- Every diagnostic carries a stable `code` from the closed set: `invalid-yaml`, `duplicate-key`, `unknown-key`, `invalid-type`, `invalid-value`, `fenced-setting-ignored`, `obsolete-setting`, `inert-setting`, `deferred-setting`, `deferred-capability-requested`. No other code may be minted.
- Snapshot **validity** is keyed on the presence of any diagnostic whose code is in the *invalid class* — `invalid-yaml`, `duplicate-key`, `unknown-key`, `invalid-type`, `invalid-value` — never on severity. `deferred-capability-requested` has severity `error` but leaves the snapshot valid.
- Diagnostics are sorted (severity rank error > warning > info, then `path`, then `code`); capabilities are sorted by `path`. Blocker sets are returned complete, never first-only.
- Diagnostics never include environment dumps or raw file contents; messages name the setting and the source file.
- Every mutation probe and manual re-verification runs `go test -count=1` (the Go test cache serves stale verdicts otherwise — see learnings `cached-runner-serves-a-mutated-tree`).
- Tests never read or write the developer's real global config: every test that touches the filesystem adapter or the built binary pins `XDG_CONFIG_HOME` and `HOME` to temp dirs (`t.Setenv`).
- Frozen fixtures under `testdata/repositories/v0.9.2/` are immutable inputs with a `PROVENANCE.md` each; tests copy before mutating and never write inside `testdata/` (see `testdata/README.md`).
- `gofmt`-clean, `go vet`-clean — `tests/test_go_toolchain.sh` gates both for the whole suite.
- Vendor model IDs are opaque passthrough; no test asserts a vendor ID exists. Parity is proven against the **frozen sidecar fixture**, and the vendor-validity question is a named human verification item (recorded at results time, not as a test).

## File Structure

```
go.mod / go.sum                                  # + go.yaml.in/yaml/v3 v3.0.4
internal/config/config.go                        # package doc, core types, error sentinels, diagnostic codes
internal/config/parse.go                         # Source → *yaml.Node (node stage): doc shape, dup keys, aliases
internal/config/schema.go                        # THE registry: paths, kinds, defaults, merge, scope, classification
internal/config/decode.go                        # node → typed rawLayer, per-leaf validation, unknown-key policy
internal/config/defaults.go                      # built-in Effective defaults + the 16×4 agent table
internal/config/resolve.go                       # precedence, fences, provenance, Effective assembly
internal/config/capability.go                    # classifier, activation rules, deterministic ordering
internal/config/preflight.go                     # PreflightMutation + GuardMutation (the transaction seam)
internal/config/fs.go                            # filesystem adapter (repo dir + XDG global)
internal/config/*_test.go                        # one test file per source file, same base name
internal/app/config.go + config_test.go          # diagnostic.config / config.preflight operations
internal/cli/root.go (modify)                    # `docket diagnostic config` subcommand
cmd/docket/config_cli_test.go                    # built-binary protocol coverage
testdata/repositories/v0.9.2/…                   # frozen configuration fixtures (Task 9)
```

`internal/config` imports nothing from `internal/app`, `internal/cli`, or `internal/buildinfo`. `internal/app` imports `internal/config`. Dependency direction is one-way, matching 0304.

---

## Reference A — core types (defined in Task 1, consumed everywhere)

These exact shapes go in `internal/config/config.go`. Later tasks use them verbatim; a name drift between tasks is a bug.

```go
// Package config owns docket's configuration vocabulary: the v0.9.2 schema,
// four-layer resolution with provenance, capability classification, and the
// mutation preflight. It is pure — no Git, no network, no writes.
package config

type LayerKind string

const (
	LayerBuiltIn         LayerKind = "built-in"
	LayerGlobal          LayerKind = "global"
	LayerRepository      LayerKind = "repository"
	LayerRepositoryLocal LayerKind = "repository-local"
)

// Source is one configuration layer's raw bytes. Name is the display path
// used in diagnostics ("built-in", ".docket.yml", ".docket.local.yml", or the
// global config's absolute path).
type Source struct {
	Layer LayerKind
	Name  string
	Data  []byte
}

// ResolveContext supplies facts resolution cannot derive from the layers.
type ResolveContext struct {
	DefaultBranch string // consumed only when integration_branch resolves to "auto"
}

type Provenance struct {
	Layer  LayerKind `json:"layer"`
	Source string    `json:"source"`
	Line   int       `json:"line,omitempty"`
	Column int       `json:"column,omitempty"`
}

// Value is one resolved leaf: the typed value, where it came from, and
// whether any honored layer set it explicitly (false = built-in default won).
type Value[T any] struct {
	Value      T          `json:"value"`
	Provenance Provenance `json:"provenance"`
	Explicit   bool       `json:"explicit"`
}

type Classification string

const (
	Supported Classification = "supported"
	Obsolete  Classification = "obsolete"
	Inert     Classification = "inert"
	Deferred  Classification = "deferred"
)

type Capability struct {
	Path           string         `json:"path"`
	Classification Classification `json:"classification"`
	Active         bool           `json:"active"`
	MutationBlock  bool           `json:"mutation_block"`
	Provenance     Provenance     `json:"provenance"`
	Reason         string         `json:"reason"`
	Remedy         string         `json:"remedy,omitempty"`
}

type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

type Diagnostic struct {
	Code           string         `json:"code"`
	Severity       Severity       `json:"severity"`
	Path           string         `json:"path,omitempty"`
	Classification Classification `json:"classification,omitempty"`
	Provenance     *Provenance    `json:"provenance,omitempty"`
	Message        string         `json:"message"`
	Remedy         string         `json:"remedy,omitempty"`
}

// Diagnostic codes — the closed set. Snapshot validity keys on invalidClass.
const (
	CodeInvalidYAML          = "invalid-yaml"
	CodeDuplicateKey         = "duplicate-key"
	CodeUnknownKey           = "unknown-key"
	CodeInvalidType          = "invalid-type"
	CodeInvalidValue         = "invalid-value"
	CodeFencedIgnored        = "fenced-setting-ignored"
	CodeObsoleteSetting      = "obsolete-setting"
	CodeInertSetting         = "inert-setting"
	CodeDeferredSetting      = "deferred-setting"
	CodeDeferredCapRequested = "deferred-capability-requested"
)

// Effective is the typed aggregate of SUPPORTED policy only. Inactive
// deferred/inert companion values surface through Capabilities and
// Diagnostics, never here.
type Effective struct {
	MetadataBranch    Value[string]   `json:"metadata_branch"`
	IntegrationBranch Value[string]   `json:"integration_branch"` // auto already resolved
	ChangesDir        Value[string]   `json:"changes_dir"`
	ADRsDir           Value[string]   `json:"adrs_dir"`
	ResultsDir        Value[string]   `json:"results_dir"`
	Finalize          Finalize        `json:"finalize"`
	Learnings         Learnings       `json:"learnings"`
	Reclaim           Reclaim         `json:"reclaim"`
	Review            Review          `json:"review"`
	GateObservation   Value[int]      `json:"gate_observation_budget"` // minutes
	BoardSurfaces     Value[[]string] `json:"board_surfaces"`
	ChangeTypes       Value[[]string] `json:"change_types"`
	Agents            AgentsTable     `json:"agents"`
}

type Finalize struct {
	Gate              Value[string] `json:"gate"`         // local|off (ci/both classify deferred-active)
	TestCommand       Value[string] `json:"test_command"` // "" == unset (the `auto` sentinel resolved away)
	RequirePRApproval Value[bool]   `json:"require_pr_approval"`
}

type Learnings struct {
	Enabled Value[bool] `json:"enabled"`
}

type Reclaim struct {
	LeaseTTL Value[int]  `json:"lease_ttl"` // hours
	Auto     Value[bool] `json:"auto"`
}

type Review struct {
	MinFixSeverity Value[string] `json:"min_fix_severity"` // minor|important|blocker
	MaxFixTasks    Value[int]    `json:"max_fix_tasks"`
}

// AgentsTable: harness → agent short name → resolved model/effort.
// Resolved from built-in + GLOBAL layers only; repository-layer agent
// declarations are deferred-capability requests and never land here.
type AgentsTable map[string]map[string]AgentSetting

type AgentSetting struct {
	Model  Value[string] `json:"model"`
	Effort Value[string] `json:"effort"` // "" == effort pin suppressed (`effort: auto`)
}

type Snapshot struct {
	Effective    Effective    `json:"effective"`
	Capabilities []Capability `json:"capabilities"`
	Diagnostics  []Diagnostic `json:"diagnostics"`
}

// Sentinel errors. Resolve wraps these; callers test with errors.Is.
var (
	ErrInvalidConfig            = errors.New("invalid configuration")
	ErrMissingResolutionContext = errors.New("missing resolution context: integration_branch is auto and no default branch was supplied")
	ErrUnsupportedConfig        = errors.New("unsupported configuration: active deferred or dropped capability requested")
)
```

Top-level API (implemented across Tasks 5–8):

```go
// Resolve parses and resolves sources (low→high precedence order:
// built-in, global, repository, repository-local — the built-in Source is
// synthesized internally and MUST NOT appear in sources).
// Returns (snapshot, allDiagnostics, nil) on a valid snapshot;
// (nil, allDiagnostics, ErrInvalidConfig) when any invalid-class diagnostic
// exists; (nil, diags, ErrMissingResolutionContext) when integration_branch
// resolves to auto with no ctx.DefaultBranch.
func Resolve(sources []Source, rctx ResolveContext) (*Snapshot, []Diagnostic, error)

type PreflightDecision struct {
	Allowed  bool
	Blockers []Diagnostic // deferred-capability-requested entries, path order
}

func PreflightMutation(s *Snapshot) PreflightDecision

// GuardMutation is the seam later transaction operations must call: it runs
// continue_ only when the preflight allows mutation, else returns
// ErrUnsupportedConfig without calling it.
func GuardMutation(s *Snapshot, continue_ func() error) error

type FSOptions struct {
	RepoDir    string // required; cleaned to an absolute path, used verbatim (no Git discovery)
	GlobalPath string // test seam; "" → ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml
}

// LoadFilesystemSources reads DIR/.docket.yml (repository), DIR/.docket.local.yml
// (repository-local), and the global config, returning present layers in
// low→high order. A missing file is an absent layer, not an error.
func LoadFilesystemSources(opts FSOptions) ([]Source, error)
```

## Reference B — the schema/policy registry data

One table drives parser, resolver, classifier, preflight, and the exhaustive tests. Static leaf paths, their YAML kind, built-in default, merge rule, layer scope, and Go v1 classification. **This is the spec's matrix restated as data — on any doubt the spec's matrix wins.**

Scopes: `any` = any layer; `repo-fenced` = coordination-fenced, machine-layer declarations get `fenced-setting-ignored` (warning) and are excluded from resolution; `local-only` = machine layers only (a committed declaration is warned-and-ignored the same way — same fence mechanism, opposite direction).

| # | path | kind | built-in default | merge | scope | classification / activation |
|---|---|---|---|---|---|---|
| 1 | `runtime.bash` | string | — | scalar | local-only | **obsolete**: `obsolete-setting` warning in EVERY layer, excluded from resolution; never blocks |
| 2 | `metadata_branch` | enum `docket\|main` | `docket` | scalar | repo-fenced | supported |
| 3 | `integration_branch` | non-empty string | `auto` | scalar | repo-fenced | supported; `auto` → `ResolveContext.DefaultBranch`, missing → `ErrMissingResolutionContext` |
| 4 | `changes_dir` | clean non-empty relative path | `docs/changes` | scalar | repo-fenced | supported |
| 5 | `adrs_dir` | same | `docs/adrs` | scalar | repo-fenced | supported |
| 6 | `results_dir` | same | `docs/results` | scalar | repo-fenced | supported |
| 7 | `finalize.gate` | enum `local\|ci\|both\|off` | `local` | scalar | any | supported when `local`/`off`; **deferred-active** (blocks) when `ci`/`both` |
| 8 | `finalize.test_command` | string | `auto` | scalar | any | supported; `auto` resolves to `""` (unset) |
| 9 | `finalize.require_pr_approval` | bool | `false` | scalar | any | supported |
| 10 | `finalize.skip_results_only_delta` | bool | `false` | scalar | repo-fenced | **deferred**: `false` inactive (info `deferred-setting` when explicit); repo `true` blocks |
| 11 | `learnings.enabled` | bool | `true` | scalar | any | supported; when effective `true`, one info `deferred-setting` notice that automatic harvest/index/capacity/promotion are deferred |
| 12 | `learnings.cap` | int ≥ 0 | `300` | scalar | any | **inert** (historical): validate; info `inert-setting` when explicit; never blocks |
| 13 | `reclaim.lease_ttl` | int ≥ 0 | `72` | scalar | any | supported |
| 14 | `reclaim.auto` | bool | `false` | scalar | any | supported |
| 15 | `build.checkpoint` | bool | `false` | scalar | any | **deferred**: `false` inactive; `true` blocks |
| 16 | `review.min_fix_severity` | enum `minor\|important\|blocker` | `minor` | scalar | any | supported |
| 17 | `review.max_fix_tasks` | int ≥ 0 | `10` | scalar | any | supported |
| 18 | `gate_observation_budget` | int ≥ 0 | `30` | scalar | any | supported |
| 19 | `delegation_observation_budget` | int ≥ 0 | `60` | scalar | any | **inert** (historical): validate + info; never blocks alone |
| 20 | `board_surfaces` | list of strings | `[inline]` | list-replace | any (the `github` **token** is repo-fenced) | `inline`/empty supported; repo `github` token = **active dropped-capability**, blocks; machine `github` token dropped + `fenced-setting-ignored`, rest of list honored; unknown token = warning `unknown-key`, dropped (v0.9.2 warn-and-ignore) |
| 21 | `github_project` | `auto` or map `{owner: non-empty string, number: int ≥ 1}` | `auto` | scalar | repo-fenced | **inert** (historical): parse + info; never blocks |
| 22 | `terminal_publish` | bool | `false` | scalar | repo-fenced | **deferred**: `false` inactive; repo `true` blocks |
| 23 | `auto_groom` | bool | `false` | scalar | any | **deferred**: `false` inactive; `true` blocks |
| 24 | `change_types` | non-empty dup-free list, tokens `^[a-z][a-z0-9-]*$`, `all`/`untyped` reserved (invalid-value) | `[chore, docs, feat, fix, refactor, perf]` | list-replace | any | supported |
| 25 | `auto_capture.enabled` | bool | `false` | scalar | any | **deferred**: `false` inactive; `true` blocks. A **scalar** `auto_capture:` node (the obsolete pre-0127 shape) is `invalid-value` with the nested replacement in the remedy — detected at the node stage, before normalization |
| 26 | `auto_capture.types` | `all` or dup-free list ⊆ effective `change_types` (subset checked post-resolution) | `all` | scalar/list-replace | any | **inert companion**: blocks only through effective `auto_capture.enabled: true` |
| 27 | `dummy_mode.enabled` | bool | `false` | scalar | any | **deferred**: `false` inactive; `true` blocks |
| 28 | `dummy_mode.persona` | string (any) | `""` | scalar | any | inert companion |
| 29 | `dummy_mode.surfaces` | `all` or list ⊆ `{dialogue, reports, results, change-sections, pr}` | `all` | scalar/list-replace | any | inert companion |
| 30 | `agent_harnesses` | list of non-empty strings | — (absent) | list-replace | any | **inert** (historical): parse + info; no Go v1 behavior |
| 31 | `skills.brainstorm` `.plan` `.build` `.review` `.finish` | non-empty string | — | scalar | any | **deferred**: ANY explicit leaf blocks, even repeating the shipped default. Unknown role keys under `skills:` = warning `unknown-key`, ignored (v0.9.2 behavior) |
| 32 | `agents.<h>.<a>.model` / `.effort` | opaque non-empty **space-free** string | Reference C (16×4) | scalar (harness-first fallback) | see classification | GLOBAL layer = supported (the sole override layer; `effort: auto` suppresses the pin). REPOSITORY or REPOSITORY-LOCAL layer = **deferred-active**, blocks even when equal to a shipped default, and never lands in `Effective.Agents` |
| 33 | `agents.<h>.<a>.runner` | non-empty string | — | scalar | any | **deferred-active**: any explicit runner assignment blocks |
| 34 | `runners.codex.sandbox` | enum `workspace-write\|danger-full-access` | — | scalar | any | inert companion (activation = an effective agent entry naming that runner) |
| 35 | `runners.codex.network` | bool | — | scalar | any | inert companion |
| 36 | `runners.opencode.permissions` | enum `ask\|auto-approve` | — | scalar | any | inert companion |
| 37 | `runners.<codex\|cursor\|opencode>.shim_model` | non-empty space-free string or `inherit` | — | scalar | any | inert companion |
| 38 | `runners.<codex\|cursor\|opencode>.shim_effort` | non-empty space-free string | — | scalar | any | inert companion |

Dynamic-subtree name rules (typos are `invalid-config`, because silently discarding an intended model override would run the wrong model):

- `agents.<h>` — `<h>` ∈ `{default, claude, codex, cursor, opencode}`; anything else → `unknown-key` error.
- `agents.<h>.<a>` — `<a>` ∈ the 16 short names of Reference C; anything else → `unknown-key` error.
- `agents.<h>.<a>.<field>` — `<field>` ∈ `{model, effort, runner}`; anything else → `unknown-key` error.
- `runners.<r>` — `<r>` ∈ `{codex, cursor, opencode}`; anything else → `unknown-key` error. Keys under a runner outside its rows above → `unknown-key` error.
- Any other unknown top-level or nested path → `unknown-key` **error** (invalid class), EXCEPT the two deliberate v0.9.2 warn-and-ignore surfaces: unknown `skills.<role>` keys and unknown `board_surfaces` tokens, which are `unknown-key` **warnings**.

## Reference C — built-in agent defaults (16 agents × 4 harnesses)

Frozen from `agents/harness-defaults.yml` at commit `096c48de` (the v0.9.2 surface). The 16 short names, in canonical order:

`adr, auto-groom, auto-groom-critic, brainstorm-consultant, build-economy, build-standard, build-premium, build-max, finalize-change, implement-next, integration-repair, rebase-resolver, review-lean, review-standard, review-deep, status`

| agent | claude | cursor (all `effort: auto` → suppressed) | codex | opencode |
|---|---|---|---|---|
| adr | claude-opus-5 / low | cursor-grok-4.5-high | gpt-5.6-terra / xhigh | openrouter/moonshotai/kimi-k3 / medium |
| auto-groom | claude-opus-5 / low | cursor-grok-4.5-medium | gpt-5.6-sol / low | openrouter/deepseek/deepseek-v4-flash-0731 / medium |
| auto-groom-critic | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / medium | openrouter/openai/gpt-5.6-luna / high |
| brainstorm-consultant | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / medium | openrouter/moonshotai/kimi-k3 / medium |
| build-economy | claude-sonnet-5 / low | cursor-grok-4.5-low | gpt-5.6-luna / xhigh | openrouter/deepseek/deepseek-v4-flash-0731 / medium |
| build-standard | claude-opus-5 / low | cursor-grok-4.5-medium | gpt-5.6-terra / medium | openrouter/deepseek/deepseek-v4-flash-0731 / high |
| build-premium | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / low | openrouter/moonshotai/kimi-k3 / medium |
| build-max | claude-opus-5 / high | claude-opus-5-high | gpt-5.6-sol / medium | openrouter/moonshotai/kimi-k3 / high |
| finalize-change | claude-opus-5 / low | cursor-grok-4.5-high-fast | gpt-5.6-terra / high | openrouter/deepseek/deepseek-v4-flash-0731 / high |
| implement-next | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / medium | openrouter/deepseek/deepseek-v4-flash-0731 / high |
| integration-repair | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / high | openrouter/moonshotai/kimi-k3 / high |
| rebase-resolver | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-sol / high | openrouter/moonshotai/kimi-k3 / high |
| review-lean | claude-sonnet-5 / high | cursor-grok-4.5-medium | gpt-5.6-terra / medium | openrouter/deepseek/deepseek-v4-flash-0731 / high |
| review-standard | claude-opus-5 / medium | cursor-grok-4.5-high | gpt-5.6-terra / high | openrouter/moonshotai/kimi-k3 / medium |
| review-deep | claude-opus-5 / high | claude-opus-5-high | gpt-5.6-sol / medium | openrouter/moonshotai/kimi-k3 / high |
| status | claude-haiku-4-5-20251001 / medium | cursor-grok-4.5-low-fast | gpt-5.6-luna / xhigh | openrouter/deepseek/deepseek-v4-flash-0731 / low |

In the built-in `AgentsTable`, cursor entries carry `Effort.Value == ""` (suppressed) with built-in provenance; every other harness carries the effort shown.

## Reference D — protocol shapes

`docket diagnostic config --repo-dir DIR [--default-branch BRANCH] [--for-mutation] [--json]`

| condition | operation | result | reason field | exit |
|---|---|---|---|---|
| valid snapshot, inspection | `diagnostic.config` | `applied` (even when `mutation_allowed` is false) | — | 0 |
| valid snapshot, `--for-mutation`, no blockers | `config.preflight` | `applied` | — | 0 |
| valid snapshot, `--for-mutation`, blockers | `config.preflight` | `unsupported-config` | `deferred-capability-requested` | 1 |
| invalid configuration (either mode) | same as mode | `invalid-input` | `invalid-config` | 2 |
| `auto` integration branch, no `--default-branch` | same as mode | `invalid-input` | `missing-resolution-context` | 2 |

JSON document (extends 0304's envelope; one document + one newline on stdout, empty stderr):

```json
{
  "protocol_version": 1,
  "operation": "diagnostic.config",
  "result": "applied",
  "source_mode": "filesystem",
  "mutation_allowed": false,
  "effective": {},
  "capabilities": [],
  "diagnostics": []
}
```

On `invalid-input` results, `effective` and `capabilities` are omitted (`omitempty` pointers/nil slices), `reason` and `message` are present, `diagnostics` carries whatever parsing produced.

---

### Task 1: YAML dependency and node-stage layer parsing

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/config/config.go` (Reference A verbatim: types, codes, sentinels — the full file)
- Create: `internal/config/parse.go`
- Test: `internal/config/parse_test.go`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: all Reference A types; `func parseLayer(src Source) (*yaml.Node, []Diagnostic)` — returns the mapping-kind root node of the single document (nil for an absent/empty layer), plus any node-stage diagnostics. Node-stage rules: malformed YAML → `invalid-yaml` error; more than one document → `invalid-yaml` error ("layer must contain zero or one YAML document"); empty file / comments-only / null root → `(nil, nil)` (absent layer); non-mapping root → `invalid-type` error; any alias node or `<<` merge key anywhere → `invalid-yaml` error; duplicate keys in any mapping (recursive walk) → `duplicate-key` error with line/column of the second occurrence. Every diagnostic carries `Provenance{Layer: src.Layer, Source: src.Name, Line, Column}` from the offending node.

- [ ] **Step 1: Add the dependency**

```bash
cd <worktree-root>
go get go.yaml.in/yaml/v3@v3.0.4
go mod tidy
```

Verify `go.mod` requires exactly `go.yaml.in/yaml/v3 v3.0.4`.

- [ ] **Step 2: Write `internal/config/config.go`** — Reference A's types, codes, and sentinels exactly as written (add the `errors` import). No behavior yet.

- [ ] **Step 3: Write the failing tests** (`parse_test.go`, table-driven):

```go
func TestParseLayer(t *testing.T) {
	src := func(data string) Source {
		return Source{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(data)}
	}
	cases := []struct {
		name      string
		data      string
		wantNil   bool
		wantCodes []string // codes of returned diagnostics, in order
	}{
		{"empty file", "", true, nil},
		{"comments only", "# just a comment\n", true, nil},
		{"null document", "---\n", true, nil},
		{"simple mapping", "metadata_branch: docket\n", false, nil},
		{"flow mapping", "{metadata_branch: docket}\n", false, nil},
		{"malformed", "a: [unclosed\n", true, []string{CodeInvalidYAML}},
		{"two documents", "a: 1\n---\nb: 2\n", true, []string{CodeInvalidYAML}},
		{"sequence root", "- a\n- b\n", true, []string{CodeInvalidType}},
		{"scalar root", "just-a-string\n", true, []string{CodeInvalidType}},
		{"alias", "a: &x 1\nb: *x\n", true, []string{CodeInvalidYAML}},
		{"merge key", "base: &b {x: 1}\nout:\n  <<: *b\n", true, []string{CodeInvalidYAML}},
		{"duplicate top-level key", "a: 1\na: 2\n", true, []string{CodeDuplicateKey}},
		{"duplicate nested key", "m:\n  a: 1\n  a: 2\n", true, []string{CodeDuplicateKey}},
	}
	// for each: node, diags := parseLayer(src(tc.data)); assert nil-ness,
	// codes, SeverityError on every returned diag, and that every diag's
	// Provenance has Source == ".docket.yml", Layer == LayerRepository, Line >= 1.
}
```

Also `TestParseLayerProvenanceLine` asserting the duplicate-key diagnostic for `"a: 1\nb: 2\na: 3\n"` reports Line 3.

- [ ] **Step 4: Run to verify failure**: `go test ./internal/config/ -count=1` → FAIL (parseLayer undefined).

- [ ] **Step 5: Implement `parse.go`**. Use `yaml.NewDecoder(bytes.NewReader(src.Data))` decoding into a `yaml.Node`; `io.EOF` on first decode = absent; a successful second decode (non-EOF) = multi-document. The document node's first content child is the root. Then one recursive walk enforcing: no `yaml.AliasNode`; no mapping key with `Value == "<<"`; per-mapping duplicate key detection over `Content[i]` (i even = key). Return the mapping root only when no error-class diagnostic was produced; otherwise `(nil, diags)`.

- [ ] **Step 6: Run to verify pass**: `go test ./internal/config/ -count=1` → PASS. Also `gofmt -l internal/config` → empty; `go vet ./internal/config/` → clean.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/config/config.go internal/config/parse.go internal/config/parse_test.go
git commit -m "feat(0305): config core types and node-stage YAML layer parsing"
```

---

### Task 2: The schema/policy registry

**Files:**
- Create: `internal/config/schema.go`
- Test: `internal/config/schema_test.go`

**Interfaces:**
- Consumes: Reference A types.
- Produces (package-private, consumed by decode/resolve/classify/tests):

```go
type valueKind int

const (
	kindString valueKind = iota
	kindBool
	kindInt          // non-negative
	kindStringList
	kindStringOrList // scalar `all` or a list (auto_capture.types, dummy_mode.surfaces)
	kindScalarOrMap  // github_project
	kindMap          // interior nodes handled structurally, not leaves
)

type mergeRule int

const (
	mergeScalar mergeRule = iota
	mergeListReplace
)

type layerScope int

const (
	scopeAny layerScope = iota
	scopeRepoFenced // machine-layer declaration → fenced-setting-ignored, excluded
	scopeLocalOnly  // committed-layer declaration → fenced-setting-ignored, excluded
)

type disposition int // static classification family; value/layer conditions applied in capability.go

const (
	dispSupported disposition = iota
	dispObsolete
	dispInert
	dispDeferred          // bool-style: false inactive, true blocks
	dispDeferredByValue   // finalize.gate: ci/both block
	dispSupportedOrDropped // board_surfaces: github token
	dispInertCompanion    // auto_capture.types, dummy_mode.persona/surfaces, runners.*
	dispAgentsLeaf        // agents.<h>.<a>.model/effort: layer-dependent
	dispDeferredActive    // skills.*, agents...runner: any explicit value blocks
)

type pathSpec struct {
	path     string // dotted; dynamic segments spelled "*": "agents.*.*.model"
	kind     valueKind
	enum     []string // non-nil for enum-constrained strings
	def      any      // built-in default; nil = no default (absent built-in)
	merge    mergeRule
	scope    layerScope
	disp     disposition
	// validate returns the typed Go value or diagnostics; implemented per kind
	// with shared constructors: boolLeaf(), intLeaf(), enumLeaf(...),
	// stringLeaf(nonEmpty, spaceFree, relPath bool), listLeaf(...), etc.
}

// registry returns the full static table (Reference B rows 1–31 as concrete
// paths, plus the dynamic patterns for rows 32–38); order = Reference B order.
func registry() []pathSpec

// agentHarnesses / agentShortNames / runnerNames / skillRoles: the closed
// name sets from Reference B/C, exported to decode and tests.
```

Static concrete paths total (rows 1–31 expanded, `skills.*` as five concrete paths): spell every one; the five `skills.<role>` and the dynamic `agents`/`runners` patterns complete the surface.

- [ ] **Step 1: Write the failing tests**:

```go
// TestRegistryPathSetMatchesV092 pins the COMPLETE static path set — a
// two-way, whole-sequence compare (learnings: correspondence-guard-runs-one-way;
// the cheapest reverse loop is one equality compare). Removing a registry row
// or adding an unlisted one MUST redden this test.
func TestRegistryPathSetMatchesV092(t *testing.T) {
	want := []string{
		"runtime.bash", "metadata_branch", "integration_branch",
		"changes_dir", "adrs_dir", "results_dir",
		"finalize.gate", "finalize.test_command", "finalize.require_pr_approval",
		"finalize.skip_results_only_delta",
		"learnings.enabled", "learnings.cap",
		"reclaim.lease_ttl", "reclaim.auto",
		"build.checkpoint",
		"review.min_fix_severity", "review.max_fix_tasks",
		"gate_observation_budget", "delegation_observation_budget",
		"board_surfaces", "github_project", "terminal_publish", "auto_groom",
		"change_types", "auto_capture.enabled", "auto_capture.types",
		"dummy_mode.enabled", "dummy_mode.persona", "dummy_mode.surfaces",
		"agent_harnesses",
		"skills.brainstorm", "skills.plan", "skills.build", "skills.review", "skills.finish",
		"agents.*.*.model", "agents.*.*.effort", "agents.*.*.runner",
		"runners.codex.sandbox", "runners.codex.network",
		"runners.opencode.permissions",
		"runners.*.shim_model", "runners.*.shim_effort",
	}
	// got := paths from registry(), in order; assert reflect.DeepEqual(got, want).
}
```

Plus: `TestRegistryFencedSet` (exactly rows with scopeRepoFenced == {metadata_branch, integration_branch, changes_dir, adrs_dir, results_dir, finalize.skip_results_only_delta, github_project, terminal_publish} — the `github` board token is fenced in board_surfaces' own validator, and `runtime.bash` is the one scopeLocalOnly row); `TestRegistryDefaults` (every supported/deferred row's `def` equals Reference B's default column; rows with "—" have nil); `TestRegistryPathsUnique`; `TestNameSets` (harnesses = default+4, agents = the 16 names in order, runners = codex/cursor/opencode, skill roles = the 5).

- [ ] **Step 2: Run to verify failure**: `go test ./internal/config/ -run TestRegistry -count=1` → FAIL.

- [ ] **Step 3: Implement `schema.go`** — the table exactly per Reference B, the name-set slices, and the validator constructors (pure functions over `*yaml.Node` producing `(any, []Diagnostic)`; strict bool rule from Global Constraints; int via `!!int` tag + `strconv.Atoi` + non-negative; enum membership; relative-path validation = non-empty, `!filepath.IsAbs`, `path.Clean(p) == p`, no `..` segment; change_types full rule; board token handling deferred to decode).

- [ ] **Step 4: Run to verify pass**: `go test ./internal/config/ -count=1` → PASS.

- [ ] **Step 5: Mutation-probe the coverage test** (both directions): delete the `learnings.cap` row from `registry()` → `go test -run TestRegistryPathSetMatchesV092 -count=1 ./internal/config/` must FAIL; restore. Add a phantom row `"learnings.bogus"` → must FAIL; restore (restore by re-editing, never `git checkout` — see learnings `mutation-restore-needs-a-backup-copy`).

- [ ] **Step 6: Commit**

```bash
git add internal/config/schema.go internal/config/schema_test.go
git commit -m "feat(0305): one schema/policy registry for the v0.9.2 surface"
```

---

### Task 3: Typed layer decode and per-leaf validation

**Files:**
- Create: `internal/config/decode.go`
- Test: `internal/config/decode_test.go`

**Interfaces:**
- Consumes: `parseLayer`, `registry()`, name sets, validator constructors.
- Produces:

```go
// leafDecl is one explicitly declared leaf in one layer.
type leafDecl struct {
	path  string     // concrete dotted path ("agents.claude.adr.model")
	spec  *pathSpec  // matched registry row
	value any        // typed Go value (string, bool, int, []string, githubProject)
	prov  Provenance // layer, source name, node line/column
}

type githubProject struct {
	Auto   bool
	Owner  string
	Number int
}

// decodeLayer walks the parsed mapping against the registry, returning every
// declared leaf plus that layer's diagnostics.
func decodeLayer(root *yaml.Node, src Source) ([]leafDecl, []Diagnostic)
```

Decode rules beyond per-leaf validation: unknown-key policy per Reference B (error except `skills.<unknown-role>` and unknown board tokens = warnings); a **scalar** `auto_capture:` → `invalid-value` error, path `auto_capture`, remedy printing the nested replacement (`auto_capture:\n  enabled: true|false\n  types: all`), detected on the raw node (the condition, not a residue); `runtime.bash` → decoded but flagged `obsolete-setting` warning and NOT returned as a leaf (excluded from resolution in every layer); `board_surfaces` returns the raw validated token list (fence/dropped-token handling happens at resolution where the layer matters); interior nodes that should be mappings but aren't (`finalize: local`) → `invalid-type` error.

- [ ] **Step 1: Write the failing tests.** Two parts:

Part 1 — table-driven per-path acceptance covering EVERY static registry row in both block and flow spellings where the kind allows both. Derive the table from the registry so a new row cannot be silently untested:

```go
// TestDecodeEveryRegisteredPath: for each static registry row, decode a
// minimal document declaring that path with a valid value (block style, and
// flow style for mappings/lists), assert exactly one leafDecl with the right
// path, typed value, and provenance — plus the expected side diagnostics
// (obsolete-setting for runtime.bash; none elsewhere).
```

Part 2 — explicit rejection cases (each asserting code, path, severity, and provenance line):

```go
cases := []struct{ name, yaml, wantCode, wantPath string; wantSev Severity }{
	{"unknown top-level key", "not_a_key: 1\n", CodeUnknownKey, "not_a_key", SeverityError},
	{"unknown nested key", "finalize:\n  bogus: 1\n", CodeUnknownKey, "finalize.bogus", SeverityError},
	{"unknown skills role is a warning", "skills:\n  deploy: x\n", CodeUnknownKey, "skills.deploy", SeverityWarning},
	{"bool yaml11 alias", "auto_groom: yes\n", CodeInvalidType, "auto_groom", SeverityError},
	{"bool quoted string", "auto_groom: \"true\"\n", CodeInvalidType, "auto_groom", SeverityError},
	{"negative int", "learnings:\n  cap: -1\n", CodeInvalidValue, "learnings.cap", SeverityError},
	{"bad enum", "finalize:\n  gate: sometimes\n", CodeInvalidValue, "finalize.gate", SeverityError},
	{"metadata branch enum", "metadata_branch: trunk\n", CodeInvalidValue, "metadata_branch", SeverityError},
	{"absolute dir", "changes_dir: /etc/changes\n", CodeInvalidValue, "changes_dir", SeverityError},
	{"unclean dir", "changes_dir: docs/../etc\n", CodeInvalidValue, "changes_dir", SeverityError},
	{"empty change_types", "change_types: []\n", CodeInvalidValue, "change_types", SeverityError},
	{"duplicate change type", "change_types: [feat, feat]\n", CodeInvalidValue, "change_types", SeverityError},
	{"reserved change type", "change_types: [all]\n", CodeInvalidValue, "change_types", SeverityError},
	{"bad token pattern", "change_types: [Feat]\n", CodeInvalidValue, "change_types", SeverityError},
	{"obsolete scalar auto_capture true", "auto_capture: true\n", CodeInvalidValue, "auto_capture", SeverityError},
	{"obsolete scalar auto_capture false", "auto_capture: false\n", CodeInvalidValue, "auto_capture", SeverityError},
	{"interior node as scalar", "finalize: local\n", CodeInvalidType, "finalize", SeverityError},
	{"harness typo", "agents:\n  cluade:\n    adr: {model: m, effort: low}\n", CodeUnknownKey, "agents.cluade", SeverityError},
	{"agent typo", "agents:\n  claude:\n    adr-writer: {model: m, effort: low}\n", CodeUnknownKey, "agents.claude.adr-writer", SeverityError},
	{"agent field typo", "agents:\n  claude:\n    adr: {model: m, efort: low}\n", CodeUnknownKey, "agents.claude.adr.efort", SeverityError},
	{"model with space", "agents:\n  claude:\n    adr: {model: claude opus, effort: low}\n", CodeInvalidValue, "agents.claude.adr.model", SeverityError},
	{"runner name typo", "runners:\n  codexx:\n    sandbox: workspace-write\n", CodeUnknownKey, "runners.codexx", SeverityError},
	{"runner key typo", "runners:\n  codex:\n    sandbx: workspace-write\n", CodeUnknownKey, "runners.codex.sandbx", SeverityError},
	{"bad sandbox enum", "runners:\n  codex:\n    sandbox: yolo\n", CodeInvalidValue, "runners.codex.sandbox", SeverityError},
	{"github_project map ok", "github_project: {owner: acme, number: 7}\n", "", "", ""},
	{"github_project bad number", "github_project: {owner: acme, number: 0}\n", CodeInvalidValue, "github_project", SeverityError},
	{"github_project unknown field", "github_project: {owner: acme, num: 7}\n", CodeUnknownKey, "github_project.num", SeverityError},
	{"dummy surfaces bad token", "dummy_mode:\n  surfaces: [banners]\n", CodeInvalidValue, "dummy_mode.surfaces", SeverityError},
	{"unknown board token is a warning", "board_surfaces: [inline, trello]\n", CodeUnknownKey, "board_surfaces", SeverityWarning},
	{"runtime.bash obsolete every layer", "runtime:\n  bash: /bin/bash\n", CodeObsoleteSetting, "runtime.bash", SeverityWarning},
}
```

- [ ] **Step 2: Run to verify failure**: `go test ./internal/config/ -run TestDecode -count=1` → FAIL.

- [ ] **Step 3: Implement `decode.go`** — a recursive mapping walk carrying the dotted-path prefix; registry lookup with dynamic-pattern matching for `agents.`/`runners.` subtrees; per-leaf validation through the spec's validator; the special rules above.

- [ ] **Step 4: Run to verify pass**: `go test ./internal/config/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/decode.go internal/config/decode_test.go
git commit -m "feat(0305): typed per-leaf layer decode against the registry"
```

---

### Task 4: Built-in defaults and the frozen sidecar parity fixture

**Files:**
- Create: `internal/config/defaults.go`
- Create: `testdata/repositories/v0.9.2/agents-harness-defaults.yml` (byte-exact copy of `agents/harness-defaults.yml`)
- Create: `testdata/repositories/v0.9.2/PROVENANCE.md`
- Test: `internal/config/defaults_test.go`

**Interfaces:**
- Consumes: Reference A/C, registry.
- Produces:

```go
// builtinProvenance is the provenance every default carries.
func builtinProvenance() Provenance // {Layer: LayerBuiltIn, Source: "built-in"}

// builtinEffective returns the complete default Effective (Reference B default
// column; IntegrationBranch carries the raw "auto" here — resolution replaces
// it) with Explicit == false everywhere.
func builtinEffective() Effective

// builtinAgents returns the full 16×4 table of Reference C (cursor efforts
// suppressed to "").
func builtinAgents() AgentsTable
```

- [ ] **Step 1: Freeze the sidecar fixture**

```bash
mkdir -p testdata/repositories/v0.9.2
cp agents/harness-defaults.yml testdata/repositories/v0.9.2/agents-harness-defaults.yml
```

Write `testdata/repositories/v0.9.2/PROVENANCE.md`: source repo `danielhanold/docket`, commit `096c48de`, date 2026-08-13, "byte-exact copy of agents/harness-defaults.yml (the final v0.9.2 shipped agent defaults) plus the configuration-only fixtures of change 0305; no redaction". (This one file covers the whole `v0.9.2/` tree per `testdata/README.md`; fixture subdirs added in Task 9 extend it with one line each.)

- [ ] **Step 2: Write the failing tests**:

```go
// TestBuiltinAgentsParityWithFrozenSidecar parses the frozen sidecar with
// yaml.v3 and compares against builtinAgents() as WHOLE structures, both
// directions in one DeepEqual: every (harness, agent) pair present in both,
// model values equal, and effort equal — where the sidecar says `auto` the
// Go table must hold "". Vendor-ID VALIDITY is deliberately not asserted:
// that oracle lives outside the repo (named human verification item).
func TestBuiltinAgentsParityWithFrozenSidecar(t *testing.T) { … }

// TestBuiltinAgentsShape: exactly 4 harnesses, each with exactly the 16
// short names; every model non-empty and space-free; every entry's
// Provenance == builtinProvenance(), Explicit == false.

// TestBuiltinEffectiveMatchesRegistryDefaults: for every registry row with a
// non-nil default that lands in Effective, the corresponding builtinEffective()
// leaf equals it (metadata_branch "docket", integration_branch "auto",
// dirs, finalize local/auto/false, learnings.enabled true, reclaim 72/false,
// review minor/10, gate_observation_budget 30, board [inline],
// change_types [chore docs feat fix refactor perf]).
```

- [ ] **Step 3: Run to verify failure**: `go test ./internal/config/ -run TestBuiltin -count=1` → FAIL.

- [ ] **Step 4: Implement `defaults.go`** with the Reference C table as a literal.

- [ ] **Step 5: Run to verify pass**, then mutation-probe parity: change one model string in `defaults.go` (e.g. claude adr → `claude-opus-6`) → `go test -run TestBuiltinAgentsParityWithFrozenSidecar -count=1 ./internal/config/` must FAIL; restore by re-editing.

- [ ] **Step 6: Commit**

```bash
git add internal/config/defaults.go internal/config/defaults_test.go testdata/repositories/v0.9.2/agents-harness-defaults.yml testdata/repositories/v0.9.2/PROVENANCE.md
git commit -m "feat(0305): built-in defaults with 16x4 agent registry and frozen sidecar parity"
```

---

### Task 5: Four-layer resolution with provenance and fences

**Files:**
- Create: `internal/config/resolve.go`
- Test: `internal/config/resolve_test.go`

**Interfaces:**
- Consumes: `parseLayer`, `decodeLayer`, `builtinEffective`, `builtinAgents`, registry.
- Produces: `Resolve(sources []Source, rctx ResolveContext) (*Snapshot, []Diagnostic, error)` (Reference A signature) plus the internal `resolution` struct `capability.go` consumes:

```go
type resolution struct {
	effective Effective
	// honored explicit declarations by path, HIGHEST honored layer's decl —
	// after fence exclusion. capability.go keys activation on these.
	declared map[string]leafDecl
	// every declaration INCLUDING fenced-away ones, for classifier context
	// (e.g. machine github token reporting). Ordered low→high layer, then
	// document order.
	allDecls []leafDecl
	diags    []Diagnostic
}

func resolve(sources []Source, rctx ResolveContext) (*resolution, error)
```

Resolution algorithm (per Global Constraints and Reference B):

1. Assert source order/layers: callers pass only `global`, `repository`, `repository-local` (each at most once, in that order); the built-in layer is synthesized. A violation is a programming error → plain `error`.
2. Parse + decode each source; collect all diagnostics. If any invalid-class diagnostic exists after ALL layers are processed (validate the whole input set first — never stop at the first bad layer), return `ErrInvalidConfig` with the complete diagnostic set.
3. Fences: a `scopeRepoFenced` leaf declared in `global` or `repository-local` → `fenced-setting-ignored` warning (keyed on the DECLARATION's provenance, regardless of other layers) and exclusion. Same mechanism for `scopeLocalOnly` declared in `repository`. The `github` token inside a machine layer's `board_surfaces` list → token dropped + `fenced-setting-ignored` warning; remaining tokens still compete.
4. Leaf-by-leaf precedence over honored declarations: built-in < global < repository < repository-local. Lists replace whole. `Explicit` = some honored layer declared it (true even when the declared value equals the default — explicit-default provenance points at the declaring layer).
5. Agents: model and effort resolve **independently**; within one layer, `agents.<h>.<a>.<f>` falls back to `agents.default.<a>.<f>` before the next layer is consulted. Only built-in + **global** land in `Effective.Agents` (repo-layer agent declarations stay in `declared`/`allDecls` for the classifier, never in the table). `effort: auto` resolves to `""` with the declaring provenance.
6. `finalize.test_command: auto` → `""`. `integration_branch: auto` → `rctx.DefaultBranch`, or `ErrMissingResolutionContext` when empty (provenance of the resolved value stays with whichever layer supplied `auto`).
7. Cross-leaf: effective `auto_capture.types` list ⊄ effective `change_types` → `invalid-value` error on `auto_capture.types` (post-resolution — the only check that must run after precedence).
8. Sort diagnostics per Global Constraints.

`Resolve` = `resolve` + `classify` (Task 6) + validity gate; until Task 6 lands, `Resolve` returns snapshots with `Capabilities: nil` and `capability_test.go` completes it. Keep `Resolve` compiling from this task onward.

- [ ] **Step 1: Write the failing tests** (helpers: `srcG/srcR/srcL(yaml string) Source`; `mustResolve(t, sources, ctx)`):

```go
// TestPrecedencePerLeaf: global sets learnings.cap 100, repo sets 200,
// local sets 300 → 300 wins, Provenance.Layer == repository-local.
// Repo alone → repository. Nothing → built-in 300? no: built-in 300 is the
// default; use distinct values (built-in 300, G 100, R 200, L 250).
// TestExplicitDefaultProvenance: repo declares `reclaim: {auto: false}`
// (the default value) → Explicit true, Layer repository.
// TestListReplacesWhole: global change_types [feat], repo change_types
// [chore, docs] → effective [chore, docs] exactly (never a merge).
// TestEveryFence: table over the 8 repo-fenced paths × both machine layers:
// declaring the key in that layer yields fenced-setting-ignored (warning,
// declaration provenance) and the effective value comes from the next
// honored layer (assert BOTH halves — the warning alone can be a false
// alarm; learnings guard-keyed-on-presence-not-provenance).
// TestRuntimeBashCommittedFence: runtime.bash in the repo layer → warned,
// ignored (and obsolete-setting on top).
// TestBoardGithubTokenMachineFence: local board_surfaces [inline, github]
// → effective [inline], fenced-setting-ignored warning.
// TestAgentsHarnessFirstFallback: global sets agents.default.adr.model=X and
// agents.claude.adr.effort=high → claude/adr resolves model X (default
// fallback, same layer) + effort high; codex/adr resolves model X + built-in
// effort xhigh. Model and effort provenance differ — assert both.
// TestAgentsEffortAuto: global agents.claude.adr.effort=auto → Effort.Value
// "", Explicit true, provenance global.
// TestAgentsRepoLayerExcludedFromEffective: repo sets
// agents.claude.adr.model=Y → Effective.Agents claude/adr model stays
// built-in claude-opus-5 (the repo declaration is a capability question,
// not effective policy).
// TestIntegrationBranchAuto: default + ctx{DefaultBranch: "main"} → "main".
// Repo sets integration_branch: develop → "develop", no ctx needed.
// TestMissingResolutionContext: default + empty ctx → ErrMissingResolutionContext.
// TestTestCommandAutoUnsets: repo finalize.test_command: auto → "".
// TestAutoCaptureTypesSubset: repo change_types [feat], global
// auto_capture.types [fix] → invalid-value on auto_capture.types.
// TestInvalidLayerFailsWhole: valid global + malformed repo → ErrInvalidConfig,
// diagnostics include BOTH layers' findings (whole-input-set validation).
// TestDiagnosticOrdering: craft errors+warnings+infos out of order → returned
// sorted severity/path/code.
```

- [ ] **Step 2: Run to verify failure**: `go test ./internal/config/ -run 'TestPrecedence|TestExplicit|TestList|TestFence|TestEveryFence|TestBoard|TestAgents|TestIntegration|TestMissing|TestTestCommand|TestAutoCapture|TestInvalid|TestDiagnosticOrdering' -count=1` → FAIL.

- [ ] **Step 3: Implement `resolve.go`** per the algorithm above.

- [ ] **Step 4: Run to verify pass**: `go test ./internal/config/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/resolve.go internal/config/resolve_test.go
git commit -m "feat(0305): four-layer per-leaf resolution with provenance and coordination fences"
```

---

### Task 6: Capability classifier

**Files:**
- Create: `internal/config/capability.go`
- Test: `internal/config/capability_test.go`

**Interfaces:**
- Consumes: `resolution` (Task 5), registry dispositions.
- Produces: `func classify(res *resolution) ([]Capability, []Diagnostic)` — wired into `Resolve` so `Snapshot.Capabilities` is now populated and `Snapshot.Diagnostics` includes classifier diagnostics.

Classifier rules (the spec matrix, mechanically):

- A `Capability` entry is emitted for every **explicitly declared, honored** setting whose disposition is not plain-supported (obsolete, inert, inert-companion, deferred, deferred-active, agents-repo-layer), and for value-conditioned rows when the deferring value won (`finalize.gate` ∈ {ci, both}; a repo-honored `github` board token). Undeclared deferred settings emit nothing (off is off).
- `Active` = the deferred/dropped behavior is actually requested per Reference B's activation column, evaluated on the RESOLVED configuration (aggregate rules): `auto_capture.types` is Active only when effective `auto_capture.enabled` is true; `dummy_mode.persona`/`.surfaces` only when effective `dummy_mode.enabled` is true; `runners.*` leaves only when an honored agents entry declares `runner: <that name>`; inert/historical rows are never Active.
- `MutationBlock` = Active AND disposition ∈ {deferred, deferred-by-value, deferred-active, dropped}. Inert/obsolete rows never block. Every MutationBlock capability also emits one `deferred-capability-requested` diagnostic (SeverityError, same path/provenance, remedy naming the exact edit — e.g. "set auto_capture.enabled: false, or remove the key").
- Non-blocking classifier diagnostics: explicit inactive deferred leaf → `deferred-setting` info; explicit inert leaf → `inert-setting` info; effective `learnings.enabled: true` → one `deferred-setting` info on path `learnings.enabled` ("automatic harvest, index rendering, capacity checks, and promotion are deferred in Go v1; manual learning reads and explicit record/update remain supported").
- `Reason` states what the classification means in one sentence; `Remedy` (blockers and obsolete/inert only where actionable) names the edit.
- Output sorted by path; the complete blocker set is always present (never first-only).

- [ ] **Step 1: Write the failing tests.** One table-driven test whose rows cover EVERY Reference B matrix row — each case: layer yaml per layer + `ResolveContext{DefaultBranch: "main"}` → expected `[]Capability` (path, classification, active, mutation_block) and expected blocker-diagnostic paths. Mandatory rows (beyond one per matrix line):

```go
// gate ci blocks:        repo "finalize: {gate: ci}"      → {finalize.gate, deferred, active, block}
// gate both blocks:      repo "finalize: {gate: both}"    → same
// gate off supported:    repo "finalize: {gate: off}"     → no capability
// skip_results repo true:                                  → block
// skip_results explicit false:                             → {…, deferred, inactive, no-block} + deferred-setting info
// build.checkpoint true → block; terminal_publish repo true → block
// auto_groom true → block; auto_capture.enabled true → block
// auto_capture.types with enabled false → inert companion, inactive, no block
// auto_capture.types with enabled true  → active (blocks THROUGH enabled; the
//   types entry itself: Active true, MutationBlock false — enabled carries the block)
// dummy_mode trio: enabled true blocks; persona/surfaces follow enabled
// board github repo → block (dropped); board github local → NO capability
//   (fenced away), fenced warning instead; board unknown token → warning only
// learnings.cap explicit → inert-setting info, no block even at enabled true
// delegation_observation_budget explicit → inert info
// github_project explicit map → inert info; agent_harnesses explicit → inert info
// skills.build: docket-build (the shipped default, repeated) → STILL blocks
// skills.review: auto → blocks
// agents repo layer model equal to shipped default → STILL blocks
// agents repo-local layer effort → blocks
// agents global layer model/effort → supported, NO capability entry
// agents...runner: codex (any layer) → blocks; runners.codex.sandbox declared
//   with no runner assignment → inert companion inactive; with a global
//   agents.claude.adr.runner: codex → runners.codex.sandbox Active true
//   (runner leaf itself carries the block)
// runtime.bash → obsolete capability, never blocks
// multi-blocker completeness: one fixture layering auto_capture.enabled true
//   (global) + build.checkpoint true, skip_results true, terminal_publish true
//   (repo) → EXACTLY four deferred-capability-requested diagnostics in path
//   order: auto_capture.enabled, build.checkpoint,
//   finalize.skip_results_only_delta, terminal_publish.
```

- [ ] **Step 2: Run to verify failure**: `go test ./internal/config/ -run TestClassify -count=1` → FAIL.

- [ ] **Step 3: Implement `capability.go`**; wire `classify` into `Resolve`.

- [ ] **Step 4: Run to verify pass**: `go test ./internal/config/ -count=1` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/capability.go internal/config/capability_test.go internal/config/resolve.go
git commit -m "feat(0305): exhaustive capability classifier with aggregate activation rules"
```

---

### Task 7: Mutation preflight seam

**Files:**
- Create: `internal/config/preflight.go`
- Test: `internal/config/preflight_test.go`

**Interfaces:**
- Consumes: `Snapshot`.
- Produces: `PreflightMutation(s *Snapshot) PreflightDecision` and `GuardMutation(s *Snapshot, continue_ func() error) error` (Reference A signatures). `PreflightMutation` collects the `deferred-capability-requested` diagnostics from `s.Diagnostics` (path order preserved); `Allowed` = zero blockers. `GuardMutation` returns `fmt.Errorf("%w: %d blocker(s), first: %s", ErrUnsupportedConfig, n, firstPath)` WITHOUT calling `continue_` when blocked; otherwise returns `continue_()`'s error verbatim.

- [ ] **Step 1: Write the failing tests**:

```go
// TestPreflightAllowed: sparse default config (+ctx) → Allowed true, no blockers.
// TestPreflightBlockedComplete: the four-blocker fixture from Task 6 →
// Allowed false, Blockers exactly the four paths in order.
// TestGuardMutationRefusesContinuation: blocked snapshot; sentinel :=
// false; err := GuardMutation(s, func() error { sentinel = true; return nil });
// assert errors.Is(err, ErrUnsupportedConfig) && !sentinel — the downstream
// mutation seam is provably not entered.
// TestGuardMutationRunsWhenAllowed: allowed snapshot → sentinel true, err nil.
// TestGuardMutationPropagatesContinuationError: allowed + failing continuation
// → that exact error back.
```

- [ ] **Step 2: Run to verify failure**: `go test ./internal/config/ -run 'TestPreflight|TestGuardMutation' -count=1` → FAIL.

- [ ] **Step 3: Implement `preflight.go`.**

- [ ] **Step 4: Run to verify pass**, then the spec's REQUIRED mutation probe: comment out the blocked-return inside `GuardMutation` (so it always calls `continue_`) → `go test -run TestGuardMutationRefusesContinuation -count=1 ./internal/config/` must FAIL (sentinel reached). Restore by re-editing and re-run to green. `-count=1` is mandatory here — a cached verdict fabricates the probe.

- [ ] **Step 5: Commit**

```bash
git add internal/config/preflight.go internal/config/preflight_test.go
git commit -m "feat(0305): mutation preflight seam with continuation guard"
```

---

### Task 8: Filesystem adapter

**Files:**
- Create: `internal/config/fs.go`
- Test: `internal/config/fs_test.go`

**Interfaces:**
- Consumes: `Source`.
- Produces: `FSOptions` + `LoadFilesystemSources` (Reference A signatures). Behavior: `RepoDir` required (empty → error), cleaned via `filepath.Abs` + `filepath.Clean`, used verbatim — no Git discovery, no parent walking. Global path: `opts.GlobalPath` when non-empty, else `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. Reads, in low→high order: global, `RepoDir/.docket.yml` (repository), `RepoDir/.docket.local.yml` (repository-local). `os.IsNotExist` → layer absent (skipped); any other read error → returned error naming the path. Source.Name = `.docket.yml` / `.docket.local.yml` / the global file's absolute path.

- [ ] **Step 1: Write the failing tests** (every test pins `t.Setenv("XDG_CONFIG_HOME", tmp)` and `t.Setenv("HOME", tmp)` FIRST — the developer's real global config must be unreachable even on assertion failure):

```go
// TestLoadAllThreeLayers: temp repo dir with both files + temp XDG global →
// three sources, correct order/layers/names/bytes.
// TestLoadMissingFilesAreAbsent: empty repo dir + empty XDG → zero sources, nil error.
// TestLoadGlobalPathOverride: opts.GlobalPath set → that file read, XDG ignored.
// TestLoadXDGDefaultPath: XDG_CONFIG_HOME=tmp, file at tmp/docket/config.yml → found.
// TestLoadHomeFallback: XDG_CONFIG_HOME="", HOME=tmp, file at tmp/.config/docket/config.yml → found.
// TestLoadRepoDirRequired: empty RepoDir → error.
// TestLoadRelativeRepoDirCleaned: relative RepoDir resolves against cwd to an absolute path.
```

- [ ] **Step 2: Run to verify failure** → FAIL. **Step 3: Implement `fs.go`.** **Step 4: Run to verify pass**: `go test ./internal/config/ -count=1`.

- [ ] **Step 5: Commit**

```bash
git add internal/config/fs.go internal/config/fs_test.go
git commit -m "feat(0305): read-only filesystem source adapter with XDG global"
```

---

### Task 9: Frozen v0.9.2 repository fixtures + end-to-end resolution tests

**Files:**
- Create: `testdata/repositories/v0.9.2/<fixture>/…` (tree below)
- Modify: `testdata/repositories/v0.9.2/PROVENANCE.md` (one line per fixture)
- Test: `internal/config/fixtures_test.go`

Fixture layout convention: each fixture dir holds `repo/` (the repository directory handed to `FSOptions.RepoDir`) and optionally `xdg/docket/config.yml` (handed via `GlobalPath: <fixture>/xdg/docket/config.yml`). Configuration files only — no Markdown documents (0306's territory).

```
sparse-defaults/repo/                      # no config files at all
example-activated/repo/.docket.yml         # the canonical example surface, active valid values:
                                           # every supported key set to its default explicitly, plus
                                           # gate: local, test_command: auto — exercises
                                           # explicit-default provenance at scale
four-layer-collision/{repo/.docket.yml,repo/.docket.local.yml,xdg/docket/config.yml}
                                           # scalar (learnings.cap G100/R200/L250), list
                                           # (change_types), nested (reclaim.lease_ttl), and agent
                                           # (agents.default.adr.model in global) collisions
mode-main-custom-paths/repo/.docket.yml    # metadata_branch: main, integration_branch: develop,
                                           # changes_dir: planning/changes, adrs_dir: planning/adrs,
                                           # results_dir: planning/results
mode-docket/repo/.docket.yml               # metadata_branch: docket (explicit), integration_branch: auto
fenced-machine-keys/{repo/.docket.local.yml,xdg/docket/config.yml}
                                           # every repo-fenced key declared once in each machine
                                           # layer (split between them), incl. board github token
docket-self/{repo/.docket.yml,xdg/docket/config.yml}
                                           # THIS repo's committed .docket.yml verbatim (copy) +
                                           # a global config.yml with auto_capture: {enabled: true}
                                           # → the four-blocker envelope
deferred-pairs/repo/.docket.yml            # every deferred setting EXPLICITLY inactive (false /
                                           # local / inline) → zero blockers, deferred-setting infos
deferred-active/repo/.docket.yml           # every repo-settable deferred setting active → the
                                           # complete blocker set in one document
invalid/malformed/repo/.docket.yml         # "a: [unclosed"
invalid/duplicate-key/repo/.docket.yml
invalid/alias-merge/repo/.docket.yml
invalid/multi-doc/repo/.docket.yml
invalid/wrong-type/repo/.docket.yml        # learnings: {cap: many}
invalid/bad-enum/repo/.docket.yml          # finalize: {gate: sometimes}
invalid/scalar-auto-capture/repo/.docket.yml  # auto_capture: true
invalid/unknown-key/repo/.docket.yml       # integration_brach: main
invalid/model-typo/repo/.docket.local.yml  # agents: {cluade: {adr: {model: m, effort: low}}}
```

- [ ] **Step 1: Write the fixtures** exactly as annotated. `docket-self/repo/.docket.yml` is a byte copy of this repo's committed `.docket.yml` (from the worktree root). Update `PROVENANCE.md` with one line per fixture (authored for change 0305; docket-self copied from commit `096c48de`).

- [ ] **Step 2: Write the failing tests** — one table over fixture dirs driving `LoadFilesystemSources` + `Resolve` end-to-end:

```go
// each case: fixture name, ctx (DefaultBranch "main" everywhere), then:
//   sparse-defaults    → valid; every leaf Explicit false, built-in provenance;
//                        integration_branch "main"; preflight allowed
//   example-activated  → valid; every declared leaf Explicit true, repository
//                        provenance; values equal built-ins
//   four-layer-collision → learnings.cap 250 (repository-local),
//                        change_types from the repo layer, reclaim.lease_ttl per
//                        its winning layer, claude/adr model from global default
//                        fallback — assert value AND provenance layer each
//   mode-main-custom-paths → metadata_branch "main", integration_branch
//                        "develop" (no ctx dependence), three custom dirs
//   mode-docket        → metadata_branch "docket", integration_branch "main" via ctx
//   fenced-machine-keys → valid; every fenced declaration → fenced-setting-ignored
//                        warning; effective values all built-in; preflight ALLOWED
//                        (fences never block)
//   docket-self        → valid; preflight BLOCKED with exactly
//                        [auto_capture.enabled, build.checkpoint,
//                         finalize.skip_results_only_delta, terminal_publish]
//   deferred-pairs     → valid; zero blockers; deferred-setting infos present
//   deferred-active    → valid; blocker set == every repo-settable deferred row
//   invalid/*          → ErrInvalidConfig; the expected code present
//                        (malformed→invalid-yaml, duplicate-key, alias-merge→
//                        invalid-yaml, multi-doc→invalid-yaml, wrong-type→
//                        invalid-type, bad-enum→invalid-value, scalar-auto-capture→
//                        invalid-value, unknown-key→unknown-key, model-typo→unknown-key)
```

- [ ] **Step 3: Run to verify failure, implement fixture wiring, run to verify pass**: `go test ./internal/config/ -count=1` → PASS. (Fixture paths from the package dir: `../../testdata/repositories/v0.9.2/…`.)

- [ ] **Step 4: Commit**

```bash
git add testdata/repositories/v0.9.2 internal/config/fixtures_test.go
git commit -m "test(0305): frozen v0.9.2 configuration fixtures and end-to-end resolution coverage"
```

---

### Task 10: `diagnostic.config` / `config.preflight` application operations

**Files:**
- Create: `internal/app/config.go`
- Test: `internal/app/config_test.go`

**Interfaces:**
- Consumes: `internal/config` public API; 0304's `Envelope`, `Result*`, `OperationResult`.
- Produces:

```go
// Stable reasons added alongside 0304's CLI reasons.
const (
	ReasonInvalidConfig            = "invalid-config"
	ReasonMissingResolutionContext = "missing-resolution-context"
	ReasonDeferredCapRequested     = "deferred-capability-requested"
)

// ConfigInspectionResult is the diagnostic.config / config.preflight document.
type ConfigInspectionResult struct {
	Envelope
	SourceMode      string              `json:"source_mode"` // "filesystem"
	MutationAllowed bool                `json:"mutation_allowed"`
	Effective       *config.Effective   `json:"effective,omitempty"`
	Capabilities    []config.Capability `json:"capabilities,omitempty"`
	Diagnostics     []config.Diagnostic `json:"diagnostics"`
	Reason          string              `json:"reason,omitempty"`
	Message         string              `json:"message,omitempty"`
}

// DiagnosticConfig computes the whole outcome per Reference D.
// forMutation selects the config.preflight operation and the
// unsupported-config mapping.
func DiagnosticConfig(sources []config.Source, rctx config.ResolveContext, forMutation bool) ConfigInspectionResult
```

Mapping (Reference D exactly): resolve; `ErrMissingResolutionContext` → `invalid-input`/`missing-resolution-context`; `ErrInvalidConfig` → `invalid-input`/`invalid-config`; valid + inspection → `applied` with `MutationAllowed` from `PreflightMutation`; valid + preflight + blockers → `unsupported-config`/`deferred-capability-requested`; valid + preflight + allowed → `applied`. `Diagnostics` always carries the full sorted set. Capabilities empty→ keep `[]` (marshal as omitted is fine per Reference D note; valid snapshots carry the slice even when empty — use a non-nil empty slice on valid results so `"capabilities": []` appears).

`HumanText()`: deterministic, grouped:

```
configuration: valid            (or: invalid)
mutation: allowed               (or: blocked (4 blockers), or: n/a on invalid)

effective (winning layer):
  metadata_branch = docket  [built-in]
  integration_branch = main  [built-in, auto→main]
  changes_dir = docs/changes  [built-in]
  …one line per Effective leaf, agents grouped last as
  agents.claude.adr = claude-opus-5 / low  [built-in]…

capabilities:
  finalize.gate: deferred (active, blocks mutation) — …reason… | remedy: …
  …path order; omitted when empty

diagnostics:
  error   deferred-capability-requested auto_capture.enabled — …message…
  warning fenced-setting-ignored terminal_publish — …
  info    inert-setting learnings.cap — …
  …severity groups, omitted when empty
```

On invalid results the effective section is replaced by `effective: (unavailable — configuration invalid)`.

- [ ] **Step 1: Write the failing tests**:

```go
// TestDiagnosticConfigApplied: sparse sources + ctx → operation
// "diagnostic.config", ResultApplied, SourceMode "filesystem",
// MutationAllowed true, Effective non-nil, Capabilities []
// TestDiagnosticConfigAppliedWhileBlocked: docket-self-shaped sources,
// inspection mode → ResultApplied, MutationAllowed false (applied even when
// blocked — the spec's rule)
// TestPreflightUnsupported: same sources, forMutation → operation
// "config.preflight", ResultUnsupportedConfig, Reason
// deferred-capability-requested, Diagnostics containing all four blockers
// TestPreflightAllowedResult: sparse + forMutation → applied
// TestInvalidConfigResult: malformed source → ResultInvalidInput, Reason
// invalid-config, Effective nil, exit code via ExitCode == 2
// TestMissingContextResult: sparse + empty ctx → ResultInvalidInput, Reason
// missing-resolution-context
// TestHumanTextGrouping: blocked snapshot's HumanText contains
// "mutation: blocked (4 blockers)", an "effective" section line for
// metadata_branch with its layer, and a diagnostics section with the error
// group first; no raw file contents or environment values anywhere
// TestJSONShape: json.Marshal of an applied result → keys exactly
// {protocol_version, operation, result, source_mode, mutation_allowed,
// effective, capabilities, diagnostics} (decode to map, assert key set)
```

- [ ] **Step 2: Run to verify failure**: `go test ./internal/app/ -count=1` → FAIL. **Step 3: Implement.** **Step 4: Run to verify pass** (`go test ./internal/app/ -count=1`; `internal/app/shadow_test.go` guards envelope-field shadowing — if it objects to the embedded Envelope usage, follow its documented pattern).

- [ ] **Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "feat(0305): diagnostic.config and config.preflight operations"
```

---

### Task 11: CLI wiring, built-binary protocol coverage, whole-tree gate

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go` (extend), Create: `cmd/docket/config_cli_test.go`

**Interfaces:**
- Consumes: `app.DiagnosticConfig`, `config.LoadFilesystemSources`, existing `Run` wiring and `diagnosticCmd` group.
- Produces: `docket diagnostic config` with flags `--repo-dir` (string, **required** via `MarkFlagRequired`), `--default-branch` (string, optional), `--for-mutation` (bool). RunE body:

```go
configCmd := &cobra.Command{
	Use:   "config",
	Short: "Inspect resolved configuration and its capability envelope",
	Args:  cobra.NoArgs,
	RunE: func(c *cobra.Command, _ []string) error {
		repoDir, _ := c.Flags().GetString("repo-dir")
		defBranch, _ := c.Flags().GetString("default-branch")
		forMutation, _ := c.Flags().GetBool("for-mutation")
		sources, err := config.LoadFilesystemSources(config.FSOptions{RepoDir: repoDir})
		if err != nil {
			return err // unreadable file / bad repo-dir → invalid-arguments path
		}
		result = app.DiagnosticConfig(sources, config.ResolveContext{DefaultBranch: defBranch}, forMutation)
		return nil
	},
}
configCmd.Flags().String("repo-dir", "", "repository directory to inspect (required; used verbatim, no Git discovery)")
configCmd.Flags().String("default-branch", "", "default branch supplied to integration_branch: auto")
configCmd.Flags().Bool("for-mutation", false, "run the mutation preflight (operation config.preflight)")
_ = configCmd.MarkFlagRequired("repo-dir")
diagnosticCmd.AddCommand(configCmd)
```

- [ ] **Step 1: Write failing built-binary tests** (`cmd/docket/config_cli_test.go`, reusing `TestMain`'s `binPath`, `run`, `assertOneJSONDocument`; every test creates a temp fixture repo dir — or copies a frozen fixture with `cp -R` semantics via Go — and pins `XDG_CONFIG_HOME`/`HOME` to a temp dir through `cmd.Env`; note `run` uses `exec.Command` so add an `runEnv` variant accepting env overrides):

```go
// TestConfigHumanInspection: sparse temp repo, --default-branch main →
// exit 0, stdout contains "configuration: valid" and "metadata_branch = docket"
// TestConfigJSONInspection: --json → one document; protocol_version 1,
// operation "diagnostic.config", result "applied", source_mode "filesystem",
// mutation_allowed true, effective object present; stderr EMPTY
// TestConfigJSONBlockedInspectionStillApplied: temp repo with the
// deferred-active fixture copied in → --json inspection: result "applied",
// mutation_allowed false
// TestConfigPreflightUnsupported: same repo, --for-mutation --json →
// operation "config.preflight", result "unsupported-config", reason
// "deferred-capability-requested", exit 1, diagnostics array carries every
// blocker (count equals the fixture's known blocker count)
// TestConfigPreflightAllowed: sparse repo, --for-mutation → exit 0, applied
// TestConfigInvalidInput: repo with "a: [unclosed" .docket.yml, --json →
// result "invalid-input", reason "invalid-config", exit 2, one document
// TestConfigMissingContext: sparse repo, NO --default-branch →
// "invalid-input", reason "missing-resolution-context", exit 2
// TestConfigMissingRepoDirFlag: no --repo-dir → CLI invalid-arguments error,
// exit 2 (human: stderr, empty stdout; matches 0304's flag-error contract)
// TestConfigGlobalConfigIsHermetic: XDG_CONFIG_HOME pointed at a temp dir
// containing docket/config.yml with auto_capture: {enabled: true} →
// preflight blocked; proves the global layer is read from the pinned env,
// and that tests never consult the developer's real one
```

- [ ] **Step 2: Extend `internal/cli/root_test.go`** minimally: `diagnostic config` without required flag → error path; `diagnostic config --repo-dir X` reaches the operation (in-process `Run` with temp dir).

- [ ] **Step 3: Run to verify failure**: `go test ./cmd/docket/ ./internal/cli/ -count=1` → FAIL. **Step 4: Implement the root.go wiring.** **Step 5: Run to verify pass**: `go test ./... -count=1` → all green.

- [ ] **Step 6: Whole-tree gate parity**: run `gofmt -l .` (empty), `go vet ./...` (clean), then `bash tests/test_go_toolchain.sh` from the worktree root and confirm every `ok - ` marker. Check `tests/runtime-budgets.tsv` for the `test_go_toolchain.sh` row: the new package grows its wall clock, so if the full-suite run (step 5 of the build) prints `OVER BUDGET:` for it, adjust that row in a dedicated commit recording the measured number in the commit message (see `scripts/run-tests.md` for the budget regime) — never silently.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go cmd/docket/config_cli_test.go
git commit -m "feat(0305): docket diagnostic config subcommand with protocol coverage"
```

---

## Acceptance check (maps to the spec's acceptance boundary)

1. Clean checkout resolves the full `v0.9.2` surface through all four layers with typed values and exact provenance — Tasks 5/9 fixtures.
2. Text and protocol-v1 JSON inspection without mutation — Tasks 10/11.
3. Every active deferred/dropped capability yields one complete `unsupported-config` preflight before the fake downstream mutation seam is entered — Tasks 6/7/11 (`docket-self` and `deferred-active` fixtures; `TestGuardMutationRefusesContinuation` plus its mandated mutation probe).
4. Nothing reads Git objects, writes metadata, or renders harness assets — enforced by the Global Constraints and the one-way package dependency direction.

Named human verification item to carry into the results file (no in-repo oracle exists): the Reference C model IDs are outside-truth; this change introduces **no new vendor ID** (all sixteen×four values are byte-copies of the already-shipped sidecar), so the item is a statement of that fact plus parity-test provenance, not a new certification run.
