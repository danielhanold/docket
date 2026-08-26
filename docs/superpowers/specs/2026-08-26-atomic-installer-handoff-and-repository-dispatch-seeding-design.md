<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0351 — Complete change 0334: stop writing global instruction files and actually deploy the recursion guard](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0351-complete-0334-retire-global-instruction-writes-and-deploy-recursion-guard.md)**
<!-- docket:backlink:end -->

# Atomic installer handoff and repository dispatch seeding

**Change:** 0351
**Status:** Approved design
**Date:** 2026-08-26

## Context

Change 0334 made Docket's parent-facing dispatch rule compact and added a recursion guard to every
generated agent wrapper. The authored renderers and their tests are correct, but a live development
install exposed two installer defects.

First, all four Go harness planners still emit parent-facing dispatch material into user-global
locations. The next install therefore recreates a global block or rule after a user removes it,
even though the intended authority is a repository's own `CLAUDE.md`, `AGENTS.md`, or Cursor rule.

Second, `development install` builds a new binary but asks the already-running, older binary to
plan and render the installation. A renderer-only change does not change the generated asset-tree
digest. The first post-merge run can consequently install the new executable while re-emitting old
wrappers; only a second run, now executing the new renderer, converges. That is why the recursion
guard existed in source and goldens but was absent from the live wrappers.

This is distinct from change 0346. Change 0346 concerns building from a source checkout whose local
branch has not pulled the merge. Change 0351 assumes the selected checkout is current and fixes the
old-executable/new-renderer boundary after that checkout has been selected.

## Goals

- Stop installing Docket dispatch instructions into personal global instruction surfaces while
  retaining globally installed skills and agent wrappers.
- Safely retire global dispatch artifacts that a prior Docket installer provably owns.
- Make one development-install invocation render and install through the binary it just built.
- Seed and reconcile only the parent-facing repository surfaces selected explicitly by that
  repository's `agent_harnesses` declaration.
- Make machine installation, global cleanup, repository reconciliation, and ownership publication
  one preflighted, rollback-capable operation.
- Prove the recursion guard and compact routing contract from a fresh harness process.

## Non-goals

- Redesigning the compact dispatch rule or the recursion-guard wording.
- Restoring Bash-era repository-local agent-wrapper generation. Agent wrappers and skills remain
  machine-installed.
- Initializing a complete Docket repository, creating `.docket.yml`, creating metadata branches or
  worktrees, managing `.gitignore`, or committing repository files. A separate related change will
  own native `repository init` and `repository check` behavior and reuse the reconciler introduced
  here.
- Folding in change 0346's stale-source-checkout repair.
- Invoking, changing, or documenting legacy repository-migration scripting.

## Command boundary and repository selection

Both `docket install` and `docket development install` use the same repository-selection contract.
They discover the Git working tree containing the current directory. A new `--repo-dir` option
selects a repository explicitly and is resolved with the same canonical, containment-aware Git
adapter used by other repository commands.

An explicit `--repo-dir` that is absent, not a Git working tree, or otherwise invalid refuses the
operation. When `--repo-dir` is absent and the current directory is not in a Git working tree, the
machine install proceeds without a repository phase. Merely standing in a repository never grants
write authority: repository files are considered only when that repository explicitly declares
`agent_harnesses` in `.docket.yml` or `.docket.local.yml`.

The installation command's existing `--harness` flag remains an explicit scope:

- with one or more `--harness` flags, the run installs those machine harnesses and reconciles only
  their corresponding repository surfaces; records belonging to other harnesses are retained;
- without `--harness`, the selected machine harness set is the union of normal harness detection
  and the repository's explicit opt-ins, so a newly opted-in harness receives its global wrappers
  and repository surface together; and
- an unknown harness is invalid input before any mutation.

The operation reports the selected machine harnesses and repository harnesses separately so a
scoped run is visible rather than inferred from changed files.

## Repository harness configuration

