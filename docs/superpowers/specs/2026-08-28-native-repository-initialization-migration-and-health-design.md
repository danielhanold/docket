<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0352 — Native repository initialization, migration, and health checks](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-28-0352-native-repository-initialization-and-health-check.md)**
<!-- docket:backlink:end -->

# Native repository initialization, migration, and health checks

**Change:** 0352 · **Type:** feat · **Priority:** high · **Date:** 2026-08-28 ·
**Status:** Approved design

## Purpose and boundary

Change 0352 supplies the repository-setup operation family required by the approved Go migration:

```text
docket repository init
docket repository migrate
docket repository check
```

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture design](2026-08-12-go-migration-architecture-design.md) govern this change. This
spec consumes their Git-native storage, application-operation, loss-preserving document, JSON
protocol, direct-process, and hard-cutover decisions. It designs only the independently deliverable
repository-setup boundary and does not absorb machine installation or the final cutover.

Change 0351 is a landed implementation dependency: its ownership-safe repository-surface planner
and per-worktree ownership record are reused rather than reimplemented. Change 0352 deliberately
targets one healthy steady-state topology: the orphan `docket` metadata branch and persistent
`.docket/` worktree. Change 0363 will remove the unused `main`-mode compatibility path from the
rest of the Go product. Until 0363 lands, a single-branch repository is recognized by this
operation family only as legacy migration input, never as a healthy setup destination.

The result is a complete native path from an ordinary, remotely anchored Git repository—or an
existing Bash-era Docket repository—to a checked, recoverable docket-mode repository. Change 0318
can then remove the Bash migration surface without inventing repository behavior inside the
cutover change.

## Current reality and independently deliverable result

The Go product already resolves repository configuration, discovers canonical primary and current
worktrees, reads authoritative remote objects, creates commits, updates refs with leases, manages
worktrees, and executes loss-preserving metadata transactions. Change 0351 adds the reusable
parent-surface reconciliation plan. No native command currently combines those seams into a fresh
repository postcondition, a legacy migration, or a repository-specific health report.

The Bash bootstrap creates an empty orphan remote branch and leaves a managed `.gitignore` edit for
the operator. The Bash migration copies planning metadata, prunes the live integration surface, and
pushes sequential commits, but it uses a partial YAML reader, aborts on several recoverable
half-migration states, depends on Bash, and cannot serve the Go-only release candidate.

Change 0352 is complete when all three native commands share one authoritative state classifier,
fresh initialization and legacy migration converge idempotently on the docket topology, health is
machine-readable and non-repairing, and every irreversible Git effect is recoverable from durable
remote postconditions.

## Supported repository contract

Repository setup operates only on an existing non-bare Git working repository. It does not run
`git init`, choose or create a remote, create an initial integration commit, or support a local-only
repository.

Before a mutation, the operation must prove all of the following:

- canonical primary-worktree discovery succeeds and every owned path resolves inside that root;
- the primary worktree is clean, is checked out on the resolved integration branch, and matches
  the authoritative remote integration tip;
- the configured remote exists and its default and integration branches are pushed and readable;
- the complete configuration and path set is valid before any ref, worktree, or file write;
- `.docket/` is absent or is already the correctly registered owned worktree, never a foreign
  directory, escaping link, or conflicting registration; and
- every managed target change 0351 would reconcile passes its marker, kind, containment, and
  ownership preflight.

The remote and branch names come from normal resolved configuration; `origin` and `main` are not
hard-coded. `.docket.yml` remains optional and operator-owned. `repository init` never creates or
rewrites it. Once the main-mode-removal prerequisite lands, `metadata_branch` is not a supported
steady-state key. Migration has one narrow legacy reader for removing an existing top-level
`metadata_branch` entry from the exact pinned integration-branch configuration bytes while
preserving all other bytes and settings.

An absent configuration file uses the docket topology and the existing `integration_branch: auto`
resolution. A present but invalid configuration fails closed. Global or repository-local values
cannot override coordination-fenced repository identity.

## One shared state classifier

All three commands use one pure classifier over explicit local and authoritative remote facts. It
returns exactly one repository state:

- `fresh` — no remote `docket` branch and no live planning surface on integration;
- `needs-review` — the metadata topology is initialized but required integration-worktree edits
  are not yet committed;
- `healthy` — every remote, integration, metadata, ignore, worktree, hook, and authorized surface
  postcondition holds;
