package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/harness/codex"
	"github.com/danielhanold/docket/internal/install"
)

type options struct {
	Source, Binary, Destination, Pins string
}
type manifest struct {
	SchemaVersion         int               `json:"schema_version"`
	SourceCommit          string            `json:"source_commit"`
	AssetSetID            string            `json:"asset_set_id"`
	Binary                string            `json:"binary"`
	BinarySHA256          string            `json:"binary_sha256"`
	Files                 map[string]string `json:"files"`
	EvidenceAuditComplete bool              `json:"evidence_audit_complete"`
	ChangeID              int               `json:"change_id"`
	ChangePath            string            `json:"change_path"`
	MetadataRevision      string            `json:"metadata_revision"`
	PrimaryHEAD           string            `json:"primary_head"`
	BuildReady            bool              `json:"build_ready"`
	MutationAllowed       bool              `json:"mutation_allowed"`
	BuildTestCommand      string            `json:"build_test_command"`
	FinalizeTestCommand   string            `json:"finalize_test_command"`
	BaselineCommand       string            `json:"baseline_command"`
	BaselinePassed        bool              `json:"baseline_passed"`
	PinsSHA256            string            `json:"pins_sha256"`
}

func main() {
	var o options
	var runtimeManifest, runtimeHome string
	flag.StringVar(&runtimeManifest, "verify-runtime", "", "read-only check of global Codex roles and both skill aliases against fixture manifest")
	flag.StringVar(&runtimeHome, "runtime-home", "", "absolute user home to verify; does not prove already-loaded session instructions")
	flag.StringVar(&o.Source, "source", "", "clean candidate source")
	flag.StringVar(&o.Binary, "binary", "", "absolute candidate docket")
	flag.StringVar(&o.Destination, "destination", "", "new absolute fixture directory")
	flag.StringVar(&o.Pins, "pins", "", "operator-authored pin config")
	flag.Parse()
	if runtimeManifest != "" || runtimeHome != "" {
		if err := verifyRuntimePins(runtimeManifest, runtimeHome); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("runtime files verified; a fresh native session is still required")
		return
	}
	if err := prepare(o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare(o options) error {
	for n, p := range map[string]string{"source": o.Source, "binary": o.Binary, "destination": o.Destination, "pins": o.Pins} {
		if !filepath.IsAbs(p) {
			return fmt.Errorf("-%s must be absolute", n)
		}
	}
	if _, err := os.Lstat(o.Destination); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("destination must not exist")
	}
	head, err := gitOut(o.Source, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	dirty, err := gitOut(o.Source, "status", "--porcelain=v2")
	if err != nil {
		return err
	}
	if dirty != "" {
		return fmt.Errorf("candidate source is dirty")
	}
	info, err := os.Stat(o.Binary)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("candidate binary must be an executable regular file")
	}
	pinsBytes, err := os.ReadFile(o.Pins)
	if err != nil {
		return err
	}
	snap, _, err := config.Resolve([]config.Source{{Layer: config.LayerGlobal, Name: o.Pins, Data: pinsBytes}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		return fmt.Errorf("pins: %w", err)
	}
	catalog, err := candidateSourceCatalog(o.Source)
	if err != nil {
		return err
	}
	if err := verifyCandidateIdentity(o.Binary, head, catalog.Manifest.AssetSetID); err != nil {
		return err
	}
	sources, err := harness.ParseInventory(catalog)
	if err != nil {
		return err
	}
	for _, s := range sources {
		setting := snap.Effective.Agents["codex"][s.ShortName]
		if setting.Model.Value == "" || setting.Effort.Value == "" {
			return fmt.Errorf("pins omit exact codex model/effort for %s", s.Name)
		}
	}
	if err := os.MkdirAll(o.Destination, 0o755); err != nil {
		return err
	}
	primary := filepath.Join(o.Destination, "primary")
	if err := os.MkdirAll(primary, 0o755); err != nil {
		return err
	}
	if err := run("", "git", "init", "--bare", filepath.Join(o.Destination, "origin.git")); err != nil {
		return err
	}
	if err := run("", "git", "init", "-b", "main", primary); err != nil {
		return err
	}
	if err := run(primary, "git", "config", "user.name", "Docket Native Fixture"); err != nil {
		return err
	}
	if err := run(primary, "git", "config", "user.email", "fixture@docket.invalid"); err != nil {
		return err
	}
	if err := run(primary, "git", "remote", "add", "origin", filepath.Join(o.Destination, "origin.git")); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(primary, "README.md"), []byte("# Native Codex acceptance fixture\n"), 0o644); err != nil {
		return err
	}
	for path, body := range map[string][]byte{
		"go.mod":                []byte("module example.invalid/nativefixture\n\ngo 1.25\n"),
		"fixture/value.go":      []byte("package fixture\n\nfunc Value() int { return 1 }\n"),
		"fixture/value_test.go": []byte("package fixture\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(Value()) } }\n"),
	} {
		if err := writeFile(filepath.Join(primary, path), body, 0o644); err != nil {
			return err
		}
	}
	if err := run(primary, "git", "add", "README.md", "go.mod", "fixture"); err != nil {
		return err
	}
	if err := run(primary, "git", "commit", "-m", "fixture: initialize repository"); err != nil {
		return err
	}
	if err := run(primary, "git", "push", "-u", "origin", "main"); err != nil {
		return err
	}
	if err := runCandidate(o.Binary, primary, nil, "repository", "init", "--repo-dir", primary, "--json"); err != nil {
		return err
	}
	if err := run(primary, "git", "add", ".docket.yml", ".gitignore"); err != nil {
		return err
	}
	if err := run(primary, "git", "commit", "-m", "fixture: accept docket configuration"); err != nil {
		return err
	}
	if err := run(primary, "git", "push", "origin", "main"); err != nil {
		return err
	}
	var configured struct {
		Result          string   `json:"result"`
		RepositoryState string   `json:"repository_state"`
		PendingPaths    []string `json:"pending_paths"`
	}
	if err := runCandidate(o.Binary, primary, &configured, "repository", "configure-tests", "--repo-dir", primary, "--json"); err != nil {
		return err
	}
	if configured.Result != "applied" && configured.Result != "no-op" {
		return fmt.Errorf("candidate repository.configure-tests did not establish test policy")
	}
	files := map[string]string{}
	if err := renderCandidateAssets(o.Source, primary, catalog, snap); err != nil {
		return err
	}
	for _, source := range sources {
		dst := filepath.Join(primary, ".codex", "agents", source.Name+".toml")
		body, err := os.ReadFile(dst)
		if err != nil {
			return fmt.Errorf("candidate rendered agent %s: %w", source.Name, err)
		}
		files[rel(primary, dst)] = hash(body)
	}
	if err := writeCatalogSkills(catalog, primary, files); err != nil {
		return err
	}
	agents, err := os.ReadFile(filepath.Join(primary, "AGENTS.md"))
	if err != nil {
		return err
	}
	files["AGENTS.md"] = hash(agents)
	if err := run(primary, "git", "add", "AGENTS.md", ".codex", ".agents", ".gitignore", ".docket.yml"); err != nil {
		return err
	}
	if err := run(primary, "git", "commit", "-m", "fixture: add candidate assets and baseline"); err != nil {
		return err
	}
	if err := run(primary, "git", "push", "origin", "main"); err != nil {
		return err
	}
	const baselineCommand = "go test ./..."
	if err := run(primary, "go", "test", "./..."); err != nil {
		return fmt.Errorf("fixture baseline failed: %w", err)
	}
	control := filepath.Join(o.Destination, "control")
	createRequest := filepath.Join(control, "change-create.json")
	createBody := []byte(`{"request_id":"native-fixture-change","title":"Add a doubled fixture value","type":"feat","priority":"high","why":"Exercise a real native planner, worker, and reviewer.","what_changes":"Add Double so it returns twice Value and cover it with a test.","out_of_scope":"No production Docket changes."}`)
	if err := writeFile(createRequest, createBody, 0o600); err != nil {
		return err
	}
	var created struct {
		Result string `json:"result"`
		ID     int    `json:"id"`
		Path   string `json:"path"`
	}
	if err := runCandidate(o.Binary, primary, &created, "change", "create", "--repo-dir", primary, "--request", createRequest, "--json"); err != nil {
		return err
	}
	if created.Result != "applied" || created.ID <= 0 || created.Path == "" {
		return fmt.Errorf("candidate change.create did not create the fixture change")
	}
	type statusDoc struct {
		Result  string `json:"result"`
		Context struct {
			MetadataRevision string `json:"metadata_revision"`
		} `json:"context"`
		Changes []struct {
			ID                       int `json:"id"`
			Path, Version, Readiness string
		} `json:"changes"`
	}
	readStatus := func() (statusDoc, error) {
		var out statusDoc
		err := runCandidate(o.Binary, primary, &out, "status", "--repo-dir", primary, "--json")
		return out, err
	}
	createdStatus, err := readStatus()
	if err != nil {
		return err
	}
	recordVersion := ""
	for _, change := range createdStatus.Changes {
		if change.ID == created.ID && change.Path == created.Path {
			recordVersion = change.Version
		}
	}
	if recordVersion == "" || createdStatus.Context.MetadataRevision == "" {
		return fmt.Errorf("candidate status did not expose the created change version")
	}
	groomRequest := filepath.Join(control, "change-groom.json")
	groomBody, _ := json.Marshal(map[string]any{"change_id": created.ID, "path": created.Path, "version": recordVersion, "outcome": "spec", "spec_markdown": "# Add a doubled fixture value\n\nImplement `Double() int` in `fixture/value.go` and add a focused test. Run `go test ./...`.\n"})
	if err := writeFile(groomRequest, groomBody, 0o600); err != nil {
		return err
	}
	var groomed struct {
		Result   string `json:"result"`
		Revision string `json:"committed_revision"`
	}
	if err := runCandidate(o.Binary, primary, &groomed, "change", "groom", "--repo-dir", primary, "--request", groomRequest, "--json"); err != nil {
		return err
	}
	if groomed.Result != "applied" || groomed.Revision == "" {
		return fmt.Errorf("candidate change.groom did not make the fixture build-ready")
	}
	groomedStatus, err := readStatus()
	if err != nil {
		return err
	}
	buildReady := false
	for _, change := range groomedStatus.Changes {
		if change.ID == created.ID && change.Path == created.Path && change.Readiness == "build-ready" {
			buildReady = true
		}
	}
	if !buildReady || groomedStatus.Context.MetadataRevision == "" {
		return fmt.Errorf("candidate status did not confirm the groomed change is build-ready")
	}
	launch := []byte("# Candidate launch\n\nOpen this disposable primary in a fresh Codex app session. Verify project agent/resource loading and use the absolute candidate executable `" + o.Binary + "` for every catalog. Stop with `codex-candidate-loading-unverified` before substantive work if parent or child provenance cannot be established. Do not modify global installations.\n")
	if err := writeFile(filepath.Join(o.Destination, "LAUNCH.md"), launch, 0o644); err != nil {
		return err
	}
	files["../LAUNCH.md"] = hash(launch)
	bb, err := os.ReadFile(o.Binary)
	if err != nil {
		return err
	}
	configBytes, err := os.ReadFile(filepath.Join(primary, ".docket.yml"))
	if err != nil {
		return err
	}
	repoConfig, _, err := config.Resolve([]config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: configBytes}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		return fmt.Errorf("fixture config: %w", err)
	}
	if repoConfig.Effective.Build.TestCommand.Value == "" || repoConfig.Effective.Finalize.TestCommand.Value == "" {
		return fmt.Errorf("fixture test policy is unconfigured")
	}
	if err := verifyMutationConfiguration(o.Binary, primary); err != nil {
		return err
	}
	primaryHead, err := gitOut(primary, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	clean, err := gitOut(primary, "status", "--porcelain=v2")
	if err != nil {
		return err
	}
	if clean != "" {
		return fmt.Errorf("fixture primary is dirty after preparation")
	}
	tracked, err := trackedFileHashes(primary)
	if err != nil {
		return err
	}
	complete, err := completeManifestFiles(primary, files, tracked)
	if err != nil {
		return err
	}
	m := manifest{SchemaVersion: 1, SourceCommit: head, AssetSetID: catalog.Manifest.AssetSetID, Binary: o.Binary, BinarySHA256: hash(bb), Files: complete, EvidenceAuditComplete: false, ChangeID: created.ID, ChangePath: created.Path, MetadataRevision: groomedStatus.Context.MetadataRevision, PrimaryHEAD: primaryHead, BuildReady: buildReady, MutationAllowed: true, BuildTestCommand: repoConfig.Effective.Build.TestCommand.Value, FinalizeTestCommand: repoConfig.Effective.Finalize.TestCommand.Value, BaselineCommand: baselineCommand, BaselinePassed: true, PinsSHA256: hash(pinsBytes)}
	mb, _ := json.MarshalIndent(m, "", "  ")
	mb = append(mb, '\n')
	return writeFile(filepath.Join(o.Destination, "manifest.json"), mb, 0o644)
}

// A build-ready change can still be blocked by machine-layer configuration.
// Ask the candidate's read-only mutation preflight before advertising readiness;
// rendering native definitions from operator pins does not authorize copying
// those pins into a repository-local configuration layer.
func verifyMutationConfiguration(binary, primary string) error {
	var result struct {
		ProtocolVersion int    `json:"protocol_version"`
		Operation       string `json:"operation"`
		Result          string `json:"result"`
		MutationAllowed bool   `json:"mutation_allowed"`
	}
	// prepare initializes this fixture's default branch as main; the diagnostic
	// command requires that resolution context explicitly for integration: auto.
	if err := runCandidate(binary, primary, &result, "diagnostic", "config", "--repo-dir", primary, "--default-branch", "main", "--for-mutation", "--json"); err != nil {
		return fmt.Errorf("fixture mutation configuration: %w", err)
	}
	if result.ProtocolVersion != 1 || result.Operation != "config.preflight" || result.Result != "applied" || !result.MutationAllowed {
		return fmt.Errorf("fixture mutation configuration was not approved by config.preflight")
	}
	return nil
}

func verifyCandidateIdentity(binary, sourceCommit, sourceAssetSetID string) error {
	versionOut, err := exec.Command(binary, "version", "--json").Output()
	if err != nil {
		return fmt.Errorf("candidate version: %w", err)
	}
	var version struct {
		Commit     string `json:"commit"`
		AssetSetID string `json:"asset_set_id"`
	}
	if err := json.Unmarshal(versionOut, &version); err != nil {
		return fmt.Errorf("candidate version: invalid JSON: %w", err)
	}
	if version.Commit != sourceCommit {
		return fmt.Errorf("candidate binary commit does not match clean source HEAD")
	}
	if version.AssetSetID == "" || version.AssetSetID != sourceAssetSetID {
		return fmt.Errorf("candidate binary asset set does not match clean source assets")
	}
	return nil
}

func renderCandidateAssets(source, destination string, catalog assets.Catalog, snapshot *config.Snapshot) error {
	targets, err := codex.New().Plan(harness.PlanInput{Assets: catalog, Mode: harness.ModeDevelopment, AssetsDir: source, Roots: install.UserRoots{Home: destination}, Agents: snapshot.Effective.Agents})
	if err != nil {
		return err
	}
	for _, target := range targets {
		if target.Kind != install.KindFile {
			continue
		}
		dst := filepath.Join(destination, ".codex", "agents", filepath.Base(target.Path))
		if err := writeFile(dst, target.Content, 0o644); err != nil {
			return err
		}
	}
	gate, err := harness.RunGate(catalog)
	if err != nil {
		return err
	}
	agents := []byte("<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->\n" + harness.CodexDispatchInterior(gate) + "<!-- docket:dispatch:end -->\n")
	return writeFile(filepath.Join(destination, "AGENTS.md"), agents, 0o644)
}

func candidateSourceCatalog(source string) (assets.Catalog, error) {
	manifest, payload, err := assets.Generate(source, assets.DefaultAllowedRoots())
	if err != nil {
		return assets.Catalog{}, fmt.Errorf("candidate source assets: %w", err)
	}
	committed := filepath.Join(source, "internal", "assets", "embedded")
	diffs, err := assets.DiffTree(committed, manifest, payload)
	if err != nil {
		return assets.Catalog{}, fmt.Errorf("candidate source assets: %w", err)
	}
	if len(diffs) != 0 {
		return assets.Catalog{}, fmt.Errorf("candidate source embedded assets are stale: %s", strings.Join(diffs, "; "))
	}
	return assets.NewCatalog(manifest, func(path string) ([]byte, error) {
		body, ok := payload[path]
		if !ok {
			return nil, fmt.Errorf("candidate source asset %s is missing", path)
		}
		return append([]byte(nil), body...), nil
	}), nil
}

// completeManifestFiles merges Git's tracked-file inventory with files emitted
// explicitly by the fixture renderer. It re-reads every explicit output before
// publication and refuses a collision unless both inventories agree on bytes.
// This keeps ignored native role definitions in the manifest and makes drift
// between rendering and manifest creation visible.
func completeManifestFiles(root string, explicit, tracked map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(explicit)+len(tracked))
	for path, digest := range tracked {
		out[path] = digest
	}
	for path, digest := range explicit {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, fmt.Errorf("manifest explicit file %s: %w", path, err)
		}
		if actual := hash(body); actual != digest {
			return nil, fmt.Errorf("manifest explicit file %s changed after rendering", path)
		}
		if prior, exists := out[path]; exists && prior != digest {
			return nil, fmt.Errorf("manifest file %s has conflicting hashes", path)
		}
		out[path] = digest
	}
	return out, nil
}

