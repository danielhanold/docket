<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0332 — Route the -race test shards out of the parallel test pool** — `docs/changes/active/0332-route-race-shards-to-serial-lane.md`
<!-- docket:backlink:end -->

# Route the -race Test Shards Out of the Parallel Test Pool — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse the four parallel `go test -race` shards into one serial whole-module gate (`go test -race ./...`, 300s ceiling) so the race detector's GOMAXPROCS-wide workers stop oversubscribing the cores the parallel shell jobs need — with the 300s ceiling carried as a narrow, documented, self-guarding, mutation-tested exemption to the runtime-budget table's hard 60s ceiling.

**Architecture:** Five files change, nothing else: `tests/test_go_race.sh` is rewritten to run `go test -race ./...` (its package-exclusion derivation and four-shard completeness guard are removed — a single `./...` run covers the module by construction); the three sibling shards are deleted; `tests/runtime-budgets.tsv` replaces four `-race` rows with one `serial` 300s row and reconciles its header prose; `tests/test_runtime_budgets.sh` moves `EXPECTED_SERIAL` 0→1, re-derives `EXPECTED_TOTAL` from the edited table, and gains the shape-keyed exemption. The serial lane in `scripts/run-tests.sh` is already wired and needs no code change.

**Tech Stack:** Bash test scripts, awk, the repo's `scripts/run-tests.sh` runner, Go toolchain (`go test -race`).

**Spec:** `docs/superpowers/specs/2026-08-19-route-race-shards-to-serial-lane-design.md` (REWRITTEN 2026-08-20 — the authority; the earlier four-way lane flip is discarded).

**Prior state note (REPLAN):** This branch was reset to `origin/main` (`5e5f7dfd`). The halted run's lane-flip commit `d602ef1e` is gone; on this base `EXPECTED_SERIAL` is **0** (not 4) and `EXPECTED_TOTAL` is **2140**. All numbers below were verified against this base.

## Global Constraints

