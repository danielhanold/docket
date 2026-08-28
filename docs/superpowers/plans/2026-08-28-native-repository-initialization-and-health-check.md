<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0352 — Native repository initialization, migration, and health checks](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-28-0352-native-repository-initialization-and-health-check.md)**
<!-- docket:backlink:end -->
# Native Repository Initialization, Migration, and Health Checks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build (this change's resolved build skill) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the native `docket repository init`, `docket repository migrate`, and `docket repository check` command family over one shared pure state classifier, so a fresh remotely anchored Git repository or a legacy single-branch Bash-era Docket repository converges idempotently and recoverably on the docket topology (orphan `docket` metadata branch + persistent `.docket/` worktree), with machine-readable non-repairing health.

**Architecture:** A new pure package `internal/reposetup` owns everything decidable without I/O: the three-valued probe model and seven-state classifier, the init effect plan, the migration copy/removal sets and receipts, the closed frontmatter-repair roster, the narrow `metadata_branch` config-key removal, the native managed-gitignore-block emitter, and health findings with exact remedies. `internal/gitcli` gains the few missing typed adapter facts/effects (empty-tree OID, parentless commit creation via `commit-tree`, tree composition via a temp index, root-ancestry inspection, per-worktree hook disabling). `internal/app` gains one application service per command that gathers facts through gitcli into `reposetup.Facts`, classifies once, executes the planned effects with create-only/exact-lease pushes and re-read remote postconditions, and returns protocol-v1 envelope results. `internal/cli` adds the `repository` command group. Change 0351's `internal/reposeed` planner and ownership record remain the single writer for parent-facing surfaces — init and migrate call it, never reimplement it. All real-Git/remote/failure-matrix tests live behind the established `integration` build-tag partition in five new feature shards.

**Tech Stack:** Go (existing module), Cobra CLI, `internal/{config,document,domain,repository,repository/transaction,gitcli,reposeed,install}`, Bash test shards under `scripts/run-tests.sh` + `tests/lib/go-integration-shard.sh`, `tests/runtime-budgets.tsv`.

**Spec:** `docs/superpowers/specs/2026-08-28-native-repository-initialization-migration-and-health-design.md` (on the `docket` metadata branch; synchronized copy read at plan time). The plan argues from that spec; executors read both.

## Global Constraints

- **One classifier, three-valued probes.** Every repository fact is `Present`, `Absent`, or `Unknown`; an errored probe is NEVER collapsed into absence (learning `probe-error-is-not-clean-absence`), and `conflict`/`unknown` never authorize a destructive write. Any code path that turns a probe error into a boolean is a defect.
- **Idempotency keys on the promised remote state**, never a local proxy (learning `idempotency-keying`): "nothing to do" for init/migrate means the remote `docket` branch already carries the exact expected orphan shape/tree/receipt — never "local branch exists" or "worktree is clean".
- **Decide and act on the same copy** (learning `decide-and-act-on-the-same-copy`): migration's preview, validation, and mutation all read the ONE pinned integration revision captured at preflight; the CLI confirm flow re-passes that exact revision into the authorized run and the service returns `contended` if the remote moved.
- **No forbidden Git effects, ever:** no force-push, no foreign-ref deletion, no hard-reset of dirty work, no rollback of a published branch, no compensating one remote commit by destroying another. Grep the final diff for `--force`, `push -f`, `reset --hard` outside gitcli's existing owned-ref rebase machinery before the suite gate.
- **Domain/app code never assembles shell strings or parses CLI-formatted Git text** when a typed `gitcli` fact can carry it. New Git behavior goes in `internal/gitcli` methods only.
- **Protocol v1 taxonomy is closed** (`internal/app/result.go`): new results reuse `applied`, `no-op`, `contended`, `invalid-input`, `invalid-state`, `blocked`, `unsupported-config`, `external-failed`, `internal-error`. The spec's `already-satisfied` maps to `no-op`, `refused` to `invalid-state` (or `invalid-input` for flag misuse), and `needs-review` is an operation-specific field (`"repository_state": "needs-review"` + `pending_paths`) on an `applied`/`no-op` result — additive fields are v1-compatible; do NOT mint new `Result` spellings.
- **`repository check` exit contract is `0`/`1`/`2`** — computed by `RepositoryCheckResult.CheckExitCode()`, not by `app.ExitCode`. `1` (diagnosed action required) is not a hard failure; JSON consumers read `findings`, never the exit code (learning `exit-code-encodes-a-non-failure`).
- **Committed-ignore guarantees are proven from the integration COMMIT tree**, never the working tree (learning `gitignore-guarantee-must-be-committed`).
- **Printed remedies must be valid in the exact state that produced them** (learning `printed-remedy-state-validity`): every health finding's remedy string is branched on the same facts that produced the finding, and each remedy is exercised by a test in that exact fixture state.
- **Integration-test partition is mandatory** for every test that opens real Git repositories/remotes, creates worktrees, injects failures, exercises response loss, runs concurrency, or spans a multi-phase migration: file ends `_integration_test.go` (or `_race_integration_test.go`), line 1 exactly `//go:build integration`, line 2 blank, prefixes `TestIntegration...`/`TestRaceIntegration...`, each prefix selected by exactly one `tests/test_go_integration_*.sh` runner via `tests/lib/go-integration-shard.sh`. New prefixes (all in `./internal/app` except the gitcli one): `TestIntegrationRepoSetup`, `TestIntegrationRepoMigration`, `TestIntegrationRepoRecovery`, `TestIntegrationRepoContention`, `TestRaceIntegrationRepoSetup`, and `TestIntegrationSetupTree` (in `./internal/gitcli`). None is a name-prefix of another and none extends an existing shard prefix (existing app prefixes: `TestIntegrationChange`, `TestIntegrationFinalize*`, `TestIntegrationWorkflow`, `TestRaceIntegrationAppConcurrency`; existing gitcli: `TestIntegrationProcess`, `TestIntegrationSource`, `TestIntegrationRepo`, `TestRaceIntegrationConcurrency` — so no new gitcli test may start with `TestIntegrationRepo`).
- **No individual Go test and no shard reaches 60s.** Shard target 45–50s standalone; a 51–60s reading means split, never a raised ceiling. Each new runner lands WITH its measured `tests/runtime-budgets.tsv` row and the `EXPECTED_TOTAL` reseed in `tests/test_runtime_budgets.sh` in the same task (a test file without a row reddens the suite — learning `intermediate-task-state-buildable`). Row sizing: next multiple of 5 above the worst standalone serial reading, plus 5, minimum 10; record the readings in the tsv header ledger (learning `tolerance-constant-calibrated-on-one-machine`).
- **`-count=1` on every verdict-producing `go test` and every mutation probe** (learning `cached-runner-serves-a-mutated-tree`).
- **Mutation-test every guard** the spec names (create-only protection, exact seed verification, full-corpus validation, repair opt-in, committed-ignore probe, foreign-branch refusal, build tag, runner selection, budget ceiling): back up with `cp "$f" "$f.bak"`, mutate, prove the mutation landed via `git diff -- "$f"`, run the probe with `-count=1`, restore with `mv "$f.bak" "$f"` — never `git checkout -- <file>` (learning `mutation-restore-needs-a-backup-copy`). Record each reading in the build evidence.
- **Canonical assert helper, byte for byte, in every new `tests/test_*.sh`:** `assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }`.
- **Shell house rules:** never `producer | grep -q/head` under pipefail (capture, then `grep <<<"$var"`); `grep -E -e`/`grep -qF --` for leading-dash patterns; `mktemp` always with a `"${TMPDIR:-/tmp}/name.XXXXXX"` template; `mv -f` on install/replace paths.
- **Tests never touch this repository's live worktrees or ambient Git/config state**: every integration test builds its own upstream bare repo + clone under `t.TempDir()`, sets `HOME`/`GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` isolation the way existing `internal/app` integration harnesses do (copy the established harness pattern from `internal/app/finalize_git_test.go` / `claim_workflow_git_test.go` fixtures).
- **YAML scalar hygiene:** any script or Go writer emitting free-text prose into frontmatter quotes unconditionally; flow collections are never quoted. The repair roster's conversion output writes flow sequences unquoted.
- **Cross-references in maintained source anchor on symbol names or verbatim-quoted clauses, never line numbers** (ADR-0054).
- The full-suite gate at the end of the build is whatever `finalize.test_command` resolves to; per-task verification below is the focused cycle only.

