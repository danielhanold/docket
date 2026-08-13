package document

import "testing"

func TestDecodeFrontmatterIntoCallerStruct(t *testing.T) {
	src := []byte("---\nid: 306\nslug: loss-preserving-document-layer\ntrivial: false\ndepends_on: [304]\n---\nbody\n")
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ID        int    `yaml:"id"`
		Slug      string `yaml:"slug"`
		Trivial   bool   `yaml:"trivial"`
		DependsOn []int  `yaml:"depends_on"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 306 || got.Slug != "loss-preserving-document-layer" || got.Trivial || len(got.DependsOn) != 1 || got.DependsOn[0] != 304 {
		t.Fatalf("decoded %+v", got)
	}
}

func TestUnknownFieldsAreNotRejected(t *testing.T) {
	src := []byte("---\nid: 1\nfuture_key: kept\n---\n")
	d, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		ID int `yaml:"id"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatalf("unknown fields are compatibility data, not errors: %v", err)
	}
}

func TestDuplicateMappingKeysRejected(t *testing.T) {
	_, err := Parse([]byte("---\nid: 1\nid: 2\n---\n"))
	if !IsKind(err, KindDuplicateField) {
		t.Fatalf("want duplicate-field, got %v", err)
	}
}

func TestMultipleYAMLDocumentsRejected(t *testing.T) {
	// The fence scanner closes the frontmatter at the LAST "---" line only when
	// it is exactly "---"; "--- extra: doc" is not, so the interior reaches the
	// YAML stage and the second decode is what rejects it. Kind pinned to
	// invalid-yaml, the classification this implementation settles on.
	_, err := Parse([]byte("---\nid: 1\n--- extra: doc\n---\n"))
	if err == nil {
		t.Fatal("want an error for a second YAML document")
	}
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("want invalid-yaml, got %v", err)
	}
}

func TestNonMappingFrontmatterRejected(t *testing.T) {
	_, err := Parse([]byte("---\n- just\n- a list\n---\n"))
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("want invalid-yaml, got %v", err)
	}
}

func TestUnresolvedAliasRejected(t *testing.T) {
	_, err := Parse([]byte("---\nid: *nowhere\n---\n"))
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("want invalid-yaml, got %v", err)
	}
}

func TestEmptyFrontmatterBlockIsValid(t *testing.T) {
	d, err := Parse([]byte("---\n---\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !d.HasFrontmatter() {
		t.Fatal("an empty frontmatter block is still frontmatter")
	}
	var got struct{}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatalf("decoding empty frontmatter into an empty struct: %v", err)
	}
}
