package install

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	// buildinfoPkg is the package whose exported identity vars the stamp
	// targets. The three-`-X` format deliberately duplicates the release
	// packager's ("reusing the release packager's exact `-X` triple format"
	// per the 0340 spec): change 0317's internal/release is not merged, and
	// its branch must not become a dependency of this path.
	buildinfoPkg = "github.com/danielhanold/docket/internal/buildinfo"
)

// HandoffRunner executes the freshly built candidate binary with an explicit
// argument vector, relaying its stdio, and returns the candidate's exit code.
// It is a vector and never a shell string, for the same reason GoRunner is: no
// path a user typed is ever handed to a shell. A non-nil error means the
// candidate could not be launched at all; a launched candidate that exits
// non-zero reports that through exitCode with a nil error.
type HandoffRunner func(binary string, argv []string, env []string) (exitCode int, err error)

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
	// GitRunner runs a git argument vector in a directory and returns its
	// stdout. Like GoRunner it is a vector, never a shell string. It only
	// feeds build identity, which is a nicety: probe failures degrade the
	// build to unstamped, but a missing runner is a wiring bug and refused.
	GitRunner func(dir string, argv []string) (string, error)
	// Handoff runs the built candidate binary as the internal continuation that
	// actually plans and installs. It is required on the parent path exactly as
	// GoRunner is: the parent builds a candidate and hands the whole
	// installation to it, never installing anything itself.
	Handoff HandoffRunner
	// Continuation marks THIS process as the candidate: it plans and applies the
	// installation, builds nothing, and hands off to nothing. It is the
	// structural recursion stop — the candidate never re-enters the build path
	// — rather than a flag consulted from inside a shared body. It is set by the
	// private --internal-continuation flag and is not a supported public mode.
	Continuation bool
	// RepoDir is the explicit repository selection the public command received,
	// propagated verbatim into the candidate's handoff argv so parent and
	// candidate resolve the same repository. Empty means none was given, and an
	// empty value is omitted from the handoff argv. It is populated from the
	// --repo-dir flag that cli/root.go reads.
	RepoDir string
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

// DefaultHandoffRunner is the production handoff seam: it execs the candidate
// binary with the given argv and environment, wiring the candidate's stdio
// straight through to this process's own so the candidate's one result
// document reaches the user's terminal. A candidate that runs and exits
// non-zero returns that code with a nil error; only a failure to launch the
// candidate at all is an error.
func DefaultHandoffRunner(binary string, argv []string, env []string) (int, error) {
	cmd := exec.Command(binary, argv...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return -1, fmt.Errorf("%s: %w", binary, err)
	}
	return 0, nil
}

// DefaultGitRunner is the production git seam. It returns stdout alone:
// git writes progress and advice to stderr, and captured stderr must never
// leak into ldflags values.
func DefaultGitRunner(dir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("%w: empty command", ErrInvalidInput)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", argv[0], err)
	}
	return string(out), nil
}

// DevelopmentInstall installs from a checkout: harness material is linked into
// the source tree and the binary is built from it.
//
// It is two actors behind one entry point. The PARENT (Continuation == false)
// validates the checkout and builds a candidate binary from it, then hands the
// entire installation to that candidate — it acquires no lock, calls no
// planner, renders no target, and writes no destination. The CANDIDATE
// (Continuation == true) is that freshly built binary re-entering through the
// private continuation: it repeats every validation, then plans and applies,
// installing its OWN bytes as the binary target. Splitting the two is what
// makes the recursion stop structural — the candidate never reaches the build
// path — and what guarantees the renderer that produces the wrappers is the one
// the user just built, not the older binary that started the command.
func DevelopmentInstall(o DevOptions) Outcome {
	if o.Continuation {
		return developmentInstallCandidate(o)
	}
	return developmentInstallParent(o)
}

