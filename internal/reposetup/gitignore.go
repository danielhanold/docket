package reposetup

// gitignore.go — native emitter and marker validation for docket's managed
// .gitignore block. This is the Go port of scripts/lib/docket-gitignore-block.sh's
// emit_docket_gitignore_block / _docket_gi_malformed: that lib documents the block
// as the "single home for ALL docket-owned ignores", and this file transcribes its
// canonical bytes and marker-balance rules verbatim. The two must stay
// byte-identical; cross-language byte-parity is proven by Task 8's integration
// fixture TestIntegrationRepoSetupGitignoreParity, which shells the bash emitter
// and byte-compares its output with GitignoreBlock(). Frozen-copy drift is caught
// by TestGitignoreBlockCanonical here (learning frozen-copy-needs-a-drift-assert).
//
// No I/O: these are pure functions over byte slices. Callers own reading and
// atomically writing the file.

import "bytes"

// GitignoreStart / GitignoreEnd are the exact marker lines owned by the bash lib
// (DOCKET_GI_START / DOCKET_GI_END). The legacy 0051 spelling is recognized only
// for one-time upgrade detection and is never re-emitted.
const (
	GitignoreStart = "# docket:start (managed by docket — do not hand-edit)"
	GitignoreEnd   = "# docket:end"

	legacyGitignoreStart = "# docket:generated:start (managed by sync-agents.sh — do not hand-edit)"
	legacyGitignoreEnd   = "# docket:generated:end"
)

// canonicalBlock is the block body between (and including) the markers, LF
// endings — transcribed from emit_docket_gitignore_block. Order is load-bearing:
// core entries, .docket.local.yml, the per-harness agent globs, the codex toml
// glob, then the cursor dispatch rule.
var canonicalBlockBytes = []byte(GitignoreStart + "\n" +
	".docket/\n" +
	".worktrees/\n" +
	".claude/settings.local.json\n" +
	".docket.local.yml\n" +
	".claude/agents/docket-*.md\n" +
	".codex/agents/docket-*.md\n" +
	".cursor/agents/docket-*.md\n" +
	".opencode/agents/docket-*.md\n" +
	".agents/agents/docket-*.md\n" +
	".kiro/agents/docket-*.md\n" +
	".windsurf/agents/docket-*.md\n" +
	".codex/agents/docket-*.toml\n" +
	".cursor/rules/docket-dispatch.mdc\n" +
	GitignoreEnd + "\n")

// GitignoreBlock returns the canonical managed block bytes (markers inclusive, LF
// line endings), byte-identical to the bash emit_docket_gitignore_block output.
// A fresh copy is returned on each call so callers may mutate it freely.
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
// canonical block — mirroring the bash rewrite exactly. The bash captures rest
// through command substitution (`rest="$(strip …)"`), which strips trailing
// newlines, then emits `printf '%s\n\n' "$rest"` when rest is non-empty followed
// by the block. So trailing blank lines in the outside content are collapsed to
// the single blank-line separator, which is what makes a second call idempotent.
// When rest is empty the block stands alone.
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

// gitignoreMarkersMalformed mirrors _docket_gi_malformed: returns true when the
// start/end markers are NOT a clean, ordered set of non-overlapping pairs
// (dangling start, dangling end, end-before-start, nested start). String-exact,
// line-anchored marker match. An empty/markerless file is well-formed (false).
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
// removed and every byte outside it preserved — the port of _docket_gi_strip_block.
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
// This matches awk's record model, on which the bash primitives rely.
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

// joinLines re-joins whole lines each terminated by LF (awk print semantics),
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