- `legacy` — a single-branch live planning surface is present and is eligible for explicit
  migration;
- `partial` — a provable initialization or migration phase landed and has one safe continuation;
- `conflict` — state is readable but foreign, divergent, dirty, or multiply interpretable; or
- `unknown` — a required probe failed, so absence or ownership cannot be established.

Probe outcomes are present, absent, or unknown. An error is never collapsed into absence, and
`conflict`/`unknown` never authorize a destructive write. The classifier reasons about the desired
post-pass state, including planned removals and replacements, rather than treating current owned
bytes as permanent collisions.

## `repository init`

### Fresh guard and effect plan

Initialization is valid only from `fresh`. If the live planning surface exists on the integration
branch, the command refuses and points to `repository migrate`; it never chooses migration as a
side effect. A foreign or unprovable local/remote `docket` branch is also a refusal.

After whole-input validation, initialization:

1. creates one parentless commit over the empty Git tree with a versioned Docket operation marker;
2. publishes that commit to the configured remote as `docket` with create-only protection;
3. creates or adopts the matching local branch and attaches the persistent root-level `.docket/`
   worktree;
4. disables Git hooks for the metadata worktree;
5. uses the established managed-block planner to prepare `.gitignore` and, only when an explicit
   repository/repository-local `agent_harnesses` declaration authorizes them, parent-facing
   dispatch surfaces; and
6. writes the machine-local change-0351 ownership record for the surfaces it actually owns.

The orphan branch contains no sample changes, directories, README, board, ADR, spec, configuration,
or schema marker. Native create operations already establish their required corpus on first use;
an empty metadata corpus is healthy.

The integration-worktree changes remain unstaged and uncommitted for human review. The init result
is `applied` with `needs-review`, names every pending path, and exits successfully because the
requested automatic effects landed. `repository check` remains nonzero until those changes are
committed and authoritative. Re-running init recomputes the same plan and converges without a
second root commit, branch, worktree, marker block, or ownership record.

Initialization does not prompt. An agent may run it after an explicit setup request, display the
structured pending-path receipt, show the reviewable diff, and ask the operator whether to commit
those integration changes.

## `repository migrate`

### Explicit authorization

Migration is the only operation allowed to convert a legacy single-branch planning layout. In an
interactive terminal it prints the resolved repository, remote, exact integration revision,
metadata destination, copy set, removal set, configuration edit, and any frontmatter repair diff,
then asks for confirmation. A non-interactive run refuses unless `--yes` is supplied. Installation,
initialization, health checking, and normal metadata commands never infer migration permission.

When an operator explicitly asks an agent to migrate, the agent may pass `--yes`. If repairable
frontmatter defects are present, non-interactive execution additionally requires
`--repair-frontmatter`; a general migration authorization cannot silently broaden into record
repair.

### Copy and removal sets

Migration reads one exact authoritative integration commit and constructs the candidate metadata
tree before removing anything. The metadata seed copies whole directories so unknown files are
loss-preserved:

- the complete configured changes directory, including active and archived changes, the board,
  existing learnings, and managed change indexes;
- the complete configured ADR directory; and
- the complete specs directory.

Plans, results, source code, and unrelated documentation are never copied to `docket`.

Only after the candidate metadata tree and any approved repairs pass full repository validation may
migration remove the mutable live surface from the integration candidate:

- the active-change directory;
- the live `BOARD.md`; and
- the Docket-managed changes entry-point README.

Archived changes, accepted ADRs, existing learning records/indexes, specs, plans, results, and all
other historical material already on integration remain there. The operation does not resurrect
terminal publishing: later terminal records stay on `docket` unless they arrive on integration
through an independently retained feature-branch artifact path.

The integration candidate also removes the obsolete top-level `metadata_branch` configuration
entry, establishes the managed `.gitignore` block, and reconciles only explicitly authorized
change-0351 parent surfaces. This is the sole configuration edit migration owns; unknown settings,
comments, ordering, quoting, blank lines, and line endings remain byte-identical.

### Commit and publication sequence

Migration creates a parentless metadata seed commit whose versioned receipt names the exact source
integration revision, copy-set digest, repair-plan digest, and operation kind. It publishes
`docket` with create-only or exact-lease protection and then re-reads the remote postcondition.

It next creates one deterministic integration descendant containing the live-surface removal,
legacy-key removal, managed ignore block, authorized repository surfaces, and any repaired retained
archive bytes. Its receipt names the source integration revision and the exact metadata revision.
The integration update uses a compare-and-swap lease; hooks and signing are disabled for the
mechanical commits.

