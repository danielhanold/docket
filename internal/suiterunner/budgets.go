// This file owns the budget vocabulary: the per-target Mode, the default
// ceiling, the runtime-budgets TSV loader, and the two integer-exact threshold
// predicates plus the oracle's threshold rendering. The arithmetic is copied
// verbatim from scripts/run-tests.sh so the Go runner and the Bash oracle reach
// identical verdicts on identical measurements (screening secs*2 > ceil*5,
// authoritative solo secs*2 > ceil*3).
package suiterunner

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Mode is a target's scheduling lane: parallel (the default) or serial.
type Mode string

const (
	ModeParallel Mode = "parallel"
	ModeSerial   Mode = "serial"
)

// DefaultCeiling is the per-file wall-clock ceiling, in seconds, used when the
// budget table has no row for a target (or a malformed one). It mirrors the
// Bash oracle's default.
const DefaultCeiling = 60

// budgetRow is one parsed runtime-budgets row, keyed elsewhere by basename.
type budgetRow struct {
	Ceiling int
	Mode    Mode
}

// LoadBudgets parses the runtime-budgets TSV: `<path>\t<seconds>\t<parallel|serial>`,
// comment/blank lines skipped, keyed by basename. A malformed seconds field
// falls back to DefaultCeiling (the oracle keeps running the tests; the table's
// own guard test makes malformed rows loud). Missing file => empty map, nil error.
func LoadBudgets(path string) (map[string]budgetRow, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]budgetRow{}, nil
		}
		return nil, err
	}
	defer f.Close()

	rows := make(map[string]budgetRow)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			continue
		}
		base := filepath.Base(strings.TrimSpace(fields[0]))
		ceiling := DefaultCeiling
		if secs, convErr := strconv.Atoi(strings.TrimSpace(fields[1])); convErr == nil {
			ceiling = secs
		}
		rows[base] = budgetRow{Ceiling: ceiling, Mode: parseMode(fields[2])}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

// parseMode maps the mode column to a Mode: only "serial" is serial; every
// other spelling (including unknown values) is the parallel default.
func parseMode(field string) Mode {
	if strings.TrimSpace(field) == string(ModeSerial) {
		return ModeSerial
	}
	return ModeParallel
}

// ScreenOver is the contended parallel screening predicate: secs*2 > ceil*5.
func ScreenOver(secs, ceil int) bool { return secs*2 > ceil*5 }

// SoloOver is the authoritative solo / serial-mode / -j1 predicate:
// secs*2 > ceil*3.
func SoloOver(secs, ceil int) bool { return secs*2 > ceil*3 }

// SoloThreshold renders ceil*3/2 the way the oracle prints it: "45" or "22.5".
func SoloThreshold(ceil int) string {
	half := ceil * 3
	if half%2 == 0 {
		return strconv.Itoa(half / 2)
	}
	return strconv.Itoa(half/2) + ".5"
}
