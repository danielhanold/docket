package release

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// sha256HexBytes is the independent oracle for the manifest's hash column.
func sha256HexBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// mkFile writes name (with content) into dir and returns its lowercase-hex
// SHA-256, so tests can build hand-crafted manifests that either agree or
// disagree with the on-disk bytes.
func mkFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return sha256HexBytes(content)
}

// writeManifest writes lines (joined with \n, trailing newline added) to
// dir/checksums.txt, the file ValidateChecksums reads.
func writeManifest(t *testing.T, dir string, lines []string) {
	t.Helper()
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, checksumsFile), []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// readManifestLines reads dir/checksums.txt and returns its non-empty lines.
func readManifestLines(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, checksumsFile))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	trimmed := strings.TrimSuffix(string(raw), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// --- WriteChecksums ---------------------------------------------------------

// Feeding names in a deliberately non-sorted order must yield a manifest whose
// filename column is sorted bytewise — the golden ordering property.
func TestWriteChecksumsSortsBytewise(t *testing.T) {
	dir := testsupport.TempDir(t)
	// Names chosen so lexical (bytewise) order differs from feed order.
	names := []string{
		"docket_v1.2.3_linux_arm64.tar.gz",
		"install.sh",
		"docket_v1.2.3_darwin_amd64.tar.gz",
		"docket_v1.2.3_linux_amd64.tar.gz",
		"docket_v1.2.3_darwin_arm64.tar.gz",
	}
	for _, n := range names {
		mkFile(t, dir, n, []byte("body-of-"+n))
	}

	if err := WriteChecksums(dir, names); err != nil {
		t.Fatalf("WriteChecksums: %v", err)
	}

	lines := readManifestLines(t, dir)
	if len(lines) != len(names) {
		t.Fatalf("manifest has %d lines, want %d", len(lines), len(names))
	}

	var gotNames []string
	for _, ln := range lines {
		// Column split on the two-space separator.
		parts := strings.SplitN(ln, "  ", 2)
		if len(parts) != 2 {
			t.Fatalf("line %q not in <hash>  <name> form", ln)
		}
		gotNames = append(gotNames, parts[1])
	}

	want := append([]string(nil), names...)
	// Bytewise-sorted expectation, independent of Go's sort implementation
	// under test: compare against a locally sorted copy.
	sortStringsBytewise(want)
	for i := range want {
		if gotNames[i] != want[i] {
			t.Fatalf("manifest not bytewise-sorted:\n got %v\nwant %v", gotNames, want)
		}
	}
	// The feed order was NOT already sorted, so this assertion is non-vacuous.
	if equalStringSlices(names, want) {
		t.Fatalf("test bug: feed order was already sorted, ordering assertion is vacuous")
	}
}

// The separator is exactly two spaces and the hash is the real lowercase-hex
// SHA-256 of the file — the sha256sum -c compatible format.
func TestWriteChecksumsTwoSpaceFormat(t *testing.T) {
	dir := testsupport.TempDir(t)
	content := []byte("deterministic-payload")
	want := mkFile(t, dir, "install.sh", content)

	if err := WriteChecksums(dir, []string{"install.sh"}); err != nil {
		t.Fatalf("WriteChecksums: %v", err)
	}

	lines := readManifestLines(t, dir)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %d: %v", len(lines), lines)
	}
	wantLine := want + "  install.sh"
	if lines[0] != wantLine {
		t.Fatalf("line = %q, want %q (two spaces, lowercase hex)", lines[0], wantLine)
	}
	if len(want) != 64 {
		t.Fatalf("hash column %q is not 64 hex chars", want)
	}
}

// A name with no backing file must fail — WriteChecksums hashes real bytes.
func TestWriteChecksumsMissingFileFails(t *testing.T) {
	dir := testsupport.TempDir(t)
	mkFile(t, dir, "install.sh", []byte("x"))
	err := WriteChecksums(dir, []string{"install.sh", "absent.tar.gz"})
	if err == nil {
		t.Fatal("WriteChecksums accepted a missing file, want error")
	}
	if !strings.Contains(err.Error(), "absent.tar.gz") {
		t.Fatalf("error %q does not name the missing file", err)
	}
}

// --- ValidateChecksums: acceptance -----------------------------------------

// WriteChecksums output round-trips through ValidateChecksums.
func TestValidateChecksumsAcceptsWrittenManifest(t *testing.T) {
	dir := testsupport.TempDir(t)
	names := []string{"a.tar.gz", "b.tar.gz", "install.sh"}
	for _, n := range names {
		mkFile(t, dir, n, []byte("content-"+n))
	}
	if err := WriteChecksums(dir, names); err != nil {
		t.Fatalf("WriteChecksums: %v", err)
	}
	if err := ValidateChecksums(dir, names); err != nil {
		t.Fatalf("ValidateChecksums rejected a valid manifest: %v", err)
	}
}

