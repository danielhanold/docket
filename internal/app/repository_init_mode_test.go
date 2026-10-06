package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// TestDecideInitMode pins init's mode decision: a fresh repository takes the
// effective visibility value overridden by a flag; an already-set-up repository
// keeps its mode, and a flag that would switch it refuses; the two mode flags
// are mutually exclusive; and --metadata-remote applies only to a private
// result.
func TestDecideInitMode(t *testing.T) {
	const refuse = layout.Mode("refuse")
	cases := []struct {
		name      string
		detected  layout.Mode
		state     reposetup.State
		opts      InitOptions
		effective string
		want      layout.Mode
	}{
		{"fresh default shared", layout.Shared, reposetup.StateFresh, InitOptions{}, "shared", layout.Shared},
		{"fresh effective private", layout.Shared, reposetup.StateFresh, InitOptions{}, "private", layout.Private},
		{"fresh --shared overrides private", layout.Shared, reposetup.StateFresh, InitOptions{Shared: true}, "private", layout.Shared},
		{"fresh --private overrides shared", layout.Shared, reposetup.StateFresh, InitOptions{Private: true}, "shared", layout.Private},
		{"healthy shared ignores effective private", layout.Shared, reposetup.StateHealthy, InitOptions{}, "private", layout.Shared},
		{"healthy shared refuses --private", layout.Shared, reposetup.StateHealthy, InitOptions{Private: true}, "shared", refuse},
		{"private stays private", layout.Private, reposetup.StateHealthy, InitOptions{}, "shared", layout.Private},
		{"private refuses --shared", layout.Private, reposetup.StateHealthy, InitOptions{Shared: true}, "shared", refuse},
		{"both flags refuse", layout.Shared, reposetup.StateFresh, InitOptions{Private: true, Shared: true}, "shared", refuse},
		{"metadata-remote on a shared result refuses", layout.Shared, reposetup.StateFresh, InitOptions{MetadataRemote: "/x.git"}, "shared", refuse},
		{"healthy shared refuses --metadata-remote", layout.Shared, reposetup.StateHealthy, InitOptions{MetadataRemote: "/x.git"}, "private", refuse},
		{"fresh metadata-remote with --private", layout.Shared, reposetup.StateFresh, InitOptions{Private: true, MetadataRemote: "/x.git"}, "shared", layout.Private},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, refusal := decideInitMode(tc.detected, tc.state, tc.opts, tc.effective)
			if tc.want == refuse {
				if refusal == "" {
					t.Fatalf("decideInitMode = %q with no refusal, want a refusal", got)
				}
				if strings.Contains(refusal, "set-visibility") {
					t.Errorf("refusal %q names set-visibility, which does not exist yet", refusal)
				}
				return
			}
			if refusal != "" {
				t.Fatalf("decideInitMode refused %q, want %q", refusal, tc.want)
			}
			if got != tc.want {
				t.Errorf("decideInitMode = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRepoConfigTarget pins where a generated test policy lands: a private
// repository writes its .git/dckt/config.yml with no pending review path, and a
// shared repository writes the root .docket.yml as a pending review path.
func TestRepoConfigTarget(t *testing.T) {
	priv := setupContext{
		repo:   gitcli.Repository{CommonDir: "/r/.git", PrimaryWorktree: "/r"},
		layout: layout.PrivateLayout("/r/.git", "/r", "/d", "o-r"),
	}
	abs, display, pending := repoConfigTarget(priv)
	if abs != "/r/.git/dckt/config.yml" || display != layout.PrivateConfigDisplay || pending {
		t.Errorf("private target = (%q, %q, %v), want (/r/.git/dckt/config.yml, %q, false)", abs, display, pending, layout.PrivateConfigDisplay)
	}
	shared := setupContext{
		repo:   gitcli.Repository{CommonDir: "/r/.git", PrimaryWorktree: "/r"},
		layout: layout.SharedLayout("/r/.git", "/r"),
	}
	abs, display, pending = repoConfigTarget(shared)
	if abs != "/r/.docket.yml" || display != ".docket.yml" || !pending {
		t.Errorf("shared target = (%q, %q, %v), want (/r/.docket.yml, .docket.yml, true)", abs, display, pending)
	}
}

// TestRunRepositoryInitRefusesBothModeFlagsBeforeGather proves the mutually
// exclusive flags are an invalid-input refusal returned before any repository
// read: the deps carry no Git client, so a gather would panic.
func TestRunRepositoryInitRefusesBothModeFlagsBeforeGather(t *testing.T) {
	res := RunRepositoryInit(t.Context(), SetupDeps{}, InitOptions{Private: true, Shared: true})
	if res.Result != ResultInvalidInput {
		t.Fatalf("Result = %q (%s), want invalid-input", res.Result, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "mutually exclusive") {
		t.Errorf("text %q must say the flags are mutually exclusive", res.HumanText())
	}
}

// TestConfigureTestsPrivateWrittenText proves a private configure-tests names
// the clone-local config it wrote and never a pending review path.
func TestConfigureTestsPrivateWrittenText(t *testing.T) {
	got := configureTestsExplicitText(reposetup.StateHealthy, true, layout.PrivateConfigDisplay, "true")
	if !strings.Contains(got, "wrote .git/dckt/config.yml") || strings.Contains(got, "pending") {
		t.Errorf("private explicit text %q must say it wrote .git/dckt/config.yml with no pending path", got)
	}
}
