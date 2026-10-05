<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0523 — repository check flags a behind-only .docket copy as a conflict and an ahead primary as behind](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0523-repository-check-reports-a-behind-only-docket-copy-as-diverg.md)**
<!-- docket:backlink:end -->

# repository check reads how the local copies relate to the remote, not just whether they match

Groomed 2026-10-05 with Daniel. Bounded fix to the existing `repository check` flow; this spec
records the settled design so the change is build-ready.

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
branch or the persistent `.docket` worktree. Only `repository prepare` does, by fast-forwarding a
clean, strictly-behind copy, and every docket workflow runs `prepare` first. So after almost any
docket action the local copy is clean and simply *behind*, and `check` reports a false conflict.
Change 511 hit this after `repair`; change 0366 hit it twice after finalize.

`prepare` already tells the cases apart: `prepareSyncRelationship` (in
`internal/app/repository_prepare.go`) classifies the local tip against the remote tip as current,
behind, ahead, diverged, or unknown, using `IsAncestor` both ways. `check` never asks.

The same classifier also gates `repository configure-tests` (`configureTestsGuard` classifies over
`augmentCheckFacts`), so `configure-tests` refuses with "not in a healthy state" after any docket
action until `prepare` runs. That sibling is fixed by the same change.

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
   touching the shared `.docket` tree. A write-side sync would have to fast-forward a folder that
   concurrent runs and editors use, and could only be best-effort, because `prepare` already
   refuses a dirty copy.
3. **One relationship computation.** `check`, `configure-tests`, and `prepare` share the existing
   `prepareSyncRelationship` logic. It may be renamed to a neutral name and moved, but it must not
   be copied.
4. **Bundle the primary-checkout label fix.** `check` splits the primary-not-at-tip finding by
   relationship so the ahead and diverged cases get an honest label and a remedy that works. The
   primary checkout's health condition is unchanged: it still requires HEAD to equal the tip,
   because `init` legitimately needs that.
5. **The upgrade guide drops its extra `prepare` step.**

## Design

### 1. The local `.docket` copy

`augmentCheckFacts` computes the relationship between the local `docket` tip and the fetched remote
tip with the shared function, and records it in the facts the classifier reads. The classifier and
health conditions key on that relationship, never on tip equality. `prepareAugment` fills the same
fact from the same call, so `prepare`'s facts and `check`'s facts never disagree. `prepare`'s own
routing (`prepareRoute`) is unchanged.

| Local copy | Cleanliness | Classification | Finding(s) |
|---|---|---|---|
| current | clean | healthy | none |
| **behind** | clean | **healthy** | **none** |
| current or behind | dirty (uncommitted or untracked files) | conflict | `metadata-worktree-dirty` only |
| ahead (local commits the remote lacks) | any | conflict | `local-metadata-ahead` (plus `metadata-worktree-dirty` if dirty) |
| diverged (both sides moved) | any | conflict | `local-metadata-diverged` (plus `metadata-worktree-dirty` if dirty) |
| unknown (an ancestry probe failed, or a tip is missing) | any | not healthy | an "unverified" warning, see below |

Rules:

- **`metadata-worktree-dirty`** fires only on `Clean == Absent`. Its message no longer has the
  "not synchronized" variants. Its remedy is unchanged.
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

### 2. Consumers of the same classifier

- `repository configure-tests` classifies over `augmentCheckFacts`, so a behind-only copy becomes
  healthy for it with no extra code. It writes only the primary worktree's `.docket.yml`, so a
  behind `.docket` copy cannot affect what it writes.
- `repository init` classifies over the base facts, which do not carry local-metadata facts, so it
  is unaffected. The operational context (`operational_context.go`) is unaffected for the same
  reason.

### 3. The primary checkout (bundled sibling)

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

### 4. Upgrade guide

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

### 5. Tests

- **Classifier and health units** (`internal/reposetup`): one case per row of both tables, covering
  classification, the finding codes present and absent, and exit code through `CheckExit`. Existing
  tests that encode the old behavior (behind → dirty + diverged, the "not synchronized" dirty
  message variants) are updated, not deleted, to assert the new rows.
- **Real-git test** (`internal/app`): build a migrated fixture, advance `origin/docket` with a typed
  metadata write (or a direct push) so `.docket` is clean and behind, then assert:
  - `check` is `healthy`, exits 0, and reports no findings for the local copy;
  - `configure-tests` is not refused;
  - a local commit in `.docket` yields `local-metadata-ahead`;
  - a local commit plus a remote advance yields `local-metadata-diverged`;
  - an uncommitted file plus a remote advance yields `metadata-worktree-dirty` only.
- **Primary real-git cases:** an unpushed commit on the integration branch yields
  `primary-ahead-of-remote-tip`; a behind primary still yields `primary-behind-remote-tip`.
- **Mutation-test the new rules** (AGENTS.md): put tip equality back as the sync predicate and watch
  the behind-is-healthy test fail; collapse ahead into diverged and watch the ahead test fail; drop
  the primary split and watch the primary-ahead test fail.
- Run the whole suite at the build gate.

## Out of scope

- How a truly diverged or ahead local copy is handled. It stays a conflict with a human remedy.
- Making typed operations advance the local `.docket` copy after they push.
- `prepare`'s routing and refusals, `repair`'s output, and `init`'s refusal text.
- `repository check` findings other than those named above.

## Acceptance

- After any typed metadata write, with no other local change, `docket repository check` reports
  `healthy` and exits 0, and `docket repository configure-tests` is not refused.
- A dirty, ahead, or diverged local `.docket` copy still reports a conflict, each with its own
  finding and no false companion finding.
- A primary checkout with unpushed commits reports `primary-ahead-of-remote-tip` with a remedy that
  works in that state.
- The upgrade guide no longer runs `prepare` after `repair`, and its executable test passes against
  the new text.
