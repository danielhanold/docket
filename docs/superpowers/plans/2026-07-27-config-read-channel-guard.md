<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0120 — docket-finalize-change claims integration_branch is read from .docket.yml, but it is an exported resolver key](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0120-docket-finalize-change-claims-integration-branch-is-read-fro.md)**
<!-- docket:backlink:end -->

# Skill config read-channel — correct the finalize provenance claim and guard the class

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Correct the one false claim that `<integration_branch>` is read from `.docket.yml`, and add a sentinel that fails on any *unclassified* `.docket.yml` occurrence in skill prose so occurrence #3 of the class cannot ship silently.

**Architecture:** One new test, `tests/test_config_read_channel.sh`, auto-discovers `skills/**/*.md` (minus two declared `docket-convention` exclusions) and requires every `.docket.yml` occurrence to carry an **in-line class marker** naming an admissible class. Four legitimate sites gain markers; the one false clause is rewritten to name the exported `INTEGRATION_BRANCH`. The rule is keyed on **shape** ("an unclassified occurrence"), never on the spelling of the bad sentence.

**Tech Stack:** Bash (POSIX-ish, `set -uo pipefail`), the repo's glob-discovered `tests/test_*.sh` suite. No new dependencies, no registration step.

## Global Constraints

Copied from the spec and AGENTS.md; every task's requirements implicitly include these.

- **Shape, not spelling.** The reject rule is "an unclassified `.docket.yml` occurrence". Never grep for `resolved from \`.docket.yml\`` or any other enumerated phrasing — AGENTS.md forbids keying a guard on a list of spellings.
- **Never infer the class from the line's wording.** `skills/docket-status/SKILL.md`'s second occurrence ("record back into the change file / `.docket.yml`") names no key at all, so any "the line must name the written key" rule reddens on correct pre-existing prose.
- **Population is computed, not hand-listed.** `find skills -name '*.md'` minus a short declared exclusion list, following the live precedent in `tests/test_skill_size_budgets.sh`.
- **The guard is code: mutation-test it.** A mutation that leaves an assert green is a defect until proven otherwise. Mutation-test the **population** too, not only the verdict — a scan that finds zero occurrences must not read as green.
- **Shell rules (AGENTS.md).** Never `producer | early-exiting-consumer` (`grep -q`, `head`) under `set -o pipefail` — capture into a variable, then `grep <<<"$var"`. A `grep` pattern leading with `--` must use `-e`/`--`.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number** (ADR-0054, enforced by `tests/test_comment_anchor_style.sh`).
- **The feature branch never modifies docket metadata** — no change file, no `BOARD.md`, **no ADR edits**. See *Out-of-band* below for ADR-0052.
- **Size budgets are pinned on both dimensions** by `tests/test_skill_size_budgets.sh`. Current actuals and budgets, re-measured at reconcile against `origin/main` @ `0da1c0aa`:
  | File | lines | words |
  |---|---|---|
  | `skills/docket-finalize-change/SKILL.md` | 189 / 193 | 4131 / 4200 |
  | `skills/docket-status/SKILL.md` | 107 / 118 | 2323 / 2393 |
  | `skills/docket-convention/github-board-mirror.md` | 17 / 19 | 420 / 462 |

## Design decision: the marker sits on the SAME LINE as the occurrence

The spec (§3) requires "an explicit line-level opt-out marker (an HTML comment carrying the class)" and, in *Verification*, assumed each marker would occupy **its own line** (budgeting 189→190, 107→109, 17→18).

**This plan appends the marker to the end of the occurrence's own line instead.** Two reasons, both from the learnings the spec's *Guard discipline* section cites:

1. **Attachment.** `marker-scoped-guard-needs-a-population-floor` names attachment as failure mode 2: a position-sensitive rule ("nearest preceding non-blank line") fails open the moment an edit inserts a blank line or moves the comment one line up, and it reads green. Same-line attachment has no such degree of freedom — there is exactly one line, and it is the line being classified.
2. **Budget.** Zero new lines instead of four, which removes `github-board-mirror.md`'s 2-line-headroom risk from the change entirely.

