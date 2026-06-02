# Change *results* Artifact — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an optional, linked close-out **results artifact** to docket — a feature-branch build file (twin of the plan), linked from a change by a new `results:` field, homed at `docs/results/` (`results_dir`).

**Architecture:** docket "code" is the five `skills/*/SKILL.md` files plus helper scripts/templates. A shared `## Convention` block is kept byte-identical across the skills by `sync-convention.sh` (canonical: `docket-new-change/SKILL.md`). This change edits the canonical block (manifest field + `.docket.yml` knob + directory layout + branch-model line), propagates it with the sync script, adds the `results:` semantics to three skills' flow prose, adds the field to `change-template.md`, ships a new `results-template.md`, and reconciles the main design spec + README. Verification is a new shell regression test asserting the convention is in sync and carries the new field/dir, plus that the templates exist.

**Tech Stack:** Markdown skill files, Bash (`sync-convention.sh`, `tests/*.sh`).

---

## File structure

- **Modify** `skills/docket-new-change/SKILL.md` — canonical convention block (the only place the block is hand-edited).
- **Modify (via `sync-convention.sh`)** `skills/docket-status/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md` — convention block propagated byte-identical.
- **Modify** `skills/docket-implement-next/SKILL.md` — new conditional "Results close-out" flow step + `results:` in the step-7 metadata write (non-convention prose).
- **Modify** `skills/docket-finalize-change/SKILL.md` — post-merge "append outcomes to the results file" note (non-convention prose).
- **Modify** `skills/docket-status/SKILL.md` — extend the broken-link health check + descriptions to cover `results:` (non-convention prose).
- **Modify** `skills/docket-new-change/change-template.md` — add the `results:` field.
- **Create** `skills/docket-implement-next/results-template.md` — the 3-section close-out template (consumer = the implementer).
- **Modify** `docs/superpowers/specs/2026-05-30-docket-design.md` — add a §3 locked decision; annotate the §12 line-401 "folded into the body" as the historical migration approach.
- **Modify** `README.md` — add `results_dir` to the `.docket.yml` example and mention `docs/results/`.
- **Create** `tests/test_results_artifact.sh` — regression test for all of the above.

---

### Task 1: Failing regression test

**Files:**
- Create: `tests/test_results_artifact.sh`

- [ ] **Step 1: Write the test**

