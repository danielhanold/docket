<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0157 — Roll up the seven build-ready changes into one branch](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0157-roll-up-the-seven-build-ready-changes-into-one-branch.md)**
<!-- docket:backlink:end -->

# Build-ready fix rollup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land seven already-designed changes — 0143, 0144, 0146, 0148, 0149, 0152, 0153 — on one branch, one PR, with the full suite green at the end.

**Architecture:** Each unit is a small, self-contained edit to `scripts/` or `tests/` in this repo. Three units are independent; four form two ordered pairs that touch a shared file (0148 → 0149 in `tests/test_docket_config.sh`; 0152 → 0153 in `scripts/lib/docket-runtime.sh`). The commit order below **is** the ordering constraint, so the diff reads unit by unit.

**Tech Stack:** POSIX `sh` and GNU Bash 4+ shell scripts, `awk`, `sed`, `grep`. Tests are hand-rolled `assert`-helper scripts under `tests/`, each run as `bash tests/<name>.sh`, printing `ok - …` / `NOT OK - …` and exiting non-zero on failure.

## Global Constraints

- **Bash 4+ is the floor for `scripts/`**, with ONE exception: `scripts/lib/docket-runtime.sh` is **BOOTSTRAP-COMPATIBLE BY REQUIREMENT** — every line must parse and run under macOS's system Bash 3.2.57. Forbidden inside that file: associative arrays, `mapfile`/`readarray`, `${x^^}`/`${x,,}`, `declare -g`, `;;&`.
- **`scripts/docket.sh`'s prologue is POSIX `sh`** (its shebang is `#!/bin/sh`) and must stay so — it runs before a Bash interpreter is chosen.
- **Use `/usr/bin/grep` for any portability re-check.** The PATH `grep` on this machine is **ugrep 7.5.0** and accepts ERE syntax `/usr/bin/grep` rejects, so a PATH-`grep` check can pass while the code is broken under BSD grep.
- **Never weaken a test to get the branch green.** If a unit cannot go green, drop that unit from the branch and report it — do not relax an assert.
- **Run the whole suite at the build gate**, never only the tests a task enumerated (AGENTS.md).
- **A guard is code: mutation-test it** — strip the thing it guards, watch it redden — or it is decoration (AGENTS.md).
- **`grep` for a pattern leading with `--` must declare it**: `grep -qF -- "<pat>"`. A bare leading `--` is parsed as an option (exit 2), and inside a negated assert that error inverts into a permanently green, vacuous guard (AGENTS.md).
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`) under `set -o pipefail` — capture into a variable, then `grep <<<"$var"` (AGENTS.md).
- **awk indent classes are `[^[:space:]]`, never `[^ ]`** — a literal-space class silently drops tab-indented input (AGENTS.md).
- **Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause, never a line number** (AGENTS.md; `tests/test_comment_anchor_style.sh` enforces the filename-plus-line-number form).

**Full-suite command** (there is no runner script; 66 files under `tests/`):

```bash
cd /Users/homer/dev/docket/.worktrees/roll-up-the-seven-build-ready-changes-into-one-branch
sfail=0
for f in tests/*.sh; do
  out="$(bash "$f" 2>&1)"; rc=$?
  n_ok="$(grep -c '^ok - ' <<<"$out")"; n_bad="$(grep -c '^NOT OK - ' <<<"$out")"
  if [ "$rc" -ne 0 ] || [ "$n_bad" -ne 0 ]; then
    sfail=1; printf 'FAIL %-46s rc=%s ok=%s bad=%s\n' "$f" "$rc" "$n_ok" "$n_bad"
    grep '^NOT OK - ' <<<"$out" | head -20
  else
    printf 'pass %-46s ok=%s\n' "$f" "$n_ok"
  fi
done
echo "SUITE_FAIL=$sfail"
```

**Measured baseline (taken on this branch at `origin/main` = `f264d7e6`, before any task):**

| Fact | Value |
|---|---|
| `tests/test_docket_config.sh` assert count | **381**, 0 failing |
| Its guard `TOTALS` line | `TOTALS sites=64 exempt=3 ok=61 viol=0` |
| `tests/test_render_board.sh` | 200 ok, 0 fail |
| `.docket.local.yml` occurrences in `skills/**` | 8 — all in the two excluded files |
| `config.yml` occurrences in `skills/**` | 5 — all in the two excluded files |
| Occurrences of either outside the exclusions | **0** |

---

### Task 1: 0143 — guard `render-board.sh`'s archive feeder and both tally loops

**Files:**
- Modify: `scripts/render-board.sh` (three one-line guards; see steps)
- Test: `tests/test_render_board.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks (first task).
- Produces: nothing later tasks rely on. `scripts/render-board.sh` is not touched again by this plan.

**Background.** The archive block feeds `sort` with a TAB-joined 4-tuple and reads it back with `IFS=$'\t'`. TAB is IFS *whitespace*, so `read` collapses runs of it and an **empty field is not preserved** — later fields shift left. Separately, both per-status tally loops assign into an associative array using `$st` as the subscript; an empty `status:` makes that assignment fail with `bad array subscript`, and **that error aborts the whole `for` loop**, silently dropping every file sorted after the offending one. The script still exits 0, so a corrupt `BOARD.md` commits silently.

**This exact behavior was reproduced and measured during planning.** Pre-fix, against the fixture in Step 1, the render emits corrupt rows `| [0005](archive/) |  | 2026-07-02 |` and `| [0000](archive/) |  | 2026-07-01 |`, a truncated header `Archive — done (1)` (two `done` files exist), the stderr block below, and a digest of exactly `backlog done 1` plus an **empty `ready` line** — the machine-parsed queue `docket-implement-next` reads.

```
scripts/render-board.sh: line 141: SECTION["$st"]: bad array subscript
scripts/render-board.sh: line 154: ARC_COUNT: bad array subscript
scripts/render-board.sh: line 154: ARC_COUNT["$st"]: bad array subscript
sed: : No such file or directory
scripts/render-board.sh: line 125: printf: done: invalid number
sed: : No such file or directory
```

**Deliberately preserved (do NOT "fix" it):** after the change, an archive file with an empty `id:` and `status: done` is still counted by `ARC_COUNT` (which keys on `status` alone) while rendering no row — the header says `Archive — done (2)` above one row. That header/table mismatch is the state change 0115's `board-row-dropped` check exists to **report**. Do **not** add an `id` guard to the `ARC_COUNT` loop. Step 4's assert pins the mismatch as intended so a later "fix" cannot land silently.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_render_board.sh`, immediately before its final `if [ "$fail" = 0 ]` / `exit` lines. Read the file's existing helpers first and reuse its `assert` and tmpdir conventions rather than the names below if they differ.

```bash
# --- change 0143: empty id/status must not collapse the TAB-joined archive sort feeder ---
# TAB is IFS whitespace, so `read` collapses runs of it: an empty field is not preserved and every
# later field shifts left. The consuming loop's own `[ -n "$id" ] || continue` sits DOWNSTREAM of
# the lossy join and never sees the empty value. Separately, an empty `status:` used as an
# associative-array subscript fails with `bad array subscript`, and that error ABORTS the whole
# `for` loop, dropping every file sorted after it from the tally.
#
# PRE-FIX OUTPUT (measured 2026-07-28, so these asserts' keys are pinned, not merely present):
#   markdown : "Archive — done (1)" above three rows, two of them
#              "| [0005](archive/) |  | 2026-07-02 |" and "| [0000](archive/) |  | 2026-07-01 |"
#   stderr   : SECTION/ARC_COUNT "bad array subscript", "printf: done: invalid number",
#              "sed: : No such file or directory"
#   digest   : "backlog done 1" and an EMPTY `ready` line (no `backlog proposed`, no `change 8`)
c143="$(mktemp -d)"; _tmpdirs+=("$c143")
mkdir -p "$c143/active" "$c143/archive"
printf -- '---\nid:\nslug: empty-id\ntitle: Empty Id\nstatus: done\ncreated: 2026-07-01\n---\n' \
  > "$c143/archive/2026-07-01-0000-empty-id.md"
printf -- '---\nid: 5\nslug: empty-status\ntitle: Empty Status\nstatus:\ncreated: 2026-07-02\n---\n' \
  > "$c143/archive/2026-07-02-0005-empty-status.md"
printf -- '---\nid: 6\nslug: ok\ntitle: Fine\nstatus: done\ncreated: 2026-07-03\n---\n' \
  > "$c143/archive/2026-07-03-0006-ok.md"
printf -- '---\nid: 7\nslug: bad-active\ntitle: Bad Active\nstatus:\npriority: medium\ncreated: 2026-07-04\n---\n' \
  > "$c143/active/0007-bad-active.md"
printf -- '---\nid: 8\nslug: good-active\ntitle: Good Active\nstatus: proposed\npriority: medium\ncreated: 2026-07-05\nspec: docs/x.md\n---\n' \
  > "$c143/active/0008-good-active.md"

c143_md="$(bash "$RENDER" --changes-dir "$c143" 2>/dev/null)"
c143_err="$(bash "$RENDER" --changes-dir "$c143" 2>&1 >/dev/null)"
c143_digest="$(bash "$RENDER" --changes-dir "$c143" --format digest 2>/dev/null)"

# No corrupt row. Anchored on the ERE `^\| \[[0-9]{4}\]\(archive/\) \|` rather than the bare
# substring `](archive/) `, which ALSO matches a legitimate shipping row: the older-done collapse
# table emits `| [2026-07](archive/) | 62 done |`. The YYYY-MM key is not four digits, so the ERE
# excludes it regardless of fixture size — the assert must not depend on the fixture staying under
# ARCHIVE_RECENT (15). /usr/bin/grep, never PATH grep (which is ugrep here).
assert "0143: no corrupt archive row with an empty basename" \
  '! /usr/bin/grep -qE "^\| \[[0-9]{4}\]\(archive/\) \|" <<<"$c143_md"'
assert "0143: the well-formed archive row still renders" \
  '/usr/bin/grep -qF -- "| [0006](archive/2026-07-03-0006-ok.md) | Fine | 2026-07-03 |" <<<"$c143_md"'
assert "0143: render stderr is clean (no subscript abort, no printf/sed noise)" \
  '[ -z "$c143_err" ]'
# The header counts BOTH done files while only one row renders. This mismatch is INTENDED: it is
# the case-(B) state change 0115's board-row-dropped check exists to report, so ARC_COUNT keeps no
# id guard. Asserted so a later "fix" to that loop cannot land silently.
assert "0143: the archive header tally is not truncated by the abort" \
  '/usr/bin/grep -qF -- "Archive — done (2)" <<<"$c143_md"'
assert "0143: digest counts both done files" \
  '/usr/bin/grep -qxF "backlog done 2" <<<"$c143_digest"'
assert "0143: digest still reaches the active change behind the empty-status file" \
  '/usr/bin/grep -qxF "backlog proposed 1" <<<"$c143_digest"'
assert "0143: the ready queue line is not emptied by the tally abort" \
  '/usr/bin/grep -qxF "ready 8" <<<"$c143_digest"'
```

**Before running:** confirm `$RENDER` and `_tmpdirs` are the names `tests/test_render_board.sh` actually uses for the script path and its cleanup array. If not, substitute the file's own conventions — do not introduce a second idiom.

- [ ] **Step 2: Run the test to verify it fails**

```bash
bash tests/test_render_board.sh 2>&1 | grep -E '^NOT OK - 0143'
```

Expected: **six** of the seven new asserts FAIL (`no corrupt archive row`, `render stderr is clean`, `the archive header tally is not truncated`, `digest counts both done files`, `digest still reaches the active change`, `the ready queue line is not emptied`). The `well-formed archive row still renders` assert passes pre-fix — it is the non-regression control. If any of the six passes pre-fix, the fixture is wrong: stop and re-derive it.

- [ ] **Step 3: Apply the three guards**

In `scripts/render-board.sh`. Locate each site by its text, not by line number.

**(a) Active tally loop** — find `id="$(int_field "$f" id)"; [ -n "$id" ] || continue` followed by `st="$(field "$f" status)"`, and append a guard to the `st` line:

```bash
  st="$(field "$f" status)"; [ -n "$st" ] || continue
```

**(b) Archive tally loop** — find the single-line `for f in "${ARCFILES[@]}"; do st="$(field "$f" status)"; ARC_COUNT[…` and insert a guard between the assignment and the array write:

```bash
for f in "${ARCFILES[@]}"; do st="$(field "$f" status)"; [ -n "$st" ] || continue; ARC_COUNT["$st"]=$(( ${ARC_COUNT[$st]:-0} + 1 )); done
```

**(c) Archive sort feeder** — find the `base="$(basename "$f")"; d="${base:0:10}"; id=…; st=…` line inside the `done < <( … )` process substitution, and insert a new line **immediately after it, above the `printf`**:

```bash
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"; st="$(field "$f" status)"
      [ -n "$id" ] && [ -n "$st" ] || continue
      printf '%s\t%s\t%s\t%s\n' "$d" "$id" "$st" "$f"
```

**Leave the downstream `[ -n "$id" ] || continue` inside the `while` loop in place** — it stays as defense in depth. **Change no delimiter.**

- [ ] **Step 4: Run the tests to verify they pass**

```bash
bash tests/test_render_board.sh 2>&1 | tail -3
```

Expected: `PASS`-equivalent — 0 `NOT OK` lines, and the pre-existing golden byte-compare still green. The fix only skips files carrying an empty `id`/`status`, neither of which occurs in the golden tree, so the golden must stay byte-identical.

Then the neighbours, all of which read this renderer:

```bash
for t in test_render_board test_board_checks test_board_refresh test_board_refresh_on_transition test_docket_status; do
  printf '%-40s' "$t"; o="$(bash tests/$t.sh 2>&1)"
  echo "$(grep -c '^ok - ' <<<"$o") ok, $(grep -c '^NOT OK - ' <<<"$o") fail"
done
```

Expected (measured during planning under this exact patch): `test_render_board` **200 ok, 0 fail**; `test_board_checks` 177/0; `test_board_refresh` 62/0; `test_board_refresh_on_transition` 18/0; `test_docket_status` 0 failing.

- [ ] **Step 5: Commit**

```bash
git add scripts/render-board.sh tests/test_render_board.sh
git commit -m "fix(0143): guard the archive sort feeder and both tally loops against empty fields"
```

---

### Task 2: 0144 — surface a `board-checks.sh` non-zero exit in the health pass

**Files:**
- Modify: `scripts/docket-status.sh` (`health_checks()` and its header comment)
- Modify: `scripts/docket-status.md` (§7 prose + the `## Output contract` table)
- Test: `tests/test_docket_status.sh`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: a new report-line token `health checks failed <exit>` on `docket-status.sh` stdout. No later task consumes it.

**Background.** `health_checks()` pipes `board-checks.sh` into a `while IFS=$'\t' read` loop and never reads the producer's exit status. `board-checks.sh` accumulates findings into `$FINDINGS` and prints once at the end, so every validation failure (`exit 2`) emits **zero** TSV lines: the loop body never runs, `health_checks` returns 0, and the report is byte-indistinguishable from a clean tree.

**Verified against the running code:** `scripts/docket-status.sh`'s prologue is `set -uo pipefail` — **no `-e`** — so `|| rc=$?` is safe and does not abort the pass.

**The token deliberately sits OUTSIDE the `board ` family.** `skills/docket-status/SKILL.md` teaches callers that `board *-failed` / "no `board …` line" means **the board step** failed. Naming this `board-checks failed` would land inside that taught contract. `health checks failed <exit>` names its own pass, matching the existing `learnings index failed` shape. Verified safe against every existing matcher: `RECLAIMABLE_LINE_RE` anchors on `^check stale-in-progress …`; `board_classify` matches `case "$line" in "board "*)`; SKILL.md's gloss is `board *-failed`. **None matches a line beginning `health checks `.** This is also why `skills/docket-status/SKILL.md` is NOT edited by this task — change 0145 owns that file.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_status.sh`. Read its existing `board-checks.sh` mock fixture first and reuse it — 0117's regression test already builds one; extend that harness rather than inventing a second.

```bash
# --- change 0144: a board-checks.sh non-zero exit must be reported, not swallowed ---
# board-checks.sh prints its findings once at the END, so a validation failure (exit 2) emits ZERO
# TSV lines: the read loop never runs and the report is byte-identical to a clean tree. 0117's
# regression test cannot see this — its mock exits 0 regardless of arguments, so the assert passes
# against both the fixed and the unfixed code (green-suite-untested-branch).

# (1) mock exits 2 with no output -> exactly one diagnostic; the run still completes.
mk_bc_mock 2 ''            # <- use this file's own mock-builder; see note below
o144="$(run_status_full)"
assert "0144: a board-checks exit 2 emits exactly one health-checks diagnostic" \
  '[ "$(grep -c "^health checks failed 2$" <<<"$o144")" = 1 ]'
assert "0144: the pass still completes after a checker failure" \
  'grep -qxF "pass ok" <<<"$o144"'

# (2) a different non-zero code is carried verbatim.
mk_bc_mock 1 ''
o144b="$(run_status_full)"
assert "0144: the exit code is carried verbatim (1)" \
  'grep -qxF "health checks failed 1" <<<"$o144b"'

# (3) ADDITIVE, not replacement: findings AND the diagnostic. A naive
# `if rc; then diagnostic; else findings; fi` fails this. Not hypothetical — board-checks.sh's
# --strict path already prints $FINDINGS and THEN exits 1; health_checks is one flag away.
mk_bc_mock 2 "$(printf 'broken-spec\t42\tspec missing')"
o144c="$(run_status_full)"
assert "0144: a finding emitted before a non-zero exit is still reported" \
  'grep -qxF "check broken-spec 42 spec missing" <<<"$o144c"'
assert "0144: the diagnostic accompanies the finding rather than replacing it" \
  'grep -qxF "health checks failed 2" <<<"$o144c"'

# --- NON-REGRESSION PINS. Expected to pass BOTH ways; do NOT apply the mutation mandate. ---
# The no-false-positive direction (correspondence-guard-runs-one-way).
mk_bc_mock 0 "$(printf 'broken-spec\t42\tspec missing')"
o144d="$(run_status_full)"
assert "0144: a clean exit with findings emits NO diagnostic" \
  '! grep -q "^health checks failed" <<<"$o144d"'
assert "0144: findings on a clean exit are unchanged" \
  'grep -qxF "check broken-spec 42 spec missing" <<<"$o144d"'

# The reclaim remedy still fires when the checker also failed.
mk_bc_mock 2 "$(printf 'stale-in-progress\t7\tlease expired [reclaimable]')"
o144e="$(run_status_full)"
assert "0144: the reclaim remedy line survives a checker failure" \
  'grep -q "reclaim-claims" <<<"$o144e"'

# --- STRUCTURAL GUARD. The capture-scope argument rests on call ordering inside main() that a
# future edit could silently break: board_pass_must_land classifies only its OWN board_out,
# captured BEFORE health_checks ever runs. A bare --must-land DOES run the full path (the arg
# parser excludes --must-land only against --digest-only), so this needs its own test.
mk_bc_mock 2 ''
o144f="$(run_status_must_land_clean_board)"; rc144f=$?
assert "0144: --must-land still exits 0 when only the health checker failed" \
  '[ "$rc144f" -eq 0 ] && grep -qxF "pass ok" <<<"$o144f"'

# --- DOC GUARD: the STALE SENTENCE IS GONE, not merely that the new line is documented. The
# existing `assert "status contract documents …"` family pins presence only.
assert "0144: the stale 'a board-checks failure produces no extra output' claim is gone" \
  '! grep -qF "or a \`board-checks.sh\` failure, produces no extra output" "$REPO/scripts/docket-status.md"'
assert "0144: the output contract documents the new health-pass line" \
  'grep -qF "health checks failed <exit>" "$REPO/scripts/docket-status.md"'
```

**Before running:** `mk_bc_mock`, `run_status_full`, `run_status_must_land_clean_board`, and `$REPO` are placeholders for whatever `tests/test_docket_status.sh` already provides. Read the file, find its existing `board-checks.sh` mock and its status-invocation helpers, and express the six scenarios through them. If no mock-builder exists, write one small helper that takes `<exit-code> <tsv-body>` and generates the mock — but reuse the file's existing mock **directory/`SCRIPTS_DIR`** plumbing.

- [ ] **Step 2: Run the tests to verify they fail**

```bash
bash tests/test_docket_status.sh 2>&1 | grep -E '^NOT OK - 0144'
```

Expected: the three mutation-mandate asserts (`emits exactly one health-checks diagnostic`, `carried verbatim (1)`, `accompanies the finding`) FAIL, plus both doc asserts FAIL. The four non-regression pins and the `--must-land` structural guard PASS pre-fix — that is what makes them pins rather than mutations. **If a non-regression pin fails pre-fix, stop**: the harness is wrong, not the code.

- [ ] **Step 3: Rewrite `health_checks()`**

In `scripts/docket-status.sh`, replace the pipeline at the end of `health_checks()` — the `"$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh … | while IFS=$'\t' read …` block — with capture-then-consume. **Keep everything above it (the `mw`/`cd_dir`/`metadata_branch` resolution and the whole `adr_args` block with its comments) byte-untouched.**

```bash
  # Capture-then-consume (change 0144). board-checks.sh accumulates findings into $FINDINGS and
  # prints once at the END, so a validation failure (exit 2) emits ZERO TSV lines: piping it
  # straight into the read loop made a broken checker byte-indistinguishable from a clean tree.
  # This file's own idiom — reclaim_pass captures then greps a here-string, per change 0067's
  # no-pipefail-SIGPIPE rule. The prologue is `set -uo pipefail` (no -e), so `|| rc=$?` is safe.
  # The loop no longer runs in a pipeline subshell, so its three variables must be `local`.
  local out rc=0 check_id change_id message
  out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh \
    --changes-dir "$cd_dir" --metadata-branch "$metadata_branch" \
    --integration-branch "origin/$INTEGRATION_BRANCH" \
    --lease-ttl-hours "${RECLAIM_LEASE_TTL:-72}" ${adr_args[@]+"${adr_args[@]}"} 2>&2)" || rc=$?
  # An empty $out yields one blank line from <<<, already swallowed by the [ -n ] guard below.
  while IFS=$'\t' read -r check_id change_id message; do
    [ -n "$check_id" ] || continue
    echo "check $check_id $change_id $message"
  done <<<"$out"
  # ADDITIVE, never a replacement: findings print first, then the diagnostic. board-checks.sh's
  # --strict path already prints $FINDINGS and THEN exits 1, so the emit-then-fail shape is one
  # flag away from this caller. The exit code is the whole payload — the CAUSE stays on stderr,
  # where board-checks.sh already writes it and this function passes it through untouched. Same
  # contract as `board inline failed`: the line signals WHICH STEP failed, not why.
  [ "$rc" -eq 0 ] || printf 'health checks failed %s\n' "$rc"
  return 0
```

Also fix the **stale header comment** on the function itself — it currently reads "a clean tree (or a board-checks failure) prints nothing extra". Change that parenthetical to say a checker failure now prints `health checks failed <exit>`, keeping the warn-only claim (which is still exactly right).

- [ ] **Step 4: Update `scripts/docket-status.md`**

Locate each site **by its text, not by line number** — the anchors drift.

**(a) The real site — §7 "Health checks".** The sentence currently reads:

> Both are best-effort/warn-only: a
> clean tree, or a `board-checks.sh` failure, produces no extra output and never aborts the pass.

Rewrite so the warn-only posture survives but the false claim goes — a checker failure now emits `health checks failed <exit>` on stdout while the pass still continues.

**(b) The `## Output contract` table.** Add a row next to `learnings index failed`:

```
| `health checks failed <exit>` | The health pass's `board-checks.sh` invocation exited non-zero (`<exit>` is its status); its findings, if any, are still printed above this line. A **health-pass** line, deliberately outside the `board ` family — the cause stays on stderr. Warn-only: the pass still continues to `pass ok`. |
```

**(c) Leave the two *warn-only* restatements byte-untouched** — the *Failure postures* health-check bullet and its echo in *Invariants*. Warn-only is still exactly right and neither carries a stale claim. **Editing only *Failure postures*, the obvious target, would miss the real site entirely.**

- [ ] **Step 5: Run the tests to verify they pass**

```bash
bash tests/test_docket_status.sh 2>&1 | grep -cE '^NOT OK'
bash tests/test_board_checks.sh 2>&1 | grep -cE '^NOT OK'
```

Expected: `0` from both. Then confirm the check-id vocabulary is untouched — this task adds **no** new `BOARD_CHECK_IDS` entry:

```bash
bash tests/test_board_checks.sh 2>&1 | grep -ciE 'check.id|vocabulary'
```

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "chore(0144): report a board-checks.sh non-zero exit as 'health checks failed <exit>'"
```

---

### Task 3: 0146 — widen the config read-channel guard to all three config layers

**Files:**
- Modify: `tests/test_config_read_channel.sh` (on this feature branch)
- Modify: `/Users/homer/dev/docket/.docket/docs/adrs/0052-config-key-resolution-boundary.md` — **in the metadata worktree on branch `docket`, NOT on this feature branch.** See Step 5.

**Interfaces:**
- Consumes: nothing from Tasks 1–2.
- Produces: nothing later tasks rely on.

**Background.** `tests/test_config_read_channel.sh` is ADR-0052's prose-side enforcer: every occurrence of the config filename in `skills/**/*.md` must carry a same-line class marker or the suite fails. The scanned token is exactly `.docket.yml`, but docket documents **two more layers** a skill could just as wrongly be instructed to read: the machine-local `<repo>/.docket.local.yml` and the user-level `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. ADR-0052's rule is about the config *file*, not one of its three filenames — so an unmarked instruction to read either sibling layer passes today. That is a genuine fail-open.

