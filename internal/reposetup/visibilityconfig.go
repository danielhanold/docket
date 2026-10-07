package reposetup

// visibilityconfig.go — the pure config and ignore-block transforms a
// visibility switch applies. Going private folds the committed and local
// config into one private config and removes the managed ignore block; going
// shared splits the private config back. Bytes in, bytes out: callers own
// reading and atomically writing every file.
//
// A leaf is any value that is not a non-empty mapping (a scalar, a sequence,
// an empty mapping), named by its dotted path. That matches config's merge
// rule: blocks merge key by key, lists replace.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

const visibilityKey = "visibility"

// configLeaf is one leaf of a config mapping: its dotted path, the key
// segments that spell it, and its value node.
type configLeaf struct {
	path string
	keys []string
	val  *yaml.Node
}

// FoldPrivateConfig folds a shared repository's committed and local config
// into one private config. It starts from committed, overlays every local
// leaf except a repo-only one (ignored while shared, so it must not become
// active), drops agent pins from both (config refuses them in repository
// layers), and sets the top-level visibility to private. dropped names every
// leaf left out, committed pins first, without repeats. Nil or whitespace
// input is an empty mapping; a root that is not a mapping errors.
func FoldPrivateConfig(committed, local []byte, repoOnly []string) (folded []byte, dropped []string, err error) {
	doc, root, err := parseConfigMapping(committed)
	if err != nil {
		return nil, nil, fmt.Errorf("reposetup: committed config: %w", err)
	}
	_, lroot, err := parseConfigMapping(local)
	if err != nil {
		return nil, nil, fmt.Errorf("reposetup: local config: %w", err)
	}
	drop := func(p string) {
		if !slices.Contains(dropped, p) {
			dropped = append(dropped, p)
		}
	}
	for _, l := range configLeaves(root, nil) {
		if isAgentPin(l.keys) {
			deleteLeaf(root, l.keys)
			drop(l.path)
		}
	}
	for _, l := range configLeaves(lroot, nil) {
		if isAgentPin(l.keys) || isRepoOnly(l.path, repoOnly) {
			drop(l.path)
			continue
		}
		setLeaf(root, l.keys, l.val)
	}
	setTopScalar(root, visibilityKey, "private")
	folded, err = encodeConfig(doc)
	if err != nil {
		return nil, nil, err
	}
	return folded, dropped, nil
}

// SplitPrivateConfig splits a private config back into committed and local
// config. The leaf set of localKeys (the saved local file) selects the local
// leaves: a leaf goes to local iff its path is in that set and it is not
// repo-only, not an agent pin, and not visibility. Every other leaf stays in
// committed, which gets visibility shared. Local leaves carry the private
// config's current values. local is nil when it would be empty.
func SplitPrivateConfig(private, localKeys []byte, repoOnly []string) (committed, local []byte, err error) {
	doc, root, err := parseConfigMapping(private)
	if err != nil {
		return nil, nil, fmt.Errorf("reposetup: private config: %w", err)
	}
	_, kroot, err := parseConfigMapping(localKeys)
	if err != nil {
		return nil, nil, fmt.Errorf("reposetup: saved local keys: %w", err)
	}
	selected := map[string]bool{}
	for _, l := range configLeaves(kroot, nil) {
		selected[l.path] = true
	}
	ldoc, lroot, _ := parseConfigMapping(nil)
	for _, l := range configLeaves(root, nil) {
		if !selected[l.path] || l.path == visibilityKey || isAgentPin(l.keys) || isRepoOnly(l.path, repoOnly) {
			continue
		}
		setLeaf(lroot, l.keys, l.val)
		deleteLeaf(root, l.keys)
	}
	setTopScalar(root, visibilityKey, "shared")
	if committed, err = encodeConfig(doc); err != nil {
		return nil, nil, err
	}
	if len(lroot.Content) == 0 {
		return committed, nil, nil
	}
	if local, err = encodeConfig(ldoc); err != nil {
		return nil, nil, err
	}
	return committed, local, nil
}

