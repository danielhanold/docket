package cli

// This file pins the CATALOG-to-TREE correspondence against the real production
// Cobra tree (not a synthetic one): every public executable leaf the wiring
// registers must be annotated and become exactly one catalog entry, and every
// catalog entry must resolve to a real, visible, executable command whose id is
// its dotted command path. The two directions are walked independently — the
// forward direction over collectCapabilities' output, the reverse over a
// test-local walker — so neither can hide the other's drift.

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/testsupport"
)

// productionRootForTest returns the assembled production Cobra tree with the
// captureTree scratch command removed. captureTree injects an unannotated
// `treewalk` leaf through run()'s extra-command seam purely to hand the tree
// back to a test; it is not part of the production surface the catalog
// describes, so it is stripped here. Cobra's built-in `help` command stays: the
// production `capabilities` command walks a tree that still carries it, and the
// walker must tolerate it exactly as production does.
func productionRootForTest(t *testing.T) *cobra.Command {
	t.Helper()
	root := captureTree(t)
	for _, c := range root.Commands() {
		if commandKey(c) == treeWalkCommand {
			root.RemoveCommand(c)
		}
	}
	return root
}

// enumeratePublicExecutableLeaves walks the tree DIRECTLY (never through
// collectCapabilities) and returns the command paths of every public executable
// leaf — the reverse-direction population the forward catalog must match one for
// one. A command counts when it is visible, runnable, not Cobra's help
// machinery, and is either annotated (an executable parent such as `install`) or
// has no visible children (an ordinary leaf). A group missing-command stub
// (`change`, `gate`, `gate drive`, …) is runnable but has visible children and
// no annotation, so it is excluded structurally. `help` is excluded by name —
// it is Cobra's own command, the one visible runnable leaf that is not a docket
// operation — deliberately NOT by the walker's exclusion marker, so the two
// directions do not share an exclusion and a real operation mistakenly marked
// machinery would still redden the count.
func enumeratePublicExecutableLeaves(root *cobra.Command) []string {
	var leaves []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			if child.Hidden {
				continue
			}
			if child.Name() == "help" {
				continue
			}
			runnable := child.Run != nil || child.RunE != nil
			_, annotated := child.Annotations[capAnnotationID]
			if runnable && (annotated || !hasVisibleChildren(child)) {
				leaves = append(leaves, child.CommandPath())
			}
			walk(child)
		}
	}
	walk(root)
	return leaves
}

// TestProductionCapabilityCorrespondence is the both-direction gate: it fails
// loudly (naming the command) on any unannotated production leaf, on any catalog
// entry that resolves to no public executable command or whose id is not its
// dotted path, on any leaf/entry count mismatch, on a gross population collapse,
// and on any group or machinery command sneaking in as an entry.
func TestProductionCapabilityCorrespondence(t *testing.T) {
	root := productionRootForTest(t)

	entries, err := collectCapabilities(root)
	if err != nil {
		// Any unannotated public leaf surfaces here, naming the command: this
		// is the walker's fail-closed producer error, and it IS this test doing
		// its job before any correspondence assertion runs.
		t.Fatal(err)
	}

	// Forward: every catalog entry resolves to a real, visible, executable
	// command, and its id is exactly that command's dotted path.
	byID := map[string]CapabilityEntry{}
	for _, e := range entries {
		if len(e.Argv) < 1 {
			t.Errorf("entry %q has empty argv", e.ID)
			continue
		}
		c, _, ferr := root.Find(e.Argv[1:])
		if ferr != nil || c == nil || c.Hidden || (c.RunE == nil && c.Run == nil) {
			t.Errorf("entry %q argv %v resolves to no public executable command", e.ID, e.Argv)
			continue
		}
		if want := operationName(commandKey(c)); e.ID != want {
			t.Errorf("id %q != dotted command path %q — a deliberate rename must update this test", e.ID, want)
		}
		byID[e.ID] = e
	}

	// Reverse: every public executable leaf appears exactly once. The two
	// populations are walked independently, so a leaf dropped by the forward
	// walker but seen by this one (or vice versa) reddens the count.
	leaves := enumeratePublicExecutableLeaves(root)
	if len(leaves) != len(entries) {
		t.Errorf("tree has %d public executable leaves, catalog has %d entries\nleaves: %v",
			len(leaves), len(entries), leaves)
	}

	// Population floor: not a hand-tuned target — computed against the
	// independent walk above; this constant only pins gross collapse (the
	// current tree carries 65 leaves).
	if len(entries) < 60 {
		t.Errorf("catalog population %d below floor 60 — the walker is dropping the tree", len(entries))
	}

	// Absences: the bare root, group stubs, and Cobra machinery must never be
	// catalog entries.
	for _, absent := range []string{"change", "gate", "gate.drive", "diagnostic", "help", "__complete", "completion"} {
		if _, ok := byID[absent]; ok {
			t.Errorf("group/machinery %q must not be a catalog entry", absent)
		}
	}
}

