<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0378 — Shared metadata-root classifier misreads any multi-commit docket branch as foreign](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-30-0378-metadata-root-classifier-rejects-multi-commit-docket-branch.md)**
<!-- docket:backlink:end -->
# Verify docket metadata ownership at the seed root

Date: 2026-08-30
Change: 0378

## Purpose and approved scope

Correct the shared repository-setup ownership proof so a docket branch remains recognizable after
ordinary metadata commits. Keep change 0378 an independently mergeable predecessor of 0377; do not
absorb it into that change or build on its unmerged branch. The human approved this design on
2026-08-30. This document is the design, not an implementation plan.

Use one ownership verifier across check, init, and migrate, while retaining each operation's
separate publication and recovery preconditions. Change 0377 must consume that verifier when it
resumes and adds `repository prepare`. No workflow is resumed by this grooming operation.

## Evidence and surrounding work

At grooming time, `augmentCheckFacts`, `expectedInitShape`, and `metadataRootParentless` all compare
the sole reachable root with the current metadata tip. That comparison recognizes only a branch
with one commit. The `RootCommits` contract already requires inspection of the root's tree/receipt;
`reposetup.RootParentless` already describes an expected receipt or exact legacy-equivalent tree.
The implementation does not fulfill those contracts.

The installed `docket repository check --json` reproduced `metadata-root-foreign` against this
repository. Its sole metadata root is `f8b226f25114411f3962157836c7ac3d51580abf`, with no native
receipt. Its entire recursive tree listing exactly matches the metadata projection of integration
revision `1e7493a6bd594c001c03f85a7130dc0590e93fc1`, the parent of the historical prune commit.
The projection comprises `docs/changes`, `docs/adrs`, and `docs/superpowers/specs`, with ten leaf
entries and no extra paths. These OIDs are evidence, never production allowlists or test constants.

Change 0352 introduced the native setup services; 0363 established the single supported topology
in ADR-0099. Both are done. Changes 0371 and 0372 are also done. Change 0377 is in-progress but
halted after its first two build tasks; its prepare implementation copied the faulty predicate.
Change 0370 remains halted behind 0377. Change 0378 has no unsatisfied dependencies and no stack
parent. ADR-0001's orphan metadata branch and ADR-0099's one-topology decision remain unchanged.

The live check also reports unrelated corpus/frontmatter findings. Removing the false ownership
finding does not promise a wholly healthy live repository or a zero exit status for that check.

## Ownership proof

The verifier reads immutable objects from one pinned, authoritatively fetched metadata tip and
the pinned remote integration history. Return the verified tip, root OID, proof kind, and any
source revision needed by callers; do not return a boolean that conflates foreign with unreadable.
An internal result type is sufficient; no public command, config key, or protocol version is added.

The shared topology requirements are:

1. The complete reachable metadata history has exactly one actual parentless root. The root need
   not equal the tip. Descendants and merges whose parents share this root are allowed.
2. Metadata history must not share ancestry with the authoritative integration history. A branch
   created from the integration branch is not an orphan metadata branch, even if its files or
   trailers resemble docket's.
3. The root satisfies one of the seed proofs below. Root count, branch name, commit subject,
   author identity, timestamps, or a populated `.docket` directory are not ownership proofs.

Incomplete/shallow history, missing objects, failed fetches, cancellation, and failed reads leave
ownership unknown. Never interpret a shallow boundary as an actual parentless root or fall back
from an errored probe to a weaker proof. No automated history rewrite or history-deepening policy
is introduced; explain the missing evidence so the operator can restore access and retry.

### Native seed receipts

Read trailers from the root commit itself. A valid receipt on a descendant cannot authorize an
unrecognized root. Require an unambiguous supported operation and operation-appropriate required
fields in their existing emitted formats; duplicated recognized fields, malformed fields,
unsupported operation versions, and a prune receipt on the root are not valid seed receipts.

- An `OpInitRoot` receipt must accompany the empty Git tree.
- An `OpMigrateSeed` receipt must name a valid source revision reachable in the pinned integration
  history, contain the migration's required copy and repair digests, and have `CopyDigest` equal
  to the actual root tree OID. Preserve the meaning of the recorded repair digest. The seed can
  contain explicitly authorized frontmatter repairs, so equality to an unmodified source
  projection is not an additional requirement for a valid native seed.