`agent_harnesses` becomes a supported, typed repository-surface input for the four known harnesses:
`claude`, `codex`, `cursor`, and `opencode`. Its repository value is resolved only from the
repository and repository-local layers, using the existing replace-list precedence and retaining
provenance and explicitness.

A global `agent_harnesses` declaration is never inherited as repository write authority. Likewise,
an `agents:` model/effort table does not opt a repository into parent-surface writes. This preserves
the rule that the opt-in signal is the explicit harness-selection key, not the presence of a nearby
file or an adjacent configuration capability.

The three meaningful states are:

- key absent from both repository layers: perform no repository-surface inspection, write, or
  retirement, and leave any prior repository ownership record untouched;
- explicit non-empty list: reconcile the corresponding surfaces; and
- explicit empty list: deliberately retire every unchanged Docket-owned repository surface in the
  command's scope.

Duplicate or unknown tokens are configuration errors. A repository-local declaration replaces the
committed declaration; it does not append to it.

## Parent-facing repository surfaces

The repository planner emits parent-facing instructions only. It never emits per-repository agent
definitions or skills.

| Harness opt-in | Repository surface |
|---|---|
| `claude` | A Docket dispatch block in `CLAUDE.md`, or a safe relative `CLAUDE.md` link to a shared `AGENTS.md` when that loses no user content |
| `codex` | The shared Docket dispatch block in `AGENTS.md` |
| `opencode` | The same shared Docket dispatch block in `AGENTS.md` |
| `cursor` | `.cursor/rules/docket-dispatch.mdc` |

Codex and OpenCode are semantic co-owners of one `AGENTS.md` target. Removing one opt-in does not
retire that block while the other remains selected. Claude shares `AGENTS.md` only when the
repository already has or this plan creates the shared surface and `CLAUDE.md` is absent or already
a proven Docket-owned relative link. An existing regular `CLAUDE.md` keeps its user content and
receives its own managed block. An unowned link, an escaping link, or a replacement that would
discard user content is a conflict.

All paths are anchored to the selected working-tree root. A symlink or resolved target outside that
root is refused before the transaction begins. The operation edits the working tree but never
stages or commits those edits; the human sees and disposes of them through the repository's normal
review process.

## Repository ownership isolation

Repository ownership cannot live as an undifferentiated list in the user-global installation
record: a later install from repository B must not prune repository A, and separate worktrees of
one Git repository must not overwrite each other's surface history.

Each selected working tree therefore has a private repository installation record under its own
Git directory, at `<git-dir>/docket/install.json`. It records only Docket-owned parent-surface
targets, their harness attribution, kind, block name or link target, and content digest. It is not a
tracked working-tree file.

The record uses the same ownership rule as the machine installer: a later run may rewrite or retire
a target only when the current bytes still match what Docket recorded. A target with no record may
be adopted only when it is already byte-identical to the desired render or to a closed, frozen
legacy Docket reproducer. Marker presence by itself is not permission to overwrite changed content.

Shared surfaces carry all harness owners that currently require them. Scoped runs update the named
harnesses while carrying unrelated records forward. An absent `agent_harnesses` key does not use a
stale record as permission to touch the repository.

## Fresh-binary development-install handoff

Development installation becomes one user command with two internal actors:

```text
currently running Docket
  -> validates the source and builds a candidate in a private temporary directory
  -> launches that candidate through an internal handoff
  -> candidate plans and applies the complete installation
```

The currently running executable may validate inputs, validate source/asset consistency, and build
the candidate. It must not acquire mutation authority, recover or open an installation transaction,
render a target, install the binary, or change any machine or repository destination.

The handoff uses an explicit argument vector, never a shell command. It carries the canonical source
and binary destinations and the public command's requested scope. The candidate recognizes a
private internal continuation so it does not recursively build another candidate. This continuation
is not a supported public installation mode.

