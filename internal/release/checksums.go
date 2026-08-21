package release

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// checksumsFile is the manifest filename. It is a distributable artifact but is
// never listed inside itself — a manifest cannot carry its own final hash.
const checksumsFile = "checksums.txt"

// checksumLineRE matches one well-formed manifest line: exactly 64 lowercase-hex
// characters, exactly two spaces (the sha256sum text-mode separator), then a
// non-empty filename. An uppercase hash, a short/long hash, or a one-space
// separator all fail to match and are rejected as malformed syntax. The line is
// matched by shape, not against an enumerated filename list.
var checksumLineRE = regexp.MustCompile(`^([0-9a-f]{64})  (.+)$`)

// hashFileHex returns the lowercase-hex SHA-256 of the file at path, streaming
// it so an arbitrarily large archive never lands in memory whole.
func hashFileHex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// WriteChecksums hashes each named file in dir and writes dir/checksums.txt with
// one "<64 lowercase hex>  <filename>\n" line (two spaces, sha256sum -c
// compatible) per file, sorted bytewise by filename. names must be exactly the
// distributable set; a missing file fails. The manifest is written to a temp
// file in dir and renamed into place, so a reader never observes a partial
// manifest (learning atomic-generated-write).
func WriteChecksums(dir string, names []string) error {
	// Sort a copy so the caller's slice is left untouched. Go's string
	// comparison is bytewise, which is exactly the required ordering.
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)

	var b strings.Builder
	for _, name := range sorted {
		sum, err := hashFileHex(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("checksum %s: %w", name, err)
		}
		fmt.Fprintf(&b, "%s  %s\n", sum, name)
	}

	return atomicWriteManifest(dir, b.String())
}

// atomicWriteManifest writes content to dir/checksums.txt via a temp file in dir
// plus an os.Rename (same filesystem, atomic).
func atomicWriteManifest(dir, content string) error {
	dest := filepath.Join(dir, checksumsFile)
	tmp, err := os.CreateTemp(dir, "."+checksumsFile+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp manifest in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// On any error the temp file is removed; on success the rename has already
	// consumed it and the remove is a harmless no-op.
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.WriteString(content); err != nil {
		return fmt.Errorf("write temp manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp manifest: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, dest, err)
	}
	return nil
}

// ValidateChecksums proves the correspondence between dir/checksums.txt and the
// expected distributable set in BOTH directions:
//
//   - Forward: every manifest line is syntactically well-formed, names a safe
//     filename (no path separator, no leading '-'), is not the manifest itself,
//     is not a duplicate, is one of the expected names, and matches the present
//     file's actual SHA-256.
//   - Reverse: every expected name has exactly one manifest line.
//
// A one-way guard would pass a manifest that silently dropped an artifact
// (learning correspondence-guard-runs-one-way); both directions are required.
func ValidateChecksums(dir string, expected []string) error {
	raw, err := os.ReadFile(filepath.Join(dir, checksumsFile))
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}

	expectedSet := make(map[string]struct{}, len(expected))
	for _, n := range expected {
		expectedSet[n] = struct{}{}
	}

	// seen records which filenames the manifest listed, both to catch
	// duplicates (forward) and to drive the expected-but-absent check (reverse).
	seen := make(map[string]bool, len(expected))

	text := strings.TrimSuffix(string(raw), "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}

	for i, line := range lines {
		m := checksumLineRE.FindStringSubmatch(line)
		if m == nil {
			return fmt.Errorf("manifest line %d %q is malformed: want <64 lowercase hex><two spaces><filename>", i+1, line)
		}
		hash, name := m[1], m[2]

		// Unsafe-name refusals come before any filesystem access so a
		// traversal-shaped or option-shaped name is never opened.
		if strings.Contains(name, "/") {
			return fmt.Errorf("manifest lists unsafe filename %q: contains a path separator", name)
		}
		if strings.HasPrefix(name, "-") {
			return fmt.Errorf("manifest lists unsafe filename %q: leading '-'", name)
		}
		if name == checksumsFile {
			return fmt.Errorf("manifest lists itself (%q); a checksum manifest never lists itself", checksumsFile)
		}
		if seen[name] {
			return fmt.Errorf("manifest lists %q more than once", name)
		}
		seen[name] = true
		if _, ok := expectedSet[name]; !ok {
			return fmt.Errorf("manifest lists unexpected file %q, not in the distributable set", name)
		}

		got, err := hashFileHex(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("manifest names %q but its file cannot be hashed: %w", name, err)
		}
		if got != hash {
			return fmt.Errorf("checksum mismatch for %q: manifest %s, actual %s", name, hash, got)
		}
	}

	// Reverse direction: every expected artifact must have appeared.
	for _, n := range expected {
		if !seen[n] {
			return fmt.Errorf("expected file %q has no manifest line", n)
		}
	}

	return nil
}
