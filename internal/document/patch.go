package document

import (
	"errors"
	"sort"
	"strings"
)

// editOp names the kind of edit a PatchSet entry requests.
type editOp int

const (
	opSetField     editOp = iota // change an EXISTING field's value token
	opInsertField                // add an ABSENT field before the closing fence
	opReplaceBlock               // rewrite a managed block's interior
	opInsertBlock                // create a managed block at a generic insertion point
	opRemoveBlock                // delete a managed block, markers and interior
)

// BlockInsertionPoint names the only two generic insertion points this change
// needs. Which one is correct for a given record is the calling renderer's
// decision, not this package's.
type BlockInsertionPoint int

const (
	AtDocumentStart  BlockInsertionPoint = iota // byte offset 0
	AfterFrontmatter                            // immediately after the closing fence line
)

// edit is one requested, not yet resolved, mutation.
type edit struct {
	op         editOp
	name       string
	value      Value  // field ops
	content    string // block ops
	annotation string // opInsertBlock
	at         BlockInsertionPoint
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

// InsertField requests that an absent field be added immediately before the
// closing fence, using the document's line ending. Whether the key is legal or
// absent-capable is a schema question this package does not answer; it checks
// only that the name is well formed and not already present.
func (p *PatchSet) InsertField(name string, v Value) {
	p.edits = append(p.edits, edit{op: opInsertField, name: name, value: v})
}

// ReplaceBlock requests that a managed block's interior become content, leaving
// both marker lines byte-identical. content is logical LF-separated Markdown
// and is emitted with the block's own line ending; a trailing newline is
// optional and does not change the result.
func (p *PatchSet) ReplaceBlock(name string, content string) {
	p.edits = append(p.edits, edit{op: opReplaceBlock, name: name, content: content})
}

// InsertBlock requests a new managed block, both marker lines constructed
// canonically, at one of the two generic insertion points. An empty annotation
// renders a marker with no parenthesized part.
func (p *PatchSet) InsertBlock(name, annotation, content string, at BlockInsertionPoint) {
	p.edits = append(p.edits, edit{
		op: opInsertBlock, name: name, annotation: annotation, content: content, at: at})
}

// RemoveBlock requests that the named managed block be deleted in full — both
// marker lines and the interior between them — leaving every byte outside that
// marker-to-marker line range untouched. It removes exactly the block's own
// lines and does not chase the blank separation InsertBlock lays down around a
// new block. Resolution fails through Apply's error path when the block is
// absent, mirroring ReplaceBlock's missing-target error.
func (p *PatchSet) RemoveBlock(name string) {
	p.edits = append(p.edits, edit{op: opRemoveBlock, name: name})
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
		case opInsertField:
			r, err = d.resolveInsertField(e, seen)
		case opReplaceBlock:
			r, err = d.resolveReplaceBlock(e, seen)
		case opInsertBlock:
			r, err = d.resolveInsertBlock(e, seen)
		case opRemoveBlock:
			r, err = d.resolveRemoveBlock(e, seen)
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
	if err := claimName(seen, "field", e.name,
		"field is edited more than once in the same patch set"); err != nil {
		return resolvedEdit{}, err
	}
	span, payload := d.setFieldPayload(f, e.value)
	return resolvedEdit{span: span, payload: payload}, nil
}

// claimName records that this patch set now owns name within a namespace, and
// refuses a second claim. The namespace is what lets a managed block and a
// frontmatter field share a name without colliding.
func claimName(seen map[string]bool, namespace, name, msg string) error {
	key := namespace + ":" + name
	if seen[key] {
		return &Error{Kind: KindDuplicateEdit, Name: name, Offset: -1, Msg: msg}
	}
	seen[key] = true
	return nil
}

// resolveInsertField validates one InsertField edit and reduces it to a
// zero-width insertion immediately before the closing fence.
//
// A field that is already present is a duplicate-edit rather than a missing
// target: SetField and InsertField are two caller-declared modes over the same
// name, so asking for both — or asking to insert what is already there — means
// the caller's intent for that name is ambiguous, which is exactly what the
// shared "field" namespace exists to catch.
func (d Document) resolveInsertField(e edit, seen map[string]bool) (resolvedEdit, error) {
	if !d.hasFM {
		return resolvedEdit{}, &Error{Kind: KindMissingFrontmatter, Name: e.name, Offset: -1,
			Msg: "cannot insert a field into a document without frontmatter"}
	}
	if !validKey(e.name) {
		return resolvedEdit{}, &Error{Kind: KindInvalidValue, Name: e.name, Offset: -1,
			Msg: "field name does not match the Docket key grammar"}
	}
	if err := e.value.validate(); err != nil {
		return resolvedEdit{}, named(err, e.name)
	}
	if f, ok := d.Field(e.name); ok {
		return resolvedEdit{}, &Error{Kind: KindDuplicateEdit, Name: e.name, Offset: f.Entry.Start,
			Msg: "field is already present; SetField changes an existing field"}
	}
	if err := claimName(seen, "field", e.name,
		"field is edited more than once in the same patch set"); err != nil {
		return resolvedEdit{}, err
	}
	line := e.name + ":"
	if serialized := e.value.serialize(); serialized != "" {
		line += " " + serialized
	}
	at := d.fmClose.Start
	return resolvedEdit{span: Span{at, at}, payload: []byte(line + d.lineEnding)}, nil
}

// resolveReplaceBlock validates one ReplaceBlock edit and reduces it to the
// block's interior span plus the re-terminated content. Both marker lines sit
// outside the replaced span, so they survive byte-identically.
func (d Document) resolveReplaceBlock(e edit, seen map[string]bool) (resolvedEdit, error) {
	if !validBlockName(e.name) {
		return resolvedEdit{}, &Error{Kind: KindInvalidValue, Name: e.name, Offset: -1,
			Msg: "block name does not match the Docket marker grammar"}
	}
	if err := validBlockContent(e.content); err != nil {
		return resolvedEdit{}, named(err, e.name)
	}
	b, ok := d.Block(e.name)
	if !ok {
		return resolvedEdit{}, &Error{Kind: KindMissingPatchTarget, Name: e.name, Offset: -1,
			Msg: "managed block is not present in the document"}
	}
	if err := claimName(seen, "block", e.name,
		"block is edited more than once in the same patch set"); err != nil {
		return resolvedEdit{}, err
	}
	payload := renderBlockContent(e.content, d.blockLineEnding(b))
	return resolvedEdit{span: b.Interior, payload: []byte(payload)}, nil
}

// resolveRemoveBlock validates one RemoveBlock edit and reduces it to the
// deletion of the block's whole marker-to-marker line range. The span runs from
// the start marker's first byte to the end marker's last, so both markers and
// the interior go and nothing outside them moves; a nil payload is the deletion.
func (d Document) resolveRemoveBlock(e edit, seen map[string]bool) (resolvedEdit, error) {
	if !validBlockName(e.name) {
		return resolvedEdit{}, &Error{Kind: KindInvalidValue, Name: e.name, Offset: -1,
			Msg: "block name does not match the Docket marker grammar"}
	}
	b, ok := d.Block(e.name)
	if !ok {
		return resolvedEdit{}, &Error{Kind: KindMissingPatchTarget, Name: e.name, Offset: -1,
			Msg: "managed block is not present in the document"}
	}
	if err := claimName(seen, "block", e.name,
		"block is edited more than once in the same patch set"); err != nil {
		return resolvedEdit{}, err
	}
	return resolvedEdit{span: Span{b.Start.Start, b.End.End}, payload: nil}, nil
}

// resolveInsertBlock validates one InsertBlock edit and reduces it to a
// zero-width insertion of a complete, canonically constructed block.
func (d Document) resolveInsertBlock(e edit, seen map[string]bool) (resolvedEdit, error) {
	if !validBlockName(e.name) {
		return resolvedEdit{}, &Error{Kind: KindInvalidValue, Name: e.name, Offset: -1,
			Msg: "block name does not match the Docket marker grammar"}
	}
	if err := validAnnotation(e.annotation); err != nil {
		return resolvedEdit{}, named(err, e.name)
	}
	if err := validBlockContent(e.content); err != nil {
		return resolvedEdit{}, named(err, e.name)
	}
	if b, ok := d.Block(e.name); ok {
		return resolvedEdit{}, &Error{Kind: KindDuplicateEdit, Name: e.name, Offset: b.Start.Start,
			Msg: "block is already present; ReplaceBlock rewrites an existing block"}
	}
	at, err := d.insertionOffset(e)
	if err != nil {
		return resolvedEdit{}, err
	}
	if err := claimName(seen, "block", e.name,
		"block is edited more than once in the same patch set"); err != nil {
		return resolvedEdit{}, err
	}
	le := d.lineEnding
	payload := startMarkerLine(e.name, e.annotation) + le +
		renderBlockContent(e.content, le) +
		endMarkerLine(e.name) + le
	return resolvedEdit{span: Span{at, at}, payload: []byte(payload)}, nil
}

// insertionOffset resolves a BlockInsertionPoint against this document.
//
// AtDocumentStart is refused on a document that HAS frontmatter, and that
// refusal is load-bearing rather than fussy: bytes written in front of the
// opening fence demote real frontmatter to ordinary prose, and the result still
// parses — as a different, frontmatterless document — so the phase-3 reparse
// gate would wave the corruption through.
func (d Document) insertionOffset(e edit) (int, error) {
	switch e.at {
	case AtDocumentStart:
		if d.hasFM {
			return 0, &Error{Kind: KindUnsupportedPatchShape, Name: e.name, Offset: 0,
				Msg: "a document with frontmatter cannot receive a block at its start; " +
					"use AfterFrontmatter"}
		}
		return 0, nil
	case AfterFrontmatter:
		if !d.hasFM {
			return 0, &Error{Kind: KindMissingFrontmatter, Name: e.name, Offset: -1,
				Msg: "document has no frontmatter to insert after"}
		}
		return d.fmClose.End, nil
	default:
		return 0, &Error{Kind: KindUnsupportedPatchShape, Name: e.name, Offset: -1,
			Msg: "unknown block insertion point"}
	}
}

// renderBlockContent turns logical LF-separated content into terminated lines
// using ending. Empty content yields an empty interior, and a caller's optional
// trailing newline is absorbed so both spellings of the same logical content
// produce the same bytes.
func renderBlockContent(content, ending string) string {
	if content == "" {
		return ""
	}
	content = strings.TrimSuffix(content, "\n")
	var b strings.Builder
	for _, line := range strings.Split(content, "\n") {
		b.WriteString(line)
		b.WriteString(ending)
	}
	return b.String()
}

// blockLineEnding reports the line ending b's own marker lines use, so replaced
// content matches the block it lands in rather than the document average. The
// start marker is always terminated — an end marker follows it — so only a
// document-level fallback for an impossible case is needed.
func (d Document) blockLineEnding(b Block) string {
	switch {
	case b.Start.End-b.Start.Start >= 2 && d.source[b.Start.End-2] == '\r' &&
		d.source[b.Start.End-1] == '\n':
		return "\r\n"
	case b.Start.End > b.Start.Start && d.source[b.Start.End-1] == '\n':
		return "\n"
	default:
		return d.lineEnding
	}
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
