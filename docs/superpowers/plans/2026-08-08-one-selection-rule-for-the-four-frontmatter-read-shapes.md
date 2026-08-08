<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0244 — One selection rule for the four frontmatter read shapes](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0244-one-selection-rule-for-the-four-frontmatter-read-shapes.md)**
<!-- docket:backlink:end -->

# One Selection Rule for the Four Frontmatter Read Shapes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record one selection rule for `scripts/lib/docket-frontmatter.sh`'s scalar read shapes, migrate every optional-key `field()` read to the anchored tier, and guard the rule so a new unanchored optional-key read turns the suite red.

**Architecture:** Three moves, in order. (1) A decision table in the library header states which accessor a call site must use, keyed on *can this key be absent*; `scripts/board-checks.md` gets a one-paragraph pointer beside its existing ADR-0057 note. (2) Every live optional-key `field()`/`field_raw()` read migrates to `fm_field` (structured values) or `fm_field_verbatim` (the free-prose `blocked_by`, whose ` #…` is data the comment strip would truncate). (3) A new test file pins the rule three ways: a static (accessor, key) census allowlist over `scripts/`, an orphan pin holding `fm_field_raw`'s production-caller count at zero, and absent-key behavioral fixtures through `render-change-links.sh` — the highest-blast-radius consumer, since it stamps values into specs, plans, results files and PR bodies.

No new scripts, no schema change, no new read shape, no new ADR. ADR-0057 (anchor when the key may be absent) and ADR-0058 (the two-tier logical/raw split) remain the decision record; this change consolidates them into an operational table.

**Tech Stack:** Bash 4.4+, `awk`, POSIX `grep`, the repo's own test harness (`bash tests/test_*.sh`, aggregated by `scripts/run-tests.sh`).

## Global Constraints

- **Bash only.** No new dependency, no new script file. Every edit lands in `scripts/`, `tests/`, or `docs/`.
- **Grep patterns stay `case`/fixed-string simple — no bounded repetition (`{0,600}`).** PATH `grep` here is ugrep and accepts bounded repetition; BSD `/usr/bin/grep` rejects it. A shape test must not depend on which one is found. (Change 0130 learning.)
- **No GNU-only `sed`.** `sed 's/x/\n/g'` inserts a literal `n` on BSD. Use `awk` for any split-on-token work.
- **Board output must be byte-identical** for every change file whose optional keys live in frontmatter. The only permissible diff is for a file currently misreading body prose — and any such diff is a bug fix to be called out in the PR body.
- **`field()`'s whole-file default is NOT inverted.** ADR-0058's two-tier split stands; this is a per-site migration only.
- **No fifth read shape.** `blocked_by` is served by `fm_field_verbatim`, accepting that a hand-quoted `blocked_by` renders with quotes intact.
- **`fm_field_raw` is kept**, documented as an orphan with zero production callers. Deleting it is out of scope and the orphan pin exists so a silent deletion is as visible as a silent adoption.
- Every task ends with the full suite green: `bash scripts/run-tests.sh`.

## File Structure

**Modified — the rule record:**
- `scripts/lib/docket-frontmatter.sh` — the decision table in the header comment (canonical); plus the `readiness()` migration in Task 5.
- `scripts/board-checks.md` — a cross-reference pointer beside the existing ADR-0057 anchoring note (~line 226).

**Modified — the migrations (Tasks 3–5):**
- `scripts/render-change-links.sh` — `branch`, `spec`, `plan`, `results`, `pr`
- `scripts/terminal-publish.sh` — `spec`, `plan`, `results`
- `scripts/reclaim-claims.sh` — `claimed_at`, `branch`
- `scripts/archive-change.sh` — `claimed_at`, `results`
- `scripts/docket-status.sh` — `blocked_by` (verbatim), `promotion_state`
- `scripts/board-checks.sh` — `spec`, `trivial`, the variable-key `plan`/`results` loop, `branch`, `claimed_at`
- `scripts/github-mirror.sh` — `issue`, `spec`, `plan`, `results`
- `scripts/render-learnings-index.sh` — `promotion_state`, `promoted_to`
- `scripts/render-board.sh` — `pr`, `spec`, `branch`, `blocked_by` (verbatim)
- `scripts/lib/docket-frontmatter.sh` — `readiness()`'s `spec`/`trivial`

**Created — the guard:**
- `tests/test_frontmatter_read_shapes.sh` — census allowlist + orphan pin + absent-key behavioral fixtures.
- `tests/runtime-budgets.tsv` — one row for the new test file (a new `tests/test_*.sh` with no row fails `tests/test_runtime_budgets.sh`).

**Untouched (explicitly):** `mint-stub.sh`'s write path, the `list_field`/`int_field` wrappers (they keep delegating to `field()`), the change-template's `type: … # chosen at creation` comment contract, and `_fm_scan`'s quoted-value return (0235's landed behavior).

---

### Task 1: The selection rule, recorded in the library header

The canonical statement of the rule. Nothing migrates yet — this task lands the text and the prose guard that binds it, so every later task has a thing to point at.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (header comment block, after the `Provides:` list ending at the `docket_priority_rank` line, before the `# resolve_deps globals` block)
- Modify: `scripts/board-checks.md` (beside the existing ADR-0057 note at ~line 226)
- Create: `tests/test_frontmatter_read_shapes.sh`
- Modify: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: nothing.
- Produces: the test file `tests/test_frontmatter_read_shapes.sh` with its `ok()`/`no()` helpers and `fail` accumulator, extended by Tasks 2 and 6. The header marker line `# --- THE SELECTION RULE` is the anchor later prose greps key on.

- [ ] **Step 1: Write the failing test**

