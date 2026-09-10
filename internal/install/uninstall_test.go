package install

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
)

var uninstallHarnesses = []string{"claude", "codex", "cursor", "opencode"}

type uninstallFixture struct {
	roots UserRoots
	state *State
	paths map[string]string
}

func newUninstallFixture(t *testing.T) uninstallFixture {
	t.Helper()
	roots := versionRoots(t)
	payload := samplePayload()
	manifest := sampleManifest(t, payload)
	if _, _, err := EnsureVersionTree(roots, manifest, openFrom(payload)); err != nil {
		t.Fatal(err)
	}
	paths := map[string]string{}
	records := make([]TargetRecord, 0, 5)
	for _, harness := range uninstallHarnesses {
		path := filepath.Join(roots.Home, "."+harness, "agents", "docket.md")
		body := harness + "\n"
		writeFileOrDie(t, path, body)
		paths[harness] = path
		records = append(records, TargetRecord{Path: path, Kind: KindFile, SHA256: hashBytes([]byte(body)), Role: "agent", Harness: harness})
	}
	binary := filepath.Join(roots.BinDir, "docket")
	writeFileOrDie(t, binary, "binary\n")
	records = append(records, TargetRecord{Path: binary, Kind: KindFile, SHA256: hashBytes([]byte("binary\n")), Role: roleBinary})
	state := &State{FormatVersion: StateFormatVersion, ProductVersion: "v-test", AssetProtocol: 1,
		AssetSetID: manifest.AssetSetID, Mode: ModeRelease, Harnesses: append([]string(nil), uninstallHarnesses...),
		AgentDigest: "sha256:agents", Targets: records}
	if err := WriteStateAtomic(roots.StatePath(), state); err != nil {
		t.Fatal(err)
	}
	// A normal install leaves the flock carrier behind. Dry-run may observe it,
	// but must never create it itself.
	lock, err := acquireInstallLock(roots)
	if err != nil {
		t.Fatal(err)
	}
	lock.release()
	return uninstallFixture{roots: roots, state: state, paths: paths}
}

func uninstallOptions(f uninstallFixture, filters ...string) UninstallOptions {
	return UninstallOptions{Roots: f.roots, FS: RealFS{}, Harnesses: filters, SupportedHarnesses: uninstallHarnesses}
}

func TestUninstallAllAndScopedHarnesses(t *testing.T) {
	t.Run("all", func(t *testing.T) {
		f := newUninstallFixture(t)
		out := Uninstall(uninstallOptions(f))
		if out.Err != nil || !out.Applied {
			t.Fatalf("Uninstall = %#v", out)
		}
		for _, harness := range uninstallHarnesses {
			if _, err := os.Lstat(f.paths[harness]); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("%s target remains: %v", harness, err)
			}
		}
		binary := filepath.Join(f.roots.BinDir, "docket")
		if got := readOrDie(t, binary); got != "binary\n" {
			t.Fatalf("unattributed binary = %q", got)
		}
		state, err := LoadState(f.roots.StatePath())
		if err != nil || len(state.Harnesses) != 0 || state.AssetSetID != "" || len(state.Targets) != 1 {
			t.Fatalf("empty published state = %#v, %v", state, err)
		}
		if _, err := os.Lstat(filepath.Dir(f.roots.VersionDir(f.state.AssetSetID))); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("unreferenced version tree remains: %v", err)
		}
	})

	t.Run("selected duplicate filter", func(t *testing.T) {
		f := newUninstallFixture(t)
		out := Uninstall(uninstallOptions(f, "codex", "codex"))
		if out.Err != nil || !out.Applied {
			t.Fatalf("Uninstall = %#v; err=%v", out, out.Err)
		}
		if _, err := os.Lstat(f.paths["codex"]); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("codex remains: %v", err)
		}
		for _, harness := range []string{"claude", "cursor", "opencode"} {
			if _, err := os.Lstat(f.paths[harness]); err != nil {
				t.Errorf("unselected %s changed: %v", harness, err)
			}
		}
		state, _ := LoadState(f.roots.StatePath())
		if !reflect.DeepEqual(state.Harnesses, []string{"claude", "cursor", "opencode"}) {
			t.Fatalf("harnesses = %v", state.Harnesses)
		}
	})
}

