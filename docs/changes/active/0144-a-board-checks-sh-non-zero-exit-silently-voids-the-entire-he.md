---
id: 144
slug: a-board-checks-sh-non-zero-exit-silently-voids-the-entire-he
title: A board-checks.sh non-zero exit silently voids the entire health pass
status: proposed
priority: medium
type: chore
created: 2026-07-27
updated: 2026-07-28
depends_on: [117]
related: [145]
discovered_from: [117]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`docket-status.sh`'s `health_checks()` pipes `board-checks.sh` into a `while read` loop, so a
non-zero exit from the checker produces ZERO `check` lines and the health pass reports a clean
tree. Change 0117's final review found a live instance of this (a missing `adrs_dir` made
`board-checks.sh` exit 2 and silently dropped every health check) and fixed the trigger — but the
*swallowing* remains, and the regression test written for it cannot see the failure it was written
about: the mock `board-checks.sh` exits 0 regardless of its arguments, so the "still emits check
lines" assert passes against both the fixed and the unfixed code.

The general shape is worth closing deliberately: any future condition that makes `board-checks.sh`
exit non-zero becomes an invisible loss of the entire health pass, and the existing test scaffolding
structurally cannot detect it.

## What changes

Add a `board-checks.sh` mock that exits 2 (and one that exits 1) and assert what `health_checks`
does with it — at minimum that the failure is SURFACED rather than read as a clean tree. Decide
whether the current best-effort posture ("a board-checks failure produces no extra output and never
aborts the pass", per `scripts/docket-status.md`) is still the right contract now that a whole-pass
loss can hide behind it, or whether the pass should emit a distinguishable diagnostic line.

## Out of scope

- Re-litigating `board-checks.sh`'s own exit-2-on-bad-argument rule, which is correct for a hand-run
  caller.
- Change 0117's specific trigger, already fixed.

## Auto-groom blocked

**2026-07-28** — autonomous grooming abstained. A full design was drafted and survived two critic
passes on substance; what it could not settle is **how this change should be sequenced against
change 0117's open PR #129**, which is a backlog-composition call with a cost the drain must not
impose unilaterally.

### The undecidable decision

0117 is `status: implemented`, `pr: #129`, **built but not merged**, and its branch touches the two
things 0144 edits:

- `health_checks()` — 0117 inserts an `adr_args=()` block above the `board-checks.sh` invocation and
  appends `${adr_args[@]+"${adr_args[@]}"}` to the command line. The edit is **additive**: the pipe
  into `while IFS=$'\t' read`, the loop body, and `return 0` are byte-identical to `main`. The single
  point of contact with 0144 is one line of the invocation.
- `tests/test_docket_status.sh` — ~64 added lines in the health-check area, where 0144's new cases go.

(0117 also appends seven sentences to `scripts/docket-status.md` §7 and adds a 13th check-id,
`adr-unpublished`, to `BOARD_CHECK_IDS` and the four pinned surfaces. Neither collides: 0144 edits a
§7 sentence 0117 leaves as unchanged context, and touches no check-id surface.)

Two defensible sequencings, with different costs:

1. **`depends_on: [117]`** — build after #129 merges. Clean, but `depends_on` gates build-readiness,
   so it parks 0144 behind a **human merge** and the board will read *waiting on #117 — needs your
   merge*. The drain should not park work behind a human gate on its own judgment.
2. **Build in parallel, reconcile at rebase** — supported by the repo's
   `concurrent-edits-compose-at-rebase` finding, and genuinely applicable here since 0117's edit is
   additive and one line wide.

`depends_on: [117]` is recorded as the **conservative placeholder**; flip it to `related:` if option 2
is preferred. `related: [145]` is recorded as subject overlap only — verified **not** a file collision
(0145 targets `skills/docket-status/SKILL.md`, and its own out-of-scope excludes the four pinned
surfaces including `scripts/docket-status.md`; its alternative shape lands in
`tests/test_board_checks.sh`).

### What a human should supply

- The sequencing ruling above.
- Confirmation that a new **non-`check`** report line is an acceptable addition to
  `docket-status.sh`'s output contract (the design assumes yes; it is a public-ish surface).

### The settled design, ready to re-use on re-arm

Everything below survived critic verification against the running code.

**Root cause.** `health_checks()` pipes `board-checks.sh` into a `while read` loop and never reads the
producer's exit status. `board-checks.sh` accumulates into `$FINDINGS` and prints once at the end, so
every validation failure (`exit 2`) emits zero TSV — the loop body never runs, `health_checks` returns
0, and the pass is indistinguishable from a clean tree. The existing regression mock exits 0
unconditionally, so it cannot see the failure it was written about (`green-suite-untested-branch`).

**Posture: keep warn-only, make it loud.** Not fatal — a checker bug must not abort a pass whose real
work (board, sweep, digest) succeeded. Confirmed shipped as an invariant in `scripts/docket-status.md`
("health checks warn-only").

**Fix: capture-then-loop.**

```
local out rc=0 check_id change_id message
out="$("$DOCKET_BASH_PATH" "$SCRIPTS_DIR"/board-checks.sh --changes-dir "$cd_dir" …)" || rc=$?
while IFS=$'\t' read -r check_id change_id message; do
  [ -n "$check_id" ] || continue
  echo "check $check_id $change_id $message"
done <<<"$out"
[ "$rc" -eq 0 ] || printf 'board-checks failed %s\n' "$rc"
return 0
```

Verified: the script runs `set -uo pipefail` **without `-e`**, so `|| rc=$?` is safe. An empty `$out`
yields one blank line from `<<<`, already swallowed by the existing guard. The loop no longer runs in
a pipeline subshell, so the three read variables must be declared `local`. Capture-then-consume is the
file's own idiom — `reclaim_pass` captures then greps a here-string, citing change 0067's
no-pipefail-SIGPIPE rule.

**New report line `board-checks failed <exit>`** — deliberately NOT a `check` line: the
`check <check-id>` vocabulary is closed (`BOARD_CHECK_IDS`) and pinned across four surfaces by change
0111's guard, and "the checker itself broke" is not a finding. The name follows the shipped
`board inline failed` / `learnings index failed` convention. Safe against both gates, verified:
`reclaim_pass`'s `RECLAIMABLE_LINE_RE` anchors `^check stale-in-progress`, and `board_classify` matches
`case "$line" in "board "*)` — a hyphen where the glob needs a space. The contract row should still say
explicitly that this is a **health-pass** line, since `skills/docket-status/SKILL.md` teaches readers
that the `board …` family means the board step failed.

**Not retryable / `board_classify` untouched** — and the reason is **capture scope**, not flag
exclusivity: `board_pass_must_land` classifies only its own `board_out="$(board_pass)"`, captured
before `health_checks` runs. (A bare `--must-land` DOES run the full path — the arg parser excludes
`--must-land` only against `--digest-only`.)

**Stale prose to retire — three sites, not two.** The claim "a clean tree, or a `board-checks.sh`
failure, produces no extra output and never aborts the pass" lives at `scripts/docket-status.md:218`,
in the **§7 "Health checks"** narrative — **not** in *Failure postures*, whose health-check bullet
(~line 404) carries no stale claim. It is duplicated a third time **in code**, at
`scripts/docket-status.sh:694`. Editing only *Failure postures* would miss entirely. Also add the row
to the *Output contract* table (~line 335).

**Tests** (`tests/test_docket_status.sh`), with the mutation mandate correctly scoped:

*Must fail against the unfixed `health_checks`:*
- mock exits 2, no output → exactly one `board-checks failed 2`; run still reaches `pass ok`, exit 0.
- mock exits 1, no output → `board-checks failed 1`.
- mock prints one well-formed TSV finding **then** exits 2 → the `check` line AND `board-checks failed 2`.
  Pins "additive, not replacement"; a naive `if rc; then diagnostic; else findings; fi` fails it. (This
  shape is forward-looking — `board-checks.sh` cannot currently emit-then-fail on this path — but it
  keeps a later streaming version from silently losing findings.)

*Non-regression pins, expected to pass both ways — do NOT apply the mutation mandate to these:*
- mock exits 0 with findings → unchanged output, no `board-checks failed` line (no-false-positive
  direction; `correspondence-guard-runs-one-way`).
- mock emits `stale-in-progress … [reclaimable]` and exits 2 → the reclaim remedy line still prints.

*Two further guards the critic asked for:*
- A guard for the capture-scope argument, which today rests on call ordering inside `main()` that a
  future edit could silently break: full path with `--must-land`, board-checks mock exits 2, board mock
  clean → run still exits 0 and reaches `pass ok`.
- A doc guard that the **stale sentence is gone**, not merely that the new line is documented. The
  existing `assert "status contract documents …"` precedent (~lines 1934-1950) pins presence only.

**No ADR** — a failure-posture refinement inside one function, expressed in the existing output
contract.

### Recommendation

Keep and build. The design is complete; only the sequencing needs a human word.
