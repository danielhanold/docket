<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0402 — Restructure the technical docs into goal-organised guide, concepts, and reference tiers](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0402-restructure-the-technical-docs-into-goal-organised-guide-con.md)**
<!-- docket:backlink:end -->
# Restructure the technical docs into guide, concepts, and reference tiers — Implementation Plan

> **For agentic workers:** This plan is executed by the `docket-build` skill (profile-routed
> workers, one task per worker, checkbox steps for tracking). Each task is one worker's whole
> assignment.

**Goal:** Replace the mechanism-organised 1068-line `docs/guide/README.md` and the three harness
directories with three question-sorted tiers — `docs/guide/` (how do I), `docs/concepts/` (what is
it and why), `docs/reference/` (exact fields and owners) — entered from a new `docs/README.md`,
with every repoguard prose-contract row repointed in the same commit as the move it survives.

**Architecture:** Pure docs restructure plus one Go test-table edit. Content flows one way: the
relocated body and the harness setup prose are rewritten page-by-page for the `dummy_mode` persona
(never moved verbatim), the harness runbooks/examples/fixtures are `git mv`'d with links rewritten,
and a committed coverage table accounts for every heading and file. The old body is deleted last,
after every phrase it carried has a new home, so the repoguard suite can stay green at every
commit.

**Tech Stack:** Markdown; Go test table (`internal/repoguard/prose_contracts_test.go`); bash for a
one-off link-resolution check; `go run ./cmd/docket development test` as the gate.

