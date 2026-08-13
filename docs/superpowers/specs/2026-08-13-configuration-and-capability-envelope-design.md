<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0305 — Configuration and capability envelope](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-13-0305-configuration-and-capability-envelope.md)**
<!-- docket:backlink:end -->

# Configuration and capability envelope

**Change:** 0305 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change gives Go one typed answer to two questions that every later repository operation must
ask: what configuration is effective, and may a mutation proceed under the capabilities Go v1
actually ships?

It loads the final Bash release's configuration surface with a real YAML parser, preserves the
four-layer precedence and coordination fences, records the winning layer for every value, and
classifies legacy settings as supported, obsolete, historical/inert, or deferred. A read-only
diagnostic may inspect any valid configuration. A mutation preflight returns
`unsupported-config` before a future transaction can begin when an active deferred or dropped
behavior is requested.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are governing constraints. This
spec resolves only change 0305. It does not reopen the hard-cutover, compatibility, capability, or
agent-first decisions in those documents, and it does not implement behavior assigned to changes
0306–0318.

## Landed foundation and independently deliverable result

Change 0304 has landed the Go module, Cobra command tree, protocol-v1 envelope, complete result
taxonomy, presenter, and repository Go-test gate. Change 0305 extends those contracts; it does not
replace them.

The independently reviewable deliverable is:

- an `internal/config` package that parses and resolves supplied configuration layers without Git,
  Markdown, domain, transaction, installation, or harness dependencies;
- built-in Go defaults for the retained configuration and the sixteen shipped agent model/effort
  entries across Claude, Codex, Cursor, and OpenCode;
- a read-only `docket diagnostic config` operation that inspects an explicit repository directory;
- a mutation-preflight mode that proves active unsupported behavior is refused with a typed result;
  and
- frozen `v0.9.2` configuration fixtures and exhaustive classification tests.

Later changes consume the resolved snapshot. Change 0308 supplies authoritative Git-object bytes,
0310 composes them into repository status, 0311 renders installation assets, and 0309/0312 enforce
the preflight at real transaction entry. None of those consumers is pulled forward here.

## Package design

### Core types

`internal/config` owns the configuration vocabulary and exposes immutable results. Concrete field
names may remain unexported when no other package needs them, but the package boundary has these
shapes:

```go
type LayerKind string

const (
    LayerBuiltIn        LayerKind = "built-in"
    LayerGlobal         LayerKind = "global"
    LayerRepository     LayerKind = "repository"
    LayerRepositoryLocal LayerKind = "repository-local"
)

type Source struct {
    Layer LayerKind
    Name  string
    Data  []byte
}

type ResolveContext struct {
    DefaultBranch string
}

type Provenance struct {
    Layer  LayerKind `json:"layer"`
    Source string    `json:"source"`
    Line   int       `json:"line,omitempty"`
    Column int       `json:"column,omitempty"`
}

type Value[T any] struct {
    Value      T
    Provenance Provenance
    Explicit   bool
}

type Snapshot struct {
    Effective    Effective
    Capabilities []Capability
    Diagnostics  []Diagnostic
}
```

`Effective` is a typed aggregate, not `map[string]any`. Its nested groups mirror the supported
configuration domains: repository layout, finalize, learnings, reclaim, review, observation
budgets, board, change taxonomy, and agent model/effort defaults. Inactive deferred companion
values remain available to diagnostics but never masquerade as effective supported policy.

`RawLayer` is private. It uses optional typed leaves so “absent,” “explicitly set to the default,”
and “set in a lower layer but overridden” remain distinguishable. Resolution itself sets
`Provenance`; no later code re-reads layers to infer where a value came from.

### One schema and policy registry

One package-owned registry defines each static `v0.9.2` path's:

- YAML type and value validator;
- built-in default, when one exists;
- merge rule (`scalar`, whole-list replacement, or field-wise nested merge);
- allowed layer scope and coordination-fence behavior; and
- capability classifier, including any value- or provenance-dependent activation rule.

The dynamic `agents.<harness>.<agent>` and `runners.<runner>` subtrees use validators registered
from the same root schema. Parser, resolver, diagnostic renderer, preflight, and exhaustive tests
all consume this registry. No second key list is maintained in production code.

