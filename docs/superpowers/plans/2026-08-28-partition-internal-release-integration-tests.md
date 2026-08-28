<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0362 — Partition internal/release integration tests to clear the race-gate per-package timeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-28-0362-partition-internal-release-to-clear-the-race-gate-timeout.md)**
<!-- docket:backlink:end -->
# Partition internal/release Integration Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `internal/release`'s slow real-tar/gzip/subprocess test corpus (`package_test.go`, `archive_test.go`) behind the `integration` build tag with one sequential non-race shard runner, generalize change 0333's fail-closed partition contract from a package allowlist to structural discovery, and re-measure every affected runtime row from a fresh Go build cache under the 60s ceiling — so the default `go test -race ./...` gate stops breaching Go's 600s per-package timeout under parallel suite load.

**Architecture:** Extends the change-0333 partition machinery already in the tree: shard runners are 3-literal declaration files delegating to `tests/lib/go-integration-shard.sh` (which owns inspection mode, `-count=1` execution, and the single-source race flag), and `tests/test_go_integration_contract.sh` proves the partition total. This change adds the `./internal/release` shard, replaces the contract's three hand-enumerated package lists with discovery from repository syntax (`*_integration_test.go` files) and live runner inspection, and adds a live fidelity-floor test keyed on a committed before/after name map — the one deletion mode structural discovery cannot see (tagged files AND runner removed together).

**Tech Stack:** Go (toolchain pinned by go.mod), bash test files under the repo's canonical byte-exact `assert` helper (`scripts/check-test-source-hygiene.sh` rule (a)), `tests/runtime-budgets.tsv` + `tests/test_runtime_budgets.sh` budget discipline, `scripts/run-tests.sh` parallel suite.

**Spec:** `docs/superpowers/specs/2026-08-28-partition-internal-release-integration-tests-design.md` (on the `docket` metadata branch; synchronized copy readable at `.docket/docs/superpowers/specs/…` from the primary checkout).

## Global Constraints

- **No production changes** under `internal/release`: `render.go` embeds, packaging/checksum logic, `.github/workflows/release-candidate.yml`, `scripts/release-smoke.sh` are untouched. Only `*_test.go` files move/rename.
- **Scenario fidelity:** apart from the build constraint, file relocation, and top-level test rename, scenario logic and assertions are byte-unchanged. Every original test survives exactly once.
- **Build-constraint shape:** line 1 exactly `//go:build integration`, line 2 blank (Go ignores the constraint otherwise — the contract's check (1) asserts both).
- **Naming:** moved package tests → `TestIntegrationReleasePackage...`; moved archive tests → `TestIntegrationReleaseArchive...`. Runner prefix `TestIntegrationRelease`, mode `normal` (no `-race`): grooming found no goroutines, sync primitives, `t.Parallel()`, or process-lifecycle coordination in these files — re-verify at Task 2/3 and STOP (escalate, do not improvise a race shard) if that is no longer true.
- **No allowlists:** the generalized contract discovers packages and runners structurally; it must not carry a hand-maintained package list in any check. Same for any package split: bidirectional exact cover against live `go list ./...`.
- **Budget rows:** target 45–50s cold; 51–60s ⇒ split per the spec's adaptive order; >60s ⇒ design failure, STOP and report (never raise a timeout or accept a >60 row). Row value = next 5s boundary above the worst valid reading, +5s headroom, minimum 10, never above 60. `EXPECTED_SERIAL` stays 0 (no serial pin is planned; a pin would be a finding).
- **Mutation discipline (CLAUDE.md + learnings):** every new/changed guard is mutation-tested — one isolated break at a time; prove the mutation landed (`grep -c` before/after); defeat Go's test cache (`-count=1`, and `-list` probes re-run fresh) where execution occurs; restore from a `cp` backup (`cp f f.bak; mutate; run; mv f.bak f`) — never `git checkout --` (learning mutation-restore-needs-a-backup-copy).
- **Shell portability:** re-check every new grep regex under `/usr/bin/grep` (PATH grep is ugrep and is more permissive); patterns that can lead with `-` use `grep -E -e "<pat>"`; never `producer | grep -q` under `set -o pipefail` (capture first, then `grep <<<"$var"`); awk indent classes are `[^[:space:]]`; comments anchor on symbol names or verbatim-quoted clauses, never line numbers.
- **Assert helper:** new test files copy the tree's canonical helper byte for byte: `assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }`
- **Evidence:** all three readings per affected runner, the chosen worst reading, the row arithmetic, mutation-run receipts, and the final parallel-suite `SUITE` line are recorded in the build-evidence record. Measured wall clock is part of acceptance (learning optimization-needs-a-measured-oracle: correctness asserts cannot judge this change).

## File Map

```
internal/release/package_integration_test.go        RENAMED from package_test.go  (Task 3)
internal/release/archive_integration_test.go        RENAMED from archive_test.go  (Task 2)
internal/release/{render,version,checksums}_test.go UNCHANGED (default corpus)
tests/test_go_integration_release.sh                NEW shard runner              (Task 4)
tests/test_go_integration_contract.sh               GENERALIZED, all 10 checks    (Task 4)
tests/test_release_partition_fidelity.sh            NEW fidelity floor            (Task 5)
tests/fixtures/release-partition-map.tsv            NEW committed old→new map     (Task 1)
docs/superpowers/plans/2026-08-28-0362-inventory-before.txt  NEW snapshot         (Task 1)
tests/runtime-budgets.tsv                           rows + commentary             (Tasks 4,5,7)
tests/test_runtime_budgets.sh                       EXPECTED_TOTAL                (Task 7)
```

Intermediate states are deliberately buildable (learning intermediate-task-state-buildable): after Tasks 2–3 the OLD contract stays green because its `PKGS` list does not include `internal/release` (it is blind to the new tagged files, not broken by them); Task 4 lands the runner and the generalized contract in ONE commit because each without the other reddens a contract check (a runner declaring `./internal/release` fails the old allowlist; a generalized contract without the runner fails exact-one-runner coverage).

---

### Task 1: Baseline inventory snapshot and the committed name map

**Files:**
- Create: `docs/superpowers/plans/2026-08-28-0362-inventory-before.txt`
- Create: `tests/fixtures/release-partition-map.tsv`

**Interfaces:**
- Produces: the before snapshot, one line per test `internal/release<TAB><TestName>`, sorted `LC_ALL=C`, captured under BOTH tag states (they are identical today — nothing in `internal/release` is tagged yet; the snapshot proves that). The map, format `<old-name><TAB><new-name><TAB><corpus>` where corpus ∈ `integration|default`, TAB-separated, one row per top-level test. Task 5's fidelity test and Task 6's final fidelity proof consume the map; the snapshot is a point-in-time record.

- [ ] **Step 1: Snapshot the before inventory (both tag states)**

```bash
cd "$(git rev-parse --show-toplevel)"
out=docs/superpowers/plans/2026-08-28-0362-inventory-before.txt
{
  echo "# internal/release top-level test inventory BEFORE change 0362 (point-in-time record)"
  echo "# default tags:"
  go test -list '^Test' ./internal/release/ | grep -E -e '^Test' | LC_ALL=C sort
  echo "# -tags integration:"
  go test -tags integration -list '^Test' ./internal/release/ | grep -E -e '^Test' | LC_ALL=C sort
} > "$out"
```

Verify both listings exited 0 (run each `go test -list` once standalone first and check `$?`; a listing failure is a red result, not an empty corpus — learning probe-error-is-not-clean-absence) and that both sections are identical and contain exactly these 37 names: 5 archive (`TestWriteArchiveDeterministic`, `TestWriteArchiveEpochEntersStream`, `TestVerifyArchiveRoundTrip`, `TestWriteArchiveNoHostLeakage`, `TestVerifyArchiveRefusals`), 4 package (`TestPackageIntegration`, `TestPackageDeterministic`, `TestPackageRefusesCollision`, `TestPackageBundleValidatesChecksums`), 14 checksums, 4 render, 10 version. If the count or the set differs, the tree moved since grooming: STOP and report the delta instead of editing the plan's numbers silently. (Reconciled 2026-08-28 by docket-implement-next: the counts were refreshed from the live tree at build time — grooming's 38/13/12 became 37/14/10, a net −1 in the default corpus only, version −2 + checksums +1; the 9 tagged integration names are unchanged, so Tasks 2–4 are unaffected.)

