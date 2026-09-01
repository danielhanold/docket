package reposetup

// testconfigedit.go — byte-preserving generation of the pending `.docket.yml`
// test-policy edit for a discovery outcome. It extends the source-preserving
// yaml.Node splice machinery of configedit.go (topLevelMapping, maxNodeLine):
// the document is never re-serialized, so every byte outside the specific
// key lines we replace or insert — unknown settings, comments, ordering,
// quoting, blank lines — is preserved exactly. A malformed existing file is
// refused with the file untouched; it is never re-written from a parsed tree.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"go.yaml.in/yaml/v3"
)

// TestPolicyEdit is the shared setup-time computation that both PlanInit and the
// app's init execution run: it discovers the suite over tree (using the resolved
// build/finalize commands so an already-configured pair short-circuits without
// probing), then renders the pending `.docket.yml` test-policy edit. existing is
// the current `.docket.yml` bytes (nil when no file exists). edited is nil when
// no write applies — configured, ambiguous, or the file already carries these
// exact settings — so a caller writes only when it is non-nil. outcome carries
// the discovery result (its candidates on ambiguity) for the caller to report.
// A probe fault or a malformed existing file surfaces as an error with no edit.
func TestPolicyEdit(cfg config.Effective, existing []byte, tree TestTree) (edited []byte, outcome DiscoveryOutcome, err error) {
	outcome, err = DiscoverTests(tree, cfg.Build.TestCommand.Value, cfg.Finalize.TestCommand.Value)
	if err != nil {
		return nil, DiscoveryOutcome{}, err
	}
	rendered, changed, rerr := RenderTestConfigEdit(existing, outcome)
	if rerr != nil {
		return nil, DiscoveryOutcome{}, rerr
	}
	if changed {
		edited = rendered
	}
	return edited, outcome, nil
}

// RenderTestConfigEdit produces the pending `.docket.yml` bytes for a discovery
// outcome. existing == nil means no file exists (fresh init renders a minimal
// file). It returns changed == false when the outcome requires no edit
// (configured, ambiguous, or the file already carries these exact settings —
// the idempotency case), returning the existing bytes untouched. It never
// writes: callers own the pending, unstaged write. Gate "off" is always the
// QUOTED scalar "off" (bare `off` is a YAML boolean keyword; AGENTS.md).
//
// detected → each of `build:` and `finalize:` carries `gate: local` and
// `test_command: <command>` (written under both keys, but they are separate
// settings). none → `gate: "off"` under both and NO `test_command` (no fake
// command). configured/ambiguous → (existing, false, nil). Malformed existing
// YAML → an error with (nil, false, err) — the file is never destructively
// rewritten.
func RenderTestConfigEdit(existing []byte, out DiscoveryOutcome) (edited []byte, changed bool, err error) {
	switch out.Kind {
	case DiscoveryConfigured, DiscoveryAmbiguous:
		return existing, false, nil
	case DiscoveryDetected, DiscoveryNone:
		// proceed
	default:
		return nil, false, fmt.Errorf("reposetup: unknown discovery kind %q; refusing to edit", out.Kind)
	}

	pairs := desiredPairs(out)

	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	starts := lineOffsets(existing)

	var splices []byteSplice
	var appends []string
	changed = false
	// build and finalize are edited independently: each owns its own gate and
	// test_command, even though a detected command is written identically to both.
	for _, owner := range []string{"build", "finalize"} {
		sp, appText, ch, err := planOwnerBlock(existing, starts, root, owner, pairs)
		if err != nil {
			return nil, false, err
		}
		if ch {
			changed = true
		}
		splices = append(splices, sp...)
		if appText != "" {
			appends = append(appends, appText)
		}
	}

	if !changed {
		return existing, false, nil
	}

	edited = applySplices(existing, splices)
	edited = appendBlocks(edited, appends)
	return edited, true, nil
}

// kvPair is one leaf setting to ensure inside an owner block. preserveExplicit
// leaves an ALREADY-explicit, genuinely different value untouched (fill-if-missing
// only) — set on gate so a legacy block that deliberately set finalize.gate to
// off/ci/both is not clobbered to the generated default (spec: already-explicit
// new-style settings are preserved). A value equal to the desired one but only
// mis-quoted (bare `off` → `"off"`, AGENTS.md) is still normalized, since that
// re-quote does not change the value.
type kvPair struct {
	key, val         string
	preserveExplicit bool
}