// validFixture builds dir with three files and returns (dir, expected names,
// the hash of each) plus a valid sorted manifest already written. Tests then
// mutate the manifest to drive one failure mode each.
func validFixture(t *testing.T) (dir string, names []string, hashes map[string]string) {
	t.Helper()
	dir = testsupport.TempDir(t)
	names = []string{"a.tar.gz", "b.tar.gz", "install.sh"}
	hashes = map[string]string{}
	for _, n := range names {
		hashes[n] = mkFile(t, dir, n, []byte("content-"+n))
	}
	return dir, names, hashes
}

func lineFor(hash, name string) string { return hash + "  " + name }

// --- ValidateChecksums: forward-direction failures -------------------------

func TestValidateChecksumsFlippedHexDigit(t *testing.T) {
	dir, names, h := validFixture(t)
	bad := flipOneHexDigit(h["a.tar.gz"])
	writeManifest(t, dir, []string{
		lineFor(bad, "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a flipped hex digit, want hash-mismatch error")
	}
}

func TestValidateChecksumsMissingLine(t *testing.T) {
	dir, names, h := validFixture(t)
	// Drop a.tar.gz's line entirely; the file is still on disk.
	writeManifest(t, dir, []string{
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a manifest missing a line for an expected file, want error")
	}
}

func TestValidateChecksumsDuplicateLine(t *testing.T) {
	dir, names, h := validFixture(t)
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["a.tar.gz"], "a.tar.gz"), // duplicate, otherwise-valid line
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a duplicated manifest line, want error")
	}
}

func TestValidateChecksumsExtraUnexpectedFile(t *testing.T) {
	dir, names, h := validFixture(t)
	// Create and list a file not in the expected set.
	extraHash := mkFile(t, dir, "extra.tar.gz", []byte("content-extra"))
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(extraHash, "extra.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a line for an unexpected file, want error")
	}
}

func TestValidateChecksumsUnsafeFilename(t *testing.T) {
	dir, names, h := validFixture(t)
	// A path-separator (traversal-shaped) filename must be refused before any
	// file read is attempted.
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
		lineFor(sha256HexBytes([]byte("whatever")), "../evil"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a ../evil filename, want unsafe-name error")
	}
}

func TestValidateChecksumsLeadingDashFilename(t *testing.T) {
	dir, names, h := validFixture(t)
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
		lineFor(sha256HexBytes([]byte("whatever")), "-rf"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a leading-dash filename, want unsafe-name error")
	}
}

func TestValidateChecksumsUppercaseHash(t *testing.T) {
	dir, names, h := validFixture(t)
	writeManifest(t, dir, []string{
		lineFor(strings.ToUpper(h["a.tar.gz"]), "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted an uppercase hash, want syntax error")
	}
}

func TestValidateChecksumsOneSpaceSeparator(t *testing.T) {
	dir, names, h := validFixture(t)
	writeManifest(t, dir, []string{
		h["a.tar.gz"] + " a.tar.gz", // single space
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a one-space separator, want syntax error")
	}
}

// checksums.txt must never list itself.
func TestValidateChecksumsSelfListing(t *testing.T) {
	dir, names, h := validFixture(t)
	// A syntactically valid line for the manifest file itself.
	selfHash := sha256HexBytes([]byte("manifest-bytes"))
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
		lineFor(h["install.sh"], "install.sh"),
		lineFor(selfHash, checksumsFile),
	})
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("accepted a self-referential checksums.txt line, want error")
	}
}

// --- ValidateChecksums: reverse direction ----------------------------------

// The reverse (expected-but-absent) direction: every expected name must have a
// line even when every present line is itself valid. Here the manifest lists a
// strict subset of the expected set; the missing name has NO on-disk mismatch
// to trip the forward loop, so only the reverse check can catch it.
func TestValidateChecksumsExpectedFileWithNoLine(t *testing.T) {
	dir := testsupport.TempDir(t)
	expected := []string{"a.tar.gz", "b.tar.gz", "install.sh"}
	h := map[string]string{}
	for _, n := range expected {
		h[n] = mkFile(t, dir, n, []byte("content-"+n))
	}
	// Manifest lists only a and b — install.sh is expected but unlisted.
	writeManifest(t, dir, []string{
		lineFor(h["a.tar.gz"], "a.tar.gz"),
		lineFor(h["b.tar.gz"], "b.tar.gz"),
	})
	if err := ValidateChecksums(dir, expected); err == nil {
		t.Fatal("accepted an expected file with no manifest line, want reverse-direction error")
	}
}

// --- test-local helpers (independent of the code under test) ---------------

func flipOneHexDigit(h string) string {
	b := []byte(h)
	// Flip the last nibble to a different lowercase-hex digit.
	if b[len(b)-1] == 'a' {
		b[len(b)-1] = 'b'
	} else {
		b[len(b)-1] = 'a'
	}
	return string(b)
}

func sortStringsBytewise(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
