# Retire the auto-approve workflow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fully decommission change 0062's `finalize.auto_approve` bot-approval subsystem, record the reversal as a new ADR, and document the branch-protection recipe that actually works.

**Architecture:** This is a removal-plus-documentation change across four independent surfaces — (1) the workflow/template/setup-script/facade op, (2) the config knob and its resolver export, (3) the finalize skill's gate prose, (4) the README — plus a metadata-branch ADR reversal. Each surface is its own task because a reviewer could reject one while approving its neighbours. There is no new runtime behavior: the deliverable is the *absence* of the subsystem plus a doc-sentinel test that keeps the replacement documentation honest.

**Tech Stack:** POSIX-ish bash (`set -uo pipefail`), hand-rolled `assert`-style test scripts in `tests/test_*.sh`, markdown skills/docs, GitHub Actions (being deleted), docket's own metadata-branch tooling (`docket.sh`, `terminal-publish.sh`).

## Global Constraints

Copied verbatim from the spec and `AGENTS.md`; every task's requirements implicitly include this section.

- **Removal is scoped to `auto_approve` only.** `finalize.gate` (change 0015, the rebase-retest **correctness** gate) and `require_pr_approval` (change 0021 / ADR-0011, the human-authorization **policy** gate) stay intact and behaviorally unchanged. (spec §2 non-goals, §3)
- **The `--admin` escape hatch remains available** on the attended / explicit-id path. (spec §2)
- **"approved ⇒ eligible"** in the finalize Selection matrix and the gate's APPROVED-satisfies-merge prose must survive the removal **unchanged**. (spec §6.3, change body Open questions)
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration. Applies to every pruned test file. (AGENTS.md; spec §7 "vacuous test pruning")
- **When a change invalidates a test's premise, ask what the block GUARDS, not what it asserts** — DELETE a guard whose subject is gone; NARROW a guard whose property still holds. (learnings: `test-premise-deleted-not-regated`)
- **Never hand-list the sites of a literal you are gating** — derive them from a whole-repo grep. (AGENTS.md)
- **Run the whole suite at the build gate**, never only the tests this plan enumerates. (AGENTS.md)
- **Never `producer | early-exiting-consumer` under `set -o pipefail`** — capture into a variable first, then `grep <<<"$var"`. (AGENTS.md)
- **Acceptance grep (spec §7):** after removal, `grep -rniE 'auto.?approve|docket-approve|FINALIZE_AUTO_APPROVE|setup-auto-approve'` across the repo returns **only** historical provenance — `docs/adrs/0042-*`, `docs/changes/archive/*`, `docs/superpowers/specs/*`, `docs/superpowers/plans/*`, `docs/results/*` — and no live script, skill, config, workflow, or test path.
- **ADR ids:** the highest ADR id on `metadata_branch` is 0042, so the new reversing ADR mints as **ADR-0043**. Confirm at authoring time; do not hardcode blind.

## File Structure

**Deleted (7 files)**
- `.github/workflows/docket-approve.yml` — the installed bot-approval workflow. Note: this is the repo's **only** workflow; `.github/workflows/` ends up empty.
- `scripts/templates/docket-approve.yml` — its static template.
- `scripts/setup-auto-approve.sh` + `scripts/setup-auto-approve.md` — the one-time installer and its contract. Deleted **as a pair**: `tests/test_script_contracts_coverage.sh` globs `scripts/*.sh` ↔ `scripts/*.md` and would flag an orphan either way.
- `docs/auto-approve-setup.md` — the setup guide; its salvageable branch-protection content moves into `README.md`.
- `tests/test_setup_auto_approve.sh`, `tests/test_docket_approve_template.sh`, `tests/test_auto_approve_docs.sh` — subjects gone ⇒ deleted, not re-gated.

**Modified (8 files)**
- `scripts/docket.sh` — drop `setup-auto-approve` from the usage comment and the `WRAPPED_OPS` allowlist.
- `scripts/docket.md` — drop its wrapped-ops contract row.
- `.docket.yml` — drop the `finalize.auto_approve` key + comment block.
- `scripts/docket-config.sh` — drop the fence-loop token, the parse/validate block, and the `emit FINALIZE_AUTO_APPROVE` line.
- `scripts/docket-config.md` — drop the classification row, the fence-list mention, the `FINALIZE_AUTO_APPROVE` export-inventory line, and the exit-code row.
- `skills/docket-finalize-change/SKILL.md` — delete gate step 6, simplify step 7, drop the YAML row, the ADR-0042 paragraph, and the auto_approve abort-and-report clause.
- `README.md` — replace the `auto_approve` section with the three-part finalize/merge documentation.
- `tests/test_docket_config.sh`, `tests/test_finalize_gate.sh`, `tests/test_docket_facade.sh` — prune `auto_approve` assertions only.

**Created (1 file)**
- `tests/test_readme_finalize_docs.sh` — the README doc-sentinel.

**Metadata branch (not the feature branch)**
- `docs/adrs/0043-<slug>.md` (new, Accepted, `reverses: [42]`), `docs/adrs/0042-auto-approve-consent-model.md` (`status:` line only), `docs/adrs/README.md` (re-rendered) — all authored via the `docket-adr` subagent on `metadata_branch`, per Task 6.

---

### Task 1: Delete the workflow, template, setup script, and facade op

