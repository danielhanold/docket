<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0327 — Stacked-merged close-out can stamp `done` after a stale-worktree rebase clobbers the child — prove reachability in git, not metadata](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0327-stack-closeout-must-prove-integration-reachability.md)**
<!-- docket:backlink:end -->

# Change 0327: Verify stacked work survives rewrites and reaches integration

## Decision

Keep change 0327 and groom it against the Go implementation. Its remaining defect is real: merged PR destinations establish a stack relationship, but do not prove that the destination still contains the child's work.

Preserve Docket's existing rebase → merge commit → squash selection. Add one shared, conservative Git proof used before a carrying branch is rewritten or published and before a stack is archived. Original commit ancestry is sufficient. When a rewrite changes commit IDs, an exact comparison of the source change's tree entries can also prove preservation. An uncertain answer stops the operation with recoverable work retained.

This is a design specification, not an implementation plan.

## Necessity assessment, 2026-09-07

Inspected fresh `origin/main` at `0d1a7e1b36af36c1f805952cd3836e84dbad5b6f`. The relevant source and test files are identical in the local checkout used for the targeted test.

- `FinalizeRebase` already fetches the effective base and feature branch, requires the local, remote, requested, and PR heads to agree on a fresh attempt, and records `OrigRemoteHead`. The publication path already uses the receipt's explicit remote-head lease. These cover the original stale-worktree/implicit-lease mechanism; do not rebuild them.
- `closeoutIntegrationDestination` already proves the root PR's merge-result commit is an ancestor of a freshly fetched integration tip.
- `DeriveRootCloseoutSet` and `proveCarry` check statuses, stack topology, and merged PR destinations. They perform no Git proof of descendant content.
- `probeDescendantFacts` carries descendant merge-result IDs, but closeout never checks their existence or preservation before archiving.
- `TestIntegrationFinalizeCloseoutRootCarry/all-proven-archives-root-and-descendants` passes while the child facts contain a fabricated `bbbb…` merge commit. Verified with `go test -tags=integration ./internal/app -run '^TestIntegrationFinalizeCloseoutRootCarry/all-proven-archives-root-and-descendants$' -count=1 -v` (PASS). This demonstrates the missing proof, not reproduction of the historical production incident.
- Changes 0298 and 0316 are done; 0336 is also done and makes rebase the preferred merge method. Changes 0369 and 0370 completed the Go consumer migration and Bash removal. No active change found replaces this work; 0380's “descendant” is a metadata-history ownership fixture, unrelated to stack delivery.

## Goals

1. Refuse to operate on a carrying branch that has lost work already recorded as merged into it.
2. Refuse to publish a rebase or conflict resolution that loses that work, even when the test suite passes.
3. Archive a stack only after independently proving each carried descendant at the root's actual integration merge.
4. Accept verifiable rebase and squash results without requiring the original child commit ID to survive.
5. Preserve atomic closeout, exact-head authorization, explicit push leases, and safe recovery.

## Assumptions and the limit of the proof

- The user selected support for rebase/squash with verified preservation during grooming.
- Authoritative PR identity, merged state, destination, and merge-result object ID still come from the existing GitHub adapter. A branch name, board row, status, PR description, or green test report cannot substitute for Git evidence.
- A proof of historical inclusion by commit ancestry remains sufficient, as it is today for ordinary changes. Detecting an explicit later revert of an ancestral commit is outside this change.
- The fallback proves exact recorded content, not semantic equivalence. A legitimate later edit to the same file can be unprovable, even if a human judges it equivalent. Such a case is reported as **preservation unproven**, never as a demonstrated loss.
- The fallback supports rewrites that preserve the source changes exactly, including unrelated destination changes. It does not promise automatic acceptance of every conflict resolution, rename transformation, or same-file follow-up edit.
- Missing objects, incomplete history, unsupported plumbing, and uncertain external observations never become successful proof. There is no “trust metadata” or “tests passed” override.

## Preservation primitive

