package export

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/exportcheck"
)

// The frozen corpus is real artifacts copied verbatim from the metadata branch,
// plus a few constructed files; testdata/corpus/PROVENANCE.md lists what each one
// exercises. A leftover found here is fixed in the transform, never in the files.
const corpusDir = "testdata/corpus"

// corpusKinds maps a corpus folder to the kind its files export as.
var corpusKinds = map[string]Kind{
	"changes":     KindIssue,
	"specs":       KindSpec,
	"plans":       KindPlan,
	"results":     KindResults,
	"constructed": KindPlan,
}

type corpusFile struct {
	rel  string
	kind Kind
	src  []byte
	in   Input
}

// loadCorpus reads titles.tsv and every corpus file, in a stable order.
func loadCorpus(t *testing.T) []corpusFile {
	t.Helper()
	titles := loadTitles(t)
	var files []corpusFile
	for dir, kind := range corpusKinds {
		paths, err := filepath.Glob(filepath.Join(corpusDir, dir, "*.md"))
		if err != nil {
			t.Fatal(err)
		}
		if len(paths) == 0 {
			t.Fatalf("corpus folder %s holds no files", dir)
		}
		for _, p := range paths {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			in := Input{Kind: kind, Source: src, Titles: titles}
			if kind == KindIssue {
				in.Title = frontmatterTitle(t, p, src)
			}
			files = append(files, corpusFile{rel: filepath.ToSlash(strings.TrimPrefix(p, corpusDir+string(filepath.Separator))), kind: kind, src: src, in: in})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files
}

// loadTitles reads titles.tsv: "<id>\t<raw title>" per line.
func loadTitles(t *testing.T) Titles {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpusDir, "titles.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	titles := Titles{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		id, title, ok := strings.Cut(sc.Text(), "\t")
		num, err := strconv.Atoi(id)
		if !ok || err != nil || title == "" {
			t.Fatalf("titles.tsv line %d is not <id>\\t<title>: %q", n, sc.Text())
		}
		titles[num] = title
	}
	if len(titles) == 0 {
		t.Fatal("titles.tsv is empty")
	}
	return titles
}

// frontmatterTitle reads the title: field of a change record's leading
// frontmatter, unquoting a YAML single- or double-quoted scalar.
func frontmatterTitle(t *testing.T, path string, src []byte) string {
	t.Helper()
	lines := strings.Split(string(src), "\n")
	if len(lines) == 0 || lines[0] != "---" {
		t.Fatalf("%s: change record has no frontmatter", path)
	}
	for _, l := range lines[1:] {
		if l == "---" {
			break
		}
		v, ok := strings.CutPrefix(l, "title:")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		switch {
		case len(v) >= 2 && v[0] == '\'' && v[len(v)-1] == '\'':
			return strings.ReplaceAll(v[1:len(v)-1], "''", "'")
		case len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"':
			return v[1 : len(v)-1]
		}
		return v
	}
	t.Fatalf("%s: change record has no title", path)
	return ""
}

// codeSpans returns the inline code spans of one line, CommonMark style: a run of
// N backticks opens a span and the next run of exactly N closes it; an escaped
// backtick opens nothing. rest is the line with every span removed.
func codeSpans(line string) (spans []string, rest string) {
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] == '\\' && i+1 < len(line) {
			b.WriteString(line[i : i+2])
			i += 2
			continue
		}
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		end := -1
		for j := i + n; j < len(line); {
			if line[j] != '`' {
				j++
				continue
			}
			m := 0
			for j+m < len(line) && line[j+m] == '`' {
				m++
			}
			if m == n {
				end = j + m
				break
			}
			j += m
		}
		if end < 0 {
			b.WriteString(line[i : i+n])
			i += n
			continue
		}
		spans = append(spans, line[i:end])
		b.WriteByte(' ')
		i = end
	}
	return spans, b.String()
}

// mdLine is one line of Markdown with its fence class and its text outside code.
type mdLine struct {
	text    string
	fenced  bool
	kind    document.LineKind
	name    string
	plain   string // text with code spans removed ("" for a fenced line)
	spans   []string
	heading int // ATX heading level outside fences, 0 otherwise
}

var testHeadingRE = regexp.MustCompile(`^ {0,3}(#{1,6})(?:[ \t]|$)`)

