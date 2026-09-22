package app

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// This file is the shared results-content validator. Two Go completion seams
// (change.attach-results at the checkpoint boundary, ChangeMarkImplemented and
// RunVerify at the final boundary) call ValidateResultsContent to decide whether
// an authored results artifact is truthful enough for its phase. It is a focused
// validator over the record's own byte view — it reuses internal/document's
// parse/managed-block machinery and never becomes a second Markdown framework.
//
// The PHASE is explicit precisely so shared helper code can never accidentally
// claim final completion: a checkpoint attach runs only the both-phases checks
// (a truthful in-progress artifact attaches), while the final content contract —
// a required substantive Human action statement, a required, substantive ## Outcome,
// and no empty/filler sections — binds only at the implemented boundary.

// ResultsPhase selects how strict ValidateResultsContent is: a checkpoint attach
// during implementation vs. the final implemented-boundary contract.
type ResultsPhase int

const (
	// ResultsPhaseCheckpoint runs ONLY the both-phases checks (parse, title,
	// placeholder) — a truthful in-progress artifact passes.
	ResultsPhaseCheckpoint ResultsPhase = iota
	// ResultsPhaseFinal adds the full content contract: a required substantive
	// Human action statement, a required, substantive `## Outcome`, and no empty
	// or filler sections.
	ResultsPhaseFinal
)

// ResultsContentFinding is one content defect. Reason is a stable machine string
// (the vocabulary below); Message is explanatory diagnostic text and must not be
// parsed.
type ResultsContentFinding struct {
	Reason  string
	Message string
}

// Stable machine reasons ValidateResultsContent reports. Additive,
// lowercase-hyphen; message text is never parsed.
const (
	reasonResultsDocMalformed   = "results-doc-malformed"
	reasonResultsTitleMissing   = "results-title-missing"
	reasonResultsPlaceholder    = "results-placeholder"
	reasonResultsOutcomeMissing = "results-outcome-missing"
	reasonResultsOutcomeEmpty   = "results-outcome-empty"
	reasonResultsEmptySection   = "results-empty-section"
	reasonResultsFillerSection  = "results-filler-section"

	// reasonResultsTemplateInvalid: the canonical results template could not
	// supply authoring prompts (missing/malformed/empty embedded asset). A
	// validator SETUP failure — it blames the binary's template, never the
	// author's document, and it fails closed (change 0414).
	reasonResultsTemplateInvalid = "results-template-invalid"

	reasonResultsActionStatementMissing = "results-action-statement-missing"
	reasonResultsActionStatementEmpty   = "results-action-statement-empty"
)

// fillerBodies is the closed set of whole-section filler bodies: a section whose
// entire body reduces (whitespace-stripped, lowercased, one optional trailing
// dot removed) to one of these is filler. Prose that merely MENTIONS one of these
// inside a longer body is legal.
var fillerBodies = map[string]bool{
	"none":           true,
	"n/a":            true,
	"not applicable": true,
	"no findings":    true,
	"nothing":        true,
	"-":              true,
	"—":              true,
}

// rcLine is one physical source line with the byte offset of its first byte, so
// a line can be tested against document.Parse's managed-block spans.
type rcLine struct {
	text  string
	start int
}

