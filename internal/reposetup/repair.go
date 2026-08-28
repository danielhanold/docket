package reposetup

// repair.go — the closed, mechanically-safe frontmatter repair roster.
//
// A repair is admitted only when it is decidable without judgement: the record
// has exactly one balanced, uniquely-located frontmatter block; the target key
// is unique; the schema admits exactly one canonical replacement; the patch
// touches only the located value span (or, for a removal, the key's own line),
// leaving the authored body and every unknown field byte-identical; and the
// re-parsed candidate proves the intended domain postcondition. Anything that
// fails one of those gates is reported as a NON-repairable finding, never
// silently repaired.
//
// The three roster entries are:
//
//	quote-unsafe-scalar   a plain string scalar whose decoded text is an
//	                      unambiguous string but whose token SHAPE violates the
//	                      scalar-safety rule (boolean keyword, leading YAML
//	                      indicator, ": ", trailing ":", or " #") — single-quoted
//	                      with its decoded text unchanged.
//	scalar-to-list        a known id-list field stored as ONE scalar that itself
//	                      parses as an exact valid sequence of the item type —
//	                      re-emitted as an UNQUOTED flow sequence, same items,
//	                      same order.
//	drop-terminal-claimed-at   claimed_at removed from an already-terminal
//	                      (done/killed) archived change.
//
// Byte edits are produced by internal/document's own PatchSet/serializer, so
// YAML validity and the reparse gate are construction properties, not hopes.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"go.yaml.in/yaml/v3"
)

// RepairCode names a member of the closed repair roster. It is empty on a
// non-repairable finding.
type RepairCode string

const (
	RepairQuoteScalar   RepairCode = "quote-unsafe-scalar"
	RepairScalarToList  RepairCode = "scalar-to-list"
	RepairDropClaimedAt RepairCode = "drop-terminal-claimed-at"
)

// RepairFinding is one diagnosis about a single change record. A repairable
// finding names a roster entry and carries a unified-diff-style preview; a
// non-repairable finding names something the roster deliberately will not touch.
type RepairFinding struct {
	Path       string     // repo-relative record path
	Field      string     // the frontmatter field the finding is about, if any
	Code       RepairCode // empty for non-repairable findings
	Repairable bool
	Message    string
	Patch      []byte // preview for repairable findings; nil otherwise
}

// changeListFields is the set of change-manifest frontmatter fields the decode
// layer reads as an id SEQUENCE: internal/repository/decode.go's changeWire
// scalarList fields DependsOn, Related, DiscoveredFrom, ADRs, each converted by
// (*decoder).intList. It is transcribed here — not imported — because
// internal/repository exports no roster symbol; the parity is anchored on those
// symbol names so drift is greppable (ADR-0054). blocked_by is deliberately
// ABSENT: decode.go reads it as a scalar OptionalString (wire field BlockedBy,
// converted by optionalString), so converting it to a sequence would corrupt
// its type.
var changeListFields = map[string]bool{
	"depends_on":      true,
	"related":         true,
	"discovered_from": true,
	"adrs":            true,
}

// changeStringFields is the set of change-manifest fields the decode layer reads
// as free-text strings: decode.go's decodeChange (*decoder).text /
// (*decoder).optionalString call sites (slug, title, type, spec, plan, results,
// branch_prefix, branch, pr, issue, blocked_by). Quoting is type-safe only for
// these — quoting an int/date/bool field would change its decoded type — so the
// quote roster is gated on membership here.
var changeStringFields = map[string]bool{
	"slug": true, "title": true, "type": true, "spec": true, "plan": true,
	"results": true, "branch_prefix": true, "branch": true, "pr": true,
	"issue": true, "blocked_by": true,
}

