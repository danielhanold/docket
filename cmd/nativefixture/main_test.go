package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestPrepareRefusesExistingDestinationBeforeCandidateActions(t *testing.T) {
	root := testsupport.TempDir(t)
	dest := filepath.Join(root, "existing")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	err := prepare(options{Source: filepath.Join(root, "source"), Binary: filepath.Join(root, "docket"), Destination: dest, Pins: filepath.Join(root, "pins.yml")})
	if err == nil || !strings.Contains(err.Error(), "destination must not exist") {
		t.Fatalf("prepare error=%v", err)
	}
}

func TestVerifyCandidateIdentityRejectsMismatchedAssetSet(t *testing.T) {
	root := testsupport.TempDir(t)
	binary := filepath.Join(root, "docket")
	body := "#!/bin/sh\nprintf '%s\\n' '{\"commit\":\"abc123\",\"asset_set_id\":\"sha256:binary\"}'\n"
	if err := os.WriteFile(binary, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	err := verifyCandidateIdentity(binary, "abc123", "sha256:source")
	if err == nil || !strings.Contains(err.Error(), "asset set") {
		t.Fatalf("verify candidate identity error=%v", err)
	}
}

func TestCompleteManifestFilesPreservesGeneratedFilesAndRejectsDrift(t *testing.T) {
	root := testsupport.TempDir(t)
	rel := ".codex/agents/docket-review.toml"
	original := []byte("model = \"test\"\n")
	if err := writeFile(filepath.Join(root, filepath.FromSlash(rel)), original, 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := map[string]string{rel: hash(original)}
	got, err := completeManifestFiles(root, explicit, map[string]string{"tracked.txt": hash([]byte("tracked"))})
	if err != nil {
		t.Fatalf("complete manifest: %v", err)
	}
	if got[rel] != hash(original) || got["tracked.txt"] == "" {
		t.Fatalf("complete manifest omitted inputs: %#v", got)
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := completeManifestFiles(root, explicit, nil); err == nil {
		t.Fatal("complete manifest accepted a rendered agent mutated after hashing")
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := completeManifestFiles(root, explicit, map[string]string{rel: hash([]byte("different"))}); err == nil {
		t.Fatal("complete manifest accepted conflicting hashes for one path")
	}
}