// TestProductionEffectsCompleteAndClosed pins every entry's effect set: at least
// one effect, each drawn from the closed vocabulary, sorted and deduplicated —
// the exact shape the catalog contract promises consumers.
func TestProductionEffectsCompleteAndClosed(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Effects) == 0 {
			t.Errorf("entry %q declares no effects", e.ID)
			continue
		}
		seen := map[string]bool{}
		for _, eff := range e.Effects {
			if !allEffects[Effect(eff)] {
				t.Errorf("entry %q declares effect %q outside the closed vocabulary", e.ID, eff)
			}
			if seen[eff] {
				t.Errorf("entry %q declares effect %q more than once", e.ID, eff)
			}
			seen[eff] = true
		}
		if !sort.SliceIsSorted(e.Effects, func(i, j int) bool { return e.Effects[i] < e.Effects[j] }) {
			t.Errorf("entry %q effects %v are not sorted", e.ID, e.Effects)
		}
	}
	// A read-only sentinel: the read-only bootstrap-adjacent operations must
	// never silently acquire a write effect. `status` is the reference.
	if e, ok := entryByID(entries, "status"); ok {
		if strings.Join(e.Effects, " ") != string(EffectRead) {
			t.Errorf("status effects = %v, want [read] — a read-only operation must not acquire a write effect", e.Effects)
		}
	} else {
		t.Errorf("status is not in the catalog")
	}
	// The preflight leaf declares the full union its composed implementation-scope
	// MaintenanceSweep may perform (metadata-write + local-write + external-write
	// via its closeout/cleanup leg), matching maintenance.sweep; declared >=
	// actual is the soundness contract (change 0397 review).
	wantPreflightEffects := "external-write local-write metadata-write"
	if e, ok := entryByID(entries, "maintenance.preflight"); !ok {
		t.Error("maintenance.preflight absent from the catalog")
	} else if got := strings.Join(e.Effects, " "); got != wantPreflightEffects {
		t.Errorf("maintenance.preflight effects = %v, want [%s]", e.Effects, wantPreflightEffects)
	}
}

func entryByID(entries []CapabilityEntry, id string) (CapabilityEntry, bool) {
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return CapabilityEntry{}, false
}

// assertDirEmpty fails if dir holds any entry — the write-independence oracle:
// the bootstrap must not create a config file, a state dir, or any scratch on
// the paths it is pointed at.
func assertDirEmpty(t *testing.T, dir string) {
	t.Helper()
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %q: %v", dir, err)
	}
	if len(names) != 0 {
		var got []string
		for _, n := range names {
			got = append(got, n.Name())
		}
		t.Fatalf("directory %q is not empty after `capabilities --json`: %v — the bootstrap wrote to disk", dir, got)
	}
}

