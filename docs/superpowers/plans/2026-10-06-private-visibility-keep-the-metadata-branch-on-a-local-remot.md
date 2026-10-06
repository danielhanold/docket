<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0531 — Keep the metadata branch on a local remote with neutral naming](../../changes/archive/2026-10-06-0531-private-visibility-keep-the-metadata-branch-on-a-local-remot.md)**
<!-- docket:backlink:end -->

# Keep the Metadata Branch on a Local Remote with Neutral Naming: Implementation Plan

> **For agentic workers:** `docket-build` executes this plan. It routes each task to a tier agent running the `docket-build-task` contract. Each task carries its own focused test cycle and ends in one commit. The whole suite runs once at the end. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Add a **private** visibility. The repository keeps docket's single metadata layout, but:
- the metadata branch is pushed to a bare repository on the user's machine instead of `origin`;
- every per-repo name docket writes is spelled `dckt`;
- nothing docket-named sits in the repository.

**Architecture:** A new stdlib-only leaf package, `internal/layout`, owns every per-repo spelling and decides the mode **from state**: a `<git-common-dir>/dckt/` directory means private. It resolves a `layout.Layout` holding the metadata remote, metadata branch, state folder, metadata worktree, and private config path.

- **Phase A (Tasks 1–4) is behavior-neutral in shared mode.** It threads `Layout` through the state-folder helpers, the operational loader (`StatusPin.Layout`), and the repository command family (`setupContext.layout`). It deletes `reposetup.MetadataBranchName` and `docketWorktreeName` so the compiler finds every site.
- **Phase B (Tasks 5–11) adds private mode:**
  - the `visibility` key and the private config source;
  - gitcli `InitBare` and `AddRemote`;
  - the `.git/info/exclude` block;
  - `init --private`;
  - private-aware check, prepare and repair, with two new findings;
  - the acceptance tests;
  - the skill prose.

**Split decision:** one plan and one PR. Phase A is the natural 2a/2b cut the spec allows if review asks for a split: every Phase A task is green on its own, with shared behavior unchanged.

**Tech Stack:** Go; real-git tests behind `//go:build integration`, sharded by name prefix (`tests/test_go_integration_*.sh`; `tests/test_go_integration_contract.sh` enforces the prefixes); cobra; embedded skill assets (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-05-private-visibility-keep-the-metadata-branch-on-a-local-remot-design.md`. A copy is on the feature branch. Read it first.

## Global Constraints

The spec's **Decisions** 1–8 are binding; read them there. In short:
- **The key:** `visibility` is an ordinary key, any layer, with normal precedence. `init` uses the effective value, and `--private` / `--shared` override it.
- **Mode is state:** a `.git/dckt/` folder means private, and config never moves a repository.
- **Private spellings:**
  - `.git/dckt/` and `.git/dckt/config.yml`;
  - the `dckt` branch and remote;
  - `${XDG_DATA_HOME:-~/.local/share}/dckt/<owner>-<repo>/{remote.git,checkouts/<clone-id>/}`;
  - the `# dckt:start` / `# dckt:end` markers.
- **Not renamed:** the binary, its install paths, the global config path, agent files, and `Docket-*` trailers.
- **Private config:** a private repository never reads `.docket.yml`, honors identity keys from `.git/dckt/config.yml`, and refuses when both local files exist.
- **Private init writes no root file.** Code-side remote work stays on `origin`.
- **Docs:** only `.docket.example.yml` (and its twin), command help, and skills.
- **AGENTS.md rules:**
  - derive sites by grep, never a list;
  - mutation-test every guard, restoring from a **backup copy** (never `git checkout --`) and running with `-count=1`;
  - validate marker balance;
  - template `mktemp`;
  - anchor cross-references on symbols, never line numbers.

## Review Focus

Implied by the spec, untested by its acceptance list. Each has a test in its owning task.
1. **Second clone of the same origin:** it adopts the store's `dckt` branch and gets its own checkout, and the first clone is untouched (Task 8).
2. **A pre-existing `dckt` remote with another URL:** refuse and name both URLs; never rewrite the remote (Task 8).
3. **`.git/info/exclude` with user lines or malformed `# dckt:` markers:** keep the bytes; refuse untouched (Task 7).
4. **Re-run `init`/`prepare` on a private repository:** no-op. A mode-switching flag refuses (Task 8).
5. **Origin URL forms** (scp, https, ssh, local path) all yield a slug. An unreadable origin URL in a private repository is a clear external failure, and a relative `XDG_DATA_HOME` is ignored (Tasks 1 and 3).

## Learnings applied

These apply throughout:
- defaulted-param-hides-caller-wiring: assert a **non-default** layout arrives.
- probe-error-is-not-clean-absence: `Detect` and the orphan scan error, never guess.
- presence-encoded-state: a failed private init names its recovery.
- shared-resource-keeps-first-owner-assumptions: the second-clone test.
- intermediate-task-state-buildable and restatement-accumulates-its-own-guards: pins move with their prose.
- budget-headroom-is-spent-before-it-is-breached: record margins as numbers.

**Overridden, per the spec:** gitignore-guarantee-must-be-committed. A single-clone private repository uses `.git/info/exclude`; record that in the results file.

## ADRs to record

The implement-next parent records these through `docket-adr`. They are not build tasks.

1. **Private visibility keeps the single metadata layout.** Only two things vary: where the metadata branch is published (bare `dckt` remote vs `origin`) and how per-repo paths are spelled. The mode is decided from state, never from config. Relates to ADR-0099 and ADR-0001.
2. **In a private repository, repository-identity keys come from `.git/dckt/config.yml`.** It is read as the repository layer, because with a single clone there is no committed file every clone must agree on. Relates to ADR-0019.

---

## Phase A: resolvers (behavior-neutral in shared mode)

### Task 1: The `internal/layout` package

**Files:** Create `internal/layout/layout.go` and `internal/layout/layout_test.go`. Use the default tag; no git.

**Interfaces.** It consumes nothing. It must import no internal package, because `gitcli` and `config` import it. It produces exactly this exported API (Step 3 sketches it): `Mode`, `Shared`, `Private`, `SharedName`, `PrivateName`, `SharedRemote`, `SharedWorktreeDir`, `PrivateConfigFile`, `PrivateConfigDisplay`, `Detect`, `StateName`, `StateDirOf`, `PrivateConfigPath`, `CommonDirOf`, `DataHome`, `OwnerRepo`, `CloneID`, `Layout`, `SharedLayout`, `PrivateLayout`, `Layout.MetadataRef`, and `Layout.TrackingRef`.

- [ ] **Step 1: Write the failing tests.** Each bullet is one test with exact expectations.
  - **`TestDetect`:**
    - an empty common dir gives `(Shared, nil)` and `StateDirOf` = `<c>/docket`;
    - a `dckt` **directory** gives `(Private, nil)`, `StateDirOf` = `<c>/dckt`, and `PrivateConfigPath` = `<c>/dckt/config.yml`;
    - a `dckt` **file** makes `Detect` return an error, while `StateName` returns `"dckt"` (fail-closed).
  - **`TestOwnerRepoForms`:**
    - `git@github.com:DanielHanold/Docket.git` → `danielhanold-docket`
    - `https://github.com/o/r` → `o-r`
    - `https://github.com/o/r.git/` → `o-r`
    - `ssh://git@host:2222/Team.X/My_Repo.git` → `team-x-my-repo`
    - `/tmp/abc/origin.git` → `abc-origin`
    - `""`, `"   "`, `"/"`, `".git"` and `"::"` → error
  - **`TestCloneID`:**
    - `CloneID("/home/u/src/api")` matches `^api-[0-9a-f]{8}$`;
    - it is deterministic;
    - it differs for `/home/u/other/api`.
  - **`TestDataHome`:**
    - an absolute `XDG_DATA_HOME` is used;
    - a relative one is ignored, giving `<home>/.local/share`;
    - unset gives `<home>/.local/share`;
    - no XDG and a home error gives an error.
  - **`TestLayouts`:**
    - `SharedLayout("/r/.git","/r")` equals `{Shared, "/r/.git/docket", "origin", "docket", "/r/.docket", "", "", "", ""}`, with refs `refs/heads/docket` and `refs/remotes/origin/docket`.
    - `PrivateLayout("/r/.git","/r","/data","o-r")` equals `{Private, "/r/.git/dckt", "dckt", "dckt", "/data/dckt/o-r/checkouts/"+CloneID("/r"), "/r/.git/dckt/config.yml", "/data/dckt/o-r", "/data/dckt/o-r/remote.git", "/data/dckt/o-r/checkouts"}`, with refs `refs/heads/dckt` and `refs/remotes/dckt/dckt`.
  - **`TestCommonDirOf`:**
    - a `<root>/.git` dir gives `(<root>/.git, true, nil)`;
    - a linked root whose `.git` file is `gitdir: <c>/worktrees/feat`, with `commondir` = `../..`, gives `(<c>, true, nil)`;
    - a plain dir gives `ok=false, err=nil`.

- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/layout/ -count=1`. Expected: build failure.

- [ ] **Step 3: Implement `layout.go`.**

```go
// Package layout owns every per-repo spelling docket writes and decides the
// visibility mode from STATE (a <git-common-dir>/dckt/ directory means private).
package layout

import ("bufio"; "crypto/sha256"; "encoding/hex"; "errors"; "fmt"; "io/fs"; "os"; "path/filepath"; "strings")

type Mode string

const (
	Shared  Mode = "shared"
	Private Mode = "private"

	SharedName           = "docket"
	PrivateName          = "dckt"
	SharedRemote         = "origin"
	SharedWorktreeDir    = ".docket"
	PrivateConfigFile    = "config.yml"
	PrivateConfigDisplay = ".git/dckt/config.yml"
)

// Detect decides the mode from state; a probe error is returned, never guessed.
func Detect(commonDir string) (Mode, error) {
	p := filepath.Join(commonDir, PrivateName)
	fi, err := os.Lstat(p)
	switch {
	case err == nil && fi.IsDir():
		return Private, nil
	case err == nil:
		return "", fmt.Errorf("layout: %s exists but is not a directory", p)
	case errors.Is(err, fs.ErrNotExist):
		return Shared, nil
	default:
		return "", fmt.Errorf("layout: probing %s: %w", p, err)
	}
}

// StateName serves path helpers with no error channel. It fails CLOSED:
// anything present (or unprobeable) at <common>/dckt selects the private name,
// so a write beneath it fails loudly instead of landing in a docket-named
// folder. Mode-deciding callers use Detect.
func StateName(commonDir string) string {
	if _, err := os.Lstat(filepath.Join(commonDir, PrivateName)); errors.Is(err, fs.ErrNotExist) {
		return SharedName
	}
	return PrivateName
}

func StateDirOf(commonDir string) string        { return filepath.Join(commonDir, StateName(commonDir)) }
func PrivateConfigPath(commonDir string) string { return filepath.Join(commonDir, PrivateName, PrivateConfigFile) }

// CommonDirOf resolves a working-tree root's common dir from the filesystem
// alone (no git process). ok=false when <root>/.git does not exist.
func CommonDirOf(root string) (string, bool, error) { /* see rules below */ }

// DataHome is ${XDG_DATA_HOME:-$HOME/.local/share}; XDG_DATA_HOME counts only
// when absolute (the installer's rule in install.ResolveRoots). No XDG and a
// home error (or empty home) → error.
func DataHome(getenv func(string) string, home func() (string, error)) (string, error) { /* … */ }

// OwnerRepo derives <owner>-<repo> from origin's URL: the last two segments
// (split on / : \, trailing "/" and ".git" removed), lowercased, with
// non-alphanumeric runs collapsed to "-".
func OwnerRepo(originURL string) (string, error) {
	s := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(originURL), "/"), ".git")
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '/' || r == ':' || r == '\\' })
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.Join(parts, "-")) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimRight(b.String(), "-")
	if slug == "" {
		return "", fmt.Errorf("layout: cannot derive an owner/repo name from origin URL %q", originURL)
	}
	return slug, nil
}