Receipt presence alone is insufficient. Conversely, a receipt claiming docket provenance but
failing these checks must not be downgraded to the receiptless legacy path. These are the existing
repository receipt/tree conventions, not cryptographic author authentication; do not describe
them as signed attestations or protection against an actor able to fabricate trusted Git history.

### Receiptless legacy seeds

Preserve exact legacy equivalence without requiring old histories to acquire a new root or receipt.
The empty tree remains the exact legacy-bootstrap seed shape, subject to the same complete,
disjoint ancestry checks. This is an explicit compatibility equivalence, not proof of an author.

For a nonempty legacy seed, require exact equality of the root's entire tree with the metadata
copy set from a source snapshot reachable in the pinned integration history. Use the historical
source's committed directory configuration and shipped defaults where absent, including the
fixed specs directory. Decode legacy topology configuration only as historical migration input;
do not re-enable main mode or let current user/machine configuration redefine historical evidence.

The source must contain the legacy live planning surface. Compare complete path, mode, object type,
and object identity across the copied changes/ADR/specs prefixes, preserving unknown files within
those prefixes. Refuse an extra path anywhere in the root, a missing copied path, changed bytes or
mode, or an unrelated single-root tree. Directory resemblance or a matching subset is insufficient.

Search relevant reachable historical snapshots, not just today's already-pruned integration tree.
Deduplicate equivalent candidate snapshots/projections and use batched object reads where possible;
commit messages and dates may not gate eligibility. More than one snapshot proving the same exact
tree is not ambiguity: the proof is content identity. Do not silently truncate a history search and
report foreign when it did not finish. Distinguish exhausted, readable history with no proof
(foreign) from incomplete or unreadable evidence (unknown). No persistent ownership cache, new
marker, root allowlist, or blanket adopt override is part of this change.

Historical comparison is content-read-only. Do not check out old source commits, rewrite metadata,
edit the real index, or write compatibility receipts. Narrow typed history/object reads may be
added to `internal/gitcli` if existing adapters cannot express the required reads.

## Shared implementation boundary

Place Git-reading orchestration in the application layer, preferably a focused metadata-ownership
helper alongside `repository_facts.go`. Keep `internal/reposetup` pure and gitcli-free: it owns
proof/result vocabulary and deterministic receipt/tree decisions where useful. Keep Git execution
inside `internal/gitcli`; do not introduce shell execution in application code.

Clarify `RootParentless` to mean a verified docket seed root with permitted descendants, and
`RootUnknown` versus `RootForeign` to preserve the unreadable/unproven distinction. Keeping these
internal names avoids unnecessary interface churn. Remove obsolete root-equals-tip ownership
comments and duplicated ownership predicates. Derive the full caller inventory from repository-wide
searches for root enumeration and seed adoption/recovery, not only the functions named here.

Check must use the fetched tip consistently for ownership, reported remote revision, and the
existing synchronization comparisons. A fetch error is unknown even if an older object happens to
be available locally. No earlier local probe can authorize adoption of a different remote tip.
Publication paths retain their authoritative rereads and create-only or exact-lease guards.

## Operation-specific safety

Sharing proof mechanics must not flatten the different operations' permissions.

| Consumer | Required behavior |
|---|---|
| `repository check` / `augmentCheckFacts` | Recognize native and legacy descendants; retain all independent corpus, ignore, worktree, synchronization, and surface findings. Content remains read-only, with only already-permitted object/remote-tracking fetch effects. |
| `repository init` / `publishOrAdoptMetadataRoot` | Retain the fresh/init guards and create-only publication. A race loser may adopt a freshly verified init-equivalent lineage at its reread remote tip, preserving descendants, never reset it to the seed. A migration-seeded lineage does not become permission to initialize or bypass the command's state guard. |
| `expectedInitShape` | Consume shared evidence plus the init-equivalent seed requirement. Test the root's empty tree, not the current tip's tree. Keep a stricter exact-tip requirement only where a specific effect needs it. |
| `repository migrate` / `migrateRoute` | Use common ownership evidence to distinguish an established migrated branch from a foreign branch. An already-pruned integration tree can permit the existing no-op/local-finish path without discarding later metadata commits. |
| `reconcileResumeSeed` and seed publication | Preserve exact current-source tree postconditions and response-loss recovery. A seed-replacement path is allowed only while the fresh remote tip is still the verified seed itself and the existing receipt/source/lease conditions hold. If descendants exist while a live integration surface still needs pruning, refuse before replacing the seed or pruning integration; a human must reconcile that partial migration. |
| Future 0377 `repository prepare` | On explicit continuation after 0378 merges, reuse this verifier and retain prepare's independent clean-worktree, fast-forward-only, divergence, and synchronization requirements. No prepare implementation is added in 0378. |

