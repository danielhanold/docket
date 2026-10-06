package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSameOrigin pins which origin URL spellings name one repository for the
// default store's origin record: transport, user, trailing ".git" and "/" do
// not matter; host, owner and repo (case included) do.
func TestSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"git@github.com:acme/web-app.git", "https://github.com/acme/web-app", true},
		{"ssh://git@github.com/acme/web-app.git", "https://github.com/acme/web-app.git/", true},
		{"https://user@github.com/acme/web-app.git", "git@github.com:acme/web-app.git", true},
		{"/srv/git/acme/web-app.git", "/srv/git/acme/web-app", true},
		{"git@github.com:acme/web-app.git", "git@github.com:acme-web/app.git", false},
		{"git@github.com:acme/web-app.git", "git@gitlab.com:acme/web-app.git", false},
		{"https://github.com/acme/Web-App", "https://github.com/acme/web-app", false},
		{"/srv/a/acme/web-app.git", "/srv/b/acme/web-app.git", false},
	} {
		if got := sameOrigin(tc.a, tc.b); got != tc.want {
			t.Errorf("sameOrigin(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestAbsMetadataRemote(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"":                          "",
		"https://example.com/x.git": "https://example.com/x.git",
		"file:///srv/x.git":         "file:///srv/x.git",
		"git@host:org/x.git":        "git@host:org/x.git",
		"host:x.git":                "host:x.git",
		"/srv/meta/../x.git":        "/srv/x.git",
		"../backup.git":             filepath.Join(filepath.Dir(cwd), "backup.git"),
		"./a:b/x.git":               filepath.Join(cwd, "a:b", "x.git"),
	}
	for in, want := range cases {
		got, err := absMetadataRemote(in)
		if err != nil {
			t.Fatalf("absMetadataRemote(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("absMetadataRemote(%q) = %q, want %q", in, got, want)
		}
	}
}
