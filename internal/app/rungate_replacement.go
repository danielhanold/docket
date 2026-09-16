package app

import (
	"path/filepath"
	"strconv"

	"github.com/danielhanold/docket/internal/gatedrive"
)

// epochReplacementResolver proves that a released slot's old epoch reaches the
// incoming active epoch through only confirmed-cancelled/superseded replacements.
// The chain, not timestamps or a shared change number alone, authorizes admission.
// Historical attempts may have launched no drive, leaving the slot several links
// behind. The returned release keeps the incoming epoch locked until the slot CAS
// completes. Cancellation must therefore see either no admission or its durable
// reservation, never a released ancestor followed by a late reservation.
func epochReplacementResolver(common string) gatedrive.EpochReplacementFunc {
	root := filepath.Join(common, "docket", "rungate")
	return func(previous, next, worktree, change string) (release func(), err error) {
		incoming, found, err := findEpochByID(root, next)
		if err != nil || !found {
			return nil, err
		}
		dir, err := gateKeyDir(common, incoming.GateKey, "replacement-proof")
		if err != nil {
			return nil, err
		}
		lock, err := acquireEpochLock(dir)
		if err != nil {
			return nil, err
		}
		defer func() {
			if release == nil {
				lock.Close()
			}
		}()
		incoming, _, err = readStoredEpoch(dir, "replacement-proof")
		if err != nil {
			return nil, err
		}
		if incoming.EpochID != next {
			return nil, nil
		}
		if incoming.State != EpochActive || !epochOwnsWorktree(incoming.Worktree, worktree) {
			return nil, nil
		}
		gate, err := LoadGateRecord(common, incoming.GateKey)
		if err != nil {
			return nil, err
		}
		if gate.AttributedID <= 0 || strconv.Itoa(gate.AttributedID) != change {
			return nil, nil
		}
		prior, found, err := findEpochByID(root, previous)
		if err != nil || !found {
			return nil, err
		}
		seen := map[string]bool{}
		for prior.EpochID != next {
			if seen[prior.EpochID] || prior.State != EpochSuperseded || prior.ReplacementReserved == "" {
				return nil, nil
			}
			seen[prior.EpochID] = true
			if prior.ChangeID != change {
				if prior.ChangeID != "" {
					return nil, nil
				}
				g, err := LoadGateRecord(common, prior.GateKey)
				if err != nil {
					return nil, err
				}
				if g.AttributedID != gate.AttributedID {
					return nil, nil
				}
			}
			prior, _, err = LoadEpochRecord(common, prior.ReplacementReserved)
			if err != nil {
				return nil, err
			}
		}
		if prior.GateKey != incoming.GateKey {
			return nil, nil
		}
		return func() { lock.Close() }, nil
	}
}