func readLines(md []byte) []mdLine {
	s := string(md)
	texts := strings.Split(s, "\n")
	if strings.HasSuffix(s, "\n") {
		texts = texts[:len(texts)-1]
	}
	info := document.ClassifyLines(md)
	out := make([]mdLine, len(texts))
	for i, t := range texts {
		l := mdLine{text: t}
		if i < len(info) {
			l.kind, l.name = info[i].Kind, info[i].Name
		}
		l.fenced = l.kind == document.LineFence
		if !l.fenced {
			l.spans, l.plain = codeSpans(t)
			if m := testHeadingRE.FindStringSubmatch(t); m != nil {
				l.heading = len(m[1])
			}
		}
		out[i] = l
	}
	return out
}

var (
	// testStampRE is a grooming stamp shape in a spec's preamble: a line opening
	// "Change N." / "Change #N,", a "Change:" label, a "Change" followed by a
	// converted title, or a "groomed <date>" sentence anywhere on it (which still
	// shows once step 6 has turned the "Change N" into a title).
	testStampRE = regexp.MustCompile(`(?i)^change #?\d+[.,]|^change:|^change "|\bgroomed \d{4}-\d{2}-\d{2}\b`)
	issueH2     = map[string]bool{"## Why": true, "## What changes": true, "## Out of scope": true, "## Open questions": true}
)

// corpusProblems checks one export: the independent detector finds no leftover,
// and each removal is also asserted directly, so the test does not rest on the
// detector alone. It returns one message per failure.
func corpusProblems(kind Kind, src, out []byte, titles Titles) []string {
	var probs []string
	add := func(format string, a ...any) { probs = append(probs, fmt.Sprintf(format, a...)) }

	report := exportcheck.Check(out, func(id int) bool { _, ok := titles[id]; return ok })
	for _, l := range report.Leftovers {
		add("self-check %s leftover on line %d: %q", l.Rule, l.Line, l.Text)
	}

	lines := readLines(out)
	if len(lines) > 0 && lines[0].text == "---" {
		add("line 1 is a frontmatter fence")
	}
	seenH2 := false
	for i, l := range lines {
		n := i + 1
		if l.fenced {
			continue
		}
		if strings.Contains(l.plain, "<!-- docket:") || strings.Contains(l.plain, "<!-- dckt:") {
			add("line %d: marker outside code: %q", n, l.text)
		}
		if strings.Contains(l.plain, "](") || strings.Contains(l.plain, "<http") {
			add("line %d: link outside code: %q", n, l.text)
		}
		if l.heading == 2 {
			seenH2 = true
			if kind == KindIssue && !issueH2[strings.TrimRight(l.text, " \t")] {
				add("line %d: issue keeps a non-allowlisted section: %q", n, l.text)
			}
		}
		if kind == KindSpec && !seenH2 && (testStampRE.MatchString(l.text) || strings.Contains(l.plain, "**Change:**")) {
			add("line %d: spec stamp before the first H2: %q", n, l.text)
		}
		if strings.TrimSpace(l.text) == "" && i > 0 && !lines[i-1].fenced && strings.TrimSpace(lines[i-1].text) == "" {
			add("line %d: two consecutive blank lines", n)
		}
		if l.heading > 0 {
			for j := i + 1; j < len(lines); j++ {
				if !lines[j].fenced && strings.TrimSpace(lines[j].text) == "" {
					continue
				}
				if !lines[j].fenced && lines[j].heading > 0 && lines[j].heading <= l.heading {
					add("line %d: heading %q has an empty section", n, l.text)
				}
				break
			}
		}
	}

	if missing := missingCodeRegions(kind, src, out); missing != "" {
		add("code region not carried byte-identical, in order: %q", missing)
	}
	return probs
}

// codeRegions returns, in order, every fenced block (its lines joined, each with
// its newline) and every inline code span of the given lines.
func codeRegions(lines []mdLine) []string {
	var regions []string
	var block strings.Builder
	for _, l := range lines {
		if l.fenced {
			block.WriteString(l.text + "\n")
			continue
		}
		if block.Len() > 0 {
			regions = append(regions, block.String())
			block.Reset()
		}
		regions = append(regions, l.spans...)
	}
	if block.Len() > 0 {
		regions = append(regions, block.String())
	}
	return regions
}