After both remote postconditions are proven, migration fast-forwards the still-clean primary
integration checkout when it still equals the expected source revision, attaches `.docket/`,
disables its hooks, and publishes the ownership record. If local state moved after preflight, the
remote migration is not rolled back; the result reports the exact local synchronization remedy and
a retry performs only safe remaining local work.

## Explicit mechanically safe frontmatter repair

Migration validates all active and archived change frontmatter before its first mutation. Findings
are divided by a closed, code-owned repair roster—not by model judgment or an open-ended
"normalization" predicate.

A repair is eligible only when the document has one balanced, uniquely located frontmatter block,
the affected key is unique, the schema admits exactly one canonical replacement, authored body and
unknown fields can remain byte-identical, and reparsing the candidate proves the intended domain
postcondition. The initial roster is limited to:

- canonical quoting of a located string scalar whose decoded text is unambiguous but whose YAML
  token shape violates the repository's scalar-safety rule;
- conversion of a known list field stored as one scalar only when that scalar itself parses as an
  exact valid sequence of the expected item type and reproduces the same items in the same order;
  and
- removal of `claimed_at` from an already-terminal archived change, for which the lifecycle model
  permits no alternative retained meaning.

The roster does not infer or alter IDs, slugs, titles' meaning, lifecycle status, priority, type,
dates, filenames, active/archive placement, artifact paths, relationships, duplicate identities,
cycles, or dangling references. An undecodable document, duplicate key, missing value, invalid
domain token, ambiguous shape, or finding outside the closed roster blocks migration.

`repository check` reports every finding with `repairable: true|false` and includes the exact
proposed patch for repairable findings but writes nothing. Interactive migration includes the
complete repair diff in its confirmation. Non-interactive migration needs both `--yes` and
`--repair-frontmatter`; without the repair flag it returns the plan and refuses before any write.

Approved repairs are applied to the in-memory candidate before publication. Active-record repairs
therefore land only in the new authoritative metadata tree because the integration copies are
removed. Archived-record repairs land byte-identically in both the metadata seed and the retained
integration copy. The complete repaired candidate must have zero error findings before either
branch changes.

This is an explicit migration operation, not a read-side normalization or a general repair API.
Ordinary reads and metadata commands continue to preserve malformed records and refuse unsafe
mutation as required by the approved loss-preserving architecture.

## `repository check`

Health checking uses the same classifier and validators as the mutating commands. It may fetch the
configured remote, updating local remote-tracking refs and the object database like native
`status`, but it never changes working-tree files, the index, local branches, configuration,
ownership records, worktree registrations, or remote refs.

A `healthy` result proves:

- the configured remote, default branch, and integration branch resolve authoritatively;
- remote `docket` has parentless-root ancestry and its current corpus parses and validates;
- the live planning surface is absent from the authoritative integration tree;
- the obsolete `metadata_branch` setting is absent;
- the committed integration tree contains the valid managed `.gitignore` block and does not track
  `.docket/` or another Docket-managed local-only path;
- the local `.docket/` worktree has the right registration, branch, remote relationship, clean
  state, synchronization, containment, and disabled hooks; and
- when `agent_harnesses` explicitly authorizes repository surfaces, their bytes and the local
  ownership record agree with change 0351's plan.

An absent `agent_harnesses` key grants no inspection or rewrite authority over coincidentally named
parent files. A committed ignore guarantee is checked from the integration commit, not merely from
the working tree. A dirty or ahead metadata worktree is an error, never an invitation to discard
local work.

The command returns one stable state plus ordered findings. Human output names the repository,
state, finding code, severity, path/ref, message, and exact remedy. `--json` exposes the same data,
including revisions and frontmatter repairability, without requiring prose parsing.

Exit behavior is:

- `0` — healthy;
- `1` — diagnosis completed and one or more actions are required; and
- `2` — usage was invalid or authoritative state could not be determined safely.

There is no `--fix`. Remedies point to explicit init/migrate commands, the pending integration
paths to review, or the conflict requiring human disposition.

## Idempotency and interruption recovery

Durable Git state is the recovery journal. No local checkpoint file determines whether a remote
effect landed.

The native operation markers make response-loss recovery precise, but legacy Bash partial states
remain adoptable when orphan ancestry and exact tree equality prove the same postcondition without
a marker. Recovery handles these boundaries:

