<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0403 — Surface config diagnostics with file:line when a command refuses on invalid configuration](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0403-surface-config-diagnostics-with-file-line-when-a-command-ref.md)**
<!-- docket:backlink:end -->

# Surface config diagnostics with file:line on invalid-configuration refusals — design

**Change:** 0403 · **Type:** fix · **Related:** 0392 (install-path unknown-key tolerance; disjoint path, same resolver)

## Problem

When `.docket.yml` (or a machine layer) is invalid, five commands refuse with the bare text `invalid configuration` and nothing else:

| Command | Today's output |
|---|---|
| `docket repository check` | `repository check: unsupported-config: invalid configuration` (exit 2); JSON `findings: null` |
| `docket repository prepare` | `repository prepare: refused: invalid configuration`; JSON carries no finding at all |
| `docket repository init` / `migrate` | `<op>: unsupported-config: invalid configuration` |
| `docket status` | `reason: invalid-input` / `message: status: invalid input: invalid configuration` |

The resolver (`internal/config.Resolve`) already returns one `config.Diagnostic` per defect with `Code`, `Severity`, `Path` (dotted key path), `Message`, optional `Remedy`, and a `Provenance{Layer, Source, Line, Column}` whose `Line`/`Column` point at the offending key. Two call sites discard that slice and wrap only the sentinel `ErrInvalidConfig`:

- `resolveSetupConfig` in `internal/app/repository_facts.go` — the repository family (check, init, migrate, and prepare via the shared setup-facts probe). It returns `&RepoResolutionError{Reason: ReasonInvalidConfig, Err: err}`.
- `loadOperationalContext` in `internal/app/operational_context.go` — every operating command (status and the rest). It returns `fmt.Errorf("%w: %v", ErrStatusInvalidInput, err)`.

`docket diagnostic config --repo-dir <dir>` does print the diagnostics, but its human line omits the line and column that its JSON `provenance` carries, and no refusal points the user at that command.

## Goals

1. Every command that refuses on an invalid configuration names each defect's source file, key path, and line, in both human and JSON output.
2. One human rendering of a config diagnostic, shared by the refusal paths and `diagnostic config`, that includes `<file>:<line>`.
3. No change to result vocabularies, reasons, or exit codes.

## Non-goals

- Relaxing any severity or tolerating unknown keys on any path (change 0392 owns install-path tolerance).
- New diagnostic classes, or changes to the resolver's diagnostic content.
- A new `diagnostics` array on any result document. The existing `findings` arrays are reused.
- Adding a "run `docket diagnostic config`" remedy line: the diagnostics are embedded instead.

## Design

### 1. Carry the diagnostics on the error

**Repository family.** `RepoResolutionError` (`internal/app/repophase.go`) gains a field:

```go
type RepoResolutionError struct {
    Reason      string
    Err         error
    Diagnostics []config.Diagnostic // resolver diagnostics when Reason == ReasonInvalidConfig; nil otherwise
}
```

`resolveSetupConfig` populates it at both of its wrap sites. The `LoadFilesystemSources` failure site has no resolver diagnostics (it failed before resolution), so it keeps `Diagnostics` nil, and the existing behaviour is unchanged there. The `config.Resolve` failure site passes the returned `diags` slice through. The other `RepoResolutionError` constructors in `repophase.go` are untouched (they carry non-config reasons); the install path's handling is 0392's territory and is not modified.

**Operating commands.** `loadOperationalContext` replaces its `fmt.Errorf` wrap with a small typed error in `internal/app`:

```go
// errInvalidConfiguration is the operational loader's invalid-config refusal.
// It carries the resolver's diagnostics so the refusing operation can lift them
// into its findings, and it classifies as ErrStatusInvalidInput so every
// existing errors.Is caller is unchanged.
type errInvalidConfiguration struct {
    diagnostics []config.Diagnostic
    err         error
}

func (e *errInvalidConfiguration) Error() string { return "status: invalid input: " + e.err.Error() }
func (e *errInvalidConfiguration) Unwrap() error  { return e.err }
func (e *errInvalidConfiguration) Is(target error) bool { return target == ErrStatusInvalidInput }
```

