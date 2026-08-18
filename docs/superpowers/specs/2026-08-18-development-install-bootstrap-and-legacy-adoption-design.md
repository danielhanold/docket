<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0322 — Bootstrap Go development installation and adopt legacy user-level artifacts](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0322-go-installer-adopt-legacy-bash-installed-user-level-artifact.md)**
<!-- docket:backlink:end -->

# Development-install bootstrap and legacy user-level adoption

**Change:** 0322 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-18 ·
**Status:** Approved design

## Purpose and boundary

This change makes change 0311's source-linked development installation usable on both a clean
machine and an existing Docket contributor machine. It closes two connected bootstrap gaps:

1. the checkout entry point installs only the legacy Bash skill/agent surface and never creates a
   `docket` executable; and
2. the Go installer's legacy ownership proof is present as an interface but not wired to a real
   byte reproducer, so Bash-generated user-level artifacts block takeover.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md),
[architecture](2026-08-12-go-migration-architecture-design.md), and
[installer design](2026-08-13-installer-embedded-assets-and-four-harnesses-design.md) govern this
slice. This change completes their existing development-install and ownership contracts. It does
not redesign installation, relax ownership, add metadata authority, or absorb change 0316's
finalize/recovery behavior.

## Independently deliverable result

After this change, a contributor can run the checkout's `install.sh` without first possessing a Go
Docket binary:

- if a compatible `docket` is already on `PATH`, the script delegates to
  `docket development install --source <checkout>`;
- otherwise it uses the Go tool as the bootstrap loader for exactly
  `go run ./cmd/docket development install --source <checkout>`; and
- the development installer builds and atomically places the resulting executable in its normal
  bin directory, source-links the authored assets, renders selected harness material, and records
  the installation through the transaction and ownership state landed by 0311.

An existing machine whose targets exactly match known Bash-installer output is adopted without
manual deletion. A clean machine follows the same declarative plan without an adoption branch.
Unknown or modified files remain conflicts.

## Checkout entry-point contract

`install.sh` remains the human-facing source-checkout entry point, but becomes a small POSIX
bootstrapper rather than a second installer. It:

1. resolves its own source directory without depending on the caller's working directory;
2. prefers an installed `docket` only when that executable exposes the development-install
   operation, otherwise requires the Go tool and uses `go run`;
3. passes the source directory and supported explicit installation arguments without shell command
   construction;
4. returns the delegated operation's status and diagnostics unchanged; and
5. runs none of the legacy `ensure-global-config.sh`, `link-skills.sh`, `sync-agents.sh`, or
   `ensure-docket-env.sh` mutation sequence before or after delegation.

There is one installation implementation: `internal/install`. The bootstrapper neither renders
files nor decides ownership. Re-running it after source or generated-asset changes refreshes the
installed development binary and harness material through the same Go operation. The installed
binary remains the steady-state entry point; `go run` is only the no-binary bootstrap path.

## Legacy adoption contract

The production installer wires 0311's third ownership proof to a frozen, deterministic reproducer
for the user-level outputs emitted by the final Bash installer. The reproducer accepts only the
legacy inputs required to recreate those bytes and returns a candidate identity for the exact
target kind and harness.

Adoption succeeds only when all of these are true:

- the target path and kind belong to the closed legacy user-level inventory;
- marker-delimited files have valid, ordered, balanced markers;
- the complete current target bytes, or the owned block where that was the legacy contract, equal
  the reproducer's output for the applicable global inputs; and
- the planned Go replacement passes every ordinary 0311 path, containment, staging, rollback, and
  state-publication check.

On success the normal install transaction replaces the target and records Go ownership in
`state/install.json`. On mismatch, missing reconstruction inputs, malformed markers, unknown
target shapes, or a probe error, the installer returns a target-specific `ownership-conflict` and
changes nothing. There is no `--force`, filename-only adoption, hand-delete instruction, or
repository-local scan.

The initial closed inventory covers only artifacts written by the final Bash machine installer:
native user-level Docket agent definitions, Cursor's Docket dispatch rule, and the Docket-managed
dispatch blocks in supported harness instruction files. Source-linked skill directories that
already point at the canonical checkout are handled by 0311's normal link-identity proof. Project
wrappers and repository-local instructions remain outside this change.

## Migration-host transition rule

For Docket's own one-time Go migration, the source-bootstrap command is narrower than the product's
normal contributor convenience:

- a separate clean checkout of immutable tag `v0.9.2` supplies the bridge skills and helper scripts;
- the implementer explicitly preloads `skills/docket-implement-next/SKILL.md` and
  `skills/docket-convention/SKILL.md` from that checkout by absolute path and routes every
  `docket.sh` call through that checkout's `scripts/` directory;
- the current named `docket-implement-next` agent is not used for these two changes, because its
  Steps 2–7 require the unavailable Go transaction CLI;
