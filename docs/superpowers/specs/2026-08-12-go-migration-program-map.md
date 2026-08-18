<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0303 — Go migration program record and Bash-backlog disposition](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-12-0303-go-migration-program-record-and-bash-backlog-disposition.md)**
<!-- docket:backlink:end -->

# Docket Go Migration Program Map

> This is the migration sprint's durable topology and scope index. It is not a build plan and does
> not live on a feature branch. Every implementation change receives its own settled spec and
> task-level plan through the ordinary Docket workflow before Claude Code claims it.

**Goal:** Replace Docket's production Bash implementation with a self-hosting Go v1 through
fifteen independently specified core changes plus the explicit transition prerequisites needed to
install and authorize the reviewed Go runtime before the first self-hosted mutation.

**Architecture:** One Go module and one `docket` binary expose a versioned JSON CLI over a
transaction-oriented domain core. Agent skills retain judgment and prose ownership; Go owns
authoritative reads, isolated metadata transactions, persistent-document safety, Git/GitHub
mechanics, local process supervision, installation, and four direct harness integrations.

**Tech Stack:** Go, Git CLI, GitHub CLI (`gh`), YAML frontmatter plus Markdown, JSON protocol,
GitHub Releases, POSIX installer bootstrap, Claude Code as the implementation host.

**Architecture:** `docs/superpowers/specs/2026-08-12-go-migration-architecture-design.md`

## Global Constraints

- Bash baseline is the immutable tag `v0.9.2`; no new Bash product feature is implemented.
- Public cutover is hard: Go v1 ships no Bash/Go hybrid and no hidden Bash fallback.
- macOS and Linux on amd64 and arm64 are required release targets.
- Existing valid, quiescent `v0.9.2` repositories open and operate without repository migration.
- Existing document bytes remain unchanged outside fields and managed blocks an operation owns.
- Markdown and Git remain authoritative; no database or daemon is introduced.
- Git and `gh` remain external processes invoked with argument arrays, never shell command strings.
- All durable metadata mutations use isolated Docket-owned transaction worktrees.
- Claude, Codex, Cursor, and OpenCode are first-class direct hosts in Go v1.
- Claude Code executes this sprint's implementation changes directly. Plans must not depend on
  Codex-specific orchestration, a Codex runner, or cross-harness runner delegation.
- Model/effort overrides in Go v1 are shipped defaults plus global, per-machine overrides only.
- GitHub Issues/Projects mirroring is removed. GitHub pull-request behavior remains.
- Automated learning harvest/index/capacity/promotion is deferred; learning consumption and the
  explicit manual learning-record operation remain.
- Before the first Go-managed mutation, transition change 0326 is pre-authorized to disable
  Docket's active deferred settings: repository-local `auto_capture`, terminal publishing, build
  checkpoints, and results-only gate skipping, and to remove repository-local agent pins while
  preserving supported global pins.
- Transition changes 0322 and 0326 are implemented and finalized through the immutable `v0.9.2`
  Bash workflow. No from-source Go transaction command manages those changes.
- The first Go source bootstrap is
  `go run ./cmd/docket development install --source <reviewed-checkout>` after 0322 and 0326 merge.
  That transient process performs machine installation only; shared-state commands begin only
  through the installed binary after install/config verification and a host restart.
- Every child change receives its own settled spec and task-level implementation plan before claim.
- Every implementation uses TDD, runs the whole resolved suite at its build gate, and commits in
  reviewable increments.

---

## Sprint accounting

The core product sprint has exactly **fifteen implementation changes**, numbered 1–15 below and
tracked as Docket changes 0304–0318. Two later-discovered transition prerequisites—0322 and
0326—are also required before core change 13 can start. They do not take finalize/recovery behavior
from 0316 or hard-cutover behavior from 0318, but they do expand the required delivery sequence.
This program map owns that topology and scope; it is not itself a unit of delivery. The first
attempt to represent it as change 0303 was retired because a change that produces no independently
reviewable build or product artifact does not belong in the lifecycle.

The fifteen core change records plus the two transition records are the live lifecycle tracker, and
`BOARD.md` is their derived view. This map records intended topology and gates; it does not
duplicate lifecycle status.

