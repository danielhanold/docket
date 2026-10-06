package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
	"github.com/danielhanold/docket/internal/buildinfo"
	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/testsupport"
)

// The retirement matrix (change 0351): for each historical global dispatch
// destination, PlanGlobalRetirements returns a proven removal, an unprovable
// conflict, a clean nothing, or a filesystem error — the three probe outcomes
// plus the proof-gated decision. Ownership is proved by BYTES (the prior record
// digest or the frozen legacy reproducer), never by markers, and an unreadable
// destination refuses the run rather than passing as "absent".

// retireRoots is a minimal user-root set anchored at home, enough for the
// retirement probe and for a removal transaction's journal directory.
func retireRoots(home string) UserRoots {
	return UserRoots{
		Home:       home,
		DataRoot:   filepath.Join(home, ".local", "share", "docket"),
		ConfigHome: filepath.Join(home, ".config"),
		BinDir:     filepath.Join(home, ".local", "bin"),
	}
}

// mbTarget is a managed-block dispatch destination — the shape claude/codex/
// opencode's GlobalDispatchTarget returns.
func mbTarget(path string) Target {
	return Target{Path: path, Kind: KindManagedBlock, BlockName: "dispatch", Role: "dispatch"}
}

// fileTarget is a whole-file dispatch destination — the shape cursor's
// GlobalDispatchTarget returns.
func fileTarget(path string) Target {
	return Target{Path: path, Kind: KindFile, Role: "dispatch"}
}

// blockRecord is the prior installation record for a managed dispatch block: the
// interior digest is the ownership proof a later run checks against disk.
func blockRecord(path, interior, harness string) TargetRecord {
	return TargetRecord{
		Path:      filepath.Clean(path),
		Kind:      KindManagedBlock,
		BlockName: "dispatch",
		SHA256:    interiorDigest([]byte(interior)),
		Role:      "dispatch",
		Harness:   harness,
	}
}

// fileRecord is the prior installation record for a whole-file dispatch rule.
func fileRecord(path, content, harness string) TargetRecord {
	return TargetRecord{
		Path:    filepath.Clean(path),
		Kind:    KindFile,
		SHA256:  hashBytes([]byte(content)),
		Role:    "dispatch",
		Harness: harness,
	}
}

// legacyStub reproduces a frozen legacy block interior (for any dispatch managed
// block) and/or whole-file bytes keyed by path, mirroring the production
// reproducer's two shapes without depending on the embedded corpus.
func legacyStub(blockInterior string, fileBytes map[string]string) LegacyReproducer {
	return func(t Target) ([]byte, bool) {
		if t.Kind == KindManagedBlock && t.BlockName == "dispatch" && blockInterior != "" {
			return []byte(blockInterior), true
		}
		if t.Kind == KindFile {
			if b, ok := fileBytes[filepath.Clean(t.Path)]; ok {
				return []byte(b), true
			}
		}
		return nil, false
	}
}

func onlyRemoval(t *testing.T, removals []TargetRecord, conflicts []Inspection, err error) TargetRecord {
	t.Helper()
	if err != nil {
		t.Fatalf("PlanGlobalRetirements errored: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d: %+v", len(conflicts), conflicts)
	}
	if len(removals) != 1 {
		t.Fatalf("expected exactly one removal, got %d: %+v", len(removals), removals)
	}
	return removals[0]
}

func onlyConflict(t *testing.T, removals []TargetRecord, conflicts []Inspection, err error) Inspection {
	t.Helper()
	if err != nil {
		t.Fatalf("PlanGlobalRetirements errored: %v", err)
	}
	if len(removals) != 0 {
		t.Fatalf("expected no removals, got %d: %+v", len(removals), removals)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected exactly one conflict, got %d: %+v", len(conflicts), conflicts)
	}
	return conflicts[0]
}

// --- managed-block matrix -----------------------------------------------------

