# Typed Changes, Selective Auto-Capture, and Backlog Filters — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every change an explicit configurable `type:`, let auto-capture admit only selected types, and add a board Type column plus report-only `--type`/`--priority` backlog filters.

**Architecture:** One authoritative change-type array lands in `scripts/lib/docket-frontmatter.sh` beside the existing `DOCKET_STATUSES`/`DOCKET_PRIORITIES` vocabularies. `scripts/docket-config.sh` gains a `change_types` list and replaces the scalar `auto_capture` with a nested block resolved exactly like the existing `learnings:` block (per-leaf fallback via `yaml_block_body`), exporting `CHANGE_TYPES`, `AUTO_CAPTURE_ENABLED`, and `AUTO_CAPTURE_TYPES`. `render-board.sh` gains a Type column on active tables and `--type`/`--priority` filters that narrow **only** the digest projection, never the markdown writer. A new deterministic `backfill-change-types.sh` applies a human-approved id→type mapping to active change files only.

**Tech Stack:** Bash 4+ (`$DOCKET_BASH_PATH`), POSIX-ish sed/awk/grep, hermetic `tests/*.sh` fixtures with bare-origin git repos. No package manager, no external deps.

## Global Constraints

Copied verbatim from the spec and this repo's AGENTS.md — every task's requirements implicitly include this section.

- Default taxonomy, in this order: `change_types: [chore, docs, feat, fix, refactor, perf]`. Ordered, non-empty, duplicate-free, lowercase tokens matching `[a-z][a-z0-9-]*`.
- `auto_capture` is a **map**. The legacy scalar `auto_capture: true|false` is invalid, has **no compatibility shim**, and fails closed with a migration-oriented diagnostic showing the new nested shape.
- Omitting the whole map ≡ `enabled: false`, `types: all`.
- Map leaves resolve **independently** (a high layer may override `enabled` while inheriting `types`). Lists **never merge** — a higher-layer list replaces the complete lower-layer list.
- `auto_capture.types` is either the scalar `all` or a duplicate-free list drawn from the effective `change_types`.
- The resolver **removes** `AUTO_CAPTURE` and emits `CHANGE_TYPES`, `AUTO_CAPTURE_ENABLED`, `AUTO_CAPTURE_TYPES`. Skills consume only resolver exports, never re-parsing YAML (ADR-0052).
- A consumer must still render and filter a type already stored in a change file even when that value is not in its current effective `change_types`. Configuration governs creation, not readability of shared historical data.
- Manifest values `all` and `untyped` are **forbidden** in a change file: `all` is a config selector and query pseudo-value; `untyped` is only a query/migration pseudo-value.
- Type filtering happens **before** the per-invocation mint cap is consumed. A suppressed candidate must not consume a mint slot. Dedup stays after admission, immediately before minting.
- Filters affect only the displayed active backlog projection (`change` lines + `ready` line). They never narrow merge detection, sweep, harvest, archive, publish, health checks, reclaim, or canonical board regeneration. A filtered `--board-only` run still writes a **complete** `BOARD.md`.
- Board row visibility (title/id/status) stays independent of type validation — never drop a row over a type problem.
- The backfill helper scans only `<changes_dir>/active/`, anchors edits to the **first** balanced frontmatter block, writes all validated files or none, is idempotent, and never reads or edits `<changes_dir>/archive/`.
- **AGENTS.md — shell:** never `producer | early-exiting-consumer` under `pipefail` (capture into a variable, then `grep <<<"$var"`); a `grep` pattern leading with `--` must use `-e`/`-F --`; awk indent classes are `[^[:space:]]`, never `[^ ]`.
- **AGENTS.md — frontmatter:** anchor every frontmatter edit to the first `---…---` block, never a bare column-0 match. Quote hand-authored YAML scalars carrying colon-space or boolean keywords. Validate marker order/balance before rewriting a marker-delimited block.
- **AGENTS.md — guards:** mutation-test every guard (strip what it guards, watch it redden). Key guards on syntactic **shape**, never an enumerated list of spellings. Derive gated sites from a whole-repo grep, never a hand-list.
- **ADR-0055:** an exhaustive vocabulary mapping is pinned by exact set equality against a single authoritative array, with an independent extractor-cardinality assert before comparison, and mutation-tested in **both** directions (remove a real arm → red; add a phantom arm → red).
- **Learning `model-authored-values-are-untrusted-input`:** never interpolate a model-authored value into a `sed` replacement (`&`/`\1` reinterpretation). Write through `awk`'s `ENVIRON[...]`. Validate by shape at argument intake — reject control characters.
- **Learning `config-knob-ship-end-to-end`:** a new config knob ships its `.docket.example.yml` entry, README documentation, and any now-relaxed prose in the same change.
- **Learning `atomic-generated-write`:** never redirect a renderer straight into the file it generates.
- Run the **whole** `tests/*.sh` suite at the build gate, not only the tests named here. `tests/test_docket_status.sh` needs `GIT_EDITOR=true` in a non-interactive run.

---

## File Structure

**Created**
- `scripts/backfill-change-types.sh` — deterministic one-time active-backlog categorization helper.
- `scripts/backfill-change-types.md` — its contract (required by `tests/test_script_contracts_coverage.sh`).
- `tests/test_backfill_change_types.sh` — hermetic fixtures for the helper.
- `tests/test_change_types.sh` — vocabulary array, membership helper, and ADR-0055 set-equality pinning.

**Modified**
- `scripts/lib/docket-frontmatter.sh` — authoritative `DOCKET_CHANGE_TYPES_DEFAULT` array + `docket_change_type_is_member` + `docket_change_type_is_reserved`.
- `scripts/docket-config.sh` — `change_types` list resolution; `auto_capture` scalar → nested map; export block.
- `scripts/docket-config.md` — layer precedence, whole-list replacement, per-leaf inheritance, new exports, legacy diagnostic.
- `scripts/mint-stub.sh` / `scripts/mint-stub.md` — `--type` argument, shape validation, frontmatter write.
- `scripts/render-board.sh` — Type column on active tables; `--type`/`--priority` digest filters.
- `scripts/render-board.md` — column and filter semantics.
- `scripts/docket-status.sh` / `scripts/docket-status.md` — `--type`/`--priority` passthrough to the digest projection only.
- `scripts/docket.sh` — `WRAPPED_OPS` gains `backfill-change-types`.
- `skills/docket-new-change/change-template.md` — `type:` field.
- `skills/docket-convention/SKILL.md` — config block, manifest block, Auto-capture shared definition.
- `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md` — capture prose reads the new exports.
- `.docket.example.yml`, `README.md` — documented keys (ADR-0052 / ADR-0053).
- `tests/test_docket_example_yml.sh` — qualified-key manifest (unblocks `auto_capture.enabled`).
- `tests/test_docket_config.sh`, `tests/test_mint_stub.sh`, `tests/test_render_board.sh`, `tests/test_docket_status.sh`, `tests/test_docket_frontmatter.sh` — new coverage.

---

## Task 1: Authoritative change-type vocabulary

Adds the single array every later task pins against. Nothing consumes it yet — this task exists so Tasks 3/4/6/7 have exactly one source to import.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (append beside `DOCKET_PRIORITIES`, ~line 147)
- Create: `tests/test_change_types.sh`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)` — the built-in taxonomy, order-significant.
  - `DOCKET_CHANGE_TYPE_RESERVED=(all untyped)` — pseudo-values forbidden in a manifest.
  - `docket_change_type_is_member <value> <type>...` → exit 0 if `<value>` is in the supplied list. Callers pass the **effective** list, not the default, so config can widen or narrow it.
  - `docket_change_type_is_reserved <value>` → exit 0 if `<value>` is `all` or `untyped`.
  - `docket_change_type_is_wellformed <value>` → exit 0 if `<value>` matches `^[a-z][a-z0-9-]*$`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_change_types.sh`:

```bash
#!/usr/bin/env bash
# tests/test_change_types.sh — the change-type vocabulary (change 0127).
# Run: bash tests/test_change_types.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# shellcheck disable=SC1090
. "$LIB"

# --- the array exists, is non-empty, ordered, duplicate-free, well-formed ----
assert "DOCKET_CHANGE_TYPES_DEFAULT is declared and non-empty" \
  '[ "${#DOCKET_CHANGE_TYPES_DEFAULT[@]}" -gt 0 ]'
assert "default taxonomy is exactly the six spec'd tokens in order" \
  '[ "${DOCKET_CHANGE_TYPES_DEFAULT[*]}" = "chore docs feat fix refactor perf" ]'
dups="$(printf '%s\n' "${DOCKET_CHANGE_TYPES_DEFAULT[@]}" | sort | uniq -d)"
assert "default taxonomy is duplicate-free (${dups:-none})" '[ -z "$dups" ]'
for t in "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"; do
  assert "default type '$t' is well-formed" 'docket_change_type_is_wellformed "$t"'
done

# --- membership is over the CALLER's list, not the default -------------------
assert "member: feat is in the default list" \
  'docket_change_type_is_member feat "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"'
assert "member: spike is NOT in the default list" \
  '! docket_change_type_is_member spike "${DOCKET_CHANGE_TYPES_DEFAULT[@]}"'
assert "member: honors a caller-supplied effective list (spike admitted)" \
  'docket_change_type_is_member spike chore spike'
assert "member: honors a caller-supplied effective list (feat excluded)" \
  '! docket_change_type_is_member feat chore spike'

# --- reserved pseudo-values --------------------------------------------------
assert "reserved: all" 'docket_change_type_is_reserved all'
assert "reserved: untyped" 'docket_change_type_is_reserved untyped'
assert "not reserved: feat" '! docket_change_type_is_reserved feat'

# --- well-formedness rejects the shapes the spec forbids ---------------------
for bad in "Feat" "1feat" "fe_at" "feat " "" "fe at" "-feat"; do
  assert "well-formed rejects '$bad'" '! docket_change_type_is_wellformed "$bad"'
done
assert "well-formed accepts a hyphenated token" 'docket_change_type_is_wellformed multi-word'

# --- ADR-0055: reserved set is pinned by set equality, with a cardinality floor
assert "reserved array has exactly 2 members" \
  '[ "${#DOCKET_CHANGE_TYPE_RESERVED[@]}" = 2 ]'
assert "reserved array is exactly {all, untyped}" \
  '[ "$(printf "%s\n" "${DOCKET_CHANGE_TYPE_RESERVED[@]}" | sort | tr "\n" " ")" = "all untyped " ]'

exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_change_types.sh`
Expected: FAIL — `NOT OK - DOCKET_CHANGE_TYPES_DEFAULT is declared and non-empty`, and `docket_change_type_is_wellformed: command not found` on stderr.