Add a narrow typed Git adapter operation with a result distinguishing `proven`, `unproven`, and an observation error. It accepts immutable commit IDs `source` and `target`, and reports the proof kind and bounded diagnostic details. The app owns lifecycle decisions; the adapter owns Git mechanics. `internal/domain` remains free of I/O.

Here `source` is the child's authoritative **merge-result commit**, not its original PR head. A rebase or squash into the parent may already have changed that head.

Algorithm:

1. Validate and resolve both IDs as real commit objects, using the repository's supported full-object-ID validation. Establish complete enough history to answer the query; a shallow boundary or unavailable required object cannot produce a negative or positive fallback by accident.
2. If `source` is an ancestor of `target`, return `proven / ancestry`.
3. Otherwise compute **all** best common ancestors. The fallback requires exactly one. No common base or multiple bases is `unproven`; an inability to observe the graph is an error. Do not choose an arbitrary base, truncate a history walk, or enable unrelated-history merging.
4. Let that common ancestor be `base`. Derive the full tracked-entry delta from `base` to `source`, with rename detection disabled. Use NUL-safe structured records and full object IDs. The population comes from Git, never a list of file types or named child paths.
5. For every changed path, require the target entry to match the source's final entry exactly: object ID, mode, and type. A source deletion requires that entry to be absent in the target. Treat a rename as deletion plus addition. Cover binary blobs, symlinks, executable bits, gitlinks, and file/directory transitions. Extra target changes outside this delta are allowed.
6. A nonempty delta whose every entry matches is `proven / exact-content`. A differing entry is `unproven`. An empty delta in the non-ancestral arm is conservatively `unproven`, rather than manufacturing proof from an empty population.

This comparison reads Git objects. It uses no merge drivers, filters, patch-id whitespace normalization, filename exclusions, or conflict-resolution policy. It does not modify an index, worktree, or branch. Fetching a required exact object is permitted only through the typed Git adapter and the already established repository remote; inability to obtain it is an observation failure. Never substitute the tip of a same-named branch for a missing object.

The exact-content fallback deliberately favors false refusals over false delivery claims. For example, if the child added `catalog.yaml`, a squash with the identical blob passes; one that omits it fails. If the destination intentionally edited that blob after the child merged, the fallback refuses until a separately verifiable state exists. User approval of prose is not proof of that state.

## App-level proof collection

Use one shared app helper for descendant collection, authoritative merged facts, and the Git primitive. Proof results are scoped to immutable target IDs and fresh change/PR identities, not cached by branch name or lifecycle status.

For a **pre-merge carrying branch**, select descendants connected to it by an unbroken chain of `stacked-merged` changes whose verified PR destinations match their recorded parent branches. Walk transitively. Stop descending into an open intermediate child: grandchildren merged into that still-open child are not yet promised as carried by the root. Preserve structural refusals for malformed/cyclic/ambiguous records. A purported carried link with unknown or mismatched facts blocks; it is never silently omitted.

For **root closeout**, retain the current `DeriveRootCloseoutSet` topology and destination checks. Every member of the set must additionally pass Git preservation. Content similarity cannot rescue a failed relationship proof. Keep the structural/domain result distinct from the new Git evidence; comments must stop calling destination checks alone proof of shipped code.

Validate each descendant's merge-result ID before probing it. Include the descendant ID, canonical PR, source merge ID, target ID, and proof failure category in structured findings. Report missing evidence separately from a successfully observed mismatch.

## Enforcement boundaries

### Rebase and continuation

On a fresh `FinalizeRebase` attempt, keep all existing workspace/head checks. Prove the carrying descendants against the exact agreed pre-rewrite head before creating the rebase receipt or invoking `BeginRebase`. A refusal leaves the workspace, branch, receipt, and remote untouched.

After a completed rewrite, prove those carried descendants against the rewritten head before returning a successful result/permit. Apply this on the normal path, `FinalizeRebaseContinue`, and receipt recovery. Re-discover the current carried set when resuming; do not trust a boolean left in an old receipt. A failed post-rewrite proof retains the existing owned attempt and original refs for the existing abort/repair flow. It must not reset or discard the user's resolution.

