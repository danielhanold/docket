package app

import "testing"

// TestPublicationDigestDomainSeparated: digests are sha256-hex, label-separated with
// an unambiguous NUL boundary so ("a","bc") can never collide with ("ab","c") or a
// different label over the same bytes.
func TestPublicationDigestDomainSeparated(t *testing.T) {
	d1 := publicationDigest("pr-title", "Add widget")
	if len(d1) != 64 {
		t.Fatalf("digest length = %d, want 64 hex chars", len(d1))
	}
	if d1 != publicationDigest("pr-title", "Add widget") {
		t.Fatal("digest must be deterministic")
	}
	if d1 == publicationDigest("pr-body", "Add widget") {
		t.Fatal("different labels over the same value must not collide")
	}
	if publicationDigest("l", "ab") == publicationDigest("la", "b") {
		t.Fatal("label/value boundary must be unambiguous")
	}
}

// TestValidPublicationPerOp: validation enforces operation/descriptor agreement —
// each op requires its own fields non-empty AND the other op's fields empty; an
// unknown op is never valid; nil is never valid.
func TestValidPublicationPerOp(t *testing.T) {
	pr := &MutationPublication{
		RepoHost: "github.com", RepoOwner: "danielhanold", RepoName: "docket",
		HeadRef: "fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		BaseBranch:  "main",
		TitleDigest: publicationDigest("pr-title", "t"),
		BodyDigest:  publicationDigest("pr-body", "b"),
	}
	ws := &MutationPublication{
		RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/widget", HeadCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if !validPublication(OperationPRPublish, pr) {
		t.Fatal("complete PR descriptor must validate")
	}
	if !validPublication(OperationWorkspacePublish, ws) {
		t.Fatal("complete workspace descriptor must validate")
	}
	if validPublication(OperationPRPublish, nil) || validPublication(OperationWorkspacePublish, nil) {
		t.Fatal("nil descriptor is never valid")
	}
	if validPublication("metadata.transaction", pr) {
		t.Fatal("unknown operation is never valid")
	}
	// Each required PR field, blanked, invalidates.
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoHost = "" },
		func(p *MutationPublication) { p.RepoOwner = "" },
		func(p *MutationPublication) { p.RepoName = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
		func(p *MutationPublication) { p.BaseBranch = "" },
		func(p *MutationPublication) { p.TitleDigest = "" },
		func(p *MutationPublication) { p.BodyDigest = "" },
	} {
		c := *pr
		blank(&c)
		if validPublication(OperationPRPublish, &c) {
			t.Fatalf("PR descriptor with a blanked required field must not validate: %+v", c)
		}
	}
	// Agreement: a PR descriptor carrying workspace-only fields (and vice versa) is
	// op/descriptor disagreement and invalid.
	crossPR := *pr
	crossPR.Remote = "origin"
	if validPublication(OperationPRPublish, &crossPR) {
		t.Fatal("PR descriptor carrying a workspace-only field must not validate")
	}
	crossWS := *ws
	crossWS.TitleDigest = "x"
	if validPublication(OperationWorkspacePublish, &crossWS) {
		t.Fatal("workspace descriptor carrying a PR-only field must not validate")
	}
	for _, blank := range []func(*MutationPublication){
		func(p *MutationPublication) { p.RepoDir = "" },
		func(p *MutationPublication) { p.Remote = "" },
		func(p *MutationPublication) { p.HeadRef = "" },
		func(p *MutationPublication) { p.HeadCommit = "" },
	} {
		c := *ws
		blank(&c)
		if validPublication(OperationWorkspacePublish, &c) {
			t.Fatalf("workspace descriptor with a blanked required field must not validate: %+v", c)
		}
	}
}
