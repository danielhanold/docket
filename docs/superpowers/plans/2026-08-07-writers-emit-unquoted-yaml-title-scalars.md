<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0235 — Writers emit unquoted YAML title scalars, so six change files fail to parse](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0235-writers-emit-unquoted-yaml-title-scalars-so-six-change-files.md)**
<!-- docket:backlink:end -->

# Writer-Guaranteed YAML Title Scalars Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `mint-stub.sh` emit a structurally valid YAML `title:` scalar on every mint, teach the reader the exact inverse, close the checker's trailing-colon gap, and repair the five change files already on disk with a broken title.

**Architecture:** Three separable pieces, in dependency order. (1) The **reader** learns single-quote undoubling (`''` → `'`), so a quoted title round-trips byte-for-byte. (2) The **writer** always single-quotes `title` at its one call site — validity becomes structural, not enumerated (ADR-0071), so a byte assert on the emitted line *is* an assert on validity. (3) The **checker** keeps a five-leg predicate — moved into the shared library and extended with the trailing-colon leg that let change 0173 through — because it judges arbitrary hand-authored scalars it did not write. Writer and checker are deliberately **two rules with different jobs**: guarantee vs. detect.

**Tech Stack:** Pure bash 4.4+, POSIX `case` pattern matching, `awk` for the frontmatter write, `git`. No YAML library is introduced anywhere, on any path, including the tests.

## Global Constraints

- **No YAML parser dependency, and no optional-parser test leg.** Python ships no stdlib YAML module; a skip-when-absent assert is silently vacuous, which is the exact failure class this change exists to close. Do not add one "as a bonus" (ADR-0071, spec A5).
- **`set_field`'s byte-for-byte ENVIRON write is not traded away.** The value still reaches `awk` through `MINT_SF_VAL`; all quoting/escaping happens in **bash**, before the export. Never through `awk`'s `gsub`, whose replacement string reinterprets `&`.
- **The feature branch never modifies docket metadata.** Change files, `BOARD.md`, ADRs and `docs/changes/learnings/` live on the `docket` branch and are edited only in the metadata worktree at `/Users/homer/dev/docket/.docket`. Task 6 is the only task that writes there, and it makes **no** feature-branch commit.
- **Shell portability.** `/usr/bin/grep` (BSD) is the portability oracle, not PATH `grep` (which is ugrep here). Prefer `case` over regex for the predicate legs — every leg below is a `case` pattern by design.
- Repo root for the feature branch: `/Users/homer/dev/docket/.worktrees/writers-emit-unquoted-yaml-title-scalars-so-six-change-files`. All paths below are relative to it unless stated otherwise.
- Full suite: `scripts/run-tests.sh`. Single file: `bash tests/test_<name>.sh`.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/lib/docket-frontmatter.sh` | Modify: `_docket_unwrap_quotes` gains the undouble leg (T1); gains `docket_yaml_single_quote` (T2) and `docket_scalar_quote_reason` / `docket_scalar_needs_quoting` (T3). |
| `scripts/mint-stub.sh` | Modify: the `title` `set_field` call site quotes unconditionally (T2). |
| `scripts/board-checks.sh` | Modify: `scalar_form_check` delegates its syntax legs to the predicate and gains three messages (T4). |
| `scripts/mint-stub.md` | Modify: document the always-quote write shape (T2). |
| `scripts/board-checks.md` | Modify: document the five legs incl. trailing-colon (T4). |
| `AGENTS.md` | Modify: widen the `yaml-scalar` rule from "hand-authored" to any writer (T5). |
| `tests/test_docket_frontmatter.sh` | Modify: reader-inverse + predicate unit coverage (T1, T3). |
| `tests/test_mint_stub.sh` | Modify: writer round-trip + byte asserts over the adversarial input table (T2). |
| `tests/test_board_checks.sh` | Modify: trailing-colon RED fixture + the new legs' fixtures (T4). |
| *(metadata worktree)* five change files, `docs/changes/learnings/yaml-scalar.md` | Modify in `/Users/homer/dev/docket/.docket` only (T6). |

---

### Task 1: Reader — undouble `''` inside a single-quoted token

The writer's escape must have an exact inverse **before** the writer starts producing it, or every apostrophe-bearing title reads back as `manifest''s` in `BOARD.md` and in `mint-stub`'s `dup_of` slug comparison. This task lands first for that reason.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:44-53` (`_docket_unwrap_quotes`)
- Test: `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `_docket_unwrap_quotes VALUE` — unchanged signature (one arg, logical scalar on stdout, **no** trailing newline). New behavior: when the stripped pair was single quotes, every `''` in the interior collapses to `'`. Double-quoted tokens stay byte-untouched. `field()` and `fm_field()` inherit this through their existing delegation; `field_raw()`/`fm_field_raw()` are unaffected.

- [ ] **Step 1: Write the failing tests**

Add this block to `tests/test_docket_frontmatter.sh` immediately **before** the final `if [ "$fail" = 0 ]; then echo "PASS"; ...` line:

```bash
# --- _docket_unwrap_quotes: the single-quote escape inverse (change 0235, ADR-0071) -------------
# Single-quoted YAML interprets no escapes and has exactly ONE rule: an embedded ' is written ''.
# The reader must be the exact inverse of docket_yaml_single_quote's doubling, or every
# apostrophe-bearing title renders as manifest''s in BOARD.md and mis-compares in dup_of.
uq="$(mktemp -d)"
printf -- "---\ntitle: 'The manifest''s elsewhere: check'\n---\n" > "$uq/sq-apos.md"
printf -- "---\ntitle: 'plain single quoted'\n---\n"              > "$uq/sq-plain.md"
printf -- '---\ntitle: "a b'"''"'c"\n---\n'                       > "$uq/dq-doubled.md"
printf -- "---\ntitle: it''s bare\n---\n"                         > "$uq/bare-doubled.md"
printf -- "---\ntitle: ''\n---\n"                                 > "$uq/sq-empty.md"
printf -- "---\ntitle: '''''''''\n---\n"                          > "$uq/sq-only-quotes.md"

assert "unwrap undoubles '' inside a single-quoted token" \
  '[ "$(field "$uq/sq-apos.md" title)" = "The manifest'"'"'s elsewhere: check" ]'
assert "unwrap leaves a single-quoted token with no doubling unchanged" \
  '[ "$(field "$uq/sq-plain.md" title)" = "plain single quoted" ]'
# The double-quoted arm adds NO escape interpretation: a literal '' inside "..." stays two bytes.
assert "unwrap does NOT undouble '' inside a DOUBLE-quoted token" \
  '[ "$(field "$uq/dq-doubled.md" title)" = "a b'"''"'c" ]'
# Undoubling is scoped to a token that was actually quoted — a BARE value is never rewritten.
assert "unwrap does NOT undouble '' in a BARE (unquoted) value" \
  '[ "$(field "$uq/bare-doubled.md" title)" = "it'"''"'s bare" ]'
assert "unwrap maps the empty single-quoted scalar '"'"''"'"' to the empty string" \
  '[ -z "$(field "$uq/sq-empty.md" title)" ]'
# Interior of '''''''' (8 quotes stripped to 6) is three doubled pairs -> three apostrophes.
assert "unwrap collapses every doubled pair, not just the first" \
  '[ "$(field "$uq/sq-only-quotes.md" title)" = "'"'''"'" ]'
assert "field_raw still preserves the doubling (the RAW token is not decoded)" \
  '[ "$(field_raw "$uq/sq-apos.md" title)" = "'"'"'The manifest'"''"'s elsewhere: check'"'"'" ]'
rm -rf "$uq"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: FAIL — the undoubling asserts report `NOT OK`, e.g. `NOT OK - unwrap undoubles '' inside a single-quoted token` (the reader returns `The manifest''s elsewhere: check`). The double-quoted, bare, and `field_raw` asserts already pass; that is the point — they pin the arms this change must *not* move.

- [ ] **Step 3: Write the minimal implementation**

Replace `_docket_unwrap_quotes` in `scripts/lib/docket-frontmatter.sh` with:

```bash
_docket_unwrap_quotes(){
  local v="$1" q
  if [ "${#v}" -ge 2 ]; then
    q="${v:0:1}"
    if { [ "$q" = '"' ] || [ "$q" = "'" ]; } && [ "${v: -1}" = "$q" ]; then
      v="${v:1:${#v}-2}"
      # Single-quoted YAML interprets NO escapes and has exactly one rule: an embedded ' is
      # written ''. Undouble it here — the exact inverse of docket_yaml_single_quote, which
      # mint-stub now applies to every title (change 0235, ADR-0071). Without this leg an
      # apostrophe-bearing title reads back as manifest''s in BOARD.md and mis-compares in dup_of.
      # Double-quoted tokens are deliberately UNTOUCHED: no escape interpretation is added there
      # (change 0138's stance), and the two double-quoted titles in the tree carry no escapes.
      if [ "$q" = "'" ]; then v="${v//\'\'/\'}"; fi
    fi
  fi
  printf '%s' "$v"
}
```

Also extend the function's header comment (the block at `scripts/lib/docket-frontmatter.sh:38-43`) so it no longer claims the reader does no unescaping at all: state that single-quoted `''` undoubling is the one escape it inverts, and that double-quoted interiors remain raw.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: PASS

- [ ] **Step 5: Mutation-test the new leg**

Comment out the `if [ "$q" = "'" ]; then ... fi` line, re-run `bash tests/test_docket_frontmatter.sh`, and confirm it goes **FAIL** with the undoubling asserts red and the double-quoted/bare/`field_raw` asserts still green. Confirm the edit landed with a count, never by eye: `grep -c "v//\\\\'\\\\'" scripts/lib/docket-frontmatter.sh` must read `1` before and `0` after. Restore the line and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "fix(0235): teach the reader the single-quote escape inverse"
```

---

### Task 2: Writer — `mint-stub` always single-quotes `title`

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (add `docket_yaml_single_quote` beside the accessors)
- Modify: `scripts/mint-stub.sh:204` (the `title` `set_field` call site)
- Modify: `scripts/mint-stub.md` (step 6 of the Behavior section, around line 64)
- Test: `tests/test_mint_stub.sh`

**Interfaces:**
- Consumes: `_docket_unwrap_quotes`'s undoubling leg from Task 1 (the round-trip asserts below fail without it).
- Produces: `docket_yaml_single_quote VALUE` — prints a **single-quoted YAML scalar** for VALUE on stdout with **no** trailing newline: a leading `'`, VALUE with every `'` doubled, a trailing `'`. Total function, no failure mode, empty VALUE yields the two-byte `''`.

`set_field` itself is **not** modified: it stays the key-agnostic byte-for-byte writer, and the quoting is applied by its caller. This keeps the six non-prose call sites (`id`, `slug`, `created`, `updated`, `type`, `discovered_from`) byte-identical — critically, `discovered_from: [234]` stays an unquoted YAML **sequence** rather than becoming a string.

