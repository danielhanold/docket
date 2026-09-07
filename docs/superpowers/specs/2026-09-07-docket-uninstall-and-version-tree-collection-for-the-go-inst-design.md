<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0323 — docket uninstall and version-tree collection for the Go installer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0323-docket-uninstall-and-version-tree-collection-for-the-go-inst.md)**
<!-- docket:backlink:end -->

# Change 0323: uninstall harness integrations and collect unused asset versions

Approved design, 2026-09-07. Automatic collection after successful installs, an explicit collection command, all-harness or selected-harness uninstall, and retention of the CLI binary.

## Problem and outcome

Release installation publishes immutable payloads under `<data-root>/versions/<asset-set-id>/assets`. Superseded payloads accumulate, and there is no command to remove installed user-level harness integrations. Provide a predictable removal path and reclaim verified payloads that no remaining installation needs.

Change 0311 supplied the installer, ownership record, extraction, exclusive lock, and rollback journal. Changes 0317 and 0322 supplied release and development binary installation; 0351 added repository surfaces with separate records and retired historical global dispatch material. These changes are done. No unmet dependency or stacked base is needed.

## Public behavior

Add these capability-catalog operations and their human/JSON contracts:

| Operation | CLI | Behavior |
| --- | --- | --- |
| `uninstall` | `docket uninstall [--harness <name>]... [--dry-run]` | Remove recorded user-level integrations for every installed harness, or the explicitly selected harnesses, then collect unused version trees. |
| `install.collect` | `docket install collect [--dry-run]` | Collect unused version trees without removing integrations. |

Both commands support the existing global `--json` flag. They resolve user roots without loading repository configuration, detecting a repository, rendering current wrappers, or requiring a compatible installed asset set. Their input authority is the installed ownership record. Unsupported or invalid ownership formats refuse safely. No installed record is a clean no-op for uninstall; collection without a record preserves version trees because it cannot establish the reference set. A valid empty record written by uninstall is distinguishable from missing state.

An omitted harness filter means all recorded harnesses, not the harnesses detected on the machine today. Explicit filters are validated together before mutation, deduplicated, and restricted to the supported harness vocabulary. Selecting a supported harness that is not recorded is a no-op. Records of other harnesses and unattributed records remain unchanged. An unknown recorded harness can be removed by the all-harness form when its target record is structurally valid; no current renderer is needed.

The command removes harness integrations and unused assets. It preserves CLI binaries and their ownership records, global configuration, contributor checkouts, repository metadata, repository-local instruction surfaces and ownership records, user-created files, and parent directories. Output explicitly states that the CLI and repository setup remain installed. There is no force or purge switch.

## Uninstall transaction

Use the existing exclusive installation lock for the entire mutation, including recovery, state reads, planning, removal, state publication, and subsequent collection. Recover abandoned installer journals before planning, and establish that no pending journal remains before collection. A lock conflict refuses with the existing contention diagnostic; never infer abandonment from a journal's age.

Plan removals from recorded targets only. Regular files must match their recorded digest and kind; symlinks must match the recorded canonical destination, including dangling-link handling; managed blocks must have valid balanced markers and the recorded interior digest. For a cleanly absent target, or a managed block already absent from a readable regular file, removal is satisfied. An inspection error is not absence. Any malformed, changed, or unprovable selected target refuses the whole uninstall before a target or ownership record changes. Report all discovered ownership conflicts together.

Delete only the recorded file/link or the proven managed block. Preserve all bytes outside a removed block, including an otherwise empty host file. Do not adopt unrecorded legacy artifacts during uninstall: the existing install/adoption path can record ownership first.

Reuse `BeginTxnWithRemovals` and the journal's rollback/recovery mechanism. Strengthen the removal path where necessary so the proof used to authorize deletion is checked against the captured pre-image and revalidated immediately before mutation. Today's whole-file removal exemption in `verifyPreImages` is insufficient for this contract. Deterministic tests must cover content edits, changed symlink destinations, and kind changes between inspection, capture, and apply. Do not claim protection against arbitrary hostile concurrent filesystem mutation beyond the supported filesystem primitives; the lock serializes cooperating Docket writers.

Publish the remaining targets and harness set in the same transaction as removal. Retain supported state identity/provenance fields needed for remaining targets. Once no harness targets remain, publish a valid empty-harness state, preserving any binary records and their provenance; clear the active release `asset_set_id` so it cannot falsely keep the final asset tree alive. Source provenance is not permission to delete a checkout. Readers that distinguish installed integrations from an uninstalled machine must recognize this empty-harness state; `RequireCompatibleInstallation` and `install check` must report installation required rather than treating the record's mere presence as an active installation. A subsequent install can repopulate this state normally and still prove ownership of a retained development binary.

Synchronous failures roll back selected targets and state. An interrupted transaction retains the existing durable recovery path. Uninstalling the same scope twice converges to a no-op.

## Reference calculation

Collection uses a set recomputed under the lock, not persisted reference counters. Protect the active release asset-set identity and every version reached by a recorded target path or symlink destination, across all harnesses. A scoped upgrade carries older target records through `desiredState`, so the top-level `asset_set_id` alone is not sufficient.

Canonicalize existing ancestors and every symlink hop when comparing paths. Also inspect the live destinations of recorded symlink targets: a user-retargeted link may introduce a reference beyond its recorded one. Preserve both recorded and observed references. Missing targets do not erase their recorded references. Any unreadable state, unsupported reference shape, or failed resolution that leaves reachability uncertain blocks the collection pass. Development source paths outside the managed versions directory are never collection candidates.

