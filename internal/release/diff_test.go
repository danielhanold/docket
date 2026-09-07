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
