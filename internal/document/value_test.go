package document

import "testing"

func TestSerializeEveryKind(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{Null(), ""},
		{String("plain"), "'plain'"},
		{String("it's"), "'it''s'"},
		{String("a: b # c"), "'a: b # c'"}, // colon-space and hash are inert inside quotes
		{String(""), "''"},
		{Int(0), "0"},
		{Int(-42), "-42"},
		{Bool(true), "true"},
		{Bool(false), "false"},
		{Seq(), "[]"},
		{Seq(Int(3), Int(7)), "[3, 7]"},
		// Null renders as the explicit keyword inside a sequence: yaml v3 v3.0.4
		// reads the plan's provisional "['a', true, ]" as a TWO-element
		// sequence, dropping the nil tail. See the comment beside serialize.
		{Seq(String("a"), Bool(true), Null()), "['a', true, null]"},
	}
	for _, c := range cases {
		if err := c.v.validate(); err != nil {
			t.Fatalf("validate(%v): %v", c.v, err)
		}
		if got := c.v.serialize(); got != c.want {
			t.Errorf("serialize = %q, want %q", got, c.want)
		}
	}
}

func TestStringValueRejectsControlCharacters(t *testing.T) {
	for _, bad := range []string{"nul\x00", "bell\x07", "line\nbreak", "cr\r"} {
		if err := String(bad).validate(); !IsKind(err, KindInvalidValue) {
			t.Errorf("String(%q).validate() = %v, want invalid-value", bad, err)
		}
	}
	if err := String("tab\tok").validate(); err != nil {
		t.Errorf("tab is allowed in strings: %v", err)
	}
}

// TestValueRejectsEveryRuneYAMLRefuses pins the closed-model rule to the reader
// in go.yaml.in/yaml/v3 v3.0.4, whose allowed set is
// "#x9 | #xA | #xD | [#x20-#x7E] | #x85 | [#xA0-#xD7FF] | [#xE000-#xFFFD] |
// [#x10000-#x10FFFF]". The C1 block (U+0080-U+009F) and the U+FFFE/U+FFFF
// non-characters sit outside it while being neither C0 controls nor DEL, so a
// C0-only guard lets the builder render a document its own reparse refuses.
// Found by FuzzValueRoundTrip; its minimized seed
// testdata/fuzz/FuzzValueRoundTrip/bfdd6f36d8108211 is the permanent corpus
// entry for the U+0098 case.
func TestValueRejectsEveryRuneYAMLRefuses(t *testing.T) {
	// U+0080 and U+009F bound the C1 block; U+0098 is the fuzz seed's rune;
	// U+0085 (NEL) is legal YAML but a control character, which the closed
	// model refuses regardless. U+FFFE/U+FFFF are the non-characters.
	for _, r := range []rune{0x80, 0x85, 0x98, 0x9f, 0xfffe, 0xffff} {
		bad := "x" + string(r) + "y"
		if err := String(bad).validate(); !IsKind(err, KindInvalidValue) {
			t.Errorf("String(%q).validate() = %v, want invalid-value", bad, err)
		}
		if err := validBlockContent(bad); !IsKind(err, KindInvalidValue) {
			t.Errorf("validBlockContent(%q) = %v, want invalid-value", bad, err)
		}
	}
	// The neighbours of each rejected range stay legal: U+007E, U+00A0,
	// U+FFFD, and the U+1FFFE non-character above the BMP.
	for _, r := range []rune{0x7e, 0xa0, 0xfffd, 0x1fffe} {
		ok := "x" + string(r) + "y"
		if err := String(ok).validate(); err != nil {
			t.Errorf("String(%q).validate() = %v, want nil", ok, err)
		}
		if err := validBlockContent(ok); err != nil {
			t.Errorf("validBlockContent(%q) = %v, want nil", ok, err)
		}
	}
}

func TestStringValueRejectsInvalidUTF8(t *testing.T) {
	if err := String(string([]byte{0xff})).validate(); !IsKind(err, KindInvalidValue) {
		t.Errorf("got %v", err)
	}
}

func TestSeqRejectsNestedSeq(t *testing.T) {
	if err := Seq(Seq(Int(1))).validate(); !IsKind(err, KindInvalidValue) {
		t.Errorf("nested sequences are outside the closed model: %v", err)
	}
}

func TestSeqValidatesEveryElement(t *testing.T) {
	if err := Seq(String("ok"), String("bad\x00")).validate(); !IsKind(err, KindInvalidValue) {
		t.Errorf("a defective tail element must fail the whole sequence: %v", err)
	}
}

func TestValidKeyGrammar(t *testing.T) {
	for _, ok := range []string{"id", "depends_on", "a1", "claimed_at"} {
		if !validKey(ok) {
			t.Errorf("validKey(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "Id", "1a", "-x", "with-hyphen", "with space", "UPPER"} {
		if validKey(bad) {
			t.Errorf("validKey(%q) = true", bad)
		}
	}
}

func TestBlockContentAllowsLFAndTabOnly(t *testing.T) {
	if err := validBlockContent("line one\nline\ttwo\n"); err != nil {
		t.Fatalf("LF and tab are legal: %v", err)
	}
	for _, bad := range []string{"nul\x00", "esc\x1b", "cr\r\n"} {
		if err := validBlockContent(bad); !IsKind(err, KindInvalidValue) {
			t.Errorf("validBlockContent(%q) = %v, want invalid-value", bad, err)
		}
	}
	if err := validBlockContent(string([]byte{0xff})); !IsKind(err, KindInvalidValue) {
		t.Errorf("invalid UTF-8 block content = %v, want invalid-value", err)
	}
}

func TestSerializedValuesRoundTripThroughYAML(t *testing.T) {
	// The serializer's output must be understood by the same semantic decoder.
	src := "---\ns: " + String("it's # tricky: yes").serialize() +
		"\nn: " + Int(-7).serialize() +
		"\nb: " + Bool(true).serialize() +
		"\nq: " + Seq(Int(3), String("x")).serialize() + "\n---\n"
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		S string `yaml:"s"`
		N int    `yaml:"n"`
		B bool   `yaml:"b"`
		Q []any  `yaml:"q"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatal(err)
	}
	if got.S != "it's # tricky: yes" || got.N != -7 || !got.B || len(got.Q) != 2 {
		t.Fatalf("round trip lost data: %+v", got)
	}
}

// TestNullSequenceElementRoundTrips pins the null-in-sequence rendering decided
// empirically in Task 6 (see the comment beside serialize): the rendered token
// must decode back as a three-element sequence whose tail is nil.
func TestNullSequenceElementRoundTrips(t *testing.T) {
	src := "---\nq: " + Seq(String("a"), Bool(true), Null()).serialize() + "\n---\n"
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Q []any `yaml:"q"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Q) != 3 {
		t.Fatalf("len(q) = %d, want 3: %+v", len(got.Q), got.Q)
	}
	if got.Q[2] != nil {
		t.Errorf("q[2] = %#v, want nil", got.Q[2])
	}
}

// TestTopLevelNullRendersEmpty pins the "key:" form for a top-level null
// regardless of the sequence-element decision.
func TestTopLevelNullRendersEmpty(t *testing.T) {
	if got := Null().serialize(); got != "" {
		t.Fatalf("Null().serialize() = %q, want empty", got)
	}
	d, err := Parse([]byte("---\nk:" + Null().serialize() + "\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		K *string `yaml:"k"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatal(err)
	}
	if got.K != nil {
		t.Errorf("k = %q, want nil", *got.K)
	}
}
