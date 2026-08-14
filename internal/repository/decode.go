package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
)

// Decoding finding codes. Every code is a stable lowercase-hyphen token.
const (
	// CodeRecordUndecodable marks a supplied document whose frontmatter could
	// not be decoded at all; the record is retained by path, never dropped.
	CodeRecordUndecodable = "record-undecodable"
	// CodeFieldMalformed marks a frontmatter scalar present but unparseable for
	// its type. Detail["raw"] carries the stored text.
	CodeFieldMalformed = "field-malformed"
	// CodeListItemMalformed marks an element of an id list that is not a usable
	// identifier. Detail["raw"] carries the stored element.
	CodeListItemMalformed = "list-item-malformed"
	// CodeChangeTypeInvalid marks a change type that fails the token grammar.
	CodeChangeTypeInvalid = "change-type-invalid"
	// CodeADRStatusUnparseable marks an ADR status outside the four spellings.
	CodeADRStatusUnparseable = "adr-status-unparseable"
	// CodeLearningPromotionUnknown marks an unrecognized promotion state.
	CodeLearningPromotionUnknown = "learning-promotion-unknown"
)

// dateLayout is the stored date-only spelling; stampLayout the claim
// timestamp's second-precision UTC spelling.
const (
	dateLayout  = "2006-01-02"
	stampLayout = time.RFC3339
)

// archiveDatePrefixLen is the length of an archive filename's "YYYY-MM-DD"
// prefix, which is followed by a hyphen.
const archiveDatePrefixLen = len(dateLayout)

// scalar captures one frontmatter scalar exactly as written, without imposing
// a Go type on it. Decoding a manifest into typed Go fields would fail the
// WHOLE record on one bad token — a non-integer `stacked_on` would take the
// id, the status, and every other field down with it — so every wire field is
// captured as text here and converted afterwards, where a failure is one
// field's finding rather than the record's.
type scalar struct {
	present   bool // the key appeared in the frontmatter mapping
	isNull    bool // present with no value ("key:", "key: ~", "key: null")
	notScalar bool // present, but written as a collection
	raw       string
}

// UnmarshalYAML implements yaml.Unmarshaler. It never returns an error: an
// unexpected node shape is recorded, not rejected.
func (s *scalar) UnmarshalYAML(node *yaml.Node) error {
	s.present = true
	switch {
	case node.Kind != yaml.ScalarNode:
		s.notScalar = true
	case node.Tag == "!!null":
		s.isNull = true
	default:
		s.raw = node.Value
	}
	return nil
}

// scalarList captures a flow or block sequence of scalars in authored order.
// Order is load-bearing: dependency diagnostics tie-break on it.
type scalarList struct {
	present bool
	isNull  bool
	notSeq  bool
	items   []scalar
}

// UnmarshalYAML implements yaml.Unmarshaler, and likewise never errors.
func (l *scalarList) UnmarshalYAML(node *yaml.Node) error {
	l.present = true
	switch {
	case node.Kind == yaml.ScalarNode && node.Tag == "!!null":
		l.isNull = true
	case node.Kind != yaml.SequenceNode:
		l.notSeq = true
	default:
		for _, child := range node.Content {
			var item scalar
			_ = item.UnmarshalYAML(child)
			l.items = append(l.items, item)
		}
	}
	return nil
}

// changeWire is the change manifest's frontmatter as written. Unknown keys are
// compatibility data and stay in the document; they are not decoded here and
// never fail the decode.
type changeWire struct {
	ID             scalar     `yaml:"id"`
	Slug           scalar     `yaml:"slug"`
	Title          scalar     `yaml:"title"`
	Status         scalar     `yaml:"status"`
	Priority       scalar     `yaml:"priority"`
	Type           scalar     `yaml:"type"`
	Created        scalar     `yaml:"created"`
	Updated        scalar     `yaml:"updated"`
	DependsOn      scalarList `yaml:"depends_on"`
	StackedOn      scalar     `yaml:"stacked_on"`
	Related        scalarList `yaml:"related"`
	DiscoveredFrom scalarList `yaml:"discovered_from"`
	ADRs           scalarList `yaml:"adrs"`
	Spec           scalar     `yaml:"spec"`
	Plan           scalar     `yaml:"plan"`
	Results        scalar     `yaml:"results"`
	Trivial        scalar     `yaml:"trivial"`
	Branch         scalar     `yaml:"branch"`
	ClaimedAt      scalar     `yaml:"claimed_at"`
	PR             scalar     `yaml:"pr"`
	Issue          scalar     `yaml:"issue"`
	BlockedBy      scalar     `yaml:"blocked_by"`
	Reconciled     scalar     `yaml:"reconciled"`
}

