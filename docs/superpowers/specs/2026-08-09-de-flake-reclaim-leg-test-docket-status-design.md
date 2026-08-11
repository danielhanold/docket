<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0268 — De-flake the reclaim leg of test_docket_status under parallel contention](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-11-0268-de-flake-the-reclaim-leg-of-test-docket-status-under-paralle.md)**
<!-- docket:backlink:end -->

# De-flake the reclaim leg of test_docket_status under parallel contention — design

**Change:** 0268 · **Date:** 2026-08-09 · **Author:** docket-auto-groom (autonomous)

## Problem

`tests/test_docket_status.sh`'s `reclaim(auto off)` leg intermittently fails one assert under
`scripts/run-tests.sh`'s parallel phase — `NOT OK - reclaim(auto off): prints the state-valid
remedy naming docket.sh reclaim-claims` — while passing standalone. First observed on change
0245's build gate, on a branch touching none of the involved files.

## Diagnosis (settled by code reading; confirm mechanically at build time)

The failing assert is written as a producer-pipe under the file's `set -uo pipefail` prologue
(tests/test_docket_status.sh, the `reclaim(auto off)` assert battery):

```sh
assert "reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims" \
  'printf "%s\n" "$(cat "$tmp/reclaim-off-out.txt")" | grep -qF "docket.sh reclaim-claims"'
```

`grep -q` exits on its first match; when the scheduler lets it exit before `printf` finishes
writing (likelier under high `-j` contention), `printf` takes SIGPIPE, the pipeline status
becomes 141 under `pipefail`, and `assert`'s `eval` reads a match as a failure. This is the
repo's promoted pipefail family — AGENTS.md's first Shell rule; learnings finding `pipefail`
(changes 11, 16, 46, 83, 108) — and this assert is the **only** instance of the
`printf "%s\n" "$(cat …)" | grep -q` shape remaining in `tests/` (verified by whole-repo grep,
one hit).

**Both suspects named in the stub are exonerated:**

- **Lease-TTL wall-clock comparison** — not on the tested path. The leg mocks
  `board-checks.sh` to emit fixed `[reclaimable]` findings; `reclaim_pass` in
  `scripts/docket-status.sh` only counts lines matching `RECLAIMABLE_LINE_RE` against the
  captured findings blob. No clock is read. The fixture's `claimed_at: 2026-01-01T00:00:00Z`
  is never compared to now by anything the leg executes.
- **Shared fixture path** — none. Every path in the leg lives under this file's own
  `tmp="$(mktemp -d)"` scratch dir (`$tmp/mock-reclaim`, `$tmp/reclaim-wire-case`,
  `$tmp/reclaim-off-out.txt`), unique per process. No other suite writes into it.

The four sibling asserts in the same leg use plain `grep -qF <pattern> <file>` and have never
flaked — consistent with the diagnosis: same fixture, same output file, no pipeline.

## Fix shape

1. **Rewrite the one assert** to the file's own no-pipeline idiom, matching its siblings:

   ```sh
   'grep -qF "docket.sh reclaim-claims" "$tmp/reclaim-off-out.txt"'
   ```

   The `printf "$(cat …)"` wrapper bought nothing: `grep` handles a file with or without a
   trailing newline identically, and the fixed-string pattern needs no normalization.

2. **Mutation-test the rewritten assert** (AGENTS.md guard rule): temporarily blank the remedy
   `printf` in `scripts/docket-status.sh`'s `reclaim_pass` else-branch, confirm the assert
   reddens, restore. This proves the rewrite still guards what it guarded.

3. **Confirm the diagnosis and demonstrate stability** at build time:
   - Reproduce the mechanism once in isolation if cheap (e.g. a tight loop of the old pipeline
     shape under load showing an occasional 141), else rely on the settled family precedent —
     the diagnosis is structural, not statistical.
   - Run `scripts/run-tests.sh` (the `finalize.test_command`) repeatedly — 10 consecutive full
     parallel runs at the suite's default/high `-j` — with `tests/test_docket_status.sh` green
     in all of them. Record the run count and results in the results file. A flake this rare
     cannot be proven absent, but the structural cause is removed and the stability run is the
     documented evidence bar.

## Out of scope

- Auditing other suites for the same shape (whole-repo grep already shows zero other hits of
  this pattern; broader hermeticity hardening is change 0252's territory).
- Any change to `scripts/reclaim-claims.sh` or `scripts/docket-status.sh` production behavior —
  the diagnosis proves the flake lives in the test's assert, not the code under test.
- `tests/lib/` fixture extraction (change 0252 owns that).

## Coupling

- **Change 0252** (harden test fixtures/hermeticity into tests-lib, groomed, high priority):
  no dependency — this fix does not need 0252's helpers, and the diagnosis found no fixture
  problem for 0252 to own. Recorded as `related:` only, as a file-collision caution: 0252's
  refactor may touch test files' fixture setup, and both changes edit under `tests/`. This
  change's edit is one assert line in `tests/test_docket_status.sh`; a rebase collision would
  be trivial either direction.

## Assumptions (audit trail)

1. **Root cause is the pipefail/SIGPIPE pipeline, not the stub's named suspects.**
   Chosen: settle on the pipeline diagnosis. Rejected: (a) lease-TTL wall-clock — the tested
   path reads no clock (board-checks mocked, `reclaim_pass` greps a captured string);
   (b) shared fixture path — all paths under a per-process `mktemp -d`. The failing assert is
   the unique pipeline-shaped assert in the file and matches a five-change precedent family
   with identical symptoms (intermittent, load-sensitive, passes standalone). Confidence is
   high; the build confirms mechanically (step 3) rather than by re-running until green.
2. **Fix is a test-side one-line rewrite, not a production change.** Chosen: rewrite the
   assert to `grep -qF … file`. Rejected: (a) trapping/ignoring SIGPIPE in the test harness —
   masks the class instead of removing the instance, and other asserts don't need it;
   (b) capturing into a variable then `grep <<<` — correct but heavier than needed when the
   output is already in a file the siblings grep directly.
3. **Stability bar is 10 consecutive green full parallel suite runs.** Chosen: 10 runs of the
   whole suite via `finalize.test_command`, since the flake only manifests under whole-suite
   contention. Rejected: (a) statistical repro of the old flake first — the failure is rare
   and machine-dependent; demanding a repro before fixing a structurally-proven defect blocks
   the fix on luck; (b) a synthetic contention harness — over-engineering for a one-line fix
   (and the tolerance-constant learning warns against calibrating machine-dependent numbers).
4. **No `depends_on`, `related: [252]` forward link only.** Chosen per the coupling analysis
   above; the reciprocal link in 0252 is deliberately not written (forward link only).
5. **`## Reason for deferral` stays historical.** The stub's deferral rationale (keep off
   0245's branch) is already satisfied by this being its own change; no action needed.