// CloneID is <basename>-<first 8 hex of sha256(abs path)>; the basename keeps
// [A-Za-z0-9._-] and maps anything else to '-'.
func CloneID(primaryWorktree string) string {
	sum := sha256.Sum256([]byte(primaryWorktree))
	base := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, filepath.Base(primaryWorktree))
	return base + "-" + hex.EncodeToString(sum[:])[:8]
}

type Layout struct {
	Mode                                                       Mode
	StateDir, MetadataRemote, MetadataBranch, MetadataWorktree string
	ConfigPath, StoreDir, DefaultBareRemote, CheckoutsDir      string // private only
}
// SharedLayout / PrivateLayout build exactly the values TestLayouts pins;
// MetadataRef() = "refs/heads/"+MetadataBranch; TrackingRef() = "refs/remotes/"+MetadataRemote+"/"+MetadataBranch.
```

  The single-line import list is shorthand. `gofmt` expands it.

  **`CommonDirOf` rules:**
  - A `<root>/.git` directory is the common dir.
  - A `.git` file names a gitdir on its first `gitdir:` line, resolved relative to root if it is not absolute.
  - That gitdir's `commondir` file, resolved relative to the gitdir and cleaned, names the common dir.
  - A gitdir with no `commondir` file (a submodule) is its own common dir.
  - Every read error other than not-exist is returned.

- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/layout/ -count=1`, then `gofmt -l internal/layout`. Expected: PASS and no gofmt output.

- [ ] **Step 5: Commit.** Message: `feat(layout): resolve per-repo spellings and visibility mode from state`.

---

### Task 2: Route every state-folder path through `layout.StateDirOf`

**Files:**
- **run tracker** (`internal/app/runtracker_store.go`): the root, mint, load, save, retry and `runKeyDir` helpers; `runtracker_start.go` `acquireResumeLock`
- **agent guardian** (`agent_guardian.go`): `markerPathFor`
- **transactions** (`internal/repository/transaction/candidate.go`): `transactionsRoot`
- **workspaces** (`internal/workspace/manifest.go`): `workspacesRoot`
- **gatedrive** (`internal/gatedrive/store.go`): `OpenStore`
- **suiterunner** (`internal/suiterunner/budgetstate.go`): `DefaultStatePath`
- **hooks** (`internal/gitcli/hooksoff.go`): `emptyHooksDir`
- **install record** (`internal/reposeed/record.go`): `RecordPath`, and its caller `internal/app/repophase.go` `ResolveRepoPhase`
- **message text** (`internal/app/gate_drive.go`): `incumbentRemedyMessage`
- **guard** (new): `internal/repoguard/layout_guard_test.go`

**Interfaces:**
- **Produces:**
  - `reposeed.RecordPath(gitDir, stateName string) string`;
  - an unexported `runTrackerStateDir(repoDir string) (string, error)`;
  - the guard `TestNoInlineStateFolderSpelling`, with the walker `walkGo(t, root, visit func(rel string))`.

**Derive the sites first:** `grep -rn --include='*.go' -E 'filepath\.Join\([^)]*"docket"' internal cmd | grep -v _test.go`. Grooming found 20. **Machine paths stay as they are:** `internal/config/fs.go` and `internal/install/roots.go`.

- [ ] **Step 1: Write the failing guard.**
  - Parse every non-test `.go` file under `internal/` and `cmd/` with `go/parser`. Skip `testdata/`, `internal/assets/embedded/`, `internal/layout/`, and the two machine-path files.
  - Report each `filepath.Join` call that has a string-literal `"docket"` argument, as `fset.Position`.
  - Reuse `guardRoot(t)`.
- [ ] **Step 2: Run the guard and watch it fail.** Run `go test ./internal/repoguard/ -run TestNoInlineStateFolderSpelling -count=1`. Expected: FAIL. The offenders must equal the grep output.
- [ ] **Step 3: Convert each site.**
  - **Run tracker:** `runTrackerStateDir` returns `layout.StateDirOf(<runTrackerGitCommonDir>)`. Replace each `Join(common, "docket", X…)` with `Join(state, X…)`. **Keep `rec.Repo = common`**: it is identity, not a path.
  - **Agent guardian:** `markerPathFor(stateDir, runKey)`.
  - **Transactions, workspaces, gatedrive** (computed once in `OpenStore`), **suiterunner and hooks:** replace each `Join(<common>, "docket", …)` with `Join(layout.StateDirOf(<common>), …)`.
  - **Install record:** `RecordPath(gitDir, stateName)` returns `Join(gitDir, stateName, "install.json")`. `ResolveRepoPhase` derives the state name from `layout.CommonDirOf(root)`. An error or `!ok` refuses with the neighboring repo-dir `RepoResolutionError` reason. Then call `reposeed.RecordPath(gitDir, layout.StateName(common))`.
  - **Message text:** `incumbentRemedyMessage` says "under the repository's per-repo state folder (worktree-locks/<key>/busy.lock)". First grep the tests for `docket/worktree-locks` and repoint any assert.
  - **Comments** spelling `<common>/docket/…` become "the per-repo state folder".
