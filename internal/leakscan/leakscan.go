// Package leakscan holds the fingerprint rules for text a private repository
// publishes: commit messages, added paths and lines, and PR text. It is pure
// and stdlib-only. Each line yields at most one hit; the rules are tried in a
// fixed order (marker, trailer, path, alias, change-ref, word).
package leakscan

import (
	"strconv"
	"strings"
)

// Rule names the fingerprint rule a match came from.
type Rule string

const (
	RuleMarker    Rule = "marker"     // docket:/dckt: followed by a letter
	RuleTrailer   Rule = "trailer"    // Docket-<Key>: at line start
	RulePath      Rule = "path"       // .docket or .git/dckt
	RuleAlias     Rule = "alias"      // the word dckt
	RuleChangeRef Rule = "change-ref" // a reference to a backlog change id
	RuleWord      Rule = "word"       // the bare word docket (Options.MatchWord)
)

// Source names where a scanned piece of text came from.
type Source string

const (
	SourceCommitMessage Source = "commit-message"
	SourceAddedLine     Source = "added-line"
	SourceAddedPath     Source = "added-path"
	SourcePRTitle       Source = "pr-title"
	SourcePRBody        Source = "pr-body"
)

// Options configures a scan. MatchWord enables the bare-word docket rule;
// ChangeIDs is the backlog's change ids, and a nil map matches no change
// reference.
type Options struct {
	MatchWord bool
	ChangeIDs map[int]bool
}

// Match is one line's fingerprint: the matched text only and its rule.
type Match struct {
	Text string
	Rule Rule
}

// Hit is an attributed match. Commit is set for a commit-message hit, and for an
// added line or path that a known commit added; File is set for an added line or
// path; Line is 1-based (0 for a path); Text is the matched text only, never the
// whole line.
type Hit struct {
	Source Source
	Commit string
	File   string
	Line   int
	Text   string
	Rule   Rule
}

// Commit is one outgoing commit's id and full message.
type Commit struct{ ID, Message string }

// AddedLine is one line a push adds, with its file and 1-based line number, and
// the commit that added it when known.
type AddedLine struct {
	Path   string
	Line   int
	Text   string
	Commit string
}

// AddedPath is one path a push adds, and the commit that added it when known.
type AddedPath struct{ Path, Commit string }

// PRText is a pull request's title and body.
type PRText struct{ Title, Body string }

// Input is everything one publish exposes. PR is nil when no PR text is written.
type Input struct {
	Commits    []Commit
	AddedPaths []AddedPath
	AddedLines []AddedLine
	PR         *PRText
}

// Scan checks every input in order: each commit message, each added path, each
// added line, then the PR title and body when PR is non-nil.
func Scan(in Input, opts Options) []Hit {
	var hits []Hit
	for _, c := range in.Commits {
		for _, h := range Text(SourceCommitMessage, c.Message, opts) {
			h.Commit = c.ID
			hits = append(hits, h)
		}
	}
	for _, p := range in.AddedPaths {
		if m, ok := Line(p.Path, opts); ok {
			hits = append(hits, Hit{Source: SourceAddedPath, Commit: p.Commit, File: p.Path, Text: m.Text, Rule: m.Rule})
		}
	}
	for _, l := range in.AddedLines {
		if m, ok := Line(l.Text, opts); ok {
			hits = append(hits, Hit{Source: SourceAddedLine, Commit: l.Commit, File: l.Path, Line: l.Line, Text: m.Text, Rule: m.Rule})
		}
	}
	if in.PR != nil {
		hits = append(hits, Text(SourcePRTitle, in.PR.Title, opts)...)
		hits = append(hits, Text(SourcePRBody, in.PR.Body, opts)...)
	}
	return hits
}

// Text splits text on "\n", trims a trailing "\r", and checks each line,
// numbering lines from 1.
func Text(src Source, text string, opts Options) []Hit {
	var hits []Hit
	for n, line := range strings.Split(text, "\n") {
		if m, ok := Line(strings.TrimSuffix(line, "\r"), opts); ok {
			hits = append(hits, Hit{Source: src, Line: n + 1, Text: m.Text, Rule: m.Rule})
		}
	}
	return hits
}