Unknown top-level or nested schema paths are `invalid-config`, except where `v0.9.2` deliberately
defined warn-and-ignore behavior (unknown `skills` roles and unrecognized board-surface tokens).
Model IDs and effort strings remain opaque, space-free passthrough values; Docket does not claim
that an external vendor ID exists.

## YAML contract

Pin `go.yaml.in/yaml/v3` at `v3.0.4`. It is the maintained stable v3 line and exposes the node
locations needed for precise diagnostics. The v4 line is still pre-release during this design;
adopting a release candidate is unnecessary for the Go v1 foundation.

Parse each layer into a `yaml.Node` before typed decoding. This is required to:

- reject duplicate mapping keys before later values erase earlier ones;
- attach line and column to diagnostics;
- preserve declaration presence separately from the decoded zero value;
- enforce the final `v0.9.2` boolean spellings `true` and `false` even where a YAML library could
  coerce YAML 1.1 aliases such as `yes`, `no`, `on`, or `off`; and
- detect the old scalar `auto_capture` shape before normalization consumes the evidence.

Each file must contain zero or one YAML document whose root is a mapping. Empty files are equivalent
to an absent layer. Multiple documents, aliases, merge keys, duplicate keys, malformed YAML, and
schema type mismatches are `invalid-config`. Ordinary block and flow mappings/sequences, quoted
scalars, comments, and legal YAML indentation are accepted. This change parses configuration only;
it neither preserves nor rewrites source bytes. Loss-preserving Markdown/frontmatter work belongs
to 0306.

## Sources, precedence, and fences

The pure resolver accepts already-supplied `Source` values in low-to-high order:

```text
built-in < global machine config < committed repository config < repository-local machine config
```

Resolution is leaf-by-leaf. Lists such as `change_types` replace as a whole. Nested leaves resolve
independently. Agent `model` and `effort` fields resolve independently through their harness-first
fallback inside a layer before moving to the next layer.

The retained coordination fence applies to `metadata_branch`, `integration_branch`, `changes_dir`,
`adrs_dir`, `results_dir`, `github_project`, `terminal_publish`,
`finalize.skip_results_only_delta`, and the `github` board token. A fenced declaration in the global
or repository-local machine layer produces `fenced-setting-ignored`, is excluded from resolution,
and is never fatal by itself. The diagnostic keys on the declaration's provenance, not on whether
the same path exists in another layer.

`docket diagnostic config --repo-dir DIR --default-branch BRANCH` is a read-only filesystem adapter
around this core. It reads `DIR/.docket.yml`, `DIR/.docket.local.yml`, and the XDG global config
location; `DIR` is used verbatim after absolute-path cleanup and is not discovered with Git.
`--default-branch` supplies `ResolveContext.DefaultBranch` and is required only when the resolved
`integration_branch` is `auto`; omitting the needed context is `invalid-input` with stable reason
`missing-resolution-context`, not a guessed `main`. Test-only constructor inputs override all paths
and environment dependencies. The result identifies this as a filesystem inspection. It does not
claim the worktree copy is the authoritative remote object. Change 0308's object source later
supplies both authoritative committed bytes and the resolved default branch to the same resolver.

No code in this change fetches, invokes Git or `gh`, creates a worktree, edits a configuration file,
or writes generated wrappers.

## Capability model

```go
type Classification string

const (
    Supported  Classification = "supported"
    Obsolete   Classification = "obsolete"
    Inert      Classification = "inert"
    Deferred   Classification = "deferred"
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
```

Classification and activation are separate. An off switch for a deferred feature is valid and
non-blocking. A companion setting is inert while its feature is disabled. An explicit request for
deferred or dropped behavior blocks mutation. Read-only inspection still succeeds so the user can
see the complete remedy.

Invalid syntax, types, enums, or structural shapes are not capability classifications. They are
`invalid-config` and prevent both inspection results and mutation preflight from claiming a valid
snapshot.

### Exhaustive `v0.9.2` setting matrix

