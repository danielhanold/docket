# Split the docket-convention skill via progressive disclosure — extract the GitHub board mirror first

**Date:** 2026-06-17
**Change:** 0020
**Status:** Design (brainstorm complete, awaiting build)
**Related:** change 0005 (convention extraction), ADR-0003 (convention reference-loading)

## 1. Context

Change 0005 extracted the shared convention out of the five (now seven) operating skills
into a single pure-reference skill, `docket-convention`, loaded as a blocking Step 0 by every
operating skill (ADR-0003). That skill has since grown — change 0011 added the GitHub board
mirror, 0016/0017 added the Agent layer — to **224 lines / ~26.6 KB**, the largest skill in the
repo. Because all seven operating skills load it up front, every one of them pays that ~26 KB of
context on every run.

The driver for this change is twofold: **(1) context/token cost** — shrink the default footprint
the operating skills load on the common path, and **(4) skill-authoring best practice** — bring the
convention into the progressive-disclosure shape good skills follow (a concise `SKILL.md` plus
on-demand sibling references). This is not cosmetic reorganization: the split is meant to be
load-bearing for token cost.

The constraint that shapes everything: ADR-0003 guarantees operating skills use convention
vocabulary *without redefinition* precisely because the Step-0 load puts that vocabulary in
context. So whatever stays in the core must remain sufficient for the **common path**; only detail
that is genuinely off that path may move to a sibling a skill reads on demand.

## 2. The extraction criterion

A convention section is a clean progressive-disclosure candidate only when it is **both**:

- **(a) heavy** — meaningful token weight, and
- **(b) off the common read-path** — most runs/skills do not need its text in context, because it
  is *either* conditional/opt-in *or* the actual work is delegated elsewhere (so a skill only needs
  the prose on a narrow path).

This criterion is the durable rule future extractions follow; it is the reason this change moves the
mirror and deliberately leaves other heavy sections in place.

### Why the GitHub board mirror qualifies

It is off the common path on two independent counts:

1. **Opt-in.** It only matters when `board_surfaces` includes `github`. This repo runs `[inline]`,
   so the section is pure context overhead today, and `github` is strictly opt-in for any repo.
2. **Script-delegated.** The convention itself states the mirror's mechanics are *"owned by the
   deterministic `scripts/github-mirror.sh` … the Board pass only invokes it."* The heavy prose —
   issue-body shape, the `docket:` label namespace, the status→issue mapping, Projects v2 minting —
   is *explanatory reference*, not runtime instructions a skill executes. A script does the work; the
   skill calls it. Nobody needs those lines in context to run a change.

### Why the Agent layer does NOT qualify (yet)

