<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0208 — Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0208-runner-dispatch-worktree-gate-3-proves-repo-containment-not.md)**
<!-- docket:backlink:end -->

# Harden runner-dispatch — worktree membership gate, feature-scoped coverage, flag-parse guards — results

Change: #0208 · Branch: feat/runner-dispatch-worktree-gate-3-proves-repo-containment-not · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-11-runner-dispatch-worktree-gate-3-proves-repo-containment-not.md · ADRs: 83 (new), 68 (`## Update`)

## Verify (human)

- [ ] **Re-run `sync-agents.sh` once on every machine with a docket install**, before trusting a
      `--check` drift result. `emit()` passes source frontmatter through verbatim, so the new
      `worktree-scope:` line now also appears in every generated Claude wrapper. Claude Code
      tolerates unknown frontmatter keys, so nothing breaks — but wrapper bytes changed, and until
      the regeneration runs, `--check` reports drift that is not drift.
- [ ] **ADR-0068's published copy on `main` is now stale.** Its `## Update` note landed on
      `origin/docket`, but 0068's bytes were already published onto the integration branch by an
      earlier change. `68` is in this change's `adrs:`, so finalize's terminal publish should carry
      the updated file across — confirm it did, or run
      `docket.sh terminal-publish --adr 68 …` by hand. (This is the
      `superseded-ADR-skipped-by-the-Accepted-gate` shape, reached from a different direction.)
- [ ] **Decide whether a linked worktree on the integration branch is acceptable.** Gate 3b rejects
      an anchor equal to the *main* worktree, which is a path-identity test. A second **linked**
      worktree that happens to have `main` checked out passes cleanly. No branch predicate is
      available — `docket-rebase-resolver` dispatches mid-rebase on a detached HEAD, so any
      branch-equality test would refuse the agent that most needs the gate. The residual is stated
      in `scripts/runner-dispatch.md`; it is a deliberate acceptance, not an oversight.
- [ ] **`tests/test_sync_agents_runners.sh` remains over budget** — 187–189s against a 60s ceiling.
      Pre-existing and tracked as change #0280. Measured serially at 93.4s before this branch's
      minor-fix batch and 97.7s after, i.e. unchanged within run-to-run noise (the same file
      measured 93.4 / 95.5 / 97.7 across three runs). The cost is the ~30 `sync-agents.sh`
      invocations, not the fixture copy this change added and then narrowed; no shard was taken on.

## Findings

### The change's own thesis, applied to its own diff — and it bit twice

This change exists because a gate keyed on an enumerated name shape (`build-*`) aged into the gap it
was written to close. The whole-branch review ran that thesis over the branch's **own** additions
(learnings: `fix-reintroduces-its-own-defect-class`) and found the same class twice more:

- **The three adapters carried leg (c)'s exact defect, untouched.** `scripts/runners/{codex,cursor,opencode}.sh`
  each parse `--agent`/`--model`/`--effort` with the same unguarded `shift 2`, in a loop with no
  trailing shift and no `set -e`. Measured, not inferred: each hung. These are **documented
  direct-hand-invocation** entry points — `codex.sh`'s own comment calls its brief-file gate "the
  DEFENSIVE TWIN for the direct hand invocation this contract documents, which bypasses the facade"
  — so fixing only the facade closed nothing for a hand caller. Fixed in `77a2f7e6`, with the bound
  extracted into `tests/lib/bounded_arg_probe.sh` so the facade and the adapters share one harness.
- **A second population of parent-facing dispatch instructions was missed entirely.** The plan's
  Task 5 hand-counted "the three skill sites"; `cursor-rules/dispatch/` is a fourth surface, and its
  five newly-feature-scoped fragments told a Cursor parent to dispatch without naming a worktree —
  so every runner-delegated review / resolver / repair dispatch would have aborted deterministically
  on the shim's own rule. This is AGENTS.md's "never hand-list the sites, derive them from a
  whole-repo grep" firing exactly as written. Fixed in `23e2046b`, and the fix left behind a guard
  keyed on the **declaration** rather than on a name list, mutation-proven in both directions
  (flipping `docket-status.md` to `feature` reddens the positive arm; deleting a source's
  declaration reddens the population floor, so a lost declaration cannot silently retire its assert).

