<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0487 — Bring test_go_race, rebaserecovery, and closeout back under budget, fix the repoguard concurrent-gate timeout, and gofmt comment_integration_test.go](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0487-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu.md)**
<!-- docket:backlink:end -->
# Race Gate and Integration Shards Back Under Budget Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execute this plan with `docket-build` (this repo's
> `skills.build`), one task per worker under the `docket-build-task` contract. Steps use checkbox
> (`- [ ]`) syntax for reading order only; nobody ticks them.

**Goal:** Bring `tests/test_go_race.sh` and the closeout and rebaserecovery integration shards back
under their runtime budgets, remove the `internal/repoguard` race-gate timeout under concurrent gates,
and gofmt `internal/githubcli/comment_integration_test.go`.

**Architecture:** The race gate is slow because `TestRetiredVocabularySeal` runs ~110 regexps on every
scanned line and the ~150-way catalog alternation on every block. Add an exact `strings.Contains`
pre-filter in front of each regexp. Each regexp embeds `regexp.QuoteMeta(r.Old)` literally, so the
filter cannot change detection, and a new differential subtest pins that. Split each over-budget
integration shard into two disjoint test-name prefixes by renaming one half (the change 0434
precedent), add a wrapper and a budget row for the new half, and size every touched row from fresh
serial solo readings.

**Tech Stack:** Go 1.x (`go test`, `-race`, `-tags integration`), bash wrappers under `tests/`, the
Go suite runner (`go run ./cmd/docket development test`), `jq` for reading `go test -json`.

**Spec:** `docs/superpowers/specs/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu-design.md`
(on the `docket` branch; synchronized copy at
`/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-01-bring-test-go-race-rebaserecovery-and-closeout-back-under-bu-design.md`).

**Worktree:** `W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu`
(branch `fix/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu`). Every command below uses
absolute paths or `git -C "$W"`; the agent shell's cwd does not persist.

## Global Constraints

- The 8-minute `-race` backstop (`RACE_TIMEOUT="8m"` in `tests/test_go_race.sh`) is **not raised**.
- **No** cross-gate or machine-wide Go test-load bound, and no ADR-0108 extension.
- **Do not** change the runner's budget regime, slack factor, report semantics, or how budgets are measured or enforced (`internal/suiterunner` is untouched).
- **Do not** convert shard tests to `t.Parallel()`, and do not speed up the slow integration tests themselves. Shard splits rename tests and add wrappers only; no test body changes.
- No retired-vocabulary table row, kind, boundary rule, population floor or failure message changes.
- `tests/test_go_integration_contract.sh` must pass **unmodified**.
- Every `tests/test_*.sh` has exactly one `tests/runtime-budgets.tsv` row (`TestRuntimeBudgetsCorrespondence`). Format: `<path><TAB><seconds><TAB>parallel`.
- Row sizing house rule: take the serial solo reading, round up to the next multiple of 5, add 5s, minimum 10s. The solo breach threshold is row × 3/2.
- `tests/test_go_race.sh` keeps its `60 parallel` row unless serial readings genuinely require otherwise (Task 5 has the rule). Any re-size records why (0466 precedent).
- Narrated numbers stay bound to their measurements (change 0289). Every number written into a header names the conditions it was measured under.
- gofmt uses the toolchain-declared gofmt (the `tests/README.md` "Formatting failures from the Go gate" idiom), never PATH's gofmt. Fix formatting only in `internal/githubcli/comment_integration_test.go`.
- Every measurement or mutation run uses `-count=1` (learning cached-runner-serves-a-mutated-tree).
- A mutation probe restores from a `cp` backup, **never** `git checkout --` (learning mutation-restore-needs-a-backup-copy).
- Scratch output goes to `S="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487.XXXXXX")"`, never inside the worktree. The tree must be clean apart from the task's own files.
- Stage explicit paths only. Never `git add -A` or `git commit -a`.
- Workers never edit the results file. Each task puts its measurements in its commit message body **and** in its worker return, so the coordinator can copy them into the results file.

## Review Focus

1. **A future text row whose matcher stops containing `Old` literally** (for example a case-insensitive kind) would let the `Contains` pre-filter silently skip real hits. Expected: the seal reddens, not passes. Pinned by the `prefilter_equivalence` subtest's upper-case variants in Task 2.
2. **A Go string literal that spells a retired token through escapes** (`"gate\x2dbefore"`). Expected: still detected, because the filter runs on the *unquoted* value. Pinned by the escaped-literal variants in Task 2's `prefilter_equivalence`.
3. **A `--version` wrapped onto the line after its operation reference** in the same block. Expected: still retired, because the block filter must look at the whole block. Pinned by the multi-line bound-flag case in Task 2's `prefilter_equivalence`.
4. **A renamed test still named by its old spelling somewhere maintained, or selected by two shards or by none.** Expected: no stale reference, and every tagged test matches exactly one runner. Pinned by the whole-repo grep step and the unmodified `tests/test_go_integration_contract.sh` run in Tasks 3 and 4.
5. **A solo reading taken on a loaded machine** would oversize a row and make the budget loose (learning tolerance-constant-calibrated-on-one-machine). Expected: every reading is recorded with its load average, and is re-taken when the 1-minute load is above half the core count. Pinned by the measurement steps in Tasks 3, 4 and 5.

---

### Task 1: gofmt `comment_integration_test.go`

**Tier hint:** economy, a mechanical one-file format.

**Files:**
- Modify: `internal/githubcli/comment_integration_test.go` (whitespace only)

**Interfaces:**
- Consumes: nothing.
- Produces: `gofmt -l internal/ cmd/` (declared toolchain) prints nothing. Acceptance 7.