// adrWire is an ADR's frontmatter as written.
type adrWire struct {
	ID         scalar     `yaml:"id"`
	Slug       scalar     `yaml:"slug"`
	Title      scalar     `yaml:"title"`
	Status     scalar     `yaml:"status"`
	Date       scalar     `yaml:"date"`
	Supersedes scalarList `yaml:"supersedes"`
	Reverses   scalarList `yaml:"reverses"`
	RelatesTo  scalarList `yaml:"relates_to"`
	Change     scalar     `yaml:"change"`
}

// learningWire is a learnings-ledger finding's frontmatter as written.
type learningWire struct {
	Slug       scalar     `yaml:"slug"`
	Hook       scalar     `yaml:"hook"`
	Topics     scalarList `yaml:"topics"`
	Changes    scalarList `yaml:"changes"`
	Created    scalar     `yaml:"created"`
	Updated    scalar     `yaml:"updated"`
	Promotion  scalar     `yaml:"promotion_state"`
	PromotedTo scalar     `yaml:"promoted_to"`
}

// decoder accumulates one record's findings while its fields are converted.
type decoder struct {
	doc      document.Document
	entity   domain.EntityRef
	findings []domain.Finding
}

// report appends a finding about field on the record being decoded.
func (d *decoder) report(code, field string, severity domain.Severity, detail map[string]string) {
	d.findings = append(d.findings, domain.Finding{
		Code:     code,
		Severity: severity,
		Entity:   d.entity,
		Field:    field,
		Detail:   detail,
	})
}

// malformed reports a scalar that is present but unparseable for its type,
// preserving the stored text so the value stays diagnosable.
func (d *decoder) malformed(field, raw string) {
	d.report(CodeFieldMalformed, field, domain.SeverityError, map[string]string{"raw": raw})
}

// state classifies how a captured scalar appeared. The located frontmatter
// entry is consulted for the "key present, no value" shape, so a key the YAML
// tree resolves to null and a key the byte locator sees as valueless agree.
//
// FieldMalformed here means only "written as something other than a scalar";
// type-level parse failures are decided by each typed converter below.
func (d *decoder) state(name string, s scalar) domain.FieldState {
	located, ok := d.doc.Field(name)
	switch {
	case !s.present && !ok:
		return domain.FieldAbsent
	case s.notScalar:
		return domain.FieldMalformed
	case s.isNull, ok && located.Shape == document.ShapeEmpty, s.present && s.raw == "":
		return domain.FieldEmpty
	default:
		return domain.FieldPresent
	}
}

// text returns the stored text of a scalar field, empty when it is absent,
// valueless, or not a scalar at all.
func (d *decoder) text(name string, s scalar) string {
	if d.state(name, s) == domain.FieldPresent {
		return s.raw
	}
	return ""
}

// optionalString converts a scalar into an optional text field.
func (d *decoder) optionalString(name string, s scalar) domain.OptionalString {
	st := d.state(name, s)
	if st == domain.FieldMalformed {
		d.malformed(name, s.raw)
	}
	if st == domain.FieldAbsent {
		return domain.OptionalString{}
	}
	return domain.OptionalString{State: st, Value: s.raw}
}