**0146's spec recorded `depends_on: [120]` because `tests/test_config_read_channel.sh` did not exist on `main`. That has since resolved** — 0120's PR #130 merged (commits `66103c51`, `cd5973b3` are on `origin/main`) and the file is present on this branch. No dependency remains.

**Audit re-verified during planning (the spec required this at reconcile, and the count is a finding, not a premise):**

| file | `.docket.local.yml` | `config.yml` |
|---|---|---|
| `skills/docket-convention/SKILL.md` | 1 | 1 |
| `skills/docket-convention/references/agent-layer.md` | 7 | 4 |

Thirteen occurrences, **all inside the two files the test already excludes**; **zero elsewhere in `skills/**`**. So the widening reclassifies nothing and requires zero new markers — it is pure fail-open closure. The zero-cost result is **load-bearing on those two exclusions**, which this change does not own; whoever narrows them owns the thirteen markers.

**The third token is bare `config.yml`, NOT `docket/config.yml`.** `skills/docket-convention/references/agent-layer.md` refers to the layer bare (``Every one of `config.yml`'s …``, and ``global `config.yml` sets `agent_harnesses:` ``) — "the global `config.yml`" is docket's **own house phrasing**, so it is the likeliest spelling a future skill author would use. A path-qualified token would widen the guard to a layer while missing how that layer is actually named in-house — the same narrowness this change exists to fix. Bare `config.yml` also subsumes `docket/config.yml` and keeps the set overlap-free (`config.yml` **is** a substring of `docket/config.yml`, so a set holding both would double-count).

**Do NOT tighten the substring occurrence test.** The current superstring behavior over-reports (fail-safe: it can never admit an unmarked real occurrence), and there is no superstring occurrence in the tree today. The obvious boundary-anchored tightening is actively worse — `grep -oE '(^|[^A-Za-z0-9_.-])\.docket\.yml($|[^A-Za-z0-9_-])'` **consumes the boundary character**, so `see .docket.yml .docket.yml here` counts 1, not 2, which is an undercount and therefore a per-line fail-open. Record the limitation in the file's header comment instead.

- [ ] **Step 1: Write the failing tests**

In `tests/test_config_read_channel.sh`, append fixtures **(h)–(m)** after the existing `(g)` block and before the final `if [ "$fail" = 0 ]` line. All fixtures drive `scan_tree` — never a re-implementation (the file's own stated rule).

```bash
# (h) an unmarked .docket.local.yml occurrence => REJECTED. This is the exact fail-open change 0146
# closes: reproduced end-to-end on 0120's branch, an unmarked "Read `.docket.local.yml` yourself and
# parse the `finalize:` block" line left the suite PASSing.
mkfix h
printf 'read `%s` yourself and parse the `finalize:` block\n' "$TOKEN_LOCAL" > "$tmp/h/skills/x/SKILL.md"
outh="$(scan_tree "$tmp/h")"
assert "mutation (h): an unmarked .docket.local.yml occurrence is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outh"'

# (i) an unmarked bare config.yml occurrence => REJECTED.
mkfix i
printf 'the global `%s` sets it\n' "$TOKEN_GLOBAL" > "$tmp/i/skills/x/SKILL.md"
outi="$(scan_tree "$tmp/i")"
assert "mutation (i): an unmarked bare config.yml occurrence is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outi"'

# (j) a MARKED occurrence of each NEW token => classified ok. Proves both new tokens reach the
# admissible arm and are not reject-only.
mkfix j
{ printf 'never by parsing `%s` <!-- docket:config-read-channel: negative -->\n' "$TOKEN_LOCAL"
  printf 'never by parsing `%s` <!-- docket:config-read-channel: negative -->\n' "$TOKEN_GLOBAL"
} > "$tmp/j/skills/x/SKILL.md"
outj="$(scan_tree "$tmp/j")"
assert "mutation (j): marked occurrences of both new tokens are ACCEPTED" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outj")" ]'
assert "mutation (j) is non-vacuous: both new-token occurrences were actually classified" \
  '[ "$(grep -c -- "$(printf "^ok\t")" <<<"$outj")" = 2 ]'

# (k) a line carrying TWO DIFFERENT tokens with only ONE marker => REJECTED. Pins that the
# equal-count rule SUMS across the token set rather than short-circuiting on the first token
# that matches.
mkfix k
printf 'either `%s` or `%s` <!-- docket:config-read-channel: negative -->\n' \
  "$TOKEN" "$TOKEN_LOCAL" > "$tmp/k/skills/x/SKILL.md"
outk="$(scan_tree "$tmp/k")"
assert "mutation (k): two different tokens with one marker is REJECTED" \
  'grep -q -- "$(printf "^unclassified\tskills/x/SKILL.md\t1\t")" <<<"$outk"'

# (l) GROUND TRUTH FOR SUMMING: a line containing .docket.local.yml ONCE with exactly one marker
# => ok. This is the direct test that the token set counts it ONCE, not twice — and it is what
# actually proves the overlap property, rather than asserting a proxy for it. (`.docket.yml` is
# NOT a substring of `.docket.local.yml`, so the count is 1; a naive alternation that also
# matched the `.yml` tail would double-count and demand a phantom second marker.)
mkfix l
printf 'never by parsing `%s` <!-- docket:config-read-channel: negative -->\n' \
  "$TOKEN_LOCAL" > "$tmp/l/skills/x/SKILL.md"
outl="$(scan_tree "$tmp/l")"
assert "mutation (l): a single .docket.local.yml occurrence counts ONCE, not twice" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outl")" ] && [ "$(grep -c -- "$(printf "^ok\t")" <<<"$outl")" = 1 ]'

# (m) a line whose only match is the PATH-QUALIFIED docket/config.yml => counted ONCE (the bare
# token matches inside the path), with one marker => ok. Pins the subsumption decision: the token
# set holds bare `config.yml`, never `docket/config.yml`, precisely so this counts once.
mkfix m
printf 'never by parsing `docket/%s` <!-- docket:config-read-channel: negative -->\n' \
  "$TOKEN_GLOBAL" > "$tmp/m/skills/x/SKILL.md"
outm="$(scan_tree "$tmp/m")"
assert "mutation (m): a path-qualified docket/config.yml counts ONCE" \
  '[ -z "$(grep -- "$(printf "^unclassified\t")" <<<"$outm")" ] && [ "$(grep -c -- "$(printf "^ok\t")" <<<"$outm")" = 1 ]'

# OVERLAP INVARIANT. Assert directly that no two tokens in the set can co-match an OVERLAPPING
# region of a line — not merely that no token is a substring of another, which is necessary but
# INSUFFICIENT (a future token whose prefix is another token's suffix would satisfy non-substring
# and still double-count). Built by concatenating each ordered pair and requiring the summed count
# to equal exactly 2.
overlap_ok=1
for _t1 in "${TOKENS[@]}"; do
  for _t2 in "${TOKENS[@]}"; do
    _line="x${_t1}y${_t2}z"
    _sum=0
    for _t in "${TOKENS[@]}"; do
      _sum=$(( _sum + $(grep -oF -- "$_t" <<<"$_line" | wc -l | tr -d ' ') ))
    done
    [ "$_sum" -eq 2 ] || { overlap_ok=0; echo "overlap: <$_t1> + <$_t2> summed to $_sum"; }
  done
done
assert "0146: no two tokens in the set co-match an overlapping region (summing is exact)" \
  '[ "$overlap_ok" = 1 ]'

# POPULATION FLOOR on the token set itself: an accidental truncation to one token must not read as
# a clean tree (backstop-must-compute-not-reenumerate — a scan that reaches nothing is
# byte-identical to green).
assert "0146: the scanned token set has exactly three members" '[ "${#TOKENS[@]}" -eq 3 ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
bash tests/test_config_read_channel.sh 2>&1 | grep -E '^NOT OK'
```

Expected: the run **errors** — `TOKENS`, `TOKEN_LOCAL`, and `TOKEN_GLOBAL` are undefined under `set -u`. That is the correct starting state. Once Step 3 defines them but before the two match sites widen, expect **(h)**, **(i)**, **(j)**, and **(k)** to fail while **(l)**, **(m)**, the overlap invariant, and the three-member floor pass.

- [ ] **Step 3: Widen the token set and BOTH match sites**

Replace the single `TOKEN` definition, keeping the by-parts construction so this file's own source is not an occurrence of what it scans for:

```bash
# Built by parts so this file's own source is not an occurrence of the tokens it scans for.
TOKEN=".docket$(printf '')".yml
TOKEN_LOCAL=".docket$(printf '')".local.yml
TOKEN_GLOBAL="config$(printf '')".yml
# ADR-0052's rule is about the config FILE, not one of its three filenames (change 0146). docket
# documents three layers a skill could just as wrongly be told to read: the repo-committed
# .docket.yml, the machine-local sibling, and the user-level global. The third token is BARE, not
# path-qualified: docket's own house phrasing is "the global `config.yml`", which is therefore the
# likeliest spelling a future author would use — and the bare form subsumes the qualified one while
# keeping the set overlap-free (the qualified form CONTAINS the bare one, so a set holding both
# would double-count every path-qualified occurrence).
TOKENS=("$TOKEN" "$TOKEN_LOCAL" "$TOKEN_GLOBAL")
```

Then widen **both** match sites inside `scan_tree`. **Widening only the counter while leaving the prefilter single-token preserves the exact fail-open being closed** — a line mentioning only `.docket.local.yml` would never reach the counter.

```bash
      # PREFILTER — must cover the whole token set. Widening only the counter below while leaving
      # this single-token silently preserves the fail-open change 0146 closes.
      _hit=0
      for _t in "${TOKENS[@]}"; do
        case "$line" in *"$_t"*) _hit=1; break ;; esac
      done
      [ "$_hit" = 1 ] || continue
      # Count OCCURRENCES across the whole token set and MARKERS on this same line and require them
      # equal — a line with 2 occurrences and 1 marker is reported unclassified, not admitted on the
      # strength of the one marker it happens to carry (Finding 1). The counts SUM exactly because
      # no two tokens can co-match an overlapping region; that property is asserted directly below,
      # and backed by ground-truth fixtures (l) and (m) rather than by the structural assert alone.
      occ=0
      for _t in "${TOKENS[@]}"; do
        occ=$(( occ + $(grep -oF -- "$_t" <<<"$line" | wc -l | tr -d ' ') ))
      done
```

Declare `_hit` and `_t` in `scan_tree`'s existing `local` line. **The per-line equal-count rule, the marker syntax, and the admissible class set all stay exactly as 0120 shipped them** — only the token set moves.

**One shared class vocabulary for all three filenames.** `write-back` and `negative` describe *what the line says about the file*, not *which file* — the classes are orthogonal to the layer, and ADR-0052 draws no per-layer distinction. Do **not** add a machine-local-only class.

- [ ] **Step 4: Record the occurrence-test limitation in the header comment**

Add to `tests/test_config_read_channel.sh`'s header block, so the next reader does not mistake it for an oversight:

```
# THE OCCURRENCE TEST IS A SUBSTRING MATCH, DELIBERATELY (change 0146). `myconfig.docket.yml.bak`
# counts as an occurrence. This over-reports — it can demand a marker on a line that arguably needs
# none — and that direction is fail-SAFE: it can never admit an unmarked real occurrence, which is
# the only failure ADR-0052 cares about. It is also unreachable today (no superstring occurrence
# exists in skills/**). The obvious tightening is actively worse: a boundary-anchored
# `grep -oE '(^|[^A-Za-z0-9_.-])<tok>($|[^A-Za-z0-9_-])'` CONSUMES the boundary character, so
# `see <tok> <tok> here` counts 1, not 2 — an undercount, which makes markers == occ satisfiable
# with fewer markers than occurrences: the per-line fail-open Finding 1 closed.
```

- [ ] **Step 5: Add a dated `## Update` to ADR-0052 — on the metadata branch, NOT here**

ADR-0052's `## Decision` is generic and its `## Context` already names `.docket.local.yml` and `~/.config/docket/config.yml` as affected layers, so widening the enforcer to match is **not a new decision** — no new ADR. But ADR-0052's 2026-07-27 `## Update` describes the enforcer as requiring *every `.docket.yml` occurrence* to carry a marker, and that record goes stale here.

**An Accepted ADR is immutable except its status line, so the instrument is a NEW dated `## Update`, never an edit to the existing one.**

**Do not write this in the feature worktree.** ADRs live on the metadata branch (`docket`), not on the code line — the feature branch never modifies docket metadata. Append the following to `/Users/homer/dev/docket/.docket/docs/adrs/0052-config-key-resolution-boundary.md`, then commit and push it **in the `.docket/` metadata worktree on `docket`**:

```markdown
## Update — 2026-07-28 (change 0146)

The prose-side enforcer added by change 0120 scanned exactly one filename, `.docket.yml`. This
decision's subject is the config **file**, not one of its three filenames — docket documents two
more layers a skill could just as wrongly be instructed to read: the machine-local
`<repo>/.docket.local.yml` and the user-level `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. An
unmarked instruction to read either sibling layer therefore passed the guard.

Change 0146 widens `tests/test_config_read_channel.sh` to a three-token set — `.docket.yml`,
`.docket.local.yml`, and bare `config.yml` — at **both** of `scan_tree`'s match sites (the per-line
prefilter and the counting pass; widening only the counter would have preserved the fail-open). The
third token is bare rather than path-qualified because "the global `config.yml`" is docket's own
in-house phrasing and therefore the likeliest spelling; the bare form also subsumes
`docket/config.yml` while keeping the token set overlap-free, so per-line occurrence counts still
sum exactly.

The three filenames share **one** class vocabulary: `write-back` and `negative` describe what a line
says about the file, which is layer-independent, and this decision draws no per-layer distinction.
The per-line equal-count rule, the marker syntax, and the admissible class set are unchanged from
change 0120. The widening reclassified nothing — all thirteen sibling-layer occurrences in
`skills/**` sit inside the two `docket-convention` exclusions change 0120 already declared.

Nothing in `## Decision`, `## Enforcement`, or `## Consequences` changes.
```

The change already carries `adrs: [52]`, so the note ships with it.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
bash tests/test_config_read_channel.sh 2>&1 | tail -3
```

Expected: `PASS`, 0 `NOT OK`. Then confirm the widening reclassified nothing in the real tree — this must still be true, and it is the whole zero-cost claim:

```bash
bash tests/test_config_read_channel.sh 2>&1 | grep -E 'every occurrence in a scanned skill file is classified'
```

Expected: `ok - …` with no trailing unclassified list.

- [ ] **Step 7: Commit**

```bash
git add tests/test_config_read_channel.sh
git commit -m "fix(0146): widen the config read-channel guard to all three config layers"
```

Then, separately, in `/Users/homer/dev/docket/.docket` on branch `docket`:

```bash
git -C /Users/homer/dev/docket/.docket add docs/adrs/0052-config-key-resolution-boundary.md
git -C /Users/homer/dev/docket/.docket commit -m "docs(0146): ADR-0052 update — enforcer widened to all three config layers"
git -C /Users/homer/dev/docket/.docket push origin HEAD:docket
```

---

### Task 4: 0148 — delete the two unfalsifiable `-z "$DOCKET_BASH_PATH"` asserts

**Files:**
- Modify: `tests/test_docket_config.sh`

**Interfaces:**
- Consumes: nothing from Tasks 1–3.
- Produces: **a changed baseline that Task 5 depends on.** After this task the assert count is **375** (from 381) and the guard `TOTALS` line must still read `sites=64 exempt=3 ok=61 viol=0`. Task 5 derives its proportional floor from those numbers, so Task 5 **must** run after this one.

**Background.** Two asserts of the form `assert "… captured value remains clear" '[ -z "$DOCKET_BASH_PATH" ]'` are each preceded, a few lines above, by a bare `DOCKET_BASH_PATH=""`. **That seed is the whole defect** — it forces the asserted value to the value the assert demands, so neither assert can ever fail.

**Two corrections to the obvious framing, both verified empirically:**

- **The vacuity is not "nothing can change the variable."** The nearest preceding `eval` — the `require_pr_approval` fixture's — *does* set `DOCKET_BASH_PATH`. Remove the `DOCKET_BASH_PATH=""` line and the assert **reddens**. The seed, not the absence of an eval, is what makes it unfalsifiable.
- **Change 0126's guard does see these asserts.** Its need-windows *tile the file* (`lo = SL[k]; hi = (k < ns ? SL[k+1] - 1 : n)`), so the `require_pr_approval` site's window extends down to the odd-runtime block, sweeping in both reads. That is exactly why the poison exists.

**The property is already proven by the assert one line above.** Both sites sit on a **fail-closed** path where the resolver aborts and emits nothing, and each already asserts `[ -z "$runtime_invalid_out" ]` / `[ -z "$runtime_absent_out" ]`. `export is empty` is the **sole channel**: `docket-config.sh --export` writes shell assignments to stdout and nothing else, and a subprocess has no channel into the parent's environment — so an empty export admits no variable. The per-variable claim is *implied*, not additive.

**Rejected, and it matters that it is rejected:** do **not** insert an `eval "$runtime_invalid_out"` to make the asserts falsifiable and pull them into 0126's guard. On this path `$out` is asserted empty, so the inserted eval is a **provable no-op added solely to satisfy a guard's site-detection heuristic** — guard-gaming. It would raise the site count and the `ok` tally while adding zero real coverage.

- [ ] **Step 1: Confirm the baseline**

```bash
cd /Users/homer/dev/docket/.worktrees/roll-up-the-seven-build-ready-changes-into-one-branch
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "asserts=$(grep -cE '^(ok - |NOT OK - )' <<<"$out")  failing=$(grep -c '^NOT OK - ' <<<"$out")"
grep -E '^TOTALS' <<<"$out"
```

Expected exactly: `asserts=381  failing=0` and `TOTALS sites=64 exempt=3 ok=61 viol=0`. **If these differ, stop** — a neighbour task moved the file and this task's post-conditions must be re-derived.

- [ ] **Step 2: Run the seed sweep in all three spellings**

```bash
grep -nE '^[[:space:]]*[A-Z_][A-Z0-9_]*=""' tests/test_docket_config.sh
grep -nE "^[[:space:]]*[A-Z_][A-Z0-9_]*=''" tests/test_docket_config.sh
grep -nE '^[[:space:]]*[A-Z_][A-Z0-9_]*=$'  tests/test_docket_config.sh
```

Expected: **exactly two hits, both `DOCKET_BASH_PATH=""`**, and nothing from the other two spellings. The first regex is narrower than the defect class — it would miss `VAR=''` and bare `VAR=` — which is why all three run before the sweep is declared complete.

- [ ] **Step 3: Delete the asserts, their seeds, and the now-dead poison clause**

Four edits in `tests/test_docket_config.sh`:

**(a)** In the `for runtime_case in relative missing nonexec legacy notbash` loop, delete the line

```bash
  assert "0132 runtime invalid $runtime_case: captured value remains clear" '[ -z "$DOCKET_BASH_PATH" ]'
```

and the `DOCKET_BASH_PATH=""` seed a few lines above it (inside the same loop). One deletion removes all five instances at once — the argument for deletion is structural and does not vary by case.

**(b)** In the runtime-absent block, delete

```bash
assert "0132 runtime absent: captured value remains clear" '[ -z "$DOCKET_BASH_PATH" ]'
```

and its `DOCKET_BASH_PATH=""` seed.

**(c)** At **each** deletion site, add a short comment recording the sole-channel reasoning, so the asserts are not re-added by someone noticing a "missing" per-variable check:

```bash
  # No per-variable `[ -z "$DOCKET_BASH_PATH" ]` assert here (change 0148). `export is empty` one
  # line above is the SOLE CHANNEL: docket-config.sh --export writes shell assignments to stdout and
  # nothing else, and a subprocess has no channel into the parent's environment — so an empty export
  # admits NO exported variable, and a per-variable restatement is implied, not additive. The
  # deleted assert was also unfalsifiable: a `DOCKET_BASH_PATH=""` seed above it forced the value the
  # assert demanded. Do NOT "repair" it by inserting an `eval "$out"` on a provably-empty export —
  # that is a no-op added solely to satisfy a guard's site-detection heuristic.
```

**(d)** In the `require_pr_approval` fixture, delete the **`DOCKET_BASH_PATH=__poison__` CLAUSE** and its six-line "Load-bearing; do not delete" comment. Both exist **only** to satisfy the asserts being removed, so leaving them behind a comment that misstates why they are there reproduces this change's own defect.

**CLAUSE, NOT LINE.** The line is compound:

```bash
DOCKET_BASH_PATH=__poison__; FINALIZE_REQUIRE_PR_APPROVAL=__poison__
```

`FINALIZE_REQUIRE_PR_APPROVAL=__poison__` is load-bearing for this fixture's own assert. Deleting the whole line reddens the guard with `SITE … viol FINALIZE_REQUIRE_PR_APPROVAL` (the preceding site's poison is outside the cleared window). The line must become:

```bash
FINALIZE_REQUIRE_PR_APPROVAL=__poison__
```

- [ ] **Step 4: Add the guard post-condition asserts**

The correct post-condition is **not** a `t_exempt` tripwire — measured, `t_exempt` is 3 before and 3 after, so a tripwire on it would pass while certifying nothing. In section (T), alongside the existing floors, add:

```bash
# Change 0148 post-conditions. `t_exempt` is deliberately NOT a tripwire here: it measured 3 both
# before and after the deletions, so an assert on its movement would pass while certifying nothing.
# The real invariants are that the guard still proves something everywhere, and that the
# require_pr_approval site kept a NON-EMPTY need set after losing its DOCKET_BASH_PATH poison —
# it retains FINALIZE_REQUIRE_PR_APPROVAL (an emitted key), so it does not fall into the exempt
# bucket and t_exempt legitimately stays 3.
assert "0148: the require_pr_approval site still has a non-empty need set" \
  '/usr/bin/grep -qE "^SITE .* (ok|viol)" <<<"$(/usr/bin/grep -F "FINALIZE_REQUIRE_PR_APPROVAL" <<<"$t_out")"'
```

Keep the existing `t_viol -eq 0`, `t_sites >= 60`, `t_keycount >= 20`, and `t_exempt -le 5` asserts untouched — **Task 5 replaces the last one; this task does not.**

