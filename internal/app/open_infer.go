package app

// open_infer.go infers which change `docket open` means when no id is given:
// it reads the branch checked out in the working directory and matches it
// against the branch each change records.

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// checkoutBranch returns the short name of the branch checked out at dir. A
// probe error is reported as checkout-unreadable, never as a branch that
// belongs to no change; a detached HEAD is its own refusal.
func checkoutBranch(ctx context.Context, probe func(context.Context, string) (gitcli.CheckoutState, error), dir string) (string, *openFailure) {
	st, err := probe(ctx, dir)
	if err != nil {
		return "", &openFailure{ResultExternalFailed, ReasonOpenCheckoutUnreadable,
			fmt.Sprintf("could not read the branch checked out at %s: %v — pass a change id", dir, err)}
	}
	if st.Detached || st.Branch == "" {
		return "", &openFailure{ResultInvalidInput, ReasonOpenHeadDetached, "HEAD is detached — pass a change id"}
	}
	return strings.TrimPrefix(string(st.Branch), branchRefPrefix), nil
}

// inferChangeFromBranch finds the change whose recorded branch is branch.
// Active records win; archived records are consulted only when no active one
// matches. No match, or more than one in the winning set, is refused with a
// message telling the user to pass a change id.
func inferChangeFromBranch(snap domain.Snapshot, branch string) (domain.Change, *openFailure) {
	var active, archived []domain.Change
	for _, c := range snap.Changes() {
		b, err := recordedBranch(c)
		if err != nil || b != branch {
			continue
		}
		if c.Location() == domain.LocationArchive {
			archived = append(archived, c)
		} else {
			active = append(active, c)
		}
	}
	matches := active
	if len(matches) == 0 {
		matches = archived
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return domain.Change{}, &openFailure{ResultInvalidInput, ReasonOpenBranchUnmatched,
			fmt.Sprintf("branch %s belongs to no change — pass a change id", branch)}
	}
	ids := make([]int, 0, len(matches))
	for _, c := range matches {
		ids = append(ids, int(c.ID()))
	}
	sort.Ints(ids)
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return domain.Change{}, &openFailure{ResultInvalidInput, ReasonOpenBranchAmbiguous,
		fmt.Sprintf("branch %s is recorded by changes %s — pass a change id", branch, strings.Join(parts, ", "))}
}