`Error()` preserves today's text byte-for-byte so `message:` stays stable. Only the `config.Resolve` failure inside `loadOperationalContext` uses this type; the `operationalConfigSources` failure keeps its existing wrap.

**One accessor.**

```go
// ConfigDiagnostics returns the resolver diagnostics an invalid-configuration
// refusal carries, in resolver order, or nil when err is not such a refusal.
func ConfigDiagnostics(err error) []config.Diagnostic
```

It `errors.As`-unwraps either type. Every failure mapper below calls it rather than type-switching itself.

### 2. Lift diagnostics into each operation's findings

Each refusing operation already renders a `findings` array. One lift helper per finding shape:

```go
// configDiagnosticFindings lifts resolver diagnostics into the repository
// family's finding shape, one finding per diagnostic, in resolver order.
func configDiagnosticFindings(diags []config.Diagnostic) []reposetup.Finding
```

and a sibling producing `[]StatusFinding` for `status`. Both are thin projections of one internal mapping so the two cannot drift:

| Finding field | Source |
|---|---|
| `code` | the diagnostic's `Code` verbatim (`unknown-key`, `invalid-type`, `invalid-value`, `invalid-yaml`, `duplicate-key`, `obsolete-setting`, …) |
| `severity` | the diagnostic's `Severity` verbatim (`error` / `warning`) |
| `ref` / `path` | `<Provenance.Source>:<Provenance.Line>` when provenance is present and `Line > 0`; `<Provenance.Source>` alone when the line is 0; empty when there is no provenance |
| `message` | `<Path>: <Message>` when `Path` is non-empty, else `<Message>` |
| `remedy` | the diagnostic's `Remedy` verbatim (may be empty) |
| `repairable` | nil (config findings are never auto-repairable) |

Warnings are lifted alongside errors so the reader sees the whole resolver verdict, but the operation's result, reason, and exit are decided exactly as today, by the error, never by the lifted findings.

**The five mappers.**

- `checkGatherFailure` (`repository_check.go`): on a `RepoResolutionError`, set `Findings` to the lifted list. The human text becomes the existing header line followed by the shared finding block (one line per finding, the same `- [severity] code (ref) message | remedy` block the healthy-path `HumanText` already emits). `CheckExitCode` is unchanged (still 2: the state is still unknown).
- `repository.prepare`'s failure mapper: on a `RepoResolutionError`, set `Findings` to the lifted list. This also makes prepare's `unsupported-config` refusal carry the structured finding the Step-0 contract promises. Human text follows prepare's existing per-finding block.
- `repository.init` and `repository.migrate` failure mappers: same lift into their `RepositoryOpResult` / `RepositoryMigrateResult` findings; human text is the existing header plus the shared block.
- `statusFailure` (`status.go`): when `ConfigDiagnostics(err)` is non-nil, set `Findings` to the `StatusFinding` projection. `Reason` (`invalid-input`) and `Message` are unchanged.

Because change 0399's schema surface is derived from the live Go types and these fields already exist, no schema artefact changes.

### 3. One shared human line

`internal/app` gains:

```go
// ConfigDiagnosticLine renders one config diagnostic the way every human
// surface prints it: severity, code, key path, <file>:<line>, message, remedy.
func ConfigDiagnosticLine(d config.Diagnostic) string
```

Format, fields separated by two spaces, omitted when empty:

```
error    invalid-type  agents.claude.adr.model  .docket.yml:6  expects a string, got int "42"
warning  obsolete-setting  runtime.bash  /Users/x/.config/docket/config.yml:3  selected the Bash implementation, which docket no longer ships; it is ignored | remedy: remove runtime.bash from this file
```

The severity column is padded to the width of `warning` so lines align. `<file>:<line>` follows the ref rule from §2 (file alone when the line is 0; nothing when there is no provenance). The remedy, when present, is appended as ` | remedy: <remedy>`, the same suffix the check finding block already uses.

