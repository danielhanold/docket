package export

import (
	"regexp"
	"strings"

	"github.com/danielhanold/docket/internal/document"
)

// Kind is the artifact type being exported; its value is the filename's type word.
type Kind string

const (
	KindIssue   Kind = "issue"
	KindSpec    Kind = "spec"
	KindPlan    Kind = "plan"
	KindResults Kind = "results"
)

// Input is one artifact to export.
type Input struct {
	Kind   Kind
	Source []byte // the artifact bytes from the metadata checkout
	Title  string // the change's raw title (the issue H1)
	Titles Titles
}

// Stray is an unpaired or malformed marker line that was removed. Its presence
// means the source is damaged.
type Stray struct {
	SourceLine int // 1-based
	Text       string
}

// Output is the exported Markdown plus the stray marker lines removed from it.
type Output struct {
	Markdown []byte
	Stray    []Stray
}

// step names one transform step, so a test can disable it and watch the
// independent check notice.
type step int

const (
	stepFrontmatter step = iota + 1
	stepBlocks
	stepIssueSections
	stepStamps
	stepLinks
	stepRefs
	stepCleanup
)

var allSteps = []step{stepFrontmatter, stepBlocks, stepIssueSections, stepStamps, stepLinks, stepRefs, stepCleanup}

// issueSections is the allowlist of H2 sections a change record keeps.
var issueSections = map[string]bool{
	"Why":            true,
	"What changes":   true,
	"Out of scope":   true,
	"Open questions": true,
}

// line is one physical line moving through the pipeline.
type line struct {
	text string
	kind document.LineKind
	name string
	src  int // 1-based source line
}

func (l line) fenced() bool { return l.kind == document.LineFence }

// Transform turns an artifact into clean, docket-free Markdown. It never fails:
// a damaged source loses only its damaged marker lines, which Output.Stray lists.
func Transform(in Input) Output { return transform(in, nil) }

// transform is Transform with the steps in skip disabled.
func transform(in Input, skip map[step]bool) Output {
	src := strings.ReplaceAll(string(in.Source), "\r\n", "\n")
	lines := splitLines(src)
	var out Output

	if !skip[stepFrontmatter] {
		lines = filter(lines, func(l line) bool { return l.kind != document.LineFrontmatter })
	}
	if !skip[stepBlocks] {
		lines, out.Stray = removeBlocks(lines)
	}
	if in.Kind == KindIssue && !skip[stepIssueSections] {
		lines = keepIssueSections(lines, cleanTitle(in.Title, in.Titles))
	}
	if in.Kind == KindSpec && !skip[stepStamps] {
		lines = removeStamps(lines)
	}
	lines = inlineSteps(lines, in.Titles, !skip[stepLinks], !skip[stepRefs])
	if !skip[stepCleanup] {
		lines = cleanup(lines)
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
		b.WriteByte('\n')
	}
	out.Markdown = []byte(b.String())
	return out
}

// splitLines tiles src into lines the way document.ClassifyLines does and pairs
// each with its class.
func splitLines(src string) []line {
	info := document.ClassifyLines([]byte(src))
	texts := strings.Split(src, "\n")
	if strings.HasSuffix(src, "\n") || src == "" {
		texts = texts[:len(texts)-1]
	}
	lines := make([]line, len(texts))
	for i, t := range texts {
		lines[i] = line{text: t, src: i + 1}
		if i < len(info) {
			lines[i].kind, lines[i].name = info[i].Kind, info[i].Name
		}
	}
	return lines
}

func filter(lines []line, keep func(line) bool) []line {
	out := lines[:0:0]
	for _, l := range lines {
		if keep(l) {
			out = append(out, l)
		}
	}
	return out
}

// removeBlocks is step 2. Markers pair leniently in source order: a start opens a
// block that the next same-name end closes, and the whole block goes. A start that
// meets another start or EOF first, an end with no open start, and a malformed
// marker are each removed alone and reported, so damage never removes content.
func removeBlocks(lines []line) ([]line, []Stray) {
	var kept []line
	var stray []Stray
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch l.kind {
		case document.LineMarkerStart:
			if end := blockEnd(lines, i); end >= 0 {
				i = end
				continue
			}
			stray = append(stray, Stray{SourceLine: l.src, Text: l.text})
		case document.LineMarkerEnd, document.LineMarkerMalformed:
			stray = append(stray, Stray{SourceLine: l.src, Text: l.text})
		default:
			kept = append(kept, l)
		}
	}
	return kept, stray
}

// blockEnd returns the index of the end marker closing the start at lines[i], or
// -1 when another start or EOF comes first.
func blockEnd(lines []line, i int) int {
	for j := i + 1; j < len(lines); j++ {
		switch {
		case lines[j].kind == document.LineMarkerStart:
			return -1
		case lines[j].kind == document.LineMarkerEnd && lines[j].name == lines[i].name:
			return j
		}
	}
	return -1
}

