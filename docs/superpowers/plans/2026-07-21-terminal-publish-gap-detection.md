# Terminal-publish gap — mark the deferral, stop the checker lying — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a terminal close-out's publish step is *expected* but deferred or blocked, leave a durable `## Publish deferred` marker on the archived change file, surface it as a `publish-deferred` health-check finding, and remove it automatically when a later publish succeeds.

**Architecture:** Three seams. (1) A new **pure, git-free** file editor `scripts/mark-publish-deferred.sh` owns the marker's add/remove — the sole writer, deterministic, `replace`-not-`append` on re-mark. (2) `scripts/board-checks.sh` gains a `publish-deferred` check keyed on the marker's *presence* via a new `publish_deferred()` helper in `lib/docket-frontmatter.sh` — offline, git-only, **never** a branch diff. (3) `scripts/terminal-publish.sh` removes the marker **on the metadata branch, before building its copy-set**, so a successful publish clears the state and the published copy is marker-free.

**Tech Stack:** Bash (`set -uo pipefail`), `awk`, `git`. Hermetic bash test scripts under `tests/` — no framework, no network, no `gh`.

## Global Constraints

Copied verbatim from `AGENTS.md` and the spec; every task's requirements implicitly include this section.

- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail`. Capture into a variable first, then `grep <<<"$var"`.
- **`grep` for a pattern that leads with `--`** must declare it: `grep -E -e "<pat>"` or `grep -qF -- "<pat>"`.
- **awk indent classes are `[^[:space:]]`, never `[^ ]`.**
- **Anchor a frontmatter-field edit to the first `---…---` block**, never a bare column-0 line match.
- **Quote any hand-authored YAML scalar** carrying a colon-space or a boolean keyword.
- **Before rewriting a marker-delimited managed block, validate marker order and balance** — never presence alone, never let a range consume to EOF unintentionally.
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration.
- **Key a guard on syntactic shape**, never an enumerated list of spellings.
- **Never hand-list the sites of a literal you are gating** — derive them from a whole-repo grep.
- **Run the whole suite at the build gate**, never only the tests this plan enumerates.
- **A value a model wrote is untrusted input to a script** (`--detail` here): reject control characters at intake by *shape*, and write it through `awk`'s `ENVIRON[...]`, never string-interpolated `sed`.
- **Marker section name is exactly `## Publish deferred`** (spec §6, settled).
- **Never write the marker under suppression** — `--enabled false` / `terminal_publish: false`, or `main`-mode. A suppressed publish is legitimate success, not a deferral.
- **The check is `publish-deferred`** and reads the marker in the change file — **not** a `git cat-file -e origin/<integration>:<path>` set-diff (spec §3.3, deliberately declined).

---

## File Structure

| File | Responsibility |
|---|---|
| `scripts/mark-publish-deferred.sh` | **Create.** Pure file editor: add (replace-on-re-mark) / remove the `## Publish deferred` section. No git, no network. |
| `scripts/mark-publish-deferred.md` | **Create.** Its contract (Purpose / Usage / Behavior / Exit codes / Invariants). |
| `scripts/docket.sh` | **Modify.** Add `mark-publish-deferred` to the usage block and `WRAPPED_OPS`. |
| `scripts/docket.md` | **Modify.** Add the inventory-table row. |
| `scripts/lib/docket-frontmatter.sh` | **Modify.** Add `publish_deferred()` beside `finalize_blocked()`. |
| `scripts/board-checks.sh` | **Modify.** Add the `publish-deferred` check + register the id in the header set. |
| `scripts/board-checks.md` | **Modify.** Add the `publish-deferred` entry to *Check enumeration*. |
| `scripts/docket-status.md` | **Modify.** Add `publish-deferred` to the closed `check <check-id>` set. |
| `scripts/terminal-publish.sh` | **Modify.** Pre-publish marker removal on the metadata branch (change mode, docket-mode, enabled only). |
| `scripts/terminal-publish.md` | **Modify.** Document the removal step and its ordering rationale. |
| `tests/test_mark_publish_deferred.sh` | **Create.** Hermetic unit tests for the editor. |
| `tests/test_board_checks.sh` | **Modify.** New `publish-deferred` fixture section. |
| `tests/test_terminal_publish.sh` | **Modify.** Real-repo fixture exercising removal + the suppression carve-outs. |
| `skills/docket-convention/SKILL.md` | **Modify.** *Change body sections* gains `## Publish deferred`. |
| `skills/docket-convention/references/terminal-close-out.md` | **Modify.** Step 3 gains the write-marker-on-defer rule. |

**Task order:** 1 (writer) → 2 (check) → 3 (terminal-publish wiring, depends on 1) → 4 (skill docs). Tasks 1 and 2 are independent of each other.

---

### Task 1: `mark-publish-deferred.sh` — the marker's sole writer

**Files:**
- Create: `scripts/mark-publish-deferred.sh`
- Create: `scripts/mark-publish-deferred.md`
- Modify: `scripts/docket.sh:16` (usage block) and `scripts/docket.sh:39` (`WRAPPED_OPS`)
- Modify: `scripts/docket.md:58` (inventory table — insert after the `runner-dispatch` row)
- Test: `tests/test_mark_publish_deferred.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the executable contract Tasks 2 and 3 depend on —
  - Marker heading line, **exactly**: `## Publish deferred`
  - CLI: `mark-publish-deferred.sh --mode add|remove --change-file PATH [--reason deferred|blocked] [--detail TEXT] [--date YYYY-MM-DD] [--integration-branch B] [--id N]`
  - Exit codes: `0` = the file now matches the requested state (including a no-op remove); `1` = a real error (missing/unreadable file, bad args, control characters in `--detail`).
  - **No git.** The caller stages, commits, and pushes.

- [ ] **Step 1: Write the failing test**

Create `tests/test_mark_publish_deferred.sh`:

```bash
#!/usr/bin/env bash
# tests/test_mark_publish_deferred.sh — verifies scripts/mark-publish-deferred.sh (change 0083):
# the sole writer of the `## Publish deferred` marker. Pure file editor — no git, no network, so
# these need only a temp file. Run: bash tests/test_mark_publish_deferred.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/mark-publish-deferred.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

MARKER='## Publish deferred'