- [ ] **Step 3: Implement the vocabulary**

In `scripts/lib/docket-frontmatter.sh`, immediately after the `DOCKET_PRIORITY_DEFAULT=medium` line (~147) and before `_docket_array_has`, insert:

```bash
# --- change types (change 0127) ----------------------------------------------
# The BUILT-IN taxonomy. `change_types` in .docket.yml can replace this whole list (never merge
# with it), so every consumer takes an EFFECTIVE list as an argument and this array is only the
# default the resolver falls back to. Ordered: order is preserved through the resolver's export
# and is what board/filter output sorts by when it needs a canonical sequence.
DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)

# Pseudo-values that are legal in a QUERY or as a config selector but never legal in a stored
# manifest: `all` is the auto_capture.types selector and the --type wildcard; `untyped` is the
# --type query token for a change with no type: yet, and the backfill's migration-set name.
# Writing either into a change file would make a selector indistinguishable from a real value.
DOCKET_CHANGE_TYPE_RESERVED=(all untyped)

# Membership over the EFFECTIVE list the caller resolved — never over the default array. A change
# file may legitimately carry a type absent from this machine's effective list (another machine's
# config wrote it), so readers must not use this to decide whether to RENDER a stored value.
docket_change_type_is_member(){ # docket_change_type_is_member VALUE TYPE...
  local value="$1"; shift
  _docket_array_has "$value" "$@"
}

docket_change_type_is_reserved(){ # docket_change_type_is_reserved VALUE
  _docket_array_has "$1" "${DOCKET_CHANGE_TYPE_RESERVED[@]}"
}

# Shape gate, matching the spec's `[a-z][a-z0-9-]*`. Keyed on shape, never on an enumerated set of
# bad spellings (AGENTS.md), so a token nobody predicted is still rejected.
docket_change_type_is_wellformed(){ # docket_change_type_is_wellformed VALUE
  case "$1" in
    ''|*[![:alnum:]-]*) return 1 ;;
  esac
  printf '%s' "$1" | grep -Eq '^[a-z][a-z0-9-]*$'
}
```

Note `_docket_array_has` is defined *below* this insertion point in the file, which is fine — these are functions, resolved at call time, and the arrays are plain assignments.

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_change_types.sh`
Expected: every line `ok - …`, exit 0.

- [ ] **Step 5: Mutation-test the set-equality pin (ADR-0055, both directions)**

Run each mutation, confirm RED, then revert:

```bash
# remove a real arm
sed -i.bak 's/^DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)$/DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor)/' scripts/lib/docket-frontmatter.sh
bash tests/test_change_types.sh; echo "expect NOT OK above"
mv scripts/lib/docket-frontmatter.sh.bak scripts/lib/docket-frontmatter.sh

# add a phantom arm
sed -i.bak 's/^DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf)$/DOCKET_CHANGE_TYPES_DEFAULT=(chore docs feat fix refactor perf spike)/' scripts/lib/docket-frontmatter.sh
bash tests/test_change_types.sh; echo "expect NOT OK above"
mv scripts/lib/docket-frontmatter.sh.bak scripts/lib/docket-frontmatter.sh
```

Expected: both runs print `NOT OK - default taxonomy is exactly the six spec'd tokens in order`.

- [ ] **Step 6: Run the two suites that already source this lib**

Run: `bash tests/test_docket_frontmatter.sh && bash tests/test_render_board.sh`
Expected: both exit 0 — the insertion is additive and must not perturb them.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_change_types.sh
git commit -m "feat(0127): authoritative change-type vocabulary in the frontmatter lib"
```

---

## Task 2: Qualified-key manifest guard

`tests/test_docket_example_yml.sh` asserts **no duplicate leaf key names** in `.docket.example.yml`, and `learnings.enabled` already owns the leaf `enabled`. Documenting `auto_capture.enabled` is structurally blocked until this guard distinguishes a leaf by its parent. This task must land before Task 3 documents the new keys.

The guard's own comment predicted this exact collision, and it names the real hazard precisely: `yaml_get`'s flat, leaf-name-only reader. That hazard is **read-shape-specific**, not universal — `finalize.gate` is read flat (`lcl gate` → `yaml_get "$LCFG" gate`), while `learnings.*`, `reclaim.*`, and `skills.*` are read **block-scoped** through `yaml_block_body`, which genuinely can tell two same-named leaves apart. So the fix is not to weaken the guard but to derive its scope from how the resolver reads each key.

**Files:**
- Modify: `tests/test_docket_example_yml.sh:289-294` (extraction), `:163-200` (`classify_key`), `:381-425` (counts + duplicate check)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: a manifest keyed by **qualified** name (`learnings.enabled`, `auto_capture.enabled`) for block-scoped keys and by bare leaf for flat-read keys. Task 3 adds `change_types`, `auto_capture.enabled`, `auto_capture.types` arms against this shape.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_example_yml.sh`, immediately after the existing `dup_leaf_keys` assert (~line 425):

```bash
# (2b-i) QUALIFIED-KEY EXTRACTION (change 0127). A leaf name is only ambiguous when the resolver
# reads it FLAT (yaml_get over the whole file, e.g. `lcl gate`). Block-scoped leaves are read
# within their own yaml_block_body, so `learnings.enabled` and `auto_capture.enabled` are
# genuinely distinct to the resolver and must be distinct to this manifest too.
assert "qualified extraction: a nested leaf carries its parent" \
  'printf "%s\n" "$example_keys_raw" | grep -qx "learnings.enabled"'
assert "qualified extraction: a top-level key stays bare" \
  'printf "%s\n" "$example_keys_raw" | grep -qx "board_surfaces"'
assert "qualified extraction: the finalize block is qualified too" \
  'printf "%s\n" "$example_keys_raw" | grep -qx "finalize.gate"'

# FLAT-READ COLLISION FLOOR: the duplicate check that actually protects yaml_get. Its population
# is the keys the resolver reads flat — every top-level key, plus the finalize leaves (read via
# `lcl <leaf>` / `yaml_get "$CFG" <leaf>`, never block-scoped). Derived from the read shape, not
# from an allowlist of names.
flat_read_keys="$(printf '%s\n' "$example_keys_raw" \
  | sed -nE 's/^(finalize\.)?([A-Za-z_][A-Za-z0-9_]*)$/\2/p')"
dup_flat_keys="$(printf '%s\n' "$flat_read_keys" | sort | uniq -d)"
assert "no duplicate FLAT-READ leaf names (${dup_flat_keys:-none}; yaml_get's head -n1 would mis-resolve these)" \
  '[ -z "$dup_flat_keys" ]'
assert "flat-read floor is non-vacuous (>= 10 keys; got $(printf "%s\n" "$flat_read_keys" | grep -c .))" \
  '[ "$(printf "%s\n" "$flat_read_keys" | grep -c .)" -ge 10 ]'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_docket_example_yml.sh`
Expected: FAIL — `NOT OK - qualified extraction: a nested leaf carries its parent` (extraction still emits bare `enabled`).

- [ ] **Step 3: Make extraction qualified**

Replace the `example_keys_raw` assignment (~line 289-293) with:

```bash
# Qualified extraction (change 0127): a top-level key emits its bare name; an INDENTED key emits
# `<parent>.<leaf>`, where parent is the nearest preceding column-0 key. Indent classes are
# [^[:space:]] / [[:space:]] so a tab-indented block is not silently dropped (AGENTS.md).
example_keys_raw="$(
  { awk '
      { line=$0; sub(/[[:space:]]*#.*/, "", line) }
      line ~ /^[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/ {
        parent=line; sub(/[[:space:]]*:.*/, "", parent); print parent; next
      }
      line ~ /^[[:space:]]+[A-Za-z_][A-Za-z0-9_]*[[:space:]]*:/ {
        leaf=line
        sub(/^[[:space:]]+/, "", leaf); sub(/[[:space:]]*:.*/, "", leaf)
        if (parent != "") print parent "." leaf; else print leaf
      }
    ' "$EX"
    commented_config_keys "$EX"
  }
)"
```

`commented_config_keys` yields the two commented top-level keys (`agents`, `agent_harnesses`) and stays bare — correct, since both are top-level.

- [ ] **Step 4: Requalify the manifest arms**

In `classify_key` (~line 163), change the nested arms from bare leaves to qualified names. Replace these arms:

```bash
    gate)                 echo 'resolved:FINALIZE_GATE' ;;
    test_command)         echo 'resolved:FINALIZE_TEST_COMMAND' ;;
    require_pr_approval)  echo 'resolved:FINALIZE_REQUIRE_PR_APPROVAL' ;;
    enabled)              echo 'resolved:LEARNINGS_ENABLED' ;;
    cap)                  echo 'resolved:LEARNINGS_CAP' ;;
    lease_ttl)            echo 'resolved:RECLAIM_LEASE_TTL' ;;
    auto)                 echo 'resolved:RECLAIM_AUTO' ;;
    brainstorm)           echo 'resolved:SKILL_BRAINSTORM' ;;
    plan)                 echo 'resolved:SKILL_PLAN' ;;
    build)                echo 'resolved:SKILL_BUILD' ;;
    review)               echo 'resolved:SKILL_REVIEW' ;;
    finish)               echo 'resolved:SKILL_FINISH' ;;
```

