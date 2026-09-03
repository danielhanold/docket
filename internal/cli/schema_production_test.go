package cli

// This file pins the CATALOG-to-SCHEMA correspondence against the real production
// Cobra tree: every catalog operation that emits a protocol-v1 document has a
// schema binding, every binding resolves to a catalog entry, and the counts agree.
// It mirrors capability_production_test.go's two-independent-directions structure
// so neither direction can hide the other's drift, joining the two surfaces on the
// stable operation id.

import (
	"testing"

	"github.com/danielhanold/docket/internal/app"
)

// noSchemaDocumentOps names catalog operations that emit NO protocol-v1 document
// and therefore carry no schema binding. `development.test` streams the suite
// report and returns only an exit code (runDevelopmentTest), so there is no
// Envelope-embedding result to describe. Naming it here, in the open, keeps the
// join exact — a second no-document op is a conscious addition, never a silent
// gap that a bare count would swallow.
var noSchemaDocumentOps = map[string]bool{
	"development.test": true,
}

// TestSchemaCatalogCorrespondence joins the capability catalog and the schema
// registry on id, both ways: every catalog entry (bar a documented no-document op)
// has a binding, every binding resolves to a catalog entry, and the counts equal.
// Mirrors TestProductionCapabilityCorrespondence.
func TestSchemaCatalogCorrespondence(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	catalog := map[string]bool{}
	for _, e := range entries {
		catalog[e.ID] = true
	}

	bindings := app.OperationBindings()
	bound := map[string]bool{}
	// Forward: every binding resolves to a real catalog entry.
	for _, b := range bindings {
		if !catalog[b.ID] {
			t.Errorf("schema binding %q has no catalog entry", b.ID)
		}
		bound[b.ID] = true
	}

	// Reverse: every catalog entry except a documented no-document op is bound.
	for _, e := range entries {
		if bound[e.ID] || noSchemaDocumentOps[e.ID] {
			continue
		}
		t.Errorf("catalog entry %q has no schema binding (and is not a documented no-document op)", e.ID)
	}

	// The named exceptions must be real catalog entries, so a stale name reddens.
	for id := range noSchemaDocumentOps {
		if !catalog[id] {
			t.Errorf("noSchemaDocumentOps names %q, which is not a catalog entry — stale exception", id)
		}
	}

	// Count closes the mirror: a new leaf without a binding, or a stale binding,
	// breaks the equality even if the per-entry loops somehow miss it.
	if len(bindings)+len(noSchemaDocumentOps) != len(entries) {
		t.Errorf("bindings %d + no-document ops %d != catalog entries %d — a new leaf without a schema binding, or a stale binding",
			len(bindings), len(noSchemaDocumentOps), len(entries))
	}
}