func TestRetireManagedBlockUnchangedPriorRecord(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	interior := "compact routing rule\n"
	writeFileOrDie(t, path, managedFile(interior))

	prior := priorWith(blockRecord(path, interior, "claude"))
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, prior, nil)
	rec := onlyRemoval(t, removals, conflicts, err)
	if rec.Path != filepath.Clean(path) || rec.Kind != KindManagedBlock || rec.BlockName != "dispatch" {
		t.Errorf("removal record = %+v", rec)
	}
	if rec.Harness != "claude" {
		t.Errorf("removal harness = %q, want claude", rec.Harness)
	}
}

func TestRetireManagedBlockLegacyWithProsePreserved(t *testing.T) {
	home := testsupport.TempDir(t)
	roots := retireRoots(home)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	legacyInterior := "frozen legacy dispatch body\n"
	original := managedFile(legacyInterior)
	writeFileOrDie(t, path, original)

	// No prior record: ownership rests entirely on the frozen legacy reproducer.
	legacy := legacyStub(legacyInterior, nil)
	removals, conflicts, err := PlanGlobalRetirements(roots, []Target{mbTarget(path)}, nil, legacy)
	rec := onlyRemoval(t, removals, conflicts, err)

	// Execute the removal through the real journaled transaction and prove the
	// surrounding user prose survives byte-for-byte — only the block's own
	// marker-to-marker lines are gone.
	txn, err := BeginTxnWithRemovals(RealFS{}, roots, nil, []TargetRecord{rec})
	if err != nil {
		t.Fatalf("BeginTxnWithRemovals: %v", err)
	}
	if err := txn.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	const want = "# Notes\n\nuser prose above\n\n\nuser prose below\n"
	if string(got) != want {
		t.Errorf("prose not preserved after retirement:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestRetireManagedBlockEditedInteriorConflicts(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	onDisk := managedFile("a user hand-edited this block\n")
	writeFileOrDie(t, path, onDisk)

	// The prior record and the legacy reproducer both describe DIFFERENT bytes, so
	// neither proof matches what is on disk.
	prior := priorWith(blockRecord(path, "the recorded interior\n", "claude"))
	legacy := legacyStub("a different frozen interior\n", nil)

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, prior, legacy)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
	if insp.Remedy == "" {
		t.Errorf("conflict carries no remedy")
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != onDisk {
		t.Errorf("edited block file was disturbed: err=%v", err)
	}
}

func TestRetireManagedBlockMalformedMarkersConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	// A dangling start marker with no matching end: an unbounded range document
	// Parse refuses, so retirement must refuse rather than consume to EOF.
	onDisk := "# mine\n\n<!-- docket:dispatch:start (managed by docket) -->\nstuff\n"
	writeFileOrDie(t, path, onDisk)

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonManagedBlockInvalid {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonManagedBlockInvalid)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != onDisk {
		t.Errorf("malformed-marker file was disturbed: err=%v", err)
	}
}

func TestRetireManagedBlockForeignKindConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	// A directory where the managed-block file belongs is a foreign kind.
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
}

func TestRetireManagedBlockEscapingSymlinkConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	// A symlink where a managed-block file belongs. A block can never be installed
	// through a link, so this is a conflict rather than a retirable block.
	symlinkOrDie(t, filepath.Join(home, "elsewhere.md"), path)

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
}

func TestRetireManagedBlockProbeErrorRefuses(t *testing.T) {
	home := testsupport.TempDir(t)
	parent := filepath.Join(home, ".claude")
	path := filepath.Join(parent, "CLAUDE.md")
	writeFileOrDie(t, path, managedFile("body\n"))
	// Strip search permission from the parent so Lstat of the child fails with a
	// permission error — the "unknown" probe outcome, which must refuse the run,
	// not be mistaken for "absent". Restored on cleanup so t.TempDir can remove it.
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o755) })

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, nil, nil)
	if err == nil {
		t.Fatalf("probe error did not refuse the run: removals=%v conflicts=%v", removals, conflicts)
	}
	if len(removals) != 0 || len(conflicts) != 0 {
		t.Errorf("a refused run still produced work: removals=%v conflicts=%v", removals, conflicts)
	}
}

func TestRetireManagedBlockCleanlyAbsentNothing(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md") // never created
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, nil, nil)
	if err != nil || len(removals) != 0 || len(conflicts) != 0 {
		t.Errorf("absent destination is not a clean no-op: removals=%v conflicts=%v err=%v", removals, conflicts, err)
	}
}

