package githubcli

import (
	"strings"
	"testing"
)

// TestDecodePullRequestFull (i): a full PR decodes from the nested documented
// fields into the typed value.
func TestDecodePullRequestFull(t *testing.T) {
	pr, err := decodePullRequest("decode", []byte(samplePRJSON))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := PullRequest{
		Number:     7,
		URL:        "https://github.com/acme/widget/pull/7",
		State:      StateOpen,
		Draft:      false,
		HeadBranch: "feat/x",
		HeadCommit: "1111111111111111111111111111111111111111",
		BaseBranch: "main",
		Title:      "Add widget",
		Body:       "Body text",
	}
	want.Version = pr.Version // compared separately below
	if pr != want {
		t.Fatalf("decoded = %+v, want %+v", pr, want)
	}
	if !strings.HasPrefix(pr.Version, "sha256:") || len(pr.Version) != len("sha256:")+64 {
		t.Fatalf("version malformed: %q", pr.Version)
	}
}

// TestDecodePullRequestStateEnum maps GitHub's uppercase state enum to the typed
// lowercase State and rejects an unknown enum as invalid-state.
func TestDecodePullRequestStateEnum(t *testing.T) {
	for raw, want := range map[string]State{"OPEN": StateOpen, "CLOSED": StateClosed, "MERGED": StateMerged} {
		j := strings.Replace(samplePRJSON, `"state": "OPEN"`, `"state": "`+raw+`"`, 1)
		pr, err := decodePullRequest("decode", []byte(j))
		if err != nil {
			t.Fatalf("state %q: %v", raw, err)
		}
		if pr.State != want {
			t.Fatalf("state %q decoded to %q, want %q", raw, pr.State, want)
		}
	}
	bad := strings.Replace(samplePRJSON, `"state": "OPEN"`, `"state": "LOCKED"`, 1)
	_, err := decodePullRequest("decode", []byte(bad))
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindInvalidState {
		t.Fatalf("unknown enum: got %v, want invalid-state", err)
	}
}

// TestDecodePullRequestRejectsBadFields (i): missing headRefOid, an abbreviated
// oid, and other missing required fields are invalid-output — never zero-value.
func TestDecodePullRequestRejectsBadFields(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"missing headRefOid", `{"number":7,"url":"https://x/pull/7","state":"OPEN","isDraft":false,"headRefName":"feat/x","baseRefName":"main","title":"t","body":"b"}`},
		{"abbreviated oid", strings.Replace(samplePRJSON, "1111111111111111111111111111111111111111", "1111111", 1)},
		{"uppercase oid", strings.Replace(samplePRJSON, "1111111111111111111111111111111111111111", "1111111111111111111111111111111111111AAA", 1)},
		{"missing url", `{"number":7,"state":"OPEN","isDraft":false,"headRefName":"feat/x","headRefOid":"1111111111111111111111111111111111111111","baseRefName":"main","title":"t","body":"b"}`},
		{"missing headRefName", `{"number":7,"url":"https://x/pull/7","state":"OPEN","isDraft":false,"headRefOid":"1111111111111111111111111111111111111111","baseRefName":"main","title":"t","body":"b"}`},
		{"zero number", strings.Replace(samplePRJSON, `"number": 7`, `"number": 0`, 1)},
		{"malformed json", `{not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pr, err := decodePullRequest("decode", []byte(tc.json))
			if err == nil {
				t.Fatalf("expected error, got %+v", pr)
			}
			f, ok := AsFailure(err)
			if !ok || f.Kind != KindInvalidOutput {
				t.Fatalf("got %v, want invalid-output", err)
			}
			if pr != (PullRequest{}) {
				t.Fatalf("non-zero PR returned on error: %+v", pr)
			}
		})
	}
}

// TestDecodeSolePullRequestRejectsAmbiguity (i): decoding a single PR from a
// list requires exactly one element; a list-of-2 is ambiguous (invalid-state)
// and an empty list is invalid-output.
func TestDecodeSolePullRequestRejectsAmbiguity(t *testing.T) {
	two := "[" + samplePRJSON + "," + samplePRJSON + "]"
	_, err := decodeSolePullRequest("decode", []byte(two))
	f, ok := AsFailure(err)
	if !ok || f.Kind != KindInvalidState {
		t.Fatalf("list-of-2: got %v, want invalid-state", err)
	}

	_, err = decodeSolePullRequest("decode", []byte("[]"))
	f, ok = AsFailure(err)
	if !ok || f.Kind != KindInvalidOutput {
		t.Fatalf("empty list: got %v, want invalid-output", err)
	}

	pr, err := decodeSolePullRequest("decode", []byte("["+samplePRJSON+"]"))
	if err != nil {
		t.Fatalf("list-of-1: %v", err)
	}
	if pr.Number != 7 {
		t.Fatalf("decoded number = %d, want 7", pr.Number)
	}
}

// TestComputeVersionSensitivity (j): the version changes when any single field
// changes and is stable across JSON map ordering.
func TestComputeVersionSensitivity(t *testing.T) {
	base := PullRequest{
		Number: 7, URL: "u", State: StateOpen, Draft: false,
		HeadBranch: "feat/x", HeadCommit: "1111111111111111111111111111111111111111",
		BaseBranch: "main", Title: "t", Body: "b",
	}
	baseV := computeVersion(base)

	mutate := []func(*PullRequest){
		func(p *PullRequest) { p.Number = 8 },
		func(p *PullRequest) { p.State = StateClosed },
		func(p *PullRequest) { p.Draft = true },
		func(p *PullRequest) { p.HeadBranch = "feat/y" },
		func(p *PullRequest) { p.HeadCommit = "2222222222222222222222222222222222222222" },
		func(p *PullRequest) { p.BaseBranch = "develop" },
		func(p *PullRequest) { p.Title = "t2" },
		func(p *PullRequest) { p.Body = "b2" },
	}
	for i, m := range mutate {
		pr := base
		m(&pr)
		if computeVersion(pr) == baseV {
			t.Fatalf("mutation %d did not change the version", i)
		}
	}
	// URL is NOT part of the version snapshot (it is server-assigned, not a
	// mutable field the caller approved); changing it must NOT change the token.
	urlChanged := base
	urlChanged.URL = "different"
	if computeVersion(urlChanged) != baseV {
		t.Fatal("URL is not part of the version snapshot but changed the token")
	}
}

// TestComputeVersionLengthPrefixCollision (j): two PRs differing only by
// (Title="ab",Body="c") vs (Title="a",Body="bc") get DIFFERENT versions —
// proving the length prefix prevents a field-boundary collision.
func TestComputeVersionLengthPrefixCollision(t *testing.T) {
	base := PullRequest{
		Number: 7, URL: "u", State: StateOpen, Draft: false,
		HeadBranch: "feat/x", HeadCommit: "1111111111111111111111111111111111111111",
		BaseBranch: "main",
	}
	a := base
	a.Title, a.Body = "ab", "c"
	b := base
	b.Title, b.Body = "a", "bc"
	if computeVersion(a) == computeVersion(b) {
		t.Fatal("field-boundary collision: length prefix not applied")
	}
}