**Spec:** `docs/superpowers/specs/2026-09-03-restructure-the-technical-docs-into-goal-organised-guide-con-design.md`
(on the `docket` metadata branch; readable at
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-03-restructure-the-technical-docs-into-goal-organised-guide-con-design.md`).
The spec's glossary and voice checklist are restated below so no task needs the metadata worktree.

## Global Constraints

Every task implicitly includes all of these.

1. **Reader and voice.** Every new page is written for the shipped default `dummy_mode` persona: a
   mid-level engineer who knows architecture and is told every docket-internal term with a gloss
   on first use. Gloss on **first use per page** (a page is a unit; a gloss on one page does not
   cover another), using the glossary below **verbatim**; later uses on the same page are bare.
   Concrete consequence over abstraction ("the second save loses its work and retries", not "a
   compare-and-swap conflict"). No harness product names in normative prose outside the harness
   guide page and the harness reference. Headings are tasks or nouns, never mechanism names
   ("Reclaim a stale claim", not "`reclaim-claims.sh`").
2. **Never drop content.** Never drop a decision, a caveat, a config key, or an option to make
   prose simpler. Simplification is vocabulary and framing only. The coverage table (Task 14) is
   the audit trail; write your page's source sections fully before condensing.
3. **ADR-0054.** No line-number cross-references anywhere in maintained source (docs included).
   Anchor on a heading, a symbol name, or a verbatim-quoted clause.
4. **Repoguard same-commit rule.** Every `internal/repoguard/prose_contracts_test.go` row whose
   `file` this change moves or deletes is repointed **in the same commit** as that move/delete.
   A row is never deleted. Where a guarded phrase is re-voiced, the row's phrase is updated to
   the new wording in the same commit. After any commit that touches guarded files, run
   `go test ./internal/repoguard/ -count=1` and require PASS before committing… i.e. run it as
   the pre-commit verification step of that task.
5. **Point-in-time records are untouched.** Archived changes, results files, specs, old plans,
   Accepted ADRs, and `docs/release/` keep their old links even where those links now dangle.
   Only maintained source is retargeted. The whole-repo referrer scan already ran at plan time:
   the only maintained-source files linking into a removed path are `README.md`,
   `internal/repoguard/prose_contracts_test.go`, `docs/guide/README.md` (itself removed),
   `docs/cursor/validation.md` and `docs/codex/validation-runbook.md` (both moved). No skill,
   agent, `cursor-rules/`, `tests/`, `scripts/`, `internal/`, `.docket.example.yml`, `AGENTS.md`,
   or `docs/comparison/` file links into a removed path. If your task's edits create a new
   referrer, retarget it yourself.
6. **`docs/guide/README.md` disposition (interpretation, settled here).** The spec both removes
   `docs/guide/README.md` (the relocated 1068-line body) and gives the guide tier "a short
   `README.md` index". Resolution: the relocated **body** is deleted in Task 12 and the same path
   is rewritten as the short guide index. "Removed" in the acceptance list means the body no
   longer exists anywhere; the path survives as an index, symmetric with the concepts and
   reference tier indexes.
7. **Link style inside the tiers.** Relative links only (e.g. from a guide page:
   `../concepts/run-gate.md`, `../reference/harness/validation.md`). Section anchors use GitHub
   slugging (lowercase, spaces→`-`, punctuation dropped).
8. **Commits.** One commit per task unless a task says otherwise, message prefix `docs(0402):`
   (the repoguard edit tasks use `docs+guard(0402):`). Stage only the paths your task names —
   never `git add -A` (the metadata worktree discipline applies even on the feature branch).
9. **Working directory.** All paths are relative to the feature worktree root
   `/Users/homer/dev/docket/.worktrees/restructure-the-technical-docs-into-goal-organised-guide-con`.
   Run all git commands with `-C` to that path or from inside it.
10. **Wrapped-prose greps.** The relocated body and new pages are hard-wrapped. Any verification
    grep for a multi-word phrase must run over a whitespace-collapsed copy:
    `tr -s '[:space:]' ' ' < FILE | grep -cF "PHRASE"` (learnings: phrase-grep-over-wrapped-prose).

### Glossary — approved one-clause glosses (use verbatim on first use per page)

A page needing a term not listed does not edit the spec (a point-in-time record); instead the
worker adds the new term and gloss to the coverage table's "Glossary extensions" section (Task 14)
and uses it consistently across pages.

| Term | Gloss |
|---|---|
| change | one unit of planned work, roughly one pull request, tracked as one markdown file |
| board | the generated overview of every change and its state, never edited by hand |
| metadata branch | the `docket` git branch where the backlog, specs, and decisions are stored, separate from the code |
| integration branch | the branch code lands on, usually `main` |
| metadata worktree | a second checkout of the repo at `.docket/`, parked on the metadata branch, so backlog edits never touch your code checkout |
| claim | the moment a change is picked up for building; it records which branch will carry the work and when it was taken |
| claim lease | a timestamp on a claim; when it expires with no branch behind it, the change goes back to the queue |
| reconcile | a check at build time that the change is still worth doing and its assumptions still hold, before any code is written |
| build-ready | a proposed change that has a spec or is marked trivial and whose dependencies are all merged |
| needs-brainstorm | a proposed change with neither a spec nor a trivial mark; it needs a design conversation first |
| spec | the design document a change links to, written before building |
| plan | the task-by-task breakdown a build follows, written on the feature branch |
| results | the optional close-out record of what a build actually did |
| ADR | an architecture decision record: one file per decision, immutable once accepted |
| learnings | the loop's memory of lessons from past builds, curated by a human |
| skill | a named, reusable instruction set an agent loads for one job |
| agent | a separately launched worker with its own context, pinned to a model and effort |
| harness | the tool that runs the agent: Claude Code, Cursor, Codex, or opencode |
| dispatch | launching a named agent to do a step and waiting for it to return |
| build profile | one of four worker tiers (economy, standard, premium, max) a plan task is routed to by risk |
| build gate | the full test-suite run at the end of a build that must be green before review |
| build evidence | the committed record of that gate run, read by the reviewer |
| run gate | the bookkeeping around a launched build run: who launched it, whether it finished, whether it may be retried |
| finalize | the close-out sequence: rebase onto the integration branch, retest, merge, archive |
| stacked change | a change built on another change's unmerged branch rather than on the integration branch |
| coordination key | a config key whose value must be identical for every clone, so it may only be set in the committed repo config |
| capability catalog | the machine-readable list of every operation the `docket` binary offers, which skills read instead of hard-coding commands |
| disposition | the one-word outcome an operation reports: applied, no-op, refused, or error |
| health check | a status-time scan for things a human should look at: stale claims, broken links, stalled dependencies |

### The source body's section map (for orientation; the file is `docs/guide/README.md`)

`## Table of contents` · `## How it works` (+ `### The change lifecycle`) · `## Why docket`
(+ `### The reconcile superpower`) · `## Install` (+ Prerequisites, steps 1–2) · `## Updating
docket` · `## Quickstart: the daily loop` (+ two `/loop` subsections) · `## Configuration`
(+ `.docket.yml`, Reclaiming stale claims, Capturing discovered work + taxonomy + migrating,
Speaking your language + persona gallery, Workflow roles, global config, `.docket.local.yml`,
coordination keys, misplaced/malformed, migrating from `agents.yaml`) · `## docket-mode: where
metadata lives` (+ two-branch model, artifact map, GitFlow, metadata worktree, finalize→selective
publish, terminal publish, `main`-mode, git-hook frameworks) · `## Tuning agent models & effort`
· `## Skills` · `## Learnings — the loop's memory` · `## Customization` (+ consultant brainstorm,
docket-build, docket-review, runner delegation, Cursor Auto-run, hands-off finalize) · `## Status`
· `## Migration` (+ docket-mode migration, pre-0051 migration).

---

### Task 1: Guide pages — capturing-work.md and designing-before-building.md

**Files:**
- Create: `docs/guide/capturing-work.md`
- Create: `docs/guide/designing-before-building.md`
- Read (source): `docs/guide/README.md`

**Interfaces:**
- Produces: `docs/guide/capturing-work.md` carrying the verbatim phrase
  `untyped set can only shrink`; `docs/guide/designing-before-building.md` carrying the verbatim
  phrase `brainstorm: docket-brainstorm` (inside its skills-map YAML example). Task 12 repoints
  repoguard rows at these files and these exact phrases.

- [ ] **Step 1: Write `docs/guide/capturing-work.md`** — title: `# Capturing work that outlives
  the session`. One-paragraph opener (what the reader will be able to do), then task-shaped
  sections. Absorb and rewrite for the persona these source sections of `docs/guide/README.md`:
  *The change lifecycle* (under How it works), *Capturing discovered work (`auto_capture`) and
  typing it (`change_types`)* including *The taxonomy (`change_types`)* and *Migrating to typed
  changes*. Cover: the change file and its manifest fields as a user meets them; the lifecycle
  states and the board; priorities, types, dependencies and `depends_on`; stacked changes; scan
  mode; the `trivial` mark; capturing discovered work mid-build. The sentence carrying the typed-
  changes guarantee must keep the exact words `untyped set can only shrink`.
- [ ] **Step 2: Write `docs/guide/designing-before-building.md`** — title: `# Designing before
  building`. Absorb: *Quickstart* step 2 (the groom step), *Consultant-authored brainstorm
  (opt-in)*, *Speaking your language (`dummy_mode`)* including the persona gallery. Cover:
  interactive grooming with `docket-groom-next`; autonomous grooming and the adversarial critic;
  consultant-authored specs and the `skills:` binding that opts in — keep the YAML example line
  exactly as `brainstorm: docket-brainstorm`; dummy mode shaping the design conversation.
- [ ] **Step 3: Verify phrases and glossary discipline**
  ```bash
  tr -s '[:space:]' ' ' < docs/guide/capturing-work.md | grep -cF "untyped set can only shrink"
  grep -cF "brainstorm: docket-brainstorm" docs/guide/designing-before-building.md
  ```
  Expected: `1` (or more) from each. Re-read each page top to bottom once, checking every glossary
  term's first use carries its gloss.
- [ ] **Step 4: Commit**
  ```bash
  git add docs/guide/capturing-work.md docs/guide/designing-before-building.md
  git commit -m "docs(0402): guide pages — capturing work, designing before building"
  ```

### Task 2: Guide pages — building-without-supervision.md and proving-the-build.md

**Files:**
- Create: `docs/guide/building-without-supervision.md`
- Create: `docs/guide/proving-the-build.md`
- Read (source): `docs/guide/README.md`

**Interfaces:**
- Produces: the two files; no repoguard phrase lands here, but the guide index (Task 12) and
  docs index (Task 13) link both paths verbatim.

- [ ] **Step 1: Write `docs/guide/building-without-supervision.md`** — title: `# Building without
  supervision`. Absorb: *Why docket*, *The reconcile superpower*, *docket-build — the lean,
  profile-routed build* (the routing/escalation half; the gate half goes to proving-the-build),
  *Draining hands-free with `/loop`*. Cover: what an implement-next run does end to end (pick,
  claim, reconcile, plan, build, review, stop at the human merge gate); reconcile as the step
  that kills stale work before code is written; plan authoring on the feature branch; the four
  build profiles and the one bounded escalation; worktrees and claims (and the claim lease);
  draining the queue with `/loop`; dispositions and halts a run can end in. Link
  `../concepts/run-gate.md` and `../concepts/build-profiles-and-gate.md` where the mechanism
  runs deeper than a how-to needs.
- [ ] **Step 2: Write `docs/guide/proving-the-build.md`** — title: `# Proving the build`. Absorb:
  the build-gate paragraphs of *docket-build* and the `build:` / `finalize:` config keys
  (test-command resolution). Cover: the build gate and its committed build evidence; the gate
  driver and wall-clock budgets (BUDGET WATCH / SERIAL CONFIRMED OVER BUDGET meanings); how
  integration repair re-greens the suite after finalize's rebase; configuring the suite command
  (`build.test_command`, `finalize.test_command`) and that both resolve from config, never a
  second copy.
- [ ] **Step 3: Verify** — re-read both pages against the voice checklist; then:
  ```bash
  grep -c '^# ' docs/guide/building-without-supervision.md docs/guide/proving-the-build.md
  ```
  Expected: `1` per file.
- [ ] **Step 4: Commit**
  ```bash
  git add docs/guide/building-without-supervision.md docs/guide/proving-the-build.md
  git commit -m "docs(0402): guide pages — building without supervision, proving the build"
  ```

### Task 3: Guide pages — reviewing-before-the-human.md and landing-changes.md

**Files:**
- Create: `docs/guide/reviewing-before-the-human.md`
- Create: `docs/guide/landing-changes.md`
- Read (source): `docs/guide/README.md`

**Interfaces:**
- Produces: `docs/guide/landing-changes.md` carrying the verbatim phrase `auto-mode classifier`
  (Task 12 repoints a `test_readme_finalize_docs` row at it).

- [ ] **Step 1: Write `docs/guide/reviewing-before-the-human.md`** — title: `# Reviewing before
  the human does`. Absorb: *docket-review — the bounded whole-branch reviewer*. Cover: the
  reviewer contract and its rungs (lean/standard/deep); read-only, never fixes, never runs the
  suite; the fix loop and the disposition table; why the suite runs in the build gate before
  review, not inside it.
- [ ] **Step 2: Write `docs/guide/landing-changes.md`** — title: `# Landing changes safely`.
  Absorb: *Closing out hands-free with `/loop`*, *Hands-off finalize — what blocks it, and the
  recipe that works*, *Finalize → selective publish*. Cover: finalize end to end (rebase, retest,
  merge, archive, board refresh); blocked states and identity repair; branch protection and the
  zero-approvals recipe; the Claude Code auto-mode classifier paragraph — keep the exact words
  `auto-mode classifier`; closing out by naming ids with `/loop`.
- [ ] **Step 3: Verify**
  ```bash
  tr -s '[:space:]' ' ' < docs/guide/landing-changes.md | grep -cF "auto-mode classifier"
  ```
  Expected: `1` or more.
- [ ] **Step 4: Commit**
  ```bash
  git add docs/guide/reviewing-before-the-human.md docs/guide/landing-changes.md
  git commit -m "docs(0402): guide pages — reviewing before the human, landing changes"
  ```

### Task 4: Guide pages — keeping-the-backlog-honest.md, remembering-why.md, daily-loop.md

**Files:**
- Create: `docs/guide/keeping-the-backlog-honest.md`
- Create: `docs/guide/remembering-why.md`
- Create: `docs/guide/daily-loop.md`
- Read (source): `docs/guide/README.md`

**Interfaces:**
- Produces: the three files, linked verbatim by Tasks 12–13. `daily-loop.md` stays standalone
  (spec's open question: keep it standalone unless the guide index would otherwise be under
  thirty lines — it will not be).

- [ ] **Step 1: Write `docs/guide/keeping-the-backlog-honest.md`** — title: `# Keeping the
  backlog honest`. Absorb: *Reclaiming stale claims (`reclaim`)*, *Status*. Cover: status versus
  the terminal sweep; each health code and what to do about it; reclaiming stale claims and the
  claim lease; recovering a halted run.
- [ ] **Step 2: Write `docs/guide/remembering-why.md`** — title: `# Remembering why`. Absorb:
  *Learnings — the loop's memory*, and the `docket-adr` portion of *Skills*. Cover: ADRs (what
  gets one, immutability, supersede/reverse); the learnings ledger, findings vs the index,
  promotion as human-gated; what stays a war story vs what graduates.
