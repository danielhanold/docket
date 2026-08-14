package repository

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// Validation finding codes. Every code is a stable lowercase-hyphen token, and
// every code this package can emit is named here — a caller keys on the
// constant, never on rendered prose.
const (
	// CodeChangeIDInvalid marks a change whose numeric identity is unusable.
	CodeChangeIDInvalid = "change-id-invalid"
	// CodeChangeSlugInvalid marks a slug that is empty or fails the token
	// grammar.
	CodeChangeSlugInvalid = "change-slug-invalid"
	// CodeChangeStatusUnknown marks a status outside the closed set — an
	// unrecognized spelling or none at all.
	CodeChangeStatusUnknown = "change-status-unknown"
	// CodeChangePriorityUnknown marks a priority outside the closed set. It is
	// a warning: an unreadable priority sorts as medium and stalls nothing.
	CodeChangePriorityUnknown = "change-priority-unknown"
	// CodeChangeFilenameMismatch marks a filename that disagrees with the
	// frontmatter identity it is supposed to name.
	CodeChangeFilenameMismatch = "change-filename-mismatch"
	// CodeChangeIDDuplicate marks a numeric identity more than one change
	// record claims. Both records stay in the snapshot; lookups report
	// ambiguity rather than picking a winner.
	CodeChangeIDDuplicate = "change-id-duplicate"
	// CodeChangeSlugDuplicate marks a slug more than one change record claims.
	CodeChangeSlugDuplicate = "change-slug-duplicate"
	// CodeRecordPathDuplicate marks a repository path supplied more than once.
	CodeRecordPathDuplicate = "record-path-duplicate"
	// CodeChangePlacementInvalid marks a record whose directory contradicts its
	// status: a terminal change left in active/, or a live one in archive/.
	CodeChangePlacementInvalid = "change-placement-invalid"
	// CodeChangeArchiveDateInvalid marks an archived record whose filename
	// carries no usable YYYY-MM-DD prefix.
	CodeChangeArchiveDateInvalid = "change-archive-date-invalid"
	// CodeChangeTerminalClaimStamp marks an archived terminal record that still
	// carries a claim stamp — a lease nothing can ever release.
	CodeChangeTerminalClaimStamp = "change-terminal-claim-stamp"
	// CodeChangeStateIncoherent marks a lifecycle state missing a fact the
	// state itself guarantees. Field names the missing fact.
	CodeChangeStateIncoherent = "change-state-incoherent"
	// CodeChangeReferenceDangling marks a change-to-change or change-to-ADR
	// reference no single supplied record resolves.
	CodeChangeReferenceDangling = "change-reference-dangling"
	// CodeChangeDependencyCycle marks one elementary cycle over depends_on.
	CodeChangeDependencyCycle = "change-dependency-cycle"
	// CodeChangeStackCycle marks one elementary cycle over stacked_on.
	CodeChangeStackCycle = "change-stack-cycle"
	// CodeArtifactReferenceKindMismatch marks a change artifact reference whose
	// path was supplied as something other than an artifact.
	CodeArtifactReferenceKindMismatch = "artifact-reference-kind-mismatch"
	// CodeADRIDInvalid marks an ADR whose numeric identity is unusable.
	CodeADRIDInvalid = "adr-id-invalid"
	// CodeADRSlugInvalid marks an ADR slug that is empty or fails the grammar.
	CodeADRSlugInvalid = "adr-slug-invalid"
	// CodeADRFilenameMismatch marks an ADR filename that disagrees with its
	// frontmatter identity.
	CodeADRFilenameMismatch = "adr-filename-mismatch"
	// CodeADRIDDuplicate marks a numeric identity more than one ADR claims.
	CodeADRIDDuplicate = "adr-id-duplicate"
	// CodeLearningSlugInvalid marks a learning slug that is empty or fails the
	// grammar.
	CodeLearningSlugInvalid = "learning-slug-invalid"
	// CodeLearningFilenameMismatch marks a learning filename that disagrees
	// with its slug.
	CodeLearningFilenameMismatch = "learning-filename-mismatch"
	// CodeLearningSlugDuplicate marks a slug more than one learning claims.
	CodeLearningSlugDuplicate = "learning-slug-duplicate"
	// CodeLearningTopicInvalid marks a topic that fails the token grammar.
	CodeLearningTopicInvalid = "learning-topic-invalid"
	// CodeLearningPromotionDestination marks a promotion state and destination
	// that contradict each other.
	CodeLearningPromotionDestination = "learning-promotion-destination-invalid"
	// CodeLearningReferenceDangling marks a learning's change reference no
	// single supplied record resolves. It is a warning for the same reason the
	// associative change references are.
	CodeLearningReferenceDangling = "learning-reference-dangling"
	// CodeRecordUnaccounted marks a supplied path that reached neither a
	// decoded entity nor an invalid-record finding. It is a defect in this
	// package, reported rather than swallowed.
	CodeRecordUnaccounted = "record-unaccounted"
)

