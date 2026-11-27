package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielhanold/docket/internal/leakscan"
	"github.com/danielhanold/docket/internal/repository"
)

// This file is the read-only `leak check` operation: it scans arbitrary
// outbound files with the publish leak check's own rules and options
// (leakOptions), in any visibility. Each file is scanned on its base name (line
// 0) and on its lines; the directory part of a path is never scanned. A hit is
// reported under applied, not refused. A missing or unreadable file is refused
// before any read of the repository, so a bad path can never read as clean.

// OperationLeakCheck is the operation key the file leak check records in its
// result envelope.
const OperationLeakCheck = "leak.check"

// Refusal reasons (both invalid-input).
const (
	// ReasonLeakCheckMissingFile: no --file given, or an empty one.
	ReasonLeakCheckMissingFile = "missing-file"
	// ReasonLeakCheckUnreadableFile: a given file could not be read.
	ReasonLeakCheckUnreadableFile = "unreadable-file"
)

// LeakCheckRequest is the closed request: the files to scan, in order.
type LeakCheckRequest struct {
	Files []string
}

// LeakCheckHit is one fingerprint in a file. File is the path as given; Line
// is 1-based, or 0 for the file's base name; Text is the matched text only.
type LeakCheckHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
	Rule string `json:"rule"`
}

// LeakCheckContext is the scan's verdict. Hits is never nil, so it always
// serializes as an array.
type LeakCheckContext struct {
	Clean     bool           `json:"clean"`
	MatchWord bool           `json:"match_word"`
	Hits      []LeakCheckHit `json:"hits"`
}

// LeakCheckResult is the protocol-v1 document the operation returns.
type LeakCheckResult struct {
	Envelope
	Context *LeakCheckContext `json:"context,omitempty"`
	Reason  string            `json:"reason,omitempty"`
	Message string            `json:"message,omitempty"`

	// files is how many files were scanned, for HumanText only.
	files int
}

// HumanText renders the one-line summary: clean, the first hits, or the
// refusal.
func (r LeakCheckResult) HumanText() string {
	if r.Result == ResultApplied && r.Context != nil {
		c := r.Context
		if len(c.Hits) == 0 {
			return fmt.Sprintf("%s: clean (%d file(s), match_word %t)", OperationLeakCheck, r.files, c.MatchWord)
		}
		var b strings.Builder
		fmt.Fprintf(&b, "%s: %d hit(s): ", OperationLeakCheck, len(c.Hits))
		for i, h := range c.Hits {
			if i == leakCheckShown {
				break
			}
			if i > 0 {
				b.WriteString("; ")
			}
			fmt.Fprintf(&b, "%s:%d %q (%s)", h.File, h.Line, h.Text, h.Rule)
		}
		if more := len(c.Hits) - leakCheckShown; more > 0 {
			fmt.Fprintf(&b, " … and %d more", more)
		}
		return b.String()
	}
	if r.Reason != "" {
		return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.Reason)
	}
	return fmt.Sprintf("%s: %s", r.Operation, r.Result)
}

func newLeakCheckResult(result Result, reason, message string, c *LeakCheckContext) LeakCheckResult {
	if c != nil && c.Hits == nil {
		c.Hits = []LeakCheckHit{}
	}
	return LeakCheckResult{
		Envelope: NewEnvelope(OperationLeakCheck, result),
		Context:  c,
		Reason:   reason,
		Message:  message,
	}
}

// leakFile is one file read for scanning: its path as given and its bytes.
type leakFile struct {
	path string
	data []byte
}

// LeakCheck scans the given files for docket fingerprints. Every file is read
// before the repository is pinned; then one pin, one corpus read, one snapshot
// supply the scan options.
func LeakCheck(ctx context.Context, deps PlanningDeps, repoDir string, req LeakCheckRequest) LeakCheckResult {
	if len(req.Files) == 0 {
		return newLeakCheckResult(ResultInvalidInput, ReasonLeakCheckMissingFile,
			"at least one --file is required", nil)
	}
	for _, p := range req.Files {
		if p == "" {
			return newLeakCheckResult(ResultInvalidInput, ReasonLeakCheckMissingFile,
				"an empty --file was given", nil)
		}
	}
	files := make([]leakFile, 0, len(req.Files))
	for _, p := range req.Files {
		data, err := os.ReadFile(p)
		if err != nil {
			return newLeakCheckResult(ResultInvalidInput, ReasonLeakCheckUnreadableFile,
				fmt.Sprintf("cannot read %s: %v", p, err), nil)
		}
		files = append(files, leakFile{path: p, data: data})
	}

	pin, err := deps.Reader.PinContext(ctx, repoDir)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newLeakCheckResult(result, reason, err.Error(), nil)
	}
	blobs, err := deps.Reader.ReadCorpus(ctx, pin)
	if err != nil {
		result, reason := classifyStatusError(ctx, err)
		return newLeakCheckResult(result, reason, err.Error(), nil)
	}
	inputs, _ := parseCorpus(blobs)
	build, err := repository.BuildSnapshot(repository.BuildInput{Config: pin.Config.Effective, Documents: inputs})
	if err != nil {
		return newLeakCheckResult(ResultInternalError, ReasonStatusInternalError, err.Error(), nil)
	}

	opts := leakOptions(pin, build.Snapshot)
	hits := scanLeakFiles(files, opts)
	res := newLeakCheckResult(ResultApplied, "", "", &LeakCheckContext{
		Clean:     len(hits) == 0,
		MatchWord: opts.MatchWord,
		Hits:      hits,
	})
	res.files = len(files)
	return res
}

// scanLeakFiles scans each file's base name (as line 0), then its lines, in
// argument order. The directory part of a path is never scanned.
func scanLeakFiles(files []leakFile, opts leakscan.Options) []LeakCheckHit {
	var hits []LeakCheckHit
	for _, f := range files {
		if m, ok := leakscan.Line(filepath.Base(f.path), opts); ok {
			hits = append(hits, LeakCheckHit{File: f.path, Line: 0, Text: m.Text, Rule: string(m.Rule)})
		}
		for _, h := range leakscan.Text(leakscan.SourceFile, string(f.data), opts) {
			hits = append(hits, LeakCheckHit{File: f.path, Line: h.Line, Text: h.Text, Rule: string(h.Rule)})
		}
	}
	return hits
}