// ConfigLeafValues returns, for each path present in src, its value rendered
// as trimmed YAML with comments stripped; an absent path is omitted.
func ConfigLeafValues(src []byte, paths []string) (map[string]string, error) {
	_, root, err := parseConfigMapping(src)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, p := range paths {
		n := lookupPath(root, strings.Split(p, "."))
		if n == nil {
			continue
		}
		b, err := yaml.Marshal(stripComments(cloneNode(n)))
		if err != nil {
			return nil, fmt.Errorf("reposetup: render %s: %w", p, err)
		}
		out[p] = strings.TrimSpace(string(b))
	}
	return out, nil
}

// RenderVisibilityEdit sets an explicit top-level visibility to value by a
// byte splice on its scalar, keeping every other byte (a trailing comment
// included). An absent key or one already equal returns (existing, false,
// nil): only an explicit, different value is rewritten. It refuses a value
// other than shared/private, a duplicated key, a non-scalar or multi-line
// value, a single-line {…} root, and any edit whose re-parse shows a change
// beyond that value.
func RenderVisibilityEdit(existing []byte, value string) (edited []byte, changed bool, err error) {
	if value != "shared" && value != "private" {
		return nil, false, fmt.Errorf("reposetup: visibility %q is not shared or private", value)
	}
	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	if root == nil {
		return existing, false, nil
	}
	byHand := "; refusing to edit — set visibility by hand"
	keyNode, val, dup := findChild(root, visibilityKey)
	if dup {
		return nil, false, fmt.Errorf("reposetup: %q declared more than once at the top level%s", visibilityKey, byHand)
	}
	if keyNode == nil {
		return existing, false, nil
	}
	if val.Kind == yaml.ScalarNode && val.Value == value {
		return existing, false, nil
	}
	if root.Style&yaml.FlowStyle != 0 {
		return nil, false, errors.New("reposetup: the config is a single-line {…} mapping" + byHand)
	}
	if val.Kind != yaml.ScalarNode || val.Line != keyNode.Line ||
		val.Style&^(yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle) != 0 {
		return nil, false, fmt.Errorf("reposetup: %q is not a one-line scalar%s", visibilityKey, byHand)
	}
	starts := lineOffsets(existing)
	from := starts[val.Line-1] + val.Column - 1
	lineEnd := lineEndByte(existing, starts, val.Line)
	to, ok := scalarSpanEnd(existing, from, lineEnd, val.Style)
	if !ok {
		return nil, false, fmt.Errorf("reposetup: cannot locate the %q value%s", visibilityKey, byHand)
	}
	out := append(append(append(make([]byte, 0, len(existing)), existing[:from]...), value...), existing[to:]...)
	if verr := verifyVisibilityEdit(root, out, value); verr != nil {
		return nil, false, fmt.Errorf("reposetup: the visibility edit cannot be made safely (%v)%s", verr, byHand)
	}
	return out, true, nil
}

// scalarSpanEnd returns the byte just past a one-line scalar starting at from:
// the closing quote for a quoted scalar, else the end of the plain text before
// a comment or the line end.
func scalarSpanEnd(src []byte, from, lineEnd int, style yaml.Style) (int, bool) {
	if from >= lineEnd {
		return 0, false
	}
	line := src[from:lineEnd]
	switch {
	case style&yaml.DoubleQuotedStyle != 0, style&yaml.SingleQuotedStyle != 0:
		q := line[0]
		for i := 1; i < len(line); i++ {
			if q == '"' && line[i] == '\\' {
				i++
				continue
			}
			if line[i] == q {
				if q == '\'' && i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				return from + i + 1, true
			}
		}
		return 0, false
	default:
		end := len(line)
		if i := bytes.Index(line, []byte(" #")); i >= 0 {
			end = i
		}
		return from + len(bytes.TrimRight(line[:end], " \t\r\n")), true
	}
}

