package reposetup

// exclude.go — emitter and marker validation for the neutral managed block a
// private repository keeps in .git/info/exclude. A private repository writes
// no committed root file, so its one ignore (the feature-worktree root) lives
// in the clone-local exclude file under neutral `# dckt:` markers. It reuses
// gitignore.go's marker helpers (gitignoreMarkersMalformed,
// stripGitignoreBlock) as they are; .gitignore behavior is untouched.
//
// No I/O: pure functions over byte slices. Callers own reading and atomically
// writing the file.

import "bytes"

// ExcludeStart / ExcludeEnd are the exact marker lines that delimit the
// managed exclude block. They are deliberately neutral: nothing docket-named.
const (
	ExcludeStart = "# dckt:start"
	ExcludeEnd   = "# dckt:end"
)

// canonicalExcludeBytes is the block, markers inclusive, LF endings.
var canonicalExcludeBytes = []byte(ExcludeStart + "\n" +
	".worktrees/\n" +
	ExcludeEnd + "\n")

// MalformedExcludeError reports refusal to rewrite an exclude file whose
// `# dckt:` markers are dangling, out of order, or nested.
type MalformedExcludeError struct{}

func (e *MalformedExcludeError) Error() string {
	return "malformed " + ExcludeStart + " / " + ExcludeEnd +
		" markers in .git/info/exclude: dangling, out-of-order, or nested — refusing to rewrite"
}

// EnsureExcludeBlock returns the full-file bytes with the managed block
// present exactly once at the end: it replaces a stale block or appends the
// block when absent, preserving every byte outside it (trailing blank lines
// aside). changed is false when the input is already canonical. Malformed
// markers return *MalformedExcludeError with out==nil and the input untouched.
func EnsureExcludeBlock(current []byte) (out []byte, changed bool, err error) {
	if gitignoreMarkersMalformed(current, ExcludeStart, ExcludeEnd) {
		return nil, false, &MalformedExcludeError{}
	}
	out = buildExcludeFile(stripGitignoreBlock(current, ExcludeStart, ExcludeEnd))
	if bytes.Equal(out, current) {
		return current, false, nil
	}
	return out, true, nil
}

// ValidExcludeBlock reports whether current has well-formed markers and is
// exactly what EnsureExcludeBlock would produce from it.
func ValidExcludeBlock(current []byte) bool {
	if gitignoreMarkersMalformed(current, ExcludeStart, ExcludeEnd) {
		return false
	}
	return bytes.Equal(current, buildExcludeFile(stripGitignoreBlock(current, ExcludeStart, ExcludeEnd)))
}

// buildExcludeFile trims trailing newlines from the outside content, adds one
// separating newline when that content is non-empty, and appends the block.
func buildExcludeFile(rest []byte) []byte {
	trimmed := bytes.TrimRight(rest, "\n")
	var b bytes.Buffer
	if len(trimmed) > 0 {
		b.Write(trimmed)
		b.WriteByte('\n')
	}
	b.Write(canonicalExcludeBytes)
	return b.Bytes()
}
