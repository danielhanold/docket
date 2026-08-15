package gitcli

import (
	"strings"
	"testing"
)

func TestValidateRepoPath(t *testing.T) {
	good := []RepoPath{"a", "a/b.txt", "docs/changes/active/0001-x.md", "spa ce/né.md"}
	for _, p := range good {
		if err := validateRepoPath(p, false); err != nil {
			t.Errorf("validateRepoPath(%q) = %v, want nil", p, err)
		}
	}
	bad := []RepoPath{"", "/abs", "a/", "a//b", ".", "./a", "a/./b", "..", "a/../b", "a\x00b"}
	for _, p := range bad {
		if err := validateRepoPath(p, false); err == nil {
			t.Errorf("validateRepoPath(%q) = nil, want error", p)
		}
	}
	if err := validateRepoPath("", true); err != nil {
		t.Errorf("empty root prefix must be legal with allowEmptyRootPrefix: %v", err)
	}
}

func TestValidateRemoteName(t *testing.T) {
	good := []RemoteName{"origin", "up.stream", "a_b"}
	for _, r := range good {
		if err := validateRemoteName(r); err != nil {
			t.Errorf("validateRemoteName(%q) = %v, want nil", r, err)
		}
	}
	bad := []RemoteName{"", "-origin", "a b", "a\tb", "a/b", "a\x00b"}
	for _, r := range bad {
		if err := validateRemoteName(r); err == nil {
			t.Errorf("validateRemoteName(%q) accepted", r)
		}
	}
}

func TestValidateRefName(t *testing.T) {
	good := []RefName{"refs/heads/main", "refs/remotes/origin/main", "refs/heads/feat/x"}
	for _, r := range good {
		if err := validateRefName(r); err != nil {
			t.Errorf("validateRefName(%q) = %v, want nil", r, err)
		}
	}
	bad := []RefName{"main", "heads/main", "refs/", "refs/heads/", "-refs/heads/x",
		"refs/heads/a b", "refs/heads/a..b", "refs/heads/a.lock", "refs/heads/a@{1}",
		"refs/heads/*", "refs/heads/a\\b", "refs/heads/.hidden", "refs/heads/a\x00b"}
	for _, r := range bad {
		if err := validateRefName(r); err == nil {
			t.Errorf("validateRefName(%q) accepted", r)
		}
	}
}

func TestValidateObjectID(t *testing.T) {
	sha1 := ObjectID(strings.Repeat("ab", 20))
	sha256 := ObjectID(strings.Repeat("cd", 32))
	for _, id := range []ObjectID{sha1, sha256} {
		if err := validateObjectID(id); err != nil {
			t.Fatalf("validateObjectID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range []ObjectID{"", "abc", ObjectID(strings.Repeat("AB", 20)), ObjectID(strings.Repeat("zz", 20))} {
		if err := validateObjectID(id); err == nil {
			t.Errorf("validateObjectID(%q) accepted", id)
		}
	}
}