with:

```bash
    finalize.gate)                echo 'resolved:FINALIZE_GATE' ;;
    finalize.test_command)        echo 'resolved:FINALIZE_TEST_COMMAND' ;;
    finalize.require_pr_approval) echo 'resolved:FINALIZE_REQUIRE_PR_APPROVAL' ;;
    learnings.enabled)            echo 'resolved:LEARNINGS_ENABLED' ;;
    learnings.cap)                echo 'resolved:LEARNINGS_CAP' ;;
    reclaim.lease_ttl)            echo 'resolved:RECLAIM_LEASE_TTL' ;;
    reclaim.auto)                 echo 'resolved:RECLAIM_AUTO' ;;
    skills.brainstorm)            echo 'resolved:SKILL_BRAINSTORM' ;;
    skills.plan)                  echo 'resolved:SKILL_PLAN' ;;
    skills.build)                 echo 'resolved:SKILL_BUILD' ;;
    skills.review)                echo 'resolved:SKILL_REVIEW' ;;
    skills.finish)                echo 'resolved:SKILL_FINISH' ;;
```

The `runtime` arm and the `codex`/`runners` nesting: inspect the example's actual shape with `grep -n . .docket.example.yml | sed -n '1,80p'` and qualify any other arm the new extraction now emits as `parent.leaf`. The header arm `finalize|learnings|reclaim|skills|runners|codex` stays bare — those ARE the column-0 parents.

- [ ] **Step 5: Fix the correspondence check's key literal**

The correspondence check greps the resolver for an assignment line containing the leaf key literal. A qualified key must be reduced to its leaf before that grep. Locate the correspondence loop (search `manifest_bad_correspondence`) and derive the leaf inside it:

```bash
  leaf_k="${k##*.}"
```

then use `$leaf_k` wherever the loop previously used `$k` for the resolver grep. Leave the `elsewhere:` consumer grep on the qualified `$k`'s leaf as well — same substitution.

- [ ] **Step 6: Bump the count**

`expected_key_count` counts extracted keys. Qualification does not change how many keys exist, so the count holds — but re-derive rather than assume:

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E "key extraction count|raw key extraction"`
If the reported `got N` differs from 33, set `expected_key_count=N` and note in the inline comment that change 0127 requalified the keys.

- [ ] **Step 7: Run to verify it passes**

Run: `bash tests/test_docket_example_yml.sh`
Expected: exit 0, including the four new asserts.

- [ ] **Step 8: Mutation-test the new floor (both directions)**

```bash
# (a) collapse qualification -> the flat-collision floor must still pass, but the
#     qualified-extraction asserts must redden.
# Temporarily emit only the leaf in the awk nested arm: print leaf (drop `parent "."`).
# Expect: NOT OK - qualified extraction: a nested leaf carries its parent
#
# (b) introduce a REAL flat collision: add `gate: x` as a NEW top-level key in .docket.example.yml.
# Expect: NOT OK - no duplicate FLAT-READ leaf names (gate; ...)
# Revert both.
```

Run each, confirm the named assert reddens, revert.

- [ ] **Step 9: Commit**

```bash
git add tests/test_docket_example_yml.sh
git commit -m "test(0127): qualify example-yml manifest keys by parent block

learnings.enabled already owned the bare leaf `enabled`, structurally blocking
auto_capture.enabled. Qualify block-scoped keys by parent and re-derive the
duplicate floor from the resolver's READ SHAPE: only flat-read keys (top-level
plus the finalize leaves) can collide under yaml_get's head -n1."
```

---

## Task 3: Resolver — `change_types` and the nested `auto_capture` map

The breaking config change. Modeled leaf-for-leaf on the existing `learnings:` block (`yaml_block_body` per layer + a `*_key` accessor giving per-leaf fallback).

**Files:**
- Modify: `scripts/docket-config.sh` — remove the `AUTO_CAPTURE` block (~line 326-336), add the new resolution after it; add three `emit` lines, remove one
- Modify: `.docket.example.yml`, `README.md`, `scripts/docket-config.md`
- Modify: `tests/test_docket_config.sh`, `tests/test_docket_example_yml.sh` (three new `classify_key` arms)

**Interfaces:**
- Consumes: `DOCKET_CHANGE_TYPES_DEFAULT` (Task 1); the qualified manifest (Task 2).
- Produces, in the export block in this order (replacing `emit AUTO_CAPTURE`):
  - `CHANGE_TYPES` — space-separated effective types, configured order preserved. Never empty.
  - `AUTO_CAPTURE_ENABLED` — `true`|`false`.
  - `AUTO_CAPTURE_TYPES` — the literal `all`, or a space-separated subset of `CHANGE_TYPES`. Never empty.

`all` is preserved literally rather than expanded so a consumer can distinguish "every type, including ones a future layer adds" from "this explicit subset" — the spec requires that distinction survive serialization.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_config.sh` (it already provides `mkrepo` and `run`):

```bash
# ---- change 0127: change_types + nested auto_capture -------------------------
d="$tmp/ct-default"; mkrepo "$d"
out="$(run "$d")"
assert "ct: CHANGE_TYPES defaults to the built-in taxonomy" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "chore docs feat fix refactor perf" ]'
assert "ct: AUTO_CAPTURE_ENABLED defaults false" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_ENABLED=//p")" = "false" ]'
assert "ct: AUTO_CAPTURE_TYPES defaults to the literal all" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_TYPES=//p")" = "all" ]'
assert "ct: the retired AUTO_CAPTURE export is gone" \
  '! printf "%s\n" "$out" | grep -q "^AUTO_CAPTURE="'

# repo-committed map, both leaves
d="$tmp/ct-repo"; mkrepo "$d"
printf 'change_types: [feat, fix, chore]\nauto_capture:\n  enabled: true\n  types: [feat]\n' > "$d/.docket.yml"
out="$(run "$d")"
assert "ct: repo change_types replaces the built-in list wholesale" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "feat fix chore" ]'
assert "ct: repo auto_capture.enabled resolves" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_ENABLED=//p")" = "true" ]'
assert "ct: repo auto_capture.types resolves" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_TYPES=//p")" = "feat" ]'

# PER-LEAF inheritance: local overrides enabled only, inheriting types from the repo layer
d="$tmp/ct-leaf"; mkrepo "$d"
printf 'auto_capture:\n  enabled: false\n  types: [fix]\n' > "$d/.docket.yml"
printf 'auto_capture:\n  enabled: true\n' > "$d/.docket.local.yml"
out="$(run "$d")"
assert "ct: local overrides enabled" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_ENABLED=//p")" = "true" ]'
assert "ct: types is INHERITED from the repo layer (per-leaf fallback)" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_TYPES=//p")" = "fix" ]'

# WHOLE-LIST REPLACEMENT (never concatenation)
d="$tmp/ct-replace"; mkrepo "$d"
printf 'change_types: [chore, docs, feat]\n' > "$d/.docket.yml"
printf 'change_types: [feat]\n' > "$d/.docket.local.yml"
out="$(run "$d")"
assert "ct: a higher-layer list REPLACES, never merges" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "feat" ]'

# cross-layer precedence repo-local > repo-committed > global > built-in
d="$tmp/ct-prec"; mkrepo "$d"
mkdir -p "$XDG_CONFIG_HOME/docket"
printf 'change_types: [chore]\n' > "$XDG_CONFIG_HOME/docket/config.yml"
out="$(run "$d")"
assert "ct: global layer resolves when repo layers are silent" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "chore" ]'
printf 'change_types: [docs]\n' > "$d/.docket.yml"
out="$(run "$d")"
assert "ct: repo-committed beats global" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "docs" ]'
printf 'change_types: [perf]\n' > "$d/.docket.local.yml"
out="$(run "$d")"
assert "ct: repo-local beats repo-committed" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^CHANGE_TYPES=//p")" = "perf" ]'
rm -f "$XDG_CONFIG_HOME/docket/config.yml"

# `all` explicitly written is preserved literally
d="$tmp/ct-all"; mkrepo "$d"
printf 'auto_capture:\n  enabled: true\n  types: all\n' > "$d/.docket.yml"
out="$(run "$d")"
assert "ct: an explicit 'all' stays the literal all" \
  '[ "$(printf "%s\n" "$out" | sed -n "s/^AUTO_CAPTURE_TYPES=//p")" = "all" ]'

# --- fail-closed cases -------------------------------------------------------
ct_fails(){ # ct_fails <label> <yaml> <expected-substring>
  local d2="$tmp/ctf-$RANDOM"; mkrepo "$d2"
  printf '%s' "$2" > "$d2/.docket.yml"
  local err rc
  err="$(run "$d2" 2>&1)"; rc=$?
  assert "ct-fail: $1 exits non-zero" '[ "'"$rc"'" != 0 ]'
  assert "ct-fail: $1 diagnostic mentions '"'"'$3'"'"'" 'grep -qF -- "'"$3"'" <<<"'"$err"'"'
}
ct_fails "legacy scalar true"  'auto_capture: true\n'                              'auto_capture'
ct_fails "empty change_types"  'change_types: []\n'                                'change_types'
ct_fails "dup change_types"    'change_types: [feat, feat]\n'                      'duplicate'
ct_fails "malformed type"      'change_types: [Feat]\n'                            'change_types'
ct_fails "non-bool enabled"    'auto_capture:\n  enabled: yes\n'                   'auto_capture.enabled'
ct_fails "types out of taxonomy" 'change_types: [feat]\nauto_capture:\n  types: [docs]\n' 'docs'
ct_fails "dup types"           'auto_capture:\n  types: [feat, feat]\n'            'duplicate'

# The legacy diagnostic must SHOW the new shape (printed-remedy-state-validity)
d="$tmp/ct-legacy"; mkrepo "$d"
printf 'auto_capture: true\n' > "$d/.docket.yml"
err="$(run "$d" 2>&1)" || true
assert "ct: legacy scalar diagnostic shows the nested replacement" \
  'grep -q "enabled:" <<<"$err" && grep -q "types:" <<<"$err"'
assert "ct: legacy scalar diagnostic preserves the old value in the remedy" \
  'grep -q "enabled: true" <<<"$err"'
```