// optionalInt converts a scalar into an optional integer field, keeping the
// stored text for a value that does not parse.
func (d *decoder) optionalInt(name string, s scalar) domain.OptionalInt {
	st := d.state(name, s)
	switch st {
	case domain.FieldAbsent:
		return domain.OptionalInt{}
	case domain.FieldEmpty:
		return domain.OptionalInt{State: domain.FieldEmpty}
	case domain.FieldMalformed:
		d.malformed(name, s.raw)
		return domain.OptionalInt{State: domain.FieldMalformed, Raw: s.raw}
	}
	n, err := strconv.Atoi(s.raw)
	if err != nil {
		d.malformed(name, s.raw)
		return domain.OptionalInt{State: domain.FieldMalformed, Raw: s.raw}
	}
	return domain.OptionalInt{State: domain.FieldPresent, Value: n, Raw: s.raw}
}

// optionalTime converts a scalar into an optional date or timestamp. A value
// that does not parse stays malformed with its raw text: a missing or
// malformed stamp is never laundered into a usable zero time, because a zero
// time reads as "long ago" to every expiry computation downstream.
func (d *decoder) optionalTime(name string, s scalar, layout string) domain.OptionalTime {
	st := d.state(name, s)
	switch st {
	case domain.FieldAbsent:
		return domain.OptionalTime{}
	case domain.FieldEmpty:
		return domain.OptionalTime{State: domain.FieldEmpty}
	case domain.FieldMalformed:
		d.malformed(name, s.raw)
		return domain.OptionalTime{State: domain.FieldMalformed, Raw: s.raw}
	}
	parsed, err := time.Parse(layout, s.raw)
	if err != nil {
		d.malformed(name, s.raw)
		return domain.OptionalTime{State: domain.FieldMalformed, Raw: s.raw}
	}
	return domain.OptionalTime{State: domain.FieldPresent, Value: parsed, Raw: s.raw}
}

// boolean converts a scalar into a bool. Absent and valueless both mean false;
// anything else that is not a YAML boolean is a finding, never a silent false.
func (d *decoder) boolean(name string, s scalar) bool {
	switch d.state(name, s) {
	case domain.FieldAbsent, domain.FieldEmpty:
		return false
	case domain.FieldMalformed:
		d.malformed(name, s.raw)
		return false
	}
	switch s.raw {
	case "true":
		return true
	case "false":
		return false
	}
	d.malformed(name, s.raw)
	return false
}

// integer converts a required numeric identity field, reporting an unusable
// value rather than substituting one.
func (d *decoder) integer(name string, s scalar) int {
	switch d.state(name, s) {
	case domain.FieldAbsent, domain.FieldEmpty:
		return 0
	case domain.FieldMalformed:
		d.malformed(name, s.raw)
		return 0
	}
	n, err := strconv.Atoi(s.raw)
	if err != nil {
		d.malformed(name, s.raw)
		return 0
	}
	return n
}

// intList converts a sequence of identifiers, preserving authored order.
// Elements that are not usable identifiers are reported and skipped; the
// usable ones keep their relative order.
func (d *decoder) intList(name string, l scalarList) []int {
	if l.notSeq {
		d.malformed(name, "")
		return nil
	}
	out := make([]int, 0, len(l.items))
	for i, item := range l.items {
		n, err := strconv.Atoi(item.raw)
		if err != nil || item.notScalar || item.isNull {
			d.report(CodeListItemMalformed, name, domain.SeverityError,
				map[string]string{"raw": item.raw, "index": strconv.Itoa(i)})
			continue
		}
		out = append(out, n)
	}
	return out
}

// stringList converts a sequence of text values, preserving authored order.
func (d *decoder) stringList(name string, l scalarList) []string {
	if l.notSeq {
		d.malformed(name, "")
		return nil
	}
	out := make([]string, 0, len(l.items))
	for i, item := range l.items {
		if item.notScalar {
			d.report(CodeListItemMalformed, name, domain.SeverityError,
				map[string]string{"raw": item.raw, "index": strconv.Itoa(i)})
			continue
		}
		out = append(out, item.raw)
	}
	return out
}

// changeIDs and adrIDs give the converted identifier lists their domain types.
func changeIDs(ids []int) []domain.ChangeID {
	out := make([]domain.ChangeID, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.ChangeID(id))
	}
	return out
}

