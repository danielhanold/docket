<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0260 — Tier finalize's in-context dispatches and name the push-denial posture](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0260-tier-finalize-s-in-context-dispatches-and-name-the-push-deni.md)**
<!-- docket:backlink:end -->

# Tier finalize's in-context dispatches and name the push-denial posture — design

**Change:** 0260 · **Date:** 2026-08-07 · **Type:** fix (docs + test rewiring, no behavior change)

## Problem

Two residuals from killed changes #0139 and #0100, both landing in the same two files:

1. The change-0137 dispatch-capability taxonomy (`skills/docket-convention/SKILL.md`, tier table
   A/B/C) does not cover `docket-finalize-change`'s two dispatches — `docket-rebase-resolver` and
   `docket-integration-repair` — whose reports flow back **in-context** to gate the merge rather
   than landing as git state. The gap is machine-pinned as a known deferral: the `PENDING_TIER`
   block in `tests/test_dispatch_capability.sh:195-235` asserts exactly these two knowingly-untiered
   sites and states it "MUST SHRINK TO EMPTY when the follow-up change tiers them."
2. `gate-failure.md`'s *abort-and-report points (the full set)* enumerates a classifier denial only
   for **the merge itself**; a **policy denial** of the gate's post-rebase
   `git push --force-with-lease` (observed live, halting an autonomous finalize — #0100) is absent,
   though a lease rejected by a *concurrent push* is listed. (#0100 called this "step 5"; under the
   current flow numbering the mandatory post-rebase push is flow item 6, with item 5's `ci` leg
   also pushing — and `gate-failure.md` already uses "gate-step-5" to mean the **red-suite** step,
   so the new prose names the push by its own noun, never by a step number.)

## Design

### 1. Carve-out, not a fourth tier

The two finalize dispatches get an explicit **carve-out paragraph** in the convention's
*Dispatch-capability resolution* section, immediately after the tier table — not a Tier D row.

Content of the carve-out (substance, not literal wording):

- Names both agents and why they sit outside the table: their contract is an **in-context report
  gating the merge**, not git state on `metadata_branch`, so neither Tier A's "inline is a
  first-class equivalent" nor Tier C's "authorized-or-halt" applies.
- Posture when dispatch is genuinely unavailable (established per the resolution rule above the
  table, never from a tool name): **finalize's existing abort-and-report** — the gate stops, the PR
  stays open, the change stays `implemented`, per `docket-finalize-change`'s own failure reference.
- States the reason inline-substitution is forbidden: inline resolution/repair by the agent that
  will then merge its own work is the same self-approval shape Tier B rejects for the critic
  (#0139's own argument for halt).
- Harness-neutral prose throughout (no product-specific retry/tool syntax) — existing convention
  rule.

### 2. Site-level wiring in `gate-failure.md`

The dispatch-site marker (the tier/posture named next to each site's own noun, plus the citation of
the convention's *Dispatch-capability resolution* rule and the "never from a tool name" clause)
lands in `skills/docket-finalize-change/references/gate-failure.md` — extending the §"The two
agents" area, one clause per agent noun. Not in `SKILL.md`'s step 2/step 5 dispatch sentences:
both of those already say "read `references/gate-failure.md` now (blocking)" at exactly the moments
dispatch happens, so gate-failure.md is loaded at dispatch time and is the single canonical home
for both sites (no copy-pinning across two files).

Also add "dispatch mechanism unavailable for either gate agent" as a named member of the
*abort-and-report points (the full set)* enumeration, so the carve-out's posture pointer resolves to
a listed reason rather than an implied one.

### 3. Push-denial sentence in the abort-and-report set

Extend the enumeration in `gate-failure.md` §*abort-and-report points* with one member: a harness
or permission denial of the gate's **post-rebase `--force-with-lease` push** (named by that noun,
never by a step number — see Problem §2 for the numbering drift and the "gate-step-5" collision) —
fires only **after** the convention's *Harness-native recovery* retry (retry the exact command once
through the harness's native approval mechanism) has been exhausted or is unavailable; same
`halted`-not-retry-loop posture as the existing merge-denial member, and points the reader at the
*Harness-native recovery* section by name. The user-level allow-rule direction stays untouched
(`ensure-claude-settings.sh` keeps force-push guarded — settled security posture, out of scope).

Housekeeping owned by the same edit: the `## Finalize blocked` section of `gate-failure.md`
currently says "the **six** distinct abort reasons" while the enumeration it references grows to
nine members with this change — **de-numeralize** that sentence ("the distinct abort reasons")
rather than re-counting, so the sentence cannot rot again when the set next changes.

### 4. Test rewiring (`tests/test_dispatch_capability.sh`)

- The two sites become ordinary `check_site` rows anchored in gate-failure.md (per the file's own
  maintainer note), with posture label the carve-out's name rather than `Tier A|B|C`, nouns
  `docket-rebase-resolver` and `docket-integration-repair`. Each row inherits the standard asserts:
  site found, cites *Dispatch-capability resolution*, forbids the tool-name conclusion, names the
  posture in the same clause as its own noun.
- The table-coherence loop (`tier_row_names`, keyed on `^\| \*\*<letter> —` rows) cannot digest a
  non-letter posture: carve-out rows are **excluded from that loop** (records tagged, or the loop
  skips non-`Tier` labels), replaced by a dedicated assert that the convention's carve-out
  paragraph names **both** agent nouns — preserving the cross-file agreement property the loop
  exists for.
- `PENDING_TIER` shrinks to empty but the **variable and its pin stay**, asserting a count of 0 —
  keeping "a new knowingly-untiered site is an in-diff decision, never a silent one" as a live
  guard rather than deleting the mechanism.
- The consumer-coverage floor (`seen -eq 6`) rises to 8; the reverse-derivation population floor
  (`-ge 11`) is re-derived by running the greps per the maintainer note and raised only after
  confirming every name is covered (new prose in gate-failure.md and the convention adds mentions).
- `tests/test_finalize_gate.sh` gains sentinel asserts on gate-failure.md for the two new
  enumeration members (push-denial + dispatch-unavailable) — sentinels sampling presence, not
  parsing, per foundational-test-discipline.

## Out of scope

- Reversing the guarded-force-push settings posture (deliberate security stance — stands).
- Any change to the rebase-resolver or integration-repair contracts themselves, or to when
  finalize dispatches them.
- Re-litigating 0137's three tiers.

## Assumptions

1. **Carve-out over a fourth tier.** Chosen: an explicit carve-out paragraph preserving finalize's
   existing abort-and-report. Rejected: a Tier D table row (implies a new posture *kind* when the
   posture is finalize's pre-existing one, and forces the coherence loop to treat an outside-the-
   taxonomy case as a taxonomy member); rejected: inline resolution/repair with sign-off (the
   self-approval shape #0139's body itself argues against — the agent would merge its own repair).
   #0139 was killed *into* this change with halt/carve-out as its own stated conclusion.
2. **Carve-out posture = abort-and-report, no new status.** Chosen: reuse the existing set (PR
   open, change `implemented`, comment + `## Finalize blocked` marker). Rejected: a new
   `halted`-flavor status or field — same reasoning the `## Finalize blocked` design already
   records (an eighth status flattens distinct reasons and touches board/mirror/health checks).
3. **Site marker lives in gate-failure.md, not SKILL.md's dispatch steps.** Chosen: gate-failure.md
   is blocking-loaded at exactly the dispatch moments and is the canonical failure-flow home; one
   file, two nouns, no copy-pinning. Rejected: marking SKILL.md:124/:147 (two more marker sites to
   keep in agreement, and check_site's paragraph-scoped asserts would need two anchors there
   anyway); rejected: marking both files (copy-pinning the exact pairing 0137's tests exist to
   avoid duplicating).
4. **Dispatch-unavailability becomes a named abort-and-report member.** Chosen: add it to the
   enumeration. Rejected: leaving it implied by the carve-out's pointer — the enumeration is
   titled "the full set" and the tests treat it as such; an unlisted reason erodes that contract.
5. **Push-denial member is conditioned on Harness-native recovery first.** Chosen: the enumeration
   entry names the one-retry-via-native-approval remedy (shipped by change 0128) and only then
   halts, matching the merge-denial member's "if a denial still fires, that is a `halted`, not a
   retry loop" shape. Rejected: an unconditional halt on first denial (discards the shipped
   remedy); rejected: any allow-rule change (settled posture, explicitly out of scope).
6. **PENDING_TIER stays as an empty-pinned guard.** Chosen: keep the variable, assert count 0.
   Rejected: deleting the block (loses the "adding a third untiered site is an in-diff decision"
   property the comment explicitly assigns to it).
7. **Coherence-loop handling of a non-tier posture.** Chosen: skip carve-out-labeled rows in the
   letter-keyed loop and add a dedicated both-nouns assert against the convention's carve-out
   paragraph. Rejected: forcing a fake letter (misstates the design in the test's own vocabulary);
   rejected: no convention-side check at all (re-opens the swapped-subjects blind spot the loop was
   built to close).
8. **No changes to the convention's Composition paragraph.** It already states the in-context
   nature of these two dispatches accurately; the carve-out cites it rather than restating it.
9. **The carve-out's label literal is `carve-out`.** Two literal-bearing surfaces agree on the
   word: the `check_site` posture argument (spliced raw into an ERE at the clause-proximity assert,
   so it must be metacharacter-free) and the clause next to each agent noun in `gate-failure.md`;
   the third surface — the test's coherence loop — never consumes the literal at all: it excludes
   carve-out records by **shape** (any posture label not `Tier <letter>`-shaped is skipped), not by
   name. Chosen: the bare hyphenated word `carve-out` (metacharacter-free, already the stub's and
   #0139's own vocabulary; the convention's carve-out paragraph and the gate-failure clauses use it
   verbatim). Rejected: a `Tier D`-style pseudo-letter (misstates the design — see Assumption 1);
   rejected: a multi-word phrase like `outside the taxonomy` (its space would be split by the
   loop's `${t##* }` last-word extraction, and it adds zero meaning over the single word).
