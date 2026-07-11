# board-refresh.sh non-empty guard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `scripts/board-refresh.sh` leave `BOARD.md` untouched (and exit non-zero) when `render-board.sh` exits 0 but produces empty output, closing the last hole in its no-truncate guarantee.

**Architecture:** `board-refresh.sh` (from change #0059) already renders `render-board.sh` into a `mktemp` temp file and only `mv`s it onto `BOARD.md` after the renderer exits 0. This change adds a second gate — the temp file must also be **non-empty** (`[ -s ]`) — between the existing exit-code check and the `mv`. The empty branch prints a distinct message, leaves `BOARD.md` byte-identical (the `EXIT` trap removes the temp file), and exits non-zero so callers skip their `git add`/commit. `render-board.sh` is not touched.

**Tech Stack:** POSIX-ish Bash (`set -uo pipefail`), the repo's hermetic shell-test harness (`tests/test_board_refresh.sh` with its `RENDER_BOARD` mock seam and `assert` helper).

## Global Constraints

- `render-board.sh` stays an **unchanged** pure stdout renderer — the guard lives entirely in `board-refresh.sh`.
- The non-empty check MUST come **after** the existing `rc != 0` check, so a genuine renderer failure still propagates the renderer's real exit code (not the empty-render code).
- Empty-render failure exit code: **`1`** (distinct from the usage `exit 2` and from a propagated renderer code); stderr message must name empty output so it is distinguishable in logs from the non-zero-exit branch.
- No structural/format validation of rendered content beyond non-empty (a `# Backlog` H1 check is out of scope, rejected as YAGNI in the spec).
- Test mock discipline (LEARNINGS #58): the `RENDER_BOARD` stub must mirror what the real renderer could actually do — exit **0** with **empty** stdout — not a shape the renderer never emits.

---

### Task 1: Non-empty guard in board-refresh.sh (+ contract note)

**Files:**
- Modify: `scripts/board-refresh.sh` (insert guard between the `rc` check ending ~line 78 and the `chmod 644` at ~line 82)
- Modify: `scripts/board-refresh.md` (Behavior + Exit codes sections — document the two-part success condition and the empty-render failure)
- Test: `tests/test_board_refresh.sh` (append a new case after the existing test #10)

**Interfaces:**
- Consumes: the existing `board-refresh.sh` internals — `$tmp_board` (the mktemp temp file inside `$CHANGES_DIR`), `$RENDER_BOARD` (mock seam), the `trap 'rm -f "$tmp_board"' EXIT` cleanup, and `$CHANGES_DIR/BOARD.md` (the target).
- Produces: no new external interface — same CLI. New observable behavior: `board-refresh.sh --surfaces inline` exits `1` and leaves `BOARD.md` byte-identical when the renderer exits 0 with empty output.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_board_refresh.sh`, immediately **before** the final `if [ "$fail" = 0 ]; then echo "PASS"; ...` block:

```bash
# --- 11: enabled render that exits 0 with EMPTY output must NOT overwrite BOARD.md. render-board.sh
# always emits a `# Backlog` header on a clean run, so this cannot happen with the real renderer —
# but the guard must be self-contained (a future render-board regression / the mock seam could hit
# it). Belt-and-suspenders companion to test #9 (non-zero exit). Mirrors what a real renderer could
# do: exit 0, print nothing.
empty_stub="$work/empty-render.sh"
cat > "$empty_stub" <<'EOF'
#!/usr/bin/env bash
# Emits nothing, exits 0 — the exit-0-but-empty case.
exit 0
EOF
chmod +x "$empty_stub"
rm -f "$tmp/BOARD.md"
printf '# Known Good Board\n\nPre-existing, must survive an empty render.\n' > "$tmp/BOARD.md"
cp "$tmp/BOARD.md" "$work/pre-empty-board.md"
RENDER_BOARD="$empty_stub" "$SCRIPT" --changes-dir "$tmp" --surfaces "inline" >"$work/out11" 2>"$work/err11"; rc11=$?
assert "empty render: exits non-zero (1), not 0" '[ "$rc11" -eq 1 ]'
assert "empty render: pre-existing BOARD.md untouched (byte-identical)" \
  'diff -u "$work/pre-empty-board.md" "$tmp/BOARD.md"'
assert "empty render: no leftover temp file in changes dir (only BOARD.md remains)" \
  '[ "$(count_files)" -eq 1 ]'
assert "empty render: reports empty output on stderr" 'grep -qF "empty output" "$work/err11"'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_refresh.sh`
Expected: FAIL — the four new assertions report `NOT OK`. Without the guard, the empty temp file is `chmod`ed and `mv`ed onto `BOARD.md`, so: `rc11` is `0` (not `1`), `BOARD.md` is now empty (diff fails), and stderr has no "empty output" line. Overall script prints `FAIL` and exits non-zero.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/board-refresh.sh`, locate this existing block:

```bash
"$RENDER_BOARD" "${render_args[@]}" > "$tmp_board"
rc=$?
if [ "$rc" -ne 0 ]; then
  printf 'board-refresh: render-board.sh failed (exit %d); BOARD.md left untouched\n' "$rc" >&2
  exit "$rc"
fi

# mktemp creates the temp file at 0600; normalize to 0644 (the git-tracked, pushed board's mode)
```

Insert the non-empty guard **between** the `fi` (closing the `rc` check) and the `# mktemp …` comment, so the block reads:

```bash
"$RENDER_BOARD" "${render_args[@]}" > "$tmp_board"
rc=$?
if [ "$rc" -ne 0 ]; then
  printf 'board-refresh: render-board.sh failed (exit %d); BOARD.md left untouched\n' "$rc" >&2
  exit "$rc"
fi

# Second gate: the render exited 0 but must also be NON-EMPTY before it replaces BOARD.md. A
# zero-exit-but-empty render (a future render-board.sh regression, or an injected stub) would
# otherwise mv an empty file over a good board. Leave BOARD.md byte-identical (the EXIT trap
# removes the temp file) and exit non-zero so the caller skips its git add/commit — the
# belt-and-suspenders companion to the non-zero-exit branch above.
if [ ! -s "$tmp_board" ]; then
  printf 'board-refresh: render produced empty output; BOARD.md left untouched\n' >&2
  exit 1
fi

# mktemp creates the temp file at 0600; normalize to 0644 (the git-tracked, pushed board's mode)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_board_refresh.sh`
Expected: PASS — all assertions (existing #1–#10 plus the four new #11 assertions) print `ok - …`; the script prints `PASS` and exits 0.

- [ ] **Step 5: Update the contract doc**

In `scripts/board-refresh.md`, update the Behavior and Exit-codes prose so the guard is documented (the doc is the script's authoritative contract). Specifically:

1. In the atomic-write behavior description, change the single "renders … and replaces `BOARD.md` only after `render-board.sh` exits 0" condition to the **two-part** condition: replace `BOARD.md` only when `render-board.sh` **exits 0 AND** the rendered output is **non-empty**; otherwise leave `BOARD.md` byte-identical.
2. Add an exit-code / failure row for the empty-render case: exit `1` with stderr `board-refresh: render produced empty output; BOARD.md left untouched`, distinct from the propagated renderer failure (`render-board.sh failed (exit N)`) and the usage `exit 2`.

Read the file first to match its exact table/section wording; make the edits align with its existing style (do not restructure the doc).

- [ ] **Step 6: Run the full suite to confirm no regression**

The repo has no aggregate runner and no CI — the suite is the set of `tests/test_*.sh` files, run individually. Run the whole set and fail on any non-zero:

```bash
rc=0; for t in tests/test_*.sh; do echo "=== $t ==="; bash "$t" || rc=1; done; echo "SUITE rc=$rc"
```

Expected: every file prints `PASS` (or its own all-`ok` tail) and `SUITE rc=0`. Per LEARNINGS #54/#55, run the WHOLE set, not only the board tests — an out-of-goal regression is exactly what the tests outside this change exist to catch. (This is also what finalize's `local` gate re-runs post-rebase.)

- [ ] **Step 7: Commit**

```bash
git add scripts/board-refresh.sh scripts/board-refresh.md tests/test_board_refresh.sh
git commit -m "feat(0060): non-empty guard on board-refresh.sh atomic write"
```

---

## Self-Review

**1. Spec coverage:**
- Spec "Decision" (non-empty guard, exit-0-AND-non-empty, empty→exit non-zero + untouched + distinct message) → Task 1 Steps 1–4.
- Spec "Decision" (update `board-refresh.md` contract) → Task 1 Step 5.
- Spec "Testing" #1 (empty render leaves target intact, exits non-zero, stderr names empty output, no temp leak) → Task 1 Step 1 assertions.
- Spec "Testing" #2 (existing coverage stands as regression guard) → Task 1 Steps 4 & 6.
- Spec "Out of scope" (render-board.sh unchanged; no `--out`; no format validation) → honored; no task touches render-board.sh.
No gaps.

**2. Placeholder scan:** No TBD/TODO/"handle edge cases"/"similar to"/uncoded steps — every code step shows exact code. Step 6's runner name is the one soft spot (repo suite-runner name unverified); mitigated by naming the concrete fallback test files. ✅

**3. Type consistency:** Shell-only. Variable names (`$tmp_board`, `$RENDER_BOARD`, `$CHANGES_DIR`, `$SCRIPT`, `$work`, `$tmp`, `count_files`, `assert`) all match the existing `board-refresh.sh` / `test_board_refresh.sh` names verified against the current files. Exit code `1` and the `empty output` stderr substring are consistent between the implementation (Step 3), the test assertions (Step 1), and the contract (Step 5). ✅