- [ ] **Step 1: Write the failing tests**

Add this block to `tests/test_mint_stub.sh`, immediately **before** its final `if [ "$fail" = 0 ]` line:

```bash
# --- (Y) title is ALWAYS single-quoted, and round-trips byte-for-byte (change 0235, ADR-0071) ---
# Validity is structural, not enumerated: single-quoted YAML interprets no escapes and has exactly
# one rule (embedded ' doubles), so pinning the emitted BYTES is pinning validity — which is what
# lets this suite prove well-formedness with no YAML parser anywhere (spec A5).
# Every row is an adversarial shape from the spec's table; the apostrophe rows are MANDATORY —
# they are the only ones that fail if the writer's doubling and the reader's undoubling are not
# exact inverses.
sq(){ printf "'%s'" "${1//\'/\'\'}"; }   # the expected shape, computed independently of the lib

y_case(){ # y_case SLUG TITLE — mint TITLE, assert the raw line's exact bytes and the logical value
  local yslug="$1" ytitle="$2" W B NEW want
  W="$(new_repo)"
  B="$(body 'discovered while building #235')"
  run_mint "$W" --title "$ytitle" --slug "$yslug" --body-file "$B" --discovered-from 235 >/dev/null 2>&1
  NEW="$W/docs/changes/active/0001-$yslug.md"
  want="$(sq "$ytitle")"
  assert "Y[$yslug]: raw title line is exactly the single-quoted form" \
    "[ \"\$(field_raw '$NEW' title)\" = \"\$(printf %s \"\$want\")\" ]"
  assert "Y[$yslug]: field() returns the original title byte-for-byte" \
    "[ \"\$(field '$NEW' title)\" = \"\$ytitle\" ]"
}

y_case apostrophe      "The manifest's elsewhere check"
y_case apos-and-colon  "Clear change 0202's three findings: dead guard, stale comment"
y_case colon-space     "Split gate-execution.md: probe evidence"
y_case trailing-colon  "a model ID containing / or :"
y_case leading-bracket "[WIP] rework the runner"
y_case bare-boolean    "off"
y_case hash-comment    "clear finding #3 from review"
y_case trailing-space  "a title with a trailing space "
y_case dquote-start    '"quoted" start of a title'
y_case backslash       'a path C:\temp and a & ampersand'

# The non-prose fields must be UNTOUCHED — this is the guard on quote-only-the-title.
WY="$(new_repo)"; BY="$(body 'seed')"
run_mint "$WY" --title "Ordinary title" --slug plainfields --body-file "$BY" --discovered-from 234 >/dev/null 2>&1
NY="$WY/docs/changes/active/0001-plainfields.md"
assert "Y: discovered_from is still an UNQUOTED list (never stringified)" \
  '[ "$(field_raw "$NY" discovered_from)" = "[234]" ]'
assert "Y: list_field still parses discovered_from" '[ "$(list_field "$NY" discovered_from)" = "234" ]'
assert "Y: id is still unquoted"      '[ "$(field_raw "$NY" id)" = "1" ]'
assert "Y: slug is still unquoted"    '[ "$(field_raw "$NY" slug)" = "plainfields" ]'
assert "Y: type is still unquoted"    '[ "$(field_raw "$NY" type)" = "chore" ]'
assert "Y: created is still unquoted" '[ "$(field_raw "$NY" created)" = "$FIXED_DAY" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_mint_stub.sh`
Expected: FAIL — every `Y[...]: raw title line is exactly the single-quoted form` assert is red (the writer emits the bare value). The `field()` round-trip asserts pass for rows with no apostrophe and fail for the two apostrophe rows only once quoting lands, so treat the raw-line asserts as the primary signal here. The four "still unquoted" asserts pass already — they are the regression floor.

- [ ] **Step 3: Write the minimal implementation**

Add to `scripts/lib/docket-frontmatter.sh`, directly after `_docket_unwrap_quotes`:

```bash
# docket_yaml_single_quote VALUE -> a single-quoted YAML scalar on stdout, NO trailing newline.
# Single-quoted YAML interprets no escapes and has exactly one rule: an embedded ' is written ''.
# That makes the output well-formed for EVERY input that carries no control character — so a caller
# quoting unconditionally needs no dangerous-input enumeration, and therefore has no leg to omit
# (ADR-0071). The doubling happens here, in bash, so the value never meets awk's gsub replacement
# syntax (where a literal & would be reinterpreted). _docket_unwrap_quotes is its exact inverse.
docket_yaml_single_quote(){
  printf "'%s'" "${1//\'/\'\'}"
}
```

Then change the `title` call site in `scripts/mint-stub.sh` (line 204) from:

```bash
  set_field "$tmp" title "$TITLE"             || die "set_field title failed for stub $id"
```

to:

```bash
  # title is the ONE free-text prose value here (slug is slugified; id/created/updated/type are
  # generated and discovered_from is a [..] list), and it is model-authored English that routinely
  # carries a colon-space, an apostrophe, or a leading indicator. Quote it UNCONDITIONALLY rather
  # than predicating on shape: a conditional is only as good as its enumeration, while the
  # single-quoted form is well-formed for every input that clears the control-character gate above
  # (ADR-0071). Deliberately NOT applied to the other six calls — quoting discovered_from would
  # turn a YAML sequence into a string.
  set_field "$tmp" title "$(docket_yaml_single_quote "$TITLE")" \
                                              || die "set_field title failed for stub $id"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_mint_stub.sh`
Expected: PASS