// desiredPairs is the ordered set of leaf settings a detected/none outcome
// writes into each of build and finalize. none writes gate only — never a
// fabricated command. gate is preserve-explicit: a missing gate is filled with
// the generated default, but an explicit divergent gate is never overwritten.
func desiredPairs(out DiscoveryOutcome) []kvPair {
	switch out.Kind {
	case DiscoveryDetected:
		return []kvPair{{"gate", "local", true}, {"test_command", out.Command, false}}
	case DiscoveryNone:
		return []kvPair{{"gate", "off", true}}
	}
	return nil
}

// byteSplice replaces src[from:to] with text. Splices from one render are
// non-overlapping and are computed against the ORIGINAL src offsets.
type byteSplice struct {
	from, to int
	text     string
}

// applySplices rewrites src with every splice applied. Splices must be
// non-overlapping; they are sorted ascending and stitched in one pass.
func applySplices(src []byte, splices []byteSplice) []byte {
	if len(splices) == 0 {
		return append([]byte(nil), src...)
	}
	sort.Slice(splices, func(i, j int) bool { return splices[i].from < splices[j].from })
	out := make([]byte, 0, len(src))
	prev := 0
	for _, s := range splices {
		out = append(out, src[prev:s.from]...)
		out = append(out, s.text...)
		prev = s.to
	}
	out = append(out, src[prev:]...)
	return out
}

// appendBlocks writes any wholly-missing owner blocks at EOF, ensuring the file
// ends with a newline before the first appended block.
func appendBlocks(src []byte, blocks []string) []byte {
	if len(blocks) == 0 {
		return src
	}
	out := append([]byte(nil), src...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	for _, b := range blocks {
		out = append(out, b...)
	}
	return out
}

// planOwnerBlock computes the edits for one owner block. When the block is
// absent it returns appendText (a whole block to write at EOF); when present it
// returns in-place splices that replace a divergent leaf line or insert a
// missing one, preserving every other byte of the block.
func planOwnerBlock(src []byte, starts []int, root *yaml.Node, owner string, pairs []kvPair) (splices []byteSplice, appendText string, changed bool, err error) {
	keyIdx := -1
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			k := root.Content[i]
			if k.Kind == yaml.ScalarNode && k.Value == owner {
				if keyIdx >= 0 {
					return nil, "", false, fmt.Errorf("reposetup: %q declared more than once at the top level; refusing to edit", owner)
				}
				keyIdx = i
			}
		}
	}
	if keyIdx < 0 {
		return nil, ownerBlockText(owner, pairs), true, nil
	}

	ownerKey := root.Content[keyIdx]
	valNode := root.Content[keyIdx+1]

	// A block value that is neither a mapping nor an explicit null cannot hold
	// leaf settings: refuse rather than corrupt it.
	isNull := valNode.Kind == yaml.ScalarNode && valNode.Tag == "!!null"
	if !isNull && valNode.Kind != yaml.MappingNode {
		return nil, "", false, fmt.Errorf("reposetup: %q is present but is not a mapping; refusing to edit", owner)
	}

	indent := "  "
	if valNode.Kind == yaml.MappingNode && len(valNode.Content) > 0 {
		indent = strings.Repeat(" ", valNode.Content[0].Column-1)
	}

	var missing []kvPair
	for _, p := range pairs {
		childKey, childVal, dup := findChild(valNode, p.key)
		if dup {
			return nil, "", false, fmt.Errorf("reposetup: %q declared more than once under %q; refusing to edit", p.key, owner)
		}
		if childKey == nil {
			missing = append(missing, p)
			continue
		}
		if valueMatches(childVal, p.val) {
			continue
		}
		// A preserve-explicit leaf (gate) whose existing value is a genuine,
		// non-empty DIFFERENT value is left intact — only a missing (or null)
		// gate is filled with the default. A same-value-but-mis-quoted leaf falls
		// through to the re-quote splice below, since that does not change the value.
		if p.preserveExplicit && childVal != nil && childVal.Value != "" && childVal.Value != p.val {
			continue
		}
		// Replace the divergent leaf line(s) [keyLine, endLine] in place.
		startLine := childKey.Line
		endLine := maxNodeLine(childVal)
		if endLine < startLine {
			endLine = startLine
		}
		from := starts[startLine-1]
		to := lineEndByte(src, starts, endLine)
		text := indent + p.key + ": " + renderYAMLScalar(p.val)
		if !(to == len(src) && (to == 0 || src[to-1] != '\n')) {
			text += "\n"
		}
		splices = append(splices, byteSplice{from, to, text})
		changed = true
	}

	if len(missing) > 0 {
		// Insert missing leaves after the block's last line.
		lastLine := ownerKey.Line
		if valNode.Kind == yaml.MappingNode && len(valNode.Content) > 0 {
			lastLine = maxNodeLine(valNode)
		}
		at := lineEndByte(src, starts, lastLine)
		var b strings.Builder
		if at > 0 && src[at-1] != '\n' {
			b.WriteByte('\n') // previous (final) line lacked a newline
		}
		for _, p := range missing {
			b.WriteString(indent)
			b.WriteString(p.key)
			b.WriteString(": ")
			b.WriteString(renderYAMLScalar(p.val))
			b.WriteByte('\n')
		}
		splices = append(splices, byteSplice{at, at, b.String()})
		changed = true
	}

	return splices, "", changed, nil
}

