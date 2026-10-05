<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0523 — Treat a behind-only .docket copy as healthy and make prepare fast-forward it in place](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0523-repository-check-reports-a-behind-only-docket-copy-as-diverg.md)**
<!-- docket:backlink:end -->

# A behind-only .docket copy is healthy, and prepare fast-forwards it in place

Groomed 2026-10-05 with Daniel, then revised the same day to make "behind is healthy" hold in
every case. The `check` side is a bounded fix to an existing flow. The `prepare` side changes the
startup step every workflow runs, so it carries its own failure-case tests.

## Problem

`docket repository check` decides whether the local `.docket` metadata copy is in sync by asking one
question: is the local `docket` tip equal to the fetched `origin/docket` tip? Any difference sets
`WorktreeFact.Synchronized` to Absent (`synchronizedPresence` in `internal/app/repository_check.go`),
and `reposetup.Classify` turns that into two conflict reasons at once:

- `metadata-worktree-dirty`, because its guard fires on `Clean == Absent || Synchronized == Absent`;
- `local-metadata-diverged`, because its guard fires on any tip inequality.

Both carry "resolve with a human" remedies.

That is wrong for the most common case. Every typed metadata write (`change.groom`, finalize's
closeout, `repository repair`, …) commits in a detached throwaway worktree
(`internal/repository/transaction/engine.go`) and pushes; none of them moves the local `docket`
branch or the persistent `.docket` worktree. Only `repository prepare` does, and every docket
workflow runs `prepare` first. So after almost any docket action the local copy is clean and simply
*behind*, and `check` reports a false conflict. Change 511 hit this after `repair`; change 0366 hit
it twice after finalize.

`prepare` already tells the cases apart: `prepareSyncRelationship` (in
`internal/app/repository_prepare.go`) classifies the local tip against the remote tip as current,
behind, ahead, diverged, or unknown, using `IsAncestor` both ways. `check` never asks.

The same classifier also gates `repository configure-tests` (`configureTestsGuard` classifies over
`augmentCheckFacts`), so `configure-tests` refuses with "not in a healthy state" after any docket
action until `prepare` runs.

**Why calling "behind" healthy needs a prepare fix too.** An audit at grooming confirmed that no Go
command reads the local `.docket` files: every corpus read, including `check`'s own board and
frontmatter findings, reads the fetched remote commit through `OpenObjectSource`. So a behind copy
never causes a stale read in Go, and skills read `.docket/` only right after `prepare`. The weak
link is `prepare` itself. `prepareRoute` always picks the fast-forward for a copy `check` would call
healthy-but-behind, but `prepareFastForwardWorktree` does it by **removing** the worktree,
**deleting** the branch, and **re-adding** both at the target. That can fail or lose content in
states the clean probe calls clean:

- A **locked** worktree (`git worktree lock`, or a stale `initializing` lock from a killed add) makes
  `git worktree remove` refuse on every run.
- **Another worktree** with `docket` checked out (forced) lets the remove succeed, after which the
  branch delete refuses. The repository is left worse than before.
- The re-add runs the repository's shared **post-checkout hook**, because the per-worktree hooks-off
  config was deleted with the old admin directory. A failing hook (husky, lefthook, git-lfs
  without the binary) stops the add with hooks left on. Every later `prepare` then reports a
  no-op while `check` reports `docket-worktree-hooks-enabled`.
- **Silently lost:** ignored files inside `.docket` (`git worktree remove` deletes them), an
  unfinished merge or cherry-pick whose index equals HEAD, and a commit made in `.docket` between
  the routing decision and the execution. That last one is lost because `prepareFastForwardWorktree`
  re-reads the old tip instead of using the one the router checked, and the branch delete also
  drops its reflog.
- A shell whose working directory is inside `.docket` is left in a deleted directory.
- Interruption windows leave a stale registration (stuck until `git worktree prune`), a re-attached
  old tip, or a current-but-hooks-enabled worktree that `prepare` never revisits.

