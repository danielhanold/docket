package cli

// End-to-end tests of the assembled `docket capabilities` command: they drive
// the real Run entry (buffers, like every other cli test) and assert the
// contract the catalog promises consumers — a valid protocol-v1 JSON document,
// byte-identical across repeated invocations of the same binary, with commands
// sorted by stable id.

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

func TestCapabilitiesCommandEmitsSortedDeterministicJSON(t *testing.T) {
	out1, errS, code := runCLI(t, "capabilities", "--json")
	if code != 0 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out1, errS, code)
	}

	// Byte-identical across repeated invocations of the same binary.
	out2, _, _ := runCLI(t, "capabilities", "--json")
	if out1 != out2 {
		t.Fatalf("repeated catalog output is not byte-identical:\n1: %q\n2: %q", out1, out2)
	}

	var doc struct {
		ProtocolVersion   int    `json:"protocol_version"`
		CapabilityVersion int    `json:"capability_version"`
		Operation         string `json:"operation"`
		Commands          []struct {
			ID string `json:"id"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out1), &doc); err != nil {
		t.Fatalf("catalog is not valid JSON: %v\n%s", err, out1)
	}
	if doc.ProtocolVersion != 1 || doc.CapabilityVersion != 1 {
		t.Fatalf("versions: protocol=%d capability=%d", doc.ProtocolVersion, doc.CapabilityVersion)
	}
	if doc.Operation != "capabilities" {
		t.Fatalf("operation = %q, want capabilities", doc.Operation)
	}
	if len(doc.Commands) == 0 {
		t.Fatal("catalog carries no commands")
	}
	if !sort.SliceIsSorted(doc.Commands, func(i, j int) bool { return doc.Commands[i].ID < doc.Commands[j].ID }) {
		t.Fatalf("commands not sorted by id: %+v", doc.Commands)
	}

	// The bootstrap appears in its own catalog: a consumer that fetched the
	// catalog can always resolve the operation it just ran.
	found := false
	for _, c := range doc.Commands {
		if c.ID == "capabilities" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("capabilities is absent from its own catalog")
	}
}

// TestCapabilitiesCommandHumanText proves the non-JSON path renders the compact
// human catalog rather than a protocol document.
func TestCapabilitiesCommandHumanText(t *testing.T) {
	out, errS, code := runCLI(t, "capabilities")
	if code != 0 || errS != "" {
		t.Fatalf("out=%q err=%q code=%d", out, errS, code)
	}
	// Human text is deliberately not a protocol document.
	if json.Valid([]byte(strings.TrimSpace(out))) {
		t.Fatalf("human mode emitted JSON: %q", out)
	}
	for _, want := range []string{"capabilities v1", "capabilities"} {
		if !strings.Contains(out, want) {
			t.Fatalf("human catalog missing %q: %q", want, out)
		}
	}
}
