<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0466 — Bring test_go_race back under its 60s budget row (transaction, workspace, gatedrive)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-29-0466-bring-test-go-race-back-under-its-60s-budget-row-transaction.md)**
<!-- docket:backlink:end -->
# Partition transaction, workspace, and gatedrive out of the race gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In this repository the build runs through `docket-build`: one `### Task N` heading is one worker and one commit.)

**Goal:** Bring `tests/test_go_race.sh` back under its 60s row and `tests/test_go_toolchain.sh` under its 55s row (cold test cache), without raising any row, by moving the real-git and real-process tests of `internal/repository/transaction`, `internal/workspace`, and `internal/gatedrive` behind `//go:build integration`.

**Architecture:** This extends change 0333/0465's integration partition to three more packages. Real-git and real-process tests move into `*_integration_test.go` / `*_race_integration_test.go` files, get `TestIntegration…` / `TestRaceIntegration…` names, and run in eight new shard runners on the unchanged `tests/lib/go-integration-shard.sh`. 0465's no-real-git PATH-shim guard is hoisted from `internal/app` into `internal/testsupport` (`InstallNoGitGuard`, with the build-tag split living there once). It is installed from `TestMain` in `internal/app` (byte-identical behavior), `internal/repository/transaction`, and `internal/workspace`. `internal/gatedrive` is process-bound, not git-bound, so it gets no guard, and its budget row stays the growth detector.

**Tech Stack:** Go 1.27 (`testing`, `os/exec`), bash shard runners over `tests/lib/go-integration-shard.sh`, the contract `tests/test_go_integration_contract.sh`, the Go suite runner (`internal/suiterunner`), `tests/runtime-budgets.tsv`.