Note the `printf` bodies above use `\n` escapes — `printf` (not `echo`) interprets them, which is why each fixture writes with `printf '...'`.

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_config.sh 2>&1 | grep "NOT OK" | head`
Expected: `NOT OK - ct: CHANGE_TYPES defaults to the built-in taxonomy` and the rest of the new block.

- [ ] **Step 3: Implement resolution**

In `scripts/docket-config.sh`, **delete** the four-line `AUTO_CAPTURE` resolution and its `case` guard (~326-336, keeping the explanatory comment block above it as the basis for the new one), and insert:

```bash
# --- change_types + auto_capture: (change 0127) --------------------------------
# change_types is a LIST, resolved with whole-list replacement: the first layer that sets it wins
# ENTIRELY. Merging would make a built-in value unremovable — a user could only ever add types,
# never drop one, which is the opposite of a configurable taxonomy.
# Inline flow style only (`[a, b]`), matching board_surfaces / agent_harnesses.
ct_raw="$(lcl change_types)"
[ -n "$ct_raw" ] || ct_raw="$(yaml_get "$CFG" change_types)"
[ -n "$ct_raw" ] || ct_raw="$(gbl change_types)"
if [ -z "$ct_raw" ]; then
  CHANGE_TYPES="${DOCKET_CHANGE_TYPES_DEFAULT[*]}"
else
  _ct="${ct_raw#[}"; _ct="${_ct%]}"; _ct="${_ct//,/ }"
  CHANGE_TYPES="$(echo $_ct)"                       # trim/collapse; "[]" => ""
  [ -n "$CHANGE_TYPES" ] || die "unparseable config: change_types must be a non-empty list"
  for _t in $CHANGE_TYPES; do
    docket_change_type_is_wellformed "$_t" \
      || die "unparseable config: change_types entry '$_t' must match [a-z][a-z0-9-]*"
    docket_change_type_is_reserved "$_t" \
      && die "unparseable config: change_types must not contain the reserved value '$_t'"
  done
  _dupes="$(printf '%s\n' $CHANGE_TYPES | sort | uniq -d | tr '\n' ' ')"
  [ -z "${_dupes// /}" ] || die "unparseable config: change_types has duplicate entries: ${_dupes% }"
fi

# auto_capture: is a MAP (change 0127, intentionally breaking). Resolved leaf-by-leaf exactly like
# learnings: — a higher layer may override `enabled` while inheriting `types`. The legacy scalar
# form has NO shim: a top-level auto_capture with a non-empty scalar value is a hard error whose
# diagnostic prints the nested replacement, carrying the user's own value through so the remedy is
# valid in the state that produced it.
for _lyr_f in "$LCFG" "$CFG" "$GCFG"; do
  _legacy="$(yaml_get "$_lyr_f" auto_capture 2>/dev/null || true)"
  if [ -n "$_legacy" ]; then
    die "unparseable config: auto_capture is now a map, not a scalar (got 'auto_capture: $_legacy' in $_lyr_f).
Replace it with:

auto_capture:
  enabled: $_legacy
  types: all"
  fi
done
AC_BLK="$(mktemp)";  yaml_block_body "$CFG"  auto_capture >"$AC_BLK"
GAC_BLK="$(mktemp)"; yaml_block_body "$GCFG" auto_capture >"$GAC_BLK"
LAC_BLK="$(mktemp)"; yaml_block_body "$LCFG" auto_capture >"$LAC_BLK"
trap 'rm -f "$CFG" "$LEARN_BLK" "$GLEARN_BLK" "$LLEARN_BLK" "$AC_BLK" "$GAC_BLK" "$LAC_BLK"' EXIT
ac_key(){  # ac_key <leaf> <default> -> resolved value on stdout
  local v; v="$(yaml_get "$LAC_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$AC_BLK" "$1")"
  [ -n "$v" ] || v="$(yaml_get "$GAC_BLK" "$1")"
  printf '%s' "${v:-$2}"
}
AUTO_CAPTURE_ENABLED="$(ac_key enabled false)"
case "$AUTO_CAPTURE_ENABLED" in
  true|false) ;;
  *) die "unparseable config: auto_capture.enabled must be 'true' or 'false', got '$AUTO_CAPTURE_ENABLED'" ;;
esac
_act_raw="$(ac_key types all)"
if [ "$_act_raw" = all ]; then
  AUTO_CAPTURE_TYPES=all
else
  _act="${_act_raw#[}"; _act="${_act%]}"; _act="${_act//,/ }"
  AUTO_CAPTURE_TYPES="$(echo $_act)"
  [ -n "$AUTO_CAPTURE_TYPES" ] \
    || die "unparseable config: auto_capture.types must be 'all' or a non-empty list"
  for _t in $AUTO_CAPTURE_TYPES; do
    docket_change_type_is_member "$_t" $CHANGE_TYPES \
      || die "unparseable config: auto_capture.types entry '$_t' is not in the effective change_types ($CHANGE_TYPES)"
  done
  _dupes="$(printf '%s\n' $AUTO_CAPTURE_TYPES | sort | uniq -d | tr '\n' ' ')"
  [ -z "${_dupes// /}" ] || die "unparseable config: auto_capture.types has duplicate entries: ${_dupes% }"
fi
```

This block must be placed **after** the `learnings:` block (so the `trap` re-issue keeps `LEARN_BLK` cleanup) and the resolver must source the vocabulary lib. Near the top of `docket-config.sh`, beside its other library sourcing, add:

```bash
. "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"
```

If the resolver does not already source that lib, verify the source line does not clobber resolver-local names — `docket-frontmatter.sh` defines only `field`/`list_field`/`int_field`/`has_section`/`iso_to_epoch`/`resolve_deps`/`readiness`/`finalize_blocked`/`publish_deferred`/`_docket_array_has`/`docket_*` plus the `DOCKET_*` arrays. Grep for each against `docket-config.sh` before committing:

```bash
for n in field list_field int_field has_section iso_to_epoch resolve_deps readiness; do
  grep -n "^${n}()" scripts/docket-config.sh && echo "COLLISION on $n"
done
```

If any collide, do **not** source — instead inline the three `docket_change_type_*` helpers' logic in the resolver and have Task 1's test additionally assert the resolver's copy stays in step (state that choice in the results file).

- [ ] **Step 4: Update the export block**

In the `emit` sequence, replace `emit AUTO_CAPTURE "$AUTO_CAPTURE"` with:

```bash
  emit CHANGE_TYPES "$CHANGE_TYPES"
  emit AUTO_CAPTURE_ENABLED "$AUTO_CAPTURE_ENABLED"
  emit AUTO_CAPTURE_TYPES "$AUTO_CAPTURE_TYPES"
```

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_docket_config.sh`
Expected: exit 0.

- [ ] **Step 6: Document the keys (config-knob-ship-end-to-end)**

`.docket.example.yml` — add, in the documented style of its neighbors:

```yaml
# The change-type taxonomy. Ordered, non-empty, duplicate-free, lowercase [a-z][a-z0-9-]*.
# A higher config layer REPLACES this whole list — it never merges with it.
change_types: [chore, docs, feat, fix, refactor, perf]

# Autonomous mid-run capture of discovered follow-up work into proposed stubs.
# NOTE: this replaced the old scalar `auto_capture: true|false`, which is now a hard error.
auto_capture:
  enabled: false          # true = autonomous skills mint stubs for discovered work
  types: all              # `all`, or a list drawn from change_types, e.g. [feat, fix]
```

`README.md` — in the configuration section, document both keys and the breaking migration. Every README yaml fence is auto-guarded (ADR-0053) and its keys must exist in `.docket.example.yml`, which the block above satisfies.

`scripts/docket-config.md` — add `CHANGE_TYPES`, `AUTO_CAPTURE_ENABLED`, `AUTO_CAPTURE_TYPES` to the export list **in emit order**, remove `AUTO_CAPTURE`, and state whole-list replacement + per-leaf inheritance in the layer-precedence prose.

- [ ] **Step 7: Add the three manifest arms**

In `tests/test_docket_example_yml.sh`'s `classify_key`, add:

```bash
    change_types)             echo 'resolved:CHANGE_TYPES' ;;
    auto_capture.enabled)     echo 'resolved:AUTO_CAPTURE_ENABLED' ;;
    auto_capture.types)       echo 'resolved:AUTO_CAPTURE_TYPES' ;;
```

Remove the retired `auto_capture) echo 'resolved:AUTO_CAPTURE'` arm, and add `auto_capture` to the block-header arm (`finalize|learnings|reclaim|skills|runners|codex|auto_capture`). Bump `expected_key_count` by the net new count and update its inline comment to name change 0127.

`CHANGE_TYPES` is built through the `ct_raw`/`_ct` intermediates, so like `BOARD_SURFACES` it cannot satisfy the same-line correspondence check — add it to `correspondence_exempt` with a one-line reason.

- [ ] **Step 8: Run the config + example + contract suites**

Run: `bash tests/test_docket_config.sh && bash tests/test_docket_example_yml.sh && bash tests/test_script_contracts_coverage.sh`
Expected: all exit 0.

