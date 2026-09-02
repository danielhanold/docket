package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "docket"}
	root.PersistentFlags().Bool("json", false, "emit protocol-v1 JSON on stdout")
	return root
}

func leaf(use, id string, effects ...Effect) *cobra.Command {
	return &cobra.Command{Use: use, Annotations: capability(id, effects...),
		RunE: func(*cobra.Command, []string) error { return nil }}
}

func TestCollectIncludesAnnotatedLeavesSorted(t *testing.T) {
	root := newTestRoot()
	grp := &cobra.Command{Use: "grp", RunE: func(*cobra.Command, []string) error { return nil }}
	grp.AddCommand(leaf("beta", "grp.beta", EffectRead), leaf("alpha", "grp.alpha", EffectMetadataWrite))
	root.AddCommand(grp)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != "grp.alpha" || entries[1].ID != "grp.beta" {
		t.Fatalf("entries = %+v", entries)
	}
	if got := strings.Join(entries[1].Argv, " "); got != "docket grp beta" {
		t.Fatalf("argv = %q", got)
	}
	if got := entries[0].Effects; len(got) != 1 || got[0] != "metadata-write" {
		t.Fatalf("effects = %v", got)
	}
}

func TestCollectAnnotatedParentIsAnEntry(t *testing.T) {
	// `install` shape: executable AND has an annotated child.
	root := newTestRoot()
	parent := leaf("install", "install", EffectLocalWrite)
	parent.AddCommand(leaf("check", "install.check", EffectRead))
	root.AddCommand(parent)
	entries, err := collectCapabilities(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries = %+v, err = %v", entries, err)
	}
	if entries[0].ID != "install" || entries[1].ID != "install.check" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestCollectRejectsUnclassifiedLeaf(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(&cobra.Command{Use: "orphan", RunE: func(*cobra.Command, []string) error { return nil }})
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("want unclassified-leaf error naming the command, got %v", err)
	}
}

func TestCollectRejectsDuplicateID(t *testing.T) {
	root := newTestRoot()
	grp := &cobra.Command{Use: "grp", RunE: func(*cobra.Command, []string) error { return nil }}
	grp.AddCommand(leaf("a", "dup.id", EffectRead), leaf("b", "dup.id", EffectMetadataWrite))
	root.AddCommand(grp)
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "dup.id") {
		t.Fatalf("want duplicate-id error naming the id, got %v", err)
	}
}

func TestCollectRejectsUnknownEffect(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(&cobra.Command{
		Use:         "bad",
		RunE:        func(*cobra.Command, []string) error { return nil },
		Annotations: map[string]string{capAnnotationID: "bad.op", capAnnotationEffects: "write"},
	})
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("want unknown-effect error naming the effect, got %v", err)
	}
}

func TestCollectRejectsEmptyEffects(t *testing.T) {
	root := newTestRoot()
	root.AddCommand(&cobra.Command{
		Use:         "bare",
		RunE:        func(*cobra.Command, []string) error { return nil },
		Annotations: map[string]string{capAnnotationID: "bare.op", capAnnotationEffects: ""},
	})
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "bare.op") {
		t.Fatalf("want empty-effects error naming the command, got %v", err)
	}
}

func TestCollectRejectsAnnotatedHidden(t *testing.T) {
	root := newTestRoot()
	c := leaf("secret", "secret.op", EffectRead)
	c.Hidden = true
	root.AddCommand(c)
	if _, err := collectCapabilities(root); err == nil || !strings.Contains(err.Error(), "secret") {
		t.Fatalf("want annotated-hidden error, got %v", err)
	}
}

func TestCollectSkipsHiddenSubtree(t *testing.T) {
	root := newTestRoot()
	grp := &cobra.Command{Use: "hgrp", Hidden: true, RunE: func(*cobra.Command, []string) error { return nil }}
	grp.AddCommand(leaf("child", "hgrp.child", EffectRead))
	root.AddCommand(grp)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatalf("hidden subtree must not error, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("hidden subtree must yield no entries, got %+v", entries)
	}
}

func TestSignatureRequiredOptionalRepeatableDefaults(t *testing.T) {
	c := leaf("reconcile", "change.reconcile", EffectMetadataWrite)
	c.Flags().String("request", "", "JSON request `file`, or - for stdin (required)")
	c.Flags().String("repo-dir", "", "repository `dir` to operate on")
	c.Flags().StringArray("type", nil, "filter `type` (repeatable)")
	_ = c.MarkFlagRequired("request")
	root := newTestRoot()
	root.AddCommand(c)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "--request <file> [--repo-dir <dir>] [--type <type>...]"
	if entries[0].Signature != want {
		t.Fatalf("signature = %q, want %q", entries[0].Signature, want)
	}
}

