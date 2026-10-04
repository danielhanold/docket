package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// TestDeclareJSONFileRecordsTheDecodedType proves the declaration records the
// flag and the full type identity of T, and the returned decoder strictly
// decodes that same T from the declared flag (stdin via "-").
func TestDeclareJSONFileRecordsTheDecodedType(t *testing.T) {
	cmd := &cobra.Command{Use: "probe"} // nil Annotations: the helper must allocate
	cmd.Flags().String("input", "", "")
	decode := declareJSONFile[app.ChangeHaltInput](cmd, "input")

	if got := cmd.Annotations[jsonFileAnnotationFlag]; got != "input" {
		t.Errorf("flag annotation = %q, want input", got)
	}
	want := jsonFileTypeName(reflect.TypeFor[app.ChangeHaltInput]())
	if !strings.HasSuffix(want, "/internal/app.ChangeHaltInput") {
		t.Fatalf("jsonFileTypeName = %q, want a full package path ending /internal/app.ChangeHaltInput", want)
	}
	if got := cmd.Annotations[jsonFileAnnotationType]; got != want {
		t.Errorf("type annotation = %q, want %q", got, want)
	}

	if err := cmd.Flags().Set("input", "-"); err != nil {
		t.Fatal(err)
	}
	cmd.SetIn(strings.NewReader(`{"report":"r"}`))
	got, err := decode(cmd)
	if err != nil || got.Report != "r" {
		t.Fatalf("decode = %+v, %v; want Report r", got, err)
	}

	cmd.SetIn(strings.NewReader(`{"report":"r","schema_version":1}`))
	if _, err := decode(cmd); err == nil || !strings.Contains(err.Error(), "--input") || !strings.Contains(err.Error(), "accepted keys: report") {
		t.Fatalf("unknown key: err = %v, want a --input refusal naming the accepted keys", err)
	}
}

// TestDeclareJSONFileKeepsCapabilityAnnotations proves the declaration adds to,
// never replaces, the capability annotations a leaf already carries.
func TestDeclareJSONFileKeepsCapabilityAnnotations(t *testing.T) {
	cmd := &cobra.Command{Use: "probe", Annotations: capability("probe.op", EffectRead)}
	declareJSONFile[app.ChangeHaltInput](cmd, "input")
	if cmd.Annotations[capAnnotationID] != "probe.op" || cmd.Annotations[capAnnotationEffects] != string(EffectRead) {
		t.Fatalf("capability annotations lost: %v", cmd.Annotations)
	}
}

// TestDeclareJSONFileRefusesASecondDeclaration proves one command declares at
// most one JSON file, so the recorded type can never be silently overwritten.
func TestDeclareJSONFileRefusesASecondDeclaration(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a second declareJSONFile on one command did not panic")
		}
	}()
	cmd := &cobra.Command{Use: "probe"}
	declareJSONFile[app.ChangeHaltInput](cmd, "input")
	declareJSONFile[app.FinalizeBlockInput](cmd, "request")
}