- [ ] **Step 2: Write the committed map fixture**

`tests/fixtures/release-partition-map.tsv` (TAB-separated; `#` comment lines allowed):

```
# release partition map (change 0362): <old-name><TAB><new-name><TAB><corpus>
# corpus=integration rows moved behind //go:build integration; corpus=default rows stayed.
# Point-in-time record of the 0362 rename, AND the live fidelity floor's fixture:
# tests/test_release_partition_fidelity.sh asserts every new-name is visible in its corpus.
TestWriteArchiveDeterministic	TestIntegrationReleaseArchiveWriteDeterministic	integration
TestWriteArchiveEpochEntersStream	TestIntegrationReleaseArchiveWriteEpochEntersStream	integration
TestVerifyArchiveRoundTrip	TestIntegrationReleaseArchiveVerifyRoundTrip	integration
TestWriteArchiveNoHostLeakage	TestIntegrationReleaseArchiveWriteNoHostLeakage	integration
TestVerifyArchiveRefusals	TestIntegrationReleaseArchiveVerifyRefusals	integration
TestPackageIntegration	TestIntegrationReleasePackageEndToEnd	integration
TestPackageDeterministic	TestIntegrationReleasePackageDeterministic	integration
TestPackageRefusesCollision	TestIntegrationReleasePackageRefusesCollision	integration
TestPackageBundleValidatesChecksums	TestIntegrationReleasePackageBundleValidatesChecksums	integration
```

…followed by one `…<TAB>same-name<TAB>default` row for EVERY test in checksums_test.go, render_test.go, and version_test.go (28 rows, old-name = new-name), generated from the Step 1 snapshot, e.g.:

```bash
go test -list '^Test' ./internal/release/ | grep -E -e '^Test' | LC_ALL=C sort \
  | grep -vE -e '^Test(Write|Verify)Archive|^TestPackage' \
  | awk -F'\t' '{printf "%s\t%s\tdefault\n", $1, $1}' >> tests/fixtures/release-partition-map.tsv
```

