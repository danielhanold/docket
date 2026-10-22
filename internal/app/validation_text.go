package app

// validation_text.go — the one plain-language table for the validator codes
// that reach `docket status` and `docket repository check` (change 0553).
//
// The domain and repository layers stay text-free: a domain.Finding carries a
// stable code plus its entity, field, related records, and detail, and this
// file turns that into a what-is-wrong message, a whether-it-matters impact,
// and a what-to-do remedy. Status (validationFinding) and check
// (corpusFindings) both call explainValidation, so the two can never word a
// code differently. TestValidationTextCoversEveryValidatorCode keeps the table
// in step with the Code constants, which it parses from the domain and
// repository source.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// findingText is the three user-facing parts of one finding.
type findingText struct {
	Message string
	Impact  string
	Remedy  string
}

// validationExplainer renders one validator code's text. loc names where a
// hand edit of the finding's record really happens.
type validationExplainer func(domain.Finding, reposetup.RecordLocator) findingText

// explainValidation renders f through its table entry. A code with no entry —
// an evolution or transaction-scope code arriving through a mutation result —
// keeps the code-plus-detail text findingMessage renders, which is never
// empty.
func explainValidation(f domain.Finding, loc reposetup.RecordLocator) findingText {
	if explain, ok := validationTexts[f.Code]; ok {
		return explain(f, loc)
	}
	return findingText{Message: findingMessage(f)}
}

// validationRecordPath is the metadata-branch path of the record a finding is
// about. ADR-graph findings name the ADR by id only, so their record path
// rides in Detail["path"].
func validationRecordPath(f domain.Finding) string {
	if f.Entity.Path != "" {
		return f.Entity.Path
	}
	return f.Detail["path"]
}

// subjectOf names a finding's record at the start of a sentence. A record
// decoded without a usable id is named by its path when it has one.
func subjectOf(e domain.EntityRef) string {
	switch e.Kind {
	case domain.EntityChange:
		if e.ID > 0 {
			return fmt.Sprintf("Change %04d", e.ID)
		}
		return unnumbered("change", e.Path)
	case domain.EntityADR:
		if e.ID > 0 {
			return fmt.Sprintf("ADR %04d", e.ID)
		}
		return unnumbered("ADR", e.Path)
	case domain.EntityLearning:
		if e.Slug != "" {
			return "The learning `" + e.Slug + "`"
		}
		return unnumbered("learning", e.Path)
	}
	if e.Path != "" {
		return "`" + e.Path + "`"
	}
	return "This record"
}

// unnumbered names a record that has no usable identity: by its path when it
// has one, else as "this <noun> record".
func unnumbered(noun, recordPath string) string {
	if recordPath != "" {
		return "The " + noun + " record `" + recordPath + "`"
	}
	return "This " + noun + " record"
}