The Agent layer is heavy (a) but fails (b). Its load-bearing parts are neither conditional nor
script-delegated: the **abort-and-report rule** is a runtime behavioral contract every autonomous
skill must honor, and the **composition contract** ("implement-next dispatches the status subagent
at step 0 and the adr subagent at step 6; auto-groom dispatches the critic") is the single source
for nested dispatch, deliberately not restated in the dispatch prose. Pulling those out of context
could strip an autonomous skill of the rule it must reason from. Only the *generator* sub-part
(`sync-agents.sh`, the three-layer precedence table, the `--check` CI gate) is install-time and
movable — but splitting one section into "movable half + stays half" is out of scope here. The
Agent layer is left whole.

## 3. Decision

Extract **only** the `### GitHub board mirror` section in this change. This establishes the
progressive-disclosure pattern with the one section that provably satisfies the criterion, lands
real common-path savings (~20 lines), and carries the lowest risk to the vocabulary guarantee.
Whether to extract more later (e.g. the Agent layer's generator sub-part) is a separate future
change, decided once this pattern proves out.

## 4. Design

### 4.1 Core stub (stays in `skills/docket-convention/SKILL.md`)

Replace the full `### GitHub board mirror` section with a **2-line stub under the same heading**, so
existing cross-references by name still resolve — `### Configuration`'s *"see GitHub board mirror"*,
the manifest's `issue:` field note, and docket-status all refer to it by that name. The stub keeps
the load-bearing vocabulary anchors and adds the on-demand pointer:

> ### GitHub board mirror (shared definition)
>
> The `github` board surface mirrors each change to one GitHub issue (+ Projects v2 item) —
> **strictly one-way**: change files are the source of truth, the mirror is derived output that is
> **never read back**. It rides in the Board pass and is **best-effort**; its external-write
> mechanics are owned by the deterministic `scripts/github-mirror.sh` (the Board pass only invokes
> it). **Full mechanics — `issue:` upsert, the `docket:` label namespace, the status→issue mapping,
> issue body, and Projects v2 — in [`github-board-mirror.md`](github-board-mirror.md); read it when
> `board_surfaces` includes `github`.**

(Exact wording finalized at build time; the anchors that must survive are: one-way,
change-files-authoritative, never-read-back, rides-in-Board-pass, script-owned, plus the heading and
the sibling pointer.)

### 4.2 The sibling — `skills/docket-convention/github-board-mirror.md`

A **flat sibling** in the skill directory, matching docket's existing convention (`change-template.md`,
`adr-template.md`, `results-template.md` are all flat siblings). Plain markdown, no frontmatter. It
opens with a one-line orientation note —

> On-demand detail for the convention's GitHub board mirror. Read this when `board_surfaces`
> includes `github`; the core contract lives in `SKILL.md`'s *GitHub board mirror* stub.

— followed by the extracted section content **verbatim** (the `issue:` field, status→issue mapping
for all seven states, the `docket:` label namespace rules, issue body shape, and Projects v2
minting). No `references/` subdirectory for a single file; introducing one is a future call if more
extractions follow.

### 4.3 Ripple — exactly one skill (docket-status)

docket-status already single-sources the `github` surface in its Board pass (`SKILL.md` line ~47,
*"the one-way Issues + Projects v2 mirror (per the convention's GitHub board mirror definition)"*).
Retarget that one reference to also point at the sibling — e.g. *"… (per the convention's GitHub
board mirror definition; read `skills/docket-convention/github-board-mirror.md` for the mechanics)."*
Every other skill runs *docket-status's Board pass* by reference, so no other skill changes.

### 4.4 Tests — extend `tests/test_convention_extraction.sh`

The existing test's header list (line ~20) does not include the mirror header and its anti-copy
sentinels include no mirror-specific phrase, so the extraction does not break current assertions —
but it also leaves the new structure unguarded. Add a small progressive-disclosure block asserting:

- (a) `skills/docket-convention/github-board-mirror.md` exists and contains a mirror-distinctive
  phrase (e.g. `closed as **not planned**`);
- (b) that same phrase is **absent** from `SKILL.md` (moved, not copied);
- (c) `SKILL.md` retains the `### GitHub board mirror` stub heading **and** a `github-board-mirror.md`
  pointer;
- (d) `skills/docket-status/SKILL.md` carries a `github-board-mirror.md` pointer.

Keep the guard in `test_convention_extraction.sh` so all convention-structure invariants live in one
file. `test_github_mirror.sh` (which exercises `scripts/github-mirror.sh`) is unaffected — the script
and its behavior do not change.

## 5. ADR

No new ADR. This is a recursive application of **ADR-0003** (the convention is reference-loaded) one
level deeper — within the convention skill itself — not a new architectural decision. The extraction
criterion in §2 is recorded here in the spec as the rule future extractions follow; if a later change
extends progressive disclosure to several sections, that broader move may warrant its own ADR or a
dated `## Update` note on ADR-0003. Not this change.

## 6. Out of scope

- Extracting any other section (Agent layer, Configuration, Bootstrap guard, Branch model, …).
- Introducing a `references/` subdirectory.
- Any change to *what the mirror does* — this moves text, it does not revise the contract.
- Editing historical records (archived changes, prior specs) that quote the old inline mirror section.

## 7. Open questions

None. Scope (mirror only), discovery mechanism (stub + explicit docket-status pointer), file location
(flat sibling), and the no-ADR call were all settled in the 2026-06-17 brainstorm.