// TestCapabilitiesIsRepositoryConfigAssetAndWriteIndependent proves the one
// property the whole bootstrap rests on: `capabilities --json` answers with a
// complete protocol document from an empty non-git working directory, with HOME
// and XDG_STATE_HOME pointed at empty temp dirs (no installation, no global
// config), and it writes nothing to any of them. If the runner resolved a git
// client, loaded config, or resolved asset roots eagerly, one of these
// assertions would redden — which is exactly why the capabilities RunE must
// stay free of gitcli.NewClient, config.Load*, and install.ResolveRoots.
func TestCapabilitiesIsRepositoryConfigAssetAndWriteIndependent(t *testing.T) {
	dir := testsupport.TempDir(t)
	home := testsupport.TempDir(t)
	xdg := testsupport.TempDir(t)
	t.Chdir(dir)
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", xdg)

	out, errS, code := runCLI(t, "capabilities", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("capabilities did not answer cleanly from an empty non-git dir: out=%q err=%q code=%d", out, errS, code)
	}

	// A full document, not a partial one.
	var doc struct {
		ProtocolVersion   int               `json:"protocol_version"`
		CapabilityVersion int               `json:"capability_version"`
		Commands          []json.RawMessage `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("catalog is not valid JSON: %v\n%s", err, out)
	}
	if doc.ProtocolVersion != 1 || doc.CapabilityVersion != 1 {
		t.Fatalf("versions: protocol=%d capability=%d", doc.ProtocolVersion, doc.CapabilityVersion)
	}
	if len(doc.Commands) == 0 {
		t.Fatal("catalog carries no commands — the document is not full")
	}

	// Write-independence: every directory it was pointed at stays empty.
	assertDirEmpty(t, dir)
	assertDirEmpty(t, home)
	assertDirEmpty(t, xdg)
}

// TestCapabilitiesPayloadWithinByteBudget is the gating oracle for compactness:
// the emitted catalog must fit the 14336-byte (14 KB) design ceiling, and it
// must carry no human help prose — the catalog is a machine bootstrap, not a
// second copy of --help. Growth past the ceiling is a design event (spec:
// Compactness boundary), never a truncation or per-skill-filter opportunity.
// The ceiling was raised one KB step (12 KB → 13 KB) for change 0397, whose spec
// deliberately adds the `maintenance.preflight` operation to the catalog — the
// conscious design event the guard exists to make visible, not a silent creep.
// It was raised a second one KB step (13 KB → 14 KB) for change 0399, whose spec
// deliberately adds the `schema` operation to the catalog — the identical
// conscious design event. That entry is a one-line invocation stub like every
// other catalog leaf; the schemas themselves are NOT inlined into the catalog
// (they live in the separate `docket schema` op), so this step does not carry
// schemas inline and is not the ceiling raise the spec's non-goal forbids.
func TestCapabilitiesPayloadWithinByteBudget(t *testing.T) {
	out, errS, code := runCLI(t, "capabilities", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	n := len(out)
	t.Logf("capabilities payload: %d bytes (budget 14336)", n)
	if n > 14*1024 {
		t.Fatalf("catalog is %d bytes, over the 14KB design ceiling — growth is a design event (spec: Compactness boundary), not a truncation opportunity", n)
	}
	// Content-exclusion: no help-prose fields. The catalog names signatures and
	// effects, never Short/Long/Example/Help text.
	for _, banned := range []string{`"short"`, `"long"`, `"example"`, `"help"`} {
		if strings.Contains(out, banned) {
			t.Errorf("catalog carries help-prose field %s — the catalog must not restate --help", banned)
		}
	}
}

// TestRepresentativeSignatures pins the exact projected signature for a
// deliberately varied handful of operations — a request-file leaf, a
// multi-repeatable-flag read, two positional-tail shapes, and the
// flags-then-`--`-argv launch. The strings below were MEASURED from the real
// production tree and reviewed for faithfulness before pinning (never
// predicted): they lock both the flag-projection rules (required/optional
// ordering, repeatable `...`, backquoted value hints) and the positional-tail
// composition (a bare `--` separator lands last; an ordinary tail leads). A
// deliberate change to a flag's registration or usage hint must update the
// pinned string here, in the open.
func TestRepresentativeSignatures(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		// request-file leaf: required file, optional repo dir.
		"change.reconcile": "--input <file> [--repo-dir <dir>]",
		// repeatable filters, both sides of the optional repo dir, sorted.
		"status": "[--priority <level>...] [--records] [--repo-dir <dir>] [--type <type>...]",
		// pure positional tail, no flags.
		"gate.observe": "<run-dir>",
		// composition leaf: optional repo dir only (change 0397).
		"maintenance.preflight": "[--repo-dir <dir>]",
		// required flags then the bare `--` separator carrying the argv tail last.
		"gate.launch": "--cwd <dir> --root <dir> -- <argv...>",
		// positional alternation tail leading, optional flags trailing.
		"run.gate-verdict": "<key> | [<id>...] [--repo-dir <dir>] [--unattributed]",
		// change 0359: gate-before gains --resume for explicit resume attribution;
		// the target positional leads, the optional flags trail sorted.
		"run.gate-before": "<target> [--repo-dir <dir>] [--resume <id>]",
		// change 0359: gate-claim redeems a single-use continuation — the two
		// positionals (key, continuation id) lead, the optional repo dir trails.
		"run.gate-claim": "<key> <continuation-id> [--repo-dir <dir>]",
		// change 0359: the config owners run their resolved suite command; the
		// task-intent owner (--owner task) alone takes the focused argv after a bare
		// `--` separator, which lands last.
		"gate.drive.start": "--owner <role> --run-root <dir> [--branch <name>] [--change-id <id>] [--child-cap <token>] [--cwd <dir>] [--env-hash <hash>] [--gate-context <token>] [--idempotent-suite-gate] [--phase <name>] [--ref <ref>] [--repo-dir <dir>] [--scope-id <id>] [--task-id <id>] -- <argv...>",
		// change 0359: recovery-scope preparation (required identity flags) and the
		// event-authorized parent takeover.
		"gate.drive.prepare-scope": "--branch <name> --change-id <id> --phase <name> --task-id <id> --worktree <dir> [--gate-context <token>] [--repo-dir <dir>]",
		"gate.drive.takeover":      "--parent-cap <token> --scope-id <id> [--drive-id <id>] [--repo-dir <dir>]",
	}
	for id, wantSig := range want {
		e, ok := entryByID(entries, id)
		if !ok {
			t.Errorf("representative operation %q is absent from the catalog", id)
			continue
		}
		if e.Signature != wantSig {
			t.Errorf("signature drift for %q:\n  got  %q\n  want %q", id, e.Signature, wantSig)
		}
	}
}
