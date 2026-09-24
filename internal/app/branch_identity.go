package app

import (
	"errors"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
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
// that cannot be a branch ref is errBranchMalformed. The shape check delegates
// to gitcli.ValidBranchName (change 0454), so it agrees with the whole-corpus
// probe filter and with status's branch-malformed finding: a name git would
// reject is refused here as errBranchMalformed rather than failing inside git
// — delegation, not a second enumeration of the grammar. Callers map the error to
// their own invalid-state refusal and perform NO mutation. It never
// reconstructs a branch from the slug, type, or prefix — a post-claim operation
// consumes the recorded branch, nothing else (mint once, record once, consume
// the record).
func recordedBranch(c domain.Change) (string, error) {
	b := c.Branch()
	if b.State != domain.FieldPresent || b.Value == "" {
		return "", errBranchMissing
	}
	// Two refusals survive the delegation because the prefixed-ref question
	// cannot see them: a "refs/" value would smuggle a full ref (prefixing it
	// still yields a *valid* ref), and a leading "-" is option smuggling
	// wherever the short name is used bare.
	if strings.HasPrefix(b.Value, "refs/") || strings.HasPrefix(b.Value, "-") ||
		!gitcli.ValidBranchName(b.Value) {
		return "", errBranchMalformed
	}
	return b.Value, nil
}