- [ ] **Step 9: Prove no stale `AUTO_CAPTURE` consumer survives**

Run:

```bash
grep -rn "AUTO_CAPTURE\b" --exclude-dir=.git --exclude-dir=.docket --exclude-dir=.worktrees . \
  | grep -v "AUTO_CAPTURE_ENABLED\|AUTO_CAPTURE_TYPES" \
  | grep -v "^./docs/changes/archive/\|^./docs/results/\|^./docs/superpowers/\|^./docs/adrs/"
```

Expected: no output from live surfaces. Every hit under `docs/changes/archive/`, `docs/results/`, `docs/superpowers/`, `docs/adrs/` is a historical record and must be left byte-untouched. Any live hit is a Task 5 obligation — record it and fix it there.

- [ ] **Step 10: Commit**

```bash
git add scripts/docket-config.sh scripts/docket-config.md .docket.example.yml README.md \
        tests/test_docket_config.sh tests/test_docket_example_yml.sh
git commit -m "feat(0127)!: change_types + nested auto_capture map

BREAKING: scalar auto_capture is removed with no shim; it now fails closed with
a diagnostic printing the nested replacement carrying the user's own value.
Exports CHANGE_TYPES, AUTO_CAPTURE_ENABLED, AUTO_CAPTURE_TYPES."
```

---

## Task 4: `mint-stub.sh --type`

**Files:**
- Modify: `scripts/mint-stub.sh` (arg table ~line 35-48, validation ~line 56-74, frontmatter write ~line 184-189)
- Modify: `scripts/mint-stub.md`
- Modify: `skills/docket-new-change/change-template.md`
- Modify: `tests/test_mint_stub.sh`

**Interfaces:**
- Consumes: `docket_change_type_is_wellformed` / `docket_change_type_is_reserved` (Task 1) — `mint-stub.sh` already sources `lib/docket-frontmatter.sh` at line 81.
- Produces: `--type <token>` (required). The minted stub's first frontmatter block carries `type: <token>`. The script performs **no** semantic classification (ADR-0012) — the caller decides the type.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_mint_stub.sh`, reusing its existing fixture helpers:

```bash
# ---- change 0127: --type -----------------------------------------------------
assert "type: minted stub carries the requested type" \
  'grep -qx "type: fix" "$minted_file"'
assert "type: missing --type is a hard error" \
  '! bash "$SCRIPT" --changes-dir "$cd" --title "T" --body-file "$bf" --discovered-from 1 2>/dev/null'
assert "type: a reserved value is rejected" \
  '! bash "$SCRIPT" --changes-dir "$cd" --title "T" --body-file "$bf" --discovered-from 1 --type all 2>/dev/null'
assert "type: an untyped pseudo-value is rejected" \
  '! bash "$SCRIPT" --changes-dir "$cd" --title "T" --body-file "$bf" --discovered-from 1 --type untyped 2>/dev/null'
assert "type: a malformed token is rejected" \
  '! bash "$SCRIPT" --changes-dir "$cd" --title "T" --body-file "$bf" --discovered-from 1 --type "Feat" 2>/dev/null'
assert "type: a control character is rejected (structural injection)" \
  '! bash "$SCRIPT" --changes-dir "$cd" --title "T" --body-file "$bf" --discovered-from 1 --type "$(printf "feat\ntrivial: true")" 2>/dev/null'
```

Bind `$minted_file`, `$cd`, and `$bf` to the fixture the surrounding test already builds; pass `--type fix` on the mint call that produces `$minted_file`.

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_mint_stub.sh 2>&1 | grep "NOT OK"`
Expected: `NOT OK - type: minted stub carries the requested type` (plus the rejection asserts, which currently pass vacuously because `--type` is an unknown argument — confirm the first one is the real red).

- [ ] **Step 3: Implement**

Add `TYPE=""` to the initialization line (~27). Add `--type` to the value-taking option list (~35) and a dispatch arm:

```bash
    --type) TYPE="$2"; shift ;;
```

Add validation beside the existing `--title` control-character gate (~74):

```bash
[ -n "$TYPE" ] || die "missing --type (the caller classifies; this script never infers — ADR-0012)"
case "$TYPE" in *[[:cntrl:]]*) die "--type must not contain control characters" ;; esac
docket_change_type_is_reserved "$TYPE" \
  && die "--type must not be the reserved value '$TYPE' (a query/selector pseudo-value, never a stored type)"
docket_change_type_is_wellformed "$TYPE" \
  || die "--type must match [a-z][a-z0-9-]*, got '$TYPE'"
```

These four gates must sit **after** line 81's `. "$SELF_DIR/lib/docket-frontmatter.sh"`; if the existing `--title` gate precedes that source line, place the `--type` shape gates immediately after the source instead and leave only the control-character check up top.

Write the field beside the other `set_field` calls (~189):

```bash
  set_field "$tmp" type "$TYPE"               || die "set_field type failed for stub $id"
```

`set_field` writes through `awk`'s `ENVIRON`, so no `sed` replacement reinterpretation is possible.

- [ ] **Step 4: Add `type:` to the template**

In `skills/docket-new-change/change-template.md`, after the `priority: medium` line:

```yaml
type:                     # one of the configured change_types (chore|docs|feat|fix|refactor|perf)
```

`mint-stub.sh` comment-strips the template before writing, so the trailing comment does not survive into a minted stub.

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_mint_stub.sh`
Expected: exit 0.

- [ ] **Step 6: Mutation-test the write**

Comment out the `set_field "$tmp" type "$TYPE"` line, run `bash tests/test_mint_stub.sh`.
Expected: `NOT OK - type: minted stub carries the requested type`. Restore the line.

- [ ] **Step 7: Update the contract**

In `scripts/mint-stub.md`, add `--type TYPE` to Usage and the option table (required; caller-classified; rejects reserved values, malformed tokens, and control characters), and note the minted frontmatter now carries `type:`.

- [ ] **Step 8: Commit**

```bash
git add scripts/mint-stub.sh scripts/mint-stub.md skills/docket-new-change/change-template.md tests/test_mint_stub.sh
git commit -m "feat(0127): mint-stub --type writes a classified stub"
```

---

## Task 5: Selective auto-capture in the skill layer

Skills are prose, so the deliverable is prose plus the sentinel guards that keep it honest.

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — the `.docket.yml` block, the change-manifest block, and the **Auto-capture (shared definition)** section
- Modify: `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md` — every `AUTO_CAPTURE` reference
- Modify: `tests/test_skill_facade_wiring.sh`

**Interfaces:**
- Consumes: `AUTO_CAPTURE_ENABLED`, `AUTO_CAPTURE_TYPES`, `CHANGE_TYPES` (Task 3); `mint-stub --type` (Task 4).
- Produces: the documented five-step capture sequence every mint site follows.

- [ ] **Step 1: Write the failing sentinel**

Append to `tests/test_skill_facade_wiring.sh`:

```bash
# ---- change 0127: capture prose reads the new exports ------------------------
mint_sites="skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/SKILL.md"
for f in $mint_sites; do
  body="$(cat "$REPO/$f")"
  assert "0127: $f carries no retired AUTO_CAPTURE reference" \
    '! grep -Eq "AUTO_CAPTURE([^_]|$)" <<<"$body"'
done
conv="$(cat "$REPO/skills/docket-convention/SKILL.md")"
assert "0127: convention names AUTO_CAPTURE_ENABLED" 'grep -q "AUTO_CAPTURE_ENABLED" <<<"$conv"'
assert "0127: convention names AUTO_CAPTURE_TYPES"   'grep -q "AUTO_CAPTURE_TYPES" <<<"$conv"'
assert "0127: convention documents the --type mint argument" 'grep -q -- "--type" <<<"$conv"'
assert "0127: convention states filtering precedes the cap" \
  'grep -qi "before the .*cap\|precedes the cap" <<<"$conv"'
```

Note the `--type` grep uses `-- ` (AGENTS.md: a pattern leading with `--` must declare it).

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_skill_facade_wiring.sh 2>&1 | grep "NOT OK"`
Expected: the four convention asserts red, plus one per file still carrying `AUTO_CAPTURE`.

- [ ] **Step 3: Rewrite the convention's Auto-capture section**

Replace the paragraph beginning "`auto_capture` (default `false`, global-able) governs…" with:

```markdown
`auto_capture` (a map; `enabled` default `false`, `types` default `all`; global-able) governs what
an **autonomous** skill does with genuine follow-up work it discovers mid-run. Disabled, the model
reports it in prose and moves on. Enabled, the model classifies it, and mints it as an ordinary
`proposed` needs-brainstorm stub with `discovered_from:` and `type:` set **only when its type is
admitted** — capture fidelity, **not** autonomy: every minted stub still waits at the human's groom
gate.
```

Replace the mint-call line with the `--type` form:

```markdown
`"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mint-stub --changes-dir .docket/<changes_dir>
--title <title> --type <type> --body-file <file> --discovered-from <this change's id> --minted <n so far>`
```

And add the five-step sequence after the materiality bar:

```markdown
For each material discovery, in this order:

1. Assign exactly one effective configured type (from `CHANGE_TYPES`). The model classifies; the
   script never infers (ADR-0012).
2. `AUTO_CAPTURE_ENABLED` false ⇒ report the discovery in the run report; mint nothing.
3. Enabled but the assigned type is outside `AUTO_CAPTURE_TYPES` ⇒ mint nothing and report the
   proposed title and type as **policy-suppressed**.
4. Enabled and admitted ⇒ call `mint-stub` with `--type`.
5. Mint success, dedup, cap overflow, and hard failure keep ADR-0045's best-effort reporting
   posture and never abort the change being built.