// idDigits is the zero-padded width of a change or ADR id in a filename.
const idDigits = 4

// validate runs the complete single-snapshot pass over an already-built
// snapshot. It reads nothing but the snapshot: which records were supplied,
// and therefore what a reference can resolve against, is exactly what the
// snapshot holds.
func validate(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	findings = append(findings, validateChanges(snap)...)
	findings = append(findings, validateADRs(snap)...)
	findings = append(findings, validateLearnings(snap)...)
	findings = append(findings, validateUniqueness(snap)...)
	findings = append(findings, validateCycles(snap)...)
	findings = append(findings, domain.ValidateADRGraph(snap)...)
	return findings
}

// changeRef builds the finding subject for a change record.
func changeRef(c domain.Change) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityChange, ID: int(c.ID()), Slug: c.Slug(), Path: c.Path()}
}

// adrRef builds the finding subject for an ADR record.
func adrEntityRef(a domain.ADR) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityADR, ID: int(a.ID()), Slug: a.Slug(), Path: a.Path()}
}

// learningRef builds the finding subject for a learning record.
func learningRef(l domain.Learning) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityLearning, Slug: l.Slug(), Path: l.Path()}
}

// finding assembles one finding.
func finding(code string, sev domain.Severity, entity domain.EntityRef, field string, detail map[string]string) domain.Finding {
	return domain.Finding{Code: code, Severity: sev, Entity: entity, Field: field, Detail: detail}
}