- [ ] **Step 1: Show the red state**

```bash
W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu
cd "$W" && GOFMT="$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" && "$GOFMT" -l internal/ cmd/
```

Expected: prints exactly `internal/githubcli/comment_integration_test.go`. If it lists any other file, stop and report BLOCKED. Other drift is out of scope.

- [ ] **Step 2: Format the file**

```bash
cd "$W" && GOFMT="$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" && "$GOFMT" -w internal/githubcli/comment_integration_test.go
```

- [ ] **Step 3: Verify green and formatting-only**

```bash
cd "$W" && GOFMT="$(GOTOOLCHAIN="$(awk '$1=="toolchain"{print $2}' go.mod)" go env GOROOT)/bin/gofmt" && out="$("$GOFMT" -l internal/ cmd/)"; printf 'gofmt-l=[%s]\n' "$out"
git -C "$W" diff --ignore-all-space --exit-code -- internal/githubcli/comment_integration_test.go && echo whitespace-only
cd "$W" && go vet -tags integration ./internal/githubcli/
```

Expected: `gofmt-l=[]`, then `whitespace-only`, then vet exits 0.

- [ ] **Step 4: Commit**

```bash
git -C "$W" add internal/githubcli/comment_integration_test.go
git -C "$W" commit -m "style(githubcli): gofmt comment_integration_test.go (change 0487, from 0478)"
```

---

### Task 2: Pre-filter the retired-vocabulary seal's scan

**Tier hint:** standard. This edits a guard, so it needs a mutation-probe discipline, but any mistake can be corrected.

**Files:**
- Modify: `internal/repoguard/retired_vocabulary_test.go`. The changes go in `scanTextLine`, `scanGoLiteral` and `scanBoundFlags`, plus a new helper `blockMayBindFlag`, a new subtest function `testRetiredPrefilterEquivalence`, and one new `t.Run` line in `TestRetiredVocabularySeal`.

**Interfaces:**
- Consumes: the existing `retiredVocabulary []retiredToken`, `retiredKind` constants (`kindToken`, `kindWord`, `kindBoundFlag`, …), `textMatcher(r retiredToken) *regexp.Regexp`, `tokenRe`, `wordRe`, `scanTextLine(rel string, lineNo int, line string) []retiredHit`, `scanGoLiteral(rel string, lineNo int, lit string) []retiredHit`, `scanBoundFlags(rel string, lines []string, md bool) []retiredHit`, `retiredHit{rel, line, row, text}`.
- Produces: `blockMayBindFlag(block string) bool` and subtest `TestRetiredVocabularySeal/prefilter_equivalence`. Signatures of the existing functions are unchanged.

**TDD note (exception, recorded):** this is a performance change. Every correctness assert passes the same with or without the optimization, so the red-first oracle is the **measured baseline** (Step 1) against the after-reading (Step 7), per learning optimization-needs-a-measured-oracle. The new equivalence subtest is a guard against future divergence. It passes before and after the change, and Step 6's mutation probes show it is not decoration.

- [ ] **Step 1: Record the unpatched baseline (before any edit)**

```bash
W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu
S="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487.XXXXXX")"; echo "S=$S"
CG="$(git -C "$W" rev-parse --git-common-dir)"
cd "$W" && uptime && GOFLAGS=-modcacherw GOMODCACHE="$CG/docket-go-cache/mod" GOCACHE="$CG/docket-go-cache/build" \
  go test -race -count=1 -json ./internal/repoguard > "$S/before.json"; echo rc=$?; uptime
jq -r 'select((.Action=="pass" or .Action=="fail") and (.Test==null or .Test=="TestRetiredVocabularySeal" or .Test=="TestRetiredVocabularySeal/maintained_surfaces")) | "\(.Action)\t\(.Elapsed)s\t\(.Test // "PACKAGE internal/repoguard")"' "$S/before.json"
```

Expected: rc=0. The package reads tens of seconds and the seal most of that (grooming: 87.6s package, 65.5s seal, 55.7s `maintained_surfaces`, at load ~5–14). Write down all three numbers and both `uptime` load averages.

- [ ] **Step 2: Add the equivalence subtest**

In `TestRetiredVocabularySeal`, add one line after `t.Run("schema_walk", testRetiredSchemaWalk)`:

```go
	t.Run("prefilter_equivalence", testRetiredPrefilterEquivalence)
```

Add this function directly after `TestRetiredVocabularySeal`:

```go
// testRetiredPrefilterEquivalence pins the scan pre-filters (change 0487) to
// the matchers they guard. For every text row, on lines and Go literals that
// do and do not carry the row's spelling, the filtered scan reports a hit
// attributed to that row exactly when the row's own regexp matches. A future
// row whose matcher stops containing its Old literally (a case-insensitive
// kind, say) reddens here instead of silently scanning nothing. Go literals
// are also planted with every '-' escaped as \x2d, so the literal filter must
// run on the unquoted value. A bound flag wrapped onto the line after its
// operation reference must still bind, so the block filter must read the
// whole block.
func testRetiredPrefilterEquivalence(t *testing.T) {
	ownHits := func(hits []retiredHit, r retiredToken) int {
		n := 0
		for _, h := range hits {
			if h.row.Kind == r.Kind && h.row.Old == r.Old {
				n++
			}
		}
		return n
	}
	checked := 0
	for _, r := range retiredVocabulary {
		re := textMatcher(r)
		if re == nil {
			continue
		}
		upper := strings.ToUpper(r.Old)
		lines := []string{
			r.Old,
			"run " + r.Old + " now",
			"x" + r.Old,
			r.Old + "-x",
			"(" + r.Old + ")",
			upper,
			"run " + upper + " now",
		}
		for _, line := range lines {
			want := 0
			if re.MatchString(line) {
				want = 1
			}
			if got := ownHits(scanTextLine("skills/x/SKILL.md", 1, line), r); got != want {
				t.Errorf("row %s: text line %q: filtered scan reports %d hit(s), its matcher says %d", r.Row, line, got, want)
			}
			lit := strconv.Quote(line)
			for _, l := range []string{lit, strings.ReplaceAll(lit, "-", `\x2d`)} {
				val, err := strconv.Unquote(l)
				if err != nil {
					t.Fatalf("row %s: planted literal %s does not unquote: %v", r.Row, l, err)
				}
				want := 0
				if re.MatchString(val) {
					want = 1
				}
				if got := ownHits(scanGoLiteral("internal/p/p.go", 1, l), r); got != want {
					t.Errorf("row %s: Go literal %s: filtered scan reports %d hit(s), its matcher says %d", r.Row, l, got, want)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatalf("population floor: no text row was checked")
	}
	bound := 0
	for _, r := range retiredVocabulary {
		if r.Kind != kindBoundFlag {
			continue
		}
		bound++
		block := "the `change.claim` operation\nwith `--id <id> " + r.Old + " <v>`"
		if got := ownHits(scanBoundFlags("skills/x/SKILL.md", strings.Split(block, "\n"), true), r); got != 1 {
			t.Errorf("row %s: %q wrapped onto the line after its operation reference: %d hit(s), want 1", r.Row, r.Old, got)
		}
	}
	if bound == 0 {
		t.Fatalf("population floor: no kindBoundFlag row was checked")
	}
}
```

- [ ] **Step 3: Run it against the unfiltered code (it must already pass)**

```bash
cd "$W" && go test -count=1 -run 'TestRetiredVocabularySeal/prefilter_equivalence' -v ./internal/repoguard 2>&1 | tail -5
```

Expected: `--- PASS: TestRetiredVocabularySeal/prefilter_equivalence` and `ok`. If it fails, the subtest's premise is wrong. Fix the subtest, never the matchers. A pass here only shows the subtest agrees with today's matchers. Step 6 shows it can redden.

- [ ] **Step 4: Add the three pre-filters**

In `scanTextLine`, make the loop body start with the literal check. Rows of other kinds are skipped either way, so placing the check first is detection-neutral for every kind:

```go
func scanTextLine(rel string, lineNo int, line string) []retiredHit {
	var hits []retiredHit
	for _, r := range retiredVocabulary {
		// Pre-filter (change 0487): every text matcher embeds
		// regexp.QuoteMeta(r.Old) literally, so a line without r.Old cannot
		// match. Skipping the regexp is detection-neutral and avoids a
		// byte-by-byte regexp walk per row per line; testRetiredPrefilterEquivalence
		// pins the equivalence.
		if !strings.Contains(line, r.Old) {
			continue
		}
		re := textMatcher(r)
		if re == nil {
			continue
		}
		if re.MatchString(line) {
			hits = append(hits, retiredHit{rel, lineNo, r, strings.TrimSpace(line)})
		}
	}
	return hits
}
```

In `scanGoLiteral`, replace the two text cases only. `kindGoFlag`, `kindGoPrefix` and `kindJSONKey` stay exactly as they are:

```go
		case kindToken:
			// Pre-filter (change 0487), on the UNQUOTED value: see scanTextLine.
			hit = strings.Contains(val, r.Old) && tokenRe(r.Old).MatchString(val)
		case kindWord:
			hit = strings.Contains(val, r.Old) && wordRe(r.Old).MatchString(val)
```

Add this helper directly above `scanBoundFlags`:

```go
// blockMayBindFlag reports whether block contains some kindBoundFlag row's Old
// literally. A block without one cannot produce a bound-flag hit (each row is
// matched by tokenRe, which embeds regexp.QuoteMeta(r.Old)), so scanBoundFlags
// skips it before running the catalog operation-reference alternation over it
// (change 0487).
func blockMayBindFlag(block string) bool {
	for _, r := range retiredVocabulary {
		if r.Kind == kindBoundFlag && strings.Contains(block, r.Old) {
			return true
		}
	}
	return false
}
```

In `scanBoundFlags`, insert the skip between the `block := …` line and the `refs := …` line. The catalog fetch and its fail-closed `panic` at the top of the function stay first and unchanged:

```go
		block := strings.Join(lines[start:end], "\n")
		if !blockMayBindFlag(block) {
			start = end
			continue
		}
		refs := re.FindAllStringSubmatchIndex(block, -1)
```

- [ ] **Step 5: Run the whole package (correctness)**

```bash
cd "$W" && go test -count=1 ./internal/repoguard 2>&1 | tail -5
cd "$W" && go test -count=1 -run 'TestRetiredVocabularySeal' -v ./internal/repoguard 2>&1 | grep -E -e '^(--- |ok|FAIL)'
```

Expected: `ok`. Every seal subtest (`catalog_vocabulary`, `table_integrity`, `non_vacuity`, `negative_controls`, `maintained_surfaces`, `generator_output`, `schema_walk`, `prefilter_equivalence`) shows `--- PASS`.

- [ ] **Step 6: Mutation probes (each must redden, then restore from backup)**

Back up once and run each probe on the edited, uncommitted file:

```bash
F="$W/internal/repoguard/retired_vocabulary_test.go"; cp "$F" "$S/rv.bak"
probe(){ cd "$W" && go test -count=1 -run 'TestRetiredVocabularySeal/(non_vacuity|prefilter_equivalence)' ./internal/repoguard > "$S/probe-$1.out" 2>&1; echo "probe $1 rc=$?"; grep -E -e '^(--- FAIL|FAIL|ok)' "$S/probe-$1.out" | head -6; cp "$S/rv.bak" "$F"; }
# M1: text filter tests r.New instead of r.Old
perl -0pi -e 's/if !strings\.Contains\(line, r\.Old\) \{/if !strings.Contains(line, r.New) {/' "$F" && grep -c 'strings.Contains(line, r.New)' "$F"; probe M1
# M2: Go-literal kindToken filter tests r.New
perl -0pi -e 's/hit = strings\.Contains\(val, r\.Old\) && tokenRe/hit = strings.Contains(val, r.New) \&\& tokenRe/' "$F" && grep -c 'Contains(val, r.New) && tokenRe' "$F"; probe M2
# M3: invert the block skip
perl -0pi -e 's/if !blockMayBindFlag\(block\) \{/if blockMayBindFlag(block) {/' "$F" && grep -c 'if blockMayBindFlag(block) {' "$F"; probe M3
cmp "$F" "$S/rv.bak" && echo restored
```

Expected: each `grep -c` prints `1`, which proves the mutation landed (learning assert-detects-removal-not-replacement). Each probe prints `rc=1` with `--- FAIL: TestRetiredVocabularySeal/non_vacuity` in its output. M1 and M3 also show `--- FAIL: …/prefilter_equivalence`, and M2 shows it for its Go-literal case. The final line prints `restored`. A probe that stays green is a defect. Stop and investigate it, never note it as a residual. Keep each probe's FAIL lines for the commit body.

- [ ] **Step 7: Record the after reading (same invocation as Step 1)**

```bash
cd "$W" && uptime && GOFLAGS=-modcacherw GOMODCACHE="$CG/docket-go-cache/mod" GOCACHE="$CG/docket-go-cache/build" \
  go test -race -count=1 -json ./internal/repoguard > "$S/after.json"; echo rc=$?; uptime
jq -r 'select((.Action=="pass" or .Action=="fail") and (.Test==null or .Test=="TestRetiredVocabularySeal" or .Test=="TestRetiredVocabularySeal/maintained_surfaces")) | "\(.Action)\t\(.Elapsed)s\t\(.Test // "PACKAGE internal/repoguard")"' "$S/after.json"
```

Expected: rc=0, `TestRetiredVocabularySeal` in single-digit seconds, and the package well under its Step 1 reading (grooming prototype: 1.4s seal, 25.8s package). An unchanged or worse number is a **red result** even with every assert green. Stop and investigate.

- [ ] **Step 8: Commit with the evidence**

```bash
git -C "$W" add internal/repoguard/retired_vocabulary_test.go
git -C "$W" commit -F - <<'EOF'
perf(repoguard): pre-filter the retired-vocabulary seal scan (change 0487, from 0484/0485)

<paste: Step 1 before readings + load, Step 7 after readings + load,
 and the M1/M2/M3 probe FAIL lines>
EOF
```

Fill the body with the real numbers and probe lines before committing. Never commit the placeholder text.

---

### Task 3: Split the closeout shard in two

**Tier hint:** standard.

**Files:**
- Modify: `internal/app/finalize_closeout_integration_test.go` (rename 8 test functions and their doc comments; no body changes)
- Modify: `tests/test_go_integration_app_closeout.sh` (header comment only; its prefix stays `TestIntegrationFinalizeCloseout`)
- Create: `tests/test_go_integration_app_archive.sh`
- Modify: `tests/runtime-budgets.tsv` (re-size the closeout row from its new reading; add the archive row after it)

**Interfaces:**
- Consumes: `tests/lib/go-integration-shard.sh` (`SHARD_PKG`, `SHARD_PREFIX`, `SHARD_MODE`, `shard_inspect_maybe`, `run_integration_shard`).
- Produces: shard prefix `TestIntegrationFinalizeArchive` (8 tests) and shard prefix `TestIntegrationFinalizeCloseout` (13 tests). Neither is a prefix of the other, and no other shard prefix in `internal/app` is a prefix of either.

**Grouping.** The root/stacked/lifecycle tests move to the fresh prefix: `RootCarry`, `StackedMerged`, `StackedParentBranchIdentity`, `StackedPreservation`, `Ordinary`, `Idempotent`, `Refusals`, `NeverEditsAuthoredBytes` (about 56s loaded at grooming). The notes, backlink and unrelated-record tests keep `TestIntegrationFinalizeCloseout`: `Notes*`, `Backlink*`, `Unrelated*`, `NoNotesEmitsNoSection` (about 52s loaded). This direction keeps `TestIntegrationFinalizeCloseoutBacklinkLegDocketMode`, which `skills/docket-convention/references/close-out.md` and its embedded mirror cite, under its current name. It also keeps the existing `-run 'TestIntegrationFinalizeCloseoutUnrelated'` comment in the test file true.

**TDD note:** the failing-first check is the contract. After the rename and before the new wrapper exists, `tests/test_go_integration_contract.sh` must go red ("none unmatched"). That proves it sees the 8 orphaned tests.

- [ ] **Step 1: Baseline listing**

```bash
W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu
S="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487.XXXXXX")"; echo "S=$S"
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeCloseout' ./internal/app | grep -c -E -e '^Test'
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeArchive' ./internal/app | grep -c -E -e '^Test'
```

Expected: `21`, then `0` (the fresh prefix is unused).

- [ ] **Step 2: Rename the 8 tests**