## File Structure

```
internal/reposetup/                       NEW pure package (no gitcli/app imports; may import config, document, domain, repository, reposeed, install)
  probe.go / probe_test.go                Presence, BranchFact, RootShape, WorktreeFact, Facts
  classify.go / classify_test.go          State (7 values), Classification, Classify(Facts)
  gitignore.go / gitignore_test.go        managed .gitignore block: emitter + marker validation + committed-tree probe helper
  configedit.go / configedit_test.go      byte-preserving top-level metadata_branch key removal
  repair.go / repair_test.go              closed frontmatter repair roster: findings, eligibility, patches, digest
  initplan.go / initplan_test.go          InitPlan: root commit, ref, worktree, pending review paths
  migrateplan.go / migrateplan_test.go    CopySet/RemovalSet, MigrationPlan, seed+integration tree ops, receipts
  receipt.go / receipt_test.go            versioned operation trailers (Docket-Operation etc.) encode/decode/digests
  health.go / health_test.go              Finding{Code,Severity,Ref,Message,Remedy,Repairable}, EvaluateHealth(Facts,…)
internal/gitcli/
  setuptree.go                            NEW EmptyTreeOID, TreeOID, BuildTree(+TreeOp), CommitTree, RootCommits
  hooksoff.go                             NEW DisableWorktreeHooks (port of scripts/disable-worktree-hooks.sh semantics)
  setuptree_integration_test.go           NEW TestIntegrationSetupTree… (tagged)
internal/app/
  repository_facts.go / _test.go          setupProber interface + GatherSetupFacts → reposetup.Facts (default tests with fakes)
  repository_init.go / _test.go           RunRepositoryInit + RepositoryOpResult
  repository_migrate.go / _test.go        RunRepositoryMigrate (+preview/authorize protocol)
  repository_check.go / _test.go          RunRepositoryCheck + RepositoryCheckResult + CheckExitCode
  reposetup_integration_test.go           NEW TestIntegrationRepoSetup… (init+check, tagged)
  repomigration_integration_test.go       NEW TestIntegrationRepoMigration… (tagged)
  reporecovery_integration_test.go        NEW TestIntegrationRepoRecovery… (tagged)
  repocontention_integration_test.go      NEW TestIntegrationRepoContention… (tagged)
  reposetup_race_integration_test.go      NEW TestRaceIntegrationRepoSetup… (tagged, race)
internal/cli/
  repository.go / repository_test.go      NEW `docket repository init|migrate|check` command group
  root.go                                 MOD register repositoryCmd
tests/
  test_go_integration_gitcli_setuptree.sh NEW shard runner (normal)
  test_go_integration_app_reposetup.sh    NEW shard runner (normal)
  test_go_integration_app_repomigration.sh NEW shard runner (normal)
  test_go_integration_app_reporecovery.sh NEW shard runner (normal)
  test_go_integration_app_repocontention.sh NEW shard runner (normal)
  test_go_integration_app_reposetup_race.sh NEW shard runner (race)
  runtime-budgets.tsv                     MOD six new measured rows + ledger entries
  test_runtime_budgets.sh                 MOD EXPECTED_TOTAL reseed (per its own header rule)
```

Verified at plan time: `internal/cli/root.go` registers command groups in `newRoot` (`root.AddCommand(versionCmd, statusCmd, …)`); `tests/test_go_integration_contract.sh` discovers packages and shard membership from live runner inspection, so the new runners and prefixes are covered by checks (1)–(8) automatically; the managed gitignore block's canonical bytes live in `scripts/lib/docket-gitignore-block.sh` (`emit_docket_gitignore_block`, markers `# docket:start (managed by docket — do not hand-edit)` / `# docket:end`); per-worktree hook disabling semantics live in `scripts/disable-worktree-hooks.sh` (enable `extensions.worktreeConfig`, then `--worktree core.hooksPath` → empty absolute dir); `internal/app/status_git.go` holds the product's single remote constant `originRemote`.

Resolution decisions locked here (spec-conformant, discovered at plan time):

- **Remote name:** there is no remote config key. The new services take the remote from one seam — `setupRemote() gitcli.RemoteName` in `repository_facts.go` returning the existing `originRemote` constant — so no new literal `"origin"` appears anywhere in new code. Minting a config key is out of scope ("create or rewrite general repository configuration").
- **Metadata branch name:** `config.Effective.MetadataBranch.Value` (default `docket`). **Integration branch:** `config.Effective.IntegrationBranch.Value` (auto already resolved).
- **Copy-set directories:** the configured changes dir (`cfg.ChangesDir.Value`, whole tree including `active/`, `archive/`, `learnings/`, `BOARD.md`, indexes — copy the whole prefix so unknown files are loss-preserved), the configured ADR dir (`cfg.ADRsDir.Value`), and the specs dir. Specs have no config key; use the convention constant `docs/superpowers/specs` declared once as `reposetup.SpecsDir`.
- **Removal set (integration candidate):** `<changes>/active/`, `<changes>/BOARD.md`, `<changes>/README.md` (the Docket-managed entry-point README), plus the `metadata_branch` key edit in `.docket.yml` (only when the key is present) and the managed `.gitignore` block establishment.

---

### Task 1: `internal/reposetup` — three-valued probes and the seven-state classifier

**Files:**
- Create: `internal/reposetup/probe.go`, `internal/reposetup/probe_test.go`
- Create: `internal/reposetup/classify.go`, `internal/reposetup/classify_test.go`

**Interfaces:**
- Produces (everything later tasks consume):

```go
package reposetup

type Presence int
const (
	PresenceUnknown Presence = iota // zero value is the SAFE value
	PresencePresent
	PresenceAbsent
)

type RootShape int
const (
	RootUnknown RootShape = iota
	RootParentless        // single parentless root, expected receipt or exact legacy-equivalent tree
	RootForeign           // readable but not provably docket's (no receipt, tree mismatch, >1 root)
)

type BranchFact struct {
	Presence Presence
	Tip      string // object id when Present, else ""
}

type WorktreeFact struct {
	Presence     Presence // .docket/ path state: absent, or present-and-probed
	Registered   Presence // registered as a linked worktree of THIS repo on the metadata branch
	Foreign      bool     // present but a foreign dir / escaping link / conflicting registration
	Clean        Presence
	Synchronized Presence // local tip == remote metadata tip
	HooksOff     Presence
}

// Facts is the complete classifier input. Every field defaults to the safe
// Unknown/zero value; gatherers must set what they proved, and ONLY what they
// proved.
type Facts struct {
	RemoteConfigured     Presence
	RemoteDefaultBranch  BranchFact
	RemoteIntegration    BranchFact
	RemoteMetadata       BranchFact // the remote `docket` branch
	MetadataRoot         RootShape  // meaningful only when RemoteMetadata is Present
	LocalMetadata        BranchFact
	LiveSurface          Presence // active dir or BOARD.md in the AUTHORITATIVE integration tree
	LegacyConfigKey      Presence // top-level metadata_branch key in the pinned .docket.yml bytes
	CommittedIgnoreBlock Presence // managed block valid in the integration COMMIT tree
	DocketWorktree       WorktreeFact
	PrimaryClean         Presence
	PrimaryOnIntegration Presence
	PrimaryAtRemoteTip   Presence
	PendingReviewPaths   []string // init-planned integration-worktree paths not yet committed
	PartialPhase         PartialPhase
	SurfacesAuthorized   bool     // agent_harnesses explicitly declared at repo/repo-local layer
	SurfacesAgree        Presence // 0351 plan vs bytes+ownership record; only meaningful when authorized
}

type PartialPhase int
const (
	PartialNone PartialPhase = iota
	PartialMetadataSeeded          // remote docket proven, integration live surface still present
	PartialIntegrationPruned       // both remote postconditions proven, local attach incomplete
)

type State string
const (
	StateFresh       State = "fresh"
	StateNeedsReview State = "needs-review"
	StateHealthy     State = "healthy"
	StateLegacy      State = "legacy"
	StatePartial     State = "partial"
	StateConflict    State = "conflict"
	StateUnknown     State = "unknown"
)

type Classification struct {
	State   State
	Reasons []string // one stable machine-readable reason token per contributing fact
}

func Classify(f Facts) Classification
```

