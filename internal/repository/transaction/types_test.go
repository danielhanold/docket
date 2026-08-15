package transaction

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
)

func TestValidateOperationKey(t *testing.T) {
	cases := []struct {
		key  OperationKey
		ok   bool
		name string
	}{
		{"change.groom", true, "dotted"},
		{"a", true, "single-letter"},
		{"x9.y-z", true, "digits-dot-dash"},
		{"", false, "empty"},
		{"Change", false, "uppercase-lead"},
		{"9x", false, "digit-lead"},
		{"a b", false, "space"},
		{"a:b", false, "colon"},
		{"café", false, "unicode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateOperationKey(c.key)
			if c.ok && err != nil {
				t.Fatalf("validateOperationKey(%q) = %v, want nil", c.key, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("validateOperationKey(%q) = nil, want error", c.key)
			}
		})
	}
}

func validDigest() RequestDigest {
	return RequestDigest("sha256:" + strings.Repeat("a", 64))
}

func TestValidateIdempotencyKeyRequestID(t *testing.T) {
	cases := []struct {
		id   string
		ok   bool
		name string
	}{
		{strings.Repeat("a", 8), true, "min-8"},
		{strings.Repeat("a", 128), true, "max-128"},
		{"a1._-b2c", true, "allowed-punct"},
		{strings.Repeat("a", 7), false, "too-short-7"},
		{strings.Repeat("a", 129), false, "too-long-129"},
		{"abcd efg", false, "space"},
		{"abcd:efg", false, "colon"},
		{"abcd\nefg", false, "newline"},
		{"abcd\x01efg", false, "control"},
		{"abcdéfg", false, "unicode"},
		{".abcdefg", false, "leading-dot"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := &IdempotencyKey{RequestID: c.id, Digest: validDigest()}
			err := validateIdempotencyKey(k)
			if c.ok && err != nil {
				t.Fatalf("validateIdempotencyKey(id=%q) = %v, want nil", c.id, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("validateIdempotencyKey(id=%q) = nil, want error", c.id)
			}
		})
	}
}

func TestValidateIdempotencyKeyDigest(t *testing.T) {
	cases := []struct {
		digest RequestDigest
		ok     bool
		name   string
	}{
		{RequestDigest("sha256:" + strings.Repeat("a", 64)), true, "valid"},
		{RequestDigest("sha256:" + strings.Repeat("A", 64)), false, "uppercase-hex"},
		{RequestDigest("sha256:" + strings.Repeat("a", 63)), false, "short-hex"},
		{RequestDigest(strings.Repeat("a", 64)), false, "missing-prefix"},
		{RequestDigest("sha1:" + strings.Repeat("a", 64)), false, "wrong-prefix"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			k := &IdempotencyKey{RequestID: strings.Repeat("a", 16), Digest: c.digest}
			err := validateIdempotencyKey(k)
			if c.ok && err != nil {
				t.Fatalf("validateIdempotencyKey(digest=%q) = %v, want nil", c.digest, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("validateIdempotencyKey(digest=%q) = nil, want error", c.digest)
			}
		})
	}
}

func TestValidateIdempotencyKeyNilIsValid(t *testing.T) {
	if err := validateIdempotencyKey(nil); err != nil {
		t.Fatalf("validateIdempotencyKey(nil) = %v, want nil", err)
	}
}

func TestValidateExpectations(t *testing.T) {
	fullHex := gitcli.ObjectID(strings.Repeat("a", 40))
	good := []EntityExpectation{
		{Path: "docs/changes/0001-x.md", Version: ExpectedVersion{Kind: VersionBlob, ObjectID: fullHex}},
		{Path: "docs/changes/0002-y.md", Version: ExpectedVersion{Kind: VersionAbsent}},
	}
	if err := validateExpectations(good); err != nil {
		t.Fatalf("validateExpectations(good) = %v, want nil", err)
	}

	cases := []struct {
		exps []EntityExpectation
		name string
	}{
		{[]EntityExpectation{{Path: "", Version: ExpectedVersion{Kind: VersionBlob, ObjectID: fullHex}}}, "empty-path"},
		{[]EntityExpectation{{Path: "a.md", Version: ExpectedVersion{Kind: VersionBlob, ObjectID: gitcli.ObjectID("abcdef0")}}}, "abbreviated-sha"},
		{[]EntityExpectation{{Path: "a.md", Version: ExpectedVersion{Kind: VersionBlob, ObjectID: ""}}}, "blob-empty-id"},
		{[]EntityExpectation{{Path: "a.md", Version: ExpectedVersion{Kind: VersionAbsent, ObjectID: fullHex}}}, "absent-with-id"},
		{[]EntityExpectation{{Path: "a.md", Version: ExpectedVersion{Kind: "", ObjectID: fullHex}}}, "unknown-kind"},
		{[]EntityExpectation{
			{Path: "a.md", Version: ExpectedVersion{Kind: VersionAbsent}},
			{Path: "a.md", Version: ExpectedVersion{Kind: VersionAbsent}},
		}, "duplicate-path"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validateExpectations(c.exps); err == nil {
				t.Fatalf("validateExpectations(%s) = nil, want error", c.name)
			}
		})
	}
}

