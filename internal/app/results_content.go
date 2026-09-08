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
// a required, substantive ## Outcome and no empty/filler sections — binds only at
// the implemented boundary.

// ResultsPhase selects how strict ValidateResultsContent is: a checkpoint attach
// during implementation vs. the final implemented-boundary contract.
type ResultsPhase int

const (
	// ResultsPhaseCheckpoint runs ONLY the both-phases checks (parse, title,
	// placeholder) — a truthful in-progress artifact passes.
	ResultsPhaseCheckpoint ResultsPhase = iota
	// ResultsPhaseFinal adds the full content contract: a required, substantive
	// ## Outcome and no empty or filler sections.
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
	fenced := make([]bool, len(lines))
	fenceChar := byte(0)
	for i, ln := range lines {
		trimmed := strings.TrimLeft(strings.TrimRight(ln.text, "\r"), " \t")
		if run := fenceRunLen(trimmed); run >= 3 {
			fenced[i] = true // the fence marker line itself is fenced territory
			if fenceChar == 0 {
				fenceChar = trimmed[0]
			} else if trimmed[0] == fenceChar {
				fenceChar = 0
			}
			continue
		}
		fenced[i] = fenceChar != 0
	}

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

	// Both phases: no unfilled authoring placeholder (first occurrence).
	for i, ln := range lines {
		if fenced[i] || inManaged(ln.start) {
			continue
		}
		if isResultsPlaceholderLine(strings.TrimRight(ln.text, "\r")) {
			findings = append(findings, ResultsContentFinding{
				Reason:  reasonResultsPlaceholder,
				Message: "the results artifact still carries unfilled authoring placeholder scaffolding",
			})
			break
		}
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

// isResultsPlaceholderLine reports whether a line is unfilled authoring
// scaffolding: (a) any whole-word placeholder token (reusing change_attach.go's
// placeholderTokenRE alternation verbatim), or (b) a line whose text — after
// stripping leading heading/list markers and whitespace — begins with `<` and is
// neither an HTML comment (`<!--`) nor an autolink (`<http`).
func isResultsPlaceholderLine(text string) bool {
	if placeholderTokenRE.MatchString(text) {
		return true
	}
	s := stripResultsLeadMarkers(text)
	return strings.HasPrefix(s, "<") && !strings.HasPrefix(s, "<!--") && !strings.HasPrefix(s, "<http")
}

// stripResultsLeadMarkers removes leading whitespace, heading markers, and one
// list/blockquote marker so an angle-bracket scaffold under any of them is seen.
func stripResultsLeadMarkers(text string) string {
	s := strings.TrimLeft(text, " \t")
	for strings.HasPrefix(s, "#") {
		s = s[1:]
	}
	s = strings.TrimLeft(s, " \t")
	switch {
	case s == "":
		return s
	case s[0] == '-' || s[0] == '*' || s[0] == '+' || s[0] == '>':
		s = strings.TrimLeft(s[1:], " \t")
	default:
		j := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j > 0 && j < len(s) && (s[j] == '.' || s[j] == ')') {
			s = strings.TrimLeft(s[j+1:], " \t")
		}
	}
	return s
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