- [ ] **Step 5: Mutation-test both halves**

Two separate mutations, each restored before the next:

1. Revert the call site to `set_field "$tmp" title "$TITLE"`. Re-run `bash tests/test_mint_stub.sh` → **FAIL** on the raw-line asserts. Confirm the mutation landed: `grep -c docket_yaml_single_quote scripts/mint-stub.sh` reads `1` before, `0` after. Restore.
2. With quoting in place, comment out the undoubling leg added in Task 1. Re-run `bash tests/test_mint_stub.sh` → **FAIL** on the two apostrophe rows' `field()` round-trip asserts (`Y[apostrophe]`, `Y[apos-and-colon]`). This is the assert that proves writer and reader are inverses rather than merely both present. Restore and re-run to green.

- [ ] **Step 6: Update the contract doc**

In `scripts/mint-stub.md`, in Behavior step 6 (the paragraph beginning "Every frontmatter value — most notably `--title`…", around line 64), record that `title` is now emitted as a **single-quoted** YAML scalar on every mint with embedded `'` doubled; that the byte-for-byte ENVIRON write is unchanged (the doubling happens in bash, before the export, so `|`, `&` and `\` are still never reinterpreted); that the other six field writes are untouched, naming `discovered_from: [234]` staying an unquoted sequence as the reason; and that `field()`/`fm_field()` invert the doubling on read, so consumers see the original title. Cite ADR-0071.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/mint-stub.sh scripts/mint-stub.md tests/test_mint_stub.sh
git commit -m "fix(0235): mint-stub always single-quotes the title scalar"
```

---

### Task 3: Checker predicate — five legs in the shared library

The writer no longer needs a predicate (Task 2 quotes unconditionally). The **checker** still does: `board-checks.sh` judges arbitrary hand-authored scalars it did not write, so it must detect rather than guarantee. This task adds the predicate and its unit coverage; Task 4 wires it in.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (add the predicate after `docket_yaml_single_quote`)
- Test: `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces:
  - `docket_scalar_quote_reason VALUE` — prints exactly **one** leg token on stdout and returns 0, or prints nothing when VALUE is safe bare. Tokens, in evaluation order: `colon-space`, `trailing-colon`, `bare-boolean`, `comment-introducer`, `indicator`.
  - `docket_scalar_needs_quoting VALUE` — exit 0 iff `docket_scalar_quote_reason` printed a token. Boolean convenience wrapper; no output.

  Both take the **logical value**, never a raw token. They deliberately do **not** carry an already-quoted skip leg: that leg belongs where the raw token lives (Task 4), because a value that logically *starts* with a quote character must be quoted, not skipped.

- [ ] **Step 1: Write the failing tests**

Add to `tests/test_docket_frontmatter.sh`, before the final `if [ "$fail" = 0 ]` line:

```bash
# --- docket_scalar_quote_reason: the CHECKER's needs-quoting predicate (change 0235) ------------
# Scoped to the logical value and to SYNTAX only. The writer does not consume this (it quotes
# unconditionally, ADR-0071); board-checks does, because it judges scalars it did not write.
reason(){ docket_scalar_quote_reason "$1"; }

assert "leg colon-space"        '[ "$(reason "a: b")" = colon-space ]'
assert "leg trailing-colon"     '[ "$(reason "ends in a colon:")" = trailing-colon ]'
assert "leg bare-boolean yes"   '[ "$(reason "yes")" = bare-boolean ]'
assert "leg bare-boolean TRUE"  '[ "$(reason "TRUE")" = bare-boolean ]'
assert "leg bare-boolean off"   '[ "$(reason "off")" = bare-boolean ]'
assert "leg comment-introducer" '[ "$(reason "clear finding #3")" = comment-introducer ]'
assert "leg indicator bracket"  '[ "$(reason "[WIP] rework")" = indicator ]'
assert "leg indicator anchor"   '[ "$(reason "&anchor thing")" = indicator ]'
assert "leg indicator star"     '[ "$(reason "*star* emphasis")" = indicator ]'
assert "leg indicator at"       '[ "$(reason "@mention someone")" = indicator ]'
assert "leg indicator quote"    '[ "$(reason "\"quoted\" start")" = indicator ]'
assert "leg indicator dash-sp"  '[ "$(reason "- a list-looking title")" = indicator ]'

# Near-misses: each is well-formed bare YAML and must stay SILENT.
assert "silent: a colon with no following space" '[ -z "$(reason "a:b ratio")" ]'
assert "silent: offset is not off"               '[ -z "$(reason "offset")" ]'
assert "silent: nobody is not no"                '[ -z "$(reason "nobody")" ]'
assert "silent: a hash not preceded by space"    '[ -z "$(reason "issue#3 reopened")" ]'
assert "silent: an interior dash"                '[ -z "$(reason "a well-formed title")" ]'
assert "silent: an interior asterisk"            '[ -z "$(reason "a b*c title")" ]'
assert "silent: an ordinary prose title"         '[ -z "$(reason "Cap the widget")" ]'
assert "silent: the EMPTY value is exempt"       '[ -z "$(reason "")" ]'

# Flow-collection exemption — a SHAPE test, never a key enumeration. Quoting a well-formed [..]
# or {..} would turn a YAML sequence/map into a string for any real parser.
assert "silent: a well-formed flow sequence" '[ -z "$(reason "[234]")" ]'
assert "silent: a flow sequence with items"  '[ -z "$(reason "[4, 6]")" ]'
assert "silent: a well-formed flow map"      '[ -z "$(reason "{owner: x}")" ]'
# ...but an UNCLOSED bracket is not a collection, it is a broken scalar, and must still fire.
assert "leg indicator: an UNCLOSED bracket still fires" '[ "$(reason "[WIP")" = indicator ]'

# The boolean wrapper agrees with the reason function in both directions.
assert "needs_quoting true for a colon-space value" 'docket_scalar_needs_quoting "a: b"'
assert "needs_quoting false for a clean value"      '! docket_scalar_needs_quoting "Cap the widget"'
assert "needs_quoting false for the empty value"    '! docket_scalar_needs_quoting ""'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: FAIL with `docket_scalar_quote_reason: command not found` on stderr and every new assert red.

- [ ] **Step 3: Write the minimal implementation**

Add to `scripts/lib/docket-frontmatter.sh` after `docket_yaml_single_quote`:

```bash
# docket_scalar_quote_reason VALUE -> ONE leg token when emitting VALUE as a BARE YAML scalar would
# not be well-formed; empty when it is safe bare. Tokens: colon-space | trailing-colon |
# bare-boolean | comment-introducer | indicator.
#
# Consumer: board-checks.sh's scalar_form_check, which judges HAND-AUTHORED scalars it did not
# write and so must detect. The WRITER does not consume it — mint-stub quotes unconditionally, so
# it has no enumeration to get wrong (ADR-0071). Two rules with different jobs, deliberately.
#
# Takes the LOGICAL value. The already-quoted skip leg lives in scalar_form_check, which is the only
# site holding a raw token: applying it here would be unsound, since a value that logically STARTS
# with a quote character must be quoted, not skipped.
#
# All legs are `case` patterns, never regex — /usr/bin/grep (BSD) and PATH grep (ugrep here) do not
# agree on bounded repetition, and a shape test has no business depending on which one is found.
docket_scalar_quote_reason(){
  local v="$1"
  [ -n "$v" ] || return 0          # empty is exempt EXPLICITLY: archive-change.sh writes claimed_at ""
  # Flow-collection exemption: a well-formed [..] or {..} is a SEQUENCE/MAP, and quoting it would
  # silently change its parsed type (discovered_from: [234] is exactly this shape). A shape test —
  # it never asks which key is being written.
  case "$v" in
    '['*']'|'{'*'}') return 0 ;;
  esac
  case "$v" in *': '*) printf 'colon-space';        return 0 ;; esac
  case "$v" in *':')   printf 'trailing-colon';     return 0 ;; esac
  case "$v" in
    [Oo][Nn]|[Oo][Ff][Ff]|[Yy][Ee][Ss]|[Nn][Oo]|[Tt][Rr][Uu][Ee]|[Ff][Aa][Ll][Ss][Ee])
                       printf 'bare-boolean';       return 0 ;;
  esac
  # ' #' opens a YAML comment: it TRUNCATES the value silently rather than aborting the parse,
  # which is the quieter and therefore worse failure. `finding #3` is ordinary auto-capture prose.
  case "$v" in *' #'*) printf 'comment-introducer'; return 0 ;; esac
  # A leading YAML indicator: & and * silently lose meaning, the rest abort the parse.
  case "$v" in
    '['*|']'*|'{'*|'}'*|','*|'&'*|'*'*|'!'*|'|'*|'>'*|"'"*|'"'*|'%'*|'@'*|'`'*|'?'*|':'*|'- '*)
                       printf 'indicator';          return 0 ;;
  esac
  return 0
}
# docket_scalar_needs_quoting VALUE — exit 0 iff the value would not be well-formed bare.
docket_scalar_needs_quoting(){ [ -n "$(docket_scalar_quote_reason "$1")" ]; }
```

Add both names to the `Provides:` header block at the top of the file (around lines 15–30), one line each, matching the existing style.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: PASS

- [ ] **Step 5: Mutation-test the trailing-colon leg specifically**

This is the leg that let change 0173 through undetected, so prove it detects rather than merely exists. Delete the `case "$v" in *':') ... esac` line, re-run `bash tests/test_docket_frontmatter.sh` → **FAIL** on `leg trailing-colon`. Confirm with `grep -c "trailing-colon" scripts/lib/docket-frontmatter.sh` (2 before — the pattern and nothing else in this file — 1 after). Restore, re-run to green.

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "feat(0235): add the shared needs-quoting predicate for the checker"
```

