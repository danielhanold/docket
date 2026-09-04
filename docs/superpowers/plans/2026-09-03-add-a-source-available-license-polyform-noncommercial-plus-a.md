<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0401 — Add a source-available license: PolyForm Noncommercial plus an individual commercial exemption](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-04-0401-add-a-source-available-license-polyform-noncommercial-plus-a.md)**
<!-- docket:backlink:end -->
# Source-Available License (PolyForm Noncommercial + Individual Exemption) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `LICENSE` (PolyForm Noncommercial 1.0.0, verbatim, with Required Notice) and `LICENSE-ADDITIONAL-PERMISSIONS.md` at the repo root, a README `## License` section, and a suite-registered guard over all three.

**Architecture:** Docs-only change plus one guard. The legal wording is fixed by the spec; the builder transcribes, never authors. The guard's assertions live in a Go test in `internal/repoguard` (the Go home for repo-shape guards), driven by a new `# docket-suite: go` wrapper `tests/test_license_files.sh` — the suite's category vocabulary is fail-closed to `go|posix-install|posix-downloader` (`internal/suiterunner/discover.go`), so a standalone grep-only shell suite is inadmissible; a Go test plus wrapper is the convention-fitting shape the spec's "following the suite conventions in tests/README.md" requires.

**Tech Stack:** Markdown, Go (stdlib-only test), Bash wrapper, `tests/runtime-budgets.tsv` registry.

**Spec:** `docs/superpowers/specs/2026-09-03-add-a-source-available-license-polyform-noncommercial-plus-a-design.md` (on the `docket` metadata branch; synchronized copy readable at `.docket/docs/superpowers/specs/...` from the primary tree). The spec is authoritative on ALL legal wording.

## Global Constraints

- **Never improvise legal language.** `LICENSE-ADDITIONAL-PERMISSIONS.md` and the README section are transcribed verbatim from the spec blocks reproduced in this plan (they are copied from the spec byte-for-byte — if they disagree with the spec, the spec wins; stop and report the discrepancy rather than picking one).
- **PolyForm text is fetched, not typed.** The license body must be the unmodified PolyForm Noncommercial 1.0.0 text, canonical at `https://polyformproject.org/licenses/noncommercial/1.0.0/`. Never reconstruct it from memory; never reflow it.
- **Tests must not reach the network** (tests/README.md, Parallel-safety). Only the build-time fetch in Task 1 touches the network; the guard reads local files only.
- **Every `tests/test_*.sh` file needs** a `# docket-suite: <category>` line in its first 10 lines and a row in `tests/runtime-budgets.tsv`, or the run/`repoguard.TestRuntimeBudgetsCorrespondence` fails.
- **`grep` patterns that lead with `-` must be declared** (`grep -qF -- "<pat>"`), and never `producer | grep -q` under pipefail — capture to a variable first (AGENTS.md, Shell).
- **Mutation-test every guard** (AGENTS.md): strip the guarded thing, watch it redden, restore **from a `cp` backup, never `git checkout --`** (the edit is uncommitted), and defeat Go's test cache with `-count=1`.
- **The build gate runs the whole suite**: `go run ./cmd/docket development test` from the feature worktree, keyed on exit status.
- All commands below run from the feature worktree root: `/Users/homer/dev/docket/.worktrees/add-a-source-available-license-polyform-noncommercial-plus-a`.

## File Structure

- Create: `LICENSE` — Required Notice line, pointer line, then the fetched PolyForm text.
- Create: `LICENSE-ADDITIONAL-PERMISSIONS.md` — the spec's exact three-clause text.
- Modify: `README.md` — append the spec's `## License` section after `## Status` (the file's final section; the README has **no** table-of-contents list, so the spec's "matching entry to the table of contents list" has no referent — nothing to add, note it in the results file).
- Create: `internal/repoguard/license_test.go` — `TestLicenseFiles` (Task 1) and `TestLicenseReadmeSection` (Task 2); runs under `go test ./...` (already wired into the suite via `tests/test_go_toolchain.sh`) and under the dedicated wrapper.
- Create: `tests/test_license_files.sh` — `# docket-suite: go` wrapper pinning that both Go tests exist and pass.
- Modify: `tests/runtime-budgets.tsv` — one row for the new wrapper.