// ValidateResultsContent reports the content defects of a results artifact for
// the given phase. An empty slice means valid. It never mutates source.
func ValidateResultsContent(source []byte, phase ResultsPhase) []ResultsContentFinding {
	doc, err := document.Parse(source)
	if err != nil {
		// A malformed managed-block population (dangling/out-of-order/nested
		// markers) or invalid UTF-8 fails the parse; nothing else can be trusted.
		return []ResultsContentFinding{{
			Reason:  reasonResultsDocMalformed,
			Message: fmt.Sprintf("the results document does not parse: %v", err),
		}}
	}

	lines := splitResultsLines(source)
	blocks := doc.Blocks()
	inManaged := func(start int) bool {
		for _, b := range blocks {
			if start >= b.Start.Start && start < b.End.End {
				return true
			}
		}
		return false
	}

	// Fence-aware line classification: a line inside a fenced code block (``` or
	// ~~~) is authored content, not structure — its heading-shaped and
	// angle-bracket-shaped lines are ignored (section-slice-needs-a-named-terminator).
	fenced := fenceMask(lines)

	// Locate headings (outside fences and outside managed blocks) and body lines.
	type hdr struct {
		level, line int
		text        string
	}
	var headings []hdr
	isHeading := make([]bool, len(lines))
	hasTitle := false
	for i, ln := range lines {
		if fenced[i] || inManaged(ln.start) {
			continue
		}
		if lvl, ht, ok := parseResultsHeading(strings.TrimRight(ln.text, "\r")); ok {
			isHeading[i] = true
			headings = append(headings, hdr{level: lvl, line: i, text: ht})
			if lvl == 1 {
				hasTitle = true
			}
		}
	}
	body := make([]bool, len(lines))
	for i, ln := range lines {
		if inManaged(ln.start) || isHeading[i] {
			continue
		}
		if strings.TrimSpace(strings.TrimRight(ln.text, "\r")) != "" {
			body[i] = true
		}
	}

	var findings []ResultsContentFinding

	// Both phases: an H1 title must exist outside fences and managed blocks.
	if !hasTitle {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsTitleMissing,
			Message: "the results artifact has no H1 title line outside fenced code and managed blocks",
		})
	}

	// Both phases (change 0414): no complete occurrence of a reserved authoring
	// prompt DERIVED from the shipped results template — outside code,
	// comments, frontmatter, and managed blocks. Markup and autolinks pass
	// regardless of capitalization (they are not emitted prompts); a literal
	// copy of a prompt passes only in code or behind an escaped bracket;
	// custom placeholder-looking prose is the author's and reviewer's call.
	prompts, perr := resultsTemplatePrompts()
	if perr != nil {
		findings = append(findings, ResultsContentFinding{
			Reason: reasonResultsTemplateInvalid,
			Message: fmt.Sprintf("the canonical results template could not supply authoring prompts "+
				"(validator setup failure — the artifact was not judged): %v", perr),
		})
	} else if p, ok := firstTemplatePrompt(authorProse(lines, fenced, frontmatterEnd(lines), inManaged), prompts); ok {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsPlaceholder,
			Message: fmt.Sprintf("the results artifact still carries the unfilled template prompt %q", p),
		})
	}

	if phase != ResultsPhaseFinal {
		return findings
	}

	// Final phase: ## Outcome must exist and be substantive; every present H2/H3
	// section must be substantive and free of whole-section filler.
	ownBody := func(hi int) string {
		start := headings[hi].line
		end := len(lines)
		if hi+1 < len(headings) {
			end = headings[hi+1].line
		}
		var b []string
		for i := start + 1; i < end; i++ {
			if body[i] {
				b = append(b, strings.TrimRight(lines[i].text, "\r"))
			}
		}
		return strings.Join(b, "\n")
	}
	// substantive: the section has body text anywhere within its full extent —
	// its own body OR the body of any nested subsection (so a bodyless parent whose
	// only content is a filled child subsection is substantive).
	substantive := func(hi int) bool {
		start := headings[hi].line
		end := len(lines)
		for j := hi + 1; j < len(headings); j++ {
			if headings[j].level <= headings[hi].level {
				end = headings[j].line
				break
			}
		}
		for i := start + 1; i < end; i++ {
			if body[i] {
				return true
			}
		}
		return false
	}

	// Final phase (change 0440): a substantive "Human action:" statement must
	// sit between the H1 title and the first H2. The statement's body is its
	// same-line remainder plus its immediate continuation lines (the rest of
	// that paragraph), so a wrapped statement is not misread as empty.
	stmtFrom := 0
	for _, h := range headings {
		if h.level == 1 {
			stmtFrom = h.line + 1
			break
		}
	}
	stmtTo := len(lines)
	for _, h := range headings {
		if h.level == 2 && h.line >= stmtFrom {
			stmtTo = h.line
			break
		}
	}
	stmtFound := false
	for i := stmtFrom; i < stmtTo; i++ {
		if fenced[i] || inManaged(lines[i].start) || isHeading[i] {
			continue
		}
		stmtBody, ok := parseResultsActionStatement(strings.TrimRight(lines[i].text, "\r"))
		if !ok {
			continue
		}
		stmtFound = true
		parts := []string{stmtBody}
		for j := i + 1; j < stmtTo && body[j] && !isHeading[j] && !fenced[j]; j++ {
			parts = append(parts, strings.TrimSpace(strings.TrimRight(lines[j].text, "\r")))
		}
		joined := strings.TrimSpace(strings.Join(parts, "\n"))
		stillPrompt := false
		if perr == nil {
			_, stillPrompt = firstTemplatePrompt(joined, prompts)
		}
		if joined == "" || isResultsFillerBody(joined) || stillPrompt {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsActionStatementEmpty,
				Message: "the Human action statement after the title has no substantive text",
			})
		}
		break
	}
	if !stmtFound {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsActionStatementMissing,
			Message: "the final results artifact has no \"Human action:\" statement between the title and the first section",
		})
	}

	outcomeIdx := -1
	for hi, h := range headings {
		if h.level == 2 && strings.EqualFold(h.text, "Outcome") {
			outcomeIdx = hi
			break
		}
	}
	if outcomeIdx == -1 {
		findings = append(findings, ResultsContentFinding{
			Reason:  reasonResultsOutcomeMissing,
			Message: "the final results artifact has no ## Outcome section",
		})
	}

	for hi, h := range headings {
		if h.level != 2 && h.level != 3 {
			continue
		}
		if isResultsFillerBody(ownBody(hi)) {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsFillerSection,
				Message: fmt.Sprintf("section %q is whole-section filler; omit it instead of writing a None/N/A body", h.text),
			})
			continue
		}
		if substantive(hi) {
			continue
		}
		if hi == outcomeIdx {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsOutcomeEmpty,
				Message: "the ## Outcome section has no substantive body text",
			})
		} else {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsEmptySection,
				Message: fmt.Sprintf("section %q has no substantive body text and no substantive subsection", h.text),
			})
		}
	}

	return findings
}

