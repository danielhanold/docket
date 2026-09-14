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
