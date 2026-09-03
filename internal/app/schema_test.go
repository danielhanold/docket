package app

import (
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// descriptorKeys returns the top-level field keys of a descriptor in order.
func descriptorKeys(d TypeDescriptor) []string {
	keys := make([]string, 0, len(d.Fields))
	for _, f := range d.Fields {
		keys = append(keys, f.Key)
	}
	return keys
}

// fieldByKey returns the field with the given key, failing the test if absent.
func fieldByKey(t *testing.T, d TypeDescriptor, key string) FieldDescriptor {
	t.Helper()
	for _, f := range d.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("no field %q in %v", key, descriptorKeys(d))
	return FieldDescriptor{}
}

// mustReflect reflects a prototype, failing the test on the fail-closed error.
func mustReflect(t *testing.T, prototype any) TypeDescriptor {
	t.Helper()
	d, err := reflectDescriptor(prototype)
	if err != nil {
		t.Fatalf("reflectDescriptor(%T) = error %v", prototype, err)
	}
	return d
}

func TestReflectDescriptorChangeReconcileRequest(t *testing.T) {
	d := mustReflect(t, ChangeReconcileRequest{})
	keys := descriptorKeys(d)
	want := []string{"id", "version", "sections", "spec_sections", "relations", "reconcile_log_entry"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}

	rel := fieldByKey(t, d, "relations")
	if rel.Type != "object" {
		t.Errorf("relations type = %q, want object", rel.Type)
	}
	if rel.Repeated {
		t.Errorf("relations must not be repeated (it is a *DesiredRelations)")
	}
	dep := fieldByKey(t, TypeDescriptor{Fields: rel.Fields}, "depends_on")
	if !dep.Repeated || dep.Type != "int" {
		t.Errorf("relations.depends_on = %+v, want repeated int", dep)
	}
	so := fieldByKey(t, TypeDescriptor{Fields: rel.Fields}, "stacked_on")
	if so.Repeated || so.Type != "int" {
		t.Errorf("relations.stacked_on = %+v, want non-repeated int (*int element)", so)
	}

	id := fieldByKey(t, d, "id")
	if !id.Required {
		t.Errorf("id must be required (docket tag)")
	}
	if id.Type != "int" {
		t.Errorf("id type = %q, want int", id.Type)
	}
	if fieldByKey(t, d, "sections").Type != "map[string]string" {
		t.Errorf("sections type = %q, want map[string]string", fieldByKey(t, d, "sections").Type)
	}
	if fieldByKey(t, d, "spec_sections").Type != "map[string]string" {
		t.Errorf("spec_sections type = %q, want map[string]string", fieldByKey(t, d, "spec_sections").Type)
	}
	if !fieldByKey(t, d, "reconcile_log_entry").Required {
		t.Errorf("reconcile_log_entry must be required (docket tag)")
	}
}

func TestReflectDescriptorChangeGroomRequest(t *testing.T) {
	d := mustReflect(t, ChangeGroomRequest{})

	if fieldByKey(t, d, "change_id").Key != "change_id" {
		t.Fatal("groom's id key is change_id, not id")
	}
	if !fieldByKey(t, d, "change_id").Required {
		t.Errorf("change_id must be required (docket tag)")
	}

	// spec_markdown optional (omitempty + not required).
	sm := fieldByKey(t, d, "spec_markdown")
	if sm.Required || sm.Type != "string" {
		t.Errorf("spec_markdown = %+v, want optional string", sm)
	}

	// sections repeated object with heading/intent/markdown.
	sec := fieldByKey(t, d, "sections")
	if !sec.Repeated || sec.Type != "object" {
		t.Errorf("sections = %+v, want repeated object", sec)
	}
	secKeys := descriptorKeys(TypeDescriptor{Fields: sec.Fields})
	wantSec := []string{"heading", "intent", "markdown"}
	if !reflect.DeepEqual(secKeys, wantSec) {
		t.Errorf("sections element keys = %v, want %v", secKeys, wantSec)
	}

	// stacked_on a non-repeated optional int (*int).
	so := fieldByKey(t, d, "stacked_on")
	if so.Repeated || so.Type != "int" || so.Required {
		t.Errorf("stacked_on = %+v, want optional scalar int", so)
	}

	// outcome is a named string type (GroomOutcome) — still described as string.
	if fieldByKey(t, d, "outcome").Type != "string" {
		t.Errorf("outcome type = %q, want string", fieldByKey(t, d, "outcome").Type)
	}
}

func TestReflectDescriptorChangeCreateRequest(t *testing.T) {
	d := mustReflect(t, ChangeCreateRequest{})

	keys := descriptorKeys(d)
	want := []string{
		"request_id", "title", "type", "priority",
		"why", "what_changes", "out_of_scope",
		"depends_on", "stacked_on", "related", "discovered_from", "adrs",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}

	// The five relation collections are repeated int, except stacked_on (*int).
	for _, k := range []string{"depends_on", "related", "discovered_from", "adrs"} {
		f := fieldByKey(t, d, k)
		if !f.Repeated || f.Type != "int" {
			t.Errorf("%s = %+v, want repeated int", k, f)
		}
	}
	so := fieldByKey(t, d, "stacked_on")
	if so.Repeated || so.Type != "int" {
		t.Errorf("stacked_on = %+v, want non-repeated int", so)
	}

	// Required set per Task 4's tags.
	wantRequired := map[string]bool{
		"request_id": true, "title": true,
		"why": true, "what_changes": true, "out_of_scope": true,
	}
	for _, f := range d.Fields {
		if f.Required != wantRequired[f.Key] {
			t.Errorf("%s required = %v, want %v", f.Key, f.Required, wantRequired[f.Key])
		}
	}
}

// TestReflectDescriptorFailsClosedOnUndescribableMap proves the generator fails
// closed rather than mis-describing a shape it has no type word for. Task 7's
// registry accounting test relies on this error path.
func TestReflectDescriptorFailsClosedOnUndescribableMap(t *testing.T) {
	type badMap struct {
		Counts map[string]int `json:"counts"`
	}
	if _, err := reflectDescriptor(badMap{}); err == nil {
		t.Fatal("reflectDescriptor(badMap{}) = nil error, want an undescribable-shape error")
	}
}

// TestReflectDescriptorRejectsNonStruct proves a non-struct prototype fails
// closed rather than producing an empty descriptor.
func TestReflectDescriptorRejectsNonStruct(t *testing.T) {
	if _, err := reflectDescriptor(42); err == nil {
		t.Fatal("reflectDescriptor(42) = nil error, want a non-struct error")
	}
}

// TestReflectDescriptorByteSliceIsString proves a []byte field is described as a
// single JSON string (its base64 wire form), not a repeated int — the shape the
// EvidenceRecord/Source/Patch byte fields on several bound prototypes carry
// (change 0399, Task 7).
func TestReflectDescriptorByteSliceIsString(t *testing.T) {
	type wrap struct {
		Blob []byte `json:"blob"`
	}
	d := mustReflect(t, wrap{})
	b := fieldByKey(t, d, "blob")
	if b.Type != "string" || b.Repeated {
		t.Errorf("[]byte field = %+v, want a non-repeated string", b)
	}
}

// TestReflectDescriptorForeignStructIsOpaque proves a struct from another package
// is described as an opaque object — the generator recurses into docket's own
// request/result types but represents a foreign embedded type (config.Effective,
// gatedrive.DriveDoc) as an opaque object rather than descending into its
// internals. This is what keeps diagnostic.config's ConfigInspectionResult (which
// embeds the whole config.Effective tree, maps-of-structs and all) describable
// without loosening the map-shape fail-closed contract app types still rely on
// (change 0399, Task 7).
func TestReflectDescriptorForeignStructIsOpaque(t *testing.T) {
	type wrap struct {
		Sort config.BoardSort `json:"sort"`
	}
	d := mustReflect(t, wrap{})
	sf := fieldByKey(t, d, "sort")
	if sf.Type != "object" {
		t.Errorf("foreign struct type = %q, want object", sf.Type)
	}
	if len(sf.Fields) != 0 {
		t.Errorf("foreign struct must be opaque (no descended fields), got %v", descriptorKeys(TypeDescriptor{Fields: sf.Fields}))
	}
}
