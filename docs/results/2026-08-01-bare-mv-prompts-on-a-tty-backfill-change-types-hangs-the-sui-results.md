<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0186 — Bare mv prompts on a tty — backfill-change-types hangs the suite and can exit 0 without installing](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0186-bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui.md)**
<!-- docket:backlink:end -->

# Bare `mv` prompts on a tty — results

Change: #0186 · Branch: feat/bare-mv-prompts-on-a-tty-backfill-change-types-hangs-the-sui · PR: (opened at close-out) · Plan: docs/superpowers/plans/2026-08-01-bare-mv-prompts-on-a-tty.md · ADRs: none

## Verify (human)

The automated suite **cannot** verify the thing this change exists to fix: every agent shell and the
finalize gate run without a tty, which is exactly the environment in which the bug was invisible.
The first check below is the merge gate for this change.

- [ ] **Run the suite from a real interactive terminal and confirm it finishes.**
      `bash tests/test_backfill_change_types.sh` — before this change it blocked forever on
      `override rw-r--r-- … for 0002-b.md? (y/n [n])`. It should now complete in a few seconds.
- [ ] **Confirm the pty guard actually ran on your machine** — no `skip -` line in that output.
      A skip means the guard proved nothing on your host; the skip line names the resolved
      `script(1)` flavor so you can tell which of the two causes fired.
- [ ] **Optional, if you want to see the bug** — revert the fix in a scratch copy
      (`mv -f` → `mv`) and re-run: the pty block goes red while the non-pty rollback block stays
      green. That contrast is the whole argument for the change.

## Findings

- **The suite was unfinishable by hand since 2026-07-23 (change 0127) and nothing noticed**, because
  the only environments that ran it — agent shells, the finalize gate — have no tty. Green tests
  were never evidence the rollback path had been exercised honestly.
- **The hang was concealing a worse bug.** Under a pty with stdin at EOF, bare `mv` declines the
  overwrite and exits **0**: the staged file is never installed, `if ! mv` never fires, no rollback
  runs, and the script reports success with a half-migrated backlog — the precise outcome the
  install's undo exists to make impossible. Verified by probe, and now pinned by the pty guard.
- **`grep` on the development machine is ugrep, and it silently corrupted the mutation evidence.**
  ugrep reads the mid-pattern `$` in `grep -c 'mv -f "$out"'` as an end-of-line anchor and returns
  **0 even on the fixed file**. The plan's literal verification command was therefore wrong, and a
  mutation test relying on it would have shown a false `0`-before-and-`0`-after — reading exactly
  like a robust guard while proving nothing. Every count in this build used `/usr/bin/grep`.
  (This is the standing `grep-is-ugrep-masks-bsd-portability-bugs` hazard, hit again in a new place:
  not a portability bug this time, but a falsified *guard verification*.)
- **The `cp -p` twins were audited and deliberately left alone** — `cp` prompts only under `-i`, and
  `-f` on the rollback-restore path would unlink the very destination the undo exists to preserve.
- **No ADR was warranted.** Every non-obvious decision — `mv -f` over redirecting stdin or
  `cp && rm`, both guard layers rather than one, probing `script(1)` for exit-status fidelity rather
  than sniffing `uname` — was settled at grooming and is recorded in the spec's A1–A8. The build
  introduced no new architectural decision.

## Follow-ups

Both minted as stubs during this build (auto-capture), both `type: fix`, both
`discovered_from: [186]`:

- **#0188 — `mktemp -d` with no template ignores `TMPDIR` on macOS.** `backfill-change-types.sh`'s
  staging dir escapes the test fixture, so the `TMPDIR=` redirect both rollback fixtures pass is a
  no-op on this platform, and `cp -p` preserving the `uchg` flag leaves an undeletable directory per
  run. **Pre-existing** (+1 per run on `origin/main`); this branch's second `uchg` fixture doubles
  it to +2 by the identical mechanism. Not fixed here — it is a distinct defect on a different line
  and deserves its own coverage. ~11,000 such dirs had accumulated locally; 238 were swept by
  signature during the build.
- **#0189 — sweep the 15 remaining bare-`mv` install sites.** The spec deferred a repo-wide sweep
  pending evidence; review produced it. The other 15 sites' `|| die` guards are **unreachable** for
  this failure mode, because the silent-decline path exits 0 — so the tool reports success having
  written nothing. Several are worse targets than the one fixed here: `ensure-docket-env.sh`,
  `ensure-claude-settings.sh`, and `ensure-global-config.sh` write user-facing config on the
  **interactive install path**, where a human demonstrably has a terminal attached.

**Plan deviation.** Task 2's Step 5 asserted the leaked-temp-dir count "does not grow across two
consecutive runs." That was **false** and is corrected in the committed plan with the measured
numbers and the root cause, so the next reader does not inherit a bogus verification.

**Learnings recommendation** (the close-out harvest is the sole writer and decides). The spec left
this a three-way choice; the review's 15-site evidence resolves it toward **`shell-portability`**,
which is already `promoted` and whose rule this extends cleanly. Suggested rule:

> A non-interactive flag on a tool that can prompt is load-bearing, not style. An unwritable
> destination turns a bare `mv` into either an infinite hang or a **zero-exit no-op** — and a
> trailing `|| die` cannot catch the second one. Corollary for guards: forcing a failure via a
> filesystem flag is sound only if the tool under test does not prompt.

A secondary candidate is `green-suite-untested-branch` (the rollback assertions were honest only
without a tty). Minting a fourth finding file stays the last resort.
