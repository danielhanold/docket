# Split docket-convention via progressive disclosure — results
Change: #20 · Branch: feat/convention-progressive-disclosure · Plan: docs/superpowers/plans/2026-06-17-convention-progressive-disclosure.md · ADRs: none

## Findings

- **Faithfulness verified two ways.** The structural test asserts the moved mechanics are gone from core `SKILL.md` and present in the sibling; an independent review byte-diffed the sibling body against the base section (exact match — intro + all five subsections) and mutation-tested all eight new `(f)` assertions, confirming each flips to `NOT OK` when its guarded clause is removed. No vacuous assertions; no `producer | grep` pipes.
- **Extraction criterion established (spec §2).** A convention section may move to an on-demand sibling only when it is **both heavy and off the common read-path** — opt-in, or its work is script-delegated. The GitHub board mirror is the first instance (opt-in via `board_surfaces: github`, and already delegated to `scripts/github-mirror.sh`). No new ADR: this is a recursive application of ADR-0003 (convention reference-loading), one level deeper.
- **Plan deviation (intentional).** Built **inline** rather than via `subagent-driven-development` (the plan's recommended sub-skill), per LEARNINGS #1 — a tightly-coupled single-artifact edit where fanning out risks inconsistent edits to shared content. TDD red→green was preserved: the `(f)` test block was written first, run to confirm red (7 of 8 new assertions failing), then the extraction made the full suite green.

## Follow-ups

- **Future candidate (separate change):** extract *only* the Agent layer's **generator** sub-part (`sync-agents.sh`, the three-layer precedence table, the `--check` CI gate) — install-time, so it passes the criterion. The Agent layer's abort-and-report rule and composition-dispatch contract must **stay** in core (runtime, non-delegated, common autonomous path). Left whole here per spec Out of scope.
- If more siblings accrue, consider a `references/` subdirectory convention — a single flat sibling didn't warrant it (matches docket's existing flat-sibling templates).
