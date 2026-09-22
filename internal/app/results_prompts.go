package app

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/danielhanold/docket/internal/assets"
)

// This file derives the reserved authoring prompts of the canonical results
// template (change 0414). Results validation used to GUESS scaffolding from
// capitalization — "< then an uppercase letter" — which false-positived on
// uppercase HTML tags (<BR>, <DETAILS>) and URI schemes (<MAILTO:…>,
// <HTTPS://…>) and missed lowercase prompts. The corrected detector matches
// only the prompts the shipped template actually emits: derived here from the
// embedded asset, never hand-enumerated (ADR-0050), never guessed.

// resultsTemplateAssetPath is the canonical results template in the embedded
// bundle — the same file skills/docket-implement-next/results-template.md the
// coordinator copies when authoring results.
const resultsTemplateAssetPath = "skills/docket-implement-next/results-template.md"

// literalMask replaces a literal-example span (code, comment, escape, fenced
// or managed line) in a prose view. It is deliberately non-whitespace and can
// never occur in a prompt, so a masked span can neither match a prompt nor
// splice its neighbors into one under whitespace normalization.
const literalMask = "\x00"

var wsRunRE = regexp.MustCompile(`[ \t\r\n\f\v]+`)

// normalizeWS collapses every whitespace run to one space and trims the ends,
// so reflow and CRLF-vs-LF never change a prompt's identity. Case and
// punctuation stay exact.
func normalizeWS(s string) string {
	return strings.TrimSpace(wsRunRE.ReplaceAllString(s, " "))
}

// authorProse renders the author-content line stream of a document: leading
// frontmatter, fenced-code lines, and generated managed-block lines become the
// mask sentinel; everything else passes through with newlines preserved.
func authorProse(lines []rcLine, fenced []bool, fmEnd int, inManaged func(int) bool) string {
	var b strings.Builder
	for i, ln := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < fmEnd || fenced[i] || inManaged(ln.start) {
			b.WriteString(literalMask)
			continue
		}
		b.WriteString(ln.text)
	}
	return b.String()
}

// maskInlineLiterals replaces literal-example spans inside a prose stream with
// the mask sentinel: HTML comments (<!-- … -->; an unclosed opener masks to the
// end), inline code spans (a backtick run closed by an EQUAL-length run, per
// CommonMark; an unclosed run is literal text), and a backslash-escaped angle
// bracket (\< or \>).
func maskInlineLiterals(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], "<!--") {
			b.WriteString(literalMask)
			end := strings.Index(s[i+4:], "-->")
			if end < 0 {
				return b.String()
			}
			i += 4 + end + 3
			continue
		}
		if s[i] == '`' {
			run := 0
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			if off := findEqualBacktickRun(s[i+run:], run); off >= 0 {
				b.WriteString(literalMask)
				i += run + off + run
				continue
			}
			b.WriteString(s[i : i+run])
			i += run
			continue
		}
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == '<' || s[i+1] == '>') {
			b.WriteString(literalMask)
			i += 2
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// findEqualBacktickRun returns the offset in s of the first backtick run of
// exactly length n (a longer or shorter run does not close the span), or -1.
func findEqualBacktickRun(s string, n int) int {
	for j := 0; j < len(s); {
		if s[j] != '`' {
			j++
			continue
		}
		run := 0
		for j+run < len(s) && s[j+run] == '`' {
			run++
		}
		if run == n {
			return j
		}
		j += run
	}
	return -1
}

// extractTemplatePrompts derives the reserved authoring prompts from a results
// template: every complete angle-bracket instruction span outside HTML
// comments and code, whitespace-normalized, deduplicated, in first-appearance
// order. The derivation reads the CANONICAL template, never a submitted
// artifact. A template with an unterminated span or no prompts at all is a
// setup error — the caller must fail closed, not validate against nothing.
func extractTemplatePrompts(template []byte) ([]string, error) {
	lines := splitResultsLines(template)
	fenced := fenceMask(lines)
	prose := maskInlineLiterals(authorProse(lines, fenced, frontmatterEnd(lines), func(int) bool { return false }))
	var prompts []string
	seen := map[string]bool{}
	for i := 0; i < len(prose); i++ {
		if prose[i] != '<' {
			continue
		}
		end := strings.IndexByte(prose[i:], '>')
		if end < 0 {
			return nil, fmt.Errorf("results template carries an unterminated angle-bracket prompt (opened at byte offset %d of the prose view)", i)
		}
		span := prose[i : i+end+1]
		i += end
		if strings.Contains(span, literalMask) {
			continue // a masked fragment inside is a literal example, not a prompt
		}
		p := normalizeWS(span)
		if len(p) <= 2 {
			continue // "<>" carries no instruction
		}
		if !seen[p] {
			seen[p] = true
			prompts = append(prompts, p)
		}
	}
	if len(prompts) == 0 {
		return nil, fmt.Errorf("results template carries no angle-bracket authoring prompts")
	}
	return prompts, nil
}

// resultsTemplatePrompts returns the prompts derived from the EMBEDDED
// canonical template. The embedded asset is immutable for a given binary, so
// the derivation runs once and is cached. It is a func var only so tests can
// stub the setup-failure path; production never reassigns it.
var resultsTemplatePrompts func() ([]string, error) = sync.OnceValues(func() ([]string, error) {
	body, err := assets.Open(resultsTemplateAssetPath)
	if err != nil {
		return nil, fmt.Errorf("embedded results template unavailable: %w", err)
	}
	return extractTemplatePrompts(body)
})
