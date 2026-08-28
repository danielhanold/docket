package reposetup

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/danielhanold/docket/internal/config"
)

// byteCase is a success case whose output is asserted byte-for-byte: every byte
// outside the removed entry's own line(s) is preserved exactly.
type byteCase struct {
	name string
	in   string
	want string
}

var removeByteCases = []byteCase{
	{
		name: "present-first",
		in:   "metadata_branch: docket\nintegration_branch: main\n",
		want: "integration_branch: main\n",
	},
	{
		name: "present-middle",
		in:   "integration_branch: main\nmetadata_branch: docket\nchanges_dir: docs/changes\n",
		want: "integration_branch: main\nchanges_dir: docs/changes\n",
	},
	{
		name: "present-last",
		in:   "integration_branch: main\nmetadata_branch: docket\n",
		want: "integration_branch: main\n",
	},
	{
		name: "present-last-no-final-newline",
		in:   "integration_branch: main\nmetadata_branch: docket",
		want: "integration_branch: main\n",
	},
	{
		name: "trailing-comment-on-key-line",
		in:   "metadata_branch: docket   # legacy key, remove me\nintegration_branch: main\n",
		want: "integration_branch: main\n",
	},
	{
		name: "crlf-preserved-elsewhere",
		in:   "metadata_branch: docket\r\nintegration_branch: main\r\n",
		want: "integration_branch: main\r\n",
	},
	{
		// The key's own head comment and the next key's head comment are both
		// preserved: only metadata_branch's key line is removed.
		name: "comments-and-unknown-keys-byte-identical",
		in:   "# top comment\nmetadata_branch: docket\n# standalone comment\nintegration_branch: main   # inline\nunknown_setting: whatever\n\n# trailing block\n",
		want: "# top comment\n# standalone comment\nintegration_branch: main   # inline\nunknown_setting: whatever\n\n# trailing block\n",
	},
	{
		name: "block-mapping-value-continuation-lines-removed",
		in:   "metadata_branch:\n  weird: nested\n  more: values\nintegration_branch: main\n",
		want: "integration_branch: main\n",
	},
}

func TestRemoveMetadataBranchKeyByteExact(t *testing.T) {
	for _, tc := range removeByteCases {
		t.Run(tc.name, func(t *testing.T) {
			out, removed, err := RemoveMetadataBranchKey([]byte(tc.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !removed {
				t.Fatalf("expected removed=true")
			}
			if !bytes.Equal(out, []byte(tc.want)) {
				t.Fatalf("byte mismatch\n got: %q\nwant: %q", out, tc.want)
			}
		})
	}
}

func TestRemoveMetadataBranchKeyDoesNotMutateInput(t *testing.T) {
	in := []byte("metadata_branch: docket\nintegration_branch: main\n")
	orig := append([]byte(nil), in...)
	if _, _, err := RemoveMetadataBranchKey(in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(in, orig) {
		t.Fatalf("input slice was mutated: got %q want %q", in, orig)
	}
}

func TestRemoveMetadataBranchKeyAbsent(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"plain-absent", "integration_branch: main\nchanges_dir: docs/changes\n"},
		{"nested-under-other-map", "finalize:\n  metadata_branch: docket\nintegration_branch: main\n"},
		{"empty", ""},
		{"comments-only", "# just a comment\n# another\n"},
		{"substring-key-not-matched", "metadata_branch_override: docket\nintegration_branch: main\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, removed, err := RemoveMetadataBranchKey([]byte(tc.in))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if removed {
				t.Fatalf("expected removed=false")
			}
			if out != nil {
				t.Fatalf("expected nil out, got %q", out)
			}
		})
	}
}

func TestRemoveMetadataBranchKeyErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"duplicate-top-level-key", "metadata_branch: a\nmetadata_branch: b\n"},
		{"undecodable-yaml", "foo: [1, 2, 3\n"},
		{"root-not-a-mapping", "- a\n- b\n"},
		{"root-scalar", "just-a-scalar\n"},
		{"flow-mapping-root-shares-line", "{metadata_branch: docket, integration_branch: main}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, removed, err := RemoveMetadataBranchKey([]byte(tc.in))
			if err == nil {
				t.Fatalf("expected error, got out=%q removed=%v", out, removed)
			}
			if out != nil || removed {
				t.Fatalf("error return must be (nil,false,err); got out=%q removed=%v", out, removed)
			}
		})
	}
}

// roundTripCase is a successful edit over config-valid bytes: the output must
// still parse through internal/config's own loader entry (Resolve), with the
// key gone and every other decoded setting equal (learning
// validator-must-match-the-reader-it-feeds).
var roundTripCases = []byteCase{
	{
		// metadata_branch value must be one of the schema's accepted branch
		// names; "main" is a valid, explicit, non-default value (default is
		// "docket"), so removal flips it back to the non-explicit default.
		name: "first",
		in:   "metadata_branch: main\nintegration_branch: main\nchanges_dir: docs/changes\n",
	},
	{
		name: "last-with-comments",
		in:   "# docket config\nintegration_branch: main\nchanges_dir: docs/changes\nmetadata_branch: main\n",
	},
}

func TestRemoveMetadataBranchKeyRoundTripThroughConfigLoader(t *testing.T) {
	for _, tc := range roundTripCases {
		t.Run(tc.name, func(t *testing.T) {
			before := resolveOrFatal(t, []byte(tc.in))
			if !before.Effective.MetadataBranch.Explicit {
				t.Fatalf("fixture precondition: metadata_branch must be explicit before removal")
			}

			out, removed, err := RemoveMetadataBranchKey([]byte(tc.in))
			if err != nil || !removed {
				t.Fatalf("removal failed: removed=%v err=%v", removed, err)
			}

			after := resolveOrFatal(t, out)
			if after.Effective.MetadataBranch.Explicit {
				t.Fatalf("metadata_branch still explicit after removal: %+v", after.Effective.MetadataBranch)
			}

			// Every other decoded setting must be equal. Mask the metadata_branch
			// leaf (the one field intentionally changed) and zero all provenance
			// positions before comparing: a leaf's source LINE necessarily shifts
			// when an earlier line is removed, which is a correct consequence of
			// the edit, not a changed setting.
			b := normalizeEffective(before.Effective)
			a := normalizeEffective(after.Effective)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("a decoded setting other than metadata_branch changed\nbefore: %+v\nafter:  %+v", b, a)
			}
		})
	}
}

var provType = reflect.TypeOf(config.Provenance{})

// normalizeEffective masks the metadata_branch leaf (intentionally changed by
// the edit) and zeroes every Provenance in the struct so the comparison reflects
// decoded VALUES only, not source positions that a line removal necessarily
// shifts.
func normalizeEffective(e config.Effective) config.Effective {
	e.MetadataBranch = config.Value[string]{}
	zeroProvenance(reflect.ValueOf(&e).Elem())
	return e
}

func zeroProvenance(v reflect.Value) {
	if v.Type() == provType {
		v.Set(reflect.Zero(provType))
		return
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).CanSet() {
				zeroProvenance(v.Field(i))
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			mv := reflect.New(v.MapIndex(k).Type()).Elem()
			mv.Set(v.MapIndex(k))
			zeroProvenance(mv)
			v.SetMapIndex(k, mv)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			zeroProvenance(v.Index(i))
		}
	case reflect.Ptr, reflect.Interface:
		if !v.IsNil() {
			zeroProvenance(v.Elem())
		}
	}
}

func resolveOrFatal(t *testing.T, data []byte) *config.Snapshot {
	t.Helper()
	src := config.Source{Layer: config.LayerRepository, Name: ".docket.yml", Data: data}
	snap, _, err := config.Resolve([]config.Source{src}, config.ResolveContext{DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("config.Resolve rejected the bytes: %v", err)
	}
	if snap == nil {
		t.Fatalf("config.Resolve returned nil snapshot")
	}
	return snap
}
