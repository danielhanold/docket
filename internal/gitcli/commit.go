package gitcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// commitPathsOp labels every Failure raised by the explicit-path commit surface.
const commitPathsOp Operation = "commit-paths"

// Trailer is one engine-owned commit trailer. Key is a token (letters, digits,
// and hyphens, beginning with a letter or digit); Value is a single line with no
// control characters. Both are validated before a message is composed so a
// trailer can neither smuggle a newline into the block nor carry a ':' into a
// key.
type Trailer struct {
	Key   string
	Value string
}

// CommitRequest describes an explicit-path commit in a detached transaction
// worktree. Paths is the exact set to stage — additions, replacements, AND
// deletions — and nothing outside it is committed. Trailers are appended as the
// message's final trailer block. HooksPath is an engine-owned (empty) directory
// passed as -c core.hooksPath so repository hooks never run. When sets both the
// author and committer dates from the engine clock.
type CommitRequest struct {
	Dir       string
	Paths     []RepoPath
	Subject   string
	Trailers  []Trailer
	HooksPath string
	When      time.Time
}

// CommitPaths stages exactly req.Paths and commits them on the worktree's current
// (detached) HEAD, returning the new commit id. Staging reads the pathspecs from
// stdin NUL-separated (`git add --pathspec-from-file=- --pathspec-file-nul`) so a
// hostile path byte cannot corrupt the vector and a named deleted path is staged
// as a removal; the commit runs with `-c core.hooksPath=<empty> -c
// commit.gpgsign=false commit --no-verify -F -`, so no repository hook fires and
// no signing is attempted regardless of repo config. The message is composed from
// the subject and a final trailer block and delivered on stdin. The repository's
// own configured identity is used; a missing user.name/user.email surfaces as a
// typed *Failure (command-failed) from git's own non-zero exit, never a
// hard-coded person.
func (c *Client) CommitPaths(ctx context.Context, repo Repository, req CommitRequest) (ObjectID, error) {
	if req.Dir == "" {
		return "", newFailure(commitPathsOp, KindInvalidRequest, "worktree dir is empty", nil)
	}
	if len(req.Paths) == 0 {
		return "", newFailure(commitPathsOp, KindInvalidRequest, "no paths to commit", nil)
	}
	if req.HooksPath == "" {
		return "", newFailure(commitPathsOp, KindInvalidRequest, "hooks path is empty", nil)
	}
	if err := validateCommitSubject(req.Subject); err != nil {
		return "", newFailure(commitPathsOp, KindInvalidRequest, "invalid commit subject", err)
	}
	for _, p := range req.Paths {
		if err := validateRepoPath(p, false); err != nil {
			return "", newFailure(commitPathsOp, KindInvalidRequest, "invalid commit path", err)
		}
	}
	for _, tr := range req.Trailers {
		if err := validateTrailer(tr); err != nil {
			return "", newFailure(commitPathsOp, KindInvalidRequest, "invalid trailer", err)
		}
	}

	// Stage exactly the declared paths, NUL-separated on stdin, so hostile bytes
	// survive and no path is ever taken as an option.
	var stage bytes.Buffer
	for _, p := range req.Paths {
		stage.WriteString(string(p))
		stage.WriteByte(0)
	}
	res, f := c.run(ctx, runRequest{
		op:    commitPathsOp,
		dir:   req.Dir,
		args:  []string{"add", "--pathspec-from-file=-", "--pathspec-file-nul"},
		stdin: stage.Bytes(),
	})
	if f != nil {
		return "", f
	}
	if res.exitCode != 0 {
		return "", newFailure(commitPathsOp, KindCommandFailed, "git add failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}

	// Compose the subject plus the engine trailer block; deliver on stdin so no
	// temp file is needed at this layer. Author and committer dates come from the
	// engine clock via the per-command environment.
	msg := composeCommitMessage(req.Subject, req.Trailers)
	date := fmt.Sprintf("@%d +0000", req.When.UTC().Unix())
	cres, f := c.run(ctx, runRequest{
		op:  commitPathsOp,
		dir: req.Dir,
		args: []string{
			"-c", "core.hooksPath=" + req.HooksPath,
			"-c", "commit.gpgsign=false",
			"commit", "--no-verify", "-F", "-",
		},
		stdin: []byte(msg),
		env: []string{
			"GIT_AUTHOR_DATE=" + date,
			"GIT_COMMITTER_DATE=" + date,
		},
	})
	if f != nil {
		return "", f
	}
	if cres.exitCode != 0 {
		return "", newFailure(commitPathsOp, KindCommandFailed, "git commit failed: "+stderrExcerpt(cres.stderr), nil).withExitCode(cres.exitCode)
	}

	// Read the resulting commit id off the detached HEAD.
	hres, f := c.run(ctx, runRequest{
		op:   commitPathsOp,
		dir:  req.Dir,
		args: []string{"rev-parse", "--verify", "HEAD"},
	})
	if f != nil {
		return "", f
	}
	if hres.exitCode != 0 {
		return "", newFailure(commitPathsOp, KindCommandFailed, "rev-parse HEAD failed: "+stderrExcerpt(hres.stderr), nil).withExitCode(hres.exitCode)
	}
	lines := stdoutLines(hres.stdout)
	if len(lines) != 1 {
		return "", newFailure(commitPathsOp, KindInvalidOutput, "unexpected rev-parse output", nil)
	}
	id := ObjectID(lines[0])
	if err := validateObjectID(id); err != nil {
		return "", newFailure(commitPathsOp, KindInvalidOutput, "rev-parse produced a malformed object id", err)
	}
	return id, nil
}

// composeCommitMessage renders the subject followed, when trailers are present,
// by a blank line and the trailer block (one "Key: Value" per line). The blank
// line makes the trailer lines the message's final paragraph, which is what
// Git's trailer interpretation requires to treat them as a trailer block.
func composeCommitMessage(subject string, trailers []Trailer) string {
	var b strings.Builder
	b.WriteString(subject)
	b.WriteByte('\n')
	if len(trailers) > 0 {
		b.WriteByte('\n')
		for _, tr := range trailers {
			b.WriteString(tr.Key)
			b.WriteString(": ")
			b.WriteString(tr.Value)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// validateCommitSubject requires a non-empty subject with no control characters
// (so it stays a single line and cannot inject trailer-looking content).
func validateCommitSubject(s string) error {
	if s == "" {
		return errors.New("gitcli: empty commit subject")
	}
	if hasControlByte(s) {
		return errors.New("gitcli: commit subject contains a control character")
	}
	return nil
}

// validateTrailer checks a trailer's key token grammar and rejects control
// characters (including newlines) in the value.
func validateTrailer(t Trailer) error {
	if err := validateTrailerKey(t.Key); err != nil {
		return err
	}
	if hasControlByte(t.Value) {
		return errors.New("gitcli: trailer value contains a control character")
	}
	return nil
}

// validateTrailerKey requires a non-empty token beginning with a letter or digit
// and otherwise composed of letters, digits, and hyphens — which also forbids the
// ':' that would let a key masquerade as a value boundary.
func validateTrailerKey(k string) error {
	if k == "" {
		return errors.New("gitcli: empty trailer key")
	}
	for i := 0; i < len(k); i++ {
		ch := k[i]
		alnum := (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
		if i == 0 {
			if !alnum {
				return errors.New("gitcli: trailer key must begin with a letter or digit")
			}
			continue
		}
		if !alnum && ch != '-' {
			return errors.New("gitcli: trailer key has an invalid character")
		}
	}
	return nil
}

// hasControlByte reports whether s contains any ASCII control byte (< 0x20) or
// DEL (0x7f). It scans bytes, so multi-byte UTF-8 runes (all bytes >= 0x80) are
// left untouched.
func hasControlByte(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] == 0x7f {
			return true
		}
	}
	return false
}
