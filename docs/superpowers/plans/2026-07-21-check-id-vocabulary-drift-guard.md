# Check-id Vocabulary Drift Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the reverse direction of `board-checks.sh`'s check-id correspondence guard, so a check-id retired from the code but left behind in either documentation surface reddens the suite.

**Architecture:** Change 0104 already ships a partial guard at `tests/test_board_checks.sh:941-999`: it derives the emitted check-id set from `board-checks.sh`, extracts the script's own header enumeration, and compares those two as sets via `comm -3`. What it does **not** do is compare the two *documentation* surfaces as sets — it only checks that every emitted id appears in each, which is blind to a phantom. This plan extends that block in place: it adds a declared `BOARD_CHECK_IDS` array in the sourceable lib, lifts both documentation surfaces from subset to set equality, and adds a lint that keeps the emitted-set extractor honest.

**Tech Stack:** Bash (`set -uo pipefail`), BSD/GNU-portable `grep`/`sed`/`comm`, the repo's own `assert` test harness.

## Global Constraints

- **Extend 0104's block; never duplicate it.** All work lands inside `tests/test_board_checks.sh`'s existing `# --- registration: …` section (starts line 941). A second block beside it would mean two derivations of one set.
- **Do NOT replace 0104's emitted-set extractor.** `grep -oE 'emit [a-z][a-z-]*[[:space:]]+"' "$BCSH" | awk '{print $2}' | sort -u` is shape-anchored and supersedes the widened line-anchor alternation in the spec's §2 fenced block. The spec's `## Reconcile amendments` R2 is normative here.
- **Baseline is 12 distinct check-ids across 17 call sites** (spec R3). Every `11` in the spec body reads `12`; every `16` reads `17`.
- **Set equality, never subset, on every documentation edge** (spec A5). Both documents claim a closed enumeration, so the correspondence is a mirror.
- **Failure messages must name the files to edit.** A legitimate new check-id reddens several asserts at once; that is the feature, and the remedy belongs in the message.
- **Portability:** BSD `grep` does not interpret `\t` inside `grep -E`; use the repo's `printf` idiom if a tab is ever needed. `comm` requires both sides `sort -u`'d.
- Run the suite with: `bash tests/test_board_checks.sh`

---

### Task 1: Declare `BOARD_CHECK_IDS` in the sourceable lib

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:135-137` (append after the `DOCKET_STATUSES` block)
- Test: `tests/test_board_checks.sh` (extend the block at :941)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `BOARD_CHECK_IDS` — a bash array of 12 check-id strings, sourceable from `scripts/lib/docket-frontmatter.sh`. Tasks 2 and 3 compare their extracted sets against `$emitted` (0104's existing variable), not against this array; this array is the *declared* object the surfaces are anchored to and is itself pinned to `$emitted` here.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_board_checks.sh`, immediately after the existing
`assert "every EMITTED check-id is registered in both documentation surfaces (whole-word)" …`
(currently line 999) and **before** the closing `if [ "$fail" = 0 ]` line:

```bash
# --- S0: the DECLARED vocabulary, sourced as a real runtime array (change 0111) -----------------
# board-checks.sh is NOT sourceable (it parses argv and runs the whole walk on source), so the
# vocabulary is declared in the lib that board-checks.sh already sources at :52. That lets this
# guard read the REAL array rather than parsing source text for it — the same mechanism
# tests/test_render_board.sh:1883-1885 uses for DOCKET_STATUSES, and it deletes a whole class of
# tokenizer fragility instead of relocating it.
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$LIB"

assert "BOARD_CHECK_IDS holds the 12 check-ids board-checks.sh emits" \
  '[ "${#BOARD_CHECK_IDS[@]}" = 12 ]'
assert "BOARD_CHECK_IDS SET == the set board-checks.sh actually emits (edit scripts/lib/docket-frontmatter.sh)" \
  '[ -z "$(comm -3 <(printf "%s\n" "${BOARD_CHECK_IDS[*]}" | tr " " "\n" | sort -u) <(printf "%s\n" "$emitted"))" ] \
   || { comm -3 <(printf "%s\n" "${BOARD_CHECK_IDS[*]}" | tr " " "\n" | sort -u) <(printf "%s\n" "$emitted") >&2; false; }'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: FAIL — `NOT OK - BOARD_CHECK_IDS holds the 12 check-ids board-checks.sh emits`, because the array does not exist yet. Under `set -u` an unset array expands to an error; that still surfaces as `NOT OK`, which is the failing state we want.

- [ ] **Step 3: Write minimal implementation**

Append to `scripts/lib/docket-frontmatter.sh`, after line 137 (`DOCKET_STATUSES=(...)`):

```bash

