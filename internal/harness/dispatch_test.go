package harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/assets"
)

func TestRunGateFromEmbedded(t *testing.T) {
	c := embeddedCatalog(t)
	got, err := RunGate(c)
	if err != nil {
		t.Fatalf("RunGate: %v", err)
	}
	want, err := c.Bytes("cursor-rules/run-gate.md")
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("RunGate returned something other than the authored payload")
	}
}

func TestRunGateMissing(t *testing.T) {
	c := syntheticCatalog(map[string]string{
		"cursor-rules/dispatch.head.md": "head\n",
	}, assets.RoleDispatch)
	if _, err := RunGate(c); err == nil {
		t.Fatalf("RunGate accepted a bundle with no run-gate payload")
	} else if !strings.Contains(err.Error(), RunGateAsset) {
		t.Fatalf("error %q does not name %q", err, RunGateAsset)
	}
}

// A dispatch payload the catalog lists but cannot serve is an error, not an
// empty gate: the interior would otherwise ship without the section it exists
// to carry.
func TestRunGateUnreadable(t *testing.T) {
	c := syntheticCatalog(map[string]string{"cursor-rules/run-gate.md": "gate\n"}, assets.RoleDispatch)
	broken := assets.NewCatalog(c.Manifest, func(string) ([]byte, error) {
		return nil, errUnreadable
	})
	if _, err := RunGate(broken); err == nil {
		t.Fatalf("RunGate accepted an unreadable payload")
	}
}

var errUnreadable = errors.New("unreadable payload")

func TestDispatchInterior(t *testing.T) {
	sources := []AgentSource{
		{ShortName: "alpha", Name: "docket-alpha", Description: "Alpha does things."},
		{ShortName: "zeta", Name: "docket-zeta", Description: "Zeta does other things."},
	}
	const gate = "## Run gate — verify a dispatched implement-next run before you relay it\n\nRead git.\n\n\n"

	got := DispatchInterior(sources, []byte(gate))

	if !strings.HasPrefix(got, DispatchHeading+"\n") {
		t.Errorf("interior does not open with the heading: %.60q", got)
	}
	for _, s := range sources {
		bullet := "- **" + s.Name + "** — " + s.Description + " Delegate to the `" + s.Name + "` agent.\n"
		if !strings.Contains(got, bullet) {
			t.Errorf("interior is missing the bullet for %s", s.Name)
		}
	}
	// The roster follows the preamble and the gate follows both: order is
	// structure in a headed markdown document.
	preambleAt := strings.Index(got, "Docket generates an agent definition")
	rosterAt := strings.Index(got, "- **docket-alpha**")
	gateAt := strings.Index(got, "## Run gate")
	if !(preambleAt < rosterAt && rosterAt < gateAt) {
		t.Errorf("sections out of order: preamble %d, roster %d, gate %d", preambleAt, rosterAt, gateAt)
	}
	if strings.Index(got, "- **docket-alpha**") > strings.Index(got, "- **docket-zeta**") {
		t.Errorf("bullets do not follow the inventory order")
	}
	// Exactly one trailing newline, whatever the payload carried.
	if !strings.HasSuffix(got, "Read git.\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("interior does not end with exactly one newline: %q", got[len(got)-12:])
	}
	// Shared by three harnesses, so it may name none of them.
	for _, token := range []string{"claude", "codex", "cursor", "opencode", "runner"} {
		if strings.Contains(strings.ToLower(got), token) {
			t.Errorf("the shared dispatch interior names the harness token %q", token)
		}
	}
}

func TestDispatchInteriorEmptyInventory(t *testing.T) {
	got := DispatchInterior(nil, []byte("## Run gate\n"))
	if strings.Contains(got, "- **") {
		t.Errorf("an empty inventory rendered a bullet: %q", got)
	}
	if !strings.Contains(got, "## Run gate") {
		t.Errorf("an empty inventory dropped the run gate")
	}
}