---

### Task 4: Checker — `scalar_form_check` delegates, and gains three legs

**Files:**
- Modify: `scripts/board-checks.sh:336-348` (`scalar_form_check`)
- Modify: `scripts/board-checks.md` (the `scalar-form` section, around lines 171-185)
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `docket_scalar_quote_reason VALUE` from Task 3. `board-checks.sh` already sources `lib/docket-frontmatter.sh` (line 73) — no new source line.
- Produces: no new function. The `scalar-form` **check id is unchanged** — it gains legs, not a sibling check — so the `docket-status` check-id vocabulary and its consumers are untouched.

The two existing messages must stay **byte-identical**; `tests/test_board_checks.sh` asserts their exact wording and this task must not move them.

- [ ] **Step 1: Write the failing tests**

Add to `tests/test_board_checks.sh`, immediately after the existing bare-boolean RED fixtures (after the `s85line` assert block, around line 700):

```bash
# Trailing colon — the leg whose absence let change 0173 sit unreported (change 0235).
read -r S86 _ < <(new_repo)
mk_sf "$S86" 86 trailing-colon-title 'title: a model ID containing / or :'
s86out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S86/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title ENDING in a colon (id 86, trailing-colon leg)" \
  'has_finding "$s86out" scalar-form 86'
s86line="$(grep -E "$(printf "^scalar-form\t86\t")" <<<"$s86out")"
assert "the trailing-colon finding names the title field and the shape (id 86)" \
  'grep -qF -- "title: unquoted scalar ends with" <<<"$s86line"'

# ' #' opens a YAML comment: it TRUNCATES silently rather than aborting, so it is the quieter defect.
read -r S84 _ < <(new_repo)
mk_sf "$S84" 84 hash-title 'title: clear finding #3 from review'
s84out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S84/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title containing ' #' (id 84, comment-introducer leg)" \
  'has_finding "$s84out" scalar-form 84'

# A leading YAML indicator character.
read -r S83 _ < <(new_repo)
mk_sf "$S83" 83 indicator-title 'title: [WIP] rework the runner'
s83out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S83/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title opening with a YAML indicator (id 83, indicator leg)" \
  'has_finding "$s83out" scalar-form 83'

# --- GREEN near-misses for the new legs: each is well-formed bare YAML and must stay SILENT ---
read -r S82 _ < <(new_repo)
mk_sf "$S82" 82 hash-nospace-title 'title: issue#3 reopened by the reporter'
s82out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S82/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a '#' not preceded by whitespace (id 82)" \
  '! has_finding "$s82out" scalar-form 82'

read -r S81 _ < <(new_repo)
mk_sf "$S81" 81 interior-dash-title 'title: a well-formed title with an interior dash'
s81out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S81/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for an interior dash (id 81)" \
  '! has_finding "$s81out" scalar-form 81'

read -r S80 _ < <(new_repo)
mk_sf "$S80" 80 colon-nospace-title 'title: the a:b ratio holds'
s80out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S80/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a colon with no following space (id 80)" \
  '! has_finding "$s80out" scalar-form 80'

# The SINGLE-quoted shape mint-stub now always writes must be accepted by the checker it will meet.
read -r S79 _ < <(new_repo)
mk_sf "$S79" 79 minted-shape-title "title: 'The manifest''s elsewhere: check proves a word occurrence'"
s79out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S79/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for the exact shape mint-stub now writes (id 79, skip leg)" \
  '! has_finding "$s79out" scalar-form 79'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_checks.sh`