# mkfile — writes a minimal archived change file, prints its path.
mkfile(){
  local d; d="$(mktemp -d)"
  cat > "$d/2026-07-08-0043-sample.md" <<'EOF'
---
id: 43
slug: sample
title: A killed proposal
status: killed
priority: medium
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
<!-- docket:artifacts:end -->

## Why

Because.

## Why killed

Obsolete.
EOF
  printf '%s' "$d/2026-07-08-0043-sample.md"
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# --- add ---------------------------------------------------------------------------------------
f="$(mkfile)"
out="$(bash "$SCRIPT" --mode add --change-file "$f" --reason deferred \
        --detail "pending human approval" --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
body="$(cat "$f")"
# NOTE on assert style: never `printf … | grep -q …` under `set -o pipefail` (AGENTS.md) — the
# producer takes SIGPIPE when grep exits early and the 141 surfaces as an intermittent failure.
# Match against a here-string, or grep the file directly.
assert "add exits zero"                       '[ "$rc" -eq 0 ]'
assert "add writes the exact marker heading"  'grep -qxF -- "$MARKER" "$f"'
assert "add writes a dated sub-heading"       'grep -qF -- "### 2026-07-08" "$f"'
# NB: no backticks inside an assert expression — assert runs `eval "$2"`, which would treat them
# as command substitution. Match the backtick positions with `.` instead.
assert "add names the integration branch"     'grep -q "terminal-publish to .main. not completed" "$f"'
assert "add carries the reason prefix"        'grep -qF -- "**deferred**" "$f"'
assert "add carries the free-text detail"     'grep -qF -- "pending human approval" "$f"'
assert "add names the re-arm command"         'grep -qF -- "terminal-publish" "$f"'
assert "add preserves pre-existing body"      'grep -qxF -- "## Why killed" "$f"'
assert "add preserves frontmatter"            'grep -qxF -- "id: 43" "$f"'

# --- add is REPLACE, not APPEND (presence-encoded-state war story (a)) ---------------------------
out="$(bash "$SCRIPT" --mode add --change-file "$f" --reason blocked \
        --detail "direct push to protected main" --date 2026-07-09 --integration-branch main --id 43 2>&1)"; rc=$?
body="$(cat "$f")"
n="$(grep -cxF -- "$MARKER" "$f")"
assert "re-mark exits zero"                             '[ "$rc" -eq 0 ]'
assert "re-mark leaves EXACTLY ONE marker heading"      '[ "$n" -eq 1 ]'
assert "re-mark replaced the old reason"                '! grep -qF -- "pending human approval" "$f"'
assert "re-mark carries the new reason"                 'grep -qF -- "direct push to protected main" "$f"'
assert "re-mark still preserves the trailing section"   'grep -qxF -- "## Why killed" "$f"'

# --- remove ------------------------------------------------------------------------------------
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
body="$(cat "$f")"
assert "remove exits zero"                          '[ "$rc" -eq 0 ]'
assert "remove strips the marker heading"           '! grep -qxF -- "$MARKER" "$f"'
assert "remove strips the marker body"              '! grep -qF -- "direct push to protected main" "$f"'
assert "remove PRESERVES the following section"     'grep -qxF -- "## Why killed" "$f"'
assert "remove preserves the preceding section"     'grep -qxF -- "## Why" "$f"'
assert "remove preserves frontmatter"               'grep -qxF -- "id: 43" "$f"'

# remove on a file with NO marker is an idempotent no-op
before="$(cat "$f")"
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
assert "remove with no marker exits zero"      '[ "$rc" -eq 0 ]'
assert "remove with no marker changes nothing" '[ "$before" = "$(cat "$f")" ]'

# --- marker LAST in the file: removal must not eat the file, nor leave a dangling tail ----------
f2="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f2" --reason deferred --detail "d" \
     --date 2026-07-08 --integration-branch main --id 43 >/dev/null 2>&1
bash "$SCRIPT" --mode remove --change-file "$f2" >/dev/null 2>&1
assert "marker-last removal keeps the final pre-existing section" 'grep -qxF -- "## Why killed" "$f2"'
assert "marker-last removal leaves no marker"                     '! grep -qxF -- "$MARKER" "$f2"'

# --- PROSE MENTION must not be treated as state (has_section's -x rule, applied to the writer) ---
f3="$(mkfile)"
printf '\nA sentence mentioning `%s` in prose.\n' "$MARKER" >> "$f3"
before="$(cat "$f3")"
bash "$SCRIPT" --mode remove --change-file "$f3" >/dev/null 2>&1
assert "remove ignores an inline prose MENTION of the marker" '[ "$before" = "$(cat "$f3")" ]'

# --- untrusted --detail (model-authored-values-are-untrusted-input) ------------------------------
f4="$(mkfile)"
err="$(bash "$SCRIPT" --mode add --change-file "$f4" --reason deferred \
        --detail "$(printf 'line1\nstatus: done')" --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
assert "multi-line --detail is REJECTED"        '[ "$rc" -ne 0 ]'
assert "multi-line --detail names the problem"  'grep -qiE "control|newline|single line" <<<"$err"'
assert "rejected --detail leaves the file untouched" '! grep -qxF -- "$MARKER" "$f4"'

# an ampersand is ordinary English and must survive verbatim (the sed-replacement trap)
f5="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f5" --reason deferred --detail "approval & sign-off pending" \
     --date 2026-07-08 --integration-branch main --id 43 >/dev/null 2>&1
assert "an '&' in --detail survives verbatim" 'grep -qF -- "approval & sign-off pending" "$f5"'

# --- arg validation ------------------------------------------------------------------------------
err="$(bash "$SCRIPT" --mode add --change-file /nonexistent/nope.md --reason deferred 2>&1)"; rc=$?
assert "missing change file exits non-zero" '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode sideways --change-file "$f" 2>&1)"; rc=$?
assert "invalid --mode exits non-zero"      '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode add --change-file "$f" --reason sideways 2>&1)"; rc=$?
assert "invalid --reason exits non-zero"    '[ "$rc" -ne 0 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_mark_publish_deferred.sh`
Expected: FAIL — the first assert (`script exists and is executable`) is NOT OK because `scripts/mark-publish-deferred.sh` does not exist, and every later assert follows it.

- [ ] **Step 3: Write the script**

Create `scripts/mark-publish-deferred.sh`:

```bash
#!/usr/bin/env bash
# scripts/mark-publish-deferred.sh — the sole writer of the `## Publish deferred` marker
# (change 0083). A terminal close-out whose publish step is EXPECTED (terminal_publish: true,
# docket-mode) but consciously deferred or blocked leaves this dated section on the archived
# change file, so the gap is visible where a human reads it instead of living only in a chat
# thread (the #0043 failure mode). `board-checks.sh`'s `publish-deferred` check reads it;
# `terminal-publish.sh` removes it on a successful publish.
#
# PURE FILE EDITOR: no git, no network, no commit, no push. The caller stages, commits, and
# pushes on the metadata branch per docket's field-write rule. This keeps the file edit
# deterministic and testable in one place (ADR-0012 script-vs-model boundary), mirroring
# render-change-links.sh.
#
# Usage:
#   mark-publish-deferred.sh --mode add --change-file PATH --reason deferred|blocked
#                            [--detail TEXT] [--date YYYY-MM-DD] [--integration-branch B] [--id N]
#   mark-publish-deferred.sh --mode remove --change-file PATH
#
#   --mode add     Write the marker. IDEMPOTENT BY REPLACEMENT: an existing section is removed
#                  first, so a re-mark never appends a second heading (the presence-encoded-state
#                  failure re-hit on `## Finalize blocked`). The section is appended LAST.
#   --mode remove  Strip the marker. A file without one is a no-op that exits 0.
#   --reason       Fixed prefix: `deferred` (a human gate that was never answered) or `blocked`
#                  (a wall the run could not pass, e.g. a protected-branch push denial).
#   --detail       Short free text after the prefix. MODEL-AUTHORED ⇒ UNTRUSTED: rejected at
#                  intake if it carries any control character (newline, CR, TAB). Written through
#                  awk ENVIRON, never interpolated into a sed replacement, so `&` and `\1` in
#                  ordinary English survive verbatim.
#
# Exit codes: 0 = the file now matches the requested state. 1 = a real error (bad args, missing
# or unreadable file, rejected --detail). The file is left BYTE-UNTOUCHED on every exit-1 path.
#
# Invariants:
#   - The heading is matched WHOLE-LINE (`$0 == "## Publish deferred"`), never as a substring:
#     change files routinely MENTION marker names in prose, and a substring match would delete
#     from an inline mention to the next heading. Mirrors has_section's `-x` rule.
#   - The section ends at the next COLUMN-0 `## ` heading or EOF. `### ` sub-headings inside the
#     section do not terminate it (`^## ` cannot match `### `, whose third char is `#`).
set -uo pipefail

MODE="" CHANGE_FILE="" REASON="" DETAIL="" DATE="" INT_BRANCH="main" ID=""
MARKER='## Publish deferred'

die(){ printf '%s\n' "mark-publish-deferred: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="${2:-}"; shift ;;
    --change-file) CHANGE_FILE="${2:-}"; shift ;;
    --reason) REASON="${2:-}"; shift ;;
    --detail) DETAIL="${2:-}"; shift ;;
    --date) DATE="${2:-}"; shift ;;
    --integration-branch) INT_BRANCH="${2:-}"; shift ;;
    --id) ID="${2:-}"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