func adrIDs(ids []int) []domain.ADRID {
	out := make([]domain.ADRID, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.ADRID(id))
	}
	return out
}

// contentID is a record's opaque content identity: the SHA-256 of its exact
// source bytes, hex encoded.
func contentID(doc document.Document) string {
	sum := sha256.Sum256(doc.Source())
	return hex.EncodeToString(sum[:])
}

// fenceLine is the frontmatter fence, matched as a whole line.
const fenceLine = "---"

// body returns the record's text after the closing frontmatter fence, or the
// whole source when the record has no frontmatter. Every body scan below runs
// over this and never over the frontmatter, so a key- or heading-shaped line
// discussed in prose is read as prose — and a heading written INSIDE
// frontmatter (a block scalar, say) is never mistaken for a body section.
func body(doc document.Document) string {
	source := string(doc.Source())
	if !doc.HasFrontmatter() {
		return source
	}
	rest := source
	offset := 0
	first := true
	for {
		line, tail, found := strings.Cut(rest, "\n")
		if !found {
			return "" // unterminated frontmatter: no body at all
		}
		offset += len(line) + 1
		if strings.TrimSuffix(line, "\r") == fenceLine && !first {
			return source[offset:]
		}
		first = false
		rest = tail
	}
}

// presenceMarkers are the Docket-owned body sections policy consults. Each is
// matched as a whole bare heading line: a dated or annotated variant
// ("## Run halted — 2026-08-14") is a different section and does not count.
const (
	markerRunHalted        = "## Run halted"
	markerAutoGroomBlocked = "## Auto-groom blocked"
	markerFinalizeBlocked  = "## Finalize blocked"
	markerPublishDeferred  = "## Publish deferred"
)

// hasHeading reports whether text carries heading as a whole line, matched
// exactly — no leading whitespace, no trailing text, CRLF tolerated.
func hasHeading(text, heading string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSuffix(line, "\r") == heading {
			return true
		}
	}
	return false
}

// archiveDate parses the "YYYY-MM-DD-" prefix an archived record's filename
// carries. A record read from anywhere else has no archive date at all; an
// archived record whose filename lacks a usable prefix is malformed, with the
// filename kept as the raw value.
func archiveDate(loc RecordLocation, recordPath string) domain.OptionalTime {
	if loc != LocationArchive {
		return domain.OptionalTime{}
	}
	base := path.Base(recordPath)
	malformed := domain.OptionalTime{State: domain.FieldMalformed, Raw: base}
	if len(base) <= archiveDatePrefixLen || base[archiveDatePrefixLen] != '-' {
		return malformed
	}
	prefix := base[:archiveDatePrefixLen]
	parsed, err := time.Parse(dateLayout, prefix)
	if err != nil {
		return malformed
	}
	return domain.OptionalTime{State: domain.FieldPresent, Value: parsed, Raw: prefix}
}

// newDecoder starts a decode over in's document, attributing findings to
// entity. A document whose frontmatter cannot be decoded at all yields
// ok=false with a record-undecodable finding: the record is still returned by
// the caller, identified by path, so nothing is silently dropped.
func newDecoder(in InputDocument, kind domain.EntityKind, wire any) (*decoder, bool) {
	d := &decoder{doc: in.Document, entity: domain.EntityRef{Kind: kind, Path: in.Path}}
	if err := in.Document.DecodeFrontmatter(wire); err != nil {
		d.report(CodeRecordUndecodable, "", domain.SeverityError,
			map[string]string{"error": err.Error()})
		return d, false
	}
	return d, true
}