// lowerSubject is subjectOf for the middle of a sentence: its first letter is
// lowercased, except in "ADR", which is an acronym.
func lowerSubject(e domain.EntityRef) string {
	s := subjectOf(e)
	if s == "" || strings.HasPrefix(s, "ADR") {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// refName names a referenced record: "ADR 0012" or "change 0172".
func refName(e domain.EntityRef) string {
	if e.Kind == domain.EntityADR {
		return fmt.Sprintf("ADR %04d", e.ID)
	}
	return fmt.Sprintf("change %04d", e.ID)
}

// relatedTarget is the record a reference finding points at: its first
// related record, or else a change named by Detail["target"].
func relatedTarget(f domain.Finding) domain.EntityRef {
	if len(f.Related) > 0 {
		return f.Related[0]
	}
	id, _ := strconv.Atoi(f.Detail["target"])
	return domain.EntityRef{Kind: domain.EntityChange, ID: id}
}

// missingTarget is the "but …" clause of an unresolved reference: the target
// is absent, or more than one record claims its number.
func missingTarget(f domain.Finding, target string) string {
	if f.Detail["lookup"] == "ambiguous" {
		return "but more than one record claims " + target
	}
	return "but no " + target + " exists"
}

// rawValue renders a stored value for a sentence.
func rawValue(raw string) string {
	if raw == "" {
		return "an empty value"
	}
	return "`" + raw + "`"
}

// fieldValue renders a field and its stored value: "`slug: Bad`", or
// "an empty `slug:`" when nothing is stored.
func fieldValue(field, raw string) string {
	if raw == "" {
		return "an empty `" + field + ":`"
	}
	return "`" + field + ": " + raw + "`"
}

// handEdit is the hand-edit-then-repair remedy for the finding's own record.
func handEdit(f domain.Finding, loc reposetup.RecordLocator, instruction string) string {
	return loc.HandEditThenRepair(instruction, validationRecordPath(f))
}

// countOthers is how many other records share a duplicated identity.
func countOthers(f domain.Finding) int {
	n, _ := strconv.Atoi(f.Detail["count"])
	if n-1 < 1 {
		return 1
	}
	return n - 1
}

// plural renders "1 record" or "2 records".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// cycleMemberList is the cycle's member ids, as the finding carries them.
func cycleMemberList(f domain.Finding) []string {
	var out []string
	for _, m := range strings.Split(f.Detail["members"], ",") {
		if m = strings.TrimSpace(m); m != "" {
			out = append(out, m)
		}
	}
	return out
}

// cycleMembers renders the cycle's members as "0012 and 0013" or
// "0012, 0013, and 0014".
func cycleMembers(f domain.Finding) string {
	m := cycleMemberList(f)
	switch len(m) {
	case 0:
		return ""
	case 1:
		return m[0]
	case 2:
		return m[0] + " and " + m[1]
	}
	return strings.Join(m[:len(m)-1], ", ") + ", and " + m[len(m)-1]
}

// suppliedAsNoun names what an artifact field's path really is.
func suppliedAsNoun(kind string) string {
	switch domain.EntityKind(kind) {
	case domain.EntityChange:
		return "a change record"
	case domain.EntityADR:
		return "an ADR record"
	case domain.EntityLearning:
		return "a learning record"
	case domain.EntityDerived:
		return "a generated file such as the board"
	}
	return "a `" + kind + "` record"
}

const (
	// harmlessLink is the impact of an unresolved cross-link nothing waits on.
	harmlessLink = "It's harmless: the link just leads nowhere, and nothing waits on it."
	// slugRule describes the shared slug and topic grammar
	// (domain.ValidSlugToken) in words.
	slugRule = "lowercase letters and digits in words joined by single hyphens"
	// typeRule describes the change-type grammar (domain.ValidTypeToken) in
	// words. It is not the slug rule: a type must start with a letter and may
	// hold any run of hyphens.
	typeRule = "a lowercase letter followed only by lowercase letters, digits, or hyphens"
	// reportBug opens the remedy of a finding that reports a docket
	// defect rather than a problem in a record.
	reportBug = "Report it as a docket bug and include the path; "
)

// identityInvalid builds the change-id-invalid / adr-id-invalid text.
func identityInvalid(noun, impact string) validationExplainer {
	return func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: idInvalidMessage(f, noun),
			Impact:  impact,
			Remedy:  handEdit(f, loc, "set `id:` to the number at the start of the file name"),
		}
	}
}

// idInvalidMessage words the id-invalid message. The validator reports an
// absent, zero, or non-numeric id alike as 0, so that case says the record
// has no valid id rather than quoting `id: 0` as if it were stored.
func idInvalidMessage(f domain.Finding, noun string) string {
	if raw := f.Detail["raw"]; raw != "" && raw != "0" {
		return fmt.Sprintf("%s has `id: %s`, which isn't a usable %s number.", subjectOf(f.Entity), raw, noun)
	}
	return fmt.Sprintf("%s has no valid `id:` (it is missing, zero, or not a number), so it has no usable %s number.", subjectOf(f.Entity), noun)
}

