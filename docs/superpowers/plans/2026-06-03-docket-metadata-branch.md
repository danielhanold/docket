# docket metadata branch — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `docket`-mode (planning metadata on a dedicated orphan `docket` branch, terminal records published to the integration branch) real, fully specified, and the default — closing the documented v1 rough edge.

**Architecture:** This is a **docs/skills change**, built **TDD-for-docs** (the model change 0001 used): a `tests/` assertion suite (grep + `sync-convention.sh --check`) is the red/green gate, and the detailed content is the linked spec — `docs/superpowers/specs/2026-06-02-docket-metadata-branch-design.md`. The plan's tasks point at the precise spec section for prose (DRY: the spec is the design; the plan is the task list) and at the concrete assertions they must turn green. The canonical **convention block** lives in `docket-new-change/SKILL.md` and is propagated byte-identical to the other four skills by `sync-convention.sh`; per-skill *procedure* prose is edited in each skill directly.

**Tech Stack:** Markdown skill files, a synced convention block (`sync-convention.sh`), POSIX/bash (`migrate-to-docket.sh`, the test suite), git worktrees + plumbing-free git.

**Spec section map (content source for each touch-point):**
- §4 — `.docket.yml` (`metadata_branch: docket` default, `integration_branch` knob)
- §5 — orphan `docket` branch, always-push
- §6 — the `.docket/` metadata worktree (creation per-state, sync-before-read, BOARD.md conflict rule)
- §7.0 — four-state bootstrap guard (convention block)
- §7.1–7.6 — per-skill behavior; §7.7 — the shared terminal-publish procedure (worktree variant)
- §8 — branch-model rewrite (convention block)
- §9 — `migrate-to-docket.sh`
- §10 — README docket-mode section + artifact-location table
- §11 — ADRs to record at review (recorded on `main` in implement-next step 6, NOT a feature-branch task)
- §12 — touch-points / the test assertions
- §13/§7.6 — `main`-mode backward-compat

---

## File Structure (feature-branch artifacts)

- **Create** `tests/test_docket_metadata_branch.sh` — the assertion suite (red/green gate).
- **Modify** `skills/docket-new-change/SKILL.md` — the **canonical convention block** (§4, §7.0, §8, "metadata working tree" wording, `.docket/` layout) + producer prose (§7.1).
- **Modify** `skills/docket-implement-next/SKILL.md` — convention (via sync) + procedure prose (§6, §7.2); **remove** the v1 `docket` caveat.
- **Modify** `skills/docket-finalize-change/SKILL.md` — convention (via sync) + the **§7.7 terminal-publish procedure** (single-source home) + done-path prose (§7.3).
- **Modify** `skills/docket-status/SKILL.md` — convention (via sync) + prose (§7.4: board-on-docket, sweep→terminal-publish, plan/results validated on integration).
- **Modify** `skills/docket-adr/SKILL.md` — convention (via sync) + prose (§7.5: `Accepted`-ADR publish via the ADR-only terminal-publish).
- **Create** `migrate-to-docket.sh` (repo root, `chmod +x`) — §9.
- **Modify** `.gitignore` — add `.docket/`, `.worktrees/` (file already exists; extend).
- **Modify** `README.md` — §10 docket-mode section + artifact table; update Status + Install.
- **Run** `sync-convention.sh` after every convention-block edit (propagates; never hand-edit a non-canonical block).

**Build decision (record as the build progresses, becomes ADR-002):** the §7.7 terminal-publish procedure is **single-sourced in `docket-finalize-change`** (the canonical terminal-transition skill); `docket-new-change`, `docket-implement-next`, and `docket-adr` *reference* it ("run the terminal-publish procedure — docket-finalize-change") rather than duplicating the git sequence. Rationale: it is an operational procedure, not a contract, so it does not belong in the byte-identical convention block (which would 5×-duplicate ~25 lines of git); the bootstrap guard (§7.0) stays in the convention because it is a short cross-agent *rule*, not a procedure.

**Not feature-branch tasks (handled elsewhere by the implementer flow):**
- The **change file / spec / BOARD.md** are metadata on `main` — never touched on the feature branch.
- The **ADRs** (§11: the branch-model ADR + the docket-default/refuse-to-migrate ADR) are recorded via `docket-adr` on `main` during implement-next **step 6**, not here.

---

## Phase 0 — Test scaffold (red gate)

