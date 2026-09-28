package transaction

import (
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

// engineClock is the pinned instant every engine test commits and stamps with.
var engineClock = fakeClock{t: time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)}

func TestNewEngineRejectsNilDependencies(t *testing.T) {
	if _, err := NewEngine(nil, engineClock); err == nil {
		t.Error("NewEngine(nil client): want error")
	}
	client, err := gitcli.NewClient()
	if err != nil {
		t.Skipf("NewClient: %v", err)
	}
	if _, err := NewEngine(client, nil); err == nil {
		t.Error("NewEngine(nil clock): want error")
	}
}