// splitResultsLines splits source into physical lines carrying each line's byte
// start offset. The trailing segment after the final newline is included.
func splitResultsLines(src []byte) []rcLine {
	var lines []rcLine
	off := 0
	for {
		nl := bytes.IndexByte(src[off:], '\n')
		if nl < 0 {
			lines = append(lines, rcLine{text: string(src[off:]), start: off})
			return lines
		}
		lines = append(lines, rcLine{text: string(src[off : off+nl]), start: off})
		off += nl + 1
	}
}

// fenceRunLen returns the length of a leading ``` or ~~~ run (0 if the line is
// not a fence marker).
func fenceRunLen(trimmed string) int {
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return leadingRun(trimmed, '`')
	case strings.HasPrefix(trimmed, "~~~"):
		return leadingRun(trimmed, '~')
	default:
		return 0
	}
}

func leadingRun(s string, c byte) int {
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	return n
}

// fenceMask classifies each line as inside (or a marker of) a ``` / ~~~
// fenced code block. A fence closes only on a run of the SAME character at
// least as long as the run that opened it (CommonMark), so a shorter embedded
// run — a ``` example inside a ```` fence — cannot close the outer fence
// (change 0414). Fence-marker lines themselves classify as fenced.
func fenceMask(lines []rcLine) []bool {
	fenced := make([]bool, len(lines))
	fenceChar := byte(0)
	fenceLen := 0
	for i, ln := range lines {
		trimmed := strings.TrimLeft(strings.TrimRight(ln.text, "\r"), " \t")
		if run := fenceRunLen(trimmed); run >= 3 {
			if fenceChar == 0 {
				fenced[i] = true
				fenceChar = trimmed[0]
				fenceLen = run
				continue
			}
			fenced[i] = true
			if trimmed[0] == fenceChar && run >= fenceLen {
				fenceChar = 0
				fenceLen = 0
			}
			continue
		}
		fenced[i] = fenceChar != 0
	}
	return fenced
}

