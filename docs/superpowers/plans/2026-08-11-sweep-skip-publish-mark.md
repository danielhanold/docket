<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0118 — Decide whether the sweep's skip-publish path should also mark an unpublished terminal record](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0118-decide-whether-the-sweep-s-skip-publish-path-should-also-mar.md)**
<!-- docket:backlink:end -->

# Mark the sweep's skip-publish path — Implementation Plan (change 0118)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When the merge sweep's `## Artifacts` re-render fails and a publish was expected, the sweep writes the durable `## Publish deferred` marker on the archived change file — closing the last automated path that leaves a change archived-but-unpublished with nothing to see it.

**Architecture:** One new shared helper in `scripts/docket-status.sh` (`sweep_mark_publish_deferred`) owns the mark + commit + push + recovery, and BOTH the existing change-0083 `terminal-publish` failure block and the new `render-change-links skipped-publish` block call it. The new call site is gated on `TERMINAL_PUBLISH=true && DOCKET_MODE=docket`; the 0083 call site is not (suppression cannot reach it). `scripts/mark-publish-deferred.sh`'s fixed body prose is generalized so it is true on both paths. Three docs are corrected to state the real reason.

**Tech Stack:** Bash 4 floor, `set -uo pipefail` (no `-e`), the repo's existing `GIT`/`SCRIPTS_DIR`/`DOCKET_BASH_PATH` mock seams, and its `assert "<name>" '<expr>'` test idiom.

## Global Constraints

