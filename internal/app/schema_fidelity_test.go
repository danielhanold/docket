package app

import (
	"reflect"
	"testing"
)

// walkEnumRefs collects every FieldDescriptor.Enum reference in a descriptor
// tree, descending into nested Fields — the whole-surface sweep operates on the
// EMITTED document, so it walks descriptors, not Go struct tags.
func walkEnumRefs(fields []FieldDescriptor, out map[string]bool) {
	for _, f := range fields {
		if f.Enum != "" {
			out[f.Enum] = true
		}
		walkEnumRefs(f.Fields, out)
	}
}

// TestSchemaResultFidelityRepresentativeOps checks that the emitted result
// descriptors for the representative ops match their *Result structs and the
// shared envelope, and that every enum reference across the WHOLE emitted
// document resolves to a published vocabulary key.
func TestSchemaResultFidelityRepresentativeOps(t *testing.T) {
	// (a) Envelope shape lists exactly the frozen v1 keys, in order. Derived
	// once from Envelope{} above; asserted literally here as the contract.
	one, ok, err := SchemaFor("change.reconcile", []string{"read"})
	if err != nil || !ok {
		t.Fatalf("SchemaFor(change.reconcile) = ok=%v err=%v", ok, err)
	}
	envKeys := descriptorKeys(one.EnvelopeShape)
	wantEnv := []string{"protocol_version", "operation", "result", "failure"}
	if !reflect.DeepEqual(envKeys, wantEnv) {
		t.Errorf("envelope shape keys = %v, want %v", envKeys, wantEnv)
	}

	// (b) change.reconcile result carries disposition tagged
	// enum=reconcile_dispositions.
	recResult := one.Operations[0].Result
	if one.Operations[0].ID != "change.reconcile" {
		t.Fatalf("SchemaFor(change.reconcile) op id = %q", one.Operations[0].ID)
	}
	disp := fieldByKey(t, recResult, "disposition")
	if disp.Enum != "reconcile_dispositions" {
		t.Errorf("reconcile disposition enum = %q, want reconcile_dispositions", disp.Enum)
	}

	// (b) change.create result carries id/slug/path/committed_revision and an
	// untagged findings. These fields carry NO presence tag on the struct
	// (ChangeCreateResult), so the emitted descriptor must report an empty
	// presence — asserted against the real struct, not the plan draft.
	createDoc, ok, err := SchemaFor("change.create", []string{"read"})
	if err != nil || !ok {
		t.Fatalf("SchemaFor(change.create) = ok=%v err=%v", ok, err)
	}
	createResult := createDoc.Operations[0].Result
	for _, key := range []string{"id", "slug", "path", "committed_revision", "findings"} {
		f := fieldByKey(t, createResult, key)
		if f.Presence != "" {
			t.Errorf("create result field %q presence = %q, want \"\" (struct carries no presence tag)", key, f.Presence)
		}
		if f.Enum != "" {
			t.Errorf("create result field %q enum = %q, want none", key, f.Enum)
		}
	}
	if fieldByKey(t, createResult, "committed_revision").Type != "string" {
		t.Errorf("create committed_revision type = %q, want string", fieldByKey(t, createResult, "committed_revision").Type)
	}
	if fieldByKey(t, createResult, "id").Type != "int" {
		t.Errorf("create id type = %q, want int", fieldByKey(t, createResult, "id").Type)
	}
	if !fieldByKey(t, createResult, "findings").Repeated {
		t.Errorf("create findings must be repeated")
	}

	// (c) Whole-surface pairing sweep on the EMITTED document: every enum
	// reference across the envelope shape and every op's request and result
	// (walked recursively) resolves to a published vocabulary key. This is
	// stronger than the per-binding check because it runs on the emitted doc.
	doc, err := Schema([]string{"read"})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	refs := map[string]bool{}
	walkEnumRefs(doc.EnvelopeShape.Fields, refs)
	for _, op := range doc.Operations {
		if op.Request != nil {
			walkEnumRefs(op.Request.Fields, refs)
		}
		walkEnumRefs(op.Result.Fields, refs)
	}
	if len(refs) == 0 {
		t.Fatal("the emitted document carries no enum references; the sweep is vacuous")
	}
	for name := range refs {
		if _, ok := doc.Vocabularies[name]; !ok {
			t.Errorf("emitted document references enum vocabulary %q, which the document does not publish", name)
		}
	}
}

// TestSchemaRequestAbsentForReadOnlyLeaves proves the read-only leaves that
// decode no JSON body carry no request block. status and capabilities are bound
// with a nil Request; schema is deliberately unbound (its own container is
// self-referential), so it has no entry at all.
func TestSchemaRequestAbsentForReadOnlyLeaves(t *testing.T) {
	for _, id := range []string{"status", "capabilities"} {
		doc, ok, err := SchemaFor(id, []string{"read"})
		if err != nil || !ok {
			t.Fatalf("SchemaFor(%s) = ok=%v err=%v", id, ok, err)
		}
		if doc.Operations[0].Request != nil {
			t.Errorf("read-only leaf %q carries a request block, want none", id)
		}
	}
	if _, ok, _ := SchemaFor("schema", []string{"read"}); ok {
		t.Error("SchemaFor(schema) ok = true, want false — the schema op is deliberately unbound")
	}
}