# --- board-checks check-id vocabulary (change 0111) --------------------------------------------
# The CLOSED check-id vocabulary board-checks.sh emits. Declared HERE, beside DOCKET_STATUSES,
# rather than in board-checks.sh itself, because board-checks.sh is not sourceable — a guard
# wanting the set would have to parse its source text, manufacturing exactly the tokenizer that
# can drift from what bash actually assigns. This lib IS sourceable (board-checks.sh sources it at
# :52, well before emit() at :71), so tests/test_board_checks.sh reads the real runtime array.
#
# Accepted impurity: this lib's name says "frontmatter" and a check-id is not a frontmatter field.
# Noted deliberately; rationalising the lib's naming is change 0116's charter.
#
# Every entry is pinned in BOTH directions against the set board-checks.sh emits, against the
# script's own --help header enumeration, against scripts/board-checks.md's per-check sections, and
# against scripts/docket-status.md's `check` report-line row. Adding a check-id means editing all
# four surfaces; the guard's failure messages name them.
BOARD_CHECK_IDS=(board-row-dropped broken-plan-results broken-spec dep-cycle field-domain
                 malformed-id merge-gate-stall merged-orphan publish-deferred
                 stale-finalize-blocked stale-in-progress unknown-commit-ref)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: PASS — both new asserts print `ok - …`, and the final line is `PASS`.

- [ ] **Step 5: Mutation-test the new asserts (both directions)**

These are guards; prove each is load-bearing before committing. Run each mutation, confirm RED, then revert it.

```bash
# (a) delete a real entry -> arity + set compare redden
sed -i '' 's/ publish-deferred$//' scripts/lib/docket-frontmatter.sh
bash tests/test_board_checks.sh 2>&1 | grep -c 'NOT OK'   # expect >= 1
git checkout scripts/lib/docket-frontmatter.sh

# (b) add a phantom entry -> set compare reddens (arity alone would too; both must)
sed -i '' 's/ unknown-commit-ref)/ unknown-commit-ref phantom-check)/' scripts/lib/docket-frontmatter.sh
bash tests/test_board_checks.sh 2>&1 | grep -c 'NOT OK'   # expect >= 1
git checkout scripts/lib/docket-frontmatter.sh

# (c) RENAME one entry -> arity stays 12, so ONLY the set compare can catch it
sed -i '' 's/ malformed-id / malformed-ids /' scripts/lib/docket-frontmatter.sh
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'      # expect the SET assert, not the arity one
git checkout scripts/lib/docket-frontmatter.sh
```