// validToken reports whether s matches the shared identifier grammar —
// lowercase alphanumerics in hyphen-separated runs, no leading, trailing, or
// doubled hyphen. Slugs and topics share it.
func validToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= '0' && ch <= '9':
		case ch == '-':
			if i == 0 || i == len(s)-1 || s[i-1] == '-' {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// present reports whether an optional text field carries usable text. A key
// that is absent, valueless, or malformed carries none.
func present(o domain.OptionalString) bool {
	return o.State == domain.FieldPresent && o.Value != ""
}

// filenameIdentity splits a record base name into its numeric id and slug
// parts. archived strips the leading YYYY-MM-DD- date prefix first. The slug
// part is returned as written, for the caller to compare against the
// frontmatter slug.
func filenameIdentity(base string, archived bool) (id int, slug string, ok bool) {
	name, found := strings.CutSuffix(base, ".md")
	if !found {
		return 0, "", false
	}
	if archived {
		if len(name) <= archiveDatePrefixLen || name[archiveDatePrefixLen] != '-' {
			return 0, "", false
		}
		name = name[archiveDatePrefixLen+1:]
	}
	digits, rest, found := strings.Cut(name, "-")
	if !found || len(digits) != idDigits {
		return 0, "", false
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, "", false
	}
	return n, rest, true
}

// identityMatchesFilename reports whether a base name names the given id and
// slug. Both halves must agree exactly: docket's writers truncate the slug
// itself before writing it, so the same value lands in the frontmatter and in
// the filename and any divergence is a defect. Prefix matching would also make
// the check vacuous for a base name carrying no slug at all.
func identityMatchesFilename(base string, archived bool, id int, slug string) bool {
	fileID, fileSlug, ok := filenameIdentity(base, archived)
	return ok && fileID == id && fileSlug == slug
}

// validateChanges runs every per-record change rule: identity, placement,
// state coherence, and references.
func validateChanges(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	for _, c := range snap.Changes() {
		findings = append(findings, changeIdentity(c)...)
		findings = append(findings, changePlacement(c)...)
		findings = append(findings, changeCoherence(c)...)
		findings = append(findings, changeReferences(snap, c)...)
		findings = append(findings, changeArtifactReferences(snap, c)...)
	}
	return findings
}

// changeIdentity checks id, slug, status, priority, and filename identity.
func changeIdentity(c domain.Change) []domain.Finding {
	ref := changeRef(c)
	var findings []domain.Finding
	if c.ID() <= 0 {
		findings = append(findings, finding(CodeChangeIDInvalid, domain.SeverityError, ref, "id",
			map[string]string{"raw": strconv.Itoa(int(c.ID()))}))
	}
	if !validToken(c.Slug()) {
		findings = append(findings, finding(CodeChangeSlugInvalid, domain.SeverityError, ref, "slug",
			map[string]string{"raw": c.Slug()}))
	}
	if c.Status() == "" {
		findings = append(findings, finding(CodeChangeStatusUnknown, domain.SeverityError, ref, "status",
			map[string]string{"raw": c.RawStatus()}))
	}
	if c.Priority() == "" {
		findings = append(findings, finding(CodeChangePriorityUnknown, domain.SeverityWarning, ref, "priority",
			map[string]string{"raw": c.RawPriority()}))
	}
	if !identityMatchesFilename(path.Base(c.Path()), c.Location() == LocationArchive, int(c.ID()), c.Slug()) {
		findings = append(findings, finding(CodeChangeFilenameMismatch, domain.SeverityError, ref, "path",
			map[string]string{"base": path.Base(c.Path()), "slug": c.Slug(),
				"id": fmt.Sprintf("%0*d", idDigits, int(c.ID()))}))
	}
	return findings
}

// changePlacement checks the record's directory against its status, the
// archive filename date, and the claim stamp a terminal record must not carry.
func changePlacement(c domain.Change) []domain.Finding {
	ref := changeRef(c)
	var findings []domain.Finding
	terminal := c.Status().Terminal()
	switch c.Location() {
	case LocationActive:
		if terminal {
			findings = append(findings, finding(CodeChangePlacementInvalid, domain.SeverityError, ref, "status",
				map[string]string{"placement": string(LocationActive), "status": string(c.Status())}))
		}
	case LocationArchive:
		if c.Status() != "" && !terminal {
			findings = append(findings, finding(CodeChangePlacementInvalid, domain.SeverityError, ref, "status",
				map[string]string{"placement": string(LocationArchive), "status": string(c.Status())}))
		}
		if c.ArchiveDate().State != domain.FieldPresent {
			findings = append(findings, finding(CodeChangeArchiveDateInvalid, domain.SeverityError, ref, "path",
				map[string]string{"base": path.Base(c.Path())}))
		}
		if terminal && c.ClaimedAt().State == domain.FieldPresent {
			findings = append(findings, finding(CodeChangeTerminalClaimStamp, domain.SeverityError, ref, "claimed_at",
				map[string]string{"raw": c.ClaimedAt().Raw, "status": string(c.Status())}))
		}
	}
	return findings
}

// changeCoherence checks the facts each lifecycle state guarantees. Only the
// state's own guarantees are required: a field that is legitimately absent
// from an older valid record is never demanded by a state that does not need
// it.
func changeCoherence(c domain.Change) []domain.Finding {
	ref := changeRef(c)
	var missing []string
	switch c.Status() {
	case domain.StatusInProgress:
		missing = append(missing, claimFacts(c)...)
	case domain.StatusBlocked:
		if !present(c.BlockedBy()) {
			missing = append(missing, "blocked_by")
		}
	case domain.StatusImplemented:
		missing = append(missing, implementedFacts(c)...)
	case domain.StatusStackedMerged:
		missing = append(missing, implementedFacts(c)...)
		if c.StackedOn().State != domain.FieldPresent {
			missing = append(missing, "stacked_on")
		}
	}
	findings := make([]domain.Finding, 0, len(missing))
	for _, field := range missing {
		findings = append(findings, finding(CodeChangeStateIncoherent, domain.SeverityError, ref, field,
			map[string]string{"status": string(c.Status())}))
	}
	return findings
}

// claimFacts names the claim facts a held lease requires.
func claimFacts(c domain.Change) []string {
	var missing []string
	if !present(c.Branch()) {
		missing = append(missing, "branch")
	}
	if c.ClaimedAt().State != domain.FieldPresent {
		missing = append(missing, "claimed_at")
	}
	return missing
}

// implementedFacts names the facts an implemented record carries: the claim
// facts plus the delivery evidence.
func implementedFacts(c domain.Change) []string {
	missing := claimFacts(c)
	if !present(c.Plan()) {
		missing = append(missing, "plan")
	}
	if !present(c.PR()) {
		missing = append(missing, "pr")
	}
	if !c.Reconciled() {
		missing = append(missing, "reconciled")
	}
	return missing
}

// referenceSeverity ranks a reference field. depends_on and stacked_on gate
// readiness, selection, and the base a branch is built on, so a reference
// neither can resolve is an error. related, discovered_from, and adrs are
// associative cross-links that gate nothing, and a composer supplying part of
// a repository legitimately leaves them unresolved — those are warnings.
func referenceSeverity(field string) domain.Severity {
	switch field {
	case "depends_on", "stacked_on":
		return domain.SeverityError
	}
	return domain.SeverityWarning
}

// changeReferences resolves every identifier a change names.
func changeReferences(snap domain.Snapshot, c domain.Change) []domain.Finding {
	ref := changeRef(c)
	var findings []domain.Finding
	lists := []struct {
		field string
		ids   []domain.ChangeID
	}{
		{"depends_on", c.DependsOn()},
		{"related", c.Related()},
		{"discovered_from", c.DiscoveredFrom()},
	}
	if c.StackedOn().State == domain.FieldPresent {
		lists = append(lists, struct {
			field string
			ids   []domain.ChangeID
		}{"stacked_on", []domain.ChangeID{domain.ChangeID(c.StackedOn().Value)}})
	}
	for _, list := range lists {
		for _, id := range list.ids {
			if _, out := snap.Change(id); out != domain.LookupFound {
				findings = append(findings, danglingReference(CodeChangeReferenceDangling, ref, list.field,
					domain.EntityRef{Kind: domain.EntityChange, ID: int(id)}, out))
			}
		}
	}
	for _, id := range c.ADRs() {
		if _, out := snap.ADR(id); out != domain.LookupFound {
			findings = append(findings, danglingReference(CodeChangeReferenceDangling, ref, "adrs",
				domain.EntityRef{Kind: domain.EntityADR, ID: int(id)}, out))
		}
	}
	return findings
}

// danglingReference builds one unresolved-reference finding, recording whether
// the target was absent or claimed by more than one record.
func danglingReference(code string, subject domain.EntityRef, field string, target domain.EntityRef, out domain.LookupOutcome) domain.Finding {
	lookup := "absent"
	if out == domain.LookupAmbiguous {
		lookup = "ambiguous"
	}
	f := finding(code, referenceSeverity(field), subject, field,
		map[string]string{"lookup": lookup, "target": strconv.Itoa(target.ID)})
	f.Related = []domain.EntityRef{target}
	return f
}

// changeArtifactReferences checks a change's spec, plan, and results paths
// against what was actually supplied. A path nobody supplied is not a finding:
// the composer decides which artifacts to read, and a reference into a
// document it chose not to supply says nothing about the repository. A path
// supplied as something OTHER than an artifact is a real contradiction.
func changeArtifactReferences(snap domain.Snapshot, c domain.Change) []domain.Finding {
	ref := changeRef(c)
	var findings []domain.Finding
	for _, r := range []struct {
		field string
		value domain.OptionalString
	}{{"spec", c.Spec()}, {"plan", c.Plan()}, {"results", c.Results()}} {
		if !present(r.value) {
			continue
		}
		kind, supplied := suppliedKind(snap, r.value.Value)
		if !supplied || kind == domain.EntityArtifact {
			continue
		}
		findings = append(findings, finding(CodeArtifactReferenceKindMismatch, domain.SeverityError, ref, r.field,
			map[string]string{"path": r.value.Value, "supplied-as": string(kind)}))
	}
	return findings
}

// suppliedKind reports what a repository path was supplied as, if anything.
func suppliedKind(snap domain.Snapshot, recordPath string) (domain.EntityKind, bool) {
	for _, a := range snap.Artifacts() {
		if a.Path() == recordPath {
			return domain.EntityArtifact, true
		}
	}
	for _, d := range snap.DerivedViews() {
		if d.Path() == recordPath {
			return domain.EntityDerived, true
		}
	}
	for _, c := range snap.Changes() {
		if c.Path() == recordPath {
			return domain.EntityChange, true
		}
	}
	for _, a := range snap.ADRs() {
		if a.Path() == recordPath {
			return domain.EntityADR, true
		}
	}
	for _, l := range snap.Learnings() {
		if l.Path() == recordPath {
			return domain.EntityLearning, true
		}
	}
	return "", false
}

// validateADRs checks ADR identity. The relationship graph itself is domain's
// ValidateADRGraph, merged in by validate.
func validateADRs(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	for _, a := range snap.ADRs() {
		ref := adrEntityRef(a)
		if a.ID() <= 0 {
			findings = append(findings, finding(CodeADRIDInvalid, domain.SeverityError, ref, "id",
				map[string]string{"raw": strconv.Itoa(int(a.ID()))}))
		}
		if !validToken(a.Slug()) {
			findings = append(findings, finding(CodeADRSlugInvalid, domain.SeverityError, ref, "slug",
				map[string]string{"raw": a.Slug()}))
		}
		if !identityMatchesFilename(path.Base(a.Path()), false, int(a.ID()), a.Slug()) {
			findings = append(findings, finding(CodeADRFilenameMismatch, domain.SeverityError, ref, "path",
				map[string]string{"base": path.Base(a.Path()), "slug": a.Slug(),
					"id": fmt.Sprintf("%0*d", idDigits, int(a.ID()))}))
		}
	}
	return findings
}

// validateLearnings checks learning identity, topics, references, and the
// agreement between a promotion state and its destination.
func validateLearnings(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	for _, l := range snap.Learnings() {
		ref := learningRef(l)
		if !validToken(l.Slug()) {
			findings = append(findings, finding(CodeLearningSlugInvalid, domain.SeverityError, ref, "slug",
				map[string]string{"raw": l.Slug()}))
		}
		if base := path.Base(l.Path()); base != l.Slug()+".md" {
			findings = append(findings, finding(CodeLearningFilenameMismatch, domain.SeverityError, ref, "path",
				map[string]string{"base": base, "slug": l.Slug()}))
		}
		for i, topic := range l.Topics() {
			if !validToken(topic) {
				findings = append(findings, finding(CodeLearningTopicInvalid, domain.SeverityError, ref, "topics",
					map[string]string{"raw": topic, "index": strconv.Itoa(i)}))
			}
		}
		findings = append(findings, learningPromotion(l, ref)...)
		for _, id := range l.Changes() {
			if _, out := snap.Change(id); out != domain.LookupFound {
				findings = append(findings, danglingReference(CodeLearningReferenceDangling, ref, "changes",
					domain.EntityRef{Kind: domain.EntityChange, ID: int(id)}, out))
			}
		}
	}
	return findings
}

// learningPromotion checks that a promotion state and its destination agree: a
// promoted finding names where it graduated to, and one that never graduated
// names nowhere.
func learningPromotion(l domain.Learning, ref domain.EntityRef) []domain.Finding {
	destination := present(l.PromotedTo())
	switch {
	case l.Promotion() == domain.PromotionPromoted && !destination:
		return []domain.Finding{finding(CodeLearningPromotionDestination, domain.SeverityError, ref, "promoted_to",
			map[string]string{"promotion_state": string(l.Promotion())})}
	case l.Promotion() != domain.PromotionPromoted && destination:
		return []domain.Finding{finding(CodeLearningPromotionDestination, domain.SeverityWarning, ref, "promoted_to",
			map[string]string{"promotion_state": string(l.Promotion()), "raw": l.PromotedTo().Value})}
	}
	return nil
}

// validateUniqueness reports identities and paths more than one record claims.
// Both records stay in the snapshot: the finding names the collision, and the
// snapshot's lookups report it as ambiguous rather than choosing between them.
func validateUniqueness(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	changeIDs := make(map[domain.ChangeID]int)
	changeSlugs := make(map[string]int)
	paths := make(map[string]int)
	for _, c := range snap.Changes() {
		changeIDs[c.ID()]++
		if c.Slug() != "" {
			changeSlugs[c.Slug()]++
		}
	}
	for _, c := range snap.Changes() {
		ref := changeRef(c)
		if changeIDs[c.ID()] > 1 {
			findings = append(findings, finding(CodeChangeIDDuplicate, domain.SeverityError, ref, "id",
				map[string]string{"count": strconv.Itoa(changeIDs[c.ID()])}))
		}
		if c.Slug() != "" && changeSlugs[c.Slug()] > 1 {
			findings = append(findings, finding(CodeChangeSlugDuplicate, domain.SeverityError, ref, "slug",
				map[string]string{"count": strconv.Itoa(changeSlugs[c.Slug()])}))
		}
	}
	adrIDCounts := make(map[domain.ADRID]int)
	for _, a := range snap.ADRs() {
		adrIDCounts[a.ID()]++
	}
	for _, a := range snap.ADRs() {
		if adrIDCounts[a.ID()] > 1 {
			findings = append(findings, finding(CodeADRIDDuplicate, domain.SeverityError, adrEntityRef(a), "id",
				map[string]string{"count": strconv.Itoa(adrIDCounts[a.ID()])}))
		}
	}
	learningSlugs := make(map[string]int)
	for _, l := range snap.Learnings() {
		learningSlugs[l.Slug()]++
	}
	for _, l := range snap.Learnings() {
		if learningSlugs[l.Slug()] > 1 {
			findings = append(findings, finding(CodeLearningSlugDuplicate, domain.SeverityError, learningRef(l), "slug",
				map[string]string{"count": strconv.Itoa(learningSlugs[l.Slug()])}))
		}
	}
	for _, entry := range snapshotPaths(snap) {
		paths[entry.path]++
	}
	for _, entry := range snapshotPaths(snap) {
		if paths[entry.path] > 1 {
			findings = append(findings, finding(CodeRecordPathDuplicate, domain.SeverityError,
				domain.EntityRef{Kind: entry.kind, Path: entry.path}, "path",
				map[string]string{"count": strconv.Itoa(paths[entry.path])}))
		}
	}
	return findings
}

// pathEntry is one accounted record path and the kind that holds it.
type pathEntry struct {
	kind domain.EntityKind
	path string
}

// snapshotPaths lists every path the snapshot accounts for, in record order.
func snapshotPaths(snap domain.Snapshot) []pathEntry {
	var out []pathEntry
	for _, c := range snap.Changes() {
		out = append(out, pathEntry{domain.EntityChange, c.Path()})
	}
	for _, a := range snap.ADRs() {
		out = append(out, pathEntry{domain.EntityADR, a.Path()})
	}
	for _, l := range snap.Learnings() {
		out = append(out, pathEntry{domain.EntityLearning, l.Path()})
	}
	for _, a := range snap.Artifacts() {
		out = append(out, pathEntry{domain.EntityArtifact, a.Path()})
	}
	for _, d := range snap.DerivedViews() {
		out = append(out, pathEntry{domain.EntityDerived, d.Path()})
	}
	return out
}

// validateCycles reports every dependency and stack cycle, naming every member
// of each so the repair is visible from one finding.
func validateCycles(snap domain.Snapshot) []domain.Finding {
	var findings []domain.Finding
	for _, cycle := range domain.DependencyCycles(snap) {
		findings = append(findings, cycleFinding(CodeChangeDependencyCycle, "depends_on", cycle))
	}
	for _, cycle := range domain.StackCycles(snap) {
		findings = append(findings, cycleFinding(CodeChangeStackCycle, "stacked_on", cycle))
	}
	return findings
}

// cycleFinding builds one cycle finding, attributed to the cycle's first
// member and relating every member.
func cycleFinding(code, field string, cycle domain.Cycle) domain.Finding {
	members := make([]domain.EntityRef, 0, len(cycle.Members))
	rendered := make([]string, 0, len(cycle.Members))
	for _, id := range cycle.Members {
		members = append(members, domain.EntityRef{Kind: domain.EntityChange, ID: int(id)})
		rendered = append(rendered, fmt.Sprintf("%0*d", idDigits, int(id)))
	}
	f := finding(code, domain.SeverityError, members[0], field,
		map[string]string{"members": strings.Join(rendered, ",")})
	f.Related = members
	return f
}
