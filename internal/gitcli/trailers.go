package gitcli

import (
	"bytes"
	"context"
	"errors"
	"strings"
)

// scanTrailersOp labels every Failure raised by the trailer-scan surface.
const scanTrailersOp Operation = "scan-trailers"

// CommitTrailers is one commit's full trailer set, returned when the commit
// carries at least one of the scanned keys.
type CommitTrailers struct {
	Commit   ObjectID
	Trailers []Trailer
}

// ScanCommitTrailers walks every commit reachable from `from` and returns, for
// each commit carrying at least one trailer whose key is in keys, that commit's
// full trailer set. Trailers are parsed with Git's own trailer interpretation via
// `git log -z --format=%H%x01%(trailers:only,unfold) <from>`, so a
// trailer-looking line in a subject or body — anywhere outside the final trailer
// block — never matches. The full ancestry is scanned; there is no depth window.
func (c *Client) ScanCommitTrailers(ctx context.Context, repo Repository, from ObjectID, keys []string) ([]CommitTrailers, error) {
	if err := validateObjectID(from); err != nil {
		return nil, newFailure(scanTrailersOp, KindInvalidRequest, "invalid from id", err)
	}
	res, f := c.run(ctx, runRequest{
		op:   scanTrailersOp,
		dir:  repo.PrimaryWorktree,
		args: []string{"log", "-z", "--format=%H%x01%(trailers:only,unfold)", string(from)},
	})
	if f != nil {
		return nil, f
	}
	if res.exitCode != 0 {
		return nil, newFailure(scanTrailersOp, KindCommandFailed, "git log failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	result, err := parseTrailerLog(res.stdout, keys)
	if err != nil {
		return nil, newFailure(scanTrailersOp, KindInvalidOutput, "malformed trailer log output", err)
	}
	return result, nil
}

// trailerSeparator is the SOH byte (%x01) placed between the commit hash and its
// trailer block in the log format, chosen because it never appears in a hash or
// an unfolded trailer line.
const trailerSeparator = 0x01

// parseTrailerLog parses the NUL-delimited `git log` records produced by
// ScanCommitTrailers. Each non-empty record is "<hash>\x01<trailer-block>"; a
// commit is included only when its trailer block carries at least one key in
// keys, and then its full trailer set is returned.
func parseTrailerLog(out []byte, keys []string) ([]CommitTrailers, error) {
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}
	var result []CommitTrailers
	for _, rec := range bytes.Split(out, []byte{0}) {
		if len(rec) == 0 {
			continue
		}
		sep := bytes.IndexByte(rec, trailerSeparator)
		if sep < 0 {
			return nil, errors.New("gitcli: trailer log record missing its separator")
		}
		hash := ObjectID(rec[:sep])
		if err := validateObjectID(hash); err != nil {
			return nil, err
		}
		trailers := parseTrailerBlock(rec[sep+1:])
		matched := false
		for _, tr := range trailers {
			if keySet[tr.Key] {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, CommitTrailers{Commit: hash, Trailers: trailers})
		}
	}
	return result, nil
}

// parseTrailerBlock parses the unfolded trailer block emitted by
// %(trailers:only,unfold): one "Key: Value" per line. Blank lines are skipped; a
// bare "Key:" (empty value) is tolerated. Because the block comes from Git's own
// trailer interpretation, every line here is a genuine trailer.
func parseTrailerBlock(b []byte) []Trailer {
	var trailers []Trailer
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ": "); idx >= 0 {
			trailers = append(trailers, Trailer{Key: line[:idx], Value: line[idx+2:]})
			continue
		}
		if strings.HasSuffix(line, ":") {
			trailers = append(trailers, Trailer{Key: strings.TrimSuffix(line, ":"), Value: ""})
		}
	}
	return trailers
}