- [ ] **Step 3: Write `docs/guide/daily-loop.md`** — title: `# The daily loop`. Absorb:
  *Quickstart: the daily loop*. One screen long, the day's cycle (capture → groom → build →
  merge → close out), each step linking into the page above that owns it
  (`capturing-work.md`, `designing-before-building.md`, `building-without-supervision.md`,
  `landing-changes.md`, `keeping-the-backlog-honest.md`).
- [ ] **Step 4: Verify** — `wc -l docs/guide/daily-loop.md` — expected: under ~60 lines. Re-read
  all three against the voice checklist.
- [ ] **Step 5: Commit**
  ```bash
  git add docs/guide/keeping-the-backlog-honest.md docs/guide/remembering-why.md docs/guide/daily-loop.md
  git commit -m "docs(0402): guide pages — backlog honesty, remembering why, daily loop"
  ```

### Task 5: Guide pages — governing-through-configuration.md and where-the-metadata-lives.md

**Files:**
- Create: `docs/guide/governing-through-configuration.md`
- Create: `docs/guide/where-the-metadata-lives.md`
- Read (source): `docs/guide/README.md`

**Interfaces:**
- Produces: the two files. These two absorb the largest source share; every config key and every
  docket-mode subsection must land here or on a page an earlier task owns (coverage table is the
  audit).

- [ ] **Step 1: Write `docs/guide/governing-through-configuration.md`** — title: `# Governing
  through configuration`. Absorb: all *Configuration* subsections not claimed by other pages —
  `.docket.yml` per-repo settings, *Workflow roles — the `skills:` map*, global config
  (`~/.config/docket/config.yml`), `.docket.local.yml`, *Coordination keys are per-repo-only*,
  *When a file is misplaced or malformed*, *Migrating from `agents.yaml`*. (Already claimed
  elsewhere: `reclaim` → Task 4; `auto_capture`/`change_types` → Task 1; `dummy_mode` → Task 1's
  second page; `build:`/`finalize:` test commands → Task 2.) Cover the four layers and their
  precedence, the coordination fence, and every remaining config block by purpose — name each
  key, and point at `.docket.example.yml` for its exact shape rather than copying it.
- [ ] **Step 2: Write `docs/guide/where-the-metadata-lives.md`** — title: `# Where the metadata
  lives`. Absorb: *docket-mode: where metadata lives* and every subsection not claimed by
  landing-changes (which took *Finalize → selective publish*): the two-branch model, where each
  artifact lives, `integration_branch` and GitFlow, the `.docket/` metadata worktree, terminal
  publish (`terminal_publish`, opt-in), `main`-mode, git-hook frameworks; plus *Migration* (both
  subsections). Link `../concepts/two-branches.md` for the why.
