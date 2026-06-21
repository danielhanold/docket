---
id: 37
slug: skill-fallback-progressive-disclosure
title: Slim skills — move the per-skill manual-fallback / script-contract prose into on-demand sibling files
status: proposed
priority: medium
created: 2026-06-21
updated: 2026-06-21
depends_on: [34]
related: [34]
adrs: []
spec: docs/superpowers/specs/2026-06-21-skill-fallback-progressive-disclosure-design.md
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-06-21-skill-fallback-progressive-disclosure-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-06-21-skill-fallback-progressive-disclosure-design.md) |
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
