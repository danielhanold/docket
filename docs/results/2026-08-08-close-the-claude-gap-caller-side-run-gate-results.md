<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0242 — Close the Claude gap in the run-completion gate with a caller-side verify in the dispatch rules](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0242-close-the-claude-gap-in-the-run-completion-gate-with-a-comma.md)**
<!-- docket:backlink:end -->

# Close the Claude gap in the run-completion gate with a caller-side verify — results

Change: #0242 · Branch: feat/close-the-claude-gap-in-the-run-completion-gate-with-a-comma · PR: #186 · Plan: docs/superpowers/plans/2026-08-08-close-the-claude-gap-caller-side-run-gate.md · ADRs: ADR-0078

## What was built

- `cursor-rules/run-gate.md` is the single authored source of the gate text; `assemble_run_gate()`
  splices it into both `assemble_dispatch_rule()` and `assemble_agents_md_dispatch()`, so the two
  parent-facing surfaces render the gate byte-identically while their surrounding per-harness prose
  stays untouched (ebf8f9a6).
- `sync-agents.sh` resolves — or creates — the one physical file a Claude parent always loads:
  `resolve_physical_path()`, `repo_wants_claude_surface()`, `claude_surface_target()`, and
  `sync_dispatch_surfaces()`, which writes the managed block once per distinct physical file
  (48b987ba).
- The write pass and `--check` now iterate the same deduped target set against one shared `seen`,
  and the `sync_agents_md_dispatch` shim is retired (7aaf1012).
- This repo takes its own medicine: a committed `CLAUDE.md` symlink to `AGENTS.md`, plus a sentence
  in the convention's *Composition* paragraph pointing the abstract verification obligation at the
  mechanical gate (fe77d45f).
- Reachability and convention-pointer guards (5a1240a7).

Full suite green at the Task 3 and Task 4 gates: **95 files, 7120 asserts, exit 0**.

## The `OVER BUDGET` trailer, measured

The build's last full-suite run trailed `OVER BUDGET: test_board_checks test_harness_defaults
test_harness_defaults_validator test_sync_agents test_sync_agents_codex test_sync_agents_runners`,
and an earlier draft of this file dismissed it as contention noise and did not act on it. That was
wrong on the repo's own terms — `AGENTS.md` says a trailing `OVER BUDGET:` line is a finding to act
on precisely because nothing else will catch it — and the list had grown from three files to six
during the build. It was measured instead.

**Method.** `scripts/run-tests.sh -j 1 --no-budget-check --timings`, against this branch and against
the merge-base (`487bfdc5`, checked out in a detached worktree), the two **interleaved** so a
drifting machine could not masquerade as a diff. Four paired passes: base-first twice, then
branch-first twice, because the first ordering alone puts the branch consistently later in time.
Seconds, serial:

| file | sync invocations | base (487bfdc5) | this branch |
|---|---|---|---|
| `test_sync_agents.sh` | 30 | 60, 59 | 56, 55 |
| `test_sync_agents_defaults.sh` | 35 | 58, 54 | 56, 55 |
| `test_sync_agents_drift_docs.sh` | 41 | 61, 59 | 55, 60 |
| `test_sync_agents_codex.sh` | 14 | 56, 67, 53, 53 | 63, 75, 57, 56 |
| `test_sync_agents_runners.sh` | 39 | 66, 76, 61, 62 | 73, 81, 65, 63 |

**What that says.**

- *The premise that every sync-agents test got heavier is not supported.* The change does make every
  `sync-agents.sh` run resolve and write a Claude surface, so the expectation was growth
  proportional to invocation count. The three files with the **most** invocations (30/35/41) are the
  three that did not grow at all — `test_sync_agents.sh` is faster on the branch in every paired
  pass. Whatever the surface write costs, it is below this measurement's resolution.
- *`test_sync_agents_codex.sh` did grow, and it is the file this change reshaped.* Paired deltas
  +7/+8/+4/+3s, the same sign in both orderings. It is the added third `sync-agents.sh --check`
  invocation (the de-list block's direction-1 leg), not a per-invocation tax — 14 invocations is the
  family's smallest count.
- *Every one of these files breaches on the merge-base too.* Absolute levels moved 20–25% between
  the loaded and quiet passes (`test_sync_agents_codex.sh` base measured 67s and 53s on the same
  commit hours apart). The rows are not lying about the *branch*; the machine is slower than the one
  the rows were seeded on. Re-seeding them upward would be the evasion `tests/runtime-budgets.tsv`
  and `scripts/run-tests.md` both name: a ceiling moves when a file is **re-shaped**, not when the
  machine gets slower.

**Acted on: `test_sync_agents_codex.sh` is sharded.** At 56–57s serial it sat past its own 55s row
with the hard 60s ceiling inside the noise band, so there was no honest row to write for it. It is
cut at its own `# --- AGENTS.md dispatch block` banner into `test_sync_agents_codex.sh` (the
per-repo `.codex/agents/*.toml` wrappers) and the new `tests/test_sync_agents_codex_dispatch.sh`
(the committed `AGENTS.md` dispatch block). The 74 asserts split 44/30 with none lost, both halves
green. Re-measured standalone over three consecutive serial runs — 21/19/22s and 41/38/41s — and
rowed at 30 and 50 by the table's own sizing rule; `EXPECTED_TOTAL` 1510 → 1535.
`tests/README.md`'s claim that this file had "no internal section banners, so there is no mechanical
boundary" was already inaccurate and is corrected there.