// slugInvalid builds the change/ADR/learning slug-invalid text.
func slugInvalid(impact string) validationExplainer {
	return func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, which isn't %s.", subjectOf(f.Entity), fieldValue("slug", f.Detail["raw"]), slugRule),
			Impact:  impact,
			Remedy:  handEdit(f, loc, "set `slug:` to "+slugRule+", and rename the file to match"),
		}
	}
}

// filenameMismatch builds the change/ADR filename-mismatch text.
func filenameMismatch(f domain.Finding, loc reposetup.RecordLocator) findingText {
	return findingText{
		Message: fmt.Sprintf("%s's file is named `%s`, but its frontmatter says `id: %s` and `slug: %s`.",
			subjectOf(f.Entity), f.Detail["base"], f.Detail["id"], f.Detail["slug"]),
		Impact: "Links and lookups that build the file name from the id and slug miss this record.",
		Remedy: handEdit(f, loc, "make the file name and the `id:` and `slug:` fields agree"),
	}
}

// numberDuplicate builds the change/ADR id-duplicate and learning
// slug-duplicate text.
func numberDuplicate(identity, field, impact string) validationExplainer {
	return func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s's %s is also used by %s.", subjectOf(f.Entity), identity, plural(countOthers(f), "other record")),
			Impact:  impact,
			Remedy:  handEdit(f, loc, "give one of them an unused `"+field+":` and rename its file to match"),
		}
	}
}

// removeReference is the instruction that drops an unresolved list entry.
func removeReference(f domain.Finding) string {
	return fmt.Sprintf("remove `%d` from `%s:` or correct it to the right number", relatedTarget(f).ID, f.Field)
}

// lowerFirst lower-cases the first rune of s so a remedy sentence can follow
// a colon mid-sentence.
func lowerFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToLower(r)) + s[n:]
}

// priorityRemedy is the change-priority-unknown remedy. A proposed change can
// take the typed groom revise, which sets the priority and keeps the board in
// step, so that comes first and the hand edit is the fallback; the revise
// refuses any other status, which gets the hand edit alone.
func priorityRemedy(f domain.Finding, loc reposetup.RecordLocator) string {
	hand := handEdit(f, loc, "set `priority:` to critical, high, medium, or low")
	if f.Detail["status"] != string(domain.StatusProposed) {
		return hand
	}
	return "Revise the change with docket-groom-next, or run `docket change groom` with a revise request, and set its priority to critical, high, medium, or low. To edit it by hand instead: " + lowerFirst(hand)
}

// deliverableRemedy is the change-deliverable-unknown remedy, ordered like
// priorityRemedy: the typed groom revise first for a proposed change, the
// hand edit alone for any other status.
func deliverableRemedy(f domain.Finding, loc reposetup.RecordLocator) string {
	hand := handEdit(f, loc, "set `deliverable:` to pr or results-artifact")
	if f.Detail["status"] != string(domain.StatusProposed) {
		return hand
	}
	return "Revise the change with docket-groom-next, or run `docket change groom` with a revise request, and set its deliverable to pr or results-artifact. To edit it by hand instead: " + lowerFirst(hand)
}

