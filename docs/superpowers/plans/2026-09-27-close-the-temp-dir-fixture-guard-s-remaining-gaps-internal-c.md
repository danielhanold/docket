<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0462 — Close the temp-dir fixture guard's remaining gaps (internal/cli gateTempDir, scan-root removal)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-27-0462-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c.md)**
<!-- docket:backlink:end -->
# Close the temp-dir fixture guard's remaining gaps — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: execute with the build role this repo resolves
> (`docket-build`), task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close change 0462's two residual gaps in `TestRealProcessPackagesUseFixtureTempDir`. The guard
does not see `<ident>.MkdirTemp(` calls, so `internal/cli`'s private `gateTempDir` gets past it, and it
walks a hand-listed `scanRoots`.

**Architecture:** Three self-contained edits, each green on its own:
(1) delete `internal/cli`'s `gateTempDir` helper and use `testsupport.TempDir(t)` at every call site;
(2) add a second violation shape to the guard, `<ident>.MkdirTemp(` on the comment- and string-masked view.
A call is exempt only when a `//` comment on the same line or the line directly above says
`tempdir-exempt: <reason>` with a non-empty reason. Then mark the legitimate sites the new guard reports;
(3) replace `scanRoots` with the shared `MaintainedFiles` whole-repo walker and keep `realProcFloors` as
the non-vacuity floor.

**Tech Stack:** Go (`go/scanner`, `regexp`, `testing`); the repo's `internal/repoguard` and
`internal/testsupport` packages.

**Spec:** `docs/superpowers/specs/2026-09-27-close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c-design.md`
(on the `docket` metadata branch). Read it alongside this plan.

## Global Constraints

- The fixture package `internal/testsupport` stays exempt by construction. Do not change `testsupport.TempDir`'s behavior and do not add fixture modes (spec, Out of scope).
- The MkdirTemp regex keys on the receiver-call **shape** (`\b([A-Za-z_][A-Za-z0-9_]*)\.MkdirTemp\(`), never on the `os` spelling. It runs over `maskProse(b, true)`.
- The marker is `// tempdir-exempt: <reason>`, read from the **raw** bytes, on the call's own line or the line immediately above. An empty or whitespace-only reason does **not** exempt.
- **Re-derive the exempt-site list by the guard's own red run plus a whole-repo grep during the build.** Never copy the spec's table blindly. Each marker's reason is a short sentence about that specific site, not a generic class label.
- Delete `scanRoots` outright. The population comes from `repoguard.MaintainedFiles(root)` filtered to `*_test.go`. Keep `realProcFloors = {"internal/process", "cmd/docket"}`.
- Out of scope: `os.CreateTemp`, calls through interface or function values, name-shadowing helpers, packages that spawn no real process, and the `cmd/` sites 0398 already converted.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never on line numbers (ADR-0054).
- Mutation probes run with `go test -count=1` (a cached PASS against a mutated tree proves nothing). Commit before mutating, restore with `git checkout -- <file>` (HEAD then holds the work), and confirm `git status --porcelain` is empty after each restore.
- Full-suite gate: `build.test_command` resolved from `.docket.yml` (currently `go run ./cmd/docket development test`), run from this worktree.

## Review Focus

1. **A trailing same-line marker** (`d, err := os.MkdirTemp("", "x") // tempdir-exempt: …`) should exempt that call. Pinned by the `same-line marker` case in Task 2's `TestMkdirTempViolations`.
2. **Marker text that is not a `//` comment** should never exempt. That covers marker text inside a string literal on the line above and a `/* tempdir-exempt: x */` block. A careless raw-byte grep would accept both. Pinned by the `marker in string` and `block comment marker` cases in Task 2.
3. **Two consecutive `MkdirTemp` calls under one marker.** Only the first call is adjacent, so the second must be a violation. Pinned by the `consecutive calls` case in Task 2.
4. **Build-tagged test files** such as `internal/app/finalize_e2e_test.go` (`//go:build e2e`) must still be scanned. The walk is by file, not by build, so their exempt sites need markers too. Pinned by Task 2 Step 5: the red run on the real tree must list the `finalize_e2e_test.go` sites, and Step 6 marks them.
5. **The converted `internal/cli` gate tests must still tear down cleanly** during the supervisor's exit window. That race is the one `gateTempDir`'s 40×50ms retry existed for, and the fixture's drain-then-retry removal must cover it. Pinned by Task 1 Step 4, which runs the package with `-count=2` and expects no `directory not empty`.

