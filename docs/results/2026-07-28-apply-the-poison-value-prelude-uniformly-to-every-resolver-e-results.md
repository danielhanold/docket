<!-- results-template.md — close-out artifact for a change. -->
# Apply the poison-value prelude uniformly to every resolver eval in the config suite — results
Change: #126 · Branch: feat/apply-the-poison-value-prelude-uniformly-to-every-resolver-e · PR: (not yet opened) · Plan: docs/superpowers/plans/2026-07-28-poison-prelude-uniformity.md · ADRs: none

## What was proven

### 1. The hazard, demonstrated on the unmodified file

The plan's completion bar was to *prove* the stale-value hazard rather than assert it, at the
natural O→P `AUTO_GROOM` coincidence (`tests/test_docket_config.sh:509–520` in the spec's frozen
line numbers; `:511`/`:520` after the file grew under 0127) — not at the stub's originally named
site (`:501`, the `L2`/`BOARD_SURFACES` fixture). At `:501` the previous fixture leaves
`BOARD_SURFACES=none`, so an aborting run there makes the assert redden, not pass vacuously; the
hazard is real there but latent, not demonstrable. `:501` still received a prelude in Task 3 like
every other violating site — it was just not usable for the demonstration itself.

Three states, same assert (`0050 P: built-ins fallback (auto_groom)`, originally `:520`):