- [ ] **Step 4: Prove the private name is honored.** Add default-tag tests:
  - `TestOpenStoreRootsFollowStateFolder` (gatedrive);
  - `TestWorkspacesRootFollowsStateFolder`;
  - `TestTransactionsRootFollowsStateFolder`;
  - `TestRecordPathUsesStateName`.

  Each creates `<tmp>/dckt` and asserts the path is under `<tmp>/dckt/`, then removes it and asserts `<tmp>/docket/`. The git-resolved sites (run tracker, suiterunner, hooks) are proven by Task 10's walk. Say so in the commit body.
- [ ] **Step 5: Run the tests and confirm they pass.** Run `go test ./internal/... -count=1` and `go vet ./...`.
- [ ] **Step 6: Mutation-test.** Re-inline `"docket"` in `workspacesRoot`. The guard must go red. Restore from the backup.
- [ ] **Step 7: Commit.** Message: `refactor: resolve the per-repo state folder through layout (shared behavior unchanged)`.

---

### Task 3: Thread the metadata remote and branch through the operational loader

**Files:**
- Create: `internal/app/layout_resolve.go` and `_test.go`
- Modify: `internal/app/operational_context.go`, `status.go` (`StatusPin`), `status_git.go` (`PinContext`), `sweep_session.go`, `link_context.go`, `implementation_context.go`, `status_human.go`
- Modify: the 22 `transaction.Request{` sites (`grep -rn 'transaction.Request{' internal/app --include='*.go' | grep -v _test`)
- Modify: `internal/reposetup/branch.go` (delete `MetadataBranchName`), `initplan.go`, the comment in `internal/config/config.go`
- Modify: `internal/render/link.go` and `internal/repoguard/layout_guard_test.go`

**Interfaces:**

```go
type remoteURLReader interface {
	RemoteURL(ctx context.Context, repo gitcli.Repository, remote gitcli.RemoteName) (string, error)
}
func resolveLayout(ctx context.Context, r remoteURLReader, repo gitcli.Repository) (layout.Layout, error)
func metadataRemote(l layout.Layout) gitcli.RemoteName // gitcli.RemoteName(l.MetadataRemote)
func metadataRef(l layout.Layout) gitcli.RefName       // gitcli.RefName(l.MetadataRef())
// StatusPin gains Layout layout.Layout; render.LinkContext gains PrivateMetadata bool
func fetchPinnedRevision(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, remote gitcli.RemoteName, branch gitcli.RefName) (string, error)
```

- [ ] **Step 1: Write the failing tests.**
  - **`layout_resolve_test.go`** uses the default tag and a `fakeURLReader{url, err}`:
    - `TestResolveLayoutSharedNeedsNoOriginURL`: with no `dckt` dir and a reader that errors if called, the result equals `layout.SharedLayout(common, "/r")`.
    - `TestResolveLayoutPrivateUsesOriginAndDataHome`: with a `dckt` dir, `XDG_DATA_HOME` set, and the URL `git@github.com:O/R.git`, the result equals `layout.PrivateLayout(common, "/r", data, "o-r")`. Also check `metadataRemote` = `dckt` and `metadataRef` = `refs/heads/dckt`.
    - `TestResolveLayoutPrivateWithoutOriginURLFails`: with a `dckt` dir and a reader that errors, the result is an error.
  - **render `TestPrivateMetadataRendersNoMetadataURL`.** With `{RepoWebURL: https://github.com/o/r, MetadataBranch: dckt, IntegrationBranch: main, PrivateMetadata: true}`:
    - `BlobURL(p)` = `""`;
    - `BlobURLOnBranch(p, "")` = `""`;
    - `BlobURLOnBranch(p, "main")` stays absolute.
  - **`TestClaimTransactionTargetsPinnedMetadataLayout`** (defaulted-param-hides-caller-wiring). Use the claim test seams: a fake reader plus a recording engine (`grep -rn 'calls = append' internal/app/*_test.go`; `claimGateEngine`/`gateClaimDeps` live in the `TestIntegrationRecordOps` shard). Make the pin's `Layout` equal `layout.PrivateLayout("/c","/r","/d","o-r")`, then assert the recorded request has `Remote == "dckt"` and `TargetRef == "refs/heads/dckt"`.
  - **Guard `TestTransactionRequestsUseResolvedMetadataRemote`.** No `transaction.Request` composite literal in non-test `internal/app/*.go` may set `Remote` to the identifier `originRemote`. Population floor: at least 20 literals found.
- [ ] **Step 2: Run the tests and watch them fail.** Expected: compile errors, and the guard lists 22 offenders.
- [ ] **Step 3: Implement.**
  - **`resolveLayout`:**
    - `layout.Detect(repo.CommonDir)`; shared returns `layout.SharedLayout(...)`.
    - Private reads `r.RemoteURL(ctx, repo, originRemote)`, then `layout.OwnerRepo`, then `layout.DataHome(os.Getenv, os.UserHomeDir)`, and returns `layout.PrivateLayout(...)`.
    - Every error is wrapped as `fmt.Errorf("%w: …: %v", ErrStatusExternal, err)`. The origin-URL error says "private repository: reading origin's URL to locate the metadata store".
  - **`loadOperationalContext`:**
    - resolve `oc.layout` right after `Discover`;
    - `fetchPinnedRevision` gains a `remote` argument;
    - the default and integration fetches pass `originRemote`;
    - the metadata pin passes `metadataRemote(oc.layout)` and `metadataRef(oc.layout)`;
    - `PinContext` copies the layout into `StatusPin.Layout`.
  - **The 22 transaction sites** become `Remote: metadataRemote(pin.Layout), TargetRef: metadataRef(pin.Layout)`.
    - Where `pin` is out of scope (`changeAttachMetadata`, `recordBuildEvidence`, `closeoutStacked`, `runCloseoutArchiveTransaction`, `adrReplace`), thread a `lay layout.Layout` parameter from the caller's pin.
    - Feature-branch probes, workspace, finalize, maintenance and `Workflow.Remote` stay `originRemote`.
  - **Other readers:**
    - `sweepSession.Prepare` fetches through `s.base.Layout`.
    - `linkContextOf` sets `MetadataBranch: pin.Layout.MetadataBranch` and `PrivateMetadata: pin.Layout.Mode == layout.Private`.
    - `BlobURLOnBranch` returns `""` when `PrivateMetadata && branch == MetadataBranch`, checked after the empty-branch fallback. Document the field: no web page exists for a private metadata branch.
    - `implementation_context.go`: `MetadataRef: pin.Layout.MetadataBranch`.
    - `status_human.go` prints a new `MetadataBranch string \`json:"metadata_branch,omitempty"\`` on the status result's context struct, filled from the pin.
  - **Spellings and comments:**
    - Delete `MetadataBranchName`.
    - `initplan.go` uses `"refs/heads/"+layout.SharedName` and `layout.SharedWorktreeDir`.
    - Fix the comments that say "fixed docket branch".
  - **Repository-family sites** the compiler lists belong to Task 4. Either do them here, or alias each through `layout.SharedName` with a `// Task 4` comment; Task 4 removes every alias.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/render/ ./internal/repoguard/ ./internal/reposetup/ -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationWorkflowLifecycle' -count=1`.
- [ ] **Step 5: Mutation-test.** Put `Remote: originRemote` back in `ChangeClaim`. The guard and the wiring test must both go red. Restore from the backup.
- [ ] **Step 6: Commit.** Message: `refactor(app): thread the resolved metadata remote and branch through StatusPin`.

---

### Task 4: Thread the layout through the repository command family

**Files:**
- `internal/app/repository_facts.go`:
  - `setupContext.layout` and `repoFactsInput.layout`;
  - `setupProber` gains `RemoteURL`;
  - `docketWorktreeFact` gains a `worktreePath` param;
  - delete `docketWorktreeName`;
  - `isDocketManagedWorktreePath` uses `layout.SharedWorktreeDir`.
- `repository_{init,check,prepare,repair,migrate}.go`
- `internal/reposetup/probe.go` and `health.go`
- The `setupProber` fakes (`grep -rln 'ProbeRemoteBranch(ctx context.Context' internal/app/*_test.go`)