**Adjust the assert to the guard's real report format.** Read the `prelude_report` awk's `SITE` output shape first (`print "SITE " SL[k] " exempt"` and its `ok`/`viol` siblings) and express "this site is not exempt" through whatever that format actually emits. Do not ship the line above unverified.

- [ ] **Step 5: Verify the deletion is complete and the survivors carry the load**

```bash
out="$(bash tests/test_docket_config.sh 2>&1)"
echo "asserts=$(grep -cE '^(ok - |NOT OK - )' <<<"$out")  failing=$(grep -c '^NOT OK - ' <<<"$out")"
grep -E '^TOTALS' <<<"$out"
```

Expected: **`asserts=375`** (381 − 6, plus 1 added in Step 4 → confirm the arithmetic against what you actually added), `failing=0`, and `TOTALS sites=64 exempt=3 ok=61 viol=0` **unchanged**.

**Positive proof the poison deletion was complete** — re-run the mutation that motivated it. With the asserts gone, removing the `require_pr_approval` site's `DOCKET_BASH_PATH` poison must now be **green** (it is genuinely unneeded). Since Step 3(d) already removed it, this is satisfied by the run above returning `viol=0`. Confirm the inverse still bites by temporarily deleting the *surviving* `FINALIZE_REQUIRE_PR_APPROVAL=__poison__` clause and checking the guard reddens with `SITE … viol FINALIZE_REQUIRE_PR_APPROVAL`, then restore it.

**Mutation-test the survivors** — `export is empty` now carries the whole load, so it must be shown to carry it. Temporarily patch `scripts/docket-config.sh` to emit a non-empty export on the invalid path, and confirm `export is empty` reddens for **all five loop cases plus the absent case** (six failures). Restore the script afterwards and re-confirm green.

