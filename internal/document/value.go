package document

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Value is the closed frontmatter-value model. Callers never provide raw YAML:
// every patched or rendered value is built from these constructors and emitted
// by the one shared serializer, which is what makes YAML validity a
// construction property rather than a runtime hope.
type Value struct {
	kind valueKind
	str  string
	num  int64
	b    bool
	seq  []Value
}

type valueKind int

const (
	kindNull valueKind = iota
	kindString
	kindInt
	kindBool
	kindSeq
)

// Null returns the null value, rendered as the bare "key:" form.
func Null() Value { return Value{kind: kindNull} }

// String returns a string value. It is always emitted single-quoted.
func String(s string) Value { return Value{kind: kindString, str: s} }

// Int returns a base-10 integer value.
func Int(i int64) Value { return Value{kind: kindInt, num: i} }

// Bool returns a boolean value, rendered as true or false.
func Bool(b bool) Value { return Value{kind: kindBool, b: b} }

// Seq returns a flow sequence of scalar values. Nested sequences are outside
// the closed model and are rejected by validate.
func Seq(items ...Value) Value {
	return Value{kind: kindSeq, seq: append([]Value(nil), items...)}
}

var keyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validKey reports whether name matches the Docket key grammar.
func validKey(name string) bool { return keyRE.MatchString(name) }

// invalidValue builds a KindInvalidValue error.
func invalidValue(msg string) error {
	return &Error{Kind: KindInvalidValue, Offset: -1, Msg: msg}
}

// illegalTextRune reports whether r may never appear in a field string or in
// managed-block content, apart from the per-context exemptions (tab, and LF for
// block content) the callers grant before consulting it.
//
// Two disjoint reasons put a rune here. Unicode control characters (category
// Cc) are refused by the closed model itself — "all control characters except
// tab". U+FFFE and U+FFFF are refused because the reader in
// go.yaml.in/yaml/v3 v3.0.4 admits only "[#xE000-#xFFFD]" in that plane, so a
// value carrying one renders a document that fails its own reparse. Note the
// asymmetry the YAML set creates: U+0085 (NEL) is legal YAML but a Cc control,
// so the closed model still refuses it; U+1FFFE is a non-character but sits in
// the allowed "[#x10000-#x10FFFF]" range, so it stays legal.
func illegalTextRune(r rune) bool {
	return unicode.IsControl(r) || r == 0xfffe || r == 0xffff
}

// validate reports whether v is representable in the closed model. The whole
// value — every element of a sequence included — is checked before any caller
// serializes a byte, so a defective tail item cannot yield a partial document.
func (v Value) validate() error {
	switch v.kind {
	case kindString:
		if !utf8.ValidString(v.str) {
			return invalidValue("string is not valid UTF-8")
		}
		for _, r := range v.str {
			if r == '\t' {
				continue
			}
			if illegalTextRune(r) {
				return invalidValue(fmt.Sprintf("character %q is not permitted in a string", r))
			}
		}
	case kindSeq:
		for _, item := range v.seq {
			if item.kind == kindSeq {
				return invalidValue("nested sequence")
			}
			if err := item.validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// serialize renders the canonical inline YAML token for v.
//
// A top-level null renders as the empty string: the caller writes "key:" with
// no trailing space. A null *sequence element* renders as the explicit null
// keyword instead. That choice was settled empirically against
// go.yaml.in/yaml/v3 v3.0.4: it reads the trailing-empty flow form
// "['a', true, ]" as a two-element sequence — the trailing comma is tolerated
// and the empty token vanishes rather than decoding as a nil tail — so only the
// explicit keyword survives the round trip. This asymmetry is pinned by
// TestNullSequenceElementRoundTrips and TestTopLevelNullRendersEmpty.
func (v Value) serialize() string {
	switch v.kind {
	case kindNull:
		return ""
	case kindString:
		return "'" + strings.ReplaceAll(v.str, "'", "''") + "'"
	case kindInt:
		return strconv.FormatInt(v.num, 10)
	case kindBool:
		return strconv.FormatBool(v.b)
	case kindSeq:
		parts := make([]string, len(v.seq))
		for i, item := range v.seq {
			if item.kind == kindNull {
				parts[i] = "null"
				continue
			}
			parts[i] = item.serialize()
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}

// validBlockContent reports whether s is legal managed-block content: valid
// UTF-8, and no rune illegalTextRune refuses other than LF and tab — which
// covers NUL and every other control character.
func validBlockContent(s string) error {
	if !utf8.ValidString(s) {
		return invalidValue("block content is not valid UTF-8")
	}
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if illegalTextRune(r) {
			return invalidValue(fmt.Sprintf("character %q is not permitted in block content", r))
		}
	}
	return nil
}