1. **Before metadata publication:** no remote effect exists; owned temporary state is removed and a
   retry replans from fresh authority.
2. **Metadata push response lost:** re-read remote `docket`; accept only the expected orphan shape,
   exact tree, and receipt, or an exact legacy-equivalent seed. Anything else is a conflict.
3. **Metadata seeded, integration live:** verify every seed path before constructing the integration
   removal commit. Never prune on evidence from a local branch alone.
4. **Integration advanced before prune:** re-read its fresh remote tip. If the seed bytes are
   unchanged, rebuild the removal atop that tip. If planning bytes changed, first update `docket`
   under its exact owned lease, validate the complete new seed, and only then retry the integration
   removal. A foreign metadata advance refuses.
5. **Integration push response lost:** re-read the remote tree and receipt. Exact postcondition is
   success; mismatch is contention, not permission to overwrite.
6. **Remote migration complete, local attachment incomplete:** perform only the clean
   fast-forward/worktree/hook/ownership steps still provably safe.

No path rolls back a published branch, deletes a foreign ref, force-pushes, hard-resets a dirty
worktree, or compensates one remote commit by destroying another. A half migration is an explicit,
diagnosable `partial` state whose safe continuation is derived anew from current authority.

Temporary worktrees and refs use invocation-unique names outside the repository and are removed on
normal completion. Abrupt-death debris is recognized by ownership shape on the next invocation;
only exact owned transient resources are removed. A user worktree or ambiguous registration is
preserved and reported.

## Application and adapter boundary

The CLI adds a `repository` command group and maps flags/output onto one application service. The
service owns operation sequencing, state classification, result envelopes, confirmation policy,
and semantic retry. Pure planners own:

- repository-state classification;
- init and migration tree/ref effects;
- the copy/removal sets;
- health findings and remedies; and
- the closed frontmatter repair plan.

The implementation reuses the existing configuration resolver, source-preserving document layer,
repository validator, transaction concepts, `internal/reposeed` planner, and narrow `internal/gitcli`
primitives. New Git adapter methods are added only for facts or effects not already expressible,
such as parentless commit inspection/creation or exact setup receipts. Domain and application code
never assemble shell command strings, invoke Bash, or depend on CLI-formatted Git text when a typed
adapter fact can carry the result.

Mutating command results use the existing protocol-v1 envelope and a closed result/reason
vocabulary such as `applied`, `already-satisfied`, `needs-review`, `refused`, `contended`, and
`external-failed`. Receipts include the exact revisions and pending paths needed by an agent to
continue without scraping human output.

## Test architecture and hard runtime boundary

The test design must preserve the repository's established default/integration partition and its
hard per-test runtime limit. Adding this operation family is not permission to put real-remote or
failure-matrix work into the default Go corpus, raise an existing runtime budget to hide growth, or
allow one test to monopolize the full suite.

Fast default-tag tests cover pure classifiers, planners, repair eligibility, result rendering,
argument validation, and adapter-independent application sequencing with deterministic fakes.
They do not open real remotes or run the full interruption/concurrency matrix.

Every test that opens real Git repositories/remotes, creates multiple worktrees, injects process or
push failures, exercises response loss, performs concurrency, or spans a multi-phase migration goes
behind the existing integration partition:

- the file name ends in `_integration_test.go` (or `_race_integration_test.go` when race
  instrumentation is required);
- line 1 is exactly `//go:build integration` and line 2 is blank;
- tests use the `TestIntegration...` or `TestRaceIntegration...` prefix; and
- each prefix is selected by exactly one `tests/test_go_integration_*.sh` runner using the shared
  `tests/lib/go-integration-shard.sh` executor.

The integration corpus is split by behavior—not accumulated into one repository-setup mega-test.
At minimum, fresh init/check, ordinary migration/repair, interruption/response-loss, and contention
or race scenarios are separate prefixes and shard runners when their measured costs require it.
No individual Go test and no shell shard may run for 60 seconds or longer. A case approaching the
limit must be split or reduced before merge; the hard ceiling and an existing shard's budget are not
raised to accommodate it. Each new runner receives a measured row in `tests/runtime-budgets.tsv`,
and the pinned total is updated only from fresh serial measurements under the existing contract.

