package githubcli

import (
	"context"
	"path/filepath"
	"testing"
)

// sampleRepoJSON is the canonical nested shape `gh repo view --json
// nameWithOwner,owner,name,url` documents.
const sampleRepoJSON = `{
  "nameWithOwner": "acme/widget",
  "owner": {"login": "acme"},
  "name": "widget",
  "url": "https://github.com/acme/widget"
}`

func repoViewArm(j string) fakeArm {
	return fakeArm{ArgvPrefix: []string{"repo", "view"}, Stdout: j, Exit: 0}
}

// TestDiscoverRepositoryDecodesIdentity (g): discovery decodes host/owner/name
// from the documented fields.
func TestDiscoverRepositoryDecodesIdentity(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{repoViewArm(sampleRepoJSON)}})
	repo, err := c.DiscoverRepository(context.Background(), t.TempDir())
	if err != nil {
		t.Fatalf("DiscoverRepository: %v", err)
	}
	if repo.Host != "github.com" || repo.Owner != "acme" || repo.Name != "widget" {
		t.Fatalf("identity = %+v, want github.com/acme/widget", repo)
	}
	if repo.Spec() != "github.com/acme/widget" {
		t.Fatalf("Spec() = %q, want github.com/acme/widget", repo.Spec())
	}
}

// TestDiscoverRepositoryRejectsBadOutput (g): missing field, malformed JSON, and
// empty owner all yield invalid-output — never a zero-value identity.
func TestDiscoverRepositoryRejectsBadOutput(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"malformed json", `{not json`},
		{"empty owner", `{"nameWithOwner":"acme/widget","owner":{"login":""},"name":"widget","url":"https://github.com/acme/widget"}`},
		{"missing name", `{"nameWithOwner":"acme/widget","owner":{"login":"acme"},"url":"https://github.com/acme/widget"}`},
		{"missing url host", `{"nameWithOwner":"acme/widget","owner":{"login":"acme"},"name":"widget","url":""}`},
		{"owner with slash", `{"nameWithOwner":"acme/widget","owner":{"login":"ac/me"},"name":"widget","url":"https://github.com/acme/widget"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{repoViewArm(tc.json)}})
			repo, err := c.DiscoverRepository(context.Background(), t.TempDir())
			if err == nil {
				t.Fatalf("expected error, got identity %+v", repo)
			}
			f, ok := AsFailure(err)
			if !ok {
				t.Fatalf("not a *Failure: %v", err)
			}
			if f.Kind != KindInvalidOutput {
				t.Fatalf("kind = %q, want invalid-output", f.Kind)
			}
			if repo != (Repository{}) {
				t.Fatalf("returned non-zero identity on error: %+v", repo)
			}
		})
	}
}

// TestDiscoverRepositoryRunsInRequestedDir (h): discovery runs in the requested
// directory (witnessed cwd), so a caller's own CWD cannot retarget it.
func TestDiscoverRepositoryRunsInRequestedDir(t *testing.T) {
	c, log := newFakeClient(t, fakeScenario{Invocations: []fakeArm{repoViewArm(sampleRepoJSON)}})
	dir := t.TempDir()
	if _, err := c.DiscoverRepository(context.Background(), dir); err != nil {
		t.Fatalf("DiscoverRepository: %v", err)
	}
	recs := log.records(t)
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	wantCwd, _ := filepath.EvalSymlinks(dir)
	gotCwd, _ := filepath.EvalSymlinks(recs[0].Cwd)
	if gotCwd != wantCwd {
		t.Fatalf("cwd = %q, want %q", gotCwd, wantCwd)
	}
}

// TestDiscoverRepositoryNonZeroExit maps a non-zero gh exit to an external
// failure rather than a zero-value identity.
func TestDiscoverRepositoryNonZeroExit(t *testing.T) {
	c, _ := newFakeClient(t, fakeScenario{Invocations: []fakeArm{
		{ArgvPrefix: []string{"repo", "view"}, Exit: 1, Stderr: "gh: could not determine repository\n"},
	}})
	_, err := c.DiscoverRepository(context.Background(), t.TempDir())
	f, ok := AsFailure(err)
	if !ok {
		t.Fatalf("not a *Failure: %v", err)
	}
	if f.Kind != KindExternal {
		t.Fatalf("kind = %q, want external", f.Kind)
	}
}
