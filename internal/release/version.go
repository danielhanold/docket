// Package release is the repository-owned release-packaging library (development
// and CI tooling only — never imported by the production docket binary). It
// accepts caller-proved facts and never infers them: the packager is handed a
// source root, a safe version, a full commit, and a source epoch, and validates
// every one before doing any work.
package release

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// safeVersion is the safe, filename-safe version grammar. It is compiled once at
// package load. The prerelease segment, when present, must lead with an
// alphanumeric ([0-9A-Za-z]) — a bare trailing dash or a separator-led
// prerelease is rejected.
var safeVersion = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$`)

// commitHex matches a full 40-character lowercase-hex Git commit.
var commitHex = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Tuple is one approved GOOS/GOARCH target.
type Tuple struct{ OS, Arch string }

// Tuples is the fixed approved target set, in emission order:
// {darwin,amd64}, {darwin,arm64}, {linux,amd64}, {linux,arm64}.
func Tuples() []Tuple {
	return []Tuple{
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
	}
}

// ArchiveName returns docket_<version>_<os>_<arch>.tar.gz for the tuple.
func ArchiveName(version string, t Tuple) string {
	return fmt.Sprintf("docket_%s_%s_%s.tar.gz", version, t.OS, t.Arch)
}

// ValidateVersion returns a descriptive error unless version matches the safe
// grammar ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z][0-9A-Za-z.-]*)?$.
func ValidateVersion(version string) error {
	if !safeVersion.MatchString(version) {
		return fmt.Errorf("version %q does not match the safe grammar v<major>.<minor>.<patch>[-prerelease]", version)
	}
	return nil
}

// Inputs are the caller-proved facts the packager accepts, never infers.
type Inputs struct {
	SourceRoot  string // absolute path to the checkout to build
	Version     string // safe version, validated
	Commit      string // full 40-hex source commit
	SourceEpoch int64  // unix seconds; derives BuildDate and all timestamps
	OutDir      string // destination directory for the bundle
}

// Validate checks every field before any work is done, aggregating all failures
// into one error so the caller learns about every bad field in a single call
// rather than one build at a time (learning validate-the-whole-input-set-first):
// version grammar, 40-hex lowercase commit, positive epoch, absolute existing
// SourceRoot, absolute OutDir.
func (in Inputs) Validate() error {
	var errs []error

	if err := ValidateVersion(in.Version); err != nil {
		errs = append(errs, err)
	}

	if !commitHex.MatchString(in.Commit) {
		errs = append(errs, fmt.Errorf("commit %q is not a 40-character lowercase-hex Git commit", in.Commit))
	}

	if in.SourceEpoch <= 0 {
		errs = append(errs, fmt.Errorf("source epoch %d is not a positive unix timestamp", in.SourceEpoch))
	}

	if !filepath.IsAbs(in.SourceRoot) {
		errs = append(errs, fmt.Errorf("SourceRoot %q is not an absolute path", in.SourceRoot))
	} else if info, err := os.Stat(in.SourceRoot); err != nil {
		errs = append(errs, fmt.Errorf("SourceRoot %q does not exist: %w", in.SourceRoot, err))
	} else if !info.IsDir() {
		errs = append(errs, fmt.Errorf("SourceRoot %q is not a directory", in.SourceRoot))
	}

	if !filepath.IsAbs(in.OutDir) {
		errs = append(errs, fmt.Errorf("OutDir %q is not an absolute path", in.OutDir))
	}

	return errors.Join(errs...)
}

// BuildDate renders SourceEpoch as a UTC RFC3339 timestamp
// (e.g. epoch 0 -> 1970-01-01T00:00:00Z).
func (in Inputs) BuildDate() string {
	return time.Unix(in.SourceEpoch, 0).UTC().Format(time.RFC3339)
}
