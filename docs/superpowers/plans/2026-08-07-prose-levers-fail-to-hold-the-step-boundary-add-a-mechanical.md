<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0237 — Prose levers fail to hold the step boundary — give the disposition contract a consumer](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0237-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md)**
<!-- docket:backlink:end -->

# A mechanical consumer for the terminal-disposition contract — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give docket's four-value terminal-disposition contract a real consumer — a git-only `verify-run` reader that evaluates Step 7's postcondition, called from `runner-dispatch.sh` after the child harness returns, with one bounded re-dispatch and a git-written `## Run halted` escape.

**Architecture:** One new pure-reader script (`scripts/verify-run.sh`) owns both the verdict and the in-progress-id snapshot, so config resolution and frontmatter reading live in exactly one place. `scripts/runner-dispatch.sh` stops `exec`ing its adapter and becomes a call-and-return that, **for `implement-next` delegations only**, diffs the in-progress set across the hand-off, verifies each newly-claimed change, re-dispatches an unfinished run exactly once, and aborts loudly on the second strike. A new presence-encoded `## Run halted` body section makes a `halted` disposition a git act rather than a self-report.

**Tech Stack:** Bash 4+ (`set -uo pipefail`), the shared `scripts/lib/docket-frontmatter.sh` accessors, `scripts/lib/docket-root.sh`, plain `git` plumbing, the repo's own `tests/test_*.sh` harness (`assert "name" 'expr'`), run via `scripts/run-tests.sh`.

## Global Constraints

- **Shell portability** — `set -uo pipefail`; no GNU-only flags; no ERE interval bounds above 255 (BSD `grep` rejects them); no producer piped into an early-exiting consumer.
- **Anchored frontmatter reads** — every optional key (`pr:`, `branch:`, `status:` is always present but read the same way) uses `fm_field`, **never** `field`. One absent-key fixture and one mutation arm **per read** (learnings: `frontmatter-anchored-read`).
- **Exit codes** — `verify-run.sh` exits **0 whenever a verdict was produced**. `run-incomplete` is a finding, not a script failure (learnings: `exit-code-encodes-a-non-failure`). Non-zero only when the check itself could not run.
- **Report-line contract** — callers key on the **stdout report line**, never on the exit code, matching the Board pass house pattern.
- **No status writes** — `verify-run.sh` flips no status, releases no claim, writes no file, and shells no `gh`. `runner-dispatch.sh` acts only by running an agent.
- **Exit-code preservation** — with `exec` gone, `runner-dispatch.sh` must propagate the adapter's exit code verbatim on every path where the gate takes no action.
- **Agent gating is load-bearing** — the gate engages **only** for `--agent implement-next`. A `build-*` delegation leaves its change `in-progress` by design.
- **`board-checks.sh` is untouched** — no leg, floor, or predicate changes.
- **Producer coverage** — `## Run halted` must have a step that *writes* it, and an assert anchored on that writing paragraph (learnings: `specified-but-unreachable`).
- Suite command: `scripts/run-tests.sh`. Single-file run: `bash tests/test_verify_run.sh`.

---

## File Structure