Expected: FAIL — the three new RED asserts (ids 86, 84, 83) are red; the four GREEN near-miss asserts already pass. Those four are the regression floor for this task: they must never go red.

- [ ] **Step 3: Write the minimal implementation**

Replace `scalar_form_check` in `scripts/board-checks.sh` with:

```bash
  scalar_form_check(){ # scalar_form_check FIELD RAW
    local sfc_field="$1" sfc_raw="$2" sfc_reason
    case "$sfc_raw" in
      ''|\"*|\'*) return 0 ;;   # skip leg: empty, or opens with a quote -> well-formed, never inspected
    esac
    # The syntax legs live in ONE place — lib/docket-frontmatter.sh's docket_scalar_quote_reason —
    # so the checker and any future consumer cannot drift into two copies of the same rule
    # (change 0235). The skip leg above stays here: it is the only leg that needs the RAW token,
    # and pushing it into the predicate would wrongly skip a value that logically STARTS with a
    # quote character. The messages stay here too, because a finding is this script's output shape.
    sfc_reason="$(docket_scalar_quote_reason "$sfc_raw")"
    case "$sfc_reason" in
      colon-space)
        emit scalar-form "$cid" "$sfc_field: unquoted scalar contains ': ' — quote it or reword (well-formed YAML)" ;;
      trailing-colon)
        emit scalar-form "$cid" "$sfc_field: unquoted scalar ends with ':' — quote it or reword (well-formed YAML)" ;;
      bare-boolean)
        emit scalar-form "$cid" "$sfc_field: unquoted bare YAML boolean ($sfc_raw) — quote it or reword (well-formed YAML)" ;;
      comment-introducer)
        emit scalar-form "$cid" "$sfc_field: unquoted scalar contains ' #', a YAML comment introducer that silently truncates it — quote it or reword (well-formed YAML)" ;;
      indicator)
        emit scalar-form "$cid" "$sfc_field: unquoted scalar opens with a YAML indicator character — quote it or reword (well-formed YAML)" ;;
    esac
  }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_board_checks.sh`
Expected: PASS — including the pre-existing colon-space and bare-boolean asserts, whose message wording is unchanged.

- [ ] **Step 5: Prove the delegation is real, not additive**

Delete the `*': '*` line from `docket_scalar_quote_reason` in `scripts/lib/docket-frontmatter.sh`, then run `bash tests/test_board_checks.sh` → it must go **FAIL** on the *existing* id-90 colon-space assert. That is the evidence that `scalar_form_check` genuinely reads the shared predicate rather than keeping a private copy alongside it. Confirm the mutation landed with `grep -c "colon-space" scripts/lib/docket-frontmatter.sh` before and after. Restore, re-run both `bash tests/test_board_checks.sh` and `bash tests/test_docket_frontmatter.sh` to green.

- [ ] **Step 6: Update the contract doc**