**Files:**
- Delete: `.github/workflows/docket-approve.yml`
- Delete: `scripts/templates/docket-approve.yml`
- Delete: `scripts/setup-auto-approve.sh`
- Delete: `scripts/setup-auto-approve.md`
- Delete: `tests/test_setup_auto_approve.sh`
- Delete: `tests/test_docket_approve_template.sh`
- Modify: `scripts/docket.sh:26` (usage comment), `scripts/docket.sh:38` (`WRAPPED_OPS`)
- Modify: `scripts/docket.md:57` (contract row)
- Test: `tests/test_docket_facade.sh:16,30-32` (prune), `tests/test_script_contracts_coverage.sh` (no edit — auto-discovers)

**Interfaces:**
- Consumes: nothing from earlier tasks (first task).
- Produces: a `WRAPPED_OPS` value with 13 ops (was 14), `setup-auto-approve` absent. Later tasks do not depend on this.

- [ ] **Step 1: Prune the facade sentinel's auto-approve assertions (the failing-first edit)**

`tests/test_docket_facade.sh` cross-checks `docket.sh`'s `WRAPPED_OPS` against `docket.md`'s table. It will redden the moment either side drops the op, so prune the test first, watch it fail against the *unmodified* scripts, then fix the scripts.

In `tests/test_docket_facade.sh`, remove `setup-auto-approve` from the stub-helper loop at line 16:

```bash
for h in docket-status board-refresh archive-change terminal-publish cleanup-feature-branch \
         github-mirror sync-integration-branch render-change-links render-adr-index \
         adr-checks board-checks docket-config; do
```

And delete the routing block at lines 30-32 in full:

```bash
out="$(SCRIPTS_DIR="$stub" bash "$FACADE" setup-auto-approve --integration-branch main 2>/dev/null)"
assert "setup-auto-approve routes to its helper with args" \
  '[ "$out" = "CALLED setup-auto-approve --integration-branch main" ]'
```

Leave every other assertion untouched — the `board-refresh`, `archive-change`, exit-code-passthrough, and `WRAPPED_OPS`-vs-`docket.md` sentinel blocks all guard surviving behavior.

- [ ] **Step 2: Run the facade test to verify it fails**

Run: `bash tests/test_docket_facade.sh`
Expected: FAIL — the `WRAPPED_OPS` ↔ `docket.md` sentinel now reports `setup-auto-approve` as declared-but-not-expected (the scripts still carry it, the test no longer does).

- [ ] **Step 3: Delete the six files**

```bash
git rm .github/workflows/docket-approve.yml \
       scripts/templates/docket-approve.yml \
       scripts/setup-auto-approve.sh \
       scripts/setup-auto-approve.md \
       tests/test_setup_auto_approve.sh \
       tests/test_docket_approve_template.sh
```

`tests/test_setup_auto_approve.sh` and `tests/test_docket_approve_template.sh` are deleted rather than re-gated: their **subject** (the installer, the template file) is gone, so there is no surviving property to narrow them onto.

- [ ] **Step 4: Remove the facade op from `scripts/docket.sh`**

Delete the usage line at `scripts/docket.sh:26`:

```
#   setup-auto-approve        one-time, human-attended install of the auto-approve workflow + repo setting
```

