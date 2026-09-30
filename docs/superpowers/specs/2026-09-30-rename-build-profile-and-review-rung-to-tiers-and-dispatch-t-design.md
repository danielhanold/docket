<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0473 — Rename build profile and review rung to tiers, and dispatch tiers to dispatch fallbacks](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-30-0473-rename-build-profile-and-review-rung-to-tiers-and-dispatch-t.md)**
<!-- docket:backlink:end -->

# Tiers rename (ADR-0129 family (c)) — design

**Change:** 0473 · **Date:** 2026-09-30 · **Status:** groomed

## Problem

ADR-0129 settled a collision-free docket vocabulary. The strength of a worker is called a **profile**, a **rung** or a **tier** depending on the page, and what a workflow does when it cannot dispatch an agent is named by meaningless letters (Tier A / B / C) plus "the carve-out". This change applies ADR-0129's family (c): rows 46, 46a, 47, 47a and 48–52.

Traced at grooming (2026-09-30, `origin/main` at `c38ca3bed`, maintained source, `*_test.go` included):

- **No wire tokens and no Go identifiers.** No flag, JSON key, finding code, schema vocabulary or Go identifier carries these terms. ADR-0129's illustrative `reviewRung` does not exist. The Go footprint is two test pins, one synthetic test string and two comments.
- **Prose volume:** about 60 maintained lines use "rung", about 140 use "profile" in the worker sense, and about 30 name the dispatch tiers or the carve-out — across skills, agents, cursor dispatch rules, `agents/harness-defaults.yml` comments, `.docket.example.yml`, docs and the glossary.
- **Three skill labels are live protocol** (row 46a): a line format one skill writes and another reads, with no Go parser. `**Build profile:** <tier>` appears in 18 committed plans.
- **"tier" already has other senses.** Most are harmless (model and effort cost tiers, validation Tier 1/2/3, the docs' "three tiers", the learnings "tiering criterion"). Two collided with "review tier" / "build tier" and are settled below (row 47a; "no tier layer").

## Decisions (settled at grooming)

1. **Apply ADR-0129 rows 46, 46a, 47, 47a, 48–52.** The human authorized editing ADR-0129 in place at grooming (no amendment section): it gained row 46a (kind **label**), row 47a, the phrase "authorized-or-halt" in row 51's old column, the kept senses under "Explicitly not renamed", and family (c)'s drain rule under Consequences.
2. **`auto-or-halt` is the single name** for the plan-writer / build / review / fix-worker fallback. The phrase "authorized-or-halt" (8 maintained uses) is replaced.
3. **Severity is never a "tier"** (row 47a): "severity-tiered findings" → "severity-ranked findings", "the tiers a reviewer assigns" → "the severity levels a reviewer assigns", "tiered by severity" → "ranked by severity".
4. **`skills/docket-convention/references/agent-layer.md`: "no tier layer" → "no model-alias layer".** Same meaning (ADR-0015: model values are direct IDs, never labels docket maps onto models), without reading as a denial of build tiers. Prose only, no ADR row; ADR-0015 itself is a frozen record and keeps its wording.
5. **Skill labels are hard-cut (row 46a)**, with no alias and no refusal text naming the old spelling; the change lands after a drain (see *Landing*).
6. **The convention keeps ADR-0086's layout:** a three-row fallback table, then the `no-fallback` paragraph after it — not a fourth row. Only the names change.
7. **The concept page file is renamed**, following 0471's precedent (`run-gate.md` → `run-tracker.md`).
8. **No new guard** (0468's decision: a guard over passing prose mentions violates the repo's guard rule). Verification is the full suite plus one closing grep.
9. **Kept:** the tier names inside each tier, agent names, the "ladder" metaphor for the ordered tiers, generic "tier" senses, "profile" in its unrelated senses, and the shared words `inline` (also a `board_surfaces` token, a different domain) and `abstain` (the critic's fallback *is* the groom's abstain).

## Design

### 1. Rename map

**Build and review strength (rows 46, 47)**

| Old | New |
|---|---|
| build profile(s); profile agent / worker | build tier(s); tier agent / worker |
| "routed to the ECONOMY / STANDARD / PREMIUM / MAX profile"; "no profile above you" | "… tier"; "no tier above you" |
| "Economy build-profile worker …" (agent descriptions); "the cheapest of docket-build's four profiles" | "Economy build-tier worker …"; "… four tiers" |
| profile-routed; task-to-profile; "character→profile rubric" | tier-routed; task-to-tier; "character→tier rubric" |
| review rung(s); reviewer rung; "select the reviewer rung" | review tier(s); "select the review tier" |
| "routed to the LEAN / STANDARD / DEEP rung"; "one rung, one pass"; "a stronger rung" | "… tier"; "one tier, one pass"; "a stronger tier" |
| rung-routed; cap rung; "one rung below max"; "the review rungs" | tier-routed; cap tier; "one tier below max"; "the review tiers" |
| "## The reviewer and its rungs"; "## Choosing the rung" (review guide) | "## The reviewer and its tiers"; "## Choosing the review tier" |
| "## Build profiles and the one escalation" (build guide) | "## Build tiers and the one escalation" |

**Skill labels (row 46a)** — producer and consumer change in the same commit:

| Old | New | Producer → consumer |
|---|---|---|
| `**Build profile:** <tier>` | `**Build tier:** <tier>` | plan writer → docket-build (`## Routing`, *Halting conditions*, `references/task-routing.md`) |
| `PROFILE: <tier> — <reason>` | `TIER: <tier> — <reason>` | docket-build-task's return block → docket-build |
| `Profile: <tier> (…)` / `Rung: <tier> (…)` | `Tier: <tier> (…)` | the dispatch prompt examples in `cursor-rules/dispatch/docket-build-*.md` / `docket-review-*.md` |

**Dispatch fallbacks (rows 48–52)**

| Old | New |
|---|---|
| dispatch tiers (A / B / C) and the carve-out | dispatch fallbacks |
| Tier A; "the convention's Tier A path"; "It is Tier A" | the `inline` fallback; "its dispatch fallback is `inline`" |
| Tier B | the `abstain` fallback |
| Tier C; Tier-C evidence; authorized-or-halt | the `auto-or-halt` fallback |
| the carve-out; "## Dispatch unavailability — the carve-out"; "the carve-out below" | `no-fallback`; "## Dispatch unavailability — no fallback"; "the `no-fallback` posture below" |
| "the posture is tiered"; "degrades in tiers"; "tiered unavailability" | "the fallback differs by kind"; "falls back by kind"; "per-kind fallbacks" |

**Collisions settled at grooming (row 47a; Decision 4)**

| Old | New |
|---|---|
| "severity-tiered findings" | "severity-ranked findings" |
| "The tiers a reviewer assigns" | "The severity levels a reviewer assigns" |
| "tiered by severity" | "ranked by severity" |
| "no tier layer" (agent-layer.md) | "no model-alias layer" |

Meaning is unchanged everywhere: this is vocabulary, not policy. Exact sentence wording is the implementer's, within the budgets in §6.

### 2. Sites (derived by grep at grooming)

The implementer re-derives every site with the closing grep in *Testing*, never from this list.

- **Skills and references:** `docket-convention` (`SKILL.md` *Dispatch-capability resolution*, the *Composition* paragraph's "unavailable dispatch is Tier C", the *Skill layer*'s "Tier-C evidence"; `references/agent-layer.md`), `docket-implement-next` (`SKILL.md` Step 4 plan-writer posture, Step 5 build posture, Step 6 review-tier selection and review posture; `references/fix-loop.md`, `references/edge-paths.md`), `docket-build` (`SKILL.md`, `references/task-routing.md`), `docket-build-task` (the return block), `docket-review`, `docket-auto-groom` (Tier B, three sites), `docket-status` ("Tier A path"), `docket-finalize-change` (`SKILL.md`, `references/gate-failure.md`).
- **Agents:** the four `agents/docket-build-*.md` and three `agents/docket-review-*.md` (descriptions and bodies); comments in `agents/harness-defaults.yml`.
- **Cursor rules:** `cursor-rules/dispatch.head.md`, `cursor-rules/dispatch/docket-build-*.md`, `cursor-rules/dispatch/docket-review-*.md`.
- **Sample config:** `.docket.example.yml` comments.
- **Docs:** `docs/reference/glossary.md`; `docs/concepts/` (`build-profiles-and-gate.md`, `README.md`, `skills-agents-dispatch.md`); `docs/guide/` (`building-without-supervision.md`, `reviewing-before-the-human.md`, `proving-the-build.md`); `docs/install/delegating-across-harnesses.md`; `docs/reference/skills-and-agents.md`, `docs/reference/harness/validation.md`, `docs/reference/harness/validation-runbook.md`; `docs/README.md`; `docs/comparison/ai-native-sdlc-playbook.md`; `scripts/runners/opencode.md` (retired-runner contract, still in the tree).
- **Go tests:** `internal/harness/inventory_test.go` (asserts `"STANDARD profile"` in the build-standard body → `"STANDARD tier"`); `internal/repoguard/inline_role_stop_test.go` (pins `"One shot at the dispatched rung"` in `skills/docket-review/SKILL.md`); `internal/repoguard/skill_handoff_sites_test.go` (synthetic string `"Dispatch the selected rung wrapper by name, foreground"`); two comments in `internal/harness/codex/codex_test.go`.

### 3. Files and anchors

- `docs/concepts/build-profiles-and-gate.md` → `docs/concepts/build-tiers-and-gate.md`, title "Build tiers and the suite gate". Update its four inbound links (`docs/README.md`, `docs/concepts/README.md`, `docs/guide/building-without-supervision.md`, `docs/guide/reviewing-before-the-human.md`). Frozen records that link the old path are not edited.
- Glossary headings and anchors: *Build profile / escalation* → *Build tier / escalation* (`#build-tier--escalation`, including the index's *Escalation (NEEDS_ESCALATION)* link); *Review rung* → *Review tier* (`#review-tier`); *Dispatch tiers (A / B / C) and the carve-out* → *Dispatch fallbacks* (`#dispatch-fallbacks`). Update the alphabetical index to match.

### 4. Glossary entries

- **Build tier / escalation** and **Review tier:** the renamed entries.
- **Dispatch fallbacks:** defines `inline` (status, ADR), `abstain` (the critic), `auto-or-halt` (plan writer, build, review, fix workers; in Go v1 an explicit `skills.*` value blocks mutation, so in practice it halts) and `no-fallback` (the finalize rebase resolver and integration repair: abort-and-report, never inline).
- **Finding severity**, **Fix loop**, **docket-review**, **docket-adr**, **docket-status:** row 47a wording, "build tiers", "ranked by severity", "its dispatch fallback is `inline`".
- No old→new mapping in the glossary (ADR-0129 Decision 10).

### 5. The convention's dispatch-capability section

- Lead-in: "so the posture is tiered" → "so the fallback differs by kind".
- Table: column `Tier` → `Fallback`; rows `` `inline` — deterministic ``, `` `abstain` — adversarial ``, `` `auto-or-halt` — discipline `` (the kind words stay as the rationale). The `auto-or-halt` row's posture opens with **`auto-or-halt`.** instead of **Authorized-or-halt.**
- The paragraph after the table becomes "**Outside the table — `no-fallback` (change 0260).**" and keeps its reasoning in the new names: neither `inline` nor `auto-or-halt` can be borrowed for these dispatches, and inline substitution is the self-approval shape `abstain` exists to avoid for the critic.
- "Tier C neither replaces nor softens the *Skill layer*'s missing-skill rule … is Tier C" → "`auto-or-halt` neither replaces … is `auto-or-halt`".

### 6. Regenerated artifacts and budgets

- `internal/assets/embedded/tree/**` is regenerated with `go generate ./internal/assets`; the harness goldens (`internal/harness/{claude,codex,cursor,opencode}/testdata/golden/`) with each package's tests run with `-update`. Never hand-edited.
- Frozen snapshots are never edited: `internal/install/legacydata/`, `internal/install/testdata/legacy/`, `testdata/repositories/v0.9.*/`, `internal/repository/testdata/`, `docs/reference/harness/fixtures/`, plus the history comments in `internal/repoguard/budgets_test.go`.
- **Budgets:** the skill line/word ceilings in `internal/repoguard/budgets_test.go`. Most affected files sit exactly at their ceiling (at grooming: `docket-implement-next/SKILL.md` 214/214 lines and 8175/8175 words, `docket-review/SKILL.md` 110/110 and 913/913, `docket-build/SKILL.md` 4478/4479 words, `references/agent-layer.md` 2349/2350 words). Swaps are made in place, word-for-word or shorter, with no reflow that adds lines. **No ceiling is raised**: a file that would exceed its ceiling gets tighter wording instead.

## Testing

- **The full suite** through `build.test_command`. Read the budget report even when it is green.
- **Updated pins are mutation-checked:** for `inventory_test.go` and `inline_role_stop_test.go`, remove the new phrase from the asserted file, watch the test go red, restore.
- **Closing grep**, run from the repo root with the exclusion pathspecs `docs/changes`, `docs/results`, `docs/superpowers`, `docs/adrs`, `internal/install/legacydata`, `internal/install/testdata/legacy`, `testdata/repositories`, `internal/repository/testdata`, `docs/reference/harness/fixtures`. It must show:
  - no "rung" word (`rung`, `rungs`, `Rung`; `rungate` and `runGit` are not hits);
  - none of: `authorized-or-halt`, `severity-tiered`, `tiered by severity`, `tiers a reviewer`, `no tier layer`, `Tier A`, `Tier B`, `Tier C`, `Tier-C`, `dispatch-tier`, `dispatch tiers`, `degrades in tiers`, `tiered unavailability`, `posture is tiered`, `**Build profile:**`, `PROFILE:`, `Profile:`, `Rung:`;
  - "profile" only in its unrelated senses: `internal/app/change_attach.go` (verification profile), `internal/app/maintenance_traffic_integration_test.go` (remote-call profile), `internal/suiterunner/budgetstate.go` (budget profile), `tests/runtime-budgets.tsv` ("profilers");
  - "carve" only in non-dispatch senses or history: `internal/repoguard/capability_surface_test.go`, `skills/docket-build/references/gate-execution.md`, the `internal/repoguard/budgets_test.go` history comment.

  The embedded tree and goldens are inside the grep's scope, so they prove the regeneration ran. The results file records the grep command and its output.

## Landing

- **Drain first** (ADR-0129 Consequences): merge, and switch any machine to the new binary, only when no change in any docket repo is mid-build or holds a committed-but-unbuilt plan. A plan written before the upgrade may carry `**Build profile:**`, which the new docket-build does not read, so a risky task could silently route to a cheaper tier. 0473's own plan and build run under the pre-merge installed skills, so its own plan may use the old label.
- The usual post-merge binary rebuild (CLAUDE.md).
- Results **Human action** (Optional): re-run `docket install` in consumer repos and restart agent sessions, so generated agent descriptions and cursor rules show the new wording. Nothing breaks without it.

## Out of scope

- Any alias, dual spelling or refusal text for the old labels (ADR-0129 Decision 2).
- Renaming agent names, the tier names, config keys or frontmatter fields (ADR-0129 Decision 9 and its "Explicitly not renamed" list).
- Editing frozen records: archived changes, results, specs, merged plans (including the 18 that carry `**Build profile:**`), and Accepted ADRs (including ADR-0015 and ADR-0086).
- A guard or retired-vocabulary seal for these terms.
- Moving `no-fallback` into the convention's table (ADR-0086).
- Rows owned by the other ADR-0129 families or by change 0469.

## Coordination

- **0474** (family (d)) may also edit the glossary; whichever lands second takes a small text merge.
- **0469** already dropped rows 48–52. Once this lands, "profile" is free, so 0469's proposed "reader profile" does not collide.