Note, not a task: when the suite runs from the **primary** checkout, `MaintainedFiles` also walks `.docket/`
(the metadata worktree), and its `docs/codex/fixtures/**/*_test.go` files are in-population. None of them
contains `exec.Command` today, so none joins the real-process set. The shared exclusion rules are out of
scope. Leave this alone, and mention it in the results file only if it ever reddens.

---

### Task 1: Replace `internal/cli`'s `gateTempDir` with the fixture

**Files:**
- Modify: `internal/cli/gate_test.go`. Delete the `gateTempDir` doc comment and function, which sit directly after `TestMain`. Rewrite every `gateTempDir(t)` occurrence (22 lines at groom time).

**Interfaces:**
- Consumes: `testsupport.TempDir(t testing.TB) string` (already imported in this file).
- Produces: nothing new. After this task, `internal/cli/gate_test.go` contains no `MkdirTemp` call, which Task 2's guard relies on.

- [ ] **Step 1: Confirm the starting shape**

Run: `grep -n 'gateTempDir\|MkdirTemp' internal/cli/gate_test.go`
Expected: one `func gateTempDir` definition, one `os.MkdirTemp("", "docket-gate-*")`, and the call sites.

- [ ] **Step 2: Delete the helper**

Remove this whole block, from its doc comment through the closing brace:

```go
// gateTempDir is a temp dir whose cleanup tolerates the external supervisor's
// brief exit window. Observe reports "passed" the instant the terminal record
// lands, which can precede the supervisor's final same-directory atomic write
// and lock release, so a single-shot RemoveAll (as t.TempDir does) races it and
// fails "directory not empty". The retry loop lets the supervisor finish.
func gateTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "docket-gate-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 40; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = os.RemoveAll(dir)
	})
	return dir
}
```

- [ ] **Step 3: Convert every call site and verify imports**

Run: `sed -i '' 's/gateTempDir(t)/testsupport.TempDir(t)/g' internal/cli/gate_test.go`
Then: `grep -c 'gateTempDir' internal/cli/gate_test.go`. Expect `0`, which exits 1, so capture it rather than piping.
Keep the `os` import (`TestMain` uses `os.Exit`) and the `time` import (the observe poll loop still calls
`time.Sleep`). Run `go vet ./internal/cli/` and expect no unused-import error. If an import did become
unused, drop it.

- [ ] **Step 4: Run the package and confirm clean teardown (Review Focus 5)**

Run: `go test -count=2 ./internal/cli/`
Expected: PASS, and no `directory not empty` or `testsupport:` surviving-path failure in the output.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/gate_test.go
git commit -m "test(cli): gate tests use testsupport.TempDir; delete private gateTempDir (change 0462)"
```

---

### Task 2: Ban unmarked `MkdirTemp` in real-process packages; mark the legitimate sites

**Files:**
- Modify: `internal/repoguard/tempdir_fixture_test.go`: the header LIMITATION note, new regexes, the new helpers `exemptMarkerLines` and `mkdirTempViolations`, the new unit test `TestMkdirTempViolations`, and the wiring in `TestRealProcessPackagesUseFixtureTempDir`.
- Modify: each legitimate `MkdirTemp` site reported in Step 5 (a one-line `//` marker above the call). The groom-time candidates, which you must re-derive rather than trust, are `cmd/docket/main_test.go`, `cmd/releasepkg/main_test.go`, `internal/app/finalize_e2e_test.go` (two sites), `internal/app/status_git_test.go`, `internal/app/gate_drive_test.go`, `internal/gatedrive/driver_test.go`, `internal/release/package_integration_test.go`, and `internal/install/references_test.go`.