The spec's stop-point (every occurrence carries a declared class marker) is unchanged; only the marker's placement differs. **Record this deviation in the results file.**

## File Structure

| File | Disposition | Responsibility |
|---|---|---|
| `tests/test_config_read_channel.sh` | **Create** | The whole sentinel: a `scan_tree` classifier, the real-tree run with its population floors, and four mutation fixtures. Self-contained — the suite discovers it by glob. |
| `skills/docket-finalize-change/SKILL.md` | Modify (2 lines) | *Per-change steps* step 1: the false clause → the exported key. The merge-gate paragraph: a `negative` marker. |
| `skills/docket-status/SKILL.md` | Modify (2 lines) | Two `write-back` markers. |
| `skills/docket-convention/github-board-mirror.md` | Modify (1 line) | One `write-back` marker. |

The scanner and its mutation fixtures live in one file because the fixtures must exercise **the same `scan_tree` function** the real run uses; a fixture testing a re-implementation of the rule would prove nothing about the rule that ships.

## Out-of-band (NOT a feature-branch task)

Spec scope item 4 — the dated `## Update` on **ADR-0052** naming the second enforcer — is a **metadata write on `metadata_branch`**, delivered by the parent skill's `docket-adr` dispatch at its review step, with `adrs: [52]` already set on the change. ADRs are docket metadata; the feature branch must not touch `docs/adrs/`. **Do not create or edit any ADR in this worktree.**

---

### Task 1: The sentinel, red against the current tree

**Files:**
- Create: `tests/test_config_read_channel.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `scan_tree <root>` — scans `<root>/skills/**/*.md`, emits tab-separated records on stdout: `file\t<relpath>` per scanned file, `ok\t<relpath>\t<lineno>\t<class>` per classified occurrence, `unclassified\t<relpath>\t<lineno>\t<line text>` per unclassified occurrence. `<relpath>` is relative to `<root>` and therefore always begins `skills/`. Task 2 relies on the marker syntax this task fixes: `<!-- docket:config-read-channel: write-back -->` and `<!-- docket:config-read-channel: negative -->`, with **exactly one space** after the colon and the comment closing with ` -->`.

- [ ] **Step 1: Write the whole guard file**

Create `tests/test_config_read_channel.sh` with exactly this content:

```bash
#!/usr/bin/env bash
# tests/test_config_read_channel.sh — run: bash tests/test_config_read_channel.sh
# Guards ADR-0052's read-channel rule AT THE SKILL-PROSE LAYER (change 0120): a documented config
# key resolves through docket-config.sh and skills read the EXPORTED value from the Step-0
# `preflight` block; a model-read of the config file is not a supported shape. ADR-0052's other
# enforcer, tests/test_docket_example_yml.sh, guards the KEY side (every documented key is wired);
# this one guards the PROSE side (no skill tells an agent to read the file).
#
# SHAPE, NOT SPELLING (AGENTS.md). The reject rule is "an unclassified occurrence of the config
# filename", not "no line says 'resolved from ...'" — an enumerated spelling misses the next
# phrasing, which is exactly how occurrence #2 shipped after ADR-0052 stated the rule. The
# admissible half is CLOSED and declared AT THE SITE:
#
#     <!-- docket:config-read-channel: write-back -->   the line describes a WRITE to the file
#     <!-- docket:config-read-channel: negative -->     the line says the file is NOT read that way
#
# The class is never inferred from wording: docket-status/SKILL.md's second occurrence ("record
# back into the change file / ...") names no key at all, so a "the line must name the written key"
# heuristic would redden on correct pre-existing prose.
#
# THE MARKER SITS ON THE SAME LINE AS THE OCCURRENCE. A position-sensitive attachment rule
# ("nearest preceding non-blank line") fails open the moment an edit inserts a blank line or moves
# the comment, and it reads green — attachment is failure mode 2 in the
# marker-scoped-guard-needs-a-population-floor learning. Same-line attachment has no such degree of
# freedom. A marker classifies EVERY occurrence on its own line.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Built by parts so this file's own source is not an occurrence of the token it scans for.
TOKEN=".docket$(printf '')".yml
MARKER_RE='<!-- docket:config-read-channel: (write-back|negative) -->'