// ownerBlockText renders a whole owner block for a file (or an EOF append) that
// has no such block yet.
func ownerBlockText(owner string, pairs []kvPair) string {
	var b strings.Builder
	b.WriteString(owner)
	b.WriteString(":\n")
	for _, p := range pairs {
		b.WriteString("  ")
		b.WriteString(p.key)
		b.WriteString(": ")
		b.WriteString(renderYAMLScalar(p.val))
		b.WriteByte('\n')
	}
	return b.String()
}

// findChild returns the first key/value node pair under mapping m whose key
// scalar equals key, and reports dup when the key appears more than once (which
// a line splice must refuse rather than edit one of two).
func findChild(m *yaml.Node, key string) (keyNode, valNode *yaml.Node, dup bool) {
	if m.Kind != yaml.MappingNode {
		return nil, nil, false
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			if keyNode != nil {
				return keyNode, valNode, true
			}
			keyNode, valNode = k, m.Content[i+1]
		}
	}
	return keyNode, valNode, false
}

// valueMatches reports whether an existing leaf already renders to the desired
// value with acceptable quoting — so no edit is needed. A value that MUST be
// quoted (e.g. "off") but is written unquoted does NOT match: it is rewritten
// to the quoted form.
func valueMatches(valNode *yaml.Node, desired string) bool {
	if valNode == nil || valNode.Value != desired {
		return false
	}
	if !plainSafe(desired) {
		if valNode.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 {
			return false
		}
	}
	return true
}

// lineOffsets returns the byte offset at which each physical line begins.
// lineOffsets(src)[N-1] is the start of the 1-based line N; a file with no
// trailing newline still counts its final line.
func lineOffsets(src []byte) []int {
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

// lineEndByte returns the byte offset just past the 1-based line (i.e. the
// start of the next line, or len(src) for the final line).
func lineEndByte(src []byte, starts []int, line int) int {
	if line >= len(starts) {
		return len(src)
	}
	return starts[line]
}

// renderYAMLScalar renders v as a YAML scalar, double-quoting whenever a plain
// scalar would be unsafe — YAML boolean/null keywords (true/false/yes/no/on/off/
// null), the empty string, or any character outside a conservative plain set.
// A script writing generated values quotes at the write boundary rather than
// predicating on a reader's tolerance (AGENTS.md; ADR-0071).
func renderYAMLScalar(v string) string {
	if plainSafe(v) {
		return v
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range v {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// plainSafe reports whether v can be emitted as a bare YAML plain scalar. It is
// deliberately conservative: alphanumerics plus a small set of command-safe
// punctuation, a leading alphanumeric, and never a YAML boolean/null keyword.
func plainSafe(v string) bool {
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return false
	}
	c := v[0]
	if !isAlnum(c) {
		return false
	}
	for i := 0; i < len(v); i++ {
		ch := v[i]
		if isAlnum(ch) {
			continue
		}
		switch ch {
		case ' ', '.', '/', '-', '_':
		default:
			return false
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
