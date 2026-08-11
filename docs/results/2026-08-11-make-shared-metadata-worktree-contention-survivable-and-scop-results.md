<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0247 — Make shared metadata worktree contention survivable and scope its commits](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0247-make-shared-metadata-worktree-contention-survivable-and-scop.md)**
<!-- docket:backlink:end -->

# Make shared metadata worktree contention survivable and scope its commits — results

Change: #0247 · Branch: feat/make-shared-metadata-worktree-contention-survivable-and-scop · PR: <url> · Plan: docs/superpowers/plans/2026-08-11-make-shared-metadata-worktree-contention-survivable-and-scop-plan.md · ADRs: 0089

## Verify (human)

- [ ] **Nothing here is a fixed finding** — every review finding was repaired in-branch and is read from the PR body's disposition table, not from this list. The one genuinely manual check is below.
- [ ] **Main-mode posture is a judgement call worth your eye.** Preflight now **aborts** when the primary worktree is not on `INTEGRATION_BRANCH`, rather than rebasing whatever branch is checked out. That is a deliberate behaviour change for `metadata_branch: main` repos — the configuration chosen by people who do not want extra worktrees, where sitting on a topic branch is normal. The alternative (skip-and-continue, as `sync-integration-branch.sh` does) was rejected because preflight is a gate, not a best-effort pass, and in main-mode the caller's very next act is a metadata commit that would land on the topic branch. Docket's own repo is docket-mode, so **the suite cannot exercise your judgement here** — only the mechanics.

## Findings

