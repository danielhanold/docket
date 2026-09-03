package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"testing"
)

// appPkgPath is the import path of package app, read from a known app type rather
// than hardcoded, so the reflection walk keys on the real package identity.
func appPkgPath(t *testing.T) string {
	t.Helper()
	p := reflect.TypeOf(Envelope{}).PkgPath()
	if p == "" {
		t.Fatal("could not resolve package app's import path from Envelope{}")
	}
	return p
}

// astRequestResultTypeNames parses every non-test .go file in package app and
// returns the names of every exported struct type whose name ends in "Request" or
// "Result". This is the population the registry must account for — it is derived
// from the source AST by shape, never an enumerated list, so a newly declared
// *Request/*Result type is discovered here automatically.
func astRequestResultTypeNames(t *testing.T) map[string]bool {
	t.Helper()
	dir := appPackageDir(t)
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !hasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	out := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); !ok {
						continue
					}
					name := ts.Name.Name
					if !ts.Name.IsExported() {
						continue
					}
					if hasSuffix(name, "Request") || hasSuffix(name, "Result") {
						out[name] = true
					}
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("astRequestResultTypeNames found no *Request/*Result types; the AST walk is broken")
	}
	return out
}

// collectReachableAppStructs records the name of every package-app struct type
// reachable from t by reflection: t itself, its fields, and the elements of any
// pointer/slice/array/map it carries. A cross-package struct (e.g.
// gatedrive.DriveDoc under GateDriveResult.Drive) is walked for nesting but never
// recorded, because the AST join is over package-app type names only.
func collectReachableAppStructs(t reflect.Type, appPkg string, seen map[reflect.Type]bool, out map[string]bool) {
	if t == nil {
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if seen[t] {
		return
	}
	seen[t] = true
	switch t.Kind() {
	case reflect.Struct:
		if t.PkgPath() == appPkg && t.Name() != "" {
			out[t.Name()] = true
		}
		for i := 0; i < t.NumField(); i++ {
			collectReachableAppStructs(t.Field(i).Type, appPkg, seen, out)
		}
	case reflect.Slice, reflect.Array:
		collectReachableAppStructs(t.Elem(), appPkg, seen, out)
	case reflect.Map:
		collectReachableAppStructs(t.Elem(), appPkg, seen, out)
	}
}

// excludedRequestTypes and excludedResultTypes are the *Request/*Result types the
// registry deliberately does not account for, each with a stated reason. Every
// exclusion is checked to be a REAL AST type and to be genuinely unreachable, so a
// stale exclusion (its type deleted, or the type later wired into an op) reddens
// here rather than silently hiding a real gap.
var excludedRequestTypes = map[string]string{
	"LocalGateRequest": "internal finalize gate seam input (RunLocalGate); no catalog op decodes or assembles it",
}

var excludedResultTypes = map[string]string{
	"CLIErrorResult":              "pre-dispatch cli parse/usage failure result (CLIError); not tied to a catalog operation id",
	"LocalGateResult":             "internal finalize gate seam return (RunLocalGate); carries no Envelope, is not an op document",
	"SweepPRSetResult":            "internal batched-PR-read seam return (ProbePRSet); carries no Envelope, is not an op document",
	"RunGateVerdictObserveResult": "run.gate-verdict's observe-mode (--unattributed) result variant; the op binds its attributed result RunGateVerdictResult",
	"SchemaResult":                "the schema document's own envelope (this registry's emitted container); its schema op is wired in a later task",
}

// TestEveryRequestAndResultStructIsBound is the two-direction registry-accounting
// guard. Forward: every exported *Request/*Result struct in package app is
// reachable from OperationBindings() — bound directly or nested inside a bound
// prototype (walked by reflection) — or is in a documented exclusion set. A new
// op's structs without a binding redden here. Reverse: every binding's prototypes
// ARE package-app types named *Request/*Result (and every Result embeds Envelope),
// so a binding pointing at a helper struct reddens. Counts are logged so a gross
// population collapse is visible.
func TestEveryRequestAndResultStructIsBound(t *testing.T) {
	appPkg := appPkgPath(t)
	bindings := OperationBindings()

	// Reachable set: reflect every bound prototype and collect app struct names.
	reachable := map[string]bool{}
	seen := map[reflect.Type]bool{}
	for _, b := range bindings {
		if b.Request != nil {
			collectReachableAppStructs(reflect.TypeOf(b.Request), appPkg, seen, reachable)
		}
		collectReachableAppStructs(reflect.TypeOf(b.Result), appPkg, seen, reachable)
	}

	astNames := astRequestResultTypeNames(t)
	t.Logf("accounting: %d bindings, %d AST *Request/*Result types, %d reachable app structs, %d excluded",
		len(bindings), len(astNames), len(reachable), len(excludedRequestTypes)+len(excludedResultTypes))

	// Forward: every AST *Request/*Result type is reachable or excluded.
	for name := range astNames {
		if reachable[name] {
			continue
		}
		_, reqExcluded := excludedRequestTypes[name]
		_, resExcluded := excludedResultTypes[name]
		if reqExcluded || resExcluded {
			continue
		}
		t.Errorf("type %q is an exported *Request/*Result but is neither bound, nested in a bound prototype, nor a documented exclusion", name)
	}

	// Exclusions stay honest: each names a real AST type and is genuinely
	// unreachable (a type later wired into a binding must drop out of the set).
	for name := range excludedRequestTypes {
		if !astNames[name] {
			t.Errorf("excludedRequestTypes names %q, which is not an exported *Request/*Result type in package app — stale exclusion", name)
		}
		if reachable[name] {
			t.Errorf("excludedRequestTypes names %q, but it IS reachable from a binding — remove the exclusion", name)
		}
	}
	for name := range excludedResultTypes {
		if !astNames[name] {
			t.Errorf("excludedResultTypes names %q, which is not an exported *Request/*Result type in package app — stale exclusion", name)
		}
		if reachable[name] {
			t.Errorf("excludedResultTypes names %q, but it IS reachable from a binding — remove the exclusion", name)
		}
	}

	// Reverse: every binding's prototypes are package-app *Request/*Result types,
	// and every Result embeds Envelope.
	for _, b := range bindings {
		if b.Request != nil {
			rt := reflect.TypeOf(b.Request)
			if rt.Kind() != reflect.Struct || rt.PkgPath() != appPkg || !hasSuffix(rt.Name(), "Request") {
				t.Errorf("binding %q Request is %v, want a package-app *Request struct", b.ID, rt)
			}
		}
		rt := reflect.TypeOf(b.Result)
		if rt.Kind() != reflect.Struct || rt.PkgPath() != appPkg || !hasSuffix(rt.Name(), "Result") {
			t.Errorf("binding %q Result is %v, want a package-app *Result struct", b.ID, rt)
		}
		if !embedsEnvelope(rt) {
			t.Errorf("binding %q Result %v does not embed Envelope", b.ID, rt)
		}
	}
}

// embedsEnvelope reports whether t has Envelope as an anonymous (embedded) field.
func embedsEnvelope(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Name == "Envelope" {
			return true
		}
	}
	return false
}

