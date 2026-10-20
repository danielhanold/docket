<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0574 — Recognize the docket branch by its shape in the everyday repository commands](../../changes/archive/2026-10-20-0574-stop-repository-prepare-re-walking-all-of-main-s-history-to.md)**
<!-- docket:backlink:end -->

# Recognize the docket branch by its shape in the everyday repository commands

## Why

`docket repository prepare` took 68.7 seconds on this repository on 2026-10-20 (29s user, 24s
system CPU). Prepare runs at the start of nearly every docket workflow, so every one of them pays
it. `repository check`, `repository repair`, `repository configure-tests`, and `repository
configure-harnesses` pay the same cost for the same reason.

The cause is the metadata-ownership verifier, `verifyMetadataOwnership`
(`internal/app/metadata_ownership.go`). It first runs two cheap structural checks on the fetched
metadata tip: the branch has exactly one parentless root, and it shares no history with the
integration branch. It then looks for proof that the root is a docket seed. A root carrying a
docket seed receipt, or an empty tree, is proved instantly. A root with no receipt and a nonempty
tree goes to `verifyLegacyEquivalence`, which walks the complete integration history newest-first,
looking for the snapshot the seed was copied from. On this repository that is 4,474 commits; the
match is commit 4,382 of them. The walk spawns separate git processes per commit (two `ls-tree`
calls per commit, about three more per distinct snapshot, 422 distinct snapshots), roughly 10,000
git processes in all. Nothing is kept, so every prepare repeats it.

Every repository seeded by the old Bash migration script has such a root. A repository whose
metadata branch began with an empty first commit already takes the instant path, and its prepare
finishes in a few seconds, mostly fetches.

The content proof exists to tell docket's own metadata branch apart from a branch somebody else
happens to have named `docket`. It earns its keep in the two commands that create or destroy
planning data: `repository migrate`, which deletes the planning surface from the integration
branch and must first confirm the metadata branch really holds it, and `repository init`, which
decides whether to adopt an existing branch. The everyday commands only write to the metadata
branch, the same as every docket operation that runs after prepare. For them the two structural
checks catch the realistic accident (an ordinary branch named `docket`, which shares the
integration branch's history), and prepare already refuses when the remote metadata branch stops
building on the local one (`local-metadata-ahead`, `local-metadata-diverged`).

Change 0378, which introduced the shared verifier, listed bare single-root acceptance as out of
scope. This change deliberately reverses that for the everyday commands only.

## Goal

The everyday repository commands (`prepare`, `check`, `repair`, `configure-tests`,
`configure-harnesses`) decide metadata-branch ownership from the branch name plus the two
structural checks, deterministically in the binary. A prepare on this repository takes seconds,
not a minute.

## Non-goals

- Retiring `repository migrate`, the old single-branch layout states, or the code that supports
  them. Considered and declined during grooming: it is unrelated to the ownership check and about
  1% of the non-test Go.
- Changing `repository init` or `repository migrate`. Both keep the full verifier, history search
  included. They run rarely, and they are the commands that create or delete planning data.
- Speeding up the full verifier (for example batching its git calls into one `cat-file
  --batch-check` stream). Not needed once the everyday commands skip it.
- The seed receipt format, or how new repositories are seeded.
- The implement-next coordination cost (#360).

## Anti-goals

- A cache, stored proof, or receipt written after the fact. That is new state to invalidate, and
  the verifier deliberately trusts only receipts on the root commit itself.
- Reading a git error as owned or as foreign. A probe that fails is unknown, and unknown refuses,
  exactly as today.
- Any model judgment. The model runs `docket repository prepare` and reads its disposition; the
  binary decides.

## Decisions

### 1. The quick ownership check

The branch-name part is unchanged: the existing remote-metadata presence probe (`docket` on the
metadata remote in a shared repository, `dckt` in a private one). Given the fetched metadata tip
and the integration tip, the quick check decides:

1. **One starting commit.** Exactly one parentless commit is reachable from the metadata tip
   (`gitcli` `RootCommits`). More than one means an unrelated history was merged in: foreign.
2. **No shared history with the integration branch.** The metadata tip and the integration tip
   have no common ancestor (`gitcli` `HasSharedAncestry`). Shared history means an ordinary branch
   cut from the integration branch: foreign.
3. **Errors stay unknown.** A failing probe, or an unknown integration tip, is `RootUnknown` with
   the error retained.

Otherwise the branch is owned (`RootParentless`). The quick check reads no receipt and no file
contents. These are exactly the steps `verifyMetadataOwnership` runs before its receipt and
content proofs today. Factor them into one function that the full verifier calls first, so the
shared steps have a single source. Each command takes about 15ms on this repository (6,112
metadata commits).

### 2. Which command uses which check

The split: commands that create or delete planning data keep the full verifier; every other
command uses the quick check.

| Command | Where it reads ownership | Ownership check |
|---|---|---|
| `repository prepare` | `prepareAugment` | quick |
| `repository check` | `augmentCheckFacts` | quick |
| `repository configure-tests` | `augmentCheckFacts` | quick |
| `repository configure-harnesses` | `augmentCheckFacts` | quick |
| `repository repair` | `migrateRoute` | quick |
| `repository migrate` | `migrateRoute`, `reconcileResumeSeed` | full, unchanged |
| `repository init` (its adopt-or-refuse decision) | `repository_init.go` | full, unchanged |

Change `augmentCheckFacts` in place, so `check` and both configure commands agree on what healthy
means. (Leaving the configure commands on the full verifier would keep them at about 70 seconds and
let them refuse a repository `check` calls healthy, sending the user to a dead end.) Private
`repository init` also calls `augmentCheckFacts` once, after it has finished, to report the state
the repository ended in. That report gets the quick check too. It is harmless, because init's own
adopt-or-refuse decision, which runs earlier, keeps the full verifier.

`repository repair` reaches the verifier through migrate's routing (`migrateRoute`, on the
re-fetched metadata tip). That routing consumes only the verifier's `Shape` (owned / foreign /
unknown), never which proof matched, so the quick check fits it without other changes. Repair
must get the quick check while migrate keeps the full one; migrate's behavior must not change.
Repair never reaches migrate's second verifier call (`reconcileResumeSeed`), because it refuses
the resume-prune phase and points at `repository migrate`.

