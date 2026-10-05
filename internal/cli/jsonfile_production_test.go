package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/danielhanold/docket/internal/app"
)

// TestPublishedRequestIsTheDecodedJSONFile is the guard for the published
// request rule: an operation's `docket schema` request is exactly the JSON
// file type its command decodes (recorded by declareJSONFile), or absent when
// the command decodes none. It walks the real production tree, so the set of
// JSON-reading operations is derived, never listed. For each JSON-reading
// operation it also compares the decoder's accepted keys with the keys the
// schema publishes, so it checks the published surface, not only Go types.
func TestPublishedRequestIsTheDecodedJSONFile(t *testing.T) {
	bindings := map[string]app.OperationBinding{}
	for _, b := range app.OperationBindings() {
		bindings[b.ID] = b
	}
	leaves, readers := 0, 0
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, child := range c.Commands() {
			walk(child)
			typ, declares := child.Annotations[jsonFileAnnotationType]
			id, isOp := child.Annotations[capAnnotationID]
			if !isOp {
				if declares {
					t.Errorf("%q declares JSON file type %s but carries no capability id", child.CommandPath(), typ)
				}
				continue
			}
			leaves++
			b, bound := bindings[id]
			if !bound {
				if declares {
					t.Errorf("operation %q reads JSON file type %s but has no schema binding", id, typ)
				}
				continue
			}
			want := ""
			if b.Request != nil {
				want = jsonFileTypeName(reflect.TypeOf(b.Request))
			}
			switch {
			case typ == want:
			case !declares:
				t.Errorf("operation %q reads no JSON file, but docket schema publishes request %s; bind Request: nil", id, want)
				continue
			case want == "":
				t.Errorf("operation %q reads JSON file type %s, but docket schema publishes no request; bind that type", id, typ)
				continue
			default:
				t.Errorf("operation %q reads JSON file type %s, but docket schema publishes %s", id, typ, want)
				continue
			}
			if !declares {
				continue
			}
			readers++
			if flag := child.Annotations[jsonFileAnnotationFlag]; child.Flags().Lookup(flag) == nil {
				t.Errorf("operation %q declares JSON file flag --%s, which the command does not define", id, flag)
			}
			doc, ok, err := app.SchemaFor(id, []string{"read"})
			if err != nil || !ok || len(doc.Operations) != 1 || doc.Operations[0].Request == nil {
				t.Errorf("operation %q: docket schema publishes no request block (ok=%v err=%v)", id, ok, err)
				continue
			}
			var published []string
			for _, f := range doc.Operations[0].Request.Fields {
				published = append(published, f.Key)
			}
			sort.Strings(published)
			if got := app.RequestJSONKeys(b.Request); !reflect.DeepEqual(got, published) {
				t.Errorf("operation %q: the decoder accepts keys %v but docket schema publishes %v", id, got, published)
			}
		}
	}
	walk(productionRootForTest(t))
	t.Logf("%d operation leaves, %d read a JSON request file", leaves, readers)
	if leaves == 0 || readers == 0 {
		t.Fatalf("vacuous walk: %d operation leaves, %d JSON-file readers", leaves, readers)
	}
}

// sampleRequest builds a JSON object from exactly the published fields: a
// zero value per type word, the first member of an enum vocabulary when it has
// members, nested objects filled from their own published fields, and a
// one-element array for a repeated field.
func sampleRequest(fields []app.FieldDescriptor, vocab map[string]app.Vocabulary) map[string]any {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		var v any
		switch f.Type {
		case "int":
			v = 0
		case "bool":
			v = false
		case "object":
			v = sampleRequest(f.Fields, vocab)
		case "map[string]string":
			v = map[string]string{}
		default:
			v = ""
			if m := vocab[f.Enum].Members; f.Enum != "" && len(m) > 0 {
				v = m[0]
			}
		}
		if f.Repeated {
			v = []any{v}
		}
		out[f.Key] = v
	}
	return out
}