- [ ] **Step 3: Verify** — list every `###`/`####` heading of the source *Configuration* and
  *docket-mode* sections and check each is covered by one of the pages written so far; note any
  gap in the page (fix, don't defer):
  ```bash
  awk '/^## Configuration/,/^## docket-mode/' docs/guide/README.md | grep '^###'
  awk '/^## docket-mode/,/^## Tuning/' docs/guide/README.md | grep '^###'
  ```
- [ ] **Step 4: Commit**
  ```bash
  git add docs/guide/governing-through-configuration.md docs/guide/where-the-metadata-lives.md
  git commit -m "docs(0402): guide pages — configuration, where the metadata lives"
  ```

### Task 6: Guide page — running-on-your-harness.md (absorbs harness setup prose, carries the 385 correction)

**Files:**
- Create: `docs/guide/running-on-your-harness.md`
- Read (source): `docs/guide/README.md`, `docs/cursor/permissions.md`, `docs/codex/setup.md`,
  `docs/opencode/setup.md`

**Interfaces:**
- Produces: `docs/guide/running-on-your-harness.md` carrying verbatim: `Fork-exclusion principle`,
  `completed (forked execution)`, the filename `permissions.example.json`, and a relative link
  whose literal text includes `](../reference/harness/`. Tasks 7 and 12 repoint repoguard rows at
  those exact strings. The harness files it links move in Task 7 — write the links at their
  **post-move** paths (`../reference/harness/validation.md`, `../reference/harness/validation-runbook.md`,
  `../reference/harness/permissions.example.json`, `../reference/harness/sandbox.example.json`);
  they dangle for one task and resolve when Task 7 lands.

- [ ] **Step 1: Write the page** — title: `# Running on your harness`. Structure: install and
  update first, then model/effort tuning, then one section per harness. Absorb, rewritten for the
  persona:
  - *Install* (+ prerequisites and both steps), *Updating docket* — from the relocated body.
  - *Tuning agent models & effort* — including the two-mechanisms paragraph (keep the exact words
    `Fork-exclusion principle`) and the invocation-paths table (keep the exact words
    `completed (forked execution)`).
  - *Runner delegation — running docket agents on another harness* and *Running under Cursor
    Auto-run* — from the body's Customization section.
  - `## Claude Code` — the fork/dispatch mechanics from *Tuning*.
  - `## Cursor` — setup prose from `docs/cursor/permissions.md` (the allowlist tiers, the
    sandbox/network story, the troubleshooting items), naming `permissions.example.json` and
    `sandbox.example.json` and linking them at `../reference/harness/`. **Change 385
    correction:** every allowlist entry or prose clause that names `scripts/docket.sh`,
    `docket.sh`, or `$DOCKET_SCRIPTS_DIR` is rewritten to the native `docket` binary invocation
    (the binary is on `PATH`; the facade script is retired). Do not reproduce the old JSON
    allowlist lines verbatim — restate them for the binary.
  - `## Codex` — setup prose from `docs/codex/setup.md` (wrapper generation, the committed
    AGENTS.md dispatch block, both entry paths, restart-after-regeneration), linking the live
    runbook at `../reference/harness/validation-runbook.md`.
  - `## opencode` — setup prose from `docs/opencode/setup.md`, same treatment.
  Product names are allowed on this page (it is the harness page).
- [ ] **Step 2: Verify phrases**
  ```bash
  tr -s '[:space:]' ' ' < docs/guide/running-on-your-harness.md > /tmp/rharn.flat 2>/dev/null || true
  f=docs/guide/running-on-your-harness.md
  tr -s '[:space:]' ' ' < "$f" | grep -cF "Fork-exclusion principle"
  tr -s '[:space:]' ' ' < "$f" | grep -cF "completed (forked execution)"
  grep -cF "permissions.example.json" "$f"
  grep -cF "](../reference/harness/" "$f"
  out=$(grep -nF "scripts/docket.sh" "$f" || true); [ -z "$out" ] && echo NO-STALE-FACADE
  ```
  Expected: each count ≥ 1, and `NO-STALE-FACADE`.
- [ ] **Step 3: Commit**
  ```bash
  git add docs/guide/running-on-your-harness.md
  git commit -m "docs(0402): guide page — running on your harness (absorbs cursor/codex/opencode setup; 385 correction)"
  ```

### Task 7: Move harness runbooks to docs/reference/harness/, delete the three harness dirs, repoint their repoguard rows — one commit

**Files:**
- Create: `docs/reference/harness/README.md`
- Move: `docs/cursor/validation.md` → `docs/reference/harness/validation.md`
- Move: `docs/cursor/permissions.example.json` → `docs/reference/harness/permissions.example.json`
- Move: `docs/cursor/sandbox.example.json` → `docs/reference/harness/sandbox.example.json`
- Move: `docs/codex/validation-runbook.md` → `docs/reference/harness/validation-runbook.md`
- Move: `docs/codex/fixtures/` → `docs/reference/harness/fixtures/`
- Delete: `docs/cursor/permissions.md`, `docs/codex/setup.md`, `docs/opencode/setup.md` (their
  prose was absorbed in Task 6), then the emptied `docs/cursor/`, `docs/codex/`, `docs/opencode/`
- Modify: `internal/repoguard/prose_contracts_test.go`

**Interfaces:**
- Consumes: `docs/guide/running-on-your-harness.md` (Task 6) — the repoint destination for the
  permissions rows.
- Produces: `docs/reference/harness/` populated; three repoguard sentinels repointed
  (`test_cursor_contract_docs`, `test_cursor_permissions_docs`, `test_codex_runbook`).

- [ ] **Step 1: git mv the five artifacts**
  ```bash
  mkdir -p docs/reference/harness
  git mv docs/cursor/validation.md docs/reference/harness/validation.md
  git mv docs/cursor/permissions.example.json docs/reference/harness/permissions.example.json
  git mv docs/cursor/sandbox.example.json docs/reference/harness/sandbox.example.json
  git mv docs/codex/validation-runbook.md docs/reference/harness/validation-runbook.md
  git mv docs/codex/fixtures docs/reference/harness/fixtures
  git rm docs/cursor/permissions.md docs/codex/setup.md docs/opencode/setup.md
  ```
- [ ] **Step 2: Rewrite links and stale references inside the two moved runbooks.** Their step
  lists are procedures and stay as written; re-voice only the opening paragraph and headings for
  the persona, and fix references:
  - `docs/reference/harness/validation.md`: the self-reference clause "name this checklist
    (`docs/cursor/validation.md`)" → the new path `docs/reference/harness/validation.md`;
    any link to `permissions.md` → `../../guide/running-on-your-harness.md` (Cursor section).
  - `docs/reference/harness/validation-runbook.md`: references to `docs/codex/setup.md` (three:
    the opening counterpart sentence, the *Restart after (re)generating* pointers) →
    `../../guide/running-on-your-harness.md` (Codex section, quoting its restart heading);
    the `docs/cursor/permissions.md` shape-comparison reference → the harness guide page; the
    fixtures link `](fixtures/nested-launch/README.md)` is relative and moves with the file —
    verify it still resolves; the prose spelling `docs/codex/fixtures/nested-launch/README.md`
    beside it → `docs/reference/harness/fixtures/nested-launch/README.md`. Apply the 385
    correction here too if any step names `scripts/docket.sh` (verify with a grep; expected
    absent already except where the guarded `absent` phrase `scripts/sync-agents.sh` must stay
    absent — do not introduce it).
- [ ] **Step 3: Write `docs/reference/harness/README.md`** — title `# Harness runbooks and
  examples`. An index: one line per file in this directory (the two runbooks, the two example
  JSONs, the fixtures tree), each with its one-line hook and a pointer back to
  `../../guide/running-on-your-harness.md` for setup.
- [ ] **Step 4: Repoint the three sentinels in `internal/repoguard/prose_contracts_test.go`.**
  Replace the three existing row groups (find them by their `sentinel:` strings) with:
  ```go
  // tests/test_cursor_contract_docs.sh — cursor validation merge-gate obligation
  // (moved to the harness reference by change 0402).
  {sentinel: "test_cursor_contract_docs", file: "docs/reference/harness/validation.md",
      present: []string{"## The merge-gate obligation"}},
  // tests/test_cursor_permissions_docs.sh — the permissions guidance survives on the
  // harness guide page, which must link the example JSONs in the harness reference.
  // Change 0402 folded docs/cursor/permissions.md into the guide page, so this
  // sentinel's two rows collapse to one file; both invariants are kept as phrases.
  {sentinel: "test_cursor_permissions_docs", file: "docs/guide/running-on-your-harness.md",
      present: []string{"permissions.example.json", "](../reference/harness/"}},
  // tests/test_codex_runbook.sh — codex runbook slug-derivation + no fabricated path
  // (moved to the harness reference by change 0402).
  {sentinel: "test_codex_runbook", file: "docs/reference/harness/validation-runbook.md",
      present: []string{"codex debug models"}, absent: []string{"scripts/sync-agents.sh"}},
  ```
  Also update the file's doc comment: the clause explaining that "docs/cursor/ and docs/codex/
  are ACTIVE operator documentation" now names `docs/reference/harness/` instead.
- [ ] **Step 5: Verify**
  ```bash
  go test ./internal/repoguard/ -count=1
  git status --porcelain docs/cursor docs/codex docs/opencode
  ```
  Expected: repoguard PASS; the three directories gone from the tree (`git status` shows only
  staged deletes/renames, and `ls docs/cursor docs/codex docs/opencode` errors).
- [ ] **Step 6: Commit (everything above, one commit — the same-commit rule)**
  ```bash
  git add docs/reference/harness internal/repoguard/prose_contracts_test.go
  git commit -m "docs+guard(0402): harness runbooks to docs/reference/harness/; cursor/codex/opencode dirs removed; rows repointed"
  ```

### Task 8: Reference tier — the five pointer pages and the reference index

**Files:**
- Create: `docs/reference/cli.md`, `docs/reference/fields.md`, `docs/reference/config-keys.md`,
  `docs/reference/outcomes.md`, `docs/reference/skills-and-agents.md`, `docs/reference/README.md`

**Interfaces:**
- Consumes: `docs/reference/harness/README.md` (Task 7) — linked from the reference index.
- Produces: `docs/reference/skills-and-agents.md` containing a literal `## Skills` heading and
  **not** containing the string `#the-eight-skills` (Task 12 repoints `test_readme_skill_catalog`
  at it).

A reference page says **where the fact is owned and how to read it** and quotes nothing that can
drift: one line per item — the item, its owner, the command or file that shows the current value.
No copied tables of values.

- [ ] **Step 1: Write the five pages**
  - `cli.md` — `# The CLI by noun and verb`. Owner lines: `docket --help` for the noun list;
    `docket <noun> --help` per verb; `docket capabilities --json` for the capability catalog.
    List the nouns as pointers (one line each: noun → what its verbs govern → the help command),
    never the flags.
  - `fields.md` — `# Change manifest and ADR fields`. Owner: the `docket-convention` skill's
    manifest and ADR sections, cited by heading (quote the heading text, per ADR-0054). One line
    per field group, no field-by-field restatement.
  - `config-keys.md` — `# Config keys`. Owner: `.docket.example.yml` (ADR-0048 makes it the
    shipped reference) and the convention's config section. One line per top-level block
    (`board_surfaces`, `agent_harnesses`, `agents:`, `skills:`, `build:`, `finalize:`,
    `reclaim`, `auto_capture`, `change_types`, `dummy_mode`, `terminal_publish`,
    `integration_branch`, …) — enumerate the blocks from `.docket.example.yml` itself at
    writing time, naming each block's purpose and layer constraints, pointing at the example
    file for shape.
  - `outcomes.md` — `# Dispositions, reason tokens, and health codes`. Owners: the convention's
    disposition vocabulary; `docket status --json` for health codes; the finalize skill's
    failure reference (`skills/docket-finalize-change/references/gate-failure.md`) for reason
    tokens.
  - `skills-and-agents.md` — `# Skills and agents inventory`. Begins its catalog section with the
    literal heading `## Skills`. Owners: the `skills/` and `agents/` directories; the harness's
    own agent registry for availability. One line per skill/agent naming its job — derive the
    list from `ls skills agents` at writing time. Must not contain the string
    `#the-eight-skills`.
- [ ] **Step 2: Write `docs/reference/README.md`** — `# Reference`. One line per page (the five
  above plus `harness/README.md`), each with its hook.
- [ ] **Step 3: Verify**
  ```bash
  grep -c '^## Skills$' docs/reference/skills-and-agents.md
  out=$(grep -RnF '#the-eight-skills' docs/reference || true); [ -z "$out" ] && echo CLEAN
  ```
  Expected: `1`, then `CLEAN`.
- [ ] **Step 4: Commit**
  ```bash
  git add docs/reference/cli.md docs/reference/fields.md docs/reference/config-keys.md \
          docs/reference/outcomes.md docs/reference/skills-and-agents.md docs/reference/README.md
  git commit -m "docs(0402): reference tier — five pointer pages and index"
  ```

### Task 9: Concepts pages — two-branches.md, change-lifecycle.md, skills-agents-dispatch.md

**Files:**
- Create: `docs/concepts/two-branches.md`, `docs/concepts/change-lifecycle.md`,
  `docs/concepts/skills-agents-dispatch.md`
- Read: `docs/adrs/README.md` (to complete the "Decided in" lists)

**Interfaces:**
- Produces: three concepts pages in the fixed four-section shape consumed by the concepts index
  (Task 11) and the acceptance check (every page: four sections in order, non-empty Decided in).

Every concepts page has exactly these four sections, in order:
`## The problem it solves` (2–3 paragraphs, no docket vocabulary until glossed) ·
`## The moving parts` (components and connections; one ASCII or mermaid diagram where a picture
beats prose) · `## The invariants` (bullets a reader could check) · `## Decided in` (bullet list
of ADR links `../adrs/NNNN-….md`, each with one clause saying what that ADR fixed — never
restating its context or consequences). Concepts pages describe **current state only**.

- [ ] **Step 1: Write `two-branches.md`** — `# Two branches and the metadata worktree`. Seed
  Decided-in: ADR-0001, ADR-0002, ADR-0025, ADR-0046; complete the list by scanning
  `docs/adrs/README.md` for branch-model / metadata-worktree / publish-channel decisions and
  adding any Accepted ADR that fixed part of this mechanism.
- [ ] **Step 2: Write `change-lifecycle.md`** — `# The change lifecycle as a state machine`.
  Diagram the states and transitions. Seed Decided-in: ADR-0004, ADR-0005, ADR-0092; complete
  from the index (lifecycle, claims, sweep, reconcile-adjacent state decisions).
- [ ] **Step 3: Write `skills-agents-dispatch.md`** — `# Skills, agents, and harness dispatch`.
  Seed Decided-in: ADR-0008, ADR-0015, ADR-0016, ADR-0018, ADR-0024, ADR-0026, ADR-0044,
  ADR-0064; complete from the index (dispatch, wrappers, fork, harness rows).
- [ ] **Step 4: Verify the shape mechanically**
  ```bash
  for f in docs/concepts/two-branches.md docs/concepts/change-lifecycle.md docs/concepts/skills-agents-dispatch.md; do
    printf '%s: ' "$f"; grep -c '^## ' "$f"
    grep -n '^## ' "$f"
    n=$(grep -A40 '^## Decided in' "$f" | grep -c '](../adrs/'); echo "adr-links=$n"
  done
  ```
  Expected per file: exactly 4 `##` headings, in the order named above, `adr-links` ≥ 1.
- [ ] **Step 5: Commit**
  ```bash
  git add docs/concepts/two-branches.md docs/concepts/change-lifecycle.md docs/concepts/skills-agents-dispatch.md
  git commit -m "docs(0402): concepts — two branches, change lifecycle, skills/agents/dispatch"
  ```

### Task 10: Concepts pages — run-gate.md, config-layers.md, reconcile.md

**Files:**
- Create: `docs/concepts/run-gate.md`, `docs/concepts/config-layers.md`,
  `docs/concepts/reconcile.md`
- Read: `docs/adrs/README.md`, `docs/comparison/ai-native-sdlc-playbook.md`

**Interfaces:**
- Produces: three more four-section concepts pages (same fixed shape as Task 9 — problem /
  moving parts / invariants / Decided in).

- [ ] **Step 1: Write `run-gate.md`** — `# The run gate and attribution`. Seed Decided-in:
  ADR-0074, ADR-0075; complete by scanning `docs/adrs/README.md` for the run-gate ADRs recorded
  under changes #271, #342, and #345 and for gate/attribution decisions generally (candidates
  visible in the index today: ADR-0078, ADR-0080, ADR-0084, and the ADR superseding ADR-0081 —
  verify each against the index before listing; list only Accepted ones, plus the superseding
  ADR where one in the seed chain was superseded).