- Classification rules (implement exactly; each bullet is a test):
  - any required probe `Unknown` (remote configured/default/integration; metadata presence; live surface when metadata absent) → `unknown`.
  - `RemoteMetadata` Absent + `LiveSurface` Absent → `fresh`.
  - `RemoteMetadata` Absent + `LiveSurface` Present → `legacy`.
  - `RemoteMetadata` Present + `MetadataRoot == RootForeign` → `conflict` (reason `metadata-root-foreign`).
  - `RemoteMetadata` Present (parentless) + `LiveSurface` Present → `partial` with `PartialMetadataSeeded` (a seeded-but-unpruned migration), UNLESS the seed tree does not correspond (gatherer sets `RootForeign` then) → conflict.
  - metadata topology complete + `PendingReviewPaths` non-empty → `needs-review`.
  - every postcondition Present/true (including worktree registered+clean+synchronized+hooks off, committed ignore block, no live surface, no legacy key, surfaces agree when authorized) → `healthy`.
  - dirty/ahead metadata worktree, foreign `.docket/`, diverged local metadata branch, surfaces disagree → `conflict` with a reason token each (`metadata-worktree-dirty`, `docket-dir-foreign`, `local-metadata-diverged`, `surfaces-drift`).
  - `PartialIntegrationPruned` → `partial`.
  - The classifier is pure and reasons about the desired post-pass state its caller passes in `Facts` — it never reads disk (learning `predicate-must-ask-the-post-pass-state` is the GATHERERS' burden; the classifier just must not conflate Unknown with Absent).

**Steps:**

- [ ] **Step 1: Write the failing tests.** Table-driven `TestClassify` in `classify_test.go` covering every bullet above plus: zero-value `Facts` classifies `unknown` (the safe default); every `State` value is reachable; no input yields an empty `State`; `Reasons` non-empty for `conflict`/`partial`/`unknown`. Add `TestPresenceZeroValueIsUnknown` in `probe_test.go` asserting `Presence(0) == PresenceUnknown`.
- [ ] **Step 2: Run to verify failure.** `go test -count=1 ./internal/reposetup/` — expect compile failure (package absent).
- [ ] **Step 3: Implement `probe.go` and `classify.go`** exactly per the interface block. Keep `Classify` a single readable decision ladder: unknown-checks first, then conflict, then partial, then legacy/fresh, then needs-review, then healthy; healthy is the fall-through that must re-verify every conjunct, never a default.
- [ ] **Step 4: Run to verify pass.** `go test -count=1 ./internal/reposetup/` — PASS.
- [ ] **Step 5: Mutation-probe one guard:** flip the `RemoteMetadata.Presence == PresenceUnknown` check to treat Unknown as Absent; `TestClassify`'s unknown cases must fail. Restore.
- [ ] **Step 6: Commit.** `git add internal/reposetup && git commit -m "feat(0352): reposetup three-valued probes and repository state classifier"`

---

### Task 2: gitcli setup-tree primitives and worktree hook disabling

**Files:**
- Create: `internal/gitcli/setuptree.go`, `internal/gitcli/hooksoff.go`
- Create: `internal/gitcli/setuptree_integration_test.go` (line 1 exactly `//go:build integration`, line 2 blank)
- Create: `tests/test_go_integration_gitcli_setuptree.sh`
- Modify: `tests/runtime-budgets.tsv` (new row + ledger note), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` reseed)

**Interfaces:**
- Produces:

```go
// setuptree.go
// EmptyTreeOID returns the repository's empty-tree object id via
// `git hash-object -t tree /dev/null` — hash-algorithm-agnostic, never a
// hardcoded SHA-1 literal.
func (c *Client) EmptyTreeOID(ctx context.Context, repo Repository) (ObjectID, error)

// TreeOID resolves <commit>^{tree}.
func (c *Client) TreeOID(ctx context.Context, repo Repository, commit ObjectID) (ObjectID, error)

// TreeOp is a closed tree-composition instruction, applied in order to a
// private temporary index (GIT_INDEX_FILE under a mktemp-templated dir):
type TreeOp struct {
	IncludePrefix *IncludePrefixOp // read-tree --prefix=<Prefix>/ <From>^{tree}:<Prefix>
	RemovePrefix  *RemovePrefixOp  // git rm --cached -r --ignore-unmatch equivalent via update-index
	RemovePath    *RemovePathOp
	PutBlob       *PutBlobOp       // hash-object -w --stdin + update-index --add --cacheinfo
}
type IncludePrefixOp struct{ From ObjectID; Prefix RepoPath }
type RemovePrefixOp struct{ Prefix RepoPath }
type RemovePathOp struct{ Path RepoPath }
type PutBlobOp struct{ Path RepoPath; Content []byte; Mode FileMode }

// BuildTree composes a tree object. base == "" starts from the empty index;
// otherwise the index is seeded with `read-tree <base>^{tree}` first.
// Exactly one field of each TreeOp must be set. IncludePrefix of an absent
// source prefix is an error (never a silent skip). Returns write-tree's OID.
func (c *Client) BuildTree(ctx context.Context, repo Repository, base ObjectID, ops []TreeOp) (ObjectID, error)

// CommitTree creates a commit object for tree with the given parents (empty
// slice → parentless root) via `git commit-tree`, hooks and signing disabled
// (-c core.hooksPath=<empty>, -c commit.gpgsign=false), identity pinned the
// same way CommitPaths pins it, subject+trailers composed by the existing
// composeCommitMessage/validateTrailer helpers.
func (c *Client) CommitTree(ctx context.Context, repo Repository, tree ObjectID, parents []ObjectID, subject string, trailers []Trailer) (ObjectID, error)

// RootCommits lists the parentless roots reachable from tip
// (`rev-list --max-parents=0 <tip>`). Callers prove orphan ancestry with
// len==1 && roots[0]'s tree/receipt checks.
func (c *Client) RootCommits(ctx context.Context, repo Repository, tip ObjectID) ([]ObjectID, error)

// hooksoff.go
// DisableWorktreeHooks ports scripts/disable-worktree-hooks.sh: ensure
// extensions.worktreeConfig=true (relocating core.worktree/core.bare to the
// main worktree config first if git demands it, exactly as the script does),
// create an empty absolute hooks dir under the worktree's private git dir, and
// set core.hooksPath there with `git config --worktree`. Idempotent.
func (c *Client) DisableWorktreeHooks(ctx context.Context, worktreeDir string) error
```

**Steps:**

- [ ] **Step 1: Write failing integration tests** in `setuptree_integration_test.go`, all named `TestIntegrationSetupTree*` (this exact prefix; NOT `TestIntegrationRepo*` — that is the existing `gitcli_repo` shard's prefix), each building its own repo under `t.TempDir()` with the package's existing integration-test fixture helpers: `TestIntegrationSetupTreeEmptyTreeOID` (matches `git hash-object -t tree /dev/null` run by the fixture), `TestIntegrationSetupTreeCommitTreeParentless` (commit over empty tree; `rev-list --max-parents=0` returns exactly it; no hooks fired — plant an executable failing pre-commit-style hook and prove it did not run), `TestIntegrationSetupTreeBuildTreeIncludeRemovePut` (seed a repo with `docs/changes/{active,archive}` + `docs/adrs`; compose a tree including two prefixes, removing one path, putting one blob; assert exact `git ls-tree -r` listing), `TestIntegrationSetupTreeBuildTreeAbsentPrefixErrors`, `TestIntegrationSetupTreeRootCommitsMultipleRoots` (merge of two roots → len 2), `TestIntegrationSetupTreeDisableWorktreeHooks` (add a linked worktree, disable, assert `git -C wt config --worktree core.hooksPath` is an existing empty absolute dir and `extensions.worktreeConfig` is true; run twice → idempotent).
- [ ] **Step 2: Verify failure.** `go test -tags integration -count=1 -run '^TestIntegrationSetupTree' ./internal/gitcli/` — compile failure.
- [ ] **Step 3: Implement `setuptree.go` and `hooksoff.go`.** Use `c.run` with a temp `GIT_INDEX_FILE` in the env for BuildTree (template `"${TMPDIR:-/tmp}"`-equivalent via `os.MkdirTemp` under the test-safe temp root); reuse `composeCommitMessage`, `validateCommitSubject`, `validateTrailer`, and the hooks/signing/identity flags exactly as `CommitPaths` spells them (read `commit.go` first and copy its `-c` set).
- [ ] **Step 4: Verify pass.** Same command as Step 2 — PASS. Also `go vet -tags integration ./internal/gitcli/`.
- [ ] **Step 5: Create the shard runner** `tests/test_go_integration_gitcli_setuptree.sh` — copy `tests/test_go_integration_gitcli_source.sh` byte-for-byte, then set the header comment and: `SHARD_PKG="./internal/gitcli"`, `SHARD_PREFIX="TestIntegrationSetupTree"`, `SHARD_MODE="normal"`.
- [ ] **Step 6: Measure and add the budget row.** Three standalone serial runs: `/usr/bin/time -p bash tests/test_go_integration_gitcli_setuptree.sh >/dev/null`. Size the row per the tsv rule from the worst reading; append the row (`tests/test_go_integration_gitcli_setuptree.sh<TAB><N><TAB>parallel`), a ledger note recording command+readings, and reseed `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` per that file's own header instructions.
- [ ] **Step 7: Run the partition guards.** `bash tests/test_go_integration_contract.sh` and `bash tests/test_runtime_budgets.sh` — both green.
- [ ] **Step 8: Commit.** `git add internal/gitcli tests/test_go_integration_gitcli_setuptree.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh && git commit -m "feat(0352): gitcli tree composition, parentless commits, root inspection, worktree hook disabling"`

---

### Task 3: Native managed `.gitignore` block

**Files:**
- Create: `internal/reposetup/gitignore.go`, `internal/reposetup/gitignore_test.go`

**Interfaces:**
- Produces:

```go
// GitignoreStart/GitignoreEnd are the exact marker lines owned by
// scripts/lib/docket-gitignore-block.sh ("# docket:start (managed by docket — do not hand-edit)"
// / "# docket:end"). GitignoreBlock() returns the canonical block bytes
// (markers inclusive, LF line endings) — byte-identical to the bash
// emit_docket_gitignore_block output.
func GitignoreBlock() []byte

// EnsureGitignoreBlock returns the new full-file bytes with the managed block
// present exactly once (replacing a stale block, upgrading the legacy 0051
// marker spelling, appending with a separating blank line when absent), plus
// changed=false when input already canonical. Malformed markers (dangling /
// out-of-order / nested, either marker generation) → error, input untouched.
func EnsureGitignoreBlock(current []byte) (out []byte, changed bool, err error)

// ValidGitignoreBlock reports whether the committed file bytes contain the
// exact canonical block (used by check against the integration COMMIT tree).
func ValidGitignoreBlock(fileBytes []byte) bool
```

**Steps:**

- [ ] **Step 1: Write failing tests:** canonical block frozen as an expected literal (with a header comment quoting the bash lib's clause `single home for ALL docket-owned ignores` as the drift anchor); ensure-on-empty, ensure-on-existing-user-lines (outside bytes byte-preserved), ensure-idempotent (`changed=false`), stale-block replacement, legacy-marker upgrade, and refusal on each malformed shape: dangling start, dangling end, end-before-start, nested start (error + input untouched — assert the returned bytes are nil and the caller's slice unmodified).
- [ ] **Step 2: Verify failure.** `go test -count=1 -run TestGitignore ./internal/reposetup/`
- [ ] **Step 3: Implement.** Transcribe the block content from `scripts/lib/docket-gitignore-block.sh` (`emit_docket_gitignore_block`): core entries `.docket/`, `.worktrees/`, `.claude/settings.local.json`, then `.docket.local.yml`, then per-harness `.{claude,codex,cursor,opencode,agents,kiro,windsurf}/agents/docket-*.md`, `.codex/agents/docket-*.toml`, `.cursor/rules/docket-dispatch.mdc`. Marker balance validation mirrors `_docket_gi_malformed` (order-aware, both marker generations).
- [ ] **Step 4: Verify pass**, then **Step 5:** cross-language parity is proven later in Task 8's integration fixture (`TestIntegrationRepoSetupGitignoreParity` shells `bash -c '. scripts/lib/docket-gitignore-block.sh && emit_docket_gitignore_block'` and byte-compares with `GitignoreBlock()`) — note it in this file's header so the tie is discoverable (learning `frozen-copy-needs-a-drift-assert`).
- [ ] **Step 6: Commit.** `git add internal/reposetup && git commit -m "feat(0352): native managed gitignore block emitter and marker validation"`

---

### Task 4: Byte-preserving `metadata_branch` key removal

**Files:**
- Create: `internal/reposetup/configedit.go`, `internal/reposetup/configedit_test.go`

**Interfaces:**
- Produces:

```go
// RemoveMetadataBranchKey removes the single top-level `metadata_branch`
// entry (its key line plus any continuation lines of that mapping value) from
// the exact pinned .docket.yml bytes, preserving every other byte: unknown
// settings, comments, ordering, quoting, blank lines, and line endings.
// Returns (out, true, nil) when removed; (nil,false,nil) when the key is
// absent; an error when the file cannot be safely edited (undecodable YAML,
// duplicate top-level key, key not a top-level mapping entry).
func RemoveMetadataBranchKey(src []byte) ([]byte, bool, error)
```

**Steps:**

- [ ] **Step 1: Write failing tests.** Cases: key present first/middle/last; key with trailing comment on its line; key absent → `(nil,false,nil)`; CRLF file preserves CRLF elsewhere; comments and unknown keys byte-identical (assert `bytes.Equal` on everything but the removed lines); duplicate `metadata_branch` → error; `metadata_branch` nested under another map is NOT removed and reports absent; undecodable YAML → error. Round-trip guard (learning `validator-must-match-the-reader-it-feeds`): every successful edit's output must still parse through `internal/config`'s own file decoder with the key gone and every other decoded setting equal — use the exported parse entry `config` already exposes for repository files (see `internal/config/parse.go`; use the same function `fs.go`'s loader calls).
- [ ] **Step 2: Verify failure**, **Step 3: Implement** (locate the key with a YAML AST parse for VALIDITY, but perform the removal as a line-range splice on the raw bytes — never re-serialize the document), **Step 4: Verify pass.**
- [ ] **Step 5: Commit.** `git commit -am "feat(0352): byte-preserving removal of the legacy metadata_branch config key"`

---

### Task 5: Closed frontmatter repair roster

**Files:**
- Create: `internal/reposetup/repair.go`, `internal/reposetup/repair_test.go`

**Interfaces:**
- Consumes: `internal/document` (`Parse`, `Document.Field`, `Field.Shape/Span`, `PatchSet`), `internal/repository` (`BuildSnapshot` for postcondition validation — invoked by callers, not here).
- Produces:

```go
type RepairCode string
const (
	RepairQuoteScalar    RepairCode = "quote-unsafe-scalar"
	RepairScalarToList   RepairCode = "scalar-to-list"
	RepairDropClaimedAt  RepairCode = "drop-terminal-claimed-at"
)

type RepairFinding struct {
	Path       string     // repo-relative record path
	Field      string
	Code       RepairCode // empty for non-repairable findings
	Repairable bool
	Message    string
	Patch      []byte // unified-diff-style preview for repairable findings; nil otherwise
}

// PlanRepairs inspects one change record's bytes. It returns repairable
// findings for exactly the closed roster and non-repairable findings for
// everything else it can name (undecodable document, duplicate key, missing
// value, invalid domain token, ambiguous shape). archived=true enables
// RepairDropClaimedAt only for a record whose status is terminal.
func PlanRepairs(path string, src []byte, archived bool) ([]RepairFinding, error)

// ApplyRepairs applies the given repairable findings to src and returns the
// repaired bytes. It re-parses the candidate and verifies each intended
// postcondition (decoded value unchanged for quoting; same items same order
// for list conversion; key absent for claimed_at) — any failure is an error
// and no bytes are returned.
func ApplyRepairs(src []byte, findings []RepairFinding) ([]byte, error)

// RepairDigest is the stable sha256 hex digest over the canonical encoding of
// an ordered repair plan (path, field, code, patch bytes) — named in the
// migration receipt.
func RepairDigest(findings []RepairFinding) string
```

- Eligibility preconditions (all enforced, each with a refusal test): one balanced uniquely-located frontmatter block (`document.Parse` succeeds and `HasFrontmatter`), unique key (Parse already rejects duplicates), schema admits exactly one canonical replacement, authored body and unknown fields byte-identical (patches touch only the located value span / the key's line), reparse proves the domain postcondition.
- Roster semantics: (1) quote a located string scalar whose decoded text is unambiguous but whose token shape violates the scalar-safety rule (colon-space, trailing colon, ` #`, leading indicator character, boolean keyword) — single-quoted output, decoded text unchanged; (2) convert a known list field (`depends_on`, `related`, `discovered_from`, `adrs`, `blocked_by` — take the authoritative list-field roster from `internal/repository`'s decode layer, not a hand copy) stored as one scalar ONLY when that scalar itself parses as an exact valid sequence of the expected item type, emitting an UNQUOTED flow sequence with the same items in order; (3) remove `claimed_at` from an already-terminal archived change.

**Steps:**

- [ ] **Step 1: Write failing tests.** For each roster entry: at least two eligible shapes repaired with byte-precise expected output, and every adjacent ambiguous shape refused as non-repairable — e.g. for (1): a scalar whose decoded text is ambiguous, a multi-line scalar, a flow collection (must NOT be quoted — quoting one is a defect per the repo rule); for (2): a scalar that parses as a sequence of the wrong item type, a partial sequence, an already-list field; for (3): `claimed_at` on an ACTIVE record (refused), on a non-terminal archived record (refused). Plus: undecodable document → single non-repairable finding; `ApplyRepairs` postcondition failure path (hand it a finding whose patch was tampered) → error; `RepairDigest` stable across runs and sensitive to order/content.
- [ ] **Step 2: Verify failure. Step 3: Implement. Step 4: Verify pass** (`go test -count=1 -run TestRepair ./internal/reposetup/`).
- [ ] **Step 5: Mutation-probe:** delete the terminal-status check inside the `claimed_at` eligibility; the active-record refusal test must fail. Restore.
- [ ] **Step 6: Commit.** `git commit -am "feat(0352): closed mechanically-safe frontmatter repair roster"`

---

### Task 6: Init and migration effect planners with versioned receipts

**Files:**
- Create: `internal/reposetup/receipt.go`, `internal/reposetup/receipt_test.go`
- Create: `internal/reposetup/initplan.go`, `internal/reposetup/initplan_test.go`
- Create: `internal/reposetup/migrateplan.go`, `internal/reposetup/migrateplan_test.go`

**Interfaces:**
- Produces:

```go
// receipt.go — versioned operation markers carried as commit trailers
// (validated by gitcli's validateTrailer rules; scanned back with
// ScanCommitTrailers).
const (
	TrailerOperation      = "Docket-Operation"        // values below
	TrailerSourceRevision = "Docket-Source-Revision"  // exact integration OID
	TrailerMetadataRev    = "Docket-Metadata-Revision"
	TrailerCopyDigest     = "Docket-Copy-Digest"
	TrailerRepairDigest   = "Docket-Repair-Digest"
	OpInitRoot            = "repository-init-root/v1"
	OpMigrateSeed         = "repository-migrate-seed/v1"
	OpMigratePrune        = "repository-migrate-prune/v1"
)
// reposetup stays gitcli-free: Trailer is a local pair the app layer maps to
// gitcli.Trailer when committing and back when scanning.
type Trailer struct{ Key, Value string }
type Receipt struct { Operation string; SourceRevision, MetadataRevision, CopyDigest, RepairDigest string }
func (r Receipt) Trailers() []Trailer
func ParseReceipt(trailers []Trailer) (Receipt, bool)

// initplan.go
type InitPlan struct {
	RootSubject   string   // "docket: initialize metadata branch" + OpInitRoot trailer
	RootTrailers  []Trailer
	MetadataRef   string   // refs/heads/<metadata branch>
	WorktreePath  string   // <primary>/.docket
	GitignorePath string   // .gitignore (edit prepared, unstaged)
	SeedInput     reposeed.PlanInput // 0351 pass input for authorized surfaces
}
// PlanInit is pure: config+facts in, effects out. Returns an error when facts
// do not classify fresh (callers pre-classify; this re-checks — defense in depth).
func PlanInit(cfg config.Effective, f Facts, primaryRoot string) (InitPlan, error)

// migrateplan.go
type CopySet struct{ Prefixes []string }  // changes dir, ADR dir, SpecsDir — whole prefixes
type RemovalSet struct{ ActiveDir, BoardPath, ReadmePath string }
type MigrationPlan struct {
	Copy          CopySet
	Removal       RemovalSet
	ConfigEdit    bool // legacy key present → one .docket.yml edit
	SeedReceipt   Receipt // OpMigrateSeed, SourceRevision + CopyDigest + RepairDigest
	PruneReceipt  Receipt // OpMigratePrune, SourceRevision + MetadataRevision (filled after seed publishes)
}
func PlanMigration(cfg config.Effective, sourceRevision string, repairs []RepairFinding) (MigrationPlan, error)
const SpecsDir = "docs/superpowers/specs"
// CopyDigest is sha256 over the sorted (path, blob-oid) pairs the seed tree
// will contain — computed by the app layer from the composed tree listing and
// passed in; the planner only names WHICH prefixes.
```

**Steps:**

- [ ] **Step 1: Write failing tests.** `receipt_test.go`: round-trip encode/parse, unknown operation → `(_, false)`, trailer values with control bytes rejected. `initplan_test.go`: plan on fresh facts yields exactly the six spec'd effects and NO sample-corpus paths (assert the plan carries no seed files — the orphan root is the empty tree); plan on non-fresh facts errors; `SeedInput.Harnesses` populated only from an explicit repo/repo-local `agent_harnesses` (absent key → empty, learning `opt-in-signal-not-file-presence`). `migrateplan_test.go`: copy prefixes exactly `{cfg.ChangesDir, cfg.ADRsDir, SpecsDir}`; removal exactly `{<changes>/active, <changes>/BOARD.md, <changes>/README.md}`; plans/results/source paths NEVER appear in either set; `ConfigEdit` true only when the legacy key fact is Present.
- [ ] **Step 2: Verify failure. Step 3: Implement. Step 4: Verify pass** (`go test -count=1 ./internal/reposetup/`).
- [ ] **Step 5: Commit.** `git commit -am "feat(0352): init and migration effect planners with versioned receipts"`

---

### Task 7: Health findings, remedies, and the check exit contract

**Files:**
- Create: `internal/reposetup/health.go`, `internal/reposetup/health_test.go`

**Interfaces:**
- Produces:

```go
type Severity string
const (SeverityError Severity = "error"; SeverityWarning Severity = "warning")

type Finding struct {
	Code       string   // stable machine token, e.g. "live-surface-present"
	Severity   Severity
	Ref        string   // path or ref the finding is about
	Message    string
	Remedy     string   // exact human remedy, state-branched
	Repairable *bool    // only for frontmatter findings; nil otherwise
}

// EvaluateHealth maps a Classification + Facts (+ frontmatter findings the
// caller gathered) to the ordered finding list. healthy → empty. Every
// non-healthy state yields at least one finding. Deterministic order:
// remote/topology findings, then integration-tree findings, then local
// worktree findings, then surface findings, then frontmatter findings.
func EvaluateHealth(c Classification, f Facts, fm []RepairFinding) []Finding

// CheckExit maps state+findings to the 0/1/2 contract:
// healthy→0; unknown (or invalid usage, mapped by CLI)→2; everything else→1.
func CheckExit(c Classification, findings []Finding) int
```

- Remedy rules (each is a test): `fresh` → remedy names `docket repository init`; `legacy` → `docket repository migrate`; `needs-review` → lists the exact pending paths to review and commit; `partial` → names the safe continuation (re-run the same command); `conflict` → names the human disposition and NEVER a destructive command; a dirty metadata worktree remedy says commit/inspect, never discard.

**Steps:**

- [ ] **Step 1: Write failing tests** for each state→finding mapping, the ordering, the remedy rules above (assert the remedy string CONTAINS the exact command for the fixture's state and does NOT contain it for neighboring states — learning `printed-remedy-state-validity`), and the full `CheckExit` matrix including `unknown → 2`.
- [ ] **Step 2–4: fail → implement → pass** (`go test -count=1 -run 'TestHealth|TestCheckExit' ./internal/reposetup/`).
- [ ] **Step 5: Commit.** `git commit -am "feat(0352): health findings, exact remedies, and the 0/1/2 check exit mapping"`

---

### Task 8: App fact gatherer + `repository init` service + CLI + first integration shard

**Files:**
- Create: `internal/app/repository_facts.go`, `internal/app/repository_facts_test.go`
- Create: `internal/app/repository_init.go`, `internal/app/repository_init_test.go`
- Create: `internal/cli/repository.go`, `internal/cli/repository_test.go`; Modify: `internal/cli/root.go`
- Create: `internal/app/reposetup_integration_test.go`
- Create: `tests/test_go_integration_app_reposetup.sh`; Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Produces:

```go
// repository_facts.go
func setupRemote() gitcli.RemoteName // returns originRemote; the ONLY spelling site

// SetupDeps carries the seams every repository service shares.
type SetupDeps struct {
	Git    *gitcli.Client
	RepoDir string // invocation dir; Discover resolves the canonical primary
}

// GatherSetupFacts resolves config (mutation preflight for init/migrate,
// read preflight for check), discovers the canonical repository, fetches the
// authoritative remote default/integration/metadata refs, and fills
// reposetup.Facts — every probe error lands as PresenceUnknown WITH the error
// retained in the returned diagnostics, never as Absent.
func GatherSetupFacts(ctx context.Context, d SetupDeps, forMutation bool) (reposetup.Facts, setupContext, error)
// setupContext (unexported) carries cfg config.Effective, repo gitcli.Repository,
// remote tips, and the pinned integration revision for later phases.

// repository_init.go
type RepositoryOpResult struct {
	Envelope
	RepositoryState string   `json:"repository_state"`          // reposetup.State
	PendingPaths    []string `json:"pending_paths,omitempty"`
	MetadataTip     string   `json:"metadata_revision,omitempty"`
	SourceRevision  string   `json:"source_revision,omitempty"`
	Findings        []reposetup.Finding `json:"findings,omitempty"`
	human           string
}
func (r RepositoryOpResult) HumanText() string
func RunRepositoryInit(ctx context.Context, d SetupDeps) RepositoryOpResult
```

- Init sequencing (each numbered effect verified in integration tests): classify; refuse non-fresh (`invalid-state`, remedy pointing at migrate when legacy, at check otherwise); then (1) `EmptyTreeOID` + `CommitTree` parentless with `OpInitRoot` receipt; (2) `PushCreateLease` to `refs/heads/<metadata>` — on `create rejected` re-read the remote: expected shape (parentless root + receipt or exact legacy-equivalent empty seed) → adopt and continue (idempotent), anything else → `conflict`; (3) create/adopt the local branch and `AttachBranchWorktree` at `<primary>/.docket`; (4) `DisableWorktreeHooks`; (5) `EnsureGitignoreBlock` written to the primary worktree UNSTAGED + `reposeed`-planned surfaces (only when authorized) via the same `install` target machinery `repophase.go` uses; (6) write the ownership record for the surfaces actually owned. Result: `applied` with `repository_state: "needs-review"`, exit 0, every pending path named. Re-run converges: `no-op`/`applied` with no second root commit, branch, worktree, block, or record.

**Steps:**

- [ ] **Step 1: Default-tag tests first.** `repository_facts_test.go`: probe-error→Unknown mapping (fake the prober seam — extract a small unexported interface over the gitcli methods the gatherer calls so fakes can inject per-probe failures), authorized-surfaces gate (global-layer `agent_harnesses` does NOT authorize). `repository_init_test.go`: refusal mapping (legacy→invalid-state naming migrate; unknown→invalid-state naming check), result JSON field names, `HumanText` names every pending path. `internal/cli/repository_test.go`: command tree registered (`docket repository init|migrate|check` exist; unknown subcommand fails); `--json` flag flows to the presenter; check's exit path calls `CheckExitCode` (assert via a stubbed runner seam consistent with how existing cli tests stub app calls — read `internal/cli/change_test.go` for the established pattern first).
- [ ] **Step 2: fail → implement → pass** (`go test -count=1 ./internal/app/ ./internal/cli/` — default corpus only).
- [ ] **Step 3: Write the integration tests** in `reposetup_integration_test.go`, prefix `TestIntegrationRepoSetup*`: `…FreshInitCreatesTopology` (bare upstream + clone with one integration commit; run init; assert remote `docket` exists, exactly one parentless root, EMPTY tree (`git ls-tree` empty), receipt trailers present, `.docket/` registered on the branch, hooks path set, gitignore edit present-and-unstaged (`git status --porcelain` shows ` M .gitignore` or `??`), ownership record written only when harnesses authorized); `…RepeatInitConverges` (run twice; single root, single worktree, byte-identical block); `…InitRefusesLegacy` (live `docs/changes/active` on integration → invalid-state, remedy names migrate, remote untouched); `…InitRefusesForeignMetadataBranch` (pre-push a non-empty foreign `docket` → conflict refusal, foreign branch byte-untouched); `…InitRefusesDirtyPrimary`; `…GitignoreParity` (bash-lib emitter vs `reposetup.GitignoreBlock()` byte equality — Task 3's drift tie); `…InitDoesNotPrompt` (no stdin read: run with a closed stdin).
- [ ] **Step 4: Verify.** `go test -tags integration -count=1 -run '^TestIntegrationRepoSetup' ./internal/app/` — PASS; every individual test well under 60s (`-v` timings recorded in evidence).
- [ ] **Step 5: Shard runner + budget row.** Create `tests/test_go_integration_app_reposetup.sh` (`SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationRepoSetup"`, `SHARD_MODE="normal"`); measure 3× standalone serial, add the row + ledger note, reseed `EXPECTED_TOTAL`; run `tests/test_go_integration_contract.sh` + `tests/test_runtime_budgets.sh`.
- [ ] **Step 6: Mutation-probe the create-only protection:** change init's push from `PushCreateLease` to a plain `PushLease` against the foreign tip; `…InitRefusesForeignMetadataBranch` must fail. Restore, re-run, record.
- [ ] **Step 7: Commit.** `git add internal/app internal/cli tests/ && git commit -m "feat(0352): repository init service, CLI command group, and reposetup integration shard"`

---

### Task 9: `repository check` service and CLI wiring

**Files:**
- Create: `internal/app/repository_check.go`, `internal/app/repository_check_test.go`
- Modify: `internal/cli/repository.go`, `internal/cli/repository_test.go`
- Modify: `internal/app/reposetup_integration_test.go` (check scenarios join the same `TestIntegrationRepoSetup` shard)

**Interfaces:**
- Produces:

```go
type RepositoryCheckResult struct {
	Envelope                     // operation "repository-check"; result no-op (read-only success) or invalid-state family for undeterminable authority
	RepositoryState string              `json:"repository_state"`
	Findings        []reposetup.Finding `json:"findings"`
	Revisions       map[string]string   `json:"revisions"` // remote-default/remote-integration/remote-metadata/local-metadata
	human           string
}
func (r RepositoryCheckResult) HumanText() string   // repo, state, then per finding: code, severity, ref, message, remedy
func (r RepositoryCheckResult) CheckExitCode() int  // reposetup.CheckExit
func RunRepositoryCheck(ctx context.Context, d SetupDeps) RepositoryCheckResult
```

- Check MAY fetch (`FetchBranch` on default/integration/metadata — same bounded read behavior as native status) but performs NO other write: no working-tree, index, local-branch, config, ownership, worktree-registration, or remote-ref effect. It validates the remote metadata corpus via `BuildSnapshot` over blobs read from the metadata tip (`OpenObjectSource` + `ReadBlobs`), gathers frontmatter findings via `PlanRepairs` (report-only, exact patches included), and proves the committed-ignore guarantee from the integration COMMIT tree.

**Steps:**

- [ ] **Step 1: Default tests:** human/JSON equivalence (same findings, revisions, state in both renderings — decode the JSON and compare against the struct the human text was rendered from), exit mapping per state, `--json` output includes `repairable` on frontmatter findings.
- [ ] **Step 2: fail → implement → pass.**
- [ ] **Step 3: Integration scenarios** (append to `reposetup_integration_test.go`): `TestIntegrationRepoSetupCheckFresh` (exit 1, remedy names init), `…CheckNeedsReviewAfterInit` (init, then check → 1 with pending paths; commit the edits → 0), `…CheckHealthyFullPostcondition` (assert every healthy conjunct the spec lists by then breaking each one in sub-cases: reintroduce live surface on integration → 1; re-add `metadata_branch` key → 1; strip the committed ignore block from the integration COMMIT while leaving the working tree intact → 1, proving the probe reads the commit; dirty the metadata worktree → 1 with a non-destructive remedy; track `.docket/` in the integration tree → 1), `…CheckIsReadOnly` (snapshot `git for-each-ref` + working tree hashes before/after check on every fixture state; only `refs/remotes/*` may differ), `…CheckUnknownAuthority` (remote URL pointing at a nonexistent path → exit 2, never 1/0).
- [ ] **Step 4: Verify + re-measure the shard** (`/usr/bin/time -p bash tests/test_go_integration_app_reposetup.sh >/dev/null` 3×); if the worst reading now exceeds the row, split check scenarios into their own shard/prefix instead of raising the row (budget rule); otherwise update the ledger note with the new readings.
- [ ] **Step 5: Mutation-probe the committed-ignore probe:** point it at the working tree instead of the integration commit; the strip-from-commit sub-case must fail. Restore.
- [ ] **Step 6: Commit.** `git commit -am "feat(0352): read-only repository check with machine-readable findings and 0/1/2 exits"`

---

### Task 10: `repository migrate` service, confirmation protocol, and migration shard

**Files:**
- Create: `internal/app/repository_migrate.go`, `internal/app/repository_migrate_test.go`
- Modify: `internal/cli/repository.go`, `internal/cli/repository_test.go` (flags `--yes`, `--repair-frontmatter`; TTY confirm flow)
- Create: `internal/app/repomigration_integration_test.go`
- Create: `tests/test_go_integration_app_repomigration.sh`; Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Produces:

```go
type MigrateOptions struct {
	Authorized       bool   // true only via --yes or an interactive confirmed preview
	RepairAuthorized bool   // --repair-frontmatter
	ExpectedSource   string // the pinned integration OID the preview showed; "" on first (preview) pass
}
type RepositoryMigrateResult struct {
	Envelope
	RepositoryState string              `json:"repository_state"`
	SourceRevision  string              `json:"source_revision"`
	MetadataTip     string              `json:"metadata_revision,omitempty"`
	IntegrationTip  string              `json:"integration_revision,omitempty"`
	CopyPrefixes    []string            `json:"copy_prefixes"`
	RemovedPaths    []string            `json:"removed_paths"`
	Repairs         []reposetup.RepairFinding `json:"repairs,omitempty"`
	PendingLocal    []string            `json:"pending_local,omitempty"` // remedy steps when local moved post-publish
	human           string
}
func RunRepositoryMigrate(ctx context.Context, d SetupDeps, o MigrateOptions) RepositoryMigrateResult
```

- Two-pass CLI protocol (decide-and-act on the same copy): unauthorized run returns the full plan (resolved repo, remote, exact integration revision, destination, copy set, removal set, config edit, complete repair diff) as `invalid-state` reason `confirmation-required` (non-interactive without `--yes`) or, interactively, the CLI prints it, prompts `migrate? [y/N]`, and on yes re-invokes with `Authorized: true, ExpectedSource: <shown OID>`. The service refuses `contended` if the fresh authoritative integration tip differs from `ExpectedSource`. Repairable findings present + `!RepairAuthorized` on a non-interactive run → the plan plus refusal before any write; interactive confirmation covers repairs because the diff was in the preview.
- Migration sequencing: pin source revision → read + validate every active/archived record (`PlanRepairs` + `BuildSnapshot`); any non-repairable error finding blocks; apply approved repairs in-memory (`ApplyRepairs`); compose the seed tree (`BuildTree`: IncludePrefix ×3 + PutBlob for repaired records) and require ZERO error findings on the complete repaired candidate before any branch changes; `CommitTree` parentless with `OpMigrateSeed` receipt (source revision, copy digest, repair digest); `PushCreateLease` (or exact-lease when resuming an owned partial); re-read and verify the remote postcondition byte-exactly; verify every seed path FROM THE REMOTE before pruning; compose the integration descendant (`BuildTree` on the source tree: RemovePrefix active/, RemovePath BOARD.md + README.md, PutBlob edited `.docket.yml` (when key present), PutBlob `.gitignore` with the managed block, PutBlob authorized surface bytes) with `OpMigratePrune` receipt naming source + metadata revisions; `PushLease` with the exact source-revision lease; re-read; then local finish: fast-forward the still-clean primary IFF it still equals the source revision, attach `.docket/`, disable hooks, publish the ownership record. Local moved post-publish → remote stays, result `applied` with `pending_local` naming the exact synchronization remedy.

**Steps:**

- [ ] **Step 1: Default tests** (`repository_migrate_test.go` + cli tests): authorization matrix — non-interactive: {no flags → plan+refusal, `--yes` alone with repairs present → plan+refusal naming `--repair-frontmatter`, `--yes --repair-frontmatter` → proceeds, `--repair-frontmatter` alone → still needs `--yes`}; `ExpectedSource` mismatch → `contended`; init/check/install never set `Authorized` (compile-level: the option type lives only in the migrate path).
- [ ] **Step 2: fail → implement → pass.**
- [ ] **Step 3: Integration tests**, prefix `TestIntegrationRepoMigration*`, fixture = a scripted legacy repo (bare upstream + clone whose integration branch carries `docs/changes/{active,archive,learnings}`, `BOARD.md`, `README.md`, indexes, `docs/adrs`, `docs/superpowers/specs`, unknown stray files inside those trees, plans/results/source OUTSIDE them, and a `.docket.yml` with `metadata_branch: docket` plus comments/unknown keys): `…ExactCopyAndRemovalSets` (remote `docket` tree ls-tree equals exactly the three prefixes including the unknown files; integration descendant lacks exactly active/BOARD/README; archived changes, ADRs, specs, plans, results still on integration; NOTHING else differs byte-wise — diff the two integration trees and assert the changed-path set exactly), `…LegacyKeyRemovedBytePreserving` (comments/unknown keys/ordering byte-identical), `…ReceiptsNameExactRevisions` (ScanCommitTrailers on both new commits), `…RepairsLandInBothTreesForArchives` (archived repair byte-identical in seed AND retained integration copy; active repair only in seed because the copy is removed), `…NonRepairableFindingBlocksBeforeAnyWrite` (malformed archived record → refusal, both remotes untouched), `…NoTerminalPublishResurrection` (a terminal record present only on `docket`-side fixtures stays off integration), `…MigrateIsIdempotent` (re-run with `--yes` → `no-op` keyed on the REMOTE postconditions), `…LocalMovedAfterPublish` (advance the local primary between preview and finish via the test harness hook — inject with a `testHookAfterRemotePublish func()` seam on the service, nil in production — remote intact, `pending_local` names the remedy, retry performs only local work).
- [ ] **Step 4: Verify** (`go test -tags integration -count=1 -run '^TestIntegrationRepoMigration' ./internal/app/`), create `tests/test_go_integration_app_repomigration.sh` (`SHARD_PREFIX="TestIntegrationRepoMigration"`), measure 3×, add row + ledger + `EXPECTED_TOTAL` reseed, run contract + budget guards.
- [ ] **Step 5: Mutation-probes:** (a) skip the full-corpus validation before seed publication → `…NonRepairableFindingBlocksBeforeAnyWrite` fails; (b) drop the `RepairAuthorized` gate → the `--yes`-alone default test fails; (c) prune on local-branch evidence instead of the remote re-read (point the seed-path verification at the local ref) → the response-loss test added in Task 11 will cover this; here assert `…ExactCopyAndRemovalSets` still passes and defer probe (c) to Task 11 Step 5. Restore after each.
- [ ] **Step 6: Commit.** `git commit -am "feat(0352): explicit authorized repository migration with exact copy/removal sets and receipts"`

---

### Task 11: Interruption, response-loss, and partial-state recovery

**Files:**
- Modify: `internal/app/repository_migrate.go`, `internal/app/repository_init.go` (recovery/adoption branches), `internal/app/repository_facts.go` (partial-phase probing: root shape, receipt parse, legacy-equivalent tree equality)
- Create: `internal/app/reporecovery_integration_test.go`
- Create: `tests/test_go_integration_app_reporecovery.sh`; Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Interfaces:**
- Consumes: the `testHookAfterRemotePublish`-style injection seams; generalize to `type setupHooks struct{ beforeSeedPush, afterSeedPush, beforePrunePush, afterPrunePush, beforeLocalFinish func() error }` (unexported field on the service deps, nil in production, settable only from package tests).
- Produces: recovery behavior per the spec's six boundaries — no new exported API.

**Steps:**

- [ ] **Step 1: Write failing integration tests**, prefix `TestIntegrationRepoRecovery*`, one per spec boundary: `…BeforeSeedPushLeavesNoRemote` (hook errors before push → no remote effect, temp state removed — assert no stray refs/worktrees/temp dirs by ownership shape, retry replans from fresh authority and succeeds), `…SeedPushResponseLost` (push succeeds, hook makes the service treat the response as lost/unknown → re-run re-reads remote `docket`, accepts the exact expected shape, continues; a tampered seed (amend one byte via the harness) → `conflict`, nothing destroyed), `…BashShapedPartialSeedAdopted` (fixture seeds remote `docket` the way the BASH migration would — parentless, exact copy-set tree, NO receipt trailers — with integration still live: classify `partial`, migrate resumes with the prune phase only; variant WITH integration already pruned but `.docket/` unattached: only the local steps run), `…IntegrationAdvancedBeforePrune` two variants: non-planning bytes changed → prune rebuilt atop the fresh tip; planning bytes changed → `docket` first updated under its exact owned lease with the complete re-validated seed, then prune; foreign `docket` advance → refusal, `…PrunePushResponseLost` (re-read: exact postcondition → success; mismatch → `contended`, no overwrite), `…AbruptDeathDebrisCleanup` (interrupted run leaves an owned temp worktree/ref with the invocation-unique owned naming; next invocation removes exactly it; a user worktree and an ambiguous registration survive and are reported — learning `probe-error-is-not-clean-absence`: make the debris probe error in a sub-case and assert the debris is RETAINED with a pending warning).
- [ ] **Step 2: Verify failure**, **Step 3: implement recovery branches** (fact gatherer learns receipt parsing + legacy-equivalent tree-equality probing: compose the expected seed tree from the CURRENT pinned source and compare tree OIDs; "legacy-equivalent" = tree matches even without a receipt), **Step 4: verify pass.**
- [ ] **Step 5: Shard + budget:** `tests/test_go_integration_app_reporecovery.sh` (`SHARD_PREFIX="TestIntegrationRepoRecovery"`), measure 3×, row + ledger + reseed, contract + budget guards. Then run deferred mutation probe (c) from Task 10: make seed-path verification read the local ref instead of the remote → `…SeedPushResponseLost` or `…IntegrationAdvancedBeforePrune` must fail. Restore.
- [ ] **Step 6: Commit.** `git commit -am "feat(0352): migration interruption recovery from durable remote postconditions"`

---

### Task 12: Contention and race shards

**Files:**
- Create: `internal/app/repocontention_integration_test.go` (normal), `internal/app/reposetup_race_integration_test.go` (line 1 `//go:build integration`, race prefix)
- Create: `tests/test_go_integration_app_repocontention.sh`, `tests/test_go_integration_app_reposetup_race.sh`; Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh`

**Steps:**

- [ ] **Step 1: Write failing tests.** `TestIntegrationRepoContention*` (sequential fixtures simulating interleaving via the hook seams): `…MetadataLeaseLoss` (second writer advances remote `docket` between re-read and prune-phase lease push → `contended`, no force, foreign advance intact), `…IntegrationLeaseLoss` (integration advances between seed and prune push → the Task 11 rebuild path; assert the lease value used is the FRESH re-read, learning `cas-re-read-fresh-origin`), `…CheckDuringMigrationSeesPartial` (after seed publish, `repository check` reports `partial` exit 1 with the resume remedy). `TestRaceIntegrationRepoSetup*` (real concurrency, in the `_race_integration_test.go` file): `…ConcurrentInitRace` (two goroutines race `RunRepositoryInit` against one upstream; exactly one create-push wins, the loser adopts or reports cleanly, postcondition identical to a single run), `…ConcurrentMigrateAndCheck` (migrate races repeated checks; checks never observe a torn state other than the classified partial, and never write).
- [ ] **Step 2–3: fail → implement whatever small refinements the races expose → pass** (`go test -tags integration -race -count=1 -run '^TestRaceIntegrationRepoSetup' ./internal/app/` and the normal prefix run).
- [ ] **Step 4: Shards + budgets:** `tests/test_go_integration_app_repocontention.sh` (`SHARD_PREFIX="TestIntegrationRepoContention"`, `SHARD_MODE="normal"`); `tests/test_go_integration_app_reposetup_race.sh` (`SHARD_PREFIX="TestRaceIntegrationRepoSetup"`, `SHARD_MODE="race"` — copy `tests/test_go_integration_app_concurrency.sh` as the race-mode template). Measure 3× each, rows + ledger + reseed, contract + budget guards green.
- [ ] **Step 5: Commit.** `git commit -am "feat(0352): repository setup contention and race coverage"`

---

### Task 13: Guard mutation sweep, forbidden-effect audit, and the full-suite gate

**Files:**
- Modify: none expected (fix-ups only if a probe finds a vacuous guard)

**Steps:**

- [ ] **Step 1: Complete the spec's mutation matrix** (those not already run in Tasks 1–12), each with the backup/mutate/prove/probe/restore procedure and `-count=1`: (a) strip `//go:build integration` from `reposetup_integration_test.go` → `tests/test_go_integration_contract.sh` check (6) reddens (default corpus leak); (b) rename one `TestIntegrationRepoMigration` test to a prefix no runner selects → contract check (5)/(4) reddens; (c) set the `reposetup` shard's tsv row above 60 → `tests/test_runtime_budgets.sh` reddens; (d) drop `-race` from the race shard by flipping `SHARD_MODE` → contract race-direction check reddens; (e) delete the repair opt-in gate → Task 10 default test reddens (re-run to confirm still wired). Record every reading in the build evidence.
- [ ] **Step 2: Forbidden-effect audit.** `git diff <base>..HEAD` (base = `c3906ddb`) — capture into a variable, then grep for `--force`, `push -f`, `reset --hard`, `update-ref -d`, `branch -D` in NEW non-test code: expected zero hits outside quoted prose. Also assert no new code spells `"origin"`/`"main"` outside `setupRemote()`/config defaults (`grep -rn '"origin"' internal/reposetup internal/app/repository_*.go internal/cli/repository.go` → only the one seam).
- [ ] **Step 3: Comment-anchor + hygiene sweeps.** `bash tests/test_comment_anchor_style.sh`; `bash scripts/check-test-source-hygiene.sh` (canonical assert bytes in all six new runners).
- [ ] **Step 4: Full suite.** Run the command `finalize.test_command` resolves to (read it via `docket diagnostic config`; never a second copy). Disposition every `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line; a `SERIAL CONFIRMED OVER BUDGET:` line on any new shard means re-cut that shard now (split the test file across two prefixes + runners), not raise a number.
- [ ] **Step 5: Commit any fix-ups.** `git commit -am "test(0352): guard mutation evidence and suite-gate fix-ups"` (omit if clean).

---

## Coverage Cross-Check (spec → task)

- One shared classifier, all states + probe outcomes → Task 1; `partial`/adoption probing → Task 11.
- Supported-contract preflight (clean primary, remote proven, whole-input validation before any write, `.docket/` never foreign, 0351 marker/ownership preflight) → Tasks 8–10 (gatherer + refusal tests).
- init effects 1–6, no-prompt, pending-path receipt, re-run convergence, empty orphan corpus → Task 8.
- migrate authorization (interactive preview, `--yes`, `--repair-frontmatter`), copy/removal sets, legacy-key removal, receipts, publication sequence, local-finish remedy → Task 10.
- Closed repair roster, eligibility, adjacent-shape refusals, preview/flag gating, both-tree landing, zero-error precondition → Tasks 5, 9 (report-only), 10.
- check semantics, fetch-only reads, healthy conjuncts, committed-ignore-from-commit, human/JSON equivalence, 0/1/2 → Tasks 7, 9.
- Idempotency + six interruption boundaries + bash-shaped partial adoption + debris ownership → Task 11.
- Contention, foreign refusal, planning-byte change between phases, races → Task 12 (+11).
- Application/adapter boundary (pure planners, typed gitcli facts, protocol envelope) → Tasks 1–7 (pure), 2 (adapters), 8–10 (services).
- Test architecture: partition, prefixes, one-runner membership, race direction, budgets < 60s, measured rows, mutation-tested guards, full-suite gate → every shard-bearing task + Task 13.

Out of scope, deliberately absent from every task (spec "Explicit exclusions"): no `git init`/remote creation, no local-only mode, no main-mode removal (0363), no general config writes beyond the one key removal, no sample seeding, no general repair API, no installer work, no Bash script modification or invocation in production code (the bash gitignore lib is only READ by a parity test), no terminal-publish restoration, no release/cutover work.