**Spec:** `docs/superpowers/specs/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction-design.md` (on the `docket` metadata branch; read-only copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-09-28-bring-test-go-race-back-under-its-60s-budget-row-transaction-design.md`).

**Feature worktree:** `/Users/homer/dev/docket/.worktrees/bring-test-go-race-back-under-its-60s-budget-row-transaction` (branch `chore/bring-test-go-race-back-under-its-60s-budget-row-transaction`). Every command below runs from this directory unless it says otherwise.

## Global Constraints

- **Partition, don't parallelize.** No `t.Parallel()` anywhere in the three packages. The transaction and workspace harnesses set `GIT_CONFIG_GLOBAL` process-wide via `t.Setenv`, which is safe only because nothing runs in parallel.
- **Do not raise** the `tests/test_go_race.sh` (60) or `tests/test_go_toolchain.sh` (55) rows, and do not touch `RACE_TIMEOUT` (8m) or `internal/repoguard/race_gate_timeout_test.go`'s `raceBackstopFloor`.
- **Do not edit** `tests/test_go_integration_contract.sh` or `tests/lib/go-integration-shard.sh`. The contract discovers the new packages from the `*_integration_test.go` census (change 0362). It must pass with no allowlist edit.
- Every `*_integration_test.go` / `*_race_integration_test.go` file has line 1 exactly `//go:build integration` and line 2 blank (contract check (1)).
- **What moves is keyed on shape, not timing.** In transaction and workspace, every test that starts real git moves. The guard is the definition. In gatedrive, every test that drives the real `internal/process.Service` supervisor or real git moves, including every test in the four existing untagged files `integration_test.go`, `integration_history_test.go`, `integration_sequence_test.go`, and `integration_takeover_test.go`.
- **Race vs normal.** A moved test goes to a `mode=race` shard (`TestRaceIntegration…` prefix) when its body starts more than one goroutine or process against shared state (`go func`/`go f(`, `sync.WaitGroup`, channels coordinating simultaneous work, or two live processes/launches against one store or slot). When in doubt, it goes to race. Everything else goes to a `mode=normal` shard (`TestIntegration…` prefix). Each race-classified test gets a one-line comment directly above its `func`: `// Race shard (change 0466): <what runs concurrently>.` The plan pre-classifies every test (see each task). A worker who disagrees with a classification reports it and does not silently re-route.
- **Renaming rule.** New name = shard prefix + the old name with its leading `Test` removed. For gatedrive names that already start `TestIntegration` or `TestRace`, strip that whole leading word instead (see Task 9's explicit table). Examples: `TestEngineAppliesHappyPath` becomes `TestIntegrationTxnApplyEngineAppliesHappyPath`, and `TestIntegrationDeadlineExpiryStopsOwnedTree` becomes `TestIntegrationGatedriveDeadlineExpiryStopsOwnedTree`.
- **Prefixes (fixed by this plan; none is a string prefix of another in the same package):**
  - transaction: `TestIntegrationTxnApply` (normal), `TestIntegrationTxnRecovery` (normal), `TestRaceIntegrationTxn` (race)
  - workspace: `TestIntegrationWorkspaceSetup` (normal), `TestIntegrationWorkspaceLifecycle` (normal), `TestRaceIntegrationWorkspace` (race)
  - gatedrive: `TestIntegrationGatedrive` (normal), `TestRaceIntegrationGatedrive` (race)
- **Runner names:** `tests/test_go_integration_transaction_{apply,recovery,race}.sh`, `tests/test_go_integration_workspace_{setup,lifecycle,race}.sh`, `tests/test_go_integration_gatedrive_{process,race}.sh`.
- **Budget rows.** Every new `tests/test_*.sh` gets exactly one row in `tests/runtime-budgets.tsv` (`internal/repoguard` `TestRuntimeBudgetsCorrespondence`). Row = the solo (serial) measured `real` seconds, rounded up to the next multiple of 5, plus 5, minimum 10. That is the table's own rule, and it guarantees at least 5s of headroom, never parity (learning `budget-headroom-is-spent-before-it-is-breached`). Format: `<path><TAB><seconds><TAB>parallel`. Insert new rows directly above the `tests/test_go_integration_release.sh` row. A computed row above 40 is a stop: return NEEDS_ESCALATION with the measurement so the shard can be split into two runners with disjoint new prefixes. Record every measurement and margin as numbers in your task report, never as "under budget".
- **Helpers.** Fixture helpers used by both corpora stay in a file with no build tag. Helpers used only by the tagged corpus move behind the tag, so each build compiles exactly what it uses. `go vet ./<pkg>/` and `go vet -tags integration ./<pkg>/` must both pass after every task.
- **Never hand-list sites.** Offenders come from the census (`census.sh`, then the guard). References to renamed tests come from a whole-repo `git grep` (`rename-tests.sh`). Point-in-time records keep old names: never rewrite `docs/results/**`, `docs/changes/**`, `docs/superpowers/**`, or `docs/adrs/**`.
- **Cache.** Every run that observes a verdict defeats Go's test cache (`-count=1`; learning `cached-runner-serves-a-mutated-tree`). Mutation probes back up the file with `cp`, prove the mutation landed (`cmp` must report a difference) before reading the result, and restore by copying the backup back. Never `git checkout --` an uncommitted edit (learning `mutation-restore-needs-a-backup-copy`).
- **Shell rules (AGENTS.md):** never pipe a producer into `grep -q`/`head`. Capture into a variable first. Write grep patterns as `grep -E -e`. Template every `mktemp` as `"${TMPDIR:-/tmp}/<name>.XXXXXX"`. Use `mv -f`. The Bash tool's shell is zsh, so run multi-line snippets through `bash -c '…'` or a script file.
- Stage only the paths your task names (`git add -- <paths>`, never `git add -A`). Check `git status --porcelain` before every commit.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (ADR-0054).
- **Every task leaves the tree green.** Tests move first, and each package's guard is installed only after all of that package's offenders have moved (Tasks 5 and 8). There is no expected-red intermediate state. If a task's default package goes red, that is a defect in that task.
- The whole-suite gate (`build.test_command`) runs once at the end of the build (docket-build). Its budget report must print no `SERIAL CONFIRMED OVER BUDGET` line for any file this change created or touched (spec acceptance 5). Read the report even when it is green.

## Shared tools

Tasks 2–9 use four throwaway helper scripts. They are **not committed**. Each task that needs them writes them to `${TMPDIR:-/tmp}/docket-0466-tools/` if that directory does not already hold them, by copying the four blocks below verbatim. All four were executed against this tree at plan time (a module copy for the mover and import pruner, the live tree for the census). Still, prove each one on your first use (learning `plan-supplied-test-code-is-unverified`): the mover must report `not found` for a bogus name and change nothing, and the census must report a non-zero count for a test you know starts git.

`movefuncs.sh` moves named top-level funcs, each with its leading comment block, from one file to another. It creates the tagged destination when absent:

```bash
#!/usr/bin/env bash
# movefuncs.sh <src> <dst> <FuncName>... — move each top-level `func <FuncName>(`
# declaration, with its contiguous leading `//` comment block, from <src> to the end of
# <dst>. When <dst> does not exist it is created as an integration-tagged file: line 1
# `//go:build integration`, line 2 blank, then <src>'s package clause and import block
# (prune-imports.sh removes the imports that turn out unused). Fails, changing
# nothing, when a name is not found exactly once or has no closing `}` at column 0.
set -euo pipefail
src="$1"; dst="$2"; shift 2
[ -f "$src" ] || { echo "movefuncs: $src does not exist" >&2; exit 2; }
names="$*"
tmp_keep="$(mktemp "${TMPDIR:-/tmp}/movefuncs-keep.XXXXXX")"
tmp_move="$(mktemp "${TMPDIR:-/tmp}/movefuncs-move.XXXXXX")"
awk -v names="$names" -v keep="$tmp_keep" -v move="$tmp_move" '
BEGIN { n = split(names, want, " "); for (i = 1; i <= n; i++) wanted[want[i]] = 1 }
{ line[NR] = $0 }
END {
  for (i = 1; i <= NR; i++) {
    if (match(line[i], /^func [A-Za-z0-9_]+\(/)) {
      nm = substr(line[i], 6, RLENGTH - 6)
      if (nm in wanted) {
        if (nm in found) { print "movefuncs: " nm " declared twice" > "/dev/stderr"; exit 3 }
        found[nm] = 1
        s = i; while (s > 1 && line[s-1] ~ /^\/\//) s--
        e = i; while (e <= NR && line[e] != "}") e++
        if (e > NR) { print "movefuncs: no closing brace for " nm > "/dev/stderr"; exit 3 }
        for (j = s; j <= e; j++) moved[j] = 1
      }
    }
  }
  for (k in wanted) if (!(k in found)) { print "movefuncs: " k " not found in source" > "/dev/stderr"; exit 3 }
  for (i = 1; i <= NR; i++) {
    if (i in moved) { print line[i] > move; if (!((i + 1) in moved)) print "" > move }
    else print line[i] > keep
  }
}' "$src"
if [ ! -e "$dst" ]; then
  {
    printf '//go:build integration\n\n'
    awk '/^package /{print; print ""; next} /^import \($/{inimp=1} inimp{print} inimp && /^\)$/{exit} /^import "/{print; exit}' "$src"
  } > "$dst"
fi
{ printf '\n'; cat "$tmp_move"; } >> "$dst"
mv -f "$tmp_keep" "$src"
rm -f "$tmp_move"
gofmt -w "$src" "$dst"
```

`prune-imports.sh` deletes compiler-reported unused imports in both builds until vet is clean:

```bash
#!/usr/bin/env bash
# prune-imports.sh <pkg dir> — delete import lines the compiler reports as unused, in
# both the default and the integration build, until neither reports any (max 20 rounds;
# go vet reports one type-check error at a time). Any OTHER compile error ends the
# script with exit 1 and the vet output: fix that by hand, then re-run.
# Run from the module root.
set -uo pipefail
pkg="$1"
re='([^ :]+\.go):([0-9]+):[0-9]+: "[^"]+" imported (as [A-Za-z_][A-Za-z0-9_]* )?and not used'
for round in $(seq 1 20); do
  changed=0
  for tags in "" "-tags integration"; do
    out="$(go vet $tags "./$pkg" 2>&1)"
    hits="$(grep -o -E -e "$re" <<<"$out" || true)"
    [ -n "$hits" ] || continue
    while IFS= read -r h; do
      file="$(sed -E "s/$re/\\1/" <<<"$h")"; line="$(sed -E "s/$re/\\2/" <<<"$h")"
      printf '%s %s\n' "$file" "$line"
    done <<<"$hits" | LC_ALL=C sort -u | LC_ALL=C sort -k1,1 -k2,2nr > "${TMPDIR:-/tmp}/prune-imports.list"
    while read -r file line; do
      sed -i.bak "${line}d" "$file" && rm -f "$file.bak"
    done < "${TMPDIR:-/tmp}/prune-imports.list"
    rm -f "${TMPDIR:-/tmp}/prune-imports.list"
    changed=1
  done
  [ "$changed" -eq 1 ] || break
done
gofmt -w "$pkg"
for tags in "" "-tags integration"; do
  out="$(go vet $tags "./$pkg" 2>&1)"; rc=$?
  [ "$rc" -eq 0 ] || { printf 'go vet %s ./%s still fails:\n%s\n' "$tags" "$pkg" "$out"; exit 1; }
done
echo "prune-imports: go vet clean (default and integration) for $pkg"
```

`census.sh` runs each default-build test in the named files alone with a logging git shim, and prints how many git execs it made:

```bash
#!/usr/bin/env bash
# census.sh <pkg dir> <test file>... — for every `func Test…` declared in the named
# default-build test files, run that test ALONE (default tags, -count=1) with a
# logging git shim first on PATH, and print "<file> <test> <git-execs> <rc>".
# git-execs > 0 means the test starts real git and must move behind the tag.
set -uo pipefail
pkg="$1"; shift
work="$(mktemp -d "${TMPDIR:-/tmp}/census.XXXXXX")"
real_git="$(command -v git)"
mkdir -p "$work/shim"
printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> "%s/git.log"\nexec "%s" "$@"\n' "$work" "$real_git" > "$work/shim/git"
chmod 755 "$work/shim/git"
( cd "$pkg" && go test -c -o "$work/pkg.test" . ) || { echo "census: build failed" >&2; exit 2; }
for f in "$@"; do
  names="$(grep -E -e '^func Test[A-Za-z0-9_]*\(' "$pkg/$f" | sed -E 's/^func (Test[A-Za-z0-9_]*)\(.*/\1/')"
  for t in $names; do
    [ "$t" = "TestMain" ] && continue
    : > "$work/git.log"
    ( cd "$pkg" && PATH="$work/shim:$PATH" "$work/pkg.test" -test.run "^${t}\$" -test.count=1 >/dev/null 2>&1 ); rc=$?
    n="$(wc -l < "$work/git.log" | tr -d ' ')"
    printf '%s %s %s %s\n' "$f" "$t" "$n" "$rc"
  done
done
rm -rf "$work"
```

(`census.sh` only works **before** a package's guard is installed, because the guard's shim shadows the logging shim. After Tasks 5 and 8, the guard itself is the census.)

`rename-tests.sh` rewrites whole-word old names to new names across maintained files, tracked and untracked. **One moved name is declared in three packages:** `TestHarnessBuildersProduceExpectedTopology` exists in `internal/gitcli`, `internal/repository/transaction`, and `internal/workspace`. Always rename it with `--scope <its package dir>`, never repo-wide. Tasks 2 and 6 do this. Before any repo-wide rename, check that the old name is declared in only one package: `git grep -l -E -e "^func <Old>\(" -- internal` must list one file.

```bash
#!/usr/bin/env bash
# rename-tests.sh [--scope <dir>] OLD=NEW... — rewrite every whole-word OLD to NEW across
# maintained files, tracked AND untracked (a file movefuncs.sh just created is untracked),
# with point-in-time records excluded, and fail if any OLD survives in that scope. Default
# scope is the whole repository; pass --scope <package dir> for a test name that another
# package also declares.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)" || exit 2
scope="."
if [ "${1-}" = "--scope" ]; then scope="$2"; shift 2; fi
excl=(-- "$scope" ':!docs/results' ':!docs/changes' ':!docs/superpowers' ':!docs/adrs')
for pair in "$@"; do
  old="${pair%%=*}"; new="${pair#*=}"
  if [ -z "$old" ] || [ -z "$new" ] || [ "$old" = "$pair" ]; then echo "rename-tests: bad pair $pair" >&2; exit 2; fi
  hits="$(git grep --untracked -l -w -e "$old" "${excl[@]}")"
  [ -z "$hits" ] || perl -pi -e "s/\\b\\Q${old}\\E\\b/${new}/g" $hits
  left="$(git grep --untracked -n -w -e "$old" "${excl[@]}")"
  [ -z "$left" ] || { printf 'STILL REFERENCED %s:\n%s\n' "$old" "$left"; exit 1; }
done
```

`defaultonly-unused.sh` lists untagged top-level helpers that no untagged test file references, meaning helpers only the tagged corpus uses:

```bash
#!/usr/bin/env bash
# defaultonly-unused.sh <pkg dir> — list top-level funcs/types/vars/consts declared in
# the package's UNTAGGED *_test.go files that no untagged *_test.go file references
# anywhere except on the declaring line. Those are helpers only the tagged corpus
# (or nothing) uses. Iterate: after moving them, re-run until it prints nothing new.
set -uo pipefail
pkg="$1"
untagged=()
for f in "$pkg"/*_test.go; do
  first="$(sed -n '1p' "$f")"
  case "$first" in //go:build*) continue;; esac
  untagged+=("$f")
done
[ "${#untagged[@]}" -gt 0 ] || exit 0
decls="$(grep -h -o -E -e '^(func|type|var|const) [A-Za-z_][A-Za-z0-9_]*' "${untagged[@]}" | awk '{print $2}' | LC_ALL=C sort -u)"
for name in $decls; do
  case "$name" in Test*|Benchmark*|Example*|Fuzz*) continue;; esac
  uses="$(grep -h -w -e "$name" "${untagged[@]}" | grep -v -E -e "^(func|type|var|const) ${name}\b" | wc -l | tr -d ' ')"
  [ "$uses" -eq 0 ] && printf '%s\t%s\n' "$name" "$(grep -l -E -e "^(func|type|var|const) ${name}\b" "${untagged[@]}" | xargs -n1 basename | tr '\n' ' ')"
done
exit 0
```

(Its known limits: methods and names declared inside grouped `const (`/`var (` blocks are not listed, and a helper referenced only by another tagged-only untagged helper surfaces on the next iteration. Both builds' `go vet` stays the authority on correctness. This script only finds candidates.)

## Plan-time census (snapshot)

Taken on `ef4a341d2` with `go test -race -c` binaries, each top-level test run alone with a logging git shim on `PATH` (three packages concurrently, so wall times are inflated by load, about 1s of per-run overhead subtracted):

| package | tests | real-git tests | time in real-git tests | stays in default corpus |
|---|---|---|---|---|
| transaction | 102 | 68 | ~61s | 34 tests, ~1.2s |
| workspace | 91 | 59 | ~59s | 32 tests, ~1.3s |
| gatedrive | 290 | 15 (fingerprint 9, handoff 2, `integration_sequence` 4) | ~6s | 265 tests after moving 25 (~15s of the real-process tests leave with them) |

The per-task lists below are this snapshot. Each task's Step 1 re-takes the census live, and **the live census wins**.

## Review Focus

1. **A helper a default test still needs ends up behind the tag** (or the reverse: a tagged-only helper stays untagged and the tagged file compiles only by accident). Every move task runs `prune-imports.sh`, which fails unless `go vet` passes in **both** builds. The guard tasks (5, 8) and the gatedrive task (9) run `defaultonly-unused.sh` and relocate the tagged-only helpers.
2. **`internal/app` must behave byte-identically after the hoist**: same diagnostic text, same shim script bytes and remedy line, same exit code 97, same probe argument, same self-probe env var, same `docket-app-nogit-*` temp dir. Pinned by Task 1's golden test `TestNoGitShimScriptKeepsInternalAppBytes`, whose literal is first proven against the **pre-hoist** function (Task 1 Step 1), and by `internal/app`'s three proving tests still passing.
3. **A package name or shard glob that the shim's single-quoted `printf` cannot carry** (a `'`, `%`, `\`, or newline). It would produce a shim that is a shell syntax error, one that exits non-zero on every call but never logs, so a tolerant test would pass. Pinned by `TestValidateNoGitGuardArgs` (Task 1), and `InstallNoGitGuard` refuses such input and exits 1 before writing the shim.
4. **The tagged builds must not get the shim.** The integration shards of transaction and workspace run real git from the same `TestMain`, so an installed guard would redden every shard. Pinned by Task 5 Step 7 and Task 8 Step 7, which run every shard of the package after the guard is installed, and by `go vet -tags e2e ./internal/app/` in Task 1.
5. **A real-process test left in gatedrive's default corpus.** It has no guard to catch it. Pinned by Task 9 Step 8's shape check: no untagged gatedrive test file references `process.NewService`, `mustService(`, `mustExe(`, or `intChildMarker`. The census over every remaining untagged gatedrive file must also report 0 git execs.

---

### Task 1: Hoist the no-real-git guard into `internal/testsupport`

**Build profile:** premium

The named risk: `internal/app`'s `TestMain` also routes the supervisor and guardian re-exec roles. A hoist that changes where or when the guard installs, or that compiles it into the wrong build, breaks every real `GateLaunch`/guardian test or the integration/e2e corpora. `internal/app` must behave byte-identically.

**Files:**
- Create: `internal/testsupport/nogit.go` (untagged: constants, diagnostic, shim renderer, verdict, argument validation, proving-test helpers)
- Create: `internal/testsupport/nogit_install.go` (`//go:build !integration && !e2e`: the real `InstallNoGitGuard`)
- Create: `internal/testsupport/nogit_install_off.go` (`//go:build integration || e2e`: the identity twin)
- Create: `internal/testsupport/nogit_test.go` (untagged: golden bytes, verdict table, validation table, executed-shim test)
- Modify: `internal/app/nogit_guard_test.go` (becomes three thin proving tests)
- Delete: `internal/app/nogit_guard_off_test.go` (its no-op twin now lives in testsupport)
- Modify: `internal/app/gate_test.go` (`TestMain` call site, two constants, comment)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (Tasks 5, 8, and 10 rely on these exact names):
  - `const testsupport.NoGitGuardProbeArg = "docket-nogit-guard-probe"`, `const testsupport.NoGitGuardExit = 97`, `const testsupport.NoGitSelfProbeEnv = "DOCKET_NOGIT_GUARD_SELF_PROBE"`
  - `func testsupport.NoGitGuardDiagnostic(pkg string) string`, which returns `"docket nogit guard: default " + pkg + " tests must not run real git"`
  - `func testsupport.NoGitShimScript(pkg, shardGlob, logPath string) string`
  - `func testsupport.NoGitVerdict(pkg, logPath string, code int, w io.Writer) int`
  - `func testsupport.InstallNoGitGuard(pkg, shardGlob string) func(code int) int`: installs the guard in the default build; the identity in `integration`/`e2e` builds
  - `func testsupport.NoGitGuardDir() string`: `""` when not installed
  - `func testsupport.AssertNoGitGuardShadowsGit(t testing.TB)`, `func testsupport.AssertNoGitGuardRefusesBareExec(t testing.TB, pkg string)`, `func testsupport.NoGitGuardTolerantProbe(t *testing.T, pkg, testName string)`
  - Per-package convention: an untagged file holding `TestMain` declares `const nogitPkg = "<module-relative package dir>"` and `const nogitShardGlob = "tests/test_go_integration_<short>_*.sh"`. A default-only `nogit_guard_test.go` holds `TestNoGitGuardShadowsGitOnPath`, `TestNoGitGuardRefusesBareExec`, and `TestNoGitGuardFailsTolerantTest`.

- [ ] **Step 1: Prove the golden literal against the pre-hoist function**

Create the throwaway file `internal/app/nogit_golden_probe_test.go`:

```go
//go:build !integration && !e2e

package app

import "testing"

func TestNoGitShimGoldenProbe(t *testing.T) {
	const want = "#!/bin/sh\n" +
		"if [ \"${1-}\" != 'docket-nogit-guard-probe' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '/x/violations.log'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a tests/test_go_integration_app_*.sh shard (change 0465; partition from change 0333)\\n' 'docket nogit guard: default internal/app tests must not run real git' \"$*\" >&2\n" +
		"exit 97\n"
	if got := nogitShimScript("/x/violations.log"); got != want {
		t.Fatalf("golden drifted from the pre-hoist shim:\n got %q\nwant %q", got, want)
	}
}
```

Run: `go test -count=1 -run '^TestNoGitShimGoldenProbe$' ./internal/app/`
Expected: PASS. This proves the literal Step 2 uses is the pre-hoist bytes. Then delete the file: `rm -f internal/app/nogit_golden_probe_test.go`.

- [ ] **Step 2: Write the failing testsupport tests**

Create `internal/testsupport/nogit_test.go`:

```go
package testsupport

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoGitShimScriptKeepsInternalAppBytes pins the hoist as byte-identical for
// internal/app (change 0466): the literal was proven against the pre-hoist
// internal/app nogitShimScript before that function was deleted.
func TestNoGitShimScriptKeepsInternalAppBytes(t *testing.T) {
	const want = "#!/bin/sh\n" +
		"if [ \"${1-}\" != 'docket-nogit-guard-probe' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '/x/violations.log'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a tests/test_go_integration_app_*.sh shard (change 0465; partition from change 0333)\\n' 'docket nogit guard: default internal/app tests must not run real git' \"$*\" >&2\n" +
		"exit 97\n"
	if got := NoGitShimScript("internal/app", "tests/test_go_integration_app_*.sh", "/x/violations.log"); got != want {
		t.Fatalf("internal/app shim bytes changed:\n got %q\nwant %q", got, want)
	}
	if got, want := NoGitGuardDiagnostic("internal/app"), "docket nogit guard: default internal/app tests must not run real git"; got != want {
		t.Fatalf("NoGitGuardDiagnostic(internal/app) = %q, want %q", got, want)
	}
}

// TestNoGitShimScriptRefusesAndLogs EXECUTES a rendered shim: a non-probe call is
// logged as "<cwd>\t<argv>" and refused with the package's diagnostic, its shard
// glob, and exit NoGitGuardExit; a probe call is refused but not logged.
func TestNoGitShimScriptRefusesAndLogs(t *testing.T) {
	dir := TempDir(t)
	logPath := filepath.Join(dir, "violations.log")
	shim := filepath.Join(dir, "git")
	pkg, glob := "internal/repository/transaction", "tests/test_go_integration_transaction_*.sh"
	if err := os.WriteFile(shim, []byte(NoGitShimScript(pkg, glob, logPath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shim, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(shim, "status", "--porcelain").CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != NoGitGuardExit {
		t.Fatalf("shim must exit %d, got err=%v output=%q", NoGitGuardExit, err, out)
	}
	for _, want := range []string{NoGitGuardDiagnostic(pkg), "(git status --porcelain)", glob} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("shim stderr must contain %q, got %q", want, out)
		}
	}
	logged, err := os.ReadFile(logPath)
	if err != nil || !strings.Contains(string(logged), "\tstatus --porcelain") {
		t.Fatalf("violation log must record the call, got %q (err %v)", logged, err)
	}
	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Command(shim, NoGitGuardProbeArg).CombinedOutput(); err == nil {
		t.Fatalf("a probe call must still be refused")
	}
	if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a probe call must not be logged (stat err %v)", err)
	}
}

// TestNoGitVerdict covers the post-m.Run verdict over the violation log (moved
// from internal/app's TestNoGitGuardVerdict by change 0466).
func TestNoGitVerdict(t *testing.T) {
	dir := TempDir(t)
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := []struct {
		name     string
		logPath  string
		code     int
		want     int
		wantText string
	}{
		{"missing log is clean", filepath.Join(dir, "absent.log"), 0, 0, ""},
		{"empty log is clean", write("empty.log", ""), 0, 0, ""},
		{"one violation fails a green run", write("one.log", "/tmp/x\tstatus --porcelain\n"), 0, 1, "1 real-git exec attempt(s)"},
		{"violation keeps an existing failure code", write("keep.log", "/tmp/x\tlog\n"), 2, 2, "/tmp/x\tlog"},
		{"unreadable log fails closed", dir, 0, 1, "cannot read the violation log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			got := NoGitVerdict("internal/app", tc.logPath, tc.code, &buf)
			if got != tc.want {
				t.Fatalf("NoGitVerdict(%q, %d) = %d, want %d; output:\n%s", tc.logPath, tc.code, got, tc.want, buf.String())
			}
			if tc.wantText == "" && buf.Len() != 0 {
				t.Fatalf("clean verdict must print nothing, got:\n%s", buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), tc.wantText) {
				t.Fatalf("verdict output must contain %q, got:\n%s", tc.wantText, buf.String())
			}
			if tc.wantText != "" && !strings.Contains(buf.String(), NoGitGuardDiagnostic("internal/app")) {
				t.Fatalf("verdict output must carry the package diagnostic, got:\n%s", buf.String())
			}
		})
	}
}

// TestValidateNoGitGuardArgs: the shim embeds pkg and shardGlob inside a
// single-quoted printf, so a quote, percent sign, backslash, or newline (or an
// empty value) is refused rather than rendered into a broken shim.
func TestValidateNoGitGuardArgs(t *testing.T) {
	okGlob := "tests/test_go_integration_app_*.sh"
	cases := []struct {
		name, pkg, glob string
		ok              bool
	}{
		{"app", "internal/app", okGlob, true},
		{"transaction", "internal/repository/transaction", "tests/test_go_integration_transaction_*.sh", true},
		{"empty package", "", okGlob, false},
		{"empty glob", "internal/app", "", false},
		{"quote in package", "internal/a'pp", okGlob, false},
		{"percent in glob", "internal/app", "tests/%s.sh", false},
		{"backslash in glob", "internal/app", `tests\x.sh`, false},
		{"newline in package", "internal/app\nx", okGlob, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateNoGitGuardArgs(tc.pkg, tc.glob)
			if tc.ok && err != nil {
				t.Fatalf("validateNoGitGuardArgs(%q, %q) = %v, want nil", tc.pkg, tc.glob, err)
			}
			if !tc.ok && err == nil {
				t.Fatalf("validateNoGitGuardArgs(%q, %q) = nil, want a refusal", tc.pkg, tc.glob)
			}
		})
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

Run: `go test -count=1 ./internal/testsupport/`
Expected: FAIL to compile with `undefined: NoGitShimScript` (and the other new names).

- [ ] **Step 4: Implement the shared guard**

Create `internal/testsupport/nogit.go`:

```go
package testsupport

// The default-build no-real-git guard (change 0465, hoisted here from internal/app
// by change 0466). Change 0333 moved the slow real-git, subprocess, and
// process-lifecycle corpus behind `//go:build integration`; this guard makes that
// partition an enforced invariant for a package: its default-tag test corpus never
// starts a real `git`. Installed from the TestMain of internal/app,
// internal/repository/transaction, and internal/workspace.
//
// Mechanism (keyed on the exec itself, never on spellings): InstallNoGitGuard,
// called from TestMain before m.Run, puts a directory holding a refusing `git` shim
// at the FRONT of PATH. Every PATH-resolved route to git (gitcli.NewClient's
// exec.LookPath, a bare exec.Command("git", …), a fixture helper, a child process
// inheriting PATH) resolves the shim. Known limits, none used by default tests
// today: a client built with gitcli.WithExecutable(<absolute path>), a test that
// replaces PATH wholesale rather than prepending to it, and a detached child that
// runs git after m.Run returns (once the shim dir is removed) all bypass the shim.
// The shim exits NoGitGuardExit with NoGitGuardDiagnostic(pkg) on stderr AND
// appends "<cwd>\t<argv>" to a violation log, so a test that tolerates the failure
// still turns the package red when NoGitVerdict reads the log after m.Run.
//
// Only a package's own proving tests may call the shim without recording a
// violation, by passing NoGitGuardProbeArg as the first argument.
//
// Build split: InstallNoGitGuard is real only in the default build
// (nogit_install.go, `//go:build !integration && !e2e`); the tagged corpora exist
// to run real git and get the identity finisher (nogit_install_off.go). A build
// tag applies to every package compiled into the test binary, testsupport
// included, so the split lives here once instead of as a twin file in each
// guarded package.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// NoGitGuardProbeArg as git's first argument marks a proving-test call the
	// shim refuses without logging a violation.
	NoGitGuardProbeArg = "docket-nogit-guard-probe"
	// NoGitGuardExit is the shim's exit code.
	NoGitGuardExit = 97
	// NoGitSelfProbeEnv routes NoGitGuardTolerantProbe's re-exec'd child.
	NoGitSelfProbeEnv = "DOCKET_NOGIT_GUARD_SELF_PROBE"
)

// noGitGuardDir is the installed shim directory ("" when the guard is not installed).
var noGitGuardDir string

// NoGitGuardDir returns the installed shim directory, or "" when no guard is installed.
func NoGitGuardDir() string { return noGitGuardDir }

// NoGitGuardDiagnostic is the guard's stderr diagnostic for package pkg.
func NoGitGuardDiagnostic(pkg string) string {
	return "docket nogit guard: default " + pkg + " tests must not run real git"
}

// validateNoGitGuardArgs refuses a pkg or shardGlob the shim's single-quoted printf
// cannot carry verbatim: empty, or holding a quote, percent sign, backslash, or newline.
func validateNoGitGuardArgs(pkg, shardGlob string) error {
	for _, v := range []struct{ name, val string }{{"package", pkg}, {"shard glob", shardGlob}} {
		if v.val == "" {
			return fmt.Errorf("the %s is empty", v.name)
		}
		if strings.ContainsAny(v.val, "'%\\\n") {
			return fmt.Errorf("the %s %q contains a quote, percent sign, backslash, or newline, which the shim's single-quoted printf cannot carry", v.name, v.val)
		}
	}
	return nil
}

// NoGitShimScript renders the refusing git for package pkg: it records every
// non-probe invocation as "<cwd>\t<argv>" in logPath and always exits
// NoGitGuardExit with the diagnostic and the remedy (naming shardGlob) on stderr.
func NoGitShimScript(pkg, shardGlob, logPath string) string {
	return "#!/bin/sh\n" +
		"if [ \"${1-}\" != '" + NoGitGuardProbeArg + "' ]; then\n" +
		"  printf '%s\\t%s\\n' \"$PWD\" \"$*\" >> '" + logPath + "'\n" +
		"fi\n" +
		"printf '%s (git %s): move the test behind //go:build integration with a TestIntegration prefix and a " + shardGlob + " shard (change 0465; partition from change 0333)\\n' '" +
		NoGitGuardDiagnostic(pkg) + "' \"$*\" >&2\n" +
		fmt.Sprintf("exit %d\n", NoGitGuardExit)
}

// NoGitVerdict folds the violation log into m.Run's exit code. A missing or empty
// log is clean and leaves code unchanged. Any recorded attempt, or a log that exists
// but cannot be read, fails the package: a probe error is never clean absence.
func NoGitVerdict(pkg, logPath string, code int, w io.Writer) int {
	diag := NoGitGuardDiagnostic(pkg)
	raw, err := os.ReadFile(logPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(w, "%s: cannot read the violation log %s: %v\n", diag, logPath, err)
		return noGitFailCode(code)
	}
	var lines []string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) == 0 {
		return code
	}
	fmt.Fprintf(w, "%s: %d real-git exec attempt(s) reached the guard shim (a test that tolerated the failure still counts); <cwd>\\t<argv>:\n", diag, len(lines))
	for _, l := range lines {
		fmt.Fprintf(w, "  %s\n", l)
	}
	return noGitFailCode(code)
}

func noGitFailCode(code int) int {
	if code == 0 {
		return 1
	}
	return code
}

// AssertNoGitGuardShadowsGit: every PATH lookup of `git` (gitcli.NewClient uses
// exec.LookPath) resolves the installed shim, not a real git.
func AssertNoGitGuardShadowsGit(t testing.TB) {
	t.Helper()
	if noGitGuardDir == "" {
		t.Fatalf("the no-real-git guard is not installed (NoGitGuardDir empty); TestMain must call testsupport.InstallNoGitGuard before m.Run")
	}
	p, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath(git): %v", err)
	}
	if filepath.Dir(p) != noGitGuardDir {
		t.Fatalf("git resolves to %q, want the guard shim in %q", p, noGitGuardDir)
	}
}

// AssertNoGitGuardRefusesBareExec pins the MECHANISM, not just "it failed": real
// git also fails on an unknown subcommand, so the assert is the guard's exit code
// AND pkg's diagnostic (learning assert-pins-outcome-not-mechanism).
func AssertNoGitGuardRefusesBareExec(t testing.TB, pkg string) {
	t.Helper()
	out, err := exec.Command("git", NoGitGuardProbeArg).CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != NoGitGuardExit {
		t.Fatalf("git exec must exit %d from the guard shim, got err=%v output=%q", NoGitGuardExit, err, out)
	}
	if !strings.Contains(string(out), NoGitGuardDiagnostic(pkg)) {
		t.Fatalf("git exec output must carry the guard diagnostic %q, got %q", NoGitGuardDiagnostic(pkg), out)
	}
}

// NoGitGuardTolerantProbe proves a test that SWALLOWS the git failure still fails
// the package. testName must be the calling test's exact name: the helper re-execs
// the test binary running only that test in child mode, where it runs `git status`
// and ignores the error, then asserts the child binary exits non-zero and lists the
// violation.
func NoGitGuardTolerantProbe(t *testing.T, pkg, testName string) {
	t.Helper()
	if os.Getenv(NoGitSelfProbeEnv) == "1" {
		_ = exec.Command("git", "status").Run() // tolerated on purpose
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+testName+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), NoGitSelfProbeEnv+"=1")
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == 0 {
		t.Fatalf("a package whose test tolerated a git exec must exit non-zero, got err=%v output:\n%s", err, out)
	}
	for _, want := range []string{NoGitGuardDiagnostic(pkg), "1 real-git exec attempt(s)", "\tstatus"} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("child output must contain %q, got:\n%s", want, out)
		}
	}
}
```

Create `internal/testsupport/nogit_install.go`:

```go
//go:build !integration && !e2e

package testsupport

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// InstallNoGitGuard installs the refusing git shim for package pkg (its
// module-relative dir, e.g. "internal/app") at the front of PATH and returns the
// finisher TestMain wraps around m.Run. shardGlob names the package's integration
// shard runners in the remedy text. Setup failure, or a pkg/shardGlob the shim
// cannot carry, exits the binary non-zero: a guard that silently failed to install
// would certify nothing.
func InstallNoGitGuard(pkg, shardGlob string) func(code int) int {
	diag := NoGitGuardDiagnostic(pkg)
	if err := validateNoGitGuardArgs(pkg, shardGlob); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", diag, err)
		os.Exit(1)
	}
	// tempdir-exempt: TestMain installs the shim for the whole package run; there is no t to own a fixture dir.
	dir, err := os.MkdirTemp("", "docket-"+path.Base(pkg)+"-nogit-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot create the shim directory: %v\n", diag, err)
		os.Exit(1)
	}
	logPath := filepath.Join(dir, "violations.log")
	if strings.ContainsAny(logPath, "'\n") {
		fmt.Fprintf(os.Stderr, "%s: shim log path %q is not single-quote safe\n", diag, logPath)
		os.Exit(1)
	}
	shim := filepath.Join(dir, "git")
	if err := os.WriteFile(shim, []byte(NoGitShimScript(pkg, shardGlob, logPath)), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot write the shim: %v\n", diag, err)
		os.Exit(1)
	}
	// Explicit chmod: a create-time mode is masked by the umask.
	if err := os.Chmod(shim, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot chmod the shim: %v\n", diag, err)
		os.Exit(1)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot prepend the shim to PATH: %v\n", diag, err)
		os.Exit(1)
	}
	noGitGuardDir = dir
	return func(code int) int {
		verdict := NoGitVerdict(pkg, logPath, code, os.Stderr)
		_ = os.RemoveAll(dir)
		return verdict
	}
}
```

Create `internal/testsupport/nogit_install_off.go`:

```go
//go:build integration || e2e

package testsupport

// InstallNoGitGuard is the tagged builds' no-op twin of the default-build guard in
// nogit_install.go (change 0466, formerly internal/app/nogit_guard_off_test.go).
// The integration and e2e corpora exist to run real git, so they install no shim
// and the finisher returns m.Run's code as-is. Exactly one of the two files
// compiles for any tag set, so every guarded TestMain stays single-sourced.
func InstallNoGitGuard(pkg, shardGlob string) func(code int) int {
	return func(code int) int { return code }
}
```

- [ ] **Step 5: Run the testsupport tests to verify they pass**

Run: `gofmt -l internal/testsupport && go vet ./internal/testsupport/ && go vet -tags integration ./internal/testsupport/ && go test -count=1 -v -run 'NoGit' ./internal/testsupport/`
Expected: `gofmt -l` prints nothing, both vets are clean, and `TestNoGitShimScriptKeepsInternalAppBytes`, `TestNoGitShimScriptRefusesAndLogs`, `TestNoGitVerdict`, and `TestValidateNoGitGuardArgs` PASS.

- [ ] **Step 6: Switch `internal/app` to the shared guard**

(a) `git rm -q internal/app/nogit_guard_off_test.go`.

(b) Replace the whole of `internal/app/nogit_guard_test.go` with:

```go
//go:build !integration && !e2e

package app

// The no-real-git guard's proving tests for internal/app (change 0465). The guard
// itself lives in internal/testsupport (InstallNoGitGuard, hoisted there by change
// 0466) and is installed from TestMain in gate_test.go. These tests prove it is
// installed in THIS package's binary and fails the package on any real-git exec,
// even one a test tolerates. The default-tag internal/app test corpus never starts
// a real `git`; real-git tests live behind //go:build integration in the
// tests/test_go_integration_app_*.sh shards.

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` resolves the shim.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	testsupport.AssertNoGitGuardShadowsGit(t)
}

// TestNoGitGuardRefusesBareExec: a bare git exec gets the guard's exit code and diagnostic.
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	testsupport.AssertNoGitGuardRefusesBareExec(t, nogitPkg)
}

// TestNoGitGuardFailsTolerantTest: a test that swallows the git failure still fails the package.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	testsupport.NoGitGuardTolerantProbe(t, nogitPkg, "TestNoGitGuardFailsTolerantTest")
}
```

(c) In `internal/app/gate_test.go`, add directly above `func TestMain`:

```go
// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (testsupport.InstallNoGitGuard): its diagnostic and its remedy text.
const (
	nogitPkg       = "internal/app"
	nogitShardGlob = "tests/test_go_integration_app_*.sh"
)
```

Then in `TestMain` replace

```go
	// Change 0465: the default build installs the no-real-git guard (nogit_guard_test.go)
	// AFTER the re-exec routing above, so the supervisor and guardian roles behave
	// exactly as before; tagged builds get the no-op twin (nogit_guard_off_test.go).
	finish := installNoGitGuard()
```

with

```go
	// Change 0465 (hoisted by change 0466): the default build installs the no-real-git
	// guard (testsupport.InstallNoGitGuard) AFTER the re-exec routing above, so the
	// supervisor and guardian roles behave exactly as before; tagged builds get
	// testsupport's no-op twin. Its proving tests are in nogit_guard_test.go.
	finish := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
```

Also update the doc comment line above `TestMain` that reads `// Ordinary runs then install the default-build no-real-git guard (change 0465) around m.Run.` so it names `testsupport.InstallNoGitGuard`.

(d) Find every other maintained reference to the removed names and fix it (comments included):

```bash
bash -c 'git grep -n -w -E -e "installNoGitGuard|nogitVerdict|nogitShimScript|nogitGuardDir|nogitGuardProbeArg|nogitGuardDiagnostic|nogitGuardExit|nogitSelfProbeEnv|nogitFailCode|nogit_guard_off_test" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs" || echo "no stale references"'
```
Expected after your edits: `no stale references`, except the historical prose in `tests/test_go_race.sh`'s header, which Task 10 rewrites. Leave that one for Task 10.

- [ ] **Step 7: Verify `internal/app` behaves identically**

```bash
gofmt -l internal/app internal/testsupport
go vet ./internal/app/ && go vet -tags integration ./internal/app/ && go vet -tags e2e ./internal/app/
go test -count=1 -v -run '^TestNoGitGuard' ./internal/app/
bash -c 'out="$(go test -count=1 ./internal/app/ 2>&1)"; rc=$?; printf "%s\n" "$out" | tail -3; grep -E -e "real-git exec attempt\(s\)|docket nogit guard" <<<"$out" || echo "guard: silent"; echo "rc=$rc"'
bash tests/test_go_finalize_e2e.sh >/dev/null 2>&1; echo "e2e rc=$?"
bash tests/test_go_integration_app_reposetup.sh >/dev/null 2>&1; echo "app shard rc=$?"
```
Expected: no gofmt output, three clean vets, three `TestNoGitGuard*` PASS, `ok  github.com/danielhanold/docket/internal/app`, `guard: silent`, `rc=0`, `e2e rc=0`, `app shard rc=0`. The e2e and shard runs prove the tagged builds still get no shim.

- [ ] **Step 8: Mutation probes**

```bash
bash -c '
f=internal/testsupport/nogit_install.go; g=internal/testsupport/nogit.go
bf="$(mktemp "${TMPDIR:-/tmp}/nogit-install.XXXXXX")"; bg="$(mktemp "${TMPDIR:-/tmp}/nogit-shared.XXXXXX")"; cp "$f" "$bf"; cp "$g" "$bg"
# (a) strip the PATH prepend: shadow + bare-exec + tolerant-child must go red in internal/app
perl -0pi -e "s/os\.Setenv\(\"PATH\", dir\+string\(os\.PathListSeparator\)\+os\.Getenv\(\"PATH\"\)\)/os.Setenv(\"DOCKET_NOGIT_MUTATED\", dir)/" "$f"
cmp -s "$f" "$bf" && { echo "mutation a did not land"; exit 1; }
go test -count=1 -run "^TestNoGitGuard" ./internal/app/ >/dev/null 2>&1; echo "mutation-a rc=$?"
cp "$bf" "$f"
# (b) make the verdict ignore violations: TestNoGitVerdict + the app tolerant child must go red
perl -0pi -e "s/if len\(lines\) == 0 \{\n\t\treturn code\n\t\}/if len(lines) >= 0 {\n\t\treturn code\n\t}/" "$g"
cmp -s "$g" "$bg" && { echo "mutation b did not land"; exit 1; }
go test -count=1 -run "^TestNoGitVerdict$" ./internal/testsupport/ >/dev/null 2>&1; echo "mutation-b testsupport rc=$?"
go test -count=1 -run "^TestNoGitGuardFailsTolerantTest$" ./internal/app/ >/dev/null 2>&1; echo "mutation-b app rc=$?"
cp "$bg" "$g"
go test -count=1 -run "NoGit" ./internal/testsupport/ ./internal/app/ >/dev/null 2>&1; echo "restored rc=$?"
rm -f "$bf" "$bg"
'
```
Expected: `mutation-a rc=1`, `mutation-b testsupport rc=1`, `mutation-b app rc=1`, `restored rc=0`, and no `did not land` line. If a `did not land` line prints, the perl pattern no longer matches the file: fix the probe, never read the run as a result. Report all four codes.

- [ ] **Step 9: Commit**

```bash
git add -- internal/testsupport/nogit.go internal/testsupport/nogit_install.go internal/testsupport/nogit_install_off.go internal/testsupport/nogit_test.go internal/app/nogit_guard_test.go internal/app/nogit_guard_off_test.go internal/app/gate_test.go
git status --porcelain
git commit -m "test(testsupport): hoist the no-real-git guard out of internal/app (change 0466)"
```

---

### Task 2: Move transaction's apply-path real-git tests into the `TestIntegrationTxnApply` shard

**Build profile:** standard

**Files:**
- Modify: `internal/repository/transaction/{engine,engine_scope,candidate,loader,harness,preserve}_test.go` (tests move out; helpers stay)
- Create: `internal/repository/transaction/{engine,engine_scope,candidate,loader,harness,preserve}_integration_test.go`
- Create: `tests/test_go_integration_transaction_apply.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row)
- Modify: any maintained file `rename-tests.sh` rewrites

**Interfaces:**
- Consumes: the Shared tools; `tests/lib/go-integration-shard.sh` and `tests/test_go_integration_contract.sh`, unchanged.
- Produces: runner `tests/test_go_integration_transaction_apply.sh` (`SHARD_PKG="./internal/repository/transaction"`, `SHARD_PREFIX="TestIntegrationTxnApply"`, `SHARD_MODE="normal"`). Transaction test files named `<base>_integration_test.go` exist for Tasks 3–5 to append to.

**Context.** Moving tests out one shard at a time keeps the package green. The guard is installed only in Task 5, after all transaction offenders have moved. Helpers stay untagged until Task 5 relocates the tagged-only ones.

**This task's tests (snapshot, all `normal`, all start real git): 25**
- `engine_test.go` (10): `TestEngineAfterGateRefusesInvalidPlan`, `TestEngineAppliesHappyPath`, `TestEngineBeforeGateRefusesInvalidBase`, `TestEngineEvolutionGateBlocksFrozenADRRewrite`, `TestEngineExpectationMatrix`, `TestEngineFailsOnMissingTargetBranch`, `TestEngineNoOpOnEmptyPlan`, `TestEngineRefusalFromOperation`, `TestEngineRejectsNonBranchTargetRef`, `TestEngineRetriesLeaseLoss`
- `engine_scope_test.go` (10): `TestEngineScopeAfterGateVolatility`, `TestEngineScopeAppliesDespiteUnrelatedError`, `TestEngineScopeCandidateSubjectsAreResolved`, `TestEngineScopeKeyedReplay`, `TestEngineScopeLeaseRetryRecomputes`, `TestEngineScopeNoOpSurfacesUnrelatedError`, `TestEngineScopePostPlanRecheckRefusesPlanTouchedError`, `TestEngineScopeRefusesRelevantBeforeErrors`, `TestEngineScopeUnresolvableCandidateIsStrict`, `TestEngineScopeUnresolvableScopeIsStrict`
- `candidate_test.go` (2): `TestAllocateCandidateStructureAndManifest`, `TestCandidateModesUnderUmask`
- `loader_test.go` (1): `TestLoaderBuildsCleanStateFromCorpus`
- `harness_test.go` (1): `TestHarnessBuildersProduceExpectedTopology`
- `preserve_test.go` (1): `TestTransactionPreservesDirtyCheckout`

Stays in the default corpus (no git): `TestNewEngineRejectsNilDependencies`, `TestLoaderErrorsOnUnparseableRecord`, `TestLiveLockExcludesSecondNonBlocking`, `TestRegistryLockMutualExclusion`. `TestEngineConcurrentExecuteIsRaceFree`, `TestSetPhaseAtomicUnderConcurrentReads`, and `TestRegistryLockAllocationExcludesConcurrentAllocation` also start git, but they belong to Task 4's race shard. Do not touch them here.

- [ ] **Step 1: Confirm the live census**

Write the Shared tools to `T="${TMPDIR:-/tmp}/docket-0466-tools"` if absent (`mkdir -p "$T"`, one file per block, `chmod +x "$T"/*.sh`). Then:
```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/census.sh" internal/repository/transaction engine_test.go engine_scope_test.go candidate_test.go loader_test.go harness_test.go preserve_test.go'
```
Expected: every test listed above prints a git-exec count `> 0`. The four "stays" tests print `0`. The three race-shard tests print `> 0` and stay untouched. If the live census differs (a test added or renamed since the snapshot), the live census wins: move every live `> 0` test of these files that is not race-classified under the Global Constraints criterion, and report the difference.

- [ ] **Step 2: Move the tests behind the tag**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/repository/transaction
"$T/movefuncs.sh" $P/engine_test.go $P/engine_integration_test.go TestEngineAfterGateRefusesInvalidPlan TestEngineAppliesHappyPath TestEngineBeforeGateRefusesInvalidBase TestEngineEvolutionGateBlocksFrozenADRRewrite TestEngineExpectationMatrix TestEngineFailsOnMissingTargetBranch TestEngineNoOpOnEmptyPlan TestEngineRefusalFromOperation TestEngineRejectsNonBranchTargetRef TestEngineRetriesLeaseLoss
"$T/movefuncs.sh" $P/engine_scope_test.go $P/engine_scope_integration_test.go TestEngineScopeAfterGateVolatility TestEngineScopeAppliesDespiteUnrelatedError TestEngineScopeCandidateSubjectsAreResolved TestEngineScopeKeyedReplay TestEngineScopeLeaseRetryRecomputes TestEngineScopeNoOpSurfacesUnrelatedError TestEngineScopePostPlanRecheckRefusesPlanTouchedError TestEngineScopeRefusesRelevantBeforeErrors TestEngineScopeUnresolvableCandidateIsStrict TestEngineScopeUnresolvableScopeIsStrict
"$T/movefuncs.sh" $P/candidate_test.go $P/candidate_integration_test.go TestAllocateCandidateStructureAndManifest TestCandidateModesUnderUmask
"$T/movefuncs.sh" $P/loader_test.go $P/loader_integration_test.go TestLoaderBuildsCleanStateFromCorpus
"$T/movefuncs.sh" $P/harness_test.go $P/harness_integration_test.go TestHarnessBuildersProduceExpectedTopology
"$T/movefuncs.sh" $P/preserve_test.go $P/preserve_integration_test.go TestTransactionPreservesDirtyCheckout
"$T/prune-imports.sh" $P
for f in $P/*_integration_test.go; do printf "%s: [%s] [%s]\n" "$f" "$(sed -n 1p "$f")" "$(sed -n 2p "$f")"; done
'
```
Expected: `prune-imports: go vet clean (default and integration) for internal/repository/transaction`, and every listed file shows `[//go:build integration] []`. An untagged file left holding only a package clause and imports (for example `preserve_test.go` if it has no helpers) is deleted with `git rm -q -f <file>`. It carries nothing. Re-run `prune-imports.sh` after any deletion.

- [ ] **Step 3: Rename the moved tests and every maintained reference**

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; X=TestIntegrationTxnApply; pairs=""
for old in TestEngineAfterGateRefusesInvalidPlan TestEngineAppliesHappyPath TestEngineBeforeGateRefusesInvalidBase TestEngineEvolutionGateBlocksFrozenADRRewrite TestEngineExpectationMatrix TestEngineFailsOnMissingTargetBranch TestEngineNoOpOnEmptyPlan TestEngineRefusalFromOperation TestEngineRejectsNonBranchTargetRef TestEngineRetriesLeaseLoss TestEngineScopeAfterGateVolatility TestEngineScopeAppliesDespiteUnrelatedError TestEngineScopeCandidateSubjectsAreResolved TestEngineScopeKeyedReplay TestEngineScopeLeaseRetryRecomputes TestEngineScopeNoOpSurfacesUnrelatedError TestEngineScopePostPlanRecheckRefusesPlanTouchedError TestEngineScopeRefusesRelevantBeforeErrors TestEngineScopeUnresolvableCandidateIsStrict TestEngineScopeUnresolvableScopeIsStrict TestAllocateCandidateStructureAndManifest TestCandidateModesUnderUmask TestLoaderBuildsCleanStateFromCorpus TestTransactionPreservesDirtyCheckout; do pairs="$pairs $old=$X${old#Test}"; done
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
# declared in internal/gitcli and internal/workspace too: rename ONLY this package copy
"$T/rename-tests.sh" --scope internal/repository/transaction TestHarnessBuildersProduceExpectedTopology=${X}HarnessBuildersProduceExpectedTopology; echo "scoped rename rc=$?"
git grep -l -E -e "^func TestHarnessBuildersProduceExpectedTopology\(" -- internal
git diff --stat -- . ":!internal/repository/transaction"
'
```
Expected: `rename rc=0` and `scoped rename rc=0`, with no `STILL REFERENCED` line. The declaration grep still lists the `internal/gitcli` and `internal/workspace` copies, untouched. The `\b` word boundary keeps a name from matching inside a longer one (e.g. `TestEngineScopeKeyedReplay`), so list order does not matter. Inspect every file the `--stat` lists outside the package. Where a comment or doc called the test a default-corpus test, correct the wording, not only the name.

- [ ] **Step 4: Create the shard runner**

Create `tests/test_go_integration_transaction_apply.sh` with exactly this content, then `chmod +x tests/test_go_integration_transaction_apply.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_transaction_apply.sh — Go integration shard (change 0466, extending
# change 0333's partition): the transaction engine's apply-path real-git tests (engine, scope
# gates, candidate allocation, loader, harness topology, dirty-checkout preservation) — moved
# out of the default internal/repository/transaction corpus, which must never start real git
# (testsupport.InstallNoGitGuard, installed from the package's TestMain) — behind the
# `integration` build tag, prefix ^TestIntegrationTxnApply. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/repository/transaction"
SHARD_PREFIX="TestIntegrationTxnApply"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

(The `assert` line is byte-identical to the canonical helper. `internal/repoguard`'s source-hygiene rule (a) allowlists it byte for byte, so copy it exactly.)

- [ ] **Step 5: Verify: default package green, shard green, contract green, no prefix collision**

```bash
go test -count=1 ./internal/repository/transaction/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/repository/transaction/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
bash tests/test_go_integration_transaction_apply.sh; echo "shard rc=$?"
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/repository/transaction"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: `ok` for the package, `no leak into the default corpus`, the shard prints only `ok - ` lines with `shard rc=0`, `contract rc=0`, and no `COLLISION` line. The contract now discovers `internal/repository/transaction` by itself, and the transaction package's runner set is exactly this one runner. The contract's check (10) passes because this runner declares the package.

- [ ] **Step 6: Measure and register the budget row**

Run the shard once to warm the build cache, then measure the second, solo run: `bash -c 'bash tests/test_go_integration_transaction_apply.sh >/dev/null 2>&1; time bash tests/test_go_integration_transaction_apply.sh >/dev/null' 2>&1 | tail -4`. Take `real` = S seconds. The row is S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert `tests/test_go_integration_transaction_apply.sh<TAB><row><TAB>parallel` directly above the `tests/test_go_integration_release.sh` row, with a literal tab. A row above 40: stop and return NEEDS_ESCALATION with S (see Global Constraints). Then: `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS. Report S, the row, and the margin (row − S).

- [ ] **Step 7: Commit**

```bash
git add -- internal/repository/transaction tests/test_go_integration_transaction_apply.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 3 rewrote; list it explicitly
git status --porcelain
git commit -m "test(transaction): move apply-path real-git tests behind the integration tag (TestIntegrationTxnApply, change 0466)"
```

---

### Task 3: Move transaction's replay and recovery real-git tests into the `TestIntegrationTxnRecovery` shard

**Build profile:** standard

**Files:**
- Modify: `internal/repository/transaction/{idempotency,materialize,cleanup,interrupt,recovery}_test.go`
- Create: `internal/repository/transaction/{idempotency,materialize,cleanup,interrupt,recovery}_integration_test.go`
- Create: `tests/test_go_integration_transaction_recovery.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row)
- Modify: any maintained file `rename-tests.sh` rewrites

**Interfaces:**
- Consumes: the Shared tools; Task 2's runner (for the prefix check).
- Produces: runner `tests/test_go_integration_transaction_recovery.sh` (`SHARD_PKG="./internal/repository/transaction"`, `SHARD_PREFIX="TestIntegrationTxnRecovery"`, `SHARD_MODE="normal"`).

**This task's tests (snapshot, all `normal`, all start real git): 31**
- `idempotency_test.go` (7): `TestDuplicateRequestIDIsInvalidState`, `TestKeyedCommitCarriesFiveTrailers`, `TestKeyedReplayFoundDeepInHistory`, `TestKeyedReplayReturnsOriginalReceipt`, `TestMalformedResultIsInvalidState`, `TestRequestIDInProseDoesNotMatch`, `TestRequestIDReusedDifferentDigest`
- `materialize_test.go` (10): `TestMaterializeCreateEmptyFile`, `TestMaterializeCreateReplaceDeleteByteExact`, `TestMaterializeHostilePathsByteExact`, `TestMaterializeRefusesReplaceTargetSymlink`, `TestMaterializeRefusesSymlinkParentComponent`, `TestMaterializeReplacePreservesExecutableMode`, `TestVerifyActualDeltaExactMatch`, `TestVerifyActualDeltaRejectsDeclaredUnchanged`, `TestVerifyActualDeltaRejectsUndeclaredChange`, `TestVerifyMaterializedDetectsCorruption`
- `cleanup_test.go` (4): `TestCleanupRetainsRegisteredCandidateOnListError`, `TestPruneDocketModeIgnoresLinkedWorktree`, `TestPruneReportDeterministicOrder`, `TestPruneReportEmptyOnCleanRoot`
- `interrupt_test.go` (4): `TestInterruptContainmentFailureDoesNotPush`, `TestInterruptDeltaMismatchDoesNotPush`, `TestInterruptLostResponseReplaysOriginalOnce`, `TestInterruptPreCancelledContext`
- `recovery_test.go` (6): `TestPruneHeldLockBeatsAncientPIDAndTimestamp`, `TestPruneLeavesMalformedAndForeignByteUntouched`, `TestPruneNeverGlobalPrunesOrTouchesUserCheckout`, `TestPrunePrunesAbandonedCandidate`, `TestPruneReportsCleanupFailedOnForcedRemovalFailure`, `TestPruneReportsLiveCandidatesUntouched`

Belong to Task 4 (race), do not touch: `TestInterruptAmbiguousPushLandedIsApplied`, `TestInterruptCancelBetweenCommitAndPush`, `TestInterruptCancelInsidePlan`, `TestInterruptLiteralLeaseRejectsFresherTrackingRef` (each runs `eng.Execute` in a goroutine while the test goroutine cancels or races it), and `TestPruneRegistryLockSerializesAllocation` (a goroutine holds the registry lock while another runs `PruneAbandoned`).

- [ ] **Step 1: Confirm the live census**

Write the Shared tools to `${TMPDIR:-/tmp}/docket-0466-tools` if absent. Then:
```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/census.sh" internal/repository/transaction idempotency_test.go materialize_test.go cleanup_test.go interrupt_test.go recovery_test.go'
```
Expected: all 36 tests print `> 0` git execs (31 here plus Task 4's five). The live census wins, as in Task 2.

- [ ] **Step 2: Move the tests behind the tag**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/repository/transaction
"$T/movefuncs.sh" $P/idempotency_test.go $P/idempotency_integration_test.go TestDuplicateRequestIDIsInvalidState TestKeyedCommitCarriesFiveTrailers TestKeyedReplayFoundDeepInHistory TestKeyedReplayReturnsOriginalReceipt TestMalformedResultIsInvalidState TestRequestIDInProseDoesNotMatch TestRequestIDReusedDifferentDigest
"$T/movefuncs.sh" $P/materialize_test.go $P/materialize_integration_test.go TestMaterializeCreateEmptyFile TestMaterializeCreateReplaceDeleteByteExact TestMaterializeHostilePathsByteExact TestMaterializeRefusesReplaceTargetSymlink TestMaterializeRefusesSymlinkParentComponent TestMaterializeReplacePreservesExecutableMode TestVerifyActualDeltaExactMatch TestVerifyActualDeltaRejectsDeclaredUnchanged TestVerifyActualDeltaRejectsUndeclaredChange TestVerifyMaterializedDetectsCorruption
"$T/movefuncs.sh" $P/cleanup_test.go $P/cleanup_integration_test.go TestCleanupRetainsRegisteredCandidateOnListError TestPruneDocketModeIgnoresLinkedWorktree TestPruneReportDeterministicOrder TestPruneReportEmptyOnCleanRoot
"$T/movefuncs.sh" $P/interrupt_test.go $P/interrupt_integration_test.go TestInterruptContainmentFailureDoesNotPush TestInterruptDeltaMismatchDoesNotPush TestInterruptLostResponseReplaysOriginalOnce TestInterruptPreCancelledContext
"$T/movefuncs.sh" $P/recovery_test.go $P/recovery_integration_test.go TestPruneHeldLockBeatsAncientPIDAndTimestamp TestPruneLeavesMalformedAndForeignByteUntouched TestPruneNeverGlobalPrunesOrTouchesUserCheckout TestPrunePrunesAbandonedCandidate TestPruneReportsCleanupFailedOnForcedRemovalFailure TestPruneReportsLiveCandidatesUntouched
"$T/prune-imports.sh" $P
for f in $P/*_integration_test.go; do printf "%s: [%s] [%s]\n" "$f" "$(sed -n 1p "$f")" "$(sed -n 2p "$f")"; done
'
```
Expected: `prune-imports: go vet clean …`, every file `[//go:build integration] []`. Delete any untagged file left with only a package clause and imports (`git rm -q -f`), then re-run `prune-imports.sh`.

- [ ] **Step 3: Rename the moved tests and every maintained reference**

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; X=TestIntegrationTxnRecovery; pairs=""
for old in TestDuplicateRequestIDIsInvalidState TestKeyedCommitCarriesFiveTrailers TestKeyedReplayFoundDeepInHistory TestKeyedReplayReturnsOriginalReceipt TestMalformedResultIsInvalidState TestRequestIDInProseDoesNotMatch TestRequestIDReusedDifferentDigest TestMaterializeCreateEmptyFile TestMaterializeCreateReplaceDeleteByteExact TestMaterializeHostilePathsByteExact TestMaterializeRefusesReplaceTargetSymlink TestMaterializeRefusesSymlinkParentComponent TestMaterializeReplacePreservesExecutableMode TestVerifyActualDeltaExactMatch TestVerifyActualDeltaRejectsDeclaredUnchanged TestVerifyActualDeltaRejectsUndeclaredChange TestVerifyMaterializedDetectsCorruption TestCleanupRetainsRegisteredCandidateOnListError TestPruneDocketModeIgnoresLinkedWorktree TestPruneReportDeterministicOrder TestPruneReportEmptyOnCleanRoot TestInterruptContainmentFailureDoesNotPush TestInterruptDeltaMismatchDoesNotPush TestInterruptLostResponseReplaysOriginalOnce TestInterruptPreCancelledContext TestPruneHeldLockBeatsAncientPIDAndTimestamp TestPruneLeavesMalformedAndForeignByteUntouched TestPruneNeverGlobalPrunesOrTouchesUserCheckout TestPrunePrunesAbandonedCandidate TestPruneReportsCleanupFailedOnForcedRemovalFailure TestPruneReportsLiveCandidatesUntouched; do pairs="$pairs $old=$X${old#Test}"; done
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
git diff --stat -- . ":!internal/repository/transaction"
'
```
Expected: `rename rc=0`, no `STILL REFERENCED`. Correct the wording of any reference outside the package, as in Task 2 Step 3.

- [ ] **Step 4: Create the shard runner**

Create `tests/test_go_integration_transaction_recovery.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_transaction_recovery.sh — Go integration shard (change 0466, extending
# change 0333's partition): the transaction engine's replay, materialization, interruption,
# cleanup, and abandoned-candidate recovery real-git tests — moved out of the default
# internal/repository/transaction corpus, which must never start real git
# (testsupport.InstallNoGitGuard, installed from the package's TestMain) — behind the
# `integration` build tag, prefix ^TestIntegrationTxnRecovery. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/repository/transaction"
SHARD_PREFIX="TestIntegrationTxnRecovery"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 5: Verify: default package green, shards green, contract green, no prefix collision**

```bash
go test -count=1 ./internal/repository/transaction/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/repository/transaction/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
bash tests/test_go_integration_transaction_recovery.sh; echo "shard rc=$?"
bash tests/test_go_integration_transaction_apply.sh >/dev/null; echo "apply shard rc=$?"
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/repository/transaction"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: package `ok`, `no leak…`, both shards `rc=0` with only `ok - ` lines, `contract rc=0`, no `COLLISION`, and `prefixes: TestIntegrationTxnApply TestIntegrationTxnRecovery` (in either order).

- [ ] **Step 6: Measure and register the budget row**

Warm, then measure the second solo run: `bash -c 'bash tests/test_go_integration_transaction_recovery.sh >/dev/null 2>&1; time bash tests/test_go_integration_transaction_recovery.sh >/dev/null' 2>&1 | tail -4`. Row = S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert `tests/test_go_integration_transaction_recovery.sh<TAB><row><TAB>parallel` directly above the `tests/test_go_integration_release.sh` row. A row above 40 means NEEDS_ESCALATION. Then run `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS. Report S, the row, and the margin.

- [ ] **Step 7: Commit**

```bash
git add -- internal/repository/transaction tests/test_go_integration_transaction_recovery.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 3 rewrote
git status --porcelain
git commit -m "test(transaction): move replay and recovery real-git tests behind the integration tag (TestIntegrationTxnRecovery, change 0466)"
```

---

### Task 4: Move transaction's concurrency real-git tests into the `TestRaceIntegrationTxn` race shard

**Build profile:** standard

**Files:**
- Modify: `internal/repository/transaction/{concurrency,candidate,engine,interrupt,recovery}_test.go`
- Create: `internal/repository/transaction/{concurrency,candidate,engine,interrupt,recovery}_race_integration_test.go`
- Create: `tests/test_go_integration_transaction_race.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row)
- Modify: any maintained file `rename-tests.sh` rewrites

**Interfaces:**
- Consumes: the Shared tools; Tasks 2–3's runners (for the prefix check).
- Produces: runner `tests/test_go_integration_transaction_race.sh` (`SHARD_PKG="./internal/repository/transaction"`, `SHARD_PREFIX="TestRaceIntegrationTxn"`, `SHARD_MODE="race"`). After this task no transaction test starts real git in the default build, which is Task 5's precondition.

**This task's tests (snapshot, all `race`, all start real git): 12.** Each gets its rationale comment:
- `concurrency_test.go` (4): `TestConcurrencyDerivedOverlapReplansView`, `TestConcurrencyFourLeaseLossesContend`, `TestConcurrencySameEntityContends`, `TestConcurrencyUnrelatedWritersConverge`. Rationale: `concurrent writers contend on one target branch`.
- `candidate_test.go` (2): `TestSetPhaseAtomicUnderConcurrentReads` (`a reader goroutine polls the manifest while the phase is rewritten`), `TestRegistryLockAllocationExcludesConcurrentAllocation` (`two goroutines allocate candidates under the registry lock`)
- `engine_test.go` (1): `TestEngineConcurrentExecuteIsRaceFree` (`concurrent Execute calls share one engine`)
- `interrupt_test.go` (4): `TestInterruptAmbiguousPushLandedIsApplied`, `TestInterruptCancelBetweenCommitAndPush`, `TestInterruptCancelInsidePlan`, `TestInterruptLiteralLeaseRejectsFresherTrackingRef`. Rationale: `Execute runs in a goroutine while the test goroutine cancels or races it through shared hooks`.
- `recovery_test.go` (1): `TestPruneRegistryLockSerializesAllocation` (`a goroutine holds the registry lock while PruneAbandoned contends for it`)

- [ ] **Step 1: Confirm the live census**

Write the Shared tools if absent. Then run `bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/census.sh" internal/repository/transaction concurrency_test.go candidate_test.go engine_test.go interrupt_test.go recovery_test.go'`.
Expected: the 12 tests above print `> 0`. `TestLiveLockExcludesSecondNonBlocking`, `TestRegistryLockMutualExclusion`, and `TestNewEngineRejectsNilDependencies` print `0` and stay. Then prove no other transaction default test still starts git:
```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; cd internal/repository/transaction && files="$(for f in *_test.go; do case "$(sed -n 1p "$f")" in //go:build*) ;; *) printf "%s " "$f";; esac; done)"; cd - >/dev/null; out="$("$T/census.sh" internal/repository/transaction $files)"; awk "\$3 > 0" <<<"$out"'
```
Expected: exactly the 12 tests above. Any other line is a live offender. Classify it under the Global Constraints criterion, move it in this task (race) or into Task 3's recovery shard file with the `TestIntegrationTxnRecovery` prefix (normal), and report it.

- [ ] **Step 2: Move the tests behind the tag and add the rationale comments**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/repository/transaction
"$T/movefuncs.sh" $P/concurrency_test.go $P/concurrency_race_integration_test.go TestConcurrencyDerivedOverlapReplansView TestConcurrencyFourLeaseLossesContend TestConcurrencySameEntityContends TestConcurrencyUnrelatedWritersConverge
"$T/movefuncs.sh" $P/candidate_test.go $P/candidate_race_integration_test.go TestSetPhaseAtomicUnderConcurrentReads TestRegistryLockAllocationExcludesConcurrentAllocation
"$T/movefuncs.sh" $P/engine_test.go $P/engine_race_integration_test.go TestEngineConcurrentExecuteIsRaceFree
"$T/movefuncs.sh" $P/interrupt_test.go $P/interrupt_race_integration_test.go TestInterruptAmbiguousPushLandedIsApplied TestInterruptCancelBetweenCommitAndPush TestInterruptCancelInsidePlan TestInterruptLiteralLeaseRejectsFresherTrackingRef
"$T/movefuncs.sh" $P/recovery_test.go $P/recovery_race_integration_test.go TestPruneRegistryLockSerializesAllocation
"$T/prune-imports.sh" $P
'
```
Then, directly above each moved `func` line (below its doc comment), add the one-line comment `// Race shard (change 0466): <rationale from the list above>.` Re-run `gofmt -l internal/repository/transaction`. Expected: no output. Delete any untagged file left holding only a package clause and imports, and re-run `prune-imports.sh`.

- [ ] **Step 3: Rename the moved tests and every maintained reference**

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; X=TestRaceIntegrationTxn; pairs=""
for old in TestConcurrencyDerivedOverlapReplansView TestConcurrencyFourLeaseLossesContend TestConcurrencySameEntityContends TestConcurrencyUnrelatedWritersConverge TestSetPhaseAtomicUnderConcurrentReads TestRegistryLockAllocationExcludesConcurrentAllocation TestEngineConcurrentExecuteIsRaceFree TestInterruptAmbiguousPushLandedIsApplied TestInterruptCancelBetweenCommitAndPush TestInterruptCancelInsidePlan TestInterruptLiteralLeaseRejectsFresherTrackingRef TestPruneRegistryLockSerializesAllocation; do pairs="$pairs $old=$X${old#Test}"; done
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
git diff --stat -- . ":!internal/repository/transaction"
'
```
Expected: `rename rc=0`.

- [ ] **Step 4: Create the race shard runner**

Create `tests/test_go_integration_transaction_race.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_transaction_race.sh — Go integration shard (change 0466, extending
# change 0333's partition): the transaction engine's real-concurrency real-git tests (writers
# contending on one target branch, concurrent Execute, interrupt/cancel races, registry-lock
# contention) — moved out of the default internal/repository/transaction corpus, which must
# never start real git (testsupport.InstallNoGitGuard, installed from the package's TestMain) —
# behind the `integration` build tag, prefix ^TestRaceIntegrationTxn, run in RACE mode.
# Declarations only — execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/repository/transaction"
SHARD_PREFIX="TestRaceIntegrationTxn"
SHARD_MODE="race"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 5: Verify: default package green, all three shards green, contract green, no collision**

```bash
go test -count=1 ./internal/repository/transaction/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/repository/transaction/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
bash tests/test_go_integration_transaction_race.sh; echo "race shard rc=$?"
bash -c 'DOCKET_SHARD_INSPECT=1 bash tests/test_go_integration_transaction_race.sh'
for r in apply recovery; do bash tests/test_go_integration_transaction_$r.sh >/dev/null; echo "$r shard rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/repository/transaction"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: package `ok`, `no leak…`, every shard `rc=0`, the inspection prints `mode=race` and `race=-race`, `contract rc=0` (its checks (5) and (9) prove that the race prefix maps to the race runner and that the runner really passes `-race`), no `COLLISION`, and three transaction prefixes.

- [ ] **Step 6: Measure and register the budget row**

Warm, then measure: `bash -c 'bash tests/test_go_integration_transaction_race.sh >/dev/null 2>&1; time bash tests/test_go_integration_transaction_race.sh >/dev/null' 2>&1 | tail -4`. Row = S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert `tests/test_go_integration_transaction_race.sh<TAB><row><TAB>parallel` directly above the `tests/test_go_integration_release.sh` row. A row above 40 means NEEDS_ESCALATION. Then `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/` must PASS. Report S, the row, and the margin.

- [ ] **Step 7: Commit**

```bash
git add -- internal/repository/transaction tests/test_go_integration_transaction_race.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 3 rewrote
git status --porcelain
git commit -m "test(transaction): move concurrency real-git tests into a race shard (TestRaceIntegrationTxn, change 0466)"
```

---

### Task 5: Install the no-real-git guard in `internal/repository/transaction`

**Build profile:** standard

**Files:**
- Create: `internal/repository/transaction/main_test.go` (untagged: `TestMain`, `nogitPkg`, `nogitShardGlob`)
- Create: `internal/repository/transaction/nogit_guard_test.go` (`//go:build !integration && !e2e`: the three proving tests)
- Modify/Move: untagged transaction `*_test.go` helper declarations that only the tagged corpus uses, relocated behind the tag
- Modify: `internal/repository/transaction/harness_test.go` (the `useBackgroundOffGit` comment, only if its wording needs to name where the harness now runs)

**Interfaces:**
- Consumes: Task 1's `testsupport.InstallNoGitGuard`, `AssertNoGitGuardShadowsGit`, `AssertNoGitGuardRefusesBareExec`, `NoGitGuardTolerantProbe`, and `NoGitGuardDiagnostic`. Tasks 2–4 must have moved every real-git transaction test.
- Produces: the invariant "the default-tag `internal/repository/transaction` test corpus never starts a real git", enforced. Task 10 records this package's default `-race` wall time.

- [ ] **Step 1: Write the failing proving tests**

Create `internal/repository/transaction/nogit_guard_test.go`:

```go
//go:build !integration && !e2e

package transaction

// The no-real-git guard's proving tests for internal/repository/transaction (change
// 0466). The guard lives in internal/testsupport (InstallNoGitGuard) and is
// installed from TestMain in main_test.go. These tests prove it is installed in
// THIS package's binary and fails the package on any real-git exec, even one a test
// tolerates. Real-git tests live behind //go:build integration in the
// tests/test_go_integration_transaction_*.sh shards.

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` resolves the shim.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	testsupport.AssertNoGitGuardShadowsGit(t)
}

// TestNoGitGuardRefusesBareExec: a bare git exec gets the guard's exit code and diagnostic.
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	testsupport.AssertNoGitGuardRefusesBareExec(t, nogitPkg)
}

// TestNoGitGuardFailsTolerantTest: a test that swallows the git failure still fails the package.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	testsupport.NoGitGuardTolerantProbe(t, nogitPkg, "TestNoGitGuardFailsTolerantTest")
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 -run '^TestNoGitGuard' ./internal/repository/transaction/`
Expected: FAIL to compile with `undefined: nogitPkg`.

- [ ] **Step 3: Install the guard from `TestMain`**

Create `internal/repository/transaction/main_test.go`:

```go
package transaction

import (
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0466): the default-tag internal/repository/transaction test corpus never
// starts a real git; real-git tests live behind //go:build integration in the
// tests/test_go_integration_transaction_*.sh shards.
const (
	nogitPkg       = "internal/repository/transaction"
	nogitShardGlob = "tests/test_go_integration_transaction_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build; the integration build gets testsupport's identity
// finisher, so the tagged shards run real git as before.
func TestMain(m *testing.M) {
	finish := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	os.Exit(finish(m.Run()))
}
```

- [ ] **Step 4: The whole default corpus is green with the guard on**

```bash
bash -c 'out="$(go test -count=1 -v ./internal/repository/transaction/ 2>&1)"; rc=$?; grep -E -e "^--- (FAIL|PASS): TestNoGitGuard" <<<"$out"; grep -E -e "^--- FAIL" <<<"$out"; grep -E -e "real-git exec attempt\(s\)|docket nogit guard" <<<"$out" || echo "guard: silent"; echo "rc=$rc"'
```
Expected: three `--- PASS: TestNoGitGuard…` lines, no other `--- FAIL`, `guard: silent`, `rc=0`. If a test fails with the guard diagnostic, it is a live offender the census missed. Move it behind the tag into the matching shard file with the Tasks 2–4 procedure (movefuncs, prune-imports, rename with that shard's prefix), then re-run, and report it.

- [ ] **Step 5: Relocate helpers that only the tagged corpus uses**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/defaultonly-unused.sh" internal/repository/transaction'
```
Each printed `<name>\t<file>` is a declaration no untagged test file uses. Move each func with `movefuncs.sh <file> <base>_integration_test.go <name>…` (same base name as its file). Move `type`/`var`/`const` declarations by hand into the same tagged file. Then run `prune-imports.sh internal/repository/transaction` and repeat `defaultonly-unused.sh` until it prints nothing. An untagged file left with no declarations is deleted (`git rm -q -f`). Finally:
```bash
go vet ./internal/repository/transaction/ && go vet -tags integration ./internal/repository/transaction/ && gofmt -l internal/repository/transaction
```
Expected: both vets clean and no gofmt output. Report which helpers moved. If a helper comment such as `useBackgroundOffGit`'s ("safe because this package runs no test in parallel") now sits in a tagged file, keep it verbatim, because it is still true.

- [ ] **Step 6: Mutation probes (spec acceptance 4)**

```bash
bash -c '
P=internal/repository/transaction
# (a) a throwaway default-tag test that runs git must redden the package with the guard diagnostic
printf "package transaction\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestZZNoGitMutationProbe(t *testing.T) { _ = exec.Command(\"git\", \"status\").Run() }\n" > $P/zz_nogit_mutation_test.go
out="$(go test -count=1 ./$P/ 2>&1)"; rc=$?; echo "mutation-a rc=$rc"
grep -E -e "docket nogit guard: default internal/repository/transaction tests must not run real git: 1 real-git exec attempt" <<<"$out" || echo "mutation-a: DIAGNOSTIC MISSING"
rm -f $P/zz_nogit_mutation_test.go
# (b) drop the install from TestMain: the proving tests must go red
f=$P/main_test.go; b="$(mktemp "${TMPDIR:-/tmp}/txn-main.XXXXXX")"; cp "$f" "$b"
perl -0pi -e "s/finish := testsupport\.InstallNoGitGuard\(nogitPkg, nogitShardGlob\)/finish := func(code int) int { return code }; _ = testsupport.NoGitGuardDir/" "$f"
cmp -s "$f" "$b" && echo "mutation b did not land"
go test -count=1 -run "^TestNoGitGuard" ./$P/ >/dev/null 2>&1; echo "mutation-b rc=$?"
cp "$b" "$f"; rm -f "$b"
go test -count=1 ./$P/ >/dev/null 2>&1; echo "restored rc=$?"
'
```
Expected: `mutation-a rc=1` with the diagnostic line printed (not `DIAGNOSTIC MISSING`), `mutation-b rc=1` with no `did not land`, `restored rc=0`. Report all three.

- [ ] **Step 7: The tagged shards still run real git (the guard stays out of the integration build)**

```bash
for r in apply recovery race; do bash tests/test_go_integration_transaction_$r.sh >/dev/null 2>&1; echo "$r rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
```
Expected: every line `rc=0`.

- [ ] **Step 8: Record the default-corpus `-race` wall time (spec acceptance 2)**

Run: `bash -c 'time go test -race -count=1 ./internal/repository/transaction/' 2>&1 | tail -4`
Expected: single-digit seconds of `real` (grooming baseline: ~46s). Report it as `transaction default -race: before ~46s, after <S>s`.

- [ ] **Step 9: Commit**

```bash
git add -- internal/repository/transaction
git status --porcelain
git commit -m "test(transaction): install the no-real-git guard in the default corpus (change 0466)"
```

---

### Task 6: Move workspace's prepare real-git tests into the `TestIntegrationWorkspaceSetup` shard and its two concurrent ones into the `TestRaceIntegrationWorkspace` race shard

**Build profile:** standard

**Files:**
- Modify: `internal/workspace/{prepare,harness}_test.go`
- Create: `internal/workspace/{prepare,harness}_integration_test.go`, `internal/workspace/prepare_race_integration_test.go`
- Create: `tests/test_go_integration_workspace_setup.sh`, `tests/test_go_integration_workspace_race.sh`
- Modify: `tests/runtime-budgets.tsv` (two new rows)
- Modify: any maintained file `rename-tests.sh` rewrites

**Interfaces:**
- Consumes: the Shared tools.
- Produces: runners `tests/test_go_integration_workspace_setup.sh` (`SHARD_PKG="./internal/workspace"`, `SHARD_PREFIX="TestIntegrationWorkspaceSetup"`, `SHARD_MODE="normal"`) and `tests/test_go_integration_workspace_race.sh` (`SHARD_PREFIX="TestRaceIntegrationWorkspace"`, `SHARD_MODE="race"`).

**This task's tests (snapshot, all start real git): 19**
- normal, `prepare_test.go` (16): `TestPrepareBlockedMatrix`, `TestPrepareExistingIdempotent`, `TestPrepareFetchFailureCreatesNothing`, `TestPrepareFreshBlockedByStaleRegistration`, `TestPrepareFreshDoneParent`, `TestPrepareFreshLiveParentStack`, `TestPrepareFreshStackedMergedRecurse`, `TestPrepareFreshUnstacked`, `TestPrepareInvocationMatrix`, `TestPrepareProbeFailureCreatesNothing`, `TestPrepareRejectsMismatchedRepository`, `TestPrepareResumeAttach`, `TestPrepareResumeBranchOffBaseBlocked`, `TestPrepareResumeCreateBoth`, `TestPrepareResumeVerifyOnly`, `TestPrepareReturnsReinspectedFacts`
- normal, `harness_test.go` (1): `TestHarnessBuildersProduceExpectedTopology`
- race, `prepare_test.go` (2): `TestPrepareConcurrentDistinctTargets` (rationale: `concurrent Prepare calls for distinct targets share one primary clone`), `TestPrepareConcurrentSameTarget` (`concurrent Prepare calls race for one target`)

Stays (no git): `TestClassifyRegistrationAbsence`.

- [ ] **Step 1: Confirm the live census**

Write the Shared tools if absent. Run: `bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/census.sh" internal/workspace prepare_test.go harness_test.go'`
Expected: the 19 tests above print `> 0`, and `TestClassifyRegistrationAbsence` prints `0`. The live census wins.

- [ ] **Step 2: Move the tests behind the tag and add the race rationale comments**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/workspace
"$T/movefuncs.sh" $P/prepare_test.go $P/prepare_integration_test.go TestPrepareBlockedMatrix TestPrepareExistingIdempotent TestPrepareFetchFailureCreatesNothing TestPrepareFreshBlockedByStaleRegistration TestPrepareFreshDoneParent TestPrepareFreshLiveParentStack TestPrepareFreshStackedMergedRecurse TestPrepareFreshUnstacked TestPrepareInvocationMatrix TestPrepareProbeFailureCreatesNothing TestPrepareRejectsMismatchedRepository TestPrepareResumeAttach TestPrepareResumeBranchOffBaseBlocked TestPrepareResumeCreateBoth TestPrepareResumeVerifyOnly TestPrepareReturnsReinspectedFacts
"$T/movefuncs.sh" $P/prepare_test.go $P/prepare_race_integration_test.go TestPrepareConcurrentDistinctTargets TestPrepareConcurrentSameTarget
"$T/movefuncs.sh" $P/harness_test.go $P/harness_integration_test.go TestHarnessBuildersProduceExpectedTopology
"$T/prune-imports.sh" $P
for f in $P/*_integration_test.go; do printf "%s: [%s] [%s]\n" "$f" "$(sed -n 1p "$f")" "$(sed -n 2p "$f")"; done
'
```
Then add `// Race shard (change 0466): <rationale>.` directly above each of the two race `func` lines, and confirm `gofmt -l internal/workspace` prints nothing.

- [ ] **Step 3: Rename the moved tests and every maintained reference**

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; X=TestIntegrationWorkspaceSetup; R=TestRaceIntegrationWorkspace; pairs=""
for old in TestPrepareBlockedMatrix TestPrepareExistingIdempotent TestPrepareFetchFailureCreatesNothing TestPrepareFreshBlockedByStaleRegistration TestPrepareFreshDoneParent TestPrepareFreshLiveParentStack TestPrepareFreshStackedMergedRecurse TestPrepareFreshUnstacked TestPrepareInvocationMatrix TestPrepareProbeFailureCreatesNothing TestPrepareRejectsMismatchedRepository TestPrepareResumeAttach TestPrepareResumeBranchOffBaseBlocked TestPrepareResumeCreateBoth TestPrepareResumeVerifyOnly TestPrepareReturnsReinspectedFacts; do pairs="$pairs $old=$X${old#Test}"; done
for old in TestPrepareConcurrentDistinctTargets TestPrepareConcurrentSameTarget; do pairs="$pairs $old=$R${old#Test}"; done
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
# declared in internal/gitcli too: rename ONLY this package copy
"$T/rename-tests.sh" --scope internal/workspace TestHarnessBuildersProduceExpectedTopology=${X}HarnessBuildersProduceExpectedTopology; echo "scoped rename rc=$?"
git grep -l -E -e "^func TestHarnessBuildersProduceExpectedTopology\(" -- internal
git diff --stat -- . ":!internal/workspace"
'
```
Expected: `rename rc=0` and `scoped rename rc=0`. The declaration grep lists only the `internal/gitcli` copy, untouched.

- [ ] **Step 4: Create both shard runners**

Create `tests/test_go_integration_workspace_setup.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_workspace_setup.sh — Go integration shard (change 0466, extending
# change 0333's partition): the workspace Prepare real-git tests (fresh, stacked, resume,
# blocked, and probe-failure preparation, plus the fixture-topology proof) — moved out of the
# default internal/workspace corpus, which must never start real git
# (testsupport.InstallNoGitGuard, installed from the package's TestMain) — behind the
# `integration` build tag, prefix ^TestIntegrationWorkspaceSetup. Declarations only — execution
# and inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/workspace"
SHARD_PREFIX="TestIntegrationWorkspaceSetup"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

Create `tests/test_go_integration_workspace_race.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_workspace_race.sh — Go integration shard (change 0466, extending
# change 0333's partition): the workspace real-concurrency real-git tests (concurrent Prepare
# for one target and for distinct targets over one primary clone) — moved out of the default
# internal/workspace corpus, which must never start real git (testsupport.InstallNoGitGuard,
# installed from the package's TestMain) — behind the `integration` build tag, prefix
# ^TestRaceIntegrationWorkspace, run in RACE mode. Declarations only — execution and inspection
# live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/workspace"
SHARD_PREFIX="TestRaceIntegrationWorkspace"
SHARD_MODE="race"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 5: Verify: default package green, shards green, contract green, no collision**

```bash
go test -count=1 ./internal/workspace/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/workspace/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
for r in setup race; do bash tests/test_go_integration_workspace_$r.sh; echo "$r shard rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/workspace"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: package `ok`, `no leak…`, both shards `rc=0` with only `ok - ` lines, `contract rc=0`, no `COLLISION`.

- [ ] **Step 6: Measure and register both budget rows**

For each of `tests/test_go_integration_workspace_setup.sh` and `tests/test_go_integration_workspace_race.sh`: warm, then measure `bash -c 'bash <runner> >/dev/null 2>&1; time bash <runner> >/dev/null' 2>&1 | tail -4`. Row = S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert `<runner><TAB><row><TAB>parallel` directly above the `tests/test_go_integration_release.sh` row. A row above 40 means NEEDS_ESCALATION. Then `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/` must PASS. Report S, the row, and the margin for each.

- [ ] **Step 7: Commit**

```bash
git add -- internal/workspace tests/test_go_integration_workspace_setup.sh tests/test_go_integration_workspace_race.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 3 rewrote
git status --porcelain
git commit -m "test(workspace): move Prepare real-git tests behind the integration tag (TestIntegrationWorkspaceSetup, TestRaceIntegrationWorkspace, change 0466)"
```

---

### Task 7: Move workspace's inspect, publish, rewrite, and cleanup real-git tests into the `TestIntegrationWorkspaceLifecycle` shard

**Build profile:** standard

**Files:**
- Modify: `internal/workspace/{inspect,publish,rewrite,cleanup}_test.go`
- Create: `internal/workspace/{inspect,publish,rewrite,cleanup}_integration_test.go`
- Create: `tests/test_go_integration_workspace_lifecycle.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row)
- Modify: any maintained file `rename-tests.sh` rewrites

**Interfaces:**
- Consumes: the Shared tools; Task 6's runners (for the prefix check).
- Produces: runner `tests/test_go_integration_workspace_lifecycle.sh` (`SHARD_PKG="./internal/workspace"`, `SHARD_PREFIX="TestIntegrationWorkspaceLifecycle"`, `SHARD_MODE="normal"`). After this task no workspace test starts real git in the default build, which is Task 8's precondition.

**This task's tests (snapshot, all `normal`, all start real git): 40.** None starts goroutines: `TestPublishRewriteContention` and `TestPublishDivergentContended` stage their contention sequentially through a writer clone.
- `cleanup_test.go` (6): `TestCleanupBlockedMatrix`, `TestCleanupDirtyNeverRemoved`, `TestCleanupNeverPrunes`, `TestCleanupProbeFailureIsFailedNotClean`, `TestCleanupReadyClean`, `TestCleanupRetryAlreadyClean`
- `inspect_test.go` (13): `TestInspectAbsent`, `TestInspectAbsentBlockedByLeftovers`, `TestInspectAbsentSlotStatErrorIsError`, `TestInspectBranchGone`, `TestInspectCleaned`, `TestInspectDirty`, `TestInspectForeignMalformed`, `TestInspectForeignUnownedCommonDir`, `TestInspectMismatch`, `TestInspectReady`, `TestInspectReadyAfterParentRebase`, `TestInspectResumable`, `TestInspectUnreadableIsError`
- `publish_test.go` (13): `TestPublishAbsentRefCreates`, `TestPublishDetachedRefused`, `TestPublishDirtyRefused`, `TestPublishDivergentContended`, `TestPublishExpectedHeadMatches`, `TestPublishExpectedHeadMoved`, `TestPublishFastForward`, `TestPublishLocalProxiesNotConsulted`, `TestPublishLostResponseAdopted`, `TestPublishPushFailsRefAbsentFailed`, `TestPublishReadyAfterParentRebase`, `TestPublishRepeatAlreadyPublished`, `TestPublishUnprobeableRemoteUnknown`
- `rewrite_test.go` (8): `TestGeneralPublishStillRefusesRewrite`, `TestPublishRewriteContention`, `TestPublishRewriteLease`, `TestPublishRewriteLeaseWithGatePair`, `TestPublishRewriteLeaseWithPublishCheckpoint`, `TestPublishRewriteNoop`, `TestPublishRewriteRefusesWithoutReceipt`, `TestPublishRewriteUnknownRetains`

- [ ] **Step 1: Confirm the live census, including every other untagged workspace file**

Write the Shared tools if absent. Then:
```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; cd internal/workspace && files="$(for f in *_test.go; do case "$(sed -n 1p "$f")" in //go:build*) ;; *) printf "%s " "$f";; esac; done)"; cd - >/dev/null; out="$("$T/census.sh" internal/workspace $files)"; awk "\$3 > 0" <<<"$out"'
```
Expected: exactly the 40 tests above. Any other line is a live offender. Classify it (race goes to Task 6's race file and prefix, normal to this shard) and move it here. Report it.

- [ ] **Step 2: Move the tests behind the tag**

```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/workspace
"$T/movefuncs.sh" $P/cleanup_test.go $P/cleanup_integration_test.go TestCleanupBlockedMatrix TestCleanupDirtyNeverRemoved TestCleanupNeverPrunes TestCleanupProbeFailureIsFailedNotClean TestCleanupReadyClean TestCleanupRetryAlreadyClean
"$T/movefuncs.sh" $P/inspect_test.go $P/inspect_integration_test.go TestInspectAbsent TestInspectAbsentBlockedByLeftovers TestInspectAbsentSlotStatErrorIsError TestInspectBranchGone TestInspectCleaned TestInspectDirty TestInspectForeignMalformed TestInspectForeignUnownedCommonDir TestInspectMismatch TestInspectReady TestInspectReadyAfterParentRebase TestInspectResumable TestInspectUnreadableIsError
"$T/movefuncs.sh" $P/publish_test.go $P/publish_integration_test.go TestPublishAbsentRefCreates TestPublishDetachedRefused TestPublishDirtyRefused TestPublishDivergentContended TestPublishExpectedHeadMatches TestPublishExpectedHeadMoved TestPublishFastForward TestPublishLocalProxiesNotConsulted TestPublishLostResponseAdopted TestPublishPushFailsRefAbsentFailed TestPublishReadyAfterParentRebase TestPublishRepeatAlreadyPublished TestPublishUnprobeableRemoteUnknown
"$T/movefuncs.sh" $P/rewrite_test.go $P/rewrite_integration_test.go TestGeneralPublishStillRefusesRewrite TestPublishRewriteContention TestPublishRewriteLease TestPublishRewriteLeaseWithGatePair TestPublishRewriteLeaseWithPublishCheckpoint TestPublishRewriteNoop TestPublishRewriteRefusesWithoutReceipt TestPublishRewriteUnknownRetains
"$T/prune-imports.sh" $P
for f in $P/*_integration_test.go; do printf "%s: [%s] [%s]\n" "$f" "$(sed -n 1p "$f")" "$(sed -n 2p "$f")"; done
'
```
Expected: `prune-imports: go vet clean …`, and every file `[//go:build integration] []`. Delete any untagged file left with only a package clause and imports, then re-run `prune-imports.sh`.

- [ ] **Step 3: Rename the moved tests and every maintained reference**

The list includes both `TestPublishRewriteLease` and `TestPublishRewriteLeaseWithGatePair`. The `\b` word boundary keeps the shorter one from rewriting inside the longer one. Order still does not matter.

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; X=TestIntegrationWorkspaceLifecycle; pairs=""
for old in TestCleanupBlockedMatrix TestCleanupDirtyNeverRemoved TestCleanupNeverPrunes TestCleanupProbeFailureIsFailedNotClean TestCleanupReadyClean TestCleanupRetryAlreadyClean TestInspectAbsent TestInspectAbsentBlockedByLeftovers TestInspectAbsentSlotStatErrorIsError TestInspectBranchGone TestInspectCleaned TestInspectDirty TestInspectForeignMalformed TestInspectForeignUnownedCommonDir TestInspectMismatch TestInspectReady TestInspectReadyAfterParentRebase TestInspectResumable TestInspectUnreadableIsError TestPublishAbsentRefCreates TestPublishDetachedRefused TestPublishDirtyRefused TestPublishDivergentContended TestPublishExpectedHeadMatches TestPublishExpectedHeadMoved TestPublishFastForward TestPublishLocalProxiesNotConsulted TestPublishLostResponseAdopted TestPublishPushFailsRefAbsentFailed TestPublishReadyAfterParentRebase TestPublishRepeatAlreadyPublished TestPublishUnprobeableRemoteUnknown TestGeneralPublishStillRefusesRewrite TestPublishRewriteContention TestPublishRewriteLease TestPublishRewriteLeaseWithGatePair TestPublishRewriteLeaseWithPublishCheckpoint TestPublishRewriteNoop TestPublishRewriteRefusesWithoutReceipt TestPublishRewriteUnknownRetains; do pairs="$pairs $old=$X${old#Test}"; done
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
git diff --stat -- . ":!internal/workspace"
'
```
Expected: `rename rc=0`. Then prove the boundary held: `bash -c 'out="$(git grep -n -E -e "TestIntegrationWorkspaceLifecycleTestIntegration|LifecyclePublishRewriteLeaseWith" -- internal/workspace)"; printf "%s\n" "$out"'` must list exactly `TestIntegrationWorkspaceLifecyclePublishRewriteLeaseWithGatePair` and `…WithPublishCheckpoint` (one declaration each, plus any references), and never a doubled prefix.

- [ ] **Step 4: Create the shard runner**

Create `tests/test_go_integration_workspace_lifecycle.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_workspace_lifecycle.sh — Go integration shard (change 0466, extending
# change 0333's partition): the workspace lifecycle real-git tests after Prepare (Inspect,
# Publish, rewrite-lease publication, and Cleanup) — moved out of the default
# internal/workspace corpus, which must never start real git (testsupport.InstallNoGitGuard,
# installed from the package's TestMain) — behind the `integration` build tag, prefix
# ^TestIntegrationWorkspaceLifecycle. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/workspace"
SHARD_PREFIX="TestIntegrationWorkspaceLifecycle"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 5: Verify: default package green, all three shards green, contract green, no collision**

```bash
go test -count=1 ./internal/workspace/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/workspace/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
for r in lifecycle setup race; do bash tests/test_go_integration_workspace_$r.sh >/dev/null; echo "$r shard rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/workspace"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: package `ok`, `no leak…`, all shards `rc=0`, `contract rc=0`, no `COLLISION`, and three workspace prefixes.

- [ ] **Step 6: Measure and register the budget row**

Warm, then measure: `bash -c 'bash tests/test_go_integration_workspace_lifecycle.sh >/dev/null 2>&1; time bash tests/test_go_integration_workspace_lifecycle.sh >/dev/null' 2>&1 | tail -4`. Row = S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert it directly above the `tests/test_go_integration_release.sh` row. A row above 40 means NEEDS_ESCALATION. Then `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/` must PASS. Report S, the row, and the margin.

- [ ] **Step 7: Commit**

```bash
git add -- internal/workspace tests/test_go_integration_workspace_lifecycle.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 3 rewrote
git status --porcelain
git commit -m "test(workspace): move inspect/publish/rewrite/cleanup real-git tests behind the integration tag (TestIntegrationWorkspaceLifecycle, change 0466)"
```

---

### Task 8: Install the no-real-git guard in `internal/workspace`

**Build profile:** standard

**Files:**
- Create: `internal/workspace/main_test.go` (untagged: `TestMain`, `nogitPkg`, `nogitShardGlob`)
- Create: `internal/workspace/nogit_guard_test.go` (`//go:build !integration && !e2e`)
- Modify/Move: untagged workspace `*_test.go` helper declarations that only the tagged corpus uses

**Interfaces:**
- Consumes: Task 1's testsupport guard API. Tasks 6–7 must have moved every real-git workspace test.
- Produces: the invariant "the default-tag `internal/workspace` test corpus never starts a real git", enforced.

- [ ] **Step 1: Write the failing proving tests**

Create `internal/workspace/nogit_guard_test.go`:

```go
//go:build !integration && !e2e

package workspace

// The no-real-git guard's proving tests for internal/workspace (change 0466). The
// guard lives in internal/testsupport (InstallNoGitGuard) and is installed from
// TestMain in main_test.go. These tests prove it is installed in THIS package's
// binary and fails the package on any real-git exec, even one a test tolerates.
// Real-git tests live behind //go:build integration in the
// tests/test_go_integration_workspace_*.sh shards.

import (
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestNoGitGuardShadowsGitOnPath: every PATH lookup of `git` resolves the shim.
func TestNoGitGuardShadowsGitOnPath(t *testing.T) {
	testsupport.AssertNoGitGuardShadowsGit(t)
}

// TestNoGitGuardRefusesBareExec: a bare git exec gets the guard's exit code and diagnostic.
func TestNoGitGuardRefusesBareExec(t *testing.T) {
	testsupport.AssertNoGitGuardRefusesBareExec(t, nogitPkg)
}

// TestNoGitGuardFailsTolerantTest: a test that swallows the git failure still fails the package.
func TestNoGitGuardFailsTolerantTest(t *testing.T) {
	testsupport.NoGitGuardTolerantProbe(t, nogitPkg, "TestNoGitGuardFailsTolerantTest")
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -count=1 -run '^TestNoGitGuard' ./internal/workspace/`
Expected: FAIL to compile with `undefined: nogitPkg`.

- [ ] **Step 3: Install the guard from `TestMain`**

Create `internal/workspace/main_test.go`:

```go
package workspace

import (
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// nogitPkg and nogitShardGlob name this package to the shared no-real-git guard
// (change 0466): the default-tag internal/workspace test corpus never starts a real
// git; real-git tests live behind //go:build integration in the
// tests/test_go_integration_workspace_*.sh shards.
const (
	nogitPkg       = "internal/workspace"
	nogitShardGlob = "tests/test_go_integration_workspace_*.sh"
)

// TestMain installs the no-real-git guard (testsupport.InstallNoGitGuard) around
// m.Run in the default build; the integration build gets testsupport's identity
// finisher, so the tagged shards run real git as before.
func TestMain(m *testing.M) {
	finish := testsupport.InstallNoGitGuard(nogitPkg, nogitShardGlob)
	os.Exit(finish(m.Run()))
}
```

- [ ] **Step 4: The whole default corpus is green with the guard on**

```bash
bash -c 'out="$(go test -count=1 -v ./internal/workspace/ 2>&1)"; rc=$?; grep -E -e "^--- (FAIL|PASS): TestNoGitGuard" <<<"$out"; grep -E -e "^--- FAIL" <<<"$out"; grep -E -e "real-git exec attempt\(s\)|docket nogit guard" <<<"$out" || echo "guard: silent"; echo "rc=$rc"'
```
Expected: three `--- PASS: TestNoGitGuard…` lines, no other `--- FAIL`, `guard: silent`, `rc=0`. A test failing with the guard diagnostic is a live offender. Move it with the Tasks 6–7 procedure into the matching shard and report it.

- [ ] **Step 5: Relocate helpers that only the tagged corpus uses**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/defaultonly-unused.sh" internal/workspace'
```
Move each listed func with `movefuncs.sh <file> <base>_integration_test.go <name>…`, and each listed type/var/const by hand into the same tagged file. Run `prune-imports.sh internal/workspace`, and repeat until `defaultonly-unused.sh` prints nothing. Delete untagged files left with no declarations. Finally: `go vet ./internal/workspace/ && go vet -tags integration ./internal/workspace/ && gofmt -l internal/workspace`. Expected: clean. Report which helpers moved.

- [ ] **Step 6: Mutation probes (spec acceptance 4)**

```bash
bash -c '
P=internal/workspace
printf "package workspace\n\nimport (\n\t\"os/exec\"\n\t\"testing\"\n)\n\nfunc TestZZNoGitMutationProbe(t *testing.T) { _ = exec.Command(\"git\", \"status\").Run() }\n" > $P/zz_nogit_mutation_test.go
out="$(go test -count=1 ./$P/ 2>&1)"; rc=$?; echo "mutation-a rc=$rc"
grep -E -e "docket nogit guard: default internal/workspace tests must not run real git: 1 real-git exec attempt" <<<"$out" || echo "mutation-a: DIAGNOSTIC MISSING"
rm -f $P/zz_nogit_mutation_test.go
f=$P/main_test.go; b="$(mktemp "${TMPDIR:-/tmp}/ws-main.XXXXXX")"; cp "$f" "$b"
perl -0pi -e "s/finish := testsupport\.InstallNoGitGuard\(nogitPkg, nogitShardGlob\)/finish := func(code int) int { return code }; _ = testsupport.NoGitGuardDir/" "$f"
cmp -s "$f" "$b" && echo "mutation b did not land"
go test -count=1 -run "^TestNoGitGuard" ./$P/ >/dev/null 2>&1; echo "mutation-b rc=$?"
cp "$b" "$f"; rm -f "$b"
go test -count=1 ./$P/ >/dev/null 2>&1; echo "restored rc=$?"
'
```
Expected: `mutation-a rc=1` with the diagnostic line printed, `mutation-b rc=1`, no `did not land`, `restored rc=0`.

- [ ] **Step 7: The tagged shards still run real git**

```bash
for r in setup lifecycle race; do bash tests/test_go_integration_workspace_$r.sh >/dev/null 2>&1; echo "$r rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
```
Expected: every line `rc=0`.

- [ ] **Step 8: Record the default-corpus `-race` wall time (spec acceptance 2)**

Run: `bash -c 'time go test -race -count=1 ./internal/workspace/' 2>&1 | tail -4`
Expected: single-digit seconds (grooming baseline: ~42s). Report `workspace default -race: before ~42s, after <S>s`.

- [ ] **Step 9: Commit**

```bash
git add -- internal/workspace
git status --porcelain
git commit -m "test(workspace): install the no-real-git guard in the default corpus (change 0466)"
```

---

### Task 9: Move gatedrive's real-supervisor and real-git tests behind the tag (`TestIntegrationGatedrive` / `TestRaceIntegrationGatedrive`)

**Build profile:** premium

The named risk: `integration_test.go` owns the package's `TestMain`, which routes the test binary's supervisor and child re-exec roles. It must move into the tagged build intact, since every real-supervisor test depends on it, and the default build must compile without it. The four files are also untagged today, so once the contract discovers `internal/gatedrive`, its check (6) reddens on any `TestIntegration…` name left visible to the default corpus.

**Files:**
- Move (`git mv`, whole file): `internal/gatedrive/integration_test.go` → `internal/gatedrive/supervisor_integration_test.go`
- Move (`git mv`, whole file): `internal/gatedrive/integration_takeover_test.go` → `internal/gatedrive/takeover_integration_test.go`
- Move (`git mv`, whole file): `internal/gatedrive/integration_sequence_test.go` → `internal/gatedrive/sequence_integration_test.go`
- Move (`git mv`, whole file): `internal/gatedrive/integration_history_test.go` → `internal/gatedrive/history_integration_test.go`
- Create: `internal/gatedrive/{takeover,sequence,history}_race_integration_test.go`
- Modify: `internal/gatedrive/{fingerprint,handoff}_test.go`; Create: `internal/gatedrive/{fingerprint,handoff}_integration_test.go`
- Create: `tests/test_go_integration_gatedrive_process.sh`, `tests/test_go_integration_gatedrive_race.sh`
- Modify: `tests/runtime-budgets.tsv` (two new rows)
- Modify: any maintained file the rename or file-reference greps find

**Interfaces:**
- Consumes: the Shared tools.
- Produces: runners `tests/test_go_integration_gatedrive_process.sh` (`SHARD_PKG="./internal/gatedrive"`, `SHARD_PREFIX="TestIntegrationGatedrive"`, `SHARD_MODE="normal"`) and `tests/test_go_integration_gatedrive_race.sh` (`SHARD_PREFIX="TestRaceIntegrationGatedrive"`, `SHARD_MODE="race"`). gatedrive gets **no** no-real-git guard (spec decision 3): its slow tests start supervised processes, which a git shim cannot see, so its budget row stays the growth detector.

**This task's tests (snapshot): 25**

| old name | file today | shard | new name |
|---|---|---|---|
| `TestIntegrationDeadlineExpiryStopsOwnedTree` | `integration_test.go` | normal | `TestIntegrationGatedriveDeadlineExpiryStopsOwnedTree` |
| `TestIntegrationDriverSlicesAcrossLiveChildThenPasses` | `integration_test.go` | normal | `TestIntegrationGatedriveDriverSlicesAcrossLiveChildThenPasses` |
| `TestIntegrationFreshProcessResumesAndChildSurvives` | `integration_test.go` | normal | `TestIntegrationGatedriveFreshProcessResumesAndChildSurvives` |
| `TestIntegrationProcessDeathPermitsAtMostOneRelaunch` | `integration_test.go` | normal | `TestIntegrationGatedriveProcessDeathPermitsAtMostOneRelaunch` |
| `TestIntegrationFastCompletionReturnsImmediately` | `integration_takeover_test.go` | normal | `TestIntegrationGatedriveFastCompletionReturnsImmediately` |
| `TestIntegrationSliceBoundIsProductionThirtySeconds` | `integration_takeover_test.go` | normal | `TestIntegrationGatedriveSliceBoundIsProductionThirtySeconds` |
| `TestIntegrationTerminalConsumedFromFreshProcess` | `integration_takeover_test.go` | normal | `TestIntegrationGatedriveTerminalConsumedFromFreshProcess` |
| `TestIntegrationTakeoverKeepsRunIdentity` | `integration_takeover_test.go` | **race** (`a parent takeover supersedes the owner of a live supervised child mid-run`) | `TestRaceIntegrationGatedriveTakeoverKeepsRunIdentity` |
| `TestIntegrationSequenceRealGitBaselineRedGreen` | `integration_sequence_test.go` | normal | `TestIntegrationGatedriveSequenceRealGitBaselineRedGreen` |
| `TestIntegrationSequenceCredentialTheftRejected` | `integration_sequence_test.go` | normal | `TestIntegrationGatedriveSequenceCredentialTheftRejected` |
| `TestIntegrationSequenceConcurrentScopesResolveOwnWork` | `integration_sequence_test.go` | **race** (`two goroutines drive distinct scopes to terminal concurrently`) | `TestRaceIntegrationGatedriveSequenceConcurrentScopesResolveOwnWork` |
| `TestIntegrationSameWorktreeGenerations` | `integration_sequence_test.go` | **race** (`a live incumbent run holds the worktree slot while a start through its alias contends for it`) | `TestRaceIntegrationGatedriveSameWorktreeGenerations` |
| `TestIntegrationOutcomeTriggersNoLegacyCensusOrSecondStart` | `integration_history_test.go` | normal | `TestIntegrationGatedriveOutcomeTriggersNoLegacyCensusOrSecondStart` |
| `TestRaceConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe` | `integration_history_test.go` | **race** (`concurrent Starts and a CleanupHistory race over one legacy-seeded store`) | `TestRaceIntegrationGatedriveConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe` |
| `TestFingerprintDanglingSymlinkHashedByValue` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintDanglingSymlinkHashedByValue` |
| `TestFingerprintDetectsModeChange` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintDetectsModeChange` |
| `TestFingerprintFileDeleted` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintFileDeleted` |
| `TestFingerprintFileRenamed` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintFileRenamed` |
| `TestFingerprintIdenticalDirtyStateEqual` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintIdenticalDirtyStateEqual` |
| `TestFingerprintStagedByteChange` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintStagedByteChange` |
| `TestFingerprintSymlinkTargetChanged` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintSymlinkTargetChanged` |
| `TestFingerprintUnstagedByteChange` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintUnstagedByteChange` |
| `TestFingerprintUntrackedFileAdded` | `fingerprint_test.go` | normal | `TestIntegrationGatedriveFingerprintUntrackedFileAdded` |
| `TestRepoHandoffPerDimensionDriftRejectsClaim` | `handoff_test.go` | normal | `TestIntegrationGatedriveRepoHandoffPerDimensionDriftRejectsClaim` |
| `TestDirtyHandoffIdenticalStateClaimsWithoutWIPCommit` | `handoff_test.go` | normal | `TestIntegrationGatedriveDirtyHandoffIdenticalStateClaimsWithoutWIPCommit` |

The spec names the whole of the four `integration*_test.go` files as moving. That includes `integration_history_test.go`'s two tests, which use scripted procs, not the real supervisor: they carry the integration naming and belong to the partition.

- [ ] **Step 1: Confirm the live census**

Write the Shared tools if absent. Then:
```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; cd internal/gatedrive && files="$(for f in *_test.go; do case "$(sed -n 1p "$f")" in //go:build*) ;; *) printf "%s " "$f";; esac; done)"; cd - >/dev/null; out="$("$T/census.sh" internal/gatedrive $files)"; awk "\$3 > 0" <<<"$out"'
grep -c -E -e '^func Test' internal/gatedrive/integration_test.go internal/gatedrive/integration_takeover_test.go internal/gatedrive/integration_sequence_test.go internal/gatedrive/integration_history_test.go
```
Expected: git-exec lines exactly for the 9 fingerprint tests, the 2 handoff tests, and the 4 `integration_sequence_test.go` tests. The four files hold 4 + 4 + 4 + 2 test functions. The live census and the live file contents win: a new test in one of the four files moves with its file, and a new git-using test elsewhere moves into the normal shard. Report any difference.

- [ ] **Step 2: Move the four integration files and tag them**

```bash
bash -c '
set -euo pipefail
P=internal/gatedrive
git mv $P/integration_test.go $P/supervisor_integration_test.go
git mv $P/integration_takeover_test.go $P/takeover_integration_test.go
git mv $P/integration_sequence_test.go $P/sequence_integration_test.go
git mv $P/integration_history_test.go $P/history_integration_test.go
for f in $P/supervisor_integration_test.go $P/takeover_integration_test.go $P/sequence_integration_test.go $P/history_integration_test.go; do
  case "$(sed -n 1p "$f")" in //go:build*) echo "UNEXPECTED existing constraint in $f"; exit 1;; esac
  tmp="$(mktemp "${TMPDIR:-/tmp}/tagmove.XXXXXX")"; { printf "//go:build integration\n\n"; cat "$f"; } > "$tmp" && mv -f "$tmp" "$f"
done
'
```
Then split the race tests out of the tagged files and move the git-using tests out of `fingerprint_test.go`/`handoff_test.go`:
```bash
bash -c '
set -euo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; P=internal/gatedrive
"$T/movefuncs.sh" $P/takeover_integration_test.go $P/takeover_race_integration_test.go TestIntegrationTakeoverKeepsRunIdentity
"$T/movefuncs.sh" $P/sequence_integration_test.go $P/sequence_race_integration_test.go TestIntegrationSequenceConcurrentScopesResolveOwnWork TestIntegrationSameWorktreeGenerations
"$T/movefuncs.sh" $P/history_integration_test.go $P/history_race_integration_test.go TestRaceConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe
"$T/movefuncs.sh" $P/fingerprint_test.go $P/fingerprint_integration_test.go TestFingerprintDanglingSymlinkHashedByValue TestFingerprintDetectsModeChange TestFingerprintFileDeleted TestFingerprintFileRenamed TestFingerprintIdenticalDirtyStateEqual TestFingerprintStagedByteChange TestFingerprintSymlinkTargetChanged TestFingerprintUnstagedByteChange TestFingerprintUntrackedFileAdded
"$T/movefuncs.sh" $P/handoff_test.go $P/handoff_integration_test.go TestRepoHandoffPerDimensionDriftRejectsClaim TestDirtyHandoffIdenticalStateClaimsWithoutWIPCommit
"$T/prune-imports.sh" $P
for f in $P/*_integration_test.go; do printf "%s: [%s] [%s]\n" "$f" "$(sed -n 1p "$f")" "$(sed -n 2p "$f")"; done
'
```
Expected: `prune-imports: go vet clean …` and every file `[//go:build integration] []`. `movefuncs.sh` created the three `*_race_integration_test.go` files from their tagged sources, so their line 1 is already the tag. Add `// Race shard (change 0466): <rationale from the table>.` directly above each of the four race `func` lines.

**`TestMain` stays whole in `supervisor_integration_test.go`.** The default build then has no `TestMain` and uses Go's default runner. That is correct because no default gatedrive test re-execs the test binary (Step 8 proves it). Append this paragraph to the `TestMain` doc comment:

```go
// Change 0466 moved this file (formerly integration_test.go) behind the integration
// tag. The default gatedrive build has no TestMain: none of its tests re-execs the
// test binary as a supervisor or child (only this tagged corpus drives the real
// process.Service), so Go's default m.Run is exactly right there.
```

- [ ] **Step 3: Relocate helpers so each build compiles exactly what it uses**

```bash
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; "$T/defaultonly-unused.sh" internal/gatedrive'
```
Move each listed func with `movefuncs.sh <file> <base>_integration_test.go <name>…` (for example `fingerprint_test.go`'s `newDirtyRepo`/`git`/`gitInit`/`gitAdd`/`gitCommit` if the census-moved tests are their only users), and each listed type/var/const by hand. Run `prune-imports.sh internal/gatedrive`, and repeat until the script prints nothing. Delete any untagged file left with no declarations (`git rm -q -f`). Finally: `go vet ./internal/gatedrive/ && go vet -tags integration ./internal/gatedrive/ && gofmt -l internal/gatedrive`. Expected: clean.

- [ ] **Step 4: Rename the tests and every maintained reference**

```bash
bash -c '
set -uo pipefail
T="${TMPDIR:-/tmp}/docket-0466-tools"; N=TestIntegrationGatedrive; R=TestRaceIntegrationGatedrive; pairs=""
for old in TestIntegrationDeadlineExpiryStopsOwnedTree TestIntegrationDriverSlicesAcrossLiveChildThenPasses TestIntegrationFreshProcessResumesAndChildSurvives TestIntegrationProcessDeathPermitsAtMostOneRelaunch TestIntegrationFastCompletionReturnsImmediately TestIntegrationSliceBoundIsProductionThirtySeconds TestIntegrationTerminalConsumedFromFreshProcess TestIntegrationSequenceRealGitBaselineRedGreen TestIntegrationSequenceCredentialTheftRejected TestIntegrationOutcomeTriggersNoLegacyCensusOrSecondStart; do pairs="$pairs $old=$N${old#TestIntegration}"; done
for old in TestFingerprintDanglingSymlinkHashedByValue TestFingerprintDetectsModeChange TestFingerprintFileDeleted TestFingerprintFileRenamed TestFingerprintIdenticalDirtyStateEqual TestFingerprintStagedByteChange TestFingerprintSymlinkTargetChanged TestFingerprintUnstagedByteChange TestFingerprintUntrackedFileAdded TestRepoHandoffPerDimensionDriftRejectsClaim TestDirtyHandoffIdenticalStateClaimsWithoutWIPCommit; do pairs="$pairs $old=$N${old#Test}"; done
for old in TestIntegrationTakeoverKeepsRunIdentity TestIntegrationSequenceConcurrentScopesResolveOwnWork TestIntegrationSameWorktreeGenerations; do pairs="$pairs $old=$R${old#TestIntegration}"; done
pairs="$pairs TestRaceConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe=${R}ConcurrentStartsOverLegacySeededStoreArbitrateAndCleanupSafe"
"$T/rename-tests.sh" $pairs; echo "rename rc=$?"
git diff --stat -- . ":!internal/gatedrive"
'
```
Expected: `rename rc=0`. Check a sample against the table, e.g. `git grep -n -e "^func TestIntegrationGatedriveDeadlineExpiryStopsOwnedTree(" -- internal/gatedrive` finds exactly one declaration.

Then fix the old **file-name** references (comments such as `integration_test.go: mustService, …` or `TestMain (integration_test.go)`):
```bash
bash -c 'git grep -n -E -e "(^|[^_A-Za-z])integration(_takeover|_sequence|_history)?_test\.go" -- internal/gatedrive internal/repoguard skills internal/assets docs/*.md tests || echo "no stale file references"'
```
Rewrite each hit to the new file name (`integration_test.go` → `supervisor_integration_test.go`, `integration_takeover_test.go` → `takeover_integration_test.go` or `takeover_race_integration_test.go`, and likewise for sequence and history), naming wherever the referenced symbol now actually lives. Expected afterwards: `no stale file references`.

- [ ] **Step 5: Create both shard runners**

Create `tests/test_go_integration_gatedrive_process.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_gatedrive_process.sh — Go integration shard (change 0466, extending
# change 0333's partition): the gate driver's real-process and real-git tests (driving the REAL
# native supervisor internal/process.Service across slices, fresh-process resume, deadline and
# death handling, real-git sequences, and the worktree fingerprint/handoff proofs over real
# repositories) — moved out of the default internal/gatedrive corpus behind the `integration`
# build tag, prefix ^TestIntegrationGatedrive. internal/gatedrive has no no-real-git guard (its
# slow tests are process-bound, not git-bound): the budget row of tests/test_go_race.sh is its
# growth detector. Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gatedrive"
SHARD_PREFIX="TestIntegrationGatedrive"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

Create `tests/test_go_integration_gatedrive_race.sh` with exactly this content, then `chmod +x` it:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_gatedrive_race.sh — Go integration shard (change 0466, extending
# change 0333's partition): the gate driver's real-concurrency integration tests (takeover of a
# live supervised run, concurrent scopes driven to terminal, same-worktree generations against a
# live incumbent, concurrent Starts over a legacy-seeded store) — moved out of the default
# internal/gatedrive corpus behind the `integration` build tag, prefix
# ^TestRaceIntegrationGatedrive, run in RACE mode. Declarations only — execution and inspection
# live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/gatedrive"
SHARD_PREFIX="TestRaceIntegrationGatedrive"
SHARD_MODE="race"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 6: Verify: default package green, shards green, contract green, no collision**

```bash
go test -count=1 ./internal/gatedrive/
bash -c 'leak="$(go test -list "^Test(Race)?Integration" ./internal/gatedrive/ 2>&1)"; grep -E -e "^Test(Race)?Integration" <<<"$leak" && echo LEAK || echo "no leak into the default corpus"'
for r in process race; do bash tests/test_go_integration_gatedrive_$r.sh; echo "$r shard rc=$?"; done
bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"
bash -c 'pk="./internal/gatedrive"; p=""; for r in tests/test_go_integration_*.sh; do [ "$r" = tests/test_go_integration_contract.sh ] && continue; d="$(DOCKET_SHARD_INSPECT=1 bash "$r")"; case "$d" in *"package=$pk"*) p="$p $(sed -n "s/^prefix=//p" <<<"$d")";; esac; done; for a in $p; do for b in $p; do [ "$a" != "$b" ] && case "$b" in "$a"*) echo "COLLISION: $a is a prefix of $b";; esac; done; done; echo "prefixes:$p"; echo prefix-check-done'
```
Expected: package `ok`, `no leak…`, both shards `rc=0` (21 and 4 tests), `contract rc=0`, no `COLLISION`.

- [ ] **Step 7: Measure and register both budget rows**

For each of `tests/test_go_integration_gatedrive_process.sh` and `tests/test_go_integration_gatedrive_race.sh`: warm, then measure `bash -c 'bash <runner> >/dev/null 2>&1; time bash <runner> >/dev/null' 2>&1 | tail -4`. Row = S rounded up to the next multiple of 5, plus 5 (minimum 10). Insert it directly above the `tests/test_go_integration_release.sh` row. A row above 40 means NEEDS_ESCALATION. Then `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/` must PASS. Report S, the row, and the margin for each.

- [ ] **Step 8: No real-process or real-git test is left in gatedrive's default corpus (Review Focus 5)**

```bash
bash -c 'hits=""; for f in internal/gatedrive/*_test.go; do case "$(sed -n 1p "$f")" in //go:build*) continue;; esac; h="$(grep -n -E -e "process\.NewService|mustService\(|mustExe\(|intChildMarker" "$f")"; [ -z "$h" ] || hits="$hits$f: $h
"; done; [ -z "$hits" ] && echo "no real-supervisor use in the default corpus" || printf "%s" "$hits"'
bash -c 'T="${TMPDIR:-/tmp}/docket-0466-tools"; cd internal/gatedrive && files="$(for f in *_test.go; do case "$(sed -n 1p "$f")" in //go:build*) ;; *) printf "%s " "$f";; esac; done)"; cd - >/dev/null; out="$("$T/census.sh" internal/gatedrive $files)"; bad="$(awk "\$3 > 0" <<<"$out")"; [ -z "$bad" ] && echo "census: no default gatedrive test runs git" || printf "%s\n" "$bad"'
```
Expected: `no real-supervisor use in the default corpus` and `census: no default gatedrive test runs git`. (The `driver_test.go` line `var _ ProcessSeam = (*process.Service)(nil)` is a compile-time assertion, not a `NewService` call, so it does not match.)

- [ ] **Step 9: Record the default-corpus `-race` wall time (spec acceptance 2)**

Run: `bash -c 'time go test -race -count=1 ./internal/gatedrive/' 2>&1 | tail -4`
Report `gatedrive default -race: before ~42s, after <S>s`. About 15s of real-process time leaves with this task, so expect a large drop but not single digits. The package keeps 265 scripted-proc tests.

- [ ] **Step 10: Commit**

```bash
git add -- internal/gatedrive tests/test_go_integration_gatedrive_process.sh tests/test_go_integration_gatedrive_race.sh tests/runtime-budgets.tsv   # plus any file outside the package that Step 4 rewrote
git status --porcelain
git commit -m "test(gatedrive): move real-supervisor and real-git tests behind the integration tag (TestIntegrationGatedrive, TestRaceIntegrationGatedrive, change 0466)"
```

---

### Task 10: Update the race gate's header, serial-confirm both gates, and record the numbers

**Build profile:** standard

**Files:**
- Modify: `tests/test_go_race.sh` (header comment only: the "PARTITION AND LANE" and "BACKSTOP TIMEOUT" paragraphs)
- Modify: `tests/runtime-budgets.tsv` (only the eight new rows, and only if a fresh measurement's rule value is higher)

**Interfaces:**
- Consumes: every earlier task. Specifically Task 1's `testsupport.InstallNoGitGuard`, the eight runners from Tasks 2–4, 6–7, and 9, and the guards installed by Tasks 5 and 8.
- Produces: the measurement record the results file cites (spec acceptance 1, 2, 3, and the margins).

- [ ] **Step 1: Rank the default corpus's slowest packages**

Run on an idle machine (record `uptime` first):
```bash
bash -c '
uptime
out="$(GOMAXPROCS=2 go test -race -count=1 -p 2 -json ./... 2>/dev/null)"
jq -r "select((.Action==\"pass\" or .Action==\"fail\") and .Test==null) | \"\(.Elapsed)\t\(.Action)\t\(.Package)\"" <<<"$out" | LC_ALL=C sort -rn > "${TMPDIR:-/tmp}/race-pkg-rank.tsv"
sed -n "1,6p" "${TMPDIR:-/tmp}/race-pkg-rank.tsv"
grep -c -E -e "	fail	" "${TMPDIR:-/tmp}/race-pkg-rank.tsv"
'
```
Expected: no `fail` rows (the count prints `0`). The top row names the new worst default-corpus package W at S_W seconds. That is the same `-p 2` shape 0465's 48.7s transaction figure was measured with. Report the top six rows, plus the transaction, workspace, and gatedrive rows.

- [ ] **Step 2: Update the race gate's header prose**

In `tests/test_go_race.sh`, in the "PARTITION AND LANE" paragraph, replace

```bash
# starts a real `git` process. installNoGitGuard (internal/app/nogit_guard_test.go)
# shadows git on PATH in the default build and fails the package on any attempt,
# so a new real-git test cannot land here unnoticed. With that tail gone, `go test
```

with

```bash
# starts a real `git` process. testsupport.InstallNoGitGuard (internal/testsupport,
# installed from the package's TestMain) shadows git on PATH in the default build and
# fails the package on any attempt, so a new real-git test cannot land here unnoticed.
# Change 0466 extended the partition and the guard to internal/repository/transaction
# and internal/workspace, and moved internal/gatedrive's real-supervisor and real-git
# tests behind the tag too. gatedrive is process-bound rather than git-bound, so it
# has no git guard; this file's budget row is its growth detector. With that tail gone, `go test
```

Then re-wrap the paragraph's lines to the file's existing width without changing any other words. In the "BACKSTOP TIMEOUT" paragraph, directly after the sentence ending `internal/repository/transaction at 48.7s (local, idle, -p 2).`, insert:

```bash
# Change 0466 then partitioned that package; the measured worst default-corpus package
# is now <W> at <S_W>s (same shape). The backstop and its floor in internal/repoguard
# keep 0465's larger 48.7s input, so the margin only grew.
```

Fill in `<W>` and `<S_W>` from Step 1. Do not change `RACE_TIMEOUT` or any executable line. Then:
```bash
go test -count=1 -run 'TestRaceGate' ./internal/repoguard/ && go test -count=1 ./internal/repoguard/
bash -c 'git grep -n -w -E -e "installNoGitGuard|nogit_guard_off_test" -- . ":!docs/results" ":!docs/changes" ":!docs/superpowers" ":!docs/adrs" || echo "no stale guard references"'
```
Expected: both PASS, and `no stale guard references`.

- [ ] **Step 3: Serial-confirm the race gate (spec acceptance 1)**

On an idle machine, run it solo twice and take the worse reading:
```bash
bash -c 'for i in 1 2; do uptime; ( time bash tests/test_go_race.sh >/dev/null 2>&1; echo "rc=$?" ) 2>&1 | grep -E -e "^(real|rc=)"; done'
```
Expected: both `rc=0`, and the worse `real` R is under 60s. The margin is 60 − R. If R ≥ 60, **do not raise the row**. Write `FINDING: test_go_race <R>s ≥ 60` with Step 1's ranking and return NEEDS_ESCALATION.

- [ ] **Step 4: Serial-confirm the toolchain gate on a cold test cache (spec acceptance 1)**

The runner uses `$GOCACHE` when set, else `<git common dir>/docket-go-cache/build`. Clear the test cache in that same location before each run:
```bash
bash -c '
c="$(git rev-parse --git-common-dir)"; case "$c" in /*) ;; *) c="$PWD/$c";; esac
gc="${GOCACHE:-$c/docket-go-cache/build}"
for i in 1 2; do GOCACHE="$gc" go clean -testcache; uptime; ( time bash tests/test_go_toolchain.sh >/dev/null 2>&1; echo "rc=$?" ) 2>&1 | grep -E -e "^(real|rc=)"; done
'
```
Expected: both `rc=0`, and the worse `real` K is under 55s. The margin is 55 − K. If K ≥ 55, do not raise the row: write `FINDING: test_go_toolchain cold <K>s ≥ 55` and return NEEDS_ESCALATION.

- [ ] **Step 5: Re-confirm every new shard row**

Measure each of the eight new runners solo once more (warm the build cache, then `time bash <runner> >/dev/null`):
`tests/test_go_integration_transaction_{apply,recovery,race}.sh`, `tests/test_go_integration_workspace_{setup,lifecycle,race}.sh`, `tests/test_go_integration_gatedrive_{process,race}.sh`.
Compute each rule value (next multiple of 5, plus 5, minimum 10). If a rule value is **higher** than the row the move task registered, raise that new row to it. These rows are new in this change, so this is their honest sizing, not a relaxation. Never lower a row below its rule value, and never touch the 60/55 rows. Then run `go test -count=1 -run TestRuntimeBudgetsCorrespondence ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 6: Contract and every new shard green together**

```bash
bash -c 'bash tests/test_go_integration_contract.sh >/dev/null; echo "contract rc=$?"; for r in tests/test_go_integration_transaction_*.sh tests/test_go_integration_workspace_*.sh tests/test_go_integration_gatedrive_*.sh; do bash "$r" >/dev/null 2>&1; echo "$r rc=$?"; done'
bash -c 'out="$(git diff --name-only ef4a341d20e392a0c6ac09dd0bb49fce97a61f73 -- tests/test_go_integration_contract.sh tests/lib/go-integration-shard.sh)"; [ -z "$out" ] && echo "contract and shard executor untouched" || printf "EDITED: %s\n" "$out"'
```
Expected: every `rc=0`, and `contract and shard executor untouched` (spec acceptance 3: new packages discovered with no allowlist edit).

- [ ] **Step 7: Commit and report**

```bash
git add -- tests/test_go_race.sh tests/runtime-budgets.tsv
git status --porcelain
git commit -m "test(race): record the post-partition worst package and re-confirm budgets (change 0466)"
```

Your report must carry, as numbers, for the results file:
- `test_go_race.sh`: both solo readings, the worse one R, and the margin 60 − R (with `uptime` load)
- `test_go_toolchain.sh` (cold test cache): both readings, the worse one K, and the margin 55 − K
- the Step 1 top-six package ranking, W, and S_W
- the default-corpus `-race` wall times of transaction, workspace, and gatedrive (from Tasks 5, 8, and 9, plus Step 1's `-p 2` rows)
- every new row: measured S, row, and margin
- any `FINDING:` line

State also that acceptance 5 (whole suite green, no `SERIAL CONFIRMED OVER BUDGET` line for any file this change touched or created) is verified at docket-build's final suite gate. That gate's budget report must be read even on a green run.
