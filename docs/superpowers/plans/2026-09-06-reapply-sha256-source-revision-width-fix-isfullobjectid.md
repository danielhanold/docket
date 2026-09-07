<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0379 — Re-apply the SHA-256 (64-hex) source-revision width fix to isFullObjectID](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0379-reapply-sha256-source-revision-width-fix-isfullobjectid.md)**
<!-- docket:backlink:end -->
# SHA-256 Source-Revision Width Fix for isFullObjectID — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repo the build role is `docket-build`; execute through it.

**Goal:** Make `isFullObjectID` in `internal/app/metadata_ownership.go` accept full 64-character SHA-256 lowercase-hex object IDs in addition to 40-character SHA-1 IDs, so a valid SHA-256 migration receipt no longer produces a false `RootForeign` verdict before the ancestry check can run.

**Architecture:** One-line predicate widening (`len != 40` becomes `len != 40 && len != 64`) plus a comment update, matching the accept set of the downstream `validateObjectID` in `internal/gitcli/types.go` ("non-empty, all-lowercase-hex id of length 40 or 64"). New focused table-driven unit test in package `app` with no build tag. All other ownership checks (copy digest, ancestry, refusal paths) are untouched.

**Tech Stack:** Go. Tests via `go test` (always with `-count=1` — this repo's rule: a cached pass is not evidence). Full gate via the configured `build.test_command` (`go run ./cmd/docket development test` today — re-resolve from config, don't trust this parenthetical).

**Spec:** `docs/superpowers/specs/2026-09-07-reapply-sha256-source-revision-width-fix-isfullobjectid-design.md` (on the `docket` metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree).

## Global Constraints

