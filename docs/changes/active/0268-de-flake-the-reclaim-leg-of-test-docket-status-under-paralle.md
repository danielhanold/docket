---
id: 268
slug: de-flake-the-reclaim-leg-of-test-docket-status-under-paralle
title: 'De-flake the reclaim leg of test_docket_status under parallel contention'
status: in-progress
priority: medium
type: fix
created: 2026-08-08
updated: 2026-08-11
depends_on: []
related: [252]
discovered_from: [245]
adrs: []
spec: docs/superpowers/specs/2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/de-flake-the-reclaim-leg-of-test-docket-status-under-paralle
claimed_at: 2026-08-11T20:38:15Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-de-flake-reclaim-leg-test-docket-status-design.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced while running change 0245's full-suite build gate. The first gate run came back `EXIT=1` on a single assert in `tests/test_docket_status.sh`: `NOT OK - reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims`. Run standalone the suite passes; re-run in parallel it passed. Change 0245 touches none of `tests/test_docket_status.sh`, `scripts/docket-status.sh`, or `scripts/reclaim-claims.sh`, so this is a pre-existing flake under parallel contention, not a regression.

**Diagnosis (settled at groom, 2026-08-09; details in the linked spec)** — the flake is neither of the originally suspected causes. The failing assert is the only one in the leg — and the only remaining instance in `tests/` — written as a `printf "%s\n" "$(cat …)" | grep -qF …` pipeline under the file's `set -uo pipefail` prologue: `grep -q` exits on first match, the producer takes SIGPIPE under load, and the 141 becomes an intermittent NOT OK. This is the repo's promoted pipefail family (AGENTS.md Shell rule; learnings `pipefail`). Both stub suspects are exonerated: the tested path reads no clock (`board-checks.sh` is mocked; `reclaim_pass` greps a captured blob), and every fixture path lives under the file's own per-process `mktemp -d`. A flaky assert in the suite that gates every merge is worse than a missing one: it trains readers to re-run until green, which is exactly how a real red gets waved through.

**Independent value** — stands entirely with change 0245 reverted; the flake predates that branch and will keep firing on unrelated branches until the pipeline assert is rewritten.

**Boundary** — rewrite the one assert to the leg's sibling idiom (`grep -qF "docket.sh reclaim-claims" "$tmp/reclaim-off-out.txt"`), mutation-test the rewritten guard, and demonstrate stability with 10 consecutive green full parallel `scripts/run-tests.sh` runs. It stops there: no production-code change (the diagnosis proves the flake lives in the test's assert), no re-audit of other suites (a whole-repo grep already shows zero other hits of the shape), no `tests/lib/` fixture work — change 0252 owns that; recorded as `related:` only, a file-collision caution with no dependency either way.

**Reason for deferral** — change 0245's branch is scoped to `sync-agents.sh` wrapper generation and its own new suites. Debugging an unrelated suite's parallel-contention flake would expand that branch into a second, independently reviewable concern, and its fix belongs with someone reading the reclaim leg's own design rather than riding a refactor branch.

## Carry-forward from #0247 (2026-08-11)

Change 0247 landed on this surface and spent its budget headroom. Before adding to
`scripts/docket-status.sh`, `tests/test_docket_status.sh`, or `skills/docket-status/SKILL.md`, read
these two numbers as measured at 0247's close-out:

- `tests/test_docket_status.sh` — roughly **3s of margin** against its 45s row in
  `tests/runtime-budgets.tsv`. This change's own acceptance bar is **10 consecutive green full
  parallel runs**, which is exactly the contended measurement the margin is compared against.
- `skills/docket-status/SKILL.md` — **22 words** of headroom against its size budget.

The next edit to either trips a budget. The remedy is already settled and should not be re-derived:
apply **change 0137's rounding rule** (next multiple of 5 plus a 5s margin, computed from the worst
*standalone serial* reading, never the contended run-of-the-day number) and carry **change 0201's
in-diff argument** for the word budget. **#0118 is queued against the same surface**, so whichever of
the two lands second inherits whatever margin the first leaves. See the learnings finding
`budget-headroom-is-spent-before-it-is-breached`.

## Carry-forward from #0118 (2026-08-11)

