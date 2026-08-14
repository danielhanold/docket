package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
)

// A development install points a contributor's harnesses at their own checkout
// instead of at an extracted release bundle. That inverts one thing and one
// thing only: where the asset bytes come from. Everything else — ownership
// proofs, the journal, the published record — is the release path, because a
// contributor's home directory is as much theirs as anyone's.
//
// The checkout is only usable if it is internally consistent, so two gates run
// before anything is built or written:
//
//   - The committed bundle must speak this binary's asset protocol. A checkout
//     from the future would render targets this binary cannot reason about.
//   - Regenerating the bundle from the authored roots must reproduce the
//     committed tree byte for byte. Installing from a drifted checkout would
//     link harnesses at files the manifest does not describe, and the digest
//     recorded in the state would be a fiction.

const (
	// embeddedBundleRel is the committed bundle's home inside a checkout. It is
	// the same path cmd/genassets writes ("internal/assets/embedded"); the two
	// are tied by the drift gate in tests/test_asset_bundle_drift.sh rather than
	// by a shared constant, because genassets is a repo tool the installed
	// binary does not import.
	embeddedBundleRel = "internal/assets/embedded"
	// binaryName is what a development install calls the binary it builds.
	binaryName = "docket"
	// binaryMode is the one target whose mode is not the bundle's 0o644: an
	// installed binary nobody can execute is not installed.
	binaryMode = 0o755
	// roleBinary marks the built binary in the installed state. It belongs to
	// the installation itself rather than to any harness.
	roleBinary = "binary"
)

// DevOptions is a development install's inputs: the release options plus the
// checkout, the destination for the built binary, and the toolchain seam.
type DevOptions struct {
	Options
	SourceRoot string
	BinDir     string
	// GoRunner runs an argument vector in a directory. It is a vector and not a
	// command string on purpose: no part of a path a user typed is ever handed
	// to a shell.
	GoRunner func(dir string, argv []string) error
}

// DefaultGoRunner is the production toolchain seam.
func DefaultGoRunner(dir string, argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%w: empty command", ErrInvalidInput)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", argv[0], err, out)
	}
	return nil
}

// DevelopmentInstall installs from a checkout: harness material is linked into
// the source tree and the binary is built from it.
func DevelopmentInstall(o DevOptions) Outcome {
	out := Outcome{Mode: ModeDevelopment, StatePath: o.Roots.StatePath()}
	if err := requireOptions(o.Options); err != nil {
		return fail(out, ReasonInvalidOptions, err)
	}
	if o.GoRunner == nil {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no Go toolchain runner", ErrInvalidInput))
	}
	if decision := config.PreflightMutation(o.Config); !decision.Allowed {
		return fail(out, ReasonDeferredCapability, fmt.Errorf("%w: %d blocker(s), first: %s",
			config.ErrUnsupportedConfig, len(decision.Blockers), decision.Blockers[0].Path))
	}

	source, err := validateSourceRoot(o.SourceRoot)
	if err != nil {
		return fail(out, ReasonInvalidSourceRoot, err)
	}
	binDir, err := resolveBinDir(o.BinDir, o.Roots)
	if err != nil {
		return fail(out, ReasonInvalidOptions, err)
	}

	committed := filepath.Join(source, filepath.FromSlash(embeddedBundleRel))
	declared, err := readCommittedManifest(committed)
	if err != nil {
		return fail(out, ReasonAssetManifestInvalid, err)
	}
	if declared.AssetProtocol != assets.AssetProtocol {
		return fail(out, ReasonAssetProtocolMismatch, fmt.Errorf(
			"%w: %s declares asset protocol %d, this binary speaks %d",
			assets.ErrManifestInvalid, source, declared.AssetProtocol, assets.AssetProtocol))
	}

	manifest, payload, err := assets.Generate(source, assets.DefaultAllowedRoots())
	if err != nil {
		return fail(out, ReasonInvalidSourceRoot, fmt.Errorf("%w: %s", ErrInvalidInput, err))
	}
	diffs, err := assets.DiffTree(committed, manifest, payload)
	if err != nil {
		return fail(out, ReasonSourceAssetsDrifted, err)
	}
	if len(diffs) > 0 {
		for _, d := range diffs {
			out.Actions = append(out.Actions, Action{Op: OpDrift, Path: committed, Detail: d})
		}
		return fail(out, ReasonSourceAssetsDrifted, fmt.Errorf(
			"%w: %s is stale in %d path(s) — run: go generate ./internal/assets/",
			ErrDrifted, embeddedBundleRel, len(diffs)))
	}
	digest, err := assets.ComputeAssetSetID(manifest)
	if err != nil {
		return fail(out, ReasonAssetManifestInvalid, err)
	}
	out.AssetSetID = digest
	out.AssetProtocol = manifest.AssetProtocol

	selected, err := selectPlanners(o.Planners, o.Harnesses, o.Roots)
	if err != nil {
		return fail(out, selectionReason(err), err)
	}
	out.Harnesses = plannerNames(selected)

	// The lock spans everything from here to the commit, exactly as it does for
	// a release install: the two share applyPlan, and a build is not a licence
	// to interleave with another run's transaction. Taking it before the build
	// also keeps a refused run from spending a compile on work it will not do.
	lock, err := acquireInstallLock(o.Roots)
	if err != nil {
		return fail(out, lockReason(err), err)
	}
	defer lock.release()

	out, err = recoverPending(o.Options, out, lock)
	if err != nil {
		return fail(out, ReasonTransactionRecoveryRequired, err)
	}

	// The build runs before the transaction opens: a toolchain failure must
	// leave the user's installation exactly as it was, and the surest way to
	// promise that is to have written nothing yet.
	binary, err := buildBinary(o.GoRunner, source)
	if err != nil {
		return fail(out, ReasonBuildFailed, err)
	}
	// The build itself is not an action: it produced bytes, and whether those
	// bytes change anything is the binary target's create/update to report. An
	// action here would make every development install look like work.
	installedBinary := filepath.Join(binDir, binaryName)

	catalog := assets.NewCatalog(manifest, payloadOpener(payload))
	targets, owner, err := planTargets(selected, ModeDevelopment, source, catalog)
	if err != nil {
		return fail(out, ReasonInternal, err)
	}
	// The binary is an ordinary owned target, journaled and rolled back like any
	// other. It is attributed to no harness, which is what keeps a later
	// harness-scoped run from treating it as that harness's stale leftover.
	targets = append(targets, Target{
		Path:    installedBinary,
		Kind:    KindFile,
		Content: binary,
		Mode:    binaryMode,
		Role:    roleBinary,
	})

	return applyPlan(o.Options, plannedInstallation{
		mode:          ModeDevelopment,
		harnesses:     out.Harnesses,
		targets:       targets,
		owner:         owner,
		assetSetID:    digest,
		assetProtocol: manifest.AssetProtocol,
		sourceRoot:    source,
		sourceDigest:  digest,
	}, out)
}

