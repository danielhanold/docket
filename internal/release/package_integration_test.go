//go:build integration

package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// Integration-test identity fixture. The commit is a syntactically valid
// 40-hex string (strings.Repeat("ab", 20)); the epoch is a fixed instant so
// BuildDate is deterministic and the whole run is reproducible.
const (
	itVersion = "v0.0.1-planintegration"
	itCommit  = "abababababababababababababababababababab" // 40 hex
	itEpoch   = int64(1700000000)
)

// repoRoot resolves the repository checkout that owns this test file:
// internal/release/package_integration_test.go -> ../.. Package builds ./cmd/docket from
// there, so the path must be the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed; cannot locate the repo root")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	root = filepath.Clean(root)
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolved repo root %q has no go.mod: %v", root, err)
	}
	return root
}

func itInputs(t *testing.T, outDir string) Inputs {
	return Inputs{
		SourceRoot:  repoRoot(t),
		Version:     itVersion,
		Commit:      itCommit,
		SourceEpoch: itEpoch,
		OutDir:      outDir,
	}
}

// extractMember reopens a bundle archive and returns its single member's bytes.
func extractMember(t *testing.T, archivePath string) []byte {
	t.Helper()
	raw, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read %s: %v", archivePath, err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip open %s: %v", archivePath, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("read tar member of %s: %v", archivePath, err)
	}
	if hdr.Name != "docket" {
		t.Fatalf("member name %q, want docket", hdr.Name)
	}
	body, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read tar body of %s: %v", archivePath, err)
	}
	return body
}

// TestIntegrationReleasePackageEndToEnd is the one real end-to-end run: four cross-builds,
// four archives, sorted manifest, rendered downloader, host-tuple identity
// check. The four cross-builds are warm from tests/test_go_toolchain.sh's
// existing four-tuple check, so this is seconds warm (minutes cold).
func TestIntegrationReleasePackageEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping four-tuple build integration in -short mode")
	}
	outDir := testsupport.TempDir(t)
	in := itInputs(t, outDir)

	if err := Package(in, "go"); err != nil {
		t.Fatalf("Package: %v", err)
	}

	// Exactly the six expected names, nothing else.
	var archiveNames []string
	for _, tuple := range Tuples() {
		archiveNames = append(archiveNames, ArchiveName(itVersion, tuple))
	}
	wantSet := map[string]bool{"checksums.txt": true, "install.sh": true}
	for _, n := range archiveNames {
		wantSet[n] = true
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read OutDir: %v", err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	sort.Strings(got)
	if len(got) != len(wantSet) {
		t.Fatalf("OutDir holds %v, want exactly the 6 bundle files (%d)", got, len(wantSet))
	}
	for _, n := range got {
		if !wantSet[n] {
			t.Fatalf("OutDir holds unexpected file %q; bundle set is %v", n, wantSet)
		}
	}

	// VerifyArchive passes on all four.
	for _, n := range archiveNames {
		if _, _, err := VerifyArchive(filepath.Join(outDir, n)); err != nil {
			t.Fatalf("VerifyArchive(%s): %v", n, err)
		}
	}

	// ValidateChecksums passes over the distributable set (4 archives +
	// install.sh; the manifest never lists itself).
	checksumSet := append([]string(nil), archiveNames...)
	checksumSet = append(checksumSet, "install.sh")
	if err := ValidateChecksums(outDir, checksumSet); err != nil {
		t.Fatalf("ValidateChecksums: %v", err)
	}

	// Rendered install.sh: stamped to the bundle version and mode 0755.
	installBytes, err := os.ReadFile(filepath.Join(outDir, "install.sh"))
	if err != nil {
		t.Fatalf("read install.sh: %v", err)
	}
	if !strings.Contains(string(installBytes), `DOCKET_DEFAULT_VERSION="`+itVersion+`"`) {
		t.Fatalf("install.sh not stamped to %s", itVersion)
	}
	if strings.Contains(string(installBytes), downloaderPlaceholder) {
		t.Fatalf("install.sh still carries the raw placeholder")
	}
	info, err := os.Stat(filepath.Join(outDir, "install.sh"))
	if err != nil {
		t.Fatalf("stat install.sh: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("install.sh mode %#o, want 0755", info.Mode().Perm())
	}

	// Host-tuple identity: extract the host GOOS/GOARCH archive's member, run
	// `docket version --json`, and assert the stamped identity exactly. A host
	// process cannot execute foreign-tuple binaries; the other three tuples get
	// identical ldflags and are identity-checked by the CI native smokes
	// (external truth). Package itself runs this same check internally so a
	// wrong stamp fails packaging everywhere, not only under test.
	hostTuple := Tuple{OS: runtime.GOOS, Arch: runtime.GOARCH}
	hostArchive := filepath.Join(outDir, ArchiveName(itVersion, hostTuple))
	if _, err := os.Stat(hostArchive); err != nil {
		t.Skipf("host tuple %s/%s is not in the approved set; identity is CI external truth", runtime.GOOS, runtime.GOARCH)
	}
	member := extractMember(t, hostArchive)
	binPath := filepath.Join(testsupport.TempDir(t), "docket")
	if err := os.WriteFile(binPath, member, 0o755); err != nil {
		t.Fatalf("write extracted host binary: %v", err)
	}
	out, err := exec.Command(binPath, "version", "--json").Output()
	if err != nil {
		t.Fatalf("run extracted host binary version --json: %v", err)
	}
	var id struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildDate string `json:"build_date"`
	}
	if err := json.Unmarshal(out, &id); err != nil {
		t.Fatalf("decode version --json (%q): %v", string(out), err)
	}
	if id.Version != itVersion {
		t.Fatalf("host binary version %q, want %q", id.Version, itVersion)
	}
	if id.Commit != itCommit {
		t.Fatalf("host binary commit %q, want %q", id.Commit, itCommit)
	}
	if id.BuildDate != in.BuildDate() {
		t.Fatalf("host binary build_date %q, want %q", id.BuildDate, in.BuildDate())
	}
}