// validationTexts is the plain-language table, keyed by validator code.
var validationTexts = map[string]validationExplainer{
	repository.CodeRecordUndecodable: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("docket can't read the frontmatter of `%s`: %s.", validationRecordPath(f), strings.TrimRight(f.Detail["error"], ". ")),
			Impact:  "docket skips this record, so it is missing from the board and from every other check.",
			Remedy:  handEdit(f, loc, "fix the frontmatter between the two `---` lines"),
		}
	},
	repository.CodeFieldMalformed: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		msg := fmt.Sprintf("%s has `%s: %s`, which isn't a valid value for that field.", subjectOf(f.Entity), f.Field, f.Detail["raw"])
		if f.Detail["raw"] == "" {
			msg = fmt.Sprintf("%s has a `%s` value docket can't read.", subjectOf(f.Entity), f.Field)
		}
		impact := fmt.Sprintf("docket ignores the value, so everything that reads `%s` sees it as missing.", f.Field)
		if f.Severity == domain.SeverityWarning {
			impact = fmt.Sprintf("It's mostly harmless: docket treats `%s` as unset.", f.Field)
		}
		return findingText{Message: msg, Impact: impact, Remedy: handEdit(f, loc, "fix the `"+f.Field+":` value")}
	},
	repository.CodeListItemMalformed: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		pos, _ := strconv.Atoi(f.Detail["index"])
		return findingText{
			Message: fmt.Sprintf("%s's `%s` list has an entry docket can't read: %s (position %d).", subjectOf(f.Entity), f.Field, rawValue(f.Detail["raw"]), pos+1),
			Impact:  "docket skips that entry and keeps the rest of the list.",
			Remedy:  handEdit(f, loc, "fix or remove that entry in `"+f.Field+":`"),
		}
	},
	repository.CodeChangeTypeInvalid: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, but a type must be %s.", subjectOf(f.Entity), fieldValue("type", f.Detail["raw"]), typeRule),
			Impact:  "docket can't label or group the change by its type.",
			Remedy:  handEdit(f, loc, "set `type:` to "+typeRule+", such as `feature` or `fix`"),
		}
	},
	repository.CodeADRStatusUnparseable: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, which isn't an ADR status.", subjectOf(f.Entity), fieldValue("status", f.Detail["raw"])),
			Impact:  "docket can't tell whether the decision is in force, so the ADR index can't list it correctly.",
			Remedy:  handEdit(f, loc, "set `status:` to `Accepted`, `Deprecated`, `Superseded by ADR-NNNN`, or `Reversed by ADR-NNNN`"),
		}
	},
	repository.CodeLearningPromotionUnknown: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, which isn't a promotion state.", subjectOf(f.Entity), fieldValue("promotion_state", f.Detail["raw"])),
			Impact:  "docket can't tell whether the learning was promoted, so the learnings index can't place it.",
			Remedy:  handEdit(f, loc, "set `promotion_state:` to `retained`, `candidate`, or `promoted`"),
		}
	},
	repository.CodeChangeIDInvalid: identityInvalid("change",
		"Nothing can refer to this change by number, so it can't be selected, depended on, or built."),
	repository.CodeADRIDInvalid: identityInvalid("ADR",
		"Other ADRs and changes can't refer to this decision, and the ADR index can't list it."),
	repository.CodeChangeSlugInvalid: slugInvalid(
		"docket names the change's branch after its slug, so it refuses to claim this change for building."),
	repository.CodeADRSlugInvalid: slugInvalid(
		"The ADR's file name and links can't be built from its slug."),
	repository.CodeLearningSlugInvalid: slugInvalid(
		"The learnings index can't link the learning by its slug."),
	repository.CodeChangeStatusUnknown: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		msg := fmt.Sprintf("%s has `status: %s`, which isn't a change status.", subjectOf(f.Entity), f.Detail["raw"])
		if f.Detail["raw"] == "" {
			msg = fmt.Sprintf("%s has no `status:`.", subjectOf(f.Entity))
		}
		return findingText{
			Message: msg,
			Impact:  "docket can't place the change on the board or decide whether it is ready to build.",
			Remedy:  handEdit(f, loc, "set `status:` to one of proposed, in-progress, blocked, deferred, implemented, stacked-merged, done, or killed"),
		}
	},
	repository.CodeChangePriorityUnknown: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, which isn't a priority.", subjectOf(f.Entity), fieldValue("priority", f.Detail["raw"])),
			Impact:  "It's harmless: docket sorts the change as medium priority.",
			Remedy:  priorityRemedy(f, loc),
		}
	},
	repository.CodeChangeDeliverableUnknown: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s has %s, which isn't a deliverable.", subjectOf(f.Entity), fieldValue("deliverable", f.Detail["raw"])),
			Impact:  "docket can't tell how this change ends, so both `docket change complete` and `docket change mark-implemented` refuse it until it is fixed.",
			Remedy:  deliverableRemedy(f, loc),
		}
	},
	repository.CodeChangeFilenameMismatch: filenameMismatch,
	repository.CodeADRFilenameMismatch:    filenameMismatch,
	repository.CodeChangeIDDuplicate: numberDuplicate("number", "id",
		"docket can't tell these changes apart, so every command that names this number refuses."),
	repository.CodeADRIDDuplicate: numberDuplicate("number", "id",
		"docket can't tell them apart, so references to this one are ambiguous."),
	repository.CodeLearningSlugDuplicate: numberDuplicate("slug", "slug",
		"docket can't tell them apart, so references to this one are ambiguous."),
	repository.CodeChangeSlugDuplicate: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		n := countOthers(f)
		verb := "use it too"
		if n == 1 {
			verb = "uses it too"
		}
		return findingText{
			Message: fmt.Sprintf("%s uses `slug: %s`, and %s %s.", subjectOf(f.Entity), f.Entity.Slug, plural(n, "other change"), verb),
			Impact:  "Their branches would get the same name, so building one can collide with the other.",
			Remedy:  handEdit(f, loc, "give one of them a different `slug:` and rename its file to match"),
		}
	},
	repository.CodeRecordPathDuplicate: func(f domain.Finding, _ reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("docket was handed `%s` %s times.", f.Entity.Path, f.Detail["count"]),
			Impact:  "This is a docket bug, not a problem in the record: lookups of this record report it as ambiguous.",
			Remedy:  reportBug + "the record itself doesn't need an edit.",
		}
	},
	repository.CodeChangePlacementInvalid: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		status := f.Detail["status"]
		if f.Detail["placement"] == string(repository.LocationArchive) {
			return findingText{
				Message: fmt.Sprintf("%s's file is in `archive/`, but its status is `%s`, not done or killed.", subjectOf(f.Entity), status),
				Impact:  "docket treats the change as closed even though its status says it is still open.",
				Remedy:  handEdit(f, loc, "set `status:` to `done` or `killed`, or move the file back into `active/` without its date prefix"),
			}
		}
		return findingText{
			Message: fmt.Sprintf("%s is `%s` but its file is still in `active/`.", subjectOf(f.Entity), status),
			Impact:  "The board and the backlog keep showing a change that is closed.",
			Remedy:  handEdit(f, loc, "move the file into `archive/` with today's date in front of its name (`YYYY-MM-DD-`), or set `status:` back to the change's real state"),
		}
	},
	repository.CodeChangeArchiveDateInvalid: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("The archived file `%s` doesn't start with a `YYYY-MM-DD-` date.", f.Detail["base"]),
			Impact:  "docket can't tell when the change was archived, so it sorts out of order in the archive.",
			Remedy:  handEdit(f, loc, "rename the file to start with the date it was archived, such as `2026-05-28-`"),
		}
	},
	repository.CodeChangeStateIncoherent: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		status := f.Detail["status"]
		return findingText{
			Message: fmt.Sprintf("%s is `%s` but has no `%s`.", subjectOf(f.Entity), status, f.Field),
			Impact:  fmt.Sprintf("Commands that expect `%s` on a change that is `%s` can't work on this change.", f.Field, status),
			Remedy:  handEdit(f, loc, "add the missing `"+f.Field+":` value, or set `status:` to the change's real state"),
		}
	},
	repository.CodeChangeReferenceDangling: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		target := refName(relatedTarget(f))
		var impact string
		switch f.Field {
		case "depends_on":
			impact = fmt.Sprintf("docket counts %s as waiting on %s, so it never becomes ready to build.", lowerSubject(f.Entity), target)
		case "stacked_on":
			impact = "docket can't find the change this one is built on, so it can't build or finish this change."
		default:
			impact = harmlessLink
		}
		return findingText{
			Message: fmt.Sprintf("%s lists %s under `%s`, %s.", subjectOf(f.Entity), target, f.Field, missingTarget(f, target)),
			Impact:  impact,
			Remedy:  handEdit(f, loc, removeReference(f)),
		}
	},
	repository.CodeChangeDependencyCycle: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		members := cycleMembers(f)
		if len(cycleMemberList(f)) == 1 {
			return findingText{
				Message: fmt.Sprintf("Change %s depends on itself through `depends_on`.", members),
				Impact:  "It can never become ready to build, because it waits on itself.",
				Remedy:  loc.HandEditThenRepair(fmt.Sprintf("remove `%d` from `depends_on:` in change %s's record", f.Entity.ID, members), ""),
			}
		}
		return findingText{
			Message: fmt.Sprintf("Changes %s depend on each other in a loop through `depends_on`.", members),
			Impact:  "None of them can ever become ready to build, because each waits on another.",
			Remedy:  loc.HandEditThenRepair("remove one `depends_on` link between changes "+members+" in its change record", ""),
		}
	},
	repository.CodeChangeStackCycle: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		members := cycleMembers(f)
		if len(cycleMemberList(f)) == 1 {
			return findingText{
				Message: fmt.Sprintf("Change %s is stacked on itself through `stacked_on`.", members),
				Impact:  "It has no base branch to build on, so it can't be built or finished.",
				Remedy:  loc.HandEditThenRepair("remove or correct `stacked_on:` in change "+members+"'s record", ""),
			}
		}
		return findingText{
			Message: fmt.Sprintf("Changes %s are stacked on each other in a loop through `stacked_on`.", members),
			Impact:  "None of them has a base branch to build on, so none can be built or finished.",
			Remedy:  loc.HandEditThenRepair("remove or correct one `stacked_on` value among changes "+members+" in its change record", ""),
		}
	},
	repository.CodeArtifactReferenceKindMismatch: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s's `%s: %s` points at %s, not a spec, plan, or results file.", subjectOf(f.Entity), f.Field, f.Detail["path"], suppliedAsNoun(f.Detail["supplied-as"])),
			Impact:  fmt.Sprintf("docket can't open or link it as the change's %s.", f.Field),
			Remedy:  handEdit(f, loc, "point `"+f.Field+":` at the right file, or clear it"),
		}
	},
	repository.CodeLearningFilenameMismatch: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		slug := f.Detail["slug"]
		return findingText{
			Message: fmt.Sprintf("The learning file `%s` doesn't match its %s.", f.Detail["base"], fieldValue("slug", slug)),
			Impact:  "Links that build the file name from the slug lead nowhere.",
			Remedy:  handEdit(f, loc, "rename the file to `"+slug+".md` or fix `slug:` to match the file name"),
		}
	},
	repository.CodeLearningTopicInvalid: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("%s lists a topic, %s, which isn't %s.", subjectOf(f.Entity), rawValue(f.Detail["raw"]), slugRule),
			Impact:  "The learnings index can't group the learning under that topic.",
			Remedy:  handEdit(f, loc, "fix or remove that entry in `topics:`"),
		}
	},
	repository.CodeLearningPromotionDestination: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		if f.Severity == domain.SeverityError {
			return findingText{
				Message: fmt.Sprintf("%s is marked promoted, but `promoted_to:` is empty.", subjectOf(f.Entity)),
				Impact:  "Nobody can tell which instructions file the rule was promoted into.",
				Remedy:  handEdit(f, loc, "set `promoted_to:` to the file it was promoted into, or change `promotion_state:`"),
			}
		}
		state := "isn't set"
		if s := f.Detail["promotion_state"]; s != "" {
			state = "is `" + s + "`"
		}
		return findingText{
			Message: fmt.Sprintf("%s names `promoted_to: %s`, but its `promotion_state` %s, not promoted.", subjectOf(f.Entity), f.Detail["raw"], state),
			Impact:  "It's harmless: docket ignores `promoted_to:` unless the learning is promoted.",
			Remedy:  handEdit(f, loc, "clear `promoted_to:`, or set `promotion_state: promoted` if it really was promoted"),
		}
	},
	repository.CodeLearningReferenceDangling: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		target := refName(relatedTarget(f))
		return findingText{
			Message: fmt.Sprintf("%s lists %s under `%s`, %s.", subjectOf(f.Entity), target, f.Field, missingTarget(f, target)),
			Impact:  harmlessLink,
			Remedy:  handEdit(f, loc, removeReference(f)),
		}
	},
	repository.CodeRecordUnaccounted: func(f domain.Finding, _ reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("docket read `%s` but couldn't match it to any record.", f.Entity.Path),
			Impact:  "This is a docket bug: the file is skipped without a reason being reported.",
			Remedy:  reportBug + "nothing in the file needs to change.",
		}
	},
	domain.CodeADRBackpointerMismatch: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		impact := "The ADR index shows the two decisions inconsistently, so a reader can't tell which one is in force."
		target := relatedTarget(f)
		if f.Field == "status" {
			other := "ADR " + f.Detail["expected-on"]
			edge := f.Detail["missing-edge"]
			return findingText{
				Message: fmt.Sprintf("%s has `status: %s`, but %s doesn't list %s under `%s`.", subjectOf(f.Entity), f.Detail["status"], other, f.Detail["expected-edge"], edge),
				Impact:  impact,
				// The file leads the remedy, so this record's own edit comes
				// first and the other ADR's edit names its file itself.
				Remedy: handEdit(f, loc, fmt.Sprintf("fix `status:`, or add `%d` to `%s:` in %s's own file",
					f.Entity.ID, edge, other)),
			}
		}
		name := refName(target)
		return findingText{
			Message: fmt.Sprintf("%s lists %s under `%s`, but %s's status is `%s`, not `%s`.", subjectOf(f.Entity), name, f.Field, name, f.Detail["actual"], f.Detail["expected"]),
			Impact:  impact,
			Remedy: handEdit(f, loc, fmt.Sprintf("remove `%d` from `%s:`, or set %s's `status:` to `%s` in %s's own file",
				target.ID, f.Field, name, f.Detail["expected"], name)),
		}
	},
	domain.CodeADRDanglingReference: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		target := refName(relatedTarget(f))
		impact := "The ADR index can't connect the two decisions, so the relationship is lost."
		if f.Severity == domain.SeverityWarning {
			impact = harmlessLink
		}
		var instruction string
		switch f.Field {
		case "status":
			instruction = "set `status:` to name an ADR that exists, or back to `Accepted`"
		case "change":
			instruction = "correct `change:` to the number of the change that produced this ADR, or delete the line"
		default:
			instruction = removeReference(f)
		}
		return findingText{
			Message: fmt.Sprintf("%s names %s in `%s`, %s.", subjectOf(f.Entity), target, f.Field, missingTarget(f, target)),
			Impact:  impact,
			Remedy:  handEdit(f, loc, instruction),
		}
	},
	domain.CodeADRIDGap: func(f domain.Finding, _ reposetup.RecordLocator) findingText {
		return findingText{
			Message: fmt.Sprintf("There is no ADR %04d, though ADR numbers go up to %s.", f.Entity.ID, f.Detail["highest"]),
			Impact:  "It's harmless: docket never reuses a missing number. This usually means an ADR was deleted or never written.",
			Remedy:  fmt.Sprintf("Nothing to do. If ADR %04d was deleted by mistake, restore its file from the metadata branch's git history.", f.Entity.ID),
		}
	},
	domain.CodeADRSelfReference: func(f domain.Finding, loc reposetup.RecordLocator) findingText {
		instruction := "remove its own number from `" + f.Field + ":`"
		if f.Field == "status" {
			instruction = "set `status:` to name a different ADR, or back to `Accepted`"
		}
		return findingText{
			Message: fmt.Sprintf("%s names itself in `%s`.", subjectOf(f.Entity), f.Field),
			Impact:  "A decision can't supersede, reverse, or relate to itself, so the ADR index shows a broken relationship.",
			Remedy:  handEdit(f, loc, instruction),
		}
	},
}
