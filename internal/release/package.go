package release

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ldflagsPkg is the buildinfo package whose exported identity vars the packager
// injects at link time. Anchored to the symbol path, not a line, so drift is
// mechanically visible against the real package.
const ldflagsPkg = "github.com/danielhanold/docket/internal/buildinfo"

// installScriptName is the rendered downloader's name inside the bundle.
const installScriptName = "install.sh"

// bundleFileNames returns every file Package emits into OutDir: the four
// archives, the rendered downloader, and the manifest — in no particular order
// (the collision check and directory listing do not depend on order).
func bundleFileNames(version string) []string {
	names := make([]string, 0, len(Tuples())+2)
	for _, t := range Tuples() {
		names = append(names, ArchiveName(version, t))
	}
	names = append(names, installScriptName, checksumsFile)
	return names
}

// distributableNames returns the files that appear in checksums.txt: the four
// archives plus the rendered downloader. The manifest never lists itself.
func distributableNames(version string) []string {
	names := make([]string, 0, len(Tuples())+1)
	for _, t := range Tuples() {
		names = append(names, ArchiveName(version, t))
	}
	names = append(names, installScriptName)
	return names
}

// Package builds the full candidate bundle into in.OutDir: four archives,
// checksums.txt, and the rendered install.sh. The output is byte-deterministic
// for equal inputs + toolchain. goBin is the go tool to run ("go" in
// production; injectable for tests). It refuses a non-empty OutDir collision
// set — any target bundle file already present in OutDir aborts the run before
// any build, so an existing artifact is never clobbered.
//
// Sequence (spec order): Validate the whole input set; refuse collisions; for
// each approved tuple cross-build ./cmd/docket with CGO disabled, -trimpath, and
// the injected buildinfo identity, then WriteArchive and the mandatory reopening
// VerifyArchive; run the host-tuple identity check on the host binary before it
// is archived so a wrong stamp fails packaging everywhere; render and write the
// downloader with an explicit 0755 mode; WriteChecksums over the distributable
// set; and ValidateChecksums before returning.
func Package(in Inputs, goBin string) error {
	if err := in.Validate(); err != nil {
		return fmt.Errorf("invalid packaging inputs: %w", err)
	}

	if err := os.MkdirAll(in.OutDir, 0o755); err != nil {
		return fmt.Errorf("create OutDir %s: %w", in.OutDir, err)
	}

	// Collision refusal: any target file already present aborts before any
	// build so nothing is clobbered. The check covers the whole bundle set, not
	// only the archives.
	for _, name := range bundleFileNames(in.Version) {
		p := filepath.Join(in.OutDir, name)
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("OutDir collision: %s already exists in %s; refusing to clobber", name, in.OutDir)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", p, err)
		}
	}

	// A scratch dir for the raw build outputs, separate from OutDir so a failed
	// build never leaves a partial binary among the bundle files.
	buildDir, err := os.MkdirTemp("", "docket-release-build-*")
	if err != nil {
		return fmt.Errorf("create build scratch dir: %w", err)
	}
	defer os.RemoveAll(buildDir)

	ldflags := fmt.Sprintf("-X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s",
		ldflagsPkg, in.Version, ldflagsPkg, in.Commit, ldflagsPkg, in.BuildDate())

	hostOS, hostArch := runtime.GOOS, runtime.GOARCH

	for _, t := range Tuples() {
		tmpBin := filepath.Join(buildDir, "docket-"+t.OS+"-"+t.Arch)

		cmd := exec.Command(goBin, "build", "-trimpath", "-ldflags", ldflags, "-o", tmpBin, "./cmd/docket")
		cmd.Dir = in.SourceRoot
		// CGO_ENABLED=0 keeps the binaries static and cross-buildable and out of
		// the host C toolchain; GOOS/GOARCH select the tuple; GOFLAGS is cleared
		// so an ambient -mod/-tags value cannot enter the release bytes. These
		// appended entries win over any ambient copy in os.Environ() because
		// exec honors the last occurrence.
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+t.OS,
			"GOARCH="+t.Arch,
			"GOFLAGS=",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s/%s: %w\n%s", t.OS, t.Arch, err, out)
		}

		binary, err := os.ReadFile(tmpBin)
		if err != nil {
			return fmt.Errorf("read built binary for %s/%s: %w", t.OS, t.Arch, err)
		}

		// Host-tuple identity check, before archiving: run the freshly built
		// host binary and assert its self-reported identity is exactly the
		// stamp we injected. A wrong stamp fails packaging here, for real, not
		// only under test. Foreign tuples cannot be executed on this host; they
		// carry identical ldflags and are identity-checked by the CI native
		// smokes (external truth).
		if t.OS == hostOS && t.Arch == hostArch {
			if err := checkHostIdentity(tmpBin, in); err != nil {
				return fmt.Errorf("host-tuple identity check failed for %s/%s: %w", t.OS, t.Arch, err)
			}
		}

		archivePath := filepath.Join(in.OutDir, ArchiveName(in.Version, t))
		if err := WriteArchive(archivePath, binary, in.SourceEpoch); err != nil {
			return fmt.Errorf("write archive for %s/%s: %w", t.OS, t.Arch, err)
		}
		// The reopening verify is mandatory, not optional: it re-reads the
		// archive from disk and proves it holds exactly the single docket member.
		if _, _, err := VerifyArchive(archivePath); err != nil {
			return fmt.Errorf("verify archive for %s/%s: %w", t.OS, t.Arch, err)
		}
	}

	// Render and write the downloader with an explicit 0755 mode: the atomic
	// write lands via a 0600 temp file, so the chmod is load-bearing under any
	// umask (learning promised-file-mode-needs-explicit-chmod).
	rendered, err := RenderDownloader(in.Version)
	if err != nil {
		return fmt.Errorf("render downloader: %w", err)
	}
	if err := writeInstaller(filepath.Join(in.OutDir, installScriptName), rendered); err != nil {
		return fmt.Errorf("write %s: %w", installScriptName, err)
	}

	// Manifest over the distributable set, then the bidirectional validation as
	// a final gate before returning a bundle.
	dist := distributableNames(in.Version)
	if err := WriteChecksums(in.OutDir, dist); err != nil {
		return fmt.Errorf("write checksums: %w", err)
	}
	if err := ValidateChecksums(in.OutDir, dist); err != nil {
		return fmt.Errorf("validate checksums: %w", err)
	}

	return nil
}