In `scripts/board-checks.md`'s `scalar-form` section, change "applies three legs, in order" to name **five**, list them in the predicate's evaluation order with one line each (colon-space, trailing-colon, bare-boolean, comment-introducer, indicator), note that the trailing-colon leg closes a real miss (an archived change whose title ended in `/ or :` went unreported), and state that the syntax legs now live in `lib/docket-frontmatter.sh`'s `docket_scalar_quote_reason` while the skip leg and the messages stay in the script. Say explicitly that the check **id** is unchanged.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "fix(0235): close the scalar-form trailing-colon gap via the shared predicate"
```

---

### Task 5: Widen the house rule from "hand-authored" to any writer

**Files:**
- Modify: `AGENTS.md:28-30`

**Interfaces:**
- Consumes: nothing. Produces: nothing consumed by later tasks.

Note (verified during reconcile): **no test greps this bullet's wording**, so this edit reddens nothing. `docs/changes/learnings/yaml-scalar.md` carries the same sentence but lives on the `docket` branch and is handled in Task 6.

- [ ] **Step 1: Make the edit**

Replace the bullet at `AGENTS.md:28-30`:

```markdown
- Quote any hand-authored YAML scalar carrying a colon-space or a boolean keyword
  (`on/off/yes/no/true/false`). Today's grep/awk reader tolerating it is not evidence it is
  well-formed.