# EXCLUSIONS — declared, each with its reason. Both files ARE the contract that describes the
# config file itself, so a read-channel rule cannot apply to them as written:
#   skills/docket-convention/SKILL.md — defines the config file, its schema, and its layers.
#   skills/docket-convention/references/agent-layer.md — describes the config LAYERING itself.
# Re-examined at change 0120's reconcile rather than waved through. The one convention line that
# carries the guarded shape — "`integration_branch` is a value *read from* the file, so the file
# cannot be located *by* it" — is a statement about WHERE the config file LIVES (on the default
# branch, not the integration branch), and the same file attributes the actual read to the resolver
# ("performed deterministically by the config resolver"). It instructs no agent to parse anything.
EXCLUDE="
skills/docket-convention/SKILL.md
skills/docket-convention/references/agent-layer.md
"

# scan_tree <root> — the single classifier. The real run and EVERY mutation fixture go through this
# function; a fixture exercising a re-implementation would prove nothing about the rule that ships.
# It emits a record per scanned FILE as well as per occurrence, so the caller can assert on the
# POPULATION and not only on the verdicts (learning: backstop-must-compute-not-reenumerate —
# mutation-test the population, because a scan that reaches nothing yields zero findings and reads
# identical to a clean tree).
scan_tree(){
  local root="$1" f rel line n
  while IFS= read -r f; do
    rel="${f#"$root"/}"
    case "$EXCLUDE" in
      *"
$rel
"*) continue ;;
    esac
    printf 'file\t%s\n' "$rel"
    n=0
    while IFS= read -r line || [ -n "$line" ]; do
      n=$((n+1))
      case "$line" in *"$TOKEN"*) ;; *) continue ;; esac
      if [[ $line =~ $MARKER_RE ]]; then
        printf 'ok\t%s\t%s\t%s\n' "$rel" "$n" "${BASH_REMATCH[1]}"
      else
        printf 'unclassified\t%s\t%s\t%s\n' "$rel" "$n" "$line"
      fi
    done < "$f"
  done < <(find "$root/skills" -name '*.md' | sort)
}

# --- (1) THE REAL TREE -------------------------------------------------------
out="$(scan_tree "$REPO")"
files="$(grep    -c "$(printf '^file\t')"         <<<"$out")"
oks="$(grep      -c "$(printf '^ok\t')"           <<<"$out")"
unclassified="$(grep "$(printf '^unclassified\t')" <<<"$out")"

# Population floors. A glob that matches nothing, an exclusion list that swallows the tree, or a
# reader that finds no occurrences must NOT read as green — each of those yields zero unclassified
# findings, which is byte-identical to a clean tree.
assert "population: at least 10 skill files scanned (got $files)" '[ "$files" -ge 10 ]'
assert "population: the finalize skill is in the scanned set" \
  'grep -q -- "$(printf "^file\tskills/docket-finalize-change/SKILL.md$")" <<<"$out"'
assert "population: the two declared exclusions are NOT scanned" \
  '! grep -q -- "$(printf "^file\tskills/docket-convention/")" <<<"$out"'
assert "population: at least 4 occurrences were reached and classified (got $oks)" '[ "$oks" -ge 4 ]'

# Coverage: BOTH admissible classes are actually exercised by the real tree, so neither arm of the
# classifier is dead code. "At least one occurrence is marked" pins a population, never coverage.
assert "coverage: at least one write-back occurrence exists" \
  'grep -q -- "$(printf "^ok\t")" <<<"$(grep -- "write-back$" <<<"$out")"'
