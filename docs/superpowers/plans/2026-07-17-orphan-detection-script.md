# Orphan Detection Script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two git-only book-keeping-drift health checks — `merged-orphan` and `unknown-commit-ref` — to `scripts/board-checks.sh`, cross-referencing change ids in integration-branch commit subjects against docket's active/archive state.

**Architecture:** Both checks ride the existing `board-checks.sh` rails (shared `emit`/`FINDINGS` accumulator, `lib/docket-frontmatter.sh`, `GIT` mock seam, `(check-id, change-id)` sort, `--strict`). One new pass runs `git log --format` over the `--integration-branch` ref, parses change ids from two docket-convention subject forms (conservative, subjects only), and emits a finding per referenced id that is either still active (orphan) or has no change file (dangling ref). Zero `docket-status.sh` edit — its `health_checks()` already auto-discovers any new check-id emitted by `board-checks.sh`.

**Tech Stack:** Bash (POSIX-ish, bash-4 assoc arrays), git plumbing, the repo's hermetic shell test harness (`tests/test_board_checks.sh`).

## Global Constraints

- **Git-only, offline.** No network, no `gh`. All reads use `git log`/`git cat-file -e`/`git rev-parse` through the `GIT="${GIT:-git}"` seam. (board-checks.sh invariant.)
- **Warn-only.** The checks emit findings and exit; they never modify change files, the index, or any branch.
- **`set -uo pipefail`** is already in force in `board-checks.sh` (note: no `set -e`).
- **Subjects only, conservative grammar.** Parse commit *subject* lines only, ids from exactly two forms: numeric conventional-commit scope `<type>(<id>):` and trailing `(change <id>)`. Bare `#NNNN` and body text are excluded (PR-number false-positive guard — the load-bearing decision from Open Q1).
- **Zero-padding tolerated, normalized to integer** via `$((10#$digits))` (base-10 forces past octal on `0085`).
- **Full history each run, stateless.** No `--since`, no persisted cursor.
- **Determinism.** Findings sort by `(check-id asc, change-id numeric asc)`; evidence is the first (reverse-chron) commit seen for an id.
- **Do NOT touch or decide #0083's class-2 detector** (archived-but-unpublished terminal record). Out of scope entirely.
- Learnings that bind this build: `escape-ere-metacharacters-in-key` (grammar patterns are static/numeric — no user-key interpolation), `green-suite-untested-branch` + `foundational-test-discipline` (every negative fixture must be discriminating: a real change file with that id + a real integration-branch commit, so deleting the grammar guard flips the test), `guards-are-code`, `shell-portability`, `pipefail`.

---

### Task 1: Add `merged-orphan` + `unknown-commit-ref` checks to `board-checks.sh`

Both checks share one extraction pass (grammar + `git log`), so they land together. `merged-orphan` fires for a referenced id whose change file is still under `active/` (non-terminal); `unknown-commit-ref` fires for a referenced id with no change file in `active/` or `archive/`. A referenced id whose file is archived (terminal) yields no finding.

**Files:**
- Modify: `scripts/board-checks.sh` — classify ids by active/archive in the existing file walk; add the extraction + emission block after the `dep-cycle` block, before the final sort/print.
- Test: `tests/test_board_checks.sh` — add a new section after `merge-gate-stall`.

**Interfaces:**
- Consumes (already present in `board-checks.sh`): `GIT`, `CHANGES_DIR`, `INTEGRATION_BRANCH`, `emit <check> <id> <msg>`, the `FILES` array (active+archive `*.md`, sorted), `int_field`.
- Produces: two new emitted check-ids `merged-orphan` and `unknown-commit-ref`, `<check-id>\t<change-id>\t<message>` on stdout, folded into the existing sort and `--strict`.

- [ ] **Step 1: Write the failing tests**

Append this section to `tests/test_board_checks.sh`, immediately BEFORE the `# ===== docket-status wiring sentinels =====` section (i.e. after the `merge-gate-stall` block). It builds one repo, adds `--allow-empty` commits on `main` with crafted subjects (subjects are all the checks read), then writes active/archive change files and runs the script with `--integration-branch main`.

