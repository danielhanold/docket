<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0504 — Guard two unproven invariants: the testdata ignore negation and the root-anchored receipt read](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0504-guard-two-unproven-invariants-the-testdata-ignore-negation-a.md)**
<!-- docket:backlink:end -->
# Guard Two Unproven Invariants Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two mutation-proven regression guards: (1) no tracked file is hidden by the repository's own committed `.gitignore` rules, and (2) a descendant commit's seed receipt never authorizes the metadata seed root.

**Architecture:** Guard 1 is a new default-build test file in `internal/repoguard` that runs one `git ls-files -z --cached --ignored --exclude-per-directory=.gitignore` probe through a shared helper, plus a committed control that builds a throwaway repository and proves the helper actually detects an ignored tracked file. Guard 2 re-applies only the test hunk of commit `9c5ced015` to the integration-tagged ownership shard. No production code changes.

**Tech Stack:** Go (`testing`, `os/exec`), git CLI, `internal/testsupport` temp-dir fixture.

**Spec:** `docs/superpowers/specs/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-design.md` (on the `docket` metadata branch; the synchronized copy is at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-guard-two-unproven-invariants-the-testdata-ignore-negation-a-design.md`).

## Global Constraints

- Test coverage only. Do not change `verifyMetadataOwnership`, `internal/app/metadata_ownership.go`, any `.gitignore`, the managed docket gitignore block, or anything under `testdata/repositories/`. Every mutation below is temporary and must be reverted before commit.
- Guard 1 probe flag is `--exclude-per-directory=.gitignore`, **never** `--exclude-standard` (machine-local `.git/info/exclude` and user-global `core.excludesFile` must neither redden nor mask the guard).
- Guard 1 is fail-closed: unresolvable root, git failing to start, or git exiting non-zero fails the test (`t.Fatal`). A failed probe is never read as an empty list (learning `probe-error-is-not-clean-absence`).
- Every temp dir in `internal/repoguard` tests comes from `testsupport.TempDir(t)`, never `t.TempDir()` / `os.MkdirTemp` (change 0373 guard `TestRealProcessPackagesUseFixtureTempDir`).
- Guard 2: re-apply **only** the `internal/app/repoownership_integration_test.go` hunk of `9c5ced015`; the F4 `verifyLegacyEquivalence` refactor in the same commit stays out. Do not rewrite the fixtures' assertions.
- Every focused and mutation run uses `-count=1` (learning `cached-runner-serves-a-mutated-tree`). Run focused tests under `timeout --kill-after=10s 10m`.
- Never run `go test -tags integration ./internal/app/` without `-run`; the package's TestMain refuses an unfiltered integration run.
- A cross-reference in new source anchors on a symbol name or a quoted clause, never a line number.
- Mutation evidence (command, red output excerpt, restore confirmation) goes in the task's commit message body, so the results file can cite it.

## Review Focus

1. **Vacuous probe.** If the git flags ever stop loading the `.gitignore` rules, the main assert passes on an empty list. Pinned by the control in Task 1 (an ignored tracked file must be reported).
2. **Negation honored.** A probe that reports every pattern match regardless of a later negation would redden on today's tree for the wrong reason, or a differently-wrong probe could report negated files. Pinned by the control's nested-negation step in Task 1 (the `testdata/repositories` shape must report nothing).
3. **Machine-local excludes leak in.** Swapping in `--exclude-standard` would let a developer's `.git/info/exclude` or `core.excludesFile` redden the guard. Pinned by the control's local-excludes step in Task 1.
4. **Inherited git environment.** A `GIT_DIR` / `GIT_WORK_TREE` / `GIT_INDEX_FILE` exported by a hook or outer tool would point `git -C <dir>` at a different repository. The helper scrubs those variables (Task 1).
5. **Tip-anchored trailer scan.** A refactor that scans or matches trailers at the tip would let a descendant's receipt authorize the root. Pinned by the two restored fixtures in Task 2, proven by mutation.

---

### Task 1: Guard 1 — no tracked file is ignored by the committed `.gitignore` rules

**Files:**
- Create: `internal/repoguard/tracked_ignored_test.go`

**Interfaces:**
- Consumes: `repoguard.Root() (string, error)` (`internal/repoguard/repoguard.go`); `testsupport.TempDir(t testing.TB) string` (`internal/testsupport/testsupport.go`).
- Produces: test-local helpers `trackedButIgnored(dir string, extraEnv []string) ([]string, error)` and `scrubGitRepoEnv(env []string) []string`; tests `TestNoTrackedFileIsIgnoredByCommittedRules` and `TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile`. Nothing outside this file depends on them.

- [ ] **Step 1: Write the control test first (it cannot compile yet)**

Create `internal/repoguard/tracked_ignored_test.go` with only the control test and imports:

```go
package repoguard

// Guard: no tracked file is hidden by the repository's own committed ignore
// rules (change 0504). A file that is tracked while a committed .gitignore rule
// still matches it is in the index only because of a one-time `git add -f`; a
// sibling added later is silently skipped. The durable form is a committed
// negation in a nested .gitignore beside the files, outside the managed docket
// block (learning gitignore-guarantee-must-be-committed). The live instance is
// the testdata/repositories/.gitignore negation that keeps the frozen
// .docket.local.yml fixtures committable.
//
// The probe asks git, never a path list: `git ls-files -z --cached --ignored
// --exclude-per-directory=.gitignore`. Only the repository's own .gitignore files
// decide — never --exclude-standard, so a developer's .git/info/exclude or
// user-global core.excludesFile can neither redden nor mask the guard.
//
// RESIDUAL (undetectable, not unprobed): a brand-new file that is ignored and
// never added is absent from every clone's committed state, so no test over the
// repository can see it. This guard catches every TRACKED instance and the
// deletion of any negation that protects one.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile is the committed
// non-vacuity control for TestNoTrackedFileIsIgnoredByCommittedRules: it runs the
// SAME helper over a throwaway repository and proves the probe reports an ignored
// tracked file, honors a nested negation, and ignores machine-local excludes.
func TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	elsewhere := testsupport.TempDir(t)
	env := []string{
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=docket test",
		"GIT_AUTHOR_EMAIL=test@docket.invalid",
		"GIT_COMMITTER_NAME=docket test",
		"GIT_COMMITTER_EMAIL=test@docket.invalid",
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(scrubGitRepoEnv(os.Environ()), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	probe := func(step string, want []string) {
		t.Helper()
		got, err := trackedButIgnored(dir, env)
		if err != nil {
			t.Fatalf("%s: probe failed: %v", step, err)
		}
		if len(got) == 0 && len(want) == 0 {
			return
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: trackedButIgnored = %q, want %q", step, got, want)
		}
	}

	git("init", "-q")
	write("keep/local.yml", "a: 1\n")
	write("keep/other.txt", "x\n")
	git("add", ".")
	git("commit", "-q", "-m", "seed")
	probe("baseline (no ignore rules)", nil)

	// A committed rule that matches an already-tracked file: the probe must name it.
	write(".gitignore", "local.yml\n")
	git("add", ".gitignore")
	git("commit", "-q", "-m", "ignore local.yml")
	probe("committed rule matches a tracked file", []string{"keep/local.yml"})

	// A committed nested negation beside the file rescues it (the
	// testdata/repositories/.gitignore shape): nothing is reported.
	write("keep/.gitignore", "!local.yml\n")
	git("add", "keep/.gitignore")
	git("commit", "-q", "-m", "negate local.yml beside it")
	probe("nested negation rescues the file", nil)

	// Machine-local excludes never decide: neither .git/info/exclude nor a
	// core.excludesFile matching a tracked file is reported.
	write(".git/info/exclude", "other.txt\n")
	globalIgnore := filepath.Join(elsewhere, "global-ignore")
	if err := os.WriteFile(globalIgnore, []byte("other.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("config", "core.excludesFile", globalIgnore)
	probe("machine-local excludes are not consulted", nil)
}
```

- [ ] **Step 2: Run the control to verify it fails**

Run: `timeout --kill-after=10s 10m go test -count=1 -run '^TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile$' ./internal/repoguard/`
Expected: FAIL to build with `undefined: scrubGitRepoEnv` and `undefined: trackedButIgnored`.

- [ ] **Step 3: Add the helpers and the main assert**

Append to `internal/repoguard/tracked_ignored_test.go`:

```go
// TestNoTrackedFileIsIgnoredByCommittedRules asserts that git reports no tracked
// file as ignored by the repository's own committed .gitignore files.
func TestNoTrackedFileIsIgnoredByCommittedRules(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	hidden, err := trackedButIgnored(root, nil)
	if err != nil {
		t.Fatalf("tracked-but-ignored probe failed (a failed probe is never an empty list): %v", err)
	}
	if len(hidden) > 0 {
		t.Fatalf("%d tracked file(s) are ignored by the repository's own committed .gitignore rules:\n  %s\n"+
			"A tracked file that an ignore rule still matches is in the index only because of a one-time "+
			"`git add -f`; a sibling added later is silently skipped. Remedy: add a committed negation "+
			"(for example `!<name>`) in a nested .gitignore beside the files, outside the managed docket "+
			"block, or stop tracking the file (learning: gitignore-guarantee-must-be-committed).",
			len(hidden), strings.Join(hidden, "\n  "))
	}
}

// trackedButIgnored returns, sorted and slash-separated, every tracked file in the
// repository at dir that the repository's own .gitignore files ignore. Only
// per-directory .gitignore files are consulted (never --exclude-standard). The
// output is NUL-delimited (-z), so paths are never C-quoted. Any git failure is
// returned as an error, never as an empty list. extraEnv is appended last.
func trackedButIgnored(dir string, extraEnv []string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "ls-files", "-z", "--cached", "--ignored",
		"--exclude-per-directory=.gitignore")
	cmd.Env = append(scrubGitRepoEnv(os.Environ()), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w: %s", dir, err, strings.TrimSpace(stderr.String()))
	}
	var paths []string
	for _, p := range strings.Split(stdout.String(), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// scrubGitRepoEnv drops the variables that would redirect git away from the
// repository named by -C (an outer hook or tool may export them).
func scrubGitRepoEnv(env []string) []string {
	var out []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR":
			continue
		}
		out = append(out, kv)
	}
	return out
}
```

- [ ] **Step 4: Run both tests to verify they pass**

Run: `timeout --kill-after=10s 10m go test -count=1 -run '^(TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile|TestNoTrackedFileIsIgnoredByCommittedRules)$' -v ./internal/repoguard/`
Expected: PASS for both. If the main assert reddens on the unmodified tree, stop and report it as a finding (the spec's groom-time dry run found the tree clean); do not add exclusions.

- [ ] **Step 5: Mutation — delete the testdata negation, watch the main assert redden**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-two-unproven-invariants-the-testdata-ignore-negation-a
rm testdata/repositories/.gitignore
test ! -e testdata/repositories/.gitignore && echo MUTATION-LANDED
timeout --kill-after=10s 10m go test -count=1 -run '^TestNoTrackedFileIsIgnoredByCommittedRules$' ./internal/repoguard/
git checkout -- testdata/repositories/.gitignore
git status --porcelain testdata/
```

Expected: `MUTATION-LANDED`; the test FAILS and lists exactly these three paths:
`testdata/repositories/v0.9.2/fenced-machine-keys/repo/.docket.local.yml`,
`testdata/repositories/v0.9.2/four-layer-collision/repo/.docket.local.yml`,
`testdata/repositories/v0.9.2/invalid/model-typo/repo/.docket.local.yml`.
After restore, `git status --porcelain testdata/` prints nothing. (The file is committed and unmodified, so `git checkout --` restores exactly the pre-mutation bytes.)

- [ ] **Step 6: Mutation — swap the probe flag, watch the control redden**

In `trackedButIgnored`, temporarily replace `"--exclude-per-directory=.gitignore"` with `"--exclude-standard"`. Confirm the edit landed (`grep -c -- '--exclude-standard' internal/repoguard/tracked_ignored_test.go` reports at least 1 in the call line). Run:

`timeout --kill-after=10s 10m go test -count=1 -run '^TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile$' ./internal/repoguard/`

Expected: FAIL at step `machine-local excludes are not consulted` reporting `keep/other.txt`. Revert the edit by hand (the file is uncommitted, so do NOT use `git checkout --`; re-type the original flag), then re-run Step 4's command and confirm PASS.

- [ ] **Step 7: Run the whole repoguard package**

Run: `timeout --kill-after=10s 10m go test -count=1 ./internal/repoguard/`
Expected: PASS, including `TestRealProcessPackagesUseFixtureTempDir` (the new file uses `testsupport.TempDir` only) and `TestNoExecutableBacktickInSuiteSource`.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-two-unproven-invariants-the-testdata-ignore-negation-a
gofmt -l internal/repoguard/tracked_ignored_test.go
git add internal/repoguard/tracked_ignored_test.go
git commit -m "test(0504): guard that no tracked file is ignored by committed .gitignore rules" \
  -m "<mutation evidence: Step 5 red output listing the three v0.9.2 fixtures, restore confirmed clean; Step 6 red output at the machine-local-excludes step, reverted>"
```

`gofmt -l` must print nothing. Replace the second `-m` placeholder with the actual observed evidence.

---

### Task 2: Guard 2 — restore the root-anchored receipt fixtures

**Files:**
- Modify: `internal/app/repoownership_integration_test.go` (insert the section before the `// --- Unknown mapping (never foreign)` section comment, after `TestIntegrationRepoOwnershipInvalidReceiptsAreForeign`)

**Interfaces:**
- Consumes (all already in the file / package): `newOwnFixture`, `f.emptyTree`, `f.mktree`, `f.commitTree`, `f.mainTip`, `f.verify`, `seedMessage`, `opTrailer`, `srcTrailer`, `copyTrailer`, `repairTrailer`, `proofLegacyEmpty`, `proofInitReceipt`, `proofMigrateReceipt`, `reposetup.OpInitRoot`, `reposetup.OpMigrateSeed`, `reposetup.RootParentless`, `reposetup.RootForeign`.
- Produces: `TestIntegrationRepoOwnershipDescendantInitReceiptCannotAuthorizeLegacyRoot`, `TestIntegrationRepoOwnershipDescendantMigrateReceiptCannotAuthorizeForeignRoot`.

- [ ] **Step 1: Apply only the test hunk of `9c5ced015`**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-two-unproven-invariants-the-testdata-ignore-negation-a
git show 9c5ced015 -- internal/app/repoownership_integration_test.go | git apply
git diff --stat
```

Expected: only `internal/app/repoownership_integration_test.go` changed, 64 insertions. `internal/app/metadata_ownership.go` must be untouched. If `git apply` refuses (helpers moved), re-author the same two tests against the current helpers with the same assertions, copying the bodies from `git show 9c5ced015 -- internal/app/repoownership_integration_test.go`.

The applied hunk is exactly this (reference; do not edit the assertions):

```go
// --- Root-anchored trailer read: a descendant's receipt never authorizes -------
//
// Receipt trailers are read from the ROOT COMMIT ITSELF (verifyMetadataOwnership
// scans `own.Root`, the sole parentless root). These two fixtures place a VALID
// seed receipt on a DESCENDANT of a receiptless root and assert the verdict stays
// driven only by the root — never adopted through the descendant. They redden if
// a refactor ever re-anchored the trailer scan (or its match) to the tip: the
// descendant's receipt would then be read as the root's proof.

// A receiptless empty-tree legacy root with a descendant carrying a valid
// OpInitRoot receipt stays proofLegacyEmpty (the root's own proof), NEVER
// proofInitReceipt adopted from the descendant. Under a scan-anchored-at-tip
// mutation the descendant's OpInitRoot would flip Proof to proofInitReceipt.
func TestIntegrationRepoOwnershipDescendantInitReceiptCannotAuthorizeLegacyRoot(t *testing.T) {
	f := newOwnFixture(t)
	et := f.emptyTree()
	root := f.commitTree(et, nil, "legacy bootstrap seed") // receiptless empty root
	tip := f.commitTree(
		f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"}),
		[]string{root},
		seedMessage("docket init on a descendant", opTrailer(reposetup.OpInitRoot)))

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootParentless {
		t.Fatalf("Shape = %v, want RootParentless from the root's own legacy proof (err=%v)", own.Shape, own.Err)
	}
	if own.Proof != proofLegacyEmpty {
		t.Fatalf("Proof = %d, want proofLegacyEmpty (the root's own proof)", own.Proof)
	}
	if own.Proof == proofInitReceipt || own.Proof == proofMigrateReceipt {
		t.Fatal("a descendant's receipt was adopted as the root's proof: the trailer scan is not root-anchored")
	}
	if own.Root != gitcli.ObjectID(root) {
		t.Errorf("Root = %s, want the parentless root %s", own.Root, root)
	}
}

// A receiptless NONEMPTY root with no historical match is RootForeign; a valid
// OpMigrateSeed receipt on a descendant — even one whose CopyDigest equals the
// ROOT's tree and whose source is reachable — must NOT rescue it. Under a
// scan-anchored-at-tip mutation the descendant's receipt would be read against
// the root's tree and flip the verdict to RootParentless/proofMigrateReceipt.
func TestIntegrationRepoOwnershipDescendantMigrateReceiptCannotAuthorizeForeignRoot(t *testing.T) {
	f := newOwnFixture(t) // main carries only README: no live planning surface, so no historical match
	proj := f.mktree(map[string]string{"docs/changes/active/0001-a.md": "a\n"})
	root := f.commitTree(proj, nil, "receiptless foreign seed") // receiptless nonempty root; rootTree == proj
	tip := f.commitTree(
		f.mktree(map[string]string{"docs/changes/active/0002-b.md": "b\n"}),
		[]string{root},
		seedMessage("docket migrate seed on a descendant",
			opTrailer(reposetup.OpMigrateSeed),
			srcTrailer(f.mainTip()), // reachable from the integration tip
			copyTrailer(proj),       // == the ROOT tree, so a tip-anchored read would validate
			repairTrailer("deadbeef")))

	own := f.verify(t, gitcli.ObjectID(tip), gitcli.ObjectID(f.mainTip()))
	if own.Shape != reposetup.RootForeign {
		t.Fatalf("Shape = %v, want RootForeign for a receiptless root with no historical match (err=%v)", own.Shape, own.Err)
	}
	if own.Proof == proofMigrateReceipt || own.Proof == proofInitReceipt {
		t.Fatal("a descendant's receipt was adopted as the root's proof: the trailer scan is not root-anchored")
	}
}
```

- [ ] **Step 2: Run the two fixtures — they pass against current code**

Run: `timeout --kill-after=10s 10m go test -tags integration -count=1 -run '^TestIntegrationRepoOwnershipDescendant' -v ./internal/app/`
Expected: PASS for both tests (these pin existing correct behavior; there is no failing-first step).

- [ ] **Step 3: Mutation — anchor the trailer scan at the tip, watch both redden**

Copy the production file aside first (it is committed, but keep a byte-exact backup per learning `mutation-restore-needs-a-backup-copy`):

```bash
cd /Users/homer/dev/docket/.worktrees/guard-two-unproven-invariants-the-testdata-ignore-negation-a
cp internal/app/metadata_ownership.go "${TMPDIR:-/tmp}/0504-metadata_ownership.go.bak"
```

In `verifyMetadataOwnership` (`internal/app/metadata_ownership.go`), change `git.ScanCommitTrailers(ctx, repo, own.Root, []string{` to `git.ScanCommitTrailers(ctx, repo, tip, []string{`, and `if s.Commit == own.Root {` to `if s.Commit == tip {`. Confirm both edits landed: `grep -c -e 'ScanCommitTrailers(ctx, repo, tip,' -e 's.Commit == tip' internal/app/metadata_ownership.go` prints `2`.

Run: `timeout --kill-after=10s 10m go test -tags integration -count=1 -run '^TestIntegrationRepoOwnershipDescendant' ./internal/app/`
Expected: FAIL for BOTH tests (the init case reports a proof other than `proofLegacyEmpty`; the migrate case reports a shape other than `RootForeign` or the adopted-receipt fatal).

Restore and verify:

```bash
mv -f "${TMPDIR:-/tmp}/0504-metadata_ownership.go.bak" internal/app/metadata_ownership.go
git status --porcelain internal/app/metadata_ownership.go
```

The status line must print nothing.

- [ ] **Step 4: Run the full ownership shard**

Run: `bash tests/test_go_integration_app_repoownership.sh`
Expected: PASS. Read the output for any `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` line against the shard's 30s ceiling in `tests/runtime-budgets.tsv`; report one if it appears (do not edit the budget in this change).

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-two-unproven-invariants-the-testdata-ignore-negation-a
git status --porcelain
git add internal/app/repoownership_integration_test.go
git commit -m "test(0504): restore root-anchored receipt fixtures from 9c5ced015 (test hunk only)" \
  -m "<mutation evidence: both tests red under the tip-anchored ScanCommitTrailers + s.Commit == tip mutation; metadata_ownership.go restored, status clean>"
```

`git status --porcelain` before staging must show only `internal/app/repoownership_integration_test.go`. Replace the second `-m` placeholder with the actual observed evidence.

---

## Build gate

docket-build runs the single full-suite gate after the last task (`build.test_command`, resolved from config). Both guards must be green there; the non-vacuity control (`TestTrackedButIgnoredProbeDetectsIgnoredTrackedFile`) is part of that run.