// PlanRepairs inspects one change record's bytes and returns the ordered set of
// findings: repairable findings for exactly the closed roster, and
// non-repairable findings for the shapes it can name but must not touch. An
// undecodable frontmatter block (including a duplicate key, which document.Parse
// rejects) yields a single non-repairable finding naming the whole record.
// archived enables RepairDropClaimedAt, and only for a terminal-status record.
func PlanRepairs(path string, src []byte, archived bool) ([]RepairFinding, error) {
	doc, err := document.Parse(src)
	if err != nil {
		return []RepairFinding{{
			Path:       path,
			Repairable: false,
			Message:    "record frontmatter is undecodable: " + err.Error(),
		}}, nil
	}
	if !doc.HasFrontmatter() {
		return nil, nil
	}

	var findings []RepairFinding
	for _, f := range doc.Fields() {
		switch {
		case changeStringFields[f.Name]:
			if fnd, ok := planQuote(path, src, doc, f); ok {
				findings = append(findings, fnd)
			}
		case changeListFields[f.Name]:
			if fnd, ok := planList(path, src, doc, f); ok {
				findings = append(findings, fnd)
			}
		}
	}
	if fnd, ok := planClaimedAt(path, src, doc, archived); ok {
		findings = append(findings, fnd)
	}
	return findings, nil
}

// planQuote decides the quote-unsafe-scalar roster entry for one located field.
func planQuote(path string, src []byte, doc document.Document, f document.Field) (RepairFinding, bool) {
	// A flow collection or a block/multi-line scalar is not an inline scalar and
	// is never quoted.
	if f.Shape != document.ShapeInline {
		return RepairFinding{}, false
	}
	token := string(src[f.Value.Start:f.Value.End])
	if !unsafeScalarShape(token) {
		return RepairFinding{}, false // already safe: nothing to repair
	}
	if !decodesToStringLiteral(token) {
		// The token's shape is unsafe but its decoded value is not an unambiguous
		// string (a real bool/int, or YAML rewrites it): quoting would change the
		// value, so this is a manual-review finding, not a repair.
		return RepairFinding{
			Path:       path,
			Field:      f.Name,
			Repairable: false,
			Message:    "unsafe scalar shape with ambiguous decoded value; needs manual review",
		}, true
	}
	candidate, preview, err := buildRepair(doc, src, RepairQuoteScalar, f.Name)
	if err != nil {
		return RepairFinding{}, false
	}
	_ = candidate
	return RepairFinding{
		Path:       path,
		Field:      f.Name,
		Code:       RepairQuoteScalar,
		Repairable: true,
		Message:    "unsafe unquoted scalar; single-quote with decoded text unchanged",
		Patch:      preview,
	}, true
}

// planList decides the scalar-to-list roster entry for one located field.
func planList(path string, src []byte, doc document.Document, f document.Field) (RepairFinding, bool) {
	// Already a flow sequence: nothing to convert.
	if f.Shape != document.ShapeInline {
		return RepairFinding{}, false
	}
	token := string(src[f.Value.Start:f.Value.End])
	if _, ok := scalarAsIntSeq(token); !ok {
		return RepairFinding{
			Path:       path,
			Field:      f.Name,
			Repairable: false,
			Message:    "list field stored as a scalar that is not an exact id sequence; needs manual review",
		}, true
	}
	candidate, preview, err := buildRepair(doc, src, RepairScalarToList, f.Name)
	if err != nil {
		return RepairFinding{}, false
	}
	_ = candidate
	return RepairFinding{
		Path:       path,
		Field:      f.Name,
		Code:       RepairScalarToList,
		Repairable: true,
		Message:    "id-list field stored as a scalar; convert to an unquoted flow sequence",
		Patch:      preview,
	}, true
}

