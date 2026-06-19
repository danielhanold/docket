# Finalize consent model — ambiguity-only prompt + `require_pr_approval` gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Execution note (this change):** the work is three tightly-coupled artifacts dominated by ONE file (`skills/docket-finalize-change/SKILL.md`). Per LEARNINGS 2026-06-02 (#1) — "build inline when tasks share one artifact; fan out only for genuinely independent tasks" — execute **inline** (`superpowers:executing-plans`), not fanned-out subagents, to avoid inconsistent edits to the shared SKILL.md.

**Goal:** Make `docket-finalize-change` prompt on the no-arg path only when more than one eligible candidate would be merged, and add a repo-level `finalize.require_pr_approval` policy knob (default `false`) gating whether the auto-detect path will merge an unapproved PR.

**Architecture:** Pure markdown/prose + one config knob — no executable code path changes. The behavior lives in `skills/docket-finalize-change/SKILL.md` (the Selection matrix and the gate config doc); a commented knob is added to this repo's `.docket.yml` for discoverability; and the structural test `tests/test_finalize_gate.sh` grows sentinels that sample the new prose + parse the new knob. A small ADR (recorded by the driver in step 6, not a plan task) captures the consent/approval model.

**Tech Stack:** Markdown skill files, YAML config (`.docket.yml`), Bash sentinel tests (`grep`/`awk`), run with `bash tests/test_finalize_gate.sh`.

## Global Constraints