func TestUninstallValidatesEveryFilterBeforeMutation(t *testing.T) {
	f := newUninstallFixture(t)
	before := snapshotHome(t, f.roots.Home)
	out := Uninstall(uninstallOptions(f, "claude", "unknown", "codex"))
	if !errors.Is(out.Err, ErrInvalidInput) || out.Reason != ReasonUnknownHarness {
		t.Fatalf("Uninstall = %#v", out)
	}
	if after := snapshotHome(t, f.roots.Home); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid filter mutated the installation")
	}
}

func TestUninstallSupportedUnrecordedIsNoopAndUnknownRecordedIsRemovedByAll(t *testing.T) {
	t.Run("supported unrecorded", func(t *testing.T) {
		f := newUninstallFixture(t)
		out := Uninstall(uninstallOptions(f, "ghost-supported"))
		if out.Err == nil { // not supported yet
			t.Fatal("fixture did not exercise support validation")
		}
		o := uninstallOptions(f, "cursor")
		f.state.Harnesses = []string{"claude", "codex", "opencode"}
		var kept []TargetRecord
		for _, rec := range f.state.Targets {
			if rec.Harness != "cursor" {
				kept = append(kept, rec)
			}
		}
		f.state.Targets = kept
		if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
			t.Fatal(err)
		}
		out = Uninstall(o)
		if out.Err != nil || out.Applied || len(out.Actions) != 0 {
			t.Fatalf("supported unrecorded = %#v", out)
		}
	})

	t.Run("unknown recorded", func(t *testing.T) {
		f := newUninstallFixture(t)
		f.state.Harnesses = append(f.state.Harnesses, "retired-harness")
		path := filepath.Join(f.roots.Home, ".retired", "owned")
		writeFileOrDie(t, path, "old\n")
		f.state.Targets = append(f.state.Targets, TargetRecord{Path: path, Kind: KindFile, SHA256: hashBytes([]byte("old\n")), Role: "agent", Harness: "retired-harness"})
		if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
			t.Fatal(err)
		}
		out := Uninstall(uninstallOptions(f))
		if out.Err != nil {
			t.Fatalf("Uninstall: %v", out.Err)
		}
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("retired target remains: %v", err)
		}
	})
}

func TestUninstallRemovalProofMatrixAndConflictAccumulation(t *testing.T) {
	f := newUninstallFixture(t)
	file := f.paths["claude"]
	writeFileOrDie(t, file, "edited\n")

	linkDest := filepath.Join(f.roots.Home, "missing", "asset")
	link := f.paths["codex"]
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	symlinkOrDie(t, linkDest, link)
	for i := range f.state.Targets {
		if f.state.Targets[i].Harness == "codex" {
			f.state.Targets[i] = TargetRecord{Path: link, Kind: KindSymlink, LinkTarget: filepath.Join(f.roots.Home, "other", "asset"), Role: "skill", Harness: "codex"}
		}
	}

	block := f.paths["cursor"]
	blockBody := managedFile("changed\n")
	writeFileOrDie(t, block, blockBody)
	for i := range f.state.Targets {
		if f.state.Targets[i].Harness == "cursor" {
			f.state.Targets[i] = TargetRecord{Path: block, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: interiorDigest([]byte("recorded\n")), Role: "dispatch", Harness: "cursor"}
		}
	}
	if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
		t.Fatal(err)
	}

	out := Uninstall(uninstallOptions(f))
	if out.Err == nil || out.Applied {
		t.Fatalf("Uninstall = %#v", out)
	}
	var conflicts []string
	for _, action := range out.Actions {
		if action.Op == OpConflict {
			conflicts = append(conflicts, action.Path)
		}
	}
	sort.Strings(conflicts)
	want := []string{file, link, block}
	sort.Strings(want)
	if !reflect.DeepEqual(conflicts, want) {
		t.Fatalf("conflicts = %v, want %v", conflicts, want)
	}
	if _, err := os.Lstat(f.paths["opencode"]); err != nil {
		t.Fatalf("clean target changed: %v", err)
	}
}

