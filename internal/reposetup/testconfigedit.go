package reposetup

// testconfigedit.go — byte-preserving generation of the pending `.docket.yml`
// test-policy edit for a discovery outcome. It extends the source-preserving
// yaml.Node splice machinery of configedit.go (topLevelMapping, maxNodeLine):
// the document is never re-serialized, so every byte outside the specific
// key lines we replace or insert — unknown settings, comments, ordering,
// quoting, blank lines — is preserved exactly. A malformed existing file is
// refused with the file untouched; it is never re-written from a parsed tree.

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/document"
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

	return renderOwnerPairs(existing, desiredPairs(out))
}

// renderOwnerPairs is the shared splice core of every test-policy render: it
// ensures each pair under BOTH the build and finalize owner blocks, replacing
// a divergent leaf in place, inserting a missing one, or appending a wholly
// missing block, and preserves every other byte. changed == false returns the
// existing bytes untouched. A malformed or structurally unsafe file is an
// error with (nil, false, err).
func renderOwnerPairs(existing []byte, pairs []kvPair) (edited []byte, changed bool, err error) {
	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	starts := lineOffsets(existing)

	var splices []byteSplice
	var appends []string
	// build and finalize are edited independently: each owns its own gate and
	// test_command, even though one command is written identically to both.
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
	if verr := verifyOwnerPairs(root, edited, pairs); verr != nil {
		return nil, false, verr
	}
	return edited, true, nil
}

// verifyOwnerPairs is the post-splice re-parse guard (mirroring
// RemoveMetadataBranchKey's). The line planner locates a leaf through
// maxNodeLine, which under-reports a folded or literal block scalar (its
// continuation lines carry no child nodes) and shares a line between a
// flow-style owner and its leaves — so a splice can leave continuation lines
// behind, silently changing the command or breaking the YAML, or delete the
// owner key itself. Re-parse the edited bytes and refuse unless every original
// top-level key survives in order (plus any appended owner), and both build and
// finalize hold exactly the desired pairs — a preserve-explicit leaf keeping its
// original value — with no owner child gained or lost.
func verifyOwnerPairs(origRoot *yaml.Node, edited []byte, pairs []kvPair) error {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("reposetup: the test-policy edit cannot be spliced safely into .docket.yml ("+format+"); refusing to edit — set build/finalize gate and test_command by hand", args...)
	}
	after, err := topLevelMapping(edited)
	if err != nil {
		return refuse("the edited file does not parse: %v", err)
	}
	if after == nil {
		return refuse("the edited file has no top-level mapping")
	}
	origKeys := mappingKeys(origRoot)
	afterKeys := mappingKeys(after)
	if len(afterKeys) < len(origKeys) {
		return refuse("a top-level setting was lost")
	}
	for i, k := range origKeys {
		if afterKeys[i] != k {
			return refuse("top-level setting %q was lost or moved", k)
		}
	}
	for _, k := range afterKeys[len(origKeys):] {
		if (k != "build" && k != "finalize") || containsString(origKeys, k) {
			return refuse("unexpected top-level setting %q", k)
		}
	}

	for _, owner := range []string{"build", "finalize"} {
		var origOwner *yaml.Node
		if origRoot != nil {
			_, origOwner, _ = findChild(origRoot, owner)
		}
		_, afterOwner, dup := findChild(after, owner)
		if dup || afterOwner == nil || afterOwner.Kind != yaml.MappingNode {
			return refuse("%q is not a single mapping after the edit", owner)
		}
		want := map[string]string{}
		for _, p := range pairs {
			w := p.val
			if p.preserveExplicit && origOwner != nil {
				if _, o, _ := findChild(origOwner, p.key); o != nil && o.Value != "" && o.Value != p.val {
					w = o.Value
				}
			}
			want[p.key] = w
			_, v, dup := findChild(afterOwner, p.key)
			if dup || v == nil || v.Kind != yaml.ScalarNode || v.Value != w {
				got := "<missing>"
				if v != nil {
					got = fmt.Sprintf("%q", v.Value)
				}
				return refuse("%s.%s would be %s, want %q", owner, p.key, got, w)
			}
		}
		// Every non-pair child the owner had survives, and nothing else appears.
		expect := map[string]bool{}
		for _, k := range mappingKeys(origOwner) {
			expect[k] = true
		}
		for k := range want {
			expect[k] = true
		}
		got := mappingKeys(afterOwner)
		if len(got) != len(expect) {
			return refuse("%q gained or lost a setting", owner)
		}
		for _, k := range got {
			if !expect[k] {
				return refuse("%q gained setting %q", owner, k)
			}
		}
	}
	return nil
}

