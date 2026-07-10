# Design: slim docket-implement-next + propagate the Step-0 preamble to the small skills

**Date:** 2026-07-10 · **Change:** 0055 · **Depends on:** 0053 (in-progress at design time) ·
**Parent:** `2026-07-10-docket-skill-slimming-design.md` (#0053 — strategy, research, and the
eight-skill categorization; this spec executes its §4 rows for `docket-implement-next` and the
four small skills)

## Problem

`docket-implement-next` (~137 lines / ~2.9k words on `main`) restates the
`render-change-links.sh` regeneration litany at its three field-write sites (steps 4, 6, 7 — the
stub said four; the actual count is three), carries the full Step-0/mode boilerplate, and restates
the reconcile-kill command sequence that #0053's `references/terminal-close-out.md` now
single-sources. The four small skills (`docket-new-change` 70 L, `docket-groom-next` 77 L,
`docket-adr` 88 L, `docket-auto-groom` 64 L) each carry the same two-paragraph Step-0 preamble
near-verbatim; new-change additionally duplicates the kill sequence (~25 lines). The parent spec
rated implement-next **medium potential / medium risk** — its repetition is partly deliberate
reinforcement for an autonomous agent — and the small skills low/low.

## Decisions (brainstormed 2026-07-10, human-approved)

1. **Named-rule dedupe** for implement-next (chosen over litany-only-conservative and
   aggressive-trim): one **field-write rule** stated once inside *Branch & metadata discipline*
   (which stays, per the stub); the three field-write steps become one-line pointers to it. The
   repetition being cut is a mechanical invariant that fires identically at every site — naming it
   once makes it more salient, and `test_change_links_coverage.sh` guards it independently. The
   full `render-change-links.sh` invocation survives exactly once per skill.
2. **The compressed Step-0 pointer keeps the literal one-line
   `eval "$(…/docket-config.sh --export)"` in every skill body** — `test_docket_config.sh`'s
   per-skill loop requires the `/docket-config.sh` literal, and the body stays self-executable
   without a hop into the convention. (This deliberately differs from #0053's `docket-status`
   compression, which relies on other occurrences of the literal in that file.)
3. **Both kill paths rewire onto the full terminal-close-out sequence** — including its step-2
   `## Artifacts` re-render, which today's kill flows lack. This is the spec's one named
   behavior delta (everything else is behavior-neutral): benign in practice (a killed change has
   no `plan:`/`results:` to re-point, so the re-render is a no-op), and it collapses the last two
   divergent callers onto the single sequence. It exposes one edge the reference must close:
   **a no-diff re-render is success** — the skip-publish guard fires on a *failed* step-2
   commit/push, and an empty diff (nothing to re-point) must not be read as failure. #0055 adds a
   one-line clarifier to `references/terminal-close-out.md` saying exactly that (commit only when
   the block changed; unchanged ⇒ proceed to publish).
4. **`docket-new-change` keeps its must-land board posture on the proposed-kill** — deliberately
   stricter than the reference's "steps 4–5 best-effort everywhere" default, and test-anchored
   (`must-land Board pass`). The caller prose states the divergence explicitly.