## Iteration loop

Run one explicitly named change at a time from a clone. Do not use an unscoped "next" invocation
while Bash-era build-ready records remain on the board.

1. Choose a change below whose declared predecessors are `done`. Start with change 0304.
2. In an interactive session, run `docket-groom-next <id>`. Grooming ends after it writes and links
   that change's focused design spec; it builds no code.
3. In Claude Code, invoke the `docket-implement-next` agent with that exact id. One invocation plans,
   builds, tests, reviews, and opens the change's PR.
4. Review and merge the PR, then invoke `docket-finalize-change <id>` so the record reaches `done`.
5. Re-read the board and repeat with another dependency-ready id.

Current Bash Docket permits only one autonomous implementer per clone. Parallel work, where this
graph exposes it, therefore uses separate clones; otherwise run the ready siblings sequentially.

### One-time bridge before change 0316

The ordinary loop cannot bootstrap the transaction CLI it already requires. After 0315, use this
explicit transition instead:

1. Create a separate clean checkout of immutable tag `v0.9.2`. Launch the implementer with an
   explicit absolute-path preload of that checkout's `docket-implement-next` and
   `docket-convention` skills, and route every facade call through that checkout's `scripts/`
   directory. Do not invoke the current named implementer: its Steps 2–7 require the unavailable Go
   transaction CLI. Do not treat the tagged `install.sh` as a selector: its link installer creates
   missing links but deliberately does not repoint existing links, so it can leave the current Go
   skill active while reporting the link as skipped.
2. Use that pinned Bash workflow to implement, merge, and finalize changes 0322 and 0326. Their
   feature branches still start from current `origin/main`; the tagged checkout supplies only the
   sanctioned lifecycle runtime. Before claim, the run must report the tagged skill paths and
   tagged helper root it resolved. Any attempted `docket change`, `docket workspace`,
   `docket evidence`, `docket pr`, or `docket run` command proves the wrong workflow was loaded and
   aborts the dispatch.
3. Fast-forward the primary checkout to a reviewed `origin/main` commit containing both changes and
   apply 0326's private migration-host config edit.
4. Run `go run ./cmd/docket development install --source <checkout>`. This invocation may mutate
   only machine installation roots. It may not run Docket metadata, workspace, PR, run, evidence,
   or finalize transactions.
5. Through the newly installed PATH-resolved binary, run `docket install check` and
   `docket diagnostic config --repo-dir <checkout> --for-mutation --json`; require a clean install
   and mutation-allowed result, verify `command -v docket` names that installed binary rather than
   an earlier ad-hoc `go install` output, then restart the host to reload generated agents and
   instructions.
6. Return to the ordinary loop with `docket-implement-next 316`. Any failed bridge check is a real
   precondition failure and stops before claim.

This bridge is deliberately human-attended and one-time. It neither makes the source build a
general transaction authority nor moves self-hosting proof into 0322, 0326, or 0316.

## Dependency graph

```mermaid
flowchart TD
    C1["1 / #0304 — executable, protocol, test skeleton"]
    C2["2 / #0305 — configuration"]
    C3["3 / #0306 — document layer"]
    C4["4 / #0307 — domain snapshot and graphs"]
    C5["5 / #0308 — Git adapter"]
    C6["6 / #0309 — transaction engine"]
    C7["7 / #0310 — status and health"]
    C8["8 / #0311 — installer and harnesses"]
    C9["9 / #0312 — planning mutations"]
    C10["10 / #0313 — workspaces, PRs, evidence"]
    C11["11 / #0314 — process supervisor"]
    C12["12 / #0315 — claim to implemented"]
    C13["13 / #0316 — finalize and recovery"]
    C14["14 / #0317 — release acceptance"]
    C15["15 / #0318 — self-host and cutover"]
    T1["transition / #0322 — development bootstrap and legacy adoption"]
    T2["transition / #0326 — pre-Go config contraction"]

    C1 --> C2
    C1 --> C3
    C1 --> C5
    C1 --> C8
    C1 --> C11
    C2 --> C4
    C3 --> C4
    C2 --> C8
    C4 --> C6
    C5 --> C6
    C4 --> C7
    C5 --> C7
    C6 --> C9
    C7 --> C9
    C5 --> C10
    C6 --> C10
    C9 --> C12
    C10 --> C12
    C11 --> C12
    C12 --> C13
    C8 --> T1
    C12 --> T2
    T1 --> C13
    T2 --> C13
    C8 --> C14
    C13 --> C14
    C14 --> C15
```