// exportScope is the part of src whose content an export keeps: never the
// frontmatter or a marker-delimited generated block, and for an issue only the
// allowlisted sections.
func exportScope(kind Kind, src []byte) []mdLine {
	var scope []mdLine
	inBlock, keepSection := "", kind != KindIssue
	for _, l := range readLines(src) {
		switch {
		case l.kind == document.LineFrontmatter:
			continue
		case l.kind == document.LineMarkerStart:
			inBlock = l.name
			continue
		case l.kind == document.LineMarkerEnd:
			if l.name == inBlock {
				inBlock = ""
			}
			continue
		case inBlock != "":
			continue
		}
		if kind == KindIssue && l.heading == 2 {
			keepSection = issueH2[strings.TrimRight(l.text, " \t")]
		}
		if keepSection {
			scope = append(scope, l)
		}
	}
	return scope
}

// missingCodeRegions returns the first code region of src's export scope that
// does not appear byte-identical, in order, in out; "" when all do.
func missingCodeRegions(kind Kind, src, out []byte) string {
	text := string(out)
	cursor := 0
	for _, r := range codeRegions(exportScope(kind, src)) {
		at := strings.Index(text[cursor:], r)
		if at < 0 {
			return r
		}
		cursor += at + len(r)
	}
	return ""
}

func TestCorpusExportsClean(t *testing.T) {
	for _, f := range loadCorpus(t) {
		t.Run(f.rel, func(t *testing.T) {
			out := Transform(f.in)
			for _, s := range out.Stray {
				t.Errorf("stray marker on source line %d: %q", s.SourceLine, s.Text)
			}
			for _, p := range corpusProblems(f.kind, f.src, out.Markdown, f.in.Titles) {
				t.Error(p)
			}
			if t.Failed() {
				t.Logf("export:\n%s", out.Markdown)
			}
		})
	}
}

// constructedCode is the hand-built file whose code regions and kept reference
// shapes are asserted directly.
const constructedCode = "constructed/code.md"

func findCorpusFile(t *testing.T, rel string) corpusFile {
	t.Helper()
	for _, f := range loadCorpus(t) {
		if f.rel == rel {
			return f
		}
	}
	t.Fatalf("corpus file %s is missing", rel)
	return corpusFile{}
}

func TestCorpusCodeRegionsUntouched(t *testing.T) {
	f := findCorpusFile(t, constructedCode)
	out := string(Transform(f.in).Markdown)
	regions := codeRegions(readLines(f.src))
	var fences, spans int
	for _, r := range regions {
		if strings.HasSuffix(r, "\n") {
			fences++
		} else {
			spans++
		}
	}
	if fences < 2 || spans < 5 {
		t.Fatalf("%s should hold at least 2 fenced blocks and 5 code spans, found %d and %d", constructedCode, fences, spans)
	}
	cursor := 0
	for _, r := range regions {
		at := strings.Index(out[cursor:], r)
		if at < 0 {
			t.Fatalf("code region not carried byte-identical, in order: %q\nexport:\n%s", r, out)
		}
		cursor += at + len(r)
	}
}

func TestCorpusConstructedSurvivors(t *testing.T) {
	f := findCorpusFile(t, constructedCode)
	if _, ok := f.in.Titles[998]; ok {
		t.Fatal("titles.tsv must not claim 998: the fixture uses it as the unknown id")
	}
	if _, ok := f.in.Titles[559]; !ok {
		t.Fatal("titles.tsv must claim 559, so the bare #559 is a known id that still stays")
	}
	out := string(Transform(f.in).Markdown)
	for _, keep := range []string{
		"a bare URL https://go.dev/doc/toolchain,",
		"a bare #559,",
		"PR #433,",
		"ADR-0012,",
		"the unknown 0998.",
		"names change 0998, which",
	} {
		if !strings.Contains(out, keep) {
			t.Errorf("export lost %q:\n%s", keep, out)
		}
	}
}

// TestEveryStepIsLoadBearing disables one transform step at a time and requires
// the corpus checks to notice. A step that stays green means the corpus lacks the
// case it handles: add a file that exercises it, never weaken this guard.
func TestEveryStepIsLoadBearing(t *testing.T) {
	files := loadCorpus(t)
	for _, s := range allSteps {
		var red []string
		for _, f := range files {
			out := transform(f.in, map[step]bool{s: true})
			if len(corpusProblems(f.kind, f.src, out.Markdown, f.in.Titles)) > 0 {
				red = append(red, f.rel)
			}
		}
		if len(red) == 0 {
			t.Errorf("disabling step %d leaves every corpus file clean", s)
			continue
		}
		t.Logf("disabling step %d reddens %d file(s): %s", s, len(red), strings.Join(red, ", "))
	}
}
