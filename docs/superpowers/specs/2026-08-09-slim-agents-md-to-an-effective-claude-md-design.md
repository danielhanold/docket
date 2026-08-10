<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0283 — Slim AGENTS.md to an effective, lean always-in-context file](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0283-slim-agents-md-to-an-effective-claude-md.md)**
<!-- docket:backlink:end -->

# Slim AGENTS.md to an effective, lean always-in-context file — design

- **Change:** 0283
- **Date:** 2026-08-09
- **Status:** Approved by Daniel (aggressiveness, dispatch table, run gate settled interactively)

## Goal

Reduce AGENTS.md — the always-in-context rules file — to the minimum that must fire unprompted,
per Claude Code's "write an effective CLAUDE.md" best-practices guidance, **without losing any
rule that has no other enforcement surface**.

## Settled decisions

1. **Prune only what is enforced.** A rule leaves AGENTS.md only when a *landed* mechanical
   enforcement surface (test, guard, or script behavior) covers it repo-wide today. Rules whose
   guards are designed but unbuilt (the 0263 set: producer-pipe/SIGPIPE, leading-`--` grep,
   awk indent-class — 0263 is spec'd but waiting on 0172) **stay** until those guards land; a
   follow-up removal rides with or after 0263.
2. **Dispatch table stays verbatim.** The "Docket agents — dispatch, don't run inline" section,
   including all per-agent rows, is untouched.
3. **Run gate stays.** The "Run gate — verify a dispatched implement-next run" section is kept
   (light wording tightening allowed; no step removed).

## What the build does

1. **Audit pass.** For every remaining rule bullet in AGENTS.md (`## Shell`,
   `## Frontmatter and generated blocks`, `## Guards and tests`,
   `## Comments and cross-references`, and the header prose), record: the enforcing
   test/guard/script if one exists repo-wide today, or "unenforced". Evidence, not memory —
   verify by running/reading the named guard. Known starting point (verified by 0263 on
   2026-08-08): `tests/test_grep_portability.sh` enforces the ERE-declaration rule on every
   tracked path except `docs/`; the other Shell rules are unenforced.
2. **Removal pass.** Remove exactly the rules with landed repo-wide enforcement. Each removal in
   the PR names its enforcing surface. Before removing, mutation-check the claimed guard once
   (introduce the violation in a scratch file it covers; watch it redden) — a removal backed by a
   guard that doesn't actually fire is a defect.
3. **Demotion pass.** A removed rule whose war-story content is not already in
   `docs/changes/learnings/` gets its finding extended or created there (normal harvest format);
   a rule already backed by a finding or an ADR is just deleted from AGENTS.md.
4. **Tightening pass.** Compress the header prose and any kept-rule wording where meaning is
   unchanged; the dispatch table is exempt; the run gate keeps all steps.

## Out of scope

- Building any new guard or test (that is 0263/0172's work).
- Removing rules whose enforcement has not landed, however imminent.
- Any change to promotion mechanics, the learnings ledger, or skill bodies.
- The generated portions of harness instruction files (sync-agents-managed blocks).

## Success criteria

- Every removed rule cites a landed, mutation-verified enforcement surface in the PR.
- AGENTS.md still contains every unenforced must-fire rule, the full dispatch table, and the
  full run gate.
- No net-new obligations added; file is strictly shorter.

## Open questions resolved

- Which Shell rules are guard-enforced → only the grep-ERE rule (per 0263's verification);
  re-verify at build time since 0172/0263 may land first.
- Dispatch table / run gate disposition → keep verbatim / keep (decisions 2–3).
