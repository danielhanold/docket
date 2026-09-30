<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0469 — Replace opaque docket terms with clearer names](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0469-replace-opaque-docket-terms-with-clearer-names.md)**
<!-- docket:backlink:end -->

# Replace opaque docket terms with clearer names — design

## Goal

Rename the docket terms that do not collide with anything but are hard to read without their glossary entry, so the docs, skills, run logs and verdict lines read plainly to a newcomer, human or agent. Retire two names that no longer match any code.

The authority for every name is **ADR-0129, family (e), rows 67-86**, edited in place at this change's grooming (2026-09-30). This spec adds the delivery rules; it does not restate the table. A deviation from a row goes through ADR-0129's *Deviations* rule, never a silent divergence.

## Decisions settled at grooming

- **Hard cut, no aliases** (ADR-0129 Decision 2). The stub's alias/deprecation-window plan is dropped: the CLI still has no alias mechanism, and docket's own consumers switch in one step through `docket install`.
- **No dependency on 0471.** Change 0471 (and 0468, 0472, 0473, 0474, 0477) are merged; nothing in this change waits on another.
- **Scope.** Seven wire rows (67-73), eleven prose rows (74-84), two retirements (85-86). The stub's other rows are dropped for the reasons ADR-0129 family (e) records under "Considered for family (e) and not renamed" (config keys and frontmatter fields under Decision 9, `abstain` under Decision 8, the protocol-v1 `disposition` key, `--owner-gen`, continuation id, `repository.sync-integration`, and so on).
- **Three suggested names were corrected by the code trace:** `identity-mismatch` is a whole-worktree fingerprint change, not "head changed" (→ `worktree-changed`); `unresolved-execution` is an unconfirmed launch, not an unconfirmed shutdown (→ `launch-unconfirmed`); `--owner-gen` is an opaque fencing token, not a counter (kept).
- **`change repair-identity` → `change relink`** was added by the human at grooming, together with its result tokens, the finalize merge condition `pr-identity-mismatch` → `pr-link-mismatch`, and the recertify reason `identity-drift` → `certified-input-changed`.

## Delivery

One change, one PR.

### Wire rows (67-73)

- **Derive every site with a whole-repo grep, never a hand list** (AGENTS.md *Guards and tests*). Search for the quoted token, its Go identifier forms (`ReadyNeedsBrainstorm`, `ErrUnresolvedExecution`, `OperationChangeRepairIdentity`, `RepairRepairedBranch`, `ReasonRecertifyIdentityDrift`, `PRIdentityMatch`, …) and its prose spellings. Include `*_test.go`, golden files, the schema registry and its vocabularies, the capability catalog, CLI help text, skills, agent wrappers, and the generated dispatch material (`internal/harness`, the embedded cursor rules). Go identifiers follow their row (ADR-0129: "Go identifiers follow their row's term").
- **Row 67** changes the board cell and the `status` readiness value. The board renderer's comment table in `internal/render/board.go` and `internal/domain/readiness.go` are the anchors. `auto-groom blocked — needs you` is unchanged.
- **Row 70** renames the operation id, the CLI verb and its schema/catalog entries. Its flags are unchanged. The finalize skill's two recovery commands ("Trust the PR" / "Trust the record") switch to `change.relink`.
- **Seal.** Append each retired wire token (rows 67-73) to the existing retired-vocabulary table in `internal/repoguard` with its replacement, so the absence seal refuses the old spelling in maintained source and names the replacement. Mutation-test every appended row: re-introduce the old token and watch the seal redden.

### Prose rows (74-84)

- Rewrite maintained prose only: `skills/`, `agents/`, `docs/guide/`, `docs/concepts/`, `docs/reference/` (glossary included), `README.md`, code comments, and the generated dispatch material that `docket install` writes into CLAUDE.md / AGENTS.md. Edit the generator source and regenerate the managed blocks; never hand-edit a generated block.
- **Row 80 (Step-0 preamble → startup check)** renames the convention's `### Step-0 preamble (every operating skill)` heading and every skill pointer to it in the same commit, so no pointer dangles. The phrase "Step-0" as a step number (for example implement-next's "step-0 implementation preflight") is a step label, not this term; leave it unless the sentence names the preamble.
- **Row 74** includes the Go type `MergeConjuncts` and its methods (→ `MergeConditions`), and comment uses of "conjunct(ion)" in the run tracker, guardian and finalize code.
- The glossary merges or renames entries to match: *Admission slot* → *Worktree slot*; *Native supervisor / gate execution* folds into *Gate run / run dir* with "gate supervisor"; *Liveness probe / liveness transition* keeps the probe and uses "moved to background"; *Presence-encoded section* → *Marker section*; *Learnings index / pay per relevance* → "read on demand"; *Identity repair* → *Relink*. No old→new mapping is written into the glossary (Decision 10).

### Retirements (85-86)

- **Row 85.** Rewrite the convention's *Bootstrap guard* paragraph that names "the `BOOTSTRAP=` verdict — `PROCEED` / `STOP_MIGRATE` / `CREATE_ORPHAN`" so it describes `repository.prepare`'s `applied` / `no-op` / `refused` dispositions and their human-typed remedies. Keep the 2×2 table's meaning. Move the old names to the glossary's "Obsolete terms" section.
- **Row 86.** `docs/guide/capturing-work.md` tells readers to run `docket status --digest-only`, a flag that does not exist. Replace it with the real `docket status` invocation, and move *Digest / digest-only read* to "Obsolete terms".

## Out of scope

- The dropped rows (see ADR-0129 family (e)), config keys, agent names, frontmatter fields (Decision 9).
- Frozen records: archived changes, merged plans and results, specs, and Accepted ADRs other than ADR-0129's own table (AGENTS.md *Comments and cross-references*).
- Persisted local state migration. Drive records that carry a renamed halt cause are not rewritten. The change lands with no gate drive in flight (ADR-0129 Consequences, family (e)).

## Landing and upgrade

- Land with **no gate drive in flight** in any docket repo on the machine, then run the post-merge binary rebuild immediately (AGENTS.md *Rebuild the binary after a merge to main*).
- Consumer repos re-run `docket install` to pick up the renamed dispatch material. The results file's `**Human action:**` says so.
- The committed `BOARD.md` keeps `needs-brainstorm` cells until its next re-render; 0469's own close-out commit re-renders it.

## Testing

- Run the whole suite at the build gate (`build.test_command`), and read the budget report.
- Update golden output and test fixtures that assert the old tokens; a test that asserted an old token should now assert the new one, not be deleted.
- Mutation-test each appended repoguard seal row (see *Seal*).
- A final maintained-source grep for every old spelling in rows 67-86 comes back empty, except inside the retired-vocabulary table, the glossary's "Obsolete terms" section, ADR-0129, and frozen records.
