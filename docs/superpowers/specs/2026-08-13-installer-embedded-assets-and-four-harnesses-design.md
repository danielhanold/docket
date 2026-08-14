<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0311 — Installer, embedded assets, and four first-class harnesses](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-14-0311-installer-embedded-assets-and-four-harnesses.md)**
<!-- docket:backlink:end -->

# Installer, embedded assets, and four first-class harnesses

**Change:** 0311 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change makes one Go binary able to install its matching Docket assets into Claude, Codex,
Cursor, and OpenCode without a source checkout. It also preserves the contributor workflow in
which skill edits are immediately visible from a checkout. Both modes use one asset manifest, one
ownership model, one transaction engine, and four native harness renderers.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are governing constraints. This
spec resolves only change 0311. It does not reopen the hard cutover, agent-first topology, supported
harnesses, global-only model overrides, or release packaging decisions in those documents.

In particular, installation here means user-level asset extraction and native harness material.
It does not mean downloading a release, routing an agent through another harness, choosing workflow
behavior, installing repository-local policy, or implementing any repository/domain operation.

## Landed foundation and independently deliverable result

Changes 0304 and 0305 have landed the Go command/protocol skeleton, build identity, result taxonomy,
typed configuration, built-in agent defaults, global override semantics, and mutation capability
preflight. Change 0311 consumes those contracts. It does not replace their protocol or expand the
configuration capability envelope.

The independently reviewable deliverable is:

- a deterministic, generated asset bundle and manifest embedded in the Go binary;
- an ownership-safe installer that extracts immutable versioned assets and atomically updates
  user-level harness material;
- release and source-linked development modes with the same planning and validation pipeline;
- native renderers for Claude, Codex, Cursor, and OpenCode using built-in plus global agent pins;
- `docket install`, `docket install check`, and `docket development install` operations with stable
  human and protocol-v1 results; and
- golden, filesystem, drift, failure-injection, and binary/asset compatibility tests.

The installed skills and agent sources remain opaque content to this change. Change 0311 packages
and renders the canonical inventory that exists when it is implemented; it does not design or
implement the workflow semantics assigned to changes 0312–0316. Those later changes may update the
authored assets and regenerate the bundle without changing this installer contract.

## Asset contract

### One canonical inventory

The repository's maintained skill directories, references, templates, agent sources, dispatch
instructions, harness defaults, configuration schema material, and document templates remain the
authoritative authored files. Change 0311 does not introduce a second hand-maintained asset tree.

A repository generator walks an explicit allowlist of asset roots and produces:

1. a canonical manifest sorted by slash-separated relative path; and
2. generated Go data containing every manifest-controlled file byte-for-byte.

The allowlist identifies roots and roles, not individual files. This lets a new reference or
template enter the bundle without adding its pathname to a second production list. Generation fails
when an entry escapes an allowed root, has an unsafe relative path, is not a regular file or
directory, or collides after canonical path normalization. Release bundles contain no symlinks.

The generator is deterministic across supported platforms. A repository test regenerates into a
temporary directory and byte-compares the result, then proves correspondence in both directions:
every authored file under an allowed root is manifested, and every manifest entry has an authored
source. This is the drift assertion for the generated frozen copy.

### Manifest and compatibility identity

The manifest is canonical JSON and has this conceptual shape:

```go
type Manifest struct {
    FormatVersion  int
    AssetProtocol  int
    AssetSetID     string
    Entries        []Entry
}

type Entry struct {
    Path   string
    Role   string
    Mode   uint32
    Size   int64
    SHA256 string
}
```

`AssetSetID` is the digest of the canonical manifest contents excluding the field itself. File
hashes cover the exact embedded bytes. Modes are portable policy modes, not whatever permissions
the generator happened to observe. The installer validates the manifest and every payload before
planning a mutation.

`AssetProtocol` is independent of the CLI protocol and product version. It changes only when the
binary/asset contract becomes incompatible. An installed state records both the asset protocol and
asset set. Startup of an asset-dependent command refuses a missing or incompatible installation;
it never runs with wrappers or source assets built for another protocol.

The root command treats asset dependence as the default. `version`, the existing runtime and config
diagnostics, the three installation operations, and help/completion are explicitly asset-independent.
Future domain commands therefore inherit the compatibility guard without maintaining an enumerated
list of commands that need it.

