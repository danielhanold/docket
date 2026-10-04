package bashupgrade

// Guide step extraction: the upgrade guide (docs/release/upgrading-from-bash.md) marks
// every block the test drives with a `<!-- upgrade-step: <name> -->` line directly
// above its fence. parseGuide pulls those blocks out in guide order and keeps every
// other fence in Unmarked, so the shape test can refuse an unmarked block that runs
// docket commands.

import (
	"fmt"
	"regexp"
	"strings"
)

const stepMarkerPrefix = "<!-- upgrade-step: "

// guideStep is one marked block: its marker name, the fence body (each line
// newline-terminated, fence lines excluded), and the 1-based line of its marker.
type guideStep struct {
	Name, Body string
	Line       int
}

// guideFence is a fenced block with no marker directly above it; Line is the 1-based
// line of its opening fence.
type guideFence struct {
	Body string
	Line int
}

type guideDoc struct {
	Steps    []guideStep
	Unmarked []guideFence
}

// parseGuide scans src line by line. A line beginning with the marker stem
// (stepMarkerPrefix without its trailing space) must be a well-formed marker, and the
// next non-blank line must open a fence. A fence opens with three or more backticks
// (optional info string) and closes at the first line made of the same backtick run.
func parseGuide(src string) (guideDoc, error) {
	markerRe := regexp.MustCompile(`^<!-- upgrade-step: ([a-z][a-z0-9-]*) -->$`)
	openRe := regexp.MustCompile("^(`{3,})[^`]*$")
	markerStem := strings.TrimSuffix(stepMarkerPrefix, " ")

	lines := strings.Split(src, "\n")
	var doc guideDoc
	seen := map[string]int{}

	// readFence consumes the fence opening at lines[i] and returns its body and the
	// index of the closing line.
	readFence := func(i int, ticks string) (string, int, error) {
		var b strings.Builder
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimRight(lines[j], " \t\r") == ticks {
				return b.String(), j, nil
			}
			b.WriteString(lines[j])
			b.WriteString("\n")
		}
		return "", 0, fmt.Errorf("line %d: unterminated fence %s", i+1, ticks)
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t\r")
		if strings.HasPrefix(line, markerStem) {
			m := markerRe.FindStringSubmatch(line)
			if m == nil {
				return guideDoc{}, fmt.Errorf("line %d: malformed upgrade-step marker %q", i+1, line)
			}
			name := m[1]
			if prev, dup := seen[name]; dup {
				return guideDoc{}, fmt.Errorf("line %d: duplicate upgrade-step %q (first at line %d)", i+1, name, prev)
			}
			seen[name] = i + 1
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			var fm []string
			if j < len(lines) {
				fm = openRe.FindStringSubmatch(strings.TrimRight(lines[j], " \t\r"))
			}
			if fm == nil {
				return guideDoc{}, fmt.Errorf("line %d: upgrade-step %q is not followed by a fenced block", i+1, name)
			}
			body, end, err := readFence(j, fm[1])
			if err != nil {
				return guideDoc{}, err
			}
			doc.Steps = append(doc.Steps, guideStep{Name: name, Body: body, Line: i + 1})
			i = end
			continue
		}
		if fm := openRe.FindStringSubmatch(line); fm != nil {
			body, end, err := readFence(i, fm[1])
			if err != nil {
				return guideDoc{}, err
			}
			doc.Unmarked = append(doc.Unmarked, guideFence{Body: body, Line: i + 1})
			i = end
		}
	}
	return doc, nil
}

// docketCommandLines returns the lines of body whose first shell word, after trimming
// leading whitespace and an optional "$ " prompt, is exactly "docket". Comment lines
// are skipped.
func docketCommandLines(body string) []string {
	var out []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "$ "))
		if f := strings.Fields(line); len(f) > 0 && f[0] == "docket" {
			out = append(out, line)
		}
	}
	return out
}

// substitutePlaceholders replaces each <key> token named in repl and refuses a body
// carrying any <[a-z-]+> token repl does not name. Replacement is single-pass, so a
// value that itself looks like a token is never re-expanded or rejected.
func substitutePlaceholders(body string, repl map[string]string) (string, error) {
	tokenRe := regexp.MustCompile(`<([a-z-]+)>`)
	var unknown []string
	for _, m := range tokenRe.FindAllStringSubmatch(body, -1) {
		if _, ok := repl[m[1]]; !ok {
			unknown = append(unknown, m[0])
		}
	}
	if len(unknown) > 0 {
		return "", fmt.Errorf("unknown placeholders %v", unknown)
	}
	var pairs []string
	for k, v := range repl {
		pairs = append(pairs, "<"+k+">", v)
	}
	return strings.NewReplacer(pairs...).Replace(body), nil
}