An edge is a real build-readiness dependency. A mere shared topic is represented by `related`, not
`depends_on`.

## Sprint administration — outside the change lifecycle

The approved architecture and this topology are durable program documents, not feature-branch
deliverables. Migration administration happens directly on the metadata branch and never enters
`docket-implement-next`.

Before the first implementation run, the migration owner:

- keeps the fifteen proposed change records and their dependency edges aligned with this map;
- dispositions every active Bash-era change: absorb a still-relevant product invariant into a
  Go child, retain a genuinely independent product change, defer a deferred-v1 capability, or kill
  a Bash-only implementation change; and
- kills change 0285 as superseded by change 0314's native Go supervisor while retaining its exact
  process-status and real-session requirements as input evidence.

The administration gate is: no active Bash-only change is accidentally selected for implementation,
and every absorbed change names its Go successor before it is killed. Until that full audit is
complete, the explicit-id rule in the iteration loop is mandatory.

## Change 1 (Docket 0304): Go executable, JSON protocol, test/build skeleton

**Purpose:** Establish the smallest releasable Go product skeleton and the contracts every later
package consumes.

**Owns:** `go.mod`, `cmd/docket`, the application result envelope, protocol v1 encoding, CLI text vs
JSON selection, build metadata, baseline Go test command, formatting/static checks, and fixture
conventions.

**Deliverable:** `docket version` and one read-only diagnostic run as one-shot processes with stable
text and JSON results on all four target tuples.

**Depends on:** Nothing. This is the first implementation change.

## Change 2 (Docket 0305): Configuration and capability envelope

**Purpose:** Load real YAML through the retained precedence and coordination fences, and classify
each legacy setting as supported, obsolete, inert, or deferred-but-requested.

**Owns:** Built-in defaults, global/repository/repository-local loaders, typed validation, global
model/effort overrides, capability diagnostics, and pre-mutation `unsupported-config` results.

**Deliverable:** Read-only configuration inspection of `v0.9.2` fixtures plus a mutation preflight
that cannot enter a transaction when active unsupported behavior is requested.

**Depends on:** Change 1.

## Change 3 (Docket 0306): Loss-preserving document layer

**Purpose:** Parse typed semantics without surrendering the source bytes needed for compatible,
reviewable edits.

**Owns:** Frontmatter location mapping, managed-block discovery and balance validation, typed value
decoding, field/block patching, canonical new-document rendering, golden compatibility fixtures,
and fuzz targets.

**Deliverable:** Existing fixtures remain byte-identical outside an intentional patch, including
unknown fields, comments, quoting, whitespace, line endings, and authored body sections.

**Depends on:** Change 1.

## Change 4 (Docket 0307): Domain snapshot, validation, graphs, and selection

**Purpose:** Represent the entire Docket repository as immutable typed state and make lifecycle
policy independent of Git, Markdown, and harnesses.

**Owns:** Changes, ADRs, learning reads, lifecycle transitions, readiness, deterministic selection,
dependency/stack graphs, effective bases, claim/reclaim rules, repository-wide validation, and
typed policy failures.

**Deliverable:** Table-driven tests reproduce retained `v0.9.2` semantic outcomes without invoking
the filesystem or a subprocess.

**Depends on:** Changes 2 and 3.

## Change 5 (Docket 0308): Git adapter and authoritative object source

**Purpose:** Expose fresh remote bytes and Git identities without touching the user's checkout, and
encapsulate Git as a typed external-command boundary. This change does not assemble domain
snapshots; change 7 composes this source with changes 2–4.

**Owns:** Git discovery, controlled execution, fetch/ref/object reads, repository-root resolution,
both metadata modes, blob entity versions, immutable object-source interfaces, and typed Git
failures.

**Deliverable:** Tests open both repository modes from temporary real repositories and return
revisioned document bytes and blob identities while leaving the invocation worktree untouched.