| Setting | Go v1 disposition | Activation and layer rule |
|---|---|---|
| `runtime.bash` | Obsolete | Warn and ignore in every layer; Go has no Bash-runtime requirement. |
| `metadata_branch` | Supported | Repository layer only; `docket` or `main`. Machine declarations are fenced and ignored. |
| `integration_branch` | Supported | Repository layer only; `auto` resolves to the supplied default-branch input, otherwise a non-empty branch name. Machine declarations are fenced and ignored. |
| `changes_dir`, `adrs_dir`, `results_dir` | Supported | Repository layer only; non-empty clean repository-relative paths. Machine declarations are fenced and ignored. |
| `finalize.gate` | Supported or deferred by value | `local` and `off` are supported. `ci` and `both` are active deferred requests and block mutation. |
| `finalize.test_command` | Supported | String; `auto` resolves to unset. Execution belongs to later workflow/process changes. |
| `finalize.require_pr_approval` | Supported | Boolean at any ordinary layer. |
| `finalize.skip_results_only_delta` | Deferred | `false` is inactive. Repository `true` blocks mutation. Machine declarations are fenced and ignored. |
| `learnings.enabled` | Supported | Gates retained learning consumption and explicit manual record/update. `true` also emits a non-blocking notice that automatic harvest/index/capacity/promotion are deferred. |
| `learnings.cap` | Historical/inert | Validate as a non-negative integer and report it; Go v1 performs no automated capacity check. It never blocks mutation alone. |
| `reclaim.lease_ttl`, `reclaim.auto` | Supported | Non-negative hours and Boolean respectively; consumed by 0316. |
| `build.checkpoint` | Deferred | `false` is inactive; `true` blocks mutation. |
| `review.min_fix_severity`, `review.max_fix_tasks` | Supported | Retain the `minor`/`important`/`blocker` enum and non-negative task count; consumed by 0315. |
| `gate_observation_budget` | Supported | Non-negative minutes; consumed by the native gate workflow. |
| `delegation_observation_budget` | Historical/inert | Validate and report. It has no effect without deferred runner delegation and never blocks alone. |
| `board_surfaces` | Supported or dropped by token | `inline` and an empty list are supported. A repository `github` token is an active dropped-capability request and blocks mutation. A machine `github` token is fenced and ignored. Unknown tokens retain `v0.9.2` warn-and-ignore behavior. |
| `github_project` | Historical/inert | Preserve and report without behavior. Machine declarations remain fenced and ignored. |
| `terminal_publish` | Deferred | `false` is inactive. Repository `true` blocks mutation; machine declarations are fenced and ignored. |
| `auto_groom` | Deferred | `false` is inactive; `true` blocks mutation. |
| `change_types` | Supported | Non-empty, duplicate-free whole-list replacement; tokens match `[a-z][a-z0-9-]*`; `all` and `untyped` remain reserved. |
| `auto_capture.enabled` | Deferred | `false` is inactive; `true` blocks mutation. The obsolete scalar `auto_capture` shape remains invalid with the nested replacement in its remedy. |
| `auto_capture.types` | Historical/inert companion | Validate `all` or a duplicate-free subset of effective `change_types`; blocks only through `auto_capture.enabled: true`. |
| `dummy_mode.enabled` | Deferred | `false` is inactive; `true` blocks mutation. |
| `dummy_mode.persona`, `dummy_mode.surfaces` | Historical/inert companions | Parse and report; block only through `dummy_mode.enabled: true`. Surfaces retain `all` or the final `v0.9.2` token set. |
| `agent_harnesses` | Historical/inert | Parse and report the list. Go installation uses detection or explicit install targets in 0311; this legacy generation-scope key has no Go v1 behavior. |
| global `agents.<harness-or-default>.<agent>.model` / `.effort` | Supported | The global machine file is the sole override layer. Values are opaque passthrough strings; `effort: auto` suppresses an effort pin. Built-ins cover all sixteen shipped agents on all four first-class harnesses. |
| repository or repository-local `agents...model` / `.effort` | Deferred | Any explicit value requests per-repository model routing and blocks mutation, even when equal to a shipped default. |
| any `agents...runner` | Deferred | Any explicit runner assignment requests cross-harness delegation and blocks mutation. |
| `runners.<name>.*` | Historical/inert companion | Parse the final Codex/OpenCode and shim keys for diagnostics; they do not block alone. A runner assignment is the activation signal. |
| `skills.brainstorm`, `.plan`, `.build`, `.review`, `.finish` | Deferred | Any explicit leaf requests skill rebinding or `auto` substitution and blocks mutation, even when it repeats the shipped default. Unknown role keys retain warn-and-ignore behavior. |