func TestRetireManagedBlockAlreadyRetiredNothing(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".claude", "CLAUDE.md")
	// The file survives but the block is gone: a prior run already retired it.
	writeFileOrDie(t, path, "# Notes\n\njust user prose, no block\n")
	prior := priorWith(blockRecord(path, "whatever\n", "claude"))
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{mbTarget(path)}, prior, nil)
	if err != nil || len(removals) != 0 || len(conflicts) != 0 {
		t.Errorf("already-retired block is not a clean no-op: removals=%v conflicts=%v err=%v", removals, conflicts, err)
	}
}

// --- cursor whole-file matrix -------------------------------------------------

func TestRetireCursorFilePriorRecord(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	content := "---\nalwaysApply: true\n---\ndocket dispatch rule\n"
	writeFileOrDie(t, path, content)
	prior := priorWith(fileRecord(path, content, "cursor"))

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, prior, nil)
	rec := onlyRemoval(t, removals, conflicts, err)
	if rec.Kind != KindFile || rec.Harness != "cursor" {
		t.Errorf("removal record = %+v", rec)
	}
}

func TestRetireCursorFileLegacyBytes(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	content := "---\nalwaysApply: true\n---\nfrozen legacy cursor rule\n"
	writeFileOrDie(t, path, content)
	legacy := legacyStub("", map[string]string{filepath.Clean(path): content})

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, nil, legacy)
	rec := onlyRemoval(t, removals, conflicts, err)
	if rec.Kind != KindFile {
		t.Errorf("removal record = %+v", rec)
	}
}

func TestRetireCursorFileEditedConflicts(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	onDisk := "the user rewrote this rule\n"
	writeFileOrDie(t, path, onDisk)
	prior := priorWith(fileRecord(path, "the recorded rule\n", "cursor"))
	legacy := legacyStub("", map[string]string{filepath.Clean(path): "a different frozen rule\n"})

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, prior, legacy)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != onDisk {
		t.Errorf("edited cursor rule was disturbed: err=%v", err)
	}
}

func TestRetireCursorFileForeignKindConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
}

func TestRetireCursorFileSymlinkConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	symlinkOrDie(t, filepath.Join(home, "elsewhere.mdc"), path)
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
}

func TestRetireCursorFileCleanlyAbsentNothing(t *testing.T) {
	home := testsupport.TempDir(t)
	path := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc") // never created
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{fileTarget(path)}, nil, nil)
	if err != nil || len(removals) != 0 || len(conflicts) != 0 {
		t.Errorf("absent cursor rule is not a clean no-op: removals=%v conflicts=%v err=%v", removals, conflicts, err)
	}
}

// --- shared-link (CLAUDE.md → AGENTS.md) matrix -------------------------------

// symlinkTarget is a shared-link dispatch destination — the shape reposeed.Plan
// emits for a CLAUDE.md that shares codex/opencode's AGENTS.md. LinkTarget is the
// absolute destination the historical target carries, so a removal record can
// name the link it retires.
func symlinkTarget(path, linkTarget string) Target {
	return Target{Path: path, Kind: KindSymlink, LinkTarget: linkTarget, Role: "dispatch"}
}

// linkRecord is the prior installation record for a shared dispatch link: the
// recorded destination is the ownership proof a later run checks against disk.
func linkRecord(path, linkTarget, harness string) TargetRecord {
	return TargetRecord{
		Path:       filepath.Clean(path),
		Kind:       KindSymlink,
		LinkTarget: filepath.Clean(linkTarget),
		Role:       "dispatch",
		Harness:    harness,
	}
}