// developmentInstallParent is the currently-running binary's half: validate,
// build a candidate, hand off. It never mutates the user's installation — the
// candidate owns all of that — so it takes no lock and touches no destination.
func developmentInstallParent(o DevOptions) Outcome {
	out := Outcome{Mode: ModeDevelopment, StatePath: o.Roots.StatePath()}
	if err := requireOptions(o.Options); err != nil {
		return fail(out, ReasonInvalidOptions, err)
	}
	// The parent's own seams: it builds and it hands off. A missing one is a
	// wiring bug, refused before any work — exactly as the release path treats
	// a missing catalog.
	if o.GoRunner == nil {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no Go toolchain runner", ErrInvalidInput))
	}
	if o.GitRunner == nil {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no git runner", ErrInvalidInput))
	}
	if o.Handoff == nil {
		return fail(out, ReasonInvalidOptions, fmt.Errorf("%w: no handoff runner", ErrInvalidInput))
	}

	ds, refusal := validateDevSource(o)
	if refusal != nil {
		return *refusal
	}

	// Build the candidate into a private staging directory and keep it alive
	// until the handoff that runs it returns. A toolchain failure must leave the
	// user's installation exactly as it was, and the surest way to promise that
	// is to have written nothing to it — which the parent never does.
	staged, cleanup, err := buildStagedBinary(o.GoRunner, ds.source, buildIdentity(o.GitRunner, ds.source, time.Now))
	if err != nil {
		return fail(out, ReasonBuildFailed, err)
	}
	defer cleanup()

	argv := parentHandoffArgv(ds.source, ds.binDir, o.Harnesses, o.RepoDir)
	code, err := o.Handoff(staged, argv, os.Environ())
	if err != nil {
		return fail(out, ReasonHandoffFailed, fmt.Errorf(
			"%w: the candidate could not be launched: %s", ErrHandoffFailed, err))
	}
	if code != 0 {
		return fail(out, ReasonHandoffFailed, fmt.Errorf(
			"%w: the candidate installation exited %d", ErrHandoffFailed, code))
	}
	// The candidate already printed the sole result document to the shared
	// stdout; the parent adds nothing of its own and simply relays the success.
	out.Relayed = true
	out.RelayExitCode = code
	return out
}

// developmentInstallCandidate is the freshly built binary's half: repeat every
// mutable-world validation, then plan and apply. It builds nothing and hands
// off to nothing — its installed binary target is its OWN running bytes.
func developmentInstallCandidate(o DevOptions) Outcome {
	out := Outcome{Mode: ModeDevelopment, StatePath: o.Roots.StatePath()}
	if err := requireOptions(o.Options); err != nil {
		return fail(out, ReasonInvalidOptions, err)
	}

	// All the same read-only gates the parent passed are re-run here: a source
	// or committed bundle that changed between the parent's build and this
	// handoff must be caught before anything is installed from it.
	ds, refusal := validateDevSource(o)
	if refusal != nil {
		return *refusal
	}
	out.AssetSetID = ds.digest
	out.AssetProtocol = ds.manifest.AssetProtocol

	selected, err := selectPlanners(o.Planners, o.Harnesses, o.HarnessOptIns, o.Roots)
	if err != nil {
		return fail(out, selectionReason(err), err)
	}
	out.Harnesses = plannerNames(selected)

	// The lock spans everything from here to the commit, exactly as it does for
	// a release install: the two share applyPlan, and this run is not a licence
	// to interleave with another run's transaction.
	lock, err := acquireInstallLock(o.Roots)
	if err != nil {
		return fail(out, lockReason(err), err)
	}
	defer lock.release()

	out, err = recoverPending(o.Options, out, lock)
	if err != nil {
		return fail(out, ReasonTransactionRecoveryRequired, err)
	}

	// The installed binary is this candidate's OWN bytes: it is the binary the
	// parent just built, so the renderer that produces the wrappers below and
	// the executable that lands in the bin directory are one and the same.
	binary, err := candidateOwnBytes()
	if err != nil {
		return fail(out, ReasonFilesystemFailed, err)
	}
	installedBinary := filepath.Join(ds.binDir, binaryName)

	catalog := assets.NewCatalog(ds.manifest, payloadOpener(ds.payload))
	targets, owner, err := planTargets(selected, ModeDevelopment, ds.source, catalog)
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

	// The candidate re-resolved the repository phase itself (the parent passed
	// --repo-dir through verbatim), so machine and repository writes ride one
	// transaction here exactly as a release install does.
	return applyPlan(o.Options, plannedInstallation{
		mode:          ModeDevelopment,
		harnesses:     out.Harnesses,
		targets:       targets,
		owner:         owner,
		assetSetID:    ds.digest,
		assetProtocol: ds.manifest.AssetProtocol,
		sourceRoot:    ds.source,
		sourceDigest:  ds.digest,
	}, o.RepoPhase, out)
}