- [ ] **Step 6: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "chore(0148): delete two unfalsifiable -z asserts, their seeds, and the dead poison clause"
```

---

### Task 5: 0149 — replace the guard's absolute exemption ceiling with a proportional floor

**Files:**
- Modify: `tests/test_docket_config.sh`, **section (T) only**

**Interfaces:**
- Consumes: **Task 4's post-deletion tree.** The floor is derived against `TOTALS` *after* 0148 lands. The numbers printed in 0149's own spec predate Task 4 — re-measure, do not copy.
- Produces: nothing later tasks rely on.

**Background.** Change 0126's correspondence guard defends itself against a degenerate key set with `assert "0126 T: exemptions stay a rounding error (guard not degenerate)" '[ "$t_exempt" -le 5 ]'`. **The bound is absolute, not proportional.** A fixed ceiling of 5 against a real value of 3 leaves two sites of headroom; as the file grows the headroom does not, so several legitimately-exempt fixtures landing together trip it. The failure mode when it ages is a loud false red — which is why it was parked rather than fixed.

**The suspected partial-rename gap does not exist as described — do not build for it.** It was tested rather than reasoned about:

- Renaming **one** emitted key (`METADATA_BRANCH` → `METADATA_BRANCHZ`) turns **four ordinary asserts red**, and `TOTALS` comes back **byte-identical**: `sites=64 exempt=3 ok=61 viol=0`. `exempt` does not rise "slightly"; it does not rise **at all**. `$t_keys` is derived from the resolver's own output, so the rename swaps the new name in and drops the old — the scanner really does lose it — but `exempt` holds because no eval window reads that key *exclusively*. A rename has to be near-total before `exempt` moves.
- Renaming **five** keys turns 14 asserts red and then kills the run outright: `LEARNINGS_CAP: unbound variable` under `set -u`. Section (T) never executes, so there is no `TOTALS` line to be vacuously green in.

So the failure the ceiling reached for is already caught twice over, by the ordinary fixtures and by `set -u`. **Do not add a reverse key-coverage pass**: under the guard's own `\$\{?KEY` read-shape `t_unread` is **3, not 0** (`AUTO_CAPTURE_ENABLED`, `AUTO_CAPTURE_TYPES`, `CHANGE_TYPES` are tested only through string-literal assertions), it does not catch an empty key set (`for (k in KEY)` over an empty array iterates zero times), and an exact-zero form would couple this guard to every assert deletion anywhere in the file — in direct conflict with Task 4.

- [ ] **Step 1: Re-measure the post-0148 baseline**

```bash
cd /Users/homer/dev/docket/.worktrees/roll-up-the-seven-build-ready-changes-into-one-branch
bash tests/test_docket_config.sh 2>&1 | grep -E '^TOTALS'
```

Record the actual `sites=` and `ok=` values. Expected `TOTALS sites=64 exempt=3 ok=61 viol=0`, but **use what you measure**, not what is written here. The whole point of the ordering is that this number is derived after Task 4.

- [ ] **Step 2: Write the failing test**

In section (T), **replace** the ceiling assert (and its six-line comment) with the proportional floor. First add the `t_ok` extractor beside the existing `t_sites`/`t_viol`/`t_exempt` ones, matching their shape:

```bash
t_ok="$(printf '%s\n' "$t_out" | sed -n 's/^TOTALS .* ok=\([0-9]*\) .*/\1/p')"
```

**Do NOT add a field to the `TOTALS` line.** `t_viol`'s extractor is end-anchored (`… viol=\([0-9]*\)$`), so appending anything after `viol=` silently empties `t_viol`, and `[ "" -eq 0 ]` errors the suite red. Nothing here needs a new field.

Then replace the assert:

```bash
# Coverage floor — the twin of the keycount floor, and the successor to change 0126's absolute
# `t_exempt <= 5` ceiling (change 0149). An EMPTY key set is caught by t_keycount above; a WRONG one
# would make every site "exempt by derivation" and the guard would report viol=0 having checked
# nothing. The old ceiling was ABSOLUTE: a fixed 5 against a real 3 left two sites of headroom that
# did not grow with the file, so several legitimately-exempt fixtures landing together would trip it
# — a loud false red as it aged.
#
# A floor on `ok` is preferred to a ratio on `exempt` for two reasons: it measures the property that
# matters (coverage PROVEN) rather than its complement, and because `viol` must independently be 0,
# the floor bounds `exempt` without naming it. Note the arithmetic direction: `t_exempt * 5 <=
# t_sites` — the obvious "proportional" rewrite — would permit 12 exempt sites at 64, which is
# LOOSER than the absolute 5 it replaces. At today's 64 sites this floor permits 6 non-`ok` sites
# where the ceiling permitted 5: one site of immediate slack traded for slack that SCALES.
#
# MEASURED FINDING, recorded so the ceiling is not reinstated by someone re-deriving the original
# worry: this guard is NOT the rename detector. Renaming one emitted export key turns four ordinary
# asserts red while TOTALS comes back byte-identical (exempt does not move at all, because no eval
# window reads a key exclusively); renaming five aborts the run under `set -u` with an unbound
# variable before section (T) ever executes. Both cheaper layers already catch it.
assert "0126 T: the guard proved something at >=90% of sites (ok=$t_ok of $t_sites)" \
  '[ $(( t_ok * 10 )) -ge $(( t_sites * 9 )) ]'
```

**Keep the `t_exempt` extraction and the existing `TOTALS` print.** `t_exempt` is never printed directly — the printed `TOTALS` line carries `exempt=` independently — so after the swap the variable becomes diagnostic-only. **Do not add a print that does not exist today.** Everything else in section (T) stays byte-untouched.

**Threshold rationale, recorded so it is re-argued rather than re-guessed:** 90% against today's 95.3% (61/64). It absorbs three more non-`ok` sites now and scales. 95% would be tripped by one legitimately-exempt fixture, reproducing today's brittleness; 75% leaves enough slack to hide a substantial partial degeneracy.

- [ ] **Step 3: Run the mutation mandate**

Four checks. Compute where possible rather than re-running the suite 64 times.

```bash
# (i) The floor's slack is exactly what the design claims: ok may fall to 58 of 64, reddens at 57.
for k in 59 58 57 56; do
  if [ $(( k * 10 )) -ge $(( 64 * 9 )) ]; then echo "ok=$k -> PASSES"; else echo "ok=$k -> REDDENS"; fi
done
```

Expected: `59 -> PASSES`, `58 -> PASSES`, `57 -> REDDENS`, `56 -> REDDENS`. (Substitute your measured `t_sites` if it is not 64.)

```bash
# (ii) Force many sites exempt by neutralising the `need` accumulation in prelude_report, then
# confirm the ok floor FIRES where the old ceiling would have.
```

Patch `prelude_report`'s awk so `need` never accumulates, run the file, and confirm the new floor assert reddens. Restore afterwards.

```bash
# (iii) The retired assert's own mutation still has a detector: with the ceiling gone, an
# all-exempt run must STILL redden — via the ok floor, not via t_keycount.
```

Confirm the failure in (ii) is reported by the `>=90% of sites` assert specifically, and that `t_keycount >= 20` is still green in that run — otherwise the ceiling's coverage was silently inherited by the wrong assert.

```bash
# (iv) Full file, unchanged TOTALS, no other assert changes verdict.
out="$(bash tests/test_docket_config.sh 2>&1)"
grep -E '^TOTALS' <<<"$out"; grep -c '^NOT OK - ' <<<"$out"
```

Expected: `TOTALS` identical to Step 1's measurement, and `0` failing.

- [ ] **Step 4: Commit**

```bash
git add tests/test_docket_config.sh
git commit -m "chore(0149): replace the prelude guard's absolute exemption ceiling with a proportional ok floor"
```

---

### Task 6: 0152a — pin `ensure-docket-env.sh`'s five diagnostics before moving any detection

**Files:**
- Modify: `tests/test_ensure_docket_env.sh`

**Interfaces:**
- Consumes: nothing from Tasks 1–5.
- Produces: five message asserts and two negative fixtures that Task 7 must keep green. Task 7's whole claim is "message-preserving", and this task is what makes that claim falsifiable.

**Background — this is the change's actual point and it is easy to miss.** `tests/test_ensure_docket_env.sh` asserts **none** of the five `die` strings; its only negative cases are a relative path and a newline path, both exit-code-only, and its runtime fixture hardcodes `GNU bash, version 5.2.0(1)-release (test)`. So "no user-visible text changes" is currently **unfalsifiable**, and post-consolidation, breaking the library's major check would leave this file **fully green**.

The five strings, verbatim from `scripts/ensure-docket-env.sh`:

| library token | existing `die` string |
|---|---|
| `not-absolute` | `DOCKET_BASH_PATH must be an absolute path` |
| `not-executable` | `DOCKET_BASH_PATH is not executable: $BASH_VALUE` |
| `no-version` | `DOCKET_BASH_PATH cannot report its version` |
| `not-gnu-bash` | `DOCKET_BASH_PATH is not GNU Bash` |
| `old-major` | `DOCKET_BASH_PATH must be Bash 4 or newer` |

- [ ] **Step 1: Write the pinning tests**

Append to `tests/test_ensure_docket_env.sh`, before its final `exit $fail`. These are **characterization tests: they pass BOTH before and after Task 7.** That is the point — do not apply a red-first mutation mandate to them.

```bash
# --- change 0152: pin all five runtime diagnostics BEFORE the detection moves to the shared
# library. The suite asserted NONE of them, which made "the consolidation is message-preserving"
# unfalsifiable exactly where it matters. These are CHARACTERIZATION tests: green before and after.
mkfake(){ # mkfake <path> <first --version line> [noexec]
  mkdir -p "$(dirname "$1")"
  cat > "$1" <<EOF
#!/bin/sh
[ "\$#" -eq 1 ] && [ "\$1" = --version ] || exit 42
printf '%s\n' '$2'
EOF
  if [ "${3-}" = noexec ]; then chmod -x "$1"; else chmod +x "$1"; fi
}
FAKE_BIN="$(mktemp -d)"; _tmpdirs+=("$FAKE_BIN")
mkfake "$FAKE_BIN/legacy"  'GNU bash, version 3.2.57(1)-release (fake-legacy)'
mkfake "$FAKE_BIN/notbash" 'zsh 5.9 (arm64-apple-darwin)'
mkfake "$FAKE_BIN/noexec"  'GNU bash, version 5.2.0(1)-release (test)' noexec
printf '#!/bin/sh\nexit 7\n' > "$FAKE_BIN/novers"; chmod +x "$FAKE_BIN/novers"

diag(){ # diag <DOCKET_BASH_PATH value> -> stderr+stdout of one rejected run
  local h; h="$(mktemp -d)"; _tmpdirs+=("$h")
  HOME="$h" DOCKET_HARNESS_ROOT="$h" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$1" \
    bash "$SCRIPT" 2>&1
}

assert "0152 diagnostic: a relative path names 'must be an absolute path'" \
  'grep -qF "DOCKET_BASH_PATH must be an absolute path" <<<"$(diag relative)"'
assert "0152 diagnostic: a missing file names 'is not executable' with the path" \
  'grep -qF "DOCKET_BASH_PATH is not executable: $FAKE_BIN/does-not-exist" <<<"$(diag "$FAKE_BIN/does-not-exist")"'
assert "0152 diagnostic: a non-executable file names 'is not executable' with the path" \
  'grep -qF "DOCKET_BASH_PATH is not executable: $FAKE_BIN/noexec" <<<"$(diag "$FAKE_BIN/noexec")"'
assert "0152 diagnostic: a binary that cannot report a version names 'cannot report its version'" \
  'grep -qF "DOCKET_BASH_PATH cannot report its version" <<<"$(diag "$FAKE_BIN/novers")"'
assert "0152 diagnostic: a non-GNU binary names 'is not GNU Bash'" \
  'grep -qF "DOCKET_BASH_PATH is not GNU Bash" <<<"$(diag "$FAKE_BIN/notbash")"'
assert "0152 diagnostic: a Bash 3 binary names 'must be Bash 4 or newer'" \
  'grep -qF "DOCKET_BASH_PATH must be Bash 4 or newer" <<<"$(diag "$FAKE_BIN/legacy")"'

# NEGATIVE FIXTURES — the coverage gap this change exists to close. Routing through the library does
# NOT by itself give this file coverage: without these two cases, breaking the library's major check
# would leave tests/test_ensure_docket_env.sh fully green (green-suite-untested-branch).
assert "0152 negative: a Bash 3.2 runtime is rejected non-zero" \
  '! HOME="$(mktemp -d)" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/legacy" bash "$SCRIPT" >/dev/null 2>&1'
assert "0152 negative: a non-GNU runtime is rejected non-zero" \
  '! HOME="$(mktemp -d)" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/notbash" bash "$SCRIPT" >/dev/null 2>&1'

# Neither rejection may touch the profile.
h152="$(mktemp -d)"; _tmpdirs+=("$h152"); printf '# keep\n' > "$h152/.zshenv"
HOME="$h152" DOCKET_HARNESS_ROOT="$h152" DOCKET_TARGET_SHELL=zsh DOCKET_BASH_PATH="$FAKE_BIN/legacy" \
  bash "$SCRIPT" >/dev/null 2>&1
assert "0152 negative: a rejected legacy runtime leaves the profile untouched" \
  '[ "$(cat "$h152/.zshenv")" = "# keep" ]'
```

**Note:** `_tmpdirs` and `$SCRIPT` are this file's existing conventions — confirm before use. `mkfake` duplicates `tests/test_docket_runtime_lib.sh`'s `fake_bash`; that duplication across two independent test files is acceptable and is **not** the duplication this change removes.

- [ ] **Step 2: Run to verify they pass NOW**

```bash
bash tests/test_ensure_docket_env.sh 2>&1 | grep -E '^NOT OK'
```

Expected: **no output** — every new assert passes against the current hand-rolled validator. **If any fails, stop**: the string you pinned is not the string the script emits, and Task 7 would then "preserve" the wrong text.

- [ ] **Step 3: Commit**

```bash
git add tests/test_ensure_docket_env.sh
git commit -m "test(0152): pin ensure-docket-env.sh's five runtime diagnostics and add negative fixtures"
```

---

### Task 7: 0152b — route `ensure-docket-env.sh` through the shared runtime library

**Files:**
- Modify: `scripts/ensure-docket-env.sh`

**Interfaces:**
- Consumes: Task 6's five message pins and two negative fixtures — all must stay green.
- Produces: `scripts/ensure-docket-env.sh` as a library caller. Task 8's equivalence guard and library-header correction depend on this having landed.

**Background.** `scripts/ensure-docket-env.sh`'s five hand-rolled lines — absolute-path `case`, `-x`, `--version` capture, the `'GNU bash, version '*` banner match, and the `>= 4` major parse — are byte-equivalent to `docket_runtime_validate_bash`. The fit is exact by design: the library "returns a machine-readable reason token instead of printing a message" precisely so "every user-facing diagnostic stays in the caller." **The mapping is 1:1 and total across all five tokens, so this task touches `scripts/lib/docket-runtime.sh` not at all** — which is also what keeps it disjoint from Task 9.

**`scripts/docket.sh`'s prologue copy STAYS** — it is a documented bootstrap exception, not a duplicate. Task 8 documents it. Do not consolidate it here.

- [ ] **Step 1: Source the library using this file's OWN path variable**

`scripts/ensure-docket-env.sh` defines `HERE`, **not** `SELF_DIR`, and it runs `set -uo pipefail` — so a copied `$SELF_DIR` is an unbound-variable abort on the consolidation's first line. The two existing precedents differ from each other (`install.sh` uses `$SCRIPT_DIR/scripts/lib/…`; `ensure-global-config.sh` uses `$SELF_DIR/lib/…`), so **neither can be copied verbatim.**

Immediately after the `HERE=` assignment, add:

```bash
# shellcheck source=lib/docket-runtime.sh
. "$HERE/lib/docket-runtime.sh"
```

- [ ] **Step 2: Replace the five hand-rolled validator lines**

Replace this block —

```bash
case "$BASH_VALUE" in /*) ;; *) die "DOCKET_BASH_PATH must be an absolute path" ;; esac
[ -x "$BASH_VALUE" ] || die "DOCKET_BASH_PATH is not executable: $BASH_VALUE"
_version="$(LC_ALL=C "$BASH_VALUE" --version 2>/dev/null)" || die "DOCKET_BASH_PATH cannot report its version"
_first="${_version%%$'\n'*}"
case "$_first" in 'GNU bash, version '*) ;; *) die "DOCKET_BASH_PATH is not GNU Bash" ;; esac
_major="$(sed -nE 's/^GNU bash, version ([0-9]+)\..*/\1/p' <<<"$_first")"
[[ "$_major" =~ ^[0-9]+$ ]] && [ "$_major" -ge 4 ] || die "DOCKET_BASH_PATH must be Bash 4 or newer"
```

— with a caller-side dispatch on the library's reason token. **Every `die` string is preserved verbatim; only the DETECTION moves.**

```bash
# Detection is delegated to the shared library (change 0152); the diagnostics stay here, which is
# exactly why docket_runtime_validate_bash returns a reason token instead of printing a message.
# The guarded capture idiom matches scripts/docket-config.sh. This caller reads only line 1 — all
# five die strings interpolate $BASH_VALUE, never the version — so nothing here depends on the
# guard; it matters where a caller consumes line 2, which is why the library documents it.
_probe="$(docket_runtime_validate_bash "$BASH_VALUE"; printf 'x')"; _probe="${_probe%x}"
_reason="${_probe%%$'\n'*}"
case "$_reason" in
  ok) ;;
  not-absolute)   die "DOCKET_BASH_PATH must be an absolute path" ;;
  not-executable) die "DOCKET_BASH_PATH is not executable: $BASH_VALUE" ;;
  no-version)     die "DOCKET_BASH_PATH cannot report its version" ;;
  not-gnu-bash)   die "DOCKET_BASH_PATH is not GNU Bash" ;;
  old-major)      die "DOCKET_BASH_PATH must be Bash 4 or newer" ;;
  *)              die "DOCKET_BASH_PATH validation returned an unrecognized result '$_reason'" ;;