assert "coverage: at least one negative occurrence exists" \
  'grep -q -- "$(printf "^ok\t")" <<<"$(grep -- "negative$" <<<"$out")"'

# THE RULE.
assert "every occurrence in a scanned skill file is classified
$unclassified" '[ -z "$unclassified" ]'

# --- (2) MUTATION FIXTURES ---------------------------------------------------
# Against tmpdir copies, never the real tree.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkfix(){ mkdir -p "$tmp/$1/skills/x"; }

# (a) the bad clause, unmarked => REJECTED. Note this fixture is a REGRESSION SPECIMEN, not the
# rule: the guard rejects it because it is unclassified, not because of how it is worded.
mkfix a
printf 'merge it into `<integration_branch>` (resolved from `%s`; not hard-coded `main`).\n' \
  "$TOKEN" > "$tmp/a/skills/x/SKILL.md"
outa="$(scan_tree "$tmp/a")"
assert "mutation (a): an unmarked occurrence of the bad clause is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outa"'

# (b) one MARKED occurrence of each admissible class => PASSES, non-vacuously.
# The positive control is a marked occurrence, NOT the corrected clause: the corrected clause
# contains no occurrence of the token at all, so it would pass while proving nothing.
mkfix b
{ printf 'writes it back into `%s` on the default branch <!-- docket:config-read-channel: write-back -->\n' "$TOKEN"
  printf 'read from the Step-0 block, never by parsing `%s` <!-- docket:config-read-channel: negative -->\n' "$TOKEN"
} > "$tmp/b/skills/x/SKILL.md"
outb="$(scan_tree "$tmp/b")"
assert "mutation (b): marked occurrences yield NO unclassified findings" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outb")" ]'
assert "mutation (b) is non-vacuous: both marked occurrences were actually reached" \
  '[ "$(grep -c -- "$(printf "^ok\t")" <<<"$outb")" = 2 ]'
assert "mutation (b): the write-back class is read off the marker" \
  'grep -q -- "$(printf "^ok\tskills/x/SKILL.md\t1\twrite-back$")" <<<"$outb"'
assert "mutation (b): the negative class is read off the marker" \
  'grep -q -- "$(printf "^ok\tskills/x/SKILL.md\t2\tnegative$")" <<<"$outb"'

# (c) the SAME occurrences with their markers stripped => REJECTED. This is the load-bearing pair
# with (b): it proves the marker, not the sentence, is what admits the line.
mkfix c
sed 's/ <!-- docket:config-read-channel: [a-z-]* -->//' "$tmp/b/skills/x/SKILL.md" > "$tmp/c/skills/x/SKILL.md"
outc="$(scan_tree "$tmp/c")"
assert "mutation (c) is non-vacuous: the marker strip actually changed the fixture" \
  '! cmp -s "$tmp/b/skills/x/SKILL.md" "$tmp/c/skills/x/SKILL.md"'
assert "mutation (c): stripping the markers REJECTS both occurrences" \
  '[ "$(grep -c -- "$(printf "^unclassified\t")" <<<"$outc")" = 2 ]'

# (d) an UNKNOWN class => REJECTED. Keeps the admissible set closed: a future author cannot widen
# the guard by inventing a class name at the site it is meant to constrain.
mkfix d
printf 'some prose about `%s` <!-- docket:config-read-channel: because-i-said-so -->\n' \
  "$TOKEN" > "$tmp/d/skills/x/SKILL.md"
outd="$(scan_tree "$tmp/d")"
assert "mutation (d): an unknown marker class is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outd"'

# (e) the exclusion list is not a blanket: a NON-excluded file under the same directory prefix is
# still scanned. Guards against an exclusion match that is accidentally a prefix match.
mkdir -p "$tmp/e/skills/docket-convention/references"
printf 'unmarked `%s`\n' "$TOKEN" > "$tmp/e/skills/docket-convention/SKILL.md"
printf 'unmarked `%s`\n' "$TOKEN" > "$tmp/e/skills/docket-convention/references/learnings.md"
oute="$(scan_tree "$tmp/e")"
assert "mutation (e): an excluded file is skipped" \
  '! grep -q -- "$(printf "^file\tskills/docket-convention/SKILL.md$")" <<<"$oute"'