| File | Responsibility |
|---|---|
| `scripts/verify-run.sh` **(create)** | The consumer. Two modes: one-change verdict, and the in-progress-id snapshot. Config resolution + frontmatter reads live here only. |
| `scripts/verify-run.md` **(create)** | Its co-located contract (Purpose / Usage / Behavior / Exit codes / Invariants). |
| `scripts/docket.sh` **(modify)** | `WRAPPED_OPS` gains `verify-run`. |
| `scripts/docket.md` **(modify)** | The operations table gains the matching row. The facade sentinel greps both, so they land together. |
| `scripts/runner-dispatch.sh` **(modify)** | `exec` → call-and-return; snapshot diff; agent gate; one bounded re-dispatch; loud two-strikes abort; exit-code preservation. |
| `scripts/runner-dispatch.md` **(modify)** | Contract updated to match. |
| `scripts/board-checks.md` **(modify)** | One pointer sentence in the `aborted-run` residual prose (see the change's reconcile log: there is **no `## Not covered` heading** in this file). |
| `skills/docket-convention/SKILL.md` **(modify)** | `## Run halted` added to the *Change body sections* list. |
| `skills/docket-implement-next/SKILL.md` **(modify)** | The `## Run halted` **producer** (the halted paths write it) and its **removal rule** (Step 2 claim). |
| `tests/test_verify_run.sh` **(create)** | Verdicts, anchored reads, exit codes, snapshot mode, and the prose sentinels for the producer/removal rules. |
| `tests/test_runner_dispatch.sh` **(modify)** | Call-and-return, exit-code preservation, snapshot diff, agent gate, bounded re-dispatch, two-strikes abort. |

---

### Task 1: `verify-run.sh` — verdicts and the snapshot

**Files:**
- Create: `scripts/verify-run.sh`
- Test: `tests/test_verify_run.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` (`fm_field`, `has_section`, `int_field`).
- Produces:
  - `verify-run.sh <id> [--changes-dir DIR]` → one stdout line: `run-complete <id>` | `run-halted <id>` | `run-incomplete <id> <unmet…>` | `run-unclaimed <id>`; exit 0.
  - `verify-run.sh --in-progress-ids [--changes-dir DIR]` → the ids of every `status: in-progress` change in `active/`, one per line, numerically sorted; exit 0.
  - Unmet-conjunct tokens, in this fixed order: `status`, `pr`, `branch`.
  - Mock seams: `GIT`, `CONFIG_EXPORT_CMD`.

- [ ] **Step 1: Write the failing test**

Create `tests/test_verify_run.sh`:

```bash
#!/usr/bin/env bash
# tests/test_verify_run.sh — run: bash tests/test_verify_run.sh
# Hermetic: every case builds a sandbox repo with its own changes dir and passes
# --changes-dir, so nothing reads the developer's real docket state.
set -uo pipefail
unset XDG_CONFIG_HOME
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VR="$ROOT/scripts/verify-run.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# --- fixture ------------------------------------------------------------------
make_sbx(){   # sets SBX (repo root) and CH (changes dir with active/ + archive/)
  SBX="$(mktemp -d)"; SBX="$(cd "$SBX" && pwd -P)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  ( cd "$SBX" && git commit --allow-empty -qm init )
  CH="$SBX/docs/changes"; mkdir -p "$CH/active" "$CH/archive"
}

# write_change ID STATUS BRANCH PR [BODY]
write_change(){
  local id="$1" status="$2" branch="$3" pr="$4" body="${5:-}"
  printf -v padded '%04d' "$id"
  cat > "$CH/active/$padded-slug$id.md" <<EOF
---
id: $id
slug: slug$id
title: "Change $id"
status: $status
branch: $branch
pr: $pr
---

## Why

body text
$body
EOF
}

# push_branch NAME — make refs/remotes/origin/NAME resolve in SBX
push_branch(){ git -C "$SBX" update-ref "refs/remotes/origin/$1" HEAD; }

vr(){ bash "$VR" "$@" --changes-dir "$CH" 2>/dev/null; }

# --- run-complete: all three conjuncts hold -----------------------------------
make_sbx
write_change 10 implemented feat/slug10 "https://github.com/o/r/pull/7"
push_branch feat/slug10
out="$( cd "$SBX" && vr 10 )"; rc=$?
assert "complete: verdict line is run-complete" '[ "$out" = "run-complete 10" ]'
assert "complete: exit 0" '[ "$rc" = "0" ]'

# --- run-incomplete: the 0235 signature (built, never delivered) --------------
write_change 11 in-progress feat/slug11 ""
out="$( cd "$SBX" && vr 11 )"; rc=$?
assert "incomplete: names the change and every unmet conjunct" \
  '[ "$out" = "run-incomplete 11 status pr branch" ]'
assert "incomplete: still exits 0 (a finding is not a script failure)" '[ "$rc" = "0" ]'

# --- run-incomplete: partial — pushed branch, PR recorded, status not advanced -
write_change 12 in-progress feat/slug12 "https://github.com/o/r/pull/8"
push_branch feat/slug12
out="$( cd "$SBX" && vr 12 )"
assert "incomplete: reports ONLY the unmet conjunct" '[ "$out" = "run-incomplete 12 status" ]'

# --- run-halted: the git-written escape ---------------------------------------
write_change 13 in-progress feat/slug13 "" "
## Run halted

### 2026-08-07 — halted

Design fundamentally invalidated; needs a human."
out="$( cd "$SBX" && vr 13 )"
assert "halted: presence of the section produces run-halted" '[ "$out" = "run-halted 13" ]'

# --- run-halted loses to a satisfied postcondition ----------------------------
# `## Run halted` is presence-encoded and its removal is owned by the Step 2 claim, which does
# NOT run on a resume — so a stale section can ride into a completed run. A satisfied
# postcondition is the stronger fact and must win.
write_change 14 implemented feat/slug14 "https://github.com/o/r/pull/9" "
## Run halted

### 2026-08-06 — halted

stale record from an earlier attempt."
push_branch feat/slug14
out="$( cd "$SBX" && vr 14 )"
assert "halted: a satisfied postcondition outranks a stale halt record" '[ "$out" = "run-complete 14" ]'

# --- a prose MENTION of the marker is not the section (whole-line match) -------
write_change 15 in-progress feat/slug15 "" "
Writing a \`## Run halted\` section is how a run clears the gate."
out="$( cd "$SBX" && vr 15 )"
assert "halted: a prose mention does not fire the section" \
  '! grep -q "^run-halted" <<<"$out"'

# --- run-unclaimed: no live run to verify -------------------------------------
write_change 16 proposed "" ""
out="$( cd "$SBX" && vr 16 )"
assert "unclaimed: a proposed change has no run to verify" '[ "$out" = "run-unclaimed 16" ]'
write_change 17 deferred "" ""
out="$( cd "$SBX" && vr 17 )"
assert "unclaimed: a deferred change too" '[ "$out" = "run-unclaimed 17" ]'
cat > "$CH/archive/2026-08-01-0018-slug18.md" <<'EOF'
---
id: 18
slug: slug18
status: done
---

## Why
EOF
out="$( cd "$SBX" && vr 18 )"
assert "unclaimed: an archived change too" '[ "$out" = "run-unclaimed 18" ]'

# --- ANCHORED READS: one absent-key fixture + one mutation arm per read -------
# frontmatter OMITS the key while the body opens a line with it. An unanchored read returns the
# prose; the anchored read returns empty. The natural fixture (key present) passes under BOTH
# implementations, so these absent-key fixtures are the whole guard.
printf -v p '%04d' 20
cat > "$CH/active/$p-slug20.md" <<'EOF'
---
id: 20
slug: slug20
status: in-progress
branch: feat/slug20
---

## Why

pr: https://example.test/not-a-real-field
EOF
push_branch feat/slug20
out="$( cd "$SBX" && vr 20 )"
assert "anchored pr: body prose opening 'pr:' is NOT read as the field" \
  '[ "$out" = "run-incomplete 20 status pr" ]'

printf -v p '%04d' 21
cat > "$CH/active/$p-slug21.md" <<'EOF'
---
id: 21
slug: slug21
status: in-progress
pr: https://github.com/o/r/pull/11
---

## Why

branch: feat/slug21
EOF
push_branch feat/slug21
out="$( cd "$SBX" && vr 21 )"
assert "anchored branch: body prose opening 'branch:' is NOT read as the field" \
  '[ "$out" = "run-incomplete 21 status branch" ]'

printf -v p '%04d' 22
cat > "$CH/active/$p-slug22.md" <<'EOF'
---
id: 22
slug: slug22
branch: feat/slug22
pr: https://github.com/o/r/pull/12
---

## Why

status: implemented
EOF
push_branch feat/slug22
out="$( cd "$SBX" && vr 22 )"
assert "anchored status: body prose opening 'status:' is NOT read as the field" \
  '[ "$out" = "run-unclaimed 22" ]'

# --- the branch conjunct is about ORIGIN, not a local ref ---------------------
write_change 23 implemented feat/slug23 "https://github.com/o/r/pull/13"
git -C "$SBX" branch feat/slug23 >/dev/null 2>&1
out="$( cd "$SBX" && vr 23 )"
assert "branch conjunct: a LOCAL branch does not satisfy 'delivered'" \
  '[ "$out" = "run-incomplete 23 branch" ]'

# --- snapshot mode ------------------------------------------------------------
make_sbx
write_change 30 in-progress feat/slug30 ""
write_change 31 proposed "" ""
write_change 32 in-progress feat/slug32 ""
ids="$( cd "$SBX" && vr --in-progress-ids )"
assert "snapshot: lists exactly the in-progress ids, numerically sorted" \
  '[ "$ids" = "$(printf "30\n32")" ]'
write_change 33 implemented feat/slug33 "https://github.com/o/r/pull/14"
ids="$( cd "$SBX" && vr --in-progress-ids )"
assert "snapshot: an implemented change is not in-progress" \
  '! grep -qx 33 <<<"$ids"'

# --- errors: the check could not run => non-zero, no verdict ------------------
out="$( cd "$SBX" && bash "$VR" 999 --changes-dir "$CH" 2>/dev/null )"; rc=$?
assert "missing id: non-zero" '[ "$rc" != "0" ]'
assert "missing id: emits no verdict line on stdout" '[ -z "$out" ]'
err="$( cd "$SBX" && bash "$VR" --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "no id and no mode: non-zero" '[ "$rc" != "0" ]'
assert "no id and no mode: diagnostic names the script" 'grep -qF "verify-run" <<<"$err"'
err="$( cd "$SBX" && bash "$VR" 10 --changes-dir "$SBX/nope" 2>&1 >/dev/null )"; rc=$?
assert "bad --changes-dir: non-zero with a diagnostic" '[ "$rc" != "0" ] && [ -n "$err" ]'
err="$( cd "$SBX" && bash "$VR" abc --changes-dir "$CH" 2>&1 >/dev/null )"; rc=$?
assert "non-numeric id is rejected up front" '[ "$rc" != "0" ]'
rm -rf "$SBX"

exit $fail
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_verify_run.sh`
Expected: every assert `NOT OK` (the script does not exist yet), non-zero exit.

- [ ] **Step 3: Write the implementation**

Create `scripts/verify-run.sh`:

```bash
#!/usr/bin/env bash
# scripts/verify-run.sh — the mechanical consumer of docket's terminal-disposition contract
# (change 0237), behind `docket.sh verify-run`. Evaluates docket-implement-next's **Step 7
# postcondition** for one change and reports a verdict on stdout; or, in --in-progress-ids mode,
# prints the ids of every in-progress change so a caller can diff the set across a hand-off.
#
# PURE READER. Git and filesystem only — no network, no `gh`, no harness, no status writes, no
# file writes, no claim release. The only thing that ACTS on a verdict is runner-dispatch.sh.
#
# Usage: verify-run.sh <id> [--changes-dir DIR]
#        verify-run.sh --in-progress-ids [--changes-dir DIR]
#   Verdict lines (one, on stdout):
#     run-complete <id>                    every conjunct holds
#     run-halted <id>                      a `## Run halted` record is present — deliberate stop
#     run-incomplete <id> <unmet…>         one or more conjuncts unmet (tokens: status pr branch)
#     run-unclaimed <id>                   not in-progress and not implemented — no run to verify
#   Exit 0 WHENEVER A VERDICT WAS PRODUCED. `run-incomplete` is a FINDING, not a script failure:
#   a bare non-zero consumer must not read it as one (LEARNINGS: exit-code-encodes-a-non-failure).
#   Non-zero (2) only when the check itself could not run — bad usage, unknown id, unreadable
#   change file, unresolvable config, not a repo.
#   NO TIME FLOOR. Sound only because of WHERE this is called: at a seam where the child process
#   has already returned, so "stopped" and "still working" are not ambiguous. board-checks.sh
#   cannot make that assumption and therefore keeps its floors — it is deliberately untouched.
#   Mock seams: GIT="${GIT:-git}", CONFIG_EXPORT_CMD (config resolution).
set -uo pipefail
GIT="${GIT:-git}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

die(){ printf 'verify-run: %s\n' "$*" >&2; exit 2; }

ID=""; CHANGES_DIR=""; MODE="verdict"
while [ $# -gt 0 ]; do
  case "$1" in
    --in-progress-ids) MODE="ids" ;;
    --changes-dir) CHANGES_DIR="${2:-}"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) die "unknown argument: $1" ;;
    *) [ -z "$ID" ] || die "unexpected extra argument: $1"; ID="$1" ;;
  esac
  shift
