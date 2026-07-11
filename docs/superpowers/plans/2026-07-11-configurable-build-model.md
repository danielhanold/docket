# Configurable SDD build models Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a top-level `build:` config surface with two per-role model IDs (`build.implementer`, `build.reviewer`) that `docket-config.sh` resolves and exports, and a behavioral rule in `docket-implement-next` that fills SDD's already-required `model:` field from them — deferring to SDD's own Model Selection when unset. Purely additive, backward-compatible, harness-neutral (direct model-ID passthrough, no tier layer).

**Architecture:** `build:` mirrors the existing `skills:` block in `docket-config.sh` exactly — a nested block parsed per-leaf with layered precedence (local `.docket.local.yml` > repo-committed `.docket.yml` > global `config.yml`), emitted as `BUILD_IMPLEMENTER` / `BUILD_REVIEWER` (empty when unset). It is global-able (a per-machine model preference, NOT a coordination key, so not fenced). `docket-implement-next`'s SDD hand-off (step 5) reads the two vars and, when set, uses them as the `model:` for the matching SDD dispatch kinds: `build.implementer` → per-task implementer + fix subagents; `build.reviewer` → task-reviewer + final whole-branch code-reviewer. No new script, no SDD fork.

**Tech Stack:** Bash (`docket-config.sh`, matching its `skills:`-block idiom), Markdown (skill, convention, README, contract), hermetic bash tests under `tests/`.

## Global Constraints

- **Purely additive / backward-compatible:** `build:` absent or a role unset ⇒ behavior is exactly today's — that dispatch keeps SDD's own per-complexity Model Selection judgment. Never override a dispatch whose role is unset.
- **Direct model-ID passthrough (no interpretation/validation):** the values go straight into SDD's `model:` field — whatever the running harness honors (a Claude alias/ID under Claude Code; a Cursor model ID under Cursor). docket does NOT parse or validate the string (same contract as the `agents:` block). An arbitrary non-Claude ID must pass through unchanged.
- **Two roles → four SDD dispatch kinds:** `build.implementer` governs the per-task implementer AND the fix subagents; `build.reviewer` governs the task-reviewer AND the final whole-branch code-reviewer. (The final reviewer folds into `build.reviewer` — spec Q2 resolved.)
- **`build:` is global-able**, resolved local > repo-committed > global (mirroring `skill_role`). It is NOT a coordination key — do NOT add it to the coordination-fence list in `docket-config.sh` (Stage 2c).
- **Emit-count invariant:** `test_docket_config.sh` asserts the exact number of `KEY=value` lines from `--export` (currently 18). Adding `BUILD_IMPLEMENTER` + `BUILD_REVIEWER` makes it **20** — update that assertion.
- **No SDD fork:** the wiring only fills SDD's already-required `model:` field (confirmed present as `model: [MODEL — REQUIRED]` in `implementer-prompt.md` and `task-reviewer-prompt.md`).
- **The build-time ADR** ("the SDD build model is docket-configurable via `build:` roles taking direct model IDs, defaulting to SDD's selection when unset") is recorded at review time via `docket-adr` — NOT a plan task.
- **Run the FULL suite as the gate** (ONE foreground call, `timeout 600000`).

---

### Task 1: Parse + export the `build:` block in `docket-config.sh` (+ contract + tests)

**Files:**
- Modify: `scripts/docket-config.sh`
- Modify: `scripts/docket-config.md` (contract)
- Modify: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: the existing `yaml_block_body` / `yaml_get` helpers and the `$CFG` / `$GCFG` / `$LCFG` layer paths.
- Produces: emitted config vars `BUILD_IMPLEMENTER` and `BUILD_REVIEWER` (literal model-ID strings; empty string when unset), resolved local > repo > global.

- [ ] **Step 1: Write the failing tests** in `tests/test_docket_config.sh` (follow the existing skills-role test style — build a fixture clone, write config, `run … --export`, `eval`, assert). Add:
  - `.docket.yml` with `build:\n  implementer: my-cheap-model\n  reviewer: my-strong-model` ⇒ `BUILD_IMPLEMENTER=my-cheap-model`, `BUILD_REVIEWER=my-strong-model` (arbitrary non-Claude IDs pass through unchanged).
  - No `build:` block ⇒ `BUILD_IMPLEMENTER` and `BUILD_REVIEWER` both empty.
  - Layering: global `config.yml` sets `build.implementer: g-model`; repo `.docket.yml` sets `build.implementer: r-model` ⇒ resolves `r-model` (repo wins); a role only in global resolves to the global value; `.docket.local.yml` wins over repo.
  - Update the existing emit-count assertion from 18 to **20** `KEY=value` lines (and the "last line" assertion if it checks a specific tail key — keep `BOOTSTRAP` last; add the BUILD emits BEFORE `emit BOOTSTRAP`).
  Run → fail.
- [ ] **Step 2: Run to verify fail** `bash tests/test_docket_config.sh`.
- [ ] **Step 3: Implement in `scripts/docket-config.sh`** — mirror the `skills:` block (around the `SKILLS_BLK` region):