// planClaimedAt decides the drop-terminal-claimed-at roster entry.
func planClaimedAt(path string, src []byte, doc document.Document, archived bool) (RepairFinding, bool) {
	f, ok := doc.Field("claimed_at")
	if !ok {
		return RepairFinding{}, false
	}
	if !archived {
		// An active record legitimately holds a claim lease; leave it alone.
		return RepairFinding{}, false
	}
	if !terminalStatus(doc) {
		return RepairFinding{
			Path:       path,
			Field:      "claimed_at",
			Repairable: false,
			Message:    "claimed_at on a non-terminal archived record; needs manual review",
		}, true
	}
	_ = f
	candidate, preview, err := buildRepair(doc, src, RepairDropClaimedAt, "claimed_at")
	if err != nil {
		return RepairFinding{}, false
	}
	_ = candidate
	return RepairFinding{
		Path:       path,
		Field:      "claimed_at",
		Code:       RepairDropClaimedAt,
		Repairable: true,
		Message:    "claimed_at on a terminal archived change; remove the stale claim lease",
		Patch:      preview,
	}, true
}

// terminalStatus reports whether the record's decoded status is a terminal end
// state (done or killed).
func terminalStatus(doc document.Document) bool {
	var head struct {
		Status string `yaml:"status"`
	}
	if err := doc.DecodeFrontmatter(&head); err != nil {
		return false
	}
	st, ok := domain.ParseStatus(head.Status)
	return ok && st.Terminal()
}

// booleanKeywords are the scalar spellings the house rule requires quoting
// because a YAML 1.1 reader would coerce them to booleans.
var booleanKeywords = map[string]bool{
	"true": true, "false": true, "yes": true, "no": true, "on": true, "off": true,
}

// unsafeScalarShape reports whether an unquoted scalar token violates the
// scalar-safety rule (CLAUDE.md "Frontmatter and generated blocks"): a boolean
// keyword, a leading YAML indicator character, an embedded ": ", a trailing ":",
// or an embedded " #". The check is keyed on shape, never on a field-value
// allowlist.
func unsafeScalarShape(token string) bool {
	if token == "" {
		return false
	}
	if booleanKeywords[strings.ToLower(token)] {
		return true
	}
	if strings.Contains(token, ": ") || strings.HasSuffix(token, ":") {
		return true
	}
	if strings.Contains(token, " #") {
		return true
	}
	return isYAMLIndicator(token[0])
}

// isYAMLIndicator reports whether b is one of YAML's c-indicator bytes, which a
// plain scalar may not safely begin with.
func isYAMLIndicator(b byte) bool {
	switch b {
	case '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|',
		'>', '\'', '"', '%', '@', '`':
		return true
	}
	return false
}

// decodesToStringLiteral reports whether token, decoded on its own, is a string
// equal to its own bytes — i.e. YAML reads it as exactly that text, with no type
// coercion and no escape rewriting. This is the "unambiguous decoded text" gate:
// a token that decodes to a bool/int (title: true) or that YAML rewrites is
// refused, because single-quoting it would change the record's value.
func decodesToStringLiteral(token string) bool {
	var v any
	if err := yaml.Unmarshal([]byte(token), &v); err != nil {
		return false
	}
	s, ok := v.(string)
	return ok && s == token
}