// TestIntegrationReleasePackageDeterministic packages the same inputs twice into two separate
// directories and byte-compares checksums.txt. Because checksums.txt transitively
// pins every archive's bytes, equal manifests prove the whole bundle is
// byte-deterministic for equal inputs + toolchain.
func TestIntegrationReleasePackageDeterministic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping determinism double-build in -short mode")
	}
	dirA := testsupport.TempDir(t)
	dirB := testsupport.TempDir(t)
	if err := Package(itInputs(t, dirA), "go"); err != nil {
		t.Fatalf("Package A: %v", err)
	}
	if err := Package(itInputs(t, dirB), "go"); err != nil {
		t.Fatalf("Package B: %v", err)
	}
	a, err := os.ReadFile(filepath.Join(dirA, "checksums.txt"))
	if err != nil {
		t.Fatalf("read checksums A: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dirB, "checksums.txt"))
	if err != nil {
		t.Fatalf("read checksums B: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("checksums.txt differ between runs; bundle is not deterministic:\nA:\n%s\nB:\n%s", a, b)
	}
}

// TestIntegrationReleasePackageRefusesCollision proves the OutDir collision guard: a bundle file
// already present in OutDir makes Package refuse before doing any build work,
// so an existing artifact is never clobbered. Refusal happens before the four
// cross-builds, so this test is fast even though it exercises Package.
func TestIntegrationReleasePackageRefusesCollision(t *testing.T) {
	outDir := testsupport.TempDir(t)
	// Plant one of the target archive names.
	collision := ArchiveName(itVersion, Tuples()[0])
	if err := os.WriteFile(filepath.Join(outDir, collision), []byte("preexisting"), 0o644); err != nil {
		t.Fatalf("plant collision: %v", err)
	}
	err := Package(itInputs(t, outDir), "go")
	if err == nil {
		t.Fatal("Package must refuse when a target bundle file already exists in OutDir")
	}
	if !strings.Contains(err.Error(), collision) {
		t.Fatalf("collision error %q does not name the offending file %q", err, collision)
	}
	// The pre-existing file is preserved byte-for-byte.
	got, err := os.ReadFile(filepath.Join(outDir, collision))
	if err != nil {
		t.Fatalf("read preserved file: %v", err)
	}
	if string(got) != "preexisting" {
		t.Fatalf("collision guard did not preserve the existing file; got %q", got)
	}
}

// TestIntegrationReleasePackageBundleValidatesChecksums is the doctored refusal-path unit test for
// the ValidateChecksums gate Package runs before returning. A hand-built bundle
// (two tiny files + a manifest) validates clean; flipping one byte of a listed
// file makes ValidateChecksums — the exact guard Package invokes — redden. This
// pins that the final gate bites on corruption; a full Package build is not
// needed to exercise the guard function.
func TestIntegrationReleasePackageBundleValidatesChecksums(t *testing.T) {
	dir := testsupport.TempDir(t)
	names := []string{"a.tar.gz", "install.sh"}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("body of "+n), 0o644); err != nil {
			t.Fatalf("write %s: %v", n, err)
		}
	}
	if err := WriteChecksums(dir, names); err != nil {
		t.Fatalf("WriteChecksums: %v", err)
	}
	if err := ValidateChecksums(dir, names); err != nil {
		t.Fatalf("clean bundle must validate: %v", err)
	}
	// Corrupt one listed file: the manifest is now stale, so the gate must fire.
	if err := os.WriteFile(filepath.Join(dir, "a.tar.gz"), []byte("CORRUPTED"), 0o644); err != nil {
		t.Fatalf("corrupt file: %v", err)
	}
	if err := ValidateChecksums(dir, names); err == nil {
		t.Fatal("ValidateChecksums must redden when a listed file is corrupted")
	}
}