done

# --- changes dir: an explicit flag, else the resolver -------------------------
# Resolving here (rather than making every caller pass it) is what makes `docket.sh verify-run <id>`
# a usable hand command. The flag exists for hermetic tests and for runner-dispatch.sh, which has
# already resolved a repo root of its own.
if [ -z "$CHANGES_DIR" ]; then
  cfg="$(${CONFIG_EXPORT_CMD:-"${DOCKET_BASH_PATH:-bash}" "$SELF_DIR/docket-config.sh" --export})" \
    || die "config export failed"
  eval "$cfg"
  case "${BOOTSTRAP:-}" in
    PROCEED) : ;;
    STOP_MIGRATE)  die "repo not migrated — run migrate-to-docket.sh" ;;
    CREATE_ORPHAN) die "fresh repo — run docket.sh bootstrap to create the docket branch" ;;
    *) die "unknown bootstrap verdict '${BOOTSTRAP:-}'" ;;
  esac
  if [ "${DOCKET_MODE:-}" = "docket" ]; then
    CHANGES_DIR="${METADATA_WORKTREE:?}/${CHANGES_DIR_REL:-${CHANGES_DIR:-docs/changes}}"
  else
    CHANGES_DIR="${REPO_ROOT:?}/${CHANGES_DIR_REL:-${CHANGES_DIR:-docs/changes}}"
  fi
fi
[ -d "$CHANGES_DIR" ] || die "changes dir not found: $CHANGES_DIR"

# shellcheck source=/dev/null
source "$SELF_DIR/lib/docket-frontmatter.sh"