**Interfaces.** This task adds:
- `PrepareContext.MetadataRemote` (`json:"metadata_remote"`) and `MetadataTrackingRef` (`json:"metadata_tracking_ref"`);
- `reposetup.Facts.MetadataWorktreeRef string` and `Facts.Private bool`;
- `reposetup.InterruptedFastForwardRemedy(worktree string) string`, now a function;
- `publishOrAdoptMetadataRoot(..., remote gitcli.RemoteName, metaRef, ...)`.

- [ ] **Step 1: Write the failing tests.**
  - The existing prepare test asserting `MetadataWorktreePath` also asserts `MetadataRemote == "origin"` and `MetadataTrackingRef == "refs/remotes/origin/docket"`.
  - Add `reposetup` `TestFindingsUseResolvedWorktreeRef`: for a missing worktree with `MetadataWorktreeRef: "/d/dckt/o-r/checkouts/r-12345678"`, every worktree finding has that `Ref`, and no message contains `.docket`.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**
  - **Fakes:** each fake's `RemoteURL` returns `"git@github.com:o/r.git", nil`.
  - **`gatherSetupFacts`:** call `resolveLayout` after `Discover` and return its error, which maps to external-failed.
  - **`gatherRepoFacts`:**
    - set `sc.layout`;
    - probe the metadata branch on `metadataRemote(in.layout)` / `metadataRef(in.layout)`;
    - call `docketWorktreeFact(..., in.layout.MetadataWorktree, metaRef)`;
    - set `f.MetadataWorktreeRef` to `.docket` when shared, or the checkout path when private;
    - set `f.Private`.
  - **`loadOperationalContext`** passes its layout into `gatherRepoFacts`.
  - **Metadata sites** use `sc.layout`:
    - the `augmentCheckFacts` fetch;
    - `prepareAugment`/`prepareExecute`;
    - the repair push, fetch and preview;
    - migrate's `migrateRoute`, `reconcileResumeSeed`, `migrateResumeLocal`, `publishSeed`, `verifySeedPublished`;
    - init's `publishOrAdoptMetadataRoot` and its worktree path.

    `migrateExecute`'s integration prune and `migratePrimarySyncRemedy` stay on `setupRemote()`. `migratePreviewText` prints both remotes.
  - **`buildPrepareContext`:** every metadata field comes from `sc.layout`, with `MetadataTrackingRef = sc.layout.TrackingRef()`. Extend the consumer-inventory comment.
  - **`prepareAttachFresh`:** `MkdirAll` the worktree's parent first.
  - **`health.go`:**
    - `Ref: ".docket"` becomes `f.MetadataWorktreeRef`;
    - `.docket` in messages and remedies becomes the ref via `fmt.Sprintf`;
    - `InterruptedFastForwardRemedy(worktree)` formats `git -C <worktree> reset --merge HEAD`.

    **First** grep the tests for `The .docket` and `git -C .docket` and repoint those asserts in this task. Shared refs stay `.docket`.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/reposetup/ -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoSetup' -count=1`. Then `grep -rn 'docketWorktreeName\|MetadataBranchName' internal --include='*.go'` must be empty.
- [ ] **Step 5: Commit.** Message: `refactor(repository): resolve metadata remote, branch, and worktree through the layout`.

---

## Phase B: private mode

### Task 5: The `visibility` key and the private config source

**Files:**
- `internal/config/{schema,config,defaults,resolve,fs}.go`
- `internal/app/config.go` (`effectiveLines`) and `operational_context.go` (`operationalConfigSources`)
- `.docket.example.yml`, then `go generate ./internal/assets/`
- The key-listing tests:
  - `TestRegistryPathSetMatchesV092`, `TestRegistryDefaults`, `TestRegistryEnumRows`;
  - `TestSettingPathsSupportSplit`;
  - the leaves map in `TestBuiltinEffectiveMatchesRegistryDefaults`;
  - `effectiveLeaf`, if it enumerates.

**Interfaces:**

```go
Effective.Visibility Value[string] `json:"visibility"`
type ConflictingLocalConfigError struct{ LocalPath, PrivatePath string } // Error() names both paths
func LoadPrivateRepositorySource(commonDir, primaryWorktree string) ([]Source, error)
```

`LoadFilesystemSources` keeps its signature but becomes private-aware through `layout.CommonDirOf` and `layout.Detect`. That covers `docket diagnostic config` (which does no git discovery), `resolveSetupConfig`, and `ResolveRepoPhase` with no caller change.

- [ ] **Step 1: Write the failing tests.** A private fixture is a temp root with `.git/dckt/`.
  - **`…PrivateReadsDcktConfigNotDocketYML`:**
    - the root's `.docket.yml` says `changes_dir: committed`;
    - `.git/dckt/config.yml` says `visibility: private` and `changes_dir: local-private`;
    - the sources are exactly `[{LayerRepository, ".git/dckt/config.yml"}]` (there is no global file);
    - resolved `ChangesDir` = `local-private` (an identity key honored), and `Visibility` = `private`.
  - **`…PrivateRefusesBothLocalFiles`:** `errors.As(err, *ConflictingLocalConfigError)` holds, and the message names both paths.
  - **`…SharedUnchanged`:** a `.git` dir without `dckt` still yields `.docket.yml`.
  - **`TestGlobalVisibilityIsOrdinary`:** a global `visibility: private` resolves from `LayerGlobal` with no warnings.
  - **`TestSharedSettingRemedyNamesPrivateConfig`:** a global `integration_branch` with a repository source named `.git/dckt/config.yml` produces a `shared-setting-ignored` remedy naming that path.
- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/config/ -count=1`.
- [ ] **Step 3: Implement.**
  - **Schema row**, placed beside `integration_branch`: `{path: "visibility", kind: kindString, enum: []string{"shared", "private"}, def: "shared", merge: mergeScalar, scope: scopeAny, disp: dispSupported, validate: enumLeaf("shared", "private")}`.
  - **Effective and wiring:**
    - add the `Effective` field;
    - add `Visibility: builtinValue("shared")` to `builtinEffective`;
    - add `set(assign(&eff.Visibility, r.declared, "visibility"))` to `assemble`;
    - add a `leafLine("visibility", …)` in `effectiveLines`.
  - **`LoadPrivateRepositorySource`:**
    - It reads `layout.PrivateConfigPath(commonDir)`; not-exist returns `nil, nil`, and any other error is returned.
    - If `<primary>/.docket.local.yml` exists, it returns `&ConflictingLocalConfigError{LocalPath, PrivatePath}`. The error text is "config: both %s and %s exist; a private repository reads only %s — move any settings you need into it and delete %s". A stat error other than not-exist is returned.
    - Otherwise it returns `[]Source{{Layer: LayerRepository, Name: layout.PrivateConfigDisplay, Data: data}}`.
  - **`LoadFilesystemSources`:** move the `globalPath` computation up. After the directory stat, if `layout.CommonDirOf(repoDir)` finds a repository that `layout.Detect` calls private, return `LoadGlobalSource(globalPath)` plus `LoadPrivateRepositorySource(common, repoDir)`. Probe errors are returned. Update the doc comment.
  - **`resolve.go`:** record `r.repoSourceName`, the `LayerRepository` source's `Name`. The `applySharedSettingGuard` remedies, including the `board_surfaces` one, name `"the committed .docket.yml"` when that name is `""` or `.docket.yml`, so the shared text stays byte-identical, and the source name otherwise.
  - **`operationalConfigSources(repo, lay, docketYML)`:** private mode returns the global source plus `LoadPrivateRepositorySource`. **`loadOperationalContext` skips the pinned `.docket.yml` blob read in private mode.** A `*ConflictingLocalConfigError` becomes `&errInvalidConfiguration{err: err}`.
  - **`.docket.example.yml`:** use the house format, written for the user deciding whether to set the key:

```yaml
# Where docket keeps this repository's planning data. `shared` (the default)
# keeps it on a `docket` branch pushed to origin, beside a `.docket/` worktree,
# so everyone with the repository sees the backlog. `private` keeps it on your
# machine only: a `dckt` branch pushed to a bare repository under
# ~/.local/share/dckt, with nothing docket-named inside the repository — for
# repositories that must not carry any docket files. `docket repository init`
# sets a new repository up in this mode (`--private` / `--shared` override it);
# changing it later does not move a repository that is already set up.
# scope: any layer
visibility: shared
```

  - Then run `go generate ./internal/assets/` and update the key-listing tests.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/config/ ./internal/assets/ ./internal/app/ -count=1`, including `TestExampleSchemaCorrespondence` and `TestEmbeddedMatchesAuthored`.
