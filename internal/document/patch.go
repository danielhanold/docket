package document

import (
	"errors"
	"sort"
)

// editOp names the kind of edit a PatchSet entry requests.
type editOp int

const (
	opSetField editOp = iota // change an EXISTING field's value token
)

// edit is one requested, not yet resolved, mutation.
type edit struct {
	op    editOp
	name  string
	value Value
}

// PatchSet is an ordered collection of requested edits. The zero value is an
// empty set; Apply on an empty set returns byte-identical content.
type PatchSet struct {
	edits []edit
}

// SetField requests that an existing field's value token become v. The field
// must already exist and be written in a patchable shape; both are checked by
// Apply, not here, so a PatchSet can be built without a document in hand.
func (p *PatchSet) SetField(name string, v Value) {
	p.edits = append(p.edits, edit{op: opSetField, name: name, value: v})
}

// resolvedEdit is one validated edit reduced to bytes: replace span with
// payload. A zero-width span is an insertion; a nil payload is a deletion.
type resolvedEdit struct {
	span    Span
	payload []byte
}

// Apply returns a fresh copy of the document's bytes with every edit in p
// applied, or nil and a *Error.
//
// The pipeline is three phases, and the order is the safety property: phase one
// validates EVERY edit and resolves it to a span and a payload before a single
// byte is constructed, so a defect in a later batch item leaves the input
// untouched; phase two splices the plan from the highest offset downward, so no
// earlier replacement shifts a later span; phase three reparses the candidate
// under the same rules Parse enforces, so no patch can hand back bytes this
// package would refuse to read back.
func (d Document) Apply(p PatchSet) ([]byte, error) {
	plan, err := d.resolve(p)
	if err != nil {
		return nil, err
	}
	if err := checkOverlaps(plan); err != nil {
		return nil, err
	}
	return d.applyResolved(plan)
}

// resolve validates every requested edit and reduces it to bytes. The returned
// plan is in span order, ascending.
func (d Document) resolve(p PatchSet) ([]resolvedEdit, error) {
	plan := make([]resolvedEdit, 0, len(p.edits))
	seen := make(map[string]bool, len(p.edits))
	for _, e := range p.edits {
		var (
			r   resolvedEdit
			err error
		)
		switch e.op {
		case opSetField:
			r, err = d.resolveSetField(e, seen)
		default:
			err = &Error{Kind: KindUnsupportedPatchShape, Name: e.name, Offset: -1,
				Msg: "unknown patch operation"}
		}
		if err != nil {
			return nil, err
		}
		plan = append(plan, r)
	}
	sort.SliceStable(plan, func(i, j int) bool { return plan[i].span.Start < plan[j].span.Start })
	return plan, nil
}

// resolveSetField validates one SetField edit against the document and returns
// the span it replaces plus the bytes that replace it.
func (d Document) resolveSetField(e edit, seen map[string]bool) (resolvedEdit, error) {
	if !d.hasFM {
		return resolvedEdit{}, &Error{Kind: KindMissingFrontmatter, Name: e.name, Offset: -1,
			Msg: "cannot patch a field on a document without frontmatter"}
	}
	if !validKey(e.name) {
		return resolvedEdit{}, &Error{Kind: KindInvalidValue, Name: e.name, Offset: -1,
			Msg: "field name does not match the Docket key grammar"}
	}
	if err := e.value.validate(); err != nil {
		return resolvedEdit{}, named(err, e.name)
	}
	f, ok := d.Field(e.name)
	if !ok {
		return resolvedEdit{}, &Error{Kind: KindMissingPatchTarget, Name: e.name, Offset: -1,
			Msg: "field is not present in the frontmatter"}
	}
	if f.Shape == ShapeUnsupported {
		return resolvedEdit{}, &Error{Kind: KindUnsupportedPatchShape, Name: e.name,
			Offset: f.Entry.Start,
			Msg:    "field value is not written in a patchable shape"}
	}
	key := "field:" + e.name
	if seen[key] {
		return resolvedEdit{}, &Error{Kind: KindDuplicateEdit, Name: e.name, Offset: -1,
			Msg: "field is edited more than once in the same patch set"}
	}
	seen[key] = true
	span, payload := d.setFieldPayload(f, e.value)
	return resolvedEdit{span: span, payload: payload}, nil
}