// Line checks one line, trying marker, trailer, path, alias, change-ref, and
// word, in that order, and returns the first match.
func Line(line string, opts Options) (Match, bool) {
	low := asciiLower(line)
	for _, w := range []string{"docket", "dckt"} {
		for _, i := range wordAt(line, low, w) {
			j := i + len(w)
			if j+1 < len(line) && line[j] == ':' && isLetter(line[j+1]) {
				return Match{line[i : j+1], RuleMarker}, true
			}
		}
	}
	if t, ok := trailerKey(line, low); ok {
		return Match{t, RuleTrailer}, true
	}
	for _, frag := range []string{".docket", ".git/dckt"} {
		if i := fragmentAt(low, frag); i >= 0 {
			return Match{line[i : i+len(frag)], RulePath}, true
		}
	}
	if is := wordAt(line, low, "dckt"); len(is) > 0 {
		return Match{line[is[0] : is[0]+4], RuleAlias}, true
	}
	if t, ok := changeRef(line, low, opts.ChangeIDs); ok {
		return Match{t, RuleChangeRef}, true
	}
	if opts.MatchWord {
		if is := wordAt(line, low, "docket"); len(is) > 0 {
			return Match{line[is[0] : is[0]+6], RuleWord}, true
		}
	}
	return Match{}, false
}

// wordAt returns the offsets of w in low not glued to a letter on either side,
// except across a camelCase boundary ("docketBranch", "myDocket" match;
// "dockets", "pocketdocket" do not). asciiLower keeps byte offsets.
func wordAt(line, low, w string) []int {
	var out []int
	for from := 0; ; {
		k := strings.Index(low[from:], w)
		if k < 0 {
			return out
		}
		i, j := from+k, from+k+len(w)
		prevOK := i == 0 || !isLetter(line[i-1]) || (isUpper(line[i]) && isLower(line[i-1]))
		nextOK := j == len(line) || !isLetter(line[j]) || (isUpper(line[j]) && isLower(line[j-1]))
		if prevOK && nextOK {
			out = append(out, i)
		}
		from = i + 1
	}
}

// fragmentAt returns the first offset of frag not followed by a letter, or -1.
func fragmentAt(low, frag string) int {
	for from := 0; ; {
		k := strings.Index(low[from:], frag)
		if k < 0 {
			return -1
		}
		i := from + k
		if j := i + len(frag); j == len(low) || !isLetter(low[j]) {
			return i
		}
		from = i + 1
	}
}

// trailerKey matches a "Docket-<Key>:" trailer key at line start (after
// leading blanks), case-insensitively.
func trailerKey(line, low string) (string, bool) {
	start := len(line) - len(strings.TrimLeft(line, " \t"))
	rest, lrest := line[start:], low[start:]
	const p = "docket-"
	if !strings.HasPrefix(lrest, p) {
		return "", false
	}
	k := len(p)
	for k < len(rest) && (isLetter(rest[k]) || isDigit(rest[k]) || rest[k] == '-') {
		k++
	}
	if k == len(p) || k >= len(rest) || rest[k] != ':' {
		return "", false
	}
	return rest[:k+1], true
}

// changeRef matches "change[s] #N" or "change[s] NNNN+", "#0NNN", and "(0NNN)";
// the id must be in ids. "(2026)" and "#612" never match (not zero-padded).
func changeRef(line, low string, ids map[int]bool) (string, bool) {
	if len(ids) == 0 {
		return "", false
	}
	for _, w := range []string{"change", "changes"} {
		for _, i := range wordAt(line, low, w) {
			j := i + len(w)
			k := j
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k == j {
				continue
			}
			hash := k < len(line) && line[k] == '#'
			if hash {
				k++
			}
			if d, end := digitsAt(line, k); d != "" && (hash || len(d) >= 4) && inBacklog(d, ids) {
				return line[i:end], true
			}
		}
	}
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '#':
			if i > 0 && (isLetter(line[i-1]) || isDigit(line[i-1])) {
				continue
			}
			if d, end := digitsAt(line, i+1); padded(d) && inBacklog(d, ids) {
				return line[i:end], true
			}
		case '(':
			if d, end := digitsAt(line, i+1); padded(d) && end < len(line) && line[end] == ')' && inBacklog(d, ids) {
				return line[i : end+1], true
			}
		}
	}
	return "", false
}

// digitsAt returns 1–9 digits at i not followed by a letter, and their end;
// "" otherwise.
func digitsAt(line string, i int) (string, int) {
	j := i
	for j < len(line) && isDigit(line[j]) {
		j++
	}
	if j == i || j-i > 9 || (j < len(line) && isLetter(line[j])) {
		return "", i
	}
	return line[i:j], j
}

func padded(d string) bool { return len(d) >= 4 && d[0] == '0' }

func inBacklog(d string, ids map[int]bool) bool {
	n, err := strconv.Atoi(d)
	return err == nil && ids[n]
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if isUpper(c) {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func isLetter(c byte) bool { return isLower(c) || isUpper(c) }
func isLower(c byte) bool  { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool  { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
