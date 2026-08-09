<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0276 — Dummy mode — persona-calibrated human-facing language simplification](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0276-dummy-mode.md)**
<!-- docket:backlink:end -->

# Dummy Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a `dummy_mode:` config key whose free-text `persona:` calibrates docket's *human-facing* prose — dialogue and run reports rewritten for that reader, results/PR/change-sections gaining an authored `### In plain terms` block — while every agent-facing artifact keeps full technical density.

**Architecture:** A convention-prose feature with exactly one code surface. `scripts/docket-config.sh` resolves three leaves (`enabled`, `persona`, `surfaces`) through the existing layered block reader and emits three exports; `skills/docket-convention` gains a compact *Dummy mode (shared definition)* section plus a `references/dummy-mode.md` carrying the mechanics; six eligible skill bodies get a one-line pointer each. No new scripts, no new runtime dependencies, no agent-behavior change.

**Tech Stack:** Bash 4.4+ (`scripts/docket-config.sh`), the repo's standalone Bash test files (`tests/test_*.sh`), markdown skill bodies under `skills/`.

## Global Constraints

Copied verbatim from the reconciled spec and from AGENTS.md, which every task inherits:

- **Persona is a single-line scalar.** `config_line_scalar_get` → `config_normalize_scalar` parses one line and strips from the first `#` **before** unquoting. A YAML block scalar (`>`, `>-`, `|`, `|-`, with any chomp/indent modifier) is **not supported in v1** and is a hard error naming the quoted single-line form. A `#`-truncated persona is detected and refused, never exported as a fragment.
- **`DUMMY_MODE_SURFACES` exports `all` LITERALLY**, never pre-expanded — the `auto_capture.types` precedent, so "every surface including future ones" stays distinguishable from an explicit subset. This resolves the change's one open question and must be stated in `scripts/docket-config.md`.
- **Layer classification: `scope: any layer`** — global-able, NOT coordination-fenced. It changes prose tone, not shared non-re-derivable state.
- **Default off:** `dummy_mode.enabled` defaults to `false`. A blank persona **never disables** anything — it falls back to the shipped default persona with a one-line stderr **notice** (not a warning).
- **The default persona lives in exactly one place** — `scripts/docket-config.sh` — and is quoted (not re-authored) everywhere else. Verbatim text:
  > A mid-level software engineer: solid grasp of software architecture and general engineering concepts — APIs, testing, CI, version control — but only working-level fluency in any specific programming language, so avoid language-specific idioms unless glossed. Assume no familiarity with this project's internal vocabulary; introduce each project-specific term with a one-clause explanation.
- **Five surface tokens, exactly:** `dialogue`, `reports` (**replace**), `results`, `change-sections`, `pr` (**additive**). An unknown token is warned-and-ignored; an empty list is equivalent to `enabled: false` for eligibility.
- **Agent-safety rule:** a `### In plain terms` block is for the human and is **never a decision input** — reconcile, review, and planning read the technical content only.
- **Quote any YAML scalar** carrying a colon-space, trailing colon, ` #`, leading indicator character, or boolean keyword (AGENTS.md). This applies to every `persona:` value written in this change's docs and fixtures.
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration (AGENTS.md). Key guards on syntactic **shape**, never an enumerated list of spellings.
- **Never `producer | early-exiting-consumer`** under `set -o pipefail`; capture into a variable, then `grep <<<"$var"` (AGENTS.md).
- **Run the whole suite at the build gate:** `scripts/run-tests.sh` (the resolved `finalize.test_command`). A trailing `OVER BUDGET:` line is a finding to act on.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/docket-config.sh` | **Modify.** The one code surface: default-persona constant, surface-token constant, the `dm_key` leaf reader, validation, three `emit` calls. |
| `scripts/docket-config.md` | **Modify.** Contract: three rows in the key table, three entries in the export list, the `all`-is-literal statement, the block-scalar and `#` limitations. |
| `.docket.example.yml` | **Modify.** The canonical all-key reference gains a `dummy_mode:` block with one header-level `scope: any layer` tag covering three leaves. |
| `README.md` | **Modify.** User-facing section + the five-example persona gallery. |
| `skills/docket-convention/SKILL.md` | **Modify.** A compact *Dummy mode (shared definition)* section: what it is, the exports, the agent-safety rule, and a blocking pointer to the reference. |
| `skills/docket-convention/references/dummy-mode.md` | **Create.** The mechanics: token table, replace/additive semantics, ad-hoc session enablement, the not-eligible list, authoring guidance. |
| `skills/docket-{new-change,groom-next,implement-next,finalize-change,status,auto-groom}/SKILL.md` | **Modify.** One pointer line each, naming only the surfaces that skill owns. |
| `tests/test_docket_config.sh` | **Modify.** Resolution coverage, `DM-a` … `DM-l`. |
| `tests/test_docket_example_yml.sh` | **Modify.** `expected_nested_key_count` 25 → 28, with the substantive confirmation the guard's own message demands. |
| `tests/test_dummy_mode.sh` | **Create.** Prose guards: the convention section, the agent-safety rule, the six skill pointers. |
| `tests/runtime-budgets.tsv` | **Modify.** One registry row for the new test file. |
| `tests/test_skill_size_budgets.sh` | **Modify.** Raise the rows this change grows; add a row for the new reference file. |

Task order is forced by two guards: `.docket.example.yml`'s "every documented key has a real consumer" assert requires the resolver to exist first (Task 1 before Task 2), and the size-budget table must be edited in the same diff that grows a skill body (Tasks 3 and 4 each carry their own rows).

---

### Task 1: Config resolution and the three exports

**Files:**
- Modify: `scripts/docket-config.sh` (constants near the other `DOCKET_*_DEFAULT` declarations; the resolution block immediately after the `auto_capture` block; three `emit` calls after `emit SKILL_FINISH`, before `emit BOOTSTRAP`)
- Test: `tests/test_docket_config.sh` (append a `DM-a` … `DM-l` block after the `DOB-` block at end of file)

