package reposetup

// gitignore.go — emitter and marker validation for docket's managed
// .gitignore block, the single home for every docket-owned ignore. This file
// is the sole owner of the block's bytes (canonicalBlockBytes) and its
// marker-balance rules; GitignoreBlock, ValidGitignoreBlock,
// EnsureGitignoreBlock, GitignoreEntries, and ExplainGitignoreBlock all derive
// from that one definition. Frozen-copy drift is caught by
// TestGitignoreBlockCanonical, which byte-compares GitignoreBlock() with the
// literal in gitignore_test.go (learning frozen-copy-needs-a-drift-assert),
// and TestGitignoreAgentGlobsNameSupportedHarnesses keeps every agent-wrapper
// glob on the accepted harness roster (ADR-0020).
//
// No I/O: these are pure functions over byte slices. Callers own reading and
// atomically writing the file.

import "bytes"

// GitignoreStart / GitignoreEnd are the exact marker lines that delimit the
// managed block. The legacy 0051 spelling is recognized only for one-time
// upgrade detection and is never re-emitted.
const (
	GitignoreStart = "# docket:start (managed by docket — do not hand-edit)"
	GitignoreEnd   = "# docket:end"

	legacyGitignoreStart = "# docket:generated:start (managed by sync-agents.sh — do not hand-edit)"
	legacyGitignoreEnd   = "# docket:generated:end"
)

// canonicalBlockBytes is the block body between (and including) the markers,
// LF endings. Order is load-bearing: core entries, .docket.local.yml, one
// agent-wrapper glob per supported harness (claude, codex, cursor, opencode —
// harness.Order), the codex toml glob, then the cursor dispatch rule. docket
// no longer writes per-repository wrappers; the wrapper globs stay so leftover
// files from older versions remain ignored.
var canonicalBlockBytes = []byte(GitignoreStart + "\n" +
	".docket/\n" +
	".worktrees/\n" +
	".claude/settings.local.json\n" +
	".docket.local.yml\n" +
	".claude/agents/docket-*.md\n" +
	".codex/agents/docket-*.md\n" +
	".cursor/agents/docket-*.md\n" +
	".opencode/agents/docket-*.md\n" +
	".codex/agents/docket-*.toml\n" +
	".cursor/rules/docket-dispatch.mdc\n" +
	GitignoreEnd + "\n")

// GitignoreBlock returns the canonical managed block bytes (markers inclusive,
// LF line endings). A fresh copy is returned on each call so callers may
// mutate it freely.
func GitignoreBlock() []byte {
	return append([]byte(nil), canonicalBlockBytes...)
}

// ValidGitignoreBlock reports whether fileBytes contains the exact canonical
// block (used by check against the integration COMMIT tree). It looks for the
// canonical byte sequence anywhere in the file, but only where the start marker
// begins a line and the block is genuinely present as-is.
func ValidGitignoreBlock(fileBytes []byte) bool {
	block := canonicalBlockBytes
	idx := bytes.Index(fileBytes, block)
	if idx < 0 {
		return false
	}
	// The start marker must begin a line (start of file or right after an LF),
	// so a canonical block embedded mid-line does not count.
	return idx == 0 || fileBytes[idx-1] == '\n'
}

// EnsureGitignoreBlock returns the new full-file bytes with the managed block
// present exactly once: it replaces a stale block, upgrades the legacy 0051
// marker spelling, or appends the block (with a separating blank line) when
// absent. changed is false when the input is already canonical. Malformed
// markers of either generation (dangling / out-of-order / nested) return an
// error with out==nil and the caller's slice untouched.
func EnsureGitignoreBlock(current []byte) (out []byte, changed bool, err error) {
	// (1) Closed-block guard on BOTH marker generations — refuse and touch
	// nothing on a dangling/out-of-order/nested block, so no user bytes are lost.
	if gitignoreMarkersMalformed(current, GitignoreStart, GitignoreEnd) {
		return nil, false, &MalformedGitignoreError{Generation: "docket"}
	}
	if gitignoreMarkersMalformed(current, legacyGitignoreStart, legacyGitignoreEnd) {
		return nil, false, &MalformedGitignoreError{Generation: "legacy"}
	}

	// (2) rest = everything OUTSIDE both the new and the legacy block.
	rest := stripGitignoreBlock(current, GitignoreStart, GitignoreEnd)
	rest = stripGitignoreBlock(rest, legacyGitignoreStart, legacyGitignoreEnd)

	// (3) Idempotence — current block already exact AND no legacy block present.
	legacyPresent := hasLine(current, legacyGitignoreStart)
	if !legacyPresent && bytes.Equal(current, buildFile(rest)) {
		return current, false, nil
	}

	return buildFile(rest), true, nil
}

