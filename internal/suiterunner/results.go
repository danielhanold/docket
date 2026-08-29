// This file owns the durable per-target result protocol: the JSON schema the
// runner publishes for every scheduled target, the atomic temp-beside-rename
// write, and the strict read the aggregator later cross-checks against the
// runner-observed truth. One file per target under <work>/stat, named by the
// target's .sh-stripped stem — mirroring the Bash oracle's per-file stat record
// (scripts/run-tests.sh launch()) so a human reads both with one set of eyes.
package suiterunner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resultSchema is the only durable-result schema version this runner writes and
// accepts. A file carrying any other value is refused rather than guessed at.
const resultSchema = 1

// Result is the durable per-target record, one JSON file per target at
// <work>/stat/<stem>.json, written temp-beside-destination + atomic rename.
type Result struct {
	Schema  int    `json:"schema"` // 1
	Target  string `json:"target"` // Base — identity, validated against filename
	RC      int    `json:"rc"`
	Seconds int    `json:"seconds"`
	OK      int    `json:"ok"`    // count of log lines matching ^ok[[:space:]]*-
	NotOK   int    `json:"notok"` // count of log lines matching ^NOT OK
}

// statFileName maps a target identity ("test_x.sh") to its durable result
// filename ("test_x.json") — the .sh-stripped stem plus .json, matching the
// oracle's stat basename.
func statFileName(target string) string {
	return strings.TrimSuffix(target, ".sh") + ".json"
}

// WriteResult atomically publishes r into dir as <stem>.json. It writes a
// temp file beside the destination (same filesystem, so os.Rename is atomic),
// chmods it 0644, then renames it over the final name — a reader therefore
// never observes a partial record. dir must already exist.
func WriteResult(dir string, r Result) error {
	if strings.TrimSpace(r.Target) == "" {
		return fmt.Errorf("suiterunner: WriteResult: empty target identity")
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("suiterunner: WriteResult: marshal: %w", err)
	}
	// Temp beside the destination so the rename stays same-filesystem atomic
	// (AGENTS.md: template a temp beside its destination for atomic rename).
	tmp, err := os.CreateTemp(dir, ".stat-*")
	if err != nil {
		return fmt.Errorf("suiterunner: WriteResult: create temp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("suiterunner: WriteResult: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("suiterunner: WriteResult: close temp: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("suiterunner: WriteResult: chmod: %w", err)
	}
	final := filepath.Join(dir, statFileName(r.Target))
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("suiterunner: WriteResult: rename: %w", err)
	}
	return nil
}

// ReadResult strictly reads and validates a durable Result file. It fails on an
// unreadable file, invalid JSON ("malformed"), an unsupported schema version
// ("unsupported schema"), or an empty target identity ("missing target
// identity"). It does NOT check the target against the filename — that
// cross-check is the aggregator's join (ValidateResults), which owns the whole
// scheduled set.
func ReadResult(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, fmt.Errorf("suiterunner: cannot read result %q: %w", path, err)
	}
	var r Result
	if err := json.Unmarshal(data, &r); err != nil {
		return Result{}, fmt.Errorf("suiterunner: malformed result %q: %w", path, err)
	}
	if r.Schema != resultSchema {
		return Result{}, fmt.Errorf("suiterunner: unsupported schema %d in result %q (want %d)", r.Schema, path, resultSchema)
	}
	if strings.TrimSpace(r.Target) == "" {
		return Result{}, fmt.Errorf("suiterunner: missing target identity in result %q", path)
	}
	return r, nil
}