case "$MODE" in add|remove) ;; *) die "invalid --mode: '$MODE' (expected add|remove)" ;; esac
[ -n "$CHANGE_FILE" ] || die "missing --change-file"
[ -f "$CHANGE_FILE" ] || die "change file not found: $CHANGE_FILE"
[ -r "$CHANGE_FILE" ] && [ -w "$CHANGE_FILE" ] || die "change file not readable/writable: $CHANGE_FILE"

# strip_marker FILE — print FILE to stdout with the `## Publish deferred` section removed.
# Whole-line heading match; the section ends at the next column-0 `## ` heading or EOF.
strip_marker(){
  MPD_MARKER="$MARKER" awk '
    BEGIN { skip = 0 }
    {
      if (skip) {
        # A column-0 `## ` heading ends the section. `### ` does not match (3rd char is #).
        if ($0 ~ /^## /) { skip = 0 } else { next }
      }
      if ($0 == ENVIRON["MPD_MARKER"]) { skip = 1; next }
      print
    }
  ' "$1"
}

# write_atomic FILE CONTENT-PRODUCER... — render to a temp file, then move into place. Never
# redirect a producer straight into the file it rewrites: `>` truncates on open, so a failed
# render would destroy the last-good file before its exit code is read (atomic-generated-write).
tmp="$(mktemp)" || die "mktemp failed"
# Every intermediate this script writes is derived from $tmp, so one trap covers them all. Listing
# them explicitly (rather than `rm -f "$tmp"*`) keeps the cleanup from depending on a glob that a
# future intermediate could silently escape.
cleanup(){ rm -f "$tmp" "$tmp.2" "$tmp.3"; }
trap cleanup EXIT

if [ "$MODE" = remove ]; then
  strip_marker "$CHANGE_FILE" > "$tmp" || die "strip failed"
  # Trim any trailing blank lines the strip left, then restore a single terminating newline.
  awk 'BEGIN{n=0} {lines[++n]=$0} END{ last=n; while (last>0 && lines[last]=="") last--; for(i=1;i<=last;i++) print lines[i] }' "$tmp" > "$tmp.2" || die "trim failed"
  mv "$tmp.2" "$CHANGE_FILE" || die "write failed"
  exit 0
fi

# ----- add -----
case "$REASON" in deferred|blocked) ;; *) die "invalid --reason: '$REASON' (expected deferred|blocked)" ;; esac
# Model-authored free text is untrusted input. Reject by SHAPE (any control character), never by
# enumerating bad strings: a newline would inject whole lines into the body, and a TAB would shift
# the findings channel's columns downstream.
case "$DETAIL" in
  *[[:cntrl:]]*) die "--detail must be a single line with no control characters (newline/CR/TAB)" ;;
esac
[ -n "$DATE" ] || DATE="$(date -u +%Y-%m-%d)"
case "$DATE" in
  [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]) ;;
  *) die "invalid --date: '$DATE' (expected YYYY-MM-DD, UTC)" ;;
esac

# Replace, never append: strip any existing section first (idempotent re-mark).
strip_marker "$CHANGE_FILE" > "$tmp" || die "strip failed"
awk 'BEGIN{n=0} {lines[++n]=$0} END{ last=n; while (last>0 && lines[last]=="") last--; for(i=1;i<=last;i++) print lines[i] }' "$tmp" > "$tmp.2" || die "trim failed"

id_hint=""
[ -n "$ID" ] && id_hint=" --id $ID"

{
  cat "$tmp.2"
  printf '\n%s\n\n' "$MARKER"
  printf '### %s — terminal-publish to `%s` not completed\n\n' "$DATE" "$INT_BRANCH"
  # ENVIRON, not interpolation: `&` and `\1` are ordinary English and must survive verbatim.
  MPD_REASON="$REASON" MPD_DETAIL="$DETAIL" awk 'BEGIN{
    d = ENVIRON["MPD_DETAIL"]
    if (d == "") printf "**%s** — no further detail recorded.\n\n", ENVIRON["MPD_REASON"]
    else         printf "**%s** — %s\n\n", ENVIRON["MPD_REASON"], d
  }' </dev/null
  printf 'Close-out steps 1–2 (archive, `## Artifacts` re-render) landed on the metadata branch;\n'
  printf 'the terminal-publish step (copying the archived change file + its `spec:` + its Accepted\n'
  printf 'ADRs onto `%s`) did **not** run. The record is on the metadata branch only.\n\n' "$INT_BRANCH"
  printf '**Re-arm:** complete the publish (`docket.sh terminal-publish%s …`), or record a decision\n' "$id_hint"
  printf 'not to. A successful publish removes this section automatically.\n'
} > "$tmp.3" || die "render failed"

mv "$tmp.3" "$CHANGE_FILE" || die "write failed"
exit 0
```

Then make it executable: `chmod +x scripts/mark-publish-deferred.sh`

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_mark_publish_deferred.sh`
Expected: every line `ok - …`, final line `PASS`, exit 0.

- [ ] **Step 5: Mutation-test the guards (a guard is code)**

Prove each guard is load-bearing. For each mutation: apply it, run the test, confirm a **NOT OK**, then revert.

1. Change `if ($0 == ENVIRON["MPD_MARKER"])` to `if ($0 ~ ENVIRON["MPD_MARKER"])` — expect `remove ignores an inline prose MENTION of the marker` to redden.
2. Delete the `*[[:cntrl:]]*) die …` arm — expect `multi-line --detail is REJECTED` to redden.
3. Replace the add path's leading `strip_marker` call with a plain `cat "$CHANGE_FILE"` — expect `re-mark leaves EXACTLY ONE marker heading` to redden.
4. Change the awk section terminator `/^## /` to `/^#/` — expect `add preserves pre-existing body` or `remove PRESERVES the following section` to redden (a `### ` sub-heading now terminates the section early).

Record in the results file which mutations were run and that each reddened.

- [ ] **Step 6: Write the contract**

Create `scripts/mark-publish-deferred.md`:

```markdown
# scripts/mark-publish-deferred.sh — contract

## Purpose

The sole writer of the `## Publish deferred` marker (change 0083). A terminal close-out whose
publish step is **expected** (`terminal_publish: true`, docket-mode) but consciously **deferred**
or **blocked** leaves this dated section on the archived change file, so the gap is visible where
a human reads it rather than living only in a chat thread — the #0043 failure mode, invisible for
eight days.