- **Report contract is frozen.** The `skipped-publish` branch must still emit exactly `sweep-failed <id> render-change-links skipped-publish` and still `return 0`. The 0083 branch must still emit exactly `sweep-failed <id> terminal-publish script-error` and still `return 0`. No line added, removed, or reordered on either.
- **Best-effort toward the report stream, transactional toward the worktree.** No step's outcome may reach the report stream or alter control flow. But a failed `add`/`commit` must restore the archived path to `HEAD` (index and worktree both) — a dirty shared worktree fails every later pass's `pull --rebase` and is worse than the gap it records. A failed `push` **retains** the local commit (it self-heals on the next pass's `pull --rebase`); never reset it.
- **Never mark under suppression.** `terminal_publish: false` or `main`-mode ⇒ a skipped publish is *success*, not a deferral (ADR-0051, `scripts/mark-publish-deferred.md`).
- **Never commit into a wedged shared worktree.** Probe `_docket_tree_wedged "$GIT" "$mw"` before marking. Inside a rebase, `HEAD` is the rebase's *detached* HEAD, so the restore step would corrupt the very file it exists to repair.
- **`--detail` must never contain the literal `terminal-publish.sh`.** `tests/test_closeout.sh`'s `find_ungated_terminal_publish_call_sites` joins logical lines and greps for that literal near `--id` without `--enabled`; the mark invocation carries `--id` and no `--enabled`, so the literal in the detail string would trip the scanner on this call site. (Comments are separate physical lines after joining and are safe — the existing 0083 comment block already names it.)
- **No third `--reason` value, no new heading, no interface change** to `mark-publish-deferred.sh` beyond §2's body-prose generalization. `board-checks.sh` and the `publish-deferred` check are untouched.
- **No change to the `commit-failed` / `push-failed` / `blocked-wedged-tree` legs** (§6a): there the close-out continues and the record publishes.
- **Budgets.** `tests/test_docket_status.sh` measured **45.15s serial** against its **45s** row before this change — already at parity. `skills/docket-status/SKILL.md` measured **2478 words / 102 lines** against **2500 / 118**. Both will trip. Task 6 resolves them with measured numbers, in-diff arguments, and a re-seeded `EXPECTED_TOTAL`. No row may exceed **60s**.

## File Structure

| File | Responsibility in this change |
|---|---|
| `scripts/docket-status.sh` | New `sweep_mark_publish_deferred` helper; the 0083 block rewritten to call it; the `skipped-publish` block gains a gated call. |
| `scripts/mark-publish-deferred.sh` | Two fixed body-prose sentences generalized (lines 174–176). Nothing else. |
| `scripts/docket-status.md` | §6's "nothing was deferred *yet*" rationale replaced. |
| `skills/docket-status/SKILL.md` | Sweep-posture paragraph corrected. |
| `skills/docket-convention/references/terminal-close-out.md` | Step-3 mark rule extended to handled post-archive abandonment; the skip-publish guard's "(all callers)" sentence carves out the sweep. |
| `tests/test_mark_publish_deferred.sh` | Marker-truthfulness cases (real script). |
| `tests/test_docket_status.sh` | Invocation, gating, and transactional-posture cases (mocked marker + `GIT` seam). |
| `tests/test_closeout.sh` | Contract sentinels for the extended `terminal-close-out.md` rule. |
| `tests/runtime-budgets.tsv`, `tests/test_skill_size_budgets.sh`, `tests/test_runtime_budgets.sh` | Budget raises with in-diff arguments. |

---

### Task 1: Generalize the marker's fixed body prose

The marker's body currently asserts "Close-out steps 1–2 (archive, `## Artifacts` re-render) landed on the metadata branch". On the new path the re-render is precisely what failed, so that sentence contradicts the detail line above it. Generalize it once, truthfully, for every marking path.

**Files:**
- Modify: `scripts/mark-publish-deferred.sh` (the `printf` block that renders the body — search for `Close-out steps 1`)
- Test: `tests/test_mark_publish_deferred.sh` (append before the `--- arg validation ---` section)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: the exact rendered sentences later tasks assert on — `The archive landed on the metadata branch;` … `did **not**` … `complete. See the dated line above for what failed. The record is on the metadata branch only.`

- [ ] **Step 1: Confirm the old prose lives in exactly one maintained file**

Run:
```bash
cd "$(git rev-parse --show-toplevel)"
grep -rn "Close-out steps 1" --exclude-dir=.git --exclude-dir=.docket --exclude-dir=.worktrees .
```
Expected: four hits — `scripts/mark-publish-deferred.sh` plus three frozen point-in-time records (`docs/superpowers/plans/2026-07-21-terminal-publish-gap-detection.md`, `docs/superpowers/specs/2026-07-12-optional-terminal-publish-design.md`, `docs/superpowers/specs/2026-07-18-terminal-publish-gap-detection-design.md`). **Only the script is edited** — merged plans and specs are frozen build records and must not be touched. If the grep shows a *fifth* maintained hit (a test or a contract doc), stop and report: the plan's premise has drifted.

- [ ] **Step 2: Write the failing tests**

Append to `tests/test_mark_publish_deferred.sh`, immediately before the `# --- arg validation ---` comment:

```bash
# --- change 0118: the fixed body prose must be true on EVERY marking path ----------------------
# The pre-0118 prose asserted "Close-out steps 1-2 (archive, `## Artifacts` re-render) landed" —
# factually FALSE on the sweep's new skipped-publish path, where the re-render is exactly what
# failed. A detail line above a contradiction below it does not cure the contradiction. These are
# written as a NEGATIVE (the removed sentence is absent) plus a POSITIVE non-vacuity companion, so
# a broken fixture reddens instead of reading as the property holding.
f12="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f12" --reason blocked --date 2026-08-11 --id 118 \
     --integration-branch main \
     --detail "sweep: the artifacts re-render failed, so the publish was never attempted — re-render before publishing" \
     >/dev/null 2>&1
rc12=$?
assert "0118: the marker still renders with the new detail (non-vacuity)" '[ "$rc12" -eq 0 ]'
assert "0118: the rendered section exists at all (non-vacuity)" \
  'grep -qxF -- "$MARKER" "$f12"'
assert "0118: no sentence claims the ## Artifacts re-render landed" \
  '! grep -qF -- "Close-out steps 1" "$f12"'
assert "0118: the generalized prose says the ARCHIVE landed" \
  'grep -qF -- "The archive landed on the metadata branch;" "$f12"'
assert "0118: the generalized prose says the publish did not COMPLETE (not: did not run)" \
  'grep -qF -- "complete. See the dated line above for what failed." "$f12"'
assert "0118: the generalized prose still says where the record is" \
  'grep -qF -- "The record is on the metadata branch only." "$f12"'
assert "0118: the dated detail line carries the re-render cause verbatim" \
  'grep -qF -- "**blocked** — sweep: the artifacts re-render failed, so the publish was never attempted — re-render before publishing" "$f12"'
assert "0118: the **Re-arm:** line is unchanged (complete the publish, with the id hint)" \
  'grep -qF -- "**Re-arm:** complete the publish" "$f12" && grep -qF -- "--id 118" "$f12"'
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `bash tests/test_mark_publish_deferred.sh 2>&1 | grep -E "^NOT OK - 0118"`

Expected: the three prose asserts fail — `no sentence claims the ## Artifacts re-render landed`, `the generalized prose says the ARCHIVE landed`, `the generalized prose says the publish did not COMPLETE (not: did not run)`. The non-vacuity, detail-line, and Re-arm asserts must already **pass**; if any of those is red, the fixture or the invocation is wrong, not the code.

- [ ] **Step 4: Write the implementation**

In `scripts/mark-publish-deferred.sh`, replace exactly these three lines:

```bash
  printf 'Close-out steps 1–2 (archive, `## Artifacts` re-render) landed on the metadata branch;\n'
  printf 'the terminal-publish step (copying the archived change file + its `spec:` + its Accepted\n'
  printf 'ADRs onto `%s`) did **not** run. The record is on the metadata branch only.\n\n' "$INT_BRANCH"
```

with:

```bash
  # change 0118: generalized from "Close-out steps 1–2 (archive, `## Artifacts` re-render) landed
  # … did **not** run". That sentence is true only on the change-0083 path. The sweep now also
  # marks when the `## Artifacts` RE-RENDER is what failed, where claiming the re-render landed
  # directly contradicts the dated detail line rendered above it. "did not complete" also reads
  # correctly on the 0083 path, where the publisher ran and exited non-zero.
  printf 'The archive landed on the metadata branch; the terminal-publish step (copying the\n'
  printf 'archived change file + its `spec:` + its Accepted ADRs onto `%s`) did **not**\n' "$INT_BRANCH"
  printf 'complete. See the dated line above for what failed. The record is on the metadata\n'
  printf 'branch only.\n\n'
```

Note the last `printf` takes **no** argument — `"$INT_BRANCH"` is consumed by the second one. Keep the em-dash-free ASCII in the new text except where already present.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_mark_publish_deferred.sh`
Expected: `PASS`, exit 0, and zero `NOT OK` lines.

- [ ] **Step 6: Mutation-probe the negative assert**

The negative assert must detect the *old* state, not merely confirm the new one. Restore the old sentence temporarily and confirm the assert reddens — and **confirm the mutation actually landed by byte count**, not by eyeballing:

```bash
cp scripts/mark-publish-deferred.sh /tmp/mpd-0118.bak
before=$(grep -c "The archive landed on the metadata branch;" scripts/mark-publish-deferred.sh)
perl -0pi -e "s/The archive landed on the metadata branch;/Close-out steps 1\xe2\x80\x932 (archive, \`## Artifacts\` re-render) landed on the metadata branch;/" scripts/mark-publish-deferred.sh
after=$(grep -c "The archive landed on the metadata branch;" scripts/mark-publish-deferred.sh)
echo "mutation landed: before=$before after=$after (expect 1 then 0)"
bash tests/test_mark_publish_deferred.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/mpd-0118.bak scripts/mark-publish-deferred.sh && rm -f /tmp/mpd-0118.bak
```

Expected: `before=1 after=0` (the mutation landed) and a **non-zero** `NOT OK` count. If the count is 0 with a landed mutation, the assert is decoration — fix it before continuing. Restore from the **backup copy**, never `git checkout --`, which would discard the whole task's uncommitted work.

- [ ] **Step 7: Commit**

```bash
git add scripts/mark-publish-deferred.sh tests/test_mark_publish_deferred.sh
git commit -m "fix(0118): the marker's body prose is true on every marking path"
```

---

### Task 2: Extract the sweep's mark into one transactional helper

Today the 0083 mark is inlined with bare `&&`-chained suppression: if the marker writes but `add` or `commit` fails, the archived file is left dirty (or staged) in the SHARED metadata worktree, which fails the next pass's `pull --rebase` for every change — the state the spec's own rationale calls worse than an unmarked gap. Extract one helper that both call sites use, with defined recovery per failure point.

**Files:**
- Modify: `scripts/docket-status.sh` (add the helper above `sweep_execute_one`; rewrite the 0083 mark block inside `sweep_execute_one` to call it)
- Test: `tests/test_docket_status.sh` (existing change-0083 assert block, ~lines 1655–1705)

**Interfaces:**
- Consumes: `_docket_tree_wedged GIT DIR` from `scripts/lib/docket-preflight.sh` (already sourced by `docket-status.sh` — confirm with `grep -n 'docket-preflight.sh' scripts/docket-status.sh` before use); `$GIT`, `$DOCKET_BASH_PATH`, `$SCRIPTS_DIR`, `$INTEGRATION_BRANCH`.
- Produces: `sweep_mark_publish_deferred MW ARCHIVED ID DETAIL` — **always returns 0**, writes nothing to stdout or stderr. Task 3 calls it with a different `DETAIL`.

- [ ] **Step 1: Write the failing test**

The 0083 block's existing asserts must all keep passing (they are the regression net for the extraction). Add these to `tests/test_docket_status.sh` immediately after the existing `assert "0083: the sweep left the shared metadata worktree CLEAN"` (~line 1690):

```bash
# --- change 0118: the mark is a SHARED helper with defined recovery ----------------------------
# Extracting the 0083 block into sweep_mark_publish_deferred is only safe if the helper actually
# exists, is the single writer, and is called by BOTH sites. Asserted on the SOURCE, because a
# behavioral test cannot distinguish "inlined twice" from "extracted once" — and inlined-twice is
# exactly the drift this extraction exists to prevent.
assert "0118: sweep_mark_publish_deferred is defined once" \
  '[ "$(grep -c "^sweep_mark_publish_deferred()" "$SCRIPT")" -eq 1 ]'
assert "0118: mark-publish-deferred.sh is invoked from exactly ONE place in the sweep" \
  '[ "$(grep -c "mark-publish-deferred\.sh" "$SCRIPT")" -eq 1 ]'
assert "0118: the helper probes the shared worktree for a rebase/merge before committing" \
  'awk "/^sweep_mark_publish_deferred\(\)/,/^}/" "$SCRIPT" | grep -q "_docket_tree_wedged"'
assert "0118: the helper restores the archived path to HEAD when add/commit fails" \
  'awk "/^sweep_mark_publish_deferred\(\)/,/^}/" "$SCRIPT" | grep -qE "checkout HEAD -- "'
assert "0118: the helper never resets a committed marker on push failure" \
  '! awk "/^sweep_mark_publish_deferred\(\)/,/^}/" "$SCRIPT" | grep -qE "reset (--hard|--soft|--mixed)?"'
```

`$SCRIPT` is already defined at the top of `tests/test_docket_status.sh` as the path to `scripts/docket-status.sh` — **verify that before writing the block** (`grep -n '^SCRIPT=' tests/test_docket_status.sh`) and use whatever variable that file actually uses.

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -E "^(NOT )?OK.*0118"`
Expected: the four structural asserts are `NOT OK` (no such function yet). The `mark-publish-deferred.sh` count assert may already pass at 1 — that is fine and expected; it becomes load-bearing after the extraction.

- [ ] **Step 3: Add the helper**

Insert immediately **above** `sweep_execute_one(){` in `scripts/docket-status.sh`:

```bash
# sweep_mark_publish_deferred MW ARCHIVED ID DETAIL — write the durable `## Publish deferred`
# marker on an archived change whose expected publish did not complete, and land it on
# metadata_branch. THE SINGLE WRITER for both sweep paths that abandon an expected publish: the
# change-0083 terminal-publish failure, and (change 0118) the render-change-links skipped-publish
# failure. Extracted rather than inlined twice because the two blocks share an invariant, and a
# second copy is where that invariant would silently diverge.
#
# BEST-EFFORT toward the report stream: always returns 0, writes nothing to stdout or stderr, and
# no caller may branch on it. A failed mark must degrade to the pre-mark observable behavior
# EXACTLY — same report lines, same order, same control flow.
#
# TRANSACTIONAL toward the worktree, which bare `|| true` suppression is not (change 0118). The
# metadata worktree is SHARED: a marker that writes but fails to commit leaves the archived path
# dirty or staged, and every later pass's `pull --rebase` then fails for EVERY change — strictly
# worse than the unmarked gap this records. So recovery is defined per failure point:
#   - precondition, path not clean  -> skip entirely; never stack a marker onto a dirty state
#                                      some other actor left behind.
#   - precondition, tree wedged     -> skip entirely (change 0247's rule at the sweep's other
#                                      exposed commit). A commit into a mid-rebase tree writes
#                                      onto that rebase's DETACHED HEAD — and the restore below
#                                      would then resolve `HEAD` to that same detached commit and
#                                      corrupt the file it exists to repair.
#   - add or commit fails           -> restore the path to HEAD, index and worktree both.
#                                      Degraded outcome: unmarked gap, CLEAN worktree — exactly
#                                      today's behavior.
#   - commit succeeds, push fails   -> RETAIN the local commit. This is the correlated case: the
#                                      motivating renderer failure is a network blip, and the push
#                                      needs the same network. A clean unpushed commit is harmless
#                                      and self-heals — the next pass's `pull --rebase` carries it
#                                      and a later push from the shared worktree publishes it.
#                                      Never reset it; destroying it re-opens the gap.
#
# DETAIL must never contain the literal `terminal-publish.sh`: this invocation carries `--id` and
# no `--enabled`, and tests/test_closeout.sh's find_ungated_terminal_publish_call_sites scans
# JOINED logical lines for that literal regardless of quoting, so it would trip on this call site.
sweep_mark_publish_deferred(){
  local mw="$1" archived="$2" id="$3" detail="$4"
  [ -z "$("$GIT" -C "$mw" status --porcelain -- "$archived" 2>/dev/null)" ] || return 0
  ! _docket_tree_wedged "$GIT" "$mw" || return 0
  "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/mark-publish-deferred.sh --mode add --change-file "$archived" \
    --reason blocked --detail "$detail" \
    --integration-branch "$INTEGRATION_BRANCH" --id "$id" >/dev/null 2>&1 || return 0
  if "$GIT" -C "$mw" add -- "$archived" >/dev/null 2>&1 \
     && "$GIT" -C "$mw" commit -q -m "docket($id): mark terminal publish deferred (blocked)" -- "$archived" >/dev/null 2>&1; then
    "$GIT" -C "$mw" push >/dev/null 2>&1 || true
  else
    "$GIT" -C "$mw" checkout HEAD -- "$archived" >/dev/null 2>&1 || true
  fi
  return 0
}
```

Both `add` and `commit` carry a `--` pathspec so another agent's staged work is never swept in under this run's message (change 0247's other rule for this shared tree).

- [ ] **Step 4: Rewrite the 0083 block to call it**

Inside `sweep_execute_one`, in the `terminal-publish` failure branch, replace the whole `if "$DOCKET_BASH_PATH" … mark-publish-deferred.sh … fi` block (currently ~lines 951–957) with:

```bash
    sweep_mark_publish_deferred "$mw" "$archived" "$id" "sweep: the publish step exited non-zero"
```

Keep the entire explanatory comment above it, but replace its final paragraph (the one beginning `STRICTLY BEST-EFFORT:` and ending with the `find_ungated_terminal_publish_call_sites` sentence) with:

```bash
    # The mark's posture, its recovery on each failure point, and the --detail literal ban now
    # live once, on sweep_mark_publish_deferred above. Change 0118 also gave this path recovery it
    # did not have: the pre-0118 bare `&&` chain could leave the archived file dirty or staged in
    # the shared worktree when the marker wrote and `add`/`commit` failed.
```

Leave `echo "sweep-failed $id terminal-publish script-error"` and `return 0` exactly as they are.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -E "^NOT OK"`
Expected: no output. In particular **every** pre-existing `0083:` assert must still pass — they are the extraction's regression net, including `the marker reached the metadata branch as a commit`, `the sweep left the shared metadata worktree CLEAN`, and the four FAILED-mark invisibility asserts.

- [ ] **Step 6: Mutation-probe the wedge guard by DELETION and by INVERSION**

Deletion and inversion are different probes; a comparison or negation needs both.

```bash
cp scripts/docket-status.sh /tmp/ds-0118.bak

# (a) DELETION — remove the wedge probe line entirely.
before=$(grep -c "_docket_tree_wedged \"\$GIT\" \"\$mw\" || return 0" scripts/docket-status.sh)
perl -0pi -e 's/^\s*! _docket_tree_wedged "\$GIT" "\$mw" \|\| return 0\n//m' scripts/docket-status.sh
after=$(grep -c "_docket_tree_wedged \"\$GIT\" \"\$mw\" || return 0" scripts/docket-status.sh)
echo "deletion landed: $before -> $after (expect 1 then 0)"
bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds-0118.bak scripts/docket-status.sh

# (b) INVERSION — flip the sense of the probe.
perl -0pi -e 's/! _docket_tree_wedged "\$GIT" "\$mw" \|\| return 0/_docket_tree_wedged "\$GIT" "\$mw" || return 0/' scripts/docket-status.sh
grep -c '^  _docket_tree_wedged "\$GIT" "\$mw" || return 0' scripts/docket-status.sh
bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds-0118.bak scripts/docket-status.sh && rm -f /tmp/ds-0118.bak
```

Expected: (a) the deletion lands (`1 -> 0`) and the structural assert reddens (non-zero count). (b) The inversion lands (count `1`) and reddens **at least one behavioral assert** — because with the sense flipped, a *clean* (non-wedged) tree now skips the mark and `0083: the marker reached the metadata branch as a commit` must fail. If inversion leaves the suite green, the behavioral coverage is missing, not merely the structural one — report it rather than proceeding. Restore from the **backup copy** each time, never `git checkout --`.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh tests/test_docket_status.sh
git commit -m "fix(0118): one transactional helper owns the sweep's publish-deferred mark"
```

---

### Task 3: Mark on the `skipped-publish` branch, gated

**Files:**
- Modify: `scripts/docket-status.sh` (`sweep_execute_one`, the `render-change-links` failure branch, ~lines 897–901)
- Test: `tests/test_docket_status.sh` (a new docket-mode sweep run reusing the existing `$sweep_dir/work` fixture, plus a suppression run)

**Interfaces:**
- Consumes: `sweep_mark_publish_deferred MW ARCHIVED ID DETAIL` from Task 2.
- Produces: nothing later tasks consume.

- [ ] **Step 1: Re-scope the existing assert whose premise this task reverses**

`tests/test_docket_status.sh` currently asserts:

```bash
assert "0083: a change that never reached the publish step is never marked" \
  '! grep -q "^mark-publish-deferred .*broken-render" "$sweep_log"'
```

That encodes exactly the semantics this change reverses. It passes today only because that run is `DOCKET_MODE=main` with `TERMINAL_PUBLISH` unset — i.e. it is really a *suppression* assert wearing a general name. Rewrite it in place to say what it now proves, and keep it in the main-mode run:

```bash
# change 0118 re-scoped this from "a change that never reached the publish step is never marked".
# That name described the pre-0118 rule; the surviving property is narrower and is the one that
# still matters — under SUPPRESSION (this run is main-mode with TERMINAL_PUBLISH unset) a skipped
# publish is SUCCESS, not a deferral, so nothing is marked. The marking case has its own
# docket-mode run below.
assert "0118: under suppression (main-mode) a failed re-render is never marked" \
  '! grep -q "^mark-publish-deferred .*broken-render" "$sweep_log"'
```

Do **not** add change id 21 (`broken-render`) to any `TERMINAL_PUBLISH=true` run.

- [ ] **Step 2: Write the failing tests — a docket-mode marking run**

Append after the existing change-0083 assert block (after the `0083: a FAILED mark does not stop the loop` assert, ~line 1705). This reuses the SAME git fixture (`$sweep_dir/work`) rather than building a second one — the earlier run archived ids 20/21/23/24/25, so fresh ids are seeded and committed:

```bash
# --- change 0118: the skipped-publish leg marks, under the expected-publish gate ---------------
# The 0083 branch could not reach this: BOTH suppressions (main-mode, --enabled false) make
# terminal-publish.sh an exit-0 no-op, so its failure branch is structurally unreachable under
# suppression and needed no gate. A RENDERER failure fires regardless of the knob, so this branch's
# gate is load-bearing. Reuses the fixture repo above (fresh ids; the earlier run archived 20-25).
seed_sweep_change 26 mark-render-broken implemented
seed_sweep_change 27 gate-render-broken implemented
git -C "$sweep_dir/work" add docs/changes
git -C "$sweep_dir/work" commit -q -m "seed 0118 sweep changes"
git -C "$sweep_dir/work" push -q origin main

# The renderer mock fails on any slug containing "broken-render"; these slugs do not, so give the
# mock a second trigger. Rewrite it rather than adding a third mock file.
cat > "$tmp/mock-scripts/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-change-links $*" >> "$SWEEP_LOG"
case "$*" in *broken-render*|*render-broken*) exit 1 ;; esac
exit 0
EOF
chmod +x "$tmp/mock-scripts/render-change-links.sh"

mark_log="$tmp/sweep-calls-0118.log"; : > "$mark_log"
printf '26\tmark-render-broken\t26\t2026-08-11\n' > "$tmp/sweep-input-0118.tsv"
mark_out="$( cd "$sweep_dir/work" && \
  DOCKET_MODE=docket METADATA_WORKTREE=. CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main TERMINAL_PUBLISH=true \
  SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$mark_log" SWEEP_INPUT="$tmp/sweep-input-0118.tsv" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' )"

assert "0118: the report stream is byte-identical to today's — exactly the one skipped-publish line" \
  '[ "$(printf "%s\n" "$mark_out" | grep -c .)" -eq 1 ] \
   && grep -qxE "sweep-failed 26 render-change-links skipped-publish" <<<"$mark_out"'
assert "0118: the skipped-publish leg emits neither swept nor harvest" \
  '! grep -qE "^(swept|harvest) 26 " <<<"$mark_out"'
assert "0118: the failed re-render invokes mark-publish-deferred on the ARCHIVED change file" \
  'mk="$(grep -m1 "^mark-publish-deferred .*mark-render-broken" "$mark_log")"; \
   [ -n "$mk" ] && grep -q -- "--change-file .*archive/2026-08-11-0026-mark-render-broken.md" <<<"$mk"'
assert "0118: the mark is --mode add --reason blocked (no third reason value)" \
  'mk="$(grep -m1 "^mark-publish-deferred .*mark-render-broken" "$mark_log")"; \
   [ -n "$mk" ] && grep -q -- "--mode add" <<<"$mk" && grep -q -- "--reason blocked" <<<"$mk"'
assert "0118: the --detail names the re-render cause and the re-render-first instruction" \
  'mk="$(grep -m1 "^mark-publish-deferred .*mark-render-broken" "$mark_log")"; \
   [ -n "$mk" ] && grep -q -- "re-render" <<<"$mk" && grep -q -- "re-render before publishing" <<<"$mk"'
assert "0118: the --detail never spells the publisher script name (call-site scanner)" \
  'mk="$(grep -m1 "^mark-publish-deferred .*mark-render-broken" "$mark_log")"; \
   [ -n "$mk" ] && ! grep -qF -- "terminal-publish.sh" <<<"$mk"'
assert "0118: the mark carries the change id and the integration branch" \
  'mk="$(grep -m1 "^mark-publish-deferred .*mark-render-broken" "$mark_log")"; \
   [ -n "$mk" ] && grep -q -- "--id 26" <<<"$mk" && grep -q -- "--integration-branch main" <<<"$mk"'
assert "0118: the skipped-publish leg still abandons publish and cleanup" \
  '! grep -q "^terminal-publish .*--id 26 " "$mark_log" \
   && ! grep -q "^cleanup-feature-branch .*--slug mark-render-broken" "$mark_log"'
assert "0118: the marker reached the metadata branch as a COMMIT" \
  'body="$(git -C "$sweep_dir/work" show "HEAD:docs/changes/archive/2026-08-11-0026-mark-render-broken.md")"; \
   grep -qxF -- "## Publish deferred" <<<"$body"'
assert "0118: the marking run left the shared metadata worktree CLEAN" \
  '[ -z "$(git -C "$sweep_dir/work" status --porcelain)" ]'

# The gate, both legs. Under TERMINAL_PUBLISH=false a skipped publish is SUCCESS, never a deferral.
gate_log="$tmp/sweep-calls-0118-gate.log"; : > "$gate_log"
printf '27\tgate-render-broken\t27\t2026-08-11\n' > "$tmp/sweep-input-0118-gate.tsv"
gate_out="$( cd "$sweep_dir/work" && \
  DOCKET_MODE=docket METADATA_WORKTREE=. CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main TERMINAL_PUBLISH=false \
  SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$gate_log" SWEEP_INPUT="$tmp/sweep-input-0118-gate.tsv" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' )"
assert "0118 gate(TERMINAL_PUBLISH=false): the failed re-render is NOT marked" \
  '! grep -q "^mark-publish-deferred " "$gate_log"'
assert "0118 gate(TERMINAL_PUBLISH=false): the report line is unchanged" \
  'grep -qxE "sweep-failed 27 render-change-links skipped-publish" <<<"$gate_out"'
assert "0118 gate: the suppressed run still reached the renderer (non-vacuity)" \
  'grep -q "^render-change-links .*gate-render-broken" "$gate_log"'
```

The last assert is the non-vacuity companion for the two negatives: without it, a fixture that never ran the sweep at all would satisfy `! grep -q "^mark-publish-deferred "` and read as the gate holding.

- [ ] **Step 3: Run to verify they fail**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -E "^NOT OK"`
Expected: the marking asserts (`invokes mark-publish-deferred`, `--mode add --reason blocked`, `--detail names the re-render cause`, `marker reached the metadata branch as a COMMIT`) are red. The gate asserts and the report-stream asserts must already be **green** — they describe today's behavior. If a gate assert is red before implementation, the fixture is wrong.

- [ ] **Step 4: Write the implementation**

In `sweep_execute_one`, replace:

```bash
  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-change-links.sh \
        --change-file "$archived" --adrs-dir "$mw/$ADRS_DIR" >&2; then
    echo "sweep-failed $id render-change-links skipped-publish"
    return 0
  fi
```

with:

```bash
  if ! "$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/render-change-links.sh \
        --change-file "$archived" --adrs-dir "$mw/$ADRS_DIR" >&2; then
    # Change 0118: mark, so this gap is not invisible. The pre-0118 rationale — "nothing published
    # means nothing was deferred yet" — does not survive the code: once archived the change leaves
    # active/, and the sweep scans active/ ONLY, so no later pass ever resumes it. The gap is
    # permanent until a human acts, which is byte-for-byte the state ADR-0051 exists to surface.
    # The marker cannot flap here either — only terminal-publish.sh's success path strips it, and
    # nothing retries an archived change — so even a TRANSIENTLY caused mark is stable, not noisy
    # (and the cause can be transient: render-change-links.sh resolves config through
    # docket-config.sh --export, which does a `git fetch`, so a network blip fires this branch).
    #
    # The gate is LOAD-BEARING here, and is the one structural difference from the change-0083
    # block above, which needs none: both of that branch's suppressions are exit-0 no-ops, so it is
    # unreachable under suppression, whereas a renderer failure fires regardless of the knob. Under
    # `terminal_publish: false` or in main-mode a skipped publish is SUCCESS, never a deferral
    # (ADR-0051) — the residual there stays what docket-status.md already documents: a stale
    # `## Artifacts` block, fixed by a manual re-render.
    if [ "${TERMINAL_PUBLISH:-false}" = true ] && [ "${DOCKET_MODE:-}" = docket ]; then
      sweep_mark_publish_deferred "$mw" "$archived" "$id" \
        "sweep: the artifacts re-render failed, so the publish was never attempted — re-render before publishing"
    fi
    echo "sweep-failed $id render-change-links skipped-publish"
    return 0
  fi
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh`
Expected: zero `NOT OK` lines.

- [ ] **Step 6: Mutation-probe the gate — deletion AND inversion**

```bash
cp scripts/docket-status.sh /tmp/ds3-0118.bak

# (a) DELETION of the gate: mark unconditionally.
before=$(grep -c 'TERMINAL_PUBLISH:-false}" = true \] && \[ "\${DOCKET_MODE:-}" = docket' scripts/docket-status.sh)
perl -0pi -e 's/if \[ "\$\{TERMINAL_PUBLISH:-false\}" = true \] && \[ "\$\{DOCKET_MODE:-\}" = docket \]; then\n(\s*sweep_mark_publish_deferred[^\n]*\n[^\n]*\n)\s*fi\n/$1/' scripts/docket-status.sh
after=$(grep -c 'TERMINAL_PUBLISH:-false}" = true \] && \[ "\${DOCKET_MODE:-}" = docket' scripts/docket-status.sh)
echo "gate deletion landed: $before -> $after (expect 1 then 0)"
bash -n scripts/docket-status.sh && bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds3-0118.bak scripts/docket-status.sh

# (b) INVERSION of the mode leg only.
perl -0pi -e 's/"\$\{DOCKET_MODE:-\}" = docket/"\$\{DOCKET_MODE:-\}" != docket/' scripts/docket-status.sh
grep -c '"${DOCKET_MODE:-}" != docket' scripts/docket-status.sh
bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds3-0118.bak scripts/docket-status.sh && rm -f /tmp/ds3-0118.bak
```

Expected: (a) the deletion lands, the script still parses (`bash -n`), and the `TERMINAL_PUBLISH=false` gate assert reddens. (b) The inversion lands and the docket-mode marking asserts redden. Both counts must be **non-zero**; a zero count with a landed mutation means the gate is untested. If the `perl -0pi` substitution reports `before=1 after=1`, the pattern did not match — **the mutation did not land and the green means nothing**; fix the pattern (or edit by hand) and re-run before believing any result.

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.sh tests/test_docket_status.sh
git commit -m "fix(0118): the sweep's skipped-publish leg marks under the expected-publish gate"
```

---

### Task 4: Prove the transactional posture with fault injection

A mocked marker that only *fails* cannot exercise recovery past the marker write. Force the failure at the git step instead, through `docket-status.sh`'s documented `GIT="${GIT:-git}"` mock seam.

**Files:**
- Test only: `tests/test_docket_status.sh` (append after Task 3's block)

**Interfaces:**
- Consumes: `sweep_mark_publish_deferred` (Task 2) and the gated call site (Task 3); the `$sweep_dir/work` fixture and `$tmp/mock-scripts`.
- Produces: nothing.

- [ ] **Step 1: Write the failing tests**

```bash
# --- change 0118: the transactional posture, forced at the git step ----------------------------
# A mocked marker that only FAILS cannot reach these paths — the failure has to happen AFTER the
# marker has already dirtied the archived file. docket-status.sh's documented GIT mock seam
# (`GIT="${GIT:-git}"`, script header) is the injection point: a wrapper that fails one subcommand
# and passes everything else through to real git.
mkgitfail(){
  # $1 = the git subcommand to fail on. Prints the wrapper's path.
  local d; d="$tmp/gitfail-$1"; mkdir -p "$d"
  cat > "$d/git" <<EOF
#!/usr/bin/env bash
# Scan for the subcommand as a bare word — \`-C <dir>\` precedes it, and a path could contain it.
for a in "\$@"; do
  case "\$a" in -*) continue ;; esac
  [ "\$a" = "$1" ] && exit 1
  break
done
exec /usr/bin/env git "\$@"
EOF
  chmod +x "$d/git"
  printf '%s\n' "$d/git"
}

fault_run(){
  # \$1 = id, \$2 = slug, \$3 = git wrapper path, \$4 = log path. Prints the sweep's report stream.
  local id="$1" slug="$2" gitbin="$3" log="$4"
  : > "$log"
  printf '%s\t%s\t%s\t2026-08-11\n' "$id" "$slug" "$id" > "$tmp/sweep-input-fault-$id.tsv"
  ( cd "$sweep_dir/work" && \
    DOCKET_MODE=docket METADATA_WORKTREE=. CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
    INTEGRATION_BRANCH=main METADATA_BRANCH=main TERMINAL_PUBLISH=true GIT="$gitbin" \
    SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$log" SWEEP_INPUT="$tmp/sweep-input-fault-$id.tsv" \
    bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' )
}

seed_sweep_change 28 add-fail-render-broken implemented
seed_sweep_change 29 commit-fail-render-broken implemented
seed_sweep_change 30 push-fail-render-broken implemented
git -C "$sweep_dir/work" add docs/changes
git -C "$sweep_dir/work" commit -q -m "seed 0118 fault-injection changes"
git -C "$sweep_dir/work" push -q origin main

# --- add fails -> the archived path is restored to HEAD, worktree AND index clean ---------------
add_out="$(fault_run 28 add-fail-render-broken "$(mkgitfail add)" "$tmp/fault-add.log")"
assert "0118 fault(add): the marker WAS attempted (non-vacuity — the failure is past the write)" \
  'grep -q "^mark-publish-deferred .*add-fail-render-broken" "$tmp/fault-add.log"'
assert "0118 fault(add): the archived path is clean — worktree and index both" \
  '[ -z "$(git -C "$sweep_dir/work" status --porcelain -- docs/changes/archive/2026-08-11-0028-add-fail-render-broken.md)" ]'
assert "0118 fault(add): the report stream is unchanged" \
  'grep -qxE "sweep-failed 28 render-change-links skipped-publish" <<<"$add_out"'
assert "0118 fault(add): no marker section survives on the archived file" \
  '! grep -qxF -- "## Publish deferred" "$sweep_dir/work/docs/changes/archive/2026-08-11-0028-add-fail-render-broken.md"'

# --- commit fails -> same clean restore --------------------------------------------------------
commit_out="$(fault_run 29 commit-fail-render-broken "$(mkgitfail commit)" "$tmp/fault-commit.log")"
assert "0118 fault(commit): the marker WAS attempted (non-vacuity)" \
  'grep -q "^mark-publish-deferred .*commit-fail-render-broken" "$tmp/fault-commit.log"'
assert "0118 fault(commit): the archived path is clean — worktree and index both" \
  '[ -z "$(git -C "$sweep_dir/work" status --porcelain -- docs/changes/archive/2026-08-11-0029-commit-fail-render-broken.md)" ]'
assert "0118 fault(commit): the report stream is unchanged" \
  'grep -qxE "sweep-failed 29 render-change-links skipped-publish" <<<"$commit_out"'

# --- push fails -> the local marker commit is RETAINED for self-healing -------------------------
push_out="$(fault_run 30 push-fail-render-broken "$(mkgitfail push)" "$tmp/fault-push.log")"
assert "0118 fault(push): the local marker commit is RETAINED, never reset" \
  'body="$(git -C "$sweep_dir/work" show "HEAD:docs/changes/archive/2026-08-11-0030-push-fail-render-broken.md")"; \
   grep -qxF -- "## Publish deferred" <<<"$body"'
assert "0118 fault(push): the worktree is left clean despite the failed push" \
  '[ -z "$(git -C "$sweep_dir/work" status --porcelain)" ]'
assert "0118 fault(push): the report stream is unchanged" \
  'grep -qxE "sweep-failed 30 render-change-links skipped-publish" <<<"$push_out"'
```

- [ ] **Step 2: Run the git-wrapper helper on its own before trusting any of it**

Plan-supplied fixture code is unverified code. Prove the wrapper does what it claims before reading any assert's colour:

```bash
cd "$(git rev-parse --show-toplevel)"
d=$(mktemp -d); cat > "$d/git" <<'EOF'
#!/usr/bin/env bash
for a in "$@"; do
  case "$a" in -*) continue ;; esac
  [ "$a" = "push" ] && exit 1
  break
done
exec /usr/bin/env git "$@"
EOF
chmod +x "$d/git"
"$d/git" -C . rev-parse --abbrev-ref HEAD; echo "passthrough rc=$?  (expect 0 and a branch name)"
"$d/git" -C . push --dry-run origin HEAD 2>/dev/null; echo "blocked rc=$?  (expect 1)"
rm -rf "$d"
```

Expected: passthrough prints a branch name at rc=0; the blocked call exits 1 **without contacting the network**. If passthrough fails, the wrapper is broken and every fault assert below it is meaningless. Note the loop breaks at the first non-flag word, so a `-C <dir>` whose *directory* is named `push` cannot false-trigger.

- [ ] **Step 3: Run the tests**

Run: `bash tests/test_docket_status.sh 2>&1 | grep -E "^NOT OK"`
Expected: no output — Tasks 2 and 3 already implement the recovery, so these are characterization tests for code that exists. If `fault(push)` fails, check the restore branch is not firing on the push leg; if `fault(add)`/`fault(commit)` fail, check the `else` arm's `checkout HEAD --`.

- [ ] **Step 4: Mutation-probe the recovery arms — deletion and inversion**

```bash
cp scripts/docket-status.sh /tmp/ds4-0118.bak

# (a) DELETION of the restore: replace the else-arm's checkout with a no-op.
before=$(grep -c 'checkout HEAD -- "$archived"' scripts/docket-status.sh)
perl -0pi -e 's/"\$GIT" -C "\$mw" checkout HEAD -- "\$archived" >\/dev\/null 2>&1 \|\| true/:/' scripts/docket-status.sh
after=$(grep -c 'checkout HEAD -- "$archived"' scripts/docket-status.sh)
echo "restore deletion landed: $before -> $after (expect 1 then 0)"
bash -n scripts/docket-status.sh && bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds4-0118.bak scripts/docket-status.sh

# (b) INVERSION — reset the commit on push failure instead of retaining it.
perl -0pi -e 's/"\$GIT" -C "\$mw" push >\/dev\/null 2>&1 \|\| true/"\$GIT" -C "\$mw" push >\/dev\/null 2>&1 || "\$GIT" -C "\$mw" reset --hard HEAD~1 >\/dev\/null 2>&1/' scripts/docket-status.sh
grep -c 'reset --hard HEAD~1' scripts/docket-status.sh
bash -n scripts/docket-status.sh && bash tests/test_docket_status.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ds4-0118.bak scripts/docket-status.sh && rm -f /tmp/ds4-0118.bak
```

Expected: (a) lands `1 -> 0` and reddens the two `archived path is clean` asserts. (b) lands (count `1`) and reddens `fault(push): the local marker commit is RETAINED` **and** the Task-2 structural assert that bans `reset`. Non-zero counts required in both; a green run with a landed mutation is a finding about the tests.

- [ ] **Step 5: Commit**

```bash
git add tests/test_docket_status.sh
git commit -m "test(0118): fault-inject the mark's git steps to prove the transactional posture"
```

---

### Task 5: State the real reason in the three docs

**Files:**
- Modify: `scripts/docket-status.md` (§6 — the paragraph containing `nothing published means nothing was deferred *yet*`)
- Modify: `skills/docket-status/SKILL.md` (the `**Sweep posture:**` paragraph under `### Merge sweep`)
- Modify: `skills/docket-convention/references/terminal-close-out.md` (step 3's mark rule; the `**The skip-publish guard (all callers):**` sentence)
- Test: `tests/test_closeout.sh` (append before the final `0174 template integrity` assert)

**Interfaces:** none — documentation and contract sentinels only.

- [ ] **Step 1: Rewrite `scripts/docket-status.md` §6's rationale**

Replace exactly this sentence:

```
The `render-change-links`
case is not marked at all: nothing published means nothing was deferred *yet*, and the close-out
never reached the publish step. The knob narrows only the
`terminal-publish` leg: under `terminal_publish: false` that step is a no-op that cannot fail, so
this recovery path never arises there — but the renderer leg still can fail in such a repo, leaving
the archived change with a stale `## Artifacts` block on `metadata_branch` that no later sweep
resumes; the follow-up there is a manual re-render on the metadata branch, not a publish.
```

with:

```
The `render-change-links` case marks too (change
0118), under the same best-effort posture and one extra gate: `TERMINAL_PUBLISH=true` **and**
docket-mode. That gate is load-bearing on this leg and absent on the other, because both of the
publish's suppressions are exit-0 no-ops — so the `terminal-publish` branch is unreachable under
suppression, while a renderer failure fires regardless of the knob. The pre-0118 rationale
("nothing published means nothing was deferred *yet*") does not survive the code: once archived the
change leaves `active/`, the sweep scans `active/` only, and no later pass resumes it — so the gap
is permanent until a human acts, which is exactly what ADR-0051's marker exists to surface. Whether
the publish was deferred, blocked, or never reached is a distinction about *cause*, and cause
travels in the dated `--detail` line. Under suppression the leg stays unmarked, because a suppressed
publish is *success*, not a deferral (ADR-0051); the residual there is unchanged — the archived
change keeps a stale `## Artifacts` block on `metadata_branch` that no later sweep resumes, and the
follow-up is a manual re-render on the metadata branch, not a publish. Both marks share one writer,
`sweep_mark_publish_deferred`, which skips entirely when the archived path is already dirty or the
shared worktree is mid-rebase/merge, restores the path to `HEAD` if `add`/`commit` fails, and
retains a committed-but-unpushed marker so the next pass's `pull --rebase` carries it.
```

- [ ] **Step 2: Rewrite the `skills/docket-status/SKILL.md` sweep-posture sentences**

This file has **22 words of headroom** (2478/2500) — Task 6 raises the budget, but keep the edit tight anyway. Replace exactly these two sentences:

```
The `terminal-publish` case is no longer invisible: the sweep marks the archived file itself (`mark-publish-deferred.sh --mode add --reason blocked`, committed and pushed on `metadata_branch`) before emitting the line, so the `publish-deferred` health check below surfaces it on every later pass until the publish lands — change 0083. That mark is best-effort and its failure changes nothing above, so an **unmarked** deferral is still invisible; see `skills/docket-convention/references/terminal-close-out.md`'s mark step for the rule every driver owes on any other non-completion path.
```

with:

```
Neither case is invisible any more: the sweep marks the archived file itself (`mark-publish-deferred.sh --mode add --reason blocked`, committed and pushed on `metadata_branch`) before emitting the line, so the `publish-deferred` health check below surfaces it on every later pass until the publish lands — change 0083 for `terminal-publish`, change 0118 for `skipped-publish`. The `skipped-publish` mark carries one extra gate the other does not need — `terminal_publish: true` **and** docket-mode — because a suppressed publish is *success*, and unlike a failed publish a failed re-render fires regardless of the knob. That mark is best-effort and its failure changes nothing above, so an **unmarked** deferral is still invisible; see `skills/docket-convention/references/terminal-close-out.md`'s mark step for the rule every driver owes on any other non-completion path.
```

Leave the rest of the paragraph — including 0247's `blocked-wedged-tree` sentence and the cross-check guidance — byte-untouched. Then delete the now-false trailing sentence of that same paragraph if and only if it still reads `A failed `docket.sh render-change-links` follow-on skips publish (a stale `## Artifacts` block is never published).` — **keep** it: the skip-publish guard itself is out of scope and unchanged.

- [ ] **Step 3: Extend the rule in `terminal-close-out.md`**

(a) In step 3, after the paragraph beginning `**When the publish is expected but does NOT complete — mark it (change 0083).**` and its command block, before `**Never mark under suppression.**`, insert:

```markdown
   **The rule reaches every HANDLED path that abandons an expected publish (change 0118)** — not
   "any path": a hard crash between archive and publish can write nothing by definition, and that
   residual stays accepted per ADR-0051, so this rule must not claim coverage it cannot enforce.
   Scope it per leg, because the drivers diverge:

   - A failed **step-2 re-render** abandons the publish for *every* driver, and every driver is
     required by this contract to mark there. The `docket-status` sweep discharges that duty in
     code (change 0118); the three skill-driven drivers discharge it by following this rule — no
     executable enforcement is added for them, so read it as their duty, not as an accomplished
     fact.
   - A failed **step-2 commit/push** skips the publish only in the skill-driven drivers, which is
     where the mark is owed. The sweep deliberately **continues** to publish on that leg (change
     0075 §5, documented in `scripts/docket-status.md` §6a), so it owes no mark there.
```

(b) Replace the guard sentence:

```
**The skip-publish guard (all callers):** a failed step 1 skips steps 2–3; a **failed step-2
commit/push skips step 3** — a stale `## Artifacts` block must never be published.
```

with:

```
**The skip-publish guard:** a failed step 1 skips steps 2–3 for every caller; a **failed step-2
commit/push skips step 3** in the skill-driven callers — a stale `## Artifacts` block must never
be published. The `docket-status` sweep is **carved out of that second clause**: on the
commit/push leg it continues to publish (change 0075 §5), because there the block is merely stale
and cosmetic while an aborted close-out is not — see `scripts/docket-status.md` §6a. A failed
step-2 **re-render** skips step 3 for every caller, sweep included.
```

- [ ] **Step 4: Write the contract sentinels**

Append to `tests/test_closeout.sh`, immediately before the final `0174 template integrity` assert. `$TCO` is already defined there as the path to `terminal-close-out.md` — **verify with `grep -n '^TCO=' tests/test_closeout.sh` before writing this block** and use whatever variable that file actually uses.

```bash
# --- change 0118: the mark rule reaches every HANDLED abandonment, scoped per leg ---------------
# Written as NEGATIVES against the states 0118 removed, plus positives, because a guard that only
# confirms the new wording is green the moment the edit lands and stays green even if the old
# claim is reintroduced beside it.
assert "0118: the guard no longer claims the commit/push clause binds ALL callers" \
  '! grep -qF -- "The skip-publish guard (all callers):" "$TCO"'
assert "0118: the guard carves the sweep out of the commit/push clause" \
  'grep -qF -- "carved out of that second clause" "$TCO"'
assert "0118: the carve-out points at the sweep deviation's owning doc" \
  'grep -qF -- "scripts/docket-status.md\` §6a" "$TCO"'
assert "0118: the mark rule states it reaches HANDLED paths, not any path" \
  'grep -qF -- "every HANDLED path that abandons an expected publish" "$TCO"'
assert "0118: the mark rule keeps the hard-crash residual explicitly out of scope" \
  'grep -qF -- "a hard crash between archive and publish can write nothing" "$TCO"'
assert "0118: the re-render leg is stated as owed by EVERY driver" \
  'grep -qF -- "abandons the publish for *every* driver" "$TCO"'
assert "0118: the sweep is stated to owe no mark on the commit/push leg" \
  'grep -qF -- "so it owes no mark there" "$TCO"'
assert "0118: never-mark-under-suppression survives the edit (non-vacuity)" \
  'grep -qF -- "**Never mark under suppression.**" "$TCO"'
```

- [ ] **Step 5: Run the tests**

Run: `bash tests/test_closeout.sh 2>&1 | grep -E "^NOT OK"` — expected: no output.
Then re-run the whole call-site scanner assert specifically, since Task 3 added a new `--detail` string near a `--id`:

```bash
bash tests/test_closeout.sh 2>&1 | grep -E "omits --enabled"
```
Expected: `ok - 0064 wiring: no terminal-publish.sh call site (skills/ prose or scripts/*.sh) omits --enabled`.

- [ ] **Step 6: Mutation-probe one negative sentinel**

```bash
cp skills/docket-convention/references/terminal-close-out.md /tmp/tco-0118.bak
before=$(grep -c "The skip-publish guard:" skills/docket-convention/references/terminal-close-out.md)
perl -0pi -e 's/\*\*The skip-publish guard:\*\*/**The skip-publish guard (all callers):**/' skills/docket-convention/references/terminal-close-out.md
after=$(grep -c "The skip-publish guard (all callers):" skills/docket-convention/references/terminal-close-out.md)
echo "mutation landed: $before -> $after (expect 1 then 1)"
bash tests/test_closeout.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/tco-0118.bak skills/docket-convention/references/terminal-close-out.md && rm -f /tmp/tco-0118.bak
```
Expected: the mutation lands and the `no longer claims ... ALL callers` assert reddens (non-zero count).

- [ ] **Step 7: Commit**

```bash
git add scripts/docket-status.md skills/docket-status/SKILL.md \
        skills/docket-convention/references/terminal-close-out.md tests/test_closeout.sh
git commit -m "docs(0118): the skipped-publish leg marks — three docs state the real reason"
```

---

### Task 6: Re-budget with measured numbers, then gate on the full suite

Both budgets were at parity **before** this change: `tests/test_docket_status.sh` measured 45.15s serial against a 45s row, and `skills/docket-status/SKILL.md` measured 2478 words against 2500. This task resolves both deliberately, with an argument in the diff — never by quietly shrinking prose to squeak under a ceiling.

**Files:**
- Modify: `tests/runtime-budgets.tsv` (the `tests/test_docket_status.sh` row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)
- Modify: `tests/test_skill_size_budgets.sh` (the `skills/docket-status/SKILL.md` row + a BUDGETS comment entry)

**Interfaces:** none.

- [ ] **Step 1: Measure — standalone and serial, three times, take the worst**

Never use `scripts/run-tests.sh --timings <test path>` against a real test file: it truncates the named file to zero bytes (tracked as #0290, unfixed).

```bash
cd "$(git rev-parse --show-toplevel)"
for i in 1 2 3; do /usr/bin/time -p bash tests/test_docket_status.sh >/dev/null; done 2>&1 | grep '^real'
wc -w < skills/docket-status/SKILL.md
awk 'END{print NR}' skills/docket-status/SKILL.md
for i in 1 2 3; do /usr/bin/time -p bash tests/test_closeout.sh >/dev/null; done 2>&1 | grep '^real'
/usr/bin/time -p bash tests/test_mark_publish_deferred.sh >/dev/null 2>&1
```

Record the **worst standalone serial** reading for each. Pre-change references: `test_docket_status.sh` 45.15s (row 45), `test_closeout.sh` 12.50s (row 20), `test_mark_publish_deferred.sh` 0.92s (row 10), SKILL.md 2478 words / 102 lines (budget 2500 / 118).

- [ ] **Step 2: Raise the runtime row**

Apply change 0137's rule as the table's own header states it: **round the worst standalone serial reading up to the next multiple of 5, then add a 5s margin**. For a measured 45–50s that is a row of **55**; for a measured 50–55s it is **60**, which is the table's hard ceiling — if the measurement lands above 55s, do **not** raise past 60: stop and report, because the remedy is then a shard, not a number.

In `tests/runtime-budgets.tsv`, change the row and add the rationale to the header comment block (the table has no per-row comment syntax, so it goes with the other header notes):

```
tests/test_docket_status.sh	<new>	parallel
```

Add to the header comment, above the rows:

```
# tests/test_docket_status.sh was raised 45 -> <new> by change 0118. It was measured at 45.15s
# STANDALONE SERIAL before the change — already at parity with its own row, passing only on the
# runner's slack factor — and 0118 adds three sweep runs plus three fault-injection runs to the one
# file that owns sweep_execute coverage. Per this header's rule, from the worst standalone serial
# reading (<measured>s): next multiple of 5 is <m5>, plus the 5s margin -> <new>. Sharding was
# considered and rejected HERE: changes 0268 and 0154 are both queued against this same file, and
# re-cutting shards is precisely the edit that cannot rebase cleanly past them — the shard belongs
# to whichever change can take the whole file, not to one adding six fixtures to it.
```

Then re-seed `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` — the guard pins the SUM of every ceiling, so any raise reddens until the total moves with it. Compute it, never hand-add:

```bash
awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
```
Set `EXPECTED_TOTAL` to that number and note in its comment that change 0118 raised the `test_docket_status.sh` row.

- [ ] **Step 3: Raise the skill word budget**

Apply the BUDGETS comment's own rule: **words round up to the next multiple of 50, and if that lands within 25 words of the actual, the multiple after it**. Update the row in `tests/test_skill_size_budgets.sh`:

```
skills/docket-status/SKILL.md                              118 <newWords>
```

Raise the **line** budget only if the measured line count exceeds 118 (it was 102; this change adds no new paragraphs, only re-writes existing ones). Add to the BUDGETS comment block, after the existing change-0247 entry, the in-diff argument change 0201 requires — it must NAME the references file the prose was considered for and say why it cannot live there:

```
# skills/docket-status/SKILL.md's WORD budget was raised 2500 -> <newWords> by change 0118, which
# corrected the sweep-posture paragraph: the `skipped-publish` leg now marks like the
# `terminal-publish` leg, under one extra gate (terminal_publish true AND docket-mode) that the
# other leg does not need. Considered for and REJECTED from
# skills/docket-convention/references/terminal-close-out.md, which already carries this change's
# cross-driver mark RULE: that file states what every driver owes, whereas this paragraph states
# what the sweep's report lines MEAN to an agent reading them right now — an agent triaging a
# `sweep-failed … skipped-publish` line is reading SKILL.md, and a correction sitting in a
# reference it has no reason to open would leave it acting on the old "not marked at all" claim.
# The paragraph was tightened in the same edit (two sentences merged into one lead), so the raise
# covers the gate clause only. Set per the rounding rule above from the measured actual:
# <measured> words -> <newWords>. The LINE budget was not raised (<lines> still fits 118).
```

- [ ] **Step 4: Run both budget guards**

```bash
bash tests/test_runtime_budgets.sh 2>&1 | grep -E "^NOT OK" ; echo "runtime rc=$?"
bash tests/test_skill_size_budgets.sh 2>&1 | grep -E "^NOT OK" ; echo "size rc=$?"
```
Expected: no `NOT OK` lines from either.

- [ ] **Step 5: Run the WHOLE suite — the build gate**

The suite command is whatever `finalize.test_command` resolves to; read it there, never from a second copy. For this repo it resolves to `scripts/run-tests.sh`. Run the whole suite, never only the files this plan touched:

```bash
cd "$(git rev-parse --show-toplevel)"
bash scripts/run-tests.sh
```

Expected: green. Two known conditions, neither of which is this branch's to fix:
- `tests/test_sync_agents_runners.sh` at ~193s against a 60s ceiling is **pre-existing**, tracked as #0280. Leave it alone.
- A red `tests/test_gate_run_stop.sh` at the assert `the stop is held where the completed marker would be written` is known flake **#0293** (fixture deadline at exact parity). It reproduces on clean `origin/main`. Re-run and proceed; do not diagnose it here.

A trailing `OVER BUDGET:` line naming any file **this change touched** is a finding to act on, not noise — it does not fail the run, so nothing else will catch it. Return to Step 1 and re-measure rather than leaving it.

- [ ] **Step 6: Commit**

```bash
git add tests/runtime-budgets.tsv tests/test_runtime_budgets.sh tests/test_skill_size_budgets.sh
git commit -m "test(0118): re-budget test_docket_status.sh and docket-status/SKILL.md from measured actuals"
```

---

## Self-review notes

**Spec coverage.** §1 (the gated mark, transactional posture, back-port to the 0083 block, unchanged report contract, `--detail` literal ban) → Tasks 2–4. §2 (body-prose generalization + the re-run whole-repo grep) → Task 1. §3 (three docs, per-leg scoping, the "(all callers)" carve-out) → Task 5. §4 (explicitly not done) → enforced by the Global Constraints and by Task 2's `grep -c` structural asserts. Spec *Tests* section: mocked-marker invocation and gating → Task 3; fault injection past the marker write → Task 4; real-marker truthfulness → Task 1; the re-scoped "never reached the publish step" assert → Task 3 Step 1; the docket-mode fixture friction (`METADATA_WORKTREE=.`) → Task 3 Step 2; assert on logged argv rather than rendered content → Task 3.

**One deliberate deviation from the spec, carried from the reconcile pass.** The spec's precondition is "the archived path is clean"; this plan widens it to *clean **and** not wedged*. Change 0247 landed after the spec was written and taught the sweep's other exposed commit to refuse a mid-rebase tree. The widening is a correctness requirement of the spec's own recovery step, not a scope addition: inside a rebase, `HEAD` resolves to the rebase's detached HEAD, so `checkout HEAD -- "$archived"` would restore the wrong bytes.

**Two budget raises are expected, not incidental**, and both were at parity before this change began. Task 6 owns them; neither is resolved by shrinking prose to fit.
