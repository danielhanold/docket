package app

import (
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// changeScope builds the transaction.ValidationScope for a named operation
// rooted at change id whose pinned record path is recPath (change 0449). The
// resolver runs against each state the engine hands it — the before state and
// the candidate — and resolves the root by IDENTITY, not path, because
// closeout legally moves the root from active/ to archive/. The resolved set
// is the root's structural closure (ADR-0093):
//
//   - recPath itself, plus every record path carrying id in that state (they
//     differ mid-closeout; an ambiguous id contributes every carrier — the
//     snapshot picks no winner, so the scope does not either);
//   - every direct depends_on target's path — a target no record carries
//     contributes nothing here, because the dangling-reference error sits on
//     the root itself and is already relevant through it;
//   - every stack ancestor's path (domain.StackAncestors);
//   - with withDescendants, every stack descendant's path
//     (domain.StackDescendantsParentFirst) — closeout carries them;
//   - every non-empty extra path the caller knows its operation requires.
//
// Associative citations (related, discovered_from, adrs) never create
// subjects. Duplicate and cycle findings need no enumeration: they name the
// root id in their Entity or Related refs, so the engine's relevance check
// already catches them. The set always holds recPath when it is non-empty; an
// empty set makes the engine fall back to strict whole-corpus validation.
func changeScope(id int, recPath string, withDescendants bool, extra ...string) *transaction.ValidationScope {
	extras := append([]string(nil), extra...)
	root := domain.ChangeID(id)
	return &transaction.ValidationScope{Subjects: func(st transaction.LoadedState) (map[gitcli.RepoPath]bool, error) {
		set := map[gitcli.RepoPath]bool{}
		add := func(p string) {
			if p != "" {
				set[gitcli.RepoPath(p)] = true
			}
		}
		add(recPath)
		for _, p := range extras {
			add(p)
		}
		snap := st.Snapshot
		for _, c := range changesCarrying(snap, root) {
			add(c.Path())
			for _, dep := range c.DependsOn() {
				for _, d := range changesCarrying(snap, dep) {
					add(d.Path())
				}
			}
			for _, anc := range domain.StackAncestors(snap, c) {
				for _, a := range changesCarrying(snap, anc) {
					add(a.Path())
				}
			}
		}
		if withDescendants {
			for _, desc := range domain.StackDescendantsParentFirst(snap, root) {
				for _, d := range changesCarrying(snap, desc) {
					add(d.Path())
				}
			}
		}
		return set, nil
	}}
}

// changesCarrying returns every change record in snap whose id is id — one for
// a found id, several for an ambiguous one, none for an absent one.
func changesCarrying(snap domain.Snapshot, id domain.ChangeID) []domain.Change {
	var out []domain.Change
	for _, c := range snap.Changes() {
		if c.ID() == id {
			out = append(out, c)
		}
	}
	return out
}

// adrScope is changeScope's shape rooted at an ADR write: the ADR's own path,
// its structural supersede/reverse target paths, and the ADR index path. ADR
// records never relocate, so the set is the same for every state the engine
// resolves it against.
func adrScope(adrPath string, targetPaths []string, indexPath string) *transaction.ValidationScope {
	paths := append([]string{adrPath, indexPath}, targetPaths...)
	return &transaction.ValidationScope{Subjects: func(transaction.LoadedState) (map[gitcli.RepoPath]bool, error) {
		set := map[gitcli.RepoPath]bool{}
		for _, p := range paths {
			if p != "" {
				set[gitcli.RepoPath(p)] = true
			}
		}
		return set, nil
	}}
}
