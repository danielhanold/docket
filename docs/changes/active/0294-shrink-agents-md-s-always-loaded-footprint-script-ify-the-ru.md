---
id: 294
slug: shrink-agents-md-s-always-loaded-footprint-script-ify-the-ru
title: 'Shrink AGENTS.md''s always-loaded footprint: script-ify the run gate''s caller procedure and de-duplicate the dispatch table'
status: proposed
priority: medium
type: refactor
created: 2026-08-11
updated: 2026-08-11
depends_on: []
related: [283, 275, 242]
discovered_from: [275]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 0275 grew the always-loaded run gate from 25 to 48 lines (+92%) by porting attribution
logic that `runner-dispatch.sh` and `verify-run.sh` already implement into hand-executed prose:
the three-filter claim attribution, the output shape of `--with-claimed-at`, and where to store a
dispatch epoch are *procedure*, not *rules*, and every session in every harness now pays for them
in context. The guard file (`tests/test_sync_agents_run_gate.sh`) grows with the text and already
consumes 60–70% of its runtime budget (0275's results).

**Meta-finding worth carrying into the design:** 0275's spec was auto-groomed and pinned "no
script changes" *because* the oracle flags already existed — treating that as a virtue when it was
the strongest evidence the procedure belonged in a script too. No downstream role owns reopening a
settled design (reconcile checks reality-drift, review checks the diff, the critic attacks
coherence and risk — cheap-and-safe passes it), and the gate text's brevity assert is a tollbooth,
not a wall: the sanctioned response is "raise the bound with rationale," so the cost got priced
and paid rather than avoided. A spec that says "no script changes" because the scripts already
expose the data should trigger the opposite reflex.

Separately, the `## Docket agents — dispatch, don't run inline` table restates every agent's
`description:` verbatim — `sync-agents.sh` generates those bullets from the same frontmatter in
`agents/*.md` that harnesses like Claude Code already inject into context as their own
available-agent listing, so the text loads twice per session on those harnesses.

## What changes

- Move the run gate's caller-side procedure behind facade subcommands (sketch: a `gate-before`
  that re-syncs, snapshots the claimed set, captures the epoch, and prints one opaque line the
  session keeps in its notes; a `gate-verdict` that re-syncs, diffs, runs the three-filter
  attribution, calls verify-run, and prints a single report line — proceed / re-dispatch-once
  `<id>` / halted / ambiguous-stop / unattributed-verify-only).
- Shrink the AGENTS.md gate text to roughly 12–15 lines: when to run each command, obey the
  report line, and the never-rules (never re-dispatch a halt, never a third dispatch, never
  re-dispatch in unattributed mode). Rationale prose moves to the script's contract and comments.
- Re-lower the gate-text brevity bound accordingly; the guard suite shrinks with the text,
  relieving the budget headroom 0275 flagged.
- Reduce the dispatch table where the harness already surfaces agent descriptions natively —
  possibly to the rule plus agent names — verified per harness (claude, cursor, codex, opencode)
  before dropping anything.

## Out of scope

- Changing the run gate's semantics — the foreground path, the detached branches, and the
  never-rules keep their meaning; only where the procedure lives moves.
- Change 0283's pruning pass over the *other* AGENTS.md rules (it explicitly excludes the run
  gate; this change is its complement and must sequence against it).

## Open questions

- How does the before-state survive tool-call boundaries — one opaque printed line the session
  carries in notes, or per-dispatch state on disk (reuse `runner-dispatch.sh`'s `gate-before`
  machinery), and how does the on-disk form behave under concurrent loops sharing the metadata
  worktree?
- Which harnesses actually inject agent descriptions into context natively, and what does the
  dispatch block keep on the ones that don't?
- Sequencing with 0283: same pass, dependent changes, or strictly ordered edits to the generated
  file?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
