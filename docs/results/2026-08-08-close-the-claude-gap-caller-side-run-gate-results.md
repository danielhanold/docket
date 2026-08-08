# Close the Claude gap in the run-completion gate with a caller-side verify — results

Change: #0242 · Branch: feat/close-the-claude-gap-in-the-run-completion-gate-with-a-comma · PR: (opened at close of this run) · Plan: docs/superpowers/plans/2026-08-08-close-the-claude-gap-caller-side-run-gate.md · ADRs: none at build time — Decision 3's parallel ADR is authored by the review-time `docket-adr` dispatch

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

Full suite green at the Task 3 and Task 4 gates: **95 files, 7120 asserts, exit 0**. The advisory
`OVER BUDGET` trailer named `test_sync_agents`, `test_sync_agents_codex`, and
`test_sync_agents_runners` — all `rc=0`, parallel-contention noise on this machine, consistent with
the slack-factor calibration note; not acted on.

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

None filed.