// TestOperationBindingsSortedUniqueAndDescribable holds the registry's structural
// invariants: ids are sorted and unique; reflectDescriptor succeeds on every bound
// prototype (an undescribable shape from a bound result surfaces HERE); every
// result embeds Envelope; every docket:"enum=<name>" reference across all bound
// prototypes names a vocabulary SchemaVocabularies emits; and, conversely, every
// disposition-family vocabulary is referenced by at least one bound field so an
// orphaned family reddens.
func TestOperationBindingsSortedUniqueAndDescribable(t *testing.T) {
	bindings := OperationBindings()
	if len(bindings) == 0 {
		t.Fatal("OperationBindings() is empty")
	}

	// Sorted + unique ids.
	seen := map[string]bool{}
	for i, b := range bindings {
		if seen[b.ID] {
			t.Errorf("duplicate binding id %q", b.ID)
		}
		seen[b.ID] = true
		if i > 0 && b.ID < bindings[i-1].ID {
			t.Errorf("bindings not sorted: %q precedes %q", bindings[i-1].ID, b.ID)
		}
	}

	vocab := SchemaVocabularies([]string{"read"})
	referenced := map[string]bool{}

	for _, b := range bindings {
		if b.Request != nil {
			d, err := reflectDescriptor(b.Request)
			if err != nil {
				t.Errorf("binding %q Request %T is undescribable: %v", b.ID, b.Request, err)
			} else {
				collectEnumRefs(d.Fields, referenced)
			}
		}
		d, err := reflectDescriptor(b.Result)
		if err != nil {
			t.Errorf("binding %q Result %T is undescribable: %v", b.ID, b.Result, err)
			continue
		}
		collectEnumRefs(d.Fields, referenced)
		if !embedsEnvelope(reflect.TypeOf(b.Result)) {
			t.Errorf("binding %q Result %T does not embed Envelope", b.ID, b.Result)
		}
	}

	// Forward pairing: every referenced enum resolves to an emitted vocabulary.
	for name := range referenced {
		if _, ok := vocab[name]; !ok {
			t.Errorf("a bound field references enum vocabulary %q, which SchemaVocabularies does not emit", name)
		}
	}

	// Converse pairing: every disposition-family vocabulary is referenced by at
	// least one bound field. A family emitted but referenced by nothing is an
	// orphan. The non-family (core) vocabularies are always emitted regardless of
	// bindings, so they are exempt — keyed on the "_dispositions" name shape, never
	// an enumerated core list.
	for name, v := range vocab {
		if !hasSuffix(name, "_dispositions") {
			continue
		}
		if len(v.Members) == 0 {
			t.Errorf("disposition vocabulary %q has no members", name)
		}
		if !referenced[name] {
			t.Errorf("disposition vocabulary %q is emitted but referenced by no bound field — an orphan family", name)
		}
	}
}