// devSource is the validated, freshly generated view of a checkout that both
// the parent and the candidate need: the canonical source root, the resolved
// bin directory, and the regenerated bundle plus its digest.
type devSource struct {
	source   string
	binDir   string
	manifest assets.Manifest
	payload  map[string][]byte
	digest   string
}

// validateDevSource runs every read-only gate a development install passes
// before it touches anything: config preflight, source root, bin directory,
// committed-manifest protocol, and source/asset drift. The parent and the
// candidate both call it, so the two cannot diverge in what they accept — a
// checkout the parent built from but that has since drifted is refused here by
// the candidate too. It writes nothing and takes no lock. requireOptions is the
// caller's to run first, because the two paths report a missing seam
// differently (the parent additionally requires its runners).
func validateDevSource(o DevOptions) (*devSource, *Outcome) {
	out := Outcome{Mode: ModeDevelopment, StatePath: o.Roots.StatePath()}
	if decision := config.PreflightMutation(o.Config); !decision.Allowed {
		return refuse(out, ReasonDeferredCapability, fmt.Errorf("%w: %d blocker(s), first: %s",
			config.ErrUnsupportedConfig, len(decision.Blockers), decision.Blockers[0].Path))
	}

	source, err := validateSourceRoot(o.SourceRoot)
	if err != nil {
		return refuse(out, ReasonInvalidSourceRoot, err)
	}
	binDir, err := resolveBinDir(o.BinDir, o.Roots)
	if err != nil {
		return refuse(out, ReasonInvalidOptions, err)
	}

	committed := filepath.Join(source, filepath.FromSlash(embeddedBundleRel))
	declared, err := readCommittedManifest(committed)
	if err != nil {
		return refuse(out, ReasonAssetManifestInvalid, err)
	}
	if declared.AssetProtocol != assets.AssetProtocol {
		return refuse(out, ReasonAssetProtocolMismatch, fmt.Errorf(
			"%w: %s declares asset protocol %d, this binary speaks %d",
			assets.ErrManifestInvalid, source, declared.AssetProtocol, assets.AssetProtocol))
	}

	manifest, payload, err := assets.Generate(source, assets.DefaultAllowedRoots())
	if err != nil {
		return refuse(out, ReasonInvalidSourceRoot, fmt.Errorf("%w: %s", ErrInvalidInput, err))
	}
	diffs, err := assets.DiffTree(committed, manifest, payload)
	if err != nil {
		return refuse(out, ReasonSourceAssetsDrifted, err)
	}
	if len(diffs) > 0 {
		for _, d := range diffs {
			out.Actions = append(out.Actions, Action{Op: OpDrift, Path: committed, Detail: d})
		}
		return refuse(out, ReasonSourceAssetsDrifted, fmt.Errorf(
			"%w: %s is stale in %d path(s) — run: go generate ./internal/assets/",
			ErrDrifted, embeddedBundleRel, len(diffs)))
	}
	digest, err := assets.ComputeAssetSetID(manifest)
	if err != nil {
		return refuse(out, ReasonAssetManifestInvalid, err)
	}
	return &devSource{source: source, binDir: binDir, manifest: manifest, payload: payload, digest: digest}, nil
}