The green local suite does not replace the preservation check. `finalize.gate: off` retains its meaning of no rebase/no local suite; it cannot disable the checks at publication, merge, or closeout.

### Publication and merge

`FinalizePublish` must freshly prove the carrying descendants against `req.Head` **before** calling the first remote rewrite operation. Cover recovered and direct invocations as well as the ordinary sequence. Keep the existing receipt identity and exact old-remote-head lease; content proof cannot authorize a different head or broaden a lease.

`FinalizeMerge` must prove the carried set against the exact freshly verified PR head before the external merge. This covers gate-off and direct-merge entry paths. Keep the existing single selected merge method, matching-head condition, open-child policy, and repository policy checks. No merge-method configuration changes and no fallback mutation after a denied merge.

A concurrent child merge changes the parent feature head; the existing explicit lease / matching-head authorization must reject that movement. New proofs complement those guards and do not replace them. A proof is invalid for any changed target ID; no caller may reuse it for a later head.

### Stacked and root closeout

Before marking a child `stacked-merged` into a live parent, fetch/pin the parent's remote head and prove the child's merge result preserved there. This also applies to the corresponding replay path; a historical PR destination is not sufficient evidence that the parent currently carries the merge.

For a root whose PR merged to integration:

1. Retain the existing fresh integration fetch and root merge-result ancestry check.
2. Derive and validate the complete root closeout set using live descendant PR facts.
3. For each descendant, accept direct ancestry of its merge-result commit in the pinned integration history. Otherwise require exact-content preservation against the **root's verified merge-result commit**.
4. Use that root merge result, rather than the current integration tree, for the fallback. This proves what the root delivered and avoids refusing merely because unrelated commits changed files after the root merged.
5. Only after all proofs succeed run the existing atomic root-plus-descendants archive/board/artifact transaction. Maintain exact record versions and the transaction's metadata lease. On contention, reload and redo the proofs for the new snapshot; never narrow the target set to get a commit through.

One unproven descendant leaves the root and every descendant unarchived, with their recovery resources retained. Finalize and maintenance sweep reach the same app operation. Already archived `done` records keep the existing idempotent short circuit; this change does not rewrite historical records or attempt a retrospective audit.

## Failure and recovery contract

- Proven relationship/content failure: blocked with a stable preservation reason and the affected child/target IDs.
- GitHub/Git/object-read failure: unknown/external failure using the operation's existing envelope conventions.
- Moved authorized head or metadata version: contended, using existing retry ownership rules.
- No external push, merge, terminal metadata write, or cleanup follows an unproven gate.
- A post-rewrite failure may leave local owned rebase state; explicitly retain it and direct the operator to the existing inspected abort/repair flow. Never suggest a force reset, an unleased push, or a manual `done` edit.
- Pre-upgrade stacks need no metadata migration. They qualify through fresh Git proof; unavailable historical merge objects remain recoverable and unproven.

Keep the closed operation result vocabularies. Add structured preservation reason/finding coverage to the schema and CLI mapping where required. No caller assembles ad hoc shell proofs.

## Tests and acceptance

Replace the fabricated-child positive root-closeout fixture with a real Git stack. Retain a separate negative fixture using an absent, well-formed merge ID; it must no longer archive anything.

Extend the existing topical Go tests and runner shards, observing `tests/README.md` and their actual budgets. Use real Git objects for graph/content assertions and fake GitHub only for authoritative external facts. Required coverage:

1. An ancestral child succeeds. Verifiable rebase and squash rewrites succeed with different original IDs, including unrelated destination advances.
2. A real, available but unreachable child whose content is absent refuses; so does partial content loss. Assert the source object exists to distinguish loss from missing-object handling.
3. An existing stale parent workspace refuses before receipt/rebase, preserving the already-shipped head checks. A local/remote/PR head that all agree but has lost a carried child also refuses, exercising the new proof rather than the existing mismatch guard.
4. A bad rebase/conflict resolution cannot publish even with green suite evidence; cover continuation, recovery, and direct publish. Assert zero remote rewrite calls and retained original refs.
5. Direct merge and gate-off paths enforce the same proof; a concurrent head move still fails the exact-head/lease checks.
6. A transitive carried stack verifies every descendant. An open intermediate branch's merged grandchild is not incorrectly treated as carried by the root at the pre-merge boundary. Unknown/mismatched carried links never disappear from the population.
7. A root merge result is reachable but a descendant was dropped: direct closeout and maintenance both leave all metadata bytes/ref tips, board, and recovery branches unchanged. Valid merge/rebase/squash stacks archive atomically with the existing root merge date.
8. Integration advances after a valid root merge: fallback against the root merge result still succeeds. Missing/invalid merge IDs, unavailable objects, Git failures, shallow/incomplete history, no common base, multiple bases, and non-ancestral empty deltas refuse without an unsafe effect.
9. Exact-content comparisons cover deletion, rename-as-delete/add, mode/type changes, binary content, symlinks, gitlinks, file/directory transitions, and unusual paths. A custom merge driver cannot manufacture a successful proof because it is never invoked.
10. Ambiguous overlapping edits refuse as unproven. Match fixtures against unchanged bytes and exact modes/OIDs, not filename presence alone.
11. Successful replay and metadata contention preserve the current transaction behavior; a failed proof never partially archives proven siblings or writes new authored notes.

Mutation-test each new enforcement boundary and the descendant-population derivation: bypass the proof or omit a transitive child and observe the corresponding specific test fail. Pin the missing push/merge/archive effect and reason, not merely a nonzero exit. Defeat Go's test cache for these probes.

At implementation's build gate run the whole suite from the current resolved `build.test_command`; at finalize use the resolved `finalize.test_command`. Both currently resolve to `go run ./cmd/docket development test`. Treat budget screening and serial-confirmed breaches as required by the repository instructions; do not raise a budget to accommodate new tests.

## Documentation, relations, and scope

Update maintained source comments and the canonical stacked-changes/finalize references so a merged destination is described as a relationship check and a verified carry is the promotion condition. Regenerate maintained copies through the supported install/generation workflow at implementation time. Preserve Accepted ADR-0092 and archived specs, plans, results, and change records.

Keep high priority, `depends_on: []`, and ADR-0092. Set `related: [298, 316, 336, 369, 370]`; these changes are already done. Do not set `stacked_on`: this fix is built from the normal integration branch.

Out of scope: resurrecting Bash; incident recovery; automatic worktree fast-forward/reset; changing the stacking model or merge-method preference; semantic patch equivalence; new user override/configuration knobs; shared proof-receipt infrastructure; retrospective repair of `done` records; and broad cleanup redesign.

The existing cleanup path conservatively refuses a carried child's PR whose destination is not integration. That is a separate retained-resource limitation, not permission to weaken cleanup here. This change must not delete those resources to make a stack look fully cleaned, and must not advertise that existing cleanup limitation as fixed.

## Alternatives considered

- **Ancestry only:** cheap and safe, but rejects the repository's normal rebase/squash workflow. Rejected by the user during grooming.
- **Ancestry plus exact changed-entry comparison (chosen):** deterministic, checks binary/mode/path information, works without trusting merge policy, and needs no new persisted state. Conservative refusal of overlapping edits is the deliberate cost.
- **Synthetic merge or patch-equivalence proof:** can recognize more equivalent rewrites but requires additional merge-driver, normalization, and ambiguity policy. A throwaway Git probe showed a synthetic merge could distinguish a preserved squash from a missing child, but that was not sufficient to choose it as the runtime proof. [Git's merge-tree documentation](https://git-scm.com/docs/git-merge-tree) describes the merge behavior and output; this design uses exact object comparisons instead.

Relevant learnings: `moving-base`, `groomed-root-cause-is-a-hypothesis`, `verify-the-claim`, `assert-pins-outcome-not-mechanism`, and `green-suite-untested-branch` informed the scope correction and required negative fixtures.