// collectEnumRefs records every FieldDescriptor.Enum in fields and their nested
// Fields.
func collectEnumRefs(fields []FieldDescriptor, out map[string]bool) {
	for _, f := range fields {
		if f.Enum != "" {
			out[f.Enum] = true
		}
		collectEnumRefs(f.Fields, out)
	}
}

// TestSchemaExcludesEnvelopeKeysPerOp proves the per-op result descriptors drop
// the envelope's own keys (computed from Envelope{}, not hand-listed) while the
// envelope shape is emitted once, and that SchemaFor filters to a single id and
// fails closed on an unknown one.
func TestSchemaExcludesEnvelopeKeysPerOp(t *testing.T) {
	doc, err := Schema([]string{"read"})
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if doc.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", doc.SchemaVersion, SchemaVersion)
	}
	if len(doc.Operations) != len(OperationBindings()) {
		t.Errorf("Schema emitted %d operations, want %d bindings", len(doc.Operations), len(OperationBindings()))
	}

	envKeys := map[string]bool{}
	for _, f := range doc.EnvelopeShape.Fields {
		envKeys[f.Key] = true
	}
	if !envKeys["protocol_version"] || !envKeys["operation"] || !envKeys["result"] {
		t.Fatalf("envelope shape is missing a core key: %v", descriptorKeys(doc.EnvelopeShape))
	}
	for _, op := range doc.Operations {
		for _, f := range op.Result.Fields {
			if envKeys[f.Key] {
				t.Errorf("operation %q result restates envelope key %q", op.ID, f.Key)
			}
		}
	}

	// SchemaFor: known id filters to exactly that op; unknown id fails closed.
	one, ok, err := SchemaFor("change.block", []string{"read"})
	if err != nil || !ok {
		t.Fatalf("SchemaFor(change.block) = ok=%v err=%v", ok, err)
	}
	if len(one.Operations) != 1 || one.Operations[0].ID != "change.block" {
		t.Errorf("SchemaFor(change.block) operations = %v, want exactly change.block", one.Operations)
	}
	if _, ok, _ := SchemaFor("no.such.operation", []string{"read"}); ok {
		t.Error("SchemaFor(no.such.operation) ok = true, want false for an unknown id")
	}
}
