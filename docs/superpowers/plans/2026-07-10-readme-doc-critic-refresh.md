# README doc-critic refresh — Implementation Plan (change 0052)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. **Every task: read this plan's Global Constraints section before starting.**

**Goal:** Rewrite the repo-root `README.md` (accuracy audit + restructure to the spec's target outline + prose pass), against the post-0051 text on `origin/main` @ `d7f4a96`.

**Architecture:** Editorial change, no code. The "test surface" is (a) the existing suite's doc sentinels over `README.md` (inventory below — must stay green; tests are NOT editable in this change) and (b) an accuracy-audit claims checklist that ships in the results doc. Spec: `2026-07-09-readme-doc-critic-refresh-design.md` on the `docket` branch (includes a 2026-07-10 reconcile addendum mapping which critique findings 0051 already resolved).

**Tech Stack:** Markdown, bash test suite (`bash tests/test_*.sh`, ok/NOT OK lines, non-zero exit on failure).

## Global Constraints

- **Files this branch may modify:** `README.md` only, plus the two build artifacts (`docs/superpowers/plans/2026-07-10-readme-doc-critic-refresh.md` — this plan — and `docs/results/2026-07-10-readme-doc-critic-refresh-results.md`). NEVER touch `tests/`, `skills/`, `scripts/`, `agents/`, `.docket.yml`, or any other doc.
- **Concision is a non-goal** (explicit spec decision). Never cut content to shorten; cut only what is wrong or redundant after relocation. Depth may grow where clarity needs it.
- **Verify claims against code, not sibling prose** (LEARNINGS #47): when auditing/writing a behavioral claim, cite the implementing file:line (scripts/*.sh, scripts/*.md contracts, skills/docket-convention/SKILL.md, install.sh, migrate-to-docket.sh, tests). Convention prose may itself have drifted — e.g. `finalize.require_pr_approval` is REAL (implemented in `skills/docket-finalize-change/SKILL.md:33-123`, change 0021/ADR-0011) even though docket-convention's schema omits it. Keep the README's line about it; note the convention's omission in the results doc as a follow-up candidate — do NOT "fix" the convention here (out of scope; LEARNINGS #21: record spec/reality discrepancies in results, never silently re-scope).
- **Spec discrepancies → results doc**, never silent scope changes (LEARNINGS #21).
- **Count guard** (LEARNINGS #14): the README says "eight skills" — if any enumeration or count is touched, grep the README for the old count word and the enumeration before finishing.
- **ADR alignment:** the agent-artifact story rests on ADR-0020 (machine-local generated artifacts; supersedes ADR-0017) + ADR-0015/0016/0019. Audit the tuning section against those.

### Sentinel inventory — every one must be green after every task that edits README.md

Plain greps over the whole README (keep these strings/patterns present somewhere sensible — in their **grammatical** home, never jammed in to pass; LEARNINGS #36):

| Test file:line | Assertion |
|---|---|
| `tests/test_docket_metadata_branch.sh:92` | `grep -q "metadata_branch: docket" README.md` |
| `tests/test_docket_metadata_branch.sh:93` | `grep -q "integration_branch" README.md` |
| `tests/test_docket_metadata_branch.sh:94-95` | `grep -qiE "docket-mode|artifact|lives on" README.md` |
| `tests/test_ensure_claude_settings.sh:109-110` | `grep -qF "scripts/ensure-claude-settings.sh" README.md` |
| `tests/test_results_artifact.sh:45` | `grep -q "results_dir" README.md` |
| `tests/test_sync_agents.sh:794` | `grep -qF ".docket.local.yml" README.md` |
| `tests/test_sync_agents.sh:795-796` | ci `machine-local` AND ci `never committed` |
| `tests/test_sync_agents.sh:797` | `grep -qF "docket:generated" README.md` |
| `tests/test_sync_agents.sh:798` | ci `migrat` AND fixed `--cached` |

Section-scoped sentinels. Two awk extractors pull an h2 section (heading line → the line before the next `^## ` h2; `###` subsections stay inside). **Exactly one h2 heading may match each pattern**, or the extractor concatenates two sections and the negative guard below can false-fire:

1. **Agent-tuning section** — h2 matching `/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/` (e.g. `## Tuning agent models & effort`). Inside it (`tests/test_sync_agents.sh:554-576, 785-786`):
   - fixed `~/.config/docket/config.yml`
   - fixed `` `agents:` block in a repo `` (backticks literal — keep the phrasing "the `agents:` block in a repo's committed `.docket.yml`")
   - regex `bash sync-agents\.sh`
   - ci regex `present.*harness` (e.g. "every **present** harness root")
   - fixed `agent_harnesses`
   - fixed `sync-agents.sh --check`
   - fixed `docket-convention` AND ci `agent layer` (point at the convention for the config shape; never restate it)
   - fixed `effort: auto` AND fixed `drops the effort line`
   - ci `both` AND regex `project (level )?win|project-over-user|project wins` (sync-agents writes both passes; project wins)
   - **Negative guard** (`:575-576`): section must NOT match ci `\b(opus|sonnet|haiku|fable)\b.*\b(xhigh|high|medium|low)\b|model:[[:space:]]*(opus|sonnet|haiku|claude-)` — no model/effort literals in this section; config examples with `model:` lines must live in a DIFFERENT h2 section.
2. **Global-config section** — h2 matching `/^##[[:space:]].*[Gg]lobal config/` (title must contain "global config", e.g. `## Configuration — .docket.yml, global config, and machine-local overrides`). Inside it (`tests/test_sync_agents.sh:775-783`):
   - fixed `~/.config/docket/config.yml`
   - ci regex `same schema as .?\.docket\.yml`
   - ci `repo-local > repo-committed > global > built-in`
   - ci `per-repo-only`
   - fixed `agents.yaml.migrated`
   - ci `user-level pass` (agent_harnesses in config.yml scopes the user-level pass only)

Sentinel check command (run after every README edit; all four files must end `exit 0`):

```bash
cd /Users/homer/dev/docket/.worktrees/readme-doc-critic-refresh
for t in test_docket_metadata_branch test_ensure_claude_settings test_results_artifact test_sync_agents; do
  bash "tests/$t.sh" > "/tmp/$t.out" 2>&1; echo "$t: rc=$?"; grep -c "NOT OK" "/tmp/$t.out" || true
done
```

Expected: `rc=0` and `NOT OK` count `0` for all four. (`test_sync_agents.sh` is slow — several minutes of sandboxed generation runs; that is normal.)

---

### Task 1: Accuracy-audit claims checklist (results doc draft)

**Files:**
- Create: `docs/results/2026-07-10-readme-doc-critic-refresh-results.md`
- Read-only: `README.md`, `scripts/*.sh` + `scripts/*.md`, `skills/docket-convention/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `install.sh`, `migrate-to-docket.sh`, `agents/docket-*.md`, `tests/*.sh`

**Interfaces:**
- Produces: the results doc containing a `## Claims audit` table — columns `# | README §/line | Claim | Verdict (correct / fix / cut) | Evidence (file:line or command)`. Task 2 consumes the verdicts; Task 4 finalizes the doc.

- [ ] **Step 1: Seed the results doc** from `docs/results/results-template.md` if it exists (check with `ls docs/results/`), else start with:

```markdown
# Results — 0052 README doc-critic refresh

**Date started:** 2026-07-10
**Change:** 0052 · **Branch:** feat/readme-doc-critic-refresh
**Baseline:** README.md @ origin/main d7f4a96 (336 lines, post-0051)

## Claims audit

| # | README § (line) | Claim | Verdict | Evidence |
|---|---|---|---|---|

## Spec/reality discrepancies

## Deviations from plan

## Follow-up candidates
```

- [ ] **Step 2: Extract every testable claim** from the current `README.md` — every command, path, config key, default value, precedence rule, and described behavior. Walk section by section (lead, What docket is, producer/implementer loop, Workflow engine, Install, Global config, .docket.local.yml, reconcile, docket-mode, Tuning, eight skills, Status). Expect roughly 50–80 rows. Every row gets a verdict verified **against code** with a file:line citation (LEARNINGS #47). Claims to check with extra care:
  - the three `install.sh` primitives and what each writes (`install.sh`, `link-skills.sh`, `sync-agents.sh`, `scripts/ensure-docket-env.sh`)
  - `sync-agents.sh` opt-in triggers, `.gitignore` managed block, `--check` three-leg semantics (`sync-agents.sh`, `tests/test_sync_agents.sh`)
  - `effort: auto` vs omitted key (`sync-agents.sh` — cite the exact line; LEARNINGS #47 says the convention prose drifted on this once)
  - `finalize.require_pr_approval` semantics (`skills/docket-finalize-change/SKILL.md:33-123`; default false, gates auto-detect only, explicit id overrides)
  - migration behaviors (`migrate-to-docket.sh`: $PWD targeting, `--yes`, prune list, `.gitignore` adds, `ensure-claude-settings.sh` grant)
  - the artifact-location table (verify each row against `skills/docket-convention/SKILL.md` Branch model + terminal-publish contract `scripts/terminal-publish.md`)
  - the seven states / lifecycle claims, build-readiness definition (convention)
  - "Status" § Markhaus line — unverifiable from this repo ⇒ verdict `cut` (spec outline item 11: "cut if stale"), soften-or-cut decision recorded
- [ ] **Step 3: Sanity-check coverage** — every h2 of the current README has ≥1 row; every `fix`/`cut` verdict has evidence. Count rows; note the total in the doc.
- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/readme-doc-critic-refresh
git add docs/results/2026-07-10-readme-doc-critic-refresh-results.md
git commit -m "results(0052): claims audit — every README claim verified against code"
```

### Task 2: Rewrite README.md to the target outline

**Files:**
- Modify: `README.md` (full-document restructure)
- Read-only: `docs/results/2026-07-10-readme-doc-critic-refresh-results.md` (Task 1 verdicts)

**Interfaces:**
- Consumes: Task 1's claims-audit verdicts (apply every `fix`, drop every `cut`).
- Produces: the restructured README; Task 3 does a prose-only pass over it.

Target outline (spec, reconciled 2026-07-10). Narrative order *what → why → try it → configure → internals → reference*. Existing text survives where noted — relocate and refine; do not re-derive what 0051 landed:

1. `# docket` — new lead: 2–3 plain sentences (a backlog of markdown change files living in your repo + agent skills that work them; you design changes interactively, an autonomous implementer drains them to PRs, you stay at the merge gate), then 3–4 "what you get" bullets. No undefined jargon — *change*, *board*, *build-ready* get defined at first use in §3.
2. **Table of contents** — one bullet per h2, GitHub anchor links.
3. **How it works** — the producer/implementer loop (current table + CAS/coordination prose survives, refined; current "What docket is" positioning paragraphs fold into §4) + a compact change-lifecycle glance: one file ≈ one PR; `proposed → in-progress → implemented → done` (+ `blocked/deferred/killed`) in one line or a small diagram. The merge-gate caveat paragraph survives here.
4. **Why docket** — positioning (superpowers = excellent execution without a persistent backlog; OpenSpec = CLI + rigid contract; docket = thin markdown lifecycle in between) merged with the **reconcile pitch promoted here** — current "The reconcile superpower" content survives largely intact (problem/what-docket-does/stance structure), tightened, as the differentiator.
5. **Install** — new *Prerequisites* subsection (Claude Code CLI; `git` + `gh`; superpowers plugin recommended-not-required with the degrade note; a GitHub remote for PR flow), then the three primitives (current post-0051 bullets survive, refined), the `data lives per consuming project` note, pointer to `migrate-to-docket.sh` for existing repos.
6. **Quickstart: the daily loop** (new) — the concrete session flow with actual invocations: `docket-new-change` (propose/brainstorm) → `docket-implement-next` (autonomous drain to PR) → you review/merge → `docket-finalize-change` (close-out) — plus where `docket-groom-next`/`docket-auto-groom` (stubs → build-ready) and `docket-status` (board refresh/sweep) fit. Frame as what you type in a Claude Code session in a docket-enabled repo.
7. **Configuration — `.docket.yml`, global config, and machine-local overrides** (h2 title MUST contain "global config"; this is the `[Gg]lobal config` sentinel section) — consolidates the current three config blocks: the annotated `.docket.yml` example (keep complete, keep `metadata_branch: docket` + `integration_branch` + `results_dir` + `require_pr_approval` lines), the four-layer per-key precedence (`repo-local > repo-committed > global > built-in`), `~/.config/docket/config.yml` (same schema as `.docket.yml`; example block survives), `.docket.local.yml` (survives), the coordination-key fence (`per-repo-only`), misplacement/malformed-file behavior, `agents.yaml.migrated` migration note, `agent_harnesses`-scopes-the-`user-level pass` note. All sentinel strings for `gsec` land INSIDE this one h2.
8. **docket-mode: where metadata lives** — current section survives (two-branch model, artifact table, `integration_branch`/GitFlow, `.docket/` worktree rationale, finalize→selective publish, migration incl. the literal `scripts/ensure-claude-settings.sh` path, `main`-mode opt-out), tightened, jargon introduced in order.
9. **Tuning agent models & effort** (h2 title must match `[Aa]gent.*[Mm]odel` — and be the ONLY such h2) — current post-0051 section survives, refined per audit; keep every `sec` sentinel string listed in Global Constraints and keep the negative guard clean (no `model:` examples in this section — those live in §7).
10. **The eight skills** — current table survives near the end, rows refined per audit.
11. **Status** — rewrite to verified claims only; drop/soften the Markhaus line per Task 1's verdict.

- [ ] **Step 1: Draft the full rewrite** in place in `README.md` following the outline above, applying every Task 1 `fix`/`cut` verdict. Keep the `---` horizontal-rule section separators style. Preserve all whole-README sentinel strings in their natural homes (`.docket.yml` block keeps `metadata_branch: docket`, `integration_branch`, `results_dir`; migration prose keeps `scripts/ensure-claude-settings.sh`; agent story keeps `machine-local`, `never committed`, `docket:generated`, migration + `--cached`; `.docket.local.yml` appears in §7).
- [ ] **Step 2: Structural self-check** — run:

```bash
cd /Users/homer/dev/docket/.worktrees/readme-doc-critic-refresh
grep -n "^## " README.md
awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{c++} END{print "agent-h2:", c}' README.md
awk '/^##[[:space:]].*[Gg]lobal config/{c++} END{print "gconf-h2:", c}' README.md
```

Expected: the outline's h2 list in order; `agent-h2: 1`; `gconf-h2: 1`. Verify every TOC anchor matches an actual heading.

- [ ] **Step 3: Run the sentinel check command** (Global Constraints). Expected: 4× `rc=0`, zero `NOT OK`.
- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(0052): restructure README — lead, TOC, quickstart, consolidated config; audit fixes applied"
```

### Task 3: Prose pass (humanizer standards)

**Files:**
- Modify: `README.md` (prose only — no section adds/removes/reorders, no heading changes)

**Interfaces:**
- Consumes: Task 2's restructured README.
- Produces: final prose; Task 4 verifies and closes out.

- [ ] **Step 1: Apply the humanizer skill's standards** to the rewritten text. If the `humanizer` skill is invocable, invoke it on `README.md`; otherwise apply its checklist manually: remove inflated/promotional language, vague attributions, filler phrases, negative parallelisms ("not X but Y" chains), rule-of-three padding, em-dash overuse, superficial "-ing" analyses; prefer active voice and concrete statements. Constraint: do NOT reword any sentinel string out of existence (re-check the Global Constraints inventory; e.g. "machine-local", "never committed", "drops the effort line" must survive verbatim).
- [ ] **Step 2: Re-run the structural self-check and sentinel check** (Task 2 Steps 2–3 commands). Expected: same green results.
- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(0052): prose pass — humanizer standards over the rewritten README"
```

### Task 4: Full-suite verification + results close-out

**Files:**
- Modify: `docs/results/2026-07-10-readme-doc-critic-refresh-results.md`

**Interfaces:**
- Consumes: everything prior.
- Produces: the final results doc — the change's verification surface (there is no code).

- [ ] **Step 1: Run the FULL test suite** (not just the sentinel files):

```bash
cd /Users/homer/dev/docket/.worktrees/readme-doc-critic-refresh
fail=0; for t in tests/test_*.sh; do bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
grep -l "NOT OK" /tmp/test_*.out || echo "no NOT OK anywhere"
```

Expected: `suite fail=0`, `no NOT OK anywhere`. If a test fails, diagnose: if the failure is a README sentinel, fix README (keep tests untouched); if it fails identically on the base commit `d7f4a96` (`git stash` + rerun to confirm), record it in the results doc as pre-existing and move on.
- [ ] **Step 2: Verify the claims checklist end-state** — every row's verdict is reflected in the final README (`fix` applied, `cut` absent, `correct` present or consciously relocated). Update rows if the rewrite changed a claim's wording; every behavioral claim in the FINAL README must trace to a checklist row.
- [ ] **Step 3: Finalize the results doc** — fill `## Spec/reality discrepancies` (minimum: docket-convention's `.docket.yml` schema omits `finalize.require_pr_approval`, README documents it, code implements it — follow-up candidate to fix the convention), `## Deviations from plan`, `## Follow-up candidates`; add a `## Merge-gate checks for the human` section: 2–3 manual spot-checks (e.g. render the README on GitHub and click every TOC anchor; skim the Quickstart as a newcomer).
- [ ] **Step 4: Count guard** — `grep -in "eight\|seven\|six\b" README.md`; verify each hit's count matches the enumerated set it describes (eight skills, seven states, …).
- [ ] **Step 5: Commit**

```bash
git add docs/results/2026-07-10-readme-doc-critic-refresh-results.md README.md
git commit -m "results(0052): full-suite green; claims checklist finalized"
```

(`README.md` in the add is for any Step-2 reconciliation edits; if none, the pathspec is still safe — it is tracked.)

## Self-review notes

- Spec coverage: audit (Task 1) → Build method step 1; rewrite/outline items 1–11 (Task 2) → Build method step 2; prose (Task 3) → step 3; verification/results (Task 4) → step 4. Reconcile addendum constraints (ADR-0020 alignment, require_pr_approval keep, Markhaus cut/soften, 0053 concurrency note) are embedded in Tasks 1–2.
- The suite-sentinel inventory was derived by grepping `tests/` for README assertions on 2026-07-10 (files: test_docket_metadata_branch, test_ensure_claude_settings, test_results_artifact, test_sync_agents); Task 4's full-suite run backstops any missed sentinel.
- Concurrent change 0053 edits skill bodies only; if it merges mid-build, the finalize rebase gate re-runs the suite — no action here beyond audit-time re-verification of convention section names referenced by README ("Agent layer", "Skill layer").