- Production diff confined to `internal/app/metadata_ownership.go`; tests confined to `internal/app/metadata_ownership_test.go` (create — it does not exist at plan time; if it exists by build time, extend it instead).
- `isFullObjectID(s)` is true iff `len(s)` is exactly 40 or exactly 64 AND every byte is `0-9` or `a-f`. Pure syntax check: no trimming, no case folding, no abbreviation expansion, no ref resolution, no hash-algorithm inference.
- `verifyMetadataOwnership` control flow unchanged. No exported API, receipt format, config field, new package, or shared-validator abstraction.
- Tests assert returned behavior, never the spelling of the production predicate.
- Every `go test` invocation in this plan uses `-count=1` (Go's cache serves stale verdicts to mutation probes).
- Mutation probes: back up with `cp`, restore with `mv` of the backup — never `git checkout --` (it restores to HEAD and destroys the uncommitted fix). Prove each mutation landed via `git diff` before believing its reading.
- `gofmt` every edited Go file before verification.
- Historical reference (do not blindly cherry-pick): commit `41245f279426574e21c48f5c9a5d7cd05233084d` from change 0378 contains the reverted correction. Current source + spec define the contract.

---

### Task 1: Widen isFullObjectID to 40/64 with proven regression coverage

**Files:**
- Create: `internal/app/metadata_ownership_test.go`
- Modify: `internal/app/metadata_ownership.go` (the `isFullObjectID` function and its doc comment — currently reading "reports whether s is a full 40-character lowercase-hex Git object id")
- Test: `internal/app/metadata_ownership_test.go`

**Interfaces:**
- Consumes: `isFullObjectID(s string) bool`, unexported, already defined in `internal/app/metadata_ownership.go` (package `app`).
- Produces: the same signature, widened accept set (40 or 64 lowercase-hex). Sole caller `verifyMetadataOwnership` (the `reposetup.SeedMigrate` arm: `if !isFullObjectID(rec.SourceRevision) || …`) needs no change.

- [ ] **Step 1: Write the failing table-driven test**

Create `internal/app/metadata_ownership_test.go` with exactly this content. Package `app`, **no** `//go:build integration` tag (the existing `repoownership_integration_test.go` has one; this file must not). Fixtures derive from one `strings.Repeat` base so byte lengths are evident and correct; no repository fixture or external Git process is needed.

```go
package app

import (
	"strings"
	"testing"
)

// TestIsFullObjectID pins the untrusted-receipt syntax check: exactly 40 or 64
// lowercase-hex bytes pass; everything else is rejected. Widths match the
// gitcli reader's validateObjectID ("length 40 or 64"). Cases assert observed
// behavior only — never the predicate's spelling.
func TestIsFullObjectID(t *testing.T) {
	t.Parallel()

	// 64 lowercase-hex bytes containing both digits and letters; hex40 is its
	// 40-byte prefix. Lengths are evident by construction: 4×16 and a [:40] slice.
	hex64 := strings.Repeat("0123456789abcdef", 4)
	hex40 := hex64[:40]
	if len(hex64) != 64 || len(hex40) != 40 {
		t.Fatalf("fixture self-check: len(hex64)=%d len(hex40)=%d", len(hex64), len(hex40))
	}
	// 40 bytes of non-ASCII content: 20 two-byte UTF-8 runes. Rejection of this
	// input depends on character validation, not on length.
	nonASCII40 := strings.Repeat("é", 20)
	if len(nonASCII40) != 40 {
		t.Fatalf("fixture self-check: len(nonASCII40)=%d", len(nonASCII40))
	}

	cases := []struct {
		name string
		in   string
		want bool
	}{
		// Valid full widths.
		{"sha1 40 lowercase hex", hex40, true},
		{"sha256 64 lowercase hex", hex64, true},

		// Wrong lengths (all bytes lowercase hex, so only length can reject).
		{"empty", "", false},
		{"abbreviated 7", hex64[:7], false},
		{"length 39", hex64[:39], false},
		{"length 41", hex64[:41], false},
		{"length 50", hex64[:50], false},
		{"length 63", hex64[:63], false},
		{"length 65", hex64 + "a", false},

		// Case violations at both accepted widths.
		{"uppercase 40", strings.ToUpper(hex40), false},
		{"mixed case final byte of 64", hex64[:63] + "F", false},

		// Non-hex bytes at both accepted widths, first and final positions —
		// including the final byte of a 64, so validation that stops after
		// 40 bytes is detected.
		{"non-hex first byte of 40", "g" + hex40[1:], false},
		{"non-hex final byte of 40", hex40[:39] + "g", false},
		{"non-hex first byte of 64", "g" + hex64[1:], false},
		{"non-hex final byte of 64", hex64[:63] + "g", false},

		// Whitespace, control bytes, non-ASCII — each at a nominally accepted
		// byte length, so rejection depends on character validation alone.
		{"embedded space at length 40", hex40[:39] + " ", false},
		{"trailing newline at length 40", hex40[:39] + "\n", false},
		{"control byte at length 64", hex64[:63] + "\x00", false},
		{"non-ascii bytes at length 40", nonASCII40, false},

		// Ref names and revision expressions must never pass.
		{"symbolic ref", "HEAD", false},
		{"revision expression", "HEAD~1", false},
		{"branch ref path", "refs/heads/main", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isFullObjectID(tc.in); got != tc.want {
				t.Errorf("isFullObjectID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test to prove the regression case fails against the original implementation**

Run: `go test -count=1 -run 'TestIsFullObjectID' ./internal/app/`
Expected: FAIL, and the only failing subtest is `sha256_64_lowercase_hex` (`isFullObjectID(…) = false, want true`). Every other case must already pass — if any other case fails, the fixture is wrong; fix the fixture, not the expectation. This run is the recorded proof that the test detects the original width defect (spec acceptance criterion 3).

- [ ] **Step 3: Widen the length predicate and update the doc comment**

In `internal/app/metadata_ownership.go`, replace the current helper:

```go
// isFullObjectID reports whether s is a full 40-character lowercase-hex Git
// object id. A receipt's recorded source revision is untrusted input, so it is
// validated here before it is ever handed to a gitcli reader.
func isFullObjectID(s string) bool {
	if len(s) != 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
```

with:

```go
// isFullObjectID reports whether s is a full lowercase-hex Git object id:
// exactly 40 characters (SHA-1) or exactly 64 (SHA-256), the same full widths
// the gitcli reader's validateObjectID accepts. A receipt's recorded source
// revision is untrusted input, so it is validated here before it is ever
// handed to a gitcli reader. This is a syntax check only: acceptance does not
// imply the object exists, is a commit, or is reachable — later ownership
// checks and Git itself still decide that.
func isFullObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
```

The byte-validation loop and boolean return contract are unchanged. Touch nothing else in the file.

- [ ] **Step 4: Format and run the test to verify it passes**

Run: `gofmt -l -w internal/app/metadata_ownership.go internal/app/metadata_ownership_test.go`
Expected: no output besides possibly the filenames being rewritten; re-run must list nothing.

Run: `go test -count=1 -run 'TestIsFullObjectID' ./internal/app/`
Expected: PASS, all subtests.

- [ ] **Step 5: Mutation probe A — restore the 40-only length restriction**

```bash
cp internal/app/metadata_ownership.go internal/app/metadata_ownership.go.bak
```

Edit `internal/app/metadata_ownership.go`: change `if len(s) != 40 && len(s) != 64 {` back to `if len(s) != 40 {`.

Verify the mutation landed before running anything: `git diff internal/app/metadata_ownership.go` must show that exact line change. A probe whose mutating edit did not land fabricates a green.

Run: `go test -count=1 -run 'TestIsFullObjectID' ./internal/app/`
Expected: FAIL, with `sha256_64_lowercase_hex` among the failures. If it stays green, the guard is decoration — stop and fix the test.

Restore: `mv internal/app/metadata_ownership.go.bak internal/app/metadata_ownership.go` (never `git checkout --`; the fix is uncommitted).

- [ ] **Step 6: Mutation probe B — bypass character validation**

```bash
cp internal/app/metadata_ownership.go internal/app/metadata_ownership.go.bak
```

Edit `internal/app/metadata_ownership.go`: inside `isFullObjectID`, replace the entire `for` loop with `_ = s` (leaving the length check and `return true` intact), so any string of length 40 or 64 passes.

Verify via `git diff internal/app/metadata_ownership.go` that the loop is gone.

Run: `go test -count=1 -run 'TestIsFullObjectID' ./internal/app/`
Expected: FAIL, with the malformed-at-accepted-width cases failing — at minimum `uppercase_40`, `non-hex_final_byte_of_64`, `embedded_space_at_length_40`, and `non-ascii_bytes_at_length_40`. If any of those stay green, its fixture length is wrong; fix the fixture.

Restore: `mv internal/app/metadata_ownership.go.bak internal/app/metadata_ownership.go`

Confirm restoration: `git diff internal/app/metadata_ownership.go` shows only the intended Step 3 change relative to HEAD, and `go test -count=1 -run 'TestIsFullObjectID' ./internal/app/` passes again.

- [ ] **Step 7: Run the package's unit tests and the relevant ownership integration coverage**

Run: `go test -count=1 ./internal/app/`
Expected: PASS (fast — the integration corpus is behind a build tag).

Run: `go test -count=1 -tags integration -run 'TestIntegrationRepoOwnership' -timeout 30m ./internal/app/`
Expected: PASS. This is the existing regression coverage for digest, ancestry, and refusal behavior (`…MigrateReceiptMalformedSource`, `…MigrateReceiptCopyDigestMismatch`, `…MigrateReceiptSourceUnreachable`, and kin) — it proves the widened predicate left the downstream ownership flow intact. It runs real Git processes; if wall clock is an issue in this environment, note it in the build evidence rather than skipping — the full gate covers it too.

- [ ] **Step 8: Commit**

```bash
git add internal/app/metadata_ownership.go internal/app/metadata_ownership_test.go
git commit -m "fix(0379): accept 64-hex SHA-256 source revisions in isFullObjectID

The ownership verifier's syntax check required exactly 40 bytes, so a valid
migration receipt with a SHA-256 source revision was declared RootForeign
before the ancestry check could run. Accept exactly 40 or 64 lowercase-hex
bytes, matching gitcli's validateObjectID; character validation and all
later ownership checks are unchanged. Table-driven unit coverage pins both
widths, the length boundaries, and malformed-at-accepted-width inputs."
```

Stage only these two paths — never `git add -A` (the worktree may be shared).

---

## Build gate (owned by the build role, not a task)

After the task completes, the build role runs the full suite by resolving `build.test_command` from current configuration (`go run ./cmd/docket development test` today) from the source checkout under review. Do not substitute a remembered command; do not run only the tests this plan enumerated. Treat any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach to act on and `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings (`tests/README.md`). No new shell wrapper or budget change is expected for this unit test.

## Self-review notes (spec coverage)

- Required-behavior matrix rows → Step 1 cases: 40-valid, 64-valid, empty/abbreviated/other lengths (39/41/50/63/65), uppercase/mixed at both widths, non-hex at both widths (first and final positions, including byte 64), whitespace/control/non-ASCII at accepted byte lengths, ref/revision expressions. All present.
- Acceptance criterion 2 (comment names both widths + syntax-only guarantee) → Step 3's replacement comment.
- Criterion 3 (uncached regression + mutation evidence) → Steps 2, 5, 6, all `-count=1`.
- Criterion 4 (existing ownership coverage + full suite + gofmt) → Steps 4, 7, and the build gate.
- Criterion 5 (diff confined to validator, comment, focused tests) → Global Constraints; `verifyMetadataOwnership` untouched.
- Out of scope honored: no abstraction, no hash-algorithm plumbing, no 0378 follow-ups (#380 owns the descendant-receipt fixture).