```bash
cd "$W" && perl -pi -e 's/\bTestIntegrationFinalizeCloseout(RootCarry|StackedMerged|StackedParentBranchIdentity|StackedPreservation|Ordinary|Idempotent|Refusals|NeverEditsAuthoredBytes)\b/TestIntegrationFinalizeArchive$1/g' internal/app/finalize_closeout_integration_test.go
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeArchive' ./internal/app | grep -E -e '^Test'
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeCloseout' ./internal/app | grep -c -E -e '^Test'
```

Expected: the 8 `TestIntegrationFinalizeArchive…` names, then `13`.

- [ ] **Step 3: Prove the contract sees the orphans (red)**

```bash
cd "$W" && bash tests/test_go_integration_contract.sh > "$S/contract-red.out" 2>&1; echo rc=$?; grep -E -e '^NOT OK' "$S/contract-red.out"
```

Expected: `rc=1` with `NOT OK - every tagged test matches exactly one runner (none unmatched)`.

- [ ] **Step 4: Create the archive wrapper**

```bash
cd "$W" && cp -p tests/test_go_integration_app_closeout.sh tests/test_go_integration_app_archive.sh
```

Then replace the whole content of `tests/test_go_integration_app_archive.sh` with:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_archive.sh — Go integration shard (change 0487;
# split out of tests/test_go_integration_app_closeout.sh): the finalize closeout
# root/stacked/lifecycle real-repository tests (root carry, stacked merge and
# parent-branch identity, stacked preservation, ordinary, idempotent, refusals,
# authored-bytes preservation), behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeArchive. Sibling of the notes/backlink/unrelated-record
# half in tests/test_go_integration_app_closeout.sh.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationFinalizeArchive"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

The `# docket-suite: go` line stays line 2, and the `assert` line stays byte-identical to the canonical one.

- [ ] **Step 5: Update the closeout wrapper header**

In `tests/test_go_integration_app_closeout.sh`, replace the comment lines from `# tests/test_go_integration_app_closeout.sh — Go integration shard (change 0333):` through `# ^TestIntegrationFinalizeCloseout. Declarations only — execution and inspection live in` with:

```bash
# tests/test_go_integration_app_closeout.sh — Go integration shard (change 0333;
# split by change 0487): the finalize closeout notes, backlink-leg, and
# unrelated-record real-repository tests, behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeCloseout. The root/stacked/lifecycle half lives in
# tests/test_go_integration_app_archive.sh. Declarations only — execution and inspection live in
```

Leave every non-comment line unchanged (`SHARD_PREFIX="TestIntegrationFinalizeCloseout"`, `SHARD_MODE="normal"`).

- [ ] **Step 6: Contract green, no stale references**

```bash
cd "$W" && bash tests/test_go_integration_contract.sh > "$S/contract.out" 2>&1; echo rc=$?; grep -E -e '^NOT OK' "$S/contract.out"
cd "$W" && hits="$(git grep -n -E -e 'TestIntegrationFinalizeCloseout(RootCarry|StackedMerged|StackedParentBranchIdentity|StackedPreservation|Ordinary|Idempotent|Refusals|NeverEditsAuthoredBytes)\b' -- . ':!docs/')"; printf 'stale=[%s]\n' "$hits"
```

Expected: `rc=0` with no `NOT OK` lines, then `stale=[]`. `docs/` is excluded because plans and results are point-in-time records (AGENTS.md "Comments and cross-references"). Any non-`docs/` hit gets renamed too.

- [ ] **Step 7: Serial solo readings (warm, idle)**

```bash
cd "$W" && for f in archive closeout; do bash "tests/test_go_integration_app_$f.sh" > "$S/warm-$f.out" 2>&1; echo "warm $f rc=$?"; done
cd "$W" && for f in archive closeout; do uptime; /usr/bin/time -p bash "tests/test_go_integration_app_$f.sh" > "$S/solo-$f.out" 2>&1; echo "solo $f rc=$?"; grep -E -e '^real' "$S/solo-$f.out"; uptime; done
sysctl -n hw.ncpu
```

Expected: every rc=0. Each wrapper's `ok - every selected test actually ran and passed (N declared)` shows N=8 for archive and N=13 for closeout. If a reading's 1-minute load is above half of `hw.ncpu`, wait and re-take it. Record each `real` value with its two load averages.

- [ ] **Step 8: Size and write the budget rows**

For each half, row = max(10, ceil5(solo) + 5), where ceil5 rounds up to the next multiple of 5. Example: solo 36.2s gives 40 + 5 = **45**, and the threshold is 67.5s. Edit `tests/runtime-budgets.tsv`:
- Set the existing `tests/test_go_integration_app_closeout.sh` row's seconds to the closeout value.
- Insert directly after it a new line `tests/test_go_integration_app_archive.sh<TAB><archive value><TAB>parallel` with real TAB characters. For example, with `printf 'tests/test_go_integration_app_archive.sh\t%s\tparallel\n' <value>` as the content of the inserted line.

Then:

```bash
cd "$W" && grep -n -E -e 'integration_app_(closeout|archive)\.sh' tests/runtime-budgets.tsv | cat -t
cd "$W" && go test -count=1 -run 'TestRuntimeBudgetsCorrespondence' ./internal/repoguard
```

Expected: two rows separated by `^I` (TAB), then `ok`. Each half's solo must sit at or under its row (so well under row × 3/2). If a half's solo exceeds 60s, the grouping is badly unbalanced. Move whole tests between the halves (renaming only), re-measure, and say so in the commit body.

- [ ] **Step 9: Commit with the evidence**

```bash
git -C "$W" add internal/app/finalize_closeout_integration_test.go tests/test_go_integration_app_closeout.sh tests/test_go_integration_app_archive.sh tests/runtime-budgets.tsv
git -C "$W" commit -F - <<'EOF'
test(integration): split the closeout shard into archive + closeout halves (change 0487, from 0475)

<paste: per-half solo readings with load averages, hw.ncpu, the chosen rows
 and their x3/2 thresholds, and the contract red/green rc lines>
EOF
```

Fill in the real values before committing.

---

### Task 4: Split the rebaserecovery shard in two

**Tier hint:** standard.

**Files:**
- Modify: `internal/app/finalize_rebase_integration_test.go` (rename the 5 forward-refresh tests and their doc comments; no body changes)
- Modify: `tests/test_go_integration_app_rebaserecovery.sh` (header comment only; its prefix stays `TestIntegrationFinalizeRebaseRecovery`)
- Create: `tests/test_go_integration_app_rebaserefresh.sh`
- Modify: `tests/runtime-budgets.tsv` (re-size the rebaserecovery row from its new reading; add the rebaserefresh row after it)

**Interfaces:**
- Consumes: `tests/lib/go-integration-shard.sh`, and Task 3's `tests/runtime-budgets.tsv` (append only; do not disturb Task 3's rows).
- Produces: shard prefix `TestIntegrationFinalizeRebaseForwardRefresh` (5 tests) and shard prefix `TestIntegrationFinalizeRebaseRecovery` (12 tests). Disjoint from each other and from `TestIntegrationFinalizeRebaseGate`, `TestIntegrationFinalizeRebaseOps` and `TestIntegrationFinalizeRebasePublished`.

