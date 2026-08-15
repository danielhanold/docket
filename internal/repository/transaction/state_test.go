package transaction

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// fakeClock is a pinned Clock for tests; the engine never reads the wall clock.
type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

// Compile-time proof the state contracts are satisfiable by concrete types.
var (
	_ Clock             = fakeClock{}
	_ StateLoader       = (*noopLoader)(nil)
	_ SemanticOperation = (*noopOperation)(nil)
)

type noopLoader struct{}

func (noopLoader) Load(context.Context, Tree) (LoadedState, error) { return LoadedState{}, nil }
func (noopLoader) ValidateEvolution(_, _ LoadedState) []domain.Finding {
	return nil
}

type noopOperation struct{}

func (noopOperation) Key() OperationKey { return "test.op" }
func (noopOperation) Plan(context.Context, AttemptState) (MutationPlan, OperationResult, error) {
	return MutationPlan{}, OperationResult{}, nil
}

// TestOverlayCopiesPlanBytes proves the overlay snapshots plan bytes at
// construction: mutating the caller's FileMutation.Bytes afterwards must not
// change what ReadBlobs serves.
func TestOverlayCopiesPlanBytes(t *testing.T) {
	original := []byte("gamma\n")
	held := make([]byte, len(original))
	copy(held, original)

	plan := makePlan(FileMutation{Path: "docs/c.md", Kind: MutationCreate, Bytes: original})
	ov, err := newOverlayTree(newBaseTree(newFixture()), plan)
	if err != nil {
		t.Fatalf("newOverlayTree: %v", err)
	}

	// Mutate the caller's backing array after construction.
	for i := range original {
		original[i] = 'Z'
	}

	got := readOne(t, ov, "docs/c.md")
	if !got.Found || !bytes.Equal(got.Blob.Bytes, held) {
		t.Fatalf("overlay served mutated bytes %q, want snapshot %q", got.Blob.Bytes, held)
	}
}

// TestAttemptStateHoldsTree proves AttemptState composes the tree, revision and
// loaded state a semantic operation reads.
func TestAttemptStateHoldsTree(t *testing.T) {
	src := newFixture()
	bt := newBaseTree(src)
	st := AttemptState{Base: src.rev, Tree: bt, State: LoadedState{}}
	if st.Tree.Revision() != src.rev || st.Base != src.rev {
		t.Fatalf("AttemptState did not carry base revision")
	}
	var _ gitcli.Revision = st.Base
}