Collection does not crawl repositories or the user's whole home directory for arbitrary links. The ownership state is its declared reference boundary; documented direct references into internal version storage are unsupported.

## Proving a candidate tree

Only immediate, non-symlink version directories inside the canonical managed versions root can qualify. A directory name that resembles an asset-set ID is not enough. Verify its payload and reject symlinks, special files, missing or changed files, unexpected contents, and unexpected directories before authorizing deletion. Never follow a candidate link or recursively delete the data root.

Future extractions publish the canonical asset manifest beside `assets/` in the same staging-and-rename operation as the tree. The manifest is immutable, validates under the supported asset protocol, hashes to the directory identity, and describes every payload file. Updating this extraction layout must preserve reuse of prior valid trees without rewriting their payloads.

Existing protocol-v1 trees have no saved manifest. Support them by reconstructing the v1 manifest from the exact legacy asset layout, normalized manifest file mode, path roles, sizes, and digests, and requiring its computed asset-set ID to equal the directory identity. The v1 role/layout contract is frozen for this compatibility path; future generator changes must not broaden it. The walk must detect entries outside the known roots and unexpected empty directories, rather than silently omitting them. A mismatch preserves the whole tree and reports why. No network lookup or older binary execution is involved.

## Collection, interruption, and reporting

A successful install, development install, or uninstall invokes the same collector after its installation transaction is finalized and no rollback journal can restore older references. An otherwise unchanged successful install still runs collection. A failed or refused install never starts collection. `install check` remains read-only.

Use a small durable collection journal separate from rollback installation journals. Before retiring one eligible tree, record its canonical manifest, original identity, source and quarantine paths, and phase under the data root. Revalidate the candidate and reference set, then move it atomically into a private collection quarantine on the same filesystem. Journal reconciliation must handle interruption before or after that rename. Delete only verified payload entries and owned directories within quarantine, with no symlink traversal. A retry may accept already-absent recorded entries but must preserve unexpected or changed content. Keep the journal until cleanup is complete, making partial deletion resumable without accepting an unverifiable tree as owned. Never reuse `Recover` to restore a partially collected asset tree.

Installation success and collection completion are separate facts. Automatic collection failures preserve the successful install/uninstall result and emit structured warnings naming pending paths and a retry command. The explicit collection command exits nonzero if an eligible cleanup fails or an unknown/malformed candidate prevents a complete pass; its report distinguishes collected, referenced, unverified, and failed entries. It may report completed deletions before a later failure, because collection is independently resumable per tree. Unknown entries remain untouched.

Dry-run performs no writes, recovery, extraction, state publication, or collection-journal creation. It reports a pending recovery rather than performing it. Use a read-only locking/inspection path that does not create the data root or lock file on a fresh machine; contention or uncertain consistency refuses. The actual execution always replans rather than trusting an earlier dry-run.

## Implementation boundaries and validation

Keep planning and reference/proof functions independently testable in `internal/install`; place uninstall and collection orchestration in focused files, reusing existing state, filesystem, locking, and transaction primitives. Wire CLI, app results, capability catalog, operation schema descriptors, and asset-independent command classification together. Update `docs/install/install.md` and the relevant keeping-current documentation with removal scope, commands, retained files, recovery, and explicit collection failures. No configuration knob or scheduled daemon is introduced.

Use hermetic temporary homes and injected filesystem failures. Cover:

- Full/scoped removal across four harnesses; binary/config/source/repository preservation; idempotence; valid empty state and reinstall.
- Missing targets and blocks, edited files, malformed markers, changed kinds, canonical/dangling links, preflight-to-apply changes, rollback failures, and interruption/recovery.
- Two harnesses referencing different versions after a scoped upgrade; removal of only one owner; release/development transitions; both recorded and retargeted live references.
- Current-manifest and old v1 trees; added/missing/changed files; unexpected directories; symlink escapes; corrupt or missing state; unknown formats and unreadable references.
- Collection journal interruption at publication, quarantine rename, partial deletion, and final cleanup; added foreign quarantine contents remain intact.
- Automatic versus explicit failure semantics, unchanged-install cleanup, refusal without collection, dry-run byte preservation, and lock contention with installation.

Mutation-test deletion guards by disabling ownership, reference, and containment checks and proving the corresponding fixtures fail. Run focused package tests during development and the full suite using the resolved `build.test_command` at the build gate; act on serially confirmed runtime-budget breaches as required by the repository rules. Grooming itself makes no code changes and runs no implementation tests.

## Alternatives

Explicit collection only reduces automatic side effects but leaves normal upgrades accumulating disk usage until the user remembers to run it. The approved design uses automatic collection with an explicit command for inspection and retries. Full CLI removal would require coordinating the separate release downloader ownership record and its binary-publication sequence with the installer transaction; that extension is deferred from this change. Deleting directories by name or keeping only the latest top-level asset ID is rejected because neither proves ownership or accounts for mixed-version harness installations.

## Traceability

Keep `depends_on: []` and `discovered_from: [311]`. Record related changes 311, 317, 322, and 351. ADR-0096 supplies the precedent for a frozen compatibility proof; ADR-0110 describes install-only config tolerance, which this design does not expand because these new commands do not read repository or model configuration. At implementation, record the new decisions about state-derived reference calculation, post-commit collection, and CLI retention through the normal ADR step. No Accepted ADR is rewritten.