Verify: map has exactly 37 non-comment rows; 9 `integration`, 28 `default`; every old-name appears exactly once (`cut -f1 | sort | uniq -d` is empty). Note the map's old-name column is what makes deleting-both-halves detectable later — it is the population floor's population (learning frozen-copy-needs-a-drift-assert: a committed copy asserts nothing until a live check reads it; Task 5 wires that check).

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/plans/2026-08-28-0362-inventory-before.txt tests/fixtures/release-partition-map.tsv
git commit -m "test(0362): baseline release test inventory and partition name map"
```

---

### Task 2: Move archive_test.go behind the integration tag

**Files:**
- Rename: `internal/release/archive_test.go` → `internal/release/archive_integration_test.go`

**Interfaces:**
- Produces: the 5 renamed `TestIntegrationReleaseArchive…` tests (exact names in the Task 1 map), tagged-only. Helpers `fileSHA256`, `readSingleHeader`, `writeGzTar`, `writeReg` are used only inside this file (verified during planning) and move with it unchanged.

- [ ] **Step 1: Re-verify the no-concurrency premise**

```bash
grep -n -E 'go func|sync\.|t\.Parallel|chan |os/signal' internal/release/archive_test.go
```

Expected: no matches (comment/prose matches are fine — read any hit). A real concurrent protocol here means the sequential-only design premise fell: STOP and report; do not invent a race shard.

- [ ] **Step 2: git mv and add the build constraint**

```bash
git mv internal/release/archive_test.go internal/release/archive_integration_test.go
```

Insert at the very top of the file (before the existing package comment/clause), as lines 1–2:

```go
//go:build integration

```

(line 1 exactly `//go:build integration`, line 2 empty.)

- [ ] **Step 3: Rename the 5 top-level tests**

Apply exactly the Task 1 map's archive rows — edit each `func TestX(t *testing.T)` declaration only; no call sites exist (top-level tests are not called by name) and no other file references these names (verified during planning by whole-repo grep):

- `TestWriteArchiveDeterministic` → `TestIntegrationReleaseArchiveWriteDeterministic`
- `TestWriteArchiveEpochEntersStream` → `TestIntegrationReleaseArchiveWriteEpochEntersStream`
- `TestVerifyArchiveRoundTrip` → `TestIntegrationReleaseArchiveVerifyRoundTrip`
- `TestWriteArchiveNoHostLeakage` → `TestIntegrationReleaseArchiveWriteNoHostLeakage`
- `TestVerifyArchiveRefusals` → `TestIntegrationReleaseArchiveVerifyRefusals`

- [ ] **Step 4: Verify the partition took, in both directions, and fidelity of content**

```bash
gofmt -l internal/release/
go vet -tags integration ./internal/release/
go test -count=1 -list '^Test' ./internal/release/ | grep -E -e 'Archive'
go test -tags integration -count=1 -list '^Test(Race)?Integration' ./internal/release/
go test -tags integration -count=1 -run '^TestIntegrationReleaseArchive' -v ./internal/release/
git diff HEAD -M --stat && git diff HEAD -M -- internal/release/
```

Expected: gofmt/vet clean; default-tags list shows NO archive tests; tagged list shows exactly the 5 new names; the tagged run passes with 5 `--- PASS:` lines; the `-M` diff shows a rename whose content hunks are ONLY the 2 constraint lines and the 5 `func` lines (scenario fidelity — anything else is a defect).

- [ ] **Step 5: Commit**

```bash
git add -A internal/release/
git commit -m "test(0362): move archive tests behind the integration tag as TestIntegrationReleaseArchive..."
```

---

### Task 3: Move package_test.go behind the integration tag

**Files:**
- Rename: `internal/release/package_test.go` → `internal/release/package_integration_test.go`

**Interfaces:**
- Produces: the 4 renamed `TestIntegrationReleasePackage…` tests (exact names in the Task 1 map), tagged-only. Helpers `repoRoot`, `itInputs`, `extractMember` are file-local (verified during planning) and move unchanged.

- [ ] **Step 1: Re-verify the no-concurrency premise**

```bash
grep -n -E 'go func|sync\.|t\.Parallel|chan |os/signal' internal/release/package_test.go
```

