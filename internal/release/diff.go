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