```

with:

```markdown
- Quote any YAML scalar carrying a colon-space, a trailing colon, a ` #`, a leading indicator
  character, or a boolean keyword (`on/off/yes/no/true/false`) — whoever writes it, model or
  script. Today's grep/awk reader tolerating it is not evidence it is well-formed. A **script**
  writing free-text prose into frontmatter quotes unconditionally at the write boundary rather
  than predicating on shape, because a conditional is only as good as its enumeration
  (ADR-0071; `mint-stub.sh`'s `title` write is the reference).
```

- [ ] **Step 2: Verify nothing depended on the old wording**

Run: `grep -rn "hand-authored YAML scalar" tests/ scripts/ skills/ docs/ AGENTS.md`
Expected: no output (the only occurrence was the line just replaced). If a hit appears, repoint that dependent at the new wording rather than restoring the old sentence.

- [ ] **Step 3: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: every test file passes.

- [ ] **Step 4: Commit**

```bash
git add AGENTS.md
git commit -m "docs(0235): widen the yaml-scalar rule to any writer"
```

---

### Task 6: Repair the five broken files on the metadata branch, and republish three

**This task makes no feature-branch commit.** Every path below is on the `docket` branch, in the metadata worktree at `/Users/homer/dev/docket/.docket`. Do not stage any of it on `feat/…`.

**Files (all under `/Users/homer/dev/docket/.docket/`):**
- Modify: `docs/changes/active/0121-the-manifest-s-elsewhere-check-proves-a-word-occurrence-not.md`
- Modify: `docs/changes/active/0234-split-gate-execution-md-probe-evidence-should-not-sit-on-a-b.md`
- Modify: `docs/changes/archive/2026-07-31-0173-field-of-silently-truncates-a-model-id-containing-or.md`
- Modify: `docs/changes/archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md`
- Modify: `docs/changes/archive/2026-08-05-0217-clear-change-0202-s-three-minor-findings-dead-guard-stale-ba.md`
- Modify: `docs/changes/learnings/yaml-scalar.md`

**Interfaces:** consumes nothing; produces nothing the suite reads.

**Quoting form — read this before editing.** The repair must be correct under the reader that is **actually running today**, which is the primary tree's `scripts/` on `origin/main` — Task 1's undoubling leg does not reach it until this PR merges. So:

- Titles with **no apostrophe** (0173, 0211, 0234) → **single**-quoted. Nothing to double; both readers agree.
- Titles **with** an apostrophe (0121, 0217) → **double**-quoted. Neither contains a `"` or a `\`, so no escaping is needed at all, the value is valid YAML, `scalar_form_check`'s skip leg accepts either quote style, and both the old and new readers return the identical logical value. Single-quoting these two would render them as `manifest''s` in `BOARD.md` for as long as the PR sits unmerged.

This is a repair-side choice about existing bytes and does not weaken the writer's unconditional single-quote rule.

**No word of any title changes.** An archived file is immutable as a *record*; a syntactically broken one is not a record anyone can read, and nothing in the convention makes an archived change file immutable (that rule covers `Accepted` ADRs).

- [ ] **Step 1: Re-sync the metadata worktree and re-read each target**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight
```

Then read each of the five files' `title:` line. Change 0234 is `in-progress` under a concurrent autonomous run — re-read it immediately before writing, and stage only the paths this task owns.

- [ ] **Step 2: Rewrite the five title lines**

Edit each `title:` line to exactly:

```yaml
title: "The manifest's elsewhere: check proves a word occurrence, not a real config read"
title: 'Split gate-execution.md: probe evidence should not sit on a blocking-read surface'
title: 'field_of() silently truncates a model ID containing / or :'
title: 'aborted-run is blind to a run that stops after the build: commits on an unpushed branch, every field coherent'
title: "Clear change 0202's three minor findings: dead guard, stale baseline comment, wrong plan pattern"
```

(in file order: 0121, 0234, 0173, 0211, 0217).

- [ ] **Step 3: Verify the logical values are byte-unchanged**

```bash
cd /Users/homer/dev/docket/.docket
. /Users/homer/dev/docket/scripts/lib/docket-frontmatter.sh
for f in docs/changes/active/0121-*.md docs/changes/active/0234-*.md \
         docs/changes/archive/*-0173-*.md docs/changes/archive/*-0211-*.md \
         docs/changes/archive/*-0217-*.md; do
  printf '%s\n  raw: %s\n  log: %s\n' "$f" "$(field_raw "$f" title)" "$(field "$f" title)"
done
```

Expected: every `raw` is quoted; every `log` reads exactly as the title did before the edit — no `''`, no stray quote. This sources the **primary tree's** (pre-merge) library on purpose: that is the reader the board will actually use until this PR lands.

- [ ] **Step 4: Widen the learnings receipt**

In `/Users/homer/dev/docket/.docket/docs/changes/learnings/yaml-scalar.md`, update the `hook:` and the `## Apply` paragraph to drop "hand-authored" in favour of *any writer*, matching Task 5's AGENTS.md wording, and add `235` to the `changes:` list with today's date in `updated:`. Append a dated war-story bullet recording that the rule detected violations for weeks while `mint-stub` kept minting them, and that the fix was an unconditional quote at the write boundary rather than another instance of the rule. Leave `promotion_state: promoted` and `promoted_to: AGENTS.md` untouched.

- [ ] **Step 5: Commit and push on the metadata branch**

```bash
cd /Users/homer/dev/docket/.docket
git add docs/changes/active/0121-*.md docs/changes/active/0234-*.md \
        docs/changes/archive/*-0173-*.md docs/changes/archive/*-0211-*.md \
        docs/changes/archive/*-0217-*.md docs/changes/learnings/yaml-scalar.md
git commit -m "fix(0235): quote the five malformed title scalars; widen the learnings receipt"
git push origin HEAD:docket
```

If the push is rejected non-fast-forward, re-run `docket.sh preflight`, re-read the five files, re-apply any lost edit, and push again — loop until it lands.

- [ ] **Step 6: Republish the three archived records onto the integration branch**

`terminal_publish: true` already put all three archived copies on `origin/main` with the broken line intact, so a `docket`-only repair would leave main unparseable and the branches divergent.

```bash
for id in 173 211 217; do
  "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh terminal-publish \
    --id "$id" --outcome done \
    --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs --enabled true
done
```

- [ ] **Step 7: Verify the repair landed on both branches**

```bash
cd /Users/homer/dev/docket
git fetch -q origin main docket
for p in docs/changes/archive/2026-07-31-0173-field-of-silently-truncates-a-model-id-containing-or.md \
         docs/changes/archive/2026-08-05-0211-aborted-run-is-blind-to-a-run-that-stops-after-the-build-com.md \
         docs/changes/archive/2026-08-05-0217-clear-change-0202-s-three-minor-findings-dead-guard-stale-ba.md; do
  printf '%s\n  main:   %s\n  docket: %s\n' "$p" \
    "$(git show origin/main:"$p" | grep -m1 '^title:')" \
    "$(git show origin/docket:"$p" | grep -m1 '^title:')"
done
```

Expected: for all three, `main` and `docket` show the **same quoted** title line.

- [ ] **Step 8: Refresh the board and confirm no rendering regression**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only
cd /Users/homer/dev/docket/.docket
grep -n "manifest" docs/changes/BOARD.md | head
grep -c "''" docs/changes/BOARD.md
```

Expected: the board renders `The manifest's elsewhere: check proves…` with a single apostrophe, and the `''` count is `0`. A non-zero count means a title was single-quoted where the pre-merge reader cannot undouble it — go back to Step 2's quoting-form rule.

- [ ] **Step 9: Confirm `scalar-form` is now clean over the live tree**

```bash
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh board-checks \
  --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
  --metadata-branch docket --integration-branch main 2>/dev/null | grep scalar-form || echo "no scalar-form findings"
```

Expected: `no scalar-form findings`. This is the live-tree backstop the hermetic suite cannot provide — the metadata branch is invisible to it — so record this output in the results file.

---

## Self-Review

**Spec coverage.** §1 shared predicate → Task 3 (library) + Task 4 (checker wiring), with the writer half intentionally dropped per the ADR-0071 revision, which supersedes it. §2 quote at the write boundary + the reader inverse → Task 2 + Task 1. §3 repair the files on both branches → Task 6 Steps 2–7. §4 widen the rule's wording → Task 5 (AGENTS.md) + Task 6 Step 4 (the learnings receipt on `docket`). §5 guards → Task 1 Step 5, Task 2 Step 5, Task 3 Step 5, Task 4 Step 5 (mutation), Task 3 Step 1 (predicate units incl. every near-miss the spec names), Task 2 Step 1 (round-trip over the adversarial table, apostrophe rows mandatory, `discovered_from` still a list, a title logically starting with `"`), Task 4 Step 1 (trailing-colon RED + quoted SILENT), Task 6 Step 9 (live-tree backstop). A5's no-parser rule is a Global Constraint and appears in no task.

The reconcile pass corrected the repair set from six files to five (change 0219's title was reworded and now parses); Task 6 lists the five.

**Placeholder scan.** No TBD, no "add error handling", no "similar to Task N", no step without its command or code. Every test block is literal code, every message string is spelled out verbatim.

**Type consistency.** `docket_yaml_single_quote` (T2) and `docket_scalar_quote_reason` / `docket_scalar_needs_quoting` (T3) are named identically in their definitions, their call sites (`mint-stub.sh:204`, `scalar_form_check`), and their tests. The five leg tokens are spelled the same in the predicate, in `scalar_form_check`'s `case`, and in the unit asserts. `_docket_unwrap_quotes` keeps its existing signature.

**One ordering constraint worth restating:** Task 1 must land before Task 2 (the writer's escape needs its inverse in place), Task 3 before Task 4 (the checker consumes the predicate), and Task 6 last (it depends on none of them, but its Step 8 board check reads the *pre-merge* reader and its quoting-form rule is written for exactly that state).