// buildFile assembles the outside bytes, a blank-line separator, and a single
// canonical block. Trailing newlines in the outside content are trimmed and
// replaced by exactly one blank-line separator, which is what makes a second
// call idempotent. When rest is empty the block stands alone.
func buildFile(rest []byte) []byte {
	trimmed := bytes.TrimRight(rest, "\n")
	if len(trimmed) == 0 {
		return GitignoreBlock()
	}
	var b bytes.Buffer
	b.Write(trimmed)
	b.WriteString("\n\n")
	b.Write(canonicalBlockBytes)
	return b.Bytes()
}

// MalformedGitignoreError reports refusal to rewrite a file whose docket markers
// (of the named generation) are dangling, out of order, or nested.
type MalformedGitignoreError struct {
	Generation string // "docket" or "legacy"
}

func (e *MalformedGitignoreError) Error() string {
	return "malformed docket gitignore markers (" + e.Generation +
		" generation): dangling, out-of-order, or nested start/end — refusing to rewrite"
}

// gitignoreMarkersMalformed returns true when the start/end markers are NOT a
// clean, ordered set of non-overlapping pairs (dangling start, dangling end,
// end-before-start, nested start). String-exact, line-anchored marker match. An
// empty/markerless file is well-formed (false).
func gitignoreMarkersMalformed(fileBytes []byte, start, end string) bool {
	inBlock := false
	bad := false
	for _, line := range splitLines(fileBytes) {
		switch string(line) {
		case start:
			if inBlock {
				bad = true
			}
			inBlock = true
		case end:
			if !inBlock {
				bad = true
			} else {
				inBlock = false
			}
		}
	}
	return bad || inBlock
}

// stripGitignoreBlock returns fileBytes with the [start,end] block (inclusive)
// removed and every byte outside it preserved.
// It assumes markers are well-formed (callers guard first). When no block is
// present the input is returned unchanged.
func stripGitignoreBlock(fileBytes []byte, start, end string) []byte {
	if !hasLine(fileBytes, start) {
		return fileBytes
	}
	var kept [][]byte
	f := false
	for _, line := range splitLines(fileBytes) {
		s := string(line)
		if s == start {
			f = true
		}
		if !f {
			kept = append(kept, line)
		}
		if s == end {
			f = false
		}
	}
	return joinLines(kept)
}

// hasLine reports whether target appears as a whole line in fileBytes.
func hasLine(fileBytes []byte, target string) bool {
	for _, line := range splitLines(fileBytes) {
		if string(line) == target {
			return true
		}
	}
	return false
}

// splitLines splits on LF, dropping a single trailing empty segment produced by
// a final LF (so a file ending in "\n" does not yield a phantom empty line).
func splitLines(fileBytes []byte) [][]byte {
	if len(fileBytes) == 0 {
		return nil
	}
	lines := bytes.Split(fileBytes, []byte("\n"))
	if n := len(lines); n > 0 && len(lines[n-1]) == 0 {
		lines = lines[:n-1]
	}
	return lines
}