**Grouping.** The forward-refresh tests move: `TestIntegrationFinalizeRebaseRecoveryForwardRefresh`, `…ForwardRefreshContended`, `…ForwardRefreshRefusals`, `…ForwardRefreshInterruptions` and `…ForwardRefreshUnchangedRetests` become `TestIntegrationFinalizeRebaseForwardRefresh…` (about 28s loaded at grooming). The checkpoint, carry, resume and attempt tests keep `TestIntegrationFinalizeRebaseRecovery` (12 tests, about 45s loaded), including the two `CarryHelper` tests in `finalize_preservation_integration_test.go`.

**TDD note:** same as Task 3. The contract must go red on the orphaned tests before the new wrapper exists.

- [ ] **Step 1: Baseline listing**

```bash
W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu
S="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487.XXXXXX")"; echo "S=$S"
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeRebaseRecovery' ./internal/app | grep -c -E -e '^Test'
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeRebaseForwardRefresh' ./internal/app | grep -c -E -e '^Test'
cd "$W" && git grep -l -E -e 'TestIntegrationFinalizeRebaseRecoveryForwardRefresh' -- . ':!docs/'
```

Expected: `17`, then `0`, then exactly `internal/app/finalize_rebase_integration_test.go`. If the grep lists other non-`docs/` files, include them in Step 2's rename.

- [ ] **Step 2: Rename the 5 tests**

```bash
cd "$W" && perl -pi -e 's/\bTestIntegrationFinalizeRebaseRecoveryForwardRefresh/TestIntegrationFinalizeRebaseForwardRefresh/g' internal/app/finalize_rebase_integration_test.go
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeRebaseForwardRefresh' ./internal/app | grep -E -e '^Test'
cd "$W" && go test -tags integration -list '^TestIntegrationFinalizeRebaseRecovery' ./internal/app | grep -c -E -e '^Test'
```

Expected: the 5 `TestIntegrationFinalizeRebaseForwardRefresh…` names, then `12`.

- [ ] **Step 3: Prove the contract sees the orphans (red)**

```bash
cd "$W" && bash tests/test_go_integration_contract.sh > "$S/contract-red.out" 2>&1; echo rc=$?; grep -E -e '^NOT OK' "$S/contract-red.out"
```

Expected: `rc=1` with `NOT OK - every tagged test matches exactly one runner (none unmatched)`.

- [ ] **Step 4: Create the rebaserefresh wrapper**

```bash
cd "$W" && cp -p tests/test_go_integration_app_rebaserecovery.sh tests/test_go_integration_app_rebaserefresh.sh
```