Create `tests/test_frontmatter_read_shapes.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_frontmatter_read_shapes.sh — guards the ONE selection rule for the frontmatter
# scalar read shapes (change 0244). Three guards, deliberately different in kind:
#   (1) prose  — the decision table exists in the library header and says what it must say;
#   (2) census — every `$(field ` / `$(field_raw ` read in scripts/ names a GUARANTEED-PRESENT
#                key, so a new unanchored optional-key read reddens the suite (ADR-0057);
#   (3) orphan — fm_field_raw's production-caller count stays 0, so silent adoption AND silent
#                deletion are both visible;
#   (4) behavior — an absent-key fixture proves the migrated reads are anchored, since a fixture
#                that HAS the key passes under both implementations and guards nothing.
# Plain bash; hermetic fixtures; no network. Run: bash tests/test_frontmatter_read_shapes.sh
#
# Every pattern here is a fixed string or a `case` glob — never bounded repetition. PATH grep is
# ugrep and accepts `{0,600}`; BSD /usr/bin/grep rejects it, so a shape test that used one would
# pass locally and fail on a stock macOS host (change 0130).
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$ROOT/scripts/lib/docket-frontmatter.sh"
fail=0
ok(){ printf 'ok   - %s\n' "$1"; }
no(){ printf 'NOT OK - %s\n' "$1"; fail=1; }
assert(){ if eval "$2"; then ok "$1"; else no "$1"; fi; }

# ---------------------------------------------------------------------------
# (1) The rule exists in the library header, and says the four things it must.
# ---------------------------------------------------------------------------
assert "library exists" '[ -f "$LIB" ]'

# The header block only — the rule is a CONTRACT for readers of the header, so a matching phrase
# further down in an implementation comment must not satisfy it. Slice on the named terminator
# rather than a generic /^## /: a heading-shaped line inside the block would end the slice early.
header="$(awk '/^# --- THE SELECTION RULE/{f=1} /^# resolve_deps globals/{f=0} f' "$LIB")"
assert "header carries the named rule marker" '[ -n "$header" ]'

# Bind each phrase to the accessor it is asserted ABOUT — a guard that merely asserts a phrase is
# PRESENT survives a rewrite that keeps the words and drops the claim.
rule_says(){ # rule_says DESCRIPTION FIXED-STRING
  if printf '%s\n' "$header" | grep -qF -- "$2"; then ok "rule: $1"; else no "rule: $1 (missing: $2)"; fi
}
rule_says "guaranteed-present keys may use field()"       'guaranteed present'
rule_says "an absent-capable key takes the anchored tier" 'may be ABSENT'
rule_says "structured anchored values use fm_field"       'fm_field'
rule_says "free-prose blocked_by uses fm_field_verbatim"  'fm_field_verbatim'
rule_says "own-decoding callers use the raw tier"         'field_raw'
rule_says "when unsure, anchor"                           'When unsure'
rule_says "the rule cites ADR-0057"                       'ADR-0057'
rule_says "the rule cites ADR-0058"                       'ADR-0058'

# board-checks.md points AT the canonical table rather than restating it — a restatement
# accumulates its own guards and quietly becomes load-bearing.
BC_MD="$ROOT/scripts/board-checks.md"
assert "board-checks.md points at the library header rule" \
  'grep -qF "docket-frontmatter.sh" "$BC_MD" && grep -qF "selection rule" "$BC_MD"'

exit "$fail"
```

Then add the budget row. `tests/runtime-budgets.tsv` is TAB-separated; insert the row in sorted position among the other `tests/test_f*` rows:

```
tests/test_frontmatter_read_shapes.sh	10	parallel
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_frontmatter_read_shapes.sh`
Expected: FAIL — `NOT OK - header carries the named rule marker`, plus a `NOT OK - rule: …` for each of the eight phrases, plus `NOT OK - board-checks.md points at the library header rule`.

Also run: `bash tests/test_runtime_budgets.sh`
Expected: PASS (the budget row was added in step 1; if it FAILs, the row's separators are spaces rather than tabs).

- [ ] **Step 3: Write the rule into the library header**

In `scripts/lib/docket-frontmatter.sh`, insert this block immediately after the `#   docket_priority_rank VALUE       — print the rank index; empty/unknown uses the default rank.` line and immediately before the blank `#` line preceding `# resolve_deps globals (keyed by integer id):`:

```bash
#
# --- THE SELECTION RULE (canonical; change 0244) ------------------------------------------------
# Four scalar read shapes with silently different behavior. The question that picks one is never
# "is this read anchored?" but "CAN THIS KEY BE ABSENT from the frontmatter of every file this
# call site reads?" — because an unanchored read of an absent key does not return empty, it runs
# past the closing `---` and returns whatever body line happens to open with that word. In a repo
# whose subject matter IS the field names, body prose opening `pr:` or `spec:` is not a contrived
# fixture; it is the normal content of a change file.
#
#   caller needs                                          | accessor
#   ------------------------------------------------------|---------------------
#   key guaranteed present, logical value                  | field
#   key guaranteed present, caller decodes quotes itself   | field_raw
#   key may be ABSENT, ordinary structured value           | fm_field
#   key may be ABSENT, caller decodes quotes itself        | fm_field_raw
#   key may be ABSENT, caller JUDGES the YAML form as authored, or the value is free prose where
#     a whitespace-preceded `#` is DATA (blocked_by)       | fm_field_verbatim
#
# GUARANTEED PRESENT means every file the call site reads carries the key, by template. Today that
# is: change files — id, status, slug, title, priority, created, updated; ADRs — id, status, title,
# change, date; learnings findings — slug, hook, topics. Those sites stay on field()/field_raw()
# with no churn: the frontmatter line is necessarily the first match, so whole-file scanning is a
# safe optimization, grandfathered rather than recommended.
#
# EVERY OTHER KEY takes an anchored read — never field() (ADR-0057). In docket's own schema the
# absent-capable set is spec, plan, results, branch, pr, issue, blocked_by, type, claimed_at,
# trivial, auto_groomable, promotion_state, promoted_to, discovered_from.
#
# Within the anchored tier, fm_field is the default. fm_field_verbatim is for exactly two jobs:
# a consumer JUDGING the scalar's YAML form as authored (board-checks's scalar_form_check, which
# cannot be handed a value the reader already repaired), and a free-prose value where the comment
# strip would TRUNCATE data — blocked_by, whose `PR #69 is stale` arrives as `PR` through fm_field.
# The accepted cost is that a hand-quoted blocked_by renders with its quotes intact.
#
# The raw tier (field_raw / fm_field_raw) is for a caller doing its OWN quote/escape decoding
# (ADR-0058). Two live callers, both field_raw on always-present keys: render-learnings-index.sh's
# dequote() on hook, and board-checks.sh's scalar_form_check on title, which must see the quotes to
# know whether a colon-space is quoted or bare. fm_field_raw has ZERO production callers today —
# tests/test_frontmatter_read_shapes.sh pins that at zero. It is kept deliberately, not by neglect:
# it is the documented raw twin the next optional-key decoding consumer reaches for, and without it
# the raw/anchored quadrant of the table above would be empty. Neither adopt nor delete it silently.
#
# When unsure whether a key is optional, use the anchored shape. Anchoring is always correct;
# whole-file is only ever an optimization. This rule is guarded by the (accessor, key) census in
# tests/test_frontmatter_read_shapes.sh — adding a genuinely always-present key there is a
# conscious one-line edit, which is the point.
#
# The list_field/int_field wrappers deliberately keep delegating to field(): their live production
# keys (id, depends_on, adrs, topics, supersedes/reverses/relates_to, pr) are empirically 0-missing
# across the tree. related/discovered_from have test-only wrapper callers, and discovered_from: is
# genuinely ABSENT from ~96 pre-template change files — so they are test-only, NOT
# template-guaranteed. Migrating the wrappers themselves is out of scope for 0244.
```

- [ ] **Step 4: Write the board-checks.md pointer**

In `scripts/board-checks.md`, immediately after the existing paragraph ending `…strips **neither** the quotes nor a` (the ADR-0057 anchoring note at ~line 226-230), append a new paragraph:

```markdown
The full **selection rule** for all four read shapes — which accessor a call site must use, keyed
on whether the key can be absent — is recorded once, canonically, in the
`scripts/lib/docket-frontmatter.sh` header (change 0244). It is not restated here: a copy would
accumulate its own guards and quietly become load-bearing. The rule is enforced by the
(accessor, key) census in `tests/test_frontmatter_read_shapes.sh`.
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_frontmatter_read_shapes.sh`
Expected: PASS — ten `ok` lines, exit 0.

- [ ] **Step 6: Verify the prose guard actually discriminates**

Mutation-check the guard before believing it. Copy the library aside first — `git checkout -- <file>` restores to HEAD, not to your uncommitted edit, and as a restore step it would destroy the work being tested:

```bash
cp scripts/lib/docket-frontmatter.sh /tmp/fm.bak
# Mutation: delete the fm_field_verbatim line from the table.
grep -v 'a whitespace-preceded `#` is DATA' scripts/lib/docket-frontmatter.sh > /tmp/fm.mut \
  && cp /tmp/fm.mut scripts/lib/docket-frontmatter.sh
bash tests/test_frontmatter_read_shapes.sh   # expect: NOT OK - rule: free-prose blocked_by ...
cp /tmp/fm.bak scripts/lib/docket-frontmatter.sh
bash tests/test_frontmatter_read_shapes.sh   # expect: green again
```

Expected: the mutated run reddens, the restored run is green. If the mutated run stays green, the phrase being grepped is not unique to the table — fix the assert, not the table.

- [ ] **Step 7: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/board-checks.md \
        tests/test_frontmatter_read_shapes.sh tests/runtime-budgets.tsv
git commit -m "docs(0244): record one selection rule for the four frontmatter read shapes"
```

---

### Task 2: Absent-key behavioral fixtures through render-change-links.sh

`render-change-links.sh` is the highest-blast-radius consumer — it stamps `spec`/`plan`/`results`/`pr` values into specs, plans, results files and PR bodies — and it reads five optional keys through the unanchored `field()`. This task writes the fixtures that DETECT that, watches them fail, then migrates the reads.

The fixture must be the **absent-key** one. A change file that *has* the field passes under both the anchored and the unanchored implementation, so the natural fixture guards nothing.

**Files:**
- Modify: `tests/test_frontmatter_read_shapes.sh` (append section 4)
- Modify: `scripts/render-change-links.sh:87-91`

**Interfaces:**
- Consumes: Task 1's `ok`/`no`/`assert` helpers and `fail` accumulator.
- Produces: the `render_cl()` fixture helper and the `$tmp` fixture dir, reused by nothing else (later tasks do not extend this section).

- [ ] **Step 1: Write the failing test**

Append to `tests/test_frontmatter_read_shapes.sh`, immediately **before** its final `exit "$fail"` line:

```bash
# ---------------------------------------------------------------------------
# (4) Behavioral: absent optional keys must read EMPTY, not fall through to body prose.
#
# Driven through render-change-links.sh — the highest-blast-radius consumer, since the values it
# reads get stamped into specs, plans, results files and PR bodies. Two fixtures, because the five
# optional reads are not all observable in one file: fixture 1 discriminates spec/plan/results/pr
# (each renders its own row), fixture 2 discriminates branch (visible only via the plan row's ref).
# One absent-key fixture and one mutation arm PER READ — a fixture that supplies body prose for
# only one key proves exactly one read, and its mirrors can be unanchored later with a green suite.
# ---------------------------------------------------------------------------
CL="$ROOT/scripts/render-change-links.sh"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
cat > "$tmp/docket-config.sh" <<'CFG'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "INTEGRATION_BRANCH=main"
echo "ADRS_DIR=docs/adrs"
echo "METADATA_WORKTREE="
CFG
chmod +x "$tmp/docket-config.sh"

render_cl(){ # render_cl CHANGEFILE
  DOCKET_CONFIG="$tmp/docket-config.sh" GIT=git \
    bash "$CL" --change-file "$1" --repo danielhanold/docket --adrs-dir "$tmp"
}

# --- Fixture 1: spec/plan/results/pr ABSENT from frontmatter, PRESENT as body prose ---
cf1="$tmp/0900-absent-keys.md"
cat > "$cf1" <<'EOF'
---
id: 900
slug: absent-keys
title: Absent optional keys
status: in-progress
priority: medium
created: 2026-08-08
updated: 2026-08-08
branch: feat/absent-keys
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

This body deliberately opens lines with the optional keys, which is ordinary content for a repo
whose subject matter is the field names:

spec: docs/superpowers/specs/PROSE-NOT-A-VALUE.md
plan: docs/superpowers/plans/PROSE-NOT-A-VALUE.md
results: docs/results/PROSE-NOT-A-VALUE.md
pr: https://github.com/danielhanold/docket/pull/99999
EOF
render_cl "$cf1" >/dev/null 2>&1
body1="$(cat "$cf1")"
for leaked in \
  'PROSE-NOT-A-VALUE' \
  '99999' ; do
  if printf '%s\n' "$body1" | grep -qF -- "$leaked"; then
    no "absent-key read leaked body prose into the Artifacts block ($leaked)"
  else
    ok "absent-key read returned empty rather than body prose ($leaked)"
  fi
done
# Positive floor: the renderer DID run and DID rewrite the block, so the two asserts above are
# not passing vacuously against an unwritten file.
assert "renderer rewrote the marker-bounded block" \
  'printf "%s\n" "$body1" | grep -qF "docket:artifacts:end"'

# --- Fixture 2: branch ABSENT from frontmatter, PRESENT as body prose, plan SET ---
# branch is invisible in fixture 1 (it only selects the blob ref for plan/results rows), so it
# needs its own fixture or its read can be unanchored later with the suite still green.
cf2="$tmp/0901-absent-branch.md"
cat > "$cf2" <<'EOF'
---
id: 901
slug: absent-branch
title: Absent branch key
status: in-progress
priority: medium
created: 2026-08-08
updated: 2026-08-08
plan: docs/superpowers/plans/2026-08-08-absent-branch.md
adrs: []
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

branch: feat/PROSE-NOT-A-BRANCH
EOF
render_cl "$cf2" >/dev/null 2>&1
body2="$(cat "$cf2")"
if printf '%s\n' "$body2" | grep -qF -- 'PROSE-NOT-A-BRANCH'; then
  no "absent branch: read leaked body prose into the plan row's blob ref"
else
  ok "absent branch: read returned empty rather than body prose"
fi
assert "plan row rendered (fixture 2 is not vacuous)" \
  'printf "%s\n" "$body2" | grep -qF "2026-08-08-absent-branch.md"'
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_frontmatter_read_shapes.sh`
Expected: FAIL — `NOT OK - absent-key read leaked body prose into the Artifacts block (PROSE-NOT-A-VALUE)`, `NOT OK - … (99999)`, and `NOT OK - absent branch: read leaked body prose into the plan row's blob ref`. The two `not vacuous` asserts pass.

This failure IS the ADR-0057 defect, reproduced. Confirm the failure text names the prose value before continuing — a fixture that fails for some other reason (a broken config stub, a missing `--adrs-dir`) proves nothing.

- [ ] **Step 3: Migrate render-change-links.sh's optional reads**

In `scripts/render-change-links.sh`, replace lines 87-91. Before:

```bash
status="$(field "$CHANGE_FILE" status)"
branch="$(field "$CHANGE_FILE" branch)"
spec="$(field "$CHANGE_FILE" spec)"
plan="$(field "$CHANGE_FILE" plan)"
results="$(field "$CHANGE_FILE" results)"
pr="$(field "$CHANGE_FILE" pr)"
```

After (`status` stays on `field` — it is guaranteed present; the five optional keys anchor):

```bash
status="$(field "$CHANGE_FILE" status)"
# The five optional keys take the ANCHORED read (ADR-0057; the rule table in
# lib/docket-frontmatter.sh). Absent from frontmatter, an unanchored field() runs past the closing
# --- and returns body prose — and this renderer stamps what it reads into specs, plans, results
# files and PR bodies.
branch="$(fm_field "$CHANGE_FILE" branch)"
spec="$(fm_field "$CHANGE_FILE" spec)"
plan="$(fm_field "$CHANGE_FILE" plan)"
results="$(fm_field "$CHANGE_FILE" results)"
pr="$(fm_field "$CHANGE_FILE" pr)"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_frontmatter_read_shapes.sh`
Expected: PASS — every line `ok`, exit 0.

- [ ] **Step 5: Run the consumer's own suite**

Run: `bash tests/test_render_change_links.sh` and `bash tests/test_change_links_coverage.sh`
Expected: PASS, unchanged. These carry the golden-output cases; a byte diff here means the migration changed rendering for a file whose keys ARE in frontmatter, which must not happen.

- [ ] **Step 6: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add tests/test_frontmatter_read_shapes.sh scripts/render-change-links.sh
git commit -m "fix(0244): anchor render-change-links' five optional-key reads"
```

---

### Task 3: Migrate the close-out and lifecycle consumers

`terminal-publish.sh`, `reclaim-claims.sh` and `archive-change.sh` read optional keys unanchored. Grouped because they share a failure mode — each acts on a *path or lease value* it then does something irreversible with (a publish onto the integration branch, a status flip back to `proposed`, a postcondition assertion) — and none is covered by Task 2's fixtures.

**Files:**
- Modify: `scripts/terminal-publish.sh:248`, `:311`
- Modify: `scripts/reclaim-claims.sh:67`, `:70`
- Modify: `scripts/archive-change.sh:120`, `:122`

**Interfaces:**
- Consumes: `fm_field` from `scripts/lib/docket-frontmatter.sh` (already sourced by all three).
- Produces: nothing new. Behavior is unchanged for every file whose keys live in frontmatter.

- [ ] **Step 1: Migrate terminal-publish.sh**

At `scripts/terminal-publish.sh:248`, before:

```bash
  spec_path="$(field "$tmpd/change.md" spec)"
```

After:

```bash
  spec_path="$(fm_field "$tmpd/change.md" spec)"   # anchored: spec: is optional (ADR-0057)
```

At `scripts/terminal-publish.sh:311`, before:

```bash
  for rel in "$(field "$tmpd/change.md" plan)" "$(field "$tmpd/change.md" results)"; do
```

After:

```bash
  # anchored: plan:/results: are optional — an unanchored read would hand a body-prose PATH to
  # the backlink re-stamp, on a record being published onto the integration branch.
  for rel in "$(fm_field "$tmpd/change.md" plan)" "$(fm_field "$tmpd/change.md" results)"; do
```

Leave `scripts/terminal-publish.sh:260` (`field "$tmpd/adr.md" status`) on `field` — `status:` is guaranteed present in every ADR.

- [ ] **Step 2: Migrate reclaim-claims.sh**

At `scripts/reclaim-claims.sh:67` and `:70`, before:

```bash
  claimed="$(field "$f" claimed_at)"; [ -n "$claimed" ] || return 1     # (2a) no positive evidence of expiry
```
```bash
  branch="$(field "$f" branch)"
```

After:

```bash
  # anchored: claimed_at:/branch: are optional (ADR-0057). Body prose reaching (2a) would be
  # positive "evidence" of a lease that was never stamped — this function flips status back to
  # proposed on the strength of it.
  claimed="$(fm_field "$f" claimed_at)"; [ -n "$claimed" ] || return 1  # (2a) no positive evidence of expiry
```
```bash
  branch="$(fm_field "$f" branch)"
```

Leave `:66` (`field "$f" status`) on `field` — guaranteed present.

- [ ] **Step 3: Migrate archive-change.sh**

At `scripts/archive-change.sh:120` and `:122`, before:

```bash
[ -z "$(field "$dest" claimed_at)" ]                     || die "postcondition: claimed_at not cleared"
```
```bash
  [ "$(field "$dest" results)" = "$RESULTS" ] || die "postcondition: results not set to $RESULTS"
```

After:

```bash
# anchored: claimed_at:/results: are optional (ADR-0057). A body-prose match would make the
# cleared-lease postcondition fail on a correctly-archived change — a fail-closed check that
# misfires is as bad as one that misses.
[ -z "$(fm_field "$dest" claimed_at)" ]                  || die "postcondition: claimed_at not cleared"
```
```bash
  [ "$(fm_field "$dest" results)" = "$RESULTS" ] || die "postcondition: results not set to $RESULTS"
```

Leave `:118` (`status`) and `:119` (`updated`) on `field` — both guaranteed present.

- [ ] **Step 4: Run the consumers' suites**

Run: `bash tests/test_closeout.sh` and `bash tests/test_reclaim_claims.sh` and `bash tests/test_results_artifact.sh` and `bash tests/test_mark_publish_deferred.sh`
Expected: PASS, unchanged.

- [ ] **Step 5: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/terminal-publish.sh scripts/reclaim-claims.sh scripts/archive-change.sh
git commit -m "fix(0244): anchor the close-out and lifecycle optional-key reads"
```

---

### Task 4: Migrate the board, mirror, status and learnings consumers

The five derived-view producers. `board-checks.sh` carries the one **variable-key** call in the tree; `docket-status.sh` and `render-board.sh` each carry a `blocked_by` read that must go to `fm_field_verbatim`, not `fm_field`, because its ` #…` is data.

**Files:**
- Modify: `scripts/board-checks.sh:363`, `:442`, `:453`, `:454`
- Modify: `scripts/docket-status.sh:1022`, `:1061`
- Modify: `scripts/github-mirror.sh:113`, `:157`, `:159`, `:203`
- Modify: `scripts/render-learnings-index.sh:115`, `:120`
- Modify: `scripts/render-board.sh:421`, `:452`, `:458`

**Interfaces:**
- Consumes: `fm_field` and `fm_field_verbatim` from the library (all five already source it).
- Produces: nothing new. **Board output must stay byte-identical** for every change file whose optional keys live in frontmatter — verify that explicitly in step 7.

- [ ] **Step 1: Migrate board-checks.sh**

At `:363`, before / after:

```bash
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
```
```bash
  # anchored: spec:/trivial: are optional (ADR-0057) — and a body-prose `spec:` makes a
  # needs-brainstorm change read as build-ready, which is the autonomous builder claiming an
  # undesigned change.
  spec="$(fm_field "$f" spec)"; trivial="$(fm_field "$f" trivial)"
```

At `:442`, the variable-key read inside the `done`-change broken-plan-results loop:

```bash
      val="$(field "$f" "$key")"
```
```bash
      val="$(fm_field "$f" "$key")"   # anchored; $key is plan|results, both optional
```

At `:453`-`:454`:

```bash
    branch="$(field "$f" branch)"
    claimed="$(field "$f" claimed_at)"
```
```bash
    branch="$(fm_field "$f" branch)"        # anchored: optional keys (ADR-0057)
    claimed="$(fm_field "$f" claimed_at)"
```

Do **not** touch `:428`-`:431` — that four-line block is a named mutation-test region (`Mutation 4 deletes these four lines … matched individually`), and `field_raw title` / `fm_field_verbatim blocked_by` are already the correct accessors there.

- [ ] **Step 2: Migrate docket-status.sh**

At `:1022`, before / after:

```bash
    blocked_by="$(field "$f" blocked_by)"
```
```bash
    # VERBATIM, not fm_field: blocked_by is free prose where a whitespace-preceded `#` is DATA
    # (`PR #69 is stale`), and fm_field's comment strip would truncate it to `PR`.
    blocked_by="$(fm_field_verbatim "$f" blocked_by)"
```

At `:1061`:

```bash
    state="$(field "$f" promotion_state)"
```
```bash
    state="$(fm_field "$f" promotion_state)"   # anchored: optional key (ADR-0057)
```

Leave `:1019` (`status`) and `:1021` (`id`) on `field`.

- [ ] **Step 3: Migrate github-mirror.sh**

At `:113`:

```bash
  iss="$(field "$f" issue)"; [ -n "$iss" ] && ISSUE_NUM["$id"]="$iss"
```
```bash
  iss="$(fm_field "$f" issue)"; [ -n "$iss" ] && ISSUE_NUM["$id"]="$iss"   # anchored: optional
```

At `:157` and `:159` inside `build_body()`:

```bash
  priority="$(field "$f" priority)"; spec="$(field "$f" spec)"
```
```bash
  priority="$(field "$f" priority)"; spec="$(fm_field "$f" spec)"   # spec: optional -> anchored
```
```bash
  plan="$(field "$f" plan)"; results="$(field "$f" results)"
```
```bash
  plan="$(fm_field "$f" plan)"; results="$(fm_field "$f" results)"  # optional -> anchored
```

At `:203` inside `mirror_change()`:

```bash
  issue="$(field "$f" issue)"
```
```bash
  issue="$(fm_field "$f" issue)"   # anchored: optional key (ADR-0057)
```

Leave `id`, `title`, `status`, `priority` on `field` throughout.

- [ ] **Step 4: Migrate render-learnings-index.sh**

At `:115` and `:120`:

```bash
  state="$(field "$f" promotion_state)"
```
```bash
  state="$(fm_field "$f" promotion_state)"   # anchored: optional key (ADR-0057)
```
```bash
  F_TO["$slug"]="$(field "$f" promoted_to)"
```
```bash
  F_TO["$slug"]="$(fm_field "$f" promoted_to)"   # anchored: optional key
```

Leave `:112` (`field slug`) and `:113` (`field_raw hook`) alone — both guaranteed present in a finding file, and `hook`'s raw read is a deliberate ADR-0058 own-decoding call site feeding `dequote()`.

- [ ] **Step 5: Migrate render-board.sh**

At `:421` inside `pr_cell()`:

```bash
pr_cell(){ local f="$1" pr num; pr="$(field "$f" pr)"
```
```bash
pr_cell(){ local f="$1" pr num; pr="$(fm_field "$f" pr)"   # anchored: pr: is optional (ADR-0057)
```

At `:452` (the `in-progress` row):

```bash
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(spec_link "$(field "$f" spec)")" "$(field "$f" branch)" ;;
```
```bash
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(spec_link "$(fm_field "$f" spec)")" "$(fm_field "$f" branch)" ;;
```

At `:458` (the `blocked` row):

```bash
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(field "$f" blocked_by)" ;;
```
```bash
          "$(pad "$id")" "$base" "$title" "$priority" "$ctype" "$(fm_field_verbatim "$f" blocked_by)" ;;
```

Add a comment above the `row_format_mapping` `case` block explaining the two accessors:

```bash
    # Optional keys go through the ANCHORED accessors (ADR-0057; rule table in
    # lib/docket-frontmatter.sh). blocked_by uses the VERBATIM shape because its ` #…` is data.
```

Leave `status`, `slug`, `priority`, `created`, `title` on `field`.

- [ ] **Step 6: Run the consumers' suites**

Run: `bash tests/test_board_checks.sh` and `bash tests/test_render_board.sh` and `bash tests/test_docket_status.sh` and `bash tests/test_github_mirror.sh` and `bash tests/test_render_learnings_index.sh` and `bash tests/test_learnings_ledger.sh`
Expected: PASS, unchanged.

- [ ] **Step 7: Prove the board output is byte-identical**

Render the real board before and after the migration and diff. Run from the repo root:

```bash
git stash
bash scripts/render-board.sh --changes-dir .docket/docs/changes > /tmp/board.before 2>/dev/null
git stash pop
bash scripts/render-board.sh --changes-dir .docket/docs/changes > /tmp/board.after 2>/dev/null
diff -u /tmp/board.before /tmp/board.after && echo "BOARD BYTE-IDENTICAL"
```

Expected: `BOARD BYTE-IDENTICAL`. If the diff is non-empty, do NOT proceed — read each differing row. A diff means some change file was being misread from body prose; that is a real bug fix, but it must be understood, recorded in the results file, and called out in the PR body rather than absorbed silently. (If `render-board.sh` rejects those flags, run `bash scripts/docket.sh render-board --changes-dir .docket/docs/changes` instead; take the invocation from `scripts/render-board.md`'s Usage section.)

- [ ] **Step 8: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add scripts/board-checks.sh scripts/docket-status.sh scripts/github-mirror.sh \
        scripts/render-learnings-index.sh scripts/render-board.sh
git commit -m "fix(0244): anchor the board, mirror, status and learnings optional-key reads"
```

---

### Task 5: Migrate readiness() inside the shared library

`readiness()` reads `spec:` and `trivial:` unanchored. This is the only migration that changes **shared-library behavior for every caller at once** (docket-status, render-board, github-mirror all consume its token), so it gets its own task and its own gate.

The behavior differs only when `spec:`/`trivial:` is absent from frontmatter while body prose opens such a line. Today that misread makes a needs-brainstorm change report `build-ready` — the autonomous builder then claims an undesigned change. The new behavior is strictly the safer one.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:312` (inside `readiness()`)
- Modify: `tests/test_docket_frontmatter.sh` (append a readiness absent-key case)

**Interfaces:**
- Consumes: `fm_field`, defined earlier in the same file (line ~240) — the definition precedes `readiness()`, so no reordering is needed.
- Produces: `readiness()` with an unchanged signature and unchanged return vocabulary (`build-ready | needs-brainstorm | auto-groom-blocked | waiting`).

- [ ] **Step 1: Write the failing test**

Append to `tests/test_docket_frontmatter.sh`, before its final exit/summary lines:

```bash
# --- readiness(): spec:/trivial: are OPTIONAL, so the reads must be anchored (change 0244) ---
# Without anchoring, a needs-brainstorm change whose BODY opens a `spec:` line reports build-ready
# and the autonomous builder claims an undesigned change. The fixture must OMIT the key: a change
# that HAS spec: in frontmatter reports build-ready under both implementations.
rdy="$(mktemp -d)"
mkdir -p "$rdy/active" "$rdy/archive"
cat > "$rdy/active/0902-prose-spec.md" <<'EOF'
---
id: 902
slug: prose-spec
title: Prose spec
status: proposed
priority: medium
created: 2026-08-08
updated: 2026-08-08
depends_on: []
---

## Why

The design will live at
spec: docs/superpowers/specs/2026-08-08-not-a-real-value-design.md
once someone writes it.
EOF
cat > "$rdy/active/0903-prose-trivial.md" <<'EOF'
---
id: 903
slug: prose-trivial
title: Prose trivial
status: proposed
priority: medium
created: 2026-08-08
updated: 2026-08-08
depends_on: []
---

## Why

Whether this is
trivial: true
is exactly the open question.
EOF
resolve_deps "$rdy"
assert "readiness: body-prose spec: does not make a change build-ready" \
  '[ "$(readiness "$rdy/active/0902-prose-spec.md")" = "needs-brainstorm" ]'
assert "readiness: body-prose trivial: does not make a change build-ready" \
  '[ "$(readiness "$rdy/active/0903-prose-trivial.md")" = "needs-brainstorm" ]'
rm -rf "$rdy"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: FAIL — both new asserts report `NOT OK`, because `readiness()` currently returns `build-ready` for each (it read the prose line as the value).

- [ ] **Step 3: Migrate readiness()**

In `scripts/lib/docket-frontmatter.sh`, at line ~312 inside `readiness()`, before:

```bash
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
```

After:

```bash
  # ANCHORED (change 0244; ADR-0057). Both keys are optional, and this is the one migration that
  # changes behavior for every caller of readiness() at once (docket-status, render-board,
  # github-mirror). It differs only when spec:/trivial: is absent from frontmatter while body
  # prose opens such a line — where the OLD behavior reported build-ready for an undesigned
  # change, which is the autonomous builder claiming work that was never designed.
  spec="$(fm_field "$f" spec)"; trivial="$(fm_field "$f" trivial)"
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: PASS — both new asserts `ok`, and every pre-existing readiness assert still `ok`.

- [ ] **Step 5: Run every readiness consumer's suite**

Run: `bash tests/test_render_board.sh` and `bash tests/test_docket_status.sh` and `bash tests/test_github_mirror.sh` and `bash tests/test_auto_groom.sh` and `bash tests/test_board_checks.sh`
Expected: PASS, unchanged.

- [ ] **Step 6: Re-verify the board is byte-identical**

Re-run Task 4 Step 7's before/after diff, now with `readiness()` migrated. `readiness()` feeds the `proposed` rows' readiness cell and the digest's `ready` line, so a diff here would change which changes the implementer selects.

Expected: `BOARD BYTE-IDENTICAL`, and the digest's `ready` line unchanged:

```bash
bash scripts/docket.sh docket-status --digest-only 2>/dev/null | grep '^ready ' > /tmp/ready.after
```

Compare against the same line captured before the branch. Any id that disappeared from `ready` was previously build-ready only because of a body-prose misread — a real fix, to be named in the results file and the PR body.

- [ ] **Step 7: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "fix(0244): anchor readiness()'s optional spec:/trivial: reads"
```

---

### Task 6: The census guard and the fm_field_raw orphan pin

The class guard. Tasks 2–5 fixed the known sites; this one makes a **new** unanchored optional-key read turn the suite red, so the rule survives the next call site written by someone who has not read the header.

The guard is anchored on the **consuming code** — it greps `scripts/` — never on a hand-maintained list of migrated sites. An allowlist of exceptions answers "is this one expected?", never "does this one exist?", and a list of migrated sites would age directly into the gap it was written to close. What IS allowlisted is the small, closed set of **guaranteed-present keys**, which is a property of the templates, not of the call sites.

**Files:**
- Modify: `tests/test_frontmatter_read_shapes.sh` (insert sections 2 and 3 between section 1 and section 4)

**Interfaces:**
- Consumes: Task 1's `ok`/`no`/`assert` helpers, `$ROOT`, `$LIB`, and the `fail` accumulator; the migrated state of `scripts/` from Tasks 2–5.
- Produces: nothing consumed elsewhere.

- [ ] **Step 1: Write the failing test**

Insert into `tests/test_frontmatter_read_shapes.sh`, after section (1)'s `board-checks.md` assert and **before** section (4)'s `CL=` line:

```bash
# ---------------------------------------------------------------------------
# (2) Census: every `$(field ` / `$(field_raw ` read in scripts/ names a GUARANTEED-PRESENT key.
#
# Anchored on the CONSUMING CODE — grep the scripts — never on a list of already-migrated sites.
# An allowlist of migrated sites answers "is this one expected?", never "does this one exist?",
# and would age straight into the gap it was written to close. The allowlist below is instead the
# closed set of keys the TEMPLATES guarantee, so adding to it is a conscious one-line edit.
#
# The library itself is path-excluded: list_field/int_field legitimately delegate to field(), and
# field() delegates to field_raw(). Those are the shapes' own plumbing, not consumer call sites.
# ---------------------------------------------------------------------------
ALLOW_FIELD=' id status slug title priority created updated change date '
ALLOW_FIELD_RAW=' title hook '

# Split each line on `$(` and keep the fragments that OPEN with an accessor name, so several reads
# on one line are all seen (`spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"`).
# awk, not `sed 's/x/\n/g'` — BSD sed inserts a literal `n` there.
census_frags(){ awk '{ n = split($0, p, /\$\(/); for (i = 2; i <= n; i++) print p[i] }' "$1"; }

census_files=()
while IFS= read -r f; do
  case "$f" in "$ROOT/scripts/lib/docket-frontmatter.sh") continue ;; esac
  census_files+=("$f")
done < <(find "$ROOT/scripts" -name '*.sh' -type f | sort)
assert "census scanned a non-empty script population" '[ "${#census_files[@]}" -gt 10 ]'

census_violations=""
census_seen=0
for f in "${census_files[@]}"; do
  while IFS= read -r frag; do
    case "$frag" in
      'field '*)     acc=field ;;
      'field_raw '*) acc=field_raw ;;
      *) continue ;;
    esac
    census_seen=$((census_seen + 1))
    rest="${frag#* }"        # "$f" key)…      (drop the accessor)
    rest="${rest#* }"        # key)…           (drop the FILE argument)
    key="${rest%%)*}"        # key
    case "$acc" in
      field)     allow="$ALLOW_FIELD" ;;
      field_raw) allow="$ALLOW_FIELD_RAW" ;;
    esac
    case "$allow" in
      *" $key "*) ;;
      *) census_violations+="  ${f#$ROOT/}: $acc $key"$'\n' ;;
    esac
  done < <(census_frags "$f")
