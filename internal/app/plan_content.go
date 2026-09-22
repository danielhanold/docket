package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// This file is the plan-side placeholder check (change 0414). change.attach-plan
// used to refuse any plan whose bytes contained a whole-word planning token
// (todo, fixme, tbd, tktk, xxx, placeholder — uppercase in real documents),
// which rejected legitimate instructions ("remove the marker in retry.go"),
// code examples, and human-approved ambiguous prose. The corrected rule
// refuses only a WHOLE-SLOT filler: a section (or the pre-heading preamble)
// whose entire authored body IS one bare token — an unfilled slot, not a
// mention. Completeness beyond that is plan authoring's and review's job;
// attachment verifies structure, never judges prose.

// planFillerTokens is the closed placeholder vocabulary a whole slot may not
// reduce to (compared lowercased). It mirrors the writing-plans
// "No Placeholders" token set; it is a slot test, never a word search.
var planFillerTokens = map[string]bool{
	"tbd": true, "todo": true, "fixme": true, "tktk": true, "xxx": true, "placeholder": true,
}

// isPlanFillerBody reports whether a slot's entire body reduces
// (whitespace-trimmed, lowercased, one optional trailing period removed) to a
// bare placeholder token. An empty body is NOT filler — empty sections stay
// with authoring/review judgment, exactly like results' isResultsFillerBody.
// The reduction itself lives in the shared reducesToToken helper (results_content.go).
func isPlanFillerBody(body string) bool {
	return reducesToToken(body, planFillerTokens)
}

// planPlaceholderSlot reports the first unfilled slot of a committed plan: a
// heading's direct body (to the next heading of any level), or the document's
// pre-heading preamble, whose entire authored content is one bare placeholder
// token. Frontmatter and generated managed blocks are not author content;
// fenced lines never parse as headings but DO count as slot content (code is
// substantive). The returned slot names the offending section for the refusal
// message. A non-parsing document returns not-found: at the attach seam the
// backlink guard (verifyBacklink) has already refused it before this runs.
func planPlaceholderSlot(source []byte) (string, bool) {
	doc, err := document.Parse(source)
	if err != nil {
		return "", false
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
	fenced := fenceMask(lines)
	fmEnd := frontmatterEnd(lines)

	type hdr struct {
		line int
		text string
	}
	var headings []hdr
	for i, ln := range lines {
		if i < fmEnd || fenced[i] || inManaged(ln.start) {
			continue
		}
		if _, ht, ok := parseResultsHeading(strings.TrimRight(ln.text, "\r")); ok {
			headings = append(headings, hdr{line: i, text: ht})
		}
	}

	slotBody := func(from, to int) string {
		var parts []string
		for i := from; i < to; i++ {
			if i < fmEnd || inManaged(lines[i].start) {
				continue
			}
			t := strings.TrimRight(lines[i].text, "\r")
			if strings.TrimSpace(t) != "" {
				parts = append(parts, t)
			}
		}
		return strings.Join(parts, "\n")
	}

	firstHeading := len(lines)
	if len(headings) > 0 {
		firstHeading = headings[0].line
	}
	if isPlanFillerBody(slotBody(0, firstHeading)) {
		return "the document preamble", true
	}
	for hi, h := range headings {
		end := len(lines)
		if hi+1 < len(headings) {
			end = headings[hi+1].line
		}
		if isPlanFillerBody(slotBody(h.line+1, end)) {
			return fmt.Sprintf("section %q", h.text), true
		}
	}
	return "", false
}