Expected: (a) and (b) redden; (c) reddens **the set assert specifically** — this is the case a count comparison is blind to and is why set equality is used.

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_board_checks.sh
git commit -m "feat(0111): declare BOARD_CHECK_IDS in the sourceable lib, pinned to the emitted set"
```

---

### Task 2: Lift both documentation surfaces from subset to set equality

**Files:**
- Modify: `tests/test_board_checks.sh:994-999` (replace the `reg_ok` membership loop)
- Read-only: `scripts/board-checks.md`, `scripts/docket-status.md`

**Interfaces:**
- Consumes: `$emitted` and `$BCMD` / `$DSMD` (declared by 0104 at :955-956); `BOARD_CHECK_IDS` from Task 1.
- Produces: `$doc_ids` (board-checks.md's set) and `$ds_ids` (docket-status.md's set) — Task 3 does not use them.

**Why this replaces rather than supplements the loop:** the `reg_ok` loop answers "is every emitted id documented?". Set equality answers that *and* "is every documented id emitted?". Keeping both would leave two derivations of the same relation, and the weaker one would be the one a future reader trusts.

- [ ] **Step 1: Write the failing test**

**Anchor on the text, not the line numbers** — Task 1 appended a block to this file, so counts have
moved. In `tests/test_board_checks.sh`, **delete** the region that runs from the comment beginning
`# The `$BCSH` arm this loop used to carry was TAUTOLOGICAL` through the line
`assert "every EMITTED check-id is registered in both documentation surfaces (whole-word)" …`
(inclusive of the `reg_ok=1` loop between them; lines 985-999 as of `HEAD`) — and replace with:

```bash
# --- the two DOCUMENTATION surfaces, pinned in BOTH directions (change 0111) -------------------
# 0104 shipped this as a one-way membership loop: every EMITTED id must appear in each document.
# (Its `$BCSH` arm was dropped as tautological — `$emitted` is derived BY grepping `$BCSH` — and
# the header set-compare above is board-checks.sh's real surface.) That direction alone cannot see
# a PHANTOM: a check-id retired from the code but left behind in either document, or a typo'd
# extra entry, passed green. Both documents assert their enumeration is CLOSED — docket-status.md
# with `∈ {...}`, board-checks.md with its `### Check enumeration` heading — and a closed set that
# can silently over-claim is exactly the failure `correspondence-guard-runs-one-way` names. So each
# document is compared as a SET now, which pins both directions at once.
#
# Each extractor anchors on that document's own structural shape, never a hand-kept list:
#   board-checks.md  — per-check section heads, `**`<id>`**` at line start
#   docket-status.md — the single `check <check-id> ...` report-line row's `{...}` span
doc_ids="$(grep -oE '^\*\*`[a-z-]+`\*\*' "$BCMD" | sed -E 's/\*\*//g; s/`//g' | sort -u)"
ds_row_count="$(grep -cE '^\| `check <check-id>' "$DSMD")"
ds_ids="$(grep -E '^\| `check <check-id>' "$DSMD" \
  | sed -E 's/.*\{([^}]*)\}.*/\1/' | tr ',' '\n' \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' | grep -v '^$' | sort -u)"

# Anchor integrity BEFORE the set compares: `grep -E ... | sed` yields an EMPTY set just as
# happily when the row was retitled as when the enumeration was emptied, and an empty set would
# make the compare fail loudly here but could pass vacuously in a future refactor of this block.
# Pinning the row count at exactly 1 distinguishes "the doc changed shape" from "the doc drifted".
assert "docket-status.md has exactly ONE 'check <check-id>' report-line row for the extractor to anchor on" \
  '[ "$ds_row_count" = 1 ]'
assert "the board-checks.md check-id extraction is non-empty (a retitled section head must redden, not pass vacuously)" \
  '[ "$(grep -c . <<<"$doc_ids")" -ge 1 ]'
assert "the docket-status.md check-id extraction is non-empty (a reflowed table row must redden, not pass vacuously)" \
  '[ "$(grep -c . <<<"$ds_ids")" -ge 1 ]'