// TestPublishedRequestKeysAreAccepted proves, for every operation that
// publishes a request, that a file built only from the published keys is
// accepted by the strict decoder, and that adding the envelope key
// schema_version is refused with every accepted key named.
func TestPublishedRequestKeysAreAccepted(t *testing.T) {
	doc, err := app.Schema([]string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	bindings := map[string]app.OperationBinding{}
	for _, b := range app.OperationBindings() {
		bindings[b.ID] = b
	}
	checked := 0
	for _, op := range doc.Operations {
		if op.Request == nil {
			continue
		}
		checked++
		typ := reflect.TypeOf(bindings[op.ID].Request)
		sample := sampleRequest(op.Request.Fields, doc.Vocabularies)
		body, _ := json.Marshal(sample)
		if err := decodeRequest(bytes.NewReader(body), "--input", "-", reflect.New(typ).Interface()); err != nil {
			t.Errorf("operation %q: a file built from its published keys is refused: %v\n%s", op.ID, err, body)
		}
		if _, clash := sample["schema_version"]; clash {
			continue
		}
		sample["schema_version"] = 1
		body, _ = json.Marshal(sample)
		err := decodeRequest(bytes.NewReader(body), "--input", "-", reflect.New(typ).Interface())
		accepted := "accepted keys: " + strings.Join(app.RequestJSONKeys(reflect.New(typ).Interface()), ", ")
		if err == nil || !strings.Contains(err.Error(), `unknown field "schema_version"`) || !strings.Contains(err.Error(), accepted) {
			t.Errorf("operation %q: schema_version refusal = %v, want an unknown-field refusal naming %q", op.ID, err, accepted)
		}
	}
	if checked == 0 {
		t.Fatal("no operation publishes a request; the check is vacuous")
	}
}

// TestSchemaPublishesTheJSONFileOfEachFixedOperation pins, end to end through
// `docket schema --operation <id> --json`, the request each operation fixed
// under this rule publishes, and that a flag-only operation resolves with no
// request block (not an unknown-operation refusal).
func TestSchemaPublishesTheJSONFileOfEachFixedOperation(t *testing.T) {
	resolver := []string{"attempt", "change_id", "conflicted_paths", "disposition", "observed_base",
		"observed_head", "recommended_action", "resolver_reservation", "summary", "touched_paths"}
	cases := map[string][]string{
		"finalize.block":             {"remedy", "report"},
		"finalize.rebase-continue":   resolver,
		"finalize.rebase-abort":      resolver,
		"finalize.closeout":          {"late_findings", "verification_outcomes"},
		"finalize.retarget-children": {"children"},
		"change.halt":                {"report"},
		"pr.publish":                 {"body", "title"},
		"finalize.rebase":            nil,
		"change.claim":               nil,
		"finalize.merge":             nil,
	}
	type field struct {
		Key    string  `json:"key"`
		Fields []field `json:"fields"`
	}
	for id, want := range cases {
		out, errS, code := runCLI(t, "schema", "--operation", id, "--json")
		if code != 0 || errS != "" {
			t.Errorf("schema --operation %s: code=%d err=%q", id, code, errS)
			continue
		}
		var doc struct {
			Operations []struct {
				Request *struct {
					Fields []field `json:"fields"`
				} `json:"request"`
			} `json:"operations"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil || len(doc.Operations) != 1 {
			t.Errorf("schema --operation %s: %v\n%s", id, err, out)
			continue
		}
		req := doc.Operations[0].Request
		if want == nil {
			if req != nil {
				t.Errorf("flag-only operation %s publishes a request %v, want none", id, req.Fields)
			}
			continue
		}
		if req == nil {
			t.Errorf("operation %s publishes no request, want keys %v", id, want)
			continue
		}
		var got []string
		for _, f := range req.Fields {
			got = append(got, f.Key)
			if id == "finalize.retarget-children" && f.Key == "children" {
				var nested []string
				for _, n := range f.Fields {
					nested = append(nested, n.Key)
				}
				sort.Strings(nested)
				if !reflect.DeepEqual(nested, []string{"id", "pr_number", "pr_revision"}) {
					t.Errorf("children nested keys = %v, want [id pr_number pr_revision]", nested)
				}
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("operation %s publishes request keys %v, want %v", id, got, want)
		}
	}
}
