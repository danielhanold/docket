# discovered-from provenance links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one optional manifest field, `discovered_from:`, that records which change(s)' work surfaced a stub — documented in the convention, seeded in the change template, and populated by the human-attended `docket-new-change` capture path.

**Architecture:** This is a **pure documentation / prose change** to three markdown files. There is no runtime code and no parser change: `list_field()` in `scripts/lib/docket-frontmatter.sh` already parses `[a, b]` → `a b`, and frontmatter is read field-by-field with no schema/allowlist, so any future consumer reads `discovered_from` for free and every existing reader (board render, health checks, mirror, Artifacts renderer) is unaffected. The field is **informational, exactly like `related:`** — never a readiness gate, never blocking, directional (child → origin(s)).

**Tech Stack:** Markdown (skill docs + change template); bash test harness (`tests/*.sh`, each prints `PASS`/`FAIL` and exits with its fail count).

## Global Constraints

- **Field name:** `discovered_from` (snake_case, matching `depends_on` / `blocked_by` / `auto_groomable`). Copy verbatim.
- **Field shape:** a **list of change ids** — `discovered_from: [62]` — parallel to `related:` / `depends_on:` / `adrs:`. Empty/absent (`[]`) = deliberately planned work.
- **Semantics:** informational like `related:`; **never** a readiness gate, never introduces blocking. No automatic `related:` back-link on the origin change.
- **Placement:** immediately **after `related:`** in the manifest frontmatter block, grouped with the other informational cross-refs.
- **Size-budget guard (hard gate):** `tests/test_skill_size_budgets.sh` enforces per-file line/word budgets. Current actuals vs budgets — `skills/docket-convention/SKILL.md` 288/4640 vs **317/5104**, `skills/docket-new-change/SKILL.md` 55/1209 vs **61/1330**, `skills/docket-new-change/change-template.md` 46/184 vs **51/203**. Every edit here is a single line + comment (or one extended sentence); do **not** raise a budget row — the headroom is sufficient. Prefer one manifest line + one clarifying comment/sentence per file over new paragraphs.
- **No new render surface** (no Artifacts-block row, board column, or mermaid edge) and **no health check** for dangling ids — deliberately deferred (spec §Rendering, §Out of scope).
- **Enumerated-floor check (already performed at reconcile):** the live sites that enumerate the manifest field set are exactly `skills/docket-convention/SKILL.md` (authoritative manifest block) and `skills/docket-new-change/change-template.md` (seed); `README.md` references `reconciled:`/reconcile only in prose (not a field enumeration) and needs no edit; `docket-new-change` is the sole live stub-minter (autonomous minting is change #0091, still dormant). Do **not** add the field anywhere else.

---

### Task 1: Define and seed the `discovered_from` field

Add the field to the authoritative manifest block (convention) and seed it empty in the change template. Both are the field's "definition" sites and belong together — a reviewer accepts/rejects the field's documented shape as one unit.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (manifest frontmatter block — the `related:` line, currently line 142)
- Modify: `skills/docket-new-change/change-template.md` (frontmatter — the `related: []` line, currently line 10)
- Test: `tests/test_skill_size_budgets.sh`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: the documented field name/shape/placement (`discovered_from: [<ids>]`, after `related:`) that Task 2's population prose refers to.

- [ ] **Step 1: Run the size-budget guard to confirm the green baseline**

Run: `cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance && bash tests/test_skill_size_budgets.sh | tail -1`
Expected: `PASS` (establishes headroom before editing).

- [ ] **Step 2: Add `discovered_from:` to the convention manifest block**

In `skills/docket-convention/SKILL.md`, insert one line **immediately after** the `related: [4, 6]` line inside the change-manifest frontmatter block:

```
related: [4, 6]           # cross-links the reconcile pass reads
discovered_from: [62]     # change id(s) whose work surfaced this one; informational like related:, never a readiness gate
adrs: [24]                # ADRs this change cites or produces
```

(Insert only the `discovered_from:` line; the `related:` and `adrs:` lines already exist and frame the placement.)

- [ ] **Step 3: Seed `discovered_from:` empty in the change template**

In `skills/docket-new-change/change-template.md`, insert one line **immediately after** the `related: []` line:

```
related: []
discovered_from: []       # change id(s) whose work surfaced this; empty for deliberately planned work
adrs: []
```

(Insert only the `discovered_from: []` line; `related: []` and `adrs: []` already exist.)

- [ ] **Step 4: Run the size-budget guard to confirm both files stay within budget**

Run: `cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance && bash tests/test_skill_size_budgets.sh`
Expected: `PASS` — every `ok - ... within line/word budget` line green; specifically `skills/docket-convention/SKILL.md` and `skills/docket-new-change/change-template.md` still under 317/5104 and 51/203 respectively.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance
git add skills/docket-convention/SKILL.md skills/docket-new-change/change-template.md
git commit -m "docs(0090): define + seed discovered_from manifest field"
```

---

### Task 2: Wire human-attended population into `docket-new-change`

Extend `docket-new-change`'s step 3 ("Scan related context") — which already pre-fills `related`/`depends_on`/`adrs` — so it also records `discovered_from` when the human names an originating change (or scan mode infers one). Prose only; new-change writes frontmatter by hand.

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (step 3, currently line 35)
- Test: `tests/test_skill_size_budgets.sh`, then the full suite (`tests/*.sh`)

**Interfaces:**
- Consumes: the field name/shape from Task 1 (`discovered_from`, list of ids).
- Produces: nothing downstream (final task).

- [ ] **Step 1: Extend step 3 to record `discovered_from`**

In `skills/docket-new-change/SKILL.md`, replace the step-3 sentence

> 3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`. In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs` after the design settles.

with

> 3. **Scan related context** — scan neighbouring changes (`active/` + recent `archive/`) and the ADR index to pre-fill `related`, `depends_on`, `adrs`, and — when the human names the change(s) whose work surfaced this one (or scan mode infers an origin) — `discovered_from` (informational, like `related:`; empty for deliberately planned work). In practice, do this quick read just *before* step 2 so the brainstorm is informed by neighbouring work; record the resulting `related`/`depends_on`/`adrs`/`discovered_from` after the design settles.

(Edit is to the existing line only — no new lines added, so the file's line count is unchanged; ~20 words added, well within the 1330-word budget.)

- [ ] **Step 2: Run the size-budget guard**

Run: `cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance && bash tests/test_skill_size_budgets.sh`
Expected: `PASS` — `skills/docket-new-change/SKILL.md` still under 61 lines / 1330 words.

- [ ] **Step 3: Run the full test suite to confirm no regression**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance
fail=0; for t in tests/*.sh; do bash "$t" >/tmp/t.out 2>&1 || { echo "FAIL: $t"; tail -3 /tmp/t.out; fail=1; }; done; [ "$fail" = 0 ] && echo "ALL GREEN"
```
Expected: `ALL GREEN` — the change touches only markdown prose and adds no parser/schema, so no board/mirror/health-check/frontmatter test regresses (spec §Testing considerations).

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/discovered-from-provenance
git add skills/docket-new-change/SKILL.md
git commit -m "docs(0090): record discovered_from in docket-new-change scan step"
```