```bash
# ============================ merged-orphan / unknown-commit-ref ============================
# Cross-reference change ids in integration-branch (main) commit *subjects* against active/archive.
# All fixtures use --allow-empty commits (subjects only). Each negative is discriminating: it pairs
# a real change file with a real commit, so the excluded grammar (bare #, body text) or the
# active/archive carve-out is what keeps the finding from firing.
read -r O _ < <(new_repo)
# --- craft integration-branch (main) history: subjects only, via empty commits ---
git -C "$O" checkout main >/dev/null 2>&1
git_quiet -C "$O" commit --allow-empty -m "docket(0050): merged-orphan via conventional scope"
git_quiet -C "$O" commit --allow-empty -m "feat: add a thing (change 0054)"      # trailing form
git_quiet -C "$O" commit --allow-empty -m "Fix a thing #51"                       # bare # only (excluded)
git_quiet -C "$O" commit --allow-empty -m "unrelated subject" -m "body mentions (change 52)"  # body only (excluded)
git_quiet -C "$O" commit --allow-empty -m "docket(0053): terminal record — done" # id 53 is archived
git_quiet -C "$O" commit --allow-empty -m "docket(0099): mystery id with no file" # unknown ref
git -C "$O" checkout docket >/dev/null 2>&1
# --- change files: 50/51/52/54 active, 53 archived (done), 99 absent ---
for pair in 50:orphan 51:barehash 52:bodyonly 54:trailing; do
  id="${pair%%:*}"; slug="${pair##*:}"
  cat > "$O/docs/changes/active/00$id-$slug.md" <<EOF
---
id: $id
slug: $slug
title: $slug
status: in-progress
priority: medium
depends_on: []
EOF
done
cat > "$O/docs/changes/archive/2026-07-01-0053-published.md" <<'EOF'
---
id: 53
slug: published
title: Terminal, published
status: done
priority: medium
depends_on: []
EOF
oout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$O/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# merged-orphan: active id referenced by a merged subject (both grammar forms)
assert "merged-orphan fires for an active id in a conventional scope docket(0050) (id 50)" \
  'has_finding "$oout" merged-orphan 50'
assert "merged-orphan fires for an active id in the trailing (change 0054) form (id 54)" \
  'has_finding "$oout" merged-orphan 54'
# negatives — each discriminating (the id HAS an active file; only the excluded grammar keeps it quiet)
assert "merged-orphan silent for a bare #51 reference (grammar excludes bare #, id 51)" \
  '! has_finding "$oout" merged-orphan 51'
assert "merged-orphan silent for a body-only (change 52) mention (subjects only, id 52)" \
  '! has_finding "$oout" merged-orphan 52'
assert "merged-orphan silent for a docket(0053) subject of an ARCHIVED change (carve-out, id 53)" \
  '! has_finding "$oout" merged-orphan 53'
# unknown-commit-ref: referenced id with no change file at all
assert "unknown-commit-ref fires for docket(0099) with no change file (id 99)" \
  'has_finding "$oout" unknown-commit-ref 99'
# unknown-commit-ref must NOT fire for ids that DO have a file (active or archived)
assert "unknown-commit-ref silent for a known active id (id 50)" \
  '! has_finding "$oout" unknown-commit-ref 50'
assert "unknown-commit-ref silent for a known archived id (id 53)" \
  '! has_finding "$oout" unknown-commit-ref 53'
# evidence: the merged-orphan message names the evidence commit subject
assert "merged-orphan names the evidence commit for id 50" \
  'printf "%s" "$oout" | grep -E "$(printf "^merged-orphan\t50\t")" | grep -qF "docket(0050)"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|merged-orphan|unknown-commit-ref'`