esac
```

The library's one collapse — an unparseable major folded into `old-major` — is a collapse this file **already makes**, so no caller variance is flattened.

- [ ] **Step 3: Fold the fourth duplicate in the same file**

`validate_literal_path` opens with `case "$1" in *$'\n'*|*$'\r'*)` — which is `docket_runtime_serializable` **verbatim**. A change whose Problem statement is "three implementations, not one" must not consolidate five lines and leave an exact fourth copy two functions away. Following the precedent one file over (`ensure-global-config.sh`'s `validate_serializable_path(){ docket_runtime_serializable "$1"; }`):

```bash
# Detection in the library, the label-bearing message caller-side — same split as the validator
# above (change 0152).
validate_literal_path(){ docket_runtime_serializable "$1" || die "$2 contains unsupported line-break characters"; }
```

- [ ] **Step 4: Run the tests to verify they still pass**

```bash
bash tests/test_ensure_docket_env.sh 2>&1 | grep -E '^NOT OK'
```

Expected: **no output.** Every Task 6 pin still green — that is the message-preserving proof.

- [ ] **Step 5: Run the mutation that this task exists to enable**

This is the real oracle. Break the library's major check and confirm `tests/test_ensure_docket_env.sh` now **reddens** — it would have stayed fully green before this task.

```bash
cp scripts/lib/docket-runtime.sh /tmp/rt.bak
# neutralise the >= 4 test
perl -pi -e 's/\[ "\$_major" -ge 4 \]/[ "$_major" -ge 0 ]/' scripts/lib/docket-runtime.sh
bash tests/test_ensure_docket_env.sh 2>&1 | grep -cE '^NOT OK'
cp /tmp/rt.bak scripts/lib/docket-runtime.sh
```

Expected: a **non-zero** count under the mutation (at minimum the `must be Bash 4 or newer` diagnostic pin and the `Bash 3.2 runtime is rejected` negative fixture), and `0` again after restore. **If the count is 0 under the mutation, the routing did not take effect** — stop and re-check Step 2.

- [ ] **Step 6: Commit**

```bash
git add scripts/ensure-docket-env.sh
git commit -m "refactor(0152): route ensure-docket-env.sh's validator and serializability check through the library"
```

---

### Task 8: 0152c — equivalence guard, `ensure-global-config.sh` coverage, and the three doc sites

**Files:**
- Modify: `tests/test_bash_runtime_routing.sh` (the equivalence guard)
- Modify: `tests/test_ensure_global_config.sh` (a 3.2.57 fixture)
- Modify: `scripts/docket.sh` (prologue comment block only)
- Modify: `scripts/docket.md`
- Modify: `scripts/lib/docket-runtime.sh` (**header comment only** — no code)

**Interfaces:**
- Consumes: Task 7's routing (the library-header claim can only be corrected once `ensure-docket-env.sh` no longer has its own copy).
- Produces: the corrected library header. **Task 9 edits the same file's code** — keep this task's edit confined to the header comment so the two are disjoint by content.

**Background.** After Task 7 the surviving validator implementations are: the library (used by `install.sh`, `ensure-global-config.sh`, `docket-config.sh`, `ensure-docket-env.sh`) and `scripts/docket.sh`'s prologue.

**Why `scripts/docket.sh`'s copy stays — and the reason is NOT the obvious one.** The easy argument ("the library is Bash, the prologue is `sh`, it won't parse") is **false** and must not be recorded: the library is BOOTSTRAP-COMPATIBLE BY REQUIREMENT (every line runs under Bash 3.2.57), `local` is a `dash` builtin, and sourcing it from `/bin/sh` and from `dash` both work. A builder who tests the easy argument will find it wrong and may consolidate anyway. The real blockers:

- **`$'\n'` degrades to a literal under a non-bash `/bin/sh`.** `docket_runtime_validate_bash` splits its two-line payload with `_first="${_version%%$'\n'*}"`. Under `dash` that strips nothing, so the payload's second line becomes the entire multi-line `--version` blob — the documented contract breaks **while the function still returns 0**. Not a crash; a wrong answer that looks right. The prologue already shows it knows this hazard: it writes `sed -n '1p'` where the library writes `${_version%%$'\n'*}` — the same operation, in syntax correct under any `sh`.
- **The prologue has no way to find the library.** Under `sh` there is no `${BASH_SOURCE[0]}`, and `SELF_DIR` is not defined until *after* the `exec`.

- [ ] **Step 1: Write the equivalence guard**

**Name the host deliberately.** Neither existing file spans both sides: `test_bash_runtime_routing.sh` drives the facade but never sources the library; `test_docket_runtime_lib.sh` sources the library but never invokes `docket.sh`. Extend **`tests/test_bash_runtime_routing.sh`**, since the prologue side is the harder half and already lives there.

**Handle the asymmetry the file already handles:** for `docket.sh`, *accept* means `exec`ing into the fixture, so an **accepted** fixture must be a delegating wrapper (`exec "$REAL_BASH" "$@"`) while a **rejected** one is a pure banner-printer.

Append before the final `exit "$fail"`:

```bash
# --- change 0152: behavioral equivalence between the two surviving GNU Bash 4+ validators ---
# scripts/docket.sh's prologue is a DELIBERATE bootstrap exception (POSIX sh, cannot source the Bash
# library — see that file's comment block), so the duplication is permanent. This guard makes the
# maintenance obligation MECHANICAL rather than aspirational: a version-grammar change applied to one
# implementation and not the other reddens here.
#
# Anchored on BEHAVIOR (invoke each with a fake bash fixture and compare verdicts), never on source
# text — the two are written in different shell dialects on purpose, so any text-level assertion
# pins the wrong property and would break on a legitimate rewrite of either.
#
# Scope: banner shape and major-version floor — the two properties a grammar change actually moves.
# NOT full token-by-token equivalence: the prologue emits prose and exits, the library emits tokens,
# and forcing them to agree on the whole vocabulary would push the prologue toward the library's
# interface, which is the coupling this design deliberately avoids.
. "$REPO/scripts/lib/docket-runtime.sh"

EQ_BIN="$tmp/eqbin"; mkdir -p "$EQ_BIN"
# An ACCEPTED fixture must delegate, or docket.sh's exec has nothing to run.
mk_accept(){ # mk_accept <path> <banner>
  cat > "$1" <<EOF
#!/bin/sh
if [ "\$1" = --version ]; then printf '%s\n' '$2'; exit 0; fi
exec "\$REAL_BASH" "\$@"
EOF
  chmod +x "$1"
}
# A REJECTED fixture never gets exec'd, so a pure banner-printer is correct (and if the prologue
# wrongly accepted it, the run would fail loudly rather than silently pass).
mk_reject(){ # mk_reject <path> <banner>
  cat > "$1" <<EOF
#!/bin/sh
printf '%s\n' '$2'
exit 0
EOF
  chmod +x "$1"
}
mk_accept "$EQ_BIN/v5"       'GNU bash, version 5.2.0(1)-release (eq)'
mk_accept "$EQ_BIN/v4"       'GNU bash, version 4.0.0(1)-release (eq)'
mk_reject "$EQ_BIN/v3"       'GNU bash, version 3.2.57(1)-release (eq)'
mk_reject "$EQ_BIN/notgnu"   'zsh 5.9 (arm64-apple-darwin)'
mk_reject "$EQ_BIN/weird"    'GNU bash, version X.Y-release (eq)'

eq_mismatch=""
for _fx in v5 v4 v3 notgnu weird; do
  DOCKET_BASH_PATH="$EQ_BIN/$_fx" SCRIPTS_DIR="$tmp/stub-scripts" "$FACADE" env >/dev/null 2>&1
  _prologue=$?; [ "$_prologue" -eq 0 ] || _prologue=1
  docket_runtime_validate_bash "$EQ_BIN/$_fx" >/dev/null 2>&1
  _library=$?; [ "$_library" -eq 0 ] || _library=1
  [ "$_prologue" -eq "$_library" ] || eq_mismatch="$eq_mismatch $_fx(prologue=$_prologue,library=$_library)"
done
assert "0152: the prologue and the library agree on banner shape and major floor$eq_mismatch" \
  '[ -z "$eq_mismatch" ]'

# NON-VACUITY: the fixture set must actually contain both verdicts, or an all-reject set would make
# the loop agree trivially (marker-scoped-guard-needs-a-population-floor). The status is captured
# into a variable first — `$?` inside the assert's quoted argument would be evaluated at eval time
# and read the assert helper's own state, not the validator's.
docket_runtime_validate_bash "$EQ_BIN/v5" >/dev/null 2>&1; eq_accept_rc=$?
assert "0152 equivalence set is non-vacuous: it contains an ACCEPTED fixture" '[ "$eq_accept_rc" -eq 0 ]'
docket_runtime_validate_bash "$EQ_BIN/v3" >/dev/null 2>&1; eq_reject_rc=$?
assert "0152 equivalence set is non-vacuous: it contains a REJECTED fixture" '[ "$eq_reject_rc" -ne 0 ]'
```

Verify `$REPO`, `$tmp`, `$FACADE`, and `$REAL_BASH` against the file's actual definitions, and confirm the `$tmp/stub-scripts` mock is still in scope at the point you append. Note that `$?` inside an `assert` argument is evaluated at `eval` time, not at the `docket_runtime_validate_bash` call above it — the two non-vacuity asserts must capture the status into a variable first (`docket_runtime_validate_bash … ; _rc=$?`) and assert on that, or they are vacuous.

- [ ] **Step 2: Verify the guard reddens on a one-sided grammar change**

```bash
cp scripts/docket.sh /tmp/dk.bak
perl -pi -e 's/-lt 4/-lt 99/' scripts/docket.sh   # prologue floor only
bash tests/test_bash_runtime_routing.sh 2>&1 | grep -E '^NOT OK.*0152'
cp /tmp/dk.bak scripts/docket.sh

cp scripts/lib/docket-runtime.sh /tmp/rt.bak
perl -pi -e 's/\[ "\$_major" -ge 4 \]/[ "$_major" -ge 99 ]/' scripts/lib/docket-runtime.sh
bash tests/test_bash_runtime_routing.sh 2>&1 | grep -E '^NOT OK.*0152'
cp /tmp/rt.bak scripts/lib/docket-runtime.sh
```

Expected: the equivalence assert reddens under **each** one-sided mutation, and is green with both restored. A guard that only catches one side is half a guard.

- [ ] **Step 3: Add `ensure-global-config.sh`'s missing legacy coverage**

`tests/test_ensure_global_config.sh` builds a single fixture hardcoding `GNU bash, version 5.2.0(1)-release (test)` with **no Bash-3 or non-GNU negative case at all** — so breaking the library's major check leaves it green today. The stub's third bullet ("removing the Bash-major check must redden a test through **every** surviving caller") is not delivered without this.

Add a 3.2.57 fixture and assert the script rejects a hand-authored explicit `runtime.bash` pointing at it, with its existing diagnostic (`configured runtime.bash is not an absolute executable GNU Bash 4+`). Read the file's existing `DOCKET_BASH_STANDARD_ROOT` plumbing and express the case through it.

- [ ] **Step 4: Verify all four library callers now redden on the major-check mutation**

```bash
cp scripts/lib/docket-runtime.sh /tmp/rt.bak
perl -pi -e 's/\[ "\$_major" -ge 4 \]/[ "$_major" -ge 0 ]/' scripts/lib/docket-runtime.sh
for t in test_install test_ensure_global_config test_docket_config test_ensure_docket_env; do
  printf '%-34s NOT_OK=%s\n' "$t" "$(bash tests/$t.sh 2>&1 | grep -c '^NOT OK - ')"
done
cp /tmp/rt.bak scripts/lib/docket-runtime.sh
```

Expected: **every one of the four is non-zero.** `install.sh` and `docket-config.sh` were already covered; `ensure-docket-env.sh` gained coverage in Task 7 and `ensure-global-config.sh` in Step 3. If any is 0, that caller's mutation coverage is still missing.

- [ ] **Step 5: Document the exception at all THREE sites**

The third site is the one that goes actively **false**.

**(a) `scripts/lib/docket-runtime.sh`'s header.** It currently reads that independent checks "still live in `scripts/docket.sh` … and `scripts/ensure-docket-env.sh`" and that folding them in is "out of this library's current scope." After Task 7 that is a **stale claim of exactly the kind change 0133 was credited with fixing.** Correct it — `ensure-docket-env.sh` is now a caller; only `scripts/docket.sh`'s prologue remains independent, permanently and by design.

**Header comment only. Do not touch a line of code in this file** — Task 9 owns its code.

**(b) `scripts/docket.sh`'s prologue comment block.** State the obligation, not just the fact:

```sh
# THIS VALIDATOR IS INTENTIONALLY DUPLICATED (change 0152). It is POSIX `sh` by necessity — it runs
# before a Bash interpreter is chosen — and it must NOT source scripts/lib/docket-runtime.sh. The
# reason is NOT a parse failure: that library is bootstrap-compatible and sources fine under
# /bin/sh and dash. The reason is that `$'\n'` degrades to a LITERAL under a non-bash /bin/sh, so
# the library's `${_version%%$'\n'*}` payload split strips nothing and returns a wrong answer while
# still returning 0 — which is why the line below uses `sed -n '1p'` instead. The prologue also has
# no way to find the library: there is no ${BASH_SOURCE[0]} under sh, and SELF_DIR is not defined
# until after the exec.
#
# MAINTENANCE OBLIGATION: any change to the version grammar (the banner match or the major-version
# floor) MUST be applied to docket_runtime_validate_bash as well. The equivalence guard in
# tests/test_bash_runtime_routing.sh drives both implementations with fake fixtures and reddens if
# they diverge on banner shape or major floor.
```

**(c) `scripts/docket.md`.** Record the same exception and the same obligation where a maintainer will meet it. Also correct `scripts/docket-config.md`'s claim that "`scripts/docket.sh` **and `scripts/ensure-docket-env.sh`** run their own independent Bash-version checks and do not use this library" — after Task 7 only the former is true.

- [ ] **Step 6: Run the affected tests**

```bash
for t in test_bash_runtime_routing test_ensure_global_config test_ensure_docket_env test_docket_runtime_lib test_docket_facade test_install; do
  printf '%-34s NOT_OK=%s\n' "$t" "$(bash tests/$t.sh 2>&1 | grep -c '^NOT OK - ')"
done
```

Expected: `0` for all six. Also run `tests/test_script_contracts_coverage.sh` and `tests/test_comment_anchor_style.sh` — the doc edits touch surfaces those guard.

- [ ] **Step 7: Commit**

```bash
git add tests/test_bash_runtime_routing.sh tests/test_ensure_global_config.sh \
        scripts/docket.sh scripts/docket.md scripts/docket-config.md scripts/lib/docket-runtime.sh