And drop the trailing token from `WRAPPED_OPS` (line 38), leaving:

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks runner-dispatch"
```

- [ ] **Step 5: Remove the contract row from `scripts/docket.md`**

Delete line 57 in full:

```
| `setup-auto-approve` | `setup-auto-approve.sh` | one-time, human-attended install of the auto-approve workflow onto the integration branch + the repo Actions setting (change 0062) |
```

- [ ] **Step 6: Run the affected tests to verify they pass**

Run: `bash tests/test_docket_facade.sh && bash tests/test_script_contracts_coverage.sh`
Expected: both PASS. `test_script_contracts_coverage.sh` needs no edit — it globs `scripts/*.sh` and `scripts/*.md` and the pair was deleted together (learnings: `check-plumbing-auto-discovery` — check whether plumbing auto-discovers before planning an edit to it).

- [ ] **Step 7: Mutation-test the pruned facade sentinel**

The pruned file must still redden on a real regression of what remains. Temporarily drop a *surviving* op from `WRAPPED_OPS`, confirm red, then restore:

```bash
sed -i.bak 's/ runner-dispatch"/"/' scripts/docket.sh
bash tests/test_docket_facade.sh   # expect: NOT OK (exit 1) — the sentinel still bites
mv scripts/docket.sh.bak scripts/docket.sh
bash tests/test_docket_facade.sh   # expect: all ok (exit 0)
```

Expected: red then green. If the mutation stays green the prune gutted the sentinel — stop and fix before committing.

- [ ] **Step 8: Commit**

```bash
git add -A .github scripts tests
git commit -m "refactor(0095): delete the docket-approve workflow, template, installer, and facade op

Change 0062's bot-approval mechanism is retired: the classifier soft-denies
the gh workflow run dispatch it depends on, so the chain can never complete.
Deletes the installed workflow + static template, setup-auto-approve.sh and
its contract, the setup-auto-approve facade op, and the two tests whose
subject no longer exists. Prunes the auto-approve arm from the facade
sentinel (mutation-tested: still reddens on a dropped surviving op)."
```

---

### Task 2: Remove the `finalize.auto_approve` config knob and resolver export

**Files:**
- Modify: `.docket.yml:25-28`
- Modify: `scripts/docket-config.sh:169` (fence loop), `:195-205` (parse+validate), `:379` (emit)
- Modify: `scripts/docket-config.md:108` (classification row), `:154` (fence list), `:255` (export inventory), `:299` (exit-code row)
- Test: `tests/test_docket_config.sh:54` (prune), `:636-660` (delete section)

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `docket-config.sh --export` no longer emits `FINALIZE_AUTO_APPROVE`. Task 3 (skill prose) and Task 4 (README) describe the config surface this task removes, so they must not reintroduce the key name as a live knob.

- [ ] **Step 1: Delete the `auto_approve` assertions from `tests/test_docket_config.sh`**

Remove the default assertion at line 54:

```bash
assert "absent cfg: FINALIZE_AUTO_APPROVE default false" '[ "$FINALIZE_AUTO_APPROVE" = false ]'
```

And delete the whole change-0062 section (lines 636-660), from the banner through the last fence assertion:

```bash
# ============================================================================
# Change 0062 — finalize.auto_approve: coordination-key fenced, repo-committed only
# ============================================================================
...
assert "fence: global finalize.auto_approve warns on stderr" \
  'rung "$gx" "$tmp/aafence" --export 2>&1 >/dev/null | grep -qi "auto_approve.*per-repo-only"'
```

**Do not touch** the adjacent change-0064 `terminal_publish` block immediately above it (lines 630-633) — `terminal_publish` is a *surviving* fenced key and those assertions are the coordination-key fence's remaining coverage. Deleting the `auto_approve` fence assertions is safe precisely because `terminal_publish` still exercises the same fence loop.

- [ ] **Step 2: Run the config test to verify it fails**

Run: `bash tests/test_docket_config.sh`
Expected: PASS at this point (removing assertions cannot redden the file). This is the one task whose test edit is subtractive-only — the real verification is Step 5's mutation test plus the Step 4 grep. Note the ordering honestly rather than inventing a fake red.

- [ ] **Step 3: Remove the knob from `scripts/docket-config.sh`**

Drop `auto_approve` from the coordination-key fence loop (line 169):

```bash
for _fkey in metadata_branch integration_branch changes_dir adrs_dir results_dir github_project terminal_publish; do
```

Delete the comment block and parse/validate lines 195-205 in full:

```bash
# change 0062: finalize.auto_approve — coordination-key fenced (ADR-0019), like terminal_publish.
# ... (through) ...
FINALIZE_AUTO_APPROVE="$(yaml_get "$CFG" auto_approve)"; FINALIZE_AUTO_APPROVE="${FINALIZE_AUTO_APPROVE:-false}"
case "$FINALIZE_AUTO_APPROVE" in
  true|false) ;;
  *) die "unparseable .docket.yml: finalize.auto_approve must be 'true' or 'false', got '$FINALIZE_AUTO_APPROVE'" ;;
esac
```

Leave lines 193-194 (`FINALIZE_GATE`, `FINALIZE_TEST_COMMAND`) and line 206 onward (`AUTO_GROOM`, `TERMINAL_PUBLISH`) exactly as they are. Delete the emit line at 379:

```bash
  emit FINALIZE_AUTO_APPROVE "$FINALIZE_AUTO_APPROVE"
```

- [ ] **Step 4: Remove the knob from `.docket.yml`**

Delete lines 25-28 — the key and its full comment block:

```yaml
  auto_approve: true     # true => headless finalize dispatches .github/workflows/docket-approve.yml
  #                      #   (install once via `docket.sh setup-auto-approve`) to approve the PR, then
  #                      #   merges WITHOUT --admin. Coordination-key fenced (per-repo-only). See
  #                      #   docs/auto-approve-setup.md.
```

The `finalize:` block keeps `gate: local`, the commented `test_command:`, and the commented `require_pr_approval:` lines unchanged. This repo had the knob set to `true`, so this is a live behavior removal for docket itself, not just a documentation cleanup.

- [ ] **Step 5: Remove the four references from `scripts/docket-config.md`**

Delete the classification-table row (line 108), the `finalize.auto_approve` mention in the fence list (line 154 — remove only that token, keep the rest of the sentence and the other fenced keys), the `FINALIZE_AUTO_APPROVE` line from the export inventory (line 255), and the exit-code row (line 299):

```
| `finalize.auto_approve` is neither `true` nor `false` | 1 |
```

- [ ] **Step 6: Run the tests and verify the export is gone**

```bash
bash tests/test_docket_config.sh
bash scripts/docket-config.sh --export | grep -c AUTO_APPROVE   # expect: 0
bash scripts/docket-config.sh --export | grep -E 'FINALIZE_(GATE|TEST_COMMAND)'  # expect: both still present
```

Expected: test PASSes, `AUTO_APPROVE` count is `0`, and `FINALIZE_GATE` / `FINALIZE_TEST_COMMAND` still emit — proving the removal was surgical.

- [ ] **Step 7: Mutation-test the pruned config test**

```bash
sed -i.bak 's/TERMINAL_PUBLISH:-false/TERMINAL_PUBLISH:-true/' scripts/docket-config.sh
bash tests/test_docket_config.sh   # expect: NOT OK — the surviving fenced-key coverage still bites
mv scripts/docket-config.sh.bak scripts/docket-config.sh
bash tests/test_docket_config.sh   # expect: all ok
```

Expected: red then green. (If the `TERMINAL_PUBLISH` default is spelled differently in the file, mutate whatever the surviving default actually is — the point is that a real regression of a *remaining* fenced key still reddens.)

- [ ] **Step 8: Commit**

```bash
git add .docket.yml scripts/docket-config.sh scripts/docket-config.md tests/test_docket_config.sh
git commit -m "refactor(0095): remove the finalize.auto_approve knob and its resolver export

Drops the key from .docket.yml (this repo had it enabled), the fence-loop
token, the parse/validate block, and the FINALIZE_AUTO_APPROVE export, plus
the four docket-config.md contract references. finalize.gate and
finalize.test_command parsing are untouched; terminal_publish remains the
coordination-key fence's live coverage (mutation-tested)."
```

---

### Task 3: Simplify the finalize skill's gate prose

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md:81-85` (YAML row), `:90` (ADR-0042 paragraph), `:102-115` (steps 6-7), `:131` (abort-and-report set)
- Test: `tests/test_finalize_gate.sh:178-188` (prune), `tests/test_skill_size_budgets.sh:23` (verify, likely no edit)

**Interfaces:**
- Consumes: Task 2's removal of the `auto_approve` config key — this task's prose must not describe a knob that no longer resolves.
- Produces: a gate flow of **6** numbered steps (was 7), with merge as step 6. No later task depends on the numbering.

- [ ] **Step 1: Prune the auto_approve assertions from `tests/test_finalize_gate.sh`**

Delete the five-assertion block at lines 178-188 in full (the `# --- auto_approve merge path (change 0062) ---` comment through the `never an --admin fallback` assertion).

**Keep** the assertion immediately following it — `publish degradation: terminal_publish headless push denial degrades, not fails` — and every assertion above line 178. Two of the deleted assertions are worth a second look before deleting, per `test-premise-deleted-not-regated`:

- `auto_approve merges WITHOUT --admin` greps for `without .*--admin|no .*--admin|not .*--admin`. Its *subject* is the auto_approve path, which is gone — but the surviving prose still says the merge runs without `--admin` on the approved path. Delete this assertion here and re-establish the surviving property as a fresh, correctly-named assertion in Step 2 rather than leaving a block whose name lies about what it guards.
- `auto_approve re-checks reviewDecision == APPROVED` greps for `reviewDecision` + `APPROVED`. Those tokens survive in the `require_pr_approval` prose, so this assert would stay **green for the wrong reason** — a classic vacuous survivor. Delete it; Step 2 replaces it with an assertion anchored on `require_pr_approval`.

- [ ] **Step 2: Add the replacement assertions for the surviving properties**

Append to `tests/test_finalize_gate.sh`, immediately before the `publish degradation` assertion:

```bash
# --- merge authorization after 0095 (auto_approve retired) ---
assert "0095: no live auto_approve/docket-approve reference in the finalize skill" \
  '! grep -Eqi "auto_approve|docket-approve|setup-auto-approve" "$FIN"'
assert "0095: require_pr_approval still gates the auto-detect path on APPROVED" \
  'grep -q "require_pr_approval" "$FIN" && grep -q "reviewDecision" "$FIN" && grep -q "APPROVED" "$FIN"'
assert "0095: an approved PR merges without --admin" \
  'grep -Eqi "without .*--admin|no .*--admin|not .*--admin" "$FIN"'
assert "0095: --admin survives only as the explicit-id / attended escape hatch" \
  'grep -Eqi "explicit[- ]id|attended" "$FIN" && grep -q -- "--admin" "$FIN"'
```

Note the `grep -q -- "--admin"` form: a pattern leading with `--` must declare it or it parses as an option (AGENTS.md).

- [ ] **Step 3: Run the test to verify it fails**

Run: `bash tests/test_finalize_gate.sh`
Expected: FAIL on `0095: no live auto_approve/docket-approve reference in the finalize skill` — the SKILL.md still carries the step-6 prose. The other three should already pass.

- [ ] **Step 4: Remove the `auto_approve` row from the YAML block**

In `skills/docket-finalize-change/SKILL.md`, delete lines 81-85 (the `auto_approve:` row and its four continuation comment lines), leaving:

```yaml
finalize:
  gate: local                 # local (default) | ci | both | off
  test_command:               # OPTIONAL override; unset => the agent auto-detects the suite
  require_pr_approval: false  # default false. true => the auto-detect path refuses to merge
                              #   an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

- [ ] **Step 5: Rewrite the `require_pr_approval` paragraph**

Replace line 90 in full. The current text explains how a bot approval satisfies the gate under `auto_approve`; with the subsystem gone, `require_pr_approval` reverts to its pure ADR-0011 meaning (spec §3). New text:

```markdown
`require_pr_approval` validates *human sign-off* (`gate` validates *correctness*); it governs only the auto-detect path — an explicit id always overrides it. `true` ⇒ the auto-detect path refuses to merge a PR whose `reviewDecision` is not `APPROVED`, surfacing it instead. The approval must come from a **human** reviewer: a co-maintainer, or the maintainer running finalize if they are an eligible reviewer on someone else's PR. See ADR-0011 for the consent model, and ADR-0043 for why the bot-approval mechanism that once satisfied this gate was retired.
```

- [ ] **Step 6: Delete gate step 6 and simplify step 7**

Delete the entire step 6 block (lines 102-112, `**Approve, if finalize.auto_approve is true...**` through `...this step is a no-op.`) and renumber the merge step from 7 to 6, replacing lines 113-115 with:

```markdown
6. `gh pr merge` — **without** `--admin` whenever the PR is already `APPROVED`, or the integration
   branch's protection requires a pull request but **zero** approvals (docket's single-maintainer
   default — see the README's finalize/merge section). `--admin` remains available only on the
   pre-existing explicit-id / attended paths, where a sole maintainer chooses to force past an
   otherwise-unsatisfiable required review → the existing close-out (harvest → archive →
   terminal-publish → cleanup → board).
```

- [ ] **Step 7: Trim the abort-and-report set**

At line 131, delete only the trailing auto_approve clause — ` · under auto_approve, a rejected dispatch, a failed/timed-out run, or an approval that never materializes (never an --admin fallback)` — so the sentence ends at `· any repair under **autonomous** finalize (sign-off).` Every other abort-and-report point stays.

- [ ] **Step 8: Verify the Selection matrix survived unchanged**

Run: `grep -n -iE 'approved|selection' skills/docket-finalize-change/SKILL.md`
Expected: the Selection matrix's "approved ⇒ eligible" behavior is present and untouched. This is the spec's explicit Open-questions requirement (§6.3) — confirm by reading, not just grepping.

- [ ] **Step 9: Run the tests to verify they pass**

```bash
bash tests/test_finalize_gate.sh
bash tests/test_skill_size_budgets.sh
```

Expected: both PASS. The size budget for `skills/docket-finalize-change/SKILL.md` is `160` lines / `2699` words and this task only *removes* prose, so no budget raise is needed. If the file somehow grew past a bound, lower or raise the row in this same diff (the guard permits an in-diff edit) — do not leave it failing.

- [ ] **Step 10: Mutation-test the pruned gate test**

```bash
sed -i.bak 's/require_pr_approval/require_pr_approvalX/g' skills/docket-finalize-change/SKILL.md
bash tests/test_finalize_gate.sh   # expect: NOT OK
mv skills/docket-finalize-change/SKILL.md.bak skills/docket-finalize-change/SKILL.md
bash tests/test_finalize_gate.sh   # expect: all ok
```

Expected: red then green.

- [ ] **Step 11: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md tests/test_finalize_gate.sh
git commit -m "refactor(0095): drop the finalize gate's approve step, restore require_pr_approval

Deletes gate step 6 (bot approval) and the auto_approve YAML row, and
simplifies merge to: no --admin when the PR is APPROVED or protection
requires 0 approvals; --admin only on the explicit-id / attended path.
require_pr_approval reverts to its ADR-0011 human-sign-off meaning. The
pruned assertions are replaced by four anchored on the surviving
properties (the old reviewDecision assert would have stayed green for the
wrong reason)."
```

---

### Task 4: Replace the README documentation and add the doc-sentinel

**Files:**
- Delete: `docs/auto-approve-setup.md`
- Delete: `tests/test_auto_approve_docs.sh`
- Modify: `README.md:580-590` (replace the section)
- Create: `tests/test_readme_finalize_docs.sh`

**Interfaces:**
- Consumes: Tasks 2 and 3 — the README must document the *post-removal* world (no knob, no installer).
- Produces: a README section whose load-bearing phrases the new sentinel keys on. Task 6's final grep depends on `docs/auto-approve-setup.md` being gone.

- [ ] **Step 1: Write the failing doc-sentinel**

Create `tests/test_readme_finalize_docs.sh`. It keys on load-bearing *content*, not on section titles, and includes a negative assertion so a future re-introduction of the dead machinery is caught:

```bash
#!/usr/bin/env bash
# tests/test_readme_finalize_docs.sh — doc-sentinel for the finalize/merge documentation
# (change 0095). Guards that README documents (a) the Claude Code auto-mode classifier
# behavior as the reason the bot-approval approach failed, (b) the single-maintainer
# branch-protection recipe, and (c) the preserved human-approval path for repos that
# require reviews. Run: bash tests/test_readme_finalize_docs.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
RM="$ROOT/README.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "README exists" '[ -f "$RM" ]'

# (a) the classifier behavior — named, and tied to what it blocks
assert "documents the auto-mode classifier" \
  'grep -qi "classifier" "$RM"'
assert "names the soft-deny the classifier applies" \
  'grep -qi "soft.deny\|soft deny" "$RM"'
assert "states an allow-rule cannot clear a soft-deny" \
  'grep -Eqi "permissions.allow|allow.rule" "$RM"'
assert "scopes the observation to mode and version" \
  'grep -Eqi "version.*(scoped|specific)|mode.*(scoped|specific)|headless" "$RM"'

# (b) the single-maintainer branch-protection recipe
assert "documents the branch-protection recipe" \
  'grep -qi "branch protection" "$RM"'
assert "names the zero-approvals setting" \
  'grep -Eq "required_approving_review_count|zero approvals|0 approvals" "$RM"'
assert "states the merge needs no --admin" \
  'grep -Eqi "without .*--admin|no .*--admin" "$RM"'

# (c) the preserved human-approval path
assert "documents the human-approval path for approval-required repos" \
  'grep -q "require_pr_approval" "$RM" && grep -q "APPROVED" "$RM"'

# negative: the retired subsystem must not come back as live documentation
assert "no live auto-approve subsystem reference" \
  '! grep -Eqi "auto_approve|setup-auto-approve|auto-approve-setup.md|docket-approve.yml" "$RM"'
assert "the deleted setup guide is not linked" \
  '[ ! -f "$ROOT/docs/auto-approve-setup.md" ]'

exit $fail
```

- [ ] **Step 2: Run the sentinel to verify it fails**

Run: `bash tests/test_readme_finalize_docs.sh`
Expected: FAIL on the classifier, recipe, human-path, and both negative assertions — the README still carries the old `auto_approve` section and `docs/auto-approve-setup.md` still exists.

- [ ] **Step 3: Replace the README section**

In `README.md`, replace the entire `### Headless / autonomous finalize merge auto-approve (opt-in)` section (lines 580-590, through the `docs/auto-approve-setup.md` link paragraph, up to but not including the `---` before `## Status`) with:

```markdown
### Hands-off finalize — what blocks it, and the recipe that works

**The Claude Code auto-mode classifier.** In interactive auto-mode, Claude Code's permission
classifier *soft-denies* capability-granting and merge-adjacent `gh` actions — notably
`gh workflow run`, and `gh pr merge` on an unreviewed PR (occasionally even a post-merge
`gh pr view`). A soft-deny is a model-side judgment, not a permission lookup: a
`permissions.allow` entry **cannot** clear it. The behavior is scoped to the harness **mode**
and **version** it was observed in — headless and interactive diverge, on the same repo, on the
same day — so treat any statement about it as an observation with an expiry date, not a fact.

This is precisely why docket's earlier bot-approval design (change 0062, ADR-0042) failed: its
very first step was a `gh workflow run` dispatch, which is exactly what gets denied. That
subsystem is retired — see ADR-0043.

**Single-maintainer hands-off finalize (the recipe).** Configure branch protection on the
integration branch to **require a pull request** but require **zero** approvals
(`required_approving_review_count: 0`; leave `enforce_admins` off). A solo maintainer cannot
approve their own PR, so a nonzero requirement is structurally unsatisfiable — but with zero
required approvals, `docket-finalize-change` runs its rebase-retest gate and then merges via a
plain `gh pr merge --rebase`: **no `--admin`, no bot, and nothing for the classifier to deny.**
Changing the real state of the external system beats arguing with the guard.

**Repos that require approvals (human sign-off preserved).** With
`required_approving_review_count >= 1`, a human approves the PR on GitHub — a co-maintainer, or
the maintainer running finalize when they are an eligible reviewer. That makes
`reviewDecision: APPROVED` satisfy both branch protection and `require_pr_approval: true`, and
finalize merges with **no `--admin`**. The attended, explicit-id `--admin` path remains the
escape hatch when a sole maintainer deliberately forces past an unsatisfiable required review.
```

- [ ] **Step 4: Delete the setup guide and its test**

```bash
git rm docs/auto-approve-setup.md tests/test_auto_approve_docs.sh
```

`test_auto_approve_docs.sh` is deleted, not re-gated: every one of its assertions has the deleted guide or the deleted knob as its subject. Its *purpose* — "a config surface is shipped end-to-end in docs" (learnings: `config-knob-ship-end-to-end`) — is what `tests/test_readme_finalize_docs.sh` now carries forward for the replacement documentation.

- [ ] **Step 5: Run the sentinel to verify it passes**

Run: `bash tests/test_readme_finalize_docs.sh`
Expected: PASS (all `ok -` lines, exit 0).

- [ ] **Step 6: Mutation-test the new sentinel (non-vacuity)**

The spec requires this sentinel to guard **non-vacuously**. Strip each documented thing in turn and confirm a redden:

```bash
cp README.md /tmp/README.bak
# mutation 1: remove the zero-approvals recipe
sed -i.bak 's/required_approving_review_count: 0/X/; s/zero. approvals/X/g' README.md
bash tests/test_readme_finalize_docs.sh   # expect: NOT OK on the zero-approvals assert
cp /tmp/README.bak README.md
# mutation 2: remove the classifier discussion
sed -i.bak 's/classifier/XXX/g; s/soft-denies/XXX/g' README.md
bash tests/test_readme_finalize_docs.sh   # expect: NOT OK on the classifier asserts
cp /tmp/README.bak README.md
# mutation 3: reintroduce the retired subsystem
printf '\nauto_approve: true\n' >> README.md
bash tests/test_readme_finalize_docs.sh   # expect: NOT OK on the negative assert
cp /tmp/README.bak README.md
bash tests/test_readme_finalize_docs.sh   # expect: all ok
rm -f README.md.bak /tmp/README.bak
```

Expected: red on each of the three mutations, green after restore. A mutation that stays green means that assertion is decoration — fix it before committing.

- [ ] **Step 7: Commit**

```bash
git add README.md tests/test_readme_finalize_docs.sh
git add -A docs tests
git commit -m "docs(0095): document the classifier wall and the branch-protection recipe

Replaces the auto_approve README section with three things the next
maintainer would otherwise re-derive the hard way: what the Claude Code
auto-mode classifier soft-denies (and that an allow-rule cannot clear it,
and that the observation is mode/version-scoped), the single-maintainer
recipe (require a PR, 0 approvals => plain gh pr merge --rebase, no
--admin), and the preserved human-approval path. Deletes the setup guide
and its test; adds tests/test_readme_finalize_docs.sh as the doc-sentinel
(mutation-tested against three separate strips)."
```

---

### Task 5: Whole-repo dangling-reference sweep and full suite

**Files:**
- Test: all of `tests/test_*.sh`
- Modify: any file the sweep turns up (unknown until run — that is the point)

**Interfaces:**
- Consumes: Tasks 1-4 complete.
- Produces: a clean acceptance grep, which Task 6's ADR work and the PR both rest on.

- [ ] **Step 1: Run the acceptance grep**

Per AGENTS.md, derive the sites from a whole-repo grep rather than hand-listing them:

```bash
grep -rniE 'auto.?approve|docket-approve|FINALIZE_AUTO_APPROVE|setup-auto-approve' . \
  --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=.docket
```

Expected: **only** historical provenance — `docs/adrs/0042-auto-approve-consent-model.md`, `docs/changes/archive/2026-07-17-0062-*.md`, `docs/superpowers/specs/2026-07-16-*`, `docs/superpowers/plans/2026-07-16-*`, `docs/results/2026-07-16-*`, and this change's own spec/plan. Sort every hit into *prose provenance* vs *executable*; **any** live script, skill, config, workflow, or test hit is a defect to fix now.

- [ ] **Step 2: Confirm `.github/workflows/` is empty or gone**

Run: `ls -la .github/workflows/ 2>&1`
Expected: empty directory or no such directory — `docket-approve.yml` was the repo's only workflow. If an empty directory remains, leave it (git does not track empty directories; it simply disappears from the tree).

- [ ] **Step 3: Run the whole suite**

```bash
for t in tests/test_*.sh; do
  printf '\n===== %s =====\n' "$t"
  bash "$t" || echo "FAILED: $t"
done
```

Expected: every test file exits 0 and no `FAILED:` line is printed. Run the **whole** suite, not just the files this plan touched (AGENTS.md) — the removal crossed `docket.sh`, the config resolver, and a skill body, all of which other tests read.

- [ ] **Step 4: Fix anything the suite turns up, then re-run**

If any test reddens, fix the cause rather than the assertion, and re-run the whole suite until clean. A test that reddens because its *premise* was deleted by this change gets deleted; one whose *property* survives gets narrowed (learnings: `test-premise-deleted-not-regated`).

- [ ] **Step 5: Commit (only if Steps 1-4 required fixes)**

```bash
git add -A
git commit -m "chore(0095): clear residual auto-approve references; full suite green"
```

If nothing needed fixing, skip the commit — do not create an empty one.

---

### Task 6: The reversing ADR (metadata branch — not the feature branch)

**Files:**
- Create (metadata branch): `docs/adrs/0043-<slug>.md`
- Modify (metadata branch): `docs/adrs/0042-auto-approve-consent-model.md` — `status:` line **only**
- Modify (metadata branch): `docs/adrs/README.md` — re-rendered, never hand-edited
- Modify (metadata branch): `.docket/docs/changes/active/0095-retire-auto-approve-workflow.md` — `adrs:` field

**Interfaces:**
- Consumes: Tasks 1-5 — the ADR records a decision whose implementation is already on the branch.
- Produces: the ADR number that Task 6's `adrs:` write and the PR body reference.

> **This task writes nothing on the feature branch.** ADRs live on `metadata_branch` (the `docket` branch, via the `.docket/` worktree). Do **not** create `docs/adrs/0043-*.md` in the feature worktree — the copies visible on `main` arrive only via `terminal-publish` at close-out.

- [ ] **Step 1: Dispatch the `docket-adr` subagent**

Dispatch it foreground, at the model/effort its wrapper resolves, to author the new ADR per spec §5:

- **Title/slug:** something like *"Retire bot auto-approval — branch protection with zero required approvals is the single-maintainer merge path"*.
- **Frontmatter:** `status: Accepted`, `date: 2026-07-18`, `reverses: [42]`, `relates_to: [11]`, `change: 95`.
- **Context:** the `docket-approve.yml` dispatch is classifier-blocked in practice (2026-07-18, the change-0088 finalize); a chain whose first step is denied can never complete. Record the mode and version the observation came from — an unscoped claim about harness behavior will be read later as universal and be wrong.
- **Decision:** retire change 0062's bot-approval mechanism entirely. The single-maintainer hands-off merge path is branch-protection configuration (`required_approving_review_count: 0`, `enforce_admins: false`) → `gh pr merge --rebase`, no `--admin`, no bot, no dispatch. Removing `auto_approve` restores `require_pr_approval: true` to its ADR-0011 "a human authorized the merge" meaning.
- **Consequences:** *enables* a genuinely working one-swoop finalize for the single-maintainer default with far less machinery; *costs* the "required review satisfied **and** recorded as an approval" property — a 0-required-approvals repo merges with no recorded review, which is acceptable and explicit for a solo maintainer. Team repos keep `require_pr_approval: true` with a real human reviewer. Classifier behavior is mode/version-scoped, so this is an empirical decision, not a permanent contract. Sets `ADR-0042.status: Reversed by ADR-0043`.

The subagent assigns the number, flips ADR-0042's `status:` line (its body is immutable and stays as the historical record of why the bot approach was tried), re-renders `docs/adrs/README.md`, and commits on `origin/docket`. Record the number it returns.

- [ ] **Step 2: Re-sync the metadata tree and verify the ADR landed**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight
ls /Users/homer/dev/docket/.docket/docs/adrs/ | tail -3
grep -n '^status:' /Users/homer/dev/docket/.docket/docs/adrs/0042-auto-approve-consent-model.md
```

Expected: `0043-<slug>.md` present, and ADR-0042's status reads `Reversed by ADR-0043`. Verify the child's **git state**, never a bare "completed" return.

- [ ] **Step 3: Write both ADR ids into the change's `adrs:` field**

In the **metadata working tree**, set the change's `adrs:` to include **both** 42 and 43:

```yaml
adrs: [42, 43]
```

Listing **42** is load-bearing, not decorative: `terminal-publish` re-copies an ADR onto the integration branch only if it is named in the producing change's `adrs:`, so this is what delivers ADR-0042's flipped `status:` line atomically with the new ADR rather than as a racing standalone push (learnings: `adr-update-delivery`).

Then regenerate the Artifacts block in the same commit, per the field-write rule:

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh render-change-links \
  --change-file .docket/docs/changes/active/0095-retire-auto-approve-workflow.md \
  --adrs-dir .docket/docs/adrs
```

- [ ] **Step 4: Record the close-out trap for the human merge gate**

⚠️ **This must reach the results file and the PR body.** `terminal-publish.sh`'s change-mode copy-set applies an **Accepted gate** (`scripts/terminal-publish.sh:141`): it publishes an ADR named in `adrs:` only if that ADR's `status:` is `Accepted` on the metadata branch. ADR-0042 will read `Reversed by ADR-0043` — **not** Accepted — so the gate will silently **skip** it (`log "adr 42: not Accepted; skipped by gate"`), and `main` will keep showing ADR-0042 as `Accepted` alongside a stale `docs/adrs/README.md` row.

The remedy is ADR-mode publish, which has **no** Accepted gate, run once after close-out:

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish \
  --adr 42 --integration-branch main --metadata-branch docket \
  --changes-dir docs/changes --adrs-dir docs/adrs --enabled true
```

Do not run this now — it belongs after the PR merges. Record it as an explicit merge-gate step so the human does not have to rediscover it.

- [ ] **Step 5: Commit and push the metadata write**

```bash
cd /Users/homer/dev/docket/.docket
git add docs/changes/active/0095-retire-auto-approve-workflow.md
git commit -m "docket(0095): adrs 42, 43 — reversal recorded, 42 relisted for atomic republish"
git push origin HEAD:docket
```

Then SHA-compare local vs `origin/docket` to confirm the push landed (learnings: `no-checkout-in-shared-worktree` — after every push, SHA-compare).

---

## Self-Review

**1. Spec coverage.**

| Spec section | Covered by |
|---|---|
| §4 workflow + template | Task 1 |
| §4 setup script + facade + docs | Task 1 (script/facade), Task 4 (`docs/auto-approve-setup.md`) |
| §4 config knob + resolver | Task 2 |
| §4 skill prose | Task 3 |
| §4 convention scrub | **Verified already clean** at plan time — `grep -rn auto_approve skills/docket-convention/` returns nothing. No task needed; Task 5's sweep re-confirms. |
| §4 ADR ledger | Task 6 |
| §4 tests (delete 3, prune 3) | Task 1 (delete 2, prune facade), Task 2 (prune config), Task 3 (prune gate), Task 4 (delete 1) |
| §4 README doc-sentinel | Task 4 |
| §5 the new ADR | Task 6 |
| §6.1 classifier docs | Task 4 Step 3 |
| §6.2 single-maintainer recipe | Task 4 Step 3 |
| §6.3 approval-required path + Selection-matrix confirmation | Task 4 Step 3; Task 3 Step 8 |
| §7 vacuous pruning risk | Mutation-test steps in Tasks 1, 2, 3, 4 |
| §7 dangling references | Task 5 Step 1 |
| §7 reversed-ADR immutability | Task 6 Step 1 (status line only) |
| §7 size budgets | Task 3 Step 9 |
| §8 acceptance | Task 5 (grep + full suite), Task 6 (ADR) |

No gaps.

**2. Placeholder scan.** No TBD/TODO, no "add appropriate error handling", no "similar to Task N". Every code step shows the actual content. Two steps are deliberately open-ended by nature — Task 5 Step 4 (fix what the suite turns up) and Task 6 Step 1 (the ADR body the subagent authors) — and both state their acceptance criteria concretely rather than deferring the work.

**3. Type consistency.** File paths verified against the worktree at plan time; line numbers are from `origin/main` @ `0bd8c2f` and will drift as earlier tasks land — each task names the surrounding text so the anchor survives the drift. The `WRAPPED_OPS` value in Task 1 Step 4 is the current 14-token string minus `setup-auto-approve`, spelled out in full. `FIN` and `$RM` are the existing test files' own variable names.

**One addition beyond the spec's letter:** Task 6 Step 4 (the `terminal-publish --adr 42` close-out trap). The spec assumes flipping ADR-0042's status is sufficient; reading `scripts/terminal-publish.sh:141` shows the change-mode Accepted gate would silently skip it, leaving `main` inconsistent. Added as an explicit merge-gate step rather than left to be rediscovered.
