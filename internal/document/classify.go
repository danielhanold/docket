package document

// LineKind is the lenient class of one physical source line.
type LineKind int

const (
	LineText            LineKind = iota
	LineFrontmatter              // a line of the leading ---…--- block, both fences included
	LineFence                    // a code-fence delimiter line, or a line inside a fenced block
	LineMarkerStart              // a well-formed start marker outside fences and frontmatter
	LineMarkerEnd                // a well-formed end marker outside fences and frontmatter
	LineMarkerMalformed          // opens like a marker (markerPrefixRE) but fails the grammar
)

// LineInfo describes one physical line.
type LineInfo struct {
	Kind LineKind
	Name string // qualified block name (dckt: prefix kept) for start/end markers; "" otherwise
}

// ClassifyLines returns one LineInfo per physical line, exactly as scanLines
// tiles src. It is the lenient counterpart of Parse's marker scan, for readers
// that must keep working on damaged sources: it never errors, an unclosed
// frontmatter opener is LineText, an unclosed fence runs to EOF, and marker
// balance is not checked. An end marker carrying an annotation is
// LineMarkerMalformed, as scanMarkers rejects it as malformed.
func ClassifyLines(src []byte) []LineInfo {
	lines := scanLines(src)
	if len(lines) == 0 {
		return nil
	}
	out := make([]LineInfo, len(lines))
	first := 0
	if hasFM, _, closing, err := locateFrontmatter(src, lines); err == nil && hasFM {
		for i := range lines {
			out[i] = LineInfo{Kind: LineFrontmatter}
			if lines[i].span == closing {
				first = i + 1
				break
			}
		}
	}

	fence := ""          // the open fence's delimiter run; "" when not inside a fence
	fenceChar := byte(0) // '`' or '~'
	for i := first; i < len(lines); i++ {
		text := src[lines[i].text.Start:lines[i].text.End]
		if run, ok := fenceRun(text); ok {
			switch {
			case fence == "":
				fence, fenceChar = run, run[0]
			case run[0] == fenceChar && len(run) >= len(fence) && isBareFence(text, run):
				fence, fenceChar = "", 0
			}
			out[i] = LineInfo{Kind: LineFence}
			continue
		}
		if fence != "" {
			out[i] = LineInfo{Kind: LineFence}
			continue
		}
		m := markerRE.FindSubmatch(text)
		switch {
		case m == nil && markerPrefixRE.Match(text):
			out[i] = LineInfo{Kind: LineMarkerMalformed}
		case m == nil:
			out[i] = LineInfo{Kind: LineText}
		default:
			name := string(m[2])
			if string(m[1]) == "dckt" {
				name = NeutralMarkerPrefix + name
			}
			switch {
			case string(m[3]) == "start":
				out[i] = LineInfo{Kind: LineMarkerStart, Name: name}
			case len(m[4]) > 0:
				out[i] = LineInfo{Kind: LineMarkerMalformed}
			default:
				out[i] = LineInfo{Kind: LineMarkerEnd, Name: name}
			}
		}
	}
	return out
}