**ADR-0089 — survivable, not impossible.** Records keeping the single shared `.docket` worktree with a bounded discriminating retry, over per-session worktrees (heavyweight lifecycle machinery ahead of #0008, which is deferred) and advisory locks (cannot span tool calls; strand on crash). Also records the correctness-over-availability halt posture behind `blocked-wedged-tree`. #0008's revival is named as the explicit re-opening point.

**A blocker this change itself introduced, found by review and empirically proven.** The new fetch-first fast path returned success on a wedged (mid-rebase) shared worktree, because during a conflicted rebase HEAD is detached at the rebase's `onto` commit — so `HEAD == FETCH_HEAD` and the ancestor test passed trivially, before the wedged probe was ever consulted. Where the old bare `pull --rebase` exited 128, the new code exited 0. The reviewer carried it end to end: after that green preflight an agent's *correctly scoped* commit lands on the detached HEAD, the branch ref never moves, and `rebase --abort` reports `file present after abort: NO - LOST`. Preflight is the only mechanical gate the agent channel has, so this was the change's own headline defect reintroduced one layer up. Fixed by hoisting the wedged probe above the fast path and adding a detached-HEAD arm.

**The plan's central premise about git was backwards, and measuring it changed the design's justification.** The plan asserted that a `--` pathspec makes a commit exit 128 mid-rebase where the pathspec-less form exited 0. Measured on git 2.55, the reverse holds: mid-rebase-with-conflicts the *pathspec* form exits 0 and writes onto the rebase's detached HEAD, while the pathspec-less form is refused with "Committing is not possible because you have unmerged files". So scoping the commits **removes an accidental protection** — which does not weaken the case for scoping, but makes the explicit wedged probe load-bearing rather than belt-and-braces. Recorded in ADR-0089 and in the code comments, which now state the measured behaviour rather than the plan's.

**Plan-supplied test code was defective in roughly twenty distinct places** — the dominant failure mode of this drain (tracked as #0292), reproduced here at full strength. Two were severe enough to name:
- `logical_lines` never passed its `FILE` argument to awk, so it read stdin. The Group A scanner would have shipped **permanently vacuous** — green, and checking nothing.
- A fixture cleanup ran `rm -rf "$(git -C "$MW" rev-parse --git-dir)/rebase-merge"`; under `-C` that prints the *relative* `.git`, so it resolved against the test's own cwd. Running the suite from the repo root while mid-rebase would have deleted the developer's real rebase state.
Others: a permanently-vacuous rebase-state assert (in a linked worktree `<dir>/.git` is a pointer file, so `mkdir -p "$wt/.git/rebase-merge"` silently does nothing — this trap was hit and fixed **three separate times** on this surface), a `pipefail` producer-into-`grep -q` violation, an unsatisfiable mutation-table row, a `--autostash` repo grep failed by the implementation's own explanatory comment, a wrong branch variable in main-mode, and inverted redirections that discarded `rebase --abort`'s errors.

**Four mutation probes were green on first run** — i.e. the guard they tested was decoration — and each was fixed by adding a control rather than by explaining the green away: the scanner's second segment-splitter arm, its comment strip, `learnings_pass`'s new token arm, and the sweep's step-6a wedged arm. One more (`M2`, the in-loop wedge re-probe) was green until a `nosync` fixture was added. A sixth probe silently failed to apply and produced a meaningless green, caught only by the before/after `grep -c` this repo's rules require.

**Half 3's coverage derivation had a real hole review caught.** Keying on `skills/*/SKILL.md` structurally could not reach `skills/docket-convention/references/terminal-close-out.md` — the *declared single source* for two agent-authored commits into the shared tree. The in-file marker in `docket-finalize-change/SKILL.md` sits on the harvest commit, so the guard was green while the instruction actually being followed was unmarked: a false green in the exact channel Half 3 exists to guard. The population is now derived over `skills/**/references/*.md` by a bidirectional verb↔`metadata_branch` binding, which also pulled in `gate-failure.md`.

**Reconciled with change 0208 rather than minting a second notion of scope.** `worktree-scope:` is 0208's declared frontmatter fact on agent sources; Group B2b reuses it as a reverse-direction floor (every metadata-scoped agent wrapping an operating skill must appear in the derived set — five of the seven; the other two are interactive and wrapper-less by construction). Its parser is keyed on shape, not list position, after review of how `skills: [...]` ordering varies.

## Follow-ups

**No stubs were minted.** Two candidates surfaced and both were suppressed as duplicates of existing changes rather than filed: the budget regime is already owned by #0251, #0273, #0280 and #0289, and nothing else this run surfaced clears the admission gates as distinct beyond-the-branch work. Every review finding was in-branch and is accounted for in the PR's disposition table.

**Runtime-budget margins, reported as numbers rather than "did not trip the check"** (learnings: `budget-headroom-is-spent-before-it-is-breached` — parity is the finding, not the breach):

| File | Measured (serial) | Row | Margin |
|---|---|---|---|
| `tests/test_docket_status.sh` | 42.0s | 45 | **~3s** |
| `tests/test_docket_preflight.sh` | 5.8s | 10 | 4.2s |
| `tests/test_shared_worktree_commit_scope.sh` | 1.7s | 10 | 8.3s |
| `skills/docket-status/SKILL.md` | 2478 words | 2500 | **22 words** |

The first and last are the ones to watch, and they are coupled to queued work: **#0118 and #0268 are both queued on `scripts/docket-status.sh`**, and its test file now has roughly one fixture of room. Sharding was considered and declined here — the file is still under ceiling, the runner did not flag it, and re-cutting a 4700-line test file would collide head-on with exactly those two changes. That decision is cheapest for whoever lands next on that surface, which is why the number is recorded rather than the verdict. `skills/docket-status/SKILL.md` at 22 words of headroom is inside change 0137's within-25 clause, so the next prose addition there must raise the row and carry the change-0201 in-diff argument.

**`tests/test_docket_config.sh` runs ~135s against a 55s ceiling.** It does not report OVER BUDGET because `run-tests.sh`'s slack factor (change 0229) absorbs 2.5x, and it is pre-existing and unrelated to this change — but at ~2.45x it sits close to that limit and will breach on a slower machine. Not filed separately: #0251/#0273/#0280 already own this regime.

**Plan deviations, all deliberate and argued in-diff:**
- The rebase target fix (refuse a wrong-branch or detached tree; read `refs/remotes/<remote>/<branch>` rather than `FETCH_HEAD`) was made **uniform across both mode arms**, not just main-mode. The spec requires both branches of the sync function to behave identically, and a shared helper resolving its target per-caller is how they drift apart.
- The in-loop wedge re-probe was placed **before `pull --rebase`**, not before `rebase --abort` as review suggested. The abort-adjacent placement would refuse to abort a rebase the loop itself started on a conflict, stranding a wedge the function owns. The chosen placement is the only point where any in-flight operation is provably someone else's. It **narrows rather than closes** the window — a rebase started between the probe and git's own start-up is unattributable — and the comment now says exactly that instead of claiming closure.
- Six skill/reference word budgets were raised, two of them rounding-only rather than overflow fixes. Rounding raises cannot be mutation-proven; they rest on change 0137's within-25 clause as written.
- Task 4 additionally swept `blocked-wedged-tree` into `skills/docket-status/SKILL.md`'s three token enumerations. Without it a `docket-status` agent would have read a wedged board pass as success — in-branch work this change's own new token created, not scope creep.