Separately, the **attach path** has a freshness hole. When `.docket` is absent but a local `docket`
branch exists, `ensureMetadataWorktree` re-attaches that branch at its own tip
(`AttachBranchWorktree`), ignoring the fetched target. `prepare` then reports applied/healthy while
`.docket` may be behind, ahead, or diverged, and a skill can read stale files right after it.

A bundled sibling with the same equality-only shape: the primary checkout's
`primary-behind-remote-tip` finding (`conditionFinding`, `CondPrimaryAtRemoteTip`) fires whenever
the primary HEAD differs from the pinned integration tip (`primaryAtTipPresence` in
`internal/app/repository_facts.go`). With unpushed local commits on the integration branch it says
"behind" and tells the user to fast-forward with `sync-integration`, which then skips with
`local-ahead`. The remedy is invalid in the state that produced it.

## Decisions (settled at grooming)

1. **A clean, behind-only local `.docket` copy is healthy.** `check` exits 0 with no finding for it.
   Being behind is docket's normal state after any typed write, and the next `repository prepare`
   (which every workflow runs) fast-forwards it. No warning, because a healthy state with any
   finding exits 1 (`CheckExit`), and "run prepare" is not a required action.
2. **Fix `check`, not the writers.** Typed operations keep pushing from detached worktrees without
   touching the shared `.docket` tree.
3. **One relationship computation.** `check`, `configure-tests`, and `prepare` share the existing
   `prepareSyncRelationship` logic. It may be renamed to a neutral name and moved, but it must not
   be copied.
4. **Bundle the primary-checkout label fix.** `check` splits the primary-not-at-tip finding by
   relationship, so the ahead and diverged cases get an honest label and a remedy that works. The
   primary checkout's health condition is unchanged: it still requires HEAD to equal the tip,
   because `init` legitimately needs that.
5. **The upgrade guide drops its extra `prepare` step.**
6. **`prepare` fast-forwards in place, and its attach path checks freshness.** This makes decision
   1's promise ("the next prepare fixes it") hold in every case, not just the common one. Its rule
   is to refuse rather than discard.

## Design

### 1. The local `.docket` copy in `check`

`augmentCheckFacts` computes the relationship between the local `docket` tip and the fetched remote
tip with the shared function, and records it in the facts the classifier reads. The classifier and
health conditions key on that relationship, never on tip equality. `prepareAugment` fills the same
fact from the same call, so `prepare`'s facts and `check`'s facts never disagree.

| Local copy | Cleanliness | Classification | Finding(s) |
|---|---|---|---|
| current | clean | healthy | none |
| **behind** | clean | **healthy** | **none** |
| current or behind | dirty (uncommitted or untracked files, or an unfinished Git operation, see §4) | conflict | `metadata-worktree-dirty` only |
| ahead (local commits the remote lacks) | any | conflict | `local-metadata-ahead` (plus `metadata-worktree-dirty` if dirty) |
| diverged (both sides moved) | any | conflict | `local-metadata-diverged` (plus `metadata-worktree-dirty` if dirty) |
| unknown (an ancestry probe failed, or a tip is missing) | any | not healthy | an "unverified" warning, see below |

Rules:

- **`metadata-worktree-dirty`** fires only when the worktree is not clean (§4). Its message no
  longer has the "not synchronized" variants. When the cause is an unfinished Git operation, the
  message names it. The remedy is unchanged.
- **`local-metadata-ahead`** is a new `check` reason using the existing finding code `prepare`
  already emits (`FCLocalMetadataAhead`). Its message and remedy match `prepare`'s ("…carries
  commits the remote does not"; reconcile with a human).
- **`local-metadata-diverged`** fires only for true divergence. Message and remedy are unchanged.
- **The healthy condition `CondWorktreeSynchronized`** is met when the relationship is *proven*
  current or behind: "the local copy holds nothing the remote lacks". Unknown never satisfies it.
  Update the condition's doc comment and the `WorktreeFact` field comment to say so.
- **Unknown relationship.** When both tips are known but the ancestry probe fails, the state must
  not read healthy and must not be guessed as behind. Surface one warning, for example
  `local-metadata-sync-unverified` ("could not determine how the local docket branch relates to the
  remote (unverified, not proven diverged)"; remedy: re-run `docket repository check` once local Git
  reads succeed). It is emitted through the existing supplemental-condition path
  (`supplementalConditionFindings` / `conditionFinding` for `CondWorktreeSynchronized`), which today
  returns nil there. When a tip is missing, the existing `local-metadata-missing` /
  `local-metadata-unverified` findings already explain it, so stay silent.