assert "mutation (e): a NON-excluded sibling is still scanned and rejected" \
  'grep -q -- "$(printf "^unclassified\tskills/docket-convention/references/learnings.md\t1\t")" <<<"$oute"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run it and verify it fails for exactly the right reason**

Run: `bash tests/test_config_read_channel.sh`

Expected: **FAIL**. Every mutation fixture (a)–(e) and every population floor must print `ok - …`, EXCEPT the two coverage asserts and the rule assert, which must print `NOT OK`:
- `NOT OK - coverage: at least one write-back occurrence exists`
- `NOT OK - coverage: at least one negative occurrence exists`
- `NOT OK - every occurrence in a scanned skill file is classified` — followed by exactly **5** `unclassified` lines: `docket-finalize-change/SKILL.md` ×2, `docket-status/SKILL.md` ×2, `docket-convention/github-board-mirror.md` ×1.

If the population floors are red, the scanner is not reaching the tree — fix that before touching any prose. If **more** than 5 unclassified lines appear, a site the reconcile audit did not find exists: **report it, do not silently mark it.**

- [ ] **Step 3: Commit**

```bash
git add tests/test_config_read_channel.sh
git commit -m "test(0120): sentinel — every .docket.yml occurrence in skill prose must be classified"
```

---

### Task 2: The prose fix and the four class markers

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md` (the *Per-change steps* step 1 clause; the *The rebase-retest merge gate* paragraph)
- Modify: `skills/docket-status/SKILL.md` (the `minted issue` bullet; the `github` mirror paragraph)
- Modify: `skills/docket-convention/github-board-mirror.md` (the `**Projects v2.**` paragraph)

**Interfaces:**
- Consumes: Task 1's marker syntax — `<!-- docket:config-read-channel: write-back -->` / `<!-- docket:config-read-channel: negative -->`, appended to the end of the occurrence's own line, separated by a single space.
- Produces: nothing later tasks consume.

Anchor every edit on the **verbatim clause** quoted below, never on a line number — the files move.

- [ ] **Step 1: Fix the one false claim**

In `skills/docket-finalize-change/SKILL.md`, *Per-change steps* step 1, replace the parenthetical only:

- Find: ``merge it into `<integration_branch>` (resolved from `.docket.yml`; not hard-coded `main`).``
- Replace with: ``merge it into `<integration_branch>` (the exported `INTEGRATION_BRANCH` from the Step-0 `preflight` block; not hard-coded `main`).``

Leave the rest of the sentence and the rest of the paragraph untouched. The phrasing matches what change 0102 established a few lines below, in *The rebase-retest merge gate*.

- [ ] **Step 2: Mark the four legitimate sites**

Append the marker to the **end of each line below**, preceded by one space. Change nothing else on any of these lines.

1. `skills/docket-finalize-change/SKILL.md` — the paragraph beginning ``Guards step 1's merge — the **only** place docket itself merges.``, which ends `…the block documents what each key means and where it's set:` — append:
   `<!-- docket:config-read-channel: negative -->`
2. `skills/docket-status/SKILL.md` — the bullet beginning ``- **`minted issue <id> <n>` / `minted project <owner> <n>` lines**``, ending `…re-run \`docket.sh preflight\`, commit, push).` — append:
   `<!-- docket:config-read-channel: write-back -->`
