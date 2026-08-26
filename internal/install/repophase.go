package install

// This file holds the two plain install-package types that let one installation
// transaction span the machine, the retired global surfaces, and a repository's
// own parent-facing dispatch surfaces without internal/install ever importing
// internal/reposeed or internal/gitcli. The app layer owns repository discovery,
// planning, and ownership projection — all of which need those packages — and
// hands the finished half across this seam as ordinary Targets and bytes.

// StateDoc is one ownership document the transaction publishes at commit. The
// bytes are the caller's already-encoded canonical form (the machine state
// through encodeState, the repository record through reposeed.EncodeRecord); the
// transaction only journals a pre-image of the destination and publishes the new
// bytes by a same-directory atomic rename, so a failed or interrupted publish
// leaves the document either wholly old or wholly new — never torn — and a
// rollback restores its pre-image.
type StateDoc struct {
	Path  string
	Bytes []byte
}

// RepoPhase is the repository half of one installation plan, assembled by
// internal/app (which may import reposeed/gitcli) and consumed here. Keeping it
// an install-package type is exactly what lets internal/install stay free of the
// repository packages: the app layer does the worktree discovery, the surface
// planning, and the ownership projection, then hands the result across as
// Targets, owners, a projected prior State, proof-gated removals, and the record
// bytes to publish.
type RepoPhase struct {
	// Authorized is the explicit repository-layer opt-in: true only when
	// agent_harnesses was supplied by the repository or repository-local layer.
	// A nil phase and an Authorized==false phase are the same to this package —
	// no repository surface is inspected, written, or retired, and any prior
	// repository ownership record is left untouched — but they are reported
	// differently by the app layer, which is why the field is carried rather
	// than collapsed into nil.
	Authorized bool
	// Targets are the parent-facing repository surfaces to reconcile, with
	// absolute cleaned paths. Empty with Authorized==true is the deliberate
	// retire-only state: every removal is in Removals and the record is emptied.
	Targets []Target
	// Owners maps a cleaned target path to the harness owners requiring it; a
	// shared surface (AGENTS.md for codex and opencode) carries several.
	Owners map[string][]string
	// PriorState is the repository's own prior ownership record projected into a
	// synthetic install.State (reposeed.Record.ToState), so InspectTarget can
	// prove ownership against the repository surfaces the same way it does the
	// machine ones. Nil when the working tree carries no record yet.
	PriorState *State
	// Removals are the retire-everything and dropped-surface removals, already
	// proof-gated by the app layer against the prior record or the frozen legacy
	// reproducer. They ride the same journaled transaction as the writes.
	Removals []TargetRecord
	// RecordPath is where the repository ownership record is published:
	// <git-dir>/docket/install.json. It is one of the transaction's commit
	// documents, journaled and rolled back beside the machine state.
	RecordPath string
	// RecordBytes is the desired record's canonical bytes; nil when the phase is
	// unauthorized. When authorized it is always non-nil — an authorized empty
	// list publishes the encoded empty record rather than deleting it.
	RecordBytes []byte
	// Worktree is the selected working-tree root, carried only so the "not
	// authorized" no-op action can name where reconciliation would have happened.
	// It is not otherwise consulted, and is empty for a machine-only run outside
	// any repository.
	Worktree string
}
