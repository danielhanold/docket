package app

import (
	"errors"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
)

// The fail-closed errors recordedBranch returns for an unusable recorded
// branch. Callers map them to their own invalid-state refusal and perform NO
// mutation (Global Constraints: unresolved identity authorizes no effects).
var (
	errBranchMissing   = errors.New("branch-missing")
	errBranchMalformed = errors.New("branch-malformed")
)

// recordedBranch returns c's recorded feature branch. It fails closed: an
// absent or empty branch: on a post-claim record is errBranchMissing, a value
// that cannot be a branch ref is errBranchMalformed. Callers map the error to
// their own invalid-state refusal and perform NO mutation. It never
// reconstructs a branch from the slug, type, or prefix — a post-claim operation
// consumes the recorded branch, nothing else (mint once, record once, consume
// the record).
func recordedBranch(c domain.Change) (string, error) {
	b := c.Branch()
	if b.State != domain.FieldPresent || b.Value == "" {
		return "", errBranchMissing
	}
	if strings.HasPrefix(b.Value, "refs/") || strings.HasPrefix(b.Value, "-") ||
		strings.ContainsAny(b.Value, " \t\r\n\v\f") || strings.Contains(b.Value, "@{") ||
		strings.Contains(b.Value, "..") || strings.IndexByte(b.Value, 0) >= 0 {
		return "", errBranchMalformed
	}
	return b.Value, nil
}