```bash
#!/usr/bin/env bash
# tests/test_results_artifact.sh — verifies the change-results-artifact convention.
# Run: bash tests/test_results_artifact.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)

# 1. The real convention blocks are byte-identical across all skills.
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'

# 2. The convention carries the results: manifest field in EVERY skill.
for s in "${SKILLS[@]}"; do
  assert "results: field present in $s" \
    'grep -q "^results:" "skills/'"$s"'/SKILL.md"'
done

# 3. The convention carries the results_dir knob + the docs/results layout entry in every skill.
for s in "${SKILLS[@]}"; do
  assert "results_dir knob present in $s" 'grep -q "results_dir" "skills/'"$s"'/SKILL.md"'
  assert "results_dir layout entry present in $s" 'grep -q "<results_dir>/" "skills/'"$s"'/SKILL.md"'
done

# 4. Branch-model line includes results.
assert "branch-model line mentions results" \
  'grep -q "plan + results + code" "skills/docket-new-change/SKILL.md"'

# 5. Templates.
assert "change-template has results: field" \
  'grep -q "^results:" skills/docket-new-change/change-template.md'
assert "results-template.md exists" \
  '[ -f skills/docket-implement-next/results-template.md ]'
assert "results-template has Verify (human) section" \
  'grep -q "## Verify (human)" skills/docket-implement-next/results-template.md'
assert "results-template has Findings section" \
  'grep -q "## Findings" skills/docket-implement-next/results-template.md'
assert "results-template has Follow-ups section" \
  'grep -q "## Follow-ups" skills/docket-implement-next/results-template.md'

# 6. Flow prose wired into the three skills.
assert "implement-next has a results close-out step" \
  'grep -qi "results close-out\|Results close-out" skills/docket-implement-next/SKILL.md'
assert "status health check covers results: link" \
  'grep -q "results:" skills/docket-status/SKILL.md'
assert "finalize mentions appending to the results file" \
  'grep -qi "results" skills/docket-finalize-change/SKILL.md'

# 7. Design spec + README reconciled.
assert "design spec has results-artifact decision" \
  'grep -qi "results artifact\|results.artifact" docs/superpowers/specs/2026-05-30-docket-design.md'
assert "README documents results_dir" 'grep -q "results_dir" README.md'

exit $fail
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `bash tests/test_results_artifact.sh`
Expected: several `NOT OK` lines (no `results:` field, no `results_dir`, no template, prose not wired), non-zero exit. (`--check` itself may pass at this point because all five blocks are still identical — that's fine; it will exercise red→green in Task 2.)

- [ ] **Step 3: Commit**

```bash
git add tests/test_results_artifact.sh
git commit -m "test(results): failing regression test for the results artifact convention"
```

---

### Task 2: Add `results` to the canonical convention block, then sync

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (canonical block only)
- Modify (generated): the other four `skills/*/SKILL.md` via `sync-convention.sh`

- [ ] **Step 1: Add the `results_dir` knob to the `.docket.yml` example**

In the canonical block, change:

```yaml
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
```
to:
```yaml
changes_dir: docs/changes    # default
adrs_dir: docs/adrs          # default
results_dir: docs/results    # default  — close-out 'results' artifacts (build-time files, like plans)
```

- [ ] **Step 2: Add the `<results_dir>/` directory-layout entry**

Immediately after the `<adrs_dir>/` block in the layout, add:

```
<results_dir>/            # default docs/results/  — optional close-out artifacts (feature-branch build files; NEVER archived)
  <YYYY-MM-DD>-<slug>-results.md
```

- [ ] **Step 3: Add the `results:` manifest field**

In the change-manifest frontmatter, immediately after the `plan:` line, add:

```yaml
results:                  # results FILE on the feature branch; this FIELD set in the main tree at close-out (optional)
```

- [ ] **Step 4: Update the branch-model one-liner**

Change `The feature branch adds only the plan + code and **never modifies** docket metadata.` to `The feature branch adds only the plan + results + code and **never modifies** docket metadata.`

- [ ] **Step 5: Propagate to the other four skills**

Run: `bash sync-convention.sh`
Expected: `synced skills/docket-status/SKILL.md` (and the other three).

- [ ] **Step 6: Verify sync + field presence**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync` (exit 0).
Run: `bash tests/test_results_artifact.sh`
Expected: the sync, `results:`, `results_dir`, `<results_dir>/`, and branch-model assertions now PASS; template + prose assertions still fail.

- [ ] **Step 7: Commit**

```bash
git add skills/*/SKILL.md
git commit -m "feat(convention): add results: field + results_dir to the synced convention block"
```

---

### Task 3: Templates

**Files:**
- Modify: `skills/docket-new-change/change-template.md`
- Create: `skills/docket-implement-next/results-template.md`

- [ ] **Step 1: Add `results:` to `change-template.md`**

After the `plan:` line in the template frontmatter, add:

```yaml
results:                  # left empty; set by docket-implement-next at close-out if warranted
```

- [ ] **Step 2: Create `results-template.md`**

```markdown
<!-- results-template.md — close-out artifact for a change. OPTIONAL: write one only when at least
     one is true: (a) the human must run interactive/manual checks at the merge gate beyond automated
     tests, (b) the build surfaced findings worth recording (incl. any that became ADRs), or
     (c) there are follow-ups / notable plan deviations. Otherwise skip it — the PR + green CI are the
     receipt. Authored in the feature worktree and committed on feat/<slug> (a build artifact, like the
     plan); keep build-receipt detail in the PR description, not here. -->
# <title> — results
Change: #<id> · Branch: feat/<slug> · PR: <url> · Plan: <path> · ADRs: <ids>

## Verify (human)

<!-- Interactive/manual checks for the merge gate. Each item PENDING until checked. -->
- [ ] …

## Findings

<!-- Discoveries during the build; note which became ADRs. Delete if none. -->

## Follow-ups

<!-- Deferred items / new proposed changes. Delete if none. -->
```

- [ ] **Step 3: Verify**

Run: `bash tests/test_results_artifact.sh`
Expected: the `change-template`, `results-template.md exists`, and three section assertions now PASS.

- [ ] **Step 4: Commit**

```bash
git add skills/docket-new-change/change-template.md skills/docket-implement-next/results-template.md
git commit -m "feat(templates): add results: to change-template; add results-template.md"
```

---

### Task 4: Wire the flow prose into the three skills

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md` (non-convention)
- Modify: `skills/docket-finalize-change/SKILL.md` (non-convention)
- Modify: `skills/docket-status/SKILL.md` (non-convention)

- [ ] **Step 1: `docket-implement-next` — add the close-out step**

At the END of `### Step 6 — Review + ADRs` (after the ADR paragraph), add a new section:

```markdown
### Step 6.5 — Results close-out (optional)

Write a results file ONLY if at least one is true: **(a)** the human must run interactive/manual checks at the merge gate beyond automated tests, **(b)** the build surfaced findings worth recording (including any that became ADRs), or **(c)** there are follow-ups or notable plan deviations to capture. Otherwise SKIP it — the PR description + green CI are the receipt.

When warranted: author `<results_dir>/<YYYY-MM-DD>-<slug>-results.md` from `results-template.md` **IN THE FEATURE WORKTREE** and commit it on `feat/<slug>` with the code — it is a build artifact, like the plan. Keep build-receipt detail (what shipped, full test tables) in the PR description, not here. The `results:` FIELD is set in the main tree in step 7 (the file is feature-branch, the field is metadata — same split as `plan:`).
```