func TestUninstallMalformedBlockAndChangedKindConflict(t *testing.T) {
	f := newUninstallFixture(t)
	malformed := f.paths["claude"]
	writeFileOrDie(t, malformed, "<!-- docket:dispatch:start (managed by docket) -->\nunterminated\n")
	changedKind := f.paths["codex"]
	if err := os.Remove(changedKind); err != nil {
		t.Fatal(err)
	}
	symlinkOrDie(t, filepath.Join(f.roots.Home, "dangling"), changedKind)
	for i := range f.state.Targets {
		switch f.state.Targets[i].Harness {
		case "claude":
			f.state.Targets[i] = TargetRecord{Path: malformed, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: interiorDigest([]byte("owned\n")), Role: "dispatch", Harness: "claude"}
		}
	}
	if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
		t.Fatal(err)
	}
	out := Uninstall(uninstallOptions(f, "claude", "codex"))
	if out.Err == nil || len(out.Actions) != 2 || out.Reason != ReasonManagedBlockInvalid {
		t.Fatalf("Uninstall = %#v", out)
	}
}

func TestUninstallSatisfiedAbsenceAndManagedBlockRemoval(t *testing.T) {
	f := newUninstallFixture(t)
	if err := os.Remove(f.paths["claude"]); err != nil {
		t.Fatal(err)
	}
	block := f.paths["codex"]
	body := managedFile("owned\n")
	writeFileOrDie(t, block, body)
	for i := range f.state.Targets {
		if f.state.Targets[i].Harness == "codex" {
			f.state.Targets[i] = TargetRecord{Path: block, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: interiorDigest([]byte("owned\n")), Role: "dispatch", Harness: "codex"}
		}
	}
	// A valid document whose recorded block is already absent is satisfied.
	writeFileOrDie(t, f.paths["cursor"], "# user configuration\n")
	for i := range f.state.Targets {
		if f.state.Targets[i].Harness == "cursor" {
			f.state.Targets[i] = TargetRecord{Path: f.paths["cursor"], Kind: KindManagedBlock, BlockName: "dispatch", SHA256: interiorDigest([]byte("old\n")), Role: "dispatch", Harness: "cursor"}
		}
	}
	if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
		t.Fatal(err)
	}
	out := Uninstall(uninstallOptions(f))
	if out.Err != nil {
		t.Fatalf("Uninstall: %v", out.Err)
	}
	if got := readOrDie(t, block); strings.Contains(got, "docket:dispatch") || !strings.Contains(got, "user prose above") {
		t.Fatalf("block removal = %q", got)
	}
	if got := readOrDie(t, f.paths["cursor"]); got != "# user configuration\n" {
		t.Fatalf("absent block file changed: %q", got)
	}
}

func TestUninstallRetainsUnrecordedConfigurationAndIsIdempotent(t *testing.T) {
	f := newUninstallFixture(t)
	for _, path := range []string{
		filepath.Join(f.roots.Home, ".docket.yml"),
		filepath.Join(f.roots.Home, "src", "README.md"),
		filepath.Join(f.roots.Home, "repo", "AGENTS.md"),
	} {
		writeFileOrDie(t, path, "keep\n")
	}
	first := Uninstall(uninstallOptions(f))
	second := Uninstall(uninstallOptions(f))
	if first.Err != nil || second.Err != nil || !first.Applied || second.Applied || len(second.Actions) != 0 {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
	for _, path := range []string{filepath.Join(f.roots.Home, ".docket.yml"), filepath.Join(f.roots.Home, "src", "README.md"), filepath.Join(f.roots.Home, "repo", "AGENTS.md")} {
		if got := readOrDie(t, path); got != "keep\n" {
			t.Errorf("%s = %q", path, got)
		}
	}
}

func TestUninstallPreservesDevelopmentProvenance(t *testing.T) {
	f := newUninstallFixture(t)
	f.state.Mode = ModeDevelopment
	f.state.SourceRoot = filepath.Join(f.roots.Home, "source")
	f.state.SourceDigest = "sha256:source"
	if err := WriteStateAtomic(f.roots.StatePath(), f.state); err != nil {
		t.Fatal(err)
	}
	out := Uninstall(uninstallOptions(f))
	if out.Err != nil {
		t.Fatal(out.Err)
	}
	got, _ := LoadState(f.roots.StatePath())
	if got.Mode != ModeDevelopment || got.SourceRoot != f.state.SourceRoot || got.SourceDigest != f.state.SourceDigest || got.ProductVersion != f.state.ProductVersion || got.AssetProtocol != f.state.AssetProtocol || got.AgentDigest != f.state.AgentDigest {
		t.Fatalf("provenance changed: %#v", got)
	}
}

func TestUninstallDryRunCreatesNothing(t *testing.T) {
	roots := versionRoots(t)
	if err := os.RemoveAll(roots.DataRoot); err != nil {
		t.Fatal(err)
	}
	out := Uninstall(UninstallOptions{Roots: roots, FS: RealFS{}, SupportedHarnesses: uninstallHarnesses, DryRun: true})
	if out.Err != nil || out.Applied {
		t.Fatalf("dry run = %#v", out)
	}
	if _, err := os.Lstat(roots.DataRoot); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("dry run created data root: %v", err)
	}
}

