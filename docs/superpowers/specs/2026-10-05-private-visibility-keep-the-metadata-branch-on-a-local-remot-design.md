<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0531 — Keep the metadata branch on a local remote with neutral naming](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0531-private-visibility-keep-the-metadata-branch-on-a-local-remot.md)**
<!-- docket:backlink:end -->

# Keep the metadata branch on a local remote with neutral naming: design

Change #531, groomed interactively on 2026-10-05. Second in the private-visibility series (#529–#533). It builds in parallel with #530; #532 and #533 depend on both.

## Summary

Some host repositories forbid a `docket` branch, root-level tool files, or any docket-specific content, yet one user should still be able to run docket there, privately, on their own machine. This change adds a **private** visibility. The repository keeps docket's one metadata layout (ADR-0099: an orphan metadata branch reached through a metadata worktree), but:

- the metadata branch is pushed to a **bare repository on the user's machine** instead of `origin`;
- every per-repo name docket writes is spelled **`dckt`**;
- **nothing** docket-named sits in the repository root.

After `docket repository init --private`, a private clone looks like this:

```
repo/
├── .git/
│   ├── config              # remote "dckt" → the bare repo
│   ├── info/exclude        # "# dckt:start" block holding .worktrees/
│   └── dckt/               # config.yml + run tracker, transactions, workspaces, gate drives, …
├── .worktrees/             # feature worktrees (neutral name)
└── src/ …

git branch → main, dckt, feat/…        git remote → origin, dckt

${XDG_DATA_HOME:-~/.local/share}/dckt/<owner>-<repo>/
├── remote.git/             # bare metadata remote
└── checkouts/<clone-id>/   # the metadata worktree
```

## Evidence gathered at grooming

- **Remote.** `const originRemote gitcli.RemoteName = "origin"` (`internal/app/status_git.go`) has 57 non-test uses across 28 files. `setupRemote()` (`internal/app/repository_facts.go`) returns it, and its comment says "minting a remote config key is out of scope". Metadata uses include:
  - every `transaction.Engine` request (about 27 call sites);
  - `loadOperationalContext` → `fetchPinnedRevision` (every read, including `run.start` and `run.verdict`);
  - the sweep session;
  - `gatherRepoFacts`;
  - the check and prepare augments;
  - `publishOrAdoptMetadataRoot` (init);
  - migrate (5 sites) and repair (2 sites).
- **Code-side uses stay on `origin`:** default-branch resolution, integration fetches, workspace publish/rewrite, finalize, cleanup, maintenance, and the `run.verify` feature-head probe.
- **Locking.** The engine's lease push (`--force-with-lease`) is the only thing serializing metadata writers, and no local lock exists. A lease push to a path remote takes the bare repository's ref lock atomically, so the compare-and-swap holds for concurrent local loops. Removing the remote entirely would need a new local compare-and-swap, which was rejected.
- **Remote names.** `validateRemoteName` (`internal/gitcli/types.go`) requires a configured remote (`git remote get-url`), and `GIT_CONFIG*` are scrubbed. The metadata remote must therefore be a real remote in `.git/config`.
- **Branch name.** `reposetup.MetadataBranchName = "docket"` (`internal/reposetup/branch.go`) is the single spelling. Metadata commit receipts (`Docket-Result`) don't record the branch name; one was decoded at grooming to `{"id","op","path","slug"}`.
- **Worktree.** `docketWorktreeName = ".docket"` (`internal/app/repository_facts.go`), `initplan.go`, and the health refs. Skills carry 18 literal `.docket/` references in 9 files, plus `origin/docket` assertions (implement-next's `.docket` HEAD equals `origin/docket` check, and docket-adr's "push `origin/docket`").
- **State folder.** `filepath.Join(<common-dir>, "docket", …)` appears in about 20 places across 10 files: run tracker, run-tracker resume, agent guardian, transactions, workspaces manifest, gate drives/scopes/budgets/worktree locks, suite budget state, empty hooks, and the install record.
- **Config.** `config.LoadFilesystemSources` (`internal/config/fs.go`) reads `.docket.yml`, `.docket.local.yml`, and the global file. Operational commands read `.docket.yml` from the pinned origin default-branch blob (`readPinnedOptionalBlob` in `operational_context.go`). The shared-setting guard (`applySharedSettingGuard` / `isMachineLayer` in `internal/config/resolve.go`) ignores repository-identity keys in machine layers.
- **Ignore block.** `reposetup.EnsureGitignoreBlock` writes the managed `.gitignore` block. `CondCommittedIgnoreValid` requires it to be **committed**, and `.git/info/exclude` is not accepted today.
- **Data root.** The install's data root resolves `XDG_DATA_HOME` (`internal/install/roots.go`).

