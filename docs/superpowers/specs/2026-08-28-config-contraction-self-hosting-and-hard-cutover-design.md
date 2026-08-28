<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0318 — Remaining configuration cleanup, self-hosting, and hard cutover](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0318-config-contraction-self-hosting-and-hard-cutover.md)**
<!-- docket:backlink:end -->

# Remaining configuration cleanup, self-hosting, and hard cutover

**Change:** 0318 · **Type:** refactor · **Priority:** critical · **Date:** 2026-08-28 ·
**Status:** Approved design

## Purpose and boundary

Change 0318 completes the Go migration by making the installed Go product Docket's only active
production implementation, proving the complete retained lifecycle through every supported host,
and publishing the exact accepted artifacts as `v1.0.0-rc1`.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture design](2026-08-12-go-migration-architecture-design.md) govern this change. This
spec consumes their platform, compatibility, product-boundary, storage, harness, capability, and
rollback decisions without reopening them. It designs only change 0318's independently
deliverable final-cutover scope.

In particular:

- change 0317 owns deterministic candidate packaging, the POSIX release downloader, native tuple
  smokes, and the first four-harness direct-invocation acceptance;
- change 0352 is a new hard prerequisite and owns the native repository initialization and health
  operation family required before the Bash repository-migration surface can leave production;
- change 0361 owns the macOS source-gate repair for the release-candidate workflow;
- changes 0322 and 0326 retain ownership of the source-development bootstrap/legacy adoption and
  the earlier configuration contraction respectively; and
- tag `v0.9.2` remains the immutable rollback artifact. The Go product gains no Bash fallback.

Change 0352 must settle the approved `repository init`, `repository migrate`, and `repository
check` boundary. If its focused design assigns part of that required family to a successor, that
successor becomes a prerequisite before 0318 implementation is claimed; 0318 does not absorb the
missing repository behavior merely to keep cutover moving.

## Current reality and independently deliverable result

The Go lifecycle, installer, harness assets, process supervisor, release packager, downloader, and
release-candidate workflow are landed. Docket still carries active Bash production entry points,
Bash-mechanism tests, active documentation and configuration references to the Bash facade, and a
canonical suite command rooted in `scripts/run-tests.sh`. The release-candidate workflow proves
packaging and tuple installation, but public release and the final installed-product self-host
proof are deliberately outstanding.

The independently reviewable result of 0318 is one hard-cutover release candidate in which:

1. every maintained Docket workflow invokes the PATH-resolved Go product and no production
   workflow requires `DOCKET_SCRIPTS_DIR`, `docket.sh`, Bash, Python, or Perl;
2. production Bash and tests whose subject disappeared with it are removed while retained product
   invariants remain covered by Go or intentionally retained POSIX tests;
3. active configuration, documentation, skills, agents, and local/CI gates describe the Go-only
   product;
4. Docket's active backlog contains no unresolved Bash-mechanism-only migration work, and required
   migration learnings are captured manually through Go;
5. the exact post-merge `v1.0.0-rc1` candidate passes all four native target smokes plus a
   fresh-session, complete retained-lifecycle self-host scenario through Claude, Codex, Cursor,
   and OpenCode; and
6. those exact accepted bytes—not a rebuild—are published and clean-installed, with isolated
   `v0.9.2` rollback evidence recorded before final metadata closeout.

## Chosen cutover design

### Derive the retirement set from active behavior

The implementation begins with whole-repository inventories rather than a hand-maintained list of
filenames. Each Bash or legacy-runtime occurrence is classified as one of:

1. a production mechanism that must be removed;
2. a retained POSIX distribution/bootstrap surface;
3. a test of a still-retained product invariant;
4. a test of a removed Bash implementation detail; or
5. immutable historical evidence.

The only retained shell production surfaces are the repository-root POSIX `install.sh` owned by
0322 and the POSIX release downloader packaged by 0317. They may have `/bin/sh` tests because their
shell behavior is part of the shipped contract. This exception does not preserve the Bash facade,
runner libraries, language-runtime helpers, or a second implementation of any lifecycle command.

