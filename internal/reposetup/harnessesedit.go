package reposetup

// harnessesedit.go — byte-preserving write of the top-level agent_harnesses
// key of a docket config file. The key is located with a YAML AST parse and
// replaced or appended by a line splice on the raw bytes, never re-serialized
// (like RemoveMetadataBranchKey), so every other byte of the file survives.
// The result is re-parsed and refused unless the only change is the key (like
// verifyOwnerPairs).

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"go.yaml.in/yaml/v3"
)

const agentHarnessesKey = "agent_harnesses"

// RenderAgentHarnessesEdit returns existing with its top-level agent_harnesses
// key set to the one flow line AgentHarnessesLine(harnesses): an existing key
// (flow or block form) is replaced in place, a missing one is appended. A nil
// existing means no file. harnesses must be non-nil and canonical (as
// ParseHarnessSelection returns it). When the key already holds that value it
// returns (existing, false, nil) and nothing should be written.
//
// It refuses, returning (nil, false, err), on: a nil, invalid, or
// non-canonical selection; a multi-document file; a root that is not a mapping
// or is a single-line {…} mapping; undecodable YAML; the key declared more
// than once at the top level; the key sharing a line with another setting; and
// any edit whose re-parse shows a change beyond the key (a block scalar value,
// for one).
func RenderAgentHarnessesEdit(existing []byte, harnesses []string) (edited []byte, changed bool, err error) {
	if harnesses == nil {
		return nil, false, errors.New("reposetup: no agent_harnesses selection to write")
	}
	if len(harnesses) > 0 {
		canon, perr := ParseHarnessSelection(harnesses)
		if perr != nil {
			return nil, false, perr
		}
		if !slices.Equal(canon, harnesses) {
			return nil, false, fmt.Errorf("reposetup: agent_harnesses selection %v is not in canonical order", harnesses)
		}
	}
	root, err := topLevelMapping(existing)
	if err != nil {
		return nil, false, err
	}
	byHand := "; refusing to edit — set agent_harnesses by hand"
	if root != nil && root.Style&yaml.FlowStyle != 0 {
		return nil, false, errors.New("reposetup: the config is a single-line {…} mapping" + byHand)
	}
	eol := "\n"
	if bytes.Contains(existing, []byte("\r\n")) {
		eol = "\r\n"
	}
	line := AgentHarnessesLine(harnesses)
	keyIdx := -1
	if root != nil {
		for i := 0; i+1 < len(root.Content); i += 2 {
			if k := root.Content[i]; k.Kind == yaml.ScalarNode && k.Value == agentHarnessesKey {
				if keyIdx >= 0 {
					return nil, false, fmt.Errorf("reposetup: %q declared more than once at the top level%s", agentHarnessesKey, byHand)
				}
				keyIdx = i
			}
		}
	}
	var out []byte
	if keyIdx < 0 {
		out = append([]byte(nil), existing...)
		if len(out) > 0 && out[len(out)-1] != '\n' {
			out = append(out, eol...)
		}
		out = append(out, line+eol...)
	} else {
		val := root.Content[keyIdx+1]
		if harnessSequenceEquals(val, harnesses) {
			return existing, false, nil
		}
		starts := lineOffsets(existing)
		start, end := root.Content[keyIdx].Line, maxNodeLine(val)
		if end < start || val.Kind == yaml.ScalarNode && val.Tag == "!!null" && val.Value == "" {
			end = start // an empty value lives on the key's own line
		}
		// A flow sequence's node stops at its last item; its closing ] may sit on
		// a later line.
		if val.Kind == yaml.SequenceNode && val.Style&yaml.FlowStyle != 0 {
			if closing := flowSequenceEndLine(existing, starts, val); closing > end {
				end = closing
			}
		}
		if keyIdx > 0 && maxNodeLine(root.Content[keyIdx-1]) >= start ||
			keyIdx+2 < len(root.Content) && root.Content[keyIdx+2].Line <= end {
			return nil, false, fmt.Errorf("reposetup: %q shares a line with another setting%s", agentHarnessesKey, byHand)
		}
		from, to := starts[start-1], lineEndByte(existing, starts, end)
		text := line
		if !(to == len(existing) && (to == 0 || existing[to-1] != '\n')) {
			text += eol
		}
		out = append(append(append(make([]byte, 0, len(existing)+len(text)), existing[:from]...), text...), existing[to:]...)
	}
	if verr := verifyHarnessesEdit(root, out, harnesses); verr != nil {
		return nil, false, verr
	}
	return out, true, nil
}

