<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0304 — Go executable, JSON protocol, and test/build skeleton](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0304-go-executable-json-protocol-test-build-skeleton.md)**
<!-- docket:backlink:end -->

# Go executable, JSON protocol, and test/build skeleton

**Change:** 0304 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change establishes the smallest independently buildable Go Docket and the contracts every
later migration slice can depend on. It implements no repository behavior. Its two executable
operations prove the application path from Cobra argument parsing through typed results to stable
human or JSON presentation:

- `docket version` reports injected build identity.
- `docket diagnostic runtime` reports the running Go toolchain and target tuple without reading the
  repository, configuration, environment-owned Docket state, Git, or `gh`.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) remain governing constraints. This
spec only resolves change 0304's package, CLI, protocol, build, test, and fixture questions. Changes
0305–0318 retain all configuration, document, domain, Git, transaction, status, installation,
harness, workflow, process-supervision, release, and cutover behavior.

## Module and dependency baseline

- Create one module at the repository root with module path `github.com/danielhanold/docket`.
- Set the language line to Go 1.26 (`go 1.26.0`) and the development toolchain to the current patch
  used to establish this skeleton (`toolchain go1.26.5`). Patch upgrades remain ordinary dependency
  maintenance; the public compatibility promise is the Go 1.26 language line, not one patch binary.
- Pin Cobra v1.10.2 as the CLI framework. Commit `go.sum`; do not vendor dependencies.
- Do not add Viper, a dependency-injection framework, a logging framework, or generated
  `cobra-cli` scaffolding. Cobra owns command/flag parsing and human help. Docket owns application
  results, protocol encoding, presentation, and exit mapping.

The executable lives at `cmd/docket`. Most implementation remains internal:

```text
cmd/docket/          process entry point; the only os.Exit site
internal/app/        result envelope, result taxonomy, and the two read-only operations
internal/buildinfo/  injected build identity and runtime facts
internal/cli/        Cobra command tree, output-mode bootstrap, presenters, and exit mapping
```

`internal/cli` is an inward-facing adapter, not a supported Go API. Cobra callbacks translate
arguments into application calls; business behavior and protocol structs do not depend on Cobra.
Command construction accepts explicit stdin, stdout, and stderr streams so tests never replace
process-global descriptors.

## Commands

### `docket version`

Build identity has three strings: `version`, `commit`, and `build_date`. Development builds use the
literal defaults `development`, `unknown`, and `unknown`. A release build may replace them with Go
linker `-X` values; this change documents and tests that seam but does not create release archives or
a release pipeline.

Default text is one line:

```text
docket development (commit unknown, built unknown)
```

Protocol output is one compact JSON document followed by one newline:

```json
{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}
```

### `docket diagnostic runtime`

This operation reads only `runtime.Version()`, `runtime.GOOS`, and `runtime.GOARCH`. It reports
whether that tuple is one of the four approved targets: `darwin/amd64`, `darwin/arm64`,
`linux/amd64`, or `linux/arm64`. Inspection itself succeeds even on another tuple, so
`supported_target: false` is data under an `applied` result, not a claim that an unsupported host is
a released product.

Default text has stable labels and order:

```text
go_version: go1.26.5
go_os: darwin
go_arch: arm64
supported_target: true
```

The values reflect the running binary. Protocol output has the same facts as typed top-level
fields:

```json
{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}
```

The `diagnostic` parent is only a command group in this change. It does not discover repositories,
load configuration, run health checks, inspect external tools, or mutate anything; those behaviors
belong to later changes.

## Protocol-v1 result contract

Every application outcome begins with the same three typed fields, encoded in this order:

1. numeric `protocol_version`, fixed at `1`;
2. string `operation`, using a stable dotted operation name where a namespace is useful; and
3. string `result`, selected from the architecture's result taxonomy.

Change 0304 defines constants for the complete v1 taxonomy so later packages share one vocabulary:
`applied`, `no-op`, `contended`, `invalid-input`, `invalid-state`, `blocked`,
`unsupported-config`, `gate-failed`, `external-failed`, `interrupted`, and `internal-error`.
Operation-specific result structs embed the envelope and add typed top-level fields; v1 does not
introduce a generic `data` wrapper. Reserved envelope names cannot be shadowed by an operation.