// named returns err with the field or block name attached, when err is one of
// this package's errors. Value validation has no document context of its own,
// so the name is filled in where that context exists.
func named(err error, name string) error {
	var de *Error
	if !errors.As(err, &de) {
		return err
	}
	clone := *de
	clone.Name = name
	return &clone
}

// setFieldPayload reduces a SetField edit on a located field to the span it
// replaces and the bytes that replace it.
//
// Two spacing rules earn their complexity, and both exist so the patched line
// still reparses as the value that was asked for:
//
//   - Writing into an empty value ("key:") inserts at a zero-width point. That
//     point sits after whatever spacing follows the colon — which, on
//     "key:   # note", is the '#' itself. A bare payload there would yield
//     "key:   211# note", a plain scalar reading "211# note", so the payload
//     keeps a space on whichever side has none.
//   - Writing null onto a field that HAS a value token drops the token and the
//     spacing before it, giving the bare "key:" form — but only when nothing
//     follows on the line. Eating the spacing in front of an inline comment
//     would produce "key:# note", where the '#' no longer opens a comment.
func (d Document) setFieldPayload(f Field, v Value) (Span, []byte) {
	serialized := v.serialize()

	if f.Shape == ShapeEmpty {
		at := f.Value.Start
		if serialized == "" {
			return Span{at, at}, nil // null onto an empty field: a legal no-op
		}
		payload := serialized
		if at > 0 && d.source[at-1] == ':' {
			payload = " " + payload
		}
		if textFollows(d.source, at) {
			payload += " "
		}
		return Span{at, at}, []byte(payload)
	}

	if serialized == "" && !textFollows(d.source, f.Value.End) {
		start := f.Value.Start
		for start > 0 && (d.source[start-1] == ' ' || d.source[start-1] == '\t') {
			start--
		}
		return Span{start, f.Value.End}, nil
	}
	return f.Value, []byte(serialized)
}

// textFollows reports whether any non-spacing byte sits between from and the
// end of that line. On a value span's boundary the only thing that can be there
// is an inline comment.
func textFollows(src []byte, from int) bool {
	for i := from; i < len(src); i++ {
		switch src[i] {
		case '\n':
			return false
		case '\r':
			// Only CRLF terminates a line; a bare CR is line content.
			return !(i+1 < len(src) && src[i+1] == '\n')
		case ' ', '\t':
			continue
		default:
			return true
		}
	}
	return false
}

// checkOverlaps refuses a plan whose spans collide. Adjacent spans may touch —
// one edit ending exactly where the next begins is well defined — but a real
// overlap, or two insertions competing for the same zero-width point, has no
// defined result and is refused rather than ordered arbitrarily. plan must be
// sorted ascending by span start.
func checkOverlaps(plan []resolvedEdit) error {
	for i := 1; i < len(plan); i++ {
		prev, cur := plan[i-1].span, plan[i].span
		collides := cur.Start < prev.End ||
			(cur.Start == prev.Start && cur.Start == cur.End && prev.Start == prev.End)
		if collides {
			return &Error{Kind: KindOverlappingEdit, Offset: cur.Start,
				Msg: "two edits claim overlapping byte ranges"}
		}
	}
	return nil
}

// applyResolved runs phases two and three over an already validated plan: the
// splice, then the candidate reparse gate. It is separate from Apply so a test
// can drive the gate with a payload the public constructors cannot produce.
func (d Document) applyResolved(plan []resolvedEdit) ([]byte, error) {
	out := d.splice(plan)
	if _, err := Parse(out); err != nil {
		return nil, &Error{Kind: KindReparseFailed, Offset: -1,
			Msg: "patched candidate failed reparse: " + err.Error()}
	}
	return out, nil
}

// splice writes the plan onto a fresh copy of the source, highest offset first
// so no applied edit shifts a span that has not been applied yet. plan must be
// sorted ascending by span start.
func (d Document) splice(plan []resolvedEdit) []byte {
	out := append([]byte(nil), d.source...)
	for i := len(plan) - 1; i >= 0; i-- {
		e := plan[i]
		next := make([]byte, 0, len(out)-(e.span.End-e.span.Start)+len(e.payload))
		next = append(next, out[:e.span.Start]...)
		next = append(next, e.payload...)
		next = append(next, out[e.span.End:]...)
		out = next
	}
	return out
}
