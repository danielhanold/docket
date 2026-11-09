package repoguard

import (
	"fmt"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

// topLevelKeys returns the keys of a YAML document's top-level mapping, in
// document order. It parses the document rather than matching lines, so a
// block mapping and a flow mapping are read alike and a commented-out key is
// not a key. An empty document has no keys.
func topLevelKeys(b []byte) ([]string, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return nil, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("top level is not a mapping (kind %d)", root.Kind)
	}
	keys := make([]string, 0, len(root.Content)/2)
	for i := 0; i+1 < len(root.Content); i += 2 {
		keys = append(keys, root.Content[i].Value)
	}
	return keys, nil
}

// TestDocketYMLDeclaresNoTicketsKey: docket's own repository is never linked
// to a ticket system, so its committed .docket.yml must not carry a tickets
// block. The ticket settings are exercised in another repository.
func TestDocketYMLDeclaresNoTicketsKey(t *testing.T) {
	keys, err := topLevelKeys([]byte(readRepoFile(t, ".docket.yml")))
	if err != nil {
		t.Fatalf("parsing .docket.yml: %v", err)
	}
	if slices.Contains(keys, "tickets") {
		t.Errorf(".docket.yml declares a top-level tickets key; docket's own configuration must never carry one")
	}
}

// TestTopLevelKeysSeesTickets is the in-suite mutation proof for the guard
// above: the helper finds a tickets key in block and flow spelling, and does
// not mistake a commented-out one for a declaration.
func TestTopLevelKeysSeesTickets(t *testing.T) {
	for _, tc := range []struct {
		doc  string
		want bool
	}{
		{"tickets:\n  provider: jira\n", true},
		{"tickets: {provider: jira}\n", true},
		{"finalize:\n  gate: local\n# tickets:\n", false},
	} {
		keys, err := topLevelKeys([]byte(tc.doc))
		if err != nil {
			t.Fatalf("%q: %v", tc.doc, err)
		}
		if got := slices.Contains(keys, "tickets"); got != tc.want {
			t.Errorf("%q: found tickets = %v, want %v (keys %v)", tc.doc, got, tc.want, keys)
		}
	}
}