// joinLines re-joins whole lines each terminated by LF,
// so an empty result is empty bytes, not a lone LF.
func joinLines(lines [][]byte) []byte {
	if len(lines) == 0 {
		return nil
	}
	var b bytes.Buffer
	for _, line := range lines {
		b.Write(line)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

// --- change 0418: explanatory detail for the committed-ignore health probe ---

// IgnoreDefect classifies why a .gitignore's managed block failed the
// acceptance predicate. IgnoreDefectNone is the zero value and doubles as
// "valid or never probed"; consumers must not read it as a defect.
type IgnoreDefect int

const (
	IgnoreDefectNone             IgnoreDefect = iota
	IgnoreDefectFileAbsent                    // the committed .gitignore file does not exist
	IgnoreDefectBlockAbsent                   // file readable, no current-generation managed block
	IgnoreDefectLegacyOnly                    // only the legacy 0051 markers are present
	IgnoreDefectMalformedMarkers              // dangling / out-of-order / nested markers
	IgnoreDefectMissingEntries                // well-formed block lacking canonical entries
	IgnoreDefectNonCanonical                  // all entries present but not the exact canonical bytes
	IgnoreDefectUnreadable                    // the committed blob could not be read

	// ignoreDefectSentinel is the terminal count marker, never a real
	// defect: tests iterate IgnoreDefectNone up to it so a defect added
	// above is covered automatically (change 0500). Keep it last.
	ignoreDefectSentinel
)

// IgnoreDetail is the small diagnostic payload the committed-ignore probe
// preserves alongside its unchanged three-valued Presence. Only inputs the
// acceptance predicate rejects carry a non-None defect.
type IgnoreDetail struct {
	Defect         IgnoreDefect
	Generation     string   // for MalformedMarkers: "docket" or "legacy"
	MissingEntries []string // for MissingEntries: canonical order
}

// GitignoreEntries returns the canonical managed entries (the block's
// interior lines) in canonical order, derived from the one canonical
// definition rather than a second hand-kept list.
func GitignoreEntries() []string {
	lines := splitLines(canonicalBlockBytes)
	entries := make([]string, 0, len(lines)-2)
	for _, line := range lines[1 : len(lines)-1] {
		entries = append(entries, string(line))
	}
	return entries
}

// ExplainGitignoreBlock explains why fileBytes fails ValidGitignoreBlock.
// The acceptance predicate remains authoritative: any input it accepts
// explains as None, so this helper can never tighten validity. It is pure
// and performs no general Git-ignore semantics analysis.
func ExplainGitignoreBlock(fileBytes []byte) IgnoreDetail {
	if ValidGitignoreBlock(fileBytes) {
		return IgnoreDetail{}
	}
	if gitignoreMarkersMalformed(fileBytes, GitignoreStart, GitignoreEnd) {
		return IgnoreDetail{Defect: IgnoreDefectMalformedMarkers, Generation: "docket"}
	}
	if gitignoreMarkersMalformed(fileBytes, legacyGitignoreStart, legacyGitignoreEnd) {
		return IgnoreDetail{Defect: IgnoreDefectMalformedMarkers, Generation: "legacy"}
	}
	if !hasLine(fileBytes, GitignoreStart) {
		if hasLine(fileBytes, legacyGitignoreStart) {
			return IgnoreDetail{Defect: IgnoreDefectLegacyOnly}
		}
		return IgnoreDetail{Defect: IgnoreDefectBlockAbsent}
	}
	// A well-formed current-generation block exists but is not canonical:
	// membership is judged against the BLOCK's own lines, so an entry
	// elsewhere in the file does not satisfy it.
	member := map[string]bool{}
	in := false
	for _, line := range splitLines(fileBytes) {
		switch string(line) {
		case GitignoreStart:
			in = true
		case GitignoreEnd:
			in = false
		default:
			if in {
				member[string(line)] = true
			}
		}
	}
	var missing []string
	for _, e := range GitignoreEntries() {
		if !member[e] {
			missing = append(missing, e)
		}
	}
	if len(missing) > 0 {
		return IgnoreDetail{Defect: IgnoreDefectMissingEntries, MissingEntries: missing}
	}
	return IgnoreDetail{Defect: IgnoreDefectNonCanonical}
}

// CommittedIgnoreOutcome maps a committed-blob read result to the presence
// fact and its preserved detail — the pure core of the app-layer probe. A
// read error is Unknown+Unreadable, never a fabricated absence (learning
// probe-error-is-not-clean-absence).
func CommittedIgnoreOutcome(blob []byte, found bool, readErr error) (Presence, IgnoreDetail) {
	if readErr != nil {
		return PresenceUnknown, IgnoreDetail{Defect: IgnoreDefectUnreadable}
	}
	if !found {
		return PresenceAbsent, IgnoreDetail{Defect: IgnoreDefectFileAbsent}
	}
	if ValidGitignoreBlock(blob) {
		return PresencePresent, IgnoreDetail{}
	}
	return PresenceAbsent, ExplainGitignoreBlock(blob)
}
