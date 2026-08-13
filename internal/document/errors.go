package document

import (
	"errors"
	"fmt"
)

// Kind is a stable package-local error kind. The set is CLOSED — the design
// spec's "Error model and mutation safety" section names each required kind,
// and no other kind may be minted.
type Kind string

const (
	KindMissingFrontmatter    Kind = "missing-frontmatter"  // DecodeFrontmatter/patch on a doc without frontmatter
	KindUnclosedFrontmatter   Kind = "unclosed-frontmatter" // opener without closer
	KindInvalidUTF8           Kind = "invalid-utf8"
	KindInvalidYAML           Kind = "invalid-yaml"     // parse failure, multi-doc, non-mapping root, unresolved alias
	KindDuplicateField        Kind = "duplicate-field"  // duplicate mapping key (parse) or duplicate builder key
	KindMalformedMarker       Kind = "malformed-marker" // grammar failure on a docket-marker-shaped line
	KindMarkerImbalance       Kind = "marker-imbalance" // dangling / out-of-order / nested / duplicate markers
	KindMissingPatchTarget    Kind = "missing-patch-target"
	KindUnsupportedPatchShape Kind = "unsupported-patch-shape"
	KindInvalidValue          Kind = "invalid-value" // bad key grammar, control chars, invalid UTF-8 in a value
	KindDuplicateEdit         Kind = "duplicate-edit"
	KindOverlappingEdit       Kind = "overlapping-edit"
	KindReparseFailed         Kind = "reparse-failed" // candidate failed the same parse rules
)

// Error is the package's one error type.
type Error struct {
	Kind   Kind
	Name   string // field or block name when applicable; "" otherwise
	Offset int    // byte offset when available; -1 otherwise
	Line   int    // 1-based; 0 when unavailable
	Column int    // 1-based; 0 when unavailable
	Msg    string
}

// Error renders "<kind>: <msg>", plus " (<name>)" and " at line L col C" when set.
func (e *Error) Error() string {
	s := string(e.Kind) + ": " + e.Msg
	if e.Name != "" {
		s += " (" + e.Name + ")"
	}
	if e.Line > 0 {
		s += fmt.Sprintf(" at line %d col %d", e.Line, e.Column)
	}
	return s
}

// IsKind reports whether err is a *Error carrying kind.
func IsKind(err error, kind Kind) bool {
	var de *Error
	return errors.As(err, &de) && de.Kind == kind
}
