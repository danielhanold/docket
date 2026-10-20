package domain

// CarriedDescendant is one stacked descendant a parent carries. A parent
// carries exactly the children whose pull requests merged into its recorded
// branch, transitively: a child is carried when its status is stacked-merged or
// implemented and its live PR facts show a merge whose base is its parent's
// recorded branch. Proof is empty when that carry RELATIONSHIP is established;
// otherwise it is a closed refusal token naming why a purported carried link
// could not be confirmed. A merged destination proves the relationship only:
// that the code actually survived into the carrying head or the integration
// branch is proven separately, in Git, by the caller. A refusal is reported,
// never dropped, so the caller can keep the whole parent recoverable.
type CarriedDescendant struct {
	ID    ChangeID
	Proof string // "" when proven; else closed refusal token
}

// The closed set of carry refusal tokens a CarriedDescendant.Proof or
// ProveDoneCarry returns. Each names exactly why one carry could not be proven.
const (
	carryNotDone             = "not-done"             // a cleanup start record is not done
	carryDestinationMismatch = "destination-mismatch" // a merged PR landed on the wrong branch
	carryPRUnknown           = "pr-unknown"           // a carrying merge cannot be confirmed
	carryChainBroken         = "chain-broken"         // a gap: an intermediate link is not a carried merge
	carryCycle               = "cycle"                // the stacked_on graph cycles through the root
	carryKilledAncestor      = "killed-ancestor"      // a killed change sits on the carry chain
)

// DeriveRootCloseoutSet is the carried set a landed root's closeout archives
// with it: exactly DeriveCarriedSet's answer for root, so closeout and the
// live-branch boundaries (rebase, publish, merge) share one carry rule. A child
// the root does not carry never blocks the root's closeout. A merged destination
// proves the carry relationship only; that each carried child's code was
// actually preserved on the integration branch is the caller's separate
// obligation before any archive write.
//
// The *PolicyFailure return is reserved for a structural problem with the root
// argument itself — an absent or ambiguous record — and is nil once the root
// resolves. A root whose own stacked_on ancestry cycles has no clean base to
// carry into, so every descendant (StackDescendantsParentFirst order) refuses
// with cycle; otherwise every per-descendant outcome travels through Proof.
func DeriveRootCloseoutSet(s Snapshot, root ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure) {
	rootChange, out := s.Change(root)
	switch out {
	case LookupAbsent:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: root, Reason: "root-not-found"}
	case LookupAmbiguous:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: root, Reason: "root-ambiguous"}
	}

	if rootAncestryCycles(s, rootChange) {
		descendants := StackDescendantsParentFirst(s, root)
		set := make([]CarriedDescendant, 0, len(descendants))
		for _, id := range descendants {
			set = append(set, CarriedDescendant{ID: id, Proof: carryCycle})
		}
		return set, nil
	}

	return DeriveCarriedSet(s, root, facts)
}

// RootCloseoutProven reports whether every carried descendant is proven. It is
// all-or-nothing: a single unproven entry fails the whole set, so the caller
// keeps the root recoverable rather than writing a false descendant done
// record. An empty set is vacuously proven.
func RootCloseoutProven(set []CarriedDescendant) bool {
	for _, d := range set {
		if d.Proof != "" {
			return false
		}
	}
	return true
}

// DeriveCarriedSet selects every descendant parent carries: the children whose
// pull requests merged into parent's recorded branch, transitively. It is the
// one carry rule rebase, publish, merge, and root closeout share. A child is
// carried when its live PR facts show a merge whose base is its parent's
// recorded branch and its status is either
//
//   - stacked-merged: its own closeout recorded the merge. Because that status
//     claims a carry, a stacked-merged child whose facts are missing or unmerged
//     is surfaced with pr-unknown, and one whose merge landed elsewhere (or whose
//     parent records no branch) with destination-mismatch — never omitted.
//   - implemented: the merge happened but the child's own closeout never ran.
//     Merging into the parent's branch is the test, so any other PR state, or no
//     usable PR at all, just means "not carried": no entry and no token.
//
// Any other status is not carried. The walk is parent-first (pre-order depth
// first) and descends through carried children only: grandchildren beneath a
// child that is not carried are not this parent's to carry, and neither appear
// nor block. The walk reads only the stacked_on graph and the injected PR
// facts: a rendered relation (branch name, board table) never promotes a
// descendant. A merged PR destination proves a RELATIONSHIP only; Git
// preservation of the content is the caller's separate obligation.
//
// The *PolicyFailure return is reserved for a structural problem with the
// parent argument itself — an absent or ambiguous record — and is nil once the
// parent resolves, in which case every per-descendant outcome travels through
// Proof. The walk is guarded by a visited set (seeded with the parent) so a
// malformed graph terminates, the revisited node reporting cycle.
func DeriveCarriedSet(s Snapshot, parent ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure) {
	pc, out := s.Change(parent)
	switch out {
	case LookupAbsent:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: parent, Reason: "parent-not-found"}
	case LookupAmbiguous:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: parent, Reason: "parent-ambiguous"}
	}

	set := []CarriedDescendant{}
	visited := map[ChangeID]bool{parent: true}
	var walk func(p Change)
	walk = func(p Change) {
		for _, cid := range StackChildren(s, p.ID()) {
			if visited[cid] {
				set = append(set, CarriedDescendant{ID: cid, Proof: carryCycle})
				continue
			}
			visited[cid] = true
			c, out := s.Change(cid)
			if out != LookupFound {
				set = append(set, CarriedDescendant{ID: cid, Proof: carryChainBroken})
				continue
			}
			f, ok := facts[cid]
			branch := p.Branch()
			switch c.Status() {
			case StatusStackedMerged:
				token := ""
				switch {
				case !ok || f.State != prStateMerged:
					token = carryPRUnknown
				case branch.State != FieldPresent || branch.Value == "" || f.BaseRef != branch.Value:
					token = carryDestinationMismatch
				}
				set = append(set, CarriedDescendant{ID: cid, Proof: token})
				if token == "" {
					walk(c)
				}
			case StatusImplemented:
				if !ok || f.State != prStateMerged ||
					branch.State != FieldPresent || branch.Value == "" || f.BaseRef != branch.Value {
					// Not merged into this parent's branch: not carried, and
					// grandchildren beneath it are not descended.
					continue
				}
				set = append(set, CarriedDescendant{ID: cid})
				walk(c)
			default:
				// An open or final child claims no carry for this parent
				// branch; grandchildren beneath it are not descended.
				continue
			}
		}
	}
	walk(pc)
	return set, nil
}