done

# Population floor: a guard that found nothing to check is not a passing guard.
assert "census found real field()/field_raw() reads to check" '[ "$census_seen" -gt 20 ]'

if [ -z "$census_violations" ]; then
  ok "census: every unanchored read names a guaranteed-present key"
else
  no "census: unanchored read of a key that may be ABSENT — use fm_field (or fm_field_verbatim for
free-prose values where a whitespace-preceded '#' is data). See the selection rule in
scripts/lib/docket-frontmatter.sh. If the key really is present in EVERY file the site reads,
add it to this test's ALLOW_FIELD/ALLOW_FIELD_RAW deliberately. Offending sites:
$census_violations"
fi

# A variable-key read (`field "$f" "$key"`) must FAIL by default, so the census cannot be routed
# around by hoisting the key into a variable. Prove that rather than asserting it in a comment.
vk_frag='field "$f" "$key")'
vk_rest="${vk_frag#* }"; vk_rest="${vk_rest#* }"; vk_key="${vk_rest%%)*}"
case "$ALLOW_FIELD" in
  *" $vk_key "*) no "census: a variable-key read would pass the allowlist" ;;
  *)             ok "census: a variable-key read fails the allowlist by default" ;;
esac

# ---------------------------------------------------------------------------
# (3) Orphan pin: fm_field_raw has ZERO production callers. Both directions matter — a silent
# adoption is a call site that never got the rule applied to it, and a silent deletion removes the
# documented raw twin the rule table's raw/anchored quadrant depends on.
# ---------------------------------------------------------------------------
orphan_hits=0
for f in "${census_files[@]}"; do
  while IFS= read -r frag; do
    case "$frag" in 'fm_field_raw '*) orphan_hits=$((orphan_hits + 1)) ;; esac
  done < <(census_frags "$f")