JSON field names and types are protocol. Removing a field, renaming one, or changing its type
requires a later protocol version. Adding an operation-specific field is compatible within v1;
consumers must ignore unknown fields. Object key order and descriptive prose are not decision
inputs, even though the encoder emits the envelope first for readability. Consumers decide from
`protocol_version`, `operation`, `result`, and documented typed reason fields.

Successful JSON output is compact UTF-8 with exactly one trailing newline. A JSON-mode invocation
emits exactly one document on stdout for both successful and handled unsuccessful outcomes. It
never prints banners, progress, help, usage, or a second JSON value. Logs and genuinely unexpected
diagnostics remain stderr-only, but a handled JSON result does not duplicate its message on stderr.
An operation computes its complete result before presentation so a failure cannot leave partial
stdout behind.

CLI parsing failures use this stable shape:

```json
{"protocol_version":1,"operation":"cli","result":"invalid-input","reason":"invalid-arguments","message":"unknown flag: --bogus"}
```

`reason` is a stable machine value. `message` is explanatory prose and must not be parsed. The
framework's concrete error text may improve without changing protocol semantics.

Exit status is intentionally coarse: `applied` and `no-op` map to 0, `invalid-input` maps to 2, and
every other non-success result maps to 1. JSON consumers use `result`, not the exit code. A later
change may refine signal-specific process termination without changing these ordinary one-shot
outcomes.

## Cobra and output control

The root command uses a persistent `--json` Boolean flag and supports it before or after a valid
subcommand. Default Cobra completion-command injection is disabled; this change exposes only
`version`, `diagnostic runtime`, and human help. There is no root `--version` alias. The root and
subcommands reject extra positional arguments.

Cobra runs with `SilenceErrors` and `SilenceUsage` enabled. Docket does not rely on Cobra's default
error routing: callbacks return results or errors without printing, and one application-owned
presenter performs the sole protocol write. Only `cmd/docket/main.go` converts the presenter's exit
classification into `os.Exit`.

Output mode must be known even when ordinary parsing stops before Cobra reaches `--json`, as in
`docket version --bogus --json`. A deliberately narrow bootstrap therefore scans raw arguments
before Cobra executes:

- it recognizes only `--json`, `--json=true`, and `--json=false`;
- the last recognized value before `--` selects the mode;
- it stops at the first standalone `--`;
- it neither validates, removes, reorders, nor interprets any other argument; and
- Cobra still performs all command and flag validation.

This is an output-transport selection step, not a second argument parser. Its bounded grammar is
kept beside the CLI adapter and table-tested against Cobra. It guarantees JSON error output even
when an earlier malformed token prevents Cobra from binding the persistent flag.

Human help remains Cobra-rendered text on stdout with exit 0. JSON mode and help are deliberately
mutually exclusive: `--json` combined with `--help`, `-h`, or the `help` command returns one
`invalid-input` JSON document with stable reason `json-help-conflict`, writes no human help, and
exits 2. This prevents help text from corrupting the protocol stream without inventing a JSON help
schema. Human parse errors write one `docket: ...` diagnostic to stderr and leave stdout empty.

## Build and test contract

The canonical developer checks are standard Go commands plus one repository integration test:

- `gofmt` must report no unformatted Go source;
- `go vet ./...` must pass;
- `go test ./...` must pass on the host;
- `CGO_ENABLED=0 go build ./cmd/docket` must succeed for each approved GOOS/GOARCH tuple, with
  outputs directed to a temporary directory; and
- the repository's existing `scripts/run-tests.sh` gate must execute the Go checks through a real
  auto-discovered `tests/test_*.sh` producer, not a documentation-only command.

The new shell producer receives a measured entry in the existing per-file runtime-budget registry.
Its test must fail if the Go gate is removed from the whole-suite path. This change does not add a
new CI provider or duplicate `finalize.test_command`; the existing resolved whole-suite command
remains authoritative.

