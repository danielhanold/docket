<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0219 — aborted-run's Step 7 seam — a fourth git-only leg, plus GitHub enrichment for leg C](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0219-aborted-run-s-sixth-signature-pr-opened-and-pr-written-run-d.md)**
<!-- docket:backlink:end -->

# aborted-run's fourth leg and its GitHub enrichment

## Context

`aborted-run` is `board-checks.sh`'s external, mechanical oracle for an autonomous
`docket-implement-next` run that stopped mid-step. It exists because an agent that dropped a
bookkeeping write is the least reliable narrator of having dropped it — every observed incident
produced a confident, specific, wrong completion report.

It has three legs today:

- **Leg A** (change 0113) — manifest/git incoherence, time-free: an artifact file committed on the
  feature branch while the field that should record it is empty.
- **Leg B** (change 0113) — a claim stamped more than `ABORTED_RUN_STALE_SECS` (12h) ago.
- **Leg C** (change 0211) — built-but-not-delivered: the branch carries commits ahead of the
  integration branch, `pr:` is empty, and the branch has been quiet longer than
  `ABORTED_RUN_IDLE_SECS` (2h).

This change closes the remaining gap around the **Step 7 seam** — the last step, where the run
pushes the branch, opens the PR, and records `status: implemented` + `pr:` together.

Two distinct states live at that seam, and the stub's original text conflated them. They are
separated here.

### State 1 — `pr:` recorded, `status:` never advanced

`docket-implement-next` (SKILL.md:96) writes `status: implemented` **and** `pr:` in a single
field-write, and no script under `scripts/` writes `pr:`. So a manifest showing `pr:` set while
`status:` is still `in-progress` is an anomaly by construction. No leg sees it:

- Leg A finds no incoherence — `plan:` and `results:` are both recorded by then.
- Leg C **short-circuits on a non-empty `pr:`** by deliberate design (`board-checks.sh`: "A
  non-empty `pr:` short-circuits the WHOLE leg"), so it exits with zero git calls.
- Leg B catches it at 12h — the same lag 0211 exists to close.

This state is manifest-internal and detectable git-only.

### State 2 — PR open on GitHub, `pr:` never written

This is the state change 0194's second stop actually produced, and 0211 records it as "the PR open
and the **manifest unwritten**."

**Leg C already detects it.** Its pushed arm emits:

> `<branch> is pushed but pr: is unset (last commit Nh ago) — a run may have stopped between its
> push and its PR record; verify the PR exists`

So this state is not undetected. What leg C cannot do is **resolve** it: `board-checks.sh` is
git-only by contract and shells no `gh`, so it can only tell a human to go look. Two very different
situations produce that one finding — a PR that exists and merely went unrecorded (remedy: record
it) versus a run that died before creating one (remedy: create it) — and today only a manual check
distinguishes them.

The honest framing: this change does not add detection for state 2. It resolves the ambiguity leg C
leaves behind.

## Decision

Two independent pieces, in two files.

### Leg D — `board-checks.sh`, git-only, contract untouched

A fourth `aborted-run` leg, scoped like the others to `status: in-progress`:

```
pr: is NON-empty  →  emit
```

**Time-free, with no idle floor.** This is a deliberate departure from legs B and C, and the
reasoning belongs in the leg's comment: the two fields are written in one stroke, so any
disagreement between them is already an anomaly rather than a run in flight. There is no healthy
window to wait out. Leg A is the precedent — it is time-free for the same reason.

**Cost.** Leg C's gate already reads `pr:`. Hoist one `ar_pr="$(fm_field "$f" pr)"` above leg C and
have both legs test the variable. `board-checks.sh` documents this path as cost-sensitive (change
0176), and a second read of the same field there is a real regression.

Message shape, naming the missing write and its remedy:

> `pr: records <pr> but status: is still in-progress — the run stopped before its final status
> write; verify the PR and set status: implemented`

**Known residual, stated in the leg's comment.** `board-checks.sh` reads change files off the
filesystem, not out of a git blob. Combined with the single-stroke field-write, leg D's honest yield
is *uncommitted partial edits in the shared `.docket` worktree, plus non-compliant drivers* — not
a routine abort signature. No idle floor is constructible for an uncommitted edit, so no floor is
correct here — but for that reason, not for the reason a first draft reaches for.

### GitHub leg — `docket-status.sh`, beside `detect_merged`

`detect_merged` (`docket-status.sh`:485) already runs a batched `gh` sweep over `active/`, and
already carries a "per-change `gh pr list` fallback only for changes with no `pr:` set" — the exact
query needed. The new leg sits beside it and reuses its posture verbatim.

Gate — the same one leg C uses, so the two findings always agree:

```
status: in-progress
AND pr: is empty
AND branch tip older than ABORTED_RUN_IDLE_SECS (2h)
→ ask GitHub for an open PR on feat/<slug>
```

The 2h floor is reused rather than re-tuned. Legs B and C both hardcode their horizons with no
config knob (`board-checks.sh`:167, :174), and that precedent holds: a second magic number would
need its own justification, and using leg C's own floor guarantees the enrichment never fires on a
change leg C stayed silent about.

Two outcomes, two remedies:

- open PR found → `PR #<n> is open but pr: is unset — record it`
- no PR found → `branch pushed, no PR on GitHub — the run stopped before opening one`

**Offline / rate-limited posture — identical to `detect_merged`.** Any gh, network, or parse failure
emits `sweep-skipped <reason>` and returns 0. It never aborts the pass. This is what keeps
`board-checks.sh`'s offline guarantee intact: the offline-safe check keeps working and keeps
emitting leg C's finding; only the enrichment goes quiet.

**Advisory, like every `aborted-run` leg.** It flips no status, releases no claim, and writes no
file.

## Consequences

### What this changes

- `scripts/board-checks.sh` — leg D, plus the hoisted `ar_pr` read shared with leg C.
- `scripts/docket-status.sh` — the GitHub enrichment leg beside `detect_merged`.
- `scripts/board-checks.md` — the `## Not covered` paragraph is **rewritten, never deleted**. It is
  the only written record of the gap, and the surviving residual is now "offline, or `gh`
  unavailable" rather than "manifest/GitHub comparison is out of contract." Line 271, which defers
  this case citing the git-only contract, must now point at `docket-status.sh`.
- `scripts/docket-status.md` — the new leg's contract.
- The `aborted-run` preamble comment in `scripts/board-checks.sh` still reads "Two INDEPENDENT
  legs; either emits, and both can emit on one change" — stale since 0211 made it three. Fix both
  halves in passing, to the "any emits / more than one may emit" phrasing `board-checks.md` already
  uses.

### What it gives up

- Leg D's reachable population is narrow. It is worth building as a cheap, additive completeness
  guarantee over the Step 7 seam, not because it is a frequent signature.
- The GitHub leg introduces docket's second `gh` dependency on the status path. It is bounded by
  `detect_merged`'s existing best-effort posture, so the failure mode is a quieter check, never a
  broken one.

### Testing

- Leg D: fixture with `status: in-progress` + `pr:` set → emits; `pr:` set + `status: implemented`
  → silent; `pr:` empty → silent (leg C's domain).
- Shared `ar_pr` hoist: leg C's existing behaviour must be unchanged — its fixtures stay green
  byte-for-byte.
- GitHub leg: stub `gh`; assert both outcomes, and assert that a failing/absent `gh` produces
  `sweep-skipped` and exit 0.
- Do **not** hardcode fixture ids. `tests/test_board_checks.sh` was at a max of 248 and is moving.

### Coupling

`related: [200, 222]` — both edit `scripts/board-checks.sh` and its test suite concurrently.
Neither gates this work; expect a rebase compose.