`AUTO_CAPTURE_TYPES` is the literal `all` (every effective type admitted) or an explicit subset.
**Type filtering happens BEFORE the per-invocation cap is consumed** — a suppressed candidate must
never spend one of the run's three mint slots. Dedup stays after admission, immediately before the
mint.
```

Also add `type:` to the convention's change-manifest frontmatter block:

```yaml
type: feat                # one of the configured change_types; set at creation, never inferred
```

and replace the config block's `auto_capture: false` line with the nested form plus `change_types`.

- [ ] **Step 4: Update the three mint-site skills**

In each of `docket-implement-next`, `docket-finalize-change`, `docket-status`, replace every `AUTO_CAPTURE` mention with `AUTO_CAPTURE_ENABLED`, and where the skill describes minting, add that the discovery is classified first and suppressed candidates are reported with their proposed type. Derive the exact sites from a grep rather than a hand-list (AGENTS.md):

```bash
grep -rn "AUTO_CAPTURE" skills/
```

`docket-implement-next`'s Step 3 and Step 6 both mint and share one running `--minted` count — leave that rule intact and state that a policy-suppressed candidate does not increment it.

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_skill_facade_wiring.sh && bash tests/test_skill_size_budgets.sh && bash tests/test_convention_extraction.sh`
Expected: all exit 0. If `test_skill_size_budgets.sh` reddens, the convention grew past its budget — tighten the new prose rather than raising the budget.

- [ ] **Step 6: Commit**

```bash
git add skills/ tests/test_skill_facade_wiring.sh
git commit -m "feat(0127): type-gated auto-capture in the skill layer"
```

---

## Task 6: Board Type column and report-only filters

**Files:**
- Modify: `scripts/render-board.sh` — `table_header_for` (~223-228), the row printers (~243-254), the digest block (~122-181), arg parsing (~27-41)
- Modify: `scripts/render-board.md`, `scripts/docket-status.sh` (~35-70), `scripts/docket-status.md`
- Modify: `tests/test_render_board.sh`, `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: `CHANGE_TYPES` (Task 3); `field <file> type` (existing `field` helper).
- Produces:
  - `render-board.sh --type <token>` / `--priority <token>` — accepted in **both** formats but applied **only** to the digest projection; the markdown writer ignores them entirely.
  - Digest `change` lines gain no new column (parsers key on position); filtering narrows **which** lines appear and which ids reach the `ready` line.
  - Active markdown tables gain a `Type` column rendering `untyped` for a missing value.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_render_board.sh`:

```bash
# ---- change 0127: Type column + filters --------------------------------------
# fixture: three active changes with distinct type/priority, one with no type at all
mkchange(){ # mkchange <dir> <id> <slug> <status> <priority> <type>
  local f="$1/active/$(printf '%04d' "$2")-$3.md"
  { printf -- '---\nid: %s\nslug: %s\ntitle: T%s\nstatus: %s\npriority: %s\n' "$2" "$3" "$2" "$4" "$5"
    [ -n "$6" ] && printf 'type: %s\n' "$6"
    printf 'created: 2026-01-0%s\ndepends_on: []\ntrivial: true\n---\n\n## Why\nx\n' "$2"
  } > "$f"
}
cd_t="$tmp/board127"; mkdir -p "$cd_t/active" "$cd_t/archive"
mkchange "$cd_t" 1 alpha proposed high fix
mkchange "$cd_t" 2 beta  proposed high feat
mkchange "$cd_t" 3 gamma proposed low  ""

md="$(bash "$RB" --changes-dir "$cd_t" --format markdown --repo o/r)"
assert "board: proposed table has a Type column header" 'grep -q "| Type |" <<<"$md"'
assert "board: a typed row renders its type"   'grep -q "\`fix\`" <<<"$md"'
assert "board: an untyped row renders untyped" 'grep -q "untyped" <<<"$md"'

dg="$(bash "$RB" --changes-dir "$cd_t" --format digest)"
assert "digest: unfiltered lists all three" '[ "$(grep -c "^change " <<<"$dg")" = 3 ]'
assert "digest: unfiltered ready has all three" '[ "$(sed -n "s/^ready //p" <<<"$dg")" = "1 2 3" ]'

dgf="$(bash "$RB" --changes-dir "$cd_t" --format digest --type fix)"
assert "digest: --type narrows change lines" '[ "$(grep -c "^change " <<<"$dgf")" = 1 ]'
assert "digest: --type narrows the ready queue" '[ "$(sed -n "s/^ready //p" <<<"$dgf")" = "1" ]'

dgu="$(bash "$RB" --changes-dir "$cd_t" --format digest --type untyped)"
assert "digest: --type untyped selects the missing-type change" \
  '[ "$(sed -n "s/^ready //p" <<<"$dgu")" = "3" ]'

dga="$(bash "$RB" --changes-dir "$cd_t" --format digest --type all)"
assert "digest: --type all is the unfiltered projection" '[ "$dga" = "$dg" ]'

dgp="$(bash "$RB" --changes-dir "$cd_t" --format digest --priority high)"
assert "digest: --priority narrows" '[ "$(sed -n "s/^ready //p" <<<"$dgp")" = "1 2" ]'

dgc="$(bash "$RB" --changes-dir "$cd_t" --format digest --type feat --priority high)"
assert "digest: combined filters AND together" '[ "$(sed -n "s/^ready //p" <<<"$dgc")" = "2" ]'

dgn="$(bash "$RB" --changes-dir "$cd_t" --format digest --type refactor)"
assert "digest: a filter matching nothing still emits a bare ready line" \
  'grep -qx "ready" <<<"$dgn"'

# THE HARD BOUNDARY: a filter never narrows the markdown writer
mdf="$(bash "$RB" --changes-dir "$cd_t" --format markdown --repo o/r --type fix)"
assert "boundary: filtered markdown still contains every active row" \
  '[ "$mdf" = "$md" ]'

# invalid values fail closed
assert "filter: an unknown --priority is rejected" \
  '! bash "$RB" --changes-dir "$cd_t" --format digest --priority urgent 2>/dev/null'
assert "filter: a malformed --type is rejected" \
  '! bash "$RB" --changes-dir "$cd_t" --format digest --type "Fix" 2>/dev/null'
assert "filter: an OBSERVED type absent from the built-in taxonomy is accepted" \
  'bash "$RB" --changes-dir "$cd_t" --format digest --type fix >/dev/null'
```

Bind `$RB` to `$REPO/scripts/render-board.sh` and `$tmp` to the suite's existing temp dir.

Append to `tests/test_docket_status.sh`:

```bash
# ---- change 0127: filter passthrough ----------------------------------------
assert "status: --type reaches the digest projection" \
  'GIT_EDITOR=true bash "$SCRIPT" --digest-only --type fix 2>/dev/null | grep -q "^ready"'
assert "status: an invalid --type exits non-zero without mutating state" \
  '! GIT_EDITOR=true bash "$SCRIPT" --digest-only --type "ALL CAPS" >/dev/null 2>&1'
assert "status: --board-only with a filter still writes a COMPLETE board" \
  'true  # asserted by the board-content comparison in the fixture below'
```

Extend the existing `--board-only` fixture to run once unfiltered and once with `--type fix`, and assert the two written `BOARD.md` files are byte-identical.

- [ ] **Step 2: Run to verify they fail**

Run: `bash tests/test_render_board.sh 2>&1 | grep "NOT OK" | head`
Expected: `NOT OK - board: proposed table has a Type column header` first.

- [ ] **Step 3: Add the Type column**

In `table_header_for`, add `Type` after `Priority` in every **active** arm (leave the archive tables untouched):

```bash
table_header_for(){ case "$1" in
  in-progress) printf '| # | Title | Priority | Type | Spec | Branch |\n|---|-------|----------|------|------|--------|\n' ;;
  proposed)    printf '| # | Title | Priority | Type | Readiness |\n|---|-------|----------|------|-----------|\n' ;;
  blocked)     printf '| # | Title | Priority | Type | Blocked by |\n|---|-------|----------|------|------------|\n' ;;
  deferred)    printf '| # | Title | Priority | Type |\n|---|-------|----------|------|\n' ;;
  implemented) printf '| # | Title | Priority | Type | PR | Readiness |\n|---|-------|----------|------|----|-----------|\n' ;;
esac; }
```

Add a cell helper beside the other cell renderers:

```bash
# The stored value verbatim, or `untyped` when absent. Deliberately NOT validated against the
# effective change_types: another machine's config may have written a type this one does not
# configure, and configuration governs CREATION, never the readability of shared history. Row
# visibility never depends on the type (change 0127) — a type problem must not drop a row.
type_cell(){ # type_cell FILE
  local t; t="$(field "$1" type)"
  printf '%s' "${t:-untyped}"
}
```

Add `$(type_cell "$f")` as a `` `%s` ``-formatted cell after `$priority` in each of the five active row printers, adding one `| \`%s\`` to each format string in the matching position.

- [ ] **Step 4: Add filter parsing and application**

In arg parsing, add `FILTER_TYPE=all FILTER_PRIORITY=all` initializers and:

```bash
    --type)     FILTER_TYPE="$2"; shift ;;
    --priority) FILTER_PRIORITY="$2"; shift ;;
```

After the `--format` validation, validate the filters:

```bash
# Filter validation. `all` is the wildcard (≡ omitted). A type filter accepts `untyped`, or any
# WELL-FORMED token — never only the effective change_types — because a repository legitimately
# contains types written under another machine's configuration, and a query for one must work.
if [ "$FILTER_TYPE" != all ] && [ "$FILTER_TYPE" != untyped ]; then
  docket_change_type_is_wellformed "$FILTER_TYPE" || {
    printf 'render-board: unknown --type value: %s (expected all|untyped|a [a-z][a-z0-9-]* token)\n' "$FILTER_TYPE" >&2
    exit 2
  }
fi
if [ "$FILTER_PRIORITY" != all ]; then
  docket_priority_is_member "$FILTER_PRIORITY" || {
    printf 'render-board: unknown --priority value: %s (expected all|%s)\n' \
      "$FILTER_PRIORITY" "${DOCKET_PRIORITIES[*]}" >&2
    exit 2
  }
fi
```