**Interfaces:**
- Consumes: nothing from earlier tasks. Existing helpers only — `config_block_get <layer> <block> <leaf>`, `parse_inline_list`, `emit`, `die`, and the test file's own `mkrepo`, `run`, `run_resolver_with`, `assert`.
- Produces: three exported names later tasks document and guard — `DUMMY_MODE_ENABLED` (`true`|`false`), `DUMMY_MODE_PERSONA` (non-empty always — the default persona when unset), `DUMMY_MODE_SURFACES` (the literal `all`, or a space-separated subset of `dialogue reports results change-sections pr`, or the empty string when a list resolves to nothing). Also the shell constant `DOCKET_DUMMY_MODE_DEFAULT_PERSONA`.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_config.sh`, after the final `DOB-` assert. It follows that block's shape exactly: fixtures via the file's own `mkrepo`/`run`/`run_resolver_with`, every assert reading the **emitted lines** rather than eval-ing the export block.

```bash
# ---- change 0276: dummy_mode (DM-a … DM-l) ------------------------------------
# Persona-calibrated human-facing prose. Three leaves in one nested block, read leaf-by-leaf
# exactly like auto_capture:/learnings:, so a machine layer can flip `enabled` while inheriting
# `persona`. Every assert reads emitted lines (the GOB header states why).
mkrepo "$tmp/dm-a"
dm_out_default="$(run "$tmp/dm-a" --export)"
assert "DM-a: enabled defaults to false" \
  'grep -qxF "DUMMY_MODE_ENABLED=false" <<<"$dm_out_default"'
assert "DM-a: surfaces defaults to the literal all" \
  'grep -qxF "DUMMY_MODE_SURFACES=all" <<<"$dm_out_default"'

# The default persona is emitted even when dummy mode is OFF: the spec's rule is that skills never
# special-case an empty persona. Bound the assert to a distinctive clause rather than the whole
# paragraph, so a re-wrap of the constant does not redden it.
assert "DM-b: the shipped default persona is exported when none is configured" \
  'grep -q "^DUMMY_MODE_PERSONA=.*mid-level software engineer" <<<"$dm_out_default"'
assert "DM-b: the default persona glosses project-internal vocabulary" \
  'grep -q "^DUMMY_MODE_PERSONA=.*one-clause explanation" <<<"$dm_out_default"'

mkrepo "$tmp/dm-c"
cat > "$tmp/dm-c/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: true
  persona: "Reads YAML, not bash. Explain scripts by outcome."
EOF
git -C "$tmp/dm-c" add .docket.yml; git -C "$tmp/dm-c" commit --quiet -m cfg
git -C "$tmp/dm-c" push --quiet origin main
dm_out_committed="$(run "$tmp/dm-c" --export)"
assert "DM-c: committed layer is honored for enabled" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_out_committed"'
assert "DM-c: a quoted persona survives with its spaces and punctuation" \
  'grep -qxF "DUMMY_MODE_PERSONA=Reads YAML, not bash. Explain scripts by outcome." <<<"$dm_out_committed"'
assert "DM-c: an unset surfaces leaf still defaults to all" \
  'grep -qxF "DUMMY_MODE_SURFACES=all" <<<"$dm_out_committed"'

# Per-leaf fallback: the local layer flips one leaf and INHERITS the other two.
mkrepo "$tmp/dm-d"
cat > "$tmp/dm-d/.docket.yml" <<'EOF'
metadata_branch: main
dummy_mode:
  enabled: false
  persona: "Committed persona."
EOF
git -C "$tmp/dm-d" add .docket.yml; git -C "$tmp/dm-d" commit --quiet -m cfg
git -C "$tmp/dm-d" push --quiet origin main
printf 'dummy_mode:\n  enabled: true\n' > "$tmp/dm-d/.docket.local.yml"
dm_out_local="$(run "$tmp/dm-d" --export)"
assert "DM-d: repo-local outranks committed on the leaf it sets" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_out_local"'
assert "DM-d: the leaf the local layer did NOT set is inherited, not defaulted" \
  'grep -qxF "DUMMY_MODE_PERSONA=Committed persona." <<<"$dm_out_local"'

# Blank persona with dummy mode ON: default persona + a NOTICE on stderr. Never a warning, never
# disabled — the spec is explicit that a blank persona is a supported configuration.
dm_blank_err="$(run_resolver_with "dummy_mode:\n  enabled: true\n  persona: \"\"\n" 2>&1 >/dev/null)"
dm_blank_out="$(run_resolver_with "dummy_mode:\n  enabled: true\n  persona: \"\"\n" 2>/dev/null)"
assert "DM-e: a blank persona does not abort the resolver" \
  '[ -n "$dm_blank_out" ]'
assert "DM-e: a blank persona still resolves enabled: true" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_blank_out"'
assert "DM-e: a blank persona falls back to the default persona" \
  'grep -q "^DUMMY_MODE_PERSONA=.*mid-level software engineer" <<<"$dm_blank_out"'
# Bind the word "notice" to what it is a notice ABOUT, so a rewrite that keeps the word and drops
# the subject reddens (learnings: prose-guard-binds-phrase-to-claim).
assert "DM-e: the fallback prints a notice naming the persona" \
  'grep -qE "notice:[^.]{0,120}persona" <<<"$dm_blank_err"'
assert "DM-e: the fallback is not phrased as a warning" \
  '! grep -qE "warning:[^.]{0,120}dummy_mode.persona" <<<"$dm_blank_err"'

# A BLOCK SCALAR is a hard error. The reader is single-line, so `persona: >` would otherwise
# resolve to the literal ">" and export a one-character persona that looks configured.
dm_fold_rc=0
dm_fold_err="$(run_resolver_with "dummy_mode:\n  enabled: true\n  persona: >\n    folded text here\n" 2>&1 >/dev/null)" || dm_fold_rc=$?
assert "DM-f: a folded block scalar aborts the resolver" '[ "$dm_fold_rc" -ne 0 ]'
assert "DM-f: the diagnostic names the key and the supported form" \
  'grep -qE "dummy_mode.persona[^.]{0,160}single-line" <<<"$dm_fold_err"'