The candidate repeats all mutable-world validation: configuration, repository discovery, source
identity, asset consistency, installation lock and recovery state. If the source or repository
changed between build and handoff, it refuses before installing anything. The candidate's own bytes
become the planned binary target, and its own renderer code produces every wrapper and dispatch
surface. The child outcome and exit status are the one result returned to the user.

A release install is already executing the candidate version and enters directly at candidate-owned
planning; it has no build handoff.

## Unified preflight and transaction

The candidate constructs the complete desired operation before changing a destination. The plan
contains:

- the executable in development mode;
- the selected global skills and agent wrappers;
- retirement of proven global dispatch material;
- the selected repository parent surfaces; and
- the updated global and, when applicable, repository ownership records.

Every target is inspected before the first destination mutation. Preflight validates target shape,
path containment, symlink resolution, managed-marker order and balance, prior ownership, expected
bytes, parent-directory type and permissions, and collisions where multiple harnesses name one
surface. It collects all ownership conflicts into one refusal so the operator can remedy them in
one pass.

Any machine or repository conflict refuses the entire operation. In particular, a malformed,
escaping, unwritable, or unowned repository target prevents binary, wrapper, skill, global-cleanup,
repository, and state changes alike.

After successful preflight, one write-ahead journal captures the pre-image of every machine,
global-cleanup, repository, and ownership-record mutation. Managed-block removal is a first-class
transaction step: it removes only the bounded Docket block and preserves every byte outside it.
Both ownership records participate in the journal rather than being an unrollbackable afterthought.

A synchronous apply or publication failure rolls back before returning. An abrupt process death
may leave a journaled partial operation; the next installation invocation recovers that journal
under the exclusive installation lock before it plans new work. Recovery restores both machine and
repository state to the same side of the operation.

## Retirement of global dispatch artifacts

All four harness adapters stop planning parent-facing targets under user-global instruction roots.
Global skills and agent wrappers remain selected exactly as before.

Removing a target from the desired plan is not sufficient for managed blocks, because the
surrounding file belongs to the user. The installer adds ownership-safe retirement:

- for Claude, Codex, and OpenCode, remove only the balanced `docket:dispatch` block when its
  interior still matches the prior global installation record or the frozen legacy reproducer;
- preserve all prose, formatting, and other managed blocks outside the retired block;
- for Cursor, remove the whole Docket-specific rule file only when the complete file matches the
  prior record or frozen reproducer; and
- remove the corresponding ownership record only in the same successful transaction.

An edited managed interior, malformed marker population, changed Cursor rule, foreign target kind,
or unprovable symlink is preserved and blocks the whole install. The refusal names the exact path,
explains why Docket cannot prove ownership, and gives a manual remove-or-repair-and-rerun remedy.
There is no `--force` bypass.

Once successfully retired, later installs do not recreate the global surfaces and no stale retained
managed-block record remains in the global installation state.

## Diagnostics and observable behavior

The result distinguishes build, handoff, preflight, ownership-conflict, apply, rollback, recovery,
and state-publication failures with stable machine reasons. It does not print success for the parent
build before the candidate installation has completed.

A successful no-op means the candidate inspected the selected operation and found machine,
repository, and ownership state already converged. When no repository opt-in exists, the result
states that repository reconciliation was not authorized rather than implying that file discovery
found nothing to do.

Because harnesses load skills, agent definitions, and parent instructions at process start, command
output tells the user to start a fresh harness process after any changed wrapper or parent surface.
Clearing an existing conversation is not acceptance evidence.

## Acceptance and verification

### Fresh-render regression

- Start from an installed old test binary whose renderer omits a witness line while the source tree's
  renderer includes it. One `development install` invocation must install a binary and wrappers that
  both contain the new witness.
- The test must prove the parent executable never calls the planner or mutates an installation
  destination. Removing the candidate handoff or making the parent plan must redden the test.
- A renderer-only change with an unchanged asset-tree digest must still update affected targets on
  that first invocation.