### Three asserts that would have shipped vacuous

Caught during the build, not by the suite — every one was green for the wrong reason:

| Assert | Why it was vacuous | Repair |
|---|---|---|
| `0208(b)`: a feature-scoped agent without `--worktree` is rejected | With no `--worktree` the anchor defaults to the main worktree, so **gate 3b** fires on the same input and its diagnostic also contains the literal `--worktree`. Both `rc != 0` and the flag grep stayed green with **gate 1 deleted**. | Re-pinned on gate 1's own mechanism (`--worktree is required for feature-scoped agents` **and** `worktree-scope: feature` in the message) |
| Tolerant-fallback leg for an unknown agent | Asserted `rc = 0`, which is false — `codex.sh` dies with `no built-in agent source for '<name>'`. | Re-pinned on the **emitter**: the adapter's diagnostic is present and carries no `runner-dispatch:` prefix, which is what the tolerance decision actually claims |
| `ANCHOR_FALLBACK` exemption | Dropping the conjunct reddened **nothing** across all four facade test files: the removed-worktree observe block dispatches `--agent status`, which is metadata-scoped, so the exemption never engaged. | Two extra facade calls riding the block's **existing** dispatch key, observing with `--agent review-lean`, plus a declaration-sanity assert so the leg cannot go vacuous if `worktree-scope:` ever leaves that source |

The gate-1 case is the sharpest: two independent gates producing indistinguishable observable
behavior for the same input is a permanent vacuity trap, and only a mutation probe surfaces it.

### The residual the plan accepted, closed instead

The plan predicted that mutation 2 on gate 3 — replacing the first-line same-repo test with an
anywhere-in-list match — would stay green, and told the worker to record it as an accepted residual:
reproducing a foreign repo whose worktree list carries a stale record for our root looked like more
fixture than the risk warranted. It was ~12 deterministic lines. The worker built it, confirmed by
hand that the hazard is **real rather than theoretical** (under the mutation the facade *accepted* a
foreign tree and ran the adapter with `DOCKET_REPO_ROOT=<foreign tree>`, rc=0), and fixtured it.
AGENTS.md's "a mutation that leaves an assert green is a defect until proven otherwise" outranks a
plan's cost estimate — the plan was wrong about the price, and the right response was to re-price it,
not to honor the estimate.

### Two fixtures the membership gate silently invalidated

Tightening gate 3 broke fixtures that had been passing on the *old* gate's laxity, and one broke
invisibly:

- The 0277 payload-gate block used a bare `mkdir -p` anchor; its two "WITH a payload runs" legs went
  **red** — visible, fixed.
- The 0237 run-gate case (a) asserted `[ ! -s "$SBX/vr.log" ]` and went **silently vacuous**: the
  dispatch now died at gate 3 and never reached the adapter, so the log was empty for entirely the
  wrong reason — precisely what that assert's own comment warns about. It gained a "the delegation
  reached the adapter" non-vacuity floor.

Generalizable: tightening an input gate re-prices every fixture that fed it, and the dangerous half
of that re-pricing is the asserts that keep passing.

### Judgment calls made against the review's suggestion

- **`DOCKET_AGENTS_SRC` refuses on shape, not on `[ -d ]`.** The review suggested `[ -d "$AGENTS_SRC" ] || die`.
  A `[ -d ]` test passes a *misdirected* path — the finding's own second condition — while every
  scope read inside it still returns empty, which is the guard-rule failure in miniature. The gate
  keys on whether the directory actually holds `docket-*.md` sources. The mutation ladder proves the
  discrimination: weakening it back to `[ -d ]` reddens exactly the three *misdirected* legs while
  the *absent* legs stay green.
- **The die is unconditional, for every agent, and the rename rides with it.** With no sources the
  facade cannot tell a feature-scoped agent from a metadata-scoped one, so "refuse only the dangerous
  half" is not computable. A metadata dispatch that never needed the read pays a loud, one-line
  install failure; the alternative it buys off is a silent delegation of the main tree. The seam was
  renamed `AGENTS_SRC` → `DOCKET_AGENTS_SRC` **as a complement, not a separate tidying**: the die
  converts a silent disarm into a hard refusal, which means an unrelated tool's ambient `AGENTS_SRC`
  would newly break *every* docket dispatch. Namespacing removes that spurious-outage vector.