// frontmatterEnd returns the index of the first line after a leading YAML
// frontmatter block (a first line of exactly "---" closed by a later "---" or
// "..." line), or 0 when the document has none. An unclosed opener is not
// frontmatter — nothing is excluded.
func frontmatterEnd(lines []rcLine) int {
	if len(lines) == 0 || strings.TrimRight(lines[0].text, "\r") != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		t := strings.TrimRight(lines[i].text, "\r")
		if t == "---" || t == "..." {
			return i + 1
		}
	}
	return 0
}

// parseResultsHeading recognizes an ATX heading (`#`..`######` then whitespace
// then nonempty text). A `#`-run with no text is not a heading.
func parseResultsHeading(text string) (level int, htext string, ok bool) {
	n := leadingRun(text, '#')
	if n < 1 || n > 6 || n >= len(text) {
		return 0, "", false
	}
	if text[n] != ' ' && text[n] != '\t' {
		return 0, "", false
	}
	htext = strings.TrimSpace(text[n:])
	if htext == "" {
		return 0, "", false
	}
	return n, htext, true
}

// parseResultsActionStatement recognizes the required action-statement line
// (change 0440): optional emphasis markers, the label "Human action" in any
// case, optional emphasis around the colon, then the statement text. It
// returns the same-line text after the colon (emphasis and whitespace
// trimmed) and whether the line is an action-statement line at all. Detection
// is keyed on this label-plus-colon SHAPE — never on an enumerated list of
// statement spellings — so `**Human action:** …`, `**Human action**: …`, and
// a plain `Human action: …` all match.
func parseResultsActionStatement(text string) (string, bool) {
	s := strings.TrimLeft(text, " \t")
	s = strings.TrimLeft(s, "*_")
	s = strings.TrimLeft(s, " \t")
	const label = "human action"
	if len(s) < len(label) || !strings.EqualFold(s[:len(label)], label) {
		return "", false
	}
	s = strings.TrimLeft(s[len(label):], " \t*_")
	if s == "" || s[0] != ':' {
		return "", false
	}
	body := strings.TrimSpace(s[1:])
	body = strings.TrimSpace(strings.Trim(body, "*_"))
	return body, true
}

// firstTemplatePrompt reports the first reserved prompt (template order) whose
// complete, whitespace-normalized text occurs in the prose view. The prose is
// masked for inline literals here, so callers pass raw prose (whole-document
// author view, or an action-statement body). Because the reserved prompts are
// DERIVED from the shipped results template (change 0414), matching no longer
// guesses from capitalization: legitimate inline HTML (<details>, <BR>,
// <DETAILS>) and autolinks (<mailto:…>, <HTTPS://…>) are not emitted prompts
// and pass regardless of case, while a lowercase prompt like
// `<short name of the action>` is caught.
func firstTemplatePrompt(prose string, prompts []string) (string, bool) {
	scan := normalizeWS(maskInlineLiterals(prose))
	for _, p := range prompts {
		if strings.Contains(scan, p) {
			return p, true
		}
	}
	return "", false
}

// isResultsFillerBody reports whether a section's entire body reduces to a
// closed-set filler token (whitespace-stripped, lowercased, one optional trailing
// dot removed). An empty body is not filler (it is handled by substantiveness).
func isResultsFillerBody(body string) bool {
	s := strings.ToLower(strings.TrimSpace(body))
	if s == "" {
		return false
	}
	s = strings.TrimSpace(strings.TrimSuffix(s, "."))
	return fillerBodies[s]
}