**Interfaces:**
- Consumes: the existing `maskProse(src []byte, maskStrings bool) []byte` in the same file.
- Produces (used by Task 4's rewrite of the walk; keep the names exact):
  - `var mkdirTempCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.MkdirTemp\(`)`
  - `var tempdirExemptRe = regexp.MustCompile(`^//[ \t]*tempdir-exempt:[ \t]*\S`)`
  - `func exemptMarkerLines(src []byte) map[int]bool`: the 1-based lines of justified `//` marker comments.
  - `func mkdirTempViolations(src []byte) (violations, exempt []int)`: the 1-based lines of unmarked and marked executable calls.

- [ ] **Step 1: Write the failing unit test**

Append to `internal/repoguard/tempdir_fixture_test.go`. Every fixture is a Go string literal. This package
is itself a real-process package, and its own guard masks string literals, so these fixtures never count
as violations or markers in this file.

```go
func TestMkdirTempViolations(t *testing.T) {
	cases := []struct {
		name          string
		src           string
		wantViolation []int
		wantExempt    []int
	}{
		{"bare call", "package p\nfunc f() {\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{3}, nil},
		{"marker above", "package p\nfunc f() {\n\t// tempdir-exempt: built once in TestMain, no t\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", nil, []int{4}},
		{"same-line marker", "package p\nfunc f() {\n\td, _ := os.MkdirTemp(\"\", \"x\") // tempdir-exempt: process-lifetime dir\n\t_ = d\n}\n", nil, []int{3}},
		{"marker two lines above", "package p\nfunc f() {\n\t// tempdir-exempt: too far away\n\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{5}, nil},
		{"empty reason", "package p\nfunc f() {\n\t// tempdir-exempt:\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"whitespace reason", "package p\nfunc f() {\n\t// tempdir-exempt:   \t\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"spelling in comment and string", "package p\n// os.MkdirTemp(\"\", \"x\")\nvar s = \"os.MkdirTemp(\"\n", nil, nil},
		{"marker in string", "package p\nfunc f() {\n\t_ = \"// tempdir-exempt: not a comment\"\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"block comment marker", "package p\nfunc f() {\n\t/* tempdir-exempt: block comments do not count */\n\td, _ := os.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{4}, nil},
		{"consecutive calls", "package p\nfunc f() {\n\t// tempdir-exempt: first only\n\ta, _ := os.MkdirTemp(\"\", \"a\")\n\tb, _ := os.MkdirTemp(\"\", \"b\")\n\t_, _ = a, b\n}\n", []int{5}, []int{4}},
		{"non-os receiver", "package p\nfunc f() {\n\td, _ := afs.MkdirTemp(\"\", \"x\")\n\t_ = d\n}\n", []int{3}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotV, gotE := mkdirTempViolations([]byte(c.src))
			if !slices.Equal(gotV, c.wantViolation) || !slices.Equal(gotE, c.wantExempt) {
				t.Fatalf("violations=%v exempt=%v, want violations=%v exempt=%v", gotV, gotE, c.wantViolation, c.wantExempt)
			}
		})
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

Run: `go test -count=1 -run TestMkdirTempViolations ./internal/repoguard/`
Expected: a build failure, `undefined: mkdirTempViolations`.

- [ ] **Step 3: Implement the helpers**

Add `"bytes"` to the import block. Add the following after `tempDirCallRe`/`testsupportAliasRe` and after
`maskProse`, respectively:

```go
var mkdirTempCallRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.MkdirTemp\(`)

// tempdirExemptRe matches the text of a justified exemption marker: a line
// comment reading "tempdir-exempt:" followed by a non-blank reason.
var tempdirExemptRe = regexp.MustCompile(`^//[ \t]*tempdir-exempt:[ \t]*\S`)
```

```go
// exemptMarkerLines returns the 1-based lines that carry a justified
// tempdir-exempt marker. It lexes the RAW source with go/scanner (the masked
// view blanks comments), so only a real // comment token counts: marker text
// inside a string literal or a /* */ block never exempts.
func exemptMarkerLines(src []byte) map[int]bool {
	lines := map[int]bool{}
	fset := token.NewFileSet()
	f := fset.AddFile("", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(f, src, nil, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT && tempdirExemptRe.MatchString(lit) {
			lines[f.Line(pos)] = true
		}
	}
	return lines
}

// mkdirTempViolations finds every executable <ident>.MkdirTemp( call in src
// (comments and string/char literals masked) and splits them by line into
// violations and exempt calls. A call is exempt only when its own line or the
// line immediately above carries a justified marker (exemptMarkerLines).
func mkdirTempViolations(src []byte) (violations, exempt []int) {
	markers := exemptMarkerLines(src)
	callView := maskProse(src, true)
	for _, loc := range mkdirTempCallRe.FindAllIndex(callView, -1) {
		// maskProse blanks in place and preserves newlines, so offsets in the
		// masked view map to the same line numbers as the raw source.
		line := 1 + bytes.Count(callView[:loc[0]], []byte("\n"))
		if markers[line] || markers[line-1] {
			exempt = append(exempt, line)
		} else {
			violations = append(violations, line)
		}
	}
	return violations, exempt
}
```

- [ ] **Step 4: Run the unit test and watch it pass**

Run: `go test -count=1 -run TestMkdirTempViolations ./internal/repoguard/`
Expected: PASS, all 11 subtests.

- [ ] **Step 5: Wire the check into the guard and let the red run derive the site list**

In `TestRealProcessPackagesUseFixtureTempDir`, inside the per-file loop, directly after the
`tempDirCallRe` loop, add:

```go
			// Change 0462: an executable MkdirTemp call is a hand-rolled temp dir
			// that skips the fixture's drain-then-retry cleanup, unless the site
			// needs a lifetime the per-test fixture cannot provide and says so.
			bad, ok := mkdirTempViolations(b)
			for _, line := range bad {
				violations = append(violations, fmt.Sprintf("%s:%d: MkdirTemp call without a justified tempdir-exempt marker — use testsupport.TempDir(t); only a dir the per-test fixture cannot serve (no t in TestMain, process lifetime under sync.Once, failure evidence that must survive, a mandated parent) may carry an adjacent \"// tempdir-exempt: <reason>\"", f, line))
			}
			for _, line := range ok {
				t.Logf("exempt MkdirTemp: %s:%d", f, line)
			}
```

Run: `go test -count=1 -run TestRealProcessPackagesUseFixtureTempDir ./internal/repoguard/`
Expected: FAIL, with one `MkdirTemp call without a justified tempdir-exempt marker` line for each current
site. **This output is the authoritative site list.** Cross-check it against a whole-repo grep (no pipe
into an early-exiting consumer):

```bash
grep -rnE --include='*_test.go' --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=testdata -e '\b[A-Za-z_][A-Za-z0-9_]*\.MkdirTemp\(' .
```

Every grep hit in a real-process package, other than `internal/testsupport`, must show up in the guard's
list. A hit the guard misses is a guard defect: stop and investigate. It must not become a residual. Any
hit in `internal/cli/gate_test.go` means Task 1 is incomplete. Confirm that
`internal/app/finalize_e2e_test.go` (build tag `e2e`) is in the list (Review Focus 4).

- [ ] **Step 6: Mark or convert each reported site**

For each site in the Step 5 list, first decide whether it is a per-test dir deleted at test end. If so,
convert it to `testsupport.TempDir(t)` instead of marking it. Otherwise add a `//` marker line directly
above the call, indented to match. If a comment block already sits above the call, the marker becomes that
block's last line. Suggested reasons for the groom-time candidates follow. Adapt each one to what the code
actually does there:

| Site (function) | Marker |
|---|---|
| `cmd/docket/main_test.go` `TestMain` | `// tempdir-exempt: TestMain builds the docket binary once for the whole package; there is no t to own a fixture dir.` |
| `cmd/releasepkg/main_test.go` `TestMain` | `// tempdir-exempt: TestMain builds the releasepkg binary once for the whole package; there is no t to own a fixture dir.` |
| `internal/app/finalize_e2e_test.go` `e2eNode` | `// tempdir-exempt: shared XDG config dir created once under e2eXDGOnce and reused by every e2e test in the process.` |
| `internal/app/finalize_e2e_test.go` `sharedBinaries` | `// tempdir-exempt: docket and gh binaries built once under sharedBinOnce and shared for the process lifetime.` |
| `internal/app/status_git_test.go` `backgroundOffGitEnv` | `// tempdir-exempt: background-off gitconfig written once under bgOffGitOnce and shared for the process lifetime.` |
| `internal/gatedrive/driver_test.go` `sampleWorktree` | `// tempdir-exempt: sample worktree built once under sampleWorktreeOnce and shared read-only across tests.` |
| `internal/release/package_integration_test.go` determinism mismatch | `// tempdir-exempt: failure evidence — the mismatched bundles must survive the test so the failure output can name them.` |
| `internal/install/references_test.go` `TestDeriveVersionReferencesCanonicalizesTmpAliases` | `// tempdir-exempt: must live under the literal /tmp to exercise the macOS /tmp -> /private/tmp alias.` |
| `internal/app/gate_drive_test.go` `runRootFixture` | `// tempdir-exempt: nested inside a testsupport.TempDir(t) parent, whose fixture cleanup removes it.` |

No marker line may begin a comment with `tempdir-exempt:` unless it sits adjacent to the call it justifies.

Run: `go test -count=1 -run 'TestRealProcessPackagesUseFixtureTempDir|TestMkdirTempViolations' -v ./internal/repoguard/`
Expected: PASS, with one `exempt MkdirTemp:` log line per marked site.

- [ ] **Step 7: Update the header LIMITATION note**

Replace the existing `// LIMITATION (asserted, per the byte-pattern-guard learning): …` paragraph in the
file header with:

```go
// LIMITATION (asserted, per the byte-pattern-guard learning): the ban
// matches the receiver-call shapes `<ident>.TempDir(` and, since change
// 0462, `<ident>.MkdirTemp(`. It cannot see a call through an interface
// value, a function value, or a helper that shadows the name, and
// os.CreateTemp (temp files) is out of scope. The aliased-import check below
// closes the one cheap evasion (import testsupport under another name and
// the receiver test goes vacuous).
//
// MkdirTemp EXEMPTIONS: a few real-process sites need a lifetime the
// per-test fixture deliberately does not provide — a binary built in
// TestMain (no t), a process-lifetime dir created under sync.Once, failure
// evidence that must survive, a mandated /tmp parent. Such a call is exempt
// only when a line comment on its own line or the line immediately above
// carries the tempdir-exempt marker with a non-empty reason (see
// tempdirExemptRe). The marker is lexed from the RAW source, so marker text
// inside a string literal or a block comment never exempts.
```

Check that no line of this header begins its comment text with `tempdir-exempt:`, because such a line
would itself be a marker.

- [ ] **Step 8: Run the package and commit**

Run: `go test -count=1 ./internal/repoguard/`
Expected: PASS.

```bash
git add internal/repoguard/tempdir_fixture_test.go <each marked or converted site file>
git commit -m "test(repoguard): ban unmarked MkdirTemp in real-process packages; mark justified sites (change 0462)"
```

Stage the files by explicit path. Never use `git add -A`.

- [ ] **Step 9: Mutation-verify the ban (spec mutations 1–5 and 7)**

Every probe runs `go test -count=1 -run TestRealProcessPackagesUseFixtureTempDir ./internal/repoguard/`.
Restore each with `git checkout -- <file>` (or `rm` for a new file) and confirm `git status --porcelain`
is empty before the next probe.

1. Restore the `gateTempDir` helper from Step 2 of Task 1 into `internal/cli/gate_test.go`. Expected: **FAIL**, naming `internal/cli/gate_test.go` and `MkdirTemp call without a justified tempdir-exempt marker`.
2. Delete one site's marker line, for example in `cmd/docket/main_test.go`. Expected: **FAIL**, naming that file and line.
3. Change one marker to `// tempdir-exempt:` with no reason. Expected: **FAIL**, naming that site.
4. Create `internal/process/zz_mutation_test.go` with `package process`, `import ("os"; "testing")`, and `func zzHelper(t *testing.T) string { d, _ := os.MkdirTemp("", "zz-*"); return d }`. Use the package clause the directory's other test files use. Expected: **FAIL**, naming the new file. Then `rm` it.
5. Insert one unrelated line (for example `_ = 0`) between a marker and its call. Expected: **FAIL**, because adjacency is enforced.
7. In any real-process test file, add `// os.MkdirTemp("", "x")` and `var _ = "os.MkdirTemp("` at package level. Expected: **PASS**, because masking hides both.

Record each probe's observed result for the results file. A probe that fails to redden is a finding about
the guard. Investigate it, and do not write it down as a residual.

---

### Task 3: Derive the scan population from the whole repo

**Files:**
- Modify: `internal/repoguard/tempdir_fixture_test.go`: delete `scanRoots`, rewrite the walk in `TestRealProcessPackagesUseFixtureTempDir`, and rewrite the header SCOPE note and the `realProcFloors` doc.

**Interfaces:**
- Consumes: `MaintainedFiles(root string) ([]string, error)` (`internal/repoguard/repoguard.go`), which returns root-relative, slash-separated, sorted paths with the categorical exclusions already pruned. Also consumes Task 2's `mkdirTempViolations`.
- Produces: no new symbols. Violation messages now print module-relative slash paths.

- [ ] **Step 1: Rewrite the walk**

Delete the `scanRoots` doc comment and `var scanRoots = …`. Replace the start of
`TestRealProcessPackagesUseFixtureTempDir`, from the `pkgs := …` line through the floor loop, with:

```go
	files, err := MaintainedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	// Module-relative slash dir -> its module-relative slash _test.go files.
	pkgs := map[string][]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			dir := path.Dir(f)
			pkgs[dir] = append(pkgs[dir], f)
		}
	}
	const fixtureDir = "internal/testsupport"
	read := func(rel string) []byte {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	var realProc []string
	for dir, fs := range pkgs {
		if dir == fixtureDir {
			continue // the fixture itself is exempt by construction
		}
		for _, f := range fs {
			if execCallRe.Match(read(f)) {
				realProc = append(realProc, dir)
				break
			}
		}
	}
	sort.Strings(realProc)
	// Population floors (marker-scoped guards need one): the whole-repo
	// derivation must find each floor package — internal/process, whose
	// supervisor tests motivated the fixture, and cmd/docket, whose built-binary
	// gate tests spawn the real supervisor. A missing floor means the
	// exec.Command shape rotted or a shared MaintainedFiles exclusion swallowed
	// a test root, and the guard would otherwise pass vacuously over it.
	for _, floor := range realProcFloors {
		if !slices.Contains(realProc, floor) {
			t.Fatalf("derivation lost %s — real-process set: %v", floor, realProc)
		}
	}
```

In the violations loop, replace the `os.ReadFile(f)` block with `b := read(f)`. Keep the alias, TempDir,
and MkdirTemp checks unchanged. Update the imports: add `"path"`, and drop `"io/fs"` because nothing uses
it now. Note that the loop variable `fs` shadows nothing once `io/fs` is gone. If you prefer, name it
`pkgFiles`. Run `go vet ./internal/repoguard/`.

- [ ] **Step 2: Rewrite the SCOPE note and the `realProcFloors` doc**

Replace the header's `// SCOPE (explicit — see scanRoots and realProcFloors below): …` paragraph with:

```go
// SCOPE (see realProcFloors below): since change 0462 the scan population is
// derived from the WHOLE repository through the shared MaintainedFiles
// walker, filtered to _test.go files — there is no guard-local list of roots
// left to shrink. Narrowing coverage now means editing the categorical
// exclusions in repoguard.go that every repoguard guard depends on.
// realProcFloors keeps a population floor in each known test root
// (internal/ from change 0373, cmd/ from change 0398), so a rotted
// exec.Command shape or an exclusion that swallows a root fails loudly
// instead of passing vacuously.
```

Replace the `realProcFloors` doc comment with:

```go
// realProcFloors are module-relative (slash-separated) packages the
// whole-repo derivation must always find. An empty or rotted derivation, or
// a shared exclusion that swallows internal/ or cmd/, then fails loudly
// instead of passing vacuously (marker-scoped guards need a population
// floor).
```

- [ ] **Step 3: Run the package**

Run: `go test -count=1 -v -run 'TestRealProcessPackagesUseFixtureTempDir|TestMkdirTempViolations' ./internal/repoguard/`
Expected: PASS. The `exempt MkdirTemp:` log lines now print module-relative paths, and there are exactly as
many as in Task 2 Step 6. A changed count means the population changed: investigate it.
Then: `go test -count=1 ./internal/repoguard/`. Expected: PASS.

- [ ] **Step 4: Confirm there is no guard-local root list left**

Run: `out=$(grep -rn 'scanRoots' --include='*.go' . || true); echo "[$out]"`
Expected: `[]`.

- [ ] **Step 5: Commit**

```bash
git add internal/repoguard/tempdir_fixture_test.go
git commit -m "test(repoguard): derive the temp-dir guard's population from the whole repo (change 0462)"
```

- [ ] **Step 6: Mutation-verify coverage (spec mutation 6)**

In `internal/repoguard/repoguard.go` `isExcludedDir`, temporarily add `"cmd"` to the exact-location
`case` list (`case "docs", "tests/fixtures", "internal/install/legacydata", "cmd":`).
Run: `go test -count=1 -run TestRealProcessPackagesUseFixtureTempDir ./internal/repoguard/`
Expected: **FAIL** with `derivation lost cmd/docket`. Other repoguard tests may redden too, which is
expected. Restore it with `git checkout -- internal/repoguard/repoguard.go` and confirm
`git status --porcelain` is empty. Repeat the probe with `"internal"` and expect
`derivation lost internal/process`. Restore again.

---

### Task 4: Full-suite gate

No code changes. `docket-build` runs this gate once at the end.

- [ ] **Step 1:** Resolve `build.test_command` from `.docket.yml` and run it from this worktree (currently `go run ./cmd/docket development test`).
Expected: the `SUITE …` summary is green. Read the budget report even when the run is green. Act on any `SERIAL CONFIRMED OVER BUDGET:` line, and record any `BUDGET WATCH:` or `PARALLEL-SENSITIVE:` lines.
- [ ] **Step 2:** Confirm `git status --porcelain` is empty and that `git log` shows the three task commits on `chore/close-the-temp-dir-fixture-guard-s-remaining-gaps-internal-c`.

---

## Self-review (plan author)

- Spec coverage: §1 is covered by Task 1. §2 is covered by Task 2: the shape regex, the masked view, the raw-lexed marker, the non-empty reason, the fixture exemption carried over from the existing `fixtureDir` skip, the message, the LIMITATION note, and the re-derived site list. §3 is covered by Task 3: `scanRoots` deleted, `MaintainedFiles`, floors kept, SCOPE and floor docs rewritten. Verification mutations 1–5 and 7 are in Task 2 Step 9, mutation 6 is in Task 3 Step 6, and the full suite is Task 4.
- Names are consistent across tasks: `mkdirTempCallRe`, `tempdirExemptRe`, `exemptMarkerLines`, `mkdirTempViolations`, `realProcFloors`, `maskProse`, `MaintainedFiles`.
- Every task leaves the tree green. Task 1 removes the only unmarked call the Task 2 guard would otherwise flag in `internal/cli`, and Task 2 adds markers in the same commit as the ban.
