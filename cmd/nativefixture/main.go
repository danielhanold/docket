package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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

type options struct{ Source, Binary, Destination, Pins string }
type manifest struct {
	SchemaVersion         int               `json:"schema_version"`
	SourceCommit          string            `json:"source_commit"`
	Binary                string            `json:"binary"`
	BinarySHA256          string            `json:"binary_sha256"`
	Files                 map[string]string `json:"files"`
	EvidenceAuditComplete bool              `json:"evidence_audit_complete"`
	ChangeID              int               `json:"change_id"`
	ChangePath            string            `json:"change_path"`
	MetadataRevision      string            `json:"metadata_revision"`
}

func main() {
	var o options
	flag.StringVar(&o.Source, "source", "", "clean candidate source")
	flag.StringVar(&o.Binary, "binary", "", "absolute candidate docket")
	flag.StringVar(&o.Destination, "destination", "", "new absolute fixture directory")
	flag.StringVar(&o.Pins, "pins", "", "operator-authored pin config")
	flag.Parse()
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
	versionOut, err := exec.Command(o.Binary, "version", "--json").Output()
	if err != nil {
		return fmt.Errorf("candidate version: %w", err)
	}
	var version struct {
		Commit string `json:"commit"`
	}
	if json.Unmarshal(versionOut, &version) != nil || version.Commit != head {
		return fmt.Errorf("candidate binary commit does not match clean source HEAD")
	}
	pinsBytes, err := os.ReadFile(o.Pins)
	if err != nil {
		return err
	}
	snap, _, err := config.Resolve([]config.Source{{Layer: config.LayerGlobal, Name: o.Pins, Data: pinsBytes}}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		return fmt.Errorf("pins: %w", err)
	}
	catalog, err := assets.EmbeddedCatalog()
	if err != nil {
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
	if err := run(primary, "git", "add", "README.md"); err != nil {
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
	roots := install.UserRoots{Home: filepath.Join(o.Destination, "staging-home")}
	targets, err := codex.New().Plan(harness.PlanInput{Assets: catalog, Mode: harness.ModeDevelopment, AssetsDir: o.Source, Roots: roots, Agents: snap.Effective.Agents})
	if err != nil {
		return err
	}
	files := map[string]string{}
	for _, t := range targets {
		if t.Kind == install.KindFile {
			dst := filepath.Join(primary, ".codex", "agents", filepath.Base(t.Path))
			if err := writeFile(dst, t.Content, 0o644); err != nil {
				return err
			}
			files[rel(primary, dst)] = hash(t.Content)
		}
	}
	if err := copyTree(filepath.Join(o.Source, "skills"), filepath.Join(primary, ".agents", "skills"), files, primary); err != nil {
		return err
	}
	gate, err := harness.RunGate(catalog)
	if err != nil {
		return err
	}
	agents := []byte("<!-- docket:dispatch:start (managed by docket — do not hand-edit) -->\n" + harness.CodexDispatchInterior(gate) + "<!-- docket:dispatch:end -->\n")
	if err := writeFile(filepath.Join(primary, "AGENTS.md"), agents, 0o644); err != nil {
		return err
	}
	files["AGENTS.md"] = hash(agents)
	for path, body := range map[string][]byte{
		"go.mod":                []byte("module example.invalid/nativefixture\n\ngo 1.25\n"),
		"fixture/value.go":      []byte("package fixture\n\nfunc Value() int { return 1 }\n"),
		"fixture/value_test.go": []byte("package fixture\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) { if Value() != 1 { t.Fatal(Value()) } }\n"),
	} {
		if err := writeFile(filepath.Join(primary, path), body, 0o644); err != nil {
			return err
		}
		files[path] = hash(body)
	}
	if err := run(primary, "git", "add", "AGENTS.md", ".codex", ".agents", ".gitignore", "go.mod", "fixture"); err != nil {
		return err
	}
	if err := run(primary, "git", "commit", "-m", "fixture: add candidate assets and baseline"); err != nil {
		return err
	}
	if err := run(primary, "git", "push", "origin", "main"); err != nil {
		return err
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
	recordVersion, err := gitOut(primary, "rev-parse", "docket:"+created.Path)
	if err != nil {
		return err
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
	launch := []byte("# Candidate launch\n\nOpen this disposable primary in a fresh Codex app session. Verify project agent/resource loading and use the absolute candidate executable `" + o.Binary + "` for every catalog. Stop with `codex-candidate-loading-unverified` before substantive work if parent or child provenance cannot be established. Do not modify global installations.\n")
	if err := writeFile(filepath.Join(o.Destination, "LAUNCH.md"), launch, 0o644); err != nil {
		return err
	}
	files["../LAUNCH.md"] = hash(launch)
	bb, err := os.ReadFile(o.Binary)
	if err != nil {
		return err
	}
	m := manifest{SchemaVersion: 1, SourceCommit: head, Binary: o.Binary, BinarySHA256: hash(bb), Files: files, EvidenceAuditComplete: false, ChangeID: created.ID, ChangePath: created.Path, MetadataRevision: groomed.Revision}
	mb, _ := json.MarshalIndent(m, "", "  ")
	mb = append(mb, '\n')
	return writeFile(filepath.Join(o.Destination, "manifest.json"), mb, 0o644)
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
func copyTree(src, dst string, files map[string]string, root string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		r, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, r)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe skill resource %s", p)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		b, err := io.ReadAll(io.LimitReader(in, 8<<20+1))
		if err != nil {
			return err
		}
		if len(b) > 8<<20 {
			return fmt.Errorf("skill resource too large: %s", p)
		}
		if err := writeFile(out, b, info.Mode().Perm()); err != nil {
			return err
		}
		files[rel(root, out)] = hash(b)
		return nil
	})
}
