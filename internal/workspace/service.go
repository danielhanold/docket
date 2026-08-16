package workspace

import (
	"errors"

	"github.com/danielhanold/docket/internal/gitcli"
)

// Service is the workspace engine. It owns a single gitcli.Client and exposes no
// mutable client, manifest, or environment: every operation reconstructs its
// state from the client, the on-disk manifest, and live Git. The zero value is
// not usable — construct one with NewService.
type Service struct {
	git *gitcli.Client
}

// NewService wraps a gitcli.Client in a Service. A nil client is invalid-input:
// the service cannot function without a Git process adapter.
func NewService(git *gitcli.Client) (*Service, error) {
	if git == nil {
		return nil, &Failure{Op: "new-service", Stage: "validate", Kind: KindInvalidInput, Detail: "nil git client"}
	}
	return &Service{git: git}, nil
}

// mapGitFailure translates a gitcli error into a workspace Failure tagged with
// op and stage, folding gitcli's finer failure kinds into the workspace's
// closed set. A gitcli.Failure's Detail is already bounded and redacted, so it
// is safe to carry through; a non-gitcli error is reported as external with a
// generic detail. Ref/remote/command/executable-unavailable conditions are all
// external Git circumstances; malformed output is invalid-output; a bad request
// (which the service should never emit) surfaces as invalid-input; cancellation
// and timeout keep their identity.
func mapGitFailure(op string, stage Stage, err error) *Failure {
	var gf *gitcli.Failure
	if errors.As(err, &gf) {
		kind := KindExternal
		switch gf.Kind {
		case gitcli.KindInvalidRequest:
			kind = KindInvalidInput
		case gitcli.KindInvalidOutput, gitcli.KindUnexpectedObject:
			kind = KindInvalidOutput
		case gitcli.KindCancelled:
			kind = KindCancelled
		case gitcli.KindTimedOut:
			kind = KindTimedOut
		case gitcli.KindInvalidRepository:
			kind = KindInvalidState
		}
		return &Failure{Op: op, Stage: stage, Kind: kind, Detail: gf.Detail, Err: err}
	}
	return &Failure{Op: op, Stage: stage, Kind: KindExternal, Detail: "git operation failed", Err: err}
}