The agent registry recognizes `default`, `claude`, `codex`, `cursor`, and `opencode`; the sixteen
shipped agent short names are the final `v0.9.2` set. Model identifiers are never allowlisted.
Harness- or agent-name typos are invalid configuration because silently discarding an intended
model override would run the wrong model. The `v0.9.2` sidecar remains a frozen parity fixture while
the Go registry becomes the source later Go consumers import.

### Aggregate activation rules

The preflight evaluates the fully resolved configuration rather than blocking on mere declaration
of an inactive companion:

- `auto_capture.types` matters only when effective `auto_capture.enabled` is true;
- dummy persona/surfaces matter only when effective `dummy_mode.enabled` is true;
- runner settings matter only when an effective agent entry names that runner;
- learning capacity remains non-blocking even when learnings are enabled, per the approved manual
  learning exception; and
- all active blockers are returned together in deterministic path order, so users repair the
  configuration in one pass rather than discovering one setting per invocation.

## Diagnostics and protocol

```go
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
```

Stable codes include `invalid-yaml`, `duplicate-key`, `unknown-key`, `invalid-type`,
`invalid-value`, `fenced-setting-ignored`, `obsolete-setting`, `inert-setting`,
`deferred-setting`, and `deferred-capability-requested`. `message` and `remedy` are human prose;
consumers decide from the stable code, path, classification, layer, and result kind.

`docket diagnostic config` emits operation `diagnostic.config`. With a valid snapshot it returns
`applied`, even when `mutation_allowed` is false. Human text groups effective values by domain,
prints their winning layers, then groups diagnostics by severity. JSON extends 0304's envelope with:

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

`--for-mutation` performs no mutation. It changes the operation to `config.preflight`. A valid,
allowed snapshot returns `applied`; any active unsupported capability returns
`unsupported-config`, stable reason `deferred-capability-requested`, and the complete ordered
blocking diagnostic set. Invalid configuration returns `invalid-input`, stable reason
`invalid-config`. JSON mode preserves the one-document/empty-stderr contract and 0304's coarse exit
mapping.

The package exposes the same preflight as a typed function over `Snapshot`. A test-only downstream
sentinel proves the continuation is not called after an unsupported result. This is the seam later
transaction operations must invoke before they construct or enter a transaction; the transaction
engine and its call sites remain later-change work.

Diagnostics never include environment dumps or file contents. Paths name the setting and source
file; values appear only where needed for a safe, actionable remedy.

## Validation and testing

### Frozen compatibility fixtures

Add configuration-only fixtures under `testdata/repositories/v0.9.2/` without inventing Markdown
document behavior:

- sparse defaults with no config files;
- the canonical final `.docket.example.yml` surface activated with valid values;
- all four layers colliding on scalar, list, nested, and agent fields;
- both metadata modes and custom ledger paths;
- fenced keys in each machine layer;
- Docket's current configuration envelope, which actively requests auto-capture, terminal
  publishing, build checkpoints, and results-only gate skipping across its winning layers;
- every supported inactive/active deferred pair; and
- malformed YAML, duplicate keys, aliases/merges, multiple documents, wrong types, invalid enums,
  obsolete scalar `auto_capture`, unknown schema paths, and model-routing typos.

Fixtures that model global or repository-local config use isolated temporary XDG and repository
roots. Tests never read or write the developer's real global config.

### Required tests

- Table-driven parser tests cover every schema entry and both block/flow YAML spellings where the
  schema allows them.
- Resolution tests prove per-leaf precedence, whole-list replacement, independent model/effort
  fallback, explicit-default provenance, and every coordination fence.
- Capability tests cover every matrix row, including value- and layer-conditioned outcomes, and
  compare the complete blocker set rather than stopping at the first.
- A schema-coverage test derives the production path set from the registry and verifies every
  final `v0.9.2` fixture path is classified. Removing one registry entry must redden the test.