dm_lit_rc=0
dm_lit_err="$(run_resolver_with "dummy_mode:\n  enabled: true\n  persona: |-\n    literal text here\n" 2>&1 >/dev/null)" || dm_lit_rc=$?
assert "DM-f: a literal block scalar with a chomp indicator also aborts" '[ "$dm_lit_rc" -ne 0 ]'
assert "DM-f: the literal-form diagnostic also names the key" \
  'grep -qF -- "dummy_mode.persona" <<<"$dm_lit_err"'

# A `#` inside the persona is eaten by the shared reader BEFORE unquoting, leaving an unbalanced
# leading quote. Refuse it loudly rather than exporting the fragment.
dm_hash_rc=0
dm_hash_err="$(run_resolver_with "dummy_mode:\n  enabled: true\n  persona: \"knows git # and yaml\"\n" 2>&1 >/dev/null)" || dm_hash_rc=$?
assert "DM-g: a persona containing '#' aborts instead of exporting a fragment" \
  '[ "$dm_hash_rc" -ne 0 ]'
assert "DM-g: the diagnostic names the offending character and the key" \
  'grep -qE "dummy_mode.persona[^.]{0,160}#" <<<"$dm_hash_err"'

# surfaces: an explicit subset is kept in order; an unknown token is warned-and-ignored, never fatal.
dm_sub_out="$(run_resolver_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, pr]\n" 2>/dev/null)"
assert "DM-h: an explicit subset replaces the literal all" \
  'grep -qxF "DUMMY_MODE_SURFACES=dialogue pr" <<<"$dm_sub_out"'
dm_unk_rc=0
dm_unk_err="$(run_resolver_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, bogus]\n" 2>&1 >/dev/null)" || dm_unk_rc=$?
dm_unk_out="$(run_resolver_with "dummy_mode:\n  enabled: true\n  surfaces: [dialogue, bogus]\n" 2>/dev/null)"
assert "DM-i: an unknown surface token does not abort the run" '[ "$dm_unk_rc" -eq 0 ]'
assert "DM-i: the unknown token is dropped and the known one kept" \
  'grep -qxF "DUMMY_MODE_SURFACES=dialogue" <<<"$dm_unk_out"'
assert "DM-i: the unknown token is named in a warning" \
  'grep -qE "warning:[^.]{0,160}bogus" <<<"$dm_unk_err"'

# An empty list is legal and means "no eligible surface" — the spec's equivalent-to-disabled case.
dm_empty_out="$(run_resolver_with "dummy_mode:\n  enabled: true\n  surfaces: []\n" 2>/dev/null)"
assert "DM-j: an empty surfaces list is legal" \
  'grep -qxF "DUMMY_MODE_ENABLED=true" <<<"$dm_empty_out"'
assert "DM-j: an empty surfaces list exports an empty value, not 'all'" \
  'grep -qxF "DUMMY_MODE_SURFACES=" <<<"$dm_empty_out"'

# enabled fails CLOSED on garbage, like learnings.enabled / auto_capture.enabled.
dm_bad_rc=0
dm_bad_err="$(run_resolver_with "dummy_mode:\n  enabled: sometimes\n" 2>&1 >/dev/null)" || dm_bad_rc=$?
assert "DM-k: a non-boolean enabled aborts the resolver" '[ "$dm_bad_rc" -ne 0 ]'
assert "DM-k: the diagnostic names the key" \
  'grep -qF -- "dummy_mode.enabled" <<<"$dm_bad_err"'

# ORDER: the trio emits after SKILL_FINISH and before BOOTSTRAP. Line numbers are derived per key
# so a missing key reads as an empty extraction rather than shifting a positional match.
dm_ln(){ grep -n "^$1=" <<<"$dm_out_default" | cut -d: -f1; }
dm_n_fin="$(dm_ln SKILL_FINISH)"
dm_n_en="$(dm_ln DUMMY_MODE_ENABLED)"
dm_n_pe="$(dm_ln DUMMY_MODE_PERSONA)"
dm_n_su="$(dm_ln DUMMY_MODE_SURFACES)"
dm_n_boot="$(dm_ln BOOTSTRAP)"
assert "DM-l: all five emit positions were extracted (fin=$dm_n_fin en=$dm_n_en pe=$dm_n_pe su=$dm_n_su boot=$dm_n_boot)" \
  '[ -n "$dm_n_fin" ] && [ -n "$dm_n_en" ] && [ -n "$dm_n_pe" ] && [ -n "$dm_n_su" ] && [ -n "$dm_n_boot" ]'
assert "DM-l: the trio emits between SKILL_FINISH and BOOTSTRAP, in enabled/persona/surfaces order" \
  '[ "${dm_n_en:-0}" -gt "${dm_n_fin:-0}" ] && [ "${dm_n_pe:-0}" -gt "${dm_n_en:-0}" ] \
   && [ "${dm_n_su:-0}" -gt "${dm_n_pe:-0}" ] && [ "${dm_n_su:-0}" -lt "${dm_n_boot:-0}" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -c "^NOT OK"`
Expected: a non-zero count, with the `NOT OK - DM-…` lines naming missing `DUMMY_MODE_*` exports. Confirm at least `DM-a`, `DM-b`, `DM-l` are among them — if the file is green, the block was appended after an early `exit`.

- [ ] **Step 3: Add the constants**

In `scripts/docket-config.sh`, beside the other `DOCKET_*` default constants (the same region as `DOCKET_CHANGE_TYPES_DEFAULT`):

```bash
# --- dummy_mode defaults (change 0276) -----------------------------------------
# The SHIPPED DEFAULT PERSONA lives here and nowhere else: docs quote it, skills read the export.
# A defensible median reader for a technical repo — it strips expert idioms without going ELI5, and
# it bakes in the universal half of the problem (project-internal jargon is unknown to every
# newcomer at any skill level, so the default always glosses it).
DOCKET_DUMMY_MODE_DEFAULT_PERSONA="A mid-level software engineer: solid grasp of software architecture and general engineering concepts — APIs, testing, CI, version control — but only working-level fluency in any specific programming language, so avoid language-specific idioms unless glossed. Assume no familiarity with this project's internal vocabulary; introduce each project-specific term with a one-clause explanation."
# The five v1 surface tokens. docket-convention's shared definition owns their SEMANTICS; this is
# only the admission list the resolver validates against.
DOCKET_DUMMY_MODE_SURFACES=(dialogue reports results change-sections pr)
```