Expected: no real matches (this file runs `exec.Command` subprocesses SEQUENTIALLY — subprocess use alone does not qualify as a concurrent protocol, per the spec's non-goals). A genuine concurrent protocol ⇒ STOP and report.

- [ ] **Step 2: git mv and add the build constraint**

```bash
git mv internal/release/package_test.go internal/release/package_integration_test.go
```

Insert lines 1–2 exactly as in Task 2 Step 2 (`//go:build integration` + blank line).

- [ ] **Step 3: Rename the 4 top-level tests** (map rows, `func` declarations only)

- `TestPackageIntegration` → `TestIntegrationReleasePackageEndToEnd`
- `TestPackageDeterministic` → `TestIntegrationReleasePackageDeterministic`
- `TestPackageRefusesCollision` → `TestIntegrationReleasePackageRefusesCollision`
- `TestPackageBundleValidatesChecksums` → `TestIntegrationReleasePackageBundleValidatesChecksums`

- [ ] **Step 4: Verify both directions and content fidelity**

```bash
gofmt -l internal/release/
go vet -tags integration ./internal/release/
go test -count=1 -list '^Test' ./internal/release/
go test -tags integration -count=1 -list '^Test(Race)?Integration' ./internal/release/
go test -count=1 ./internal/release/
go test -tags integration -count=1 -run '^TestIntegrationReleasePackage' -v ./internal/release/
git diff HEAD -M -- internal/release/
```

Expected: default list = exactly the 28 `default`-corpus names from the map (no `TestPackage*`, no archive names); tagged `Integration` list = exactly the 9 moved names; default-corpus run passes (render/version/checksums still green, fast); tagged package run passes with 4 `--- PASS:` lines; `-M` diff hunks = constraint + 4 `func` lines only.

- [ ] **Step 5: Commit**

```bash
git add -A internal/release/
git commit -m "test(0362): move package tests behind the integration tag as TestIntegrationReleasePackage..."
```

---

### Task 4: Release shard runner + structural generalization of the partition contract (one commit)

These land together: the runner's `./internal/release` declaration reddens the OLD contract's allowlist, and the generalized contract without the runner reddens exact-one-runner coverage — neither half is green alone.

**Files:**
- Create: `tests/test_go_integration_release.sh`
- Modify: `tests/test_go_integration_contract.sh`
- Modify: `tests/runtime-budgets.tsv` (two provisional rows + commentary; finalized in Task 7)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`; finalized in Task 7)

**Interfaces:**
- Consumes: `tests/lib/go-integration-shard.sh` — `shard_inspect_maybe` / `run_integration_shard`, driven by the three literals `SHARD_PKG`, `SHARD_PREFIX`, `SHARD_MODE`; inspection prints `package=/prefix=/mode=/race=` lines under `DOCKET_SHARD_INSPECT=1`.
- Produces: the runner (inspectable, mode `normal`), and a contract whose 10 checks hold with ZERO hand-enumerated package names. Tasks 6–8 rely on the contract's assert descriptions staying byte-stable except where noted, so mutation receipts can name them.

- [ ] **Step 1: Write the runner** — clone of the `tests/test_go_integration_app_change.sh` shape:

```bash
#!/usr/bin/env bash
# tests/test_go_integration_release.sh — Go integration shard (change 0362): the
# release packaging/archive real cross-build, subprocess, tar/gzip and filesystem
# corpus, behind the `integration` build tag, prefix ^TestIntegrationRelease,
# sequential and non-race (no concurrent protocol; see the 0362 design's
# "No release race shard" non-goal). Declarations only — execution and inspection
# live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/release"
SHARD_PREFIX="TestIntegrationRelease"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

`chmod +x` to match sibling runners. Sanity: `DOCKET_SHARD_INSPECT=1 bash tests/test_go_integration_release.sh` prints `package=./internal/release`, `prefix=TestIntegrationRelease`, `mode=normal`, `race=` (empty); `bash tests/test_go_integration_release.sh` passes, `--- PASS` count 9.

- [ ] **Step 2: Generalize the contract — discovery replaces the three enumerations**

In `tests/test_go_integration_contract.sh`, replace `PKGS="internal/app internal/githubcli internal/gitcli"` with structural discovery from repository syntax, and keep everything downstream keyed on the discovered set:

```bash
# Packages are DISCOVERED from the repository's own *_integration_test.go files —
# never a hand-maintained list (learning enumerated-floor / backstop-must-compute-not-
# reenumerate): the package someone forgets to enumerate is the one that leaks.
# git ls-files is the census: tracked files only, no .git/.worktrees noise, and a
# tagged file someone forgot to `git add` cannot certify anything anyway.
int_files="$(git ls-files -- '*_integration_test.go')"
assert "at least one *_integration_test.go exists (discovery is non-empty)" '[ -n "$int_files" ]'
PKGS="$(while read -r f; do [ -n "$f" ] && dirname "$f"; done <<<"$int_files" | LC_ALL=C sort -u)"
```

`PKGS` is now newline-separated; the existing `for pkg in $PKGS` loops keep working via word-splitting (paths have no spaces — assert that: a discovered path containing whitespace or a leading `-` is a red result, shape-checked as `case "$pkg" in *[![:alnum:]/._-]*|-*) …fail…;; esac`).

Downstream edits, keeping all 10 properties:
1. **(1) tag placement loop** — iterate the discovered `$int_files` directly instead of `"$pkg"/*_integration_test.go` (same asserts: line 1 exact, line 2 blank). This also reaches a tagged file in a package the old glob never visited.
2. **(2) tagged corpus** — unchanged logic, over discovered `PKGS`.
3. **(3) runner well-formedness** — replace `case "$p" in ./internal/app|./internal/githubcli|./internal/gitcli)` with shape + module-existence validation:
   ```bash
   # once, before the runner loop — fail-closed module census:
   golist_out="$(go list ./... 2>&1)"; golist_rc=$?
   assert "go list ./... succeeds (module package census)" '[ "$golist_rc" -eq 0 ] || { printf "%s\n" "$golist_out" >&2; false; }'
   module_dirs="$(go list -f '{{.ImportPath}}' ./... 2>/dev/null | sed 's|^docket/||')"
   ```
   (Read the module path prefix from `go list -m` rather than hardcoding `docket` — capture `mod="$(go list -m)"` and strip `"$mod/"`.) A declaration is well-formed when `$p` matches shape `./?*` AND `${p#./}` is a member of `$module_dirs` (membership via `grep -qxF -- "${p#./}" <<<"$module_dirs"` — `-x` whole-line, `-F` literal, `--` guard).
4. **(4)/(5)/(9)/(7)** — unchanged: they already iterate `$tagged` × `$decl`, which are now discovery-fed.
5. **(6) leak probe** — unchanged logic, over discovered `PKGS`.
6. **(8) vet** — replace the enumerated three-package invocation with the discovered set:
   ```bash
   vet_pkgs="$(while read -r p; do [ -n "$p" ] && printf './%s/ ' "$p"; done <<<"$PKGS")"
   # shellcheck disable=SC2086 # deliberate word-splitting: one ./pkg/ per token.
   vet_out="$(go vet -tags integration $vet_pkgs 2>&1)"
   ```
   Update the assert text to "go vet -tags integration passes for every discovered integration package".
7. **(10) NEW bidirectional set correspondence** (learning correspondence-guard-runs-one-way): the discovered tagged-package set (dirs of `*_integration_test.go`) and the runner-declared package set (`$decl` field 2, `sort -u`) must be EQUAL — `comm -3` of the two sorted sets is empty, with each direction reported separately (a tagged package with no runner; a runner whose package has no tagged file). Note check (7) already reddens a runner whose PREFIX matches nothing; (10) is the package-level correspondence that catches a runner pointed at an untagged package before its prefix math even runs.

Also update the file's header comment: "the three packages" → discovery wording, and refresh the check list to include (10). Keep the header's check numbering in sync with the asserts.

- [ ] **Step 3: Prove the generalized contract over the real tree**

```bash
bash tests/test_go_integration_contract.sh
```

Expected: all asserts `ok - `, exit 0, and the discovered package set is exactly `internal/app internal/gitcli internal/githubcli internal/release` (echo it during development; do not leave debug output in). Re-run the sibling runners' inspection to confirm nothing regressed: `for r in tests/test_go_integration_*.sh; do DOCKET_SHARD_INSPECT=1 bash "$r" >/dev/null || echo "BROKEN $r"; done` (skip the contract itself — it does not source the shard lib; expect no BROKEN lines).

- [ ] **Step 4: Portability re-check of every new/changed regex**

Run each new grep pattern under `/usr/bin/grep` against a sample (ugrep masks BSD bugs — memory/learning grep-is-ugrep): at minimum the `-qxF` membership probe and any new `-E` class. Also confirm no new `producer | grep -q` pipelines under the file's `set -uo pipefail`.

- [ ] **Step 5: Provisional budget rows**

Warm-standalone time the two changed entry points (`/usr/bin/time -p bash tests/test_go_integration_release.sh >/dev/null`, same for the contract) and add/adjust rows in `tests/runtime-budgets.tsv`:
- `tests/test_go_integration_release.sh` — NEW row, provisional value from the warm reading via the row formula (expect the shard's cold cost to dominate later; Task 7 re-derives from fresh-cache readings and owns the final number). Mark the commentary "provisional — finalized from three fresh-cache readings (0362 Task 7)".
- `tests/test_go_integration_contract.sh` — existing row 15; the generalized contract adds one `go list ./...` and one package's `-list` probes; re-measure warm and raise only if the formula says so.
Update `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by the delta, with a comment naming the mover ("a new test file brings its own row"), then `bash tests/test_runtime_budgets.sh` → green.

- [ ] **Step 6: Commit**

```bash
git add tests/test_go_integration_release.sh tests/test_go_integration_contract.sh \
        tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0362): release integration shard + structural (allowlist-free) partition contract"
```

---

### Task 5: The live fidelity floor — tests/test_release_partition_fidelity.sh

The structural contract cannot see a package whose tagged files AND runner were deleted together — discovery just returns a smaller set. The committed map from Task 1 is the population floor; this test wires it to the live corpus (spec: "wire a fidelity check to compare that evidence with the live corpus").

**Files:**
- Create: `tests/test_release_partition_fidelity.sh`
- Modify: `tests/runtime-budgets.tsv` (+ row), `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces:**
- Consumes: `tests/fixtures/release-partition-map.tsv` (Task 1 format: `<old><TAB><new><TAB><integration|default>`).
- Produces: a suite file proving every mapped test name is alive in its declared corpus, fail-closed.

- [ ] **Step 1: Write the test**

```bash
#!/usr/bin/env bash
# tests/test_release_partition_fidelity.sh — the population floor of the 0362
# release partition. The structural contract (tests/test_go_integration_contract.sh)
# discovers packages from what EXISTS, so deleting internal/release's tagged files
# and runner together shrinks discovery instead of reddening it. This file pins the
# population against the committed map tests/fixtures/release-partition-map.tsv:
# every mapped name must be listed by `go test -list` in its declared corpus.
# FAIL-CLOSED: a listing error, an empty/malformed map, or a missing name is red.
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the CACHES
# note in that file's header): <git common dir>/docket-go-cache/{mod,build}.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the fidelity floor cannot certify anything without a Go toolchain\n'
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

MAP="tests/fixtures/release-partition-map.tsv"
map_rows="$(grep -vE -e '^#|^$' "$MAP" 2>/dev/null)"
assert "the partition map exists and is non-empty" '[ -n "$map_rows" ]'
n_rows="$(grep -c -E -e '.' <<<"$map_rows")"
badshape="$(awk -F'\t' 'NF!=3 || $1!~/^Test/ || $2!~/^Test/ || ($3!="integration" && $3!="default"){print NR": "$0}' <<<"$map_rows")"
assert "every map row is <old>TAB<new>TAB<integration|default>" \
  '[ -z "$badshape" ] || { printf "%s\n" "$badshape" >&2; false; }'
dup_old="$(cut -f1 <<<"$map_rows" | LC_ALL=C sort | uniq -d)"
dup_new="$(cut -f2 <<<"$map_rows" | LC_ALL=C sort | uniq -d)"
assert "no old name is mapped twice and no new name is claimed twice" \
  '[ -z "$dup_old" ] && [ -z "$dup_new" ] || { echo "  dup old:$dup_old dup new:$dup_new" >&2; false; }'

# Live corpora — each listing's exit status asserted before its output is trusted.
dout="$(go test -list '^Test' ./internal/release/ 2>&1)"; drc=$?
assert "go test -list (default tags) succeeds for internal/release" \
  '[ "$drc" -eq 0 ] || { printf "%s\n" "$dout" >&2; false; }'
iout="$(go test -tags integration -list '^Test' ./internal/release/ 2>&1)"; irc=$?
assert "go test -tags integration -list succeeds for internal/release" \
  '[ "$irc" -eq 0 ] || { printf "%s\n" "$iout" >&2; false; }'
dlist="$(grep -E -e '^Test' <<<"$dout" | LC_ALL=C sort -u)"
ilist="$(grep -E -e '^Test' <<<"$iout" | LC_ALL=C sort -u)"

# The floor, both directions (learning correspondence-guard-runs-one-way):
# (a) every mapped name is alive in its declared corpus;
missing=""
while IFS=$'\t' read -r old new corpus; do
  [ -n "$new" ] || continue
  case "$corpus" in
    integration) grep -qxF -- "$new" <<<"$ilist" || missing="$missing $new(integration)";;
    default)     grep -qxF -- "$new" <<<"$dlist" || missing="$missing $new(default)";;
  esac
done <<<"$map_rows"
assert "every mapped release test is alive in its declared corpus (population floor)" \
  '[ -z "$missing" ] || { echo "  missing:$missing" >&2; false; }'
# (b) every live release test is in the map — an unmapped addition must be a
# conscious map edit, or the floor silently stops describing the population.
live_all="$(printf '%s\n%s\n' "$dlist" "$ilist" | grep -E -e '.' | LC_ALL=C sort -u)"
mapped_new="$(cut -f2 <<<"$map_rows" | LC_ALL=C sort -u)"
unmapped="$(comm -23 <(printf '%s\n' "$live_all") <(printf '%s\n' "$mapped_new") | tr '\n' ' ')"
assert "every live internal/release test appears in the partition map (reverse direction)" \
  '[ -z "${unmapped// /}" ] || { echo "  unmapped:$unmapped" >&2; false; }'

exit "$fail"
```

- [ ] **Step 2: Run it, prove it can pass, and portability-check**

`bash tests/test_release_partition_fidelity.sh` → all `ok - `, exit 0 (learning plan-supplied-test-code-is-unverified: prove the asserts CAN pass before trusting them). Re-check the awk program and each grep under `/usr/bin/grep`/`/usr/bin/awk` semantics; note `grep -qxF -- "$new"` uses `--` because a name is data.

- [ ] **Step 3: Budget row**

`/usr/bin/time -p bash tests/test_release_partition_fidelity.sh >/dev/null` warm (expect ~2 `-list` compiles; likely the 10s floor). Add the row + commentary; bump `EXPECTED_TOTAL` accordingly; `bash tests/test_runtime_budgets.sh` green.

- [ ] **Step 4: Commit**

```bash
git add tests/test_release_partition_fidelity.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0362): live fidelity floor for the release partition map"
```

---

### Task 6: Mutation-prove the contract guards and the fidelity floor

One isolated break at a time; for each: `cp` backup → mutate → PROVE the mutation landed (`grep -c` target before=1/after=0 or equivalent) → run the guard fresh (`go test` legs are inside the scripts and already `-count=1`; `-list` probes re-execute every run) → observe the NAMED assert redden (the specific `NOT OK - ` line, not just exit 1 — learning assert-pins-outcome-not-mechanism) → `mv` the backup back → re-run green. Record each receipt (mutation, landed-proof, exact reddened assert text, restore-green) in build evidence.

**Files:**
- No durable file changes (all mutations restored). Evidence only.

- [ ] **Step 1: M1 — remove the release build tag.** Backup beside the file: `cp internal/release/package_integration_test.go internal/release/package_integration_test.go.0362bak` (same idiom for every later backup: `cp f f.0362bak` … `mv f.0362bak f`). Delete line 1 (`//go:build integration`) and the now-leading blank. Landed: `grep -c -F -- '//go:build integration' f` 1→0. Expect `bash tests/test_go_integration_contract.sh` to redden BOTH "every *_integration_test.go opens with //go:build integration" AND "no integration-prefixed test is visible to the default-tag corpus" (the 4 package tests leak). Restore, re-run green.

- [ ] **Step 2: M2 — remove the release runner.** `mv tests/test_go_integration_release.sh tests/go_integration_release.0362bak` (move OUT of the discovery glob's namespace, not a rename within it). Expect the contract to redden "every tagged test matches exactly one runner (none unmatched)" (9 release tests) AND the (10) set-correspondence direction "tagged package with no runner". Restore, green.

- [ ] **Step 3: M3 — remove BOTH tagged files and the runner.** Move `archive_integration_test.go`, `package_integration_test.go`, and the runner aside together. Expect: the CONTRACT stays green (discovery legitimately shrinks — this is the blindness the floor exists for) and `bash tests/test_release_partition_fidelity.sh` reddens "every mapped release test is alive in its declared corpus (population floor)" naming the 9 integration names. Both observations are required — the contract-green half proves the floor is not redundant decoration. Restore all three, both suites green.

- [ ] **Step 4: M4 — duplicate matching prefix.** `cp tests/test_go_integration_release.sh tests/test_go_integration_release_dup.sh`. Expect the contract to redden "every tagged test matches exactly one runner (none doubled)" for the 9 release tests. Remove the dup, green.

- [ ] **Step 5: M5 — misdeclare normal release coverage as race mode.** In the runner, `SHARD_MODE="normal"` → `SHARD_MODE="race"` (landed: `grep -c` on each spelling). Expect "race-prefixed tests run in race shards, and only they do" to redden with `want normal` entries. Restore, green.

- [ ] **Step 6: M6 — drop the executed race flag from an existing race runner.** Backup `tests/lib/go-integration-shard.sh`; mutate `shard_race_flag` to `printf '%s' ''` unconditionally (landed: `grep -c -F -- "printf '%s' '-race'"` 1→0). Run inspection-only through the contract: expect "race shards pass -race to go test and normal shards do not (executed decision)" to redden for `test_go_integration_app_concurrency.sh` and `test_go_integration_gitcli_concurrency.sh` — the contract reads race= from inspection, so this reddens without executing the heavy shards. Restore, green.

- [ ] **Step 7: M7 — expose an integration-prefixed test to the default corpus.** Create untagged `internal/release/leak_probe_test.go` containing `package release` and `func TestIntegrationReleaseLeakProbe(t *testing.T) {}` (add the imports Go requires: `import "testing"`). Expect the contract to redden "no integration-prefixed test is visible to the default-tag corpus" naming it, AND the fidelity floor's reverse direction "every live internal/release test appears in the partition map" to redden. Delete the probe file, both green.

- [ ] **Step 8: M8 — floor's own extractor is alive.** Corrupt the map fixture (backup first): delete one `integration` row. Expect the floor's reverse direction to redden ("unmapped: TestIntegrationRelease…"). Restore, green. (This proves the map is load-bearing, not decorative — learning frozen-copy-needs-a-drift-assert.)

- [ ] **Step 9: Confirm zero residue and commit nothing**

`git status --porcelain` must be empty (all backups/probes removed). No commit for this task; the receipts go into build evidence. If any expected redden did NOT occur, that is a guard defect: STOP, fix the guard in the owning task's files, commit the fix, and re-run the mutation — never record an unreddened mutation as acceptable (CLAUDE.md: a mutation that leaves an assert green is a defect until proven otherwise).

---

### Task 7: Three-fresh-cache worst-case measurement and final budget rows

Acceptance environment (spec): supported macOS host, otherwise idle, module-download cache WARM (network is not a budget), Go BUILD cache newly created and empty per measurement. Correctness asserts cannot judge this change — these numbers are the oracle (learning optimization-needs-a-measured-oracle); record the machine + contention context beside each number (learning tolerance-constant-calibrated-on-one-machine).

**Files:**
- Modify: `tests/runtime-budgets.tsv` (final rows + measurement commentary)
- Modify: `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL` final; `EXPECTED_SERIAL` stays 0)

- [ ] **Step 1: Measure — three readings each, fresh GOCACHE every time**

For each runner R in {`tests/test_go_integration_release.sh`, `tests/test_go_race.sh`, `tests/test_go_toolchain.sh`, `tests/test_release_partition_fidelity.sh`, `tests/test_go_integration_contract.sh`} run three times:

```bash
fresh="$(mktemp -d "${TMPDIR:-/tmp}/docket-0362-gocache.XXXXXX")"
GOCACHE="$fresh" /usr/bin/time -p bash R >/dev/null
rm -rf "$fresh"
```

(mktemp WITH a template — bare mktemp ignores TMPDIR on macOS.) Leave `GOMODCACHE` at its resolved shared value. A FAILED run is not a timing sample: diagnose it; if it looks environmental, run the identical command on a clean `origin/main` checkout before classifying (learning environment: a red run in a modified tree is a hypothesis). Record every `real` reading verbatim in build evidence, plus host model, macOS version, `go version`, and "idle" confirmation.

- [ ] **Step 2: Derive rows from the worst valid readings**

Per runner: worst reading → next 5s boundary above it → +5s → clamp to [10, 60]. Targets: each affected runner 45–50s cold. Decision table:
- worst ≤ 50s → write the row, done.
- 51–60s → SPLIT per the spec's adaptive order before writing any row at 55/60: release shard splits by the structural prefixes (`TestIntegrationReleasePackage` / `TestIntegrationReleaseArchive` — two runners, contract re-run, no `t.Parallel()` without an explicit isolation audit of temp dirs/env/globals/named resources); toolchain gate first separates static checks (gofmt/vet/cross-owner) from the `go test ./...` leg, then packages via `go list ./...`; race gate partitions its package set via `go list ./...`. ANY package-level split ships a bidirectional exact-cover guard in the contract: shard sets disjoint AND union == live `go list ./...` (a later package must auto-assign or redden — never an exclusion list). Each split runner is then re-measured 3× fresh-cache before its row is written.
- \>60s → immediate design failure: STOP and report with the readings; do not raise anything.

Expectation to verify, not assume (grooming data: release pkg cold non-race 28.62s; toolchain cold was 68.99s PRE-partition, and the partition removes package/archive compilation+execution from its `go test ./...` leg): toolchain likely lands ≤50 post-partition — if it does not, the split path above is mandatory, not optional.

- [ ] **Step 3: Write final rows + commentary; re-derive the totals**

Update `tests/runtime-budgets.tsv`: final values for `test_go_integration_release.sh`, `test_release_partition_fidelity.sh`, and any changed `test_go_race.sh` / `test_go_toolchain.sh` / `test_go_integration_contract.sh` rows (a LOWERED row is also a legitimate re-shape mover), with header commentary in the file's house style: the three readings, the worst, the formula, machine context, and change id. Set `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` to the new sum with a comment naming each mover and which legitimate case it is; `EXPECTED_SERIAL` remains 0. Run `bash tests/test_runtime_budgets.sh` → green. Report remaining headroom per touched row as a NUMBER in evidence (learning budget-headroom-is-spent-before-it-is-breached: parity is a finding).

- [ ] **Step 4: Commit**

```bash
git add tests/runtime-budgets.tsv tests/test_runtime_budgets.sh
git commit -m "test(0362): fresh-cache measured budget rows for the partitioned release corpus"
```

(If Step 2 forced a split, the new runner files + contract exact-cover guard land in this same task's commit with their own mutation receipts per Task 6's procedure.)

---

### Task 8: Whole-suite verification and evidence

**Files:** none (evidence only; fixes, if any, belong to the owning task's files).

- [ ] **Step 1: Run the complete configured suite** — the command `finalize.test_command` resolves to (read it from config; do not spell a second copy here). Expected: `SUITE files=<N> passed=<N> failed=0`; N grew by 2 (release shard + fidelity floor; more if Task 7 split). Treat any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line on a touched file as a screening finding to serially confirm (`scripts/run-tests.sh -j 1 --timings` on that file), and any `SERIAL CONFIRMED OVER BUDGET:` as an authoritative breach to act on (back to Task 7's decision table). Nothing else will catch these — the suite does not fail on them by default.

- [ ] **Step 2: Confirm the change's own completion criteria against live state** — each with its command receipt in evidence:
  - default corpus blindness: `go test -count=1 -list '^Test(Race)?Integration' ./internal/release/` lists nothing;
  - tagged runner executes all 9 moved tests exactly once (runner output's `--- PASS` count vs `-list` count — the shard lib already asserts this; capture the line);
  - no release race runner exists: `grep -l 'SHARD_MODE="race"' tests/test_go_integration_*.sh` names only the two pre-existing concurrency runners;
  - contract green, floor green, mutation receipts complete (Task 6), all rows fresh-cache-derived under 60 with 45–50 target met or split executed, race gate's `internal/release` line in `go test -race` output far below 600s (record the per-package `ok … <seconds>` line), totals match topology.

- [ ] **Step 3: No commit** — hand the branch to review with evidence recorded.

---

## Self-Review (performed while writing)

- Spec coverage: partition + rename family (Tasks 2–3), single sequential non-race runner via shared helper (Task 4), structural contract with all 10 fail-closed properties incl. bidirectional set correspondence (Task 4), before/after inventory + committed map + WIRED fidelity floor (Tasks 1, 5), mutation proofs incl. the both-halves-deleted case and the dropped `-race` case (Task 6), three-fresh-cache measurement with 45–50 target / 60 ceiling / adaptive split order / exact-cover guard on any split (Task 7), re-derived `EXPECTED_SERIAL`/`EXPECTED_TOTAL` and full parallel suite + evidence (Tasks 7–8). Non-goals honored: no production edits, no race shard, no re-partition of 0333's packages, no global timeout raise, no 0361 mutation.
- Names used consistently: `TestIntegrationRelease` prefix, 9 mapped integration names identical across Tasks 1/2/3/6; `tests/fixtures/release-partition-map.tsv` across Tasks 1/5/6.
- Known intermediate reds are confined inside tasks; every task boundary leaves contract, budgets, and Go builds green.