Pure file editor: **no git, no network, no commit, no push.** The caller stages, commits, and
pushes on the metadata branch per docket's field-write rule. The model never hand-writes the
section (ADR-0012 script-vs-model boundary).

## Usage

```
mark-publish-deferred.sh --mode add --change-file PATH --reason deferred|blocked
                         [--detail TEXT] [--date YYYY-MM-DD] [--integration-branch B] [--id N]
mark-publish-deferred.sh --mode remove --change-file PATH
```

| Flag | Meaning |
|---|---|
| `--mode add` | Write the marker. **Idempotent by replacement** — an existing section is stripped first, so a re-mark never appends a second heading. Appended last in the file. |
| `--mode remove` | Strip the marker. A file carrying none is a no-op that exits 0. |
| `--change-file` | Path to the change file **in the metadata working tree**. Required; must exist and be writable. |
| `--reason` | Fixed prefix, `add` only: `deferred` (a human gate never answered) or `blocked` (a wall the run could not pass). |
| `--detail` | Short single-line free text after the prefix. Optional. |
| `--date` | UTC `YYYY-MM-DD` for the sub-heading. Defaults to today (UTC). |
| `--integration-branch` | Named in the marker prose. Defaults to `main`. |
| `--id` | Change id, inlined into the re-arm command hint. Optional. |

## Behavior

`add` renders, in order: the exact heading `## Publish deferred`; a dated
`### <date> — terminal-publish to \`<branch>\` not completed` sub-heading; a
`**<reason>** — <detail>` line; the standing prose naming what did not run and where the record
lives; and a `**Re-arm:**` line. `remove` deletes from the heading through the line before the
next column-0 `## ` heading (or EOF), then trims trailing blank lines.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | The file now matches the requested state (including a no-op `remove`). |
| `1` | A real error: bad `--mode`/`--reason`/`--date`, missing `--change-file`, an unreadable or unwritable file, or a `--detail` carrying control characters. **The file is left byte-untouched.** |

## Invariants

- **Whole-line heading match.** The section is located by `$0 == "## Publish deferred"`, never a
  substring: change files routinely *mention* marker names in prose, and a substring match would
  delete from an inline mention to the next heading. Mirrors `has_section`'s `-x` rule.
- **`### ` does not terminate the section.** The terminator is a column-0 `## ` heading; `^## `
  cannot match `### ` (whose third character is `#`).
- **`--detail` is untrusted input.** A model authors it, so it is rejected at intake by *shape*
  (any control character) and written through `awk`'s `ENVIRON[...]` — never interpolated into a
  `sed` replacement, where an `&` in ordinary English ("approval & sign-off") would be
  reinterpreted.
- **Never written under suppression.** Callers must not invoke `--mode add` when
  `terminal_publish: false` or in `main`-mode: a suppressed publish is legitimate *success*, not a
  deferral. The gate lives at the call site (`terminal-publish.sh` and the close-out drivers),
  not here — this script edits whatever file it is handed.
- **Atomic write.** Content is rendered to a temp file and moved into place; the target is never
  the redirect target of a producer that could fail mid-render.
```

- [ ] **Step 7: Register the facade op**

In `scripts/docket.sh`, add to the usage block after the `mint-stub` line (line 26):

```
#   mark-publish-deferred     add/remove the `## Publish deferred` marker on a change file
```

And add the op to `WRAPPED_OPS` (line 39) — append it to the end of the string:

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-adr-index render-learnings-index adr-checks board-checks reclaim-claims mint-stub runner-dispatch mark-publish-deferred"
```

In `scripts/docket.md`, add a row to the inventory table immediately after the `runner-dispatch` row (line 58):

