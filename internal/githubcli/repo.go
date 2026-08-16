package githubcli

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
)

// discoverRepositoryOp labels every Failure raised while resolving repository
// identity from Git context.
const discoverRepositoryOp = "discover-repository"

// Repository is the canonical GitHub identity of a checkout: host, owner, and
// repository name. Every field is validated non-empty, free of '/' and
// whitespace, so the value can be composed into a "--repo host/owner/name"
// argument without ambiguity.
type Repository struct {
	Host  string
	Owner string
	Name  string
}

// Spec renders the "host/owner/name" form gh's --repo flag consumes.
func (r Repository) Spec() string {
	return r.Host + "/" + r.Owner + "/" + r.Name
}

// repoViewJSON is the subset of `gh repo view --json ...` fields discovery
// decodes. owner is a nested object whose login is the owner handle; url carries
// the host in its authority component. These are gh's documented field shapes,
// never a flattened fake-only shape.
type repoViewJSON struct {
	NameWithOwner string `json:"nameWithOwner"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DiscoverRepository resolves the current checkout's GitHub host/owner/name via
// `gh repo view` run FROM dir. This is the ONLY call that infers a repository
// from Git context; every later call passes the returned identity explicitly
// with --repo, so a caller's CWD, GH_REPO, or a different checked-out worktree
// cannot retarget the effect. A missing required field, malformed JSON, an empty
// owner, or a host that cannot be derived is invalid-output — never a
// zero-value identity.
func (c *Client) DiscoverRepository(ctx context.Context, dir string) (Repository, error) {
	if dir == "" {
		return Repository{}, newFailure(discoverRepositoryOp, StageValidate, KindInvalidInput, "discovery directory is empty", nil)
	}
	res, f := c.run(ctx, runRequest{
		op:      discoverRepositoryOp,
		dir:     dir,
		args:    []string{"repo", "view", "--json", "nameWithOwner,owner,name,url"},
		network: true,
	})
	if f != nil {
		return Repository{}, f
	}
	if res.exitCode != 0 {
		return Repository{}, newFailure(discoverRepositoryOp, StageInvoke, KindExternal,
			"gh repo view failed: "+stderrExcerpt(res.stderr), nil)
	}

	var raw repoViewJSON
	if err := json.Unmarshal(res.stdout, &raw); err != nil {
		return Repository{}, newFailure(discoverRepositoryOp, StageDecode, KindInvalidOutput, "gh repo view output is not valid JSON", err)
	}

	host, err := hostFromRepoURL(raw.URL)
	if err != nil {
		return Repository{}, newFailure(discoverRepositoryOp, StageDecode, KindInvalidOutput, "gh repo view carried no derivable host", err)
	}
	repo := Repository{Host: host, Owner: raw.Owner.Login, Name: raw.Name}
	if err := validateRepository(repo); err != nil {
		return Repository{}, newFailure(discoverRepositoryOp, StageDecode, KindInvalidOutput, err.Error(), err)
	}
	return repo, nil
}

// hostFromRepoURL extracts the authority host from a repository URL such as
// "https://github.com/owner/name". A URL that does not parse or carries no host
// is rejected rather than silently yielding an empty host.
func hostFromRepoURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", errRepoField("url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", errRepoField("url has no host")
	}
	return u.Host, nil
}

// validateRepository enforces the Repository invariants: every field non-empty,
// free of '/' and whitespace, so it composes into "--repo host/owner/name"
// unambiguously.
func validateRepository(r Repository) error {
	for _, fld := range []struct {
		name  string
		value string
	}{{"host", r.Host}, {"owner", r.Owner}, {"name", r.Name}} {
		if fld.value == "" {
			return errRepoField(fld.name + " is empty")
		}
		if strings.ContainsAny(fld.value, "/") {
			return errRepoField(fld.name + " contains a slash")
		}
		if strings.ContainsFunc(fld.value, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }) {
			return errRepoField(fld.name + " contains whitespace")
		}
	}
	return nil
}

// repoFieldError is a small typed error so validateRepository failures carry a
// stable, redaction-safe message.
type repoFieldError struct{ msg string }

func (e repoFieldError) Error() string { return "repository identity: " + e.msg }

func errRepoField(msg string) error { return repoFieldError{msg: msg} }