Then in `### Step 7 — PR + stop`, change `set status: implemented + pr:` to `set status: implemented + pr: (and results: if a results file was written in step 6.5)`.

- [ ] **Step 2: `docket-finalize-change` — post-merge append note**

After per-change `2. Verify the merge landed on main …`, add a note:

```markdown
> **Close-out (optional).** If the change carries a `results:` file, this is the moment to append interactive-verification **outcomes** and any late findings to it — on `main`, post-merge. The results file is the durable record of what was hand-verified at the gate.
```

- [ ] **Step 3: `docket-status` — extend the broken-link health check + mentions**

Change the health-check bullet:

`- **Broken \`plan:\` link on \`done\` changes** — a \`done\` change's \`plan:\` path must resolve (link rot check). Ignore a missing \`plan:\` on an \`implemented\` change — its plan legitimately still lives on the unmerged feature branch.`

to:

`- **Broken \`plan:\`/\`results:\` link on \`done\` changes** — a \`done\` change's \`plan:\` and \`results:\` paths must resolve (link rot check). Ignore a missing \`plan:\` or \`results:\` on an \`implemented\` change — those files legitimately still live on the unmerged feature branch.`

Also update the "When to use" line `- You suspect spec or plan links are stale or broken.` to `- You suspect spec, plan, or results links are stale or broken.` and the frontmatter `description:` phrase `broken spec/plan links` to `broken spec/plan/results links`.

- [ ] **Step 4: Verify convention still in sync (prose edits must not touch the block)**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync` (exit 0). If it reports drift, a prose edit accidentally landed inside the markers — move it outside and re-check.
Run: `bash tests/test_results_artifact.sh`
Expected: the three flow-prose assertions now PASS.

- [ ] **Step 5: Commit**

```bash
git add skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md
git commit -m "feat(skills): wire results close-out into implement-next, finalize, status"
```

---

### Task 5: Reconcile the design spec + README

**Files:**
- Modify: `docs/superpowers/specs/2026-05-30-docket-design.md`
- Modify: `README.md`

- [ ] **Step 1: Annotate the §12 line-401 historical note**

Find the §12 dogfood bullet containing `(with their results folded into the body)` and append, in the same sentence/parenthetical:

`(That body-folding was the one-time Markhaus migration approach; the go-forward convention for new close-out docs is the linked **results artifact** — see change 0001 and \`docs/superpowers/specs/2026-06-02-results-artifact-design.md\`.)`

- [ ] **Step 2: Add a §3 locked decision**

Read `## 3. Locked decisions`, find the highest existing item number N, and append item N+1:

`N+1. **The change *results* artifact.** A change's optional close-out doc — the human's merge-gate verification checklist, findings, and follow-ups — is a *feature-branch build artifact* (a twin of the plan), linked by a \`results:\` field and homed at \`docs/results/\` (the \`results_dir\` knob). Written only when warranted, never by default; otherwise the PR + CI are the receipt. This supersedes the earlier "fold results into the body" intent (§12). Full design: \`docs/superpowers/specs/2026-06-02-results-artifact-design.md\`.`

- [ ] **Step 3: README — `.docket.yml` example**

In the README `.docket.yml` code block, after `adrs_dir: docs/adrs          # default`, add:

`results_dir: docs/results    # default`

- [ ] **Step 4: README — mention `docs/results/`**

Find the line `The change data — \`docs/changes/\`, \`docs/adrs/\` — lives per consuming project, not in the docket repo itself.` and change it to include results:

`The change data — \`docs/changes/\`, \`docs/adrs/\`, \`docs/results/\` — lives per consuming project, not in the docket repo itself.`

- [ ] **Step 5: Verify**

Run: `bash tests/test_results_artifact.sh`
Expected: the design-spec and README assertions now PASS (all assertions PASS overall).

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/specs/2026-05-30-docket-design.md README.md
git commit -m "docs: reconcile design spec line-401 + README for the results artifact"
```

---

### Task 6: Full green + suite

**Files:** none (verification only)

- [ ] **Step 1: Run the whole test suite**

Run: `for t in tests/*.sh; do echo "== $t =="; bash "$t"; done`
Expected: every test prints only `ok - …` lines and exits 0 — including the pre-existing `test_sync_convention.sh` and `test_link_skills.sh` (this change must not regress them).

- [ ] **Step 2: Final convention sync check**

Run: `bash sync-convention.sh --check`
Expected: `convention in sync`.

- [ ] **Step 3: Confirm no metadata leaked onto the feature branch**

Run: `git diff --name-only origin/main... | grep '^docs/changes/' || echo "clean — no docs/changes/ edits on the branch"`
Expected: `clean — no docs/changes/ edits on the branch` (the change file, BOARD, ADRs are metadata and must stay on `main`).