### Task 1: Author the assertion suite

**Files:**
- Create: `tests/test_docket_metadata_branch.sh`

- [ ] **Step 1: Write the test (it will mostly fail — red)**

```bash
#!/usr/bin/env bash
# tests/test_docket_metadata_branch.sh — verifies docket-mode (the metadata-branch change, 0002).
# Run: bash tests/test_docket_metadata_branch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr)

# A. Convention blocks byte-identical across all skills.
assert "convention blocks in sync (sync-convention.sh --check)" \
  'bash sync-convention.sh --check >/dev/null 2>&1'

# B. metadata_branch default flipped to docket, in every skill's convention.
for s in "${SKILLS[@]}"; do
  assert "metadata_branch default is docket in $s" \
    'grep -Eq "^metadata_branch: docket" "skills/'"$s"'/SKILL.md"'
done

# C. integration_branch knob present in every skill's convention.
for s in "${SKILLS[@]}"; do
  assert "integration_branch knob present in $s" \
    'grep -q "integration_branch" "skills/'"$s"'/SKILL.md"'
done

# D. The "metadata working tree" abstraction + .docket worktree appear in every skill.
for s in "${SKILLS[@]}"; do
  assert "metadata working tree wording in $s" \
    'grep -qi "metadata working tree" "skills/'"$s"'/SKILL.md"'
done

# E. Branch-model: feature branch cut from the integration branch (not hard-coded main).
assert "branch-model generalized to integration_branch" \
  'grep -q "origin/<integration_branch>" "skills/docket-new-change/SKILL.md"'

# F. Bootstrap guard (refuse-to-migrate) present in the convention.
assert "bootstrap guard present in convention" \
  'grep -qiE "half-migrated|bootstrap guard|migrate-to-docket" "skills/docket-status/SKILL.md"'

# G. The v1 docket caveat is REMOVED from docket-implement-next.
assert "v1 docket caveat removed from implement-next" \
  '! grep -qi "v1 rough edge" skills/docket-implement-next/SKILL.md'

# H. Terminal-publish: single-sourced in finalize; copy-set is change+spec+Accepted ADRs.
assert "terminal-publish procedure in finalize" \
  'grep -qi "terminal publish\|terminal-publish" skills/docket-finalize-change/SKILL.md'
assert "publish copies from origin/docket (not a branch merge)" \
  'grep -q "checkout origin/docket" skills/docket-finalize-change/SKILL.md'
assert "Accepted gate on ADR publish" \
  'grep -qi "Accepted" skills/docket-finalize-change/SKILL.md'

# I. Kill-publish wired in BOTH kill origins (producer + implementer), not just finalize.
assert "proposed-kill wired in docket-new-change" \
  'grep -qi "kill" skills/docket-new-change/SKILL.md && grep -qi "terminal.publish\|terminal-publish" skills/docket-new-change/SKILL.md'
assert "reconcile-kill wired in docket-implement-next" \
  'grep -qi "kill" skills/docket-implement-next/SKILL.md && grep -qi "terminal.publish\|terminal-publish" skills/docket-implement-next/SKILL.md'

# J. docket-status: sweep invokes terminal-publish; plan/results validated against integration.
assert "status sweep invokes terminal-publish" \
  'grep -qi "terminal.publish\|terminal-publish" skills/docket-status/SKILL.md'

# K. docket-adr: Accepted ADRs publish via the ADR-only path.
assert "adr skill references terminal-publish / ADR-only publish" \
  'grep -qi "terminal.publish\|terminal-publish\|publish" skills/docket-adr/SKILL.md'

# L. main-mode backward-compat documented (the pinned opt-out).
assert "main-mode opt-out documented in convention" \
  'grep -qiE "metadata_branch: main|single-branch|main-mode" "skills/docket-new-change/SKILL.md"'

# M. .gitignore ignores the metadata worktree + feature worktrees.
assert ".gitignore ignores .docket/" 'grep -qE "^\.docket/?" .gitignore'
assert ".gitignore ignores .worktrees/" 'grep -qE "^\.worktrees/?" .gitignore'

# N. migrate-to-docket.sh exists, is executable, creates the orphan branch + prunes.
assert "migrate-to-docket.sh exists" '[ -f migrate-to-docket.sh ]'
assert "migrate-to-docket.sh is executable" '[ -x migrate-to-docket.sh ]'
assert "migration creates an orphan docket branch" \
  'grep -q "checkout --orphan docket\|worktree add --orphan" migrate-to-docket.sh'
assert "migration prunes the live surface" \
  'grep -qi "active\|BOARD.md" migrate-to-docket.sh'

# O. README documents docket-mode + the integration_branch knob + artifact locations.
assert "README documents metadata_branch: docket default" \
  'grep -q "metadata_branch: docket" README.md'
assert "README documents integration_branch" 'grep -q "integration_branch" README.md'
assert "README has docket-mode / artifact-location content" \
  'grep -qiE "docket-mode|artifact|lives on" README.md'

# P. Existing conventions preserved (no regression of the 0001 results work).
assert "results: field still present (no regression)" \
  'grep -q "^results:" skills/docket-new-change/SKILL.md'

exit $fail
```