Expected: the new `merged-orphan` / `unknown-commit-ref` assertions report `NOT OK` (the checks do not exist yet; positives don't fire). The pre-existing assertions still pass.

- [ ] **Step 3: Classify ids by active/archive in the existing file walk**

In `scripts/board-checks.sh`, declare the classification maps just before the `FINDINGS=""` line:

```bash
declare -A ID_ACTIVE ID_ARCHIVED ID_EXISTS   # id -> 1; populated in the FILES walk below
FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end
```

Then, inside the `for f in "${FILES[@]}"; do` loop, immediately AFTER the malformed-id guard (the `if [ -z "$id" ]; then … continue; fi` block) and before `status="$(field "$f" status)"`, record the id's presence and directory:

```bash
  ID_EXISTS["$id"]=1
  case "$f" in
    */active/*)  ID_ACTIVE["$id"]=1 ;;
    */archive/*) ID_ARCHIVED["$id"]=1 ;;
  esac
```

- [ ] **Step 4: Add the extraction + emission block**

In `scripts/board-checks.sh`, insert this block AFTER the `dep-cycle` block (after its `for node in "${!ONCYCLE[@]}"; do … done` loop) and BEFORE the `# Emit findings sorted …` comment:

```bash
# --- merged-orphan / unknown-commit-ref: cross-reference integration-branch commit subjects
#     against the active/archive change set. Git-only, subjects only, conservative grammar
#     (numeric conventional-commit scope + trailing "(change N)"); bare #N and bodies excluded
#     to bound PR-number false positives. Zero-padding tolerated (10# strips it). Full history.
declare -A REF_EVIDENCE                       # id -> "<short-sha> <subject>" (first commit seen)
re_scope='^[a-zA-Z]+\(0*([0-9]{1,4})\):'      # docket(0085): … / results(0085): …
re_trailing='\(change 0*([0-9]{1,4})\)'       # … (change 0085)
while IFS=$'\t' read -r ev_sha ev_subject; do
  [ -n "$ev_subject" ] || continue
  refs=""
  [[ "$ev_subject" =~ $re_scope ]]    && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  [[ "$ev_subject" =~ $re_trailing ]] && refs+=" $(( 10#${BASH_REMATCH[1]} ))"
  for rid in $refs; do
    [ -n "${REF_EVIDENCE[$rid]:-}" ] || REF_EVIDENCE["$rid"]="$ev_sha $ev_subject"
  done
done < <("$GIT" -C "$CHANGES_DIR" log --format='%h%x09%s' "$INTEGRATION_BRANCH" 2>/dev/null)

for rid in "${!REF_EVIDENCE[@]}"; do
  ev="${REF_EVIDENCE[$rid]}"
  if [ -n "${ID_ACTIVE[$rid]:-}" ]; then
    emit merged-orphan "$rid" "merged on $INTEGRATION_BRANCH ($ev) but still active (not archived)"
  elif [ -z "${ID_EXISTS[$rid]:-}" ]; then
    emit unknown-commit-ref "$rid" "referenced by $INTEGRATION_BRANCH commit ($ev) but no change file exists"
  fi
  # archived (terminal) ⇒ properly closed out ⇒ no finding
done
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_board_checks.sh`
Expected: final line `PASS`; every `merged-orphan` / `unknown-commit-ref` assertion reports `ok -`, and all pre-existing assertions still pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "feat(docket): add merged-orphan + unknown-commit-ref checks to board-checks.sh (change 0092)"
```

---

### Task 2: Document the two checks

Bring the contract docs and the in-script enumeration in line with the new behavior. Doc-only; no logic change.

**Files:**
- Modify: `scripts/board-checks.sh` — the header-comment `check-id ∈ {…}` enumeration.
- Modify: `scripts/board-checks.md` — Purpose count, Check enumeration entries, the extraction grammar + history-window note.
- Modify: `scripts/docket-status.md` — extend the `check <check-id> …` vocabulary note to name the two new ids.

**Interfaces:**
- Consumes: the behavior built in Task 1.
- Produces: no code; documentation only.

- [ ] **Step 1: Update the `board-checks.sh` header enumeration**

In `scripts/board-checks.sh`, change the header-comment enumeration line:

```
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall}
```

to:

```
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall,
#                 merged-orphan, unknown-commit-ref}
```

- [ ] **Step 2: Update `scripts/board-checks.md`**

Change the Purpose sentence "Performs the five deterministic git-only health checks over the change files (`active/` and `archive/`)" to "Performs the deterministic git-only health checks over the change files (`active/` and `archive/`) and cross-references integration-branch commit subjects against them". Then, in the **Check enumeration** section, after the `merge-gate-stall` entry and before `dep-cycle`/`malformed-id`, add:

```markdown
**`merged-orphan`** — A change id is referenced by a commit *subject* on `--integration-branch`
while the change is still non-terminal (a file under `active/`, not yet archived). This is the
classic orphan: work merged, but the docket record was never closed out. It is a git-history
signal that complements the PR-status sweep — it catches orphans the sweep structurally cannot
(squash-merge under a differently-named branch, an unrecorded `pr:`, or a sweep that never ran).
The message names the evidence commit (short sha + subject). Warn-only; a legitimately
just-merged change has already been archived by the time health checks run (they run after the
sweep), and a transient orphan from a skipped sweep self-clears next pass.