- Update `reasonExplains` so each reason explains exactly the conditions it covers: dirty →
  `CondWorktreeClean`; ahead and diverged → `CondWorktreeSynchronized`.
- **The ancestry probe ignores replace refs and grafts** (run `merge-base --is-ancestor` with Git's
  no-replace-objects mode), so a graft can never fake "behind" and strand local-only commits. If
  `gitcli.IsAncestor` is shared with other callers, add this as an option the relationship helper
  sets rather than changing every caller.

### 2. Consumers of the same classifier

- `repository configure-tests` classifies over `augmentCheckFacts`, so a behind-only copy becomes
  healthy for it with no extra code. It writes only the primary worktree's `.docket.yml`, so a
  behind `.docket` copy cannot affect what it writes.
- `repository init` classifies over the base facts, which do not carry local-metadata facts, so it
  is unaffected. The operational context (`operational_context.go`) is unaffected for the same
  reason: its classification never sees local-metadata or worktree-clean facts.

### 3. `prepare` fast-forwards in place

Replace `prepareFastForwardWorktree`'s remove / delete / re-add sequence with a fast-forward
performed **inside** the existing `.docket` worktree. The plan picks the mechanism. A natural one is
`git -C .docket merge --ff-only <target>`, or an equivalent checked tree update plus a
compare-and-swap ref update. It must have these properties:

1. **Nothing is removed.** The worktree directory, its admin directory (registration, per-worktree
   config, hooks-off setting), the branch, and the branch's reflog all survive. A worktree lock is
   irrelevant, and a shell inside `.docket` keeps a valid working directory.
2. **Compare-and-swap on the router's tip.** The branch moves only from the local tip the router
   observed (`f.LocalMetadata.Tip`, carried on the verdict) to the target, and only when the target
   descends from it. If the tip moved in between (a commit made in `.docket`), the fast-forward
   refuses and changes nothing; the commit survives and the next run classifies it as ahead or
   diverged.
3. **No repository hooks run.** Force the empty hooks directory for the command itself (for
   example `-c core.hooksPath=<docket empty-hooks dir>`) rather than trusting the per-worktree
   setting, so a worktree whose hooks-off config is missing still runs no hooks.
4. **Refuse rather than discard.** An unfinished Git operation, a local change the update would
   overwrite, or an untracked file in the way stops the fast-forward with Git's own refusal mapped to
   a `prepare` error. Ignored files survive, except at a path the target itself tracks, which is
   Git's normal checkout rule.
5. **Interruption never destroys state.** A kill mid-update can leave Git's own `index.lock` (which
   the clean probe reports as unknown, never clean) or a partly updated tree (which reads as dirty),
   but never a missing worktree, a stale registration, or a deleted branch. The human remedy then
   names plain Git (finish or reset the update inside `.docket`), and no `git worktree prune` is
   needed.
6. **Idempotent.** A re-run after success is a no-op (current), and concurrent prepares converge:
   the loser fails its compare-and-swap or meets Git's index lock once, and the next run sees
   current.

After the fast-forward, `prepare` does not need to re-apply hooks-off, because nothing removed it.
`prepareRoute`'s decision (clean + registered + behind → fast-forward) is unchanged, so `check`'s
"healthy" and `prepare`'s ability to act still line up exactly.

**Another worktree holding `docket`.** That is a forced, unsupported setup. The in-place fast-forward
moves the branch inside `.docket` without touching the other worktree's files, so `.docket` is fixed
and never left worse. The other worktree's own state is not docket's to manage.

### 4. "Clean" includes no unfinished Git operation

`worktreeCleanPresence`, the one clean probe both `check` and `prepare` use, also treats an
unfinished merge, cherry-pick, revert, or am inside `.docket` as **not clean**. Use the existing
`gitcli.WorktreeCheckoutState` markers. Today a merge whose index equals HEAD reads clean, and the
remove/re-add silently throws away MERGE_HEAD. A rebase or bisect already shows as a detached HEAD,
which reads as foreign; leave that as is. A probe error stays Unknown.