git commit -m "test(0152): equivalence guard for the two validators; document the bootstrap exception"
```

---

### Task 9: 0153a — depth-anchor the runtime leaf and report the rejected shape

**Files:**
- Modify: `scripts/lib/docket-runtime.sh` (`_docket_runtime_scan` and `docket_runtime_unique`)
- Test: `tests/test_docket_runtime_lib.sh`

**Interfaces:**
- Consumes: Task 8's corrected library header (same file, header only — no conflict).
- Produces: a new global `DOCKET_RUNTIME_DEEP` (count of too-deeply-nested `bash:` leaves), a **three-line** `_docket_runtime_scan` payload, and `docket_runtime_unique` **return code 3**. Task 10 wires all three into the consumers.

**Background — verified empirically during planning, all three claims:**

```
runtime:
  codex:
    bash: /opt/weird/bash
```
→ **`count=1`, `value=/opt/weird/bash`.** A user writing that plainly means "the bash for the codex runner"; docket silently adopts it as the machine's Bash runtime for **every** docket operation, with no diagnostic. This is a **wrong-value** bug, not a strictness preference.

```
runtime:
    bash: /four/space/bash
```
→ **`count=1`, resolves today.** So **do not hard-code two spaces** — that would be a second, unannounced tightening.

A managed block plus a hand-authored deep block → **`explicit_count=1`**, which is what makes `ensure-global-config.sh`'s both-declarations guard fire today. Task 10 owns keeping it firing.

**Anchor on the block's SHALLOWEST structural child, not the first child's.** If the first child is itself the nested key, a first-child anchor lands too deep and a subsequent legitimate one-level `bash:` would be wrongly rejected.

**Marker and managed-block handling needs no change:** managed lines `next` out **before** `structural` is computed, so depth tracking never sees them. Comment-only lines are blanked by the existing `sub(/[[:space:]]*#.*/, "", structural)` and blank lines are empty, so neither can become the anchor.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_docket_runtime_lib.sh`, before the bootstrap-compatibility section.

```bash
# --- depth anchoring of the runtime leaf (mutation target M6) -----------------
# The pre-0153 pattern `in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/` matched ANY
# indentation depth under the header, and in_runtime is cleared only by a column-0 non-space line.
# Measured pre-fix: `runtime:` -> `codex:` -> `bash: /opt/weird/bash` resolved to count=1,
# value=/opt/weird/bash. A user writing that means "the bash for the codex runner"; docket adopted
# it as the machine's Bash runtime for every operation, with no diagnostic.
printf 'runtime:\n  nested:\n    bash: /some/path\n' > "$tmp/deep.yml"
assert "M6 a leaf deeper than the block's shallowest child is not counted" \
  '[ "$(docket_runtime_count "$tmp/deep.yml")" = 0 ]'
assert "M6 a too-deep leaf yields no value" \
  '[ -z "$(docket_runtime_first "$tmp/deep.yml")" ]'
docket_runtime_unique "$tmp/deep.yml" >/dev/null 2>&1; deep_rc=$?
assert "M6 docket_runtime_unique reports a too-deep leaf with rc 3" '[ "$deep_rc" -eq 3 ]'

# The motivating hazard: a SIBLING key, not a decorative nesting.
printf 'runtime:\n  codex:\n    bash: /opt/weird/bash\n' > "$tmp/sibling.yml"
assert "M6 a bash: under a sibling nested key is not adopted" \
  '[ "$(docket_runtime_count "$tmp/sibling.yml")" = 0 ]'
docket_runtime_unique "$tmp/sibling.yml" >/dev/null 2>&1; sib_rc=$?
assert "M6 the sibling-key shape is reported with rc 3" '[ "$sib_rc" -eq 3 ]'

# DEPTH-RELATIVE, NOT TWO-SPACE: a four-space canonical file resolved before this change and must
# keep resolving. Hard-coding two spaces would be a second, unannounced tightening.
printf 'runtime:\n    bash: /four/space/bash\n' > "$tmp/four.yml"
assert "M6 a four-space one-level leaf still resolves" \
  '[ "$(docket_runtime_first "$tmp/four.yml")" = "/four/space/bash" ]'
assert "M6 the four-space file is counted once" \
  '[ "$(docket_runtime_count "$tmp/four.yml")" = 1 ]'

# The anchor is the SHALLOWEST structural child, not the FIRST: when the first child is the nested
# key, a first-child anchor lands too deep and would wrongly reject a later legitimate leaf.
printf 'runtime:\n    deep_first:\n      x: 1\n  bash: /correct/bash\n' > "$tmp/shallow-later.yml"
assert "M6 a one-level leaf after a deeper first child still resolves" \
  '[ "$(docket_runtime_first "$tmp/shallow-later.yml")" = "/correct/bash" ]'

# PER-BLOCK RESET: without it, block 2 inherits block 1's anchor.
printf 'runtime:\n  nested:\n    bash: /deep/one\nruntime:\n  bash: /good/two\n' > "$tmp/two-blocks.yml"
assert "M6 the depth anchor resets when in_runtime clears" \
  '[ "$(docket_runtime_first "$tmp/two-blocks.yml")" = "/good/two" ]'
assert "M6 block 2 is counted once despite block 1's deep leaf" \
  '[ "$(docket_runtime_count "$tmp/two-blocks.yml")" = 1 ]'

# UNCHANGED SHAPES — regression pins, expected green both ways.
assert "M6 the canonical two-space file is unchanged" \
  '[ "$(docket_runtime_first "$tmp/one.yml")" = "/one" ]'
assert "M6 a tab-indented leaf is still read" \
  '[ "$(docket_runtime_first "$tmp/tab.yml")" = "/tab/bash" ]'
assert "M6 a bash: at column 0 outside any runtime: block is still ignored" \
  '[ "$(docket_runtime_count "$tmp/decoy.yml")" = 0 ]'
assert "M6 the managed-block file is unchanged" \
  '[ "$(docket_runtime_count "$tmp/managed-only.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 0 ]'

# A deep leaf INSIDE a managed block must not be seen at all: managed lines `next` out before
# `structural` is computed, so depth tracking never sees them.
printf '%s\nruntime:\n  nested:\n    bash: /managed/deep\n%s\nruntime:\n  bash: /explicit/ok\n' \
  "$MARK_OPEN" "$MARK_CLOSE" > "$tmp/managed-deep.yml"
assert "M6 a deep leaf inside the managed block does not leak a DEEP report" \
  '[ "$(docket_runtime_count "$tmp/managed-deep.yml" "$MARK_OPEN" "$MARK_CLOSE")" = 1 ]'
```

**Verify `$tmp/decoy.yml` is the name the existing M1 block uses** (it may be `decoy2.yml`); substitute the file's own fixture names.

- [ ] **Step 2: Add M6 to the mutation table**

The repo pins mutations by id (M1–M5 in this file). **Claim M6 for the anchor and add it to the mutation table**, or it will not be enforced the way the rest of the file is. Find the table in the file's header/comment block and add the row: reverting the leaf pattern to the loose `in_runtime && structural ~ /^[[:space:]]+bash[[:space:]]*:/` form must redden the nested-leaf and sibling-key cases specifically.

- [ ] **Step 3: Run the tests to verify they fail**

```bash
bash tests/test_docket_runtime_lib.sh 2>&1 | grep -E '^NOT OK - M6'
```

Expected: the five deep/sibling/reset asserts FAIL. The four-space, shallow-later, and all four regression pins PASS pre-fix (measured: the four-space file resolves today). If a regression pin fails pre-fix, the fixture is wrong.

- [ ] **Step 4: Implement the depth anchor**

In `_docket_runtime_scan`'s awk. **No snippet is dictated for the rule itself beyond the structure below — implement the RULE, and check your conjuncts are not tautologies** (an earlier spec draft carried a snippet whose added conjunct was implied by the leaf pattern it was anded with).

Rule ordering matters, because awk runs pattern-action blocks in source order and several apply to the same line:

1. The `runtime:` header rule sets `in_runtime=1` and **resets the anchor state**, then `next`.
2. The dedent rule (`in_runtime && structural ~ /^[^[:space:]]/`) clears `in_runtime` and **resets the anchor state**.
3. **A new anchor-tracking rule** runs *before* the leaf rule: for any non-blank `structural` line while `in_runtime`, compute its indent width as `match(structural, /[^[:space:]]/) - 1` and keep the running **minimum**. Guard with a `have_anchor` flag so the first child sets the anchor rather than being compared against an uninitialised 0.
4. The leaf rule compares the leaf's own indent to the anchor. `indent > anchor` ⇒ increment `deep`; otherwise ⇒ the existing `count++` / `first=scalar(value)` path, unchanged.

Because the leaf is itself a structural child, rule 3 sets the anchor to the leaf's own indent when the leaf is the first child — which is exactly why the four-space file keeps resolving.

**Use `[^[:space:]]`, never `[^ ]`** — a literal-space class silently drops the tab-indented fixture (AGENTS.md).

Then extend the payload. **Payload ordering is load-bearing:** the `$( )` capture strips trailing newlines, which is why the `printf x` guard exists. If `DEEP` were appended **last**, the existing `DOCKET_RUNTIME_VALUE="${_raw%$'\n'}"` would silently swallow it. Put `DEEP` on **line 2, with the value terminal**:

```awk
    END { printf "%d\n%d\n%s\n", count+0, deep+0, first }
```

and read it back in the same order:

```bash
  DOCKET_RUNTIME_COUNT=0
  DOCKET_RUNTIME_DEEP=0
  DOCKET_RUNTIME_VALUE=""
  ...
  DOCKET_RUNTIME_COUNT="${_raw%%$'\n'*}"
  _raw="${_raw#*$'\n'}"
  DOCKET_RUNTIME_DEEP="${_raw%%$'\n'*}"
  _raw="${_raw#*$'\n'}"
  DOCKET_RUNTIME_VALUE="${_raw%$'\n'}"
```

Update the function's header comment to document `DOCKET_RUNTIME_DEEP` and the three-line payload, and the file's top-of-header interface list to document `docket_runtime_unique`'s new return code.

- [ ] **Step 5: Give `docket_runtime_unique` return code 3**

```bash
docket_runtime_unique(){ # docket_runtime_unique <file> [open] [close]
  _docket_runtime_scan "$@"
  # DEEP is gated REGARDLESS of COUNT, not on `COUNT == 0 && DEEP > 0`: a file carrying both a
  # valid one-level leaf and a too-deep one must still report the deep shape, or the installer's
  # both-declarations guard loses its signal (see ensure-global-config.sh).
  [ "${DOCKET_RUNTIME_DEEP:-0}" -eq 0 ] || return 3
  [ "$DOCKET_RUNTIME_COUNT" -le 1 ] || return 2
  printf '%s\n' "$DOCKET_RUNTIME_VALUE"
}
```

**Bash 3.2 compatibility still applies to this whole file** — no associative arrays, no `mapfile`, no `${x^^}`, no `declare -g`, no `;;&`.

- [ ] **Step 6: Run the tests to verify they pass, and mutation-test M6**

```bash
bash tests/test_docket_runtime_lib.sh 2>&1 | grep -cE '^NOT OK'
```

Expected: `0` — including the Bash 3.2 witness case at the end of the file, which re-runs the core reads under `/bin/bash`. **That case asserts an exact pipe-joined string; if the payload change altered what it reads, update it deliberately, not reflexively.**

M6 mutation: revert the leaf rule to the loose form and confirm the nested-leaf and sibling-key asserts specifically redden.

```bash
bash tests/test_docket_config.sh 2>&1 | grep -cE '^NOT OK'
```

Expected: `0` — the resolver reads this library and must be unaffected until Task 10 wires the new code.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-runtime.sh tests/test_docket_runtime_lib.sh
git commit -m "fix(0153): depth-anchor the runtime.bash leaf to its block's shallowest structural child"
```

---

### Task 10: 0153b — wire `DEEP` into every consumer and document the grammar

**Files:**
- Modify: `scripts/docket-config.sh` (two rc consumers)
- Modify: `scripts/ensure-global-config.sh` (the `explicit_*` wrappers and the both-declarations guard)
- Modify: `install.sh` (inline comment only)
- Modify: `scripts/docket-config.md`, `scripts/ensure-global-config.md`
- Test: `tests/test_docket_config.sh`, `tests/test_ensure_global_config.sh`

**Interfaces:**
- Consumes: Task 9's `DOCKET_RUNTIME_DEEP` global, three-line payload, and `docket_runtime_unique` rc 3.
- Produces: the final state of the branch. No later task depends on it.

**Background — this task is NOT optional polish; it is where the change stops being a regression.**

**(a) The two rc consumers must be edited.** `scripts/docket-config.sh` has exactly two `runtime_get` sites, and each **hard-codes** the meaning of non-zero into its message (`".docket.local.yml contains multiple runtime.bash declarations; keep exactly one"` and the global twin). An unmapped code 3 would emit an **actively false** diagnostic pointing at a duplicate that does not exist — strictly worse than the "not configured" message this change exists to replace.

**(b) `count` and `first` need a channel too.** They always return 0, and `install.sh` and `ensure-global-config.sh` reach the library **only** through them — so without this, the reporting promise is delivered for the resolver alone.

**(c) The both-declarations guard is a hole a naive tightening opens, on the WRITE path.** Verified: today a global config carrying the managed block **plus** a hand-authored deep block yields `explicit_count = 1`, so `ensure-global-config.sh` hits its hard die. After the tightening that count becomes 0, the guard stops firing, and the installer **rewrites the managed block over a file whose author declared something else**. The resolver then reads `COUNT=1` from the managed block, so a `COUNT == 0` gate never fires and no diagnostic appears anywhere — **a loud abort becomes a silent override**, the exact failure mode this change exists to remove, newly introduced.

**Shell-mode note:** `scripts/docket-config.sh` is `set -uo pipefail` (**no `-e`**), so `x="$(cmd)"; rc=$?` is safe. `scripts/ensure-global-config.sh` is **`set -eu`** — a wrapper that returns non-zero aborts the script, so any new wrapper there must always return 0.

- [ ] **Step 1: Write the failing tests**

**In `tests/test_docket_config.sh`** — add to the change-0132 runtime section:

```bash
# --- change 0153: a too-deeply-nested runtime.bash leaf is a NAMED error, never an absence ---
# Both rc consumers hard-code non-zero to mean "multiple declarations", so an unmapped rc 3 would
# emit an actively FALSE diagnostic naming a duplicate that does not exist.
mkrepo "$tmp/runtime-deep"
mkdir -p "$tmp/runtime-deep.xdg/docket"
printf 'runtime:\n  codex:\n    bash: %s\n' "$tmp/runtime-bin/global-bash" \
  > "$tmp/runtime-deep.xdg/docket/config.yml"
deep_rc="$(rung_rc "$tmp/runtime-deep.xdg" "$tmp/runtime-deep" --export)"
deep_err="$(XDG_CONFIG_HOME="$tmp/runtime-deep.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-deep" --export 2>&1 >/dev/null)"
assert "0153 deep global: resolver aborts" '[ "$deep_rc" != 0 ]'
assert "0153 deep global: diagnostic names the nesting depth, not a duplicate" \
  'grep -qF "exactly one level" <<<"$deep_err"'
assert "0153 deep global: diagnostic does NOT claim multiple declarations" \
  '! grep -qF "multiple runtime.bash declarations" <<<"$deep_err"'
assert "0153 deep global: diagnostic names the offending file" \
  'grep -qF "config.yml" <<<"$deep_err"'

# The repo-local twin — same shape, different file, and it must name ITS file.
mkrepo "$tmp/runtime-deep-local"
printf 'runtime:\n  codex:\n    bash: %s\n' "$tmp/runtime-bin/local-bash" \
  > "$tmp/runtime-deep-local/.docket.local.yml"
deepl_rc="$(rung_rc "$tmp/runtime-deep-local.xdg" "$tmp/runtime-deep-local" --export)"
deepl_err="$(XDG_CONFIG_HOME="$tmp/runtime-deep-local.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-deep-local" --export 2>&1 >/dev/null)"
assert "0153 deep local: resolver aborts" '[ "$deepl_rc" != 0 ]'
assert "0153 deep local: diagnostic names .docket.local.yml, not a duplicate" \
  'grep -qF ".docket.local.yml" <<<"$deepl_err" && ! grep -qF "multiple runtime.bash declarations" <<<"$deepl_err"'

# NON-REGRESSION: a genuine duplicate must STILL get the duplicate message, not the depth one.
mkrepo "$tmp/runtime-dup"
printf 'runtime:\n  bash: /a\n  bash: /b\n' > "$tmp/runtime-dup/.docket.local.yml"
dup_err="$(XDG_CONFIG_HOME="$tmp/runtime-dup.xdg" bash "$SCRIPT" --repo-dir "$tmp/runtime-dup" --export 2>&1 >/dev/null)"
assert "0153: a real duplicate still gets the duplicate diagnostic" \
  'grep -qF "multiple runtime.bash declarations" <<<"$dup_err"'
```

**In `tests/test_ensure_global_config.sh`** — the most important pin in this task:

```bash
# --- change 0153 part 3: the both-declarations guard must keep firing on a DEEP explicit block ---
# Verified pre-tightening: managed block + hand-authored deep block yields explicit_count = 1, so
# this die fires today. After the depth anchor that count becomes 0 — without the DEEP-aware guard
# the installer would rewrite its managed block over a file whose author declared something else,
# and the resolver would then read COUNT=1 from the managed block, so NO diagnostic would appear
# anywhere. A loud abort would become a silent override.
DEEP_DEST="$(mktemp -d)/config.yml"; _tmpdirs+=("$(dirname "$DEEP_DEST")")
printf '# >>> docket (runtime.bash) >>>\nruntime:\n  bash: %s\n# <<< docket (runtime.bash) <<<\nruntime:\n  codex:\n    bash: /hand/authored\n' \
  "$RUNTIME_ROOT/opt/homebrew/bin/bash" > "$DEEP_DEST"
cp "$DEEP_DEST" "$DEEP_DEST.before"
deep_out="$(DOCKET_GLOBAL_CONFIG="$DEEP_DEST" bash "$GLOBAL_SCRIPT" 2>&1)"; deep_rc=$?
assert "0153: managed + deep explicit still dies (unchanged message)" \
  '[ "$deep_rc" -ne 0 ] && grep -qF "contains both managed and explicit runtime.bash declarations" <<<"$deep_out"'
assert "0153: the refused file is left byte-identical" 'cmp -s "$DEEP_DEST.before" "$DEEP_DEST"'

# A deep block with NO managed block must also be named, never silently overwritten.
DEEP2="$(mktemp -d)/config.yml"; _tmpdirs+=("$(dirname "$DEEP2")")
printf 'runtime:\n  codex:\n    bash: /hand/authored\n' > "$DEEP2"; cp "$DEEP2" "$DEEP2.before"
deep2_out="$(DOCKET_GLOBAL_CONFIG="$DEEP2" bash "$GLOBAL_SCRIPT" 2>&1)"; deep2_rc=$?
assert "0153: a deep-only explicit block is reported, not overwritten" \
  '[ "$deep2_rc" -ne 0 ] && grep -qF "exactly one level" <<<"$deep2_out"'
assert "0153: the deep-only file is left byte-identical" 'cmp -s "$DEEP2.before" "$DEEP2"'
```

**`DOCKET_GLOBAL_CONFIG` / `$GLOBAL_SCRIPT` / `$RUNTIME_ROOT` are placeholders** — read `tests/test_ensure_global_config.sh` and use its actual `DEST` plumbing and env seams.

- [ ] **Step 2: Run to verify they fail**

```bash
bash tests/test_docket_config.sh 2>&1 | grep -E '^NOT OK - 0153'
bash tests/test_ensure_global_config.sh 2>&1 | grep -E '^NOT OK - 0153'
```

Expected: the depth-diagnostic asserts fail (the resolver currently emits the *duplicate* message for rc 3 — verify that is literally what you see, since it is the false-diagnostic regression), and both `ensure-global-config` deep cases fail. The duplicate non-regression assert passes.

- [ ] **Step 3: Branch on rc 3 at BOTH `docket-config.sh` sites**

`|| die` cannot see the code, so restructure each site. `set -uo pipefail` (no `-e`) makes this safe:

```bash
_runtime_local="$(runtime_get "$LCFG")"; _rt_rc=$?
case "$_rt_rc" in
  0) ;;
  3) die "runtime.bash must be nested exactly one level under \`runtime:\`; found it deeper in .docket.local.yml" ;;
  *) die ".docket.local.yml contains multiple runtime.bash declarations; keep exactly one" ;;