- **Spec is authoritative:** `docs/superpowers/specs/2026-06-17-finalize-consent-model-design.md` (read from the `docket` metadata branch). The §4.1 matrix, §4.2 explicit-id rule, and §3 config are copied verbatim below — do not re-derive.
- **Knob:** `finalize.require_pr_approval`, default `false`, nested in the `finalize:` block beside `gate:`/`test_command:`. `true` ⇒ the **auto-detect** path refuses to merge an unapproved PR (`reviewDecision != APPROVED`), surfacing it instead.
- **Explicit id always overrides** `require_pr_approval` (an explicit id is itself the human sign-off); the rebase-retest correctness gate still runs regardless of which proof was used.
- **Scope guard (out of scope):** do NOT touch the rebase-retest `gate` behavior/CI logic, CI-state selection, kill paths, or terminal-publish. Do NOT add a `--yes`/`all` bypass flag. Do NOT edit `skills/docket-convention/` — the spec scopes `require_pr_approval` documentation to finalize's own SKILL.md (follows `gate`/`test_command`'s precedent).
- **Test discipline (LEARNINGS):** every new assert must be **non-vacuous** — deleting the prose/config clause it guards must flip it to `NOT OK` (mutation-test each one) [#2, 2026-06-04]. **Anchor order/presence asserts to the UNIQUE phrase the target clause owns**, never a broad keyword OR-set that can latch onto an earlier line [#15, PR #32]. **Never `producer | grep -q`** under `pipefail` — `gate_of`/`rpa_of` capture into a var, then read the var [#16/#11]. Sentinels are sampling, not parsing — they are paired with the step-6 whole-branch review [#5, PR #6].
- **Tests run against the integration-branch checkout** — every file a sentinel reads (`SKILL.md`, `.docket.yml`) lands on the feature branch (→ `main`); no metadata-branch-only assertions [#6, PR #8].
- **Commit cadence:** commit per task. Run `bash tests/test_finalize_gate.sh` after each test/impl step; run **all** `tests/*.sh` once at the end (no aggregate runner exists — loop the glob) to catch regressions in sibling sentinels.

---

## File Structure

- **Modify** `skills/docket-finalize-change/SKILL.md`
  - *The rebase-retest merge gate* config block (the `finalize:` YAML + its prose) → document `require_pr_approval` + default `false`.
  - *Selection* section → replace the unconditional "PROMPT before merging" rule with the §4.1 auto-detect matrix (single eligible → no prompt; >1 → prompt; surface-don't-merge for un-mergeable and, under policy, unapproved) and the §4.2 explicit-id-override note.
  - *Per-change steps* step 1 → update the one sentence that says "under auto-detect, PROMPT first … before merging" to match the matrix.
- **Modify** `.docket.yml` (repo root, on the default/integration branch `main`) → add a **commented** `require_pr_approval` line in the `finalize:` block for discoverability (effective value stays the default `false`).
- **Modify** `tests/test_finalize_gate.sh` → add an `rpa_of()` parser + config-parse asserts, the SKILL-doc/Selection-matrix/explicit-id sentinels, and the repo-`.docket.yml` discoverability asserts.

---

## Task 1: `require_pr_approval` config knob — parser, repo config, SKILL doc

**Files:**
- Test: `tests/test_finalize_gate.sh` (add `rpa_of()` + config-parse + doc/discoverability asserts)
- Modify: `.docket.yml` (commented knob in `finalize:` block)
- Modify: `skills/docket-finalize-change/SKILL.md` (gate config block + prose)

**Interfaces:**
- Produces: `rpa_of <path>` Bash helper echoing `true|false` (default `false`), mirroring the existing `gate_of` awk idiom — block-scoped to `finalize:`, comment-stripping, SIGPIPE-safe (capture into a var).
- Consumes: the existing `assert`, `$FIN`, `$DYML` from the test header (unchanged).

- [ ] **Step 1: Write the failing asserts (parser + doc + discoverability)**

In `tests/test_finalize_gate.sh`, immediately AFTER the existing `gate_of()`/config-parse block (after the line `rm -rf "$TMPC"`), add:

```bash
# ---- Config parse: the nested finalize.require_pr_approval key (default false) -
# Same block-scoped awk idiom as gate_of; SIGPIPE-safe (capture, no producer|grep).
rpa_of(){  # $1 = path to a .docket.yml ; echoes true|false (default false)
  local v
  v="$(awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f&&/^[^[:space:]#]/{f=0}
    f&&/^[[:space:]]+require_pr_approval[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*require_pr_approval[[:space:]]*:[[:space:]]*/,"",line);
      gsub(/[[:space:]]/,"",line); print line; exit
    }' "$1" 2>/dev/null)"
  printf '%s' "${v:-false}"
}
TMPR="$(mktemp -d)"
printf 'finalize:\n  require_pr_approval: true\n'  > "$TMPR/true.yml"
printf 'finalize:\n  require_pr_approval: false\n' > "$TMPR/false.yml"
printf 'finalize:\n  gate: local\n'                > "$TMPR/nokey.yml"   # finalize block, no rpa key
printf 'metadata_branch: docket\n'                 > "$TMPR/absent.yml"  # no finalize block
assert "rpa-parse: require_pr_approval true"            '[ "$(rpa_of "$TMPR/true.yml")"   = "true" ]'
assert "rpa-parse: require_pr_approval false"           '[ "$(rpa_of "$TMPR/false.yml")"  = "false" ]'
assert "rpa-parse: key absent in finalize => false"     '[ "$(rpa_of "$TMPR/nokey.yml")"  = "false" ]'
assert "rpa-parse: no finalize block => false"          '[ "$(rpa_of "$TMPR/absent.yml")" = "false" ]'
# A commented knob must parse as the default (commented line is not a key):
printf 'finalize:\n  # require_pr_approval: false\n'    > "$TMPR/commented.yml"
assert "rpa-parse: commented knob => default false"     '[ "$(rpa_of "$TMPR/commented.yml")" = "false" ]'
rm -rf "$TMPR"

# ---- finalize SKILL documents require_pr_approval with default false ----------
assert "finalize documents require_pr_approval default false" \
  'grep -Eqi "require_pr_approval.*default.*false" "$FIN"'
assert "finalize ties require_pr_approval to the auto-detect path + unapproved PR" \
  'grep -q "reviewDecision != APPROVED" "$FIN"'

# ---- repo .docket.yml carries the knob (commented) at its default -------------
assert "repo .docket.yml mentions require_pr_approval (discoverability)" \
  'grep -q "require_pr_approval" "$DYML"'
assert "repo .docket.yml leaves require_pr_approval at default false" \
  '[ "$(rpa_of "$DYML")" = "false" ]'
```

- [ ] **Step 2: Run the suite — confirm the new asserts FAIL where expected**

Run: `bash tests/test_finalize_gate.sh`
Expected: the five `rpa-parse:` asserts already pass (they test the just-added parser against synthetic temp files). The four asserts that read the real files — `finalize documents require_pr_approval default false`, `finalize ties require_pr_approval to the auto-detect path + unapproved PR`, `repo .docket.yml mentions require_pr_approval`, and `repo .docket.yml leaves require_pr_approval at default false` (the last passes coincidentally since a missing key defaults false, but the `mentions` one FAILS) — print `NOT OK`, overall exit `1`. This proves the doc/config asserts are live before implementation.

- [ ] **Step 3: Add the commented knob to the repo `.docket.yml`**

In `.docket.yml`, in the `finalize:` block, change:

```yaml
finalize:
  gate: local
  # test_command:
```

to:

```yaml
finalize:
  gate: local
  # test_command:
  # require_pr_approval: false  # default. true => the auto-detect (no-arg) path refuses to merge an
  #                             #   unapproved PR (reviewDecision != APPROVED), surfacing it instead.
  #                             #   An explicit `docket-finalize-change <id>` always overrides this.
```

(Commented so the effective value stays the default `false`; `rpa_of` ignores a `#`-led line, so the discoverability assert sees the literal while the default-false assert still holds.)

- [ ] **Step 4: Document the knob in the SKILL.md gate config block**

In `skills/docket-finalize-change/SKILL.md`, under *## The rebase-retest merge gate*, change the config block:

```yaml
finalize:
  gate: local          # local (default) | ci | both | off
  test_command:        # OPTIONAL override; unset => the agent auto-detects the suite
```

to:

```yaml
finalize:
  gate: local                 # local (default) | ci | both | off
  test_command:               # OPTIONAL override; unset => the agent auto-detects the suite
  require_pr_approval: false  # default false. true => the auto-detect path refuses to merge
                              #   an unapproved PR (reviewDecision != APPROVED), surfacing instead.
```

Then, immediately AFTER the existing paragraph that ends "…the override is used verbatim only when auto-detection guesses wrong." add a new paragraph:

```markdown
`require_pr_approval` defaults to **`false`** — approval is never a selection-time
blocker (the single-human-friendly default: the author pushes their own PR and so
cannot approve it on GitHub at all). Set it **`true`** to make the **auto-detect path**
refuse to merge an unapproved PR (`reviewDecision != APPROVED`), surfacing it instead of
merging — `gate` validates *correctness*, `require_pr_approval` validates *human sign-off*.
It governs **only the auto-detect path**: an explicit `docket-finalize-change <id>` always
overrides it (the explicit id is itself the human authorization the gate asks for), and the
rebase-retest correctness gate still runs regardless.
```

- [ ] **Step 5: Run the suite — confirm Task 1 asserts pass**

Run: `bash tests/test_finalize_gate.sh`
Expected: all `rpa-parse:` asserts `ok`; `finalize documents require_pr_approval default false` `ok`; `finalize ties require_pr_approval to the auto-detect path + unapproved PR` `ok`; both `repo .docket.yml …` asserts `ok`. No new `NOT OK`. (The Selection-matrix asserts are added in Task 2, so overall exit is still `0` here — every pre-existing assert plus Task 1's pass.)

- [ ] **Step 6: Mutation-check (non-vacuity), then commit**

Temporarily delete the `require_pr_approval: false` line from `.docket.yml`'s `finalize:` block and the `require_pr_approval` line from the SKILL config block; run `bash tests/test_finalize_gate.sh` and confirm the `mentions`/`documents` asserts flip to `NOT OK`. Restore both lines, re-run (back to green), then commit:

```bash
git add tests/test_finalize_gate.sh .docket.yml skills/docket-finalize-change/SKILL.md
git commit -m "docket(0021): add finalize.require_pr_approval knob (default false) + sentinels"
```

---

## Task 2: Ambiguity-only Selection matrix + explicit-id override

**Files:**
- Test: `tests/test_finalize_gate.sh` (add Selection-matrix + explicit-id sentinels)
- Modify: `skills/docket-finalize-change/SKILL.md` (*Selection* section + step-1 sentence)

**Interfaces:**
- Consumes: `$FIN`, `assert` (unchanged); the `require_pr_approval` doc from Task 1 (the matrix references the knob).
- Produces: nothing downstream — this is the last code task; step 6 (review) and the ADR follow.

- [ ] **Step 1: Write the failing Selection sentinels**

In `tests/test_finalize_gate.sh`, after Task 1's `repo .docket.yml …` asserts, add:

```bash
# ---- Selection: ambiguity-only prompting (the §4.1 matrix) --------------------
# Anchor each assert to the UNIQUE phrase its matrix row owns (LEARNINGS #15) — not a
# broad keyword set that could latch onto step-1 prose. Each is a single-line grep so
# the two halves must co-occur in the same row.
assert "selection: exactly one eligible => no prompt" \
  'grep -Eqi "exactly one eligible.*no prompt" "$FIN"'
assert "selection: more than one eligible => prompt" \
  'grep -Eqi "more than one eligible.*prompt" "$FIN"'
assert "selection: surface-don't-merge an un-mergeable candidate" \
  'grep -Eqi "not git-mergeable.*surface, do not merge" "$FIN"'
assert "selection: surface-don't-merge an unapproved PR under the policy" \
  'grep -Eqi "require_pr_approval.{0,40}surface, do not merge|reviewDecision != APPROVED.{0,80}surface, do not merge" "$FIN"'
# ---- §4.2 explicit id overrides the approval policy --------------------------
assert "selection: explicit id overrides require_pr_approval" \
  'grep -Eqi "explicit id overrides .{0,4}require_pr_approval|explicit id.{0,40}overrides.{0,40}require_pr_approval" "$FIN"'
```

- [ ] **Step 2: Run the suite — confirm the Selection asserts FAIL**

Run: `bash tests/test_finalize_gate.sh`
Expected: the five new `selection:` asserts print `NOT OK` (the current Selection section still says only "PROMPT before merging — merging is a deliberate act", with no matrix, no "eligible", no override). Overall exit `1`.

- [ ] **Step 3: Rewrite the Selection section**

In `skills/docket-finalize-change/SKILL.md`, replace the entire *## Selection* body — from `Given an explicit change id, OR auto-detect:` through `The per-change steps below run for each selected change.` — with:

```markdown
Given an explicit change id, OR auto-detect.

**Explicit id** (`docket-finalize-change <id>`) — never prompts (an explicit id is
unambiguous). The rebase-retest correctness gate still runs. The explicit id is itself the
human authorization, so **an explicit id overrides `require_pr_approval`**: it merges even an
unapproved PR. The approval policy governs only the auto-detect path; merging an unapproved PR
simply requires being explicit about it.

**Auto-detect** — already-merged PRs are archived silently (idempotent, unchanged). For the
rest, classify every `implemented` candidate and act per this matrix:

| Candidate | Behavior |
|---|---|
| Not git-mergeable (`CLOSED`, `DRAFT`, or a GitHub-reported conflict the gate can't act on) | **Surface, do not merge** |
| `require_pr_approval: true` AND unapproved (`reviewDecision != APPROVED`) | **Surface, do not merge** — the policy gate; report it so you know docket saw it and why it skipped |
| **Exactly one** eligible candidate | **Run the full flow — gate + merge + finalize — with NO prompt** |
| **More than one** eligible candidate | **Prompt**: list them and confirm the batch (the blast-radius guard) |

"Eligible" = git-mergeable AND (`require_pr_approval: false` OR approved). The ambiguity count
is over *eligible* candidates only: under `require_pr_approval: true` an unapproved PR is
surfaced-not-merged and does **not** count toward the prompt. Git-conflict *resolution* is
delegated to the rebase-retest gate (it rebases onto base and dispatches
`docket-rebase-resolver`); selection's "surface, do not merge" covers only states the gate
can't act on (draft/closed/flatly un-mergeable).

The prompt exists **only to guard the bulk-merge blast radius** — more than one eligible
target at once. The common case, one obvious target the human deliberately invoked finalize on,
merges with **no prompt**.

The per-change steps below run for each selected change.
```

- [ ] **Step 4: Update the step-1 sentence to match the matrix**

In the same file, under *## Per-change steps*, in step 1, locate the sentence:

> Invoking finalize on an **explicit change id** IS the merge decision — the gate is respected; under **auto-detect**, PROMPT first per the Selection rules above before merging.

and replace it with:

> Invoking finalize on an **explicit change id** IS the merge decision (and overrides `require_pr_approval`) — the gate is respected; under **auto-detect**, follow the Selection matrix above — a single eligible candidate merges with **no prompt**, and finalize prompts **only when more than one** is eligible.

- [ ] **Step 5: Run the suite — confirm all asserts pass**

Run: `bash tests/test_finalize_gate.sh`
Expected: every assert `ok`, overall exit `0` — Task 1's asserts, Task 2's five `selection:` asserts, and all pre-existing finalize-gate asserts (the rebase-retest gate, the two agents, sign-off, abort-and-report set, convention/status cross-checks) still green (the Selection rewrite preserves the gate/agent prose those guard).

- [ ] **Step 6: Mutation-check (non-vacuity), then commit**

For each of the five `selection:` asserts, momentarily remove the matrix row / override sentence it targets and confirm it flips to `NOT OK`; restore and re-run green. Then run the **full** suite to catch sibling regressions:

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" | grep -E "NOT OK" && echo "FAILED: $t" || echo "  all ok"; done
```
Expected: no `NOT OK` in any file. Then commit:

```bash
git add tests/test_finalize_gate.sh skills/docket-finalize-change/SKILL.md
git commit -m "docket(0021): Selection — ambiguity-only prompt matrix + explicit-id override"
```

---

## Post-build (driver-owned, not plan tasks)

- **Step 6 (review):** whole-branch `superpowers:requesting-code-review`; re-read `LEARNINGS.md` first. Verify the sentinels are non-vacuous and the prose reads coherently (sampling ≠ parsing, #5).
- **ADR:** dispatch the `docket-adr` subagent to record the finalize consent/approval model (ambiguity-only prompt + `require_pr_approval`), `relates_to` ADR-0010, `change: 21`; append the returned number to the change's `adrs:` in the metadata working tree.
- **Step 6.5 (results):** optional — write a results file only if findings/follow-ups/interactive checks warrant; otherwise the PR description + green tests are the receipt.

## Self-Review (against the spec)

- **§3 config** → Task 1 (knob nested in `finalize:`, default `false`, documented in finalize's SKILL; commented in repo `.docket.yml`). ✓
- **§4.1 matrix** (archived-silent / surface-un-mergeable / surface-unapproved-under-policy / one-eligible-no-prompt / >1-prompt) → Task 2 Selection rewrite + four sentinels. ✓
- **§4.2 explicit id** (never prompts; gate still runs; overrides approval) → Task 2 Selection "Explicit id" paragraph + step-1 sentence + override sentinel. ✓
- **§5 principle** (human authorized; approval or explicit id; correctness regardless) → captured in the Selection prose + config paragraph; recorded durably in the ADR. ✓
- **§6 scope** (edit finalize SKILL Selection + gate config; commented `.docket.yml`; extend `test_finalize_gate.sh` non-vacuously; ADR) → Tasks 1–2 + post-build ADR. ✓
- **§7 out of scope** (no gate/CI change, no `--yes` flag, no kill/terminal-publish, no convention edit) → honored by the scope guard; no task touches them. ✓