- [ ] **Step 5: Mutation-test.** Disable the private branch. `…PrivateReadsDcktConfigNotDocketYML` must go red. Restore from the backup.
- [ ] **Step 6: Commit.** Message: `feat(config): add the visibility key and the private .git/dckt/config.yml repository layer`.

---

### Task 6: gitcli `InitBare` and `AddRemote`

**Files:** Create `internal/gitcli/remoteadmin.go` and `internal/gitcli/remoteadmin_integration_test.go`. Line 1 of the test file is `//go:build integration` and line 2 is blank. Name the test `TestIntegrationRepo…`, after checking the gitcli shard prefix in `tests/test_go_integration_*gitcli*.sh`.

**Interfaces:**

```go
func (c *Client) InitBare(ctx context.Context, path string) error  // idempotent on an existing bare repo
func (c *Client) AddRemote(ctx context.Context, repo Repository, name RemoteName, url string) error
```

- [ ] **Step 1: Write the failing test `TestIntegrationRepoInitBareAndAddRemote`.** Use the harness from `refs_integration_test.go`: `requireGit`, `newRealClient`, `newMainModeRepos`, `mustDiscover`, `gitOut`. Assert each of these:
  - `InitBare` on `<tmp>/store/o-r/remote.git` succeeds when the parents do not exist yet;
  - a second `InitBare` on the same path succeeds (idempotent);
  - `rev-parse --is-bare-repository` reports `true`;
  - `AddRemote(repo, "dckt", bare)` succeeds;
  - `RemoteURL(repo, "dckt")` returns `bare`;
  - a second `AddRemote` with the same name fails;
  - `InitBare` on an existing **non-bare** repository fails.
- [ ] **Step 2: Run the test and watch it fail.** Run `go test -tags integration ./internal/gitcli/ -run TestIntegrationRepoInitBareAndAddRemote -count=1`.
- [ ] **Step 3: Implement**, modeled on `RemoteURL` in `refs.go` (`c.run(ctx, runRequest{op, dir, args})`, `newFailure(...)`, `stderrExcerpt`, `withExitCode`). Add two op labels, `"init-bare"` and `"add-remote"`.
  - **`InitBare`:**
    1. A non-absolute path is `KindInvalidRequest`.
    2. If the path exists and is a directory, run `rev-parse --is-bare-repository` there. Output `true` returns nil. Anything else, or a non-directory, is `KindInvalidRequest` ("not a bare repository").
    3. Otherwise `os.MkdirAll(filepath.Dir(path), 0o755)` and run `git init --bare -q <path>` with `dir` = the parent. GIT_DIR is scrubbed, so the path must be an argument.
    4. A non-zero exit is `KindCommandFailed` with the stderr excerpt.
  - **`AddRemote`:**
    1. `validateRemoteName(name)`.
    2. Refuse an empty URL, a leading `-`, or a NUL or newline.
    3. Run `git remote add <name> <url>` in `repo.PrimaryWorktree`.
    4. A non-zero exit, including an already-existing remote, is `KindCommandFailed`. Callers compare URLs first; docket never rewrites a remote.
- [ ] **Step 4: Run the test and confirm it passes.** Re-run the Step 2 command and `go vet ./internal/gitcli/`. Expected: PASS.
- [ ] **Step 5: Commit.** Message: `feat(gitcli): add InitBare and AddRemote for the private metadata remote`.

---

### Task 7: The `# dckt:` exclude block

**Files:**
- Create: `internal/reposetup/exclude.go` and `exclude_test.go`.
- Reuse `gitignore.go`'s unexported helpers (`gitignoreMarkersMalformed`, `stripGitignoreBlock`, `splitLines`, `joinLines`) as they are. Do not change `.gitignore` behavior.

**Interfaces:**

```go
const (ExcludeStart = "# dckt:start"; ExcludeEnd = "# dckt:end")
type MalformedExcludeError struct{}
func EnsureExcludeBlock(current []byte) (out []byte, changed bool, err error)
func ValidExcludeBlock(current []byte) bool
```

The canonical block is `# dckt:start\n.worktrees/\n# dckt:end\n`.

- [ ] **Step 1: Write the failing tests.**
  - **User lines and idempotence.** With `in` = `"# git ls-files …\n*.swp\n"`:
    - the output starts with `in` byte-for-byte and contains the canonical block;
    - `ValidExcludeBlock(out)` holds;
    - a second call returns `changed=false` and identical bytes.
  - **Nil input.** `nil` gives exactly the canonical block, with `changed=true`.
  - **Malformed markers.** `"# dckt:start\n.worktrees/\n"`, `"# dckt:end\n# dckt:start\n"`, and `"# dckt:start\n# dckt:start\n# dckt:end\n"` each give an error.
  - **Stale body.** `"keep\n# dckt:start\nold/\n# dckt:end\n"` is rewritten: it keeps `keep\n` and drops `old/`.
  - **Neutral spelling.** The output contains no `docket`, case-insensitive.
- [ ] **Step 2: Run the tests and watch them fail.** Run `go test ./internal/reposetup/ -run Exclude -count=1`.
- [ ] **Step 3: Implement** by mirroring `EnsureGitignoreBlock`:
  1. Validate marker balance and order first. On failure, return `&MalformedExcludeError{}` with the input untouched.
  2. Strip the existing block.
  3. Trim trailing blank lines.
  4. Add a separating newline if the remaining content is non-empty.
  5. Append the block.

  `changed` is `!bytes.Equal(out, current)`. `ValidExcludeBlock` holds when the markers are well-formed and a rebuild equals the input.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/reposetup/ -count=1`. The existing gitignore tests must be unchanged.
- [ ] **Step 5: Commit.** Message: `feat(reposetup): manage a neutral # dckt: block in .git/info/exclude`.

---

### Task 8: `docket repository init --private`

**Files:**
- Create: `internal/app/repository_init_private.go`
- Create: `internal/app/repository_init_mode_test.go` (default tag)
- Create: `internal/app/repository_private_integration_test.go` (`//go:build integration`, prefix `TestIntegrationRepoSetup`; confirm its shard with `grep -l TestIntegrationRepoSetup tests/test_go_integration_app_*.sh`)
- Modify: `internal/app/repository_init.go`, `repository_configure_tests.go`, `repository_migrate.go`
- Modify: `internal/cli/repository.go` and the CLI tests that stub `repositoryInitRunner`
- Modify: the capability goldens (`grep -rn '"repository.init"' internal cmd --include='*_test.go'`)

**Interfaces:**

```go
type InitOptions struct { Private, Shared bool; MetadataRemote string }
func RunRepositoryInit(ctx context.Context, d SetupDeps, o InitOptions) RepositoryOpResult
func decideInitMode(detected layout.Mode, state reposetup.State, o InitOptions, effective string) (layout.Mode, string /* refusal, "" = admitted */)
func repoConfigTarget(sc setupContext) (absPath, display string, pending bool) // private: (lay.ConfigPath, layout.PrivateConfigDisplay, false); shared: (<primary>/.docket.yml, ".docket.yml", true)
func newPrivateInitRepo(t *testing.T, files map[string]string) (*initRepo, string /*data home*/) // integration helper
func (r *initRepo) runInitWith(t *testing.T, o InitOptions) RepositoryOpResult
```