**Depends on:** Change 1.

## Change 6 (Docket 0309): Isolated metadata transaction engine

**Purpose:** Make one semantic metadata operation atomic, isolated from other agents, and safely
retryable after unrelated push contention.

**Owns:** Docket-owned transaction worktrees, ownership/live-lock manifests, expected-ref leases,
entity-version checks, explicit-path commits, semantic retry, request-ID idempotency trailers,
cleanup, and recovery pruning.

**Deliverable:** Real-repository tests prove unrelated concurrent writes converge, same-entity
writes contend, response-loss retries do not duplicate records, and no shared index exists.

**Depends on:** Changes 4 and 5.

## Change 7 (Docket 0310): Read-only status and health vertical slice

**Purpose:** Deliver the first user-visible retained workflow without mutation.

**Owns:** Composition of config, document parsing, domain snapshot assembly, and the Git object
source; status context; readiness and dependency/stack presentation; health findings; human text
output; protocol JSON output; and the separation between read-only status and mutating maintenance.

**Deliverable:** Semantic comparison against frozen `v0.9.2` repositories for selection, status,
stack state, and health results.

**Depends on:** Changes 4 and 5.

## Change 8 (Docket 0311): Installer, embedded assets, and four harnesses

**Purpose:** Install one version-matched Go product for every first-class direct host while retaining
the source-linked contributor workflow.

**Owns:** Embedded assets and manifest, atomic versioned extraction, ownership-safe installation,
release vs development mode, global model/effort rendering, drift checks, and native Claude, Codex,
Cursor, and OpenCode installation plans.

**Deliverable:** Golden and filesystem tests for all four hosts plus development-source drift and
binary/asset mismatch detection. No runner delegation or per-repository routing is generated.

**Depends on:** Changes 1 and 2.

## Change 9 (Docket 0312): Planning mutations, board, and ADRs

**Purpose:** Complete the retained planning-state lifecycle through coarse metadata transactions.

**Owns:** Change creation/grooming/block/defer/kill, manual learning record/update, ADR
record/supersede/reverse, inline board rendering, artifact links, backlinks, ADR index, and canonical
new records.

**Deliverable:** Capture-through-groom and ADR workflows complete atomically under concurrent
writers; the board and v1-owned indexes cannot trail their source records.

**Depends on:** Changes 6 and 7.

## Change 10 (Docket 0313): Workspaces, GitHub PR adapter, and build evidence

**Purpose:** Encapsulate deterministic feature-branch mechanics and idempotent PR publication
without making Go an agent workflow engine.

**Owns:** Feature-worktree preparation/inspection/cleanup, effective-base branch creation, GitHub CLI
adapter, PR lookup/create/update, external-effect probing, build-evidence read/write, and authoritative
verification.

**Deliverable:** Rerunning after a lost PR response adopts the existing PR; cleanup touches only
Docket-owned workspaces; stack PR bases use the domain resolver.

**Depends on:** Changes 5 and 6.

## Change 11 (Docket 0314): Native process supervisor and local gate

**Purpose:** Replace `gate-run` and change 0285's proposed interpreter with exact native process
semantics on Darwin and Linux.

**Owns:** Session/process-group launch, durable private run directories, separate logs, exact exit vs
signal terminal records, observe/stop, identity-safe signalling, abandonment recovery, and platform
implementations.

**Deliverable:** Real-process tests distinguish `exit 143` from signal 15 and prove bounded group
termination on macOS and Linux. No Bash, Python, Perl, or global daemon is involved.

**Depends on:** Change 1.

## Change 12 (Docket 0315): Claim-to-implemented workflow

**Purpose:** Complete the essential implementation half of the agent-first lifecycle while leaving
judgment and dispatch inside Claude Code.

**Owns:** Candidate context, claim and lease refresh, reconciliation transaction, plan/results
attachment, workspace orchestration inputs, gate result recording, run verification, PR association,
and the transition to `implemented`.

**Deliverable:** A Claude-hosted Docket run can take a retained build-ready change through claim,
reconcile, plan, build/review dispatch, local gate, results, and PR without direct metadata edits.

**Depends on:** Changes 9, 10, and 11.