```bash
# --- build: role-keyed SDD build models (change 0044) — direct model-ID passthrough ---
# Nested block; per-key precedence: per-repo leaf > global leaf (machine-local wins over repo).
# Global-able (a per-machine model preference), so NOT in the coordination-key fence.
BUILD_BLK="$(mktemp)";  yaml_block_body "$CFG"  build >"$BUILD_BLK"
GBUILD_BLK="$(mktemp)"; yaml_block_body "$GCFG" build >"$GBUILD_BLK"
LBUILD_BLK="$(mktemp)"; yaml_block_body "$LCFG" build >"$LBUILD_BLK"
build_role(){  # build_role <role> -> resolved model-ID or empty
  local v; v="$(yaml_get "$LBUILD_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$BUILD_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GBUILD_BLK" "$1")"
  printf '%s' "$v"
}
BUILD_IMPLEMENTER="$(build_role implementer)"
BUILD_REVIEWER="$(build_role reviewer)"
# Unknown keys under build: warn-and-ignore (a typo must never abort), mirroring skills:.
for _blk in "$LBUILD_BLK" "$BUILD_BLK" "$GBUILD_BLK"; do
  while IFS= read -r _brole; do
    [ -n "$_brole" ] || continue
    case " implementer reviewer " in
      *" $_brole "*) : ;;
      *) printf 'docket-config: warning: unknown build role %s (known: implementer, reviewer); ignored\n' "$_brole" >&2 ;;
    esac
  done < <(yaml_block_keys "$_blk" 2>/dev/null || true)
done
```
  (Use the SAME key-enumeration helper the `skills:` unknown-role loop uses — read that loop and match it exactly; if it uses a different mechanism than `yaml_block_keys`, mirror that.) Add the emits before `emit BOOTSTRAP`:

```bash
  emit BUILD_IMPLEMENTER "$BUILD_IMPLEMENTER"
  emit BUILD_REVIEWER "$BUILD_REVIEWER"
```
- [ ] **Step 4: Run to verify pass** `bash tests/test_docket_config.sh` → all ok (including the 20-line count).
- [ ] **Step 5: Document in `scripts/docket-config.md`** — add `BUILD_IMPLEMENTER` / `BUILD_REVIEWER` to the emitted-vars list, noting: direct model-ID passthrough, layered local>repo>global, empty when unset, global-able (not fenced).
- [ ] **Step 6: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "feat(0044): docket-config parses + exports the build: model-ID surface"
```

---

### Task 2: The build-dispatch rule in `docket-implement-next`

**Files:**
- Modify: `skills/docket-implement-next/SKILL.md`
- Test: `tests/test_configurable_build_model.sh` (new)

**Interfaces:**
- Consumes: `BUILD_IMPLEMENTER` / `BUILD_REVIEWER` from the Step-0 config export.
- Produces: a documented rule at the SDD hand-off (step 5) that maps the two roles to SDD's four dispatch kinds and states the unset→SDD default.

- [ ] **Step 1: Write the failing test** `tests/test_configurable_build_model.sh` (harness shape per `tests/test_render_board.sh`). Assert on `IMPL="$REPO/skills/docket-implement-next/SKILL.md"`:

```bash
assert "implement-next names the build model surface" 'grep -q "BUILD_IMPLEMENTER" "$IMPL" && grep -q "BUILD_REVIEWER" "$IMPL"'
assert "build.implementer governs implementer + fix dispatches" 'grep -qiE "BUILD_IMPLEMENTER[^.]*(implementer|fix)|(implementer|fix)[^.]*BUILD_IMPLEMENTER" "$IMPL"'
assert "build.reviewer governs reviewer + final-review dispatches" 'grep -qiE "BUILD_REVIEWER[^.]*(review)|(review)[^.]*BUILD_REVIEWER" "$IMPL"'
assert "unset build role defers to SDD Model Selection" 'grep -qiE "unset[^.]*SDD|SDD.{0,40}Model Selection|defer to SDD" "$IMPL"'
assert "build wiring fills SDD model: field, no SDD fork" 'grep -qiE "model:" "$IMPL" && grep -qiE "no.{0,4}fork|already-required|SDD.s.{0,10}model" "$IMPL"'
```
Run → fail.
- [ ] **Step 2: Add the build-dispatch rule** to `skills/docket-implement-next/SKILL.md` at Step 5 (the SDD hand-off — "The resolved build skill … executes the plan task-by-task"). State: resolve `BUILD_IMPLEMENTER` / `BUILD_REVIEWER` from the Step-0 config export; when set, use `BUILD_IMPLEMENTER` as the `model:` for the per-task implementer AND fix subagents, and `BUILD_REVIEWER` as the `model:` for the task-reviewer AND the final whole-branch code-reviewer; when a role is unset, instruct nothing — SDD keeps its own Model Selection judgment for that dispatch. Note it fills SDD's already-required `model:` field (no SDD fork), and that a set role is a deliberate blunt override of SDD's per-complexity adaptivity. Keep it concise; do NOT restate SDD's internals.
  - Do NOT introduce a model tier literal or a specific model ID in the prose (the values are config-resolved, harness-neutral).
- [ ] **Step 3: Run** `bash tests/test_configurable_build_model.sh` → ok. Spot-run `bash tests/test_composition_wiring.sh` and `bash tests/test_docket_config.sh` → no new failures.
- [ ] **Step 4: Commit** `git add skills/docket-implement-next/SKILL.md tests/test_configurable_build_model.sh && git commit -m "feat(0044): docket-implement-next build-dispatch rule — fill SDD model: from build: roles"`.

---

### Task 3: Document `build:` in the convention + README

**Files:**
- Modify: `skills/docket-convention/SKILL.md`
- Modify: `README.md`
- Test: `tests/test_configurable_build_model.sh` (extend)

**Interfaces:**
- Consumes: the config surface + wiring.
- Produces: `build:` documented in the `.docket.yml` schema (convention) and in the README configuration section.

- [ ] **Step 1: Extend the test:**

```bash
CONV="$REPO/skills/docket-convention/SKILL.md"; RM="$REPO/README.md"
assert "convention documents the build: surface" 'grep -q "build:" "$CONV" && grep -qE "implementer|reviewer" "$CONV"'
assert "convention notes build: takes direct model IDs / defers to SDD" 'grep -qiE "model id|direct model|defers to SDD|SDD.{0,20}selection" "$CONV"'
assert "README documents build:" 'grep -q "build:" "$RM" && grep -qiE "implementer|reviewer" "$RM"'
```
Run → fail.
- [ ] **Step 2: Document in `skills/docket-convention/SKILL.md`** — in the `.docket.yml` config schema block (near `skills:`/`agents:`), add the `build:` key with `implementer:`/`reviewer:` and a one-line comment (per-role SDD build model IDs; direct passthrough; unset ⇒ SDD's own Model Selection). Add a short prose note in the Skill-layer or Agent-layer section: `build:` governs only the SDD sub-dispatches (implementer/fix vs reviewer/final-review), takes direct model IDs (harness-neutral, same passthrough as `agents:`), is global-able, and defers to SDD when unset. Keep it accurate; do not restate SDD internals.
- [ ] **Step 3: Document in `README.md`** — in the Configuration section (near the `.docket.yml` example / workflow-roles area), add the `build:` block to the commented `.docket.yml` example and a short explanation (the biggest cost lever; per-role model IDs; unset ⇒ SDD chooses; motivating case: a non-Claude/mixed roster through Cursor).
- [ ] **Step 4: Run** `bash tests/test_configurable_build_model.sh` → ok; spot-run `bash tests/test_convention_extraction.sh` and `bash tests/test_docket_metadata_branch.sh` → no new failures.
- [ ] **Step 5: Commit** `git add skills/docket-convention/SKILL.md README.md tests/test_configurable_build_model.sh && git commit -m "docs(0044): document the build: config surface in the convention + README"`.

---

### Task 4: Full-suite verification + read-only smoke

**Files:** none (verification only, unless a fix is needed).

**Interfaces:**
- Consumes: the whole change.
- Produces: evidence the suite is green and the export resolves the new vars.

- [ ] **Step 1: Read-only smoke** — `eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"` in this repo and confirm `BUILD_IMPLEMENTER` / `BUILD_REVIEWER` are emitted (empty here, since this repo sets no `build:`), and that the total emit line count is 20. Also do a scratch check: a temp `.docket.yml` with a `build:` block resolves an arbitrary ID through (do this in a `mktemp` fixture, NOT by editing this repo's committed config).
- [ ] **Step 2: FULL SUITE, ONE foreground call, timeout 600000:**

```bash
for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; n=$(grep -c "^NOT OK" <<<"$out"); [ "$n" -gt 0 ] && echo "FAIL $(basename "$t") ($n)"; done; echo "suite done"
```
Zero `FAIL`. A failure outside the new/updated tests = a missed consequence (e.g. another emit-count or schema assertion) — fix it: update a legitimately-stale count/schema assertion with justification, or fix the code if an invariant genuinely broke.
- [ ] **Step 3: Commit** (only if a fix was needed) `git add -A && git commit -m "test(0044): full-suite green for the build: surface"`.

---

## Notes for the implementer

- **This is a config + docs change.** The actual SDD dispatch honoring `model:` is NOT hermetically unit-testable (spec §"Tests"): assert the resolution (docket-config) + the documented wiring rule (skill), and note that the live per-harness honoring (spec Q4) is verified the same way the `agents:` block is — at build/runtime, not in the suite.
- **Mirror `skills:` exactly** for the `build:` block — same `yaml_block_body`/`yaml_get`/unknown-key-warn idiom, same layering. Do not invent a new parsing mechanism.
- **The 18→20 emit-count** in `test_docket_config.sh` is the one easy-to-miss mechanical consequence — update it in Task 1, and let the full suite (Task 4) catch any other count/schema assertion.
- **No model-ID or tier literal** in the skill/convention prose — the values are config-resolved and harness-neutral.
- Run the full suite in ONE foreground Bash call with `timeout 600000`.