**Recorded, not acted on: five pre-existing breaches.** `test_sync_agents.sh`,
`test_sync_agents_defaults.sh`, and `test_sync_agents_drift_docs.sh` are untouched by this change
and measure **at or below** the merge-base. `test_sync_agents_runners.sh` is untouched and does show
a small consistent +1 to +7s, but it breaches on the merge-base as well (61–76s against a 60s row),
so its breach is not this branch's to absorb — sharding an 845-line file on that evidence would be
scope this change cannot justify. The three non-family files in the trailer were measured once
serially and are likewise pre-existing: `test_board_checks.sh` 61s (row 55),
`test_harness_defaults.sh` 50s (row 45), `test_harness_defaults_validator.sh` 53s (row 50) — the
same ~10% overshoot the family shows on its base side, on files this branch never touched. Whether
the whole table wants re-seeding on a quiet machine is a question for its own change, filed as a
follow-up below rather than settled here.

## Human verification

- [ ] **Does a live Claude parent session actually run the gate?** No in-repo test can answer this:
      the gate is prose a model follows, and the only oracle is a real session. After merging, run
      `/docket-implement-next` in an interactive Claude session in this repo and confirm the
      transcript shows `docket.sh verify-run --in-progress-ids` **before** the dispatch and
      `docket.sh verify-run <id>` **after** the fork returns. A missing command is the degradation
      signal the spec's §3 predicted; if it recurs, the recorded escalation path is the
      `Stop`/`SubagentStop` hook preserved in the spec's *Rejected* section.

## Findings

**The plan's code was unverified code, and five of six tasks had to correct it.** Each item below
was found by building the thing, not by reading the plan.

- **Task 1 — the slice helper would have run to EOF.** The plan's `slice_gate` terminated on a
  marker that exists only in `AGENTS.md`, so the *cursor* slice had no terminator at all. Replaced
  with a bound derived from the template's own line count, which incidentally upgraded the assert
  from "the two renderings share a prefix" to "the template renders verbatim into both".

- **Task 2 — the dedupe was defeated by macOS's own `/tmp`.** The plan's `resolve_physical_path`
  absolute-symlink branch trusted the link target's *spelling* rather than canonicalising each hop.
  On macOS (`/tmp -> /private/tmp`) one physical file then answers to two non-equal names and the
  dedupe silently fails — a second managed block, forever. Every hop is now re-canonicalised, and
  reverting just that hop reddens a named assert.

- **Task 2 — the symlink predicate had to become future-tense.** `claude_surface_target()` asks
  whether the repo *will* have an `AGENTS.md`, not whether it has one now: on a virgin
  `[claude, codex]` repo `AGENTS.md` is created by the very write pass that resolves this target, so
  the present-tense question seeds a second real `CLAUDE.md` carrying a permanently duplicated
  block.

- **Task 2 — an honest negative: the write-pass dedupe is not independently observable.**
  `ensure_managed_block` is idempotent, so writing the same block twice to one file is
  indistinguishable from writing it once. The `seen` set earns its keep through the **STRIP** pass,
  which is where the test bites. Recording this because it is exactly the shape that reads as a
  guarded property and is not one: an assert aimed at "writes once" through the write pass alone
  would have been green with the dedupe deleted.

- **Task 3 — the `--check` leg needed the stronger predicate.** Gated on
  `project_wrappers_generated`, the predicate `project_level_pass` actually writes under. Under the
  weaker `gitignore_block_wanted` — and with `HARNESSES` defaulting to a set containing `claude` —
  the check demanded a `CLAUDE.md` from every repo that merely has a docket branch.

- **Task 5 — the plan's convention-pointer assert was vacuous.** It flattened the whole SKILL.md and
  matched `verify-run` and `once` from unrelated paragraphs, staying green with the guarded sentence
  deleted (mutation-proven). Fixed by binding the window to the *Composition* paragraph and adding
  an anchor-existence assert so a renamed paragraph fails loudly instead of silently matching
  nothing.

- **Task 4 — docket's own opt-in is machine-local, which explains the gap.** This repo's dispatch
  opt-in lives in `.docket.local.yml`, which is gitignored. CI never sees it, so this repo's own
  Claude dispatch surface could never have been generated anywhere but a maintainer's machine —
  which is why the file being absent looked like a design choice rather than an omission.

**Accepted risk (spec *Risks*).** This repo now carries a committed `CLAUDE.md` **symlink** (mode
`120000`). A Windows checkout without symlink support materializes that as a plain text file
containing the string `AGENTS.md`, silently breaking the surface for such a clone. Accepted: this is
a solo-maintainer macOS project.

## Follow-ups

- **Re-seed `tests/runtime-budgets.tsv` from a quiet machine.** Eight files now measure above their
  rows on the merge-base as well as on this branch, with 20–25% spread between loaded and quiet
  passes on the *same commit*. Either the rows were seeded on a faster machine-state than today's or
  the table has drifted; a per-file wall-clock table that breaches on an untouched base is a table
  nobody will read. This is the contention-independent basis change **0229** already owns — worth
  attaching the numbers above to it rather than opening a second change.