## Transition prerequisite (Docket 0322): Development bootstrap and legacy adoption

**Purpose:** Make change 8's source-linked Go installer reachable when no Go Docket binary is yet
installed, and let it take ownership of exact final-Bash user-level artifacts without hand deletion.

**Owns:** The thin `install.sh` delegation to installed `docket development install` or the
no-binary `go run` form of that same operation; the frozen byte reproducer for known Bash-installed
user-level agents and dispatch material; production wiring of change 8's legacy ownership seam; and
bootstrap/adoption failure tests.

**Deliverable:** From reviewed merged source, an existing contributor machine can atomically install
the permanent development binary and Go-owned source-linked harness state. Unknown, modified, or
repository-local artifacts still fail closed. The temporary bootstrap process performs installation
only and never becomes a shared metadata transaction authority.

**Depends on:** Change 8 (Docket 0311).

## Transition prerequisite (Docket 0326): Pre-Go mutation configuration contraction

**Purpose:** Put Docket's repository and migration host inside Go v1's already-approved capability
envelope before the first Go transaction, without weakening the fail-closed classifier.

**Owns:** Explicitly disabling committed build checkpoints, results-only gate skipping, and terminal
publishing; removing repository-local agent pins; disabling repository-local automatic capture; and
the four-layer mutation diagnostic proving global agent pins remain supported.

**Deliverable:** The reviewed repository plus the migration host return mutation-allowed from Go's
unchanged configuration diagnostic. The change itself is implemented and finalized through the
frozen Bash baseline, not through a temporary Go transaction binary.

**Depends on:** Change 12 (Docket 0315).

## Change 13 (Docket 0316): Finalize, recovery, reclaim, archive, and stacks

**Purpose:** Complete the terminal lifecycle and make every cross-system boundary idempotent and
resumable.

**Owns:** Local rebase/retest gate, conflict/repair report inputs, merge verification, archive,
stacked-merged and root close-out, reclaim, merged-PR maintenance sweep, halted/finalize-blocked
state, branch/worktree cleanup, and interrupted close-out recovery.

**Deliverable:** Both repository modes complete claim-to-terminal workflows; failures injected after
each irreversible external effect converge on retry without false `done` records.

**Depends on:** Change 12 and transition prerequisites 0322 and 0326.

## Change 14 (Docket 0317): Release packaging and four-harness acceptance

**Purpose:** Turn the complete retained engine into installable, checksummed release artifacts and
prove direct invocation through every supported host.

**Owns:** Darwin/Linux amd64/arm64 release builds, checksum manifest, POSIX downloader, install
smoke tests, release-upgrade tests, and live Claude/Codex/Cursor/OpenCode acceptance runs.

**Deliverable:** A clean machine can install a release without Bash/Python/Perl runtime setup and
complete at least one retained workflow directly through each harness.

**Depends on:** Changes 8 and 13.

## Change 15 (Docket 0318): Remaining configuration cleanup, self-hosting, and hard cutover

**Purpose:** Prove the installed Go product can manage Docket's complete retained lifecycle, finish
migration cleanup, and remove the retired production implementation.

**Authorization:** The human approved the capability contraction on 2026-08-12 and approved moving
the minimum mutation-enabling subset to 0326 on 2026-08-18. This change does not repeat that work;
it may finish later cleanup implied by the hard cutover without re-enabling a deferred capability.

**Owns:** Full self-hosting rehearsal and recovery proof through the installed runtime, remaining
configuration and migration-ledger cleanup, manual migration learning capture, removal of
production Bash and Bash-only tests, documentation replacement, release publication, and cutover
verification.

**Deliverable:** The Go binary manages Docket's complete retained lifecycle from all four hosts;
existing repositories require no format migration; tag `v0.9.2` is the documented rollback artifact.

**Depends on:** Change 14.

## Backlog disposition rules

The direct sprint-administration pass applies these rules to every active change, with a recorded
successor where applicable:

1. **Bash-mechanism-only:** kill as superseded by the Go migration.
2. **Retained product invariant:** absorb into the owning child above, link it as input, then kill
   the Bash implementation change as superseded.