// scalarAsIntSeq parses token as if it were the interior of a flow sequence and
// returns the item ids when every item is a plain integer scalar. It mirrors the
// decode layer's (*decoder).intList item rule (strconv.Atoi over scalar items):
// a wrong-typed item (foo) or a partial sequence (3, foo) yields ok=false.
func scalarAsIntSeq(token string) ([]int, bool) {
	var seq []yaml.Node
	if err := yaml.Unmarshal([]byte("["+token+"]"), &seq); err != nil {
		return nil, false
	}
	if len(seq) == 0 {
		return nil, false
	}
	out := make([]int, 0, len(seq))
	for i := range seq {
		n := &seq[i]
		if n.Kind != yaml.ScalarNode {
			return nil, false
		}
		id, err := strconv.Atoi(strings.TrimSpace(n.Value))
		if err != nil {
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}

// buildRepair re-derives the canonical edit for (code, field) directly from doc
// and src, applies it, proves the intended postcondition on the re-parsed
// candidate, and returns the candidate bytes plus the unified-diff preview. It
// is the single source of truth shared by PlanRepairs (which keeps the preview)
// and ApplyRepairs (which re-derives it to detect a tampered finding). An error
// means the finding does not correspond to a real, safe repair.
func buildRepair(doc document.Document, src []byte, code RepairCode, field string) (candidate, preview []byte, err error) {
	f, ok := doc.Field(field)
	if !ok {
		return nil, nil, fmt.Errorf("reposetup: field %q is not located", field)
	}
	oldToken := string(src[f.Value.Start:f.Value.End])

	switch code {
	case RepairQuoteScalar:
		if f.Shape != document.ShapeInline || !unsafeScalarShape(oldToken) || !decodesToStringLiteral(oldToken) {
			return nil, nil, fmt.Errorf("reposetup: field %q is not a quote-unsafe scalar", field)
		}
		cand, err := applyValue(doc, field, document.String(oldToken))
		if err != nil {
			return nil, nil, err
		}
		if err := verifyStringUnchanged(cand, field, oldToken); err != nil {
			return nil, nil, err
		}
		return cand, diffPreview(field, oldToken, tokenOf(cand, field)), nil

	case RepairScalarToList:
		ids, ok := scalarAsIntSeq(oldToken)
		if f.Shape != document.ShapeInline || !ok {
			return nil, nil, fmt.Errorf("reposetup: field %q is not a scalar-encoded id list", field)
		}
		items := make([]document.Value, len(ids))
		for i, id := range ids {
			items[i] = document.Int(int64(id))
		}
		cand, err := applyValue(doc, field, document.Seq(items...))
		if err != nil {
			return nil, nil, err
		}
		if err := verifyIntSeq(cand, field, ids); err != nil {
			return nil, nil, err
		}
		return cand, diffPreview(field, oldToken, tokenOf(cand, field)), nil

	case RepairDropClaimedAt:
		if field != "claimed_at" {
			return nil, nil, fmt.Errorf("reposetup: drop applies only to claimed_at")
		}
		cand := spliceOut(src, f.Entry)
		if _, perr := document.Parse(cand); perr != nil {
			return nil, nil, fmt.Errorf("reposetup: claimed_at removal did not reparse: %w", perr)
		}
		if fieldStillPresent(cand, "claimed_at") {
			return nil, nil, fmt.Errorf("reposetup: claimed_at still present after removal")
		}
		return cand, []byte("-" + field + ": " + oldToken + "\n"), nil

	default:
		return nil, nil, fmt.Errorf("reposetup: unknown repair code %q", code)
	}
}

// applyValue rewrites field's value token to v through document's own PatchSet,
// which validates the edit and reparses the candidate before returning bytes.
func applyValue(doc document.Document, field string, v document.Value) ([]byte, error) {
	var ps document.PatchSet
	ps.SetField(field, v)
	return doc.Apply(ps)
}

// tokenOf returns the located value-token bytes of field in candidate, so the
// preview reflects document's real serializer output.
func tokenOf(candidate []byte, field string) string {
	cdoc, err := document.Parse(candidate)
	if err != nil {
		return ""
	}
	f, ok := cdoc.Field(field)
	if !ok {
		return ""
	}
	return string(candidate[f.Value.Start:f.Value.End])
}

// diffPreview renders the two-line unified-diff-style preview for a value edit.
func diffPreview(field, oldToken, newToken string) []byte {
	return []byte("-" + field + ": " + oldToken + "\n+" + field + ": " + newToken + "\n")
}

// verifyStringUnchanged proves the quote postcondition: field decodes to exactly
// the same string it named before the quote.
func verifyStringUnchanged(candidate []byte, field, want string) error {
	got, ok := decodedString(candidate, field)
	if !ok || got != want {
		return fmt.Errorf("reposetup: quote changed the decoded value of %q", field)
	}
	return nil
}

// verifyIntSeq proves the list postcondition: field decodes to exactly want, in
// order.
func verifyIntSeq(candidate []byte, field string, want []int) error {
	cdoc, err := document.Parse(candidate)
	if err != nil {
		return err
	}
	var m map[string]yaml.Node
	if err := cdoc.DecodeFrontmatter(&m); err != nil {
		return err
	}
	node, ok := m[field]
	if !ok || node.Kind != yaml.SequenceNode || len(node.Content) != len(want) {
		return fmt.Errorf("reposetup: list conversion of %q did not yield the expected sequence", field)
	}
	for i, child := range node.Content {
		id, err := strconv.Atoi(strings.TrimSpace(child.Value))
		if err != nil || child.Kind != yaml.ScalarNode || id != want[i] {
			return fmt.Errorf("reposetup: list conversion of %q changed item %d", field, i)
		}
	}
	return nil
}

// decodedString decodes field from candidate as a string; ok is false when the
// field is absent or not a string.
func decodedString(candidate []byte, field string) (string, bool) {
	cdoc, err := document.Parse(candidate)
	if err != nil {
		return "", false
	}
	var m map[string]yaml.Node
	if err := cdoc.DecodeFrontmatter(&m); err != nil {
		return "", false
	}
	node, ok := m[field]
	if !ok || node.Kind != yaml.ScalarNode {
		return "", false
	}
	var s string
	if err := node.Decode(&s); err != nil {
		return "", false
	}
	return s, true
}

// fieldStillPresent reports whether field survives in candidate's frontmatter.
func fieldStillPresent(candidate []byte, field string) bool {
	cdoc, err := document.Parse(candidate)
	if err != nil {
		return true // fail closed: an unreadable candidate is not proof of removal
	}
	_, ok := cdoc.Field(field)
	return ok
}

// spliceOut returns src with the half-open byte range span removed.
func spliceOut(src []byte, span document.Span) []byte {
	out := make([]byte, 0, len(src)-(span.End-span.Start))
	out = append(out, src[:span.Start]...)
	out = append(out, src[span.End:]...)
	return out
}

// ApplyRepairs applies the given repairable findings to src and returns the
// repaired bytes. Each finding is re-derived from the current bytes and its
// preview is compared against the stored patch, so a tampered finding is
// rejected; every intended postcondition is re-proven on the re-parsed
// candidate. Any failure returns an error and no bytes.
func ApplyRepairs(src []byte, findings []RepairFinding) ([]byte, error) {
	cur := append([]byte(nil), src...)
	for _, f := range findings {
		if !f.Repairable {
			return nil, fmt.Errorf("reposetup: ApplyRepairs given a non-repairable finding for %q", f.Path)
		}
		doc, err := document.Parse(cur)
		if err != nil {
			return nil, fmt.Errorf("reposetup: candidate no longer parses: %w", err)
		}
		candidate, preview, err := buildRepair(doc, cur, f.Code, f.Field)
		if err != nil {
			return nil, err
		}
		if string(preview) != string(f.Patch) {
			return nil, fmt.Errorf("reposetup: finding patch for %q does not match the canonical repair (tampered?)", f.Field)
		}
		cur = candidate
	}
	return cur, nil
}

// RepairDigest is the stable sha256 hex digest over the canonical encoding of an
// ordered repair plan: for each repairable finding, in order, its path, field,
// code, and patch bytes. Non-repairable findings are diagnostics, not plan
// members, and do not contribute.
func RepairDigest(findings []RepairFinding) string {
	h := sha256.New()
	for _, f := range findings {
		if !f.Repairable {
			continue
		}
		// Length-prefix each variable-length field so no concatenation of one
		// plan can alias another.
		fmt.Fprintf(h, "%d:%s\x00%d:%s\x00%d:%s\x00%d:",
			len(f.Path), f.Path, len(f.Field), f.Field, len(f.Code), f.Code, len(f.Patch))
		h.Write(f.Patch)
		h.Write([]byte{0x1e})
	}
	return hex.EncodeToString(h.Sum(nil))
}