### 3. Outcome changes, deliberate

Under the quick-check commands, these now read as owned instead of foreign:

- a root with no receipt and a nonempty tree that matches no historical integration snapshot;
- a root whose docket receipt fails validation (malformed trailers, an init receipt over a
  nonempty tree, a migrate receipt whose copy digest mismatches or whose source revision is
  malformed or unreachable).

`repository migrate` and `repository init` still refuse them. Everything else is unchanged:
two roots and shared history are foreign, failed probes are unknown, and the dirty, ahead, and
diverged refusals stay.

This reaches the half-migrated detection. With the quick check, these commands classify "owned
metadata branch, planning surface still on the integration branch" as an interrupted migration
and point at `repository migrate`, which then runs the full verifier and refuses a branch it
cannot prove. That is safe. The same holds for the attach-incomplete partial state
`augmentCheckFacts` sets (`PartialIntegrationPruned`): a branch only the full verifier would reject
can now reach the `attach-incomplete` or `local-metadata-missing` remedies, which point at
`repository migrate`, which refuses it. Leave those remedies as they are (my call); prepare still
attaches the worktree normally. The comment in `reposetup.Classify` saying the
partial state "resolves here only because the RootForeign case above already claimed the
non-corresponding seed tree as conflict" stops being true for the quick-check callers; rewrite it.
The `RootShape` doc comments in `internal/reposetup/probe.go` describe owned as "a verified
docket seed root"; reword them to mean "proven owned by the caller's ownership check", quick or
full, and say which commands use which.

### 4. Finding text

The `metadata-root-foreign` text lives in two places, and the reason is now structural in both:

- `internal/reposetup/health.go`, used by `check` and the configure commands: "The remote docket
  branch is not a docket-created orphan root (foreign or non-corresponding tree)."
- `prepareRoute` (`internal/app/repository_prepare.go`), used by prepare: "The remote docket
  branch is not a docket-created parentless orphan root."

Reword both in plain words: what was found (the remote metadata branch has more than one starting
commit, or shares history with the integration branch), that docket therefore will not treat it
as its own, and the remedy (leave it unchanged, resolve it with a human). The code stays
`metadata-root-foreign`. Migrate's and init's own refusal text is unchanged. Repair's foreign
refusal comes from the routing it shares with migrate ("not a verified docket metadata branch"),
which is still accurate. It may keep that text unless the builder splits the routing.

### 5. ADR (my call)

Record a new ADR through `docket-adr` during the build. Decision: the everyday repository commands
(prepare, check, repair, configure-tests, configure-harnesses) recognize the metadata branch by
name and shape; the content proof
(receipts, legacy equivalence) is reserved for the commands that create or delete planning data
(init, migrate). Alternatives considered: batching the history walk (fast, but keeps a full walk
on every command for no everyday benefit), caching the proof (new state to invalidate), name only
(adopts an ordinary branch someone named `docket`), and retiring migrate (unrelated to ownership).
The ADR exists so a later change does not quietly put the content proof back into prepare.

### 6. Tests

- **The everyday commands no longer run the content proof.** A fixture whose metadata root has no
  receipt, a nonempty tree, and no matching integration snapshot: prepare reaches a healthy
  disposition, check reports no `metadata-root-foreign`, `configure-tests` is not refused on
  ownership, and a repair preview reaches its already-migrated phase instead of refusing as
  foreign. `repository migrate` on the same fixture still refuses it as foreign. This is the
  deterministic stand-in for the speed fix; there is no wall-clock assertion.
- **The structural checks still refuse.** For prepare, check, and repair: two parentless roots
  are foreign, shared history with the integration branch is foreign, and an unreadable tip or
  unknown integration tip is unknown. Mutation-test each structural check: remove it and watch its
  test redden.
- **Existing tests.** Command-level tests that expect prepare or check to report foreign for a
  content or receipt mismatch (for example `TestIntegrationRepoOwnershipCheckForeignNonemptyRoot`)
  flip to the new expectation. Verifier-level tests of `verifyMetadataOwnership` stay, because
  migrate and init still use it. Sweep every test that references `RootForeign`,
  `metadata-root-foreign`, or the verifier; today that includes `finding_codes_test.go`,
  `repoownership_integration_test.go`, `repoprepare_integration_test.go`,
  `repository_prepare_test.go`, `internal/bashupgrade/bashupgrade_integration_test.go`, and the
  `reposetup` classify and health tests.
- **Acceptance by hand**, recorded in the results file: time `docket repository prepare
  --repo-dir <this repository> --json`, `docket repository check`, and a `docket repository
  repair` preview on this repository with the new binary. Each takes seconds.

## Left to the builder

- Function names and how the shared structural steps are factored.
- How repair's routing receives the quick check while migrate keeps the full verifier (a
  parameter on the shared routing, or a split).
- Exact finding wording, within decision 4.
