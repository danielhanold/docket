<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0184 — Four-tier build profile ladder — economy/standard/premium/max](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0184-four-tier-build-profile-ladder.md)**
<!-- docket:backlink:end -->

# Four-tier build profile ladder — results

Change: #184 · Branch: feat/four-tier-build-profile-ladder · Plan: docs/superpowers/plans/2026-08-01-four-tier-build-profile-ladder.md · ADRs: none

## Verify (human)

- [ ] **Start a fresh session before the next `docket-build` run.** `install.sh` has been re-run (during the late rename below), so the wrappers on disk are current — but harnesses register agent definitions only at process start, and this session still has the pre-0184 three registered. Until a fresh session, a build halts on the harness rejecting a dispatch to `docket-build-max`, the one genuinely new agent — which is the documented, correct behavior, not a defect.
- [x] **Certify `cursor-grok-4.5-low` against the live Cursor catalog.** ✅ **Confirmed valid by Daniel, 2026-08-01.** This is the one shipped value in the change that is *new*, not a re-pointed existing ID: the cursor family previously shipped only `-low-fast`, `-medium`, `-high`, and `-high-fast`, and a boundary-anchored grep found no occurrence of `cursor-grok-4.5-low` anywhere in the repo's history. Per ADR-0015 docket keeps no vendor allowlist, and a harness handed an unknown model ID silently runs its house default — the hermetic suite compares the wrapper against the sidecar, so **both sides move together and no test in this repo can ever detect a wrong ID.** `docs/cursor/validation.md` Phase 7 step 1 already requires an explicit `**Build profile:** economy` dispatch that reports its resolved model; that run is the certification.
- [ ] **Check other clones for stranded config overrides.** A repo-committed rename cannot reach `~/.config/docket/config.yml` or a machine-local `.docket.local.yml`. Both were inspected on this machine during planning and neither sets any `build-*` agent key, so nothing is stranded here. Any *other* clone overriding `build-economy` / `build-standard` / `build-premium` silently falls back to shipped defaults until its owner renames the keys — the known, accepted cost of the clean break.

## Late rename — the shipped names are economy/standard/premium/max

The ladder shipped in this change's build as `low`/`medium`/`high`/`max`. Before merge that was
reversed to **`economy`/`standard`/`premium`/`max`** on two objections, both sound:

1. **The profile names collided with the effort vocabulary.** `agents/harness-defaults.yml` carries
   both ladders in the same table, and they deliberately do not line up — `build-low` shipped at
   effort `xhigh` on Codex, `build-high` at effort `low`. Two identical four-word ladders that
   disagree row by row read as a typo rather than as two axes.
2. **No name identified the default.** `standard` carries "this is the everyday rung" in the word
   itself; `medium` only means "the middle one", which is a different claim.

Rungs 1–3 revert to their pre-0184 names; `max`, the rung this change actually added, keeps its
name. The rename is mechanical everywhere except the retirement guards, whose polarity had to
invert — they now forbid `low`/`medium`/`high` as profile names. Both were mutation-proven in the
new direction: a bare `low` appended to the controller reddens the bare-word arm, and a
`docket-build-medium` planted in `docs/cursor/validation.md` (a surface no per-file assert covers)
reddens the repo-wide arm and reports the hit.

One property improved in the reversal. The bare-word ban on `low|medium|high` across the controller
and worker contracts is only sound because neither may state an effort tier — the controller's own
"never restate literal model IDs or effort tiers" rule. So that assert now enforces two properties
at once, and the no-effort-tiers rule gains the detector it never had. The equivalent ban under the
old names could never have been written: `economy`/`standard`/`premium` are unusual enough to grep
as bare words, which is what made the original guard possible, but that cut only one way.

## Findings

