<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0333 — Partition slow Go integration tests and retire the race gate's 300s ceiling exemption](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0333-partition-internal-app-to-retire-the-race-gate-s-300s-ceilin.md)**
<!-- docket:backlink:end -->
# Partition Slow Go Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build (this change's resolved build skill) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move slow real-process integration tests in `internal/app`, `internal/githubcli`, and `internal/gitcli` behind one `integration` build tag, run them through declarative feature-named `tests/test_*.sh` shards guarded by a fail-closed contract, and return `tests/test_go_race.sh` to the parallel lane so the runtime-budget table's 300s exemption, `RACE_GATE`/`RACE_CEILING`, and the serial coupling can be deleted.

**Architecture:** One structural boundary (`//go:build integration` as each moved file's first line) partitions the slow corpus out of ordinary `go test ./...`. Tagged tests carry structural prefixes — `TestIntegration<Feature>` for sequential drivers, `TestRaceIntegration<Feature>` (in `*_race_integration_test.go` files) for the few tests exercising real concurrent behavior. Each shard is a thin `tests/test_go_integration_*.sh` runner declaring three literals (package, prefix, mode) and delegating to one shared helper in `tests/lib/`, which also serves an inspection mode so the contract test reads shard membership from the live runner invocation, never a second registry. A dedicated contract test proves: every tagged test is assigned to exactly one shard in the correct race mode, no tagged test leaks into the default corpus, no runner is a stale no-op, and `go vet -tags integration` passes. The budget machinery then loses all change-0332 temporary machinery, with every new/changed row re-derived from fresh standalone measurements.

**Tech Stack:** Go toolchain (build tags, `-list`, `-run`, `-race`, `-count=1`), Bash test runners under `scripts/run-tests.sh` discovery, `tests/runtime-budgets.tsv` + `tests/test_runtime_budgets.sh`.

**Spec:** `docs/superpowers/specs/2026-08-27-partition-slow-go-integration-tests-design.md` (on the `docket` metadata branch; synchronized copy read at plan time). The plan argues from that spec; executors read both.

## Global Constraints

- **Measurements are build-time truth, never frozen estimates.** Every shard cut, every budget row, `EXPECTED_SERIAL`, and `EXPECTED_TOTAL` are derived from FRESH standalone measurements taken during the build on this machine — the per-feature groupings named below are starting hypotheses from the spec, and the measured numbers decide the final cuts. Record every reading (command, count, worst value) in the tsv header note and the build evidence, per learnings `tolerance-constant-calibrated-on-one-machine` (record the measurement, not just the number) and `optimization-needs-a-measured-oracle` (green asserts cannot prove a performance outcome).
- **No Go timeout raise, no budget raise, anywhere.** A slow shard is reshaped (better cut, evidenced `t.Parallel()`, removal of duplicated execution) — never absorbed by a bigger number or a `-timeout` flag. Do not add `-timeout` to any gate or shard.
- **Shard target is 45–50s standalone wall clock; every row ≤ 60s.** A shard measuring 51–60s has no headroom and must be split again (learnings `budget-headroom-is-spent-before-it-is-breached`: parity is the finding, not the breach). Row sizing follows the table's own rule: next multiple of 5 above the worst standalone serial reading, plus 5s margin, minimum 10.
- **`-count=1` on every test execution and every mutation probe/verification** (learnings `cached-runner-serves-a-mutated-tree`). `go test -list` compiles but does not execute, so it needs no `-count=1`; every `go test` that produces a pass/fail verdict does.
- **Scenario fidelity.** Aside from build constraints, top-level renaming, helper relocation, and isolation-audited `t.Parallel()` additions, test assertions and scenario logic are byte-unchanged. Every original test survives exactly once (Task 1 snapshots the before-inventory; Task 8 proves the mapping).
- **The `e2e` tag is untouched.** `internal/app/finalize_e2e_test.go`, its `TestE2E*` matrix, and `tests/test_go_finalize_e2e.sh` stay exactly as they are. `finalize_e2e_test.go` does not match the `*_integration_test.go` glob and `TestE2E` is not an integration prefix, so the contract never sees it.
- **`t.Parallel()` is an evidenced optimization, not a default.** Add it inside a new shard only after a written isolation audit (own temp repos, no `t.Setenv`/process-env mutation, no package-global mutation, no shared named resources) recorded in the build evidence. When in doubt, leave it out — runner-level parallelism is the baseline.
- **Canonical assert helper, byte for byte,** in every new `tests/test_*.sh` file: `assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }` — rule (a) of `scripts/check-test-source-hygiene.sh` is a byte-exact allowlist.
- **Every new `tests/test_*.sh` file lands with its budget row and an `EXPECTED_TOTAL` bump in the same task** — `tests/test_runtime_budgets.sh` check (2) reddens on any test file without a row, so each task leaves the whole suite green (learnings `intermediate-task-state-buildable`).
- **Mutation-test procedure:** back up with `cp "$f" "$f.bak"`, mutate, PROVE the mutation landed (`git diff -- "$f"` shows the change) before reading any result, run the probe, restore with `mv "$f.bak" "$f"`. Never `git checkout -- <file>` as a restore (learnings `mutation-restore-needs-a-backup-copy` and its landed-mutation corollary).
- **Cross-references anchor on symbol names or verbatim-quoted clauses, never line numbers** (`tests/test_comment_anchor_style.sh`, ADR-0054). Point-in-time records (archived changes, specs, old tsv ledger entries) keep their historical wording — the restatement sweep in Task 7 touches maintained source only.
- **Shell house rules:** never `producer | grep -q/head` under pipefail (capture first), `grep -E -e` for leading-dash patterns, `mktemp` always with a template.
- The full-suite gate at the end of the build is whatever `finalize.test_command` resolves to (docket-build's suite gate runs it); per-task verification below is the focused cycle only.

## File Structure

```
tests/lib/go-integration-shard.sh              NEW  shared shard executor + inspection mode
tests/test_go_integration_<pkg>_<feature>.sh   NEW  one per measured shard (normal + race), count set at build time by measurement
tests/test_go_integration_contract.sh          NEW  the fail-closed completeness contract (8 checks)
internal/app/*_integration_test.go             NEW  moved/split slow tests (tagged)
internal/app/*_race_integration_test.go        NEW  concurrency-bearing tagged tests (if any qualify)
internal/githubcli/… , internal/gitcli/…       NEW  same shapes
tests/test_go_race.sh                          MOD  header rewrite; returns to parallel over the fast corpus
tests/runtime-budgets.tsv                      MOD  exemption prose deleted; race row re-cut; new rows; ledger entry
tests/test_runtime_budgets.sh                  MOD  RACE_GATE/RACE_CEILING/(4b) deleted; EXPECTED_SERIAL, EXPECTED_TOTAL re-derived
docs/superpowers/plans/2026-08-27-0333-inventory-before.txt  NEW  Task 1 snapshot (point-in-time record)
docs/superpowers/plans/2026-08-27-0333-inventory-map.txt     NEW  cumulative old→new mapping (Tasks 3–5 append)
```

Current corpus (verified at plan time): `internal/app` 59 test files / 435 top-level tests (one already tagged `e2e`), `internal/githubcli` 11 files / 78 tests, `internal/gitcli` 19 files / 87 tests. The only pre-existing build tag in the three packages is `internal/app/finalize_e2e_test.go`.

---

### Task 1: Baseline inventory and fresh measurements

**Files:**
- Create: `docs/superpowers/plans/2026-08-27-0333-inventory-before.txt`
- Create: `docs/superpowers/plans/2026-08-27-0333-inventory-map.txt` (header only; Tasks 3–5 append)

**Interfaces:**
- Produces: the before-inventory file, format one line per test: `<import-path-suffix><TAB><TestName>` (e.g. `internal/app	TestFinalizeMerge`), sorted; and the mapping file, format `<pkg><TAB><old-name><TAB><new-name>` (identical names for unrenamed moves), which Task 8's fidelity check consumes.
- Produces (evidence only): fresh standalone package baselines and per-test timings that Tasks 3–5 use to draw cuts.

- [ ] **Step 1: Snapshot the top-level test inventory of all three packages (all tag states)**

```bash
cd "$(git rev-parse --show-toplevel)"
out=docs/superpowers/plans/2026-08-27-0333-inventory-before.txt
: > "$out"
for pkg in internal/app internal/githubcli internal/gitcli; do
  for tags in "" "-tags e2e" "-tags integration"; do
    # -list compiles but does not run; failures here are fatal findings, not skips.
    go test $tags -list '^Test' "./$pkg" | grep -E '^Test' | while read -r t; do
      printf '%s\t%s\n' "$pkg" "$t"
    done
  done
done
LC_ALL=C sort -u -o "$out" "$out"
wc -l "$out"   # expect ~600 lines (435+78+87, e2e matrix included; integration corpus is empty today)
printf '# pkg\told-name\tnew-name — appended by Tasks 3–5; consumed by Task 8\n' \
  > docs/superpowers/plans/2026-08-27-0333-inventory-map.txt
```

- [ ] **Step 2: Take fresh standalone package baselines (the numbers the results file reports as "before")**

```bash
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
# Use the shared caches the gates use, so readings are warm-cache like the suite's:
cache_root="$(git rev-parse --git-common-dir)/docket-go-cache"
export GOMODCACHE="$cache_root/mod" GOCACHE="$cache_root/build"
for pkg in ./internal/app ./internal/githubcli ./internal/gitcli; do
  /usr/bin/time -p go test -count=1 "$pkg"          2>&1 | tail -4
  /usr/bin/time -p go test -race -count=1 -timeout 30m "$pkg" 2>&1 | tail -4
done
```

Record all six readings in the build evidence. Expected shape per the change file's 2026-08-27 measurements: githubcli ~129s and gitcli ~91s under `-race`; `internal/app` may need the explicit `-timeout 30m` to finish at all (Go's default 10m per-package deadline is the very blocker this change removes — the flag is a MEASUREMENT accommodation only and ships in no committed file).

- [ ] **Step 3: Capture per-test timings for cut-drawing**

```bash
for pkg in ./internal/app ./internal/githubcli ./internal/gitcli; do
  go test -count=1 -timeout 30m -v "$pkg" 2>&1 | grep -E -e '^--- (PASS|FAIL)' \
    | sort -t'(' -k2 -rn > "/tmp/claude/0333-timings-$(basename "$pkg").txt" 2>/dev/null \
    || go test -count=1 -timeout 30m -v "$pkg" 2>&1 | grep -E -e '^--- (PASS|FAIL)' \
    > "docs/superpowers/plans/../../..$(basename "$pkg")-scratch.txt"
done
```

(Use the session scratchpad directory if `/tmp` is unavailable; these per-test listings are working data, not committed artifacts — only the summed per-feature groupings go into the evidence.)

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-08-27-0333-inventory-before.txt \
        docs/superpowers/plans/2026-08-27-0333-inventory-map.txt
git commit -m "test(0333): baseline inventory snapshot for the integration partition"
```

---

### Task 2: The shared shard helper with inspection mode

**Files:**
- Create: `tests/lib/go-integration-shard.sh`

**Interfaces:**
- Produces (consumed by every runner in Tasks 3–5): a sourceable library. A runner defines `SHARD_PKG` (one of `./internal/app`, `./internal/githubcli`, `./internal/gitcli`), `SHARD_PREFIX` (a `TestIntegration…`/`TestRaceIntegration…` top-level prefix), `SHARD_MODE` (`normal`|`race`), plus the canonical `assert`/`fail`, sources the library, then calls `shard_inspect_maybe` and `run_integration_shard`.
- Produces (consumed by Task 6's contract): under `DOCKET_SHARD_INSPECT=1`, a runner prints exactly three lines — `package=…`, `prefix=…`, `mode=…` — and exits 0 without running any test. The LIVE runner invocation is the source of truth for shard membership.

- [x] **Step 1: Write the library**

```bash
#!/usr/bin/env bash
# tests/lib/go-integration-shard.sh — shared executor for the Go integration shard
# runners (change 0333). Each tests/test_go_integration_*.sh runner declares three
# literals — SHARD_PKG, SHARD_PREFIX, SHARD_MODE — defines the canonical assert,
# sources this file, then calls shard_inspect_maybe and run_integration_shard.
#
# INSPECTION MODE. Under DOCKET_SHARD_INSPECT=1 the runner prints its three
# declarations and exits 0 WITHOUT running go test. The contract test
# (tests/test_go_integration_contract.sh) reads shard membership from this live
# invocation, never from a duplicated registry — the runner's own execution path
# and its inspected declaration cannot drift apart because they are one file.
#
# -count=1 IS MANDATORY on the executing run: correctness, completeness, and
# performance evidence must never be served from Go's test-result cache
# (learning cached-runner-serves-a-mutated-tree). This helper never converts a
# failed go test into an empty success: output is captured and replayed on the
# failure path, and an empty selection is a red assert, not a no-op.
#
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the
# CACHES note in that file's header): <git common dir>/docket-go-cache/{mod,build},
# shared across worktrees, concurrent-safe, -modcacherw required.

shard_inspect_maybe(){
  if [ "${DOCKET_SHARD_INSPECT:-0}" = "1" ]; then
    printf 'package=%s\nprefix=%s\nmode=%s\n' "$SHARD_PKG" "$SHARD_PREFIX" "$SHARD_MODE"
    exit 0
  fi
}

run_integration_shard(){
  assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
  if ! command -v go >/dev/null 2>&1; then
    printf 'NOT OK - this integration shard cannot certify anything without a Go toolchain\n'
    exit 1
  fi

  export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
  if [ -z "${GOMODCACHE:-}" ] || [ -z "${GOCACHE:-}" ]; then
    common_git_dir="$(git rev-parse --git-common-dir 2>/dev/null)"
    if [ -n "$common_git_dir" ]; then
      case "$common_git_dir" in /*) ;; *) common_git_dir="$REPO/$common_git_dir" ;; esac
      cache_root="$common_git_dir/docket-go-cache"
      if mkdir -p "$cache_root/mod" "$cache_root/build" 2>/dev/null; then
        export GOMODCACHE="${GOMODCACHE:-$cache_root/mod}"
        export GOCACHE="${GOCACHE:-$cache_root/build}"
      fi
    fi
  fi

  race_flag=""
  [ "$SHARD_MODE" = "race" ] && race_flag="-race"

  # The prefix must select at least one test — a renamed corpus must redden the
  # shard, never let it pass vacuously. -list compiles but does not execute.
  listed="$(go test -tags integration -list "^${SHARD_PREFIX}" "$SHARD_PKG" 2>&1)"
  listed_rc=$?
  declared="$(grep -c -E -e '^Test' <<<"$listed")"
  assert "prefix ^${SHARD_PREFIX} selects at least one tagged test in ${SHARD_PKG}" \
    '[ "$listed_rc" -eq 0 ] && [ "$declared" -ge 1 ] || { printf "%s\n" "$listed" >&2; false; }'

  # The shard itself. -v so per-test PASS markers can be counted against the
  # declared selection, catching a -run filter that silently narrowed.
  test_out="$(go test -tags integration $race_flag -count=1 -run "^${SHARD_PREFIX}" -v "$SHARD_PKG" 2>&1)"
  test_rc=$?
  assert "go test -tags integration ${race_flag:+$race_flag }-run ^${SHARD_PREFIX} ${SHARD_PKG} passes" \
    '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
  ran="$(printf '%s\n' "$test_out" | grep -c -E -e "^--- PASS: ${SHARD_PREFIX}")"
  assert "every selected test actually ran and passed (${declared} declared)" '[ "$ran" -eq "$declared" ]'
}
```

- [x] **Step 2: Smoke the inspection path with a throwaway runner (not committed)**

```bash
cat > /tmp/claude-shard-smoke.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"  # smoke only
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
SHARD_PKG="./internal/gitcli"; SHARD_PREFIX="TestIntegrationSmoke"; SHARD_MODE="normal"
. "$(git rev-parse --show-toplevel)/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
EOF
DOCKET_SHARD_INSPECT=1 bash /tmp/claude-shard-smoke.sh
# Expected: exactly three lines package=./internal/gitcli / prefix=TestIntegrationSmoke / mode=normal, exit 0
bash /tmp/claude-shard-smoke.sh; echo "rc=$?"
# Expected: NOT OK on the selects-at-least-one assert (no tagged tests exist yet), rc=1 — the fail-closed direction.
rm /tmp/claude-shard-smoke.sh
```

- [x] **Step 3: Commit**

```bash
git add tests/lib/go-integration-shard.sh
git commit -m "test(0333): shared Go integration shard executor with inspection mode"
```

---

### Task 3: Partition `internal/gitcli` (pattern-prover: smallest package)

**Files:**
- Modify/split: slow files under `internal/gitcli/` → `*_integration_test.go` / `*_race_integration_test.go`
- Create: `tests/test_go_integration_gitcli_<feature>.sh` runners (count decided by measurement; spec's starting hypothesis: batch/blob protocols, process timeout/wait behavior, repository/worktree operations)
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` only)
- Append: `docs/superpowers/plans/2026-08-27-0333-inventory-map.txt`

**Interfaces:**
- Consumes: Task 2's runner protocol (`SHARD_PKG`/`SHARD_PREFIX`/`SHARD_MODE` + `shard_inspect_maybe` + `run_integration_shard`), Task 1's per-test timings.
- Produces: a tagged gitcli corpus whose every test carries a `TestIntegration…`/`TestRaceIntegration…` prefix and belongs to exactly one committed runner; map-file lines for every moved/renamed test.

- [ ] **Step 1: Classify every gitcli test against the tag criterion**

A test moves behind `integration` when it runs real Git subprocesses, constructs multi-repository/worktree workflows, exercises process timeouts, or drives a complete operation through those boundaries. Fake-backed and fast pure-orchestration tests stay untagged. Use Task 1's per-test timings as the tiebreaker: the partition exists to remove wall clock, so a sub-100ms test with a real repo may stay untagged if its family is fast — but a family with a broad slow tail (the spec names `readblobs`' ~14s malformed-batch test and the 2–4s process/repository tests) moves as a family. Homogeneous files move whole (`git mv file_test.go file_integration_test.go` then add the tag line); mixed files split — fast tests must not become integration tests because a sibling is slow.

- [ ] **Step 2: Move, tag, and rename**

For each moved file: first line becomes exactly `//go:build integration` followed by a blank line, then `package gitcli`. Rename each moved top-level test to `TestIntegration<Feature><Rest>` — feature names are stable behavioral areas, chosen so that **no runner prefix is a string prefix of another runner prefix** (the contract's exactly-one check reddens on overlap). Audit `internal/gitcli/concurrency_test.go` as the seed race candidate: a test qualifies for `TestRaceIntegration<Feature>` + a `*_race_integration_test.go` file only for shared mutable state, concurrent adapter calls, process lifecycle coordination, or a race/recovery protocol — and each such test gets a short adjacent comment naming that concurrent behavior. Sequential drivers stay `TestIntegration…` in normal shards; broad race coverage of sequential code is explicitly not wanted. Shared real-repository helpers used only by tagged tests move into a tagged helper file (e.g. `harness_integration_test.go`); no untagged test may reference a tagged helper — `go test ./internal/gitcli/` (default tags) failing to compile is the detector, and the fix is relocating the helper, never tagging unrelated fast tests. Append one map line per moved test: `internal/gitcli<TAB><old><TAB><new>`.

- [ ] **Step 3: Verify both tag states compile and the fast corpus is genuinely fast**

```bash
go vet ./internal/gitcli/ && go vet -tags integration ./internal/gitcli/
go test -count=1 ./internal/gitcli/                       # fast corpus: passes, and time it
go test -tags integration -count=1 ./internal/gitcli/     # whole tagged corpus passes
gofmt -l internal/gitcli/                                 # empty
```

- [ ] **Step 4: Measure and cut shards fresh, then write the runners**

```bash
go test -tags integration -count=1 -timeout 30m -v ./internal/gitcli/ 2>&1 | grep -E -e '^--- PASS'
```

Group by feature prefix; sum per group; draw cuts so each shard's standalone wall clock targets 45–50s (a race shard measured WITH `-race`). If one behavioral area exceeds the target, split into narrower named areas first; numeric suffixes only after natural boundaries are exhausted. Then write one runner per shard from this template (full file — the canonical assert byte-exact):

```bash
#!/usr/bin/env bash
# tests/test_go_integration_gitcli_batch.sh — Go integration shard (change 0333):
# the gitcli batch/blob protocol tests, behind the `integration` build tag, prefix
# ^TestIntegrationGitBatch. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gitcli"
SHARD_PREFIX="TestIntegrationGitBatch"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(Adapt the three literals and both header sentences per shard; a race shard sets `SHARD_MODE="race"` and a `TestRaceIntegration…` prefix.)

- [ ] **Step 5: Measure each runner standalone and add its budget row + total in the same commit**

```bash
for r in tests/test_go_integration_gitcli_*.sh; do
  /usr/bin/time -p bash "$r" >/dev/null   # three readings each, one session
done
```

Size each row per the table rule from the WORST reading; add `<path><TAB><row><TAB>parallel` rows (sorted into the table), a header note carrying the readings/command/machine context, and move `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` with a ledger entry ("NEW-FILE case, change 0333, gitcli integration shards"). Recompute the total from the table itself: `awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv`.

- [ ] **Step 6: Run the budget guard and the shards, then commit**

```bash
bash tests/test_runtime_budgets.sh
for r in tests/test_go_integration_gitcli_*.sh; do bash "$r" || echo "RED: $r"; done
git add internal/gitcli tests/test_go_integration_gitcli_*.sh tests/runtime-budgets.tsv \
        tests/test_runtime_budgets.sh docs/superpowers/plans/2026-08-27-0333-inventory-map.txt
git commit -m "test(0333): partition internal/gitcli behind the integration tag"
```

---

### Task 4: Partition `internal/githubcli`

**Files:**
- Modify/split: `internal/githubcli/` test files → tagged siblings
- Create: `tests/test_go_integration_githubcli_<feature>.sh` runners (starting hypothesis: merge/retarget behavior, PR ensure/comment behavior, discovery/probe protocols)
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)
- Append: `docs/superpowers/plans/2026-08-27-0333-inventory-map.txt`

**Interfaces:**
- Consumes: Task 2's runner protocol; Task 3's established file/renaming pattern (repeat it exactly — same tag line, same runner template, same map-line format `internal/githubcli<TAB><old><TAB><new>`).
- Produces: tagged githubcli corpus, its runners, its rows.

- [ ] **Step 1: Classify, move, tag, rename** — identical procedure to Task 3 Steps 1–2. Package-specific notes: the fake-`gh` protocol tests spawn real subprocesses; the spec's profiling found one ~20s merge test plus many 3–5s command-protocol tests, i.e. a broad tail — expect most of `merge_test.go`, `retarget_test.go`, `ensure_test.go`, `comment_test.go`, `pr_test.go`, `probe_test.go`, `repo_test.go` families to move, with `fakegh_test.go`'s harness helpers relocating to a tagged helper file if only tagged tests consume them (compile both tag states to prove which). Race candidates only where real concurrent adapter calls exist; do not manufacture any.
- [ ] **Step 2: Verify both tag states** — same four commands as Task 3 Step 3, against `./internal/githubcli/`.
- [ ] **Step 3: Measure, cut, write runners** — Task 3 Step 4's procedure and template with `SHARD_PKG="./internal/githubcli"`.
- [ ] **Step 4: Rows + total, budget guard, commit**

```bash
bash tests/test_runtime_budgets.sh
git add internal/githubcli tests/test_go_integration_githubcli_*.sh tests/runtime-budgets.tsv \
        tests/test_runtime_budgets.sh docs/superpowers/plans/2026-08-27-0333-inventory-map.txt
git commit -m "test(0333): partition internal/githubcli behind the integration tag"
```

---

### Task 5: Partition `internal/app` (the dominant package)

**Files:**
- Modify/split: `internal/app/` test files → tagged siblings (59 files, 435 tests; `finalize_e2e_test.go` untouched)
- Create: `tests/test_go_integration_app_<feature>.sh` runners (starting hypothesis: finalize cleanup/publish, finalize merge/rebase, planning/change operations, remaining real-repository workflows — split further wherever a measured group exceeds the target)
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)
- Append: `docs/superpowers/plans/2026-08-27-0333-inventory-map.txt`

**Interfaces:**
- Consumes: Task 2's runner protocol; Tasks 3–4's pattern.
- Produces: tagged app corpus, runners, rows; after this task ordinary `go test ./internal/app/` must finish comfortably inside Go's default 10m deadline with real margin.

- [ ] **Step 1: Classify by the file-name seams first, then verify per test.** The `*_git_test.go` files (`change_attach_git_test.go`, `claim_workflow_git_test.go`, `finalize_git_test.go`, `link_context_git_test.go`, `planning_git_test.go`, `status_git_test.go`, `workspace_ops_git_test.go`), `workflow_e2e_test.go` (an untagged file today despite its name), and the finalize merge/rebase/publish/cleanup/retarget families are the expected movers; pure-fake orchestration files (`config_test.go`, `result_test.go`, `version_test.go`, the `rungate_*` in-process tests, etc.) are expected stayers — but the classification criterion and Task 1's timings decide, not the file name. Mixed files split. Every scenario keeps its assertions byte-identical.
- [ ] **Step 2: Move, tag, rename; relocate shared real-repo helpers** into tagged helper files where only tagged tests consume them. Prove helper placement by compiling both tag states — a default-tag compile failure means a helper moved that an untagged test still needs; relocate the helper back or split it, never tag the fast test.
- [ ] **Step 3: Verify both tag states, and time the fast corpus**

```bash
go vet ./internal/app/ && go vet -tags integration ./internal/app/
/usr/bin/time -p go test -count=1 ./internal/app/           # the number that unblocks 0357
go test -tags integration -count=1 -timeout 30m ./internal/app/
gofmt -l internal/app/
```

- [ ] **Step 4: Measure, cut, write runners** — Task 3 Step 4's procedure and template with `SHARD_PKG="./internal/app"`. Expect several shards (~190s of integration cost against a 45–50s target ⇒ roughly four to five normal shards, measurement decides). Race shards only for audited concurrency-bearing tests, each with its adjacent rationale comment.
- [ ] **Step 5: Rows + total, budget guard, all shards standalone, commit**

```bash
bash tests/test_runtime_budgets.sh
for r in tests/test_go_integration_app_*.sh; do bash "$r" || echo "RED: $r"; done
git add internal/app tests/test_go_integration_app_*.sh tests/runtime-budgets.tsv \
        tests/test_runtime_budgets.sh docs/superpowers/plans/2026-08-27-0333-inventory-map.txt
git commit -m "test(0333): partition internal/app behind the integration tag"
```

---

### Task 6: The fail-closed integration contract

**Files:**
- Create: `tests/test_go_integration_contract.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces:**
- Consumes: the inspection protocol (`DOCKET_SHARD_INSPECT=1` → `package=`/`prefix=`/`mode=` lines) from Task 2; the live runner files from Tasks 3–5.
- Produces: the eight-check contract the spec's Completeness section enumerates. Task 8 mutation-proves it.

- [ ] **Step 1: Write the contract test.** Skeleton (complete the loops verbatim; every check fails closed — a probe error is never clean absence):

```bash
#!/usr/bin/env bash
# tests/test_go_integration_contract.sh — the fail-closed completeness contract over
# the Go integration partition (change 0333). Discovery is live: tagged tests come
# from `go test -tags integration -list` and shard membership from each runner's
# own inspection mode (DOCKET_SHARD_INSPECT=1) — never a second registry. Checks:
#   (1) every *_integration_test.go / *_race_integration_test.go in the three
#       packages opens with `//go:build integration` on line 1
#   (2) the tagged corpus lists cleanly per package and is non-empty overall
#   (3) every discovered runner inspects to a well-formed declaration
#   (4) every tagged test matches EXACTLY ONE runner (same package, name-prefix)
#   (5) TestRaceIntegration… tests match a race runner and TestIntegration… tests
#       never do — both directions (learning correspondence-guard-runs-one-way)
#   (6) no integration-prefixed test is visible to the default-tag corpus
#   (7) every runner selects at least one test (a stale runner cannot no-op)
#   (8) go vet -tags integration passes for all three packages
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
command -v go >/dev/null 2>&1 || { printf 'NOT OK - the contract cannot certify anything without a Go toolchain\n'; exit 1; }
export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modcacherw"
# … the canonical GOMODCACHE/GOCACHE pin block from tests/test_go_toolchain.sh …

PKGS="internal/app internal/githubcli internal/gitcli"

# (1) build-constraint placement — first line, exactly.
bad_tag=""
for pkg in $PKGS; do
  for f in "$pkg"/*_integration_test.go; do
    [ -e "$f" ] || continue
    [ "$(sed -n '1p' "$f")" = "//go:build integration" ] || bad_tag="$bad_tag $f"
  done
done
assert "every *_integration_test.go opens with //go:build integration" '[ -z "$bad_tag" ] || { echo "  missing/misplaced tag:$bad_tag" >&2; false; }'

# (2) tagged corpus, per package; listing or compile failure is fatal; empty corpus fatal.
tagged=""            # lines: <pkg><TAB><TestName>
for pkg in $PKGS; do
  out="$(go test -tags integration -list '^Test' "./$pkg" 2>&1)"; rc=$?
  assert "go test -tags integration -list succeeds for $pkg" '[ "$rc" -eq 0 ] || { printf "%s\n" "$out" >&2; false; }'
  names="$(grep -E -e '^Test(Race)?Integration' <<<"$out" || true)"
  while read -r t; do [ -n "$t" ] && tagged="$tagged$pkg	$t
"; done <<<"$names"
done
assert "the tagged corpus is non-empty" '[ -n "$tagged" ]'
# Also fail closed on tagged tests that carry NEITHER structural prefix:
stray="$(for pkg in $PKGS; do go test -tags integration -list '^Test' "./$pkg" 2>/dev/null \
  | grep -E -e '^Test' | grep -vE -e '^Test(Race)?Integration' | grep -vE -e '^TestE2E' \
  | while read -r t; do
      # only tests DEFINED in tagged files are in scope; a test also listed by the
      # default corpus is untagged and handled by (6)'s complement
      go test -list "^${t}\$" "./$pkg" 2>/dev/null | grep -qxF "$t" || echo "$pkg:$t"
    done; done)"
assert "every tagged-only test carries a TestIntegration/TestRaceIntegration prefix" \
  '[ -z "$stray" ] || { echo "  unprefixed tagged tests: $stray" >&2; false; }'

# (3) runner discovery + inspection. The contract excludes itself by exact name.
runners="$(find tests -maxdepth 1 -name 'test_go_integration_*.sh' ! -name 'test_go_integration_contract.sh' | LC_ALL=C sort)"
assert "at least one shard runner exists" '[ -n "$runners" ]'
decl=""              # lines: <runner><TAB><pkg-dir></><TAB stripped>…
bad_decl=""
while read -r r; do
  [ -n "$r" ] || continue
  out="$(DOCKET_SHARD_INSPECT=1 bash "$r" 2>&1)"; rc=$?
  p="$(sed -n 's/^package=//p' <<<"$out")"; x="$(sed -n 's/^prefix=//p' <<<"$out")"; m="$(sed -n 's/^mode=//p' <<<"$out")"
  ok=1
  [ "$rc" -eq 0 ] || ok=0
  case "$p" in ./internal/app|./internal/githubcli|./internal/gitcli) ;; *) ok=0 ;; esac
  case "$m" in normal|race) ;; *) ok=0 ;; esac
  case "$x" in TestIntegration?*|TestRaceIntegration?*) ;; *) ok=0 ;; esac
  [ "$ok" = 1 ] && decl="$decl$r	${p#./}	$x	$m
" || bad_decl="$bad_decl $r"
done <<<"$runners"
assert "every runner inspects to a well-formed declaration" '[ -z "$bad_decl" ] || { echo "  malformed:$bad_decl" >&2; false; }'

# (4)+(5)+(7): one matching pass over tagged × declarations.
unmatched=""; multi=""; wrongmode=""
while IFS=$'\t' read -r pkg t; do
  [ -n "$t" ] || continue
  hits=0; hitmode=""
  while IFS=$'\t' read -r r rp rx rm; do
    [ -n "$r" ] || continue
    [ "$rp" = "$pkg" ] || continue
    case "$t" in "$rx"*) hits=$((hits+1)); hitmode="$rm";; esac
  done <<<"$decl"
  [ "$hits" -eq 0 ] && unmatched="$unmatched $pkg:$t"
  [ "$hits" -gt 1 ] && multi="$multi $pkg:$t"
  case "$t" in
    TestRaceIntegration*) [ "$hits" -eq 1 ] && [ "$hitmode" != "race" ]   && wrongmode="$wrongmode $pkg:$t(want race)";;
    TestIntegration*)     [ "$hits" -eq 1 ] && [ "$hitmode" != "normal" ] && wrongmode="$wrongmode $pkg:$t(want normal)";;
  esac
done <<<"$tagged"
assert "every tagged test matches exactly one runner (none unmatched)" '[ -z "$unmatched" ] || { echo " $unmatched" >&2; false; }'
assert "every tagged test matches exactly one runner (none doubled)"   '[ -z "$multi" ]     || { echo " $multi" >&2; false; }'
assert "race-prefixed tests run in race shards, and only they do"      '[ -z "$wrongmode" ] || { echo " $wrongmode" >&2; false; }'
# (7) reverse direction: every runner selects at least one tagged test.
empty_runners=""
while IFS=$'\t' read -r r rp rx rm; do
  [ -n "$r" ] || continue
  n="$(grep -c -E -e "^${rp}	${rx}" <<<"$tagged")"
  [ "$n" -ge 1 ] || empty_runners="$empty_runners $r"
done <<<"$decl"
assert "no runner is a stale empty no-op" '[ -z "$empty_runners" ] || { echo " $empty_runners" >&2; false; }'

# (6) the default corpus must not see any integration-prefixed test.
leak=""
for pkg in $PKGS; do
  out="$(go test -list '^Test(Race)?Integration' "./$pkg" 2>&1)"; rc=$?
  assert "go test -list (default tags) succeeds for $pkg" '[ "$rc" -eq 0 ] || { printf "%s\n" "$out" >&2; false; }'
  l="$(grep -E -e '^Test(Race)?Integration' <<<"$out" || true)"
  [ -z "$l" ] || leak="$leak $pkg:{$l}"
done
assert "no integration-prefixed test is visible to the default-tag corpus" '[ -z "$leak" ] || { echo " $leak" >&2; false; }'

# (8) tagged static analysis — default `go vet ./...` cannot see this corpus.
vet_out="$(go vet -tags integration ./internal/app/ ./internal/githubcli/ ./internal/gitcli/ 2>&1)"
vet_rc=$?
assert "go vet -tags integration passes for all three packages" '[ "$vet_rc" -eq 0 ] || { printf "%s\n" "$vet_out" >&2; false; }'

exit "$fail"
```

Fill in the elided cache-pin block by copying it verbatim from `tests/test_go_toolchain.sh`. If the `(2)` stray-prefix sub-check proves too slow (it runs one `-list` per tagged test), replace it with one default-tag `-list '^Test'` per package captured once and set-subtracted — the property to keep is: a tagged test with neither structural prefix reddens.

- [ ] **Step 2: Run it green, then measure and add its row**

```bash
bash tests/test_go_integration_contract.sh; echo "rc=$?"
for i in 1 2 3; do /usr/bin/time -p bash tests/test_go_integration_contract.sh >/dev/null; done
```

Size the row from the worst reading (the vet-with-tags compile is the dominant term; a fresh-clone cold-cache reading is the reachable worst — take one with `GOCACHE`/`GOMODCACHE` pointed at empty scratch dirs, the drift-test methodology). Add the row + header note + `EXPECTED_TOTAL` ledger entry.

- [ ] **Step 3: Commit**

```bash
bash tests/test_runtime_budgets.sh
git add tests/test_go_integration_contract.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0333): fail-closed integration completeness contract"
```

---

### Task 7: Retire the 300s exemption; race gate back to the parallel lane

**Files:**
- Modify: `tests/test_go_race.sh` (header only — the command stays `go test -race -count=1 ./...`)
- Modify: `tests/runtime-budgets.tsv` (race row re-cut serial→parallel; toolchain row re-measured; header exemption prose deleted; ledger entry)
- Modify: `tests/test_runtime_budgets.sh` (delete `RACE_GATE`, `RACE_CEILING`, assertion (4b), the exemption prose in (3); `EXPECTED_SERIAL` 1→0; `EXPECTED_TOTAL` re-derived)

**Interfaces:**
- Consumes: the partitioned fast corpus from Tasks 3–5 (this task's measurements are only meaningful after them).
- Produces: a budget table with no row above 60, zero serial rows, and a pinned total matching the awk recomputation — the state Task 8's mutations and the acceptance criteria check.

- [ ] **Step 1: Measure the two whole-module gates fresh over the fast corpus**

```bash
for i in 1 2 3; do /usr/bin/time -p bash tests/test_go_race.sh      >/dev/null; done
for i in 1 2 3; do /usr/bin/time -p bash tests/test_go_toolchain.sh >/dev/null; done
```

Size both rows from worst readings per the table rule. The race row MUST land ≤ 60 in the parallel lane; if the worst reading sizes above 60, the partition cut is not deep enough — go back to the owning package task's classification and move more of the measured tail behind the tag (Global Constraints: never raise a budget to absorb a slow shard, and a ceiling exemption is not a fallback path). `tests/test_go_toolchain.sh` was re-shaped by the same exclusion, so re-cut its row from its own fresh worst reading (the re-cut case may lower it; record the readings either way).

- [ ] **Step 2: Edit `tests/runtime-budgets.tsv`**

Row: `tests/test_go_race.sh<TAB><measured row><TAB>parallel`. Header: delete the sentence beginning "NO row may exceed 60 seconds — with ONE documented exemption" and restore the plain "NO row may exceed 60 seconds." rule; rewrite the `tests/test_go_race.sh` header note to record change 0333's re-cut (partition landed, fast-corpus readings, sizing); add the ledger entry naming both movers and the arithmetic. Recompute the sum with the header's own awk line.

- [ ] **Step 3: Edit `tests/test_runtime_budgets.sh`**

- `EXPECTED_SERIAL=1` → `EXPECTED_SERIAL=0`, replacing the comment's 0332 justification with: no file is currently pinned serial; the race gate returned to the parallel lane when change 0333 partitioned the slow corpus out of `go test -race ./...`; raising this is a finding that must name the shared state forcing the pin in the same diff. Assertion (4) itself stays — it is the guard that a future serial pin is a conscious, counted decision.
- In assertion (3): delete `RACE_GATE=`/`RACE_CEILING=` and the "ONE DOCUMENTED EXEMPTION" comment block; simplify the awk back to `'$2 > c {print $1}'` and the assert label to `"no budget row exceeds the ${CEILING}s ceiling"`.
- Delete assertion (4b) entirely (its premise — an exemption to serial-bind — no longer exists).
- Re-derive `EXPECTED_TOTAL` from the table and prepend the ledger entry (state which rows moved and why: exemption re-cut −240±, toolchain re-cut, plus the shard/contract rows Tasks 3–6 already ledgered).

- [ ] **Step 4: Rewrite `tests/test_go_race.sh`'s header**

Keep: what the gate is, `-count=1` rationale, repo-wide `./...` shape argument, the not-a-fifth-check and does-not-replace-the-plain-run paragraphs, toolchain fail-loud stance, canonical-assert note, CACHES block. Delete: the LANE AND CEILING paragraph, the 300s/exemption/serial prose, and the "owned by follow-up change 0333" sentences. Add one paragraph: change 0333 partitioned the slow integration corpus of `internal/app`, `internal/githubcli`, and `internal/gitcli` behind the `integration` build tag (dedicated shard runners own it; `tests/test_go_integration_contract.sh` proves completeness), so this gate covers the fast default corpus and rides the parallel lane under an ordinary row.

- [ ] **Step 5: Restatement sweep (learnings `restatement-accumulates-its-own-guards`)**

```bash
git grep -nE -e 'RACE_GATE|RACE_CEILING|RELIEF COUNTER A|300s|exemption' -- \
  ':!docs/changes/archive' ':!docs/superpowers/specs' ':!docs/superpowers/plans' ':!docs/adrs'
git grep -nF -e 'test_go_race' -- tests scripts skills README.md
```

Sort every hit into prose vs executable (AGENTS.md rule: only executable sites can violate a gate, but stale maintained prose is a defect too). Known owners already edited in Steps 2–4; fix any survivor in maintained source (candidates: `tests/README.md`, `scripts/run-tests.md` if either narrates the exemption or the serial lane's sole occupant). Point-in-time records stay untouched. If any test greps wording this task deletes, repoint that assert at the surviving owner — never re-add deleted text to keep a grep green.

- [ ] **Step 6: Verify and commit**

```bash
bash tests/test_runtime_budgets.sh          # green, with 0 serial rows and the new total
bash tests/test_go_race.sh                  # green over the fast corpus
bash tests/test_go_toolchain.sh             # green
git add tests/test_go_race.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh \
        $(git diff --name-only -- tests scripts skills README.md)
git commit -m "test(0333): retire the race gate's 300s exemption and serial pin"
```

---

### Task 8: Mutation evidence, migration fidelity, and the full-suite gate

**Files:**
- No new committed source (evidence recorded per docket-build's build-evidence contract; results file cites it)

**Interfaces:**
- Consumes: `tests/test_go_integration_contract.sh` (Task 6), the inventory files (Tasks 1, 3–5), the whole partitioned tree.
- Produces: the four mutation outcomes, the before/after fidelity proof, the post-partition timing record, and a green full-suite run — the acceptance evidence the results file reports.

- [ ] **Step 1: Mutation 1 — remove one build tag**

```bash
f=$(ls internal/gitcli/*_integration_test.go | head -1)   # any tagged file
cp "$f" "$f.bak"
sed -i '' '1s|^//go:build integration$||' "$f"
git diff -- "$f" | head            # PROVE the mutation landed before reading any result
bash tests/test_go_integration_contract.sh; echo "rc=$?"
# Expected: rc=1, reddening BOTH check (1) ("opens with //go:build integration")
# and check (6) (the now-untagged tests leak into the default corpus) — record which fired.
mv "$f.bak" "$f"
```

- [ ] **Step 2: Mutation 2 — delete one runner**

```bash
r=$(find tests -maxdepth 1 -name 'test_go_integration_*.sh' ! -name 'test_go_integration_contract.sh' | head -1)
mv "$r" "$r.bak"
bash tests/test_go_integration_contract.sh; echo "rc=$?"
# Expected: rc=1 on check (4) "none unmatched" — that runner's tests now match zero runners.
mv "$r.bak" "$r"
```

- [ ] **Step 3: Mutation 3 — duplicate one runner prefix**

```bash
cp "$r" tests/test_go_integration_zzz_dup.sh
bash tests/test_go_integration_contract.sh; echo "rc=$?"
# Expected: rc=1 on check (4) "none doubled" — every test of that shard now matches two runners.
rm tests/test_go_integration_zzz_dup.sh
```

(The duplicate also lacks a budget row — that is `tests/test_runtime_budgets.sh`'s check, not the contract's; the contract must redden on its own.)

- [ ] **Step 4: Mutation 4 — flip a race runner to normal mode**

```bash
rr=$(grep -l -E -e '^SHARD_MODE="race"$' tests/test_go_integration_*.sh | head -1)
cp "$rr" "$rr.bak"
sed -i '' 's/^SHARD_MODE="race"$/SHARD_MODE="normal"/' "$rr"
git diff -- "$rr" | head           # mutation landed
bash tests/test_go_integration_contract.sh; echo "rc=$?"
# Expected: rc=1 on check (5) — its TestRaceIntegration tests now match a normal-mode runner.
mv "$rr.bak" "$rr"
```

If no race shard exists because no test survived the concurrency audit, record that explicitly in the evidence and substitute the reverse probe: add a temporary `TestRaceIntegrationProbe` test to a tagged file with no race runner and watch check (4)/(5) redden — the race-mode direction of the guard must be proven live either way, then the probe is removed.

- [ ] **Step 5: Migration fidelity — the before/after mapping closes**

```bash
cd "$(git rev-parse --show-toplevel)"
after=$(mktemp "${TMPDIR:-/tmp}/0333-after.XXXXXX")
for pkg in internal/app internal/githubcli internal/gitcli; do
  for tags in "" "-tags e2e" "-tags integration"; do
    go test $tags -list '^Test' "./$pkg" | grep -E -e '^Test' \
      | while read -r t; do printf '%s\t%s\n' "$pkg" "$t"; done
  done
done | LC_ALL=C sort -u > "$after"
# Apply the map to the before-snapshot and diff: the result must be EXACTLY the after set.
awk -F'\t' 'NR==FNR && !/^#/ {map[$1 FS $2]=$1 FS $3; next}
            { print ($0 in map) ? map[$0] : $0 }' \
    docs/superpowers/plans/2026-08-27-0333-inventory-map.txt \
    docs/superpowers/plans/2026-08-27-0333-inventory-before.txt \
  | LC_ALL=C sort -u | diff - "$after"
# Expected: empty diff — every original scenario survives exactly once, no phantom additions
# beyond tests the map names. Any non-empty diff is a blocking finding: a lost or duplicated scenario.
```

- [ ] **Step 6: Post-partition timing record**

Re-run Task 1 Step 2's six package measurements plus every shard runner standalone; record before/after side by side in the evidence. Green correctness with unchanged-or-worse wall clock on the default/race package runs is a FAILED performance outcome (the spec's Migration-integrity clause; learnings `optimization-needs-a-measured-oracle`) — the partition exists to move wall clock, and the numbers are the oracle.

- [ ] **Step 7: The full-suite gate**

Resolve the suite command from `finalize.test_command` (never a second copy) and run it whole. Read the budget report: `SERIAL CONFIRMED OVER BUDGET:` lines are authoritative breaches to fix before proceeding (a new shard breaching means re-cutting it, not re-budgeting); `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines are screening findings to record with the `-j` level. Record every new row's remaining margin as a NUMBER in the evidence (learnings `budget-headroom-is-spent-before-it-is-breached`), and note that change 0357's halted gate is the first consumer of the fast `internal/app`.

- [ ] **Step 8: Commit any evidence-file updates**

```bash
git add -u && git commit -m "test(0333): mutation evidence, migration fidelity, suite gate" || true
```

(Only if the evidence record lives in a committed file per docket-build's convention; otherwise nothing to commit.)

---

## Self-Review

- **Spec coverage:** one structural boundary → Tasks 3–5; structural names/feature shards → Tasks 3–5; targeted race tests with rationale comments → Tasks 3–5 Step 2; declarative runners + shared helper + inspection → Task 2; eight completeness checks → Task 6 (checks 1–8 all present in the skeleton); four mutation cases → Task 8 Steps 1–4; budget/race-gate transition incl. `RACE_GATE`/`RACE_CEILING`/(4b)/`EXPECTED_SERIAL`/`EXPECTED_TOTAL` → Task 7; `go vet -tags integration` → Task 6 check 8 (plus per-package verification in Tasks 3–5); migration inventory → Tasks 1 + 8 Step 5; fresh-measurement mandate and no-raise rule → Global Constraints + Task 7 Step 1's explicit back-to-the-cut remedy; error-handling section (fail-loud toolchain, replayed output, no empty-success) → Task 2's helper; acceptance 1–10 → Tasks 3–8 verifications plus Task 8's evidence.
- **Placeholder scan:** shard counts and row values are deliberately measurement outputs, per the spec's own "build-time measurement outputs, not frozen estimates" — the procedures to produce them are fully specified.
- **Type consistency:** `SHARD_PKG`/`SHARD_PREFIX`/`SHARD_MODE`, `shard_inspect_maybe`, `run_integration_shard`, the `package=`/`prefix=`/`mode=` inspection lines, and the two inventory-file formats are used identically across Tasks 2–8.