## Installation model

### Roots and immutable release trees

The default Docket data root is `${XDG_DATA_HOME}/docket` when `XDG_DATA_HOME` is set and an
absolute path, otherwise `$HOME/.local/share/docket`. Production path resolution requires a real
home directory and validates that each pre-existing root is a directory. Tests inject all roots;
they never mutate the developer's home directory.

A released binary extracts its validated bundle to:

```text
<data-root>/
  versions/<asset-set-id>/assets/...
  transactions/<transaction-id>/...
  state/install.json
```

The version directory is staged underneath `versions`, verified again, made read-only to ordinary
installation writes, and renamed into place. An already complete byte-identical version is reused.
An incomplete or mismatching directory is never adopted merely because its name exists.

Harness skill entries are links to the immutable version tree. Harness-specific agent definitions
and dispatch material are generated from that tree and current global configuration. A target link
points directly to its immutable version, not through a mutable `current` link, so publishing a new
installation cannot silently change an older recorded target.

### Source-linked development mode

`docket development install --source <checkout>` uses the same manifest validation, harness
planning, ownership, and transaction paths. Its differences are limited to the settled contributor
contract:

- validate that the source root exists, is a directory, and contains the manifest-controlled roots;
- run the bundle generator's check mode before any mutation so authored/generated drift fails;
- build `./cmd/docket` with the Go tool through an argument-vector process seam, never a shell;
- install the staged development binary into an explicit `--bin-dir`, defaulting to
  `${XDG_BIN_HOME}` or `$HOME/.local/bin`;
- link harness skill directories to canonicalized source directories rather than extracted copies;
- render wrappers from source templates and the current global configuration; and
- record the canonical source root, the source asset digest, and the asset protocol.

The source asset digest is the revision identity owned by this change. Change 0311 does not invoke
Git or interpret repository history; Git-backed revision reporting remains change 0308's concern.
If build information already supplies a commit identity, it may be recorded as auxiliary evidence
but is not required to validate source assets.

Every symlink hop in a source root or destination parent is canonicalized before containment and
identity checks. Repointing a source link, editing manifest-controlled source bytes without
regeneration, or running an incompatible development binary produces drift; it is not silently
accepted.

The release downloader and archive/checksum production remain change 0317. A released
`docket install` configures assets for the already executing binary and never downloads, replaces,
or relocates that binary.

### Transaction and rollback

Installation spans independent harness directories, so no single filesystem rename can make the
entire set atomic. The common installer instead provides a journaled transaction with atomic
publication:

1. resolve configuration and run change 0305's mutation capability preflight;
2. validate the bundle/source and compute the complete declarative plan;
3. inspect every target and prove it is absent, unchanged Docket-owned state, or a conflict;
4. stage new files beside their destinations and persist rollback material plus the ordered plan in
   a private transaction directory;
5. apply deterministic target replacements with same-directory temporary files and atomic renames;
6. atomically publish `state/install.json` only after all targets match the plan; and
7. remove the completed journal and any now-unreferenced staging material.

No target is changed before the full preflight succeeds. If an apply step fails, the installer
restores every target already changed from the journal and returns `external-failed`. A process
interruption may leave a private journal; the next installation operation detects it before making
a new plan and deterministically rolls back the unpublished transaction. `install check` reports
the recovery requirement without mutating it.

The journal contains enough bytes and metadata to restore only paths the transaction changed. It
does not snapshot whole user directories. Directories are created with private or ordinary
user-readable modes appropriate to their role, and a directory is pruned only when the ownership
record says Docket created it and it is empty.

## Ownership, preservation, and drift

`state/install.json` is a canonical ownership manifest containing:

- installer format, product build identity, asset protocol, and asset set;
- release or development mode and its mode-specific identity;
- selected harnesses and the resolved global agent-setting digest; and
- for every installed target, its target path, target kind, expected link destination or content
  hash, ownership technique, and originating asset role.

An existing target is replaceable only when one of these proofs succeeds:

- it exactly matches the corresponding entry in the prior ownership manifest;
- it is a Docket-managed block with valid, ordered, balanced markers and the recorded block hash; or
- it is a known legacy user-level Docket artifact whose complete bytes can be reproduced from the
  frozen legacy renderer and the same global inputs.