The last two requirements prevent a dangerous consequence of the fix: a broadened ownership proof
must never let migration replace a mature branch with a newly composed parentless seed. Recheck
identity at the mutation boundary and bind the lease to the same verified tip. A concurrent advance
must produce contention/refusal and leave the winner's history intact.

## Verification and acceptance

Use meaningful behavioral tests in the repository's existing unit and real-Git integration
partitions. Read `tests/README.md` and resolve the whole-suite command from `finalize.test_command`
at build time. The current value is `go run ./cmd/docket development test`. Do not add fragile prose
greps in place of exercising ownership and preservation behavior.

Required positive coverage:

- Native init and migration seeds, both alone and followed by multiple content-changing commits.
- A merge of descendants sharing the same verified root.
- Receiptless empty bootstrap ancestry and nonempty legacy migration ancestry with descendants.
- Legacy equality against an older reachable snapshot after the live planning surface is pruned;
  include historical nondefault directories and changed current configuration.
- Healthy check classification when all other fixture facts are healthy, init race adoption of
  a valid init lineage, and migrated/no-op or local-attachment recovery at the actual latest tip.
- Existing exact seed response-loss and stale-source recovery cases continue to work while the
  branch has no descendants and their operation-specific preconditions hold.

Required negative/preservation coverage:

- A foreign nonempty single-root branch, multiple roots, and ancestry shared with integration.
- A native receipt only on a descendant; missing, malformed, duplicate, unknown-version, or wrong
  operation fields; wrong init tree; migration digest mismatch or unavailable source history.
- Legacy roots with an extra file outside the copy set, a missing file, changed content/mode,
  or no exact historical source match. A plausible commit subject does not rescue a mismatch.
- Failed metadata fetch, root/tree/receipt/history reads, and truncated/shallow history map to
  unknown/error without adopting, overwriting, or pruning anything.
- Descendants added after migration seeding, before resume, and during the publication race:
  no descendant is lost; unsafe partial migration is refused before integration pruning.
- Ordinary metadata updates need no seed receipt on each new commit and need not retain the
  root's tree contents. Independent tip/corpus validation still runs.

For refusal/race cases, assert remote OIDs and relevant local branch, index, worktree, and config
state are preserved, allowing only documented fetch effects. Mutation-test the ownership and
replacement guards: remove root identity validation, weaken exact legacy equality, collapse a
probe error, or permit seed replacement after descendants, and require the corresponding test to
fail. Preserve uncommitted edits when restoring mutation targets.

Record a content-read-only live-history check at build time using the newly built source. Against
this repository, `metadata-root-foreign` must disappear for the pinned real history, without
silencing unrelated health findings. Record metadata and integration OIDs and the proof category;
do not turn live OIDs into hermetic fixture requirements. Run a tampered legacy-tree negative
control in a disposable repository. Measure the legacy history path on the real history and
report its cost; honor suite budget diagnostics and do not hide incomplete scans behind a cap.

## Delivery and non-goals

The grooming commit links this spec from 0378, adds 0352/0363 context and ADR-0001/0099 references,
and adds 0378 to 0377's dependencies. Preserve 0377's halt record, branch, committed tasks, and
uncommitted work. Its later authorized continuation must reconcile/rebase onto merged 0378 and
replace the prepare-local ownership copy with the shared verifier. Do not modify its frozen build
plan as part of grooming.

Build 0378 using the currently functioning workflow; do not require 0377's unfinished prepare
command or delete the frozen facade needed to build this prerequisite. Stop at an open PR under
the normal implementation workflow; grooming itself writes only metadata Markdown.

Excluded: implementing or resuming 0377/0370, facade deletion, main-mode restoration, metadata
topology or receipt-format redesign, a new trust-management subsystem, history rewriting, blanket
foreign-branch adoption, unrelated health/corpus repair, installer/release work, and relaxation of
dirty-worktree, synchronization, lease, or authorization gates.
