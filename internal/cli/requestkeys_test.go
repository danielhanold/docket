package cli

import (
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// TestRequestJSONKeysReconcile proves the reflection helper returns exactly the
// sorted top-level JSON keys DisallowUnknownFields enforces for a real request.
func TestRequestJSONKeysReconcile(t *testing.T) {
	got := requestJSONKeys(&app.ChangeReconcileRequest{})
	want := []string{"id", "reconcile_log_entry", "relations", "sections", "spec_sections", "version"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requestJSONKeys = %v, want %v", got, want)
	}
}

// TestRequestJSONKeysSkipsAndPromotes proves a `json:"-"` field contributes no
// key and an embedded struct's fields are promoted into the key set.
func TestRequestJSONKeysSkipsAndPromotes(t *testing.T) {
	type embedded struct {
		Promoted string `json:"promoted"`
	}
	type fixture struct {
		embedded
		Kept    string `json:"kept"`
		Skipped string `json:"-"`
		Named   string `json:"renamed"`
	}
	got := requestJSONKeys(&fixture{})
	want := []string{"kept", "promoted", "renamed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requestJSONKeys = %v, want %v", got, want)
	}
}