// flowSequenceEndLine returns the 1-based line holding the ] that closes the
// flow sequence n, whose [ is at n's own position in src, skipping quoted
// scalars and comments. It returns 0 when that bracket cannot be matched; the
// post-edit re-parse then refuses any splice that cut the value short.
func flowSequenceEndLine(src []byte, starts []int, n *yaml.Node) int {
	if n.Line < 1 || n.Line > len(starts) || n.Column < 1 {
		return 0
	}
	i := starts[n.Line-1] + n.Column - 1
	if i >= len(src) || src[i] != '[' {
		return 0
	}
	line, depth := n.Line, 0
	for ; i < len(src); i++ {
		switch c := src[i]; c {
		case '\n':
			line++
		case '[', '{':
			depth++
		case ']', '}':
			if depth--; depth == 0 {
				return line
			}
		case '#':
			if i > 0 && src[i-1] != ' ' && src[i-1] != '\t' && src[i-1] != '\n' {
				continue
			}
			for i+1 < len(src) && src[i+1] != '\n' {
				i++
			}
		case '"', '\'':
			for i++; i < len(src); i++ {
				if src[i] == '\n' {
					line++
				} else if c == '"' && src[i] == '\\' {
					if i++; i < len(src) && src[i] == '\n' {
						line++
					}
				} else if src[i] == c {
					if c == '\'' && i+1 < len(src) && src[i+1] == '\'' {
						i++
						continue
					}
					break
				}
			}
		}
	}
	return 0
}

// harnessSequenceEquals reports whether n is a sequence of exactly the string
// scalars want, in order.
func harnessSequenceEquals(n *yaml.Node, want []string) bool {
	if n == nil || n.Kind != yaml.SequenceNode || len(n.Content) != len(want) {
		return false
	}
	for i, item := range n.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Value != want[i] {
			return false
		}
	}
	return true
}

// verifyHarnessesEdit re-parses the edited bytes and refuses unless the
// top-level keys are the original ones (plus an appended agent_harnesses),
// every other value decodes identically, and agent_harnesses equals want. It
// is what catches a block scalar value, whose node under-reports its last line.
func verifyHarnessesEdit(origRoot *yaml.Node, edited []byte, want []string) error {
	refuse := func(format string, args ...any) error {
		return fmt.Errorf("reposetup: the agent_harnesses edit cannot be made safely ("+format+"); refusing to edit — set agent_harnesses by hand", args...)
	}
	after, err := topLevelMapping(edited)
	if err != nil || after == nil {
		return refuse("the edited file does not parse as a mapping: %v", err)
	}
	wantKeys := mappingKeys(origRoot)
	if !containsString(wantKeys, agentHarnessesKey) {
		wantKeys = append(append([]string(nil), wantKeys...), agentHarnessesKey)
	}
	if got := mappingKeys(after); !slices.Equal(got, wantKeys) {
		return refuse("the top-level settings would be %v, want %v", got, wantKeys)
	}
	for i := 0; i+1 < len(after.Content); i += 2 {
		k, v := after.Content[i].Value, after.Content[i+1]
		if k == agentHarnessesKey {
			if !harnessSequenceEquals(v, want) {
				return refuse("agent_harnesses would not be %s", FormatHarnessList(want))
			}
			continue
		}
		_, orig, _ := findChild(origRoot, k) // reached only for original keys, so origRoot != nil
		var a, b any
		if orig == nil || orig.Decode(&a) != nil || v.Decode(&b) != nil || !reflect.DeepEqual(a, b) {
			return refuse("setting %q would change", k)
		}
	}
	return nil
}