- [ ] **Step 2: Run it — expect RED**

Run: `bash tests/test_docket_metadata_branch.sh; echo "exit=$?"`
Expected: many `NOT OK` lines, `exit=1`. (Assertion A may already pass; B–O fail.)

- [ ] **Step 3: Commit the red gate**

```bash
git add tests/test_docket_metadata_branch.sh
git commit -m "test(0002): assertion suite for docket-mode (red)"
```

---

## Phase 1 — Convention block (shared contract)

### Task 2: Rewrite the canonical convention block + propagate

**Files:**
- Modify: `skills/docket-new-change/SKILL.md` (the canonical block between `<!-- docket:convention:begin -->` / `:end -->`)
- Run: `sync-convention.sh`

- [ ] **Step 1: Edit the canonical convention block** to apply, verbatim from the spec:
  - **§4** — the `.docket.yml` example: `metadata_branch: docket` (default) `| main`; add `integration_branch: auto # auto(→origin/HEAD, fallback main) | main | develop`; keep `changes_dir`/`adrs_dir`/`results_dir`. Update the prose: `.docket.yml` lives on the **default branch (`origin/HEAD`)**, repaired via `git remote set-head origin -a`, read via `git show origin/HEAD:.docket.yml`; ref-unresolvable ≠ file-absent.
  - **Directory layout** — add the `.docket/` metadata-worktree note (gitignored; not under `.worktrees/`).
  - **§7.0** — the four-state bootstrap guard (`DOCKET`/`LIVE` probes via `git ls-tree origin/<integration_branch> -- <changes_dir>/active <changes_dir>/README.md BOARD.md`; exit≠0 ⇒ config error; the 2×2 table → STOP/proceed/create).
  - **§8** — the Branch-model rewrite: "metadata working tree" abstraction (main tree in single-branch mode, `.docket/` in docket-mode), always-push, `feat/<slug>` from `origin/<integration_branch>`, terminal-publish is the only metadata→code-line flow.
- [ ] **Step 2: Propagate** — `bash sync-convention.sh` (writes the block into the other four skills).
- [ ] **Step 3: Run** `bash sync-convention.sh --check` → exit 0; then `bash tests/test_docket_metadata_branch.sh` → assertions A, B, C, D, E, F, L now PASS (G–K, M–P still fail).
- [ ] **Step 4: Commit**

```bash
git add skills/*/SKILL.md
git commit -m "feat(0002): rewrite convention block — docket default, integration_branch, bootstrap guard, branch model (sync'd)"
```

---

## Phase 2 — Shared terminal-publish procedure

### Task 3: Add the §7.7 terminal-publish procedure to docket-finalize-change

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`

- [ ] **Step 1:** Add a labeled **"Terminal publish (docket-mode)"** section reproducing spec **§7.7** (worktree variant): two entry shapes (change-publish / ADR-only); step 1 archive-on-`docket`-first (idempotent filename reuse, `mkdir -p`); step 2 transient `mktemp` worktree on a `pub-<T>` branch (`-B` + `git worktree prune`); step 3 copy-set assembled as a list (change always; `spec:` iff set; **`Accepted`** ADRs), guarded commit, fast-forward `push HEAD:<integration_branch>` with rebase-retry + same-file re-copy; step 4 force teardown. Plus the **done-path** prose (§7.3): merge PR → run terminal-publish(`done`); **`main`-mode skips §7.7** (archive on the integration branch is the terminal record; the archive-move contract is identical in both modes).
- [ ] **Step 2: Run** `bash tests/test_docket_metadata_branch.sh` → H now PASS.
- [ ] **Step 3: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md
git commit -m "feat(0002): finalize — shared terminal-publish procedure + done-path (spec §7.7/§7.3)"
```