3. `skills/docket-status/SKILL.md` — the paragraph beginning ``\`github\` is the one-way Issues + Projects v2 mirror``, ending ``…to record back into the change file / `.docket.yml`.`` — append:
   `<!-- docket:config-read-channel: write-back -->`
4. `skills/docket-convention/github-board-mirror.md` — the paragraph beginning `**Projects v2.** The optional half of `github`.`, ending `…skip Projects and still mirror Issues + labels.` — append:
   `<!-- docket:config-read-channel: write-back -->`

- [ ] **Step 3: Run the sentinel and verify it now passes**

Run: `bash tests/test_config_read_channel.sh`

Expected: **PASS**, with zero `NOT OK` lines. Both coverage asserts now green (`write-back` and `negative` each have a real site).

- [ ] **Step 4: Verify the false claim is gone and the mechanism agrees**

Run:

```bash
grep -rn -F 'resolved from `.docket.yml`' skills/ ; echo "exit=$?"
grep -n 'INTEGRATION_BRANCH' skills/docket-finalize-change/SKILL.md
grep -n '^INTEGRATION_BRANCH=' scripts/docket-config.sh || grep -rn 'INTEGRATION_BRANCH' scripts/docket-config.sh | head -3
```

Expected: the first grep finds **nothing** (`exit=1`); the second shows the corrected clause; the third confirms `INTEGRATION_BRANCH` really is an emitted export, so the new prose is true of the running code (never of sibling prose).

- [ ] **Step 5: Verify both size-budget dimensions**

Run:

```bash
for f in skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/github-board-mirror.md; do
  printf '%s lines=%s words=%s\n' "$f" "$(wc -l < "$f" | tr -d ' ')" "$(wc -w < "$f" | tr -d ' ')"
done
bash tests/test_skill_size_budgets.sh
```

Expected: **line counts unchanged** (189 / 107 / 17 — same-line markers add no lines); words approximately 4140 / 2331 / 424, all under 4200 / 2393 / 462. `test_skill_size_budgets.sh` prints `PASS`. If any budget is exceeded, **raise the row in the same diff** with a comment stating why (that file's documented procedure) — do not shrink prose to fit.

- [ ] **Step 6: Run the whole suite**

Run the entire suite in ONE foreground call — never in the background, and never only the tests this plan names (AGENTS.md):

```bash
for t in tests/test_*.sh; do printf '\n== %s\n' "$t"; bash "$t" 2>&1 | tail -5; done
```

Expected: every test ends `PASS`. Pay particular attention to `test_skill_size_budgets.sh`, `test_comment_anchor_style.sh`, `test_skill_facade_wiring.sh`, and `test_docket_example_yml.sh` — the four most likely to react to skill-prose edits.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/github-board-mirror.md
git commit -m "docs(0120): name the exported INTEGRATION_BRANCH; classify the four legitimate .docket.yml sites"
```

---

## Self-review

**Spec coverage.** §1 the one clause → Task 2 Step 1. §2 the audit → re-run at reconcile and recorded in the change's `## Reconcile log`; its result (no second false claim) is why no other prose task exists, and the sentinel now fails on any site the audit missed. §3 the sentinel, its computed population, its declared exclusions, its site-declared classes, and its file-line-text failure messages → Task 1. §4 the exclusion re-examination → confirmed at reconcile; recorded in the guard's own `EXCLUDE` comment; the fallback rephrase is **not** triggered. *Guard discipline*: population floor → Task 1 asserts 1–4; mutation (a)/(b)/(c) → fixtures (a)/(b)/(c), plus (d) closed-class and (e) exclusion-is-not-a-prefix; shape-not-spelling → the rule assert keys on `unclassified`. §ADR-0052 → *Out-of-band*, delivered by the `docket-adr` dispatch.

**Placeholder scan.** No TBDs; every code step carries literal content; the two prose tasks quote the exact find/replace strings.

**Type consistency.** `scan_tree`, `$TOKEN`, `$MARKER_RE`, `$EXCLUDE`, and the three record shapes (`file` / `ok` / `unclassified`) are named identically in Task 1's code, its Interfaces block, and Task 2's steps. The marker string is byte-identical in the guard's regex, its fixtures, and both prose steps.

**Deviation from the spec to record in results:** the marker is same-line rather than own-line (rationale above), so no file gains a line and the spec's 189→190 / 107→109 / 17→18 line projections do not apply.
