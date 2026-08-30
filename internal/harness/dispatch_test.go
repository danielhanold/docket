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
	const gate = "## Run gate — verify a dispatched implement-next run before you relay it\n\nRead git.\n\n\n"

	got := DispatchInterior([]byte(gate))

	if !strings.HasPrefix(got, DispatchHeading+"\n") {
		t.Errorf("interior does not open with the heading: %.60q", got)
	}
	// The compact routing rule (change 0334): its load-bearing phrases replace
	// the roster it used to enumerate.
	for _, phrase := range []string{
		"registered same-name",
		"authoritative for agent names, descriptions, and availability",
		"do not invent one",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("interior is missing the routing-rule phrase %q", phrase)
		}
	}
	// The routing rule precedes the run gate's heading: order is structure in a
	// headed markdown document.
	ruleAt := strings.Index(got, "registered same-name")
	gateAt := strings.Index(got, "## Run gate")
	if !(ruleAt >= 0 && gateAt >= 0 && ruleAt < gateAt) {
		t.Errorf("sections out of order: rule %d, gate %d", ruleAt, gateAt)
	}
	// The roster is gone. Detect its removal by SHAPE, never a spelling list: no
	// line is a `- **docket-...` bullet, and the delegation clause is absent.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "- **docket-") {
			t.Errorf("interior still carries a roster bullet: %q", line)
		}
	}
	if strings.Contains(got, "Delegate to the") {
		t.Errorf("interior still carries the roster delegation clause")
	}
	// Exactly one trailing newline, whatever the payload carried.
	if !strings.HasSuffix(got, "Read git.\n") || strings.HasSuffix(got, "\n\n") {
		t.Errorf("interior does not end with exactly one newline: %q", got[len(got)-12:])
	}
	// Shared by three harnesses, so it may name none of them. The retired runner
	// path is banned by its full `runner-dispatch` spelling, not the bare word
	// "runner": change 0371's never-fall-back rule legitimately says "shell
	// runner" while forbidding exactly that reroute.
	for _, token := range []string{"claude", "codex", "cursor", "opencode", "runner-dispatch"} {
		if strings.Contains(strings.ToLower(got), token) {
			t.Errorf("the shared dispatch interior names the harness token %q", token)
		}
	}
}

// TestDispatchPreambleStatesNativeOnlyPolicy pins the canonical policy's
// load-bearing clauses (change 0371, spec § Canonical dispatch policy). Asserts
// collapse whitespace so a re-wrap never reddens them, and each phrase is a
// verbatim slice of the claim, not a common noun (learnings:
// assert-detects-removal-not-replacement).
func TestDispatchPreambleStatesNativeOnlyPolicy(t *testing.T) {
	collapse := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	got := collapse(dispatchPreamble)
	for _, phrase := range []string{
		// exact registered same-name identity
		"registered same-name `docket-*` agent",
		// native facility, request forwarded unchanged
		"harness's native named-agent dispatch, and pass the request through unchanged",
		// registry is authoritative; no invented registration
		"native agent registry is authoritative",
		"do not invent one",
		// the never-fall-back rule (NEW in 0371)
		"Never reroute a registered workflow through a shell runner, another harness, a generic agent, or an inline reconstruction of its contract",
		"a missing registration is a visible capability failure, not a fallback trigger",
	} {
		if !strings.Contains(got, phrase) {
			t.Errorf("dispatchPreamble lost the policy clause %q", phrase)
		}
	}
	// The policy must never name or recommend the retired runner path, and must
	// stay machine-neutral (no harness names — the same interior lands on every
	// host surface).
	for _, banned := range []string{"runner-dispatch", "docket.sh", "scripts/runners", "claude", "codex", "cursor", "opencode"} {
		if strings.Contains(strings.ToLower(got), banned) {
			t.Errorf("dispatchPreamble contains banned token %q", banned)
		}
	}
}

// The interior is inventory-independent: it renders the same block whatever the
// bundle carries, because the roster has moved to the harness's own registry.
func TestDispatchInteriorCarriesGate(t *testing.T) {
	got := DispatchInterior([]byte("## Run gate\n"))
	if strings.Contains(got, "- **") {
		t.Errorf("the interior rendered a bullet: %q", got)
	}
	if !strings.Contains(got, "## Run gate") {
		t.Errorf("the interior dropped the run gate")
	}
}