- [ ] **Step 2: Write `config-layers.md`** — `# Config layers and the coordination fence`. Seed
  Decided-in: ADR-0019, ADR-0048, ADR-0052; complete from the index (config channels,
  precedence, coordination keys).
- [ ] **Step 3: Write `reconcile.md`** — `# Reconcile`. Decided-in: scan `docs/adrs/README.md`
  for reconcile decisions (grep the index for "reconcile"); the page also links the comparison
  brief's reconcile row: cite `../comparison/ai-native-sdlc-playbook.md` and name the row by its
  quoted row label so the reference is greppable (ADR-0054).
- [ ] **Step 4: Verify** — same four-section check as Task 9 Step 4, over these three files.
- [ ] **Step 5: Commit**
  ```bash
  git add docs/concepts/run-gate.md docs/concepts/config-layers.md docs/concepts/reconcile.md
  git commit -m "docs(0402): concepts — run gate, config layers, reconcile"
  ```

### Task 11: Concepts pages — build-profiles-and-gate.md, finalize-sequencer.md, memory.md, and the concepts index

**Files:**
- Create: `docs/concepts/build-profiles-and-gate.md`, `docs/concepts/finalize-sequencer.md`,
  `docs/concepts/memory.md`, `docs/concepts/README.md`
- Read: `docs/adrs/README.md`