### 5. `prepare`'s attach path checks freshness

When `.docket` is absent, `prepareRoute` routes on the sync relationship `prepareAugment` already
computes, instead of attaching whatever local branch exists:

| Local `docket` branch | Action |
|---|---|
| absent | create the branch at the target and attach it (today's behavior) |
| current | attach it (today's behavior) |
| behind | move the branch to the target with a compare-and-swap ref update from the observed tip, then attach. If another worktree has the branch checked out (a forced setup), refuse with the existing `docket-worktree-ambiguous-registration` finding and touch nothing |
| ahead | refuse with `local-metadata-ahead`; touch nothing |
| diverged | refuse with `local-metadata-diverged`; touch nothing |
| unknown | refuse with the existing local-state-unknown verdict |

After attaching, apply hooks-off as today. Keep this in `prepare`'s route and execution.
`ensureMetadataWorktree` is shared with `init` and `migrate`'s local finish, so don't change its
contract for them; give `prepare` its own fresh-attach action, or pass the decision in.

### 6. The primary checkout (bundled sibling)

When `check` finds the primary HEAD is not the pinned integration tip, it computes the HEAD-to-tip
relationship with the same shared function and picks the finding by relationship. The health
condition `CondPrimaryAtRemoteTip` and its "must equal the tip" meaning are unchanged, so every
non-current case stays non-healthy.

| Primary HEAD vs integration tip | Finding | Remedy |
|---|---|---|
| behind | `primary-behind-remote-tip` (existing code; reword the message to say behind) | existing: fast-forward, e.g. `docket repository sync-integration`, preserving local work |
| ahead | `primary-ahead-of-remote-tip` (new), severity error | push the local commits (or move them to a branch) yourself, preserving them, then re-run `docket repository check` |
| diverged | `primary-diverged-from-remote-tip` (new), severity error | reconcile the primary checkout with the remote integration branch yourself (rebase or merge), preserving local work, then re-run `docket repository check` |
| unknown (probe failed) | `primary-tip-unverified` (existing) | existing |

Compute this only where it is reported, in `check`'s augmentation (`augmentCheckFacts`). Do not add
ancestry probes to the shared base gather (`gatherRepoFacts` / `primaryAtTipPresence`), which every
operational command runs. `init`'s refusal text is out of scope.

`check`'s health findings are built as plain code strings in `internal/reposetup/health.go`;
`local-metadata-ahead` is already in the app-layer registry (`internal/app/finding_codes.go`)
because `prepare` emits it. Add the new codes next to their siblings, and extend any registry or
guard test that lists `check`'s codes. A whole-repo grep for `primary-behind-remote-tip` finds
those places.

### 7. Upgrade guide

In `docs/release/upgrading-from-bash.md`, after `docket repository repair --yes`:

- Delete the claim that `check` reports `metadata-worktree-dirty` and `local-metadata-diverged`
  until `prepare` runs, and the instruction built on it.
- The `repo-confirm` step becomes `docket repository check` alone, which reports `healthy` right
  after the repair.
- `repair`'s own output is unchanged. It still says to re-run `prepare` to sync the local copy,
  which is true but no longer required. If the guide mentions it, say it is optional and that the
  next docket command does it anyway. Follow the docs rule: describe current behavior only, with no
  change or PR citations.

Update the guide's executable test in `internal/bashupgrade/registry_test.go` to match.
`runRepairApply` currently asserts the two codes appear before `prepare`. It must instead assert
the guide's new claim (check is healthy right after repair) and stop requiring the deleted text.
`runRepoConfirm` keeps asserting `repository check: no-op (healthy)` against the new block. Keep
every assertion tied to a sentence the guide actually makes.

### 8. Tests

- **Classifier and health units** (`internal/reposetup`): one case per row of the §1 and §6 tables,
  covering classification, the finding codes present and absent, and exit code through `CheckExit`.
  Existing tests that encode the old behavior (behind → dirty + diverged, the "not synchronized"
  dirty message variants) are updated, not deleted, to assert the new rows.
- **Real-git `check` tests** (`internal/app`): build a migrated fixture, advance `origin/docket` with
  a typed metadata write (or a direct push) so `.docket` is clean and behind, then assert:
  - `check` is `healthy`, exits 0, and reports no findings for the local copy;
  - `configure-tests` is not refused;
  - a local commit in `.docket` yields `local-metadata-ahead`;
  - a local commit plus a remote advance yields `local-metadata-diverged`;
  - an uncommitted file plus a remote advance yields `metadata-worktree-dirty` only;
  - an unfinished `merge --no-commit -s ours` in `.docket` yields `metadata-worktree-dirty`, and
    `prepare` refuses without discarding MERGE_HEAD.
- **Real-git `prepare` fast-forward tests** (extend `internal/app/repoprepare_integration_test.go`).
  Each starts from a clean, behind `.docket`:
  - **locked worktree:** the fast-forward succeeds and the lock is still present;
  - **failing or file-writing repository hook** (post-checkout and post-merge installed through the
    repository's shared hooks path): the fast-forward succeeds, the hook never runs, and `check` is
    healthy after;
  - **ignored file in `.docket`:** it survives the fast-forward;
  - **directory identity:** a sentinel ignored file (or the inode) proves the worktree directory
    was never removed and re-created;
  - **compare-and-swap:** the execute step is handed a stale observed tip (a commit was added in
    `.docket` after routing); it refuses, and the commit is still on the branch;
  - **hooks-off missing before the fast-forward:** no hook runs (the forced empty hooks path);
  - **idempotence:** a second `prepare` is a no-op, and `check` is healthy.
- **Real-git attach tests:** `.docket` removed with the local branch behind → attached at the
  target, and `check` is healthy; ahead → `local-metadata-ahead`, branch untouched, nothing attached;
  diverged → `local-metadata-diverged`, same.
- **Primary real-git cases:** an unpushed commit on the integration branch yields
  `primary-ahead-of-remote-tip`; a behind primary still yields `primary-behind-remote-tip`.
- **Mutation-test the new rules** (AGENTS.md):
  - put tip equality back as the sync predicate, and the behind-is-healthy test fails;
  - collapse ahead into diverged, and the ahead test fails;
  - drop the primary split, and the primary-ahead test fails;
  - restore remove/re-add in `prepareFastForwardWorktree`, and the lock, hook, ignored-file and
    directory-identity tests fail;
  - drop the compare-and-swap, and the stale-tip test fails;
  - drop the attach-path routing, and the attach-behind test fails.
- Run the whole suite at the build gate.

## Out of scope

- How a truly diverged or ahead local copy is handled. It stays a conflict with a human remedy.
- Making typed operations advance the local `.docket` copy after they push.
- `migrate`'s and `init`'s attach behavior (`ensureMetadataWorktree`'s contract for them),
  `repair`'s output, and `init`'s refusal text.
- Self-healing a worktree whose hooks-off config is already missing. `check` keeps reporting
  `docket-worktree-hooks-enabled` with its manual remedy. The in-place fast-forward simply never
  causes it again.
- `transaction.Engine.candidateReachable`, which reads the always-stale local ref for a
  report-only field and has no production caller.
- `repository check` findings other than those named above.

## Acceptance

- After any typed metadata write, with no other local change, `docket repository check` reports
  `healthy` and exits 0, and `docket repository configure-tests` is not refused.
- In every state `check` reports as healthy-but-behind, the next `docket repository prepare`
  fast-forwards `.docket` without removing it, running no repository hook, and discarding nothing.
  Afterwards `check` is still healthy.
- `prepare` never attaches a stale, ahead, or diverged local branch and then reports healthy.
- A dirty (including an unfinished Git operation), ahead, or diverged local `.docket` copy still
  reports a conflict, each with its own finding and no false companion finding.
- A primary checkout with unpushed commits reports `primary-ahead-of-remote-tip` with a remedy that
  works in that state.
- The upgrade guide no longer runs `prepare` after `repair`, and its executable test passes against
  the new text.