- **No ADR was minted, deliberately.** The candidate decisions — the `max`/`premium` boundary (irreversibility, not severity), the three-doors rarity construction, and the pin-compression posture — are all stated **normatively** in `skills/docket-build/SKILL.md` and the `agents/harness-defaults.yml` block comments, which are what a router and a retuner actually read. An ADR would restate them, and this repo has an explicit, tracked allergy to restatement (the copy becomes load-bearing and drifts). The decisions were also settled at brainstorm and recorded in the spec, not made during implementation.
- **The suite cannot see a wrong model ID, by construction.** Recorded here because it is the structural reason merge-gate item 2 exists rather than a test: every pin assert compares generated output against the sidecar, so the sidecar is both the input and the oracle. Only a live harness run distinguishes a valid ID from a typo.
- **Two claude ladder invariants became false by design and were replaced, not patched.** Efforts are no longer pairwise distinct (`low, low, medium, high`) and the models are no longer all one (`claude-sonnet-5` on the bottom rung). The surviving invariant is **model/effort pair distinctness**, which is also the principle the codex block's own header already argued — codex deliberately reuses `gpt-5.6-sol` at two efforts, so a model-distinctness assert would have been wrong there. Both new asserts carry a non-vacuity half (four non-empty values counted before `sort -u`) so a deleted row cannot collapse into a silent pass.
- **The repo-wide retirement guard paid for itself immediately.** On its first run it caught a live surface every per-file assert had missed — a `build-premium` reference surviving in `tests/test_docket_example_yml.sh`'s anchor-rationale comment. That is the case for a whole-tree guard over per-file greps: no per-file assert can see a surface nobody thought to list.
- **A guard that had gone silently vacuous was re-armed.** `tests/test_cursor_dispatch_rule.sh` gated its head asserts on `n_cursor_pinned -lt n_src`, which has been false since change 0168 completed the Cursor block — so the head's claim went unchecked in exactly the state where it was wrong, and `cursor-rules/dispatch.head.md` shipped a false "three build-profile workers only … every other wrapper is generated unpinned" line verbatim into every consumer repo's `.cursor/rules/docket-dispatch.mdc`. Both the prose and the missing `else` arm are fixed here. **This absorbed open stub #183**, which tracked the same stale claim; #183 was re-read against this branch and killed on 2026-08-01.
- **Whole-branch review: no Critical, two Important, both addressed.** The unvalidated Cursor ID became merge-gate item 2 above. The second — the prose retirement guard covered `economy|premium` only, leaving a *bare* `standard` token with no detector in either skill body — was fixed in `dce8876c` and mutation-proven (appending a bare `standard` sentence to the controller takes the count 0 → 1 and reddens exactly that assert).

## Plan deviations

All were reported by their workers and are judged justified:

- **A plan regex could never have matched.** Task 2 Step 1's `low` rubric assert omitted the em dash that Step 3's prose writes (`- **\`low\`** — *only when*`), while the sibling `medium` assert included it. The prose was kept verbatim and the assert corrected — without this the plan's own Step 1 and Step 3 contradicted each other.
- **Task 3 was under-specified and the worker extended it correctly.** `tests/test_docket_example_yml.sh` failed 29 asserts, not the ~27 the plan enumerated: a codex generator round-trip and a codex sentinel both reference generated wrapper *filenames* that the rename moved. Both were repointed preserving intent.
- **An unplanned file entered the diff by the repo's own rule.** The four-tier rubric grows `skills/docket-build/SKILL.md` past its `tests/test_skill_size_budgets.sh` row; that file's header requires the row to move in the same diff, so it did (250/2350 against a measured 247/2313).
- **One in-scope fix the plan did not list:** `agents/harness-defaults.yml`'s key-format comment documented `(build-economy, not docket-build-economy)`; updated with the block it annotates.
- **A dead accumulator in a plan snippet was dropped** (`efforts=""` with no consumer after the loop was restructured).

## Follow-ups

- **#187** (minted by auto-capture from the review) — harden `.docket.example.yml`'s mirror guards: the sidecar→example loop runs one direction only while both blocks claim completeness (a mirror, so the reverse loop is mandatory); the round-trip slice terminates above the cursor build rows so those rows never get resolver coverage, and its row-count comment is wrong; and each slice terminator is prefix-weak (`claude-opus-5` is a strict prefix of `claude-opus-5-high`). All three are pre-existing — this change renamed the rows they check but created none of the gaps.
- **#183** — re-read against this branch first; `cursor-rules/dispatch.head.md`'s stale claim and its vacuous guard are both fixed here.