// verifyVisibilityEdit re-parses edited and refuses unless the top-level keys
// are unchanged, every other value decodes identically, and visibility is want.
func verifyVisibilityEdit(origRoot *yaml.Node, edited []byte, want string) error {
	after, err := topLevelMapping(edited)
	if err != nil || after == nil {
		return fmt.Errorf("the edited file does not parse as a mapping: %v", err)
	}
	if got, orig := mappingKeys(after), mappingKeys(origRoot); !slices.Equal(got, orig) {
		return fmt.Errorf("the top-level settings would be %v, want %v", got, orig)
	}
	for i := 0; i+1 < len(after.Content); i += 2 {
		k, v := after.Content[i].Value, after.Content[i+1]
		if k == visibilityKey {
			if v.Kind != yaml.ScalarNode || v.Value != want {
				return fmt.Errorf("visibility would not be %s", want)
			}
			continue
		}
		_, orig, _ := findChild(origRoot, k)
		var a, b any
		if orig == nil || orig.Decode(&a) != nil || v.Decode(&b) != nil || !reflect.DeepEqual(a, b) {
			return fmt.Errorf("setting %q would change", k)
		}
	}
	return nil
}

// RemoveGitignoreBlock removes the managed docket block from a .gitignore,
// keeping every byte outside it and trimming trailing blank lines to one final
// newline (empty when nothing else remains). Malformed markers refuse with
// *MalformedGitignoreError and a nil out; no block returns (current, false, nil).
func RemoveGitignoreBlock(current []byte) (out []byte, changed bool, err error) {
	if gitignoreMarkersMalformed(current, GitignoreStart, GitignoreEnd) {
		return nil, false, &MalformedGitignoreError{Generation: "docket"}
	}
	return removeManagedBlock(current, GitignoreStart, GitignoreEnd)
}

// RemoveExcludeBlock removes the managed `# dckt:` block from
// .git/info/exclude with RemoveGitignoreBlock's rules; malformed markers
// refuse with *MalformedExcludeError.
func RemoveExcludeBlock(current []byte) (out []byte, changed bool, err error) {
	if gitignoreMarkersMalformed(current, ExcludeStart, ExcludeEnd) {
		return nil, false, &MalformedExcludeError{}
	}
	return removeManagedBlock(current, ExcludeStart, ExcludeEnd)
}

// removeManagedBlock strips a well-formed [start,end] block; callers guard the
// markers first.
func removeManagedBlock(current []byte, start, end string) ([]byte, bool, error) {
	if !hasLine(current, start) {
		return current, false, nil
	}
	rest := bytes.TrimRight(stripGitignoreBlock(current, start, end), "\n")
	out := make([]byte, 0, len(rest)+1)
	if len(rest) > 0 {
		out = append(append(out, rest...), '\n')
	}
	return out, true, nil
}

// parseConfigMapping parses src into a document node whose single child is
// the root mapping. Nil, whitespace, comments-only, or null input yields an
// empty mapping; more than one document or a non-mapping root errors.
func parseConfigMapping(src []byte) (doc, root *yaml.Node, err error) {
	doc = &yaml.Node{Kind: yaml.DocumentNode}
	dec := yaml.NewDecoder(bytes.NewReader(src))
	if derr := dec.Decode(doc); derr != nil && !errors.Is(derr, io.EOF) {
		return nil, nil, fmt.Errorf("undecodable YAML: %w", derr)
	} else if derr == nil {
		var extra yaml.Node
		if xerr := dec.Decode(&extra); !errors.Is(xerr, io.EOF) {
			return nil, nil, errors.New("more than one YAML document")
		}
	}
	doc.Kind = yaml.DocumentNode
	if len(doc.Content) == 0 || doc.Content[0].Tag == "!!null" {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root = doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, errors.New("the document root is not a mapping")
	}
	return doc, root, nil
}