| State | rc | ok | notok | `0050 P: built-ins fallback (auto_groom)` |
|---|---|---|---|---|
| Clean baseline | 0 | 375 | 0 | `ok - 0050 P: built-ins fallback (auto_groom)` |
| Sabotage only (P's resolver made to abort) | 1 | 373 | 2 | `ok - 0050 P: built-ins fallback (auto_groom)` — **vacuous pass**: P's `eval` received an empty string, and the assert silently read block O's stale `AUTO_GROOM=false` |
| Sabotage + `AUTO_GROOM=__poison__` prelude | 1 | 372 | 3 | `NOT OK - 0050 P: built-ins fallback (auto_groom)` — **caught** |

The 375→373→372 / 0→2→3 deltas must **not** be read as "only this one assert's status changed."
Under sabotage, two sibling asserts in the same block legitimately redden for unrelated, correct
reasons — `NOT OK - 0050 P: malformed global warned` (was `:519`) and `NOT OK - 0050 P: malformed
global not fatal (exit 0)` (was `:521`) — because the resolver genuinely failed to warn and to
exit 0. The demonstration is specifically about the third assert, `:520`, whose own status is
`ok` (vacuous) under sabotage-only and flips to `NOT OK` only once the prelude is added, with the
sabotage held constant. All mutations were reverted; working tree confirmed clean before Task 2
began.

### 2. The derived population — 64 sites, never hand-written

Two independent prior reviews of this file had hand-counted 64 and 65 eval sites — the
disagreement is why the spec forbade hardcoding a count. The guard derives the site count at
suite runtime from a shape (`eval "$V"` where `V` was assigned from a command substitution,
comment lines and the `assert()` helper's own `eval "$2"` excluded), then cross-checks that
number against a **structurally different** extractor (a plain grep on the literal `eval "$`,
netted against comment-line occurrences and the guard's own literal-defining line) rather than
a second pass of the same tokenizer, and asserts a `>= 60` floor underneath both. Measured:
`sites=64`, agreeing with the independent extractor, comfortably above the floor. A corpus-
truncation probe (appending a throwaway compliant site) raised the count to 65, confirming the
corpus is the whole file, not truncated at an end-of-file marker.

### 3. The split and final state

- **3 sites exempt by derivation** — no exported variable is read between that eval and the next.
- **21 sites already compliant** (9 of them the existing `unset`-idiom blocks, left byte-untouched).
- **40 sites needed a new poison prelude** — added one line each, in a single bottom-up sweep
  (`git diff --stat`: 40 insertions, no other changes).

Final: `TOTALS sites=64 exempt=3 ok=61 viol=0`. Full suite: `381 ok / 0 NOT OK`, `PASS` — reproduced
from a different cwd as well (see I4 below), confirming the guard doesn't secretly depend on
where it's invoked from.

### 4. One deliberate departure from the spec, recorded rather than silent

The spec's design section phrases the clearing window narrowly: "the same line or the preceding
non-blank line." The guard as built instead treats the clearing window as **everything since the
previous eval site**. Measured against the real file, the narrow rule flags all 9 pre-existing
`unset`-idiom sites as violations (their `unset VAR` lines sit earlier in the block, not on the
immediately preceding line), which would force edits to blocks that the spec's own assumption 2
explicitly forbids touching ("existing `unset` blocks stay byte-untouched"). The wide window is
also the more faithful model of the actual hazard: the stale value that can leak into an assert is
whatever the *previous fixture's* eval left standing, not merely whatever sits one line above.

### 5. The mutation matrix (Task 4) — four cells, all reddened as required

| Cell | What it does | What catches it |
|---|---|---|
| A — delete a prelude | Removes a `VAR=__poison__` line entirely | Correspondence assert, `viol=1` |
| **B — clear the wrong variable** | Prelude present, but poisons a variable the site's asserts never read | Correspondence assert, `viol=1`, naming the actual uncleared variable |
| C — unprotected fixture appended at file tail | New eval+assert block with no prelude, appended past the guard | Correspondence assert, `sites=65 viol=1` (also proves the corpus isn't truncated) |
| D — break the corpus bound | Shrinks the self-block marker span from 7 lines to 2, exposing the guard's own literals to the scan | The self-block-bounded assert (not, as the plan's example named, either count assert) |

**Mutation B is the load-bearing cell.** It is the one property a presence-only guard (checking
only "is there some `__poison__` line here") cannot see: a prelude line is physically present, it
just clears the wrong name. A presence-only guard would be green here. This guard names the exact
missing variable (`BOARD_SURFACES`) and fails. That is the difference this change exists to build.

Mutation D reddened via the self-block-bounded structural assert rather than either of the two
count asserts the plan's text named as the expected trip point — the requirement itself (a
mis-bound corpus is caught loudly, never silently reported as a smaller-but-plausible population)
was still met, just by an adjacent structural assert built for exactly that purpose. Recorded as
a deviation from the plan's literal wording, not a gap.

### 6. Whether the guard shipped at all

It did. The spec's fallback — ship **no** guard, with the reason recorded, if correspondence
proved infeasible at build time — was never triggered, because correspondence was reachable. The
Mutation B result above is the direct evidence: the guard implements true correspondence, not
presence.

## Review history — read this section

The whole-branch review found **four Important vacuity holes in the guard itself**, on top of one
already closed by an earlier per-task review. All five were closed by review, none by the suite
noticing on its own. That is the honest framing and the change's main lesson: a guard written
specifically to kill vacuous greens shipped its first draft carrying five of them.

- **Per-task review, closed in commit `a4682cfd`:** an empty derived key set made every site
  "exempt by derivation," all four original asserts green, zero violations reported. Closed with a
  runtime-derived floor (`>= 20 keys`, 28 measured today).
- **I1 — a WRONG key set (not merely empty)** made every site exempt too, and the keycount floor
  didn't catch it because a wholesale rename preserves the count. Closed with a ceiling:
  `exempt <= 5` against a measured 3. Proven with a scratch copy that renamed every derived key
  (`sed 's/^/X/'`) — reproduced `exempt=64`, and the new ceiling assert alone caught it.
- **I2 — the clearing regex blessed `VAR=""` as a clear.** In front of a `[ -z "$VAR" ]` assert,
  an empty-string clear is indistinguishable from the asserted value itself — a clear that can't
  fail. Closed by narrowing the regex to `__poison__` only. This surfaced two real, previously
  invisible violations (`AUTO_GROOM` at one site, `DOCKET_BASH_PATH` at another), each fixed on its
  own merits — no exemption list was added to route around them.
- **I3 — violations were computed but never printed.** A red run showed no line number and no
  variable name, only a bare count. Closed by widening the guard's own print filter to also emit
  `SITE ... viol ...` records.
- **I4 — key derivation was non-hermetic and cwd-dependent** (it shelled out without
  `--repo-dir`, coupling the guard to this repo's own committed `.docket.yml`). Running the suite
  from a different working directory reddened the guard. Closed by routing the derivation through
  the file's own fixture-repo builder, matching every other fixture's pattern. Verified green from
  `/tmp` after the fix.

All four Important findings were fixed in one wave, commit `406cc3c3`, and confirmed clean by a
scoped re-review ("ship it"). End state: `TOTALS sites=64 exempt=3 ok=61 viol=0`, `381 ok / 0 NOT
OK`, `PASS`, reproduced from a non-repo cwd.

## Parked / follow-ups for the merge gate

- **The exempt ceiling (`-le 5`) is a fixed number against today's `exempt=3`.** A proportional
  bound, or a floor on `ok` instead, would be more drift-proof as the file grows. Its failure mode
  when it ages is a loud false red (a legitimate future exemption trips the ceiling), never a
  silent pass, so this is parked as a suggestion rather than fixed now.
- **A partial key rename** (e.g. 5 of 28 keys renamed, not all of them) would raise `exempt` only
  slightly and could slip under the current ceiling. Only wholesale key-set degeneracy is
  guaranteed to be caught; a small, partial corruption is a real remaining gap in the ceiling's
  coverage.
- **Pre-existing, out of scope — worth its own follow-up change:** `tests/test_docket_config.sh`
  contains asserts of the form `[ -z "$DOCKET_BASH_PATH" ]` (near the 0132 runtime-resolution
  section) inside blocks that contain **no eval at all**. These can never fail regardless of what
  the variable actually holds — a correspondence guard has no reach into fixtures that never eval
  anything, since its whole mechanism is keyed on eval sites. Not touched here; a distinct defect
  class from the one this change addresses.
- **The guard's need-window and cleared-window are asymmetric** (need-window runs eval-to-next-eval;
  cleared-window runs previous-eval-to-this-eval), so a prelude can be load-bearing for a site
  hundreds of lines away. Concretely: the last detected eval site before the guard's own self-block
  clears variables that a much later, unrelated section's asserts actually depend on. Deleting an
  apparently irrelevant poison line near that site can redden a distant fixture with no visible
  connection in a local diff. This is documented in-file, at the one site where it currently bites
  (the `DOCKET_BASH_PATH=__poison__` line near the require_pr_approval fixture), rather than fixed
  — fixing the window asymmetry is a larger change to the guard's shape than this task's scope.

## Verify (human)

- [ ] Confirm the exempt ceiling (`-le 5`) and the partial-rename gap above are acceptable residual
      risk, or file a follow-up change for a proportional bound.
- [ ] Confirm the pre-existing `DOCKET_BASH_PATH` vacuity at the 0132 section (outside this guard's
      reach) is worth its own follow-up stub.