// refuse packages a failed Outcome as validateDevSource's error return: the
// devSource is nil, the Outcome pointer non-nil, and the caller returns it.
func refuse(out Outcome, reason string, err error) (*devSource, *Outcome) {
	f := fail(out, reason, err)
	return nil, &f
}

// parentHandoffArgv is the exact argument vector the parent runs the candidate
// with. It names the private continuation (so the candidate does not build
// another candidate), the canonical source and bin directory, and the public
// command's requested scope — one --harness per explicit selection and
// --repo-dir when the public command received one.
func parentHandoffArgv(source, binDir string, harnesses []string, repoDir string) []string {
	argv := []string{"development", "install", "--internal-continuation",
		"--source", source, "--bin-dir", binDir}
	for _, h := range harnesses {
		argv = append(argv, "--harness", h)
	}
	if repoDir != "" {
		argv = append(argv, "--repo-dir", repoDir)
	}
	return argv
}

// candidateOwnBytes reads the running executable's own bytes. In a development
// install's candidate these bytes ARE the freshly built binary this process is,
// so they become the installed binary target's content. The path is resolved
// through its symlinks so a candidate launched by a symlinked path still reads
// the real file.
func candidateOwnBytes() ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("install: locating the running binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return nil, fmt.Errorf("install: resolving %s: %w", exe, err)
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("install: reading %s: %w", resolved, err)
	}
	return body, nil
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

// buildIdentity renders the -ldflags value stamping this build's identity,
// or "" when the checkout's git state cannot be read. Identity is a nicety,
// never a gate: every failure path degrades to an unstamped build, and the
// stamp is all-three-or-none — a Version beside an unknown Commit would be a
// new, misleading shape. Dirtiness is probed once, via describe's --dirty
// suffix, and applied to both Version and Commit.
func buildIdentity(run func(string, []string) (string, error), source string, now func() time.Time) string {
	describe, err := run(source, []string{"git", "describe", "--tags", "--always", "--dirty"})
	if err != nil {
		return ""
	}
	head, err := run(source, []string{"git", "rev-parse", "HEAD"})
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(describe)
	commit := strings.TrimSpace(head)
	if strings.HasSuffix(version, "-dirty") {
		commit += "-dirty"
	}
	for _, v := range []string{version, commit} {
		// A value with whitespace would silently corrupt the space-separated
		// -X list; treat it as a failed probe, not a stampable identity.
		if v == "" || strings.ContainsAny(v, " \t\n") {
			return ""
		}
	}
	return fmt.Sprintf("-X %s.Version=%s -X %s.Commit=%s -X %s.BuildDate=%s",
		buildinfoPkg, version, buildinfoPkg, commit, buildinfoPkg,
		now().UTC().Format(time.RFC3339))
}

// buildStagedBinary runs the toolchain into a staging directory outside the
// user's installation and returns the path to the binary it produced plus a
// cleanup that removes the staging directory. Staging elsewhere is what makes a
// failed build a no-op: nothing under the destination has been touched when the
// runner returns an error. The caller keeps the staged file alive until the
// handoff that executes it returns, then calls cleanup — so, unlike a
// build-and-read helper, this one hands back the path, not the bytes.
func buildStagedBinary(run func(string, []string) error, source, ldflags string) (string, func(), error) {
	staging, err := os.MkdirTemp("", "docket-build-")
	if err != nil {
		return "", nil, fmt.Errorf("%w: staging the build: %s", ErrBuildFailed, err)
	}
	cleanup := func() { os.RemoveAll(staging) }

	staged := filepath.Join(staging, binaryName)
	argv := []string{"go", "build"}
	if ldflags != "" {
		argv = append(argv, "-ldflags", ldflags)
	}
	argv = append(argv, "-o", staged, "./cmd/docket")
	if err := run(source, argv); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("%w: %s", ErrBuildFailed, err)
	}
	if _, err := os.Stat(staged); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("%w: the build reported success but produced no binary: %s", ErrBuildFailed, err)
	}
	return staged, cleanup, nil
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