// configLeaves lists m's leaves depth-first in file order, recursing only into
// non-empty mappings.
func configLeaves(m *yaml.Node, prefix []string) []configLeaf {
	var out []configLeaf
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys := append(slices.Clone(prefix), m.Content[i].Value)
		v := m.Content[i+1]
		if v.Kind == yaml.MappingNode && len(v.Content) > 0 {
			out = append(out, configLeaves(v, keys)...)
			continue
		}
		out = append(out, configLeaf{path: strings.Join(keys, "."), keys: keys, val: v})
	}
	return out
}

// isAgentPin reports an agents.<harness>.<agent>.model|effort|runner leaf.
func isAgentPin(keys []string) bool {
	if len(keys) != 4 || keys[0] != "agents" {
		return false
	}
	switch keys[3] {
	case "model", "effort", "runner":
		return true
	}
	return false
}

// isRepoOnly reports whether leaf path p is, or sits under, a repo-only entry.
func isRepoOnly(p string, repoOnly []string) bool {
	for _, r := range repoOnly {
		if p == r || strings.HasPrefix(p, r+".") {
			return true
		}
	}
	return false
}

// lookupPath returns the value node at keys under m, or nil.
func lookupPath(m *yaml.Node, keys []string) *yaml.Node {
	n := m
	for _, k := range keys {
		if n.Kind != yaml.MappingNode {
			return nil
		}
		_, v, _ := findChild(n, k)
		if v == nil {
			return nil
		}
		n = v
	}
	return n
}

// setLeaf sets keys under m to a copy of val, creating intermediate mappings
// (replacing a non-mapping in the way) and replacing an existing value. An
// empty-mapping leaf never replaces an existing mapping: it declares nothing.
func setLeaf(m *yaml.Node, keys []string, val *yaml.Node) {
	n := m
	for i, k := range keys {
		_, v, _ := findChild(n, k)
		last := i == len(keys)-1
		if last {
			if v != nil && v.Kind == yaml.MappingNode && val.Kind == yaml.MappingNode && len(val.Content) == 0 {
				return
			}
			c := cloneNode(val)
			if v != nil {
				*v = *c
			} else {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, c)
			}
			return
		}
		if v == nil {
			v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, v)
		} else if v.Kind != yaml.MappingNode {
			*v = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		}
		n = v
	}
}

// deleteLeaf removes keys under m and prunes every mapping the removal empties.
func deleteLeaf(m *yaml.Node, keys []string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value != keys[0] {
			continue
		}
		if len(keys) > 1 {
			v := m.Content[i+1]
			if v.Kind != yaml.MappingNode {
				return
			}
			deleteLeaf(v, keys[1:])
			if len(v.Content) > 0 {
				return
			}
		}
		m.Content = slices.Delete(m.Content, i, i+2)
		return
	}
}

// setTopScalar sets the top-level key to a plain string scalar, keeping an
// existing value's comments, or appends the key when absent.
func setTopScalar(m *yaml.Node, key, value string) {
	if _, v, _ := findChild(m, key); v != nil {
		*v = yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value,
			HeadComment: v.HeadComment, LineComment: v.LineComment, FootComment: v.FootComment}
		return
	}
	m.Content = append(m.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
}

// encodeConfig renders a document node with two-space indentation, so node
// comments survive.
func encodeConfig(doc *yaml.Node) ([]byte, error) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("reposetup: encode config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("reposetup: encode config: %w", err)
	}
	return b.Bytes(), nil
}

// cloneNode deep-copies n.
func cloneNode(n *yaml.Node) *yaml.Node {
	c := *n
	if n.Content != nil {
		c.Content = make([]*yaml.Node, len(n.Content))
		for i, ch := range n.Content {
			c.Content[i] = cloneNode(ch)
		}
	}
	return &c
}

// stripComments clears every comment and optional quoting in n and returns it,
// so equal values render equally (the encoder still quotes where it must).
func stripComments(n *yaml.Node) *yaml.Node {
	n.HeadComment, n.LineComment, n.FootComment = "", "", ""
	n.Style &^= yaml.SingleQuotedStyle | yaml.DoubleQuotedStyle
	for _, c := range n.Content {
		stripComments(c)
	}
	return n
}
