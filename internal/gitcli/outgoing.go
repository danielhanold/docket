package gitcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// readOutgoingOp labels every Failure raised by ReadOutgoing.
const readOutgoingOp Operation = "read-outgoing"

// OutgoingCommit is one commit a push would expose, with its raw message (%B):
// subject, body, and trailers exactly as stored.
type OutgoingCommit struct {
	Commit  ObjectID
	Message string
}

// OutgoingLine is one added line: its path, its 1-based line number in head's
// version of the file, and its text without the leading '+'.
type OutgoingLine struct {
	Path string
	Line int
	Text string
}

// Outgoing is everything a push of head over base exposes that base does not
// already carry: the commits in base..head, plus the paths added and the lines
// added in the merge-base diff base...head.
type Outgoing struct {
	Commits    []OutgoingCommit
	AddedPaths []string
	AddedLines []OutgoingLine
}

// ReadOutgoing reads what pushing head exposes beyond base, from three local
// reads in the primary worktree:
//
//   - `log -z --format=%H%x01%B <base>..<head>` — every commit head carries that
//     base does not, a merge commit included;
//   - `diff --name-status -z --diff-filter=A <base>...<head>` — the added paths;
//   - `diff -U0 <base>...<head>` — the added lines, attributed to their file.
//
// The diffs are three-dot (merge-base) diffs on purpose: they compare head with
// the merge base of base and head, so upstream content merged into the branch —
// or rewritten upstream after that merge — is never reported as the branch's own
// added lines, while a two-tree diff against base would report it. Every diff
// option that user or repository configuration could otherwise change (quoting,
// prefixes, renames, external or textconv drivers, inter-hunk context, binary
// detection, submodule rendering) is pinned on the command line, and replace refs
// and grafts are ignored, so the lines read are the objects a push sends.
//
// Every failure is an error and never an empty or partial result: an invalid id
// is invalid-request, a non-zero exit — including base and head sharing no merge
// base — is command-failed, and output that does not parse is invalid-output. A
// caller must treat an error as "unverified", never as "nothing outgoing".
func (c *Client) ReadOutgoing(ctx context.Context, repo Repository, base, head ObjectID) (Outgoing, error) {
	if err := validateObjectID(base); err != nil {
		return Outgoing{}, newFailure(readOutgoingOp, KindInvalidRequest, "invalid base id", err)
	}
	if err := validateObjectID(head); err != nil {
		return Outgoing{}, newFailure(readOutgoingOp, KindInvalidRequest, "invalid head id", err)
	}
	twoDot := string(base) + ".." + string(head)
	threeDot := string(base) + "..." + string(head)

	logOut, err := c.readOutgoingRun(ctx, repo, "git log",
		"log", "--no-show-signature", "-z", "--format=%H%x01%B", twoDot)
	if err != nil {
		return Outgoing{}, err
	}
	commits, perr := parseOutgoingLog(logOut)
	if perr != nil {
		return Outgoing{}, newFailure(readOutgoingOp, KindInvalidOutput, "malformed outgoing log output", perr)
	}

	namesOut, err := c.readOutgoingRun(ctx, repo, "git diff --name-status",
		"diff", "--name-status", "-z", "--no-renames", "--no-relative", "--diff-filter=A", threeDot)
	if err != nil {
		return Outgoing{}, err
	}
	paths, perr := parseAddedPaths(namesOut)
	if perr != nil {
		return Outgoing{}, newFailure(readOutgoingOp, KindInvalidOutput, "malformed added-paths output", perr)
	}

	patchOut, err := c.readOutgoingRun(ctx, repo, "git diff",
		"-c", "core.quotePath=false", "-c", "diff.suppressBlankEmpty=false",
		"diff", "--no-renames", "--no-color", "--no-ext-diff", "--no-textconv", "--no-relative",
		"--text", "--submodule=short", "--inter-hunk-context=0",
		"--src-prefix=a/", "--dst-prefix=b/", "-U0", threeDot)
	if err != nil {
		return Outgoing{}, err
	}
	lines, perr := parseAddedLines(patchOut)
	if perr != nil {
		return Outgoing{}, newFailure(readOutgoingOp, KindInvalidOutput, "malformed patch output", perr)
	}

	return Outgoing{Commits: commits, AddedPaths: paths, AddedLines: lines}, nil
}