func TestSignatureDefaultRenderedInBrackets(t *testing.T) {
	c := leaf("scope", "maintenance.sweep", EffectMetadataWrite)
	c.Flags().String("scope", "full", "sweep `scope`")
	root := newTestRoot()
	root.AddCommand(c)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	// A meaningful, non-empty default surfaces inside the optional brackets;
	// false/0/[] would not.
	want := "[--scope <scope>=full]"
	if entries[0].Signature != want {
		t.Fatalf("signature = %q, want %q", entries[0].Signature, want)
	}
}

func TestSignaturePositionalTailStripsFlagRestatements(t *testing.T) {
	// gate launch shape: "launch --root <dir> --cwd <dir> -- <argv...>" — the
	// registered flags are restated in Use; the walker takes flags from typed
	// pflag data and keeps only the bare `--` separator and its tail from Use.
	launch := leaf("launch --root <dir> --cwd <dir> -- <argv...>", "gate.launch", EffectProcessControl)
	launch.Flags().String("root", "", "supervisor `dir`")
	launch.Flags().String("cwd", "", "working `dir`")
	_ = launch.MarkFlagRequired("root")
	_ = launch.MarkFlagRequired("cwd")
	root := newTestRoot()
	root.AddCommand(launch)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	// Composition: tail begins with the bare `--` separator, so the flags lead
	// and the `-- <argv...>` tail lands last. Required flags sorted by name.
	wantLaunch := "--cwd <dir> --root <dir> -- <argv...>"
	if entries[0].Signature != wantLaunch {
		t.Fatalf("launch signature = %q, want %q", entries[0].Signature, wantLaunch)
	}

	// gate-verdict shape: "gate-verdict <key> | --unattributed [<id>...]" — the
	// `--unattributed` restatement is stripped from the tail (its following
	// token `[<id>...]` is NOT a value placeholder, so it survives), and the
	// real optional flag is projected from pflag data. Tail leads (no bare --).
	verdict := leaf("gate-verdict <key> | --unattributed [<id>...]", "run.gate-verdict", EffectLocalWrite)
	verdict.Flags().Bool("unattributed", false, "attribute to no run")
	root2 := newTestRoot()
	root2.AddCommand(verdict)
	entries2, err := collectCapabilities(root2)
	if err != nil {
		t.Fatal(err)
	}
	wantVerdict := "<key> | [<id>...] [--unattributed]"
	if entries2[0].Signature != wantVerdict {
		t.Fatalf("verdict signature = %q, want %q", entries2[0].Signature, wantVerdict)
	}
}

func TestSignatureStartArgvBoundaryLandsLast(t *testing.T) {
	// gate drive start shape (change 0359): required flags, optional flags, then a
	// trailing bare `-- <argv...>` separator that the task-intent owner supplies.
	// The bare `--` in Use makes the argv tail land last while the flags lead,
	// exactly as `gate launch` composes.
	start := leaf("start --repo-dir <dir> --run-root <dir> --owner build|finalize|task -- <argv...>",
		"gate.drive.start", EffectProcessControl, EffectLocalWrite)
	start.Flags().String("run-root", "", "run `dir`")
	start.Flags().String("owner", "", "policy `role`")
	start.Flags().String("scope-id", "", "scope `id`")
	_ = start.MarkFlagRequired("run-root")
	_ = start.MarkFlagRequired("owner")
	root := newTestRoot()
	root.AddCommand(start)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "--owner <role> --run-root <dir> [--scope-id <id>] -- <argv...>"
	if entries[0].Signature != want {
		t.Fatalf("start signature = %q, want %q", entries[0].Signature, want)
	}
}

func TestSignatureGateClaimPositionalPair(t *testing.T) {
	// gate-claim shape (change 0359): two leading positionals (key, continuation
	// id) taken from Use, then the optional repo-dir flag from pflag data. No bare
	// `--`, so the positional tail leads and the flag trails.
	claim := leaf("gate-claim <key> <continuation-id>", "run.gate-claim", EffectLocalWrite)
	claim.Flags().String("repo-dir", "", "repository `dir` to operate on")
	root := newTestRoot()
	root.AddCommand(claim)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "<key> <continuation-id> [--repo-dir <dir>]"
	if entries[0].Signature != want {
		t.Fatalf("gate-claim signature = %q, want %q", entries[0].Signature, want)
	}
}

func TestSignatureExcludesRootJSONFlag(t *testing.T) {
	c := leaf("observe", "gate.observe", EffectRead)
	c.Flags().String("run-dir", "", "supervisor `dir`")
	root := newTestRoot()
	root.AddCommand(c)
	entries, err := collectCapabilities(root)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(entries[0].Signature, "json") {
		t.Fatalf("root --json leaked into signature %q", entries[0].Signature)
	}
}