done
if [ "$orphan_hits" -eq 0 ]; then
  ok "orphan pin: fm_field_raw still has 0 production callers"
else
  no "orphan pin: fm_field_raw gained $orphan_hits production caller(s). That is not forbidden —
it is the documented raw twin for an optional key whose consumer decodes quotes itself — but it is
a deliberate adoption: update the rule table in scripts/lib/docket-frontmatter.sh (which currently
states the count is zero) and this pin together."
fi
# The pin is only meaningful if the accessor still EXISTS — a deletion must not read as "0 callers,
# all good".
assert "orphan pin: fm_field_raw is still defined in the library" \
  'grep -qF "fm_field_raw(){" "$LIB"'
```

- [ ] **Step 2: Run test to verify it fails, then passes**

Run: `bash tests/test_frontmatter_read_shapes.sh`
Expected: PASS on the current (fully migrated) tree — every census assert `ok`.

A guard written to CONFIRM the state you just created detects nothing, so the real verification is step 3. Do not accept this green as evidence.

- [ ] **Step 3: Mutation-verify the census guard in both directions**

Back the file up first — `git checkout --` restores to HEAD, not to your working edit.

```bash
cp scripts/render-change-links.sh /tmp/rcl.bak

# Direction A — a NEW unanchored optional-key read must redden.
perl -0pi -e 's/spec="\$\(fm_field "\$CHANGE_FILE" spec\)"/spec="\$(field "\$CHANGE_FILE" spec)"/' \
  scripts/render-change-links.sh
