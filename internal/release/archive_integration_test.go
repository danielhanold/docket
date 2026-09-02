//go:build integration

package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// fileSHA256 hashes the whole file at path and returns lowercase hex — the
// independent oracle for VerifyArchive's returned digest.
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// readSingleHeader reopens a WriteArchive output and returns its one tar
// header, failing the test if the archive does not decode or holds anything
// other than exactly one member.
func readSingleHeader(t *testing.T, path string) *tar.Header {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip open %s: %v", path, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("read first member of %s: %v", path, err)
	}
	if _, err := tr.Next(); err == nil {
		t.Fatalf("archive %s holds more than one member", path)
	}
	return hdr
}

// writeGzTar builds a gzip'd tar at path from the supplied header/content
// pairs, verbatim — the hostile-archive factory for the refusal tests. It
// applies no normalization, so a caller can craft members VerifyArchive must
// reject.
func writeGzTar(t *testing.T, path string, members func(tw *tar.Writer)) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	members(tw)
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar %s: %v", path, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close gzip %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close %s: %v", path, err)
	}
}

func TestIntegrationReleaseArchiveWriteDeterministic(t *testing.T) {
	dir := testsupport.TempDir(t)
	bin := []byte("fake docket binary bytes\x00\x01\x02")
	const epoch = 1700000000

	p1 := filepath.Join(dir, "a.tar.gz")
	p2 := filepath.Join(dir, "b.tar.gz")
	if err := WriteArchive(p1, bin, epoch); err != nil {
		t.Fatalf("WriteArchive(p1) = %v", err)
	}
	if err := WriteArchive(p2, bin, epoch); err != nil {
		t.Fatalf("WriteArchive(p2) = %v", err)
	}
	b1, err := os.ReadFile(p1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := os.ReadFile(p2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Errorf("identical inputs produced differing archive bytes (%d vs %d bytes)", len(b1), len(b2))
	}
}

func TestIntegrationReleaseArchiveWriteEpochEntersStream(t *testing.T) {
	dir := testsupport.TempDir(t)
	bin := []byte("fake docket binary bytes")

	p1 := filepath.Join(dir, "a.tar.gz")
	p2 := filepath.Join(dir, "b.tar.gz")
	if err := WriteArchive(p1, bin, 1700000000); err != nil {
		t.Fatal(err)
	}
	if err := WriteArchive(p2, bin, 1700000001); err != nil {
		t.Fatal(err)
	}
	b1, _ := os.ReadFile(p1)
	b2, _ := os.ReadFile(p2)
	if bytes.Equal(b1, b2) {
		t.Error("archives with different epochs are byte-identical — epoch does not enter the stream")
	}
}

func TestIntegrationReleaseArchiveVerifyRoundTrip(t *testing.T) {
	dir := testsupport.TempDir(t)
	bin := []byte("fake docket binary of a known length")
	path := filepath.Join(dir, "docket_v1.2.3_linux_amd64.tar.gz")
	if err := WriteArchive(path, bin, 1700000000); err != nil {
		t.Fatal(err)
	}

	size, sum, err := VerifyArchive(path)
	if err != nil {
		t.Fatalf("VerifyArchive = %v, want nil", err)
	}
	if size != int64(len(bin)) {
		t.Errorf("VerifyArchive size = %d, want %d", size, len(bin))
	}
	if want := fileSHA256(t, path); sum != want {
		t.Errorf("VerifyArchive sha256 = %q, want %q", sum, want)
	}
	if sum != strings.ToLower(sum) {
		t.Errorf("VerifyArchive sha256 %q is not lowercase", sum)
	}
}

// TestIntegrationReleaseArchiveWriteNoHostLeakage reopens WriteArchive's own output and pins the
// header fields that would otherwise leak the building host's identity, plus
// the USTAR format selection.
func TestIntegrationReleaseArchiveWriteNoHostLeakage(t *testing.T) {
	dir := testsupport.TempDir(t)
	path := filepath.Join(dir, "docket.tar.gz")
	if err := WriteArchive(path, []byte("bin"), 1700000000); err != nil {
		t.Fatal(err)
	}
	hdr := readSingleHeader(t, path)
	if hdr.Uid != 0 || hdr.Gid != 0 {
		t.Errorf("member uid/gid = %d/%d, want 0/0", hdr.Uid, hdr.Gid)
	}
	if hdr.Uname != "" || hdr.Gname != "" {
		t.Errorf("member uname/gname = %q/%q, want empty", hdr.Uname, hdr.Gname)
	}
	if hdr.Name != "docket" {
		t.Errorf("member name = %q, want \"docket\"", hdr.Name)
	}
	if hdr.Mode&0o7777 != 0o755 {
		t.Errorf("member mode = %#o, want 0755", hdr.Mode)
	}
	if hdr.Format != tar.FormatUSTAR {
		t.Errorf("member format = %v, want USTAR", hdr.Format)
	}
}

// TestIntegrationReleaseArchiveVerifyRefusals hands VerifyArchive a battery of hostile archives
// crafted directly with archive/tar and asserts each is refused with an error
// that names the offending member (learning guards-are-code — each guard has a
// distinct, mutation-tested branch).
func TestIntegrationReleaseArchiveVerifyRefusals(t *testing.T) {
	dir := testsupport.TempDir(t)

	cases := []struct {
		name       string
		build      func(tw *tar.Writer)
		wantSubstr string // a substring the error must mention (the member)
	}{
		{
			name: "second member",
			build: func(tw *tar.Writer) {
				writeReg(t, tw, "docket", 0o755, []byte("bin"))
				writeReg(t, tw, "extra", 0o755, []byte("more"))
			},
			wantSubstr: "extra",
		},
		{
			name: "symlink member",
			build: func(tw *tar.Writer) {
				if err := tw.WriteHeader(&tar.Header{
					Typeflag: tar.TypeSymlink,
					Name:     "docket",
					Linkname: "/etc/passwd",
					Mode:     0o755,
					Format:   tar.FormatUSTAR,
				}); err != nil {
					t.Fatal(err)
				}
			},
			wantSubstr: "docket",
		},
		{
			name: "parent-traversal name",
			build: func(tw *tar.Writer) {
				writeReg(t, tw, "../docket", 0o755, []byte("bin"))
			},
			wantSubstr: "../docket",
		},
		{
			name: "subdir name",
			build: func(tw *tar.Writer) {
				writeReg(t, tw, "bin/docket", 0o755, []byte("bin"))
			},
			wantSubstr: "bin/docket",
		},
		{
			name: "wrong mode",
			build: func(tw *tar.Writer) {
				writeReg(t, tw, "docket", 0o644, []byte("bin"))
			},
			wantSubstr: "docket",
		},
		{
			name:       "empty archive",
			build:      func(tw *tar.Writer) {},
			wantSubstr: "no members",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "hostile_"+strings.ReplaceAll(tc.name, " ", "_")+".tar.gz")
			writeGzTar(t, path, tc.build)
			_, _, err := VerifyArchive(path)
			if err == nil {
				t.Fatalf("VerifyArchive(%s) = nil, want refusal", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("VerifyArchive(%s) error %q does not mention %q", tc.name, err.Error(), tc.wantSubstr)
			}
		})
	}
}

// writeReg is a test helper that writes one regular USTAR member verbatim.
func writeReg(t *testing.T, tw *tar.Writer, name string, mode int64, content []byte) {
	t.Helper()
	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     name,
		Mode:     mode,
		Size:     int64(len(content)),
		Format:   tar.FormatUSTAR,
	}); err != nil {
		t.Fatalf("WriteHeader(%q): %v", name, err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Write(%q): %v", name, err)
	}
}