func trackedFileHashes(root string) (map[string]string, error) {
	c := exec.Command("git", "ls-files", "-z")
	c.Dir = root
	b, err := c.Output()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, raw := range strings.Split(string(b), "\x00") {
		if raw == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(raw)))
		if err != nil {
			return nil, err
		}
		out[raw] = hash(body)
	}
	return out, nil
}
func runCandidate(binary, dir string, out any, args ...string) error {
	c := exec.Command(binary, args...)
	c.Dir = dir
	b, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("candidate %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("candidate %s returned invalid JSON: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}
func gitOut(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = dir
	b, err := c.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(b)), nil
}
func run(dir, name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Dir = dir
	if b, err := c.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(b)))
	}
	return nil
}
func writeFile(p string, b []byte, m os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, m)
}
func hash(b []byte) string      { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func rel(root, p string) string { r, _ := filepath.Rel(root, p); return filepath.ToSlash(r) }
func writeCatalogSkills(catalog assets.Catalog, primary string, files map[string]string) error {
	for _, entry := range catalog.EntriesByRole(assets.RoleSkill) {
		if !strings.HasPrefix(entry.Path, "skills/") {
			return fmt.Errorf("skill asset has unexpected path %q", entry.Path)
		}
		body, err := catalog.Bytes(entry.Path)
		if err != nil {
			return fmt.Errorf("skill asset %s: %w", entry.Path, err)
		}
		if int64(len(body)) != entry.Size || hash(body) != entry.SHA256 {
			return fmt.Errorf("skill asset %s differs from the verified catalog", entry.Path)
		}
		out := filepath.Join(primary, ".agents", filepath.FromSlash(entry.Path))
		if err := writeFile(out, body, os.FileMode(entry.Mode)); err != nil {
			return err
		}
		files[rel(primary, out)] = entry.SHA256
	}
	return nil
}