grep -n 'field "$CHANGE_FILE" spec' scripts/render-change-links.sh   # PROVE the mutation landed
bash tests/test_frontmatter_read_shapes.sh   # expect: NOT OK - census: unanchored read ... spec
cp /tmp/rcl.bak scripts/render-change-links.sh
bash tests/test_frontmatter_read_shapes.sh   # expect: green

# Direction B — a NEW fm_field_raw production caller must redden the orphan pin.
cp scripts/render-board.sh /tmp/rb.bak
perl -0pi -e 's/pr="\$\(fm_field "\$f" pr\)"/pr="\$(fm_field_raw "\$f" pr)"/' scripts/render-board.sh
grep -n 'fm_field_raw' scripts/render-board.sh                       # PROVE the mutation landed
bash tests/test_frontmatter_read_shapes.sh   # expect: NOT OK - orphan pin: fm_field_raw gained 1
cp /tmp/rb.bak scripts/render-board.sh
bash tests/test_frontmatter_read_shapes.sh   # expect: green
```

Expected: each mutated run reddens with the named message; each restored run is green. **Confirm the mutation landed** (the `grep -n` lines) before believing a red or a green — a `perl` substitution that matched nothing produces a meaningless reading in both directions.

Then confirm the tree is clean: `git status --porcelain scripts/` must print nothing.

- [ ] **Step 4: Verify the guard is BSD-grep portable**

Every pattern in the new test is a fixed string or a `case` glob, but verify rather than assert:

```bash
PATH=/usr/bin:/bin bash tests/test_frontmatter_read_shapes.sh
```

Expected: PASS. PATH `grep` in this environment is ugrep, which accepts constructs stock BSD `grep` rejects; this run uses `/usr/bin/grep`. A failure here means a pattern slipped in that is not portable — fix the pattern, do not skip the check.

- [ ] **Step 5: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: PASS, and `tests/test_frontmatter_read_shapes.sh` inside its 10s budget (a budget breach is reported loudly; if it breaches, raise the row rather than thinning the guard).

- [ ] **Step 6: Commit**

```bash
git add tests/test_frontmatter_read_shapes.sh
git commit -m "test(0244): pin the selection rule with a census guard and an fm_field_raw orphan pin"
```

---

### Task 7: Documentation touch-ups and final verification

Close the loop: the script contracts that describe a migrated read must not still describe the old accessor, and `scripts/lib/` needs no new contract file (the rule lives in the header, which is the library's contract surface).

**Files:**
- Modify: `scripts/board-checks.md`, `scripts/docket-status.md`, `scripts/render-board.md`, `scripts/github-mirror.md`, `scripts/terminal-publish.md`, `scripts/reclaim-claims.md`, `scripts/archive-change.md`, `scripts/render-change-links.md`, `scripts/render-learnings-index.md` — only where a sentence names the accessor of a read this change migrated.

**Interfaces:**
- Consumes: the migrated state of every script from Tasks 2–6.
- Produces: nothing.

- [ ] **Step 1: Find every contract sentence that names a migrated read**

Run:

```bash
grep -rn 'field(' scripts/*.md
grep -rn 'unanchored\|whole-file' scripts/*.md
```

Expected: a short list. For each hit, read the surrounding sentence and decide: does it describe a read this change migrated? If yes, it is now wrong and must be corrected. If it describes a `field()` read on a guaranteed-present key, it is still right — leave it.

- [ ] **Step 2: Correct each stale sentence**

For each hit identified in step 1, rewrite the sentence to name the accessor the code now uses, and point at the rule rather than restating it. Pattern to follow (this is the shape, not a literal edit — the exact wording depends on the sentence):

```markdown
The `spec:` read is **anchored** via `fm_field` — the key is optional, so an unanchored read would
return body prose (the selection rule lives in the `scripts/lib/docket-frontmatter.sh` header).
```

Do **not** add a fresh restatement of the rule table to any contract file. Deleting a restatement is never a one-file edit — tests grep the copy, not the source, so a copy quietly becomes load-bearing. One pointer per file, maximum.

- [ ] **Step 3: Run the contract-coverage guard**

Run: `bash tests/test_script_contracts_coverage.sh`
Expected: PASS — every `scripts/<name>.sh` still has its co-located `scripts/<name>.md`, and no new script was added that lacks one.

- [ ] **Step 4: Final whole-branch verification**

```bash
bash scripts/run-tests.sh
git diff --stat origin/main...HEAD
grep -rn '$(field ' scripts/ | grep -v 'scripts/lib/docket-frontmatter.sh'
```

Expected: the suite is green; the diff touches only the files this plan names; and every remaining `$(field ` read names a key from the guaranteed-present allowlist. Read that last list yourself — the census guard is a sampling instrument, and a whole-branch read for meaning is what catches what a grep cannot.

- [ ] **Step 5: Commit**

```bash
git add scripts/
git commit -m "docs(0244): align the script contracts with the migrated read accessors"
```
