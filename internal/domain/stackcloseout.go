package domain

// CarriedDescendant is one stacked descendant's share of a root closeout. Proof
// is empty when the descendant's carry RELATIONSHIP into the root is
// established — an unbroken chain of merged PR destinations matching the recorded
// parent branches; otherwise it is a closed refusal token naming the first broken
// link. A merged-destination chain proves that relationship only: that the code
// actually survived into the carrying head is proven separately, in Git, by the
// caller. The descendant is always surfaced — a refusal is reported, never
// dropped — so the caller can keep the whole root recoverable.
type CarriedDescendant struct {
	ID    ChangeID
	Proof string // "" when proven; else closed refusal token
}

// The closed set of root-closeout refusal tokens a CarriedDescendant.Proof
// carries. Each names exactly why one descendant's carry into the root could
// not be proven.
const (
	carryNotStackedMerged    = "not-stacked-merged"   // the descendant itself is not stacked-merged
	carryNotDone             = "not-done"             // a cleanup start record is not done
	carryDestinationMismatch = "destination-mismatch" // a merged PR landed on the wrong branch
	carryPRUnknown           = "pr-unknown"           // a carrying merge cannot be confirmed
	carryChainBroken         = "chain-broken"         // a gap: an intermediate link is not a carried merge
	carryCycle               = "cycle"                // the stacked_on graph cycles through the root
	carryKilledAncestor      = "killed-ancestor"      // a killed change sits on the carry chain
)

// DeriveRootCloseoutSet verifies, for every change stacked transitively on root,
// the chain of merged pull-request destinations that establishes its
// stacked-merged code's carry RELATIONSHIP up into the root. A merged destination
// proves that relationship only; that the code was actually preserved in Git —
// reachable in the pinned integration history, or exact-content at the root's own
// merge result — is the caller's separate obligation before any archive write.
// The returned slice is parent-first (StackDescendantsParentFirst
// order); each entry's Proof is empty when proven or a closed refusal token
// otherwise. facts supplies each change's live PR state; a missing or unknown
// entry is treated as an unconfirmable merge (pr-unknown), never a clean carry.
//
// The *PolicyFailure return is reserved for a structural problem with the root
// argument itself — an absent or ambiguous record — and is nil once the root
// resolves, in which case every per-descendant outcome travels through Proof.
// The derivation reads only the stacked_on graph and injected PR facts: a
// rendered relation (branch name, board table) never promotes a descendant.
func DeriveRootCloseoutSet(s Snapshot, root ChangeID, facts map[ChangeID]PRFacts) ([]CarriedDescendant, *PolicyFailure) {
	rootChange, out := s.Change(root)
	switch out {
	case LookupAbsent:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: root, Reason: "root-not-found"}
	case LookupAmbiguous:
		return nil, &PolicyFailure{Kind: FailInvalidInput, Change: root, Reason: "root-ambiguous"}
	}

	descendants := StackDescendantsParentFirst(s, root)
	set := make([]CarriedDescendant, 0, len(descendants))

	// A root whose own stacked_on ancestry cycles has no clean base to carry
	// into: every descendant refuses with cycle rather than the derivation
	// looping. The check reuses the visited-set cycle-guard idiom from
	// StackDescendantsParentFirst.
	if rootAncestryCycles(s, rootChange) {
		for _, id := range descendants {
			set = append(set, CarriedDescendant{ID: id, Proof: carryCycle})
		}
		return set, nil
	}

	for _, id := range descendants {
		set = append(set, CarriedDescendant{ID: id, Proof: proveCarry(s, root, id, facts)})
	}
	return set, nil
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

// DeriveCarriedSet selects, for a LIVE carrying change (a branch about to be
// rewritten, published, or merged), every descendant whose stacked-merged code
// that branch currently PROMISES to carry: children connected by an unbroken
// chain of stacked-merged changes whose verified PR destinations match their
// recorded parent branches. It stops descending into an open intermediate
// child — grandchildren merged into a still-open child are not yet promised by
// this branch. A purported carried link (a stacked-merged child) with unknown
// or mismatched facts is surfaced with a refusal token, never omitted.
// A merged PR destination proves a RELATIONSHIP only; Git preservation of the
// content is the caller's separate obligation.
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
			if c.Status() != StatusStackedMerged {
				// An open or final child claims no carry for this parent
				// branch; grandchildren beneath it are not descended.
				continue
			}
			token := ""
			f, ok := facts[cid]
			branch := p.Branch()
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
		}
	}
	walk(pc)
	return set, nil
}

// proveCarry proves that id's stacked-merged code is carried into the closeout
// root: the start must itself be stacked-merged, and walkCarryChain must reach
// root through stacked-merged intermediates. It returns the empty string when
// the chain reaches root cleanly, or the first refusal token otherwise.
func proveCarry(s Snapshot, root, id ChangeID, facts map[ChangeID]PRFacts) string {
	start, out := s.Change(id)
	if out != LookupFound {
		return carryChainBroken
	}
	if start.Status() != StatusStackedMerged {
		return carryNotStackedMerged
	}
	_, token := walkCarryChain(s, start, facts, StatusStackedMerged, func(parent Change) bool {
		return parent.ID() == root
	})
	return token
}

// ProveDoneCarry proves the carry RELATIONSHIP of a done change whose PR merged
// into its stack parent's branch rather than the integration branch: the same
// walk closeout uses (walkCarryChain), applied to a population that is now done
// rather than stacked-merged. Every link must be done, and the walk ends at the
// first done ancestor whose own PR merged into integration — the stack root,
// whose id is returned with an empty token. Any other outcome returns a zero
// root and the first refusal token: not-done when the start is not done,
// chain-broken when it is absent or the walk never meets such an ancestor.
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

// walkCarryChain is the one stack carry walk closeout and cleanup share. It
// climbs start's stacked_on chain, verifying at each link that no node is
// revisited (cycle), the parent resolves (chain-broken), the parent is not
// killed (killed-ancestor), the current node's PR is merged (pr-unknown), and
// that merge landed on the parent's recorded branch (destination-mismatch).
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