## Decisions (settled with the human)

1. **`visibility: shared | private` is an ordinary config key.** It can be set in any layer with normal precedence (repo-local > committed > global > built-in `shared`). The human rejected any special scoping twice: config keys must behave uniformly.
2. **`init` uses the effective value, and flags override it.** `docket repository init` sets the repository up in the effective mode; `--private` and `--shared` override it. A global `visibility: private` therefore makes new repositories on that machine private, and a repository's own file overrides that.
3. **A repository's actual mode is its state, not its config.** A `.git/dckt/` folder means private. Editing config never moves a repository; #533's `set-visibility` does that. When a **repository-level** file states a different mode than the state, `repository check` reports it (report-only). A global value never triggers that finding.
4. **Every per-repo name docket writes is spelled `dckt` in private mode:** the state folder, the config file location, the metadata branch, the git remote, the bare-remote and checkout store, and the exclude-block markers. Not renamed: the `docket` binary, its install paths, `~/.config/docket/config.yml`, agent files, and `Docket-*` trailers inside metadata commits.
5. **The bare remote lives outside the clone,** at `${XDG_DATA_HOME:-~/.local/share}/dckt/<owner>-<repo>/remote.git`, named from origin's URL. It survives re-cloning, and two clones of one repository share one backlog. `--metadata-remote <url>` points it at any git URL instead, for example a private personal repository for backup. The `dckt` git remote's URL *is* the setting; there is no `metadata_remote` config key.
6. **The metadata worktree lives outside the clone** at `…/<owner>-<repo>/checkouts/<clone-id>/`. `<clone-id>` is the clone's folder name plus the first 8 hex characters of sha256 of the clone's absolute path, for example `api-3f9c2a1b`. It is stateless: a moved clone gets a fresh checkout, and the old one is reported and pruned. Nothing is lost, because every write lives in the bare remote.
7. **Private config comes from `.git/dckt/config.yml` plus the global file.** A committed `.docket.yml` is never read, and repository-identity keys are honored from `.git/dckt/config.yml`. With a single clone the guard's rationale (every clone must agree) does not apply; this relates to ADR-0019. If both `.docket.local.yml` and `.git/dckt/config.yml` exist, docket refuses and names both paths.
8. **Sparse documentation.** The private feature is documented only in `.docket.example.yml` (and its embedded twin), command help, and skills. There are no `docs/` pages.

## Design

### 1. The `visibility` key

- A schema row: enum `shared|private`, default `shared`, all layers, ordinary precedence. `docket diagnostic config` shows the winning layer.
- `.docket.example.yml` documents it in two or three lines. Its correspondence guard requires that.
- A new finding (report-only) for a repository-level file that disagrees with the repository's state. Its remedy names `docket repository set-visibility <mode>`, which arrives with #533.

### 2. Mode detection and resolved values

One resolver decides the mode from state: `.git/dckt/` exists → private. Four resolved values replace the hard-coded spellings:

| Value | Shared | Private |
|---|---|---|
| metadata remote | `origin` | `dckt` |
| metadata branch | `docket` | `dckt` |
| state folder | `<git-common-dir>/docket/` | `<git-common-dir>/dckt/` |
| metadata worktree | `<primary>/.docket` | `<data>/dckt/<owner>-<repo>/checkouts/<clone-id>/` |

Rules for the replacement:

- Derive every replacement site from a whole-repository grep (`originRemote`, `setupRemote`, `MetadataBranchName`, `docketWorktreeName`, `Join(… "docket" …)` under the common dir), never from this list.
- Sort each site into metadata versus code-side. Only metadata sites change; code-side remote work stays on `origin`.
- `fetchPinnedRevision`'s metadata fetch is split from the default and integration fetches.
- `repository.prepare`'s context gains `metadata_remote` (it already returns `metadata_branch` and `metadata_worktree_path`), so skills key on context values rather than spellings.
- `<owner>-<repo>` is derived from origin's URL: lowercase, with non-alphanumeric runs collapsed to `-`.

### 3. `docket repository init --private`

The steps run in an order that keeps `git status` clean throughout:

1. Write the `# dckt:start` / `# dckt:end` block into `.git/info/exclude`, holding `.worktrees/`.
2. Create `.git/dckt/config.yml` with `visibility: private` plus the discovered test policy (what `ensureTestPolicyConfig` writes to `.docket.yml` today).
3. Create the bare remote (`git init --bare`) at the default path, or use `--metadata-remote <url>`. If it already holds a `dckt` branch (a second clone, or an earlier private stint), adopt it.
4. Add the `dckt` remote, or verify that an existing one points at the same URL.
5. Create the orphan `dckt` branch with a create-only lease push, or adopt the existing branch.
6. Attach the metadata worktree at the resolved checkout path, with worktree-scoped hooks disabled under `.git/dckt/`.

Private init never writes `.gitignore`, `.docket.yml`, `.docket.local.yml`, or any file in the worktree root. Shared init (`--shared`, or the default) is unchanged. `repository configure-tests` writes to `.git/dckt/config.yml` in private repositories.

### 4. Config in private repositories

The sources are built-in defaults, the global file, and `.git/dckt/config.yml`. The last one acts as the repository layer and honors repository-identity keys. A committed `.docket.yml` is never read, neither from the origin default-branch blob nor from disk. The global layer still cannot set repository-identity keys. Agent model and effort pins remain global-only, exactly as today. Two local config files (`.docket.local.yml` and `.git/dckt/config.yml`) is a configuration error naming both, with the same posture as a malformed file.

### 5. Check, prepare, repair

- **Bootstrap guard.** The 2×2 guard probes the metadata branch on the resolved remote. Legacy detection on the integration branch is unchanged.
- **Health.**
  - metadata branch present on the resolved remote;
  - metadata worktree at the resolved path;
  - in private mode, the ignore block present in `.git/info/exclude`, satisfying the condition the committed `.gitignore` block satisfies in shared mode;
  - no pending-review requirement for `.docket.yml` in private mode.
- **Orphaned checkouts.** A checkout whose clone moved or vanished is reported, and `repository repair` prunes it.
- **The mismatch finding** from section 1.

### 6. Links

After #530, metadata links are relative, which is what private mode needs: there is no GitHub URL for a private metadata branch. Any remaining metadata `BlobURLOnBranch` use is relative in private mode. PR rows stay absolute, because the PR is on GitHub.

### 7. Skills and agents

Replace the 18 literal `.docket/` references and every `origin/docket` spelling with the prepare-context values (`metadata_worktree_path`, `metadata_remote`, `metadata_branch`, `metadata_branch_revision`). This includes:

- implement-next's metadata-HEAD assertion;
- docket-adr's commit-and-push instruction;
- the convention's bootstrap-guard and branch-model prose, which describe the shared layout as the default and the resolved values as authoritative.

The embedded twins change in step, and the prose-contract pins are updated. Skills mention private mode only where behavior differs.

### 8. Documentation

`.docket.example.yml` (plus its twin) and command help only. No `docs/` page mentions private visibility.

## Acceptance criteria

1. **Integration test** against a temporary bare "origin": `init --private`, then create → groom → claim → plan → build → PR (fake GitHub) → mark-implemented.
   - Origin gains only the feature branch.
   - `git status --porcelain --ignored` in the root shows nothing docket-named; `.worktrees/` appears only as ignored.
   - `git branch` and `git remote` show `dckt`.
   - No path under the clone contains `docket`.
2. **Concurrency.** Two concurrent claims of one change in a private repository: exactly one wins, and the other reports `contended` (the lease push to the bare remote is the compare-and-swap).
3. **Global default.** With global `visibility: private`, an already-set-up shared repository keeps running shared with no finding. A new repository's `init` is private. `--shared` and a repository file saying `shared` both override.
4. **Refusals and findings.** Both local config files present: refusal naming both. A repository file contradicting state: mismatch finding. A moved clone: orphaned-checkout finding, then `repair` prunes it.
5. **Shared mode unchanged.** The shared-mode suite is unchanged and green.
6. **Mutation tests.** Hard-wiring any resolver to its shared spelling turns a private-mode test red.
7. **Example config.** `.docket.example.yml` documents `visibility`, and the correspondence guard passes.

The plan may split this into 2a (remote, branch, worktree resolvers) and 2b (config, state folder, init/check/prepare) if the PR grows too large.

## ADRs expected

- Private visibility keeps the single metadata layout and varies only where the metadata branch is published and how per-repo paths are spelled. Relates to ADR-0099 and ADR-0001.
- Repository-identity keys come from the private repository's local config when there is a single clone. Relates to ADR-0019.

## Out of scope

- Keeping docket out of PRs, commits, and shipped files: writing rules and the leak check (#532).
- Switching an existing repository's mode (#533).
- Renaming the binary, install paths, global config path, agent files, or metadata-commit trailers.
- Branch-name templates, fork workflows, and repositories without PR permission.