Go tests cover at least:

- exact development and injected-build `version` text and JSON;
- deterministic runtime diagnostic fields using injectable runtime facts rather than host-specific
  assertions;
- every result-kind spelling and the 0/1/2 exit mapping;
- one JSON document and empty stderr for success, unknown command, unknown flag before `--json`,
  unknown flag after `--json`, extra argument, and missing command;
- `--json` before and after the subcommand, `--json=false`, and the `--` boundary;
- all three JSON/help conflict forms and ordinary human help isolation;
- empty stdout plus controlled stderr for human-mode parse failures;
- no default `completion` command; and
- the four cross-build tuples without executing foreign binaries.

The output tests include a built-binary subprocess layer, not only calls to a command constructor,
so accidental writes to process-global stdout/stderr are visible. Protocol goldens are decoded to
prove there is exactly one complete JSON value and are also byte-compared where exact field names,
types, compactness, and newline placement are contractual. During implementation, mutation checks
remove the Cobra silence setting and bypass the bootstrap on the hostile argument-order case; each
must make the relevant guard fail.

## Fixture convention

- Narrow unit fixtures and output goldens live in the owning package's `testdata/` directory.
- Frozen cross-package repository fixtures live under root
  `testdata/repositories/v0.9.2/<fixture-name>/` and are treated as immutable source inputs.
- A root `testdata/README.md` records provenance, immutability, and copy-before-mutation rules.
- Expected transformed output belongs beside the owning test, not inside the frozen input tree.
- This change adds only the documentation and protocol/CLI goldens it needs. It does not invent
  configuration or document fixtures on behalf of changes 0305 and 0306.

## Alternatives considered

### Standard-library-only routing

Starting with `flag` plus a hand-built subcommand router would minimize the initial dependency and
binary size. It was rejected because later migration changes add a broad nested command surface,
persistent global flags, validation, help, and completion decisions. Replacing an early router or
growing a private framework would cost more than pinning Cobra now.

### Unmodified Cobra output behavior

Letting Cobra print parse errors and usage was rejected because it cannot uphold the one-document
stdout contract, and its output-routing semantics are broader than Docket's protocol boundary.
Silencing automatic error/usage output and retaining one presenter is an explicit adapter design,
not command-by-command cleanup.

### Require `--json` before the command

This avoids bootstrap detection but makes a global flag's behavior depend on position and cannot
return JSON for the hostile parse-order case. The narrow transport scan preserves one consistent
contract without taking parsing away from Cobra.

### JSON help

A structured command-schema/help protocol was rejected as behavior with no current consumer. The
explicit conflict result is deterministic and leaves room for a separately designed protocol if a
future harness needs command introspection.

## Out of scope

- Configuration files, precedence, capability fences, and unsupported-setting diagnostics (0305).
- YAML/Markdown parsing, loss-preserving patches, and real compatibility fixtures (0306).
- Domain snapshots, lifecycle policy, graphs, selection, or repository validation (0307).
- Git/`gh`, repository discovery, metadata transactions, status, or health checks (0308–0310).
- Installation, embedded assets, harness rendering, or development-link management (0311).
- Planning mutations, worktrees, PRs, evidence, process supervision, claim/finalize workflows, or
  recovery (0312–0316).
- Release archives, checksums, downloader behavior, on-target execution, four-harness acceptance,
  or CI release automation (0317).
- Self-hosting, Bash removal, or public cutover (0318).
- Shell completion generation, man pages, localization, a daemon, a public Go library, or a plugin
  API.

Cross-compiling the executable for four tuples is a buildability gate only. Change 0317 still owns
release packaging and execution-based acceptance on those platforms.

## Acceptance boundary

Change 0304 is complete when a clean checkout can run the resolved whole suite, build the one Go
module for all four target tuples, and demonstrate the two read-only commands with byte-stable text
and protocol-v1 JSON—including malformed CLI input with `--json` after the failing token—without
reading or mutating repository state. No behavior assigned to changes 0305–0318 is required for
that proof.