Then replace the whole content of `tests/test_go_integration_app_rebaserefresh.sh` with:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_rebaserefresh.sh — Go integration shard (change 0487;
# split out of tests/test_go_integration_app_rebaserecovery.sh): the finalize rebase
# forward-refresh real-repository tests (forward refresh, contended, refusals,
# interruptions, unchanged-retests), behind the `integration` build tag, prefix
# ^TestIntegrationFinalizeRebaseForwardRefresh. Sibling of the checkpoint/carry/
# resume half in tests/test_go_integration_app_rebaserecovery.sh.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationFinalizeRebaseForwardRefresh"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

- [ ] **Step 5: Update the rebaserecovery wrapper header**

In `tests/test_go_integration_app_rebaserecovery.sh`, replace the line `# ^TestIntegrationFinalizeRebaseRecovery. Split out of tests/test_go_integration_app_rebase.sh.` with:

```bash
# ^TestIntegrationFinalizeRebaseRecovery. Split out of tests/test_go_integration_app_rebase.sh;
# change 0487 moved the forward-refresh tests to tests/test_go_integration_app_rebaserefresh.sh.
```

Leave every non-comment line unchanged.

- [ ] **Step 6: Contract green, no stale references**

```bash
cd "$W" && bash tests/test_go_integration_contract.sh > "$S/contract.out" 2>&1; echo rc=$?; grep -E -e '^NOT OK' "$S/contract.out"
cd "$W" && hits="$(git grep -n -E -e 'TestIntegrationFinalizeRebaseRecoveryForwardRefresh' -- . ':!docs/')"; printf 'stale=[%s]\n' "$hits"
```

Expected: `rc=0` with no `NOT OK` lines, then `stale=[]`.

- [ ] **Step 7: Serial solo readings (warm, idle)**

```bash
cd "$W" && for f in rebaserefresh rebaserecovery; do bash "tests/test_go_integration_app_$f.sh" > "$S/warm-$f.out" 2>&1; echo "warm $f rc=$?"; done
cd "$W" && for f in rebaserefresh rebaserecovery; do uptime; /usr/bin/time -p bash "tests/test_go_integration_app_$f.sh" > "$S/solo-$f.out" 2>&1; echo "solo $f rc=$?"; grep -E -e '^real' "$S/solo-$f.out"; uptime; done
sysctl -n hw.ncpu
```

Expected: every rc=0, with declared counts of 5 (rebaserefresh) and 12 (rebaserecovery). If a reading's 1-minute load is above half of `hw.ncpu`, wait and re-take it. Record each `real` value with its load averages.

- [ ] **Step 8: Size and write the budget rows**

Use the same rule as Task 3: row = max(10, ceil5(solo) + 5). Set the `tests/test_go_integration_app_rebaserecovery.sh` row's seconds to its value. Insert directly after it `tests/test_go_integration_app_rebaserefresh.sh<TAB><value><TAB>parallel` with real TAB characters.

```bash
cd "$W" && grep -n -E -e 'integration_app_(rebaserecovery|rebaserefresh)\.sh' tests/runtime-budgets.tsv | cat -t
cd "$W" && go test -count=1 -run 'TestRuntimeBudgetsCorrespondence' ./internal/repoguard
```

Expected: two TAB-separated rows, then `ok`. Each half's solo must sit at or under its row. If the checkpoint/carry half's solo exceeds 60s, move `TestIntegrationFinalizeRebaseRecoveryCheckpointInvalidation` (17.7s loaded at grooming) into its own fresh-prefix group with a new wrapper (renaming only), re-measure, and say so in the commit body.

- [ ] **Step 9: Commit with the evidence**

```bash
git -C "$W" add internal/app/finalize_rebase_integration_test.go tests/test_go_integration_app_rebaserecovery.sh tests/test_go_integration_app_rebaserefresh.sh tests/runtime-budgets.tsv
git -C "$W" commit -F - <<'EOF'
test(integration): split forward-refresh out of the rebaserecovery shard (change 0487, from 0476)

<paste: per-half solo readings with load averages, hw.ncpu, the chosen rows
 and their x3/2 thresholds, and the contract red/green rc lines>
EOF
```

Fill in the real values before committing.

---

### Task 5: Serial-confirm the race gate, re-bind its narration, and prove the two-gate fix

**Tier hint:** standard. The work is measurement-heavy and the edit is a one-paragraph header update.

**Depends on:** Task 2 (the seal pre-filter must be on the branch).

**Files:**
- Modify: `tests/test_go_race.sh` (header comment, the BACKSTOP TIMEOUT paragraph only)
- Modify: `tests/runtime-budgets.tsv` **only** if Step 2's rule requires it

**Interfaces:**
- Consumes: the Task 2 commit on the branch.
- Produces: an updated narrated worst-package sentence in `tests/test_go_race.sh`, plus the race-gate solo reading and the before/after two-gate readings in the commit body.

**TDD note (exception, recorded):** measurement-only acceptance. The oracles are wall clock and the per-package `Elapsed` values from `go test -json`, not an assert (learning optimization-needs-a-measured-oracle).

- [ ] **Step 1: Idle per-package reading under the header's stated conditions**

```bash
W=/Users/homer/dev/docket/.worktrees/bring-test-go-race-rebaserecovery-and-closeout-back-under-bu
S="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487.XXXXXX")"; echo "S=$S"
CG="$(git -C "$W" rev-parse --git-common-dir)"
cd "$W" && uptime && GOFLAGS=-modcacherw GOMODCACHE="$CG/docket-go-cache/mod" GOCACHE="$CG/docket-go-cache/build" GOMAXPROCS=2 \
  go test -race -count=1 -p 2 -timeout 8m -json ./... > "$S/race-p2.json"; echo rc=$?; uptime
jq -r 'select(.Test==null and (.Action=="pass" or .Action=="fail")) | "\(.Elapsed)\t\(.Action)\t\(.Package)"' "$S/race-p2.json" | sort -rn | head -6
```

Expected: rc=0. Record the top six packages with their `Elapsed` and both load averages. `internal/repoguard` should no longer be far ahead of the rest.

- [ ] **Step 2: Serial solo reading of the gate file**

```bash
cd "$W" && bash tests/test_go_race.sh > "$S/race-warm.out" 2>&1; echo warm rc=$?
cd "$W" && uptime; /usr/bin/time -p bash tests/test_go_race.sh > "$S/race-solo.out" 2>&1; echo solo rc=$?; grep -E -e '^real' "$S/race-solo.out"; uptime
```

Expected: rc=0 and `real` under 90s (60 × 3/2). Grooming projects it under 60s. Re-take the reading if the 1-minute load is above half of `sysctl -n hw.ncpu`. Then apply this rule. **Keep the `60 parallel` row** when solo ≤ 75s (at least 15s of headroom under the 90s threshold). Above 75s, re-size the row with the house rule (ceil5(solo) + 5) and put the reason and readings in the commit body (0466 precedent).

- [ ] **Step 3: Re-bind the narrated worst-package number**

In `tests/test_go_race.sh`, the BACKSTOP TIMEOUT paragraph currently says:

```
# partitioned that package; the measured worst default-corpus package is now
# internal/cli at 24.0s (-p 2, GOMAXPROCS=2, load 2-5). The backstop and its
# floor in internal/repoguard keep 0465's larger 48.7s input, so the margin
# only grew.
```

Replace those four lines with the following, filling `<pkg>`, `<secs>` and `<load>` from Step 1's top line:

```
# partitioned that package, and the measured worst default-corpus package
# became internal/cli at 24.0s (-p 2, GOMAXPROCS=2, load 2-5). Change 0487
# pre-filtered internal/repoguard's retired-vocabulary seal, which had grown
# to dominate the gate, and re-measured: the worst default-corpus package is
# now <pkg> at <secs>s (-p 2, GOMAXPROCS=2, load <load>). The backstop and
# its floor in internal/repoguard keep 0465's larger 48.7s input, so the
# margin only grew.
```

If Step 1's worst package is at or above 48.7s, the "margin only grew" claim is false. Stop and return BLOCKED with the reading: re-reasoning the backstop floor is out of scope.

Then:

```bash
cd "$W" && go test -count=1 ./internal/repoguard 2>&1 | tail -3
```

Expected: `ok`. This includes `TestRaceGatePassesTimeoutBackstopBelowGoDefault` and the comment-anchor guards.

- [ ] **Step 4: Two-gate proof — BEFORE (untouched merge-base checkout)**

Reproduce the #0485 shape: one full suite plus two concurrent race gates, started together on one machine. Run BEFORE and AFTER one after the other, never at the same time, so each sees comparable load. The advisory budget state goes to scratch so the proof does not pollute the shared store.

```bash
BASE="$(git -C "$W" merge-base HEAD origin/main)"; echo "BASE=$BASE"
BD="$(mktemp -d "${TMPDIR:-/tmp}/docket-0487-before.XXXXXX")"
git -C "$W" worktree add --detach "$BD" "$BASE"
twogate(){ # $1 = checkout dir, $2 = label
  ( while :; do uptime; sleep 15; done ) > "$S/load-$2.log" 2>&1 & LP=$!
  ( cd "$1" && DOCKET_RUNTESTS_STATE="$S/state-$2" go run ./cmd/docket development test > "$S/suite-$2.log" 2>&1; echo "suite rc=$?" >> "$S/suite-$2.log" ) & SP=$!
  for n in 1 2; do ( cd "$1" && GOFLAGS=-modcacherw GOMODCACHE="$CG/docket-go-cache/mod" GOCACHE="$CG/docket-go-cache/build" go test -race -count=1 -timeout 8m -json ./... > "$S/race-$2-$n.json" 2>&1; echo "race$n rc=$?" > "$S/race-$2-$n.rc" ) & eval "RP$n=\$!"; done
  wait "$SP" "$RP1" "$RP2"; kill "$LP" 2>/dev/null
  tail -1 "$S/suite-$2.log"; cat "$S/race-$2-1.rc" "$S/race-$2-2.rc"
  for n in 1 2; do echo "== $2 race$n"; jq -r 'select(.Test==null and (.Action=="pass" or .Action=="fail")) | "\(.Elapsed)\t\(.Action)\t\(.Package)"' "$S/race-$2-$n.json" | sort -rn | head -5; done
  grep -E -e 'internal/repoguard' "$S/suite-$2.log" | head -5
  awk -F'load averages?: ' 'NF>1{print $2}' "$S/load-$2.log" | sort -t' ' -k1,1rn | head -1 | sed 's/^/peak 1-min load: /'
}
twogate "$BD" before
```

Expected: `internal/repoguard` is the outlier in the before race runs (grooming: 87.6s alone; under #0485's load it hit the 480s backstop). A red or timed-out BEFORE run is an acceptable observation. Record it as is.

- [ ] **Step 5: Two-gate proof — AFTER (branch head), then clean up**

```bash
twogate "$W" after
git -C "$W" worktree remove --force "$BD" && git -C "$W" worktree prune
git -C "$W" status --porcelain
```

Expected: both after race runs `rc=0`, with `internal/repoguard` in line with the other slow packages and nowhere near 480s. The suite's final line is `suite rc=0`. `git status --porcelain` shows only this task's own edits. If an AFTER race run still hits the backstop, **do not widen scope**. Keep the readings and write in the commit body: "confirmed follow-up: repoguard backstop still trips under two concurrent gates", for the coordinator to record in the results file.

- [ ] **Step 6: Commit with the evidence**

```bash
git -C "$W" add tests/test_go_race.sh
# only if Step 2 required a re-size:  git -C "$W" add tests/runtime-budgets.tsv
git -C "$W" commit -F - <<'EOF'
docs(tests): re-bind test_go_race's worst-package reading after the seal fix (change 0487, from 0484/0485)

<paste: Step 1 top-6 packages + load; Step 2 solo real + load + row decision;
 Step 4/5 before/after per-run repoguard Elapsed, top packages, suite rc,
 race rcs, and peak 1-min load for each>
EOF
```

Fill in the real values before committing.

---

## Build-gate expectations (for the coordinator, not a task)

`docket-build`'s single full-suite gate runs `build.test_command` (`go run ./cmd/docket development test`) on the final head. On top of a green suite, it must read the budget report (AGENTS.md "Read the budget report even on a green run"). There must be **no** `SERIAL CONFIRMED OVER BUDGET:` line naming `tests/test_go_race.sh`, `tests/test_go_integration_app_closeout.sh`, `tests/test_go_integration_app_archive.sh`, `tests/test_go_integration_app_rebaserecovery.sh` or `tests/test_go_integration_app_rebaserefresh.sh` (spec acceptance 2). The results file collects, from the task commit bodies, the readings for spec acceptance 3–7: the race-gate and per-shard solo readings with thresholds, the seal before/after, the mutation-probe output, the two-gate before/after with load, and the empty `gofmt -l`.