#0118 landed second on this surface and consumed the rest of the margin the #0247 note above
anticipated. **The situation has changed in kind, not just in degree: `tests/test_docket_status.sh`
is now at the runtime table's HARD 60s ceiling, and there is no next raise.** The table's own header
states the remedy past this point is a shard, not another budget bump — so the rounding rule cited
in the #0247 note is **no longer applicable to this row**; do not apply it and do not re-derive a
higher number.

Measured at #0118's close-out:

- **~15s** of margin against the 60s row from the quiet worst reading.
- **8.6s** of margin from the worst reading ever observed (the contended parallel case — which is
  exactly the measurement this change's own "10 consecutive green full parallel runs" acceptance
  bar produces).

**#0296 exists to shard this file** and is the settled remedy. This change must therefore do one of
two things, decided at plan time and stated in the plan:

1. **Stay inside the remaining margin** — the de-flake rewrite is a single assert, so this is the
   expected path; measure the file's runtime before and after and show the after-number still
   clears 60s against the *contended* reading, not the quiet one; or
2. **Land behind the shard** — if the work grows past that margin, take a `depends_on: [296]` and
   land after #0296 splits the file, rather than breaching a ceiling that has no raise.

#0154 also targets this same file; whichever of the three lands next inherits whatever is left.

## Reconcile log

### 2026-08-11 — reconciled against current `origin/main`: the fix already landed; change is obsolete

The spec (2026-08-09) is a snapshot artifact. Its entire fix shape was implemented and merged
before this run claimed the change, by **#0276** — whose build gate went red on this same SIGPIPE
class and whose integration repair eliminated the class suite-wide.

**The targeted assert was rewritten in `3b93574d` ("fix(0276): eliminate the SIGPIPE pipe class
suite-wide, guard it repo-wide"), merged to `main` via PR #190 on 2026-08-11.** It now reads the
herestring form, not a pipeline:

```sh
assert "reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims" \
  'grep <<<"$(cat "$tmp/reclaim-off-out.txt")" -qF "docket.sh reclaim-claims"'
```

Each of the spec's three deliverables was verified discharged, mechanically, not by reading:

1. **Rewrite the assert** — done by 0276. Whole-repo `/usr/bin/grep` for
   `printf "%s\n" "$(cat …)"` returns two hits, both *prose in comments*
   (`tests/test_docket_example_yml.sh:2233`, `tests/test_pipe_shapes.sh:15`) — zero executable
   sites remain.
2. **Mutation-test the rewritten assert** — run here, in an isolated copy of the tree, because a
   mechanical bulk rewrite is exactly where an assert goes vacuously green. Both probes confirmed
   a byte change before trusting any verdict:
   - *Targeted* (`removed=1 added=1`): replacing only the `docket.sh reclaim-claims` token in
     `scripts/docket-status.sh:1173` reddens **that assert alone**; the sibling count assert stays
     green. Precise isolation.
   - *Deletion* (`removed=1 added=0`): deleting the remedy `printf` reddens the target assert.
   The rewritten assert still guards what it guarded.
3. **Demonstrate stability** — `tests/test_docket_status.sh` runs 602/602 green, exit 0, standalone
   serial 47.56s.

**The class is now permanently guarded, so the value cannot regress.** `tests/test_pipe_shapes.sh`
is a repo-wide shape guard with a budget row (`tests/runtime-budgets.tsv:107`). Probing the guard
itself: clean tree ⇒ 0 failures; re-introducing the exact pre-0276 pipeline into the reclaim assert
(`removed=1 added=1`) ⇒ the guard reddens. A regression is caught by the suite, not by luck.

**Runtime-budget decision (the carry-forwards from #0247 and #0118).** Moot, and recorded rather
than skipped. This change's only proposed edit was one assert line that no longer needs editing, so
it adds zero runtime — neither carry-forward option applies. Option (b), `depends_on: [296]`, was
considered and rejected: #0296 is still `needs-brainstorm` (no spec), and parking an
already-satisfied change behind an ungroomed one would leave a permanently unbuildable entry in the
backlog. The 60s row is untouched by this outcome; #0296 and #0154 inherit it exactly as they
found it. The 10-consecutive-parallel-run acceptance bar was deliberately **not** run: it exists to
prove a fix that is already proven by 0276's own gate, and running it would spend the contended
budget headroom the #0118 carry-forward warns about for no deliverable.

**Outcome: killed as obsolete.** Nothing remains to build; a PR here would be empty.
