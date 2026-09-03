package render

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// SectionIntent names what an edit does to one owned top-level section.
type SectionIntent string

const (
	SectionPreserve SectionIntent = "preserve"
	SectionReplace  SectionIntent = "replace"
	SectionRemove   SectionIntent = "remove"
)

// AllSectionIntents is the closed section-intent set, exported for the
// request/result schema surface's section_intents vocabulary (change 0399).
// Change 0399's AST completeness guard holds it in correspondence with the
// SectionIntent const group above.
var AllSectionIntents = []SectionIntent{SectionPreserve, SectionReplace, SectionRemove}

// SectionEdit names one operation-owned top-level section by its exact heading
// line (e.g. "## Why killed") and what to do with it. Markdown is the section
// body WITHOUT the heading line; it must be empty unless Intent is replace.
type SectionEdit struct {
	Heading  string
	Intent   SectionIntent
	Markdown string
}

// Owned heading sets, exported for app operations.
var ChangeOwnedHeadings = []string{
	"## Why", "## What changes", "## Out of scope", "## Open questions",
	"## Why deferred", "## Why killed", "## Auto-groom blocked",
}

var LearningOwnedHeadings = []string{"## Apply", "## War story"}

// h2Prefix is the CommonMark ATX H2 lead-in; a top-level section boundary is any
// column-zero line beginning with it, outside fenced code blocks. "### " and
// deeper subheadings do not match (their third byte is '#', not a space), so
// they stay inside their enclosing section.
var h2Prefix = []byte("## ")

// codeFenceRE matches a CommonMark fenced-code-block delimiter: up to three
// leading spaces, then a run of at least three backticks or tildes. This mirrors
// the fence rules in internal/document/markers.go's codeFenceRE — the reference
// behavior — reimplemented locally because markers.go's helpers are private and
// marker-specific.
var codeFenceRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// physLine is one physical source line: [start, end) covers the terminator,
// [start, textEnd) excludes it.
type physLine struct {
	start   int
	textEnd int
	end     int
}

// splitLines tiles src into physical lines whose spans cover every byte exactly
// once, in order (mirrors document.scanLines' contract).
func splitLines(src []byte) []physLine {
	var out []physLine
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			textEnd := i
			if i > start && src[i-1] == '\r' {
				textEnd = i - 1
			}
			out = append(out, physLine{start: start, textEnd: textEnd, end: i + 1})
			start = i + 1
		}
	}
	if start < len(src) {
		out = append(out, physLine{start: start, textEnd: len(src), end: len(src)})
	}
	return out
}