- **Out of scope (owned by follow-up change 0333):** partitioning `internal/app`; ANY edit to `scripts/run-tests.sh`; `GOMAXPROCS`/`-p` pinning. Touch none of these.
- The new budget row is exactly `tests/test_go_race.sh<TAB>300<TAB>serial` — real tab characters, three fields.
- `EXPECTED_TOTAL` is re-derived by awk-summing the EDITED table (`awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv`), never hand arithmetic. Sanity target: **2265** (2140 − 25 − 45 − 45 + (300 − 60)); if the awk sum disagrees, the awk sum wins and the discrepancy must be explained.
- The 60s-ceiling exemption must be: keyed on the race gate's path with its own explicit 300s sub-ceiling; bound to the serial lane (the single serial row and the single over-60 row must be the SAME file); documented at the counter as a TEMPORARY hole owned by follow-up change 0333; mutation-tested (Task 3).
- The exemption reaches the race gate only: 60s still binds every other row, and a race-gate creep past 300 still reddens.
- Repo rules that bite here: comment cross-references anchor on symbol names or verbatim-quoted clauses, never `file:line`; the canonical `assert(){ ... }` helper line is a byte-exact allowlist entry in `scripts/check-test-source-hygiene.sh` — copy it byte-for-byte, never retype it; mutation-restore only from committed state (`git checkout --` restores to HEAD, so mutate only AFTER the task's work is committed).
- Point-in-time records (`docs/results/*`, `docs/superpowers/plans/*` from earlier changes, archived changes, Accepted ADRs) keep their references to the deleted shard files — rewriting them falsifies history. Only current-state prose in maintained source must not dangle.

---

### Task 1: Rewrite `tests/test_go_race.sh` as the whole-module serial gate

**Files:**
- Modify: `tests/test_go_race.sh` (full rewrite below)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: the file whose path Task 2 keys the exemption on (`tests/test_go_race.sh`) and whose row Task 2 writes. After this task the file no longer references its three siblings, so Task 2 can delete them without breaking this file's asserts (on the current base, this file greps its siblings — deleting them first would redden it).

**What changes and why:** The body drops the `TXN_PKG`/`WS_PKG`/`PROC_PKG` derivation, the complement `grep -v`, and the entire completeness-guard block (union/disjointness/sibling-target asserts) — a single `go test -race ./...` covers `go list ./...` by construction, so there is nothing to partition and nothing to drift. The header is rewritten so no prose claims the shard structure still exists. The toolchain check, `GOFLAGS` append, and cache pinning are kept verbatim — they are orthogonal to the collapse.

- [ ] **Step 1: Replace the file's entire contents with the following**

```bash
#!/usr/bin/env bash
# tests/test_go_race.sh — the whole-module data-race gate (change 0308), collapsed
# back to a single `go test -race ./...` run by change 0332.
#
# HISTORY. Changes 0309, 0313, and 0314 sharded this gate four ways so each piece
# could fit under the parallel phase's hard 60s budget ceiling. Change 0332
# measured the shards and collapsed them: the shards existed to fit the PARALLEL
# phase, and the parallel phase is exactly what 0332 removed this gate from. In
# the serial lane the four shard invocations ran sequentially and summed to
# ~299s, while a single `go test -race ./...` invocation is ~206s because go test
# overlaps packages internally — so once serialized, the shard structure was not
# just unnecessary scaffolding but slower than not sharding. One `./...` run
# covers the module by construction: nothing to partition, no completeness guard
# to maintain.
#
# LANE AND CEILING. tests/runtime-budgets.tsv pins this file `serial` with a
# 300s row — the table's one documented exemption to the hard 60s ceiling (see
# the exemption note at RELIEF COUNTER A in tests/test_runtime_budgets.sh). The
# serial pin is the point of change 0332: `go test -race` spawns GOMAXPROCS-wide
# race workers, and inside the parallel `-j` fan-out those workers oversubscribe
# the cores the shell test jobs need, inflating every OTHER file's wall clock —
# the load-dependent gate that halted change 0329. Run alone in the serial phase
# this gate uses the whole machine, which is what an isolated gate should do, so
# its internal parallelism is deliberately NOT capped (no GOMAXPROCS/-p pin).
# The ~206s is dominated by internal/app's ~190s integration suite — a cost the
# race detector barely moves (~1.05x multiplier) and that no lane or `go list`
# shard can split, because internal/app is one Go package. The durable fix, a
# test-level partition of internal/app, is owned by follow-up change 0333; when
# it lands, this gate's row and the exemption shrink with it.
#
# WHY THIS IS ITS OWN FILE and not a fifth check inside
# tests/test_go_toolchain.sh. The detector is expensive — instrumented binaries
# run several times slower — and this file's 300s exemption is deliberately
# scoped to the race gate alone. Folding the detector into the Go gate would
# drag that file's row through the same exemption and blur the two verdicts.
#
# WHY REPO-WIDE and not an enumerated package list. The adapter surfaces held
# concurrently by design today are known — but an enumerated list gates only the
# packages someone remembered to enumerate, and the package that grows a race is
# by definition the one nobody thought of. `./...` is the shape-keyed spelling
# and needs no maintenance as packages are added.
#
# THIS FILE DOES NOT REPLACE THE PLAIN RUN. tests/test_go_toolchain.sh keeps its
# own `go test ./...`, which is also the single owner of the four-tuple CGO-off
# cross-build (TestCrossCompileApprovedTargets). The two runs answer different
# questions and their caches are independent — an instrumented build is a
# separate build-cache entry — so neither makes the other free.
#
# Requires a Go toolchain on PATH (go.mod pins the version); fails loudly if
# absent rather than skipping — a skipped gate certifies nothing. The race
# detector additionally requires cgo and a host C toolchain on a supported
# platform (linux/darwin/windows on amd64, and arm64 on linux/darwin); where it
# is unavailable `go test -race` fails loudly rather than silently degrading to
# an uninstrumented run, which is the behavior this gate wants.
#
# The assert helper is the tree's canonical one byte for byte: rule (a) of
# scripts/check-test-source-hygiene.sh is a byte-exact allowlist, and
# scripts/run-tests.sh accounts results on the `ok - ` / `NOT OK - ` markers
# it prints.
#
# CACHES. scripts/run-tests.sh gives every job a private HOME, so with
# GOMODCACHE/GOCACHE unset `go` finds neither a module cache nor a build cache
# and recompiles cold — network-dependent — on every suite run. This file pins
# both to `<git common dir>/docket-go-cache/{mod,build}` when the caller has not
# chosen its own, which is the same location and the same reasoning as
# tests/test_go_toolchain.sh (see the CACHES note in that file's header: outside
# every working tree, shared across worktrees, concurrent-safe, and `-modcacherw`
# required so an ordinary `rm -rf` can still remove it). Only the first run after
# a fresh clone needs the network.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the race gate cannot certify anything without a Go toolchain\n'
  exit 1
fi

# Keep whatever GOFLAGS the caller set; append rather than replace.
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"

# Pin the caches out of the job's throwaway HOME — see CACHES in this header.
if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
  common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
  if [ -n "$common_git_dir" ]; then
    # `--git-common-dir` answers relative to the working tree in a plain clone
    # and absolute from a linked worktree; normalize before building on it.
    case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
    cache_root="$common_git_dir/docket-go-cache"
    if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
      export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
      export GOCACHE="${GOCACHE:-$cache_root/build}"
    fi
  fi
fi

# The detector's verdict. A race is reported on stderr and turns the exit
# non-zero, so the captured output is replayed on failure rather than
# summarized — the WARNING block names the two conflicting stacks and is the
# whole diagnostic.
race_out="$(go test -race ./... 2>&1)"
race_rc=$?
assert "go test -race ./... (the whole module) passes" '[ "$race_rc" -eq 0 ] || { printf "%s\n" "$race_out" >&2; false; }'

exit "$fail"
```

Everything from `set -uo pipefail` through the cache-pinning block is byte-identical to the current file (including the canonical `assert(){ ... }` line — copy it from the existing file, do not retype it). Only the header prose and the post-cache body change.

- [ ] **Step 2: Syntax-check and prove the shard machinery is gone**

Run:
```bash
bash -n tests/test_go_race.sh
grep -c -e 'TXN_PKG\|WS_PKG\|PROC_PKG\|test_go_race_process\|test_go_race_transaction\|test_go_race_workspace\|completeness guard' tests/test_go_race.sh
```
Expected: `bash -n` silent; the grep prints `0` (exit 1). Any hit is leftover shard prose or logic — remove it.

- [ ] **Step 3: Run the rewritten gate**

Run: `bash tests/test_go_race.sh`
Expected: two `ok - ` lines (toolchain, whole-module race run), exit 0. **This takes ~3.5 minutes** (~206s measured standalone; warm Go caches) — do not kill it early. The three sibling shard files still exist at this point and still pass; they are deleted in Task 2, after this file stops asserting about them.

- [ ] **Step 4: Commit**

```bash
git add tests/test_go_race.sh
git commit -m "test(0332): collapse the race gate to a single whole-module go test -race ./... run"
```

---

### Task 2: One serial 300s row — budget guard first (red), then the table and shard deletions (green)

**Files:**
- Modify: `tests/test_runtime_budgets.sh`
- Modify: `tests/runtime-budgets.tsv`
- Delete: `tests/test_go_race_process.sh`, `tests/test_go_race_transaction.sh`, `tests/test_go_race_workspace.sh`

**Interfaces:**
- Consumes: Task 1's rewritten `tests/test_go_race.sh` (no longer greps its siblings, so they can be deleted here).
- Produces: `RACE_GATE="tests/test_go_race.sh"` and `RACE_CEILING=300` shell constants inside `tests/test_runtime_budgets.sh`, the exemption logic Task 3 mutation-tests, and the final table Task 4 sweeps against.

**Why one task:** the guard's counters (`EXPECTED_SERIAL`, `EXPECTED_TOTAL`, the exemption) and the table/file changes are two halves of one atomic swap — any commit carrying only one half leaves `tests/test_runtime_budgets.sh` red. TDD still applies: the guard edits ARE the failing test (they redden against the unedited table), and the table edit plus deletions make them pass.

- [ ] **Step 1: Edit `tests/test_runtime_budgets.sh` — the failing guard**

Four edits. Apply all of them before running anything.

**(a) `EXPECTED_SERIAL` 0 → 1.** Replace the current declaration-plus-comment:

```bash
EXPECTED_SERIAL=0   # files pinned serial by the change-0227 audit. RAISING THIS IS A FINDING:
                    # a serial pin removes a file from the parallel phase, so it must be justified
                    # in the same diff with the shared state that forced it.
```

with:

```bash
EXPECTED_SERIAL=1   # tests/test_go_race.sh (change 0332). The shared state that forces the pin is
                    # the machine's cores: `go test -race` spawns GOMAXPROCS-wide race workers,
                    # which inside the parallel fan-out oversubscribe the cores every other job
                    # needs — the load-dependent gate that halted change 0329. RAISING THIS IS A
                    # FINDING: a serial pin removes a file from the parallel phase, so it must be
                    # justified in the same diff with the shared state that forced it.
```

**(b) `EXPECTED_TOTAL` 2140 → 2265, with a new ledger entry.** Replace the first two lines of the declaration:

```bash
EXPECTED_TOTAL=2140 # the sum of every ceiling, seeded with the table from the measured serial run.
                    # 2115 -> 2140 (change 0316): ONE legitimate mover — a NEW test file brings its
```

with:

```bash
EXPECTED_TOTAL=2265 # the sum of every ceiling, seeded with the table from the measured serial run.
                    # 2140 -> 2265 (change 0332): the SHARD-RE-CUT case, in reverse — four race
                    # rows collapse into one. The serial lane removes the oversubscription the
                    # shards were cut for, and in that lane the four shard invocations ran
                    # sequentially anyway, summing to ~299s, while one `go test -race ./...`
                    # invocation is ~206s because go test overlaps packages internally. So
                    # tests/test_go_race_process.sh (25), tests/test_go_race_transaction.sh (45)
                    # and tests/test_go_race_workspace.sh (45) are DELETED, and
                    # tests/test_go_race.sh moves 60 parallel -> 300 serial — the one documented
                    # exemption to the 60s ceiling (see RELIEF COUNTER A), a TEMPORARY hole owned
                    # by follow-up change 0333. Net move: -115 for the deleted rows, +240 for the
                    # re-cut row.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 2115 -> 2140 (change 0316): ONE legitimate mover — a NEW test file brings its
```

Every other existing ledger line stays byte-identical — the chain is dated history and is never rewritten.

**(c) The exemption at RELIEF COUNTER A (assertion 3).** Replace:

```bash
# (3) RELIEF COUNTER A — rows above the hard ceiling. Independent of (2): laundering a single
# file's ceiling upward leaves (2) green and reddens only this.
over="$(awk -F'\t' -v c="$CEILING" '$2 > c {print $1}' <<<"$rows")"
assert "no budget row exceeds the ${CEILING}s ceiling" \
  '[ -z "$over" ] || { echo "  over ceiling: $over" >&2; echo "  Shard the file or move its new assertions into a shard with room. Raising the ceiling is not the remedy." >&2; false; }'
```

with:

```bash
# (3) RELIEF COUNTER A — rows above the hard ceiling. Independent of (2): laundering a single
# file's ceiling upward leaves (2) green and reddens only this.
#
# ONE DOCUMENTED EXEMPTION (change 0332) — A TEMPORARY HOLE, owned by follow-up change 0333.
# tests/test_go_race.sh is the serial-isolated whole-module race gate; its ~206s is dominated by
# internal/app's ~190s integration suite — a cost the race detector barely moves (~1.05x) and that
# no lane or `go list` shard can split, because internal/app is one Go package. Until change 0333
# partitions that package, the race gate answers to its own explicit sub-ceiling here instead of
# the 60s one. The exemption is keyed on the row's PATH: every other row still answers to the 60s
# ceiling, and the race gate creeping past ${RACE_CEILING}s still reddens this same assert. The
# serial binding lives with assertion (4): the single serial row must BE the race gate, so an
# exempt row cannot ride the parallel phase and reintroduce the oversubscription 0332 removed.
RACE_GATE="tests/test_go_race.sh"
RACE_CEILING=300
over="$(awk -F'\t' -v c="$CEILING" -v rg="$RACE_GATE" -v rc="$RACE_CEILING" \
  '($1 == rg ? $2 > rc : $2 > c) {print $1}' <<<"$rows")"
assert "no budget row exceeds the ${CEILING}s ceiling (sole exemption: $RACE_GATE at ${RACE_CEILING}s)" \
  '[ -z "$over" ] || { echo "  over ceiling: $over" >&2; echo "  Shard the file or move its new assertions into a shard with room. Raising the ceiling is not the remedy." >&2; false; }'
```

(Note: `${RACE_CEILING}s` inside the comment block is prose, not an expansion — comments do not expand; write it as literal text exactly as shown.)

**(d) The serial-identity tie, immediately after assertion (4).** After the existing `assert "exactly $EXPECTED_SERIAL files are pinned serial" ...` block, insert:

```bash
# (4b) the serial pin and the ceiling exemption must be the SAME file: an exempt row riding the
# parallel phase would reintroduce the oversubscription change 0332 removed, and a second serial
# row would hide behind the exemption's justification. Both directions collapse to one equality.
serial_rows="$(awk -F'\t' '$3 == "serial" {print $1}' <<<"$rows")"
assert "the serial row is the race gate (the exemption is serial-bound)" \
  '[ "$serial_rows" = "$RACE_GATE" ] || { echo "  serial rows: ${serial_rows:-<none>}" >&2; echo "  The one sanctioned serial row is $RACE_GATE — its 300s exemption exists only because the serial lane isolates it. Any other serial pin needs its own justified counter bump." >&2; false; }'
```

- [ ] **Step 2: Run the guard — verify it fails against the unedited table**

Run: `bash tests/test_runtime_budgets.sh`
Expected: FAIL (exit 1) with exactly these three asserts NOT OK — `exactly 1 files are pinned serial`, `the serial row is the race gate (the exemption is serial-bound)`, and `the table's budgeted total is 2265 seconds`. Everything else stays `ok - `. If assertion (3) also reddens, the exemption awk is wrong — with the old table (no row over 60) it must stay green.

- [ ] **Step 3: Edit `tests/runtime-budgets.tsv` — data rows**

Replace the four race rows:

```
tests/test_go_race.sh	60	parallel
tests/test_go_race_process.sh	25	parallel
tests/test_go_race_transaction.sh	45	parallel
tests/test_go_race_workspace.sh	45	parallel
```

with the single row (fields separated by real tabs, in the same position in the file):

```
tests/test_go_race.sh	300	serial
```

Verify the row parses: `awk -F'\t' '$1=="tests/test_go_race.sh"{print $2, $3}' tests/runtime-budgets.tsv` must print exactly `300 serial`.

- [ ] **Step 4: Edit `tests/runtime-budgets.tsv` — header prose**

Three edits:

**(a)** The rule paragraph near the top currently begins `# NO row may exceed 60 seconds.` Change that first sentence to:

```
# NO row may exceed 60 seconds — with ONE documented exemption: tests/test_go_race.sh, the
# serial-isolated whole-module race gate, carries a 300s row under its own sub-ceiling in
# tests/test_runtime_budgets.sh (RELIEF COUNTER A; a TEMPORARY hole owned by follow-up change
# 0333). If any other file outgrows its ceiling, shard it or move the new
```

(then the paragraph continues unchanged from `# assertions into a shard with room. ...`).

**(b)** Delete these four header paragraphs wholesale — they document rows and files that no longer exist (each starts with the quoted lead-in and runs to its final `EXPECTED_TOTAL moves ...` / `...unchanged.` sentence):
- `# tests/test_go_race.sh is a NEW FILE brought by change 0308 ...` (ends `... EXPECTED_TOTAL moves 1825 -> 1885.`)
- `# tests/test_go_race_transaction.sh is a NEW FILE brought by change 0309 ...` (ends `... plus the tests/test_go_toolchain.sh re-budget below.`)
- `# tests/test_go_race_workspace.sh is a NEW FILE brought by change 0313 ...` (ends `... so that row is unchanged.`)
- `# tests/test_go_race_process.sh is a NEW FILE brought by change 0314 ...` (ends `... plus the tests/test_go_toolchain.sh re-budget below.`)

The two standalone `tests/test_go_toolchain.sh` re-budget paragraphs (`20 -> 45 (change 0309)` and `45 -> 55 (change 0314)`) STAY — they document a live row and name no deleted file. Confirm with `grep -n 'test_go_race_' tests/runtime-budgets.tsv` after the edit: zero hits.

**(c)** In place of the deleted 0308 paragraph, insert the new row's rationale:

```
# tests/test_go_race.sh is the whole-module data-race gate (change 0308), sharded four ways by
# changes 0309/0313/0314 to fit the parallel phase's 60s ceiling, and COLLAPSED back to a single
# `go test -race ./...` run by change 0332 — which also moved it to the SERIAL lane. The shards
# existed to fit the parallel phase, and the parallel phase is what 0332 removed this gate from:
# `go test -race` spawns GOMAXPROCS-wide race workers that oversubscribed the cores the other
# parallel jobs needed, making the whole suite load-dependent (change 0329's build-gate halt). In
# the serial lane the four shard invocations ran sequentially and summed to ~299s, while one
# `./...` invocation measures ~206s standalone serial because go test overlaps packages
# internally — the collapse is strictly faster AND retires the four-shard completeness guard.
# The ~206s is dominated by internal/app (~200s under -race, ~190s uninstrumented — a ~1.05x
# multiplier: the cost is the package's own integration suite, one Go package, unshardable by
# `go list`). The row is 300, ABOVE the 60s hard ceiling: this table's ONE documented exemption,
# path-keyed and sub-ceilinged at RELIEF COUNTER A in tests/test_runtime_budgets.sh, serial-bound
# by that file's serial-identity assert, TEMPORARY, and owned by follow-up change 0333 (partition
# internal/app). 300 rather than the sizing rule's 215 (206 -> 210 + 5) is deliberate headroom
# for a ~3.5-minute gate whose reading moves with machine load; when 0333 lands, the row and the
# exemption shrink with it. EXPECTED_TOTAL moves 2140 -> 2265: -25 -45 -45 for the deleted shard
# rows, +240 for the 60 -> 300 re-cut.
```

- [ ] **Step 5: Delete the three sibling shards**

```bash
git rm tests/test_go_race_process.sh tests/test_go_race_transaction.sh tests/test_go_race_workspace.sh
```

- [ ] **Step 6: Run the guard — verify it passes, and verify the total from the table**

Run:
```bash
bash tests/test_runtime_budgets.sh
awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
```
Expected: all asserts `ok - `, exit 0; the awk sum prints `2265`. If the sum differs, the table edit is wrong (a stray row or a mistyped ceiling) — fix the table, never the constant, unless a recount shows the base truly differed, in which case set `EXPECTED_TOTAL` to the awk value and say so in the ledger entry.

- [ ] **Step 7: Commit**

```bash
git add tests/test_runtime_budgets.sh tests/runtime-budgets.tsv
git commit -m "test(0332): one serial 300s race-gate row, with a documented, path-keyed 60s-ceiling exemption"
```

(`git rm` already staged the deletions; `git status --porcelain` must show a clean tree after the commit.)

---

### Task 3: Mutation-test the exemption (verification-only — no diff, no commit)

**Files:**
- Temporarily mutate, then restore: `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: Task 2's committed guard (`RACE_GATE`/`RACE_CEILING`, assertions 3, 4, 4b) and committed table.
- Produces: recorded mutation evidence for the build-evidence record. No file changes survive this task.

**Ground rules:** Run these AFTER Task 2's commit — the restore step is `git checkout -- tests/runtime-budgets.tsv`, which restores to HEAD, and that is only safe when HEAD already carries the edit under test (repo learning: a `git checkout --` restore against uncommitted work silently destroys it). After each mutation, restore and confirm `git status --porcelain` is empty before the next. The guard is a plain bash script with no result cache, so no cache-defeat flag is needed; capture each run's output for the evidence record.

- [ ] **Step 1: Mutation A — a race-gate creep past 300 must redden the exemption**

```bash
sed -i '' $'s|^tests/test_go_race.sh\t300\tserial$|tests/test_go_race.sh\t999\tserial|' tests/runtime-budgets.tsv
bash tests/test_runtime_budgets.sh; echo "exit=$?"
git checkout -- tests/runtime-budgets.tsv
```
Expected: `NOT OK - no budget row exceeds the 60s ceiling (sole exemption: tests/test_go_race.sh at 300s)` with `over ceiling: tests/test_go_race.sh` on stderr, plus `NOT OK` on the total (assertion 5); exit 1. If assertion 3 stays green, the sub-ceiling is not being applied — the exemption is a blanket hole and must be fixed before proceeding. Before believing the sed landed, confirm the mutated row: `grep -c '999' tests/runtime-budgets.tsv` prints 1.

- [ ] **Step 2: Mutation B — the exemption must not cover any other path**

```bash
sed -i '' $'s|^tests/test_adr_checks.sh\t10\tparallel$|tests/test_adr_checks.sh\t70\tparallel|' tests/runtime-budgets.tsv
bash tests/test_runtime_budgets.sh; echo "exit=$?"
git checkout -- tests/runtime-budgets.tsv
```
Expected: `NOT OK - no budget row exceeds the 60s ceiling ...` with `over ceiling: tests/test_adr_checks.sh`; exit 1. This proves 60s still binds every non-race row — the key is the path, not the presence of an exemption.

- [ ] **Step 3: Mutation C — the exemption must be serial-bound**

```bash
sed -i '' $'s|^tests/test_go_race.sh\t300\tserial$|tests/test_go_race.sh\t300\tparallel|' tests/runtime-budgets.tsv
bash tests/test_runtime_budgets.sh; echo "exit=$?"
git checkout -- tests/runtime-budgets.tsv
```
Expected: `NOT OK - exactly 1 files are pinned serial` AND `NOT OK - the serial row is the race gate (the exemption is serial-bound)`; exit 1. A parallel race gate is exactly the state this change removes, and two independent asserts must both catch it.

- [ ] **Step 4: Confirm restoration and record the evidence**

Run: `git status --porcelain` (must print nothing) and `bash tests/test_runtime_budgets.sh` once more (must be green, exit 0). Record all three mutation outputs (the specific `NOT OK` lines observed) in the task report for the build-evidence record. This task intentionally produces no commit.

---

### Task 4: Stale-reference sweep and gate expectations

**Files:**
- Modify: none expected; only if the sweep finds a current-state straggler.

**Interfaces:**
- Consumes: everything committed by Tasks 1–2.
- Produces: the sweep verdict for the build-evidence record; the full-suite gate that follows the last task is the change's validation.

- [ ] **Step 1: Whole-repo sweep for the deleted names — derive, then classify**

```bash
grep -rn -e 'test_go_race_process\|test_go_race_transaction\|test_go_race_workspace' --exclude-dir=.git . | grep -v -e '^\./docs/results/' -e '^\./docs/superpowers/plans/' -e '^\./docs/changes/'
```

Classification rule (from the spec and the repo's cross-reference rule): hits inside `docs/results/`, `docs/superpowers/plans/` (including this plan), and archived/active change files are point-in-time records — they stay untouched, hence the filter. What remains after the filter must be only **dated ledger entries** in `tests/test_runtime_budgets.sh`'s `EXPECTED_TOTAL` chain (lines of the form `NNNN -> NNNN (change NNNN): ...` — historical receipts, kept by design, including the new 0332 entry that names the deleted files as deleted) and the historical `-race` war-story entries nowhere else. Any hit that reads as a claim about the CURRENT tree — a header describing a live sibling, a doc pointing at a shard file — is a defect: fix it in the file where it lives and amend it into a small follow-on commit. Also verify `grep -rn 'TXN_PKG\|WS_PKG\|PROC_PKG' --exclude-dir=.git .` matches nothing outside point-in-time records.

- [ ] **Step 2: Sanity-check the runner schedules the gate serial (read-only — do NOT edit the runner)**

```bash
grep -n 'serial' scripts/run-tests.sh | head
```
Expected: the existing serial-lane machinery (`mode_of`, the `PAR`/`SER` split) is present and unmodified — `git diff origin/main -- scripts/run-tests.sh` must be empty. The lane is data-driven from the table; no runner change is in scope.

- [ ] **Step 3: Full-suite gate expectations (for the build gate that follows the plan)**

The build's final full-suite gate runs whatever `finalize.test_command` resolves to (read it from `.docket.local.yml` then `.docket.yml` — never a second copy). Expectations to verify in its output:
- The run is GREEN, including `tests/test_go_race.sh` scheduled in the serial phase — a green run under whatever load the machine has is the direct evidence the 0329 failure mode is gone.
- NO `OVER BUDGET:` line for `tests/test_go_race.sh` (its 300s row covers the ~206s reading; the runner's slack factor covers load on top). An `OVER BUDGET:` line for any file is a finding to act on, not noise.
- Wall clock will be LONGER than recent runs (~206s of serial phase appended after the parallel phase) — that is the designed trade, not a regression.

---

## Self-Review

- **Spec coverage:** Decision item 1 (whole-module serial gate, exclusions and completeness guard removed, header reconciled) → Task 1. Item 2 (delete three siblings) → Task 2 Step 5. Item 3 (one `300 serial` row, header prose reconciled incl. the toolchain-note check) → Task 2 Steps 3–4. Item 4 (`EXPECTED_SERIAL` 1, awk-derived `EXPECTED_TOTAL`, narrow documented exemption, serial binding, 0333 ownership) → Task 2 Step 1. Exemption mutation-testing → Task 3. "No stale references" validation → Task 1 Step 2, Task 2 Step 4(b), Task 4 Step 1. Self-validating build gate → Task 4 Step 3. Out-of-scope fence (run-tests.sh, GOMAXPROCS, internal/app) → Global Constraints and Task 4 Step 2.
- **Placeholder scan:** all code steps carry full content; no TBDs.
- **Consistency:** `RACE_GATE`/`RACE_CEILING` named identically in Task 2(c), Task 2(d)'s message prose, and Task 3's expectations; the row string `tests/test_go_race.sh<TAB>300<TAB>serial` and the total 2265 are the same everywhere; `EXPECTED_SERIAL=1` matches assertion 4's message `exactly 1 files are pinned serial` as asserted in Task 2 Step 2 and Task 3 Step 3.