// readOutgoingRun runs one local read for ReadOutgoing with replace refs and
// grafts disabled, returning stdout on a clean exit and a command-failed
// *Failure (carrying a stderr excerpt) on any other.
func (c *Client) readOutgoingRun(ctx context.Context, repo Repository, what string, args ...string) ([]byte, error) {
	res, f := c.run(ctx, runRequest{
		op:   readOutgoingOp,
		dir:  repo.PrimaryWorktree,
		args: append([]string{"--no-replace-objects"}, args...),
		env:  []string{"GIT_GRAFT_FILE="},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(readOutgoingOp, KindCommandFailed, what+" failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return res.stdout, nil
}

// parseOutgoingLog parses the NUL-delimited `git log -z --format=%H%x01%B`
// records: each non-empty record is "<hash>\x01<raw message>". A record without
// the separator, or with a malformed hash, is an error — never a skipped commit.
func parseOutgoingLog(out []byte) ([]OutgoingCommit, error) {
	var commits []OutgoingCommit
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		sep := bytes.IndexByte(rec, trailerSeparator)
		if sep < 0 {
			return nil, errors.New("gitcli: outgoing log record missing its separator")
		}
		hash := ObjectID(rec[:sep])
		if err := validateObjectID(hash); err != nil {
			return nil, err
		}
		commits = append(commits, OutgoingCommit{Commit: hash, Message: string(rec[sep+1:])})
	}
	return commits, nil
}

// parseAddedPaths parses `diff --name-status -z --diff-filter=A` output: NUL
// tokens alternating the status "A" and a verbatim path. An odd token count, a
// status other than A, or an empty path is an error.
func parseAddedPaths(out []byte) ([]string, error) {
	if len(out) == 0 {
		return nil, nil
	}
	toks := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if len(toks)%2 != 0 {
		return nil, errors.New("gitcli: added-paths output has an odd token count")
	}
	paths := make([]string, 0, len(toks)/2)
	for i := 0; i < len(toks); i += 2 {
		if toks[i] != "A" {
			return nil, fmt.Errorf("gitcli: added-paths output has status %q, want A", toks[i])
		}
		if toks[i+1] == "" {
			return nil, errors.New("gitcli: added-paths output has an empty path")
		}
		paths = append(paths, toks[i+1])
	}
	return paths, nil
}

// parseAddedLines reads a unified patch (as ReadOutgoing requests it: -U0, a/ and
// b/ prefixes, no renames, no color) and returns every added line with its file
// and 1-based line number in head's version.
//
// Hunk bodies are consumed by the counts in their "@@" header, never by line
// shape: an added line whose content begins "++ " arrives as "+++ …" inside a
// hunk and is counted as content, never read as a file header. A "\ No newline at
// end of file" marker counts toward neither side, and a context line (' ')
// counts toward both. Parsing is strict so that a misread can never silently
// drop lines: a body shorter or longer than its header's counts, a "+", "-", or
// " " line outside any hunk, a second "+++" header in one file section, a hunk
// before its file's "+++" header, an added line in a deleted file, and a
// malformed header or path are each an error, and an error returns no lines.
func parseAddedLines(patch []byte) ([]OutgoingLine, error) {
	var out []OutgoingLine
	lines := strings.Split(string(patch), "\n")
	var (
		inFile   bool   // a "diff --git" section is open
		haveNew  bool   // its "+++" header has been read
		inHunks  bool   // a hunk of this section has been read
		filePath string // head-side path; "" for a deleted file (+++ /dev/null)
	)
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case strings.HasPrefix(l, "diff --git "):
			inFile, haveNew, inHunks, filePath = true, false, false, ""
		case strings.HasPrefix(l, "+++ "):
			if !inFile || haveNew || inHunks {
				return nil, errors.New("gitcli: unexpected +++ header in patch")
			}
			p, err := patchPath(strings.TrimPrefix(l, "+++ "))
			if err != nil {
				return nil, err
			}
			filePath, haveNew = p, true
		case strings.HasPrefix(l, "--- "):
			if !inFile || haveNew || inHunks {
				return nil, errors.New("gitcli: unexpected --- header in patch")
			}
		case strings.HasPrefix(l, "@@ "):
			if !haveNew {
				return nil, errors.New("gitcli: patch hunk precedes its file header")
			}
			inHunks = true
			oldCount, newStart, newCount, err := parseHunkHeader(l)
			if err != nil {
				return nil, err
			}
			oldSeen, newSeen := 0, 0
			for oldSeen < oldCount || newSeen < newCount {
				i++
				if i >= len(lines) {
					return nil, errors.New("gitcli: patch hunk is truncated")
				}
				body := lines[i]
				switch {
				case strings.HasPrefix(body, `\`): // "\ No newline at end of file"
				case strings.HasPrefix(body, "-"):
					oldSeen++
				case strings.HasPrefix(body, "+"):
					if filePath == "" {
						return nil, errors.New("gitcli: patch adds a line to a deleted file")
					}
					out = append(out, OutgoingLine{Path: filePath, Line: newStart + newSeen, Text: body[1:]})
					newSeen++
				case strings.HasPrefix(body, " "):
					oldSeen++
					newSeen++
				default:
					return nil, errors.New("gitcli: unexpected line inside a patch hunk")
				}
				if oldSeen > oldCount || newSeen > newCount {
					return nil, errors.New("gitcli: patch hunk exceeds its header counts")
				}
			}
		case strings.HasPrefix(l, "+"), strings.HasPrefix(l, "-"), strings.HasPrefix(l, " "):
			// A body line outside any hunk means a hunk ran longer than its header
			// said: refuse rather than drop it.
			return nil, errors.New("gitcli: patch body line outside any hunk")
		}
	}
	return out, nil
}

// parseHunkHeader reads "@@ -a[,b] +c[,d] @@[ section]"; an omitted count is 1.
func parseHunkHeader(l string) (oldCount, newStart, newCount int, err error) {
	f := strings.Fields(l)
	if len(f) < 4 || f[0] != "@@" || f[3] != "@@" || !strings.HasPrefix(f[1], "-") || !strings.HasPrefix(f[2], "+") {
		return 0, 0, 0, errors.New("gitcli: malformed hunk header")
	}
	if _, oldCount, err = hunkRange(f[1][1:]); err != nil {
		return 0, 0, 0, err
	}
	newStart, newCount, err = hunkRange(f[2][1:])
	return oldCount, newStart, newCount, err
}

// hunkRange parses "start[,count]" (count defaults to 1). Each part must be a
// non-empty run of ASCII digits; a sign or any other byte is an error.
func hunkRange(s string) (start, count int, err error) {
	startText, countText, hasCount := strings.Cut(s, ",")
	if start, err = hunkNumber(startText); err != nil {
		return 0, 0, err
	}
	count = 1
	if hasCount {
		if count, err = hunkNumber(countText); err != nil {
			return 0, 0, err
		}
	}
	return start, count, nil
}

// hunkNumber parses one unsigned decimal hunk-header number.
func hunkNumber(s string) (int, error) {
	if s == "" {
		return 0, errors.New("gitcli: empty hunk header number")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("gitcli: malformed hunk header number %q", s)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("gitcli: malformed hunk header number %q: %w", s, err)
	}
	return n, nil
}

// patchPath reads the path of a "+++ " header: /dev/null is "" (a deleted file),
// a C-quoted path is unquoted, the trailing TAB Git appends to a name containing
// a space is dropped, and the b/ prefix is required and removed.
func patchPath(s string) (string, error) {
	s = strings.TrimSuffix(s, "\t")
	if s == "/dev/null" {
		return "", nil
	}
	if strings.HasPrefix(s, `"`) {
		u, err := strconv.Unquote(s)
		if err != nil {
			return "", fmt.Errorf("gitcli: unquoting patch path: %w", err)
		}
		s = u
	}
	p, ok := strings.CutPrefix(s, "b/")
	if !ok || p == "" {
		return "", errors.New("gitcli: patch path lacks the b/ prefix")
	}
	return p, nil
}
