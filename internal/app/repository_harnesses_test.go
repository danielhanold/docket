package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// recordingChooser returns a HarnessChooser that records the request it was
// shown, counts its calls, and answers ret, err. Integration tests reuse it.
func recordingChooser(got *HarnessChoiceRequest, calls *int, ret []string, err error) HarnessChooser {
	return func(_ context.Context, req HarnessChoiceRequest) ([]string, error) {
		if got != nil {
			*got = req
		}
		if calls != nil {
			*calls++
		}
		return ret, err
	}
}

// harnessCfg builds an Effective whose agent_harnesses is explicit at layer,
// or unset when layer is "".
func harnessCfg(layer config.LayerKind, v ...string) config.Effective {
	var cfg config.Effective
	if layer != "" {
		if v == nil {
			v = []string{}
		}
		cfg.AgentHarnesses = config.Value[[]string]{Value: v, Explicit: true, Provenance: config.Provenance{Layer: layer}}
	}
	return cfg
}

func TestHarnessTokensMatchHarnessOrder(t *testing.T) {
	if !reflect.DeepEqual(config.AgentHarnessTokens(), harness.Order) {
		t.Fatalf("config tokens %v != harness.Order %v", config.AgentHarnessTokens(), harness.Order)
	}
}

func TestResolveHarnessChoice(t *testing.T) {
	ctx := context.Background()
	noDetect := func(t *testing.T) func() []string {
		return func() []string { t.Fatal("detect must not be called"); return nil }
	}

	t.Run("flag wins without asking", func(t *testing.T) {
		calls := 0
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			flag: []string{"claude"}, chooser: recordingChooser(nil, &calls, []string{"codex"}, nil),
			policy: harnessPolicyInit, cfg: harnessCfg(config.LayerRepository, "cursor"), repoDeclared: true,
		})
		if err != nil || !ch.decided || !reflect.DeepEqual(ch.selection, []string{"claude"}) || calls != 0 {
			t.Fatalf("got %+v err=%v calls=%d", ch, err, calls)
		}
	})

	t.Run("init keeps a repository value", func(t *testing.T) {
		calls := 0
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(nil, &calls, []string{"codex"}, nil),
			policy:  harnessPolicyInit, cfg: harnessCfg(config.LayerRepository, "claude"), repoDeclared: true,
		})
		if err != nil || ch.decided || ch.warning != "" || calls != 0 {
			t.Fatalf("got %+v err=%v calls=%d", ch, err, calls)
		}
	})

	t.Run("configure asks with the repository value pre-checked", func(t *testing.T) {
		var req HarnessChoiceRequest
		calls := 0
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(&req, &calls, []string{"cursor"}, nil),
			policy:  harnessPolicyConfigure, cfg: harnessCfg(config.LayerRepository, "claude", "cursor"),
			repoDeclared: true, configDisplay: ".docket.yml", detect: noDetect(t),
		})
		if err != nil || !ch.decided || !reflect.DeepEqual(ch.selection, []string{"cursor"}) || calls != 1 {
			t.Fatalf("got %+v err=%v calls=%d", ch, err, calls)
		}
		want := HarnessChoiceRequest{Options: harness.Order, Preselected: []string{"claude", "cursor"}, ConfigPath: ".docket.yml"}
		if !reflect.DeepEqual(req, want) {
			t.Fatalf("request %+v, want %+v", req, want)
		}
	})

	t.Run("a global value pre-checks without detection", func(t *testing.T) {
		var req HarnessChoiceRequest
		_, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(&req, nil, []string{"codex"}, nil),
			policy:  harnessPolicyInit, cfg: harnessCfg(config.LayerGlobal, "codex"), detect: noDetect(t),
		})
		if err != nil || !reflect.DeepEqual(req.Preselected, []string{"codex"}) {
			t.Fatalf("preselected %v err=%v", req.Preselected, err)
		}
	})

	t.Run("unset pre-checks detection; an empty confirmation decides none", func(t *testing.T) {
		var req HarnessChoiceRequest
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(&req, nil, []string{}, nil),
			policy:  harnessPolicyInit, cfg: harnessCfg(""), detect: func() []string { return []string{"cursor"} },
		})
		if err != nil || !reflect.DeepEqual(req.Preselected, []string{"cursor"}) {
			t.Fatalf("preselected %v err=%v", req.Preselected, err)
		}
		if !ch.decided || ch.selection == nil || len(ch.selection) != 0 {
			t.Fatalf("want decided non-nil empty, got %+v (nil=%v)", ch, ch.selection == nil)
		}
	})

	t.Run("chooser output is canonicalized and validated", func(t *testing.T) {
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(nil, nil, []string{"opencode", "claude"}, nil), policy: harnessPolicyConfigure,
		})
		if err != nil || !reflect.DeepEqual(ch.selection, []string{"claude", "opencode"}) {
			t.Fatalf("got %+v err=%v", ch, err)
		}
		if _, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(nil, nil, []string{"bogus"}, nil), policy: harnessPolicyConfigure,
		}); err == nil {
			t.Fatal("want an error for an invalid chooser selection")
		}
	})

	t.Run("cancel propagates", func(t *testing.T) {
		_, err := resolveHarnessChoice(ctx, harnessChoiceInput{
			chooser: recordingChooser(nil, nil, nil, ErrHarnessChoiceCancelled), policy: harnessPolicyInit,
		})
		if !errors.Is(err, ErrHarnessChoiceCancelled) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("init with no input warns", func(t *testing.T) {
		ch, err := resolveHarnessChoice(ctx, harnessChoiceInput{policy: harnessPolicyInit, configDisplay: ".git/dckt/config.yml"})
		if err != nil || ch.decided {
			t.Fatalf("got %+v err=%v", ch, err)
		}
		for _, want := range []string{"`agent_harnesses: [claude]`", ".git/dckt/config.yml", reposetup.ConfigureHarnessesCommand} {
			if !strings.Contains(ch.warning, want) {
				t.Fatalf("warning %q lacks %q", ch.warning, want)
			}
		}
	})

	t.Run("configure with no input refuses", func(t *testing.T) {
		_, err := resolveHarnessChoice(ctx, harnessChoiceInput{policy: harnessPolicyConfigure})
		if !errors.Is(err, errHarnessesFlagRequired) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestParseHarnessesFlag(t *testing.T) {
	got, refusal := parseHarnessesFlag(OperationRepositoryInit, HarnessesOptions{})
	if got != nil || refusal != nil {
		t.Fatalf("unset: got %v refusal %v", got, refusal)
	}
	got, refusal = parseHarnessesFlag(OperationRepositoryInit, HarnessesOptions{Set: true, Tokens: []string{"none"}})
	if refusal != nil || got == nil || len(got) != 0 {
		t.Fatalf("none: got %v (nil=%v) refusal %v", got, got == nil, refusal)
	}
	_, refusal = parseHarnessesFlag(OperationRepositoryInit, HarnessesOptions{Set: true, Tokens: []string{"bogus"}})
	if refusal == nil || refusal.Result != ResultInvalidInput || !strings.Contains(refusal.HumanText(), "bogus") {
		t.Fatalf("bogus: refusal %+v", refusal)
	}
}

func TestDetectHarnessesReadsTheHome(t *testing.T) {
	home := testsupport.TempDir(t)
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if got := detectHarnesses(); got == nil || len(got) != 0 {
		t.Fatalf("empty home: got %v", got)
	}
	for _, d := range []string{filepath.Join(home, ".cursor"), filepath.Join(xdg, "opencode")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got := detectHarnesses(); !reflect.DeepEqual(got, []string{"cursor", "opencode"}) {
		t.Fatalf("got %v", got)
	}
}

func TestRepositoryOpResultHarnessFields(t *testing.T) {
	b, err := json.Marshal(RepositoryOpResult{AgentHarnesses: &[]string{}, Warnings: []string{"w"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"agent_harnesses":[]`, `"warnings":["w"]`} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("%s lacks %s", b, want)
		}
	}
	b, err = json.Marshal(RepositoryOpResult{})
	if err != nil {
		t.Fatal(err)
	}
	for _, absent := range []string{"agent_harnesses", "warnings"} {
		if strings.Contains(string(b), absent) {
			t.Fatalf("%s carries %s", b, absent)
		}
	}
}

func TestAppendPendingDedupes(t *testing.T) {
	got := appendPending([]string{".gitignore"}, "", ".docket.yml", ".gitignore", ".docket.yml")
	if !reflect.DeepEqual(got, []string{".gitignore", ".docket.yml"}) {
		t.Fatalf("got %v", got)
	}
}