// generateSourceCatalog reads a checkout's authored roots and returns the
// catalog they describe plus their digest. It only reads, which is what lets
// `install check` verify a development installation without writing.
func generateSourceCatalog(source string) (assets.Catalog, string, error) {
	if source == "" {
		return assets.Catalog{}, "", fmt.Errorf("%w: the installation records no source root", ErrInvalidInput)
	}
	manifest, payload, err := assets.Generate(source, assets.DefaultAllowedRoots())
	if err != nil {
		return assets.Catalog{}, "", fmt.Errorf("%w: %s", ErrDrifted, err)
	}
	digest, err := assets.ComputeAssetSetID(manifest)
	if err != nil {
		return assets.Catalog{}, "", err
	}
	return assets.NewCatalog(manifest, payloadOpener(payload)), digest, nil
}

// validateSourceRoot proves the path is a docket checkout before it is used to
// steer anything: it exists, it is a directory, and it carries every authored
// asset root the bundle is generated from.
func validateSourceRoot(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("%w: no source root", ErrInvalidInput)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%w: resolving %q: %s", ErrInvalidInput, p, err)
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("%w: %s does not exist", ErrInvalidInput, abs)
	case err != nil:
		return "", fmt.Errorf("%w: inspecting %s: %s", ErrInvalidInput, abs, err)
	case !info.IsDir():
		return "", fmt.Errorf("%w: %s is not a directory", ErrInvalidInput, abs)
	}
	// Canonical, because every link this install plans points into the checkout
	// and ownership proofs compare canonical paths.
	source, err := canonicalPath(abs)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidInput, err)
	}
	for _, root := range assets.DefaultAllowedRoots() {
		if _, err := os.Lstat(filepath.Join(source, filepath.FromSlash(root.Root))); err != nil {
			return "", fmt.Errorf("%w: %s carries no %s, so it is not a docket checkout",
				ErrInvalidInput, source, root.Root)
		}
	}
	return source, nil
}

// resolveBinDir picks where the built binary is installed.
func resolveBinDir(binDir string, roots UserRoots) (string, error) {
	if binDir == "" {
		binDir = roots.BinDir
	}
	if binDir == "" || !filepath.IsAbs(binDir) {
		return "", fmt.Errorf("%w: %q is not an absolute bin directory", ErrInvalidInput, binDir)
	}
	return filepath.Clean(binDir), nil
}

// readCommittedManifest decodes a checkout's committed bundle manifest without
// validating it, so an unreadable protocol number can still be reported as a
// protocol mismatch rather than as generic corruption.
func readCommittedManifest(bundleDir string) (assets.Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		return assets.Manifest{}, fmt.Errorf("%w: reading %s: %s", assets.ErrManifestInvalid, bundleDir, err)
	}
	return assets.DecodeManifest(raw)
}

// buildBinary runs the toolchain into a staging directory outside the user's
// installation and returns the bytes it produced. Staging elsewhere is what
// makes a failed build a no-op: nothing under the destination has been touched
// when the runner returns an error.
func buildBinary(run func(string, []string) error, source string) ([]byte, error) {
	staging, err := os.MkdirTemp("", "docket-build-")
	if err != nil {
		return nil, fmt.Errorf("%w: staging the build: %s", ErrBuildFailed, err)
	}
	defer os.RemoveAll(staging)

	staged := filepath.Join(staging, binaryName)
	if err := run(source, []string{"go", "build", "-o", staged, "./cmd/docket"}); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrBuildFailed, err)
	}
	body, err := os.ReadFile(staged)
	if err != nil {
		return nil, fmt.Errorf("%w: the build reported success but produced no binary: %s", ErrBuildFailed, err)
	}
	return body, nil
}

// payloadOpener turns a generated payload map into a catalog accessor. Each
// call hands back a copy: a catalog consumer that trimmed or appended to the
// slice would otherwise mutate the generated bundle in place.
func payloadOpener(payload map[string][]byte) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		body, ok := payload[p]
		if !ok {
			return nil, fmt.Errorf("%w: no payload for %s", assets.ErrManifestInvalid, p)
		}
		return append([]byte(nil), body...), nil
	}
}