```markdown
| `mark-publish-deferred` | `mark-publish-deferred.sh` | add/remove the `## Publish deferred` marker on a change file (terminal-publish gap visibility, change 0083) |
```

- [ ] **Step 8: Verify the facade and contract sentinels pass**

Run: `bash tests/test_docket_facade.sh && bash tests/test_script_contracts_coverage.sh`
Expected: both end `PASS` / exit 0. Specifically `docket.sh op set == docket.md documented op set` is `ok`, `every wrapped op maps to scripts/<op>.sh` is `ok`, and `contract present for mark-publish-deferred.sh` is `ok`.

- [ ] **Step 9: Commit**

```bash
git add scripts/mark-publish-deferred.sh scripts/mark-publish-deferred.md scripts/docket.sh scripts/docket.md tests/test_mark_publish_deferred.sh
git commit -m "feat(0083): mark-publish-deferred.sh — the Publish deferred marker's sole writer"
```

---

### Task 2: the `publish-deferred` health check

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:107-112` (add `publish_deferred()` after `finalize_blocked()`)
- Modify: `scripts/board-checks.sh:12-13` (header check-id set) and the per-file `FILES` walk
- Modify: `scripts/board-checks.md` (*Check enumeration*, after the `stale-finalize-blocked` entry)
- Modify: `scripts/docket-status.md:344` (the closed `check <check-id>` set)
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: the marker heading `## Publish deferred` (Task 1's contract). This task does **not** call Task 1's script — it only reads the section, so it can be built and tested independently.
- Produces: `publish_deferred FILE` (exit 0 iff the body carries the marker) in `lib/docket-frontmatter.sh`; a finding line `publish-deferred\t<id>\t<message>`.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_board_checks.sh`, immediately **before** the final summary/exit lines:

```bash
# ============================ publish-deferred ============================
# A change carrying the `## Publish deferred` marker emits exactly one publish-deferred finding —
# in archive/ (where the marker is actually written) and in active/ (harmlessly, per spec §3.3).
# NO status gate and NO directory gate: the marker's PRESENCE is the state, so a marker anywhere
# in the change set is a pending deferral. A change without the marker is silent, and an inline
# PROSE MENTION of the marker name is not state (has_section's whole-line rule).
read -r PD _ < <(new_repo)
# id 50: ARCHIVED + marker ⇒ fires (the real shape — the marker is written on the archived file).
cat > "$PD/docs/changes/archive/2026-07-08-0050-deferredkill.md" <<'EOF'
---
id: 50
slug: deferredkill
title: Killed proposal whose publish was deferred
status: killed
priority: medium
depends_on: []
---

## Why killed

Obsolete.

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**deferred** — pending human approval

The record is on the metadata branch only.
EOF
# id 51: ARCHIVED, no marker ⇒ silent.
cat > "$PD/docs/changes/archive/2026-07-08-0051-cleankill.md" <<'EOF'
---
id: 51
slug: cleankill
title: Killed proposal published cleanly
status: killed
priority: medium
depends_on: []
---

## Why killed

Obsolete.
EOF
# id 52: ACTIVE + marker ⇒ fires too (no directory gate).
cat > "$PD/docs/changes/active/0052-activemarker.md" <<'EOF'
---
id: 52
slug: activemarker
title: Active change carrying the marker
status: proposed
priority: medium
depends_on: []
trivial: true
---

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**blocked** — direct push to protected main
EOF
# id 53: a PROSE MENTION of the marker name, not a section ⇒ silent.
cat > "$PD/docs/changes/active/0053-prosemention.md" <<'EOF'
---
id: 53
slug: prosemention
title: Change whose body merely mentions the marker
status: proposed
priority: medium
depends_on: []
trivial: true
---

## What changes

Append a dated `## Publish deferred` section when the publish is deferred.
EOF
git -C "$PD" add docs/changes; git_quiet -C "$PD" commit -m "pd fixtures"
pdout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$PD/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "publish-deferred fires for an ARCHIVED change carrying the marker (id 50)" \
  'has_finding "$pdout" publish-deferred 50'
# Isolate the id-50 line into a variable FIRST, then match it. Never `producer | grep -q` under
# `set -o pipefail` (AGENTS.md): grep exits early, the producer takes SIGPIPE, and the 141 shows
# up as an intermittent failure. `grep -c`/`grep` without -q are safe producers to pipe from.
pd50="$(grep -E "$(printf '^publish-deferred\t50\t')" <<<"$pdout")"
assert "publish-deferred message names the integration branch" \
  'grep -qF -- "main" <<<"$pd50"'
assert "publish-deferred message says the record is on the metadata branch only" \
  'grep -qF -- "docket" <<<"$pd50"'
assert "publish-deferred silent for an archived change with no marker (id 51)" \
  '! has_finding "$pdout" publish-deferred 51'
assert "publish-deferred fires for an ACTIVE change carrying the marker (id 52, no directory gate)" \
  'has_finding "$pdout" publish-deferred 52'
assert "publish-deferred silent for a PROSE MENTION of the marker (id 53)" \
  '! has_finding "$pdout" publish-deferred 53'
# Exactly one finding per marked change — not one per line of the section.
assert "publish-deferred emits exactly ONE finding for id 50" \
  '[ "$(grep -cE "$(printf "^publish-deferred\t50\t")" <<<"$pdout")" -eq 1 ]'
# The marker must NOT suppress board-row-dropped, and must not itself drop a row: id 52 is a
# legal active change, so it renders and no board-row-dropped fires for it.
assert "a marked ACTIVE change does not trip board-row-dropped (id 52)" \
  '! has_finding "$pdout" board-row-dropped 52'
# warn-only: findings alone never change the exit status
assert "board-checks still exits 0 with publish-deferred findings (warn-only)" \
  'NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$PD/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1'

# --- registration: the check-id is documented everywhere it must be (correspondence guard) ------
# Derived by grep from the emitting script, never hand-listed: every check-id board-checks.sh can
# EMIT must appear in the script's own header set, in board-checks.md, and in docket-status.md's
# closed enumeration. Anchored on the emitting code so a new check-id added without registering
# reddens here (change 0104's three-mirror drift; tracked structurally as change 0111).
BCSH="$REPO/scripts/board-checks.sh"; BCMD="$REPO/scripts/board-checks.md"; DSMD="$REPO/scripts/docket-status.md"
emitted="$(grep -oE '^[[:space:]]*emit [a-z-]+' "$BCSH" | awk '{print $2}' | sort -u)"
assert "emitted check-id set is non-empty (the grep itself is not vacuous)" \
  '[ "$(printf "%s\n" "$emitted" | grep -c .)" -ge 8 ]'
assert "publish-deferred is among the emitted check-ids" \
  'printf "%s\n" "$emitted" | grep -qxF "publish-deferred"'
reg_ok=1
for c in $emitted; do
  grep -qF -- "$c" "$BCSH" || { echo "check-id $c missing from board-checks.sh header" >&2; reg_ok=0; }
  grep -qF -- "$c" "$BCMD" || { echo "check-id $c missing from board-checks.md" >&2; reg_ok=0; }
  grep -qF -- "$c" "$DSMD" || { echo "check-id $c missing from docket-status.md" >&2; reg_ok=0; }
done
assert "every EMITTED check-id is registered in all three documentation surfaces" '[ "$reg_ok" -eq 1 ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_checks.sh 2>&1 | grep -E "publish-deferred|registered|NOT OK"`
Expected: `NOT OK - publish-deferred fires for an ARCHIVED change carrying the marker (id 50)` and the other new asserts, because no `publish-deferred` check exists yet.

- [ ] **Step 3: Add the `publish_deferred()` helper**

In `scripts/lib/docket-frontmatter.sh`, immediately after the `finalize_blocked()` function (which ends at line 112), add:

```bash
publish_deferred(){ # publish_deferred FILE  (meaningful on any change file, active or archived)
  # `## Publish deferred` is presence-encoded state written by mark-publish-deferred.sh when a
  # terminal close-out's publish step was EXPECTED but deferred or blocked (change 0083). Unlike
  # finalize_blocked(), this has NO status gate: the marker is written on the ARCHIVED file, at
  # which point the change is terminal, so gating on a lifecycle status would make it unreadable
  # exactly where it is written. Presence is the whole state.
  has_section "$1" "## Publish deferred"
}
```

- [ ] **Step 4: Add the check to `board-checks.sh`**

First register the id in the header set. Replace lines 12-13:

```bash
#     check-id ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain,
#                 stale-in-progress, merge-gate-stall, stale-finalize-blocked, merged-orphan,
#                 unknown-commit-ref, malformed-id}
```

with:

```bash
#     check-id ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain,
#                 publish-deferred, stale-in-progress, merge-gate-stall, stale-finalize-blocked,
#                 merged-orphan, unknown-commit-ref, malformed-id}
```

Then add the check inside the per-file `FILES` walk, immediately after the `stale-finalize-blocked`
block (which ends at line 270, just before the loop's closing `done`):

```bash
  # --- publish-deferred: the change carries the `## Publish deferred` marker (change 0083).
  # A terminal close-out's publish step was EXPECTED (terminal_publish: true, docket-mode) but
  # deferred or blocked, so the archived record never reached the integration branch. Before this
  # check, board-checks.sh had NO terminal-record check at all and certified exactly this gap
  # clean for eight days (#0043).
  #
  # NO status gate and NO directory gate, both deliberate: the marker is written on the ARCHIVED
  # file (terminal status), and a status gate would make it unreadable where it is written. The
  # marker's PRESENCE is the entire state — mark-publish-deferred.sh writes it only on the defer
  # path and terminal-publish.sh removes it on success, so a marker in the tree always means a
  # pending deferral. An `active/` file carrying one (a close-out interrupted before archiving)
  # reports the same way, harmlessly.
  #
  # Reads the marker in the change file — NOT a `git cat-file -e origin/<integration>:<path>`
  # set-diff. That would reintroduce the detector this change deliberately declined (spec §1a),
  # fire forever under `terminal_publish: false`, and break the script's git-only/offline
  # invariant. This check neither marks EXPLAINED nor feeds board-row-dropped: a body section
  # cannot drop a board row.
  if publish_deferred "$f"; then
    emit publish-deferred "$cid" "terminal-publish to $INTEGRATION_BRANCH not completed — record on $METADATA_BRANCH only; complete the publish or record a decision not to"
  fi
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -25`
Expected: all `publish-deferred` asserts `ok`; the registration asserts still `NOT OK` (the docs are not updated yet). That is the expected intermediate state — Step 6 closes it.

- [ ] **Step 6: Register the check-id in both contracts**

In `scripts/board-checks.md`, add this entry immediately after the `stale-finalize-blocked`
paragraph (which ends `…not a config knob.`):

```markdown
**`publish-deferred`** — The change carries the `## Publish deferred` body section
(`publish_deferred`), written by `mark-publish-deferred.sh` when a terminal close-out's publish
step was **expected** (`terminal_publish: true`, docket-mode) but consciously deferred or blocked.
The finding names the integration branch the record never reached and the metadata branch it is
still confined to. **No status gate and no directory gate:** the marker is written on the
*archived* file, so gating on a lifecycle status would make it unreadable exactly where it is
written; presence is the entire state, and `terminal-publish.sh` removes the marker on a
successful publish (so a marker in the tree always means a pending deferral). It reads the marker
in the change file, **never** a `git cat-file -e origin/<integration>:<path>` set-diff — a
branch-set diff would reintroduce the standing detector change 0083 deliberately declined, fire
forever under `terminal_publish: false`, and break this script's git-only/offline invariant.
Warn-only; it never mutates the change file.
```

In `scripts/docket-status.md:344`, extend the closed enumeration — replace:

```
`<check-id>` ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain, stale-in-progress, stale-finalize-blocked, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}.
```

with:

```
`<check-id>` ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain, publish-deferred, stale-in-progress, stale-finalize-blocked, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}.
```

- [ ] **Step 7: Run the test to verify everything passes**

Run: `bash tests/test_board_checks.sh 2>&1 | tail -20`
Expected: final line `PASS`, exit 0 — including `every EMITTED check-id is registered in all three documentation surfaces`.

- [ ] **Step 8: Mutation-test the check and the registration guard**

Apply each mutation, run the test, confirm a **NOT OK**, revert:

1. Comment out the `emit publish-deferred …` line — expect `publish-deferred fires for an ARCHIVED change carrying the marker (id 50)` to redden.
2. Change `publish_deferred()`'s `has_section "$1" "## Publish deferred"` to a substring `grep -qF` — expect `publish-deferred silent for a PROSE MENTION of the marker (id 53)` to redden.
3. Remove `publish-deferred` from `scripts/docket-status.md`'s enumeration — expect `every EMITTED check-id is registered in all three documentation surfaces` to redden. **This is the guard that would have caught change 0098's unregistered check-id**; prove it fires.
4. Change the registration loop's corpus grep so `emitted` comes back empty (e.g. grep for `emitx`) — expect `emitted check-id set is non-empty (the grep itself is not vacuous)` to redden. This proves the loop cannot pass vacuously.

Record the mutations and their results in the results file.

- [ ] **Step 9: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/board-checks.sh scripts/board-checks.md scripts/docket-status.md tests/test_board_checks.sh
git commit -m "feat(0083): publish-deferred health check — stop board-checks certifying a pending deferral clean"
```

