package app

import (
	"github.com/danielhanold/docket/internal/repository"
	"testing"
)

// These are the `docket run gate-before` (arm the gate) tests (change 0334,
// Task 2). gate-before re-syncs the metadata worktree to fresh origin, reads the
// in-progress claim set, captures a dispatch epoch AFTER that read, and mints a
// durable gate record — printing `gate-armed <key>` on success and
// `gate-unarmed <reason-token>` on any failure, exiting 0 either way (the report
// line is the contract). Only `implement-next` is an accepted target; anything
// else is a usage error that exits non-zero.
//
// The in-progress read reuses the same PinContext/ReadCorpus plumbing the claim
// path uses (a fresh-origin fetch inside PinContext, one change-file parse), so
// these tests inject the scriptable fakeReader for the corpus and a real temp
// git repo (newGateRepo) for the store's git-common-dir rooting.

// gateBeforeCorpus is a scriptable in-progress claim set: ids 3 and 7 are
// in-progress (the before-set), id 5 is proposed and id 8 is blocked (neither is
// an in-progress claim), so a correct before-read collects exactly {3, 7}.
func gateBeforeCorpus() []StatusBlob {
	blob := func(id int, slug, status string) StatusBlob {
		return StatusBlob{
			Kind:     repository.KindChange,
			Location: repository.LocationActive,
			Path:     groomPath(id, slug),
			Version:  miVersion,
			Data:     []byte(lifecycleChange(id, slug, status)),
		}
	}
	return []StatusBlob{
		blob(3, "alpha", "in-progress"),
		blob(7, "bravo", "in-progress"),
		blob(5, "charlie", "proposed"),
		blob(8, "delta", "blocked"),
	}
}

// gateBeforeReader is the fakeReader wired with a mainPin and the given corpus /
// injected errors.
func gateBeforeReader(t *testing.T, corpus []StatusBlob, pinErr, corpusErr error) *fakeReader {
	t.Helper()
	return &fakeReader{pin: mainPin(t), corpus: corpus, pinErr: pinErr, corpusErr: corpusErr}
}
