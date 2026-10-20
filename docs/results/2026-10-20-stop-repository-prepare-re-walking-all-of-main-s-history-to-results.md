<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0574 — Recognize the docket branch by its shape in the everyday repository commands](../changes/active/0574-stop-repository-prepare-re-walking-all-of-main-s-history-to.md)**
<!-- docket:backlink:end -->

# Results: Recognize the docket branch by its shape in the everyday repository commands

**Human action:** None required to merge. The one Optional item below is a quick timing check you can repeat yourself.

## Outcome

`docket repository prepare`, `check`, `repair`, `configure-tests`, and `configure-harnesses` now decide whether the remote `docket` branch belongs to docket using two quick structural checks. The branch must have exactly one starting commit, and it must share no history with the integration branch. They no longer search all of `main`'s history for the snapshot the branch was seeded from. `repository init` and `repository migrate` still run the full content proof, unchanged. ADR-0157 records the split.

On this repository, with the new binary:

| Command | Before | After | Result |
|---|---|---|---|
| `repository prepare --json` | 68.7s | 3.1s | no-op, healthy |
| `repository check --json` | about 70s | 4.0s | healthy |
| `repository repair --json` (preview) | about 70s | 3.7s | no-op, healthy |

The `metadata-root-foreign` finding now says in plain words what was found and what to do. Prepare and check share one copy of that text.

## Human actions and testing

- **Optional: repeat the timing check.** Prerequisites: a checkout of this branch (or of `main` after the merge) and Go installed. Steps: `go build -o /tmp/docket-574 ./cmd/docket`, then `time /tmp/docket-574 repository prepare --repo-dir <your docket checkout> --json`. Expected: it finishes in a few seconds with disposition `no-op` or `applied`. Cleanup: `rm /tmp/docket-574`. Nothing else changes.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the final head. The repoownership shard took 58s and the bashupgrade shard 40s. Neither was flagged in the budget report. The budget-watch lines on the run are all on unrelated shards (rebase, recordops, repovisibility, state).
- Mutation probes: removing each structural check (more than one root, shared history, unknown integration tip) turned its test red. So did dropping the full verifier's reset to unknown, and so did each of the four wiring swaps (check, prepare, repair, migrate). Breaking the legacy-equivalence proof turned the new Bash-upgrade migrate leg red.
- The everyday commands accept a receiptless, nonempty seed root that the full verifier rejects. Migrate still refuses it, and both refusal paths are pinned by tests.
- Review: a deep-tier review found one minor issue and no blockers or importants. The minor issue was that the real Bash-made fixtures no longer exercised the full proof once prepare and check stopped running it. It was fixed in 7d83a4d05: a `repository migrate` preview leg in `TestIntegrationBashUpgradeOwnership`.

## Known issues and follow-ups

- **Only one saved Bash-made case reaches the legacy-equivalence proof.** In `TestIntegrationBashUpgradeOwnership`, the v0.9.3 case goes through `verifyLegacyEquivalence`. The v0.9.2 case is proven another way (it stayed green when the proof was broken). That other path was not mutation-tested in this change. This is confirmed and has low impact: migrate's code is unchanged. No action is needed unless that path is changed later.
- **Expected behavior change:** a docket branch with a missing or invalid seed receipt, but the right shape, now reads as healthy to prepare and check. Migrate and init still refuse it. If such a repository also still has its planning files on `main`, prepare points at `repository migrate`, which then refuses with a clear message. This is deliberate (ADR-0157).