- **The shared scope reader does not delegate to `docket-frontmatter.sh`.** That is the repo's
  canonical anchored frontmatter reader and was the obvious target. It was built that way first and
  broke a real guard: `sync-agents.sh` must run under macOS system Bash 3.2, where
  `docket-frontmatter.sh`'s `declare -gA` errors at **source** time and `set -e` aborts the
  generator. `scripts/lib/docket-agent-scope.sh` therefore carries its own anchored `awk`, and its
  header records the constraint by name so the next person does not re-attempt the delegation.
  Captured as change **#287**.
- **`validate_agent_scopes` bonds `build-*` to `feature`.** This couples a validator to a name shape
  inside a change whose thesis is that name shapes age badly — defensible **only** because the
  facade still keys the empty-payload gate on that same shape, so the bond is a consistency check
  between two existing readings rather than a new enumerated floor. The code comment says exactly
  that, and says the bond must be deleted only together with the reading it bonds to.

### Gate 3b's diagnostic over-claimed, in the same shape this change indicts

As first built, gate 3b tested `ANCHOR != REPO_ROOT` and reported "on the integration branch" — a
branch fact it never reads. Gate 3's own new comment condemns the pre-0208 code for exactly this
("the diagnostic asserted a membership nothing had checked"). The predicate is defensible; the
message was not. Reworded to state what was measured, with the residual recorded in the contract.

## Follow-ups

- **#287** (refactor) — make `docket-frontmatter.sh` usable from the bootstrap path, or split a Bash
  3.2-safe core out of it. Today docket's canonical anchored frontmatter reader cannot be used by
  any script that must run under system Bash, so each one hand-rolls a parser — the duplicated
  extraction the canonical reader exists to prevent.
- **#288** (chore) — namespace the remaining un-namespaced mock seams. `RUNNERS_DIR` and `GIT` sit on
  the same `# Mock seams:` line as the newly-namespaced `DOCKET_AGENTS_SRC`; a bare `GIT` exported by
  any surrounding tool is silently honored by a docket dispatch, which is why the rule exists.
- **#0280** (pre-existing) — `tests/test_sync_agents_runners.sh` at ~188s against 60s. Untouched by
  this change and re-measured here as unchanged within noise.

## Plan deviations

- **Task 6 produced no commit.** `tests/test_runner_dispatch.sh` measured 16–17s against its 20s row,
  so no budget re-measurement was needed. The budget work that *was* needed turned out to be a
  different table entirely — `tests/test_skill_size_budgets.sh`, where `docket-finalize-change/SKILL.md`
  had been pushed to two words of headroom. That file's own governing comment names near-zero
  headroom as the failure mode it exists to forbid, and prescribes the rounding rule that was then
  applied (3848 → next multiple of 50 is 3850, inside the 25-word floor, so the multiple after: 3900).
- **The facade's scope probe became a shared library.** The plan explicitly said the facade should
  implement its own probe, "deliberately not a shared helper". Review reversed that: the value
  semantics could not drift, but the *extraction* could, and only `sync-agents.sh` fails loudly — so
  a key-spelling change made there alone would leave the facade ungated and silent. The plan is a
  point-in-time record and was left unedited.
- **`README.md` was in scope and the plan's file list omitted it.** A whole-repo grep during Task 5
  surfaced a fourth stale `build-*` statement there; corrected in the same commit.

## Test evidence

Full suite green at branch HEAD, via `scripts/run-tests.sh` (the resolved `finalize.test_command`):
104 files, 104 passed, 0 failed, 8351 asserts, exit 0. Run twice at the build gate — once at
`506d427d` (8217 asserts) before the fix loop, once at `2c75e0dd` after it.

Per-file timings for the files this change touches, against their `tests/runtime-budgets.tsv` rows:
`test_runner_dispatch` 16s/20s, `test_runner_dispatch_observe` 22s/25s, `test_runner_dispatch_build_gate`
8s/10s, `test_runner_dispatch_detach` 8s/15s, `test_sync_agents_validator` 14s/15s (thin headroom —
the cheapest cut, if it starts breaching, is moving the per-agent scope table into
`tests/test_cursor_dispatch_rule.sh`, which already derives the same population and runs in 0s).