The integration completeness contract must prove build tags, prefixes, one-runner membership,
race direction, non-empty execution, default-corpus exclusion, and tagged vet coverage. Tests must
also prove the suite remains parallel-safe and does not touch this repository's live worktrees or
ambient Git/config state.

Required behavioral coverage includes:

- all classifier states and present/absent/unknown probe outcomes;
- fresh init, repeat init, empty orphan shape, pending review, committed health, and explicit
  surface opt-in/absence;
- exact migration copy and removal sets, legacy-key removal, no terminal-publish behavior, and
  preservation of unknown/historical bytes;
- every allowed repair shape, refusal of every adjacent ambiguous shape, preview/flag gating, and
  post-repair whole-corpus validation;
- a current Bash-shaped partial seed with and without integration pruning;
- failure injection after every irreversible effect, including lost successful responses;
- integration and metadata contention, planning-byte changes between phases, foreign branch
  refusal, and no destructive cleanup on unknown state;
- canonical-root, dirty-worktree, symlink escape, worktree registration, hooks, committed-ignore,
  and ownership-drift cases; and
- human/JSON result equivalence and the `0`/`1`/`2` exit contract.

Every guard is mutation-tested: remove the create-only protection, exact seed verification,
full-corpus validation, repair opt-in, committed-ignore probe, foreign-branch refusal, integration
build tag, runner selection, or 60-second budget constraint and the relevant test must fail. The
build gate runs the repository's complete configured suite and dispositions every budget finding;
a green package subset is not completion evidence.

## Alternatives considered

### Split migration into another change

Rejected. Init, migration, and health share the state classifier, remote topology, copy planner,
repair preflight, worktree postcondition, and recovery receipts. Splitting them would leave the
release cutover with another incomplete repository family and duplicate its hardest safety logic.

### One generic ensure-or-repair command

Rejected. A generic converger could silently interpret a legacy live surface as permission to
migrate. Three explicit commands keep read-only diagnosis, fresh mutation, and human-authorized
branch conversion distinct.

### Automatically commit init's integration edits

Rejected. Fresh metadata-branch creation is a narrow create-only Git effect. `.gitignore` and
parent-facing instruction files live beside user code and remain reviewable under the established
installer ownership model.

### Treat check as bit-for-bit locally read-only

Rejected. Authoritative health requires current remote objects. Fetching remote-tracking state is
the same bounded read behavior already used by native status and avoids a temporary-clone subsystem;
content, branches, worktrees, ownership, and remote refs remain untouched.

### Normalize all malformed frontmatter during migration

Rejected. Many findings require semantic judgment, and historical records are loss-preserving.
Only the closed, previewed, explicitly authorized repair roster may change record bytes; every
other defect blocks before mutation.

### Preserve `main` mode as a healthy setup target

Rejected by the approved grooming decision. It has no known users and doubles steady-state setup,
transaction, finalize, link, and test behavior. Migration recognizes the legacy layout only long
enough to leave it; a dedicated follow-up removes the remaining compatibility implementation.

## Explicit exclusions

Change 0352 does not:

- create a Git repository, remote, default branch, or initial integration commit;
- support local-only setup or a healthy single-branch metadata mode;
- implement the cross-product removal of existing `main`-mode compatibility;
- create or rewrite general repository configuration outside migration's one obsolete-key removal;
- seed sample records, templates, plans, results, code, or a mandatory schema version;
- provide a general frontmatter repair/normalization command or repair ADR/learning/spec content;
- install the Docket binary, skills, or agents, or replace change 0351's planner;
- invoke, restore, or modify the Bash migration/bootstrap scripts;
- restore terminal publishing or copy new terminal records onto integration;
- force-push, delete foreign refs, hard-reset dirty work, or auto-resolve ambiguous state; or
- create a plan, implement change 0318, remove Bash production code, or publish a release.

## Acceptance boundary

Change 0352 is designed when this focused spec is linked from its change record. It is implemented
when the three commands and their structured contracts are available; fresh and legacy repositories
converge on the exact docket topology; check proves the complete local/remote postcondition;
repairable frontmatter requires explicit previewed authorization; every interruption and
contention state either resumes from authoritative evidence or refuses without loss; change 0351's
surface ownership remains the single writer; and the full configured test suite passes with all
long-running work correctly integration-partitioned and no individual test or shard reaching 60
seconds.

The change stops at repository setup and health. Change 0363's main-mode removal and change 0318's
self-hosting, Bash retirement, release-candidate acceptance, and public cutover remain separate
deliverables.