---

### Task 3: `terminal-publish.sh` removes the marker on a successful publish

**Files:**
- Modify: `scripts/terminal-publish.sh` (change mode only, after `change_path` resolves at line 127, before the copy-set is built)
- Modify: `scripts/terminal-publish.md`
- Test: `tests/test_terminal_publish.sh`

**Interfaces:**
- Consumes: `mark-publish-deferred.sh --mode remove --change-file PATH` (Task 1).
- Produces: no new exported symbol. One new **optional** flag `--metadata-worktree PATH`; when omitted the script resolves it via `lib/docket-root.sh`'s `docket_main_worktree` + `/.docket`.

**Ordering rationale (load-bearing — do not "simplify" it):** the removal happens on the **metadata branch, BEFORE the copy-set is built**, not after the publish lands. `terminal-publish.sh` copies the change file *from `origin/<metadata-branch>`*, so removing afterwards would publish a copy that still carries a "publish not completed" marker onto the integration branch, and nothing would ever correct it there. Removing first, then re-fetching, makes both branches agree.

**Accepted, documented trade-off:** if the publish then *fails*, the marker has already been cleared. The script exits non-zero, and the driver's defer path re-marks (Task 4's close-out rule) — which is exactly what `--mode add`'s replace semantics are for. A rollback-on-failure re-add inside the script was considered and declined: it puts a failure path inside a failure path for a window the documented re-mark already covers.

- [ ] **Step 1: Write the failing test**

Replace the final two lines of `tests/test_terminal_publish.sh` (`if [ "$fail" = 0 ] …` / `exit "$fail"`) with the following block, then re-add those two lines at the end:

```bash
# --- change 0083: remove the `## Publish deferred` marker on a successful publish ---------------
# Needs a real repo (the arg-guard tests above do not): the removal writes and pushes on the
# metadata branch, which is exactly the state a hermetic fixture must construct rather than mock
# (metadata-branch-invisible-to-suite). Local bare origin, two branches, no network, no gh.
MARKER='## Publish deferred'
git_quiet(){ git "$@" >/dev/null 2>&1; }

# tp_repo: prints "<work> <origin>" — bare origin holding main + docket; docket carries an
# archived change file (id 60) that CARRIES the marker, plus its spec.
tp_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git_quiet -C "$work" checkout -b main
  mkdir -p "$work/docs/adrs"; echo "# adr index" > "$work/docs/adrs/README.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main baseline"; git_quiet -C "$work" push -u origin main
  git_quiet -C "$work" checkout --orphan docket
  git_quiet -C "$work" rm -rf .
  mkdir -p "$work/docs/changes/archive" "$work/docs/superpowers/specs" "$work/docs/adrs"
  echo "# spec" > "$work/docs/superpowers/specs/2026-07-08-sample.md"
  cat > "$work/docs/changes/archive/2026-07-08-0060-sample.md" <<'CF'
---
id: 60
slug: sample
title: Archived change whose publish was deferred
status: killed
priority: medium
spec: docs/superpowers/specs/2026-07-08-sample.md
adrs: []
---

## Why killed

Obsolete.

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**deferred** — pending human approval
CF
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"; git_quiet -C "$work" push -u origin docket
  printf '%s %s\n' "$work" "$origin"
}

read -r TW TO < <(tp_repo)
tp_args=(--id 60 --outcome killed --integration-branch main --metadata-branch docket
         --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$TW")

# (a) suppression carve-outs write and remove NOTHING — the marker must survive untouched.
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled false >/dev/null 2>&1 )
assert "publish suppressed (--enabled false) leaves the marker in place" \
  'grep -qxF -- "$MARKER" "$TW/docs/changes/archive/2026-07-08-0060-sample.md"'

( cd "$TW" && bash "$SCRIPT" --id 60 --outcome killed --integration-branch main --metadata-branch main \
    --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$TW" --enabled true >/dev/null 2>&1 )
assert "main-mode publish leaves the marker in place" \
  'grep -qxF -- "$MARKER" "$TW/docs/changes/archive/2026-07-08-0060-sample.md"'

# (b) a real, enabled publish removes the marker on the METADATA branch and pushes it...
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled true >/dev/null 2>&1 ); tprc=$?
assert "enabled publish exits zero" '[ "$tprc" -eq 0 ]'
git_quiet -C "$TW" fetch origin docket
meta_body="$(git -C "$TW" show origin/docket:docs/changes/archive/2026-07-08-0060-sample.md 2>/dev/null)"
assert "successful publish removed the marker on origin/docket" \
  '! grep -qxF -- "$MARKER" <<<"$meta_body"'
assert "marker removal preserved the rest of the archived record" \
  'grep -qxF -- "## Why killed" <<<"$meta_body"'