if [ "$MODE" = "ids" ]; then
  # The snapshot half. Numerically sorted so a caller's `comm`/diff is stable.
  for f in "$CHANGES_DIR"/active/*.md; do
    [ -f "$f" ] || continue
    [ "$(fm_field "$f" status)" = "in-progress" ] || continue
    id="$(int_field "$f" id)"; [ -n "$id" ] && printf '%s\n' "$id"
  done | sort -n
  exit 0
fi

[ -n "$ID" ] || die "an <id> is required (or --in-progress-ids)"
case "$ID" in ''|*[!0-9]*) die "invalid id: $ID (must be a non-negative integer)" ;; esac

# Locate the change: active/ first, then archive/. An archived change is a legitimate
# `run-unclaimed` (terminal — there is no run to verify); a change that exists NOWHERE is a caller
# error and must not be reported as a benign verdict.
printf -v padded '%04d' "$ID"
FILE=""
for cand in "$CHANGES_DIR/active/$padded-"*.md "$CHANGES_DIR"/archive/*-"$padded-"*.md; do
  [ -f "$cand" ] && { FILE="$cand"; break; }
done
[ -n "$FILE" ] || die "no change file for id $ID under $CHANGES_DIR"
[ -r "$FILE" ] || die "change file is unreadable: $FILE"

# EVERY read is fm_field, never field: `pr:` and `branch:` are optional keys, and this repo's
# change bodies routinely open lines with them (LEARNINGS: frontmatter-anchored-read). One
# absent-key fixture and one mutation arm per read live in tests/test_verify_run.sh.
status="$(fm_field "$FILE" status)"
pr="$(fm_field "$FILE" pr)"
branch="$(fm_field "$FILE" branch)"

case "$status" in
  in-progress|implemented) : ;;
  *) printf 'run-unclaimed %s\n' "$ID"; exit 0 ;;
esac

# --- Step 7's postcondition, as three conjuncts ------------------------------
unmet=()
[ "$status" = "implemented" ] || unmet+=(status)
[ -n "$pr" ]                  || unmet+=(pr)
if [ -z "$branch" ] || ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$branch"; then
  unmet+=(branch)
fi

# ORDER IS DELIBERATE: a satisfied postcondition outranks a `## Run halted` record. The section is
# presence-encoded state whose removal is owned by docket-implement-next's Step 2 claim — which
# does NOT run on a resume — so a stale record can ride into a genuinely completed run. Checking
# the conjuncts first means a stale marker can never downgrade a complete run
# (LEARNINGS: presence-encoded-state — enumerate the readers, then decide).
if [ "${#unmet[@]}" -eq 0 ]; then
  printf 'run-complete %s\n' "$ID"; exit 0
fi
if has_section "$FILE" "## Run halted"; then
  printf 'run-halted %s\n' "$ID"; exit 0
fi
printf 'run-incomplete %s %s\n' "$ID" "${unmet[*]}"
exit 0
```

Then make it executable:

```bash
chmod +x scripts/verify-run.sh
```

**Note on the config branch:** `docket-config.sh --export` emits `CHANGES_DIR` as a repo-relative
value. Read the real export before finalising the two lines above — run
`bash scripts/docket-config.sh --export | grep -E '^(CHANGES_DIR|METADATA_WORKTREE|REPO_ROOT|DOCKET_MODE)='`
and use the actual variable names it prints. If it emits `CHANGES_DIR` (not `CHANGES_DIR_REL`),
capture it into a local before the `eval` clobbers the flag value, i.e.:

```bash
  rel="${CHANGES_DIR:-docs/changes}"     # after eval, this is the resolver's repo-relative value
  if [ "${DOCKET_MODE:-}" = "docket" ]; then CHANGES_DIR="${METADATA_WORKTREE:?}/$rel"
  else CHANGES_DIR="${REPO_ROOT:?}/$rel"; fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_verify_run.sh`
Expected: every line `ok - …`, exit 0.

- [ ] **Step 5: Mutation-test the anchored reads — one arm per read**

For each of the three reads, temporarily swap `fm_field` → `field` **one at a time**, re-run the
suite, confirm the matching absent-key assert goes `NOT OK`, then restore:

```bash
cp scripts/verify-run.sh /tmp/verify-run.bak            # NEVER `git checkout --` to restore:
                                                        # it restores to HEAD, destroying the
                                                        # uncommitted work under test
sed -i.mut 's/pr="$(fm_field "$FILE" pr)"/pr="$(field "$FILE" pr)"/' scripts/verify-run.sh
bash tests/test_verify_run.sh | grep 'NOT OK'           # expect the anchored-pr assert
cp /tmp/verify-run.bak scripts/verify-run.sh
```

Repeat for `branch` and `status`. Expected each time: exactly the matching anchored assert fails.
Restore the file and re-run the suite green before committing.

- [ ] **Step 6: Commit**

```bash
git add scripts/verify-run.sh tests/test_verify_run.sh
git commit -m "feat(0237): add verify-run — the git-only consumer of the disposition contract"
```

---

### Task 2: Facade registration + the co-located contract

**Files:**
- Create: `scripts/verify-run.md`
- Modify: `scripts/docket.sh:97` (`WRAPPED_OPS`)
- Modify: `scripts/docket.md` (operations table, alphabetical position)
- Test: `tests/test_docket_facade.sh` (existing sentinel — no edit needed; it derives both sides by grep)

**Interfaces:**
- Consumes: Task 1's `scripts/verify-run.sh`.
- Produces: `docket.sh verify-run <id>` as a supported operation.

- [ ] **Step 1: Run the existing facade sentinel to see it pass before the change**

Run: `bash tests/test_docket_facade.sh`
Expected: all `ok -` (baseline; the op does not exist on either side yet).

- [ ] **Step 2: Register the op in `scripts/docket.sh`**

Append `verify-run` to the `WRAPPED_OPS` string (line 97). The dispatch `case` is **not** touched —
the generic `*` arm loops over `WRAPPED_OPS`, and `tests/test_docket_facade.sh` asserts the case
block contains only its six known labels, so a hand-added arm would redden.

```bash
WRAPPED_OPS="docket-status board-refresh archive-change terminal-publish cleanup-feature-branch github-mirror sync-integration-branch render-change-links render-artifact-backlink render-adr-index render-learnings-index adr-checks board-checks reclaim-claims mint-stub runner-dispatch mark-publish-deferred backfill-change-types verify-run"
```

- [ ] **Step 3: Run the sentinel to verify it FAILS**

Run: `bash tests/test_docket_facade.sh`
Expected: `NOT OK - docket.sh op set == docket.md documented op set` — the doc row is missing.

- [ ] **Step 4: Add the `scripts/docket.md` row**

In the operations table, in the same style as its neighbours:

```markdown
| `verify-run` | `verify-run.sh` | evaluate docket-implement-next's Step 7 postcondition for one change and report a verdict (change 0237) |
```

- [ ] **Step 5: Write the contract `scripts/verify-run.md`**

```markdown
# verify-run.sh — the terminal-disposition consumer

## Purpose

docket's terminal-disposition contract (`advanced` / `contended` / `drained` / `halted`) had a
producer and no consumer: `advanced` is claimable only when `docket-implement-next`'s **Step 7
postcondition** holds — a statement entirely readable from git that no code read. Six autonomous
runs executed a prefix of the seven steps and reported success. This script is the missing reader
(change 0237).

It is a **pure reader**: git and filesystem only. No network, no `gh`, no harness. It flips no
status, releases no claim, and writes no file. The only thing that acts on a verdict is
`runner-dispatch.sh`.

## Usage

```
docket.sh verify-run <id>
docket.sh verify-run --in-progress-ids
```

- `<id>` — the change id (integer; the file is located by its zero-padded name in `active/`, then
  `archive/`).
- `--in-progress-ids` — print the id of every `status: in-progress` change in `active/`, one per
  line, numerically sorted. This is the snapshot half `runner-dispatch.sh` diffs across a hand-off.
- `--changes-dir DIR` — bypass config resolution and read this directory. For hermetic tests and
  for a caller that has already resolved a repo root.

Mock seams: `GIT`, `CONFIG_EXPORT_CMD`.

## Behavior

Verdict mode evaluates three conjuncts, each read with the **anchored** `fm_field` (ADR-0057):

| Conjunct | Read | Token when unmet |
|---|---|---|
| status advanced | `status: implemented` | `status` |
| PR recorded | `pr:` non-empty | `pr` |
| branch delivered | `refs/remotes/origin/<branch:>` resolves | `branch` |

One report line on stdout — the same house pattern as the Board pass, where **callers key on the
line, never on the exit code**:

- `run-complete <id>` — every conjunct holds.
- `run-halted <id>` — a `## Run halted` record is present; the run ended deliberately.
- `run-incomplete <id> <unmet…>` — tokens in the fixed order `status pr branch`.
- `run-unclaimed <id>` — the change is neither `in-progress` nor `implemented`; there is no run to
  verify (`proposed` after a reclaim, `deferred`, or archived).

**Precedence.** The conjuncts are evaluated **before** the halt record, so a satisfied
postcondition outranks a stale `## Run halted`. The section's removal is owned by
`docket-implement-next`'s Step 2 claim, which does not run on a resume, so a stale record is
reachable; this ordering means it can never downgrade a complete run.

**No time floor.** This is the point of the script, and it is sound only because of where it is
called: at a seam where the child process has already returned, so "stopped" and "still working"
are not ambiguous. `board-checks.sh` cannot make that assumption and keeps its floors — it is
deliberately untouched by change 0237.

## Exit codes

- `0` — **whenever a verdict was produced**, including `run-incomplete`. A finding is not a script
  failure, and a bare non-zero consumer must not read it as one.
- `2` — the check could not run: bad usage, non-numeric or unknown id, unreadable change file,
  failed config export, non-`PROCEED` bootstrap verdict, missing changes dir.

## Invariants

- Never writes: no status flip, no claim release, no file write, no `gh`, no network.
- Every frontmatter read is `fm_field`, never `field` — `pr:` and `branch:` are optional keys and
  this repo's change bodies routinely open lines with them.
- A verdict is always exactly one line on stdout; diagnostics always go to stderr.
- `run-incomplete` never exits non-zero.
```

- [ ] **Step 6: Run the sentinel to verify it passes**

Run: `bash tests/test_docket_facade.sh`
Expected: all `ok -`, including the op-set equality assert.

- [ ] **Step 7: Verify the op actually routes**

Run: `bash scripts/docket.sh verify-run --help`
Expected: the script's header usage text, exit 0.

- [ ] **Step 8: Commit**

```bash
git add scripts/verify-run.md scripts/docket.sh scripts/docket.md
git commit -m "feat(0237): register verify-run on the facade and ship its contract"
```

---

### Task 3: `runner-dispatch.sh` — `exec` → call-and-return

**Files:**
- Modify: `scripts/runner-dispatch.sh:124` (the handoff)
- Test: `tests/test_runner_dispatch.sh` (append a new section)

**Interfaces:**
- Consumes: nothing new.
- Produces: the facade returns after the adapter and exits with `$rc`, the adapter's verbatim exit
  code. This is the seam Tasks 4 and 5 hang the gate on.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch.sh`, immediately before the final `exit $fail`:

```bash
# ---- 0237: exec -> call-and-return, exit code preserved verbatim -------------------
# The facade must regain control after the adapter (that is the whole seam the run gate hangs on),
# and every path where the gate takes no action must be byte-identical to the pre-0237 exec.
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/rc.sh" <<'RCA'
#!/usr/bin/env bash
# echoes a marker, then exits with the code named in $SBX/rc-wanted
printf 'adapter-ran\n'
printf 'adapter-stderr\n' >&2
exit "$(cat "${RC_WANTED:?}")"
RCA
chmod +x "$SBX/runners/rc.sh"
export RC_WANTED="$SBX/rc-wanted"

for want in 0 3 7; do
  printf '%s\n' "$want" > "$RC_WANTED"
  out="$( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
      bash "$FACADE" --runner rc --agent status 2>"$SBX/e.log" )"; rc=$?
  assert "0237: adapter exit code $want is preserved verbatim" '[ "$rc" = "$want" ]'
  assert "0237: adapter stdout still relayed (rc=$want)" '[ "$out" = "adapter-ran" ]'
  assert "0237: adapter stderr still relayed (rc=$want)" 'grep -qF "adapter-stderr" "$SBX/e.log"'
done

# The facade must no longer exec — prove it by execution, not by grep: code AFTER the handoff runs.
assert "0237: the facade no longer execs its adapter" \
  '! grep -qE "^[[:space:]]*exec[[:space:]]+\"\$DOCKET_BASH_PATH\"[[:space:]]+\"\$ADAPTER\"" "$FACADE"'
unset RC_WANTED
rm -rf "$SBX"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_runner_dispatch.sh`
Expected: `NOT OK - 0237: the facade no longer execs its adapter`. (The exit-code asserts pass
already under `exec` — they are the regression fence, not the discriminator. The grep assert is
the one that must go red now and green after.)

- [ ] **Step 3: Replace the handoff**

In `scripts/runner-dispatch.sh`, replace the final line:

```bash
exec "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
```

with:

```bash
# --- handoff: foreground, adapter owns everything child-specific --------------------
# CALL-AND-RETURN, not exec (change 0237). The facade must regain control so the run gate below
# can read what the delegated run actually left in git. Removing `exec` changes process semantics
# — the adapter is now a child rather than a replacement image — so the adapter's exit code is
# captured and propagated VERBATIM on every path where the gate takes no action; no existing
# caller observes a behavior change.
"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
rc=$?
exit "$rc"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 5: Check signal behavior deliberately (spec Risk 2)**

The spec calls out that removing `exec` shifts the process tree, so signal delivery needs a
deliberate check rather than an assumption. Run:

```bash
cd "$(mktemp -d)" && git init -q . && git commit -q --allow-empty -m init
mkdir -p runners && cat > runners/slow.sh <<'S'
#!/usr/bin/env bash
trap 'echo "adapter got SIGINT" >&2; exit 130' INT
sleep 30
S
chmod +x runners/slow.sh
RUNNERS_DIR="$PWD/runners" bash /Users/homer/dev/docket/scripts/runner-dispatch.sh \
  --runner slow --agent status & fpid=$!
sleep 1; kill -INT "$fpid"; wait "$fpid"; echo "facade exit: $?"
```

Expected: the adapter's trap fires (SIGINT reaches the child via the terminal's process group) and
the facade exits non-zero. Record the observed exit code in the results file as a human-verify
item — this is an interactive/manual check the suite cannot carry.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "refactor(0237): runner-dispatch calls and returns instead of exec'ing"
```

---

### Task 4: The snapshot diff and the agent gate

**Files:**
- Modify: `scripts/runner-dispatch.sh` (after the handoff)
- Test: `tests/test_runner_dispatch.sh` (append)

**Interfaces:**
- Consumes: `verify-run.sh --in-progress-ids` and `verify-run.sh <id>` from Task 1.
- Produces:
  - Mock seams `VERIFY_RUN` (default `$SELF_DIR/verify-run.sh`) and `DOCKET_FACADE`
    (default `$SELF_DIR/docket.sh`, used for the post-return re-sync).
  - A gate that engages **only** for `--agent implement-next`, reports each new claim's verdict on
    stderr, and still exits `$rc`. Re-dispatch arrives in Task 5.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch.sh` before `exit $fail`:

```bash
# ---- 0237: the run gate — snapshot diff + agent gating ----------------------------
# A stub verify-run records its argv and replies from files the fixture controls, so these asserts
# pin the FACADE's diffing and gating, not verify-run's verdict logic (Task 1 owns that).
make_gate_fixture(){
  make_fixture
  mkdir -p "$SBX/runners"
  cat > "$SBX/runners/ad.sh" <<'AD'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${AD_LOG:?}"
# each invocation advances the "after" snapshot to the next staged file, if one exists
n=$(cat "${AD_LOG:?}" | wc -l | tr -d ' ')
[ -f "${SNAP_DIR:?}/after.$n" ] && cp "${SNAP_DIR}/after.$n" "${SNAP_DIR}/current"
exit 0
AD
  chmod +x "$SBX/runners/ad.sh"
  SNAP="$SBX/snap"; mkdir -p "$SNAP"
  cat > "$SBX/fake-verify-run.sh" <<'VR'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${VR_LOG:?}"
for a in "$@"; do [ "$a" = "--in-progress-ids" ] && { cat "${SNAP_DIR:?}/current"; exit 0; }; done
for a in "$@"; do case "$a" in [0-9]*) id="$a" ;; esac; done
cat "${SNAP_DIR:?}/verdict.$id" 2>/dev/null || printf 'run-complete %s\n' "$id"
VR
  chmod +x "$SBX/fake-verify-run.sh"
  : > "$SBX/ad.log"; : > "$SBX/vr.log"
  cat > "$SBX/fake-facade.sh" <<'FF'
#!/usr/bin/env bash
exit 0
FF
  chmod +x "$SBX/fake-facade.sh"
}
run_gate(){  # $@ = facade args
  ( cd "$SBX" && RUNNERS_DIR="$SBX/runners" DOCKET_HARNESS_ROOT="$SBX" \
      SNAP_DIR="$SNAP" AD_LOG="$SBX/ad.log" VR_LOG="$SBX/vr.log" \
      VERIFY_RUN="$SBX/fake-verify-run.sh" DOCKET_FACADE="$SBX/fake-facade.sh" \
      bash "$FACADE" "$@" )
}

# (a) a NON-implement-next agent never engages the gate — load-bearing, not an optimization:
#     a build-* delegation leaves its change in-progress BY DESIGN.
make_gate_fixture
printf '5\n' > "$SNAP/current"; printf '5\n7\n' > "$SNAP/after.1"
run_gate --runner ad --agent status >/dev/null 2>&1; rc=$?
assert "0237 gate: a status delegation exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: a status delegation never calls verify-run" '[ ! -s "$SBX/vr.log" ]'
mkdir -p "$SBX/.worktrees/w"
run_gate --runner ad --agent build-standard --worktree "$SBX/.worktrees/w" >/dev/null 2>&1
assert "0237 gate: a build-* delegation never calls verify-run" '[ ! -s "$SBX/vr.log" ]'
rm -rf "$SBX"

# (b) implement-next: only the NEWLY-claimed id is verified; a pre-held claim is ignored,
#     so concurrent runs never cross-fire.
make_gate_fixture
printf '5\n' > "$SNAP/current"; printf '5\n7\n' > "$SNAP/after.1"
printf 'run-complete 7\n' > "$SNAP/verdict.7"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 gate: implement-next exits 0 when the run completed" '[ "$rc" = "0" ]'
assert "0237 gate: verify-run was called on the NEW id" 'grep -qw 7 "$SBX/vr.log"'
assert "0237 gate: the pre-held claim (5) is NOT verified" '! grep -qE "(^| )5( |$)" "$SBX/vr.log"'
rm -rf "$SBX"

# (c) an EMPTY diff (drained / contended — the run claimed nothing) is a no-op.
make_gate_fixture
printf '5\n' > "$SNAP/current"; printf '5\n' > "$SNAP/after.1"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 gate: an empty diff exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: an empty diff verifies nothing" '! grep -qE "^[0-9]" "$SBX/vr.log"'
assert "0237 gate: an empty diff dispatches exactly once" '[ "$(wc -l < "$SBX/ad.log")" = "1" ]'
rm -rf "$SBX"

# (d) run-halted NEVER re-dispatches — a halt means a human is needed.
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '9\n' > "$SNAP/after.1"
printf 'run-halted 9\n' > "$SNAP/verdict.9"
run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
assert "0237 gate: run-halted exits 0" '[ "$rc" = "0" ]'
assert "0237 gate: run-halted does NOT re-dispatch" '[ "$(wc -l < "$SBX/ad.log")" = "1" ]'
rm -rf "$SBX"

# (e) a broken snapshot disables the gate and warns — it never converts a healthy
#     dispatch into a failure (the facade's standing tolerant posture on this live path).
make_gate_fixture
cat > "$SBX/fake-verify-run.sh" <<'VRB'
#!/usr/bin/env bash
echo "verify-run: boom" >&2; exit 2
VRB
chmod +x "$SBX/fake-verify-run.sh"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 gate: an unusable snapshot does not fail the dispatch" '[ "$rc" = "0" ]'
assert "0237 gate: and it warns on stderr" 'grep -qiF "run gate" <<<"$err"'
rm -rf "$SBX"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_runner_dispatch.sh`
Expected: the `0237 gate:` asserts fail (no gate exists; every delegation dispatches once and never
calls verify-run — so (a) passes vacuously while (b), (d) and (e) fail).

- [ ] **Step 3: Implement the snapshot + gate**

In `scripts/runner-dispatch.sh`, add the seams near the top (beside `RUNNERS_DIR`):

```bash
VERIFY_RUN="${VERIFY_RUN:-$SELF_DIR/verify-run.sh}"
DOCKET_FACADE="${DOCKET_FACADE:-$SELF_DIR/docket.sh}"
```

Then replace the Task-3 handoff block with:

```bash
# --- run gate (change 0237): the disposition contract's consumer --------------------
# Engages ONLY for an implement-next delegation. That scoping is load-bearing, not an
# optimization: a build-* delegation leaves its change `in-progress` BY DESIGN (the build role
# does not reach Step 7), so gating one would fire on every healthy build. status / adr /
# review-* / finalize-change / auto-groom are likewise out of scope, and an unrecognised agent is
# a no-op — never a guess.
GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1

in_progress_ids(){ "$DOCKET_BASH_PATH" "$VERIFY_RUN" --in-progress-ids 2>/dev/null; }

BEFORE=""
if [ "$GATE" = 1 ]; then
  if ! BEFORE="$(in_progress_ids)"; then
    printf 'runner-dispatch: run gate disabled — could not read the in-progress set\n' >&2
    GATE=0
  fi
fi

"$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@"
rc=$?

[ "$GATE" = 1 ] || exit "$rc"

# The "after" read must come from FRESH ORIGIN state, never from the local tree the child just
# wrote (LEARNINGS: cas-re-read-fresh-origin). Best-effort: a failed re-sync degrades the gate's
# freshness, it does not fail a dispatch that may well have succeeded.
"$DOCKET_BASH_PATH" "$DOCKET_FACADE" preflight >/dev/null 2>&1 \
  || printf 'runner-dispatch: run gate — metadata re-sync failed; verifying against local state\n' >&2

AFTER="$(in_progress_ids)" || {
  printf 'runner-dispatch: run gate disabled — could not re-read the in-progress set\n' >&2
  exit "$rc"
}

# This run's claim = any id in AFTER that was not in BEFORE. A change another agent already held
# was in BEFORE and is ignored, so concurrent runs never cross-fire; a run that claimed nothing
# (drained, or contended where the CAS was lost) yields an empty diff and the gate is a no-op.
NEW_IDS=()
while IFS= read -r nid; do
  [ -n "$nid" ] || continue
  grep -qxF "$nid" <<<"$BEFORE" || NEW_IDS+=("$nid")
done <<<"$AFTER"

for nid in "${NEW_IDS[@]:-}"; do
  [ -n "$nid" ] || continue
  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — %s\n' "${verdict:-run-unverifiable $nid}" >&2
done

exit "$rc"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 5: Commit**

```bash
git add scripts/runner-dispatch.sh tests/test_runner_dispatch.sh
git commit -m "feat(0237): gate implement-next delegations on a snapshot-diffed verify-run"
```

---

### Task 5: One bounded re-dispatch, then a loud abort

**Files:**
- Modify: `scripts/runner-dispatch.sh` (the `for nid` loop from Task 4)
- Modify: `scripts/runner-dispatch.md`
- Test: `tests/test_runner_dispatch.sh` (append)

**Interfaces:**
- Consumes: Task 4's `NEW_IDS` array and `verdict` variable.
- Produces: exactly one re-dispatch per unfinished change; a second `run-incomplete` exits `1` with
  a stderr message naming the change id and the still-unmet conjuncts.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_runner_dispatch.sh` before `exit $fail`:

```bash
# ---- 0237: one bounded re-dispatch, then abort-and-report -------------------------
# Mirrors docket-build's one-escalation-per-task rule: exactly one more chance, then stop.

# (f) run-incomplete -> ONE re-dispatch; a now-complete second verdict exits with the adapter's code
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '4\n' > "$SNAP/after.1"
printf 'run-incomplete 4 status pr\n' > "$SNAP/verdict.4"
cat > "$SBX/fake-verify-run.sh" <<'VR2'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${VR_LOG:?}"
for a in "$@"; do [ "$a" = "--in-progress-ids" ] && { cat "${SNAP_DIR:?}/current"; exit 0; }; done
# first verdict call is incomplete, second is complete
n=$(grep -c '^4' "${VR_LOG:?}" || true)
if [ "$n" -le 1 ]; then printf 'run-incomplete 4 status pr\n'; else printf 'run-complete 4\n'; fi
VR2
chmod +x "$SBX/fake-verify-run.sh"
out="$( run_gate --runner ad --agent implement-next 2>"$SBX/e.log" )"; rc=$?
assert "0237 redispatch: exits 0 once the second verdict is complete" '[ "$rc" = "0" ]'
assert "0237 redispatch: the adapter ran exactly TWICE" '[ "$(wc -l < "$SBX/ad.log")" = "2" ]'
assert "0237 redispatch: the retry carries the change id as task context" \
  'grep -qF " 4" "$SBX/ad.log"'
assert "0237 redispatch: the retry names the unmet conjuncts" \
  'grep -qE "status|pr" "$SBX/ad.log"'
assert "0237 redispatch: the retry keeps --agent implement-next" \
  'grep -qF -- "--agent implement-next" "$SBX/ad.log"'
rm -rf "$SBX"

# (g) two strikes -> loud non-zero naming the change and the still-unmet conjuncts
make_gate_fixture
printf '\n' > "$SNAP/current"; printf '6\n' > "$SNAP/after.1"
cat > "$SBX/fake-verify-run.sh" <<'VR3'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${VR_LOG:?}"
for a in "$@"; do [ "$a" = "--in-progress-ids" ] && { cat "${SNAP_DIR:?}/current"; exit 0; }; done
printf 'run-incomplete 6 status pr branch\n'
VR3
chmod +x "$SBX/fake-verify-run.sh"
err="$( run_gate --runner ad --agent implement-next 2>&1 >/dev/null )"; rc=$?
assert "0237 two-strikes: exits NON-ZERO" '[ "$rc" != "0" ]'
assert "0237 two-strikes: names the change id" 'grep -qE "(^| )6( |$)" <<<"$err"'
assert "0237 two-strikes: names the still-unmet conjuncts" 'grep -qF "branch" <<<"$err"'
assert "0237 two-strikes: caps the adapter at exactly TWO runs" \
  '[ "$(wc -l < "$SBX/ad.log")" = "2" ]'
rm -rf "$SBX"

# (h) the re-dispatch does NOT fire on run-complete / run-halted / run-unclaimed
for v in run-complete run-halted run-unclaimed; do
  make_gate_fixture
  printf '\n' > "$SNAP/current"; printf '8\n' > "$SNAP/after.1"
  printf '%s 8\n' "$v" > "$SNAP/verdict.8"
  run_gate --runner ad --agent implement-next >/dev/null 2>&1; rc=$?
  assert "0237 redispatch: $v exits 0" '[ "$rc" = "0" ]'
  assert "0237 redispatch: $v dispatches exactly once" '[ "$(wc -l < "$SBX/ad.log")" = "1" ]'
  rm -rf "$SBX"
done
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_runner_dispatch.sh`
Expected: the `0237 redispatch:` and `0237 two-strikes:` asserts fail — the adapter runs once and
the facade exits 0.

- [ ] **Step 3: Implement the bounded retry**

Replace Task 4's `for nid` loop in `scripts/runner-dispatch.sh` with:

```bash
STILL_INCOMPLETE=()
for nid in "${NEW_IDS[@]:-}"; do
  [ -n "$nid" ] || continue
  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) : ;;
    # run-halted NEVER re-dispatches: a halt means a human is needed, and spending a second full
    # agent run on it is waste. run-complete and run-unclaimed need nothing.
    *) continue ;;
  esac

  # ONE bounded re-dispatch — docket-build's one-escalation-per-task rule, applied at this seam.
  unmet="${verdict#run-incomplete "$nid" }"
  retry_ctx="docket-implement-next $nid — the previous run left Step 7 unmet (${unmet}); resume that change and finish it: push the branch, open the PR, and write status: implemented + pr:. If it genuinely cannot proceed, write a dated '## Run halted' section into the change file and commit it."
  printf 'runner-dispatch: run gate — re-dispatching once for change %s (%s)\n' "$nid" "$unmet" >&2
  "$DOCKET_BASH_PATH" "$ADAPTER" "${args[@]}" -- "$@" "$retry_ctx"

  verdict="$("$DOCKET_BASH_PATH" "$VERIFY_RUN" "$nid" 2>/dev/null)"
  printf 'runner-dispatch: run gate — after re-dispatch: %s\n' "${verdict:-run-unverifiable $nid}" >&2
  case "$verdict" in
    run-incomplete*) STILL_INCOMPLETE+=("$verdict") ;;
  esac
done

if [ "${#STILL_INCOMPLETE[@]}" -gt 0 ]; then
  # Abort-and-report. The change stays `in-progress` with its claim intact; board-checks'
  # `aborted-run` remains the standing backstop. This is the only NEW non-zero this change
  # introduces, and it is on a path that is presently silent.
  printf 'runner-dispatch: RUN GATE FAILED after one re-dispatch — a delegated implement-next run did not reach its PR:\n' >&2
  for v in "${STILL_INCOMPLETE[@]}"; do printf '  %s\n' "$v" >&2; done
  exit 1
fi

exit "$rc"
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 5: Update `scripts/runner-dispatch.md`**

Three edits, matching the code exactly:

1. In **Behavior**, replace step 4 (`**Handoff** — exec …`) with:

```markdown
4. **Handoff** — `"$DOCKET_BASH_PATH" scripts/runners/<name>.sh --agent <agent> [--model m]
   [--effort e] -- <args…>`, foreground, **call-and-return** (change 0237 — no longer `exec`).
   The adapter's stdout/stderr pass through and its exit code is propagated **verbatim** on every
   path where the run gate takes no action.
5. **Run gate (change 0237)** — engages **only** for `--agent implement-next`. Before the handoff
   the facade records the set of `in-progress` change ids (`verify-run --in-progress-ids`); after
   the handoff it re-syncs the metadata worktree and re-reads the set. Any id **not** in the
   before-set is this run's claim, and each is checked with `verify-run <id>`:

   - `run-complete` / `run-halted` / `run-unclaimed` → nothing; exit the adapter's code.
   - `run-incomplete` → **one** bounded re-dispatch of the same adapter, with the change id and the
     unmet conjuncts as task context. If the second verdict is still `run-incomplete`, the facade
     aborts loudly with exit `1`, naming the change and the still-unmet conjuncts.

   `run-halted` never re-dispatches — a halt means a human is needed. A `build-*` delegation leaves
   its change `in-progress` by design, which is why the agent gate is load-bearing rather than an
   optimization. An unrecognised agent is a no-op, never a guess. A snapshot that cannot be read
   **disables the gate with a warning** — it never converts a healthy dispatch into a failure.
```

2. In **Exit codes**, add:

```markdown
- `1` — the run gate's two-strikes abort: a delegated `implement-next` run was still
  `run-incomplete` after one re-dispatch. The change stays `in-progress` with its claim intact.
```

3. In **Invariants**, add:

```markdown
- The adapter's exit code is propagated verbatim whenever the run gate takes no action; the
  two-strikes abort is the only new non-zero, and only on a path that was previously silent.
- The run gate is scoped to `--agent implement-next` and never writes docket state — it acts only
  by running an agent.
```

Also update the **Purpose** paragraph's "hands off — foreground — to the named per-runner adapter"
to say it **calls and returns**, and drop the "the facade itself never changes" clause, which
change 0237 falsifies.

- [ ] **Step 6: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: `failed=0`.

- [ ] **Step 7: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "feat(0237): give an unfinished delegated run one re-dispatch, then abort loudly"
```

---

### Task 6: `## Run halted` — the section, its producer, and its removal

**Files:**
- Modify: `skills/docket-convention/SKILL.md:192` (after the `## Finalize blocked` bullet)
- Modify: `skills/docket-implement-next/SKILL.md` (Step 2 claim — removal; Step 3 + *Terminal
  disposition* — the producer)
- Modify: `scripts/board-checks.md` (one pointer sentence)
- Test: `tests/test_verify_run.sh` (append a prose-sentinel section)

**Interfaces:**
- Consumes: Task 1's `run-halted` verdict (the consumer).
- Produces: the **producer** — a numbered step that writes the section — plus its removal rule.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_verify_run.sh`, before `exit $fail`:

```bash
# ---- 0237: `## Run halted` — producer coverage, not just definition ----------------
# LEARNINGS specified-but-unreachable: a contract can be fully specified and ship INERT because
# nothing writes it. Consumer-side asserts pass identically in both worlds, so at least one assert
# must anchor on the paragraph that performs the WRITE.
CONV="$ROOT/skills/docket-convention/SKILL.md"
IMPL="$ROOT/skills/docket-implement-next/SKILL.md"
BCMD="$ROOT/scripts/board-checks.md"

# collapse wrapped prose before matching, so a pure re-flow never reddens a policy assert
flat(){ tr '\n' ' ' < "$1" | tr -s ' '; }

assert "0237 prose: the convention lists '## Run halted' as a body section" \
  'grep -qF "- \`## Run halted\`" "$CONV"'
assert "0237 prose: the convention's entry names it presence-encoded" \
  'grep -F "## Run halted" "$CONV" | grep -qiF "presence-encoded"'

# PRODUCER — anchored on the halted disposition prose that performs the write, not on a section
# that merely defines what the write means.
assert "0237 prose: the halted disposition WRITES the section" \
  'flat "$IMPL" | grep -qiE "halted[^.]{0,400}(write|writing|append)[^.]{0,200}## Run halted|## Run halted[^.]{0,200}(commit|committed)"'
assert "0237 prose: the halted write is described as a COMMITTED git act" \
  'flat "$IMPL" | grep -qiE "## Run halted[^.]{0,300}commit"'

# REMOVAL — owned by Step 2's claim (presence-encoded-state: every transition out removes it).
step2="$(awk "/^### Step 2 — Claim/,/^### Step 3/" "$IMPL" | tr '\n' ' ' | tr -s ' ')"
assert "0237 prose: Step 2's claim removes a stale '## Run halted'" \
  'grep -qF "## Run halted" <<<"$step2"'
assert "0237 prose: and states removal, not merely mentions the section" \
  'grep -qiE "(remove|delete|strip)[^.]{0,120}## Run halted|## Run halted[^.]{0,120}(remove|delete|strip)" <<<"$step2"'

# board-checks.md gains the pointer sentence and NOTHING in board-checks.sh changed.
assert "0237 prose: board-checks.md points at verify-run" 'grep -qF "verify-run" "$BCMD"'
assert "0237 prose: the pointer says the check is floor-free at a dispatch seam" \
  'flat "$BCMD" | grep -qiE "verify-run[^.]{0,200}(floor-free|no floor|without a floor|dispatch seam)"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_verify_run.sh`
Expected: every `0237 prose:` assert `NOT OK`.

- [ ] **Step 3: Add the convention entry**

In `skills/docket-convention/SKILL.md`, immediately after the `## Finalize blocked` bullet:

```markdown
- `## Run halted` — dated record appended by an autonomous run that stops needing a human (the
  `halted` disposition). **Presence-encoded state**, in the same family as `## Auto-groom blocked`
  and `## Finalize blocked`: the run clears `verify-run`'s gate by *writing this section and
  committing it*, which is what makes a `halted` disposition verifiable in git rather than a claim
  in a completion report. Removal is owned by `docket-implement-next`'s Step 2 claim — the only
  transition back into a live run — and is stated there, not restated here.
```

- [ ] **Step 4: Wire the producer and the removal in `docket-implement-next`**

**(a) Step 2 — removal.** Append to the Step 2 paragraph, after `No worktree yet.`:

```markdown
The claim also **removes any `## Run halted` section** the change carries: the section is
presence-encoded state recording that a previous run stopped needing a human, and a fresh claim is
the transition back into a live run, so a stale record left in place would tell `verify-run` that
an actively-running change had deliberately stopped. Delete the whole section in the same claim
commit; git history keeps it.
```

**(b) Step 3 — the producer.** In the second escape hatch (*Design FUNDAMENTALLY invalidated*),
after `end the run with the **`halted`** disposition`, insert:

```markdown
— and **write that halt into git before stopping**: append a dated `## Run halted` section to the
change file naming what stopped the run and what a human must decide, and **commit and push it on
`metadata_branch`** with the rest of the metadata discipline. This is the producer half of the
section the convention defines: a `halted` disposition that exists only as a sentence in a
completion report is exactly the untrusted self-report `verify-run` was built to stop trusting.
The same write is required for **any** hard error that ends the run `halted`, wherever it occurs.
```

**(c) Terminal disposition table.** In the `halted` row's *Meaning* cell, append:
`Records a dated \`## Run halted\` section, committed on \`metadata_branch\`.`

- [ ] **Step 5: Add the `board-checks.md` pointer sentence**

`scripts/board-checks.md` has **no `## Not covered` heading** (verified at reconcile). Add the
sentence to the end of the `aborted-run` block's **"The surviving residual is offline, or `gh`
unavailable"** paragraph:

```markdown
  A **floor-free** check of the same postcondition does exist, but only where a board pass cannot
  reach: `verify-run` (change 0237) evaluates Step 7's postcondition with **no time floor at all**,
  because it is called at a **dispatch seam** where the child process has already returned and
  "stopped" is therefore unambiguous. That is why this script keeps its floors and is otherwise
  untouched — the two checks answer the same question from positions with different information.
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `bash tests/test_verify_run.sh`
Expected: all `ok -`, exit 0.

- [ ] **Step 7: Prove the producer assert actually detects removal (mutation)**

```bash
cp skills/docket-implement-next/SKILL.md /tmp/impl.bak
# delete the producer sentence added in Step 4(b)
perl -0pi -e 's/— and \*\*write that halt into git before stopping\*\*.*?wherever it occurs\.//s' \
  skills/docket-implement-next/SKILL.md
bash tests/test_verify_run.sh | grep 'NOT OK'      # expect the two PRODUCER asserts
cp /tmp/impl.bak skills/docket-implement-next/SKILL.md
```

Expected: the producer asserts go `NOT OK` with the sentence gone, and green again once restored.
Restore before committing. (Do **not** use `git checkout --` to restore — it restores to HEAD and
would destroy the uncommitted work under test.)

- [ ] **Step 8: Confirm `board-checks.sh` is untouched**

Run: `git diff --stat origin/main...HEAD -- scripts/board-checks.sh`
Expected: **no output** — the spec's §4 scope boundary.

- [ ] **Step 9: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: `failed=0`.

- [ ] **Step 10: Commit**

```bash
git add skills/docket-convention/SKILL.md skills/docket-implement-next/SKILL.md \
        scripts/board-checks.md tests/test_verify_run.sh
git commit -m "feat(0237): add the '## Run halted' section, its producer, and its removal rule"
```

---

## Self-Review

**1. Spec coverage.**

| Spec item | Task |
|---|---|
| §1 `verify-run.sh` — four verdicts, anchored reads, exit-0-on-verdict, no time floor | 1 |
| §1 "deliberately does not do" (no writes, no `gh`) | 1 (contract + invariants), 2 (`verify-run.md`) |
| §2 `exec` → call-and-return, exit-code preservation | 3 |
| §2 snapshot diff, fresh-origin re-sync, agent gating | 4 |
| §2 one bounded re-dispatch, two-strikes abort, `run-halted` never re-dispatches | 5 |
| §3 `## Run halted` section, its git-act semantics, removal at Step 2 | 6 |
| §4 `board-checks.sh` untouched; `board-checks.md` gains one pointer sentence | 6 (Steps 5 and 8) |
| Scope: facade `WRAPPED_OPS` + `docket.md` table together | 2 |
| Scope: `scripts/verify-run.md` co-located contract | 2 |
| Scope: `tests/test_verify_run.sh` + runner-dispatch suite extensions | 1, 3, 4, 5, 6 |
| Risk: removing `exec` shifts signal semantics — needs a deliberate check | 3 (Step 5, → results file) |

Nothing in the spec's **In** list is unassigned. Nothing in its **Out** list is touched: no
`Stop`/`SubagentStop` hook, no `settings.json`, no installer, no `board-checks.sh` legs or floors,
no new config knob, no status flip or claim release, and no additional prose rider beyond the two
the section itself requires (its definition and its producer/removal, which are the mechanism, not
another exhortation).

**2. Placeholder scan.** No `TBD`, no "add error handling", no "similar to Task N", no referenced
symbol that is undefined. The one deliberately-conditional step is Task 1 Step 3's note about the
resolver's exported variable name — it names the exact command to run and both branches to take,
rather than deferring the decision.

**3. Type consistency.** `verify-run.sh` is called with the same two shapes everywhere
(`<id> [--changes-dir DIR]`, `--in-progress-ids [--changes-dir DIR]`); the unmet-conjunct tokens
are `status pr branch` in that fixed order in the script, its contract, and every assert; the
seams are `VERIFY_RUN`, `DOCKET_FACADE`, `GIT`, `CONFIG_EXPORT_CMD` throughout; `NEW_IDS` and
`STILL_INCOMPLETE` are introduced in Task 4 and consumed in Task 5 under those names.