- [ ] **Step 4: Add the resolution block**

In `scripts/docket-config.sh`, immediately after the `auto_capture` resolution block (after `AUTO_CAPTURE_TYPES` is settled) and before the `skills:` block:

```bash
# --- dummy_mode: persona-calibrated HUMAN-FACING prose (change 0276) -----------
# Nested block, parsed leaf-by-leaf exactly like learnings:/reclaim:/auto_capture: — which is what
# gives per-leaf fallback (a machine layer may flip `enabled` while inheriting `persona`) and what
# keeps `enabled` from colliding with learnings.enabled under the snapshot scalar reader.
# Global-able, NOT coordination-fenced: it changes prose tone, never shared non-re-derivable state.
dm_key(){  # dm_key <leaf> <default> -> resolved value on stdout
  local v; v="$(config_block_get local dummy_mode "$1")"
  [ -n "$v" ] || v="$(config_block_get committed dummy_mode "$1")"
  [ -n "$v" ] || v="$(config_block_get global dummy_mode "$1")"
  printf '%s' "${v:-$2}"
}
DUMMY_MODE_ENABLED="$(dm_key enabled false)"
case "$DUMMY_MODE_ENABLED" in
  true|false) ;;
  *) die "unparseable config: dummy_mode.enabled must be 'true' or 'false', got '$DUMMY_MODE_ENABLED'" ;;
esac

DUMMY_MODE_PERSONA="$(dm_key persona '')"
# The shared scalar reader is SINGLE-LINE and strips from the first `#` BEFORE unquoting, so two
# YAML shapes that look right resolve to garbage. Both are refused loudly rather than exported.
#   1. A block scalar (`>`, `|`, with any chomp/indent modifier) resolves to just the indicator.
#      v1 does not support it: extending the shared reader for one cosmetic key would put every
#      skill's Step 0 at risk, and the folded form can be added later without breaking a
#      single-line persona.
#   2. A `#` anywhere in the value truncates it, leaving an UNBALANCED LEADING QUOTE — the
#      signature this branch keys on, since a well-formed quoted scalar has both quotes stripped.
#      Known limitation, stated rather than papered over: a persona whose text legitimately BEGINS
#      with a quote character trips this too. That shape is vanishingly rare and the diagnostic
#      names the real constraint either way.
case "$DUMMY_MODE_PERSONA" in
  '>'|'|'|'>'[-+]|'|'[-+]|'>'[0-9]*|'|'[0-9]*)
    die "unparseable config: dummy_mode.persona must be a single-line quoted scalar — YAML block scalars (>, |, >-, |-) are not supported. Write it as: persona: \"<one line>\"" ;;
esac
case "$DUMMY_MODE_PERSONA" in
  '"'*|\'*)
    die "unparseable config: dummy_mode.persona looks truncated at a '#' — that character opens a YAML comment and is not supported inside a persona. Remove the '#' and keep the persona on one quoted line." ;;
esac
if [ -z "$DUMMY_MODE_PERSONA" ]; then
  # A blank persona is a SUPPORTED configuration, not a misconfiguration: it selects the shipped
  # default. Notice, never warning — and only when dummy mode is actually on, since the export
  # carries the default unconditionally so skills never special-case an empty persona.
  [ "$DUMMY_MODE_ENABLED" = true ] && printf 'docket-config: notice: dummy_mode is enabled with no persona set — using the shipped default persona\n' >&2
  DUMMY_MODE_PERSONA="$DOCKET_DUMMY_MODE_DEFAULT_PERSONA"
fi

# `all` is preserved LITERALLY rather than expanded, on the auto_capture.types precedent: a
# consumer can still tell "every surface, including ones added later" from an explicit subset.
dm_surfaces_raw="$(dm_key surfaces all)"
if [ "$dm_surfaces_raw" = all ]; then
  DUMMY_MODE_SURFACES=all
else
  dm_kept=()
  dm_norm="$(parse_inline_list "$dm_surfaces_raw")"
  if [ -n "$dm_norm" ]; then
    read -r -a dm_arr <<< "$dm_norm"
    for dm_tok in "${dm_arr[@]}"; do
      dm_known=0
      for dm_ref in "${DOCKET_DUMMY_MODE_SURFACES[@]}"; do
        [ "$dm_tok" = "$dm_ref" ] && { dm_known=1; break; }
      done
      if [ "$dm_known" -eq 1 ]; then
        dm_kept+=("$dm_tok")
      else
        # Warned-and-ignored, never fatal: a typo in a tone knob must never abort a build.
        printf 'docket-config: warning: unknown dummy_mode.surfaces token %s — ignored (known: %s)\n' \
          "$dm_tok" "${DOCKET_DUMMY_MODE_SURFACES[*]}" >&2
      fi
    done
  fi
  # `${dm_kept[*]-}` (not `:-`): an EMPTY list is a legal value meaning "no eligible surface", so
  # the empty case must resolve to the empty string rather than be defaulted away.
  DUMMY_MODE_SURFACES="${dm_kept[*]-}"