A Docket-looking filename is not ownership proof. Unknown files, altered owned files, malformed
managed blocks, links with a different canonical target, and legacy artifacts that cannot be
reproduced are conflicts. They are preserved and reported with a target-specific remedy. Change
0311 adds no destructive `--force` bypass.

On upgrade, a previously owned target absent from the new plan is removed only if its current
identity still equals the prior manifest. A drifted stale target is preserved and blocks the
upgrade. Unrelated bytes surrounding a valid managed block are preserved exactly, including final
newline behavior.

Repository-local agent wrappers are deliberately outside this transaction. Change 0311 neither
searches users' repositories nor installs per-repository dispatch policy. It exposes the same pure
ownership-comparison primitive for a later explicit-repository operation to prove and remove a
shadowing legacy Docket wrapper. That later consumer must preserve unrelated project files and is
not implemented here.

## Harness planning and rendering

### Pure adapter boundary

`internal/install` owns path validation, filesystem inspection, ownership, staging, transaction,
rollback, and state publication. `internal/harness` defines a pure planning interface; four child
packages render native artifacts but never write files:

```go
type PlanInput struct {
    Assets   AssetCatalog
    Mode     InstallMode
    Roots    UserRoots
    Agents   config.AgentSettings
}

type Adapter interface {
    Name() string
    Detect(UserRoots) Detection
    Plan(PlanInput) ([]install.Target, error)
}
```

The concrete packages are `internal/harness/claude`, `codex`, `cursor`, and `opencode`. A harness
plan contains dedicated files, links, or marker-delimited blocks. The common installer is the only
package allowed to turn those declarations into filesystem mutations.

With no `--harness` flags, installation selects harnesses detected from their established user
configuration roots. Detection is read-only and never shells out. Repeated `--harness` flags select
exactly the named supported harnesses and may create an absent root. An empty auto-detection result
is `invalid-state` with reason `no-harness-detected`, not a successful no-op. Planning order is the
fixed harness-name order and is independent of directory iteration.

`agent_harnesses` from the legacy configuration is inert, as settled by change 0305. Harness
selection comes only from explicit flags or detection. The installer resolves built-in plus global
agent settings through `internal/config`; it does not supply or read repository and
repository-local layers. Any active unsupported global capability fails before planning.

### Native target matrix

The initial native user-level targets are:

| Harness | Skills | Agent definitions | Dispatch material |
| --- | --- | --- | --- |
| Claude | `~/.claude/skills/<skill>` | `~/.claude/agents/<agent>.md` | managed block in `~/.claude/CLAUDE.md` |
| Codex | `$HOME/.agents/skills/<skill>` | `~/.codex/agents/<agent>.toml` | managed block in `~/.codex/AGENTS.md` |
| Cursor | `~/.cursor/skills/<skill>` | `~/.cursor/agents/<agent>.md` | `~/.cursor/rules/docket-dispatch.mdc` |
| OpenCode | `~/.config/opencode/skills/<skill>` | `~/.config/opencode/agents/<agent>.md` | managed block in `~/.config/opencode/AGENTS.md` |

`~` in the matrix means the validated injected home, never string concatenation against an
unchecked environment value. OpenCode honors XDG configuration roots when its current native
contract does; the adapter owns that path policy and its golden fixtures.

Each renderer uses the host's native syntax and direct named-agent dispatch. No generated artifact
mentions a runner shim or routes Claude through Codex, Codex through OpenCode, or any other
cross-harness combination. Dispatch instructions are generated from the canonical agent inventory,
not a hand-maintained second list, and preserve the caller's request unchanged. They also carry the
relevant run-gate instruction for agents whose completion claim requires caller verification.

The adapter maps the resolved model and effort values into native fields:

- Claude Markdown frontmatter uses its native model and effort keys;
- Codex TOML uses `model` and `model_reasoning_effort`;
- Cursor Markdown uses the currently supported native model representation, omitting automatic
  values according to that schema; and
- OpenCode Markdown uses `model` and `reasoningEffort`.

