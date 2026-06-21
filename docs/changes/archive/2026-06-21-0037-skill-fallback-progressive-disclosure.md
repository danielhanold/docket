---
id: 37
slug: skill-fallback-progressive-disclosure
title: Slim skills — move the per-skill manual-fallback / script-contract prose into on-demand sibling files
status: done
priority: medium
created: 2026-06-21
updated: 2026-06-21
depends_on: [34]
related: [34]
adrs: []
spec: docs/superpowers/specs/2026-06-21-skill-fallback-progressive-disclosure-design.md
plan: docs/superpowers/plans/2026-06-21-skill-fallback-progressive-disclosure.md
results: docs/results/2026-06-21-skill-fallback-progressive-disclosure-results.md
trivial: false
auto_groomable: false
branch: feat/skill-fallback-progressive-disclosure
pr: https://github.com/danielhanold/docket/pull/48
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-21-skill-fallback-progressive-disclosure-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-skill-fallback-progressive-disclosure-design.md) |
| Plan | [2026-06-21-skill-fallback-progressive-disclosure.md](https://github.com/danielhanold/docket/blob/feat/skill-fallback-progressive-disclosure/docs/superpowers/plans/2026-06-21-skill-fallback-progressive-disclosure.md) |
| Results | [2026-06-21-skill-fallback-progressive-disclosure-results.md](https://github.com/danielhanold/docket/blob/feat/skill-fallback-progressive-disclosure/docs/results/2026-06-21-skill-fallback-progressive-disclosure-results.md) |
| PR | [#48](https://github.com/danielhanold/docket/pull/48) |
<!-- docket:artifacts:end -->

## Why

Each docket skill body carries detailed prose describing what its helper-script operations
do — e.g. `docket-new-change` / `docket-status` spell out the `archive-change.sh` /
`terminal-publish.sh` steps; the convention asserts "the prose is the contract the script
implements verbatim." That prose serves two readers: (a) a human/agent who wants to
understand what a step does (reference), and (b) the historical "hand-work it if the script
is absent" fallback. It is valuable, but it lives **in the always-loaded `SKILL.md`** of
every skill, so it inflates every skill's context cost on every invocation.

Change #34 makes the scripts reliably reachable and switches the skills to **fail loud**
(`${DOCKET_SCRIPTS_DIR:?…}`) rather than silently hand-work — which removes reader (b)'s
*urgency* but not the prose. So the prose is now mostly reader (a): the script's
**operations/contract reference**. It should be **kept** (it is the human-readable spec of
each script) but **moved off the hot path**.

## What changes

Give every helper script an authoritative, readable prose contract **co-located with the
script** — `scripts/<name>.md` beside `scripts/<name>.sh` — and shed the script-*internals*
prose out of the always-loaded skill bodies onto it. Two wins in one: a durable per-script
spec (bash is the hard-to-read part) **and** slimmer skill bodies. Design detail is in the
linked spec; the shape:

- **Exhaustive, scaled to complexity.** Every script gets a contract — full mechanics for
  `terminal-publish.sh`, a tight few lines for a thin wrapper.
- **Co-located, reachable via #34's `DOCKET_SCRIPTS_DIR`** (`$DOCKET_SCRIPTS_DIR/<name>.md`) —
  so consuming repos reach the contract by the same mechanism #34 built for the scripts.
- **The naming convention is the pointer.** Stated once in `docket-convention`: every
  `scripts/<name>.sh` has a co-located `scripts/<name>.md` — no hand-written pointer at the
  ~65 call sites.
- **Body↔contract boundary:** bodies keep the *operational* facts (when to call, the args,
  exit-code handling, step ordering); the script's *internals* move to the contract. The
  convention is special-cased — it keeps the conceptual definitions it owns (knob/verdict
  meaning, the bootstrap 2×2 semantics) and points to `scripts/docket-config.md` for the
  script's mechanics.
- **Drift discipline:** a test-suite static audit asserts `scripts/*.sh` ↔ `scripts/*.md`
  match 1:1 (catches a script or contract added/removed without its pair); content fidelity is
  left to co-location + review. `docket-convention/github-board-mirror.md` stays put — it is
  skill-reference, not a single-script contract.

**Folded in (found at #34's merge gate):** harden `tests/test_consuming_repo_scripts.sh`
so it can't false-RED. Its fail-loud assertions (the `${DOCKET_SCRIPTS_DIR:?…}` checks)
run `bash -c '…'` sub-shells that **inherit an exported `DOCKET_SCRIPTS_DIR`** — and
#34's `install.sh` exports exactly that into the dev shell's profile. So in any shell
where docket is installed, those sub-shells see the var set, `:?` never fires, and the
three fail-loud assertions go NOT OK even though the code is correct (observed live
finalizing #34 — a clean-env `env -u DOCKET_SCRIPTS_DIR` re-run was all-green). Fix: have
the test's fail-loud sub-shells `env -u DOCKET_SCRIPTS_DIR bash -c '…'` so they exercise
the unset path regardless of the ambient environment. Small and self-contained; rides here
because this change already revisits the suite's sentinels.

## Out of scope

- The script-reachability fix itself (`DOCKET_SCRIPTS_DIR`, install-time injection,
  fail-loud) — that is **#34** (now done), which this builds on.
- Rewriting the scripts or their behaviour.
- Mechanical content-sync verification of prose against bash (flaky/gameable — the audit is
  existence-only; content fidelity rests on co-location + review).
- `docket-convention/github-board-mirror.md` — skill-reference, not a single-script contract.

## Reconcile log

### 2026-06-21 — reconciled against the as-landed tree (implementer, pre-plan)

Dependency #34 (PR #45) confirmed **done** (archived `2026-06-21-0034-…`); its as-landed form
verified and pinned for the build:

- **Pointer syntax** is `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh` (43 call
  sites across the eight skill bodies). Contracts are reached the same way:
  `"${DOCKET_SCRIPTS_DIR:?…}"/<name>.md`.
- **#34's bare-path audit** is `tests/test_consuming_repo_scripts.sh`; the **§6 coverage
  audit** mirrors `tests/test_change_links_coverage.sh` (ROOT resolution + `ok`/`no` + exit
  `$fail`). New audit name: `tests/test_script_contracts_coverage.sh`.
- ADR-0012 (script↔model boundary) and ADR-0014 (#34's consuming-repo resolution) both
  present; this change's body↔contract split is downstream of ADR-0012.

**Inventory pinned** (`ls scripts/*.sh`, top-level): **13 scripts**, each gets
`scripts/<name>.md`. Two were not named in the original draft —
`ensure-claude-settings.sh` + `ensure-docket-env.sh` (landed with #34) — **folded in**.

**Scope narrowed** (decisions pinned, see spec *Scope / inventory*):
- Repo-**root** scripts (`install.sh`, `link-skills.sh`, `sync-agents.sh`,
  `migrate-to-docket.sh`) are **out** — they live at the repo root, not `scripts/`, and are
  unreachable via `$DOCKET_SCRIPTS_DIR/<name>.md`; the original draft loosely listed three of
  them. Their convention prose is conceptual/operational and stays per §4.
- `scripts/lib/docket-frontmatter.sh` is **out** — sourced helper, not an entry point; the
  audit globs top-level `scripts/*.sh` only (non-recursive).

**Constraint carried into the plan:** stripping internals prose must not delete literal
substrings the wiring-sentinel tests grep for — audit `test_convention_extraction.sh`,
`test_composition_wiring.sh`, `test_change_links_coverage.sh`, `test_render_board.sh`, and the
per-script tests in lockstep, exactly as #34 did. No new constraints invalidate the design;
proceeding to plan.

### 2026-06-21 — mid-build reconciliation: #0036 merged into `main` (implementer, at PR open)

While this change was building, **PR #47 / change 0036** ("status sweep — delegate archiving
to `archive-change.sh`; remove double-archive") merged into `origin/main`, advancing the
feature base from `0e68c54` → `d9243e9` and editing `skills/docket-status/SKILL.md` — the same
file this change's Task 10 stripped. The pre-0036 strip would have **reverted** two 0036 fixes
(the #0035 re-render-before-publish ordering and the per-change *log-and-continue* sweep
failure posture; my strip had rewritten the latter to finalize's *abort-and-report*).
**Resolution:** rebased onto `origin/main` and resolved `docket-status` to **0036's version**
— this change's docket-status strip is **superseded** (0036 already slimmed that body by
delegating the sweep to the script). All other work (13 contracts, convention §4/§2, the
finalize/adr/new-change/implement-next strips, the audit, the folded fix) is independent of
0036 and rebased cleanly; full suite re-validated green (both ambient and `env -u`) post-rebase.
A follow-up (trim 0036's residual archive-internals prose against `scripts/archive-change.md`)
is recorded in the results file. No new ADR minted — the co-located-contracts decision is
captured in the published spec + the convention §2 rule (extends ADR-0012).
