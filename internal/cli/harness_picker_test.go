package cli

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/charmbracelet/huh"

	"github.com/danielhanold/docket/internal/app"
)

func harnessRequest(pre ...string) app.HarnessChoiceRequest {
	return app.HarnessChoiceRequest{Options: []string{"claude", "codex", "cursor", "opencode"}, Preselected: pre, ConfigPath: ".docket.yml"}
}

// TestHarnessFormPrechecksAndToggles drives the form in huh's accessible mode:
// cursor arrives pre-checked, toggling option 1 adds claude, and 0 confirms.
func TestHarnessFormPrechecksAndToggles(t *testing.T) {
	var picked []string
	f := newHarnessForm(harnessRequest("cursor"), &picked)
	in := iotest.OneByteReader(strings.NewReader("1\n0\n"))
	if err := f.WithAccessible(true).WithInput(in).WithOutput(io.Discard).Run(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(picked)
	if !slices.Equal(picked, []string{"claude", "cursor"}) {
		t.Fatalf("picked = %v, want [claude cursor]", picked)
	}
}

// TestHarnessFormConfirmWithNothingChecked proves confirming an empty checklist
// is a valid answer (no agents), not a refusal.
func TestHarnessFormConfirmWithNothingChecked(t *testing.T) {
	var picked []string
	f := newHarnessForm(harnessRequest(), &picked)
	in := iotest.OneByteReader(strings.NewReader("0\n"))
	if err := f.WithAccessible(true).WithInput(in).WithOutput(io.Discard).Run(); err != nil {
		t.Fatal(err)
	}
	if len(picked) != 0 {
		t.Fatalf("picked = %v, want empty", picked)
	}
}

func stubHarnessForm(t *testing.T, fn func(ctx context.Context, f *huh.Form) error) {
	t.Helper()
	old := runHarnessForm
	runHarnessForm = fn
	t.Cleanup(func() { runHarnessForm = old })
}

func TestHuhHarnessChooserMapsAbortToCancel(t *testing.T) {
	stubHarnessForm(t, func(context.Context, *huh.Form) error { return huh.ErrUserAborted })
	_, err := huhHarnessChooser(context.Background(), harnessRequest("claude"))
	if !errors.Is(err, app.ErrHarnessChoiceCancelled) {
		t.Fatalf("err = %v, want ErrHarnessChoiceCancelled", err)
	}
}

func TestHuhHarnessChooserReturnsNonNilEmpty(t *testing.T) {
	stubHarnessForm(t, func(context.Context, *huh.Form) error { return nil })
	got, err := huhHarnessChooser(context.Background(), harnessRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %#v, want a non-nil empty selection", got)
	}
}
