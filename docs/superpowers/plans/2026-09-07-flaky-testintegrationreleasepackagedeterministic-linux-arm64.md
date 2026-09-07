<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0406 — Flaky TestIntegrationReleasePackageDeterministic — linux_arm64 bundle nondeterminism reddens the suite](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0406-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md)**
<!-- docket:backlink:end -->
# Deterministic release bundles (change 0406) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Root-cause the intermittent linux_arm64 release-tarball checksum mismatch in `TestIntegrationReleasePackageDeterministic`, fix the demonstrated cause at the packaging boundary, and make a future mismatch name its affected artifact and differing layer.

**Architecture:** Four moves in spec order: (1) bounded reproduction on the unmodified base with recorded evidence; (2) a layered archive/bundle comparator (`DiffArchives`/`DiffBundles`) unit-tested on small fixtures; (3) wire that comparator plus artifact preservation into the determinism test's failure path; (4) demonstrate the leading mechanism — the release build embeds ambient git state (`vcs.revision`/`vcs.time`/`vcs.modified`) as an undeclared build input — and, only if the demonstration reddens, close it with `-buildvcs=false` in an extracted `buildTuple`, pinned by a mechanism-based regression test on a hermetic fixture module.

**Tech Stack:** Go 1.26 (`archive/tar`, `compress/gzip`, `debug/buildinfo`), the repo's `integration`-tagged test shard (`tests/test_go_integration_release.sh`, prefix `^TestIntegrationRelease`), git-based hermetic fixtures.

**Spec:** `docs/superpowers/specs/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-design.md` (on the `docket` metadata branch; synchronized copy under `.docket/` at the primary checkout). Change file: `docs/changes/active/0406-flaky-testintegrationreleasepackagedeterministic-linux-arm64.md` (same branch).

## Global Constraints