**`unknown-commit-ref`** — A change id is referenced by an `--integration-branch` commit subject
but no change file with that id exists under `active/` or `archive/` (a typo'd or deleted id).
The change-id column is the referenced id; the message names the evidence commit.

**Id-extraction grammar (both checks).** Ids are parsed from commit *subject* lines only, in
exactly two docket-convention forms: a numeric conventional-commit scope `<type>(<id>):`
(e.g. `docket(0085):`, `results(0085):`) and a trailing `(change <id>)` (e.g. `… (change 0085)`).
Zero-padding is tolerated and normalized to the integer value. Bare `#NNNN` and body text are
deliberately excluded — `#NNNN` collides with PR numbers, and subject-only parsing drops free-text
mentions. The full integration-branch history is scanned on every run (stateless; no `--since`
window, no persisted cursor).
```

- [ ] **Step 3: Update `scripts/docket-status.md`**

In `scripts/docket-status.md`, the report-line vocabulary table row for `check` reads:

```
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. |
```

Extend its description to name the enumeration so a reader knows the vocabulary grew:

```
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. `<check-id>` ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}. |
```

(The `check` line *shape* is unchanged — this only documents the two new ids, which surface through the existing `health_checks()` pipe with no wiring change.)

- [ ] **Step 4: Verify docs are internally consistent**

Run: `grep -n -e merged-orphan -e unknown-commit-ref scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.md`
Expected: both ids appear in all three files (script header enumeration, contract check-enumeration + grammar, status vocabulary row).

- [ ] **Step 5: Run the full board-checks suite once more (docs change must not regress behavior)**

Run: `bash tests/test_board_checks.sh`
Expected: final line `PASS`.

- [ ] **Step 6: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.md
git commit -m "docs(docket): document merged-orphan + unknown-commit-ref checks (change 0092)"
```

---

## Self-Review

**1. Spec coverage:**
- Class 1 `merged-orphan` (active id referenced by merged subject) → Task 1.
- Class 3 `unknown-commit-ref` (referenced id with no file) → Task 1.
- Class 2 (terminal-publish gap) → deliberately NOT built (stays #0083's) — no task, by design.
- Extraction grammar (two forms, subjects only, bare `#`/bodies excluded, zero-pad normalized) → Task 1 Step 4 + Task 2 Step 2.
- Full-history stateless window → Task 1 Step 4 (`git log` over full history, no cursor).
- Home = `board-checks.sh`, zero `docket-status.sh` edit (auto-discovery) → no docket-status.sh code touched; only its `.md` vocabulary note.
- Warn-only / detection-only, rides existing sort + `--strict` → Task 1 uses `emit`; no exit-code change.
- Test cases: merged-orphan (both forms), swept/archived → no finding, unknown-commit-ref, negatives (bare `#`, body-only, terminal-publish of archived) → Task 1 Step 1, each discriminating.
- Docs: board-checks.md, docket-status.md, in-script enumeration → Task 2.

**2. Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows exact code.

**3. Type consistency:** Check-ids `merged-orphan` / `unknown-commit-ref` spelled identically across script, tests, and docs. Maps `ID_ACTIVE`/`ID_ARCHIVED`/`ID_EXISTS`/`REF_EVIDENCE` and regex vars `re_scope`/`re_trailing` are consistent between Step 3 and Step 4. Evidence variable is `ev`/`REF_EVIDENCE` throughout.
