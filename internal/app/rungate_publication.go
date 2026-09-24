// Publication-identity reconciliation for the run-epoch mutation journal
// (change 0444). A publication whose remote outcome could not be observed leaves
// an `uncertain` admitted-mutation entry; when a LATER admission in the SAME
// epoch with an IDENTICAL publication identity completed, the completed retry's
// adapter already verified the exact postcondition (githubcli EnsurePullRequest's
// post-mutation verification; workspace PublishHead's reprobeAfterPush), so the
// original obligation is settled by a LOCAL journal comparison — no Git or GitHub
// call is ever made here. Everything in this file is a pure function of the
// durable epoch record, except settleUncertainPublications, which persists the
// settlement through the ordinary epochCAS.
package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// MutationPublication is the immutable publication identity captured at a
// publication boundary, persisted with the `admitted` journal entry BEFORE the
// external adapter runs. It carries identity only — digests and resolved
// locators, never body bytes, remote URLs, credentials, or argv. The struct is
// comparable: field-for-field identity is Go ==.
type MutationPublication struct {
	// PR publication (pr.publish): the already-resolved GitHub identity, exact
	// head branch and full requested commit, effective base branch, and
	// deterministic digests of the requested title and fully assembled body.
	RepoHost    string `json:"repo_host,omitempty"`
	RepoOwner   string `json:"repo_owner,omitempty"`
	RepoName    string `json:"repo_name,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	TitleDigest string `json:"title_digest,omitempty"`
	BodyDigest  string `json:"body_digest,omitempty"`
	// Workspace publication (workspace.publish): canonical repository identity
	// (the discovered common dir, already symlink-canonical) and remote name.
	RepoDir string `json:"repo_dir,omitempty"`
	Remote  string `json:"remote,omitempty"`
	// Shared: the exact ref/branch published and the full intended commit —
	// the head actually handed to the publication operation.
	HeadRef    string `json:"head_ref,omitempty"`
	HeadCommit string `json:"head_commit,omitempty"`
}

// publicationDigest is the deterministic digest convention for authored inputs
// in a publication descriptor: sha256 over label + NUL + exact value bytes,
// hex-encoded. The NUL keeps the label/value boundary unambiguous (labels
// contain no NUL), so distinct (label, value) pairs never collide by
// concatenation.
func publicationDigest(label, value string) string {
	sum := sha256.Sum256(append(append([]byte(label), 0), []byte(value)...))
	return hex.EncodeToString(sum[:])
}

// validPublication reports whether p is a COMPLETE descriptor for op — every
// field the operation requires is present AND every field it does not own is
// empty (operation/descriptor agreement). A nil descriptor, a partial one, or
// an unknown operation is invalid: reconciliation must never trust it
// (acceptance: legacy/malformed/unknown entries remain pending).
func validPublication(op string, p *MutationPublication) bool {
	if p == nil {
		return false
	}
	switch op {
	case OperationPRPublish:
		return p.RepoHost != "" && p.RepoOwner != "" && p.RepoName != "" &&
			p.BaseBranch != "" && p.TitleDigest != "" && p.BodyDigest != "" &&
			p.HeadRef != "" && p.HeadCommit != "" &&
			p.RepoDir == "" && p.Remote == ""
	case OperationWorkspacePublish:
		return p.RepoDir != "" && p.Remote != "" &&
			p.HeadRef != "" && p.HeadCommit != "" &&
			p.RepoHost == "" && p.RepoOwner == "" && p.RepoName == "" &&
			p.BaseBranch == "" && p.TitleDigest == "" && p.BodyDigest == ""
	default:
		return false
	}
}

// publicationRetryMatch reports whether journal entry i is an uncertain
// publication settled by a later completed identical retry — the ONLY settling
// evidence this change accepts (spec "Match against a completed identical
// retry"). Eligibility: status uncertain AND a valid descriptor for its own
// operation. Settlement: some HIGHER-index entry in the same record with the
// same OpKey, a valid descriptor equal field-for-field, and status completed.
// A still-admitted, uncertain, lower-index, cross-operation, descriptor-less,
// or malformed candidate never settles. Pure over the record: no IO, no Git,
// no GitHub.
func publicationRetryMatch(rec EpochRecord, i int) bool {
	if i < 0 || i >= len(rec.AdmittedMutations) {
		return false
	}
	m := rec.AdmittedMutations[i]
	if m.Status != mutationStatusUncertain || !validPublication(m.OpKey, m.Publication) {
		return false
	}
	for j := i + 1; j < len(rec.AdmittedMutations); j++ {
		c := rec.AdmittedMutations[j]
		if c.Status != mutationStatusCompleted || c.OpKey != m.OpKey {
			continue
		}
		if !validPublication(c.OpKey, c.Publication) {
			continue
		}
		if *c.Publication == *m.Publication {
			return true
		}
	}
	return false
}

// settleablePublicationIndexes returns, ascending, every journal index
// publicationRetryMatch settles. The settlement writer re-derives this under
// the epoch lock; accounting callers never act on a stale copy.
func settleablePublicationIndexes(rec EpochRecord) []int {
	var idxs []int
	for i := range rec.AdmittedMutations {
		if publicationRetryMatch(rec, i) {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

// settleUncertainPublications durably settles every uncertain publication entry
// proven by a later completed identical retry (change 0444). The whole
// re-read + match + write runs under one epochCAS, so the matches are
// re-derived from the FRESH record under the lock — an appended unrelated
// entry can never be cleared by an older snapshot, a raced completion
// callback or concurrent cancel serializes, and completed is never
// downgraded (the only transition is uncertain→completed on a matched
// original; identity, siblings, participants, and epoch state are untouched).
// It is invoked ONLY from authorized write paths (cancellation teardown and
// the attributed keyed successful closeout); read-only verification paths
// never call it. A persistence/read failure is a bounded finding
// ("mutation-settle-failed") — the entry stays uncertain, exclusion is
// retained, and repeating the same cancel/keyed verdict retries the write.
// A no-match pass writes nothing (errEpochFenceNoWrite).
func settleUncertainPublications(repoDir, gateKey string) (settled, findings []string) {
	var tokens []string
	err := epochCAS(repoDir, gateKey, func(rec *EpochRecord) error {
		tokens = nil // the closure's view is the fresh locked record; never carry a stale pass
		idxs := settleablePublicationIndexes(*rec)
		if len(idxs) == 0 {
			return errEpochFenceNoWrite
		}
		for _, i := range idxs {
			rec.AdmittedMutations[i].Status = mutationStatusCompleted
			tokens = append(tokens, "mutation-settled:"+rec.AdmittedMutations[i].OpKey)
		}
		return nil
	})
	if errors.Is(err, errEpochFenceNoWrite) {
		return nil, nil // nothing to settle: no write, no finding
	}
	if err != nil {
		// The write never landed: discard any tokens the aborted closure
		// accumulated so no settlement is ever reported that is not durable.
		return nil, []string{"mutation-settle-failed"}
	}
	return tokens, nil
}