**Interfaces:**
- Consumes: the six concepts pages from Tasks 9–10 (the index links all nine).
- Produces: the completed concepts tier.

- [ ] **Step 1: Write `build-profiles-and-gate.md`** — `# Build profiles and the test gate`.
  Seed Decided-in: ADR-0023, ADR-0074; complete from the index (profile ladder, gate verdict,
  suite-runner decisions — ADR-0066 is a visible candidate; verify in the index).
- [ ] **Step 2: Write `finalize-sequencer.md`** — `# Finalize as a sequencer`. Seed Decided-in:
  ADR-0010, ADR-0011, ADR-0042; complete from the index (note ADR-0042 was reversed — link the
  reversing ADR in the same list with its one-clause note; the page body describes only current
  state).
- [ ] **Step 3: Write `memory.md`** — `# Learnings and ADRs as memory`. Seed Decided-in:
  ADR-0041; complete from the index (learnings-ledger, promotion, ADR-format decisions).
- [ ] **Step 4: Write `docs/concepts/README.md`** — `# Concepts`. One line per page (all nine),
  each with its hook, ordered from two-branches outward.
- [ ] **Step 5: Verify** — four-section check (Task 9 Step 4 loop) over the three new pages;
  then confirm the index links nine pages:
  ```bash
  grep -c '](\./\|](' docs/concepts/README.md
  ls docs/concepts/*.md | wc -l
  ```
  Expected: 10 files (nine pages + README); index links ≥ 9.
- [ ] **Step 6: Commit**
  ```bash
  git add docs/concepts/build-profiles-and-gate.md docs/concepts/finalize-sequencer.md \
          docs/concepts/memory.md docs/concepts/README.md
  git commit -m "docs(0402): concepts — build profiles/gate, finalize sequencer, memory, index"
  ```

### Task 12: Replace the relocated body with the guide index; repoint its five repoguard sentinels — one commit

**Files:**
- Rewrite: `docs/guide/README.md` (delete the 1068-line body; write the short guide index)
- Modify: `internal/repoguard/prose_contracts_test.go`

**Interfaces:**
- Consumes: all twelve guide pages (Tasks 1–6), `docs/reference/skills-and-agents.md` (Task 8) —
  every repoint destination must already carry its phrase.
- Produces: the guide tier complete; the relocated body gone.