### Global retirement matrix

For each managed-block harness, cover an unchanged prior-record block, an exact frozen legacy block,
surrounding user prose, an edited interior, dangling/out-of-order/nested markers, a foreign file kind,
and an escaping symlink. For Cursor, cover exact prior and legacy whole-file bytes, edited bytes,
foreign kinds, and symlinks. Only proven artifacts are removed; every conflict leaves the complete
machine and repository world unchanged.

### Repository matrix

- Discover from the repository root and a nested directory; honor explicit `--repo-dir`; reject an
  invalid explicit repository; and perform a machine-only install outside Git.
- Prove that absent, non-empty, and explicit-empty repository harness declarations have the three
  distinct behaviors specified above.
- Prove a global `agent_harnesses` value and an `agents:` table cannot authorize repository writes.
- Cover each harness surface, Codex/OpenCode shared ownership, Claude's safe share and regular-file
  preservation paths, and Cursor's rule.
- Cover unknown and duplicate harness tokens, scoped `--harness` runs, default detection augmented by
  repository opt-ins, path escape, malformed markers, unwritable parents, and ownership drift.
- Operate on two repositories and two worktrees of one repository, then prove that updating or
  retiring one never changes another's targets or ownership record.
- Prove repository files remain unstaged and uncommitted.

### Atomicity and recovery

Inject a failure at every journaled mutation and at each ownership-publication boundary. Each
synchronous failure must reproduce the byte-for-byte pre-install world, including file modes,
symlink text, global state, and repository state. Interrupt at every durable journal point and prove
the next run recovers before applying a new plan.

The all-or-nothing guard is mutation-tested: disabling the repository-conflict preflight must make
a test observe a partial machine change, so the intact test cannot pass vacuously.

### Harness contract

Automated adapter and golden tests cover all four harnesses and prove that repository surfaces stay
compact while native agent definitions retain the descriptions required for named dispatch. No
global parent surface and no repository-local wrapper may appear in the planned target inventory.

After one development install, fresh processes for Claude, Codex, Cursor, and OpenCode must observe
the installed recursion guard and resolve the named Docket agents through the compact repository
surface. A process started before the install is explicitly invalid evidence. Any harness whose live
vendor behavior cannot be exercised must be reported as unverified rather than inferred from
another harness.

### Documentation and full suite

Active installation and harness setup documentation must describe the Go-owned automatic repository
surface reconciliation, explicit opt-in, `--repo-dir`, scoped `--harness`, global retirement safety,
and fresh-process requirement. It must not claim that the removed Bash synchronization path runs.

Run the repository's complete configured test command. Treat budget and parallel-sensitivity output
according to the repository test policy rather than relying only on the focused installer tests.

## Alternatives considered

### Ask the user to run development install twice

Rejected. It exposes a mixed binary/wrapper state, makes correctness depend on operator memory, and
does not make the candidate renderer authoritative for the first run.

### Let the old process install the candidate, then let the candidate repair wrappers

Rejected. That is two mutating stages with separate failure windows. It cannot satisfy the approved
all-or-nothing boundary.

### Make complete repository initialization a prerequisite

Rejected for change 0351. Full repository initialization owns broader Git and configuration effects
that cannot be folded into this file transaction and would delay the urgent global-cleanup and
fresh-render fixes. The missing repository command is captured separately and reuses this change's
surface reconciler.

### Store all repository targets in the global installation state

Rejected. A run from one repository could prune another repository, and worktrees would share an
ownership namespace despite having different working-tree files.

### Treat managed markers or Docket-looking filenames as sufficient ownership

Rejected. Names and markers identify intent, not unchanged bytes. Prior state, an identical desired
render, or the frozen legacy reproducer remains the overwrite and removal proof.

### Retain global dispatch as a fallback

Rejected. It restores cross-repository personal instructions, masks missing repository opt-in, and
recreates the behavior change 0334 explicitly retired.