fi
```

- [ ] **Step 5: Add the three emits**

In the `emit` sequence, between `emit SKILL_FINISH "$SKILL_FINISH"` and `emit BOOTSTRAP "$BOOTSTRAP"`:

```bash
  emit DUMMY_MODE_ENABLED "$DUMMY_MODE_ENABLED"
  emit DUMMY_MODE_PERSONA "$DUMMY_MODE_PERSONA"
  emit DUMMY_MODE_SURFACES "$DUMMY_MODE_SURFACES"
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_docket_config.sh 2>&1 | grep -E "^(NOT OK|ok - DM-)" | tail -40`
Expected: every `DM-` assert `ok`, and no `NOT OK` lines anywhere in the file.

Also confirm the live repo still resolves, since every skill's Step 0 runs this path:
Run: `scripts/docket-config.sh --export | grep DUMMY_MODE`
Expected: `DUMMY_MODE_ENABLED=false`, a `DUMMY_MODE_PERSONA=` line containing `mid-level software engineer`, `DUMMY_MODE_SURFACES=all`.

- [ ] **Step 7: Mutation-test the two refusals**

Guards are code. Temporarily delete the block-scalar `case` (keep a backup copy of the file — `git checkout --` would destroy the uncommitted work being tested), re-run `bash tests/test_docket_config.sh`, and confirm `DM-f` goes `NOT OK`. Restore from the backup, then repeat for the `#`-truncation `case` and confirm `DM-g` reddens. Record both readings in the commit message.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-config.sh tests/test_docket_config.sh
git commit -m "feat(0276): resolve dummy_mode.{enabled,persona,surfaces} and export them"
```

---

### Task 2: Documentation surfaces — contract, example config, README

**Files:**
- Modify: `scripts/docket-config.md` (key table; export list; a limitations note)
- Modify: `.docket.example.yml` (new `dummy_mode:` block)
- Modify: `tests/test_docket_example_yml.sh` (`expected_nested_key_count` 25 → 28)
- Modify: `README.md` (user-facing section + persona gallery)

**Interfaces:**
- Consumes: Task 1's three exports and the default-persona constant — the "real consumer" `.docket.example.yml`'s guard requires for each documented key.
- Produces: nothing later tasks consume in code; Tasks 3 and 4 quote the token names this task documents.

- [ ] **Step 1: Run the example-yml guard to see the current state**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`. This is the baseline — the count assert must go red *because of this task's edit*, not before it.

- [ ] **Step 2: Add the `dummy_mode:` block to `.docket.example.yml`**

Place it immediately after the `auto_capture:` block (both are model-behavior policy knobs). One header-level scope tag covers all three leaves — the same rule-2 shape `auto_capture:` uses. Register check: this comment is written for a user deciding whether to set the key, so it leads with the payoff, not with the change number or the mechanism.

```yaml
# dummy_mode — rewrite docket's HUMAN-FACING prose for a reader you describe. When a question from
# a groom or a run report assumes vocabulary or language expertise you do not have, this stops you
# from having to ask "simplify that" every time. Agent-facing artifacts (plans, specs, learnings,
# build evidence) always keep full technical density — only what a human reads changes.
#   enabled  — off by default.
#   persona  — WHO the reader is, in your own words: what they know, what they do not, and how
#              trade-offs should be framed. Leave it blank to get the shipped default (a mid-level
#              engineer, architecture-literate, working-level in any one language, all project
#              jargon glossed). ONE QUOTED LINE — a YAML block scalar (`>` or `|`) and a `#` inside
#              the text are both hard errors, because the config reader is line-oriented.
#   surfaces — `all` (default) or a subset of: dialogue, reports, results, change-sections, pr.
#              The first two are REWRITTEN for the persona; the last three keep their technical
#              content and gain an authored "In plain terms" block. An empty list means none.
# Primarily a per-repo setting: the persona describes THIS repo's reader, and the same person may
# be an expert in one repo's domain and a novice in another's.
# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)
dummy_mode:
  enabled: false
  persona: ""
  surfaces: all
```

- [ ] **Step 3: Run the guard to verify it fails on the count**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -E "^NOT OK"`
Expected: exactly one failure — the `scope tag: the pass enumerated exactly 25 keys` assert, now reporting `got 28`. Two things must be true before touching the number, and the guard's own message demands both: the block carries its **own** `scope:` tag (it does, on the header, covering all three leaves by rule 2), and the adjacency-inheritance counter has **not** moved. Confirm the second explicitly:

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep "adjacency"`
Expected: still `ok` at exactly 2 — if this moved, a leaf is riding the rule-4 free ride and needs its own tag instead of a count bump.

- [ ] **Step 4: Update the expected count**

In `tests/test_docket_example_yml.sh`, change `expected_nested_key_count=25` to `28`, and extend the comment block above it with the same accounting the existing entries use:

```bash
# +3 (change 0276): dummy_mode.{enabled,persona,surfaces} — all three covered by rule 2, the
# `dummy_mode:` header's own `# scope: any layer` tag; none inherits via rule-4 adjacency, so
# expected_adjacency_inherit_count is unchanged at 2.
expected_nested_key_count=28
```

- [ ] **Step 5: Document the key in `scripts/docket-config.md`**

Add three rows to the key table, immediately after the `auto_capture.types` row, matching that table's column shape:

```markdown
| `dummy_mode.enabled` | `false` | yes | read from the nested `dummy_mode:` block; resolves repo-local > repo-committed > global; `true`/`false`, anything else aborts (change 0276) |
| `dummy_mode.persona` | the shipped default persona | yes | read from the nested `dummy_mode:` block. **Single-line scalar only** — a YAML block scalar (`>`, `|`, with any chomp/indent modifier) and a `#` inside the text each abort with a diagnostic, because the shared reader is line-oriented and strips from the first `#` before unquoting. A blank or absent value exports the shipped default persona with a stderr **notice** (never a warning, never disabled), so consumers never special-case an empty persona (change 0276) |
| `dummy_mode.surfaces` | `all` | yes | read from the nested `dummy_mode:` block; the literal scalar `all` (**preserved verbatim, never expanded** — the `auto_capture.types` precedent, so "every surface including future ones" stays distinguishable from an explicit subset) or an inline list drawn from `dialogue`, `reports`, `results`, `change-sections`, `pr`. An unknown token is warned-and-ignored; an empty list exports the empty string and means no eligible surface (change 0276) |
```

Add the three names to the export list in the same file, in emit order, positioned after `SKILL_FINISH` and before `BOOTSTRAP`.

- [ ] **Step 6: Document it in `README.md`**

Add a section after the `auto_capture` section, titled `### Speaking your language (dummy_mode)`. It must contain: the problem in one paragraph; the config block as written; the five surface tokens with replace-vs-additive; the statement that agent-facing artifacts are never simplified; the ad-hoc "enable dummy mode" session request; and the five-example persona gallery **copied from the reconciled spec's gallery section verbatim** (all five are already in single-line quoted form). Close the gallery with the composition axes the spec names: subject-matter gaps, language gaps, tooling/vocabulary gaps, framing preferences.