`ConfigInspectionResult.HumanText` (`internal/app/config.go`, the `diagnostic config` renderer) replaces its inline `diagnostics:` loop with this function. That is the only behavioural change to `diagnostic config`: its human lines gain the file and line.

The refusal paths do not call `ConfigDiagnosticLine` directly. They render the lifted findings through each operation's existing finding block, which prints `code (ref) message | remedy`, so the human output for a refusal is, for example:

```
repository check: unsupported-config (unknown)
- [error] invalid-type (.docket.yml:6) agents.claude.adr.model: expects a string, got int "42"
- [error] invalid-value (.docket.yml:7) auto_capture: is a section, not a switch: the scalar form is no longer supported | remedy: auto_capture:
  enabled: true|false
  types: all
- [error] unknown-key (.docket.yml:2) bogus_key: is not a docket configuration setting
```

The multi-line remedy is rendered verbatim, as the resolver authored it.

### 4. Files touched

- `internal/app/repophase.go` — `Diagnostics` field on `RepoResolutionError`.
- `internal/app/repository_facts.go` — populate it in `resolveSetupConfig`.
- `internal/app/operational_context.go` — `errInvalidConfiguration`; use it at the `config.Resolve` failure.
- `internal/app/config_diagnostics.go` (new) — `ConfigDiagnostics`, `ConfigDiagnosticLine`, `configDiagnosticFindings`, and the `StatusFinding` projection.
- `internal/app/repository_check.go`, `repository_prepare.go`, `repository_init.go`, `repository_migrate.go`, `status.go` — the five mappers.
- `internal/app/config.go` — `HumanText` uses `ConfigDiagnosticLine`.
- `internal/config` — untouched. If a provenance string helper is wanted it lives in `internal/app`, not the resolver.

## Testing

**Fixture.** One committed `.docket.yml` on a temp repository's default branch with three defects at known lines:

```yaml
changes_dir: docs/changes
bogus_key: 1                # line 2 — unknown-key
agents:
  claude:
    adr:
      model: 42             # line 6 — invalid-type
auto_capture: true          # line 7 — invalid-value (obsolete scalar)
```

**Integration tests** (`internal/app`, alongside `repocheck_integration_test.go`), one per command: check, prepare, init, migrate, status. Each asserts:

- the result/reason/exit are unchanged from today (`unsupported-config` and exit 2 for check; `refused` for prepare; `invalid-input` for status);
- the JSON `findings` contain exactly the three error findings with `code` ∈ {`unknown-key`, `invalid-type`, `invalid-value`}, `ref` ∈ {`.docket.yml:2`, `.docket.yml:6`, `.docket.yml:7`}, in resolver order;
- the human text contains each of the three `.docket.yml:<line>` refs.

**Unit tests.**

- `ConfigDiagnosticLine`: a table over {with remedy, without remedy, no provenance, line 0} pinning the exact string.
- `configDiagnosticFindings` and the status projection: the field mapping above, including the no-provenance and line-0 ref rules, and that warnings are carried with their severity.
- `ConfigDiagnostics`: returns the slice through both error types and nil for an unrelated error; `errors.Is(err, ErrStatusInvalidInput)` still holds for `errInvalidConfiguration`.
- `ConfigInspectionResult.HumanText`: the existing `diagnostic config` golden (if any) is updated to the new line; a new assertion checks `.docket.yml:6` appears.

**Mutation checks** (per the repo's guard rule): with the lift removed from any one mapper, that command's integration test must fail on the empty `findings`; with the `Diagnostics` field left unpopulated in `resolveSetupConfig`, all four repository-family tests must fail. Both are run once during the build and recorded in the build evidence, not kept as tests.

**Suite.** The whole suite runs at the build gate via the configured `build.test_command`.

## Open questions

None. The design forks (embed vs. point at `diagnostic config`; both resolve sites vs. the repository family only; a shared renderer including `diagnostic config`) were settled in the brainstorm on 2026-09-03.