# ...and the copy that landed on the INTEGRATION branch is marker-free too (the ordering property:
# the removal must precede the copy-set build, or main receives a stale "not completed" marker).
git_quiet -C "$TW" fetch origin main
int_body="$(git -C "$TW" show origin/main:docs/changes/archive/2026-07-08-0060-sample.md 2>/dev/null)"
assert "the published record landed on origin/main"        '[ -n "$int_body" ]'
assert "the published record carries NO stale marker"      '! grep -qxF -- "$MARKER" <<<"$int_body"'

# (c) idempotent re-run: a second publish on an already-clean record is a no-op that still exits 0.
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled true >/dev/null 2>&1 ); tprc2=$?
assert "re-publish with no marker exits zero (idempotent)" '[ "$tprc2" -eq 0 ]'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_terminal_publish.sh 2>&1 | grep -E "NOT OK|PASS|FAIL"`
Expected: `NOT OK - successful publish removed the marker on origin/docket` (and the `origin/main` marker assert), because no removal exists yet. The suppression asserts (a) pass vacuously at this stage — that is fine; Step 5's mutation testing proves they are not decoration.

- [ ] **Step 3: Add the `--metadata-worktree` flag and source the root helper**

In `scripts/terminal-publish.sh`, after the existing `. "$(dirname "$0")/lib/docket-frontmatter.sh"` line (line 28), add:

```bash
. "$(dirname "$0")/lib/docket-root.sh"
```

Extend the variable initialization (line 31) to declare the new flag:

```bash
META_WORKTREE=""
```

Add the parse arm inside the `while` loop, after the `--enabled` arm (line 52):

```bash
    --metadata-worktree) META_WORKTREE="$2"; shift ;;
```

And document it in the header usage block, after the `[--enabled true|false]` line (line 16):

```bash
# --metadata-worktree PATH points at the metadata working tree (docket-mode: <repo>/.docket) whose
# archived change file carries any `## Publish deferred` marker to clear. Optional: when omitted it
# is resolved from lib/docket-root.sh's main-worktree anchor + "/.docket", never from the caller's
# CWD. Change mode only; ignored in --adr mode (an ADR has no change file to mark).
```

- [ ] **Step 4: Add the removal, before the copy-set is built**

In `scripts/terminal-publish.sh`'s change branch, insert this block immediately after
`[ -n "$change_path" ] || die "no archived change file for id $ID on $metaref"` (line 127) and
**before** `$GIT show "$metaref:$change_path" > "$tmpd/change.md" …` (line 128):

```bash
  # --- change 0083: clear a `## Publish deferred` marker BEFORE the copy-set is read ------------
  # Ordering is load-bearing. The copy-set is read FROM $metaref, so a removal done after the push
  # would publish a record still carrying a "publish not completed" marker onto the integration
  # branch, with nothing to correct it there. Removing on the metadata branch first — then
  # re-fetching — makes both branches agree.
  #
  # Reached only past BOTH no-op guards above, so a suppressed publish (`--enabled false`) and
  # main-mode never clear a marker: a suppressed publish is legitimate success, not a completed
  # deferral, and the marker must survive it.
  #
  # If the publish below then FAILS, the marker has already been cleared; the script exits
  # non-zero and the driver's defer path re-marks (mark-publish-deferred.sh's `add` replaces
  # rather than appends, so the re-mark is clean). A rollback re-add here was declined: it puts a
  # failure path inside a failure path for a window the documented re-mark already covers.
  [ -n "$META_WORKTREE" ] || META_WORKTREE="$(docket_main_worktree)/.docket"
  mark_file="$META_WORKTREE/$change_path"
  if [ -f "$mark_file" ] && grep -qxF -- '## Publish deferred' "$mark_file"; then
    log "clearing the ## Publish deferred marker on $META_BRANCH before publishing"
    "$(dirname "$0")/mark-publish-deferred.sh" --mode remove --change-file "$mark_file" \
      || { teardown_tmp; die "could not clear the publish-deferred marker"; }
    $GIT -C "$META_WORKTREE" add -- "$change_path" \
      || { teardown_tmp; die "git add failed for the marker removal"; }
    $GIT -C "$META_WORKTREE" commit -q -m "docket($pad): clear publish-deferred marker (publish completing)" -- "$change_path" \
      || { teardown_tmp; die "commit failed for the marker removal"; }
    # CAS: a concurrent metadata writer must not lose this removal. Bounded retry against a fresh
    # origin tip; the removal is idempotent, so a replay after a rebase is safe.
    mark_tries=0
    until $GIT -C "$META_WORKTREE" push "$REMOTE" "HEAD:$META_BRANCH" >/dev/null 2>&1; do
      mark_tries=$(( mark_tries + 1 ))
      [ "$mark_tries" -le 5 ] || { teardown_tmp; die "could not push the marker removal after $mark_tries attempts"; }
      $GIT -C "$META_WORKTREE" pull --rebase "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 \
        || { teardown_tmp; die "rebase failed while pushing the marker removal"; }
    done
    # Re-fetch so the copy-set below is built from the marker-free tip.
    $GIT fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || { teardown_tmp; die "re-fetch after marker removal failed"; }
  fi
```

This block references `teardown_tmp` and `$pad`, neither of which exists at that point in the current
file. Add the helper immediately after the `trap 'rm -rf "$tmpd"' EXIT` line (line 102):

```bash
# Early-failure teardown: at this point only $tmpd exists (the pub worktree is provisioned later),
# and the EXIT trap already removes it — this named no-op keeps the failure paths above readable
# and symmetrical with the later teardown(). Deliberately not `teardown`, which tears down a pub
# worktree that does not exist yet.
teardown_tmp(){ :; }
```

`$pad` is already set at line 121 (`pad="$(printf '%04d' "$ID")"`), which precedes the insertion
point — no change needed.

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_terminal_publish.sh 2>&1 | tail -20`
Expected: final line `PASS`, exit 0 — including `successful publish removed the marker on origin/docket` and `the published record carries NO stale marker`.

- [ ] **Step 6: Mutation-test the ordering and the suppression gates**

Apply each mutation, run the test, confirm a **NOT OK**, revert:

1. Move the whole removal block to *after* the post-push postcondition assert (just before the final `teardown`) — expect `the published record carries NO stale marker` to redden while the `origin/docket` assert still passes. **This is the ordering property**; prove it is real rather than asserted.
2. Move the removal block *above* the `--enabled` knob guard (line 92) — expect `publish suppressed (--enabled false) leaves the marker in place` to redden. This proves the suppression assert is not vacuous.
3. Move the removal block *above* the mode guard (line 84) — expect `main-mode publish leaves the marker in place` to redden.
4. Delete the `grep -qxF -- '## Publish deferred' "$mark_file"` condition so the block runs unconditionally — expect `re-publish with no marker exits zero (idempotent)` to still pass but the commit to fail on an empty diff; if it does not redden, note it and tighten the assert.

Record the mutations and their results in the results file.

- [ ] **Step 7: Update the contract**

In `scripts/terminal-publish.md`, add a `--metadata-worktree` row to the flags table and a
subsection under *Behavior*:

```markdown
### Clearing the `## Publish deferred` marker (change 0083)

In **change mode**, past both no-op guards, the script clears any `## Publish deferred` marker on
the archived change file in the metadata working tree — invoking `mark-publish-deferred.sh --mode
remove`, committing change-file-only on `metadata_branch`, CAS-pushing (bounded retry, 5
attempts), and re-fetching so the copy-set is read from the marker-free tip.

