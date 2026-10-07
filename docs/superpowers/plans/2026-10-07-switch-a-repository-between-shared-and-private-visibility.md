<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0533 — Switch a repository between shared and private visibility](../../changes/active/0533-switch-a-repository-between-shared-and-private-visibility.md)**
<!-- docket:backlink:end -->

# Switch a Repository Between Shared and Private Visibility: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:**
- A new `docket repository set-visibility <shared|private>` moves an existing repository between modes, keeping every record. It previews, applies under `--yes` (or an interactive confirmation pinned to the preview), resumes after interruption, and refuses while any run is live.
- `docket repository repair` also re-stamps the generated `docket:backlink` block of every metadata-branch artifact (spec, plan, results) as a relative link; `docket repository check` reports the stale ones.
- The `set-visibility` preview warns (never refuses) about metadata files that still carry absolute same-branch links, naming `docket repository repair`.

**Architecture:**
- **Pure pieces in `internal/reposetup`:** fold `.docket.yml` + `.docket.local.yml` into one private config, split it back, align `visibility`, remove the `.gitignore` / exclude blocks. Bytes in, bytes out.
- **Repair extension:** the check corpus also reads every artifact a change links (`artifactPathsOf`). A new derived view `artifact-backlinks` compares each artifact's backlink block with `render.ArtifactBacklinkContent`; repair re-stamps it the way close-out's `retargetArtifactBacklinks` does.
- **The switch** (`internal/app/repository_set_visibility*.go`) follows `repository_migrate.go`: two-pass authorization with a composite pin as `ExpectedSource`; a router over phases whose completion is read from **state** (refs on each remote, folder names, worktree registrations, config files), never from the detected mode alone; no force-push, no foreign-ref deletion, no rollback. It makes **no metadata commit**: it pushes the identical history under the other branch name, so receipts and relative links survive.
- **Integration-branch commits** go through one machine: a write-ahead journal in the state folder, `gitcli.CommitOwnPaths` (`git commit --only`; the user's hooks run), and two fixed subjects.

**Tech Stack:** Go, cobra/pflag, `go.yaml.in/yaml/v3` nodes, real-git tests behind `//go:build integration`: the existing `TestIntegrationRepoRepair` shard (`tests/test_go_integration_app_reporepair.sh`) and a **new** `TestIntegrationRepoVisibility` shard (`tests/test_go_integration_app_repovisibility.sh`).

**Spec:** `docs/superpowers/specs/2026-10-05-switch-a-repository-between-shared-and-private-visibility-design.md` (in the feature worktree). Read *Design*, *Commits on the integration branch*, and *Acceptance criteria* first. The change record's `## What changes` and `## Reconcile log` (2026-10-07) add the repair extension and the preview warning.

## Global Constraints

- **Command:** `docket repository set-visibility <shared|private> [--yes] [--metadata-remote <url>] [--delete-shared-branch] [--remove-shared-files] [--repo-dir <dir>]`.
- **Capability:** `repository.set-visibility`; effects `local-write`, `metadata-write`, `external-write`; result `RepositorySetVisibilityResult` in `schema_registry.go`; asset-independent (`assetIndependent["repository set-visibility"] = true` in `internal/cli/install.go`).
- **Subjects, verbatim, no body, no trailer:** going private `Remove docket configurations from repository`; going shared `Add docket configurations to repository`. Never `shared`, `public`, `private`, or `visibility`, in any case.
- **Never push the integration branch.** Commits are local, on the primary checkout's current branch, holding only the switch's own paths.
- **Pushes:** every `PushLease`/`PushCreateLease` sits in `publishVisibilityHistory`, added to `pushAllowlist` (`internal/repoguard/integration_push_test.go`) with "repository set-visibility publishes the identical metadata history under the target branch name". The deletion uses `DeleteRemoteRefLease` with the exact tip.
- **Spelling:** never `filepath.Join(..., "docket")` (`TestNoInlineStateFolderSpelling`); never a `"refs/heads/docket"`/`"origin/docket"` literal in non-test Go. Use `layout.SharedLayout`/`PrivateLayout`, `layout.SharedName`/`PrivateName`, `metadataRef`, `metadataRemote`.
- **Docs are command help, skills, and `.docket.example.yml` only.** Skills name the operation id `repository.set-visibility`, never the command, and never `.docket/` or `origin/docket`.
- **AGENTS.md:** mutate from a **backup copy**, `-count=1`; template `mktemp`; anchor comments on symbols; no pipes into `grep -q`/`head`; `go generate ./internal/assets/` after editing skills or `.docket.example.yml`.
- **Hermetic home:** integration tests reaching the private store or `installAuthorizedSurfaces` pin `HOME`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME` (`pinInitUserRoots`, `newPrivateInitRepo`, `newGitClient`).

## Review Focus

Each item has a test in its owning task.
1. **A `.docket.local.yml` setting a repo-only key** (`integration_branch: develop`, ignored while shared) must not become active after going private (Task 1, `TestFoldPrivateConfigDropsRepoOnlyLocalKeys`).
2. **The user's own work around the commit:** an unrelated staged file stays staged; an uncommitted edit to a commit path refuses, unless it is the switch's own journaled edit (Tasks 7, 11).
3. **A spec with an absolute backlink and an absolute `blob/docket` link in prose:** repair rewrites only the block; the preview still lists the file; `--yes` still succeeds (Tasks 3, 8).
4. **Round trip without `--delete-shared-branch`, with private writes between:** going shared fast-forwards the left-behind `origin/docket` instead of refusing (Task 10, `...SharedFastForwardsLeftBehindBranch`).
5. **A second clone after the first went private with both flags** (`origin/docket` gone, `.docket.yml` off origin's default branch): same machine, it adopts the store, folds config from its own `HEAD`, runs local phases only (Task 9, `...PrivateSecondCloneAfterCleanup`).

## Decisions made in planning

The results file records each as a deviation or residual.
- **Order differs from the spec's list; outcomes do not.** Remote phases (bare remote, `dckt` git remote, publish) run **before** the state-folder rename; the folded config is written into the **shared** state folder first, so the rename is the atomic flip to a configured private repository. Going shared, `.docket.yml` is written before the rename, `.docket.local.yml` after. Config resolves at every crash point, and the spec's phase 4/5 conflict (config first makes the rename target non-empty) is gone.
- **Metadata worktree:** clean remove + re-attach, not `git worktree move`: same postcondition, and hooks are re-scoped (the empty-hooks folder moved).
- **"Personal keys":** no schema class exists. Going private saves `.docket.local.yml` as `.git/dckt/local-keys.yml`; going shared, its leaves return to `.docket.local.yml` with **current** values, all else to `.docket.yml` (born-private: all). Repo-only leaves always go to `.docket.yml`; `visibility` never local.
- **"Origin already has docket" refuses only unrelated or diverged history.** Equal/ancestor/descendant tips are one backlog (fast-forward or nothing); else the documented round trip could never return. Two backlogs are still never merged.
- **Metadata sync is ancestor-or-equal** of the authoritative tip (a second clone whose `origin/docket` is gone can never be equal).
- **Fold source:** origin's default-branch `.docket.yml`, else the primary's `HEAD` one, else empty; the preview names it.
- **No private Cursor rule file exists** (#535 used a user-level hook); going shared removes only `.git/dckt/AGENTS.md`.
- **Open PRs are listed only going private** (private PR text carries nothing docket-identifiable); without GitHub the preview says so and never blocks.
- **Dispatch surfaces:** going private **disowns** working-tree surfaces in the moved ownership record (the private install never retires them); `--remove-shared-files` strips them itself.
- **Journal** `<state folder>/visibility-switch.json`: write-ahead record of every integration-branch path the switch writes, so a re-run tells its own edit from the user's; removed once `HEAD` holds them.
- **Backlinks:** spec, plan, results (`artifactPathsOf`), only where a balanced block exists; never inserted; a path shared by two changes is manual review.
- **`--yes` alone is unpinned** (like `migrate`/`repair`); the interactive confirm pins.
- **Live gates** = busy `worktree-locks/*/busy.lock` (held by a gate supervisor for life, change 0490).

## Learnings applied

probe-error-is-not-clean-absence, idempotency-keying (completion keyed on postconditions), decide-and-act-on-the-same-copy and cas-re-read-fresh-origin (pin, fresh re-reads), presence-encoded-state (journal, `local-keys.yml`, pending config removed on the way out), validator-must-match-the-reader-it-feeds (`config.Resolve` before writes), printed-remedy-state-validity, intermediate-task-state-buildable (Task 8), metadata-branch-invisible-to-suite (Task 3), plan-supplied-test-code-is-unverified.

## ADRs to record

Recorded by the implement-next parent through `docket-adr`; not a build task.
1. **A visibility switch preserves records by publishing the identical metadata history under the target branch name.** It never re-seeds and never rewrites a record. Relates to ADR-0099 and ADR-0001.

---

## Phase A: pure pieces and the repair extension

### Task 1: Config fold/split, visibility alignment, and block removal (pure)

**Files:**
- Create: `internal/reposetup/visibilityconfig.go` (+ `_test.go`)
- Modify: `internal/reposetup/gitignore.go`, `internal/reposetup/exclude.go` (+ tests)
- Modify: `internal/config/schema.go` (+ `setting_paths_test.go`)

**Interfaces (produces):**

```go
// internal/config: every registry path with scope == scopeRepoOnly, in registry order.
func RepoOnlyPaths() []string

// internal/reposetup
func FoldPrivateConfig(committed, local []byte, repoOnly []string) (folded []byte, dropped []string, err error)
func SplitPrivateConfig(private, localKeys []byte, repoOnly []string) (committed, local []byte, err error)
func ConfigLeafValues(src []byte, paths []string) (map[string]string, error) // leaf -> trimmed yaml.Marshal of its value; absent omitted
func RenderVisibilityEdit(existing []byte, value string) (edited []byte, changed bool, err error)
func RemoveGitignoreBlock(current []byte) (out []byte, changed bool, err error)
func RemoveExcludeBlock(current []byte) (out []byte, changed bool, err error)
```

Semantics:
- **Leaf:** a non-empty mapping is recursed; anything else (scalar, sequence, empty mapping) is a leaf, named by dotted path. This matches config's merge rule (blocks merge key by key; lists replace).
- **Agent pins** `agents.<h>.<a>.model|effort|runner` are never carried (config refuses them in repository layers) and are reported in `dropped`.
- **Repo-only:** leaf `p` matches entry `r` when `p == r` or `p` starts with `r + "."`.
- **Fold:** start from `committed`; overlay each `local` leaf except repo-only ones (reported in `dropped`: ignored while shared, must not become active); drop agent pins; set top-level `visibility: private`. Nil/whitespace input = empty mapping; a non-mapping root errors.
- **Split:** the leaf set of `localKeys` selects local leaves. A leaf goes to `local` iff it is in that set, not repo-only, not an agent pin, not `visibility`. All else goes to `committed`, which gets `visibility: shared`. `local` is nil when empty.
- **Encoding:** `yaml.NewEncoder` with `SetIndent(2)` over a document node, so node comments survive.
- **`RenderVisibilityEdit`:** rewrites only an **explicit, different** top-level `visibility` (absent or equal -> `(existing, false, nil)`), byte-exactly in the style of `RenderAgentHarnessesEdit` (`topLevelMapping`, `findChild`, `lineOffsets`), keeping a trailing comment; re-parse and refuse unless only that value changed.
- **Block removal:** malformed markers refuse with the existing error type (`*MalformedGitignoreError{Generation: "docket"}` / `*MalformedExcludeError{}`), file untouched; else `stripGitignoreBlock(current, start, end)` and trim trailing blank lines to one final newline.

- [ ] **Step 1: Write the failing tests** (table-driven; every fold/split output must also resolve through `config.Resolve` as a repository layer):
  - `TestFoldPrivateConfigMergesLeaves`: committed `finalize: {gate: local, test_command: make}` + local `finalize: {test_command: make quick}` -> `gate=local`, `test_command="make quick"`, `visibility=private`.
  - `TestFoldPrivateConfigDropsRepoOnlyLocalKeys` (Review Focus 1): local `integration_branch: develop`, committed `main` -> `main`; `dropped` names it.
  - `TestFoldPrivateConfigDropsAgentPins` (`agents.claude.docket-build.model` in either input -> gone, listed, empty `agents` removed); `nil, nil` -> exactly `visibility: private\n`; a `- a\n` root errors.
  - `TestSplitPrivateConfigRoutesSavedLocalLeaves`: private `{visibility: private, build.test_command: make, reclaim.auto: true, integration_branch: main}`, localKeys `{reclaim.auto: false, integration_branch: x}` -> committed `{build.test_command, integration_branch: main, visibility: shared}`, local exactly `reclaim.auto: true`.
  - `TestSplitPrivateConfigBornPrivate` (nil localKeys -> nil local); `TestFoldSplitRoundTrip`.
  - `TestRenderVisibilityEdit`: `visibility: shared # note\nx: 1\n` -> `visibility: private # note\nx: 1\n`; absent, equal, nested -> unchanged.
  - Block removal: outside lines byte-exact; block-only -> empty; dangling start -> refusal, nil `out`; none -> unchanged.
  - `TestRepoOnlyPaths`: `integration_branch, changes_dir, adrs_dir, results_dir, github_project, terminal_publish`.
- [ ] **Step 2: Run and watch them fail.** `go test ./internal/reposetup/ ./internal/config/ -count=1 -run 'Fold|Split|RenderVisibility|Remove(Gitignore|Exclude)|RepoOnlyPaths'`.
- [ ] **Step 3: Implement** with yaml.v3 nodes: `parseConfigMapping` (nil/whitespace -> empty `MappingNode`; root must be one mapping), `configLeaves(m, prefix) []configLeaf{path, key, val}` (depth-first, file order, recursing only non-empty mappings), `isAgentPin` (4 dotted parts, `agents` first, last `model|effort|runner`), `isRepoOnly`, `setLeaf` (creates intermediate mappings, replaces a value), `deleteLeaf` (prunes emptied parents), `setTopScalar`, `encodeConfig`.
- [ ] **Step 4: Run and confirm pass;** then `go test ./internal/reposetup/ ./internal/config/ -count=1`.
- [ ] **Step 5: Mutation-test.** From a backup (`mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"`), remove the fold's repo-only skip: `...DropsRepoOnlyLocalKeys` goes red. Restore with `cp`.
- [ ] **Step 6: Commit.** `feat(reposetup): fold and split repository config across layers and remove managed ignore blocks`

---

### Task 2: `repository check` reports stale artifact backlinks

**Files:**
- Modify: `internal/reposetup/derived.go` (+ test); `internal/app/repository_check.go`; `internal/app/finalize_closeout.go` (`retargetArtifactBacklinks`)
- Create: `internal/app/repository_check_backlinks_test.go` (default tag)

**Interfaces (produces):**

```go
// internal/reposetup
const DerivedViewArtifactBacklinks DerivedView = "artifact-backlinks"
const (
	CodeArtifactBacklinkStale     = "artifact-backlink-stale"     // repairable
	CodeArtifactBacklinkMalformed = "artifact-backlink-malformed" // manual review
	CodeArtifactBacklinkShared    = "artifact-backlink-shared"    // manual review: one path, several changes
)

// internal/app
// checkCorpus gains: artifacts map[string][]byte — linked artifacts present at the pinned tip.
func artifactBacklinkFindings(snap domain.Snapshot, corpus checkCorpus) []reposetup.DerivedFinding
func canonicalArtifactBacklink(src []byte, c domain.Change, path string) (out []byte, hasBlock bool, err error)
```

- **`readCheckCorpus`:** after reading records, `buildCorpusSnapshot(sc.cfg, corpus.records)`; when it builds, collect `artifactPathsOf(c)` over all changes (deduplicated, sorted) and read them with **one** `ReadBlobs` on the same pinned source; keep `Found` ones. A read error is returned (-> `corpus-unreadable`). A failed build reads none.
- **`canonicalArtifactBacklink`:** `document.Parse` (error returned); no `docket:backlink` block -> `(src, false, nil)`; else `ReplaceBlock(backlinkBlockName, backlinkInterior(render.ArtifactBacklinkContent(c, path)))` and `Apply`. Refactor `retargetArtifactBacklinks` to call it: one renderer.
- **`artifactBacklinkFindings`**, paths sorted: linked by ≥2 changes -> `shared` (not repairable); absent from `artifacts` -> nothing (legacy, on the integration branch); malformed markers (`markerMalformed`) -> `malformed`; no block -> nothing (never inserted); canonical ≠ stored -> `stale`, repairable, message "the generated backlink differs from the canonical relative link to its change." `derivedViewFindings` appends these after `artifactLinkFindings`, reusing its one snapshot.

- [ ] **Step 1: Write the failing tests** over in-memory `checkCorpus` values (one change whose `spec:` is `docs/superpowers/specs/x.md`):
  - absolute: `<!-- docket:backlink:start (generated — do not hand-edit) -->\n> ↩ **[Change 0007 — T](https://github.com/o/r/blob/docket/docs/changes/active/0007-t.md)**\n<!-- docket:backlink:end -->\n\n# body\n` -> one repairable `stale` on the spec path;
  - canonical (`assembleSpecFile(render.ArtifactBacklinkContent(c, p), "# body")`) -> none; no block -> none; start marker only -> one `malformed`; two changes linking it -> one `shared`; an absolute **plan** path -> one `stale`.
  - `TestDerivedFindingLiftsBacklinkView`: the repairable finding lifts to a warning whose remedy names `docket repository repair`.
- [ ] **Step 2: Run and watch them fail.** `go test ./internal/app/ ./internal/reposetup/ -count=1 -run 'Backlink'`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run and confirm pass;** then `go test ./internal/app/ ./internal/reposetup/ -count=1`. Fixture specs are written by operations, so existing exact-finding tests stay green; if one does not, make its fixture write the canonical block and say so in the commit message.
- [ ] **Step 5: Mutation-test.** Drop the `artifactBacklinkFindings` call: the absolute case goes red. Restore.
- [ ] **Step 6: Commit.** `feat(repository): check reports metadata artifacts whose generated backlink is not the canonical relative link`

---

### Task 3: `repository repair` re-stamps artifact backlinks

**Files:**
- Modify: `internal/app/repository_repair.go` (+ `repository_repair_test.go`, `repository_repair_integration_test.go`)

**Interfaces:** `composeDerivedRepairBytes(sc setupContext, snap domain.Snapshot, corpus checkCorpus, recByPath map[string]corpusRecord, view reposetup.DerivedView, file string) ([]byte, error)`: the view is passed explicitly (`planRepositoryRepair` builds `viewByPath` from the repairable findings). New case: `DerivedViewArtifactBacklinks` -> the one change linking `file` (via `artifactPathsOf`), then `canonicalArtifactBacklink(corpus.artifacts[file], change, file)`.

- [ ] **Step 1: Write the failing tests.**
  - Unit: `planRepositoryRepair` over Task 2's absolute corpus lists the spec in `files`, `contents` = canonical bytes, body byte-identical.
  - `TestIntegrationRepoRepairRestampsAbsoluteSpecBacklink` (Review Focus 3): with the shard's existing fixture (read the top of `repository_repair_integration_test.go`), groom a change with a spec (`ChangeGroom`), then `writeDocketFileAndPush` the spec with its block pointing at `https://github.com/o/r/blob/docket/<record path>` and a prose line `see https://github.com/o/r/blob/docket/docs/adrs/README.md`. Preview: `confirmation-required`, `RepairedViews` has the spec, human text has `[artifact-backlink-stale] <spec>`. Authorized with `ExpectedSource`: the block equals `render.ArtifactBacklinkContent`, the prose line and every other byte are unchanged; a following `RunRepositoryCheck` has no `stale` finding.
- [ ] **Step 2: Run and watch them fail.** `go test ./internal/app/ -count=1 -run Repair` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoRepair' -count=1`.
- [ ] **Step 3: Implement.** **Step 4:** rerun, confirm pass.
- [ ] **Step 5: Real-tree check** (metadata-branch-invisible-to-suite). In a throwaway clone (`mktemp -d "${TMPDIR:-/tmp}/repair-probe.XXXXXX"`, clone, fetch `docket`), run the feature worktree's binary: `repository check --json` (count `artifact-backlink-stale`; reconcile counted 288 absolute spec backlinks, so expect about 287, one being prose-only) and `repository repair` **preview only**. Never push from it; delete it. Record both numbers.
- [ ] **Step 6: Mutation-test.** Make the new case return the stored bytes: the integration test goes red. Restore.
- [ ] **Step 7: Commit.** `feat(repository): repair re-stamps metadata artifact backlinks as relative links`

---

## Phase B: primitives

### Task 4: gitcli `CommitOwnPaths` and `RemoveRemote`

**Files:** `internal/gitcli/commit.go`, `internal/gitcli/remoteadmin.go` (+ real-git tests in the package style)

```go
// CommitOwnPaths commits exactly paths on dir's current branch with the one-line
// message subject and returns the new HEAD. Every other staged or unstaged change
// stays put (`git commit --only`). The repository's own hooks and signing apply,
// as for any commit a person makes. A path absent from HEAD but present in the
// working tree is first registered with `git add --intent-to-add`. Detached HEAD
// is invalid-request; a hook or other failure is command-failed with a stderr excerpt.
func (c *Client) CommitOwnPaths(ctx context.Context, dir string, paths []RepoPath, subject string) (ObjectID, error)

// RemoveRemote deletes a remote and its tracking refs (`git remote remove`);
// an unconfigured remote is remote-unavailable.
func (c *Client) RemoveRemote(ctx context.Context, repo Repository, name RemoteName) error
```

`CommitOwnPaths`: validate (`dir` absolute; `validateRepoPath(p, false)`; one-line non-empty subject); refuse detached HEAD (`symbolic-ref --quiet HEAD`); `ls-files --error-unmatch -- <p>` per path and `add --intent-to-add --pathspec-from-file=- --pathspec-file-nul` for untracked present ones; `commit --only -m <subject> --pathspec-from-file=- --pathspec-file-nul` with **no** `-c core.hooksPath`, **no** `--no-verify`, **no** signing override; return `rev-parse HEAD`.

- [ ] **Step 1: Failing tests.**
  - `TestCommitOwnPathsLeavesOtherStagedChanges`: stage `other.txt`; modify tracked `a.txt`, create `new.yml`, delete tracked `gone.txt`; commit those three with `Add docket configurations to repository` -> `git show --name-status --format=%B HEAD` lists exactly them, message = subject + newline; `git diff --cached --name-only` still lists `other.txt`.
  - `TestCommitOwnPathsRunsRepositoryHooks`: a `.git/hooks/commit-msg` that touches a marker and exits 1 -> command-failed, marker exists, HEAD unchanged, edits remain.
  - `TestCommitOwnPathsRefusesDetachedHead`; `TestRemoveRemote` (remove, `RemoteURL` remote-unavailable, second remove remote-unavailable).
- [ ] **Step 2:** `go test ./internal/gitcli/ -count=1 -run 'CommitOwnPaths|RemoveRemote'` fails. **Step 3:** implement. **Step 4:** pass, then the whole package.
- [ ] **Step 5: Mutation-test.** Drop `--only`: `...LeavesOtherStagedChanges` goes red. Restore.
- [ ] **Step 6: Commit.** `feat(gitcli): commit only named paths with the repository's own hooks, and remove a remote`

---

### Task 5: Live-run and live-gate probes

**Files:** `internal/gatedrive/worktree_lock.go` (+ test); create `internal/app/runtracker_live.go` (+ test)

```go
// internal/gatedrive: try every <stateDir>/worktree-locks/*/busy.lock without blocking
// (process.TryExclusiveLock, closed at once); return the dirs a live process holds,
// sorted. Missing root -> (nil, nil); a dir without busy.lock is skipped; other errors returned.
func BusyWorktreeLocks(stateDir string) ([]string, error)

// internal/app
type liveRunLocator struct{ Key, State, ChangeID, Remedy string }
// liveRunsUnder scans <stateDir>/<runTrackerDirName>/*/run.json via readStoredRun.
// Live = RunActive, RunCompleting, RunCancelling, or any unknown state; RunCompleted,
// RunCancelled, RunSuperseded are not. An unreadable record is State "unreadable"
// (liveness unknown is never "not live"). Missing root -> (nil, nil).
func liveRunsUnder(stateDir string) ([]liveRunLocator, error)
```

Remedies: active/completing -> `docket run cancel --key <key> --reason <why>`; cancelling -> "re-run the same `docket run cancel --key <key> --reason <why>` until it reports cancelled"; unreadable -> "inspect or remove <dir> by hand". Use the state constants, never their strings.

- [ ] **Step 1: Failing tests.** gatedrive: one held lock and one free -> exactly the held dir; empty root -> nil. app: `storedRun`-envelope fixtures for active, completing, cancelling, completed, cancelled, superseded, plus an undecodable file -> exactly four locators (active, completing, cancelling, unreadable) with their remedies.
- [ ] **Step 2:** `go test ./internal/gatedrive/ ./internal/app/ -count=1 -run 'BusyWorktreeLocks|LiveRuns'` fails. **Step 3:** implement. **Step 4:** pass.
- [ ] **Step 5: Mutation-test.** Skip unreadable records: the unreadable assertion goes red. Restore.
- [ ] **Step 6: Commit.** `feat(app): enumerate live runs and live gate locks for a repository-wide precondition`

---

### Task 6: Behavior-neutral extractions for the private layout and metadata remote

**Files:** `internal/layout/layout.go` (+ test); `internal/app/layout_resolve.go`; `internal/app/repository_init_private.go`

```go
// internal/layout
const PrivateLocalKeysFile = "local-keys.yml" // <common>/dckt/local-keys.yml: the saved .docket.local.yml
func PrivateLocalKeysPath(commonDir string) string

// internal/app
// privateLayoutOf is resolveLayout's private branch (store from origin's URL and
// the data home), whatever the current mode.
func privateLayoutOf(ctx context.Context, r remoteURLReader, repo gitcli.Repository) (layout.Layout, error)

// metadataRemoteError: Conflict = a refusal a human resolves (foreign store, a dckt
// remote pointing elsewhere); otherwise an external failure at Stage.
type metadataRemoteError struct {
	Conflict bool
	Stage    string
	Err      error
}
func (e *metadataRemoteError) Error() string

// ensurePrivateMetadataRemote is runPrivateInit's steps 4-5: flagURL as given; else
// the configured dckt remote when reuseConfigured; else the default store (created
// when absent, with its origin record); then add the dckt git remote or verify it.
func ensurePrivateMetadataRemote(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, lay layout.Layout, flagURL string, reuseConfigured bool) (url string, changed bool, err error)
```

`resolveLayout` calls `privateLayoutOf`. `runPrivateInit` calls `ensurePrivateMetadataRemote(ctx, d.Git, sc.repo, lay, o.MetadataRemote, sc.layout.Mode == layout.Private)` and maps `Conflict` -> `fail(initRefusal(reposetup.StateConflict, e.Err.Error()))`, else `fail(repositoryExternalFailure(OperationRepositoryInit, cls.State, e.Stage, e.Err))`. Every stage string and refusal text stays byte-identical.

- [ ] **Step 1:** record green: `go test ./internal/app/ ./internal/layout/ -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoSetup' -count=1`.
- [ ] **Step 2:** add `TestPrivateLocalKeysPath`; watch it fail. **Step 3:** extract. **Step 4:** Step 1's commands and the new test are green with no test changed.
- [ ] **Step 5: Commit.** `refactor(app): extract the private layout and metadata-remote setup for reuse`

---

## Phase C: the switch

### Task 7: Integration-branch commit machinery

**Files:** create `internal/app/repository_set_visibility_commit.go` (+ `_test.go`, default tag, real git via `runGit`/`newGitClient` as in `status_git_test.go`)

```go
const (
	visibilityRemoveSubject = "Remove docket configurations from repository"
	visibilityAddSubject    = "Add docket configurations to repository"
)
var visibilityCommitPaths = []string{".docket.yml", ".gitignore", "AGENTS.md", "CLAUDE.md", ".cursor/rules/docket-dispatch.mdc"}
const switchJournalFile = "visibility-switch.json" // in layout.StateDirOf(common): travels with a rename

// switchJournal: path -> digest of what the switch wrote ("deleted" for a removal).
type switchJournal struct {
	Subject string            `json:"subject"`
	Paths   map[string]string `json:"paths"`
}

// pathDigest: "sha256:<hex>" of a regular file, "link:<target>" of a symlink,
// "deleted" when absent; anything else errors. headPathDigest: the same for rel in
// commit (mode 120000 -> "link:<blob>").
func pathDigest(abs string) (string, error)
func headPathDigest(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commit gitcli.ObjectID, rel string) (string, error)

func loadSwitchJournal(commonDir string) (switchJournal, bool, error)
func saveSwitchJournal(commonDir string, j switchJournal) error // temp file beside it + rename
func clearSwitchJournal(commonDir string) error

// writeSwitchPath journals the digest of content (nil = delete) BEFORE writing
// <primary>/<rel> atomically (or removing it); subject must match an existing journal's.
func writeSwitchPath(commonDir, primary, subject, rel string, content []byte) error
// journalSwitchPath records rel's current on-disk digest (after install wrote a surface).
func journalSwitchPath(commonDir, primary, subject, rel string) error

// commitPathConflicts: one refusal line per path in ChangedPaths whose working-tree
// digest is not its journal digest (or is unjournaled), plus a detached HEAD.
// A probe error is returned.
func commitPathConflicts(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commonDir string, paths []string) ([]string, error)

type visibilityCommitOutcome struct {
	Commit   string   // "" when nothing was committed
	Paths    []string
	Complete bool     // HEAD holds every journaled path
}
// commitSwitchJournal commits the journaled paths whose HEAD digest differs from the
// journal, with its subject, via CommitOwnPaths, then clears the journal. When HEAD
// already holds them all (death between commit and cleanup) it commits nothing and
// clears. A commit failure leaves journal and edits in place.
func commitSwitchJournal(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, commonDir string) (visibilityCommitOutcome, error)
```

- [ ] **Step 1: Failing tests.**
  - `TestVisibilitySubjectsCarryNoModeVocabulary`, keyed on the constants' **values**: none of `shared`, `public`, `private`, `visibility` (any case); one line each; exactly two.
  - `TestCommitPathConflicts`: clean -> none; user edit -> names it; `writeSwitchPath` write -> none; then hand-edited -> refusal; detached HEAD -> refusal.
  - `TestCommitSwitchJournalCommitsOnlyJournaledPaths`: staged `other.txt`; journaled write of `.docket.yml` and delete of tracked `.gitignore` -> exact subject, exactly those two paths, `other.txt` still staged, journal gone.
  - `TestCommitSwitchJournalIsIdempotentAfterCrash`: re-saving the journal after a commit -> next call commits nothing, `Complete`, clears.
  - `TestCommitSwitchJournalRetriesAfterHookFailure`: a once-failing `commit-msg` hook -> error with journal and edits kept; the retry commits once.
- [ ] **Step 2:** `go test ./internal/app/ -count=1 -run 'VisibilitySubjects|CommitPathConflicts|CommitSwitchJournal'` fails. **Step 3:** implement. **Step 4:** pass.
- [ ] **Step 5: Mutation-test.** Append ` (visibility)` to `visibilityAddSubject` -> vocabulary test red; accept every dirty path in `commitPathConflicts` -> hand-edit case red. Restore each.
- [ ] **Step 6: Commit.** `feat(app): journaled, own-path integration-branch commits with fixed subjects`

---

### Task 8: `set-visibility` service core, preview, and CLI

**Files:**
- Create: `internal/app/repository_set_visibility.go` (options, result, router, preview), `internal/app/repository_set_visibility_probe.go` (probe, preconditions, warnings), `internal/app/repository_set_visibility_test.go`, `internal/app/repository_set_visibility_integration_test.go` (prefix `TestIntegrationRepoVisibility`)
- Create: `tests/test_go_integration_app_repovisibility.sh` (copy the reporepair shard; `SHARD_PREFIX="TestIntegrationRepoVisibility"`; its own header); `tests/runtime-budgets.tsv` row `60 parallel` (Task 11 measures)
- Modify: `internal/app/repository_facts.go` (`setupHooks.afterVisibilityPhase func(phase string) error`; `SetupDeps.GitHub` comment names set-visibility), `schema_registry.go`, `internal/cli/repository.go`, `internal/cli/install.go`, `internal/cli/repository_test.go`, any golden `grep -rn '"repository.repair"' internal cmd --include='*_test.go'` shows

```go
const OperationRepositorySetVisibility = "repository.set-visibility"

type SetVisibilityOptions struct {
	Target             string // "shared" | "private"
	Authorized         bool
	ExpectedSource     string // the preview's pin ("" on the preview pass)
	MetadataRemote     string // going private only
	DeleteSharedBranch bool   // going private only
	RemoveSharedFiles  bool   // going private only
}
type VisibilityPhase struct{ Name, Status, Detail string } // done|pending|applied|kept|skipped; snake_case JSON tags
type VisibilityCommit struct {                              // status planned|committed|failed
	Subject, Branch string
	Paths           []string
	Commit, Status  string
}
type RepositorySetVisibilityResult struct {
	Envelope
	RepositoryState string              `json:"repository_state"` // confirmation-required, or the mode after the run
	Current         string              `json:"current"`
	Target          string              `json:"target"`
	SourceRevision  string              `json:"source_revision"` // the composite pin
	Phases          []VisibilityPhase   `json:"phases"`
	Commits         []VisibilityCommit  `json:"commits,omitempty"`
	Warnings        []string            `json:"warnings,omitempty"`
	PendingLocal    []string            `json:"pending_local,omitempty"`
	Lessons         string              `json:"lessons,omitempty"`
	BackupRemote    string              `json:"backup_remote,omitempty"`
	Findings        []reposetup.Finding `json:"findings,omitempty"`
	human           string
}
func (r RepositorySetVisibilityResult) HumanText() string
func (r RepositorySetVisibilityResult) SourceRev() string
func (r RepositorySetVisibilityResult) ConfirmationRequired() bool
func RunRepositorySetVisibility(ctx context.Context, d SetupDeps, o SetVisibilityOptions) RepositorySetVisibilityResult

// visibilityState is read without trusting the detected mode: both layouts
// (private via privateLayoutOf), layout.Detect, whether <common>/docket and
// <common>/dckt are dirs, origin's docket ref, the dckt remote URL ("" =
// unconfigured) and its dckt ref, origin's default tip, the worktree list, local
// docket/dckt tips ("" = absent), whether .docket.local.yml exists, a config.yml
// waiting in either state folder, and the journal.
type visibilityState struct {
	sc                               setupContext
	facts                            reposetup.Facts
	common, primary                  string
	shared, private                  layout.Layout
	current                          layout.Mode
	sharedStateDir, privateStateDir  bool
	originDocket, bareDckt           gitcli.RemoteRef
	dcktURL, defaultTip              string
	worktrees                        []gitcli.WorktreeInfo
	localDocket, localDckt           string
	localConfig                      bool
	pendingConfig                    string
	journal                          switchJournal
	journalPresent                   bool
}
func probeVisibility(ctx context.Context, d SetupDeps, sc setupContext, facts reposetup.Facts) (visibilityState, error)

type visibilityStep struct {
	name    string
	done    bool   // read from state
	detail  string // preview line
	commits bool   // makes an integration-branch commit
	run     func(ctx context.Context, x *visibilityRun) error
}
type visibilityRun struct {
	d   SetupDeps
	o   SetVisibilityOptions
	st  visibilityState
	res *RepositorySetVisibilityResult
}
func planToPrivate(st visibilityState, o SetVisibilityOptions) []visibilityStep // executors: Task 9
func planToShared(st visibilityState, o SetVisibilityOptions) []visibilityStep  // executors: Task 10
func runVisibilitySteps(ctx context.Context, x *visibilityRun, steps []visibilityStep) error
```

**`RunRepositorySetVisibility`, in order:**
1. **Input** (before any read): a bad `Target`, or a going-private flag with `shared` -> `invalid-input` ("--metadata-remote, --delete-shared-branch and --remove-shared-files apply only when switching to private").
2. **Gather** `GatherSetupFacts(ctx, d, true)`; errors via `migrateGatherFailure`, re-stamped as `repairFromMigrateResult` does.
3. **Set up?** `invalid-state` on a live surface ("run `docket repository migrate` first"), on no metadata branch on origin **or** a configured `dckt` remote ("run `docket repository init` first"), or on an unknown required probe (named, with `docket repository check`).
4. **Debris** `sweepSetupDebris`; **probe** `probeVisibility` (error -> `external-failed`).
5. **Live runs:** `liveRunsUnder` + `gatedrive.BusyWorktreeLocks` over **both** existing state folders; each hit is a refusal line with its remedy (a lock names its `holder.json` when readable).
6. **Metadata sync:** the worktree at either layout's `MetadataWorktree` is clean with no unfinished operation (`worktreeCleanState`; else "commit or discard the changes in <path>, then re-run"), and its branch tip is ancestor-or-equal of the authoritative tip (`docket`: origin's, else the bare `dckt`; `dckt`: the bare `dckt`; else "publish them with `docket repository prepare`, or resolve them by hand"). No registered worktree passes.
7. **Plan**; if a pending step `commits`, `commitPathConflicts` over `visibilityCommitPaths` (lines refuse: "commit or set aside these edits, then re-run").
8. **Pin** `"origin-docket=<oid|absent> dckt=<oid|absent> origin-default=<oid>"`; **warnings**.
9. Nothing pending -> `no-op` ("already <mode>; nothing to do"). Unauthorized -> `invalid-state`, `confirmation-required`, preview + "confirmation required: re-run with --yes to authorize this switch".
10. Authorized: `migrateSourceMoved(o.ExpectedSource, pin)` -> `contended` ("the repository moved since the preview; re-run to preview the new state"); else `runVisibilitySteps` runs each pending step, records it `applied`, then `fire(afterVisibilityPhase(name))`. An error stops (`external-failed` or the step's refusal) keeping the phases so far; a nil `run` is internal-error "phase <name> has no executor". Success -> `applied`, state = `layout.Detect` afterwards.

**`visibilityWarnings`:**
- Going private: "teammates' later writes to `<shared.MetadataRemote>/<shared.MetadataBranch>` will not be seen by this clone" (from the layout); with `--delete-shared-branch`, "existing pull-request backlinks into that branch will stop resolving".
- Both directions: `absoluteMetadataLinks(webURL, branch string, files map[string][]byte) []string` (pure, sorted) matches `webURL+"/blob/"+branch+"/"` and `webURL+"/tree/"+branch+"/"`, `branch = layout.SharedName`, over every `.md` blob at the metadata tip (origin `docket`, else bare `dckt`; `ListTree(ctx, nil)` + one `ReadBlobs`). `webURL = githubWebURL(origin URL)`; empty skips. Hits -> "N metadata file(s) still carry absolute links into the docket branch; run `docket repository repair` to make the generated ones relative: <≤20 paths, then "and K more">". A read error -> "metadata files were not checked for absolute links: <err>". Never a refusal.
- Going private only: `listDocketTextPRs(ctx, gh RepairGitHub, dir string, changes []domain.Change) ([]string, error)`: changes with `pr:` and status `in-progress`/`implemented`, numbers via `parsePRNumber`, one `ViewPullRequestsBatch`, keep `OPEN` bodies containing `<!-- docket:` -> "open pull requests whose descriptions carry docket text (listed, not edited): <urls>". Nil `d.GitHub` or an error -> "open pull requests were not checked: <reason>".

**Preview** (`visibilityPreviewText`): header `docket repository set-visibility <target> — plan`, then `repository`, `current`, `target`, `pinned`; `phases:` one `[done]`/`[pending] <name>: <detail>` line each; a `commit:` block (`branch`, `message` = exact subject, `paths` with write/delete) when a pending step commits; `warnings:` bullets.

**CLI** `newRepositorySetVisibilityCommand(setResult)`, mirroring `newRepositoryRepairCommand`: `Use: "set-visibility <shared|private>"`, `cobra.ExactArgs(1)`, `capability("repository.set-visibility", EffectLocalWrite, EffectMetadataWrite, EffectExternalWrite)`, Short "Move an existing repository between shared and private visibility, keeping every record". `Long`: preview first; `--yes` applies; a re-run resumes; live runs refuse; going private keeps origin's branch and the committed files unless flagged; going shared publishes to origin and keeps the bare remote as a backup; commits are local, never pushed. Flags `--repo-dir`, `--yes`, `--metadata-remote` ("going private: push the dckt branch to this git `url` instead of the default bare repository"), `--delete-shared-branch` ("going private: delete origin's docket branch once the bare remote holds the same tip"), `--remove-shared-files` ("going private: remove .docket.yml, the .gitignore block, and the dispatch instructions in one local commit"). Seams `repositorySetVisibilityRunner` and `repositorySetVisibilityGitHub = repositoryRepairGitHub` (an error leaves `deps.GitHub` nil).

- [ ] **Step 1: Failing unit tests:** `TestSetVisibilityInputValidation` (`public`, and each going-private flag with `shared`, give `invalid-input` against a non-existent `RepoDir`); `TestAbsoluteMetadataLinks` (`blob/docket/` and `tree/docket/` match; `blob/main/`, another host, empty `webURL` do not); `TestListDocketTextPRs` with the repair tests' fake (open + marker listed; merged or markerless not); `TestVisibilityPreviewTextNamesEveryPendingPhase`; `TestSetVisibilityNilExecutorIsInternalError` (via `runVisibilitySteps`); CLI: command, `ExactArgs(1)`, flags -> options, `assetIndependent`, three effects.
- [ ] **Step 2: Failing integration tests** (shared fixture `newInitRepo(t, defaultSetupYML, nil)` + `runInit`; private `newPrivateInitRepo` + `runInitWith`; driver `(r *initRepo) runSetVisibility(t, o SetVisibilityOptions) RepositorySetVisibilityResult` pinning roots with `newGitClient(t)`):
  - `...PreviewWritesNothing`: a `private` preview lists the phases; `git status --porcelain`, origin/store refs, and the `.git` listing are byte-identical before and after.
  - `...RefusesLiveRun` (`MintRunTrackerRecord` + `MintRunRecord` as in `runrecord_integration_test.go`; text has `docket run cancel --key <key>`); `...RefusesBusyGateLock` (held `<state>/worktree-locks/<x>/busy.lock`); `...RefusesDirtyMetadataWorktree`; `...RefusesUnpublishedMetadataCommit`; `...RefusesFreshAndLegacy` (names `init`; with `repomigration_integration_test.go`'s live surface, `migrate`).
  - `...WarnsAbsoluteLinksWithoutRefusing` (Review Focus 3): a pushed spec whose prose holds `https://github.com/acme/app/blob/docket/x.md`, origin made GitHub-shaped (an `insteadOf` rewrite, or a helper from `grep -rn 'github.com/' internal/app/*integration_test.go`): the preview lists the file and `docket repository repair` and stays a `confirmation-required` preview.
- [ ] **Step 3:** run `go test ./internal/app/ ./internal/cli/ -count=1 -run 'SetVisibility|AbsoluteMetadataLinks|DocketTextPRs|VisibilityPreview'` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoVisibility' -count=1`; they fail.
- [ ] **Step 4: Implement.** `planToPrivate`/`planToShared` return each step with its `done` predicate and `commits` flag and a **nil `run`**; preview and refusals are complete now. Green because no test here authorizes a run except the nil-executor test (intermediate-task-state-buildable).
- [ ] **Step 5:** pass; also `go test ./internal/repoguard/ ./internal/cli/ -count=1` and `bash tests/test_go_integration_contract.sh`.
- [ ] **Step 6: Mutation-test.** Skip the live-run scan: `...RefusesLiveRun` red. Restore.
- [ ] **Step 7: Commit.** `feat(repository): set-visibility previews the switch and refuses live runs, dirty metadata, and unset repositories`

---

### Task 9: Going private

**Files:** create `internal/app/repository_set_visibility_private.go`; modify `repository_set_visibility.go`, `repository_set_visibility_integration_test.go`, `internal/repoguard/integration_push_test.go` (allowlist)

```go
// publishVisibilityHistory makes remote's ref hold tip without rewriting history:
// create-only when absent; a fast-forward lease (expected = current remote tip) when
// that tip is an ancestor of tip; nothing when it equals or descends from tip.
// Unrelated or diverged history refuses naming both tips. Re-read afterwards: the
// remote must hold tip or a descendant.
func publishVisibilityHistory(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, remote gitcli.RemoteName, ref gitcli.RefName, tip gitcli.ObjectID) (changed bool, err error)

// stripDispatchBlock removes the `dispatch` managed block; remove = only whitespace
// is left. Malformed markers error (manual review); no block -> (src, false, nil).
func stripDispatchBlock(src []byte) (out []byte, remove bool, err error)

// disownWorkingTreeSurfaces rewrites the ownership record without surfaces outside
// .git/, so a private-mode install never retires them. Absent record -> (false, nil).
func disownWorkingTreeSurfaces(recordPath string) (bool, error)
```

**`planToPrivate` steps** (name — done when — run). Re-probe a remote tip before acting on it; never reuse the gather-time value.
1. **`metadata-remote`** — `dcktURL` equals the wanted URL (`absMetadataRemote(--metadata-remote)`, else the configured URL, else `private.DefaultBareRemote`) — `ensurePrivateMetadataRemote(ctx, git, repo, st.private, o.MetadataRemote, st.dcktURL != "")`; `Conflict` -> `invalid-state` with its text.
2. **`publish`** — the bare `dckt` equals or descends from the source tip (origin's `docket` when present, fetched; else the bare tip) — `publishVisibilityHistory(..., metadataRemote(st.private), metadataRef(st.private), sourceTip)`. Neither ref present -> refuse "no metadata history is reachable: origin has no docket branch and <url> holds no dckt branch; re-run with --metadata-remote <url of the bare repository that holds it>".
3. **`config`** — `.docket.local.yml` absent and a `config.yml` exists in either state folder — when no pending `config.yml`: committed = `.docket.yml` at `st.defaultTip` (`readCommitBlob`), else at the primary's `HEAD`, else nil (detail names the source); local = `.docket.local.yml` or nil; `FoldPrivateConfig(committed, local, config.RepoOnlyPaths())`; validate via `config.Resolve([]config.Source{{Layer: config.LayerRepository, Name: layout.PrivateConfigDisplay, Data: folded}}, config.ResolveContext{DefaultBranch: st.sc.defaultBranch})` (error refuses before any write); `MkdirAll(st.shared.StateDir)`; write `local` verbatim to `<shared.StateDir>/local-keys.yml` when non-nil; write `folded` to `<shared.StateDir>/config.yml` (temp beside + rename); remove `.docket.local.yml`. With a pending `config.yml`, only remove `.docket.local.yml`. `dropped` -> warning "not carried into the private config: <paths>".
4. **`state-folder`** — `privateStateDir && !sharedStateDir` — if `<common>/dckt` exists beside `<common>/docket`, refuse "<common>/dckt already exists beside <common>/docket; inspect both by hand" (the switch never writes into `dckt` before the rename); else `os.Rename(st.shared.StateDir, st.private.StateDir)`. **From here the repository detects private.**
5. **`ignore`** — `EnsureExcludeBlock` on `.git/info/exclude` reports unchanged — `ensureExcludeFile(filepath.Join(common, "info", "exclude"))`.
6. **`metadata-worktree`** — registered at `private.MetadataWorktree` on `metadataRef(private)` with hooks off (`WorktreeHooksDisabled`), none at `shared.MetadataWorktree`, no local `docket` — `RemoveWorktreeClean(shared.MetadataWorktree)` when registered; `MkdirAll(private.CheckoutsDir)`; `removeStaleOwnCheckout(private)`; `ensureMetadataWorktree(..., private.MetadataWorktree, metadataRef(private), bareTip)`; `DisableWorktreeHooks`; `DeleteLocalBranchChecked(docket, its tip)` only when ancestor-or-equal of the bare tip, else `kept` + pending "the local docket branch has commits the dckt branch lacks; inspect it, then delete it by hand".
7. **`instructions`** — the record at `reposeed.RecordPath(common, layout.PrivateName)` lists no working-tree surface, and (when `agent_harnesses` is authorized) a read-only `ResolveRepoPhase` + `install.InspectTarget` is `DispositionNoop` for every target — `disownWorkingTreeSurfaces`, then `installAuthorizedSurfaces(ctx, git, primary)`.
8. **`delete-shared-branch`** (flag) — origin has no `docket` — re-read both tips fresh (`FetchBranch` origin `docket`, `ProbeRemoteBranch` `dckt`); differ -> `kept` + pending "origin's docket branch is at <a>, the dckt branch at <b>; nothing was deleted" (not an error); else `DeleteRemoteRefLease(ctx, repo, originRemote, metadataRef(st.shared), tip)`, `PushLeaseLost` -> `kept` the same way.
9. **`remove-shared-files`** (flag; `commits`) — `HEAD` has no `.docket.yml`, no `.gitignore` block, no `dispatch` block in `AGENTS.md`/`CLAUDE.md`, no cursor rule, and no journal — targets from each path's `HEAD` blob via `writeSwitchPath(..., visibilityRemoveSubject, rel, content)`: `.docket.yml` delete; `.gitignore` `RemoveGitignoreBlock` (delete when empty); `AGENTS.md`/`CLAUDE.md` regular files `stripDispatchBlock`; a `CLAUDE.md` symlink deleted only when `AGENTS.md` is; the cursor rule deleted. Then `commitSwitchJournal`: failure -> `external-failed`, "the edits are in the working tree and journaled; re-run `docket repository set-visibility private --remove-shared-files` to retry the commit", commit row `failed`; success -> row `committed`, branch from `WorktreeCheckoutState`.
10. **`align-visibility`** — `.git/dckt/config.yml` sets no different explicit `visibility` — `RenderVisibilityEdit(existing, "private")`, written atomically.

- [ ] **Step 1: Failing integration tests** (`-run '^TestIntegrationRepoVisibilityPrivate'`):
  - `...PrivateHappyPath`: shared fixture, `.docket.yml` = `defaultSetupYML + "agent_harnesses: [codex]\nbuild:\n  test_command: make\n"`, `.docket.local.yml` = `reclaim:\n  auto: true\n`, one `ChangeCreate` via `planningDepsFor`; preview, then authorize with `SourceRev()`. Assert: bare `dckt` tip = old origin `docket` tip; origin keeps `docket`; only `.git/dckt/`; its `config.yml` resolves `visibility: private`, `build.test_command: make`, `reclaim.auto: true`; `local-keys.yml` = the old local bytes; no `.docket.local.yml`; valid exclude block; no `.docket`; the private checkout on `dckt`; no local `docket`; `Instructions(primary, "dispatch")` returns the block; committed `AGENTS.md` unchanged; `git status --porcelain` empty; a re-preview is `no-op`; `RunRepositoryPrepare` is no-op with `MetadataRemote == "dckt"`.
  - `...PrivateDeleteSharedBranch`: origin loses `docket`; a re-run is `no-op`.
  - `...PrivateDeleteSharedBranchKeepsOnMismatch`: `afterVisibilityPhase("publish")` runs `advanceRemoteDocketChild` -> applied, origin stays advanced, the phase is `kept`, `PendingLocal` names both tips.
  - `...PrivateRemoveSharedFiles` (staged `notes.txt`): one new `main` commit, `%B` = subject + newline, `--name-status` = `.docket.yml` D, `.gitignore` M/D, `AGENTS.md` M/D; `notes.txt` still staged; origin `main` unchanged. `...PrivateRemoveSharedFilesLater`: no flag, then the flag -> the same.
  - `...PrivateRefusesForeignBareRemote`: an unrelated `dckt` pre-pushed at the default store -> refusal naming both tips; `.git/docket/` unchanged.
  - `...PrivateMetadataRemoteFlag`: `--metadata-remote <tmp>/backup.git` -> that URL; no default `remote.git`.
  - `...PrivateSecondCloneAfterCleanup` (Review Focus 5): B cloned while shared; A switches with both flags and the test pushes A's commit to `main`; B (same `XDG_DATA_HOME`) switches -> applied, `publish` was `done`, config detail names `HEAD`, B's checkout holds A's change.
- [ ] **Step 2:** watch them fail. **Step 3: Implement** the executors, helpers, and allowlist entry, with unit tests for `stripDispatchBlock` (middle block; block-only -> `remove`; malformed -> error; none -> unchanged) and `disownWorkingTreeSurfaces` (`AGENTS.md` + `.git/dckt/AGENTS.md` -> keeps the latter; absent -> `false, nil`).
- [ ] **Step 4:** pass, plus `go test ./internal/app/ ./internal/repoguard/ -count=1`.
- [ ] **Step 5: Mutation-test.** Treat diverged history as done in `publishVisibilityHistory` -> `...RefusesForeignBareRemote` red; skip the tip comparison before deletion -> `...KeepsOnMismatch` red. Restore each.
- [ ] **Step 6: Commit.** `feat(repository): set-visibility private moves the metadata history to a local bare remote`

---

### Task 10: Going shared

**Files:** create `internal/app/repository_set_visibility_shared.go`; modify `repository_set_visibility.go`, `repository_set_visibility_integration_test.go`

Produces `var repositoryIdentityKeys = []string{"integration_branch", "changes_dir", "adrs_dir", "results_dir"}` (the spec's four; `github_project`/`terminal_publish` are repo-only but inert, so they do not gate). The private config source is `pendingConfig` (`<common>/dckt/config.yml` before the rename, `<common>/docket/config.yml` after); saved keys are `local-keys.yml` beside it.

**`planToShared` steps:**
1. **`identity-keys`** (`commits` when pending) — the private config sets none of the keys, or `ConfigLeafValues` of origin's default-branch `.docket.yml` (fetched fresh) equals its values for every key it sets — when `HEAD` does not hold the split's committed half: `SplitPrivateConfig`, validate as `.docket.yml`, `writeSwitchPath(..., visibilityAddSubject, ".docket.yml", committed)`, `commitSwitchJournal`, then **stop**: `applied`, state `private`, `PendingLocal` "get this commit onto <default branch> on origin (push it, or merge it through a pull request), then re-run `docket repository set-visibility shared`". When `HEAD` already holds it: `invalid-state` with the same remedy.
2. **`publish`** — origin's `docket` equals or descends from the bare tip — `publishVisibilityHistory(..., originRemote, metadataRef(st.shared), bareTip)`; diverged/unrelated -> "origin already has an unrelated docket branch (<a>) and this repository's history is at <b>; two backlogs are never merged; reconcile or remove origin's branch by hand".
3. **`instructions`** — `.git/dckt/AGENTS.md` absent and the record lists nothing under `.git/` — non-nil `SelectInstructionsSection(content, InstructionsSectionLessons)` goes to `res.Lessons` with "promoted lessons from the private instructions file — move what you want into the committed AGENTS.md:"; remove the file; rewrite the record without it (delete the record when empty).
4. **`config-committed`** — working-tree `.docket.yml` equals the split's committed half, or no private source remains — split, validate, `writeSwitchPath(..., ".docket.yml", committed)` (never read while private).
5. **`state-folder`** — `sharedStateDir && !privateStateDir` — non-empty `<common>/docket` refuses naming it; else `os.Rename(st.private.StateDir, st.shared.StateDir)`. **Now shared; `.docket.yml` resolves.**
6. **`config-local`** — no `config.yml` in the shared state folder — non-nil local half: validate `[committed LayerRepository, local LayerRepositoryLocal]` via `config.Resolve`, write `.docket.local.yml` atomically (ignored, never committed); remove `local-keys.yml`, then `config.yml` **last**.
7. **`ignore`** — no `# dckt:` block in the exclude file and `ValidGitignoreBlock` on the working-tree `.gitignore` — `RemoveExcludeBlock` (write when changed); `EnsureGitignoreBlock` through `writeSwitchPath(..., ".gitignore", out)`.
8. **`metadata-worktree`** — `.docket` on `metadataRef(shared)` with hooks off, nothing at `private.MetadataWorktree`, no local `dckt`, no `dckt` remote — `RemoveWorktreeClean(private.MetadataWorktree)`; `ensureMetadataWorktree(..., shared.MetadataWorktree, metadataRef(shared), originTip)`; `DisableWorktreeHooks`; delete local `dckt` (checked; `kept` when it holds commits origin's `docket` lacks); `RemoveRemote(dckt)`; `res.BackupRemote` = its URL.
9. **`commit`** (`commits`) — journal absent, `HEAD` holds the `.gitignore` block, no `visibilityCommitPaths` entry in `ChangedPaths`, and (decidable only once shared) every planned surface `DispositionNoop` on disk; pending before the flip — `installAuthorizedSurfaces`; `journalSwitchPath(..., visibilityAddSubject, rel)` per returned path; `commitSwitchJournal` (failure handling as Task 9).
10. **`backup`** — always done; detail "the bare repository <url> is kept as a backup".
11. **`align-visibility`** — `.docket.local.yml` sets no different explicit `visibility` — `RenderVisibilityEdit(existing, "shared")`.

- [ ] **Step 1: Failing integration tests** (`-run '^TestIntegrationRepoVisibilityShared'`):
  - `...SharedFromBornPrivate`: `newPrivateInitRepo` + `runInitWith` (codex), a change, a lessons paragraph outside the block of `.git/dckt/AGENTS.md`; `--yes` -> origin `docket` = bare tip; only `.git/docket/`; `.docket` on `docket`; no `dckt` remote or branch; the store kept and in `BackupRemote`; one commit `Add docket configurations to repository` with `.docket.yml` (`visibility: shared`), `.gitignore` (valid block), `AGENTS.md` (block); `Lessons` has the paragraph; `Instructions` reports not private; origin `main` unchanged.
  - `...SharedIdentityKeysStopThenResume`: private config sets `integration_branch: main` -> run 1 applied, one `.docket.yml` commit, push remedy, no origin `docket`; run 2 `invalid-state`; the test pushes `main`; run 3 completes, `.docket.yml` not recommitted, last commit = `.gitignore` + `AGENTS.md`.
  - `...SharedFastForwardsLeftBehindBranch` (Review Focus 4): shared -> private (no flags) -> `ChangeCreate` -> shared: origin `docket` fast-forwards.
  - `...SharedRefusesUnrelatedOriginBranch`: an unrelated root as origin `docket` -> refusal with "two backlogs are never merged"; nothing renamed.
  - `...SharedRestoresLocalKeys`: local `reclaim.auto: true` survives shared -> private -> shared in `.docket.local.yml` (no `visibility`); `.docket.yml` has no `reclaim`.
- [ ] **Step 2:** fail. **Step 3:** implement. **Step 4:** pass; re-run the `...Private` tests.
- [ ] **Step 5: Mutation-test.** Always-done identity gate -> `...StopThenResume` red; write `.docket.local.yml` before the rename -> `...RestoresLocalKeys` refuses on the conflicting local config, red. Restore each.
- [ ] **Step 6: Commit.** `feat(repository): set-visibility shared publishes the metadata history to origin and restores the shared layout`

---

### Task 11: Acceptance: round trip, interruption, and the private result

**Files:** `internal/app/repository_set_visibility_integration_test.go`; maybe `tests/runtime-budgets.tsv`

- [ ] **Step 1: Acceptance tests.**
  - `...RoundTripPreservesRecords` (AC 1): create request A and a claim with context X (as `claim_workflow_git_test.go`); record origin `docket` tip T and `ls-tree -r T`; go private (both flags; push the removal commit), then shared (push any identity commit; re-run). Tip is T, tree identical; replaying A returns the same id with the replay disposition (`grep -n 'Replay' internal/app/change_create_test.go`) and moves nothing; the claim replays likewise.
  - `...InterruptedPhasesResume` (AC 2): for every phase P in both directions (both flags going private), a fresh fixture whose `afterVisibilityPhase` errors at P; run 1 fails, run 2 completes. The final state equals an uninterrupted run's (tips, folder name, one worktree registration, branches); each subject once in `git log --format=%s main`; metadata tips unchanged; never both state folders, no journal, no leftover `config.yml`/`local-keys.yml` in the shared folder.
  - `...PrivateResultIsClean` (AC 7): after both flags, `RunRepositoryCheck` has no error, no `metadata-on-shared-remote`, no `visibility-mismatch`; no root entry contains `docket` (case-insensitive) besides `.git`; the metadata worktree is outside the primary.
  - `...CommitMessages` (AC 9): every switch commit has one of the two subjects and empty `%b`; a hand-edited `.gitignore` refuses the shared preview; detached HEAD refuses a committing plan; a once-failing hook -> `external-failed` with edits kept, then exactly one commit.
  - `...UserEditsSurvive` (Review Focus 2): a staged `notes.txt` and an unstaged `README.md` edit survive both directions.
  - `...SecondCloneShared` (AC 6): A and B share a private store; A goes shared; B's `publish` is `done` and B completes with local phases only.
- [ ] **Step 2:** `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoVisibility' -count=1`. Fix code, never the oracle; an assert that cannot hold as written -> `NEEDS_ESCALATION` naming it.
- [ ] **Step 3: Mutations** (backup, one at a time, restore with `cp`, record each red message): `publishVisibilityHistory` lease with an empty expected tip -> round-trip or foreign-remote test red; `state-folder` before `config` -> interruption table red at `state-folder`; `commitSwitchJournal` never clears -> the journal assertion red.
- [ ] **Step 4: Budget.** Run `tests/test_go_integration_app_repovisibility.sh` serially three times; set its row to the worst reading rounded up to the next multiple of 5 plus 5s. Record the numbers.
- [ ] **Step 5: Commit.** `test(repository): set-visibility acceptance — round trip, resume after every phase, private result, commit messages`

---

### Task 12: Documentation surfaces

**Files:** `.docket.example.yml`; `skills/docket-convention/SKILL.md`; regenerated `internal/assets/embedded/`; `internal/repoguard/budgets_test.go` `skillBudgets` row if the ratchet requires it

- [ ] **Step 1:** in the `visibility` note of `.docket.example.yml`, replace "changing it later does not move a repository that is already set up." with "changing it later does not move a repository that is already set up; `docket repository set-visibility <shared|private>` moves one, keeping every record."
- [ ] **Step 2:** after the convention sentence "A private repository (`.git/dckt/` present) spells the metadata branch and remote `dckt`, ...", add "Editing `visibility` never moves a repository; the `repository.set-visibility` operation does, keeping every record." (no `.docket/`, no `origin/docket`).
- [ ] **Step 3:** `go generate ./internal/assets/`; `go test ./internal/config/ ./internal/repoguard/ ./internal/assets/ -count=1` (`TestEmbeddedMatchesAuthored`, `TestSkillsSpellNoMetadataLiterals`, `TestCapabilitySurface`, example correspondence, `TestSkillSizeBudgets`).
- [ ] **Step 4:** `go run ./cmd/docket repository set-visibility --help` from the feature worktree reads plainly: preview first, `--yes` applies, a re-run resumes, live runs refuse, each flag's effect, commits local and never pushed, the bare remote kept going shared.
- [ ] **Step 5: Commit.** `docs: describe moving a repository between visibilities in the example config, the convention, and command help`

---

### Final: whole-suite gate

- [ ] **Step 1:** `go generate ./internal/assets/`, then `git status --porcelain` is empty.
- [ ] **Step 2:** run whatever `build.test_command` resolves to (from config, never this plan); green. Act on `SERIAL CONFIRMED OVER BUDGET:`; record `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines.
- [ ] **Step 3: Residue greps,** both empty:
  - `grep -rn --include='*.go' -E '"refs/heads/docket"|"origin/docket"' internal cmd | grep -v _test.go`
  - `grep -rn --include='*.go' 'docket configurations' internal cmd | grep -v _test.go | grep -v -e 'visibilityRemoveSubject *=' -e 'visibilityAddSubject *='`
- [ ] **Step 4: Notes for the results file** (the parent writes it): every mutation and its red message; the new shard's serial readings and row; Task 3's real-tree numbers; the deviations under *Decisions made in planning*; residuals: a feature worktree's `core.hooksPath` still names the old state folder's `empty-hooks` after a switch (git reads a missing folder as no hooks; `workspace prepare` re-points it); `refs/docket/*` owned refs and a linked worktree's own record under `.git/worktrees/<name>/docket/` are not renamed; after an interrupted switch to shared, an edit outside the dispatch block in `AGENTS.md`/`CLAUDE.md` made before the re-run is refused (not journaled), and the remedy asks for review.