- [ ] **Step 1: Write the failing tests.**

  **`TestDecideInitMode`** is table-driven. Each row is `(detected, state, opts, effective)` → result:
  - `(Shared, Fresh, {}, "shared")` → Shared.
  - `(Shared, Fresh, {}, "private")` → Private.
  - `(Shared, Fresh, {Shared}, "private")` → Shared.
  - `(Shared, Fresh, {Private}, "shared")` → Private.
  - `(Shared, Healthy, {}, "private")` → Shared.
  - `(Shared, Healthy, {Private}, _)` → refuse.
  - `(Private, Healthy, {}, "shared")` → Private.
  - `(Private, Healthy, {Shared}, _)` → refuse.
  - `(Shared, Fresh, {Private, Shared}, _)` → refuse.
  - `(Shared, Fresh, {MetadataRemote: "/x.git"}, "shared")` → refuse.

  **Integration fixture.** `newPrivateInitRepo` copies `newInitRepo` but commits **no `.docket.yml`**. It sets `XDG_DATA_HOME` to `testsupport.TempDir(t)` and returns it. `runInit` passes `InitOptions{}`.

  **Integration tests:**
  - **`…PrivateFreshInitCreatesNeutralLayout`.** Init returns `applied`. Assert:
    - `.git/dckt/config.yml` holds `visibility: private`;
    - `.git/info/exclude` satisfies `ValidExcludeBlock`;
    - the `dckt` remote's URL is `<data>/dckt/<OwnerRepo(origin)>/remote.git`, which has `refs/heads/dckt`;
    - origin has neither `docket` nor `dckt`;
    - the worktree list shows a path under `<data>/dckt/`;
    - there is no `.docket`, `.docket.yml`, `.gitignore`, or `.docket.local.yml` in the root;
    - `git status --porcelain` is empty.
  - **`…PrivateRerunIsNoOp`.** A second init returns `no-op`. `RunRepositoryPrepare` then gives a no-op disposition, `MetadataRemote == "dckt"`, and `MetadataWorktreePath` = the layout's checkout.
  - **`…PrivateSecondCloneAdoptsSharedStore`:**
    - clone A runs `init --private` and `ChangeCreate(validChangeCreateRequest())`;
    - clone B, a second clone of the same origin, runs `init --private`;
    - the two `dckt` URLs are equal and the checkout paths differ;
    - B's checkout has A's change file;
    - A's checkout HEAD is unchanged.
  - **`…PrivateRemoteURLConflictRefuses`.** After `git remote add dckt /elsewhere.git`, init refuses. The text names both URLs, and the remote is unchanged.
  - **`…PrivateMetadataRemoteFlag`.** With `--metadata-remote <tmp>/backup.git`, pre-initialized bare by the test, the `dckt` URL equals the flag and no default `remote.git` is created.
  - **`…InitModeFlagsRefuseSwitch`.** A shared `newInitRepo` init followed by `--private` refuses and creates no `.git/dckt`.
  - **`…GlobalPrivateDefault`** (acceptance 3), with `XDG_CONFIG_HOME/docket/config.yml` = `visibility: private`:
    - a fresh private fixture with no flags inits private;
    - a committed `.docket.yml` with `visibility: shared` inits shared;
    - `--shared` inits shared;
    - an already-healthy shared repository stays shared, and its check has **no** `visibility-mismatch` finding.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**

  Implement `decideInitMode` in exactly this order:
  1. Both flags → refuse ("--private and --shared are mutually exclusive").
  2. Detected private → refuse on `--shared` ("this repository is already private; init never switches a repository's visibility"), else Private.
  3. A non-fresh state → refuse on `--private` or `--metadata-remote` ("…already set up as shared…"), else Shared.
  4. Otherwise the mode is the effective value, overridden by a flag.
  5. A non-private result with `--metadata-remote` → refuse ("--metadata-remote applies only to a private repository").

  **Refusals.** Mutually exclusive flags are `ResultInvalidInput`, returned before gather and built like `RunRepositoryConfigureTests`' empty-`--command` refusal. Mode switches use `initRefusal`. Never print `set-visibility`; it does not exist until #533.

  **`RunRepositoryInit`:**
  1. Gather.
  2. `initGuard`.
  3. Debris sweep.
  4. `decideInitMode(sc.layout.Mode, cls.State, o, sc.cfg.Visibility.Value)`.
  5. Private goes to `runPrivateInit(ctx, d, sc, cls, o, debris)`.
  6. Shared is unchanged.

  **`runPrivateInit`.** Every step is idempotent, and `git status` stays clean throughout:
  1. **Exclude:** run `EnsureExcludeBlock` on `<common>/info/exclude` (absent means empty). If it changed, `MkdirAll` and write with 0644. A malformed block is a refusal that leaves the file untouched.
  2. **Config:** write `layout.PrivateConfigPath(common)` (after `MkdirAll`), starting from the existing bytes or `"visibility: private\n"`, and apply `reposetup.TestPolicyEdit(sc.cfg, existing, newOSTree(primary))`. **From here the repository detects as private.**
  3. **Layout:** `lay, _ := resolveLayout(ctx, d.Git, sc.repo)` must be Private.
  4. **Bare remote:** `want` is `o.MetadataRemote`, or else `lay.DefaultBareRemote` with `d.Git.InitBare(ctx, want)`. A flag URL never creates the default remote.
  5. **The `dckt` remote:** check `RemoteURL`.
     - `KindRemoteUnavailable`: `AddRemote(want)`.
     - Equal to `want`: no-op.
     - Otherwise refuse: "the dckt git remote already points at <got>, not <want>; resolve it by hand (docket never rewrites a remote)".
  6. **Branch:** `publishOrAdoptMetadataRoot(ctx, d.Git, sc.repo, metadataRemote(lay), metadataRef(lay), sc.sourceRevision, sc.defaultBranch)`. It creates the branch create-only, or adopts a verified init lineage, which is the second-clone path.
  7. **Worktree:** `MkdirAll(lay.CheckoutsDir)`, then `ensureMetadataWorktree(…, lay.MetadataWorktree, metadataRef(lay), tip)`, then `DisableWorktreeHooks(lay.MetadataWorktree)`.
  8. **Skip** `.gitignore` and the dispatch surfaces. A comment explains that surfaces in private repositories belong to the follow-up change that keeps docket out of commits.
  9. **Report:** re-run `GatherSetupFacts(ctx, d, false)`, then `augmentCheckFacts` when the metadata branch is present, then `Classify`. Report that state. The result is `applied` if anything changed, else `no-op`. Human text: `initialized private docket metadata: branch dckt on <url>, worktree <path>`.

  **Every failure after step 2** appends: "`.git/dckt/` now exists, so this repository reads as private; re-run `docket repository init` to finish (every step is idempotent)." That is the presence-encoded-state rule.

  **Config target:**
  - Generalize `writePendingDocketYML` into `writeRepoConfig(absPath, render) (bool, error)`.
  - `ensureTestPolicyConfig` and `ensureExplicitTestCommand` write to `repoConfigTarget`. Shared text is unchanged.
  - Private configure-tests reports no pending path and says `wrote .git/dckt/config.yml`.

  **Migrate:** refuse right after gather when private, with "migrate converts a legacy repository to the shared layout; this repository is private and has nothing to migrate".

  **CLI:** `repositoryInitRunner` becomes `func(ctx, d, o app.InitOptions)`. The init short help is "Initialize the docket metadata branch and its metadata worktree (shared by default; --private keeps it on this machine)". The flags:
  - `--private`: "keep docket's metadata on this machine: a `dckt` branch pushed to a bare repository under ~/.local/share/dckt, with nothing docket-named in the repository (overrides the visibility setting)"
  - `--shared`: "keep docket's metadata on a `docket` branch pushed to origin with a .docket/ worktree (overrides the visibility setting)"
  - `--metadata-remote <url>`: "with --private: push the dckt branch to this git `url` instead of the default bare repository"

  Update the stubs and the goldens; `buildSignature` projects the flags.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/cli/ -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoSetup' -count=1`. The shared init tests must be unchanged.
- [ ] **Step 5: Mutation-test.** Make the private branch of `decideInitMode` return Shared. `TestDecideInitMode` and `…PrivateRerunIsNoOp` must go red. Restore from the backup.
- [ ] **Step 6: Commit.** Message: `feat(repository): init --private publishes the metadata branch to a local bare remote with neutral naming`.

---

### Task 9: Private-aware check, prepare and repair, plus the mismatch and orphaned-checkout findings

**Files:**
- Create: `internal/app/repository_private_findings.go` and `_test.go` (default tag)
- Modify: `internal/app/repository_check.go`, `repository_repair.go`, and `internal/reposetup/health.go`
- Extend: `repository_private_integration_test.go`

**Interfaces:**

```go
const (FindingVisibilityMismatch = "visibility-mismatch"; FindingOrphanedCheckout = "orphaned-checkout")
func visibilityMismatchFinding(lay layout.Layout, eff config.Effective) *reposetup.Finding
func scanOrphanedCheckouts(lay layout.Layout) ([]string, error) // errors returned, never read as "none"
```