Global model identifiers remain opaque strings. Renderers escape them for their destination syntax
and do not claim to validate vendor inventory. An `auto` value is omitted where the harness's own
default is the native meaning. The exact vendor schemas are revalidated against current official
documentation during implementation reconcile; golden fixtures then freeze the supported shape and
version. A vendor change does not justify adding a compatibility wrapper or cross-harness fallback.

The authored agent inventory may contain entries whose full workflows land in later changes. This
change renders their native definitions as opaque installed assets; it does not execute, test, or
complete those later workflows. Rendering inventory is installation behavior, not ownership of the
behavior named by the files.

## Operations and protocol results

### `docket install`

Validates the embedded release bundle, resolves the global configuration, selects harnesses, and
applies the release-mode plan. Repeating the same install returns a successful no-op after proving
all targets and state. A changed global model/effort value produces a normal ownership-safe update
to generated agent definitions.

### `docket development install`

Requires `--source`, validates and builds the checkout, then applies the source-linked plan. It may
also accept the common repeated `--harness` selection and `--bin-dir`. The built binary is staged
and ownership-checked like every other dedicated target. Failure to run or complete the Go build is
`external-failed`; a source/bundle drift or protocol incompatibility is `invalid-state`.

### `docket install check`

Is strictly read-only. It validates the current binary's embedded manifest, installed state,
versioned tree, link destinations, generated agent bytes under current global settings, managed
blocks, development source digest, and transaction cleanliness. It reports missing installation,
drift, collision, or binary/asset mismatch without repairing anything.

All three operations use the protocol-v1 envelope from change 0304. Their result data includes the
mode, selected harnesses, asset protocol, asset set, state path, whether work was applied, and an
ordered action/diagnostic list with paths relative to a named root where possible. Stable reasons
include:

- `no-harness-detected`;
- `ownership-conflict`;
- `managed-block-invalid`;
- `installation-required`;
- `installation-drift`;
- `transaction-recovery-required`;
- `asset-manifest-invalid`;
- `asset-protocol-mismatch`; and
- `source-assets-drifted`.

Bad arguments and unsafe paths are `invalid-input`; a valid but conflicting or incompatible state
is `invalid-state`; active deferred configuration is `unsupported-config`; and filesystem or Go
tool failures are `external-failed`. Human output summarizes the outcome and remediation. JSON is
the authoritative automation surface and never embeds terminal formatting.

## Package shape and dependency rules

The implementation adds these focused seams:

- `internal/assets`: manifest types, embedded generated data, validation, and test/generator
  support;
- `internal/install`: roots, target plans, ownership state, drift inspection, transactions,
  rollback, and the three operation services;
- `internal/harness` plus four child adapters: native pure rendering and detection;
- `internal/config`: at most a small source-loading helper that returns only a global `Source` for
  the existing resolver; no new key, precedence, or capability policy; and
- `internal/app` / `internal/cli`: operation composition, protocol mapping, and commands.

`assets` does not import a harness. Harness adapters may import asset and configuration value types
but not filesystem implementations. `install` may execute only the single synchronous Go build in
development mode through an injected argument-vector seam; it does not introduce the durable
process supervision assigned to change 0314. No package imports Git, GitHub, Markdown-domain,
transaction-domain, board, planning, or workflow packages from neighboring changes.

## Validation and testing

Implementation follows TDD and finishes with the repository's resolved whole-suite command. The
minimum focused evidence is:

### Generated asset tests

- regeneration is byte-for-byte deterministic;
- authored-to-manifest and manifest-to-authored correspondence both hold;
- unsafe paths, duplicate canonical paths, non-regular inputs, changed payloads, and manifest digest
  corruption fail;
- removing either a source or generated entry makes the drift guard red; and
- embedded payload bytes, modes, sizes, hashes, and asset set validate at runtime.

### Harness golden tests

- all four adapters render every canonical agent and skill target in deterministic order;
- Claude, Codex, Cursor, and OpenCode definitions match separate native golden fixtures;
- built-in pins, global model-only overrides, model-and-effort overrides, and `auto` omission render
  correctly and escape adversarial strings;
- dispatch material names the direct native agents, preserves request-passthrough language, and
  contains no runner or cross-harness delegation;
- Codex skills use `$HOME/.agents/skills`, while the other three use their distinct native roots;
  and