Historical specs, results, archived changes, accepted ADRs, and frozen `v0.9.2` fixtures remain
point-in-time records. They are not rewritten to make the repository appear Go-only. Current
source, active operational documentation, generated assets, and active configuration are the
cutover surfaces.

### Make installed Go the only workflow path

Every maintained skill, native agent definition, dispatch block, workflow, setup check, and
operator command resolves `docket` from `PATH` and uses the public Go CLI/JSON contract. Production
instructions no longer construct a helper path, source shell libraries, or call the Bash facade.
Direct `git` and `gh` execution remains inside the Go adapters already approved by the
architecture.

The cutover is ownership-safe across global, per-machine, and repository surfaces. It removes or
replaces only bytes Docket can prove it owns, reports drift or unrelated user content, and never
normalizes an existing repository merely because Go opens it. Installed assets are reloaded in a
new host process before acceptance; a session that loaded pre-cutover definitions cannot supply
release evidence.

Once all callers are migrated and the coverage gates below are green, remove the Bash facade,
production runner/helper tree, and implementation-only shell tests in the same change. There is no
compatibility launcher, environment-variable bridge, hidden fallback, or grace period in the
public candidate.

### Contract active configuration and environment references

Change 0326 already placed Docket's committed policy inside the supported Go v1 capability
envelope. Change 0318 performs only the remaining mechanical cleanup discovered against current
source:

- remove active `runtime.bash` and Bash-facade examples or validation paths that no longer have a
  consumer;
- remove `DOCKET_SCRIPTS_DIR` and related helper-path requirements from maintained runtime and
  test setup;
- keep global model/effort pins and unrelated machine overrides intact; and
- cut a new sparse frozen fixture/provenance layer when authoritative defaults change, rather than
  mutating the `v0.9.2` compatibility corpus.

Any newly discovered setting that changes product capability rather than deleting an obsolete
mechanism is outside this cleanup. It is recorded as follow-up work unless it prevents a settled
cutover gate from being true.

### Replace the canonical test and gate entry point

The repository's configured `finalize.test_command`, local contributor command, and
release-candidate source gate converge on one Go-first whole-suite entry point. The entry point may
coordinate Go tests and the intentionally retained POSIX installer/downloader tests, but it cannot
depend on the removed Bash runtime or invoke the old facade.

Before a Bash test is deleted, classify the assertion it protected. If it guards a retained
product invariant—atomic replacement, marker balance, document preservation, concurrent Git
writes, external-effect recovery, process status, ownership, or installer integrity—the matching
Go/POSIX test must already fail when that premise is mutated. Tests that only specify a removed
Bash spelling, quoting workaround, pipeline shape, or helper protocol leave with their production
subject.

The whole-suite gate remains the configured authoritative command. Release acceptance fails on a
test failure, an undispositioned serial budget breach, generated-asset drift, a Bash/runtime
dependency found by the source inventory, or a guard whose mutation stays green.

### Replace active documentation without falsifying history

Active contributor, installation, release, troubleshooting, and agent-facing documentation is
rewritten around the release downloader, repository-root source bootstrap, PATH-resolved Go
binary, native repository operations from 0352, and direct four-harness dispatch. Examples use
the current JSON and exit contracts and name `v1.0.0-rc1` as the first public Go candidate.

Documentation must distinguish the two intentionally retained POSIX scripts from the removed Bash
product and state that `v0.9.2` is a separate rollback installation, not a runtime mode. Historical
records keep their original commands and references.

### Close the migration ledger and capture learnings through Go

Run a fresh active-backlog audit derived from repository-wide mechanism references and change
relationships. Each relevant active item receives one explicit disposition:

- kill a Bash-mechanism-only item as superseded by the Go migration;
- retain the product invariant in its landed Go owner, link that successor, and kill the obsolete
  mechanism proposal;
- leave deferred Go v1 work deferred and non-blocking;
- preserve independent post-Go product work; or
- leave an ambiguous item proposed for human disposition, which blocks 0318 acceptance.