3. **Deferred Go v1 capability:** keep deferred or explicitly defer it; do not make it a sprint
   dependency.
4. **Independent post-Go product work:** retain, but ensure it cannot outrank or block the migration
   sprint accidentally.
5. **Ambiguous:** leave proposed and require a human disposition; never bulk-kill by filename or
   keyword.

Change 0285 is already settled by rule 2: its exact exit/signal and real-session requirements move
to change 11, while its Python-runtime implementation is killed.

## Child-spec and child-plan contract

Before any implementation change is claimed:

1. Groom its stub against the current repository and completed predecessors.
2. Write a child spec scoped to that one change's independently testable deliverable.
3. Write a task-level plan using `superpowers:writing-plans`, with exact files, interfaces, failing
   tests, commands, expected failures, minimal implementations, passing commands, and commits.
4. Verify the plan does not pull a deferred capability forward or re-create a dropped mechanism.
5. Confirm all interface names agree with already-landed predecessors rather than copying
   provisional names from this program plan.
6. Let Claude Code claim and implement the change through the ordinary Docket workflow.

Plans may be authored ahead only when every consumed interface has landed or is frozen by an
approved predecessor spec. This prevents fifteen detailed plans from becoming stale while the
foundation is still changing.

## Architecture-spec coverage

| Approved architecture concern | Owning change(s) |
|---|---|
| Compatibility corpus and hard replacement | 1, 14, 15 |
| Agent-first boundary and JSON protocol | 1, 12 |
| Configuration and deferred-capability refusal | 2, 0326, 15 |
| Loss-preserving repository documents | 3 |
| Domain policy and repository-wide validation | 4 |
| Authoritative Git reads and entity versions | 5, 7 |
| Isolated metadata transactions and concurrency | 6 |
| Read-only status and explicit maintenance | 7, 13 |
| Embedded/source-linked installation and four harnesses | 8, 0322, 14 |
| Planning lifecycle, board, ADRs, and manual learnings | 9 |
| External Git/GitHub effects and owned workspaces | 10, 13 |
| Native exact-status process supervision | 11 |
| Claim-to-implemented agent workflow | 12 |
| Finalization, recovery, archive, and stacks | 13 |
| Reviewed-source bootstrap and legacy user-level adoption | 0322 |
| Release artifacts, dogfood, remaining config cleanup, and cutover | 14, 15 |
| Domain/document/transaction/adapter/harness/end-to-end tests | Each owning change; integrated by 14–15 |

## Sprint completion

The sprint is complete only when changes 1–15 plus transition prerequisites 0322 and 0326 are
`done`, the direct backlog-disposition audit has no unresolved Bash-only item, the cutover gates in
the architecture spec pass, and Go Docket has successfully self-hosted a real retained workflow.
A green build of the binary or a successful source bootstrap alone is not sprint completion.

## Other post-sprint addenda

Changes outside the fifteen core changes and the two transition prerequisites that nonetheless
touch the migrated Go surface are recorded here so this map stays an accurate index of what moves
the shipped registry and its frozen oracles. These other addenda do not expand the sprint or its
dependency graph.

- **Docket 0324 — model-pinned plan-writer agent (agent-side change).** Registers a **17th** shipped
  agent (`docket-plan-writer`) in `agents/harness-defaults.yml`. Because change 0305 made the Go
  built-in registry (`internal/config/defaults.go` `builtinAgents()`) the authoritative parity
  oracle for the shipped defaults, shipping the agent reconciles that registry and cuts a new frozen
  fixture tree. The chosen release version is **`0.9.3`**: a **sparse** `testdata/repositories/v0.9.3/`
  holding only `agents-harness-defaults.yml` (17-agent byte copy) + a tree-wide `PROVENANCE.md`;
  `sidecarPath` in `internal/config/defaults_test.go` re-points `v0.9.2` → `v0.9.3`, and
  `TestBuiltinAgentsShape` moves 16 → 17. Every other frozen input stays on `v0.9.2/`. The Bash
  rollback artifact and this sprint's baseline remain tag `v0.9.2` (Global Constraints, unchanged);
  `0.9.3` is the first post-sprint agent-registry release, and its git tag is cut only after 0324
  merges and is confirmed working.