Add one predicate and apply it in **exactly two** places inside the `if [ "$FORMAT" = digest ]` block — the `change`-line loop and the `ready`-line loop — and nowhere else:

```bash
# Report-only projection filter (change 0127). Applied ONLY inside the digest block: the markdown
# writer above must never consult it, or a filtered --board-only run would write a truncated
# BOARD.md. That boundary is asserted in tests/test_render_board.sh.
digest_admits(){ # digest_admits FILE
  local t p
  if [ "$FILTER_TYPE" != all ]; then
    t="$(field "$1" type)"; t="${t:-untyped}"
    [ "$t" = "$FILTER_TYPE" ] || return 1
  fi
  if [ "$FILTER_PRIORITY" != all ]; then
    p="$(field "$1" priority)"; p="${p:-$DOCKET_PRIORITY_DEFAULT}"
    [ "$p" = "$FILTER_PRIORITY" ] || return 1
  fi
  return 0
}
```

In the `change`-line loop add `digest_admits "$f" || continue` after the `[ -n "$id" ] || continue` guard. In the `ready`-line loop add the same guard beside the existing `digest_readiness … = build-ready || continue` check.

Leave the `backlog <status> <count>` rollups **unfiltered** — they report the real backlog, not the projection. State that in a comment.

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_render_board.sh`
Expected: exit 0, including the byte-identical markdown boundary assert.

- [ ] **Step 6: Wire the passthrough in `docket-status.sh`**

Add `TYPE_FLAG="" PRIORITY_FLAG=""` initializers, parse `--type`/`--priority` into them, and append them to the `render-board.sh` invocation inside `backlog_pass` only:

```bash
  local -a filt=()
  [ -n "$TYPE_FLAG" ]     && filt+=(--type "$TYPE_FLAG")
  [ -n "$PRIORITY_FLAG" ] && filt+=(--priority "$PRIORITY_FLAG")
  if ! out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest "${filt[@]+"${filt[@]}"}" 2>&2)"; then
```

Do **not** pass them to `board-refresh.sh` — that is the writer. Add a comment saying so.

- [ ] **Step 7: Run the status suite**

Run: `GIT_EDITOR=true bash tests/test_docket_status.sh`
Expected: exit 0.

- [ ] **Step 8: Mutation-test the boundary**

Temporarily pass `"${filt[@]}"` to the `board-refresh.sh` call too, run `bash tests/test_render_board.sh` and `GIT_EDITOR=true bash tests/test_docket_status.sh`.
Expected: the board-completeness assert reddens. Revert.

- [ ] **Step 9: Update both contracts**

`scripts/render-board.md` — document the Type column (active tables only; `untyped` fallback; never drops a row) and both filters (accepted values, `all` ≡ omitted, digest-only application).
`scripts/docket-status.md` — add `--type`/`--priority` to Usage and the flag table, stating they narrow only the `change`/`ready` projection and never the sweep, health checks, or the written board.

- [ ] **Step 10: Commit**

```bash
git add scripts/render-board.sh scripts/render-board.md scripts/docket-status.sh scripts/docket-status.md \
        tests/test_render_board.sh tests/test_docket_status.sh
git commit -m "feat(0127): board Type column and report-only --type/--priority filters"
```

---

## Task 7: `backfill-change-types.sh`

**Files:**
- Create: `scripts/backfill-change-types.sh`, `scripts/backfill-change-types.md`, `tests/test_backfill_change_types.sh`
- Modify: `scripts/docket.sh` (`WRAPPED_OPS`, line 81; usage header ~line 29)

**Interfaces:**
- Consumes: `docket_change_type_is_wellformed`/`_is_reserved` (Task 1); `CHANGE_TYPES` (Task 3).
- Produces: `backfill-change-types.sh --changes-dir DIR --map <id>=<type>[,<id>=<type>...] [--dry-run]`. All-or-nothing; idempotent; active-only.

- [ ] **Step 1: Write the failing test**

Create `tests/test_backfill_change_types.sh`:

```bash
#!/usr/bin/env bash
# tests/test_backfill_change_types.sh — the one-time active categorization helper (change 0127).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
SCRIPT="$REPO/scripts/backfill-change-types.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

mkfix(){ # mkfix <dir> -> active/0001,0002 untyped; 0003 typed; archive/0009 untyped
  rm -rf "$1"; mkdir -p "$1/active" "$1/archive"
  printf -- '---\nid: 1\nslug: a\ntitle: A\nstatus: proposed\npriority: high\n---\n\n## Why\nstatus: proposed is discussed here too\n' > "$1/active/0001-a.md"
  printf -- '---\nid: 2\nslug: b\ntitle: B\nstatus: proposed\npriority: low\n---\n\n## Why\nx\n' > "$1/active/0002-b.md"
  printf -- '---\nid: 3\nslug: c\ntitle: C\nstatus: proposed\npriority: low\ntype: feat\n---\n\n## Why\nx\n' > "$1/active/0003-c.md"
  printf -- '---\nid: 9\nslug: z\ntitle: Z\nstatus: done\npriority: low\n---\n\n## Why\nx\n' > "$1/archive/2026-01-01-0009-z.md"
}
arc_hash(){ find "$1/archive" -type f -exec cat {} + | shasum | cut -d' ' -f1; }

# --- happy path --------------------------------------------------------------
d="$tmp/ok"; mkfix "$d"; before="$(arc_hash "$d")"
assert "apply: a complete valid mapping succeeds" \
  'bash "$SCRIPT" --changes-dir "$d" --map "1=fix,2=docs" >/dev/null'
assert "apply: id 1 typed"  'grep -qx "type: fix" "$d/active/0001-a.md"'
assert "apply: id 2 typed"  'grep -qx "type: docs" "$d/active/0002-b.md"'
assert "apply: archive is byte-identical" '[ "$(arc_hash "$d")" = "'"$before"'" ]'
assert "apply: the type lands INSIDE the first frontmatter block" \
  '[ "$(awk "/^---$/{n++; next} n==1 && /^type:/{print \"in\"; exit}" "$d/active/0001-a.md")" = in ]'

# --- idempotent --------------------------------------------------------------
snap="$(cat "$d/active/0001-a.md")"
assert "idempotent: rerunning the applied mapping is a no-op" \
  'bash "$SCRIPT" --changes-dir "$d" --map "1=fix,2=docs" >/dev/null && [ "$(cat "$d/active/0001-a.md")" = "'"$snap"'" ]'

# --- all-or-nothing refusals -------------------------------------------------
refuses(){ # refuses <label> <map>
  local d2="$tmp/r$RANDOM"; mkfix "$d2"
  local h0; h0="$(cat "$d2/active/0001-a.md")"
  assert "refuse: $1 exits non-zero" '! bash "$SCRIPT" --changes-dir "'"$d2"'" --map "'"$2"'" 2>/dev/null'
  assert "refuse: $1 leaves every file untouched" '[ "$(cat "'"$d2"'/active/0001-a.md")" = "'"$h0"'" ]'
}
refuses "unknown id"            "1=fix,77=docs"
refuses "duplicate assignment"  "1=fix,1=docs"
refuses "malformed type"        "1=Fix,2=docs"
refuses "reserved type"         "1=all,2=docs"
refuses "partial mapping"       "1=fix"
refuses "conflicting overwrite" "1=fix,2=docs,3=chore"
refuses "archived id"           "1=fix,2=docs,9=chore"

# --- injection ---------------------------------------------------------------
refuses "control character in a type" "1=$(printf 'fix\ntrivial: true'),2=docs"

exit $fail
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_backfill_change_types.sh`
Expected: FAIL — `bash: .../backfill-change-types.sh: No such file or directory` on every assert.

- [ ] **Step 3: Implement the helper**

Create `scripts/backfill-change-types.sh`:

```bash
#!/usr/bin/env bash
# scripts/backfill-change-types.sh — apply a human-approved id->type mapping to ACTIVE change
# files (change 0127). Deterministic mechanics only: an interactive agent proposes the mapping and
# a human approves it as one decision (ADR-0012); this script validates and applies it, all files
# or none. Never reads or edits <changes-dir>/archive/.
#
# Usage: backfill-change-types.sh --changes-dir DIR --map ID=TYPE[,ID=TYPE...] [--dry-run]
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
. "$SELF_DIR/lib/docket-frontmatter.sh"

CHANGES_DIR=""; MAP=""; DRY=0
die(){ printf '%s\n' "backfill-change-types: $*" >&2; exit 1; }
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="${2:-}"; shift ;;
    --map)         MAP="${2:-}"; shift ;;
    --dry-run)     DRY=1 ;;
    -h|--help)     sed -n '2,8p' "$0"; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done
[ -n "$CHANGES_DIR" ] || die "missing --changes-dir"
[ -n "$MAP" ]         || die "missing --map"
[ -d "$CHANGES_DIR/active" ] || die "no active/ under $CHANGES_DIR"

# --- parse + validate the mapping (nothing is written until every check passes)
declare -A WANT=()
IFS=',' read -r -a _pairs <<< "$MAP"
for _p in "${_pairs[@]}"; do
  case "$_p" in
    *=*) : ;;
    *) die "malformed --map entry '$_p' (expected ID=TYPE)" ;;
  esac
  _id="${_p%%=*}"; _ty="${_p#*=}"
  case "$_id" in ''|*[!0-9]*) die "malformed id '$_id' in --map" ;; esac
  case "$_ty" in *[[:cntrl:]]*) die "type for id $_id contains control characters" ;; esac
  docket_change_type_is_reserved "$_ty" && die "type for id $_id is the reserved value '$_ty'"
  docket_change_type_is_wellformed "$_ty" || die "type for id $_id must match [a-z][a-z0-9-]*, got '$_ty'"
  [ -z "${WANT[$_id]:-}" ] || die "duplicate assignment for id $_id"
  WANT["$_id"]="$_ty"