func TestUninstallDryRunPlansWithoutMutationAndReportsPendingRecovery(t *testing.T) {
	t.Run("plan", func(t *testing.T) {
		f := newUninstallFixture(t)
		before := snapshotHome(t, f.roots.Home)
		o := uninstallOptions(f, "claude")
		o.DryRun = true
		out := Uninstall(o)
		if out.Err != nil || out.Applied || len(out.Actions) != 1 || out.Actions[0].Path != f.paths["claude"] {
			t.Fatalf("dry uninstall = %#v", out)
		}
		if after := snapshotHome(t, f.roots.Home); !reflect.DeepEqual(before, after) {
			t.Fatal("dry uninstall changed the user's home")
		}
	})

	t.Run("pending transaction", func(t *testing.T) {
		f := newUninstallFixture(t)
		path := f.paths["claude"]
		if _, err := BeginTxnWithRemovals(RealFS{}, f.roots, nil, []TargetRecord{{Path: path, Kind: KindFile, SHA256: hashBytes([]byte("claude\n")), Role: "agent", Harness: "claude"}}); err != nil {
			t.Fatal(err)
		}
		o := uninstallOptions(f)
		o.DryRun = true
		out := Uninstall(o)
		if !errors.Is(out.Err, ErrJournalInvalid) || out.Applied || len(out.Actions) != 1 || out.Actions[0].Op != OpRecover {
			t.Fatalf("dry pending uninstall = %#v", out)
		}
		if _, found, err := DetectRecovery(f.roots); err != nil || !found {
			t.Fatalf("dry run consumed recovery: %v %v", found, err)
		}
	})
}

func TestUninstallCollectorFailureDoesNotRollbackCommittedRemoval(t *testing.T) {
	f := newUninstallFixture(t)
	ifs := &injectFS{inner: RealFS{}}
	ifs.fail = func(op, path string) error {
		if op == "Rename" && filepath.Base(path) == "quarantine" && filepath.Base(filepath.Dir(path)) == "collection" {
			return errors.New("collection unavailable")
		}
		return nil
	}
	o := uninstallOptions(f)
	o.FS = ifs
	out := Uninstall(o)
	if out.Err == nil || !out.Applied {
		t.Fatalf("Uninstall = %#v", out)
	}
	for _, path := range f.paths {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("committed removal rolled back at %s: %v", path, err)
		}
	}
	state, err := LoadState(f.roots.StatePath())
	if err != nil || len(state.Harnesses) != 0 {
		t.Fatalf("committed state = %#v, %v", state, err)
	}
}

func TestReinstallEmptyState(t *testing.T) {
	f := newUninstallFixture(t)
	if out := Uninstall(uninstallOptions(f)); out.Err != nil {
		t.Fatal(out.Err)
	}
	payload := samplePayload()
	manifest := sampleManifest(t, payload)
	target := f.paths["claude"]
	opts := Options{
		Roots: f.roots, FS: RealFS{}, Config: &config.Snapshot{},
		Catalog: assets.NewCatalog(manifest, openFrom(payload)),
		Info:    buildinfo.Info{Version: "reinstall"}, AgentDigest: "sha256:new",
		Harnesses: []string{"claude"},
		Planners: []Planner{{
			Name: "claude",
			Plan: func(Mode, string, assets.Catalog) ([]Target, error) {
				return []Target{{Path: target, Kind: KindFile, Content: []byte("reinstalled\n"), Role: "agent"}}, nil
			},
		}},
	}
	out := Install(opts)
	if out.Err != nil || readOrDie(t, target) != "reinstalled\n" {
		t.Fatalf("reinstall = %#v", out)
	}
}
