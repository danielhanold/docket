# Facade skill rewiring — retire the eval preamble Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewire the seven operating skills + the convention's Step-0 preamble to invoke docket helpers only through the `docket.sh` facade (shipped by change 0068), retiring the `eval "$(docket-config.sh --export)"` preamble, the inline worktree/hook/fetch/pull programs, and every direct per-helper invocation — plus a mutation-tested wiring test that locks the new shapes in.

**Architecture:** Pure prose/test surgery on markdown skill files + one new shell test. No script or facade behavior changes (0068 owns the facade). Feature branch cut from `origin/main`; the change file/spec live on `origin/docket` and are NOT touched here.

**Tech Stack:** Bash (portable GNU/BSD), markdown. Test harness: the repo's `tests/*.sh` + `tests/lib` `assert` helper (follow an existing test's boilerplate, e.g. `tests/test_docket_facade.sh`).

## Global Constraints

- **Canonical facade spelling (byte-exact, from 0068 / ADR-0029):**
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op> [args...]` — `<op>` = wrapped helper basename; args unchanged.
- **The one sanctioned direct-helper carve-out (byte-exact, convention Step-0 ONLY, exactly once repo-wide):**
  `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap` (CREATE_ORPHAN, fresh-repo, human-attended).
- **Facade op inventory (grep-derive from `scripts/docket.md`'s Subcommand table — NEVER hand-list):**
  `preflight env docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index adr-checks board-checks`.
- **Helper → op map for the mechanical invocation swap:**
  `docket-config.sh --export` (eval preamble) → **`docket.sh preflight`** (Step-0 side effects + prints the block);
  `board-refresh.sh` → `docket.sh board-refresh`; `render-change-links.sh` → `docket.sh render-change-links`;
  `render-adr-index.sh` → `docket.sh render-adr-index`; `terminal-publish.sh` → `docket.sh terminal-publish`;
  `archive-change.sh` → `docket.sh archive-change`; `cleanup-feature-branch.sh` → `docket.sh cleanup-feature-branch`;
  `board-checks.sh` → `docket.sh board-checks`; `adr-checks.sh` → `docket.sh adr-checks`;
  `docket-status.sh` → `docket.sh docket-status`; `sync-integration-branch.sh` → `docket.sh sync-integration-branch`;
  `github-mirror.sh` → `docket.sh github-mirror`.
  **`disable-worktree-hooks.sh` and `render-board.sh` DROP OUT of runtime prose** (internal to `preflight` / `board-refresh`).
- **All metadata-tree SYNC instructions route through `preflight`.** Every code-span `git … fetch origin docket`, `git … pull --rebase …` (pre-read syncs AND push-retry CAS loops) becomes prose "re-run `docket.sh preflight`" (for CAS: "re-run `docket.sh preflight`, then retry the push"). Plain git plumbing (`git add`/`commit`/`push`, `git -C` forms, `git rev-parse`, feature-branch git, `gh`) STAYS direct and unrestricted.
- **Path composition (ADR-0029):** metadata paths compose `<METADATA_WORKTREE>/<CHANGES_DIR>` etc. (absolute root from the `env`/`preflight` block + repo-relative subpath); `RESULTS_DIR` composes against the feature worktree. Existing `<changes_dir>`/`.docket/` placeholder prose is unchanged by this rule — only invocation/sync SHAPES change.
- **Files in scope (live agent-facing prose):** `skills/docket-{new-change,groom-next,implement-next,status,finalize-change,adr,auto-groom}/SKILL.md`, `skills/docket-convention/SKILL.md`, `skills/docket-convention/references/terminal-close-out.md`. `skills/docket-convention/references/agent-layer.md` is verified but only rewired if grep finds a daily-helper invocation / eval / fetch / pull shape (expected: none — it carries only descriptive human-tier + `docket-config.sh` NOUNS). `skills/docket-convention/github-board-mirror.md` is OUT of the tokenizer scope and unchanged.
- **OUT of scope:** any facade/helper/script behavior; `scripts/*.md` contracts; README; all immutable artifacts (archive/specs/plans/results/ADRs). Descriptive NOUN mentions of internal scripts (e.g. `` `board-refresh.sh` `` naming the writer while describing mechanics) are PERMITTED — the tokenizer guards INVOCATIONS, not nouns (see Task 1 rationale). Do NOT change what any skill does — only how its shell surface is expressed.
- **Guards-are-code (LEARNINGS 2026-06-17→07-13):** every new assert is mutation-tested (strip the guarded clause → the test goes RED). Anchor Layer-2 presence asserts to a UNIQUE phrase the target clause owns (`grep -c == 1`), never a keyword set, never a blunt `! grep`/`grep -q` over a literal that can legitimately appear elsewhere.
- **Enumerated-set (LEARNINGS):** the site counts here are a FLOOR; derive the exact set by whole-repo grep at build time. Run the WHOLE suite at the gate, never only the enumerated tests.
- **Follow-the-call (LEARNINGS 2026-07-13 #64):** where a NEW spelling legitimately violates an OLD absolutist test anchor, NARROW that anchor to its load-bearing property — never delete or loosen it.

---

### Task 1: The wiring test `tests/test_skill_facade_wiring.sh` (Layer 1 sweep + Layer 2 anchors), proven to bite

**Files:**
- Create: `tests/test_skill_facade_wiring.sh`
- Read (for boilerplate): `tests/test_docket_facade.sh`, `tests/lib/*` (the `assert` helper)
- Read (op inventory source): `scripts/docket.md`

**Interfaces:**
- Produces: an executable test that later tasks run to verify their rewiring. Green ONLY after all in-scope prose is rewired (Tasks 2-4). This task ends with the test RED (current prose still carries old shapes) — that RED, per assert, is the reverse-mutation proof each guard bites.

**Design — two layers (sound-guard reading; rationale below the steps):**

Layer 1 (absence sweep, per in-scope file, judged over code UNITS = fenced blocks + inline spans):
extract code units → strip the two byte-exact canonical forms (`…/docket.sh` and `…/docket-config.sh --bootstrap`) → assert the remainder contains NONE of: `${DOCKET_SCRIPTS_DIR` (any surviving invocation prefix = a non-facade or non-byte-exact invocation), `eval "$(`, `fetch origin`, `pull --rebase`. Separately assert every `docket.sh <op>` uses an op ∈ the grep-derived inventory, and that `docket-config.sh --bootstrap` occurs exactly once across the in-scope set.

Layer 2 (presence anchors, `grep -c == 1` on `skills/docket-convention/SKILL.md`): the Step-0 preamble runs `preflight` as its own call and reads the block; the mid-run re-sync verb is defined; the CREATE_ORPHAN carve-out is present.

**Rationale (record verbatim as a header comment in the test — this is the load-bearing design decision):**
The sweep guards non-facade *invocations*, not every `.sh` token. Discriminator = the invocation prefix `${DOCKET_SCRIPTS_DIR` (every retired invocation — direct helper, eval preamble, disable-hooks — carries it) plus the retired shapes. Descriptive NOUN mentions (`` `board-refresh.sh` ``, `` `sync-agents.sh` ``, `` `render-board.sh` ``, `scripts/<name>.sh`) carry no prefix and are permitted — this is what lets `references/agent-layer.md` (full of descriptive `sync-agents.sh` code spans) pass the sweep without rewiring, exactly as spec §3 requires ("rewired only if grep finds old shapes"). A rule forbidding every `.sh` token would force a heavy rewrite of a reference doc whose SUBJECT is `sync-agents.sh` — contradicting spec §Out-of-scope. Stripping the canonical `…/docket.sh` before scanning is what makes the `${DOCKET_SCRIPTS_DIR` assert sound (the canonical spelling itself contains `install.sh` in its `:?` guard).

- [ ] **Step 1: Write the test file.** Use the repo's test boilerplate (source `tests/lib`, define `assert`). Content:

```bash
#!/usr/bin/env bash
# tests/test_skill_facade_wiring.sh — change 0072 (facade skill rewiring)
#
# Guards that live agent-facing skill prose invokes docket helpers ONLY through the
# docket.sh facade (canonical byte-exact spelling), that the retired shapes
# (eval "$(", inline `fetch origin`, `pull --rebase`) are gone from code spans, and that
# the Step-0 preamble + mid-run re-sync + CREATE_ORPHAN carve-out are PRESENT.
#
# SOUND-GUARD READING (load-bearing): the Layer-1 sweep guards non-facade INVOCATIONS,
# not every `.sh` token. Discriminator = the invocation prefix `${DOCKET_SCRIPTS_DIR`
# (every retired invocation carries it) + the retired shapes. Descriptive NOUN mentions
# (`board-refresh.sh`, `sync-agents.sh`, `render-board.sh`, `scripts/<name>.sh`) carry no
# prefix and are PERMITTED — this is what lets references/agent-layer.md pass without
# rewiring (spec §3), and avoids over-scoping into a reference doc about sync-agents.sh
# (spec §Out-of-scope). Stripping the canonical `…/docket.sh` first is what makes the
# `${DOCKET_SCRIPTS_DIR` assert sound (the canonical spelling contains install.sh in :?).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
source "$REPO/tests/lib/harness.sh"   # <-- match the actual harness path used by sibling tests

CONV="$REPO/skills/docket-convention/SKILL.md"

# In-scope files (glob-derived; a FLOOR guarded structurally below is unnecessary — the
# glob IS the derivation).
mapfile -t SCOPE < <(
  { ls "$REPO"/skills/*/SKILL.md
    ls "$REPO"/skills/docket-convention/references/*.md; } 2>/dev/null | sort -u
)

# Op inventory: grep-derived from scripts/docket.md's Subcommand table (NEVER hand-listed;
# same derivation the facade's own sentinel uses).
INVENTORY="$(grep -oE '^\| `[a-z-]+` ' "$REPO/scripts/docket.md" | tr -d '|` ' | sort -u)"

# Emit code UNITS for a file: fenced-block bodies (verbatim lines) + inline code spans.
extract_code_units() {
  awk '
    /^```/ { infence = !infence; next }
    infence { print; next }
    {
      line = $0
      while (match(line, /`[^`]*`/)) {
        print substr(line, RSTART, RLENGTH)
        line = substr(line, RSTART + RLENGTH)
      }
    }
  ' "$1"
}

# Strip the two byte-exact canonical forms so only the haystack remains.
strip_canonical() {
  sed -E \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket-config\.sh --bootstrap##g' \
    -e 's#"\$\{DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/docket\.sh##g'
}

# ---- Layer 1: absence sweep, per in-scope file ----
for f in "${SCOPE[@]}"; do
  rel="${f#$REPO/}"
  units="$(extract_code_units "$f")"
  hay="$(printf '%s\n' "$units" | strip_canonical)"

  assert "no non-facade / non-byte-exact helper invocation prefix survives in $rel" \
    '! printf "%s" "$hay" | grep -qF "${DOCKET_SCRIPTS_DIR"'
  assert "no eval-preamble shape in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF '\''eval "$('\'''
  assert "no inline \`fetch origin\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "fetch origin"'
  assert "no inline \`pull --rebase\` in code spans of $rel" \
    '! printf "%s" "$hay" | grep -qF "pull --rebase"'

  # every docket.sh <op> uses an inventory op (checked on RAW units, incl. canonical form)
  while read -r op; do
    [ -z "$op" ] && continue
    if ! printf '%s\n' "$INVENTORY" | grep -qxF "$op"; then
      assert "docket.sh op '$op' in $rel is in the inventory" 'false'
    fi
  done < <(printf '%s\n' "$units" | grep -oE 'docket\.sh [a-z-]+' | awk '{print $2}' | sort -u)
done

# Bootstrap carve-out is unique across the in-scope set (Layer 1 rule 5).
carve="$(grep -rhoF 'docket-config.sh --bootstrap' "${SCOPE[@]}" | wc -l | tr -d ' ')"
assert "CREATE_ORPHAN carve-out (docket-config.sh --bootstrap) occurs exactly once in skill prose" \
  '[ "$carve" = "1" ]'

# ---- Layer 2: presence anchors on the convention (grep -c == 1) ----
assert "Step-0 preamble runs preflight as its own call (unique anchor)" \
  '[ "$(grep -cF "as its own Bash call" "$CONV")" = "1" ]'
assert "mid-run re-sync verb is defined once (unique anchor)" \
  '[ "$(grep -cF "push-retry CAS loops alike" "$CONV")" = "1" ]'
assert "Step-0 instructs reading the printed block (unique anchor)" \
  '[ "$(grep -cF "read the printed \`KEY=value\` block" "$CONV")" = "1" ]'

finish   # <-- match the harness's summary/exit call used by sibling tests
```

- [ ] **Step 2: Align boilerplate with the real harness.** Open `tests/test_docket_facade.sh`; copy its exact `source …/tests/lib/…` line, `assert` signature, and summary/exit call into the new file (the placeholders `tests/lib/harness.sh` and `finish` above are stand-ins). Ensure `assert "<msg>" '<cmd>'` matches the repo convention (message, then a command string that must exit 0).

- [ ] **Step 3: Run it — expect RED now (reverse-mutation proof).**

Run: `bash tests/test_skill_facade_wiring.sh; echo "exit=$?"`
Expected: FAIL — every operating skill + the convention still carry `${DOCKET_SCRIPTS_DIR}"/…helper.sh` and `eval "$(`; the convention Step-0 anchors ("as its own Bash call", etc.) do not yet exist. Record which asserts fire; each firing proves that guard bites the pre-rewiring prose.

- [ ] **Step 4: Prove each guard is INDEPENDENTLY non-vacuous.** For each of the four Layer-1 asserts and three Layer-2 anchors, confirm at least one in-scope file triggers it now (from Step 3 output). For the op-inventory assert, temporarily inject `` `docket.sh bogus-op` `` into a scratch copy of a scope file, run, confirm RED, revert. Note the result inline.

- [ ] **Step 5: Commit.**

```bash
git add tests/test_skill_facade_wiring.sh
git commit -m "test(0072): skill-facade wiring guard (RED until prose rewired)"
```

---

### Task 2: Convention SKILL.md — Step-0 preamble, helper-resolution, sync-verb definition, drop-out cleanup

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (Step-0 preamble §; "Reaching the helper scripts" ¶; "Derived-view script family" ¶ line ~238; the bootstrap-guard/branch-model prose that names `disable-worktree-hooks.sh`/`render-board.sh` as INVOCATIONS)
- Test: `tests/test_skill_facade_wiring.sh` (Layer-2 anchors + convention Layer-1), plus any existing convention-anchoring test surfaced by grep

**Interfaces:**
- Produces: the master Step-0 preamble (steps 1-3 below), the unique Layer-2 anchor phrases, and the single CREATE_ORPHAN carve-out that all other files point to.

- [ ] **Step 1: Rewrite the Step-0 preamble** (`### Step-0 preamble (every operating skill)`). Replace current numbered steps 2-3 with three steps. New text (author to contain the exact anchor substrings `as its own Bash call`, `read the printed \`KEY=value\` block`, and `push-retry CAS loops alike`):

```markdown
1. Load this convention (blocking).
2. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` **as its own Bash call** — never compounded with other commands — then read the printed `KEY=value` block off stdout and carry those values forward as literals in later commands (no `eval`, no `source`). `preflight` performs every former Step-0 side effect: it resolves config, enforces the bootstrap verdict **fail-closed**, and (docket-mode) ensures the persistent `.docket/` metadata worktree exists, parks it on `docket`, disables its shared git hooks, and fetch + `pull --rebase` syncs it — or (main-mode) syncs the primary tree. On success it prints the block; on any verdict other than `PROCEED` it exits non-zero with a stderr diagnostic instead.
3. Act on the verdict: `PROCEED` → continue. `STOP_MIGRATE` → refuse and point at `migrate-to-docket.sh` (a human-initiated setup script, never an agent runtime invocation). `CREATE_ORPHAN` (fresh repo, once, human-attended) → run the one sanctioned direct-helper spelling `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --bootstrap`, then re-run `docket.sh preflight`. This carve-out exists only here, because the facade deliberately does not expose `docket-config.sh` and `preflight` fails closed.

All metadata reads and writes happen in the metadata working tree on `metadata_branch`, pushed to its remote immediately. Every mid-run metadata re-sync — pre-read syncs and **push-retry CAS loops alike** — is a fresh `docket.sh preflight` run (for a CAS loop: re-run `docket.sh preflight`, then retry the push); plain git plumbing (`git add`/`commit`/`push`, `git -C` forms) stays direct.
```

Note: keep the prose DESCRIPTION of preflight's mechanics (worktree ensure, hook disable, sync, main-mode degradation) — describing mechanics is fine; only the inline PROGRAMS are retired.

- [ ] **Step 2: Rewrite "Reaching the helper scripts" ¶.** Change the generic resolution template `` `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh` `` → `` `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>` `` and reword the sentence so it describes reaching the FACADE (whose ops wrap the helpers), e.g.: "a skill invokes every docket helper through the single facade `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>` (op = the wrapped helper's basename; the `preflight`/`env` verbs are the two exceptions)." Leave the `install.sh` NOUN mentions and `scripts/ensure-docket-env.md` reference intact (nouns, no prefix).

- [ ] **Step 3: Reword the two drop-out INVOCATIONS.** In Step-0's old step 3 the `` `"${DOCKET_SCRIPTS_DIR…}"/disable-worktree-hooks.sh --worktree .docket` `` invocation is already gone (folded into preflight's prose, Step 1). In the "Derived-view script family" ¶ (~line 238) and the branch-model hook ¶ (~line 262), the mentions of `render-board.sh` / `disable-worktree-hooks.sh` are descriptive NOUNS (no prefix) — leave them, OR (spec §3 "drop out of runtime prose entirely") optionally reword to op-names for hygiene. They are NOT tokenizer violations. Do not touch `github-mirror.sh`/`render-change-links.sh` NOUNS.

- [ ] **Step 4: Run the wiring test.**

Run: `bash tests/test_skill_facade_wiring.sh; echo "exit=$?"`
Expected: the convention's Layer-1 asserts + all three Layer-2 anchors now PASS; the seven operating-skill files still FAIL (rewired in Task 3).

- [ ] **Step 5: Follow-the-call any convention-anchoring existing test.** Run `grep -rn 'docket-config.sh --export\|eval "\$(' tests/ | grep -i conv` and inspect `tests/test_convention_extraction.sh`, `tests/test_docket_metadata_branch.sh` for asserts that anchor the OLD convention Step-0 shape against the convention markdown. For each, NARROW the anchor to its load-bearing property under the new spelling (never delete/loosen). Re-run those tests to green.

- [ ] **Step 6: Commit.**

```bash
git add skills/docket-convention/SKILL.md tests/
git commit -m "docs(0072): rewire convention Step-0 preamble onto docket.sh preflight"
```

---

### Task 3: The seven operating skills — Step-0 pointer, invocation swaps, sync loops

**Files (Modify):** `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-auto-groom/SKILL.md`
**Test:** `tests/test_skill_facade_wiring.sh` + every existing per-skill prose test surfaced by grep.

**Interfaces:**
- Consumes: the canonical spelling, op map, and sync-verb rule from Global Constraints; the convention Step-0 preamble from Task 2.

Apply the SAME mechanical transform to each file (derive the exact sites per file by grep — the counts in Global Constraints are a floor):

- [ ] **Step 1: Rewrite each skill's Step-0 reference (the `eval "$(` line).** Replace the inline eval with a pointer to the convention preamble. Pattern: `resolve config + the bootstrap verdict (`eval "$(…docket-config.sh --export)"`, …)` → `run the convention's Step-0 preamble (`docket.sh preflight` as its own Bash call; read the printed `KEY=value` block)`. Preserve each file's surrounding sentence about WHERE its writes land. For `docket-status/SKILL.md:29` specifically, keep its note that `docket.sh docket-status` re-derives the bootstrap gate + sync itself, but the Step-0 `preflight` gives the block for the rest of the skill.

- [ ] **Step 2: Swap every direct-helper invocation to the facade.** For each `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<helper>.sh <args>` code span, apply the Helper→op map: replace `/<helper>.sh` with `/docket.sh <op>`, args unchanged. Verbatim examples:
  - `…/board-refresh.sh --changes-dir .docket/<changes_dir> --surfaces "$BOARD_SURFACES"` → `…/docket.sh board-refresh --changes-dir .docket/<changes_dir> --surfaces "$BOARD_SURFACES"`
  - `…/render-change-links.sh --change-file … --adrs-dir …` → `…/docket.sh render-change-links --change-file … --adrs-dir …`
  - `…/render-adr-index.sh --adrs-dir .docket/<adrs_dir> > .docket/<adrs_dir>/README.md` → `…/docket.sh render-adr-index --adrs-dir .docket/<adrs_dir> > .docket/<adrs_dir>/README.md`
  - `…/terminal-publish.sh --id <id> --enabled <terminal_publish>` → `…/docket.sh terminal-publish --id <id> --enabled <terminal_publish>`
  - `…/archive-change.sh …` → `…/docket.sh archive-change …`; `…/cleanup-feature-branch.sh --slug <slug>` → `…/docket.sh cleanup-feature-branch --slug <slug>`; `…/sync-integration-branch.sh …` → `…/docket.sh sync-integration-branch …`.
  Also convert descriptive INVOCATION-shaped spans that are daily helpers (e.g. docket-status's `` `terminal-publish.sh --id <id> --enabled true` ``, `` `/board-checks.sh` ``, `` `/render-change-links.sh` ``, `` `docket-status.sh` `` when shown as the thing to run) to the facade op form. Leave `$BOARD_SURFACES`/`$SKILL_*` placeholder reads as-is (they now name keys from the preflight/env block — the convention defines the placeholder→key mapping; no shape change needed).

- [ ] **Step 3: Route every sync instruction through preflight.** Replace each code-span `git … fetch origin docket` / `git … pull --rebase …` sync/CAS instruction with prose "re-run `docket.sh preflight`" (CAS: "re-run `docket.sh preflight`, then retry the push"). Specific sites (grep to confirm the full set):
  - `docket-implement-next/SKILL.md`: the claim CAS (`re-sync (`git pull --rebase`, or `git -C .docket pull --rebase origin docket`)` → `re-sync (re-run `docket.sh preflight`)`); the Step-4 SHA-compare (`git -C .docket fetch origin docket` → `re-run `docket.sh preflight``, keeping the plain `git … rev-parse` compares direct).
  - `docket-new-change/SKILL.md:35,43`, `docket-groom-next/SKILL.md:71`, `docket-auto-groom/SKILL.md:56`, `docket-status/SKILL.md:65`: replace the inline `pull --rebase` sync/retry phrases with "re-run `docket.sh preflight`". Keep `git add`/`commit`/`push` direct.

- [ ] **Step 4: Run the wiring test after each file (or after all).**

Run: `bash tests/test_skill_facade_wiring.sh; echo "exit=$?"`
Expected: PASS once all seven files + the convention (Task 2) are rewired.

- [ ] **Step 5: Follow-the-call the per-skill existing tests.** Grep the enumerated candidates for asserts anchoring OLD skill-prose shapes and NARROW each to its load-bearing property under the new spelling:
  Run: `for f in tests/test_*.sh; do grep -lE "docket-config.sh --export|/board-refresh.sh|/render-change-links.sh|/terminal-publish.sh|/archive-change.sh|/cleanup-feature-branch.sh|/render-adr-index.sh|pull --rebase" "$f"; done | sort -u`
  For each hit, confirm whether the assert targets SKILL MARKDOWN (in scope — narrow it) or a SCRIPT/`scripts/*.md`/facade (OUT of scope — leave untouched; e.g. `tests/test_docket_facade.sh`, `tests/test_docket_status.sh` script asserts, `tests/test_worktree_hooks_wiring.sh`, `tests/test_consuming_repo_scripts.sh`'s live resolver `eval`). Re-run each edited test to green.

- [ ] **Step 6: Commit.**

```bash
git add skills/ tests/
git commit -m "docs(0072): rewire the seven operating skills onto the docket.sh facade"
```

---

### Task 4: `references/terminal-close-out.md` + `references/agent-layer.md` verify

**Files:**
- Modify: `skills/docket-convention/references/terminal-close-out.md`
- Verify (rewire only if grep finds old shapes): `skills/docket-convention/references/agent-layer.md`
- Test: `tests/test_skill_facade_wiring.sh` + `tests/test_closeout.sh`

- [ ] **Step 1: Rewire terminal-close-out.md.** Swap its daily-helper INVOCATION spans to the facade op form per the map: `…/archive-change.sh …` → `…/docket.sh archive-change …`; `…/render-change-links.sh` → `…/docket.sh render-change-links`; `…/terminal-publish.sh …` → `…/docket.sh terminal-publish …`; `…/cleanup-feature-branch.sh …` → `…/docket.sh cleanup-feature-branch …`; `…/board-refresh.sh …` → `…/docket.sh board-refresh …`. The `docket-config.sh --export` mention (line ~1/context prose describing config resolution) — if it is a descriptive NOUN, leave it; if it instructs an eval, reword to "the `env`/`preflight` block". Route any `pull --rebase` phrase (line ~122 loser's rebase note) — that one is a git-plumbing description of a concurrent writer's `git pull --rebase`, NOT a docket sync instruction inside a code span; confirm it is prose (no backticks). If it is a bare-prose "pull --rebase" it does NOT trip the code-span tokenizer — leave it; if backticked, reword to "re-run `docket.sh preflight`".

- [ ] **Step 2: Verify agent-layer.md.** Run `bash tests/test_skill_facade_wiring.sh 2>&1 | grep -i agent-layer`. Expected: agent-layer.md passes with NO edits (its `sync-agents.sh`/`link-skills.sh`/`docket-config.sh` code spans are descriptive nouns, no `${DOCKET_SCRIPTS_DIR}` prefix). If any assert fires on it, inspect: only rewire a genuine daily-helper INVOCATION / eval / fetch / pull shape; never touch descriptive human-tier nouns.

- [ ] **Step 3: Run wiring + closeout tests.**

Run: `bash tests/test_skill_facade_wiring.sh; echo "w=$?"; bash tests/test_closeout.sh; echo "c=$?"`
Expected: both PASS (0). Follow-the-call any `test_closeout.sh` assert anchoring an old close-out prose invocation.

- [ ] **Step 4: Commit.**

```bash
git add skills/ tests/
git commit -m "docs(0072): rewire terminal-close-out onto the facade; verify agent-layer clean"
```

---

### Task 5: Whole-suite green + mutation re-verification + review prep

**Files:** none (verification only) — plus any regression fix the suite surfaces.

- [ ] **Step 1: Run the WHOLE suite (never only the enumerated tests — LEARNINGS goal-scoped-rewrite family).**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" >/tmp/out.$$ 2>&1 || { echo "FAIL $t"; tail -30 /tmp/out.$$; }; done; echo DONE`
Expected: every test passes. Any RED that is environment-bound (LEARNINGS #34/#66) is confirmed by re-running the identical test against unmodified `origin/main` and byte-comparing the failing set; record the differential. Any genuine regression is fixed minimally (prose or an over-narrowed anchor), never by loosening a guard.

- [ ] **Step 2: Re-mutation-test the new guards (guards-are-code).** For each Layer-1 assert: re-introduce ONE old shape into a throwaway copy of an in-scope file (e.g. add `` `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/board-refresh.sh` `` to a scratch file), run the wiring test, confirm RED, discard the scratch edit. For each Layer-2 anchor: delete the anchored sentence from a scratch copy of the convention, confirm RED, discard. Confirm the bootstrap-uniqueness assert goes RED if a second `docket-config.sh --bootstrap` is added. Record each result.

- [ ] **Step 3: Whole-branch grep audit (completeness).** Confirm zero residual old shapes in scope:

```bash
git grep -nE 'eval "\$\("'\''"'\''\$\{DOCKET_SCRIPTS_DIR' -- 'skills/**/SKILL.md' 'skills/docket-convention/references/*.md' || echo "no eval preambles"
git grep -nE 'DOCKET_SCRIPTS_DIR:\?run docket/install\.sh\}"/[a-z-]+\.sh' -- 'skills/**/SKILL.md' 'skills/docket-convention/references/*.md' | grep -v '/docket\.sh\|/docket-config\.sh --bootstrap' || echo "only facade + bootstrap remain"
```
Expected: the only surviving `${DOCKET_SCRIPTS_DIR}"/…\.sh` invocations are `…/docket.sh` and the single `…/docket-config.sh --bootstrap`.

- [ ] **Step 4: Commit any fix; leave the branch clean for review.**

```bash
git add -A && git commit -m "test(0072): whole-suite green; mutation-verified wiring guards" || echo "nothing to commit"
```
```

## Self-Review

Spec coverage: §1 new Step-0 preamble → Task 2; §2 value interpolation (placeholders = env/preflight keys) → Global Constraints + Task 2 helper-resolution; §3 invocation rewiring (all daily helpers → facade, all syncs → preflight, plumbing stays, two helpers drop out) → Tasks 2-4; §4 tokenizer + unique anchors, mutation-tested → Task 1 + Task 5 Step 2; existing-sentinel migration (follow-the-call) → Tasks 2/3/4 Step 5, Task 5. CREATE_ORPHAN carve-out (byte-exact, once) → Task 2 Step 1 + Task 1 uniqueness assert.

Interpretation recorded (for the human merge gate): the tokenizer guards non-facade INVOCATIONS (prefix + retired shapes), NOT every `.sh` token — resolved from the spec §3 agent-layer.md tiebreaker + guards-are-code soundness. Documented in the test header and to be restated in the PR body + results file.
