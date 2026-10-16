---
id: 3
slug: convention-reference-loading
title: The docket convention is reference-loaded from a docket-convention skill, not embedded per skill
status: Accepted
date: 2026-06-10
supersedes: []
reverses: []
relates_to: [2]
change: 5
---

## Context

From the first skill set through change 0004, every docket skill embedded the full shared convention — 146 lines, 52–72% of each skill's body — between `<!-- docket:convention:begin/end -->` markers, kept byte-identical by `sync-convention.sh` and guarded by a dedicated test plus sync-check asserts in three others. The duplication was deliberate: each skill was self-contained and never depended on a centralized template being installed. The cost was permanent: every convention edit was a five-file change mediated by the sync script, and drift between copies was a standing failure mode that needed active machinery to suppress.

The alternative — a sixth, pure-reference skill that the operating skills load at startup — stood or fell on one question: how reliably does a skill mid-procedure actually perform a "load the convention first" instruction? Two findings settled it (change 0005's spec §2 holds the full analysis): (a) mid-flow skill invocation is a proven mechanism here, not a novel bet — docket skills already chain `superpowers:brainstorming`, `docket-status`, and `writing-plans` as checklist steps; (b) the realistic failure mode is not *can't* but *thinks it doesn't need to* — a model believing it already knows the convention and skipping a soft "review the convention" suggestion. ADR-0002 had already set the relevant precedent in miniature: the terminal-publish *procedure* is single-sourced in `docket-finalize-change` and referenced — never restated — by its other callers.

## Decision

The convention lives **only** in `skills/docket-convention/SKILL.md` — a pure-reference skill with no procedure, reads, writes, or git. Each operating skill's embedded block is replaced by a blocking **Step 0**: invoke `docket-convention` before anything else (unless already loaded this session).

The rule that makes the reference reliable is the **undefined-terms forcing function**: operating skills keep *using* convention vocabulary — build-ready, metadata working tree, terminal-publish, the `DOCKET`/`LIVE` probes — **without redefining any of it**. A skill body that is not executable without the reference makes skipping the load self-defeating. The corollary is the **reference-never-restate** rule: convention content reappearing in an operating skill is a defect, even when it looks like helpful context. `tests/test_convention_extraction.sh` enforces both directions mechanically — ten collision-verified sentinel strings (plus the old sync markers) must each appear in the reference and in no operating skill, and every operating skill must carry the Step-0 load line. `sync-convention.sh`, its test, and the sync-check asserts elsewhere are retired; the convention-content asserts of older change-guard tests repoint to the single source.

## Consequences

- **Enables:** convention edits are one-file changes; cross-skill drift is structurally impossible rather than machinery-suppressed; ~725 lines of duplication removed; the convention is independently invocable by a human asking how docket tracks work.
- **Costs:** the skills are no longer self-contained reading — a skill file alone is deliberately not executable, and correct operation depends on the `docket-convention` skill being installed alongside the five (accepted: they ship as a set) and on the Step-0 load actually happening (mitigated as above; context-compaction exposure is unchanged from the embedded design, which carried the same risk).
- **Accepted gap (found in the change-0005 whole-branch review):** the sentinel tripwire is sampling, not parsing — *operational* restatements that dodge all ten sentinels remain possible (one such pre-existing restatement, the selection definition in `docket-implement-next` Step 1, was removed during the build). The discipline is the reference-never-restate rule; the test is its tripwire, and sentinels should be extended when new convention sections grow restatement-prone.
- **Given up:** the option of per-skill convention divergence (intentionally — it was never wanted), and the sync tooling's ability to catch a *mangled* convention copy (no copies exist to mangle).