func isH2(l line) bool { return !l.fenced() && strings.HasPrefix(l.text, "## ") }

// keepIssueSections is step 3: only the allowlisted H2 sections stay, in source
// order, under an H1 of the cleaned change title.
func keepIssueSections(lines []line, title string) []line {
	out := []line{{text: "# " + title}, {text: ""}}
	keep := false
	for _, l := range lines {
		if isH2(l) {
			keep = issueSections[strings.TrimSpace(l.text[len("## "):])]
		}
		if keep {
			out = append(out, l)
		}
	}
	return out
}

var (
	// stampSentenceRE is a grooming stamp sentence at the start of a line.
	stampSentenceRE = regexp.MustCompile(`^Change #?\d+[,.] [Gg]roomed \d{4}-\d{2}-\d{2}(?: \([^)]*\))?\.`)
	changeOnlyRE    = regexp.MustCompile(`^Change #?\d+\.$`)
	boldChangeRE    = regexp.MustCompile(`^\*\*Change:\*\* #?\d+$`)
	boldPairRE      = regexp.MustCompile(`^\*\*[^*]+:\*\* \S.*$`)
)

// removeStamps is step 4: before the first H2 of a spec, every shape of the
// grooming change stamp goes, and the rest of a stamp sentence's line stays.
func removeStamps(lines []line) []line {
	var out []line
	for i, l := range lines {
		if isH2(l) {
			return append(out, lines[i:]...)
		}
		if l.fenced() {
			out = append(out, l)
			continue
		}
		if loc := stampSentenceRE.FindStringIndex(l.text); loc != nil {
			l.text = strings.TrimLeft(l.text[loc[1]:], " \t")
		}
		if changeOnlyRE.MatchString(l.text) || boldChangeRE.MatchString(l.text) || isBoldChangeRun(l.text) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// isBoldChangeRun reports a "·"-separated run of **Key:** value pairs that
// includes **Change:**.
func isBoldChangeRun(text string) bool {
	parts := strings.Split(text, "·")
	if len(parts) < 2 {
		return false
	}
	hasChange := false
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if !boldPairRE.MatchString(p) {
			return false
		}
		if strings.HasPrefix(p, "**Change:**") {
			hasChange = true
		}
	}
	return hasChange
}

// inlineSteps runs steps 5 and 6 on every line outside fences: link-reference
// definitions go, then each line is code-masked, its links are stripped, and its
// change references are converted, before its code spans are restored.
func inlineSteps(lines []line, titles Titles, links, refs bool) []line {
	if !links && !refs {
		return lines
	}
	var labels map[string]bool
	if links {
		texts := make([]string, len(lines))
		info := make([]document.LineInfo, len(lines))
		for i, l := range lines {
			texts[i], info[i] = l.text, document.LineInfo{Kind: l.kind, Name: l.name}
		}
		labels = linkLabels(texts, info)
		lines = filter(lines, func(l line) bool { return l.fenced() || !isLinkDefinition(l.text) })
	}
	for i, l := range lines {
		if l.fenced() {
			continue
		}
		masked, restore := maskCode(l.text)
		if links {
			masked = stripLinks(masked, labels)
		}
		if refs {
			masked = convertRefs(masked, titles)
		}
		lines[i].text = restore(masked)
	}
	return lines
}

// headingRE is an ATX heading; its first group is the level's run of "#".
var headingRE = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]|$)`)

func headingLevel(l line) int {
	if l.fenced() {
		return 0
	}
	if m := headingRE.FindStringSubmatch(l.text); m != nil {
		return len(m[1])
	}
	return 0
}

func blank(l line) bool { return !l.fenced() && strings.TrimSpace(l.text) == "" }

// cleanup is step 7: headings left with empty sections go (repeated until
// stable, so an emptied parent follows its emptied child), blank-line runs
// collapse to one, and no blank line leads or trails. Fenced lines are content
// and are never collapsed.
func cleanup(lines []line) []line {
	for {
		drop := map[int]bool{}
		for i, l := range lines {
			level := headingLevel(l)
			if level == 0 {
				continue
			}
			empty := true
			for j := i + 1; j < len(lines); j++ {
				if next := headingLevel(lines[j]); next > 0 && next <= level {
					break
				}
				if !blank(lines[j]) {
					empty = false
					break
				}
			}
			if empty {
				drop[i] = true
			}
		}
		if len(drop) == 0 {
			break
		}
		var kept []line
		for i, l := range lines {
			if !drop[i] {
				kept = append(kept, l)
			}
		}
		lines = kept
	}
	var out []line
	for _, l := range lines {
		if blank(l) && (len(out) == 0 || blank(out[len(out)-1])) {
			continue
		}
		out = append(out, l)
	}
	for len(out) > 0 && blank(out[len(out)-1]) {
		out = out[:len(out)-1]
	}
	return out
}
