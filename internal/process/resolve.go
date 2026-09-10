package process

import (
	"os"
	"path/filepath"
)

// ReservationResolution is ResolveReservation's verdict for a caller
// reservation token whose launch response never reached the caller. Disposition
// is exactly one of:
//
//	"never-launched" — a clean, complete census carried no matching manifest;
//	                   the launch provably never wrote a run.
//	"identified"     — a manifest carries the token and the run is addressable;
//	                   State and Terminal report its observed disposition.
//	"unresolved"     — the token's fate cannot be proven: a matching manifest is
//	                   still in the allocated phase (no addressable group), or the
//	                   census was incomplete (an unreadable slot could be the one).
//
// State and Terminal are populated only for "identified"; RunID/RunDir are set
// whenever a matching manifest was found (identified or unresolved-allocated).
type ReservationResolution struct {
	Disposition string
	RunID       string
	RunDir      string
	State       State
	Terminal    *Terminal
}

// ResolveReservation answers what became of a launch that was handed the caller
// reservation token but whose response was lost. It takes the slot census under
// the registry lock so no allocation races it (mirroring recoverSnapshot), then
// reads each censused manifest with no lock held — every slot visible in a
// census taken under the lock has a durable manifest, because Launch writes the
// manifest before releasing the registry lock.
//
// The order is fail-closed (spec: fail closed everywhere): a matching manifest
// still in the allocated phase is "unresolved" (groupAlive(0) is unprovable by
// design and stays that way); an established/running/terminal match delegates to
// Observe for its exact state and is "identified". Only an empty result over a
// census in which every slot was readable is "never-launched" — an unreadable,
// malformed, or self-disagreeing manifest is not clean absence (it could be the
// very slot carrying the token), so a no-match verdict then fails closed to
// "unresolved", never "never-launched".
func (s *Service) ResolveReservation(root, token string) (*ReservationResolution, error) {
	if !filepath.IsAbs(root) {
		return nil, failf(FailInvalidInput, "resolve-reservation", "root must be an absolute path")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, failf(FailInvalidInput, "resolve-reservation", "root must be an existing directory")
	}
	if !reservationTokenPattern.MatchString(token) {
		return nil, failf(FailInvalidInput, "resolve-reservation", "token must be lowercase hex up to 128 characters")
	}

	snap, err := s.recoverSnapshot(root)
	if err != nil {
		return nil, err
	}

	censusClean := true
	for _, name := range snap.candidates {
		runDir := filepath.Join(root, name)
		m, merr := readManifest(runDir)
		if merr != nil || m == nil || m.RunID != name {
			// Not clean absence: this could be the slot carrying the token. Fail
			// closed so a final no-match verdict becomes unresolved.
			censusClean = false
			continue
		}
		if m.Token != token {
			continue
		}
		// Matched. An allocated-phase (or otherwise unrecognized) manifest never
		// published an addressable group — its outcome is genuinely unresolved.
		switch m.Phase {
		case "established", "running", "terminal":
			obs, oerr := s.Observe(runDir)
			if oerr != nil {
				return nil, oerr
			}
			return &ReservationResolution{
				Disposition: "identified",
				RunID:       obs.RunID,
				RunDir:      obs.RunDir,
				State:       obs.State,
				Terminal:    obs.Terminal,
			}, nil
		default:
			return &ReservationResolution{
				Disposition: "unresolved",
				RunID:       m.RunID,
				RunDir:      runDir,
			}, nil
		}
	}

	if !censusClean {
		return &ReservationResolution{Disposition: "unresolved"}, nil
	}
	return &ReservationResolution{Disposition: "never-launched"}, nil
}