**The ordering is load-bearing.** The copy-set is read *from* `origin/<metadata-branch>`, so a
removal done after the push would publish a record still carrying a "publish not completed"
marker onto the integration branch, with nothing there to correct it. Removing first makes both
branches agree.

**Never under suppression.** The block sits past the mode guard and the `--enabled` knob guard, so
`--enabled false` and `main`-mode clear nothing: a suppressed publish is legitimate *success*, not
a completed deferral, and the marker must survive it.

**On a later publish failure** the marker has already been cleared; the script exits non-zero and
the driver's defer path re-marks (`--mode add` replaces rather than appends). A rollback re-add
inside this script was considered and declined — a failure path inside a failure path, for a
window the documented re-mark already covers.

`--metadata-worktree PATH` locates that working tree; when omitted it resolves from
`lib/docket-root.sh`'s main-worktree anchor plus `/.docket`, never from the caller's CWD. Ignored
in `--adr` mode: an ADR has no change file to mark (see change 0083's spec §5 — deferred
ADR-publish visibility is deliberately a follow-on, tracked as change #0117).
```

- [ ] **Step 8: Commit**

```bash
git add scripts/terminal-publish.sh scripts/terminal-publish.md tests/test_terminal_publish.sh
git commit -m "feat(0083): terminal-publish clears the publish-deferred marker before publishing"
```

---

### Task 4: register the marker in the convention and the close-out sequence

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (*Change body sections* list, after the `## Auto-groom blocked` bullet)
- Modify: `skills/docket-convention/references/terminal-close-out.md` (step 3)
- Test: `tests/test_docket_metadata_branch.sh` (sentinel asserts)

**Interfaces:**
- Consumes: the marker name and lifecycle from Tasks 1–3.
- Produces: the documented close-out rule the autonomous drivers follow. No code.

- [ ] **Step 1: Write the failing sentinel test**

Append to `tests/test_docket_metadata_branch.sh`, before its final summary/exit lines:

```bash
# --- change 0083: the `## Publish deferred` marker is registered in the convention -------------
CONV="$REPO/skills/docket-convention/SKILL.md"
TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
conv_lines="$(grep -F -- "## Publish deferred" "$CONV")"
tco_lines="$(grep -iE "publish deferred|marker" "$TCO")"
assert "convention's body-section list documents ## Publish deferred" \
  '[ -n "$conv_lines" ]'
assert "convention names the marker's REMOVAL on a successful publish" \
  'grep -qiE "remov|clear" <<<"$conv_lines"'
assert "close-out step 3 documents the write-marker-on-defer rule" \
  'grep -qF -- "## Publish deferred" "$TCO"'
assert "close-out states the marker is NEVER written under suppression" \
  'grep -qiE "never|not written|suppress" <<<"$tco_lines"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_metadata_branch.sh 2>&1 | grep -E "Publish deferred|NOT OK"`
Expected: all four new asserts print `NOT OK`.

- [ ] **Step 3: Add the body-section entry to the convention**

In `skills/docket-convention/SKILL.md`, insert this bullet in the *Change body sections* list,
immediately after the `## Auto-groom blocked` bullet and before `## Finalize blocked`:

```markdown
- `## Publish deferred` — dated record appended by `mark-publish-deferred.sh` (change 0083) when a
  terminal close-out's publish step was **expected** (`terminal_publish: true`, docket-mode) but
  consciously deferred or blocked, so the archived record never reached the integration branch.
  **Presence-encoded state:** `board-checks.sh`'s `publish-deferred` check surfaces it as a
  finding, and `terminal-publish.sh` **removes it automatically** on a successful publish — so a
  backfill self-heals the marker for free. Never written when the publish is legitimately
  suppressed (`terminal_publish: false`, or `main`-mode), where a skipped publish is success
  rather than a deferral. Written and removed by the script; never hand-authored.
```

- [ ] **Step 4: Add the defer rule to close-out step 3**

In `skills/docket-convention/references/terminal-close-out.md`, append to the end of step 3
(after the paragraph ending `…no caller branches on the knob itself.`):

```markdown
   **When the publish is expected but does NOT complete — mark it (change 0083).** If
   `terminal_publish` is `true` and this is docket-mode, but the publish is consciously deferred
   (a human gate) or blocked (a wall the run cannot pass, e.g. a protected-branch push denial),
   the driver appends the durable marker before reporting:

   ```
   "${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh mark-publish-deferred --mode add \
     --change-file .docket/<changes_dir>/archive/<UTC-date>-<id>-<slug>.md \
     --reason <deferred|blocked> --detail "<short single-line why>" \
     --date <UTC-date> --integration-branch <integration_branch> --id <id>
   ```

   Commit and push it on `metadata_branch` like any other metadata write. Autonomous callers
   still abort-and-report — the marker makes that abort **durable and self-describing** instead
   of living only in a chat thread, which is precisely how #0043's record went missing for eight
   days with every health check reporting clean. `mark-publish-deferred.sh` **replaces** an
   existing section rather than appending a second, so re-marking is safe.

   **Never mark under suppression.** When `terminal_publish` is `false`, or in `main`-mode, the
   publish is legitimately a no-op that exits 0 — that is *success*, not a deferral, and no
   marker is written. **Never mark on a successful publish**, and never remove the marker by
   hand: `terminal-publish.sh` clears it itself on the success path, so the state stays
   presence-encoded.
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_docket_metadata_branch.sh 2>&1 | tail -10`
Expected: final line `PASS`, exit 0.

- [ ] **Step 6: Run the skill size-budget guard**

Run: `bash tests/test_skill_size_budgets.sh`
Expected: `PASS`. `docket-convention/SKILL.md` grew by one bullet; if the budget guard reddens,
report it rather than trimming unrelated prose to fit — the budget is a direction, not a licence
to delete another change's content (`size-target-is-direction`).

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/references/terminal-close-out.md tests/test_docket_metadata_branch.sh
git commit -m "docs(0083): register the Publish deferred marker in the convention and close-out"
```

---

## Final gate

- [ ] **Run the WHOLE suite, not only the tests this plan enumerated** (AGENTS.md):

```bash
rc=0; for t in tests/test_*.sh; do out="$(bash "$t" 2>&1)"; n="$(printf '%s\n' "$out" | grep -c '^NOT OK')"; if [ "$n" -gt 0 ]; then echo "FAIL $(basename "$t") ($n)"; rc=1; fi; done; echo "SUITE rc=$rc"
```

Expected: `SUITE rc=0`. This is one foreground run; it takes several minutes. Do not background it.

- [ ] **Verify against the live repo** (`metadata-branch-invisible-to-suite`): the hermetic suite
  cannot see this repo's real `docket` branch, so also run the check over the real tree and confirm
  zero false positives, then prove the path is not a swallowed no-op:

```bash
bash scripts/board-checks.sh --changes-dir "$(git rev-parse --show-toplevel)/.docket/docs/changes" \
  --metadata-branch docket --integration-branch main | grep publish-deferred || echo "no publish-deferred findings (expected — no deferral pending)"
```

Then copy one archived change file to a throwaway directory, append a `## Publish deferred`
section by running `mark-publish-deferred.sh --mode add` against the copy, re-run `board-checks.sh`
against that throwaway changes dir, and confirm the finding fires. Record both in the results file.