// decodeChange converts one supplied change manifest into a domain Change.
// Policy decisions are not made here: unknown statuses, priorities and
// identities are retained as stored for the validation pass to judge.
func decodeChange(in InputDocument) (domain.Change, []domain.Finding) {
	var wire changeWire
	d, ok := newDecoder(in, domain.EntityChange, &wire)

	spec := domain.ChangeSpec{Location: in.Location, Path: in.Path}
	spec.ArchiveDate = archiveDate(in.Location, in.Path)
	if !ok {
		return domain.NewChange(spec), d.findings
	}

	spec.ID = domain.ChangeID(d.integer("id", wire.ID))
	spec.Slug = d.text("slug", wire.Slug)
	spec.Title = d.text("title", wire.Title)
	// The finding entity is only fully known once identity is decoded; every
	// finding raised from here on carries it.
	d.entity.ID, d.entity.Slug = int(spec.ID), spec.Slug

	spec.RawStatus = d.text("status", wire.Status)
	spec.Status, _ = domain.ParseStatus(spec.RawStatus)
	spec.RawPriority = d.text("priority", wire.Priority)
	spec.Priority, _ = domain.ParsePriority(spec.RawPriority)

	// The type token is checked for SHAPE only. Membership in the repository's
	// configured change_types is deliberately not an error: a record authored
	// under a different configuration must stay readable.
	spec.Type = d.text("type", wire.Type)
	if spec.Type != "" && !domain.ValidTypeToken(spec.Type) {
		d.report(CodeChangeTypeInvalid, "type", domain.SeverityError,
			map[string]string{"raw": spec.Type})
	}

	spec.Created = d.optionalTime("created", wire.Created, dateLayout)
	spec.Updated = d.optionalTime("updated", wire.Updated, dateLayout)
	spec.DependsOn = changeIDs(d.intList("depends_on", wire.DependsOn))
	spec.StackedOn = d.optionalInt("stacked_on", wire.StackedOn)
	spec.Related = changeIDs(d.intList("related", wire.Related))
	spec.DiscoveredFrom = changeIDs(d.intList("discovered_from", wire.DiscoveredFrom))
	spec.ADRs = adrIDs(d.intList("adrs", wire.ADRs))
	spec.Spec = d.optionalString("spec", wire.Spec)
	spec.Plan = d.optionalString("plan", wire.Plan)
	spec.Results = d.optionalString("results", wire.Results)
	spec.Trivial = d.boolean("trivial", wire.Trivial)
	spec.Branch = d.optionalString("branch", wire.Branch)
	spec.ClaimedAt = d.optionalTime("claimed_at", wire.ClaimedAt, stampLayout)
	spec.PR = d.optionalString("pr", wire.PR)
	spec.Issue = d.optionalString("issue", wire.Issue)
	spec.BlockedBy = d.optionalString("blocked_by", wire.BlockedBy)
	spec.Reconciled = d.boolean("reconciled", wire.Reconciled)

	text := body(in.Document)
	spec.HasRunHalted = hasHeading(text, markerRunHalted)
	spec.HasAutoGroomBlocked = hasHeading(text, markerAutoGroomBlocked)
	spec.HasFinalizeBlocked = hasHeading(text, markerFinalizeBlocked)
	spec.HasPublishDeferred = hasHeading(text, markerPublishDeferred)

	return domain.NewChange(spec), d.findings
}

// decodeADR converts one supplied ADR. An unparseable status is a finding and
// the record is retained: an invalid record stays visible to validation rather
// than disappearing from the snapshot.
func decodeADR(in InputDocument) (domain.ADR, []domain.Finding) {
	var wire adrWire
	d, ok := newDecoder(in, domain.EntityADR, &wire)

	spec := domain.ADRSpec{Path: in.Path, ContentID: contentID(in.Document)}
	if !ok {
		return domain.NewADR(spec), d.findings
	}

	spec.ID = domain.ADRID(d.integer("id", wire.ID))
	spec.Slug = d.text("slug", wire.Slug)
	spec.Title = d.text("title", wire.Title)
	d.entity.ID, d.entity.Slug = int(spec.ID), spec.Slug

	spec.RawStatus = d.text("status", wire.Status)
	if parsed, parsedOK := domain.ParseADRStatus(spec.RawStatus); parsedOK {
		spec.Status = parsed
	} else {
		d.report(CodeADRStatusUnparseable, "status", domain.SeverityError,
			map[string]string{"raw": spec.RawStatus})
	}

	spec.Date = d.optionalTime("date", wire.Date, dateLayout)
	spec.Supersedes = adrIDs(d.intList("supersedes", wire.Supersedes))
	spec.Reverses = adrIDs(d.intList("reverses", wire.Reverses))
	spec.RelatesTo = adrIDs(d.intList("relates_to", wire.RelatesTo))
	spec.Change = d.optionalInt("change", wire.Change)

	return domain.NewADR(spec), d.findings
}