// mappingKeys returns the scalar key names of mapping m in document order; a
// nil or non-mapping node has none.
func mappingKeys(m *yaml.Node) []string {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	var keys []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		keys = append(keys, m.Content[i].Value)
	}
	return keys
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
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

// ConfigureTestsCommandRemedy is the configure-tests invocation that sets both
// gates to `local` with an operator-supplied suite command. Every note that
// sends an operator to an explicit command names it through this constant.
const ConfigureTestsCommandRemedy = `docket repository configure-tests --command "<cmd>"`

// ErrInvalidTestCommand classifies a refused --command value.
var ErrInvalidTestCommand = errors.New("invalid test command")

// ExplicitTestCommand validates an operator-supplied suite command and returns
// it trimmed of surrounding whitespace. It refuses an empty or whitespace-only
// value, the legacy `auto` sentinel (it never survives resolution as a command;
// isConfiguredCommand), invalid UTF-8, and any rune document.IllegalTextRune
// refuses (control characters, tab, and the U+2028/U+2029 line separators),
// because a double-quoted YAML scalar folds a line break to a space and would
// write a command other than the one given.
func ExplicitTestCommand(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", fmt.Errorf("%w: --command is not valid UTF-8", ErrInvalidTestCommand)
	}
	cmd := strings.TrimSpace(raw)
	if cmd == "" {
		return "", fmt.Errorf(`%w: --command is empty; pass the suite command, e.g. --command "make test"`, ErrInvalidTestCommand)
	}
	if !isConfiguredCommand(cmd) {
		return "", fmt.Errorf("%w: --command %q is the legacy unconfigured sentinel, not a suite command", ErrInvalidTestCommand, cmd)
	}
	for _, r := range cmd {
		if document.IllegalTextRune(r) {
			return "", fmt.Errorf("%w: --command contains a control or line-break character (%U); pass a single-line command", ErrInvalidTestCommand, r)
		}
	}
	return cmd, nil
}

// RenderExplicitTestCommandEdit produces the pending `.docket.yml` bytes that
// set BOTH build and finalize to `gate: local` + `test_command: <cmd>`. Unlike
// the discovery render, the explicit command is the human's choice, so it wins:
// the gate pair is NOT preserve-explicit, an existing `off` (or any other)
// gate becomes `local`, and a different command is replaced. changed == false
// means the file already carries exactly these settings. cmd must already have
// passed ExplicitTestCommand; an invalid cmd is refused here too.
func RenderExplicitTestCommandEdit(existing []byte, cmd string) (edited []byte, changed bool, err error) {
	if _, verr := ExplicitTestCommand(cmd); verr != nil {
		return nil, false, verr
	}
	return renderOwnerPairs(existing, explicitPairs(cmd))
}

// explicitPairs is the explicit-command policy: gate local and the command,
// neither preserve-explicit.
func explicitPairs(cmd string) []kvPair {
	return []kvPair{{"gate", "local", false}, {"test_command", cmd, false}}
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
	// The charset admits digit-led tokens that YAML resolves to a non-string
	// (123 → !!int, 1.5 → !!float, 2026-10-05 → !!timestamp), and the config
	// schema's string leaves accept only !!str. Ask the resolver itself rather
	// than enumerating number shapes.
	return resolvesToStr(v)
}

// resolvesToStr reports whether v, emitted as a bare plain scalar, reads back
// as the !!str v.
func resolvesToStr(v string) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(v), &doc); err != nil || len(doc.Content) != 1 {
		return false
	}
	n := doc.Content[0]
	return n.Kind == yaml.ScalarNode && n.Tag == "!!str" && n.Value == v
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