func TestRetireSharedLinkPriorRecord(t *testing.T) {
	home := testsupport.TempDir(t)
	agents := filepath.Join(home, "AGENTS.md")
	writeFileOrDie(t, agents, "shared agents surface\n")
	link := filepath.Join(home, "CLAUDE.md")
	symlinkOrDie(t, agents, link)

	prior := priorWith(linkRecord(link, agents, "claude"))
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{symlinkTarget(link, agents)}, prior, nil)
	rec := onlyRemoval(t, removals, conflicts, err)
	if rec.Path != filepath.Clean(link) || rec.Kind != KindSymlink {
		t.Errorf("removal record = %+v", rec)
	}
	if rec.Harness != "claude" {
		t.Errorf("removal harness = %q, want claude", rec.Harness)
	}
	if rec.LinkTarget != filepath.Clean(agents) {
		t.Errorf("removal link target = %q, want %q", rec.LinkTarget, filepath.Clean(agents))
	}
	// Retirement plans; the transaction removes. The link is still on disk.
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("retirement disturbed the link before the transaction: %v", err)
	}
}

func TestRetireSharedLinkRetargetedConflicts(t *testing.T) {
	home := testsupport.TempDir(t)
	agents := filepath.Join(home, "AGENTS.md")
	writeFileOrDie(t, agents, "shared agents surface\n")
	// The user re-pointed CLAUDE.md at their own notes; it no longer matches the
	// recorded destination, so ownership cannot be proved.
	elsewhere := filepath.Join(home, "MY-NOTES.md")
	writeFileOrDie(t, elsewhere, "my own notes\n")
	link := filepath.Join(home, "CLAUDE.md")
	symlinkOrDie(t, elsewhere, link)

	prior := priorWith(linkRecord(link, agents, "claude"))
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{symlinkTarget(link, agents)}, prior, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
	if insp.Remedy == "" {
		t.Errorf("conflict carries no remedy")
	}
	// The user's retargeted link is untouched.
	if dest, err := os.Readlink(link); err != nil || dest != elsewhere {
		t.Errorf("retargeted link was disturbed: dest=%q err=%v", dest, err)
	}
}

func TestRetireSharedLinkForeignKindConflict(t *testing.T) {
	home := testsupport.TempDir(t)
	agents := filepath.Join(home, "AGENTS.md")
	link := filepath.Join(home, "CLAUDE.md")
	// A regular file where the shared link belongs is a foreign kind — never
	// deleted, even with a prior record naming the link.
	onDisk := "a hand-written CLAUDE.md\n"
	writeFileOrDie(t, link, onDisk)

	prior := priorWith(linkRecord(link, agents, "claude"))
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{symlinkTarget(link, agents)}, prior, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
	if got, err := os.ReadFile(link); err != nil || string(got) != onDisk {
		t.Errorf("foreign file was disturbed: err=%v", err)
	}
}

func TestRetireSharedLinkNoRecordConflicts(t *testing.T) {
	home := testsupport.TempDir(t)
	agents := filepath.Join(home, "AGENTS.md")
	writeFileOrDie(t, agents, "shared agents surface\n")
	link := filepath.Join(home, "CLAUDE.md")
	symlinkOrDie(t, agents, link)

	// No prior record and no legacy reproducer: a link even to the right place is
	// unprovable, so it is a conflict, never a blind delete.
	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home), []Target{symlinkTarget(link, agents)}, nil, nil)
	insp := onlyConflict(t, removals, conflicts, err)
	if insp.Reason != ReasonOwnershipConflict {
		t.Errorf("conflict reason = %q, want %q", insp.Reason, ReasonOwnershipConflict)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Errorf("unprovable link was deleted: %v", err)
	}
}

// One refusal collects every conflict: several unprovable destinations in one
// call all surface, so an operator remedies them in a single pass.
func TestRetireCollectsAllConflicts(t *testing.T) {
	home := testsupport.TempDir(t)
	block := filepath.Join(home, ".claude", "CLAUDE.md")
	writeFileOrDie(t, block, managedFile("edited\n"))
	rule := filepath.Join(home, ".cursor", "rules", "docket-dispatch.mdc")
	writeFileOrDie(t, rule, "edited rule\n")

	removals, conflicts, err := PlanGlobalRetirements(retireRoots(home),
		[]Target{mbTarget(block), fileTarget(rule)}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removals) != 0 {
		t.Errorf("expected no removals, got %v", removals)
	}
	if len(conflicts) != 2 {
		t.Errorf("expected both conflicts collected, got %d: %+v", len(conflicts), conflicts)
	}
}

