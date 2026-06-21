# Plan — drop over-escaped dashes in `test_docket_metadata_branch.sh` grep (change 0038)

## Context

`tests/test_docket_metadata_branch.sh:106` asserts `migrate-to-docket.sh` carries a `--yes`/`-y`
confirmation bypass with:

```sh
'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'
```

The `\-\-yes` over-escapes the leading dashes. Under **GNU grep** (Linux/CI) this emits
`grep: warning: stray \ before -` to stderr on every run, violating the project's
"a green run leaves stderr pristine" discipline (LEARNINGS #19/#22). BSD grep stays silent, which
is why it went unnoticed on macOS dev hosts; it was surfaced during change #34's regression sweep.

**Reconcile correction (see the change's `## Reconcile log`):** the obvious fix — just removing the
backslashes to give `grep -qE "--yes\b|\b-y\b"` — is **broken**. A pattern beginning with `--` is
parsed by grep as a command-line option, so it fails with `unrecognized option '--yes…'` (exit 2) on
GNU grep. The over-escaping was incidentally guarding against option-parsing. The correct fix uses
the explicit pattern flag `-e`, which is POSIX and tells grep the next argument is a pattern:

```sh
'grep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh'
```

Verified by hand: exit 0 with **empty stderr on both GNU grep (`ggrep`) and BSD grep**.

## Scope

- One file: `tests/test_docket_metadata_branch.sh`, line 106.
- One assertion's grep invocation. The assertion's meaning is unchanged (still matches `--yes` or `-y`).

## Out of scope

- Any other test-file stderr noise (audit separately if found — none found for this `\-` pattern;
  line 106 is the sole occurrence in `tests/`).
- Changing what the assertion verifies, or the migrate `--yes`/`-y` behavior it guards.

## Task 1 — fix the grep invocation, verify clean stderr under GNU grep

**Test-first (the failing observation):** Under GNU grep, the assertion line emits
`grep: warning: stray \ before -` to stderr. Capture this as the RED state: run the relevant grep
under GNU grep (`ggrep` on this host) with the *current* over-escaped pattern and confirm non-empty
stderr. (The suite assertion itself stays green either way — the bug is stderr noise, not a failed
assertion — so the verification target is **0-byte stderr under GNU grep**, not pass/fail.)

**Change:** In `tests/test_docket_metadata_branch.sh:106`, replace
`'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'`
with
`'grep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh'`.

**GREEN verification:**
1. `ggrep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh` → exit 0, empty stderr.
2. `grep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh` (BSD) → exit 0, empty stderr.
3. The assertion still finds the bypass (exit 0 ⇒ assertion passes).
4. Full run `bash tests/test_docket_metadata_branch.sh` under GNU grep leaves 0-byte stderr and all
   `ok -` lines (no `NOT OK`).

## Review / regression

- Re-run the full suite (`tests/test_docket_metadata_branch.sh` at minimum; spot-check the broader
  suite is unaffected since only one test file changed) to confirm no `NOT OK` and pristine stderr.
- Confirm no other `\-` over-escaped grep remains in `tests/`.