- Agent-default parity tests compare the Go registry with the frozen final
  `agents/harness-defaults.yml` set and values. Vendor IDs receive a named human verification item;
  no in-repo test pretends to prove an external model exists.
- Diagnostic tests prove stable codes, deterministic order, provenance, actionable remedies, and
  absence of raw environment/file leakage.
- Application and built-binary tests cover human and JSON inspection, invalid configuration, an
  allowed preflight, and a multi-blocker `unsupported-config` preflight while preserving the
  one-document protocol.
- Mutation tests remove or bypass the preflight continuation guard and prove the fake downstream
  sentinel is then reached; the intact implementation keeps it unreachable.
- All Go mutation probes and manual reruns use `go test -count=1` so the result cannot come from the
  Go test cache.
- The existing auto-discovered Go shell producer remains the whole-suite entry; this change does
  not add a second test command or CI provider.

## Alternatives considered

### Merge generic YAML maps, then decode once

This is compact but loses the facts Docket needs most: which layer supplied a value, whether an
explicit default was requested, and whether a fenced lower layer declared a key that a higher layer
also holds. Re-deriving provenance afterward repeats the class of bugs already seen in the Bash
resolver. Rejected in favor of typed per-layer patches and provenance set at resolution time.

### Port the Bash resolver branch by branch

A literal port would reproduce line-oriented YAML limitations, scatter defaults and fences across
conditionals, and couple Go to retired runtime mechanics. It would also make the capability envelope
an after-the-fact second pass. Rejected: one schema/policy registry gives both compatibility and an
exhaustive mutation gate.

### Adopt `go.yaml.in/yaml/v4` before its stable release

The maintained v4 line is the future-facing option, but it is pre-release during this design. The
stable v3 node API already meets this change's needs. Go v1 pins v3.0.4; a later dependency change
can adopt stable v4 with fixture parity rather than making the migration foundation depend on an RC.

### Silently disable every deferred behavior

This would let mutations proceed but violate the approved capability boundary: active deferred
behavior must fail safely, not degrade. It would be especially dangerous for terminal publishing,
GitHub board mirroring, skill rebinding, and per-repository model routing. Rejected in favor of
readable diagnostics plus pre-transaction refusal.

## Out of scope

- Markdown/frontmatter parsing, source-byte preservation, field/block patching, and canonical
  document rendering (0306).
- Changes, ADRs, lifecycle states, dependencies, stacks, selection, readiness, claim policy, and
  repository-wide domain validation (0307).
- Git discovery, default-branch probing, fetches, refs, object reads, blob identities, authoritative
  repository sources, or bootstrap/migration effects (0308).
- Transaction worktrees, leases, commits, pushes, retry, idempotency, or recovery pruning (0309).
- Status/health snapshot assembly or report design beyond this configuration diagnostic (0310).
- Embedded assets, wrapper generation, harness installation plans, or consuming global model/effort
  values into files (0311).
- Any change/ADR/learning mutation, board renderer, artifact link, or backlink (0312).
- Feature workspaces, GitHub PRs, build evidence, process supervision, claim/finalize orchestration,
  archive, reclaim execution, or stack close-out (0313–0316).
- Release archives, downloader behavior, on-target harness acceptance, self-hosting, configuration
  contraction, Bash removal, or cutover (0317–0318).
- Reintroducing autonomous grooming/capture, automatic learning work, dummy mode, terminal
  publishing, CI gates, results-only skipping, runner delegation, per-repository model routing,
  skill rebinding, GitHub backlog mirroring, or build checkpoints into Go v1.

## Acceptance boundary

Change 0305 is complete when a clean checkout can resolve the final `v0.9.2` configuration surface
through all four layers with typed values and exact provenance; inspect it in text or protocol-v1
JSON without mutation; and prove that every active deferred or dropped capability yields one
complete `unsupported-config` preflight before the fake downstream mutation seam is entered.

The change is not required to read authoritative Git objects, assemble a repository domain
snapshot, write metadata, render a harness, or alter Docket's current active deferred settings. The
approved contraction of those settings remains change 0318's independently reviewed deliverable.
