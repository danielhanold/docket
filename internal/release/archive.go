package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// archiveMember is the single member name every release archive must hold.
const archiveMember = "docket"

// WriteArchive writes path as a gzip'd USTAR tar holding exactly one regular
// member named "docket" with the given bytes, mode 0755, uid/gid 0, empty
// uname/gname, and ModTime = epoch (UTC). The gzip header's ModTime is epoch
// and its OS byte is 0xFF (unknown) with an empty Name, so no building-host
// identity leaks into the stream. Given identical inputs and toolchain the
// output is byte-identical.
//
// The archive is written to a temp file beside path and renamed into place, so
// a reader never observes a partial archive (learning atomic-generated-write).
func WriteArchive(path string, binary []byte, epoch int64) error {
	when := time.Unix(epoch, 0).UTC()

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp archive in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// On any error below, remove the temp file. On success the rename has
	// already consumed it, so the remove is a harmless no-op.
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	zw, err := gzip.NewWriterLevel(tmp, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("gzip writer: %w", err)
	}
	// Pin the gzip header so it carries no host identity and stays
	// deterministic: fixed ModTime, unknown-OS byte, empty Name/Comment.
	zw.ModTime = when
	zw.OS = 0xFF

	tw := tar.NewWriter(zw)
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     archiveMember,
		Mode:     0o755,
		Size:     int64(len(binary)),
		ModTime:  when,
		Uid:      0,
		Gid:      0,
		Uname:    "",
		Gname:    "",
		Format:   tar.FormatUSTAR,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}
	if _, err := tw.Write(binary); err != nil {
		return fmt.Errorf("write tar body: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp archive: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp archive: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

// VerifyArchive reopens path, proves it holds exactly one regular member named
// "docket" with mode 0755 and nothing else — no links, no directories, no
// second member, and no traversal-shaped name (a path separator or a basename
// other than "docket") — and returns the member's size and the archive file's
// lowercase-hex SHA-256. Every failure mode returns a distinct error naming the
// offending member.
func VerifyArchive(path string) (size int64, sha256hex string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, "", fmt.Errorf("read archive %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return 0, "", fmt.Errorf("archive %s: gzip open: %w", path, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var members []*tar.Header
	for {
		hdr, rerr := tr.Next()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return 0, "", fmt.Errorf("archive %s: reading tar: %w", path, rerr)
		}
		members = append(members, hdr)
	}

	if len(members) == 0 {
		return 0, "", fmt.Errorf("archive %s: contains no members, want exactly one %q", path, archiveMember)
	}
	if len(members) > 1 {
		return 0, "", fmt.Errorf("archive %s: contains more than one member (unexpected member %q), want exactly one %q", path, members[1].Name, archiveMember)
	}

	hdr := members[0]
	if hdr.Typeflag != tar.TypeReg {
		return 0, "", fmt.Errorf("archive %s: member %q is not a regular file (typeflag %q)", path, hdr.Name, string(hdr.Typeflag))
	}

	// Traversal/separator guard and basename guard are deliberately separate,
	// independently-mutable properties: a member is the docket binary only if
	// its name has no directory component (no path separator) AND its basename
	// is exactly "docket". Collapsing them would let a mutation that drops the
	// separator check slip a "bin/docket" or "../docket" member through on the
	// basename alone.
	if strings.Contains(hdr.Name, "/") {
		return 0, "", fmt.Errorf("archive %s: member %q contains a path separator (traversal-shaped name)", path, hdr.Name)
	}
	base := hdr.Name
	if i := strings.LastIndex(hdr.Name, "/"); i >= 0 {
		base = hdr.Name[i+1:]
	}
	if base != archiveMember {
		return 0, "", fmt.Errorf("archive %s: unexpected member name %q, want %q", path, hdr.Name, archiveMember)
	}

	if hdr.Mode&0o7777 != 0o755 {
		return 0, "", fmt.Errorf("archive %s: member %q has mode %#o, want 0755", path, hdr.Name, hdr.Mode&0o7777)
	}

	return hdr.Size, hexsum, nil
}