// ProveDoneCarry proves the carry RELATIONSHIP of a done change whose PR merged
// into its stack parent's branch rather than the integration branch, climbing
// its stacked_on chain with walkCarryChain. Once a root's closeout has archived
// everything it carried, each carried child is done with a PR merged into its
// parent's branch, which is the population this walk expects. Every link must
// be done, and the walk ends at the first done ancestor whose own PR merged
// into integration — the stack root, whose id is returned with an empty token.
// Any other outcome returns a zero root and the first refusal token: not-done
// when the start is not done, chain-broken when it is absent or the walk never
// meets such an ancestor.
//
// facts supplies each change's live PR state; a missing or unmerged entry is an
// unconfirmable link, never a clean carry. A merged destination proves the
// relationship only: that the child's content actually reached the integration
// branch is the caller's separate obligation before anything is deleted.
func ProveDoneCarry(s Snapshot, id ChangeID, facts map[ChangeID]PRFacts, integration string) (root ChangeID, token string) {
	start, out := s.Change(id)
	if out != LookupFound {
		return 0, carryChainBroken
	}
	if start.Status() != StatusDone {
		return 0, carryNotDone
	}
	return walkCarryChain(s, start, facts, StatusDone, func(parent Change) bool {
		if integration == "" || parent.Status() != StatusDone {
			return false
		}
		f, ok := facts[parent.ID()]
		return ok && f.State == prStateMerged && f.BaseRef == integration
	})
}

// walkCarryChain is the upward carry walk ProveDoneCarry uses for cleanup;
// closeout, rebase, publish, and merge walk downward with DeriveCarriedSet
// instead. It climbs start's stacked_on chain, verifying at each link that no
// node is revisited (cycle), the parent resolves (chain-broken), the parent is
// not killed (killed-ancestor), the current node's PR is merged (pr-unknown),
// and that merge landed on the parent's recorded branch (destination-mismatch).
// When isRoot accepts the parent, the walk returns its id with an empty token;
// otherwise the parent must itself carry linkStatus — an intermediate carried
// link — or the chain has a gap (chain-broken). On any refusal the returned id
// is zero. The visited set guarantees a malformed graph terminates.
func walkCarryChain(s Snapshot, start Change, facts map[ChangeID]PRFacts, linkStatus Status, isRoot func(parent Change) bool) (ChangeID, string) {
	visited := map[ChangeID]bool{}
	for current := start; ; {
		if visited[current.ID()] {
			return 0, carryCycle
		}
		visited[current.ID()] = true

		parent, out := StackParent(s, current)
		if out != LookupFound {
			return 0, carryChainBroken
		}
		if parent.Status() == StatusKilled {
			return 0, carryKilledAncestor
		}

		f, ok := facts[current.ID()]
		if !ok || f.State != prStateMerged {
			return 0, carryPRUnknown
		}
		branch := parent.Branch()
		if branch.State != FieldPresent || branch.Value == "" || f.BaseRef != branch.Value {
			return 0, carryDestinationMismatch
		}

		if isRoot(parent) {
			return parent.ID(), "" // carried all the way into the root
		}
		// An intermediate link must itself be a carried link of the walked
		// population, or the destination chain has a gap the root cannot absorb.
		if parent.Status() != linkStatus {
			return 0, carryChainBroken
		}
		current = parent
	}
}

// rootAncestryCycles reports whether root's own stacked_on chain loops. A clean
// root is an unstacked base (StackParent absent); any chain that revisits a
// node before ending is a cycle, detected with the same visited-set idiom the
// stack walks use.
func rootAncestryCycles(s Snapshot, root Change) bool {
	visited := map[ChangeID]bool{root.ID(): true}
	for current := root; ; {
		parent, out := StackParent(s, current)
		if out != LookupFound {
			return false
		}
		if visited[parent.ID()] {
			return true
		}
		visited[parent.ID()] = true
		current = parent
	}
}
