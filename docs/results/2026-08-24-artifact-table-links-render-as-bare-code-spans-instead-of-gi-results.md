<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0341 — Artifact-table links render as bare code spans instead of GitHub links](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0341-artifact-table-links-render-as-bare-code-spans-instead-of-gi.md)**
<!-- docket:backlink:end -->
# Artifact-table links render as bare code spans instead of GitHub links — results
Change: #341 · Branch: feat/artifact-table-links-render-as-bare-code-spans-instead-of-gi · PR: <url> · Plan: docs/superpowers/plans/2026-08-24-artifact-table-links-render-as-bare-code-spans-instead-of-gi.md · ADRs: none

## Verify (human)

<!-- GENUINELY MANUAL checks. The hermetic suite cannot see the metadata branch or the real GitHub
     origin (learning: metadata-branch-invisible-to-suite), so the corpus heal and the end-to-end
     render can only be certified by eye against origin. -->

- [ ] **Metadata corpus healed on `origin/docket`.** Spot-read a few of the swept files at `origin/docket` and confirm the `## Artifacts` tables and spec `docket:backlink` blocks now render as `[…](https://github.com/…/blob/…)` links, not bare `` `docs/…` `` code spans. (Verified during build for the 0340 archive entry; confirm a couple more.)
- [ ] **End-to-end after merge + rebuild.** After this PR merges and `docket development install` rebuilds the binary, run any lifecycle op that re-renders a block (e.g. a claim/attach) and confirm the Go binary now emits a GitHub link — the root fix, not just the one-time sweep.
- [ ] **Excluded prose files intact.** Confirm the three deliberately-excluded files that *document* the markers are byte-unchanged from before this change: `docs/superpowers/specs/2026-07-23-artifact-backlinks-design.md`, `docs/superpowers/plans/2026-07-24-artifact-backlinks.md`, `docs/superpowers/plans/2026-08-22-finalize-backlink-leg-corpus-scoping.md`.

## Findings

- **Full suite green:** `SUITE files=122 passed=122 failed=0 asserts=9129 wall=594s`. Nine `BUDGET WATCH` lines (config/harness/sync/board files, consecutive-overrun streak 1/5) are screening findings only — parallel wall-clock is machine-dependent; there was **no** `SERIAL CONFIRMED OVER BUDGET` line, so per `scripts/run-tests.md` the run does not gate on them. Unrelated to this change's files.
- **No ADRs** were produced; the change carries no architecture decision.

- **Task 5 — metadata sweep (`docket` branch), commit `59a269a6`, pushed to `origin/docket`.** Swept 312 change files (30 skipped: pre-0035 stubs with no block — correctly *not* injected) and 137 spec backlinks (70 skipped: no block). 26 files actually changed (idempotent no-ops otherwise); all under `docs/changes/{active,archive}` and `docs/superpowers/specs`, marker-blocks-only. Real-tree check: `origin/docket`'s 0340 archive entry now renders GitHub links.
- **Task 6 — integration-line sweep (feature branch), commit `92a00bcd`.** Stamped 204 plan/results backlinks (181 skipped: no block); 16 files healed under `docs/superpowers/plans` and `docs/results`. Each surviving heal is exactly one backlink line changed (bare code-span → GitHub link, or a stale `active/` → canonical `archive/` path for a since-archived change).
- **Go/bash parity (Task 6 Step 3).** Ran the fixed Go verb `docket artifact backlink` on a bash-stamped file (`docs/results/2026-07-24-artifact-backlinks-results.md`, change `docs/changes/archive/2026-07-24-0136-artifact-backlinks.md`); it returned **"already current"** — a no-op, proving the Go renderer produces byte-identical output to the bash renderer on the same input.
- **Guard mutation probe (Task 4).** `TestLinkContextSoleConstructor` green at HEAD. Floor probe: raising `const floor` 17→99 reddened it (`linkContextOf used 17 times in production files, want >= 99`); restored to 17, green. `floor = 17` reflects the actual swapped call-site count (the plan's template said 15; adjusted upward to reality, never down).
- **Deliberate divergences from the bash renderer (for review, not accidents):**
  - *Task 1* (`gitcli.RemoteURL`): reads **raw** remote config via `git remote get-url` and deliberately does **not** apply `url.<base>.insteadOf` transport rewrites, so a transport rewrite cannot corrupt the derived web URL (`internal/gitcli/refs.go`).
  - *Task 2* (`parseGitHubWebURL`): an empty `owner/repo` remainder yields `""` (the bare-path fallback) where bash would emit a broken empty-`/repo` URL — `""` is the strictly safer reading of the degenerate input (`internal/app/link_context.go`).

## Follow-ups

- **Latent renderer fragility surfaced by this change's own sweeps (recommend a follow-up change).** `render-artifact-backlink.sh` and `render-change-links.sh` locate the managed block by a **fixed-string first-match** on the marker text, with no fence/prose awareness. Any artifact whose *prose or a code-fence example* contains the marker string is corrupted when swept: the renderer grabs the first textual occurrence (a fenced example, an indented bullet, a shell `START_MARKER=` line, a Go test literal) instead of a real top-of-file block. Three files hit this during the sweeps — the artifact-backlinks **design spec**, the 0136 artifact-backlinks **plan**, and the finalize-backlink-leg-corpus-scoping **plan** — all of which *document/test the backlink feature itself*. Each was restored to HEAD and excluded; the sweeps' marker-presence gate is not sufficient because the gate's own grep matches the prose mention. The Go port likely shares the first-match behavior. A hardening change should anchor the renderers on a genuine block (column-0 marker outside any `` ``` `` fence, ideally at the top) rather than the first textual hit — this is the same rendering-correctness family as change 0341.
- The two sweep scripts were one-time throwaways (not committed, per spec A6); their output was verified by hand (marker-blocks-only diff, per-file +1/-1 backlink-line check, casualty restore).

## Suite timing note

`SUITE … wall=594s` with nine advisory `BUDGET WATCH` lines (exit 0, not a failure) for pre-existing config/harness/sync/board files (`test_board_checks`, `test_go_toolchain`, `test_harness_defaults*`, `test_render_board`, `test_sync_agents*`). Consecutive-overrun streak 1/5, parallel wall-clock, machine-dependent; unrelated to this change's test files and not gated per `scripts/run-tests.md`.