Also add `dummy_mode:` to each of the two abbreviated config listings in README that already show `auto_capture:` (found at the `auto_capture:                # a MAP since change 0127` lines), with a one-line trailing comment: `# persona-calibrated human-facing prose; off by default`.

- [ ] **Step 7: Run the affected tests**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep -c "^NOT OK"; bash tests/test_config_read_channel.sh 2>&1 | grep -c "^NOT OK"; bash tests/test_docket_config.sh 2>&1 | grep -c "^NOT OK"`
Expected: `0`, `0`, `0`. The read-channel guard matters here: the new README and contract prose must not tell an agent to read the config file — skills read the exports.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-config.md .docket.example.yml tests/test_docket_example_yml.sh README.md
git commit -m "docs(0276): document dummy_mode across the contract, example config, and README"
```

---

### Task 3: The convention's shared definition and its reference

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (a compact new section, placed after *Auto-capture (shared definition)*)
- Create: `skills/docket-convention/references/dummy-mode.md`
- Create: `tests/test_dummy_mode.sh`
- Modify: `tests/runtime-budgets.tsv` (register the new test file)
- Modify: `tests/test_skill_size_budgets.sh` (raise the convention row; add a row for the new reference)

**Interfaces:**
- Consumes: the three export names from Task 1 and the five token names from Task 2.
- Produces: the section heading `## Dummy mode (shared definition)` and the reference path `references/dummy-mode.md`, both of which Task 4's skill pointers cite by name.

- [ ] **Step 1: Write the failing guard**

Create `tests/test_dummy_mode.sh`. Every prose assert binds a phrase to the claim it makes, over a whitespace-collapsed haystack so a pure re-flow cannot redden it.

```bash
#!/usr/bin/env bash
# tests/test_dummy_mode.sh — change 0276. Guards the dummy-mode prose contract: the convention owns
# a shared definition, that definition carries the agent-safety rule, and every eligible skill body
# points at it. Asserts BIND each phrase to the claim it makes (learnings:
# prose-guard-binds-phrase-to-claim) over a whitespace-COLLAPSED haystack (learnings:
# phrase-grep-over-wrapped-prose), so a re-wrap is invisible and a reworded rule is not.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

flat(){ tr '\n' ' ' < "$1" | tr -s '[:space:]' ' '; }

CONV="$REPO/skills/docket-convention/SKILL.md"
REF="$REPO/skills/docket-convention/references/dummy-mode.md"

conv_flat="$(flat "$CONV")"

# Anchor existence first: a window-bound assert that silently matches nothing is worse than none.
assert "convention: the shared-definition heading exists" \
  'grep -qF "## Dummy mode (shared definition)" "$CONV"'
assert "convention: the section points at its reference file" \
  'grep -qE "Dummy mode \(shared definition\).{0,1200}references/dummy-mode\.md" <<<"$conv_flat"'

# The agent-safety rule is the load-bearing sentence: bind "never a decision input" to the block it
# is about, so a rewrite that keeps the words and drops the subject reddens.
assert "convention: the plain-terms block is bound to 'never a decision input'" \
  'grep -qE "In plain terms.{0,200}never a decision input" <<<"$conv_flat"'
assert "convention: the three exports are named" \
  'grep -qF "DUMMY_MODE_ENABLED" "$CONV" && grep -qF "DUMMY_MODE_PERSONA" "$CONV" && grep -qF "DUMMY_MODE_SURFACES" "$CONV"'

assert "reference: the file exists" '[ -f "$REF" ]'
ref_flat="$(flat "$REF")"

# All five tokens, each bound to its own mode — adjacency to the word "replace" somewhere in a
# table is satisfied by a table that lists both modes for everything.
for tok in dialogue reports; do
  assert "reference: $tok is classified replace" \
    'grep -qE "\`'"$tok"'\`.{0,200}replace" <<<"$ref_flat"'
done
for tok in results change-sections pr; do
  assert "reference: $tok is classified additive" \
    'grep -qE "\`'"$tok"'\`.{0,200}additive" <<<"$ref_flat"'
done

assert "reference: additive blocks are authored with their parent, not retro-added" \
  'grep -qE "In plain terms.{0,300}same (commit|moment)" <<<"$ref_flat"'
assert "reference: ad-hoc enablement is session-scoped and writes nothing" \
  'grep -qE "session.{0,300}no (config )?writes?|no writes.{0,300}session" <<<"$ref_flat"'
assert "reference: agent-facing artifacts are named as never eligible" \
  'grep -qE "(plans|spec).{0,300}never" <<<"$ref_flat"'

exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_dummy_mode.sh 2>&1 | grep -c "^NOT OK"`
Expected: a non-zero count, including `convention: the shared-definition heading exists` and `reference: the file exists`.

- [ ] **Step 3: Write the reference file**

Create `skills/docket-convention/references/dummy-mode.md`. Budget: ≤ 95 lines / ≤ 900 words. Required content, in this order:

1. A one-paragraph preamble: what dummy mode is, that it is loaded on demand, and that it changes only what a human reads.
2. The surface-token table, five rows, columns `Token | Covers | Mode`, copied from the spec's *Surface tokens (v1)* table.
3. **Replace** semantics: the prose itself is written calibrated to the persona; no separate technical copy; the underlying decisions and artifacts stay fully technical.
4. **Additive** semantics: the artifact keeps its full technical content and gains an authored `### In plain terms` sub-section, written **at the same moment the parent is authored** so it rides the same commit and respects the frozen-artifact rule — never retro-added to a merged results file. Plain heading, not marker-bounded: authored prose, not a rendered view, so no script owns it.
5. **Ad-hoc session enablement**: effect, persona source (always the resolved config persona — ad-hoc never defines its own), duration (the session; the reverse request disables it the same way), no writes, and the subagent boundary — a dispatched agent is not the session, so the dispatching prose must carry the enablement forward explicitly if the child's output is a covered surface.
6. **Not eligible**, as a list: agent-facing artifacts (plans, the spec *file*, learnings findings, build evidence, script contracts) — with the spec's own resolution that the *walkthrough dialogue* is what gets simplified, never the file; script-generated views (`BOARD.md`, mirror issue bodies, `## Artifacts`/backlink blocks, index READMEs, `## Reclaim log`, `## Publish deferred`); the change body's `## Why` (already the PM-altitude plain layer); ADRs.
7. **Authoring guidance**: gloss every project-internal term on first use; prefer the persona's own frame for trade-offs; never drop a decision or a caveat to make prose simpler — simplification is about vocabulary and framing, never about content.