No filename/keyword bulk edit substitutes for reading the item. The installed Go transaction
performs every metadata mutation and board refresh. Migration findings worth preserving are
recorded manually through the Go learning workflow because automated harvesting remains deferred.
No Bash ledger writer or direct metadata-file edit counts as self-host evidence.

## Self-host and release protocol

### Build the final candidate once from the merged source

The public bytes are produced from the exact clean `main` commit containing 0318. The bounded
cutover sequence is:

1. Before merge, pass review, the Go-first whole suite, source/asset drift checks, and all tests
   that can run without the final post-merge identity.
2. Merge the reviewed source through the native Go finalize primitives, but defer 0318's metadata
   closeout while the post-merge release gate runs. Do not run a maintenance sweep during this
   bounded interval.
3. Manually dispatch the landed 0317 candidate workflow for version `v1.0.0-rc1` and the exact
   merged commit. It packages once, emits one checksum-identified bundle, and runs the four native
   tuple smokes against that bundle.
4. Download the immutable workflow artifact and use those same bytes for self-host, rollback, and
   publication acceptance. Any source change invalidates the bundle and requires a new reviewed
   commit plus a new candidate; an operator never substitutes or silently rebuilds one target.
5. After every gate passes, create the immutable `v1.0.0-rc1` tag at that exact commit, create the
   GitHub Release, upload the accepted archives, checksum manifest, and downloader, verify the
   remote tag/assets/checksums, and publish the release.
6. Prove a clean installation through the public release URL, then complete 0318's native Go
   metadata closeout with the durable evidence references.

Tag or release creation is an explicit, human-attended irreversible boundary. The publication
driver probes GitHub before each effect, refuses a conflicting tag/release/asset, and converges on
retry only when the remote object exactly matches the accepted source identity and checksum.
Release notes identify this as the first Go release candidate, enumerate the four supported
targets and four direct harnesses, and link the rollback instructions.

### Prove complete retained lifecycle through all four harnesses

The exact accepted candidate is installed from its bundle into an isolated acceptance root. For
each of Claude, Codex, Cursor, and OpenCode, start a genuinely fresh native process and directly
invoke the installed named Docket agents. Each harness receives its own disposable clone of the
Docket repository plus isolated HOME/XDG roots and an authoritative test remote, so runs share no
mutable metadata worktree or Git index.

Each harness must drive one complete retained lifecycle with the installed Go product:

1. verify release identity, installation ownership, repository initialization/health, and the
   known starting state;
2. capture or select a fixture change, groom/link its focused spec, and move it through the native
   planning and claim boundary;
3. reconcile, build a deterministic fixture edit, record build evidence, and publish the fixture
   PR through controlled real `git`/`gh` seams;
4. finalize through rebase/retest/merge, results, archive, cleanup, and board refresh; and
5. restart or resume at designated failure boundaries to prove semantic idempotency and recovery.

The scenario is the same across hosts but the four repositories are independent. A parent running
the CLI inline, a source-built binary, `go run`, a runner shim, another harness's agent, a stale
host process, or a shared-state replay does not pass. Controlled fakes may isolate external GitHub
responses already covered by the adapter contract, but repository transitions, Git refs,
worktrees, process supervision, and installed host dispatch are real.

In addition, one direct harness uses the accepted candidate for Docket's live, remaining
0318-owned ledger/learning mutations and final closeout. This establishes that Go manages
Docket's own repository, while the four disposable clones provide repeatable complete-lifecycle
evidence without risking the live metadata branch.

### Rehearse rollback without creating a fallback

Rollback is a separately installed `v0.9.2` artifact exercised against an isolated copy of a
quiescent, pre-cutover-compatible repository. Acceptance proves that the documented operator
procedure can obtain and invoke the frozen artifact and that the original fixture remains
semantically usable. The rehearsal never replaces the accepted candidate in the live bin path,
mutates live Docket metadata, or asks the Go binary to delegate to Bash.

