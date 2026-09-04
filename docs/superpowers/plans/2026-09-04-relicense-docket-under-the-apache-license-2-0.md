<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0404 — Relicense docket under the Apache License 2.0](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0404-relicense-docket-under-the-apache-license-2-0.md)**
<!-- docket:backlink:end -->
# Apache License 2.0 Relicense Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace docket's PolyForm Noncommercial license (change 0401) with the verbatim Apache License 2.0 — `LICENSE` swapped, `NOTICE` and `CONTRIBUTING.md` added, `LICENSE-ADDITIONAL-PERMISSIONS.md` deleted, the README `## License` section rewritten — and retarget the existing license guard at the new artifacts.

**Architecture:** Docs-only change plus a guard retarget. The legal wording is fixed by the spec; the builder transcribes, never authors. The guard stays where 0401 put it: `internal/repoguard/license_test.go` (`TestLicenseFiles` + `TestLicenseReadmeSection`), driven by the existing `# docket-suite: go` wrapper `tests/test_license_files.sh`, whose mechanics are unchanged — only its header comment is rewritten. No new suite file, so `tests/runtime-budgets.tsv` is untouched.

**Tech Stack:** Markdown, Go (stdlib-only test), Bash wrapper (edited comment only).

**Spec:** `docs/superpowers/specs/2026-09-04-relicense-docket-under-the-apache-license-2-0-design.md` (on the `docket` metadata branch; synchronized copy readable at `.docket/docs/superpowers/specs/...` from the primary tree). The spec is authoritative on ALL wording.

## Global Constraints