assert "emitted check-id SET == scripts/board-checks.md's per-check sections (add or remove a '**\`<id>\`**' section there)" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$doc_ids"))" ] \
   || { echo "board-checks.md drift (left=emitted only, right=documented only):" >&2; \
        comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$doc_ids") >&2; false; }'
assert "emitted check-id SET == scripts/docket-status.md's 'check <check-id>' enumeration (edit that row's {...} set)" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$ds_ids"))" ] \
   || { echo "docket-status.md drift (left=emitted only, right=documented only):" >&2; \
        comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$ds_ids") >&2; false; }'
```

- [ ] **Step 2: Run test to verify it passes at HEAD, then prove it CAN fail**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: PASS — all five new asserts `ok`. Both documents are correct at `HEAD` (all four sets hold the same 12 ids), so a green run here is the expected starting state, **not** evidence the asserts work. Step 3 is what establishes that.

- [ ] **Step 3: Mutation-test in BOTH directions on BOTH documents**

This is the completion bar from `correspondence-guard-runs-one-way`. Run each, confirm RED, revert.

```bash
# (a) REVERSE on board-checks.md — phantom section (the case 0104 was blind to)
printf '\n**`phantom-check`** — a retired check left in the docs.\n' >> scripts/board-checks.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the board-checks.md set assert
git checkout scripts/board-checks.md

# (b) FORWARD on board-checks.md — delete a real section head
sed -i '' 's/^\*\*`merged-orphan`\*\*/**merged-orphan**/' scripts/board-checks.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the board-checks.md set assert
git checkout scripts/board-checks.md

# (c) REVERSE on docket-status.md — phantom entry in the closed enumeration
sed -i '' 's/, malformed-id}/, malformed-id, phantom-check}/' scripts/docket-status.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the docket-status.md set assert
git checkout scripts/docket-status.md

# (d) FORWARD on docket-status.md — drop a real entry
sed -i '' 's/publish-deferred, //' scripts/docket-status.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the docket-status.md set assert
git checkout scripts/docket-status.md

# (e) RENAME on docket-status.md — counts stay equal; only a SET compare catches it
sed -i '' 's/dep-cycle/dep-cycles/' scripts/docket-status.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the docket-status.md set assert
git checkout scripts/docket-status.md

# (f) ANCHOR integrity — retitle the row so the extractor matches nothing
sed -i '' 's/^| `check <check-id>/| `checkx <check-id>/' scripts/docket-status.md
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'   # expect the ds_row_count assert to fire FIRST
git checkout scripts/docket-status.md
```

Expected: every mutant reddens. (e) is the case a count-equality compare would miss. (f) must redden on the **anchor** assert, proving the extractor cannot silently degrade to an empty set.

- [ ] **Step 4: Confirm the tree is clean after mutation testing**

Run: `git status --porcelain scripts/`
Expected: **no output**. Any leftover means a `git checkout` above was missed and a mutation is about to be committed.

- [ ] **Step 5: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "feat(0111): pin both doc surfaces as SETS, closing the guard's reverse direction"
```

---

### Task 3: Lint that no check-id is emitted dynamically

**Files:**
- Modify: `tests/test_board_checks.sh` (append after Task 2's asserts)
- Read-only: `scripts/board-checks.sh`

**Interfaces:**
- Consumes: `$BCSH` (0104, :955).
- Produces: nothing later tasks use.

**Why this is not redundant with the set compares — verified, not assumed:** the emitted-set extractor keys on the literal shape `emit <id> "`. A site written `emit "$var" …` is invisible to it. Mutating `board-checks.sh:181` from `emit field-domain` to `emit "$dyn"` leaves the distinct emitted set at **12**, unchanged, because `field-domain` is emitted at other sites too — so every set compare stays green while a real emit site has silently left the guard's view. Only a call-site count catches it.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_board_checks.sh`, after Task 2's asserts:

```bash
# --- extractor integrity: every emit site uses a LITERAL check-id (change 0111) ----------------
# Everything above derives the emitted set with `emit <id> "` — a literal check-id followed by the
# quoted change-id argument. A site written `emit "$var" ...` matches none of it and is therefore
# invisible to every assert in this section, WITHOUT reddening any of them: the distinct-id set is
# unchanged whenever the dynamic site's id is also emitted somewhere else. Verified, not assumed —
# mutating board-checks.sh:181's `emit field-domain` to `emit "$dyn"` holds the set at 12 and
# drops the call-site count from 17 to 16. So the count is the only thing that can see it.
#
# Comments are stripped first so the header's prose (`emit a table row`, :94) is out of scope; it
# would not match the literal shape anyway, but stripping makes the two counts comparable over the
# same text. `emit(){` is excluded for free by requiring the space after `emit`.
bcsh_code="$(grep -vE '^[[:space:]]*#' "$BCSH")"
emit_sites="$(grep -oE '\bemit [^;|&)]*' <<<"$bcsh_code" | grep -c .)"
emit_literal_sites="$(grep -oE '\bemit [a-z][a-z-]*[[:space:]]+"' <<<"$bcsh_code" | grep -c .)"

assert "board-checks.sh has emit call sites for the lint to inspect (17 at the 0111 baseline)" \
  '[ "$emit_sites" -ge 1 ]'
assert "every board-checks.sh emit call site names a LITERAL check-id (an 'emit \$var' site is invisible to this whole guard)" \
  '[ "$emit_sites" = "$emit_literal_sites" ] \
   || { echo "emit sites: $emit_sites, literal-id sites: $emit_literal_sites — a dynamic check-id would escape the set compares above" >&2; false; }'
```

- [ ] **Step 2: Run test to verify it passes at HEAD**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -10`
Expected: PASS — both asserts `ok`; at `HEAD` `emit_sites` and `emit_literal_sites` are both 17.

- [ ] **Step 3: Mutation-test the lint**

```bash
sed -i '' '181s/emit field-domain/emit "$dyn"/' scripts/board-checks.sh
sed -n '181p' scripts/board-checks.sh   # CONFIRM the mutation actually applied before trusting the result
bash tests/test_board_checks.sh 2>&1 | grep 'NOT OK'
git checkout scripts/board-checks.sh
```

Expected: the literal-site assert reddens with `emit sites: 17, literal-id sites: 16`.

**Verify the mutation applied before reading the result.** Line 181 is a `case` arm (`''|*[!a-z0-9-]*) emit field-domain …`), so a `sed` anchored on leading whitespace silently matches nothing and the run reads green — a no-op mutation that looks like a blind guard (`agent-shell-noop-reads-as-success`). The `sed -n '181p'` echo above is the guard against that.

- [ ] **Step 4: Confirm the tree is clean**

Run: `git status --porcelain scripts/`
Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add tests/test_board_checks.sh
git commit -m "test(0111): lint that no emit site uses a dynamic check-id"
```

---

### Task 4: Documentation — the header pointer and `malformed-id`'s framing

**Files:**
- Modify: `scripts/board-checks.sh:11-13` (add a pointer line **after** the closing `}` line)
- Modify: `scripts/board-checks.md:192` (the `malformed-id` framing) and its `## Invariants` section (:212)

**Interfaces:**
- Consumes: `BOARD_CHECK_IDS` from Task 1 (named in the prose).
- Produces: nothing later tasks use.

- [ ] **Step 1: Add the pointer line to board-checks.sh's header**

The enumeration at `:11-13` is retained **verbatim** — `board-checks.sh:34` is `-h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0`, so this block is user-facing `--help` output, not an internal comment. Replacing it with a pointer would both delete a guarded surface and stop `--help` listing the vocabulary.

Insert **after** line 13 (the line ending `malformed-id}`) and before the `#   Clean tree ⇒ no output` line:

```
#     The set above is declared in lib/docket-frontmatter.sh as BOARD_CHECK_IDS and pinned to it,
#     to board-checks.md, and to docket-status.md by tests/test_board_checks.sh — edit all four.
```

**Do not put a `}` in the added line.** The header extractor is `sed -n '/check-id ∈ {/,/}/p'`, a range that closes on the first line containing `}`; a stray brace here would extend or corrupt the captured span.

- [ ] **Step 2: Run the suite to confirm the header extractor still spans correctly**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: PASS — in particular `ok - emitted check-id SET == the header's own check-id ∈ {...} enumeration`. A `NOT OK` here means the added line disturbed the brace span.

Also confirm the help output still reads correctly:

Run: `bash scripts/board-checks.sh --help | sed -n '8,14p'`
Expected: the `check-id ∈ {…}` set followed by the new two-line pointer.

- [ ] **Step 3: Reword `malformed-id`'s framing in board-checks.md**

Replace at `scripts/board-checks.md:192-193`:

```
**`malformed-id`** — Guard/carve-out, not counted among the named checks above. A change file
whose `id:` field is non-empty but non-integer emits a `malformed-id` finding.
```

with:

```
**`malformed-id`** — Guard/carve-out — it reports a malformed *file* rather than an unhealthy
*change* — but a first-class emitted check-id like the rest, and a full member of the closed
enumeration below. A change file whose `id:` field is non-empty but non-integer emits a
`malformed-id` finding.
```

The old wording ("not counted among the named checks above") contradicts its membership in the enumeration this change now pins in both directions. The section head keeps its uniform `**\`malformed-id\`**` shape, so the extractor is unaffected.

- [ ] **Step 4: Add the closed-vocabulary sentence to `## Invariants`**

Append to `scripts/board-checks.md`'s `## Invariants` section (starts :212):

```
- **The check-id vocabulary is closed, and guarded both ways.** `## Behavior`'s `### Check
  enumeration` is this file's completeness claim: every check-id `board-checks.sh` can emit has a
  section there, and every section there names a check-id it can emit. The set is declared as
  `BOARD_CHECK_IDS` in `lib/docket-frontmatter.sh` and pinned — in both directions — against the
  emitting code, this file, `board-checks.sh`'s `--help` header, and `docket-status.md`'s `check`
  report-line row by `tests/test_board_checks.sh`. Adding a check-id means editing all four.
```

- [ ] **Step 5: Run the full suite**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: PASS. The prose edits must not move any set: `malformed-id` keeps its section head, and the `## Invariants` bullet mentions no new check-id token at a line-start `**\`…\`**` position.

- [ ] **Step 6: Verify the prose edits did not perturb the extracted sets**

Run:

```bash
grep -oE '^\*\*`[a-z-]+`\*\*' scripts/board-checks.md | sed -E 's/\*\*//g; s/`//g' | sort -u | grep -c .
```

Expected: `12` — unchanged. If this reads 13, the `## Invariants` bullet accidentally introduced a line-start `**\`id\`**` head.

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md
git commit -m "docs(0111): point the header at BOARD_CHECK_IDS; fix malformed-id's framing"
```

---

### Task 5: Whole-suite verification

**Files:** none modified.

- [ ] **Step 1: Run the full repo test suite**

There is no runner script; the suite is every `tests/test_*.sh`. Run it as **one foreground
command** and let it finish — it takes several minutes:

```bash
for t in tests/test_*.sh; do
  r="$(bash "$t" 2>&1 | tail -1)"
  printf '%-50s %s\n' "$(basename "$t")" "$r"
done
```

Expected: every line ends `PASS`. Any `FAIL` — or any line that is neither — must be investigated
before the branch is reviewed; `tests/test_render_board.sh` and `tests/test_docket_frontmatter.sh`
both source `scripts/lib/docket-frontmatter.sh`, so Task 1's addition to that lib must not disturb
them.

- [ ] **Step 2: Confirm no stray mutation survived**

Run: `git status --porcelain`
Expected: no output (all work committed, no leftover mutant).

- [ ] **Step 3: Confirm the guard covers all four surfaces**

Run:

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'SET ==|LITERAL check-id|exactly ONE'
```

Expected: five `ok -` lines — the header compare (0104's), the `BOARD_CHECK_IDS` compare, the two document compares, and the literal-check-id lint, plus the anchor assert.