---

## Phase 3 — Per-skill procedure prose

### Task 4: docket-implement-next (§6, §7.2) — remove v1 caveat, add docket-mode mechanics

**Files:** Modify `skills/docket-implement-next/SKILL.md`

- [ ] **Step 1:** Apply spec §6 + §7.2: **delete** the "`docket` mode caveat (v1 rough edge)" subsection; document the `.docket/` worktree surface (state-specific creation, sync-before-read, exists-guard), reconcile-confirm by SHA on `origin/docket`, feature base `origin/<integration_branch>` (+ fetch), spec read from `.docket/` (cross-tree plan write), and the **reconcile-kill** sub-path (set `killed`, push `origin/docket`, run terminal-publish(`killed`) — docket-finalize-change; main-mode degradation). Generalize Step 4's "confirm reconcile landed on origin/main" to the metadata branch.
- [ ] **Step 2: Run tests** → G PASS, I (implementer half) PASS. `sync-convention.sh --check` still 0 (prose edits are outside the convention markers).
- [ ] **Step 3: Commit** `git commit -am "feat(0002): implement-next — docket-mode mechanics; remove v1 caveat (spec §6/§7.2)"`

### Task 5: docket-new-change (§7.1) — producer docket-mode + proposed-kill

**Files:** Modify `skills/docket-new-change/SKILL.md` (prose outside the convention block)

- [ ] **Step 1:** Apply §7.1: producer reads/writes in `.docket/`, pushes `origin/docket`; spec written under `.docket/docs/superpowers/specs/`; **proposed-kill** sub-path (set `killed`, push, run terminal-publish(`killed`); main-mode degradation).
- [ ] **Step 2: Run tests** → I (producer half) PASS.
- [ ] **Step 3: Commit** `git commit -am "feat(0002): new-change — producer docket-mode + proposed-kill (spec §7.1)"`

### Task 6: docket-status (§7.4) — board on docket, sweep→publish, link validation

**Files:** Modify `skills/docket-status/SKILL.md`

- [ ] **Step 1:** Apply §7.4: board regenerated in `.docket/` and never published (stays on `docket`); the **sweep `→done` invokes terminal-publish**; broken-link checks resolve `spec:` against `docket` and **`plan:`/`results:` against `origin/<integration_branch>`** (not `docket`); BOARD.md conflict = regenerate, never 3-way merge.
- [ ] **Step 2: Run tests** → J PASS.
- [ ] **Step 3: Commit** `git commit -am "feat(0002): status — board-on-docket, sweep→terminal-publish, link validation (spec §7.4)"`

### Task 7: docket-adr (§7.5) — Accepted-ADR publish

**Files:** Modify `skills/docket-adr/SKILL.md`

- [ ] **Step 1:** Apply §7.5: ADRs authored in `.docket/docs/adrs/`; **an `Accepted` ADR publishes to the integration branch** — change-tied via its change's terminal-publish, standalone/superseded via `docket-adr`'s own ADR-only terminal-publish call.
- [ ] **Step 2: Run tests** → K PASS.
- [ ] **Step 3: Commit** `git commit -am "feat(0002): adr — Accepted-ADR publish via ADR-only terminal-publish (spec §7.5)"`

---

## Phase 4 — Migration script, ignore, README

### Task 8: Create migrate-to-docket.sh

**Files:** Create `migrate-to-docket.sh` (repo root)

- [ ] **Step 1:** Author the one-shot script per spec **§9**: preconditions (clean tree; live surface present on integration; abort if `origin/docket` already exists — adopt instead); seed orphan `docket` from `<changes_dir>/` + `<adrs_dir>/` + `docs/superpowers/specs/` + `BOARD.md` (whole dirs, incl. their README blurbs), push; prune the **live surface** from the integration branch (`<changes_dir>/active/`, `<changes_dir>/README.md`, `BOARD.md`) keeping terminal records + build artifacts; extend `.gitignore`; idempotent **split mutation (probe LOCAL, tolerant `git rm -r --ignore-unmatch`) + push-guard (probe `origin/<branch>`)**; `git ls-tree` probes (exit≠0 ⇒ config error); print next steps. `chmod +x migrate-to-docket.sh`.
- [ ] **Step 2: Lint** — `bash -n migrate-to-docket.sh` (syntax) and `shellcheck migrate-to-docket.sh` if available; expect clean.
- [ ] **Step 3: Run tests** → N PASS.
- [ ] **Step 4: Commit** `git add migrate-to-docket.sh && git commit -m "feat(0002): migrate-to-docket.sh — one-shot single-branch→docket migration (spec §9)"`

