package reposetup

// configedit.go — byte-preserving removal of the legacy top-level
// `metadata_branch` key from the pinned .docket.yml bytes.
//
// The key is LOCATED with a YAML AST parse (for validity), but the REMOVAL is a
// line-range splice on the raw bytes: the document is never re-serialized, so
// every byte outside the removed entry's own line(s) — unknown settings,
// comments, ordering, quoting, blank lines, and the file's line endings
// (LF or CRLF) — is preserved exactly.

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// metadataBranchKey is the single top-level setting this editor removes.
const metadataBranchKey = "metadata_branch"

// RemoveMetadataBranchKey removes the single top-level `metadata_branch` entry
// (its key line plus any continuation lines of that mapping value) from src,
// preserving every other byte. It returns (out, true, nil) when the key was
// removed; (nil, false, nil) when the key is absent at the top level; and an
// error (with out==nil, removed==false) when the file cannot be safely edited:
// undecodable YAML, a duplicate top-level `metadata_branch`, a document root
// that is not a mapping, or a located entry whose lines overlap another
// top-level entry (a flow-style root mapping) — none of which a line splice can
// touch without corrupting the file.
//
// A `metadata_branch` key nested under another mapping is NOT a top-level entry:
// it is left in place and reported absent.
func RemoveMetadataBranchKey(src []byte) (out []byte, removed bool, err error) {
	root, err := topLevelMapping(src)
	if err != nil {
		return nil, false, err
	}
	if root == nil {
		// Empty, comments-only, or explicit-null document: no top-level keys.
		return nil, false, nil
	}

	// Locate every top-level metadata_branch key node. yaml.v3's Node decode
	// does NOT collapse duplicate keys, so a repeated key surfaces here as two
	// nodes — which we must refuse rather than silently splice one of.
	var keyIdx []int // index into root.Content of each matching KEY node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if k := root.Content[i]; k.Kind == yaml.ScalarNode && k.Value == metadataBranchKey {
			keyIdx = append(keyIdx, i)
		}
	}
	switch len(keyIdx) {
	case 0:
		return nil, false, nil
	case 1:
		// proceed
	default:
		return nil, false, fmt.Errorf("reposetup: %q declared more than once at the top level; refusing to edit", metadataBranchKey)
	}

	i := keyIdx[0]
	keyNode := root.Content[i]
	valNode := root.Content[i+1]

	// The entry occupies raw lines [startLine, endLine] (1-based, matching
	// yaml.v3's line numbers, which count physical '\n'-separated lines).
	startLine := keyNode.Line
	endLine := maxNodeLine(valNode)
	if endLine < startLine {
		endLine = startLine
	}

	// Overlap guard: the removed line range must not intrude on any OTHER
	// top-level entry. A flow-style root mapping ({metadata_branch: x, a: y})
	// puts sibling keys on the same physical line, where a line splice would
	// delete unrelated settings — that is not a safe edit.
	if i-2 >= 0 {
		if prevEnd := maxNodeLine(root.Content[i-1]); prevEnd >= startLine {
			return nil, false, fmt.Errorf("reposetup: %q shares a line with another top-level setting; refusing to edit", metadataBranchKey)
		}
	}
	if i+2 < len(root.Content) {
		if nextStart := root.Content[i+2].Line; nextStart <= endLine {
			return nil, false, fmt.Errorf("reposetup: %q shares a line with another top-level setting; refusing to edit", metadataBranchKey)
		}
	}

	spliced, ok := spliceOutLines(src, startLine, endLine)
	if !ok {
		return nil, false, fmt.Errorf("reposetup: computed line range [%d,%d] is out of bounds; refusing to edit", startLine, endLine)
	}

	// Defense in depth: the located value may span lines the AST under-reports
	// (a literal/folded block scalar carries no child line nodes), which would
	// leave an orphaned value fragment. Re-parse the spliced output as a single
	// YAML document and refuse if the edit did not leave a clean, key-free file.
	after, err := topLevelMapping(spliced)
	if err != nil {
		return nil, false, fmt.Errorf("reposetup: line-splice removal did not yield valid YAML; refusing to edit: %w", err)
	}
	if after != nil {
		for j := 0; j+1 < len(after.Content); j += 2 {
			if k := after.Content[j]; k.Kind == yaml.ScalarNode && k.Value == metadataBranchKey {
				return nil, false, fmt.Errorf("reposetup: %q still present after removal; refusing to edit", metadataBranchKey)
			}
		}
	}

	return spliced, true, nil
}

// topLevelMapping parses src as a single YAML document and returns its root
// mapping node. It returns (nil, nil) for an empty, comments-only, or
// explicit-null document (no top-level keys), and an error for undecodable
// YAML, more than one document, or a root that is not a mapping (there is no
// top-level mapping to hold the key).
func topLevelMapping(src []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, fmt.Errorf("reposetup: undecodable YAML: %w", err)
	}
	if len(doc.Content) == 0 {
		return nil, nil // empty or comments-only
	}
	if len(doc.Content) > 1 {
		return nil, errors.New("reposetup: file contains more than one YAML document; refusing to edit")
	}
	root := doc.Content[0]
	if root.Tag == "!!null" {
		return nil, nil // explicit null document
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("reposetup: document root is not a mapping; there is no top-level key to remove")
	}
	return root, nil
}

// maxNodeLine returns the largest line number spanned by n and its descendants.
// For a plain/quoted scalar this is the node's own line; for a flow collection
// or a block mapping/sequence value it is the deepest child's line, which is the
// last physical line the value occupies. (Block SCALARS carry no child nodes and
// thus under-report; RemoveMetadataBranchKey's post-splice re-parse catches that
// pathological case for a branch-name setting.)
func maxNodeLine(n *yaml.Node) int {
	m := n.Line
	for _, c := range n.Content {
		if cl := maxNodeLine(c); cl > m {
			m = cl
		}
	}
	return m
}

// spliceOutLines removes physical lines [startLine, endLine] (1-based, inclusive)
// from src, keeping each surviving line's exact bytes and terminator. A line's
// terminator is the trailing '\n' (a preceding '\r' stays attached to the line
// content, so CRLF endings on surviving lines are preserved). It reports ok
// false when the range falls outside the file.
func spliceOutLines(src []byte, startLine, endLine int) (out []byte, ok bool) {
	if startLine < 1 || endLine < startLine {
		return nil, false
	}
	// lineStart[k] is the byte offset where the (k+1)-th line begins. There is
	// one entry per line; a file with no trailing newline still counts its final
	// line.
	lineStart := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lineStart = append(lineStart, i+1)
		}
	}
	nLines := len(lineStart)
	if startLine > nLines || endLine > nLines {
		return nil, false
	}
	from := lineStart[startLine-1]
	var to int
	if endLine < nLines {
		to = lineStart[endLine] // start of the first surviving line after the range
	} else {
		to = len(src) // removing through the last line (which may lack a final '\n')
	}
	out = make([]byte, 0, len(src)-(to-from))
	out = append(out, src[:from]...)
	out = append(out, src[to:]...)
	return out, true
}