// decodeLearning converts one learnings-ledger finding. An absent or valueless
// promotion state is the legacy-missing spelling and resolves to retained.
func decodeLearning(in InputDocument) (domain.Learning, []domain.Finding) {
	var wire learningWire
	d, ok := newDecoder(in, domain.EntityLearning, &wire)

	spec := domain.LearningSpec{Path: in.Path}
	if !ok {
		return domain.NewLearning(spec), d.findings
	}

	spec.Slug = d.text("slug", wire.Slug)
	d.entity.Slug = spec.Slug
	spec.Hook = d.text("hook", wire.Hook)
	spec.Topics = d.stringList("topics", wire.Topics)
	spec.Changes = changeIDs(d.intList("changes", wire.Changes))
	spec.Created = d.optionalTime("created", wire.Created, dateLayout)
	spec.Updated = d.optionalTime("updated", wire.Updated, dateLayout)

	raw := d.text("promotion_state", wire.Promotion)
	if parsed, parsedOK := domain.ParsePromotionState(raw); parsedOK {
		spec.Promotion = parsed
	} else {
		d.report(CodeLearningPromotionUnknown, "promotion_state", domain.SeverityError,
			map[string]string{"raw": raw})
	}

	spec.PromotedTo = d.optionalString("promoted_to", wire.PromotedTo)
	spec.Content = body(in.Document)

	return domain.NewLearning(spec), d.findings
}

// artifactKindByPath classifies an authored artifact by the directory it sits
// in. It reads path structure only — never the document's prose.
func artifactKindByPath(recordPath string) domain.ArtifactKind {
	for _, segment := range strings.Split(path.Dir(recordPath), "/") {
		switch segment {
		case "specs":
			return domain.ArtifactSpecKind
		case "plans":
			return domain.ArtifactPlan
		case "results":
			return domain.ArtifactResults
		}
	}
	return domain.ArtifactOther
}

// backlinkBlockName is the managed block a change's artifact carries pointing
// back at the change.
const backlinkBlockName = "backlink"

// decodeArtifact converts one authored artifact: path, kind, content identity,
// and managed-marker presence, with no interpretation of its prose.
func decodeArtifact(in InputDocument) (domain.Artifact, []domain.Finding) {
	_, hasBacklink := in.Document.Block(backlinkBlockName)
	return domain.NewArtifact(domain.ArtifactSpec{
		Path:              in.Path,
		Kind:              artifactKindByPath(in.Path),
		ContentID:         contentID(in.Document),
		HasBacklinkMarker: hasBacklink,
	}), nil
}

// derivedKindByPath classifies a generated view by its path. A derived view is
// accounted for, never consulted as authority, so nothing else is read.
func derivedKindByPath(recordPath string) domain.DerivedViewKind {
	base := path.Base(recordPath)
	dir := path.Dir(recordPath)
	switch {
	case base == "BOARD.md":
		return domain.DerivedBoard
	case base == "LEARNINGS.md":
		return domain.DerivedLearningsIndex
	case base == "README.md" && path.Base(dir) == "adrs":
		return domain.DerivedADRIndex
	case base == "README.md" && path.Base(dir) == "learnings":
		return domain.DerivedLearningsIndex
	}
	return domain.DerivedOther
}

// decodeDerived converts one generated view into its accounting record.
func decodeDerived(in InputDocument) (domain.DerivedView, []domain.Finding) {
	return domain.NewDerivedView(domain.DerivedViewSpec{
		Path: in.Path,
		Kind: derivedKindByPath(in.Path),
	}), nil
}