esac
```

and the global twin, naming `global config.yml`. **Keep both existing duplicate messages verbatim** — only the new arm is added.

- [ ] **Step 4: Make the `explicit_*` wrappers DEEP-aware and keep the guard firing**

In `scripts/ensure-global-config.sh`, add a wrapper beside the existing two. It must always return 0 (`set -eu`):

```bash
explicit_runtime_deep(){ _docket_runtime_scan "$1" "$MARK_OPEN" "$MARK_CLOSE"; printf '%s\n' "$DOCKET_RUNTIME_DEEP"; }
```

Then, beside the existing `_explicit` / `_explicit_count` reads:

```bash
_explicit_deep="$(explicit_runtime_deep "$DEST")"
```

Widen the both-declarations guard so it fires on a deep block too:

```bash
# change 0153: the depth anchor drops a hand-authored deep block out of _explicit_count, which
# would silently disarm this guard and let the installer rewrite its managed block over a file
# whose author declared something else. Gate on DEEP as well.
if { [ "$_explicit_count" -eq 1 ] || [ "$_explicit_deep" -gt 0 ]; } && grep -qF -- "$MARK_OPEN" "$DEST"; then
  die "$DEST contains both managed and explicit runtime.bash declarations; remove one so exactly one runtime is authoritative — left unchanged"
fi
# A deep block with NO managed block is equally unsafe to overwrite: the author declared something
# the resolver will not read. Name it rather than absorbing it.
if [ "$_explicit_deep" -gt 0 ]; then
  die "$DEST declares runtime.bash deeper than one level under \`runtime:\`; it must be nested exactly one level — left unchanged"
fi
```

Order matters: the both-declarations message must win when a managed block is present, so keep that `if` first.

- [ ] **Step 5: Update `install.sh`'s stale inline comment**

`install.sh` carries `# No markers and no duplicate handling: ensure-global-config.sh has just guaranteed exactly one` **authoritative declaration**. That claim goes stale in the same stroke — the guarantee now also rests on the DEEP check. Update the comment to say so. **Comment only; no code change in `install.sh`.**

- [ ] **Step 6: Document the grammar — including the two stale sites**

**(a) `scripts/docket-config.md` still claims** "`yaml_block_body` isolates each `runtime:` block before `bash:` is read, so a bare `bash:` elsewhere cannot shadow the runtime setting." That has been **stale since change 0133** (the read moved into the shared library), and it is exactly the grammar sentence this change rewrites. Replace it: the leaf must sit **exactly one level** under `runtime:` — anchored on the block's shallowest structural child, so a four-space file resolves — and a deeper leaf is a **reported error**, never an absent key.

**(b) `scripts/docket-config.md`'s exit-code table** needs a row for the new failure mode, beside the existing `runtime.bash is absent, relative, non-executable, unversionable, nonnumeric, or Bash <4 | 1`:

```
| `runtime.bash` is declared deeper than one level under `runtime:` | 1 |
```

**(c) `scripts/ensure-global-config.md`** wherever `runtime.bash` is described — state the one-level rule and that a deeper declaration is refused with the file left unchanged.

- [ ] **Step 7: Run the tests to verify they pass**

```bash
for t in test_docket_config test_ensure_global_config test_docket_runtime_lib test_install \
         test_bash_runtime_routing test_ensure_docket_env test_script_contracts_coverage; do
  printf '%-36s NOT_OK=%s\n' "$t" "$(bash tests/$t.sh 2>&1 | grep -c '^NOT OK - ')"
done
bash tests/test_docket_config.sh 2>&1 | grep -E '^TOTALS'
```

Expected: `0` for all seven. The `TOTALS` line may legitimately move if Step 1's new fixtures added eval sites — **that is fine and is exactly why Task 5's bound is a ratio**: deletions and additions move `t_sites` and `t_ok` together. Confirm the proportional floor is still satisfied by the new numbers rather than assuming it.

- [ ] **Step 8: Commit**

```bash
git add scripts/docket-config.sh scripts/ensure-global-config.sh install.sh \
        scripts/docket-config.md scripts/ensure-global-config.md \
        tests/test_docket_config.sh tests/test_ensure_global_config.sh
git commit -m "fix(0153): report the too-deep runtime.bash leaf through every consumer"
```

---

### Task 11: Whole-branch integration — one green full-suite run

**Files:**
- Modify: none expected. Any edit this task requires is a genuine integration defect — fix it, and note it for the review.

**Interfaces:**
- Consumes: all ten preceding tasks.
- Produces: the branch's single integration signal.

**Background.** The point of batching is **one** integration signal. A per-unit green that goes red later is not done. Three of the seven units edit files that other units also touch (`tests/test_docket_config.sh` by Tasks 4/5/10; `scripts/lib/docket-runtime.sh` by Tasks 8/9), and this repo's own history says whole-branch review is where cross-task interaction defects surface after every per-task review passed.

- [ ] **Step 1: Run the full suite**

Use the **Full-suite command** from *Global Constraints*, verbatim. Run it in the **foreground** and allow it time — the suite is large (66 files).

- [ ] **Step 2: Confirm the result**

Expected: `SUITE_FAIL=0` and no `FAIL` lines.

**If anything is red:** apply `superpowers:systematic-debugging`. Do **not** weaken a test to get green. If a unit genuinely cannot go green, drop that unit's commits from the branch and report it as a re-mint candidate — that is the fallback the design names, and it is a reporting outcome, not a silent one.

- [ ] **Step 3: Re-check portability under the real `grep`**

The PATH `grep` here is **ugrep**, which accepts ERE syntax `/usr/bin/grep` rejects — a portability bug can pass locally while being real.

```bash
/usr/bin/grep --version | head -1
grep --version | head -1
```

Confirm every ERE this branch introduced is spelled through `/usr/bin/grep` where it is used as a portability check (Task 1's asserts already are). Sanity-check any new ERE:

```bash
/usr/bin/grep -qE "^\| \[[0-9]{4}\]\(archive/\) \|" /dev/null; echo "ERE accepted by /usr/bin/grep: rc=$?"
```

rc 1 (no match) is success here; rc 2 means the ERE was **rejected** and must be rewritten.

- [ ] **Step 4: Verify the branch touched no docket metadata**

The feature branch adds only plan + results + code and **never modifies** docket metadata (change files, `BOARD.md`, ADRs). ADR-0052's update from Task 3 belongs on `docket`, not here.

```bash
git diff --stat origin/main...HEAD -- docs/changes docs/adrs
```

Expected: **empty**. If anything shows, it was written to the wrong tree — move it to `/Users/homer/dev/docket/.docket` and remove it from this branch.

- [ ] **Step 5: Review the diff unit by unit**

```bash
git log --oneline origin/main..HEAD
git diff origin/main...HEAD --stat
```

Expected: ten commits in the ordered sequence 0143 → 0144 → 0146 → 0148 → 0149 → 0152(×3) → 0153(×2), so the diff reads unit by unit. Confirm no commit mixes two units.

- [ ] **Step 6: Commit anything the integration run required**

```bash
git status --porcelain
```

If clean, nothing to do. Otherwise commit the integration fix on its own:

```bash
git add -A
git commit -m "fix(0157): <what the whole-branch integration surfaced>"
```