- [ ] **Step 4: Write the convention section**

In `skills/docket-convention/SKILL.md`, after the *Auto-capture (shared definition)* section, add — kept deliberately short, because the mechanics live in the reference:

```markdown
### Dummy mode (shared definition)

Docket's **human-facing** prose can be calibrated to a reader the repo describes. `dummy_mode`
(a map: `enabled` default `false`, `persona` free text, `surfaces` default `all`; global-able —
resolved as `DUMMY_MODE_ENABLED` / `DUMMY_MODE_PERSONA` / `DUMMY_MODE_SURFACES`) governs it. Five
surfaces are eligible: `dialogue` and `reports` are **replaced** — written calibrated to the
persona; `results`, `change-sections`, and `pr` are **additive** — the technical content is
untouched and an authored `### In plain terms` block is written alongside it, in the same commit.
`DUMMY_MODE_PERSONA` always carries a persona (the shipped default when none is configured), so no
skill special-cases an empty one.

**Agent-safety rule:** an `### In plain terms` block is written for the human and is **never a
decision input** — reconcile, review, planning, and every worker read the technical content only.
Agent-facing artifacts (plans, the spec file, learnings, build evidence) are never simplified;
simplifying them would degrade the build loop itself.

**When `DUMMY_MODE_ENABLED` is `true`, or a human asks for dummy mode in-session, and you are about
to author any of the five surfaces → read [`references/dummy-mode.md`](references/dummy-mode.md)
now (blocking)** — it owns the token table, the replace/additive mechanics, ad-hoc session
enablement, the not-eligible list, and the authoring guidance.
```

- [ ] **Step 5: Raise the size budgets in the same diff**

Measure first, then set the rows:

```bash
wc -l skills/docket-convention/SKILL.md skills/docket-convention/references/dummy-mode.md
wc -w skills/docket-convention/SKILL.md skills/docket-convention/references/dummy-mode.md
```

In `tests/test_skill_size_budgets.sh`, raise `skills/docket-convention/SKILL.md` from `355 6150` to the measured actual with a working margin — lines to the next multiple of 5, words to the next multiple of 50, and if that lands within 25 words of the actual, the multiple after it. Insert a new row for `skills/docket-convention/references/dummy-mode.md` on the same rule, keeping the table's alphabetical order. Add the raise note the table's own rules require, which must name the reference file the prose was considered for and say why the residual cannot live there:

```
# skills/docket-convention/SKILL.md's budget was raised 355/6150 -> <L>/<W> by change 0276, which
# added the Dummy mode shared definition. The mechanics — the five-row token table, replace/additive
# semantics, ad-hoc session enablement, the not-eligible list — were extracted to
# references/dummy-mode.md on arrival, not after the budget failed. The residual is the part that
# must be in context UNPROMPTED: the agent-safety rule (an agent that reads the reference has
# already decided to author a plain block; the rule has to reach the agent that has NOT) and the
# export names, which every skill's Step 0 block surfaces.
```

- [ ] **Step 6: Register the new test file's runtime budget**

Add to `tests/runtime-budgets.tsv`, in the file's existing sort position (tab-separated, no spaces):

```
tests/test_dummy_mode.sh	10	parallel
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_dummy_mode.sh; bash tests/test_skill_size_budgets.sh 2>&1 | grep -c "^NOT OK"; bash tests/test_runtime_budgets.sh 2>&1 | grep -c "^NOT OK"; bash tests/test_convention_extraction.sh 2>&1 | grep -c "^NOT OK"`
Expected: `test_dummy_mode.sh` all `ok` and exit 0; the other three report `0`.

- [ ] **Step 8: Mutation-test the guard**

Back up `skills/docket-convention/SKILL.md`, delete the agent-safety paragraph, re-run `bash tests/test_dummy_mode.sh`, and confirm the `never a decision input` assert reddens while the heading assert stays green (that separation is the point of binding the phrase to its claim). Restore from the backup — not with `git checkout --`, which would discard the whole uncommitted section.

- [ ] **Step 9: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/references/dummy-mode.md \
        tests/test_dummy_mode.sh tests/runtime-budgets.tsv tests/test_skill_size_budgets.sh
git commit -m "feat(0276): add the dummy-mode shared definition and its reference"
```

---

### Task 4: Skill-body pointers

**Files:**
- Modify: `skills/docket-new-change/SKILL.md`, `skills/docket-groom-next/SKILL.md` (`dialogue`)
- Modify: `skills/docket-implement-next/SKILL.md` (`pr`, `reports`, `change-sections`)
- Modify: `skills/docket-finalize-change/SKILL.md` (`dialogue`, `reports`, `change-sections`)
- Modify: `skills/docket-status/SKILL.md` (`reports`)
- Modify: `skills/docket-auto-groom/SKILL.md` (`reports`, `change-sections`)
- Modify: `tests/test_dummy_mode.sh` (append the pointer guards)
- Modify: `tests/test_skill_size_budgets.sh` (raise any row this task pushes over)

**Interfaces:**
- Consumes: Task 3's section heading and export names — each pointer cites the shared definition, never restates it.
- Produces: nothing downstream; this is the last task.

- [ ] **Step 1: Append the pointer guards to `tests/test_dummy_mode.sh`**

Insert before the final `exit $fail`. The loop binds each skill to **its own** surface list, so a pointer copy-pasted into the wrong skill reddens:

```bash
# Each eligible skill body carries ONE pointer naming the surfaces IT owns. The assert binds the
# skill's own tokens to the dummy-mode reference in a bounded window; a bare "the file mentions
# dummy mode" would survive a pointer that names the wrong surfaces.
check_pointer(){ # check_pointer <skill-relpath> <token>...
  local rel="$1"; shift
  local f="$REPO/$rel" body tok
  assert "pointer: $rel exists" '[ -f "$f" ]'
  [ -f "$f" ] || return 0
  body="$(flat "$f")"
  assert "pointer: $rel names the shared definition" \
    'grep -qE "[Dd]ummy mode.{0,300}shared definition" <<<"$body"'
  for tok in "$@"; do
    assert "pointer: $rel binds the $tok surface to dummy mode" \
      'grep -qE "[Dd]ummy mode.{0,400}\`'"$tok"'\`|\`'"$tok"'\`.{0,400}[Dd]ummy mode" <<<"$body"'
  done
}
check_pointer skills/docket-new-change/SKILL.md      dialogue
check_pointer skills/docket-groom-next/SKILL.md      dialogue
check_pointer skills/docket-implement-next/SKILL.md  pr reports change-sections
check_pointer skills/docket-finalize-change/SKILL.md dialogue reports change-sections
check_pointer skills/docket-status/SKILL.md          reports
check_pointer skills/docket-auto-groom/SKILL.md      reports

# The reverse direction: no skill body RESTATES the token table, which is the restatement class
# change 0154 exists to stop. A body that lists four or more tokens has copied the table.
for rel in skills/docket-new-change/SKILL.md skills/docket-groom-next/SKILL.md \
           skills/docket-implement-next/SKILL.md skills/docket-finalize-change/SKILL.md \
           skills/docket-status/SKILL.md skills/docket-auto-groom/SKILL.md; do
  n=0
  for tok in dialogue reports results change-sections pr; do
    grep -qF -- "\`$tok\`" "$REPO/$rel" && n=$((n+1))
  done
  assert "no restatement: $rel names at most 3 surface tokens (got $n)" '[ "$n" -le 3 ]'
done
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_dummy_mode.sh 2>&1 | grep "^NOT OK"`
Expected: six `pointer: … names the shared definition` failures plus the per-token failures. The `no restatement` asserts should already pass — they are a floor, not the thing being driven red.

- [ ] **Step 3: Add the pointer to each dialogue skill**

In `skills/docket-new-change/SKILL.md` and `skills/docket-groom-next/SKILL.md`, in the step that opens the brainstorm conversation:

```markdown
When `DUMMY_MODE_ENABLED` is `true` (Step-0 export) — or the human asks for dummy mode in-session — this skill's `dialogue` surface is written calibrated to `DUMMY_MODE_PERSONA` per the convention's *Dummy mode* shared definition. The spec file itself is never simplified.
```

- [ ] **Step 4: Add the pointer to `docket-implement-next`**

In `skills/docket-implement-next/SKILL.md`, as a bullet in the *Terminal disposition* area where the final report is defined, and referenced from Step 7's PR-body assembly:

```markdown
When `DUMMY_MODE_ENABLED` is `true` (Step-0 export) and the surface is in `DUMMY_MODE_SURFACES`, this run's `reports` are written calibrated to `DUMMY_MODE_PERSONA`, and its `pr` body and any `change-sections` it writes (`## Run halted`) gain an authored `### In plain terms` block alongside their full technical content — per the convention's *Dummy mode* shared definition. The plain block is never a decision input.
```

- [ ] **Step 5: Add the pointer to `docket-finalize-change`, `docket-status`, and `docket-auto-groom`**

Same shape, each naming only its own surfaces:

- `docket-finalize-change`: `dialogue` (its human-present prompts), `reports`, `change-sections` (`## Finalize blocked`).
- `docket-status`: `reports` only.
- `docket-auto-groom`: `reports` and `change-sections` (`## Auto-groom blocked`).

- [ ] **Step 6: Run the guard and the budgets**

Run: `bash tests/test_dummy_mode.sh; bash tests/test_skill_size_budgets.sh 2>&1 | grep "^NOT OK"`
Expected: `test_dummy_mode.sh` all `ok`. For each budget failure, raise that row using the table's rounding rule and add the one-line raise note naming this change; a pointer line is the smallest form the content can take, so there is no extraction alternative to argue.

- [ ] **Step 7: Run the whole suite**

Run: `scripts/run-tests.sh`
Expected: exit `0`, no `NOT OK`. Read any trailing `OVER BUDGET:` block as a finding and act on it. Files most likely to be reached by this task beyond the ones already run: `test_config_read_channel.sh`, `test_role_skill_self_description.sh`, `test_skill_handoff_precedence.sh`, `test_readme_skill_catalog.sh`, `test_comment_anchor_style.sh`.

- [ ] **Step 8: Commit**

```bash
git add skills tests/test_dummy_mode.sh tests/test_skill_size_budgets.sh
git commit -m "feat(0276): point the six eligible skill bodies at the dummy-mode definition"
```

---

## Self-Review

**Spec coverage.** Config shape and validation → Task 1. Default persona, single-owner → Task 1 (constant) + Task 2 (quoted in docs). Exports and their resolution → Task 1, documented Task 2. Layer classification (`any layer`) → Task 2's scope tag and contract row. Surface tokens and replace/additive → Task 3's reference, guarded in Task 3. Ad-hoc session enablement → Task 3's reference. Agent-safety rule → Task 3's convention section, guarded. Not-eligible list → Task 3's reference. Skill-body pointers → Task 4. Persona gallery → Task 2. Testing section → Tasks 1, 3, 4. The spec's open question (literal vs expanded `all`) → settled in Global Constraints as **literal**, stated in the contract row.

**Placeholder scan.** Every code step carries the actual text. Two steps are deliberately parametric rather than literal: the budget numbers in Task 3 Step 5 and Task 4 Step 6, which are measured from the file the task just wrote — the plan states the rounding rule and the required justification instead of guessing a number, which is what the budget table's own rules demand.

**Type consistency.** `DUMMY_MODE_ENABLED` / `DUMMY_MODE_PERSONA` / `DUMMY_MODE_SURFACES` and the helper `dm_key` are spelled identically in Tasks 1, 2, 3, and 4. The five tokens are spelled `dialogue`, `reports`, `results`, `change-sections`, `pr` everywhere — `change-sections` is hyphenated in the config, the reference table, and the guards alike.
