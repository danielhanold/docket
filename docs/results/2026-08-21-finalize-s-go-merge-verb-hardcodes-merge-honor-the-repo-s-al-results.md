<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0336 — Finalize selects the best merge method permitted by repository and branch policy** — `docs/changes/active/0336-finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al.md`
<!-- docket:backlink:end -->
# Finalize selects the best merge method permitted by repository and branch policy — results
Change: #336 · Branch: feat/finalize-s-go-merge-verb-hardcodes-merge-honor-the-repo-s-al · PR: <url> · Plan: docs/superpowers/plans/2026-08-21-finalize-effective-merge-method.md · ADRs: 10, 11, 43

## Verify (human)

<!-- GENUINELY MANUAL checks for the merge gate. GitHub's live repository fields, active-rule
     payloads, and merge behavior are outside-repo truth — no in-repo test can be their oracle. -->

- [ ] **Live capability payloads.** Observe the real repository and effective base-branch capability payloads on a live GitHub repository (`gh api repos/<o>/<n>` and `gh api repos/<o>/<n>/rules/branches/<branch>`) and confirm the decoded fields and token spellings match what `probeRepoMergeMethods` / `probeBranchMergeRules` expect.
- [ ] **Rebase-first path on this repo.** Finalize change 0336 itself through this repository's rebase-first path (rebase and squash enabled, merge commits disabled) and confirm the selected method is `rebase` and the finalize document reports it in the `method` field.
- [ ] **Squash-only fallback.** Finalize a scratch squash-only repository to certify the last-priority (`squash`) fallback and its reachability proof.

## Findings

- The whole-branch review (docket-review-deep) returned **0 findings** (0 blocker / 0 major / 0 minor). Full suite green: 121/121 files, 9366 asserts.
- No ADRs were produced by this change; it cites the pre-existing finalize decisions ADR-0010, ADR-0011, ADR-0043.
- **Plan/authored-tree deviation (Task 7).** The plan named the embedded mirror `internal/assets/embedded/tree/skills/docket-finalize-change/SKILL.md` as the edit target, but the authored root is repo-root `skills/` (per `assets.DefaultAllowedRoots`); `internal/assets/embedded` is its generated mirror. The identical prose edit was applied to the authored `skills/docket-finalize-change/SKILL.md`, then `go generate ./internal/assets/` regenerated the embedded copy and `manifest.json` — obeying the asset-bundle-drift guard's own remedy rather than hand-patching the frozen mirror (learning: `config-edit-trips-its-own-frozen-drift-guard`).
- **Resume note.** This change was resumed from a `## Run halted` marker. The halt was a spec-internal contradiction at Task 6: the fixed rebase→merge→squash order makes rebase the effective default, but `finalizeCleanupLocalRef` proved merge-chain containment against the original PR head, which is never an ancestor of the integration tip after a rebase/squash merge (`tip-not-in-merge-chain`). The human amended the spec to bring that one cleanup predicate in scope; Task 5a re-keys the containment proof on `facts.MergeCommit` (validated with `validFullObjectID`), graph-shape-independent, while the tip-identity check and `DeleteLocalBranchChecked` lease still key on the head.

## Follow-ups

<!-- None. -->

## Suite timing note

The full suite reports an advisory `OVER BUDGET` line (exit 0, not a failure) for a set of config/harness/sync/board test files (`test_docket_config`, `test_go_toolchain`, `test_harness_defaults*`, `test_render_board`, `test_sync_agents*`). These are pre-existing, machine-dependent wall-clock findings unrelated to this change's test files; per `scripts/run-tests.md` the slack factor is calibrated to one machine and the run does not gate on it.