- adding an agent or skill to the canonical inventory changes every applicable golden plan without
  editing a renderer-side name list.

### Filesystem and ownership tests

- fake homes and XDG roots cover absent targets, existing roots, explicit selection, detection, and
  no detected harness;
- unrelated files and surrounding managed-file bytes survive installs and upgrades exactly;
- owned no-op, owned update, owned prune, unknown collision, drifted owned target, malformed marker,
  alternate symlink spelling, symlink escape, and reproducible/non-reproducible legacy takeover are
  distinct cases;
- each apply step has a failure-injection test proving rollback, and each persisted transaction
  phase has a restart test proving deterministic recovery;
- version extraction never adopts a partial tree and same-directory replacement keeps old or new
  complete bytes, never a torn file;
- two harnesses sharing an ancestor do not acquire ownership of one another's directory or file;
  and
- repository-local configuration and project wrapper paths are absent from the install plan.

### Mode and compatibility tests

- release mode uses only embedded bytes and links to the immutable version tree;
- development mode rejects a missing root, generator drift, incompatible asset protocol, and
  repointed source link before mutation;
- development mode builds through an argument vector and publishes neither binary nor links after a
  failed build;
- a binary/installed-state mismatch blocks a default asset-dependent command while version,
  diagnostics, and installation commands remain available;
- global configuration affects render output, repository layers are never loaded, and an active
  unsupported global capability blocks mutation; and
- `install check` performs no writes, including when a recovery journal is present.

Live vendor discovery, a fresh-session named-agent invocation, release archive installation, and
download/checksum validation are recorded as human verification for change 0317. Change 0311's
completion gate is the deterministic golden and filesystem evidence above; it does not claim that a
fixture can prove a vendor application loaded a process-start artifact.

## Alternatives considered

### Install agent definitions without global dispatch material

Rejected. The files would exist, but callers would not have a reliable global mapping from Docket
operations to native named agents or the caller-side run gate. Direct native dispatch is part of the
installed contract, not optional documentation.

### Generate repository-local wrappers during machine installation

Rejected. A machine installer cannot safely discover all repositories, and repository-local routing
is outside Go v1. It would also entangle this change with Git and repository setup. Later explicit
repository operations may use the ownership proof primitive to remove a proven shadow.

### Copy mutable skills independently into each harness

Rejected. Four copies drift and make upgrades harder to roll back. Release mode links each harness
to one immutable, verified version tree; development mode links each harness to the canonical
source.

### Treat Docket-looking paths as replaceable or provide `--force`

Rejected. Names do not prove ownership, and a broad overwrite switch makes preservation dependent
on operator intuition. Exact prior state, managed markers, or byte-reproducible legacy output are
the only takeover proofs in this change.

## Explicitly out of scope

Change 0311 does not implement or redesign:

- change 0305's schema, precedence, capability classification, or repository-layer behavior;
- changes 0306–0310's documents, domain transitions, Git adapter, metadata transaction engine,
  status, health, or selection;
- change 0312's new/groom/lifecycle planning workflows or any later workflow semantics;
- change 0313's repository init/migrate behavior or discovery of project-local shadow files;
- change 0314's durable process supervisor;
- change 0315's ADR workflows;
- change 0316's finalize, review, learnings, rendering, and remaining operation behavior;
- change 0317's release archives, checksums, POSIX downloader, release binary placement, or live
  release/harness acceptance; or
- change 0318's Bash retirement and migration-ledger cleanup.

It also excludes cross-harness runners, per-repository routing, repository-local model/effort
overrides, skill rebinding, Homebrew, a daemon, a database, and a public Bash-to-Go transition mode.

## Acceptance boundary

Change 0311 is designed when this spec is linked from the change. It is implemented only when one
Go binary carries a deterministic valid asset set; release and development installs produce native,
ownership-safe user-level plans for all four harnesses; global pins render correctly; interruption
and drift preserve unrelated user state; `install check` diagnoses without mutation; and incompatible
binary/assets refuse asset-dependent startup.

The change remains independently deliverable because those outcomes are exercised with opaque
current assets and native golden fixtures. No operation owned by changes 0306–0310 or 0312–0318 is
needed to demonstrate them.
