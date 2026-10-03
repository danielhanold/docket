// Publication-identity reconciliation for the run mutation journal
// (change 0444). A publication whose remote outcome could not be observed leaves
// an `uncertain` admitted-mutation entry; when a LATER admission in the SAME
// run with an IDENTICAL publication identity completed VERIFIED (applied or
// no-op — the retry's adapter observed the exact postcondition: githubcli
// EnsurePullRequest's post-mutation verification; workspace PublishHead's
// reprobeAfterPush), the original obligation is settled by a LOCAL journal
// comparison — no Git or GitHub call is ever made here. A retry that completed
// without verifying (contended, a local refusal, an internal error) is no
// evidence. Everything in this file is a pure function of the durable run
// record, except settleUncertainPublications, which persists the settlement
// through the ordinary runRecordCAS (a settled original becomes completed but stays
// unverified: its own attempt observed nothing). A dead publisher's
// admitted→uncertain rewrite (change 0494) is bookkeeping, not settlement, and
// never marks an entry completed.
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
// publication settled by a later verified identical retry — the ONLY settling
// evidence this change accepts (spec "Match against a completed identical
// retry"). Eligibility: status uncertain AND a valid descriptor for its own
// operation. Settlement: some HIGHER-index entry in the same record with the
// same OpKey, a valid descriptor equal field-for-field, status completed, AND
// Verified (the retry observed the postcondition — mutationJournalOutcome). A
// still-admitted, uncertain, completed-but-unverified (contended, refused,
// internally failed, or a legacy entry with no verified flag), lower-index,
// cross-operation, descriptor-less, or malformed candidate never settles —
// missing evidence never counts as success. Pure over the record: no IO, no Git,
// no GitHub. A dead publisher's admitted→uncertain rewrite (change 0494) is
// bookkeeping, not settlement, and never marks an entry completed.
func publicationRetryMatch(rec RunRecord, i int) bool {
	if i < 0 || i >= len(rec.AdmittedMutations) {
		return false
	}
	m := rec.AdmittedMutations[i]
	if m.Status != mutationStatusUncertain || !validPublication(m.OpKey, m.Publication) {
		return false
	}
	for j := i + 1; j < len(rec.AdmittedMutations); j++ {
		c := rec.AdmittedMutations[j]
		if c.Status != mutationStatusCompleted || !c.Verified || c.OpKey != m.OpKey {
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
// the run lock; accounting callers never act on a stale copy.
func settleablePublicationIndexes(rec RunRecord) []int {
	var idxs []int
	for i := range rec.AdmittedMutations {
		if publicationRetryMatch(rec, i) {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

// settleUncertainPublications durably records abandonment and settles every
// uncertain publication entry proven by a later verified identical retry. Its
// only transitions are admitted→uncertain, for an entry whose publisher is
// provably gone (publisherGone: its publish lock reads free; change 0494), and
// uncertain→completed, for a retry-matched original (change 0444). The
// abandonment step runs first, so a dead publisher's entry can still be settled
// by the 0444 match in the same pass. The whole re-read + mark + match + write
// runs under one runRecordCAS, so both are re-derived from the FRESH record
// under the lock — an appended unrelated entry can never be cleared by an older
// snapshot, a raced completion callback or concurrent cancel serializes, and
// completed is never downgraded (identity, siblings, participants, and run
// state are untouched). It is invoked ONLY from authorized write paths
// (cancellation teardown and the attributed keyed successful closeout);
// read-only verification paths never call it. A persistence/read failure is a
// bounded finding ("mutation-settle-failed") and repeating the same cancel/keyed
// verdict retries the write. The abandonment write is bookkeeping: the lock
// probe is the evidence, so a failed write still lets classifyAdmittedMutation
// read the free lock and account the entry abandoned. A pass with nothing to
// mark or settle writes nothing (errRunFenceNoWrite).
func settleUncertainPublications(repoDir, runKey string) (settled, findings []string) {
	dir := runJournalDir(repoDir, runKey) // "" ⇒ no entry can be proven abandoned
	var tokens []string
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		tokens = nil // the closure's view is the fresh locked record; never carry a stale pass
		// (change 0494) Record abandonment first: an admitted entry whose publish lock
		// reads free has no publisher left to write its outcome, so it becomes
		// uncertain ("outcome unknown", never verified). The probe only tries and never
		// waits, so holding run.lock here cannot deadlock a publisher that holds its
		// publish lock while it waits on run.lock for its own outcome write.
		marked := false
		for i := range rec.AdmittedMutations {
			if publisherGone(dir, rec.AdmittedMutations[i]) {
				rec.AdmittedMutations[i].Status = mutationStatusUncertain
				rec.AdmittedMutations[i].Verified = false
				marked = true
			}
		}
		idxs := settleablePublicationIndexes(*rec)
		if len(idxs) == 0 && !marked {
			return errRunFenceNoWrite
		}
		for _, i := range idxs {
			rec.AdmittedMutations[i].Status = mutationStatusCompleted
			tokens = append(tokens, "mutation-settled:"+rec.AdmittedMutations[i].OpKey)
		}
		return nil
	})
	if errors.Is(err, errRunFenceNoWrite) {
		return nil, nil // nothing to mark or settle: no write, no finding
	}
	if err != nil {
		// The write never landed: discard any tokens the aborted closure
		// accumulated so no settlement is ever reported that is not durable.
		return nil, []string{"mutation-settle-failed"}
	}
	return tokens, nil
}
