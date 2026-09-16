package app

// This file is the generated-bundle fast path (change 0413) for the finalize
// rebase controller: recognizing conflict stops whose only unmerged paths are
// Docket's own embedded asset bundle, regenerating that bundle in-process with
// the existing generator (internal/assets — the same entrypoints cmd/genassets
// drives), and continuing the owned rebase without spending a resolver
// reservation. Eligibility is deliberately narrow: the exact Docket module
// identity and the exact bundle location — never generic generated-file
// detection. Regeneration is conflict resolution only; it is never permission
// to merge or to bypass the post-rebase gate.

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
)

// embeddedBundleDir is the repo-relative home of the generated bundle — the
// same location cmd/genassets freezes (its embeddedRel constant).
const embeddedBundleDir = "internal/assets/embedded"

// docketModulePath is the module identity that gates eligibility: only
// Docket's own repository carries this bundle contract.
const docketModulePath = "github.com/danielhanold/docket"

// pathsGeneratedOnly reports whether every path is a bundle OUTPUT: the
// manifest or a tree/ payload strictly under embeddedBundleDir. An empty set
// is not generated-only (there is nothing to clear), and the directory name
// itself is not an output path.
func pathsGeneratedOnly(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	prefix := embeddedBundleDir + "/"
	for _, p := range paths {
		if !strings.HasPrefix(p, prefix) {
			return false
		}
	}
	return true
}

// bundleRepoEligible reports whether the feature workspace is Docket itself
// with the generator contract present: go.mod declares docketModulePath, every
// DefaultAllowedRoots root exists, and the committed bundle location exists. A
// probe error is never a clean "eligible" — any failure answers false and the
// stop falls through to the normal resolver path.
func bundleRepoEligible(wsDir string) bool {
	if goModModule(filepath.Join(wsDir, "go.mod")) != docketModulePath {
		return false
	}
	for _, root := range assets.DefaultAllowedRoots() {
		if _, err := os.Lstat(filepath.Join(wsDir, filepath.FromSlash(root.Root))); err != nil {
			return false
		}
	}
	if _, err := os.Lstat(filepath.Join(wsDir, filepath.FromSlash(embeddedBundleDir))); err != nil {
		return false
	}
	return true
}

// goModModule extracts the module path from a go.mod file, or "" when the file
// is unreadable or declares none.
func goModModule(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// regenerateEmbeddedBundle regenerates the bundle from the workspace's
// authored roots, mirroring cmd/genassets' publish mechanics: generate in
// memory, write into a staging directory beside the destination, then swap via
// remove+rename so a generation failure never leaves a partial bundle.
func regenerateEmbeddedBundle(wsDir string) error {
	m, payload, err := assets.Generate(wsDir, assets.DefaultAllowedRoots())
	if err != nil {
		return err
	}
	outDir := filepath.Join(wsDir, filepath.FromSlash(embeddedBundleDir))
	staging, err := os.MkdirTemp(filepath.Dir(outDir), ".embedded-staging-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, "embedded")
	if err := assets.WriteTree(staged, m, payload); err != nil {
		return err
	}
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	return os.Rename(staged, outDir)
}

// errBundleNotAdvancing marks a fast-path continue that left the rebase
// stopped on the same commit — regeneration cannot clear this stop, so it
// blocks through the existing failure path rather than looping.
var errBundleNotAdvancing = errors.New("the generated-bundle continue did not advance past the stopped commit")

// regenBundle returns the injected regeneration seam, or the in-process
// production generator when none is wired (production leaves RegenerateBundle
// unset).
func regenBundle(deps FinalizeDeps) func(string) error {
	if deps.RegenerateBundle != nil {
		return deps.RegenerateBundle
	}
	return regenerateEmbeddedBundle
}
