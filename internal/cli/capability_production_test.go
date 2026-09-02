package cli

// This file pins the CATALOG-to-TREE correspondence against the real production
// Cobra tree (not a synthetic one): every public executable leaf the wiring
// registers must be annotated and become exactly one catalog entry, and every
// catalog entry must resolve to a real, visible, executable command whose id is
// its dotted command path. The two directions are walked independently — the
// forward direction over collectCapabilities' output, the reverse over a
// test-local walker — so neither can hide the other's drift.

import (
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
}

func entryByID(entries []CapabilityEntry, id string) (CapabilityEntry, bool) {
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return CapabilityEntry{}, false
}