done

# --- resolve the migration set: ACTIVE files with no non-empty type: ----------
declare -A FILE_OF=()
migration_ids=""
for f in "$CHANGES_DIR"/active/*.md; do
  [ -e "$f" ] || continue
  fid="$(field "$f" id)"; [ -n "$fid" ] || continue
  FILE_OF["$fid"]="$f"
  [ -z "$(field "$f" type)" ] && migration_ids="$migration_ids $fid"
done

for id in "${!WANT[@]}"; do
  [ -n "${FILE_OF[$id]:-}" ] || die "id $id is not an active change (archived ids are never touched)"
  existing="$(field "${FILE_OF[$id]}" type)"
  if [ -n "$existing" ] && [ "$existing" != "${WANT[$id]}" ]; then
    die "id $id already has type '$existing'; refusing to overwrite with '${WANT[$id]}'"
  fi
done
for id in $migration_ids; do
  [ -n "${WANT[$id]:-}" ] || die "incomplete mapping: active change $id has no type and no assignment"
done

# --- apply: all files or none ------------------------------------------------
# Staged into a scratch copy first, then moved into place, so a mid-loop failure cannot leave the
# backlog half-migrated. The write goes through awk's ENVIRON, never a sed replacement, because the
# value reaches us as an argument (model-authored-values-are-untrusted-input).
stage="$(mktemp -d)"; trap 'rm -rf "$stage"' EXIT
wrote=0
for id in "${!WANT[@]}"; do
  src="${FILE_OF[$id]}"
  [ "$(field "$src" type)" = "${WANT[$id]}" ] && continue     # already applied — idempotent
  out="$stage/$(basename "$src")"
  BF_TYPE="${WANT[$id]}" awk '
    BEGIN { val = ENVIRON["BF_TYPE"]; n = 0; done = 0 }
    /^---[[:space:]]*$/ { n++; if (n == 2 && !done) { print "type: " val; done = 1 } ; print; next }
    n == 1 && /^type:[[:space:]]*$/ { print "type: " val; done = 1; next }
    { print }
  ' "$src" > "$out" || die "rewrite failed for id $id"
  grep -qx "type: ${WANT[$id]}" "$out" || die "post-write verification failed for id $id"
  wrote=$((wrote + 1))
done
if [ "$DRY" = 1 ]; then
  printf 'backfill-change-types: dry-run — %s file(s) would change\n' "$wrote"; exit 0
fi
for out in "$stage"/*.md; do
  [ -e "$out" ] || continue
  mv "$out" "$CHANGES_DIR/active/$(basename "$out")" || die "install failed for $(basename "$out")"
done
printf 'backfill-change-types: applied %s\n' "$wrote"
```

The awk anchors on the **second** `---` (the close of the first frontmatter block), so a body line reading `type: …` in prose can never be rewritten — the AGENTS.md frontmatter-anchor rule. The `n == 1 && /^type:[[:space:]]*$/` arm fills a template's empty `type:` placeholder in place rather than adding a duplicate.

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_backfill_change_types.sh`
Expected: exit 0.

- [ ] **Step 5: Mutation-test all-or-nothing**

Move the `for id in $migration_ids` completeness loop to *after* the apply loop, run the suite.
Expected: `NOT OK - refuse: partial mapping leaves every file untouched`. Revert.

- [ ] **Step 6: Wire the facade**

In `scripts/docket.sh`, append `backfill-change-types` to `WRAPPED_OPS` (line 81) and add a usage line beside `mint-stub` (~line 29):

```
#   backfill-change-types [args]  apply a human-approved id->type mapping to active changes
```

- [ ] **Step 7: Write the contract**

Create `scripts/backfill-change-types.md` following the house Purpose / Usage / Behavior / Exit codes / Invariants shape. Invariants must state: active-only; never reads or edits `archive/`; all-or-nothing; idempotent; anchored to the first frontmatter block; refuses unknown ids, duplicate assignments, malformed or reserved types, incomplete mappings, and conflicting overwrites.

- [ ] **Step 8: Run the facade + contract suites**

Run: `bash tests/test_docket_facade.sh && bash tests/test_script_contracts_coverage.sh && bash tests/test_backfill_change_types.sh`
Expected: all exit 0.

- [ ] **Step 9: Commit**

```bash
git add scripts/backfill-change-types.sh scripts/backfill-change-types.md scripts/docket.sh tests/test_backfill_change_types.sh
git commit -m "feat(0127): deterministic active-backlog type backfill helper"
```

---

## Task 8: Migration note and full-suite gate

The live migration of **this** repository's active backlog is deliberately **not** a feature-branch commit: change files live on the `docket` metadata branch, the feature branch never modifies docket metadata, and the mapping needs the human's single-decision approval. This task ships the runbook and proves the whole suite is green.

**Files:**
- Modify: `README.md` (migration note)
- Create: `docs/results/2026-07-22-typed-changes-selective-auto-capture-results.md`

**Interfaces:**
- Consumes: everything above.
- Produces: the merge-gate runbook and the results artifact.

- [ ] **Step 1: Write the migration note**

In `README.md`, add a short subsection under the configuration docs:

````markdown
#### Migrating to typed changes (change 0127)

`auto_capture` became a map. The old scalar is a hard error — rewrite it:

```yaml
# before (no longer valid)
# auto_capture: true

# after
auto_capture:
  enabled: true
  types: all
```

Then categorize the active backlog once. `docket.sh docket-status --type untyped` is the exact
inventory; an agent proposes a complete id→type mapping, you approve it as one decision, and the
deterministic helper applies it:

```bash
docket.sh backfill-change-types --changes-dir .docket/docs/changes --map 7=feat,8=feat,9=fix
```

Archived changes are never reclassified.
````

- [ ] **Step 2: Run the FULL suite (one foreground run)**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/typed-changes-selective-auto-capture && \
  fails=""; for t in tests/*.sh; do \
    if GIT_EDITOR=true bash "$t" >/tmp/0127-$(basename "$t").log 2>&1; then :; else fails="$fails $t"; fi; \
  done; echo "FAILED:${fails:-none}"
```

Expected: `FAILED: none`. If any suite fails, re-run that same test on the unmodified `origin/main` checkout before treating it as a regression (learning `environment`: a red suite in a build sandbox is a hypothesis, not a verdict). Record any pre-existing baseline failure in the results file rather than "fixing" it here.

- [ ] **Step 3: Write the results file**

Create `docs/results/2026-07-22-typed-changes-selective-auto-capture-results.md` from the house results template. It must record:
- the qualified-key guard upgrade (Task 2) and why the spec's `auto_capture.enabled` required it;
- the migration-set correction (#0127 already carries `type: feat`, so it is not in the untyped set);
- **the outstanding manual step:** this repository's own active backlog is still untyped, and the human must run the inventory → propose → approve → apply loop after merge, because it writes to the `docket` metadata branch and needs their approval;
- any baseline suite failures reproduced on `origin/main`;
- whether the resolver sourced `lib/docket-frontmatter.sh` or inlined the helpers (Task 3, Step 3).

- [ ] **Step 4: Commit**

```bash
git add README.md docs/results/2026-07-22-typed-changes-selective-auto-capture-results.md
git commit -m "docs(0127): typed-changes migration runbook and results"
```

---

## Self-Review

**1. Spec coverage.**

| Spec section | Task |
|---|---|
| §1 manifest type + default taxonomy | 1 (vocabulary), 4 (template + mint write) |
| §2 four-layer configuration | 3 (+ 2 unblocking it) |
| §3 selective auto-capture | 5 (prose + gate order), 4 (`--type`) |
| §4 canonical board + read-time filters | 6 |
| §5 one-time categorization | 7 (helper), 8 (this repo's runbook) |
| §6 validation + failure behavior | 3 (config fail-closed), 4 (mint), 6 (filters), 7 (backfill) |
| §7 documentation surfaces | 3 (example/README/resolver contract), 4 (mint contract), 5 (convention + skills), 6 (board/status contracts), 8 (migration note) |
| Verification — configuration | 3 Step 1 |
| Verification — creation and capture | 4 Steps 1/6, 5 Step 1 |
| Verification — board and status | 6 Steps 1/8 |
| Verification — migration | 7 Steps 1/5, 8 Step 2 |

Two deliberate deviations, both recorded in Task 8's results file: (a) the spec's "mutation tests remove the type write from each executable mint path" is realized as Task 4 Step 6 for `mint-stub.sh`, the only *executable* mint path — the other mint sites are skill prose, guarded by Task 5's sentinels; (b) this repository's live categorization is a post-merge human-approved step, not a feature-branch commit, because it writes metadata-branch files.

**2. Placeholder scan.** No TBD/TODO/"handle edge cases"/"similar to Task N". Task 2 Steps 4/6 and Task 3 Step 3 direct the implementer to *derive* values (the other qualified arms, the key count, a name-collision check) rather than hard-coding numbers this plan cannot observe — each states the exact command to run and the exact remedy for each outcome, which is a derivation instruction, not a placeholder.

**3. Type consistency.** `docket_change_type_is_member` / `_is_reserved` / `_is_wellformed` and `DOCKET_CHANGE_TYPES_DEFAULT` / `DOCKET_CHANGE_TYPE_RESERVED` are spelled identically in Tasks 1, 3, 4, 6, 7. Exports are `CHANGE_TYPES` / `AUTO_CAPTURE_ENABLED` / `AUTO_CAPTURE_TYPES` throughout. `type_cell` and `digest_admits` (Task 6) and `ac_key` (Task 3) are each defined once and referenced only within their own task.