- [ ] **Step 1: Pre-flight — prove every destination phrase exists** (learnings:
  restatement-accumulates-its-own-guards — the body's phrases have dependents; relocate, never
  restore):
  ```bash
  tr -s '[:space:]' ' ' < docs/guide/designing-before-building.md | grep -cF "brainstorm: docket-brainstorm"
  tr -s '[:space:]' ' ' < docs/guide/running-on-your-harness.md | grep -cF "completed (forked execution)"
  tr -s '[:space:]' ' ' < docs/guide/landing-changes.md | grep -cF "auto-mode classifier"
  tr -s '[:space:]' ' ' < docs/guide/running-on-your-harness.md | grep -cF "Fork-exclusion principle"
  grep -c '^## Skills$' docs/reference/skills-and-agents.md
  tr -s '[:space:]' ' ' < docs/guide/capturing-work.md | grep -cF "untyped set can only shrink"
  ```
  Expected: every count ≥ 1. Any zero: fix that page first (add the phrase where its content
  lives), never proceed.
- [ ] **Step 2: Rewrite `docs/guide/README.md`** as the guide index — `# Guide`. The twelve
  pages in reading order (daily-loop first, then capturing-work, designing-before-building,
  building-without-supervision, proving-the-build, reviewing-before-the-human, landing-changes,
  keeping-the-backlog-honest, remembering-why, governing-through-configuration,
  running-on-your-harness, where-the-metadata-lives), one line and one hook each. Nothing else —
  the body is deleted, not trimmed.
- [ ] **Step 3: Repoint the five sentinels.** In `internal/repoguard/prose_contracts_test.go`,
  replace the five rows whose `file` is `"docs/guide/README.md"` with (keep each row's original
  sentinel comment, adding "(split by change 0402)" where the file changed):
  ```go
  {sentinel: "test_consultant_brainstorm", file: "docs/guide/designing-before-building.md",
      present: []string{"brainstorm: docket-brainstorm"}},
  {sentinel: "test_skill_fork_dispatch", file: "docs/guide/running-on-your-harness.md",
      present: []string{"completed (forked execution)"}},
  // test_readme_finalize_docs guarded two phrases that now live on two pages —
  // one row per (sentinel, file), neither phrase dropped.
  {sentinel: "test_readme_finalize_docs", file: "docs/guide/landing-changes.md",
      present: []string{"auto-mode classifier"}},
  {sentinel: "test_readme_finalize_docs", file: "docs/guide/running-on-your-harness.md",
      present: []string{"Fork-exclusion principle"}},
  {sentinel: "test_readme_skill_catalog", file: "docs/reference/skills-and-agents.md",
      present: []string{"## Skills"}, absent: []string{"#the-eight-skills"}},
  {sentinel: "test_typed_changes_docs", file: "docs/guide/capturing-work.md",
      present: []string{"untyped set can only shrink"}},
  ```
  (Six rows replace five: the `test_readme_finalize_docs` split is deliberate.)
- [ ] **Step 4: Verify**
  ```bash
  go test ./internal/repoguard/ -count=1
  wc -l docs/guide/README.md
  ```
  Expected: PASS; the index well under 100 lines. Then mutation-test one repointed row: remove
  the words `untyped set can only shrink` from `docs/guide/capturing-work.md`, rerun
  `go test ./internal/repoguard/ -count=1`, require FAIL, restore the exact wording by re-adding
  it (do **not** `git checkout` the file — it has uncommitted edits; re-insert the phrase by
  editing), rerun, require PASS.
- [ ] **Step 5: Commit (one commit — the same-commit rule)**
  ```bash
  git add docs/guide/README.md internal/repoguard/prose_contracts_test.go
  git commit -m "docs+guard(0402): relocated body split complete — guide index replaces it; five sentinels repointed"
  ```

### Task 13: docs/README.md, top-level README retarget, and the 0400 landing row — one commit

**Files:**
- Create: `docs/README.md`
- Modify: `README.md` (documentation-map and inline guide links only — no other content;
  change 0400 owns the rest)
- Modify: `internal/repoguard/prose_contracts_test.go` (the `change_0400_readme_landing` row)

**Interfaces:**
- Consumes: all three tier indexes and the guide pages (link targets).
- Produces: the docs entry point; a fully-resolving `README.md`.

- [ ] **Step 1: Write `docs/README.md`** — `# Docket documentation`. Two sentences stating the
  three tiers (guide = how do I, concepts = what is it and why, reference = exact fields and
  owners). Then a **Start here** path of four links: `guide/daily-loop.md` →
  `guide/capturing-work.md` → `guide/building-without-supervision.md` →
  `guide/landing-changes.md`. Then the three tiers, every page listed with its one-line hook
  (reuse the hooks from the tier indexes; keep them identical so there is one wording to
  maintain — cite the tier index as the owner if you shorten here).
- [ ] **Step 2: Retarget `README.md`.** The full set of links into removed paths (found by
  plan-time grep; re-verify with `grep -n 'docs/guide\|docs/cursor\|docs/codex\|docs/opencode' README.md`):
  - the *Tuning agent models & effort* link → `docs/guide/running-on-your-harness.md` (its
    tuning section anchor);
  - the *Install* link → `docs/guide/running-on-your-harness.md`;
  - the *Migration* link → `docs/guide/where-the-metadata-lives.md`;
  - the *Quickstart* link → `docs/guide/daily-loop.md`;
  - the documentation-map **Technical guide** bullet → link `docs/README.md` **first**, then the
    three tier indexes (`docs/guide/README.md`, `docs/concepts/README.md`,
    `docs/reference/README.md`), rewriting the bullet's description to name the three tiers;
  - the **Harness setup** bullet → `docs/guide/running-on-your-harness.md` and
    `docs/reference/harness/README.md`, dropping the four per-file links.
  Touch nothing else in `README.md`.
- [ ] **Step 3: Repoint the 0400 row.** In `internal/repoguard/prose_contracts_test.go`:
  ```go
  // change 0400 — the goal-first landing page cannot silently lose its two
  // load-bearing map links (the docs index — retargeted from the relocated
  // guide by change 0402 — and the comparison page).
  {sentinel: "change_0400_readme_landing", file: "README.md",
      present: []string{"](docs/README.md)", "](docs/comparison/ai-native-sdlc-playbook.md)"}},
  ```
- [ ] **Step 4: Verify**
  ```bash
  go test ./internal/repoguard/ -count=1
  out=$(grep -n 'docs/cursor\|docs/codex\|docs/opencode' README.md || true); [ -z "$out" ] && echo README-CLEAN
  ```
  Expected: PASS, `README-CLEAN`.
- [ ] **Step 5: Commit**
  ```bash
  git add docs/README.md README.md internal/repoguard/prose_contracts_test.go
  git commit -m "docs+guard(0402): docs index; README map retargeted to it; 0400 landing row repointed"
  ```

### Task 14: The coverage table

