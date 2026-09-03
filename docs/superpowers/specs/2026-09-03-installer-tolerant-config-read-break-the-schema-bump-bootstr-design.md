<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0392 — Installer-tolerant config read: break the schema-bump bootstrap deadlock](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0392-installer-tolerant-config-read-break-the-schema-bump-bootstr.md)**
<!-- docket:backlink:end -->

# Installer-tolerant config read: break the schema-bump bootstrap deadlock

## Summary

Make docket's **install path** tolerate unknown configuration keys so that `docket install`,
`docket install check`, and `docket development install` always complete when the running binary
predates the on-disk `.docket.yml` schema. The tolerance is one boolean on `config.ResolveContext`,
set at exactly one call site (the CLI's shared `installOptions`), and it degrades only the
`unknown-key` diagnostic class from error to warning. Every other command keeps the strict typo
policy unchanged, and every other diagnostic class stays fatal on the install path too.

## Problem

`internal/config` treats an unknown key at any depth as `unknown-key`, which is in the resolver's
`invalidClass`, so `config.Resolve` returns `ErrInvalidConfig` ("invalid configuration"). That is
the deliberate strict typo policy.

Both install verbs resolve configuration before doing anything else: `installOptions`
(`internal/cli/install.go`) reads the global layer for the machine half, then — for `install` and
`development install` — calls `app.ResolveRepoPhase` (`internal/app/repophase.go`), which reads the
full filesystem layers for the repository half. Any resolve error becomes a `ReasonInvalidConfig`
refusal and the whole install aborts before the repository phase is even planned.

For `development install` this happens in the **parent** process — the still-installed, older
binary — before it builds the candidate from source (`root.go`'s `developmentInstallCmd` calls
`installOptions` first, then `RunDevelopmentInstall`). So when a merged change adds a schema field
(change 0374 added `build:`), the old binary rejects the config, the installer never reaches the
build step, and the only tool that would replace the binary is blocked by the field it needs to be
rebuilt to understand. Recovery today is an out-of-band `go build` and a manual swap; CLAUDE.md
carries the caveat. This recurs for every schema-extending change.

## Design

### 1. `config.ResolveContext.TolerateUnknownKeys`

Add one field to `ResolveContext`:

```go
type ResolveContext struct {
    DefaultBranch string
    // TolerateUnknownKeys reclassifies every unknown-key ERROR diagnostic as a
    // WARNING so an unrecognized setting cannot invalidate the snapshot. It exists
    // for the install path alone: an installer must never be blocked by a
    // configuration written for a newer docket than the one running. Operating
    // commands never set it.
    TolerateUnknownKeys bool
}
```

Behaviour, applied inside `resolve` after every layer is decoded and before `hasInvalid` runs:

- Every diagnostic with `Code == CodeUnknownKey` and `Severity == SeverityError` becomes
  `SeverityWarning`. Depth is irrelevant: a top-level `build:` and a nested
  `finalize.some_new_field` degrade identically. The decoder already refuses to descend into an
  unknown subtree, so the tolerated key contributes no leaves and the resolved value is the default.
- The diagnostic's `Message` is kept; its `Remedy` is set to a fixed sentence naming both causes:
  the key belongs to a newer docket than the one running (rebuild or upgrade docket), or it is a
  typo (fix it). The remedy text is a single package constant so tests and presenters share it.
- No other code changes class. `invalid-yaml`, `duplicate-key`, `invalid-type`, `invalid-value`
  stay errors and still invalidate the snapshot. The coordination fence
  (`fenced-setting-ignored`, ADR-0019) is unaffected: an unknown key has no registry spec, so the
  fence never sees it, and a fenced *known* key keeps its existing warn-and-ignore posture.
- The two pre-existing warn-and-ignore surfaces (unknown `skills.<role>`, unknown
  `board.sorting.<section>`) already emit warnings and are untouched.
- With the field false (the zero value) the resolver is byte-for-byte the current behaviour.

The reclassification lives in `internal/config/resolve.go` as one small function applied to the
collected diagnostics, not in the decoder: the decoder has no `ResolveContext`, and one site keeps
the rule auditable.

### 2. One call site: `installOptions`

`internal/cli/install.go`'s `installOptions` is the sole function that assembles install
options for all three install operations. It sets `TolerateUnknownKeys: true` on **both** of its
config reads:

- the global-only read (`config.LoadGlobalSource` → `config.Resolve`) that feeds the machine half
  and the agent table;
- the repository-phase read inside `app.ResolveRepoPhase`, which receives the context from its
  caller. `ResolveRepoPhase` gains the `config.ResolveContext` as a parameter (today it builds one
  internally from `repoConfigBranch`) so the CLI owns the decision and the app layer has no
  install-specific knowledge of *why*.

The global read is included deliberately, widening the stub's "repository layer" wording: a stale
`~/.config/docket/config.yml` strands the same rebuild, and the learnings ledger records that
outer-layer breaks are the ones no PR can fix. `install check` is included because it is the same
function and the same rule — an install operation is never blocked by configuration it does not
understand — not because it participates in the deadlock.

No other caller sets the field. In particular `diagnostic config` (`root.go`), `status`,
`repository.prepare`, and every `change.*`/`finalize.*` operation keep the strict read, so an old
binary still refuses to *operate* on an unknown field and a human can still see the strict verdict
on demand with `docket diagnostic config`.

### 3. Warnings become visible on the install path

Both install reads currently discard their diagnostics (`snapshot, _, err`). With tolerance in
place a discarded warning would make a typo, or a stale binary, invisible, so:

- `app.ResolveRepoPhase` returns the warning-severity diagnostics from its resolve alongside the
  phase. The global read's warnings are collected in `installOptions` the same way.
- `install.Options` carries them as `ConfigWarnings []config.Diagnostic`, and `app.InstallResult`
  gains `Warnings []config.Diagnostic` with JSON tag `warnings,omitempty`, populated from the
  options by `NewInstallResult`/the existing `withRepoReporting` seam. Every warning-severity
  diagnostic from the install-path reads is surfaced — not only tolerated unknown keys — because
  filtering would add code whose only effect is to hide information, and today's silent drop is
  itself a gap.
- `InstallResult.HumanText` prints one line per warning after the repository lines and before the
  actions: `warning: <source>:<line> <path> — <message> (<remedy>)`, with the provenance omitted
  when the diagnostic carries none.
- `development install` relay: the parent (old binary) tolerates silently and prints nothing, the
  candidate (new binary) prints the sole result document. That is the intended outcome: after
  hand-off the running binary is current, so the "newer docket" cause is gone; a genuine typo is
  still unknown to the candidate and warns from there. No parent-side stderr line is added — the
  one-document protocol is preserved.

### 4. Records and prose

- **ADR** (recorded by the implementer via `docket-adr`): *Install-path configuration reads
  tolerate unknown keys; the strict typo policy binds operating commands only.* Relates to
  ADR-0019 (the fence is unchanged) and ADR-0102 (the `build:` block whose merge exposed the
  deadlock). The consequence to record: a schema-extending change no longer needs an out-of-band
  rebuild, and the strict policy is now a property of *operating* the repository, not of parsing.
- **CLAUDE.md**: the second bullet under *Rebuild the binary after a merge to main* (the
  schema-bump deadlock caveat and its manual `go build` recovery) is replaced by one sentence
  stating that the installer tolerates unknown keys since this change, so the tracked
  `development.install` reinstall works directly. The repo-root files are guarded (see the
  `repoguard` commits preceding this change); the implementer follows that guard's remedy rather
  than editing around it.
- `.docket.yml.example` and the config reference need no edit: no key is added.

## Testing

Unit tests in `internal/config` (table-driven, alongside the existing resolve tests):

1. `TolerateUnknownKeys: true` + unknown top-level key → valid snapshot, one `unknown-key`
   warning carrying the shared remedy, resolved value is the default.
2. Same with a nested unknown key under a known section (`finalize.some_new_field`) → identical.
3. Same option + each of `invalid-yaml`, `duplicate-key`, `invalid-type`, `invalid-value` fixtures →
   still `ErrInvalidConfig`; the option changes nothing for those classes.
4. Option false + unknown key → `ErrInvalidConfig`, diagnostics identical to today (mutation test
   for the option: the reclassifier must be the only thing that flips the verdict).
5. A fenced known key in a machine layer with the option on → still `fenced-setting-ignored`
   warning, unchanged.

`internal/app` / `internal/cli` tests:

6. `ResolveRepoPhase` with a `.docket.yml` containing an unknown key and an explicit
   `agent_harnesses` → authorized phase, warnings returned; without tolerance → `ReasonInvalidConfig`
   refusal (existing behaviour, kept as the control).
7. `install` result over the same fixture → `result: applied`, JSON `warnings` populated, human
   text contains the `warning:` line. `install check` over an unknown-key global config → completes.
8. `development install` over the fixture through the existing devmode test seams → the parent
   reaches the build/hand-off step instead of refusing.
9. Regression: `docket status` and `docket diagnostic config` over the same unknown-key fixture
   still report invalid configuration.

A "schema-older binary" is simulated by an unknown key against the current binary; the two are
equivalent by construction, since the binary has no other way to know a key is newer than itself.

## Out of scope

- General forward-compatibility for operating commands, and schema versioning — rejected in the
  brainstorm in favour of the narrow install-only fix.
- Any change to the strict policy for `invalid-yaml`, `duplicate-key`, `invalid-type`,
  `invalid-value`, or the coordination fence, on any path.
- A parent-side warning for `development install`.
- Filtering or grouping the surfaced warnings beyond the one-line-each presenter.