- [ ] **Step 1: Write the failing tests.**
  - **`TestVisibilityMismatchOnlyFromRepositoryLevelFiles`** builds an `Effective` with `Visibility` `Value`/`Explicit`/`Provenance.Layer` set (use the real field names). Expect a finding exactly when:
    - a shared repository has `LayerRepository` saying private;
    - a private repository has `LayerRepository` saying shared;
    - a shared repository has `LayerRepositoryLocal` saying private.

    Expect **none** for a global private, an agreeing file, or a built-in default.
  - **`TestScanOrphanedCheckouts`** uses a `PrivateLayout` with a temp data home and checkout dirs whose `.git` file is `gitdir: <p>`:
    - the own checkout with a missing gitdir is never reported;
    - `other-11111111` with an existing gitdir is kept;
    - `moved-22222222` with a missing gitdir is reported;
    - the result is exactly `[moved]`;
    - shared mode, or a missing checkouts dir, gives `nil, nil`.
  - **Integration tests:**
    - **`…PrivateCheckHealthy`.** After init and configure-tests (`--command true`), the state is `healthy` and no finding mentions `.gitignore`, `.docket.yml` or `.docket`.
    - **`…PrivateCheckFlagsMissingExclude`.** With the block stripped, check reports `committed-ignore-valid`, with text naming `.git/info/exclude`.
    - **`…PrivateVisibilityMismatch`** (acceptance 4). With the config's `visibility:` line rewritten to `shared` and other keys kept, check gives one mismatch naming `.git/dckt/config.yml`, and the state is unchanged.
    - **`…PrivateBothLocalConfigsRefuse`** (acceptance 4). With a root `.docket.local.yml`, `RunRepositoryCheck` gives `unsupported-config` and `Status` gives invalid input. Both texts name both paths.
    - **`…PrivateMovedCloneOrphanPruned`** (acceptance 4):
      1. Init clone A, then `os.Rename` it.
      2. Check from the moved clone reports `orphaned-checkout` with `Ref` = the old checkout.
      3. The repair preview lists it.
      4. Repair, `Authorized` with the previewed `ExpectedSource`, removes it.
      5. A re-check is clean.
      6. `RunRepositoryPrepare` then either attaches the new checkout or reports the held-elsewhere finding (remedy `git worktree prune`). Assert whichever it does and comment which.
- [ ] **Step 2: Run the tests and watch them fail.**
- [ ] **Step 3: Implement.**
  - **`visibilityMismatchFinding`:**
    - Return nil unless `Visibility.Explicit` holds, the layer is `LayerRepository` or `LayerRepositoryLocal`, and `layout.Mode(value) != lay.Mode`.
    - Otherwise return a warning with `Ref` = `Provenance.Source`.
    - Message: "<source> sets visibility: <value>, but this repository is set up <mode>; the setting does not move an existing repository."
    - Remedy: "Change visibility in <source> to <mode>, or switch the repository with `docket repository set-visibility <value>`." The spec mandates this remedy, and the command lands with #533; the edit-the-file option, valid today, comes first.
  - **`scanOrphanedCheckouts`:**
    - Shared mode, or an absent `CheckoutsDir`, gives `nil, nil`.
    - For each subdir except the own checkout, parse the `gitdir:` in its `.git`.
    - An unreadable or unparsable `.git` is skipped: it is not proven ours.
    - Report the dir when the gitdir `os.Stat` gives `ErrNotExist`.
    - Return any other error.
  - **`RunRepositoryCheck`:** after classification, append:
    - the mismatch finding;
    - one `orphaned-checkout` warning per orphan, with `Ref` = the path, message "A metadata checkout whose clone moved or no longer exists.", remedy "Run `docket repository repair` to remove it; its data lives in the metadata remote.", and `Repairable: true`.

    A scan error adds one unverified warning instead. Neither finding changes the state.
  - **`augmentCheckFacts` in private mode:**
    - `CommittedIgnoreBlock` comes from `ValidExcludeBlock(<common>/info/exclude)`; a read error is Unknown, with a diagnostic.
    - `LegacyConfigKey = Absent`.
    - `pendingReviewPaths` skips `.gitignore` and `.docket.yml`.
    - `TestConfigFinding` reads bytes from `repoConfigTarget`.
  - **`health.go`:** when `f.Private`, the `CondCommittedIgnoreValid` message is "The `# dckt:` ignore block is missing from .git/info/exclude." and the remedy is "Run `docket repository init` to restore it." Init is idempotent, so that remedy is valid.
  - **Repair:**
    - The plan gains the scanned orphans; a scan error is a refusal.
    - The preview prints `remove orphaned checkout: <path>`.
    - Execute re-verifies each previewed path is still an orphan, then calls `os.RemoveAll` (decide-and-act-on-the-same-copy).
    - An orphan-only plan is `applied`.
    - The prune is independent of the metadata push.
    - If `phaseAlreadyMigrated` blocks a private repository whose metadata branch is present, admit it.
- [ ] **Step 4: Run the tests and confirm they pass.** Run `go test ./internal/app/ ./internal/reposetup/ -count=1` and `go test -tags integration ./internal/app/ -run '^TestIntegrationRepoSetup' -count=1`.
- [ ] **Step 5: Mutation-test.** Restore after each:
  - Admitting `LayerGlobal` reddens the global row.
  - Reporting every non-own checkout reddens `TestScanOrphanedCheckouts`.
  - Forcing private `CommittedIgnoreBlock` to Present reddens `…FlagsMissingExclude`.
- [ ] **Step 6: Commit.** Message: `feat(repository): private-aware check/prepare/repair with visibility-mismatch and orphaned-checkout findings`.

---

### Task 10: Acceptance tests: private lifecycle, concurrency, and resolver mutations

**Files:**
- Create: `internal/app/workflow_private_integration_test.go` (prefix `TestIntegrationWorkflowLifecycle`)
- Create: `internal/app/app_concurrency_private_race_integration_test.go` (prefix `TestRaceIntegrationAppConcurrency`, race shard)
- Modify: `internal/app/workflow_e2e_test.go` and `status_git_test.go`
- Maybe modify: `tests/runtime-budgets.tsv`

**Interfaces.** `gitRepo` gains `meta string`, the bare repository holding the metadata branch; **every builder sets it explicitly**. The driver is:

```go
func driveClaimToImplemented(t *testing.T, repo *gitRepo, branch, ghBin string, id int, slug, recPath, specPath string, entries ...workflowEntry)
```

It is the post-build half of `runClaimToImplemented`, from context through run.verify, for an already build-ready, spec-linked change. Its metadata reads go to `repo.meta`.

- [ ] **Step 1: Refactor the driver with no behavior change.**
  - Split `runClaimToImplemented`.
  - In the driver, the metadata reads (`originTip`, `blobRevisionAt`, `originFile`, `assertEngineOnlyMetadataCommits`; re-derive them with `grep -n 'repo.origin' internal/app/workflow_e2e_test.go`) use `repo.meta`. Feature-branch reads stay on `origin`.
  - Run `go test -tags integration ./internal/app/ -run '^TestIntegrationWorkflowLifecycle' -count=1`. Expected: PASS.
  - Commit with message `test(app): extract the claim-to-implemented driver and name the metadata remote explicitly`.
- [ ] **Step 2: Write `TestIntegrationWorkflowLifecyclePrivateInitToImplemented`** (acceptance 1).
  - **Setup:**
    - `DOCKET_SCRIPTS_DIR=""`, `buildFakeGH`, and `newPrivateInitRepo`.
    - Assert the fixture's paths contain no `docket`.
  - **Init and config:**
    - `init --private`.
    - `RunRepositoryConfigureTests` with `Command: passingGateScript(t)`.
    - Record origin's `refs/heads`.
  - **Create and groom:**
    - `node := planningDepsFor(t, r.invocation)`.
    - `ChangeCreate(validChangeCreateRequest())`.
    - `ChangeGroom` to build-ready with a spec (`workflowMetadataSpec`). Mirror the real-git spec groom in `change_groom_integration_test.go` for `ChangeGroomRequest`'s fields.
  - **Drive:** `driveClaimToImplemented` with `meta` = `lay.DefaultBareRemote`, where `lay` comes from `resolveLayout` with a real client, on branch `dckt`.
  - **Assertions:**
    - origin gained exactly one `refs/heads/feat/…`;
    - every `git status --porcelain --ignored` line is `!! .worktrees/`;
    - `git branch` has `dckt` and not `docket`;
    - `git remote` has `dckt`;
    - `assertNoDocketPathUnder(t, r.invocation)` walks the whole clone, **including `.git`**, and fails listing every path component containing `docket` (case-insensitive).
  - **If the walk finds a docket-named path** outside the spec's enumerated names (e.g. `refs/docket/…` or a `.docket-*` transient), return `NEEDS_ESCALATION` naming it. Never loosen the walk.