- running the tagged `install.sh` is not sufficient to select the legacy workflow: its
  `link-skills.sh` creates missing links but intentionally skips existing links, so current
  source-linked skills may remain active; the run must report and verify its exact tagged skill and
  helper roots before claim;
- any attempted `docket change`, `docket workspace`, `docket evidence`, `docket pr`, or `docket run`
  command means the wrong workflow was loaded and aborts before mutation;
- changes 0322 and 0326 are implemented and finalized through that pinned Bash workflow, while
  their feature branches still start from current `origin/main`;
- the bootstrap is then run from a clean checkout at a reviewed `origin/main` commit containing
  both merged changes;
- the transient `go run` executable may invoke only `development install`; it must not invoke
  `context`, `change`, `workspace`, `evidence`, `pr`, `run`, `finalize`, or any other command that
  mutates shared repository or GitHub state;
- the newly installed `docket` must pass `install check` and the migration repository must pass
  `diagnostic config --for-mutation` before change 0316 is eligible to start;
- `command -v docket` must resolve to the binary recorded by that successful development install,
  not an earlier ad-hoc `go install` output; and
- the host application is restarted after installation so it reloads the generated agents and
  dispatch instructions.

This is an explicit bootstrap from reviewed product source, not permission for an implementation
branch to install or use itself. A failed adoption or verification stops the transition and leaves
0316 waiting.

## Failure and recovery

All filesystem mutation remains inside 0311's journaled install transaction. Failure before plan
completion writes nothing. Failure during apply rolls back every changed target. An interrupted
unpublished journal is recovered by the next install operation. The shell bootstrapper adds no
separate journal or partial-success state.

Executable discovery is tri-state: a compatible installed command is used, clean absence chooses
the `go run` path, and an errored/ambiguous probe fails. A missing Go tool on the no-binary path is
an actionable external failure. A development build failure publishes neither the binary nor
harness changes.

## Testing strategy

- Bootstrapper tests cover invocation outside the checkout, spaces in paths, compatible installed
  binary delegation, absent/incompatible binary fallback, missing Go, argument preservation,
  delegated exit propagation, and proof that no legacy installer primitive ran.
- Golden legacy fixtures cover every supported Bash user-level artifact across applicable harness
  and global-pin shapes. Mutating any byte, marker, path, input, or target kind makes adoption
  refuse.
- Filesystem tests start from clean, exactly legacy, partially legacy, mixed unknown, drifted, and
  interrupted homes and prove atomic replacement, rollback, unrelated-byte preservation, and final
  ownership state.
- Development-install tests prove the bootstrapped binary is placed in the configured/default bin
  directory, source links and generated material match the reviewed checkout, and `install check`
  accepts the result.
- A migration-host acceptance fixture runs the bootstrap from a reviewed commit with supported
  configuration and proves that only machine-install roots change; fake metadata remotes and GitHub
  adapters record zero transaction calls.

The whole resolved suite runs at the build gate. Live restart/reload confirmation is recorded as a
human verification item because a repository test cannot prove that a host application reloaded
its process-start configuration.

## Explicit exclusions

This change does not:

- add, bypass, or weaken the Go configuration capability fence;
- edit `.docket.yml`, `.docket.local.yml`, or global model/effort configuration;
- implement repository metadata, workspace, evidence, PR, run, finalize, archive, reclaim, stack,
  or maintenance operations;
- adopt repository-local wrappers, arbitrary Docket-looking files, or modified legacy artifacts;
- package/download releases, publish a release, support Homebrew, or remove the Bash product; or
- authorize `go run ./cmd/docket <transaction-command>` against Docket's shared metadata.

Change 0326 owns the early configuration contraction. Change 0316 owns finalize/recovery. Change
0317 owns release packaging and live four-harness acceptance. Change 0318 owns the remaining
self-hosting proof, Bash removal, documentation replacement, publication, and hard cutover.

## Acceptance criteria

1. `install.sh` reaches the same Go development-install operation through an existing compatible
   binary or a no-binary `go run` bootstrap and performs no legacy installation mutations itself.
2. An exact final-Bash user-level installation is adopted into 0311's ownership state without hand
   deletion, while every unknown, drifted, malformed, or repository-local target is preserved and
   reported as a conflict.
3. Clean and adopted installs build and publish a PATH-resolvable development binary, source-linked
   assets, native harness material, and valid `state/install.json` atomically and idempotently.
4. The Docket migration-host bridge proves it loaded the exact tagged Bash skills and helper root,
   and the later bootstrap runs only from reviewed merged source, performs machine installation
   only, and stops on dispatch/install/config/path verification failure before 0316.
5. No broad overwrite, source-built metadata authority, release packaging, configuration
   contraction, or change 0316 behavior enters this change.