// fenceRun returns the leading fence delimiter run of a code-fence line, and
// whether the line is one (mirrors document.fenceRun).
func fenceRun(text []byte) (string, bool) {
	m := codeFenceRE.FindSubmatch(text)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// isBareFence reports whether the line carries nothing but its delimiter run and
// whitespace — CommonMark's rule for a CLOSING fence (mirrors document.isBareFence).
func isBareFence(text []byte, run string) bool {
	rest := bytes.TrimLeft(text, " ")[len(run):]
	return len(bytes.TrimSpace(rest)) == 0
}

// h2Heading is one located top-level section heading: its exact line content and
// the byte offset where its line begins.
type h2Heading struct {
	heading string
	start   int
}

// scanH2Headings returns the top-level "## " headings in src, in source order,
// skipping fenced code blocks so that marker-shaped or heading-shaped example
// text inside a fence is treated as authored content.
func scanH2Headings(src []byte) []h2Heading {
	var heads []h2Heading
	fence := ""          // the open fence's delimiter run; "" when not inside a fence
	fenceChar := byte(0) // '`' or '~'
	for _, ln := range splitLines(src) {
		text := src[ln.start:ln.textEnd]
		if run, ok := fenceRun(text); ok {
			switch {
			case fence == "":
				fence, fenceChar = run, run[0]
			case run[0] == fenceChar && len(run) >= len(fence) && isBareFence(text, run):
				fence, fenceChar = "", 0
			}
			continue
		}
		if fence != "" {
			continue // inside a fenced code block: heading-shaped text is content
		}
		if bytes.HasPrefix(text, h2Prefix) {
			heads = append(heads, h2Heading{heading: string(text), start: ln.start})
		}
	}
	return heads
}

// ApplySectionEdits splices edits into src, touching only owned sections.
//   - owned is the closed set of headings this operation may touch; an edit
//     naming a heading outside owned is an error.
//   - Headings are matched as exact full lines at column 0, outside fenced code
//     blocks.
//   - A section spans from its heading line to the line before the next
//     top-level "## " heading outside fences, or EOF.
//   - Every owned heading present in src must be unique; a duplicate refuses the
//     whole edit set. All edits are validated before any splice.
//   - replace: substitute the section body (appended at EOF, preceded by exactly
//     one blank line, when the section is absent). remove: delete heading and
//     body (error when absent). preserve: assert untouched (no-op, valid whether
//     present or not).
//   - Splices apply from the last edit position toward the first, then the
//     candidate is reparsed with document.Parse; a parse failure refuses.
//   - All other bytes — unknown headings, prose, line endings — are identical.
func ApplySectionEdits(src []byte, owned []string, edits []SectionEdit) ([]byte, error) {
	ownedSet := make(map[string]bool, len(owned))
	for _, h := range owned {
		ownedSet[h] = true
	}

	heads := scanH2Headings(src)
	count := make(map[string]int)
	firstIdx := make(map[string]int)
	for i, h := range heads {
		if !ownedSet[h.heading] {
			continue
		}
		count[h.heading]++
		if _, ok := firstIdx[h.heading]; !ok {
			firstIdx[h.heading] = i
		}
	}

	// Duplicate-owned-heading guard: refuse the whole edit set on any repeated
	// owned section — a splice keyed on a non-unique heading is ambiguous.
	for h, c := range count {
		if c > 1 {
			return nil, fmt.Errorf("owned heading %q appears %d times; sections must be unique", h, c)
		}
	}

	// Validate every edit before splicing anything.
	seen := make(map[string]bool, len(edits))
	for _, e := range edits {
		if !ownedSet[e.Heading] {
			return nil, fmt.Errorf("edit names heading %q outside the owned set", e.Heading)
		}
		if seen[e.Heading] {
			return nil, fmt.Errorf("duplicate edit for heading %q", e.Heading)
		}
		seen[e.Heading] = true

		switch e.Intent {
		case SectionPreserve, SectionRemove:
			if e.Markdown != "" {
				return nil, fmt.Errorf("intent %q for %q must carry empty Markdown", e.Intent, e.Heading)
			}
		case SectionReplace:
			// Markdown may be non-empty.
		default:
			return nil, fmt.Errorf("unknown intent %q for %q", e.Intent, e.Heading)
		}

		if e.Intent == SectionRemove && count[e.Heading] != 1 {
			return nil, fmt.Errorf("cannot remove absent section %q", e.Heading)
		}
	}

	le := detectLineEnding(src)

	// Split edits into in-place splices (replace-present, remove) applied against
	// original offsets, and appends (replace-absent) applied after at EOF.
	type splice struct {
		start, end int
		repl       []byte
	}
	var inplace []splice
	var appends []SectionEdit
	for _, e := range edits {
		switch e.Intent {
		case SectionPreserve:
			continue
		case SectionRemove:
			k := firstIdx[e.Heading]
			inplace = append(inplace, splice{start: heads[k].start, end: sectionEnd(heads, k, len(src))})
		case SectionReplace:
			if count[e.Heading] == 1 {
				k := firstIdx[e.Heading]
				end := sectionEnd(heads, k, len(src))
				inplace = append(inplace, splice{
					start: heads[k].start,
					end:   end,
					repl:  renderSection(e.Heading, e.Markdown, le, end == len(src)),
				})
			} else {
				appends = append(appends, e)
			}
		}
	}

	out := append([]byte(nil), src...)
	sort.Slice(inplace, func(i, j int) bool { return inplace[i].start > inplace[j].start })
	for _, s := range inplace {
		next := make([]byte, 0, len(out)-(s.end-s.start)+len(s.repl))
		next = append(next, out[:s.start]...)
		next = append(next, s.repl...)
		next = append(next, out[s.end:]...)
		out = next
	}
	for _, e := range appends {
		out = appendSection(out, e.Heading, e.Markdown, le)
	}

	if _, err := document.Parse(out); err != nil {
		return nil, fmt.Errorf("edited candidate does not reparse: %w", err)
	}
	return out, nil
}

// sectionEnd returns the byte offset one past section k: the start of the next
// top-level heading, or EOF.
func sectionEnd(heads []h2Heading, k, srcLen int) int {
	if k+1 < len(heads) {
		return heads[k+1].start
	}
	return srcLen
}

// renderSection lays out a replaced section: heading, one blank line, the body
// (trailing newlines normalized), then a blank line before the following heading
// unless the section sits at EOF (a single trailing newline there).
func renderSection(heading, markdown, le string, atEOF bool) []byte {
	var b bytes.Buffer
	b.WriteString(heading)
	b.WriteString(le)
	b.WriteString(le)
	b.WriteString(strings.TrimRight(markdown, "\r\n"))
	b.WriteString(le)
	if !atEOF {
		b.WriteString(le)
	}
	return b.Bytes()
}

// appendSection appends a new section at EOF, preceded by exactly one blank line,
// preserving a single trailing newline.
func appendSection(src []byte, heading, markdown, le string) []byte {
	content := bytes.TrimRight(src, "\r\n")
	var b bytes.Buffer
	b.Write(content)
	if len(content) > 0 {
		b.WriteString(le) // terminate the last content line
		b.WriteString(le) // exactly one blank line
	}
	b.WriteString(heading)
	b.WriteString(le)
	b.WriteString(le)
	b.WriteString(strings.TrimRight(markdown, "\r\n"))
	b.WriteString(le)
	return b.Bytes()
}

// detectLineEnding returns the document-level line ending: "\r\n" when the first
// terminated line ends that way, else "\n".
func detectLineEnding(src []byte) string {
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			if i > 0 && src[i-1] == '\r' {
				return "\r\n"
			}
			return "\n"
		}
	}
	return "\n"
}