// checkHostIdentity runs the built host binary's `version --json` operation and
// asserts its self-reported build identity matches the injected stamp exactly.
func checkHostIdentity(binPath string, in Inputs) error {
	out, err := exec.Command(binPath, "version", "--json").Output()
	if err != nil {
		return fmt.Errorf("run %s version --json: %w", binPath, err)
	}
	var id struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildDate string `json:"build_date"`
	}
	if err := json.Unmarshal(out, &id); err != nil {
		return fmt.Errorf("decode version --json (%q): %w", string(out), err)
	}
	if id.Version != in.Version {
		return fmt.Errorf("version stamp mismatch: binary reports %q, want %q", id.Version, in.Version)
	}
	if id.Commit != in.Commit {
		return fmt.Errorf("commit stamp mismatch: binary reports %q, want %q", id.Commit, in.Commit)
	}
	if want := in.BuildDate(); id.BuildDate != want {
		return fmt.Errorf("build_date stamp mismatch: binary reports %q, want %q", id.BuildDate, want)
	}
	return nil
}

// writeInstaller writes content to path via a temp file in the same directory
// plus an os.Rename (atomic, same filesystem — learning atomic-generated-write),
// then chmods the final file to 0755 explicitly so the mode does not depend on
// the process umask.
func writeInstaller(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp installer in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	if _, err := tmp.WriteString(content); err != nil {
		return fmt.Errorf("write temp installer: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp installer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp installer: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return fmt.Errorf("chmod %s 0755: %w", path, err)
	}
	return nil
}