- **Never improvise legal or policy language.** `NOTICE`, `CONTRIBUTING.md`, and the README section are transcribed verbatim from the spec blocks reproduced in this plan (byte-for-byte copies of the spec's blocks — if they disagree with the spec, the spec wins; stop and report the discrepancy rather than picking one).
- **The Apache text is fetched, not typed.** `LICENSE` must be the complete, unmodified Apache License 2.0 text from the canonical source `https://www.apache.org/licenses/LICENSE-2.0.txt`, appendix included. Never reconstruct it from memory; never reflow it. A clean `diff` against the fetched file gates the commit.
- **Tests must not reach the network** (tests/README.md, Parallel-safety). Only the build-time fetch in Task 1 touches the network; the guard reads local files only.
- **Any edited Go file must be `gofmt`-clean before the suite runs** — 0401's results record a review fix that had to be reverted for exactly this. Run `gofmt -l internal/repoguard/` after every edit; empty output required.
- **Mutation-test every guard** (AGENTS.md): break the guarded thing, watch it redden, restore **from a `cp` backup, never `git checkout --`** (the edits are uncommitted — checkout restores to HEAD and destroys the work under test). Defeat Go's test cache with `-count=1` on every probe, and prove each mutation landed (`diff` against the backup, or `/usr/bin/grep -cF` — PATH grep is ugrep and misreads some patterns) before believing any reading.
- **The build gate runs the whole suite**: `go run ./cmd/docket development test` from the feature worktree, keyed on exit status.
- All commands below run from the feature worktree root: `/Users/homer/dev/docket/.worktrees/relicense-docket-under-the-apache-license-2-0`.

## File Structure

- Replace content: `LICENSE` — the fetched Apache License 2.0 text, byte-for-byte, nothing else (no preamble, no Required Notice, no pointer line).
- Create: `NOTICE` — product name + copyright line + license pointer (spec's exact 4-line block).
- Delete: `LICENSE-ADDITIONAL-PERMISSIONS.md` (`git rm`).
- Create: `CONTRIBUTING.md` — DCO adoption + workflow pointer (spec's exact block).
- Modify: `README.md` — replace the whole `## License` section (it is the file's final section, lines 123–end today) with the spec's exact block.
- Modify: `internal/repoguard/license_test.go` — retarget `TestLicenseFiles` (Task 1) and `TestLicenseReadmeSection` (Task 2) at the new artifacts; `readRepoFile` helper unchanged.
- Modify: `tests/test_license_files.sh` — header comment paragraph only; every executable line stays byte-identical.

---

### Task 1: Apache LICENSE, NOTICE, CONTRIBUTING.md, deletion — and the retargeted `TestLicenseFiles`

**Files:**
- Modify: `internal/repoguard/license_test.go` (rewrite `TestLicenseFiles` and its doc comment; add `errors` and `io/fs` imports; leave `readRepoFile` and `TestLicenseReadmeSection` untouched)
- Replace: `LICENSE`
- Create: `NOTICE`
- Create: `CONTRIBUTING.md`
- Delete: `LICENSE-ADDITIONAL-PERMISSIONS.md`

**Interfaces:**
- Consumes: `repoguard.Root()` (exported, walks up from CWD to the `go.mod` root) and the existing `readRepoFile(t, rel)` helper in the same file.
- Produces: the four root artifacts and the retargeted `TestLicenseFiles`. Task 2 edits `TestLicenseReadmeSection` in the same file; Task 3's wrapper matches both via the existing `-run '^TestLicense'`.

- [ ] **Step 1: Rewrite the failing test**

In `internal/repoguard/license_test.go`, extend the import block to:

```go
import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

Leave `readRepoFile` exactly as it is. Replace `TestLicenseFiles` and its doc comment (from the line `// TestLicenseFiles guards the license artifacts of change 0401: LICENSE carries` through that function's closing brace) with:

```go
// TestLicenseFiles guards the license artifacts of change 0404: LICENSE is the
// verbatim Apache License 2.0 (identifier, date line, and the appendix's
// distinctive boilerplate clause, which pins the complete text) and carries no
// trace of the retired PolyForm license; NOTICE carries the copyright line
// Apache section 4(d) obliges redistributors to preserve; the retired
// additional-permissions file is absent; CONTRIBUTING.md adopts the DCO
// trailer. Fixed-string containment only — no regexp, no network.
func TestLicenseFiles(t *testing.T) {
	license := readRepoFile(t, "LICENSE")
	for _, want := range []string{
		"Apache License",
		"Version 2.0, January 2004",
		`Licensed under the Apache License, Version 2.0 (the "License");`,
	} {
		if !strings.Contains(license, want) {
			t.Errorf("LICENSE does not contain %q", want)
		}
	}
	for _, banned := range []string{
		"PolyForm",
		"LICENSE-ADDITIONAL-PERMISSIONS.md",
	} {
		if strings.Contains(license, banned) {
			t.Errorf("LICENSE still contains %q — the PolyForm-era content must be gone", banned)
		}
	}

	notice := readRepoFile(t, "NOTICE")
	if want := "Copyright 2026 Daniel Hanold"; !strings.Contains(notice, want) {
		t.Errorf("NOTICE does not contain %q", want)
	}

	root, err := Root()
	if err != nil {
		t.Fatalf("resolving repo root: %v", err)
	}
	_, err = os.Stat(filepath.Join(root, "LICENSE-ADDITIONAL-PERMISSIONS.md"))
	switch {
	case err == nil:
		t.Errorf("LICENSE-ADDITIONAL-PERMISSIONS.md still exists at the repo root; change 0404 deletes it")
	case !errors.Is(err, fs.ErrNotExist):
		t.Fatalf("stat LICENSE-ADDITIONAL-PERMISSIONS.md: %v", err)
	}

	contributing := readRepoFile(t, "CONTRIBUTING.md")
	if want := "Signed-off-by"; !strings.Contains(contributing, want) {
		t.Errorf("CONTRIBUTING.md does not contain %q", want)
	}
}
```

Notes: the distinctive clause is a Go raw-string literal (backquotes) because it embeds double quotes; the absence check distinguishes "cleanly absent" from "stat errored" — any error other than not-exist is a test failure, never a pass (a probe that errors and a probe that reports clean absence are different answers).

Then run `gofmt -l internal/repoguard/` — output must be empty.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1 -v`
Expected: FAIL — the PolyForm-era `LICENSE` misses all three Apache wants and hits both banned strings, `NOTICE` and `CONTRIBUTING.md` are unreadable, and the additional-permissions file still exists. This proves every leg of the new test can go red.

- [ ] **Step 3: Fetch the Apache License 2.0 text, byte-for-byte**

```bash
curl -fsSL -o "${TMPDIR:-/tmp}/apache-2.0.txt" "https://www.apache.org/licenses/LICENSE-2.0.txt"
```

Verify the fetched body before using it — all checks must hold, else stop and report BLOCKED rather than shipping a wrong or truncated license (capture-then-grep; never pipe a producer into `grep -q` under pipefail):

```bash
f="${TMPDIR:-/tmp}/apache-2.0.txt"
grep -qF -- "Apache License" "$f" || echo "MISSING identifier"
grep -qF -- "Version 2.0, January 2004" "$f" || echo "MISSING date line"
grep -qF -- 'Licensed under the Apache License, Version 2.0 (the "License");' "$f" || echo "MISSING appendix clause"
grep -qF -- "APPENDIX: How to apply the Apache License to your work" "$f" || echo "MISSING appendix heading"
grep -qiF -- "<html" "$f" && echo "HTML CONTAMINATION"
[ "$(wc -c < "$f")" -gt 10000 ] || echo "SUSPICIOUSLY SHORT"
```

Expected: no diagnostic lines (the canonical file is ~11,358 bytes). Do not edit the body in any way.

- [ ] **Step 4: Install `LICENSE` and confirm it is the fetched file exactly**

```bash
cp "${TMPDIR:-/tmp}/apache-2.0.txt" LICENSE
diff LICENSE "${TMPDIR:-/tmp}/apache-2.0.txt"
```

The `diff` must be empty — `LICENSE` is the canonical text and nothing else: no preamble, no `Required Notice:` line, no pointer to any other file.

- [ ] **Step 5: Write `NOTICE`**

Exact content (spec block, transcribe verbatim — three lines of substance plus the blank line):

```
docket
Copyright 2026 Daniel Hanold

This product is licensed under the Apache License, Version 2.0; see LICENSE.
```

- [ ] **Step 6: Delete the additional-permissions file**

```bash
git rm LICENSE-ADDITIONAL-PERMISSIONS.md
```

Every clause in it either widened the noncommercial license (moot under Apache) or described how to obtain a commercial license (no longer offered).

- [ ] **Step 7: Write `CONTRIBUTING.md`**

Exact content (spec block, transcribe verbatim including line breaks):

```markdown
# Contributing to docket

docket is licensed under the [Apache License 2.0](LICENSE). Contributions are
accepted under the same license.

## Developer Certificate of Origin

Every commit must carry a `Signed-off-by:` trailer certifying the
[Developer Certificate of Origin](https://developercertificate.org/): that you
wrote the change or otherwise have the right to submit it under the Apache
License 2.0. `git commit -s` adds the trailer. Pull requests whose commits lack
it are not merged.

## How work flows here

docket develops itself with its own workflow. Read `docs/changes/README.md` on
the `docket` branch and `tests/README.md` before starting: every change goes
through a change document, a build gate, and the full test suite, and the
rules in [CLAUDE.md](CLAUDE.md) apply to every edit. Bug reports and ideas are
welcome as issues; the maintainer captures them as changes deliberately.
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1 -v`
Expected: PASS (`--- PASS: TestLicenseFiles`).

- [ ] **Step 9: Commit**

```bash
git add LICENSE NOTICE CONTRIBUTING.md internal/repoguard/license_test.go
git commit -m "docs(0404): relicense under Apache 2.0 — LICENSE swapped, NOTICE + CONTRIBUTING.md added, additional-permissions file deleted, guard retargeted"
```

(`git rm` in Step 6 already staged the deletion; this commit carries it.)

---

### Task 2: README `## License` rewrite + retargeted `TestLicenseReadmeSection`

**Files:**
- Modify: `internal/repoguard/license_test.go` (edit `TestLicenseReadmeSection`'s want list and doc comment only)
- Modify: `README.md` (replace the `## License` section — the file's final section)

**Interfaces:**
- Consumes: `readRepoFile(t, rel)` helper (unchanged, same file).
- Produces: the rewritten README section and the retargeted `TestLicenseReadmeSection` (Task 3's wrapper matches it via `-run '^TestLicense'`).

- [ ] **Step 1: Edit the test to fail against the current README**

In `internal/repoguard/license_test.go`, replace `TestLicenseReadmeSection`'s doc comment and want list. The function becomes:

```go
// TestLicenseReadmeSection guards the README's License section: the heading is
// present and the section links to LICENSE, NOTICE, and CONTRIBUTING.md
// (change 0404). The link targets are matched as markdown link destinations,
// so a rewrite that keeps the words but drops the links reddens.
func TestLicenseReadmeSection(t *testing.T) {
	readme := readRepoFile(t, "README.md")
	for _, want := range []string{
		"## License",
		"](LICENSE)",
		"](NOTICE)",
		"](CONTRIBUTING.md)",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("README.md does not contain %q", want)
		}
	}
}
```

The 0401 assertion `"](LICENSE-ADDITIONAL-PERMISSIONS.md)"` is dropped (the file no longer exists; its absence is asserted in `TestLicenseFiles`). Run `gofmt -l internal/repoguard/` — empty output required.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/repoguard/ -run '^TestLicenseReadmeSection$' -count=1 -v`
Expected: FAIL — the current section has no `](NOTICE)` and no `](CONTRIBUTING.md)` link.

- [ ] **Step 3: Replace the README section**

First confirm the section is still the file's last (nothing after it to preserve): the only `^## ` heading at or after the `## License` line must be `## License` itself. Then delete from the `## License` heading line through end of file and append this exact text in its place (spec block, verbatim):

```markdown
## License

docket is open source under the [Apache License 2.0](LICENSE). The
[NOTICE](NOTICE) file carries the attribution that section 4(d) of the license
asks redistributors to preserve.

- **Files docket generates in your repository are yours.** Change documents,
  specs, plans, results, decision records, and configuration belong to you; the
  license places no condition on them.
- **The license grants no rights in the docket name.** Use the software freely;
  do not present a modified version as docket itself.
- **Contributions** are accepted under the Developer Certificate of Origin; see
  [CONTRIBUTING.md](CONTRIBUTING.md).

The license applies to the whole history of this repository, including every
commit made before it was added.
```

If a later heading has appeared below `## License` (a concurrent change moved it), stop and report BLOCKED with what you found rather than guessing at a splice.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/repoguard/ -run '^TestLicense' -count=1 -v`
Expected: PASS on both `TestLicenseFiles` and `TestLicenseReadmeSection` (the readme edit must not have broken Task 1's asserts).

- [ ] **Step 5: Commit**

```bash
git add README.md internal/repoguard/license_test.go
git commit -m "docs(0404): README License section — Apache-2.0, NOTICE, generated-files/name/DCO points"
```

---

### Task 3: Wrapper header rewrite, mutation evidence, whole-suite gate

**Files:**
- Modify: `tests/test_license_files.sh` (header comment paragraph only)

**Interfaces:**
- Consumes: `TestLicenseFiles` and `TestLicenseReadmeSection` from Tasks 1–2.
- Produces: nothing downstream; the wrapper is suite-discovered and its `tests/runtime-budgets.tsv` row already exists (unchanged — no new suite file, no budget edit).

- [ ] **Step 1: Rewrite the wrapper's header comment**

In `tests/test_license_files.sh`, replace only the comment block from the line `# tests/test_license_files.sh — the license-artifact guard (change 0401).` through the line ending `# even though `go test` would happily pass without it.` with:

```bash
# tests/test_license_files.sh — the license-artifact guard (change 0404,
# superseding 0401's PolyForm guard).
#
# Drives internal/repoguard's TestLicenseFiles + TestLicenseReadmeSection:
# LICENSE (verbatim Apache License 2.0 — identifier, date line, and the
# appendix's distinctive clause — with the PolyForm-era strings asserted
# absent), NOTICE (the copyright line), CONTRIBUTING.md (the Signed-off-by
# DCO trailer), the asserted ABSENCE of LICENSE-ADDITIONAL-PERMISSIONS.md at
# the repo root, and the README License section (heading + links to LICENSE,
# NOTICE, and CONTRIBUTING.md). `go test -run` with
# a pattern that matches nothing exits 0, so this wrapper pins BOTH
# `--- PASS:` lines: deleting or renaming either Go test reddens THIS file
# even though `go test` would happily pass without it.
```

Everything else in the file — the `# docket-suite: go` line, the CACHES note, `set -uo pipefail`, the canonical `assert()` line, the cache block, the `go test` invocation, and both `--- PASS: ... (` pins with their trailing `" ("` — stays byte-identical. Verify with `git diff tests/test_license_files.sh`: only comment lines may appear in the hunk.

- [ ] **Step 2: Run the wrapper directly**

Run: `bash tests/test_license_files.sh`
Expected: `ok - ` lines for the toolchain and all three test asserts, `ALL PASS`, exit 0.

- [ ] **Step 3: Mutation-test the whole guard chain**

Back up first — restores come from these copies, never from `git checkout --` (the Task 1–2 commits protect most files, but the wrapper edit may be uncommitted and the discipline is unconditional); `-count=1` on every Go probe so the cache cannot serve a stale verdict:

```bash
bk="${TMPDIR:-/tmp}/0404-backup.$$"; mkdir -p "$bk"
cp LICENSE NOTICE CONTRIBUTING.md README.md internal/repoguard/license_test.go tests/test_license_files.sh "$bk/"
```

For each row: apply the mutation, **prove it landed** (`diff` the file against `$bk` — a no-op edit fabricates a "guard is vacuous" green), run the oracle, observe the expected red, restore from `$bk`, and re-run the oracle green before the next row.

| # | Mutation | Oracle | Expected |
|---|---|---|---|
| 1 | Append the single word `PolyForm` as a new last line of `LICENSE` (mutates the retired defect back in) | `go test ./internal/repoguard/ -run '^TestLicenseFiles$' -count=1` | FAIL naming `"PolyForm"` — proves the negative assert detects the removed state, not merely the new wording |
| 2 | `mv NOTICE "$bk/NOTICE.moved"` (restore: move back) | same as 1 | FAIL (`readRepoFile` fatal on NOTICE) |
| 3 | `touch LICENSE-ADDITIONAL-PERMISSIONS.md` (restore: `rm`) | same as 1 | FAIL on "still exists at the repo root" |
| 4 | Delete the `Signed-off-by:` line from `CONTRIBUTING.md` | same as 1 | FAIL naming `"Signed-off-by"` |
| 5 | Change README's `## License` heading to `## Licensing` | `go test ./internal/repoguard/ -run '^TestLicenseReadmeSection$' -count=1` | FAIL naming `"## License"` |
| 6 | Rewrite README's `[NOTICE](NOTICE)` to `NOTICE` (drop the link form, keep the word) | same as 5 | FAIL naming `"](NOTICE)"` — proves the link-destination pin, not a word match |
| 7 | Rename `TestLicenseFiles` to `TestLicenseFilesX` in the Go file | `bash tests/test_license_files.sh` | `NOT OK` on the `TestLicenseFiles (` PASS-line assert, exit 1 — proves the wrapper survives `go test`'s exit-0-on-no-match |

After the last restore: `diff` every restored file against its `$bk` copy (all must be empty), `go test ./internal/repoguard/ -run '^TestLicense' -count=1` green, then `rm -rf "$bk"`.

- [ ] **Step 4: gofmt + whole suite at the build gate**

Run: `gofmt -l internal/repoguard/` — empty output required.
Run: `go run ./cmd/docket development test`
Expected: exit 0, keyed on exit status. Read any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:` lines — they do not fail the run and nothing else will surface them. This gate also proves no repo-wide guard (link checkers, source hygiene, README structure guards) trips on the new root files or the deleted one.

- [ ] **Step 5: Commit**

```bash
git add tests/test_license_files.sh
git commit -m "test(0404): retarget the license-guard wrapper's header at the Apache artifacts"
```

---

## Self-Review (performed at plan time)

- **Spec coverage:** LICENSE verbatim swap (fetch + clean diff) → Task 1 Steps 3–4; NOTICE exact block → Task 1 Step 5; additional-permissions deletion → Task 1 Step 6; CONTRIBUTING.md exact block → Task 1 Step 7; README section exact block → Task 2 Step 3; every `## Test` bullet of the spec → the retargeted asserts in Tasks 1–2 (identifier, date line, distinctive clause, both PolyForm-era negatives, NOTICE copyright, absence via `os.Stat` distinguishing not-exist from other errors, `Signed-off-by`, README heading + three link destinations, `](LICENSE-ADDITIONAL-PERMISSIONS.md)` assertion dropped); wrapper header rewrite with both PASS pins kept → Task 3 Step 1; the spec's two named mutations (rename NOTICE; reinsert `PolyForm`) → Task 3 rows 2 and 1, extended to seven rows; gofmt discipline → Global Constraints + Task 3 Step 4. Non-goals untouched: no CLA, no SPDX headers, no trademark doc, no behavior change, no ADR.
- **Placeholder scan:** none — every file's content or exact edit is in its task.
- **Type consistency:** `readRepoFile(t, rel)` unchanged and consumed by both tests; test names match the wrapper's `-run '^TestLicense'` and its two `--- PASS: ... (`-pinned asserts; imports added in Task 1 (`errors`, `io/fs`) cover the only new calls.
- **Intermediate states are green:** Task 1 flips test + artifacts in one commit (a split would leave the tree red between them); Task 2 likewise pairs its test edit with the README edit; the wrapper needs no mechanical change, so Task 3 cannot redden it.
- **Not-a-lawyer caveat** (spec §Caveat): outside this change; carries to the results file, not to any task.
