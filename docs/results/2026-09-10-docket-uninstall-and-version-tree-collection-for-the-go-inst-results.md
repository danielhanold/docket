<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0323 — docket uninstall and version-tree collection for the Go installer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0323-docket-uninstall-and-version-tree-collection-for-the-go-inst.md)**
<!-- docket:backlink:end -->
# docket uninstall and version-tree collection for the Go installer — Results

## Outcome

Complete. All nine plan tasks are implemented and committed on the feature branch, the whole-branch deep review returned no blockers, its three findings are fixed in-branch, and the full suite is green. The change delivers:

- `docket uninstall` for all recorded harness integrations or an explicit harness selection — ownership-gated removal, deduplicated harness filters, dry-run, journaled recovery, a valid reinstallable empty state, and idempotence. Uninstall retains the CLI binary, global configuration, contributor source checkouts, and repository-local setup; there is no force/purge mode.
- `docket install collect`, plus automatic best-effort collection after every successful (and no-op) release install, development-install candidate, and uninstall. Collection reclaims verified version trees — current and legacy/mixed-version alike — that no remaining installation references, and preserves any unprovable target.
- A strict, resumable collection journal; cleanup failures are reported separately from installation success (a `collection-pending` warning carrying the pending paths and the `docket install collect` retry) and never reclassify the primary operation.

This run was a resume: it adopted the prior premium worker's uncommitted-but-correct Task 8 edits (verified, not discarded), finished Task 8, executed Task 9, then completed the review + in-branch fix loop.

## Verification performed

- Authoritative full build gate `go run ./cmd/docket development test` — green at the final branch head, driven through the native gate driver; build-evidence recorded and verified against HEAD. The Task 9 certification run reported `SUITE files=46 passed=46 failed=0 asserts=399` (~263s); the post-fix certification run was likewise green.
- Test-driven throughout, with mutation evidence on the load-bearing gates: the Task 8 post-commit ordering gate (three separate mutations — collection moved before state publication, allowed after a refused plan, and skipped on the unchanged path — each reddened a dedicated fixture, then restored); the review-important refactor's re-proof characterization tests for both collector callers; the review-minor transient-scratch skip (RED→GREEN).
- Hermetic four-harness lifecycle smoke test under temporary `HOME`/`XDG_DATA_HOME`/`XDG_CONFIG_HOME`/`XDG_BIN_HOME`, with the real machine install untouched. Installed all four harnesses (one shared version tree); retargeted one harness to a second valid tree with a distinct asset-set id; uninstalled one owner and proved both referenced trees survived; uninstalled the final owners and proved only the unreferenced, verified trees were collected; proved the CLI binary, seeded global config, source checkout, `.docket` repo-local marker, and `.docket.yml` all survived a full uninstall; reinstalled and confirmed `install check` returned to active.
- Destructive-boundary audit (whole-repo `rg` over `Remove`/`RemoveAll`/quarantine-rename/managed-block removal): no collector path calls `os.RemoveAll` on a collection candidate; every deletion is entry-by-entry with symlink/non-regular refusal and empty-directory-only directory removal, dominated by ownership + reference + containment + kind proof and re-proven immediately before the quarantine rename. Uninstall and collect load no configuration and no repository state. No accepted ADR was rewritten.
- Metadata-branch verification (read-only, outside the feature suite): change 0323 still links its spec and now cites ADR-0096, ADR-0110, and the three decisions recorded for this change (ADR-0121, ADR-0122, ADR-0123).

## Human testing

The hermetic smoke test above exercises the full install → retarget → partial-uninstall → full-uninstall → reclaim → reinstall lifecycle across all four harnesses end-to-end, which the unit suite does not. Residual scenario worth a human's eyes before wide use: a real-machine run against genuinely pre-existing, human-authored harness integration files (ownership-refusal behavior on files docket did not record) — the automated coverage uses docket-created state, so ownership-refusal on truly foreign files is proven by unit tests rather than a live foreign-file fixture.

## Findings and limitations

- Whole-branch deep review returned **no blockers**. Its three findings (1 important, 2 minor), all on this branch's own diff, were fixed in-branch — see the PR body disposition table.
- Incidental, untested error-string convergence from the review-important refactor: on the escaped-versions-root re-check, the empty-release adapter now emits the general collector's more informative "refusing escaped collection candidate <path>" message instead of its former wording. Status is still `Failed`, the entry `Detail` is unchanged, the pass still breaks, and no test asserts that text; behavior is otherwise identical.
- A leaked `.staging-<id>` scratch directory under `versions/` (from a crash mid-`EnsureVersionTree`) is no longer surfaced as an unclearable `collection-pending` warning; it is skipped from collection candidates by its docket-owned prefix. Real version trees (`sha256-…`) never carry that prefix.
- Prior resume history (audit trail): this change halted across several earlier resume attempts on native-gate handoff/ownership seams (focused-gate credential capture using a reserved zsh variable; an `invalid-request` GREEN gate-start; an installed-vs-source `docket-build-task` contract mismatch; a Task 7 mutation-gate response gap; and the Task 8 `handoff-outstanding` takeover). This run adopted the preserved Task 8 edits and completed without re-hitting those seams. One operational note surfaced: once the run epoch owns the worktree's gate-admission slot, task-owned `gate.drive.start` calls must present that run epoch or they are refused `stale-run-epoch`.

## Follow-ups

None requiring capture beyond already-tracked work. The review's important finding (destructive re-proof/quarantine duplication) was resolved in-branch rather than deferred.