// --- retiring one block and writing another in the same file ----------------

// pointerBlockName is the neutral-namespace block the codex planner writes into
// the same file that still carries a leftover docket:dispatch block.
const pointerBlockName = "dckt:private-instructions"

// leftoverCodexWorld seeds <home>/.codex/AGENTS.md with the user's prose around
// a docket:dispatch block whose interior the prior state records, and returns
// install options whose codex planner writes a pointer block into that same file
// (plus any extra targets). The planner's GlobalDispatchTarget names the
// leftover, so one install must retire it and add the pointer together.
func leftoverCodexWorld(t *testing.T, extra ...Target) (opts Options, path, original string) {
	t.Helper()
	return leftoverCodexWorldAt(t, versionRoots(t), extra...)
}

// leftoverCodexWorldAt is leftoverCodexWorld over roots the caller already
// holds, so an extra target can be planned under the same home.
func leftoverCodexWorldAt(t *testing.T, roots UserRoots, extra ...Target) (opts Options, path, original string) {
	t.Helper()
	path = filepath.Join(roots.Home, ".codex", "AGENTS.md")
	interior := "leftover dispatch rules\n"
	original = "# mine\n\n" +
		"<!-- docket:dispatch:start (managed by docket) -->\n" + interior + "<!-- docket:dispatch:end -->\n" +
		"tail\n"
	writeFileOrDie(t, path, original)

	payload := samplePayload()
	manifest := sampleManifest(t, payload)
	prior := &State{FormatVersion: StateFormatVersion, ProductVersion: "v-prior", AssetProtocol: assets.AssetProtocol,
		AssetSetID: manifest.AssetSetID, Mode: ModeRelease, Harnesses: []string{"codex"},
		AgentDigest: "sha256:agents", Targets: []TargetRecord{blockRecord(path, interior, "codex")}}
	if err := WriteStateAtomic(roots.StatePath(), prior); err != nil {
		t.Fatal(err)
	}

	opts = Options{
		Roots: roots, FS: RealFS{}, Config: &config.Snapshot{},
		Catalog:   assets.NewCatalog(manifest, openFrom(payload)),
		Info:      buildinfo.Info{Version: "retire-and-write"},
		Harnesses: []string{"codex"},
		Planners: []Planner{{
			Name: "codex",
			Plan: func(Mode, string, assets.Catalog) ([]Target, error) {
				return append([]Target{{
					Path: path, Kind: KindManagedBlock, BlockName: pointerBlockName,
					Annotation: "managed — do not hand-edit", Content: []byte("pointer"), Role: "trigger",
				}}, extra...), nil
			},
			GlobalDispatchTarget: func(r UserRoots) Target {
				return mbTarget(filepath.Join(r.Home, ".codex", "AGENTS.md"))
			},
		}},
	}
	return opts, path, original
}

func TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile(t *testing.T) {
	opts, path, _ := leftoverCodexWorld(t)

	out := Install(opts)
	if out.Err != nil {
		t.Fatalf("Install: %v (reason %s, actions %+v)", out.Err, out.Reason, out.Actions)
	}

	got := readOrDie(t, path)
	doc, err := document.Parse([]byte(got))
	if err != nil {
		t.Fatalf("parsing the installed file: %v\n%s", err, got)
	}
	if _, ok := doc.Block("dispatch"); ok || strings.Contains(got, "docket:dispatch") {
		t.Errorf("the leftover docket:dispatch block survived:\n%s", got)
	}
	block, ok := doc.Block(pointerBlockName)
	if !ok {
		t.Fatalf("no %s block was written:\n%s", pointerBlockName, got)
	}
	if interior := got[block.Interior.Start:block.Interior.End]; interior != "pointer\n" {
		t.Errorf("pointer interior = %q, want %q", interior, "pointer\n")
	}
	if n := strings.Count(got, "<!-- dckt:private-instructions:start"); n != 1 {
		t.Errorf("found %d pointer blocks, want 1:\n%s", n, got)
	}
	for _, kept := range []string{"# mine\n", "tail\n"} {
		if !strings.Contains(got, kept) {
			t.Errorf("the user's %q did not survive:\n%s", kept, got)
		}
	}

	var removes, updates int
	for _, a := range out.Actions {
		if a.Path != path {
			continue
		}
		switch a.Op {
		case OpRemove:
			removes++
		case OpUpdate:
			updates++
		}
	}
	if removes != 1 || updates != 1 {
		t.Errorf("actions on %s: %d remove, %d update; want one of each: %+v", path, removes, updates, out.Actions)
	}

	state, err := LoadState(opts.Roots.StatePath())
	if err != nil || state == nil {
		t.Fatalf("LoadState = %v, %v", state, err)
	}
	var records []TargetRecord
	for _, rec := range state.Targets {
		if filepath.Clean(rec.Path) == path {
			records = append(records, rec)
		}
	}
	if len(records) != 1 || records[0].BlockName != pointerBlockName {
		t.Errorf("state records for %s = %+v, want exactly the %s record", path, records, pointerBlockName)
	}
}