- Never relax, retry-away, skip, or quarantine `TestIntegrationReleasePackageDeterministic`; the strict `checksums.txt` byte comparison must stay red on any mismatch.
- Every observational `go test` run uses `-count=1` (the result cache serves stale verdicts otherwise — learning `cached-runner-serves-a-mutated-tree`). Integration tests need `-tags integration`.
- **No speculative fix:** the `-buildvcs=false` repair in Task 4 ships only if that task's positive control demonstrates ambient git state reaching the built bytes (Step 5 reddens). If the control cannot demonstrate it, stop at diagnostics and write the non-reproduction report (Task 5's contingency wording).
- A non-reproduction report must be plainly distinguishable from repair evidence — use the exact phrases prescribed in Task 5.
- Preserve archive format (gzip'd USTAR, single `docket` member, mode 0755), bundle naming, installer behavior, and the injected ldflags identity. `VerifyArchive`'s contract is untouched.
- Bounded experiments: run exactly the repetition counts written in each task and record verbatim outcomes; never retry until green.
- The final gate is the whole suite via the resolved `build.test_command` (today `go run ./cmd/docket development test`), run from the feature worktree.
- Commits: one per task, message style `<type>(0406): <summary>` matching repo history.
- All commands below run from the feature worktree root: `/Users/homer/dev/docket/.worktrees/flaky-testintegrationreleasepackagedeterministic-linux-arm64`.

## Background: what is already known (read before Task 1)

- `internal/release/archive.go` pins every tar/gzip metadata field (tar ModTime=epoch, uid/gid 0, empty uname/gname, USTAR; gzip ModTime=epoch, OS=0xFF, empty Name). The archive layer is not assumed to be the leak.
- `internal/release/package.go` builds each tuple with `go build -trimpath -ldflags <fixed identity>` under `CGO_ENABLED=0`, `GOOS/GOARCH`, `GOFLAGS=` — but **not** `-buildvcs=false`. A plan-time probe on this exact configuration (2026-09-06, go1.26.5, HEAD effc9a6d) confirmed the produced linux_arm64 binary embeds:
  `build vcs=git`, `build vcs.revision=effc9a6d…`, `build vcs.time=2026-09-06T13:00:04Z`, `build vcs.modified=false`.
  So the effective build inputs include ambient repository git state. A transient flip of `vcs.modified` (any dirtying/cleaning of the checkout between the two `Package` calls' builds — scratch files, a concurrent process touching the worktree) or a mid-run commit changes the embedded buildinfo blob and therefore the binary bytes. `Tuples()` builds linux_arm64 **last** in each Package call, which is consistent with (not yet proof of) the observed single-tuple mismatch: a transient window covering exactly one of the eight builds.
- Per learning `groomed-root-cause-is-a-hypothesis`, this is the leading enumerated route, not the verdict. Other routes to the same symptom, which the Task 2 diagnostics will classify if they ever occur: (b) build-cache corruption/race under parallel suite load; (c) toolchain/linker nondeterminism; (d) an archive-layer leak despite the pinned writer; (e) manifest-content drift. The plan's fix is gated on a controlled demonstration, and the results file must state precisely which of "mechanism demonstrated" vs "wild event's cause confirmed" was achieved.

---

### Task 1: Bounded reproduction on the unmodified base, with recorded evidence

**Files:**
- Create: `docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-evidence.md`

**Interfaces:**
- Consumes: nothing (runs against the branch as-is; the plan file is the only prior commit).
- Produces: the evidence record later tasks and the results file cite. No code.

- [ ] **Step 1: Record the experiment header.** Create the evidence file with: `git rev-parse HEAD`, `git status --porcelain` (must be clean apart from nothing — run before creating the file, or note the evidence file itself as the only untracked entry), `go version`, date, machine note (darwin/arm64 host), and whether the Go build cache is warm (it is, unless `go clean -cache` was run — do not clean it; record "warm").

- [ ] **Step 2: Direct input probe.** Repeat the plan-time probe and paste its output into the evidence file:

```bash
GOFLAGS= CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /tmp/docket-0406-probe ./cmd/docket
go version -m /tmp/docket-0406-probe | grep -E 'vcs'
rm /tmp/docket-0406-probe
```

Expected: `vcs=git`, `vcs.revision=…`, `vcs.time=…`, `vcs.modified=…` lines present. This establishes ambient git state as an embedded build input in the exact release build configuration (minus ldflags, which do not affect vcs stamping).

- [ ] **Step 3: Isolated reproduction — exactly 5 runs.**

```bash
for i in 1 2 3 4 5; do
  go test -tags integration -count=1 -run '^TestIntegrationReleasePackageDeterministic$' ./internal/release/
done
```

Record each run's PASS/FAIL verbatim. On any FAIL, save the complete failure output into the evidence file before anything else (the current test names no layer; Task 3 fixes that — capture what exists).

- [ ] **Step 4: Suite-load reproduction — exactly 2 whole-suite runs.**

```bash
go run ./cmd/docket development test
```

Run twice; record each run's `SUITE` summary line and, specifically, the `test_go_integration_release.sh` row's outcome and wall clock. On a determinism failure, save the full failing output. Budget-clause lines (`BUDGET WATCH:` / `SERIAL CONFIRMED OVER BUDGET:`) are recorded, not acted on here (they route through the standing rules at the gate).

- [ ] **Step 5: Write the bounded-extent statement.** Close the evidence file's reproduction section with the actual counts run and outcomes, e.g. "5 isolated + 2 suite-load executions; 0 mismatches reproduced" or the mismatch details. Do not add runs beyond the bound.

- [ ] **Step 6: Commit**

```bash
git add docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-evidence.md
git commit -m "docs(0406): reproduction evidence — bounded base-state runs and build-input probe"
```

---

### Task 2: Layered bundle/archive comparator with unit-tested classification

**Files:**
- Create: `internal/release/diff.go`
- Create: `internal/release/diff_test.go` (no build tag — cheap fixtures only)

**Interfaces:**
- Consumes: `WriteArchive(path string, binary []byte, epoch int64) error` (existing, `internal/release/archive.go`) for fixtures.
- Produces: `DiffArchives(pathA, pathB string) (ArchiveDiff, error)`, `DiffBundles(dirA, dirB string, names []string) string`, layer constants `LayerIdentical`, `LayerGzipHeader`, `LayerTarMetadata`, `LayerPayload`. Task 3 consumes these.

- [ ] **Step 1: Write the failing tests** in `internal/release/diff_test.go`. Fixtures are built with `WriteArchive` on small payloads; the gzip-header-only fixture is made by byte-surgery on the gzip MTIME field (offsets 4–7 of the gzip header; header FLG is 0 so no header CRC invalidates the edit):

```go
package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureArchive(t *testing.T, dir, name string, payload []byte, epoch int64) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := WriteArchive(p, payload, epoch); err != nil {
		t.Fatalf("WriteArchive(%s): %v", name, err)
	}
	return p
}

func TestDiffArchivesIdentical(t *testing.T) {
	dir := t.TempDir()
	a := writeFixtureArchive(t, dir, "a.tar.gz", []byte("same-bytes"), 1700000000)
	b := writeFixtureArchive(t, dir, "b.tar.gz", []byte("same-bytes"), 1700000000)
	d, err := DiffArchives(a, b)
	if err != nil {
		t.Fatalf("DiffArchives: %v", err)
	}
	if d.Layer != LayerIdentical {
		t.Fatalf("layer %q, want %q", d.Layer, LayerIdentical)
	}
}

func TestDiffArchivesGzipHeaderOnly(t *testing.T) {
	dir := t.TempDir()
	a := writeFixtureArchive(t, dir, "a.tar.gz", []byte("same-bytes"), 1700000000)
	raw, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	mutated := append([]byte(nil), raw...)
	// gzip MTIME lives at bytes 4..7 (little-endian); FLG (byte 3) is 0 for
	// WriteArchive output, so there is no header CRC to invalidate.
	mutated[4] ^= 0xFF
	b := filepath.Join(dir, "b.tar.gz")
	if err := os.WriteFile(b, mutated, 0o644); err != nil {
		t.Fatalf("write mutated: %v", err)
	}
	d, err := DiffArchives(a, b)
	if err != nil {
		t.Fatalf("DiffArchives: %v", err)
	}
	if d.Layer != LayerGzipHeader {
		t.Fatalf("layer %q, want %q (detail %q)", d.Layer, LayerGzipHeader, d.Detail)
	}
}

func TestDiffArchivesTarMetadata(t *testing.T) {
	dir := t.TempDir()
	a := writeFixtureArchive(t, dir, "a.tar.gz", []byte("same-bytes"), 1700000000)
	b := writeFixtureArchive(t, dir, "b.tar.gz", []byte("same-bytes"), 1700000001)
	d, err := DiffArchives(a, b)
	if err != nil {
		t.Fatalf("DiffArchives: %v", err)
	}
	if d.Layer != LayerTarMetadata {
		t.Fatalf("layer %q, want %q (detail %q)", d.Layer, LayerTarMetadata, d.Detail)
	}
	if !strings.Contains(d.Detail, "ModTime") {
		t.Fatalf("detail %q does not name the differing header field", d.Detail)
	}
}

func TestDiffArchivesPayload(t *testing.T) {
	dir := t.TempDir()
	a := writeFixtureArchive(t, dir, "a.tar.gz", []byte("payload-AAAA"), 1700000000)
	b := writeFixtureArchive(t, dir, "b.tar.gz", []byte("payload-AAAB"), 1700000000)
	d, err := DiffArchives(a, b)
	if err != nil {
		t.Fatalf("DiffArchives: %v", err)
	}
	if d.Layer != LayerPayload {
		t.Fatalf("layer %q, want %q (detail %q)", d.Layer, LayerPayload, d.Detail)
	}
	if !strings.Contains(d.Detail, "first differing offset 11") {
		t.Fatalf("detail %q does not report the first differing payload offset (want 11)", d.Detail)
	}
}

func TestDiffBundlesNamesAffectedFiles(t *testing.T) {
	dirA, dirB := t.TempDir(), t.TempDir()
	writeFixtureArchive(t, dirA, "x.tar.gz", []byte("same"), 1700000000)
	writeFixtureArchive(t, dirB, "x.tar.gz", []byte("same"), 1700000000)
	writeFixtureArchive(t, dirA, "y.tar.gz", []byte("one"), 1700000000)
	writeFixtureArchive(t, dirB, "y.tar.gz", []byte("two"), 1700000000)
	if err := os.WriteFile(filepath.Join(dirA, "install.sh"), []byte("s"), 0o755); err != nil {
		t.Fatalf("write install.sh A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "install.sh"), []byte("s"), 0o755); err != nil {
		t.Fatalf("write install.sh B: %v", err)
	}
	report := DiffBundles(dirA, dirB, []string{"x.tar.gz", "y.tar.gz", "install.sh"})
	if !strings.Contains(report, "y.tar.gz") || !strings.Contains(report, LayerPayload) {
		t.Fatalf("report does not name the differing archive and layer:\n%s", report)
	}
	if !strings.Contains(report, "x.tar.gz: identical") {
		t.Fatalf("report does not mark the matching archive identical:\n%s", report)
	}
}
```

- [ ] **Step 2: Run to verify they fail.**

Run: `go test -count=1 -run 'TestDiff' ./internal/release/`
Expected: compile FAIL — `DiffArchives`, `DiffBundles`, layer constants undefined.

- [ ] **Step 3: Implement `internal/release/diff.go`.** Classification precedence is **deepest differing layer wins**: payload > tar metadata > gzip header.

```go
package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Layer classifications for DiffArchives, ordered shallowest to deepest.
// Precedence on a mismatch is deepest-wins: a payload difference is reported
// as LayerPayload even when shallower layers also differ.
const (
	LayerIdentical   = "identical"
	LayerGzipHeader  = "gzip-header"
	LayerTarMetadata = "tar-metadata"
	LayerPayload     = "member-payload"
)

// ArchiveDiff classifies where two release archives' bytes diverge and
// carries a human-readable account of the difference. It exists for
// failure-time diagnostics; it never relaxes any comparison.
type ArchiveDiff struct {
	Layer  string
	Detail string
}

// archiveParts is one archive decomposed for layer comparison.
type archiveParts struct {
	raw     []byte
	gzHdr   gzip.Header
	tarBody []byte // full decompressed tar stream
	hdr     *tar.Header
	member  []byte
}

func readArchiveParts(path string) (*archiveParts, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: gzip open: %w", path, err)
	}
	defer gz.Close()
	gzHdr := gz.Header
	tarBody, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("%s: decompress: %w", path, err)
	}
	tr := tar.NewReader(bytes.NewReader(tarBody))
	hdr, err := tr.Next()
	if err != nil {
		return nil, fmt.Errorf("%s: read tar member: %w", path, err)
	}
	member, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("%s: read tar body: %w", path, err)
	}
	return &archiveParts{raw: raw, gzHdr: gzHdr, tarBody: tarBody, hdr: hdr, member: member}, nil
}

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func firstDiffOffset(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n // one is a prefix of the other
}

// DiffArchives reads two single-member release archives and reports the
// deepest layer at which they differ. Both must parse; a parse failure is an
// error, not a classification.
func DiffArchives(pathA, pathB string) (ArchiveDiff, error) {
	a, err := readArchiveParts(pathA)
	if err != nil {
		return ArchiveDiff{}, err
	}
	b, err := readArchiveParts(pathB)
	if err != nil {
		return ArchiveDiff{}, err
	}
	if bytes.Equal(a.raw, b.raw) {
		return ArchiveDiff{Layer: LayerIdentical, Detail: "byte-identical"}, nil
	}
	if !bytes.Equal(a.member, b.member) {
		detail := fmt.Sprintf(
			"member %q differs: sizes %d vs %d, sha256 %s vs %s, first differing offset %d\n%s",
			archiveMember, len(a.member), len(b.member),
			sha256hex(a.member), sha256hex(b.member),
			firstDiffOffset(a.member, b.member),
			describeBinaryDiff(a.member, b.member),
		)
		return ArchiveDiff{Layer: LayerPayload, Detail: detail}, nil
	}
	if !bytes.Equal(a.tarBody, b.tarBody) {
		var fields []string
		if !a.hdr.ModTime.Equal(b.hdr.ModTime) {
			fields = append(fields, fmt.Sprintf("ModTime %s vs %s", a.hdr.ModTime.UTC(), b.hdr.ModTime.UTC()))
		}
		if a.hdr.Mode != b.hdr.Mode {
			fields = append(fields, fmt.Sprintf("Mode %#o vs %#o", a.hdr.Mode, b.hdr.Mode))
		}
		if a.hdr.Name != b.hdr.Name {
			fields = append(fields, fmt.Sprintf("Name %q vs %q", a.hdr.Name, b.hdr.Name))
		}
		if a.hdr.Uid != b.hdr.Uid || a.hdr.Gid != b.hdr.Gid {
			fields = append(fields, fmt.Sprintf("Uid/Gid %d/%d vs %d/%d", a.hdr.Uid, a.hdr.Gid, b.hdr.Uid, b.hdr.Gid))
		}
		if a.hdr.Uname != b.hdr.Uname || a.hdr.Gname != b.hdr.Gname {
			fields = append(fields, fmt.Sprintf("Uname/Gname %q/%q vs %q/%q", a.hdr.Uname, a.hdr.Gname, b.hdr.Uname, b.hdr.Gname))
		}
		if len(fields) == 0 {
			fields = append(fields, fmt.Sprintf("tar stream differs outside the parsed header (first differing offset %d)", firstDiffOffset(a.tarBody, b.tarBody)))
		}
		return ArchiveDiff{Layer: LayerTarMetadata, Detail: strings.Join(fields, "; ")}, nil
	}
	detail := fmt.Sprintf(
		"identical tar stream, differing compressed bytes: gzip ModTime %s vs %s, OS %#x vs %#x, Name %q vs %q, sizes %d vs %d, first differing offset %d",
		a.gzHdr.ModTime.UTC(), b.gzHdr.ModTime.UTC(), a.gzHdr.OS, b.gzHdr.OS,
		a.gzHdr.Name, b.gzHdr.Name, len(a.raw), len(b.raw), firstDiffOffset(a.raw, b.raw),
	)
	return ArchiveDiff{Layer: LayerGzipHeader, Detail: detail}, nil
}

// describeBinaryDiff compares the embedded Go build info of two member
// binaries, best-effort: it distinguishes "the declared build inputs changed"
// (differing settings such as vcs.modified) from "same recorded inputs,
// different bytes" (true output nondeterminism). Parse failures are reported
// inline rather than returned — this only ever decorates a failure message.
func describeBinaryDiff(a, b []byte) string {
	ia, errA := buildinfo.Read(bytes.NewReader(a))
	ib, errB := buildinfo.Read(bytes.NewReader(b))
	if errA != nil || errB != nil {
		return fmt.Sprintf("buildinfo: unreadable (A: %v, B: %v)", errA, errB)
	}
	settings := func(bi *buildinfo.BuildInfo) map[string]string {
		m := make(map[string]string, len(bi.Settings))
		for _, s := range bi.Settings {
			m[s.Key] = s.Value
		}
		return m
	}
	sa, sb := settings(ia), settings(ib)
	var lines []string
	for k, va := range sa {
		if vb, ok := sb[k]; !ok || vb != va {
			lines = append(lines, fmt.Sprintf("buildinfo setting %q: %q vs %q", k, va, sb[k]))
		}
	}
	for k, vb := range sb {
		if _, ok := sa[k]; !ok {
			lines = append(lines, fmt.Sprintf("buildinfo setting %q: %q vs %q", k, "", vb))
		}
	}
	if len(lines) == 0 {
		return "buildinfo: identical recorded build settings — bytes differ with the same declared inputs (output nondeterminism)"
	}
	return strings.Join(lines, "\n")
}

// DiffBundles compares each named file across two bundle directories and
// returns a report naming every affected file; differing .tar.gz files are
// classified per layer via DiffArchives. Errors are reported inline — the
// report is failure-time diagnostics, never a gate.
func DiffBundles(dirA, dirB string, names []string) string {
	var sb strings.Builder
	for _, name := range names {
		pa, pb := filepath.Join(dirA, name), filepath.Join(dirB, name)
		ra, errA := os.ReadFile(pa)
		rb, errB := os.ReadFile(pb)
		switch {
		case errA != nil || errB != nil:
			fmt.Fprintf(&sb, "%s: unreadable (A: %v, B: %v)\n", name, errA, errB)
		case bytes.Equal(ra, rb):
			fmt.Fprintf(&sb, "%s: identical\n", name)
		case strings.HasSuffix(name, ".tar.gz"):
			d, err := DiffArchives(pa, pb)
			if err != nil {
				fmt.Fprintf(&sb, "%s: DIFFERS, classification failed: %v\n", name, err)
			} else {
				fmt.Fprintf(&sb, "%s: DIFFERS at layer %s — %s\n", name, d.Layer, d.Detail)
			}
		default:
			fmt.Fprintf(&sb, "%s: DIFFERS (sizes %d vs %d, sha256 %s vs %s, first differing offset %d)\n",
				name, len(ra), len(rb), sha256hex(ra), sha256hex(rb), firstDiffOffset(ra, rb))
		}
	}
	return sb.String()
}
```

Note: `bytes.NewReader` satisfies the `io.ReaderAt` that `buildinfo.Read` requires. `gz.Header` is populated after `gzip.NewReader` returns.

- [ ] **Step 4: Run the tests.**

Run: `go test -count=1 -run 'TestDiff' ./internal/release/`
Expected: PASS (all five).

- [ ] **Step 5: Mutation-test the classifier** (guards are code). Temporarily swap the precedence — move the `tarBody` comparison above the `member` comparison — and run: `TestDiffArchivesPayload` must FAIL (payload diff would misreport as tar-metadata, since a differing member also changes the tar stream). Revert. Then temporarily make `describeBinaryDiff` return `""` and confirm no test reds (it is decoration on `Detail` — acceptable: its consumer is a human reading a Fatalf; the layer classification is the guarded property). Restore.

Run after revert: `go test -count=1 -run 'TestDiff' ./internal/release/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/release/diff.go internal/release/diff_test.go
git commit -m "feat(0406): layered bundle/archive comparator for determinism diagnostics"
```

---

### Task 3: Wire failure-only diagnostics and artifact preservation into the determinism test

**Files:**
- Modify: `internal/release/package_integration_test.go` (function `TestIntegrationReleasePackageDeterministic`)

**Interfaces:**
- Consumes: `DiffBundles(dirA, dirB string, names []string) string` (Task 2), `distributableNames(version string) []string` (existing, same package).
- Produces: on mismatch, a failure message naming affected artifact + differing layer, and a preserved copy of both bundles at a path named in the output. No new exported API.

- [ ] **Step 1: Replace the mismatch branch.** In `TestIntegrationReleasePackageDeterministic`, replace the final `if !bytes.Equal(a, b) { t.Fatalf(...) }` block with a failure path that preserves both bundle directories somewhere `t.TempDir` cleanup cannot reach and prints the layered report. Uses the existing two builds only — no third build (spec: failure-only analysis).

```go
	if !bytes.Equal(a, b) {
		// Preserve the evidence before TempDir cleanup destroys it: copy both
		// bundles to a directory the test framework does not remove, and name
		// it in the failure output. Failure-only work — the passing path does
		// nothing extra.
		keep, kerr := os.MkdirTemp("", "docket-0406-determinism-mismatch-*")
		if kerr == nil {
			for _, cp := range []struct{ src, dst string }{
				{dirA, filepath.Join(keep, "bundleA")},
				{dirB, filepath.Join(keep, "bundleB")},
			} {
				if err := os.CopyFS(cp.dst, os.DirFS(cp.src)); err != nil {
					t.Logf("preserve %s: %v", cp.src, err)
				}
			}
		} else {
			keep = "(preservation failed: " + kerr.Error() + ")"
		}
		report := DiffBundles(dirA, dirB, distributableNames(itVersion))
		t.Fatalf("checksums.txt differ between runs; bundle is not deterministic:\nA:\n%s\nB:\n%s\nlayer report:\n%s\npreserved bundles: %s", a, b, report, keep)
	}
```

(`os.CopyFS` exists since Go 1.23; toolchain is go1.26.)

- [ ] **Step 2: Prove the new failure path actually fires and classifies** (mutation test — never trust an unexercised failure branch). Temporarily insert, just before the `bytes.Equal` check, a probe that corrupts `dirB`'s linux_arm64 archive at the gzip-header layer and re-derives that bundle's manifest so the checksum comparison genuinely mismatches:

```go
	// TEMPORARY MUTATION — do not commit.
	arm := filepath.Join(dirB, ArchiveName(itVersion, Tuple{OS: "linux", Arch: "arm64"}))
	raw, rerr := os.ReadFile(arm)
	if rerr != nil { t.Fatal(rerr) }
	raw[4] ^= 0xFF // flip a gzip MTIME byte: archive DIFFERS at gzip-header layer
	if err := os.WriteFile(arm, raw, 0o644); err != nil { t.Fatal(err) }
	if err := os.Remove(filepath.Join(dirB, "checksums.txt")); err != nil { t.Fatal(err) }
	if err := WriteChecksums(dirB, distributableNames(itVersion)); err != nil { t.Fatal(err) }
	b, err = os.ReadFile(filepath.Join(dirB, "checksums.txt"))
	if err != nil { t.Fatal(err) }
```

(If `WriteChecksums` overwrites an existing manifest cleanly, drop the `os.Remove` line; adjust to the real API when implementing.)

Run: `go test -tags integration -count=1 -run '^TestIntegrationReleasePackageDeterministic$' ./internal/release/`
Expected: FAIL, and the output must (1) name `docket_v0.0.1-planintegration_linux_arm64.tar.gz`, (2) say `layer gzip-header`, (3) print a `preserved bundles: /…/docket-0406-determinism-mismatch-…` path that exists and holds `bundleA/` and `bundleB/`. Delete the preserved dir afterward.

- [ ] **Step 3: Remove the temporary mutation and re-run.**

Run: `go test -tags integration -count=1 -run '^TestIntegrationReleasePackageDeterministic$' ./internal/release/`
Expected: PASS. Diff the test file against the intended change to confirm only the failure-path replacement remains.

- [ ] **Step 4: Commit**

```bash
git add internal/release/package_integration_test.go
git commit -m "test(0406): determinism mismatch now names the artifact, layer, and preserved evidence"
```

---

### Task 4: Demonstrate the ambient-VCS-input mechanism; fix with -buildvcs=false; regression-test it

**Files:**
- Modify: `internal/release/package.go` (extract `buildTuple`; add `-buildvcs=false`)
- Create: `internal/release/build_determinism_integration_test.go` (build tag `integration`; name prefix `TestIntegrationRelease` so the existing shard `tests/test_go_integration_release.sh` picks it up with zero suite-runner changes)

**Interfaces:**
- Consumes: `Tuple` (existing, `internal/release/version.go`).
- Produces: `buildTuple(goBin, sourceRoot, mainPkg, ldflags string, t Tuple, outPath string) error` (unexported; `Package` and the regression test both call it — the test exercises the real production argv/env, not a copy).

- [ ] **Step 1: Pure refactor — extract `buildTuple` without behavior change.** In `package.go`, replace the inline `exec.Command` block inside `Package`'s tuple loop with a call to a new function; the flag set is exactly today's:

```go
// buildTuple cross-compiles mainPkg at sourceRoot for tuple t into outPath
// with the release flag set: -trimpath and -buildvcs=false so the produced
// bytes depend only on the declared inputs (sources, ldflags identity,
// toolchain) and never on ambient repository VCS state; CGO_ENABLED=0 keeps
// the binaries static; GOFLAGS is cleared so an ambient -mod/-tags value
// cannot enter the release bytes. These appended env entries win over any
// ambient copy in os.Environ() because exec honors the last occurrence.
func buildTuple(goBin, sourceRoot, mainPkg, ldflags string, t Tuple, outPath string) error {
	cmd := exec.Command(goBin, "build", "-trimpath", "-ldflags", ldflags, "-o", outPath, mainPkg)
	cmd.Dir = sourceRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+t.OS,
		"GOARCH="+t.Arch,
		"GOFLAGS=",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s/%s: %w\n%s", t.OS, t.Arch, err, out)
	}
	return nil
}
```

(`-buildvcs=false` is **not** added yet — the doc comment above is the final state; write the comment without the buildvcs sentence in this step and add both flag and sentence in Step 5, or write the comment in Step 5 entirely. Keep this step a behavior-preserving extraction.) The loop body becomes:

```go
		if err := buildTuple(goBin, in.SourceRoot, "./cmd/docket", ldflags, t, tmpBin); err != nil {
			return err
		}
```

Run: `go test -count=1 ./internal/release/` then `go test -tags integration -count=1 -run '^TestIntegrationReleasePackageEndToEnd$' ./internal/release/`
Expected: PASS both (pure refactor).

- [ ] **Step 2: Write the failing regression test** in `internal/release/build_determinism_integration_test.go`. It builds a hermetic single-file module inside its own git repo, so no probe ever mutates the real checkout (the suite reads it concurrently — learning `no-checkout-in-shared-worktree` family). The **positive control** (Step 5's gate) proves the fixture actually exercises the vcs mechanism, so the main assert cannot be vacuously green (e.g., a git-less environment where `-buildvcs=auto` silently skips stamping):

```go
//go:build integration

package release

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// gitFixtureModule creates a minimal committed Go module in its own git repo
// and returns its root. The module has a tracked non-Go file (README.md) whose
// edit dirties the tree without changing any compile input.
func gitFixtureModule(t *testing.T) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	files := map[string]string{
		"go.mod":    "module detfixture\n\ngo 1.24\n",
		"main.go":   "package main\n\nfunc main() {}\n",
		"README.md": "clean\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	git := func(args ...string) {
		t.Helper()
		full := append([]string{"-C", dir, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)
		if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")
	return dir
}

// buildDefault runs a plain `go build -trimpath` (buildvcs at its default) —
// the positive control proving the fixture's git state reaches built bytes.
func buildDefault(t *testing.T, dir, out string, tuple Tuple) []byte {
	t.Helper()
	cmd := exec.Command("go", "build", "-trimpath", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+tuple.OS, "GOARCH="+tuple.Arch, "GOFLAGS=")
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("control build: %v\n%s", err, o)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read control binary: %v", err)
	}
	return b
}

// TestIntegrationReleaseBuildIgnoresAmbientGitState pins the demonstrated
// 0406 mechanism: ambient repository VCS state (vcs.modified et al.) is an
// undeclared build input under default -buildvcs, and buildTuple — the real
// production build invocation — must be immune to it. The control asserts the
// mechanism is live in this environment first, so the immunity assert cannot
// pass vacuously.
func TestIntegrationReleaseBuildIgnoresAmbientGitState(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping fixture-module builds in -short mode")
	}
	fixture := gitFixtureModule(t)
	scratch := testsupport.TempDir(t)
	host := Tuple{OS: runtime.GOOS, Arch: runtime.GOARCH}
	readme := filepath.Join(fixture, "README.md")

	// Clean-tree builds.
	if err := buildTuple("go", fixture, ".", "", host, filepath.Join(scratch, "release-clean")); err != nil {
		t.Fatalf("buildTuple clean: %v", err)
	}
	releaseClean, err := os.ReadFile(filepath.Join(scratch, "release-clean"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	controlClean := buildDefault(t, fixture, filepath.Join(scratch, "control-clean"), host)

	// Dirty the tree via a tracked NON-GO file: vcs.modified flips while every
	// compile input stays identical.
	if err := os.WriteFile(readme, []byte("clean\ndirty\n"), 0o644); err != nil {
		t.Fatalf("dirty README: %v", err)
	}

	// Positive control: with default buildvcs the dirty-tree binary MUST
	// differ, or this environment is not exercising the mechanism and the
	// assert below would prove nothing.
	controlDirty := buildDefault(t, fixture, filepath.Join(scratch, "control-dirty"), host)
	if bytes.Equal(controlClean, controlDirty) {
		t.Fatalf("control builds are byte-identical across a tree-state change; fixture does not exercise the vcs-stamping mechanism (git missing or buildvcs inert?)")
	}

	// The pinned property: the release build invocation is immune.
	if err := buildTuple("go", fixture, ".", "", host, filepath.Join(scratch, "release-dirty")); err != nil {
		t.Fatalf("buildTuple dirty: %v", err)
	}
	releaseDirty, err := os.ReadFile(filepath.Join(scratch, "release-dirty"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(releaseClean, releaseDirty) {
		t.Fatalf("buildTuple output depends on ambient git state: clean and dirty-tree builds differ (%d vs %d bytes)", len(releaseClean), len(releaseDirty))
	}
}
```

- [ ] **Step 3: Run it — it must fail for the intended reason.**

Run: `go test -tags integration -count=1 -run '^TestIntegrationReleaseBuildIgnoresAmbientGitState$' ./internal/release/`
Expected: FAIL at the **last** assert (`buildTuple output depends on ambient git state`) — and the control assert before it must NOT be what fails. If instead the **control** fails (clean == dirty under default buildvcs), STOP: the mechanism is not demonstrable in this environment; do not proceed to Step 5, do not ship `-buildvcs=false`; delete Steps 5–6's flag change, keep the extraction and this test marked with the control converted to a `t.Skipf` — no: **remove the test entirely, revert Step 1's extraction if nothing else needs it, and jump to Task 5's non-reproduction contingency.** A repair without a red demonstration is the speculative fix the spec forbids.

- [ ] **Step 4: Record the demonstration in the evidence file.** Append to `docs/superpowers/plans/…-evidence.md`: the failing output of Step 3 (control red-on-equal skipped, immunity assert red), phrased as "mechanism demonstrated on a hermetic fixture: ambient tree-state change alters built bytes under the current release flag set."

- [ ] **Step 5: Apply the fix.** Add `"-buildvcs=false"` to `buildTuple`'s argv (after `"-trimpath"`), and finalize the doc comment as shown in Step 1.

- [ ] **Step 6: Run the regression test and the release shard.**

Run: `go test -tags integration -count=1 -run '^TestIntegrationReleaseBuildIgnoresAmbientGitState$' ./internal/release/`
Expected: PASS.

Run: `go test -tags integration -count=1 -run '^TestIntegrationRelease' ./internal/release/` (all four tuples rebuilt, end-to-end + determinism + collision + checksums)
Expected: PASS — verifies all four tuples still package, `VerifyArchive` passes, host identity check passes (ldflags identity is untouched by `-buildvcs=false`).

- [ ] **Step 7: Mutation-test the repair linkage.** Remove `"-buildvcs=false"` from `buildTuple`, run the Step 6 regression command again with `-count=1`: Expected FAIL at the immunity assert (proves remove-repair → red for the intended reason). Restore the flag, re-run: PASS (restore → green). This is the spec's revert/restore proof — paste both outcomes into the evidence file.

- [ ] **Step 8: Commit**

```bash
git add internal/release/package.go internal/release/build_determinism_integration_test.go docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-evidence.md
git commit -m "fix(0406): release builds pass -buildvcs=false — ambient git state was an undeclared build input"
```

---

### Task 5: Post-repair repetition and the results-wording contract

**Files:**
- Modify: `docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-evidence.md`

**Interfaces:**
- Consumes: everything landed in Tasks 1–4.
- Produces: the final evidence sections the results file and PR body must draw from.

- [ ] **Step 1: Repeat the reproduction conditions after the repair — exactly 5 isolated + 1 suite-load run.**

```bash
for i in 1 2 3 4 5; do
  go test -tags integration -count=1 -run '^TestIntegrationReleasePackageDeterministic$' ./internal/release/
done
go run ./cmd/docket development test
```

Record counts and outcomes in the evidence file (the Task 6 gate run is a second suite-load data point — cite it too). Include verbatim the spec's framing: "finite passing repetitions support the mechanism-based regression; they are not proof by themselves."

- [ ] **Step 2: Write the causal-status section.** The evidence file (and later the results file + PR body) must state exactly one of:
  - **Repair on a demonstrated mechanism:** "The wild 0403-gate event was not re-reproduced within the bounded experiment (N isolated + M suite-load runs). A mechanism producing exactly this symptom class — ambient VCS state as an undeclared build input, embedded in the release binaries and volatile across a Package run — was demonstrated red-to-green on a hermetic fixture and closed with `-buildvcs=false`. Whether that mechanism caused the specific 2026-09-03 event is consistent with the evidence (linux_arm64 is the last-built tuple; a transient tree-state flip covering one of the eight builds reproduces the exact observed pattern) but is not directly confirmed."
  - **Non-reproduction (contingency — only if Task 4 Step 3's control failed):** "Bounded investigation (N isolated + M suite-load runs, plus a controlled fixture probe) neither reproduced the mismatch nor demonstrated a cause. No production fix is included; this change ships diagnostics only (layer-classifying mismatch report + preserved artifacts), so the next occurrence self-describes. The flake is NOT fixed; the unresolved cause returns to the backlog for follow-up." — and in this branch, file the follow-up as an explicit note in the results for the parent workflow to surface.

  Never blend the two vocabularies; "fixed" appears only in the first.

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/plans/2026-09-07-flaky-testintegrationreleasepackagedeterministic-linux-arm64-evidence.md
git commit -m "docs(0406): post-repair repetition evidence and causal-status statement"
```

---

### Task 6: Whole-suite gate

**Files:** none (verification only; fix regressions if red).

- [ ] **Step 1: Resolve the gate command from config** (never a second copy): read `build.test_command` via the repo's config resolution (today it resolves to `go run ./cmd/docket development test`); run it from the feature worktree root.

- [ ] **Step 2: Act on the outcome.** Green: done. Red rows: a red on files this change touched is yours — fix and re-run. A broad unrelated spread (many shell/config rows at a similar factor) is the machine-saturation signature — re-run per `tests/README.md`, never bump budgets. `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines are screening findings; a `SERIAL CONFIRMED OVER BUDGET:` line on `test_go_integration_release.sh` (this plan adds two small fixture-module builds to that shard, seconds warm) must be confirmed serially and acted on before calling the gate green.

- [ ] **Step 3: Record the gate outcome** (SUITE summary line) in the evidence file if anything noteworthy occurred; otherwise the runner's output stands as the record for the results file.

---

## Self-review (performed at plan time)

- Spec §1 (reproduce, bounded, both load conditions, recorded) → Task 1; §2 (layer-distinguishing evidence before temp outputs vanish, buildinfo inspection, no full-binary dumps — first-offset + hashes + settings diff only) → Tasks 2–3; §3 (fix at earliest responsible boundary, minimal, all four tuples verified, identity preserved) → Task 4; §4 (failure-only diagnostics, no extra builds in the permanent test) → Task 3; regression + revert/restore proof + cache-defeat → Task 4 Steps 3/7; post-repair repetition → Task 5; suite gate → Task 6; non-reproduction wording → Task 5 Step 2 and the Global Constraints gate on the fix.
- The one deliberate deviation from a naive reading: the "demonstrated cause" is established by a controlled fixture demonstration (red → green), because the wild event is a transient race unlikely to recur on demand; the causal-status wording in Task 5 keeps that distinction honest instead of overclaiming.