func canonicalReceipt() []byte {
	return []byte(`{"a":1,"b":"x","c":true,"d":[1,2,3]}`)
}

func validPlan() MutationPlan {
	return MutationPlan{
		Files: []FileMutation{
			{Path: "docs/changes/0001-x.md", Kind: MutationCreate, Bytes: []byte("body")},
			{Path: "docs/changes/0002-y.md", Kind: MutationReplace, Bytes: []byte("new")},
			{Path: "docs/changes/0003-z.md", Kind: MutationDelete},
		},
		CommitSubject: "docket: apply change",
		Receipt:       canonicalReceipt(),
	}
}

func TestValidatePlanAccepts(t *testing.T) {
	if err := validatePlan(validPlan()); err != nil {
		t.Fatalf("validatePlan(valid) = %v, want nil", err)
	}
	// empty Bytes on create is legal (intentionally empty file).
	p := validPlan()
	p.Files = []FileMutation{{Path: "docs/empty.md", Kind: MutationCreate, Bytes: []byte{}}}
	if err := validatePlan(p); err != nil {
		t.Fatalf("validatePlan(empty-create-bytes) = %v, want nil", err)
	}
	// 200-byte subject is legal.
	p = validPlan()
	p.CommitSubject = strings.Repeat("a", 200)
	if err := validatePlan(p); err != nil {
		t.Fatalf("validatePlan(200-byte subject) = %v, want nil", err)
	}
}

func TestValidatePlanPaths(t *testing.T) {
	cases := []struct {
		files []FileMutation
		name  string
	}{
		{[]FileMutation{
			{Path: "a.md", Kind: MutationCreate, Bytes: []byte("x")},
			{Path: "a.md", Kind: MutationReplace, Bytes: []byte("y")},
		}, "duplicate-path"},
		{[]FileMutation{{Path: "/etc/passwd", Kind: MutationCreate, Bytes: []byte("x")}}, "absolute"},
		{[]FileMutation{{Path: "a/../b", Kind: MutationCreate, Bytes: []byte("x")}}, "dotdot"},
		{[]FileMutation{{Path: "a/./b", Kind: MutationCreate, Bytes: []byte("x")}}, "dot-segment"},
		{[]FileMutation{{Path: "a\x00b", Kind: MutationCreate, Bytes: []byte("x")}}, "nul"},
		{[]FileMutation{{Path: ".git", Kind: MutationCreate, Bytes: []byte("x")}}, "dotgit"},
		{[]FileMutation{{Path: ".git/config", Kind: MutationReplace, Bytes: []byte("x")}}, "dotgit-child"},
		{[]FileMutation{{Path: "", Kind: MutationCreate, Bytes: []byte("x")}}, "empty"},
		{[]FileMutation{{Path: "a.md", Kind: MutationDelete, Bytes: []byte("x")}}, "delete-with-bytes"},
		{[]FileMutation{{Path: "a.md", Kind: "mutate", Bytes: []byte("x")}}, "unknown-kind"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPlan()
			p.Files = c.files
			if err := validatePlan(p); err == nil {
				t.Fatalf("validatePlan(%s) = nil, want error", c.name)
			}
		})
	}
}

func TestValidatePlanSubject(t *testing.T) {
	cases := []struct {
		subject string
		name    string
	}{
		{"", "empty"},
		{"line1\nline2", "embedded-newline"},
		{"hello\x01world", "control-char"},
		{strings.Repeat("a", 201), "201-bytes"},
		{"bad\xffutf8", "invalid-utf8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validPlan()
			p.CommitSubject = c.subject
			if err := validatePlan(p); err == nil {
				t.Fatalf("validatePlan(subject=%q) = nil, want error", c.subject)
			}
		})
	}
}