### Task 9: Extend .gitignore

**Files:** Modify `.gitignore`

- [ ] **Step 1:** Append `.docket/` and `.worktrees/` (keep the existing `.DS_Store`).
- [ ] **Step 2: Run tests** → M PASS.
- [ ] **Step 3: Commit** `git commit -am "feat(0002): gitignore .docket/ and .worktrees/"`

### Task 10: Rewrite the README docket-mode section

**Files:** Modify `README.md`

- [ ] **Step 1:** Replace *How metadata is stored* with the full **docket-mode** section per spec **§10**: two-branch model + the **artifact-location table** (which artifact lives where, how it reaches the integration branch), the `integration_branch` knob + GitFlow, the `.docket/` worktree (what/why/gitignored), finalize → selective-publish (live board stays on `docket`), the migration story + refuse-to-migrate bootstrap, the `main`-mode pinned opt-out. Update *Status* (docket-mode is the supported default; drop "rough edge / not recommended") and *Install* (`.docket.yml` defaults: `metadata_branch: docket`, `integration_branch: auto`).
- [ ] **Step 2: Run tests** → O PASS.
- [ ] **Step 3: Commit** `git commit -am "docs(0002): README — docket-mode section + artifact table; Status/Install (spec §10)"`

---

## Phase 5 — Full green + regression

### Task 11: Whole-suite green + no regressions

- [ ] **Step 1: Run** every test + the sync check:

```bash
bash sync-convention.sh --check && \
bash tests/test_docket_metadata_branch.sh && \
bash tests/test_sync_convention.sh && \
bash tests/test_results_artifact.sh && \
bash tests/test_link_skills.sh
echo "ALL exit=$?"
```
Expected: all `ok -`, `ALL exit=0`. (`test_results_artifact` / `test_sync_convention` guard against regressing the 0001 work and the convention sync.)

- [ ] **Step 2:** Fix any failures inline (most likely: a convention edit drifted a non-canonical block → re-run `sync-convention.sh`; or a 0001 assertion regressed → restore the relevant line). Re-run until green.
- [ ] **Step 3: Commit** any fixups: `git commit -am "test(0002): full suite green; no regressions"`

---

## Post-build (NOT feature-branch tasks — implement-next flow)

- **Step 6 (review + ADRs):** record via `docket-adr` **on `main`** — (1) *the docket metadata-branch model* (orphan branch + `.docket/` worktree + selective terminal publish; merge is the wrong tool, `checkout origin/docket -- <paths>` is right) and (2) *docket-mode as default + the single-sourcing of §7.7 in finalize + refuse-and-migrate bootstrap*. Append their numbers to `0002`'s `adrs:` in the main tree.
- **Step 6.5 (results):** warranted here (the human should, at the merge gate, eyeball the rendered README docket-mode section and skim `migrate-to-docket.sh`, and the build will likely surface follow-ups e.g. dogfooding this repo's own migration). Author `<results_dir>/2026-06-03-docket-metadata-branch-results.md` in the worktree.
- **Step 7 (PR + stop):** push `feat/docket-metadata-branch`, open the PR (no merge); set `0002` `status: implemented` + `pr:` + `results:` on `main`. Stop at the human merge gate.

## Self-review notes (done)

- **Spec coverage:** every §12 touch-point maps to a task (convention→T2, §7.7→T3, §7.2→T4, §7.1→T5, §7.4→T6, §7.5→T7, §9→T8, .gitignore→T9, §10→T10) and a test assertion (A–P). ADRs (§11) are step-6 main-tree work, explicitly out of the feature-branch task list.
- **No placeholders:** the new artifact (the test suite) is shown in full; prose edits cite the exact spec section that holds the final content (docket's model: detailed design lives in the spec, the plan is the task list).
- **Consistency:** the §7.7 single-source decision (finalize) is applied uniformly — T3 writes it, T4/T5/T6/T7 *reference* it; the test (H–K) checks both the home and the references.