5. **Sentinels follow content** (#0053 precedent): tests re-point only where the guarded content
   moves into the reference; must-stay phrases keep their grammatical location (learnings
   #36/#37); the `## Convention (load first — blocking)` section stays verbatim in every skill
   (the convention cannot instruct its own loading).
6. **No semantics change** to selection, the claim CAS, reconcile, plan/build/review, PR flow, or
   kill outcomes. The only behavior delta is the kill-path re-render named in decision 3.

## Design

### 1. docket-implement-next (~137 → ~95–100 lines)

- **Step 0** → ~4-line pointer: run the convention's *Step-0 preamble* (keeping the literal
  one-line config eval per decision 2), act on `BOOTSTRAP`, writes land on `origin/docket`.
  **Stays:** the unconditional foreground `docket-status` dispatch before selection, its
  git-state-not-in-context-return contract, and the wrapper-resolved-tier phrasing (no
  model/effort literals — `test_composition_wiring.sh`).
- **Step 3 reconcile-kill** → rewire to `references/terminal-close-out.md` with
  `--outcome killed` (same shape as #0053's status-sweep rewire): the reference owns invocations,
  ordering, and the `main`-mode degradation; the skill keeps caller posture only — trust each
  exit code, a failure aborts the kill and is surfaced, then loop back to Step 1 with a
  best-effort board refresh. The OBSOLETE/FUNDAMENTALLY-INVALIDATED escape-hatch distinction
  stays verbatim.
- **Field-write rule** (new, in *Branch & metadata discipline*): every change-file field write
  (claim, reconcile, `status:`, `plan:`, `adrs:`, `pr:`, `results:`) is a metadata commit — made
  in the metadata working tree on `metadata_branch`, never in the feature worktree, pushed
  immediately; a write to a **link-bearing field** (`spec:`, `plan:`, `adrs:`, `pr:`, `results:`)
  additionally regenerates the change's `## Artifacts` block in the same commit via the full
  `render-change-links.sh` invocation (sole writer of the block). Steps 4/6/7 shrink to
  "…per the **field-write rule**" one-liners. Scope note: the rule must not over-generalize — a
  claim writes `status:`/`branch:` (metadata discipline applies, Artifacts regen does not).
- **Stays verbatim:** Step 1 selection, the Step 2 CAS loop, the Step 4 SHA-compare push
  confirmation and cross-tree spec-read explanation, the reconcile-pass + `reconciled`-flag
  section (narration trims only — its flag semantics and resume-safety rule are load-bearing),
  *Best-effort board refresh* (heading + ≥3 `run the Board pass (best-effort` occurrences —
  `test_board_refresh_on_transition.sh`), all four `SKILL_*` role references, ≥2 `LEARNINGS.md`
  mentions, the results-close-out section.
- **Provenance narration cut**; bare `(ADR-NNNN)` pointers and cross-skill rationale one-liners
  (e.g. why `docket-status` ignores a missing `plan:` on `implemented`) stay.

### 2. The four small skills

| Skill | Size (L) | Edits |
|---|---|---|
| docket-new-change | 70 → ~55 | Step-0 → preamble pointer; proposed-kill → close-out reference, keeping: the two-kill-origins framing (one line), the nothing-to-clean-up note (a `proposed` change never had a branch — the reference's step 4 has nothing to remove), and the **must-land Board pass** posture (decision 4) |
| docket-groom-next | 77 → ~65 | "Where everything is read and written" → preamble pointer; selection bands, recap contract, `LEARNINGS.md`, `SKILL_BRAINSTORM` all stay (test-anchored) |
| docket-adr | 88 → ~78 | "Where ADRs are read and written" → preamble pointer; publish wiring untouched |
| docket-auto-groom | 64 → ~58 | "Where everything is read and written" → preamble pointer; critic dispatch untouched |

Each small skill keeps its single full `render-change-links.sh` statement (one occurrence
satisfies `test_change_links_coverage.sh`); the field-write rule is implement-next-internal.
Provenance narration cut across all four.

### 3. Sentinel discipline (hot list; exhaustive disposition table is plan-time, per #0053)

- `test_docket_config.sh` per-skill `/docket-config.sh` loop — **no edit** (decision 2 keeps the
  literal).
- `test_docket_metadata_branch.sh` K3/K4 (`the integration branch), performing the archive move`
  in new-change + implement-next) — the clause moves; the reference's `main`-mode section phrases
  it as "the step-1 archive commit is itself the terminal record". Re-point to the reference's
  phrasing + add caller-pointer asserts (`grep -qF "terminal-close-out.md"` on both skills).
- `test_docket_metadata_branch.sh` kill-wiring greps (`kill` + `terminal.publish`) — **no edit**:
  the rewire's summary line names terminal-publish, as the status-sweep rewire's does.
- `test_closeout.sh:372–391` NEWCHG/IMPL asserts — re-point the script-invocation asserts to the
  reference (0053's Task-4 pattern; label rename to "close-out ref"); keep caller-posture asserts
  on the skill files.
- `test_board_refresh_on_transition.sh` — implement-next keeps the heading + ≥3 count;
  new-change keeps `must-land Board pass`. **No edit.**
- `test_auto_groom.sh`, `test_groom_recap.sh`, `test_learnings_ledger.sh`,
  `test_results_artifact.sh`, `test_adr_checks.sh`, `test_render_adr_index.sh`,
  `test_convention_extraction.sh` (operating-skill loop) — anchored content stays in place;
  **no edit** expected.

### 4. Verification (build-time acceptance; mirrors #0053 §5)

1. **Anchor grep-gate:** no reference from the five files to a convention section heading or
   reference file that does not exist on the merged base.
2. **Behavior-neutrality diff pass:** every deleted sentence is (a) narration, (b) restated
   inline, (c) present in a reference file, or (d) covered by a script contract — with decision
   3's re-render addition as the one recorded exception.
3. **Full suite green**; sentinel edits confined to re-points of moved content.
4. **Read-only Step-0 smoke** of one groomed skill (config export → `BOOTSTRAP` → metadata-tree
   sync) against this clone.
5. **Sizes recorded in the PR body:** implement-next ≤ ~100 L; new-change ~55 L; groom-next
   ~65 L; adr ~78 L; auto-groom ~58 L.

## Scheduling / dependency posture

`depends_on: [53]` — #0053 was mid-build at design time; this spec cites its plan's landed-shape
anchors (`### Step-0 preamble (every operating skill)`, `references/terminal-close-out.md`). The
build-time reconcile re-verifies both against what #0053 actually merged, plus #0054's landed
state (disjoint files — finalize only — so no ordering constraint; if #0054 lands first and
already re-pointed shared test lines, fold in, don't duplicate).

## Out of scope

- Any semantics change beyond decision 3's kill-path re-render.
- `docket-convention`, `docket-status`, `docket-finalize-change` (#0053/#0054 files) — except the
  single no-diff-is-success clarifier line in `references/terminal-close-out.md` (decision 3).
- Scripts, script contracts, frontmatter `description:` lines, agent wrappers.