- [ ] **Step 3: Write `TestRaceIntegrationAppConcurrencyPrivateClaimsOneAppliesOneContends`** (acceptance 2).
  - Identical ungated claims are an idempotent **replay**, because they share `claimRequestID` and digest. So each contender uses its own run context.
  - Clone A runs `init --private` and seeds a build-ready change (helper `seedBuildReadyPrivate(t, node) (id int, rev string)`).
  - Clone B, a second clone, runs `init --private` and adopts the store (helper `clonePrivate`).
  - `mintRunTrackerWithHash(t, dir, runTrackerHashToken(tok), false)` runs with `ctx-a` in A and `ctx-b` in B; each lands under that clone's `.git/dckt/`.
  - Two goroutines, gated on a `start` channel, call `ChangeClaim{ID, Revision: rev, RunContext: tok}`.
  - Assert exactly one `applied` and one `contended`.
  - Do not call `t.Parallel()`, because `planningDepsFor` uses `t.Setenv`.
  - If the loser maps to something else, report the actual mapping via `NEEDS_ESCALATION`. Never loosen the oracle.
- [ ] **Step 4: Run the acceptance tests.**
  - `go test -tags integration ./internal/app/ -run '^TestIntegrationWorkflowLifecyclePrivate' -count=1`
  - `go test -tags integration -race ./internal/app/ -run '^TestRaceIntegrationAppConcurrencyPrivate' -count=1`

  Expected: PASS.
- [ ] **Step 5: Resolver mutations** (acceptance 6). Back up `internal/layout/layout.go` into a `mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"` dir. For each mutation, run the lifecycle test with `-count=1`, expect red, and `cp` the backup back:
  - `MetadataRemote: SharedRemote` reddens the origin-heads assertion, or init fails.
  - `MetadataBranch: SharedName` reddens the `git branch` assertion.
  - `StateName` always returning `SharedName` makes the walk fail on `.git/docket/…`.
  - `MetadataWorktree` set to `<primary>/.docket` makes the status or walk assertion fail.

  Record each red message.
- [ ] **Step 6: Check the budgets.** Run serially the workflowlifecycle shard, the concurrency shard, and the `TestIntegrationRepoSetup` shard, and compare each to `tests/runtime-budgets.tsv`. If the worst serial reading exceeds a row, raise it to the next multiple of 5 plus 5s. Record every margin as a number.
- [ ] **Step 7: Commit.** Message: `test(app): private-visibility acceptance — lifecycle, concurrent claims, resolver mutations`.

---

### Task 11: Skill prose, pins, budgets, and the skill guard

**Files:**
- `skills/` docket-adr, docket-auto-groom, docket-build, docket-build-task, docket-convention (+ `references/close-out.md`), docket-finalize-change, docket-groom-next, docket-implement-next (+ `references/edge-paths.md`), docket-new-change, docket-review, docket-status
- `internal/assets/embedded/`, regenerated
- `internal/repoguard/{prose_contracts,budgets,layout_guard}_test.go`

**Interfaces.** These files use the prepare-context keys `metadata_worktree_path`, `metadata_remote`, and `metadata_tracking_ref` (Task 4). **Never name `metadata_branch` or `metadata_branch_revision`.** The `oneLayoutAbsent` pin forbids that substring in 11 of these files to guard the retired config key, and `metadata_tracking_ref` removes the need. This narrows spec section 7; record it in the results file.

- [ ] **Step 1: Derive the sites.** Run `grep -rn -E '\.docket/|--repo-dir \.docket|git -C \.docket|origin/docket' skills/ agents/`. Grooming found about 48 sites, all in skills.
- [ ] **Step 2: Write the failing guard `TestSkillsSpellNoMetadataLiterals`.**
  - It scans `skills/**/*.md` and `agents/*.md`.
  - It fails on `--repo-dir .docket`, `git -C .docket`, `origin/docket`, and `.docket/` (when not followed by `.yml`).
  - **One exemption:** `skills/docket-convention/SKILL.md` may hold one `.docket/`, on a line that also contains `shared` (prose-guard-binds-phrase-to-claim).
  - Population floor: at least 13 files.
  - Run it. Expected: FAIL.
- [ ] **Step 3: Rewrite the prose.** Read each sentence as a worker in an unknown repository.
  - **Worktree:** "the `.docket/` worktree" becomes "the metadata worktree (`metadata_worktree_path` in the `repository.prepare` context)": the full form once per file, then the short form.
  - **Paths:**
    - `--repo-dir .docket` becomes `--repo-dir <metadata_worktree_path>`;
    - `.docket/<path>` becomes `<metadata_worktree_path>/<path>`;
    - `git -C .docket …` becomes `git -C <metadata_worktree_path> …`.
  - **Remote:** `origin/docket` becomes "the metadata remote (`metadata_remote`)".
  - **implement-next HEAD assertion:** `git -C <metadata_worktree_path> rev-parse HEAD` equals `git -C <metadata_worktree_path> rev-parse <metadata_tracking_ref>`.
  - **docket-adr:** commit in the metadata worktree, then `git -C <metadata_worktree_path> push <metadata_remote> HEAD`.
  - **auto-groom:** "never inside the metadata worktree".
  - **Convention:**
    - Startup step 3 names the three keys.
    - Directory layout: "`<primary>/.docket/` in a shared repository (gitignored, deliberately not under `.worktrees/`), or a checkout outside the clone in a private one; skills always use `metadata_worktree_path`". This is the single exempt line.
    - Branch model: metadata is pushed to the metadata remote, which is `origin` when shared and the local `dckt` bare remote when private, and the context values are authoritative.
    - Bootstrap guard: `DOCKET` = the metadata branch exists on the metadata remote or locally.
    - Add one private sentence: "A private repository (`.git/dckt/` present) spells the metadata branch and remote `dckt`, keeps its config in `.git/dckt/config.yml`, and never reads `.docket.yml`."
  - Other "the `docket` branch" phrasing stays. It is a residual for the results file.
- [ ] **Step 4: Move the pins in the same task.**
  - The `align_0502_autogroom_draft` present phrase becomes `never inside the metadata worktree`.
  - First run `grep -rn -- '\.docket' internal/repoguard/*_test.go tests/` and repoint any other prose assert.
  - Run `go generate ./internal/assets/`.
  - Set the `skillBudgets` rows to the measured counts (the file's ratchet rule).
- [ ] **Step 5: Run the tests and confirm they pass.** Run `go test ./internal/repoguard/ ./internal/assets/ -count=1`, including `TestAlignmentContracts`, `TestProseContracts`, `TestSkillSizeBudgets`, `TestEmbeddedMatchesAuthored`, and the new guard.
- [ ] **Step 6: Mutation-test.** Re-add `--repo-dir .docket` to docket-new-change. The guard must go red. Restore from the backup.
- [ ] **Step 7: Commit.** Message: `docs(skills): key metadata worktree and remote on the prepare context instead of .docket/ and origin/docket`.

---

### Final: whole-suite gate

- [ ] **Step 1: Check the twins.** Run `go generate ./internal/assets/`, then `git status --porcelain`. Expected: empty.
- [ ] **Step 2: Run the build gate.** Run whatever `build.test_command` resolves to; read it from config, never from this plan. Expected: green. Act on `SERIAL CONFIRMED OVER BUDGET:`. Record any `BUDGET WATCH:` or `PARALLEL-SENSITIVE:` line.
- [ ] **Step 3: Run the residue greps.** Both must return nothing:
  - `grep -rn --include='*.go' -E 'MetadataBranchName|docketWorktreeName' internal cmd`
  - `grep -rn --include='*.go' -E '"refs/heads/docket"|"origin/docket"' internal cmd | grep -v _test.go`
- [ ] **Step 4: Collect notes for the results file.** The parent writes it, not a build worker. It needs:
  - every mutation and its red message;
  - the shard margins as numbers;
  - the deviations:
    - `metadata_tracking_ref` instead of naming `metadata_branch*` in skills;
    - `.git/info/exclude` as the private ignore guarantee;
  - the residuals:
    - "docket branch" wording in health text and skills;
    - any `refs/docket/…` finalize refs;
    - the install repository phase can still write surfaces into a private repository (owned by the follow-up change that keeps docket out of commits);
  - the metadata-branch note: the acceptance test proves the bare remote's `dckt` shape, and this repository's own `docket` branch is untouched.