**Files:**
- Create: `docs/superpowers/plans/2026-09-03-docs-coverage.md`

**Interfaces:**
- Consumes: every page written above; the pre-change body and harness files (read them from git
  history: `git show HEAD~N:docs/guide/README.md` etc., or simply from the merge-base:
  `git show $(git merge-base HEAD origin/main):docs/guide/README.md`).

- [ ] **Step 1: Enumerate the sources.** From the merge-base copy of `docs/guide/README.md`,
  list **every** `##`/`###`/`####` heading:
  ```bash
  git show "$(git merge-base HEAD origin/main)":docs/guide/README.md | grep -E '^#{2,4} '
  ```
  Plus every file that lived under `docs/cursor/`, `docs/codex/` (fixtures tree included —
  list files, one row each), `docs/opencode/`:
  ```bash
  git show "$(git merge-base HEAD origin/main)" --name-only -- docs/cursor docs/codex docs/opencode >/dev/null 2>&1 || true
  git ls-tree -r --name-only "$(git merge-base HEAD origin/main)" -- docs/cursor docs/codex docs/opencode
  ```
- [ ] **Step 2: Write the table.** `# Change 0402 — docs coverage table`. One markdown table
  (or one per source), columns: **Source heading/file** | **Now carried by (page § section)** |
  **Notes**. Every heading and file gets a row. A row may say *dropped* only with a reason that
  names why the content was already stale (e.g. it described the retired `scripts/docket.sh`
  facade, corrected under change 385). The table-of-contents heading of the old body maps to the
  new indexes. Add a final section `## Glossary extensions` listing any term+gloss added beyond
  the spec's table (empty is fine — say "none").
- [ ] **Step 3: Verify completeness mechanically.**
  ```bash
  base=$(git merge-base HEAD origin/main)
  nsrc=$(git show "$base":docs/guide/README.md | grep -cE '^#{2,4} ')
  nfiles=$(git ls-tree -r --name-only "$base" -- docs/cursor docs/codex docs/opencode | wc -l | tr -d ' ')
  nrows=$(grep -c '^|' docs/superpowers/plans/2026-09-03-docs-coverage.md)
  echo "headings=$nsrc files=$nfiles table-lines=$nrows"
  ```
  Require: table body rows ≥ headings + files (header/separator lines inflate `nrows`; count and
  reconcile by eye against the two source lists — every listed heading and file appears in
  column 1 exactly once).
- [ ] **Step 4: Commit**
  ```bash
  git add docs/superpowers/plans/2026-09-03-docs-coverage.md
  git commit -m "docs(0402): coverage table — every relocated-body heading and harness file accounted for"
  ```

### Task 15: One-off link-resolution check, full-suite gate

**Files:**
- No new files (the link check is a one-off, run and recorded — not committed as a permanent
  guard, per the spec's "not a permanent guard").

- [ ] **Step 1: Run the link-resolution check** over the maintained docs surface (the three
  tiers, the docs index, the comparison page, and `README.md` — point-in-time trees under
  `docs/changes`, `docs/results`, `docs/superpowers`, `docs/adrs`, `docs/release` are excluded
  by the spec's own records rule):
  ```bash
  set -u
  fail=0
  files="README.md docs/README.md $(find docs/guide docs/concepts docs/reference docs/comparison -name '*.md')"
  for f in $files; do
    links=$(grep -oE '\]\([^)[:space:]]+\)' "$f" | sed -E 's/^\]\(([^)#]*).*\)$/\1/' | sort -u)
    for l in $links; do
      case "$l" in http://*|https://*|mailto:*|"") continue ;; esac
      d=$(dirname "$f")
      if [ ! -e "$d/$l" ] && [ ! -e "$l" ]; then echo "BROKEN: $f -> $l"; fail=1; fi
    done
  done
  [ "$fail" -eq 0 ] && echo LINKS-OK
  ```
  Expected: `LINKS-OK` and no `BROKEN:` lines. Fix any broken link (in the maintained page, not
  by excluding it) and re-run until clean. Commit any fixes as
  `docs(0402): link-resolution fixes from the acceptance-3 check`.
- [ ] **Step 2: Acceptance spot-checks**
  ```bash
  ls docs/README.md docs/guide docs/concepts docs/reference docs/reference/harness
  ls docs/cursor docs/codex docs/opencode 2>&1 | head -3   # expected: No such file or directory ×3
  for f in docs/concepts/*.md; do case "$f" in */README.md) continue;; esac
    h=$(grep '^## ' "$f" | tr '\n' '|'); echo "$f: $h"; done
  ```
  Expected: every concepts page prints exactly
  `## The problem it solves|## The moving parts|## The invariants|## Decided in|`.
- [ ] **Step 3: Run the full suite (the build gate).**
  Run: `go run ./cmd/docket development test` (drive it per the docket-build gate contract —
  inline blocking `docket gate drive advance` slices, never backgrounded-and-yield).
  Expected: green; act on any `SERIAL CONFIRMED OVER BUDGET:` line; a `BUDGET WATCH:` line is a
  screening finding to record, not a failure.
- [ ] **Step 4: Record the one-off check's output** (the `LINKS-OK` line and the suite summary)
  in the build evidence per the docket-build contract. No commit unless Step 1 fixed links.

---

## Self-review notes (spec → plan)

- Spec §Tier 1 (twelve pages + index): Tasks 1–6 write the twelve, Task 12 the index. The
  spec's source-section column is distributed verbatim into the tasks; the coverage table
  (Task 14) is the completeness audit, per the spec's own designation of it as authoritative.
- Spec §Tier 2 (nine pages, four-section shape, seed ADR lists): Tasks 9–11, with the
  four-section shape restated in Task 9 and checked mechanically in each task and again in
  Task 15 (acceptance 4).
- Spec §Tier 3 (pointer pages, no copied values): Tasks 7–8 (acceptance 5 is each page's own
  "owner per item, no value tables" rule, restated in Task 8's preamble).
- Spec §docs index + README map: Task 13 (acceptance 3's README half; the 0400 row repoint
  matches the change file's reconcile log exactly).
- Spec §Guards, links, records: repoguard rows in Tasks 7, 12, 13 — every original row survives
  (one deliberate two-rows→one-file collapse and one one-row→two-files split, both commented in
  the table); link retargeting is closed by the plan-time referrer scan (Global Constraint 5);
  point-in-time records untouched.
- Spec §Coverage table: Task 14; §Acceptance 3 one-off check: Task 15; §Acceptance 6 (glossary
  first-use) is reviewer-verified by reading — the voice rules and verbatim glossary ride in
  Global Constraints so every worker applies them.
- 385 correction: Task 6 (guide page) and Task 7 Step 2 (moved runbooks).
- Open questions: daily-loop stays standalone (Task 4); ADR seed-list completion is in Tasks
  9–11 with the index-scan instructions.