---

### Task 1: License files + their guard

**Files:**
- Create: `internal/repoguard/license_test.go`
- Create: `LICENSE`
- Create: `LICENSE-ADDITIONAL-PERMISSIONS.md`

**Interfaces:**
- Produces: `LICENSE` and `LICENSE-ADDITIONAL-PERMISSIONS.md` at repo root; Go test `TestLicenseFiles` in package `repoguard` (Task 2 appends `TestLicenseReadmeSection` to the same file; Task 3's wrapper runs `-run '^TestLicense'`).
- Consumes: `repoguard.Root()` (exported, walks up from CWD to the `go.mod` root — `go test` sets CWD to the package dir, so it resolves correctly).

- [ ] **Step 1: Write the failing test**

Create `internal/repoguard/license_test.go`. `package repoguard` is safe regardless of what sibling test files declare (Go permits `repoguard` and `repoguard_test` files in one directory).

```go
package repoguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readRepoFile reads a repo-root-relative file, failing the test on any error:
// a missing or unreadable license artifact is a red result, never a skip.
func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(b)
}

// TestLicenseFiles guards the license artifacts of change 0401: LICENSE carries
// the PolyForm Noncommercial 1.0.0 identifier, the Required Notice, and the
// pointer to the additional-permissions file; that file carries its three
// clause headings. Fixed-string containment only — no regexp, no network.
func TestLicenseFiles(t *testing.T) {
	license := readRepoFile(t, "LICENSE")
	for _, want := range []string{
		"PolyForm Noncommercial License 1.0.0",
		"Required Notice: Copyright Daniel Hanold",
		"LICENSE-ADDITIONAL-PERMISSIONS.md",
	} {
		if !strings.Contains(license, want) {
			t.Errorf("LICENSE does not contain %q", want)
		}
	}

	perms := readRepoFile(t, "LICENSE-ADDITIONAL-PERMISSIONS.md")
	for _, want := range []string{
		"## 1. Individual commercial exemption",
		"## 2. Scope over the repository history",
		"## 3. Obtaining a commercial license",
	} {
		if !strings.Contains(perms, want) {
			t.Errorf("LICENSE-ADDITIONAL-PERMISSIONS.md does not contain heading %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1 -v`
Expected: FAIL — `reading LICENSE: ... no such file or directory`.

- [ ] **Step 3: Fetch the PolyForm Noncommercial 1.0.0 text, byte-for-byte**

Fetch the license body into a scratch file (build-time network use is fine; the test never fetches):

```bash
curl -fsSL -o "${TMPDIR:-/tmp}/polyform-nc-1.0.0.md" \
  "https://raw.githubusercontent.com/polyformproject/polyform-licenses/1.0.0/PolyForm-Noncommercial-1.0.0.md"
```

If that URL 404s (tag or filename drift), fall back to saving the license text from the canonical page `https://polyformproject.org/licenses/noncommercial/1.0.0/` — the license text only, not the page chrome.

Then verify the fetched body before using it — all four checks must hold, else stop and report BLOCKED rather than shipping a wrong or truncated license:

```bash
f="${TMPDIR:-/tmp}/polyform-nc-1.0.0.md"
grep -qF -- "PolyForm Noncommercial License 1.0.0" "$f" || echo "MISSING identifier"
grep -qF -- "Required Notice:" "$f" || echo "MISSING Required Notice clause"
grep -qiF -- "<html" "$f" && echo "HTML CONTAMINATION"
[ "$(wc -c < "$f")" -gt 3000 ] || echo "SUSPICIOUSLY SHORT"
```

Also open the canonical page once and spot-check that the fetched text's section headings match it (Acceptance / the purpose limitation / notices / disclaimer sections in the same order). Do not edit the body in any way.

- [ ] **Step 4: Assemble `LICENSE`**

Spec order: Required Notice line, pointer line, then the unmodified license text.

```bash
{
  printf 'Required Notice: Copyright Daniel Hanold (https://github.com/danielhanold/docket)\n'
  printf '\n'
  printf 'Additional permissions that widen this license are in LICENSE-ADDITIONAL-PERMISSIONS.md.\n'
  printf '\n'
  cat "${TMPDIR:-/tmp}/polyform-nc-1.0.0.md"
} > LICENSE
```

Then confirm the body survived intact: `diff <(tail -n +5 LICENSE) "${TMPDIR:-/tmp}/polyform-nc-1.0.0.md"` must be empty (the assembled file is exactly 4 lines of preamble plus the fetched body).

- [ ] **Step 5: Write `LICENSE-ADDITIONAL-PERMISSIONS.md`**

Exact content — transcribe verbatim, including line breaks (the only permitted deviation would be an owner-supplied contact address, and none was supplied, so `danny@danielhanold.com` stands):

```markdown
# Additional Permissions to the PolyForm Noncommercial License 1.0.0

These additional permissions are granted by the licensor, Daniel Hanold, on top of
the PolyForm Noncommercial License 1.0.0 in `LICENSE`. They only widen that license.
Where they and the license disagree, whichever grants you more permission applies.
Every other term of the license, including its conditions and its disclaimer of
warranty and liability, applies unchanged.

## 1. Individual commercial exemption

In addition to the noncommercial purposes the license permits, you may use the
software for any commercial purpose if you are an **individual working alone**:
a single natural person, whether acting in your own name or through a sole
proprietorship, single-member company, or similar vehicle that you alone own,
and that has no employees, contractors, or other workers besides you.

This exemption is personal to you. It does not extend to any organization with
more than one worker, and it ends if you begin using the software on behalf of,
or in work performed for, such an organization as its employee, contractor, or
agent, unless that organization holds its own license from the licensor.

Any other commercial use requires the licensor's prior written permission or a
separate license from the licensor.

## 2. Scope over the repository history

The license and these additional permissions apply to every version of the
software in this repository, including every commit, tag, and release made
before the license file was added. No earlier version is licensed on different
terms.

## 3. Obtaining a commercial license

To ask for written permission or a commercial license, contact Daniel Hanold at
danny@danielhanold.com. Written permission is only valid if it comes from the
licensor in writing; a public statement, an issue reply, or a chat message is
not a license unless it says so.
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1 -v`
Expected: PASS (`--- PASS: TestLicenseFiles`).

- [ ] **Step 7: Commit**

```bash
git add LICENSE LICENSE-ADDITIONAL-PERMISSIONS.md internal/repoguard/license_test.go
git commit -m "docs(0401): add PolyForm Noncommercial 1.0.0 LICENSE and additional-permissions notice, with repoguard test"
```

---

### Task 2: README `## License` section + its guard

**Files:**
- Modify: `internal/repoguard/license_test.go` (append one test function)
- Modify: `README.md` (append one section at end of file; it currently ends with the `## Status` section)

**Interfaces:**
- Consumes: `readRepoFile(t, rel)` helper from Task 1 (same file).
- Produces: `TestLicenseReadmeSection` in package `repoguard` (Task 3's wrapper matches it via `-run '^TestLicense'`).

- [ ] **Step 1: Write the failing test**

Append to `internal/repoguard/license_test.go`:

```go
// TestLicenseReadmeSection guards the README's License section: the heading is
// present and the section links to both license files (change 0401). The link
// targets are matched as markdown link destinations, so a rewrite that keeps
// the words but drops the links reddens.
func TestLicenseReadmeSection(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{
		"## License",
		"](LICENSE)",
		"](LICENSE-ADDITIONAL-PERMISSIONS.md)",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not contain %q", want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/repoguard/ -run '^TestLicenseReadmeSection$' -count=1 -v`
Expected: FAIL — all three wants missing.

- [ ] **Step 3: Append the section to `README.md`**

The README has no table-of-contents list, so there is no TOC entry to add (record this in the results file as the spec sentence with no referent). Append after the final `## Status` section, separated by one blank line — exact text:

```markdown
## License

docket is **source-available, not open source**. It is licensed under the
[PolyForm Noncommercial License 1.0.0](LICENSE) with
[additional permissions](LICENSE-ADDITIONAL-PERMISSIONS.md). In short:

- **Personal and noncommercial use is free.** Individuals, charities, schools,
  public research, and government may use docket without asking.
- **Individuals working alone may use it commercially.** A freelancer or solo
  consultant with no employees or contractors is covered by the additional
  permissions.
- **Any other commercial use needs written permission.** Companies and other
  organizations must obtain explicit written permission or a separate license
  from the owner; see the additional-permissions file for how to ask.

The license applies to the whole history of this repository, including every
commit made before the license was added.
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/repoguard/ -run '^TestLicense' -count=1 -v`
Expected: PASS on both `TestLicenseFiles` and `TestLicenseReadmeSection`.

- [ ] **Step 5: Commit**

```bash
git add README.md internal/repoguard/license_test.go
git commit -m "docs(0401): README License section — source-available, three rules, both files linked"
```

---

### Task 3: Suite wrapper, budget row, mutation evidence, whole-suite gate

**Files:**
- Create: `tests/test_license_files.sh`
- Modify: `tests/runtime-budgets.tsv` (one row)

**Interfaces:**
- Consumes: `TestLicenseFiles` and `TestLicenseReadmeSection` from Tasks 1–2.
- Produces: a suite-discovered wrapper; nothing downstream consumes it.

- [ ] **Step 1: Write the wrapper**

Create `tests/test_license_files.sh`, modeled on `tests/test_release_partition_fidelity.sh`. Three house rules are load-bearing here: the `assert()` helper must be the tree's canonical one **byte for byte** (the source-hygiene guard in `internal/repoguard` allowlists it exactly — copy it from `tests/test_release_partition_fidelity.sh`, do not retype); `go test -run` with a pattern matching nothing **exits 0** with "no tests to run", so the wrapper must pin the two `--- PASS:` lines, not the exit code alone (same one-owner reasoning as `tests/test_go_toolchain.sh`); and `--- PASS:` leads with dashes, so every grep for it must be `grep -qF -- "..."` against a captured variable (AGENTS.md: declare leading-dash patterns; never producer-pipe into `grep -q` under pipefail).

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_license_files.sh — the license-artifact guard (change 0401).
#
# Drives internal/repoguard's TestLicenseFiles + TestLicenseReadmeSection:
# LICENSE (PolyForm Noncommercial 1.0.0 identifier, Required Notice, pointer),
# LICENSE-ADDITIONAL-PERMISSIONS.md (its three clause headings), and the
# README License section (heading + links to both files). `go test -run` with
# a pattern that matches nothing exits 0, so this wrapper pins BOTH
# `--- PASS:` lines: deleting or renaming either Go test reddens THIS file
# even though `go test` would happily pass without it.
# CACHES. Same location and reasoning as tests/test_go_toolchain.sh (see the
# CACHES note in that file's header): <git common dir>/docket-go-cache/{mod,build}.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "a Go toolchain is on PATH (the module pins its version)" 'command -v go >/dev/null 2>&1'
if ! command -v go >/dev/null 2>&1; then
  printf 'NOT OK - the license guard cannot certify anything without a Go toolchain\n'
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

test_out="$(go test ./internal/repoguard/ -run '^TestLicense' -v 2>&1)"
test_rc=$?
assert "go test runs the license guards without failure" '[ "$test_rc" -eq 0 ] || { printf "%s\n" "$test_out" >&2; false; }'
assert "TestLicenseFiles exists and passed (a non-matching -run exits 0, so the PASS line is the proof)" \
  'grep -qF -- "--- PASS: TestLicenseFiles" <<<"$test_out"'
assert "TestLicenseReadmeSection exists and passed" \
  'grep -qF -- "--- PASS: TestLicenseReadmeSection" <<<"$test_out"'

if [ "$fail" -eq 0 ]; then printf 'ALL PASS\n'; exit 0; else exit 1; fi
```

Before trusting the file, copy the exact `assert(){ ... }` line and the whole `GOFLAGS`/cache block from `tests/test_release_partition_fidelity.sh` over the versions above (they are transcribed from it, but the allowlist is byte-exact — the source file wins), then run `go test ./internal/repoguard/ -count=1` to clear the source-hygiene and backtick guards over the new suite file.

- [ ] **Step 2: Register the budget row**

Add to `tests/runtime-budgets.tsv`, keeping the table's alphabetical order (between the `test_install_bootstrap.sh` and `test_release_downloader*` rows), tab-separated:

```
tests/test_license_files.sh	15	parallel
```

- [ ] **Step 3: Run the wrapper directly**

Run: `bash tests/test_license_files.sh`
Expected: three `ok - ` lines, `ALL PASS`, exit 0.

- [ ] **Step 4: Mutation-test the whole guard chain**

Back up first — restores come from these copies, never from git (`mutation-restore-needs-a-backup-copy`); use `-count=1` everywhere so Go's cache cannot serve a stale green (`cached-runner-serves-a-mutated-tree`):

```bash
mkdir -p "${TMPDIR:-/tmp}/0401-backup.$$"
cp LICENSE LICENSE-ADDITIONAL-PERMISSIONS.md README.md internal/repoguard/license_test.go tests/test_license_files.sh "${TMPDIR:-/tmp}/0401-backup.$$/"
```

Run each mutation, confirm the mutation actually landed (`diff` against the backup — a no-op substitution fabricates a "guard did not redden" reading), observe the redden, restore from the backup, and confirm green again before the next row:

| # | Mutation | Oracle | Expected |
|---|---|---|---|
| 1 | `rm LICENSE-ADDITIONAL-PERMISSIONS.md` | `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1` | FAIL (readRepoFile fatal) |
| 2 | Delete the `Required Notice:` line from `LICENSE` | same as 1 | FAIL naming the notice |
| 3 | Delete the pointer line (`Additional permissions that widen...`) from `LICENSE` | same as 1 | FAIL naming the filename want |
| 4 | Delete the `## 2. Scope over the repository history` heading line | same as 1 | FAIL naming that heading |
| 5 | Change README's `## License` heading to `## Licensing` | `go test ./internal/repoguard/ -run '^TestLicenseReadmeSection$' -count=1` | FAIL naming `## License` |
| 6 | Delete `(LICENSE-ADDITIONAL-PERMISSIONS.md)` from the README link | same as 5 | FAIL naming the link want |
| 7 | Rename `TestLicenseFiles` to `TestLicenseFilesX` in the Go file | `bash tests/test_license_files.sh` | `NOT OK` on the `TestLicenseFiles` PASS-line assert, exit 1 — proves the wrapper survives `go test`'s exit-0-on-no-match |

After the last restore: `diff` every restored file against its backup copy (must be empty), then remove the backup dir.

- [ ] **Step 5: Run the whole suite at the build gate**

Run: `go run ./cmd/docket development test`
Expected: exit 0, keyed on exit status (many suites end with `ALL PASS`/`ALL OK`, not `PASS`). Read any `BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:` lines — they do not fail the run and nothing else will surface them. This also proves the new wrapper self-registers (discovery glob + budget row) and that the new root files trip no repo-wide guard.

- [ ] **Step 6: Commit**

```bash
git add tests/test_license_files.sh tests/runtime-budgets.tsv
git commit -m "test(0401): suite wrapper for the license guards, with budget row"
```

---

## Self-Review (performed at plan time)

- **Spec coverage:** LICENSE (notice + pointer + verbatim body) → Task 1; additional-permissions exact text → Task 1; README section exact text → Task 2 (TOC entry: no TOC exists — recorded as a spec sentence with no referent, for the results file); test asserting all five spec bullets → Tasks 1–3; mutation-testing and `grep -qF --` → Task 3. Non-goals untouched.
- **Deviation from the spec's letter, argued:** the spec sketches the guard as grep asserts in a shell file. The suite it also binds this file to is fail-closed on category (`go|posix-install|posix-downloader`); a grep-only file has no honest category, so the assertions are Go (fixed-string `strings.Contains`, the spirit of `grep -qF`) and the named `tests/test_license_files.sh` is the `go`-category wrapper pinning them. Every spec assertion is still asserted, in the named file's chain.
- **Type consistency:** `readRepoFile` defined Task 1, consumed Task 2; test names match the wrapper's `-run '^TestLicense'` and its two PASS-line asserts.
- **Legal-review caveat** (spec §Caveat): outside this change; carries to the results file, not to any task.