func TestInstallRetiresLeftoverBlockAndWritesPointerInOneFileRollsBackBothSteps(t *testing.T) {
	roots := versionRoots(t)
	later := filepath.Join(roots.Home, ".codex", "zz.txt")
	// The later-sorting whole file is planned against the same home the world
	// builds, so it is created after both AGENTS.md steps have run.
	opts, path, original := leftoverCodexWorldAt(t, roots, Target{
		Path: later, Kind: KindFile, Content: []byte("later\n"), Role: "agent",
	})

	agentsRenames := 0
	ifs := &injectFS{inner: RealFS{}, fail: func(op, p string) error {
		if op != "Rename" {
			return nil
		}
		switch p {
		case path:
			agentsRenames++
		case later:
			return fmt.Errorf("injected failure publishing %s", p)
		}
		return nil
	}}
	opts.FS = ifs

	out := Install(opts)
	if out.Err == nil {
		t.Fatalf("Install succeeded despite the injected failure: %+v", out.Actions)
	}
	// Both AGENTS.md steps published before the failure (the retirement, then the
	// pointer) and the rollback published both restores (newest first).
	if agentsRenames != 4 {
		t.Errorf("AGENTS.md was published %d times, want 2 applies + 2 restores", agentsRenames)
	}
	if got := readOrDie(t, path); got != original {
		t.Errorf("AGENTS.md after rollback =\n%q\nwant\n%q", got, original)
	}
	if _, err := os.Lstat(later); !os.IsNotExist(err) {
		t.Errorf("%s survived the rollback: %v", later, err)
	}
	assertNoStaging(t, filepath.Dir(path))
	if _, found, err := DetectRecovery(roots); err != nil || found {
		t.Errorf("DetectRecovery after rollback = (found %v, err %v), want (false, nil)", found, err)
	}
}

// --- block remedies spell the block's own marker namespace ------------------

func TestBlockRemediesSpellTheMarkerNamespace(t *testing.T) {
	// The docket:dispatch wording is pinned byte-for-byte: it is what every
	// existing conflict report says.
	pinned := map[string]string{
		remedyBlockMarkers("dispatch"): "the docket:dispatch markers in this file are malformed, unbalanced, or out of order; " +
			"repair them by hand, then re-run",
		remedyForeignBlock("dispatch"): "the docket:dispatch block in this file holds content docket did not write; " +
			"delete the block, or move the file aside, then re-run",
		remedyDriftedBlock("dispatch"): "the docket:dispatch block no longer matches the recorded install, " +
			"so docket cannot prove it may rewrite it; restore its recorded content, or delete the block, then re-run",
	}
	for got, want := range pinned {
		if got != want {
			t.Errorf("remedy = %q, want %q", got, want)
		}
	}
	for _, remedy := range []string{
		remedyBlockMarkers(pointerBlockName),
		remedyForeignBlock(pointerBlockName),
		remedyDriftedBlock(pointerBlockName),
	} {
		if !strings.HasPrefix(remedy, "the "+pointerBlockName+" ") || strings.Contains(remedy, "docket:dckt:") {
			t.Errorf("a neutral block's remedy misspells its markers: %q", remedy)
		}
	}
}
