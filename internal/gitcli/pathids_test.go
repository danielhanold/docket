package gitcli

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

func TestPathTreesPreserveLiteralNamesAndObjectWidths(t *testing.T) {
	for _, width := range []int{40, 64} {
		root := ObjectID(strings.Repeat("a", width))
		leaf := strings.Repeat("b", width)
		raw, err := hex.DecodeString(leaf)
		if err != nil {
			t.Fatal(err)
		}
		body := append([]byte("40000 space\nname\x00"), raw...)
		frame := append([]byte(fmt.Sprintf("%s tree %d\n", root, len(body))), body...)
		frame = append(frame, '\n')
		trees, err := parsePathTrees(frame, []ObjectID{root})
		if err != nil {
			t.Fatal(err)
		}
		entry := trees[root]["space\nname"]
		if !entry.directory || entry.id != ObjectID(leaf) {
			t.Fatalf("lost literal entry or hash width %d: %+v", width, entry)
		}
	}
}

func TestPathTreesRejectIncompleteOrUnprovenResponses(t *testing.T) {
	id := ObjectID(strings.Repeat("a", 40))
	for _, out := range []string{"", string(id) + " missing\n", string(id) + " tree 10\nshort\n", string(id) + " blob 0\n\n", strings.Repeat("b", 40) + " tree 0\n\n", string(id) + " tree 0\n\nextra", fmt.Sprintf("%s tree 5\nwrong\n", id)} {
		if trees, err := parsePathTrees([]byte(out), []ObjectID{id}); err == nil || trees != nil {
			t.Fatalf("accepted unproven response %q: %v %v", out, trees, err)
		}
	}
}