The public documentation is explicit that post-cutover Go writes do not carry an indefinite
reverse-compatibility promise. Rollback means restoring an operator-controlled compatible
snapshot and installing `v0.9.2`, not opening arbitrary post-cutover state with Bash.

## Failure, retry, and evidence

The cutover is fail-closed. Any failed source, tuple, harness, lifecycle, rollback, or public-install
gate stops before publication or final metadata closeout. Published external effects are never
automatically compensated. Retry resumes from authoritative Git/GitHub probes and the recorded
candidate checksum; it does not infer success from local files or elapsed time.

The 0318 results record names:

- the exact merged commit, `v1.0.0-rc1` tag, Go/toolchain identity, candidate workflow run, and
  SHA-256 for every published asset;
- the authoritative whole-suite command and outcome, budget findings, source inventory, removed
  production surfaces, retained POSIX exceptions, and coverage replacements;
- one native tuple-smoke row for Darwin/Linux on amd64/arm64;
- one fresh-session complete-lifecycle row for Claude, Codex, Cursor, and OpenCode, including host
  version/mode, installed binary identity, repository/remote isolation, child-agent proof,
  lifecycle terminal state, failure/retry evidence, and sanitized transcript location;
- the active-backlog disposition audit and manual learning records;
- isolated `v0.9.2` rollback-rehearsal evidence; and
- the remote tag/release verification plus a checksum-verified clean public install and
  `docket install check` result.

External host behavior, GitHub publication, and process-start asset loading remain human-verified
truth. Missing or ambiguous evidence is a failed gate, not a documentation follow-up.

## Alternatives considered

### Publish a stable `v1.0.0`

Rejected for 0318. This is the first public Go candidate, so the selected tag is
`v1.0.0-rc1`. Stable `v1.0.0` requires a separate promotion decision after candidate experience.

### Publish first and treat public downloads as acceptance

Rejected. Publication is the irreversible boundary. The candidate artifact supplies the exact
bytes for all pre-publication gates; the public URL is tested afterward only to verify that those
same accepted bytes were exposed correctly.

### Keep the Bash facade for rollback or one transition release

Rejected by the approved hard-replacement architecture. The rollback artifact is the independent
`v0.9.2` release, and the Go candidate contains no Bash fallback or compatibility launcher.

### Treat 0317's status-only harness evidence as final self-hosting

Rejected. That evidence correctly proves candidate installation and direct dispatch. Final
cutover additionally requires the exact merged candidate to complete the retained mutating
lifecycle and recovery boundary through every supported host.

## Explicit exclusions

Change 0318 does not:

- implement or redesign change 0352's repository operation family;
- repeat 0322's source bootstrap/adoption, 0326's capability contraction, 0317's packaging engine,
  or 0361's macOS workflow repair;
- restore deferred capabilities, add a Bash fallback, or change the existing-repository semantic
  compatibility contract;
- publish stable `v1.0.0`, Homebrew, Windows artifacts, code signing/notarization, an SBOM,
  provenance signing, uninstall, or immutable version-tree garbage collection;
- redesign Docket's Markdown/Git storage, JSON protocol, harness topology, workflow judgment split,
  or Git/`gh` adapter architecture; or
- rewrite historical specs, results, archived changes, accepted ADRs, or the frozen `v0.9.2`
  compatibility corpus.

## Acceptance boundary

Change 0318 is designed when this focused spec is linked from the change. It is implemented only
after 0317, 0352, and any prerequisite 0352 creates for the required repository operation family
are complete; the Go-only source/configuration/documentation/test cutover is green; the migration
ledger and manual learnings are complete; one exact post-merge `v1.0.0-rc1` bundle passes all four
native tuple smokes and all four fresh-session complete-lifecycle scenarios; isolated `v0.9.2`
rollback is proven; the exact accepted bytes are published and clean-installed; and Docket closes
0318 through the installed Go product.

The change stops at the verified release candidate and hard cutover. Stable release promotion and
post-candidate distribution work remain separate decisions.
